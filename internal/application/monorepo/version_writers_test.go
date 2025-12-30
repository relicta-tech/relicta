package monorepo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/monorepo"
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
