// Package release provides domain types for release management.
package release

import (
	"time"

	"github.com/felixgeelhaar/release-pilot/internal/domain/version"
)

// DomainEvent represents an event that occurred in the domain.
type DomainEvent interface {
	// EventName returns the name of the event.
	EventName() string
	// OccurredAt returns when the event occurred.
	OccurredAt() time.Time
	// AggregateID returns the ID of the aggregate this event belongs to.
	AggregateID() ReleaseID
}

// BaseEvent contains common fields for all domain events.
type BaseEvent struct {
	occurredAt  time.Time
	aggregateID ReleaseID
}

// OccurredAt returns when the event occurred.
func (e BaseEvent) OccurredAt() time.Time {
	return e.occurredAt
}

// AggregateID returns the aggregate ID.
func (e BaseEvent) AggregateID() ReleaseID {
	return e.aggregateID
}

// ReleaseInitializedEvent is raised when a release is initialized.
type ReleaseInitializedEvent struct {
	BaseEvent
	Branch     string
	Repository string
}

// EventName returns the event name.
func (e ReleaseInitializedEvent) EventName() string {
	return "release.initialized"
}

// NewReleaseInitializedEvent creates a new ReleaseInitializedEvent.
func NewReleaseInitializedEvent(id ReleaseID, branch, repo string) ReleaseInitializedEvent {
	return ReleaseInitializedEvent{
		BaseEvent: BaseEvent{
			occurredAt:  time.Now(),
			aggregateID: id,
		},
		Branch:     branch,
		Repository: repo,
	}
}

// ReleasePlannedEvent is raised when a release plan is created.
type ReleasePlannedEvent struct {
	BaseEvent
	CurrentVersion version.SemanticVersion
	NextVersion    version.SemanticVersion
	ReleaseType    string
	CommitCount    int
}

// EventName returns the event name.
func (e ReleasePlannedEvent) EventName() string {
	return "release.planned"
}

// NewReleasePlannedEvent creates a new ReleasePlannedEvent.
func NewReleasePlannedEvent(id ReleaseID, current, next version.SemanticVersion, releaseType string, commitCount int) ReleasePlannedEvent {
	return ReleasePlannedEvent{
		BaseEvent: BaseEvent{
			occurredAt:  time.Now(),
			aggregateID: id,
		},
		CurrentVersion: current,
		NextVersion:    next,
		ReleaseType:    releaseType,
		CommitCount:    commitCount,
	}
}

// ReleaseVersionedEvent is raised when a release version is set.
type ReleaseVersionedEvent struct {
	BaseEvent
	Version version.SemanticVersion
	TagName string
}

// EventName returns the event name.
func (e ReleaseVersionedEvent) EventName() string {
	return "release.versioned"
}

// NewReleaseVersionedEvent creates a new ReleaseVersionedEvent.
func NewReleaseVersionedEvent(id ReleaseID, ver version.SemanticVersion, tagName string) ReleaseVersionedEvent {
	return ReleaseVersionedEvent{
		BaseEvent: BaseEvent{
			occurredAt:  time.Now(),
			aggregateID: id,
		},
		Version: ver,
		TagName: tagName,
	}
}

// ReleaseNotesGeneratedEvent is raised when release notes are generated.
type ReleaseNotesGeneratedEvent struct {
	BaseEvent
	ChangelogUpdated bool
	NotesLength      int
}

// EventName returns the event name.
func (e ReleaseNotesGeneratedEvent) EventName() string {
	return "release.notes_generated"
}

// NewReleaseNotesGeneratedEvent creates a new ReleaseNotesGeneratedEvent.
func NewReleaseNotesGeneratedEvent(id ReleaseID, changelogUpdated bool, notesLen int) ReleaseNotesGeneratedEvent {
	return ReleaseNotesGeneratedEvent{
		BaseEvent: BaseEvent{
			occurredAt:  time.Now(),
			aggregateID: id,
		},
		ChangelogUpdated: changelogUpdated,
		NotesLength:      notesLen,
	}
}

// ReleaseNotesUpdatedEvent is raised when release notes are manually updated.
type ReleaseNotesUpdatedEvent struct {
	BaseEvent
	NotesLength int
}

// EventName returns the event name.
func (e ReleaseNotesUpdatedEvent) EventName() string {
	return "release.notes_updated"
}

// NewReleaseNotesUpdatedEvent creates a new ReleaseNotesUpdatedEvent.
func NewReleaseNotesUpdatedEvent(id ReleaseID, notesLen int) ReleaseNotesUpdatedEvent {
	return ReleaseNotesUpdatedEvent{
		BaseEvent: BaseEvent{
			occurredAt:  time.Now(),
			aggregateID: id,
		},
		NotesLength: notesLen,
	}
}

// ReleaseApprovedEvent is raised when a release is approved.
type ReleaseApprovedEvent struct {
	BaseEvent
	ApprovedBy string
}

// EventName returns the event name.
func (e ReleaseApprovedEvent) EventName() string {
	return "release.approved"
}

// NewReleaseApprovedEvent creates a new ReleaseApprovedEvent.
func NewReleaseApprovedEvent(id ReleaseID, approvedBy string) ReleaseApprovedEvent {
	return ReleaseApprovedEvent{
		BaseEvent: BaseEvent{
			occurredAt:  time.Now(),
			aggregateID: id,
		},
		ApprovedBy: approvedBy,
	}
}

// ReleasePublishingStartedEvent is raised when publishing starts.
type ReleasePublishingStartedEvent struct {
	BaseEvent
	Plugins []string
}

// EventName returns the event name.
func (e ReleasePublishingStartedEvent) EventName() string {
	return "release.publishing_started"
}

// NewReleasePublishingStartedEvent creates a new ReleasePublishingStartedEvent.
func NewReleasePublishingStartedEvent(id ReleaseID, plugins []string) ReleasePublishingStartedEvent {
	return ReleasePublishingStartedEvent{
		BaseEvent: BaseEvent{
			occurredAt:  time.Now(),
			aggregateID: id,
		},
		Plugins: plugins,
	}
}

// ReleasePublishedEvent is raised when a release is successfully published.
type ReleasePublishedEvent struct {
	BaseEvent
	Version    version.SemanticVersion
	TagName    string
	ReleaseURL string
}

// EventName returns the event name.
func (e ReleasePublishedEvent) EventName() string {
	return "release.published"
}

// NewReleasePublishedEvent creates a new ReleasePublishedEvent.
func NewReleasePublishedEvent(id ReleaseID, ver version.SemanticVersion, tagName, url string) ReleasePublishedEvent {
	return ReleasePublishedEvent{
		BaseEvent: BaseEvent{
			occurredAt:  time.Now(),
			aggregateID: id,
		},
		Version:    ver,
		TagName:    tagName,
		ReleaseURL: url,
	}
}

// ReleaseFailedEvent is raised when a release fails.
type ReleaseFailedEvent struct {
	BaseEvent
	Reason        string
	FailedAt      ReleaseState
	IsRecoverable bool
}

// EventName returns the event name.
func (e ReleaseFailedEvent) EventName() string {
	return "release.failed"
}

// NewReleaseFailedEvent creates a new ReleaseFailedEvent.
func NewReleaseFailedEvent(id ReleaseID, reason string, failedAt ReleaseState, recoverable bool) ReleaseFailedEvent {
	return ReleaseFailedEvent{
		BaseEvent: BaseEvent{
			occurredAt:  time.Now(),
			aggregateID: id,
		},
		Reason:        reason,
		FailedAt:      failedAt,
		IsRecoverable: recoverable,
	}
}

// ReleaseCanceledEvent is raised when a release is canceled.
type ReleaseCanceledEvent struct {
	BaseEvent
	Reason     string
	CanceledBy string
}

// EventName returns the event name.
func (e ReleaseCanceledEvent) EventName() string {
	return "release.canceled"
}

// NewReleaseCanceledEvent creates a new ReleaseCanceledEvent.
func NewReleaseCanceledEvent(id ReleaseID, reason, canceledBy string) ReleaseCanceledEvent {
	return ReleaseCanceledEvent{
		BaseEvent: BaseEvent{
			occurredAt:  time.Now(),
			aggregateID: id,
		},
		Reason:     reason,
		CanceledBy: canceledBy,
	}
}

// PluginExecutedEvent is raised when a plugin completes execution.
type PluginExecutedEvent struct {
	BaseEvent
	PluginName string
	Hook       string
	Success    bool
	Message    string
	Duration   time.Duration
}

// EventName returns the event name.
func (e PluginExecutedEvent) EventName() string {
	return "release.plugin_executed"
}

// NewPluginExecutedEvent creates a new PluginExecutedEvent.
func NewPluginExecutedEvent(id ReleaseID, pluginName, hook string, success bool, msg string, duration time.Duration) PluginExecutedEvent {
	return PluginExecutedEvent{
		BaseEvent: BaseEvent{
			occurredAt:  time.Now(),
			aggregateID: id,
		},
		PluginName: pluginName,
		Hook:       hook,
		Success:    success,
		Message:    msg,
		Duration:   duration,
	}
}
