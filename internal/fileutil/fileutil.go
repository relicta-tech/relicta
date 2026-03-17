// Package fileutil provides shared file utilities for Relicta.
package fileutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type tempFile interface {
	Name() string
	Chmod(os.FileMode) error
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

type fsOps struct {
	createTemp func(dir, pattern string) (tempFile, error)
	rename     func(oldpath, newpath string) error
	remove     func(path string) error
}

func defaultFSOps() fsOps {
	return fsOps{
		createTemp: func(dir, pattern string) (tempFile, error) {
			return os.CreateTemp(dir, pattern)
		},
		rename: os.Rename,
		remove: os.Remove,
	}
}

// ReadFileLimited reads a file up to maxSize bytes.
// Returns an error if the file exceeds the maximum size.
// This prevents denial of service from maliciously crafted large files.
func ReadFileLimited(path string, maxSize int64) ([]byte, error) {
	f, err := os.Open(path) // #nosec G304 -- caller is responsible for path validation
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Check file size first
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > maxSize {
		return nil, fmt.Errorf("file size %d exceeds maximum allowed size %d", info.Size(), maxSize)
	}

	// Use LimitReader as an additional safety measure
	limitedReader := io.LimitReader(f, maxSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed size %d", maxSize)
	}

	return data, nil
}

// AtomicWriteFile writes data to a file atomically by writing to a temp file
// and then renaming it. This ensures the file is never in a partially written state.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	return atomicWriteFile(path, data, perm, defaultFSOps())
}

func atomicWriteFile(path string, data []byte, perm os.FileMode, ops fsOps) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	// Create temp file in same directory (required for atomic rename)
	tmpFile, err := ops.createTemp(dir, base+".tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file on any error
	defer func() {
		if tmpFile != nil {
			_ = tmpFile.Close()
			_ = ops.remove(tmpPath)
		}
	}()

	// Set permissions before writing
	if err := tmpFile.Chmod(perm); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Write data
	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	// Close before rename
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}
	tmpFile = nil // Prevent cleanup defer from closing/removing again

	// Atomic rename
	if err := ops.rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}
