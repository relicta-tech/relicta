package protocol

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Records nobody can read are only marginally better than records that do not
// exist. The handshake was made durable so a decision made by an agent leaves
// evidence, and the only way to retrieve it was to already know the proposal's ID
// — which is exactly what someone auditing a release afterwards does not have.

func TestListProposalsIsNewestFirst(t *testing.T) {
	root := t.TempDir()
	store := NewFileProposalStore(root)
	ctx := context.Background()

	for _, id := range []string{"prop_oldest", "prop_middle", "prop_newest"} {
		if err := store.SaveProposal(ctx, sampleProposal(id)); err != nil {
			t.Fatalf("SaveProposal(%s): %v", id, err)
		}
		// Modification times have limited resolution, and the ordering is derived
		// from them; without a gap the result would depend on filesystem timestamp
		// granularity and the test would pass or fail by luck.
		stampFile(t, root, id, time.Now())
		time.Sleep(10 * time.Millisecond)
	}

	ids, err := store.ListProposals(ctx)
	if err != nil {
		t.Fatalf("ListProposals: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("got %d proposals, want 3: %v", len(ids), ids)
	}
	if ids[0] != "prop_newest" || ids[2] != "prop_oldest" {
		t.Errorf("expected newest first, got %v", ids)
	}
}

// stampFile sets a record's modification time, so ordering is deterministic.
func stampFile(t *testing.T, root, id string, at time.Time) {
	t.Helper()
	path := filepath.Join(root, ".relicta", cgpDirName, proposalsDir, id+".json")
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatalf("stamp %s: %v", id, err)
	}
}

// A repository where no agent has proposed anything is an ordinary state, not an
// error. Returning an error here would make `relicta cgp list` fail on every
// repository that has not used the protocol yet.
func TestListProposalsOnAnUnusedRepository(t *testing.T) {
	ids, err := NewFileProposalStore(t.TempDir()).ListProposals(context.Background())
	if err != nil {
		t.Errorf("an unused repository is not an error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected no proposals, got %v", ids)
	}
}

// Only proposals. Decisions and authorizations live in sibling directories and
// share the proposal's ID, so a listing that read the wrong directory would look
// plausible while reporting the wrong thing.
func TestListProposalsIgnoresOtherRecordKinds(t *testing.T) {
	root := t.TempDir()
	store := NewFileProposalStore(root)
	ctx := context.Background()

	if err := store.SaveProposal(ctx, sampleProposal("prop_one")); err != nil {
		t.Fatalf("SaveProposal: %v", err)
	}
	if err := store.SaveDecision(ctx, decisionFor("prop_one")); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}
	if err := store.SaveAuthorization(ctx, authorizationFor("prop_one")); err != nil {
		t.Fatalf("SaveAuthorization: %v", err)
	}

	ids, err := store.ListProposals(ctx)
	if err != nil {
		t.Fatalf("ListProposals: %v", err)
	}
	if len(ids) != 1 || ids[0] != "prop_one" {
		t.Errorf("expected exactly the one proposal, got %v", ids)
	}
}

// A stray non-JSON file in the directory must not become a proposal ID — the
// mistake that made the release store's List return a sibling artifact as a run
// (#261).
func TestListProposalsIgnoresNonRecords(t *testing.T) {
	root := t.TempDir()
	store := NewFileProposalStore(root)
	ctx := context.Background()

	if err := store.SaveProposal(ctx, sampleProposal("prop_real")); err != nil {
		t.Fatalf("SaveProposal: %v", err)
	}

	dir := filepath.Join(root, ".relicta", cgpDirName, proposalsDir)
	for _, name := range []string{"README", "prop_partial.json.tmp", ".DS_Store"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	ids, err := store.ListProposals(ctx)
	if err != nil {
		t.Fatalf("ListProposals: %v", err)
	}
	if len(ids) != 1 || ids[0] != "prop_real" {
		t.Errorf("expected only the real proposal, got %v", ids)
	}
}
