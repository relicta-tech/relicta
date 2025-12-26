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

func (e *RunRetriedEvent) EventName() string    { return "run.retried" }
func (e *RunRetriedEvent) OccurredAt() time.Time { return e.At }
