package mcp

import cgpsdk "github.com/relicta-tech/relicta/v4/pkg/cgp"

// Structured tool outputs.
//
// These types back the outputSchema advertised by data-returning tools. When a
// handler returns one of these typed values, mcp-go emits both a JSON text
// content block and a typed structuredContent object matching the schema, so
// strict MCP clients can consume the result without re-parsing text.
//
// Field tags mirror the exact JSON keys the handlers previously produced (via
// map[string]any) so the text payload is unchanged apart from key ordering.
// omitempty is used wherever the handler conditionally included a key.

// StatusToolOutput is the structured output for relicta_status when an active
// release is present. Degenerate states (no active release, not configured)
// keep returning their existing status/message text payloads.
type StatusToolOutput struct {
	ReleaseID       string `json:"release_id"`
	State           string `json:"state"`
	Version         string `json:"version"`
	Created         string `json:"created"`
	Updated         string `json:"updated"`
	CanApprove      bool   `json:"can_approve"`
	NextAction      string `json:"next_action"`
	ApprovalMessage string `json:"approval_message,omitempty"`
	Stale           bool   `json:"stale,omitempty"`
	Warning         string `json:"warning,omitempty"`
}

// PlanCommitInfo describes a single analyzed commit in a plan result.
type PlanCommitInfo struct {
	SHA     string `json:"sha"`
	Type    string `json:"type"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Scope   string `json:"scope,omitempty"`
}

// PlanToolOutput is the structured output for relicta_plan.
type PlanToolOutput struct {
	ReleaseID      string           `json:"release_id"`
	CurrentVersion string           `json:"current_version"`
	NextVersion    string           `json:"next_version"`
	ReleaseType    string           `json:"release_type"`
	CommitCount    int              `json:"commit_count"`
	HasBreaking    bool             `json:"has_breaking"`
	HasFeatures    bool             `json:"has_features"`
	HasFixes       bool             `json:"has_fixes"`
	Commits        []PlanCommitInfo `json:"commits,omitempty"`
}

// InferVersionToolOutput is the structured output for relicta_infer_version.
// RiskScore and RiskSeverity are pointers so they are advertised and emitted
// only when risk assessment was requested (include_risk=true), matching the
// prior conditional-inclusion behavior even when the values are zero.
type InferVersionToolOutput struct {
	CurrentVersion string   `json:"current_version"`
	NextVersion    string   `json:"next_version"`
	BumpType       string   `json:"bump_type"`
	HasBreaking    bool     `json:"has_breaking"`
	HasFeatures    bool     `json:"has_features"`
	HasFixes       bool     `json:"has_fixes"`
	CommitCount    int      `json:"commit_count"`
	Confidence     float64  `json:"confidence"`
	Rationale      []string `json:"rationale,omitempty"`
	RiskScore      *float64 `json:"risk_score,omitempty"`
	RiskSeverity   *string  `json:"risk_severity,omitempty"`
}

// ValidateReleaseToolOutput is the structured output for relicta_validate_release.
// It reuses ValidationCheckResult (defined in adapters.go) for check entries.
type ValidateReleaseToolOutput struct {
	Valid          bool                    `json:"valid"`
	CanProceed     bool                    `json:"can_proceed"`
	Recommendation string                  `json:"recommendation"`
	Checks         []ValidationCheckResult `json:"checks,omitempty"`
	BlockingIssues []string                `json:"blocking_issues,omitempty"`
	Warnings       []string                `json:"warnings,omitempty"`
}

// CGPStatusToolOutput is the structured output for cgp_status. Proposal,
// Decision, and Authorization intentionally omit omitempty so a nil value
// renders as JSON null, preserving the prior payload shape.
type CGPStatusToolOutput struct {
	ProposalID    string                         `json:"proposalId"`
	State         string                         `json:"state"`
	Proposal      *cgpsdk.ChangeProposal         `json:"proposal"`
	Decision      *cgpsdk.GovernanceDecision     `json:"decision"`
	Authorization *cgpsdk.ExecutionAuthorization `json:"authorization"`
}
