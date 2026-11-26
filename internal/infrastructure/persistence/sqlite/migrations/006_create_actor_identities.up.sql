CREATE TABLE IF NOT EXISTS cgp_actor_identities (
    identity_id TEXT NOT NULL PRIMARY KEY,
    updated_at  INTEGER NOT NULL,
    document    TEXT NOT NULL
) STRICT;

CREATE INDEX IF NOT EXISTS idx_cgp_actor_identities_updated_at
    ON cgp_actor_identities (updated_at DESC, identity_id ASC);
