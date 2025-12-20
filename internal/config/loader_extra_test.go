package config

import "testing"

func TestGitConfigUseCLI(t *testing.T) {
	g := GitConfig{}
	if !g.UseCLI() {
		t.Fatal("expected UseCLI to default to true")
	}

	flag := true
	g.UseCLIFallback = &flag
	if !g.UseCLI() {
		t.Fatal("expected UseCLI to honor pointer value")
	}

	flag = false
	if g.UseCLI() {
		t.Fatal("expected UseCLI to respect false value")
	}
}

func TestGovernancePolicyConfigIsEnabled(t *testing.T) {
	cfg := GovernancePolicyConfig{}
	if !cfg.IsPolicyEnabled() {
		t.Fatal("expected policy enabled by default")
	}

	disabled := false
	cfg.Enabled = &disabled
	if cfg.IsPolicyEnabled() {
		t.Fatal("expected policy disabled when pointer false")
	}
}

func TestLoaderGetConfigPathAndMerge(t *testing.T) {
	l := NewLoader()
	if got := l.GetConfigPath(); got != "" {
		t.Fatalf("expected empty config path, got %q", got)
	}

	if err := l.MergeConfig(map[string]any{"ai.enabled": true}); err != nil {
		t.Fatalf("unexpected merge error: %v", err)
	}
	if !l.v.GetBool("ai.enabled") {
		t.Fatalf("expected ai.enabled to be true after merge")
	}
}

func TestMustLoadReturnsConfig(t *testing.T) {
	cfg := MustLoad()
	if cfg == nil {
		t.Fatal("expected MustLoad to return config")
	}
}
