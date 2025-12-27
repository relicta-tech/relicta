package domain

import (
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestRunCreatedEvent(t *testing.T) {
	now := time.Now()
	event := &RunCreatedEvent{
		RunID:   "run-123",
		RepoID:  "github.com/test/repo",
		HeadSHA: "abc123",
		At:      now,
	}

	if event.EventName() != "run.created" {
		t.Errorf("EventName() = %v, want run.created", event.EventName())
	}
	if event.OccurredAt() != now {
		t.Errorf("OccurredAt() = %v, want %v", event.OccurredAt(), now)
	}
	if event.AggregateID() != "run-123" {
		t.Errorf("AggregateID() = %v, want run-123", event.AggregateID())
	}
}

func TestStateTransitionedEvent(t *testing.T) {
	now := time.Now()
	event := &StateTransitionedEvent{
		RunID: "run-123",
		From:  StateDraft,
		To:    StatePlanned,
		Event: "PLAN",
		Actor: "user",
		At:    now,
	}

	if event.EventName() != "run.state_transitioned" {
		t.Errorf("EventName() = %v, want run.state_transitioned", event.EventName())
	}
	if event.OccurredAt() != now {
		t.Errorf("OccurredAt() = %v, want %v", event.OccurredAt(), now)
	}
	if event.AggregateID() != "run-123" {
		t.Errorf("AggregateID() = %v, want run-123", event.AggregateID())
	}
}

func TestRunApprovedEvent(t *testing.T) {
	now := time.Now()
	event := &RunApprovedEvent{
		RunID:        "run-123",
		PlanHash:     "hash123",
		ApprovedBy:   "approver",
		AutoApproved: false,
		At:           now,
	}

	if event.EventName() != "run.approved" {
		t.Errorf("EventName() = %v, want run.approved", event.EventName())
	}
	if event.OccurredAt() != now {
		t.Errorf("OccurredAt() = %v, want %v", event.OccurredAt(), now)
	}
	if event.AggregateID() != "run-123" {
		t.Errorf("AggregateID() = %v, want run-123", event.AggregateID())
	}
}

func TestStepCompletedEvent(t *testing.T) {
	now := time.Now()
	event := &StepCompletedEvent{
		RunID:    "run-123",
		StepName: "tag",
		Success:  true,
		Error:    "",
		At:       now,
	}

	if event.EventName() != "run.step_completed" {
		t.Errorf("EventName() = %v, want run.step_completed", event.EventName())
	}
	if event.OccurredAt() != now {
		t.Errorf("OccurredAt() = %v, want %v", event.OccurredAt(), now)
	}
	if event.AggregateID() != "run-123" {
		t.Errorf("AggregateID() = %v, want run-123", event.AggregateID())
	}
}

func TestRunPublishedEvent(t *testing.T) {
	now := time.Now()
	event := &RunPublishedEvent{
		RunID:   "run-123",
		Version: version.MustParse("1.0.0"),
		At:      now,
	}

	if event.EventName() != "run.published" {
		t.Errorf("EventName() = %v, want run.published", event.EventName())
	}
	if event.OccurredAt() != now {
		t.Errorf("OccurredAt() = %v, want %v", event.OccurredAt(), now)
	}
	if event.AggregateID() != "run-123" {
		t.Errorf("AggregateID() = %v, want run-123", event.AggregateID())
	}
}

func TestRunFailedEvent(t *testing.T) {
	now := time.Now()
	event := &RunFailedEvent{
		RunID:  "run-123",
		Reason: "step failed",
		At:     now,
	}

	if event.EventName() != "run.failed" {
		t.Errorf("EventName() = %v, want run.failed", event.EventName())
	}
	if event.OccurredAt() != now {
		t.Errorf("OccurredAt() = %v, want %v", event.OccurredAt(), now)
	}
	if event.AggregateID() != "run-123" {
		t.Errorf("AggregateID() = %v, want run-123", event.AggregateID())
	}
}

func TestRunCanceledEvent(t *testing.T) {
	now := time.Now()
	event := &RunCanceledEvent{
		RunID:  "run-123",
		Reason: "user requested",
		By:     "user",
		At:     now,
	}

	if event.EventName() != "run.canceled" {
		t.Errorf("EventName() = %v, want run.canceled", event.EventName())
	}
	if event.OccurredAt() != now {
		t.Errorf("OccurredAt() = %v, want %v", event.OccurredAt(), now)
	}
	if event.AggregateID() != "run-123" {
		t.Errorf("AggregateID() = %v, want run-123", event.AggregateID())
	}
}

func TestRunVersionedEvent(t *testing.T) {
	now := time.Now()
	event := &RunVersionedEvent{
		RunID:       "run-123",
		VersionNext: version.MustParse("1.1.0"),
		BumpKind:    BumpMinor,
		TagName:     "v1.1.0",
		Actor:       "user",
		At:          now,
	}

	if event.EventName() != "run.versioned" {
		t.Errorf("EventName() = %v, want run.versioned", event.EventName())
	}
	if event.OccurredAt() != now {
		t.Errorf("OccurredAt() = %v, want %v", event.OccurredAt(), now)
	}
	if event.AggregateID() != "run-123" {
		t.Errorf("AggregateID() = %v, want run-123", event.AggregateID())
	}
}

func TestRunRetriedEvent(t *testing.T) {
	now := time.Now()
	event := &RunRetriedEvent{
		RunID: "run-123",
		By:    "user",
		At:    now,
	}

	if event.EventName() != "run.retried" {
		t.Errorf("EventName() = %v, want run.retried", event.EventName())
	}
	if event.OccurredAt() != now {
		t.Errorf("OccurredAt() = %v, want %v", event.OccurredAt(), now)
	}
	if event.AggregateID() != "run-123" {
		t.Errorf("AggregateID() = %v, want run-123", event.AggregateID())
	}
}

func TestRunPlannedEvent(t *testing.T) {
	now := time.Now()
	event := &RunPlannedEvent{
		RunID:          "run-123",
		VersionCurrent: version.MustParse("1.0.0"),
		VersionNext:    version.MustParse("1.1.0"),
		BumpKind:       BumpMinor,
		CommitCount:    5,
		RiskScore:      0.3,
		Actor:          "user",
		At:             now,
	}

	if event.EventName() != "run.planned" {
		t.Errorf("EventName() = %v, want run.planned", event.EventName())
	}
	if event.OccurredAt() != now {
		t.Errorf("OccurredAt() = %v, want %v", event.OccurredAt(), now)
	}
	if event.AggregateID() != "run-123" {
		t.Errorf("AggregateID() = %v, want run-123", event.AggregateID())
	}
}

func TestRunNotesGeneratedEvent(t *testing.T) {
	now := time.Now()
	event := &RunNotesGeneratedEvent{
		RunID:       "run-123",
		NotesLength: 500,
		Provider:    "openai",
		Model:       "gpt-4",
		Actor:       "user",
		At:          now,
	}

	if event.EventName() != "run.notes_generated" {
		t.Errorf("EventName() = %v, want run.notes_generated", event.EventName())
	}
	if event.OccurredAt() != now {
		t.Errorf("OccurredAt() = %v, want %v", event.OccurredAt(), now)
	}
	if event.AggregateID() != "run-123" {
		t.Errorf("AggregateID() = %v, want run-123", event.AggregateID())
	}
}

func TestRunNotesUpdatedEvent(t *testing.T) {
	now := time.Now()
	event := &RunNotesUpdatedEvent{
		RunID:       "run-123",
		NotesLength: 600,
		Actor:       "editor",
		At:          now,
	}

	if event.EventName() != "run.notes_updated" {
		t.Errorf("EventName() = %v, want run.notes_updated", event.EventName())
	}
	if event.OccurredAt() != now {
		t.Errorf("OccurredAt() = %v, want %v", event.OccurredAt(), now)
	}
	if event.AggregateID() != "run-123" {
		t.Errorf("AggregateID() = %v, want run-123", event.AggregateID())
	}
}

func TestRunPublishingStartedEvent(t *testing.T) {
	now := time.Now()
	event := &RunPublishingStartedEvent{
		RunID:    "run-123",
		Steps:    []string{"tag", "build", "notify"},
		PlanHash: "hash123",
		Actor:    "user",
		At:       now,
	}

	if event.EventName() != "run.publishing_started" {
		t.Errorf("EventName() = %v, want run.publishing_started", event.EventName())
	}
	if event.OccurredAt() != now {
		t.Errorf("OccurredAt() = %v, want %v", event.OccurredAt(), now)
	}
	if event.AggregateID() != "run-123" {
		t.Errorf("AggregateID() = %v, want run-123", event.AggregateID())
	}
}

func TestPluginExecutedEvent(t *testing.T) {
	now := time.Now()
	event := &PluginExecutedEvent{
		RunID:      "run-123",
		PluginName: "github",
		Hook:       "PostPublish",
		Success:    true,
		Message:    "Created release",
		Duration:   2 * time.Second,
		At:         now,
	}

	if event.EventName() != "run.plugin_executed" {
		t.Errorf("EventName() = %v, want run.plugin_executed", event.EventName())
	}
	if event.OccurredAt() != now {
		t.Errorf("OccurredAt() = %v, want %v", event.OccurredAt(), now)
	}
	if event.AggregateID() != "run-123" {
		t.Errorf("AggregateID() = %v, want run-123", event.AggregateID())
	}
}

// Test that all events implement DomainEvent interface
func TestAllEventsImplementDomainEvent(t *testing.T) {
	now := time.Now()
	events := []DomainEvent{
		&RunCreatedEvent{RunID: "run-1", At: now},
		&StateTransitionedEvent{RunID: "run-1", At: now},
		&RunApprovedEvent{RunID: "run-1", At: now},
		&StepCompletedEvent{RunID: "run-1", At: now},
		&RunPublishedEvent{RunID: "run-1", At: now},
		&RunFailedEvent{RunID: "run-1", At: now},
		&RunCanceledEvent{RunID: "run-1", At: now},
		&RunVersionedEvent{RunID: "run-1", At: now},
		&RunRetriedEvent{RunID: "run-1", At: now},
		&RunPlannedEvent{RunID: "run-1", At: now},
		&RunNotesGeneratedEvent{RunID: "run-1", At: now},
		&RunNotesUpdatedEvent{RunID: "run-1", At: now},
		&RunPublishingStartedEvent{RunID: "run-1", At: now},
		&PluginExecutedEvent{RunID: "run-1", At: now},
	}

	for _, event := range events {
		// All should have non-empty event name
		if event.EventName() == "" {
			t.Errorf("Event %T has empty EventName()", event)
		}
		// All should have the correct aggregate ID
		if event.AggregateID() != "run-1" {
			t.Errorf("Event %T AggregateID() = %v, want run-1", event, event.AggregateID())
		}
		// All should have the occurrence time
		if event.OccurredAt() != now {
			t.Errorf("Event %T OccurredAt() = %v, want %v", event, event.OccurredAt(), now)
		}
	}
}
