// Package adapters provides infrastructure implementations for the release governance domain.
package adapters

import (
	"context"
	"log/slog"

	"github.com/relicta-tech/relicta/v5/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v5/internal/domain/release/ports"
)

// EventPublishingConfig configures an EventPublishingRepository.
type EventPublishingConfig struct {
	// Repository is the repository being wrapped. Required.
	Repository ports.ReleaseRunRepository

	// Publisher hands the events to subscribers — the outcome tracker, webhook
	// delivery. Optional.
	Publisher ports.EventPublisher

	// Logger reports a publication that failed after the run was already saved.
	// Optional; defaults to slog.Default().
	Logger *slog.Logger
}

// EventPublishingRepository wraps a repository and publishes domain events after save.
type EventPublishingRepository struct {
	repo                   ports.ReleaseRunRepository
	recommendationFallback ports.RecommendationStore
	publisher              ports.EventPublisher
	logger                 *slog.Logger
}

// NewEventPublishingRepository creates a new event-publishing repository wrapper.
func NewEventPublishingRepository(cfg EventPublishingConfig) *EventPublishingRepository {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &EventPublishingRepository{
		repo:                   cfg.Repository,
		recommendationFallback: NewFileReleaseRunRepository(),
		publisher:              cfg.Publisher,
		logger:                 logger,
	}
}

// Ensure EventPublishingRepository implements the interface.
var _ ports.ReleaseRunRepository = (*EventPublishingRepository)(nil)
var _ ports.RecommendationStore = (*EventPublishingRepository)(nil)

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

	if len(events) == 0 {
		return nil
	}
	if r.publisher == nil {
		// There is no retry path when no publisher is configured. Preserve the events
		// on the aggregate so a caller that attaches a publisher can still deliver them.
		r.logger.Warn("release saved with unpublished domain events; no event publisher is configured",
			"run_id", string(run.ID()), "event_count", len(events))
		return nil
	}
	if err := r.publisher.Publish(ctx, events...); err != nil {
		// Keep them on the aggregate for an explicit retry in this process. The saved
		// run does not serialize pending events, so this is not crash recovery; the
		// log makes that limit clear to operators.
		r.logger.Error("release saved but event publication failed; events remain pending only in memory",
			"run_id", string(run.ID()), "event_count", len(events), "error", err)
		return nil
	}
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

// SaveRecommendation delegates to the release repository when it supports
// recommendation storage. Database-backed release stores keep the ADR-009
// artifact as the same file sidecar used by the file backend so consumers
// receive the original bytes regardless of the selected run store.
func (r *EventPublishingRepository) SaveRecommendation(repoRoot string, runID domain.RunID, artifact []byte) error {
	store, ok := r.repo.(ports.RecommendationStore)
	if ok {
		return store.SaveRecommendation(repoRoot, runID, artifact)
	}
	return r.recommendationFallback.SaveRecommendation(repoRoot, runID, artifact)
}

// LoadRecommendation delegates optional recommendation reads to the wrapped
// repository, keeping the HTTP recommendation endpoint available through the
// normal event-publishing container wiring.
func (r *EventPublishingRepository) LoadRecommendation(repoRoot string, runID domain.RunID) ([]byte, bool, error) {
	store, ok := r.repo.(ports.RecommendationStore)
	if ok {
		return store.LoadRecommendation(repoRoot, runID)
	}
	return r.recommendationFallback.LoadRecommendation(repoRoot, runID)
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
