// Package ports defines the interfaces (ports) for the release governance bounded context.
package ports

import (
	"context"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
)

// LockManager manages exclusive access to release operations.
type LockManager interface {
	// Acquire attempts to acquire an exclusive lock for the given run.
	// Returns a release function that must be called when the lock is no longer needed.
	// Returns an error if the lock cannot be acquired (e.g., another process holds it).
	Acquire(ctx context.Context, repoRoot string, runID domain.RunID) (release func(), err error)

	// TryAcquire attempts to acquire a lock without blocking.
	// Returns (release func, true, nil) if acquired, (nil, false, nil) if not available.
	TryAcquire(ctx context.Context, repoRoot string, runID domain.RunID) (release func(), acquired bool, err error)

	// IsLocked checks if a lock is currently held.
	IsLocked(ctx context.Context, repoRoot string, runID domain.RunID) (bool, error)
}

// LockInfo contains information about a held lock.
type LockInfo struct {
	RunID      domain.RunID
	HolderPID  int
	AcquiredAt string
	Hostname   string
}
