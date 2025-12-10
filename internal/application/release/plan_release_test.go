// Package release provides application use cases for release management.
package release

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/felixgeelhaar/release-pilot/internal/domain/release"
	"github.com/felixgeelhaar/release-pilot/internal/domain/sourcecontrol"
	"github.com/felixgeelhaar/release-pilot/internal/domain/version"
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
	releases   map[release.ReleaseID]*release.Release
	saveErr    error
	findErr    error
	saveCalled bool
}

func newMockReleaseRepository() *mockReleaseRepository {
	return &mockReleaseRepository{
		releases: make(map[release.ReleaseID]*release.Release),
	}
}

func (m *mockReleaseRepository) Save(ctx context.Context, r *release.Release) error {
	m.saveCalled = true
	if m.saveErr != nil {
		return m.saveErr
	}
	m.releases[r.ID()] = r
	return nil
}

func (m *mockReleaseRepository) FindByID(ctx context.Context, id release.ReleaseID) (*release.Release, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	r, ok := m.releases[id]
	if !ok {
		return nil, errors.New("release not found")
	}
	return r, nil
}

func (m *mockReleaseRepository) FindByState(ctx context.Context, state release.ReleaseState) ([]*release.Release, error) {
	return nil, nil
}

func (m *mockReleaseRepository) FindLatest(ctx context.Context, repoPath string) (*release.Release, error) {
	return nil, nil
}

func (m *mockReleaseRepository) FindActive(ctx context.Context) ([]*release.Release, error) {
	return nil, nil
}

func (m *mockReleaseRepository) Delete(ctx context.Context, id release.ReleaseID) error {
	return nil
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
			name: "dry run does not save",
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
			wantSaved:      false,
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
			wantSaved:      false,
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

	uc := NewPlanReleaseUseCase(releaseRepo, gitRepo, versionCalc, eventPublisher)

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

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}
