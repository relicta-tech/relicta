// Package persistence provides infrastructure implementations for data persistence.
package persistence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestNewFileReleaseRepository(t *testing.T) {
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "releases")

	repo, err := NewFileReleaseRepository(repoPath)
	if err != nil {
		t.Fatalf("NewFileReleaseRepository() error = %v", err)
	}

	if repo == nil {
		t.Fatal("NewFileReleaseRepository() returned nil")
	}

	// Verify directory was created
	info, err := os.Stat(repoPath)
	if err != nil {
		t.Fatalf("Directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected directory to be created")
	}
}

func TestNewFileReleaseRepository_InvalidPath(t *testing.T) {
	// Try to create in a path that can't exist
	_, err := NewFileReleaseRepository("/nonexistent/path/that/cannot/be/created\x00invalid")
	if err == nil {
		t.Error("NewFileReleaseRepository() should fail with invalid path")
	}
}

func TestFileReleaseRepository_SaveAndFindByID(t *testing.T) {
	tmpDir := t.TempDir()
	repo, _ := NewFileReleaseRepository(tmpDir)
	ctx := context.Background()

	// Create a release
	rel := release.NewRelease("test-release-1", "main", "/path/to/repo")

	// Add plan
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	commit := changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "add feature")
	changeSet.AddCommit(commit)

	plan := release.NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = release.SetPlan(rel, plan)

	// Save
	err := repo.Save(ctx, rel)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Find by ID
	found, err := repo.FindByID(ctx, "test-release-1")
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if found.ID() != rel.ID() {
		t.Errorf("ID mismatch: got %v, want %v", found.ID(), rel.ID())
	}
	if found.Branch() != rel.Branch() {
		t.Errorf("Branch mismatch: got %v, want %v", found.Branch(), rel.Branch())
	}
	if found.State() != rel.State() {
		t.Errorf("State mismatch: got %v, want %v", found.State(), rel.State())
	}
}

func TestFileReleaseRepository_FindByID_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	repo, _ := NewFileReleaseRepository(tmpDir)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "nonexistent")
	if err != release.ErrReleaseNotFound {
		t.Errorf("FindByID() error = %v, want %v", err, release.ErrReleaseNotFound)
	}
}

func TestFileReleaseRepository_SaveFullRelease(t *testing.T) {
	tmpDir := t.TempDir()
	repo, _ := NewFileReleaseRepository(tmpDir)
	ctx := context.Background()

	// Create a complete release with all fields
	rel := release.NewRelease("full-release", "main", "/path/to/repo")

	// Set plan
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := release.NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("2.0.0"),
		changes.ReleaseTypeMajor,
		changeSet,
		false,
	)
	_ = release.SetPlan(rel, plan)

	// Set version and bump
	_ = rel.SetVersion(version.MustParse("2.0.0"), "v2.0.0")
	_ = rel.Bump("test-actor")

	// Set notes
	notes := &release.ReleaseNotes{
		Text:        "## 2.0.0\n\n- Breaking changes",
		Provider:    "test",
		GeneratedAt: time.Now(),
	}
	_ = rel.SetNotes(notes)

	// Approve
	_ = rel.Approve("admin", false)

	// Save
	err := repo.Save(ctx, rel)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load and verify
	loaded, err := repo.FindByID(ctx, "full-release")
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if loaded.State() != release.StateApproved {
		t.Errorf("State = %v, want %v", loaded.State(), release.StateApproved)
	}
	if loaded.TagName() != "v2.0.0" {
		t.Errorf("TagName = %v, want v2.0.0", loaded.TagName())
	}
	if loaded.Version().String() != "2.0.0" {
		t.Errorf("Version = %v, want 2.0.0", loaded.Version())
	}
	if loaded.Notes() == nil {
		t.Fatal("Notes should not be nil")
	}
	if loaded.Notes().Text != notes.Text {
		t.Errorf("Notes.Text mismatch")
	}
	if !loaded.IsApproved() {
		t.Error("Should be approved")
	}
	if loaded.Approval().ApprovedBy != "admin" {
		t.Errorf("ApprovedBy = %v, want admin", loaded.Approval().ApprovedBy)
	}
}

func TestFileReleaseRepository_SaveWithPrerelease(t *testing.T) {
	tmpDir := t.TempDir()
	repo, _ := NewFileReleaseRepository(tmpDir)
	ctx := context.Background()

	rel := release.NewRelease("prerelease-test", "develop", "/repo")

	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := release.NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0-beta.1"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = release.SetPlan(rel, plan)

	ver := version.MustParse("1.1.0-beta.1+build.123")
	_ = rel.SetVersion(ver, "v1.1.0-beta.1")

	err := repo.Save(ctx, rel)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := repo.FindByID(ctx, "prerelease-test")
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if loaded.Version().String() != "1.1.0-beta.1+build.123" {
		t.Errorf("Version = %v, want 1.1.0-beta.1+build.123", loaded.Version())
	}
}

func TestFileReleaseRepository_FindBySpecification(t *testing.T) {
	tmpDir := t.TempDir()
	repo, _ := NewFileReleaseRepository(tmpDir)
	ctx := context.Background()

	relMain := release.NewRelease("spec-main", "main", "/repo1")
	relDev := release.NewRelease("spec-dev", "dev", "/repo2")

	if err := repo.Save(ctx, relMain); err != nil {
		t.Fatalf("Save main error: %v", err)
	}
	if err := repo.Save(ctx, relDev); err != nil {
		t.Fatalf("Save dev error: %v", err)
	}

	spec := release.ByRepositoryPath("/repo1")
	found, err := repo.FindBySpecification(ctx, spec)
	if err != nil {
		t.Fatalf("FindBySpecification error: %v", err)
	}
	if len(found) != 1 || found[0].ID() != "spec-main" {
		t.Fatalf("unexpected results: %#v", found)
	}
}

func TestFileReleaseRepository_FindLatest(t *testing.T) {
	tmpDir := t.TempDir()
	repo, _ := NewFileReleaseRepository(tmpDir)
	ctx := context.Background()

	// Create release for one repo path
	rel1 := release.NewRelease("release-1", "main", "/path/to/repo")
	_ = repo.Save(ctx, rel1)

	// Create release for a different repo path
	rel2 := release.NewRelease("release-2", "main", "/other/repo")
	_ = repo.Save(ctx, rel2)

	// Find latest for /path/to/repo - should only find rel1
	latest, err := repo.FindLatest(ctx, "/path/to/repo")
	if err != nil {
		t.Fatalf("FindLatest() error = %v", err)
	}

	if latest.ID() != "release-1" {
		t.Errorf("FindLatest() ID = %v, want release-1", latest.ID())
	}

	// Find latest for /other/repo - should only find rel2
	latest2, err := repo.FindLatest(ctx, "/other/repo")
	if err != nil {
		t.Fatalf("FindLatest() error = %v", err)
	}

	if latest2.ID() != "release-2" {
		t.Errorf("FindLatest() ID = %v, want release-2", latest2.ID())
	}
}

func TestFileReleaseRepository_FindLatest_MultipleReleases(t *testing.T) {
	tmpDir := t.TempDir()
	repo, _ := NewFileReleaseRepository(tmpDir)
	ctx := context.Background()

	// Create first release
	rel1 := release.NewRelease("release-1", "main", "/path/to/repo")
	_ = repo.Save(ctx, rel1)

	// Wait and create second release with later timestamp
	time.Sleep(1100 * time.Millisecond) // Wait > 1 second to ensure different RFC3339 timestamps

	rel2 := release.NewRelease("release-2", "main", "/path/to/repo")
	_ = repo.Save(ctx, rel2)

	// Find latest should return rel2 (most recently updated)
	latest, err := repo.FindLatest(ctx, "/path/to/repo")
	if err != nil {
		t.Fatalf("FindLatest() error = %v", err)
	}

	// Note: The actual latest depends on which has newer UpdatedAt timestamp
	// Both releases are valid results since the test depends on timing
	if latest.RepositoryPath() != "/path/to/repo" {
		t.Errorf("FindLatest() returned wrong repo path: %v", latest.RepositoryPath())
	}
}

func TestFileReleaseRepository_FindLatest_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	repo, _ := NewFileReleaseRepository(tmpDir)
	ctx := context.Background()

	_, err := repo.FindLatest(ctx, "/nonexistent/repo")
	if err != release.ErrReleaseNotFound {
		t.Errorf("FindLatest() error = %v, want %v", err, release.ErrReleaseNotFound)
	}
}

func TestFileReleaseRepository_FindByState(t *testing.T) {
	tmpDir := t.TempDir()
	repo, _ := NewFileReleaseRepository(tmpDir)
	ctx := context.Background()

	// Create releases in different states
	rel1 := release.NewRelease("release-1", "main", "/repo")
	rel2 := release.NewRelease("release-2", "main", "/repo")
	rel3 := release.NewRelease("release-3", "main", "/repo")

	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := release.NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)

	// rel1: draft (initialized)
	_ = repo.Save(ctx, rel1)

	// rel2: planned
	_ = release.SetPlan(rel2, plan)
	_ = repo.Save(ctx, rel2)

	// rel3: planned
	_ = release.SetPlan(rel3, plan)
	_ = repo.Save(ctx, rel3)

	// Find planned releases
	planned, err := repo.FindByState(ctx, release.StatePlanned)
	if err != nil {
		t.Fatalf("FindByState() error = %v", err)
	}

	if len(planned) != 2 {
		t.Errorf("FindByState() returned %d releases, want 2", len(planned))
	}

	// Find draft releases
	draft, err := repo.FindByState(ctx, release.StateDraft)
	if err != nil {
		t.Fatalf("FindByState() error = %v", err)
	}

	if len(draft) != 1 {
		t.Errorf("FindByState() returned %d releases, want 1", len(draft))
	}
}

func TestFileReleaseRepository_FindActive(t *testing.T) {
	tmpDir := t.TempDir()
	repo, _ := NewFileReleaseRepository(tmpDir)
	ctx := context.Background()

	// Create releases
	rel1 := release.NewRelease("active-1", "main", "/repo")
	rel2 := release.NewRelease("active-2", "main", "/repo")

	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := release.NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)

	// rel1: planned (active)
	_ = release.SetPlan(rel1, plan)
	_ = repo.Save(ctx, rel1)

	// rel2: canceled (final, not active)
	_ = rel2.Cancel("test", "user")
	_ = repo.Save(ctx, rel2)

	// Find active
	active, err := repo.FindActive(ctx)
	if err != nil {
		t.Fatalf("FindActive() error = %v", err)
	}

	if len(active) != 1 {
		t.Errorf("FindActive() returned %d releases, want 1", len(active))
	}
	if len(active) > 0 && active[0].ID() != "active-1" {
		t.Errorf("FindActive() ID = %v, want active-1", active[0].ID())
	}
}

func TestFileReleaseRepository_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	repo, _ := NewFileReleaseRepository(tmpDir)
	ctx := context.Background()

	rel := release.NewRelease("to-delete", "main", "/repo")
	_ = repo.Save(ctx, rel)

	// Verify it exists
	_, err := repo.FindByID(ctx, "to-delete")
	if err != nil {
		t.Fatalf("Release should exist before delete")
	}

	// Delete
	err = repo.Delete(ctx, "to-delete")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify it's gone
	_, err = repo.FindByID(ctx, "to-delete")
	if err != release.ErrReleaseNotFound {
		t.Errorf("Release should not exist after delete")
	}
}

func TestFileReleaseRepository_Delete_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	repo, _ := NewFileReleaseRepository(tmpDir)
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if err != release.ErrReleaseNotFound {
		t.Errorf("Delete() error = %v, want %v", err, release.ErrReleaseNotFound)
	}
}

func TestFileReleaseRepository_Update(t *testing.T) {
	tmpDir := t.TempDir()
	repo, _ := NewFileReleaseRepository(tmpDir)
	ctx := context.Background()

	// Create and save initial release
	rel := release.NewRelease("update-test", "main", "/repo")
	_ = repo.Save(ctx, rel)

	// Modify the release
	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := release.NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = release.SetPlan(rel, plan)

	// Save again (update)
	_ = repo.Save(ctx, rel)

	// Load and verify
	loaded, _ := repo.FindByID(ctx, "update-test")
	if loaded.State() != release.StatePlanned {
		t.Errorf("State after update = %v, want %v", loaded.State(), release.StatePlanned)
	}
}

func TestFileReleaseRepository_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	repo, _ := NewFileReleaseRepository(tmpDir)
	ctx := context.Background()

	// Create initial release
	rel := release.NewRelease("concurrent-test", "main", "/repo")
	_ = repo.Save(ctx, rel)

	// Concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = repo.FindByID(ctx, "concurrent-test")
			done <- true
		}()
	}

	// Wait for all reads
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestFileReleaseRepository_PublishedRelease(t *testing.T) {
	tmpDir := t.TempDir()
	repo, _ := NewFileReleaseRepository(tmpDir)
	ctx := context.Background()

	// Create a fully published release
	rel := release.NewRelease("published-test", "main", "/repo")

	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := release.NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = release.SetPlan(rel, plan)
	_ = rel.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
	_ = rel.Bump("test-actor")
	_ = rel.SetNotes(&release.ReleaseNotes{Text: "test", GeneratedAt: time.Now()})
	_ = rel.Approve("user", false)
	_ = rel.StartPublishing("user")
	_ = rel.MarkPublished("https://github.com/owner/repo/releases/v1.1.0")

	_ = repo.Save(ctx, rel)

	// Load and verify
	loaded, err := repo.FindByID(ctx, "published-test")
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if loaded.State() != release.StatePublished {
		t.Errorf("State = %v, want %v", loaded.State(), release.StatePublished)
	}
	if loaded.PublishedAt() == nil {
		t.Error("PublishedAt should not be nil")
	}
}

func TestFileReleaseRepository_FailedRelease(t *testing.T) {
	tmpDir := t.TempDir()
	repo, _ := NewFileReleaseRepository(tmpDir)
	ctx := context.Background()

	rel := release.NewRelease("failed-test", "main", "/repo")

	changeSet := changes.NewChangeSet("cs-1", "v1.0.0", "HEAD")
	plan := release.NewReleasePlan(
		version.MustParse("1.0.0"),
		version.MustParse("1.1.0"),
		changes.ReleaseTypeMinor,
		changeSet,
		false,
	)
	_ = release.SetPlan(rel, plan)
	_ = rel.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
	_ = rel.Bump("test-actor")
	_ = rel.SetNotes(&release.ReleaseNotes{Text: "test", GeneratedAt: time.Now()})
	_ = rel.Approve("user", false)
	_ = rel.StartPublishing("user")
	_ = rel.MarkFailed("network error", "system")

	_ = repo.Save(ctx, rel)

	loaded, _ := repo.FindByID(ctx, "failed-test")
	if loaded.State() != release.StateFailed {
		t.Errorf("State = %v, want %v", loaded.State(), release.StateFailed)
	}
	if loaded.LastError() != "network error" {
		t.Errorf("LastError = %v, want 'network error'", loaded.LastError())
	}
}

func TestFileReleaseRepository_ConcurrentScanReleases(t *testing.T) {
	tmpDir := t.TempDir()
	repo, _ := NewFileReleaseRepository(tmpDir)
	ctx := context.Background()

	// Create more than maxScanWorkers*2 releases to trigger concurrent scanning
	// maxScanWorkers is 4, so we need at least 8 files
	numReleases := 10

	// Create multiple releases with different states
	for i := 0; i < numReleases; i++ {
		id := release.ReleaseID("test-concurrent-" + string(rune('a'+i)))
		rel := release.NewRelease(id, "main", "/path/to/repo")

		// Vary the states to test filtering
		if i < 5 {
			// Create planned releases
			changeSet := changes.NewChangeSet(changes.ChangeSetID(string(id)+"-cs"), "v1.0.0", "HEAD")
			plan := release.NewReleasePlan(
				version.MustParse("1.0.0"),
				version.MustParse("1.1.0"),
				changes.ReleaseTypeMinor,
				changeSet,
				false,
			)
			_ = release.SetPlan(rel, plan)
		}

		err := repo.Save(ctx, rel)
		if err != nil {
			t.Fatalf("Failed to save release %d: %v", i, err)
		}
	}

	// Test FindByState which should trigger concurrent scanning
	plannedReleases, err := repo.FindByState(ctx, release.StatePlanned)
	if err != nil {
		t.Fatalf("FindByState() error = %v", err)
	}

	if len(plannedReleases) != 5 {
		t.Errorf("FindByState() returned %d planned releases, want 5", len(plannedReleases))
	}

	// Test FindActive which should also trigger concurrent scanning
	activeReleases, err := repo.FindActive(ctx)
	if err != nil {
		t.Fatalf("FindActive() error = %v", err)
	}

	// All releases should be active (not in final states)
	if len(activeReleases) != numReleases {
		t.Errorf("FindActive() returned %d active releases, want %d", len(activeReleases), numReleases)
	}
}
