package monorepo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A bump changes one value. These tests hold the rest of the file still — and cover the two
// defects the writers carried while nothing could reach them.

func TestPackageJSONKeepsItsOrderIndentAndOtherFields(t *testing.T) {
	original := `{
    "name": "api",
    "version": "1.0.0",
    "private": true,
    "scripts": {
        "build": "tsc"
    },
    "dependencies": {
        "left-pad": "1.3.0"
    }
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "package.json")
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := (&NPMVersionWriter{}).WriteVersion(context.Background(), dir, "1.1.0"); err != nil {
		t.Fatalf("WriteVersion: %v", err)
	}

	got := readFile(t, path)
	want := strings.Replace(original, `"version": "1.0.0"`, `"version": "1.1.0"`, 1)
	if got != want {
		t.Errorf("the manifest changed beyond its version.\n got:\n%s\nwant:\n%s\n"+
			"Decoding into a map and re-marshaling sorts keys alphabetically and reindents, "+
			"so every release would have rewritten the whole file", got, want)
	}
}

// A version inside dependencies is not the package's version.
func TestPackageJSONDoesNotTouchANestedVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "package.json")
	if err := os.WriteFile(path, []byte(`{"name":"api","dependencies":{"dep":{"version":"9.9.9"}},"version":"1.0.0"}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := (&NPMVersionWriter{}).WriteVersion(context.Background(), dir, "1.1.0"); err != nil {
		t.Fatalf("WriteVersion: %v", err)
	}

	got := readFile(t, path)
	if !strings.Contains(got, `"version":"9.9.9"`) {
		t.Errorf("the dependency's version was rewritten: %s", got)
	}
	if !strings.Contains(got, `"version":"1.1.0"`) {
		t.Errorf("the package's version was not written: %s", got)
	}
}

// The defect that would have corrupted a build: a dependency declared as its own table is a
// line beginning with `version =`, and the old whole-file regex wrote the package's version
// over it.
func TestCargoTomlVersionsOnlyThePackageTable(t *testing.T) {
	original := `[package]
name = "api"
version = "1.0.0"   # keep this comment

[dependencies.serde]
version = "1.0"
features = ["derive"]

[package.metadata.release]
version = "0.0.1"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "Cargo.toml")
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := (&CargoVersionWriter{}).WriteVersion(context.Background(), dir, "1.1.0"); err != nil {
		t.Fatalf("WriteVersion: %v", err)
	}

	got := readFile(t, path)
	if !strings.Contains(got, `version = "1.1.0"   # keep this comment`) {
		t.Errorf("the package version line did not survive intact:\n%s\n"+
			"Rebuilding the line deletes the trailing comment", got)
	}
	if !strings.Contains(got, "[dependencies.serde]\nversion = \"1.0\"") {
		t.Errorf("the serde dependency's version was overwritten with the package's:\n%s\n"+
			"That corrupts a file the build reads", got)
	}
	if !strings.Contains(got, "[package.metadata.release]\nversion = \"0.0.1\"") {
		t.Errorf("a nested table under [package] was treated as [package]:\n%s", got)
	}
	if !strings.Contains(got, `features = ["derive"]`) {
		t.Errorf("the rest of the file did not survive:\n%s", got)
	}
}

func TestCargoTomlWithNoPackageVersionIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"),
		[]byte("[workspace]\nmembers = [\"a\"]\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := (&CargoVersionWriter{}).WriteVersion(context.Background(), dir, "1.1.0")
	if err == nil {
		t.Fatal("a Cargo.toml with no [package] version was reported as written.\nThe package " +
			"would be tagged at a version its manifest never claimed")
	}
}

func TestPyprojectVersionsTheProjectNotItsDependencies(t *testing.T) {
	original := `[project]
name = "api"
version = "1.0.0"

[tool.poetry.dependencies]
requests = "2.0.0"

[tool.poetry.group.dev.dependencies.pytest]
version = "7.0.0"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "pyproject.toml")
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := (&PythonVersionWriter{}).WriteVersion(context.Background(), dir, "1.1.0"); err != nil {
		t.Fatalf("WriteVersion: %v", err)
	}

	got := readFile(t, path)
	if !strings.Contains(got, "[project]\nname = \"api\"\nversion = \"1.1.0\"") {
		t.Errorf("the project version was not written:\n%s", got)
	}
	if !strings.Contains(got, "version = \"7.0.0\"") {
		t.Errorf("pytest's pinned version was overwritten:\n%s", got)
	}
}

// The mode a manifest already has is the project's decision.
func TestAManifestKeepsItsPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "package.json")
	if err := os.WriteFile(path, []byte(`{"name":"api","version":"1.0.0"}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := (&NPMVersionWriter{}).WriteVersion(context.Background(), dir, "1.1.0"); err != nil {
		t.Fatalf("WriteVersion: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 600: a release widened who can read a manifest",
			info.Mode().Perm())
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(data)
}
