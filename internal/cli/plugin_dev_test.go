package cli

import (
	"path/filepath"
	"testing"
)

func TestRunPluginDevNoGoMod(t *testing.T) {
	tmpDir := t.TempDir()

	if err := runPluginDev(nil, []string{tmpDir}); err == nil {
		t.Fatal("expected error when go.mod is missing")
	}
}

func TestBuildPluginMissingSource(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	dest := filepath.Join(t.TempDir(), "plugin-bin")

	if err := buildPlugin(missing, dest); err == nil {
		t.Fatal("expected buildPlugin to fail with missing source")
	}
}

func TestWatchAndRebuildMissingSource(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	dest := filepath.Join(t.TempDir(), "plugin-bin")

	if err := watchAndRebuild(missing, dest, "plugin"); err == nil {
		t.Fatal("expected watchAndRebuild to fail with missing source")
	}
}
