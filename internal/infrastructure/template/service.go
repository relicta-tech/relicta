// Package template provides template rendering for Relicta.
package template

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/relicta-tech/relicta/internal/domain/version"
	rperrors "github.com/relicta-tech/relicta/internal/errors"
	"github.com/relicta-tech/relicta/internal/infrastructure/git"
)

// bufferPool is used to reuse buffers for template execution to reduce GC pressure.
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// DefaultExecutionTimeout is the maximum time allowed for template execution.
// This prevents DoS attacks from malicious templates with infinite loops.
const DefaultExecutionTimeout = 5 * time.Second

//go:embed templates/*.tmpl
var embeddedTemplates embed.FS

// Service provides template rendering capabilities.
type Service interface {
	// Render renders a template with the given data.
	Render(name string, data any) (string, error)

	// RenderString renders a template string with the given data.
	RenderString(tmpl string, data any) (string, error)

	// RenderFile renders a template file with the given data.
	RenderFile(path string, data any) (string, error)

	// LoadTemplate loads a template by name.
	LoadTemplate(name string) (*template.Template, error)

	// RegisterTemplate registers a custom template.
	RegisterTemplate(name string, content string) error

	// ListTemplates returns all available template names.
	ListTemplates() []string
}

// ServiceImpl is the implementation of the template service.
type ServiceImpl struct {
	mu               sync.RWMutex
	templates        map[string]*template.Template
	customDir        string
	funcMap          template.FuncMap
	defaultFormat    string
	executionTimeout time.Duration
}

// ServiceConfig configures the template service.
type ServiceConfig struct {
	// CustomDir is the directory for custom templates.
	CustomDir string
	// DefaultFormat is the default template format (text, html).
	DefaultFormat string
	// ExecutionTimeout is the maximum time allowed for template execution.
	// Zero or negative values use DefaultExecutionTimeout.
	ExecutionTimeout time.Duration
}

// DefaultServiceConfig returns the default service configuration.
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		DefaultFormat:    "text",
		ExecutionTimeout: DefaultExecutionTimeout,
	}
}

// ServiceOption configures the template service.
type ServiceOption func(*ServiceConfig)

// WithCustomDir sets the custom templates directory.
func WithCustomDir(dir string) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.CustomDir = dir
	}
}

// WithExecutionTimeout sets the maximum template execution time.
func WithExecutionTimeout(timeout time.Duration) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.ExecutionTimeout = timeout
	}
}

// NewService creates a new template service.
func NewService(opts ...ServiceOption) (*ServiceImpl, error) {
	cfg := DefaultServiceConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	// Ensure execution timeout is set
	timeout := cfg.ExecutionTimeout
	if timeout <= 0 {
		timeout = DefaultExecutionTimeout
	}

	s := &ServiceImpl{
		templates:        make(map[string]*template.Template),
		customDir:        cfg.CustomDir,
		defaultFormat:    cfg.DefaultFormat,
		funcMap:          createFuncMap(),
		executionTimeout: timeout,
	}

	// Load embedded templates
	if err := s.loadEmbeddedTemplates(); err != nil {
		return nil, rperrors.TemplateWrap(err, "template.NewService", "failed to load embedded templates")
	}

	// Load custom templates if directory is specified
	if cfg.CustomDir != "" {
		if err := s.loadCustomTemplates(); err != nil {
			// Custom templates are optional, log the error but continue
			slog.Warn("failed to load custom templates",
				"dir", cfg.CustomDir,
				"error", err,
			)
		}
	}

	return s, nil
}

// loadEmbeddedTemplates loads templates from the embedded filesystem.
func (s *ServiceImpl) loadEmbeddedTemplates() error {
	return fs.WalkDir(embeddedTemplates, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}

		content, err := embeddedTemplates.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read embedded template %s: %w", path, err)
		}

		name := strings.TrimPrefix(path, "templates/")
		name = strings.TrimSuffix(name, ".tmpl")

		tmpl, err := template.New(name).Funcs(s.funcMap).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse embedded template %s: %w", name, err)
		}

		s.templates[name] = tmpl
		return nil
	})
}

// loadCustomTemplates loads templates from the custom directory.
func (s *ServiceImpl) loadCustomTemplates() error {
	if s.customDir == "" {
		return nil
	}

	return filepath.Walk(s.customDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read custom template %s: %w", path, err)
		}

		name := strings.TrimPrefix(path, s.customDir+string(os.PathSeparator))
		name = strings.TrimSuffix(name, ".tmpl")

		tmpl, err := template.New(name).Funcs(s.funcMap).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse custom template %s: %w", name, err)
		}

		// Custom templates override embedded ones
		s.templates[name] = tmpl
		return nil
	})
}

// Render renders a template with the given data.
func (s *ServiceImpl) Render(name string, data any) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.executionTimeout)
	defer cancel()
	return s.RenderWithContext(ctx, name, data)
}

// RenderWithContext renders a template with the given data and context.
// The context allows for cancellation and timeout control.
func (s *ServiceImpl) RenderWithContext(ctx context.Context, name string, data any) (string, error) {
	const op = "template.Render"

	s.mu.RLock()
	tmpl, ok := s.templates[name]
	s.mu.RUnlock()

	if !ok {
		return "", rperrors.NotFound(op, fmt.Sprintf("template not found: %s", name))
	}

	return s.executeWithTimeout(ctx, op, tmpl, data)
}

// executeWithTimeout executes a template with timeout protection.
// This prevents DoS attacks from malicious templates with infinite loops.
// Uses buffer pooling to reduce GC pressure for frequent template renders.
//
// Note: Go's template.Execute is not cancellable. If the context times out,
// the goroutine will continue running until template execution completes.
// The buffer is properly returned to the pool in all cases to prevent leaks.
// This is a known limitation - for truly malicious templates, consider
// running in a separate process with resource limits.
func (s *ServiceImpl) executeWithTimeout(ctx context.Context, op string, tmpl *template.Template, data any) (string, error) {
	type result struct {
		output string
		err    error
	}

	// Buffered channel ensures goroutine can always send result and exit
	done := make(chan result, 1)

	go func() {
		// Get a buffer from the pool - must be returned in all paths
		buf := bufferPool.Get().(*bytes.Buffer)
		buf.Reset()

		// Recover from panics in template execution to prevent crashes
		defer func() {
			// Always return buffer to pool, even on panic
			bufferPool.Put(buf)

			if r := recover(); r != nil {
				done <- result{err: rperrors.TemplateWrap(
					fmt.Errorf("template panic: %v", r),
					op,
					fmt.Sprintf("template execution panicked: %s", tmpl.Name()),
				)}
			}
		}()

		if err := tmpl.Execute(buf, data); err != nil {
			done <- result{err: rperrors.TemplateWrap(err, op, fmt.Sprintf("failed to render template %s", tmpl.Name()))}
			return
		}

		// Copy the result before returning the buffer to the pool (via defer)
		output := buf.String()
		done <- result{output: output}
	}()

	select {
	case <-ctx.Done():
		// Goroutine will continue but buffer will be returned via defer
		return "", rperrors.TimeoutWrap(ctx.Err(), op, fmt.Sprintf("template execution timed out: %s", tmpl.Name()))
	case r := <-done:
		return r.output, r.err
	}
}

// RenderString renders a template string with the given data.
func (s *ServiceImpl) RenderString(tmplStr string, data any) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.executionTimeout)
	defer cancel()
	return s.RenderStringWithContext(ctx, tmplStr, data)
}

// RenderStringWithContext renders a template string with the given data and context.
func (s *ServiceImpl) RenderStringWithContext(ctx context.Context, tmplStr string, data any) (string, error) {
	const op = "template.RenderString"

	tmpl, err := template.New("inline").Funcs(s.funcMap).Parse(tmplStr)
	if err != nil {
		return "", rperrors.TemplateWrap(err, op, "failed to parse template string")
	}

	return s.executeWithTimeout(ctx, op, tmpl, data)
}

// RenderFile renders a template file with the given data.
func (s *ServiceImpl) RenderFile(path string, data any) (string, error) {
	const op = "template.RenderFile"

	content, err := os.ReadFile(path)
	if err != nil {
		return "", rperrors.IOWrap(err, op, fmt.Sprintf("failed to read template file: %s", path))
	}

	return s.RenderString(string(content), data)
}

// LoadTemplate loads a template by name.
// This method is thread-safe and can be called concurrently.
func (s *ServiceImpl) LoadTemplate(name string) (*template.Template, error) {
	const op = "template.LoadTemplate"

	s.mu.RLock()
	tmpl, ok := s.templates[name]
	s.mu.RUnlock()

	if !ok {
		return nil, rperrors.NotFound(op, fmt.Sprintf("template not found: %s", name))
	}

	return tmpl, nil
}

// RegisterTemplate registers a custom template.
// This method is thread-safe and can be called concurrently.
func (s *ServiceImpl) RegisterTemplate(name string, content string) error {
	const op = "template.RegisterTemplate"

	tmpl, err := template.New(name).Funcs(s.funcMap).Parse(content)
	if err != nil {
		return rperrors.TemplateWrap(err, op, fmt.Sprintf("failed to parse template %s", name))
	}

	s.mu.Lock()
	s.templates[name] = tmpl
	s.mu.Unlock()
	return nil
}

// ListTemplates returns all available template names.
// This method is thread-safe and can be called concurrently.
func (s *ServiceImpl) ListTemplates() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.templates))
	for name := range s.templates {
		names = append(names, name)
	}
	return names
}

// createFuncMap creates the template function map.
func createFuncMap() template.FuncMap {
	return template.FuncMap{
		// String functions
		"upper":      strings.ToUpper,
		"lower":      strings.ToLower,
		"title":      cases.Title(language.English).String,
		"trim":       strings.TrimSpace,
		"trimPrefix": strings.TrimPrefix,
		"trimSuffix": strings.TrimSuffix,
		"replace":    strings.ReplaceAll,
		"contains":   strings.Contains,
		"hasPrefix":  strings.HasPrefix,
		"hasSuffix":  strings.HasSuffix,
		"split":      strings.Split,
		"join":       strings.Join,

		// Date functions
		"now":        time.Now,
		"formatDate": formatDate,
		"dateISO":    dateISO,

		// Version functions
		"formatVersion": formatVersionFunc,

		// Commit functions
		"commitTypeDisplay": git.CommitTypeDisplayName,
		"commitTypeEmoji":   git.CommitTypeEmoji,

		// Utility functions
		"default":  defaultFunc,
		"coalesce": coalesceFunc,
		"ternary":  ternaryFunc,
		"indent":   indentFunc,
		"nindent":  nindentFunc,
		"wrap":     wrapFunc,

		// List functions
		"list":   listFunc,
		"first":  firstFunc,
		"last":   lastFunc,
		"len":    lenFunc,
		"empty":  emptyFunc,
		"append": appendFunc,

		// Markdown functions
		"mdLink":  mdLinkFunc,
		"mdBold":  mdBoldFunc,
		"mdCode":  mdCodeFunc,
		"mdQuote": mdQuoteFunc,
	}
}

// Template functions

func formatDate(format string, t time.Time) string {
	return t.Format(format)
}

func dateISO(t time.Time) string {
	return t.Format("2006-01-02")
}

func formatVersionFunc(v *version.SemanticVersion) string {
	if v == nil {
		return ""
	}
	return v.String()
}

func defaultFunc(def, value any) any {
	if value == nil || value == "" {
		return def
	}
	return value
}

func coalesceFunc(values ...any) any {
	for _, v := range values {
		if v != nil && v != "" {
			return v
		}
	}
	return nil
}

func ternaryFunc(condition bool, trueVal, falseVal any) any {
	if condition {
		return trueVal
	}
	return falseVal
}

func indentFunc(spaces int, s string) string {
	indent := strings.Repeat(" ", spaces)
	return indent + strings.ReplaceAll(s, "\n", "\n"+indent)
}

func nindentFunc(spaces int, s string) string {
	return "\n" + indentFunc(spaces, s)
}

func wrapFunc(width int, s string) string {
	if width <= 0 || len(s) <= width {
		return s
	}

	var result strings.Builder
	words := strings.Fields(s)
	lineLen := 0

	for i, word := range words {
		if i > 0 {
			if lineLen+1+len(word) > width {
				result.WriteString("\n")
				lineLen = 0
			} else {
				result.WriteString(" ")
				lineLen++
			}
		}
		result.WriteString(word)
		lineLen += len(word)
	}

	return result.String()
}

func listFunc(args ...any) []any {
	return args
}

func firstFunc(list []any) any {
	if len(list) == 0 {
		return nil
	}
	return list[0]
}

func lastFunc(list []any) any {
	if len(list) == 0 {
		return nil
	}
	return list[len(list)-1]
}

func lenFunc(v any) int {
	switch val := v.(type) {
	case string:
		return len(val)
	case []any:
		return len(val)
	case map[string]any:
		return len(val)
	default:
		return 0
	}
}

func emptyFunc(v any) bool {
	return lenFunc(v) == 0
}

func appendFunc(list []any, item any) []any {
	return append(list, item)
}

func mdLinkFunc(text, url string) string {
	return fmt.Sprintf("[%s](%s)", text, url)
}

func mdBoldFunc(text string) string {
	return fmt.Sprintf("**%s**", text)
}

func mdCodeFunc(text string) string {
	return fmt.Sprintf("`%s`", text)
}

func mdQuoteFunc(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = "> " + line
	}
	return strings.Join(lines, "\n")
}

// Data types for templates

// ChangelogData contains data for changelog templates.
type ChangelogData struct {
	// Version is the release version.
	Version *version.SemanticVersion
	// PreviousVersion is the previous version.
	PreviousVersion *version.SemanticVersion
	// Date is the release date.
	Date time.Time
	// Changes are the categorized changes.
	Changes *git.CategorizedChanges
	// RepositoryURL is the repository URL.
	RepositoryURL string
	// IssueURL is the issue tracker URL pattern.
	IssueURL string
	// CompareURL is the URL for comparing versions.
	CompareURL string
}

// ReleaseNotesData contains data for release notes templates.
type ReleaseNotesData struct {
	// Version is the release version.
	Version *version.SemanticVersion
	// Date is the release date.
	Date time.Time
	// Changelog is the generated changelog.
	Changelog string
	// Summary is a summary of the release.
	Summary string
	// Highlights are the key highlights.
	Highlights []string
	// Changes are the categorized changes.
	Changes *git.CategorizedChanges
	// Contributors are the contributors to this release.
	Contributors []string
	// RepositoryURL is the repository URL.
	RepositoryURL string
}

// MarketingData contains data for marketing blurb templates.
type MarketingData struct {
	// Version is the release version.
	Version *version.SemanticVersion
	// Date is the release date.
	Date time.Time
	// Summary is a summary of the release.
	Summary string
	// Highlights are the key highlights.
	Highlights []string
	// ProductName is the product name.
	ProductName string
	// ReleaseURL is the URL to the release.
	ReleaseURL string
}
