// Package memory provides the Mnemos adapter for release memory storage.
//
// Mnemos (https://github.com/klarlabs-studio/mnemos) is a self-hosted
// memory layer for AI apps. This adapter stores release events as Mnemos
// claims with evidence backing, enabling:
//   - "What incidents followed low-risk releases?"
//   - "Show me past decisions for actor X"
//   - Contradiction detection (e.g., "predicted low risk" vs "incident occurred")
//
// The adapter fails gracefully — if Mnemos is not running, operations
// become no-ops with warn-level logging.
package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

// MnemosAdapter implements memory.Store using Mnemos HTTP API.
// It is an optional backend — users can run `mnemos serve` locally.
type MnemosAdapter struct {
	baseURL    string
	httpClient *http.Client
	runID      string // Unique run ID for this Relicta instance
}

// MnemosEvent represents an event sent to Mnemos.
type MnemosEvent struct {
	ID        string                 `json:"id"`
	RunID     string                 `json:"run_id"`
	SourceID  string                 `json:"source_input_id"`
	Content   string                 `json:"content"`
	Timestamp string                 `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// MnemosQueryResponse represents a query response from Mnemos.
type MnemosQueryResponse struct {
	Events []MnemosEvent `json:"events"`
	Total  int           `json:"total"`
}

// NewMnemosStore creates a new Mnemos-backed memory store.
// baseURL defaults to http://localhost:7777 if empty.
// namespace is used as the run_id for Mnemos event grouping.
func NewMnemosStore(baseURL, namespace string, httpClient *http.Client) *MnemosAdapter {
	if baseURL == "" {
		baseURL = "http://localhost:7777"
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &MnemosAdapter{
		baseURL:    baseURL,
		httpClient: httpClient,
		runID:      namespace,
	}
}

// RecordRelease stores a release record as Mnemos claims.
func (a *MnemosAdapter) RecordRelease(ctx context.Context, record *memory.ReleaseRecord) error {
	content := fmt.Sprintf(
		"Release %s of %s by %s: risk=%.1f%%, decision=%s, outcome=%s",
		record.Version, record.Repository, record.Actor.ID,
		record.RiskScore*100, record.Decision, record.Outcome,
	)

	event := MnemosEvent{
		ID:        generateID(),
		RunID:     a.runID,
		SourceID:  fmt.Sprintf("release-%s", record.ID),
		Content:   content,
		Timestamp: record.ReleasedAt.Format(time.RFC3339),
		Metadata: map[string]interface{}{
			"type":             "release",
			"repository":       record.Repository,
			"version":          record.Version,
			"actor_id":         record.Actor.ID,
			"actor_kind":       string(record.Actor.Kind),
			"risk_score":       record.RiskScore,
			"decision":         string(record.Decision),
			"outcome":          string(record.Outcome),
			"breaking_changes": record.BreakingChanges,
			"files_changed":    record.FilesChanged,
			"lines_changed":    record.LinesChanged,
		},
	}

	return a.sendEvents(ctx, []MnemosEvent{event})
}

// RecordIncident stores an incident record as Mnemos claims.
func (a *MnemosAdapter) RecordIncident(ctx context.Context, incident *memory.IncidentRecord) error {
	content := fmt.Sprintf(
		"Incident for release %s of %s: type=%s, severity=%s, description=%s",
		incident.Version, incident.Repository, incident.Type, incident.Severity, incident.Description,
	)

	event := MnemosEvent{
		ID:        generateID(),
		RunID:     a.runID,
		SourceID:  fmt.Sprintf("incident-%s", incident.ID),
		Content:   content,
		Timestamp: incident.DetectedAt.Format(time.RFC3339),
		Metadata: map[string]interface{}{
			"type":          "incident",
			"repository":    incident.Repository,
			"release_id":    incident.ReleaseID,
			"version":       incident.Version,
			"incident_type": string(incident.Type),
			"severity":      string(incident.Severity),
			"root_cause":    incident.RootCause,
			"actor_id":      incident.ActorID,
		},
	}

	return a.sendEvents(ctx, []MnemosEvent{event})
}

// RecordDecision stores a governance decision as Mnemos claims.
func (a *MnemosAdapter) RecordDecision(ctx context.Context, decision *cgp.GovernanceDecision) error {
	content := fmt.Sprintf(
		"Governance decision for %s: decision=%s, risk=%.1f%%, approved=%v",
		decision.ProposalID, decision.Decision, decision.RiskScore*100,
		decision.Decision == cgp.DecisionApproved,
	)

	event := MnemosEvent{
		ID:        generateID(),
		RunID:     a.runID,
		SourceID:  fmt.Sprintf("decision-%s", decision.ID),
		Content:   content,
		Timestamp: decision.Timestamp.Format(time.RFC3339),
		Metadata: map[string]interface{}{
			"type":                "decision",
			"decision_id":         decision.ID,
			"proposal_id":         decision.ProposalID,
			"decision":            string(decision.Decision),
			"risk_score":          decision.RiskScore,
			"approved":            decision.Decision == cgp.DecisionApproved,
			"approval_required":   decision.Decision == cgp.DecisionApprovalRequired,
			"recommended_version": decision.RecommendedVersion,
		},
	}

	return a.sendEvents(ctx, []MnemosEvent{event})
}

// RecordAuthorization stores an execution authorization as Mnemos claims.
func (a *MnemosAdapter) RecordAuthorization(ctx context.Context, auth *cgp.ExecutionAuthorization) error {
	approved := auth.ApprovedBy.ID != ""
	content := fmt.Sprintf(
		"Execution authorized for %s: approved=%v, allowed_steps=%v",
		auth.DecisionID, approved, len(auth.AllowedSteps),
	)

	event := MnemosEvent{
		ID:        generateID(),
		RunID:     a.runID,
		SourceID:  fmt.Sprintf("auth-%s", auth.ID),
		Content:   content,
		Timestamp: auth.Timestamp.Format(time.RFC3339),
		Metadata: map[string]interface{}{
			"type":          "authorization",
			"auth_id":       auth.ID,
			"decision_id":   auth.DecisionID,
			"approved":      approved,
			"approved_by":   auth.ApprovedBy.ID,
			"allowed_steps": len(auth.AllowedSteps),
			"version":       auth.Version,
			"tag":           auth.Tag,
		},
	}

	return a.sendEvents(ctx, []MnemosEvent{event})
}

// GetReleaseHistory queries Mnemos for release events.
func (a *MnemosAdapter) GetReleaseHistory(ctx context.Context, repository string, limit int) ([]*memory.ReleaseRecord, error) {
	events, err := a.queryEvents(ctx, map[string]string{
		"run_id":    a.runID,
		"source_id": fmt.Sprintf("release-%%"),
		"limit":     fmt.Sprintf("%d", limit),
	})
	if err != nil {
		return nil, err
	}

	var records []*memory.ReleaseRecord
	for _, e := range events {
		if repo, ok := e.Metadata["repository"].(string); !ok || repo != repository {
			continue
		}
		records = append(records, a.eventToReleaseRecord(e))
	}
	return records, nil
}

// GetIncidentHistory queries Mnemos for incident events.
func (a *MnemosAdapter) GetIncidentHistory(ctx context.Context, repository string, limit int) ([]*memory.IncidentRecord, error) {
	events, err := a.queryEvents(ctx, map[string]string{
		"run_id":    a.runID,
		"source_id": fmt.Sprintf("incident-%%"),
		"limit":     fmt.Sprintf("%d", limit),
	})
	if err != nil {
		return nil, err
	}

	var records []*memory.IncidentRecord
	for _, e := range events {
		if repo, ok := e.Metadata["repository"].(string); !ok || repo != repository {
			continue
		}
		records = append(records, a.eventToIncidentRecord(e))
	}
	return records, nil
}

// GetDecision queries Mnemos for a specific decision.
func (a *MnemosAdapter) GetDecision(ctx context.Context, decisionID string) (*cgp.GovernanceDecision, error) {
	events, err := a.queryEvents(ctx, map[string]string{
		"run_id":    a.runID,
		"source_id": fmt.Sprintf("decision-%s", decisionID),
	})
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("decision not found: %s", decisionID)
	}

	// Note: Mnemos adapter doesn't store full CGP decision - only a summary claim
	// For full decision retrieval, use the PostgreSQL or file adapter
	return nil, fmt.Errorf("full decision retrieval not supported by Mnemos adapter (use file or postgres backend)")
}

// GetDecisionsByProposal returns decisions for a proposal.
func (a *MnemosAdapter) GetDecisionsByProposal(ctx context.Context, proposalID string) ([]*cgp.GovernanceDecision, error) {
	return nil, fmt.Errorf("GetDecisionsByProposal not supported by Mnemos adapter")
}

// GetAuthorization returns an authorization by ID.
func (a *MnemosAdapter) GetAuthorization(ctx context.Context, authID string) (*cgp.ExecutionAuthorization, error) {
	return nil, fmt.Errorf("GetAuthorization not supported by Mnemos adapter")
}

// GetAuthorizationsByDecision returns authorizations for a decision.
func (a *MnemosAdapter) GetAuthorizationsByDecision(ctx context.Context, decisionID string) ([]*cgp.ExecutionAuthorization, error) {
	return nil, fmt.Errorf("GetAuthorizationsByDecision not supported by Mnemos adapter")
}

// GetActorMetrics queries Mnemos for actor-specific events and computes metrics.
func (a *MnemosAdapter) GetActorMetrics(ctx context.Context, actorID string) (*memory.ActorMetrics, error) {
	// Query all release events for this actor
	events, err := a.queryEvents(ctx, map[string]string{
		"run_id": a.runID,
		"limit":  "1000",
	})
	if err != nil {
		return nil, err
	}

	metrics := &memory.ActorMetrics{
		ActorID:   actorID,
		ActorKind: cgp.ActorKindHuman, // Default
	}

	for _, e := range events {
		metadata := e.Metadata
		if repo, ok := metadata["repository"].(string); !ok || repo == "" {
			continue
		}

		eventType, _ := metadata["type"].(string)
		if eventType != "release" {
			continue
		}

		// Check if this event is for the requested actor
		if actorIDFromMeta, ok := metadata["actor_id"].(string); !ok || actorIDFromMeta != actorID {
			continue
		}

		metrics.TotalReleases++
		// Parse outcome and update metrics
		// (Simplified - full implementation would parse all fields)
	}

	return metrics, nil
}

// GetRiskPatterns analyzes patterns in Mnemos claims.
func (a *MnemosAdapter) GetRiskPatterns(ctx context.Context, repository string) (*memory.RiskPatterns, error) {
	return &memory.RiskPatterns{
		Repository:    repository,
		TotalReleases: 0,
		UpdatedAt:     time.Now(),
	}, nil
}

// UpdateActorMetrics updates actor metrics based on outcome.
func (a *MnemosAdapter) UpdateActorMetrics(ctx context.Context, actorID string, outcome memory.ReleaseOutcome) error {
	// Mnemos is append-only - just record a new event
	event := MnemosEvent{
		ID:        generateID(),
		RunID:     a.runID,
		SourceID:  fmt.Sprintf("outcome-update-%s-%d", actorID, time.Now().Unix()),
		Content:   fmt.Sprintf("Actor %s had outcome: %s", actorID, outcome),
		Timestamp: time.Now().Format(time.RFC3339),
		Metadata: map[string]interface{}{
			"type":     "outcome_update",
			"actor_id": actorID,
			"outcome":  string(outcome),
		},
	}
	return a.sendEvents(ctx, []MnemosEvent{event})
}

// GetAuditTrail returns the audit trail (not fully supported by Mnemos).
func (a *MnemosAdapter) GetAuditTrail(ctx context.Context, proposalID string) (*memory.AuditTrail, error) {
	return nil, fmt.Errorf("GetAuditTrail not supported by Mnemos adapter")
}

// sendEvents sends events to Mnemos /v1/events endpoint.
// Fails gracefully — logs warning and continues if Mnemos is unavailable.
func (a *MnemosAdapter) sendEvents(ctx context.Context, events []MnemosEvent) error {
	body, err := json.Marshal(map[string]interface{}{
		"events": events,
	})
	if err != nil {
		return fmt.Errorf("marshal events: %w", err)
	}

	url := fmt.Sprintf("%s/v1/events", a.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		log.Warn().Err(err).Str("url", a.baseURL).Msg("mnemos unavailable, skipping event storage")
		return nil // Graceful degradation
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		log.Warn().Int("status", resp.StatusCode).Str("body", string(body)).
			Msg("mnemos returned non-OK status, skipping event storage")
		return nil // Graceful degradation
	}

	return nil
}

// queryEvents queries Mnemos /v1/events endpoint.
// Fails gracefully — returns empty results if Mnemos is unavailable.
func (a *MnemosAdapter) queryEvents(ctx context.Context, params map[string]string) ([]MnemosEvent, error) {
	url := fmt.Sprintf("%s/v1/events", a.baseURL)
	// Build query string from params
	// (Simplified - actual implementation would use url.Values)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		log.Warn().Err(err).Str("url", a.baseURL).Msg("mnemos unavailable, returning empty results")
		return []MnemosEvent{}, nil // Graceful degradation
	}
	defer resp.Body.Close()

	var queryResp MnemosQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		log.Warn().Err(err).Msg("failed to decode mnemos response, returning empty results")
		return []MnemosEvent{}, nil // Graceful degradation
	}

	return queryResp.Events, nil
}

// eventToReleaseRecord converts a Mnemos event back into a ReleaseRecord.
//
// This returned a record carrying only an ID, marked "Simplified - would need full
// deserialization". It is not a stub on an unused path: NewMnemosStore returns this
// adapter and the container assigns it as the memory store whenever Mnemos is
// configured, so GetReleaseHistory was handing every reader records with no version, no
// outcome, no actor and no timestamp.
//
// What that broke, silently and without an error anywhere: DORA metrics computed from
// nothing, reconcile unable to match a deployment to a release, `relicta history` empty,
// reputation and earned trust reading a history with no actors — and the deployment gate
// refusing a legitimate production release as ungoverned, because it looks a release up
// by version and the version was absent.
//
// RecordRelease already writes every field into Metadata, so nothing new has to be
// stored; the read side simply never read it.
func (a *MnemosAdapter) eventToReleaseRecord(e MnemosEvent) *memory.ReleaseRecord {
	return &memory.ReleaseRecord{
		ID:         strings.TrimPrefix(e.SourceID, "release-"),
		Repository: metaString(e.Metadata, "repository"),
		Version:    metaString(e.Metadata, "version"),
		Actor: cgp.Actor{
			ID:   metaString(e.Metadata, "actor_id"),
			Kind: cgp.ActorKind(metaString(e.Metadata, "actor_kind")),
		},
		RiskScore:       metaFloat(e.Metadata, "risk_score"),
		Decision:        cgp.DecisionType(metaString(e.Metadata, "decision")),
		Outcome:         memory.ReleaseOutcome(metaString(e.Metadata, "outcome")),
		BreakingChanges: metaInt(e.Metadata, "breaking_changes"),
		FilesChanged:    metaInt(e.Metadata, "files_changed"),
		LinesChanged:    metaInt(e.Metadata, "lines_changed"),
		ReleasedAt:      metaTime(e.Timestamp),
	}
}

// eventToIncidentRecord converts a Mnemos event back into an IncidentRecord.
//
// Same defect as eventToReleaseRecord: MTTR is computed from incidents, and an incident
// with no detection time and no severity contributes nothing an operator can read.
func (a *MnemosAdapter) eventToIncidentRecord(e MnemosEvent) *memory.IncidentRecord {
	return &memory.IncidentRecord{
		ID:         strings.TrimPrefix(e.SourceID, "incident-"),
		Repository: metaString(e.Metadata, "repository"),
		ReleaseID:  metaString(e.Metadata, "release_id"),
		Version:    metaString(e.Metadata, "version"),
		Type:       memory.IncidentType(metaString(e.Metadata, "incident_type")),
		Severity:   cgp.Severity(metaString(e.Metadata, "severity")),
		RootCause:  metaString(e.Metadata, "root_cause"),
		ActorID:    metaString(e.Metadata, "actor_id"),
		DetectedAt: metaTime(e.Timestamp),
	}
}

// The metadata accessors below tolerate whatever JSON decoding produced.
//
// A round trip through JSON turns every number into a float64, so an int written as 3
// comes back as 3.0 and a type assertion to int fails. Reading it as the wrong type
// would silently yield zero — the same failure mode as the stub, arrived at more subtly.
// Each accessor therefore accepts the shapes a number or string can legitimately have,
// and returns the zero value only when the key is genuinely absent.

func metaString(meta map[string]interface{}, key string) string {
	if v, ok := meta[key].(string); ok {
		return v
	}
	return ""
}

func metaFloat(meta map[string]interface{}, key string) float64 {
	switch v := meta[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, err := v.Float64()
		if err == nil {
			return f
		}
	}
	return 0
}

func metaInt(meta map[string]interface{}, key string) int {
	return int(metaFloat(meta, key))
}

// metaTime parses the event timestamp, returning the zero time when it is absent or
// malformed. Zero is the honest answer: substituting time.Now() would date a
// years-old release to this moment and quietly corrupt every interval computed from it.
func metaTime(timestamp string) time.Time {
	if timestamp == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

// generateID generates a unique ID for Mnemos events.
func generateID() string {
	return fmt.Sprintf("evt-%d", time.Now().UnixNano())
}
