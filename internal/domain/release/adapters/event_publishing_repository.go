// Package adapters provides infrastructure implementations for the release governance domain.
package adapters

import (
	"context"
	"log/slog"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
)

// EventPublishingConfig configures an EventPublishingRepository.
type EventPublishingConfig struct {
	// Repository is the repository being wrapped. Required.
	Repository ports.ReleaseRunRepository

	// EventStore appends the events to a durable per-run stream. Optional.
	EventStore ports.EventStore

	// Publisher hands the events to subscribers — the outcome tracker, webhook
	// delivery. Optional.
	Publisher ports.EventPublisher

	// Logger reports a publication that failed after the run was already saved.
	// Optional; defaults to slog.Default().
	Logger *slog.Logger
}

// EventPublishingRepository wraps a repository and publishes domain events after save.
type EventPublishingRepository struct {
	repo       ports.ReleaseRunRepository
	eventStore ports.EventStore
	publisher  ports.EventPublisher
	logger     *slog.Logger
}

// NewEventPublishingRepository creates a new event-publishing repository wrapper.
func NewEventPublishingRepository(cfg EventPublishingConfig) *EventPublishingRepository {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &EventPublishingRepository{
		repo:       cfg.Repository,
		eventStore: cfg.EventStore,
		publisher:  cfg.Publisher,
		logger:     logger,
	}
}

// Ensure EventPublishingRepository implements the interface.
var _ ports.ReleaseRunRepository = (*EventPublishingRepository)(nil)

// Save persists a release run and publishes any pending domain events.
//
// Publication happens after persistence and does not fail the save. The run is already
// durable at that point, so returning an error would tell the caller the release did not
// happen and invite a retry that redoes work. But it is reported rather than dropped: a
// failed publication means the governance record for this release is incomplete, and the
// previous version discarded that with `_ = err` and a comment saying production might
// want to handle it differently. An audit trail that loses entries silently is the one
// failure this system exists to prevent.
func (r *EventPublishingRepository) Save(ctx context.Context, run *domain.ReleaseRun) error {
	// Collected before the save, because the events are cleared once they are out.
	events := run.DomainEvents()

	if err := r.repo.Save(ctx, run); err != nil {
		return err
	}

	if len(events) > 0 {
		// The repo root travels in the context for the file-based event store, which
		// writes inside the repository being released.
		publishCtx := WithRepoRoot(ctx, run.RepoRoot())

		if r.eventStore != nil {
			if err := r.eventStore.Append(publishCtx, run.ID(), events); err != nil {
				r.logger.Error("release saved but its events could not be appended to the event store",
					"run_id", string(run.ID()), "event_count", len(events), "error", err)
			}
		}

		if r.publisher != nil {
			if err := r.publisher.Publish(publishCtx, events...); err != nil {
				r.logger.Error("release saved but its events could not be published; "+
					"governance history and webhook delivery for this release are incomplete",
					"run_id", string(run.ID()), "event_count", len(events), "error", err)
			}
		}
	}

	// Cleared only after the events are out, so a failure above cannot silently drop
	// them from the aggregate as well.
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
