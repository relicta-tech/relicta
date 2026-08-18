package audit

// store.go is the persistence port for the audit chain, and the reason the chain is
// storage at all rather than a projection over the governance records.
//
// A hash chain derived from `governance_decisions` would be worthless as evidence.
// Every backend's RecordDecision is an upsert — "replacing one already recorded under
// its ID", which is what assigning into the reference's map does — so a decision is
// mutable by design. Re-hashing mutable rows produces a chain that agrees with whatever
// the rows currently say: edit a risk score, recompute, and the derived chain
// re-links around the edit and reports itself intact. That is a checksum of the present,
// not a record of the past.
//
// So the chain is appended at the moment each governance event happens, and what is
// appended is frozen. An entry's hash covers its own content and its predecessor's hash;
// nothing later can change either without breaking every link after it. The decision
// record and the chain entry then say the same thing twice, on purpose — the value of the
// second copy is precisely that it cannot be revised to match the first.
//
// It is not a fifth store. ADR-013 put one port set behind `persistence.backend`, and
// these three methods are part of the governance memory port (memory.Store embeds this
// interface), resolved by the one resolver, implemented by the same four adapters, and
// covered by the same conformance suite. A chain that lived in its own file beside
// memory.json would be exactly the split ADR-013 was written to close: an operator
// selecting sqlite would move their governance records and leave their evidence behind.

import (
	"context"
	"errors"
)

// Store persists one hash-linked audit chain per repository.
//
// Repositories are scoped by governance identity ("owner/repo"), the same key the release
// records use, because a shared PostgreSQL backend serves many checkouts and their
// evidence must not interleave into one chain — two repositories appending in turn would
// each see the other's entries as their own predecessors.
type Store interface {
	// AppendAuditEntry appends an already-linked entry to the repository's chain.
	//
	// The entry arrives with its PreviousHash and Hash set; linking and hashing are
	// Recorder's job so that there is one definition of them rather than one per
	// backend. What an implementation must enforce is the append-only precondition:
	// the entry follows the chain as stored right now.
	//
	// It returns ErrChainForked if entry.PreviousHash is not the hash of the stored
	// tail (or not empty for an empty chain), and ErrDuplicateEntry if the ID is
	// already in the chain. Both mean a writer linked against a tail that has since
	// moved — two `relicta publish` runs in one repository, or a replayed entry — and
	// both must fail rather than write a second entry claiming the same predecessor.
	// A fork that stored quietly would leave two valid-looking chains and no way to
	// say which is the record.
	AppendAuditEntry(ctx context.Context, repository string, entry *Entry) error

	// LastAuditEntry returns the repository's tail entry, or nil when the chain is
	// empty.
	//
	// A nil entry and a nil error is the honest answer for a repository that has
	// produced no governance events yet; it is what an append links its genesis
	// entry against. Callers must not read "no tail" as "could not read the chain" —
	// that distinction is the difference between a first release and a lost one, and
	// it is why this returns an error separately rather than folding both into nil.
	LastAuditEntry(ctx context.Context, repository string) (*Entry, error)

	// AuditChain returns the repository's entries in append order.
	//
	// Append order, not timestamp order: the links are what make the chain verifiable,
	// and they follow the sequence entries were written in. An entry backdated by a
	// clock skew still belongs where it was appended, and sorting by Timestamp would
	// reorder it into a chain that no longer verifies.
	AuditChain(ctx context.Context, repository string) ([]*Entry, error)
}

// ErrChainForked reports an append whose predecessor is no longer the chain's tail.
//
// Distinct from ErrChainCorrupted, which is about entries already stored: this one is
// about a write that would have created the corruption, refused before it did.
var ErrChainForked = errors.New("audit entry does not follow the stored chain tail")
