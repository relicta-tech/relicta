// Package analysis provides commit classification benchmarks.
package analysis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
)

// BenchmarkAnalyzeAll_Parallel benchmarks parallel vs sequential commit analysis.
// This validates Phase 2.4 of the performance optimization plan.
func BenchmarkAnalyzeAll_Parallel(b *testing.B) {
	b.ReportAllocs()

	benchCases := []struct {
		name        string
		commitCount int
		concurrency int // 1 = sequential, >1 = parallel
	}{
		{"100_commits_seq", 100, 1},
		{"100_commits_parallel", 100, 0}, // 0 = NumCPU
		{"500_commits_seq", 500, 1},
		{"500_commits_parallel", 500, 0},
		{"1000_commits_seq", 1000, 1},
		{"1000_commits_parallel", 1000, 0},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			commits := generateBenchmarkCommits(bc.commitCount)
			cfg := DefaultConfig()
			cfg.Concurrency = bc.concurrency
			cfg.EnableAI = false // Disable AI for consistent benchmark

			analyzer := NewAnalyzer(cfg,
				WithHeuristics(&benchmarkHeuristics{}),
			)

			ctx := context.Background()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := analyzer.AnalyzeAll(ctx, commits)
				if err != nil {
					b.Fatalf("AnalyzeAll failed: %v", err)
				}
			}

			// Report commits per second
			b.ReportMetric(float64(bc.commitCount), "commits/op")
		})
	}
}

// BenchmarkAnalyzeAll_Scalability tests how analysis scales with commit count.
func BenchmarkAnalyzeAll_Scalability(b *testing.B) {
	b.ReportAllocs()

	commitCounts := []int{10, 50, 100, 250, 500, 750, 1000}

	for _, count := range commitCounts {
		b.Run(fmt.Sprintf("%d_commits", count), func(b *testing.B) {
			commits := generateBenchmarkCommits(count)
			cfg := DefaultConfig()
			cfg.EnableAI = false

			analyzer := NewAnalyzer(cfg,
				WithHeuristics(&benchmarkHeuristics{}),
			)

			ctx := context.Background()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := analyzer.AnalyzeAll(ctx, commits)
				if err != nil {
					b.Fatalf("AnalyzeAll failed: %v", err)
				}
			}

			b.ReportMetric(float64(count), "commits/op")
		})
	}
}

// BenchmarkAnalyze_Single benchmarks single commit classification.
func BenchmarkAnalyze_Single(b *testing.B) {
	b.ReportAllocs()

	benchCases := []struct {
		name    string
		message string
	}{
		{"conventional", "feat(api): add user authentication"},
		{"heuristic_feat", "Add support for new database driver"},
		{"heuristic_fix", "Fix memory leak in connection pool"},
		{"non_conventional", "Updated README with new examples"},
	}

	cfg := DefaultConfig()
	cfg.EnableAI = false

	analyzer := NewAnalyzer(cfg,
		WithHeuristics(&benchmarkHeuristics{}),
	)

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			commit := CommitInfo{
				Hash:    sourcecontrol.CommitHash("abc123def456789"),
				Message: bc.message,
				Subject: bc.message,
			}

			ctx := context.Background()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := analyzer.Analyze(ctx, commit)
				if err != nil {
					b.Fatalf("Analyze failed: %v", err)
				}
			}
		})
	}
}

// generateBenchmarkCommits generates test commits for benchmarking.
func generateBenchmarkCommits(count int) []CommitInfo {
	commits := make([]CommitInfo, count)
	messages := []string{
		"feat(api): add new endpoint for user management",
		"fix(core): resolve race condition in worker pool",
		"docs: update API documentation",
		"refactor: extract common logic to shared package",
		"test: add integration tests for auth flow",
		"chore: update dependencies to latest versions",
		"perf: optimize database query performance",
		"Add support for new feature", // non-conventional
		"Fixed bug in authentication", // non-conventional
		"Updated configuration files", // non-conventional
	}

	for i := 0; i < count; i++ {
		message := messages[i%len(messages)]
		commits[i] = CommitInfo{
			Hash:    sourcecontrol.CommitHash(fmt.Sprintf("%040x", i)),
			Message: message,
			Subject: message,
			Files:   []string{"internal/service/example.go"},
			Stats: DiffStats{
				Additions:    10,
				Deletions:    5,
				FilesChanged: 1,
			},
		}
	}

	return commits
}

// benchmarkHeuristics provides a lightweight heuristics analyzer for benchmarks.
type benchmarkHeuristics struct{}

func (h *benchmarkHeuristics) Classify(commit CommitInfo) *CommitClassification {
	// Simple keyword-based classification for benchmark purposes
	message := commit.Message

	// Check for common patterns
	if containsKeyword(message, []string{"add", "implement", "introduce", "new"}) {
		return &CommitClassification{
			CommitHash: commit.Hash,
			Type:       "feat",
			Confidence: 0.75,
			Method:     MethodHeuristic,
			Reasoning:  "keyword match: feature",
		}
	}

	if containsKeyword(message, []string{"fix", "resolve", "correct", "repair"}) {
		return &CommitClassification{
			CommitHash: commit.Hash,
			Type:       "fix",
			Confidence: 0.75,
			Method:     MethodHeuristic,
			Reasoning:  "keyword match: fix",
		}
	}

	if containsKeyword(message, []string{"update", "change", "modify"}) {
		return &CommitClassification{
			CommitHash: commit.Hash,
			Type:       "chore",
			Confidence: 0.5,
			Method:     MethodHeuristic,
			Reasoning:  "keyword match: update",
		}
	}

	return &CommitClassification{
		CommitHash: commit.Hash,
		Type:       "chore",
		Confidence: 0.3,
		Method:     MethodHeuristic,
		Reasoning:  "default classification",
	}
}

func containsKeyword(message string, keywords []string) bool {
	for _, kw := range keywords {
		if containsIgnoreCase(message, kw) {
			return true
		}
	}
	return false
}

func containsIgnoreCase(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1 := s[i+j]
			c2 := substr[j]
			// Simple case-insensitive check for ASCII
			if c1 != c2 && c1 != c2+32 && c1 != c2-32 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// BenchmarkAnalyzeAll_ParallelVsSequential directly compares performance.
func BenchmarkAnalyzeAll_ParallelVsSequential(b *testing.B) {
	commits := generateBenchmarkCommits(500)
	ctx := context.Background()

	b.Run("sequential", func(b *testing.B) {
		b.ReportAllocs()
		cfg := DefaultConfig()
		cfg.Concurrency = 1
		cfg.EnableAI = false

		analyzer := NewAnalyzer(cfg,
			WithHeuristics(&benchmarkHeuristics{}),
		)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := analyzer.AnalyzeAll(ctx, commits)
			if err != nil {
				b.Fatalf("AnalyzeAll failed: %v", err)
			}
		}
	})

	b.Run("parallel", func(b *testing.B) {
		b.ReportAllocs()
		cfg := DefaultConfig()
		cfg.Concurrency = 0 // Use NumCPU
		cfg.EnableAI = false

		analyzer := NewAnalyzer(cfg,
			WithHeuristics(&benchmarkHeuristics{}),
		)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := analyzer.AnalyzeAll(ctx, commits)
			if err != nil {
				b.Fatalf("AnalyzeAll failed: %v", err)
			}
		}
	})
}

// BenchmarkAnalyzeAll_Concurrent tests concurrent access to the analyzer.
func BenchmarkAnalyzeAll_Concurrent(b *testing.B) {
	b.ReportAllocs()

	commits := generateBenchmarkCommits(100)
	cfg := DefaultConfig()
	cfg.EnableAI = false

	analyzer := NewAnalyzer(cfg,
		WithHeuristics(&benchmarkHeuristics{}),
	)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := analyzer.AnalyzeAll(ctx, commits)
			if err != nil {
				b.Fatalf("AnalyzeAll failed: %v", err)
			}
		}
	})
}

// BenchmarkClassifyMethod_Stats benchmarks statistics aggregation.
func BenchmarkClassifyMethod_Stats(b *testing.B) {
	b.ReportAllocs()

	commits := generateBenchmarkCommits(1000)
	cfg := DefaultConfig()
	cfg.EnableAI = false

	analyzer := NewAnalyzer(cfg,
		WithHeuristics(&benchmarkHeuristics{}),
	)

	ctx := context.Background()

	// Pre-warm the cache
	result, _ := analyzer.AnalyzeAll(ctx, commits)
	_ = result

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := analyzer.AnalyzeAll(ctx, commits)
		if err != nil {
			b.Fatalf("AnalyzeAll failed: %v", err)
		}
		// Access stats to ensure they're computed
		_ = result.Stats.AverageConfidence
		_ = result.Stats.MethodBreakdown
	}
}

// BenchmarkCommitInfo_Creation benchmarks commit info creation overhead.
func BenchmarkCommitInfo_Creation(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = CommitInfo{
			Hash:    sourcecontrol.CommitHash(fmt.Sprintf("%040x", i)),
			Message: "feat(api): add new endpoint",
			Subject: "feat(api): add new endpoint",
			Files:   []string{"internal/service/example.go"},
			Stats: DiffStats{
				Additions:    10,
				Deletions:    5,
				FilesChanged: 1,
			},
		}
	}
}

// BenchmarkAnalyzeAll_WithSlowWork simulates real-world scenarios with I/O delays.
func BenchmarkAnalyzeAll_WithSlowWork(b *testing.B) {
	benchCases := []struct {
		name        string
		commitCount int
		concurrency int
	}{
		{"50_commits_seq", 50, 1},
		{"50_commits_parallel", 50, 0},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			commits := generateBenchmarkCommits(bc.commitCount)
			cfg := DefaultConfig()
			cfg.Concurrency = bc.concurrency
			cfg.EnableAI = false

			analyzer := NewAnalyzer(cfg,
				WithHeuristics(&slowBenchmarkHeuristics{}),
			)

			ctx := context.Background()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := analyzer.AnalyzeAll(ctx, commits)
				if err != nil {
					b.Fatalf("AnalyzeAll failed: %v", err)
				}
			}

			b.ReportMetric(float64(bc.commitCount), "commits/op")
		})
	}
}

// slowBenchmarkHeuristics simulates I/O-bound classification work.
type slowBenchmarkHeuristics struct{}

func (h *slowBenchmarkHeuristics) Classify(commit CommitInfo) *CommitClassification {
	// Simulate minimal I/O delay (100µs) to test parallelization benefits
	time.Sleep(100 * time.Microsecond)

	return &CommitClassification{
		CommitHash: commit.Hash,
		Type:       "feat",
		Confidence: 0.8,
		Method:     MethodHeuristic,
		Reasoning:  "simulated slow classification",
	}
}
