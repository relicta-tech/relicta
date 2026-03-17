// Package communication provides domain types for release communication.
package communication

import "errors"

// Domain errors for communication operations.
var (
	// ErrInvalidFormat indicates an invalid changelog format.
	ErrInvalidFormat = errors.New("invalid changelog format")

	// ErrInvalidTone indicates an invalid note tone.
	ErrInvalidTone = errors.New("invalid note tone")

	// ErrInvalidAudience indicates an invalid note audience.
	ErrInvalidAudience = errors.New("invalid note audience")

	// ErrEmptyChangeset indicates an empty changeset was provided.
	ErrEmptyChangeset = errors.New("changeset is empty")

	// ErrGenerationFailed indicates generation failed.
	ErrGenerationFailed = errors.New("generation failed")
)
