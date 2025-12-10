// Package release provides domain types for release management.
package release

import (
	"context"
)

// Repository defines the interface for persisting and retrieving releases.
// This is a repository interface in DDD - part of the domain layer.
type Repository interface {
	// Save persists a release.
	Save(ctx context.Context, release *Release) error

	// FindByID retrieves a release by its ID.
	FindByID(ctx context.Context, id ReleaseID) (*Release, error)

	// FindLatest retrieves the latest release for a repository.
	FindLatest(ctx context.Context, repoPath string) (*Release, error)

	// FindByState retrieves releases in a specific state.
	FindByState(ctx context.Context, state ReleaseState) ([]*Release, error)

	// FindActive retrieves all active (non-final) releases.
	FindActive(ctx context.Context) ([]*Release, error)

	// Delete removes a release.
	Delete(ctx context.Context, id ReleaseID) error
}

// EventPublisher defines the interface for publishing domain events.
type EventPublisher interface {
	// Publish publishes domain events.
	Publish(ctx context.Context, events ...DomainEvent) error
}

// UnitOfWork defines the interface for transactional operations.
type UnitOfWork interface {
	// Begin starts a new unit of work.
	Begin(ctx context.Context) (UnitOfWork, error)

	// Commit commits the unit of work.
	Commit() error

	// Rollback rolls back the unit of work.
	Rollback() error

	// ReleaseRepository returns the release repository within this unit of work.
	ReleaseRepository() Repository
}
