package analytics

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Service captures and queries analytics events.
type Service struct {
	store Store
	clock func() time.Time
}

// NewService creates a new analytics service backed by the given store.
func NewService(store Store) *Service {
	return &Service{
		store: store,
		clock: time.Now,
	}
}

// WithClock sets a custom clock function for testing.
func (s *Service) WithClock(clock func() time.Time) *Service {
	s.clock = clock
	return s
}

// Capture records an analytics event with the given type and payload.
func (s *Service) Capture(ctx context.Context, eventType EventType, releaseID string, payload any) error {
	if !eventType.Valid() {
		return fmt.Errorf("invalid event type: %s", eventType)
	}

	raw, err := MarshalPayload(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	event := Event{
		ID:        generateID(),
		Timestamp: s.clock().UTC(),
		Type:      eventType,
		ReleaseID: releaseID,
		Payload:   raw,
	}

	if err := s.store.Append(ctx, event); err != nil {
		return fmt.Errorf("failed to store event: %w", err)
	}

	return nil
}

// Query returns events matching the filter.
func (s *Service) Query(ctx context.Context, filter QueryFilter) ([]Event, error) {
	return s.store.Query(ctx, filter)
}

// generateID creates a random hex ID for events.
func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
