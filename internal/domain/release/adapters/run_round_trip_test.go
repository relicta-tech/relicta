package adapters

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
)

// There were two file-based implementations of this aggregate with incompatible on-disk
// schemas, and commands disagreed about which to use. The other one reconstructed a run
// lossily — BaseRef came from the branch, HeadSHA came back empty, Commits were dropped,
// and the changeset was looked for in a different place — so a run loaded through it could
// not support anything needing commits, HEAD or the base ref, which is most of governance.
// `relicta evaluate` failed on every release in every repository with "invalid scope:
// either commitRange or commits is required".
//
// One implementation survives, reached through a bridge for the callers that used the other
// interface. What was missing from the backlog's acceptance was this test: the absence of a
// round trip covering all four fields is why the divergence lasted. BaseRef in particular
// had no assertion anywhere, and it was the field the lossy loader filled with the wrong
// value rather than leaving empty — the failure mode that looks like data instead of
// absence.
func TestARunRoundTripsWithEverythingGovernanceNeeds(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	commits := []domain.CommitSHA{"aaa111", "bbb222", "ccc333"}
	run := domain.NewReleaseRun(
		"acme/widget",
		repoRoot,
		"refs/tags/v1.2.0", // the base ref, deliberately not a branch name
		domain.CommitSHA("ccc333"),
		commits,
		"config-hash",
		"plugin-hash",
	)

	changeSet := changes.NewChangeSet("cs-round-trip", "refs/tags/v1.2.0", "ccc333")
	changeSet.AddCommit(changes.NewConventionalCommit("aaa111", changes.CommitTypeFeat, "add the thing"))
	changeSet.AddCommit(changes.NewConventionalCommit("bbb222", changes.CommitTypeFix, "correct the thing"))
	run.SetChangeSet(changeSet)

	if err := repo.Save(ctx, run); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := repo.LoadFromRepo(ctx, repoRoot, run.ID())
	if err != nil {
		t.Fatalf("LoadFromRepo: %v", err)
	}

	// The base ref, which nothing asserted before. The lossy loader set this from the
	// branch, so it came back populated and wrong — governance then evaluated a range
	// that was not the one planned.
	if loaded.BaseRef() != run.BaseRef() {
		t.Errorf("BaseRef = %q, want %q: a run whose base ref does not survive describes a "+
			"different range than the one that was planned", loaded.BaseRef(), run.BaseRef())
	}

	if loaded.HeadSHA() != run.HeadSHA() {
		t.Errorf("HeadSHA = %q, want %q: an empty HEAD leaves governance unable to say what "+
			"it evaluated", loaded.HeadSHA(), run.HeadSHA())
	}

	if got := loaded.Commits(); len(got) != len(commits) {
		t.Fatalf("loaded %d commits, want %d: without them the proposal has no scope and "+
			"evaluation refuses it", len(got), len(commits))
	} else {
		for i := range commits {
			if got[i] != commits[i] {
				t.Errorf("commit %d = %q, want %q", i, got[i], commits[i])
			}
		}
	}

	if !loaded.HasChangeSet() {
		t.Fatal("the changeset did not survive: this is the exact failure that made " +
			"`relicta evaluate` refuse every release with \"invalid scope\"")
	}
	if got, want := len(loaded.ChangeSet().Commits()), len(changeSet.Commits()); got != want {
		t.Errorf("changeset carries %d commits, want %d", got, want)
	}
}
