package cli

import (
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/application/governance"
	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
)

// The governance gate is the product. These tests exist because it did not hold.
//
// With governance enabled and a breaking change, `relicta evaluate` reported
// decision=approval_required, can_auto_approve=false, and a required action of
// "Review breaking changes before release". `relicta approve --ci` then approved
// the release anyway: displayAutoApprovalBlocked printed a notice and execution
// fell through to promptForApproval, which returns true for --ci and --yes. The
// audit trail recorded a normal approval attributed to whatever $USER the runner
// used.
//
// A gate that reports itself and does not gate is worse than no gate, because a
// pipeline believes it is governed. The unit under test here is the decision to
// refuse, kept separate from how governance reaches its verdict.

// gateDecision is the part of a governance result the refusal depends on.
func gateResult(canAutoApprove bool, decision cgp.DecisionType, actions ...string) *governance.EvaluateReleaseOutput {
	out := &governance.EvaluateReleaseOutput{
		CanAutoApprove: canAutoApprove,
		Decision:       decision,
		RiskScore:      0.42,
	}
	for _, a := range actions {
		out.RequiredActions = append(out.RequiredActions, cgp.RequiredAction{
			Type:        "human_approval",
			Description: a,
		})
	}
	return out
}

func TestErrApprovalRequiresHuman_NamesTheRequiredActions(t *testing.T) {
	err := errApprovalRequiresHuman(gateResult(false, cgp.DecisionApprovalRequired,
		"Review breaking changes before release"))
	if err == nil {
		t.Fatal("expected an error")
	}

	msg := err.Error()
	// The operator's next step depends on which requirement applies, so the
	// reasons have to travel with the refusal rather than only the refusal.
	if !strings.Contains(msg, "Review breaking changes before release") {
		t.Errorf("refusal should list governance's required actions; got %q", msg)
	}
	// And it has to say what to do, or it is a dead end.
	if !strings.Contains(msg, "relicta approve") {
		t.Errorf("refusal should name the interactive path; got %q", msg)
	}
	if !strings.Contains(msg, "--override-governance") {
		t.Errorf("refusal should name the deliberate override; got %q", msg)
	}
}

// An override is only meaningful if its reason survives. Printing it and
// persisting nothing is what the first version of this change did.
func TestApprovalJustification_MarksAnOverride(t *testing.T) {
	restoreOverride, restoreReason := approveOverride, approveOverrideReason
	t.Cleanup(func() { approveOverride, approveOverrideReason = restoreOverride, restoreReason })

	approveOverride, approveOverrideReason = true, "security sign-off, INC-4471"
	got := approvalJustification()
	if !strings.Contains(got, "INC-4471") {
		t.Errorf("justification must carry the reason; got %q", got)
	}
	// Prefixed so a reader of the audit trail can tell a bypass from an ordinary
	// note without knowing which flag was passed.
	if !strings.HasPrefix(got, "governance override:") {
		t.Errorf("justification should mark itself as an override; got %q", got)
	}
}

func TestApprovalJustification_EmptyWithoutOverride(t *testing.T) {
	restoreOverride, restoreReason := approveOverride, approveOverrideReason
	t.Cleanup(func() { approveOverride, approveOverrideReason = restoreOverride, restoreReason })

	approveOverride, approveOverrideReason = false, ""
	if got := approvalJustification(); got != "" {
		t.Errorf("an ordinary approval carries no justification; got %q", got)
	}

	// A reason without the flag must not be recorded as an override — runApprove
	// rejects that combination, and this guards the formatter independently.
	approveOverride, approveOverrideReason = false, "stray reason"
	if got := approvalJustification(); got != "" {
		t.Errorf("a reason without --override-governance is not an override; got %q", got)
	}
}

// TestApproverActorType_DistinguishesCIFromHuman covers an attribution
// falsehood: the actor type was hardcoded to human, so a pipeline's approval was
// recorded as a person's decision, attributed to whatever $USER the runner used.
// Asked later who approved a release, the trail named someone who was not
// involved.
func TestApproverActorType_DistinguishesCIFromHuman(t *testing.T) {
	restore := ciMode
	t.Cleanup(func() { ciMode = restore })

	ciMode = true
	if got := approverActorType(); got != domain.ActorCI {
		t.Errorf("a --ci approval must be recorded as %q, got %q", domain.ActorCI, got)
	}

	ciMode = false
	if got := approverActorType(); got != domain.ActorHuman {
		t.Errorf("an interactive approval must be recorded as %q, got %q", domain.ActorHuman, got)
	}
}
