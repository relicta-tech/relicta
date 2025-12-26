// Package domain provides the core domain model for release governance.
package domain

import (
	"time"

	"github.com/relicta-tech/relicta/internal/domain/version"
)

// DomainEvent is the interface for all domain events.
type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
	AggregateID() RunID
}

// RunCreatedEvent is emitted when a new release run is created.
type RunCreatedEvent struct {
	RunID   RunID
	RepoID  string
	HeadSHA CommitSHA
	At      time.Time
}

func (e *RunCreatedEvent) EventName() string    { return "run.created" }
func (e *RunCreatedEvent) OccurredAt() time.Time { return e.At }

// StateTransitionedEvent is emitted on any state transition.
type StateTransitionedEvent struct {
	RunID RunID
	From  RunState
	To    RunState
	Event string
	Actor string
	At    time.Time
}

func (e *StateTransitionedEvent) EventName() string    { return "run.state_transitioned" }
func (e *StateTransitionedEvent) OccurredAt() time.Time { return e.At }

// RunApprovedEvent is emitted when a run is approved.
type RunApprovedEvent struct {
	RunID        RunID
	PlanHash     string
	ApprovedBy   string
	AutoApproved bool
	At           time.Time
}

func (e *RunApprovedEvent) EventName() string    { return "run.approved" }
func (e *RunApprovedEvent) OccurredAt() time.Time { return e.At }

// StepCompletedEvent is emitted when a publishing step completes.
type StepCompletedEvent struct {
	RunID    RunID
	StepName string
	Success  bool
	Error    string
	At       time.Time
}

func (e *StepCompletedEvent) EventName() string    { return "run.step_completed" }
func (e *StepCompletedEvent) OccurredAt() time.Time { return e.At }

// RunPublishedEvent is emitted when a run is successfully published.
type RunPublishedEvent struct {
	RunID   RunID
	Version version.SemanticVersion
	At      time.Time
}

func (e *RunPublishedEvent) EventName() string    { return "run.published" }
func (e *RunPublishedEvent) OccurredAt() time.Time { return e.At }

// RunFailedEvent is emitted when a run fails.
type RunFailedEvent struct {
	RunID  RunID
	Reason string
	At     time.Time
}

func (e *RunFailedEvent) EventName() string    { return "run.failed" }
func (e *RunFailedEvent) OccurredAt() time.Time { return e.At }

// RunCancelledEvent is emitted when a run is cancelled.
type RunCancelledEvent struct {
	RunID  RunID
	Reason string
	By     string
	At     time.Time
}

func (e *RunCancelledEvent) EventName() string     { return "run.cancelled" }
func (e *RunCancelledEvent) OccurredAt() time.Time { return e.At }

// RunVersionedEvent is emitted when a version is applied to the run.
type RunVersionedEvent struct {
	RunID       RunID
	VersionNext version.SemanticVersion
	BumpKind    BumpKind
	TagName     string
	Actor       string
	At          time.Time
}

func (e *RunVersionedEvent) EventName() string     { return "run.versioned" }
func (e *RunVersionedEvent) OccurredAt() time.Time { return e.At }

// RunRetriedEvent is emitted when a failed run is retried.
type RunRetriedEvent struct {
	RunID RunID
	By    string
	At    time.Time
}

func (e *RunRetriedEvent) EventName() string     { return "run.retried" }
func (e *RunRetriedEvent) OccurredAt() time.Time { return e.At }

// RunPlannedEvent is emitted when a run is planned.
type RunPlannedEvent struct {
	RunID          RunID
	VersionCurrent version.SemanticVersion
	VersionNext    version.SemanticVersion
	BumpKind       BumpKind
	CommitCount    int
	RiskScore      float64
	Actor          string
	At             time.Time
}

func (e *RunPlannedEvent) EventName() string     { return "run.planned" }
func (e *RunPlannedEvent) OccurredAt() time.Time { return e.At }

// RunNotesGeneratedEvent is emitted when release notes are generated.
type RunNotesGeneratedEvent struct {
	RunID       RunID
	NotesLength int
	Provider    string
	Model       string
	Actor       string
	At          time.Time
}

func (e *RunNotesGeneratedEvent) EventName() string     { return "run.notes_generated" }
func (e *RunNotesGeneratedEvent) OccurredAt() time.Time { return e.At }

// RunNotesUpdatedEvent is emitted when release notes are manually updated.
type RunNotesUpdatedEvent struct {
	RunID       RunID
	NotesLength int
	Actor       string
	At          time.Time
}

func (e *RunNotesUpdatedEvent) EventName() string     { return "run.notes_updated" }
func (e *RunNotesUpdatedEvent) OccurredAt() time.Time { return e.At }

// RunPublishingStartedEvent is emitted when publishing begins.
type RunPublishingStartedEvent struct {
	RunID    RunID
	Steps    []string
	PlanHash string
	Actor    string
	At       time.Time
}

func (e *RunPublishingStartedEvent) EventName() string     { return "run.publishing_started" }
func (e *RunPublishingStartedEvent) OccurredAt() time.Time { return e.At }

// PluginExecutedEvent is emitted when a plugin completes execution.
type PluginExecutedEvent struct {
	RunID      RunID
	PluginName string
	Hook       string
	Success    bool
	Message    string
	Duration   time.Duration
	At         time.Time
}

func (e *PluginExecutedEvent) EventName() string     { return "run.plugin_executed" }
func (e *PluginExecutedEvent) OccurredAt() time.Time { return e.At }

// AggregateID returns the aggregate ID for events that need it.
func (e *RunCreatedEvent) AggregateID() RunID        { return e.RunID }
func (e *StateTransitionedEvent) AggregateID() RunID { return e.RunID }
func (e *RunApprovedEvent) AggregateID() RunID       { return e.RunID }
func (e *StepCompletedEvent) AggregateID() RunID     { return e.RunID }
func (e *RunPublishedEvent) AggregateID() RunID      { return e.RunID }
func (e *RunFailedEvent) AggregateID() RunID         { return e.RunID }
func (e *RunCancelledEvent) AggregateID() RunID      { return e.RunID }
func (e *RunVersionedEvent) AggregateID() RunID      { return e.RunID }
func (e *RunRetriedEvent) AggregateID() RunID        { return e.RunID }
func (e *RunPlannedEvent) AggregateID() RunID        { return e.RunID }
func (e *RunNotesGeneratedEvent) AggregateID() RunID { return e.RunID }
func (e *RunNotesUpdatedEvent) AggregateID() RunID   { return e.RunID }
func (e *RunPublishingStartedEvent) AggregateID() RunID { return e.RunID }
func (e *PluginExecutedEvent) AggregateID() RunID    { return e.RunID }
