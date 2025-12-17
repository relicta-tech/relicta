// Package memory provides the Release Memory store for CGP.
//
// Release Memory maintains historical context across releases to enable
// continuous improvement in risk assessment and governance decisions.
// It tracks past incidents, risky change patterns, and agent behavior.
package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
)

// Store provides access to release memory for historical analysis.
type Store interface {
	// RecordRelease stores a release record.
	RecordRelease(ctx context.Context, record *ReleaseRecord) error

	// RecordIncident stores an incident record.
	RecordIncident(ctx context.Context, incident *IncidentRecord) error

	// GetReleaseHistory returns release records for a repository.
	GetReleaseHistory(ctx context.Context, repository string, limit int) ([]*ReleaseRecord, error)

	// GetIncidentHistory returns incident records for a repository.
	GetIncidentHistory(ctx context.Context, repository string, limit int) ([]*IncidentRecord, error)

	// GetActorMetrics returns behavior metrics for an actor.
	GetActorMetrics(ctx context.Context, actorID string) (*ActorMetrics, error)

	// GetRiskPatterns returns historical risk patterns for a repository.
	GetRiskPatterns(ctx context.Context, repository string) (*RiskPatterns, error)

	// UpdateActorMetrics updates metrics for an actor based on a release outcome.
	UpdateActorMetrics(ctx context.Context, actorID string, outcome ReleaseOutcome) error
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

	// Duration is how long the release process took.
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
)

// IsValid returns true if the outcome is a valid value.
func (o ReleaseOutcome) IsValid() bool {
	switch o {
	case OutcomeSuccess, OutcomeRollback, OutcomeFailed, OutcomePartial:
		return true
	default:
		return false
	}
}

// IsNegative returns true if this outcome indicates a problem.
func (o ReleaseOutcome) IsNegative() bool {
	return o == OutcomeRollback || o == OutcomeFailed || o == OutcomePartial
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
	IncidentRollback     IncidentType = "rollback"      // Release rolled back
	IncidentBugIntro     IncidentType = "bug_intro"     // Bug introduced
	IncidentPerformance  IncidentType = "performance"   // Performance regression
	IncidentSecurity     IncidentType = "security"      // Security issue
	IncidentAvailability IncidentType = "availability"  // Service availability impact
	IncidentDataIssue    IncidentType = "data_issue"    // Data corruption or loss
	IncidentBreaking     IncidentType = "breaking"      // Unexpected breaking change
	IncidentOther        IncidentType = "other"         // Other incident type
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
	mu        sync.RWMutex
	releases  map[string][]*ReleaseRecord // keyed by repository
	incidents map[string][]*IncidentRecord // keyed by repository
	actors    map[string]*ActorMetrics     // keyed by actor ID
}

// NewInMemoryStore creates a new in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		releases:  make(map[string][]*ReleaseRecord),
		incidents: make(map[string][]*IncidentRecord),
		actors:    make(map[string]*ActorMetrics),
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

	s.releases[record.Repository] = append(s.releases[record.Repository], record)

	// Update actor metrics
	s.updateActorMetricsLocked(record)

	return nil
}

// updateActorMetricsLocked updates actor metrics based on a release record.
// Must be called with the lock held.
func (s *InMemoryStore) updateActorMetricsLocked(record *ReleaseRecord) {
	actorID := record.Actor.ID
	metrics, exists := s.actors[actorID]
	if !exists {
		metrics = &ActorMetrics{
			ActorID:   actorID,
			ActorKind: record.Actor.Kind,
		}
		s.actors[actorID] = metrics
	}

	now := time.Now()

	// Update counts
	metrics.TotalReleases++
	switch record.Outcome {
	case OutcomeSuccess:
		metrics.SuccessfulReleases++
	case OutcomeFailed, OutcomePartial:
		metrics.FailedReleases++
	case OutcomeRollback:
		metrics.RollbackCount++
		metrics.FailedReleases++
	}

	if record.RiskScore > 0.7 {
		metrics.HighRiskReleases++
	}
	if record.BreakingChanges > 0 {
		metrics.BreakingChangeReleases++
	}

	// Update average risk score (running average)
	n := float64(metrics.TotalReleases)
	metrics.AverageRiskScore = ((n-1)*metrics.AverageRiskScore + record.RiskScore) / n

	// Update success rate
	metrics.SuccessRate = float64(metrics.SuccessfulReleases) / float64(metrics.TotalReleases)

	// Update timestamps
	if metrics.FirstReleaseAt == nil {
		metrics.FirstReleaseAt = &record.ReleasedAt
	}
	metrics.LastReleaseAt = &record.ReleasedAt
	metrics.UpdatedAt = now

	// Recalculate reliability score
	metrics.ReliabilityScore = metrics.CalculateReliabilityScore()
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

	s.incidents[incident.Repository] = append(s.incidents[incident.Repository], incident)

	// Update actor incident count
	if incident.ActorID != "" {
		if metrics, exists := s.actors[incident.ActorID]; exists {
			metrics.IncidentCount++
			metrics.ReliabilityScore = metrics.CalculateReliabilityScore()
			metrics.UpdatedAt = time.Now()
		}
	}

	return nil
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
	copy := *metrics
	return &copy, nil
}

// GetRiskPatterns returns historical risk patterns for a repository.
func (s *InMemoryStore) GetRiskPatterns(ctx context.Context, repository string) (*RiskPatterns, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	releases := s.releases[repository]
	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found for repository: %s", repository)
	}

	// Calculate patterns from historical data
	patterns := &RiskPatterns{
		Repository:    repository,
		TotalReleases: len(releases),
		UpdatedAt:     time.Now(),
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
	if len(releases) >= 4 {
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

		diff := secondHalfAvg - firstHalfAvg
		if diff > 0.1 {
			patterns.RiskTrend = TrendIncreasing
		} else if diff < -0.1 {
			patterns.RiskTrend = TrendDecreasing
		} else {
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

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
