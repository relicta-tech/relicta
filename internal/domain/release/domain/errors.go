// Package domain provides the core domain model for release governance.
package domain

import (
	"errors"
	"fmt"
)

// Domain errors for release run operations.
var (
	// ErrInvalidState indicates an invalid state for the requested operation.
	ErrInvalidState = errors.New("invalid state for this operation")

	// ErrHeadSHAChanged indicates the repository HEAD has changed since planning.
	ErrHeadSHAChanged = errors.New("repository HEAD has changed since planning")

	// ErrAlreadyPublished indicates the release is already published.
	ErrAlreadyPublished = errors.New("release is already published")

	// ErrNotApproved indicates the release is not approved.
	ErrNotApproved = errors.New("release is not approved")

	// ErrPlanHashMismatch indicates the plan hash does not match.
	ErrPlanHashMismatch = errors.New("plan hash does not match approved plan")

	// ErrStepAlreadyDone indicates a step is already completed.
	ErrStepAlreadyDone = errors.New("step is already completed")

	// ErrStepNotFound indicates a step was not found.
	ErrStepNotFound = errors.New("step not found in execution plan")

	// ErrRunNotFound indicates a release run was not found.
	ErrRunNotFound = errors.New("release run not found")

	// ErrApprovalBoundToHash indicates approval is bound to a different plan hash.
	ErrApprovalBoundToHash = errors.New("approval is bound to a different plan hash")

	// ErrNilNotes indicates nil release notes were provided.
	ErrNilNotes = errors.New("release notes cannot be nil")

	// ErrNoChanges indicates there are no changes to release.
	ErrNoChanges = errors.New("no changes to release")

	// ErrCannotCancel indicates the release cannot be canceled.
	ErrCannotCancel = errors.New("release cannot be canceled in current state")

	// ErrCannotRetry indicates the release cannot be retried.
	ErrCannotRetry = errors.New("release cannot be retried in current state")

	// ErrVersionNotSet indicates the version has not been set.
	ErrVersionNotSet = errors.New("version must be set before this operation")

	// ErrRiskTooHigh indicates the risk score is too high for the operation.
	ErrRiskTooHigh = errors.New("risk score exceeds threshold")

	// ErrDuplicateRun indicates a run with the same plan hash already exists.
	ErrDuplicateRun = errors.New("a release run with this plan already exists")
)

// StateTransitionError provides a detailed error message for invalid state transitions.
// It includes actionable guidance to help users understand what to do next.
type StateTransitionError struct {
	CurrentState RunState
	TargetState  RunState
	Action       string
}

// Error implements the error interface.
func (e *StateTransitionError) Error() string {
	guidance := e.getGuidance()
	if guidance != "" {
		return fmt.Sprintf("cannot %s: release run is in '%s' state. %s",
			e.Action, e.CurrentState, guidance)
	}
	return fmt.Sprintf("cannot %s in state '%s'", e.Action, e.CurrentState)
}

// Unwrap returns the underlying error for errors.Is compatibility.
func (e *StateTransitionError) Unwrap() error {
	return ErrInvalidState
}

// getGuidance returns actionable guidance based on the current state and desired action.
func (e *StateTransitionError) getGuidance() string {
	switch e.Action {
	case "set version", "bump version", "bump":
		switch e.CurrentState {
		case StateDraft:
			return "Run 'relicta plan' first to analyze changes."
		case StateVersioned, StateNotesReady, StateApproved, StatePublished:
			return "Version is already set. Use 'relicta release' for a new release."
		case StateFailed:
			return "Release failed. Use 'relicta retry' or start a new release."
		case StateCanceled:
			return "Release was canceled. Start a new release."
		}
	case "generate notes", "set notes":
		switch e.CurrentState {
		case StateDraft:
			return "Run 'relicta plan' then 'relicta bump' first."
		case StatePlanned:
			return "Run 'relicta bump' first to set the version."
		case StateNotesReady, StateApproved, StatePublished:
			return "Notes are already generated. Use 'relicta release' for a new release."
		case StateFailed:
			return "Release failed. Use 'relicta retry' or start a new release."
		}
	case "approve":
		switch e.CurrentState {
		case StateDraft:
			return "Run 'relicta plan', 'relicta bump', and 'relicta notes' first."
		case StatePlanned:
			return "Run 'relicta bump' and 'relicta notes' first."
		case StateVersioned:
			return "Run 'relicta notes' first to generate release notes."
		case StateApproved:
			return "Release is already approved. Ready to publish."
		case StatePublished:
			return "Release is already published."
		case StatePublishing:
			return "Release is currently being published."
		case StateFailed:
			return "Release failed. Use 'relicta retry' or start a new release."
		}
	case "publish", "start publishing":
		switch e.CurrentState {
		case StateDraft:
			return "Complete the workflow: plan -> bump -> notes -> approve."
		case StatePlanned:
			return "Run 'relicta bump', 'relicta notes', and 'relicta approve' first."
		case StateVersioned:
			return "Run 'relicta notes' and 'relicta approve' first."
		case StateNotesReady:
			return "Run 'relicta approve' first to approve the release."
		case StatePublishing:
			return "Release is already being published."
		case StatePublished:
			return "Release is already published."
		case StateFailed:
			return "Release failed. Use 'relicta retry' to resume publishing."
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
			return "Cannot cancel during publishing. Wait for completion or let it fail."
		case StateCanceled:
			return "Release is already canceled."
		}
	case "update notes":
		if e.CurrentState != StateNotesReady {
			return "Notes can only be updated in 'notes_ready' state before approval."
		}
	case "plan":
		switch e.CurrentState {
		case StateVersioned, StateNotesReady, StateApproved, StatePublishing, StatePublished:
			return "Release has progressed past planning. Use 'relicta release' for a new release."
		case StateFailed:
			return "Release failed. Use 'relicta retry' or start a new release."
		}
	}
	return ""
}

// NewStateTransitionError creates a new StateTransitionError.
func NewStateTransitionError(currentState RunState, action string) *StateTransitionError {
	return &StateTransitionError{
		CurrentState: currentState,
		Action:       action,
	}
}

// HeadMismatchError provides detailed information about HEAD SHA mismatches.
type HeadMismatchError struct {
	ExpectedSHA CommitSHA
	ActualSHA   CommitSHA
}

// Error implements the error interface.
func (e *HeadMismatchError) Error() string {
	return fmt.Sprintf("repository HEAD has changed: expected %s, got %s. "+
		"Use --force to override or start a new release.",
		e.ExpectedSHA.Short(), e.ActualSHA.Short())
}

// Unwrap returns the underlying error.
func (e *HeadMismatchError) Unwrap() error {
	return ErrHeadSHAChanged
}

// NewHeadMismatchError creates a new HeadMismatchError.
func NewHeadMismatchError(expected, actual CommitSHA) *HeadMismatchError {
	return &HeadMismatchError{
		ExpectedSHA: expected,
		ActualSHA:   actual,
	}
}

// StepError provides detailed information about step failures.
type StepError struct {
	StepName  string
	StepType  StepType
	Attempts  int
	LastError string
}

// Error implements the error interface.
func (e *StepError) Error() string {
	return fmt.Sprintf("step '%s' (%s) failed after %d attempt(s): %s",
		e.StepName, e.StepType, e.Attempts, e.LastError)
}

// NewStepError creates a new StepError.
func NewStepError(stepName string, stepType StepType, attempts int, lastError string) *StepError {
	return &StepError{
		StepName:  stepName,
		StepType:  stepType,
		Attempts:  attempts,
		LastError: lastError,
	}
}

// RiskThresholdError indicates a risk score exceeds the allowed threshold.
type RiskThresholdError struct {
	RiskScore float64
	Threshold float64
	Reasons   []string
}

// Error implements the error interface.
func (e *RiskThresholdError) Error() string {
	return fmt.Sprintf("release blocked: risk score %.2f exceeds threshold %.2f",
		e.RiskScore, e.Threshold)
}

// Unwrap returns the underlying error.
func (e *RiskThresholdError) Unwrap() error {
	return ErrRiskTooHigh
}

// NewRiskThresholdError creates a new RiskThresholdError.
func NewRiskThresholdError(score, threshold float64, reasons []string) *RiskThresholdError {
	return &RiskThresholdError{
		RiskScore: score,
		Threshold: threshold,
		Reasons:   reasons,
	}
}
