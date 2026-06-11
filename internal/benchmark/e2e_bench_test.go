// Package benchmark provides end-to-end command benchmarks.
package benchmark

import (
	"context"
	"testing"
	"time"

	analysisfactory "github.com/relicta-tech/relicta/v4/internal/analysis/factory"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/template"
	"github.com/relicta-tech/relicta/v4/internal/service/release"
)

// BenchmarkE2E_PlanCommand benchmarks the complete plan workflow.
// This simulates what happens when a user runs `relicta plan`.
// Target: < 1s for repos with < 1000 commits.
func BenchmarkE2E_PlanCommand(b *testing.B) {
	b.ReportAllocs()

	benchCases := []struct {
		name        string
		commitCount int
	}{
		{"100_commits", 100},
		{"500_commits", 500},
		{"1000_commits", 1000},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			gitRepo := NewMockGitRepo(bc.commitCount)
			factory := analysisfactory.NewFactory(nil)
			versionCalc := NewMockVersionCalc()

			analyzer := release.NewAnalyzer(gitRepo, versionCalc, factory)
			input := release.AnalyzeInput{
				RepositoryPath: "/test/repo",
				Branch:         "main",
				TagPrefix:      "v",
			}

			ctx := context.Background()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Simulate full plan: analyze commits, calculate version, prepare output
				output, err := analyzer.Analyze(ctx, input)
				if err != nil {
					b.Fatalf("Analyze failed: %v", err)
				}

				// Simulate output generation
				_ = output.NextVersion.String()
				_ = output.ReleaseType.String()
				_ = output.ChangeSet.CommitCount()
			}
		})
	}
}

// BenchmarkE2E_NotesCommand benchmarks the complete notes generation workflow.
// This simulates what happens when a user runs `relicta notes`.
// Target: < 500ms for 100 commits.
func BenchmarkE2E_NotesCommand(b *testing.B) {
	b.ReportAllocs()

	benchCases := []struct {
		name        string
		commitCount int
	}{
		{"50_commits", 50},
		{"100_commits", 100},
		{"200_commits", 200},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			// Create template service
			tmplService, err := template.NewService()
			if err != nil {
				b.Fatalf("Failed to create template service: %v", err)
			}

			// Create mock git repo and analyzer
			gitRepo := NewMockGitRepo(bc.commitCount)
			factory := analysisfactory.NewFactory(nil)
			versionCalc := NewMockVersionCalc()

			analyzer := release.NewAnalyzer(gitRepo, versionCalc, factory)
			input := release.AnalyzeInput{
				RepositoryPath: "/test/repo",
				Branch:         "main",
				TagPrefix:      "v",
			}

			ctx := context.Background()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Step 1: Analyze commits (like relicta plan)
				output, err := analyzer.Analyze(ctx, input)
				if err != nil {
					b.Fatalf("Analyze failed: %v", err)
				}

				// Step 2: Render changelog template (core of notes generation)
				changelogData := &template.ChangelogData{
					Version:         &output.NextVersion,
					PreviousVersion: &output.CurrentVersion,
					Date:            time.Now(),
					RepositoryURL:   "https://github.com/example/repo",
				}
				_, err = tmplService.Render("changelog", changelogData)
				if err != nil {
					b.Fatalf("Render failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkE2E_ChangeSetBuild benchmarks building a ChangeSet from commits.
// This is a core operation in the release workflow.
func BenchmarkE2E_ChangeSetBuild(b *testing.B) {
	b.ReportAllocs()

	benchCases := []struct {
		name        string
		commitCount int
	}{
		{"100_commits", 100},
		{"500_commits", 500},
		{"1000_commits", 1000},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			commits := generateBenchCommits(bc.commitCount)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				cs := changes.NewChangeSet("bench-cs", "v1.0.0", "HEAD")
				for _, commit := range commits {
					cs.AddCommit(commit)
				}
				_ = cs.ReleaseType()
			}
		})
	}
}

// BenchmarkE2E_VersionCalculation benchmarks version bump calculation.
func BenchmarkE2E_VersionCalculation(b *testing.B) {
	b.ReportAllocs()

	calc := version.NewDefaultVersionCalculator()

	benchCases := []struct {
		name     string
		current  string
		bumpType version.BumpType
	}{
		{"patch", "1.2.3", version.BumpPatch},
		{"minor", "1.2.3", version.BumpMinor},
		{"major", "1.2.3", version.BumpMajor},
		{"prerelease", "2.0.0-alpha.1", version.BumpPatch},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			current, _ := version.Parse(bc.current)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = calc.CalculateNextVersion(current, bc.bumpType)
			}
		})
	}
}

// BenchmarkE2E_CommitParsing benchmarks conventional commit parsing.
func BenchmarkE2E_CommitParsing(b *testing.B) {
	b.ReportAllocs()

	benchCases := []struct {
		name    string
		message string
	}{
		{"simple_feat", "feat: add new feature"},
		{"with_scope", "feat(api): add user authentication"},
		{"breaking", "feat(api)!: change API response format"},
		{"with_body", "fix(core): resolve memory leak\n\nThis fixes the memory leak in the event loop."},
		{"with_footer", "feat(api): add rate limiting\n\nBREAKING CHANGE: API now returns 429 for rate limited requests."},
		{"non_conventional", "Update README.md"},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = changes.ParseConventionalCommit(
					"abc123def456",
					bc.message,
					changes.WithAuthor("Test User", "test@example.com"),
					changes.WithDate(time.Now()),
				)
			}
		})
	}
}

// BenchmarkE2E_CommitAnalysis benchmarks the full commit analysis pipeline.
func BenchmarkE2E_CommitAnalysis(b *testing.B) {
	b.ReportAllocs()

	benchCases := []struct {
		name       string
		commitFunc func() []*sourcecontrol.Commit
	}{
		{"100_commits", func() []*sourcecontrol.Commit { return Commits100 }},
		{"500_commits", func() []*sourcecontrol.Commit { return Commits500 }},
		{"1000_commits", func() []*sourcecontrol.Commit { return Commits1000 }},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			commits := bc.commitFunc()
			gitRepo := &E2EBenchMockGitRepo{
				Info: &sourcecontrol.RepositoryInfo{
					Name:          "bench-repo",
					Owner:         "relicta",
					CurrentBranch: "main",
				},
				Tags:    sourcecontrol.TagList{sourcecontrol.NewTag("v1.0.0", "abc123")},
				Commits: commits,
			}
			factory := analysisfactory.NewFactory(nil)
			versionCalc := NewMockVersionCalc()

			analyzer := release.NewAnalyzer(gitRepo, versionCalc, factory)
			input := release.AnalyzeInput{
				RepositoryPath: "/test/repo",
				TagPrefix:      "v",
			}

			ctx := context.Background()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _, err := analyzer.AnalyzeCommits(ctx, input)
				if err != nil {
					b.Fatalf("AnalyzeCommits failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkE2E_Categories benchmarks change categorization.
func BenchmarkE2E_Categories(b *testing.B) {
	b.ReportAllocs()

	benchCases := []struct {
		name        string
		commitCount int
	}{
		{"50_commits", 50},
		{"100_commits", 100},
		{"500_commits", 500},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			commits := generateBenchCommits(bc.commitCount)
			cs := changes.NewChangeSet("bench-cs", "v1.0.0", "HEAD")
			for _, commit := range commits {
				cs.AddCommit(commit)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = cs.Categories()
			}
		})
	}
}

// generateBenchCommits generates conventional commits for benchmarking.
func generateBenchCommits(count int) []*changes.ConventionalCommit {
	commits := make([]*changes.ConventionalCommit, count)
	types := []changes.CommitType{
		changes.CommitTypeFeat,
		changes.CommitTypeFix,
		changes.CommitTypeDocs,
		changes.CommitTypeRefactor,
		changes.CommitTypePerf,
		changes.CommitTypeTest,
		changes.CommitTypeChore,
	}
	scopes := []string{"api", "cli", "config", "core", "plugin", "release", "template"}
	descriptions := []string{
		"add new feature",
		"fix critical bug",
		"update documentation",
		"refactor code structure",
		"improve performance",
		"add unit tests",
		"update dependencies",
	}

	for i := 0; i < count; i++ {
		commitType := types[i%len(types)]
		scope := scopes[i%len(scopes)]
		desc := descriptions[i%len(descriptions)]

		commits[i] = changes.NewConventionalCommit(
			generateBenchHashForCommit(i),
			commitType,
			desc,
			changes.WithScope(scope),
			changes.WithAuthor("Benchmark User", "bench@example.com"),
			changes.WithDate(time.Now().Add(-time.Duration(i)*time.Hour)),
		)
	}

	return commits
}

func generateBenchHashForCommit(idx int) string {
	hash := make([]byte, 40)
	for i := range hash {
		hash[i] = "0123456789abcdef"[(idx+i)%16]
	}
	return string(hash)
}

// E2EBenchMockGitRepo is a mock git repository for e2e benchmarking.
type E2EBenchMockGitRepo struct {
	Info    *sourcecontrol.RepositoryInfo
	Tags    sourcecontrol.TagList
	Commits []*sourcecontrol.Commit
}

func (m *E2EBenchMockGitRepo) GetInfo(ctx context.Context) (*sourcecontrol.RepositoryInfo, error) {
	return m.Info, nil
}

func (m *E2EBenchMockGitRepo) GetTags(ctx context.Context) (sourcecontrol.TagList, error) {
	return m.Tags, nil
}

func (m *E2EBenchMockGitRepo) GetCommitsBetween(ctx context.Context, from, to string) ([]*sourcecontrol.Commit, error) {
	return m.Commits, nil
}

func (m *E2EBenchMockGitRepo) GetCommitDiffStats(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.DiffStats, error) {
	return &sourcecontrol.DiffStats{
		Files: []sourcecontrol.FileStats{
			{Path: "internal/service/release/analyzer.go", Additions: 10, Deletions: 5},
		},
		Additions: 10,
		Deletions: 5,
	}, nil
}

func (m *E2EBenchMockGitRepo) GetBatchCommitDiffStats(ctx context.Context, hashes []sourcecontrol.CommitHash) (map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats, error) {
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

func (m *E2EBenchMockGitRepo) GetRemotes(ctx context.Context) ([]sourcecontrol.RemoteInfo, error) {
	return nil, nil
}
func (m *E2EBenchMockGitRepo) GetBranches(ctx context.Context) ([]sourcecontrol.BranchInfo, error) {
	return nil, nil
}
func (m *E2EBenchMockGitRepo) GetCurrentBranch(ctx context.Context) (string, error) {
	return "main", nil
}
func (m *E2EBenchMockGitRepo) GetCommit(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.Commit, error) {
	return nil, nil
}
func (m *E2EBenchMockGitRepo) GetCommitsSince(ctx context.Context, ref string) ([]*sourcecontrol.Commit, error) {
	return nil, nil
}
func (m *E2EBenchMockGitRepo) GetLatestCommit(ctx context.Context, branch string) (*sourcecontrol.Commit, error) {
	return nil, nil
}
func (m *E2EBenchMockGitRepo) GetCommitPatch(ctx context.Context, hash sourcecontrol.CommitHash) (string, error) {
	return "", nil
}
func (m *E2EBenchMockGitRepo) GetFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	return nil, nil
}
func (m *E2EBenchMockGitRepo) GetTag(ctx context.Context, name string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (m *E2EBenchMockGitRepo) GetLatestVersionTag(ctx context.Context, prefix string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (m *E2EBenchMockGitRepo) CreateTag(ctx context.Context, name string, hash sourcecontrol.CommitHash, message string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (m *E2EBenchMockGitRepo) DeleteTag(ctx context.Context, name string) error { return nil }
func (m *E2EBenchMockGitRepo) PushTag(ctx context.Context, name string, remote string) error {
	return nil
}
func (m *E2EBenchMockGitRepo) IsDirty(ctx context.Context) (bool, error) { return false, nil }
func (m *E2EBenchMockGitRepo) GetStatus(ctx context.Context) (*sourcecontrol.WorkingTreeStatus, error) {
	return &sourcecontrol.WorkingTreeStatus{IsClean: true}, nil
}
func (m *E2EBenchMockGitRepo) Fetch(ctx context.Context, remote string) error        { return nil }
func (m *E2EBenchMockGitRepo) Pull(ctx context.Context, remote, branch string) error { return nil }
func (m *E2EBenchMockGitRepo) Push(ctx context.Context, remote, branch string) error { return nil }
