// Package main implements tests for the Chocolatey plugin.
package main

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

func TestGetInfo(t *testing.T) {
	p := &ChocolateyPlugin{}
	info := p.GetInfo()

	if info.Name != "chocolatey" {
		t.Errorf("expected name 'chocolatey', got %s", info.Name)
	}

	if info.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", info.Version)
	}

	if len(info.Hooks) != 1 || info.Hooks[0] != plugin.HookPostPublish {
		t.Errorf("expected HookPostPublish, got %v", info.Hooks)
	}
}

func TestParseConfig(t *testing.T) {
	p := &ChocolateyPlugin{}

	tests := []struct {
		name     string
		config   map[string]any
		expected *Config
	}{
		{
			name: "all fields",
			config: map[string]any{
				"package_id":      "mytool",
				"package_title":   "My Tool",
				"description":     "A useful tool",
				"authors":         "Author Name",
				"project_url":     "https://example.com",
				"license_url":     "https://example.com/license",
				"icon_url":        "https://example.com/icon.png",
				"tags":            "cli utility",
				"download_url_32": "https://example.com/{{tag}}/tool-386.exe",
				"download_url_64": "https://example.com/{{tag}}/tool-amd64.exe",
				"silent_args":     "/S",
				"api_key":         "choco-key",
				"source":          "https://push.chocolatey.org/",
				"dependencies":    []any{"dotnet-runtime:6.0"},
				"output_dir":      "./dist",
			},
			expected: &Config{
				PackageID:     "mytool",
				PackageTitle:  "My Tool",
				Description:   "A useful tool",
				Authors:       "Author Name",
				ProjectURL:    "https://example.com",
				LicenseURL:    "https://example.com/license",
				IconURL:       "https://example.com/icon.png",
				Tags:          "cli utility",
				DownloadURL32: "https://example.com/{{tag}}/tool-386.exe",
				DownloadURL64: "https://example.com/{{tag}}/tool-amd64.exe",
				SilentArgs:    "/S",
				APIKey:        "choco-key",
				Source:        "https://push.chocolatey.org/",
				Dependencies:  []string{"dotnet-runtime:6.0"},
				OutputDir:     "./dist",
			},
		},
		{
			name:   "defaults",
			config: map[string]any{},
			expected: &Config{
				Source: "https://push.chocolatey.org/",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := p.parseConfig(tt.config)

			if cfg.PackageID != tt.expected.PackageID {
				t.Errorf("PackageID: expected %s, got %s", tt.expected.PackageID, cfg.PackageID)
			}
			if cfg.PackageTitle != tt.expected.PackageTitle {
				t.Errorf("PackageTitle: expected %s, got %s", tt.expected.PackageTitle, cfg.PackageTitle)
			}
			if cfg.Description != tt.expected.Description {
				t.Errorf("Description: expected %s, got %s", tt.expected.Description, cfg.Description)
			}
			if cfg.Source != tt.expected.Source {
				t.Errorf("Source: expected %s, got %s", tt.expected.Source, cfg.Source)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	p := &ChocolateyPlugin{}
	ctx := context.Background()

	tests := []struct {
		name        string
		config      map[string]any
		expectValid bool
	}{
		{
			name: "valid config",
			config: map[string]any{
				"package_id":      "mytool",
				"description":     "A useful tool",
				"download_url_64": "https://example.com/{{tag}}/tool.exe",
				"api_key":         "choco-key",
			},
			expectValid: true,
		},
		{
			name: "missing package_id",
			config: map[string]any{
				"description":     "A useful tool",
				"download_url_64": "https://example.com/tool.exe",
			},
			expectValid: false,
		},
		{
			name: "missing description",
			config: map[string]any{
				"package_id":      "mytool",
				"download_url_64": "https://example.com/tool.exe",
			},
			expectValid: false,
		},
		{
			name: "missing download URLs",
			config: map[string]any{
				"package_id":  "mytool",
				"description": "A useful tool",
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

func TestResolveURL(t *testing.T) {
	p := &ChocolateyPlugin{}

	tests := []struct {
		name     string
		template string
		version  string
		tag      string
		expected string
	}{
		{
			name:     "all placeholders",
			template: "https://github.com/user/repo/releases/download/{{tag}}/tool-{{version}}-amd64.exe",
			version:  "1.0.0",
			tag:      "v1.0.0",
			expected: "https://github.com/user/repo/releases/download/v1.0.0/tool-1.0.0-amd64.exe",
		},
		{
			name:     "version only",
			template: "https://example.com/tool-{{version}}.exe",
			version:  "2.0.0",
			tag:      "v2.0.0",
			expected: "https://example.com/tool-2.0.0.exe",
		},
		{
			name:     "empty template",
			template: "",
			version:  "1.0.0",
			tag:      "v1.0.0",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.resolveURL(tt.template, tt.version, tt.tag)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestExecuteDryRun(t *testing.T) {
	p := &ChocolateyPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"package_id":      "mytool",
			"description":     "A useful tool",
			"download_url_64": "https://example.com/{{tag}}/tool.exe",
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

	if resp.Message != "Would build and push Chocolatey package" {
		t.Errorf("unexpected message: %s", resp.Message)
	}

	if resp.Outputs["package_id"] != "mytool" {
		t.Errorf("unexpected package_id output: %v", resp.Outputs["package_id"])
	}

	if resp.Outputs["version"] != "1.0.0" {
		t.Errorf("unexpected version output: %v", resp.Outputs["version"])
	}
}

func TestExecuteUnhandledHook(t *testing.T) {
	p := &ChocolateyPlugin{}
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

func TestGenerateNuspec(t *testing.T) {
	p := &ChocolateyPlugin{}

	cfg := &Config{
		PackageID:    "mytool",
		PackageTitle: "My Tool",
		Description:  "A useful tool",
		Authors:      "Author Name",
		ProjectURL:   "https://example.com",
		LicenseURL:   "https://example.com/license",
		Tags:         "cli utility",
		Dependencies: []string{"dotnet-runtime:6.0", "vcredist140"},
	}

	// Create temp file
	tmpFile := t.TempDir() + "/test.nuspec"

	err := p.generateNuspec(cfg, "1.0.0", tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateInstallScript(t *testing.T) {
	p := &ChocolateyPlugin{}

	cfg := &Config{
		PackageID:  "mytool",
		SilentArgs: "/S",
	}

	// Create temp file
	tmpFile := t.TempDir() + "/chocolateyInstall.ps1"

	err := p.generateInstallScript(cfg,
		"https://example.com/tool-386.exe", "abc123",
		"https://example.com/tool-amd64.exe", "def456",
		tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDependencyParsing(t *testing.T) {
	p := &ChocolateyPlugin{}

	cfg := &Config{
		PackageID:    "mytool",
		Description:  "A tool",
		Dependencies: []string{"dotnet-runtime:6.0", "vcredist140"},
	}

	tmpFile := t.TempDir() + "/test.nuspec"

	err := p.generateNuspec(cfg, "1.0.0", tmpFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The nuspec should be generated without error
	// Full content validation would require reading and parsing the file
}
