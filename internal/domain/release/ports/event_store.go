// Package ports defines the interfaces (ports) for the release governance bounded context.
package ports

import (
	"context"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
)

// EventStore defines the interface for persisting and retrieving domain events.
// This provides a complete audit trail for forensic analysis and event sourcing.
type EventStore interface {
	// Append appends events to the event stream for a release run.
	// Events are stored in order and immutable once persisted.
	Append(ctx context.Context, runID domain.RunID, events []domain.DomainEvent) error

	// LoadEvents retrieves all events for a release run in order.
	LoadEvents(ctx context.Context, runID domain.RunID) ([]domain.DomainEvent, error)

	// LoadEventsSince retrieves events after the given timestamp.
	LoadEventsSince(ctx context.Context, runID domain.RunID, since time.Time) ([]domain.DomainEvent, error)

	// LoadAllEvents retrieves all events for a repository (for auditing).
	LoadAllEvents(ctx context.Context, repoRoot string) ([]domain.DomainEvent, error)
}

// StoredEvent represents a persisted domain event with metadata.
type StoredEvent struct {
	ID          string             `json:"id"`
	RunID       domain.RunID       `json:"run_id"`
	EventName   string             `json:"event_name"`
	OccurredAt  time.Time          `json:"occurred_at"`
	StoredAt    time.Time          `json:"stored_at"`
	SequenceNum int64              `json:"sequence_num"`
	Payload     domain.DomainEvent `json:"-"` // Actual event (serialized separately)
	PayloadJSON string             `json:"payload"`
}
