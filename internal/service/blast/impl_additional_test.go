// Package blast provides blast radius analysis for monorepos.
package blast

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPythonPackage(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "blast-python-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create setup.py
	setupContent := `from setuptools import setup

setup(
    name="my-python-package",
    version="1.0.0",
    packages=["mypackage"],
)
`
	if err := os.WriteFile(filepath.Join(tmpDir, "setup.py"), []byte(setupContent), 0644); err != nil {
		t.Fatalf("Failed to write setup.py: %v", err)
	}

	svc := NewService(WithRepoPath(tmpDir)).(*serviceImpl)
	pkg := svc.detectPythonPackage(tmpDir, "testpkg")

	if pkg == nil {
		t.Fatal("Expected package, got nil")
	}

	if pkg.Type != PackageTypePython {
		t.Errorf("Package type = %v, want %v", pkg.Type, PackageTypePython)
	}
}

func TestDetectCargoPackage(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "blast-cargo-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Cargo.toml
	cargoContent := `[package]
name = "my-rust-package"
version = "0.2.1"
edition = "2021"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(cargoContent), 0644); err != nil {
		t.Fatalf("Failed to write Cargo.toml: %v", err)
	}

	svc := NewService(WithRepoPath(tmpDir)).(*serviceImpl)
	pkg := svc.detectCargoPackage(tmpDir, "testcargo")

	if pkg == nil {
		t.Fatal("Expected package, got nil")
	}

	if pkg.Type != PackageTypeCargo {
		t.Errorf("Package type = %v, want %v", pkg.Type, PackageTypeCargo)
	}

	if pkg.Name != "my-rust-package" {
		t.Errorf("Package name = %v, want my-rust-package", pkg.Name)
	}

	if pkg.Version != "0.2.1" {
		t.Errorf("Package version = %v, want 0.2.1", pkg.Version)
	}
}

func TestHasSourceFiles(t *testing.T) {
	tests := []struct {
		name     string
		dir      string
		create   []string // files to create
		expected bool
	}{
		{
			name:     "has source files",
			create:   []string{"main.go", "README.md"},
			expected: true,
		},
		{
			name:     "no source files",
			create:   []string{"README.md", "config.yaml"},
			expected: false,
		},
		{
			name:     "empty directory",
			create:   []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "blast-hassource-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Create files
			for _, file := range tt.create {
				filePath := filepath.Join(tmpDir, file)
				if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
					t.Fatalf("Failed to write file: %v", err)
				}
			}

			result := hasSourceFiles(tmpDir)
			if result != tt.expected {
				t.Errorf("hasSourceFiles() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestShouldExclude(t *testing.T) {
	svc := NewService().(*serviceImpl)

	tests := []struct {
		name          string
		path          string
		excludes      []string
		shouldExclude bool
	}{
		{
			name:          "node_modules",
			path:          "/proj/node_modules/package",
			excludes:      []string{"node_modules", "vendor"},
			shouldExclude: true,
		},
		{
			name:          "vendor directory",
			path:          "/proj/vendor/lib",
			excludes:      []string{"node_modules", "vendor"},
			shouldExclude: true,
		},
		{
			name:          "normal path",
			path:          "/proj/src/main.go",
			excludes:      []string{"node_modules", "vendor"},
			shouldExclude: false,
		},
		{
			name:          "empty excludes",
			path:          "/proj/src/main.go",
			excludes:      []string{},
			shouldExclude: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.shouldExclude(tt.path, tt.excludes)
			if result != tt.shouldExclude {
				t.Errorf("shouldExclude(%q) = %v, want %v", tt.path, result, tt.shouldExclude)
			}
		})
	}
}

func TestIdentifyRiskFactors(t *testing.T) {
	svc := NewService().(*serviceImpl)

	tests := []struct {
		name     string
		summary  *Summary
		impacts  []*Impact
		files    []ChangedFile
		wantMore bool
	}{
		{
			name: "high risk scenario",
			summary: &Summary{
				TotalPackages:    10,
				TotalAffected:    6,
				DirectlyAffected: 6,
				HighRiskCount:    2,
			},
			impacts: []*Impact{
				{
					Level: ImpactLevelDirect,
					DirectChanges: []ChangedFile{
						{Path: "main.go", Category: FileCategorySource, Insertions: 500, Deletions: 200},
					},
				},
			},
			files: []ChangedFile{
				{Path: "main.go", Category: FileCategorySource, Insertions: 500, Deletions: 200},
			},
			wantMore: true,
		},
		{
			name: "low risk scenario",
			summary: &Summary{
				TotalPackages:    10,
				TotalAffected:    1,
				DirectlyAffected: 1,
				HighRiskCount:    0,
			},
			impacts: []*Impact{
				{
					Level: ImpactLevelDirect,
					DirectChanges: []ChangedFile{
						{Path: "README.md", Category: FileCategoryDocs, Insertions: 10, Deletions: 5},
					},
				},
			},
			files: []ChangedFile{
				{Path: "README.md", Category: FileCategoryDocs, Insertions: 10, Deletions: 5},
			},
			wantMore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factors := svc.identifyRiskFactors(tt.summary, tt.impacts, tt.files)
			if tt.wantMore && len(factors) == 0 {
				t.Error("identifyRiskFactors() should return risk factors for high risk scenario")
			}
			// For low risk, we just check it doesn't panic
		})
	}
}

func TestDetectPackage(t *testing.T) {
	// Create temporary directory with Go module
	tmpDir, err := os.MkdirTemp("", "blast-detect-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create go.mod
	goModContent := `module example.com/test

go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	svc := NewService(WithRepoPath(tmpDir)).(*serviceImpl)
	pkg, err := svc.detectPackage(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("detectPackage() error = %v", err)
	}

	if pkg == nil {
		t.Fatal("Expected package, got nil")
	}

	if pkg.Type != PackageTypeGoModule {
		t.Errorf("Package type = %v, want %v", pkg.Type, PackageTypeGoModule)
	}
}

func TestDetectPackage_NoPackage(t *testing.T) {
	// Create temporary directory without any package files
	tmpDir, err := os.MkdirTemp("", "blast-nopackage-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	svc := NewService(WithRepoPath(tmpDir)).(*serviceImpl)
	pkg, err := svc.detectPackage(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("detectPackage() error = %v", err)
	}

	if pkg != nil {
		t.Errorf("Expected nil package for directory without package files, got %v", pkg)
	}
}

func TestDetectGoModule_Partial(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "blast-gomod-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create minimal go.mod without version info
	goModContent := `module example.com/mymodule

go 1.21

require (
	github.com/some/dep v1.0.0
)
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	svc := NewService(WithRepoPath(tmpDir)).(*serviceImpl)
	pkg := svc.detectGoModule(tmpDir, "mymodule")

	if pkg == nil {
		t.Fatal("Expected package, got nil")
	}

	if pkg.Name != "example.com/mymodule" {
		t.Errorf("Package name = %v, want example.com/mymodule", pkg.Name)
	}

	if len(pkg.Dependencies) < 1 {
		t.Error("Expected at least one dependency")
	}
}

func TestDetectNPMPackage_Partial(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "blast-npm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create package.json with dependencies
	packageJSON := `{
  "name": "my-package",
  "version": "2.0.0",
  "dependencies": {
    "react": "^18.0.0",
    "lodash": "^4.17.21"
  },
  "devDependencies": {
    "jest": "^29.0.0"
  }
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	svc := NewService(WithRepoPath(tmpDir)).(*serviceImpl)
	pkg := svc.detectNPMPackage(tmpDir, "mypackage")

	if pkg == nil {
		t.Fatal("Expected package, got nil")
	}

	if pkg.Name != "my-package" {
		t.Errorf("Package name = %v, want my-package", pkg.Name)
	}

	if pkg.Version != "2.0.0" {
		t.Errorf("Package version = %v, want 2.0.0", pkg.Version)
	}

	if len(pkg.Dependencies) < 2 {
		t.Errorf("Expected at least 2 dependencies, got %d", len(pkg.Dependencies))
	}
}
