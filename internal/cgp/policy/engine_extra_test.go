package policy

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/internal/cgp"
)

func TestEngine_Evaluate_ActionAddReviewerAndCondition(t *testing.T) {
	policy := NewPolicy("test-policy")
	policy.AddRule(*NewRule("require-manual", "Require manual review").
		WithPriority(50).
		AddCondition("actor.kind", OperatorEqual, "human").
		AddAction(ActionAddReviewer, map[string]any{"reviewer": "alice"}).
		AddAction(ActionAddReviewer, map[string]any{"reviewers": []string{"bob", "carol"}}).
		AddAction(ActionAddCondition, map[string]any{"type": "approval", "value": "manual"}).
		AddAction(ActionAddRationale, map[string]any{"message": "Manual review required"}))

	engine := NewEngine([]Policy{*policy}, nil)
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Important change", Confidence: 0.9},
	)

	result, err := engine.Evaluate(context.Background(), proposal, nil, 0.2)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if len(result.Reviewers) != 3 {
		t.Fatalf("Reviewers = %v, want 3", result.Reviewers)
	}
	expected := map[string]bool{"alice": true, "bob": true, "carol": true}
	for _, reviewer := range result.Reviewers {
		if !expected[reviewer] {
			t.Fatalf("unexpected reviewer %s", reviewer)
		}
	}

	if len(result.Conditions) != 1 {
		t.Fatalf("Conditions = %v, want 1", result.Conditions)
	}
	if result.Conditions[0].Type != "approval" || result.Conditions[0].Value != "manual" {
		t.Fatalf("Condition mismatch %+v", result.Conditions[0])
	}

	found := false
	for _, r := range result.Rationale {
		if r == "Manual review required" || r == "Rule 'require-manual': Require manual review" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected rationale about manual review, got %v", result.Rationale)
	}
}
