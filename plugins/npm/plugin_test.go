// Package main implements tests for the npm plugin.
package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

func TestValidatePackageDir(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir, err := os.MkdirTemp("", "npm-plugin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "packages", "my-package")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Create a file in the subdirectory (to simulate package.json)
	testFile := filepath.Join(subDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create an outside directory for testing path escapes
	outsideDir, err := os.MkdirTemp("", "outside-test-*")
	if err != nil {
		t.Fatalf("failed to create outside dir: %v", err)
	}
	defer os.RemoveAll(outsideDir)

	// Save original working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	// Change to temp dir as working directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	tests := []struct {
		name        string
		dir         string
		wantErr     bool
		errContains string
	}{
		{
			name:    "empty string defaults to current dir",
			dir:     "",
			wantErr: false,
		},
		{
			name:    "current directory",
			dir:     ".",
			wantErr: false,
		},
		{
			name:    "valid subdirectory",
			dir:     "packages/my-package",
			wantErr: false,
		},
		{
			name:    "valid absolute path within cwd",
			dir:     subDir,
			wantErr: false,
		},
		{
			name:        "path traversal with ..",
			dir:         "..",
			wantErr:     true,
			errContains: "path traversal not allowed",
		},
		{
			name:        "path traversal nested",
			dir:         "packages/../../../etc",
			wantErr:     true,
			errContains: "path traversal not allowed",
		},
		{
			name:        "absolute path outside cwd",
			dir:         outsideDir,
			wantErr:     true,
			errContains: "must be within",
		},
		{
			name:        "non-existent directory",
			dir:         "nonexistent",
			wantErr:     true,
			errContains: "not accessible",
		},
		{
			name:        "file instead of directory",
			dir:         "packages/my-package/test.txt",
			wantErr:     true,
			errContains: "not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validatePackageDir(tt.dir)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validatePackageDir(%q) = %q, want error containing %q", tt.dir, result, tt.errContains)
				} else if tt.errContains != "" && !containsSubstring(err.Error(), tt.errContains) {
					t.Errorf("validatePackageDir(%q) error = %q, want error containing %q", tt.dir, err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("validatePackageDir(%q) unexpected error: %v", tt.dir, err)
				}
				if result == "" {
					t.Errorf("validatePackageDir(%q) returned empty path", tt.dir)
				}
			}
		})
	}
}

func TestValidatePackageDir_Symlinks(t *testing.T) {
	// Skip on systems that don't support symlinks
	if !supportsSymlinks() {
		t.Skip("symlinks not supported on this system")
	}

	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "symlink-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	outsideDir, err := os.MkdirTemp("", "outside-symlink-*")
	if err != nil {
		t.Fatalf("failed to create outside dir: %v", err)
	}
	defer os.RemoveAll(outsideDir)

	// Create a symlink inside tmpDir pointing to outside directory
	symlinkPath := filepath.Join(tmpDir, "sneaky-link")
	if err := os.Symlink(outsideDir, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Save original working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	// Test that symlink to outside directory is rejected
	_, err = validatePackageDir("sneaky-link")
	if err == nil {
		t.Error("validatePackageDir should reject symlink to directory outside working directory")
	}
}

func TestValidateRegistry(t *testing.T) {
	tests := []struct {
		name     string
		registry string
		wantErr  bool
	}{
		{
			name:     "empty string",
			registry: "",
			wantErr:  false,
		},
		{
			name:     "valid https registry",
			registry: "https://registry.npmjs.org",
			wantErr:  false,
		},
		{
			name:     "valid https with path",
			registry: "https://npm.pkg.github.com/@myorg",
			wantErr:  false,
		},
		{
			name:     "localhost http allowed",
			registry: "http://localhost:4873",
			wantErr:  false,
		},
		{
			name:     "127.0.0.1 http allowed",
			registry: "http://127.0.0.1:4873",
			wantErr:  false,
		},
		{
			name:     "non-localhost http rejected",
			registry: "http://registry.example.com",
			wantErr:  true,
		},
		{
			name:     "invalid URL",
			registry: "not-a-url",
			wantErr:  true,
		},
		{
			name:     "newline injection",
			registry: "https://registry.npmjs.org\nmalicious",
			wantErr:  true,
		},
		{
			name:     "carriage return injection",
			registry: "https://registry.npmjs.org\rmalicious",
			wantErr:  true,
		},
		{
			name:     "tab injection",
			registry: "https://registry.npmjs.org\tmalicious",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRegistry(tt.registry)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRegistry(%q) error = %v, wantErr %v", tt.registry, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTag(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		wantErr bool
	}{
		{name: "empty string", tag: "", wantErr: false},
		{name: "latest", tag: "latest", wantErr: false},
		{name: "next", tag: "next", wantErr: false},
		{name: "beta", tag: "beta", wantErr: false},
		{name: "alpha-1", tag: "alpha-1", wantErr: false},
		{name: "canary.123", tag: "canary.123", wantErr: false},
		{name: "with underscore", tag: "my_tag", wantErr: false},
		{name: "starts with number", tag: "1.0.0-beta", wantErr: false},
		{name: "too long", tag: string(make([]byte, 129)), wantErr: true},
		{name: "invalid chars space", tag: "my tag", wantErr: true},
		{name: "invalid chars special", tag: "my@tag", wantErr: true},
		{name: "starts with dash", tag: "-invalid", wantErr: true},
		{name: "starts with dot", tag: ".hidden", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTag(tt.tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTag(%q) error = %v, wantErr %v", tt.tag, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAccess(t *testing.T) {
	tests := []struct {
		name    string
		access  string
		wantErr bool
	}{
		{name: "empty string", access: "", wantErr: false},
		{name: "public", access: "public", wantErr: false},
		{name: "restricted", access: "restricted", wantErr: false},
		{name: "invalid", access: "private", wantErr: true},
		{name: "uppercase", access: "PUBLIC", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAccess(tt.access)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAccess(%q) error = %v, wantErr %v", tt.access, err, tt.wantErr)
			}
		})
	}
}

func TestValidateOTP(t *testing.T) {
	tests := []struct {
		name    string
		otp     string
		wantErr bool
	}{
		{name: "empty string", otp: "", wantErr: false},
		{name: "6 digits", otp: "123456", wantErr: false},
		{name: "7 digits", otp: "1234567", wantErr: false},
		{name: "8 digits", otp: "12345678", wantErr: false},
		{name: "5 digits too short", otp: "12345", wantErr: true},
		{name: "9 digits too long", otp: "123456789", wantErr: true},
		{name: "letters", otp: "abcdef", wantErr: true},
		{name: "mixed", otp: "123abc", wantErr: true},
		{name: "special chars", otp: "123-456", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOTP(tt.otp)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateOTP(%q) error = %v, wantErr %v", tt.otp, err, tt.wantErr)
			}
		})
	}
}

func TestNpmPlugin_ValidateConfig(t *testing.T) {
	plugin := &NpmPlugin{}

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Registry: "https://registry.npmjs.org",
				Tag:      "latest",
				Access:   "public",
			},
			wantErr: false,
		},
		{
			name: "invalid registry",
			config: &Config{
				Registry: "http://external.example.com",
				Tag:      "latest",
			},
			wantErr: true,
		},
		{
			name: "invalid tag",
			config: &Config{
				Tag: "invalid tag with space",
			},
			wantErr: true,
		},
		{
			name: "invalid access",
			config: &Config{
				Access: "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid OTP",
			config: &Config{
				OTP: "abc",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := plugin.validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseConfig(t *testing.T) {
	plugin := &NpmPlugin{}

	tests := []struct {
		name     string
		raw      map[string]any
		expected *Config
	}{
		{
			name: "empty config uses defaults",
			raw:  map[string]any{},
			expected: &Config{
				Tag:           "latest",
				UpdateVersion: true,
			},
		},
		{
			name: "all fields set",
			raw: map[string]any{
				"registry":       "https://registry.npmjs.org",
				"tag":            "next",
				"access":         "public",
				"otp":            "123456",
				"dry_run":        true,
				"package_dir":    "./packages/core",
				"update_version": false,
			},
			expected: &Config{
				Registry:      "https://registry.npmjs.org",
				Tag:           "next",
				Access:        "public",
				OTP:           "123456",
				DryRun:        true,
				PackageDir:    "./packages/core",
				UpdateVersion: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := plugin.parseConfig(tt.raw)

			if result.Registry != tt.expected.Registry {
				t.Errorf("Registry = %q, want %q", result.Registry, tt.expected.Registry)
			}
			if result.Tag != tt.expected.Tag {
				t.Errorf("Tag = %q, want %q", result.Tag, tt.expected.Tag)
			}
			if result.Access != tt.expected.Access {
				t.Errorf("Access = %q, want %q", result.Access, tt.expected.Access)
			}
			if result.OTP != tt.expected.OTP {
				t.Errorf("OTP = %q, want %q", result.OTP, tt.expected.OTP)
			}
			if result.DryRun != tt.expected.DryRun {
				t.Errorf("DryRun = %v, want %v", result.DryRun, tt.expected.DryRun)
			}
			if result.PackageDir != tt.expected.PackageDir {
				t.Errorf("PackageDir = %q, want %q", result.PackageDir, tt.expected.PackageDir)
			}
			if result.UpdateVersion != tt.expected.UpdateVersion {
				t.Errorf("UpdateVersion = %v, want %v", result.UpdateVersion, tt.expected.UpdateVersion)
			}
		})
	}
}

// containsSubstring checks if s contains substr
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// supportsSymlinks checks if the current system supports symlinks
func supportsSymlinks() bool {
	tmpDir, err := os.MkdirTemp("", "symlink-check-*")
	if err != nil {
		return false
	}
	defer os.RemoveAll(tmpDir)

	testDir := filepath.Join(tmpDir, "testdir")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		return false
	}

	linkPath := filepath.Join(tmpDir, "link")
	if err := os.Symlink(testDir, linkPath); err != nil {
		return false
	}

	return true
}

func TestNpmPlugin_GetInfo(t *testing.T) {
	p := &NpmPlugin{}
	info := p.GetInfo()

	if info.Name != "npm" {
		t.Errorf("GetInfo().Name = %q, want %q", info.Name, "npm")
	}

	if info.Version != "1.0.0" {
		t.Errorf("GetInfo().Version = %q, want %q", info.Version, "1.0.0")
	}

	if info.Author != "Relicta Team" {
		t.Errorf("GetInfo().Author = %q, want %q", info.Author, "Relicta Team")
	}

	expectedHooks := []plugin.Hook{plugin.HookPrePublish, plugin.HookPostPublish}
	if len(info.Hooks) != len(expectedHooks) {
		t.Errorf("GetInfo().Hooks len = %d, want %d", len(info.Hooks), len(expectedHooks))
	}

	if info.Description == "" {
		t.Error("GetInfo().Description should not be empty")
	}

	if info.ConfigSchema == "" {
		t.Error("GetInfo().ConfigSchema should not be empty")
	}
}

func TestNpmPlugin_Execute_PrePublish_UpdateVersion(t *testing.T) {
	p := &NpmPlugin{}

	// Create temp directory with package.json inside current working directory
	cwd, _ := os.Getwd()
	tmpDir, err := os.MkdirTemp(cwd, "npm-execute-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := map[string]any{
		"name":    "test-package",
		"version": "1.0.0",
	}
	data, _ := json.Marshal(packageJSON)
	packagePath := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(packagePath, data, 0644); err != nil {
		t.Fatalf("failed to create package.json: %v", err)
	}

	// Use just the directory name relative to cwd
	relDir := filepath.Base(tmpDir)

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPrePublish,
		Config: map[string]any{
			"update_version": true,
			"package_dir":    relDir,
		},
		Context: plugin.ReleaseContext{
			Version: "2.0.0",
		},
		DryRun: true,
	}

	resp, err := p.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !resp.Success {
		t.Errorf("Execute() Success = false, Error = %s", resp.Error)
	}

	if resp.Message == "" {
		t.Error("Execute() Message should not be empty")
	}
}

func TestNpmPlugin_Execute_PrePublish_UpdateVersionDisabled(t *testing.T) {
	p := &NpmPlugin{}

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPrePublish,
		Config: map[string]any{
			"update_version": false,
		},
		Context: plugin.ReleaseContext{
			Version: "2.0.0",
		},
	}

	resp, err := p.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !resp.Success {
		t.Errorf("Execute() Success = false, Error = %s", resp.Error)
	}

	if resp.Message != "Version update disabled" {
		t.Errorf("Execute() Message = %q, want %q", resp.Message, "Version update disabled")
	}
}

func TestNpmPlugin_Execute_PostPublish_DryRun(t *testing.T) {
	p := &NpmPlugin{}

	// Create temp directory with package.json inside current working directory
	cwd, _ := os.Getwd()
	tmpDir, err := os.MkdirTemp(cwd, "npm-publish-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := map[string]any{
		"name":    "test-package",
		"version": "1.0.0",
	}
	data, _ := json.Marshal(packageJSON)
	packagePath := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(packagePath, data, 0644); err != nil {
		t.Fatalf("failed to create package.json: %v", err)
	}

	relDir := filepath.Base(tmpDir)

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"package_dir": relDir,
			"tag":         "beta",
			"access":      "public",
		},
		Context: plugin.ReleaseContext{
			Version: "1.0.0",
		},
		DryRun: true,
	}

	resp, err := p.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !resp.Success {
		t.Errorf("Execute() Success = false, Error = %s", resp.Error)
	}

	if resp.Outputs == nil {
		t.Fatal("Execute() Outputs should not be nil")
	}

	if resp.Outputs["package"] != "test-package" {
		t.Errorf("Execute() Outputs[package] = %v, want test-package", resp.Outputs["package"])
	}
}

func TestNpmPlugin_Execute_PostPublish_PrivatePackage(t *testing.T) {
	p := &NpmPlugin{}

	// Create temp directory with private package.json inside current working directory
	cwd, _ := os.Getwd()
	tmpDir, err := os.MkdirTemp(cwd, "npm-private-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := map[string]any{
		"name":    "test-package",
		"version": "1.0.0",
		"private": true,
	}
	data, _ := json.Marshal(packageJSON)
	packagePath := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(packagePath, data, 0644); err != nil {
		t.Fatalf("failed to create package.json: %v", err)
	}

	relDir := filepath.Base(tmpDir)

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"package_dir": relDir,
		},
		Context: plugin.ReleaseContext{
			Version: "1.0.0",
		},
	}

	resp, err := p.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !resp.Success {
		t.Errorf("Execute() Success = false, Error = %s", resp.Error)
	}

	if resp.Message != "Package is private, skipping npm publish" {
		t.Errorf("Execute() Message = %q, want private skip message", resp.Message)
	}
}

func TestNpmPlugin_Execute_UnhandledHook(t *testing.T) {
	p := &NpmPlugin{}

	req := plugin.ExecuteRequest{
		Hook:   plugin.HookPreInit,
		Config: map[string]any{},
		Context: plugin.ReleaseContext{
			Version: "1.0.0",
		},
	}

	resp, err := p.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !resp.Success {
		t.Error("Execute() Success = false, want true for unhandled hook")
	}

	if resp.Message == "" {
		t.Error("Execute() Message should indicate hook not handled")
	}
}

func TestNpmPlugin_updatePackageVersion(t *testing.T) {
	p := &NpmPlugin{}

	t.Run("updates version successfully", func(t *testing.T) {
		cwd, _ := os.Getwd()
		tmpDir, err := os.MkdirTemp(cwd, "npm-update-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		packageJSON := map[string]any{
			"name":    "test-package",
			"version": "1.0.0",
		}
		data, _ := json.MarshalIndent(packageJSON, "", "  ")
		packagePath := filepath.Join(tmpDir, "package.json")
		if err := os.WriteFile(packagePath, data, 0644); err != nil {
			t.Fatalf("failed to create package.json: %v", err)
		}

		relDir := filepath.Base(tmpDir)

		cfg := &Config{
			PackageDir: relDir,
		}

		ctx := plugin.ReleaseContext{
			Version: "2.0.0",
		}

		resp, err := p.updatePackageVersion(context.Background(), cfg, ctx, false)
		if err != nil {
			t.Fatalf("updatePackageVersion() error = %v", err)
		}

		if !resp.Success {
			t.Errorf("updatePackageVersion() Success = false, Error = %s", resp.Error)
		}

		// Verify version was updated
		updatedData, _ := os.ReadFile(packagePath)
		var updated map[string]any
		json.Unmarshal(updatedData, &updated)
		if updated["version"] != "2.0.0" {
			t.Errorf("Version not updated, got %v", updated["version"])
		}
	})

	t.Run("fails with invalid package dir", func(t *testing.T) {
		cfg := &Config{
			PackageDir: "../../../etc",
		}

		ctx := plugin.ReleaseContext{
			Version: "2.0.0",
		}

		resp, err := p.updatePackageVersion(context.Background(), cfg, ctx, false)
		if err != nil {
			t.Fatalf("updatePackageVersion() error = %v", err)
		}

		if resp.Success {
			t.Error("updatePackageVersion() Success = true, want false for invalid dir")
		}
	})

	t.Run("fails with missing package.json", func(t *testing.T) {
		cwd, _ := os.Getwd()
		tmpDir, _ := os.MkdirTemp(cwd, "npm-missing-test-*")
		defer os.RemoveAll(tmpDir)

		relDir := filepath.Base(tmpDir)

		cfg := &Config{
			PackageDir: relDir,
		}

		ctx := plugin.ReleaseContext{
			Version: "2.0.0",
		}

		resp, err := p.updatePackageVersion(context.Background(), cfg, ctx, false)
		if err != nil {
			t.Fatalf("updatePackageVersion() error = %v", err)
		}

		if resp.Success {
			t.Error("updatePackageVersion() Success = true, want false for missing package.json")
		}
	})
}

func TestNpmPlugin_Validate(t *testing.T) {
	p := &NpmPlugin{}

	tests := []struct {
		name       string
		config     map[string]any
		wantValid  bool
		wantErrors int
	}{
		{
			name:       "valid config",
			config:     map[string]any{"access": "public"},
			wantValid:  true,
			wantErrors: 0,
		},
		{
			name:       "invalid access",
			config:     map[string]any{"access": "invalid"},
			wantValid:  false,
			wantErrors: 1,
		},
		{
			name:       "empty config",
			config:     map[string]any{},
			wantValid:  true,
			wantErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := p.Validate(context.Background(), tt.config)
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}

			if resp.Valid != tt.wantValid {
				t.Errorf("Validate() Valid = %v, want %v", resp.Valid, tt.wantValid)
			}

			if len(resp.Errors) != tt.wantErrors {
				t.Errorf("Validate() Errors len = %d, want %d", len(resp.Errors), tt.wantErrors)
			}
		})
	}
}

func TestNpmPlugin_publishPackage_InvalidConfig(t *testing.T) {
	p := &NpmPlugin{}

	cfg := &Config{
		Registry: "http://malicious-registry.com", // Invalid registry
	}

	ctx := plugin.ReleaseContext{
		Version: "1.0.0",
	}

	resp, err := p.publishPackage(context.Background(), cfg, ctx, true)
	if err != nil {
		t.Fatalf("publishPackage() error = %v", err)
	}

	if resp.Success {
		t.Error("publishPackage() Success = true, want false for invalid registry")
	}
}

func TestNpmPlugin_publishPackage_MissingPackageJSON(t *testing.T) {
	p := &NpmPlugin{}

	cwd, _ := os.Getwd()
	tmpDir, _ := os.MkdirTemp(cwd, "npm-missing-pkg-*")
	defer os.RemoveAll(tmpDir)

	relDir := filepath.Base(tmpDir)

	cfg := &Config{
		PackageDir: relDir,
	}

	ctx := plugin.ReleaseContext{
		Version: "1.0.0",
	}

	resp, err := p.publishPackage(context.Background(), cfg, ctx, true)
	if err != nil {
		t.Fatalf("publishPackage() error = %v", err)
	}

	if resp.Success {
		t.Error("publishPackage() Success = true, want false for missing package.json")
	}
}
