package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/relicta-tech/relicta/v5/internal/cgp/identity"
)

// IdentityStore persists organization actor identities in PostgreSQL.
type IdentityStore struct{ pool *pgxpool.Pool }

var _ identity.RegistryStore = (*IdentityStore)(nil)

func NewIdentityStore(pool *pgxpool.Pool) *IdentityStore { return &IdentityStore{pool: pool} }

func (s *IdentityStore) LoadAll(ctx context.Context) ([]*identity.ActorIdentity, error) {
	rows, err := s.pool.Query(ctx, "SELECT document FROM cgp_actor_identities ORDER BY identity_id")
	if err != nil {
		return nil, fmt.Errorf("list actor identities: %w", err)
	}
	defer rows.Close()
	var actors []*identity.ActorIdentity
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan actor identity: %w", err)
		}
		var actor identity.ActorIdentity
		if err := json.Unmarshal(raw, &actor); err != nil {
			return nil, fmt.Errorf("decode actor identity: %w", err)
		}
		actors = append(actors, &actor)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate actor identities: %w", err)
	}
	return actors, nil
}

func (s *IdentityStore) Save(ctx context.Context, actor *identity.ActorIdentity) error {
	if actor == nil || actor.ID == "" {
		return fmt.Errorf("actor identity and ID are required")
	}
	if err := identity.ValidateActorIdentity(actor); err != nil {
		return fmt.Errorf("invalid actor identity: %w", err)
	}
	raw, err := json.Marshal(actor)
	if err != nil {
		return fmt.Errorf("encode actor identity %s: %w", actor.ID, err)
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO cgp_actor_identities(identity_id, updated_at, document)
		VALUES ($1, $2, $3) ON CONFLICT(identity_id) DO UPDATE SET
		updated_at = EXCLUDED.updated_at, document = EXCLUDED.document`, actor.ID, time.Now().UTC(), raw)
	if err != nil {
		return fmt.Errorf("save actor identity %s: %w", actor.ID, err)
	}
	return nil
}

func (s *IdentityStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, "DELETE FROM cgp_actor_identities WHERE identity_id = $1", id)
	if err != nil {
		return fmt.Errorf("delete actor identity %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("actor not found: %s", id)
	}
	return nil
}

func VerifyIdentitySchema(ctx context.Context, pool *pgxpool.Pool) error {
	var exists bool
	err := pool.QueryRow(ctx, "SELECT to_regclass('cgp_actor_identities') IS NOT NULL").Scan(&exists)
	if err != nil {
		return fmt.Errorf("check actor identity schema: %w", err)
	}
	if !exists {
		return fmt.Errorf("table cgp_actor_identities is unavailable: %w", pgx.ErrNoRows)
	}
	return nil
}
