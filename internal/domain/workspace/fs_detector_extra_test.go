package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFSDetector_IsExcluded(t *testing.T) {
	d := NewFSDetector()

	tests := []struct {
		name     string
		path     string
		patterns []string
		expected bool
	}{
		{
			name:     "no patterns",
			path:     "packages/foo",
			patterns: nil,
			expected: false,
		},
		{
			name:     "matching pattern",
			path:     "node_modules",
			patterns: []string{"node_modules"},
			expected: true,
		},
		{
			name:     "glob pattern match",
			path:     "test-pkg",
			patterns: []string{"test-*"},
			expected: true,
		},
		{
			name:     "no match",
			path:     "src/main.go",
			patterns: []string{"vendor", "node_modules"},
			expected: false,
		},
		{
			name:     "multiple patterns first matches",
			path:     "vendor",
			patterns: []string{"vendor", "dist"},
			expected: true,
		},
		{
			name:     "empty patterns list",
			path:     "src/main.go",
			patterns: []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.isExcluded(tt.path, tt.patterns)
			if got != tt.expected {
				t.Errorf("isExcluded(%q, %v) = %v, want %v", tt.path, tt.patterns, got, tt.expected)
			}
		})
	}
}

func TestFSDetector_ParsePomXML(t *testing.T) {
	d := NewFSDetector()

	tests := []struct {
		name     string
		data     string
		wantType WorkspaceType
		wantConf float64
	}{
		{
			name: "has modules section",
			data: `<?xml version="1.0"?>
<project>
  <modules>
    <module>core</module>
    <module>api</module>
  </modules>
</project>`,
			wantType: WorkspaceTypeMaven,
			wantConf: 0.9,
		},
		{
			name: "no modules section",
			data: `<?xml version="1.0"?>
<project>
  <groupId>com.example</groupId>
  <artifactId>app</artifactId>
</project>`,
			wantType: WorkspaceTypeNone,
			wantConf: 0,
		},
		{
			name:     "empty data",
			data:     "",
			wantType: WorkspaceTypeNone,
			wantConf: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wsType, conf, err := d.parsePomXML([]byte(tt.data))
			if err != nil {
				t.Fatalf("parsePomXML() error = %v", err)
			}
			if wsType != tt.wantType {
				t.Errorf("type = %v, want %v", wsType, tt.wantType)
			}
			if conf != tt.wantConf {
				t.Errorf("confidence = %v, want %v", conf, tt.wantConf)
			}
		})
	}
}

func TestDefaultConfigResolver_ResolveConfig(t *testing.T) {
	resolver := NewDefaultConfigResolver()
	cfg, err := resolver.ResolveConfig(context.Background(), "/some/path")
	if err != nil {
		t.Fatalf("ResolveConfig() error = %v", err)
	}
	if cfg != nil {
		t.Error("expected nil config from default resolver")
	}
}

func TestDefaultConfigResolver_MergeConfig(t *testing.T) {
	resolver := NewDefaultConfigResolver()

	t.Run("nil explicit", func(t *testing.T) {
		detected := &WorkspaceConfig{Type: WorkspaceTypeGoModule}
		result := resolver.MergeConfig(detected, nil)
		if result.Type != WorkspaceTypeGoModule {
			t.Errorf("expected detected config, got type %v", result.Type)
		}
	})

	t.Run("nil detected", func(t *testing.T) {
		explicit := &WorkspaceConfig{Type: WorkspaceTypeNpm}
		result := resolver.MergeConfig(nil, explicit)
		if result.Type != WorkspaceTypeNpm {
			t.Errorf("expected explicit config, got type %v", result.Type)
		}
	})

	t.Run("explicit overrides detected", func(t *testing.T) {
		detected := &WorkspaceConfig{
			Type:     WorkspaceTypeGoModule,
			Strategy: "independent",
		}
		explicit := &WorkspaceConfig{
			Type: WorkspaceTypeNpm,
		}
		result := resolver.MergeConfig(detected, explicit)
		if result.Type != WorkspaceTypeNpm {
			t.Errorf("type = %v, want %v", result.Type, WorkspaceTypeNpm)
		}
		if result.Strategy != "independent" {
			t.Errorf("strategy = %v, want independent (from detected)", result.Strategy)
		}
	})

	t.Run("explicit overrides all fields", func(t *testing.T) {
		detected := &WorkspaceConfig{
			Type:     WorkspaceTypeGoModule,
			Strategy: "fixed",
		}
		explicit := &WorkspaceConfig{
			Type:         WorkspaceTypeCargo,
			Strategy:     "independent",
			PackagePaths: []string{"crates/*"},
			ExcludePaths: []string{"test-*"},
		}
		result := resolver.MergeConfig(detected, explicit)
		if result.Type != WorkspaceTypeCargo {
			t.Errorf("type = %v, want %v", result.Type, WorkspaceTypeCargo)
		}
		if result.Strategy != "independent" {
			t.Errorf("strategy = %v, want independent", result.Strategy)
		}
		if len(result.PackagePaths) != 1 || result.PackagePaths[0] != "crates/*" {
			t.Errorf("PackagePaths = %v, want [crates/*]", result.PackagePaths)
		}
	})
}

func TestFSDetector_DiscoverPackagesByGlob(t *testing.T) {
	root := t.TempDir()

	// Create package directories
	for _, dir := range []string{"packages/foo", "packages/bar", "apps/web"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	d := NewFSDetector()
	ws := &Workspace{
		RootPath:     root,
		PackagePaths: []string{"packages/*"},
	}

	pkgs, err := d.discoverPackagesByGlob(ws)
	if err != nil {
		t.Fatalf("discoverPackagesByGlob() error = %v", err)
	}

	if len(pkgs) != 2 {
		t.Errorf("expected 2 packages, got %d", len(pkgs))
	}
}

func TestFSDetector_DiscoverPackagesByGlob_WithExcludes(t *testing.T) {
	root := t.TempDir()

	// Create package directories
	for _, dir := range []string{"packages/foo", "packages/bar", "packages/test-util"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	d := NewFSDetector()
	ws := &Workspace{
		RootPath:     root,
		PackagePaths: []string{"packages/*"},
		ExcludePaths: []string{"packages/test-util"},
	}

	pkgs, err := d.discoverPackagesByGlob(ws)
	if err != nil {
		t.Fatalf("discoverPackagesByGlob() error = %v", err)
	}

	if len(pkgs) != 2 {
		t.Errorf("expected 2 packages (excluding test-util), got %d", len(pkgs))
		for _, p := range pkgs {
			t.Logf("  package: %s", p.Path)
		}
	}
}

func TestFSDetector_DiscoverPackagesByGlob_DefaultPatterns(t *testing.T) {
	root := t.TempDir()

	// Create dirs matching default patterns
	for _, dir := range []string{"packages/a", "apps/b", "libs/c"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	d := NewFSDetector()
	ws := &Workspace{
		RootPath:     root,
		PackagePaths: nil, // Will use defaults
	}

	pkgs, err := d.discoverPackagesByGlob(ws)
	if err != nil {
		t.Fatalf("discoverPackagesByGlob() error = %v", err)
	}

	if len(pkgs) != 3 {
		t.Errorf("expected 3 packages from default patterns, got %d", len(pkgs))
	}
}
