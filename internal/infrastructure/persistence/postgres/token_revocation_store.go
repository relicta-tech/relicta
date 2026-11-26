package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/relicta-tech/relicta/v5/internal/security/token"
)

// TokenRevocationStore persists dashboard token denials in PostgreSQL.
type TokenRevocationStore struct{ pool *pgxpool.Pool }

var _ token.RevocationStore = (*TokenRevocationStore)(nil)

func tokenStoreContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

func NewTokenRevocationStore(pool *pgxpool.Pool) *TokenRevocationStore {
	return &TokenRevocationStore{pool: pool}
}

func (s *TokenRevocationStore) IsRevoked(jti string) (bool, error) {
	ctx, cancel := tokenStoreContext()
	defer cancel()
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM token_revocations WHERE jti = $1 AND expires_at > NOW())`, jti).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("read token revocation: %w", err)
	}
	return exists, nil
}

func (s *TokenRevocationStore) Revoke(jti string, expiresAt time.Time) error {
	ctx, cancel := tokenStoreContext()
	defer cancel()
	_, err := s.pool.Exec(ctx, `INSERT INTO token_revocations(jti, expires_at) VALUES ($1, $2) ON CONFLICT(jti) DO UPDATE SET expires_at = EXCLUDED.expires_at`, jti, expiresAt.UTC())
	if err != nil {
		return fmt.Errorf("persist token revocation: %w", err)
	}
	return nil
}

func (s *TokenRevocationStore) ClaimRefresh(jti string, expiresAt time.Time) (bool, error) {
	ctx, cancel := tokenStoreContext()
	defer cancel()
	tag, err := s.pool.Exec(ctx, `INSERT INTO token_revocations(jti, expires_at) VALUES ($1, $2) ON CONFLICT(jti) DO NOTHING`, jti, expiresAt.UTC())
	if err != nil {
		return false, fmt.Errorf("claim refresh token: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (s *TokenRevocationStore) RevokePair(accessJTI string, accessExpiry time.Time, refreshJTI string, refreshExpiry time.Time) error {
	ctx, cancel := tokenStoreContext()
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin token revocation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `INSERT INTO token_revocations(jti, expires_at) VALUES ($1, $2) ON CONFLICT(jti) DO UPDATE SET expires_at = EXCLUDED.expires_at`, accessJTI, accessExpiry.UTC()); err != nil {
		return fmt.Errorf("persist access-token revocation: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO token_revocations(jti, expires_at) VALUES ($1, $2) ON CONFLICT(jti) DO UPDATE SET expires_at = EXCLUDED.expires_at`, refreshJTI, refreshExpiry.UTC()); err != nil {
		return fmt.Errorf("persist refresh-token revocation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit token revocations: %w", err)
	}
	return nil
}

func (s *TokenRevocationStore) CleanExpired(now time.Time) error {
	ctx, cancel := tokenStoreContext()
	defer cancel()
	if _, err := s.pool.Exec(ctx, `DELETE FROM token_revocations WHERE expires_at <= $1`, now.UTC()); err != nil {
		return fmt.Errorf("clean expired token revocations: %w", err)
	}
	return nil
}

func VerifyTokenRevocationSchema(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `SELECT 1 FROM token_revocations LIMIT 0`); err != nil {
		return fmt.Errorf("table token_revocations is unavailable: %w", err)
	}
	return nil
}
