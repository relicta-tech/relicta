package memory

// file_store_audit_chain_test.go is the one thing the conformance suite cannot ask: does
// the chain still exist after the process that wrote it has gone?
//
// The suite builds a store per case and never reopens one, so a file backend that appended
// to its in-memory map and forgot to write memory.json passes every case in it. That is
// the failure the original defect was made of — an audit chain that existed only for the
// lifetime of the command — and `relicta plan`, `approve` and `publish` are three separate
// processes, so a chain that does not survive one of them records nothing at all.

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/audit"
)

func TestTheChainSurvivesReopeningTheStore(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	const repository = "acme/widget"

	writer, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	recorder := audit.NewRecorder(writer, repository)
	for i, id := range []string{"e1", "e2", "e3"} {
		entry := audit.NewEntry(id, audit.EventDecisionMade).
			WithProposal("run-1").
			WithActor("human:alice", cgp.ActorKindHuman).
			WithTimestamp(time.Date(2026, 5, 1, 12, i, 0, 0, time.UTC)).
			WithDetail("riskScore", 0.35).
			// Too large for a float64 to hold exactly, so it only survives
			// memory.json if the entry was normalized through JSON before it was
			// hashed. Without it this test round trips only values that cannot
			// notice the difference, and the file backend's encoding would be
			// unguarded on the one path that actually writes to disk.
			WithDetail("linesChanged", int64(9007199254740993)).
			Build()
		if err := recorder.Record(ctx, entry); err != nil {
			t.Fatalf("recording %s: %v", id, err)
		}
	}

	// A second store over the same directory is what the next `relicta` invocation
	// opens. Nothing is shared with the first but the files.
	reader, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("reopening the store: %v", err)
	}

	chain, err := audit.LoadChain(ctx, reader, repository)
	if err != nil {
		t.Fatalf("the chain written by one process does not verify when another reads "+
			"it: %v", err)
	}
	if chain.Len() != 3 {
		t.Fatalf("a reopened store holds %d chain entries, want 3: the evidence died "+
			"with the process that recorded it", chain.Len())
	}

	// And the reopened chain must be appendable — the next process continues it rather
	// than starting a second one beside it.
	fourth := audit.NewEntry("e4", audit.EventExecutionCompleted).
		WithProposal("run-1").
		WithTimestamp(time.Date(2026, 5, 1, 12, 4, 0, 0, time.UTC)).
		Build()
	if err := audit.NewRecorder(reader, repository).Record(ctx, fourth); err != nil {
		t.Fatalf("appending to a reopened chain: %v", err)
	}
	if fourth.PreviousHash != chain.LastHash() {
		t.Errorf("the fourth entry follows %q but the reopened chain ended at %q: the "+
			"second process started a chain of its own",
			fourth.PreviousHash, chain.LastHash())
	}
}
