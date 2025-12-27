// Package fileutil provides shared file utilities for Relicta.
package fileutil

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

func TestReadFileLimited(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		maxSize     int64
		wantErr     bool
		errContains string
	}{
		{
			name:    "read small file",
			content: "hello world",
			maxSize: 100,
			wantErr: false,
		},
		{
			name:    "read file at exact limit",
			content: "12345",
			maxSize: 5,
			wantErr: false,
		},
		{
			name:        "file exceeds limit",
			content:     "this content is too long",
			maxSize:     10,
			wantErr:     true,
			errContains: "exceeds maximum",
		},
		{
			name:    "empty file",
			content: "",
			maxSize: 100,
			wantErr: false,
		},
		{
			name:    "file with newlines",
			content: "line1\nline2\nline3\n",
			maxSize: 100,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create temp file
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "test.txt")
			if err := os.WriteFile(filePath, []byte(tt.content), 0600); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			// Test read
			data, err := ReadFileLimited(filePath, tt.maxSize)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if string(data) != tt.content {
				t.Errorf("content mismatch: got %q, want %q", string(data), tt.content)
			}
		})
	}
}

func TestReadFileLimited_FileNotFound(t *testing.T) {
	t.Parallel()

	_, err := ReadFileLimited("/nonexistent/path/file.txt", 100)
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist error, got: %v", err)
	}
}

func TestReadFileLimited_Directory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	_, err := ReadFileLimited(tmpDir, 1024*1024)
	if err == nil {
		t.Error("expected error when reading directory, got nil")
	}
}

func TestReadFileLimited_PermissionDenied(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("permissions behave differently on windows")
	}

	// Skip if running as root since root can read any file regardless of permissions
	if os.Geteuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "secret.txt")
	if err := os.WriteFile(filePath, []byte("secret"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.Chmod(filePath, 0000); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}

	_, err := ReadFileLimited(filePath, 10)
	if err == nil {
		t.Fatal("expected permission error")
	}
}

func TestReadFileLimited_FIFOExceedsLimit(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("mkfifo not supported on windows")
	}

	tmpDir := t.TempDir()
	fifoPath := filepath.Join(tmpDir, "fifo")
	if err := syscall.Mkfifo(fifoPath, 0600); err != nil {
		t.Fatalf("failed to create fifo: %v", err)
	}

	maxSize := int64(5)
	done := make(chan error, 1)

	go func() {
		w, err := os.OpenFile(fifoPath, os.O_WRONLY, 0600)
		if err != nil {
			done <- err
			return
		}
		_, err = w.Write([]byte("1234567"))
		_ = w.Close()
		done <- err
	}()

	_, err := ReadFileLimited(fifoPath, maxSize)
	if err == nil {
		t.Fatal("expected size error for fifo content")
	}
	if werr := <-done; werr != nil {
		t.Fatalf("fifo writer error: %v", werr)
	}
}

func TestAtomicWriteFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content []byte
		perm    os.FileMode
	}{
		{
			name:    "write simple content",
			content: []byte("hello world"),
			perm:    0600,
		},
		{
			name:    "write empty file",
			content: []byte{},
			perm:    0600,
		},
		{
			name:    "write with different permissions",
			content: []byte("test content"),
			perm:    0644,
		},
		{
			name:    "write binary content",
			content: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE},
			perm:    0600,
		},
		{
			name:    "write large content",
			content: []byte(strings.Repeat("x", 1024*1024)), // 1MB
			perm:    0600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "test.txt")

			// Write file
			err := AtomicWriteFile(filePath, tt.content, tt.perm)
			if err != nil {
				t.Fatalf("AtomicWriteFile failed: %v", err)
			}

			// Verify content
			data, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("failed to read written file: %v", err)
			}

			if string(data) != string(tt.content) {
				t.Errorf("content mismatch: got %d bytes, want %d bytes", len(data), len(tt.content))
			}

			// Verify permissions
			info, err := os.Stat(filePath)
			if err != nil {
				t.Fatalf("failed to stat file: %v", err)
			}

			// On Unix, check permissions (masking out the type bits)
			gotPerm := info.Mode().Perm()
			if gotPerm != tt.perm {
				t.Errorf("permissions mismatch: got %o, want %o", gotPerm, tt.perm)
			}
		})
	}
}

func TestAtomicWriteFile_Overwrite(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	// Write initial content
	if err := AtomicWriteFile(filePath, []byte("initial"), 0600); err != nil {
		t.Fatalf("first write failed: %v", err)
	}

	// Overwrite with new content
	if err := AtomicWriteFile(filePath, []byte("updated"), 0600); err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	// Verify new content
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(data) != "updated" {
		t.Errorf("content not updated: got %q, want %q", string(data), "updated")
	}
}

func TestAtomicWriteFile_NoTempFileLeftOnSuccess(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	if err := AtomicWriteFile(filePath, []byte("content"), 0600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Check that no temp files remain
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 file, got %d", len(entries))
		for _, e := range entries {
			t.Logf("  file: %s", e.Name())
		}
	}

	if entries[0].Name() != "test.txt" {
		t.Errorf("unexpected file: %s", entries[0].Name())
	}
}

func TestAtomicWriteFile_InvalidDirectory(t *testing.T) {
	t.Parallel()

	err := AtomicWriteFile("/nonexistent/dir/file.txt", []byte("content"), 0600)
	if err == nil {
		t.Error("expected error for nonexistent directory, got nil")
	}
}

func TestAtomicWriteFile_RenameToExistingDirFails(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := AtomicWriteFile(tmpDir, []byte("content"), 0600); err == nil {
		t.Fatal("expected error when renaming over existing directory")
	}
}

func TestAtomicWriteFile_ConcurrentWrites(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	const numWriters = 10
	done := make(chan error, numWriters)

	for i := 0; i < numWriters; i++ {
		go func(id int) {
			content := []byte(strings.Repeat(string(rune('A'+id)), 100))
			done <- AtomicWriteFile(filePath, content, 0600)
		}(i)
	}

	// Wait for all writers
	for i := 0; i < numWriters; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent write failed: %v", err)
		}
	}

	// Verify file exists and has valid content from one of the writers
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read final file: %v", err)
	}

	if len(data) != 100 {
		t.Errorf("unexpected content length: %d", len(data))
	}

	// Content should be all the same character (from one writer)
	firstChar := data[0]
	for i, b := range data {
		if b != firstChar {
			t.Errorf("content corrupted at position %d: got %c, expected %c", i, b, firstChar)
			break
		}
	}
}

func TestReadFileLimited_Integration(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	content := "integration test content"

	// Write using atomic write
	if err := AtomicWriteFile(filePath, []byte(content), 0600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Read using limited read
	data, err := ReadFileLimited(filePath, 1024)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if string(data) != content {
		t.Errorf("content mismatch: got %q, want %q", string(data), content)
	}
}

type stubTempFile struct {
	name     string
	chmodErr error
	writeErr error
	syncErr  error
	closeErr error
}

func (s *stubTempFile) Name() string {
	return s.name
}

func (s *stubTempFile) Chmod(os.FileMode) error {
	return s.chmodErr
}

func (s *stubTempFile) Write(p []byte) (int, error) {
	if s.writeErr != nil {
		return 0, s.writeErr
	}
	return len(p), nil
}

func (s *stubTempFile) Sync() error {
	return s.syncErr
}

func (s *stubTempFile) Close() error {
	return s.closeErr
}

func TestAtomicWriteFile_CreateTempError(t *testing.T) {
	ops := fsOps{
		createTemp: func(dir, pattern string) (tempFile, error) {
			return nil, errors.New("boom")
		},
		rename: func(oldpath, newpath string) error { return nil },
		remove: func(path string) error { return nil },
	}

	err := atomicWriteFile("path.txt", []byte("data"), 0o600, ops)
	if err == nil || !strings.Contains(err.Error(), "failed to create temp file") {
		t.Fatalf("expected create temp error, got %v", err)
	}
}

func TestAtomicWriteFile_ChmodError(t *testing.T) {
	tmp := &stubTempFile{
		name:     "temp",
		chmodErr: errors.New("chmod fail"),
	}
	ops := fsOps{
		createTemp: func(dir, pattern string) (tempFile, error) { return tmp, nil },
		rename:     func(oldpath, newpath string) error { return nil },
		remove:     func(path string) error { return nil },
	}

	err := atomicWriteFile("path.txt", []byte("data"), 0o600, ops)
	if err == nil || !strings.Contains(err.Error(), "failed to set file permissions") {
		t.Fatalf("expected chmod error, got %v", err)
	}
}

func TestAtomicWriteFile_WriteError(t *testing.T) {
	tmp := &stubTempFile{
		name:     "temp",
		writeErr: errors.New("write fail"),
	}
	ops := fsOps{
		createTemp: func(dir, pattern string) (tempFile, error) { return tmp, nil },
		rename:     func(oldpath, newpath string) error { return nil },
		remove:     func(path string) error { return nil },
	}

	err := atomicWriteFile("path.txt", []byte("data"), 0o600, ops)
	if err == nil || !strings.Contains(err.Error(), "failed to write data") {
		t.Fatalf("expected write error, got %v", err)
	}
}

func TestAtomicWriteFile_SyncError(t *testing.T) {
	tmp := &stubTempFile{
		name:    "temp",
		syncErr: errors.New("sync fail"),
	}
	ops := fsOps{
		createTemp: func(dir, pattern string) (tempFile, error) { return tmp, nil },
		rename:     func(oldpath, newpath string) error { return nil },
		remove:     func(path string) error { return nil },
	}

	err := atomicWriteFile("path.txt", []byte("data"), 0o600, ops)
	if err == nil || !strings.Contains(err.Error(), "failed to sync file") {
		t.Fatalf("expected sync error, got %v", err)
	}
}

func TestAtomicWriteFile_CloseError(t *testing.T) {
	tmp := &stubTempFile{
		name:     "temp",
		closeErr: errors.New("close fail"),
	}
	ops := fsOps{
		createTemp: func(dir, pattern string) (tempFile, error) { return tmp, nil },
		rename:     func(oldpath, newpath string) error { return nil },
		remove:     func(path string) error { return nil },
	}

	err := atomicWriteFile("path.txt", []byte("data"), 0o600, ops)
	if err == nil || !strings.Contains(err.Error(), "failed to close file") {
		t.Fatalf("expected close error, got %v", err)
	}
}

func TestAtomicWriteFile_RenameError(t *testing.T) {
	tmp := &stubTempFile{name: "temp"}
	ops := fsOps{
		createTemp: func(dir, pattern string) (tempFile, error) { return tmp, nil },
		rename:     func(oldpath, newpath string) error { return errors.New("rename fail") },
		remove:     func(path string) error { return nil },
	}

	err := atomicWriteFile("path.txt", []byte("data"), 0o600, ops)
	if err == nil || !strings.Contains(err.Error(), "failed to rename temp file") {
		t.Fatalf("expected rename error, got %v", err)
	}
}
