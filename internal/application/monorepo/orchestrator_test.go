package monorepo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/v4/internal/domain/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
	"github.com/relicta-tech/relicta/v4/internal/domain/workspace"
)

func TestOrchestrator_Plan(t *testing.T) {
	commits := []*sourcecontrol.Commit{
		newTestCommit("abc123", "feat(core): add new feature"),
		newTestCommit("def456", "fix(utils): fix bug"),
	}

	diffStats := map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats{
		"abc123": {
			Files: []sourcecontrol.FileStats{
				{Path: "packages/core/main.go", Additions: 20, Deletions: 5},
			},
		},
		"def456": {
			Files: []sourcecontrol.FileStats{
				{Path: "packages/utils/helper.go", Additions: 5, Deletions: 3},
			},
		},
	}

	mockRepo := &mockGitRepository{
		commits:   commits,
		diffStats: diffStats,
	}

	mockVersions := &mockVersionReader{
		versions: map[string]version.SemanticVersion{
			"packages/core":  version.NewSemanticVersion(1, 0, 0),
			"packages/utils": version.NewSemanticVersion(0, 5, 0),
		},
	}

	ws := &workspace.Workspace{
		ID:             "test-workspace",
		RootPath:       "/test",
		Type:           workspace.WorkspaceTypePnpm,
		PackageManager: workspace.PackageManagerPnpm,
		Packages: []*workspace.Package{
			{Name: "core", Path: "packages/core", Version: "1.0.0"},
			{Name: "utils", Path: "packages/utils", Version: "0.5.0"},
		},
	}

	analyzer := NewMonorepoAnalyzer(mockRepo, version.NewDefaultVersionCalculator(), nil, mockVersions)
	orchestrator := NewOrchestrator(analyzer, monorepo.TagPatternSlash)

	input := OrchestratorInput{
		RepositoryPath: "/test",
		FromRef:        "v1.0.0",
		ToRef:          "HEAD",
		Workspace:      ws,
		Strategy:       monorepo.StrategyIndependent,
	}

	output, err := orchestrator.Plan(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, output)

	// Verify release was created
	assert.NotNil(t, output.Release)
	assert.Equal(t, monorepo.StatePlanned, output.Release.State)

	// Verify version plan
	assert.NotNil(t, output.VersionPlan)
	coreEntry := output.VersionPlan.GetEntry("packages/core")
	require.NotNil(t, coreEntry)
	assert.Equal(t, monorepo.BumpTypeMinor, coreEntry.BumpType)

	utilsEntry := output.VersionPlan.GetEntry("packages/utils")
	require.NotNil(t, utilsEntry)
	assert.Equal(t, monorepo.BumpTypePatch, utilsEntry.BumpType)

	// Verify release order
	assert.Len(t, output.ReleaseOrder, 2)

	// Verify package results
	assert.Len(t, output.PackageResults, 2)
}

func TestOrchestrator_PlanWithTargetPackages(t *testing.T) {
	commits := []*sourcecontrol.Commit{
		newTestCommit("abc123", "feat: changes everywhere"),
	}

	diffStats := map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats{
		"abc123": {
			Files: []sourcecontrol.FileStats{
				{Path: "packages/core/main.go", Additions: 10, Deletions: 5},
				{Path: "packages/utils/helper.go", Additions: 10, Deletions: 5},
			},
		},
	}

	mockRepo := &mockGitRepository{
		commits:   commits,
		diffStats: diffStats,
	}

	mockVersions := &mockVersionReader{
		versions: map[string]version.SemanticVersion{
			"packages/core":  version.NewSemanticVersion(1, 0, 0),
			"packages/utils": version.NewSemanticVersion(0, 5, 0),
		},
	}

	ws := &workspace.Workspace{
		ID:       "test-workspace",
		RootPath: "/test",
		Packages: []*workspace.Package{
			{Name: "core", Path: "packages/core"},
			{Name: "utils", Path: "packages/utils"},
		},
	}

	analyzer := NewMonorepoAnalyzer(mockRepo, version.NewDefaultVersionCalculator(), nil, mockVersions)
	orchestrator := NewOrchestrator(analyzer, monorepo.TagPatternSlash)

	input := OrchestratorInput{
		RepositoryPath: "/test",
		FromRef:        "v1.0.0",
		ToRef:          "HEAD",
		Workspace:      ws,
		Strategy:       monorepo.StrategyIndependent,
		TargetPackages: []string{"packages/core"}, // Only target core
	}

	output, err := orchestrator.Plan(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, output)

	// Only core should be included
	included := output.Release.GetIncludedPackages()
	assert.Len(t, included, 1)
	assert.Equal(t, "packages/core", included[0].PackagePath)
}

func TestOrchestrator_ExecuteDryRun(t *testing.T) {
	commits := []*sourcecontrol.Commit{
		newTestCommit("abc123", "feat(core): new feature"),
	}

	diffStats := map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats{
		"abc123": {
			Files: []sourcecontrol.FileStats{
				{Path: "packages/core/main.go", Additions: 10, Deletions: 5},
			},
		},
	}

	mockRepo := &mockGitRepository{
		commits:   commits,
		diffStats: diffStats,
	}

	mockVersions := &mockVersionReader{
		versions: map[string]version.SemanticVersion{
			"packages/core": version.NewSemanticVersion(1, 0, 0),
		},
	}

	ws := &workspace.Workspace{
		ID:       "test-workspace",
		RootPath: "/test",
		Packages: []*workspace.Package{
			{Name: "core", Path: "packages/core"},
		},
	}

	analyzer := NewMonorepoAnalyzer(mockRepo, version.NewDefaultVersionCalculator(), nil, mockVersions)

	publishCalled := false
	orchestrator := NewOrchestrator(analyzer, monorepo.TagPatternSlash).
		WithPublishStep(func(ctx context.Context, pkgPath string, release *monorepo.MonorepoRelease) error {
			publishCalled = true
			return nil
		})

	input := OrchestratorInput{
		RepositoryPath: "/test",
		FromRef:        "v1.0.0",
		ToRef:          "HEAD",
		Workspace:      ws,
		Strategy:       monorepo.StrategyIndependent,
		DryRun:         true,
	}

	output, err := orchestrator.Execute(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, output)

	// Publish should NOT have been called in dry run
	assert.False(t, publishCalled, "publish should not be called in dry run mode")
}

func TestOrchestrator_ExecuteWithSteps(t *testing.T) {
	commits := []*sourcecontrol.Commit{
		newTestCommit("abc123", "feat(core): new feature"),
	}

	diffStats := map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats{
		"abc123": {
			Files: []sourcecontrol.FileStats{
				{Path: "packages/core/main.go", Additions: 10, Deletions: 5},
			},
		},
	}

	mockRepo := &mockGitRepository{
		commits:   commits,
		diffStats: diffStats,
	}

	mockVersions := &mockVersionReader{
		versions: map[string]version.SemanticVersion{
			"packages/core": version.NewSemanticVersion(1, 0, 0),
		},
	}

	ws := &workspace.Workspace{
		ID:       "test-workspace",
		RootPath: "/test",
		Packages: []*workspace.Package{
			{Name: "core", Path: "packages/core"},
		},
	}

	analyzer := NewMonorepoAnalyzer(mockRepo, version.NewDefaultVersionCalculator(), nil, mockVersions)

	var stepsCalled []string
	orchestrator := NewOrchestrator(analyzer, monorepo.TagPatternSlash).
		WithNotesStep(func(ctx context.Context, pkgPath string, release *monorepo.MonorepoRelease) error {
			stepsCalled = append(stepsCalled, "notes:"+pkgPath)
			pkg := release.GetPackageByPath(pkgPath)
			if pkg != nil {
				pkg.SetNotes("Release notes for " + pkgPath)
			}
			return nil
		}).
		WithApproveStep(func(ctx context.Context, pkgPath string, release *monorepo.MonorepoRelease) error {
			stepsCalled = append(stepsCalled, "approve:"+pkgPath)
			return nil
		}).
		WithPublishStep(func(ctx context.Context, pkgPath string, release *monorepo.MonorepoRelease) error {
			stepsCalled = append(stepsCalled, "publish:"+pkgPath)
			return nil
		})

	input := OrchestratorInput{
		RepositoryPath: "/test",
		FromRef:        "v1.0.0",
		ToRef:          "HEAD",
		Workspace:      ws,
		Strategy:       monorepo.StrategyIndependent,
	}

	output, err := orchestrator.Execute(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, output)

	// Verify steps were called in order
	assert.Equal(t, []string{
		"notes:packages/core",
		"approve:packages/core",
		"publish:packages/core",
	}, stepsCalled)

	// Verify result
	result := output.PackageResults["packages/core"]
	require.NotNil(t, result)
	assert.True(t, result.Approved)
	assert.True(t, result.Published)
	assert.Contains(t, result.Notes, "Release notes for packages/core")
}

func TestOrchestrator_ExecuteStepFailure(t *testing.T) {
	commits := []*sourcecontrol.Commit{
		newTestCommit("abc123", "feat(core): new feature"),
	}

	diffStats := map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats{
		"abc123": {
			Files: []sourcecontrol.FileStats{
				{Path: "packages/core/main.go", Additions: 10, Deletions: 5},
			},
		},
	}

	mockRepo := &mockGitRepository{
		commits:   commits,
		diffStats: diffStats,
	}

	mockVersions := &mockVersionReader{
		versions: map[string]version.SemanticVersion{
			"packages/core": version.NewSemanticVersion(1, 0, 0),
		},
	}

	ws := &workspace.Workspace{
		ID:       "test-workspace",
		RootPath: "/test",
		Packages: []*workspace.Package{
			{Name: "core", Path: "packages/core"},
		},
	}

	analyzer := NewMonorepoAnalyzer(mockRepo, version.NewDefaultVersionCalculator(), nil, mockVersions)

	publishCalled := false
	orchestrator := NewOrchestrator(analyzer, monorepo.TagPatternSlash).
		WithApproveStep(func(ctx context.Context, pkgPath string, release *monorepo.MonorepoRelease) error {
			return fmt.Errorf("approval denied")
		}).
		WithPublishStep(func(ctx context.Context, pkgPath string, release *monorepo.MonorepoRelease) error {
			publishCalled = true
			return nil
		})

	input := OrchestratorInput{
		RepositoryPath: "/test",
		FromRef:        "v1.0.0",
		ToRef:          "HEAD",
		Workspace:      ws,
		Strategy:       monorepo.StrategyIndependent,
	}

	output, err := orchestrator.Execute(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, output)

	// Publish should NOT have been called since approval failed
	assert.False(t, publishCalled)

	// Package result should have error
	result := output.PackageResults["packages/core"]
	require.NotNil(t, result)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "approval denied")
}

func TestOrchestrator_NilWorkspace(t *testing.T) {
	mockRepo := &mockGitRepository{}
	mockVersions := &mockVersionReader{versions: map[string]version.SemanticVersion{}}
	analyzer := NewMonorepoAnalyzer(mockRepo, version.NewDefaultVersionCalculator(), nil, mockVersions)
	orchestrator := NewOrchestrator(analyzer, monorepo.TagPatternSlash)

	input := OrchestratorInput{
		RepositoryPath: "/test",
		FromRef:        "v1.0.0",
		ToRef:          "HEAD",
		Workspace:      nil,
	}

	_, err := orchestrator.Plan(context.Background(), input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workspace is required")
}

func TestOrchestrator_LockstepStrategy(t *testing.T) {
	commits := []*sourcecontrol.Commit{
		newTestCommit("abc123", "feat(core): breaking change"),
		newTestCommit("def456", "fix(utils): small fix"),
	}

	diffStats := map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats{
		"abc123": {
			Files: []sourcecontrol.FileStats{
				{Path: "packages/core/main.go", Additions: 50, Deletions: 20},
			},
		},
		"def456": {
			Files: []sourcecontrol.FileStats{
				{Path: "packages/utils/helper.go", Additions: 5, Deletions: 3},
			},
		},
	}

	mockRepo := &mockGitRepository{
		commits:   commits,
		diffStats: diffStats,
	}

	mockVersions := &mockVersionReader{
		versions: map[string]version.SemanticVersion{
			"packages/core":  version.NewSemanticVersion(1, 0, 0),
			"packages/utils": version.NewSemanticVersion(1, 0, 0),
		},
	}

	ws := &workspace.Workspace{
		ID:       "test-workspace",
		RootPath: "/test",
		Packages: []*workspace.Package{
			{Name: "core", Path: "packages/core"},
			{Name: "utils", Path: "packages/utils"},
		},
	}

	analyzer := NewMonorepoAnalyzer(mockRepo, version.NewDefaultVersionCalculator(), nil, mockVersions)
	orchestrator := NewOrchestrator(analyzer, monorepo.TagPatternSlash)

	input := OrchestratorInput{
		RepositoryPath: "/test",
		FromRef:        "v1.0.0",
		ToRef:          "HEAD",
		Workspace:      ws,
		Strategy:       monorepo.StrategyLockstep,
	}

	output, err := orchestrator.Plan(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, output)

	// In lockstep, all packages should have the same next version
	plan := output.VersionPlan
	coreEntry := plan.GetEntry("packages/core")
	utilsEntry := plan.GetEntry("packages/utils")

	if coreEntry != nil && utilsEntry != nil {
		assert.Equal(t, coreEntry.NextVersion, utilsEntry.NextVersion,
			"lockstep packages should share version")
	}
}

// Ensure helper functions from analyzer_test.go are available.
// We use the same package so newTestCommit is already defined.
var _ = time.Now // Suppress unused import
