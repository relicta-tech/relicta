-- 001_create_events.up.sql
-- Creates the append-only events table and supporting indexes.

CREATE TABLE IF NOT EXISTS events (
    id           TEXT        PRIMARY KEY,
    run_id       TEXT        NOT NULL,
    event_name   TEXT        NOT NULL,
    payload      JSONB       DEFAULT 'null'::jsonb,
    occurred_at  TIMESTAMPTZ NOT NULL,
    stored_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sequence_num BIGINT      NOT NULL,
    repo_root    TEXT        NOT NULL DEFAULT ''
);

-- Index for querying events by run.
CREATE INDEX IF NOT EXISTS idx_events_run_id ON events (run_id);

-- Index for querying events by event name.
CREATE INDEX IF NOT EXISTS idx_events_event_name ON events (event_name);

-- Index for querying events by timestamp (supports LoadEventsSince).
CREATE INDEX IF NOT EXISTS idx_events_occurred_at ON events (occurred_at);

-- Composite index for run + sequence ordering.
CREATE INDEX IF NOT EXISTS idx_events_run_id_sequence ON events (run_id, sequence_num);
