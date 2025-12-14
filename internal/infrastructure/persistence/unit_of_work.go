// Package persistence provides infrastructure implementations for data persistence.
package persistence

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/felixgeelhaar/release-pilot/internal/domain/release"
)

// FileUnitOfWork implements release.UnitOfWork for file-based storage.
// It provides transactional semantics for release operations by deferring
// writes until commit and providing rollback capability.
type FileUnitOfWork struct {
	baseRepo       *FileReleaseRepository
	eventPublisher release.EventPublisher
	mu             sync.Mutex

	// Transaction state
	active         bool
	pendingWrites  map[release.ReleaseID]*release.Release
	pendingDeletes map[release.ReleaseID]struct{}
	pendingEvents  []release.DomainEvent
}

// FileUnitOfWorkFactory creates new FileUnitOfWork instances.
type FileUnitOfWorkFactory struct {
	baseRepo       *FileReleaseRepository
	eventPublisher release.EventPublisher
}

// NewFileUnitOfWorkFactory creates a new FileUnitOfWorkFactory.
func NewFileUnitOfWorkFactory(
	baseRepo *FileReleaseRepository,
	eventPublisher release.EventPublisher,
) *FileUnitOfWorkFactory {
	return &FileUnitOfWorkFactory{
		baseRepo:       baseRepo,
		eventPublisher: eventPublisher,
	}
}

// Begin starts a new unit of work transaction.
func (f *FileUnitOfWorkFactory) Begin(ctx context.Context) (release.UnitOfWork, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return &FileUnitOfWork{
		baseRepo:       f.baseRepo,
		eventPublisher: f.eventPublisher,
		active:         true,
		pendingWrites:  make(map[release.ReleaseID]*release.Release),
		pendingDeletes: make(map[release.ReleaseID]struct{}),
		pendingEvents:  make([]release.DomainEvent, 0),
	}, nil
}

// unitOfWorkRepository wraps the base repository to track changes within a transaction.
type unitOfWorkRepository struct {
	uow      *FileUnitOfWork
	baseRepo *FileReleaseRepository
}

// Begin starts a new unit of work.
// Returns an error if called on an already active unit of work to prevent
// nested transaction confusion and accidental premature commits.
func (u *FileUnitOfWork) Begin(ctx context.Context) (release.UnitOfWork, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if !u.active {
		return nil, fmt.Errorf("unit of work is not active")
	}

	// Prevent nested transactions - return error instead of self
	return nil, fmt.Errorf("unit of work already active: nested transactions not supported")
}

// Commit commits all pending changes.
// The context is used for cancellation and timeout control during persistence operations.
func (u *FileUnitOfWork) Commit(ctx context.Context) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if !u.active {
		return fmt.Errorf("unit of work is not active")
	}

	// Check for context cancellation before starting
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("commit aborted: %w", err)
	}

	// Process deletes first
	for id := range u.pendingDeletes {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("commit aborted during deletes: %w", err)
		}
		if err := u.baseRepo.Delete(ctx, id); err != nil {
			// Rollback is not possible for file operations, fail fast on error
			return fmt.Errorf("failed to delete release %s: %w", id, err)
		}
	}

	// Process writes
	for _, rel := range u.pendingWrites {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("commit aborted during writes: %w", err)
		}
		if err := u.baseRepo.Save(ctx, rel); err != nil {
			return fmt.Errorf("failed to save release %s: %w", rel.ID(), err)
		}
	}

	// Publish events after successful persistence
	if u.eventPublisher != nil && len(u.pendingEvents) > 0 {
		if err := u.eventPublisher.Publish(ctx, u.pendingEvents...); err != nil {
			// Event publishing failure after persistence is non-fatal
			// Log the error for visibility and debugging
			slog.Warn("failed to publish domain events after commit",
				"error", err,
				"event_count", len(u.pendingEvents))
		}
	}

	// Mark transaction as complete
	u.active = false
	u.clearPending()

	return nil
}

// Rollback discards all pending changes.
func (u *FileUnitOfWork) Rollback() error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if !u.active {
		// Already rolled back or committed, no-op
		return nil
	}

	u.active = false
	u.clearPending()

	return nil
}

// ReleaseRepository returns the release repository within this unit of work.
func (u *FileUnitOfWork) ReleaseRepository() release.Repository {
	return &unitOfWorkRepository{
		uow:      u,
		baseRepo: u.baseRepo,
	}
}

// AddEvents adds domain events to be published on commit.
func (u *FileUnitOfWork) AddEvents(events ...release.DomainEvent) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.active {
		u.pendingEvents = append(u.pendingEvents, events...)
	}
}

// clearPending clears all pending operations.
func (u *FileUnitOfWork) clearPending() {
	u.pendingWrites = make(map[release.ReleaseID]*release.Release)
	u.pendingDeletes = make(map[release.ReleaseID]struct{})
	u.pendingEvents = make([]release.DomainEvent, 0)
}

// unitOfWorkRepository implementation

// Save stages a release for saving on commit.
func (r *unitOfWorkRepository) Save(ctx context.Context, rel *release.Release) error {
	r.uow.mu.Lock()
	defer r.uow.mu.Unlock()

	if !r.uow.active {
		return fmt.Errorf("unit of work is not active")
	}

	// Remove from deletes if present
	delete(r.uow.pendingDeletes, rel.ID())

	// Stage for write
	r.uow.pendingWrites[rel.ID()] = rel

	// Collect domain events
	events := rel.DomainEvents()
	if len(events) > 0 {
		r.uow.pendingEvents = append(r.uow.pendingEvents, events...)
		rel.ClearDomainEvents()
	}

	return nil
}

// FindByID retrieves a release, checking pending changes first.
func (r *unitOfWorkRepository) FindByID(ctx context.Context, id release.ReleaseID) (*release.Release, error) {
	r.uow.mu.Lock()
	defer r.uow.mu.Unlock()

	if !r.uow.active {
		return nil, fmt.Errorf("unit of work is not active")
	}

	// Check if deleted
	if _, deleted := r.uow.pendingDeletes[id]; deleted {
		return nil, release.ErrReleaseNotFound
	}

	// Check pending writes first
	if rel, ok := r.uow.pendingWrites[id]; ok {
		return rel, nil
	}

	// Fall through to base repository
	return r.baseRepo.FindByID(ctx, id)
}

// FindLatest retrieves the latest release for a repository.
func (r *unitOfWorkRepository) FindLatest(ctx context.Context, repoPath string) (*release.Release, error) {
	r.uow.mu.Lock()
	defer r.uow.mu.Unlock()

	if !r.uow.active {
		return nil, fmt.Errorf("unit of work is not active")
	}

	// For consistency, we need to consider pending changes
	// First get from base
	baseRelease, baseErr := r.baseRepo.FindLatest(ctx, repoPath)

	// Check pending writes for newer releases
	var latestPending *release.Release
	for id, rel := range r.uow.pendingWrites {
		// Skip deleted
		if _, deleted := r.uow.pendingDeletes[id]; deleted {
			continue
		}

		if rel.RepositoryPath() == repoPath {
			if latestPending == nil || rel.UpdatedAt().After(latestPending.UpdatedAt()) {
				latestPending = rel
			}
		}
	}

	// Return the most recent
	if latestPending != nil {
		if baseRelease == nil || latestPending.UpdatedAt().After(baseRelease.UpdatedAt()) {
			return latestPending, nil
		}
	}

	if baseRelease != nil {
		// Check if base release is deleted
		if _, deleted := r.uow.pendingDeletes[baseRelease.ID()]; deleted {
			return nil, release.ErrReleaseNotFound
		}
		return baseRelease, nil
	}

	return nil, baseErr
}

// FindByState retrieves releases in a specific state.
func (r *unitOfWorkRepository) FindByState(ctx context.Context, state release.ReleaseState) ([]*release.Release, error) {
	r.uow.mu.Lock()
	defer r.uow.mu.Unlock()

	if !r.uow.active {
		return nil, fmt.Errorf("unit of work is not active")
	}

	// Get from base repository
	baseReleases, err := r.baseRepo.FindByState(ctx, state)
	if err != nil {
		return nil, err
	}

	// Build result set considering pending changes
	result := make([]*release.Release, 0, len(baseReleases))
	seen := make(map[release.ReleaseID]bool)

	// Add pending writes that match state
	for id, rel := range r.uow.pendingWrites {
		if _, deleted := r.uow.pendingDeletes[id]; deleted {
			continue
		}
		if rel.State() == state {
			result = append(result, rel)
			seen[id] = true
		}
	}

	// Add base releases not in pending writes or deletes
	for _, rel := range baseReleases {
		if _, deleted := r.uow.pendingDeletes[rel.ID()]; deleted {
			continue
		}
		if seen[rel.ID()] {
			continue
		}
		// Check if overridden in pending writes (state may have changed)
		if pending, ok := r.uow.pendingWrites[rel.ID()]; ok {
			if pending.State() == state {
				result = append(result, pending)
			}
			continue
		}
		result = append(result, rel)
	}

	return result, nil
}

// FindActive retrieves all active (non-final) releases.
func (r *unitOfWorkRepository) FindActive(ctx context.Context) ([]*release.Release, error) {
	r.uow.mu.Lock()
	defer r.uow.mu.Unlock()

	if !r.uow.active {
		return nil, fmt.Errorf("unit of work is not active")
	}

	// Get from base repository
	baseReleases, err := r.baseRepo.FindActive(ctx)
	if err != nil {
		return nil, err
	}

	// Build result set considering pending changes
	result := make([]*release.Release, 0, len(baseReleases))
	seen := make(map[release.ReleaseID]bool)

	// Add pending writes that are active
	for id, rel := range r.uow.pendingWrites {
		if _, deleted := r.uow.pendingDeletes[id]; deleted {
			continue
		}
		if !rel.State().IsFinal() {
			result = append(result, rel)
			seen[id] = true
		}
	}

	// Add base releases not in pending writes or deletes
	for _, rel := range baseReleases {
		if _, deleted := r.uow.pendingDeletes[rel.ID()]; deleted {
			continue
		}
		if seen[rel.ID()] {
			continue
		}
		// Check if overridden in pending writes
		if pending, ok := r.uow.pendingWrites[rel.ID()]; ok {
			if !pending.State().IsFinal() {
				result = append(result, pending)
			}
			continue
		}
		result = append(result, rel)
	}

	return result, nil
}

// Delete stages a release for deletion on commit.
func (r *unitOfWorkRepository) Delete(ctx context.Context, id release.ReleaseID) error {
	r.uow.mu.Lock()
	defer r.uow.mu.Unlock()

	if !r.uow.active {
		return fmt.Errorf("unit of work is not active")
	}

	// Remove from pending writes
	delete(r.uow.pendingWrites, id)

	// Stage for delete
	r.uow.pendingDeletes[id] = struct{}{}

	return nil
}

// FindBySpecification retrieves releases matching the given specification.
func (r *unitOfWorkRepository) FindBySpecification(ctx context.Context, spec release.Specification) ([]*release.Release, error) {
	r.uow.mu.Lock()
	defer r.uow.mu.Unlock()

	if !r.uow.active {
		return nil, fmt.Errorf("unit of work is not active")
	}

	// Get from base repository
	baseReleases, err := r.baseRepo.FindBySpecification(ctx, spec)
	if err != nil {
		return nil, err
	}

	// Build result set considering pending changes
	result := make([]*release.Release, 0, len(baseReleases))
	seen := make(map[release.ReleaseID]bool)

	// Add pending writes that match specification
	for id, rel := range r.uow.pendingWrites {
		if _, deleted := r.uow.pendingDeletes[id]; deleted {
			continue
		}
		if spec.IsSatisfiedBy(rel) {
			result = append(result, rel)
			seen[id] = true
		}
	}

	// Add base releases not in pending writes or deletes
	for _, rel := range baseReleases {
		if _, deleted := r.uow.pendingDeletes[rel.ID()]; deleted {
			continue
		}
		if seen[rel.ID()] {
			continue
		}
		// Check if overridden in pending writes
		if pending, ok := r.uow.pendingWrites[rel.ID()]; ok {
			if spec.IsSatisfiedBy(pending) {
				result = append(result, pending)
			}
			continue
		}
		result = append(result, rel)
	}

	return result, nil
}

// Ensure FileUnitOfWork implements the release.UnitOfWork interface.
var _ release.UnitOfWork = (*FileUnitOfWork)(nil)
