package autoapproval

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/risk"
)

// Decision represents an auto-approval decision.
type Decision string

// Decision types.
const (
	DecisionAutoApproved  Decision = "auto_approved"  // Automatically approved
	DecisionRequireReview Decision = "require_review" // Needs human review
	DecisionAutoRejected  Decision = "auto_rejected"  // Automatically rejected
	DecisionDeferred      Decision = "deferred"       // Deferred for later
)

// Result contains the auto-approval evaluation outcome.
type Result struct {
	// Decision is the auto-approval outcome.
	Decision Decision `json:"decision"`

	// CGPDecision is the corresponding CGP decision type.
	CGPDecision cgp.DecisionType `json:"cgpDecision"`

	// Approved indicates if the change was auto-approved.
	Approved bool `json:"approved"`

	// MatchedPolicy is the policy that triggered auto-approval (if any).
	MatchedPolicy *AutoApprovalPolicy `json:"matchedPolicy,omitempty"`

	// Rationale explains why the decision was made.
	Rationale []string `json:"rationale"`

	// ExemptionHits lists exemptions that prevented auto-approval.
	ExemptionHits []string `json:"exemptionHits,omitempty"`

	// FailedConditions lists policy conditions that failed.
	FailedConditions []FailedCondition `json:"failedConditions,omitempty"`

	// RequiredApprovers is the number of approvers needed.
	RequiredApprovers int `json:"requiredApprovers,omitempty"`

	// RiskScore is the evaluated risk score.
	RiskScore float64 `json:"riskScore"`

	// EvaluatedAt is when the evaluation occurred.
	EvaluatedAt time.Time `json:"evaluatedAt"`

	// Duration is how long the evaluation took.
	Duration time.Duration `json:"duration"`

	// AuditID is a unique identifier for audit purposes.
	AuditID string `json:"auditId"`
}

// FailedCondition records a condition that didn't match.
type FailedCondition struct {
	PolicyID    string          `json:"policyId"`
	PolicyName  string          `json:"policyName"`
	Condition   PolicyCondition `json:"condition"`
	ActualValue any             `json:"actualValue,omitempty"`
	Reason      string          `json:"reason"`
}

// Evaluator evaluates auto-approval policies.
type Evaluator struct {
	config *Config
	logger *slog.Logger
}

// Option configures the evaluator.
type Option func(*Evaluator)

// WithLogger sets the logger.
func WithLogger(logger *slog.Logger) Option {
	return func(e *Evaluator) {
		e.logger = logger
	}
}

// WithConfig sets the configuration.
func WithConfig(cfg *Config) Option {
	return func(e *Evaluator) {
		e.config = cfg
	}
}

// New creates a new auto-approval evaluator.
func New(opts ...Option) *Evaluator {
	e := &Evaluator{
		config: DefaultConfig(),
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Evaluate determines if a proposal can be auto-approved.
func (e *Evaluator) Evaluate(
	ctx context.Context,
	proposal *cgp.ChangeProposal,
	analysis *cgp.ChangeAnalysis,
	riskAssessment *risk.Assessment,
) (*Result, error) {
	startTime := time.Now()

	result := &Result{
		Decision:    DecisionRequireReview,
		CGPDecision: cgp.DecisionApprovalRequired,
		Approved:    false,
		Rationale:   []string{},
		EvaluatedAt: startTime,
		AuditID:     generateAuditID(),
	}

	if riskAssessment != nil {
		result.RiskScore = riskAssessment.Score
	}

	// Check if auto-approval is enabled
	if !e.config.Enabled {
		result.Rationale = append(result.Rationale, "Auto-approval is disabled")
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Check risk thresholds first
	if riskAssessment != nil {
		// Auto-reject if risk is too high
		if riskAssessment.Score >= e.config.Thresholds.RejectMin {
			result.Decision = DecisionAutoRejected
			result.CGPDecision = cgp.DecisionRejected
			result.Rationale = append(result.Rationale,
				fmt.Sprintf("Risk score %.2f exceeds rejection threshold %.2f",
					riskAssessment.Score, e.config.Thresholds.RejectMin))
			result.Duration = time.Since(startTime)
			return result, nil
		}

		// Require review if risk is above auto-approve threshold
		if riskAssessment.Score >= e.config.Thresholds.RequireReviewMin {
			result.RequiredApprovers = e.getApproverCount(riskAssessment.Score)
			result.Rationale = append(result.Rationale,
				fmt.Sprintf("Risk score %.2f requires human review (threshold: %.2f)",
					riskAssessment.Score, e.config.Thresholds.RequireReviewMin))
			result.Duration = time.Since(startTime)
			return result, nil
		}
	}

	// Check exemptions
	exemptions := e.checkExemptions(proposal, analysis)
	if len(exemptions) > 0 {
		result.ExemptionHits = exemptions
		result.Rationale = append(result.Rationale, "Exemptions prevent auto-approval:")
		result.Rationale = append(result.Rationale, exemptions...)
		result.RequiredApprovers = e.getApproverCount(result.RiskScore)
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Check actor rules
	if proposal != nil {
		actorBlock, actorReason := e.checkActorRules(proposal.Actor)
		if actorBlock {
			result.Rationale = append(result.Rationale, actorReason)
			result.Duration = time.Since(startTime)
			return result, nil
		}
	}

	// Evaluate policies
	evalCtx := e.buildEvalContext(proposal, analysis, riskAssessment)
	matchedPolicy, failedConditions := e.evaluatePolicies(evalCtx)

	if matchedPolicy != nil {
		// Check time constraints
		if matchedPolicy.TimeConstraints != nil {
			blocked, reason := e.checkTimeConstraints(matchedPolicy.TimeConstraints)
			if blocked {
				result.Rationale = append(result.Rationale, reason)
				result.Duration = time.Since(startTime)
				return result, nil
			}
		}

		// Check actor constraints
		if matchedPolicy.ActorConstraints != nil && proposal != nil {
			blocked, reason := e.checkActorConstraints(matchedPolicy.ActorConstraints, proposal.Actor)
			if blocked {
				result.Rationale = append(result.Rationale, reason)
				result.Duration = time.Since(startTime)
				return result, nil
			}
		}

		// Auto-approve
		result.Decision = DecisionAutoApproved
		result.CGPDecision = cgp.DecisionApproved
		result.Approved = true
		result.MatchedPolicy = matchedPolicy
		result.Rationale = append(result.Rationale,
			fmt.Sprintf("Auto-approved by policy '%s': %s", matchedPolicy.Name, matchedPolicy.Description))

		e.logger.Info("auto-approved",
			"policy_id", matchedPolicy.ID,
			"risk_score", result.RiskScore,
			"audit_id", result.AuditID,
		)
	} else {
		result.FailedConditions = failedConditions
		result.RequiredApprovers = e.getApproverCount(result.RiskScore)
		result.Rationale = append(result.Rationale, "No auto-approval policy matched")
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// checkExemptions checks if any exemptions apply.
func (e *Evaluator) checkExemptions(proposal *cgp.ChangeProposal, analysis *cgp.ChangeAnalysis) []string {
	var exemptions []string

	if analysis == nil {
		return exemptions
	}

	// Breaking changes
	if e.config.Exemptions.BreakingChanges && analysis.Breaking > 0 {
		exemptions = append(exemptions,
			fmt.Sprintf("Breaking changes detected (%d)", analysis.Breaking))
	}

	// Security changes
	if e.config.Exemptions.SecurityChanges && analysis.Security > 0 {
		exemptions = append(exemptions,
			fmt.Sprintf("Security-related changes detected (%d)", analysis.Security))
	}

	// Major versions
	if e.config.Exemptions.MajorVersions && proposal != nil {
		if strings.ToLower(string(proposal.Intent.SuggestedBump)) == "major" {
			exemptions = append(exemptions, "Major version bump requires review")
		}
	}

	// New dependencies
	if e.config.Exemptions.NewDependencies && analysis.Dependencies > 0 {
		exemptions = append(exemptions,
			fmt.Sprintf("New dependencies added (%d)", analysis.Dependencies))
	}

	// Large changes
	if e.config.Exemptions.LargeChanges != nil && analysis.BlastRadius != nil {
		lc := e.config.Exemptions.LargeChanges
		if lc.MaxFiles > 0 && analysis.BlastRadius.FilesChanged > lc.MaxFiles {
			exemptions = append(exemptions,
				fmt.Sprintf("Too many files changed (%d > %d)",
					analysis.BlastRadius.FilesChanged, lc.MaxFiles))
		}
		if lc.MaxLines > 0 && analysis.BlastRadius.LinesChanged > lc.MaxLines {
			exemptions = append(exemptions,
				fmt.Sprintf("Too many lines changed (%d > %d)",
					analysis.BlastRadius.LinesChanged, lc.MaxLines))
		}
	}

	// Protected branches
	if proposal != nil && len(e.config.Exemptions.ProtectedBranches) > 0 {
		for _, branch := range e.config.Exemptions.ProtectedBranches {
			if matchPattern(proposal.Scope.Branch, branch) {
				exemptions = append(exemptions,
					fmt.Sprintf("Branch '%s' is protected", proposal.Scope.Branch))
				break
			}
		}
	}

	// Affected components
	if analysis.BlastRadius != nil && len(e.config.Exemptions.AffectedComponents) > 0 {
		for _, component := range analysis.BlastRadius.Components {
			for _, protected := range e.config.Exemptions.AffectedComponents {
				if matchPattern(component, protected) {
					exemptions = append(exemptions,
						fmt.Sprintf("Protected component affected: %s", component))
				}
			}
		}
	}

	return exemptions
}

// checkActorRules checks actor-specific rules.
func (e *Evaluator) checkActorRules(actor cgp.Actor) (blocked bool, reason string) {
	// Check if actor is in trusted list (bypass all checks)
	for _, trustedID := range e.config.ActorRules.TrustedActors {
		if actor.ID == trustedID {
			return false, ""
		}
	}

	// Check agent rules
	if actor.Kind == cgp.ActorKindAgent {
		if !e.config.ActorRules.AgentAutoApprove {
			return true, "Agent-initiated changes require human review"
		}
		if e.config.ActorRules.RequireHumanForAgentChanges {
			return true, "Agent changes require human approval by policy"
		}
	}

	// Check CI rules
	if actor.Kind == cgp.ActorKindCI && !e.config.ActorRules.CIAutoApprove {
		return true, "CI-initiated changes require human review"
	}

	return false, ""
}

// checkTimeConstraints checks if time constraints allow auto-approval.
func (e *Evaluator) checkTimeConstraints(tc *TimeConstraints) (blocked bool, reason string) {
	now := time.Now()

	// Check business hours
	if tc.BusinessHoursOnly {
		hour := now.Hour()
		weekday := int(now.Weekday())
		if hour < 9 || hour >= 18 || weekday == 0 || weekday == 6 {
			return true, "Auto-approval only allowed during business hours"
		}
	}

	// Check allowed days
	if len(tc.AllowedDays) > 0 {
		weekday := int(now.Weekday())
		allowed := false
		for _, d := range tc.AllowedDays {
			if d == weekday {
				allowed = true
				break
			}
		}
		if !allowed {
			return true, fmt.Sprintf("Auto-approval not allowed on %s", now.Weekday().String())
		}
	}

	// Check blocked dates
	today := now.Format("2006-01-02")
	for _, blocked := range tc.BlockedDates {
		if blocked == today {
			return true, fmt.Sprintf("Auto-approval blocked on %s", today)
		}
	}

	// Check freeze windows
	for _, freeze := range tc.FreezeWindows {
		if now.After(freeze.Start) && now.Before(freeze.End) {
			return true, fmt.Sprintf("Freeze window '%s': %s", freeze.Name, freeze.Reason)
		}
	}

	return false, ""
}

// checkActorConstraints checks if actor constraints allow auto-approval.
func (e *Evaluator) checkActorConstraints(ac *ActorConstraints, actor cgp.Actor) (blocked bool, reason string) {
	// Check blocked actors
	for _, blockedID := range ac.BlockedActorIDs {
		if actor.ID == blockedID {
			return true, fmt.Sprintf("Actor %s is blocked from auto-approval", actor.ID)
		}
	}

	// Check allowed actors (if specified, must be in list)
	if len(ac.AllowedActorIDs) > 0 {
		allowed := false
		for _, allowedID := range ac.AllowedActorIDs {
			if actor.ID == allowedID {
				allowed = true
				break
			}
		}
		if !allowed {
			return true, fmt.Sprintf("Actor %s not in allowed list for this policy", actor.ID)
		}
	}

	// Check allowed actor kinds
	if len(ac.AllowedActorKinds) > 0 {
		allowed := false
		for _, kind := range ac.AllowedActorKinds {
			if actor.Kind == kind {
				allowed = true
				break
			}
		}
		if !allowed {
			return true, fmt.Sprintf("Actor kind %s not allowed for this policy", actor.Kind)
		}
	}

	// Check minimum trust level
	if ac.MinTrustLevel > 0 && actor.TrustLevel < ac.MinTrustLevel {
		return true, fmt.Sprintf("Actor trust level %s below required %s",
			actor.TrustLevel, ac.MinTrustLevel)
	}

	return false, ""
}

// evaluatePolicies evaluates all policies and returns the first match.
func (e *Evaluator) evaluatePolicies(evalCtx map[string]any) (*AutoApprovalPolicy, []FailedCondition) {
	var failedConditions []FailedCondition

	// Sort policies by priority
	policies := make([]AutoApprovalPolicy, len(e.config.Policies))
	copy(policies, e.config.Policies)
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Priority > policies[j].Priority
	})

	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}

		allMatch := true
		for _, cond := range policy.Conditions {
			matched, actualValue := e.evaluateCondition(cond, evalCtx)
			if !matched {
				allMatch = false
				failedConditions = append(failedConditions, FailedCondition{
					PolicyID:    policy.ID,
					PolicyName:  policy.Name,
					Condition:   cond,
					ActualValue: actualValue,
					Reason:      fmt.Sprintf("%s %s %v (actual: %v)", cond.Field, cond.Operator, cond.Value, actualValue),
				})
				break // Move to next policy
			}
		}

		if allMatch {
			return &policy, nil
		}
	}

	return nil, failedConditions
}

// evaluateCondition evaluates a single condition.
func (e *Evaluator) evaluateCondition(cond PolicyCondition, evalCtx map[string]any) (matched bool, actualValue any) {
	actualValue = getNestedValue(evalCtx, cond.Field)
	if actualValue == nil {
		return false, nil
	}

	switch cond.Operator {
	case "eq":
		return valuesEqual(actualValue, cond.Value), actualValue
	case "ne":
		return !valuesEqual(actualValue, cond.Value), actualValue
	case "lt":
		return compareNumeric(actualValue, cond.Value, func(a, b float64) bool { return a < b }), actualValue
	case "lte":
		return compareNumeric(actualValue, cond.Value, func(a, b float64) bool { return a <= b }), actualValue
	case "gt":
		return compareNumeric(actualValue, cond.Value, func(a, b float64) bool { return a > b }), actualValue
	case "gte":
		return compareNumeric(actualValue, cond.Value, func(a, b float64) bool { return a >= b }), actualValue
	case "in":
		return valueIn(actualValue, cond.Value), actualValue
	case "not_in":
		return !valueIn(actualValue, cond.Value), actualValue
	case "contains":
		return valueContains(actualValue, cond.Value), actualValue
	case "matches":
		return valueMatches(actualValue, cond.Value), actualValue
	default:
		return false, actualValue
	}
}

// buildEvalContext builds the evaluation context from proposal and analysis.
func (e *Evaluator) buildEvalContext(
	proposal *cgp.ChangeProposal,
	analysis *cgp.ChangeAnalysis,
	riskAssessment *risk.Assessment,
) map[string]any {
	ctx := make(map[string]any)

	// Risk context
	if riskAssessment != nil {
		ctx["risk"] = map[string]any{
			"score":    riskAssessment.Score,
			"severity": string(riskAssessment.Severity),
		}
	} else {
		ctx["risk"] = map[string]any{
			"score":    0.0,
			"severity": "low",
		}
	}

	// Actor context
	if proposal != nil {
		ctx["actor"] = map[string]any{
			"kind":       string(proposal.Actor.Kind),
			"id":         proposal.Actor.ID,
			"name":       proposal.Actor.Name,
			"trustLevel": int(proposal.Actor.TrustLevel),
		}
		ctx["intent"] = map[string]any{
			"summary":       proposal.Intent.Summary,
			"suggestedBump": string(proposal.Intent.SuggestedBump),
			"confidence":    proposal.Intent.Confidence,
		}
		ctx["scope"] = map[string]any{
			"repository":  proposal.Scope.Repository,
			"branch":      proposal.Scope.Branch,
			"commitRange": proposal.Scope.CommitRange,
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
		}
		if analysis.BlastRadius != nil {
			ctx["blastRadius"] = map[string]any{
				"score":        analysis.BlastRadius.Score,
				"filesChanged": analysis.BlastRadius.FilesChanged,
				"linesChanged": analysis.BlastRadius.LinesChanged,
			}
		}
	} else {
		ctx["change"] = map[string]any{
			"features":     0,
			"fixes":        0,
			"breaking":     0,
			"security":     0,
			"dependencies": 0,
			"other":        0,
			"total":        0,
		}
	}

	return ctx
}

// getApproverCount determines the number of approvers needed based on risk.
func (e *Evaluator) getApproverCount(riskScore float64) int {
	for rangeStr, count := range e.config.Thresholds.ApproverCountByRisk {
		parts := strings.Split(rangeStr, "-")
		if len(parts) != 2 {
			continue
		}
		var rangeMin, rangeMax float64
		_, _ = fmt.Sscanf(parts[0], "%f", &rangeMin)
		_, _ = fmt.Sscanf(parts[1], "%f", &rangeMax)

		if riskScore >= rangeMin && riskScore < rangeMax {
			return count
		}
	}

	// Default to 1 if no range matches
	if riskScore > 0 {
		return 1
	}
	return 0
}

// Helper functions

func getNestedValue(data map[string]any, path string) any {
	parts := strings.Split(path, ".")
	current := any(data)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[part]
			if !ok {
				return nil
			}
			current = val
		default:
			return nil
		}
	}

	return current
}

func valuesEqual(a, b any) bool {
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

func compareNumeric(a, b any, cmp func(float64, float64) bool) bool {
	av, aok := toFloat64(a)
	bv, bok := toFloat64(b)
	if !aok || !bok {
		return false
	}
	return cmp(av, bv)
}

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

func valueMatches(fieldValue, pattern any) bool {
	str, ok := fieldValue.(string)
	if !ok {
		return false
	}
	pat, ok := pattern.(string)
	if !ok {
		return false
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return false
	}
	return re.MatchString(str)
}

func matchPattern(value, pattern string) bool {
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(value, prefix)
	}
	return value == pattern
}

func generateAuditID() string {
	return fmt.Sprintf("auto-%d", time.Now().UnixNano())
}

// ToGovernanceDecision converts the result to a CGP governance decision.
func (r *Result) ToGovernanceDecision(proposalID string) *cgp.GovernanceDecision {
	decision := cgp.NewDecision(proposalID, r.CGPDecision)
	decision.WithRiskScore(r.RiskScore)

	for _, rationale := range r.Rationale {
		decision.AddRationale(rationale)
	}

	if r.MatchedPolicy != nil {
		decision.AddRationale(fmt.Sprintf("Policy: %s (%s)", r.MatchedPolicy.Name, r.MatchedPolicy.ID))
	}

	if r.RequiredApprovers > 0 {
		decision.AddRequiredAction("human_approval",
			fmt.Sprintf("Requires %d approver(s)", r.RequiredApprovers))
	}

	return decision
}
