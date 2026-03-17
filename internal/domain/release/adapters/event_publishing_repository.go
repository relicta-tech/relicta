// Package adapters provides infrastructure implementations for the release governance domain.
package adapters

import (
	"context"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
)

// EventPublishingRepository wraps a repository and publishes domain events after save.
type EventPublishingRepository struct {
	repo       ports.ReleaseRunRepository
	eventStore ports.EventStore
}

// NewEventPublishingRepository creates a new event-publishing repository wrapper.
func NewEventPublishingRepository(repo ports.ReleaseRunRepository, eventStore ports.EventStore) *EventPublishingRepository {
	return &EventPublishingRepository{
		repo:       repo,
		eventStore: eventStore,
	}
}

// Ensure EventPublishingRepository implements the interface.
var _ ports.ReleaseRunRepository = (*EventPublishingRepository)(nil)

// Save persists a release run and publishes any pending domain events.
func (r *EventPublishingRepository) Save(ctx context.Context, run *domain.ReleaseRun) error {
	// Collect events before save
	events := run.DomainEvents()

	// Save the run
	if err := r.repo.Save(ctx, run); err != nil {
		return err
	}

	// Publish events (non-blocking, best-effort)
	if r.eventStore != nil && len(events) > 0 {
		// Add repo root to context for file-based event store
		ctx = WithRepoRoot(ctx, run.RepoRoot())
		if err := r.eventStore.Append(ctx, run.ID(), events); err != nil {
			// Log error but don't fail the save
			// In production, you might want to handle this differently
			_ = err
		}
	}

	// Clear events from aggregate after successful persistence
	run.ClearDomainEvents()

	return nil
}

// Load retrieves a release run by its ID.
func (r *EventPublishingRepository) Load(ctx context.Context, runID domain.RunID) (*domain.ReleaseRun, error) {
	return r.repo.Load(ctx, runID)
}

// LoadBatch retrieves multiple release runs by their IDs.
func (r *EventPublishingRepository) LoadBatch(ctx context.Context, repoRoot string, runIDs []domain.RunID) (map[domain.RunID]*domain.ReleaseRun, error) {
	return r.repo.LoadBatch(ctx, repoRoot, runIDs)
}

// LoadLatest retrieves the latest release run for a repository.
func (r *EventPublishingRepository) LoadLatest(ctx context.Context, repoRoot string) (*domain.ReleaseRun, error) {
	return r.repo.LoadLatest(ctx, repoRoot)
}

// SetLatest sets the latest run ID pointer for a repository.
func (r *EventPublishingRepository) SetLatest(ctx context.Context, repoRoot string, runID domain.RunID) error {
	return r.repo.SetLatest(ctx, repoRoot, runID)
}

// List returns all run IDs for a repository.
func (r *EventPublishingRepository) List(ctx context.Context, repoRoot string) ([]domain.RunID, error) {
	return r.repo.List(ctx, repoRoot)
}

// Delete removes a release run.
func (r *EventPublishingRepository) Delete(ctx context.Context, runID domain.RunID) error {
	return r.repo.Delete(ctx, runID)
}

// FindByState finds runs in a specific state.
func (r *EventPublishingRepository) FindByState(ctx context.Context, repoRoot string, state domain.RunState) ([]*domain.ReleaseRun, error) {
	return r.repo.FindByState(ctx, repoRoot, state)
}

// FindActive finds all non-terminal runs for a repository.
func (r *EventPublishingRepository) FindActive(ctx context.Context, repoRoot string) ([]*domain.ReleaseRun, error) {
	return r.repo.FindActive(ctx, repoRoot)
}

// FindByPlanHash finds a run by its plan hash for duplicate detection.
func (r *EventPublishingRepository) FindByPlanHash(ctx context.Context, repoRoot string, planHash string) (*domain.ReleaseRun, error) {
	return r.repo.FindByPlanHash(ctx, repoRoot, planHash)
}

// LoadFromRepo delegates to the underlying repository if it supports this method.
func (r *EventPublishingRepository) LoadFromRepo(ctx context.Context, repoRoot string, runID domain.RunID) (*domain.ReleaseRun, error) {
	if fileRepo, ok := r.repo.(interface {
		LoadFromRepo(context.Context, string, domain.RunID) (*domain.ReleaseRun, error)
	}); ok {
		return fileRepo.LoadFromRepo(ctx, repoRoot, runID)
	}
	return r.repo.Load(ctx, runID)
}
