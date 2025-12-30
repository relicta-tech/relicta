// Package config provides configuration management for Relicta.
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test versioning defaults
	if cfg.Versioning.Strategy != "conventional" {
		t.Errorf("Versioning.Strategy = %v, want conventional", cfg.Versioning.Strategy)
	}
	if cfg.Versioning.TagPrefix != "v" {
		t.Errorf("Versioning.TagPrefix = %v, want v", cfg.Versioning.TagPrefix)
	}
	if !cfg.Versioning.GitTag {
		t.Error("Versioning.GitTag should be true by default")
	}
	if !cfg.Versioning.GitPush {
		t.Error("Versioning.GitPush should be true by default")
	}
	if cfg.Versioning.BumpFrom != "tag" {
		t.Errorf("Versioning.BumpFrom = %v, want tag", cfg.Versioning.BumpFrom)
	}

	// Test changelog defaults
	if cfg.Changelog.File != "CHANGELOG.md" {
		t.Errorf("Changelog.File = %v, want CHANGELOG.md", cfg.Changelog.File)
	}
	if cfg.Changelog.Format != "keep-a-changelog" {
		t.Errorf("Changelog.Format = %v, want keep-a-changelog", cfg.Changelog.Format)
	}
	if cfg.Changelog.GroupBy != "type" {
		t.Errorf("Changelog.GroupBy = %v, want type", cfg.Changelog.GroupBy)
	}

	// Test AI defaults
	if cfg.AI.Enabled {
		t.Error("AI.Enabled should be false by default")
	}
	if cfg.AI.Provider != "openai" {
		t.Errorf("AI.Provider = %v, want openai", cfg.AI.Provider)
	}
	if cfg.AI.Model != "gpt-4" {
		t.Errorf("AI.Model = %v, want gpt-4", cfg.AI.Model)
	}
	if cfg.AI.Temperature != 0.7 {
		t.Errorf("AI.Temperature = %v, want 0.7", cfg.AI.Temperature)
	}

	// Test workflow defaults
	if !cfg.Workflow.RequireApproval {
		t.Error("Workflow.RequireApproval should be true by default")
	}
	if len(cfg.Workflow.AllowedBranches) != 2 {
		t.Errorf("Workflow.AllowedBranches length = %d, want 2", len(cfg.Workflow.AllowedBranches))
	}

	// Test output defaults
	if cfg.Output.Format != "text" {
		t.Errorf("Output.Format = %v, want text", cfg.Output.Format)
	}
	if !cfg.Output.Color {
		t.Error("Output.Color should be true by default")
	}
	if cfg.Output.LogLevel != "info" {
		t.Errorf("Output.LogLevel = %v, want info", cfg.Output.LogLevel)
	}
}

func TestPluginConfig_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  *bool
		expected bool
	}{
		{
			name:     "nil (default enabled)",
			enabled:  nil,
			expected: true,
		},
		{
			name:     "explicitly enabled",
			enabled:  boolPtr(true),
			expected: true,
		},
		{
			name:     "explicitly disabled",
			enabled:  boolPtr(false),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := PluginConfig{Enabled: tt.enabled}
			if got := p.IsEnabled(); got != tt.expected {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func TestValidationError(t *testing.T) {
	ve := &ValidationError{}

	if ve.HasErrors() {
		t.Error("New ValidationError should not have errors")
	}

	ve.Addf("error %d", 1)
	ve.Addf("error %d", 2)

	if !ve.HasErrors() {
		t.Error("ValidationError should have errors after Add")
	}

	errStr := ve.Error()
	if !strings.Contains(errStr, "error 1") {
		t.Errorf("Error() should contain 'error 1', got %v", errStr)
	}
	if !strings.Contains(errStr, "error 2") {
		t.Errorf("Error() should contain 'error 2', got %v", errStr)
	}
}

func TestValidator_Validate_ValidConfig(t *testing.T) {
	cfg := DefaultConfig()
	// Disable AI to avoid API key requirement
	cfg.AI.Enabled = false

	err := Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestValidator_Validate_InvalidStrategy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Versioning.Strategy = "invalid"
	cfg.AI.Enabled = false

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should return error for invalid strategy")
	}
	if !strings.Contains(err.Error(), "versioning.strategy") {
		t.Errorf("Error should mention versioning.strategy, got: %v", err)
	}
}

func TestValidator_Validate_InvalidBumpFrom(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Versioning.BumpFrom = "invalid"
	cfg.AI.Enabled = false

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should return error for invalid bump_from")
	}
	if !strings.Contains(err.Error(), "versioning.bump_from") {
		t.Errorf("Error should mention versioning.bump_from, got: %v", err)
	}
}

func TestValidator_Validate_FileVersionWithoutPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Versioning.BumpFrom = "file"
	cfg.Versioning.VersionFile = ""
	cfg.AI.Enabled = false

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should require version_file when bump_from is file")
	}
	if !strings.Contains(err.Error(), "version_file") {
		t.Errorf("Error should mention version_file, got: %v", err)
	}
}

func TestValidator_Validate_InvalidChangelogFormat(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Changelog.Format = "invalid"
	cfg.AI.Enabled = false

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should return error for invalid changelog format")
	}
	if !strings.Contains(err.Error(), "changelog.format") {
		t.Errorf("Error should mention changelog.format, got: %v", err)
	}
}

func TestValidator_Validate_CustomFormatWithoutTemplate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Changelog.Format = "custom"
	cfg.Changelog.Template = ""
	cfg.AI.Enabled = false

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should require template when format is custom")
	}
	if !strings.Contains(err.Error(), "changelog.template") {
		t.Errorf("Error should mention changelog.template, got: %v", err)
	}
}

func TestValidator_Validate_InvalidOutputFormat(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Output.Format = "invalid"
	cfg.AI.Enabled = false

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should return error for invalid output format")
	}
	if !strings.Contains(err.Error(), "output.format") {
		t.Errorf("Error should mention output.format, got: %v", err)
	}
}

func TestValidator_Validate_InvalidLogLevel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Output.LogLevel = "invalid"
	cfg.AI.Enabled = false

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should return error for invalid log level")
	}
	if !strings.Contains(err.Error(), "output.log_level") {
		t.Errorf("Error should mention output.log_level, got: %v", err)
	}
}

func TestValidator_Validate_QuietAndVerbose(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Output.Quiet = true
	cfg.Output.Verbose = true
	cfg.AI.Enabled = false

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should reject quiet and verbose together")
	}
	if !strings.Contains(err.Error(), "quiet and verbose") {
		t.Errorf("Error should mention quiet and verbose, got: %v", err)
	}
}

func TestValidator_Validate_AIEnabled_RequiresAPIKey(t *testing.T) {
	// Clear any existing API key env vars
	origOpenAI := os.Getenv("OPENAI_API_KEY")
	origRP := os.Getenv("RELICTA_AI_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("RELICTA_AI_API_KEY")
	defer func() {
		if origOpenAI != "" {
			os.Setenv("OPENAI_API_KEY", origOpenAI)
		}
		if origRP != "" {
			os.Setenv("RELICTA_AI_API_KEY", origRP)
		}
	}()

	cfg := DefaultConfig()
	cfg.AI.Enabled = true
	cfg.AI.APIKey = ""

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should require API key when AI is enabled")
	}
	if !strings.Contains(err.Error(), "ai.api_key") {
		t.Errorf("Error should mention ai.api_key, got: %v", err)
	}
}

func TestValidator_Validate_InvalidAITone(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.Enabled = true
	cfg.AI.APIKey = "test-key"
	cfg.AI.Tone = "invalid"

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should return error for invalid AI tone")
	}
	if !strings.Contains(err.Error(), "ai.tone") {
		t.Errorf("Error should mention ai.tone, got: %v", err)
	}
}

func TestValidator_Validate_InvalidAIAudience(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.Enabled = true
	cfg.AI.APIKey = "test-key"
	cfg.AI.Audience = "invalid"

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should return error for invalid AI audience")
	}
	if !strings.Contains(err.Error(), "ai.audience") {
		t.Errorf("Error should mention ai.audience, got: %v", err)
	}
}

func TestValidator_Validate_InvalidAITemperature(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.Enabled = true
	cfg.AI.APIKey = "test-key"
	cfg.AI.Temperature = 3.0 // Out of range

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should return error for invalid AI temperature")
	}
	if !strings.Contains(err.Error(), "ai.temperature") {
		t.Errorf("Error should mention ai.temperature, got: %v", err)
	}
}

func TestValidator_Validate_DuplicatePluginNames(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.Enabled = false
	cfg.Plugins = []PluginConfig{
		{Name: "github"},
		{Name: "github"}, // Duplicate
	}

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should reject duplicate plugin names")
	}
	if !strings.Contains(err.Error(), "duplicate plugin name") {
		t.Errorf("Error should mention duplicate plugin name, got: %v", err)
	}
}

func TestValidator_Validate_EmptyPluginName(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.Enabled = false
	cfg.Plugins = []PluginConfig{
		{Name: ""},
	}

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should reject empty plugin name")
	}
	if !strings.Contains(err.Error(), "name: required") {
		t.Errorf("Error should mention name required, got: %v", err)
	}
}

func TestValidator_Validate_InvalidPluginHook(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.Enabled = false
	cfg.Plugins = []PluginConfig{
		{
			Name:  "test",
			Hooks: []string{"invalid_hook"},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should reject invalid plugin hook")
	}
	if !strings.Contains(err.Error(), "invalid hook") {
		t.Errorf("Error should mention invalid hook, got: %v", err)
	}
}

func TestValidator_Validate_AutoCommitWithoutMessage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AI.Enabled = false
	cfg.Workflow.AutoCommitChangelog = true
	cfg.Workflow.ChangelogCommitMessage = ""

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should require commit message when auto_commit_changelog is enabled")
	}
	if !strings.Contains(err.Error(), "changelog_commit_message") {
		t.Errorf("Error should mention changelog_commit_message, got: %v", err)
	}
}

func TestExpandEnvVar(t *testing.T) {
	// Set test env vars
	os.Setenv("TEST_VAR", "test_value")
	os.Setenv("ANOTHER_VAR", "another_value")
	defer func() {
		os.Unsetenv("TEST_VAR")
		os.Unsetenv("ANOTHER_VAR")
	}()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no variables",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "${VAR} syntax",
			input:    "${TEST_VAR}",
			expected: "test_value",
		},
		{
			name:     "$VAR syntax",
			input:    "$TEST_VAR",
			expected: "test_value",
		},
		{
			name:     "${VAR:-default} with existing var",
			input:    "${TEST_VAR:-default}",
			expected: "test_value",
		},
		{
			name:     "${VAR:-default} with missing var",
			input:    "${MISSING_VAR:-default_value}",
			expected: "default_value",
		},
		{
			name:     "multiple variables",
			input:    "${TEST_VAR}/${ANOTHER_VAR}",
			expected: "test_value/another_value",
		},
		{
			name:     "mixed text and variables",
			input:    "prefix_${TEST_VAR}_suffix",
			expected: "prefix_test_value_suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandEnvVar(tt.input)
			if result != tt.expected {
				t.Errorf("expandEnvVar(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExpandPluginConfig(t *testing.T) {
	os.Setenv("TEST_TOKEN", "secret_token")
	defer os.Unsetenv("TEST_TOKEN")

	config := map[string]any{
		"token":  "${TEST_TOKEN}",
		"plain":  "plain_value",
		"number": 123,
		"nested": map[string]any{
			"nested_token": "${TEST_TOKEN}",
		},
	}

	expandPluginConfig(config)

	if config["token"] != "secret_token" {
		t.Errorf("token = %v, want secret_token", config["token"])
	}
	if config["plain"] != "plain_value" {
		t.Errorf("plain = %v, want plain_value", config["plain"])
	}
	if config["number"] != 123 {
		t.Errorf("number = %v, want 123", config["number"])
	}

	nested := config["nested"].(map[string]any)
	if nested["nested_token"] != "secret_token" {
		t.Errorf("nested.nested_token = %v, want secret_token", nested["nested_token"])
	}
}

func TestExpandPluginConfig_Nil(t *testing.T) {
	// Should not panic
	expandPluginConfig(nil)
}

func TestLoader_NewLoader(t *testing.T) {
	loader := NewLoader()
	if loader == nil {
		t.Fatal("NewLoader() returned nil")
	}
	if loader.v == nil {
		t.Error("Loader.v is nil")
	}
	if len(loader.searchPaths) != 1 {
		t.Errorf("searchPaths length = %d, want 1", len(loader.searchPaths))
	}
}

func TestLoader_WithConfigPath(t *testing.T) {
	loader := NewLoader().WithConfigPath("/some/path/config.yaml")
	if loader.configPath != "/some/path/config.yaml" {
		t.Errorf("configPath = %v, want /some/path/config.yaml", loader.configPath)
	}
}

func TestLoader_WithSearchPaths(t *testing.T) {
	loader := NewLoader().WithSearchPaths("/path1", "/path2")
	if len(loader.searchPaths) != 3 { // "." + 2 new paths
		t.Errorf("searchPaths length = %d, want 3", len(loader.searchPaths))
	}
}

func TestLoader_Load_WithDefaults(t *testing.T) {
	// Load from empty directory (no config file)
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	loader := NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should have default values
	if cfg.Versioning.Strategy != "conventional" {
		t.Errorf("Strategy = %v, want conventional", cfg.Versioning.Strategy)
	}
}

func TestLoader_Load_WithConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config file
	configContent := `
versioning:
  strategy: manual
  tag_prefix: "release-"
changelog:
  file: HISTORY.md
`
	configPath := filepath.Join(tmpDir, "relicta.config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader().WithConfigPath(configPath)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Versioning.Strategy != "manual" {
		t.Errorf("Strategy = %v, want manual", cfg.Versioning.Strategy)
	}
	if cfg.Versioning.TagPrefix != "release-" {
		t.Errorf("TagPrefix = %v, want release-", cfg.Versioning.TagPrefix)
	}
	if cfg.Changelog.File != "HISTORY.md" {
		t.Errorf("Changelog.File = %v, want HISTORY.md", cfg.Changelog.File)
	}
}

func TestFindConfigFile_Found(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config file (.relicta.yaml is the only supported format)
	configPath := filepath.Join(tmpDir, ".relicta.yaml")
	err := os.WriteFile(configPath, []byte("versioning:\n  strategy: conventional"), 0600)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	found, err := FindConfigFile(tmpDir)
	if err != nil {
		t.Fatalf("FindConfigFile() error = %v", err)
	}
	if found != configPath {
		t.Errorf("FindConfigFile() = %v, want %v", found, configPath)
	}
}

func TestFindConfigFile_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := FindConfigFile(tmpDir)
	if err == nil {
		t.Error("FindConfigFile() should return error when no config found")
	}
}

func TestConfigExists(t *testing.T) {
	tmpDir := t.TempDir()

	// No config file
	if ConfigExists(tmpDir) {
		t.Error("ConfigExists() should return false when no config")
	}

	// Create a config file (.relicta.yaml is the only supported format)
	configPath := filepath.Join(tmpDir, ".relicta.yaml")
	err := os.WriteFile(configPath, []byte("versioning:\n  strategy: conventional"), 0600)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	if !ConfigExists(tmpDir) {
		t.Error("ConfigExists() should return true when config exists")
	}
}

func TestLoadFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config file
	configContent := `
versioning:
  strategy: manual
`
	configPath := filepath.Join(tmpDir, "test-config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if cfg.Versioning.Strategy != "manual" {
		t.Errorf("Strategy = %v, want manual", cfg.Versioning.Strategy)
	}
}

func TestLoadFromDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a config file (.relicta.yaml is the only supported format)
	configContent := `
versioning:
  strategy: manual
`
	configPath := filepath.Join(tmpDir, ".relicta.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory() error = %v", err)
	}

	if cfg.Versioning.Strategy != "manual" {
		t.Errorf("Strategy = %v, want manual", cfg.Versioning.Strategy)
	}
}

func TestWriteConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "output-config.yaml")

	cfg := DefaultConfig()
	cfg.Versioning.Strategy = "manual"
	cfg.Versioning.TagPrefix = "test-"

	err := WriteConfig(cfg, configPath)
	if err != nil {
		t.Fatalf("WriteConfig() error = %v", err)
	}

	// Verify file was written
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("WriteConfig() did not create file")
	}

	// Load it back
	loadedCfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if loadedCfg.Versioning.Strategy != "manual" {
		t.Errorf("Loaded Strategy = %v, want manual", loadedCfg.Versioning.Strategy)
	}
}

func TestWriteDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "default-config.yaml")

	err := WriteDefaultConfig(configPath)
	if err != nil {
		t.Fatalf("WriteDefaultConfig() error = %v", err)
	}

	// Verify file was written
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("WriteDefaultConfig() did not create file")
	}
}

func TestConfigFileNames(t *testing.T) {
	// Only .relicta is supported (Go ecosystem convention)
	expectedNames := []string{".relicta"}

	if len(ConfigFileNames) != len(expectedNames) {
		t.Errorf("ConfigFileNames length = %d, want %d", len(ConfigFileNames), len(expectedNames))
	}

	for _, expected := range expectedNames {
		found := false
		for _, name := range ConfigFileNames {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ConfigFileNames missing: %s", expected)
		}
	}
}

func TestConfigFileExtensions(t *testing.T) {
	expectedExtensions := []string{"yaml", "yml", "json", "toml"}

	if len(ConfigFileExtensions) != len(expectedExtensions) {
		t.Errorf("ConfigFileExtensions length = %d, want %d", len(ConfigFileExtensions), len(expectedExtensions))
	}

	for _, expected := range expectedExtensions {
		found := false
		for _, ext := range ConfigFileExtensions {
			if ext == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ConfigFileExtensions missing: %s", expected)
		}
	}
}

func TestAIConfig_Defaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.AI.MaxTokens != 2048 {
		t.Errorf("AI.MaxTokens = %d, want 2048", cfg.AI.MaxTokens)
	}
	if cfg.AI.Timeout != 30*time.Second {
		t.Errorf("AI.Timeout = %v, want 30s", cfg.AI.Timeout)
	}
	if cfg.AI.RetryAttempts != 3 {
		t.Errorf("AI.RetryAttempts = %d, want 3", cfg.AI.RetryAttempts)
	}
}

func TestChangelogConfig_DefaultCategories(t *testing.T) {
	cfg := DefaultConfig()

	expectedCategories := map[string]string{
		"feat":     "Features",
		"fix":      "Bug Fixes",
		"perf":     "Performance Improvements",
		"refactor": "Code Refactoring",
		"revert":   "Reverts",
		"build":    "Build System",
	}

	for key, expected := range expectedCategories {
		if cfg.Changelog.Categories[key] != expected {
			t.Errorf("Categories[%s] = %v, want %v", key, cfg.Changelog.Categories[key], expected)
		}
	}
}

func TestChangelogConfig_DefaultExcludes(t *testing.T) {
	cfg := DefaultConfig()

	expectedExcludes := []string{"chore", "ci", "docs", "style", "test"}

	if len(cfg.Changelog.Exclude) != len(expectedExcludes) {
		t.Errorf("Exclude length = %d, want %d", len(cfg.Changelog.Exclude), len(expectedExcludes))
	}
}

func TestValidateAndLoad_NoConfigFile(t *testing.T) {
	// Run in temp directory without config
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	cfg, err := ValidateAndLoad()
	if err != nil {
		t.Fatalf("ValidateAndLoad() error = %v", err)
	}
	if cfg == nil {
		t.Error("ValidateAndLoad() returned nil config")
	}
}

func TestValidator_Validate_SlackPluginWithoutWebhook(t *testing.T) {
	// Clear webhook env var
	origWebhook := os.Getenv("SLACK_WEBHOOK_URL")
	os.Unsetenv("SLACK_WEBHOOK_URL")
	defer func() {
		if origWebhook != "" {
			os.Setenv("SLACK_WEBHOOK_URL", origWebhook)
		}
	}()

	cfg := DefaultConfig()
	cfg.AI.Enabled = false
	cfg.Plugins = []PluginConfig{
		{
			Name:   "slack",
			Config: map[string]any{},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should require webhook for Slack plugin")
	}
	if !strings.Contains(err.Error(), "webhook") {
		t.Errorf("Error should mention webhook, got: %v", err)
	}
}

func TestConvertToHTTPSURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "SSH URL with .git suffix",
			input:    "git@github.com:owner/repo.git",
			expected: "https://github.com/owner/repo",
		},
		{
			name:     "SSH URL without .git suffix",
			input:    "git@github.com:owner/repo",
			expected: "https://github.com/owner/repo",
		},
		{
			name:     "SSH URL with nested path",
			input:    "git@gitlab.com:group/subgroup/repo.git",
			expected: "https://gitlab.com/group/subgroup/repo",
		},
		{
			name:     "HTTPS URL with .git suffix",
			input:    "https://github.com/owner/repo.git",
			expected: "https://github.com/owner/repo",
		},
		{
			name:     "HTTPS URL without .git suffix",
			input:    "https://github.com/owner/repo",
			expected: "https://github.com/owner/repo",
		},
		{
			name:     "HTTP URL",
			input:    "http://github.com/owner/repo.git",
			expected: "https://github.com/owner/repo",
		},
		{
			name:     "Unknown format returned as-is",
			input:    "file:///path/to/repo",
			expected: "file:///path/to/repo",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToHTTPSURL(tt.input)
			if result != tt.expected {
				t.Errorf("convertToHTTPSURL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDefaultConfig_LinkingDisabledByDefault(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Changelog.LinkCommits {
		t.Error("Changelog.LinkCommits should be false by default")
	}
	if cfg.Changelog.LinkIssues {
		t.Error("Changelog.LinkIssues should be false by default")
	}
}

func TestAutoDetectRepositoryURL(t *testing.T) {
	// Test that auto-detection doesn't override explicit config
	loader := NewLoader()
	cfg := &Config{
		Changelog: ChangelogConfig{
			RepositoryURL: "https://example.com/explicit/repo",
			LinkCommits:   false,
		},
	}

	loader.autoDetectRepositoryURL(cfg)

	// Should not change explicitly set URL
	if cfg.Changelog.RepositoryURL != "https://example.com/explicit/repo" {
		t.Errorf("RepositoryURL = %q, should not change when explicitly set", cfg.Changelog.RepositoryURL)
	}
}
