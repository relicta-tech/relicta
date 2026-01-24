// Package benchmark provides profiling utilities for performance analysis.
package benchmark

import (
	"context"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"testing"

	analysisfactory "github.com/relicta-tech/relicta/internal/analysis/factory"
	"github.com/relicta-tech/relicta/internal/service/release"
)

// ProfileConfig contains configuration for profiling tests.
type ProfileConfig struct {
	// CPUProfile specifies the path for CPU profile output.
	CPUProfile string
	// MemProfile specifies the path for memory profile output.
	MemProfile string
	// CommitCount specifies the number of commits to generate.
	CommitCount int
}

// DefaultProfileConfig returns the default profiling configuration.
func DefaultProfileConfig() ProfileConfig {
	return ProfileConfig{
		CPUProfile:  "cpu.prof",
		MemProfile:  "mem.prof",
		CommitCount: 1000,
	}
}

// TestProfile_AnalyzePipeline is a test that can be run with profiling.
// Run with: go test -run=TestProfile_AnalyzePipeline -cpuprofile=cpu.prof -memprofile=mem.prof
func TestProfile_AnalyzePipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping profile test in short mode")
	}

	cfg := DefaultProfileConfig()

	// Check environment for custom commit count
	if envCount := os.Getenv("BENCH_COMMIT_COUNT"); envCount != "" {
		if count, err := strconv.Atoi(envCount); err == nil && count > 0 {
			cfg.CommitCount = count
		}
	}

	gitRepo := NewMockGitRepo(cfg.CommitCount)
	factory := analysisfactory.NewFactory(nil)
	versionCalc := NewMockVersionCalc()

	analyzer := release.NewAnalyzer(gitRepo, versionCalc, factory)
	input := release.AnalyzeInput{
		RepositoryPath: "/test/repo",
		Branch:         "main",
		TagPrefix:      "v",
	}

	ctx := context.Background()

	// Run the analysis multiple times to get meaningful profile data
	for i := 0; i < 10; i++ {
		_, err := analyzer.Analyze(ctx, input)
		if err != nil {
			t.Fatalf("Analyze failed: %v", err)
		}
	}
}

// RunWithProfiling executes a function with CPU and memory profiling.
// Usage:
//
//	RunWithProfiling("cpu.prof", "mem.prof", func() {
//	    // Your code here
//	})
func RunWithProfiling(cpuProfilePath, memProfilePath string, fn func()) error {
	// CPU profiling
	if cpuProfilePath != "" {
		f, err := os.Create(cpuProfilePath)
		if err != nil {
			return err
		}
		defer f.Close()

		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}

	// Run the function
	fn()

	// Memory profiling
	if memProfilePath != "" {
		f, err := os.Create(memProfilePath)
		if err != nil {
			return err
		}
		defer f.Close()

		runtime.GC() // Get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			return err
		}
	}

	return nil
}

// TestProfile_MemoryAllocation profiles memory allocations in the analysis pipeline.
func TestProfile_MemoryAllocation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping profile test in short mode")
	}

	gitRepo := NewMockGitRepo(1000)
	factory := analysisfactory.NewFactory(nil)
	versionCalc := NewMockVersionCalc()

	analyzer := release.NewAnalyzer(gitRepo, versionCalc, factory)
	input := release.AnalyzeInput{
		RepositoryPath: "/test/repo",
		Branch:         "main",
		TagPrefix:      "v",
	}

	ctx := context.Background()

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	for i := 0; i < 10; i++ {
		_, err := analyzer.Analyze(ctx, input)
		if err != nil {
			t.Fatalf("Analyze failed: %v", err)
		}
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	allocatedBytes := m2.TotalAlloc - m1.TotalAlloc
	allocatedMB := float64(allocatedBytes) / 1024 / 1024

	t.Logf("Total allocated: %.2f MB", allocatedMB)
	t.Logf("Heap in use: %.2f MB", float64(m2.HeapInuse)/1024/1024)
	t.Logf("Number of allocations: %d", m2.Mallocs-m1.Mallocs)
	t.Logf("GC cycles: %d", m2.NumGC-m1.NumGC)

	// Warn if memory usage is too high
	if allocatedMB > 100 {
		t.Logf("WARNING: High memory usage detected (>100MB for 10 iterations)")
	}
}

// BenchmarkProfile_GCPressure measures GC pressure during analysis.
func BenchmarkProfile_GCPressure(b *testing.B) {
	b.ReportAllocs()

	gitRepo := NewMockGitRepo(500)
	factory := analysisfactory.NewFactory(nil)
	versionCalc := NewMockVersionCalc()

	analyzer := release.NewAnalyzer(gitRepo, versionCalc, factory)
	input := release.AnalyzeInput{
		RepositoryPath: "/test/repo",
		Branch:         "main",
		TagPrefix:      "v",
	}

	ctx := context.Background()

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := analyzer.Analyze(ctx, input)
		if err != nil {
			b.Fatalf("Analyze failed: %v", err)
		}
	}

	b.StopTimer()
	runtime.ReadMemStats(&m2)

	// Report GC metrics
	gcCycles := m2.NumGC - m1.NumGC
	if gcCycles > 0 {
		b.ReportMetric(float64(gcCycles), "gc-cycles")
		b.ReportMetric(float64(m2.PauseTotalNs-m1.PauseTotalNs)/float64(b.N), "gc-pause-ns/op")
	}
}

// BenchmarkProfile_Throughput measures commit processing throughput.
func BenchmarkProfile_Throughput(b *testing.B) {
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
				_, err := analyzer.Analyze(ctx, input)
				if err != nil {
					b.Fatalf("Analyze failed: %v", err)
				}
			}

			// Report commits per second
			b.ReportMetric(float64(bc.commitCount), "commits/op")
		})
	}
}

// BenchmarkProfile_Scalability tests how performance scales with commit count.
func BenchmarkProfile_Scalability(b *testing.B) {
	b.ReportAllocs()

	commitCounts := []int{10, 50, 100, 250, 500, 750, 1000}

	for _, count := range commitCounts {
		b.Run("commits_"+string(rune('0'+count/100))+"xx", func(b *testing.B) {
			gitRepo := NewMockGitRepo(count)
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
				_, err := analyzer.Analyze(ctx, input)
				if err != nil {
					b.Fatalf("Analyze failed: %v", err)
				}
			}
		})
	}
}
