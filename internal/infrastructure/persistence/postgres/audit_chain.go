package postgres

// audit_chain.go is the PostgreSQL half of the governance audit chain — see migration
// 004 for why the chain is stored rather than derived from the decision rows.
//
// This is the backend where the append has to be defended hardest. Reading the tail and
// inserting behind it is a read-then-write, and a shared database has concurrent writers
// by design: two releases approved at the same moment would each read the same tail and
// each insert, producing two entries that name one predecessor. Both chains would verify
// and they would disagree about what happened, with nothing in the data to say which is
// the record.
//
// Two things stop that. The insert runs in a serializable transaction, so the second
// writer is either serialized behind the first or rolled back by the server. And
// (repository, previous_hash) is unique, so even a caller that skipped the transaction
// gets a constraint violation instead of a fork. The constraint is the guarantee; the
// transaction is what turns it into a clear error rather than a race.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/relicta-tech/relicta/v4/internal/cgp/audit"
)

// uniqueViolation is PostgreSQL's SQLSTATE for a broken unique constraint.
const uniqueViolation = "23505"

// serializationFailure is PostgreSQL's SQLSTATE for a transaction the server rolled back
// to preserve serializability.
const serializationFailure = "40001"

// AppendAuditEntry appends one linked entry to a repository's chain.
func (s *GovernanceMemoryStore) AppendAuditEntry(
	ctx context.Context, repository string, entry *audit.Entry,
) error {
	if repository == "" {
		return errors.New("repository is required")
	}
	if entry == nil {
		return errors.New("audit entry is required")
	}
	if entry.ID == "" {
		return errors.New("audit entry ID is required")
	}

	payload, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encoding audit entry %s: %w", entry.ID, err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("opening the audit chain transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var tail string
	err = tx.QueryRow(ctx, `
		SELECT entry_hash FROM governance_audit_entries
		WHERE repository = $1
		ORDER BY seq DESC
		LIMIT 1`, repository).Scan(&tail)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("reading the audit chain tail for %s: %w", repository, err)
	}

	// Named before the insert rather than decoded out of a constraint violation, because
	// the two failures mean different things to the caller: a duplicate is a transition
	// already recorded, a fork is a different transition claiming a predecessor that has
	// moved. The constraints still stand behind both — this is which error the caller
	// sees, not whether the write is refused.
	var existing int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM governance_audit_entries
		WHERE repository = $1 AND entry_id = $2`, repository, entry.ID).Scan(&existing); err != nil {
		return fmt.Errorf("checking audit entry %s: %w", entry.ID, err)
	}
	if existing > 0 {
		return audit.ErrDuplicateEntry
	}

	if entry.PreviousHash != tail {
		return fmt.Errorf("%w: entry %s follows %q but the chain ends at %q",
			audit.ErrChainForked, entry.ID, entry.PreviousHash, tail)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO governance_audit_entries
			(repository, entry_id, proposal_id, event_type,
			 entry_hash, previous_hash, recorded_at, payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		repository, entry.ID, entry.ProposalID, string(entry.EventType),
		entry.Hash, entry.PreviousHash, entry.Timestamp, payload,
	)
	if err != nil {
		return appendConflictError(err, entry)
	}

	if err := tx.Commit(ctx); err != nil {
		return appendConflictError(err, entry)
	}
	return nil
}

// appendConflictError names what a refused append means.
//
// A unique violation or a serialization failure here is one thing: another writer got to
// this tail first. Reporting it as a fork rather than as a database error is the whole
// point — the caller is told its entry was not recorded and why, instead of being handed
// a SQLSTATE and left to guess whether the chain now has a hole in it.
func appendConflictError(err error, entry *audit.Entry) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) &&
		(pgErr.Code == uniqueViolation || pgErr.Code == serializationFailure) {
		return fmt.Errorf("%w: entry %s follows %q, which another writer already extended",
			audit.ErrChainForked, entry.ID, entry.PreviousHash)
	}
	return fmt.Errorf("appending audit entry %s: %w", entry.ID, err)
}

// LastAuditEntry returns the repository's tail entry, or nil when it has no chain yet.
func (s *GovernanceMemoryStore) LastAuditEntry(
	ctx context.Context, repository string,
) (*audit.Entry, error) {
	var payload []byte
	err := s.pool.QueryRow(ctx, `
		SELECT payload FROM governance_audit_entries
		WHERE repository = $1
		ORDER BY seq DESC
		LIMIT 1`, repository).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		// An empty chain, not a failure: it is where every repository starts.
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the audit chain tail for %s: %w", repository, err)
	}
	return unmarshalPayload[audit.Entry](payload, "audit entry")
}

// AuditChain returns the repository's entries in append order.
func (s *GovernanceMemoryStore) AuditChain(
	ctx context.Context, repository string,
) ([]*audit.Entry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT payload FROM governance_audit_entries
		WHERE repository = $1
		ORDER BY seq`, repository)
	if err != nil {
		return nil, fmt.Errorf("reading the audit chain for %s: %w", repository, err)
	}
	return scanPayloads[audit.Entry](rows, "audit entry")
}
