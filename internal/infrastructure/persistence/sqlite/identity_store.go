package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/relicta-tech/relicta/v5/internal/cgp/identity"
)

// IdentityStore persists organization actor identities in the repository database.
type IdentityStore struct {
	db   *sql.DB
	path string
}

var _ identity.RegistryStore = (*IdentityStore)(nil)

func OpenIdentityStore(ctx context.Context, path string) (*IdentityStore, error) {
	db, err := openDatabase(ctx, path)
	if err != nil {
		return nil, err
	}
	return &IdentityStore{db: db, path: path}, nil
}

func (s *IdentityStore) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("closing identity database %s: %w", s.path, err)
	}
	return nil
}

func (s *IdentityStore) LoadAll(ctx context.Context) (actors []*identity.ActorIdentity, retErr error) {
	rows, err := s.db.QueryContext(ctx, "SELECT document FROM cgp_actor_identities ORDER BY identity_id")
	if err != nil {
		return nil, fmt.Errorf("list actor identities: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close actor identity rows: %w", err))
		}
	}()
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan actor identity: %w", err)
		}
		var actor identity.ActorIdentity
		if err := json.Unmarshal([]byte(raw), &actor); err != nil {
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
	_, err = s.db.ExecContext(ctx, `INSERT INTO cgp_actor_identities(identity_id, updated_at, document)
		VALUES (?, ?, ?) ON CONFLICT(identity_id) DO UPDATE SET
		updated_at = excluded.updated_at, document = excluded.document`, actor.ID, time.Now().UTC().UnixNano(), string(raw))
	if err != nil {
		return fmt.Errorf("save actor identity %s: %w", actor.ID, err)
	}
	return nil
}

func (s *IdentityStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM cgp_actor_identities WHERE identity_id = ?", id)
	if err != nil {
		return fmt.Errorf("delete actor identity %s: %w", id, err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted actor identity %s: %w", id, err)
	}
	if count == 0 {
		return fmt.Errorf("actor not found: %s", id)
	}
	return nil
}
