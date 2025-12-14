// Package main implements tests for the NuGet plugin.
package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

func TestGetInfo(t *testing.T) {
	p := &NuGetPlugin{}
	info := p.GetInfo()

	if info.Name != "nuget" {
		t.Errorf("expected name 'nuget', got %s", info.Name)
	}

	if info.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", info.Version)
	}

	if len(info.Hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(info.Hooks))
	}
}

func TestParseConfig(t *testing.T) {
	p := &NuGetPlugin{}

	tests := []struct {
		name     string
		config   map[string]any
		expected *Config
	}{
		{
			name: "all fields",
			config: map[string]any{
				"api_key":        "nuget-key",
				"source":         "https://custom.nuget.org/",
				"symbol_source":  "https://symbols.nuget.org/",
				"skip_duplicate": true,
				"no_symbols":     true,
				"project_dir":    "./src",
				"configuration":  "Debug",
				"update_version": true,
				"package_id":     "MyPackage",
				"output_dir":     "./artifacts",
			},
			expected: &Config{
				APIKey:        "nuget-key",
				Source:        "https://custom.nuget.org/",
				SymbolSource:  "https://symbols.nuget.org/",
				SkipDuplicate: true,
				NoSymbols:     true,
				ProjectDir:    "./src",
				Configuration: "Debug",
				UpdateVersion: true,
				PackageID:     "MyPackage",
				OutputDir:     "./artifacts",
			},
		},
		{
			name:   "defaults",
			config: map[string]any{},
			expected: &Config{
				Source:        "https://api.nuget.org/v3/index.json",
				Configuration: "Release",
				UpdateVersion: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := p.parseConfig(tt.config)

			if cfg.Source != tt.expected.Source {
				t.Errorf("Source: expected %s, got %s", tt.expected.Source, cfg.Source)
			}
			if cfg.Configuration != tt.expected.Configuration {
				t.Errorf("Configuration: expected %s, got %s", tt.expected.Configuration, cfg.Configuration)
			}
			if cfg.UpdateVersion != tt.expected.UpdateVersion {
				t.Errorf("UpdateVersion: expected %v, got %v", tt.expected.UpdateVersion, cfg.UpdateVersion)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	p := &NuGetPlugin{}

	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid https source",
			config: &Config{
				Source: "https://api.nuget.org/v3/index.json",
			},
			expectError: false,
		},
		{
			name: "invalid http source",
			config: &Config{
				Source: "http://insecure.nuget.org/",
			},
			expectError: true,
		},
		{
			name: "localhost http allowed",
			config: &Config{
				Source: "http://localhost:5000/",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.validateConfig(tt.config)
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateProjectDir(t *testing.T) {
	p := &NuGetPlugin{}

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

func TestFindProjectFile(t *testing.T) {
	p := &NuGetPlugin{}

	tmpDir := t.TempDir()

	// Create .csproj file
	csprojPath := filepath.Join(tmpDir, "MyProject.csproj")
	if err := os.WriteFile(csprojPath, []byte("<Project></Project>"), 0644); err != nil {
		t.Fatal(err)
	}

	found, err := p.findProjectFile(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if found != csprojPath {
		t.Errorf("expected %s, got %s", csprojPath, found)
	}
}

func TestUpdateVersion(t *testing.T) {
	p := &NuGetPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()
	csprojPath := filepath.Join(tmpDir, "MyProject.csproj")

	// Write initial .csproj
	initialContent := `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <Version>0.1.0</Version>
  </PropertyGroup>
</Project>
`
	if err := os.WriteFile(csprojPath, []byte(initialContent), 0644); err != nil {
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
	content, _ := os.ReadFile(csprojPath)
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
	content, _ = os.ReadFile(csprojPath)
	if !contains(string(content), "<Version>1.0.0</Version>") {
		t.Errorf("version not updated in file: %s", string(content))
	}
}

func TestExecuteDryRun(t *testing.T) {
	p := &NuGetPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create .csproj
	csprojPath := filepath.Join(tmpDir, "MyProject.csproj")
	if err := os.WriteFile(csprojPath, []byte(`<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <Version>0.1.0</Version>
  </PropertyGroup>
</Project>
`), 0644); err != nil {
		t.Fatal(err)
	}

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"project_dir": tmpDir,
			"api_key":     "test-key",
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

	if resp.Message != "Would build and push to NuGet" {
		t.Errorf("unexpected message: %s", resp.Message)
	}

	if resp.Outputs["version"] != "1.0.0" {
		t.Errorf("unexpected version output: %v", resp.Outputs["version"])
	}
}

func TestExecuteUnhandledHook(t *testing.T) {
	p := &NuGetPlugin{}
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
	p := &NuGetPlugin{}
	ctx := context.Background()

	// Check if dotnet is available
	_, dotnetAvailable := exec.LookPath("dotnet")

	tests := []struct {
		name        string
		config      map[string]any
		expectValid bool
	}{
		{
			name: "valid config with api key",
			config: map[string]any{
				"api_key": "test-key",
			},
			expectValid: dotnetAvailable == nil,
		},
		{
			name:        "no api key warning",
			config:      map[string]any{},
			expectValid: dotnetAvailable == nil, // Only valid if dotnet is available
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
