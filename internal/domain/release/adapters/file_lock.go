// Package adapters provides infrastructure implementations for the release governance domain.
package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
)

const (
	lockFileName      = "lock"
	lockStaleDuration = 10 * time.Minute
)

// FileLockManager implements LockManager using file-based locking.
type FileLockManager struct{}

// NewFileLockManager creates a new file-based lock manager.
func NewFileLockManager() *FileLockManager {
	return &FileLockManager{}
}

// Ensure FileLockManager implements the interface.
var _ ports.LockManager = (*FileLockManager)(nil)

// LockFileContents represents the contents of the lock file.
type LockFileContents struct {
	RunID      string    `json:"run_id"`
	PID        int       `json:"pid"`
	Hostname   string    `json:"hostname"`
	AcquiredAt time.Time `json:"acquired_at"`
}

// lockPath returns the path to the lock file.
func lockPath(repoRoot string) string {
	return filepath.Join(repoRoot, runsDir, lockFileName)
}

// Acquire attempts to acquire an exclusive lock for the given run.
func (m *FileLockManager) Acquire(ctx context.Context, repoRoot string, runID domain.RunID) (func(), error) {
	path := lockPath(repoRoot)

	// Ensure the directory exists
	if err := ensureDir(repoRoot); err != nil {
		return nil, fmt.Errorf("failed to create runs directory: %w", err)
	}

	// Check for existing lock
	if existing, err := m.readLock(path); err == nil {
		// Check if lock is stale
		if time.Since(existing.AcquiredAt) > lockStaleDuration {
			// Stale lock - we can take it
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to remove stale lock: %w", err)
			}
		} else {
			// Lock is still valid
			return nil, fmt.Errorf("lock held by PID %d on %s since %s for run %s",
				existing.PID, existing.Hostname, existing.AcquiredAt.Format(time.RFC3339), existing.RunID)
		}
	}

	// Create lock file
	hostname, _ := os.Hostname()
	lock := LockFileContents{
		RunID:      string(runID),
		PID:        os.Getpid(),
		Hostname:   hostname,
		AcquiredAt: time.Now(),
	}

	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal lock: %w", err)
	}

	// Use O_EXCL to ensure atomic creation
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			// Race condition - someone else got the lock
			return nil, errors.New("lock acquired by another process")
		}
		return nil, fmt.Errorf("failed to create lock file: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(path)
		return nil, fmt.Errorf("failed to write lock file: %w", err)
	}
	f.Close()

	// Return release function
	release := func() {
		os.Remove(path)
	}

	return release, nil
}

// TryAcquire attempts to acquire a lock without blocking.
func (m *FileLockManager) TryAcquire(ctx context.Context, repoRoot string, runID domain.RunID) (func(), bool, error) {
	release, err := m.Acquire(ctx, repoRoot, runID)
	if err != nil {
		// Check if it's a "lock held" error vs a real error
		if os.IsExist(err) || errors.Is(err, os.ErrExist) {
			return nil, false, nil
		}
		// For other lock-related errors, return not acquired
		if isLockHeldError(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return release, true, nil
}

// IsLocked checks if a lock is currently held.
func (m *FileLockManager) IsLocked(ctx context.Context, repoRoot string, runID domain.RunID) (bool, error) {
	path := lockPath(repoRoot)
	existing, err := m.readLock(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	// Check if lock is stale
	if time.Since(existing.AcquiredAt) > lockStaleDuration {
		return false, nil // Stale lock doesn't count
	}

	return true, nil
}

// GetLockInfo returns information about the current lock holder.
func (m *FileLockManager) GetLockInfo(repoRoot string) (*ports.LockInfo, error) {
	path := lockPath(repoRoot)
	existing, err := m.readLock(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	return &ports.LockInfo{
		RunID:      domain.RunID(existing.RunID),
		HolderPID:  existing.PID,
		AcquiredAt: existing.AcquiredAt.Format(time.RFC3339),
		Hostname:   existing.Hostname,
	}, nil
}

// readLock reads the lock file contents.
func (m *FileLockManager) readLock(path string) (*LockFileContents, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var lock LockFileContents
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}

	return &lock, nil
}

// isLockHeldError checks if an error indicates a lock is held.
func isLockHeldError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return len(msg) > 0 && (msg[0] == 'l' || msg[0] == 'L') // "lock held by..."
}
