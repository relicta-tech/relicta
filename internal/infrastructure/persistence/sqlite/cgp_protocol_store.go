package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	cgpprotocol "github.com/relicta-tech/relicta/v5/internal/cgp/protocol"
	cgpsdk "github.com/relicta-tech/relicta/v5/pkg/cgp"
)

// CGPProtocolStore persists proposals, decisions, and authorizations in the
// configured SQLite database.
type CGPProtocolStore struct {
	db   *sql.DB
	path string
}

var _ cgpprotocol.RecordStore = (*CGPProtocolStore)(nil)

// OpenCGPProtocolStore opens the shared Relicta database and applies its
// migrations before exposing the protocol store.
func OpenCGPProtocolStore(ctx context.Context, path string) (*CGPProtocolStore, error) {
	db, err := openDatabase(ctx, path)
	if err != nil {
		return nil, err
	}
	return &CGPProtocolStore{db: db, path: path}, nil
}

func (s *CGPProtocolStore) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("closing CGP protocol database %s: %w", s.path, err)
	}
	return nil
}

func (s *CGPProtocolStore) SaveProposal(ctx context.Context, proposal *cgpsdk.ChangeProposal) error {
	if proposal == nil {
		return fmt.Errorf("save proposal: %w: nil proposal", cgpprotocol.ErrInvalidID)
	}
	if err := cgpprotocol.ValidateProtocolID(proposal.ID); err != nil {
		return err
	}
	return s.save(ctx, `INSERT INTO cgp_proposals (proposal_id, updated_at, document)
		VALUES (?, ?, ?) ON CONFLICT(proposal_id) DO UPDATE SET
		updated_at = excluded.updated_at, document = excluded.document`, proposal.ID, proposal)
}

func (s *CGPProtocolStore) GetProposal(ctx context.Context, id string) (*cgpsdk.ChangeProposal, error) {
	if err := cgpprotocol.ValidateProtocolID(id); err != nil {
		return nil, err
	}
	return loadCGPDocument[cgpsdk.ChangeProposal](ctx, s.db,
		"SELECT document FROM cgp_proposals WHERE proposal_id = ?", id, "proposal")
}

func (s *CGPProtocolStore) SaveDecision(ctx context.Context, decision *cgpsdk.GovernanceDecision) error {
	if decision == nil {
		return fmt.Errorf("save decision: %w: nil decision", cgpprotocol.ErrInvalidID)
	}
	if err := cgpprotocol.ValidateProtocolID(decision.ProposalID); err != nil {
		return err
	}
	return s.save(ctx, `INSERT INTO cgp_decisions (proposal_id, updated_at, document)
		VALUES (?, ?, ?) ON CONFLICT(proposal_id) DO UPDATE SET
		updated_at = excluded.updated_at, document = excluded.document`, decision.ProposalID, decision)
}

func (s *CGPProtocolStore) GetDecision(ctx context.Context, id string) (*cgpsdk.GovernanceDecision, error) {
	if err := cgpprotocol.ValidateProtocolID(id); err != nil {
		return nil, err
	}
	return loadCGPDocument[cgpsdk.GovernanceDecision](ctx, s.db,
		"SELECT document FROM cgp_decisions WHERE proposal_id = ?", id, "decision")
}

func (s *CGPProtocolStore) SaveAuthorization(ctx context.Context, authorization *cgpsdk.ExecutionAuthorization) error {
	if authorization == nil {
		return fmt.Errorf("save authorization: %w: nil authorization", cgpprotocol.ErrInvalidID)
	}
	if err := cgpprotocol.ValidateProtocolID(authorization.ProposalID); err != nil {
		return err
	}
	return s.save(ctx, `INSERT INTO cgp_authorizations (proposal_id, updated_at, document)
		VALUES (?, ?, ?) ON CONFLICT(proposal_id) DO UPDATE SET
		updated_at = excluded.updated_at, document = excluded.document`, authorization.ProposalID, authorization)
}

func (s *CGPProtocolStore) GetAuthorization(ctx context.Context, id string) (*cgpsdk.ExecutionAuthorization, error) {
	if err := cgpprotocol.ValidateProtocolID(id); err != nil {
		return nil, err
	}
	return loadCGPDocument[cgpsdk.ExecutionAuthorization](ctx, s.db,
		"SELECT document FROM cgp_authorizations WHERE proposal_id = ?", id, "authorization")
}

func (s *CGPProtocolStore) ListProposals(ctx context.Context) (ids []string, retErr error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT proposal_id FROM cgp_proposals ORDER BY updated_at DESC, proposal_id DESC")
	if err != nil {
		return nil, fmt.Errorf("list CGP proposals: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close CGP proposal rows: %w", err))
		}
	}()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan CGP proposal ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate CGP proposals: %w", err)
	}
	return ids, nil
}

func (s *CGPProtocolStore) save(ctx context.Context, query, id string, value any) error {
	document, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal CGP protocol record %s: %w", id, err)
	}
	if _, err := s.db.ExecContext(ctx, query, id, time.Now().UTC().UnixNano(), string(document)); err != nil {
		return fmt.Errorf("save CGP protocol record %s: %w", id, err)
	}
	return nil
}

func loadCGPDocument[T any](ctx context.Context, db *sql.DB, query, id, kind string) (*T, error) {
	var document string
	if err := db.QueryRowContext(ctx, query, id).Scan(&document); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s %s not found", cgpprotocol.ErrRecordNotFound, kind, id)
		}
		return nil, fmt.Errorf("load CGP %s %s: %w", kind, id, err)
	}
	var value T
	if err := json.Unmarshal([]byte(document), &value); err != nil {
		return nil, fmt.Errorf("decode CGP %s %s: %w", kind, id, err)
	}
	return &value, nil
}
