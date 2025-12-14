// Package main implements tests for the crates.io plugin.
package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

func TestGetInfo(t *testing.T) {
	p := &CratesPlugin{}
	info := p.GetInfo()

	if info.Name != "crates" {
		t.Errorf("expected name 'crates', got %s", info.Name)
	}

	if info.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", info.Version)
	}

	if len(info.Hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(info.Hooks))
	}
}

func TestParseConfig(t *testing.T) {
	p := &CratesPlugin{}

	tests := []struct {
		name     string
		config   map[string]any
		expected *Config
	}{
		{
			name: "all fields",
			config: map[string]any{
				"token":               "test-token",
				"registry":            "my-registry",
				"allow_dirty":         true,
				"no_verify":           true,
				"features":            []any{"feature1", "feature2"},
				"all_features":        false,
				"no_default_features": true,
				"package_dir":         "./src",
				"update_version":      true,
				"workspace":           true,
				"package":             "my-crate",
			},
			expected: &Config{
				Token:             "test-token",
				Registry:          "my-registry",
				AllowDirty:        true,
				NoVerify:          true,
				Features:          []string{"feature1", "feature2"},
				AllFeatures:       false,
				NoDefaultFeatures: true,
				PackageDir:        "./src",
				UpdateVersion:     true,
				Workspace:         true,
				Package:           "my-crate",
			},
		},
		{
			name:   "defaults",
			config: map[string]any{},
			expected: &Config{
				UpdateVersion: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := p.parseConfig(tt.config)

			if cfg.Registry != tt.expected.Registry {
				t.Errorf("Registry: expected %s, got %s", tt.expected.Registry, cfg.Registry)
			}
			if cfg.AllowDirty != tt.expected.AllowDirty {
				t.Errorf("AllowDirty: expected %v, got %v", tt.expected.AllowDirty, cfg.AllowDirty)
			}
			if cfg.UpdateVersion != tt.expected.UpdateVersion {
				t.Errorf("UpdateVersion: expected %v, got %v", tt.expected.UpdateVersion, cfg.UpdateVersion)
			}
			if cfg.Package != tt.expected.Package {
				t.Errorf("Package: expected %s, got %s", tt.expected.Package, cfg.Package)
			}
		})
	}
}

func TestValidatePackageDir(t *testing.T) {
	p := &CratesPlugin{}

	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		dir         string
		expectError bool
	}{
		{
			name:        "empty dir returns current",
			dir:         "",
			expectError: false,
		},
		{
			name:        "valid directory",
			dir:         tmpDir,
			expectError: false,
		},
		{
			name:        "path traversal blocked",
			dir:         "../../../etc",
			expectError: true,
		},
		{
			name:        "nonexistent directory",
			dir:         "/nonexistent/path/to/dir",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.validatePackageDir(tt.dir)
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestUpdateCargoVersion(t *testing.T) {
	p := &CratesPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()
	cargoPath := filepath.Join(tmpDir, "Cargo.toml")

	// Write initial Cargo.toml
	initialContent := `[package]
name = "my-crate"
version = "0.1.0"
edition = "2021"

[dependencies]
serde = "1.0"
`
	if err := os.WriteFile(cargoPath, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		PackageDir:    tmpDir,
		UpdateVersion: true,
	}

	releaseCtx := plugin.ReleaseContext{
		Version: "1.0.0",
		TagName: "v1.0.0",
	}

	// Test dry run
	resp, err := p.updateCargoVersion(ctx, cfg, releaseCtx, true)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	if resp.Outputs["old_version"] != "0.1.0" {
		t.Errorf("unexpected old_version: %v", resp.Outputs["old_version"])
	}

	// Verify file unchanged after dry run
	content, _ := os.ReadFile(cargoPath)
	if string(content) != initialContent {
		t.Error("file was modified during dry run")
	}

	// Test actual update
	resp, err = p.updateCargoVersion(ctx, cfg, releaseCtx, false)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	// Verify file updated
	content, _ = os.ReadFile(cargoPath)
	if !contains(string(content), `version = "1.0.0"`) {
		t.Errorf("version not updated in file: %s", string(content))
	}
}

func TestExecuteDryRun(t *testing.T) {
	p := &CratesPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create Cargo.toml
	cargoPath := filepath.Join(tmpDir, "Cargo.toml")
	if err := os.WriteFile(cargoPath, []byte(`[package]
name = "testcrate"
version = "0.1.0"
`), 0644); err != nil {
		t.Fatal(err)
	}

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"package_dir": tmpDir,
			"registry":    "my-registry",
		},
		Context: plugin.ReleaseContext{
			Version: "1.0.0",
			TagName: "v1.0.0",
		},
		DryRun: true,
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	if resp.Message != "Would publish to crates.io" {
		t.Errorf("unexpected message: %s", resp.Message)
	}

	if resp.Outputs["version"] != "1.0.0" {
		t.Errorf("unexpected version output: %v", resp.Outputs["version"])
	}

	if resp.Outputs["registry"] != "my-registry" {
		t.Errorf("unexpected registry output: %v", resp.Outputs["registry"])
	}
}

func TestExecuteUnhandledHook(t *testing.T) {
	p := &CratesPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook:   plugin.HookOnSuccess,
		Config: map[string]any{},
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success for unhandled hook")
	}
}

func TestValidate(t *testing.T) {
	p := &CratesPlugin{}
	ctx := context.Background()

	tests := []struct {
		name        string
		config      map[string]any
		expectValid bool
	}{
		{
			name: "valid config with token",
			config: map[string]any{
				"token": "test-token",
			},
			expectValid: true,
		},
		{
			name:        "no token configured",
			config:      map[string]any{},
			expectValid: true, // Just a warning
		},
		{
			name: "valid package name",
			config: map[string]any{
				"package": "my-crate",
			},
			expectValid: true,
		},
		{
			name: "invalid package name",
			config: map[string]any{
				"package": "123-invalid",
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := p.Validate(ctx, tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Valid != tt.expectValid {
				t.Errorf("expected valid=%v, got valid=%v, errors=%v", tt.expectValid, resp.Valid, resp.Errors)
			}
		})
	}
}

func TestExecutePrePublish(t *testing.T) {
	p := &CratesPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()
	cargoPath := filepath.Join(tmpDir, "Cargo.toml")
	if err := os.WriteFile(cargoPath, []byte(`[package]
name = "testcrate"
version = "0.1.0"
`), 0644); err != nil {
		t.Fatal(err)
	}

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPrePublish,
		Config: map[string]any{
			"package_dir":    tmpDir,
			"update_version": true,
		},
		Context: plugin.ReleaseContext{
			Version: "1.0.0",
		},
		DryRun: true,
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	if !contains(resp.Message, "Would update Cargo.toml") {
		t.Errorf("unexpected message: %s", resp.Message)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
