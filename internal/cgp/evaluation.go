package cgp

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GovernanceEvaluation represents the analysis of a change proposal.
// This is an intermediate step between proposal and decision, containing
// the detailed assessment results from the governance process.
type GovernanceEvaluation struct {
	// CGPVersion is the protocol version.
	CGPVersion string `json:"cgpVersion"`

	// Type is always "change.evaluation" for evaluations.
	Type MessageType `json:"type"`

	// ID is a unique identifier for this evaluation.
	ID string `json:"id"`

	// ProposalID links to the evaluated proposal.
	ProposalID string `json:"proposalId"`

	// Timestamp is when the evaluation was completed.
	Timestamp time.Time `json:"timestamp"`

	// Status indicates the evaluation outcome.
	Status EvaluationStatus `json:"status"`

	// RiskAssessment contains the risk analysis results.
	RiskAssessment *RiskAssessment `json:"riskAssessment"`

	// PolicyResults contains policy evaluation outcomes.
	PolicyResults []PolicyResult `json:"policyResults,omitempty"`

	// VersionAnalysis contains version calculation results.
	VersionAnalysis *VersionAnalysis `json:"versionAnalysis,omitempty"`

	// ChangeAnalysis contains detailed change analysis.
	ChangeAnalysis *ChangeAnalysis `json:"changeAnalysis,omitempty"`

	// Warnings lists non-blocking issues found during evaluation.
	Warnings []EvaluationWarning `json:"warnings,omitempty"`

	// Errors lists blocking issues that prevent proceeding.
	Errors []EvaluationError `json:"errors,omitempty"`

	// Duration is how long the evaluation took.
	Duration time.Duration `json:"duration"`

	// Metadata contains additional evaluation data.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// EvaluationStatus indicates the outcome of evaluation.
type EvaluationStatus string

// Evaluation statuses.
const (
	EvaluationStatusComplete EvaluationStatus = "complete" // Evaluation finished successfully
	EvaluationStatusPartial  EvaluationStatus = "partial"  // Some checks couldn't complete
	EvaluationStatusFailed   EvaluationStatus = "failed"   // Evaluation itself failed
	EvaluationStatusTimeout  EvaluationStatus = "timeout"  // Evaluation timed out
)

// String returns the string representation.
func (s EvaluationStatus) String() string {
	return string(s)
}

// IsValid returns true if the status is recognized.
func (s EvaluationStatus) IsValid() bool {
	switch s {
	case EvaluationStatusComplete, EvaluationStatusPartial, EvaluationStatusFailed, EvaluationStatusTimeout:
		return true
	default:
		return false
	}
}

// RiskAssessment contains the risk analysis results.
type RiskAssessment struct {
	// OverallScore is the combined risk score (0.0-1.0).
	OverallScore float64 `json:"overallScore"`

	// Severity is the risk severity classification.
	Severity Severity `json:"severity"`

	// Factors are the individual risk contributions.
	Factors []RiskFactor `json:"factors"`

	// Confidence indicates how confident we are in this assessment (0.0-1.0).
	Confidence float64 `json:"confidence"`

	// Recommendations are suggested actions based on risk.
	Recommendations []string `json:"recommendations,omitempty"`
}

// PolicyResult contains the outcome of a single policy evaluation.
type PolicyResult struct {
	// PolicyID is the policy that was evaluated.
	PolicyID string `json:"policyId"`

	// PolicyName is the human-readable policy name.
	PolicyName string `json:"policyName"`

	// Matched indicates if the policy matched.
	Matched bool `json:"matched"`

	// Decision is the outcome if matched.
	Decision DecisionType `json:"decision,omitempty"`

	// Actions are the actions triggered by this policy.
	Actions []string `json:"actions,omitempty"`

	// Rationale explains why this policy matched or not.
	Rationale string `json:"rationale,omitempty"`
}

// VersionAnalysis contains version calculation results.
type VersionAnalysis struct {
	// CurrentVersion is the current version before release.
	CurrentVersion string `json:"currentVersion"`

	// RecommendedVersion is the calculated next version.
	RecommendedVersion string `json:"recommendedVersion"`

	// BumpType is the version bump type (major, minor, patch).
	BumpType BumpType `json:"bumpType"`

	// Reasoning explains why this version was chosen.
	Reasoning []string `json:"reasoning,omitempty"`

	// Prerelease indicates if this is a prerelease version.
	Prerelease bool `json:"prerelease"`

	// PrereleaseTag is the prerelease identifier if applicable.
	PrereleaseTag string `json:"prereleaseTag,omitempty"`
}

// EvaluationWarning represents a non-blocking issue.
type EvaluationWarning struct {
	// Code is a unique warning identifier.
	Code string `json:"code"`

	// Message is a human-readable description.
	Message string `json:"message"`

	// Category classifies the warning type.
	Category string `json:"category"`

	// Location points to the source of the warning.
	Location string `json:"location,omitempty"`
}

// EvaluationError represents a blocking issue.
type EvaluationError struct {
	// Code is a unique error identifier.
	Code string `json:"code"`

	// Message is a human-readable description.
	Message string `json:"message"`

	// Category classifies the error type.
	Category string `json:"category"`

	// Fatal indicates if this error prevents all further processing.
	Fatal bool `json:"fatal"`

	// Location points to the source of the error.
	Location string `json:"location,omitempty"`
}

// NewEvaluation creates a new governance evaluation.
func NewEvaluation(proposalID string) *GovernanceEvaluation {
	return &GovernanceEvaluation{
		CGPVersion:    Version,
		Type:          MessageTypeEvaluation,
		ID:            GenerateEvaluationID(),
		ProposalID:    proposalID,
		Timestamp:     time.Now().UTC(),
		Status:        EvaluationStatusComplete,
		PolicyResults: []PolicyResult{},
		Warnings:      []EvaluationWarning{},
		Errors:        []EvaluationError{},
		Metadata:      make(map[string]any),
	}
}

// GenerateEvaluationID generates a unique evaluation ID.
func GenerateEvaluationID() string {
	return fmt.Sprintf("eval_%s", uuid.New().String()[:12])
}

// Validate checks if the evaluation is valid.
func (e *GovernanceEvaluation) Validate() error {
	if e.CGPVersion == "" {
		return fmt.Errorf("CGP version is required")
	}
	if e.Type != MessageTypeEvaluation {
		return fmt.Errorf("invalid message type for evaluation: %s", e.Type)
	}
	if e.ID == "" {
		return fmt.Errorf("evaluation ID is required")
	}
	if e.ProposalID == "" {
		return fmt.Errorf("proposal ID is required")
	}
	if !e.Status.IsValid() {
		return fmt.Errorf("invalid evaluation status: %s", e.Status)
	}
	return nil
}

// WithRiskAssessment sets the risk assessment.
func (e *GovernanceEvaluation) WithRiskAssessment(assessment *RiskAssessment) *GovernanceEvaluation {
	e.RiskAssessment = assessment
	return e
}

// WithVersionAnalysis sets the version analysis.
func (e *GovernanceEvaluation) WithVersionAnalysis(analysis *VersionAnalysis) *GovernanceEvaluation {
	e.VersionAnalysis = analysis
	return e
}

// WithChangeAnalysis sets the change analysis.
func (e *GovernanceEvaluation) WithChangeAnalysis(analysis *ChangeAnalysis) *GovernanceEvaluation {
	e.ChangeAnalysis = analysis
	return e
}

// AddPolicyResult adds a policy evaluation result.
func (e *GovernanceEvaluation) AddPolicyResult(result PolicyResult) *GovernanceEvaluation {
	e.PolicyResults = append(e.PolicyResults, result)
	return e
}

// AddWarning adds a warning to the evaluation.
func (e *GovernanceEvaluation) AddWarning(code, message, category string) *GovernanceEvaluation {
	e.Warnings = append(e.Warnings, EvaluationWarning{
		Code:     code,
		Message:  message,
		Category: category,
	})
	return e
}

// AddError adds an error to the evaluation.
func (e *GovernanceEvaluation) AddError(code, message, category string, fatal bool) *GovernanceEvaluation {
	e.Errors = append(e.Errors, EvaluationError{
		Code:     code,
		Message:  message,
		Category: category,
		Fatal:    fatal,
	})
	return e
}

// WithDuration sets the evaluation duration.
func (e *GovernanceEvaluation) WithDuration(d time.Duration) *GovernanceEvaluation {
	e.Duration = d
	return e
}

// WithMetadata adds metadata.
func (e *GovernanceEvaluation) WithMetadata(key string, value any) *GovernanceEvaluation {
	e.Metadata[key] = value
	return e
}

// HasErrors returns true if there are any errors.
func (e *GovernanceEvaluation) HasErrors() bool {
	return len(e.Errors) > 0
}

// HasFatalErrors returns true if there are any fatal errors.
func (e *GovernanceEvaluation) HasFatalErrors() bool {
	for _, err := range e.Errors {
		if err.Fatal {
			return true
		}
	}
	return false
}

// HasWarnings returns true if there are any warnings.
func (e *GovernanceEvaluation) HasWarnings() bool {
	return len(e.Warnings) > 0
}

// IsComplete returns true if the evaluation completed successfully.
func (e *GovernanceEvaluation) IsComplete() bool {
	return e.Status == EvaluationStatusComplete
}

// CanProceed returns true if the evaluation allows proceeding to decision.
func (e *GovernanceEvaluation) CanProceed() bool {
	return e.IsComplete() && !e.HasFatalErrors()
}

// SuggestedDecision returns a suggested decision based on the evaluation.
func (e *GovernanceEvaluation) SuggestedDecision() DecisionType {
	if e.HasFatalErrors() {
		return DecisionRejected
	}

	if e.RiskAssessment == nil {
		return DecisionDeferred
	}

	// High risk requires approval
	if e.RiskAssessment.OverallScore >= 0.7 {
		return DecisionApprovalRequired
	}

	// Check if any policy requires approval
	for _, pr := range e.PolicyResults {
		if pr.Matched && pr.Decision == DecisionApprovalRequired {
			return DecisionApprovalRequired
		}
		if pr.Matched && pr.Decision == DecisionRejected {
			return DecisionRejected
		}
	}

	// Low risk can be auto-approved
	if e.RiskAssessment.OverallScore < 0.4 {
		return DecisionApproved
	}

	// Medium risk requires approval
	return DecisionApprovalRequired
}
