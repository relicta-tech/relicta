// Package template provides template rendering for ReleasePilot.
package template

import (
	"strings"
	"testing"
	"time"

	"github.com/felixgeelhaar/release-pilot/internal/domain/version"
)

func TestDefaultServiceConfig(t *testing.T) {
	cfg := DefaultServiceConfig()

	if cfg.DefaultFormat != "text" {
		t.Errorf("DefaultFormat = %v, want text", cfg.DefaultFormat)
	}
	if cfg.ExecutionTimeout != DefaultExecutionTimeout {
		t.Errorf("ExecutionTimeout = %v, want %v", cfg.ExecutionTimeout, DefaultExecutionTimeout)
	}
}

func TestServiceOptions(t *testing.T) {
	cfg := DefaultServiceConfig()

	WithCustomDir("/custom/templates")(&cfg)
	WithExecutionTimeout(10 * time.Second)(&cfg)

	if cfg.CustomDir != "/custom/templates" {
		t.Errorf("CustomDir = %v, want /custom/templates", cfg.CustomDir)
	}
	if cfg.ExecutionTimeout != 10*time.Second {
		t.Errorf("ExecutionTimeout = %v, want 10s", cfg.ExecutionTimeout)
	}
}

func TestNewService(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	if svc == nil {
		t.Fatal("NewService returned nil")
	}

	// Check default timeout is set
	if svc.executionTimeout != DefaultExecutionTimeout {
		t.Errorf("executionTimeout = %v, want %v", svc.executionTimeout, DefaultExecutionTimeout)
	}
}

func TestNewService_WithOptions(t *testing.T) {
	svc, err := NewService(
		WithExecutionTimeout(15 * time.Second),
	)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	if svc.executionTimeout != 15*time.Second {
		t.Errorf("executionTimeout = %v, want 15s", svc.executionTimeout)
	}
}

func TestServiceImpl_RenderString(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	tests := []struct {
		name     string
		template string
		data     any
		want     string
		wantErr  bool
	}{
		{
			name:     "simple template",
			template: "Hello, {{.Name}}!",
			data:     map[string]string{"Name": "World"},
			want:     "Hello, World!",
		},
		{
			name:     "empty template",
			template: "",
			data:     nil,
			want:     "",
		},
		{
			name:     "template with upper function",
			template: "{{upper .Text}}",
			data:     map[string]string{"Text": "hello"},
			want:     "HELLO",
		},
		{
			name:     "template with lower function",
			template: "{{lower .Text}}",
			data:     map[string]string{"Text": "HELLO"},
			want:     "hello",
		},
		{
			name:     "template with trim function",
			template: "{{trim .Text}}",
			data:     map[string]string{"Text": "  hello  "},
			want:     "hello",
		},
		{
			name:     "invalid template",
			template: "{{.Invalid",
			data:     nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.RenderString(tt.template, tt.data)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("RenderString failed: %v", err)
				return
			}
			if result != tt.want {
				t.Errorf("RenderString = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestServiceImpl_RegisterTemplate(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Register a custom template
	err = svc.RegisterTemplate("custom", "Custom: {{.Value}}")
	if err != nil {
		t.Fatalf("RegisterTemplate failed: %v", err)
	}

	// Render the custom template
	result, err := svc.Render("custom", map[string]string{"Value": "test"})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Custom: test" {
		t.Errorf("Render = %q, want %q", result, "Custom: test")
	}
}

func TestServiceImpl_RegisterTemplate_Invalid(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Try to register an invalid template
	err = svc.RegisterTemplate("invalid", "{{.Invalid")
	if err == nil {
		t.Error("Expected error for invalid template")
	}
}

func TestServiceImpl_LoadTemplate(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Register a template
	err = svc.RegisterTemplate("loadtest", "Test template")
	if err != nil {
		t.Fatalf("RegisterTemplate failed: %v", err)
	}

	// Load the template
	tmpl, err := svc.LoadTemplate("loadtest")
	if err != nil {
		t.Fatalf("LoadTemplate failed: %v", err)
	}
	if tmpl == nil {
		t.Error("LoadTemplate returned nil")
	}
}

func TestServiceImpl_LoadTemplate_NotFound(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = svc.LoadTemplate("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent template")
	}
}

func TestServiceImpl_ListTemplates(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Register some templates
	_ = svc.RegisterTemplate("test1", "Template 1")
	_ = svc.RegisterTemplate("test2", "Template 2")

	templates := svc.ListTemplates()
	if len(templates) < 2 {
		t.Errorf("ListTemplates returned %d templates, want at least 2", len(templates))
	}

	// Check our templates are in the list
	found := make(map[string]bool)
	for _, name := range templates {
		found[name] = true
	}
	if !found["test1"] {
		t.Error("test1 not found in template list")
	}
	if !found["test2"] {
		t.Error("test2 not found in template list")
	}
}

func TestServiceImpl_Render_NotFound(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	_, err = svc.Render("nonexistent", nil)
	if err == nil {
		t.Error("Expected error for nonexistent template")
	}
}

func TestTemplateFunctions_DateFunctions(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	testDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	data := map[string]time.Time{"Date": testDate}

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "dateISO",
			template: `{{dateISO .Date}}`,
			want:     "2024-01-15",
		},
		{
			name:     "formatDate",
			template: `{{formatDate "Jan 2, 2006" .Date}}`,
			want:     "Jan 15, 2024",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.RenderString(tt.template, data)
			if err != nil {
				t.Errorf("RenderString failed: %v", err)
				return
			}
			if result != tt.want {
				t.Errorf("RenderString = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestTemplateFunctions_UtilityFunctions(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	tests := []struct {
		name     string
		template string
		data     any
		want     string
	}{
		{
			name:     "default with empty",
			template: `{{default "fallback" .Value}}`,
			data:     map[string]string{"Value": ""},
			want:     "fallback",
		},
		{
			name:     "default with value",
			template: `{{default "fallback" .Value}}`,
			data:     map[string]string{"Value": "actual"},
			want:     "actual",
		},
		{
			name:     "ternary true",
			template: `{{ternary .Cond "yes" "no"}}`,
			data:     map[string]bool{"Cond": true},
			want:     "yes",
		},
		{
			name:     "ternary false",
			template: `{{ternary .Cond "yes" "no"}}`,
			data:     map[string]bool{"Cond": false},
			want:     "no",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.RenderString(tt.template, tt.data)
			if err != nil {
				t.Errorf("RenderString failed: %v", err)
				return
			}
			if result != tt.want {
				t.Errorf("RenderString = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestTemplateFunctions_MarkdownFunctions(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	tests := []struct {
		name     string
		template string
		data     any
		want     string
	}{
		{
			name:     "mdLink",
			template: `{{mdLink "GitHub" "https://github.com"}}`,
			data:     nil,
			want:     "[GitHub](https://github.com)",
		},
		{
			name:     "mdBold",
			template: `{{mdBold "important"}}`,
			data:     nil,
			want:     "**important**",
		},
		{
			name:     "mdCode",
			template: `{{mdCode "code"}}`,
			data:     nil,
			want:     "`code`",
		},
		{
			name:     "mdQuote single line",
			template: `{{mdQuote "quoted text"}}`,
			data:     nil,
			want:     "> quoted text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.RenderString(tt.template, tt.data)
			if err != nil {
				t.Errorf("RenderString failed: %v", err)
				return
			}
			if result != tt.want {
				t.Errorf("RenderString = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestTemplateFunctions_StringFunctions(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	tests := []struct {
		name     string
		template string
		data     any
		want     string
	}{
		{
			name:     "trimPrefix",
			template: `{{trimPrefix .Text "v"}}`,
			data:     map[string]string{"Text": "v1.0.0"},
			want:     "1.0.0",
		},
		{
			name:     "trimSuffix",
			template: `{{trimSuffix .Text ".md"}}`,
			data:     map[string]string{"Text": "README.md"},
			want:     "README",
		},
		{
			name:     "replace",
			template: `{{replace .Text "-" "_"}}`,
			data:     map[string]string{"Text": "hello-world"},
			want:     "hello_world",
		},
		{
			name:     "contains true",
			template: `{{if contains .Text "hello"}}found{{end}}`,
			data:     map[string]string{"Text": "hello world"},
			want:     "found",
		},
		{
			name:     "hasPrefix true",
			template: `{{if hasPrefix .Text "v"}}has prefix{{end}}`,
			data:     map[string]string{"Text": "v1.0.0"},
			want:     "has prefix",
		},
		{
			name:     "hasSuffix true",
			template: `{{if hasSuffix .Text ".go"}}go file{{end}}`,
			data:     map[string]string{"Text": "main.go"},
			want:     "go file",
		},
		{
			name:     "split",
			template: `{{index (split .Text ",") 0}}`,
			data:     map[string]string{"Text": "a,b,c"},
			want:     "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.RenderString(tt.template, tt.data)
			if err != nil {
				t.Errorf("RenderString failed: %v", err)
				return
			}
			if result != tt.want {
				t.Errorf("RenderString = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestChangelogData_Fields(t *testing.T) {
	v1 := version.NewSemanticVersion(1, 0, 0)
	v2 := version.NewSemanticVersion(0, 9, 0)
	data := ChangelogData{
		Version:         &v1,
		PreviousVersion: &v2,
		Date:            time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		RepositoryURL:   "https://github.com/user/repo",
		IssueURL:        "https://github.com/user/repo/issues/{id}",
		CompareURL:      "https://github.com/user/repo/compare/v0.9.0...v1.0.0",
	}

	if data.Version.String() != "1.0.0" {
		t.Errorf("Version = %v, want 1.0.0", data.Version.String())
	}
	if data.PreviousVersion.String() != "0.9.0" {
		t.Errorf("PreviousVersion = %v, want 0.9.0", data.PreviousVersion.String())
	}
	if data.RepositoryURL != "https://github.com/user/repo" {
		t.Errorf("RepositoryURL unexpected value")
	}
}

func TestReleaseNotesData_Fields(t *testing.T) {
	v := version.NewSemanticVersion(2, 0, 0)
	data := ReleaseNotesData{
		Version:       &v,
		Date:          time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		Changelog:     "## Changelog content",
		Summary:       "Major release",
		Highlights:    []string{"Feature 1", "Feature 2"},
		Contributors:  []string{"john", "jane"},
		RepositoryURL: "https://github.com/user/repo",
	}

	if data.Version.String() != "2.0.0" {
		t.Errorf("Version = %v, want 2.0.0", data.Version.String())
	}
	if data.Summary != "Major release" {
		t.Errorf("Summary = %v, want Major release", data.Summary)
	}
	if len(data.Highlights) != 2 {
		t.Errorf("Highlights length = %v, want 2", len(data.Highlights))
	}
	if len(data.Contributors) != 2 {
		t.Errorf("Contributors length = %v, want 2", len(data.Contributors))
	}
}

func TestMarketingData_Fields(t *testing.T) {
	v := version.NewSemanticVersion(1, 5, 0)
	data := MarketingData{
		Version:     &v,
		Date:        time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
		Summary:     "Exciting new features",
		Highlights:  []string{"Speed improvements", "New UI"},
		ProductName: "MyProduct",
		ReleaseURL:  "https://github.com/user/repo/releases/v1.5.0",
	}

	if data.Version.String() != "1.5.0" {
		t.Errorf("Version = %v, want 1.5.0", data.Version.String())
	}
	if data.ProductName != "MyProduct" {
		t.Errorf("ProductName = %v, want MyProduct", data.ProductName)
	}
	if !strings.Contains(data.ReleaseURL, "v1.5.0") {
		t.Errorf("ReleaseURL should contain version")
	}
}

func TestIndentFunc(t *testing.T) {
	tests := []struct {
		name   string
		spaces int
		input  string
		want   string
	}{
		{
			name:   "single line",
			spaces: 2,
			input:  "hello",
			want:   "  hello",
		},
		{
			name:   "multi line",
			spaces: 4,
			input:  "line1\nline2\nline3",
			want:   "    line1\n    line2\n    line3",
		},
		{
			name:   "zero indent",
			spaces: 0,
			input:  "hello",
			want:   "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indentFunc(tt.spaces, tt.input)
			if result != tt.want {
				t.Errorf("indentFunc(%d, %q) = %q, want %q", tt.spaces, tt.input, result, tt.want)
			}
		})
	}
}

func TestWrapFunc(t *testing.T) {
	tests := []struct {
		name  string
		width int
		input string
		want  string
	}{
		{
			name:  "no wrap needed",
			width: 50,
			input: "short text",
			want:  "short text",
		},
		{
			name:  "zero width",
			width: 0,
			input: "any text",
			want:  "any text",
		},
		{
			name:  "wrap long text",
			width: 10,
			input: "hello world test",
			want:  "hello\nworld test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapFunc(tt.width, tt.input)
			if result != tt.want {
				t.Errorf("wrapFunc(%d, %q) = %q, want %q", tt.width, tt.input, result, tt.want)
			}
		})
	}
}

func TestLenFunc(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  int
	}{
		{"string", "hello", 5},
		{"empty string", "", 0},
		{"slice", []any{1, 2, 3}, 3},
		{"empty slice", []any{}, 0},
		{"map", map[string]any{"a": 1, "b": 2}, 2},
		{"unsupported type", 123, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lenFunc(tt.input)
			if result != tt.want {
				t.Errorf("lenFunc(%v) = %d, want %d", tt.input, result, tt.want)
			}
		})
	}
}

func TestEmptyFunc(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  bool
	}{
		{"empty string", "", true},
		{"non-empty string", "hello", false},
		{"empty slice", []any{}, true},
		{"non-empty slice", []any{1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := emptyFunc(tt.input)
			if result != tt.want {
				t.Errorf("emptyFunc(%v) = %v, want %v", tt.input, result, tt.want)
			}
		})
	}
}

func TestFirstFunc(t *testing.T) {
	tests := []struct {
		name  string
		input []any
		want  any
	}{
		{"non-empty", []any{1, 2, 3}, 1},
		{"single element", []any{"only"}, "only"},
		{"empty", []any{}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := firstFunc(tt.input)
			if result != tt.want {
				t.Errorf("firstFunc(%v) = %v, want %v", tt.input, result, tt.want)
			}
		})
	}
}

func TestLastFunc(t *testing.T) {
	tests := []struct {
		name  string
		input []any
		want  any
	}{
		{"non-empty", []any{1, 2, 3}, 3},
		{"single element", []any{"only"}, "only"},
		{"empty", []any{}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lastFunc(tt.input)
			if result != tt.want {
				t.Errorf("lastFunc(%v) = %v, want %v", tt.input, result, tt.want)
			}
		})
	}
}

func TestCoalesceFunc(t *testing.T) {
	tests := []struct {
		name   string
		values []any
		want   any
	}{
		{"all nil", []any{nil, nil, nil}, nil},
		{"first non-nil", []any{nil, "value", "other"}, "value"},
		{"first non-empty", []any{"", "", "value"}, "value"},
		{"no values", []any{}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := coalesceFunc(tt.values...)
			if result != tt.want {
				t.Errorf("coalesceFunc(%v) = %v, want %v", tt.values, result, tt.want)
			}
		})
	}
}
