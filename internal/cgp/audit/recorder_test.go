package audit_test

// recorder_test.go covers what the conformance suite cannot: the linking itself, and the
// two things a caller does with a chain once it is stored — reload it, and find out that
// somebody changed it.
//
// The suite in internal/cgp/memory/conformance proves every backend stores entries the
// same way. These tests prove the entries are worth storing.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/audit"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

const testRepo = "acme/widget"

func newStore(t *testing.T) memory.Store {
	t.Helper()
	return memory.NewInMemoryStore()
}

func record(t *testing.T, r *audit.Recorder, id string, eventType audit.EventType) *audit.Entry {
	t.Helper()

	entry := audit.NewEntry(id, eventType).
		WithProposal("run-1").
		WithActor("human:alice", cgp.ActorKindHuman).
		WithTimestamp(time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)).
		WithDetail("riskScore", 0.42).
		Build()

	if err := r.Record(context.Background(), entry); err != nil {
		t.Fatalf("recording %s: %v", id, err)
	}
	return entry
}

func TestEachEntryLinksToTheOneBeforeIt(t *testing.T) {
	store := newStore(t)
	recorder := audit.NewRecorder(store, testRepo)

	first := record(t, recorder, "e1", audit.EventProposalReceived)
	second := record(t, recorder, "e2", audit.EventDecisionMade)

	if first.PreviousHash != "" {
		t.Errorf("the first entry names predecessor %q, want none", first.PreviousHash)
	}
	if second.PreviousHash != first.Hash {
		t.Errorf("the second entry follows %q but the first hashed to %q: the chain has "+
			"two starting points and neither can be checked against the other",
			second.PreviousHash, first.Hash)
	}
}

// The point of the whole design: an entry, once appended, cannot be revised without every
// entry after it failing. A test that only checked Verify on an untouched chain would pass
// against a chain that verified nothing.
func TestEditingAStoredEntryBreaksTheChain(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	recorder := audit.NewRecorder(store, testRepo)

	record(t, recorder, "e1", audit.EventProposalReceived)
	record(t, recorder, "e2", audit.EventDecisionMade)
	record(t, recorder, "e3", audit.EventApprovalGranted)

	if _, err := audit.LoadChain(ctx, store, testRepo); err != nil {
		t.Fatalf("the chain does not verify before anything was corrupted: %v", err)
	}

	// Reach past the store's copy-on-write and edit the evidence in place, the way
	// somebody with the memory.json or the database file would.
	entries, err := store.AuditChain(ctx, testRepo)
	if err != nil {
		t.Fatalf("AuditChain: %v", err)
	}
	entries[1].Details["riskScore"] = 0.01
	corrupted, err := audit.Restore(entries)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}

	verifyErr := corrupted.Verify()
	if verifyErr == nil {
		t.Fatal("a chain whose second entry had its risk score rewritten reports itself " +
			"intact: the chain certifies whatever the records currently say")
	}
	if !errors.Is(verifyErr, audit.ErrChainCorrupted) {
		t.Errorf("Verify returned %v, want an ErrChainCorrupted: callers tell tampering "+
			"apart from a read failure by this error", verifyErr)
	}
	if !contains(verifyErr.Error(), "e2") {
		t.Errorf("Verify said %q without naming the entry that broke, so an operator "+
			"is told the chain is bad and not which link to look at", verifyErr)
	}
}

// Reordering keeps every entry byte-identical and still has to fail: an audit trail that
// can be resequenced is one where an approval can be moved to before the change it
// approved.
func TestReorderingEntriesBreaksTheChain(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	recorder := audit.NewRecorder(store, testRepo)

	record(t, recorder, "e1", audit.EventProposalReceived)
	record(t, recorder, "e2", audit.EventDecisionMade)
	record(t, recorder, "e3", audit.EventApprovalGranted)

	entries, err := store.AuditChain(ctx, testRepo)
	if err != nil {
		t.Fatalf("AuditChain: %v", err)
	}
	entries[1], entries[2] = entries[2], entries[1]

	reordered, err := audit.Restore(entries)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if err := reordered.Verify(); err == nil {
		t.Fatal("a chain with two entries swapped reports itself intact: the order of " +
			"governance events is not part of what the chain attests to")
	}
}

// A repository that has recorded nothing must be distinguishable from one whose evidence
// could not be read. They render identically in an attestation — an empty hash and a zero
// count — and mean opposite things.
func TestAnEmptyChainIsNotAFailure(t *testing.T) {
	chain, err := audit.LoadChain(context.Background(), newStore(t), testRepo)
	if err != nil {
		t.Fatalf("loading the chain of a repository with no governance events: %v", err)
	}
	if chain.Len() != 0 || chain.LastHash() != "" {
		t.Errorf("an untouched repository has a chain of %d entries ending at %q, want "+
			"an empty one", chain.Len(), chain.LastHash())
	}
}

// The recorder is built even when governance memory is off, and must then do nothing
// rather than fail every release.
func TestRecordingWithoutAStoreIsNotAnError(t *testing.T) {
	entry := audit.NewEntry("e1", audit.EventDecisionMade).Build()

	if err := audit.NewRecorder(nil, testRepo).Record(context.Background(), entry); err != nil {
		t.Errorf("recording with no store returned %v: a build with governance memory "+
			"disabled would fail every governance transition", err)
	}
}

// Filing entries under "" would collect every repository that failed to resolve an
// identity into one chain — which verifies perfectly and attributes one project's releases
// to another.
func TestRecordingWithoutARepositoryIsRefused(t *testing.T) {
	entry := audit.NewEntry("e1", audit.EventDecisionMade).Build()

	err := audit.NewRecorder(newStore(t), "").Record(context.Background(), entry)
	if !errors.Is(err, audit.ErrNoRepository) {
		t.Errorf("recording without a repository returned %v, want ErrNoRepository: the "+
			"entry would join every other unidentified repository's chain", err)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
