package domain

import (
	"errors"
	"testing"
)

func TestStateTransitionError_Error(t *testing.T) {
	tests := []struct {
		name     string
		state    RunState
		action   string
		contains string
	}{
		{
			name:     "bump in draft state",
			state:    StateDraft,
			action:   "bump version",
			contains: "Run 'relicta plan' first",
		},
		{
			name:     "approve in versioned state",
			state:    StateVersioned,
			action:   "approve",
			contains: "Run 'relicta notes' first",
		},
		{
			name:     "publish in approved state has no guidance",
			state:    StateApproved,
			action:   "publish",
			contains: "cannot publish in state 'approved'",
		},
		{
			name:     "unknown action",
			state:    StateDraft,
			action:   "unknown",
			contains: "cannot unknown in state 'draft'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewStateTransitionError(tt.state, tt.action)
			msg := err.Error()
			if len(msg) == 0 {
				t.Error("Error() returned empty string")
			}
			if tt.contains != "" && !contains(msg, tt.contains) {
				t.Errorf("Error() = %q, want to contain %q", msg, tt.contains)
			}
		})
	}
}

func TestStateTransitionError_Unwrap(t *testing.T) {
	err := NewStateTransitionError(StateDraft, "bump")
	unwrapped := err.Unwrap()
	if !errors.Is(unwrapped, ErrInvalidState) {
		t.Errorf("Unwrap() = %v, want ErrInvalidState", unwrapped)
	}
}

func TestStateTransitionError_getGuidance(t *testing.T) {
	tests := []struct {
		action   string
		state    RunState
		expected string
	}{
		// set version / bump version / bump
		{"set version", StateDraft, "Run 'relicta plan' first"},
		{"bump version", StateVersioned, "Version is already set"},
		{"bump", StateFailed, "Release failed. Use 'relicta retry'"},
		{"bump", StateCanceled, "Release was canceled"},
		// generate notes / set notes
		{"generate notes", StateDraft, "Run 'relicta plan' then 'relicta bump' first"},
		{"set notes", StatePlanned, "Run 'relicta bump' first"},
		{"generate notes", StateNotesReady, "Notes are already generated"},
		{"set notes", StateFailed, "Release failed"},
		// approve
		{"approve", StateDraft, "plan', 'relicta bump', and 'relicta notes' first"},
		{"approve", StatePlanned, "'relicta bump' and 'relicta notes' first"},
		{"approve", StateVersioned, "Run 'relicta notes' first"},
		{"approve", StateApproved, "already approved"},
		{"approve", StatePublished, "already published"},
		{"approve", StatePublishing, "currently being published"},
		{"approve", StateFailed, "Release failed"},
		// publish / start publishing
		{"publish", StateDraft, "Complete the workflow"},
		{"start publishing", StatePlanned, "'relicta bump', 'relicta notes', and 'relicta approve' first"},
		{"publish", StateVersioned, "'relicta notes' and 'relicta approve' first"},
		{"publish", StateNotesReady, "Run 'relicta approve' first"},
		{"publish", StatePublishing, "already being published"},
		{"start publishing", StatePublished, "already published"},
		{"publish", StateFailed, "Use 'relicta retry'"},
		// retry
		{"retry", StateDraft, "Only failed or canceled releases"},
		{"retry", StatePlanned, "Only failed or canceled releases"},
		{"retry", StateFailed, ""},   // valid state, no guidance
		{"retry", StateCanceled, ""}, // valid state, no guidance
		// cancel
		{"cancel", StatePublished, "Cannot cancel a published release"},
		{"cancel", StatePublishing, "Cannot cancel during publishing"},
		{"cancel", StateCanceled, "already canceled"},
		// update notes
		{"update notes", StateApproved, "Notes can only be updated in 'notes_ready' state"},
		{"update notes", StateNotesReady, ""}, // valid state, no guidance
		// plan
		{"plan", StateVersioned, "Release has progressed past planning"},
		{"plan", StateFailed, "Release failed"},
	}

	for _, tt := range tests {
		t.Run(tt.action+"_"+string(tt.state), func(t *testing.T) {
			err := &StateTransitionError{CurrentState: tt.state, Action: tt.action}
			guidance := err.getGuidance()
			if tt.expected != "" && !contains(guidance, tt.expected) {
				t.Errorf("getGuidance() = %q, want to contain %q", guidance, tt.expected)
			}
		})
	}
}

func TestHeadMismatchError_Error(t *testing.T) {
	err := NewHeadMismatchError("abc1234567890", "def0987654321")
	msg := err.Error()

	if !contains(msg, "abc1234") || !contains(msg, "def0987") {
		t.Errorf("Error() = %q, expected to contain short SHA hashes", msg)
	}
	if !contains(msg, "HEAD has changed") {
		t.Errorf("Error() = %q, expected to contain 'HEAD has changed'", msg)
	}
}

func TestHeadMismatchError_Unwrap(t *testing.T) {
	err := NewHeadMismatchError("abc123", "def456")
	unwrapped := err.Unwrap()
	if !errors.Is(unwrapped, ErrHeadSHAChanged) {
		t.Errorf("Unwrap() = %v, want ErrHeadSHAChanged", unwrapped)
	}
}

func TestNewHeadMismatchError(t *testing.T) {
	err := NewHeadMismatchError("expected", "actual")
	if err == nil {
		t.Fatal("NewHeadMismatchError returned nil")
	}
	if err.ExpectedSHA != "expected" {
		t.Errorf("ExpectedSHA = %s, want expected", err.ExpectedSHA)
	}
	if err.ActualSHA != "actual" {
		t.Errorf("ActualSHA = %s, want actual", err.ActualSHA)
	}
}

func TestStepError_Error(t *testing.T) {
	err := NewStepError("create-tag", StepTypeTag, 3, "permission denied")
	msg := err.Error()

	if !contains(msg, "create-tag") {
		t.Errorf("Error() = %q, expected to contain step name", msg)
	}
	if !contains(msg, string(StepTypeTag)) {
		t.Errorf("Error() = %q, expected to contain step type", msg)
	}
	if !contains(msg, "3") {
		t.Errorf("Error() = %q, expected to contain attempt count", msg)
	}
	if !contains(msg, "permission denied") {
		t.Errorf("Error() = %q, expected to contain last error", msg)
	}
}

func TestNewStepError(t *testing.T) {
	err := NewStepError("my-step", StepTypeChangelog, 2, "failed")
	if err == nil {
		t.Fatal("NewStepError returned nil")
	}
	if err.StepName != "my-step" {
		t.Errorf("StepName = %s, want my-step", err.StepName)
	}
	if err.StepType != StepTypeChangelog {
		t.Errorf("StepType = %s, want changelog", err.StepType)
	}
	if err.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", err.Attempts)
	}
	if err.LastError != "failed" {
		t.Errorf("LastError = %s, want failed", err.LastError)
	}
}

func TestRiskThresholdError_Error(t *testing.T) {
	err := NewRiskThresholdError(0.85, 0.70, []string{"high complexity", "no tests"})
	msg := err.Error()

	if !contains(msg, "0.85") {
		t.Errorf("Error() = %q, expected to contain risk score", msg)
	}
	if !contains(msg, "0.70") {
		t.Errorf("Error() = %q, expected to contain threshold", msg)
	}
	if !contains(msg, "exceeds threshold") {
		t.Errorf("Error() = %q, expected to contain 'exceeds threshold'", msg)
	}
}

func TestRiskThresholdError_Unwrap(t *testing.T) {
	err := NewRiskThresholdError(0.9, 0.7, nil)
	unwrapped := err.Unwrap()
	if !errors.Is(unwrapped, ErrRiskTooHigh) {
		t.Errorf("Unwrap() = %v, want ErrRiskTooHigh", unwrapped)
	}
}

func TestNewRiskThresholdError(t *testing.T) {
	reasons := []string{"reason1", "reason2"}
	err := NewRiskThresholdError(0.9, 0.7, reasons)
	if err == nil {
		t.Fatal("NewRiskThresholdError returned nil")
	}
	if err.RiskScore != 0.9 {
		t.Errorf("RiskScore = %f, want 0.9", err.RiskScore)
	}
	if err.Threshold != 0.7 {
		t.Errorf("Threshold = %f, want 0.7", err.Threshold)
	}
	if len(err.Reasons) != 2 {
		t.Errorf("Reasons length = %d, want 2", len(err.Reasons))
	}
}

// contains is a helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
