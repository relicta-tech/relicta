// Package release provides release analysis service benchmarks.
package release

import (
	"context"
	"testing"

	analysisfactory "github.com/relicta-tech/relicta/internal/analysis/factory"
	"github.com/relicta-tech/relicta/internal/benchmark"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
)

// BenchmarkAnalyzer_Analyze benchmarks the full analysis pipeline.
// Target: < 1s for repos with < 1000 commits.
func BenchmarkAnalyzer_Analyze(b *testing.B) {
	b.ReportAllocs()

	benchCases := []struct {
		name       string
		commitFunc func() []*sourcecontrol.Commit
	}{
		{"10_commits", func() []*sourcecontrol.Commit { return benchmark.Commits10 }},
		{"100_commits", func() []*sourcecontrol.Commit { return benchmark.Commits100 }},
		{"500_commits", func() []*sourcecontrol.Commit { return benchmark.Commits500 }},
		{"1000_commits", func() []*sourcecontrol.Commit { return benchmark.Commits1000 }},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			commits := bc.commitFunc()
			gitRepo := &benchMockGitRepo{
				info: &sourcecontrol.RepositoryInfo{
					Name:          "bench-repo",
					Owner:         "relicta",
					CurrentBranch: "main",
				},
				tags:    sourcecontrol.TagList{sourcecontrol.NewTag("v1.0.0", "abc123")},
				commits: commits,
			}
			factory := analysisfactory.NewFactory(nil)
			versionCalc := benchmark.NewMockVersionCalc()

			analyzer := NewAnalyzer(gitRepo, versionCalc, factory)
			input := AnalyzeInput{
				RepositoryPath: "/test/repo",
				Branch:         "main",
				TagPrefix:      "v",
			}

			ctx := context.Background()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := analyzer.Analyze(ctx, input)
				if err != nil {
					b.Fatalf("Analyze failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkAnalyzer_collectCommits benchmarks the git operations isolation.
func BenchmarkAnalyzer_collectCommits(b *testing.B) {
	b.ReportAllocs()

	benchCases := []struct {
		name       string
		commitFunc func() []*sourcecontrol.Commit
	}{
		{"10_commits", func() []*sourcecontrol.Commit { return benchmark.Commits10 }},
		{"100_commits", func() []*sourcecontrol.Commit { return benchmark.Commits100 }},
		{"500_commits", func() []*sourcecontrol.Commit { return benchmark.Commits500 }},
		{"1000_commits", func() []*sourcecontrol.Commit { return benchmark.Commits1000 }},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			commits := bc.commitFunc()
			gitRepo := &benchMockGitRepo{
				info: &sourcecontrol.RepositoryInfo{
					Name:          "bench-repo",
					Owner:         "relicta",
					CurrentBranch: "main",
				},
				tags:    sourcecontrol.TagList{sourcecontrol.NewTag("v1.0.0", "abc123")},
				commits: commits,
			}
			factory := analysisfactory.NewFactory(nil)
			versionCalc := benchmark.NewMockVersionCalc()

			analyzer := NewAnalyzer(gitRepo, versionCalc, factory)
			input := AnalyzeInput{
				RepositoryPath: "/test/repo",
				TagPrefix:      "v",
			}

			ctx := context.Background()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _, _, commits, err := analyzer.collectCommits(ctx, input)
				if err != nil {
					b.Fatalf("collectCommits failed: %v", err)
				}
				_ = commits
			}
		})
	}
}

// BenchmarkAnalyzer_AnalyzeCommits benchmarks the commit classification pipeline.
func BenchmarkAnalyzer_AnalyzeCommits(b *testing.B) {
	b.ReportAllocs()

	benchCases := []struct {
		name       string
		commitFunc func() []*sourcecontrol.Commit
	}{
		{"10_commits", func() []*sourcecontrol.Commit { return benchmark.Commits10 }},
		{"100_commits", func() []*sourcecontrol.Commit { return benchmark.Commits100 }},
		{"500_commits", func() []*sourcecontrol.Commit { return benchmark.Commits500 }},
		{"1000_commits", func() []*sourcecontrol.Commit { return benchmark.Commits1000 }},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			commits := bc.commitFunc()
			gitRepo := &benchMockGitRepo{
				info: &sourcecontrol.RepositoryInfo{
					Name:          "bench-repo",
					Owner:         "relicta",
					CurrentBranch: "main",
				},
				tags:    sourcecontrol.TagList{sourcecontrol.NewTag("v1.0.0", "abc123")},
				commits: commits,
			}
			factory := analysisfactory.NewFactory(nil)
			versionCalc := benchmark.NewMockVersionCalc()

			analyzer := NewAnalyzer(gitRepo, versionCalc, factory)
			input := AnalyzeInput{
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

// BenchmarkAnalyzeInput_Validate benchmarks input validation.
func BenchmarkAnalyzeInput_Validate(b *testing.B) {
	b.ReportAllocs()

	b.Run("valid_input", func(b *testing.B) {
		input := AnalyzeInput{
			RepositoryPath: "/path/to/repo",
			Branch:         "main",
			FromRef:        "v1.0.0",
			ToRef:          "HEAD",
			TagPrefix:      "v",
		}

		for i := 0; i < b.N; i++ {
			_ = input.Validate()
		}
	})

	b.Run("empty_input", func(b *testing.B) {
		input := AnalyzeInput{}

		for i := 0; i < b.N; i++ {
			_ = input.Validate()
		}
	})

	b.Run("feature_branch", func(b *testing.B) {
		input := AnalyzeInput{
			Branch: "feature/add-user-authentication",
		}

		for i := 0; i < b.N; i++ {
			_ = input.Validate()
		}
	})
}

// benchMockGitRepo is a minimal mock for benchmarking.
type benchMockGitRepo struct {
	info    *sourcecontrol.RepositoryInfo
	tags    sourcecontrol.TagList
	commits []*sourcecontrol.Commit
}

func (m *benchMockGitRepo) GetInfo(ctx context.Context) (*sourcecontrol.RepositoryInfo, error) {
	return m.info, nil
}

func (m *benchMockGitRepo) GetTags(ctx context.Context) (sourcecontrol.TagList, error) {
	return m.tags, nil
}

func (m *benchMockGitRepo) GetCommitsBetween(ctx context.Context, from, to string) ([]*sourcecontrol.Commit, error) {
	return m.commits, nil
}

func (m *benchMockGitRepo) GetCommitDiffStats(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.DiffStats, error) {
	return &sourcecontrol.DiffStats{
		Files: []sourcecontrol.FileStats{
			{Path: "internal/service/release/analyzer.go", Additions: 10, Deletions: 5},
		},
		Additions: 10,
		Deletions: 5,
	}, nil
}

func (m *benchMockGitRepo) GetBatchCommitDiffStats(ctx context.Context, hashes []sourcecontrol.CommitHash) (map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats, error) {
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

func (m *benchMockGitRepo) GetRemotes(ctx context.Context) ([]sourcecontrol.RemoteInfo, error) {
	return nil, nil
}
func (m *benchMockGitRepo) GetBranches(ctx context.Context) ([]sourcecontrol.BranchInfo, error) {
	return nil, nil
}
func (m *benchMockGitRepo) GetCurrentBranch(ctx context.Context) (string, error) { return "main", nil }
func (m *benchMockGitRepo) GetCommit(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.Commit, error) {
	return nil, nil
}
func (m *benchMockGitRepo) GetCommitsSince(ctx context.Context, ref string) ([]*sourcecontrol.Commit, error) {
	return nil, nil
}
func (m *benchMockGitRepo) GetLatestCommit(ctx context.Context, branch string) (*sourcecontrol.Commit, error) {
	return nil, nil
}
func (m *benchMockGitRepo) GetCommitPatch(ctx context.Context, hash sourcecontrol.CommitHash) (string, error) {
	return "", nil
}
func (m *benchMockGitRepo) GetFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	return nil, nil
}
func (m *benchMockGitRepo) GetTag(ctx context.Context, name string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (m *benchMockGitRepo) GetLatestVersionTag(ctx context.Context, prefix string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (m *benchMockGitRepo) CreateTag(ctx context.Context, name string, hash sourcecontrol.CommitHash, message string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (m *benchMockGitRepo) DeleteTag(ctx context.Context, name string) error { return nil }
func (m *benchMockGitRepo) PushTag(ctx context.Context, name string, remote string) error {
	return nil
}
func (m *benchMockGitRepo) IsDirty(ctx context.Context) (bool, error) { return false, nil }
func (m *benchMockGitRepo) GetStatus(ctx context.Context) (*sourcecontrol.WorkingTreeStatus, error) {
	return &sourcecontrol.WorkingTreeStatus{IsClean: true}, nil
}
func (m *benchMockGitRepo) Fetch(ctx context.Context, remote string) error        { return nil }
func (m *benchMockGitRepo) Pull(ctx context.Context, remote, branch string) error { return nil }
func (m *benchMockGitRepo) Push(ctx context.Context, remote, branch string) error { return nil }
