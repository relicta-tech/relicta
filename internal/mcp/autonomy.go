package mcp

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
)

// MCPActorID is the synthetic actor ID used for MCP-driven operations when the
// transport does not surface a more specific identity. Aligns with the
// "mcp-agent" identifier already used by handleApprove for approval records.
const MCPActorID = "mcp-agent"

// MCPActorKind is the actor kind for MCP-driven operations. All MCP-driven
// privileged operations are treated as agent actions for budget purposes —
// human operators using the CLI bypass the MCP layer entirely.
const MCPActorKind = "agent"

// MCPBudgetDenialError is returned when an autonomy budget rejects an MCP
// tool invocation. The error message lists every violation so callers can
// surface the full failure reason without a second round-trip.
type MCPBudgetDenialError struct {
	Tool     string
	Decision policy.Decision
}

func (e *MCPBudgetDenialError) Error() string {
	if len(e.Decision.Violations) == 0 {
		return fmt.Sprintf("autonomy budget denied tool %q", e.Tool)
	}
	msg := fmt.Sprintf("autonomy budget denied tool %q (%d violation(s)):", e.Tool, len(e.Decision.Violations))
	for _, v := range e.Decision.Violations {
		msg += "\n  - " + v.Code + ": " + v.Message
	}
	return msg
}

// checkBudget enforces the actor autonomy budget for an MCP-driven privileged
// operation. Returns nil if allowed; an *MCPBudgetDenialError if denied.
//
// When no budget set is configured (`WithActorBudgets` not called) it falls
// back to `DefaultRestrictiveAgentBudget` for safety — the autonomy slider
// fails closed for agents.
//
// For matching purposes the operation Tool name is normalized: the
// `relicta_` prefix used in MCP tool registration is stripped so budget
// configs can use logical operation names (e.g. `publish`, `approve`)
// regardless of the transport surface.
func (s *Server) checkBudget(_ context.Context, op policy.Operation) error {
	budget := policy.ResolveBudget(s.actorBudgets, MCPActorKind, MCPActorID)
	op.Tool = normalizeToolName(op.Tool)
	d := budget.Evaluate(op)
	if d.Allowed {
		return nil
	}
	return &MCPBudgetDenialError{Tool: op.Tool, Decision: d}
}

// normalizeToolName strips the `relicta_` prefix from MCP tool names so
// downstream policy matching uses logical operation identifiers.
func normalizeToolName(tool string) string {
	const prefix = "relicta_"
	if len(tool) > len(prefix) && tool[:len(prefix)] == prefix {
		return tool[len(prefix):]
	}
	return tool
}

// blastRadiusForRiskScore maps a numeric risk score to a blast-radius
// category. This is a coarse approximation — when the adapter exposes a
// computed blast radius directly, prefer that.
func blastRadiusForRiskScore(risk float64) policy.BlastRadius {
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
