package governance

import (
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// The evaluator enforces freeze windows and the risk budget — it calls budget.CheckFreeze on
// every evaluation — and neither builder passed them, so its fields stayed at their zero
// values. A configured release freeze permitted every release through it.
//
// For a governance tool that is the worst place for this defect: the control reports itself as
// configured, the evaluation says approved, and the record shows a release that a policy said
// should not happen.

func TestTheFreezeWindowsReachTheEvaluator(t *testing.T) {
	cfg := &config.GovernanceConfig{
		FreezePeriods: []config.FreezePeriodConfig{
			{Name: "weekend", Start: "Friday 16:00", End: "Monday 09:00", MaxRisk: 0.2},
		},
	}

	evalCfg := EvaluatorConfigFromGovernance(cfg)

	if len(evalCfg.FreezePeriods) != 1 {
		t.Fatalf("the evaluator was given %d freeze periods, want 1.\nWithout them a "+
			"configured freeze permits every release, while reporting itself as configured",
			len(evalCfg.FreezePeriods))
	}
	if evalCfg.FreezePeriods[0].Name != "weekend" {
		t.Errorf("freeze period = %q, want weekend", evalCfg.FreezePeriods[0].Name)
	}
	if evalCfg.FreezePeriods[0].MaxRisk != 0.2 {
		t.Errorf("max risk = %v, want 0.2 — the number that decides what the freeze lets "+
			"through", evalCfg.FreezePeriods[0].MaxRisk)
	}
}

func TestTheRiskBudgetReachesTheEvaluator(t *testing.T) {
	budget := &config.RiskBudgetConfig{}
	cfg := &config.GovernanceConfig{RiskBudget: budget}

	if got := EvaluatorConfigFromGovernance(cfg).RiskBudget; got != budget {
		t.Error("the risk budget did not reach the evaluator that enforces it")
	}
}

// A repository that configured neither is unaffected, which is every repository until now.
func TestNoFreezeConfigurationLeavesTheEvaluatorUnconstrained(t *testing.T) {
	evalCfg := EvaluatorConfigFromGovernance(&config.GovernanceConfig{})

	if len(evalCfg.FreezePeriods) != 0 {
		t.Errorf("freeze periods = %v for a repository that configured none", evalCfg.FreezePeriods)
	}
	if evalCfg.RiskBudget != nil {
		t.Error("a risk budget appeared from nowhere")
	}
}
