package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	cgpprotocol "github.com/relicta-tech/relicta/v5/internal/cgp/protocol"
	cgpsdk "github.com/relicta-tech/relicta/v5/pkg/cgp"
)

// CGPProtocolStore persists CGP handshake records in PostgreSQL.
type CGPProtocolStore struct {
	pool *pgxpool.Pool
}

var _ cgpprotocol.RecordStore = (*CGPProtocolStore)(nil)

// NewCGPProtocolStore creates an adapter over an initialized PostgreSQL pool.
func NewCGPProtocolStore(pool *pgxpool.Pool) *CGPProtocolStore {
	return &CGPProtocolStore{pool: pool}
}

func (s *CGPProtocolStore) SaveProposal(ctx context.Context, proposal *cgpsdk.ChangeProposal) error {
	if proposal == nil {
		return fmt.Errorf("save proposal: %w: nil proposal", cgpprotocol.ErrInvalidID)
	}
	if err := cgpprotocol.ValidateProtocolID(proposal.ID); err != nil {
		return err
	}
	return s.save(ctx, `INSERT INTO cgp_proposals (proposal_id, updated_at, document)
		VALUES ($1, $2, $3) ON CONFLICT (proposal_id) DO UPDATE SET
		updated_at = EXCLUDED.updated_at, document = EXCLUDED.document`, proposal.ID, proposal)
}

func (s *CGPProtocolStore) GetProposal(ctx context.Context, id string) (*cgpsdk.ChangeProposal, error) {
	if err := cgpprotocol.ValidateProtocolID(id); err != nil {
		return nil, err
	}
	return loadCGPDocument[cgpsdk.ChangeProposal](ctx, s.pool,
		"SELECT document FROM cgp_proposals WHERE proposal_id = $1", id, "proposal")
}

func (s *CGPProtocolStore) SaveDecision(ctx context.Context, decision *cgpsdk.GovernanceDecision) error {
	if decision == nil {
		return fmt.Errorf("save decision: %w: nil decision", cgpprotocol.ErrInvalidID)
	}
	if err := cgpprotocol.ValidateProtocolID(decision.ProposalID); err != nil {
		return err
	}
	return s.save(ctx, `INSERT INTO cgp_decisions (proposal_id, updated_at, document)
		VALUES ($1, $2, $3) ON CONFLICT (proposal_id) DO UPDATE SET
		updated_at = EXCLUDED.updated_at, document = EXCLUDED.document`, decision.ProposalID, decision)
}

func (s *CGPProtocolStore) GetDecision(ctx context.Context, id string) (*cgpsdk.GovernanceDecision, error) {
	if err := cgpprotocol.ValidateProtocolID(id); err != nil {
		return nil, err
	}
	return loadCGPDocument[cgpsdk.GovernanceDecision](ctx, s.pool,
		"SELECT document FROM cgp_decisions WHERE proposal_id = $1", id, "decision")
}

func (s *CGPProtocolStore) SaveAuthorization(ctx context.Context, authorization *cgpsdk.ExecutionAuthorization) error {
	if authorization == nil {
		return fmt.Errorf("save authorization: %w: nil authorization", cgpprotocol.ErrInvalidID)
	}
	if err := cgpprotocol.ValidateProtocolID(authorization.ProposalID); err != nil {
		return err
	}
	return s.save(ctx, `INSERT INTO cgp_authorizations (proposal_id, updated_at, document)
		VALUES ($1, $2, $3) ON CONFLICT (proposal_id) DO UPDATE SET
		updated_at = EXCLUDED.updated_at, document = EXCLUDED.document`, authorization.ProposalID, authorization)
}

func (s *CGPProtocolStore) GetAuthorization(ctx context.Context, id string) (*cgpsdk.ExecutionAuthorization, error) {
	if err := cgpprotocol.ValidateProtocolID(id); err != nil {
		return nil, err
	}
	return loadCGPDocument[cgpsdk.ExecutionAuthorization](ctx, s.pool,
		"SELECT document FROM cgp_authorizations WHERE proposal_id = $1", id, "authorization")
}

func (s *CGPProtocolStore) ListProposals(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT proposal_id FROM cgp_proposals ORDER BY updated_at DESC, proposal_id DESC")
	if err != nil {
		return nil, fmt.Errorf("list CGP proposals: %w", err)
	}
	defer rows.Close()

	var ids []string
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
	if _, err := s.pool.Exec(ctx, query, id, time.Now().UTC(), document); err != nil {
		return fmt.Errorf("save CGP protocol record %s: %w", id, err)
	}
	return nil
}

type cgpQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func loadCGPDocument[T any](ctx context.Context, db cgpQuerier, query, id, kind string) (*T, error) {
	var document []byte
	if err := db.QueryRow(ctx, query, id).Scan(&document); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s %s not found", cgpprotocol.ErrRecordNotFound, kind, id)
		}
		return nil, fmt.Errorf("load CGP %s %s: %w", kind, id, err)
	}
	var value T
	if err := json.Unmarshal(document, &value); err != nil {
		return nil, fmt.Errorf("decode CGP %s %s: %w", kind, id, err)
	}
	return &value, nil
}

// VerifyCGPProtocolSchema checks that manual PostgreSQL migrations created the
// tables this adapter needs.
func VerifyCGPProtocolSchema(ctx context.Context, pool *pgxpool.Pool) error {
	for _, table := range []string{"cgp_proposals", "cgp_decisions", "cgp_authorizations"} {
		if _, err := pool.Exec(ctx, "SELECT 1 FROM "+table+" LIMIT 0"); err != nil {
			return fmt.Errorf("table %s is unavailable: %w", table, err)
		}
	}
	return nil
}
