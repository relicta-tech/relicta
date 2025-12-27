// Package ports defines the interfaces (ports) for the release governance bounded context.
// These are the abstractions that the domain and application layers depend on.
package ports

import (
	"context"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
)

// ReleaseRunRepository defines the interface for persisting and retrieving release runs.
type ReleaseRunRepository interface {
	// Save persists a release run.
	Save(ctx context.Context, run *domain.ReleaseRun) error

	// Load retrieves a release run by its ID.
	Load(ctx context.Context, runID domain.RunID) (*domain.ReleaseRun, error)

	// LoadLatest retrieves the latest release run for a repository.
	LoadLatest(ctx context.Context, repoRoot string) (*domain.ReleaseRun, error)

	// SetLatest sets the latest run ID pointer for a repository.
	SetLatest(ctx context.Context, repoRoot string, runID domain.RunID) error

	// List returns all run IDs for a repository, ordered by creation time (newest first).
	List(ctx context.Context, repoRoot string) ([]domain.RunID, error)

	// Delete removes a release run.
	Delete(ctx context.Context, runID domain.RunID) error

	// FindByState finds runs in a specific state.
	FindByState(ctx context.Context, repoRoot string, state domain.RunState) ([]*domain.ReleaseRun, error)

	// FindActive finds all non-terminal runs for a repository.
	FindActive(ctx context.Context, repoRoot string) ([]*domain.ReleaseRun, error)

	// FindByPlanHash finds a run by its plan hash for duplicate detection.
	// Returns nil, nil if no run exists with that plan hash.
	FindByPlanHash(ctx context.Context, repoRoot string, planHash string) (*domain.ReleaseRun, error)
}
