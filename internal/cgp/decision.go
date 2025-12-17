package cgp

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DecisionType represents the governance outcome.
type DecisionType string

// Decision types supported by CGP.
const (
	DecisionApproved         DecisionType = "approved"          // Auto-approved, ready for execution
	DecisionApprovalRequired DecisionType = "approval_required" // Needs human review
	DecisionRejected         DecisionType = "rejected"          // Policy violation, blocked
	DecisionDeferred         DecisionType = "deferred"          // Needs more information
)

// AllDecisionTypes returns all supported decision types.
func AllDecisionTypes() []DecisionType {
	return []DecisionType{
		DecisionApproved,
		DecisionApprovalRequired,
		DecisionRejected,
		DecisionDeferred,
	}
}

// String returns the string representation of the decision type.
func (d DecisionType) String() string {
	return string(d)
}

// IsValid returns true if the decision type is recognized.
func (d DecisionType) IsValid() bool {
	switch d {
	case DecisionApproved, DecisionApprovalRequired, DecisionRejected, DecisionDeferred:
		return true
	default:
		return false
	}
}

// AllowsExecution returns true if this decision allows release execution.
func (d DecisionType) AllowsExecution() bool {
	return d == DecisionApproved
}

// RequiresHumanAction returns true if human intervention is needed.
func (d DecisionType) RequiresHumanAction() bool {
	return d == DecisionApprovalRequired || d == DecisionDeferred
}

// IsTerminal returns true if this decision is final (no further action possible).
func (d DecisionType) IsTerminal() bool {
	return d == DecisionApproved || d == DecisionRejected
}

// GovernanceDecision is the response to a ChangeProposal.
type GovernanceDecision struct {
	// CGPVersion is the protocol version.
	CGPVersion string `json:"cgpVersion"`

	// Type is always "change.decision" for decisions.
	Type MessageType `json:"type"`

	// ID is a unique identifier for this decision.
	ID string `json:"id"`

	// ProposalID links to the original proposal.
	ProposalID string `json:"proposalId"`

	// Timestamp is when the decision was made.
	Timestamp time.Time `json:"timestamp"`

	// Decision is the governance outcome.
	Decision DecisionType `json:"decision"`

	// RecommendedVersion is the calculated version (e.g., "1.2.0").
	RecommendedVersion string `json:"recommendedVersion,omitempty"`

	// RiskScore is the overall risk assessment (0.0-1.0).
	RiskScore float64 `json:"riskScore"`

	// RiskFactors are the contributing risk factors.
	RiskFactors []RiskFactor `json:"riskFactors"`

	// Rationale explains the decision.
	Rationale []string `json:"rationale"`

	// RequiredActions specify what must happen before execution.
	RequiredActions []RequiredAction `json:"requiredActions,omitempty"`

	// Conditions specify constraints on execution.
	Conditions []Condition `json:"conditions,omitempty"`

	// Analysis contains detailed change analysis results.
	Analysis *ChangeAnalysis `json:"analysis,omitempty"`
}

// Severity represents the severity level of a risk factor.
type Severity string

// Severity levels.
const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// RiskFactor describes a specific risk contribution.
type RiskFactor struct {
	// Category identifies the type of risk.
	// Examples: "api_change", "dependency", "security", "blast_radius", "historical".
	Category string `json:"category"`

	// Description is a human-readable explanation.
	Description string `json:"description"`

	// Score is the contribution to overall risk (0.0-1.0).
	Score float64 `json:"score"`

	// Severity indicates the importance level.
	Severity Severity `json:"severity"`
}

// RequiredAction specifies what must happen before execution.
type RequiredAction struct {
	// Type identifies the action type.
	// Examples: "human_approval", "release_note_review", "security_review", "test_run".
	Type string `json:"type"`

	// Description explains what needs to be done.
	Description string `json:"description"`

	// Assignee is who should perform the action (optional).
	Assignee string `json:"assignee,omitempty"`

	// Deadline is when the action must be completed (ISO8601 duration or timestamp).
	Deadline string `json:"deadline,omitempty"`
}

// Condition specifies constraints on execution.
type Condition struct {
	// Type identifies the condition type.
	// Examples: "time_window", "feature_flag", "manual_gate", "test_pass".
	Type string `json:"type"`

	// Value is condition-specific configuration.
	Value string `json:"value"`
}

// ChangeAnalysis contains detailed analysis results.
type ChangeAnalysis struct {
	// APIChanges lists detected API modifications.
	APIChanges []APIChange `json:"apiChanges,omitempty"`

	// DependencyImpact describes impact on downstream consumers.
	DependencyImpact *DependencyImpact `json:"dependencyImpact,omitempty"`

	// BlastRadius quantifies potential impact.
	BlastRadius *BlastRadius `json:"blastRadius,omitempty"`

	// Commit categorization counts.
	Features     int `json:"features"`
	Fixes        int `json:"fixes"`
	Breaking     int `json:"breaking"`
	Security     int `json:"security"`
	Dependencies int `json:"dependencies"`
	Other        int `json:"other"`
}

// APIChange describes a public API modification.
type APIChange struct {
	// Type is the kind of change: "added", "removed", "modified", "deprecated".
	Type string `json:"type"`

	// Symbol is the function/type/endpoint name.
	Symbol string `json:"symbol"`

	// Location is the file path and optionally line number.
	Location string `json:"location"`

	// Breaking indicates if this is a breaking change.
	Breaking bool `json:"breaking"`

	// Description explains what changed.
	Description string `json:"description"`
}

// DependencyImpact describes impact on downstream consumers.
type DependencyImpact struct {
	// DirectDependents is the count of immediate consumers.
	DirectDependents int `json:"directDependents"`

	// TransitiveDependents is the count of all affected consumers.
	TransitiveDependents int `json:"transitiveDependents"`

	// AffectedServices lists specific services impacted.
	AffectedServices []string `json:"affectedServices,omitempty"`

	// AffectedPackages lists specific packages impacted.
	AffectedPackages []string `json:"affectedPackages,omitempty"`
}

// BlastRadius quantifies potential impact.
type BlastRadius struct {
	// Score is the normalized blast radius (0.0-1.0).
	Score float64 `json:"score"`

	// FilesChanged is the number of files modified.
	FilesChanged int `json:"filesChanged"`

	// LinesChanged is the total lines added/removed/modified.
	LinesChanged int `json:"linesChanged"`

	// Components lists affected system components.
	Components []string `json:"components,omitempty"`
}

// NewDecision creates a new governance decision.
func NewDecision(proposalID string, decision DecisionType) *GovernanceDecision {
	return &GovernanceDecision{
		CGPVersion:  Version,
		Type:        MessageTypeDecision,
		ID:          GenerateDecisionID(),
		ProposalID:  proposalID,
		Timestamp:   time.Now().UTC(),
		Decision:    decision,
		RiskFactors: []RiskFactor{},
		Rationale:   []string{},
	}
}

// GenerateDecisionID generates a unique decision ID.
func GenerateDecisionID() string {
	return fmt.Sprintf("dec_%s", uuid.New().String()[:12])
}

// Validate checks if the decision is valid.
func (d *GovernanceDecision) Validate() error {
	if d.CGPVersion == "" {
		return fmt.Errorf("CGP version is required")
	}
	if d.Type != MessageTypeDecision {
		return fmt.Errorf("invalid message type for decision: %s", d.Type)
	}
	if d.ID == "" {
		return fmt.Errorf("decision ID is required")
	}
	if d.ProposalID == "" {
		return fmt.Errorf("proposal ID is required")
	}
	if !d.Decision.IsValid() {
		return fmt.Errorf("invalid decision type: %s", d.Decision)
	}
	if d.RiskScore < 0 || d.RiskScore > 1 {
		return fmt.Errorf("risk score must be between 0.0 and 1.0")
	}
	return nil
}

// WithRiskScore sets the risk score.
func (d *GovernanceDecision) WithRiskScore(score float64) *GovernanceDecision {
	d.RiskScore = score
	return d
}

// WithRecommendedVersion sets the recommended version.
func (d *GovernanceDecision) WithRecommendedVersion(version string) *GovernanceDecision {
	d.RecommendedVersion = version
	return d
}

// AddRiskFactor adds a risk factor to the decision.
func (d *GovernanceDecision) AddRiskFactor(category, description string, score float64, severity Severity) *GovernanceDecision {
	d.RiskFactors = append(d.RiskFactors, RiskFactor{
		Category:    category,
		Description: description,
		Score:       score,
		Severity:    severity,
	})
	return d
}

// AddRationale adds an explanation to the decision.
func (d *GovernanceDecision) AddRationale(reason string) *GovernanceDecision {
	d.Rationale = append(d.Rationale, reason)
	return d
}

// AddRequiredAction adds a required action.
func (d *GovernanceDecision) AddRequiredAction(actionType, description string) *GovernanceDecision {
	d.RequiredActions = append(d.RequiredActions, RequiredAction{
		Type:        actionType,
		Description: description,
	})
	return d
}

// AddCondition adds a condition constraint.
func (d *GovernanceDecision) AddCondition(condType, value string) *GovernanceDecision {
	d.Conditions = append(d.Conditions, Condition{
		Type:  condType,
		Value: value,
	})
	return d
}

// WithAnalysis sets the change analysis.
func (d *GovernanceDecision) WithAnalysis(analysis *ChangeAnalysis) *GovernanceDecision {
	d.Analysis = analysis
	return d
}

// IsHighRisk returns true if the risk score indicates high risk (>= 0.7).
func (d *GovernanceDecision) IsHighRisk() bool {
	return d.RiskScore >= 0.7
}

// IsMediumRisk returns true if the risk score indicates medium risk (0.4-0.7).
func (d *GovernanceDecision) IsMediumRisk() bool {
	return d.RiskScore >= 0.4 && d.RiskScore < 0.7
}

// IsLowRisk returns true if the risk score indicates low risk (< 0.4).
func (d *GovernanceDecision) IsLowRisk() bool {
	return d.RiskScore < 0.4
}

// RiskSeverity returns the severity classification based on risk score.
func (d *GovernanceDecision) RiskSeverity() Severity {
	switch {
	case d.RiskScore >= 0.8:
		return SeverityCritical
	case d.RiskScore >= 0.6:
		return SeverityHigh
	case d.RiskScore >= 0.4:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

// HasBreakingChanges returns true if the analysis found breaking changes.
func (d *GovernanceDecision) HasBreakingChanges() bool {
	if d.Analysis == nil {
		return false
	}
	return d.Analysis.Breaking > 0
}

// HasSecurityImpact returns true if the analysis found security-related changes.
func (d *GovernanceDecision) HasSecurityImpact() bool {
	if d.Analysis == nil {
		return false
	}
	return d.Analysis.Security > 0
}

// TotalChanges returns the total number of categorized changes.
func (a *ChangeAnalysis) TotalChanges() int {
	if a == nil {
		return 0
	}
	return a.Features + a.Fixes + a.Breaking + a.Security + a.Dependencies + a.Other
}

// HasAPIChanges returns true if there are any API changes.
func (a *ChangeAnalysis) HasAPIChanges() bool {
	return a != nil && len(a.APIChanges) > 0
}

// BreakingAPIChanges returns only the breaking API changes.
func (a *ChangeAnalysis) BreakingAPIChanges() []APIChange {
	if a == nil {
		return nil
	}
	var breaking []APIChange
	for _, change := range a.APIChanges {
		if change.Breaking {
			breaking = append(breaking, change)
		}
	}
	return breaking
}
