package cgp

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ChangeProposal represents a request to release changes.
// This is the primary input to the CGP governance process.
type ChangeProposal struct {
	// CGPVersion is the protocol version (e.g., "0.1").
	CGPVersion string `json:"cgpVersion"`

	// Type is always "change.proposal" for proposals.
	Type MessageType `json:"type"`

	// ID is a unique identifier for this proposal.
	ID string `json:"id"`

	// Timestamp is when the proposal was created.
	Timestamp time.Time `json:"timestamp"`

	// Actor is who is proposing the change.
	Actor Actor `json:"actor"`

	// Scope defines what changes are included.
	Scope ProposalScope `json:"scope"`

	// Intent describes the proposer's understanding of the changes.
	Intent ProposalIntent `json:"intent"`

	// Context provides additional information for evaluation.
	Context *ProposalContext `json:"context,omitempty"`
}

// ProposalScope defines what changes are included in a proposal.
type ProposalScope struct {
	// Repository is the target repository in "owner/repo" format.
	Repository string `json:"repository"`

	// Branch is the target branch (optional, defaults to default branch).
	Branch string `json:"branch,omitempty"`

	// CommitRange specifies the commits in "from..to" format.
	// Example: "abc123..def456" or "HEAD~5..HEAD".
	CommitRange string `json:"commitRange"`

	// Commits is an optional list of individual commit SHAs.
	Commits []string `json:"commits,omitempty"`

	// Files is an optional list of changed file paths.
	Files []string `json:"files,omitempty"`
}

// BumpType represents a semantic version bump type.
type BumpType string

// Supported bump types.
const (
	BumpTypeMajor BumpType = "major"
	BumpTypeMinor BumpType = "minor"
	BumpTypePatch BumpType = "patch"
)

// String returns the string representation of the bump type.
func (b BumpType) String() string {
	return string(b)
}

// IsValid returns true if the bump type is recognized.
func (b BumpType) IsValid() bool {
	switch b {
	case BumpTypeMajor, BumpTypeMinor, BumpTypePatch:
		return true
	default:
		return false
	}
}

// ProposalIntent describes the proposer's understanding of the changes.
type ProposalIntent struct {
	// Summary is a human-readable description of what changed.
	Summary string `json:"summary"`

	// SuggestedBump is the proposer's suggested version bump.
	// One of: "major", "minor", "patch".
	SuggestedBump BumpType `json:"suggestedBump,omitempty"`

	// Confidence is the proposer's confidence in their assessment (0.0-1.0).
	Confidence float64 `json:"confidence"`

	// Categories classifies the changes.
	// Examples: "feature", "bugfix", "security", "performance", "documentation".
	Categories []string `json:"categories,omitempty"`

	// BreakingChanges lists known breaking changes.
	BreakingChanges []string `json:"breakingChanges,omitempty"`
}

// ProposalContext provides additional information for evaluation.
type ProposalContext struct {
	// Issues links to related issue tracker items.
	Issues []IssueReference `json:"issues,omitempty"`

	// AgentSession is an optional session ID for multi-turn agent conversations.
	AgentSession string `json:"agentSession,omitempty"`

	// PriorProposals lists previous proposal IDs from this session.
	PriorProposals []string `json:"priorProposals,omitempty"`

	// Metadata contains arbitrary additional context.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// IssueReference links to an external issue tracker.
type IssueReference struct {
	// Provider is the issue tracker: "github", "gitlab", "jira", "linear".
	Provider string `json:"provider"`

	// ID is the issue identifier.
	ID string `json:"id"`

	// URL is an optional direct link to the issue.
	URL string `json:"url,omitempty"`
}

// NewProposal creates a new change proposal with generated ID and timestamp.
func NewProposal(actor Actor, scope ProposalScope, intent ProposalIntent) *ChangeProposal {
	return &ChangeProposal{
		CGPVersion: Version,
		Type:       MessageTypeProposal,
		ID:         GenerateProposalID(),
		Timestamp:  time.Now().UTC(),
		Actor:      actor,
		Scope:      scope,
		Intent:     intent,
	}
}

// GenerateProposalID generates a unique proposal ID.
func GenerateProposalID() string {
	return fmt.Sprintf("prop_%s", uuid.New().String()[:12])
}

// Validate checks if the proposal is valid.
func (p *ChangeProposal) Validate() error {
	if p.CGPVersion == "" {
		return fmt.Errorf("CGP version is required")
	}
	if p.Type != MessageTypeProposal {
		return fmt.Errorf("invalid message type for proposal: %s", p.Type)
	}
	if p.ID == "" {
		return fmt.Errorf("proposal ID is required")
	}
	if err := p.Actor.Validate(); err != nil {
		return fmt.Errorf("invalid actor: %w", err)
	}
	if err := p.Scope.Validate(); err != nil {
		return fmt.Errorf("invalid scope: %w", err)
	}
	if err := p.Intent.Validate(); err != nil {
		return fmt.Errorf("invalid intent: %w", err)
	}
	return nil
}

// Validate checks if the scope is valid.
func (s ProposalScope) Validate() error {
	if s.Repository == "" {
		return fmt.Errorf("repository is required")
	}
	if s.CommitRange == "" && len(s.Commits) == 0 {
		return fmt.Errorf("either commitRange or commits is required")
	}
	return nil
}

// Validate checks if the intent is valid.
func (i ProposalIntent) Validate() error {
	if i.Summary == "" {
		return fmt.Errorf("summary is required")
	}
	if i.Confidence < 0 || i.Confidence > 1 {
		return fmt.Errorf("confidence must be between 0.0 and 1.0")
	}
	if i.SuggestedBump != "" && !i.SuggestedBump.IsValid() {
		return fmt.Errorf("invalid suggested bump: %s", i.SuggestedBump)
	}
	return nil
}

// HasBreakingChanges returns true if the intent includes breaking changes.
func (i ProposalIntent) HasBreakingChanges() bool {
	return len(i.BreakingChanges) > 0
}

// IsHighConfidence returns true if the proposer has high confidence (>= 0.8).
func (i ProposalIntent) IsHighConfidence() bool {
	return i.Confidence >= 0.8
}

// IsMediumConfidence returns true if confidence is medium (0.5-0.8).
func (i ProposalIntent) IsMediumConfidence() bool {
	return i.Confidence >= 0.5 && i.Confidence < 0.8
}

// IsLowConfidence returns true if confidence is low (< 0.5).
func (i ProposalIntent) IsLowConfidence() bool {
	return i.Confidence < 0.5
}

// WithContext adds context to the proposal.
func (p *ChangeProposal) WithContext(ctx *ProposalContext) *ChangeProposal {
	p.Context = ctx
	return p
}

// AddIssue adds an issue reference to the proposal context.
func (p *ChangeProposal) AddIssue(provider, id, url string) *ChangeProposal {
	if p.Context == nil {
		p.Context = &ProposalContext{}
	}
	p.Context.Issues = append(p.Context.Issues, IssueReference{
		Provider: provider,
		ID:       id,
		URL:      url,
	})
	return p
}

// AddMetadata adds metadata to the proposal context.
func (p *ChangeProposal) AddMetadata(key string, value any) *ChangeProposal {
	if p.Context == nil {
		p.Context = &ProposalContext{}
	}
	if p.Context.Metadata == nil {
		p.Context.Metadata = make(map[string]any)
	}
	p.Context.Metadata[key] = value
	return p
}
