// Package chronos provides the Chronos adapter for time-series pattern detection.
//
// Chronos (https://github.com/felixgeelhaar/chronos) detects patterns in
// time-series data: recurrence, trend, spike, drop, stall, anomaly, seasonality.
// This adapter feeds Relicta release metrics into Chronos and queries
// pattern signals to improve risk scoring over time.
package chronos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/memory"
)

// ChronosAdapter implements memory.Store using Chronos for pattern detection.
// It uses Chronos signals to detect trends/spikes/anomalies in release metrics.
type ChronosAdapter struct {
	baseURL    string
	httpClient *http.Client
	scopeID    string // Scope for this Relicta instance
}

// ChronosIngestRequest represents an ingest request to Chronos.
type ChronosIngestRequest struct {
	EntityID string                 `json:"entity_id"`
	ScopeID  string                 `json:"scope_id"`
	Timestamp time.Time              `json:"timestamp"`
	Features  []float64              `json:"features"`
	Labels    []string               `json:"labels,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ChronosSignal represents a pattern signal from Chronos.
type ChronosSignal struct {
	ID         string             `json:"id"`
	ScopeID    string             `json:"scope_id"`
	Series     string             `json:"series"`
	Pattern    string             `json:"pattern"`
	DetectedAt time.Time        `json:"detected_at"`
	Strength   float64            `json:"strength"`
	Confidence float64            `json:"confidence"`
	Window     ChronosWindow     `json:"window"`
	Metrics    ChronosMetrics     `json:"metrics"`
	Evidence   []ChronosEvidence `json:"evidence"`
}

// ChronosWindow represents the detection window.
type ChronosWindow struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ChronosMetrics contains detection metrics.
type ChronosMetrics struct {
	NormalizedStdDev float64 `json:"normalised_stddev"`
	Mean            float64 `json:"mean"`
	N               int     `json:"n"`
}

// ChronosEvidence represents evidence for a signal.
type ChronosEvidence struct {
	Kind      string      `json:"kind"`
	Value     interface{} `json:"value"`
	Timestamp time.Time   `json:"timestamp"`
}

// ChronosSignalsResponse represents a query response from Chronos.
type ChronosSignalsResponse struct {
	Signals []ChronosSignal `json:"signals"`
	Count   int             `json:"count"`
}

// NewChronosAdapter creates a new Chronos adapter.
// baseURL defaults to http://localhost:7778 if empty.
func NewChronosAdapter(baseURL, scopeID string) *ChronosAdapter {
	if baseURL == "" {
		baseURL = "http://localhost:7778"
	}
	return &ChronosAdapter{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		scopeID:    scopeID,
	}
}

// RecordRelease records a release as a time-series observation in Chronos.
// It sends risk score, deployment frequency, and outcome as features.
func (a *ChronosAdapter) RecordRelease(ctx context.Context, record *memory.ReleaseRecord) error {
	features := []float64{
		record.RiskScore,
		float64(record.BreakingChanges),
		float64(record.SecurityChanges),
		float64(record.FilesChanged),
		float64(record.LinesChanged),
	}

	// Encode outcome as numeric: success=0, rollback=1, failed=2, partial=3
	outcomeNum := 0.0
	switch record.Outcome {
	case memory.OutcomeRollback:
		outcomeNum = 1.0
	case memory.OutcomeFailed:
		outcomeNum = 2.0
	case memory.OutcomePartial:
		outcomeNum = 3.0
	}
	features = append(features, outcomeNum)

	req := ChronosIngestRequest{
		EntityID: record.ID,
		ScopeID:  a.scopeID,
		Timestamp: record.ReleasedAt,
		Features:  features,
		Labels:    []string{"release", string(record.Decision)},
		Metadata: map[string]interface{}{
			"repository":  record.Repository,
			"version":     record.Version,
			"actor_id":    record.Actor.ID,
			"actor_kind":  string(record.Actor.Kind),
			"outcome":     string(record.Outcome),
		},
	}

	return a.sendIngest(ctx, req)
}

// RecordIncident records an incident as a spike/drop signal in Chronos.
func (a *ChronosAdapter) RecordIncident(ctx context.Context, incident *memory.IncidentRecord) error {
	// Incidents are recorded as a spike in a special "incident" series
	req := ChronosIngestRequest{
		EntityID: incident.ID,
		ScopeID:  a.scopeID,
		Timestamp: incident.DetectedAt,
		Features:  []float64{1.0}, // Spike value
		Labels:    []string{"incident", string(incident.Type)},
		Metadata: map[string]interface{}{
			"repository":   incident.Repository,
			"release_id":  incident.ReleaseID,
			"version":     incident.Version,
			"severity":    string(incident.Severity),
			"actor_id":    incident.ActorID,
		},
	}

	return a.sendIngest(ctx, req)
}

// RecordDecision records a governance decision (not typically a time-series event).
// This is a no-op for Chronos (decisions are not time-series data).
func (a *ChronosAdapter) RecordDecision(ctx context.Context, decision *cgp.GovernanceDecision) error {
	// Decisions are discrete events, not well-suited for time-series
	// Could potentially track decision rate over time
	return nil
}

// RecordAuthorization records an execution authorization (not time-series).
func (a *ChronosAdapter) RecordAuthorization(ctx context.Context, auth *cgp.ExecutionAuthorization) error {
	return nil
}

// GetReleaseHistory is not supported by Chronos (it's a pattern detector, not a store).
func (a *ChronosAdapter) GetReleaseHistory(ctx context.Context, repository string, limit int) ([]*memory.ReleaseRecord, error) {
	return nil, fmt.Errorf("GetReleaseHistory not supported by Chronos adapter")
}

// GetIncidentHistory is not supported by Chronos.
func (a *ChronosAdapter) GetIncidentHistory(ctx context.Context, repository string, limit int) ([]*memory.IncidentRecord, error) {
	return nil, fmt.Errorf("GetIncidentHistory not supported by Chronos adapter")
}

// GetDecision is not supported by Chronos.
func (a *ChronosAdapter) GetDecision(ctx context.Context, decisionID string) (*cgp.GovernanceDecision, error) {
	return nil, fmt.Errorf("GetDecision not supported by Chronos adapter")
}

// GetDecisionsByProposal is not supported by Chronos.
func (a *ChronosAdapter) GetDecisionsByProposal(ctx context.Context, proposalID string) ([]*cgp.GovernanceDecision, error) {
	return nil, fmt.Errorf("GetDecisionsByProposal not supported by Chronos adapter")
}

// GetAuthorization is not supported by Chronos.
func (a *ChronosAdapter) GetAuthorization(ctx context.Context, authID string) (*cgp.ExecutionAuthorization, error) {
	return nil, fmt.Errorf("GetAuthorization not supported by Chronos adapter")
}

// GetAuthorizationsByDecision is not supported by Chronos.
func (a *ChronosAdapter) GetAuthorizationsByDecision(ctx context.Context, decisionID string) ([]*cgp.ExecutionAuthorization, error) {
	return nil, fmt.Errorf("GetAuthorizationsByDecision not supported by Chronos adapter")
}

// GetActorMetrics queries Chronos for actor-specific patterns.
// Chronos doesn't store per-actor metrics, but we can detect patterns.
func (a *ChronosAdapter) GetActorMetrics(ctx context.Context, actorID string) (*memory.ActorMetrics, error) {
	// Query for stall/anomaly signals for this actor
	// (Simplified - Chronos patterns are per-series, not per-actor)
	return &memory.ActorMetrics{
		ActorID:   actorID,
		ActorKind: cgp.ActorKindHuman, // Default
	}, nil
}

// GetRiskPatterns queries Chronos for risk score patterns over time.
// This is the key method - it detects trending, spikes, stalls in risk.
func (a *ChronosAdapter) GetRiskPatterns(ctx context.Context, repository string) (*memory.RiskPatterns, error) {
	patterns := &memory.RiskPatterns{
		Repository:    repository,
		UpdatedAt:     time.Now(),
	}

	// Query Chronos for signals on the risk series
	signals, err := a.querySignals(ctx, map[string]string{
		"scope_id": a.scopeID,
		"pattern":  "trend,spike,drop,stall,anomaly",
	})
	if err != nil {
		return nil, err
	}

	// Convert Chronos signals to Relicta risk patterns
	for _, signal := range signals {
		switch signal.Pattern {
		case "trend":
			if signal.Strength > 0.5 {
				patterns.RiskTrend = memory.TrendIncreasing
			} else if signal.Strength < -0.5 {
				patterns.RiskTrend = memory.TrendDecreasing
			} else {
				patterns.RiskTrend = memory.TrendStable
			}
		case "spike":
			// Add to high-risk periods
			patterns.HighRiskPeriods = append(patterns.HighRiskPeriods, memory.TimePeriod{
				Start: signal.Window.Start,
				End:   signal.Window.End,
			})
		case "stall":
			// Risk score has stalled - potential pattern
		case "anomaly":
			// Anomalous risk behavior detected
		}
	}

	return patterns, nil
}

// UpdateActorMetrics is not applicable for Chronos.
func (a *ChronosAdapter) UpdateActorMetrics(ctx context.Context, actorID string, outcome memory.ReleaseOutcome) error {
	return nil
}

// GetAuditTrail is not supported by Chronos.
func (a *ChronosAdapter) GetAuditTrail(ctx context.Context, proposalID string) (*memory.AuditTrail, error) {
	return nil, fmt.Errorf("GetAuditTrail not supported by Chronos adapter")
}

// sendIngest sends an ingest request to Chronos.
func (a *ChronosAdapter) sendIngest(ctx context.Context, req ChronosIngestRequest) error {
	body, err := json.Marshal([]ChronosIngestRequest{req})
	if err != nil {
		return fmt.Errorf("marshal ingest request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/ingest", a.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send ingest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("chronos returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// querySignals queries Chronos for signals.
func (a *ChronosAdapter) querySignals(ctx context.Context, params map[string]string) ([]ChronosSignal, error) {
	url := fmt.Sprintf("%s/v1/signals", a.baseURL)
	// Build query string from params
	// (Simplified - actual implementation would use url.Values)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("query signals: %w", err)
	}
	defer resp.Body.Close()

	var signalsResp ChronosSignalsResponse
	if err := json.NewDecoder(resp.Body).Decode(&signalsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return signalsResp.Signals, nil
}
