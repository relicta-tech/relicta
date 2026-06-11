// Package evaluator provides the CGP evaluation orchestration layer.
//
// The Evaluator combines risk assessment and policy evaluation to produce
// governance decisions for change proposals.
package evaluator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/budget"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
	"github.com/relicta-tech/relicta/v4/internal/cgp/risk"
	"github.com/relicta-tech/relicta/v4/internal/config"
)

// Evaluator orchestrates CGP evaluation by combining risk assessment
// and policy evaluation to produce governance decisions.
type Evaluator struct {
	riskCalculator *risk.Calculator
	policyEngine   *policy.Engine
	logger         *slog.Logger
	config         Config
	memoryStore    memory.Store
	nowFunc        func() time.Time
	repository     string
}

// Config configures the evaluator behavior.
type Config struct {
	// DefaultDecision is used when no policies match.
	DefaultDecision cgp.DecisionType

	// AutoApproveThreshold is the risk score below which changes
	// may be auto-approved (if policy allows).
	AutoApproveThreshold float64

	// RequireHumanForBreaking forces human review for breaking changes.
	RequireHumanForBreaking bool

	// RequireHumanForSecurity forces human review for security changes.
	RequireHumanForSecurity bool

	// MaxAutoApproveRisk is the maximum risk score for auto-approval.
	MaxAutoApproveRisk float64

	// StrictMode controls whether budget/freeze violations block releases
	// (true) or only add warnings (false).
	StrictMode bool

	// RiskBudget configures cumulative risk budget limits.
	RiskBudget *config.RiskBudgetConfig

	// FreezePeriods configures recurring freeze windows.
	FreezePeriods []config.FreezePeriodConfig
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() Config {
	return Config{
		DefaultDecision:         cgp.DecisionApprovalRequired,
		AutoApproveThreshold:    0.3,
		RequireHumanForBreaking: true,
		RequireHumanForSecurity: true,
		MaxAutoApproveRisk:      0.4,
	}
}

// Option configures the evaluator.
type Option func(*Evaluator)

// WithLogger sets the logger for the evaluator.
func WithLogger(logger *slog.Logger) Option {
	return func(e *Evaluator) {
		e.logger = logger
	}
}

// WithConfig sets the configuration for the evaluator.
func WithConfig(cfg Config) Option {
	return func(e *Evaluator) {
		e.config = cfg
	}
}

// WithRiskCalculator sets a custom risk calculator.
func WithRiskCalculator(calc *risk.Calculator) Option {
	return func(e *Evaluator) {
		e.riskCalculator = calc
	}
}

// WithPolicyEngine sets a custom policy engine.
func WithPolicyEngine(engine *policy.Engine) Option {
	return func(e *Evaluator) {
		e.policyEngine = engine
	}
}

// WithMemoryStore sets the release memory store for budget tracking.
func WithMemoryStore(store memory.Store) Option {
	return func(e *Evaluator) {
		e.memoryStore = store
	}
}

// WithRepository sets the repository identifier for memory lookups.
func WithRepository(repo string) Option {
	return func(e *Evaluator) {
		e.repository = repo
	}
}

// WithNowFunc sets the time provider (useful for testing).
func WithNowFunc(fn func() time.Time) Option {
	return func(e *Evaluator) {
		e.nowFunc = fn
	}
}

// New creates a new Evaluator with the given options.
func New(opts ...Option) *Evaluator {
	e := &Evaluator{
		riskCalculator: risk.NewCalculatorWithDefaults(),
		policyEngine:   policy.NewEngine([]policy.Policy{}, nil),
		logger:         slog.Default(),
		config:         DefaultConfig(),
		nowFunc:        time.Now,
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// NewWithPolicies creates an evaluator with the given policies.
func NewWithPolicies(policies []policy.Policy, opts ...Option) *Evaluator {
	e := New(opts...)
	e.policyEngine = policy.NewEngine(policies, e.logger)
	return e
}

// EvaluationResult contains the complete evaluation outcome.
type EvaluationResult struct {
	// Decision is the governance decision.
	Decision *cgp.GovernanceDecision

	// RiskAssessment contains the risk analysis.
	RiskAssessment *risk.Assessment

	// PolicyResult contains the policy evaluation outcome.
	PolicyResult *policy.Result

	// EvaluatedAt is when the evaluation occurred.
	EvaluatedAt time.Time

	// Duration is how long the evaluation took.
	Duration time.Duration
}

// Evaluate processes a change proposal and produces a governance decision.
func (e *Evaluator) Evaluate(ctx context.Context, proposal *cgp.ChangeProposal, analysis *cgp.ChangeAnalysis) (*EvaluationResult, error) {
	startTime := time.Now()

	if proposal == nil {
		return nil, fmt.Errorf("proposal is required")
	}

	if err := proposal.Validate(); err != nil {
		return nil, fmt.Errorf("invalid proposal: %w", err)
	}

	e.logger.Info("evaluating proposal",
		"proposal_id", proposal.ID,
		"actor", proposal.Actor.ID,
		"actor_kind", proposal.Actor.Kind,
	)

	// Step 1: Risk Assessment
	riskAssessment, err := e.riskCalculator.Calculate(ctx, proposal, analysis)
	if err != nil {
		return nil, fmt.Errorf("risk assessment failed: %w", err)
	}

	e.logger.Debug("risk assessment complete",
		"score", riskAssessment.Score,
		"severity", riskAssessment.Severity,
		"factors", len(riskAssessment.Factors),
	)

	// Step 2: Policy Evaluation
	policyResult, err := e.policyEngine.Evaluate(ctx, proposal, analysis, riskAssessment.Score)
	if err != nil {
		return nil, fmt.Errorf("policy evaluation failed: %w", err)
	}

	e.logger.Debug("policy evaluation complete",
		"decision", policyResult.Decision,
		"matched_rules", policyResult.MatchedRules,
		"required_approvers", policyResult.RequiredApprovers,
	)

	// Step 3: Budget and Freeze Period Checks
	e.applyBudgetAndFreezeChecks(ctx, policyResult, riskAssessment.Score)

	// Step 4: Build Governance Decision
	decision := e.buildDecision(proposal, analysis, riskAssessment, policyResult)

	// Step 5: Apply additional governance rules
	e.applyGovernanceRules(decision, proposal, analysis, riskAssessment)

	duration := time.Since(startTime)

	e.logger.Info("evaluation complete",
		"proposal_id", proposal.ID,
		"decision", decision.Decision,
		"risk_score", decision.RiskScore,
		"duration", duration,
	)

	return &EvaluationResult{
		Decision:       decision,
		RiskAssessment: riskAssessment,
		PolicyResult:   policyResult,
		EvaluatedAt:    startTime,
		Duration:       duration,
	}, nil
}

// buildDecision constructs the governance decision from evaluation results.
func (e *Evaluator) buildDecision(
	proposal *cgp.ChangeProposal,
	analysis *cgp.ChangeAnalysis,
	riskAssessment *risk.Assessment,
	policyResult *policy.Result,
) *cgp.GovernanceDecision {
	decision := cgp.NewDecision(proposal.ID, policyResult.Decision)
	decision.WithRiskScore(riskAssessment.Score)

	// Add risk factors
	for _, factor := range riskAssessment.Factors {
		decision.AddRiskFactor(factor.Category, factor.Description, factor.Score, factor.Severity)
	}

	// Add rationale from policy evaluation
	for _, r := range policyResult.Rationale {
		decision.AddRationale(r)
	}

	// Add required actions
	for _, action := range policyResult.RequiredActions {
		decision.AddRequiredAction(action.Type, action.Description)
	}

	// Add conditions
	for _, cond := range policyResult.Conditions {
		decision.AddCondition(cond.Type, cond.Value)
	}

	// Set recommended version if proposal suggests one
	if proposal.Intent.SuggestedBump != "" {
		decision.WithRecommendedVersion(string(proposal.Intent.SuggestedBump))
	}

	// Attach analysis if provided
	if analysis != nil {
		decision.WithAnalysis(analysis)
	}

	return decision
}

// applyGovernanceRules applies additional governance constraints.
func (e *Evaluator) applyGovernanceRules(
	decision *cgp.GovernanceDecision,
	proposal *cgp.ChangeProposal,
	analysis *cgp.ChangeAnalysis,
	riskAssessment *risk.Assessment,
) {
	// Rule: Require human review for agent-initiated changes with high risk
	if proposal.Actor.Kind == cgp.ActorKindAgent && riskAssessment.Score > e.config.MaxAutoApproveRisk {
		if decision.Decision == cgp.DecisionApproved {
			decision.Decision = cgp.DecisionApprovalRequired
			decision.AddRationale("Agent-initiated change with elevated risk requires human review")
			decision.AddRequiredAction("human_approval", "Review agent-initiated change before release")
		}
	}

	// Rule: Breaking changes require human review
	if e.config.RequireHumanForBreaking && analysis != nil && analysis.Breaking > 0 {
		if decision.Decision == cgp.DecisionApproved {
			decision.Decision = cgp.DecisionApprovalRequired
			decision.AddRationale(fmt.Sprintf("%d breaking changes detected - human review required", analysis.Breaking))
			decision.AddRequiredAction("human_approval", "Review breaking changes before release")
		}
	}

	// Rule: Security changes require human review
	if e.config.RequireHumanForSecurity && analysis != nil && analysis.Security > 0 {
		if decision.Decision == cgp.DecisionApproved {
			decision.Decision = cgp.DecisionApprovalRequired
			decision.AddRationale(fmt.Sprintf("%d security-related changes detected - human review required", analysis.Security))
			decision.AddRequiredAction("human_approval", "Review security changes before release")
		}
	}

	// Rule: Low risk changes from trusted actors may be auto-approved
	if proposal.Actor.TrustLevel.CanAutoApprove() &&
		riskAssessment.Score < e.config.AutoApproveThreshold &&
		decision.Decision == cgp.DecisionApprovalRequired {
		// Check if there are no blocking conditions
		hasBlockingCondition := false
		for _, action := range decision.RequiredActions {
			if action.Type == "human_approval" {
				hasBlockingCondition = true
				break
			}
		}
		if !hasBlockingCondition {
			decision.Decision = cgp.DecisionApproved
			decision.AddRationale("Low-risk change from trusted actor auto-approved")
		}
	}
}

// EvaluateQuick performs a lightweight evaluation without full policy processing.
// Useful for previews and dry-run scenarios.
func (e *Evaluator) EvaluateQuick(ctx context.Context, proposal *cgp.ChangeProposal, analysis *cgp.ChangeAnalysis) (*risk.Assessment, error) {
	if proposal == nil {
		return nil, fmt.Errorf("proposal is required")
	}

	return e.riskCalculator.Calculate(ctx, proposal, analysis)
}

// ValidateProposal checks if a proposal is valid for evaluation.
func (e *Evaluator) ValidateProposal(proposal *cgp.ChangeProposal) error {
	if proposal == nil {
		return fmt.Errorf("proposal is nil")
	}
	return proposal.Validate()
}

// GetPolicies returns the loaded policies.
func (e *Evaluator) GetPolicies() []policy.Policy {
	// Note: This would require exposing policies from the engine
	// For now, return empty - can be enhanced later
	return nil
}

// AddPolicy adds a policy to the evaluator's policy engine.
func (e *Evaluator) AddPolicy(p policy.Policy) {
	e.policyEngine.AddPolicy(p)
}

// applyBudgetAndFreezeChecks evaluates risk budget and freeze period
// constraints. Depending on StrictMode, violations either block the
// release or add warnings to the policy result.
func (e *Evaluator) applyBudgetAndFreezeChecks(ctx context.Context, policyResult *policy.Result, riskScore float64) {
	// Check freeze periods.
	if len(e.config.FreezePeriods) > 0 {
		now := e.nowFunc()
		freezeResult := budget.CheckFreeze(now, e.config.FreezePeriods, riskScore)
		if freezeResult.Frozen && !freezeResult.Allowed {
			if e.config.StrictMode {
				policyResult.Decision = cgp.DecisionRejected
			} else if policyResult.Decision == cgp.DecisionApproved {
				policyResult.Decision = cgp.DecisionApprovalRequired
			}
			policyResult.Rationale = append(policyResult.Rationale, freezeResult.Reason)
			policyResult.RequiredActions = append(policyResult.RequiredActions, cgp.RequiredAction{
				Type:        "freeze_period",
				Description: freezeResult.Reason,
			})
			e.logger.Warn("freeze period violation",
				"freeze", freezeResult.FreezeName,
				"risk", riskScore,
				"max_risk", freezeResult.MaxRisk,
			)
		} else if freezeResult.Frozen && freezeResult.Allowed {
			policyResult.Rationale = append(policyResult.Rationale, freezeResult.Reason)
			e.logger.Info("freeze period active but release permitted",
				"freeze", freezeResult.FreezeName,
				"risk", riskScore,
			)
		}
	}

	// Check risk budget.
	if e.config.RiskBudget != nil && e.memoryStore != nil && e.repository != "" {
		releases, err := e.memoryStore.GetReleaseHistory(ctx, e.repository, 100)
		if err != nil {
			e.logger.Warn("failed to load release history for budget check", "error", err)
			return
		}
		budgetResult := budget.CheckBudget(riskScore, e.config.RiskBudget, releases)
		if !budgetResult.Allowed {
			if e.config.StrictMode {
				policyResult.Decision = cgp.DecisionRejected
			} else if policyResult.Decision == cgp.DecisionApproved {
				policyResult.Decision = cgp.DecisionApprovalRequired
			}
			policyResult.Rationale = append(policyResult.Rationale, budgetResult.Reason)
			policyResult.RequiredActions = append(policyResult.RequiredActions, cgp.RequiredAction{
				Type:        "budget_exceeded",
				Description: budgetResult.Reason,
			})
			e.logger.Warn("risk budget exceeded",
				"used", budgetResult.UsedBudget,
				"remaining", budgetResult.RemainingBudget,
				"risk", riskScore,
			)
		}
	}
}
