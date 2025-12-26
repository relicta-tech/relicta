// Package adapters provides infrastructure implementations for the release governance domain.
package adapters

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	// Load without repo context should fail
	_, err := repo.Load(ctx, "some-id")
	if err == nil {
		t.Error("Expected error from Load without repo context")
	}
	if !strings.Contains(err.Error(), "requires repo root context") {
		t.Errorf("Expected 'requires repo root context' error, got: %v", err)
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
	if string(data) != string(machineJSON) {
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
		branchErr: os.ErrNotExist,
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
	expected := "/tmp/myrepo/.relicta/runs"
	if rp != expected {
		t.Errorf("runsPath mismatch: got %s, want %s", rp, expected)
	}

	// Test runPath
	runP := runPath(repoRoot, runID)
	expected = "/tmp/myrepo/.relicta/runs/test-run-123.json"
	if runP != expected {
		t.Errorf("runPath mismatch: got %s, want %s", runP, expected)
	}

	// Test latestPath
	lp := latestPath(repoRoot)
	expected = "/tmp/myrepo/.relicta/runs/latest"
	if lp != expected {
		t.Errorf("latestPath mismatch: got %s, want %s", lp, expected)
	}

	// Test machinePath
	mp := machinePath(repoRoot, runID)
	expected = "/tmp/myrepo/.relicta/runs/test-run-123.machine.json"
	if mp != expected {
		t.Errorf("machinePath mismatch: got %s, want %s", mp, expected)
	}

	// Test statePath
	sp := statePath(repoRoot, runID)
	expected = "/tmp/myrepo/.relicta/runs/test-run-123.state.json"
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
