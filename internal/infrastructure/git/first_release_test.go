package git

import (
	"context"
	"testing"
)

// `relicta plan` could not run in a repository with no version tags — every
// project's first release. The plan use case resolves a baseline by asking for the
// latest version tag, sets baseRef to "" when there is none, and passes that to
// GetCommitsBetween, which asked git to resolve "" as a reference:
//
//	✗ failed to plan release: failed to get commits: git.GetCommitsBetween:
//	  failed to resolve from reference : failed to resolve reference : reference not found
//
// GetCommitsSince has always handled the same case by walking all of history, and
// getAllCommitsFromHead says in its own comment that it is "for first release
// scenarios". It was simply unreachable from GetCommitsBetween. Every test fixture
// in this package tagged before planning, which is what hid it.

// newTestRepoWithCommits builds a repository with the given commits and no tags —
// the state no existing fixture in this package produces, since they all tag
// before exercising commit ranges.
func newTestRepoWithCommits(t *testing.T, messages ...string) *ServiceImpl {
	t.Helper()

	helper := newTestRepo(t)
	for _, m := range messages {
		helper.makeCommit(m)
	}

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

func TestGetCommitsBetweenTreatsAnEmptyFromAsTheWholeHistory(t *testing.T) {
	repo := newTestRepoWithCommits(t, "feat: first", "fix: second", "feat: third")
	ctx := context.Background()

	head, err := repo.GetHeadCommit(ctx)
	if err != nil {
		t.Fatalf("GetHeadCommit: %v", err)
	}

	commits, err := repo.GetCommitsBetween(ctx, "", head.Hash)
	if err != nil {
		t.Fatalf("an empty from must mean the whole history, got: %v", err)
	}
	if len(commits) != 3 {
		t.Errorf("got %d commits, want all 3", len(commits))
	}
}

// The two siblings must agree. GetCommitsSince("") already returned all of history;
// GetCommitsBetween("", head) returning an error for the same question is the
// inconsistency that produced the bug.
func TestGetCommitsBetweenAndSinceAgreeOnAnEmptyRef(t *testing.T) {
	repo := newTestRepoWithCommits(t, "feat: one", "fix: two")
	ctx := context.Background()

	head, err := repo.GetHeadCommit(ctx)
	if err != nil {
		t.Fatalf("GetHeadCommit: %v", err)
	}

	since, err := repo.GetCommitsSince(ctx, "")
	if err != nil {
		t.Fatalf("GetCommitsSince: %v", err)
	}
	between, err := repo.GetCommitsBetween(ctx, "", head.Hash)
	if err != nil {
		t.Fatalf("GetCommitsBetween: %v", err)
	}

	if len(since) != len(between) {
		t.Errorf("GetCommitsSince returned %d commits and GetCommitsBetween returned %d "+
			"for the same range", len(since), len(between))
	}
}

// A non-empty from must still be resolved, and an unresolvable one must still
// error. Making "" mean "everything" must not turn a typo into a silent
// whole-history changeset.
func TestGetCommitsBetweenStillRejectsAnUnresolvableRef(t *testing.T) {
	repo := newTestRepoWithCommits(t, "feat: one")
	ctx := context.Background()

	head, err := repo.GetHeadCommit(ctx)
	if err != nil {
		t.Fatalf("GetHeadCommit: %v", err)
	}

	if _, err := repo.GetCommitsBetween(ctx, "v9.9.9-does-not-exist", head.Hash); err == nil {
		t.Error("a tag that does not exist must be an error, not the whole history")
	}
}

// An unresolvable `to` must be reported as such. The empty-from branch resolves
// `to` before taking its shortcut, so this covers that it does not swallow the
// failure.
func TestGetCommitsBetweenReportsAnUnresolvableTo(t *testing.T) {
	repo := newTestRepoWithCommits(t, "feat: one")

	_, err := repo.GetCommitsBetween(context.Background(), "", "v9.9.9-does-not-exist")
	if err == nil {
		t.Fatal("an unresolvable to must be an error")
	}
}
