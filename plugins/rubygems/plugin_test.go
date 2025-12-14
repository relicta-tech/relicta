// Package main implements tests for the RubyGems plugin.
package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

func TestGetInfo(t *testing.T) {
	p := &RubyGemsPlugin{}
	info := p.GetInfo()

	if info.Name != "rubygems" {
		t.Errorf("expected name 'rubygems', got %s", info.Name)
	}

	if info.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", info.Version)
	}

	if len(info.Hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(info.Hooks))
	}
}

func TestParseConfig(t *testing.T) {
	p := &RubyGemsPlugin{}

	tests := []struct {
		name     string
		config   map[string]any
		expected *Config
	}{
		{
			name: "all fields",
			config: map[string]any{
				"api_key":          "rubygems-key",
				"host":             "https://custom.rubygems.org/",
				"gem_dir":          "./gem",
				"gem_name":         "mygem",
				"update_version":   true,
				"version_file":     "lib/mygem/version.rb",
				"allow_prerelease": true,
			},
			expected: &Config{
				APIKey:          "rubygems-key",
				Host:            "https://custom.rubygems.org/",
				GemDir:          "./gem",
				GemName:         "mygem",
				UpdateVersion:   true,
				VersionFile:     "lib/mygem/version.rb",
				AllowPrerelease: true,
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

			if cfg.Host != tt.expected.Host {
				t.Errorf("Host: expected %s, got %s", tt.expected.Host, cfg.Host)
			}
			if cfg.UpdateVersion != tt.expected.UpdateVersion {
				t.Errorf("UpdateVersion: expected %v, got %v", tt.expected.UpdateVersion, cfg.UpdateVersion)
			}
			if cfg.GemName != tt.expected.GemName {
				t.Errorf("GemName: expected %s, got %s", tt.expected.GemName, cfg.GemName)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	p := &RubyGemsPlugin{}

	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid https host",
			config: &Config{
				Host: "https://rubygems.org/",
			},
			expectError: false,
		},
		{
			name: "invalid http host",
			config: &Config{
				Host: "http://insecure.rubygems.org/",
			},
			expectError: true,
		},
		{
			name: "localhost http allowed",
			config: &Config{
				Host: "http://localhost:9292/",
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

func TestValidateGemDir(t *testing.T) {
	p := &RubyGemsPlugin{}

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
			_, err := p.validateGemDir(tt.dir)
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestFindGemspec(t *testing.T) {
	p := &RubyGemsPlugin{}

	tmpDir := t.TempDir()

	// Create .gemspec file
	gemspecPath := filepath.Join(tmpDir, "mygem.gemspec")
	if err := os.WriteFile(gemspecPath, []byte("Gem::Specification.new"), 0644); err != nil {
		t.Fatal(err)
	}

	found, err := p.findGemspec(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if found != gemspecPath {
		t.Errorf("expected %s, got %s", gemspecPath, found)
	}
}

func TestFindVersionFile(t *testing.T) {
	p := &RubyGemsPlugin{}

	tmpDir := t.TempDir()

	// Create lib/mygem/version.rb
	libDir := filepath.Join(tmpDir, "lib", "mygem")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatal(err)
	}

	versionPath := filepath.Join(libDir, "version.rb")
	if err := os.WriteFile(versionPath, []byte(`VERSION = "0.1.0"`), 0644); err != nil {
		t.Fatal(err)
	}

	found := p.findVersionFile(tmpDir)
	if found == "" {
		t.Error("expected to find version file")
	}
}

func TestUpdateVersion(t *testing.T) {
	p := &RubyGemsPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create gemspec
	gemspecPath := filepath.Join(tmpDir, "mygem.gemspec")
	if err := os.WriteFile(gemspecPath, []byte(`Gem::Specification.new do |spec|
  spec.name = "mygem"
  spec.version = "0.1.0"
end
`), 0644); err != nil {
		t.Fatal(err)
	}

	// Create version.rb
	libDir := filepath.Join(tmpDir, "lib", "mygem")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatal(err)
	}

	versionPath := filepath.Join(libDir, "version.rb")
	initialContent := `module MyGem
  VERSION = "0.1.0"
end
`
	if err := os.WriteFile(versionPath, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		GemDir:        tmpDir,
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
	content, _ := os.ReadFile(versionPath)
	if !contains(string(content), `VERSION = "1.0.0"`) {
		t.Errorf("version not updated in file: %s", string(content))
	}
}

func TestExecuteDryRun(t *testing.T) {
	p := &RubyGemsPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create .gemspec
	gemspecPath := filepath.Join(tmpDir, "mygem.gemspec")
	if err := os.WriteFile(gemspecPath, []byte(`Gem::Specification.new do |spec|
  spec.name = "mygem"
  spec.version = "0.1.0"
end
`), 0644); err != nil {
		t.Fatal(err)
	}

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"gem_dir": tmpDir,
			"api_key": "test-key",
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

	if resp.Message != "Would build and push gem" {
		t.Errorf("unexpected message: %s", resp.Message)
	}

	if resp.Outputs["version"] != "1.0.0" {
		t.Errorf("unexpected version output: %v", resp.Outputs["version"])
	}
}

func TestExecuteUnhandledHook(t *testing.T) {
	p := &RubyGemsPlugin{}
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
	p := &RubyGemsPlugin{}
	ctx := context.Background()

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
			expectValid: true,
		},
		{
			name:        "no api key warning",
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
