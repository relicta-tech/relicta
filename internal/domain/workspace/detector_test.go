package workspace

import (
	"testing"
)

func TestDefaultWorkspaceConfig(t *testing.T) {
	cfg := DefaultWorkspaceConfig()

	if cfg == nil {
		t.Fatal("DefaultWorkspaceConfig should not return nil")
	}

	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}

	if cfg.Strategy != StrategyIndependent {
		t.Errorf("Strategy = %s, want %s", cfg.Strategy, StrategyIndependent)
	}

	// Check default package paths
	expectedPaths := []string{"packages/*", "apps/*", "libs/*"}
	if len(cfg.PackagePaths) != len(expectedPaths) {
		t.Errorf("PackagePaths length = %d, want %d", len(cfg.PackagePaths), len(expectedPaths))
	}
	for i, path := range expectedPaths {
		if cfg.PackagePaths[i] != path {
			t.Errorf("PackagePaths[%d] = %s, want %s", i, cfg.PackagePaths[i], path)
		}
	}

	// Check default exclude paths
	expectedExcludes := []string{"**/node_modules/**", "**/vendor/**", "**/.git/**"}
	if len(cfg.ExcludePaths) != len(expectedExcludes) {
		t.Errorf("ExcludePaths length = %d, want %d", len(cfg.ExcludePaths), len(expectedExcludes))
	}
	for i, path := range expectedExcludes {
		if cfg.ExcludePaths[i] != path {
			t.Errorf("ExcludePaths[%d] = %s, want %s", i, cfg.ExcludePaths[i], path)
		}
	}

	// Check dependency coordination
	if !cfg.DependencyCoordination.AutoUpdate {
		t.Error("DependencyCoordination.AutoUpdate should be true")
	}
	if cfg.DependencyCoordination.VersionPrefix != "workspace:" {
		t.Errorf("DependencyCoordination.VersionPrefix = %s, want workspace:", cfg.DependencyCoordination.VersionPrefix)
	}
}

func TestDefaultMarkerFiles(t *testing.T) {
	markers := DefaultMarkerFiles()

	if len(markers) == 0 {
		t.Fatal("DefaultMarkerFiles should not return empty slice")
	}

	// Test that expected markers are present
	expectedMarkers := map[string]struct {
		wantType    WorkspaceType
		wantPM      PackageManagerType
		minPriority int
	}{
		"pnpm-workspace.yaml": {WorkspaceTypePnpm, PackageManagerPnpm, 100},
		"lerna.json":          {WorkspaceTypeLerna, PackageManagerNpm, 90},
		"nx.json":             {WorkspaceTypeNx, PackageManagerNpm, 85},
		"turbo.json":          {WorkspaceTypeTurborepo, PackageManagerNpm, 85},
		"package.json":        {WorkspaceTypeNpm, PackageManagerNpm, 50},
		"go.work":             {WorkspaceTypeGoModule, PackageManagerGo, 100},
		"Cargo.toml":          {WorkspaceTypeCargo, PackageManagerCargo, 80},
		"pom.xml":             {WorkspaceTypeMaven, PackageManagerMaven, 70},
		"settings.gradle":     {WorkspaceTypeGradle, PackageManagerGradle, 75},
		"settings.gradle.kts": {WorkspaceTypeGradle, PackageManagerGradle, 75},
		"pnpm-lock.yaml":      {WorkspaceTypeNone, PackageManagerPnpm, 40},
		"yarn.lock":           {WorkspaceTypeNone, PackageManagerYarn, 40},
		"package-lock.json":   {WorkspaceTypeNone, PackageManagerNpm, 30},
		"bun.lockb":           {WorkspaceTypeNone, PackageManagerBun, 40},
	}

	markerMap := make(map[string]MarkerFile)
	for _, m := range markers {
		markerMap[m.Name] = m
	}

	for name, expected := range expectedMarkers {
		m, ok := markerMap[name]
		if !ok {
			t.Errorf("Missing marker: %s", name)
			continue
		}
		if m.Type != expected.wantType {
			t.Errorf("Marker %s: Type = %s, want %s", name, m.Type, expected.wantType)
		}
		if m.PackageManager != expected.wantPM {
			t.Errorf("Marker %s: PackageManager = %s, want %s", name, m.PackageManager, expected.wantPM)
		}
		if m.Priority != expected.minPriority {
			t.Errorf("Marker %s: Priority = %d, want %d", name, m.Priority, expected.minPriority)
		}
	}
}

func TestMarkerFile_RequiresContent(t *testing.T) {
	markers := DefaultMarkerFiles()

	// These markers require content parsing
	requiresContent := map[string]bool{
		"package.json": true,
		"Cargo.toml":   true,
		"pom.xml":      true,
	}

	markerMap := make(map[string]MarkerFile)
	for _, m := range markers {
		markerMap[m.Name] = m
	}

	for name, shouldRequire := range requiresContent {
		m, ok := markerMap[name]
		if !ok {
			t.Errorf("Missing marker: %s", name)
			continue
		}
		if m.RequiresContent != shouldRequire {
			t.Errorf("Marker %s: RequiresContent = %v, want %v", name, m.RequiresContent, shouldRequire)
		}
	}
}
