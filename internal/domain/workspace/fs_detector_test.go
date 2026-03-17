package workspace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFSDetector_DetectGoWorkspace(t *testing.T) {
	// Create temp directory structure for Go workspace
	root := t.TempDir()

	// Create go.work file
	goWorkContent := `go 1.22.0

use (
	./cmd/api
	./pkg/shared
	./services/auth
)
`
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte(goWorkContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create module directories with go.mod files
	modules := map[string]string{
		"cmd/api":       "module github.com/test/monorepo/cmd/api",
		"pkg/shared":    "module github.com/test/monorepo/pkg/shared",
		"services/auth": "module github.com/test/monorepo/services/auth",
	}

	for dir, modContent := range modules {
		modDir := filepath.Join(root, dir)
		if err := os.MkdirAll(modDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(modDir, "go.mod"), []byte(modContent+"\n\ngo 1.22.0\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	detector := NewFSDetector()
	ctx := context.Background()
	opts := DetectionOptions{
		MaxDepth:        0,
		IncludePackages: true,
	}

	result, err := detector.DetectFromRoot(ctx, root, opts)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Success() {
		t.Fatalf("detection failed: %v", result.Error)
	}

	ws := result.Workspace
	if ws.Type != WorkspaceTypeGoModule {
		t.Errorf("Type = %s, want %s", ws.Type, WorkspaceTypeGoModule)
	}

	if ws.PackageManager != PackageManagerGo {
		t.Errorf("PackageManager = %s, want %s", ws.PackageManager, PackageManagerGo)
	}

	if len(ws.Packages) != 3 {
		t.Errorf("Packages count = %d, want 3", len(ws.Packages))
	}

	// Verify packages are discovered
	packagePaths := make(map[string]bool)
	for _, pkg := range ws.Packages {
		packagePaths[pkg.Path] = true
	}

	expectedPaths := []string{"cmd/api", "pkg/shared", "services/auth"}
	for _, path := range expectedPaths {
		if !packagePaths[path] {
			t.Errorf("missing package path: %s", path)
		}
	}
}

func TestFSDetector_DetectNpmWorkspace(t *testing.T) {
	root := t.TempDir()

	// Create root package.json with workspaces
	rootPkg := map[string]interface{}{
		"name":       "test-monorepo",
		"version":    "1.0.0",
		"private":    true,
		"workspaces": []string{"packages/*"},
	}
	rootPkgData, _ := json.MarshalIndent(rootPkg, "", "  ")
	if err := os.WriteFile(filepath.Join(root, "package.json"), rootPkgData, 0644); err != nil {
		t.Fatal(err)
	}

	// Create workspace packages
	packages := map[string]map[string]interface{}{
		"packages/core": {
			"name":    "@test/core",
			"version": "1.0.0",
		},
		"packages/utils": {
			"name":    "@test/utils",
			"version": "0.5.0",
			"dependencies": map[string]string{
				"@test/core": "^1.0.0",
			},
		},
		"packages/app": {
			"name":    "@test/app",
			"version": "2.0.0",
			"private": true,
			"dependencies": map[string]string{
				"@test/core":  "^1.0.0",
				"@test/utils": "^0.5.0",
			},
		},
	}

	for dir, pkg := range packages {
		pkgDir := filepath.Join(root, dir)
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			t.Fatal(err)
		}
		pkgData, _ := json.MarshalIndent(pkg, "", "  ")
		if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), pkgData, 0644); err != nil {
			t.Fatal(err)
		}
	}

	detector := NewFSDetector()
	ctx := context.Background()
	opts := DetectionOptions{
		MaxDepth:        0,
		IncludePackages: true,
	}

	result, err := detector.DetectFromRoot(ctx, root, opts)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Success() {
		t.Fatalf("detection failed: %v", result.Error)
	}

	ws := result.Workspace
	if ws.Type != WorkspaceTypeNpm {
		t.Errorf("Type = %s, want %s", ws.Type, WorkspaceTypeNpm)
	}

	if len(ws.Packages) != 3 {
		t.Errorf("Packages count = %d, want 3", len(ws.Packages))
	}

	// Verify internal dependencies are resolved
	for _, pkg := range ws.Packages {
		if pkg.Name == "@test/utils" {
			if len(pkg.InternalDependencies) != 1 {
				t.Errorf("@test/utils InternalDependencies = %d, want 1", len(pkg.InternalDependencies))
			} else if pkg.InternalDependencies[0] != "@test/core" {
				t.Errorf("@test/utils InternalDependencies[0] = %s, want @test/core", pkg.InternalDependencies[0])
			}
		}
		if pkg.Name == "@test/app" {
			if len(pkg.InternalDependencies) != 2 {
				t.Errorf("@test/app InternalDependencies = %d, want 2", len(pkg.InternalDependencies))
			}
		}
	}
}

func TestFSDetector_DetectCargoWorkspace(t *testing.T) {
	root := t.TempDir()

	// Create root Cargo.toml with workspace section
	cargoContent := `[workspace]
members = [
    "crates/core",
    "crates/cli",
]

[workspace.dependencies]
serde = "1.0"
`
	if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte(cargoContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create member crates
	crates := map[string]string{
		"crates/core": `[package]
name = "myproject-core"
version = "0.1.0"
edition = "2021"
`,
		"crates/cli": `[package]
name = "myproject-cli"
version = "0.2.0"
edition = "2021"

[dependencies]
myproject-core = { path = "../core" }
`,
	}

	for dir, content := range crates {
		crateDir := filepath.Join(root, dir)
		if err := os.MkdirAll(crateDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(crateDir, "Cargo.toml"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	detector := NewFSDetector()
	ctx := context.Background()
	opts := DetectionOptions{
		MaxDepth:        0,
		IncludePackages: true,
	}

	result, err := detector.DetectFromRoot(ctx, root, opts)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Success() {
		t.Fatalf("detection failed: %v", result.Error)
	}

	ws := result.Workspace
	if ws.Type != WorkspaceTypeCargo {
		t.Errorf("Type = %s, want %s", ws.Type, WorkspaceTypeCargo)
	}

	if ws.PackageManager != PackageManagerCargo {
		t.Errorf("PackageManager = %s, want %s", ws.PackageManager, PackageManagerCargo)
	}

	if len(ws.Packages) != 2 {
		t.Errorf("Packages count = %d, want 2", len(ws.Packages))
	}

	// Verify package names and versions
	for _, pkg := range ws.Packages {
		switch pkg.Name {
		case "myproject-core":
			if pkg.Version != "0.1.0" {
				t.Errorf("myproject-core version = %s, want 0.1.0", pkg.Version)
			}
		case "myproject-cli":
			if pkg.Version != "0.2.0" {
				t.Errorf("myproject-cli version = %s, want 0.2.0", pkg.Version)
			}
		default:
			t.Errorf("unexpected package name: %s", pkg.Name)
		}
	}
}

func TestFSDetector_DetectNoWorkspace(t *testing.T) {
	root := t.TempDir()

	// Create a simple single-project directory (no workspace markers)
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	detector := NewFSDetector()
	ctx := context.Background()
	opts := DetectionOptions{
		MaxDepth:        0,
		IncludePackages: true,
	}

	result, err := detector.Detect(ctx, root, opts)
	if err != nil {
		t.Fatal(err)
	}

	if result.Workspace == nil {
		t.Fatal("workspace should not be nil")
	}

	if result.Workspace.Type != WorkspaceTypeNone {
		t.Errorf("Type = %s, want %s", result.Workspace.Type, WorkspaceTypeNone)
	}
}

func TestFSDetector_DetectUpward(t *testing.T) {
	root := t.TempDir()

	// Create go.work at root
	goWorkContent := `go 1.22.0

use ./services/api
`
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte(goWorkContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a nested directory
	nestedDir := filepath.Join(root, "services", "api", "internal")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "services", "api", "go.mod"),
		[]byte("module github.com/test/api\n\ngo 1.22.0\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	detector := NewFSDetector()
	ctx := context.Background()
	opts := DetectionOptions{
		MaxDepth:        5,
		IncludePackages: true,
	}

	// Detect from nested directory, should find go.work at root
	result, err := detector.Detect(ctx, nestedDir, opts)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Success() {
		t.Fatalf("detection failed: %v", result.Error)
	}

	if result.Workspace.Type != WorkspaceTypeGoModule {
		t.Errorf("Type = %s, want %s", result.Workspace.Type, WorkspaceTypeGoModule)
	}
}

func TestFSDetector_PnpmWorkspace(t *testing.T) {
	root := t.TempDir()

	// Create pnpm-workspace.yaml
	pnpmContent := `packages:
  - 'packages/*'
  - 'apps/*'
`
	if err := os.WriteFile(filepath.Join(root, "pnpm-workspace.yaml"), []byte(pnpmContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create pnpm-lock.yaml to detect package manager
	if err := os.WriteFile(filepath.Join(root, "pnpm-lock.yaml"), []byte("lockfileVersion: 5\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create packages
	pkgDir := filepath.Join(root, "packages", "ui")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	uiPkg, _ := json.MarshalIndent(map[string]interface{}{
		"name":    "@test/ui",
		"version": "1.0.0",
	}, "", "  ")
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), uiPkg, 0644); err != nil {
		t.Fatal(err)
	}

	detector := NewFSDetector()
	ctx := context.Background()
	opts := DetectionOptions{
		MaxDepth:        0,
		IncludePackages: true,
	}

	result, err := detector.DetectFromRoot(ctx, root, opts)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Success() {
		t.Fatalf("detection failed: %v", result.Error)
	}

	ws := result.Workspace
	if ws.Type != WorkspaceTypePnpm {
		t.Errorf("Type = %s, want %s", ws.Type, WorkspaceTypePnpm)
	}
	if ws.PackageManager != PackageManagerPnpm {
		t.Errorf("PackageManager = %s, want %s", ws.PackageManager, PackageManagerPnpm)
	}
}

func TestFSDetector_DetectType(t *testing.T) {
	root := t.TempDir()

	// Create go.work
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.22.0\n"), 0644); err != nil {
		t.Fatal(err)
	}

	detector := NewFSDetector()
	ctx := context.Background()

	wsType, err := detector.DetectType(ctx, root)
	if err != nil {
		t.Fatal(err)
	}

	if wsType != WorkspaceTypeGoModule {
		t.Errorf("DetectType = %s, want %s", wsType, WorkspaceTypeGoModule)
	}
}

func TestFSDetector_ValidateWorkspace(t *testing.T) {
	root := t.TempDir()
	pkgDir := filepath.Join(root, "packages", "core")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	detector := NewFSDetector()
	ctx := context.Background()

	tests := []struct {
		name    string
		ws      *Workspace
		wantErr bool
	}{
		{
			name:    "nil workspace",
			ws:      nil,
			wantErr: true,
		},
		{
			name:    "empty root path",
			ws:      &Workspace{},
			wantErr: true,
		},
		{
			name: "nonexistent root",
			ws: &Workspace{
				RootPath: "/nonexistent/path",
			},
			wantErr: true,
		},
		{
			name: "valid workspace",
			ws: &Workspace{
				RootPath: root,
				Packages: []*Package{
					{Name: "core", Path: "packages/core"},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate package names",
			ws: &Workspace{
				RootPath: root,
				Packages: []*Package{
					{Name: "core", Path: "packages/core"},
					{Name: "core", Path: "packages/core2"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := detector.ValidateWorkspace(ctx, tt.ws)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkspace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFSDetector_ConfigResolverMerge(t *testing.T) {
	resolver := NewDefaultConfigResolver()

	detected := &WorkspaceConfig{
		Enabled:      true,
		Type:         WorkspaceTypeNpm,
		Strategy:     StrategyIndependent,
		PackagePaths: []string{"packages/*"},
	}

	explicit := &WorkspaceConfig{
		Strategy:     StrategyLockstep,
		PackagePaths: []string{"libs/*", "apps/*"},
	}

	merged := resolver.MergeConfig(detected, explicit)
	if merged.Strategy != StrategyLockstep {
		t.Errorf("merged Strategy = %s, want %s", merged.Strategy, StrategyLockstep)
	}
	if len(merged.PackagePaths) != 2 {
		t.Errorf("merged PackagePaths = %d, want 2", len(merged.PackagePaths))
	}

	// Test with nil explicit
	merged2 := resolver.MergeConfig(detected, nil)
	if merged2.Type != WorkspaceTypeNpm {
		t.Errorf("merged2 Type = %s, want %s", merged2.Type, WorkspaceTypeNpm)
	}

	// Test with nil detected
	merged3 := resolver.MergeConfig(nil, explicit)
	if merged3.Strategy != StrategyLockstep {
		t.Errorf("merged3 Strategy = %s, want %s", merged3.Strategy, StrategyLockstep)
	}
}

func TestFSDetector_GoWorkspaceInternalDeps(t *testing.T) {
	root := t.TempDir()

	goWorkContent := `go 1.22.0

use (
	./pkg/core
	./pkg/utils
)
`
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte(goWorkContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Core module
	coreDir := filepath.Join(root, "pkg", "core")
	if err := os.MkdirAll(coreDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coreDir, "go.mod"), []byte("module github.com/test/pkg/core\n\ngo 1.22.0\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Utils module depending on core
	utilsDir := filepath.Join(root, "pkg", "utils")
	if err := os.MkdirAll(utilsDir, 0755); err != nil {
		t.Fatal(err)
	}
	utilsMod := `module github.com/test/pkg/utils

go 1.22.0

require github.com/test/pkg/core v0.0.0
`
	if err := os.WriteFile(filepath.Join(utilsDir, "go.mod"), []byte(utilsMod), 0644); err != nil {
		t.Fatal(err)
	}

	detector := NewFSDetector()
	ctx := context.Background()
	opts := DetectionOptions{
		MaxDepth:        0,
		IncludePackages: true,
	}

	result, err := detector.DetectFromRoot(ctx, root, opts)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Success() {
		t.Fatalf("detection failed: %v", result.Error)
	}

	// Verify internal dependency resolution
	ws := result.Workspace
	if len(ws.Packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(ws.Packages))
	}

	for _, pkg := range ws.Packages {
		if pkg.Name == "github.com/test/pkg/utils" {
			if len(pkg.InternalDependencies) != 1 {
				t.Errorf("utils InternalDependencies = %d, want 1", len(pkg.InternalDependencies))
			}
		}
	}
}
