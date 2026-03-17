// Package benchmark provides memory-focused performance benchmarks.
package benchmark

import (
	"context"
	"runtime"
	"testing"

	analysisfactory "github.com/relicta-tech/relicta/internal/analysis/factory"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/service/release"
)

// BenchmarkMemory_PlanCommand benchmarks memory usage for the plan workflow.
// Target: < 50MB typical memory usage for 1000 commits.
func BenchmarkMemory_PlanCommand(b *testing.B) {
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

			// Measure memory before
			var memStatsBefore, memStatsAfter runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&memStatsBefore)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := analyzer.Analyze(ctx, input)
				if err != nil {
					b.Fatalf("Analyze failed: %v", err)
				}
			}

			b.StopTimer()

			// Measure memory after
			runtime.ReadMemStats(&memStatsAfter)

			// Report memory metrics
			allocBytes := memStatsAfter.TotalAlloc - memStatsBefore.TotalAlloc
			allocBytesPerOp := allocBytes / uint64(b.N)
			b.ReportMetric(float64(allocBytesPerOp), "B/op-total")
		})
	}
}

// BenchmarkMemory_CommitGeneration benchmarks memory for commit generation.
func BenchmarkMemory_CommitGeneration(b *testing.B) {
	b.ReportAllocs()

	benchCases := []struct {
		name  string
		count int
	}{
		{"100_commits", 100},
		{"500_commits", 500},
		{"1000_commits", 1000},
		{"5000_commits", 5000},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			gen := NewCommitGenerator(DefaultDistribution())

			var memStatsBefore, memStatsAfter runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&memStatsBefore)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				commits := gen.GenerateCommits(bc.count)
				_ = commits
			}

			b.StopTimer()

			runtime.ReadMemStats(&memStatsAfter)
			allocBytes := memStatsAfter.TotalAlloc - memStatsBefore.TotalAlloc
			allocBytesPerOp := allocBytes / uint64(b.N)
			b.ReportMetric(float64(allocBytesPerOp), "B/op-total")
		})
	}
}

// BenchmarkMemory_MockGitRepo benchmarks memory for mock repository creation.
func BenchmarkMemory_MockGitRepo(b *testing.B) {
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
			var memStatsBefore, memStatsAfter runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&memStatsBefore)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				repo := NewMockGitRepo(bc.commitCount)
				_ = repo
			}

			b.StopTimer()

			runtime.ReadMemStats(&memStatsAfter)
			allocBytes := memStatsAfter.TotalAlloc - memStatsBefore.TotalAlloc
			allocBytesPerOp := allocBytes / uint64(b.N)
			b.ReportMetric(float64(allocBytesPerOp), "B/op-total")
		})
	}
}

// BenchmarkMemory_AnalyzeCommits benchmarks memory for commit analysis.
func BenchmarkMemory_AnalyzeCommits(b *testing.B) {
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
			gitRepo := &MemoryBenchMockGitRepo{
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

			var memStatsBefore, memStatsAfter runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&memStatsBefore)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _, err := analyzer.AnalyzeCommits(ctx, input)
				if err != nil {
					b.Fatalf("AnalyzeCommits failed: %v", err)
				}
			}

			b.StopTimer()

			runtime.ReadMemStats(&memStatsAfter)
			allocBytes := memStatsAfter.TotalAlloc - memStatsBefore.TotalAlloc
			allocBytesPerOp := allocBytes / uint64(b.N)
			b.ReportMetric(float64(allocBytesPerOp), "B/op-total")
		})
	}
}

// MemoryBenchMockGitRepo is a mock git repository for memory benchmarking.
type MemoryBenchMockGitRepo struct {
	Info    *sourcecontrol.RepositoryInfo
	Tags    sourcecontrol.TagList
	Commits []*sourcecontrol.Commit
}

func (m *MemoryBenchMockGitRepo) GetInfo(ctx context.Context) (*sourcecontrol.RepositoryInfo, error) {
	return m.Info, nil
}

func (m *MemoryBenchMockGitRepo) GetTags(ctx context.Context) (sourcecontrol.TagList, error) {
	return m.Tags, nil
}

func (m *MemoryBenchMockGitRepo) GetCommitsBetween(ctx context.Context, from, to string) ([]*sourcecontrol.Commit, error) {
	return m.Commits, nil
}

func (m *MemoryBenchMockGitRepo) GetCommitDiffStats(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.DiffStats, error) {
	return &sourcecontrol.DiffStats{
		Files: []sourcecontrol.FileStats{
			{Path: "internal/service/release/analyzer.go", Additions: 10, Deletions: 5},
		},
		Additions: 10,
		Deletions: 5,
	}, nil
}

func (m *MemoryBenchMockGitRepo) GetBatchCommitDiffStats(ctx context.Context, hashes []sourcecontrol.CommitHash) (map[sourcecontrol.CommitHash]*sourcecontrol.DiffStats, error) {
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

func (m *MemoryBenchMockGitRepo) GetRemotes(ctx context.Context) ([]sourcecontrol.RemoteInfo, error) {
	return nil, nil
}
func (m *MemoryBenchMockGitRepo) GetBranches(ctx context.Context) ([]sourcecontrol.BranchInfo, error) {
	return nil, nil
}
func (m *MemoryBenchMockGitRepo) GetCurrentBranch(ctx context.Context) (string, error) {
	return "main", nil
}
func (m *MemoryBenchMockGitRepo) GetCommit(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.Commit, error) {
	return nil, nil
}
func (m *MemoryBenchMockGitRepo) GetCommitsSince(ctx context.Context, ref string) ([]*sourcecontrol.Commit, error) {
	return nil, nil
}
func (m *MemoryBenchMockGitRepo) GetLatestCommit(ctx context.Context, branch string) (*sourcecontrol.Commit, error) {
	return nil, nil
}
func (m *MemoryBenchMockGitRepo) GetCommitPatch(ctx context.Context, hash sourcecontrol.CommitHash) (string, error) {
	return "", nil
}
func (m *MemoryBenchMockGitRepo) GetFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	return nil, nil
}
func (m *MemoryBenchMockGitRepo) GetTag(ctx context.Context, name string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (m *MemoryBenchMockGitRepo) GetLatestVersionTag(ctx context.Context, prefix string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (m *MemoryBenchMockGitRepo) CreateTag(ctx context.Context, name string, hash sourcecontrol.CommitHash, message string) (*sourcecontrol.Tag, error) {
	return nil, nil
}
func (m *MemoryBenchMockGitRepo) DeleteTag(ctx context.Context, name string) error { return nil }
func (m *MemoryBenchMockGitRepo) PushTag(ctx context.Context, name string, remote string) error {
	return nil
}
func (m *MemoryBenchMockGitRepo) IsDirty(ctx context.Context) (bool, error) { return false, nil }
func (m *MemoryBenchMockGitRepo) GetStatus(ctx context.Context) (*sourcecontrol.WorkingTreeStatus, error) {
	return &sourcecontrol.WorkingTreeStatus{IsClean: true}, nil
}
func (m *MemoryBenchMockGitRepo) Fetch(ctx context.Context, remote string) error        { return nil }
func (m *MemoryBenchMockGitRepo) Pull(ctx context.Context, remote, branch string) error { return nil }
func (m *MemoryBenchMockGitRepo) Push(ctx context.Context, remote, branch string) error { return nil }
