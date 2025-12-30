package websocket

import (
	"context"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
)

// EventBroadcaster implements EventPublisher and broadcasts domain events
// to connected WebSocket clients.
type EventBroadcaster struct {
	hub *Hub
}

// NewEventBroadcaster creates a new event broadcaster.
func NewEventBroadcaster(hub *Hub) *EventBroadcaster {
	return &EventBroadcaster{hub: hub}
}

// Publish broadcasts domain events to all connected WebSocket clients.
func (b *EventBroadcaster) Publish(ctx context.Context, events ...domain.DomainEvent) error {
	for _, event := range events {
		msg := b.eventToMessage(event)
		b.hub.Broadcast(msg)
	}
	return nil
}

// PublishAsync broadcasts events asynchronously (for AsyncEventPublisher compatibility).
func (b *EventBroadcaster) PublishAsync(ctx context.Context, events ...domain.DomainEvent) {
	go func() {
		for _, event := range events {
			msg := b.eventToMessage(event)
			b.hub.Broadcast(msg)
		}
	}()
}

// eventToMessage converts a domain event to a WebSocket message.
func (b *EventBroadcaster) eventToMessage(event domain.DomainEvent) Message {
	payload := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	// Extract common fields and event-specific data
	switch e := event.(type) {
	case *domain.RunCreatedEvent:
		payload["run_id"] = string(e.RunID)
		payload["repo_id"] = e.RepoID
		payload["head_sha"] = string(e.HeadSHA)
		payload["created_at"] = e.At.Format(time.RFC3339)
		return Message{Type: "release.created", Payload: payload}

	case *domain.StateTransitionedEvent:
		payload["run_id"] = string(e.RunID)
		payload["from_state"] = string(e.From)
		payload["to_state"] = string(e.To)
		payload["event"] = e.Event
		payload["actor"] = e.Actor
		payload["transitioned_at"] = e.At.Format(time.RFC3339)
		return Message{Type: "release.state_changed", Payload: payload}

	case *domain.RunVersionedEvent:
		payload["run_id"] = string(e.RunID)
		payload["version_next"] = e.VersionNext.String()
		payload["bump_kind"] = string(e.BumpKind)
		payload["tag_name"] = e.TagName
		payload["actor"] = e.Actor
		payload["versioned_at"] = e.At.Format(time.RFC3339)
		return Message{Type: "release.versioned", Payload: payload}

	case *domain.RunApprovedEvent:
		payload["run_id"] = string(e.RunID)
		payload["plan_hash"] = e.PlanHash
		payload["approved_by"] = e.ApprovedBy
		payload["auto_approved"] = e.AutoApproved
		payload["approved_at"] = e.At.Format(time.RFC3339)
		return Message{Type: "release.approved", Payload: payload}

	case *domain.RunPublishedEvent:
		payload["run_id"] = string(e.RunID)
		payload["version"] = e.Version.String()
		payload["published_at"] = e.At.Format(time.RFC3339)
		return Message{Type: "release.published", Payload: payload}

	case *domain.RunFailedEvent:
		payload["run_id"] = string(e.RunID)
		payload["reason"] = e.Reason
		payload["failed_at"] = e.At.Format(time.RFC3339)
		return Message{Type: "release.failed", Payload: payload}

	case *domain.RunCanceledEvent:
		payload["run_id"] = string(e.RunID)
		payload["reason"] = e.Reason
		payload["canceled_by"] = e.By
		payload["canceled_at"] = e.At.Format(time.RFC3339)
		return Message{Type: "release.canceled", Payload: payload}

	case *domain.RunRetriedEvent:
		payload["run_id"] = string(e.RunID)
		payload["retried_by"] = e.By
		payload["retried_at"] = e.At.Format(time.RFC3339)
		return Message{Type: "release.retried", Payload: payload}

	case *domain.StepCompletedEvent:
		payload["run_id"] = string(e.RunID)
		payload["step_name"] = e.StepName
		payload["success"] = e.Success
		if e.Error != "" {
			payload["error"] = e.Error
		}
		payload["completed_at"] = e.At.Format(time.RFC3339)
		return Message{Type: "release.step_completed", Payload: payload}

	case *domain.PluginExecutedEvent:
		payload["run_id"] = string(e.RunID)
		payload["plugin_name"] = e.PluginName
		payload["hook"] = e.Hook
		payload["success"] = e.Success
		payload["message"] = e.Message
		payload["duration_ms"] = e.Duration.Milliseconds()
		payload["executed_at"] = e.At.Format(time.RFC3339)
		return Message{Type: "release.plugin_executed", Payload: payload}

	case *domain.RunNotesUpdatedEvent:
		payload["run_id"] = string(e.RunID)
		payload["notes_length"] = e.NotesLength
		payload["actor"] = e.Actor
		payload["updated_at"] = e.At.Format(time.RFC3339)
		return Message{Type: "release.notes_updated", Payload: payload}

	default:
		// Generic event handling
		payload["event_name"] = event.EventName()
		return Message{Type: "release.event", Payload: payload}
	}
}
