// Package domain provides the core domain model for release governance.
package domain

import "fmt"

// RunState represents the current state of a release run.
type RunState string

const (
	// StateDraft is the initial state before planning is complete.
	StateDraft RunState = "draft"

	// StatePlanned means the release has been planned with a pinned head_sha.
	StatePlanned RunState = "planned"

	// StateNotesReady means release notes have been generated.
	StateNotesReady RunState = "notes_ready"

	// StateApproved means the release has been approved for publishing.
	StateApproved RunState = "approved"

	// StatePublishing means the release is currently being published.
	// This is a compound state with sub-states for each step.
	StatePublishing RunState = "publishing"

	// StatePublished is the terminal success state.
	StatePublished RunState = "published"

	// StateFailed indicates the release failed during publishing.
	StateFailed RunState = "failed"

	// StateCancelled indicates the release was cancelled.
	StateCancelled RunState = "cancelled"
)

// AllStates returns all valid run states.
func AllStates() []RunState {
	return []RunState{
		StateDraft,
		StatePlanned,
		StateNotesReady,
		StateApproved,
		StatePublishing,
		StatePublished,
		StateFailed,
		StateCancelled,
	}
}

// String returns the string representation of the state.
func (s RunState) String() string {
	return string(s)
}

// IsValid returns true if the state is a valid run state.
func (s RunState) IsValid() bool {
	switch s {
	case StateDraft, StatePlanned, StateNotesReady, StateApproved,
		StatePublishing, StatePublished, StateFailed, StateCancelled:
		return true
	default:
		return false
	}
}

// IsFinal returns true if this is a terminal state.
func (s RunState) IsFinal() bool {
	return s == StatePublished || s == StateFailed || s == StateCancelled
}

// IsActive returns true if the run is actively in progress.
func (s RunState) IsActive() bool {
	return !s.IsFinal() && s != StateDraft
}

// CanTransitionTo returns true if transitioning to the target state is valid.
func (s RunState) CanTransitionTo(target RunState) bool {
	transitions := validTransitions()
	validTargets, exists := transitions[s]
	if !exists {
		return false
	}

	for _, valid := range validTargets {
		if valid == target {
			return true
		}
	}
	return false
}

// validTransitions defines the state machine transitions.
func validTransitions() map[RunState][]RunState {
	return map[RunState][]RunState{
		StateDraft:      {StatePlanned, StateCancelled},
		StatePlanned:    {StateNotesReady, StateCancelled},
		StateNotesReady: {StateApproved, StatePlanned, StateCancelled}, // Can go back to Planned to regenerate notes
		StateApproved:   {StatePublishing, StateCancelled},
		StatePublishing: {StatePublished, StateFailed},
		StatePublished:  {},                           // Terminal - no transitions
		StateFailed:     {StatePublishing, StateDraft}, // Can retry or start over
		StateCancelled:  {StateDraft},                  // Can restart
	}
}

// NextValidStates returns the valid next states from the current state.
func (s RunState) NextValidStates() []RunState {
	transitions := validTransitions()
	if valid, exists := transitions[s]; exists {
		return valid
	}
	return nil
}

// ParseRunState parses a string into a RunState.
func ParseRunState(s string) (RunState, error) {
	state := RunState(s)
	if !state.IsValid() {
		return "", fmt.Errorf("invalid run state: %q", s)
	}
	return state, nil
}

// Description returns a human-readable description of the state.
func (s RunState) Description() string {
	switch s {
	case StateDraft:
		return "Release run created, awaiting planning"
	case StatePlanned:
		return "Release planned with pinned commit range and version"
	case StateNotesReady:
		return "Release notes generated and ready for review"
	case StateApproved:
		return "Release approved and ready to publish"
	case StatePublishing:
		return "Release is being published"
	case StatePublished:
		return "Release successfully published"
	case StateFailed:
		return "Release failed during publishing"
	case StateCancelled:
		return "Release was cancelled"
	default:
		return "Unknown state"
	}
}

// Icon returns a text icon for the state.
func (s RunState) Icon() string {
	switch s {
	case StateDraft:
		return "[DRAFT]"
	case StatePlanned:
		return "[PLANNED]"
	case StateNotesReady:
		return "[NOTES]"
	case StateApproved:
		return "[APPROVED]"
	case StatePublishing:
		return "[PUBLISHING]"
	case StatePublished:
		return "[PUBLISHED]"
	case StateFailed:
		return "[FAILED]"
	case StateCancelled:
		return "[CANCELLED]"
	default:
		return "[?]"
	}
}
