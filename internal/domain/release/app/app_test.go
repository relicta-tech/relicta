package app

import (
	"context"
	"errors"
	"strings"
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

func (m *mockRepository) LoadBatch(_ context.Context, _ string, runIDs []domain.RunID) (map[domain.RunID]*domain.ReleaseRun, error) {
	result := make(map[domain.RunID]*domain.ReleaseRun)
	for _, runID := range runIDs {
		if run, ok := m.runs[runID]; ok {
			result[runID] = run
		}
	}
	return result, nil
}

func (m *mockRepository) LoadLatest(ctx context.Context, repoRoot string) (*domain.ReleaseRun, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	runID, ok := m.latestRuns[repoRoot]
	if !ok {
		return nil, errors.New("no latest run")
	}
	return m.Load(ctx, runID)
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

func (m *mockRepository) FindByPlanHash(_ context.Context, _ string, planHash string) (*domain.ReleaseRun, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	for _, run := range m.runs {
		if run.PlanHash() == planHash {
			return run, nil
		}
	}
	return nil, nil
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

func TestPlanReleaseUseCase_Execute_EmptyActorID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	uc := NewPlanReleaseUseCase(repo, inspector, nil)

	input := PlanReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "", // Empty Actor ID - should fail validation
		},
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Fatal("Execute() expected error for empty Actor.ID")
	}

	// Verify it's a ValidationError
	if !IsValidationError(err) {
		t.Errorf("Execute() error should be ValidationError, got %T", err)
	}

	// Verify error message contains field name
	if !contains(err.Error(), "Actor.ID") {
		t.Errorf("Execute() error should mention Actor.ID, got %q", err.Error())
	}
}

// TestPlanReleaseUseCase_Execute_TagPushMode tests tag-push mode scenarios using table-driven tests.
func TestPlanReleaseUseCase_Execute_TagPushMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		tagPushMode    bool
		tagName        string
		nextVersion    *version.SemanticVersion
		currentVersion *version.SemanticVersion
		bumpKind       *domain.BumpKind
		wantState      domain.RunState
		wantTagName    string
		wantVersion    string
		wantBumpKind   domain.BumpKind
		wantErr        bool
		errContains    string
	}{
		{
			name:           "tag-push mode transitions to versioned state",
			tagPushMode:    true,
			tagName:        "v2.0.0",
			nextVersion:    ptr(version.MustParse("2.0.0")),
			currentVersion: ptr(version.MustParse("1.0.0")),
			bumpKind:       ptr(domain.BumpMajor),
			wantState:      domain.StateVersioned,
			wantTagName:    "v2.0.0",
			wantVersion:    "2.0.0",
			wantBumpKind:   domain.BumpMajor,
			wantErr:        false,
		},
		{
			name:           "tag-push mode defaults tag name from version",
			tagPushMode:    true,
			tagName:        "", // Empty - should default to "v" + version
			nextVersion:    ptr(version.MustParse("1.5.0")),
			currentVersion: ptr(version.MustParse("1.4.0")),
			bumpKind:       ptr(domain.BumpMinor),
			wantState:      domain.StateVersioned,
			wantTagName:    "v1.5.0",
			wantVersion:    "1.5.0",
			wantBumpKind:   domain.BumpMinor,
			wantErr:        false,
		},
		{
			name:           "tag-push mode requires NextVersion",
			tagPushMode:    true,
			tagName:        "v1.0.0",
			nextVersion:    nil, // Missing - should fail validation
			currentVersion: ptr(version.MustParse("0.9.0")),
			bumpKind:       ptr(domain.BumpPatch),
			wantErr:        true,
			errContains:    "tag-push mode requires NextVersion",
		},
		{
			name:           "non-tag-push mode stays in planned state",
			tagPushMode:    false,
			tagName:        "",
			nextVersion:    ptr(version.MustParse("1.1.0")),
			currentVersion: ptr(version.MustParse("1.0.0")),
			bumpKind:       ptr(domain.BumpMinor),
			wantState:      domain.StatePlanned,
			wantVersion:    "1.1.0",
			wantBumpKind:   domain.BumpMinor,
			wantErr:        false,
		},
		{
			name:           "tag-push mode with patch version",
			tagPushMode:    true,
			tagName:        "v1.0.1",
			nextVersion:    ptr(version.MustParse("1.0.1")),
			currentVersion: ptr(version.MustParse("1.0.0")),
			bumpKind:       ptr(domain.BumpPatch),
			wantState:      domain.StateVersioned,
			wantTagName:    "v1.0.1",
			wantVersion:    "1.0.1",
			wantBumpKind:   domain.BumpPatch,
			wantErr:        false,
		},
		// Edge cases from specialist reviews
		{
			name:           "tag-push mode with nil BumpKind succeeds",
			tagPushMode:    true,
			tagName:        "v1.2.0",
			nextVersion:    ptr(version.MustParse("1.2.0")),
			currentVersion: ptr(version.MustParse("1.1.0")),
			bumpKind:       nil, // BumpKind is optional - version proposal won't be set
			wantState:      domain.StateVersioned,
			wantTagName:    "v1.2.0",
			wantVersion:    "1.2.0",
			wantErr:        false,
		},
		{
			name:           "tag-push mode with nil CurrentVersion (initial release)",
			tagPushMode:    true,
			tagName:        "v1.0.0",
			nextVersion:    ptr(version.MustParse("1.0.0")),
			currentVersion: nil, // Initial release - no previous version
			bumpKind:       ptr(domain.BumpMajor),
			wantState:      domain.StateVersioned,
			wantTagName:    "v1.0.0",
			wantVersion:    "1.0.0",
			wantErr:        false,
		},
		{
			name:           "tag-push mode with prerelease version",
			tagPushMode:    true,
			tagName:        "v2.0.0-beta.1",
			nextVersion:    ptr(version.MustParse("2.0.0-beta.1")),
			currentVersion: ptr(version.MustParse("1.0.0")),
			bumpKind:       ptr(domain.BumpPrerelease),
			wantState:      domain.StateVersioned,
			wantTagName:    "v2.0.0-beta.1",
			wantVersion:    "2.0.0-beta.1",
			wantBumpKind:   domain.BumpPrerelease,
			wantErr:        false,
		},
		{
			name:           "tag-push mode with build metadata",
			tagPushMode:    true,
			tagName:        "v1.0.0+build.123",
			nextVersion:    ptr(version.MustParse("1.0.0+build.123")),
			currentVersion: ptr(version.MustParse("0.9.0")),
			bumpKind:       ptr(domain.BumpMinor),
			wantState:      domain.StateVersioned,
			wantTagName:    "v1.0.0+build.123",
			wantVersion:    "1.0.0+build.123",
			wantBumpKind:   domain.BumpMinor,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
				TagPushMode:    tt.tagPushMode,
				TagName:        tt.tagName,
				CurrentVersion: tt.currentVersion,
				NextVersion:    tt.nextVersion,
				BumpKind:       tt.bumpKind,
			}

			output, err := uc.Execute(ctx, input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Execute() expected error, got nil")
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("Execute() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if output.RunID == "" {
				t.Error("Execute() returned empty RunID")
			}

			savedRun := repo.runs[output.RunID]

			// State assertion
			if savedRun.State() != tt.wantState {
				t.Errorf("Run state = %v, want %v", savedRun.State(), tt.wantState)
			}

			// Tag name assertion
			if tt.wantTagName != "" && savedRun.TagName() != tt.wantTagName {
				t.Errorf("Run tagName = %q, want %q", savedRun.TagName(), tt.wantTagName)
			}

			// Deeper assertions: VersionNext
			if tt.wantVersion != "" && savedRun.VersionNext().String() != tt.wantVersion {
				t.Errorf("Run VersionNext = %v, want %v", savedRun.VersionNext(), tt.wantVersion)
			}

			// Deeper assertions: BumpKind (only check if expected and set)
			if tt.wantBumpKind != "" && savedRun.BumpKind() != tt.wantBumpKind {
				t.Errorf("Run BumpKind = %v, want %v", savedRun.BumpKind(), tt.wantBumpKind)
			}

			// Deeper assertions: PlanHash should not be empty
			if savedRun.PlanHash() == "" {
				t.Error("Run PlanHash should not be empty")
			}

			// Deeper assertions: UpdatedAt should be set
			if savedRun.UpdatedAt().IsZero() {
				t.Error("Run UpdatedAt should be set")
			}

			// Deeper assertions: HeadSHA should match mock
			if savedRun.HeadSHA() == "" {
				t.Error("Run HeadSHA should not be empty")
			}
		})
	}
}

// contains checks if s contains substr (helper for error checking).
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestTagPushModeWorkflow tests the complete tag-push workflow:
// plan (tag-push mode) → notes → approve → publish
// This verifies that notes can be generated without running bump.
func TestTagPushModeWorkflow(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	// Step 1: Plan with tag-push mode
	planUC := NewPlanReleaseUseCase(repo, inspector, nil)

	nextVersion := version.MustParse("3.0.0")
	bumpKind := domain.BumpMajor

	planInput := PlanReleaseInput{
		RepoRoot:       "/path/to/repo",
		ConfigHash:     "config-hash",
		PluginPlanHash: "plugin-hash",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "user@example.com",
		},
		TagPushMode:    true,
		TagName:        "v3.0.0",
		CurrentVersion: ptr(version.MustParse("2.0.0")),
		NextVersion:    &nextVersion,
		BumpKind:       &bumpKind,
	}

	planOutput, err := planUC.Execute(ctx, planInput)
	if err != nil {
		t.Fatalf("Plan Execute() error = %v", err)
	}

	// Verify run is in versioned state (ready for notes)
	run := repo.runs[planOutput.RunID]
	if run.State() != domain.StateVersioned {
		t.Fatalf("After plan, state = %v, want %v", run.State(), domain.StateVersioned)
	}

	// Step 2: Generate notes (should work without bump!)
	mockNotesGen := &mockNotesGenerator{
		notes:    "## Release Notes\n\nMajor version bump with breaking changes.",
		provider: "mock",
		model:    "test",
	}
	notesUC := NewGenerateNotesUseCase(repo, inspector, mockNotesGen, nil)

	notesInput := GenerateNotesInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "user@example.com",
		},
	}

	_, err = notesUC.Execute(ctx, notesInput)
	if err != nil {
		t.Fatalf("Notes Execute() error = %v (this was the bug - notes failed in tag-push mode)", err)
	}

	// Verify state advanced to notes_ready
	run = repo.runs[planOutput.RunID]
	if run.State() != domain.StateNotesReady {
		t.Errorf("After notes, state = %v, want %v", run.State(), domain.StateNotesReady)
	}

	// Step 3: Approve
	lockMgr := &mockLockManager{}
	approveUC := NewApproveReleaseUseCase(repo, inspector, lockMgr, nil)

	approveInput := ApproveReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "approver@example.com",
		},
	}

	_, err = approveUC.Execute(ctx, approveInput)
	if err != nil {
		t.Fatalf("Approve Execute() error = %v", err)
	}

	// Verify state advanced to approved
	run = repo.runs[planOutput.RunID]
	if run.State() != domain.StateApproved {
		t.Errorf("After approve, state = %v, want %v", run.State(), domain.StateApproved)
	}
}

// ptr is a helper to create a pointer to a value.
func ptr[T any](v T) *T {
	return &v
}

// TestTagPushModeRecordsEvent verifies that TagPushModeDetectedEvent is recorded
// when tag-push mode is used. This event is essential for audit trails.
func TestTagPushModeRecordsEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	uc := NewPlanReleaseUseCase(repo, inspector, nil)

	nextVersion := version.MustParse("2.0.0")
	bumpKind := domain.BumpMajor

	input := PlanReleaseInput{
		RepoRoot:       "/path/to/repo",
		ConfigHash:     "config-hash",
		PluginPlanHash: "plugin-hash",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "user@example.com",
		},
		TagPushMode:    true,
		TagName:        "v2.0.0",
		CurrentVersion: ptr(version.MustParse("1.0.0")),
		NextVersion:    &nextVersion,
		BumpKind:       &bumpKind,
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	savedRun := repo.runs[output.RunID]

	// Verify TagPushModeDetectedEvent is among the domain events
	events := savedRun.DomainEvents()
	var foundEvent *domain.TagPushModeDetectedEvent
	for _, evt := range events {
		if tpe, ok := evt.(*domain.TagPushModeDetectedEvent); ok {
			foundEvent = tpe
			break
		}
	}

	if foundEvent == nil {
		t.Fatal("TagPushModeDetectedEvent not found in domain events")
	}

	// Verify event fields
	if foundEvent.TagName != "v2.0.0" {
		t.Errorf("TagPushModeDetectedEvent.TagName = %q, want %q", foundEvent.TagName, "v2.0.0")
	}
	if foundEvent.Actor != "user@example.com" {
		t.Errorf("TagPushModeDetectedEvent.Actor = %q, want %q", foundEvent.Actor, "user@example.com")
	}
	if foundEvent.VersionNext.String() != "2.0.0" {
		t.Errorf("TagPushModeDetectedEvent.VersionNext = %v, want 2.0.0", foundEvent.VersionNext)
	}
	if foundEvent.EventName() != "run.tag_push_mode_detected" {
		t.Errorf("TagPushModeDetectedEvent.EventName() = %q, want %q", foundEvent.EventName(), "run.tag_push_mode_detected")
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

// Mock publisher for publish tests
type mockPublisher struct {
	stepResults      map[string]*ports.StepResult
	checkIdempotency map[string]bool
	executeErr       error
	checkErr         error
}

func newMockPublisher() *mockPublisher {
	return &mockPublisher{
		stepResults:      make(map[string]*ports.StepResult),
		checkIdempotency: make(map[string]bool),
	}
}

func (m *mockPublisher) ExecuteStep(_ context.Context, _ *domain.ReleaseRun, step *domain.StepPlan) (*ports.StepResult, error) {
	if m.executeErr != nil {
		return nil, m.executeErr
	}
	if result, ok := m.stepResults[step.Name]; ok {
		return result, nil
	}
	return &ports.StepResult{
		Success: true,
		Output:  "step completed",
	}, nil
}

func (m *mockPublisher) CheckIdempotency(_ context.Context, _ *domain.ReleaseRun, step *domain.StepPlan) (bool, error) {
	if m.checkErr != nil {
		return false, m.checkErr
	}
	return m.checkIdempotency[step.Name], nil
}

func TestPublishReleaseUseCase_Execute(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	lockMgr := &mockLockManager{}
	publisher := newMockPublisher()

	// Create an approved run with execution plan
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{
		{Name: "tag", Type: domain.StepTypeTag},
		{Name: "notify", Type: domain.StepTypeNotify},
	})
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewPublishReleaseUseCase(repo, inspector, lockMgr, publisher, nil)

	input := PublishReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "publisher@example.com",
		},
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !output.Published {
		t.Error("Execute() Published = false, want true")
	}

	if len(output.StepResults) != 2 {
		t.Errorf("Execute() StepResults count = %d, want 2", len(output.StepResults))
	}

	// Verify final state
	savedRun := repo.runs[run.ID()]
	if savedRun.State() != domain.StatePublished {
		t.Errorf("Run state = %v, want %v", savedRun.State(), domain.StatePublished)
	}
}

func TestPublishReleaseUseCase_Execute_AlreadyPublished(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	// Create a published run
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{{Name: "tag", Type: domain.StepTypeTag}})
	_ = run.StartPublishing("test")
	_ = run.MarkStepDone("tag", "done")
	_ = run.MarkPublished("test")
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewPublishReleaseUseCase(repo, inspector, nil, nil, nil)

	input := PublishReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "publisher@example.com",
		},
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !output.Published {
		t.Error("Execute() Published = false for already published run, want true")
	}
}

func TestPublishReleaseUseCase_Execute_WrongState(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	// Create a notes-ready run (not approved)
	run := createNotesReadyRun()
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewPublishReleaseUseCase(repo, inspector, nil, nil, nil)

	input := PublishReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "publisher@example.com",
		},
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error for notes_ready state")
	}
}

func TestPublishReleaseUseCase_Execute_HeadMismatch(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	inspector.headSHA = domain.CommitSHA("different-sha")

	// Create an approved run
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{{Name: "tag", Type: domain.StepTypeTag}})
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewPublishReleaseUseCase(repo, inspector, nil, nil, nil)

	input := PublishReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "publisher@example.com",
		},
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error for HEAD mismatch")
	}
}

func TestPublishReleaseUseCase_Execute_ForceBypassesHeadCheck(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	inspector.headSHA = domain.CommitSHA("different-sha")
	publisher := newMockPublisher()

	// Create an approved run
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{{Name: "tag", Type: domain.StepTypeTag}})
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewPublishReleaseUseCase(repo, inspector, nil, publisher, nil)

	input := PublishReleaseInput{
		RepoRoot: "/path/to/repo",
		Force:    true, // Force should bypass HEAD check
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "publisher@example.com",
		},
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() with Force error = %v", err)
	}

	if !output.Published {
		t.Error("Execute() with Force Published = false, want true")
	}
}

func TestPublishReleaseUseCase_Execute_StepFailure(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	publisher := newMockPublisher()
	publisher.stepResults["tag"] = &ports.StepResult{
		Success: false,
		Error:   errors.New("tag creation failed"),
	}

	// Create an approved run
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{
		{Name: "tag", Type: domain.StepTypeTag},
		{Name: "notify", Type: domain.StepTypeNotify},
	})
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewPublishReleaseUseCase(repo, inspector, nil, publisher, nil)

	input := PublishReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "publisher@example.com",
		},
	}

	output, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error when step fails")
	}

	if output.Published {
		t.Error("Execute() Published = true after step failure, want false")
	}

	// Verify first step result is recorded
	if len(output.StepResults) != 1 {
		t.Errorf("Execute() StepResults count = %d, want 1", len(output.StepResults))
	}
}

func TestPublishReleaseUseCase_Execute_DryRun(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	publisher := newMockPublisher()

	// Create an approved run
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{
		{Name: "tag", Type: domain.StepTypeTag},
		{Name: "notify", Type: domain.StepTypeNotify},
	})
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewPublishReleaseUseCase(repo, inspector, nil, publisher, nil)

	input := PublishReleaseInput{
		RepoRoot: "/path/to/repo",
		DryRun:   true,
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "publisher@example.com",
		},
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() with DryRun error = %v", err)
	}

	// All steps should be skipped in dry run
	for _, result := range output.StepResults {
		if !result.Skipped {
			t.Errorf("Step %s Skipped = false in dry run, want true", result.StepName)
		}
	}
}

func TestPublishReleaseUseCase_Execute_IdempotentStep(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	publisher := newMockPublisher()
	publisher.checkIdempotency["tag"] = true // Tag already exists

	// Create an approved run
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{
		{Name: "tag", Type: domain.StepTypeTag},
	})
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewPublishReleaseUseCase(repo, inspector, nil, publisher, nil)

	input := PublishReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "publisher@example.com",
		},
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Step should be marked as skipped due to idempotency
	if len(output.StepResults) != 1 {
		t.Fatalf("Execute() StepResults count = %d, want 1", len(output.StepResults))
	}
	if !output.StepResults[0].Skipped {
		t.Error("Step should be skipped due to idempotency check")
	}
}

func TestPublishReleaseUseCase_Execute_LockError(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	lockMgr := &mockLockManager{acquireErr: errors.New("lock failed")}

	// Create an approved run
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{{Name: "tag", Type: domain.StepTypeTag}})
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewPublishReleaseUseCase(repo, inspector, lockMgr, nil, nil)

	input := PublishReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "publisher@example.com",
		},
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error when lock fails")
	}
}

func TestPublishReleaseUseCase_Execute_ResumePublishing(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	publisher := newMockPublisher()

	// Create a run that's already publishing with one step done
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{
		{Name: "tag", Type: domain.StepTypeTag},
		{Name: "notify", Type: domain.StepTypeNotify},
	})
	_ = run.StartPublishing("test")
	_ = run.MarkStepDone("tag", "done") // First step already done
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewPublishReleaseUseCase(repo, inspector, nil, publisher, nil)

	input := PublishReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "publisher@example.com",
		},
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Should only execute the second step
	if len(output.StepResults) != 1 {
		t.Errorf("Execute() StepResults count = %d, want 1 (only pending step)", len(output.StepResults))
	}

	if output.StepResults[0].StepName != "notify" {
		t.Errorf("Execute() executed step %s, want notify", output.StepResults[0].StepName)
	}

	if !output.Published {
		t.Error("Execute() Published = false, want true")
	}
}

func TestPublishReleaseUseCase_Execute_ByRunID(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	publisher := newMockPublisher()

	// Create an approved run
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{{Name: "tag", Type: domain.StepTypeTag}})
	repo.runs[run.ID()] = run

	uc := NewPublishReleaseUseCase(repo, inspector, nil, publisher, nil)

	input := PublishReleaseInput{
		RepoRoot: "/path/to/repo",
		RunID:    run.ID(), // Explicitly specify run ID
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "publisher@example.com",
		},
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.RunID != run.ID() {
		t.Errorf("Execute() RunID = %v, want %v", output.RunID, run.ID())
	}
}

func TestPublishReleaseUseCase_Execute_NoPublisher(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	// Create an approved run
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{{Name: "tag", Type: domain.StepTypeTag}})
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	// No publisher configured
	uc := NewPublishReleaseUseCase(repo, inspector, nil, nil, nil)

	input := PublishReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "publisher@example.com",
		},
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error when no publisher configured")
	}
}

func TestPublishReleaseUseCase_Execute_ExecuteStepError(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	publisher := newMockPublisher()
	publisher.executeErr = errors.New("execution error")

	// Create an approved run
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{{Name: "tag", Type: domain.StepTypeTag}})
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewPublishReleaseUseCase(repo, inspector, nil, publisher, nil)

	input := PublishReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "publisher@example.com",
		},
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error when ExecuteStep fails")
	}
}

// Tests for RetryPublishUseCase

func TestRetryPublishUseCase_Execute(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	publisher := newMockPublisher()

	// Create a failed run
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{
		{Name: "tag", Type: domain.StepTypeTag},
		{Name: "notify", Type: domain.StepTypeNotify},
	})
	_ = run.StartPublishing("test")
	_ = run.MarkStepStarted("tag")
	_ = run.MarkStepFailed("tag", errors.New("tag failed"))
	_ = run.MarkFailed("step failed", "test")
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewRetryPublishUseCase(repo, inspector, nil, publisher, nil)

	input := RetryPublishInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "retry@example.com",
		},
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !output.Published {
		t.Error("Execute() Published = false, want true")
	}
}

func TestRetryPublishUseCase_Execute_NotFailed(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	// Create an approved run (not failed)
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewRetryPublishUseCase(repo, inspector, nil, nil, nil)

	input := RetryPublishInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "retry@example.com",
		},
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error when run is not failed")
	}
}

func TestRetryPublishUseCase_Execute_HeadMismatch(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	inspector.headSHA = domain.CommitSHA("different-sha")

	// Create a failed run
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{{Name: "tag", Type: domain.StepTypeTag}})
	_ = run.StartPublishing("test")
	_ = run.MarkFailed("error", "test")
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewRetryPublishUseCase(repo, inspector, nil, nil, nil)

	input := RetryPublishInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "retry@example.com",
		},
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error for HEAD mismatch")
	}
}

func TestRetryPublishUseCase_Execute_ForceBypassesHeadCheck(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	inspector.headSHA = domain.CommitSHA("different-sha")
	publisher := newMockPublisher()

	// Create a failed run
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{{Name: "tag", Type: domain.StepTypeTag}})
	_ = run.StartPublishing("test")
	_ = run.MarkStepStarted("tag")
	_ = run.MarkStepFailed("tag", errors.New("tag failed"))
	_ = run.MarkFailed("error", "test")
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewRetryPublishUseCase(repo, inspector, nil, publisher, nil)

	input := RetryPublishInput{
		RepoRoot: "/path/to/repo",
		Force:    true,
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "retry@example.com",
		},
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() with Force error = %v", err)
	}

	if !output.Published {
		t.Error("Execute() with Force Published = false, want true")
	}
}

func TestRetryPublishUseCase_Execute_LockError(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	lockMgr := &mockLockManager{acquireErr: errors.New("lock failed")}

	// Create a failed run
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{{Name: "tag", Type: domain.StepTypeTag}})
	_ = run.StartPublishing("test")
	_ = run.MarkFailed("error", "test")
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewRetryPublishUseCase(repo, inspector, lockMgr, nil, nil)

	input := RetryPublishInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "retry@example.com",
		},
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error when lock fails")
	}
}

func TestRetryPublishUseCase_Execute_ByRunID(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	publisher := newMockPublisher()

	// Create a failed run
	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{{Name: "tag", Type: domain.StepTypeTag}})
	_ = run.StartPublishing("test")
	_ = run.MarkStepStarted("tag")
	_ = run.MarkStepFailed("tag", errors.New("tag failed"))
	_ = run.MarkFailed("error", "test")
	repo.runs[run.ID()] = run

	uc := NewRetryPublishUseCase(repo, inspector, nil, publisher, nil)

	input := RetryPublishInput{
		RepoRoot: "/path/to/repo",
		RunID:    run.ID(),
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "retry@example.com",
		},
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.RunID != run.ID() {
		t.Errorf("Execute() RunID = %v, want %v", output.RunID, run.ID())
	}
}

// Additional edge case tests for existing use cases

func TestBumpVersionUseCase_Execute_WriteError(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	lockMgr := &mockLockManager{}
	verWriter := &mockVersionWriter{writeErr: errors.New("write error")}

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

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error when version write fails")
	}
}

func TestBumpVersionUseCase_Execute_HeadMismatch(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	inspector.headSHA = domain.CommitSHA("different-sha")

	// Create a planned run
	run := domain.NewReleaseRun(
		"repo", "/path/to/repo", "v1.0.0",
		domain.CommitSHA("abc123def456"), nil, "", "",
	)
	_ = run.SetVersionProposal(version.MustParse("1.0.0"), version.MustParse("1.1.0"), domain.BumpMinor, 0.95)
	_ = run.Plan("test")
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

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
		t.Error("Execute() expected error for HEAD mismatch")
	}
}

func TestGenerateNotesUseCase_Execute_NoRun(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	uc := NewGenerateNotesUseCase(repo, inspector, nil, nil)

	input := GenerateNotesInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "test-actor",
		},
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error when no run exists")
	}
}

func TestApproveReleaseUseCase_Execute_NoRun(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

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
		t.Error("Execute() expected error when no run exists")
	}
}

func TestPublishReleaseUseCase_Execute_NoRun(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	uc := NewPublishReleaseUseCase(repo, inspector, nil, nil, nil)

	input := PublishReleaseInput{
		RepoRoot: "/path/to/repo",
		Actor: ports.ActorInfo{
			Type: domain.ActorHuman,
			ID:   "publisher@example.com",
		},
	}

	_, err := uc.Execute(ctx, input)
	if err == nil {
		t.Error("Execute() expected error when no run exists")
	}
}

// Additional tests for GetStatusUseCase

func TestGetStatusUseCase_Execute_AllStates(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		state          domain.RunState
		expectedAction string
	}{
		{"draft", domain.StateDraft, "plan"},
		{"planned", domain.StatePlanned, "bump"},
		{"versioned", domain.StateVersioned, "notes"},
		{"notes_ready", domain.StateNotesReady, "approve"},
		{"approved", domain.StateApproved, "publish"},
		{"publishing", domain.StatePublishing, "wait"},
		{"published", domain.StatePublished, "done"},
		{"failed", domain.StateFailed, "retry or cancel"},
		{"canceled", domain.StateCanceled, "plan"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMockRepository()
			inspector := newMockRepoInspector()

			run := createRunInState(tc.state)
			repo.runs[run.ID()] = run
			repo.latestRuns["/path/to/repo"] = run.ID()

			uc := NewGetStatusUseCase(repo, inspector)
			output, err := uc.Execute(ctx, GetStatusInput{RepoRoot: "/path/to/repo"})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if output.NextAction != tc.expectedAction {
				t.Errorf("NextAction = %v, want %v", output.NextAction, tc.expectedAction)
			}
		})
	}
}

func TestGetStatusUseCase_Execute_HeadDrift(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()
	inspector.headSHA = domain.CommitSHA("different-sha")

	run := createNotesReadyRun()
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewGetStatusUseCase(repo, inspector)
	output, err := uc.Execute(ctx, GetStatusInput{RepoRoot: "/path/to/repo"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.Warning == "" {
		t.Error("Execute() expected warning for HEAD drift")
	}
}

func TestGetStatusUseCase_Execute_ByRunID(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	run := createNotesReadyRun()
	repo.runs[run.ID()] = run

	uc := NewGetStatusUseCase(repo, inspector)
	output, err := uc.Execute(ctx, GetStatusInput{
		RepoRoot: "/path/to/repo",
		RunID:    run.ID(),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.RunID != run.ID() {
		t.Errorf("Execute() RunID = %v, want %v", output.RunID, run.ID())
	}
}

func TestGetStatusUseCase_Execute_PublishedRun(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	inspector := newMockRepoInspector()

	run := createNotesReadyRun()
	_ = run.Approve("approver", false)
	run.SetExecutionPlan([]domain.StepPlan{{Name: "tag", Type: domain.StepTypeTag}})
	_ = run.StartPublishing("test")
	_ = run.MarkStepDone("tag", "done")
	_ = run.MarkPublished("test")
	repo.runs[run.ID()] = run
	repo.latestRuns["/path/to/repo"] = run.ID()

	uc := NewGetStatusUseCase(repo, inspector)
	output, err := uc.Execute(ctx, GetStatusInput{RepoRoot: "/path/to/repo"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.State != domain.StatePublished {
		t.Errorf("Execute() State = %v, want %v", output.State, domain.StatePublished)
	}

	if output.StepsDone != 1 {
		t.Errorf("Execute() StepsDone = %d, want 1", output.StepsDone)
	}
}

// createRunInState creates a run in a specific state for testing
func createRunInState(state domain.RunState) *domain.ReleaseRun {
	run := domain.NewReleaseRun(
		"repo", "/path/to/repo", "v1.0.0",
		domain.CommitSHA("abc123def456"), nil, "", "",
	)
	_ = run.SetVersionProposal(version.MustParse("1.0.0"), version.MustParse("1.1.0"), domain.BumpMinor, 0.95)

	switch state {
	case domain.StateDraft:
		return run
	case domain.StatePlanned:
		_ = run.Plan("test")
		return run
	case domain.StateVersioned:
		_ = run.Plan("test")
		_ = run.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
		_ = run.Bump("test")
		return run
	case domain.StateNotesReady:
		_ = run.Plan("test")
		_ = run.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
		_ = run.Bump("test")
		notes := &domain.ReleaseNotes{Text: "notes", Provider: "test", GeneratedAt: time.Now()}
		_ = run.GenerateNotes(notes, "hash", "test")
		return run
	case domain.StateApproved:
		_ = run.Plan("test")
		_ = run.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
		_ = run.Bump("test")
		notes := &domain.ReleaseNotes{Text: "notes", Provider: "test", GeneratedAt: time.Now()}
		_ = run.GenerateNotes(notes, "hash", "test")
		_ = run.Approve("test", false)
		return run
	case domain.StatePublishing:
		_ = run.Plan("test")
		_ = run.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
		_ = run.Bump("test")
		notes := &domain.ReleaseNotes{Text: "notes", Provider: "test", GeneratedAt: time.Now()}
		_ = run.GenerateNotes(notes, "hash", "test")
		_ = run.Approve("test", false)
		run.SetExecutionPlan([]domain.StepPlan{{Name: "tag", Type: domain.StepTypeTag}})
		_ = run.StartPublishing("test")
		return run
	case domain.StatePublished:
		_ = run.Plan("test")
		_ = run.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
		_ = run.Bump("test")
		notes := &domain.ReleaseNotes{Text: "notes", Provider: "test", GeneratedAt: time.Now()}
		_ = run.GenerateNotes(notes, "hash", "test")
		_ = run.Approve("test", false)
		run.SetExecutionPlan([]domain.StepPlan{{Name: "tag", Type: domain.StepTypeTag}})
		_ = run.StartPublishing("test")
		_ = run.MarkStepDone("tag", "done")
		_ = run.MarkPublished("test")
		return run
	case domain.StateFailed:
		_ = run.Plan("test")
		_ = run.SetVersion(version.MustParse("1.1.0"), "v1.1.0")
		_ = run.Bump("test")
		notes := &domain.ReleaseNotes{Text: "notes", Provider: "test", GeneratedAt: time.Now()}
		_ = run.GenerateNotes(notes, "hash", "test")
		_ = run.Approve("test", false)
		run.SetExecutionPlan([]domain.StepPlan{{Name: "tag", Type: domain.StepTypeTag}})
		_ = run.StartPublishing("test")
		_ = run.MarkFailed("error", "test")
		return run
	case domain.StateCanceled:
		_ = run.Plan("test")
		_ = run.Cancel("canceled", "test")
		return run
	default:
		return run
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
