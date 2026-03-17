package blast

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPythonPackage_Pyproject(t *testing.T) {
	dir := t.TempDir()
	pyproject := `[project]
name = "mypackage"
version = "1.2.3"
`
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &serviceImpl{}
	pkg := s.detectPythonPackage(dir, "pkg")
	if pkg == nil {
		t.Fatal("expected package, got nil")
	}
	if pkg.Name != "mypackage" {
		t.Errorf("name = %s, want mypackage", pkg.Name)
	}
	if pkg.Version != "1.2.3" {
		t.Errorf("version = %s, want 1.2.3", pkg.Version)
	}
	if pkg.Type != PackageTypePython {
		t.Errorf("type = %s, want python", pkg.Type)
	}
	if pkg.Path != "pkg" {
		t.Errorf("path = %s, want pkg", pkg.Path)
	}
}

func TestDetectPythonPackage_PyprojectMinimal(t *testing.T) {
	dir := t.TempDir()
	// pyproject.toml without [project] section
	pyproject := `[tool.setuptools]
packages = ["mylib"]
`
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &serviceImpl{}
	pkg := s.detectPythonPackage(dir, "libs/mylib")
	if pkg == nil {
		t.Fatal("expected package, got nil")
	}
	// Should use directory basename as name
	if pkg.Name != filepath.Base(dir) {
		t.Errorf("name = %s, want %s", pkg.Name, filepath.Base(dir))
	}
	if pkg.Version != "" {
		t.Errorf("version = %s, want empty", pkg.Version)
	}
}

func TestDetectPythonPackage_SetupPy(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "setup.py"), []byte("from setuptools import setup\nsetup()"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &serviceImpl{}
	pkg := s.detectPythonPackage(dir, ".")
	if pkg == nil {
		t.Fatal("expected package, got nil")
	}
	if pkg.Type != PackageTypePython {
		t.Errorf("type = %s, want python", pkg.Type)
	}
}

func TestDetectPythonPackage_NoPython(t *testing.T) {
	dir := t.TempDir()

	s := &serviceImpl{}
	pkg := s.detectPythonPackage(dir, ".")
	if pkg != nil {
		t.Errorf("expected nil, got %+v", pkg)
	}
}

func TestDetectPythonPackage_InvalidToml(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("not valid toml {{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Also add setup.py as fallback
	if err := os.WriteFile(filepath.Join(dir, "setup.py"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &serviceImpl{}
	pkg := s.detectPythonPackage(dir, ".")
	if pkg == nil {
		t.Fatal("expected fallback to setup.py")
	}
	if pkg.Type != PackageTypePython {
		t.Errorf("type = %s, want python", pkg.Type)
	}
}
