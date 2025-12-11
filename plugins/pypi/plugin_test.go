// Package main implements tests for the PyPI plugin.
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
	p := &PyPIPlugin{}
	info := p.GetInfo()

	if info.Name != "pypi" {
		t.Errorf("expected name 'pypi', got %s", info.Name)
	}

	if info.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", info.Version)
	}

	if len(info.Hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(info.Hooks))
	}
}

func TestParseConfig(t *testing.T) {
	p := &PyPIPlugin{}

	tests := []struct {
		name     string
		config   map[string]any
		expected *Config
	}{
		{
			name: "all fields",
			config: map[string]any{
				"repository":     "testpypi",
				"repository_url": "https://test.pypi.org/legacy/",
				"username":       "testuser",
				"password":       "testpass",
				"token":          "pypi-token",
				"dist_dir":       "./build/dist",
				"skip_existing":  true,
				"use_twine":      true,
				"build_command":  "python -m build",
				"update_version": true,
				"package_dir":    "./src",
			},
			expected: &Config{
				Repository:    "testpypi",
				RepositoryURL: "https://test.pypi.org/legacy/",
				Username:      "testuser",
				Password:      "testpass",
				Token:         "pypi-token",
				DistDir:       "./build/dist",
				SkipExisting:  true,
				UseTwine:      true,
				BuildCommand:  "python -m build",
				UpdateVersion: true,
				PackageDir:    "./src",
			},
		},
		{
			name:   "defaults",
			config: map[string]any{},
			expected: &Config{
				DistDir:       "dist",
				UseTwine:      true,
				UpdateVersion: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := p.parseConfig(tt.config)

			if cfg.Repository != tt.expected.Repository {
				t.Errorf("Repository: expected %s, got %s", tt.expected.Repository, cfg.Repository)
			}
			if cfg.RepositoryURL != tt.expected.RepositoryURL {
				t.Errorf("RepositoryURL: expected %s, got %s", tt.expected.RepositoryURL, cfg.RepositoryURL)
			}
			if cfg.DistDir != tt.expected.DistDir {
				t.Errorf("DistDir: expected %s, got %s", tt.expected.DistDir, cfg.DistDir)
			}
			if cfg.UseTwine != tt.expected.UseTwine {
				t.Errorf("UseTwine: expected %v, got %v", tt.expected.UseTwine, cfg.UseTwine)
			}
			if cfg.UpdateVersion != tt.expected.UpdateVersion {
				t.Errorf("UpdateVersion: expected %v, got %v", tt.expected.UpdateVersion, cfg.UpdateVersion)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	p := &PyPIPlugin{}

	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid pypi repository",
			config: &Config{
				Repository: "pypi",
			},
			expectError: false,
		},
		{
			name: "valid testpypi repository",
			config: &Config{
				Repository: "testpypi",
			},
			expectError: false,
		},
		{
			name: "valid custom repository URL",
			config: &Config{
				RepositoryURL: "https://custom.pypi.org/simple/",
			},
			expectError: false,
		},
		{
			name: "invalid repository URL scheme",
			config: &Config{
				RepositoryURL: "http://insecure.pypi.org/",
			},
			expectError: true,
		},
		{
			name: "localhost http allowed",
			config: &Config{
				RepositoryURL: "http://localhost:8080/",
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

func TestValidatePackageDir(t *testing.T) {
	p := &PyPIPlugin{}

	// Create a temporary directory for testing
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

func TestUpdatePyprojectVersion(t *testing.T) {
	p := &PyPIPlugin{}

	tmpDir := t.TempDir()
	pyprojectPath := filepath.Join(tmpDir, "pyproject.toml")

	// Write initial pyproject.toml
	initialContent := `[project]
name = "mypackage"
version = "0.1.0"
description = "A test package"
`
	if err := os.WriteFile(pyprojectPath, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Test dry run
	result, err := p.updatePyprojectVersion(pyprojectPath, "1.0.0", true)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if result != "0.1.0 -> 1.0.0" {
		t.Errorf("unexpected dry run result: %s", result)
	}

	// Verify file unchanged after dry run
	content, _ := os.ReadFile(pyprojectPath)
	if string(content) != initialContent {
		t.Error("file was modified during dry run")
	}

	// Test actual update
	result, err = p.updatePyprojectVersion(pyprojectPath, "1.0.0", false)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// Verify file updated
	content, _ = os.ReadFile(pyprojectPath)
	if !contains(string(content), `version = "1.0.0"`) {
		t.Errorf("version not updated in file: %s", string(content))
	}
}

func TestUpdateSetupPyVersion(t *testing.T) {
	p := &PyPIPlugin{}

	tmpDir := t.TempDir()
	setupPath := filepath.Join(tmpDir, "setup.py")

	// Write initial setup.py
	initialContent := `from setuptools import setup

setup(
    name="mypackage",
    version="0.1.0",
    description="A test package",
)
`
	if err := os.WriteFile(setupPath, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Test update
	result, err := p.updateSetupPyVersion(setupPath, "1.0.0", false)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if result != "0.1.0 -> 1.0.0" {
		t.Errorf("unexpected result: %s", result)
	}

	// Verify file updated
	content, _ := os.ReadFile(setupPath)
	if !contains(string(content), `version="1.0.0"`) {
		t.Errorf("version not updated in file: %s", string(content))
	}
}

func TestExecuteDryRun(t *testing.T) {
	p := &PyPIPlugin{}
	ctx := context.Background()

	// Create temp directory with dist folder
	tmpDir := t.TempDir()
	distDir := filepath.Join(tmpDir, "dist")
	if err := os.Mkdir(distDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create pyproject.toml
	pyprojectPath := filepath.Join(tmpDir, "pyproject.toml")
	if err := os.WriteFile(pyprojectPath, []byte(`[project]
name = "testpkg"
version = "0.1.0"
`), 0644); err != nil {
		t.Fatal(err)
	}

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"repository":  "testpypi",
			"package_dir": tmpDir,
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

	if resp.Message != "Would publish to PyPI" {
		t.Errorf("unexpected message: %s", resp.Message)
	}

	if resp.Outputs["version"] != "1.0.0" {
		t.Errorf("unexpected version output: %v", resp.Outputs["version"])
	}

	if resp.Outputs["repository"] != "testpypi" {
		t.Errorf("unexpected repository output: %v", resp.Outputs["repository"])
	}
}

func TestExecuteUnhandledHook(t *testing.T) {
	p := &PyPIPlugin{}
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
	p := &PyPIPlugin{}
	ctx := context.Background()

	// Check if twine or python is available
	_, twineAvailable := exec.LookPath("twine")
	_, pythonAvailable := exec.LookPath("python")
	hasDependency := twineAvailable == nil || pythonAvailable == nil

	tests := []struct {
		name        string
		config      map[string]any
		expectValid bool
	}{
		{
			name: "valid config with token",
			config: map[string]any{
				"token":      "pypi-token",
				"repository": "pypi",
			},
			expectValid: hasDependency,
		},
		{
			name: "valid config with username/password",
			config: map[string]any{
				"username":   "testuser",
				"password":   "testpass",
				"repository": "testpypi",
			},
			expectValid: hasDependency,
		},
		{
			name:        "no credentials configured",
			config:      map[string]any{},
			expectValid: hasDependency, // Only valid if twine/python is available
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

func TestPreparePackageDryRun(t *testing.T) {
	p := &PyPIPlugin{}
	ctx := context.Background()

	// Create temp directory with pyproject.toml
	tmpDir := t.TempDir()
	pyprojectPath := filepath.Join(tmpDir, "pyproject.toml")
	if err := os.WriteFile(pyprojectPath, []byte(`[project]
name = "testpkg"
version = "0.1.0"
`), 0644); err != nil {
		t.Fatal(err)
	}

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPrePublish,
		Config: map[string]any{
			"package_dir":    tmpDir,
			"update_version": true,
			"build_command":  "python -m build",
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

	if !contains(resp.Message, "Would update version") {
		t.Errorf("unexpected message: %s", resp.Message)
	}

	if resp.Outputs["build_command"] != "python -m build" {
		t.Errorf("unexpected build_command output: %v", resp.Outputs["build_command"])
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
