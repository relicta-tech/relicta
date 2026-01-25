package protocol

import (
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
)

func TestValidator_ValidateProposal(t *testing.T) {
	v := NewValidator()

	proposal := cgp.NewProposal(
		cgp.NewHumanActor("test@example.com", "Test User"),
		cgp.ProposalScope{
			Repository:  "org/repo",
			CommitRange: "abc..def",
		},
		cgp.ProposalIntent{
			Summary:    "Test change",
			Confidence: 0.8,
		},
	)

	result := v.ValidateProposal(proposal)
	if !result.Valid {
		t.Errorf("Valid proposal should pass validation: %s", result.ErrorMessages())
	}
}

func TestValidator_ValidateProposal_Invalid(t *testing.T) {
	v := NewValidator()

	proposal := &cgp.ChangeProposal{
		// Missing required fields
	}

	result := v.ValidateProposal(proposal)
	if result.Valid {
		t.Error("Invalid proposal should fail validation")
	}
}

func TestValidator_ValidateDecision(t *testing.T) {
	v := NewValidator()

	decision := cgp.NewDecision("proposal-123", cgp.DecisionApproved).
		WithRiskScore(0.3).
		AddRationale("Low risk change")

	result := v.ValidateDecision(decision)
	if !result.Valid {
		t.Errorf("Valid decision should pass validation: %s", result.ErrorMessages())
	}
}

func TestValidator_ValidateEvaluation(t *testing.T) {
	v := NewValidator()

	eval := cgp.NewEvaluation("proposal-123").
		WithRiskAssessment(&cgp.RiskAssessment{
			OverallScore: 0.4,
			Severity:     cgp.SeverityMedium,
		})

	result := v.ValidateEvaluation(eval)
	if !result.Valid {
		t.Errorf("Valid evaluation should pass validation: %s", result.ErrorMessages())
	}
}

func TestValidator_ValidateAuthorization(t *testing.T) {
	v := NewValidator()

	auth := cgp.NewAuthorization(
		"decision-123",
		"proposal-123",
		cgp.NewHumanActor("approver@example.com", "Approver"),
		"1.0.0",
	)

	result := v.ValidateAuthorization(auth)
	if !result.Valid {
		t.Errorf("Valid authorization should pass validation: %s", result.ErrorMessages())
	}
}

func TestValidator_ValidateMessage(t *testing.T) {
	v := NewValidator()

	proposal := cgp.NewProposal(
		cgp.NewHumanActor("test@example.com", "Test"),
		cgp.ProposalScope{Repository: "org/repo", CommitRange: "a..b"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.8},
	)

	msg, _ := NewProposalMessage(proposal)
	result := v.Validate(msg)
	if !result.Valid {
		t.Errorf("Valid message should pass validation: %s", result.ErrorMessages())
	}
}

func TestValidator_ValidateMessage_InvalidHeader(t *testing.T) {
	v := NewValidator()

	msg := &Message{
		Header: Header{
			MessageID:  "", // Missing
			Type:       cgp.MessageTypeProposal,
			CGPVersion: cgp.Version,
			Timestamp:  time.Now(),
		},
		Payload: []byte("{}"),
	}

	result := v.Validate(msg)
	if result.Valid {
		t.Error("Message with invalid header should fail")
	}

	found := false
	for _, e := range result.Errors {
		if e.Field == "header.messageId" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Should report messageId error")
	}
}

func TestValidator_ValidateMessage_UnknownType(t *testing.T) {
	v := NewValidator()

	msg := &Message{
		Header: Header{
			MessageID:  "msg-123",
			Type:       cgp.MessageType("unknown"),
			CGPVersion: cgp.Version,
			Timestamp:  time.Now(),
		},
		Payload: []byte("{}"),
	}

	result := v.Validate(msg)
	if result.Valid {
		t.Error("Message with unknown type should fail")
	}
}

func TestValidator_StrictMode(t *testing.T) {
	v := NewStrictValidator()

	// Decision without rationale should warn in strict mode
	decision := cgp.NewDecision("proposal-123", cgp.DecisionApproved)
	decision.Rationale = []string{} // Clear rationale

	result := v.ValidateDecision(decision)
	// Should have warnings
	if len(result.Warnings) == 0 {
		t.Error("Strict mode should warn about missing rationale")
	}
}

func TestValidator_StrictMode_RiskScore(t *testing.T) {
	v := NewStrictValidator()

	decision := cgp.NewDecision("proposal-123", cgp.DecisionApproved)
	decision.RiskFactors = []cgp.RiskFactor{
		{Category: "test", Score: 1.5, Severity: cgp.SeverityLow}, // Invalid score > 1
	}

	result := v.ValidateDecision(decision)
	if result.Valid {
		t.Error("Strict mode should reject invalid risk score")
	}
}

func TestValidator_StrictMode_ExpiredAuth(t *testing.T) {
	v := NewStrictValidator()

	auth := cgp.NewAuthorization(
		"decision-123",
		"proposal-123",
		cgp.NewHumanActor("approver@example.com", "Approver"),
		"1.0.0",
	)
	// Set to expired
	auth.ValidUntil = time.Now().Add(-time.Hour)

	result := v.ValidateAuthorization(auth)
	if result.Valid {
		t.Error("Strict mode should reject expired authorization")
	}
}

func TestValidator_StrictMode_RequireCorrelationID(t *testing.T) {
	v := NewStrictValidator()

	msg := &Message{
		Header: Header{
			MessageID:     "msg-123",
			Type:          cgp.MessageTypeProposal,
			CGPVersion:    cgp.Version,
			Timestamp:     time.Now(),
			CorrelationID: "", // Missing
		},
		Payload: []byte(`{"cgpVersion":"0.1","type":"change.proposal","id":"prop_123","timestamp":"2024-01-01T00:00:00Z","actor":{"kind":"human","id":"human:test@example.com"},"scope":{"repository":"org/repo","commitRange":"a..b"},"intent":{"summary":"Test","confidence":0.8}}`),
	}

	result := v.Validate(msg)
	if result.Valid {
		t.Error("Strict mode should require correlation ID")
	}
}

func TestValidator_ValidateJSON(t *testing.T) {
	v := NewValidator()

	proposal := cgp.NewProposal(
		cgp.NewHumanActor("test@example.com", "Test"),
		cgp.ProposalScope{Repository: "org/repo", CommitRange: "a..b"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.8},
	)
	msg, _ := NewProposalMessage(proposal)
	data, _ := msg.Encode()

	result := v.ValidateJSON(data)
	if !result.Valid {
		t.Errorf("Valid JSON should pass: %s", result.ErrorMessages())
	}
}

func TestValidator_ValidateJSON_Invalid(t *testing.T) {
	v := NewValidator()

	result := v.ValidateJSON([]byte("not json"))
	if result.Valid {
		t.Error("Invalid JSON should fail")
	}
}

func TestValidationResult_ErrorMessages(t *testing.T) {
	result := &ValidationResult{
		Valid: false,
		Errors: []ValidationError{
			{Field: "field1", Message: "error 1"},
			{Field: "field2", Message: "error 2"},
		},
	}

	msg := result.ErrorMessages()
	if msg == "" {
		t.Error("ErrorMessages should not be empty")
	}
	if result.Error() == nil {
		t.Error("Error() should not be nil when there are errors")
	}
}

func TestValidationResult_NoErrors(t *testing.T) {
	result := &ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
	}

	if result.ErrorMessages() != "" {
		t.Error("ErrorMessages should be empty when valid")
	}
	if result.Error() != nil {
		t.Error("Error() should be nil when valid")
	}
}

func TestValidateAny(t *testing.T) {
	// Proposal
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("test@example.com", "Test"),
		cgp.ProposalScope{Repository: "org/repo", CommitRange: "a..b"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.8},
	)
	if err := ValidateAny(proposal); err != nil {
		t.Errorf("ValidateAny(proposal) failed: %v", err)
	}

	// Decision
	decision := cgp.NewDecision("proposal-123", cgp.DecisionApproved)
	if err := ValidateAny(decision); err != nil {
		t.Errorf("ValidateAny(decision) failed: %v", err)
	}

	// Evaluation
	eval := cgp.NewEvaluation("proposal-123")
	if err := ValidateAny(eval); err != nil {
		t.Errorf("ValidateAny(evaluation) failed: %v", err)
	}

	// Authorization
	auth := cgp.NewAuthorization("decision-123", "proposal-123", cgp.NewHumanActor("test@example.com", "Test"), "1.0.0")
	if err := ValidateAny(auth); err != nil {
		t.Errorf("ValidateAny(authorization) failed: %v", err)
	}

	// Message
	msg, _ := NewProposalMessage(proposal)
	if err := ValidateAny(msg); err != nil {
		t.Errorf("ValidateAny(message) failed: %v", err)
	}

	// Unsupported type
	err := ValidateAny("string")
	if err == nil {
		t.Error("ValidateAny should fail for unsupported type")
	}
}

func TestValidationError_Error(t *testing.T) {
	// With field
	e1 := ValidationError{Field: "header.type", Message: "is required"}
	if e1.Error() != "header.type: is required" {
		t.Errorf("Error() = %s, want 'header.type: is required'", e1.Error())
	}

	// Without field
	e2 := ValidationError{Message: "general error"}
	if e2.Error() != "general error" {
		t.Errorf("Error() = %s, want 'general error'", e2.Error())
	}
}
