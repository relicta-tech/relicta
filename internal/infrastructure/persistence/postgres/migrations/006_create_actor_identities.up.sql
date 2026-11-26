CREATE TABLE IF NOT EXISTS cgp_actor_identities (
    identity_id TEXT PRIMARY KEY,
    updated_at  TIMESTAMPTZ NOT NULL,
    document    JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cgp_actor_identities_updated_at
    ON cgp_actor_identities (updated_at DESC, identity_id ASC);
