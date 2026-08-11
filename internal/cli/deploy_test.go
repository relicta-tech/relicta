package cli

import (
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// Environments are declared rather than free-form. A deployment record arrives from
// outside — a GitOps controller, a CI step, a script — and free-form names let
// "prod", "production" and "Production" become three environments in one audit
// report, each holding part of the history, with nothing reporting that as wrong.

func withEnvironments(t *testing.T, envs ...config.EnvironmentConfig) {
	t.Helper()

	orig := cfg
	t.Cleanup(func() { cfg = orig })

	cfg = config.DefaultConfig()
	cfg.Environments = envs
}

func TestDeclaredEnvironmentRefusesUnknownNames(t *testing.T) {
	withEnvironments(t,
		config.EnvironmentConfig{Name: "staging"},
		config.EnvironmentConfig{Name: "production", Production: true},
	)

	if _, err := declaredEnvironment("production"); err != nil {
		t.Errorf("a declared environment must be accepted: %v", err)
	}

	// The case that matters: a near-miss must fail rather than create a phantom
	// environment that then appears in an audit report.
	_, err := declaredEnvironment("prod")
	if err == nil {
		t.Fatal(`"prod" is not declared and must be refused`)
	}
	// The message has to name what is declared, or the caller has to guess.
	for _, want := range []string{"prod", "staging", "production"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal should name %q; got %v", want, err)
		}
	}
}

func TestDeclaredEnvironmentWithNoneConfigured(t *testing.T) {
	withEnvironments(t)

	_, err := declaredEnvironment("production")
	if err == nil {
		t.Fatal("with no environments declared, recording must be refused")
	}
	if !strings.Contains(err.Error(), "environments") {
		t.Errorf("the error should say what to add; got %v", err)
	}
}

// Deployment frequency means changes reaching users, so exactly one environment
// carries the designation. Without it the report counts every environment and reads
// high — which is why the reporting path falls back instead of guessing.
func TestProductionEnvironmentName(t *testing.T) {
	withEnvironments(t,
		config.EnvironmentConfig{Name: "staging"},
		config.EnvironmentConfig{Name: "production", Production: true},
	)
	if got := productionEnvironmentName(); got != "production" {
		t.Errorf("productionEnvironmentName() = %q, want production", got)
	}

	withEnvironments(t, config.EnvironmentConfig{Name: "staging"})
	if got := productionEnvironmentName(); got != "" {
		t.Errorf("with nothing marked production, got %q, want empty so the report "+
			"falls back rather than counting staging", got)
	}
}

// The commands have to be reachable and grouped, since a correct command nothing
// registers is the failure this codebase keeps producing.
func TestDeployCommandsAreRegistered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "deploy" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("`relicta deploy` is not registered")
	}

	want := map[string]bool{"record": false, "list": false}
	for _, c := range deployCmd.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, ok := range want {
		if !ok {
			t.Errorf("`relicta deploy %s` is not registered", name)
		}
	}
}
