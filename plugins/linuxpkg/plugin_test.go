// Package main implements tests for the Linux package plugin.
package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

func TestGetInfo(t *testing.T) {
	p := &LinuxPkgPlugin{}
	info := p.GetInfo()

	if info.Name != "linuxpkg" {
		t.Errorf("expected name 'linuxpkg', got %s", info.Name)
	}

	if info.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", info.Version)
	}

	if len(info.Hooks) != 1 || info.Hooks[0] != plugin.HookPostPublish {
		t.Errorf("expected HookPostPublish, got %v", info.Hooks)
	}
}

func TestParseConfig(t *testing.T) {
	p := &LinuxPkgPlugin{}

	tests := []struct {
		name     string
		config   map[string]any
		expected *Config
	}{
		{
			name: "all fields",
			config: map[string]any{
				"package_name":    "mytool",
				"package_type":    "both",
				"description":     "A useful tool",
				"maintainer":      "Test User <test@example.com>",
				"homepage":        "https://example.com",
				"license":         "Apache-2.0",
				"architecture":    "arm64",
				"section":         "utils",
				"priority":        "optional",
				"dependencies":    []any{"libc6"},
				"conflicts":       []any{"othertool"},
				"binary_path":     "/path/to/binary",
				"install_path":    "/usr/bin",
				"preinst_script":  "#!/bin/bash\necho pre",
				"postinst_script": "#!/bin/bash\necho post",
				"output_dir":      "./dist",
			},
			expected: &Config{
				PackageName:    "mytool",
				PackageType:    "both",
				Description:    "A useful tool",
				Maintainer:     "Test User <test@example.com>",
				Homepage:       "https://example.com",
				License:        "Apache-2.0",
				Architecture:   "arm64",
				Section:        "utils",
				Priority:       "optional",
				Dependencies:   []string{"libc6"},
				Conflicts:      []string{"othertool"},
				BinaryPath:     "/path/to/binary",
				InstallPath:    "/usr/bin",
				PreInstScript:  "#!/bin/bash\necho pre",
				PostInstScript: "#!/bin/bash\necho post",
				OutputDir:      "./dist",
			},
		},
		{
			name:   "defaults",
			config: map[string]any{},
			expected: &Config{
				PackageType:  "both",
				License:      "MIT",
				Architecture: "amd64",
				Priority:     "optional",
				InstallPath:  "/usr/local/bin",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := p.parseConfig(tt.config)

			if cfg.PackageName != tt.expected.PackageName {
				t.Errorf("PackageName: expected %s, got %s", tt.expected.PackageName, cfg.PackageName)
			}
			if cfg.PackageType != tt.expected.PackageType {
				t.Errorf("PackageType: expected %s, got %s", tt.expected.PackageType, cfg.PackageType)
			}
			if cfg.License != tt.expected.License {
				t.Errorf("License: expected %s, got %s", tt.expected.License, cfg.License)
			}
			if cfg.Architecture != tt.expected.Architecture {
				t.Errorf("Architecture: expected %s, got %s", tt.expected.Architecture, cfg.Architecture)
			}
			if cfg.InstallPath != tt.expected.InstallPath {
				t.Errorf("InstallPath: expected %s, got %s", tt.expected.InstallPath, cfg.InstallPath)
			}
		})
	}
}

func TestParseConfigWithAPTRepo(t *testing.T) {
	p := &LinuxPkgPlugin{}

	config := map[string]any{
		"package_name": "mytool",
		"description":  "A tool",
		"binary_path":  "/path/to/binary",
		"apt_repository": map[string]any{
			"repo_path":    "/var/repo/apt",
			"distribution": "focal",
			"component":    "main",
			"gpg_key_id":   "ABC123",
		},
	}

	cfg := p.parseConfig(config)

	if cfg.APTRepository == nil {
		t.Fatal("expected APTRepository to be set")
	}

	if cfg.APTRepository.RepoPath != "/var/repo/apt" {
		t.Errorf("APT RepoPath: expected /var/repo/apt, got %s", cfg.APTRepository.RepoPath)
	}
	if cfg.APTRepository.Distribution != "focal" {
		t.Errorf("APT Distribution: expected focal, got %s", cfg.APTRepository.Distribution)
	}
}

func TestParseConfigWithYUMRepo(t *testing.T) {
	p := &LinuxPkgPlugin{}

	config := map[string]any{
		"package_name": "mytool",
		"description":  "A tool",
		"binary_path":  "/path/to/binary",
		"yum_repository": map[string]any{
			"repo_path":  "/var/repo/yum",
			"gpg_key_id": "DEF456",
		},
	}

	cfg := p.parseConfig(config)

	if cfg.YUMRepository == nil {
		t.Fatal("expected YUMRepository to be set")
	}

	if cfg.YUMRepository.RepoPath != "/var/repo/yum" {
		t.Errorf("YUM RepoPath: expected /var/repo/yum, got %s", cfg.YUMRepository.RepoPath)
	}
}

func TestValidate(t *testing.T) {
	p := &LinuxPkgPlugin{}
	ctx := context.Background()

	// Create a temporary binary for valid tests
	tmpDir := t.TempDir()
	tmpBinary := filepath.Join(tmpDir, "testbin")
	if err := os.WriteFile(tmpBinary, []byte("test"), 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		config      map[string]any
		expectValid bool
	}{
		{
			name: "valid config",
			config: map[string]any{
				"package_name": "mytool",
				"description":  "A useful tool",
				"binary_path":  tmpBinary,
			},
			expectValid: true,
		},
		{
			name: "missing package_name",
			config: map[string]any{
				"description": "A useful tool",
				"binary_path": tmpBinary,
			},
			expectValid: false,
		},
		{
			name: "missing description",
			config: map[string]any{
				"package_name": "mytool",
				"binary_path":  tmpBinary,
			},
			expectValid: false,
		},
		{
			name: "missing binary_path",
			config: map[string]any{
				"package_name": "mytool",
				"description":  "A useful tool",
			},
			expectValid: false,
		},
		{
			name: "invalid package_type",
			config: map[string]any{
				"package_name": "mytool",
				"description":  "A useful tool",
				"binary_path":  tmpBinary,
				"package_type": "invalid",
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

func TestExecuteDryRun(t *testing.T) {
	p := &LinuxPkgPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"package_name": "mytool",
			"description":  "A useful tool",
			"binary_path":  "/path/to/binary",
			"package_type": "both",
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

	if resp.Message != "Would build Linux packages" {
		t.Errorf("unexpected message: %s", resp.Message)
	}

	if resp.Outputs["package_name"] != "mytool" {
		t.Errorf("unexpected package_name output: %v", resp.Outputs["package_name"])
	}

	if resp.Outputs["version"] != "1.0.0" {
		t.Errorf("unexpected version output: %v", resp.Outputs["version"])
	}
}

func TestExecuteUnhandledHook(t *testing.T) {
	p := &LinuxPkgPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook:   plugin.HookPreVersion,
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

func TestGenerateDebControl(t *testing.T) {
	p := &LinuxPkgPlugin{}

	cfg := &Config{
		PackageName:  "mytool",
		Description:  "A useful tool for testing",
		Maintainer:   "Test User <test@example.com>",
		Homepage:     "https://example.com",
		Section:      "utils",
		Architecture: "amd64",
		Dependencies: []string{"libc6", "libssl1.1"},
		Conflicts:    []string{"othertool"},
	}

	tmpFile := filepath.Join(t.TempDir(), "control")

	err := p.generateDebControl(cfg, "1.0.0", tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read control file: %v", err)
	}

	// Verify content
	contentStr := string(content)
	if !contains(contentStr, "Package: mytool") {
		t.Error("expected Package field")
	}
	if !contains(contentStr, "Version: 1.0.0") {
		t.Error("expected Version field")
	}
	if !contains(contentStr, "Depends: libc6, libssl1.1") {
		t.Error("expected Depends field")
	}
}

func TestGenerateRPMSpec(t *testing.T) {
	p := &LinuxPkgPlugin{}

	cfg := &Config{
		PackageName:  "mytool",
		Description:  "A useful tool for testing. It does many things.",
		Homepage:     "https://example.com",
		License:      "Apache-2.0",
		Architecture: "amd64",
		BinaryPath:   "/path/to/mytool",
		InstallPath:  "/usr/local/bin",
		Dependencies: []string{"glibc"},
	}

	tmpFile := filepath.Join(t.TempDir(), "mytool.spec")

	err := p.generateRPMSpec(cfg, "1.0.0", tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read spec file: %v", err)
	}

	// Verify content
	contentStr := string(content)
	if !contains(contentStr, "Name:           mytool") {
		t.Error("expected Name field")
	}
	if !contains(contentStr, "Version:        1.0.0") {
		t.Error("expected Version field")
	}
	if !contains(contentStr, "License:        Apache-2.0") {
		t.Error("expected License field")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
