package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// Mock implementations

type mockRepository struct {
	runs       map[domain.RunID]*domain.ReleaseRun
	latestRuns map[string]domain.RunID
	saveErr    error
	loadErr    error
	findErr    error
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		runs:       make(map[domain.RunID]*domain.ReleaseRun),
		latestRuns: make(map[string]domain.RunID),
	}
}

func (m *mockRepository) Save(_ context.Context, run *domain.ReleaseRun) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.runs[run.ID()] = run
	return nil
}

func (m *mockRepository) Load(_ context.Context, runID domain.RunID) (*domain.ReleaseRun, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	run, ok := m.runs[runID]
	if !ok {
		return nil, errors.New("run not found")
	}
	return run, nil
}

func (m *mockRepository) LoadLatest(_ context.Context, repoRoot string) (*domain.ReleaseRun, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	runID, ok := m.latestRuns[repoRoot]
	if !ok {
		return nil, errors.New("no latest run")
	}
	return m.Load(context.Background(), runID)
}

func (m *mockRepository) SetLatest(_ context.Context, repoRoot string, runID domain.RunID) error {
	m.latestRuns[repoRoot] = runID
	return nil
}

func (m *mockRepository) List(_ context.Context, _ string) ([]domain.RunID, error) {
	ids := make([]domain.RunID, 0, len(m.runs))
	for id := range m.runs {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *mockRepository) Delete(_ context.Context, runID domain.RunID) error {
	delete(m.runs, runID)
	return nil
}

func (m *mockRepository) FindByState(_ context.Context, _ string, state domain.RunState) ([]*domain.ReleaseRun, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var runs []*domain.ReleaseRun
	for _, run := range m.runs {
		if run.State() == state {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

func (m *mockRepository) FindActive(_ context.Context, _ string) ([]*domain.ReleaseRun, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var runs []*domain.ReleaseRun
	for _, run := range m.runs {
		if !run.State().IsFinal() {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

type mockRepoInspector struct {
	headSHA          domain.CommitSHA
	isClean          bool
	commits          []domain.CommitSHA
	remoteURL        string
	branch           string
	latestTag        string
	tagExists        bool
	releaseExists    bool
	headSHAErr       error
	isCleanErr       error
	commitsErr       error
	remoteURLErr     error
	branchErr        error
	latestTagErr     error
	tagExistsErr     error
	releaseExistsErr error
}

func newMockRepoInspector() *mockRepoInspector {
	return &mockRepoInspector{
		headSHA:   domain.CommitSHA("abc123def456"),
		isClean:   true,
		commits:   []domain.CommitSHA{"abc123def456", "def789012345"},
		remoteURL: "https://github.com/test/repo",
		branch:    "main",
		latestTag: "v1.0.0",
	}
}

func (m *mockRepoInspector) HeadSHA(_ context.Context) (domain.CommitSHA, error) {
	if m.headSHAErr != nil {
		return "", m.headSHAErr
	}
	return m.headSHA, nil
}

func (m *mockRepoInspector) IsClean(_ context.Context) (bool, error) {
	if m.isCleanErr != nil {
		return false, m.isCleanErr
	}
	return m.isClean, nil
}

func (m *mockRepoInspector) ResolveCommits(_ context.Context, _ string, _ domain.CommitSHA) ([]domain.CommitSHA, error) {
	if m.commitsErr != nil {
		return nil, m.commitsErr
	}
	return m.commits, nil
}

func (m *mockRepoInspector) GetRemoteURL(_ context.Context) (string, error) {
	if m.remoteURLErr != nil {
		return "", m.remoteURLErr
	}
	return m.remoteURL, nil
}

func (m *mockRepoInspector) GetCurrentBranch(_ context.Context) (string, error) {
	if m.branchErr != nil {
		return "", m.branchErr
	}
	return m.branch, nil
}

func (m *mockRepoInspector) GetLatestVersionTag(_ context.Context, _ string) (string, error) {
	if m.latestTagErr != nil {
		return "", m.latestTagErr
	}
	return m.latestTag, nil
}

func (m *mockRepoInspector) TagExists(_ context.Context, _ string) (bool, error) {
	if m.tagExistsErr != nil {
		return false, m.tagExistsErr
	}
	return m.tagExists, nil
}

func (m *mockRepoInspector) ReleaseExists(_ context.Context, _ string) (bool, error) {
	if m.releaseExistsErr != nil {
		return false, m.releaseExistsErr
	}
	return m.releaseExists, nil
}

type mockLockManager struct {
	acquireErr error
	isLocked   bool
}

func (m *mockLockManager) Acquire(_ context.Context, _ string, _ domain.RunID) (func(), error) {
	if m.acquireErr != nil {
		return nil, m.acquireErr
	}
	return func() {}, nil
}

func (m *mockLockManager) TryAcquire(_ context.Context, _ string, _ domain.RunID) (func(), bool, error) {
	if m.acquireErr != nil {
		return nil, false, m.acquireErr
	}
	return func() {}, true, nil
}

func (m *mockLockManager) IsLocked(_ context.Context, _ string, _ domain.RunID) (bool, error) {
	return m.isLocked, nil
}

type mockVersionWriter struct {
	writeErr error
}

func (m *mockVersionWriter) WriteVersion(_ context.Context, _ version.SemanticVersion) error {
	return m.writeErr
}

func (m *mockVersionWriter) WriteChangelog(_ context.Context, _ version.SemanticVersion, _ string) error {
	return m.writeErr
}

type mockNotesGenerator struct {
	notes    string
	provider string
	model    string
	err      error
}

func (m *mockNotesGenerator) Generate(_ context.Context, _ *domain.ReleaseRun, _ ports.NotesOptions) (*domain.ReleaseNotes, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &domain.ReleaseNotes{
		Text:        m.notes,
		Provider:    m.provider,
		Model:       m.model,
		GeneratedAt: time.Now(),
	}, nil
}

func (m *mockNotesGenerator) ComputeInputsHash(_ *domain.ReleaseRun, _ ports.NotesOptions) string {
	return "inputs-hash"
}

// Tests

func TestPlanReleaseUseCase_Execute(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	uc := NewPlanReleaseUseCase(repo, inspector, nil)

	input := PlanReleaseInput{
		RepoRoot:       "/path/to/repo",
		ConfigHash:     "config-hash",
		PluginPlanHash: "plugin-hash",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "user@example.com",
		},
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.RunID == "" {
		t.Error("Execute() returned empty RunID")
	}

	if output.HeadSHA != inspector.headSHA {
		t.Errorf("Execute() HeadSHA = %v, want %v", output.HeadSHA, inspector.headSHA)
	}

	// Verify run was saved
	if len(repo.runs) != 1 {
		t.Errorf("Expected 1 run saved, got %d", len(repo.runs))
	}

	// Verify run state
	savedRun := repo.runs[output.RunID]
	if savedRun.State() != domain.StatePlanned {
		t.Errorf("Saved run state = %v, want %v", savedRun.State(), domain.StatePlanned)
	}
}

func TestPlanReleaseUseCase_Execute_ActiveRunExists(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	// Create an active run
	activeRun := domain.NewReleaseRun(
		"repo", "/path/to/repo", "v1.0.0",
		domain.CommitSHA("abc123"), nil, "", "",
	)
	_ = activeRun.Plan("test")
	repo.runs[activeRun.ID()] = activeRun

	uc := NewPlanReleaseUseCase(repo, inspector, nil)

	input := PlanReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "user@example.com",
		},
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error when active run exists")
	}
}

func TestPlanReleaseUseCase_Execute_ForceOverride(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	// Create an active run
	activeRun := domain.NewReleaseRun(
		"repo", "/path/to/repo", "v1.0.0",
		domain.CommitSHA("abc123"), nil, "", "",
	)
	_ = activeRun.Plan("test")
	repo.runs[activeRun.ID()] = activeRun

	uc := NewPlanReleaseUseCase(repo, inspector, nil)

	input := PlanReleaseInput{
		RepoRoot: "/path/to/repo",
		Force:    true, // Force override
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "user@example.com",
		},
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() with Force error = %v", err)
	}

	if output.RunID == "" {
		t.Error("Execute() with Force returned empty RunID")
	}
}

func TestPlanReleaseUseCase_Execute_HeadSHAError(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	inspector.headSHAErr = errors.New("git error")

	uc := NewPlanReleaseUseCase(repo, inspector, nil)

	input := PlanReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "user@example.com",
		},
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error when HeadSHA fails")
	}
}

func TestPlanReleaseUseCase_Execute_CommitsError(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	inspector.commitsErr = errors.New("git error")

	uc := NewPlanReleaseUseCase(repo, inspector, nil)

	input := PlanReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "user@example.com",
		},
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error when ResolveCommits fails")
	}
}

func TestBumpVersionUseCase_Execute(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	lockMgr := &mockLockManager{}
	verWriter := &mockVersionWriter{}

	// Create a planned run with version proposal
	run := domain.NewReleaseRun(
		"repo", "/path/to/repo", "v1.0.0",
		domain.CommitSHA("abc123def456"), nil, "", "",
	)
	_ = run.SetVersionProposal(version.MustParse("1.0.0"), version.MustParse("1.1.0"), domain.BumpMinor, 0.95)
	_ = run.Plan("test")
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewBumpVersionUseCase(repo, inspector, lockMgr, verWriter, nil)

	input := BumpVersionInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "test-actor",
		},
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.VersionNext == "" {
		t.Error("Execute() returned empty VersionNext")
	}

	// Check state transitioned
	savedRun := repo.runs[run.ID()]
	if savedRun.State() != domain.StateVersioned {
		t.Errorf("Run state = %v, want %v", savedRun.State(), domain.StateVersioned)
	}
}

func TestBumpVersionUseCase_Execute_NoLatestRun(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	uc := NewBumpVersionUseCase(repo, inspector, nil, nil, nil)

	input := BumpVersionInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "test-actor",
		},
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error when no latest run")
	}
}

func TestGenerateNotesUseCase_Execute(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	// Create a versioned run
	run := domain.NewReleaseRun(
		"repo", "/path/to/repo", "v1.0.0",
		domain.CommitSHA("abc123def456"), nil, "", "",
	)
	_ = run.SetVersionProposal(version.MustParse("1.0.0"), version.MustParse("1.1.0"), domain.BumpMinor, 0.95)
	_ = run.Plan("test")
	_ = run.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
	_ = run.Bump("test")
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	notesGen := &mockNotesGenerator{
		notes:    "## Release Notes\n\n- Feature A",
		provider: "mock",
		model:    "test-model",
	}

	uc := NewGenerateNotesUseCase(repo, inspector, notesGen, nil)

	input := GenerateNotesInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "test-actor",
		},
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.Notes == nil || output.Notes.Text == "" {
		t.Error("Execute() returned empty Notes")
	}

	// Check state transitioned
	savedRun := repo.runs[run.ID()]
	if savedRun.State() != domain.StateNotesReady {
		t.Errorf("Run state = %v, want %v", savedRun.State(), domain.StateNotesReady)
	}
}

func TestGenerateNotesUseCase_Execute_GeneratorError(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	// Create a versioned run
	run := domain.NewReleaseRun(
		"repo", "/path/to/repo", "v1.0.0",
		domain.CommitSHA("abc123def456"), nil, "", "",
	)
	_ = run.SetVersionProposal(version.MustParse("1.0.0"), version.MustParse("1.1.0"), domain.BumpMinor, 0.95)
	_ = run.Plan("test")
	_ = run.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
	_ = run.Bump("test")
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	notesGen := &mockNotesGenerator{
		err: errors.New("AI error"),
	}

	uc := NewGenerateNotesUseCase(repo, inspector, notesGen, nil)

	input := GenerateNotesInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "test-actor",
		},
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error when generator fails")
	}
}

func TestApproveReleaseUseCase_Execute(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	// Create a notes-ready run
	run := createNotesReadyRun()
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewApproveReleaseUseCase(repo, inspector, nil, nil)

	input := ApproveReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "approver@example.com",
		},
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !output.Approved {
		t.Error("Execute() Approved = false, want true")
	}

	// Check state transitioned
	savedRun := repo.runs[run.ID()]
	if savedRun.State() != domain.StateApproved {
		t.Errorf("Run state = %v, want %v", savedRun.State(), domain.StateApproved)
	}
}

func TestApproveReleaseUseCase_Execute_AlreadyApproved(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	// Create an already approved run
	run := createNotesReadyRun()
	_ = run.Approve("previous-approver", false)
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewApproveReleaseUseCase(repo, inspector, nil, nil)

	input := ApproveReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "approver@example.com",
		},
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error when already approved")
	}
}

func TestGetStatusUseCase_Execute(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	// Create a run
	run := domain.NewReleaseRun(
		"repo", "/path/to/repo", "v1.0.0",
		domain.CommitSHA("abc123def456"), nil, "", "",
	)
	_ = run.Plan("test")
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewGetStatusUseCase(repo, inspector)

	input := GetStatusInput{
		RepoRoot: "/path/to/repo",
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.State != domain.StatePlanned {
		t.Errorf("Execute() State = %v, want %v", output.State, domain.StatePlanned)
	}

	if output.NextAction == "" {
		t.Error("Execute() NextAction is empty")
	}

	if output.NextAction != "bump" {
		t.Errorf("Execute() NextAction = %v, want bump", output.NextAction)
	}
}

func TestGetStatusUseCase_Execute_NoRun(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	uc := NewGetStatusUseCase(repo, inspector)

	input := GetStatusInput{
		RepoRoot: "/path/to/repo",
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error when no run exists")
	}
}

func TestDetermineNextAction(t *testing.T) {
	tests := []struct {
		state  domain.RunState
		action string
	}{
		{domain.StateDraft, "plan"},
		{domain.StatePlanned, "bump"},
		{domain.StateVersioned, "notes"},
		{domain.StateNotesReady, "approve"},
		{domain.StateApproved, "publish"},
		{domain.StatePublishing, "wait"},
		{domain.StatePublished, "done"},
		{domain.StateFailed, "retry or cancel"},
		{domain.StateCanceled, "plan"},
	}

	for _, tt := range tests {
		got := determineNextAction(tt.state)
		if got != tt.action {
			t.Errorf("determineNextAction(%v) = %v, want %v", tt.state, got, tt.action)
		}
	}
}

// Helper functions

func createNotesReadyRun() *domain.ReleaseRun {
	run := domain.NewReleaseRun(
		"repo", "/path/to/repo", "v1.0.0",
		domain.CommitSHA("abc123def456"), nil, "", "",
	)
	_ = run.SetVersionProposal(version.MustParse("1.0.0"), version.MustParse("1.1.0"), domain.BumpMinor, 0.95)
	_ = run.Plan("test")
	_ = run.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
	_ = run.Bump("test")
	notes := &domain.ReleaseNotes{
		Text:        "## Release Notes",
		Provider:    "test",
		GeneratedAt: time.Now(),
	}
	_ = run.GenerateNotes(notes, "hash", "test")
	return run
}
