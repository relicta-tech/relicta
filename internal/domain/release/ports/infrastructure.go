// Package ports defines the interfaces (ports) for the release governance bounded context.
package ports

import (
	"context"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// Clock provides time-related functionality.
// This abstraction enables testing with controlled time.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
}

// RealClock implements Clock using the system time.
type RealClock struct{}

// Now returns the current system time.
func (RealClock) Now() time.Time {
	return time.Now()
}

// IdentityProvider provides actor identity information.
type IdentityProvider interface {
	// CurrentActor returns the current actor's identity.
	CurrentActor(ctx context.Context) (ActorInfo, error)
}

// ActorInfo contains information about the current actor.
type ActorInfo struct {
	Type domain.ActorType
	ID   string
	Name string
}

// EventPublisher publishes domain events.
type EventPublisher interface {
	// Publish publishes one or more domain events.
	Publish(ctx context.Context, events ...domain.DomainEvent) error
}

// StateExporter exports state machine definitions and states.
type StateExporter interface {
	// ExportMachineJSON exports the state machine definition as XState JSON.
	ExportMachineJSON(run *domain.ReleaseRun) ([]byte, error)

	// ExportStateJSON exports the current state snapshot as JSON.
	ExportStateJSON(run *domain.ReleaseRun) ([]byte, error)
}

// VersionWriter writes version information to files in the repository.
type VersionWriter interface {
	// WriteVersion writes the version to configured files (VERSION, package.json, etc.)
	WriteVersion(ctx context.Context, ver version.SemanticVersion) error

	// WriteChangelog writes or updates the changelog file.
	WriteChangelog(ctx context.Context, ver version.SemanticVersion, notes string) error
}

// UnitOfWork provides transactional boundaries for domain operations.
// This enables atomic commits of aggregate changes with their domain events.
type UnitOfWork interface {
	// Begin starts a new unit of work.
	Begin(ctx context.Context) (UnitOfWorkContext, error)
}

// UnitOfWorkContext represents an active unit of work.
type UnitOfWorkContext interface {
	// Repository returns the release run repository scoped to this unit of work.
	Repository() ReleaseRunRepository

	// RegisterForEventPublication registers domain events to be published on commit.
	RegisterForEventPublication(events ...domain.DomainEvent)

	// Commit commits all changes and publishes registered events.
	Commit(ctx context.Context) error

	// Rollback rolls back all changes.
	Rollback(ctx context.Context) error
}

// AsyncEventPublisher extends EventPublisher with async capabilities.
type AsyncEventPublisher interface {
	EventPublisher

	// PublishAsync publishes events asynchronously.
	PublishAsync(ctx context.Context, events ...domain.DomainEvent)

	// Flush waits for all pending async publications to complete.
	Flush(ctx context.Context) error
}

// EventHandler handles domain events.
type EventHandler interface {
	// Handle processes a domain event.
	Handle(ctx context.Context, event domain.DomainEvent) error

	// Handles returns the event types this handler processes.
	Handles() []string
}

// EventDispatcher dispatches domain events to registered handlers.
type EventDispatcher interface {
	// Register registers a handler for event types.
	Register(handler EventHandler)

	// Dispatch dispatches an event to all registered handlers.
	Dispatch(ctx context.Context, event domain.DomainEvent) error
}
