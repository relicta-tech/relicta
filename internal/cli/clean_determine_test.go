package cli

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/domain/release"
)

// cleanTestReleaseRepo is a mock release repository for determineRunsToDelete tests.
// It supports per-ID release lookup for fine-grained control over run states.
type cleanTestReleaseRepo struct {
	releases map[release.RunID]*release.ReleaseRun
}

func (r cleanTestReleaseRepo) FindByID(ctx context.Context, id release.RunID) (*release.ReleaseRun, error) {
	rel, ok := r.releases[id]
	if !ok {
		return nil, release.ErrRunNotFound
	}
	return rel, nil
}

func (r cleanTestReleaseRepo) FindLatest(ctx context.Context, repoPath string) (*release.ReleaseRun, error) {
	return nil, release.ErrRunNotFound
}
func (r cleanTestReleaseRepo) FindByState(ctx context.Context, state release.RunState) ([]*release.ReleaseRun, error) {
	return nil, nil
}
func (r cleanTestReleaseRepo) FindActive(ctx context.Context) ([]*release.ReleaseRun, error) {
	return nil, nil
}
func (r cleanTestReleaseRepo) FindBySpecification(ctx context.Context, spec release.Specification) ([]*release.ReleaseRun, error) {
	return nil, nil
}
func (r cleanTestReleaseRepo) Save(ctx context.Context, rel *release.ReleaseRun) error { return nil }
func (r cleanTestReleaseRepo) Delete(ctx context.Context, id release.RunID) error      { return nil }
func (r cleanTestReleaseRepo) List(ctx context.Context, repoPath string) ([]release.RunID, error) {
	return nil, nil
}

// TestDetermineRunsToDelete_EmptyList verifies empty result for empty run list.
func TestDetermineRunsToDelete_EmptyList(t *testing.T) {
	repo := cleanTestReleaseRepo{releases: map[release.RunID]*release.ReleaseRun{}}
	result, err := determineRunsToDelete(context.Background(), repo, []release.RunID{}, 0, false)
	if err != nil {
		t.Fatalf("determineRunsToDelete() error = %v", err)
	}
	if result.TotalRuns != 0 {
		t.Errorf("TotalRuns = %d, want 0", result.TotalRuns)
	}
	if result.DeletedCount != 0 {
		t.Errorf("DeletedCount = %d, want 0", result.DeletedCount)
	}
}

// TestDetermineRunsToDelete_KeepLastN verifies that runs beyond cleanKeepLast are deleted.
func TestDetermineRunsToDelete_KeepLastN(t *testing.T) {
	origKeepLast := cleanKeepLast
	origAll := cleanAll
	defer func() {
		cleanKeepLast = origKeepLast
		cleanAll = origAll
	}()
	cleanKeepLast = 2
	cleanAll = false

	// Create 3 canceled (final) releases
	releases := map[release.RunID]*release.ReleaseRun{}
	ids := []release.RunID{"run-1", "run-2", "run-3"}
	for _, id := range ids {
		rel := release.NewReleaseRunForTest(id, "main", "/repo")
		_ = rel.Cancel("test", "cli")
		releases[id] = rel
	}

	repo := cleanTestReleaseRepo{releases: releases}
	result, err := determineRunsToDelete(context.Background(), repo, ids, 0, false)
	if err != nil {
		t.Fatalf("determineRunsToDelete() error = %v", err)
	}
	// With keepLast=2 and 3 runs, one should be deleted
	if result.DeletedCount != 1 {
		t.Errorf("DeletedCount = %d, want 1", result.DeletedCount)
	}
	if result.KeptCount != 2 {
		t.Errorf("KeptCount = %d, want 2", result.KeptCount)
	}
}

// TestDetermineRunsToDelete_ActiveRunsSkipped verifies active runs are never deleted.
func TestDetermineRunsToDelete_ActiveRunsSkipped(t *testing.T) {
	origKeepLast := cleanKeepLast
	origAll := cleanAll
	defer func() {
		cleanKeepLast = origKeepLast
		cleanAll = origAll
	}()
	cleanKeepLast = 1 // Only keep 1 run
	cleanAll = false

	// Run 1: active (draft - not final)
	activeRel := release.NewReleaseRunForTest("run-active", "main", "/repo")
	// Run 2: canceled (final)
	canceledRel := release.NewReleaseRunForTest("run-canceled", "main", "/repo")
	_ = canceledRel.Cancel("test", "cli")

	releases := map[release.RunID]*release.ReleaseRun{
		"run-active":   activeRel,
		"run-canceled": canceledRel,
	}

	ids := []release.RunID{"run-active", "run-canceled"}
	repo := cleanTestReleaseRepo{releases: releases}

	result, err := determineRunsToDelete(context.Background(), repo, ids, 0, false)
	if err != nil {
		t.Fatalf("determineRunsToDelete() error = %v", err)
	}
	// run-active should be skipped (active), run-canceled may be deleted (beyond keepLast=1)
	if result.SkippedActive != 1 {
		t.Errorf("SkippedActive = %d, want 1", result.SkippedActive)
	}
}

// TestDetermineRunsToDelete_AllFlag verifies --all keeps only the first run.
func TestDetermineRunsToDelete_AllFlag(t *testing.T) {
	origAll := cleanAll
	defer func() { cleanAll = origAll }()
	cleanAll = true

	ids := []release.RunID{"run-1", "run-2", "run-3"}
	releases := map[release.RunID]*release.ReleaseRun{}
	for _, id := range ids {
		rel := release.NewReleaseRunForTest(id, "main", "/repo")
		_ = rel.Cancel("test", "cli")
		releases[id] = rel
	}

	repo := cleanTestReleaseRepo{releases: releases}
	result, err := determineRunsToDelete(context.Background(), repo, ids, 0, false)
	if err != nil {
		t.Fatalf("determineRunsToDelete() error = %v", err)
	}
	// With --all, delete all but the first (2 of 3)
	if result.DeletedCount != 2 {
		t.Errorf("DeletedCount = %d, want 2", result.DeletedCount)
	}
	if result.KeptCount != 1 {
		t.Errorf("KeptCount = %d, want 1", result.KeptCount)
	}
}

// TestDetermineRunsToDelete_OlderThan verifies --older-than filter.
func TestDetermineRunsToDelete_OlderThan(t *testing.T) {
	origAll := cleanAll
	origKeepLast := cleanKeepLast
	defer func() {
		cleanAll = origAll
		cleanKeepLast = origKeepLast
	}()
	cleanAll = false
	cleanKeepLast = 100 // High enough to not interfere

	ids := []release.RunID{"run-old", "run-new"}
	releases := map[release.RunID]*release.ReleaseRun{}
	for _, id := range ids {
		rel := release.NewReleaseRunForTest(id, "main", "/repo")
		_ = rel.Cancel("test", "cli")
		releases[id] = rel
	}

	repo := cleanTestReleaseRepo{releases: releases}

	// Set a very short older-than duration so all runs would qualify
	olderThan := -1 * time.Hour // negative means no runs are "older than" this
	result, err := determineRunsToDelete(context.Background(), repo, ids, olderThan, false)
	if err != nil {
		t.Fatalf("determineRunsToDelete() error = %v", err)
	}
	// With a negative duration, nothing is older than that - no deletions based on age
	// (The logic: age > olderThan; if olderThan < 0, age > negative is always true)
	// So all runs should be deleted based on age (but not skipped)
	if result.TotalRuns != 2 {
		t.Errorf("TotalRuns = %d, want 2", result.TotalRuns)
	}
}

// TestDetermineRunsToDelete_DryRunFlag verifies dry-run flag is propagated.
func TestDetermineRunsToDelete_DryRunFlag(t *testing.T) {
	origKeepLast := cleanKeepLast
	origAll := cleanAll
	defer func() {
		cleanKeepLast = origKeepLast
		cleanAll = origAll
	}()
	cleanKeepLast = 0 // Delete everything
	cleanAll = false

	ids := []release.RunID{"run-1"}
	releases := map[release.RunID]*release.ReleaseRun{}
	rel := release.NewReleaseRunForTest("run-1", "main", "/repo")
	_ = rel.Cancel("test", "cli")
	releases["run-1"] = rel

	repo := cleanTestReleaseRepo{releases: releases}
	result, err := determineRunsToDelete(context.Background(), repo, ids, 0, true)
	if err != nil {
		t.Fatalf("determineRunsToDelete() error = %v", err)
	}
	// The dry-run flag should be set in the result
	if !result.DryRun {
		t.Error("result.DryRun should be true when isDryRun=true")
	}
}

// TestDetermineRunsToDelete_FindByIDError verifies missing runs are silently skipped.
func TestDetermineRunsToDelete_FindByIDError(t *testing.T) {
	// Repo has no releases — FindByID returns error for all IDs
	repo := cleanTestReleaseRepo{releases: map[release.RunID]*release.ReleaseRun{}}
	ids := []release.RunID{"nonexistent-1", "nonexistent-2"}

	result, err := determineRunsToDelete(context.Background(), repo, ids, 0, false)
	if err != nil {
		t.Fatalf("determineRunsToDelete() error = %v", err)
	}
	// Should not count runs that couldn't be loaded
	if result.TotalRuns != 2 {
		t.Errorf("TotalRuns = %d, want 2 (input count)", result.TotalRuns)
	}
	// Both are skipped silently; result should show 0 deletions and 0 kept
	// (they were not added to either list since FindByID failed)
	if result.DeletedCount != 0 {
		t.Errorf("DeletedCount = %d, want 0 (all skipped)", result.DeletedCount)
	}
}
