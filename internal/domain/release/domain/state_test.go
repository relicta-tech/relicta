package domain

import "testing"

func TestRunState_String(t *testing.T) {
	tests := []struct {
		state RunState
		want  string
	}{
		{StateDraft, "draft"},
		{StatePlanned, "planned"},
		{StateVersioned, "versioned"},
		{StateNotesReady, "notes_ready"},
		{StateApproved, "approved"},
		{StatePublishing, "publishing"},
		{StatePublished, "published"},
		{StateFailed, "failed"},
		{StateCanceled, "canceled"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("RunState(%v).String() = %v, want %v", tt.state, got, tt.want)
		}
	}
}

func TestRunState_IsValid(t *testing.T) {
	valid := []RunState{
		StateDraft,
		StatePlanned,
		StateVersioned,
		StateNotesReady,
		StateApproved,
		StatePublishing,
		StatePublished,
		StateFailed,
		StateCanceled,
	}

	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("RunState(%v).IsValid() = false, want true", s)
		}
	}

	invalid := RunState("invalid")
	if invalid.IsValid() {
		t.Error("RunState(invalid).IsValid() = true, want false")
	}
}

func TestRunState_IsFinal(t *testing.T) {
	terminal := []RunState{StatePublished, StateFailed, StateCanceled}
	for _, s := range terminal {
		if !s.IsFinal() {
			t.Errorf("RunState(%v).IsFinal() = false, want true", s)
		}
	}

	nonTerminal := []RunState{StateDraft, StatePlanned, StateVersioned, StateNotesReady, StateApproved, StatePublishing}
	for _, s := range nonTerminal {
		if s.IsFinal() {
			t.Errorf("RunState(%v).IsFinal() = true, want false", s)
		}
	}
}

func TestRunState_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from    RunState
		to      RunState
		allowed bool
	}{
		// Draft transitions
		{StateDraft, StatePlanned, true},
		{StateDraft, StateCanceled, true},
		{StateDraft, StateVersioned, false},
		{StateDraft, StatePublished, false},

		// Planned transitions
		{StatePlanned, StateVersioned, true},
		{StatePlanned, StateCanceled, true},
		{StatePlanned, StateDraft, false},
		{StatePlanned, StatePublished, false},

		// Versioned transitions
		{StateVersioned, StateNotesReady, true},
		{StateVersioned, StateCanceled, true},
		{StateVersioned, StatePlanned, true}, // Can go back to re-plan

		// NotesReady transitions
		{StateNotesReady, StateApproved, true},
		{StateNotesReady, StateCanceled, true},
		{StateNotesReady, StateVersioned, true}, // Can go back to regenerate notes

		// Approved transitions
		{StateApproved, StatePublishing, true},
		{StateApproved, StateCanceled, true},
		{StateApproved, StateNotesReady, false},

		// Publishing transitions
		{StatePublishing, StatePublished, true},
		{StatePublishing, StateFailed, true},
		{StatePublishing, StateCanceled, false}, // Can't cancel during publishing
		{StatePublishing, StateApproved, false},

		// Terminal states
		{StatePublished, StateDraft, false},
		{StatePublished, StateCanceled, false},
		{StateFailed, StatePublishing, true}, // Retry allowed
		{StateFailed, StateDraft, true},      // Can start over
		{StateCanceled, StateDraft, true},    // Can restart
	}

	for _, tt := range tests {
		got := tt.from.CanTransitionTo(tt.to)
		if got != tt.allowed {
			t.Errorf("RunState(%v).CanTransitionTo(%v) = %v, want %v", tt.from, tt.to, got, tt.allowed)
		}
	}
}

func TestAllStates(t *testing.T) {
	states := AllStates()
	if len(states) != 9 {
		t.Errorf("AllStates() returned %d states, want 9", len(states))
	}

	// Verify all states are valid
	for _, s := range states {
		if !s.IsValid() {
			t.Errorf("AllStates() contains invalid state: %v", s)
		}
	}
}

func TestRunState_IsActive(t *testing.T) {
	active := []RunState{StatePlanned, StateVersioned, StateNotesReady, StateApproved, StatePublishing}
	for _, s := range active {
		if !s.IsActive() {
			t.Errorf("RunState(%v).IsActive() = false, want true", s)
		}
	}

	inactive := []RunState{StateDraft, StatePublished, StateFailed, StateCanceled}
	for _, s := range inactive {
		if s.IsActive() {
			t.Errorf("RunState(%v).IsActive() = true, want false", s)
		}
	}
}

func TestRunState_NextValidStates(t *testing.T) {
	tests := []struct {
		state    RunState
		expected int
	}{
		{StateDraft, 2},      // Planned, Canceled
		{StatePlanned, 2},    // Versioned, Canceled
		{StateVersioned, 3},  // NotesReady, Planned, Canceled
		{StateNotesReady, 3}, // Approved, Versioned, Canceled
		{StateApproved, 2},   // Publishing, Canceled
		{StatePublishing, 2}, // Published, Failed
		{StatePublished, 0},  // Terminal
		{StateFailed, 2},     // Publishing, Draft
		{StateCanceled, 1},   // Draft
	}

	for _, tt := range tests {
		next := tt.state.NextValidStates()
		if len(next) != tt.expected {
			t.Errorf("RunState(%v).NextValidStates() returned %d states, want %d", tt.state, len(next), tt.expected)
		}
	}
}

func TestRunState_Description(t *testing.T) {
	tests := []struct {
		state       RunState
		wantContain string
	}{
		{StateDraft, "awaiting"},
		{StatePlanned, "planned"},
		{StateVersioned, "Version"},
		{StateNotesReady, "notes"},
		{StateApproved, "approved"},
		{StatePublishing, "publishing"},
		{StatePublished, "successfully"},
		{StateFailed, "failed"},
		{StateCanceled, "canceled"},
	}

	for _, tt := range tests {
		desc := tt.state.Description()
		if desc == "" {
			t.Errorf("RunState(%v).Description() is empty", tt.state)
		}
	}
}

func TestRunState_Icon(t *testing.T) {
	tests := []struct {
		state RunState
		want  string
	}{
		{StateDraft, "[DRAFT]"},
		{StatePlanned, "[PLANNED]"},
		{StateVersioned, "[VERSIONED]"},
		{StateNotesReady, "[NOTES]"},
		{StateApproved, "[APPROVED]"},
		{StatePublishing, "[PUBLISHING]"},
		{StatePublished, "[PUBLISHED]"},
		{StateFailed, "[FAILED]"},
		{StateCanceled, "[CANCELED]"},
	}

	for _, tt := range tests {
		got := tt.state.Icon()
		if got != tt.want {
			t.Errorf("RunState(%v).Icon() = %v, want %v", tt.state, got, tt.want)
		}
	}

	// Invalid state
	invalid := RunState("invalid")
	if invalid.Icon() != "[?]" {
		t.Errorf("RunState(invalid).Icon() = %v, want [?]", invalid.Icon())
	}
}

func TestParseRunState(t *testing.T) {
	tests := []struct {
		input   string
		want    RunState
		wantErr bool
	}{
		{"draft", StateDraft, false},
		{"planned", StatePlanned, false},
		{"versioned", StateVersioned, false},
		{"notes_ready", StateNotesReady, false},
		{"approved", StateApproved, false},
		{"publishing", StatePublishing, false},
		{"published", StatePublished, false},
		{"failed", StateFailed, false},
		{"canceled", StateCanceled, false},
		{"invalid", "", true},
		{"DRAFT", "", true},  // Case sensitive
		{"", "", true},
	}

	for _, tt := range tests {
		got, err := ParseRunState(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseRunState(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("ParseRunState(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
