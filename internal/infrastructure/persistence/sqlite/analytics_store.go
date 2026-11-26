package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/relicta-tech/relicta/v5/internal/analytics"
)

// AnalyticsStore persists analytics events in the repository SQLite database.
type AnalyticsStore struct {
	db   *sql.DB
	path string
}

var _ analytics.Store = (*AnalyticsStore)(nil)

func OpenAnalyticsStore(ctx context.Context, path string) (*AnalyticsStore, error) {
	db, err := openDatabase(ctx, path)
	if err != nil {
		return nil, err
	}
	return &AnalyticsStore{db: db, path: path}, nil
}

func (s *AnalyticsStore) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("closing analytics database %s: %w", s.path, err)
	}
	return nil
}

func (s *AnalyticsStore) Append(ctx context.Context, event analytics.Event) error {
	if event.ID == "" || !event.Type.Valid() || event.Timestamp.IsZero() || !json.Valid(event.Payload) {
		return fmt.Errorf("analytics event requires an ID, valid type, timestamp, and JSON payload")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO analytics_events(event_id, timestamp, event_type, release_id, payload)
		VALUES (?, ?, ?, ?, ?)`,
		event.ID, event.Timestamp.UTC().UnixNano(), string(event.Type), event.ReleaseID, string(event.Payload))
	if err != nil {
		return fmt.Errorf("append analytics event %s: %w", event.ID, err)
	}
	return nil
}

func (s *AnalyticsStore) Query(ctx context.Context, filter analytics.QueryFilter) (events []analytics.Event, retErr error) {
	query := `SELECT event_id, timestamp, event_type, release_id, payload FROM analytics_events WHERE 1=1`
	args := make([]any, 0, 4)
	if filter.From != nil {
		query += " AND timestamp >= ?"
		args = append(args, filter.From.UTC().UnixNano())
	}
	if filter.To != nil {
		query += " AND timestamp <= ?"
		args = append(args, filter.To.UTC().UnixNano())
	}
	if filter.EventType != nil {
		query += " AND event_type = ?"
		args = append(args, string(*filter.EventType))
	}
	if filter.ReleaseID != "" {
		query += " AND release_id = ?"
		args = append(args, filter.ReleaseID)
	}
	query += " ORDER BY timestamp ASC, event_id ASC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query analytics events: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close analytics event rows: %w", err))
		}
	}()
	for rows.Next() {
		var event analytics.Event
		var timestamp int64
		var eventType, payload string
		if err := rows.Scan(&event.ID, &timestamp, &eventType, &event.ReleaseID, &payload); err != nil {
			return nil, fmt.Errorf("scan analytics event: %w", err)
		}
		event.Timestamp = time.Unix(0, timestamp).UTC()
		event.Type = analytics.EventType(eventType)
		event.Payload = json.RawMessage(payload)
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate analytics events: %w", err)
	}
	return events, nil
}
