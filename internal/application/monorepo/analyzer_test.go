package monorepo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/v4/internal/domain/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
	"github.com/relicta-tech/relicta/v4/internal/domain/workspace"
)

// mockGitRepository implements sourcecontrol.GitRepository for testing.
type mockGitRepository struct {
	commits   []*sourcecontrol.Commit
	diffStats map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats
	// filesAtRef is keyed by repository-relative path and stands for the tree at the base
	// ref, which is where a package's released version is read from.
	filesAtRef map[string][]byte
	// tags stands for the repository's tags, which is where a package's own last release is
	// found.
	tags sourcecontrol.TagList
}

// RepositoryInfoReader methods
func (m *mockGitRepository) GetInfo(ctx context.Context) (*sourcecontrol.RepositoryInfo, error) {
	return &sourcecontrol.RepositoryInfo{
		Name:          "test-repo",
		Path:          "/test",
		RemoteURL:     "https://github.com/test/repo",
		DefaultBranch: "main",
	}, nil
}

func (m *mockGitRepository) GetRemotes(ctx context.Context) ([]sourcecontrol.RemoteInfo, error) {
	return []sourcecontrol.RemoteInfo{{Name: "origin", URL: "https://github.com/test/repo"}}, nil
}

func (m *mockGitRepository) GetBranches(ctx context.Context) ([]sourcecontrol.BranchInfo, error) {
	return []sourcecontrol.BranchInfo{{Name: "main", IsCurrent: true}}, nil
}

func (m *mockGitRepository) GetCurrentBranch(ctx context.Context) (string, error) {
	return "main", nil
}

// CommitReader methods
func (m *mockGitRepository) GetCommit(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.Commit, error) {
	for _, c := range m.commits {
		if c.Hash() == hash {
			return c, nil
		}
	}
	return nil, nil
}

func (m *mockGitRepository) GetCommitsBetween(ctx context.Context, from, to string) ([]*sourcecontrol.Commit, error) {
	return m.commits, nil
}

func (m *mockGitRepository) GetCommitsSince(ctx context.Context, ref string) ([]*sourcecontrol.Commit, error) {
	return m.commits, nil
}

func (m *mockGitRepository) GetLatestCommit(ctx context.Context, branch string) (*sourcecontrol.Commit, error) {
	if len(m.commits) > 0 {
		return m.commits[0], nil
	}
	return nil, nil
}

// DiffReader methods
func (m *mockGitRepository) GetCommitDiffStats(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.DiffStats, error) {
	if stats, ok := m.diffStats[hash]; ok {
		return stats, nil
	}
	return &sourcecontrol.DiffStats{}, nil
}

func (m *mockGitRepository) GetBatchCommitDiffStats(ctx context.Context, hashes []sourcecontrol.CommitHash) (map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats, error) {
	return m.diffStats, nil
}

func (m *mockGitRepository) GetCommitPatch(ctx context.Context, hash sourcecontrol.CommitHash) (string, error) {
	return "", nil
}

func (m *mockGitRepository) GetFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	if content, ok := m.filesAtRef[path]; ok {
		return content, nil
	}
	return nil, nil
}

// TagReader methods
func (m *mockGitRepository) GetTags(ctx context.Context) (sourcecontrol.TagList, error) {
	return m.tags, nil
}

func (m *mockGitRepository) GetTag(ctx context.Context, name string) (*sourcecontrol.Tag, error) {
	return nil, nil
}

func (m *mockGitRepository) GetLatestVersionTag(ctx context.Context, prefix string) (*sourcecontrol.Tag, error) {
	return nil, nil
}

// TagWriter methods
func (m *mockGitRepository) CreateTag(ctx context.Context, name string, hash sourcecontrol.CommitHash, message string) (*sourcecontrol.Tag, error) {
	return nil, nil
}

func (m *mockGitRepository) DeleteTag(ctx context.Context, name string) error {
	return nil
}

func (m *mockGitRepository) PushTag(ctx context.Context, name string, remote string) error {
	return nil
}

// WorkingTreeInspector methods
func (m *mockGitRepository) IsDirty(ctx context.Context) (bool, error) {
	return false, nil
}

func (m *mockGitRepository) GetStatus(ctx context.Context) (*sourcecontrol.WorkingTreeStatus, error) {
	return &sourcecontrol.WorkingTreeStatus{IsClean: true}, nil
}

// RemoteOperator methods
func (m *mockGitRepository) Fetch(ctx context.Context, remote string) error {
	return nil
}

func (m *mockGitRepository) Pull(ctx context.Context, remote, branch string) error {
	return nil
}

func (m *mockGitRepository) Push(ctx context.Context, remote, branch string) error {
	return nil
}

// mockVersionReader implements VersionReader for testing.
type mockVersionReader struct {
	versions map[string]version.SemanticVersion
}

func (m *mockVersionReader) ReadVersion(ctx context.Context, pkgPath string, pkgType monorepo.PackageType) (version.SemanticVersion, error) {
	if ver, ok := m.versions[pkgPath]; ok {
		return ver, nil
	}
	return version.NewSemanticVersion(0, 1, 0), nil
}

func newTestCommit(hash, message string) *sourcecontrol.Commit {
	return sourcecontrol.NewCommit(
		sourcecontrol.CommitHash(hash),
		message,
		sourcecontrol.Author{Name: "Test User", Email: "test@example.com"},
		time.Now(),
	)
}

func TestMonorepoAnalyzer_Analyze_IndependentStrategy(t *testing.T) {
	// Setup test data
	commits := []*sourcecontrol.Commit{
		newTestCommit("abc123", "feat(pkg-a): add new feature"),
		newTestCommit("def456", "fix(pkg-b): fix bug"),
		newTestCommit("ghi789", "docs: update readme"),
	}

	diffStats := map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats{
		"abc123": {
			FilesChanged: 2,
			Additions:    50,
			Deletions:    10,
			Files: []sourcecontrol.FileStats{
				{Path: "packages/pkg-a/src/main.go", Additions: 40, Deletions: 5},
				{Path: "packages/pkg-a/src/util.go", Additions: 10, Deletions: 5},
			},
		},
		"def456": {
			FilesChanged: 1,
			Additions:    5,
			Deletions:    3,
			Files: []sourcecontrol.FileStats{
				{Path: "packages/pkg-b/index.ts", Additions: 5, Deletions: 3},
			},
		},
		"ghi789": {
			FilesChanged: 1,
			Additions:    20,
			Deletions:    5,
			Files: []sourcecontrol.FileStats{
				{Path: "README.md", Additions: 20, Deletions: 5},
			},
		},
	}

	mockRepo := &mockGitRepository{
		commits:   commits,
		diffStats: diffStats,
	}

	mockVersions := &mockVersionReader{
		versions: map[string]version.SemanticVersion{
			"packages/pkg-a": version.NewSemanticVersion(1, 0, 0),
			"packages/pkg-b": version.NewSemanticVersion(2, 1, 0),
		},
	}

	ws := &workspace.Workspace{
		ID:             "test-workspace",
		RootPath:       "/test",
		Type:           workspace.WorkspaceTypePnpm,
		PackageManager: workspace.PackageManagerPnpm,
		Packages: []*workspace.Package{
			{Name: "@test/pkg-a", Path: "packages/pkg-a", Version: "1.0.0"},
			{Name: "@test/pkg-b", Path: "packages/pkg-b", Version: "2.1.0"},
		},
	}

	analyzer := NewMonorepoAnalyzer(
		mockRepo,
		version.NewDefaultVersionCalculator(),
		nil, // No AI analysis factory for tests
		mockVersions,
	)

	ctx := context.Background()
	input := MonorepoAnalyzeInput{
		RepositoryPath: "/test",
		FromRef:        "v1.0.0",
		ToRef:          "HEAD",
		Workspace:      ws,
		Strategy:       monorepo.StrategyIndependent,
	}

	output, err := analyzer.Analyze(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, output)

	// Verify output
	assert.Equal(t, 3, output.TotalCommits)
	assert.Equal(t, 2, output.AffectedPackages)
	assert.Len(t, output.Packages, 2)

	// Check pkg-a (should have minor bump due to feat commit)
	var pkgA *PackageAnalysisResult
	for _, pkg := range output.Packages {
		if pkg.PackagePath == "packages/pkg-a" {
			pkgA = pkg
			break
		}
	}
	require.NotNil(t, pkgA)
	assert.Equal(t, monorepo.BumpTypeMinor, pkgA.BumpType)
	assert.Equal(t, uint64(1), pkgA.NextVersion.Major())
	assert.Equal(t, uint64(1), pkgA.NextVersion.Minor())
	assert.Equal(t, uint64(0), pkgA.NextVersion.Patch())
	assert.Len(t, pkgA.Commits, 1)

	// Check pkg-b (should have patch bump due to fix commit)
	var pkgB *PackageAnalysisResult
	for _, pkg := range output.Packages {
		if pkg.PackagePath == "packages/pkg-b" {
			pkgB = pkg
			break
		}
	}
	require.NotNil(t, pkgB)
	assert.Equal(t, monorepo.BumpTypePatch, pkgB.BumpType)
	assert.Equal(t, uint64(2), pkgB.NextVersion.Major())
	assert.Equal(t, uint64(1), pkgB.NextVersion.Minor())
	assert.Equal(t, uint64(1), pkgB.NextVersion.Patch())
	assert.Len(t, pkgB.Commits, 1)
}

func TestMonorepoAnalyzer_Analyze_LockstepStrategy(t *testing.T) {
	commits := []*sourcecontrol.Commit{
		newTestCommit("abc123", "feat(pkg-a): add new feature"),
		newTestCommit("def456", "fix(pkg-b): fix bug"),
	}

	diffStats := map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats{
		"abc123": {
			Files: []sourcecontrol.FileStats{
				{Path: "packages/pkg-a/src/main.go", Additions: 10, Deletions: 5},
			},
		},
		"def456": {
			Files: []sourcecontrol.FileStats{
				{Path: "packages/pkg-b/index.ts", Additions: 5, Deletions: 3},
			},
		},
	}

	mockRepo := &mockGitRepository{
		commits:   commits,
		diffStats: diffStats,
	}

	mockVersions := &mockVersionReader{
		versions: map[string]version.SemanticVersion{
			"packages/pkg-a": version.NewSemanticVersion(1, 0, 0),
			"packages/pkg-b": version.NewSemanticVersion(1, 0, 0),
		},
	}

	ws := &workspace.Workspace{
		ID:             "test-workspace",
		RootPath:       "/test",
		Type:           workspace.WorkspaceTypePnpm,
		PackageManager: workspace.PackageManagerPnpm,
		Packages: []*workspace.Package{
			{Name: "@test/pkg-a", Path: "packages/pkg-a", Version: "1.0.0"},
			{Name: "@test/pkg-b", Path: "packages/pkg-b", Version: "1.0.0"},
		},
	}

	analyzer := NewMonorepoAnalyzer(mockRepo, version.NewDefaultVersionCalculator(), nil, mockVersions)

	ctx := context.Background()
	input := MonorepoAnalyzeInput{
		RepositoryPath: "/test",
		FromRef:        "v1.0.0",
		ToRef:          "HEAD",
		Workspace:      ws,
		Strategy:       monorepo.StrategyLockstep,
	}

	output, err := analyzer.Analyze(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, output)

	// In lockstep mode, all packages should have the same version
	require.Len(t, output.Packages, 2)
	assert.Equal(t, output.Packages[0].NextVersion, output.Packages[1].NextVersion)
	// Both should be bumped to minor (highest bump type)
	assert.Equal(t, monorepo.BumpTypeMinor, output.Packages[0].BumpType)
	assert.Equal(t, monorepo.BumpTypeMinor, output.Packages[1].BumpType)
}

func TestMonorepoAnalyzer_CreateMonorepoRelease(t *testing.T) {
	commits := []*sourcecontrol.Commit{
		newTestCommit("abc123", "feat(pkg-a): add new feature"),
	}

	diffStats := map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats{
		"abc123": {
			Files: []sourcecontrol.FileStats{
				{Path: "packages/pkg-a/main.go", Additions: 10, Deletions: 5},
			},
		},
	}

	mockRepo := &mockGitRepository{
		commits:   commits,
		diffStats: diffStats,
	}

	mockVersions := &mockVersionReader{
		versions: map[string]version.SemanticVersion{
			"packages/pkg-a": version.NewSemanticVersion(1, 0, 0),
		},
	}

	ws := &workspace.Workspace{
		ID:             "test-workspace",
		RootPath:       "/test",
		Type:           workspace.WorkspaceTypePnpm,
		PackageManager: workspace.PackageManagerPnpm,
		Packages: []*workspace.Package{
			{Name: "@test/pkg-a", Path: "packages/pkg-a", Version: "1.0.0"},
		},
	}

	analyzer := NewMonorepoAnalyzer(mockRepo, version.NewDefaultVersionCalculator(), nil, mockVersions)

	ctx := context.Background()
	input := MonorepoAnalyzeInput{
		RepositoryPath: "/test",
		FromRef:        "v1.0.0",
		ToRef:          "HEAD",
		Workspace:      ws,
		Strategy:       monorepo.StrategyIndependent,
	}

	output, err := analyzer.Analyze(ctx, input)
	require.NoError(t, err)

	release, err := analyzer.CreateMonorepoRelease(ctx, input, output)
	require.NoError(t, err)
	require.NotNil(t, release)

	assert.Equal(t, monorepo.StatePlanned, release.State)
	assert.Len(t, release.Packages, 1)
	assert.Equal(t, monorepo.PackageStateIncluded, release.Packages[0].State)
}

func TestCompareBumpTypes(t *testing.T) {
	tests := []struct {
		name     string
		a, b     monorepo.BumpType
		expected int
	}{
		{"none < patch", monorepo.BumpTypeNone, monorepo.BumpTypePatch, -1},
		{"patch < minor", monorepo.BumpTypePatch, monorepo.BumpTypeMinor, -1},
		{"minor < major", monorepo.BumpTypeMinor, monorepo.BumpTypeMajor, -1},
		{"major > minor", monorepo.BumpTypeMajor, monorepo.BumpTypeMinor, 1},
		{"patch == patch", monorepo.BumpTypePatch, monorepo.BumpTypePatch, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareBumpTypes(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		a, b     version.SemanticVersion
		expected int
	}{
		{
			"1.0.0 < 2.0.0",
			version.NewSemanticVersion(1, 0, 0),
			version.NewSemanticVersion(2, 0, 0),
			-1,
		},
		{
			"1.1.0 < 1.2.0",
			version.NewSemanticVersion(1, 1, 0),
			version.NewSemanticVersion(1, 2, 0),
			-1,
		},
		{
			"1.0.1 < 1.0.2",
			version.NewSemanticVersion(1, 0, 1),
			version.NewSemanticVersion(1, 0, 2),
			-1,
		},
		{
			"1.0.0 == 1.0.0",
			version.NewSemanticVersion(1, 0, 0),
			version.NewSemanticVersion(1, 0, 0),
			0,
		},
		{
			"2.0.0 > 1.0.0",
			version.NewSemanticVersion(2, 0, 0),
			version.NewSemanticVersion(1, 0, 0),
			1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersions(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}
