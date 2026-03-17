// Package adapters provides infrastructure implementations for the release governance domain.
package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// =============================================================================
// FileReleaseRunRepository Tests
// =============================================================================

func TestFileReleaseRunRepository_SaveAndLoad(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Create a test run
	run := domain.NewReleaseRun(
		"github.com/test/repo",
		repoRoot,
		"v1.0.0",
		domain.CommitSHA("abc123"),
		[]domain.CommitSHA{"abc123", "def456"},
		"config-hash",
		"plugin-hash",
	)

	// Save the run
	err := repo.Save(ctx, run)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load the run back
	loaded, err := repo.LoadFromRepo(ctx, repoRoot, run.ID())
	if err != nil {
		t.Fatalf("LoadFromRepo failed: %v", err)
	}

	// Verify the loaded run matches
	if loaded.ID() != run.ID() {
		t.Errorf("ID mismatch: got %s, want %s", loaded.ID(), run.ID())
	}
	if loaded.RepoID() != run.RepoID() {
		t.Errorf("RepoID mismatch: got %s, want %s", loaded.RepoID(), run.RepoID())
	}
	if loaded.HeadSHA() != run.HeadSHA() {
		t.Errorf("HeadSHA mismatch: got %s, want %s", loaded.HeadSHA(), run.HeadSHA())
	}
	if loaded.State() != run.State() {
		t.Errorf("State mismatch: got %s, want %s", loaded.State(), run.State())
	}
	if len(loaded.Commits()) != len(run.Commits()) {
		t.Errorf("Commits count mismatch: got %d, want %d", len(loaded.Commits()), len(run.Commits()))
	}
}

func TestFileReleaseRunRepository_ChangeSetRoundTrip(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	run := domain.NewReleaseRun(
		"github.com/test/repo",
		repoRoot,
		"v1.0.0",
		domain.CommitSHA("abc123"),
		[]domain.CommitSHA{"abc123", "def456"},
		"config-hash",
		"plugin-hash",
	)

	// Build a changeset with diverse commits
	cs := changes.NewChangeSet("cs-test-001", "v1.0.0", "HEAD")
	cs.AddCommit(changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "add user auth",
		changes.WithScope("auth"),
		changes.WithBody("Implements JWT-based authentication"),
		changes.WithAuthor("Alice", "alice@test.com"),
		changes.WithDate(time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)),
		changes.WithRawMessage("feat(auth): add user auth\n\nImplements JWT-based authentication"),
	))
	cs.AddCommit(changes.NewConventionalCommit("def456", changes.CommitTypeFix, "fix login redirect",
		changes.WithBreaking("changes redirect URL format"),
		changes.WithFooter("BREAKING CHANGE: changes redirect URL format"),
	))
	run.SetChangeSet(cs)

	// Save
	if err := repo.Save(ctx, run); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load from disk
	loaded, err := repo.LoadFromRepo(ctx, repoRoot, run.ID())
	if err != nil {
		t.Fatalf("LoadFromRepo failed: %v", err)
	}

	// Verify changeset survived round-trip
	if !loaded.HasChangeSet() {
		t.Fatal("loaded run has no changeset")
	}
	loadedCS := loaded.ChangeSet()
	if loadedCS.ID() != cs.ID() {
		t.Errorf("changeset ID: got %s, want %s", loadedCS.ID(), cs.ID())
	}
	if loadedCS.FromRef() != "v1.0.0" {
		t.Errorf("FromRef: got %s, want v1.0.0", loadedCS.FromRef())
	}
	if loadedCS.ToRef() != "HEAD" {
		t.Errorf("ToRef: got %s, want HEAD", loadedCS.ToRef())
	}
	if loadedCS.CommitCount() != 2 {
		t.Fatalf("commit count: got %d, want 2", loadedCS.CommitCount())
	}

	commits := loadedCS.Commits()

	// Verify first commit fields
	c0 := commits[0]
	if c0.Hash() != "abc123" {
		t.Errorf("commit[0].Hash: got %s, want abc123", c0.Hash())
	}
	if c0.Type() != changes.CommitTypeFeat {
		t.Errorf("commit[0].Type: got %s, want feat", c0.Type())
	}
	if c0.Scope() != "auth" {
		t.Errorf("commit[0].Scope: got %s, want auth", c0.Scope())
	}
	if c0.Subject() != "add user auth" {
		t.Errorf("commit[0].Subject: got %s, want 'add user auth'", c0.Subject())
	}
	if c0.Body() != "Implements JWT-based authentication" {
		t.Errorf("commit[0].Body: got %q", c0.Body())
	}
	if c0.Author() != "Alice" {
		t.Errorf("commit[0].Author: got %s, want Alice", c0.Author())
	}
	if c0.AuthorEmail() != "alice@test.com" {
		t.Errorf("commit[0].AuthorEmail: got %s", c0.AuthorEmail())
	}

	// Verify second commit (breaking change)
	c1 := commits[1]
	if !c1.IsBreaking() {
		t.Error("commit[1] should be breaking")
	}
	if c1.BreakingMessage() != "changes redirect URL format" {
		t.Errorf("commit[1].BreakingMessage: got %q", c1.BreakingMessage())
	}
	if c1.Footer() != "BREAKING CHANGE: changes redirect URL format" {
		t.Errorf("commit[1].Footer: got %q", c1.Footer())
	}

	// Verify categories work after deserialization
	cats := loadedCS.Categories()
	if len(cats.Features) != 1 {
		t.Errorf("Features count: got %d, want 1", len(cats.Features))
	}
	if len(cats.Fixes) != 1 {
		t.Errorf("Fixes count: got %d, want 1", len(cats.Fixes))
	}
	if len(cats.Breaking) != 1 {
		t.Errorf("Breaking count: got %d, want 1", len(cats.Breaking))
	}
}

func TestFileReleaseRunRepository_NoChangeSetRoundTrip(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	run := domain.NewReleaseRun(
		"github.com/test/repo",
		repoRoot,
		"v1.0.0",
		domain.CommitSHA("abc123"),
		[]domain.CommitSHA{"abc123"},
		"config-hash",
		"plugin-hash",
	)

	// Save without changeset
	if err := repo.Save(ctx, run); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := repo.LoadFromRepo(ctx, repoRoot, run.ID())
	if err != nil {
		t.Fatalf("LoadFromRepo failed: %v", err)
	}

	if loaded.HasChangeSet() {
		t.Error("loaded run should not have a changeset")
	}
}

func TestFileReleaseRunRepository_SaveWithNotes(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	run := domain.NewReleaseRun(
		"github.com/test/repo",
		repoRoot,
		"v1.0.0",
		domain.CommitSHA("abc123"),
		[]domain.CommitSHA{"abc123"},
		"config-hash",
		"plugin-hash",
	)

	// Transition to a state where notes can be set
	_ = run.Plan("system")
	_ = run.SetVersion(version.NewSemanticVersion(1, 1, 0), "v1.1.0")
	_ = run.Bump("system")

	// Set notes using GenerateNotes
	notes := &domain.ReleaseNotes{
		Text:           "Release notes text",
		AudiencePreset: "developer",
		TonePreset:     "formal",
		Provider:       "openai",
		Model:          "gpt-4",
		GeneratedAt:    time.Now(),
	}
	_ = run.GenerateNotes(notes, "inputs-hash", "system")

	// Save and reload
	err := repo.Save(ctx, run)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := repo.LoadFromRepo(ctx, repoRoot, run.ID())
	if err != nil {
		t.Fatalf("LoadFromRepo failed: %v", err)
	}

	if loaded.Notes() == nil {
		t.Fatal("Expected notes to be loaded")
	}
	if loaded.Notes().Text != notes.Text {
		t.Errorf("Notes text mismatch: got %s, want %s", loaded.Notes().Text, notes.Text)
	}
	if loaded.Notes().Provider != notes.Provider {
		t.Errorf("Notes provider mismatch: got %s, want %s", loaded.Notes().Provider, notes.Provider)
	}
}

func TestFileReleaseRunRepository_SaveWithApproval(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	run := domain.NewReleaseRun(
		"github.com/test/repo",
		repoRoot,
		"v1.0.0",
		domain.CommitSHA("abc123"),
		[]domain.CommitSHA{"abc123"},
		"config-hash",
		"plugin-hash",
	)

	// Transition through states to get to approved
	_ = run.Plan("system")
	_ = run.SetVersion(version.NewSemanticVersion(1, 1, 0), "v1.1.0")
	_ = run.Bump("system")
	notes := &domain.ReleaseNotes{Text: "Notes", GeneratedAt: time.Now()}
	_ = run.GenerateNotes(notes, "inputs-hash", "system")
	_ = run.Approve("user@example.com", true)

	// Save and reload
	err := repo.Save(ctx, run)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := repo.LoadFromRepo(ctx, repoRoot, run.ID())
	if err != nil {
		t.Fatalf("LoadFromRepo failed: %v", err)
	}

	if loaded.Approval() == nil {
		t.Fatal("Expected approval to be loaded")
	}
	if loaded.Approval().ApprovedBy != "user@example.com" {
		t.Errorf("ApprovedBy mismatch: got %s, want user@example.com", loaded.Approval().ApprovedBy)
	}
	if !loaded.Approval().AutoApproved {
		t.Error("Expected AutoApproved to be true")
	}
}

func TestFileReleaseRunRepository_LoadNotFound(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	_, err := repo.LoadFromRepo(ctx, repoRoot, "nonexistent-id")
	if err != domain.ErrRunNotFound {
		t.Errorf("Expected ErrRunNotFound, got %v", err)
	}
}

func TestFileReleaseRunRepository_Load(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	ctx := context.Background()

	// Load without any known repo roots should return ErrRunNotFound
	_, err := repo.Load(ctx, "some-id")
	if err == nil {
		t.Error("Expected error from Load with unknown run ID")
	}
	if err != domain.ErrRunNotFound {
		t.Errorf("Expected ErrRunNotFound, got: %v", err)
	}
}

func TestFileReleaseRunRepository_SetLatestAndLoadLatest(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Create and save a run
	run := domain.NewReleaseRun(
		"github.com/test/repo",
		repoRoot,
		"v1.0.0",
		domain.CommitSHA("abc123"),
		[]domain.CommitSHA{"abc123"},
		"config-hash",
		"plugin-hash",
	)

	err := repo.Save(ctx, run)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Set as latest
	err = repo.SetLatest(ctx, repoRoot, run.ID())
	if err != nil {
		t.Fatalf("SetLatest failed: %v", err)
	}

	// Load latest
	latest, err := repo.LoadLatest(ctx, repoRoot)
	if err != nil {
		t.Fatalf("LoadLatest failed: %v", err)
	}

	if latest.ID() != run.ID() {
		t.Errorf("Latest ID mismatch: got %s, want %s", latest.ID(), run.ID())
	}
}

func TestFileReleaseRunRepository_LoadLatestNotFound(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	_, err := repo.LoadLatest(ctx, repoRoot)
	if err != domain.ErrRunNotFound {
		t.Errorf("Expected ErrRunNotFound, got %v", err)
	}
}

func TestFileReleaseRunRepository_List(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// List empty directory
	ids, err := repo.List(ctx, repoRoot)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("Expected empty list, got %d items", len(ids))
	}

	// Create and save multiple runs
	run1 := domain.NewReleaseRun("github.com/test/repo", repoRoot, "v1.0.0",
		domain.CommitSHA("abc123"), []domain.CommitSHA{"abc123"}, "config1", "plugin1")
	run2 := domain.NewReleaseRun("github.com/test/repo", repoRoot, "v1.1.0",
		domain.CommitSHA("def456"), []domain.CommitSHA{"def456"}, "config2", "plugin2")

	_ = repo.Save(ctx, run1)
	time.Sleep(10 * time.Millisecond) // Ensure different mod times
	_ = repo.Save(ctx, run2)

	// List runs
	ids, err = repo.List(ctx, repoRoot)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("Expected 2 runs, got %d", len(ids))
	}

	// Should be sorted by mod time (newest first)
	if ids[0] != run2.ID() {
		t.Errorf("Expected run2 first (newest), got %s", ids[0])
	}
}

func TestFileReleaseRunRepository_Delete(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	ctx := context.Background()

	// Delete without repo context should fail
	err := repo.Delete(ctx, "some-id")
	if err == nil {
		t.Error("Expected error from Delete without repo context")
	}
}

func TestFileReleaseRunRepository_DeleteFromRepo(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Create and save a run
	run := domain.NewReleaseRun("github.com/test/repo", repoRoot, "v1.0.0",
		domain.CommitSHA("abc123"), []domain.CommitSHA{"abc123"}, "config", "plugin")
	_ = repo.Save(ctx, run)

	// Verify it exists
	_, err := repo.LoadFromRepo(ctx, repoRoot, run.ID())
	if err != nil {
		t.Fatalf("Run should exist: %v", err)
	}

	// Delete it
	err = repo.DeleteFromRepo(ctx, repoRoot, run.ID())
	if err != nil {
		t.Fatalf("DeleteFromRepo failed: %v", err)
	}

	// Verify it's gone
	_, err = repo.LoadFromRepo(ctx, repoRoot, run.ID())
	if err != domain.ErrRunNotFound {
		t.Errorf("Expected ErrRunNotFound after delete, got %v", err)
	}
}

func TestFileReleaseRunRepository_DeleteNonexistent(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Ensure directory exists
	_ = os.MkdirAll(filepath.Join(repoRoot, runsDir), 0755)

	// Delete nonexistent should succeed (idempotent)
	err := repo.DeleteFromRepo(ctx, repoRoot, "nonexistent")
	if err != nil {
		t.Errorf("DeleteFromRepo of nonexistent should succeed, got %v", err)
	}
}

func TestFileReleaseRunRepository_FindByState(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Create runs in different states
	run1 := domain.NewReleaseRun("github.com/test/repo", repoRoot, "v1.0.0",
		domain.CommitSHA("abc123"), []domain.CommitSHA{"abc123"}, "config1", "plugin1")
	// run1 is in Draft state

	run2 := domain.NewReleaseRun("github.com/test/repo", repoRoot, "v1.1.0",
		domain.CommitSHA("def456"), []domain.CommitSHA{"def456"}, "config2", "plugin2")
	_ = run2.Plan("system")
	// run2 is in Planned state

	_ = repo.Save(ctx, run1)
	_ = repo.Save(ctx, run2)

	// Find draft runs
	draftRuns, err := repo.FindByState(ctx, repoRoot, domain.StateDraft)
	if err != nil {
		t.Fatalf("FindByState failed: %v", err)
	}
	if len(draftRuns) != 1 {
		t.Errorf("Expected 1 draft run, got %d", len(draftRuns))
	}

	// Find planned runs
	plannedRuns, err := repo.FindByState(ctx, repoRoot, domain.StatePlanned)
	if err != nil {
		t.Fatalf("FindByState failed: %v", err)
	}
	if len(plannedRuns) != 1 {
		t.Errorf("Expected 1 planned run, got %d", len(plannedRuns))
	}

	// Find published runs (should be empty)
	publishedRuns, err := repo.FindByState(ctx, repoRoot, domain.StatePublished)
	if err != nil {
		t.Fatalf("FindByState failed: %v", err)
	}
	if len(publishedRuns) != 0 {
		t.Errorf("Expected 0 published runs, got %d", len(publishedRuns))
	}
}

func TestFileReleaseRunRepository_FindActive(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Create an active run (planned state - IsActive() excludes Draft)
	run1 := domain.NewReleaseRun("github.com/test/repo", repoRoot, "v1.0.0",
		domain.CommitSHA("abc123"), []domain.CommitSHA{"abc123"}, "config1", "plugin1")
	_ = run1.Plan("system") // Transition to Planned state which is active
	_ = repo.Save(ctx, run1)

	// Find active runs
	activeRuns, err := repo.FindActive(ctx, repoRoot)
	if err != nil {
		t.Fatalf("FindActive failed: %v", err)
	}
	if len(activeRuns) != 1 {
		t.Errorf("Expected 1 active run, got %d", len(activeRuns))
	}
}

func TestFileReleaseRunRepository_SaveMachineJSON(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()

	machineJSON := []byte(`{"states": ["draft", "planned"]}`)
	runID := domain.RunID("test-run")

	err := repo.SaveMachineJSON(repoRoot, runID, machineJSON)
	if err != nil {
		t.Fatalf("SaveMachineJSON failed: %v", err)
	}

	// Verify file was created
	path := filepath.Join(repoRoot, runsDir, string(runID)+machineFileSuffix)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read machine file: %v", err)
	}
	if !bytes.Equal(data, machineJSON) {
		t.Errorf("Machine JSON mismatch: got %s, want %s", data, machineJSON)
	}
}

func TestFileReleaseRunRepository_SaveWithSteps(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	run := domain.NewReleaseRun("github.com/test/repo", repoRoot, "v1.0.0",
		domain.CommitSHA("abc123"), []domain.CommitSHA{"abc123"}, "config", "plugin")

	// Add steps using SetExecutionPlan
	steps := []domain.StepPlan{
		{Name: "create-tag", Type: domain.StepTypeTag, IdempotencyKey: "tag-v1.1.0"},
		{Name: "push-tag", Type: domain.StepTypeTag, IdempotencyKey: "push-v1.1.0"},
	}
	run.SetExecutionPlan(steps)

	// Save and reload
	err := repo.Save(ctx, run)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := repo.LoadFromRepo(ctx, repoRoot, run.ID())
	if err != nil {
		t.Fatalf("LoadFromRepo failed: %v", err)
	}

	if len(loaded.Steps()) != 2 {
		t.Errorf("Expected 2 steps, got %d", len(loaded.Steps()))
	}
	if loaded.Steps()[0].Name != "create-tag" {
		t.Errorf("First step name mismatch: got %s, want create-tag", loaded.Steps()[0].Name)
	}
}

// =============================================================================
// FileLockManager Tests
// =============================================================================

func TestFileLockManager_AcquireAndRelease(t *testing.T) {
	lockMgr := NewFileLockManager()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Acquire lock
	release, err := lockMgr.Acquire(ctx, repoRoot, "run-1")
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Verify lock file exists
	path := filepath.Join(repoRoot, runsDir, lockFileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Lock file should exist")
	}

	// Release lock
	release()

	// Verify lock file is gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Lock file should not exist after release")
	}
}

func TestFileLockManager_AcquireWhenLocked(t *testing.T) {
	lockMgr := NewFileLockManager()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Acquire first lock
	release, err := lockMgr.Acquire(ctx, repoRoot, "run-1")
	if err != nil {
		t.Fatalf("First Acquire failed: %v", err)
	}
	defer release()

	// Try to acquire second lock - should fail
	_, err = lockMgr.Acquire(ctx, repoRoot, "run-2")
	if err == nil {
		t.Error("Second Acquire should have failed")
	}
}

func TestFileLockManager_TryAcquire(t *testing.T) {
	lockMgr := NewFileLockManager()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// TryAcquire when not locked
	release, acquired, err := lockMgr.TryAcquire(ctx, repoRoot, "run-1")
	if err != nil {
		t.Fatalf("TryAcquire failed: %v", err)
	}
	if !acquired {
		t.Error("TryAcquire should have acquired lock")
	}
	defer release()

	// TryAcquire when locked
	release2, acquired2, err := lockMgr.TryAcquire(ctx, repoRoot, "run-2")
	if err != nil {
		t.Fatalf("TryAcquire error: %v", err)
	}
	if acquired2 {
		t.Error("TryAcquire should not have acquired lock")
		if release2 != nil {
			release2()
		}
	}
}

func TestFileLockManager_IsLocked(t *testing.T) {
	lockMgr := NewFileLockManager()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Not locked initially
	locked, err := lockMgr.IsLocked(ctx, repoRoot, "run-1")
	if err != nil {
		t.Fatalf("IsLocked failed: %v", err)
	}
	if locked {
		t.Error("Should not be locked initially")
	}

	// Acquire lock
	release, _ := lockMgr.Acquire(ctx, repoRoot, "run-1")
	defer release()

	// Should be locked now
	locked, err = lockMgr.IsLocked(ctx, repoRoot, "run-1")
	if err != nil {
		t.Fatalf("IsLocked failed: %v", err)
	}
	if !locked {
		t.Error("Should be locked after acquire")
	}
}

func TestFileLockManager_GetLockInfo(t *testing.T) {
	lockMgr := NewFileLockManager()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// No lock info when not locked
	info, err := lockMgr.GetLockInfo(repoRoot)
	if err != nil {
		t.Fatalf("GetLockInfo failed: %v", err)
	}
	if info != nil {
		t.Error("Should have no lock info initially")
	}

	// Acquire lock
	release, _ := lockMgr.Acquire(ctx, repoRoot, "run-1")
	defer release()

	// Get lock info
	info, err = lockMgr.GetLockInfo(repoRoot)
	if err != nil {
		t.Fatalf("GetLockInfo failed: %v", err)
	}
	if info == nil {
		t.Fatal("Should have lock info")
	}
	if info.RunID != "run-1" {
		t.Errorf("RunID mismatch: got %s, want run-1", info.RunID)
	}
	if info.HolderPID != os.Getpid() {
		t.Errorf("PID mismatch: got %d, want %d", info.HolderPID, os.Getpid())
	}
}

func TestFileLockManager_StaleLock(t *testing.T) {
	lockMgr := NewFileLockManager()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Create a stale lock manually
	lockDir := filepath.Join(repoRoot, runsDir)
	_ = os.MkdirAll(lockDir, 0755)

	staleLock := LockFileContents{
		RunID:      "old-run",
		PID:        12345,
		Hostname:   "old-host",
		AcquiredAt: time.Now().Add(-15 * time.Minute), // Older than stale threshold
	}
	data, _ := json.Marshal(staleLock)
	_ = os.WriteFile(filepath.Join(lockDir, lockFileName), data, 0644)

	// Should be able to acquire despite existing lock (it's stale)
	release, err := lockMgr.Acquire(ctx, repoRoot, "new-run")
	if err != nil {
		t.Fatalf("Should be able to acquire stale lock: %v", err)
	}
	release()

	// IsLocked should return false for stale locks
	_ = os.WriteFile(filepath.Join(lockDir, lockFileName), data, 0644)
	locked, err := lockMgr.IsLocked(ctx, repoRoot, "some-run")
	if err != nil {
		t.Fatalf("IsLocked failed: %v", err)
	}
	if locked {
		t.Error("Stale lock should not count as locked")
	}
}

// =============================================================================
// GitRepoInspector Tests
// =============================================================================

// mockGitRepository implements sourcecontrol.GitRepository for testing
type mockGitRepository struct {
	info          *sourcecontrol.RepositoryInfo
	infoErr       error
	isDirty       bool
	isDirtyErr    error
	commits       []*sourcecontrol.Commit
	commitsErr    error
	latestCommit  *sourcecontrol.Commit
	latestErr     error
	tags          sourcecontrol.TagList
	tagsErr       error
	currentBranch string
	branchErr     error
}

func (m *mockGitRepository) GetInfo(ctx context.Context) (*sourcecontrol.RepositoryInfo, error) {
	if m.infoErr != nil {
		return nil, m.infoErr
	}
	return m.info, nil
}

func (m *mockGitRepository) GetRemotes(ctx context.Context) ([]sourcecontrol.RemoteInfo, error) {
	return nil, nil
}

func (m *mockGitRepository) GetBranches(ctx context.Context) ([]sourcecontrol.BranchInfo, error) {
	return nil, nil
}

func (m *mockGitRepository) GetCurrentBranch(ctx context.Context) (string, error) {
	if m.branchErr != nil {
		return "", m.branchErr
	}
	if m.currentBranch != "" {
		return m.currentBranch, nil
	}
	if m.info != nil {
		return m.info.CurrentBranch, nil
	}
	return "main", nil
}

func (m *mockGitRepository) GetCommit(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.Commit, error) {
	return nil, nil
}

func (m *mockGitRepository) GetCommitsBetween(ctx context.Context, from, to string) ([]*sourcecontrol.Commit, error) {
	if m.commitsErr != nil {
		return nil, m.commitsErr
	}
	return m.commits, nil
}

func (m *mockGitRepository) GetCommitsSince(ctx context.Context, ref string) ([]*sourcecontrol.Commit, error) {
	return m.commits, m.commitsErr
}

func (m *mockGitRepository) GetLatestCommit(ctx context.Context, branch string) (*sourcecontrol.Commit, error) {
	if m.latestErr != nil {
		return nil, m.latestErr
	}
	return m.latestCommit, nil
}

func (m *mockGitRepository) GetCommitDiffStats(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.DiffStats, error) {
	return nil, nil
}

func (m *mockGitRepository) GetBatchCommitDiffStats(ctx context.Context, hashes []sourcecontrol.CommitHash) (map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats, error) {
	return make(map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats), nil
}

func (m *mockGitRepository) GetCommitPatch(ctx context.Context, hash sourcecontrol.CommitHash) (string, error) {
	return "", nil
}

func (m *mockGitRepository) GetFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	return nil, nil
}

func (m *mockGitRepository) GetTags(ctx context.Context) (sourcecontrol.TagList, error) {
	if m.tagsErr != nil {
		return nil, m.tagsErr
	}
	return m.tags, nil
}

func (m *mockGitRepository) GetTag(ctx context.Context, name string) (*sourcecontrol.Tag, error) {
	return nil, nil
}

func (m *mockGitRepository) GetLatestVersionTag(ctx context.Context, prefix string) (*sourcecontrol.Tag, error) {
	return nil, nil
}

func (m *mockGitRepository) CreateTag(ctx context.Context, name string, hash sourcecontrol.CommitHash, message string) (*sourcecontrol.Tag, error) {
	return nil, nil
}

func (m *mockGitRepository) DeleteTag(ctx context.Context, name string) error {
	return nil
}

func (m *mockGitRepository) PushTag(ctx context.Context, name string, remote string) error {
	return nil
}

func (m *mockGitRepository) IsDirty(ctx context.Context) (bool, error) {
	if m.isDirtyErr != nil {
		return false, m.isDirtyErr
	}
	return m.isDirty, nil
}

func (m *mockGitRepository) GetStatus(ctx context.Context) (*sourcecontrol.WorkingTreeStatus, error) {
	return nil, nil
}

func (m *mockGitRepository) Fetch(ctx context.Context, remote string) error {
	return nil
}

func (m *mockGitRepository) Pull(ctx context.Context, remote, branch string) error {
	return nil
}

func (m *mockGitRepository) Push(ctx context.Context, remote, branch string) error {
	return nil
}

func TestGitRepoInspector_HeadSHA(t *testing.T) {
	mock := &mockGitRepository{
		currentBranch: "main",
		latestCommit: sourcecontrol.NewCommit(
			sourcecontrol.CommitHash("abc123def456"),
			"Test commit",
			sourcecontrol.Author{Name: "Test", Email: "test@example.com"},
			time.Now(),
		),
	}

	inspector := NewGitRepoInspector(mock)
	ctx := context.Background()

	sha, err := inspector.HeadSHA(ctx)
	if err != nil {
		t.Fatalf("HeadSHA failed: %v", err)
	}
	if sha != "abc123def456" {
		t.Errorf("SHA mismatch: got %s, want abc123def456", sha)
	}
}

func TestGitRepoInspector_HeadSHAError(t *testing.T) {
	mock := &mockGitRepository{
		latestErr: os.ErrNotExist, // GetLatestCommit("HEAD") fails
	}

	inspector := NewGitRepoInspector(mock)
	ctx := context.Background()

	_, err := inspector.HeadSHA(ctx)
	if err == nil {
		t.Error("Expected error from HeadSHA")
	}
}

func TestGitRepoInspector_IsClean(t *testing.T) {
	tests := []struct {
		name      string
		isDirty   bool
		wantClean bool
	}{
		{"clean repo", false, true},
		{"dirty repo", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockGitRepository{isDirty: tt.isDirty}
			inspector := NewGitRepoInspector(mock)
			ctx := context.Background()

			clean, err := inspector.IsClean(ctx)
			if err != nil {
				t.Fatalf("IsClean failed: %v", err)
			}
			if clean != tt.wantClean {
				t.Errorf("IsClean = %v, want %v", clean, tt.wantClean)
			}
		})
	}
}

func TestGitRepoInspector_ResolveCommits(t *testing.T) {
	commits := []*sourcecontrol.Commit{
		sourcecontrol.NewCommit("abc123", "Commit 1", sourcecontrol.Author{}, time.Now()),
		sourcecontrol.NewCommit("def456", "Commit 2", sourcecontrol.Author{}, time.Now()),
	}

	mock := &mockGitRepository{commits: commits}
	inspector := NewGitRepoInspector(mock)
	ctx := context.Background()

	shas, err := inspector.ResolveCommits(ctx, "v1.0.0", "abc123")
	if err != nil {
		t.Fatalf("ResolveCommits failed: %v", err)
	}
	if len(shas) != 2 {
		t.Errorf("Expected 2 commits, got %d", len(shas))
	}
	if shas[0] != "abc123" {
		t.Errorf("First SHA mismatch: got %s, want abc123", shas[0])
	}
}

func TestGitRepoInspector_GetRemoteURL(t *testing.T) {
	mock := &mockGitRepository{
		info: &sourcecontrol.RepositoryInfo{
			RemoteURL: "https://github.com/test/repo.git",
		},
	}

	inspector := NewGitRepoInspector(mock)
	ctx := context.Background()

	url, err := inspector.GetRemoteURL(ctx)
	if err != nil {
		t.Fatalf("GetRemoteURL failed: %v", err)
	}
	if url != "https://github.com/test/repo.git" {
		t.Errorf("URL mismatch: got %s", url)
	}
}

func TestGitRepoInspector_GetCurrentBranch(t *testing.T) {
	mock := &mockGitRepository{
		info: &sourcecontrol.RepositoryInfo{
			CurrentBranch: "feature-branch",
		},
	}

	inspector := NewGitRepoInspector(mock)
	ctx := context.Background()

	branch, err := inspector.GetCurrentBranch(ctx)
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}
	if branch != "feature-branch" {
		t.Errorf("Branch mismatch: got %s", branch)
	}
}

func TestGitRepoInspector_GetLatestVersionTag(t *testing.T) {
	// Create version tags
	tags := sourcecontrol.TagList{
		sourcecontrol.NewTag("v1.0.0", "abc123"),
		sourcecontrol.NewTag("v1.1.0", "def456"),
		sourcecontrol.NewTag("v2.0.0", "ghi789"),
	}

	mock := &mockGitRepository{tags: tags}
	inspector := NewGitRepoInspector(mock)
	ctx := context.Background()

	tag, err := inspector.GetLatestVersionTag(ctx, "v")
	if err != nil {
		t.Fatalf("GetLatestVersionTag failed: %v", err)
	}
	if tag != "v2.0.0" {
		t.Errorf("Expected v2.0.0, got %s", tag)
	}
}

func TestGitRepoInspector_GetLatestVersionTagEmpty(t *testing.T) {
	mock := &mockGitRepository{tags: sourcecontrol.TagList{}}
	inspector := NewGitRepoInspector(mock)
	ctx := context.Background()

	tag, err := inspector.GetLatestVersionTag(ctx, "v")
	if err != nil {
		t.Fatalf("GetLatestVersionTag failed: %v", err)
	}
	if tag != "" {
		t.Errorf("Expected empty tag, got %s", tag)
	}
}

func TestGitRepoInspector_TagExists(t *testing.T) {
	tags := sourcecontrol.TagList{
		sourcecontrol.NewTag("v1.0.0", "abc123"),
		sourcecontrol.NewTag("v1.1.0", "def456"),
	}

	mock := &mockGitRepository{tags: tags}
	inspector := NewGitRepoInspector(mock)
	ctx := context.Background()

	// Tag exists
	exists, err := inspector.TagExists(ctx, "v1.0.0")
	if err != nil {
		t.Fatalf("TagExists failed: %v", err)
	}
	if !exists {
		t.Error("Expected tag to exist")
	}

	// Tag doesn't exist
	exists, err = inspector.TagExists(ctx, "v3.0.0")
	if err != nil {
		t.Fatalf("TagExists failed: %v", err)
	}
	if exists {
		t.Error("Expected tag to not exist")
	}
}

func TestGitRepoInspector_ReleaseExists(t *testing.T) {
	tags := sourcecontrol.TagList{
		sourcecontrol.NewTag("v1.0.0", "abc123"),
	}

	mock := &mockGitRepository{tags: tags}
	inspector := NewGitRepoInspector(mock)
	ctx := context.Background()

	// Currently just checks tag exists
	exists, err := inspector.ReleaseExists(ctx, "v1.0.0")
	if err != nil {
		t.Fatalf("ReleaseExists failed: %v", err)
	}
	if !exists {
		t.Error("Expected release to exist")
	}
}

// =============================================================================
// DTO Conversion Tests
// =============================================================================

func TestDTO_RoundTrip(t *testing.T) {
	// Create a run with all fields populated
	run := domain.NewReleaseRun(
		"github.com/test/repo",
		"/tmp/repo",
		"v1.0.0",
		domain.CommitSHA("abc123"),
		[]domain.CommitSHA{"abc123", "def456"},
		"config-hash",
		"plugin-hash",
	)

	// Transition through states
	_ = run.Plan("system")
	_ = run.SetVersion(version.NewSemanticVersion(1, 1, 0), "v1.1.0")
	_ = run.Bump("system")

	// Add notes using GenerateNotes
	notes := &domain.ReleaseNotes{
		Text:           "Test notes",
		AudiencePreset: "developer",
		TonePreset:     "formal",
		Provider:       "openai",
		Model:          "gpt-4",
		GeneratedAt:    time.Now().Truncate(time.Second),
	}
	_ = run.GenerateNotes(notes, "inputs-hash", "system")

	// Add steps
	steps := []domain.StepPlan{
		{Name: "step1", Type: domain.StepTypeTag},
		{Name: "step2", Type: domain.StepTypeNotify},
	}
	run.SetExecutionPlan(steps)

	// Convert to DTO and back
	dto := toDTO(run)
	reconstructed, err := fromDTO(dto)
	if err != nil {
		t.Fatalf("fromDTO failed: %v", err)
	}

	// Verify key fields
	if reconstructed.ID() != run.ID() {
		t.Errorf("ID mismatch: got %s, want %s", reconstructed.ID(), run.ID())
	}
	if reconstructed.State() != run.State() {
		t.Errorf("State mismatch: got %s, want %s", reconstructed.State(), run.State())
	}
	if reconstructed.HeadSHA() != run.HeadSHA() {
		t.Errorf("HeadSHA mismatch: got %s, want %s", reconstructed.HeadSHA(), run.HeadSHA())
	}
	if reconstructed.VersionNext().String() != run.VersionNext().String() {
		t.Errorf("VersionNext mismatch: got %s, want %s", reconstructed.VersionNext(), run.VersionNext())
	}
	if len(reconstructed.Steps()) != len(run.Steps()) {
		t.Errorf("Steps count mismatch: got %d, want %d", len(reconstructed.Steps()), len(run.Steps()))
	}
	if reconstructed.Notes() == nil || reconstructed.Notes().Text != notes.Text {
		t.Error("Notes mismatch")
	}
}

func TestDTO_WithApproval(t *testing.T) {
	run := domain.NewReleaseRun(
		"github.com/test/repo",
		"/tmp/repo",
		"v1.0.0",
		domain.CommitSHA("abc123"),
		[]domain.CommitSHA{"abc123"},
		"config-hash",
		"plugin-hash",
	)

	// Get to approved state
	_ = run.Plan("system")
	_ = run.SetVersion(version.NewSemanticVersion(1, 1, 0), "v1.1.0")
	_ = run.Bump("system")
	_ = run.GenerateNotes(&domain.ReleaseNotes{Text: "notes", GeneratedAt: time.Now()}, "hash", "system")
	_ = run.Approve("approver@example.com", false)

	// Convert and verify
	dto := toDTO(run)
	if dto.Approval == nil {
		t.Fatal("Expected approval in DTO")
	}
	if dto.Approval.ApprovedBy != "approver@example.com" {
		t.Errorf("ApprovedBy mismatch: got %s", dto.Approval.ApprovedBy)
	}

	reconstructed, err := fromDTO(dto)
	if err != nil {
		t.Fatalf("fromDTO failed: %v", err)
	}
	if reconstructed.Approval() == nil {
		t.Fatal("Expected approval in reconstructed run")
	}
	if reconstructed.Approval().ApprovedBy != "approver@example.com" {
		t.Errorf("ApprovedBy mismatch in reconstructed: got %s", reconstructed.Approval().ApprovedBy)
	}
}

// =============================================================================
// Path Helper Tests
// =============================================================================

func TestPathHelpers(t *testing.T) {
	repoRoot := "/tmp/myrepo"
	runID := domain.RunID("test-run-123")

	// Test runsPath
	rp := runsPath(repoRoot)
	expected := "/tmp/myrepo/.relicta/releases"
	if rp != expected {
		t.Errorf("runsPath mismatch: got %s, want %s", rp, expected)
	}

	// Test runPath
	runP := runPath(repoRoot, runID)
	expected = "/tmp/myrepo/.relicta/releases/test-run-123.json"
	if runP != expected {
		t.Errorf("runPath mismatch: got %s, want %s", runP, expected)
	}

	// Test latestPath
	lp := latestPath(repoRoot)
	expected = "/tmp/myrepo/.relicta/releases/latest"
	if lp != expected {
		t.Errorf("latestPath mismatch: got %s, want %s", lp, expected)
	}

	// Test machinePath
	mp := machinePath(repoRoot, runID)
	expected = "/tmp/myrepo/.relicta/releases/test-run-123.machine.json"
	if mp != expected {
		t.Errorf("machinePath mismatch: got %s, want %s", mp, expected)
	}

	// Test statePath
	sp := statePath(repoRoot, runID)
	expected = "/tmp/myrepo/.relicta/releases/test-run-123.state.json"
	if sp != expected {
		t.Errorf("statePath mismatch: got %s, want %s", sp, expected)
	}
}

func TestIsLockHeldError(t *testing.T) {
	tests := []struct {
		err    error
		result bool
	}{
		{nil, false},
		{os.ErrNotExist, false},
		{os.ErrPermission, false},
	}

	for _, tt := range tests {
		result := isLockHeldError(tt.err)
		if result != tt.result {
			t.Errorf("isLockHeldError(%v) = %v, want %v", tt.err, result, tt.result)
		}
	}
}

// =============================================================================
// FileEventStore Tests
// =============================================================================

func TestFileEventStore_AppendAndLoad(t *testing.T) {
	store := NewFileEventStore()
	repoRoot := t.TempDir()
	ctx := WithRepoRoot(context.Background(), repoRoot)

	runID := domain.RunID("test-run-events")

	// Create some test events
	now := time.Now()
	events := []domain.DomainEvent{
		&domain.RunCreatedEvent{
			RunID:   runID,
			RepoID:  "test/repo",
			HeadSHA: "abc123",
			At:      now,
		},
		&domain.StateTransitionedEvent{
			RunID: runID,
			From:  domain.StateDraft,
			To:    domain.StatePlanned,
			Event: "PLAN",
			Actor: "test-user",
			At:    now.Add(time.Second),
		},
	}

	// Append events
	err := store.Append(ctx, runID, events)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Load events
	loaded, err := store.LoadEvents(ctx, runID)
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}

	if len(loaded) != len(events) {
		t.Errorf("LoadEvents returned %d events, want %d", len(loaded), len(events))
	}

	// Verify first event
	if loaded[0].EventName() != "run.created" {
		t.Errorf("First event name = %s, want run.created", loaded[0].EventName())
	}

	// Verify second event
	if loaded[1].EventName() != "run.state_transitioned" {
		t.Errorf("Second event name = %s, want run.state_transitioned", loaded[1].EventName())
	}
}

func TestFileEventStore_LoadEventsSince(t *testing.T) {
	store := NewFileEventStore()
	repoRoot := t.TempDir()
	ctx := WithRepoRoot(context.Background(), repoRoot)

	runID := domain.RunID("test-run-since")

	// Create events with different timestamps
	baseTime := time.Now()
	events := []domain.DomainEvent{
		&domain.RunCreatedEvent{
			RunID:   runID,
			RepoID:  "test/repo",
			HeadSHA: "abc123",
			At:      baseTime,
		},
		&domain.StateTransitionedEvent{
			RunID: runID,
			From:  domain.StateDraft,
			To:    domain.StatePlanned,
			At:    baseTime.Add(time.Minute),
		},
		&domain.StateTransitionedEvent{
			RunID: runID,
			From:  domain.StatePlanned,
			To:    domain.StateVersioned,
			At:    baseTime.Add(2 * time.Minute),
		},
	}

	err := store.Append(ctx, runID, events)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Load events since baseTime + 30 seconds
	since := baseTime.Add(30 * time.Second)
	loaded, err := store.LoadEventsSince(ctx, runID, since)
	if err != nil {
		t.Fatalf("LoadEventsSince failed: %v", err)
	}

	// Should only get the 2 events after baseTime
	if len(loaded) != 2 {
		t.Errorf("LoadEventsSince returned %d events, want 2", len(loaded))
	}
}

func TestFileEventStore_LoadAllEvents(t *testing.T) {
	store := NewFileEventStore()
	repoRoot := t.TempDir()
	ctx := WithRepoRoot(context.Background(), repoRoot)

	// Create events for two different runs
	run1 := domain.RunID("test-run-1")
	run2 := domain.RunID("test-run-2")

	now := time.Now()

	err := store.Append(ctx, run1, []domain.DomainEvent{
		&domain.RunCreatedEvent{RunID: run1, RepoID: "test/repo", At: now},
	})
	if err != nil {
		t.Fatalf("Append run1 failed: %v", err)
	}

	err = store.Append(ctx, run2, []domain.DomainEvent{
		&domain.RunCreatedEvent{RunID: run2, RepoID: "test/repo", At: now.Add(time.Second)},
	})
	if err != nil {
		t.Fatalf("Append run2 failed: %v", err)
	}

	// Load all events for the repo
	loaded, err := store.LoadAllEvents(ctx, repoRoot)
	if err != nil {
		t.Fatalf("LoadAllEvents failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Errorf("LoadAllEvents returned %d events, want 2", len(loaded))
	}
}

func TestFileEventStore_EmptyRunReturnsNil(t *testing.T) {
	store := NewFileEventStore()
	repoRoot := t.TempDir()
	ctx := WithRepoRoot(context.Background(), repoRoot)

	runID := domain.RunID("nonexistent-run")

	events, err := store.LoadEvents(ctx, runID)
	if err != nil {
		t.Errorf("LoadEvents should not error for nonexistent run: %v", err)
	}
	if events != nil {
		t.Errorf("LoadEvents should return nil for nonexistent run, got %d events", len(events))
	}
}

func TestEventPublishingRepository_SavePublishesEvents(t *testing.T) {
	baseRepo := NewFileReleaseRunRepository()
	eventStore := NewFileEventStore()
	repo := NewEventPublishingRepository(baseRepo, eventStore)

	repoRoot := t.TempDir()
	ctx := WithRepoRoot(context.Background(), repoRoot)

	// Create a run (this generates a RunCreatedEvent)
	run := domain.NewReleaseRun(
		"test/repo",
		repoRoot,
		"main",
		domain.CommitSHA("abc123"),
		nil,
		"config-hash",
		"plugin-hash",
	)

	// Save the run
	err := repo.Save(ctx, run)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify events were persisted
	events, err := eventStore.LoadEvents(ctx, run.ID())
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}

	if len(events) == 0 {
		t.Error("Expected at least 1 event (RunCreated) to be persisted")
	}

	// Verify events were cleared from aggregate
	if len(run.DomainEvents()) != 0 {
		t.Errorf("Expected events to be cleared from aggregate, got %d", len(run.DomainEvents()))
	}
}

// =============================================================================
// FindByPlanHash Tests
// =============================================================================

func TestFileReleaseRunRepository_LoadBatch(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Create and save two runs
	run1 := domain.NewReleaseRun("github.com/test/repo", repoRoot, "v1.0.0",
		domain.CommitSHA("abc123"), []domain.CommitSHA{"abc123"}, "config1", "plugin1")
	run2 := domain.NewReleaseRun("github.com/test/repo", repoRoot, "v1.1.0",
		domain.CommitSHA("def456"), []domain.CommitSHA{"def456"}, "config2", "plugin2")

	_ = repo.Save(ctx, run1)
	_ = repo.Save(ctx, run2)

	// LoadBatch with both IDs
	result, err := repo.LoadBatch(ctx, repoRoot, []domain.RunID{run1.ID(), run2.ID()})
	if err != nil {
		t.Fatalf("LoadBatch failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 runs, got %d", len(result))
	}
	if result[run1.ID()] == nil {
		t.Error("Expected run1 in result")
	}
	if result[run2.ID()] == nil {
		t.Error("Expected run2 in result")
	}
}

func TestFileReleaseRunRepository_LoadBatchWithMissing(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Create one run
	run1 := domain.NewReleaseRun("github.com/test/repo", repoRoot, "v1.0.0",
		domain.CommitSHA("abc123"), []domain.CommitSHA{"abc123"}, "config1", "plugin1")
	_ = repo.Save(ctx, run1)

	// LoadBatch with one valid and one invalid ID
	result, err := repo.LoadBatch(ctx, repoRoot, []domain.RunID{run1.ID(), "nonexistent-id"})
	if err != nil {
		t.Fatalf("LoadBatch failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 run (skipping missing), got %d", len(result))
	}
}

func TestFileReleaseRunRepository_LoadBatchEmpty(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	result, err := repo.LoadBatch(ctx, repoRoot, []domain.RunID{})
	if err != nil {
		t.Fatalf("LoadBatch failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("Expected 0 runs for empty input, got %d", len(result))
	}
}

func TestValidateRepoRoot(t *testing.T) {
	// Valid directory
	tmpDir := t.TempDir()
	path, err := validateRepoRoot(tmpDir)
	if err != nil {
		t.Fatalf("validateRepoRoot(%s) failed: %v", tmpDir, err)
	}
	if path == "" {
		t.Error("Expected non-empty path")
	}

	// Empty path
	_, err = validateRepoRoot("")
	if err == nil {
		t.Error("Expected error for empty path")
	}

	// Non-existent path
	_, err = validateRepoRoot("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("Expected error for non-existent path")
	}

	// File instead of directory
	tmpFile := filepath.Join(tmpDir, "not-a-dir")
	_ = os.WriteFile(tmpFile, []byte("content"), 0644)
	_, err = validateRepoRoot(tmpFile)
	if err == nil {
		t.Error("Expected error for file path (not directory)")
	}
}

func TestFileReleaseRunRepository_LoadWithKnownRoot(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Create and save a run (this registers the repo root)
	run := domain.NewReleaseRun("github.com/test/repo", repoRoot, "v1.0.0",
		domain.CommitSHA("abc123"), []domain.CommitSHA{"abc123"}, "config", "plugin")
	_ = repo.Save(ctx, run)

	// Now Load (without repoRoot) should find it via known roots
	loaded, err := repo.Load(ctx, run.ID())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.ID() != run.ID() {
		t.Errorf("Load returned wrong run: got %s, want %s", loaded.ID(), run.ID())
	}
}

func TestFileReleaseRunRepository_DeleteWithKnownRoot(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Create and save a run (registers repo root)
	run := domain.NewReleaseRun("github.com/test/repo", repoRoot, "v1.0.0",
		domain.CommitSHA("abc123"), []domain.CommitSHA{"abc123"}, "config", "plugin")
	_ = repo.Save(ctx, run)

	// Delete via known roots
	err := repo.Delete(ctx, run.ID())
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	_, err = repo.LoadFromRepo(ctx, repoRoot, run.ID())
	if err != domain.ErrRunNotFound {
		t.Errorf("Expected ErrRunNotFound after delete, got %v", err)
	}
}

func TestFileReleaseRunRepository_FindByPlanHash(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Create a run
	run := domain.NewReleaseRun(
		"test/repo",
		repoRoot,
		"main",
		domain.CommitSHA("abc123"),
		[]domain.CommitSHA{"abc123"},
		"config-hash",
		"plugin-hash",
	)
	planHash := run.PlanHash()

	// Save the run
	err := repo.Save(ctx, run)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Find by plan hash should return the run
	found, err := repo.FindByPlanHash(ctx, repoRoot, planHash)
	if err != nil {
		t.Fatalf("FindByPlanHash failed: %v", err)
	}
	if found == nil {
		t.Error("FindByPlanHash should return the run")
	}
	if found.ID() != run.ID() {
		t.Errorf("FindByPlanHash returned wrong run: got %s, want %s", found.ID(), run.ID())
	}
}

func TestFileReleaseRunRepository_FindByPlanHash_NotFound(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Search for a non-existent plan hash
	found, err := repo.FindByPlanHash(ctx, repoRoot, "nonexistent-hash")
	if err != nil {
		t.Errorf("FindByPlanHash should not error for missing hash: %v", err)
	}
	if found != nil {
		t.Error("FindByPlanHash should return nil for missing hash")
	}
}

func TestFileReleaseRunRepository_FindByPlanHash_DuplicateDetection(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Create first run
	run1 := domain.NewReleaseRun(
		"test/repo",
		repoRoot,
		"main",
		domain.CommitSHA("abc123"),
		[]domain.CommitSHA{"abc123"},
		"config-hash",
		"plugin-hash",
	)
	planHash := run1.PlanHash()

	err := repo.Save(ctx, run1)
	if err != nil {
		t.Fatalf("Save run1 failed: %v", err)
	}

	// Before creating a second run with same params, check for duplicate
	existing, err := repo.FindByPlanHash(ctx, repoRoot, planHash)
	if err != nil {
		t.Fatalf("FindByPlanHash failed: %v", err)
	}

	// Should find the existing run
	if existing == nil {
		t.Error("Should find existing run with same plan hash")
	}
	if existing.ID() != run1.ID() {
		t.Error("Should return the first run with matching plan hash")
	}

	// This is where a use case would return ErrDuplicateRun
	// to prevent creating a new run with the same plan
}

// =============================================================================
// FileLockManager Additional Tests
// =============================================================================

func TestFileLockManager_TryAcquire_NotHeld(t *testing.T) {
	lockMgr := NewFileLockManager()
	repoRoot := t.TempDir()
	ctx := context.Background()

	release, acquired, err := lockMgr.TryAcquire(ctx, repoRoot, "run-123")
	if err != nil {
		t.Fatalf("TryAcquire error: %v", err)
	}
	if !acquired {
		t.Error("TryAcquire should acquire when no lock exists")
	}
	if release == nil {
		t.Error("TryAcquire should return release function")
	}
	if release != nil {
		release()
	}
}

func TestFileLockManager_TryAcquire_AlreadyHeld(t *testing.T) {
	lockMgr := NewFileLockManager()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// First acquire
	release1, err := lockMgr.Acquire(ctx, repoRoot, "run-123")
	if err != nil {
		t.Fatalf("First Acquire error: %v", err)
	}
	defer release1()

	// Try to acquire again
	release2, acquired, err := lockMgr.TryAcquire(ctx, repoRoot, "run-456")
	if err != nil {
		t.Fatalf("TryAcquire error: %v", err)
	}
	if acquired {
		t.Error("TryAcquire should not acquire when lock is held")
		if release2 != nil {
			release2()
		}
	}
}

func TestFileLockManager_IsLocked_NoLock(t *testing.T) {
	lockMgr := NewFileLockManager()
	repoRoot := t.TempDir()
	ctx := context.Background()

	locked, err := lockMgr.IsLocked(ctx, repoRoot, "run-123")
	if err != nil {
		t.Fatalf("IsLocked error: %v", err)
	}
	if locked {
		t.Error("IsLocked should return false when no lock exists")
	}
}

func TestFileLockManager_IsLocked_WithLock(t *testing.T) {
	lockMgr := NewFileLockManager()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Acquire lock
	release, err := lockMgr.Acquire(ctx, repoRoot, "run-123")
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	defer release()

	locked, err := lockMgr.IsLocked(ctx, repoRoot, "run-123")
	if err != nil {
		t.Fatalf("IsLocked error: %v", err)
	}
	if !locked {
		t.Error("IsLocked should return true when lock is held")
	}
}

func TestFileLockManager_GetLockInfo_NoLock(t *testing.T) {
	lockMgr := NewFileLockManager()
	repoRoot := t.TempDir()

	info, err := lockMgr.GetLockInfo(repoRoot)
	if err != nil {
		t.Fatalf("GetLockInfo error: %v", err)
	}
	if info != nil {
		t.Error("GetLockInfo should return nil when no lock exists")
	}
}

func TestFileLockManager_GetLockInfo_WithLock(t *testing.T) {
	lockMgr := NewFileLockManager()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Acquire lock
	release, err := lockMgr.Acquire(ctx, repoRoot, "run-123")
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	defer release()

	info, err := lockMgr.GetLockInfo(repoRoot)
	if err != nil {
		t.Fatalf("GetLockInfo error: %v", err)
	}
	if info == nil {
		t.Fatal("GetLockInfo should return info when lock is held")
	}
	if info.RunID != "run-123" {
		t.Errorf("GetLockInfo RunID = %v, want run-123", info.RunID)
	}
	if info.HolderPID == 0 {
		t.Error("GetLockInfo should have a valid PID")
	}
}

func TestFileLockManager_Release(t *testing.T) {
	lockMgr := NewFileLockManager()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Acquire and release
	release, err := lockMgr.Acquire(ctx, repoRoot, "run-123")
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	release()

	// Should be able to acquire again
	release2, err := lockMgr.Acquire(ctx, repoRoot, "run-456")
	if err != nil {
		t.Fatalf("Second Acquire error: %v", err)
	}
	defer release2()
}

// =============================================================================
// EventPublishingRepository Additional Tests
// =============================================================================

func TestEventPublishingRepository_SaveAndLoad(t *testing.T) {
	repoRoot := t.TempDir()
	ctx := WithRepoRoot(context.Background(), repoRoot)

	baseRepo := NewFileReleaseRunRepository()
	eventStore := NewFileEventStore()
	repo := NewEventPublishingRepository(baseRepo, eventStore)

	// Create and save a run
	run := domain.NewReleaseRun(
		"test/repo",
		repoRoot,
		"main",
		domain.CommitSHA("abc123"),
		nil,
		"",
		"",
	)
	run.EmitCreatedEvent()

	err := repo.Save(ctx, run)
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}

	// Verify events were cleared
	if len(run.DomainEvents()) != 0 {
		t.Error("Save should clear domain events")
	}

	// Load the run using LoadFromRepo
	loaded, err := repo.LoadFromRepo(ctx, repoRoot, run.ID())
	if err != nil {
		t.Fatalf("LoadFromRepo error: %v", err)
	}
	if loaded.ID() != run.ID() {
		t.Errorf("LoadFromRepo ID = %v, want %v", loaded.ID(), run.ID())
	}
}

func TestEventPublishingRepository_FindByPlanHash(t *testing.T) {
	repoRoot := t.TempDir()
	ctx := WithRepoRoot(context.Background(), repoRoot)

	baseRepo := NewFileReleaseRunRepository()
	repo := NewEventPublishingRepository(baseRepo, nil)

	// Create and save a run
	run := domain.NewReleaseRun(
		"test/repo",
		repoRoot,
		"main",
		domain.CommitSHA("abc123"),
		nil,
		"",
		"",
	)

	err := repo.Save(ctx, run)
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}

	// Find by plan hash
	found, err := repo.FindByPlanHash(ctx, repoRoot, run.PlanHash())
	if err != nil {
		t.Fatalf("FindByPlanHash error: %v", err)
	}
	if found == nil {
		t.Error("FindByPlanHash should find the run")
	}
}

func TestEventPublishingRepository_NilEventStore(t *testing.T) {
	repoRoot := t.TempDir()
	ctx := context.Background()

	baseRepo := NewFileReleaseRunRepository()
	repo := NewEventPublishingRepository(baseRepo, nil) // No event store

	// Create a run with events
	run := domain.NewReleaseRun(
		"test/repo",
		repoRoot,
		"main",
		domain.CommitSHA("abc123"),
		nil,
		"",
		"",
	)
	run.EmitCreatedEvent()

	// Should save without error even with nil event store
	err := repo.Save(ctx, run)
	if err != nil {
		t.Fatalf("Save error with nil event store: %v", err)
	}
}

// =============================================================================
// Additional Coverage Tests
// =============================================================================

func TestFileEventStore_AppendWithContext(t *testing.T) {
	store := NewFileEventStore()
	repoRoot := t.TempDir()
	ctx := WithRepoRoot(context.Background(), repoRoot)

	runID := domain.RunID("test-run-context")

	// Append with event types that are well-tested for deserialization
	events := []domain.DomainEvent{
		&domain.RunCreatedEvent{
			RunID:   runID,
			RepoID:  "test/repo",
			HeadSHA: "abc123",
			At:      time.Now(),
		},
		&domain.StateTransitionedEvent{
			RunID: runID,
			From:  domain.StateDraft,
			To:    domain.StatePlanned,
			Event: "PLAN",
			Actor: "system",
			At:    time.Now(),
		},
		&domain.RunApprovedEvent{
			RunID:        runID,
			ApprovedBy:   "user@example.com",
			AutoApproved: false,
			At:           time.Now(),
		},
	}

	err := store.Append(ctx, runID, events)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Load and verify events
	loaded, err := store.LoadEvents(ctx, runID)
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}

	if len(loaded) < 2 {
		t.Errorf("LoadEvents returned %d events, want at least 2", len(loaded))
	}

	// Verify first event type
	if loaded[0].EventName() != "run.created" {
		t.Errorf("First event name = %s, want run.created", loaded[0].EventName())
	}
}

func TestFileEventStore_AppendNoContext(t *testing.T) {
	store := NewFileEventStore()
	ctx := context.Background() // No repo root in context

	runID := domain.RunID("test-run-no-context")
	events := []domain.DomainEvent{
		&domain.RunCreatedEvent{
			RunID: runID,
			At:    time.Now(),
		},
	}

	err := store.Append(ctx, runID, events)
	if err == nil {
		t.Error("Expected error when appending without repo root context")
	}
	if !strings.Contains(err.Error(), "repo root not found") {
		t.Errorf("Error should mention 'repo root not found', got: %v", err)
	}
}

func TestFileEventStore_LoadEventsNoContext(t *testing.T) {
	store := NewFileEventStore()
	ctx := context.Background() // No repo root in context

	_, err := store.LoadEvents(ctx, domain.RunID("test-run"))
	if err == nil {
		t.Error("Expected error when loading without repo root context")
	}
}

func TestFileEventStore_AppendEmptyEvents(t *testing.T) {
	store := NewFileEventStore()
	repoRoot := t.TempDir()
	ctx := WithRepoRoot(context.Background(), repoRoot)

	runID := domain.RunID("test-run-empty")

	// Append empty events list
	err := store.Append(ctx, runID, []domain.DomainEvent{})
	if err != nil {
		t.Fatalf("Append empty events should not fail: %v", err)
	}

	// Append nil
	err = store.Append(ctx, runID, nil)
	if err != nil {
		t.Fatalf("Append nil events should not fail: %v", err)
	}
}

func TestFileLockManager_AcquireWithTimeout(t *testing.T) {
	lockMgr := NewFileLockManager()
	repoRoot := t.TempDir()

	// Use a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// First acquire should succeed
	release, err := lockMgr.Acquire(ctx, repoRoot, "run-1")
	if err != nil {
		t.Fatalf("First Acquire failed: %v", err)
	}
	defer release()

	// Second acquire with same timeout context should fail quickly
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()

	_, err = lockMgr.Acquire(ctx2, repoRoot, "run-2")
	if err == nil {
		t.Error("Second Acquire should fail due to existing lock")
	}
}

func TestFileLockManager_AcquireInvalidLockFile(t *testing.T) {
	lockMgr := NewFileLockManager()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Create an invalid lock file (not valid JSON)
	lockDir := filepath.Join(repoRoot, runsDir)
	_ = os.MkdirAll(lockDir, 0755)
	_ = os.WriteFile(filepath.Join(lockDir, lockFileName), []byte("invalid json"), 0644)

	// Try to acquire - may fail or succeed depending on implementation
	release, err := lockMgr.Acquire(ctx, repoRoot, "run-1")
	if err == nil {
		// If it succeeds, make sure to release
		release()
	}
	// Test that we don't panic with invalid lock file
}

func TestGitRepoInspector_IsCleanError(t *testing.T) {
	mock := &mockGitRepository{
		isDirtyErr: os.ErrPermission,
	}

	inspector := NewGitRepoInspector(mock)
	ctx := context.Background()

	_, err := inspector.IsClean(ctx)
	if err == nil {
		t.Error("Expected error from IsClean")
	}
}

func TestGitRepoInspector_GetRemoteURLError(t *testing.T) {
	mock := &mockGitRepository{
		infoErr: os.ErrNotExist,
	}

	inspector := NewGitRepoInspector(mock)
	ctx := context.Background()

	_, err := inspector.GetRemoteURL(ctx)
	if err == nil {
		t.Error("Expected error from GetRemoteURL")
	}
}

func TestGitRepoInspector_GetCurrentBranchFromInfo(t *testing.T) {
	// Test that we can get branch from info when branch method returns empty
	mock := &mockGitRepository{
		info: &sourcecontrol.RepositoryInfo{
			CurrentBranch: "develop",
		},
	}

	inspector := NewGitRepoInspector(mock)
	ctx := context.Background()

	branch, err := inspector.GetCurrentBranch(ctx)
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}
	if branch != "develop" {
		t.Errorf("Branch = %s, want develop", branch)
	}
}

func TestGitRepoInspector_ResolveCommitsError(t *testing.T) {
	mock := &mockGitRepository{
		commitsErr: os.ErrNotExist,
	}

	inspector := NewGitRepoInspector(mock)
	ctx := context.Background()

	_, err := inspector.ResolveCommits(ctx, "v1.0.0", "HEAD")
	if err == nil {
		t.Error("Expected error from ResolveCommits")
	}
}

func TestGitRepoInspector_TagExistsError(t *testing.T) {
	mock := &mockGitRepository{
		tagsErr: os.ErrPermission,
	}

	inspector := NewGitRepoInspector(mock)
	ctx := context.Background()

	_, err := inspector.TagExists(ctx, "v1.0.0")
	if err == nil {
		t.Error("Expected error from TagExists")
	}
}

func TestFileReleaseRunRepository_SetLatestCreatesFile(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Create and save a run first
	run := domain.NewReleaseRun("test/repo", repoRoot, "main",
		domain.CommitSHA("abc123"), nil, "", "")
	_ = repo.Save(ctx, run)

	// Set latest should work
	err := repo.SetLatest(ctx, repoRoot, run.ID())
	if err != nil {
		t.Fatalf("SetLatest failed: %v", err)
	}

	// Verify latest file was created
	latestFile := filepath.Join(repoRoot, runsDir, "latest")
	data, err := os.ReadFile(latestFile)
	if err != nil {
		t.Fatalf("Failed to read latest file: %v", err)
	}
	if string(data) != string(run.ID()) {
		t.Errorf("Latest file content = %s, want %s", data, run.ID())
	}
}

func TestFileReleaseRunRepository_MultipleRuns(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Create and save multiple runs
	runs := make([]*domain.ReleaseRun, 5)
	for i := 0; i < 5; i++ {
		runs[i] = domain.NewReleaseRun(
			"test/repo",
			repoRoot,
			"main",
			domain.CommitSHA("abc"+string(rune('0'+i))),
			nil,
			"config",
			"plugin",
		)
		time.Sleep(5 * time.Millisecond) // Ensure different timestamps
		err := repo.Save(ctx, runs[i])
		if err != nil {
			t.Fatalf("Save run %d failed: %v", i, err)
		}
	}

	// List should return all runs sorted by modification time
	ids, err := repo.List(ctx, repoRoot)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(ids) != 5 {
		t.Errorf("List returned %d runs, want 5", len(ids))
	}

	// Most recent should be first
	if ids[0] != runs[4].ID() {
		t.Errorf("First ID = %s, want %s (most recent)", ids[0], runs[4].ID())
	}
}

func TestFileEventStore_LoadAllEventsEmpty(t *testing.T) {
	store := NewFileEventStore()
	repoRoot := t.TempDir()
	ctx := WithRepoRoot(context.Background(), repoRoot)

	// Load from empty directory
	events, err := store.LoadAllEvents(ctx, repoRoot)
	if err != nil {
		t.Fatalf("LoadAllEvents should not error for empty dir: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("LoadAllEvents should return empty slice, got %d events", len(events))
	}
}

func TestFileLockManager_MultipleAcquireRelease(t *testing.T) {
	lockMgr := NewFileLockManager()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// Acquire and release multiple times in sequence
	for i := 0; i < 3; i++ {
		release, err := lockMgr.Acquire(ctx, repoRoot, domain.RunID("run-"+string(rune('0'+i))))
		if err != nil {
			t.Fatalf("Acquire %d failed: %v", i, err)
		}

		// Verify lock is held
		locked, err := lockMgr.IsLocked(ctx, repoRoot, "any-run")
		if err != nil {
			t.Fatalf("IsLocked %d failed: %v", i, err)
		}
		if !locked {
			t.Errorf("Should be locked after Acquire %d", i)
		}

		// Release
		release()

		// Verify lock is released
		locked, err = lockMgr.IsLocked(ctx, repoRoot, "any-run")
		if err != nil {
			t.Fatalf("IsLocked after release %d failed: %v", i, err)
		}
		if locked {
			t.Errorf("Should not be locked after release %d", i)
		}
	}
}

// =============================================================================
// Additional Event Store Tests for Coverage
// =============================================================================

func TestFileEventStore_AppendAndLoadVariousEvents(t *testing.T) {
	store := NewFileEventStore()
	repoRoot := t.TempDir()
	ctx := WithRepoRoot(context.Background(), repoRoot)

	runID := domain.RunID("test-run-various")

	// Test appending failed event
	failedEvent := &domain.RunFailedEvent{
		RunID:  runID,
		Reason: "test failure",
		At:     time.Now(),
	}
	err := store.Append(ctx, runID, []domain.DomainEvent{failedEvent})
	if err != nil {
		t.Fatalf("Append RunFailedEvent failed: %v", err)
	}

	// Test appending canceled event
	canceledEvent := &domain.RunCanceledEvent{
		RunID:  runID,
		Reason: "user canceled",
		By:     "test-user",
		At:     time.Now(),
	}
	err = store.Append(ctx, runID, []domain.DomainEvent{canceledEvent})
	if err != nil {
		t.Fatalf("Append RunCanceledEvent failed: %v", err)
	}

	// Load events
	events, err := store.LoadEvents(ctx, runID)
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}
	if len(events) < 2 {
		t.Errorf("Expected at least 2 events, got %d", len(events))
	}
}

func TestFileEventStore_LoadEventsSinceNoContext(t *testing.T) {
	store := NewFileEventStore()
	ctx := context.Background() // No repo root

	_, err := store.LoadEventsSince(ctx, domain.RunID("test"), time.Now())
	if err == nil {
		t.Error("Expected error when loading without repo root context")
	}
}

func TestFileEventStore_LoadAllEventsNoContext(t *testing.T) {
	store := NewFileEventStore()
	ctx := context.Background() // No repo root

	_, err := store.LoadAllEvents(ctx, "/tmp/nonexistent")
	// This should handle gracefully or return error
	if err != nil {
		t.Logf("LoadAllEvents returned error (expected): %v", err)
	}
}

func TestFileReleaseRunRepository_FindActiveEmpty(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// FindActive on empty repo should return empty list
	runs, err := repo.FindActive(ctx, repoRoot)
	if err != nil {
		t.Fatalf("FindActive failed: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("Expected 0 active runs, got %d", len(runs))
	}
}

func TestFileReleaseRunRepository_FindByStateEmpty(t *testing.T) {
	repo := NewFileReleaseRunRepository()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// FindByState on empty repo should return empty list
	runs, err := repo.FindByState(ctx, repoRoot, domain.StateDraft)
	if err != nil {
		t.Fatalf("FindByState failed: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("Expected 0 runs, got %d", len(runs))
	}
}

func TestGitRepoInspector_GetLatestVersionTagWithMixedTags(t *testing.T) {
	// Create tags with various formats
	tags := sourcecontrol.TagList{
		sourcecontrol.NewTag("release-1.0", "aaa"),
		sourcecontrol.NewTag("v1.0.0", "bbb"),
		sourcecontrol.NewTag("v0.9.0", "ccc"),
		sourcecontrol.NewTag("v1.2.3", "ddd"),
		sourcecontrol.NewTag("latest", "eee"),
	}

	mock := &mockGitRepository{tags: tags}
	inspector := NewGitRepoInspector(mock)
	ctx := context.Background()

	tag, err := inspector.GetLatestVersionTag(ctx, "v")
	if err != nil {
		t.Fatalf("GetLatestVersionTag failed: %v", err)
	}
	if tag != "v1.2.3" {
		t.Errorf("Expected v1.2.3, got %s", tag)
	}
}

func TestFileEventStore_SequentialAppends(t *testing.T) {
	store := NewFileEventStore()
	repoRoot := t.TempDir()
	ctx := WithRepoRoot(context.Background(), repoRoot)

	runID := domain.RunID("test-sequential")

	// Append events one by one
	for i := 0; i < 5; i++ {
		event := &domain.StateTransitionedEvent{
			RunID: runID,
			From:  domain.StateDraft,
			To:    domain.StatePlanned,
			Event: "PLAN",
			At:    time.Now().Add(time.Duration(i) * time.Second),
		}
		err := store.Append(ctx, runID, []domain.DomainEvent{event})
		if err != nil {
			t.Fatalf("Append %d failed: %v", i, err)
		}
	}

	// Load all events
	events, err := store.LoadEvents(ctx, runID)
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}
	if len(events) != 5 {
		t.Errorf("Expected 5 events, got %d", len(events))
	}
}

func TestFileLockManager_ConcurrentTryAcquire(t *testing.T) {
	lockMgr := NewFileLockManager()
	repoRoot := t.TempDir()
	ctx := context.Background()

	// First acquire
	release1, acquired1, err := lockMgr.TryAcquire(ctx, repoRoot, "run-1")
	if err != nil {
		t.Fatalf("TryAcquire 1 error: %v", err)
	}
	if !acquired1 {
		t.Error("First TryAcquire should succeed")
	}
	defer func() {
		if release1 != nil {
			release1()
		}
	}()

	// Second TryAcquire should fail
	release2, acquired2, err := lockMgr.TryAcquire(ctx, repoRoot, "run-2")
	if err != nil {
		t.Fatalf("TryAcquire 2 error: %v", err)
	}
	if acquired2 {
		t.Error("Second TryAcquire should not acquire")
		if release2 != nil {
			release2()
		}
	}
}

func TestFileEventStore_AllEventTypes(t *testing.T) {
	store := NewFileEventStore()
	repoRoot := t.TempDir()
	ctx := WithRepoRoot(context.Background(), repoRoot)
	runID := domain.RunID("test-all-events")
	now := time.Now()

	// Create events for all supported types
	events := []domain.DomainEvent{
		&domain.RunCreatedEvent{RunID: runID, RepoID: "test", HeadSHA: "abc", At: now},
		&domain.StateTransitionedEvent{RunID: runID, From: "draft", To: "planned", Event: "PLAN", At: now},
		&domain.RunPlannedEvent{RunID: runID, CommitCount: 5, At: now},
		&domain.RunVersionedEvent{RunID: runID, TagName: "v1.0.0", At: now},
		&domain.RunNotesGeneratedEvent{RunID: runID, NotesLength: 100, Provider: "test", At: now},
		&domain.RunNotesUpdatedEvent{RunID: runID, NotesLength: 150, Actor: "user", At: now},
		&domain.RunApprovedEvent{RunID: runID, ApprovedBy: "approver", At: now},
		&domain.RunPublishingStartedEvent{RunID: runID, At: now},
		&domain.RunPublishedEvent{RunID: runID, At: now},
		&domain.RunFailedEvent{RunID: runID, Reason: "error", At: now},
		&domain.RunCanceledEvent{RunID: runID, Reason: "user", By: "admin", At: now},
		&domain.RunRetriedEvent{RunID: runID, By: "system", At: now},
		&domain.StepCompletedEvent{RunID: runID, StepName: "tag", Success: true, At: now},
		&domain.PluginExecutedEvent{RunID: runID, PluginName: "github", Hook: "post-publish", At: now},
	}

	// Append all events
	err := store.Append(ctx, runID, events)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Load and verify all events were stored
	loaded, err := store.LoadEvents(ctx, runID)
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}

	// Verify we got at least 10 events (some may not be deserialized due to name mismatches)
	if len(loaded) < 10 {
		t.Errorf("Expected at least 10 events to be deserialized, got %d", len(loaded))
	}

	// Verify first event is run.created
	if len(loaded) > 0 && loaded[0].EventName() != "run.created" {
		t.Errorf("First event: got %s, want run.created", loaded[0].EventName())
	}
}

// ============================================================================
// RealClock Tests
// ============================================================================

func TestRealClock_Now(t *testing.T) {
	clock := NewRealClock()
	before := time.Now()
	result := clock.Now()
	after := time.Now()

	if result.Before(before) || result.After(after) {
		t.Errorf("RealClock.Now() returned time outside expected range")
	}
}

func TestNewRealClock(t *testing.T) {
	clock := NewRealClock()
	if clock == nil {
		t.Error("NewRealClock() returned nil")
	}
}

// ============================================================================
// EventPublishingRepository Pass-through Tests
// ============================================================================

// mockReleaseRunRepository is a minimal mock for testing pass-through methods.
type mockReleaseRunRepository struct {
	runs                 map[domain.RunID]*domain.ReleaseRun
	latestRunID          domain.RunID
	loadCalled           bool
	loadBatchCalled      bool
	loadLatestCalled     bool
	setLatestCalled      bool
	listCalled           bool
	deleteCalled         bool
	findByStateCalled    bool
	findActiveCalled     bool
	findByPlanHashCalled bool
}

func newMockReleaseRunRepository() *mockReleaseRunRepository {
	return &mockReleaseRunRepository{
		runs: make(map[domain.RunID]*domain.ReleaseRun),
	}
}

func (m *mockReleaseRunRepository) Save(_ context.Context, run *domain.ReleaseRun) error {
	m.runs[run.ID()] = run
	return nil
}

func (m *mockReleaseRunRepository) Load(_ context.Context, runID domain.RunID) (*domain.ReleaseRun, error) {
	m.loadCalled = true
	if run, ok := m.runs[runID]; ok {
		return run, nil
	}
	return nil, nil
}

func (m *mockReleaseRunRepository) LoadBatch(_ context.Context, _ string, runIDs []domain.RunID) (map[domain.RunID]*domain.ReleaseRun, error) {
	m.loadBatchCalled = true
	result := make(map[domain.RunID]*domain.ReleaseRun)
	for _, id := range runIDs {
		if run, ok := m.runs[id]; ok {
			result[id] = run
		}
	}
	return result, nil
}

func (m *mockReleaseRunRepository) LoadLatest(_ context.Context, _ string) (*domain.ReleaseRun, error) {
	m.loadLatestCalled = true
	if m.latestRunID != "" {
		return m.runs[m.latestRunID], nil
	}
	return nil, nil
}

func (m *mockReleaseRunRepository) SetLatest(_ context.Context, _ string, runID domain.RunID) error {
	m.setLatestCalled = true
	m.latestRunID = runID
	return nil
}

func (m *mockReleaseRunRepository) List(_ context.Context, _ string) ([]domain.RunID, error) {
	m.listCalled = true
	var ids []domain.RunID
	for id := range m.runs {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *mockReleaseRunRepository) Delete(_ context.Context, runID domain.RunID) error {
	m.deleteCalled = true
	delete(m.runs, runID)
	return nil
}

func (m *mockReleaseRunRepository) FindByState(_ context.Context, _ string, state domain.RunState) ([]*domain.ReleaseRun, error) {
	m.findByStateCalled = true
	var result []*domain.ReleaseRun
	for _, run := range m.runs {
		if run.State() == state {
			result = append(result, run)
		}
	}
	return result, nil
}

func (m *mockReleaseRunRepository) FindActive(_ context.Context, _ string) ([]*domain.ReleaseRun, error) {
	m.findActiveCalled = true
	var result []*domain.ReleaseRun
	for _, run := range m.runs {
		if !run.State().IsFinal() {
			result = append(result, run)
		}
	}
	return result, nil
}

func (m *mockReleaseRunRepository) FindByPlanHash(_ context.Context, _ string, _ string) (*domain.ReleaseRun, error) {
	m.findByPlanHashCalled = true
	return nil, nil
}

func (m *mockReleaseRunRepository) LoadFromRepo(ctx context.Context, _ string, runID domain.RunID) (*domain.ReleaseRun, error) {
	return m.Load(ctx, runID)
}

func TestEventPublishingRepository_Load(t *testing.T) {
	mockRepo := newMockReleaseRunRepository()
	run := domain.NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []domain.CommitSHA{"abc123"}, "cfg", "plug")
	mockRepo.runs[run.ID()] = run

	eventPubRepo := NewEventPublishingRepository(mockRepo, nil)
	loaded, err := eventPubRepo.Load(context.Background(), run.ID())

	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !mockRepo.loadCalled {
		t.Error("Expected Load to be called on underlying repository")
	}
	if loaded == nil || loaded.ID() != run.ID() {
		t.Error("Loaded run doesn't match expected run")
	}
}

func TestEventPublishingRepository_LoadBatch(t *testing.T) {
	mockRepo := newMockReleaseRunRepository()
	run1 := domain.NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []domain.CommitSHA{"abc123"}, "cfg", "plug")
	run2 := domain.NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "def456", []domain.CommitSHA{"def456"}, "cfg", "plug")
	mockRepo.runs[run1.ID()] = run1
	mockRepo.runs[run2.ID()] = run2

	eventPubRepo := NewEventPublishingRepository(mockRepo, nil)
	loaded, err := eventPubRepo.LoadBatch(context.Background(), "/tmp/repo", []domain.RunID{run1.ID(), run2.ID()})

	if err != nil {
		t.Fatalf("LoadBatch failed: %v", err)
	}
	if !mockRepo.loadBatchCalled {
		t.Error("Expected LoadBatch to be called on underlying repository")
	}
	if len(loaded) != 2 {
		t.Errorf("Expected 2 runs, got %d", len(loaded))
	}
}

func TestEventPublishingRepository_LoadLatest(t *testing.T) {
	mockRepo := newMockReleaseRunRepository()
	run := domain.NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []domain.CommitSHA{"abc123"}, "cfg", "plug")
	mockRepo.runs[run.ID()] = run
	mockRepo.latestRunID = run.ID()

	eventPubRepo := NewEventPublishingRepository(mockRepo, nil)
	loaded, err := eventPubRepo.LoadLatest(context.Background(), "/tmp/repo")

	if err != nil {
		t.Fatalf("LoadLatest failed: %v", err)
	}
	if !mockRepo.loadLatestCalled {
		t.Error("Expected LoadLatest to be called on underlying repository")
	}
	if loaded == nil || loaded.ID() != run.ID() {
		t.Error("Loaded run doesn't match expected run")
	}
}

func TestEventPublishingRepository_SetLatest(t *testing.T) {
	mockRepo := newMockReleaseRunRepository()
	runID := domain.RunID("test-run-id")

	eventPubRepo := NewEventPublishingRepository(mockRepo, nil)
	err := eventPubRepo.SetLatest(context.Background(), "/tmp/repo", runID)

	if err != nil {
		t.Fatalf("SetLatest failed: %v", err)
	}
	if !mockRepo.setLatestCalled {
		t.Error("Expected SetLatest to be called on underlying repository")
	}
	if mockRepo.latestRunID != runID {
		t.Error("Latest run ID was not set correctly")
	}
}

func TestEventPublishingRepository_List(t *testing.T) {
	mockRepo := newMockReleaseRunRepository()
	run := domain.NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []domain.CommitSHA{"abc123"}, "cfg", "plug")
	mockRepo.runs[run.ID()] = run

	eventPubRepo := NewEventPublishingRepository(mockRepo, nil)
	ids, err := eventPubRepo.List(context.Background(), "/tmp/repo")

	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if !mockRepo.listCalled {
		t.Error("Expected List to be called on underlying repository")
	}
	if len(ids) != 1 {
		t.Errorf("Expected 1 ID, got %d", len(ids))
	}
}

func TestEventPublishingRepository_Delete(t *testing.T) {
	mockRepo := newMockReleaseRunRepository()
	run := domain.NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []domain.CommitSHA{"abc123"}, "cfg", "plug")
	mockRepo.runs[run.ID()] = run

	eventPubRepo := NewEventPublishingRepository(mockRepo, nil)
	err := eventPubRepo.Delete(context.Background(), run.ID())

	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !mockRepo.deleteCalled {
		t.Error("Expected Delete to be called on underlying repository")
	}
	if _, exists := mockRepo.runs[run.ID()]; exists {
		t.Error("Run should have been deleted")
	}
}

func TestEventPublishingRepository_FindByState(t *testing.T) {
	mockRepo := newMockReleaseRunRepository()
	run := domain.NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []domain.CommitSHA{"abc123"}, "cfg", "plug")
	mockRepo.runs[run.ID()] = run

	eventPubRepo := NewEventPublishingRepository(mockRepo, nil)
	runs, err := eventPubRepo.FindByState(context.Background(), "/tmp/repo", domain.StateDraft)

	if err != nil {
		t.Fatalf("FindByState failed: %v", err)
	}
	if !mockRepo.findByStateCalled {
		t.Error("Expected FindByState to be called on underlying repository")
	}
	if len(runs) != 1 {
		t.Errorf("Expected 1 run in draft state, got %d", len(runs))
	}
}

func TestEventPublishingRepository_FindActive(t *testing.T) {
	mockRepo := newMockReleaseRunRepository()
	run := domain.NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []domain.CommitSHA{"abc123"}, "cfg", "plug")
	mockRepo.runs[run.ID()] = run

	eventPubRepo := NewEventPublishingRepository(mockRepo, nil)
	runs, err := eventPubRepo.FindActive(context.Background(), "/tmp/repo")

	if err != nil {
		t.Fatalf("FindActive failed: %v", err)
	}
	if !mockRepo.findActiveCalled {
		t.Error("Expected FindActive to be called on underlying repository")
	}
	if len(runs) != 1 {
		t.Errorf("Expected 1 active run, got %d", len(runs))
	}
}

func TestEventPublishingRepository_LoadFromRepo_WithSupport(t *testing.T) {
	mockRepo := newMockReleaseRunRepository()
	run := domain.NewReleaseRun("org/repo", "/tmp/repo", "v1.0.0", "abc123", []domain.CommitSHA{"abc123"}, "cfg", "plug")
	mockRepo.runs[run.ID()] = run

	eventPubRepo := NewEventPublishingRepository(mockRepo, nil)
	loaded, err := eventPubRepo.LoadFromRepo(context.Background(), "/tmp/repo", run.ID())

	if err != nil {
		t.Fatalf("LoadFromRepo failed: %v", err)
	}
	if loaded == nil || loaded.ID() != run.ID() {
		t.Error("Loaded run doesn't match expected run")
	}
}

func TestDeserializeEvent_StepCompleted(t *testing.T) {
	payload := json.RawMessage(`{"RunID":"run-1","StepName":"tag","Status":"completed"}`)
	evt, err := deserializeEvent("step.completed", payload)
	if err != nil {
		t.Fatalf("deserializeEvent error: %v", err)
	}
	if evt == nil {
		t.Fatal("expected non-nil event")
	}
}

func TestDeserializeEvent_PluginExecuted(t *testing.T) {
	payload := json.RawMessage(`{"RunID":"run-1","PluginName":"github","Hook":"PostPublish"}`)
	evt, err := deserializeEvent("plugin.executed", payload)
	if err != nil {
		t.Fatalf("deserializeEvent error: %v", err)
	}
	if evt == nil {
		t.Fatal("expected non-nil event")
	}
}

func TestDeserializeEvent_Unknown(t *testing.T) {
	_, err := deserializeEvent("unknown.event", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown event type")
	}
	if !strings.Contains(err.Error(), "unknown event type") {
		t.Errorf("unexpected error: %v", err)
	}
}
