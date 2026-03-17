package config

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func cleanupEnv(keys ...string) func() {
	original := make(map[string]string)
	for _, key := range keys {
		original[key] = os.Getenv(key)
	}
	return func() {
		for _, key := range keys {
			if val, ok := original[key]; ok && val != "" {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
	}
}

func TestLoaderExpandEnvVar(t *testing.T) {
	cleanup := cleanupEnv("TOKEN_VALUE", "FALLBACK", "PATH_VAR")
	defer cleanup()

	os.Setenv("TOKEN_VALUE", "abc123")
	os.Setenv("FALLBACK", "fallback")

	value := expandEnvVar("prefix-${TOKEN_VALUE}-suffix:$MISSING:${MISSING:-default}:${FALLBACK}")

	if !strings.Contains(value, "abc123") {
		t.Fatalf("expected TOKEN_VALUE to expand, got %q", value)
	}
	if !strings.Contains(value, "default") {
		t.Fatalf("expected default to be used, got %q", value)
	}
	if !strings.Contains(value, "fallback") {
		t.Fatalf("expected FALLBACK to expand, got %q", value)
	}
}

func TestLoaderExpandPluginConfig(t *testing.T) {
	cleanup := cleanupEnv("PLUGIN_TOKEN")
	defer cleanup()

	os.Setenv("PLUGIN_TOKEN", "secret")

	cfg := map[string]any{
		"token": "${PLUGIN_TOKEN}",
		"nested": map[string]any{
			"url": "$PLUGIN_TOKEN",
		},
	}

	expandPluginConfig(cfg)

	if cfg["token"] != "secret" {
		t.Fatalf("expected token to expand, got %v", cfg["token"])
	}
	nested, _ := cfg["nested"].(map[string]any)
	if nested["url"] != "secret" {
		t.Fatalf("expected nested url to expand, got %v", nested["url"])
	}
}

func TestLoaderAutoDetectAISingleProvider(t *testing.T) {
	cleanup := cleanupEnv("OPENAI_API_KEY")
	defer cleanup()

	os.Setenv("OPENAI_API_KEY", "openai-token")

	l := NewLoader()
	l.autoDetectAI()

	if !l.v.GetBool("ai.enabled") {
		t.Fatalf("expected ai.enabled to be true")
	}
	if l.v.GetString("ai.provider") != "openai" {
		t.Fatalf("expected provider openai, got %s", l.v.GetString("ai.provider"))
	}
	if l.v.GetString("ai.api_key") != "${OPENAI_API_KEY}" {
		t.Fatalf("expected api_key placeholder, got %s", l.v.GetString("ai.api_key"))
	}
}

func TestLoaderAutoDetectAIMultipleProvidersWarns(t *testing.T) {
	cleanup := cleanupEnv("OPENAI_API_KEY", "ANTHROPIC_API_KEY")
	defer cleanup()

	os.Setenv("OPENAI_API_KEY", "a")
	os.Setenv("ANTHROPIC_API_KEY", "b")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer r.Close()

	origStderr := os.Stderr
	os.Stderr = w
	defer func() {
		os.Stderr = origStderr
		w.Close()
	}()

	l := NewLoader()
	l.autoDetectAI()
	w.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read stderr: %v", err)
	}

	if !strings.Contains(buf.String(), "Multiple AI provider API keys detected") {
		t.Fatalf("expected warning about multiple providers, got %q", buf.String())
	}
}

func TestLoaderAutoDetectRepositoryURL(t *testing.T) {
	orig := gitRemoteURLFetcher
	t.Cleanup(func() { gitRemoteURLFetcher = orig })

	gitRemoteURLFetcher = func() string {
		return convertToHTTPSURL("git@github.com:relicta-tech/relicta.git")
	}

	l := NewLoader()
	cfg := DefaultConfig()
	l.autoDetectRepositoryURL(cfg)

	if cfg.Changelog.RepositoryURL != "https://github.com/relicta-tech/relicta" {
		t.Fatalf("unexpected repository url: %s", cfg.Changelog.RepositoryURL)
	}
	if !cfg.Changelog.LinkCommits {
		t.Fatalf("expected LinkCommits to be enabled")
	}

	cfg.Changelog.RepositoryURL = "https://example.com"
	cfg.Changelog.LinkCommits = false
	l.autoDetectRepositoryURL(cfg)
	if cfg.Changelog.LinkCommits {
		t.Fatalf("expected LinkCommits to remain false when repository already configured")
	}
}
