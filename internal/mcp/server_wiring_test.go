package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
)

// `relicta mcp serve` never called WithReleaseRepository or WithActorBudgets, so two
// features were off in production while their unit tests passed:
//
//   - Five resources — release state, active runs, history, the run's recommendation —
//     answered "no release repository configured", though the container had the repository.
//     An agent asking what release is in progress got a stub, which reads as "no release"
//     rather than "this server was not wired".
//
//   - A configured governance.actor_budget_path gated the CLI and was ignored here, on the
//     surface agents actually use. ResolveBudget's fallback is restrictive, so nothing was
//     unsafe by default; what was missing was the operator's own policy, whether it widened
//     a trusted agent's budget or tightened one past the default.
//
// These assert the option reaches the behavior, not that the option assigns a field — the
// assignment was never the broken part.

func TestStateResourceReportsWiringRatherThanEmptiness(t *testing.T) {
	server, err := NewServer("test")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	content, err := server.handleResourceState(context.Background(), "relicta://state", nil)
	if err != nil {
		t.Fatalf("handleResourceState: %v", err)
	}

	// Unwired, it must say so rather than claim there is no release: the two are
	// indistinguishable to a caller, and only one of them is the server's fault.
	if !strings.Contains(content.Text, "repository") {
		t.Errorf("an unconfigured server's state resource says %q, which a caller cannot "+
			"tell apart from a repository that genuinely has no release", content.Text)
	}
}

// The budget the MCP surface enforces must be the operator's when they configured one.
//
// The assertion has to be that a *permissive* budget takes effect, and this is the second
// version of the test: the first configured a restrictive budget and asserted the operation
// was refused, which passed whether or not the budget was applied — ResolveBudget's fallback
// is the restrictive default, so it refuses the same operation for its own reasons. Making
// the option a no-op left that version green. Only a budget that allows something the
// default forbids can tell the two apart.
func TestConfiguredBudgetsReachTheMCPGate(t *testing.T) {
	// Every limit at its zero value, which this type defines as unrestricted. Whether
	// that is wise is the operator's call; whether it is honored is this test's.
	set := &policy.ActorBudgetSet{
		Budgets: []policy.ActorBudget{{ActorKind: MCPActorKind}},
	}

	highRisk := policy.Operation{
		Tool:        "relicta_publish",
		RiskScore:   0.95,
		BlastRadius: policy.BlastRadiusHigh,
	}

	configured, err := NewServer("test", WithActorBudgets(set))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := configured.checkBudget(context.Background(), highRisk); err != nil {
		t.Errorf("the operator's unrestricted budget was not applied — the operation was "+
			"refused as though no budget had been configured: %v", err)
	}

	// The control: the identical operation on a server with no configured budgets must be
	// refused, or the test above proves nothing about the budget being consulted.
	unconfigured, err := NewServer("test")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := unconfigured.checkBudget(context.Background(), highRisk); err == nil {
		t.Fatal("the control case was allowed too, so this test cannot distinguish a " +
			"configured budget from the default and asserts nothing")
	}
}

// Without a configured set the defaults apply, which must stay restrictive: this is what
// protects a server whose operator has configured nothing at all.
func TestTheDefaultBudgetStillRefusesHighRiskWork(t *testing.T) {
	server, err := NewServer("test")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	if err := server.checkBudget(context.Background(), policy.Operation{
		Tool:        "relicta_publish",
		RiskScore:   0.95,
		BlastRadius: policy.BlastRadiusHigh,
	}); err == nil {
		t.Error("with no configured budgets, a high-risk publish was allowed: the fallback " +
			"is supposed to be the restrictive agent budget")
	}
}
