package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/domain/release"
)

// runInState builds a ReleaseRun pinned to a given state and UpdatedAt, so the
// clean criteria can be exercised over terminal and in-progress runs alike.
func runInState(t *testing.T, id release.RunID, state release.RunState, updated time.Time) *release.ReleaseRun {
	t.Helper()

	run := release.NewReleaseRunForTest(id, "main", "/test/repo")
	run.ReconstructState(release.RunSnapshot{
		ID:         id,
		RepoID:     "/test/repo",
		RepoRoot:   "/test/repo",
		BaseRef:    "main",
		State:      state,
		StepStatus: map[string]*release.StepStatus{},
		CreatedAt:  updated.Add(-time.Hour),
		UpdatedAt:  updated,
	})
	return run
}

// cleanTestRepo serves a fixed set of runs by ID.
type cleanTestRepo struct {
	cancelTestReleaseRepo
	runs map[release.RunID]*release.ReleaseRun
	ids  []release.RunID
}

func (r cleanTestRepo) FindByID(_ context.Context, id release.RunID) (*release.ReleaseRun, error) {
	if rel, ok := r.runs[id]; ok {
		return rel, nil
	}
	return nil, release.ErrRunNotFound
}

func (r cleanTestRepo) List(_ context.Context, _ string) ([]release.RunID, error) {
	return r.ids, nil
}

func newCleanTestRepo(runs ...*release.ReleaseRun) cleanTestRepo {
	repo := cleanTestRepo{runs: map[release.RunID]*release.ReleaseRun{}}
	for _, rel := range runs {
		repo.runs[rel.ID()] = rel
		repo.ids = append(repo.ids, rel.ID())
	}
	return repo
}

// TestSelectRunByID_TerminalLatestRun is the case from issue #192: a published
// run that is also the latest. --all skips it (index 0), the --keep default
// covers it, and reset/cancel decline it because nothing is in flight, so before
// --run existed there was no supported way to remove it at all.
func TestSelectRunByID_TerminalLatestRun(t *testing.T) {
	rel := runInState(t, "run-af793f8218ff0596", release.StatePublished, time.Now().Add(-90*24*time.Hour))
	repo := newCleanTestRepo(rel)

	result, err := selectRunByID(context.Background(), repo, repo.ids, "run-af793f8218ff0596", true)
	if err != nil {
		t.Fatalf("selectRunByID() error = %v", err)
	}
	if result.DeletedCount != 1 {
		t.Fatalf("DeletedCount = %d, want 1", result.DeletedCount)
	}
	if result.DeletedIDs[0] != "run-af793f8218ff0596" {
		t.Errorf("DeletedIDs = %v, want the requested run", result.DeletedIDs)
	}
	if result.KeptCount != 0 {
		t.Errorf("KeptCount = %d, want 0", result.KeptCount)
	}
}

func TestSelectRunByID_AcceptsUnambiguousPrefix(t *testing.T) {
	rel := runInState(t, "run-af793f8218ff0596", release.StateCanceled, time.Now())
	other := runInState(t, "run-b0000000", release.StatePublished, time.Now())
	repo := newCleanTestRepo(rel, other)

	result, err := selectRunByID(context.Background(), repo, repo.ids, "run-af79", true)
	if err != nil {
		t.Fatalf("selectRunByID() error = %v", err)
	}
	if len(result.DeletedIDs) != 1 || result.DeletedIDs[0] != "run-af793f8218ff0596" {
		t.Errorf("DeletedIDs = %v, want the run matching the prefix", result.DeletedIDs)
	}
	if result.KeptCount != 1 {
		t.Errorf("KeptCount = %d, want 1 (the untouched run)", result.KeptCount)
	}
}

func TestSelectRunByID_RejectsAmbiguousPrefix(t *testing.T) {
	a := runInState(t, "run-abc111", release.StatePublished, time.Now())
	b := runInState(t, "run-abc222", release.StatePublished, time.Now())
	repo := newCleanTestRepo(a, b)

	_, err := selectRunByID(context.Background(), repo, repo.ids, "run-abc", true)
	if err == nil {
		t.Fatal("selectRunByID() should reject a prefix matching several runs")
	}
	if !strings.Contains(err.Error(), "matches 2 runs") {
		t.Errorf("error = %q, want it to report the ambiguity", err)
	}
}

func TestSelectRunByID_UnknownID(t *testing.T) {
	rel := runInState(t, "run-abc111", release.StatePublished, time.Now())
	repo := newCleanTestRepo(rel)

	_, err := selectRunByID(context.Background(), repo, repo.ids, "run-does-not-exist", true)
	if err == nil {
		t.Fatal("selectRunByID() should error for an unknown run")
	}
	if !strings.Contains(err.Error(), "no release run matching") {
		t.Errorf("error = %q, want it to say nothing matched", err)
	}
}

// An in-progress run must still be protected: --run is for clearing stale
// terminal runs, not for yanking a release out from under a running workflow.
func TestSelectRunByID_RefusesActiveRun(t *testing.T) {
	rel := runInState(t, "run-active", release.StateDraft, time.Now())
	repo := newCleanTestRepo(rel)

	_, err := selectRunByID(context.Background(), repo, repo.ids, "run-active", true)
	if err == nil {
		t.Fatal("selectRunByID() should refuse a run that is still in progress")
	}
	if !strings.Contains(err.Error(), "relicta cancel") {
		t.Errorf("error = %q, want it to point at 'relicta cancel'", err)
	}
}

// TestDetermineRunsToDelete_OlderThanWithoutKeep covers the second half of #192:
// --keep defaults to 10, and that default used to override --older-than, so in a
// repository with fewer than 10 runs --older-than could never delete anything at
// any age.
func TestDetermineRunsToDelete_OlderThanWithoutKeep(t *testing.T) {
	origAll, origKeep, origExplicit := cleanAll, cleanKeepLast, cleanKeepExplicit
	t.Cleanup(func() { cleanAll, cleanKeepLast, cleanKeepExplicit = origAll, origKeep, origExplicit })

	// --older-than 30d given, --keep not given.
	cleanAll = false
	cleanKeepLast = 10
	cleanKeepExplicit = false

	rel := runInState(t, "run-old", release.StatePublished, time.Now().Add(-90*24*time.Hour))
	repo := newCleanTestRepo(rel)

	result, err := determineRunsToDelete(context.Background(), repo, repo.ids, 30*24*time.Hour, true)
	if err != nil {
		t.Fatalf("determineRunsToDelete() error = %v", err)
	}
	if result.DeletedCount != 1 {
		t.Errorf("DeletedCount = %d, want 1; a 90-day-old run should match --older-than 30d even as the only run", result.DeletedCount)
	}
}

// With --keep passed explicitly, the floor applies again and the old run is kept.
func TestDetermineRunsToDelete_OlderThanRespectsExplicitKeep(t *testing.T) {
	origAll, origKeep, origExplicit := cleanAll, cleanKeepLast, cleanKeepExplicit
	t.Cleanup(func() { cleanAll, cleanKeepLast, cleanKeepExplicit = origAll, origKeep, origExplicit })

	cleanAll = false
	cleanKeepLast = 10
	cleanKeepExplicit = true

	rel := runInState(t, "run-old", release.StatePublished, time.Now().Add(-90*24*time.Hour))
	repo := newCleanTestRepo(rel)

	result, err := determineRunsToDelete(context.Background(), repo, repo.ids, 30*24*time.Hour, true)
	if err != nil {
		t.Fatalf("determineRunsToDelete() error = %v", err)
	}
	if result.DeletedCount != 0 {
		t.Errorf("DeletedCount = %d, want 0; an explicit --keep 10 should still protect the newest run", result.DeletedCount)
	}
}

// Active runs are never deleted by the bulk criteria either.
func TestDetermineRunsToDelete_SkipsActive(t *testing.T) {
	origAll, origKeep, origExplicit := cleanAll, cleanKeepLast, cleanKeepExplicit
	t.Cleanup(func() { cleanAll, cleanKeepLast, cleanKeepExplicit = origAll, origKeep, origExplicit })

	cleanAll = false
	cleanKeepLast = 10
	cleanKeepExplicit = false

	rel := runInState(t, "run-active", release.StateDraft, time.Now().Add(-90*24*time.Hour))
	repo := newCleanTestRepo(rel)

	result, err := determineRunsToDelete(context.Background(), repo, repo.ids, 30*24*time.Hour, true)
	if err != nil {
		t.Fatalf("determineRunsToDelete() error = %v", err)
	}
	if result.DeletedCount != 0 {
		t.Errorf("DeletedCount = %d, want 0 for an in-progress run", result.DeletedCount)
	}
	if result.SkippedActive != 1 {
		t.Errorf("SkippedActive = %d, want 1", result.SkippedActive)
	}
}
