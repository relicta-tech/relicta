// Package adapters provides infrastructure implementations for the release governance domain.
package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
)

const (
	eventsDir        = ".relicta/events"
	eventsFileSuffix = ".events.jsonl"
)

// FileEventStore implements EventStore using append-only JSON lines files.
type FileEventStore struct {
	mu sync.Mutex
}

// NewFileEventStore creates a new file-based event store.
func NewFileEventStore() *FileEventStore {
	return &FileEventStore{}
}

// Ensure FileEventStore implements the interface.
var _ ports.EventStore = (*FileEventStore)(nil)

// eventsPath returns the path to the events directory for a repo.
func eventsPath(repoRoot string) string {
	return filepath.Join(repoRoot, eventsDir)
}

// eventFilePath returns the path to events file for a specific run.
func eventFilePath(repoRoot string, runID domain.RunID) string {
	return filepath.Join(eventsPath(repoRoot), string(runID)+eventsFileSuffix)
}

// ensureEventsDir creates the events directory if it doesn't exist.
func ensureEventsDir(repoRoot string) error {
	dir := eventsPath(repoRoot)
	return os.MkdirAll(dir, 0755)
}

// storedEventDTO is the serialized form of an event.
type storedEventDTO struct {
	ID          string          `json:"id"`
	RunID       string          `json:"run_id"`
	EventName   string          `json:"event_name"`
	OccurredAt  time.Time       `json:"occurred_at"`
	StoredAt    time.Time       `json:"stored_at"`
	SequenceNum int64           `json:"sequence_num"`
	Payload     json.RawMessage `json:"payload"`
}

// Append appends events to the event stream for a release run.
func (s *FileEventStore) Append(ctx context.Context, runID domain.RunID, events []domain.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// For file-based storage, we need the repo root from context
	repoRoot := getRepoRootFromContext(ctx)
	if repoRoot == "" {
		return fmt.Errorf("repo root not found in context")
	}

	if err := ensureEventsDir(repoRoot); err != nil {
		return fmt.Errorf("failed to ensure events directory: %w", err)
	}

	// Open file for append
	path := eventFilePath(repoRoot, runID)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open events file: %w", err)
	}

	// Get current sequence number
	seqNum := s.getNextSequence(path)

	encoder := json.NewEncoder(f)
	now := time.Now()

	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			_ = f.Close() // Best-effort cleanup
			return fmt.Errorf("failed to marshal event: %w", err)
		}

		dto := storedEventDTO{
			ID:          fmt.Sprintf("%s-%d", runID, seqNum),
			RunID:       string(runID),
			EventName:   event.EventName(),
			OccurredAt:  event.OccurredAt(),
			StoredAt:    now,
			SequenceNum: seqNum,
			Payload:     payload,
		}

		if err := encoder.Encode(dto); err != nil {
			_ = f.Close() // Best-effort cleanup
			return fmt.Errorf("failed to write event: %w", err)
		}
		seqNum++
	}

	// Ensure data is written to disk
	if err := f.Sync(); err != nil {
		_ = f.Close() // Best-effort cleanup
		return fmt.Errorf("failed to sync events file: %w", err)
	}

	// Close file and check for errors (potential data loss if Close fails after Sync)
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close events file: %w", err)
	}

	return nil
}

// LoadEvents retrieves all events for a release run in order.
func (s *FileEventStore) LoadEvents(ctx context.Context, runID domain.RunID) ([]domain.DomainEvent, error) {
	repoRoot := getRepoRootFromContext(ctx)
	if repoRoot == "" {
		return nil, fmt.Errorf("repo root not found in context")
	}

	path := eventFilePath(repoRoot, runID)
	dtos, err := s.loadEventsFromFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No events yet
		}
		return nil, err
	}

	return s.deserializeEvents(dtos)
}

// LoadEventsSince retrieves events after the given timestamp.
func (s *FileEventStore) LoadEventsSince(ctx context.Context, runID domain.RunID, since time.Time) ([]domain.DomainEvent, error) {
	repoRoot := getRepoRootFromContext(ctx)
	if repoRoot == "" {
		return nil, fmt.Errorf("repo root not found in context")
	}

	path := eventFilePath(repoRoot, runID)
	dtos, err := s.loadEventsFromFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Filter by timestamp
	var filtered []storedEventDTO
	for _, dto := range dtos {
		if dto.OccurredAt.After(since) {
			filtered = append(filtered, dto)
		}
	}

	return s.deserializeEvents(filtered)
}

// LoadAllEvents retrieves all events for a repository (for auditing).
func (s *FileEventStore) LoadAllEvents(ctx context.Context, repoRoot string) ([]domain.DomainEvent, error) {
	dir := eventsPath(repoRoot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read events directory: %w", err)
	}

	var allDTOs []storedEventDTO
	for _, entry := range entries {
		if entry.IsDir() || !hasEventsSuffix(entry.Name()) {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		dtos, err := s.loadEventsFromFile(path)
		if err != nil {
			continue // Skip files that fail to parse
		}
		allDTOs = append(allDTOs, dtos...)
	}

	// Sort by occurred_at
	sort.Slice(allDTOs, func(i, j int) bool {
		return allDTOs[i].OccurredAt.Before(allDTOs[j].OccurredAt)
	})

	return s.deserializeEvents(allDTOs)
}

// loadEventsFromFile loads events from a JSONL file.
func (s *FileEventStore) loadEventsFromFile(path string) ([]storedEventDTO, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var dtos []storedEventDTO
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	for decoder.More() {
		var dto storedEventDTO
		if err := decoder.Decode(&dto); err != nil {
			continue // Skip malformed lines
		}
		dtos = append(dtos, dto)
	}

	return dtos, nil
}

// deserializeEvents converts DTOs back to domain events.
func (s *FileEventStore) deserializeEvents(dtos []storedEventDTO) ([]domain.DomainEvent, error) {
	events := make([]domain.DomainEvent, 0, len(dtos))
	for _, dto := range dtos {
		event, err := deserializeEvent(dto.EventName, dto.Payload)
		if err != nil {
			continue // Skip unknown event types
		}
		events = append(events, event)
	}
	return events, nil
}

// getNextSequence returns the next sequence number for a file.
func (s *FileEventStore) getNextSequence(path string) int64 {
	dtos, err := s.loadEventsFromFile(path)
	if err != nil || len(dtos) == 0 {
		return 1
	}
	return dtos[len(dtos)-1].SequenceNum + 1
}

// hasEventsSuffix checks if filename has events suffix.
func hasEventsSuffix(name string) bool {
	return len(name) > len(eventsFileSuffix) && name[len(name)-len(eventsFileSuffix):] == eventsFileSuffix
}

// deserializeEvent converts raw JSON back to a domain event.
func deserializeEvent(eventName string, payload json.RawMessage) (domain.DomainEvent, error) {
	switch eventName {
	case "run.created":
		var e domain.RunCreatedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "run.state_transitioned":
		var e domain.StateTransitionedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "run.planned":
		var e domain.RunPlannedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "run.versioned":
		var e domain.RunVersionedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "run.notes_generated":
		var e domain.RunNotesGeneratedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "run.notes_updated":
		var e domain.RunNotesUpdatedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "run.approved":
		var e domain.RunApprovedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "run.publishing_started":
		var e domain.RunPublishingStartedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "run.published":
		var e domain.RunPublishedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "run.failed":
		var e domain.RunFailedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "run.canceled":
		var e domain.RunCanceledEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "run.retried":
		var e domain.RunRetriedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "step.completed":
		var e domain.StepCompletedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "plugin.executed":
		var e domain.PluginExecutedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	default:
		return nil, fmt.Errorf("unknown event type: %s", eventName)
	}
}

// Context key for repo root
type repoRootKey struct{}

// WithRepoRoot adds repo root to context.
func WithRepoRoot(ctx context.Context, repoRoot string) context.Context {
	return context.WithValue(ctx, repoRootKey{}, repoRoot)
}

// getRepoRootFromContext retrieves repo root from context.
func getRepoRootFromContext(ctx context.Context) string {
	if v := ctx.Value(repoRootKey{}); v != nil {
		return v.(string)
	}
	return ""
}
