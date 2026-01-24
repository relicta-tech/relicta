// Package benchmark provides testing fixtures and utilities for performance benchmarks.
package benchmark

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// CommitTypeDistribution defines the probability distribution for commit types.
// Reflects realistic repository patterns based on empirical data.
type CommitTypeDistribution struct {
	Feat     float64
	Fix      float64
	Docs     float64
	Style    float64
	Refactor float64
	Perf     float64
	Test     float64
	Build    float64
	CI       float64
	Chore    float64
	Revert   float64
}

// DefaultDistribution returns a realistic commit type distribution.
// Based on empirical data from open-source repositories.
func DefaultDistribution() CommitTypeDistribution {
	return CommitTypeDistribution{
		Feat:     0.25, // 25% new features
		Fix:      0.30, // 30% bug fixes
		Docs:     0.08, // 8% documentation
		Style:    0.03, // 3% formatting
		Refactor: 0.10, // 10% refactoring
		Perf:     0.02, // 2% performance
		Test:     0.07, // 7% test changes
		Build:    0.05, // 5% build system
		CI:       0.03, // 3% CI changes
		Chore:    0.05, // 5% maintenance
		Revert:   0.02, // 2% reverts
	}
}

// CommitGenerator generates realistic test commits for benchmarking.
type CommitGenerator struct {
	dist         CommitTypeDistribution
	scopes       []string
	subjects     map[changes.CommitType][]string
	breakingRate float64
}

// NewCommitGenerator creates a new commit generator with the given distribution.
func NewCommitGenerator(dist CommitTypeDistribution) *CommitGenerator {
	return &CommitGenerator{
		dist: dist,
		scopes: []string{
			"api", "cli", "config", "core", "docs", "git", "plugin",
			"release", "state", "template", "ui", "version",
		},
		subjects:     defaultSubjects(),
		breakingRate: 0.05, // 5% breaking changes
	}
}

// defaultSubjects returns realistic commit subjects per type.
func defaultSubjects() map[changes.CommitType][]string {
	return map[changes.CommitType][]string{
		changes.CommitTypeFeat: {
			"add user authentication",
			"implement plugin system",
			"add release notes generation",
			"support custom templates",
			"add changelog formatting",
			"implement version bumping",
			"add CI/CD integration",
			"support monorepo workflows",
			"add approval workflows",
			"implement audit logging",
		},
		changes.CommitTypeFix: {
			"resolve race condition in state manager",
			"fix version parsing edge case",
			"correct changelog date format",
			"fix plugin loading error",
			"resolve git tag creation issue",
			"fix template rendering bug",
			"correct release notes ordering",
			"fix memory leak in commit analysis",
			"resolve configuration merge conflict",
			"fix approval workflow timeout",
		},
		changes.CommitTypeDocs: {
			"update README installation guide",
			"add API documentation",
			"document configuration options",
			"add contributing guidelines",
			"update changelog format docs",
			"document plugin development",
			"add architecture overview",
			"update CLI reference",
		},
		changes.CommitTypeStyle: {
			"format code with gofmt",
			"fix linting issues",
			"update code style",
			"standardize imports",
		},
		changes.CommitTypeRefactor: {
			"extract commit analysis logic",
			"reorganize package structure",
			"simplify version calculation",
			"improve error handling",
			"refactor plugin interface",
			"extract common utilities",
			"improve type safety",
			"consolidate duplicate code",
		},
		changes.CommitTypePerf: {
			"optimize commit parsing",
			"improve memory usage",
			"speed up version lookup",
			"reduce allocations in hot path",
			"optimize git operations",
		},
		changes.CommitTypeTest: {
			"add unit tests for analyzer",
			"improve test coverage",
			"add integration tests",
			"fix flaky test",
			"add benchmark tests",
			"test edge cases",
		},
		changes.CommitTypeBuild: {
			"update Go version",
			"upgrade dependencies",
			"fix build configuration",
			"add goreleaser config",
			"update Makefile targets",
		},
		changes.CommitTypeCI: {
			"add GitHub Actions workflow",
			"update CI pipeline",
			"add security scanning",
			"configure release automation",
		},
		changes.CommitTypeChore: {
			"update .gitignore",
			"clean up unused files",
			"update license",
			"bump version",
		},
		changes.CommitTypeRevert: {
			"revert breaking change",
			"revert failed experiment",
		},
	}
}

// GenerateCommits generates n test commits with realistic distribution.
func (g *CommitGenerator) GenerateCommits(n int) []*sourcecontrol.Commit {
	commits := make([]*sourcecontrol.Commit, n)

	// Pre-compute cumulative distribution for weighted selection
	types := []changes.CommitType{
		changes.CommitTypeFeat, changes.CommitTypeFix, changes.CommitTypeDocs,
		changes.CommitTypeStyle, changes.CommitTypeRefactor, changes.CommitTypePerf,
		changes.CommitTypeTest, changes.CommitTypeBuild, changes.CommitTypeCI,
		changes.CommitTypeChore, changes.CommitTypeRevert,
	}
	weights := []float64{
		g.dist.Feat, g.dist.Fix, g.dist.Docs, g.dist.Style, g.dist.Refactor,
		g.dist.Perf, g.dist.Test, g.dist.Build, g.dist.CI, g.dist.Chore, g.dist.Revert,
	}

	cumulative := make([]float64, len(weights))
	total := 0.0
	for i, w := range weights {
		total += w
		cumulative[i] = total
	}

	baseTime := time.Now().Add(-time.Duration(n) * time.Hour)

	for i := 0; i < n; i++ {
		// Select commit type based on distribution
		r := randomFloat() * total
		typeIdx := 0
		for j, c := range cumulative {
			if r <= c {
				typeIdx = j
				break
			}
		}
		commitType := types[typeIdx]

		// Generate message
		scope := g.scopes[i%len(g.scopes)]
		subjects := g.subjects[commitType]
		subject := subjects[i%len(subjects)]

		// Determine if breaking change
		isBreaking := randomFloat() < g.breakingRate && commitType == changes.CommitTypeFeat

		var message string
		if isBreaking {
			message = fmt.Sprintf("%s(%s)!: %s\n\nBREAKING CHANGE: This is a breaking change.", commitType, scope, subject)
		} else {
			message = fmt.Sprintf("%s(%s): %s", commitType, scope, subject)
		}

		commits[i] = sourcecontrol.NewCommit(
			sourcecontrol.CommitHash(generateHash()),
			message,
			sourcecontrol.Author{
				Name:  "Benchmark User",
				Email: "bench@example.com",
			},
			baseTime.Add(time.Duration(i)*time.Hour),
		)
	}

	return commits
}

// generateHash generates a random commit hash.
func generateHash() string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// randomFloat returns a random float64 between 0 and 1.
func randomFloat() float64 {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	// Convert to float64 in [0, 1)
	var v uint64
	for i := 0; i < 8; i++ {
		v = (v << 8) | uint64(b[i])
	}
	return float64(v) / float64(1<<64)
}

// MockGitRepo is a mock git repository for benchmarking.
type MockGitRepo struct {
	Info    *sourcecontrol.RepositoryInfo
	Tags    sourcecontrol.TagList
	Commits []*sourcecontrol.Commit
}

// NewMockGitRepo creates a mock git repository with pre-generated commits.
func NewMockGitRepo(commitCount int) *MockGitRepo {
	gen := NewCommitGenerator(DefaultDistribution())
	commits := gen.GenerateCommits(commitCount)

	tags := sourcecontrol.TagList{
		sourcecontrol.NewTag("v1.0.0", sourcecontrol.CommitHash("abc123")),
	}

	return &MockGitRepo{
		Info: &sourcecontrol.RepositoryInfo{
			Name:          "benchmark-repo",
			Owner:         "relicta",
			CurrentBranch: "main",
		},
		Tags:    tags,
		Commits: commits,
	}
}

// GetInfo returns repository information.
func (m *MockGitRepo) GetInfo(ctx context.Context) (*sourcecontrol.RepositoryInfo, error) {
	return m.Info, nil
}

// GetTags returns repository tags.
func (m *MockGitRepo) GetTags(ctx context.Context) (sourcecontrol.TagList, error) {
	return m.Tags, nil
}

// GetCommitsBetween returns commits between two refs.
func (m *MockGitRepo) GetCommitsBetween(ctx context.Context, from, to string) ([]*sourcecontrol.Commit, error) {
	return m.Commits, nil
}

// GetCommitDiffStats returns diff stats for a commit.
func (m *MockGitRepo) GetCommitDiffStats(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.DiffStats, error) {
	// Return minimal diff stats for benchmarking
	return &sourcecontrol.DiffStats{
		Files: []sourcecontrol.FileStats{
			{Path: "internal/service/release/analyzer.go", Additions: 10, Deletions: 5},
		},
		Additions: 10,
		Deletions: 5,
	}, nil
}

// GetBatchCommitDiffStats returns diff stats for multiple commits.
func (m *MockGitRepo) GetBatchCommitDiffStats(ctx context.Context, hashes []sourcecontrol.CommitHash) (map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats, error) {
	result := make(map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats, len(hashes))
	for _, hash := range hashes {
		result[hash] = &sourcecontrol.DiffStats{
			Files: []sourcecontrol.FileStats{
				{Path: "internal/service/release/analyzer.go", Additions: 10, Deletions: 5},
			},
			Additions: 10,
			Deletions: 5,
		}
	}
	return result, nil
}

// GetRemotes returns remote information.
func (m *MockGitRepo) GetRemotes(ctx context.Context) ([]sourcecontrol.RemoteInfo, error) {
	return nil, nil
}

// GetBranches returns branch information.
func (m *MockGitRepo) GetBranches(ctx context.Context) ([]sourcecontrol.BranchInfo, error) {
	return nil, nil
}

// GetCurrentBranch returns the current branch.
func (m *MockGitRepo) GetCurrentBranch(ctx context.Context) (string, error) {
	return "main", nil
}

// GetCommit returns a commit by hash.
func (m *MockGitRepo) GetCommit(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.Commit, error) {
	return nil, nil
}

// GetCommitsSince returns commits since a ref.
func (m *MockGitRepo) GetCommitsSince(ctx context.Context, ref string) ([]*sourcecontrol.Commit, error) {
	return nil, nil
}

// GetLatestCommit returns the latest commit on a branch.
func (m *MockGitRepo) GetLatestCommit(ctx context.Context, branch string) (*sourcecontrol.Commit, error) {
	return nil, nil
}

// GetCommitPatch returns the patch for a commit.
func (m *MockGitRepo) GetCommitPatch(ctx context.Context, hash sourcecontrol.CommitHash) (string, error) {
	return "", nil
}

// GetFileAtRef returns file contents at a ref.
func (m *MockGitRepo) GetFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	return nil, nil
}

// GetTag returns a tag by name.
func (m *MockGitRepo) GetTag(ctx context.Context, name string) (*sourcecontrol.Tag, error) {
	return nil, nil
}

// GetLatestVersionTag returns the latest version tag.
func (m *MockGitRepo) GetLatestVersionTag(ctx context.Context, prefix string) (*sourcecontrol.Tag, error) {
	return nil, nil
}

// CreateTag creates a new tag.
func (m *MockGitRepo) CreateTag(ctx context.Context, name string, hash sourcecontrol.CommitHash, message string) (*sourcecontrol.Tag, error) {
	return nil, nil
}

// DeleteTag deletes a tag.
func (m *MockGitRepo) DeleteTag(ctx context.Context, name string) error {
	return nil
}

// PushTag pushes a tag to remote.
func (m *MockGitRepo) PushTag(ctx context.Context, name string, remote string) error {
	return nil
}

// IsDirty returns whether the working tree is dirty.
func (m *MockGitRepo) IsDirty(ctx context.Context) (bool, error) {
	return false, nil
}

// GetStatus returns the working tree status.
func (m *MockGitRepo) GetStatus(ctx context.Context) (*sourcecontrol.WorkingTreeStatus, error) {
	return &sourcecontrol.WorkingTreeStatus{IsClean: true}, nil
}

// Fetch fetches from remote.
func (m *MockGitRepo) Fetch(ctx context.Context, remote string) error {
	return nil
}

// Pull pulls from remote.
func (m *MockGitRepo) Pull(ctx context.Context, remote, branch string) error {
	return nil
}

// Push pushes to remote.
func (m *MockGitRepo) Push(ctx context.Context, remote, branch string) error {
	return nil
}

// Pre-generated commit sets for common benchmark sizes.
var (
	// Commits10 contains 10 pre-generated commits.
	Commits10 []*sourcecontrol.Commit

	// Commits100 contains 100 pre-generated commits.
	Commits100 []*sourcecontrol.Commit

	// Commits500 contains 500 pre-generated commits.
	Commits500 []*sourcecontrol.Commit

	// Commits1000 contains 1000 pre-generated commits.
	Commits1000 []*sourcecontrol.Commit
)

func init() {
	gen := NewCommitGenerator(DefaultDistribution())
	Commits10 = gen.GenerateCommits(10)
	Commits100 = gen.GenerateCommits(100)
	Commits500 = gen.GenerateCommits(500)
	Commits1000 = gen.GenerateCommits(1000)
}

// MockVersionCalc is a mock version calculator for benchmarking.
type MockVersionCalc struct {
	NextVersion version.SemanticVersion
}

// NewMockVersionCalc creates a new mock version calculator.
func NewMockVersionCalc() *MockVersionCalc {
	v, _ := version.Parse("2.0.0")
	return &MockVersionCalc{NextVersion: v}
}

// CalculateNextVersion calculates the next version.
func (m *MockVersionCalc) CalculateNextVersion(current version.SemanticVersion, bump version.BumpType) version.SemanticVersion {
	return m.NextVersion
}

// DetermineRequiredBump determines the required bump type.
func (m *MockVersionCalc) DetermineRequiredBump(hasBreaking, hasFeature, hasFix bool) version.BumpType {
	if hasBreaking {
		return version.BumpMajor
	}
	if hasFeature {
		return version.BumpMinor
	}
	return version.BumpPatch
}
