// Package governance provides CGP (Change Governance Protocol) integration for release workflows.
package governance

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/attribution"
	"github.com/relicta-tech/relicta/v4/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/release"
)

// Re-export ReleaseOutcome type and constants for CLI convenience.
// This allows CLI code to use governance.OutcomeSuccess without importing memory directly.
type ReleaseOutcome = memory.ReleaseOutcome

const (
	OutcomeSuccess  = memory.OutcomeSuccess
	OutcomeRollback = memory.OutcomeRollback
	OutcomeFailure  = memory.OutcomeFailed
	OutcomePartial  = memory.OutcomePartial
)

// Service provides CGP governance evaluation for release workflows.
type Service struct {
	evaluator   *evaluator.Evaluator
	memoryStore memory.Store
	logger      *slog.Logger

	// calibrationEnabled retunes risk weights from historical outcomes once,
	// lazily, on first evaluation. reputationEnabled downgrades auto-approvals
	// for actors with a demonstrated poor track record. Both require a memory
	// store and only ever tighten outcomes.
	calibrationEnabled bool
	reputationEnabled  bool
	calibrateOnce      sync.Once

	// attributionEnabled turns on authorship detection: the actual author of a
	// changeset's commits (human, AI agent, or CI) is detected from commit
	// metadata and, when a machine authored a human-initiated release, governs
	// the proposal so agent-authored changes face agent governance rules. Only
	// ever tightens — machine authorship adds scrutiny, never removes it.
	attributionEnabled bool
	detector           *attribution.Detector

	// reputationProbationThreshold is the overall-score floor below which the
	// reputation guard downgrades an auto-approval. Zero means use the package
	// default (reputation.ThresholdProbation).
	reputationProbationThreshold float64

	// calibrationMinAccuracy is the accuracy floor below which a calibration
	// result is considered unreliable. Zero disables the check. When
	// calibrationStrict is set, suspect weights are not applied (fail closed).
	calibrationMinAccuracy float64
	calibrationStrict      bool
}

// ServiceOption configures a governance Service.
type ServiceOption func(*Service)

// WithLogger sets the logger for the service.
func WithLogger(logger *slog.Logger) ServiceOption {
	return func(s *Service) {
		s.logger = logger
	}
}

// WithMemoryStore sets the release memory store.
func WithMemoryStore(store memory.Store) ServiceOption {
	return func(s *Service) {
		s.memoryStore = store
	}
}

// WithCalibration enables data-driven risk calibration (requires a memory store).
func WithCalibration(enabled bool) ServiceOption {
	return func(s *Service) {
		s.calibrationEnabled = enabled
	}
}

// WithReputation enables actor-reputation guarding (requires a memory store).
func WithReputation(enabled bool) ServiceOption {
	return func(s *Service) {
		s.reputationEnabled = enabled
	}
}

// WithReputationThreshold sets the reputation probation floor for the guard.
// Values outside (0, 1) are ignored and the package default is used.
func WithReputationThreshold(threshold float64) ServiceOption {
	return func(s *Service) {
		if threshold > 0 && threshold < 1 {
			s.reputationProbationThreshold = threshold
		}
	}
}

// WithAttribution enables commit-authorship detection so machine-authored
// changes are governed as such even when a human initiates the release.
func WithAttribution(enabled bool) ServiceOption {
	return func(s *Service) {
		s.attributionEnabled = enabled
	}
}

// WithCalibrationValidation guards calibration on prediction accuracy. When the
// calibrated accuracy is below minAccuracy a warning is logged; when strict is
// true the suspect weights are not applied. minAccuracy <= 0 disables the check.
func WithCalibrationValidation(minAccuracy float64, strict bool) ServiceOption {
	return func(s *Service) {
		s.calibrationMinAccuracy = minAccuracy
		s.calibrationStrict = strict
	}
}

// NewService creates a new governance service.
func NewService(eval *evaluator.Evaluator, opts ...ServiceOption) *Service {
	s := &Service{
		evaluator: eval,
		logger:    slog.Default().With("service", "governance"),
		detector:  attribution.NewDetector(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// EvaluateReleaseInput represents input for evaluating a release.
type EvaluateReleaseInput struct {
	// Release is the release being evaluated.
	Release *release.ReleaseRun

	// Actor is the actor initiating the release.
	Actor cgp.Actor

	// Repository is the repository path (owner/repo).
	Repository string

	// IncludeHistory indicates whether to include historical analysis.
	IncludeHistory bool
}

// EvaluateReleaseOutput represents the result of governance evaluation.
type EvaluateReleaseOutput struct {
	// Decision is the governance decision.
	Decision cgp.DecisionType

	// RiskScore is the calculated risk score (0.0-1.0).
	RiskScore float64

	// Severity is the risk severity level.
	Severity cgp.Severity

	// RequiredActions lists actions required before proceeding.
	RequiredActions []cgp.RequiredAction

	// RiskFactors lists identified risk factors.
	RiskFactors []cgp.RiskFactor

	// Rationale explains the decision.
	Rationale []string

	// Conditions that must be met for approval.
	Conditions []cgp.Condition

	// CanAutoApprove indicates if auto-approval is allowed.
	CanAutoApprove bool

	// HistoricalContext provides historical analysis if available.
	HistoricalContext *HistoricalContext

	// Reputation is the initiating actor's reputation assessment, when
	// reputation guarding is enabled and the actor has history. Informational;
	// the guard's effect is already reflected in Decision/Rationale.
	Reputation *ReputationInfo
}

// ReputationInfo summarizes an actor's reputation for evaluation output.
type ReputationInfo struct {
	// Overall is the composite reputation score in [0.0, 1.0].
	Overall float64
	// Level is the qualitative band: trusted, reliable, probation, restricted.
	Level string
	// SampleSize is the number of release records the score is based on.
	SampleSize int
	// Trend indicates improving, stable, or declining reputation.
	Trend string
	// Guarded is true when poor reputation downgraded the decision.
	Guarded bool
}

// HistoricalContext provides historical analysis for a release.
type HistoricalContext struct {
	// RecentReleases is the number of recent releases analyzed.
	RecentReleases int

	// SuccessRate is the actor's success rate.
	SuccessRate float64

	// RollbackRate is the actor's rollback rate.
	RollbackRate float64

	// AverageRiskScore is the historical average risk score.
	AverageRiskScore float64

	// RiskTrend indicates the risk trend.
	RiskTrend memory.RiskTrend

	// ReliabilityScore is the actor's reliability score.
	ReliabilityScore float64
}

// EvaluateRelease evaluates a release against CGP governance rules.
func (s *Service) EvaluateRelease(ctx context.Context, input EvaluateReleaseInput) (*EvaluateReleaseOutput, error) {
	if input.Release == nil {
		return nil, fmt.Errorf("release is required")
	}

	// Retune risk weights from historical outcomes once, before the first
	// evaluation, when calibration is enabled.
	if s.calibrationEnabled && s.memoryStore != nil {
		s.ensureCalibrated(ctx, input.Repository)
	}

	// Build change proposal and analysis from release
	proposal, analysis := s.buildProposalAndAnalysis(input)

	// Evaluate the proposal
	result, err := s.evaluator.Evaluate(ctx, proposal, analysis)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate proposal: %w", err)
	}

	output := &EvaluateReleaseOutput{
		Decision:        result.Decision.Decision,
		RiskScore:       result.Decision.RiskScore,
		Severity:        result.RiskAssessment.Severity,
		RequiredActions: result.Decision.RequiredActions,
		RiskFactors:     result.Decision.RiskFactors,
		Rationale:       result.Decision.Rationale,
		Conditions:      result.Decision.Conditions,
		CanAutoApprove:  result.Decision.Decision == cgp.DecisionApproved,
	}

	// Reputation guard: downgrade an auto-approval when the initiating actor has
	// a demonstrated poor track record. Tightens only — never loosens.
	if s.reputationEnabled && s.memoryStore != nil {
		s.applyReputationGuard(ctx, input, output)
	}

	// Add historical context if requested and available
	if input.IncludeHistory && s.memoryStore != nil {
		historicalCtx, err := s.getHistoricalContext(ctx, input)
		if err != nil {
			s.logger.Debug("failed to get historical context", "error", err)
		} else {
			output.HistoricalContext = historicalCtx
		}
	}

	s.logger.Info("release evaluated",
		"release_id", input.Release.ID(),
		"decision", output.Decision,
		"risk_score", output.RiskScore,
		"severity", output.Severity,
		"can_auto_approve", output.CanAutoApprove)

	return output, nil
}

// buildProposalAndAnalysis creates CGP proposal and analysis from a release.
func (s *Service) buildProposalAndAnalysis(input EvaluateReleaseInput) (*cgp.ChangeProposal, *cgp.ChangeAnalysis) {
	rel := input.Release
	plan := release.GetPlan(rel)

	// Build scope
	scope := cgp.ProposalScope{
		Repository: input.Repository,
	}
	if plan != nil && plan.HasChangeSet() {
		scope.CommitRange = plan.GetChangeSet().FromRef() + ".." + plan.GetChangeSet().ToRef()
	}

	// Determine bump type from release type
	var bumpType cgp.BumpType
	var breakingChanges, securityChanges int

	if plan != nil {
		switch plan.ReleaseType {
		case changes.ReleaseTypeMajor:
			bumpType = cgp.BumpTypeMajor
		case changes.ReleaseTypeMinor:
			bumpType = cgp.BumpTypeMinor
		default:
			bumpType = cgp.BumpTypePatch
		}

		if plan.HasChangeSet() {
			cats := plan.GetChangeSet().Categories()
			breakingChanges = len(cats.Breaking)
			// Detect security-related changes from commits
			securityChanges = countSecurityChanges(plan.GetChangeSet())
		}
	}

	// Build intent using VersionNext from release run
	intent := cgp.ProposalIntent{
		Summary:       fmt.Sprintf("Release %s for %s", rel.VersionNext(), input.Repository),
		SuggestedBump: bumpType,
		Confidence:    1.0, // Default; refined by authorship detection below.
	}

	// Detect who actually authored the changeset's commits and let machine
	// authorship govern the proposal. Tightens only — see applyAttribution.
	governingActor := input.Actor
	var detection *attribution.DetectionResult
	if s.attributionEnabled && plan != nil && plan.HasChangeSet() {
		detection = s.detectAuthorship(plan.GetChangeSet())
		governingActor, intent.Confidence = applyAttribution(input.Actor, detection)
	}

	// Create proposal
	proposal := cgp.NewProposal(governingActor, scope, intent)
	if detection != nil {
		recordAttributionContext(proposal, input.Actor, detection)
	}

	// Build analysis
	var analysis *cgp.ChangeAnalysis
	if plan != nil && plan.HasChangeSet() {
		summary := plan.GetChangeSet().Summary()
		analysis = &cgp.ChangeAnalysis{
			Breaking: breakingChanges,
			Security: securityChanges,
			BlastRadius: &cgp.BlastRadius{
				FilesChanged: summary.TotalCommits, // Approximate
				Score:        0.0,                  // Will be calculated by risk calculator
			},
		}
	}

	return proposal, analysis
}

// securityScopes are commit scopes that indicate security-related changes.
var securityScopes = []string{
	"security", "auth", "authentication", "authorization",
	"crypto", "encryption", "ssl", "tls", "cert", "certificate",
	"oauth", "jwt", "token", "session", "password", "credential",
	"acl", "rbac", "permission", "access-control",
}

// securityKeywords are keywords in commit messages that indicate security changes.
var securityKeywords = []string{
	"security", "cve", "vulnerability", "vuln", "exploit",
	"injection", "xss", "csrf", "sqli", "rce",
	"authentication", "authorization", "privilege",
	"sanitize", "escape", "validate input",
	"secret", "credential", "password", "token",
	"encrypt", "decrypt", "hash", "salt",
	"owasp", "pentest", "security fix", "security patch",
}

// countSecurityChanges counts the number of security-related commits in a changeset.
func countSecurityChanges(cs *changes.ChangeSet) int {
	if cs == nil {
		return 0
	}

	count := 0
	commits := cs.Commits()

	for _, commit := range commits {
		if isSecurityCommit(commit) {
			count++
		}
	}

	return count
}

// isSecurityCommit determines if a commit is security-related.
func isSecurityCommit(commit *changes.ConventionalCommit) bool {
	if commit == nil {
		return false
	}

	scope := strings.ToLower(commit.Scope())
	subject := strings.ToLower(commit.Subject())
	body := strings.ToLower(commit.Body())

	// Check if scope indicates security
	for _, secScope := range securityScopes {
		if scope == secScope || strings.Contains(scope, secScope) {
			return true
		}
	}

	// Check if subject or body contains security keywords
	textToCheck := subject + " " + body
	for _, keyword := range securityKeywords {
		if strings.Contains(textToCheck, keyword) {
			return true
		}
	}

	return false
}

// getHistoricalContext retrieves historical context from memory store.
func (s *Service) getHistoricalContext(ctx context.Context, input EvaluateReleaseInput) (*HistoricalContext, error) {
	if s.memoryStore == nil {
		return nil, fmt.Errorf("memory store not configured")
	}

	hCtx := &HistoricalContext{}

	// Get release history
	releases, err := s.memoryStore.GetReleaseHistory(ctx, input.Repository, 10)
	if err == nil && len(releases) > 0 {
		hCtx.RecentReleases = len(releases)
	}

	// Get actor metrics
	metrics, err := s.memoryStore.GetActorMetrics(ctx, input.Actor.ID)
	if err == nil {
		hCtx.SuccessRate = metrics.SuccessRate
		hCtx.RollbackRate = float64(metrics.RollbackCount) / float64(max(metrics.TotalReleases, 1))
		hCtx.ReliabilityScore = metrics.ReliabilityScore
	}

	// Get risk patterns
	patterns, err := s.memoryStore.GetRiskPatterns(ctx, input.Repository)
	if err == nil {
		hCtx.AverageRiskScore = patterns.AverageRiskScore
		hCtx.RiskTrend = patterns.RiskTrend
	}

	return hCtx, nil
}

// RecordReleaseOutcome records the outcome of a release for future analysis.
func (s *Service) RecordReleaseOutcome(ctx context.Context, input RecordOutcomeInput) error {
	if s.memoryStore == nil {
		s.logger.Debug("memory store not configured, skipping outcome recording")
		return nil
	}

	record := &memory.ReleaseRecord{
		ID:              string(input.ReleaseID),
		Repository:      input.Repository,
		Version:         input.Version,
		Actor:           input.Actor,
		RiskScore:       input.RiskScore,
		Decision:        input.Decision,
		BreakingChanges: input.BreakingChanges,
		SecurityChanges: input.SecurityChanges,
		FilesChanged:    input.FilesChanged,
		LinesChanged:    input.LinesChanged,
		Outcome:         input.Outcome,
		ReleasedAt:      time.Now(),
		Duration:        input.Duration,
		Tags:            input.Tags,
	}

	if err := s.memoryStore.RecordRelease(ctx, record); err != nil {
		return fmt.Errorf("failed to record release: %w", err)
	}

	s.logger.Info("release outcome recorded",
		"release_id", input.ReleaseID,
		"outcome", input.Outcome,
		"risk_score", input.RiskScore)

	return nil
}

// RecordOutcomeInput represents input for recording a release outcome.
type RecordOutcomeInput struct {
	ReleaseID       release.RunID
	Repository      string
	Version         string
	Actor           cgp.Actor
	RiskScore       float64
	Decision        cgp.DecisionType
	BreakingChanges int
	SecurityChanges int
	FilesChanged    int
	LinesChanged    int
	Outcome         memory.ReleaseOutcome
	Duration        time.Duration
	Tags            []string
}

// RecordIncident records an incident related to a release.
func (s *Service) RecordIncident(ctx context.Context, input RecordIncidentInput) error {
	if s.memoryStore == nil {
		s.logger.Debug("memory store not configured, skipping incident recording")
		return nil
	}

	record := &memory.IncidentRecord{
		ID:            input.ID,
		Repository:    input.Repository,
		ReleaseID:     string(input.ReleaseID),
		Version:       input.Version,
		ActorID:       input.ActorID,
		Type:          input.Type,
		Severity:      input.Severity,
		Description:   input.Description,
		RootCause:     input.RootCause,
		DetectedAt:    input.DetectedAt,
		ResolvedAt:    input.ResolvedAt,
		TimeToDetect:  input.TimeToDetect,
		TimeToResolve: input.TimeToResolve,
	}

	if err := s.memoryStore.RecordIncident(ctx, record); err != nil {
		return fmt.Errorf("failed to record incident: %w", err)
	}

	s.logger.Info("incident recorded",
		"incident_id", input.ID,
		"release_id", input.ReleaseID,
		"severity", input.Severity)

	return nil
}

// RecordIncidentInput represents input for recording an incident.
type RecordIncidentInput struct {
	ID            string
	Repository    string
	ReleaseID     release.RunID
	Version       string
	ActorID       string
	Type          memory.IncidentType
	Severity      cgp.Severity
	Description   string
	RootCause     string
	DetectedAt    time.Time
	ResolvedAt    *time.Time
	TimeToDetect  time.Duration
	TimeToResolve time.Duration
}

// QuickRiskCheck performs a fast risk-only evaluation without full policy checks.
func (s *Service) QuickRiskCheck(ctx context.Context, input EvaluateReleaseInput) (float64, cgp.Severity, error) {
	if input.Release == nil {
		return 0, "", fmt.Errorf("release is required")
	}

	proposal, analysis := s.buildProposalAndAnalysis(input)
	result, err := s.evaluator.EvaluateQuick(ctx, proposal, analysis)
	if err != nil {
		return 0, "", fmt.Errorf("failed to evaluate proposal: %w", err)
	}

	return result.Score, result.Severity, nil
}
