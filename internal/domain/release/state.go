// Package release provides domain types for release management.
package release

import (
	"fmt"
	"strings"
)

// ReleaseState represents the current state of a release process.
// This is a value object in DDD terms.
type ReleaseState string

const (
	// StateInitialized indicates the release has been initialized.
	StateInitialized ReleaseState = "initialized"
	// StatePlanned indicates the release has been planned (version determined).
	StatePlanned ReleaseState = "planned"
	// StateVersioned indicates the version has been set.
	StateVersioned ReleaseState = "versioned"
	// StateNotesGenerated indicates release notes have been generated.
	StateNotesGenerated ReleaseState = "notes_generated"
	// StateApproved indicates the release has been approved.
	StateApproved ReleaseState = "approved"
	// StatePublishing indicates the release is being published.
	StatePublishing ReleaseState = "publishing"
	// StatePublished indicates the release has been published.
	StatePublished ReleaseState = "published"
	// StateFailed indicates the release failed.
	StateFailed ReleaseState = "failed"
	// StateCanceled indicates the release was canceled.
	StateCanceled ReleaseState = "canceled"
)

// AllStates returns all valid release states.
func AllStates() []ReleaseState {
	return []ReleaseState{
		StateInitialized,
		StatePlanned,
		StateVersioned,
		StateNotesGenerated,
		StateApproved,
		StatePublishing,
		StatePublished,
		StateFailed,
		StateCanceled,
	}
}

// String returns the string representation of the state.
func (s ReleaseState) String() string {
	return string(s)
}

// IsValid returns true if the state is a valid release state.
func (s ReleaseState) IsValid() bool {
	switch s {
	case StateInitialized, StatePlanned, StateVersioned, StateNotesGenerated,
		StateApproved, StatePublishing, StatePublished, StateFailed, StateCanceled:
		return true
	default:
		return false
	}
}

// IsFinal returns true if this is a final (terminal) state.
func (s ReleaseState) IsFinal() bool {
	return s == StatePublished || s == StateFailed || s == StateCanceled
}

// IsActive returns true if the release is actively in progress.
func (s ReleaseState) IsActive() bool {
	return !s.IsFinal() && s != StateInitialized
}

// CanTransitionTo returns true if transitioning to the target state is valid.
func (s ReleaseState) CanTransitionTo(target ReleaseState) bool {
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

// validTransitions defines the state machine for release workflow.
func validTransitions() map[ReleaseState][]ReleaseState {
	return map[ReleaseState][]ReleaseState{
		StateInitialized:    {StatePlanned, StateCanceled},
		StatePlanned:        {StateVersioned, StateCanceled, StateInitialized},
		StateVersioned:      {StateNotesGenerated, StateCanceled, StatePlanned},
		StateNotesGenerated: {StateApproved, StateCanceled, StateVersioned},
		StateApproved:       {StatePublishing, StateCanceled, StateNotesGenerated},
		StatePublishing:     {StatePublished, StateFailed},
		StatePublished:      {},                               // Terminal state
		StateFailed:         {StateInitialized, StatePlanned}, // Can retry
		StateCanceled:       {StateInitialized},               // Can restart
	}
}

// NextValidStates returns the valid next states from the current state.
func (s ReleaseState) NextValidStates() []ReleaseState {
	transitions := validTransitions()
	if valid, exists := transitions[s]; exists {
		return valid
	}
	return nil
}

// ParseReleaseState parses a string into a ReleaseState.
func ParseReleaseState(s string) (ReleaseState, error) {
	state := ReleaseState(strings.ToLower(strings.TrimSpace(s)))
	if !state.IsValid() {
		return "", fmt.Errorf("invalid release state: %q", s)
	}
	return state, nil
}

// Description returns a human-readable description of the state.
func (s ReleaseState) Description() string {
	switch s {
	case StateInitialized:
		return "Release initialized"
	case StatePlanned:
		return "Release planned with version bump determined"
	case StateVersioned:
		return "Version has been calculated and set"
	case StateNotesGenerated:
		return "Release notes have been generated"
	case StateApproved:
		return "Release approved and ready to publish"
	case StatePublishing:
		return "Release is being published"
	case StatePublished:
		return "Release successfully published"
	case StateFailed:
		return "Release failed"
	case StateCanceled:
		return "Release canceled"
	default:
		return "Unknown state"
	}
}

// Icon returns an emoji icon for the state.
func (s ReleaseState) Icon() string {
	switch s {
	case StateInitialized:
		return "🔵"
	case StatePlanned:
		return "📋"
	case StateVersioned:
		return "🏷️"
	case StateNotesGenerated:
		return "📝"
	case StateApproved:
		return "✅"
	case StatePublishing:
		return "🚀"
	case StatePublished:
		return "🎉"
	case StateFailed:
		return "❌"
	case StateCanceled:
		return "⏹️"
	default:
		return "❓"
	}
}
