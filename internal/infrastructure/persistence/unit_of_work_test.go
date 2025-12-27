package persistence

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestFileUnitOfWork_BasicOperations(t *testing.T) {
	// Setup
	tempDir, err := os.MkdirTemp("", "uow-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	baseRepo, err := NewFileReleaseRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create base repo: %v", err)
	}

	eventPublisher := NewInMemoryEventPublisher()
	factory := NewFileUnitOfWorkFactory(baseRepo, eventPublisher)

	ctx := context.Background()

	t.Run("commit saves changes", func(t *testing.T) {
		// Begin transaction
		uow, err := factory.Begin(ctx)
		if err != nil {
			t.Fatalf("failed to begin: %v", err)
		}

		// Create and save release
		rel := release.NewRelease("test-rel-1", "main", "/repo")
		repo := uow.ReleaseRepository()

		if err := repo.Save(ctx, rel); err != nil {
			t.Fatalf("failed to save: %v", err)
		}

		// Before commit, should not be in base repo
		_, err = baseRepo.FindByID(ctx, "test-rel-1")
		if err == nil {
			t.Error("expected release to not be in base repo before commit")
		}

		// Commit
		if err := uow.Commit(ctx); err != nil {
			t.Fatalf("failed to commit: %v", err)
		}

		// After commit, should be in base repo
		found, err := baseRepo.FindByID(ctx, "test-rel-1")
		if err != nil {
			t.Fatalf("failed to find after commit: %v", err)
		}
		if found.ID() != "test-rel-1" {
			t.Errorf("expected ID test-rel-1, got %s", found.ID())
		}
	})

	t.Run("rollback discards changes", func(t *testing.T) {
		// Begin transaction
		uow, err := factory.Begin(ctx)
		if err != nil {
			t.Fatalf("failed to begin: %v", err)
		}

		// Create and save release
		rel := release.NewRelease("test-rel-rollback", "main", "/repo")
		repo := uow.ReleaseRepository()

		if err := repo.Save(ctx, rel); err != nil {
			t.Fatalf("failed to save: %v", err)
		}

		// Rollback
		if err := uow.Rollback(); err != nil {
			t.Fatalf("failed to rollback: %v", err)
		}

		// After rollback, should not be in base repo
		_, err = baseRepo.FindByID(ctx, "test-rel-rollback")
		if err == nil {
			t.Error("expected release to not be in base repo after rollback")
		}
	})

	t.Run("find returns pending write", func(t *testing.T) {
		// Begin transaction
		uow, err := factory.Begin(ctx)
		if err != nil {
			t.Fatalf("failed to begin: %v", err)
		}
		defer uow.Rollback()

		// Create and save release
		rel := release.NewRelease("test-rel-pending", "main", "/repo")
		repo := uow.ReleaseRepository()

		if err := repo.Save(ctx, rel); err != nil {
			t.Fatalf("failed to save: %v", err)
		}

		// Find should return the pending write
		found, err := repo.FindByID(ctx, "test-rel-pending")
		if err != nil {
			t.Fatalf("failed to find pending: %v", err)
		}
		if found.ID() != "test-rel-pending" {
			t.Errorf("expected ID test-rel-pending, got %s", found.ID())
		}
	})

	t.Run("delete stages deletion", func(t *testing.T) {
		// First create a release in base repo
		rel := release.NewRelease("test-rel-delete", "main", "/repo")
		if err := baseRepo.Save(ctx, rel); err != nil {
			t.Fatalf("failed to save to base: %v", err)
		}

		// Begin transaction
		uow, err := factory.Begin(ctx)
		if err != nil {
			t.Fatalf("failed to begin: %v", err)
		}

		repo := uow.ReleaseRepository()

		// Delete should stage for deletion
		if err := repo.Delete(ctx, "test-rel-delete"); err != nil {
			t.Fatalf("failed to delete: %v", err)
		}

		// Find should return not found
		_, err = repo.FindByID(ctx, "test-rel-delete")
		if err == nil {
			t.Error("expected not found after delete")
		}

		// Commit
		if err := uow.Commit(ctx); err != nil {
			t.Fatalf("failed to commit: %v", err)
		}

		// After commit, should be deleted from base repo
		_, err = baseRepo.FindByID(ctx, "test-rel-delete")
		if err == nil {
			t.Error("expected release to be deleted from base repo")
		}
	})

	t.Run("inactive uow returns error", func(t *testing.T) {
		// Begin and commit
		uow, _ := factory.Begin(ctx)
		_ = uow.Commit(ctx)

		// Operations on committed uow should fail
		repo := uow.ReleaseRepository()
		_, err := repo.FindByID(ctx, "any")
		if err == nil {
			t.Error("expected error on inactive uow")
		}
	})
}

func TestFileUnitOfWork_EventCollection(t *testing.T) {
	// Setup
	tempDir, err := os.MkdirTemp("", "uow-event-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	baseRepo, err := NewFileReleaseRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create base repo: %v", err)
	}

	eventPublisher := NewInMemoryEventPublisher()
	factory := NewFileUnitOfWorkFactory(baseRepo, eventPublisher)

	ctx := context.Background()

	t.Run("events are published on commit", func(t *testing.T) {
		// Clear any existing events
		eventPublisher.ClearEvents()

		// Begin transaction
		uow, _ := factory.Begin(ctx)
		repo := uow.ReleaseRepository()

		// Create release (which generates events)
		rel := release.NewRelease("test-rel-events", "main", "/repo")

		// Save to collect events
		repo.Save(ctx, rel)

		// Events should not be published yet
		events := eventPublisher.GetEvents()
		if len(events) > 0 {
			t.Error("events should not be published before commit")
		}

		// Commit
		_ = uow.Commit(ctx)

		// Events should be published after commit
		events = eventPublisher.GetEvents()
		if len(events) == 0 {
			t.Error("events should be published after commit")
		}
	})
}

func TestFileUnitOfWork_FindByStateActiveAndSpec(t *testing.T) {
	tempDir := t.TempDir()
	baseRepo, err := NewFileReleaseRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create base repo: %v", err)
	}

	factory := NewFileUnitOfWorkFactory(baseRepo, NewInMemoryEventPublisher())
	ctx := context.Background()

	// Helper to create a planned release
	createPlannedRelease := func(id, branch, repoPath string) *release.Release {
		r := release.NewRelease(release.ReleaseID(id), branch, repoPath)
		cs := changes.NewChangeSet(changes.ChangeSetID("cs-"+id), "v1.0.0", "HEAD")
		cs.AddCommit(changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "feature"))
		plan := release.NewReleasePlan(
			version.MustParse("1.0.0"),
			version.MustParse("1.1.0"),
			changes.ReleaseTypeMinor,
			cs,
			false,
		)
		_ = release.SetPlan(r, plan)
		return r
	}

	// Create an active (planned) release
	active := createPlannedRelease("base-active", "main", "/repo1")

	// Create a final (canceled) release
	final := release.NewRelease("base-final", "main", "/repo2")
	_ = final.Cancel("test", "user")

	if err := baseRepo.Save(ctx, active); err != nil {
		t.Fatalf("save active error: %v", err)
	}
	if err := baseRepo.Save(ctx, final); err != nil {
		t.Fatalf("save final error: %v", err)
	}

	uow, err := factory.Begin(ctx)
	if err != nil {
		t.Fatalf("begin error: %v", err)
	}
	defer uow.Rollback()

	repo := uow.ReleaseRepository()
	pending := createPlannedRelease("pending-active", "dev", "/repo3")
	if err := repo.Save(ctx, pending); err != nil {
		t.Fatalf("save pending error: %v", err)
	}

	planned, err := repo.FindByState(ctx, release.StatePlanned)
	if err != nil {
		t.Fatalf("FindByState error: %v", err)
	}
	if len(planned) != 2 {
		t.Fatalf("expected 2 planned releases, got %d", len(planned))
	}

	activeList, err := repo.FindActive(ctx)
	if err != nil {
		t.Fatalf("FindActive error: %v", err)
	}
	if len(activeList) != 2 {
		t.Fatalf("expected 2 active releases, got %d", len(activeList))
	}

	specList, err := repo.FindBySpecification(ctx, release.ByRepositoryPath("/repo3"))
	if err != nil {
		t.Fatalf("FindBySpecification error: %v", err)
	}
	if len(specList) != 1 || specList[0].ID() != "pending-active" {
		t.Fatalf("unexpected spec results: %#v", specList)
	}
}

func TestFileUnitOfWork_IndependentTransactions(t *testing.T) {
	// Setup
	tempDir, err := os.MkdirTemp("", "uow-independent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	baseRepo, err := NewFileReleaseRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create base repo: %v", err)
	}

	factory := NewFileUnitOfWorkFactory(baseRepo, nil)
	ctx := context.Background()

	// Begin first transaction
	uow1, err := factory.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin first: %v", err)
	}
	defer uow1.Rollback()

	// Begin second independent transaction via factory - this should work
	uow2, err := factory.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin second: %v", err)
	}
	defer uow2.Rollback()

	// Both transactions should be independent and usable
	repo1 := uow1.ReleaseRepository()
	repo2 := uow2.ReleaseRepository()

	rel1 := release.NewRelease("test-rel-1", "main", "/repo1")
	rel2 := release.NewRelease("test-rel-2", "main", "/repo2")

	if err := repo1.Save(ctx, rel1); err != nil {
		t.Fatalf("failed to save in uow1: %v", err)
	}
	if err := repo2.Save(ctx, rel2); err != nil {
		t.Fatalf("failed to save in uow2: %v", err)
	}

	// Commit first, rollback second
	if err := uow1.Commit(ctx); err != nil {
		t.Fatalf("failed to commit uow1: %v", err)
	}
	_ = uow2.Rollback()

	// Only rel1 should be persisted
	if _, err := baseRepo.FindByID(ctx, "test-rel-1"); err != nil {
		t.Error("expected rel1 to be persisted after uow1 commit")
	}
	if _, err := baseRepo.FindByID(ctx, "test-rel-2"); err == nil {
		t.Error("expected rel2 to not be persisted after uow2 rollback")
	}
}

func TestFileUnitOfWork_ContextCancellation(t *testing.T) {
	// Setup
	tempDir, err := os.MkdirTemp("", "uow-cancel-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	baseRepo, err := NewFileReleaseRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create base repo: %v", err)
	}

	factory := NewFileUnitOfWorkFactory(baseRepo, nil)

	t.Run("commit with canceled context fails", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		uow, err := factory.Begin(ctx)
		if err != nil {
			t.Fatalf("failed to begin: %v", err)
		}

		// Save a release
		rel := release.NewRelease("test-rel-cancel", "main", "/repo")
		repo := uow.ReleaseRepository()
		_ = repo.Save(ctx, rel)

		// Cancel context before commit
		cancel()

		// Commit should fail
		err = uow.Commit(ctx)
		if err == nil {
			t.Error("expected error when committing with canceled context")
		}
	})

	t.Run("begin with canceled context fails", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := factory.Begin(ctx)
		if err == nil {
			t.Error("expected error when beginning with canceled context")
		}
	})
}

func TestFileUnitOfWork_FindLatest(t *testing.T) {
	// Setup
	tempDir, err := os.MkdirTemp("", "uow-findlatest-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	baseRepo, err := NewFileReleaseRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create base repo: %v", err)
	}

	factory := NewFileUnitOfWorkFactory(baseRepo, nil)
	ctx := context.Background()

	t.Run("finds pending write", func(t *testing.T) {
		uow, _ := factory.Begin(ctx)
		defer uow.Rollback()

		rel := release.NewRelease("test-rel-latest", "main", "/test/repo")
		repo := uow.ReleaseRepository()
		_ = repo.Save(ctx, rel)

		// FindLatest should return the pending write
		found, err := repo.FindLatest(ctx, "/test/repo")
		if err != nil {
			t.Fatalf("FindLatest failed: %v", err)
		}
		if found.ID() != "test-rel-latest" {
			t.Errorf("expected ID test-rel-latest, got %s", found.ID())
		}
	})

	t.Run("respects deleted releases", func(t *testing.T) {
		// First save to base repo
		rel := release.NewRelease("test-rel-delete-latest", "main", "/delete/repo")
		_ = baseRepo.Save(ctx, rel)

		uow, _ := factory.Begin(ctx)
		defer uow.Rollback()

		repo := uow.ReleaseRepository()
		_ = repo.Delete(ctx, "test-rel-delete-latest")

		// FindLatest should not find deleted release
		_, err := repo.FindLatest(ctx, "/delete/repo")
		if err == nil {
			t.Error("expected not found for deleted release")
		}
	})
}

func TestFileUnitOfWork_FindByState(t *testing.T) {
	// Setup
	tempDir, err := os.MkdirTemp("", "uow-findstate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	baseRepo, err := NewFileReleaseRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create base repo: %v", err)
	}

	factory := NewFileUnitOfWorkFactory(baseRepo, nil)
	ctx := context.Background()

	t.Run("includes pending writes", func(t *testing.T) {
		uow, _ := factory.Begin(ctx)
		defer uow.Rollback()

		// NewRelease starts in StateDraft
		rel := release.NewRelease("test-rel-state", "main", "/repo")
		repo := uow.ReleaseRepository()
		_ = repo.Save(ctx, rel)

		// FindByState should include the pending write (new releases start in StateDraft)
		releases, err := repo.FindByState(ctx, release.StateDraft)
		if err != nil {
			t.Fatalf("FindByState failed: %v", err)
		}
		found := false
		for _, r := range releases {
			if r.ID() == "test-rel-state" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find pending release in FindByState")
		}
	})
}

func TestFileUnitOfWork_FindActive(t *testing.T) {
	// Setup
	tempDir, err := os.MkdirTemp("", "uow-findactive-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	baseRepo, err := NewFileReleaseRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create base repo: %v", err)
	}

	factory := NewFileUnitOfWorkFactory(baseRepo, nil)
	ctx := context.Background()

	t.Run("includes pending active releases", func(t *testing.T) {
		uow, _ := factory.Begin(ctx)
		defer uow.Rollback()

		rel := release.NewRelease("test-rel-active", "main", "/repo")
		repo := uow.ReleaseRepository()
		_ = repo.Save(ctx, rel)

		// FindActive should include the pending write (Planned state is not final)
		releases, err := repo.FindActive(ctx)
		if err != nil {
			t.Fatalf("FindActive failed: %v", err)
		}
		found := false
		for _, r := range releases {
			if r.ID() == "test-rel-active" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find pending release in FindActive")
		}
	})
}

func TestFileUnitOfWork_AddEvents(t *testing.T) {
	// Setup
	tempDir, err := os.MkdirTemp("", "uow-addevents-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	baseRepo, err := NewFileReleaseRepository(tempDir)
	if err != nil {
		t.Fatalf("failed to create base repo: %v", err)
	}

	eventPublisher := NewInMemoryEventPublisher()
	factory := NewFileUnitOfWorkFactory(baseRepo, eventPublisher)
	ctx := context.Background()

	t.Run("manual events are published on commit", func(t *testing.T) {
		eventPublisher.ClearEvents()

		uow, _ := factory.Begin(ctx)

		// Cast to access AddEvents
		fileUoW := uow.(*FileUnitOfWork)

		// Manually add an event
		testEvent := &testDomainEvent{
			eventName:   "TestEvent",
			aggregateID: "test-1",
			occurredAt:  time.Now(),
		}
		fileUoW.AddEvents(testEvent)

		_ = uow.Commit(ctx)

		// Event should be published
		events := eventPublisher.GetEvents()
		found := false
		for _, e := range events {
			if e.EventName() == "TestEvent" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected manually added event to be published")
		}
	})
}

// testDomainEvent is a simple domain event for testing
type testDomainEvent struct {
	eventName   string
	aggregateID release.ReleaseID
	occurredAt  time.Time
}

func (e *testDomainEvent) EventName() string              { return e.eventName }
func (e *testDomainEvent) AggregateID() release.ReleaseID { return e.aggregateID }
func (e *testDomainEvent) OccurredAt() time.Time          { return e.occurredAt }
