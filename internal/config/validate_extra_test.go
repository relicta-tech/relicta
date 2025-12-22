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
