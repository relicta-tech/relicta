package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/release"
)

func newTestRepository(t *testing.T) *FileReleaseRepository {
	t.Helper()
	tmpDir := t.TempDir()
	repo, err := NewFileReleaseRepository(tmpDir)
	if err != nil {
		t.Fatalf("failed to create release repository: %v", err)
	}
	return repo
}

func writeReleaseDTO(t *testing.T, repo *FileReleaseRepository, id string) string {
	t.Helper()
	now := time.Now().UTC()
	dto := releaseDTO{
		ID:             id,
		State:          string(release.StatePlanned),
		Branch:         "main",
		RepositoryPath: "/tmp/repo",
		RepositoryName: "repo",
		TagName:        "v1.0.0",
		CreatedAt:      now.Format(time.RFC3339),
		UpdatedAt:      now.Format(time.RFC3339),
	}

	data, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("failed to marshal dto: %v", err)
	}

	path := filepath.Join(repo.basePath, id+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("failed to write release file: %v", err)
	}
	return path
}

func TestCheckContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := checkContext(ctx); err == nil {
		t.Fatal("expected error when context canceled")
	}
}

func TestReleaseFilePath(t *testing.T) {
	repo := newTestRepository(t)
	id := release.RunID("test")
	want := filepath.Join(repo.basePath, "test.json")
	if got := repo.releaseFilePath(id); got != want {
		t.Fatalf("releaseFilePath = %q, want %q", got, want)
	}
}

func TestScanSingleFile(t *testing.T) {
	repo := newTestRepository(t)
	path := writeReleaseDTO(t, repo, "release1")

	rel, err := repo.scanSingleFile(path, func(_ *releaseDTO) bool { return true })
	if err != nil {
		t.Fatalf("scanSingleFile returned error: %v", err)
	}
	if rel == nil {
		t.Fatal("scanSingleFile should return release when filter accepts")
	}
}

func TestScanSingleFile_FilterRejects(t *testing.T) {
	repo := newTestRepository(t)
	path := writeReleaseDTO(t, repo, "release-filter")

	rel, err := repo.scanSingleFile(path, func(_ *releaseDTO) bool { return false })
	if err != nil {
		t.Fatalf("scanSingleFile error = %v", err)
	}
	if rel != nil {
		t.Fatalf("expected nil release when filter rejects, got %v", rel)
	}
}

func TestScanReleasesSequential(t *testing.T) {
	repo := newTestRepository(t)
	for i := 0; i < 2; i++ {
		writeReleaseDTO(t, repo, fmt.Sprintf("seq%d", i))
	}

	list, err := repo.scanReleases(context.Background(), func(*releaseDTO) bool { return true })
	if err != nil {
		t.Fatalf("scanReleases returned error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(list))
	}
}

func TestScanReleasesConcurrent(t *testing.T) {
	repo := newTestRepository(t)
	count := maxScanWorkers * 2
	for i := 0; i < count; i++ {
		writeReleaseDTO(t, repo, fmt.Sprintf("concurrent%d", i))
	}

	list, err := repo.scanReleases(context.Background(), func(*releaseDTO) bool { return true })
	if err != nil {
		t.Fatalf("scanReleases concurrent error: %v", err)
	}
	if len(list) != count {
		t.Fatalf("expected %d releases, got %d", count, len(list))
	}
}
