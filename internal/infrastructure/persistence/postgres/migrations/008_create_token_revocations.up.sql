CREATE TABLE IF NOT EXISTS token_revocations (
    jti TEXT PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_token_revocations_expiry
    ON token_revocations (expires_at);
