package plugin

import (
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/pkg/plugin"
)

func TestValidateSafety_UndeclaredIsGracePeriodWarning(t *testing.T) {
	info := plugin.Info{Name: "legacy", Hooks: []plugin.Hook{plugin.HookPostPublish}}

	decision, err := ValidateSafety(info, false)
	if err != nil {
		t.Fatalf("undeclared legacy plugin must load during grace period: %v", err)
	}
	if decision.Declared {
		t.Error("decision must record the plugin as undeclared")
	}
	if decision.RiskClass != plugin.RiskRuntime {
		t.Errorf("undeclared plugins are runtime-class, got %s", decision.RiskClass)
	}
	if decision.Warning == "" {
		t.Error("undeclared plugins must produce an operator warning")
	}
}

func TestValidateSafety_InconsistentDeclarationRefused(t *testing.T) {
	info := plugin.Info{
		Name:  "liar",
		Hooks: []plugin.Hook{plugin.HookPostPublish},
		Safety: &plugin.SafetyRequirements{
			RiskClass: plugin.RiskPassive,
		},
	}

	_, err := ValidateSafety(info, true) // even with trust flag
	if err == nil || !strings.Contains(err.Error(), "inconsistent") {
		t.Fatalf("passive declaration on a publish hook must be refused, got %v", err)
	}
}

func TestValidateSafety_RuntimeRequiresTrust(t *testing.T) {
	info := plugin.Info{
		Name:   "runner",
		Hooks:  []plugin.Hook{plugin.HookOnSuccess},
		Safety: &plugin.SafetyRequirements{RiskClass: plugin.RiskRuntime},
	}

	if _, err := ValidateSafety(info, false); err == nil {
		t.Fatal("runtime-class plugin must require --allow-untrusted-plugins")
	}
	decision, err := ValidateSafety(info, true)
	if err != nil {
		t.Fatalf("runtime-class plugin with trust flag must load: %v", err)
	}
	if decision.RiskClass != plugin.RiskRuntime {
		t.Errorf("got %s", decision.RiskClass)
	}
}

func TestValidateSafety_ConsistentDeclarations(t *testing.T) {
	cases := []struct {
		name  string
		hooks []plugin.Hook
		class plugin.RiskClass
	}{
		{"passive-notifier", []plugin.Hook{plugin.HookOnSuccess, plugin.HookOnError}, plugin.RiskPassive},
		{"active-publisher", []plugin.Hook{plugin.HookPostPublish}, plugin.RiskActive},
	}
	for _, c := range cases {
		info := plugin.Info{Name: c.name, Hooks: c.hooks, Safety: &plugin.SafetyRequirements{RiskClass: c.class}}
		decision, err := ValidateSafety(info, false)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if !decision.Declared || decision.RiskClass != c.class {
			t.Errorf("%s: decision %+v", c.name, decision)
		}
	}
}

func TestValidateSafety_UnknownRiskClass(t *testing.T) {
	info := plugin.Info{
		Name:   "weird",
		Safety: &plugin.SafetyRequirements{RiskClass: "yolo"},
	}
	if _, err := ValidateSafety(info, true); err == nil || !strings.Contains(err.Error(), "unknown risk_class") {
		t.Fatalf("unknown risk class must be refused, got %v", err)
	}
}
