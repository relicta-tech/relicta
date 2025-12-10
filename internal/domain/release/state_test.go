// Package release provides domain types for release management.
package release

import (
	"testing"
)

func TestReleaseState_String(t *testing.T) {
	tests := []struct {
		state    ReleaseState
		expected string
	}{
		{StateInitialized, "initialized"},
		{StatePlanned, "planned"},
		{StateVersioned, "versioned"},
		{StateNotesGenerated, "notes_generated"},
		{StateApproved, "approved"},
		{StatePublishing, "publishing"},
		{StatePublished, "published"},
		{StateFailed, "failed"},
		{StateCanceled, "canceled"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestReleaseState_IsValid(t *testing.T) {
	validStates := []ReleaseState{
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

	for _, state := range validStates {
		if !state.IsValid() {
			t.Errorf("IsValid() = false for %s, want true", state)
		}
	}

	invalidStates := []ReleaseState{
		"invalid",
		"",
		"INITIALIZED",
		"unknown",
	}

	for _, state := range invalidStates {
		if state.IsValid() {
			t.Errorf("IsValid() = true for %q, want false", state)
		}
	}
}

func TestReleaseState_IsFinal(t *testing.T) {
	finalStates := []ReleaseState{
		StatePublished,
		StateFailed,
		StateCanceled,
	}

	for _, state := range finalStates {
		if !state.IsFinal() {
			t.Errorf("IsFinal() = false for %s, want true", state)
		}
	}

	nonFinalStates := []ReleaseState{
		StateInitialized,
		StatePlanned,
		StateVersioned,
		StateNotesGenerated,
		StateApproved,
		StatePublishing,
	}

	for _, state := range nonFinalStates {
		if state.IsFinal() {
			t.Errorf("IsFinal() = true for %s, want false", state)
		}
	}
}

func TestReleaseState_IsActive(t *testing.T) {
	activeStates := []ReleaseState{
		StatePlanned,
		StateVersioned,
		StateNotesGenerated,
		StateApproved,
		StatePublishing,
	}

	for _, state := range activeStates {
		if !state.IsActive() {
			t.Errorf("IsActive() = false for %s, want true", state)
		}
	}

	inactiveStates := []ReleaseState{
		StateInitialized,
		StatePublished,
		StateFailed,
		StateCanceled,
	}

	for _, state := range inactiveStates {
		if state.IsActive() {
			t.Errorf("IsActive() = true for %s, want false", state)
		}
	}
}

func TestReleaseState_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from     ReleaseState
		to       ReleaseState
		expected bool
	}{
		// From Initialized
		{StateInitialized, StatePlanned, true},
		{StateInitialized, StateCanceled, true},
		{StateInitialized, StatePublished, false},
		{StateInitialized, StateVersioned, false},

		// From Planned
		{StatePlanned, StateVersioned, true},
		{StatePlanned, StateCanceled, true},
		{StatePlanned, StateInitialized, true}, // Allow reset
		{StatePlanned, StatePublished, false},

		// From Versioned
		{StateVersioned, StateNotesGenerated, true},
		{StateVersioned, StateCanceled, true},
		{StateVersioned, StatePlanned, true}, // Allow rollback
		{StateVersioned, StatePublished, false},

		// From NotesGenerated
		{StateNotesGenerated, StateApproved, true},
		{StateNotesGenerated, StateCanceled, true},
		{StateNotesGenerated, StateVersioned, true}, // Allow rollback
		{StateNotesGenerated, StatePublished, false},

		// From Approved
		{StateApproved, StatePublishing, true},
		{StateApproved, StateCanceled, true},
		{StateApproved, StateNotesGenerated, true}, // Allow rollback
		{StateApproved, StatePublished, false},

		// From Publishing
		{StatePublishing, StatePublished, true},
		{StatePublishing, StateFailed, true},
		{StatePublishing, StateCanceled, false}, // Can't cancel during publish

		// From Published (terminal)
		{StatePublished, StateInitialized, false},
		{StatePublished, StateFailed, false},

		// From Failed
		{StateFailed, StateInitialized, true},
		{StateFailed, StatePlanned, true},
		{StateFailed, StatePublished, false},

		// From Canceled
		{StateCanceled, StateInitialized, true},
		{StateCanceled, StatePlanned, false},
	}

	for _, tt := range tests {
		name := string(tt.from) + "_to_" + string(tt.to)
		t.Run(name, func(t *testing.T) {
			if got := tt.from.CanTransitionTo(tt.to); got != tt.expected {
				t.Errorf("CanTransitionTo(%s) = %v, want %v", tt.to, got, tt.expected)
			}
		})
	}
}

func TestReleaseState_NextValidStates(t *testing.T) {
	tests := []struct {
		state    ReleaseState
		expected []ReleaseState
	}{
		{StateInitialized, []ReleaseState{StatePlanned, StateCanceled}},
		{StatePublishing, []ReleaseState{StatePublished, StateFailed}},
		{StatePublished, nil}, // Terminal state
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			got := tt.state.NextValidStates()

			if len(got) != len(tt.expected) {
				t.Errorf("NextValidStates() length = %d, want %d", len(got), len(tt.expected))
				return
			}

			for i, state := range got {
				found := false
				for _, exp := range tt.expected {
					if state == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("NextValidStates()[%d] = %v, not in expected", i, state)
				}
			}
		})
	}
}

func TestParseReleaseState(t *testing.T) {
	tests := []struct {
		input    string
		expected ReleaseState
		wantErr  bool
	}{
		{"initialized", StateInitialized, false},
		{"INITIALIZED", StateInitialized, false},
		{"  planned  ", StatePlanned, false},
		{"published", StatePublished, false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseReleaseState(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseReleaseState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != tt.expected {
				t.Errorf("ParseReleaseState() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestReleaseState_Description(t *testing.T) {
	// Just verify all states have non-empty descriptions
	for _, state := range AllStates() {
		desc := state.Description()
		if desc == "" || desc == "Unknown state" {
			t.Errorf("Description() for %s is empty or unknown", state)
		}
	}

	// Test unknown state
	unknown := ReleaseState("unknown")
	if unknown.Description() != "Unknown state" {
		t.Errorf("Description() for unknown state = %q, want 'Unknown state'", unknown.Description())
	}
}

func TestReleaseState_Icon(t *testing.T) {
	// Just verify all states have icons
	for _, state := range AllStates() {
		icon := state.Icon()
		if icon == "" {
			t.Errorf("Icon() for %s is empty", state)
		}
	}
}

func TestAllStates(t *testing.T) {
	states := AllStates()

	expectedCount := 9
	if len(states) != expectedCount {
		t.Errorf("AllStates() length = %d, want %d", len(states), expectedCount)
	}

	// Verify all returned states are valid
	for _, state := range states {
		if !state.IsValid() {
			t.Errorf("AllStates() contains invalid state: %v", state)
		}
	}
}
