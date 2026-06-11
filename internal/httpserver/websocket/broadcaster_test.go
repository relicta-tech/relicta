package websocket

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
)

func TestEventBroadcaster_Publish(t *testing.T) {
	// Create hub and broadcaster (nil origins for test)
	hub := NewHub(nil)
	broadcaster := NewEventBroadcaster(hub)

	// Test that Publish doesn't panic with no clients
	err := broadcaster.Publish(context.Background(), &domain.RunCreatedEvent{
		RunID:   "test-run-1",
		RepoID:  "test/repo",
		HeadSHA: "abc123",
		At:      time.Now(),
	})

	if err != nil {
		t.Errorf("Publish returned error: %v", err)
	}
}

func TestEventBroadcaster_PublishAsync(t *testing.T) {
	// Create hub and broadcaster (nil origins for test)
	hub := NewHub(nil)
	broadcaster := NewEventBroadcaster(hub)

	// Test that PublishAsync doesn't panic
	broadcaster.PublishAsync(context.Background(), &domain.StateTransitionedEvent{
		RunID: "test-run-1",
		From:  domain.StateDraft,
		To:    domain.StatePlanned,
		Event: "plan",
		Actor: "test-user",
		At:    time.Now(),
	})

	// Give async goroutine time to complete
	time.Sleep(10 * time.Millisecond)
}

func TestEventBroadcaster_EventToMessage(t *testing.T) {
	hub := NewHub(nil)
	broadcaster := NewEventBroadcaster(hub)

	tests := []struct {
		name         string
		event        domain.DomainEvent
		expectedType string
	}{
		{
			name: "RunCreatedEvent",
			event: &domain.RunCreatedEvent{
				RunID:   "run-1",
				RepoID:  "test/repo",
				HeadSHA: "abc123",
				At:      time.Now(),
			},
			expectedType: "release.created",
		},
		{
			name: "StateTransitionedEvent",
			event: &domain.StateTransitionedEvent{
				RunID: "run-1",
				From:  domain.StateDraft,
				To:    domain.StatePlanned,
				Event: "plan",
				Actor: "user",
				At:    time.Now(),
			},
			expectedType: "release.state_changed",
		},
		{
			name: "RunVersionedEvent",
			event: &domain.RunVersionedEvent{
				RunID:    "run-1",
				BumpKind: domain.BumpMinor,
				TagName:  "v1.0.0",
				Actor:    "user",
				At:       time.Now(),
			},
			expectedType: "release.versioned",
		},
		{
			name: "RunApprovedEvent",
			event: &domain.RunApprovedEvent{
				RunID:        "run-1",
				PlanHash:     "hash123",
				ApprovedBy:   "admin",
				AutoApproved: false,
				At:           time.Now(),
			},
			expectedType: "release.approved",
		},
		{
			name: "RunPublishedEvent",
			event: &domain.RunPublishedEvent{
				RunID: "run-1",
				At:    time.Now(),
			},
			expectedType: "release.published",
		},
		{
			name: "RunFailedEvent",
			event: &domain.RunFailedEvent{
				RunID:  "run-1",
				Reason: "test failure",
				At:     time.Now(),
			},
			expectedType: "release.failed",
		},
		{
			name: "RunCanceledEvent",
			event: &domain.RunCanceledEvent{
				RunID:  "run-1",
				Reason: "user canceled",
				By:     "user",
				At:     time.Now(),
			},
			expectedType: "release.canceled",
		},
		{
			name: "RunRetriedEvent",
			event: &domain.RunRetriedEvent{
				RunID: "run-1",
				By:    "user",
				At:    time.Now(),
			},
			expectedType: "release.retried",
		},
		{
			name: "StepCompletedEvent",
			event: &domain.StepCompletedEvent{
				RunID:    "run-1",
				StepName: "analyze",
				Success:  true,
				At:       time.Now(),
			},
			expectedType: "release.step_completed",
		},
		{
			name: "StepCompletedEvent with error",
			event: &domain.StepCompletedEvent{
				RunID:    "run-1",
				StepName: "tag",
				Success:  false,
				Error:    "tag creation failed",
				At:       time.Now(),
			},
			expectedType: "release.step_completed",
		},
		{
			name: "PluginExecutedEvent",
			event: &domain.PluginExecutedEvent{
				RunID:      "run-1",
				PluginName: "github",
				Hook:       "post_publish",
				Success:    true,
				Message:    "created release",
				Duration:   100 * time.Millisecond,
				At:         time.Now(),
			},
			expectedType: "release.plugin_executed",
		},
		{
			name: "RunNotesUpdatedEvent",
			event: &domain.RunNotesUpdatedEvent{
				RunID:       "run-1",
				NotesLength: 500,
				Actor:       "user",
				At:          time.Now(),
			},
			expectedType: "release.notes_updated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := broadcaster.eventToMessage(tt.event)
			if msg.Type != tt.expectedType {
				t.Errorf("expected message type %q, got %q", tt.expectedType, msg.Type)
			}
			// Payload is map[string]any
			payload, ok := msg.Payload.(map[string]any)
			if !ok {
				t.Error("expected payload to be map[string]any")
				return
			}
			// Check timestamp is present
			if _, ok := payload["timestamp"]; !ok {
				t.Error("expected timestamp in payload")
			}
		})
	}
}

func TestEventBroadcaster_StepCompletedWithError(t *testing.T) {
	hub := NewHub(nil)
	broadcaster := NewEventBroadcaster(hub)

	// Test step with error field set
	msg := broadcaster.eventToMessage(&domain.StepCompletedEvent{
		RunID:    "run-1",
		StepName: "tag",
		Success:  false,
		Error:    "tag creation failed",
		At:       time.Now(),
	})

	payload, ok := msg.Payload.(map[string]any)
	if !ok {
		t.Fatal("expected payload to be map[string]any")
	}

	errField, ok := payload["error"]
	if !ok {
		t.Error("expected error field in payload for failed step")
	}
	if errField != "tag creation failed" {
		t.Errorf("expected error 'tag creation failed', got %v", errField)
	}

	// Test step without error field
	msg2 := broadcaster.eventToMessage(&domain.StepCompletedEvent{
		RunID:    "run-1",
		StepName: "analyze",
		Success:  true,
		At:       time.Now(),
	})

	payload2, _ := msg2.Payload.(map[string]any)
	if _, ok := payload2["error"]; ok {
		t.Error("expected no error field in payload for successful step")
	}
}
