package sqlite

// audit_chain.go is the SQLite half of the governance audit chain — see migration 004
// for why the chain is stored rather than derived from the decision rows.
//
// The append is a transaction on purpose. Reading the tail and inserting behind it is a
// read-then-write, and two `relicta publish` runs in one repository would otherwise both
// read the same tail and both insert, producing two entries claiming the same
// predecessor: a fork that verifies from either side and cannot be adjudicated
// afterwards. Under BEGIN IMMEDIATE (see openDatabase) the second writer waits, re-reads
// the tail it now has to follow, and its entry is refused as a fork rather than stored.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/relicta-tech/relicta/v4/internal/cgp/audit"
)

// AppendAuditEntry appends one linked entry to a repository's chain.
func (s *MemoryStore) AppendAuditEntry(
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

	document, err := encodeDocument(entry, "audit entry "+entry.ID)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("opening the audit chain transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var tail string
	err = tx.QueryRowContext(ctx, `
		SELECT entry_hash FROM governance_audit_entries
		WHERE repository = ?
		ORDER BY seq DESC
		LIMIT 1`, repository).Scan(&tail)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("reading the audit chain tail for %s: %w", repository, err)
	}

	// Checked before the insert rather than left to the UNIQUE constraint, because the
	// two failures need different names: a duplicate is a transition already recorded,
	// a fork is a different transition claiming a predecessor that has moved on. A
	// caller that absorbs duplicates as "already done" must not absorb forks the same
	// way.
	var existing int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM governance_audit_entries
		WHERE repository = ? AND entry_id = ?`, repository, entry.ID).Scan(&existing); err != nil {
		return fmt.Errorf("checking audit entry %s: %w", entry.ID, err)
	}
	if existing > 0 {
		return audit.ErrDuplicateEntry
	}

	if entry.PreviousHash != tail {
		return fmt.Errorf("%w: entry %s follows %q but the chain ends at %q",
			audit.ErrChainForked, entry.ID, entry.PreviousHash, tail)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO governance_audit_entries
			(repository, entry_id, proposal_id, event_type, entry_hash, previous_hash, document)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		repository, entry.ID, entry.ProposalID, string(entry.EventType),
		entry.Hash, entry.PreviousHash, document,
	); err != nil {
		return fmt.Errorf("appending audit entry %s: %w", entry.ID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing audit entry %s: %w", entry.ID, err)
	}
	return nil
}

// LastAuditEntry returns the repository's tail entry, or nil when it has no chain yet.
func (s *MemoryStore) LastAuditEntry(
	ctx context.Context, repository string,
) (*audit.Entry, error) {
	entry, err := queryDocument[audit.Entry](ctx, s.db, "audit entry", `
		SELECT document FROM governance_audit_entries
		WHERE repository = ?
		ORDER BY seq DESC
		LIMIT 1`, repository)
	if errors.Is(err, sql.ErrNoRows) {
		// An empty chain, not a failure: it is where every repository starts, and
		// the caller links its genesis entry against it.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// AuditChain returns the repository's entries in append order.
func (s *MemoryStore) AuditChain(
	ctx context.Context, repository string,
) ([]*audit.Entry, error) {
	return queryDocuments[audit.Entry](ctx, s.db, "audit entry", `
		SELECT document FROM governance_audit_entries
		WHERE repository = ?
		ORDER BY seq`, repository)
}
