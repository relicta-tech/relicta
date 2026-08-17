package config

import (
	"os"
	"path/filepath"
	"testing"
)

// LoadFromDirectory was NewLoader().WithSearchPaths(dir), which does not load from dir: NewLoader
// seeds the search path with "." and WithSearchPaths appends, so the process working directory
// was searched first and won.
//
// Every caller names a directory precisely because it is not the working directory — a group
// member's checkout, a repository the dashboard server was asked about — so each of them got the
// invoking repository's configuration instead, silently.
//
// Verified against the shipped binary: a group member whose approved run was in SQLite while the
// calling repository used files was reported as "no release has been planned — run 'relicta
// plan'", which the operator had already done.

func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".relicta.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestTheNamedDirectoryWinsOverTheWorkingDirectory(t *testing.T) {
	working := t.TempDir()
	named := t.TempDir()

	writeConfig(t, working, "persistence:\n  backend: file\n")
	writeConfig(t, named, "persistence:\n  backend: sqlite\n")

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(working); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	cfg, err := LoadFromDirectory(named)
	if err != nil {
		t.Fatalf("LoadFromDirectory: %v", err)
	}

	if cfg.Persistence.Backend != BackendSQLite {
		t.Errorf("backend = %q, want %q.\nThe configuration came from the working directory "+
			"instead of the directory named, so every caller asking about another repository "+
			"was answered about its own", cfg.Persistence.Backend, BackendSQLite)
	}
}

// A repository that has never been configured has always run on defaults, and asking about one
// must not be an error — a group may legitimately contain a member with no .relicta.yaml.
func TestADirectoryWithNoConfigLoadsDefaults(t *testing.T) {
	cfg, err := LoadFromDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("LoadFromDirectory on an unconfigured directory: %v", err)
	}
	if cfg.Persistence.Backend != BackendFile {
		t.Errorf("backend = %q, want the default %q", cfg.Persistence.Backend, BackendFile)
	}
}

// The working directory must not leak in even when the named directory has no config of its own:
// falling back to the caller's settings is the same defect wearing a different hat.
func TestAnUnconfiguredDirectoryDoesNotInheritTheWorkingDirectory(t *testing.T) {
	working := t.TempDir()
	writeConfig(t, working, "persistence:\n  backend: sqlite\n")

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(working); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	cfg, err := LoadFromDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("LoadFromDirectory: %v", err)
	}
	if cfg.Persistence.Backend != BackendFile {
		t.Errorf("backend = %q, want the default %q: an unconfigured repository picked up the "+
			"invoking one's store", cfg.Persistence.Backend, BackendFile)
	}
}
