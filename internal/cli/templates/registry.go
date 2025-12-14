// Package templates provides project detection and template management for the init wizard.
package templates

import (
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

//go:embed data/*.yaml.tmpl
var templateFiles embed.FS

// Template represents a configuration template with metadata.
type Template struct {
	// Name is the unique identifier for this template.
	Name string
	// DisplayName is the human-readable name shown in the UI.
	DisplayName string
	// Description explains what this template is for.
	Description string
	// SupportedLanguages are the languages this template works well with.
	SupportedLanguages []Language
	// SupportedProjectTypes are the project types this template targets.
	SupportedProjectTypes []ProjectType
	// Tags are additional categorization labels.
	Tags []string
	// Content is the raw template content.
	Content string
	// Template is the parsed Go template.
	Template *template.Template
}

// Registry manages available configuration templates.
type Registry struct {
	templates map[string]*Template
}

// NewRegistry creates a new template registry and loads all embedded templates.
func NewRegistry() (*Registry, error) {
	r := &Registry{
		templates: make(map[string]*Template),
	}

	if err := r.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return r, nil
}

// templateFuncs returns the template functions available to all templates.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"extractOwner": extractOwner,
	}
}

// extractOwner extracts the owner/organization from a Git repository URL.
// Examples:
// - https://github.com/user/repo → user
// - https://github.com/user/repo.git → user
// - git@github.com:user/repo.git → user
func extractOwner(repoURL string) string {
	if repoURL == "" {
		return ""
	}

	// Remove .git suffix
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// Handle SSH URLs (git@github.com:user/repo)
	if strings.Contains(repoURL, "@") && strings.Contains(repoURL, ":") {
		parts := strings.Split(repoURL, ":")
		if len(parts) > 1 {
			pathParts := strings.Split(parts[1], "/")
			if len(pathParts) > 0 {
				return pathParts[0]
			}
		}
	}

	// Handle HTTPS URLs (https://github.com/user/repo)
	if strings.Contains(repoURL, "://") {
		parts := strings.Split(repoURL, "/")
		if len(parts) >= 4 {
			return parts[3] // https://github.com/[USER]/repo
		}
	}

	// Fallback: try to extract from path format
	parts := strings.Split(repoURL, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}

	return ""
}

// loadTemplates reads and parses all embedded template files.
func (r *Registry) loadTemplates() error {
	entries, err := templateFiles.ReadDir("data")
	if err != nil {
		return fmt.Errorf("failed to read template directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml.tmpl") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".yaml.tmpl")
		path := filepath.Join("data", entry.Name())

		// Read template content
		content, err := templateFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", name, err)
		}

		// Parse template with custom functions
		tmpl, err := template.New(name).Funcs(templateFuncs()).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", name, err)
		}

		// Create template with metadata
		tpl := &Template{
			Name:     name,
			Content:  string(content),
			Template: tmpl,
		}

		// Set metadata based on template name
		r.setTemplateMetadata(tpl)

		r.templates[name] = tpl
	}

	return nil
}

// setTemplateMetadata assigns metadata to a template based on its name.
func (r *Registry) setTemplateMetadata(t *Template) {
	switch t.Name {
	case "base":
		t.DisplayName = "Basic Configuration"
		t.Description = "Minimal configuration with essential settings"
		t.Tags = []string{"minimal", "starter"}

	case "opensource-go":
		t.DisplayName = "Open Source Go Project"
		t.Description = "Configuration for open source Go projects with GitHub releases and Homebrew"
		t.SupportedLanguages = []Language{LanguageGo}
		t.SupportedProjectTypes = []ProjectType{ProjectTypeOpenSource, ProjectTypeCLI, ProjectTypeLibrary}
		t.Tags = []string{"golang", "oss", "homebrew", "github"}

	case "opensource-node":
		t.DisplayName = "Open Source Node.js Project"
		t.Description = "Configuration for open source Node.js projects with npm publishing"
		t.SupportedLanguages = []Language{LanguageNode}
		t.SupportedProjectTypes = []ProjectType{ProjectTypeOpenSource, ProjectTypeLibrary}
		t.Tags = []string{"nodejs", "npm", "oss", "github"}

	case "opensource-python":
		t.DisplayName = "Open Source Python Project"
		t.Description = "Configuration for open source Python projects with PyPI publishing"
		t.SupportedLanguages = []Language{LanguagePython}
		t.SupportedProjectTypes = []ProjectType{ProjectTypeOpenSource, ProjectTypeLibrary}
		t.Tags = []string{"python", "pypi", "oss", "github"}

	case "opensource-rust":
		t.DisplayName = "Open Source Rust Project"
		t.Description = "Configuration for open source Rust projects with Cargo publishing"
		t.SupportedLanguages = []Language{LanguageRust}
		t.SupportedProjectTypes = []ProjectType{ProjectTypeOpenSource, ProjectTypeCLI, ProjectTypeLibrary}
		t.Tags = []string{"rust", "cargo", "oss", "github"}

	case "saas-web":
		t.DisplayName = "SaaS Web Application"
		t.Description = "Configuration for web applications with Docker and Kubernetes deployment"
		t.SupportedLanguages = []Language{LanguageGo, LanguageNode, LanguagePython}
		t.SupportedProjectTypes = []ProjectType{ProjectTypeSaaS}
		t.Tags = []string{"saas", "web", "docker", "kubernetes"}

	case "saas-api":
		t.DisplayName = "SaaS API Service"
		t.Description = "Configuration for API services with container deployment"
		t.SupportedLanguages = []Language{LanguageGo, LanguageNode, LanguagePython}
		t.SupportedProjectTypes = []ProjectType{ProjectTypeAPI, ProjectTypeSaaS}
		t.Tags = []string{"api", "rest", "docker", "kubernetes"}

	case "cli-tool":
		t.DisplayName = "CLI Tool"
		t.Description = "Configuration for command-line tools with multi-platform releases"
		t.SupportedLanguages = []Language{LanguageGo, LanguageRust}
		t.SupportedProjectTypes = []ProjectType{ProjectTypeCLI}
		t.Tags = []string{"cli", "tool", "homebrew", "goreleaser"}

	case "mobile-app":
		t.DisplayName = "Mobile Application"
		t.Description = "Configuration for mobile apps with app store releases"
		t.SupportedProjectTypes = []ProjectType{ProjectTypeMobile}
		t.Tags = []string{"mobile", "ios", "android"}

	case "container":
		t.DisplayName = "Container Application"
		t.Description = "Configuration for containerized applications with registry publishing"
		t.SupportedProjectTypes = []ProjectType{ProjectTypeContainer}
		t.Tags = []string{"docker", "container", "registry"}

	case "monorepo":
		t.DisplayName = "Monorepo"
		t.Description = "Configuration for monorepo with multiple packages"
		t.SupportedLanguages = []Language{LanguageGo, LanguageNode, LanguagePython}
		t.SupportedProjectTypes = []ProjectType{ProjectTypeMonorepo}
		t.Tags = []string{"monorepo", "workspace", "multi-package"}
	}
}

// Get retrieves a template by name.
func (r *Registry) Get(name string) (*Template, error) {
	t, ok := r.templates[name]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", name)
	}
	return t, nil
}

// List returns all available templates, optionally filtered.
func (r *Registry) List(filters ...TemplateFilter) []*Template {
	var result []*Template

	for _, t := range r.templates {
		include := true
		for _, filter := range filters {
			if !filter(t) {
				include = false
				break
			}
		}
		if include {
			result = append(result, t)
		}
	}

	// Sort by display name for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].DisplayName < result[j].DisplayName
	})

	return result
}

// All returns all available templates.
func (r *Registry) All() []*Template {
	return r.List()
}

// TemplateFilter is a function that filters templates.
type TemplateFilter func(*Template) bool

// FilterByLanguage returns templates that support the given language.
func FilterByLanguage(lang Language) TemplateFilter {
	return func(t *Template) bool {
		if len(t.SupportedLanguages) == 0 {
			return true // Templates without language restrictions match all
		}
		for _, supported := range t.SupportedLanguages {
			if supported == lang {
				return true
			}
		}
		return false
	}
}

// FilterByProjectType returns templates that support the given project type.
func FilterByProjectType(projectType ProjectType) TemplateFilter {
	return func(t *Template) bool {
		if len(t.SupportedProjectTypes) == 0 {
			return true // Templates without type restrictions match all
		}
		for _, supported := range t.SupportedProjectTypes {
			if supported == projectType {
				return true
			}
		}
		return false
	}
}

// FilterByTag returns templates that have the given tag.
func FilterByTag(tag string) TemplateFilter {
	return func(t *Template) bool {
		for _, tagValue := range t.Tags {
			if tagValue == tag {
				return true
			}
		}
		return false
	}
}

// SuggestTemplate suggests the best template based on detection results.
func (r *Registry) SuggestTemplate(detection *Detection) *Template {
	// Use the detection's suggested template name first
	if detection.SuggestedTemplate != "" {
		if t, err := r.Get(detection.SuggestedTemplate); err == nil {
			return t
		}
	}

	// Fallback: find best match based on language and project type
	filters := []TemplateFilter{}
	if detection.Language != LanguageUnknown {
		filters = append(filters, FilterByLanguage(detection.Language))
	}
	if detection.ProjectType != ProjectTypeUnknown {
		filters = append(filters, FilterByProjectType(detection.ProjectType))
	}

	matches := r.List(filters...)
	if len(matches) > 0 {
		return matches[0]
	}

	// Ultimate fallback: return base template
	t, _ := r.Get("base")
	return t
}

// Count returns the number of templates in the registry.
func (r *Registry) Count() int {
	return len(r.templates)
}

// Names returns all template names.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.templates))
	for name := range r.templates {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
