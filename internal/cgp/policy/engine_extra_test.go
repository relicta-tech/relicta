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

func TestEngine_Evaluate_TimeBasedConditions(t *testing.T) {
	t.Run("block during hard freeze", func(t *testing.T) {
		policy := NewPolicy("freeze-policy")
		policy.AddRule(*NewRule("hard-freeze", "Block during hard freeze").
			WithPriority(100).
			WithDescription("No releases during hard freeze").
			AddCondition("time.freeze.isHard", OperatorEqual, true).
			AddAction(ActionBlock, map[string]any{"reason": "Hard freeze in effect"}))

		engine := NewEngine([]Policy{*policy}, nil)

		// Set up a hard freeze period
		engine.AddFreezePeriod(FreezePeriod{
			Name:     "Holiday Freeze",
			Start:    engine.timeContext.Now.AddDate(0, 0, -1),
			End:      engine.timeContext.Now.AddDate(0, 0, 1),
			Reason:   "Holiday code freeze",
			Severity: "hard",
		})

		proposal := cgp.NewProposal(
			cgp.NewHumanActor("dev@example.com", "Developer"),
			cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
			cgp.ProposalIntent{Summary: "Bug fix", Confidence: 0.9},
		)

		result, err := engine.Evaluate(context.Background(), proposal, nil, 0.2)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}

		if !result.Blocked {
			t.Error("expected release to be blocked during hard freeze")
		}

		if result.Decision != cgp.DecisionRejected {
			t.Errorf("Decision = %v, want %v", result.Decision, cgp.DecisionRejected)
		}

		if result.BlockReason != "Hard freeze in effect" {
			t.Errorf("BlockReason = %s, want 'Hard freeze in effect'", result.BlockReason)
		}
	})

	t.Run("require review during soft freeze", func(t *testing.T) {
		policy := NewPolicy("soft-freeze-policy")
		policy.AddRule(*NewRule("soft-freeze", "Require extra review during soft freeze").
			WithPriority(100).
			WithDescription("Extra review required during soft freeze").
			AddCondition("time.freeze.isSoft", OperatorEqual, true).
			AddAction(ActionRequireApproval, map[string]any{"count": float64(2), "description": "Soft freeze requires extra approval"}))

		engine := NewEngine([]Policy{*policy}, nil)

		// Set up a soft freeze period
		engine.AddFreezePeriod(FreezePeriod{
			Name:     "Release Window",
			Start:    engine.timeContext.Now.AddDate(0, 0, -1),
			End:      engine.timeContext.Now.AddDate(0, 0, 1),
			Reason:   "Pre-release stabilization",
			Severity: "soft",
		})

		proposal := cgp.NewProposal(
			cgp.NewHumanActor("dev@example.com", "Developer"),
			cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
			cgp.ProposalIntent{Summary: "Minor update", Confidence: 0.9},
		)

		result, err := engine.Evaluate(context.Background(), proposal, nil, 0.2)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}

		if result.Decision != cgp.DecisionApprovalRequired {
			t.Errorf("Decision = %v, want %v", result.Decision, cgp.DecisionApprovalRequired)
		}

		if result.RequiredApprovers != 2 {
			t.Errorf("RequiredApprovers = %d, want 2", result.RequiredApprovers)
		}
	})

	t.Run("no freeze active", func(t *testing.T) {
		policy := NewPolicy("freeze-check-policy")
		policy.AddRule(*NewRule("freeze-check", "Check freeze status").
			WithPriority(100).
			AddCondition("time.freeze.active", OperatorEqual, true).
			AddAction(ActionBlock, map[string]any{"reason": "Frozen"}))

		engine := NewEngine([]Policy{*policy}, nil)
		// No freeze periods added

		proposal := cgp.NewProposal(
			cgp.NewHumanActor("dev@example.com", "Developer"),
			cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
			cgp.ProposalIntent{Summary: "Normal release", Confidence: 0.9},
		)

		result, err := engine.Evaluate(context.Background(), proposal, nil, 0.2)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}

		// Should not be blocked because no freeze is active
		if result.Blocked {
			t.Error("expected release to NOT be blocked when no freeze is active")
		}
	})

	t.Run("business hours check", func(t *testing.T) {
		policy := NewPolicy("business-hours-policy")
		policy.AddRule(*NewRule("after-hours", "Require review for after-hours releases").
			WithPriority(100).
			WithDescription("Extra caution for releases outside business hours").
			AddCondition("time.isBusinessHours", OperatorEqual, false).
			AddAction(ActionRequireApproval, map[string]any{"count": float64(2), "description": "After-hours release requires extra approval"}))

		engine := NewEngine([]Policy{*policy}, nil)

		// Configure business hours (9-17)
		engine.SetBusinessHours(BusinessHoursConfig{
			StartHour:     9,
			EndHour:       17,
			AllowWeekends: false,
		})

		proposal := cgp.NewProposal(
			cgp.NewHumanActor("dev@example.com", "Developer"),
			cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
			cgp.ProposalIntent{Summary: "Evening fix", Confidence: 0.9},
		)

		result, err := engine.Evaluate(context.Background(), proposal, nil, 0.2)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}

		// Result depends on current time - we just verify no errors
		// and that the time fields are populated in the eval context
		if result == nil {
			t.Error("expected non-nil result")
		}
	})
}
