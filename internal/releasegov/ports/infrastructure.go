// Package ports defines the interfaces (ports) for the release governance bounded context.
package ports

import (
	"context"
	"time"

	"github.com/relicta-tech/relicta/internal/releasegov/domain"
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
