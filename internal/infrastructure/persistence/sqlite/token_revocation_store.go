package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/relicta-tech/relicta/v5/internal/security/token"
)

// TokenRevocationStore persists dashboard token denials in the repository database.
type TokenRevocationStore struct {
	db   *sql.DB
	path string
}

var _ token.RevocationStore = (*TokenRevocationStore)(nil)

func tokenStoreContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

func OpenTokenRevocationStore(ctx context.Context, path string) (*TokenRevocationStore, error) {
	db, err := openDatabase(ctx, path)
	if err != nil {
		return nil, err
	}
	return &TokenRevocationStore{db: db, path: path}, nil
}

func (s *TokenRevocationStore) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("closing token revocation database %s: %w", s.path, err)
	}
	return nil
}

func (s *TokenRevocationStore) IsRevoked(jti string) (bool, error) {
	ctx, cancel := tokenStoreContext()
	defer cancel()
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM token_revocations WHERE jti = ? AND expires_at > ?)`,
		jti, time.Now().UTC().UnixNano()).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("read token revocation: %w", err)
	}
	return exists, nil
}

func (s *TokenRevocationStore) Revoke(jti string, expiresAt time.Time) error {
	ctx, cancel := tokenStoreContext()
	defer cancel()
	_, err := s.db.ExecContext(ctx, `INSERT INTO token_revocations(jti, expires_at) VALUES (?, ?) ON CONFLICT(jti) DO UPDATE SET expires_at = excluded.expires_at`,
		jti, expiresAt.UTC().UnixNano())
	if err != nil {
		return fmt.Errorf("persist token revocation: %w", err)
	}
	return nil
}

func (s *TokenRevocationStore) ClaimRefresh(jti string, expiresAt time.Time) (bool, error) {
	ctx, cancel := tokenStoreContext()
	defer cancel()
	result, err := s.db.ExecContext(ctx, `INSERT INTO token_revocations(jti, expires_at) VALUES (?, ?) ON CONFLICT(jti) DO NOTHING`,
		jti, expiresAt.UTC().UnixNano())
	if err != nil {
		return false, fmt.Errorf("claim refresh token: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read refresh-token claim result: %w", err)
	}
	return count == 1, nil
}

func (s *TokenRevocationStore) RevokePair(accessJTI string, accessExpiry time.Time, refreshJTI string, refreshExpiry time.Time) error {
	ctx, cancel := tokenStoreContext()
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin token revocation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO token_revocations(jti, expires_at) VALUES (?, ?) ON CONFLICT(jti) DO UPDATE SET expires_at = excluded.expires_at`, accessJTI, accessExpiry.UTC().UnixNano()); err != nil {
		return fmt.Errorf("persist access-token revocation: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO token_revocations(jti, expires_at) VALUES (?, ?) ON CONFLICT(jti) DO UPDATE SET expires_at = excluded.expires_at`, refreshJTI, refreshExpiry.UTC().UnixNano()); err != nil {
		return fmt.Errorf("persist refresh-token revocation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit token revocations: %w", err)
	}
	return nil
}

func (s *TokenRevocationStore) CleanExpired(now time.Time) error {
	ctx, cancel := tokenStoreContext()
	defer cancel()
	if _, err := s.db.ExecContext(ctx, `DELETE FROM token_revocations WHERE expires_at <= ?`, now.UTC().UnixNano()); err != nil {
		return fmt.Errorf("clean expired token revocations: %w", err)
	}
	return nil
}
