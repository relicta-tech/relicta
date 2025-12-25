// Package release provides domain types for release management.
package release

import (
	"errors"
	"fmt"
)

// Domain errors for release operations.
var (
	// ErrReleaseNotFound indicates a release was not found.
	ErrReleaseNotFound = errors.New("release not found")

	// ErrInvalidStateTransition indicates an invalid state transition.
	ErrInvalidStateTransition = errors.New("invalid state transition")

	// ErrReleaseAlreadyExists indicates a release already exists.
	ErrReleaseAlreadyExists = errors.New("release already exists")

	// ErrNilPlan indicates a nil release plan was provided.
	ErrNilPlan = errors.New("release plan cannot be nil")

	// ErrNilNotes indicates nil release notes were provided.
	ErrNilNotes = errors.New("release notes cannot be nil")

	// ErrNotApproved indicates the release is not approved.
	ErrNotApproved = errors.New("release is not approved")

	// ErrAlreadyPublished indicates the release is already published.
	ErrAlreadyPublished = errors.New("release is already published")

	// ErrNoChanges indicates there are no changes to release.
	ErrNoChanges = errors.New("no changes to release")

	// ErrCannotCancel indicates the release cannot be canceled.
	ErrCannotCancel = errors.New("release cannot be canceled in current state")

	// ErrCannotRetry indicates the release cannot be retried.
	ErrCannotRetry = errors.New("release cannot be retried in current state")
)

// StateTransitionError provides a detailed error message for invalid state transitions.
type StateTransitionError struct {
	CurrentState ReleaseState
	TargetState  ReleaseState
	Action       string
}

// Error implements the error interface.
func (e *StateTransitionError) Error() string {
	guidance := e.getGuidance()
	if guidance != "" {
		return fmt.Sprintf("cannot %s: release is in '%s' state. %s",
			e.Action, e.CurrentState, guidance)
	}
	return fmt.Sprintf("cannot %s in state '%s'", e.Action, e.CurrentState)
}

// Unwrap returns the underlying error for errors.Is compatibility.
func (e *StateTransitionError) Unwrap() error {
	return ErrInvalidStateTransition
}

// getGuidance returns actionable guidance based on the current state and desired action.
func (e *StateTransitionError) getGuidance() string {
	switch e.Action {
	case "set version", "bump version":
		switch e.CurrentState {
		case StateInitialized:
			return "Run 'relicta plan' first to analyze changes."
		case StateNotesGenerated, StateApproved, StatePublished:
			return "Version is already set. Use 'relicta release' for a new release."
		}
	case "generate notes", "set notes":
		switch e.CurrentState {
		case StateInitialized:
			return "Run 'relicta plan' then 'relicta bump' first."
		case StatePlanned:
			return "Run 'relicta bump' first to set the version."
		case StateApproved, StatePublished:
			return "Notes are already generated. Use 'relicta release' for a new release."
		}
	case "approve":
		switch e.CurrentState {
		case StateInitialized:
			return "Run 'relicta plan', 'relicta bump', and 'relicta notes' first."
		case StatePlanned:
			return "Run 'relicta bump' and 'relicta notes' first."
		case StateVersioned:
			return "Run 'relicta notes' first to generate release notes."
		case StatePublished:
			return "Release is already published."
		}
	case "publish", "start publishing":
		switch e.CurrentState {
		case StateInitialized:
			return "Complete the workflow: plan → bump → notes → approve."
		case StatePlanned:
			return "Run 'relicta bump', 'relicta notes', and 'relicta approve' first."
		case StateVersioned:
			return "Run 'relicta notes' and 'relicta approve' first."
		case StateNotesGenerated:
			return "Run 'relicta approve' first to approve the release."
		case StatePublished:
			return "Release is already published."
		}
	case "retry":
		if e.CurrentState != StateFailed && e.CurrentState != StateCanceled {
			return "Only failed or canceled releases can be retried."
		}
	case "cancel":
		switch e.CurrentState {
		case StatePublished:
			return "Cannot cancel a published release."
		case StatePublishing:
			return "Cannot cancel during publishing. Wait for completion."
		}
	case "update notes":
		if e.CurrentState != StateNotesGenerated {
			return "Notes can only be updated in 'notes_generated' state before approval."
		}
	case "set plan":
		switch e.CurrentState {
		case StateVersioned, StateNotesGenerated, StateApproved, StatePublished:
			return "Release has progressed past planning. Use 'relicta release' for a new release."
		}
	}
	return ""
}

// NewStateTransitionError creates a new StateTransitionError.
func NewStateTransitionError(currentState ReleaseState, action string) *StateTransitionError {
	return &StateTransitionError{
		CurrentState: currentState,
		Action:       action,
	}
}
