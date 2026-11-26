package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/relicta-tech/relicta/v5/internal/analytics"
)

// AnalyticsStore persists analytics events in PostgreSQL.
type AnalyticsStore struct{ pool *pgxpool.Pool }

var _ analytics.Store = (*AnalyticsStore)(nil)

func NewAnalyticsStore(pool *pgxpool.Pool) *AnalyticsStore { return &AnalyticsStore{pool: pool} }

func (s *AnalyticsStore) Append(ctx context.Context, event analytics.Event) error {
	if event.ID == "" || !event.Type.Valid() || event.Timestamp.IsZero() || !json.Valid(event.Payload) {
		return fmt.Errorf("analytics event requires an ID, valid type, timestamp, and JSON payload")
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO analytics_events(event_id, timestamp, event_type, release_id, payload)
		VALUES ($1, $2, $3, $4, $5)`,
		event.ID, event.Timestamp.UTC(), string(event.Type), event.ReleaseID, event.Payload)
	if err != nil {
		return fmt.Errorf("append analytics event %s: %w", event.ID, err)
	}
	return nil
}

func (s *AnalyticsStore) Query(ctx context.Context, filter analytics.QueryFilter) ([]analytics.Event, error) {
	query := `SELECT event_id, timestamp, event_type, release_id, payload FROM analytics_events WHERE TRUE`
	args := make([]any, 0, 4)
	add := func(clause string, value any) {
		args = append(args, value)
		query += fmt.Sprintf(" AND %s $%d", clause, len(args))
	}
	if filter.From != nil {
		add("timestamp >=", filter.From.UTC())
	}
	if filter.To != nil {
		add("timestamp <=", filter.To.UTC())
	}
	if filter.EventType != nil {
		add("event_type =", string(*filter.EventType))
	}
	if filter.ReleaseID != "" {
		add("release_id =", filter.ReleaseID)
	}
	query += " ORDER BY timestamp ASC, event_id ASC"
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query analytics events: %w", err)
	}
	defer rows.Close()
	var events []analytics.Event
	for rows.Next() {
		var event analytics.Event
		var eventType string
		var payload []byte
		if err := rows.Scan(&event.ID, &event.Timestamp, &eventType, &event.ReleaseID, &payload); err != nil {
			return nil, fmt.Errorf("scan analytics event: %w", err)
		}
		event.Timestamp = event.Timestamp.UTC()
		event.Type = analytics.EventType(eventType)
		event.Payload = json.RawMessage(payload)
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate analytics events: %w", err)
	}
	return events, nil
}

func VerifyAnalyticsSchema(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, "SELECT 1 FROM analytics_events LIMIT 0"); err != nil {
		return fmt.Errorf("table analytics_events is unavailable: %w", err)
	}
	return nil
}
