// Package reputation computes and tracks actor reputation over time based on
// verifiable release outcomes. Reputation scores decay exponentially so that
// recent behavior carries more weight than distant history.
package reputation

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp/memory"
)

// Reputation thresholds for governance decisions.
const (
	ThresholdTrusted    = 0.8 // Can auto-approve minor releases.
	ThresholdReliable   = 0.6 // Normal governance.
	ThresholdProbation  = 0.4 // Enhanced scrutiny.
	ThresholdRestricted = 0.2 // Blocked from auto-approval.
)

// Component weights for the overall score.
const (
	weightReleaseSuccess = 0.35
	weightRiskAccuracy   = 0.20
	weightIncidentRate   = 0.20
	weightRecoverySpeed  = 0.10
	weightConsistency    = 0.15
)

// Trend describes whether an actor's reputation is improving, stable, or declining.
type Trend string

const (
	TrendImproving Trend = "improving"
	TrendStable    Trend = "stable"
	TrendDeclining Trend = "declining"
)

// Score represents a comprehensive reputation assessment.
type Score struct {
	// Overall is the composite score in [0.0, 1.0].
	Overall float64 `json:"overall"`

	// ReleaseSuccess is the success rate weighted by recency.
	ReleaseSuccess float64 `json:"releaseSuccess"`

	// RiskAccuracy measures how accurate the actor's risk predictions are.
	RiskAccuracy float64 `json:"riskAccuracy"`

	// IncidentRate is the inverse of incidents caused.
	IncidentRate float64 `json:"incidentRate"`

	// RecoverySpeed measures how fast incidents are resolved.
	RecoverySpeed float64 `json:"recoverySpeed"`

	// Consistency captures variance in performance (lower variance = higher score).
	Consistency float64 `json:"consistency"`

	// LastUpdated is when this score was computed.
	LastUpdated time.Time `json:"lastUpdated"`

	// SampleSize is the number of release records used.
	SampleSize int `json:"sampleSize"`

	// Trend indicates whether the actor's reputation is improving, stable, or declining.
	Trend Trend `json:"trend"`
}

// Level returns a governance level label based on the overall score.
func (s Score) Level() string {
	switch {
	case s.Overall >= ThresholdTrusted:
		return "trusted"
	case s.Overall >= ThresholdReliable:
		return "reliable"
	case s.Overall >= ThresholdProbation:
		return "probation"
	default:
		return "restricted"
	}
}

// ReputationStore persists reputation scores.
type ReputationStore interface {
	// GetScore returns the current reputation score for an actor.
	GetScore(ctx context.Context, actorID string) (*Score, error)

	// SaveScore persists a reputation score for an actor.
	SaveScore(ctx context.Context, actorID string, score *Score) error

	// GetHistory returns historical scores for an actor, most recent first.
	GetHistory(ctx context.Context, actorID string, limit int) ([]Score, error)
}

// EngineOption configures the Engine.
type EngineOption func(*Engine)

// WithDecayHalfLife sets the half-life for exponential decay.
func WithDecayHalfLife(d time.Duration) EngineOption {
	return func(e *Engine) {
		if d > 0 {
			e.decayHalfLife = d
		}
	}
}

// WithLogger sets a custom logger for the engine.
func WithLogger(logger *slog.Logger) EngineOption {
	return func(e *Engine) {
		e.logger = logger
	}
}

// Engine computes and tracks actor reputation over time.
type Engine struct {
	store         ReputationStore
	decayHalfLife time.Duration
	logger        *slog.Logger
	now           func() time.Time // for testing
}

// NewEngine creates a new reputation engine.
func NewEngine(store ReputationStore, opts ...EngineOption) (*Engine, error) {
	if store == nil {
		return nil, fmt.Errorf("reputation store is required")
	}

	e := &Engine{
		store:         store,
		decayHalfLife: 90 * 24 * time.Hour, // 90 days
		logger:        slog.Default(),
		now:           time.Now,
	}

	for _, opt := range opts {
		opt(e)
	}

	return e, nil
}

// ComputeScore calculates a reputation score from release records for a given actor.
// Records with no matching actor releases produce a neutral score of 0.5.
func (e *Engine) ComputeScore(records []*memory.ReleaseRecord, incidents []*memory.IncidentRecord, actorID string) Score {
	now := e.now()

	// Filter records for this actor.
	var actorRecords []*memory.ReleaseRecord
	for _, r := range records {
		if r.Actor.ID == actorID {
			actorRecords = append(actorRecords, r)
		}
	}

	if len(actorRecords) == 0 {
		return Score{
			Overall:        0.5,
			ReleaseSuccess: 0.5,
			RiskAccuracy:   0.5,
			IncidentRate:   0.5,
			RecoverySpeed:  0.5,
			Consistency:    0.5,
			LastUpdated:    now,
			SampleSize:     0,
			Trend:          TrendStable,
		}
	}

	// Build incident map: releaseID -> incidents for this actor.
	actorIncidents := make(map[string][]*memory.IncidentRecord)
	for _, inc := range incidents {
		if inc.ActorID == actorID {
			actorIncidents[inc.ReleaseID] = append(actorIncidents[inc.ReleaseID], inc)
		}
	}

	releaseSuccess := e.computeReleaseSuccess(actorRecords, now)
	riskAccuracy := e.computeRiskAccuracy(actorRecords, now)
	incidentRate := e.computeIncidentRate(actorRecords, actorIncidents, now)
	recoverySpeed := e.computeRecoverySpeed(actorIncidents)
	consistency := e.computeConsistency(actorRecords)

	overall := releaseSuccess*weightReleaseSuccess +
		riskAccuracy*weightRiskAccuracy +
		incidentRate*weightIncidentRate +
		recoverySpeed*weightRecoverySpeed +
		consistency*weightConsistency

	overall = clamp(overall, 0.0, 1.0)

	trend := e.computeTrend(actorRecords, now)

	return Score{
		Overall:        overall,
		ReleaseSuccess: releaseSuccess,
		RiskAccuracy:   riskAccuracy,
		IncidentRate:   incidentRate,
		RecoverySpeed:  recoverySpeed,
		Consistency:    consistency,
		LastUpdated:    now,
		SampleSize:     len(actorRecords),
		Trend:          trend,
	}
}

// UpdateScore computes and persists a reputation score for the given actor.
func (e *Engine) UpdateScore(ctx context.Context, records []*memory.ReleaseRecord, incidents []*memory.IncidentRecord, actorID string) (*Score, error) {
	score := e.ComputeScore(records, incidents, actorID)

	if err := e.store.SaveScore(ctx, actorID, &score); err != nil {
		return nil, fmt.Errorf("saving reputation score: %w", err)
	}

	e.logger.Info("reputation score updated",
		"actor_id", actorID,
		"overall", score.Overall,
		"level", score.Level(),
		"trend", score.Trend,
		"sample_size", score.SampleSize,
	)

	return &score, nil
}

// GetScore returns the current reputation score for an actor.
func (e *Engine) GetScore(ctx context.Context, actorID string) (*Score, error) {
	return e.store.GetScore(ctx, actorID)
}

// GetHistory returns historical scores for an actor.
func (e *Engine) GetHistory(ctx context.Context, actorID string, limit int) ([]Score, error) {
	return e.store.GetHistory(ctx, actorID, limit)
}

// computeReleaseSuccess calculates the success rate with exponential decay.
// Recent releases are weighted more heavily.
func (e *Engine) computeReleaseSuccess(records []*memory.ReleaseRecord, now time.Time) float64 {
	decayLambda := math.Ln2 / e.decayHalfLife.Hours()

	var weightedSuccesses, totalWeight float64
	for _, r := range records {
		hoursAgo := now.Sub(r.ReleasedAt).Hours()
		weight := math.Exp(-decayLambda * hoursAgo)

		totalWeight += weight
		if r.Outcome == memory.OutcomeSuccess {
			weightedSuccesses += weight
		}
	}

	if totalWeight == 0 {
		return 0.5
	}

	return weightedSuccesses / totalWeight
}

// computeRiskAccuracy measures how well an actor's releases correlate with outcomes.
// If an actor consistently ships low-risk changes that succeed and high-risk ones
// that fail, the score is high. Random correlation yields ~0.5.
func (e *Engine) computeRiskAccuracy(records []*memory.ReleaseRecord, now time.Time) float64 {
	decayLambda := math.Ln2 / e.decayHalfLife.Hours()

	var weightedCorrect, totalWeight float64
	for _, r := range records {
		hoursAgo := now.Sub(r.ReleasedAt).Hours()
		weight := math.Exp(-decayLambda * hoursAgo)
		totalWeight += weight

		isNegative := r.Outcome.IsNegative()
		highRisk := r.RiskScore >= 0.5

		// Correct prediction: low risk + success, or high risk + negative.
		if (highRisk && isNegative) || (!highRisk && !isNegative) {
			weightedCorrect += weight
		}
	}

	if totalWeight == 0 {
		return 0.5
	}

	return weightedCorrect / totalWeight
}

// computeIncidentRate calculates 1.0 - (incidents / total releases) with decay.
func (e *Engine) computeIncidentRate(records []*memory.ReleaseRecord, incidents map[string][]*memory.IncidentRecord, now time.Time) float64 {
	decayLambda := math.Ln2 / e.decayHalfLife.Hours()

	var weightedIncidents, totalWeight float64
	for _, r := range records {
		hoursAgo := now.Sub(r.ReleasedAt).Hours()
		weight := math.Exp(-decayLambda * hoursAgo)
		totalWeight += weight

		if len(incidents[r.ID]) > 0 {
			weightedIncidents += weight
		}
	}

	if totalWeight == 0 {
		return 0.5
	}

	incidentRatio := weightedIncidents / totalWeight
	return clamp(1.0-incidentRatio, 0.0, 1.0)
}

// computeRecoverySpeed calculates the average time-to-recovery for incidents.
// < 1h = 1.0, > 24h = 0.0, linear interpolation between.
func (e *Engine) computeRecoverySpeed(incidents map[string][]*memory.IncidentRecord) float64 {
	var totalRecoveryHours float64
	var count int

	for _, incs := range incidents {
		for _, inc := range incs {
			if inc.TimeToResolve > 0 {
				totalRecoveryHours += inc.TimeToResolve.Hours()
				count++
			}
		}
	}

	if count == 0 {
		// No incidents or no resolved incidents: neutral score.
		return 1.0
	}

	avgHours := totalRecoveryHours / float64(count)

	// Normalize: < 1h = 1.0, > 24h = 0.0, linear between.
	const minHours = 1.0
	const maxHours = 24.0

	if avgHours <= minHours {
		return 1.0
	}
	if avgHours >= maxHours {
		return 0.0
	}

	return 1.0 - (avgHours-minHours)/(maxHours-minHours)
}

// computeConsistency measures the consistency of risk scores.
// 1.0 - normalized standard deviation. Lower variance = higher score.
func (e *Engine) computeConsistency(records []*memory.ReleaseRecord) float64 {
	if len(records) < 2 {
		return 1.0
	}

	// Compute mean risk score.
	var sum float64
	for _, r := range records {
		sum += r.RiskScore
	}
	mean := sum / float64(len(records))

	// Compute standard deviation.
	var sumSqDiff float64
	for _, r := range records {
		diff := r.RiskScore - mean
		sumSqDiff += diff * diff
	}
	stdDev := math.Sqrt(sumSqDiff / float64(len(records)))

	// Normalize: max possible std dev for [0,1] range is 0.5.
	// Score = 1.0 - (stdDev / 0.5), clamped to [0, 1].
	normalized := stdDev / 0.5
	return clamp(1.0-normalized, 0.0, 1.0)
}

// computeTrend compares the last 30 days vs the previous 30 days.
func (e *Engine) computeTrend(records []*memory.ReleaseRecord, now time.Time) Trend {
	thirtyDaysAgo := now.Add(-30 * 24 * time.Hour)
	sixtyDaysAgo := now.Add(-60 * 24 * time.Hour)

	var recentSuccess, recentTotal float64
	var previousSuccess, previousTotal float64

	for _, r := range records {
		if r.ReleasedAt.After(thirtyDaysAgo) {
			recentTotal++
			if r.Outcome == memory.OutcomeSuccess {
				recentSuccess++
			}
		} else if r.ReleasedAt.After(sixtyDaysAgo) {
			previousTotal++
			if r.Outcome == memory.OutcomeSuccess {
				previousSuccess++
			}
		}
	}

	// Need data in both windows to compute trend.
	if recentTotal == 0 || previousTotal == 0 {
		return TrendStable
	}

	recentRate := recentSuccess / recentTotal
	previousRate := previousSuccess / previousTotal

	diff := recentRate - previousRate
	switch {
	case diff > 0.05:
		return TrendImproving
	case diff < -0.05:
		return TrendDeclining
	default:
		return TrendStable
	}
}

// clamp restricts v to the range [lo, hi].
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
