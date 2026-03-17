// Package domain provides the core domain model for release governance.
package domain

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/statekit"
)

// RunContext is the context passed to the state machine.
type RunContext struct {
	Run         *ReleaseRun
	CurrentHead CommitSHA
	ForceMode   bool
	Actor       string
}

// Event names for the state machine.
const (
	EventPlan          statekit.EventType = "PLAN"
	EventBump          statekit.EventType = "BUMP"
	EventGenerateNotes statekit.EventType = "GENERATE_NOTES"
	EventApprove       statekit.EventType = "APPROVE"
	EventStartPublish  statekit.EventType = "START_PUBLISH"
	EventStepOK        statekit.EventType = "STEP_OK"
	EventStepFail      statekit.EventType = "STEP_FAIL"
	EventPublishDone   statekit.EventType = "PUBLISH_COMPLETE"
	EventRetryPublish  statekit.EventType = "RETRY_PUBLISH"
	EventCancel        statekit.EventType = "CANCEL"
	EventFail          statekit.EventType = "FAIL"
)

// Guard names for the state machine.
const (
	GuardHeadMatches         statekit.GuardType = "headMatches"
	GuardNotAlreadyPublished statekit.GuardType = "notAlreadyPublished"
	GuardAllStepsSucceeded   statekit.GuardType = "allStepsSucceeded"
)

// State IDs for the state machine.
var (
	StateIDDraft      statekit.StateID = statekit.StateID(StateDraft)
	StateIDPlanned    statekit.StateID = statekit.StateID(StatePlanned)
	StateIDVersioned  statekit.StateID = statekit.StateID(StateVersioned)
	StateIDNotesReady statekit.StateID = statekit.StateID(StateNotesReady)
	StateIDApproved   statekit.StateID = statekit.StateID(StateApproved)
	StateIDPublishing statekit.StateID = statekit.StateID(StatePublishing)
	StateIDPublished  statekit.StateID = statekit.StateID(StatePublished)
	StateIDFailed     statekit.StateID = statekit.StateID(StateFailed)
	StateIDCanceled   statekit.StateID = statekit.StateID(StateCanceled)
)

// ReleaseRunMachine wraps the Statekit state machine for release runs.
type ReleaseRunMachine struct {
	interpreter *statekit.Interpreter[RunContext]
}

// NewReleaseRunMachine creates a new state machine for release runs.
func NewReleaseRunMachine() (*ReleaseRunMachine, error) {
	machine, err := statekit.NewMachine[RunContext]("release-run").
		WithInitial(StateIDDraft).
		// Guards
		WithGuard(GuardHeadMatches, guardHeadMatches).
		WithGuard(GuardNotAlreadyPublished, guardNotAlreadyPublished).
		WithGuard(GuardAllStepsSucceeded, guardAllStepsSucceeded).
		// Draft state
		State(StateIDDraft).
		On(EventPlan).Target(StateIDPlanned).
		On(EventCancel).Target(StateIDCanceled).
		Done().
		// Planned state
		State(StateIDPlanned).
		On(EventBump).Target(StateIDVersioned).Guard(GuardHeadMatches).
		On(EventCancel).Target(StateIDCanceled).
		Done().
		// Versioned state (version calculated and applied)
		State(StateIDVersioned).
		On(EventGenerateNotes).Target(StateIDNotesReady).Guard(GuardHeadMatches).
		On(EventPlan).Target(StateIDPlanned).Guard(GuardHeadMatches). // Can go back to re-plan
		On(EventCancel).Target(StateIDCanceled).
		Done().
		// NotesReady state
		State(StateIDNotesReady).
		On(EventApprove).Target(StateIDApproved).Guard(GuardHeadMatches).
		On(EventGenerateNotes).Target(StateIDNotesReady).Guard(GuardHeadMatches). // Regenerate notes
		On(EventBump).Target(StateIDVersioned).Guard(GuardHeadMatches).           // Can go back to versioned
		On(EventCancel).Target(StateIDCanceled).
		Done().
		// Approved state
		State(StateIDApproved).
		On(EventStartPublish).Target(StateIDPublishing).Guard(GuardHeadMatches).
		On(EventCancel).Target(StateIDCanceled).
		Done().
		// Publishing state (compound state with sub-steps handled externally)
		State(StateIDPublishing).
		On(EventStepOK).Target(StateIDPublishing). // Stay in publishing, step completed
		On(EventStepFail).Target(StateIDFailed).   // Step failed
		On(EventPublishDone).Target(StateIDPublished).Guard(GuardAllStepsSucceeded).
		Done().
		// Published state (terminal)
		State(StateIDPublished).
		Final().
		Done().
		// Failed state
		State(StateIDFailed).
		On(EventRetryPublish).Target(StateIDPublishing).
		On(EventCancel).Target(StateIDCanceled).
		Done().
		// Canceled state (terminal)
		State(StateIDCanceled).
		Final().
		Done().
		Build()

	if err != nil {
		return nil, fmt.Errorf("failed to build state machine: %w", err)
	}

	interp := statekit.NewInterpreter(machine)

	return &ReleaseRunMachine{
		interpreter: interp,
	}, nil
}

// Guard implementations - Guards take context by value (not pointer)

func guardHeadMatches(ctx RunContext, _ statekit.Event) bool {
	if ctx.ForceMode {
		return true // Force mode bypasses head check
	}
	if ctx.Run == nil {
		return false
	}
	return ctx.Run.HeadSHA() == ctx.CurrentHead
}

func guardNotAlreadyPublished(ctx RunContext, _ statekit.Event) bool {
	if ctx.Run == nil {
		return false
	}
	return ctx.Run.State() != StatePublished
}

func guardAllStepsSucceeded(ctx RunContext, _ statekit.Event) bool {
	if ctx.Run == nil {
		return false
	}
	return ctx.Run.AllStepsSucceeded()
}

// Start starts the state machine interpreter.
func (m *ReleaseRunMachine) Start() {
	m.interpreter.Start()
}

// Send sends an event to the interpreter.
func (m *ReleaseRunMachine) Send(event statekit.EventType) error {
	if m.interpreter == nil {
		return fmt.Errorf("interpreter not started")
	}
	m.interpreter.Send(statekit.Event{Type: event})
	return nil
}

// CurrentState returns the current state.
func (m *ReleaseRunMachine) CurrentState() statekit.StateID {
	if m.interpreter == nil {
		return ""
	}
	return m.interpreter.State().Value
}

// IsDone returns true if the machine is in a final state.
func (m *ReleaseRunMachine) IsDone() bool {
	if m.interpreter == nil {
		return false
	}
	return m.interpreter.Done()
}

// XStateJSON represents the XState JSON format for visualization.
type XStateJSON struct {
	ID      string                     `json:"id"`
	Initial string                     `json:"initial"`
	States  map[string]XStateStateJSON `json:"states"`
	Context interface{}                `json:"context,omitempty"`
}

// XStateStateJSON represents a state in XState JSON format.
type XStateStateJSON struct {
	Type    string                      `json:"type,omitempty"` // "final" for terminal states
	On      map[string]XStateTransition `json:"on,omitempty"`
	Initial string                      `json:"initial,omitempty"` // For compound states
	States  map[string]XStateStateJSON  `json:"states,omitempty"`  // For compound states
}

// XStateTransition represents a transition in XState JSON format.
type XStateTransition struct {
	Target string `json:"target"`
	Guard  string `json:"cond,omitempty"`
}

// ExportXStateJSON exports the state machine definition as XState-compatible JSON.
func (m *ReleaseRunMachine) ExportXStateJSON() ([]byte, error) {
	xstate := XStateJSON{
		ID:      "release-run",
		Initial: string(StateDraft),
		States: map[string]XStateStateJSON{
			string(StateDraft): {
				On: map[string]XStateTransition{
					string(EventPlan):   {Target: string(StatePlanned)},
					string(EventCancel): {Target: string(StateCanceled)},
				},
			},
			string(StatePlanned): {
				On: map[string]XStateTransition{
					string(EventBump):   {Target: string(StateVersioned), Guard: string(GuardHeadMatches)},
					string(EventCancel): {Target: string(StateCanceled)},
				},
			},
			string(StateVersioned): {
				On: map[string]XStateTransition{
					string(EventGenerateNotes): {Target: string(StateNotesReady), Guard: string(GuardHeadMatches)},
					string(EventPlan):          {Target: string(StatePlanned), Guard: string(GuardHeadMatches)},
					string(EventCancel):        {Target: string(StateCanceled)},
				},
			},
			string(StateNotesReady): {
				On: map[string]XStateTransition{
					string(EventApprove):       {Target: string(StateApproved), Guard: string(GuardHeadMatches)},
					string(EventGenerateNotes): {Target: string(StateNotesReady), Guard: string(GuardHeadMatches)},
					string(EventBump):          {Target: string(StateVersioned), Guard: string(GuardHeadMatches)},
					string(EventCancel):        {Target: string(StateCanceled)},
				},
			},
			string(StateApproved): {
				On: map[string]XStateTransition{
					string(EventStartPublish): {Target: string(StatePublishing), Guard: string(GuardHeadMatches)},
					string(EventCancel):       {Target: string(StateCanceled)},
				},
			},
			string(StatePublishing): {
				On: map[string]XStateTransition{
					string(EventStepOK):      {Target: string(StatePublishing)},
					string(EventStepFail):    {Target: string(StateFailed)},
					string(EventPublishDone): {Target: string(StatePublished), Guard: string(GuardAllStepsSucceeded)},
				},
			},
			string(StatePublished): {
				Type: "final",
			},
			string(StateFailed): {
				On: map[string]XStateTransition{
					string(EventRetryPublish): {Target: string(StatePublishing)},
					string(EventCancel):       {Target: string(StateCanceled)},
				},
			},
			string(StateCanceled): {
				Type: "final",
			},
		},
	}

	return json.MarshalIndent(xstate, "", "  ")
}

// ExportStateSnapshot exports the current state of a run as JSON.
func ExportStateSnapshot(run *ReleaseRun) ([]byte, error) {
	snapshot := struct {
		RunID       string                 `json:"run_id"`
		State       string                 `json:"state"`
		HeadSHA     string                 `json:"head_sha"`
		PlanHash    string                 `json:"plan_hash"`
		VersionNext string                 `json:"version_next"`
		Steps       map[string]*StepStatus `json:"steps"`
		History     []TransitionRecord     `json:"history"`
		UpdatedAt   string                 `json:"updated_at"`
	}{
		RunID:       string(run.ID()),
		State:       string(run.State()),
		HeadSHA:     string(run.HeadSHA()),
		PlanHash:    run.PlanHash(),
		VersionNext: run.VersionNext().String(),
		Steps:       run.AllStepStatuses(),
		History:     run.History(),
		UpdatedAt:   run.UpdatedAt().Format("2006-01-02T15:04:05Z07:00"),
	}

	return json.MarshalIndent(snapshot, "", "  ")
}

// ValidateTransition checks if a transition is valid without executing it.
func ValidateTransition(run *ReleaseRun, event statekit.EventType, currentHead CommitSHA, force bool) error {
	ctx := RunContext{
		Run:         run,
		CurrentHead: currentHead,
		ForceMode:   force,
	}

	// Check guards based on event
	switch event {
	case EventBump, EventGenerateNotes, EventApprove, EventStartPublish:
		if !guardHeadMatches(ctx, statekit.Event{}) {
			return fmt.Errorf("%w: expected %s, got %s", ErrHeadSHAChanged, run.HeadSHA().Short(), currentHead.Short())
		}
	}

	// Check state transition is valid
	var targetState RunState
	switch event {
	case EventPlan:
		targetState = StatePlanned
	case EventBump:
		targetState = StateVersioned
	case EventGenerateNotes:
		targetState = StateNotesReady
	case EventApprove:
		targetState = StateApproved
	case EventStartPublish:
		targetState = StatePublishing
	case EventRetryPublish:
		targetState = StatePublishing
	case EventCancel:
		targetState = StateCanceled
	default:
		return fmt.Errorf("unknown event: %s", event)
	}

	if !run.State().CanTransitionTo(targetState) {
		return fmt.Errorf("%w: cannot transition from %s to %s via %s", ErrInvalidState, run.State(), targetState, event)
	}

	return nil
}

// StateMachineService provides state machine operations as a domain service.
type StateMachineService struct {
	machine *ReleaseRunMachine
}

// NewStateMachineService creates a new state machine service.
func NewStateMachineService() (*StateMachineService, error) {
	machine, err := NewReleaseRunMachine()
	if err != nil {
		return nil, err
	}
	return &StateMachineService{machine: machine}, nil
}

// ExportMachineJSON exports the state machine definition.
func (s *StateMachineService) ExportMachineJSON() ([]byte, error) {
	return s.machine.ExportXStateJSON()
}

// ExportRunStateJSON exports a run's current state.
func (s *StateMachineService) ExportRunStateJSON(run *ReleaseRun) ([]byte, error) {
	return ExportStateSnapshot(run)
}

// ValidateAndTransition validates and executes a state transition.
func (s *StateMachineService) ValidateAndTransition(
	ctx context.Context,
	run *ReleaseRun,
	event statekit.EventType,
	currentHead CommitSHA,
	actor string,
	force bool,
) error {
	// Validate first
	if err := ValidateTransition(run, event, currentHead, force); err != nil {
		return err
	}

	// Execute the transition on the aggregate
	switch event {
	case EventPlan:
		return run.Plan(actor)
	case EventBump:
		// Version is set separately via SetVersion, this just validates and transitions
		return run.Bump(actor)
	case EventGenerateNotes:
		// Notes are set separately, this just validates the transition is possible
		return nil
	case EventApprove:
		return run.Approve(actor, false)
	case EventStartPublish:
		return run.StartPublishing(actor)
	case EventRetryPublish:
		return run.RetryPublish(actor)
	case EventCancel:
		return run.Cancel("Canceled by user", actor)
	case EventFail:
		return run.MarkFailed("Failed", actor)
	default:
		return fmt.Errorf("unhandled event: %s", event)
	}
}
