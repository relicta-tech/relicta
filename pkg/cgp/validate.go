package cgp

import (
	"fmt"
	"strings"
)

// ValidationError collects one or more validation failures.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return strings.Join(e.Errors, "; ")
}

// add appends a validation failure message.
func (e *ValidationError) add(msg string) {
	e.Errors = append(e.Errors, msg)
}

// addf appends a formatted validation failure message.
func (e *ValidationError) addf(format string, args ...any) {
	e.Errors = append(e.Errors, fmt.Sprintf(format, args...))
}

// hasErrors returns true if at least one error was recorded.
func (e *ValidationError) hasErrors() bool {
	return len(e.Errors) > 0
}

// result returns the ValidationError if any errors were recorded, nil otherwise.
func (e *ValidationError) result() error {
	if e.hasErrors() {
		return e
	}
	return nil
}

// ValidateProposal checks that a ChangeProposal contains all required fields.
func ValidateProposal(p *ChangeProposal) error {
	if p == nil {
		return fmt.Errorf("proposal is nil")
	}
	ve := &ValidationError{}

	if p.CGPVersion == "" {
		ve.add("cgpVersion is required")
	}
	if p.Type != TypeChangeProposal {
		ve.addf("type must be %q, got %q", TypeChangeProposal, p.Type)
	}
	if p.ID == "" {
		ve.add("id is required")
	}
	if p.Timestamp.IsZero() {
		ve.add("timestamp is required")
	}

	// Actor validation
	if p.Actor.Kind == "" {
		ve.add("actor.kind is required")
	}
	if p.Actor.ID == "" {
		ve.add("actor.id is required")
	}

	// Scope validation
	if p.Scope.Repository == "" {
		ve.add("scope.repository is required")
	}
	if p.Scope.CommitRange == "" && len(p.Scope.Commits) == 0 {
		ve.add("scope requires commitRange or commits")
	}

	// Intent validation
	if p.Intent.Summary == "" {
		ve.add("intent.summary is required")
	}
	if p.Intent.Confidence < 0 || p.Intent.Confidence > 1 {
		ve.addf("intent.confidence must be between 0.0 and 1.0, got %f", p.Intent.Confidence)
	}

	return ve.result()
}

// ValidateEvaluation checks that a GovernanceEvaluation contains all required fields.
func ValidateEvaluation(e *GovernanceEvaluation) error {
	if e == nil {
		return fmt.Errorf("evaluation is nil")
	}
	ve := &ValidationError{}

	if e.CGPVersion == "" {
		ve.add("cgpVersion is required")
	}
	if e.Type != TypeGovernanceEvaluation {
		ve.addf("type must be %q, got %q", TypeGovernanceEvaluation, e.Type)
	}
	if e.ID == "" {
		ve.add("id is required")
	}
	if e.ProposalID == "" {
		ve.add("proposalId is required")
	}
	if e.Timestamp.IsZero() {
		ve.add("timestamp is required")
	}
	if e.RiskScore < 0 || e.RiskScore > 1 {
		ve.addf("riskScore must be between 0.0 and 1.0, got %f", e.RiskScore)
	}

	return ve.result()
}

// ValidateDecision checks that a GovernanceDecision contains all required fields.
func ValidateDecision(d *GovernanceDecision) error {
	if d == nil {
		return fmt.Errorf("decision is nil")
	}
	ve := &ValidationError{}

	if d.CGPVersion == "" {
		ve.add("cgpVersion is required")
	}
	if d.Type != TypeGovernanceDecision {
		ve.addf("type must be %q, got %q", TypeGovernanceDecision, d.Type)
	}
	if d.ID == "" {
		ve.add("id is required")
	}
	if d.ProposalID == "" {
		ve.add("proposalId is required")
	}
	if d.Timestamp.IsZero() {
		ve.add("timestamp is required")
	}

	validDecisions := map[string]bool{
		"approved":          true,
		"denied":            true,
		"approval_required": true,
	}
	if !validDecisions[d.Decision] {
		ve.addf("decision must be one of approved, denied, approval_required; got %q", d.Decision)
	}

	if d.RiskScore < 0 || d.RiskScore > 1 {
		ve.addf("riskScore must be between 0.0 and 1.0, got %f", d.RiskScore)
	}

	return ve.result()
}

// ValidateAuthorization checks that an ExecutionAuthorization contains all required fields.
func ValidateAuthorization(a *ExecutionAuthorization) error {
	if a == nil {
		return fmt.Errorf("authorization is nil")
	}
	ve := &ValidationError{}

	if a.CGPVersion == "" {
		ve.add("cgpVersion is required")
	}
	if a.Type != TypeExecutionAuthorization {
		ve.addf("type must be %q, got %q", TypeExecutionAuthorization, a.Type)
	}
	if a.ID == "" {
		ve.add("id is required")
	}
	if a.ProposalID == "" {
		ve.add("proposalId is required")
	}
	if a.DecisionID == "" {
		ve.add("decisionId is required")
	}
	if a.Timestamp.IsZero() {
		ve.add("timestamp is required")
	}

	// ApprovedBy validation
	if a.ApprovedBy.Kind == "" {
		ve.add("approvedBy.kind is required")
	}
	if a.ApprovedBy.ID == "" {
		ve.add("approvedBy.id is required")
	}

	if a.Version == "" {
		ve.add("version is required")
	}
	if a.ValidUntil.IsZero() {
		ve.add("validUntil is required")
	}

	return ve.result()
}
