package config

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
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

func TestLoaderAutoDetectAIMultipleProvidersRecorded(t *testing.T) {
	cleanup := cleanupEnv("OPENAI_API_KEY", "ANTHROPIC_API_KEY")
	defer cleanup()

	os.Setenv("OPENAI_API_KEY", "a")
	os.Setenv("ANTHROPIC_API_KEY", "b")

	// Detection must be silent (issue #127: the warning used to print on
	// every command, even ones that never touch AI) and instead recorded
	// for AI-using callers to surface.
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

	if buf.String() != "" {
		t.Fatalf("expected silent auto-detection, got stderr output %q", buf.String())
	}
	if l.autoSelectedProvider != "openai" {
		t.Fatalf("expected auto-selected provider 'openai', got %q", l.autoSelectedProvider)
	}
	if len(l.detectedProviders) != 2 {
		t.Fatalf("expected 2 detected providers, got %v", l.detectedProviders)
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

// TestLoad_FindsConfigFromSubdirectory ensures a run from anywhere inside a
// repository picks up the repository's own config rather than silently falling
// back to built-in defaults. The defaults enable versioning.git_push, so a
// project that had disabled pushing would otherwise get a tag pushed purely
// because of the directory the command ran in.
func TestLoad_FindsConfigFromSubdirectory(t *testing.T) {
	repoRoot := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(repoRoot); err == nil {
		repoRoot = resolved
	}

	// Mark the repository root and give it a config that departs from defaults.
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o750); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	configBody := "versioning:\n  tag_prefix: \"rel-\"\n  git_push: false\n"
	if err := os.WriteFile(filepath.Join(repoRoot, ".relicta.yaml"), []byte(configBody), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	nested := filepath.Join(repoRoot, "services", "api")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatalf("MkdirAll(nested) error = %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(nested); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	loader := NewLoader()
	if !loader.configFileExists() {
		t.Error("configFileExists() = false, want true for a config at the repository root")
	}

	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Versioning.TagPrefix != "rel-" {
		t.Errorf("TagPrefix = %q, want %q (config at repo root was not loaded)",
			cfg.Versioning.TagPrefix, "rel-")
	}
	if cfg.Versioning.GitPush {
		t.Error("GitPush = true, want false; the repository config was ignored in favor of defaults")
	}
}

// TestLoad_StopsAtRepositoryRoot verifies the upward search does not escape the
// repository and pick up an unrelated config from a parent directory.
func TestLoad_StopsAtRepositoryRoot(t *testing.T) {
	outer := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(outer); err == nil {
		outer = resolved
	}

	// A config outside the repository that must not be used.
	if err := os.WriteFile(filepath.Join(outer, ".relicta.yaml"),
		[]byte("versioning:\n  tag_prefix: \"outside-\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	repoRoot := filepath.Join(outer, "repo")
	nested := filepath.Join(repoRoot, "pkg")
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o750); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatalf("MkdirAll(nested) error = %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(nested); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	cfg, err := NewLoader().Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Versioning.TagPrefix == "outside-" {
		t.Error("loaded a config from outside the repository; the search must stop at the repository root")
	}
}

// TestWriteConfig_AnnotatesPublishAffectingKeys covers issue #194: the generated
// config was ~80 alphabetically-sorted keys with zero comments, so the settings
// that can fire a public release looked exactly like the one that sets log level.
func TestWriteConfig_AnnotatesPublishAffectingKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".relicta.yaml")

	if err := WriteConfig(DefaultConfig(), path); err != nil {
		t.Fatalf("WriteConfig() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	body := string(data)

	for _, want := range []string{
		"# Relicta configuration",
		"THIS IS IRREVERSIBLE",
		"will publish twice",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("generated config is missing the annotation %q", want)
		}
	}

	// The annotations must not break parsing, and the values must survive.
	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() on the annotated config error = %v", err)
	}
	if loaded.Versioning.GitPush {
		t.Error("GitPush = true after round-trip, want false")
	}
	if !loaded.Versioning.GitTag {
		t.Error("GitTag = false after round-trip, want true")
	}
}

// Re-running init with --force must not stack a second copy of the comments.
func TestWriteConfig_AnnotationIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".relicta.yaml")

	count := func() int {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		n := 0
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				n++
			}
		}
		return n
	}

	if err := WriteConfig(DefaultConfig(), path); err != nil {
		t.Fatalf("WriteConfig() error = %v", err)
	}
	first := count()

	if err := WriteConfig(DefaultConfig(), path); err != nil {
		t.Fatalf("WriteConfig() second call error = %v", err)
	}
	if second := count(); second != first {
		t.Errorf("comment count = %d after rewrite, want %d - annotations are stacking", second, first)
	}
}

// JSON output must be left alone; a '#' comment would make it invalid.
func TestWriteConfig_JSONIsNotAnnotated(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".relicta.json")

	if err := WriteConfig(DefaultConfig(), path); err != nil {
		t.Fatalf("WriteConfig() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(data), "#") {
		t.Error("JSON config contains a '#' comment, which makes it invalid")
	}
}
