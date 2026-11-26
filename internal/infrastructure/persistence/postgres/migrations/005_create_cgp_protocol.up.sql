CREATE TABLE IF NOT EXISTS cgp_proposals (
    proposal_id TEXT        PRIMARY KEY,
    updated_at  TIMESTAMPTZ NOT NULL,
    document    JSONB       NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cgp_proposals_updated_at
    ON cgp_proposals (updated_at DESC, proposal_id DESC);

CREATE TABLE IF NOT EXISTS cgp_decisions (
    proposal_id TEXT        PRIMARY KEY,
    updated_at  TIMESTAMPTZ NOT NULL,
    document    JSONB       NOT NULL
);

CREATE TABLE IF NOT EXISTS cgp_authorizations (
    proposal_id TEXT        PRIMARY KEY,
    updated_at  TIMESTAMPTZ NOT NULL,
    document    JSONB       NOT NULL
);
