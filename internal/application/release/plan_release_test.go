// Package release provides application use cases for release management.
package release

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/analysis"
	analysisfactory "github.com/relicta-tech/relicta/internal/analysis/factory"
	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// mockGitRepository implements sourcecontrol.GitRepository for testing.
type mockGitRepository struct {
	info             *sourcecontrol.RepositoryInfo
	infoErr          error
	commits          []*sourcecontrol.Commit
	commitsErr       error
	latestVersionTag *sourcecontrol.Tag
	latestTagErr     error
	tagCreated       *sourcecontrol.Tag
	tagCreateErr     error
	latestCommit     *sourcecontrol.Commit
	latestCommitErr  error
	pushTagErr       error
	diffStats        map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats
	patches          map[sourcecontrol.CommitHash]string
	filesAtRef       map[string]map[string][]byte
}

func (m *mockGitRepository) GetInfo(ctx context.Context) (*sourcecontrol.RepositoryInfo, error) {
	return m.info, m.infoErr
}

func (m *mockGitRepository) GetRemotes(ctx context.Context) ([]sourcecontrol.RemoteInfo, error) {
	return nil, nil
}

func (m *mockGitRepository) GetBranches(ctx context.Context) ([]sourcecontrol.BranchInfo, error) {
	return nil, nil
}

func (m *mockGitRepository) GetCurrentBranch(ctx context.Context) (string, error) {
	if m.info != nil {
		return m.info.CurrentBranch, nil
	}
	return "", nil
}

func (m *mockGitRepository) GetCommit(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.Commit, error) {
	return nil, nil
}

func (m *mockGitRepository) GetCommitsBetween(ctx context.Context, from, to string) ([]*sourcecontrol.Commit, error) {
	return m.commits, m.commitsErr
}

func (m *mockGitRepository) GetCommitsSince(ctx context.Context, ref string) ([]*sourcecontrol.Commit, error) {
	return m.commits, m.commitsErr
}

func (m *mockGitRepository) GetLatestCommit(ctx context.Context, branch string) (*sourcecontrol.Commit, error) {
	return m.latestCommit, m.latestCommitErr
}

func (m *mockGitRepository) GetCommitDiffStats(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.DiffStats, error) {
	if m.diffStats != nil {
		if stats, ok := m.diffStats[hash]; ok {
			return stats, nil
		}
	}
	return nil, nil
}

func (m *mockGitRepository) GetCommitPatch(ctx context.Context, hash sourcecontrol.CommitHash) (string, error) {
	if m.patches != nil {
		if patch, ok := m.patches[hash]; ok {
			return patch, nil
		}
	}
	return "", nil
}

func (m *mockGitRepository) GetFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	if m.filesAtRef != nil {
		if files, ok := m.filesAtRef[ref]; ok {
			if data, ok := files[path]; ok {
				return data, nil
			}
		}
	}
	return nil, nil
}

func (m *mockGitRepository) GetTags(ctx context.Context) (sourcecontrol.TagList, error) {
	return nil, nil
}

func (m *mockGitRepository) GetTag(ctx context.Context, name string) (*sourcecontrol.Tag, error) {
	return nil, nil
}

func (m *mockGitRepository) GetLatestVersionTag(ctx context.Context, prefix string) (*sourcecontrol.Tag, error) {
	return m.latestVersionTag, m.latestTagErr
}

func (m *mockGitRepository) CreateTag(ctx context.Context, name string, hash sourcecontrol.CommitHash, message string) (*sourcecontrol.Tag, error) {
	return m.tagCreated, m.tagCreateErr
}

func (m *mockGitRepository) DeleteTag(ctx context.Context, name string) error {
	return nil
}

func (m *mockGitRepository) PushTag(ctx context.Context, name string, remote string) error {
	return m.pushTagErr
}

func (m *mockGitRepository) IsDirty(ctx context.Context) (bool, error) {
	if m.info != nil {
		return m.info.IsDirty, nil
	}
	return false, nil
}

func (m *mockGitRepository) GetStatus(ctx context.Context) (*sourcecontrol.WorkingTreeStatus, error) {
	return &sourcecontrol.WorkingTreeStatus{IsClean: true}, nil
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

// mockReleaseRepository implements release.Repository for testing.
type mockReleaseRepository struct {
	releases   map[release.RunID]*release.ReleaseRun
	saveErr    error
	findErr    error
	saveCalled bool
}

func newMockReleaseRepository() *mockReleaseRepository {
	return &mockReleaseRepository{
		releases: make(map[release.RunID]*release.ReleaseRun),
	}
}

func (m *mockReleaseRepository) Save(ctx context.Context, r *release.ReleaseRun) error {
	m.saveCalled = true
	if m.saveErr != nil {
		return m.saveErr
	}
	m.releases[r.ID()] = r
	return nil
}

func (m *mockReleaseRepository) FindByID(ctx context.Context, id release.RunID) (*release.ReleaseRun, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	r, ok := m.releases[id]
	if !ok {
		return nil, errors.New("release not found")
	}
	return r, nil
}

func (m *mockReleaseRepository) FindByState(ctx context.Context, state release.RunState) ([]*release.ReleaseRun, error) {
	return nil, nil
}

func (m *mockReleaseRepository) FindLatest(ctx context.Context, repoPath string) (*release.ReleaseRun, error) {
	return nil, nil
}

func (m *mockReleaseRepository) FindActive(ctx context.Context) ([]*release.ReleaseRun, error) {
	return nil, nil
}

func (m *mockReleaseRepository) Delete(ctx context.Context, id release.RunID) error {
	return nil
}

func (m *mockReleaseRepository) FindBySpecification(ctx context.Context, spec release.Specification) ([]*release.ReleaseRun, error) {
	result := make([]*release.ReleaseRun, 0)
	for _, r := range m.releases {
		if spec.IsSatisfiedBy(r) {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *mockReleaseRepository) List(ctx context.Context, repoPath string) ([]release.RunID, error) {
	return nil, nil
}

// mockVersionCalculator implements version.VersionCalculator for testing.
type mockVersionCalculator struct{}

func (m *mockVersionCalculator) CalculateNextVersion(current version.SemanticVersion, bumpType version.BumpType) version.SemanticVersion {
	switch bumpType {
	case version.BumpMajor:
		return version.BumpMajorVersion(current)
	case version.BumpMinor:
		return version.BumpMinorVersion(current)
	case version.BumpPatch:
		return version.BumpPatchVersion(current)
	default:
		return version.BumpPatchVersion(current)
	}
}

func (m *mockVersionCalculator) DetermineRequiredBump(hasBreaking, hasFeature, hasFix bool) version.BumpType {
	if hasBreaking {
		return version.BumpMajor
	}
	if hasFeature {
		return version.BumpMinor
	}
	return version.BumpPatch
}

// mockEventPublisher implements release.EventPublisher for testing.
type mockEventPublisher struct {
	published  []release.DomainEvent
	publishErr error
}

func (m *mockEventPublisher) Publish(ctx context.Context, events ...release.DomainEvent) error {
	if m.publishErr != nil {
		return m.publishErr
	}
	m.published = append(m.published, events...)
	return nil
}

type mockUnitOfWork struct {
	repo           *mockReleaseRepository
	commitErr      error
	commitCalled   bool
	rollbackCalled bool
}

func (u *mockUnitOfWork) Commit(ctx context.Context) error {
	u.commitCalled = true
	if u.commitErr != nil {
		return u.commitErr
	}
	return nil
}

func (u *mockUnitOfWork) Rollback() error {
	u.rollbackCalled = true
	return nil
}

func (u *mockUnitOfWork) ReleaseRepository() release.Repository {
	return u.repo
}

type mockUnitOfWorkFactory struct {
	beginErr error
	uow      *mockUnitOfWork
}

func (f *mockUnitOfWorkFactory) Begin(ctx context.Context) (release.UnitOfWork, error) {
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	if f.uow == nil {
		f.uow = &mockUnitOfWork{
			repo: newMockReleaseRepository(),
		}
	}
	return f.uow, nil
}

// createTestCommit creates a commit for testing.
func createTestCommit(hash, message string) *sourcecontrol.Commit {
	return sourcecontrol.NewCommit(
		sourcecontrol.CommitHash(hash),
		message,
		sourcecontrol.Author{Name: "Test Author", Email: "test@example.com"},
		time.Now(),
	)
}

func TestPlanReleaseInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   PlanReleaseInput
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid empty input",
			input:   PlanReleaseInput{},
			wantErr: false,
		},
		{
			name: "valid with all fields",
			input: PlanReleaseInput{
				RepositoryPath: "/path/to/repo",
				Branch:         "main",
				FromRef:        "v1.0.0",
				ToRef:          "HEAD",
				TagPrefix:      "v",
			},
			wantErr: false,
		},
		{
			name: "path traversal attempt - relative",
			input: PlanReleaseInput{
				RepositoryPath: "../../../etc/passwd",
			},
			wantErr: true,
			errMsg:  "invalid traversal",
		},
		{
			name: "invalid branch with special chars",
			input: PlanReleaseInput{
				Branch: "branch~1",
			},
			wantErr: true,
			errMsg:  "invalid branch name",
		},
		{
			name: "branch starting with slash",
			input: PlanReleaseInput{
				Branch: "/feature",
			},
			wantErr: true,
			errMsg:  "cannot start or end with /",
		},
		{
			name: "branch ending with slash",
			input: PlanReleaseInput{
				Branch: "feature/",
			},
			wantErr: true,
			errMsg:  "cannot start or end with /",
		},
		{
			name: "branch with double dot",
			input: PlanReleaseInput{
				Branch: "feature..branch",
			},
			wantErr: true,
			errMsg:  "cannot contain '..'",
		},
		{
			name: "tag prefix too long",
			input: PlanReleaseInput{
				TagPrefix: "this-is-a-very-long-tag-prefix-that-exceeds-the-limit",
			},
			wantErr: true,
			errMsg:  "too long",
		},
		{
			name: "tag prefix with invalid chars",
			input: PlanReleaseInput{
				TagPrefix: "v*",
			},
			wantErr: true,
			errMsg:  "invalid characters",
		},
		{
			name: "invalid ToRef",
			input: PlanReleaseInput{
				ToRef: "ref*with*asterisks",
			},
			wantErr: true,
			errMsg:  "invalid to reference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestPlanReleaseUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		input          PlanReleaseInput
		gitRepo        *mockGitRepository
		releaseRepo    *mockReleaseRepository
		versionCalc    *mockVersionCalculator
		eventPublisher *mockEventPublisher
		wantErr        bool
		errMsg         string
		wantVersion    string
		wantSaved      bool
	}{
		{
			name: "successful plan with feature commits",
			input: PlanReleaseInput{
				DryRun: false,
			},
			gitRepo: &mockGitRepository{
				info: &sourcecontrol.RepositoryInfo{
					Name:          "test-repo",
					CurrentBranch: "main",
					IsDirty:       false,
				},
				commits: []*sourcecontrol.Commit{
					createTestCommit("abc123", "feat: add new feature"),
					createTestCommit("def456", "fix: bug fix"),
				},
				latestTagErr: errors.New("no tags found"),
			},
			releaseRepo:    newMockReleaseRepository(),
			versionCalc:    &mockVersionCalculator{},
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantVersion:    "0.2.0", // version.Initial is 0.1.0, minor bump -> 0.2.0
			wantSaved:      true,
		},
		{
			name: "successful plan with breaking change",
			input: PlanReleaseInput{
				DryRun: false,
			},
			gitRepo: &mockGitRepository{
				info: &sourcecontrol.RepositoryInfo{
					Name:          "test-repo",
					CurrentBranch: "main",
					IsDirty:       false,
				},
				commits: []*sourcecontrol.Commit{
					createTestCommit("abc123", "feat!: breaking change"),
				},
				latestTagErr: errors.New("no tags found"),
			},
			releaseRepo:    newMockReleaseRepository(),
			versionCalc:    &mockVersionCalculator{},
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantVersion:    "1.0.0",
			wantSaved:      true,
		},
		{
			name: "dry run still saves for workflow tracking",
			input: PlanReleaseInput{
				DryRun: true,
			},
			gitRepo: &mockGitRepository{
				info: &sourcecontrol.RepositoryInfo{
					Name:          "test-repo",
					CurrentBranch: "main",
					IsDirty:       false,
				},
				commits: []*sourcecontrol.Commit{
					createTestCommit("abc123", "fix: minor fix"),
				},
				latestTagErr: errors.New("no tags found"),
			},
			releaseRepo:    newMockReleaseRepository(),
			versionCalc:    &mockVersionCalculator{},
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantVersion:    "0.1.1", // version.Initial is 0.1.0, patch bump -> 0.1.1
			wantSaved:      true,    // Always save for workflow state tracking
		},
		{
			name: "dirty working tree blocks release",
			input: PlanReleaseInput{
				DryRun: false,
			},
			gitRepo: &mockGitRepository{
				info: &sourcecontrol.RepositoryInfo{
					Name:          "test-repo",
					CurrentBranch: "main",
					IsDirty:       true,
				},
				commits: []*sourcecontrol.Commit{
					createTestCommit("abc123", "feat: new feature"),
				},
				latestTagErr: errors.New("no tags found"),
			},
			releaseRepo:    newMockReleaseRepository(),
			versionCalc:    &mockVersionCalculator{},
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "uncommitted", // actual error: "working tree has uncommitted changes"
			wantSaved:      false,
		},
		{
			name: "dirty working tree allowed in dry run",
			input: PlanReleaseInput{
				DryRun: true,
			},
			gitRepo: &mockGitRepository{
				info: &sourcecontrol.RepositoryInfo{
					Name:          "test-repo",
					CurrentBranch: "main",
					IsDirty:       true,
				},
				commits: []*sourcecontrol.Commit{
					createTestCommit("abc123", "feat: new feature"),
				},
				latestTagErr: errors.New("no tags found"),
			},
			releaseRepo:    newMockReleaseRepository(),
			versionCalc:    &mockVersionCalculator{},
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantVersion:    "0.2.0", // version.Initial is 0.1.0, minor bump -> 0.2.0
			wantSaved:      true,    // Always save for workflow state tracking
		},
		{
			name: "no commits found",
			input: PlanReleaseInput{
				DryRun: false,
			},
			gitRepo: &mockGitRepository{
				info: &sourcecontrol.RepositoryInfo{
					Name:          "test-repo",
					CurrentBranch: "main",
					IsDirty:       false,
				},
				commits:      []*sourcecontrol.Commit{},
				latestTagErr: errors.New("no tags found"),
			},
			releaseRepo:    newMockReleaseRepository(),
			versionCalc:    &mockVersionCalculator{},
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "no commits",
			wantSaved:      false,
		},
		{
			name: "git repo error",
			input: PlanReleaseInput{
				DryRun: false,
			},
			gitRepo: &mockGitRepository{
				infoErr: errors.New("git error"),
			},
			releaseRepo:    newMockReleaseRepository(),
			versionCalc:    &mockVersionCalculator{},
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "repository info",
			wantSaved:      false,
		},
		{
			name: "commits fetch error",
			input: PlanReleaseInput{
				DryRun: false,
			},
			gitRepo: &mockGitRepository{
				info: &sourcecontrol.RepositoryInfo{
					Name:          "test-repo",
					CurrentBranch: "main",
					IsDirty:       false,
				},
				commitsErr:   errors.New("fetch error"),
				latestTagErr: errors.New("no tags found"),
			},
			releaseRepo:    newMockReleaseRepository(),
			versionCalc:    &mockVersionCalculator{},
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "failed to get commits",
			wantSaved:      false,
		},
		{
			name: "invalid input - path traversal",
			input: PlanReleaseInput{
				RepositoryPath: "../../../etc",
			},
			gitRepo: &mockGitRepository{
				info: &sourcecontrol.RepositoryInfo{
					Name:          "test-repo",
					CurrentBranch: "main",
				},
			},
			releaseRepo:    newMockReleaseRepository(),
			versionCalc:    &mockVersionCalculator{},
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "invalid input",
			wantSaved:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := NewPlanReleaseUseCase(
				tt.releaseRepo,
				tt.gitRepo,
				tt.versionCalc,
				tt.eventPublisher,
				nil,
			)

			output, err := uc.Execute(ctx, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if output == nil {
				t.Error("expected output, got nil")
				return
			}

			if tt.wantVersion != "" && output.NextVersion.String() != tt.wantVersion {
				t.Errorf("NextVersion = %s, want %s", output.NextVersion.String(), tt.wantVersion)
			}

			if tt.wantSaved != tt.releaseRepo.saveCalled {
				t.Errorf("save called = %v, want %v", tt.releaseRepo.saveCalled, tt.wantSaved)
			}
		})
	}
}

func TestNewPlanReleaseUseCase(t *testing.T) {
	releaseRepo := newMockReleaseRepository()
	gitRepo := &mockGitRepository{}
	versionCalc := &mockVersionCalculator{}
	eventPublisher := &mockEventPublisher{}

	uc := NewPlanReleaseUseCase(releaseRepo, gitRepo, versionCalc, eventPublisher, nil)

	if uc == nil {
		t.Fatal("expected non-nil use case")
	}
	if uc.releaseRepo == nil {
		t.Error("releaseRepo should not be nil")
	}
	if uc.gitRepo == nil {
		t.Error("gitRepo should not be nil")
	}
	if uc.versionCalc == nil {
		t.Error("versionCalc should not be nil")
	}
	if uc.eventPublisher == nil {
		t.Error("eventPublisher should not be nil")
	}
	if uc.logger == nil {
		t.Error("logger should not be nil")
	}
}

func TestPlanReleaseUseCase_AnalyzeCommits(t *testing.T) {
	ctx := context.Background()
	commit := createTestCommit("abc123", "update docs")

	gitRepo := &mockGitRepository{
		info: &sourcecontrol.RepositoryInfo{
			Name:          "test-repo",
			CurrentBranch: "main",
			IsDirty:       false,
		},
		commits:      []*sourcecontrol.Commit{commit},
		latestTagErr: errors.New("no tags found"),
		diffStats: map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats{
			commit.Hash(): {
				Additions:    2,
				Deletions:    1,
				FilesChanged: 1,
				Files: []sourcecontrol.FileStats{
					{Path: "README.md", Additions: 2, Deletions: 1},
				},
			},
		},
	}

	analysisFactory := analysisfactory.NewFactory(nil)
	uc := NewPlanReleaseUseCase(newMockReleaseRepository(), gitRepo, &mockVersionCalculator{}, &mockEventPublisher{}, analysisFactory)

	result, infos, err := uc.AnalyzeCommits(ctx, PlanReleaseInput{})
	if err != nil {
		t.Fatalf("AnalyzeCommits error: %v", err)
	}
	if result == nil {
		t.Fatal("expected analysis result")
	}
	if len(infos) != 1 {
		t.Fatalf("infos length = %d, want 1", len(infos))
	}
	if infos[0].Stats.FilesChanged != 1 {
		t.Errorf("FilesChanged = %d, want 1", infos[0].Stats.FilesChanged)
	}
	if result.Stats.TotalCommits != 1 {
		t.Errorf("TotalCommits = %d, want 1", result.Stats.TotalCommits)
	}
}

func TestPlanReleaseUseCase_AnalyzeCommits_NoFactory(t *testing.T) {
	ctx := context.Background()
	gitRepo := &mockGitRepository{
		info: &sourcecontrol.RepositoryInfo{
			Name:          "test-repo",
			CurrentBranch: "main",
		},
		commits: []*sourcecontrol.Commit{
			createTestCommit("abc123", "update docs"),
		},
		latestTagErr: errors.New("no tags found"),
	}

	uc := NewPlanReleaseUseCase(newMockReleaseRepository(), gitRepo, &mockVersionCalculator{}, &mockEventPublisher{}, nil)
	_, _, err := uc.AnalyzeCommits(ctx, PlanReleaseInput{})
	if err == nil {
		t.Fatal("expected error when analysis factory is nil")
	}
}

func TestClassificationToCommit_BreakingScope(t *testing.T) {
	commit := createTestCommit("abc123", "update stuff")
	classification := &analysis.CommitClassification{
		CommitHash:     commit.Hash(),
		Type:           changes.CommitTypeFeat,
		Scope:          "api",
		IsBreaking:     true,
		BreakingReason: "api removed",
	}

	result := classificationToCommit(commit, classification)
	if result.Scope() != "api" {
		t.Errorf("Scope = %q, want %q", result.Scope(), "api")
	}
	if !result.IsBreaking() {
		t.Error("IsBreaking = false, want true")
	}
	if result.BreakingMessage() != "api removed" {
		t.Errorf("BreakingMessage = %q, want %q", result.BreakingMessage(), "api removed")
	}
}

func TestPlanReleaseUseCase_buildCommitInfos_AST(t *testing.T) {
	commit := createTestCommit("abc123", "update api")
	commit.SetParents([]sourcecontrol.CommitHash{"parent123"})

	gitRepo := &mockGitRepository{
		diffStats: map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats{
			commit.Hash(): {
				Additions:    10,
				Deletions:    2,
				FilesChanged: 1,
				Files: []sourcecontrol.FileStats{
					{Path: "internal/api.go", Additions: 10, Deletions: 2},
				},
			},
		},
		filesAtRef: map[string]map[string][]byte{
			"parent123": {
				"internal/api.go": []byte("package api\nfunc Old() {}\n"),
			},
			commit.Hash().String(): {
				"internal/api.go": []byte("package api\nfunc New() {}\n"),
			},
		},
	}

	uc := NewPlanReleaseUseCase(newMockReleaseRepository(), gitRepo, &mockVersionCalculator{}, &mockEventPublisher{}, analysisfactory.NewFactory(nil))
	cfg := analysis.DefaultConfig()
	cfg.EnableAST = true

	infos, err := uc.buildCommitInfos(context.Background(), []*sourcecontrol.Commit{commit}, cfg)
	if err != nil {
		t.Fatalf("buildCommitInfos error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("infos length = %d, want 1", len(infos))
	}
	if len(infos[0].FileDiffs) != 1 {
		t.Fatalf("FileDiffs length = %d, want 1", len(infos[0].FileDiffs))
	}
}

func TestPlanReleaseUseCase_NewWithUoW(t *testing.T) {
	factory := &mockUnitOfWorkFactory{}
	uc := NewPlanReleaseUseCaseWithUoW(factory, &mockGitRepository{}, &mockVersionCalculator{}, &mockEventPublisher{}, nil)
	if uc == nil {
		t.Fatal("expected non-nil use case")
	}
	if uc.unitOfWorkFactory == nil {
		t.Fatal("expected unitOfWorkFactory to be set")
	}
	if uc.releaseRepo != nil {
		t.Fatal("releaseRepo should be nil for UoW constructor")
	}
}

func TestPlanReleaseUseCase_SaveReleaseWithUoW(t *testing.T) {
	ctx := context.Background()
	gitRepo := &mockGitRepository{
		info: &sourcecontrol.RepositoryInfo{
			Name:          "test-repo",
			CurrentBranch: "main",
			IsDirty:       false,
		},
		commits: []*sourcecontrol.Commit{
			createTestCommit("abc123", "feat: add feature"),
		},
		latestTagErr: errors.New("no tags"),
	}
	factory := &mockUnitOfWorkFactory{
		uow: &mockUnitOfWork{
			repo: newMockReleaseRepository(),
		},
	}

	uc := NewPlanReleaseUseCaseWithUoW(factory, gitRepo, &mockVersionCalculator{}, &mockEventPublisher{}, nil)
	_, err := uc.Execute(ctx, PlanReleaseInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if factory.uow == nil {
		t.Fatal("expected unit of work")
	}
	if !factory.uow.commitCalled {
		t.Error("expected commit to be called")
	}
	if !factory.uow.repo.saveCalled {
		t.Error("expected release to be saved via UoW")
	}
}

func TestPlanReleaseUseCase_PrepareCommitClassifications_Overrides(t *testing.T) {
	ctx := context.Background()
	commit := createTestCommit("abc123", "fix: bug")
	uc := NewPlanReleaseUseCase(newMockReleaseRepository(), &mockGitRepository{}, &mockVersionCalculator{}, &mockEventPublisher{}, nil)
	overrides := map[sourcecontrol.CommitHash]*analysis.CommitClassification{
		commit.Hash(): {
			CommitHash: commit.Hash(),
			Type:       changes.CommitTypeFix,
		},
	}

	result, classifications, err := uc.prepareCommitClassifications(ctx, []*sourcecontrol.Commit{commit}, PlanReleaseInput{
		CommitClassifications: overrides,
	})
	if err != nil {
		t.Fatalf("prepareCommitClassifications error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	if got := classifications[commit.Hash()]; got == nil || got.Type != changes.CommitTypeFix {
		t.Fatalf("unexpected classification: %+v", got)
	}
}

func TestPlanReleaseUseCase_PrepareCommitClassifications_WithAnalysis(t *testing.T) {
	ctx := context.Background()
	commit := createTestCommit("abc123", "fix: bug fix")
	gitRepo := &mockGitRepository{
		commits: []*sourcecontrol.Commit{commit},
		diffStats: map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats{
			commit.Hash(): {
				Additions:    1,
				Deletions:    0,
				FilesChanged: 1,
				Files: []sourcecontrol.FileStats{
					{Path: "README.md", Additions: 1, Deletions: 0},
				},
			},
		},
		latestTagErr: errors.New("no tags"),
	}
	analysisFactory := analysisfactory.NewFactory(nil)
	uc := NewPlanReleaseUseCase(newMockReleaseRepository(), gitRepo, &mockVersionCalculator{}, &mockEventPublisher{}, analysisFactory)

	result, classifications, err := uc.prepareCommitClassifications(ctx, []*sourcecontrol.Commit{commit}, PlanReleaseInput{})
	if err != nil {
		t.Fatalf("prepareCommitClassifications error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result to be non-nil")
	}
	if len(classifications) == 0 {
		t.Fatal("expected at least one classification")
	}
}

func TestShouldAnalyzeAST(t *testing.T) {
	cfg := analysis.DefaultConfig()
	cfg.EnableAST = true
	if !shouldAnalyzeAST("internal/service/api.go", cfg) {
		t.Error("expected .go file to be analyzed")
	}
	if shouldAnalyzeAST("internal/service/api_test.go", cfg) {
		t.Error("expected _test.go file to be skipped")
	}
	if shouldAnalyzeAST("docs/README.md", cfg) {
		t.Error("expected non-go file to be skipped")
	}
}

func TestComputeConfigHash(t *testing.T) {
	t.Run("same config produces same hash", func(t *testing.T) {
		input := PlanReleaseInput{
			TagPrefix: "v",
			AnalysisConfig: &analysis.AnalyzerConfig{
				MinConfidence: 0.75,
				EnableAI:      true,
				EnableAST:     false,
				Languages:     []string{"go", "python"},
				SkipPaths:     []string{"vendor/**", "testdata/**"},
			},
		}
		hash1 := computeConfigHash(input)
		hash2 := computeConfigHash(input)
		if hash1 != hash2 {
			t.Errorf("same config should produce same hash, got %s and %s", hash1, hash2)
		}
	})

	t.Run("different tag prefix produces different hash", func(t *testing.T) {
		input1 := PlanReleaseInput{TagPrefix: "v"}
		input2 := PlanReleaseInput{TagPrefix: "release-"}
		hash1 := computeConfigHash(input1)
		hash2 := computeConfigHash(input2)
		if hash1 == hash2 {
			t.Error("different tag prefixes should produce different hashes")
		}
	})

	t.Run("different analysis config produces different hash", func(t *testing.T) {
		input1 := PlanReleaseInput{
			TagPrefix:      "v",
			AnalysisConfig: &analysis.AnalyzerConfig{EnableAI: true},
		}
		input2 := PlanReleaseInput{
			TagPrefix:      "v",
			AnalysisConfig: &analysis.AnalyzerConfig{EnableAI: false},
		}
		hash1 := computeConfigHash(input1)
		hash2 := computeConfigHash(input2)
		if hash1 == hash2 {
			t.Error("different analysis configs should produce different hashes")
		}
	})

	t.Run("nil analysis config produces deterministic hash", func(t *testing.T) {
		input := PlanReleaseInput{TagPrefix: "v"}
		hash1 := computeConfigHash(input)
		hash2 := computeConfigHash(input)
		if hash1 != hash2 {
			t.Errorf("nil analysis config should produce deterministic hash, got %s and %s", hash1, hash2)
		}
	})

	t.Run("hash is 16 characters", func(t *testing.T) {
		input := PlanReleaseInput{TagPrefix: "v"}
		hash := computeConfigHash(input)
		if len(hash) != 16 {
			t.Errorf("expected hash length 16, got %d", len(hash))
		}
	})
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}
