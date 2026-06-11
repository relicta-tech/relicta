package plugin

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

func TestTrustGate_BlocksOnBestEffortPlatformWithoutOptIn(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("trust gate is no-op on linux strict-enforcement platforms")
	}

	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "trust-test", Enabled: boolPtrTG(true)},
		},
	}
	m := NewManager(cfg)
	// Default: no opt-in.
	err := m.LoadPlugins(context.Background())
	if err == nil {
		t.Fatal("expected trust gate to refuse load on best-effort platform without opt-in")
	}
	if !strings.Contains(err.Error(), "untrusted") && !strings.Contains(err.Error(), "Plugin Sandbox Security Posture") {
		t.Errorf("error should reference trust gate / security posture; got: %v", err)
	}
}

func TestTrustGate_AllowsAfterOptIn(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("trust gate is no-op on linux")
	}

	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "trust-test", Enabled: boolPtrTG(true), ContinueOnError: true},
		},
	}
	m := NewManager(cfg)
	m.AllowUntrustedPlugins(true)

	// With opt-in + ContinueOnError, missing plugin still loads (ignoring
	// non-existence) — what matters is the trust gate doesn't reject.
	err := m.LoadPlugins(context.Background())
	if err != nil {
		t.Errorf("opt-in should bypass trust gate; got %v", err)
	}
}

func TestTrustGate_NoEnabledPluginsShortCircuits(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "off-1", Enabled: boolPtrTG(false)},
			{Name: "off-2", Enabled: boolPtrTG(false)},
		},
	}
	m := NewManager(cfg)
	// Default no opt-in. With all plugins disabled, the gate is skipped.
	if err := m.LoadPlugins(context.Background()); err != nil {
		t.Errorf("LoadPlugins with all-disabled plugins should not invoke trust gate; got %v", err)
	}
}

func TestHasEnabledPlugins(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{
			"empty config",
			&config.Config{},
			false,
		},
		{
			"all disabled",
			&config.Config{Plugins: []config.PluginConfig{
				{Name: "a", Enabled: boolPtrTG(false)},
			}},
			false,
		},
		{
			"one enabled",
			&config.Config{Plugins: []config.PluginConfig{
				{Name: "a", Enabled: boolPtrTG(false)},
				{Name: "b", Enabled: boolPtrTG(true)},
			}},
			true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := NewManager(c.cfg)
			if got := m.hasEnabledPlugins(); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

// boolPtrTG is a local helper to avoid colliding with boolPtr in manager_test.go.
func boolPtrTG(b bool) *bool { return &b }
