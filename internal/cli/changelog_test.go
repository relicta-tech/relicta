package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/internal/config"
)

func TestUpdateChangelogFileInsertsContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")
	initial := "# Changelog\n\n## [1.0.0]\n- previous change"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial file failed: %v", err)
	}

	newContent := "## [Unreleased]\n- upcoming change"
	if err := updateChangelogFile(path, newContent); err != nil {
		t.Fatalf("updateChangelogFile error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	content := string(data)

	if strings.Index(content, "## [Unreleased]") > strings.Index(content, "## [1.0.0]") {
		t.Fatal("expected new content before existing entries")
	}
}

func TestUpdateChangelogFileCreatesHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")
	if err := updateChangelogFile(path, "## [Unreleased]\n- init"); err != nil {
		t.Fatalf("updateChangelogFile error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}

	if !strings.Contains(string(data), "# Changelog") {
		t.Fatal("expected header in new file")
	}
}

func TestHandleChangelogUpdateWritesNotes(t *testing.T) {
	origCfg := cfg
	t.Cleanup(func() { cfg = origCfg })
	cfg = config.DefaultConfig()
	dir := t.TempDir()
	cfg.Changelog.File = filepath.Join(dir, "CHANGELOG.md")

	rel := newTestRelease(t, "changelog-updates")
	handleChangelogUpdate(rel)

	data, err := os.ReadFile(cfg.Changelog.File)
	if err != nil {
		t.Fatalf("read changelog failed: %v", err)
	}
	if !strings.Contains(string(data), rel.Notes().Changelog) {
		t.Fatal("expected notes content in changelog")
	}
}
