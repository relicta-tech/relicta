package policy

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"github.com/relicta-tech/relicta/internal/cgp"
)

// Engine evaluates policies against proposals.
type Engine struct {
	policies    []Policy
	logger      *slog.Logger
	timeContext *TimeContext
	teamContext *TeamContext
}

// Result contains the outcome of policy evaluation.
type Result struct {
	// Decision is the governance outcome.
	Decision cgp.DecisionType

	// RequiredActions lists actions that must be completed.
	RequiredActions []cgp.RequiredAction

	// Conditions lists constraints on execution.
	Conditions []cgp.Condition

	// MatchedRules lists rule IDs that matched.
	MatchedRules []string

	// Rationale explains the decision.
	Rationale []string

	// RequiredApprovers is the number of approvals needed.
	RequiredApprovers int

	// Reviewers lists required reviewer IDs.
	Reviewers []string

	// Blocked indicates if the change was explicitly blocked.
	Blocked bool

	// BlockReason explains why the change was blocked.
	BlockReason string
}

// ruleWithPolicy pairs a rule with its parent policy.
type ruleWithPolicy struct {
	rule   Rule
	policy Policy
}

// NewEngine creates a policy engine with loaded policies.
func NewEngine(policies []Policy, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{
		policies:    policies,
		logger:      logger,
		timeContext: DefaultTimeContext(),
		teamContext: DefaultTeamContext(),
	}
}

// WithTimeContext sets the time context for policy evaluation.
func (e *Engine) WithTimeContext(tc *TimeContext) *Engine {
	e.timeContext = tc
	return e
}

// SetBusinessHours configures business hours for the engine.
func (e *Engine) SetBusinessHours(config BusinessHoursConfig) *Engine {
	if e.timeContext == nil {
		e.timeContext = DefaultTimeContext()
	}
	e.timeContext.BusinessHours = config
	return e
}

// AddFreezePeriod adds a freeze period to the engine.
func (e *Engine) AddFreezePeriod(freeze FreezePeriod) *Engine {
	if e.timeContext == nil {
		e.timeContext = DefaultTimeContext()
	}
	e.timeContext.FreezePeriods = append(e.timeContext.FreezePeriods, freeze)
	return e
}

// WithTeamContext sets the team context for policy evaluation.
func (e *Engine) WithTeamContext(tc *TeamContext) *Engine {
	e.teamContext = tc
	return e
}

// AddTeam adds a team to the engine's team context.
func (e *Engine) AddTeam(team *Team) *Engine {
	if e.teamContext == nil {
		e.teamContext = DefaultTeamContext()
	}
	e.teamContext.AddTeam(team)
	return e
}

// AddRole adds a role to the engine's team context.
func (e *Engine) AddRole(role *Role) *Engine {
	if e.teamContext == nil {
		e.teamContext = DefaultTeamContext()
	}
	e.teamContext.AddRole(role)
	return e
}

// AssignActorRole assigns a role to an actor.
func (e *Engine) AssignActorRole(actorID, roleName string) *Engine {
	if e.teamContext == nil {
		e.teamContext = DefaultTeamContext()
	}
	e.teamContext.AssignRole(actorID, roleName)
	return e
}

// AddPolicy adds a policy to the engine.
func (e *Engine) AddPolicy(policy Policy) {
	e.policies = append(e.policies, policy)
}

// Evaluate runs all policies against a proposal and analysis.
func (e *Engine) Evaluate(ctx context.Context, proposal *cgp.ChangeProposal, analysis *cgp.ChangeAnalysis, riskScore float64) (*Result, error) {
	result := &Result{
		Decision:          cgp.DecisionApproved,
		RequiredActions:   []cgp.RequiredAction{},
		Conditions:        []cgp.Condition{},
		MatchedRules:      []string{},
		Rationale:         []string{},
		RequiredApprovers: 0,
		Reviewers:         []string{},
	}

	// If no policies, use defaults
	if len(e.policies) == 0 {
		result.Decision = cgp.DecisionApproved
		result.Rationale = append(result.Rationale, "No policies configured, defaulting to approved")
		return result, nil
	}

	// Collect all rules from all policies, sorted by priority
	var allRules []ruleWithPolicy
	for _, policy := range e.policies {
		for _, rule := range policy.Rules {
			if rule.Enabled {
				allRules = append(allRules, ruleWithPolicy{
					rule:   rule,
					policy: policy,
				})
			}
		}
	}
	sort.Slice(allRules, func(i, j int) bool {
		return allRules[i].rule.Priority > allRules[j].rule.Priority
	})

	// Get actor ID for team context
	var actorID string
	if proposal != nil {
		actorID = proposal.Actor.ID
	}

	// Build evaluation context
	evalCtx := buildEvalContext(proposal, analysis, riskScore, e.timeContext, e.teamContext, actorID)

	// Evaluate each rule
	for _, rp := range allRules {
		matched, err := e.evaluateRule(ctx, rp.rule, evalCtx)
		if err != nil {
			e.logger.Warn("rule evaluation failed",
				"rule", rp.rule.ID,
				"error", err,
			)
			continue
		}

		if matched {
			result.MatchedRules = append(result.MatchedRules, rp.rule.ID)
			e.applyActions(result, rp.rule.Actions, e.teamContext)
			if rp.rule.Description != "" {
				result.Rationale = append(result.Rationale,
					fmt.Sprintf("Rule '%s': %s", rp.rule.Name, rp.rule.Description))
			}
		}
	}

	// Apply defaults from first policy if no rules blocked or set decision
	if !result.Blocked && len(result.MatchedRules) == 0 && len(e.policies) > 0 {
		defaults := e.policies[0].Defaults
		switch defaults.Decision {
		case DecisionApprove:
			result.Decision = cgp.DecisionApproved
		case DecisionRequireReview:
			result.Decision = cgp.DecisionApprovalRequired
		case DecisionReject:
			result.Decision = cgp.DecisionRejected
		}
		result.RequiredApprovers = defaults.RequiredApprovers
		result.Rationale = append(result.Rationale, "Applied default policy")
	}

	// Convert blocked to rejected
	if result.Blocked {
		result.Decision = cgp.DecisionRejected
		if result.BlockReason != "" {
			result.Rationale = append(result.Rationale, result.BlockReason)
		}
	}

	return result, nil
}

// evaluateRule checks if all conditions match.
func (e *Engine) evaluateRule(ctx context.Context, rule Rule, evalCtx map[string]any) (bool, error) {
	for _, cond := range rule.Conditions {
		matched, err := e.evaluateCondition(cond, evalCtx)
		if err != nil {
			return false, err
		}
		if !matched {
			return false, nil
		}
	}
	return true, nil
}

// evaluateCondition checks a single condition.
func (e *Engine) evaluateCondition(cond Condition, evalCtx map[string]any) (bool, error) {
	fieldValue, ok := getNestedValue(evalCtx, cond.Field)
	if !ok {
		return false, nil // Field doesn't exist, condition doesn't match
	}

	return compareValues(fieldValue, cond.Operator, cond.Value)
}

// applyActions applies rule actions to the result.
func (e *Engine) applyActions(result *Result, actions []Action, teamCtx *TeamContext) {
	for _, action := range actions {
		switch action.Type {
		case ActionSetDecision:
			if decision, ok := action.Params["decision"].(string); ok {
				switch decision {
				case "approve", "approved":
					result.Decision = cgp.DecisionApproved
				case "require_review", "approval_required":
					result.Decision = cgp.DecisionApprovalRequired
				case "reject", "rejected":
					result.Decision = cgp.DecisionRejected
				case "defer", "deferred":
					result.Decision = cgp.DecisionDeferred
				}
			}

		case ActionRequireApproval:
			result.Decision = cgp.DecisionApprovalRequired
			if count, ok := action.Params["count"].(int); ok {
				if count > result.RequiredApprovers {
					result.RequiredApprovers = count
				}
			} else if countFloat, ok := action.Params["count"].(float64); ok {
				count := int(countFloat)
				if count > result.RequiredApprovers {
					result.RequiredApprovers = count
				}
			}
			if desc, ok := action.Params["description"].(string); ok {
				result.RequiredActions = append(result.RequiredActions, cgp.RequiredAction{
					Type:        "human_approval",
					Description: desc,
				})
			}

		case ActionAddReviewer:
			if reviewer, ok := action.Params["reviewer"].(string); ok {
				result.Reviewers = append(result.Reviewers, reviewer)
			}
			if reviewers, ok := action.Params["reviewers"].([]string); ok {
				result.Reviewers = append(result.Reviewers, reviewers...)
			}

		case ActionBlock:
			result.Blocked = true
			if reason, ok := action.Params["reason"].(string); ok {
				result.BlockReason = reason
			}

		case ActionAddRationale:
			if rationale, ok := action.Params["message"].(string); ok {
				result.Rationale = append(result.Rationale, rationale)
			}

		case ActionAddCondition:
			if condType, ok := action.Params["type"].(string); ok {
				condValue, _ := action.Params["value"].(string)
				result.Conditions = append(result.Conditions, cgp.Condition{
					Type:  condType,
					Value: condValue,
				})
			}

		case ActionRequireTeamReview:
			result.Decision = cgp.DecisionApprovalRequired
			if teamName, ok := action.Params["team"].(string); ok {
				// Add team members as required reviewers
				if teamCtx != nil {
					members := teamCtx.GetTeamMembers(teamName)
					result.Reviewers = append(result.Reviewers, members...)
				}
				// Set minimum approvers from team
				if count, ok := action.Params["count"].(float64); ok {
					if int(count) > result.RequiredApprovers {
						result.RequiredApprovers = int(count)
					}
				} else {
					// Default to 1 team member
					if result.RequiredApprovers < 1 {
						result.RequiredApprovers = 1
					}
				}
				result.RequiredActions = append(result.RequiredActions, cgp.RequiredAction{
					Type:        "team_approval",
					Description: fmt.Sprintf("Requires approval from team '%s'", teamName),
				})
			}

		case ActionRequireRoleReview:
			result.Decision = cgp.DecisionApprovalRequired
			if roleName, ok := action.Params["role"].(string); ok {
				// Add actors with this role as required reviewers
				if teamCtx != nil {
					for actorID, roles := range teamCtx.ActorRoles {
						for _, r := range roles {
							if r == roleName {
								result.Reviewers = append(result.Reviewers, actorID)
								break
							}
						}
					}
				}
				// Set minimum approvers
				if count, ok := action.Params["count"].(float64); ok {
					if int(count) > result.RequiredApprovers {
						result.RequiredApprovers = int(count)
					}
				} else {
					if result.RequiredApprovers < 1 {
						result.RequiredApprovers = 1
					}
				}
				result.RequiredActions = append(result.RequiredActions, cgp.RequiredAction{
					Type:        "role_approval",
					Description: fmt.Sprintf("Requires approval from role '%s'", roleName),
				})
			}

		case ActionRequireTeamLead:
			result.Decision = cgp.DecisionApprovalRequired
			if teamName, ok := action.Params["team"].(string); ok {
				// Add team leads as required reviewers
				if teamCtx != nil {
					leads := teamCtx.GetTeamLeads(teamName)
					result.Reviewers = append(result.Reviewers, leads...)
				}
				if result.RequiredApprovers < 1 {
					result.RequiredApprovers = 1
				}
				result.RequiredActions = append(result.RequiredActions, cgp.RequiredAction{
					Type:        "team_lead_approval",
					Description: fmt.Sprintf("Requires approval from lead of team '%s'", teamName),
				})
			}
		}
	}
}

// buildEvalContext creates the context for rule evaluation.
func buildEvalContext(proposal *cgp.ChangeProposal, analysis *cgp.ChangeAnalysis, riskScore float64, timeCtx *TimeContext, teamCtx *TeamContext, actorID string) map[string]any {
	ctx := map[string]any{
		"risk": map[string]any{
			"score": riskScore,
		},
	}

	// Add time context if available
	if timeCtx != nil {
		ctx["time"] = timeCtx.ToEvalContext()
	}

	// Add team context if available
	if teamCtx != nil {
		ctx["team"] = teamCtx.ToEvalContext(actorID)
	}

	// Actor context
	if proposal != nil {
		actorCtx := map[string]any{
			"kind": string(proposal.Actor.Kind),
			"id":   proposal.Actor.ID,
			"name": proposal.Actor.Name,
		}
		// Add team/role info to actor context
		if teamCtx != nil {
			actorCtx["teams"] = teamCtx.GetActorTeams(actorID)
			actorCtx["roles"] = teamCtx.GetActorRoles(actorID)
			actorCtx["canApprove"] = teamCtx.CanApprove(actorID)
			actorCtx["canPublish"] = teamCtx.CanPublish(actorID)
			actorCtx["isTeamLead"] = teamCtx.isAnyTeamLead(actorID)
		}
		ctx["actor"] = actorCtx
		ctx["intent"] = map[string]any{
			"summary":       proposal.Intent.Summary,
			"suggestedBump": string(proposal.Intent.SuggestedBump),
			"confidence":    proposal.Intent.Confidence,
			"hasBreaking":   proposal.Intent.HasBreakingChanges(),
		}
		ctx["scope"] = map[string]any{
			"repository":  proposal.Scope.Repository,
			"branch":      proposal.Scope.Branch,
			"commitRange": proposal.Scope.CommitRange,
			"fileCount":   len(proposal.Scope.Files),
		}
	}

	// Analysis context
	if analysis != nil {
		ctx["change"] = map[string]any{
			"features":     analysis.Features,
			"fixes":        analysis.Fixes,
			"breaking":     analysis.Breaking,
			"security":     analysis.Security,
			"dependencies": analysis.Dependencies,
			"other":        analysis.Other,
			"total":        analysis.TotalChanges(),
			"hasAPIChange": analysis.HasAPIChanges(),
		}
		if analysis.BlastRadius != nil {
			ctx["blastRadius"] = map[string]any{
				"score":        analysis.BlastRadius.Score,
				"filesChanged": analysis.BlastRadius.FilesChanged,
				"linesChanged": analysis.BlastRadius.LinesChanged,
			}
		}
	}

	return ctx
}

// getNestedValue retrieves a value from a nested map using dot notation.
func getNestedValue(data map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	current := any(data)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[part]
			if !ok {
				return nil, false
			}
			current = val
		default:
			return nil, false
		}
	}

	return current, true
}

// compareValues compares two values using the given operator.
func compareValues(fieldValue any, operator string, compareValue any) (bool, error) {
	switch operator {
	case OperatorEqual:
		return valuesEqual(fieldValue, compareValue), nil

	case OperatorNotEqual:
		return !valuesEqual(fieldValue, compareValue), nil

	case OperatorGreaterThan:
		return compareNumeric(fieldValue, compareValue, func(a, b float64) bool { return a > b })

	case OperatorLessThan:
		return compareNumeric(fieldValue, compareValue, func(a, b float64) bool { return a < b })

	case OperatorGreaterOrEqual:
		return compareNumeric(fieldValue, compareValue, func(a, b float64) bool { return a >= b })

	case OperatorLessOrEqual:
		return compareNumeric(fieldValue, compareValue, func(a, b float64) bool { return a <= b })

	case OperatorIn:
		return valueIn(fieldValue, compareValue), nil

	case OperatorContains:
		return valueContains(fieldValue, compareValue), nil

	case OperatorMatches:
		return valueMatches(fieldValue, compareValue)

	default:
		return false, fmt.Errorf("unsupported operator: %s", operator)
	}
}

// valuesEqual compares two values for equality.
func valuesEqual(a, b any) bool {
	// Handle type conversions for common cases
	switch av := a.(type) {
	case string:
		if bv, ok := b.(string); ok {
			return av == bv
		}
	case int:
		return compareIntEqual(av, b)
	case int64:
		return compareIntEqual(int(av), b)
	case float64:
		return compareFloatEqual(av, b)
	case bool:
		if bv, ok := b.(bool); ok {
			return av == bv
		}
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func compareIntEqual(a int, b any) bool {
	switch bv := b.(type) {
	case int:
		return a == bv
	case int64:
		return int64(a) == bv
	case float64:
		return float64(a) == bv
	}
	return false
}

func compareFloatEqual(a float64, b any) bool {
	switch bv := b.(type) {
	case float64:
		return a == bv
	case int:
		return a == float64(bv)
	case int64:
		return a == float64(bv)
	}
	return false
}

// compareNumeric compares two numeric values.
func compareNumeric(a, b any, cmp func(float64, float64) bool) (bool, error) {
	av, ok := toFloat64(a)
	if !ok {
		return false, fmt.Errorf("cannot convert %v to number", a)
	}
	bv, ok := toFloat64(b)
	if !ok {
		return false, fmt.Errorf("cannot convert %v to number", b)
	}
	return cmp(av, bv), nil
}

// toFloat64 converts a value to float64.
func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	default:
		return 0, false
	}
}

// valueIn checks if a value is in a list.
func valueIn(fieldValue, listValue any) bool {
	switch list := listValue.(type) {
	case []any:
		for _, item := range list {
			if valuesEqual(fieldValue, item) {
				return true
			}
		}
	case []string:
		if str, ok := fieldValue.(string); ok {
			for _, item := range list {
				if str == item {
					return true
				}
			}
		}
	}
	return false
}

// valueContains checks if a string contains a substring.
func valueContains(fieldValue, searchValue any) bool {
	str, ok := fieldValue.(string)
	if !ok {
		return false
	}
	search, ok := searchValue.(string)
	if !ok {
		return false
	}
	return strings.Contains(str, search)
}

// valueMatches checks if a string matches a regex pattern.
func valueMatches(fieldValue, pattern any) (bool, error) {
	str, ok := fieldValue.(string)
	if !ok {
		return false, nil
	}
	pat, ok := pattern.(string)
	if !ok {
		return false, fmt.Errorf("pattern must be a string")
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return false, fmt.Errorf("invalid regex pattern: %w", err)
	}
	return re.MatchString(str), nil
}
