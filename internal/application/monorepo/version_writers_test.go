package monorepo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/monorepo"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestNPMVersionWriter(t *testing.T) {
	w := &NPMVersionWriter{}

	// Test CanHandle
	if !w.CanHandle(monorepo.PackageTypeNPM) {
		t.Error("CanHandle should return true for npm")
	}
	if w.CanHandle(monorepo.PackageTypeCargo) {
		t.Error("CanHandle should return false for cargo")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Write test package.json
	pkgJSON := `{
  "name": "test-package",
  "version": "1.2.3",
  "description": "Test package"
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	// Test ReadVersion
	ctx := context.Background()
	ver, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion failed: %v", err)
	}
	if ver != "1.2.3" {
		t.Errorf("ReadVersion = %s, want 1.2.3", ver)
	}

	// Test WriteVersion
	if err := w.WriteVersion(ctx, tmpDir, "2.0.0"); err != nil {
		t.Fatalf("WriteVersion failed: %v", err)
	}

	// Verify new version
	newVer, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion after write failed: %v", err)
	}
	if newVer != "2.0.0" {
		t.Errorf("ReadVersion after write = %s, want 2.0.0", newVer)
	}

	// Test Files
	files := w.Files(tmpDir)
	if len(files) != 1 {
		t.Errorf("Files() returned %d files, want 1", len(files))
	}
}

func TestCargoVersionWriter(t *testing.T) {
	w := &CargoVersionWriter{}

	// Test CanHandle
	if !w.CanHandle(monorepo.PackageTypeCargo) {
		t.Error("CanHandle should return true for cargo")
	}

	tmpDir := t.TempDir()

	// Write test Cargo.toml
	cargoToml := `[package]
name = "test-crate"
version = "1.2.3"
edition = "2021"

[dependencies]
serde = "1.0"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(cargoToml), 0644); err != nil {
		t.Fatalf("Failed to write Cargo.toml: %v", err)
	}

	ctx := context.Background()

	// Test ReadVersion
	ver, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion failed: %v", err)
	}
	if ver != "1.2.3" {
		t.Errorf("ReadVersion = %s, want 1.2.3", ver)
	}

	// Test WriteVersion
	if err := w.WriteVersion(ctx, tmpDir, "2.0.0"); err != nil {
		t.Fatalf("WriteVersion failed: %v", err)
	}

	// Verify new version
	newVer, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion after write failed: %v", err)
	}
	if newVer != "2.0.0" {
		t.Errorf("ReadVersion after write = %s, want 2.0.0", newVer)
	}
}

func TestPythonVersionWriter(t *testing.T) {
	w := &PythonVersionWriter{}

	if !w.CanHandle(monorepo.PackageTypePython) {
		t.Error("CanHandle should return true for python")
	}

	t.Run("pyproject.toml", func(t *testing.T) {
		tmpDir := t.TempDir()

		pyproject := `[project]
name = "test-package"
version = "1.2.3"
description = "Test"
`
		if err := os.WriteFile(filepath.Join(tmpDir, "pyproject.toml"), []byte(pyproject), 0644); err != nil {
			t.Fatalf("Failed to write pyproject.toml: %v", err)
		}

		ctx := context.Background()
		ver, err := w.ReadVersion(ctx, tmpDir)
		if err != nil {
			t.Fatalf("ReadVersion failed: %v", err)
		}
		if ver != "1.2.3" {
			t.Errorf("ReadVersion = %s, want 1.2.3", ver)
		}

		if err := w.WriteVersion(ctx, tmpDir, "2.0.0"); err != nil {
			t.Fatalf("WriteVersion failed: %v", err)
		}

		newVer, err := w.ReadVersion(ctx, tmpDir)
		if err != nil {
			t.Fatalf("ReadVersion after write failed: %v", err)
		}
		if newVer != "2.0.0" {
			t.Errorf("ReadVersion after write = %s, want 2.0.0", newVer)
		}
	})
}

func TestGoModuleVersionWriter(t *testing.T) {
	w := &GoModuleVersionWriter{}

	if !w.CanHandle(monorepo.PackageTypeGoModule) {
		t.Error("CanHandle should return true for go_module")
	}

	tmpDir := t.TempDir()

	versionGo := `package main

const Version = "1.2.3"

func main() {}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "version.go"), []byte(versionGo), 0644); err != nil {
		t.Fatalf("Failed to write version.go: %v", err)
	}

	ctx := context.Background()
	ver, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion failed: %v", err)
	}
	if ver != "1.2.3" {
		t.Errorf("ReadVersion = %s, want 1.2.3", ver)
	}

	if err := w.WriteVersion(ctx, tmpDir, "2.0.0"); err != nil {
		t.Fatalf("WriteVersion failed: %v", err)
	}

	newVer, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion after write failed: %v", err)
	}
	if newVer != "2.0.0" {
		t.Errorf("ReadVersion after write = %s, want 2.0.0", newVer)
	}
}

func TestComposerVersionWriter(t *testing.T) {
	w := &ComposerVersionWriter{}

	if !w.CanHandle(monorepo.PackageTypeComposer) {
		t.Error("CanHandle should return true for composer")
	}

	tmpDir := t.TempDir()

	composerJSON := `{
    "name": "vendor/package",
    "version": "1.2.3",
    "type": "library"
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "composer.json"), []byte(composerJSON), 0644); err != nil {
		t.Fatalf("Failed to write composer.json: %v", err)
	}

	ctx := context.Background()
	ver, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion failed: %v", err)
	}
	if ver != "1.2.3" {
		t.Errorf("ReadVersion = %s, want 1.2.3", ver)
	}

	if err := w.WriteVersion(ctx, tmpDir, "2.0.0"); err != nil {
		t.Fatalf("WriteVersion failed: %v", err)
	}

	newVer, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion after write failed: %v", err)
	}
	if newVer != "2.0.0" {
		t.Errorf("ReadVersion after write = %s, want 2.0.0", newVer)
	}
}

func TestDirectoryVersionWriter(t *testing.T) {
	w := &DirectoryVersionWriter{}

	if !w.CanHandle(monorepo.PackageTypeDirectory) {
		t.Error("CanHandle should return true for directory")
	}

	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "VERSION"), []byte("1.2.3\n"), 0644); err != nil {
		t.Fatalf("Failed to write VERSION: %v", err)
	}

	ctx := context.Background()
	ver, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion failed: %v", err)
	}
	if ver != "1.2.3" {
		t.Errorf("ReadVersion = %s, want 1.2.3", ver)
	}

	if err := w.WriteVersion(ctx, tmpDir, "2.0.0"); err != nil {
		t.Fatalf("WriteVersion failed: %v", err)
	}

	newVer, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion after write failed: %v", err)
	}
	if newVer != "2.0.0" {
		t.Errorf("ReadVersion after write = %s, want 2.0.0", newVer)
	}
}

func TestVersionWriterRegistry(t *testing.T) {
	registry := NewVersionWriterRegistry()

	tests := []struct {
		pkgType monorepo.PackageType
		want    bool
	}{
		{monorepo.PackageTypeNPM, true},
		{monorepo.PackageTypeCargo, true},
		{monorepo.PackageTypePython, true},
		{monorepo.PackageTypeGoModule, true},
		{monorepo.PackageTypeMaven, true},
		{monorepo.PackageTypeGradle, true},
		{monorepo.PackageTypeComposer, true},
		{monorepo.PackageTypeGem, true},
		{monorepo.PackageTypeNuGet, true},
		{monorepo.PackageTypeDirectory, true},
		{monorepo.PackageType("unknown"), false},
	}

	for _, tt := range tests {
		_, ok := registry.GetWriter(tt.pkgType)
		if ok != tt.want {
			t.Errorf("GetWriter(%s) = %v, want %v", tt.pkgType, ok, tt.want)
		}
	}
}

func TestCompositeVersionWriter(t *testing.T) {
	w := NewCompositeVersionWriter()

	// Test CanHandle
	if !w.CanHandle(monorepo.PackageTypeNPM) {
		t.Error("CanHandle should return true for npm")
	}
	if w.CanHandle(monorepo.PackageType("unknown")) {
		t.Error("CanHandle should return false for unknown type")
	}

	// Test with NPM package
	tmpDir := t.TempDir()
	pkgJSON := `{
  "name": "test-package",
  "version": "1.2.3"
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	ctx := context.Background()
	ver, err := w.ReadVersion(ctx, tmpDir, monorepo.PackageTypeNPM)
	if err != nil {
		t.Fatalf("ReadVersion failed: %v", err)
	}
	if ver.String() != "1.2.3" {
		t.Errorf("ReadVersion = %s, want 1.2.3", ver.String())
	}
}

func TestGradleVersionWriter(t *testing.T) {
	w := &GradleVersionWriter{}

	if !w.CanHandle(monorepo.PackageTypeGradle) {
		t.Error("CanHandle should return true for gradle")
	}

	t.Run("build.gradle.kts", func(t *testing.T) {
		tmpDir := t.TempDir()

		buildGradle := `plugins {
    kotlin("jvm") version "1.9.0"
}

group = "com.example"
version = "1.2.3"

repositories {
    mavenCentral()
}
`
		if err := os.WriteFile(filepath.Join(tmpDir, "build.gradle.kts"), []byte(buildGradle), 0644); err != nil {
			t.Fatalf("Failed to write build.gradle.kts: %v", err)
		}

		ctx := context.Background()
		ver, err := w.ReadVersion(ctx, tmpDir)
		if err != nil {
			t.Fatalf("ReadVersion failed: %v", err)
		}
		if ver != "1.2.3" {
			t.Errorf("ReadVersion = %s, want 1.2.3", ver)
		}

		if err := w.WriteVersion(ctx, tmpDir, "2.0.0"); err != nil {
			t.Fatalf("WriteVersion failed: %v", err)
		}

		newVer, err := w.ReadVersion(ctx, tmpDir)
		if err != nil {
			t.Fatalf("ReadVersion after write failed: %v", err)
		}
		if newVer != "2.0.0" {
			t.Errorf("ReadVersion after write = %s, want 2.0.0", newVer)
		}
	})
}

func TestMavenVersionWriter(t *testing.T) {
	w := &MavenVersionWriter{}

	if !w.CanHandle(monorepo.PackageTypeMaven) {
		t.Error("CanHandle should return true for maven")
	}

	tmpDir := t.TempDir()

	pomXML := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>my-app</artifactId>
    <version>1.2.3</version>
</project>
`
	if err := os.WriteFile(filepath.Join(tmpDir, "pom.xml"), []byte(pomXML), 0644); err != nil {
		t.Fatalf("Failed to write pom.xml: %v", err)
	}

	ctx := context.Background()
	ver, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion failed: %v", err)
	}
	if ver != "1.2.3" {
		t.Errorf("ReadVersion = %s, want 1.2.3", ver)
	}

	if err := w.WriteVersion(ctx, tmpDir, "2.0.0"); err != nil {
		t.Fatalf("WriteVersion failed: %v", err)
	}

	newVer, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion after write failed: %v", err)
	}
	if newVer != "2.0.0" {
		t.Errorf("ReadVersion after write = %s, want 2.0.0", newVer)
	}
}

func TestNuGetVersionWriter(t *testing.T) {
	w := &NuGetVersionWriter{}

	if !w.CanHandle(monorepo.PackageTypeNuGet) {
		t.Error("CanHandle should return true for nuget")
	}

	tmpDir := t.TempDir()

	csproj := `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <Version>1.2.3</Version>
  </PropertyGroup>
</Project>
`
	if err := os.WriteFile(filepath.Join(tmpDir, "MyPackage.csproj"), []byte(csproj), 0644); err != nil {
		t.Fatalf("Failed to write .csproj: %v", err)
	}

	ctx := context.Background()
	ver, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion failed: %v", err)
	}
	if ver != "1.2.3" {
		t.Errorf("ReadVersion = %s, want 1.2.3", ver)
	}

	if err := w.WriteVersion(ctx, tmpDir, "2.0.0"); err != nil {
		t.Fatalf("WriteVersion failed: %v", err)
	}

	newVer, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion after write failed: %v", err)
	}
	if newVer != "2.0.0" {
		t.Errorf("ReadVersion after write = %s, want 2.0.0", newVer)
	}
}

func TestGemVersionWriter(t *testing.T) {
	w := &GemVersionWriter{}

	if !w.CanHandle(monorepo.PackageTypeGem) {
		t.Error("CanHandle should return true for gem")
	}

	tmpDir := t.TempDir()

	// Create lib/mygem/version.rb structure
	libDir := filepath.Join(tmpDir, "lib", "mygem")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatalf("Failed to create lib directory: %v", err)
	}

	versionRb := `# frozen_string_literal: true

module MyGem
  VERSION = "1.2.3"
end
`
	if err := os.WriteFile(filepath.Join(libDir, "version.rb"), []byte(versionRb), 0644); err != nil {
		t.Fatalf("Failed to write version.rb: %v", err)
	}

	ctx := context.Background()
	ver, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion failed: %v", err)
	}
	if ver != "1.2.3" {
		t.Errorf("ReadVersion = %s, want 1.2.3", ver)
	}

	if err := w.WriteVersion(ctx, tmpDir, "2.0.0"); err != nil {
		t.Fatalf("WriteVersion failed: %v", err)
	}

	newVer, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion after write failed: %v", err)
	}
	if newVer != "2.0.0" {
		t.Errorf("ReadVersion after write = %s, want 2.0.0", newVer)
	}
}

func TestCargoVersionWriter_Files(t *testing.T) {
	w := &CargoVersionWriter{}
	tmpDir := t.TempDir()

	// Create Cargo.toml
	if err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte("[package]\nversion = \"1.0.0\""), 0644); err != nil {
		t.Fatalf("Failed to write Cargo.toml: %v", err)
	}

	files := w.Files(tmpDir)
	if len(files) != 1 {
		t.Errorf("Files() returned %d files, want 1", len(files))
	}
	if files[0] != filepath.Join(tmpDir, "Cargo.toml") {
		t.Errorf("Files()[0] = %s, want Cargo.toml path", files[0])
	}
}

func TestPythonVersionWriter_Files(t *testing.T) {
	w := &PythonVersionWriter{}
	tmpDir := t.TempDir()

	// Create pyproject.toml
	if err := os.WriteFile(filepath.Join(tmpDir, "pyproject.toml"), []byte("[project]\nversion = \"1.0.0\""), 0644); err != nil {
		t.Fatalf("Failed to write pyproject.toml: %v", err)
	}

	files := w.Files(tmpDir)
	if len(files) < 1 {
		t.Errorf("Files() returned %d files, want at least 1", len(files))
	}
}

func TestGoModuleVersionWriter_Files(t *testing.T) {
	w := &GoModuleVersionWriter{}
	tmpDir := t.TempDir()

	// Create version.go
	if err := os.WriteFile(filepath.Join(tmpDir, "version.go"), []byte("package main\nconst Version = \"1.0.0\""), 0644); err != nil {
		t.Fatalf("Failed to write version.go: %v", err)
	}

	files := w.Files(tmpDir)
	if len(files) < 1 {
		t.Errorf("Files() returned %d files, want at least 1", len(files))
	}
}

func TestNPMVersionWriter_ReadVersion_NoFile(t *testing.T) {
	w := &NPMVersionWriter{}
	tmpDir := t.TempDir()

	ctx := context.Background()
	_, err := w.ReadVersion(ctx, tmpDir)
	if err == nil {
		t.Error("ReadVersion should fail when package.json doesn't exist")
	}
}

func TestCargoVersionWriter_ReadVersion_NoFile(t *testing.T) {
	w := &CargoVersionWriter{}
	tmpDir := t.TempDir()

	ctx := context.Background()
	_, err := w.ReadVersion(ctx, tmpDir)
	if err == nil {
		t.Error("ReadVersion should fail when Cargo.toml doesn't exist")
	}
}

func TestPythonVersionWriter_ReadVersion_NoFile(t *testing.T) {
	w := &PythonVersionWriter{}
	tmpDir := t.TempDir()

	ctx := context.Background()
	_, err := w.ReadVersion(ctx, tmpDir)
	if err == nil {
		t.Error("ReadVersion should fail when no Python version files exist")
	}
}

func TestGoModuleVersionWriter_ReadVersion_NoFile(t *testing.T) {
	w := &GoModuleVersionWriter{}
	tmpDir := t.TempDir()

	ctx := context.Background()
	_, err := w.ReadVersion(ctx, tmpDir)
	if err == nil {
		t.Error("ReadVersion should fail when version.go doesn't exist")
	}
}

func TestDirectoryVersionWriter_Files(t *testing.T) {
	w := &DirectoryVersionWriter{}
	tmpDir := t.TempDir()

	// Create VERSION file
	if err := os.WriteFile(filepath.Join(tmpDir, "VERSION"), []byte("1.0.0"), 0644); err != nil {
		t.Fatalf("Failed to write VERSION: %v", err)
	}

	files := w.Files(tmpDir)
	if len(files) != 1 {
		t.Errorf("Files() returned %d files, want 1", len(files))
	}
}

func TestPythonVersionWriter_SetupPy(t *testing.T) {
	w := &PythonVersionWriter{}
	tmpDir := t.TempDir()

	setupPy := `from setuptools import setup

setup(
    name="mypackage",
    version="1.2.3",
    packages=["mypackage"],
)
`
	if err := os.WriteFile(filepath.Join(tmpDir, "setup.py"), []byte(setupPy), 0644); err != nil {
		t.Fatalf("Failed to write setup.py: %v", err)
	}

	ctx := context.Background()
	ver, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion failed: %v", err)
	}
	if ver != "1.2.3" {
		t.Errorf("ReadVersion = %s, want 1.2.3", ver)
	}
}

func TestGradleVersionWriter_BuildGradle(t *testing.T) {
	w := &GradleVersionWriter{}
	tmpDir := t.TempDir()

	// Test with build.gradle (Groovy DSL)
	buildGradle := `plugins {
    id 'java'
}

group = 'com.example'
version = '1.2.3'

repositories {
    mavenCentral()
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "build.gradle"), []byte(buildGradle), 0644); err != nil {
		t.Fatalf("Failed to write build.gradle: %v", err)
	}

	ctx := context.Background()
	ver, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion failed: %v", err)
	}
	if ver != "1.2.3" {
		t.Errorf("ReadVersion = %s, want 1.2.3", ver)
	}
}

func TestGradleVersionWriter_WriteVersion_BuildGradle(t *testing.T) {
	w := &GradleVersionWriter{}
	tmpDir := t.TempDir()

	buildGradle := `plugins {
    id 'java'
}

group = 'com.example'
version = '1.2.3'
`
	if err := os.WriteFile(filepath.Join(tmpDir, "build.gradle"), []byte(buildGradle), 0644); err != nil {
		t.Fatalf("Failed to write build.gradle: %v", err)
	}

	ctx := context.Background()
	if err := w.WriteVersion(ctx, tmpDir, "2.0.0"); err != nil {
		t.Fatalf("WriteVersion failed: %v", err)
	}

	newVer, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion after write failed: %v", err)
	}
	if newVer != "2.0.0" {
		t.Errorf("ReadVersion after write = %s, want 2.0.0", newVer)
	}
}

func TestPythonVersionWriter_SetupPyWrite(t *testing.T) {
	w := &PythonVersionWriter{}
	tmpDir := t.TempDir()

	setupPy := `from setuptools import setup

setup(
    name="mypackage",
    version="1.2.3",
    packages=["mypackage"],
)
`
	if err := os.WriteFile(filepath.Join(tmpDir, "setup.py"), []byte(setupPy), 0644); err != nil {
		t.Fatalf("Failed to write setup.py: %v", err)
	}

	ctx := context.Background()
	if err := w.WriteVersion(ctx, tmpDir, "2.0.0"); err != nil {
		t.Fatalf("WriteVersion failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "setup.py"))
	if err != nil {
		t.Fatalf("Read setup.py failed: %v", err)
	}
	if !strings.Contains(string(data), "2.0.0") {
		t.Errorf("setup.py should contain 2.0.0, got %s", string(data))
	}
}

func TestPythonVersionWriter_VersionPyWrite(t *testing.T) {
	w := &PythonVersionWriter{}
	tmpDir := t.TempDir()

	versionPy := `__version__ = "1.2.3"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "__version__.py"), []byte(versionPy), 0644); err != nil {
		t.Fatalf("Failed to write __version__.py: %v", err)
	}

	ctx := context.Background()
	ver, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion failed: %v", err)
	}
	if ver != "1.2.3" {
		t.Errorf("ReadVersion = %s, want 1.2.3", ver)
	}

	if err := w.WriteVersion(ctx, tmpDir, "3.0.0"); err != nil {
		t.Fatalf("WriteVersion failed: %v", err)
	}

	newVer, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion after write failed: %v", err)
	}
	if newVer != "3.0.0" {
		t.Errorf("ReadVersion after write = %s, want 3.0.0", newVer)
	}
}

func TestPythonVersionWriter_WriteNoFiles(t *testing.T) {
	w := &PythonVersionWriter{}
	tmpDir := t.TempDir()

	ctx := context.Background()
	err := w.WriteVersion(ctx, tmpDir, "1.0.0")
	if err == nil {
		t.Error("WriteVersion should fail when no Python version files exist")
	}
}

func TestGradleVersionWriter_Files(t *testing.T) {
	w := &GradleVersionWriter{}

	t.Run("with build.gradle.kts", func(t *testing.T) {
		tmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmpDir, "build.gradle.kts"), []byte(`version = "1.0.0"`), 0644); err != nil {
			t.Fatalf("Failed to write build.gradle.kts: %v", err)
		}
		files := w.Files(tmpDir)
		if len(files) != 1 {
			t.Errorf("Files() returned %d files, want 1", len(files))
		}
	})

	t.Run("with build.gradle", func(t *testing.T) {
		tmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmpDir, "build.gradle"), []byte(`version = '1.0.0'`), 0644); err != nil {
			t.Fatalf("Failed to write build.gradle: %v", err)
		}
		files := w.Files(tmpDir)
		if len(files) != 1 {
			t.Errorf("Files() returned %d files, want 1", len(files))
		}
	})

	t.Run("no gradle file", func(t *testing.T) {
		tmpDir := t.TempDir()
		files := w.Files(tmpDir)
		if files != nil {
			t.Errorf("Files() returned %v, want nil", files)
		}
	})
}

func TestMavenVersionWriter_Files(t *testing.T) {
	w := &MavenVersionWriter{}
	tmpDir := t.TempDir()
	files := w.Files(tmpDir)
	if len(files) != 1 {
		t.Errorf("Files() returned %d files, want 1", len(files))
	}
	expected := filepath.Join(tmpDir, "pom.xml")
	if files[0] != expected {
		t.Errorf("Files()[0] = %s, want %s", files[0], expected)
	}
}

func TestComposerVersionWriter_Files(t *testing.T) {
	w := &ComposerVersionWriter{}
	tmpDir := t.TempDir()
	files := w.Files(tmpDir)
	if len(files) != 1 {
		t.Errorf("Files() returned %d files, want 1", len(files))
	}
	expected := filepath.Join(tmpDir, "composer.json")
	if files[0] != expected {
		t.Errorf("Files()[0] = %s, want %s", files[0], expected)
	}
}

func TestGemVersionWriter_Files(t *testing.T) {
	w := &GemVersionWriter{}

	t.Run("with version.rb", func(t *testing.T) {
		tmpDir := t.TempDir()
		libDir := filepath.Join(tmpDir, "lib", "mygem")
		if err := os.MkdirAll(libDir, 0755); err != nil {
			t.Fatalf("Failed to create lib directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(libDir, "version.rb"), []byte(`VERSION = "1.0.0"`), 0644); err != nil {
			t.Fatalf("Failed to write version.rb: %v", err)
		}

		files := w.Files(tmpDir)
		if len(files) != 1 {
			t.Errorf("Files() returned %d files, want 1", len(files))
		}
	})

	t.Run("no version files", func(t *testing.T) {
		tmpDir := t.TempDir()
		files := w.Files(tmpDir)
		if len(files) != 0 {
			t.Errorf("Files() returned %d files, want 0", len(files))
		}
	})
}

func TestNuGetVersionWriter_Files(t *testing.T) {
	w := &NuGetVersionWriter{}

	t.Run("with csproj", func(t *testing.T) {
		tmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmpDir, "MyApp.csproj"), []byte("<Project></Project>"), 0644); err != nil {
			t.Fatalf("Failed to write .csproj: %v", err)
		}
		files := w.Files(tmpDir)
		if len(files) != 1 {
			t.Errorf("Files() returned %d files, want 1", len(files))
		}
	})

	t.Run("no csproj", func(t *testing.T) {
		tmpDir := t.TempDir()
		files := w.Files(tmpDir)
		if len(files) != 0 {
			t.Errorf("Files() returned %d files, want 0", len(files))
		}
	})
}

func TestGemVersionWriter_ReadFromGemspec(t *testing.T) {
	w := &GemVersionWriter{}
	tmpDir := t.TempDir()

	gemspec := `Gem::Specification.new do |s|
  s.name        = "mygem"
  s.version     = "1.5.0"
  s.summary     = "A test gem"
end
`
	if err := os.WriteFile(filepath.Join(tmpDir, "mygem.gemspec"), []byte(gemspec), 0644); err != nil {
		t.Fatalf("Failed to write gemspec: %v", err)
	}

	ctx := context.Background()
	ver, err := w.ReadVersion(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ReadVersion from gemspec failed: %v", err)
	}
	if ver != "1.5.0" {
		t.Errorf("ReadVersion = %s, want 1.5.0", ver)
	}
}

func TestNuGetVersionWriter_ReadNoVersionTag(t *testing.T) {
	w := &NuGetVersionWriter{}
	tmpDir := t.TempDir()

	csproj := `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
</Project>
`
	if err := os.WriteFile(filepath.Join(tmpDir, "MyApp.csproj"), []byte(csproj), 0644); err != nil {
		t.Fatalf("Failed to write .csproj: %v", err)
	}

	ctx := context.Background()
	_, err := w.ReadVersion(ctx, tmpDir)
	if err == nil {
		t.Error("ReadVersion should fail when no <Version> tag exists")
	}
}

func TestNPMVersionWriter_WriteNoFile(t *testing.T) {
	w := &NPMVersionWriter{}
	tmpDir := t.TempDir()

	ctx := context.Background()
	err := w.WriteVersion(ctx, tmpDir, "1.0.0")
	if err == nil {
		t.Error("WriteVersion should fail when package.json doesn't exist")
	}
}

func TestCargoVersionWriter_WriteNoFile(t *testing.T) {
	w := &CargoVersionWriter{}
	tmpDir := t.TempDir()

	ctx := context.Background()
	err := w.WriteVersion(ctx, tmpDir, "1.0.0")
	if err == nil {
		t.Error("WriteVersion should fail when Cargo.toml doesn't exist")
	}
}

func TestNuGetVersionWriter_WriteNoFile(t *testing.T) {
	w := &NuGetVersionWriter{}
	tmpDir := t.TempDir()

	ctx := context.Background()
	err := w.WriteVersion(ctx, tmpDir, "1.0.0")
	if err == nil {
		t.Error("WriteVersion should fail when .csproj doesn't exist")
	}
}

func TestDirectoryVersionWriter_FilesPath(t *testing.T) {
	w := &DirectoryVersionWriter{}
	files := w.Files("/some/path")
	if len(files) != 1 {
		t.Fatalf("Files() returned %d files, want 1", len(files))
	}
	expected := filepath.Join("/some/path", "VERSION")
	if files[0] != expected {
		t.Errorf("Files()[0] = %s, want %s", files[0], expected)
	}
}

func TestCompositeVersionWriter_Files(t *testing.T) {
	w := NewCompositeVersionWriter()

	t.Run("npm package", func(t *testing.T) {
		tmpDir := t.TempDir()
		files := w.Files(tmpDir, monorepo.PackageTypeNPM)
		if len(files) != 1 {
			t.Errorf("Files() returned %d files, want 1", len(files))
		}
	})

	t.Run("unknown type returns nil", func(t *testing.T) {
		files := w.Files("/tmp", monorepo.PackageType("unknown"))
		if files != nil {
			t.Errorf("Files() returned %v, want nil", files)
		}
	})
}

func TestCompositeVersionWriter_WriteVersion(t *testing.T) {
	w := NewCompositeVersionWriter()

	t.Run("unknown type returns error", func(t *testing.T) {
		ctx := context.Background()
		ver, _ := version.Parse("1.0.0")
		err := w.WriteVersion(ctx, "/tmp", monorepo.PackageType("unknown"), ver)
		if err == nil {
			t.Error("WriteVersion should fail for unknown type")
		}
	})
}

func TestCompositeVersionWriter_ReadVersion_UnknownType(t *testing.T) {
	w := NewCompositeVersionWriter()
	ctx := context.Background()
	_, err := w.ReadVersion(ctx, "/tmp", monorepo.PackageType("unknown"))
	if err == nil {
		t.Error("ReadVersion should fail for unknown type")
	}
}

func TestGradleVersionWriter_ReadVersion_NoFile(t *testing.T) {
	w := &GradleVersionWriter{}
	tmpDir := t.TempDir()
	ctx := context.Background()
	_, err := w.ReadVersion(ctx, tmpDir)
	if err == nil {
		t.Error("ReadVersion should fail when no Gradle files exist")
	}
}

func TestGradleVersionWriter_WriteVersion_NoFile(t *testing.T) {
	w := &GradleVersionWriter{}
	tmpDir := t.TempDir()
	ctx := context.Background()
	err := w.WriteVersion(ctx, tmpDir, "1.0.0")
	if err == nil {
		t.Error("WriteVersion should fail when no Gradle files exist")
	}
}

func TestMavenVersionWriter_ReadVersion_NoFile(t *testing.T) {
	w := &MavenVersionWriter{}
	tmpDir := t.TempDir()
	ctx := context.Background()
	_, err := w.ReadVersion(ctx, tmpDir)
	if err == nil {
		t.Error("ReadVersion should fail when pom.xml doesn't exist")
	}
}

func TestNuGetVersionWriter_ReadVersion_NoFile(t *testing.T) {
	w := &NuGetVersionWriter{}
	tmpDir := t.TempDir()
	ctx := context.Background()
	_, err := w.ReadVersion(ctx, tmpDir)
	if err == nil {
		t.Error("ReadVersion should fail when no .csproj exists")
	}
}

func TestComposerVersionWriter_ReadVersion_NoFile(t *testing.T) {
	w := &ComposerVersionWriter{}
	tmpDir := t.TempDir()
	ctx := context.Background()
	_, err := w.ReadVersion(ctx, tmpDir)
	if err == nil {
		t.Error("ReadVersion should fail when composer.json doesn't exist")
	}
}

func TestGemVersionWriter_ReadVersion_NoFile(t *testing.T) {
	w := &GemVersionWriter{}
	tmpDir := t.TempDir()
	ctx := context.Background()
	_, err := w.ReadVersion(ctx, tmpDir)
	if err == nil {
		t.Error("ReadVersion should fail when no gem version files exist")
	}
}

func TestGemVersionWriter_WriteVersion_NoFile(t *testing.T) {
	w := &GemVersionWriter{}
	tmpDir := t.TempDir()
	ctx := context.Background()
	err := w.WriteVersion(ctx, tmpDir, "1.0.0")
	if err == nil {
		t.Error("WriteVersion should fail when no gem version files exist")
	}
}

func TestDirectoryVersionWriter_ReadVersion_NoFile(t *testing.T) {
	w := &DirectoryVersionWriter{}
	tmpDir := t.TempDir()
	ctx := context.Background()
	_, err := w.ReadVersion(ctx, tmpDir)
	if err == nil {
		t.Error("ReadVersion should fail when VERSION file doesn't exist")
	}
}
