// Package versioning provides application use cases for version management.
package versioning

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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

// mockVersionCalculator implements version.VersionCalculator for testing.
type mockVersionCalculator struct {
	nextVersion version.SemanticVersion
	bumpType    version.BumpType
}

func (m *mockVersionCalculator) CalculateNextVersion(current version.SemanticVersion, bumpType version.BumpType) version.SemanticVersion {
	if m.nextVersion != (version.SemanticVersion{}) {
		return m.nextVersion
	}
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
	if m.bumpType != "" {
		return m.bumpType
	}
	if hasBreaking {
		return version.BumpMajor
	}
	if hasFeature {
		return version.BumpMinor
	}
	return version.BumpPatch
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

func TestCalculateVersionUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		input          CalculateVersionInput
		gitRepo        *mockGitRepository
		versionCalc    *mockVersionCalculator
		wantErr        bool
		errMsg         string
		wantVersion    string
		wantBumpType   version.BumpType
		wantAutoDetect bool
	}{
		{
			name: "auto-detect minor bump from feature commits",
			input: CalculateVersionInput{
				Auto: true,
			},
			gitRepo: &mockGitRepository{
				commits: []*sourcecontrol.Commit{
					createTestCommit("abc123", "feat: add new feature"),
					createTestCommit("def456", "fix: bug fix"),
				},
				latestTagErr: errors.New("no tags found"),
			},
			versionCalc:    &mockVersionCalculator{},
			wantErr:        false,
			wantVersion:    "0.2.0", // version.Initial is 0.1.0, minor bump -> 0.2.0
			wantBumpType:   version.BumpMinor,
			wantAutoDetect: true,
		},
		{
			name: "auto-detect major bump from breaking change",
			input: CalculateVersionInput{
				Auto: true,
			},
			gitRepo: &mockGitRepository{
				commits: []*sourcecontrol.Commit{
					createTestCommit("abc123", "feat!: breaking change"),
				},
				latestTagErr: errors.New("no tags found"),
			},
			versionCalc:    &mockVersionCalculator{},
			wantErr:        false,
			wantVersion:    "1.0.0",
			wantBumpType:   version.BumpMajor,
			wantAutoDetect: true,
		},
		{
			name: "auto-detect patch bump from fix only",
			input: CalculateVersionInput{
				Auto: true,
			},
			gitRepo: &mockGitRepository{
				commits: []*sourcecontrol.Commit{
					createTestCommit("abc123", "fix: minor bug fix"),
				},
				latestTagErr: errors.New("no tags found"),
			},
			versionCalc:    &mockVersionCalculator{},
			wantErr:        false,
			wantVersion:    "0.1.1", // version.Initial is 0.1.0, patch bump -> 0.1.1
			wantBumpType:   version.BumpPatch,
			wantAutoDetect: true,
		},
		{
			name: "explicit major bump",
			input: CalculateVersionInput{
				Auto:     false,
				BumpType: version.BumpMajor,
			},
			gitRepo: &mockGitRepository{
				latestTagErr: errors.New("no tags found"),
			},
			versionCalc:    &mockVersionCalculator{},
			wantErr:        false,
			wantVersion:    "1.0.0",
			wantBumpType:   version.BumpMajor,
			wantAutoDetect: false,
		},
		{
			name: "explicit minor bump",
			input: CalculateVersionInput{
				Auto:     false,
				BumpType: version.BumpMinor,
			},
			gitRepo: &mockGitRepository{
				latestTagErr: errors.New("no tags found"),
			},
			versionCalc:    &mockVersionCalculator{},
			wantErr:        false,
			wantVersion:    "0.2.0", // version.Initial is 0.1.0, minor bump -> 0.2.0
			wantBumpType:   version.BumpMinor,
			wantAutoDetect: false,
		},
		{
			name: "explicit patch bump",
			input: CalculateVersionInput{
				Auto:     false,
				BumpType: version.BumpPatch,
			},
			gitRepo: &mockGitRepository{
				latestTagErr: errors.New("no tags found"),
			},
			versionCalc:    &mockVersionCalculator{},
			wantErr:        false,
			wantVersion:    "0.1.1", // version.Initial is 0.1.0, patch bump -> 0.1.1
			wantBumpType:   version.BumpPatch,
			wantAutoDetect: false,
		},
		{
			name: "no bump type and no auto",
			input: CalculateVersionInput{
				Auto:     false,
				BumpType: "",
			},
			gitRepo: &mockGitRepository{
				latestTagErr: errors.New("no tags found"),
			},
			versionCalc: &mockVersionCalculator{},
			wantErr:     true,
			errMsg:      "bump type must be specified",
		},
		{
			name: "commits fetch error",
			input: CalculateVersionInput{
				Auto: true,
			},
			gitRepo: &mockGitRepository{
				commitsErr:   errors.New("fetch error"),
				latestTagErr: errors.New("no tags found"),
			},
			versionCalc: &mockVersionCalculator{},
			wantErr:     true,
			errMsg:      "failed to get commits",
		},
		{
			name: "custom tag prefix",
			input: CalculateVersionInput{
				Auto:      false,
				BumpType:  version.BumpPatch,
				TagPrefix: "release-",
			},
			gitRepo: &mockGitRepository{
				latestTagErr: errors.New("no tags found"),
			},
			versionCalc:    &mockVersionCalculator{},
			wantErr:        false,
			wantVersion:    "0.1.1", // version.Initial is 0.1.0, patch bump -> 0.1.1
			wantBumpType:   version.BumpPatch,
			wantAutoDetect: false,
		},
		{
			name: "with prerelease identifier",
			input: CalculateVersionInput{
				Auto:       false,
				BumpType:   version.BumpMinor,
				Prerelease: "alpha.1",
			},
			gitRepo: &mockGitRepository{
				latestTagErr: errors.New("no tags found"),
			},
			versionCalc:    &mockVersionCalculator{},
			wantErr:        false,
			wantVersion:    "0.2.0-alpha.1", // version.Initial is 0.1.0, minor bump -> 0.2.0
			wantBumpType:   version.BumpMinor,
			wantAutoDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := NewCalculateVersionUseCase(tt.gitRepo, tt.versionCalc)

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

			if output.BumpType != tt.wantBumpType {
				t.Errorf("BumpType = %s, want %s", output.BumpType, tt.wantBumpType)
			}

			if output.AutoDetected != tt.wantAutoDetect {
				t.Errorf("AutoDetected = %v, want %v", output.AutoDetected, tt.wantAutoDetect)
			}
		})
	}
}

func TestSetVersionUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		input       SetVersionInput
		gitRepo     *mockGitRepository
		wantErr     bool
		errMsg      string
		wantTagName string
		wantCreated bool
		wantPushed  bool
	}{
		{
			name: "dry run returns without creating tag",
			input: SetVersionInput{
				Version:   version.MustParse("1.2.3"),
				CreateTag: true,
				DryRun:    true,
			},
			gitRepo:     &mockGitRepository{},
			wantErr:     false,
			wantTagName: "v1.2.3",
			wantCreated: false,
			wantPushed:  false,
		},
		{
			name: "create tag without push",
			input: SetVersionInput{
				Version:   version.MustParse("1.2.3"),
				CreateTag: true,
				PushTag:   false,
				DryRun:    false,
			},
			gitRepo: &mockGitRepository{
				info: &sourcecontrol.RepositoryInfo{
					CurrentBranch: "main",
				},
				latestCommit: createTestCommit("abc123", "latest commit"),
				tagCreated:   &sourcecontrol.Tag{},
			},
			wantErr:     false,
			wantTagName: "v1.2.3",
			wantCreated: true,
			wantPushed:  false,
		},
		{
			name: "create and push tag",
			input: SetVersionInput{
				Version:   version.MustParse("2.0.0"),
				CreateTag: true,
				PushTag:   true,
				DryRun:    false,
			},
			gitRepo: &mockGitRepository{
				info: &sourcecontrol.RepositoryInfo{
					CurrentBranch: "main",
				},
				latestCommit: createTestCommit("abc123", "latest commit"),
				tagCreated:   &sourcecontrol.Tag{},
			},
			wantErr:     false,
			wantTagName: "v2.0.0",
			wantCreated: true,
			wantPushed:  true,
		},
		{
			name: "custom tag prefix",
			input: SetVersionInput{
				Version:   version.MustParse("1.0.0"),
				TagPrefix: "release-",
				CreateTag: true,
				DryRun:    false,
			},
			gitRepo: &mockGitRepository{
				info: &sourcecontrol.RepositoryInfo{
					CurrentBranch: "main",
				},
				latestCommit: createTestCommit("abc123", "latest commit"),
				tagCreated:   &sourcecontrol.Tag{},
			},
			wantErr:     false,
			wantTagName: "release-1.0.0",
			wantCreated: true,
			wantPushed:  false,
		},
		{
			name: "repo info error",
			input: SetVersionInput{
				Version:   version.MustParse("1.0.0"),
				CreateTag: true,
				DryRun:    false,
			},
			gitRepo: &mockGitRepository{
				infoErr: errors.New("repo info error"),
			},
			wantErr: true,
			errMsg:  "repo info",
		},
		{
			name: "latest commit error",
			input: SetVersionInput{
				Version:   version.MustParse("1.0.0"),
				CreateTag: true,
				DryRun:    false,
			},
			gitRepo: &mockGitRepository{
				info: &sourcecontrol.RepositoryInfo{
					CurrentBranch: "main",
				},
				latestCommitErr: errors.New("commit error"),
			},
			wantErr: true,
			errMsg:  "latest commit",
		},
		{
			name: "tag create error",
			input: SetVersionInput{
				Version:   version.MustParse("1.0.0"),
				CreateTag: true,
				DryRun:    false,
			},
			gitRepo: &mockGitRepository{
				info: &sourcecontrol.RepositoryInfo{
					CurrentBranch: "main",
				},
				latestCommit: createTestCommit("abc123", "latest commit"),
				tagCreateErr: errors.New("create tag error"),
			},
			wantErr: true,
			errMsg:  "create tag",
		},
		{
			name: "push tag error",
			input: SetVersionInput{
				Version:   version.MustParse("1.0.0"),
				CreateTag: true,
				PushTag:   true,
				DryRun:    false,
			},
			gitRepo: &mockGitRepository{
				info: &sourcecontrol.RepositoryInfo{
					CurrentBranch: "main",
				},
				latestCommit: createTestCommit("abc123", "latest commit"),
				tagCreated:   &sourcecontrol.Tag{},
				pushTagErr:   errors.New("push error"),
			},
			wantErr: true,
			errMsg:  "push tag",
		},
		{
			name: "no tag creation",
			input: SetVersionInput{
				Version:   version.MustParse("1.0.0"),
				CreateTag: false,
				DryRun:    false,
			},
			gitRepo:     &mockGitRepository{},
			wantErr:     false,
			wantTagName: "v1.0.0",
			wantCreated: false,
			wantPushed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := NewSetVersionUseCase(tt.gitRepo)

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

			if output.TagName != tt.wantTagName {
				t.Errorf("TagName = %s, want %s", output.TagName, tt.wantTagName)
			}

			if output.TagCreated != tt.wantCreated {
				t.Errorf("TagCreated = %v, want %v", output.TagCreated, tt.wantCreated)
			}

			if output.TagPushed != tt.wantPushed {
				t.Errorf("TagPushed = %v, want %v", output.TagPushed, tt.wantPushed)
			}
		})
	}
}

func TestNewCalculateVersionUseCase(t *testing.T) {
	gitRepo := &mockGitRepository{}
	versionCalc := &mockVersionCalculator{}

	uc := NewCalculateVersionUseCase(gitRepo, versionCalc)

	if uc == nil {
		t.Fatal("expected non-nil use case")
	}
	if uc.gitRepo == nil {
		t.Error("gitRepo should not be nil")
	}
	if uc.versionCalc == nil {
		t.Error("versionCalc should not be nil")
	}
	if uc.logger == nil {
		t.Error("logger should not be nil")
	}
}

func TestNewSetVersionUseCase(t *testing.T) {
	gitRepo := &mockGitRepository{}

	uc := NewSetVersionUseCase(gitRepo)

	if uc == nil {
		t.Fatal("expected non-nil use case")
	}
	if uc.gitRepo == nil {
		t.Error("gitRepo should not be nil")
	}
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}
