// Package policy implements the CGP policy engine for release governance.
//
// The policy engine evaluates organizational rules against change proposals
// to determine governance decisions. Policies are defined declaratively and
// can be loaded from YAML or JSON configuration files.
package policy

import (
	"fmt"
)

// Policy defines organizational release governance rules.
type Policy struct {
	// Version is the policy schema version.
	Version string `json:"version" yaml:"version"`

	// Name is a unique identifier for this policy.
	Name string `json:"name" yaml:"name"`

	// Description explains the policy's purpose.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Rules define the governance rules in priority order.
	Rules []Rule `json:"rules" yaml:"rules"`

	// Defaults specify behavior when no rules match.
	Defaults Defaults `json:"defaults" yaml:"defaults"`
}

// Defaults specifies default behavior when no rules match.
type Defaults struct {
	// Decision is the default governance outcome: "approve", "require_review", "reject".
	Decision string `json:"decision" yaml:"decision"`

	// RequiredApprovers is the minimum number of approvals required.
	RequiredApprovers int `json:"requiredApprovers" yaml:"requiredApprovers"`

	// AllowedActors lists actor IDs or patterns permitted to propose changes.
	AllowedActors []string `json:"allowedActors,omitempty" yaml:"allowedActors,omitempty"`
}

// Rule defines a single governance rule.
type Rule struct {
	// ID is a unique identifier for this rule.
	ID string `json:"id" yaml:"id"`

	// Name is a human-readable name for the rule.
	Name string `json:"name" yaml:"name"`

	// Description explains what this rule does.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Priority determines evaluation order (higher = evaluated first).
	Priority int `json:"priority" yaml:"priority"`

	// Enabled controls whether this rule is active.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Conditions define when this rule applies.
	Conditions []Condition `json:"conditions" yaml:"conditions"`

	// Actions define what happens when the rule matches.
	Actions []Action `json:"actions" yaml:"actions"`
}

// Condition defines when a rule applies.
type Condition struct {
	// Field is the path to evaluate: "actor.kind", "risk.score", "change.breaking", etc.
	Field string `json:"field" yaml:"field"`

	// Operator is the comparison operator: "eq", "ne", "gt", "lt", "gte", "lte", "in", "contains", "matches".
	Operator string `json:"operator" yaml:"operator"`

	// Value is the comparison value.
	Value any `json:"value" yaml:"value"`
}

// Action defines what happens when a rule matches.
type Action struct {
	// Type is the action to perform: "set_decision", "require_approval", "add_reviewer", "block", "add_rationale".
	Type string `json:"type" yaml:"type"`

	// Params contains action-specific parameters.
	Params map[string]any `json:"params,omitempty" yaml:"params,omitempty"`
}

// ActionType constants define supported actions.
const (
	ActionSetDecision       = "set_decision"
	ActionRequireApproval   = "require_approval"
	ActionAddReviewer       = "add_reviewer"
	ActionBlock             = "block"
	ActionAddRationale      = "add_rationale"
	ActionAddCondition      = "add_condition"
	ActionRequireTeamReview = "require_team_review"
	ActionRequireRoleReview = "require_role_review"
	ActionRequireTeamLead   = "require_team_lead"
)

// OperatorType constants define supported operators.
const (
	OperatorEqual          = "eq"
	OperatorNotEqual       = "ne"
	OperatorGreaterThan    = "gt"
	OperatorLessThan       = "lt"
	OperatorGreaterOrEqual = "gte"
	OperatorLessOrEqual    = "lte"
	OperatorIn             = "in"
	OperatorContains       = "contains"
	OperatorMatches        = "matches"
)

// DecisionType constants for policy decisions.
const (
	DecisionApprove       = "approve"
	DecisionRequireReview = "require_review"
	DecisionReject        = "reject"
)

// Validate checks if the policy is valid.
func (p *Policy) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("policy name is required")
	}
	if p.Defaults.Decision == "" {
		return fmt.Errorf("default decision is required")
	}
	if !isValidDecision(p.Defaults.Decision) {
		return fmt.Errorf("invalid default decision: %s", p.Defaults.Decision)
	}

	for i, rule := range p.Rules {
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("invalid rule at index %d: %w", i, err)
		}
	}
	return nil
}

// Validate checks if the rule is valid.
func (r *Rule) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("rule ID is required")
	}
	if r.Name == "" {
		return fmt.Errorf("rule name is required")
	}
	if len(r.Conditions) == 0 {
		return fmt.Errorf("at least one condition is required")
	}
	if len(r.Actions) == 0 {
		return fmt.Errorf("at least one action is required")
	}

	for i, cond := range r.Conditions {
		if err := cond.Validate(); err != nil {
			return fmt.Errorf("invalid condition at index %d: %w", i, err)
		}
	}
	for i, action := range r.Actions {
		if err := action.Validate(); err != nil {
			return fmt.Errorf("invalid action at index %d: %w", i, err)
		}
	}
	return nil
}

// Validate checks if the condition is valid.
func (c *Condition) Validate() error {
	if c.Field == "" {
		return fmt.Errorf("condition field is required")
	}
	if c.Operator == "" {
		return fmt.Errorf("condition operator is required")
	}
	if !isValidOperator(c.Operator) {
		return fmt.Errorf("invalid operator: %s", c.Operator)
	}
	return nil
}

// Validate checks if the action is valid.
func (a *Action) Validate() error {
	if a.Type == "" {
		return fmt.Errorf("action type is required")
	}
	if !isValidAction(a.Type) {
		return fmt.Errorf("invalid action type: %s", a.Type)
	}
	return nil
}

// Helper functions

func isValidDecision(d string) bool {
	switch d {
	case DecisionApprove, DecisionRequireReview, DecisionReject:
		return true
	default:
		return false
	}
}

func isValidOperator(op string) bool {
	switch op {
	case OperatorEqual, OperatorNotEqual, OperatorGreaterThan, OperatorLessThan,
		OperatorGreaterOrEqual, OperatorLessOrEqual, OperatorIn, OperatorContains, OperatorMatches:
		return true
	default:
		return false
	}
}

func isValidAction(a string) bool {
	switch a {
	case ActionSetDecision, ActionRequireApproval, ActionAddReviewer,
		ActionBlock, ActionAddRationale, ActionAddCondition,
		ActionRequireTeamReview, ActionRequireRoleReview, ActionRequireTeamLead:
		return true
	default:
		return false
	}
}

// NewPolicy creates a new policy with defaults.
func NewPolicy(name string) *Policy {
	return &Policy{
		Version: "1.0",
		Name:    name,
		Rules:   []Rule{},
		Defaults: Defaults{
			Decision:          DecisionRequireReview,
			RequiredApprovers: 1,
		},
	}
}

// AddRule adds a rule to the policy.
func (p *Policy) AddRule(rule Rule) *Policy {
	p.Rules = append(p.Rules, rule)
	return p
}

// NewRule creates a new rule with the given ID and name.
func NewRule(id, name string) *Rule {
	return &Rule{
		ID:         id,
		Name:       name,
		Priority:   0,
		Enabled:    true,
		Conditions: []Condition{},
		Actions:    []Action{},
	}
}

// WithPriority sets the rule priority.
func (r *Rule) WithPriority(priority int) *Rule {
	r.Priority = priority
	return r
}

// WithDescription sets the rule description.
func (r *Rule) WithDescription(desc string) *Rule {
	r.Description = desc
	return r
}

// AddCondition adds a condition to the rule.
func (r *Rule) AddCondition(field, operator string, value any) *Rule {
	r.Conditions = append(r.Conditions, Condition{
		Field:    field,
		Operator: operator,
		Value:    value,
	})
	return r
}

// AddAction adds an action to the rule.
func (r *Rule) AddAction(actionType string, params map[string]any) *Rule {
	r.Actions = append(r.Actions, Action{
		Type:   actionType,
		Params: params,
	})
	return r
}
