package policy

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/internal/cgp"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine([]Policy{}, nil)

	if engine == nil {
		t.Error("NewEngine() should not return nil")
	}
}

func TestEngine_Evaluate_NoPolicies(t *testing.T) {
	engine := NewEngine([]Policy{}, nil)
	proposal := cgp.NewProposal(
		cgp.NewAgentActor("cursor", "Cursor", "gpt-4"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
	)

	result, err := engine.Evaluate(context.Background(), proposal, nil, 0.5)
	if err != nil {
		t.Errorf("Evaluate() error = %v", err)
	}
	if result.Decision != cgp.DecisionApproved {
		t.Errorf("Evaluate() Decision = %v, want %v", result.Decision, cgp.DecisionApproved)
	}
}

func TestEngine_Evaluate_WithMatchingRule(t *testing.T) {
	policy := NewPolicy("test-policy")
	policy.AddRule(*NewRule("agent-review", "Require review for agent changes").
		WithPriority(100).
		WithDescription("AI agents require human review").
		AddCondition("actor.kind", OperatorEqual, "agent").
		AddAction(ActionRequireApproval, map[string]any{"count": float64(1), "description": "Agent change requires review"}))

	engine := NewEngine([]Policy{*policy}, nil)
	proposal := cgp.NewProposal(
		cgp.NewAgentActor("cursor", "Cursor", "gpt-4"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
	)

	result, err := engine.Evaluate(context.Background(), proposal, nil, 0.5)
	if err != nil {
		t.Errorf("Evaluate() error = %v", err)
	}
	if result.Decision != cgp.DecisionApprovalRequired {
		t.Errorf("Evaluate() Decision = %v, want %v", result.Decision, cgp.DecisionApprovalRequired)
	}
	if len(result.MatchedRules) != 1 {
		t.Errorf("Evaluate() MatchedRules = %d, want 1", len(result.MatchedRules))
	}
	if result.MatchedRules[0] != "agent-review" {
		t.Errorf("Evaluate() MatchedRules[0] = %v, want agent-review", result.MatchedRules[0])
	}
	if result.RequiredApprovers != 1 {
		t.Errorf("Evaluate() RequiredApprovers = %d, want 1", result.RequiredApprovers)
	}
}

func TestEngine_Evaluate_RiskScoreCondition(t *testing.T) {
	policy := NewPolicy("test-policy")
	policy.AddRule(*NewRule("high-risk-block", "Block high risk changes").
		WithPriority(100).
		AddCondition("risk.score", OperatorGreaterOrEqual, 0.8).
		AddAction(ActionBlock, map[string]any{"reason": "Risk score too high"}))

	engine := NewEngine([]Policy{*policy}, nil)
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
	)

	// Test with high risk
	result, err := engine.Evaluate(context.Background(), proposal, nil, 0.9)
	if err != nil {
		t.Errorf("Evaluate() error = %v", err)
	}
	if result.Decision != cgp.DecisionRejected {
		t.Errorf("Evaluate() Decision = %v, want %v", result.Decision, cgp.DecisionRejected)
	}
	if !result.Blocked {
		t.Error("Evaluate() Blocked should be true")
	}

	// Test with low risk
	result, err = engine.Evaluate(context.Background(), proposal, nil, 0.3)
	if err != nil {
		t.Errorf("Evaluate() error = %v", err)
	}
	if result.Decision == cgp.DecisionRejected {
		t.Errorf("Evaluate() Decision should not be rejected for low risk")
	}
}

func TestEngine_Evaluate_BreakingChanges(t *testing.T) {
	policy := NewPolicy("test-policy")
	policy.AddRule(*NewRule("breaking-review", "Review breaking changes").
		WithPriority(100).
		AddCondition("change.breaking", OperatorGreaterThan, 0).
		AddAction(ActionRequireApproval, map[string]any{"count": float64(2)}))

	engine := NewEngine([]Policy{*policy}, nil)
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
	)
	analysis := &cgp.ChangeAnalysis{
		Features: 1,
		Breaking: 2,
	}

	result, err := engine.Evaluate(context.Background(), proposal, analysis, 0.5)
	if err != nil {
		t.Errorf("Evaluate() error = %v", err)
	}
	if result.Decision != cgp.DecisionApprovalRequired {
		t.Errorf("Evaluate() Decision = %v, want %v", result.Decision, cgp.DecisionApprovalRequired)
	}
	if result.RequiredApprovers != 2 {
		t.Errorf("Evaluate() RequiredApprovers = %d, want 2", result.RequiredApprovers)
	}
}

func TestEngine_Evaluate_MultipleRules(t *testing.T) {
	policy := NewPolicy("test-policy")
	policy.AddRule(*NewRule("low-priority", "Low priority rule").
		WithPriority(10).
		AddCondition("actor.kind", OperatorEqual, "agent").
		AddAction(ActionAddRationale, map[string]any{"message": "Low priority matched"}))
	policy.AddRule(*NewRule("high-priority", "High priority rule").
		WithPriority(100).
		AddCondition("actor.kind", OperatorEqual, "agent").
		AddAction(ActionRequireApproval, map[string]any{"count": float64(1)}))

	engine := NewEngine([]Policy{*policy}, nil)
	proposal := cgp.NewProposal(
		cgp.NewAgentActor("cursor", "Cursor", "gpt-4"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
	)

	result, err := engine.Evaluate(context.Background(), proposal, nil, 0.5)
	if err != nil {
		t.Errorf("Evaluate() error = %v", err)
	}
	// Both rules should match
	if len(result.MatchedRules) != 2 {
		t.Errorf("Evaluate() MatchedRules = %d, want 2", len(result.MatchedRules))
	}
	// High priority should be first
	if result.MatchedRules[0] != "high-priority" {
		t.Errorf("Evaluate() MatchedRules[0] = %v, want high-priority", result.MatchedRules[0])
	}
}

func TestEngine_Evaluate_InOperator(t *testing.T) {
	policy := NewPolicy("test-policy")
	policy.AddRule(*NewRule("allowed-actors", "Allow specific actors").
		WithPriority(100).
		AddCondition("actor.kind", OperatorIn, []any{"human", "ci"}).
		AddAction(ActionSetDecision, map[string]any{"decision": "approve"}))

	engine := NewEngine([]Policy{*policy}, nil)

	// Test with human (should match)
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
	)
	result, _ := engine.Evaluate(context.Background(), proposal, nil, 0.3)
	if len(result.MatchedRules) != 1 {
		t.Errorf("Human should match 'in' condition, got %d matches", len(result.MatchedRules))
	}

	// Test with agent (should not match)
	proposal = cgp.NewProposal(
		cgp.NewAgentActor("cursor", "Cursor", "gpt-4"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
	)
	result, _ = engine.Evaluate(context.Background(), proposal, nil, 0.3)
	if len(result.MatchedRules) != 0 {
		t.Errorf("Agent should not match 'in' condition, got %d matches", len(result.MatchedRules))
	}
}

func TestEngine_Evaluate_DisabledRule(t *testing.T) {
	policy := NewPolicy("test-policy")
	rule := NewRule("disabled-rule", "This rule is disabled").
		AddCondition("actor.kind", OperatorEqual, "agent").
		AddAction(ActionBlock, map[string]any{"reason": "Should not apply"})
	rule.Enabled = false
	policy.AddRule(*rule)

	engine := NewEngine([]Policy{*policy}, nil)
	proposal := cgp.NewProposal(
		cgp.NewAgentActor("cursor", "Cursor", "gpt-4"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
	)

	result, err := engine.Evaluate(context.Background(), proposal, nil, 0.5)
	if err != nil {
		t.Errorf("Evaluate() error = %v", err)
	}
	if len(result.MatchedRules) != 0 {
		t.Errorf("Disabled rule should not match, got %d matches", len(result.MatchedRules))
	}
	if result.Blocked {
		t.Error("Disabled rule should not block")
	}
}

func TestGetNestedValue(t *testing.T) {
	data := map[string]any{
		"actor": map[string]any{
			"kind": "agent",
			"name": "Cursor",
		},
		"risk": map[string]any{
			"score": 0.5,
		},
	}

	tests := []struct {
		name     string
		path     string
		expected any
		found    bool
		isMap    bool // indicate if expected is a map (skip value comparison)
	}{
		{"top-level field", "actor", nil, true, true},
		{"nested field", "actor.kind", "agent", true, false},
		{"another nested", "risk.score", 0.5, true, false},
		{"missing field", "missing", nil, false, false},
		{"missing nested", "actor.missing", nil, false, false},
		{"deep missing", "actor.kind.deep", nil, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found := getNestedValue(data, tt.path)
			if found != tt.found {
				t.Errorf("getNestedValue() found = %v, want %v", found, tt.found)
			}
			// Skip value comparison for maps
			if tt.isMap {
				if found {
					if _, ok := value.(map[string]any); !ok {
						t.Errorf("getNestedValue() expected map type, got %T", value)
					}
				}
				return
			}
			if found && value != tt.expected {
				t.Errorf("getNestedValue() value = %v, want %v", value, tt.expected)
			}
		})
	}
}

func TestCompareValues(t *testing.T) {
	tests := []struct {
		name      string
		fieldVal  any
		operator  string
		compareVal any
		expected  bool
		expectErr bool
	}{
		// Equal
		{"string equal", "agent", OperatorEqual, "agent", true, false},
		{"string not equal", "agent", OperatorEqual, "human", false, false},
		{"int equal", 5, OperatorEqual, 5, true, false},
		{"float equal", 0.5, OperatorEqual, 0.5, true, false},
		{"bool equal", true, OperatorEqual, true, true, false},

		// Not Equal
		{"string ne", "agent", OperatorNotEqual, "human", true, false},
		{"string ne same", "agent", OperatorNotEqual, "agent", false, false},

		// Greater Than
		{"gt true", 10.0, OperatorGreaterThan, 5.0, true, false},
		{"gt false", 5.0, OperatorGreaterThan, 10.0, false, false},
		{"gt equal", 5.0, OperatorGreaterThan, 5.0, false, false},

		// Less Than
		{"lt true", 5.0, OperatorLessThan, 10.0, true, false},
		{"lt false", 10.0, OperatorLessThan, 5.0, false, false},

		// Greater Or Equal
		{"gte greater", 10.0, OperatorGreaterOrEqual, 5.0, true, false},
		{"gte equal", 5.0, OperatorGreaterOrEqual, 5.0, true, false},
		{"gte less", 3.0, OperatorGreaterOrEqual, 5.0, false, false},

		// Less Or Equal
		{"lte less", 3.0, OperatorLessOrEqual, 5.0, true, false},
		{"lte equal", 5.0, OperatorLessOrEqual, 5.0, true, false},
		{"lte greater", 10.0, OperatorLessOrEqual, 5.0, false, false},

		// In
		{"in found", "agent", OperatorIn, []any{"agent", "human"}, true, false},
		{"in not found", "ci", OperatorIn, []any{"agent", "human"}, false, false},

		// Contains
		{"contains true", "hello world", OperatorContains, "world", true, false},
		{"contains false", "hello world", OperatorContains, "foo", false, false},

		// Matches
		{"matches true", "v1.2.3", OperatorMatches, `v\d+\.\d+\.\d+`, true, false},
		{"matches false", "invalid", OperatorMatches, `v\d+\.\d+\.\d+`, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := compareValues(tt.fieldVal, tt.operator, tt.compareVal)
			if (err != nil) != tt.expectErr {
				t.Errorf("compareValues() error = %v, expectErr %v", err, tt.expectErr)
			}
			if result != tt.expected {
				t.Errorf("compareValues() = %v, want %v", result, tt.expected)
			}
		})
	}
}
