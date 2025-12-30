// Package dto provides data transfer objects for the dashboard API.
package dto

import "time"

// PaginatedResponse is a generic paginated response.
type PaginatedResponse[T any] struct {
	Data       []T `json:"data"`
	Total      int `json:"total"`
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalPages int `json:"total_pages"`
}

// ErrorResponse is an API error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

// ReleaseDTO is the API representation of a release.
type ReleaseDTO struct {
	ID           string     `json:"id"`
	State        string     `json:"state"`
	BaseRef      string     `json:"base_ref"`
	HeadRef      string     `json:"head_ref"`
	Version      string     `json:"version,omitempty"`
	NextVersion  string     `json:"next_version,omitempty"`
	BumpType     string     `json:"bump_type,omitempty"`
	RiskScore    float64    `json:"risk_score,omitempty"`
	RiskLevel    string     `json:"risk_level,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	ApprovedAt   *time.Time `json:"approved_at,omitempty"`
	ApprovedBy   string     `json:"approved_by,omitempty"`
	PublishedAt  *time.Time `json:"published_at,omitempty"`
	CommitCount  int        `json:"commit_count"`
	ChangeTypes  []string   `json:"change_types,omitempty"`
	ReleaseNotes string     `json:"release_notes,omitempty"`
}

// GovernanceDecisionDTO is the API representation of a governance decision.
type GovernanceDecisionDTO struct {
	ID             string    `json:"id"`
	ReleaseID      string    `json:"release_id"`
	Decision       string    `json:"decision"` // approve, deny, require_review
	RiskScore      float64   `json:"risk_score"`
	RiskLevel      string    `json:"risk_level"`
	Factors        []string  `json:"factors"`
	Timestamp      time.Time `json:"timestamp"`
	ActorID        string    `json:"actor_id,omitempty"`
	ActorKind      string    `json:"actor_kind,omitempty"`
	PolicyMatched  string    `json:"policy_matched,omitempty"`
	RequiresReview bool      `json:"requires_review"`
	ReviewReason   string    `json:"review_reason,omitempty"`
}

// RiskTrendDTO represents a risk trend data point.
type RiskTrendDTO struct {
	Date      time.Time `json:"date"`
	RiskScore float64   `json:"risk_score"`
	Releases  int       `json:"releases"`
}

// FactorDistributionDTO represents risk factor distribution.
type FactorDistributionDTO struct {
	Factor     string  `json:"factor"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// ActorDTO is the API representation of an actor.
type ActorDTO struct {
	ID               string    `json:"id"`
	Kind             string    `json:"kind"` // human, ci, ai_agent
	Name             string    `json:"name,omitempty"`
	ReleaseCount     int       `json:"release_count"`
	SuccessRate      float64   `json:"success_rate"`
	AverageRiskScore float64   `json:"average_risk_score"`
	ReliabilityScore float64   `json:"reliability_score"`
	LastSeen         time.Time `json:"last_seen"`
	TrustLevel       string    `json:"trust_level"` // trusted, standard, probation
}

// ApprovalDTO is the API representation of a pending approval.
type ApprovalDTO struct {
	ReleaseID      string    `json:"release_id"`
	Version        string    `json:"version"`
	RiskScore      float64   `json:"risk_score"`
	RiskLevel      string    `json:"risk_level"`
	RequiresReview bool      `json:"requires_review"`
	ReviewReason   string    `json:"review_reason,omitempty"`
	SubmittedAt    time.Time `json:"submitted_at"`
	SubmittedBy    string    `json:"submitted_by,omitempty"`
	CommitCount    int       `json:"commit_count"`
	Changes        []string  `json:"changes,omitempty"`
}

// AuditEventDTO is the API representation of an audit event.
type AuditEventDTO struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	ReleaseID string         `json:"release_id,omitempty"`
	ActorID   string         `json:"actor_id,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
}

// WebSocketEventDTO is the payload for WebSocket events.
type WebSocketEventDTO struct {
	Type      string    `json:"type"`
	ReleaseID string    `json:"release_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data,omitempty"`
}
