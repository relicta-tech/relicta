package policy

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
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

	// RuleTrace captures per-rule and per-condition evaluation details.
	RuleTrace []RuleTrace
}

// RuleTrace captures the evaluation result for a single rule.
type RuleTrace struct {
	RuleID     string           `json:"rule_id"`
	RuleName   string           `json:"rule_name"`
	Priority   int              `json:"priority"`
	Matched    bool             `json:"matched"`
	Conditions []ConditionTrace `json:"conditions,omitempty"`
}

// ConditionTrace captures the evaluation result for a single condition.
type ConditionTrace struct {
	Field        string `json:"field"`
	Operator     string `json:"operator"`
	Expected     any    `json:"expected,omitempty"`
	Actual       any    `json:"actual,omitempty"`
	Matched      bool   `json:"matched"`
	MissingField bool   `json:"missing_field,omitempty"`
	Error        string `json:"error,omitempty"`
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
		RuleTrace:         []RuleTrace{},
	}

	// No policies means this engine has nothing to contribute, not that the
	// release is approved.
	//
	// The rationale used to read "No policies configured, defaulting to approved",
	// which appears in the audit trail as the first line of the reasoning — so a
	// release whose recorded decision was approval_required carried "defaulting to
	// approved" above the rule that required review. The verdict is the caller's
	// to compose from this and the built-in rules; this line now says only what
	// this engine did.
	if len(e.policies) == 0 {
		result.Decision = cgp.DecisionApproved
		result.Rationale = append(result.Rationale,
			"No policy rules configured; built-in governance rules apply")
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
		matched, trace, err := e.evaluateRuleWithTrace(ctx, rp.rule, evalCtx)
		result.RuleTrace = append(result.RuleTrace, trace)
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

func (e *Engine) evaluateRuleWithTrace(ctx context.Context, rule Rule, evalCtx map[string]any) (bool, RuleTrace, error) {
	trace := RuleTrace{
		RuleID:     rule.ID,
		RuleName:   rule.Name,
		Priority:   rule.Priority,
		Conditions: make([]ConditionTrace, 0, len(rule.Conditions)),
	}

	for _, cond := range rule.Conditions {
		conditionTrace := ConditionTrace{
			Field:    cond.Field,
			Operator: cond.Operator,
			Expected: cond.Value,
		}

		// Composite conditions carry their operands in Value, not in Field, so
		// there is no field to look up. Resolving "_or" as a field path is what
		// made every rule containing OR or NOT silently unmatchable.
		if isCompositeCondition(cond) {
			matched, err := e.evaluateCondition(cond, evalCtx)
			if err != nil {
				conditionTrace.Error = err.Error()
				trace.Conditions = append(trace.Conditions, conditionTrace)
				trace.Matched = false
				return false, trace, err
			}
			conditionTrace.Matched = matched
			trace.Conditions = append(trace.Conditions, conditionTrace)
			if !matched {
				trace.Matched = false
				return false, trace, nil
			}
			continue
		}

		fieldValue, ok := getNestedValue(evalCtx, cond.Field)
		if !ok {
			conditionTrace.Matched = false
			conditionTrace.MissingField = true
			trace.Conditions = append(trace.Conditions, conditionTrace)
			trace.Matched = false
			return false, trace, nil
		}

		conditionTrace.Actual = fieldValue
		matched, err := e.evaluateCondition(cond, evalCtx)
		if err != nil {
			conditionTrace.Error = err.Error()
			conditionTrace.Matched = false
			trace.Conditions = append(trace.Conditions, conditionTrace)
			trace.Matched = false
			return false, trace, err
		}
		conditionTrace.Matched = matched
		trace.Conditions = append(trace.Conditions, conditionTrace)
		if !matched {
			trace.Matched = false
			return false, trace, nil
		}
	}
	trace.Matched = true
	return true, trace, nil
}

// isCompositeCondition reports whether a condition combines other conditions
// rather than comparing a field.
func isCompositeCondition(cond Condition) bool {
	return cond.Field == compositeFieldOr || cond.Field == compositeFieldNot
}

// evaluateCondition checks a single condition.
//
// The DSL compiles `a OR b` into one condition with Field "_or" and the two sides
// in Value, and `NOT a` into Field "_not" with the operand in Value. Nothing here
// understood either, so both were looked up as field paths named "_or" and "_not",
// found missing, and reported as not matched — the documented OR and NOT operators
// produced rules that could never fire, in silence. Four of the five policies this
// project ships used one or the other.
func (e *Engine) evaluateCondition(cond Condition, evalCtx map[string]any) (bool, error) {
	switch cond.Field {
	case compositeFieldOr:
		left, right := orOperands(cond.Value)
		matched, err := e.evaluateConditionGroup(left, evalCtx)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
		return e.evaluateConditionGroup(right, evalCtx)

	case compositeFieldNot:
		matched, err := e.evaluateConditionGroup(operandConditions(cond.Value), evalCtx)
		if err != nil {
			return false, err
		}
		return !matched, nil
	}

	fieldValue, ok := getNestedValue(evalCtx, cond.Field)
	if !ok {
		return false, nil // Field doesn't exist, condition doesn't match
	}

	return compareValues(fieldValue, cond.Operator, cond.Value)
}

// evaluateConditionGroup evaluates a group of conditions with AND semantics,
// matching how a rule's own condition list is evaluated. Operands may themselves
// be composite, so this recurses through evaluateCondition.
//
// An empty group does not match. It can only arise from a malformed composite,
// and treating "no conditions" as vacuously true would make such a rule fire on
// every release — the most dangerous possible reading of a broken policy.
func (e *Engine) evaluateConditionGroup(conds []Condition, evalCtx map[string]any) (bool, error) {
	if len(conds) == 0 {
		return false, nil
	}
	for _, cond := range conds {
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

// orOperands splits an _or condition's Value into its two sides.
func orOperands(value any) (left, right []Condition) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, nil
	}
	return operandConditions(m["left"]), operandConditions(m["right"])
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
			if count, ok := intFromValue(action.Params["count"]); ok {
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
				if count, ok := intFromValue(action.Params["count"]); ok {
					if count > result.RequiredApprovers {
						result.RequiredApprovers = count
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
				if count, ok := intFromValue(action.Params["count"]); ok {
					if count > result.RequiredApprovers {
						result.RequiredApprovers = count
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

func intFromValue(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case uint:
		return int(n), true
	case uint8:
		return int(n), true
	case uint16:
		return int(n), true
	case uint32:
		return int(n), true
	case uint64:
		return int(n), true
	case float32:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

// buildEvalContext creates the context for rule evaluation.
func buildEvalContext(proposal *cgp.ChangeProposal, analysis *cgp.ChangeAnalysis, riskScore float64, timeCtx *TimeContext, teamCtx *TeamContext, actorID string) map[string]any {
	ctx := map[string]any{
		"risk": map[string]any{
			"score": riskScore,
		},
		// Legacy/DSL-friendly aliases used in docs and policy examples.
		"risk_score": riskScore,
	}

	// Add time context if available
	if timeCtx != nil {
		ctx["time"] = timeCtx.ToEvalContext()
		ctx["hour"] = timeCtx.Hour()
		ctx["day_of_week"] = timeCtx.WeekdayNum()
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
		addActorTrust(actorCtx, proposal)
		ctx["actor"] = actorCtx
		ctx["actor_type"] = string(proposal.Actor.Kind)
		ctx["actor_id"] = proposal.Actor.ID
		ctx["intent"] = map[string]any{
			"summary":       proposal.Intent.Summary,
			"suggestedBump": string(proposal.Intent.SuggestedBump),
			"confidence":    proposal.Intent.Confidence,
			"hasBreaking":   proposal.Intent.HasBreakingChanges(),
		}
		ctx["bump_type"] = string(proposal.Intent.SuggestedBump)
		ctx["has_breaking_changes"] = proposal.Intent.HasBreakingChanges()
		ctx["scope"] = map[string]any{
			"repository":  proposal.Scope.Repository,
			"branch":      proposal.Scope.Branch,
			"commitRange": proposal.Scope.CommitRange,
			"fileCount":   len(proposal.Scope.Files),
			// The paths themselves, not only how many. Only the count was exposed,
			// so path-ownership rules — "infrastructure changes need the platform
			// team" — were inexpressible, and the three shipped policies that write
			// them as `change.files contains "terraform/"` matched nothing.
			"files": proposal.Scope.Files,
		}
		ctx["commit_count"] = len(proposal.Scope.Commits)
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
			ctx["files_changed"] = analysis.BlastRadius.FilesChanged
			ctx["lines_changed"] = analysis.BlastRadius.LinesChanged
		}
	}

	return ctx
}

// addActorTrust exposes how much autonomy the actor holds, and the track record
// behind it, to policy conditions.
//
// The trust machinery already existed and decided nothing a policy could see.
// `actor.trusted` was written in this repository's own agent-aware policy — the
// rule meant "auto-approve low-risk changes from actors who have earned it" — and
// the evaluator provided no such field, so the clause was removed rather than
// left looking active (see docs/backlog.md). Meanwhile three independent sources
// raise an actor's trust before evaluation: the governance.trusted_actors
// allowlist, an identity-registry grant, and earned trust from stored release
// outcomes. All three land in proposal.Actor.TrustLevel, which is what this reads,
// so a policy sees the effective trust rather than a fourth notion of it.
//
// trustLevel/trusted are always present: every actor carries a trust level (the
// CLI starts one at Limited), so `trusted == false` means "has not earned or been
// granted it", never "nobody looked".
//
// reputation is different and its absence is load-bearing. It is computed only
// when a memory store is configured AND reputation guarding or earned trust is
// enabled, so exposing 0 when it was not computed would read as a spotless
// record's opposite — a rule like `actor.reputation.overall < 0.5` would fire on
// every actor in a deployment that never computes reputation. The branch is
// therefore omitted entirely, and a condition under it reports a missing field,
// which `relicta policy test --explain` prints.
func addActorTrust(actorCtx map[string]any, proposal *cgp.ChangeProposal) {
	actorCtx["trustLevel"] = proposal.Actor.TrustLevel.String()
	actorCtx["trusted"] = proposal.Actor.TrustLevel.CanAutoApprove()

	if proposal.Context == nil || proposal.Context.ActorReputation == nil {
		return
	}
	rep := proposal.Context.ActorReputation
	actorCtx["reputation"] = map[string]any{
		"overall": rep.Overall,
		"level":   rep.Level,
		"samples": rep.SampleSize,
		"trend":   rep.Trend,
	}
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
// valueContains supports both a string field and a list field.
//
// It handled only string-in-string, so `change.files contains "api/"` — the way
// every path-ownership rule in the shipped example policies is written — returned
// false for a list of changed files, silently. Lists are the natural shape for
// the fields these conditions ask about (changed files, an actor's teams and
// roles), so a `contains` that cannot read one is a `contains` that quietly fails
// on its main use.
//
// For a list, the condition holds when any element contains the search string.
// Substring rather than equality, to match the string case and because the
// intent is `contains "api/"` against paths like "internal/api/handler.go".
func valueContains(fieldValue, searchValue any) bool {
	search, ok := searchValue.(string)
	if !ok {
		return false
	}

	switch field := fieldValue.(type) {
	case string:
		return strings.Contains(field, search)
	case []string:
		for _, item := range field {
			if strings.Contains(item, search) {
				return true
			}
		}
	case []any:
		// The shape a policy takes after a JSON round trip.
		for _, item := range field {
			if s, isStr := item.(string); isStr && strings.Contains(s, search) {
				return true
			}
		}
	}
	return false
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
