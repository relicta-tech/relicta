CREATE TABLE IF NOT EXISTS token_revocations (
    jti TEXT NOT NULL PRIMARY KEY,
    expires_at INTEGER NOT NULL
) STRICT;

CREATE INDEX IF NOT EXISTS idx_token_revocations_expiry
    ON token_revocations (expires_at);
