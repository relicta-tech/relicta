// Package memory provides the Release Memory store for CGP.
//
// Release Memory maintains historical context across releases to enable
// continuous improvement in risk assessment and governance decisions.
// It tracks past incidents, risky change patterns, and agent behavior.
package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
)

// Store provides access to release memory for historical analysis.
type Store interface {
	// RecordRelease stores a release record.
	RecordRelease(ctx context.Context, record *ReleaseRecord) error

	// RecordIncident stores an incident record.
	RecordIncident(ctx context.Context, incident *IncidentRecord) error

	// RecordDecision stores a governance decision for audit trail.
	RecordDecision(ctx context.Context, decision *cgp.GovernanceDecision) error

	// RecordAuthorization stores an execution authorization for audit trail.
	RecordAuthorization(ctx context.Context, auth *cgp.ExecutionAuthorization) error

	// GetReleaseHistory returns release records for a repository.
	GetReleaseHistory(ctx context.Context, repository string, limit int) ([]*ReleaseRecord, error)

	// GetIncidentHistory returns incident records for a repository.
	GetIncidentHistory(ctx context.Context, repository string, limit int) ([]*IncidentRecord, error)

	// GetDecision returns a governance decision by ID.
	GetDecision(ctx context.Context, decisionID string) (*cgp.GovernanceDecision, error)

	// GetDecisionsByProposal returns all decisions for a proposal.
	GetDecisionsByProposal(ctx context.Context, proposalID string) ([]*cgp.GovernanceDecision, error)

	// GetAuthorization returns an execution authorization by ID.
	GetAuthorization(ctx context.Context, authID string) (*cgp.ExecutionAuthorization, error)

	// GetAuthorizationsByDecision returns all authorizations for a decision.
	GetAuthorizationsByDecision(ctx context.Context, decisionID string) ([]*cgp.ExecutionAuthorization, error)

	// GetActorMetrics returns behavior metrics for an actor.
	GetActorMetrics(ctx context.Context, actorID string) (*ActorMetrics, error)

	// GetRiskPatterns returns historical risk patterns for a repository.
	GetRiskPatterns(ctx context.Context, repository string) (*RiskPatterns, error)

	// UpdateActorMetrics updates metrics for an actor based on a release outcome.
	UpdateActorMetrics(ctx context.Context, actorID string, outcome ReleaseOutcome) error

	// GetAuditTrail returns the complete audit trail for a proposal.
	GetAuditTrail(ctx context.Context, proposalID string) (*AuditTrail, error)
}

// AuditTrail provides a complete governance history for a release proposal.
type AuditTrail struct {
	// ProposalID is the identifier of the original proposal.
	ProposalID string `json:"proposalId"`

	// Decisions are all governance decisions made for this proposal.
	Decisions []*cgp.GovernanceDecision `json:"decisions"`

	// Authorizations are all execution authorizations granted.
	Authorizations []*cgp.ExecutionAuthorization `json:"authorizations"`

	// Release is the final release record (if published).
	Release *ReleaseRecord `json:"release,omitempty"`

	// Incidents are any incidents associated with this release.
	Incidents []*IncidentRecord `json:"incidents,omitempty"`

	// CreatedAt is when the first decision was made.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the trail was last updated.
	UpdatedAt time.Time `json:"updatedAt"`
}

// ReleaseRecord stores information about a completed release.
type ReleaseRecord struct {
	// ID is a unique identifier for this release record.
	ID string `json:"id"`

	// Repository is the repository path (owner/repo).
	Repository string `json:"repository"`

	// Version is the released version.
	Version string `json:"version"`

	// Actor identifies who initiated the release.
	Actor cgp.Actor `json:"actor"`

	// RiskScore is the risk score at time of release.
	RiskScore float64 `json:"riskScore"`

	// Decision is the governance decision made.
	Decision cgp.DecisionType `json:"decision"`

	// BreakingChanges counts breaking changes in this release.
	BreakingChanges int `json:"breakingChanges"`

	// SecurityChanges counts security-related changes.
	SecurityChanges int `json:"securityChanges"`

	// FilesChanged counts files modified.
	FilesChanged int `json:"filesChanged"`

	// LinesChanged counts lines modified.
	LinesChanged int `json:"linesChanged"`

	// Outcome is the final outcome of the release.
	Outcome ReleaseOutcome `json:"outcome"`

	// ReleasedAt is when the release was published.
	ReleasedAt time.Time `json:"releasedAt"`

	// FirstCommitAt is when the earliest change in this release was committed.
	//
	// DORA lead time for changes is the interval from a change being committed to it
	// running in production, so this is where that interval starts. Without it the
	// metric has no beginning and can only be approximated from the release itself,
	// which measures delivery lag rather than lead time.
	//
	// Zero for records written before this field existed, and for a release whose
	// commits are unknown. Readers must treat zero as "unknown" rather than as the
	// epoch, or every historical release becomes a 56-year lead time.
	FirstCommitAt time.Time `json:"firstCommitAt,omitempty"`

	// Duration is how long the release process took.
	//
	// This is the runtime of the release itself — a few seconds or minutes. It is not
	// lead time for changes, and reporting it as such rated every project "elite"
	// regardless of how long its changes actually took to reach users.
	Duration time.Duration `json:"duration"`

	// Tags are labels for categorization.
	Tags []string `json:"tags,omitempty"`

	// Metadata contains additional release information.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ReleaseOutcome represents the final outcome of a release.
type ReleaseOutcome string

const (
	OutcomeSuccess  ReleaseOutcome = "success"  // Release succeeded without issues
	OutcomeRollback ReleaseOutcome = "rollback" // Release was rolled back
	OutcomeFailed   ReleaseOutcome = "failed"   // Release failed to complete
	OutcomePartial  ReleaseOutcome = "partial"  // Release partially succeeded

	// OutcomeCanceled records a run someone deliberately stopped. It is not a
	// release: nothing was published and nothing reached users.
	//
	// It exists because canceling used to be recorded as OutcomePartial, which
	// IsNegative counts as a problem and Accumulate counts as a failed release. So
	// deciding not to ship — the governance system working — damaged the actor's
	// reliability score and raised change failure rate, while a project that never
	// reconsidered a release looked flawless. A cancel belongs in the record for audit
	// and belongs out of every rate computed from releases; CountsAsRelease is what
	// keeps those two facts from contradicting each other.
	OutcomeCanceled ReleaseOutcome = "canceled"
)

// IsValid returns true if the outcome is a valid value.
func (o ReleaseOutcome) IsValid() bool {
	switch o {
	case OutcomeSuccess, OutcomeRollback, OutcomeFailed, OutcomePartial, OutcomeCanceled:
		return true
	default:
		return false
	}
}

// IsNegative returns true if this outcome indicates a problem.
//
// A cancellation is not a problem. Someone looked at a release and decided against it,
// which is the intended use of a governance gate.
func (o ReleaseOutcome) IsNegative() bool {
	return o == OutcomeRollback || o == OutcomeFailed || o == OutcomePartial
}

// CountsAsRelease reports whether this outcome belongs in the population of releases
// that rates are computed over.
//
// Every rate here is a fraction of releases — success rate, change failure rate,
// deployment frequency, an actor's reliability. A canceled run is in the store for audit
// but never became a release, so counting it in the denominator understates every rate
// derived from it: it would make a team that cancels carefully look worse than one that
// ships everything. One predicate, so a reader and a writer cannot disagree about which
// records are releases.
func (o ReleaseOutcome) CountsAsRelease() bool {
	return o != OutcomeCanceled
}

// IncidentRecord stores information about a release incident.
type IncidentRecord struct {
	// ID is a unique identifier for this incident.
	ID string `json:"id"`

	// Repository is the repository path.
	Repository string `json:"repository"`

	// ReleaseID is the associated release record ID.
	ReleaseID string `json:"releaseId"`

	// Version is the version that had the incident.
	Version string `json:"version"`

	// Type categorizes the incident.
	Type IncidentType `json:"type"`

	// Severity indicates incident severity.
	Severity cgp.Severity `json:"severity"`

	// Description provides details about the incident.
	Description string `json:"description"`

	// RootCause is the identified root cause (if known).
	RootCause string `json:"rootCause,omitempty"`

	// DetectedAt is when the incident was detected.
	DetectedAt time.Time `json:"detectedAt"`

	// ResolvedAt is when the incident was resolved.
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`

	// TimeToDetect is how long until the incident was detected.
	TimeToDetect time.Duration `json:"timeToDetect"`

	// TimeToResolve is how long until the incident was resolved.
	TimeToResolve time.Duration `json:"timeToResolve,omitempty"`

	// ActorID is the actor who initiated the release.
	ActorID string `json:"actorId"`

	// Tags are labels for categorization.
	Tags []string `json:"tags,omitempty"`
}

// IncidentType categorizes the type of incident.
type IncidentType string

const (
	IncidentRollback     IncidentType = "rollback"     // Release rolled back
	IncidentBugIntro     IncidentType = "bug_intro"    // Bug introduced
	IncidentPerformance  IncidentType = "performance"  // Performance regression
	IncidentSecurity     IncidentType = "security"     // Security issue
	IncidentAvailability IncidentType = "availability" // Service availability impact
	IncidentDataIssue    IncidentType = "data_issue"   // Data corruption or loss
	IncidentBreaking     IncidentType = "breaking"     // Unexpected breaking change
	IncidentOther        IncidentType = "other"        // Other incident type
)

// IsValid returns true if the incident type is a valid value.
func (t IncidentType) IsValid() bool {
	switch t {
	case IncidentRollback, IncidentBugIntro, IncidentPerformance,
		IncidentSecurity, IncidentAvailability, IncidentDataIssue,
		IncidentBreaking, IncidentOther:
		return true
	default:
		return false
	}
}

// ActorMetrics tracks historical behavior metrics for an actor.
type ActorMetrics struct {
	// ActorID is the unique identifier of the actor.
	ActorID string `json:"actorId"`

	// ActorKind is the type of actor (agent, human, ci, system).
	ActorKind cgp.ActorKind `json:"actorKind"`

	// TotalReleases is the total number of releases by this actor.
	TotalReleases int `json:"totalReleases"`

	// SuccessfulReleases is the count of successful releases.
	SuccessfulReleases int `json:"successfulReleases"`

	// FailedReleases is the count of failed releases.
	FailedReleases int `json:"failedReleases"`

	// RollbackCount is the number of releases that were rolled back.
	RollbackCount int `json:"rollbackCount"`

	// IncidentCount is the total incidents associated with this actor.
	IncidentCount int `json:"incidentCount"`

	// AverageRiskScore is the average risk score of their releases.
	AverageRiskScore float64 `json:"averageRiskScore"`

	// HighRiskReleases counts releases with risk score > 0.7.
	HighRiskReleases int `json:"highRiskReleases"`

	// BreakingChangeReleases counts releases with breaking changes.
	BreakingChangeReleases int `json:"breakingChangeReleases"`

	// SuccessRate is SuccessfulReleases / TotalReleases.
	SuccessRate float64 `json:"successRate"`

	// ReliabilityScore is a composite score (0-1) of actor reliability.
	ReliabilityScore float64 `json:"reliabilityScore"`

	// FirstReleaseAt is the timestamp of first release.
	FirstReleaseAt *time.Time `json:"firstReleaseAt,omitempty"`

	// LastReleaseAt is the timestamp of last release.
	LastReleaseAt *time.Time `json:"lastReleaseAt,omitempty"`

	// UpdatedAt is when these metrics were last updated.
	UpdatedAt time.Time `json:"updatedAt"`
}

// CalculateReliabilityScore computes a reliability score from metrics.
func (m *ActorMetrics) CalculateReliabilityScore() float64 {
	if m.TotalReleases == 0 {
		return 0.5 // Neutral for unknown actors
	}

	// Weight factors for reliability calculation
	successWeight := 0.4
	rollbackWeight := 0.3
	incidentWeight := 0.2
	riskWeight := 0.1

	// Success rate component (higher is better)
	successComponent := m.SuccessRate * successWeight

	// Rollback rate component (lower is better)
	rollbackRate := float64(m.RollbackCount) / float64(m.TotalReleases)
	rollbackComponent := (1.0 - rollbackRate) * rollbackWeight

	// Incident rate component (lower is better)
	incidentRate := float64(m.IncidentCount) / float64(m.TotalReleases)
	// Cap at 1.0 for the calculation
	if incidentRate > 1.0 {
		incidentRate = 1.0
	}
	incidentComponent := (1.0 - incidentRate) * incidentWeight

	// Risk component (lower average risk is better)
	riskComponent := (1.0 - m.AverageRiskScore) * riskWeight

	return successComponent + rollbackComponent + incidentComponent + riskComponent
}

// IsReliable returns true if the actor has a good track record.
func (m *ActorMetrics) IsReliable() bool {
	return m.ReliabilityScore >= 0.7 && m.TotalReleases >= 5
}

// RiskPatterns captures historical risk patterns for a repository.
type RiskPatterns struct {
	// Repository is the repository path.
	Repository string `json:"repository"`

	// AverageRiskScore is the historical average risk score.
	AverageRiskScore float64 `json:"averageRiskScore"`

	// RiskTrend indicates whether risk is increasing, stable, or decreasing.
	RiskTrend RiskTrend `json:"riskTrend"`

	// HighRiskPeriods identifies time periods with elevated risk.
	HighRiskPeriods []TimePeriod `json:"highRiskPeriods,omitempty"`

	// CommonRiskFactors are frequently occurring risk factors.
	CommonRiskFactors []RiskFactorPattern `json:"commonRiskFactors,omitempty"`

	// IncidentCorrelations maps patterns to incident likelihood.
	IncidentCorrelations []IncidentCorrelation `json:"incidentCorrelations,omitempty"`

	// TotalReleases is the number of releases analyzed.
	TotalReleases int `json:"totalReleases"`

	// AnalysisPeriod is the time range of the analysis.
	AnalysisPeriod TimePeriod `json:"analysisPeriod"`

	// UpdatedAt is when this analysis was last updated.
	UpdatedAt time.Time `json:"updatedAt"`
}

// RiskTrend indicates the direction of risk over time.
type RiskTrend string

const (
	TrendIncreasing RiskTrend = "increasing"
	TrendStable     RiskTrend = "stable"
	TrendDecreasing RiskTrend = "decreasing"
)

// TimePeriod represents a time range.
type TimePeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// RiskFactorPattern captures a recurring risk factor.
type RiskFactorPattern struct {
	// Category is the risk factor category.
	Category string `json:"category"`

	// Frequency is how often this factor appears (0-1).
	Frequency float64 `json:"frequency"`

	// AverageImpact is the average risk contribution.
	AverageImpact float64 `json:"averageImpact"`

	// CorrelatedIncidents is the count of associated incidents.
	CorrelatedIncidents int `json:"correlatedIncidents"`
}

// IncidentCorrelation maps patterns to incident likelihood.
type IncidentCorrelation struct {
	// Pattern describes the risk pattern.
	Pattern string `json:"pattern"`

	// IncidentProbability is the historical incident rate (0-1).
	IncidentProbability float64 `json:"incidentProbability"`

	// SampleSize is the number of releases with this pattern.
	SampleSize int `json:"sampleSize"`
}

// InMemoryStore provides an in-memory implementation of the Store interface.
// This is useful for testing and short-lived processes.
type InMemoryStore struct {
	mu             sync.RWMutex
	releases       map[string][]*ReleaseRecord            // keyed by repository
	deployments    map[string][]*DeploymentRecord         // keyed by repository
	incidents      map[string][]*IncidentRecord           // keyed by repository
	actors         map[string]*ActorMetrics               // keyed by actor ID
	decisions      map[string]*cgp.GovernanceDecision     // keyed by decision ID
	authorizations map[string]*cgp.ExecutionAuthorization // keyed by authorization ID
}

// NewInMemoryStore creates a new in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		releases:       make(map[string][]*ReleaseRecord),
		deployments:    make(map[string][]*DeploymentRecord),
		incidents:      make(map[string][]*IncidentRecord),
		actors:         make(map[string]*ActorMetrics),
		decisions:      make(map[string]*cgp.GovernanceDecision),
		authorizations: make(map[string]*cgp.ExecutionAuthorization),
	}
}

// RecordRelease stores a release record.
func (s *InMemoryStore) RecordRelease(ctx context.Context, record *ReleaseRecord) error {
	if record == nil {
		return fmt.Errorf("record is required")
	}
	if record.ID == "" {
		return fmt.Errorf("record ID is required")
	}
	if record.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	records, replaced := UpsertReleaseRecord(s.releases[record.Repository], record)
	s.releases[record.Repository] = records

	if replaced != nil {
		// See FileStore.RecordRelease: a replaced record cannot be folded into a
		// running average, so the affected actors are recomputed instead.
		s.rebuildActorMetricsLocked(record.Actor.ID, record.Actor.Kind)
		if replaced.Actor.ID != record.Actor.ID {
			s.rebuildActorMetricsLocked(replaced.Actor.ID, replaced.Actor.Kind)
		}
	} else {
		s.updateActorMetricsLocked(record)
	}

	return nil
}

// rebuildActorMetricsLocked recomputes one actor's metrics from the stored records.
// Must be called with the lock held.
func (s *InMemoryStore) rebuildActorMetricsLocked(actorID string, kind cgp.ActorKind) {
	if actorID == "" {
		return
	}
	s.actors[actorID] = RebuildActorMetrics(actorID, kind, s.releases, s.incidents, time.Now())
}

// updateActorMetricsLocked updates actor metrics based on a release record.
// Must be called with the lock held.
func (s *InMemoryStore) updateActorMetricsLocked(record *ReleaseRecord) {
	actorID := record.Actor.ID
	metrics, exists := s.actors[actorID]
	if !exists {
		metrics = &ActorMetrics{ActorID: actorID, ActorKind: record.Actor.Kind}
		s.actors[actorID] = metrics
	}
	// One definition, shared with FileStore and the Mnemos adapter: see
	// ActorMetrics.Accumulate for why this must not be reimplemented per store.
	metrics.Accumulate(record, time.Now())
}

// RecordIncident stores an incident record.
func (s *InMemoryStore) RecordIncident(ctx context.Context, incident *IncidentRecord) error {
	if incident == nil {
		return fmt.Errorf("incident is required")
	}
	if incident.ID == "" {
		return fmt.Errorf("incident ID is required")
	}
	if incident.Repository == "" {
		return fmt.Errorf("repository is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	records, replaced := UpsertIncidentRecord(s.incidents[incident.Repository], incident)
	s.incidents[incident.Repository] = records

	// Rebuilt rather than incremented, matching RecordRelease and the file store: a counter
	// cannot be un-added, so a corrected incident would leave the actor scored against one
	// that happened once. Rebuilding also covers an actor an incident names before they have
	// any release, whom the old `if metrics, exists` dropped entirely.
	if incident.ActorID != "" {
		s.rebuildActorMetricsLocked(incident.ActorID, s.knownActorKindLocked(incident.ActorID))
	}
	if replaced != nil && replaced.ActorID != "" && replaced.ActorID != incident.ActorID {
		s.rebuildActorMetricsLocked(replaced.ActorID, s.knownActorKindLocked(replaced.ActorID))
	}

	return nil
}

// knownActorKindLocked returns the kind already recorded for an actor, or the zero kind.
//
// An IncidentRecord names an actor but not their kind, so an incident cannot introduce one.
// Must be called with the lock held.
func (s *InMemoryStore) knownActorKindLocked(actorID string) cgp.ActorKind {
	if metrics, exists := s.actors[actorID]; exists {
		return metrics.ActorKind
	}
	return ""
}

// GetReleaseHistory returns release records for a repository.
func (s *InMemoryStore) GetReleaseHistory(ctx context.Context, repository string, limit int) ([]*ReleaseRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	releases := s.releases[repository]
	if len(releases) == 0 {
		return []*ReleaseRecord{}, nil
	}

	// Return most recent first
	result := make([]*ReleaseRecord, 0, min(limit, len(releases)))
	start := len(releases) - limit
	if start < 0 {
		start = 0
	}
	for i := len(releases) - 1; i >= start; i-- {
		result = append(result, releases[i])
	}

	return result, nil
}

// GetIncidentHistory returns incident records for a repository.
func (s *InMemoryStore) GetIncidentHistory(ctx context.Context, repository string, limit int) ([]*IncidentRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	incidents := s.incidents[repository]
	if len(incidents) == 0 {
		return []*IncidentRecord{}, nil
	}

	// Return most recent first
	result := make([]*IncidentRecord, 0, min(limit, len(incidents)))
	start := len(incidents) - limit
	if start < 0 {
		start = 0
	}
	for i := len(incidents) - 1; i >= start; i-- {
		result = append(result, incidents[i])
	}

	return result, nil
}

// GetActorMetrics returns behavior metrics for an actor.
func (s *InMemoryStore) GetActorMetrics(ctx context.Context, actorID string) (*ActorMetrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics, exists := s.actors[actorID]
	if !exists {
		return nil, fmt.Errorf("no metrics found for actor: %s", actorID)
	}

	// Return a copy
	metricsCopy := *metrics
	return &metricsCopy, nil
}

// GetRiskPatterns returns historical risk patterns for a repository.
func (s *InMemoryStore) GetRiskPatterns(ctx context.Context, repository string) (*RiskPatterns, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return RiskPatternsFrom(repository, s.releases[repository], time.Now())
}

// UpdateActorMetrics updates metrics for an actor based on a release outcome.
func (s *InMemoryStore) UpdateActorMetrics(ctx context.Context, actorID string, outcome ReleaseOutcome) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metrics, exists := s.actors[actorID]
	if !exists {
		return fmt.Errorf("no metrics found for actor: %s", actorID)
	}

	// Update based on outcome (used for updating after initial record)
	switch outcome {
	case OutcomeRollback:
		metrics.RollbackCount++
		metrics.FailedReleases++
		metrics.SuccessfulReleases-- // Undo the initial success count
	}

	metrics.SuccessRate = float64(metrics.SuccessfulReleases) / float64(metrics.TotalReleases)
	metrics.ReliabilityScore = metrics.CalculateReliabilityScore()
	metrics.UpdatedAt = time.Now()

	return nil
}

// RecordDecision stores a governance decision.
func (s *InMemoryStore) RecordDecision(ctx context.Context, decision *cgp.GovernanceDecision) error {
	if decision == nil {
		return fmt.Errorf("decision is required")
	}
	if decision.ID == "" {
		return fmt.Errorf("decision ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.decisions[decision.ID] = decision
	return nil
}

// RecordAuthorization stores an execution authorization.
func (s *InMemoryStore) RecordAuthorization(ctx context.Context, auth *cgp.ExecutionAuthorization) error {
	if auth == nil {
		return fmt.Errorf("authorization is required")
	}
	if auth.ID == "" {
		return fmt.Errorf("authorization ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.authorizations[auth.ID] = auth
	return nil
}

// GetDecision returns a governance decision by ID.
func (s *InMemoryStore) GetDecision(ctx context.Context, decisionID string) (*cgp.GovernanceDecision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	decision, exists := s.decisions[decisionID]
	if !exists {
		return nil, fmt.Errorf("decision not found: %s", decisionID)
	}
	return decision, nil
}

// GetDecisionsByProposal returns all decisions for a proposal.
func (s *InMemoryStore) GetDecisionsByProposal(ctx context.Context, proposalID string) ([]*cgp.GovernanceDecision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var decisions []*cgp.GovernanceDecision
	for _, d := range s.decisions {
		if d.ProposalID == proposalID {
			decisions = append(decisions, d)
		}
	}
	return decisions, nil
}

// GetAuthorization returns an execution authorization by ID.
func (s *InMemoryStore) GetAuthorization(ctx context.Context, authID string) (*cgp.ExecutionAuthorization, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	auth, exists := s.authorizations[authID]
	if !exists {
		return nil, fmt.Errorf("authorization not found: %s", authID)
	}
	return auth, nil
}

// GetAuthorizationsByDecision returns all authorizations for a decision.
func (s *InMemoryStore) GetAuthorizationsByDecision(ctx context.Context, decisionID string) ([]*cgp.ExecutionAuthorization, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var auths []*cgp.ExecutionAuthorization
	for _, a := range s.authorizations {
		if a.DecisionID == decisionID {
			auths = append(auths, a)
		}
	}
	return auths, nil
}

// GetAuditTrail returns the complete audit trail for a proposal.
func (s *InMemoryStore) GetAuditTrail(ctx context.Context, proposalID string) (*AuditTrail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Gather all decisions for this proposal
	var decisions []*cgp.GovernanceDecision
	for _, d := range s.decisions {
		if d.ProposalID == proposalID {
			decisions = append(decisions, d)
		}
	}

	if len(decisions) == 0 {
		return nil, fmt.Errorf("no audit trail found for proposal: %s", proposalID)
	}

	// Gather all authorizations for these decisions
	var auths []*cgp.ExecutionAuthorization
	decisionIDs := make(map[string]bool)
	for _, d := range decisions {
		decisionIDs[d.ID] = true
	}
	for _, a := range s.authorizations {
		if decisionIDs[a.DecisionID] {
			auths = append(auths, a)
		}
	}

	// Find earliest and latest timestamps
	var earliest, latest time.Time
	for i, d := range decisions {
		if i == 0 || d.Timestamp.Before(earliest) {
			earliest = d.Timestamp
		}
		if i == 0 || d.Timestamp.After(latest) {
			latest = d.Timestamp
		}
	}
	for _, a := range auths {
		if a.Timestamp.After(latest) {
			latest = a.Timestamp
		}
	}

	return &AuditTrail{
		ProposalID:     proposalID,
		Decisions:      decisions,
		Authorizations: auths,
		CreatedAt:      earliest,
		UpdatedAt:      latest,
	}, nil
}

// UpsertReleaseRecord replaces the record carrying the same ID, or appends it.
//
// A ReleaseRecord is the outcome of one release run, so a repository holds at most one
// per run ID. The stores appended unconditionally, which made two things duplicate
// history: retrying a publish for the same run, and — once release domain events reach
// the outcome tracker — the tracker and the CLI's own recordPublishOutcome both writing
// the result of a single publish. Two records for one run inflate deployment frequency
// and count the actor's release twice, and nothing reports it because both writes
// succeed.
//
// Reports whether an existing record was replaced, because a replacement invalidates
// UpsertIncidentRecord replaces the incident sharing an ID, or appends a new one.
//
// The counterpart of UpsertReleaseRecord, and it did not exist: every store appended incidents
// unconditionally while upserting releases, an asymmetry inside one implementation rather than a
// decision. A retried incident, or two processes reacting to one alert, left two rows and counted
// twice against the actor's incident rate — which feeds ReliabilityScore and the autonomy budget.
//
// One definition, shared, for the same reason the release one is: a rule added in whichever
// adapter noticed is how the backends came to disagree.
func UpsertIncidentRecord(records []*IncidentRecord, record *IncidentRecord) (result []*IncidentRecord, replaced *IncidentRecord) {
	for i, existing := range records {
		if existing != nil && existing.ID == record.ID {
			records[i] = record
			return records, existing
		}
	}
	return append(records, record), nil
}

// the incrementally accumulated actor metrics and the caller has to rebuild them.
func UpsertReleaseRecord(records []*ReleaseRecord, record *ReleaseRecord) (result []*ReleaseRecord, replaced *ReleaseRecord) {
	for i, existing := range records {
		if existing != nil && existing.ID == record.ID {
			records[i] = record
			return records, existing
		}
	}
	return append(records, record), nil
}

// RebuildActorMetrics recomputes one actor's metrics from the records that define them.
//
// Needed whenever a record is replaced rather than added: the release-derived fields are
// folded in incrementally by Accumulate, and a running average cannot be un-added, so the
// only honest way to reflect a changed record is to recompute from the set. Incidents are
// counted here too, since they contribute to the same metrics and a rebuild that ignored
// them would silently reset an actor's incident history to zero.
func RebuildActorMetrics(
	actorID string,
	kind cgp.ActorKind,
	releases map[string][]*ReleaseRecord,
	incidents map[string][]*IncidentRecord,
	at time.Time,
) *ActorMetrics {
	metrics := &ActorMetrics{ActorID: actorID, ActorKind: kind}

	// Ordered by release date so FirstReleaseAt/LastReleaseAt and the running average
	// land where they would have had the records arrived in order.
	var owned []*ReleaseRecord
	for _, records := range releases {
		for _, record := range records {
			if record != nil && record.Actor.ID == actorID {
				owned = append(owned, record)
			}
		}
	}
	sort.Slice(owned, func(i, j int) bool { return owned[i].ReleasedAt.Before(owned[j].ReleasedAt) })

	for _, record := range owned {
		metrics.Accumulate(record, at)
	}

	for _, records := range incidents {
		for _, incident := range records {
			if incident != nil && incident.ActorID == actorID {
				metrics.IncidentCount++
			}
		}
	}

	metrics.UpdatedAt = at
	metrics.ReliabilityScore = metrics.CalculateReliabilityScore()
	return metrics
}

// RiskPatternsFrom derives a repository's risk patterns from its release records.
//
// Extracted for the same reason Accumulate was: FileStore and InMemoryStore held
// identical copies of this arithmetic, and every backend added since would have written
// a third. What comes out of here feeds HistoricalContext straight into risk evaluation,
// so two implementations drifting apart would mean the same history justified different
// decisions depending on which backend was configured — and nothing would report it.
//
// Reports an error rather than zeroed patterns for a repository with no releases: a
// zeroed RiskPatterns reads as "this repository has never shipped anything risky", which
// is an assertion about history rather than an admission that there is none.
//
// The records are expected in the order the store holds them; the trend compares the
// first half against the second. RiskTrendOf in insights.go is a second definition of
// that comparison, sorted by release date and ignoring records with no timestamp.
// Reconciling the two changes what the reference reports, so it is a decision to take
// once for every adapter rather than here.
func RiskPatternsFrom(repository string, releases []*ReleaseRecord, at time.Time) (*RiskPatterns, error) {
	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found for repository: %s", repository)
	}

	patterns := &RiskPatterns{
		Repository:    repository,
		TotalReleases: len(releases),
		UpdatedAt:     at,
	}

	// Calculate average risk score
	var totalRisk float64
	riskFactorCounts := make(map[string]int)

	var minTime, maxTime time.Time
	for i, r := range releases {
		totalRisk += r.RiskScore

		if i == 0 || r.ReleasedAt.Before(minTime) {
			minTime = r.ReleasedAt
		}
		if i == 0 || r.ReleasedAt.After(maxTime) {
			maxTime = r.ReleasedAt
		}

		// Count risk factors from tags
		for _, tag := range r.Tags {
			riskFactorCounts[tag]++
		}
	}

	patterns.AverageRiskScore = totalRisk / float64(len(releases))
	patterns.AnalysisPeriod = TimePeriod{Start: minTime, End: maxTime}

	// Determine trend (comparing first half to second half)
	if len(releases) >= minimumReleasesForATrend {
		mid := len(releases) / 2
		var firstHalfRisk, secondHalfRisk float64
		for i := 0; i < mid; i++ {
			firstHalfRisk += releases[i].RiskScore
		}
		for i := mid; i < len(releases); i++ {
			secondHalfRisk += releases[i].RiskScore
		}
		firstHalfAvg := firstHalfRisk / float64(mid)
		secondHalfAvg := secondHalfRisk / float64(len(releases)-mid)

		switch diff := secondHalfAvg - firstHalfAvg; {
		case diff > riskTrendTolerance:
			patterns.RiskTrend = TrendIncreasing
		case diff < -riskTrendTolerance:
			patterns.RiskTrend = TrendDecreasing
		default:
			patterns.RiskTrend = TrendStable
		}
	} else {
		patterns.RiskTrend = TrendStable
	}

	// Build common risk factor patterns
	for factor, count := range riskFactorCounts {
		patterns.CommonRiskFactors = append(patterns.CommonRiskFactors, RiskFactorPattern{
			Category:  factor,
			Frequency: float64(count) / float64(len(releases)),
		})
	}

	return patterns, nil
}

// Accumulate folds one release into an actor's running metrics.
//
// Extracted because three stores need this and two had already copied it verbatim
// (InMemoryStore and FileStore), while the Mnemos adapter had no copy at all and
// therefore counted releases without ever looking at their outcome — an actor with
// nothing but failures showed a perfect record, and reputation and earned trust read
// that as grounds to widen the actor's autonomy.
//
// One definition matters more here than in most places. These numbers decide whether an
// actor's next change is auto-approved, so two implementations drifting apart would mean
// the same history justified different autonomy depending on which backend was
// configured — and nothing would report the disagreement.
func (m *ActorMetrics) Accumulate(record *ReleaseRecord, at time.Time) {
	if m == nil || record == nil {
		return
	}

	// A canceled run is recorded but is not a release, so it enters none of these
	// numbers — not the total, and therefore not the success rate's denominator either.
	if !record.Outcome.CountsAsRelease() {
		return
	}

	m.TotalReleases++
	switch record.Outcome {
	case OutcomeSuccess:
		m.SuccessfulReleases++
	case OutcomeFailed, OutcomePartial:
		m.FailedReleases++
	case OutcomeRollback:
		// A rollback counts as both: the change was withdrawn, and it failed. Counting it
		// only as a rollback would leave the failure rate reading clean.
		m.RollbackCount++
		m.FailedReleases++
	}

	if record.RiskScore > highRiskThreshold {
		m.HighRiskReleases++
	}
	if record.BreakingChanges > 0 {
		m.BreakingChangeReleases++
	}

	// Running average, so the whole history need not be held in memory.
	n := float64(m.TotalReleases)
	m.AverageRiskScore = ((n-1)*m.AverageRiskScore + record.RiskScore) / n

	m.SuccessRate = float64(m.SuccessfulReleases) / float64(m.TotalReleases)

	// A zero ReleasedAt is not a real timestamp and must not become the actor's first
	// release: it would date the actor's history to year one and make every interval
	// derived from it meaningless.
	if !record.ReleasedAt.IsZero() {
		if m.FirstReleaseAt == nil || record.ReleasedAt.Before(*m.FirstReleaseAt) {
			releasedAt := record.ReleasedAt
			m.FirstReleaseAt = &releasedAt
		}
		if m.LastReleaseAt == nil || record.ReleasedAt.After(*m.LastReleaseAt) {
			releasedAt := record.ReleasedAt
			m.LastReleaseAt = &releasedAt
		}
	}

	m.UpdatedAt = at
	m.ReliabilityScore = m.CalculateReliabilityScore()
}

// highRiskThreshold is the risk score above which a release counts as high risk.
const highRiskThreshold = 0.7
