package domain

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/felixgeelhaar/statekit"

	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestNewReleaseRunMachine(t *testing.T) {
	machine, err := NewReleaseRunMachine()
	if err != nil {
		t.Fatalf("NewReleaseRunMachine() error = %v", err)
	}
	if machine == nil {
		t.Fatal("NewReleaseRunMachine() returned nil machine")
	}
}

func TestReleaseRunMachine_Start(t *testing.T) {
	machine, err := NewReleaseRunMachine()
	if err != nil {
		t.Fatalf("NewReleaseRunMachine() error = %v", err)
	}

	machine.Start()

	// Should start in draft state
	if machine.CurrentState() != StateIDDraft {
		t.Errorf("CurrentState() = %v, want %v", machine.CurrentState(), StateIDDraft)
	}
}

func TestReleaseRunMachine_IsDone(t *testing.T) {
	machine, err := NewReleaseRunMachine()
	if err != nil {
		t.Fatalf("NewReleaseRunMachine() error = %v", err)
	}

	// Before starting
	if machine.IsDone() {
		t.Error("IsDone() = true before starting, want false")
	}

	machine.Start()

	// After starting in non-final state
	if machine.IsDone() {
		t.Error("IsDone() = true in draft state, want false")
	}
}

func TestReleaseRunMachine_Send_AfterStart(t *testing.T) {
	machine, err := NewReleaseRunMachine()
	if err != nil {
		t.Fatalf("NewReleaseRunMachine() error = %v", err)
	}

	machine.Start()

	// After starting, sending should work
	err = machine.Send(EventPlan)
	if err != nil {
		t.Errorf("Send() after Start() error = %v", err)
	}
}

func TestReleaseRunMachine_CurrentState_NotStarted(t *testing.T) {
	machine, err := NewReleaseRunMachine()
	if err != nil {
		t.Fatalf("NewReleaseRunMachine() error = %v", err)
	}

	// Current state before starting
	state := machine.CurrentState()
	if state != "" {
		t.Errorf("CurrentState() = %v, want empty string before starting", state)
	}
}

func TestReleaseRunMachine_ExportXStateJSON(t *testing.T) {
	machine, err := NewReleaseRunMachine()
	if err != nil {
		t.Fatalf("NewReleaseRunMachine() error = %v", err)
	}

	jsonBytes, err := machine.ExportXStateJSON()
	if err != nil {
		t.Fatalf("ExportXStateJSON() error = %v", err)
	}

	if len(jsonBytes) == 0 {
		t.Error("ExportXStateJSON() returned empty bytes")
	}

	// Verify it's valid JSON
	var xstate XStateJSON
	if err := json.Unmarshal(jsonBytes, &xstate); err != nil {
		t.Fatalf("ExportXStateJSON() returned invalid JSON: %v", err)
	}

	// Verify structure
	if xstate.ID != "release-run" {
		t.Errorf("XState ID = %v, want release-run", xstate.ID)
	}
	if xstate.Initial != "draft" {
		t.Errorf("XState Initial = %v, want draft", xstate.Initial)
	}
	if len(xstate.States) != 9 {
		t.Errorf("XState States count = %d, want 9", len(xstate.States))
	}

	// Verify terminal states have correct type
	publishedState := xstate.States["published"]
	if publishedState.Type != "final" {
		t.Errorf("Published state type = %v, want final", publishedState.Type)
	}
	canceledState := xstate.States["canceled"]
	if canceledState.Type != "final" {
		t.Errorf("Canceled state type = %v, want final", canceledState.Type)
	}
}

func TestExportStateSnapshot(t *testing.T) {
	run := newApprovedRun()
	run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})

	jsonBytes, err := ExportStateSnapshot(run)
	if err != nil {
		t.Fatalf("ExportStateSnapshot() error = %v", err)
	}

	if len(jsonBytes) == 0 {
		t.Error("ExportStateSnapshot() returned empty bytes")
	}

	// Verify it's valid JSON
	var snapshot map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &snapshot); err != nil {
		t.Fatalf("ExportStateSnapshot() returned invalid JSON: %v", err)
	}

	// Verify fields
	if snapshot["state"] != "approved" {
		t.Errorf("Snapshot state = %v, want approved", snapshot["state"])
	}
	if _, ok := snapshot["run_id"]; !ok {
		t.Error("Snapshot missing run_id field")
	}
	if _, ok := snapshot["head_sha"]; !ok {
		t.Error("Snapshot missing head_sha field")
	}
	if _, ok := snapshot["steps"]; !ok {
		t.Error("Snapshot missing steps field")
	}
}

func TestValidateTransition(t *testing.T) {
	t.Run("valid transition", func(t *testing.T) {
		run := newTestRun()
		_ = run.Plan("test")
		err := ValidateTransition(run, EventBump, run.HeadSHA(), false)
		if err != nil {
			t.Errorf("ValidateTransition() error = %v, want nil", err)
		}
	})

	t.Run("invalid state transition", func(t *testing.T) {
		run := newTestRun()
		err := ValidateTransition(run, EventBump, run.HeadSHA(), false)
		if err == nil {
			t.Error("ValidateTransition() expected error for invalid transition from draft")
		}
	})

	t.Run("head SHA mismatch", func(t *testing.T) {
		run := newTestRun()
		_ = run.Plan("test")
		err := ValidateTransition(run, EventBump, "different-sha", false)
		if err == nil {
			t.Error("ValidateTransition() expected error for head SHA mismatch")
		}
	})

	t.Run("force mode bypasses head check", func(t *testing.T) {
		run := newTestRun()
		_ = run.Plan("test")
		err := ValidateTransition(run, EventBump, "different-sha", true)
		if err != nil {
			t.Errorf("ValidateTransition() with force error = %v, want nil", err)
		}
	})

	t.Run("unknown event", func(t *testing.T) {
		run := newTestRun()
		err := ValidateTransition(run, "UNKNOWN_EVENT", run.HeadSHA(), false)
		if err == nil {
			t.Error("ValidateTransition() expected error for unknown event")
		}
	})

	t.Run("all event types", func(t *testing.T) {
		events := []struct {
			event       statekit.EventType
			fromState   func() *ReleaseRun
			expectError bool
		}{
			{EventPlan, newTestRun, false},
			{EventBump, func() *ReleaseRun {
				run := newTestRun()
				_ = run.Plan("test")
				return run
			}, false},
			{EventGenerateNotes, func() *ReleaseRun {
				return newVersionedRun()
			}, false},
			{EventApprove, func() *ReleaseRun {
				return newNotesReadyRun()
			}, false},
			{EventStartPublish, func() *ReleaseRun {
				return newApprovedRun()
			}, false},
			{EventCancel, newTestRun, false},
		}

		for _, tc := range events {
			run := tc.fromState()
			err := ValidateTransition(run, tc.event, run.HeadSHA(), false)
			hasErr := err != nil
			if hasErr != tc.expectError {
				t.Errorf("ValidateTransition(%s) hasErr = %v, want %v", tc.event, hasErr, tc.expectError)
			}
		}
	})
}

func TestGuardHeadMatches(t *testing.T) {
	t.Run("matches", func(t *testing.T) {
		run := newTestRun()
		ctx := RunContext{
			Run:         run,
			CurrentHead: run.HeadSHA(),
			ForceMode:   false,
		}
		if !guardHeadMatches(ctx, statekit.Event{}) {
			t.Error("guardHeadMatches() = false when heads match, want true")
		}
	})

	t.Run("does not match", func(t *testing.T) {
		run := newTestRun()
		ctx := RunContext{
			Run:         run,
			CurrentHead: "different-sha",
			ForceMode:   false,
		}
		if guardHeadMatches(ctx, statekit.Event{}) {
			t.Error("guardHeadMatches() = true when heads don't match, want false")
		}
	})

	t.Run("force mode bypasses", func(t *testing.T) {
		run := newTestRun()
		ctx := RunContext{
			Run:         run,
			CurrentHead: "different-sha",
			ForceMode:   true,
		}
		if !guardHeadMatches(ctx, statekit.Event{}) {
			t.Error("guardHeadMatches() = false in force mode, want true")
		}
	})

	t.Run("nil run", func(t *testing.T) {
		ctx := RunContext{
			Run:         nil,
			CurrentHead: "sha",
			ForceMode:   false,
		}
		if guardHeadMatches(ctx, statekit.Event{}) {
			t.Error("guardHeadMatches() = true with nil run, want false")
		}
	})
}

func TestGuardNotAlreadyPublished(t *testing.T) {
	t.Run("not published", func(t *testing.T) {
		run := newApprovedRun()
		ctx := RunContext{Run: run}
		if !guardNotAlreadyPublished(ctx, statekit.Event{}) {
			t.Error("guardNotAlreadyPublished() = false for approved run, want true")
		}
	})

	t.Run("published", func(t *testing.T) {
		run := newPublishingRun()
		_ = run.MarkPublished("test")
		ctx := RunContext{Run: run}
		if guardNotAlreadyPublished(ctx, statekit.Event{}) {
			t.Error("guardNotAlreadyPublished() = true for published run, want false")
		}
	})

	t.Run("nil run", func(t *testing.T) {
		ctx := RunContext{Run: nil}
		if guardNotAlreadyPublished(ctx, statekit.Event{}) {
			t.Error("guardNotAlreadyPublished() = true with nil run, want false")
		}
	})
}

func TestGuardAllStepsSucceeded(t *testing.T) {
	t.Run("all steps succeeded", func(t *testing.T) {
		run := newApprovedRun()
		run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
		_ = run.StartPublishing("test")
		_ = run.MarkStepDone("tag", "done")
		ctx := RunContext{Run: run}
		if !guardAllStepsSucceeded(ctx, statekit.Event{}) {
			t.Error("guardAllStepsSucceeded() = false when all steps done, want true")
		}
	})

	t.Run("step failed", func(t *testing.T) {
		run := newApprovedRun()
		run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
		_ = run.StartPublishing("test")
		_ = run.MarkStepStarted("tag")
		_ = run.MarkStepFailed("tag", ErrStepNotFound)
		ctx := RunContext{Run: run}
		if guardAllStepsSucceeded(ctx, statekit.Event{}) {
			t.Error("guardAllStepsSucceeded() = true when step failed, want false")
		}
	})

	t.Run("nil run", func(t *testing.T) {
		ctx := RunContext{Run: nil}
		if guardAllStepsSucceeded(ctx, statekit.Event{}) {
			t.Error("guardAllStepsSucceeded() = true with nil run, want false")
		}
	})
}

func TestNewStateMachineService(t *testing.T) {
	svc, err := NewStateMachineService()
	if err != nil {
		t.Fatalf("NewStateMachineService() error = %v", err)
	}
	if svc == nil {
		t.Fatal("NewStateMachineService() returned nil")
	}
}

func TestStateMachineService_ExportMachineJSON(t *testing.T) {
	svc, err := NewStateMachineService()
	if err != nil {
		t.Fatalf("NewStateMachineService() error = %v", err)
	}

	jsonBytes, err := svc.ExportMachineJSON()
	if err != nil {
		t.Fatalf("ExportMachineJSON() error = %v", err)
	}

	if len(jsonBytes) == 0 {
		t.Error("ExportMachineJSON() returned empty bytes")
	}
}

func TestStateMachineService_ExportRunStateJSON(t *testing.T) {
	svc, err := NewStateMachineService()
	if err != nil {
		t.Fatalf("NewStateMachineService() error = %v", err)
	}

	run := newApprovedRun()
	jsonBytes, err := svc.ExportRunStateJSON(run)
	if err != nil {
		t.Fatalf("ExportRunStateJSON() error = %v", err)
	}

	if len(jsonBytes) == 0 {
		t.Error("ExportRunStateJSON() returned empty bytes")
	}
}

func TestStateMachineService_ValidateAndTransition(t *testing.T) {
	svc, err := NewStateMachineService()
	if err != nil {
		t.Fatalf("NewStateMachineService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("plan transition", func(t *testing.T) {
		run := newTestRun()
		err := svc.ValidateAndTransition(ctx, run, EventPlan, run.HeadSHA(), "test", false)
		if err != nil {
			t.Errorf("ValidateAndTransition(EventPlan) error = %v", err)
		}
		if run.State() != StatePlanned {
			t.Errorf("Run state = %v, want %v", run.State(), StatePlanned)
		}
	})

	t.Run("bump transition", func(t *testing.T) {
		run := newTestRun()
		_ = run.Plan("test")
		_ = run.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
		err := svc.ValidateAndTransition(ctx, run, EventBump, run.HeadSHA(), "test", false)
		if err != nil {
			t.Errorf("ValidateAndTransition(EventBump) error = %v", err)
		}
		if run.State() != StateVersioned {
			t.Errorf("Run state = %v, want %v", run.State(), StateVersioned)
		}
	})

	t.Run("approve transition", func(t *testing.T) {
		run := newNotesReadyRun()
		err := svc.ValidateAndTransition(ctx, run, EventApprove, run.HeadSHA(), "test", false)
		if err != nil {
			t.Errorf("ValidateAndTransition(EventApprove) error = %v", err)
		}
		if run.State() != StateApproved {
			t.Errorf("Run state = %v, want %v", run.State(), StateApproved)
		}
	})

	t.Run("start publish transition", func(t *testing.T) {
		run := newApprovedRun()
		run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
		err := svc.ValidateAndTransition(ctx, run, EventStartPublish, run.HeadSHA(), "test", false)
		if err != nil {
			t.Errorf("ValidateAndTransition(EventStartPublish) error = %v", err)
		}
		if run.State() != StatePublishing {
			t.Errorf("Run state = %v, want %v", run.State(), StatePublishing)
		}
	})

	t.Run("cancel transition", func(t *testing.T) {
		run := newApprovedRun()
		err := svc.ValidateAndTransition(ctx, run, EventCancel, run.HeadSHA(), "test", false)
		if err != nil {
			t.Errorf("ValidateAndTransition(EventCancel) error = %v", err)
		}
		if run.State() != StateCanceled {
			t.Errorf("Run state = %v, want %v", run.State(), StateCanceled)
		}
	})

	t.Run("fail transition through MarkFailed", func(t *testing.T) {
		run := newApprovedRun()
		run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
		_ = run.StartPublishing("test")
		// EventFail is not handled by ValidateAndTransition - use MarkFailed directly
		err := run.MarkFailed("step error", "test")
		if err != nil {
			t.Errorf("MarkFailed() error = %v", err)
		}
		if run.State() != StateFailed {
			t.Errorf("Run state = %v, want %v", run.State(), StateFailed)
		}
	})

	t.Run("retry publish transition", func(t *testing.T) {
		run := newApprovedRun()
		run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
		_ = run.StartPublishing("test")
		_ = run.MarkStepStarted("tag")
		_ = run.MarkStepFailed("tag", ErrStepNotFound)
		_ = run.MarkFailed("step failed", "test")
		err := svc.ValidateAndTransition(ctx, run, EventRetryPublish, run.HeadSHA(), "test", false)
		if err != nil {
			t.Errorf("ValidateAndTransition(EventRetryPublish) error = %v", err)
		}
		if run.State() != StatePublishing {
			t.Errorf("Run state = %v, want %v", run.State(), StatePublishing)
		}
	})

	t.Run("generate notes transition returns nil", func(t *testing.T) {
		run := newVersionedRun()
		err := svc.ValidateAndTransition(ctx, run, EventGenerateNotes, run.HeadSHA(), "test", false)
		if err != nil {
			t.Errorf("ValidateAndTransition(EventGenerateNotes) error = %v", err)
		}
		// State doesn't change as notes are set separately
	})

	t.Run("unhandled event", func(t *testing.T) {
		run := newTestRun()
		err := svc.ValidateAndTransition(ctx, run, "UNKNOWN_EVENT", run.HeadSHA(), "test", false)
		if err == nil {
			t.Error("ValidateAndTransition() expected error for unknown event")
		}
	})
}

func TestValidateTransition_AllEvents(t *testing.T) {
	tests := []struct {
		name    string
		event   statekit.EventType
		run     func() *ReleaseRun
		wantErr bool
	}{
		{"EventPlan from draft", EventPlan, newTestRun, false},
		{"EventRetryPublish from failed", EventRetryPublish, func() *ReleaseRun {
			run := newApprovedRun()
			run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
			_ = run.StartPublishing("test")
			_ = run.MarkFailed("error", "test")
			return run
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := tt.run()
			err := ValidateTransition(run, tt.event, run.HeadSHA(), false)
			hasErr := err != nil
			if hasErr != tt.wantErr {
				t.Errorf("ValidateTransition() hasErr = %v, wantErr %v, err = %v", hasErr, tt.wantErr, err)
			}
		})
	}
}
