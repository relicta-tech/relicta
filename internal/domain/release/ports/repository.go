// Package ports defines the interfaces (ports) for the release governance bounded context.
// These are the abstractions that the domain and application layers depend on.
package ports

import (
	"context"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
)

// RunReader defines read operations for release runs.
// Use this interface when you only need to read runs without modifying them.
type RunReader interface {
	// Load retrieves a release run by its ID.
	Load(ctx context.Context, runID domain.RunID) (*domain.ReleaseRun, error)

	// LoadBatch retrieves multiple release runs by their IDs.
	// Returns a map of runID to run, skipping any runs that could not be loaded.
	LoadBatch(ctx context.Context, repoRoot string, runIDs []domain.RunID) (map[domain.RunID]*domain.ReleaseRun, error)

	// LoadLatest retrieves the latest release run for a repository.
	LoadLatest(ctx context.Context, repoRoot string) (*domain.ReleaseRun, error)

	// List returns all run IDs for a repository, most recently saved first.
	//
	// Saved, not created: a run that was planned first and updated last leads. This said
	// "creation time" and every implementation sorted by modification time, which is what
	// callers were written against — see the conformance suite, which pins it.
	List(ctx context.Context, repoRoot string) ([]domain.RunID, error)
}

// RunWriter defines write operations for release runs.
// Use this interface when you need to persist or remove runs.
type RunWriter interface {
	// Save persists a release run.
	Save(ctx context.Context, run *domain.ReleaseRun) error

	// SetLatest sets the latest run ID pointer for a repository.
	SetLatest(ctx context.Context, repoRoot string, runID domain.RunID) error

	// Delete removes a release run.
	Delete(ctx context.Context, runID domain.RunID) error
}

// RunQuery defines query operations for release runs.
// Use this interface when you need to find runs by specific criteria.
type RunQuery interface {
	// FindByState finds runs in a specific state.
	FindByState(ctx context.Context, repoRoot string, state domain.RunState) ([]*domain.ReleaseRun, error)

	// FindActive finds all non-terminal runs for a repository.
	FindActive(ctx context.Context, repoRoot string) ([]*domain.ReleaseRun, error)

	// FindByPlanHash finds a run by its plan hash for duplicate detection.
	// Returns nil, nil if no run exists with that plan hash.
	FindByPlanHash(ctx context.Context, repoRoot string, planHash string) (*domain.ReleaseRun, error)
}

// RecommendationStore persists the deterministic recommendation artifact
// (ADR-009) alongside the run it describes.
//
// Kept separate from ReleaseRunRepository on purpose. ADR-009 makes the artifact
// the contract every interface returns, but not every repository implementation
// needs to store one, and folding these two methods into the full interface would
// oblige the bridge and every test double to implement them. Callers type-assert,
// which is also how SaveMachineJSON is reached.
type RecommendationStore interface {
	// SaveRecommendation stores the artifact bytes for a run, as given.
	SaveRecommendation(repoRoot string, runID domain.RunID, artifact []byte) error

	// LoadRecommendation returns a run's artifact. found is false when the run has
	// none — an ordinary case for runs planned before artifacts were persisted,
	// and distinct from a read failure.
	LoadRecommendation(repoRoot string, runID domain.RunID) (artifact []byte, found bool, err error)
}

// ReleaseRunRepository is the full interface combining all repository operations.
// Prefer using the smaller interfaces (RunReader, RunWriter, RunQuery) when possible.
type ReleaseRunRepository interface {
	RunReader
	RunWriter
	RunQuery
}
