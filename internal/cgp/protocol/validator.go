package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
)

// ValidationError represents a message validation failure.
type ValidationError struct {
	// Field is the JSON path to the invalid field.
	Field string `json:"field"`

	// Message describes the validation failure.
	Message string `json:"message"`

	// Value is the actual value that failed validation.
	Value any `json:"value,omitempty"`
}

func (e ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

// ValidationResult contains the outcome of message validation.
type ValidationResult struct {
	// Valid is true if the message passed validation.
	Valid bool `json:"valid"`

	// Errors are the validation errors found.
	Errors []ValidationError `json:"errors,omitempty"`

	// Warnings are non-critical validation issues.
	Warnings []ValidationError `json:"warnings,omitempty"`
}

// Error returns the first error, or nil if valid.
func (r *ValidationResult) Error() error {
	if len(r.Errors) == 0 {
		return nil
	}
	return r.Errors[0]
}

// ErrorMessages returns all error messages joined.
func (r *ValidationResult) ErrorMessages() string {
	if len(r.Errors) == 0 {
		return ""
	}
	msgs := make([]string, len(r.Errors))
	for i, e := range r.Errors {
		msgs[i] = e.Error()
	}
	return strings.Join(msgs, "; ")
}

// Validator validates CGP messages.
type Validator struct {
	// StrictMode enables additional validation checks.
	StrictMode bool

	// RequireCorrelationID requires messages to have correlation IDs.
	RequireCorrelationID bool
}

// NewValidator creates a new validator with default settings.
func NewValidator() *Validator {
	return &Validator{
		StrictMode:           false,
		RequireCorrelationID: false,
	}
}

// NewStrictValidator creates a validator with strict mode enabled.
func NewStrictValidator() *Validator {
	return &Validator{
		StrictMode:           true,
		RequireCorrelationID: true,
	}
}

// Validate validates a message.
func (v *Validator) Validate(msg *Message) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Validate header
	v.validateHeader(&msg.Header, result)

	// Validate payload based on type
	switch msg.Header.Type {
	case cgp.MessageTypeProposal:
		v.validateProposalPayload(msg.Payload, result)
	case cgp.MessageTypeEvaluation:
		v.validateEvaluationPayload(msg.Payload, result)
	case cgp.MessageTypeDecision:
		v.validateDecisionPayload(msg.Payload, result)
	case cgp.MessageTypeAuthorization:
		v.validateAuthorizationPayload(msg.Payload, result)
	default:
		result.addError("header.type", "unknown message type", msg.Header.Type)
	}

	result.Valid = len(result.Errors) == 0
	return result
}

// ValidateJSON validates a raw JSON message.
func (v *Validator) ValidateJSON(data []byte) *ValidationResult {
	msg, err := DecodeMessage(data)
	if err != nil {
		return &ValidationResult{
			Valid:  false,
			Errors: []ValidationError{{Message: fmt.Sprintf("invalid JSON: %v", err)}},
		}
	}
	return v.Validate(msg)
}

// ValidateProposal validates a proposal directly.
func (v *Validator) ValidateProposal(p *cgp.ChangeProposal) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if err := p.Validate(); err != nil {
		result.addError("", err.Error(), nil)
	}

	v.validateProposalFields(p, result)

	result.Valid = len(result.Errors) == 0
	return result
}

// ValidateEvaluation validates an evaluation directly.
func (v *Validator) ValidateEvaluation(e *cgp.GovernanceEvaluation) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if err := e.Validate(); err != nil {
		result.addError("", err.Error(), nil)
	}

	v.validateEvaluationFields(e, result)

	result.Valid = len(result.Errors) == 0
	return result
}

// ValidateDecision validates a decision directly.
func (v *Validator) ValidateDecision(d *cgp.GovernanceDecision) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if err := d.Validate(); err != nil {
		result.addError("", err.Error(), nil)
	}

	v.validateDecisionFields(d, result)

	result.Valid = len(result.Errors) == 0
	return result
}

// ValidateAuthorization validates an authorization directly.
func (v *Validator) ValidateAuthorization(a *cgp.ExecutionAuthorization) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if err := a.Validate(); err != nil {
		result.addError("", err.Error(), nil)
	}

	v.validateAuthorizationFields(a, result)

	result.Valid = len(result.Errors) == 0
	return result
}

func (v *Validator) validateHeader(h *Header, result *ValidationResult) {
	if h.MessageID == "" {
		result.addError("header.messageId", "message ID is required", nil)
	}

	if !h.Type.IsValid() {
		result.addError("header.type", "invalid message type", h.Type)
	}

	if h.CGPVersion == "" {
		result.addError("header.cgpVersion", "CGP version is required", nil)
	}

	if h.Timestamp.IsZero() {
		result.addError("header.timestamp", "timestamp is required", nil)
	}

	if v.RequireCorrelationID && h.CorrelationID == "" {
		result.addError("header.correlationId", "correlation ID is required", nil)
	}
}

func (v *Validator) validateProposalPayload(payload json.RawMessage, result *ValidationResult) {
	var p cgp.ChangeProposal
	if err := json.Unmarshal(payload, &p); err != nil {
		result.addError("payload", fmt.Sprintf("failed to parse proposal: %v", err), nil)
		return
	}

	if err := p.Validate(); err != nil {
		result.addError("payload", err.Error(), nil)
	}

	v.validateProposalFields(&p, result)
}

func (v *Validator) validateProposalFields(p *cgp.ChangeProposal, result *ValidationResult) {
	if v.StrictMode {
		// In strict mode, require additional fields
		if p.Intent.Confidence == 0 {
			result.addWarning("payload.intent.confidence", "confidence is zero", nil)
		}

		if len(p.Intent.Categories) == 0 {
			result.addWarning("payload.intent.categories", "no categories specified", nil)
		}
	}
}

func (v *Validator) validateEvaluationPayload(payload json.RawMessage, result *ValidationResult) {
	var e cgp.GovernanceEvaluation
	if err := json.Unmarshal(payload, &e); err != nil {
		result.addError("payload", fmt.Sprintf("failed to parse evaluation: %v", err), nil)
		return
	}

	if err := e.Validate(); err != nil {
		result.addError("payload", err.Error(), nil)
	}

	v.validateEvaluationFields(&e, result)
}

func (v *Validator) validateEvaluationFields(e *cgp.GovernanceEvaluation, result *ValidationResult) {
	if v.StrictMode {
		// In strict mode, require risk assessment
		if e.RiskAssessment == nil {
			result.addError("payload.riskAssessment", "risk assessment is required", nil)
		} else {
			if e.RiskAssessment.OverallScore < 0 || e.RiskAssessment.OverallScore > 1 {
				result.addError("payload.riskAssessment.overallScore", "score must be between 0 and 1", e.RiskAssessment.OverallScore)
			}
		}
	}
}

func (v *Validator) validateDecisionPayload(payload json.RawMessage, result *ValidationResult) {
	var d cgp.GovernanceDecision
	if err := json.Unmarshal(payload, &d); err != nil {
		result.addError("payload", fmt.Sprintf("failed to parse decision: %v", err), nil)
		return
	}

	if err := d.Validate(); err != nil {
		result.addError("payload", err.Error(), nil)
	}

	v.validateDecisionFields(&d, result)
}

func (v *Validator) validateDecisionFields(d *cgp.GovernanceDecision, result *ValidationResult) {
	if v.StrictMode {
		// In strict mode, require rationale
		if len(d.Rationale) == 0 {
			result.addWarning("payload.rationale", "no rationale provided", nil)
		}

		// Check risk factor scores
		for i, rf := range d.RiskFactors {
			if rf.Score < 0 || rf.Score > 1 {
				result.addError(
					fmt.Sprintf("payload.riskFactors[%d].score", i),
					"score must be between 0 and 1",
					rf.Score,
				)
			}
		}
	}
}

func (v *Validator) validateAuthorizationPayload(payload json.RawMessage, result *ValidationResult) {
	var a cgp.ExecutionAuthorization
	if err := json.Unmarshal(payload, &a); err != nil {
		result.addError("payload", fmt.Sprintf("failed to parse authorization: %v", err), nil)
		return
	}

	if err := a.Validate(); err != nil {
		result.addError("payload", err.Error(), nil)
	}

	v.validateAuthorizationFields(&a, result)
}

func (v *Validator) validateAuthorizationFields(a *cgp.ExecutionAuthorization, result *ValidationResult) {
	if v.StrictMode {
		// In strict mode, ensure authorization hasn't expired
		if a.IsExpired() {
			result.addError("payload", "authorization has expired", nil)
		}

		// Ensure there are allowed steps
		if len(a.AllowedSteps) == 0 {
			result.addWarning("payload.allowedSteps", "no allowed steps specified", nil)
		}
	}
}

func (r *ValidationResult) addError(field, message string, value any) {
	r.Errors = append(r.Errors, ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

func (r *ValidationResult) addWarning(field, message string, value any) {
	r.Warnings = append(r.Warnings, ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

// ValidateAny validates any CGP type.
func ValidateAny(v any) error {
	validator := NewValidator()

	switch t := v.(type) {
	case *cgp.ChangeProposal:
		result := validator.ValidateProposal(t)
		if !result.Valid {
			return errors.New(result.ErrorMessages())
		}
	case *cgp.GovernanceEvaluation:
		result := validator.ValidateEvaluation(t)
		if !result.Valid {
			return errors.New(result.ErrorMessages())
		}
	case *cgp.GovernanceDecision:
		result := validator.ValidateDecision(t)
		if !result.Valid {
			return errors.New(result.ErrorMessages())
		}
	case *cgp.ExecutionAuthorization:
		result := validator.ValidateAuthorization(t)
		if !result.Valid {
			return errors.New(result.ErrorMessages())
		}
	case *Message:
		result := validator.Validate(t)
		if !result.Valid {
			return errors.New(result.ErrorMessages())
		}
	default:
		return fmt.Errorf("unsupported type for validation: %T", v)
	}

	return nil
}
