// Package app provides application services (use cases) for release governance.
package app

import (
	"errors"
	"fmt"
)

// ValidationError represents an input validation error in the application layer.
// It provides structured error information for programmatic handling.
type ValidationError struct {
	Field   string // The field that failed validation
	Message string // Human-readable error message
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s - %s", e.Field, e.Message)
}

// NewValidationError creates a new ValidationError.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// IsValidationError checks if an error is a ValidationError.
// Uses errors.As to properly handle wrapped errors.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

// Common validation errors for reuse.
var (
	// ErrActorIDRequired is returned when Actor.ID is empty.
	ErrActorIDRequired = NewValidationError("Actor.ID", "actor ID is required for audit trail")

	// ErrTagPushMissingVersion is returned when TagPushMode is true but NextVersion is nil.
	ErrTagPushMissingVersion = NewValidationError("NextVersion", "tag-push mode requires NextVersion to be set")
)
