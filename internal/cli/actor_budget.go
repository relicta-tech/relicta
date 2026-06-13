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
	var set *policy.ActorBudgetSet
	if cfg != nil && cfg.Governance.ActorBudgetPath != "" {
		loaded, err := policy.LoadActorBudgets(cfg.Governance.ActorBudgetPath)
		if err != nil {
			return fmt.Errorf("failed to load actor budgets from %s: %w", cfg.Governance.ActorBudgetPath, err)
		}
		set = loaded
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
