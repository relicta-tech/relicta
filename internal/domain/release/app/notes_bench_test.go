// Package app provides benchmark tests for release application services.
package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// BenchmarkGenerateNotesUseCase_Execute benchmarks notes generation.
// Target: < 500ms for notes generation without AI.
func BenchmarkGenerateNotesUseCase_Execute(b *testing.B) {
	b.ReportAllocs()

	benchCases := []struct {
		name        string
		commitCount int
	}{
		{"10_commits", 10},
		{"50_commits", 50},
		{"100_commits", 100},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			// Create a run in the correct state for notes generation
			run := createBenchRun(bc.commitCount)

			// Create mocks
			repo := &benchMockRepository{runs: make(map[domain.RunID]*domain.ReleaseRun)}
			repo.runs[run.ID()] = run

			repoInspector := &benchMockRepoInspector{headSHA: run.HeadSHA()}
			notesGen := &benchMockNotesGenerator{commitCount: bc.commitCount}
			stateMachine, err := domain.NewStateMachineService()
			if err != nil {
				b.Fatalf("failed to create state machine: %v", err)
			}

			uc := NewGenerateNotesUseCase(repo, repoInspector, notesGen, stateMachine)

			ctx := context.Background()
			input := GenerateNotesInput{
				RepoRoot: run.RepoRoot(),
				RunID:    run.ID(),
				Options:  ports.NotesOptions{UseAI: false},
				Actor:    ports.ActorInfo{ID: "bench-user"},
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Reset the run state for each iteration
				run = createBenchRun(bc.commitCount)
				repo.runs[run.ID()] = run
				repo.latest = run

				_, err := uc.Execute(ctx, input)
				if err != nil {
					b.Fatalf("Execute failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkNotesInputsHash benchmarks the hash computation for notes inputs.
func BenchmarkNotesInputsHash(b *testing.B) {
	b.ReportAllocs()

	benchCases := []struct {
		name        string
		commitCount int
	}{
		{"10_commits", 10},
		{"50_commits", 50},
		{"100_commits", 100},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			run := createBenchRun(bc.commitCount)
			notesGen := &benchMockNotesGenerator{commitCount: bc.commitCount}
			options := ports.NotesOptions{
				UseAI:          false,
				AudiencePreset: "developer",
				TonePreset:     "formal",
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = notesGen.ComputeInputsHash(run, options)
			}
		})
	}
}

// createBenchRun creates a run in StateVersioned state with the given number of commits.
func createBenchRun(commitCount int) *domain.ReleaseRun {
	commits := make([]domain.CommitSHA, commitCount)
	for i := 0; i < commitCount; i++ {
		commits[i] = domain.CommitSHA(fmt.Sprintf("commit%d", i))
	}

	v1, _ := version.Parse("1.0.0")
	v2, _ := version.Parse("1.1.0")

	run := domain.NewReleaseRun(
		"bench-repo",
		"/bench/repo",
		"v1.0.0",
		domain.CommitSHA("headsha123"),
		commits,
		"confighash",
		"pluginhash",
	)

	// Create a changeset with commits
	changeSet := changes.NewChangeSet(
		changes.ChangeSetID("bench-changeset"),
		"v1.0.0",
		"HEAD",
	)
	for i := 0; i < commitCount; i++ {
		commit := changes.NewConventionalCommit(
			fmt.Sprintf("commit%d", i),
			changes.CommitTypeFeat,
			fmt.Sprintf("add feature %d", i),
			changes.WithScope("core"),
		)
		changeSet.AddCommit(commit)
	}
	run.SetChangeSet(changeSet)

	// Progress through states to reach StateVersioned
	_ = run.SetVersionProposal(v1, v2, domain.BumpMinor, 0.95)
	_ = run.Plan("bench-user")
	_ = run.SetVersion(v2, "v1.1.0")
	_ = run.Bump("bench-user")

	return run
}

// benchMockRepository is a mock repository for benchmarking.
type benchMockRepository struct {
	runs   map[domain.RunID]*domain.ReleaseRun
	latest *domain.ReleaseRun
}

func (r *benchMockRepository) Load(ctx context.Context, runID domain.RunID) (*domain.ReleaseRun, error) {
	if run, ok := r.runs[runID]; ok {
		return run, nil
	}
	return nil, fmt.Errorf("run not found: %s", runID)
}

func (r *benchMockRepository) LoadBatch(ctx context.Context, repoRoot string, runIDs []domain.RunID) (map[domain.RunID]*domain.ReleaseRun, error) {
	result := make(map[domain.RunID]*domain.ReleaseRun)
	for _, id := range runIDs {
		if run, ok := r.runs[id]; ok {
			result[id] = run
		}
	}
	return result, nil
}

func (r *benchMockRepository) LoadLatest(ctx context.Context, repoRoot string) (*domain.ReleaseRun, error) {
	if r.latest != nil {
		return r.latest, nil
	}
	return nil, fmt.Errorf("no latest run")
}

func (r *benchMockRepository) List(ctx context.Context, repoRoot string) ([]domain.RunID, error) {
	ids := make([]domain.RunID, 0, len(r.runs))
	for id := range r.runs {
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *benchMockRepository) Save(ctx context.Context, run *domain.ReleaseRun) error {
	r.runs[run.ID()] = run
	r.latest = run
	return nil
}

func (r *benchMockRepository) SetLatest(ctx context.Context, repoRoot string, runID domain.RunID) error {
	if run, ok := r.runs[runID]; ok {
		r.latest = run
	}
	return nil
}

func (r *benchMockRepository) Delete(ctx context.Context, runID domain.RunID) error {
	delete(r.runs, runID)
	return nil
}

func (r *benchMockRepository) FindByState(ctx context.Context, repoRoot string, state domain.RunState) ([]*domain.ReleaseRun, error) {
	var result []*domain.ReleaseRun
	for _, run := range r.runs {
		if run.State() == state {
			result = append(result, run)
		}
	}
	return result, nil
}

func (r *benchMockRepository) FindActive(ctx context.Context, repoRoot string) ([]*domain.ReleaseRun, error) {
	return nil, nil
}

func (r *benchMockRepository) FindByPlanHash(ctx context.Context, repoRoot string, planHash string) (*domain.ReleaseRun, error) {
	for _, run := range r.runs {
		if run.PlanHash() == planHash {
			return run, nil
		}
	}
	return nil, nil
}

// benchMockRepoInspector is a mock repo inspector for benchmarking.
type benchMockRepoInspector struct {
	headSHA domain.CommitSHA
}

func (m *benchMockRepoInspector) HeadSHA(ctx context.Context) (domain.CommitSHA, error) {
	return m.headSHA, nil
}

func (m *benchMockRepoInspector) IsClean(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *benchMockRepoInspector) ResolveCommits(ctx context.Context, baseRef string, headSHA domain.CommitSHA) ([]domain.CommitSHA, error) {
	return []domain.CommitSHA{headSHA}, nil
}

func (m *benchMockRepoInspector) GetRemoteURL(ctx context.Context) (string, error) {
	return "https://github.com/test/bench-repo", nil
}

func (m *benchMockRepoInspector) GetCurrentBranch(ctx context.Context) (string, error) {
	return "main", nil
}

func (m *benchMockRepoInspector) GetLatestVersionTag(ctx context.Context, prefix string) (string, error) {
	return "v1.0.0", nil
}

func (m *benchMockRepoInspector) TagExists(ctx context.Context, tagName string) (bool, error) {
	return false, nil
}

func (m *benchMockRepoInspector) ReleaseExists(ctx context.Context, tagName string) (bool, error) {
	return false, nil
}

// benchMockNotesGenerator is a mock notes generator for benchmarking.
type benchMockNotesGenerator struct {
	commitCount int
}

func (m *benchMockNotesGenerator) Generate(ctx context.Context, run *domain.ReleaseRun, options ports.NotesOptions) (*domain.ReleaseNotes, error) {
	// Simulate some work by building release notes
	var sb strings.Builder
	sb.WriteString("# Release Notes\n\n")
	sb.WriteString("## Features\n\n")

	// Generate notes based on commit count
	for i := 0; i < m.commitCount; i++ {
		sb.WriteString(fmt.Sprintf("- Feature %d: Description of feature %d\n", i, i))
	}

	return &domain.ReleaseNotes{
		Text:           sb.String(),
		AudiencePreset: options.AudiencePreset,
		TonePreset:     options.TonePreset,
		Provider:       "mock",
		Model:          "benchmark",
		GeneratedAt:    time.Now(),
	}, nil
}

func (m *benchMockNotesGenerator) ComputeInputsHash(run *domain.ReleaseRun, options ports.NotesOptions) string {
	// Simple hash computation
	var sb strings.Builder
	sb.WriteString(run.ID().String())
	sb.WriteString(run.VersionNext().String())
	sb.WriteString(options.AudiencePreset)
	sb.WriteString(options.TonePreset)

	cs := run.ChangeSet()
	if cs != nil {
		for _, c := range cs.Commits() {
			sb.WriteString(c.Hash())
		}
	}

	return fmt.Sprintf("hash_%d", sb.Len())
}
