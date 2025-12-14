// Package templates provides project detection and template management for the init wizard.
package templates

import (
	"strings"
	"testing"
)

func TestNewBuilder(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	detection := &Detection{
		Language:    LanguageGo,
		ProjectType: ProjectTypeCLI,
	}

	builder := NewBuilder(registry, detection)

	if builder == nil {
		t.Fatal("NewBuilder() returned nil")
	}

	if builder.registry == nil {
		t.Error("builder.registry should not be nil")
	}

	if builder.detection == nil {
		t.Error("builder.detection should not be nil")
	}

	if builder.data == nil {
		t.Error("builder.data should not be nil")
	}

	if builder.data.Custom == nil {
		t.Error("builder.data.Custom map should be initialized")
	}
}

func TestBuilder_WithDetection(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	detection := &Detection{
		Language:       LanguageGo,
		Platform:       PlatformDocker,
		ProjectType:    ProjectTypeCLI,
		GitRepository:  "https://github.com/user/my-project",
		GitBranch:      "main",
		PackageManager: "go mod",
		BuildTool:      "go build",
		IsMonorepo:     true,
		HasDockerfile:  true,
	}

	builder := NewBuilder(registry, detection)
	builder.WithDetection()

	// Verify detection data was populated
	if builder.data.Language != string(LanguageGo) {
		t.Errorf("Language = %q, want %q", builder.data.Language, LanguageGo)
	}

	if builder.data.Platform != string(PlatformDocker) {
		t.Errorf("Platform = %q, want %q", builder.data.Platform, PlatformDocker)
	}

	if builder.data.ProjectType != string(ProjectTypeCLI) {
		t.Errorf("ProjectType = %q, want %q", builder.data.ProjectType, ProjectTypeCLI)
	}

	if builder.data.RepositoryURL != detection.GitRepository {
		t.Errorf("RepositoryURL = %q, want %q", builder.data.RepositoryURL, detection.GitRepository)
	}

	if builder.data.GitBranch != detection.GitBranch {
		t.Errorf("GitBranch = %q, want %q", builder.data.GitBranch, detection.GitBranch)
	}

	if builder.data.PackageManager != detection.PackageManager {
		t.Errorf("PackageManager = %q, want %q", builder.data.PackageManager, detection.PackageManager)
	}

	if builder.data.BuildTool != detection.BuildTool {
		t.Errorf("BuildTool = %q, want %q", builder.data.BuildTool, detection.BuildTool)
	}

	if builder.data.IsMonorepo != detection.IsMonorepo {
		t.Errorf("IsMonorepo = %v, want %v", builder.data.IsMonorepo, detection.IsMonorepo)
	}

	// Verify project name was extracted
	if builder.data.ProjectName != "my-project" {
		t.Errorf("ProjectName = %q, want %q", builder.data.ProjectName, "my-project")
	}

	// Verify plugins were auto-enabled
	if !builder.data.EnableGitHub {
		t.Error("EnableGitHub should be true for github.com repository")
	}

	if !builder.data.EnableHomebrew {
		t.Error("EnableHomebrew should be true for Go CLI project")
	}

	if !builder.data.EnableDocker {
		t.Error("EnableDocker should be true when HasDockerfile is true")
	}
}

func TestBuilder_WithDetection_NilDetection(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	builder := NewBuilder(registry, nil)
	result := builder.WithDetection()

	// Should return the builder unchanged
	if result != builder {
		t.Error("WithDetection() should return the same builder instance")
	}
}

func TestBuilder_SetProjectName(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	builder := NewBuilder(registry, nil)
	result := builder.SetProjectName("test-project")

	if result != builder {
		t.Error("SetProjectName() should return the builder for chaining")
	}

	if builder.data.ProjectName != "test-project" {
		t.Errorf("ProjectName = %q, want %q", builder.data.ProjectName, "test-project")
	}
}

func TestBuilder_SetRepositoryURL(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	builder := NewBuilder(registry, nil)
	testURL := "https://github.com/user/repo"
	result := builder.SetRepositoryURL(testURL)

	if result != builder {
		t.Error("SetRepositoryURL() should return the builder for chaining")
	}

	if builder.data.RepositoryURL != testURL {
		t.Errorf("RepositoryURL = %q, want %q", builder.data.RepositoryURL, testURL)
	}
}

func TestBuilder_SetGitSign(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	builder := NewBuilder(registry, nil)
	result := builder.SetGitSign(true)

	if result != builder {
		t.Error("SetGitSign() should return the builder for chaining")
	}

	if !builder.data.GitSign {
		t.Error("GitSign should be true")
	}
}

func TestBuilder_SetAI(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	builder := NewBuilder(registry, nil)
	result := builder.SetAI(true, "openai", "gpt-4")

	if result != builder {
		t.Error("SetAI() should return the builder for chaining")
	}

	if !builder.data.AIEnabled {
		t.Error("AIEnabled should be true")
	}

	if builder.data.AIProvider != "openai" {
		t.Errorf("AIProvider = %q, want %q", builder.data.AIProvider, "openai")
	}

	if builder.data.AIModel != "gpt-4" {
		t.Errorf("AIModel = %q, want %q", builder.data.AIModel, "gpt-4")
	}
}

func TestBuilder_EnablePlugin(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	tests := []struct {
		name           string
		plugin         string
		config         map[string]string
		checkField     func(*Builder) bool
		checkConfig    func(*Builder) string
		expectedConfig string
	}{
		{
			name:   "GitHub",
			plugin: "github",
			config: nil,
			checkField: func(b *Builder) bool {
				return b.data.EnableGitHub
			},
		},
		{
			name:   "GitLab",
			plugin: "gitlab",
			config: nil,
			checkField: func(b *Builder) bool {
				return b.data.EnableGitLab
			},
		},
		{
			name:   "Slack with webhook",
			plugin: "slack",
			config: map[string]string{"webhook": "https://hooks.slack.com/test"},
			checkField: func(b *Builder) bool {
				return b.data.EnableSlack
			},
			checkConfig: func(b *Builder) string {
				return b.data.SlackWebhook
			},
			expectedConfig: "https://hooks.slack.com/test",
		},
		{
			name:   "Discord with webhook",
			plugin: "discord",
			config: map[string]string{"webhook": "https://discord.com/api/webhooks/test"},
			checkField: func(b *Builder) bool {
				return b.data.EnableDiscord
			},
			checkConfig: func(b *Builder) string {
				return b.data.DiscordWebhook
			},
			expectedConfig: "https://discord.com/api/webhooks/test",
		},
		{
			name:   "Teams with webhook",
			plugin: "teams",
			config: map[string]string{"webhook": "https://outlook.office.com/webhook/test"},
			checkField: func(b *Builder) bool {
				return b.data.EnableTeams
			},
			checkConfig: func(b *Builder) string {
				return b.data.TeamsWebhook
			},
			expectedConfig: "https://outlook.office.com/webhook/test",
		},
		{
			name:   "Jira with project",
			plugin: "jira",
			config: map[string]string{"project": "PROJ"},
			checkField: func(b *Builder) bool {
				return b.data.EnableJira
			},
			checkConfig: func(b *Builder) string {
				return b.data.JiraProject
			},
			expectedConfig: "PROJ",
		},
		{
			name:   "LaunchNotes with API key",
			plugin: "launchnotes",
			config: map[string]string{"api_key": "test-api-key"},
			checkField: func(b *Builder) bool {
				return b.data.EnableLaunchNotes
			},
			checkConfig: func(b *Builder) string {
				return b.data.LaunchNotesAPIKey
			},
			expectedConfig: "test-api-key",
		},
		{
			name:   "Homebrew with tap",
			plugin: "homebrew",
			config: map[string]string{"tap": "user/homebrew-tap"},
			checkField: func(b *Builder) bool {
				return b.data.EnableHomebrew
			},
			checkConfig: func(b *Builder) string {
				return b.data.HomebrewTap
			},
			expectedConfig: "user/homebrew-tap",
		},
		{
			name:   "Docker with registry",
			plugin: "docker",
			config: map[string]string{"registry": "ghcr.io/user"},
			checkField: func(b *Builder) bool {
				return b.data.EnableDocker
			},
			checkConfig: func(b *Builder) string {
				return b.data.DockerRegistry
			},
			expectedConfig: "ghcr.io/user",
		},
		{
			name:   "NPM with registry",
			plugin: "npm",
			config: map[string]string{"registry": "https://registry.npmjs.org"},
			checkField: func(b *Builder) bool {
				return b.data.EnableNPM
			},
			checkConfig: func(b *Builder) string {
				return b.data.NPMRegistry
			},
			expectedConfig: "https://registry.npmjs.org",
		},
		{
			name:   "PyPI",
			plugin: "pypi",
			config: nil,
			checkField: func(b *Builder) bool {
				return b.data.EnablePyPI
			},
		},
		{
			name:   "Cargo",
			plugin: "cargo",
			config: nil,
			checkField: func(b *Builder) bool {
				return b.data.EnableCargo
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewBuilder(registry, nil)
			result := builder.EnablePlugin(tt.plugin, tt.config)

			if result != builder {
				t.Error("EnablePlugin() should return the builder for chaining")
			}

			if !tt.checkField(builder) {
				t.Errorf("Plugin %q should be enabled", tt.plugin)
			}

			if tt.checkConfig != nil {
				actualConfig := tt.checkConfig(builder)
				if actualConfig != tt.expectedConfig {
					t.Errorf("Config = %q, want %q", actualConfig, tt.expectedConfig)
				}
			}
		})
	}
}

func TestBuilder_DisablePlugin(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	plugins := []string{
		"github", "gitlab", "slack", "discord", "teams", "jira",
		"launchnotes", "homebrew", "docker", "npm", "pypi", "cargo",
	}

	for _, plugin := range plugins {
		t.Run(plugin, func(t *testing.T) {
			builder := NewBuilder(registry, nil)

			// First enable the plugin
			builder.EnablePlugin(plugin, nil)

			// Then disable it
			result := builder.DisablePlugin(plugin)

			if result != builder {
				t.Error("DisablePlugin() should return the builder for chaining")
			}

			// Verify it's disabled (all should be false)
			if builder.data.EnableGitHub || builder.data.EnableGitLab ||
				builder.data.EnableSlack || builder.data.EnableDiscord ||
				builder.data.EnableTeams || builder.data.EnableJira ||
				builder.data.EnableLaunchNotes || builder.data.EnableHomebrew ||
				builder.data.EnableDocker || builder.data.EnableNPM ||
				builder.data.EnablePyPI || builder.data.EnableCargo {
				t.Errorf("Plugin %q should be disabled", plugin)
			}
		})
	}
}

func TestBuilder_SetCustom(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	builder := NewBuilder(registry, nil)
	result := builder.SetCustom("custom_field", "custom_value")

	if result != builder {
		t.Error("SetCustom() should return the builder for chaining")
	}

	if builder.data.Custom["custom_field"] != "custom_value" {
		t.Errorf("Custom[custom_field] = %v, want %q", builder.data.Custom["custom_field"], "custom_value")
	}
}

func TestBuilder_AutoEnablePlugins(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	tests := []struct {
		name      string
		detection *Detection
		checkFunc func(*Builder) bool
		want      bool
	}{
		{
			name: "GitHub repository",
			detection: &Detection{
				GitRepository: "https://github.com/user/repo",
			},
			checkFunc: func(b *Builder) bool { return b.data.EnableGitHub },
			want:      true,
		},
		{
			name: "GitLab repository",
			detection: &Detection{
				GitRepository: "https://gitlab.com/user/repo",
			},
			checkFunc: func(b *Builder) bool { return b.data.EnableGitLab },
			want:      true,
		},
		{
			name: "Go CLI enables Homebrew",
			detection: &Detection{
				Language:    LanguageGo,
				ProjectType: ProjectTypeCLI,
			},
			checkFunc: func(b *Builder) bool { return b.data.EnableHomebrew },
			want:      true,
		},
		{
			name: "Node.js enables NPM",
			detection: &Detection{
				Language: LanguageNode,
			},
			checkFunc: func(b *Builder) bool { return b.data.EnableNPM },
			want:      true,
		},
		{
			name: "Python enables PyPI",
			detection: &Detection{
				Language: LanguagePython,
			},
			checkFunc: func(b *Builder) bool { return b.data.EnablePyPI },
			want:      true,
		},
		{
			name: "Rust enables Cargo",
			detection: &Detection{
				Language: LanguageRust,
			},
			checkFunc: func(b *Builder) bool { return b.data.EnableCargo },
			want:      true,
		},
		{
			name: "Dockerfile enables Docker",
			detection: &Detection{
				HasDockerfile: true,
			},
			checkFunc: func(b *Builder) bool { return b.data.EnableDocker },
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewBuilder(registry, tt.detection)
			builder.autoEnablePlugins()

			got := tt.checkFunc(builder)
			if got != tt.want {
				t.Errorf("autoEnablePlugins() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuilder_Build(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	detection := &Detection{
		Language:      LanguageGo,
		ProjectType:   ProjectTypeCLI,
		GitRepository: "https://github.com/user/my-project",
	}

	builder := NewBuilder(registry, detection)
	builder.WithDetection().SetProjectName("test-project")

	config, err := builder.Build("base")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if config == "" {
		t.Error("Build() returned empty config")
	}

	// Verify it's valid YAML by checking it contains expected sections
	if !strings.Contains(config, "versioning:") {
		t.Error("Config should contain 'versioning:' section")
	}

	if !strings.Contains(config, "changelog:") {
		t.Error("Config should contain 'changelog:' section")
	}
}

func TestBuilder_Build_InvalidTemplate(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	builder := NewBuilder(registry, nil)
	_, err = builder.Build("nonexistent-template")

	if err == nil {
		t.Error("Build() should return error for nonexistent template")
	}

	if !strings.Contains(err.Error(), "failed to get template") {
		t.Errorf("Error should mention template not found, got: %v", err)
	}
}

func TestBuilder_BuildWithSuggested(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	detection := &Detection{
		Language:          LanguageGo,
		ProjectType:       ProjectTypeCLI,
		GitRepository:     "https://github.com/user/my-project",
		SuggestedTemplate: "cli-tool",
	}

	builder := NewBuilder(registry, detection)
	builder.WithDetection()

	config, err := builder.BuildWithSuggested()
	if err != nil {
		t.Fatalf("BuildWithSuggested() error = %v", err)
	}

	if config == "" {
		t.Error("BuildWithSuggested() returned empty config")
	}

	// Verify it contains CLI-specific configuration
	if !strings.Contains(config, "versioning:") {
		t.Error("Config should contain 'versioning:' section")
	}
}

func TestBuilder_BuildWithSuggested_NoSuggestion(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	// Create a detection that won't match any template
	detection := &Detection{
		Language:          LanguageUnknown,
		ProjectType:       ProjectTypeUnknown,
		SuggestedTemplate: "nonexistent",
	}

	builder := NewBuilder(registry, detection)

	// Should fall back to base template
	config, err := builder.BuildWithSuggested()
	if err != nil {
		t.Fatalf("BuildWithSuggested() error = %v", err)
	}

	if config == "" {
		t.Error("BuildWithSuggested() should fall back to base template")
	}
}

func TestBuilder_Data(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	builder := NewBuilder(registry, nil)
	builder.SetProjectName("test-project")

	data := builder.Data()

	if data == nil {
		t.Fatal("Data() returned nil")
	}

	if data.ProjectName != "test-project" {
		t.Errorf("Data().ProjectName = %q, want %q", data.ProjectName, "test-project")
	}
}

func TestValidateYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "Valid YAML",
			yaml: `
versioning:
  strategy: conventional
changelog:
  file: CHANGELOG.md
`,
			wantErr: false,
		},
		{
			name:    "Invalid YAML - syntax error",
			yaml:    "key: : invalid",
			wantErr: true,
		},
		{
			name:    "Invalid YAML - unclosed quote",
			yaml:    `key: "unclosed`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateYAML(tt.yaml)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateYAML() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExtractProjectName(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		expected string
	}{
		{
			name:     "HTTPS GitHub URL",
			repoURL:  "https://github.com/user/my-project",
			expected: "my-project",
		},
		{
			name:     "HTTPS GitHub URL with .git",
			repoURL:  "https://github.com/user/my-project.git",
			expected: "my-project",
		},
		{
			name:     "SSH GitHub URL",
			repoURL:  "git@github.com:user/my-project.git",
			expected: "my-project",
		},
		{
			name:     "SSH GitHub URL without .git",
			repoURL:  "git@github.com:user/my-project",
			expected: "my-project",
		},
		{
			name:     "GitLab HTTPS URL",
			repoURL:  "https://gitlab.com/user/awesome-project",
			expected: "awesome-project",
		},
		{
			name:     "Project name with hyphens",
			repoURL:  "https://github.com/user/my-awesome-project",
			expected: "my-awesome-project",
		},
		{
			name:     "Project name with underscores",
			repoURL:  "https://github.com/user/my_awesome_project",
			expected: "my_awesome_project",
		},
		{
			name:     "Project name with dots",
			repoURL:  "https://github.com/user/my.awesome.project",
			expected: "my.awesome.project",
		},
		{
			name:     "Project name with special characters (sanitized)",
			repoURL:  "https://github.com/user/my@special#project",
			expected: "myspecialproject",
		},
		{
			name:     "Empty URL",
			repoURL:  "",
			expected: "my-project",
		},
		{
			name:     "URL with only special characters (sanitized to default)",
			repoURL:  "https://github.com/user/@@@",
			expected: "my-project",
		},
		{
			name:     "Just a dot (sanitized to default)",
			repoURL:  "https://github.com/user/.",
			expected: "my-project",
		},
		{
			name:     "Double dot (sanitized to default)",
			repoURL:  "https://github.com/user/..",
			expected: "my-project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractProjectName(tt.repoURL)
			if got != tt.expected {
				t.Errorf("extractProjectName(%q) = %q, want %q", tt.repoURL, got, tt.expected)
			}
		})
	}
}

func TestBuilder_Chaining(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	detection := &Detection{
		Language:      LanguageGo,
		ProjectType:   ProjectTypeCLI,
		GitRepository: "https://github.com/user/my-project",
	}

	// Test method chaining
	builder := NewBuilder(registry, detection).
		WithDetection().
		SetProjectName("chained-project").
		SetRepositoryURL("https://github.com/user/chained-repo").
		SetGitSign(true).
		SetAI(true, "openai", "gpt-4").
		EnablePlugin("github", nil).
		EnablePlugin("slack", map[string]string{"webhook": "https://hooks.slack.com/test"}).
		SetCustom("custom_key", "custom_value")

	// Verify all methods were applied
	if builder.data.ProjectName != "chained-project" {
		t.Errorf("ProjectName = %q, want %q", builder.data.ProjectName, "chained-project")
	}

	if builder.data.RepositoryURL != "https://github.com/user/chained-repo" {
		t.Errorf("RepositoryURL = %q, want %q", builder.data.RepositoryURL, "https://github.com/user/chained-repo")
	}

	if !builder.data.GitSign {
		t.Error("GitSign should be true")
	}

	if !builder.data.AIEnabled {
		t.Error("AIEnabled should be true")
	}

	if !builder.data.EnableGitHub {
		t.Error("EnableGitHub should be true")
	}

	if !builder.data.EnableSlack {
		t.Error("EnableSlack should be true")
	}

	if builder.data.Custom["custom_key"] != "custom_value" {
		t.Errorf("Custom[custom_key] = %v, want %q", builder.data.Custom["custom_key"], "custom_value")
	}
}
