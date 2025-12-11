// Package main implements tests for the Hex plugin.
package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/felixgeelhaar/release-pilot/pkg/plugin"
)

func TestGetInfo(t *testing.T) {
	p := &HexPlugin{}
	info := p.GetInfo()

	if info.Name != "hex" {
		t.Errorf("expected name 'hex', got %s", info.Name)
	}

	if info.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", info.Version)
	}

	if len(info.Hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(info.Hooks))
	}
}

func TestParseConfig(t *testing.T) {
	p := &HexPlugin{}

	tests := []struct {
		name     string
		config   map[string]any
		expected *Config
	}{
		{
			name: "all fields",
			config: map[string]any{
				"api_key":        "hex-key",
				"organization":   "myorg",
				"project_dir":    "./elixir-project",
				"update_version": true,
				"replace":        true,
				"revert":         true,
			},
			expected: &Config{
				APIKey:        "hex-key",
				Organization:  "myorg",
				ProjectDir:    "./elixir-project",
				UpdateVersion: true,
				Replace:       true,
				Revert:        true,
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

			if cfg.Organization != tt.expected.Organization {
				t.Errorf("Organization: expected %s, got %s", tt.expected.Organization, cfg.Organization)
			}
			if cfg.UpdateVersion != tt.expected.UpdateVersion {
				t.Errorf("UpdateVersion: expected %v, got %v", tt.expected.UpdateVersion, cfg.UpdateVersion)
			}
			if cfg.Replace != tt.expected.Replace {
				t.Errorf("Replace: expected %v, got %v", tt.expected.Replace, cfg.Replace)
			}
		})
	}
}

func TestValidateProjectDir(t *testing.T) {
	p := &HexPlugin{}

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.validateProjectDir(tt.dir)
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestUpdateVersion(t *testing.T) {
	p := &HexPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()
	mixPath := filepath.Join(tmpDir, "mix.exs")

	// Write initial mix.exs
	initialContent := `defmodule MyApp.MixProject do
  use Mix.Project

  def project do
    [
      app: :my_app,
      version: "0.1.0",
      elixir: "~> 1.14",
      deps: deps()
    ]
  end

  defp deps do
    []
  end
end
`
	if err := os.WriteFile(mixPath, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		ProjectDir:    tmpDir,
		UpdateVersion: true,
	}

	releaseCtx := plugin.ReleaseContext{
		Version: "1.0.0",
	}

	// Test dry run
	resp, err := p.updateVersion(ctx, cfg, releaseCtx, true)
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
	content, _ := os.ReadFile(mixPath)
	if string(content) != initialContent {
		t.Error("file was modified during dry run")
	}

	// Test actual update
	resp, err = p.updateVersion(ctx, cfg, releaseCtx, false)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	// Verify file updated
	content, _ = os.ReadFile(mixPath)
	if !contains(string(content), `version: "1.0.0"`) {
		t.Errorf("version not updated in file: %s", string(content))
	}
}

func TestExecuteDryRun(t *testing.T) {
	p := &HexPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create mix.exs
	mixPath := filepath.Join(tmpDir, "mix.exs")
	if err := os.WriteFile(mixPath, []byte(`defmodule MyApp.MixProject do
  use Mix.Project

  def project do
    [
      app: :my_app,
      version: "0.1.0"
    ]
  end
end
`), 0644); err != nil {
		t.Fatal(err)
	}

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"project_dir":  tmpDir,
			"api_key":      "test-key",
			"organization": "myorg",
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

	if resp.Message != "Would publish to Hex.pm" {
		t.Errorf("unexpected message: %s", resp.Message)
	}

	if resp.Outputs["version"] != "1.0.0" {
		t.Errorf("unexpected version output: %v", resp.Outputs["version"])
	}

	if resp.Outputs["organization"] != "myorg" {
		t.Errorf("unexpected organization output: %v", resp.Outputs["organization"])
	}
}

func TestExecuteUnhandledHook(t *testing.T) {
	p := &HexPlugin{}
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
	p := &HexPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()
	mixPath := filepath.Join(tmpDir, "mix.exs")

	// Create mix.exs
	if err := os.WriteFile(mixPath, []byte(`defmodule MyApp.MixProject do
  use Mix.Project
end
`), 0644); err != nil {
		t.Fatal(err)
	}

	// Check if mix is available
	_, mixAvailable := exec.LookPath("mix")

	tests := []struct {
		name        string
		config      map[string]any
		expectValid bool
	}{
		{
			name: "valid config with api key",
			config: map[string]any{
				"api_key":     "test-key",
				"project_dir": tmpDir,
			},
			expectValid: mixAvailable == nil,
		},
		{
			name:        "no api key warning",
			config:      map[string]any{},
			expectValid: mixAvailable == nil, // Only valid if mix is available
		},
		{
			name: "invalid project dir",
			config: map[string]any{
				"project_dir": "/nonexistent/path",
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

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
