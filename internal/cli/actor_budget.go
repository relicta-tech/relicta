// Package cli provides the command-line interface for Relicta.
package cli

import (
	"fmt"
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
)

// enforceActorBudget gates a privileged CLI operation (publish/approve/
// rollback) against the per-actor autonomy budget — the same control the MCP
// surface applies. Before this, the CLI path skipped budget enforcement
// entirely, so an agent or CI actor could drive high-risk releases the
// autonomy slider was meant to block.
//
// Budgets load from governance.actor_budget_path when set; otherwise
// ResolveBudget supplies safe defaults (humans permissive, agents/CI
// restrictive). tool is the logical operation name ("publish", "approve",
// "rollback"); riskScore is the CGP risk of the change being acted upon.
func enforceActorBudget(tool string, riskScore float64) error {
	set, err := loadActorBudgetSet()
	if err != nil {
		return err
	}

	actor := createCGPActor()
	budget := policy.ResolveBudget(set, actor.Kind.String(), actor.ID)

	decision := budget.Evaluate(policy.Operation{
		Tool:        tool,
		BlastRadius: blastRadiusForRisk(riskScore),
		RiskScore:   riskScore,
	})
	if decision.Allowed {
		return nil
	}

	var msgs []string
	for _, v := range decision.Violations {
		msgs = append(msgs, v.Message)
	}
	return fmt.Errorf("operation %q denied by autonomy budget for actor %s: %s",
		tool, actor.ID, strings.Join(msgs, "; "))
}

// blastRadiusForRisk maps a risk score to a blast-radius category, mirroring
// the MCP mapping so both surfaces gate identically.
func blastRadiusForRisk(risk float64) policy.BlastRadius {
	switch {
	case risk >= 0.8:
		return policy.BlastRadiusCritical
	case risk >= 0.6:
		return policy.BlastRadiusHigh
	case risk >= 0.4:
		return policy.BlastRadiusMedium
	case risk > 0:
		return policy.BlastRadiusLow
	default:
		return policy.BlastRadiusNone
	}
}

// loadActorBudgetSet reads the operator's per-actor autonomy budgets, or nil when none are
// configured (ResolveBudget then supplies the restrictive defaults).
//
// One loader, because the CLI gate and the MCP server both need the same answer and only
// the CLI was asking. `relicta mcp serve` never called WithActorBudgets, so a configured
// governance.actor_budget_path constrained the CLI and was ignored by the MCP surface —
// the surface agents actually use, and the reason per-actor budgets exist. The fallback is
// restrictive, so nothing was unsafe by default; what was lost is the operator's own
// policy, in either direction: a budget widened for a trusted agent and a budget tightened
// beyond the default were both silently absent.
func loadActorBudgetSet() (*policy.ActorBudgetSet, error) {
	if cfg == nil || cfg.Governance.ActorBudgetPath == "" {
		return nil, nil
	}
	set, err := policy.LoadActorBudgets(cfg.Governance.ActorBudgetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load actor budgets from %s: %w", cfg.Governance.ActorBudgetPath, err)
	}
	return set, nil
}
