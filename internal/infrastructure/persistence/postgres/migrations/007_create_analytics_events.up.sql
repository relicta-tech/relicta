CREATE TABLE IF NOT EXISTS analytics_events (
    event_id TEXT NOT NULL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    event_type TEXT NOT NULL,
    release_id TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_analytics_events_timestamp
    ON analytics_events (timestamp ASC, event_id ASC);
CREATE INDEX IF NOT EXISTS idx_analytics_events_type_timestamp
    ON analytics_events (event_type, timestamp ASC);
CREATE INDEX IF NOT EXISTS idx_analytics_events_release_timestamp
    ON analytics_events (release_id, timestamp ASC);
