// Package templates provides project detection and template management for the init wizard.
package templates

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Pre-compiled regex patterns for performance
var (
	// projectNameSanitizeRegex removes characters not allowed in project names.
	// Only allows: alphanumeric, hyphen, underscore, and dot.
	// This prevents template injection and special character issues.
	projectNameSanitizeRegex = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)
)

// TemplateData holds all data available to templates during rendering.
type TemplateData struct {
	// Project information
	ProjectName   string
	RepositoryURL string
	GitBranch     string

	// Language and platform
	Language       string
	Platform       string
	ProjectType    string
	PackageManager string
	BuildTool      string
	IsMonorepo     bool

	// AI configuration
	AIEnabled  bool
	AIProvider string
	AIModel    string

	// Git configuration
	GitSign bool

	// Plugin enablement
	EnableGitHub      bool
	EnableGitLab      bool
	EnableSlack       bool
	EnableDiscord     bool
	EnableTeams       bool
	EnableJira        bool
	EnableLaunchNotes bool
	EnableHomebrew    bool
	EnableDocker      bool
	EnableNPM         bool
	EnablePyPI        bool
	EnableCargo       bool

	// Plugin-specific configuration
	HomebrewTap       string
	DockerRegistry    string
	NPMRegistry       string
	SlackWebhook      string
	DiscordWebhook    string
	TeamsWebhook      string
	JiraProject       string
	LaunchNotesAPIKey string

	// Custom fields for advanced users
	Custom map[string]interface{}
}

// Builder constructs configuration files from templates and user data.
type Builder struct {
	registry  *Registry
	detection *Detection
	data      *TemplateData
}

// NewBuilder creates a new configuration builder.
func NewBuilder(registry *Registry, detection *Detection) *Builder {
	return &Builder{
		registry:  registry,
		detection: detection,
		data: &TemplateData{
			Custom: make(map[string]interface{}),
		},
	}
}

// WithDetection populates template data from detection results.
func (b *Builder) WithDetection() *Builder {
	if b.detection == nil {
		return b
	}

	// Project information
	b.data.RepositoryURL = b.detection.GitRepository
	b.data.GitBranch = b.detection.GitBranch

	// Language and platform
	b.data.Language = string(b.detection.Language)
	b.data.Platform = string(b.detection.Platform)
	b.data.ProjectType = string(b.detection.ProjectType)
	b.data.PackageManager = b.detection.PackageManager
	b.data.BuildTool = b.detection.BuildTool
	b.data.IsMonorepo = b.detection.IsMonorepo

	// Auto-detect project name from repository URL
	if b.detection.GitRepository != "" {
		b.data.ProjectName = extractProjectName(b.detection.GitRepository)
	}

	// Enable plugins based on detection
	b.autoEnablePlugins()

	return b
}

// SetProjectName sets the project name.
func (b *Builder) SetProjectName(name string) *Builder {
	b.data.ProjectName = name
	return b
}

// SetRepositoryURL sets the repository URL.
func (b *Builder) SetRepositoryURL(url string) *Builder {
	b.data.RepositoryURL = url
	return b
}

// SetGitSign sets whether to sign git tags.
func (b *Builder) SetGitSign(sign bool) *Builder {
	b.data.GitSign = sign
	return b
}

// SetAI configures AI settings.
func (b *Builder) SetAI(enabled bool, provider, model string) *Builder {
	b.data.AIEnabled = enabled
	b.data.AIProvider = provider
	b.data.AIModel = model
	return b
}

// EnablePlugin enables a specific plugin.
func (b *Builder) EnablePlugin(name string, config map[string]string) *Builder {
	switch strings.ToLower(name) {
	case "github":
		b.data.EnableGitHub = true
	case "gitlab":
		b.data.EnableGitLab = true
	case "slack":
		b.data.EnableSlack = true
		if webhook, ok := config["webhook"]; ok {
			b.data.SlackWebhook = webhook
		}
	case "discord":
		b.data.EnableDiscord = true
		if webhook, ok := config["webhook"]; ok {
			b.data.DiscordWebhook = webhook
		}
	case "teams":
		b.data.EnableTeams = true
		if webhook, ok := config["webhook"]; ok {
			b.data.TeamsWebhook = webhook
		}
	case "jira":
		b.data.EnableJira = true
		if project, ok := config["project"]; ok {
			b.data.JiraProject = project
		}
	case "launchnotes":
		b.data.EnableLaunchNotes = true
		if apiKey, ok := config["api_key"]; ok {
			b.data.LaunchNotesAPIKey = apiKey
		}
	case "homebrew":
		b.data.EnableHomebrew = true
		if tap, ok := config["tap"]; ok {
			b.data.HomebrewTap = tap
		}
	case "docker":
		b.data.EnableDocker = true
		if registry, ok := config["registry"]; ok {
			b.data.DockerRegistry = registry
		}
	case "npm":
		b.data.EnableNPM = true
		if registry, ok := config["registry"]; ok {
			b.data.NPMRegistry = registry
		}
	case "pypi":
		b.data.EnablePyPI = true
	case "cargo":
		b.data.EnableCargo = true
	}
	return b
}

// DisablePlugin disables a specific plugin.
func (b *Builder) DisablePlugin(name string) *Builder {
	switch strings.ToLower(name) {
	case "github":
		b.data.EnableGitHub = false
	case "gitlab":
		b.data.EnableGitLab = false
	case "slack":
		b.data.EnableSlack = false
	case "discord":
		b.data.EnableDiscord = false
	case "teams":
		b.data.EnableTeams = false
	case "jira":
		b.data.EnableJira = false
	case "launchnotes":
		b.data.EnableLaunchNotes = false
	case "homebrew":
		b.data.EnableHomebrew = false
	case "docker":
		b.data.EnableDocker = false
	case "npm":
		b.data.EnableNPM = false
	case "pypi":
		b.data.EnablePyPI = false
	case "cargo":
		b.data.EnableCargo = false
	}
	return b
}

// SetCustom sets a custom template variable.
func (b *Builder) SetCustom(key string, value interface{}) *Builder {
	b.data.Custom[key] = value
	return b
}

// autoEnablePlugins enables plugins based on detection results.
func (b *Builder) autoEnablePlugins() {
	if b.detection == nil {
		return
	}

	// Enable version control plugin
	if strings.Contains(b.detection.GitRepository, "github.com") {
		b.data.EnableGitHub = true
	} else if strings.Contains(b.detection.GitRepository, "gitlab.com") {
		b.data.EnableGitLab = true
	}

	// Enable language-specific package managers
	switch b.detection.Language {
	case LanguageGo:
		// Homebrew for Go CLI tools
		if b.detection.ProjectType == ProjectTypeCLI {
			b.data.EnableHomebrew = true
		}
	case LanguageNode:
		b.data.EnableNPM = true
	case LanguagePython:
		b.data.EnablePyPI = true
	case LanguageRust:
		b.data.EnableCargo = true
	}

	// Enable Docker for containerized projects
	if b.detection.HasDockerfile {
		b.data.EnableDocker = true
	}
}

// Build renders the template and returns the configuration YAML.
func (b *Builder) Build(templateName string) (string, error) {
	// Get the template
	tmpl, err := b.registry.Get(templateName)
	if err != nil {
		return "", fmt.Errorf("failed to get template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Template.Execute(&buf, b.data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	// Validate YAML
	config := buf.String()
	if err := validateYAML(config); err != nil {
		return "", fmt.Errorf("generated invalid YAML: %w", err)
	}

	return config, nil
}

// BuildWithSuggested builds configuration using the suggested template.
func (b *Builder) BuildWithSuggested() (string, error) {
	suggested := b.registry.SuggestTemplate(b.detection)
	if suggested == nil {
		return "", fmt.Errorf("no suitable template found")
	}
	return b.Build(suggested.Name)
}

// Data returns the current template data for inspection.
func (b *Builder) Data() *TemplateData {
	return b.data
}

// validateYAML checks if the generated config is valid YAML.
func validateYAML(content string) error {
	var data interface{}
	if err := yaml.Unmarshal([]byte(content), &data); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return nil
}

// extractProjectName extracts the project name from a git repository URL.
// Sanitizes the name to prevent template injection attacks.
func extractProjectName(repoURL string) string {
	// Remove .git suffix
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// Handle SSH URLs: git@github.com:user/repo
	if strings.HasPrefix(repoURL, "git@") {
		parts := strings.Split(repoURL, ":")
		if len(parts) >= 2 {
			repoURL = parts[1]
		}
	}

	// Handle HTTPS URLs: https://github.com/user/repo
	if strings.Contains(repoURL, "://") {
		parts := strings.Split(repoURL, "://")
		if len(parts) >= 2 {
			repoURL = parts[1]
		}
	}

	// Extract last path component
	parts := strings.Split(repoURL, "/")
	var name string
	if len(parts) > 0 {
		name = parts[len(parts)-1]
	} else {
		name = "my-project"
	}

	// Sanitize using pre-compiled regex
	sanitized := projectNameSanitizeRegex.ReplaceAllString(name, "")

	// If sanitization removed everything, use default
	if sanitized == "" || sanitized == "." || sanitized == ".." {
		return "my-project"
	}

	return sanitized
}
