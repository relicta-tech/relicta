// Package persistence provides infrastructure implementations for data persistence.
package persistence

import (
	"context"
	"sync"

	"github.com/felixgeelhaar/release-pilot/internal/domain/release"
)

// InMemoryEventPublisher implements release.EventPublisher with in-memory storage.
// This is useful for testing and simple scenarios.
// For production, consider using a message broker like RabbitMQ or Kafka.
type InMemoryEventPublisher struct {
	mu       sync.RWMutex
	events   []release.DomainEvent
	handlers []EventHandler
}

// EventHandler is a function that handles domain events.
type EventHandler func(event release.DomainEvent)

// NewInMemoryEventPublisher creates a new in-memory event publisher.
func NewInMemoryEventPublisher() *InMemoryEventPublisher {
	return &InMemoryEventPublisher{
		events:   make([]release.DomainEvent, 0, 10), // Typical session produces ~10 events
		handlers: make([]EventHandler, 0, 2),         // Usually 1-2 handlers registered
	}
}

// Publish publishes domain events.
func (p *InMemoryEventPublisher) Publish(ctx context.Context, events ...release.DomainEvent) error {
	// Store events under lock
	p.mu.Lock()
	p.events = append(p.events, events...)
	// Copy handlers to avoid holding lock during handler execution
	handlers := make([]EventHandler, len(p.handlers))
	copy(handlers, p.handlers)
	p.mu.Unlock()

	// Notify handlers without holding the lock to prevent contention
	// This allows handlers to call back into the publisher if needed
	for _, event := range events {
		for _, handler := range handlers {
			handler(event)
		}
	}

	return nil
}

// Subscribe adds an event handler.
func (p *InMemoryEventPublisher) Subscribe(handler EventHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers = append(p.handlers, handler)
}

// GetEvents returns all published events.
func (p *InMemoryEventPublisher) GetEvents() []release.DomainEvent {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return append([]release.DomainEvent{}, p.events...)
}

// ClearEvents clears all stored events.
func (p *InMemoryEventPublisher) ClearEvents() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = p.events[:0] // Preserve capacity for reuse
}

// GetEventsByType returns events of a specific type.
func (p *InMemoryEventPublisher) GetEventsByType(eventName string) []release.DomainEvent {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []release.DomainEvent
	for _, event := range p.events {
		if event.EventName() == eventName {
			result = append(result, event)
		}
	}
	return result
}

// GetEventsByAggregateID returns events for a specific aggregate.
func (p *InMemoryEventPublisher) GetEventsByAggregateID(id release.ReleaseID) []release.DomainEvent {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []release.DomainEvent
	for _, event := range p.events {
		if event.AggregateID() == id {
			result = append(result, event)
		}
	}
	return result
}

// NoOpEventPublisher is a no-op implementation for when events are not needed.
type NoOpEventPublisher struct{}

// NewNoOpEventPublisher creates a new no-op event publisher.
func NewNoOpEventPublisher() *NoOpEventPublisher {
	return &NoOpEventPublisher{}
}

// Publish does nothing.
func (p *NoOpEventPublisher) Publish(ctx context.Context, events ...release.DomainEvent) error {
	return nil
}
