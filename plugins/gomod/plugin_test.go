// Package main implements tests for the Go Modules plugin.
package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

func TestGetInfo(t *testing.T) {
	p := &GoModPlugin{}
	info := p.GetInfo()

	if info.Name != "gomod" {
		t.Errorf("expected name 'gomod', got %s", info.Name)
	}

	if info.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", info.Version)
	}

	if len(info.Hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(info.Hooks))
	}
}

func TestParseConfig(t *testing.T) {
	p := &GoModPlugin{}

	tests := []struct {
		name     string
		config   map[string]any
		expected *Config
	}{
		{
			name: "all fields",
			config: map[string]any{
				"module_dir":    "./mymodule",
				"private":       true,
				"proxy_url":     "https://custom.proxy.org",
				"sumdb":         "https://sum.golang.org",
				"noproxy":       "github.com/private/*",
				"validate_tag":  false,
				"trigger_proxy": false,
			},
			expected: &Config{
				ModuleDir:    "./mymodule",
				Private:      true,
				ProxyURL:     "https://custom.proxy.org",
				SumDB:        "https://sum.golang.org",
				NoProxy:      "github.com/private/*",
				ValidateTag:  false,
				TriggerProxy: false,
			},
		},
		{
			name:   "defaults",
			config: map[string]any{},
			expected: &Config{
				ProxyURL:     "https://proxy.golang.org",
				ValidateTag:  true,
				TriggerProxy: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := p.parseConfig(tt.config)

			if cfg.ProxyURL != tt.expected.ProxyURL {
				t.Errorf("ProxyURL: expected %s, got %s", tt.expected.ProxyURL, cfg.ProxyURL)
			}
			if cfg.ValidateTag != tt.expected.ValidateTag {
				t.Errorf("ValidateTag: expected %v, got %v", tt.expected.ValidateTag, cfg.ValidateTag)
			}
			if cfg.TriggerProxy != tt.expected.TriggerProxy {
				t.Errorf("TriggerProxy: expected %v, got %v", tt.expected.TriggerProxy, cfg.TriggerProxy)
			}
			if cfg.Private != tt.expected.Private {
				t.Errorf("Private: expected %v, got %v", tt.expected.Private, cfg.Private)
			}
		})
	}
}

func TestValidateModuleDir(t *testing.T) {
	p := &GoModPlugin{}

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
			_, err := p.validateModuleDir(tt.dir)
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetModulePath(t *testing.T) {
	p := &GoModPlugin{}

	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")

	// Write go.mod
	if err := os.WriteFile(goModPath, []byte(`module github.com/example/mymodule

go 1.21

require (
    github.com/stretchr/testify v1.8.4
)
`), 0644); err != nil {
		t.Fatal(err)
	}

	modulePath, err := p.getModulePath(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if modulePath != "github.com/example/mymodule" {
		t.Errorf("expected github.com/example/mymodule, got %s", modulePath)
	}
}

func TestValidateModuleDryRun(t *testing.T) {
	p := &GoModPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")

	// Write go.mod
	if err := os.WriteFile(goModPath, []byte(`module github.com/example/testmod

go 1.21
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		ModuleDir:   tmpDir,
		ValidateTag: true,
	}

	releaseCtx := plugin.ReleaseContext{
		Version: "1.0.0",
		TagName: "v1.0.0",
	}

	resp, err := p.validateModule(ctx, cfg, releaseCtx, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	if resp.Outputs["module"] != "github.com/example/testmod" {
		t.Errorf("unexpected module output: %v", resp.Outputs["module"])
	}
}

func TestValidateModuleInvalidTag(t *testing.T) {
	p := &GoModPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")

	// Write go.mod
	if err := os.WriteFile(goModPath, []byte(`module github.com/example/testmod

go 1.21
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		ModuleDir:   tmpDir,
		ValidateTag: true,
	}

	// Tag without v prefix
	releaseCtx := plugin.ReleaseContext{
		Version: "1.0.0",
		TagName: "1.0.0", // Invalid - no v prefix
	}

	resp, err := p.validateModule(ctx, cfg, releaseCtx, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Success {
		t.Error("expected failure for invalid tag format")
	}
}

func TestExecuteDryRun(t *testing.T) {
	p := &GoModPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")

	// Write go.mod
	if err := os.WriteFile(goModPath, []byte(`module github.com/example/testmod

go 1.21
`), 0644); err != nil {
		t.Fatal(err)
	}

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"module_dir":    tmpDir,
			"trigger_proxy": true,
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

	if resp.Message != "Would trigger Go proxy to fetch module" {
		t.Errorf("unexpected message: %s", resp.Message)
	}

	if resp.Outputs["version"] != "v1.0.0" {
		t.Errorf("unexpected version output: %v", resp.Outputs["version"])
	}

	if resp.Outputs["module"] != "github.com/example/testmod" {
		t.Errorf("unexpected module output: %v", resp.Outputs["module"])
	}
}

func TestExecutePrivateModule(t *testing.T) {
	p := &GoModPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")

	// Write go.mod
	if err := os.WriteFile(goModPath, []byte(`module github.com/private/testmod

go 1.21
`), 0644); err != nil {
		t.Fatal(err)
	}

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"module_dir": tmpDir,
			"private":    true,
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

	if resp.Outputs["private"] != true {
		t.Errorf("expected private=true, got %v", resp.Outputs["private"])
	}
}

func TestExecuteUnhandledHook(t *testing.T) {
	p := &GoModPlugin{}
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
	p := &GoModPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")

	// Write go.mod
	if err := os.WriteFile(goModPath, []byte(`module github.com/example/testmod

go 1.21
`), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		config      map[string]any
		expectValid bool
	}{
		{
			name: "valid config with module dir",
			config: map[string]any{
				"module_dir": tmpDir,
			},
			expectValid: true,
		},
		{
			name:        "empty config",
			config:      map[string]any{},
			expectValid: true,
		},
		{
			name: "invalid module dir",
			config: map[string]any{
				"module_dir": "/nonexistent/path",
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
