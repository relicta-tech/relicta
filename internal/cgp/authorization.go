package cgp

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ExecutionStep represents a release execution step.
type ExecutionStep string

// Execution steps supported by CGP.
const (
	ExecutionStepTag          ExecutionStep = "tag"
	ExecutionStepChangelog    ExecutionStep = "changelog"
	ExecutionStepReleaseNotes ExecutionStep = "release_notes"
	ExecutionStepPublish      ExecutionStep = "publish"
	ExecutionStepNotify       ExecutionStep = "notify"
)

// AllExecutionSteps returns all supported execution steps.
func AllExecutionSteps() []ExecutionStep {
	return []ExecutionStep{
		ExecutionStepTag,
		ExecutionStepChangelog,
		ExecutionStepReleaseNotes,
		ExecutionStepPublish,
		ExecutionStepNotify,
	}
}

// String returns the string representation of the execution step.
func (s ExecutionStep) String() string {
	return string(s)
}

// ExecutionAuthorization grants permission to execute a release.
type ExecutionAuthorization struct {
	// CGPVersion is the protocol version.
	CGPVersion string `json:"cgpVersion"`

	// Type is always "change.execution_authorized" for authorizations.
	Type MessageType `json:"type"`

	// ID is a unique identifier for this authorization.
	ID string `json:"id"`

	// DecisionID links to the governance decision.
	DecisionID string `json:"decisionId"`

	// ProposalID links to the original proposal.
	ProposalID string `json:"proposalId"`

	// Timestamp is when the authorization was created.
	Timestamp time.Time `json:"timestamp"`

	// ApprovedBy is the actor who granted approval.
	ApprovedBy Actor `json:"approvedBy"`

	// ApprovedAt is when approval was granted.
	ApprovedAt time.Time `json:"approvedAt"`

	// Version is the final version number to release (e.g., "1.2.0").
	Version string `json:"version"`

	// Tag is the git tag name (e.g., "v1.2.0").
	Tag string `json:"tag"`

	// ReleaseNotes contains the approved release notes.
	ReleaseNotes string `json:"releaseNotes,omitempty"`

	// Changelog contains the approved changelog entry.
	Changelog string `json:"changelog,omitempty"`

	// ValidUntil is when this authorization expires.
	ValidUntil time.Time `json:"validUntil"`

	// AllowedSteps specifies which execution steps are permitted.
	AllowedSteps []ExecutionStep `json:"allowedSteps"`

	// Restrictions lists any limitations on execution.
	Restrictions []string `json:"restrictions,omitempty"`

	// ApprovalChain records the full approval history.
	ApprovalChain []ApprovalRecord `json:"approvalChain"`
}

// ApprovalAction represents the type of approval action.
type ApprovalAction string

// Approval actions.
const (
	ApprovalActionApprove        ApprovalAction = "approve"
	ApprovalActionReject         ApprovalAction = "reject"
	ApprovalActionRequestChanges ApprovalAction = "request_changes"
	ApprovalActionComment        ApprovalAction = "comment"
)

// ApprovalRecord tracks each approval in the chain.
type ApprovalRecord struct {
	// Actor who performed the action.
	Actor Actor `json:"actor"`

	// Action taken: "approve", "reject", "request_changes", "comment".
	Action ApprovalAction `json:"action"`

	// Timestamp when the action was taken.
	Timestamp time.Time `json:"timestamp"`

	// Comment is an optional explanation.
	Comment string `json:"comment,omitempty"`

	// Signature is an optional cryptographic signature.
	Signature string `json:"signature,omitempty"`
}

// NewAuthorization creates a new execution authorization.
func NewAuthorization(decisionID, proposalID string, approvedBy Actor, version string) *ExecutionAuthorization {
	now := time.Now().UTC()
	return &ExecutionAuthorization{
		CGPVersion:    Version,
		Type:          MessageTypeAuthorization,
		ID:            GenerateAuthorizationID(),
		DecisionID:    decisionID,
		ProposalID:    proposalID,
		Timestamp:     now,
		ApprovedBy:    approvedBy,
		ApprovedAt:    now,
		Version:       version,
		Tag:           fmt.Sprintf("v%s", version),
		ValidUntil:    now.Add(24 * time.Hour), // Default 24 hour validity
		AllowedSteps:  AllExecutionSteps(),
		ApprovalChain: []ApprovalRecord{},
	}
}

// GenerateAuthorizationID generates a unique authorization ID.
func GenerateAuthorizationID() string {
	return fmt.Sprintf("auth_%s", uuid.New().String()[:12])
}

// Validate checks if the authorization is valid.
func (a *ExecutionAuthorization) Validate() error {
	if a.CGPVersion == "" {
		return fmt.Errorf("CGP version is required")
	}
	if a.Type != MessageTypeAuthorization {
		return fmt.Errorf("invalid message type for authorization: %s", a.Type)
	}
	if a.ID == "" {
		return fmt.Errorf("authorization ID is required")
	}
	if a.DecisionID == "" {
		return fmt.Errorf("decision ID is required")
	}
	if a.ProposalID == "" {
		return fmt.Errorf("proposal ID is required")
	}
	if err := a.ApprovedBy.Validate(); err != nil {
		return fmt.Errorf("invalid approver: %w", err)
	}
	if a.Version == "" {
		return fmt.Errorf("version is required")
	}
	if len(a.AllowedSteps) == 0 {
		return fmt.Errorf("at least one allowed step is required")
	}
	return nil
}

// IsValid returns true if the authorization is currently valid.
func (a *ExecutionAuthorization) IsValid() bool {
	return time.Now().Before(a.ValidUntil)
}

// IsExpired returns true if the authorization has expired.
func (a *ExecutionAuthorization) IsExpired() bool {
	return time.Now().After(a.ValidUntil)
}

// TimeToExpiry returns the duration until expiry.
func (a *ExecutionAuthorization) TimeToExpiry() time.Duration {
	return time.Until(a.ValidUntil)
}

// IsStepAllowed checks if a specific execution step is permitted.
func (a *ExecutionAuthorization) IsStepAllowed(step ExecutionStep) bool {
	for _, allowed := range a.AllowedSteps {
		if allowed == step {
			return true
		}
	}
	return false
}

// WithValidity sets the authorization validity period.
func (a *ExecutionAuthorization) WithValidity(duration time.Duration) *ExecutionAuthorization {
	a.ValidUntil = a.ApprovedAt.Add(duration)
	return a
}

// WithReleaseNotes sets the release notes.
func (a *ExecutionAuthorization) WithReleaseNotes(notes string) *ExecutionAuthorization {
	a.ReleaseNotes = notes
	return a
}

// WithChangelog sets the changelog entry.
func (a *ExecutionAuthorization) WithChangelog(changelog string) *ExecutionAuthorization {
	a.Changelog = changelog
	return a
}

// WithAllowedSteps sets the allowed execution steps.
func (a *ExecutionAuthorization) WithAllowedSteps(steps ...ExecutionStep) *ExecutionAuthorization {
	a.AllowedSteps = steps
	return a
}

// AddRestriction adds a restriction to the authorization.
func (a *ExecutionAuthorization) AddRestriction(restriction string) *ExecutionAuthorization {
	a.Restrictions = append(a.Restrictions, restriction)
	return a
}

// RecordApproval adds an approval record to the chain.
func (a *ExecutionAuthorization) RecordApproval(actor Actor, action ApprovalAction, comment string) *ExecutionAuthorization {
	a.ApprovalChain = append(a.ApprovalChain, ApprovalRecord{
		Actor:     actor,
		Action:    action,
		Timestamp: time.Now().UTC(),
		Comment:   comment,
	})
	return a
}

// LastApproval returns the most recent approval record.
func (a *ExecutionAuthorization) LastApproval() *ApprovalRecord {
	if len(a.ApprovalChain) == 0 {
		return nil
	}
	return &a.ApprovalChain[len(a.ApprovalChain)-1]
}

// ApprovalCount returns the number of approvals in the chain.
func (a *ExecutionAuthorization) ApprovalCount() int {
	count := 0
	for _, record := range a.ApprovalChain {
		if record.Action == ApprovalActionApprove {
			count++
		}
	}
	return count
}

// HasApprovalFrom checks if a specific actor has approved.
func (a *ExecutionAuthorization) HasApprovalFrom(actorID string) bool {
	for _, record := range a.ApprovalChain {
		if record.Actor.ID == actorID && record.Action == ApprovalActionApprove {
			return true
		}
	}
	return false
}

// HasHumanApproval returns true if at least one human has approved.
func (a *ExecutionAuthorization) HasHumanApproval() bool {
	for _, record := range a.ApprovalChain {
		if record.Actor.IsHuman() && record.Action == ApprovalActionApprove {
			return true
		}
	}
	return false
}
