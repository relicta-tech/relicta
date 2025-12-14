// Package main implements tests for the Packagist plugin.
package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

func TestGetInfo(t *testing.T) {
	p := &PackagistPlugin{}
	info := p.GetInfo()

	if info.Name != "packagist" {
		t.Errorf("expected name 'packagist', got %s", info.Name)
	}

	if info.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", info.Version)
	}

	if len(info.Hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(info.Hooks))
	}
}

func TestParseConfig(t *testing.T) {
	p := &PackagistPlugin{}

	tests := []struct {
		name     string
		config   map[string]any
		expected *Config
	}{
		{
			name: "all fields",
			config: map[string]any{
				"api_token":      "packagist-token",
				"username":       "myuser",
				"package_name":   "vendor/package",
				"project_dir":    "./php-project",
				"update_version": true,
				"packagist_url":  "https://custom.packagist.org",
				"webhook_url":    "https://packagist.org/api/update-package",
			},
			expected: &Config{
				APIToken:      "packagist-token",
				Username:      "myuser",
				PackageName:   "vendor/package",
				ProjectDir:    "./php-project",
				UpdateVersion: true,
				PackagistURL:  "https://custom.packagist.org",
				WebhookURL:    "https://packagist.org/api/update-package",
			},
		},
		{
			name:   "defaults",
			config: map[string]any{},
			expected: &Config{
				UpdateVersion: true,
				PackagistURL:  "https://packagist.org",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := p.parseConfig(tt.config)

			if cfg.PackagistURL != tt.expected.PackagistURL {
				t.Errorf("PackagistURL: expected %s, got %s", tt.expected.PackagistURL, cfg.PackagistURL)
			}
			if cfg.UpdateVersion != tt.expected.UpdateVersion {
				t.Errorf("UpdateVersion: expected %v, got %v", tt.expected.UpdateVersion, cfg.UpdateVersion)
			}
			if cfg.PackageName != tt.expected.PackageName {
				t.Errorf("PackageName: expected %s, got %s", tt.expected.PackageName, cfg.PackageName)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	p := &PackagistPlugin{}

	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid https URL",
			config: &Config{
				PackagistURL: "https://packagist.org",
			},
			expectError: false,
		},
		{
			name: "invalid http URL",
			config: &Config{
				PackagistURL: "http://insecure.packagist.org",
			},
			expectError: true,
		},
		{
			name: "localhost http allowed",
			config: &Config{
				PackagistURL: "http://localhost:8080",
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
	p := &PackagistPlugin{}

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
	p := &PackagistPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()
	composerPath := filepath.Join(tmpDir, "composer.json")

	// Write initial composer.json
	initialContent := `{
    "name": "vendor/package",
    "version": "0.1.0",
    "description": "A test package"
}
`
	if err := os.WriteFile(composerPath, []byte(initialContent), 0644); err != nil {
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

	// Test actual update
	resp, err = p.updateVersion(ctx, cfg, releaseCtx, false)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	// Verify file updated
	content, _ := os.ReadFile(composerPath)
	if !contains(string(content), `"version": "1.0.0"`) {
		t.Errorf("version not updated in file: %s", string(content))
	}
}

func TestExecuteDryRun(t *testing.T) {
	p := &PackagistPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create composer.json
	composerPath := filepath.Join(tmpDir, "composer.json")
	if err := os.WriteFile(composerPath, []byte(`{
    "name": "vendor/package",
    "version": "0.1.0"
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"project_dir": tmpDir,
			"api_token":   "test-token",
			"username":    "testuser",
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

	if resp.Message != "Would notify Packagist to update package" {
		t.Errorf("unexpected message: %s", resp.Message)
	}

	if resp.Outputs["version"] != "1.0.0" {
		t.Errorf("unexpected version output: %v", resp.Outputs["version"])
	}

	if resp.Outputs["package_name"] != "vendor/package" {
		t.Errorf("unexpected package_name output: %v", resp.Outputs["package_name"])
	}
}

func TestExecuteUnhandledHook(t *testing.T) {
	p := &PackagistPlugin{}
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
	p := &PackagistPlugin{}
	ctx := context.Background()

	tests := []struct {
		name        string
		config      map[string]any
		expectValid bool
	}{
		{
			name: "valid config with credentials",
			config: map[string]any{
				"api_token": "test-token",
				"username":  "testuser",
			},
			expectValid: true,
		},
		{
			name:        "no credentials warning",
			config:      map[string]any{},
			expectValid: true, // Just a warning
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
