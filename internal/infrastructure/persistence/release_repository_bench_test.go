package persistence

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// ============================================================================
// Repository Creation Benchmarks
// ============================================================================

// BenchmarkRepository_Creation measures repository creation overhead.
func BenchmarkRepository_Creation(b *testing.B) {
	b.ReportAllocs()

	tmpDir := b.TempDir()

	b.Run("new_repository", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			dir := filepath.Join(tmpDir, fmt.Sprintf("repo-%d", i))
			repo, err := NewFileReleaseRepository(dir)
			if err != nil {
				b.Fatal(err)
			}
			_ = repo
		}
	})

	b.Run("existing_directory", func(b *testing.B) {
		existingDir := filepath.Join(tmpDir, "existing")
		if err := os.MkdirAll(existingDir, 0700); err != nil {
			b.Fatal(err)
		}
		for i := 0; i < b.N; i++ {
			repo, err := NewFileReleaseRepository(existingDir)
			if err != nil {
				b.Fatal(err)
			}
			_ = repo
		}
	})
}

// ============================================================================
// DTO Serialization Benchmarks
// ============================================================================

// BenchmarkDTO_Serialization measures DTO conversion overhead.
func BenchmarkDTO_Serialization(b *testing.B) {
	b.ReportAllocs()

	tmpDir := b.TempDir()
	repo, err := NewFileReleaseRepository(tmpDir)
	if err != nil {
		b.Fatal(err)
	}

	createRelease := func(numCommits int) *release.ReleaseRun {
		// Create commit SHAs (not ConventionalCommit objects)
		commits := make([]release.CommitSHA, numCommits)
		for i := 0; i < numCommits; i++ {
			commits[i] = release.CommitSHA(fmt.Sprintf("abc%04d", i))
		}

		rel := release.NewReleaseRun(
			"/path/to/repo",
			"/path/to/repo",
			"main",
			release.CommitSHA("abc123"),
			commits,
			"config-hash",
			"plugin-hash",
		)

		// Set up plan using proper state transitions
		currentVer := version.NewSemanticVersion(1, 0, 0)
		nextVer := version.NewSemanticVersion(1, 1, 0)

		// SetVersionProposal in Draft state
		_ = rel.SetVersionProposal(currentVer, nextVer, release.BumpMinor, 0.9)
		// Transition to Planned
		_ = rel.Plan("benchmark")
		// Set version in Planned state
		_ = rel.SetVersion(nextVer, "v1.1.0")
		// Transition to Versioned
		_ = rel.Bump("benchmark")

		// Generate notes to advance to NotesReady state
		notes := &release.ReleaseNotes{
			Text:           "## What's Changed\n\n- Added new features",
			AudiencePreset: "developers",
			TonePreset:     "professional",
			Provider:       "openai",
			Model:          "gpt-4",
			GeneratedAt:    time.Now(),
		}
		_ = rel.GenerateNotes(notes, "notes-hash", "benchmark")

		return rel
	}

	b.Run("to_dto_minimal", func(b *testing.B) {
		rel := release.NewReleaseRun("/path", "/path", "main", release.CommitSHA("abc"), nil, "", "")
		// Set version proposal to avoid empty release type parsing errors
		_ = rel.SetVersionProposal(version.NewSemanticVersion(1, 0, 0), version.NewSemanticVersion(1, 0, 1), release.BumpPatch, 0.9)
		for i := 0; i < b.N; i++ {
			_ = repo.toDTO(rel)
		}
	})

	b.Run("to_dto_with_plan", func(b *testing.B) {
		rel := createRelease(10)
		for i := 0; i < b.N; i++ {
			_ = repo.toDTO(rel)
		}
	})

	b.Run("to_dto_large", func(b *testing.B) {
		rel := createRelease(100)
		for i := 0; i < b.N; i++ {
			_ = repo.toDTO(rel)
		}
	})

	b.Run("from_dto_minimal", func(b *testing.B) {
		rel := release.NewReleaseRun("/path", "/path", "main", release.CommitSHA("abc"), nil, "", "")
		// Set version proposal to avoid empty release type parsing errors
		_ = rel.SetVersionProposal(version.NewSemanticVersion(1, 0, 0), version.NewSemanticVersion(1, 0, 1), release.BumpPatch, 0.9)
		dto := repo.toDTO(rel)
		for i := 0; i < b.N; i++ {
			_, err := repo.fromDTO(dto)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("from_dto_with_plan", func(b *testing.B) {
		rel := createRelease(10)
		dto := repo.toDTO(rel)
		for i := 0; i < b.N; i++ {
			_, err := repo.fromDTO(dto)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("from_dto_large", func(b *testing.B) {
		rel := createRelease(100)
		dto := repo.toDTO(rel)
		for i := 0; i < b.N; i++ {
			_, err := repo.fromDTO(dto)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// ============================================================================
// File Operations Benchmarks
// ============================================================================

// BenchmarkRepository_Save measures save operation overhead.
func BenchmarkRepository_Save(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	tmpDir := b.TempDir()
	repo, err := NewFileReleaseRepository(tmpDir)
	if err != nil {
		b.Fatal(err)
	}

	createRelease := func(idx int, numCommits int) *release.ReleaseRun {
		commits := make([]release.CommitSHA, numCommits)
		for i := 0; i < numCommits; i++ {
			commits[i] = release.CommitSHA(fmt.Sprintf("hash%04d%04d", idx, i))
		}

		rel := release.NewReleaseRun(
			"/path/to/repo",
			"/path/to/repo",
			"main",
			release.CommitSHA(fmt.Sprintf("head%d", idx)),
			commits,
			"config",
			"plugin",
		)

		currentVer := version.NewSemanticVersion(1, 0, 0)
		nextVer := version.NewSemanticVersion(1, 1, 0)
		_ = rel.SetVersionProposal(currentVer, nextVer, release.BumpMinor, 0.9)
		_ = rel.Plan("benchmark")

		return rel
	}

	b.Run("save_small", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rel := createRelease(i, 5)
			if err := repo.Save(ctx, rel); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("save_medium", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rel := createRelease(i+10000, 50)
			if err := repo.Save(ctx, rel); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("save_large", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rel := createRelease(i+20000, 200)
			if err := repo.Save(ctx, rel); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkRepository_FindByID measures find operation overhead.
func BenchmarkRepository_FindByID(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	tmpDir := b.TempDir()
	repo, err := NewFileReleaseRepository(tmpDir)
	if err != nil {
		b.Fatal(err)
	}

	// Create releases for lookup
	var releaseID release.RunID
	for i := 0; i < 100; i++ {
		commits := make([]release.CommitSHA, 20)
		for j := 0; j < 20; j++ {
			commits[j] = release.CommitSHA(fmt.Sprintf("hash%04d%04d", i, j))
		}

		rel := release.NewReleaseRun(
			"/path/to/repo",
			"/path/to/repo",
			"main",
			release.CommitSHA(fmt.Sprintf("head%d", i)),
			commits,
			"config",
			"plugin",
		)

		currentVer := version.NewSemanticVersion(1, 0, 0)
		nextVer := version.NewSemanticVersion(1, 1, 0)
		_ = rel.SetVersionProposal(currentVer, nextVer, release.BumpMinor, 0.9)
		_ = rel.Plan("benchmark")

		if err := repo.Save(ctx, rel); err != nil {
			b.Fatal(err)
		}
		if i == 50 {
			releaseID = rel.ID() // Save middle one for lookup
		}
	}

	b.ResetTimer()

	b.Run("find_existing", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := repo.FindByID(ctx, releaseID)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("find_not_found", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = repo.FindByID(ctx, "nonexistent-id")
		}
	})
}

// ============================================================================
// Scanning Benchmarks
// ============================================================================

// BenchmarkRepository_Scan measures scan operation overhead.
func BenchmarkRepository_Scan(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()

	setupRepo := func(numReleases int) (*FileReleaseRepository, string) {
		tmpDir := b.TempDir()
		repo, err := NewFileReleaseRepository(tmpDir)
		if err != nil {
			b.Fatal(err)
		}

		for i := 0; i < numReleases; i++ {
			commits := make([]release.CommitSHA, 10)
			for j := 0; j < 10; j++ {
				commits[j] = release.CommitSHA(fmt.Sprintf("hash%04d%04d", i, j))
			}

			repoPath := "/path/to/repo"
			if i%3 == 0 {
				repoPath = "/path/to/other-repo"
			}

			rel := release.NewReleaseRun(
				repoPath,
				repoPath,
				"main",
				release.CommitSHA(fmt.Sprintf("head%d", i)),
				commits,
				"config",
				"plugin",
			)

			currentVer := version.NewSemanticVersion(1, 0, 0)
			nextVer := version.NewSemanticVersion(1, 1, 0)
			_ = rel.SetVersionProposal(currentVer, nextVer, release.BumpMinor, 0.9)
			_ = rel.Plan("benchmark")

			if err := repo.Save(ctx, rel); err != nil {
				b.Fatal(err)
			}
		}

		return repo, "/path/to/repo"
	}

	b.Run("find_latest_10_releases", func(b *testing.B) {
		repo, repoPath := setupRepo(10)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := repo.FindLatest(ctx, repoPath)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("find_latest_50_releases", func(b *testing.B) {
		repo, repoPath := setupRepo(50)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := repo.FindLatest(ctx, repoPath)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("find_active_50_releases", func(b *testing.B) {
		repo, _ := setupRepo(50)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := repo.FindActive(ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("list_50_releases", func(b *testing.B) {
		repo, repoPath := setupRepo(50)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := repo.List(ctx, repoPath)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// ============================================================================
// Unit of Work Benchmarks
// ============================================================================

// BenchmarkUnitOfWork_Creation measures unit of work creation overhead.
func BenchmarkUnitOfWork_Creation(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	tmpDir := b.TempDir()
	repo, err := NewFileReleaseRepository(tmpDir)
	if err != nil {
		b.Fatal(err)
	}

	factory := NewFileUnitOfWorkFactory(repo, nil)

	b.Run("begin_transaction", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			uow, err := factory.Begin(ctx)
			if err != nil {
				b.Fatal(err)
			}
			_ = uow.Rollback()
		}
	})
}

// BenchmarkUnitOfWork_Operations measures unit of work operation overhead.
func BenchmarkUnitOfWork_Operations(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	tmpDir := b.TempDir()
	repo, err := NewFileReleaseRepository(tmpDir)
	if err != nil {
		b.Fatal(err)
	}

	factory := NewFileUnitOfWorkFactory(repo, nil)

	createRelease := func(idx int) *release.ReleaseRun {
		commits := make([]release.CommitSHA, 10)
		for i := 0; i < 10; i++ {
			commits[i] = release.CommitSHA(fmt.Sprintf("hash%04d%04d", idx, i))
		}

		rel := release.NewReleaseRun(
			"/path/to/repo",
			"/path/to/repo",
			"main",
			release.CommitSHA(fmt.Sprintf("head%d", idx)),
			commits,
			"config",
			"plugin",
		)

		// Set version proposal to ensure valid release type
		currentVer := version.NewSemanticVersion(1, 0, 0)
		nextVer := version.NewSemanticVersion(1, 1, 0)
		_ = rel.SetVersionProposal(currentVer, nextVer, release.BumpMinor, 0.9)
		_ = rel.Plan("benchmark")

		return rel
	}

	b.Run("save_commit", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			uow, err := factory.Begin(ctx)
			if err != nil {
				b.Fatal(err)
			}

			rel := createRelease(i)
			if err := uow.ReleaseRepository().Save(ctx, rel); err != nil {
				b.Fatal(err)
			}

			if err := uow.Commit(ctx); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("save_rollback", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			uow, err := factory.Begin(ctx)
			if err != nil {
				b.Fatal(err)
			}

			rel := createRelease(i + 100000)
			if err := uow.ReleaseRepository().Save(ctx, rel); err != nil {
				b.Fatal(err)
			}

			if err := uow.Rollback(); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("multiple_saves_commit", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			uow, err := factory.Begin(ctx)
			if err != nil {
				b.Fatal(err)
			}

			for j := 0; j < 5; j++ {
				rel := createRelease(i*5 + j + 200000)
				if err := uow.ReleaseRepository().Save(ctx, rel); err != nil {
					b.Fatal(err)
				}
			}

			if err := uow.Commit(ctx); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkUnitOfWork_FindOperations measures find operations within a transaction.
func BenchmarkUnitOfWork_FindOperations(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	tmpDir := b.TempDir()
	repo, err := NewFileReleaseRepository(tmpDir)
	if err != nil {
		b.Fatal(err)
	}

	// Pre-populate with some releases
	var releaseID release.RunID
	for i := 0; i < 20; i++ {
		commits := make([]release.CommitSHA, 10)
		for j := 0; j < 10; j++ {
			commits[j] = release.CommitSHA(fmt.Sprintf("hash%04d%04d", i, j))
		}

		rel := release.NewReleaseRun(
			"/path/to/repo",
			"/path/to/repo",
			"main",
			release.CommitSHA(fmt.Sprintf("head%d", i)),
			commits,
			"config",
			"plugin",
		)

		// Set version proposal to avoid empty release type parsing errors
		currentVer := version.NewSemanticVersion(1, 0, 0)
		nextVer := version.NewSemanticVersion(1, 1, 0)
		_ = rel.SetVersionProposal(currentVer, nextVer, release.BumpMinor, 0.9)
		_ = rel.Plan("benchmark")

		if err := repo.Save(ctx, rel); err != nil {
			b.Fatal(err)
		}
		if i == 10 {
			releaseID = rel.ID()
		}
	}

	factory := NewFileUnitOfWorkFactory(repo, nil)

	b.Run("find_by_id_in_transaction", func(b *testing.B) {
		uow, err := factory.Begin(ctx)
		if err != nil {
			b.Fatal(err)
		}
		defer func() { _ = uow.Rollback() }()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := uow.ReleaseRepository().FindByID(ctx, releaseID)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("find_latest_in_transaction", func(b *testing.B) {
		uow, err := factory.Begin(ctx)
		if err != nil {
			b.Fatal(err)
		}
		defer func() { _ = uow.Rollback() }()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := uow.ReleaseRepository().FindLatest(ctx, "/path/to/repo")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// ============================================================================
// Context Cancellation Benchmarks
// ============================================================================

// BenchmarkRepository_ContextCheck measures context check overhead.
func BenchmarkRepository_ContextCheck(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()

	b.Run("active_context", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = checkContext(ctx)
		}
	})

	b.Run("canceled_context", func(b *testing.B) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		for i := 0; i < b.N; i++ {
			_ = checkContext(cancelCtx)
		}
	})
}

// ============================================================================
// Concurrent Access Benchmarks
// ============================================================================

// BenchmarkRepository_ConcurrentSave measures concurrent save performance.
func BenchmarkRepository_ConcurrentSave(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	tmpDir := b.TempDir()
	repo, err := NewFileReleaseRepository(tmpDir)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("parallel_saves", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				commits := make([]release.CommitSHA, 5)
				for j := 0; j < 5; j++ {
					commits[j] = release.CommitSHA(fmt.Sprintf("parallel%d%d", i, j))
				}

				rel := release.NewReleaseRun(
					"/path/to/repo",
					"/path/to/repo",
					"main",
					release.CommitSHA(fmt.Sprintf("parallel-head%d", i)),
					commits,
					"config",
					"plugin",
				)

				// Set version proposal to ensure valid release type
				currentVer := version.NewSemanticVersion(1, 0, 0)
				nextVer := version.NewSemanticVersion(1, 1, 0)
				_ = rel.SetVersionProposal(currentVer, nextVer, release.BumpMinor, 0.9)
				_ = rel.Plan("benchmark")

				if err := repo.Save(ctx, rel); err != nil {
					b.Fatal(err)
				}
				i++
			}
		})
	})
}

// BenchmarkRepository_ConcurrentRead measures concurrent read performance.
func BenchmarkRepository_ConcurrentRead(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	tmpDir := b.TempDir()
	repo, err := NewFileReleaseRepository(tmpDir)
	if err != nil {
		b.Fatal(err)
	}

	// Pre-populate
	var releaseIDs []release.RunID
	for i := 0; i < 50; i++ {
		commits := make([]release.CommitSHA, 5)
		for j := 0; j < 5; j++ {
			commits[j] = release.CommitSHA(fmt.Sprintf("read%d%d", i, j))
		}

		rel := release.NewReleaseRun(
			"/path/to/repo",
			"/path/to/repo",
			"main",
			release.CommitSHA(fmt.Sprintf("read-head%d", i)),
			commits,
			"config",
			"plugin",
		)

		// Set version proposal to avoid empty release type parsing errors
		currentVer := version.NewSemanticVersion(1, 0, 0)
		nextVer := version.NewSemanticVersion(1, 1, 0)
		_ = rel.SetVersionProposal(currentVer, nextVer, release.BumpMinor, 0.9)
		_ = rel.Plan("benchmark")

		if err := repo.Save(ctx, rel); err != nil {
			b.Fatal(err)
		}
		releaseIDs = append(releaseIDs, rel.ID())
	}

	b.ResetTimer()

	b.Run("parallel_reads", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				_, err := repo.FindByID(ctx, releaseIDs[i%len(releaseIDs)])
				if err != nil {
					b.Fatal(err)
				}
				i++
			}
		})
	})
}

// ============================================================================
// Target Validation Benchmark
// ============================================================================

// BenchmarkPersistence_FullCycleOverhead validates persistence overhead target.
func BenchmarkPersistence_FullCycleOverhead(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	tmpDir := b.TempDir()
	repo, err := NewFileReleaseRepository(tmpDir)
	if err != nil {
		b.Fatal(err)
	}

	factory := NewFileUnitOfWorkFactory(repo, nil)

	b.Run("full_cycle_under_100ms", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			start := time.Now()

			// 1. Begin transaction
			uow, err := factory.Begin(ctx)
			if err != nil {
				b.Fatal(err)
			}

			// 2. Create release with commits
			commits := make([]release.CommitSHA, 50)
			for j := 0; j < 50; j++ {
				commits[j] = release.CommitSHA(fmt.Sprintf("target%d%d", i, j))
			}

			rel := release.NewReleaseRun(
				"/path/to/repo",
				"/path/to/repo",
				"main",
				release.CommitSHA(fmt.Sprintf("target-head%d", i)),
				commits,
				"config",
				"plugin",
			)

			// Set up plan using proper state machine transitions
			currentVer := version.NewSemanticVersion(1, 0, 0)
			nextVer := version.NewSemanticVersion(1, 1, 0)
			_ = rel.SetVersionProposal(currentVer, nextVer, release.BumpMinor, 0.9)
			_ = rel.Plan("benchmark")
			_ = rel.SetVersion(nextVer, "v1.1.0")
			_ = rel.Bump("benchmark")

			// Generate notes
			notes := &release.ReleaseNotes{
				Text:           "## What's Changed\n\n- Added new features\n- Fixed bugs",
				AudiencePreset: "developers",
				TonePreset:     "professional",
				Provider:       "openai",
				Model:          "gpt-4",
				GeneratedAt:    time.Now(),
			}
			_ = rel.GenerateNotes(notes, "notes-hash", "benchmark")

			// 3. Save
			if err := uow.ReleaseRepository().Save(ctx, rel); err != nil {
				b.Fatal(err)
			}

			// 4. Commit
			if err := uow.Commit(ctx); err != nil {
				b.Fatal(err)
			}

			// 5. Read back
			_, err = repo.FindByID(ctx, rel.ID())
			if err != nil {
				b.Fatal(err)
			}

			elapsed := time.Since(start)
			if elapsed > 100*time.Millisecond {
				b.Errorf("Full persistence cycle took %v, exceeds 100ms target", elapsed)
			}
		}
	})
}

// ============================================================================
// Changeset Storage Benchmarks
// ============================================================================

// BenchmarkRepository_ChangeSetStorage measures changeset storage overhead.
func BenchmarkRepository_ChangeSetStorage(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	tmpDir := b.TempDir()
	repo, err := NewFileReleaseRepository(tmpDir)
	if err != nil {
		b.Fatal(err)
	}

	createReleaseWithChangeset := func(idx int, numCommits int) *release.ReleaseRun {
		commits := make([]release.CommitSHA, numCommits)
		for i := 0; i < numCommits; i++ {
			commits[i] = release.CommitSHA(fmt.Sprintf("cs%04d%04d", idx, i))
		}

		rel := release.NewReleaseRun(
			"/path/to/repo",
			"/path/to/repo",
			"v1.0.0",
			release.CommitSHA(fmt.Sprintf("cshead%d", idx)),
			commits,
			"config",
			"plugin",
		)

		// Create a ChangeSet and attach it
		cs := changes.NewChangeSet(
			changes.ChangeSetID(fmt.Sprintf("changeset-%d", idx)),
			"v1.0.0",
			fmt.Sprintf("cshead%d", idx),
		)

		// Add conventional commits to the changeset
		for i := 0; i < numCommits; i++ {
			commit := changes.NewConventionalCommit(
				fmt.Sprintf("cs%04d%04d", idx, i),
				changes.CommitTypeFeat,
				fmt.Sprintf("Feature %d", i),
				changes.WithScope(fmt.Sprintf("mod%d", i%5)),
			)
			cs.AddCommit(commit)
		}

		rel.SetChangeSet(cs)

		// Set up plan
		currentVer := version.NewSemanticVersion(1, 0, 0)
		nextVer := version.NewSemanticVersion(1, 1, 0)
		_ = rel.SetVersionProposal(currentVer, nextVer, release.BumpMinor, 0.9)
		_ = rel.Plan("benchmark")

		return rel
	}

	b.Run("save_with_changeset_small", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rel := createReleaseWithChangeset(i, 10)
			if err := repo.Save(ctx, rel); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("save_with_changeset_large", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rel := createReleaseWithChangeset(i+50000, 100)
			if err := repo.Save(ctx, rel); err != nil {
				b.Fatal(err)
			}
		}
	})
}
