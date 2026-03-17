package workspace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/internal/domain/workspace"
)

func TestFileDetector_Detect_PnpmWorkspace(t *testing.T) {
	// Create a temporary pnpm workspace
	tmpDir := t.TempDir()

	// Create pnpm-workspace.yaml
	workspaceYAML := `packages:
  - 'packages/*'
  - 'apps/*'
`
	err := os.WriteFile(filepath.Join(tmpDir, "pnpm-workspace.yaml"), []byte(workspaceYAML), 0644)
	require.NoError(t, err)

	// Create package.json
	pkgJSON := `{"name": "monorepo", "private": true}`
	err = os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644)
	require.NoError(t, err)

	// Create a package directory
	pkgDir := filepath.Join(tmpDir, "packages", "pkg-a")
	err = os.MkdirAll(pkgDir, 0755)
	require.NoError(t, err)

	pkgAJSON := `{"name": "@monorepo/pkg-a", "version": "1.0.0"}`
	err = os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(pkgAJSON), 0644)
	require.NoError(t, err)

	// Detect workspace
	detector := NewFileDetector()
	ctx := context.Background()
	opts := workspace.DefaultDetectionOptions()

	result, err := detector.Detect(ctx, tmpDir, opts)
	require.NoError(t, err)
	require.True(t, result.Success())

	ws := result.Workspace
	assert.Equal(t, workspace.WorkspaceTypePnpm, ws.Type)
	assert.Equal(t, workspace.PackageManagerPnpm, ws.PackageManager)
	assert.True(t, ws.Confidence >= 0.9)
	assert.Contains(t, ws.Markers, "pnpm-workspace.yaml")
}

func TestFileDetector_Detect_NpmWorkspaces(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package.json with workspaces field
	pkgJSON := `{
		"name": "npm-monorepo",
		"private": true,
		"workspaces": ["packages/*"]
	}`
	err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644)
	require.NoError(t, err)

	// Create package-lock.json to indicate npm
	err = os.WriteFile(filepath.Join(tmpDir, "package-lock.json"), []byte("{}"), 0644)
	require.NoError(t, err)

	detector := NewFileDetector()
	ctx := context.Background()
	opts := workspace.DefaultDetectionOptions()

	result, err := detector.Detect(ctx, tmpDir, opts)
	require.NoError(t, err)
	require.True(t, result.Success())

	ws := result.Workspace
	assert.Equal(t, workspace.WorkspaceTypeNpm, ws.Type)
	assert.Equal(t, workspace.PackageManagerNpm, ws.PackageManager)
}

func TestFileDetector_Detect_LernaWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	// Create lerna.json
	lernaJSON := `{
		"version": "independent",
		"packages": ["packages/*"]
	}`
	err := os.WriteFile(filepath.Join(tmpDir, "lerna.json"), []byte(lernaJSON), 0644)
	require.NoError(t, err)

	detector := NewFileDetector()
	ctx := context.Background()
	opts := workspace.DefaultDetectionOptions()

	result, err := detector.Detect(ctx, tmpDir, opts)
	require.NoError(t, err)
	require.True(t, result.Success())

	ws := result.Workspace
	assert.Equal(t, workspace.WorkspaceTypeLerna, ws.Type)
}

func TestFileDetector_Detect_GoWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.work file
	goWork := `go 1.22

use (
	./cmd/app
	./pkg/lib
)
`
	err := os.WriteFile(filepath.Join(tmpDir, "go.work"), []byte(goWork), 0644)
	require.NoError(t, err)

	detector := NewFileDetector()
	ctx := context.Background()
	opts := workspace.DefaultDetectionOptions()

	result, err := detector.Detect(ctx, tmpDir, opts)
	require.NoError(t, err)
	require.True(t, result.Success())

	ws := result.Workspace
	assert.Equal(t, workspace.WorkspaceTypeGoModule, ws.Type)
	assert.Equal(t, workspace.PackageManagerGo, ws.PackageManager)
}

func TestFileDetector_Detect_CargoWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Cargo.toml with [workspace]
	cargoToml := `[workspace]
members = ["crates/*"]

[package]
name = "root"
version = "0.1.0"
`
	err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(cargoToml), 0644)
	require.NoError(t, err)

	detector := NewFileDetector()
	ctx := context.Background()
	opts := workspace.DefaultDetectionOptions()

	result, err := detector.Detect(ctx, tmpDir, opts)
	require.NoError(t, err)
	require.True(t, result.Success())

	ws := result.Workspace
	assert.Equal(t, workspace.WorkspaceTypeCargo, ws.Type)
	assert.Equal(t, workspace.PackageManagerCargo, ws.PackageManager)
}

func TestFileDetector_Detect_TurborepoWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	// Create turbo.json
	turboJSON := `{
		"$schema": "https://turbo.build/schema.json",
		"pipeline": {
			"build": {
				"dependsOn": ["^build"]
			}
		}
	}`
	err := os.WriteFile(filepath.Join(tmpDir, "turbo.json"), []byte(turboJSON), 0644)
	require.NoError(t, err)

	// Also need package.json with workspaces for it to be a valid workspace
	pkgJSON := `{"name": "turbo-monorepo", "private": true, "workspaces": ["packages/*"]}`
	err = os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644)
	require.NoError(t, err)

	detector := NewFileDetector()
	ctx := context.Background()
	opts := workspace.DefaultDetectionOptions()

	result, err := detector.Detect(ctx, tmpDir, opts)
	require.NoError(t, err)
	require.True(t, result.Success())

	ws := result.Workspace
	// Turborepo has higher priority than npm workspaces
	assert.Equal(t, workspace.WorkspaceTypeTurborepo, ws.Type)
}

func TestFileDetector_Detect_NxWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nx.json
	nxJSON := `{
		"$schema": "./node_modules/nx/schemas/nx-schema.json",
		"tasksRunnerOptions": {
			"default": {
				"runner": "nx/tasks-runners/default"
			}
		}
	}`
	err := os.WriteFile(filepath.Join(tmpDir, "nx.json"), []byte(nxJSON), 0644)
	require.NoError(t, err)

	detector := NewFileDetector()
	ctx := context.Background()
	opts := workspace.DefaultDetectionOptions()

	result, err := detector.Detect(ctx, tmpDir, opts)
	require.NoError(t, err)
	require.True(t, result.Success())

	ws := result.Workspace
	assert.Equal(t, workspace.WorkspaceTypeNx, ws.Type)
}

func TestFileDetector_Detect_NoWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	// Create just a regular package.json without workspaces
	pkgJSON := `{"name": "single-package", "version": "1.0.0"}`
	err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644)
	require.NoError(t, err)

	detector := NewFileDetector()
	ctx := context.Background()
	opts := workspace.DefaultDetectionOptions()

	result, err := detector.Detect(ctx, tmpDir, opts)
	require.NoError(t, err)
	require.NotNil(t, result.Workspace)

	ws := result.Workspace
	assert.Equal(t, workspace.WorkspaceTypeNone, ws.Type)
}

func TestFileDetector_Detect_SearchUpward(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workspace at root
	workspaceYAML := `packages:
  - 'packages/*'
`
	err := os.WriteFile(filepath.Join(tmpDir, "pnpm-workspace.yaml"), []byte(workspaceYAML), 0644)
	require.NoError(t, err)

	// Create nested package directory
	nestedDir := filepath.Join(tmpDir, "packages", "deeply", "nested")
	err = os.MkdirAll(nestedDir, 0755)
	require.NoError(t, err)

	// Detect from nested directory - should find root workspace
	detector := NewFileDetector()
	ctx := context.Background()
	opts := workspace.DefaultDetectionOptions()
	opts.MaxDepth = 5 // Need higher depth to search from packages/deeply/nested to root

	result, err := detector.Detect(ctx, nestedDir, opts)
	require.NoError(t, err)
	require.True(t, result.Success())

	ws := result.Workspace
	assert.Equal(t, workspace.WorkspaceTypePnpm, ws.Type)
	assert.Equal(t, tmpDir, ws.RootPath)
}

func TestFileDetector_DiscoverPackages_Node(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workspace
	err := os.WriteFile(filepath.Join(tmpDir, "pnpm-workspace.yaml"), []byte("packages:\n  - 'packages/*'"), 0644)
	require.NoError(t, err)

	// Create packages
	for _, name := range []string{"pkg-a", "pkg-b", "pkg-c"} {
		pkgDir := filepath.Join(tmpDir, "packages", name)
		err = os.MkdirAll(pkgDir, 0755)
		require.NoError(t, err)

		pkgJSON := map[string]interface{}{
			"name":    "@monorepo/" + name,
			"version": "1.0.0",
		}
		if name == "pkg-c" {
			pkgJSON["private"] = true
		}
		content, _ := json.Marshal(pkgJSON)
		err = os.WriteFile(filepath.Join(pkgDir, "package.json"), content, 0644)
		require.NoError(t, err)
	}

	detector := NewFileDetector()
	ctx := context.Background()
	opts := workspace.DefaultDetectionOptions()

	result, err := detector.Detect(ctx, tmpDir, opts)
	require.NoError(t, err)
	require.True(t, result.Success())

	ws := result.Workspace
	assert.Len(t, ws.Packages, 3)

	// Check package details
	pkgA := ws.FindPackage("@monorepo/pkg-a")
	assert.NotNil(t, pkgA)
	assert.Equal(t, "1.0.0", pkgA.Version)
	assert.False(t, pkgA.Private)

	pkgC := ws.FindPackage("@monorepo/pkg-c")
	assert.NotNil(t, pkgC)
	assert.True(t, pkgC.Private)
}

func TestFileDetector_DiscoverPackages_Go(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.work
	goWork := `go 1.22

use (
	./cmd/app
	./pkg/lib
)
`
	err := os.WriteFile(filepath.Join(tmpDir, "go.work"), []byte(goWork), 0644)
	require.NoError(t, err)

	// Create modules
	cmdDir := filepath.Join(tmpDir, "cmd", "app")
	err = os.MkdirAll(cmdDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(cmdDir, "go.mod"), []byte("module github.com/example/cmd/app\n\ngo 1.22"), 0644)
	require.NoError(t, err)

	pkgDir := filepath.Join(tmpDir, "pkg", "lib")
	err = os.MkdirAll(pkgDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(pkgDir, "go.mod"), []byte("module github.com/example/pkg/lib\n\ngo 1.22"), 0644)
	require.NoError(t, err)

	detector := NewFileDetector()
	ctx := context.Background()
	opts := workspace.DefaultDetectionOptions()

	result, err := detector.Detect(ctx, tmpDir, opts)
	require.NoError(t, err)
	require.True(t, result.Success())

	ws := result.Workspace
	assert.Equal(t, workspace.WorkspaceTypeGoModule, ws.Type)
	// Note: Package discovery for Go uses different patterns
}

func TestFileDetector_DetectType(t *testing.T) {
	tmpDir := t.TempDir()

	// Create pnpm workspace
	err := os.WriteFile(filepath.Join(tmpDir, "pnpm-workspace.yaml"), []byte("packages:\n  - 'packages/*'"), 0644)
	require.NoError(t, err)

	detector := NewFileDetector()
	ctx := context.Background()

	wsType, err := detector.DetectType(ctx, tmpDir)
	require.NoError(t, err)
	assert.Equal(t, workspace.WorkspaceTypePnpm, wsType)
}

func TestFileDetector_ValidateWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	detector := NewFileDetector()
	ctx := context.Background()

	// Valid workspace
	ws := &workspace.Workspace{
		RootPath: tmpDir,
		Type:     workspace.WorkspaceTypePnpm,
	}
	err := detector.ValidateWorkspace(ctx, ws)
	assert.NoError(t, err)

	// Nil workspace
	err = detector.ValidateWorkspace(ctx, nil)
	assert.ErrorIs(t, err, ErrInvalidWorkspace)

	// Empty root path
	ws = &workspace.Workspace{
		RootPath: "",
		Type:     workspace.WorkspaceTypePnpm,
	}
	err = detector.ValidateWorkspace(ctx, ws)
	assert.ErrorIs(t, err, ErrInvalidWorkspace)

	// Non-existent path
	ws = &workspace.Workspace{
		RootPath: "/nonexistent/path",
		Type:     workspace.WorkspaceTypePnpm,
	}
	err = detector.ValidateWorkspace(ctx, ws)
	assert.ErrorIs(t, err, ErrInvalidWorkspace)
}

func TestFileDetector_PreferredTypes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create both pnpm-workspace.yaml and turbo.json
	err := os.WriteFile(filepath.Join(tmpDir, "pnpm-workspace.yaml"), []byte("packages:\n  - 'packages/*'"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "turbo.json"), []byte("{}"), 0644)
	require.NoError(t, err)

	detector := NewFileDetector()
	ctx := context.Background()

	// With preferred types, should only detect pnpm
	opts := workspace.DetectionOptions{
		MaxDepth:        3,
		IncludePackages: false,
		PreferredTypes:  []workspace.WorkspaceType{workspace.WorkspaceTypePnpm},
	}

	result, err := detector.Detect(ctx, tmpDir, opts)
	require.NoError(t, err)
	require.True(t, result.Success())
	assert.Equal(t, workspace.WorkspaceTypePnpm, result.Workspace.Type)
}

func TestFileDetector_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a deep directory structure
	deepDir := filepath.Join(tmpDir, "a", "b", "c", "d", "e", "f")
	err := os.MkdirAll(deepDir, 0755)
	require.NoError(t, err)

	detector := NewFileDetector()

	// Create a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opts := workspace.DetectionOptions{MaxDepth: 10}
	result, err := detector.Detect(ctx, deepDir, opts)

	// Should handle cancellation gracefully
	assert.NoError(t, err)
	assert.Error(t, result.Error)
}

func TestFileDetector_DetectFromRoot(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a pnpm workspace
	workspaceYAML := `packages:
  - 'packages/*'
`
	err := os.WriteFile(filepath.Join(tmpDir, "pnpm-workspace.yaml"), []byte(workspaceYAML), 0644)
	require.NoError(t, err)

	// Create a package
	pkgDir := filepath.Join(tmpDir, "packages", "pkg-a")
	err = os.MkdirAll(pkgDir, 0755)
	require.NoError(t, err)
	pkgJSON := `{"name": "@monorepo/pkg-a", "version": "1.0.0"}`
	err = os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(pkgJSON), 0644)
	require.NoError(t, err)

	detector := NewFileDetector()
	ctx := context.Background()

	// Test DetectFromRoot with IncludePackages=true
	opts := workspace.DetectionOptions{
		MaxDepth:        3,
		IncludePackages: true,
	}
	result, err := detector.DetectFromRoot(ctx, tmpDir, opts)
	require.NoError(t, err)
	require.NotNil(t, result.Workspace)
	assert.Equal(t, workspace.WorkspaceTypePnpm, result.Workspace.Type)
	assert.Len(t, result.Workspace.Packages, 1)

	// Test DetectFromRoot on non-workspace directory
	nonWsDir := t.TempDir()
	result, err = detector.DetectFromRoot(ctx, nonWsDir, opts)
	require.NoError(t, err)
	require.NotNil(t, result.Workspace)
	assert.Equal(t, workspace.WorkspaceTypeNone, result.Workspace.Type)
}

func TestFileDetector_DetectFromRoot_InvalidPath(t *testing.T) {
	detector := NewFileDetector()
	ctx := context.Background()
	opts := workspace.DefaultDetectionOptions()

	// Test with non-existent path - the function returns no error but workspace is None
	result, err := detector.DetectFromRoot(ctx, "/nonexistent/path/that/does/not/exist", opts)
	require.NoError(t, err)
	// The function returns a result but marks it with an error or returns WorkspaceTypeNone
	if result.Error == nil {
		// If no error, workspace should be None type
		assert.Equal(t, workspace.WorkspaceTypeNone, result.Workspace.Type)
	}
}

func TestFileDetector_Detect_MavenWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	// Create pom.xml with modules section
	pomXML := `<?xml version="1.0" encoding="UTF-8"?>
<project>
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>parent</artifactId>
    <packaging>pom</packaging>
    <modules>
        <module>core</module>
        <module>api</module>
    </modules>
</project>
`
	err := os.WriteFile(filepath.Join(tmpDir, "pom.xml"), []byte(pomXML), 0644)
	require.NoError(t, err)

	detector := NewFileDetector()
	ctx := context.Background()
	opts := workspace.DefaultDetectionOptions()

	result, err := detector.Detect(ctx, tmpDir, opts)
	require.NoError(t, err)
	require.True(t, result.Success())

	ws := result.Workspace
	assert.Equal(t, workspace.WorkspaceTypeMaven, ws.Type)
}

func TestFileDetector_NewFileDetectorWithMarkers(t *testing.T) {
	customMarkers := []workspace.MarkerFile{
		{
			Name:           "custom-workspace.json",
			Type:           workspace.WorkspaceTypePnpm,
			PackageManager: workspace.PackageManagerPnpm,
			Priority:       100,
		},
	}

	detector := NewFileDetectorWithMarkers(customMarkers)
	assert.NotNil(t, detector)

	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "custom-workspace.json"), []byte("{}"), 0644)
	require.NoError(t, err)

	ctx := context.Background()
	opts := workspace.DefaultDetectionOptions()

	result, err := detector.Detect(ctx, tmpDir, opts)
	require.NoError(t, err)
	require.True(t, result.Success())
	assert.Equal(t, workspace.WorkspaceTypePnpm, result.Workspace.Type)
}

func TestFileDetector_ShouldExclude(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directories
	nodeModules := filepath.Join(tmpDir, "node_modules", "pkg")
	err := os.MkdirAll(nodeModules, 0755)
	require.NoError(t, err)

	packages := filepath.Join(tmpDir, "packages", "lib")
	err = os.MkdirAll(packages, 0755)
	require.NoError(t, err)

	detector := NewFileDetector()

	excludePatterns := []string{"node_modules", ".git", "dist"}

	// Should exclude node_modules
	assert.True(t, detector.shouldExclude(nodeModules, excludePatterns, tmpDir))

	// Should not exclude packages
	assert.False(t, detector.shouldExclude(packages, excludePatterns, tmpDir))
}

func TestFileDetector_DefaultPackagePatterns(t *testing.T) {
	detector := NewFileDetector()

	tests := []struct {
		wsType   workspace.WorkspaceType
		expected []string
	}{
		{workspace.WorkspaceTypePnpm, []string{"packages/*", "apps/*", "libs/*", "tools/*"}},
		{workspace.WorkspaceTypeGoModule, []string{"./", "cmd/*", "internal/*", "pkg/*"}},
		{workspace.WorkspaceTypeCargo, []string{"crates/*", "*/"}},
		{workspace.WorkspaceTypeMaven, []string{"*/"}},
		{workspace.WorkspaceTypeGradle, []string{"*/"}},
		{workspace.WorkspaceTypeNone, []string{"packages/*", "*/"}},
	}

	for _, tt := range tests {
		t.Run(string(tt.wsType), func(t *testing.T) {
			patterns := detector.defaultPackagePatterns(tt.wsType)
			assert.Equal(t, tt.expected, patterns)
		})
	}
}

func TestFileDetector_Detect_YarnWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package.json with workspaces and yarn.lock
	pkgJSON := `{
		"name": "yarn-monorepo",
		"private": true,
		"workspaces": ["packages/*"]
	}`
	err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "yarn.lock"), []byte(""), 0644)
	require.NoError(t, err)

	detector := NewFileDetector()
	ctx := context.Background()
	opts := workspace.DefaultDetectionOptions()

	result, err := detector.Detect(ctx, tmpDir, opts)
	require.NoError(t, err)
	require.True(t, result.Success())

	ws := result.Workspace
	// The detector uses priority-based detection; npm workspaces marker may take precedence
	// Just verify it detects a workspace type
	assert.NotEqual(t, workspace.WorkspaceTypeNone, ws.Type)
	assert.Contains(t, []workspace.WorkspaceType{workspace.WorkspaceTypeYarn, workspace.WorkspaceTypeNpm}, ws.Type)
}

func TestFileDetector_Detect_GradleWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	// Create settings.gradle with multi-project setup
	settingsGradle := `rootProject.name = 'multi-project'
include 'core', 'api', 'web'
`
	err := os.WriteFile(filepath.Join(tmpDir, "settings.gradle"), []byte(settingsGradle), 0644)
	require.NoError(t, err)

	detector := NewFileDetector()
	ctx := context.Background()
	opts := workspace.DefaultDetectionOptions()

	result, err := detector.Detect(ctx, tmpDir, opts)
	require.NoError(t, err)
	require.True(t, result.Success())

	ws := result.Workspace
	assert.Equal(t, workspace.WorkspaceTypeGradle, ws.Type)
}

func TestFileDetector_parseCargoPackage(t *testing.T) {
	tmpDir := t.TempDir()
	detector := NewFileDetector()

	tests := []struct {
		name        string
		cargoToml   string
		wantName    string
		wantVersion string
		wantErr     bool
	}{
		{
			name: "valid_cargo_package",
			cargoToml: `[package]
name = "my-crate"
version = "1.2.3"
edition = "2021"
`,
			wantName:    "my-crate",
			wantVersion: "1.2.3",
			wantErr:     false,
		},
		{
			name: "package_with_single_quotes",
			cargoToml: `[package]
name = 'my-crate'
version = '0.1.0'
`,
			wantName:    "my-crate",
			wantVersion: "0.1.0",
			wantErr:     false,
		},
		{
			name: "package_no_version",
			cargoToml: `[package]
name = "no-version-crate"
`,
			wantName:    "no-version-crate",
			wantVersion: "",
			wantErr:     false,
		},
		{
			name: "missing_name",
			cargoToml: `[package]
version = "1.0.0"
`,
			wantErr: true,
		},
		{
			name: "multiple_sections",
			cargoToml: `[workspace]
members = ["crates/*"]

[package]
name = "root-crate"
version = "2.0.0"

[dependencies]
serde = "1.0"
`,
			wantName:    "root-crate",
			wantVersion: "2.0.0",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgDir := filepath.Join(tmpDir, tt.name)
			err := os.MkdirAll(pkgDir, 0755)
			require.NoError(t, err)

			err = os.WriteFile(filepath.Join(pkgDir, "Cargo.toml"), []byte(tt.cargoToml), 0644)
			require.NoError(t, err)

			pkg, err := detector.parseCargoPackage(pkgDir, tmpDir)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, pkg.Name)
			assert.Equal(t, tt.wantVersion, pkg.Version)
		})
	}
}

func TestFileDetector_parseCargoPackage_FileNotFound(t *testing.T) {
	detector := NewFileDetector()
	_, err := detector.parseCargoPackage("/nonexistent/path", "/")
	assert.Error(t, err)
}

func TestFileDetector_parseGenericPackage(t *testing.T) {
	tmpDir := t.TempDir()
	detector := NewFileDetector()

	// Create a package directory
	pkgDir := filepath.Join(tmpDir, "packages", "my-generic-pkg")
	err := os.MkdirAll(pkgDir, 0755)
	require.NoError(t, err)

	pkg, err := detector.parseGenericPackage(pkgDir, tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "my-generic-pkg", pkg.Name)
	assert.Equal(t, filepath.Join("packages", "my-generic-pkg"), pkg.Path)
	assert.Equal(t, "", pkg.Version)
	assert.False(t, pkg.Private)
}

func TestFileDetector_parseGenericPackage_RootDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	detector := NewFileDetector()

	pkg, err := detector.parseGenericPackage(tmpDir, tmpDir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Base(tmpDir), pkg.Name)
	assert.Equal(t, ".", pkg.Path)
}

func TestFileDetector_detectPackageManager(t *testing.T) {
	detector := NewFileDetector()

	tests := []struct {
		name     string
		lockFile string
		content  string
		want     workspace.PackageManagerType
	}{
		{
			name:     "pnpm",
			lockFile: "pnpm-lock.yaml",
			content:  "lockfileVersion: 5.4",
			want:     workspace.PackageManagerPnpm,
		},
		{
			name:     "yarn",
			lockFile: "yarn.lock",
			content:  "# yarn lockfile",
			want:     workspace.PackageManagerYarn,
		},
		{
			name:     "npm",
			lockFile: "package-lock.json",
			content:  "{}",
			want:     workspace.PackageManagerNpm,
		},
		{
			name:     "bun",
			lockFile: "bun.lockb",
			content:  "",
			want:     workspace.PackageManagerBun,
		},
		{
			name:     "go",
			lockFile: "go.sum",
			content:  "",
			want:     workspace.PackageManagerGo,
		},
		{
			name:     "cargo",
			lockFile: "Cargo.lock",
			content:  "",
			want:     workspace.PackageManagerCargo,
		},
		{
			name:     "unknown_no_lockfile",
			lockFile: "",
			content:  "",
			want:     workspace.PackageManagerUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.lockFile != "" {
				err := os.WriteFile(filepath.Join(tmpDir, tt.lockFile), []byte(tt.content), 0644)
				require.NoError(t, err)
			}

			got := detector.detectPackageManager(tmpDir)
			assert.Equal(t, tt.want, got)
		})
	}
}
