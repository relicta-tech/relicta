package conformance

// audit_chain.go is the append-only half of the contract: the hash-linked chain of
// governance events every Store must hold identically.
//
// It is the part of the port where an adapter's freedom is smallest and the cost of
// getting it wrong is largest. The rest of the store answers questions; this one makes a
// claim about the past, and a backend that accepts an entry the others refuse produces
// evidence that verifies under its own rules and nowhere else. The three rules below are
// the whole of it — an entry follows the stored tail, an ID appears once, and the order
// entries come back in is the order they went in — and each has a case here because each
// has an obvious wrong implementation that passes every other test in the suite.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/audit"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

var auditChainCases = []testCase{
	{"a repository with no governance events has an empty chain", anEmptyChainIsNotAnError},
	{"an appended entry comes back", anAppendedEntryComesBack},
	{"a reloaded chain still verifies", aReloadedChainStillVerifies},
	{"the chain comes back in append order", theChainComesBackInAppendOrder},
	{"the tail is the entry appended last", theTailIsTheEntryAppendedLast},
	{"an entry that does not follow the tail is refused", aForkedEntryIsRefused},
	{"an entry ID cannot be recorded twice", aDuplicateEntryIsRefused},
	{"two repositories keep separate chains", twoRepositoriesKeepSeparateChains},
	{"a stored entry cannot be edited through the pointer that wrote it", storedEntriesAreCopies},
}

// recordEvent appends one governance event through the same recorder production uses.
//
// Deliberately not hand-linked: the suite tests what a backend does with entries the
// recorder produces, and a test that computed its own hashes would be free to compute
// them differently from the code under test.
func recordEvent(t *testing.T, store memory.Store, repository, id string, at time.Time) *audit.Entry {
	t.Helper()

	entry := audit.NewEntry(id, audit.EventDecisionMade).
		WithProposal("run-1").
		WithActor("human:alice", cgp.ActorKindHuman).
		WithTimestamp(at).
		WithDetail("decisionType", string(cgp.DecisionApproved)).
		WithDetail("riskScore", 0.35).
		// An integer too large for a float64 to hold exactly. It is here to make the
		// round trip through storage load-bearing: json.Unmarshal into `any` produces
		// float64 for every number, so an entry hashed before that conversion and
		// verified after it disagrees with itself. The recorder normalizes details
		// through JSON before hashing for exactly this reason, and without a value
		// that notices, every case below would pass against an entry encoding that
		// silently rewrites what it stores.
		WithDetail("linesChanged", int64(9007199254740993)).
		Build()

	if err := audit.NewRecorder(store, repository).Record(context.Background(), entry); err != nil {
		t.Fatalf("recording audit entry %s: %v", id, err)
	}
	return entry
}

// An empty chain and an unreadable one are the same value in an attestation and opposite
// facts about a release, so the store has to keep them apart at the source.
func anEmptyChainIsNotAnError(t *testing.T, store memory.Store) {
	entries, err := store.AuditChain(context.Background(), testRepo)
	if err != nil {
		t.Fatalf("AuditChain on a repository with no events: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("a repository that recorded nothing has %d chain entries, want none: "+
			"an attestation over an invented chain certifies events that never happened",
			len(entries))
	}

	tail, err := store.LastAuditEntry(context.Background(), testRepo)
	if err != nil {
		t.Fatalf("LastAuditEntry on a repository with no events: %v", err)
	}
	if tail != nil {
		t.Errorf("the tail of an empty chain is %+v, want nil: a genesis entry linked "+
			"against it would name a predecessor that does not exist", tail)
	}
}

func anAppendedEntryComesBack(t *testing.T, store memory.Store) {
	recordEvent(t, store, testRepo, "entry-1", time.Now())

	entries, err := store.AuditChain(context.Background(), testRepo)
	if err != nil {
		t.Fatalf("AuditChain: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("the chain has %d entries after one append, want 1", len(entries))
	}
	if entries[0].ID != "entry-1" || entries[0].EventType != audit.EventDecisionMade {
		t.Errorf("the entry came back as %+v, with its identity or event type lost",
			entries[0])
	}
	if entries[0].PreviousHash != "" {
		t.Errorf("the genesis entry names predecessor %q, want none: the chain would "+
			"start from a link nothing can be checked against", entries[0].PreviousHash)
	}
}

// The case that catches encoding drift rather than logic. An entry is hashed in memory
// and verified after a round trip through the backend's serialization, so any field the
// storage rewrites — a timestamp losing its offset, a number widened, a detail dropped —
// shows up here as tampering. Which is the honest report: the stored entry no longer
// matches the one that was signed for.
//
// It bites hardest on the SQL adapters, which decode a document on every read. The two
// slice-backed stores answer from the objects they were handed, so nothing is re-encoded
// inside one process and this case cannot fail for them here — the file backend's real
// round trip is through memory.json and is covered by
// TestTheChainSurvivesReopeningTheStore, which reopens the directory.
func aReloadedChainStillVerifies(t *testing.T, store memory.Store) {
	at := time.Date(2026, 3, 4, 5, 6, 7, 89000000, time.UTC)
	recordEvent(t, store, testRepo, "entry-1", at)
	recordEvent(t, store, testRepo, "entry-2", at.Add(time.Second))
	recordEvent(t, store, testRepo, "entry-3", at.Add(2*time.Second))

	chain, err := audit.LoadChain(context.Background(), store, testRepo)
	if err != nil {
		t.Fatalf("the chain does not verify after a round trip through storage: %v", err)
	}
	if chain.Len() != 3 {
		t.Errorf("the reloaded chain has %d entries, want 3", chain.Len())
	}
}

// Append order is not a presentation choice here. Every entry hashes its predecessor's
// hash, so a backend that returned its chain sorted by timestamp — a reasonable-looking
// default — would hand back a chain that fails verification whenever two events share a
// clock tick or a clock steps backwards between two invocations.
func theChainComesBackInAppendOrder(t *testing.T, store memory.Store) {
	now := time.Now()

	// Timestamps deliberately descending while the append order ascends: a store that
	// orders by time returns exactly the reverse of the right answer.
	recordEvent(t, store, testRepo, "entry-1", now)
	recordEvent(t, store, testRepo, "entry-2", now.Add(-time.Hour))
	recordEvent(t, store, testRepo, "entry-3", now.Add(-2*time.Hour))

	entries, err := store.AuditChain(context.Background(), testRepo)
	if err != nil {
		t.Fatalf("AuditChain: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("the chain has %d entries, want 3", len(entries))
	}
	for i, want := range []string{"entry-1", "entry-2", "entry-3"} {
		if entries[i].ID != want {
			t.Fatalf("entry %d is %s, want %s: the chain came back in an order its "+
				"links do not follow, so it cannot be verified", i, entries[i].ID, want)
		}
	}
}

func theTailIsTheEntryAppendedLast(t *testing.T, store memory.Store) {
	recordEvent(t, store, testRepo, "entry-1", time.Now())
	last := recordEvent(t, store, testRepo, "entry-2", time.Now().Add(-time.Hour))

	tail, err := store.LastAuditEntry(context.Background(), testRepo)
	if err != nil {
		t.Fatalf("LastAuditEntry: %v", err)
	}
	if tail == nil || tail.ID != "entry-2" {
		t.Fatalf("the tail is %+v, want entry-2: the next append would link against the "+
			"wrong predecessor and fork the chain", tail)
	}
	if tail.Hash != last.Hash {
		t.Errorf("the tail's hash is %q but the entry appended last hashed to %q",
			tail.Hash, last.Hash)
	}
}

// The rule that makes concurrent writers safe. Two releases appending at once both read
// one tail, and the second must be refused rather than stored beside the first: two
// entries naming one predecessor are two chains that each verify and disagree, with
// nothing in the data to say which is the record.
func aForkedEntryIsRefused(t *testing.T, store memory.Store) {
	recordEvent(t, store, testRepo, "entry-1", time.Now())

	forked := audit.NewEntry("entry-2", audit.EventApprovalGranted).
		WithProposal("run-1").
		WithTimestamp(time.Now()).
		Build()
	forked.PreviousHash = "" // links to the genesis position, which is taken
	forked.Hash = forked.ComputeHash()

	err := store.AppendAuditEntry(context.Background(), testRepo, forked)
	if !errors.Is(err, audit.ErrChainForked) {
		t.Fatalf("appending an entry that does not follow the tail returned %v, want "+
			"ErrChainForked: the chain now has two entries claiming one predecessor", err)
	}

	entries, chainErr := store.AuditChain(context.Background(), testRepo)
	if chainErr != nil {
		t.Fatalf("AuditChain: %v", chainErr)
	}
	if len(entries) != 1 {
		t.Errorf("the chain has %d entries after a refused append, want 1: the entry was "+
			"reported as refused and stored anyway", len(entries))
	}
}

func aDuplicateEntryIsRefused(t *testing.T, store memory.Store) {
	first := recordEvent(t, store, testRepo, "entry-1", time.Now())

	replay := audit.NewEntry("entry-1", audit.EventDecisionMade).
		WithProposal("run-1").
		WithTimestamp(time.Now()).
		Build()
	replay.PreviousHash = first.Hash
	replay.Hash = replay.ComputeHash()

	err := store.AppendAuditEntry(context.Background(), testRepo, replay)
	if !errors.Is(err, audit.ErrDuplicateEntry) {
		t.Fatalf("appending an ID already in the chain returned %v, want "+
			"ErrDuplicateEntry: a retried release would record its transitions twice "+
			"and the chain would report events that happened once as happening twice", err)
	}
}

// A shared backend serves several repositories. Interleaving their entries into one chain
// would make each repository's chain unverifiable on its own — every entry would link
// through another project's — and would attribute one team's releases to another.
func twoRepositoriesKeepSeparateChains(t *testing.T, store memory.Store) {
	const other = "owner/other"

	recordEvent(t, store, testRepo, "entry-1", time.Now())
	recordEvent(t, store, other, "entry-1", time.Now())
	recordEvent(t, store, testRepo, "entry-2", time.Now())

	mine, err := audit.LoadChain(context.Background(), store, testRepo)
	if err != nil {
		t.Fatalf("%s does not verify, so the other repository's entries were "+
			"interleaved into its chain: %v", testRepo, err)
	}
	theirs, err := audit.LoadChain(context.Background(), store, other)
	if err != nil {
		t.Fatalf("%s does not verify, so the other repository's entries were "+
			"interleaved into its chain: %v", other, err)
	}

	if mine.Len() != 2 {
		t.Errorf("%s has %d chain entries, want 2", testRepo, mine.Len())
	}
	if theirs.Len() != 1 {
		t.Errorf("%s has %d chain entries, want 1: it can see another repository's "+
			"governance events as its own", other, theirs.Len())
	}
	if theirs.LastHash() == mine.LastHash() {
		t.Errorf("both repositories' chains end at the same hash, so they are one chain")
	}
}

// The store's copy is the evidence. A backend that handed out the pointer it stored would
// let a caller edit an entry after it was recorded — and the in-memory store, where the
// caller's entry and the stored one would be the same object, is where that happens
// invisibly.
func storedEntriesAreCopies(t *testing.T, store memory.Store) {
	entry := recordEvent(t, store, testRepo, "entry-1", time.Now())

	entry.ActorID = "human:mallory"
	entry.EventType = audit.EventApprovalGranted

	chain, err := audit.LoadChain(context.Background(), store, testRepo)
	if err != nil {
		t.Fatalf("the chain stopped verifying after its caller edited the entry it "+
			"appended, so the store handed out the evidence rather than a copy: %v", err)
	}
	if got := chain.List()[0].ActorID; got != "human:alice" {
		t.Errorf("the stored entry names actor %q, want human:alice: editing the "+
			"caller's copy rewrote the record", got)
	}
}
