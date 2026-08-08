package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsOpenAIKeyFormat(t *testing.T) {
	valid := "sk-" + strings.Repeat("a", 48)
	if !isOpenAIKeyFormat(valid) {
		t.Fatalf("expected %q to be considered OpenAI key format", valid)
	}

	if isOpenAIKeyFormat("${ENV_VAR}") {
		t.Fatal("environment variable references should not be treated as OpenAI key")
	}
}

func TestValidator_AIValidationErrors(t *testing.T) {
	origEnv := os.Getenv("AZURE_OPENAI_ENDPOINT")
	_ = os.Setenv("AZURE_OPENAI_ENDPOINT", "https://example.com")
	t.Cleanup(func() { _ = os.Setenv("AZURE_OPENAI_ENDPOINT", origEnv) })

	cfg := DefaultConfig()
	cfg.AI.Enabled = true
	cfg.AI.Provider = "azure-openai"
	cfg.AI.Model = ""
	cfg.AI.APIKey = "sk-" + strings.Repeat("a", 48)
	cfg.AI.BaseURL = "://invalid"
	cfg.AI.Tone = "loud"
	cfg.AI.Audience = "everyone"
	cfg.AI.MaxTokens = 999999
	cfg.AI.Temperature = 1.5
	cfg.AI.Timeout = 0
	cfg.AI.RetryAttempts = -1

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for AI config")
	}
	for _, substr := range []string{
		"ai.model", "ai.base_url", "ai.tone", "ai.audience", "ai.temperature", "ai.max_tokens", "ai.timeout", "ai.retry_attempts",
	} {
		if !strings.Contains(err.Error(), substr) {
			t.Errorf("expected error message to mention %q, got %q", substr, err.Error())
		}
	}
}

func TestValidator_ChangelogIssues(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Changelog.Format = "custom"
	cfg.Changelog.Template = filepath.Join(t.TempDir(), "missing.md")
	cfg.Changelog.LinkCommits = true
	cfg.Changelog.LinkIssues = true
	cfg.AI.Enabled = false

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for changelog configuration")
	}
	if !strings.Contains(err.Error(), "changelog.template") {
		t.Errorf("expected changelog.template error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "changelog.link_commits") {
		t.Errorf("expected changelog.link_commits error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "changelog.link_issues") {
		t.Errorf("expected changelog.link_issues error, got %q", err.Error())
	}
}

func TestValidator_ChangelogInvalidGroupBy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.Enabled = false
	cfg.Changelog.GroupBy = "invalid"

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for invalid group_by")
	}
	if !strings.Contains(err.Error(), "changelog.group_by") {
		t.Errorf("expected group_by error, got %q", err.Error())
	}
}

func TestValidator_ChangelogValidCustomTemplate(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "template.md")
	if err := os.WriteFile(tmpl, []byte("# Template"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	cfg.AI.Enabled = false
	cfg.Changelog.Format = "custom"
	cfg.Changelog.Template = tmpl

	err := Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error for valid custom template, got %v", err)
	}
}

func TestValidator_ChangelogWithRepoURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.Enabled = false
	cfg.Changelog.LinkCommits = true
	cfg.Changelog.RepositoryURL = "https://github.com/owner/repo"

	err := Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error with valid repo URL, got %v", err)
	}
}

func TestValidator_ChangelogWithIssueURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.Enabled = false
	cfg.Changelog.LinkIssues = true
	cfg.Changelog.IssueURL = "https://github.com/owner/repo/issues"

	err := Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error with valid issue URL, got %v", err)
	}
}

func TestValidator_SlackPluginWithWebhook(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.Enabled = false
	cfg.Plugins = []PluginConfig{
		{Name: "slack", Config: map[string]any{"webhook": "https://hooks.slack.com/services/T00/B00/xxx"}},
	}

	err := Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error with valid slack webhook, got %v", err)
	}
}

func TestValidator_NPMPluginWithRegistry(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.Enabled = false
	cfg.Plugins = []PluginConfig{
		{Name: "npm", Config: map[string]any{"registry": "https://registry.npmjs.org"}},
	}

	err := Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error with valid npm registry, got %v", err)
	}
}

func TestValidator_WorkflowAutoCommitNoMessage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.Enabled = false
	cfg.Workflow.AutoCommitChangelog = true
	cfg.Workflow.ChangelogCommitMessage = ""

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for auto_commit_changelog without message")
	}
	if !strings.Contains(err.Error(), "changelog_commit_message") {
		t.Errorf("expected changelog_commit_message error, got %q", err.Error())
	}
}

func TestValidator_PluginValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.Enabled = false
	cfg.Plugins = []PluginConfig{
		{Name: "", Path: "missing"},
		{Name: "github", Hooks: []string{"invalid_hook"}, Timeout: -1},
		{Name: "github", Hooks: []string{"pre_plan"}},
		{Name: "slack", Config: map[string]any{"webhook": ""}},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for plugins")
	}
	if !strings.Contains(err.Error(), "plugins[0].name") {
		t.Errorf("expected missing name error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "plugins[1].hooks") {
		t.Errorf("expected invalid hook error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "duplicate plugin name") {
		t.Errorf("expected duplicate name error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "plugins[3].config.webhook") {
		t.Errorf("expected slack webhook error, got %q", err.Error())
	}
}

// TestValidateVersionFiles covers the config errors worth catching at load time
// rather than partway through a release (issue #195).
func TestValidateVersionFiles(t *testing.T) {
	tests := []struct {
		name    string
		targets []VersionTarget
		wantErr string
	}{
		{
			name:    "valid semver entry",
			targets: []VersionTarget{{Path: "package.json", Key: "version"}},
		},
		{
			name: "valid multi-format set",
			targets: []VersionTarget{
				{Path: "package.json", Format: VersionFormatSemver, Key: "version"},
				{Path: "manifest.json", Format: VersionFormatSemverBuild, Key: "Version"},
				{Path: "app.json", Format: VersionFormatInteger, Key: "versionCode", Strategy: StrategyIncrement},
			},
		},
		{
			name:    "missing path",
			targets: []VersionTarget{{Key: "version"}},
			wantErr: "path: required",
		},
		{
			name:    "unknown format",
			targets: []VersionTarget{{Path: "f.json", Format: "nonsense", Key: "v"}},
			wantErr: "format: must be one of",
		},
		{
			name:    "unknown strategy",
			targets: []VersionTarget{{Path: "f.json", Strategy: "sideways", Key: "v"}},
			wantErr: "strategy: must be one of",
		},
		{
			name:    "template format without template",
			targets: []VersionTarget{{Path: "f.json", Format: VersionFormatTemplate, Key: "v"}},
			wantErr: "template: required",
		},
		{
			name:    "template without template format",
			targets: []VersionTarget{{Path: "f.json", Key: "v", Template: "${major}"}},
			wantErr: "only applies when format is 'template'",
		},
		{
			name:    "increment without integer format",
			targets: []VersionTarget{{Path: "f.json", Key: "v", Strategy: StrategyIncrement}},
			wantErr: "'increment' requires format 'integer'",
		},
		{
			// Two entries for the same path and key means the second silently wins.
			name: "duplicate path and key",
			targets: []VersionTarget{
				{Path: "f.json", Key: "v"},
				{Path: "f.json", Key: "v"},
			},
			wantErr: "duplicates entry 0",
		},
		{
			// Same file, different keys is legitimate (Chart.yaml version + appVersion).
			name: "same path different keys is allowed",
			targets: []VersionTarget{
				{Path: "Chart.yaml", Key: "version"},
				{Path: "Chart.yaml", Key: "appVersion"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.AI.Enabled = false
			cfg.Versioning.VersionFiles = tt.targets

			err := Validate(cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() should have reported %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// bump_from: file must accept either the deprecated single key or the list.
func TestValidate_BumpFromFileAcceptsEitherForm(t *testing.T) {
	single := DefaultConfig()
	single.AI.Enabled = false
	single.Versioning.BumpFrom = "file"
	single.Versioning.VersionFile = "VERSION"
	if err := Validate(single); err != nil {
		t.Errorf("Validate() with version_file error = %v, want nil", err)
	}

	list := DefaultConfig()
	list.AI.Enabled = false
	list.Versioning.BumpFrom = "file"
	list.Versioning.VersionFiles = []VersionTarget{{Path: "package.json", Key: "version"}}
	if err := Validate(list); err != nil {
		t.Errorf("Validate() with version_files error = %v, want nil", err)
	}

	neither := DefaultConfig()
	neither.AI.Enabled = false
	neither.Versioning.BumpFrom = "file"
	if err := Validate(neither); err == nil {
		t.Error("Validate() should require a version file when bump_from is 'file'")
	}
}
