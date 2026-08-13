// Package postgres implements a PostgreSQL-backed EventStore adapter.
//
// Events are stored in an append-only table with JSONB payload columns.
// The adapter uses pgx/v5 with connection pooling for high-throughput
// concurrent access.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
)

// Store implements the ports.EventStore interface using PostgreSQL.
// It is safe for concurrent use.
type Store struct {
	pool *pgxpool.Pool
}

// Config holds the PostgreSQL connection configuration.
type Config struct {
	ConnectionString string
	PoolSize         int32
}

// New creates a new PostgreSQL event store with connection pooling.
func New(ctx context.Context, cfg Config) (*Store, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("parsing postgres connection string: %w", err)
	}

	if cfg.PoolSize > 0 {
		poolCfg.MaxConns = cfg.PoolSize
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("creating postgres connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}

	return &Store{pool: pool}, nil
}

// NewFromPool creates a Store from an existing connection pool.
// This is useful for testing and for sharing a pool across components.
func NewFromPool(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Append persists domain events atomically within a transaction.
func (s *Store) Append(ctx context.Context, runID domain.RunID, events []domain.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Get the next sequence number for this run.
	var seqNum int64
	err = tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(sequence_num), 0) FROM events WHERE run_id = $1`,
		string(runID),
	).Scan(&seqNum)
	if err != nil {
		return fmt.Errorf("querying max sequence_num: %w", err)
	}
	seqNum++

	now := time.Now().UTC()

	for _, evt := range events {
		payload, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("marshaling event %s: %w", evt.EventName(), err)
		}

		eventID := fmt.Sprintf("%s-%d", runID, seqNum)

		_, err = tx.Exec(ctx,
			`INSERT INTO events (id, run_id, event_name, payload, occurred_at, stored_at, sequence_num, repo_root)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			eventID, string(runID), evt.EventName(), payload, evt.OccurredAt(), now, seqNum, "",
		)
		if err != nil {
			return fmt.Errorf("inserting event %s: %w", eventID, err)
		}
		seqNum++
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

// LoadEvents retrieves all events for a release run in order.
func (s *Store) LoadEvents(ctx context.Context, runID domain.RunID) ([]domain.DomainEvent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT event_name, payload
		 FROM events
		 WHERE run_id = $1
		 ORDER BY sequence_num ASC`,
		string(runID),
	)
	if err != nil {
		return nil, fmt.Errorf("querying events for run %s: %w", runID, err)
	}
	defer rows.Close()

	return scanDomainEvents(rows)
}

// LoadEventsSince retrieves events after the given timestamp for a run.
func (s *Store) LoadEventsSince(ctx context.Context, runID domain.RunID, since time.Time) ([]domain.DomainEvent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT event_name, payload
		 FROM events
		 WHERE run_id = $1 AND occurred_at > $2
		 ORDER BY sequence_num ASC`,
		string(runID), since,
	)
	if err != nil {
		return nil, fmt.Errorf("querying events since %s for run %s: %w", since, runID, err)
	}
	defer rows.Close()

	return scanDomainEvents(rows)
}

// LoadAllEvents retrieves all events for a repository (for auditing).
func (s *Store) LoadAllEvents(ctx context.Context, repoRoot string) ([]domain.DomainEvent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT event_name, payload
		 FROM events
		 ORDER BY occurred_at ASC, sequence_num ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying all events: %w", err)
	}
	defer rows.Close()

	return scanDomainEvents(rows)
}

// Close releases the connection pool.
func (s *Store) Close() error {
	s.pool.Close()
	return nil
}

// scanDomainEvents reads rows containing (event_name, payload) and deserializes them.
func scanDomainEvents(rows pgx.Rows) ([]domain.DomainEvent, error) {
	var events []domain.DomainEvent
	for rows.Next() {
		var eventName string
		var payload []byte

		if err := rows.Scan(&eventName, &payload); err != nil {
			return nil, fmt.Errorf("scanning event row: %w", err)
		}

		evt, err := deserializeEvent(eventName, payload)
		if err != nil {
			// Skip unknown event types for forward compatibility.
			continue
		}
		events = append(events, evt)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating event rows: %w", err)
	}
	return events, nil
}

// deserializeEvent converts raw JSON back to a domain event.
func deserializeEvent(eventName string, payload json.RawMessage) (domain.DomainEvent, error) {
	// Accepts the historical "run.*" spelling, so events persisted by earlier versions
	// still reconstruct.
	switch domain.CanonicalEventName(eventName) {
	case "release.created":
		var e domain.RunCreatedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "release.state_transitioned":
		var e domain.StateTransitionedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "release.planned":
		var e domain.RunPlannedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "release.versioned":
		var e domain.RunVersionedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "release.notes_generated":
		var e domain.RunNotesGeneratedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "release.notes_updated":
		var e domain.RunNotesUpdatedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "release.approved":
		var e domain.RunApprovedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "release.publishing_started":
		var e domain.RunPublishingStartedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "release.published":
		var e domain.RunPublishedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "release.failed":
		var e domain.RunFailedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "release.canceled":
		var e domain.RunCanceledEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "release.retried":
		var e domain.RunRetriedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "release.step_completed":
		var e domain.StepCompletedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "release.plugin_executed":
		var e domain.PluginExecutedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "release.tag_push_mode_detected":
		var e domain.TagPushModeDetectedEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	default:
		return nil, fmt.Errorf("unknown event type: %s", eventName)
	}
}

// compile-time interface check
var _ ports.EventStore = (*Store)(nil)
