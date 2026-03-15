// Package analytics provides analytics event capture, storage, and aggregation
// for release governance metrics in Relicta.
package analytics

import (
	"encoding/json"
	"time"
)

// EventType represents the type of analytics event.
type EventType string

const (
	// EventRiskEvaluation is recorded when a risk assessment is performed.
	EventRiskEvaluation EventType = "risk_evaluation"
	// EventPolicyDecision is recorded when a policy decision is made.
	EventPolicyDecision EventType = "policy_decision"
	// EventApprovalOutcome is recorded when an approval completes.
	EventApprovalOutcome EventType = "approval_outcome"
	// EventReleaseDuration is recorded when a release completes with timing data.
	EventReleaseDuration EventType = "release_duration"
	// EventBumpType is recorded when a version bump is determined.
	EventBumpType EventType = "bump_type"
)

// Valid returns true if the event type is a known type.
func (t EventType) Valid() bool {
	switch t {
	case EventRiskEvaluation, EventPolicyDecision, EventApprovalOutcome,
		EventReleaseDuration, EventBumpType:
		return true
	}
	return false
}

// Event represents a single analytics event.
type Event struct {
	ID        string          `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	Type      EventType       `json:"event_type"`
	ReleaseID string          `json:"release_id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

// RiskEvaluationPayload is the payload for EventRiskEvaluation.
type RiskEvaluationPayload struct {
	RiskScore float64  `json:"risk_score"`
	RiskLevel string   `json:"risk_level"`
	Factors   []string `json:"factors,omitempty"`
}

// PolicyDecisionPayload is the payload for EventPolicyDecision.
type PolicyDecisionPayload struct {
	Decision      string  `json:"decision"` // approve, deny, require_review
	RiskScore     float64 `json:"risk_score"`
	PolicyMatched string  `json:"policy_matched,omitempty"`
	AutoApproved  bool    `json:"auto_approved"`
}

// ApprovalOutcomePayload is the payload for EventApprovalOutcome.
type ApprovalOutcomePayload struct {
	Outcome    string `json:"outcome"` // approved, rejected
	ActorID    string `json:"actor_id"`
	ActorKind  string `json:"actor_kind"` // human, ci, ai_agent
	DurationMs int64  `json:"duration_ms"`
}

// ReleaseDurationPayload is the payload for EventReleaseDuration.
type ReleaseDurationPayload struct {
	DurationMs int64  `json:"duration_ms"`
	Success    bool   `json:"success"`
	Version    string `json:"version,omitempty"`
}

// BumpTypePayload is the payload for EventBumpType.
type BumpTypePayload struct {
	BumpType       string `json:"bump_type"` // major, minor, patch
	CurrentVersion string `json:"current_version"`
	NextVersion    string `json:"next_version"`
}

// MarshalPayload marshals a typed payload to json.RawMessage for embedding in an Event.
func MarshalPayload(v any) (json.RawMessage, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}
