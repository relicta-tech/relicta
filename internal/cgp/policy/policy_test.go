package policy

import (
	"strings"
	"testing"
)

func TestNewPolicy(t *testing.T) {
	policy := NewPolicy("test-policy")

	if policy.Name != "test-policy" {
		t.Errorf("NewPolicy().Name = %v, want test-policy", policy.Name)
	}
	if policy.Version != "1.0" {
		t.Errorf("NewPolicy().Version = %v, want 1.0", policy.Version)
	}
	if policy.Defaults.Decision != DecisionRequireReview {
		t.Errorf("NewPolicy().Defaults.Decision = %v, want %v", policy.Defaults.Decision, DecisionRequireReview)
	}
	if policy.Defaults.RequiredApprovers != 1 {
		t.Errorf("NewPolicy().Defaults.RequiredApprovers = %v, want 1", policy.Defaults.RequiredApprovers)
	}
}

func TestPolicy_Validate(t *testing.T) {
	tests := []struct {
		name      string
		policy    *Policy
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid policy",
			policy: &Policy{
				Name: "test",
				Defaults: Defaults{
					Decision: DecisionApprove,
				},
				Rules: []Rule{},
			},
			expectErr: false,
		},
		{
			name: "missing name",
			policy: &Policy{
				Defaults: Defaults{Decision: DecisionApprove},
			},
			expectErr: true,
			errMsg:    "policy name is required",
		},
		{
			name: "missing default decision",
			policy: &Policy{
				Name:     "test",
				Defaults: Defaults{},
			},
			expectErr: true,
			errMsg:    "default decision is required",
		},
		{
			name: "invalid default decision",
			policy: &Policy{
				Name:     "test",
				Defaults: Defaults{Decision: "invalid"},
			},
			expectErr: true,
			errMsg:    "invalid default decision",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("Policy.Validate() error = %v, expectErr %v", err, tt.expectErr)
			}
			if tt.expectErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Policy.Validate() error = %v, should contain %v", err, tt.errMsg)
			}
		})
	}
}

func TestPolicy_AddRule(t *testing.T) {
	policy := NewPolicy("test")
	rule := NewRule("rule-1", "Test Rule")

	policy.AddRule(*rule.
		AddCondition("actor.kind", OperatorEqual, "agent").
		AddAction(ActionRequireApproval, map[string]any{"count": 1}))

	if len(policy.Rules) != 1 {
		t.Errorf("AddRule() should add rule, got %d rules", len(policy.Rules))
	}
	if policy.Rules[0].ID != "rule-1" {
		t.Errorf("AddRule() rule ID = %v, want rule-1", policy.Rules[0].ID)
	}
}

func TestNewRule(t *testing.T) {
	rule := NewRule("rule-1", "Test Rule")

	if rule.ID != "rule-1" {
		t.Errorf("NewRule().ID = %v, want rule-1", rule.ID)
	}
	if rule.Name != "Test Rule" {
		t.Errorf("NewRule().Name = %v, want Test Rule", rule.Name)
	}
	if !rule.Enabled {
		t.Error("NewRule().Enabled should be true by default")
	}
}

func TestRule_Validate(t *testing.T) {
	tests := []struct {
		name      string
		rule      *Rule
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid rule",
			rule: NewRule("rule-1", "Test").
				AddCondition("actor.kind", OperatorEqual, "agent").
				AddAction(ActionSetDecision, map[string]any{"decision": "approve"}),
			expectErr: false,
		},
		{
			name: "missing ID",
			rule: &Rule{
				Name:       "Test",
				Conditions: []Condition{{Field: "x", Operator: OperatorEqual, Value: "y"}},
				Actions:    []Action{{Type: ActionSetDecision}},
			},
			expectErr: true,
			errMsg:    "rule ID is required",
		},
		{
			name: "missing name",
			rule: &Rule{
				ID:         "rule-1",
				Conditions: []Condition{{Field: "x", Operator: OperatorEqual, Value: "y"}},
				Actions:    []Action{{Type: ActionSetDecision}},
			},
			expectErr: true,
			errMsg:    "rule name is required",
		},
		{
			name: "no conditions",
			rule: &Rule{
				ID:      "rule-1",
				Name:    "Test",
				Actions: []Action{{Type: ActionSetDecision}},
			},
			expectErr: true,
			errMsg:    "at least one condition is required",
		},
		{
			name: "no actions",
			rule: &Rule{
				ID:         "rule-1",
				Name:       "Test",
				Conditions: []Condition{{Field: "x", Operator: OperatorEqual, Value: "y"}},
			},
			expectErr: true,
			errMsg:    "at least one action is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("Rule.Validate() error = %v, expectErr %v", err, tt.expectErr)
			}
			if tt.expectErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Rule.Validate() error = %v, should contain %v", err, tt.errMsg)
			}
		})
	}
}

func TestRule_WithPriority(t *testing.T) {
	rule := NewRule("rule-1", "Test").WithPriority(100)

	if rule.Priority != 100 {
		t.Errorf("WithPriority() = %v, want 100", rule.Priority)
	}
}

func TestRule_WithDescription(t *testing.T) {
	rule := NewRule("rule-1", "Test").WithDescription("This is a test rule")

	if rule.Description != "This is a test rule" {
		t.Errorf("WithDescription() = %v, want 'This is a test rule'", rule.Description)
	}
}

func TestCondition_Validate(t *testing.T) {
	tests := []struct {
		name      string
		condition Condition
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid condition",
			condition: Condition{Field: "actor.kind", Operator: OperatorEqual, Value: "agent"},
			expectErr: false,
		},
		{
			name:      "missing field",
			condition: Condition{Operator: OperatorEqual, Value: "agent"},
			expectErr: true,
			errMsg:    "condition field is required",
		},
		{
			name:      "missing operator",
			condition: Condition{Field: "actor.kind", Value: "agent"},
			expectErr: true,
			errMsg:    "condition operator is required",
		},
		{
			name:      "invalid operator",
			condition: Condition{Field: "actor.kind", Operator: "invalid", Value: "agent"},
			expectErr: true,
			errMsg:    "invalid operator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.condition.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("Condition.Validate() error = %v, expectErr %v", err, tt.expectErr)
			}
			if tt.expectErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Condition.Validate() error = %v, should contain %v", err, tt.errMsg)
			}
		})
	}
}

func TestAction_Validate(t *testing.T) {
	tests := []struct {
		name      string
		action    Action
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid action",
			action:    Action{Type: ActionSetDecision, Params: map[string]any{"decision": "approve"}},
			expectErr: false,
		},
		{
			name:      "missing type",
			action:    Action{Params: map[string]any{"decision": "approve"}},
			expectErr: true,
			errMsg:    "action type is required",
		},
		{
			name:      "invalid type",
			action:    Action{Type: "invalid"},
			expectErr: true,
			errMsg:    "invalid action type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.action.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("Action.Validate() error = %v, expectErr %v", err, tt.expectErr)
			}
			if tt.expectErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Action.Validate() error = %v, should contain %v", err, tt.errMsg)
			}
		})
	}
}

func TestIsValidOperator(t *testing.T) {
	validOperators := []string{
		OperatorEqual, OperatorNotEqual, OperatorGreaterThan, OperatorLessThan,
		OperatorGreaterOrEqual, OperatorLessOrEqual, OperatorIn, OperatorContains, OperatorMatches,
	}

	for _, op := range validOperators {
		if !isValidOperator(op) {
			t.Errorf("isValidOperator(%s) = false, want true", op)
		}
	}

	if isValidOperator("invalid") {
		t.Error("isValidOperator(invalid) = true, want false")
	}
}

func TestIsValidAction(t *testing.T) {
	validActions := []string{
		ActionSetDecision, ActionRequireApproval, ActionAddReviewer,
		ActionBlock, ActionAddRationale, ActionAddCondition,
	}

	for _, action := range validActions {
		if !isValidAction(action) {
			t.Errorf("isValidAction(%s) = false, want true", action)
		}
	}

	if isValidAction("invalid") {
		t.Error("isValidAction(invalid) = true, want false")
	}
}

func TestIsValidDecision(t *testing.T) {
	validDecisions := []string{DecisionApprove, DecisionRequireReview, DecisionReject}

	for _, decision := range validDecisions {
		if !isValidDecision(decision) {
			t.Errorf("isValidDecision(%s) = false, want true", decision)
		}
	}

	if isValidDecision("invalid") {
		t.Error("isValidDecision(invalid) = true, want false")
	}
}
