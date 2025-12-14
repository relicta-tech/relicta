// Package main implements tests for the Homebrew plugin.
package main

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

func TestGetInfo(t *testing.T) {
	p := &HomebrewPlugin{}
	info := p.GetInfo()

	if info.Name != "homebrew" {
		t.Errorf("expected name 'homebrew', got %s", info.Name)
	}

	if info.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", info.Version)
	}

	if len(info.Hooks) != 1 || info.Hooks[0] != plugin.HookPostPublish {
		t.Errorf("expected HookPostPublish, got %v", info.Hooks)
	}
}

func TestParseConfig(t *testing.T) {
	p := &HomebrewPlugin{}

	tests := []struct {
		name     string
		config   map[string]any
		expected *Config
	}{
		{
			name: "all fields",
			config: map[string]any{
				"tap_repository":        "user/homebrew-tap",
				"formula_name":          "my-tool",
				"formula_path":          "Formula/my-tool.rb",
				"description":           "A useful tool",
				"homepage":              "https://example.com",
				"license":               "Apache-2.0",
				"download_url_template": "https://github.com/user/repo/releases/download/{{tag}}/tool-{{os}}-{{arch}}.tar.gz",
				"github_token":          "ghp_token",
				"commit_message":        "Update {{version}}",
				"create_pr":             true,
				"pr_branch":             "release-{{version}}",
				"dependencies":          []any{"go", "git"},
				"install_script":        `bin.install "mytool"`,
				"test_script":           `system "#{bin}/mytool", "-v"`,
			},
			expected: &Config{
				TapRepository:       "user/homebrew-tap",
				FormulaName:         "my-tool",
				FormulaPath:         "Formula/my-tool.rb",
				Description:         "A useful tool",
				Homepage:            "https://example.com",
				License:             "Apache-2.0",
				DownloadURLTemplate: "https://github.com/user/repo/releases/download/{{tag}}/tool-{{os}}-{{arch}}.tar.gz",
				GitHubToken:         "ghp_token",
				CommitMessage:       "Update {{version}}",
				CreatePR:            true,
				PRBranch:            "release-{{version}}",
				Dependencies:        []string{"go", "git"},
				InstallScript:       `bin.install "mytool"`,
				TestScript:          `system "#{bin}/mytool", "-v"`,
			},
		},
		{
			name:   "defaults",
			config: map[string]any{},
			expected: &Config{
				License: "MIT",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := p.parseConfig(tt.config)

			if cfg.TapRepository != tt.expected.TapRepository {
				t.Errorf("TapRepository: expected %s, got %s", tt.expected.TapRepository, cfg.TapRepository)
			}
			if cfg.FormulaName != tt.expected.FormulaName {
				t.Errorf("FormulaName: expected %s, got %s", tt.expected.FormulaName, cfg.FormulaName)
			}
			if cfg.License != tt.expected.License {
				t.Errorf("License: expected %s, got %s", tt.expected.License, cfg.License)
			}
			if cfg.CreatePR != tt.expected.CreatePR {
				t.Errorf("CreatePR: expected %v, got %v", tt.expected.CreatePR, cfg.CreatePR)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	p := &HomebrewPlugin{}
	ctx := context.Background()

	tests := []struct {
		name        string
		config      map[string]any
		expectValid bool
	}{
		{
			name: "valid config",
			config: map[string]any{
				"tap_repository":        "user/homebrew-tap",
				"download_url_template": "https://example.com/{{tag}}/file.tar.gz",
				"github_token":          "ghp_token",
			},
			expectValid: true,
		},
		{
			name: "missing tap_repository",
			config: map[string]any{
				"download_url_template": "https://example.com/{{tag}}/file.tar.gz",
			},
			expectValid: false,
		},
		{
			name: "missing download_url_template",
			config: map[string]any{
				"tap_repository": "user/homebrew-tap",
			},
			expectValid: false,
		},
		{
			name: "invalid tap_repository format",
			config: map[string]any{
				"tap_repository":        "invalid-format",
				"download_url_template": "https://example.com/file.tar.gz",
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
	p := &HomebrewPlugin{}

	tests := []struct {
		name     string
		template string
		version  string
		tag      string
		os       string
		arch     string
		expected string
	}{
		{
			name:     "all placeholders",
			template: "https://github.com/user/repo/releases/download/{{tag}}/tool-{{version}}-{{os}}-{{arch}}.tar.gz",
			version:  "1.0.0",
			tag:      "v1.0.0",
			os:       "darwin",
			arch:     "amd64",
			expected: "https://github.com/user/repo/releases/download/v1.0.0/tool-1.0.0-darwin-amd64.tar.gz",
		},
		{
			name:     "version only",
			template: "https://example.com/tool-{{version}}.tar.gz",
			version:  "2.0.0",
			tag:      "v2.0.0",
			os:       "linux",
			arch:     "arm64",
			expected: "https://example.com/tool-2.0.0.tar.gz",
		},
		{
			name:     "no placeholders",
			template: "https://example.com/tool.tar.gz",
			version:  "1.0.0",
			tag:      "v1.0.0",
			os:       "darwin",
			arch:     "amd64",
			expected: "https://example.com/tool.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.resolveURL(tt.template, tt.version, tt.tag, tt.os, tt.arch)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestToClassName(t *testing.T) {
	p := &HomebrewPlugin{}

	tests := []struct {
		input    string
		expected string
	}{
		{"my-tool", "MyTool"},
		{"relicta", "Relicta"},
		{"simple", "Simple"},
		{"a-b-c", "ABC"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := p.toClassName(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGenerateFormula(t *testing.T) {
	p := &HomebrewPlugin{}

	cfg := &Config{
		Description:  "A test tool",
		Homepage:     "https://example.com",
		License:      "MIT",
		Dependencies: []string{"go"},
	}

	content, err := p.generateFormula(cfg, "test-tool", "1.0.0",
		"https://example.com/test-tool-darwin-amd64.tar.gz", "abc123",
		"https://example.com/test-tool-darwin-arm64.tar.gz", "def456")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify formula contains expected content
	if !containsString(content, "class TestTool < Formula") {
		t.Error("expected class declaration")
	}
	if !containsString(content, `desc "A test tool"`) {
		t.Error("expected description")
	}
	if !containsString(content, `homepage "https://example.com"`) {
		t.Error("expected homepage")
	}
	if !containsString(content, `version "1.0.0"`) {
		t.Error("expected version")
	}
	if !containsString(content, `license "MIT"`) {
		t.Error("expected license")
	}
	if !containsString(content, `depends_on "go"`) {
		t.Error("expected dependency")
	}
}

func TestExecuteDryRun(t *testing.T) {
	p := &HomebrewPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"tap_repository":        "user/homebrew-tap",
			"download_url_template": "https://example.com/{{tag}}/tool-{{os}}-{{arch}}.tar.gz",
		},
		Context: plugin.ReleaseContext{
			Version:        "1.0.0",
			TagName:        "v1.0.0",
			RepositoryName: "my-tool",
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

	if resp.Message != "Would publish Homebrew formula" {
		t.Errorf("unexpected message: %s", resp.Message)
	}

	if resp.Outputs["tap_repository"] != "user/homebrew-tap" {
		t.Errorf("unexpected tap_repository output: %v", resp.Outputs["tap_repository"])
	}
}

func TestExecuteUnhandledHook(t *testing.T) {
	p := &HomebrewPlugin{}
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

func TestFormulaWithCustomScripts(t *testing.T) {
	p := &HomebrewPlugin{}

	cfg := &Config{
		Description:   "A test tool",
		Homepage:      "https://example.com",
		License:       "MIT",
		InstallScript: `bin.install "custom-binary"`,
		TestScript:    `system "#{bin}/custom-binary", "--help"`,
	}

	content, err := p.generateFormula(cfg, "test-tool", "1.0.0",
		"https://example.com/x86.tar.gz", "sha1",
		"https://example.com/arm.tar.gz", "sha2")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsString(content, `bin.install "custom-binary"`) {
		t.Error("expected custom install script")
	}
	if !containsString(content, `system "#{bin}/custom-binary", "--help"`) {
		t.Error("expected custom test script")
	}
}

func containsString(s, substr string) bool {
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
