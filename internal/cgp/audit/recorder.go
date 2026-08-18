package audit

// recorder.go links governance events onto the persisted chain.
//
// The linking lives here, in one place, rather than in each backend. An implementation
// of Store stores an entry and refuses one that does not follow its tail; deciding what
// "follows" means — which predecessor, which bytes are hashed — is a property of the
// evidence and not of where it is kept, and four adapters computing it four times is
// four chances for two backends to produce chains that verify under their own rules and
// not under each other's.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Recorder appends governance events to one repository's chain.
//
// It holds no entries. Each Record reads the stored tail, links against it and writes,
// so two `relicta` invocations — approve in one process, publish in another, which is
// the ordinary case — continue the same chain rather than each starting their own.
type Recorder struct {
	store      Store
	repository string
}

// NewRecorder returns a recorder appending to repository's chain in store.
func NewRecorder(store Store, repository string) *Recorder {
	return &Recorder{store: store, repository: repository}
}

// ErrNoRepository reports a recorder that cannot say whose chain it is appending to.
//
// An entry filed under "" would join every repository that also failed to resolve its
// identity into one chain, which is worse than not recording it: the entries would
// verify, and they would attribute one project's releases to another.
var ErrNoRepository = errors.New("audit chain: no repository identity to record against")

// Record links entry onto the stored tail and appends it.
//
// entry arrives from a builder with its ID, timestamp, event type and details set and
// its hash fields empty; both are filled here. On return the entry carries the hash that
// was persisted, so a caller sealing an attestation can use it without re-reading.
//
// A duplicate ID comes back as ErrDuplicateEntry rather than being swallowed. Callers
// that legitimately re-run a step — a retried publish reaching the same transition twice
// — decide for themselves that the event is already recorded; a recorder that decided
// for them could not tell a retry from a replay.
func (r *Recorder) Record(ctx context.Context, entry *Entry) error {
	if r == nil || r.store == nil {
		// Not an error. Governance memory is optional (governance.memory_enabled),
		// and a build with it off has nowhere to put evidence and no pretense of
		// having any: the attestation reports an empty chain, honestly.
		return nil
	}
	if r.repository == "" {
		return ErrNoRepository
	}
	if entry == nil {
		return errors.New("audit chain: entry is required")
	}
	if entry.ID == "" {
		return errors.New("audit chain: entry ID is required")
	}

	// Details are round-tripped through JSON before anything is hashed, so the entry
	// in hand is byte-identical to the entry that comes back from storage.
	//
	// Without this the chain would break on reload for reasons no operator could act
	// on. json.Unmarshal into `any` produces float64 for every number, so an int64
	// large enough to lose precision re-marshals differently than it went in, and the
	// entry that verified on write fails on read — reported as tampering, caused by
	// encoding. Normalizing here means ComputeHash only ever sees values that survive
	// the round trip.
	if err := entry.normalizeDetails(); err != nil {
		return fmt.Errorf("audit chain: encoding details of entry %s: %w", entry.ID, err)
	}

	tail, err := r.store.LastAuditEntry(ctx, r.repository)
	if err != nil {
		return fmt.Errorf("audit chain: reading the tail of %s: %w", r.repository, err)
	}

	entry.PreviousHash = ""
	if tail != nil {
		entry.PreviousHash = tail.Hash
	}
	entry.Hash = entry.ComputeHash()

	if err := r.store.AppendAuditEntry(ctx, r.repository, entry); err != nil {
		return fmt.Errorf("audit chain: appending %s to %s: %w", entry.ID, r.repository, err)
	}
	return nil
}

// RecordIgnoringDuplicate records entry, treating an ID already in the chain as done.
//
// The lifecycle transitions are recorded from the release event stream, and a step that
// runs twice — a retried publish, a re-approved run — re-emits the event that produced
// the entry. Its ID is derived from the run and the transition, so the second attempt
// names an entry that is already evidence, and appending a second copy would say the
// transition happened twice when it happened once.
//
// Only a duplicate is absorbed. A fork, a storage failure or a missing repository still
// come back, because those mean the entry was *not* recorded and the caller is about to
// believe it was.
func (r *Recorder) RecordIgnoringDuplicate(ctx context.Context, entry *Entry) error {
	err := r.Record(ctx, entry)
	if errors.Is(err, ErrDuplicateEntry) {
		return nil
	}
	return err
}

// normalizeDetails replaces Details with its JSON round trip. See Record.
func (e *Entry) normalizeDetails() error {
	if len(e.Details) == 0 {
		return nil
	}

	encoded, err := json.Marshal(e.Details)
	if err != nil {
		return err
	}

	var normalized map[string]any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return err
	}
	e.Details = normalized
	return nil
}

// LoadChain rebuilds a repository's chain from storage and verifies it.
//
// Verification happens here rather than being left to the caller because every caller
// wants it and the one that forgot would be the one sealing an attestation over a
// tampered chain. The returned error names the entry that broke — see Chain.Verify — so
// an operator is told which link to look at rather than that "the chain is invalid".
//
// A repository with no governance events yet loads as an empty chain and no error: Len
// is 0 and LastHash is "". That is deliberately distinct from a load failure, which is
// an error and never an empty chain, because the two look identical in an attestation
// and mean opposite things — nothing has happened yet, versus the evidence could not be
// read.
func LoadChain(ctx context.Context, store Store, repository string) (*Chain, error) {
	if store == nil {
		return NewChain(), nil
	}
	if repository == "" {
		return nil, ErrNoRepository
	}

	entries, err := store.AuditChain(ctx, repository)
	if err != nil {
		return nil, fmt.Errorf("audit chain: reading %s: %w", repository, err)
	}

	chain, err := Restore(entries)
	if err != nil {
		return nil, err
	}
	if err := chain.Verify(); err != nil {
		return nil, fmt.Errorf("audit chain for %s: %w", repository, err)
	}
	return chain, nil
}

// Restore rebuilds a chain from entries that were already linked and stored.
//
// Unlike Append, it does not recompute anything: the stored PreviousHash and Hash are
// the evidence, and a restore that recalculated them would launder a tampered entry into
// a valid-looking one on its way through memory. Verify then has something to disagree
// with.
func Restore(entries []*Entry) (*Chain, error) {
	chain := NewChain()
	for i, entry := range entries {
		if entry == nil {
			return nil, fmt.Errorf("%w: entry %d is missing", ErrChainCorrupted, i)
		}
		if _, exists := chain.byID[entry.ID]; exists {
			return nil, fmt.Errorf("%w: entry %d (%s) appears twice", ErrChainCorrupted, i, entry.ID)
		}
		chain.entries = append(chain.entries, entry)
		chain.byID[entry.ID] = entry
		chain.byProposal[entry.ProposalID] = append(chain.byProposal[entry.ProposalID], entry)
	}
	return chain, nil
}
