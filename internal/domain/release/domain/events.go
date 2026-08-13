// Package domain provides the core domain model for release governance.
package domain

import (
	"strings"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// DomainEvent is the interface for all domain events.
type DomainEvent interface {
	// EventName is the event's name on the wire: "release.published",
	// "release.canceled", and so on.
	//
	// These used to be "run.*" while the webhook configuration documented "release.*"
	// and offered "release.*" as its wildcard example. So a user who configured exactly
	// what the comment described received nothing, and the mismatch was invisible —
	// shouldSendEvent simply matched no event and there was nothing to log. The
	// documented vocabulary won because it is also the one pkg/cgp and Hub use, so a
	// webhook payload and a Hub event now name the same thing the same way.
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

// CanonicalEventName maps a stored event name to the current vocabulary.
//
// Event stores hold names written by whichever version persisted them, and these events
// were named "run.*" before the rename to the documented "release.*" form. A deserializer
// that only knew the new spelling would fail to reconstruct every event already on disk
// or in Postgres — an audit trail that cannot read its own history, which is a worse
// outcome than the naming inconsistency the rename fixed. So both spellings resolve here,
// in one place, rather than each deserializer growing a second case per event.
func CanonicalEventName(name string) string {
	if suffix, ok := strings.CutPrefix(name, "run."); ok {
		return "release." + suffix
	}
	return name
}

func (e *RunCreatedEvent) EventName() string     { return "release.created" }
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

func (e *StateTransitionedEvent) EventName() string     { return "release.state_transitioned" }
func (e *StateTransitionedEvent) OccurredAt() time.Time { return e.At }

// RunApprovedEvent is emitted when a run is approved.
type RunApprovedEvent struct {
	RunID        RunID
	PlanHash     string
	ApprovedBy   string
	AutoApproved bool
	At           time.Time
}

func (e *RunApprovedEvent) EventName() string     { return "release.approved" }
func (e *RunApprovedEvent) OccurredAt() time.Time { return e.At }

// StepCompletedEvent is emitted when a publishing step completes.
type StepCompletedEvent struct {
	RunID    RunID
	StepName string
	Success  bool
	Error    string
	At       time.Time
}

func (e *StepCompletedEvent) EventName() string     { return "release.step_completed" }
func (e *StepCompletedEvent) OccurredAt() time.Time { return e.At }

// RunPublishedEvent is emitted when a run is successfully published.
type RunPublishedEvent struct {
	RunID   RunID
	Version version.SemanticVersion
	At      time.Time
}

func (e *RunPublishedEvent) EventName() string     { return "release.published" }
func (e *RunPublishedEvent) OccurredAt() time.Time { return e.At }

// RunFailedEvent is emitted when a run fails.
type RunFailedEvent struct {
	RunID  RunID
	Reason string
	At     time.Time

	// Version is the version this run was working toward, empty if it failed before one
	// was calculated.
	//
	// Carried on the event because a subscriber in another process has nothing else to
	// read it from. The outcome tracker caches the version from RunVersionedEvent, but
	// the CLI runs one command per process: bump raises that event and exits, so a
	// failure recorded later had no version at all and the failed release could not be
	// tied to the version that failed — which is half of what change failure rate is
	// computed from.
	Version string
}

func (e *RunFailedEvent) EventName() string     { return "release.failed" }
func (e *RunFailedEvent) OccurredAt() time.Time { return e.At }

// RunCanceledEvent is emitted when a run is canceled.
type RunCanceledEvent struct {
	RunID  RunID
	Reason string
	By     string
	At     time.Time

	// Version is the version this run was working toward, empty if it was canceled
	// before one was calculated. Carried for the same reason as RunFailedEvent.Version:
	// the process that raises this event is not the one that calculated the version.
	Version string
}

func (e *RunCanceledEvent) EventName() string     { return "release.canceled" }
func (e *RunCanceledEvent) OccurredAt() time.Time { return e.At }

// RunVersionedEvent is emitted when a version is applied to the run.
type RunVersionedEvent struct {
	RunID       RunID
	VersionNext version.SemanticVersion
	BumpKind    BumpKind
	TagName     string
	Actor       string
	At          time.Time
}

func (e *RunVersionedEvent) EventName() string     { return "release.versioned" }
func (e *RunVersionedEvent) OccurredAt() time.Time { return e.At }

// RunRetriedEvent is emitted when a failed run is retried.
type RunRetriedEvent struct {
	RunID RunID
	By    string
	At    time.Time
}

func (e *RunRetriedEvent) EventName() string     { return "release.retried" }
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

func (e *RunPlannedEvent) EventName() string     { return "release.planned" }
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

func (e *RunNotesGeneratedEvent) EventName() string     { return "release.notes_generated" }
func (e *RunNotesGeneratedEvent) OccurredAt() time.Time { return e.At }

// RunNotesUpdatedEvent is emitted when release notes are manually updated.
type RunNotesUpdatedEvent struct {
	RunID       RunID
	NotesLength int
	Actor       string
	At          time.Time
}

func (e *RunNotesUpdatedEvent) EventName() string     { return "release.notes_updated" }
func (e *RunNotesUpdatedEvent) OccurredAt() time.Time { return e.At }

// RunPublishingStartedEvent is emitted when publishing begins.
type RunPublishingStartedEvent struct {
	RunID    RunID
	Steps    []string
	PlanHash string
	Actor    string
	At       time.Time
}

func (e *RunPublishingStartedEvent) EventName() string     { return "release.publishing_started" }
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

func (e *PluginExecutedEvent) EventName() string     { return "release.plugin_executed" }
func (e *PluginExecutedEvent) OccurredAt() time.Time { return e.At }

// TagPushModeDetectedEvent is emitted when a run is created in tag-push mode.
// Tag-push mode occurs when HEAD is already tagged, allowing the workflow
// to skip directly to versioned state without running the bump command.
type TagPushModeDetectedEvent struct {
	RunID       RunID
	TagName     string
	VersionNext version.SemanticVersion
	Actor       string
	At          time.Time
}

func (e *TagPushModeDetectedEvent) EventName() string     { return "release.tag_push_mode_detected" }
func (e *TagPushModeDetectedEvent) OccurredAt() time.Time { return e.At }

// AggregateID returns the aggregate ID for events that need it.
func (e *RunCreatedEvent) AggregateID() RunID           { return e.RunID }
func (e *StateTransitionedEvent) AggregateID() RunID    { return e.RunID }
func (e *RunApprovedEvent) AggregateID() RunID          { return e.RunID }
func (e *StepCompletedEvent) AggregateID() RunID        { return e.RunID }
func (e *RunPublishedEvent) AggregateID() RunID         { return e.RunID }
func (e *RunFailedEvent) AggregateID() RunID            { return e.RunID }
func (e *RunCanceledEvent) AggregateID() RunID          { return e.RunID }
func (e *RunVersionedEvent) AggregateID() RunID         { return e.RunID }
func (e *RunRetriedEvent) AggregateID() RunID           { return e.RunID }
func (e *RunPlannedEvent) AggregateID() RunID           { return e.RunID }
func (e *RunNotesGeneratedEvent) AggregateID() RunID    { return e.RunID }
func (e *RunNotesUpdatedEvent) AggregateID() RunID      { return e.RunID }
func (e *RunPublishingStartedEvent) AggregateID() RunID { return e.RunID }
func (e *PluginExecutedEvent) AggregateID() RunID       { return e.RunID }
func (e *TagPushModeDetectedEvent) AggregateID() RunID  { return e.RunID }
