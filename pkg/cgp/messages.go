package cgp

import "time"

// MessageType identifies the kind of CGP message.
type MessageType string

// Supported CGP message types.
const (
	TypeChangeProposal         MessageType = "change.proposal"
	TypeGovernanceEvaluation   MessageType = "change.evaluation"
	TypeGovernanceDecision     MessageType = "change.decision"
	TypeExecutionAuthorization MessageType = "change.execution_authorized"
)

// IsValid returns true if the message type is a recognized CGP type.
func (t MessageType) IsValid() bool {
	switch t {
	case TypeChangeProposal, TypeGovernanceEvaluation,
		TypeGovernanceDecision, TypeExecutionAuthorization:
		return true
	default:
		return false
	}
}

// String returns the string representation.
func (t MessageType) String() string {
	return string(t)
}

// Actor identifies who is proposing or authorizing a change.
type Actor struct {
	Kind string `json:"kind"` // "agent", "ci", "human", "system"
	ID   string `json:"id"`   // unique identifier, e.g. "agent:cursor", "ci:github-actions"
	Name string `json:"name,omitempty"`
}

// Scope defines which changes are included in a proposal.
type Scope struct {
	Repository  string   `json:"repository"` // "owner/repo" format
	Branch      string   `json:"branch,omitempty"`
	CommitRange string   `json:"commitRange"` // "from..to" format
	Commits     []string `json:"commits,omitempty"`
	Files       []string `json:"files,omitempty"`
}

// Intent describes the proposer's understanding of the changes.
type Intent struct {
	Summary    string   `json:"summary"`
	Confidence float64  `json:"confidence"`           // 0.0 - 1.0
	Categories []string `json:"categories,omitempty"` // "feature", "bugfix", etc.
}

// Metadata holds arbitrary key-value context for a proposal.
type Metadata map[string]any

// ChangeProposal represents a request to release changes.
// It is the primary input to the CGP governance process.
type ChangeProposal struct {
	CGPVersion string      `json:"cgpVersion"`
	Type       MessageType `json:"type"`
	ID         string      `json:"id"`
	Timestamp  time.Time   `json:"timestamp"`
	Actor      Actor       `json:"actor"`
	Scope      Scope       `json:"scope"`
	Intent     Intent      `json:"intent"`
	Metadata   Metadata    `json:"metadata,omitempty"`
}

// RiskFactor describes a single risk contribution.
type RiskFactor struct {
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`    // 0.0 - 1.0
	Severity    string  `json:"severity"` // "low", "medium", "high", "critical"
}

// PolicyResult contains the outcome of one policy evaluation.
type PolicyResult struct {
	PolicyID  string `json:"policyId"`
	Name      string `json:"name"`
	Matched   bool   `json:"matched"`
	Decision  string `json:"decision,omitempty"`
	Rationale string `json:"rationale,omitempty"`
}

// GovernanceEvaluation represents the analysis of a change proposal.
type GovernanceEvaluation struct {
	CGPVersion         string         `json:"cgpVersion"`
	Type               MessageType    `json:"type"`
	ID                 string         `json:"id"`
	ProposalID         string         `json:"proposalId"`
	Timestamp          time.Time      `json:"timestamp"`
	RiskScore          float64        `json:"riskScore"`
	RecommendedVersion string         `json:"recommendedVersion,omitempty"`
	Rationale          []string       `json:"rationale"`
	PolicyResults      []PolicyResult `json:"policyResults,omitempty"`
}

// RequiredAction specifies what must happen before execution.
type RequiredAction struct {
	Type        string `json:"type"` // "human_approval", "test_run", etc.
	Description string `json:"description"`
	Assignee    string `json:"assignee,omitempty"`
}

// Condition constrains when or how execution may proceed.
type Condition struct {
	Type  string `json:"type"` // "time_window", "feature_flag", etc.
	Value string `json:"value"`
}

// GovernanceDecision is the response to a ChangeProposal.
type GovernanceDecision struct {
	CGPVersion         string           `json:"cgpVersion"`
	Type               MessageType      `json:"type"`
	ID                 string           `json:"id"`
	ProposalID         string           `json:"proposalId"`
	Timestamp          time.Time        `json:"timestamp"`
	Decision           string           `json:"decision"` // "approved", "denied", "approval_required"
	RiskScore          float64          `json:"riskScore"`
	RecommendedVersion string           `json:"recommendedVersion,omitempty"`
	Rationale          []string         `json:"rationale"`
	RequiredActions    []RequiredAction `json:"requiredActions,omitempty"`
	Conditions         []Condition      `json:"conditions,omitempty"`
}

// ExecutionAuthorization grants permission to execute a release.
type ExecutionAuthorization struct {
	CGPVersion string      `json:"cgpVersion"`
	Type       MessageType `json:"type"`
	ID         string      `json:"id"`
	ProposalID string      `json:"proposalId"`
	DecisionID string      `json:"decisionId"`
	Timestamp  time.Time   `json:"timestamp"`
	ApprovedBy Actor       `json:"approvedBy"`
	Version    string      `json:"version"`
	ValidUntil time.Time   `json:"validUntil"`
	Scope      []string    `json:"scope,omitempty"` // allowed execution steps
}
