// Package main implements tests for the GitLab plugin.
package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

func TestGetInfo(t *testing.T) {
	p := &GitLabPlugin{}
	info := p.GetInfo()

	if info.Name != "gitlab" {
		t.Errorf("Name = %v, want gitlab", info.Name)
	}
	if info.Version != "1.0.0" {
		t.Errorf("Version = %v, want 1.0.0", info.Version)
	}
	if info.Description == "" {
		t.Error("Description should not be empty")
	}
	if len(info.Hooks) != 3 {
		t.Errorf("Expected 3 hooks, got %d", len(info.Hooks))
	}

	// Check hooks
	expectedHooks := map[plugin.Hook]bool{
		plugin.HookPostPublish: true,
		plugin.HookOnSuccess:   true,
		plugin.HookOnError:     true,
	}
	for _, hook := range info.Hooks {
		if !expectedHooks[hook] {
			t.Errorf("Unexpected hook: %v", hook)
		}
	}
}

func TestParseConfig(t *testing.T) {
	p := &GitLabPlugin{}

	tests := []struct {
		name     string
		raw      map[string]any
		expected *Config
	}{
		{
			name:     "empty config",
			raw:      map[string]any{},
			expected: &Config{},
		},
		{
			name: "full config",
			raw: map[string]any{
				"base_url":    "https://gitlab.example.com",
				"project_id":  "group/project",
				"token":       "test-token",
				"name":        "v1.0.0 Release",
				"description": "Release notes here",
				"ref":         "main",
				"released_at": "2024-01-01T00:00:00Z",
				"milestones":  []any{"v1.0.0", "Q1-2024"},
				"assets":      []any{"dist/app.tar.gz", "dist/app.zip"},
				"asset_links": []any{
					map[string]any{
						"name":      "Docker Image",
						"url":       "https://registry.example.com/app:v1.0.0",
						"link_type": "image",
					},
					map[string]any{
						"name":     "Documentation",
						"url":      "https://docs.example.com",
						"filepath": "/docs/v1.0.0/index.html",
					},
				},
			},
			expected: &Config{
				BaseURL:     "https://gitlab.example.com",
				ProjectID:   "group/project",
				Token:       "test-token",
				Name:        "v1.0.0 Release",
				Description: "Release notes here",
				Ref:         "main",
				ReleasedAt:  "2024-01-01T00:00:00Z",
				Milestones:  []string{"v1.0.0", "Q1-2024"},
				Assets:      []string{"dist/app.tar.gz", "dist/app.zip"},
				AssetLinks: []AssetLink{
					{
						Name:     "Docker Image",
						URL:      "https://registry.example.com/app:v1.0.0",
						LinkType: "image",
					},
					{
						Name:     "Documentation",
						URL:      "https://docs.example.com",
						FilePath: "/docs/v1.0.0/index.html",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := p.parseConfig(tt.raw)

			if cfg.BaseURL != tt.expected.BaseURL {
				t.Errorf("BaseURL = %v, want %v", cfg.BaseURL, tt.expected.BaseURL)
			}
			if cfg.ProjectID != tt.expected.ProjectID {
				t.Errorf("ProjectID = %v, want %v", cfg.ProjectID, tt.expected.ProjectID)
			}
			if cfg.Token != tt.expected.Token {
				t.Errorf("Token = %v, want %v", cfg.Token, tt.expected.Token)
			}
			if cfg.Name != tt.expected.Name {
				t.Errorf("Name = %v, want %v", cfg.Name, tt.expected.Name)
			}
			if cfg.Description != tt.expected.Description {
				t.Errorf("Description = %v, want %v", cfg.Description, tt.expected.Description)
			}
			if cfg.Ref != tt.expected.Ref {
				t.Errorf("Ref = %v, want %v", cfg.Ref, tt.expected.Ref)
			}
			if cfg.ReleasedAt != tt.expected.ReleasedAt {
				t.Errorf("ReleasedAt = %v, want %v", cfg.ReleasedAt, tt.expected.ReleasedAt)
			}
			if len(cfg.Milestones) != len(tt.expected.Milestones) {
				t.Errorf("Milestones length = %v, want %v", len(cfg.Milestones), len(tt.expected.Milestones))
			}
			if len(cfg.Assets) != len(tt.expected.Assets) {
				t.Errorf("Assets length = %v, want %v", len(cfg.Assets), len(tt.expected.Assets))
			}
			if len(cfg.AssetLinks) != len(tt.expected.AssetLinks) {
				t.Errorf("AssetLinks length = %v, want %v", len(cfg.AssetLinks), len(tt.expected.AssetLinks))
			}
		})
	}
}

func TestValidate(t *testing.T) {
	p := &GitLabPlugin{}
	ctx := context.Background()

	// Clear environment variables for testing
	os.Unsetenv("GITLAB_TOKEN")
	os.Unsetenv("GL_TOKEN")

	tests := []struct {
		name        string
		config      map[string]any
		envToken    string
		expectValid bool
	}{
		{
			name:        "missing token",
			config:      map[string]any{},
			expectValid: false,
		},
		{
			name: "token in config",
			config: map[string]any{
				"token": "test-token",
			},
			expectValid: true,
		},
		{
			name:        "token in env",
			config:      map[string]any{},
			envToken:    "env-token",
			expectValid: true,
		},
		{
			name: "invalid base_url",
			config: map[string]any{
				"token":    "test-token",
				"base_url": "not-a-url",
			},
			expectValid: false,
		},
		{
			name: "valid base_url",
			config: map[string]any{
				"token":    "test-token",
				"base_url": "https://gitlab.example.com",
			},
			expectValid: true,
		},
		{
			name: "invalid asset type",
			config: map[string]any{
				"token":  "test-token",
				"assets": []any{123}, // should be string
			},
			expectValid: false,
		},
		{
			name: "valid assets",
			config: map[string]any{
				"token":  "test-token",
				"assets": []any{"file1.tar.gz", "file2.zip"},
			},
			expectValid: true,
		},
		{
			name: "asset_link missing name",
			config: map[string]any{
				"token": "test-token",
				"asset_links": []any{
					map[string]any{
						"url": "https://example.com",
					},
				},
			},
			expectValid: false,
		},
		{
			name: "asset_link missing url",
			config: map[string]any{
				"token": "test-token",
				"asset_links": []any{
					map[string]any{
						"name": "Test Link",
					},
				},
			},
			expectValid: false,
		},
		{
			name: "asset_link invalid link_type",
			config: map[string]any{
				"token": "test-token",
				"asset_links": []any{
					map[string]any{
						"name":      "Test Link",
						"url":       "https://example.com",
						"link_type": "invalid",
					},
				},
			},
			expectValid: false,
		},
		{
			name: "valid asset_links",
			config: map[string]any{
				"token": "test-token",
				"asset_links": []any{
					map[string]any{
						"name":      "Docker Image",
						"url":       "https://registry.example.com/app:v1",
						"link_type": "image",
					},
					map[string]any{
						"name":      "Package",
						"url":       "https://pkg.example.com/app",
						"link_type": "package",
					},
				},
			},
			expectValid: true,
		},
		{
			name: "invalid milestone type",
			config: map[string]any{
				"token":      "test-token",
				"milestones": []any{123}, // should be string
			},
			expectValid: false,
		},
		{
			name: "valid milestones",
			config: map[string]any{
				"token":      "test-token",
				"milestones": []any{"v1.0.0", "Q1-2024"},
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set/clear env token
			if tt.envToken != "" {
				os.Setenv("GITLAB_TOKEN", tt.envToken)
				defer os.Unsetenv("GITLAB_TOKEN")
			}

			resp, err := p.Validate(ctx, tt.config)
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}

			if resp.Valid != tt.expectValid {
				t.Errorf("Valid = %v, want %v, errors: %v", resp.Valid, tt.expectValid, resp.Errors)
			}
		})
	}
}

func TestExecute_DryRun(t *testing.T) {
	p := &GitLabPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook:   plugin.HookPostPublish,
		DryRun: true,
		Config: map[string]any{
			"token":      "test-token",
			"project_id": "group/project",
		},
		Context: plugin.ReleaseContext{
			Version:         "1.0.0",
			TagName:         "v1.0.0",
			RepositoryOwner: "group",
			RepositoryName:  "project",
			ReleaseNotes:    "Release notes here",
		},
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !resp.Success {
		t.Errorf("Success = false, want true")
	}
	if resp.Message == "" {
		t.Error("Message should not be empty")
	}
	if resp.Outputs["tag_name"] != "v1.0.0" {
		t.Errorf("tag_name = %v, want v1.0.0", resp.Outputs["tag_name"])
	}
}

func TestExecute_OnSuccess(t *testing.T) {
	p := &GitLabPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook:   plugin.HookOnSuccess,
		Config: map[string]any{},
		Context: plugin.ReleaseContext{
			Version: "1.0.0",
		},
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !resp.Success {
		t.Errorf("Success = false, want true")
	}
	if resp.Message != "Release successful" {
		t.Errorf("Message = %v, want 'Release successful'", resp.Message)
	}
}

func TestExecute_OnError(t *testing.T) {
	p := &GitLabPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook:   plugin.HookOnError,
		Config: map[string]any{},
		Context: plugin.ReleaseContext{
			Version: "1.0.0",
		},
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !resp.Success {
		t.Errorf("Success = false, want true")
	}
	if resp.Message != "Release failed notification acknowledged" {
		t.Errorf("Message = %v, want 'Release failed notification acknowledged'", resp.Message)
	}
}

func TestExecute_UnhandledHook(t *testing.T) {
	p := &GitLabPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook:   plugin.HookPrePlan,
		Config: map[string]any{},
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !resp.Success {
		t.Errorf("Success = false, want true")
	}
}

func TestExecute_MissingProjectID(t *testing.T) {
	p := &GitLabPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook:   plugin.HookPostPublish,
		DryRun: false,
		Config: map[string]any{
			"token": "test-token",
			// project_id is missing
		},
		Context: plugin.ReleaseContext{
			Version: "1.0.0",
			TagName: "v1.0.0",
			// No repository owner/name either
		},
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if resp.Success {
		t.Error("Should fail when project_id is missing")
	}
	if resp.Error == "" {
		t.Error("Error message should not be empty")
	}
}

func TestExecute_ProjectIDFromContext(t *testing.T) {
	p := &GitLabPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook:   plugin.HookPostPublish,
		DryRun: true,
		Config: map[string]any{
			"token": "test-token",
			// project_id NOT set in config
		},
		Context: plugin.ReleaseContext{
			Version:         "1.0.0",
			TagName:         "v1.0.0",
			RepositoryOwner: "mygroup",
			RepositoryName:  "myproject",
		},
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !resp.Success {
		t.Errorf("Success = false, want true, error: %s", resp.Error)
	}
	if resp.Outputs["project_id"] != "mygroup/myproject" {
		t.Errorf("project_id = %v, want mygroup/myproject", resp.Outputs["project_id"])
	}
}

func TestGetClient_MissingToken(t *testing.T) {
	p := &GitLabPlugin{}

	// Clear env vars
	os.Unsetenv("GITLAB_TOKEN")
	os.Unsetenv("GL_TOKEN")

	cfg := &Config{}

	_, err := p.getClient(cfg)
	if err == nil {
		t.Error("Expected error when token is missing")
	}
}

func TestGetClient_TokenFromConfig(t *testing.T) {
	p := &GitLabPlugin{}

	cfg := &Config{
		Token: "test-token",
	}

	client, err := p.getClient(cfg)
	if err != nil {
		t.Errorf("getClient() error = %v", err)
	}
	if client == nil {
		t.Error("client should not be nil")
	}
}

func TestGetClient_TokenFromEnv(t *testing.T) {
	p := &GitLabPlugin{}

	os.Setenv("GITLAB_TOKEN", "env-token")
	defer os.Unsetenv("GITLAB_TOKEN")

	cfg := &Config{}

	client, err := p.getClient(cfg)
	if err != nil {
		t.Errorf("getClient() error = %v", err)
	}
	if client == nil {
		t.Error("client should not be nil")
	}
}

func TestGetClient_CustomBaseURL(t *testing.T) {
	p := &GitLabPlugin{}

	cfg := &Config{
		Token:   "test-token",
		BaseURL: "https://gitlab.example.com",
	}

	client, err := p.getClient(cfg)
	if err != nil {
		t.Errorf("getClient() error = %v", err)
	}
	if client == nil {
		t.Error("client should not be nil")
	}
}

func TestUploadAsset_PathTraversal(t *testing.T) {
	p := &GitLabPlugin{}
	ctx := context.Background()

	cfg := &Config{
		Token: "test-token",
	}

	client, err := p.getClient(cfg)
	if err != nil {
		t.Fatalf("getClient() error = %v", err)
	}

	dangerousPaths := []string{
		"../../../etc/passwd",
		"dist/../../../etc/shadow",
	}

	for _, path := range dangerousPaths {
		_, err := p.uploadAsset(ctx, client, "test/project", "v1.0.0", path)
		if err == nil {
			t.Errorf("uploadAsset should reject path traversal: %s", path)
		}
	}
}

func TestUploadAsset_NonExistentFile(t *testing.T) {
	p := &GitLabPlugin{}
	ctx := context.Background()

	cfg := &Config{
		Token: "test-token",
	}

	client, err := p.getClient(cfg)
	if err != nil {
		t.Fatalf("getClient() error = %v", err)
	}

	_, err = p.uploadAsset(ctx, client, "test/project", "v1.0.0", "/nonexistent/file.tar.gz")
	if err == nil {
		t.Error("uploadAsset should fail for non-existent file")
	}
}

func TestUploadAsset_Directory(t *testing.T) {
	p := &GitLabPlugin{}
	ctx := context.Background()

	cfg := &Config{
		Token: "test-token",
	}

	client, err := p.getClient(cfg)
	if err != nil {
		t.Fatalf("getClient() error = %v", err)
	}

	// Use temp directory
	tmpDir := t.TempDir()

	_, err = p.uploadAsset(ctx, client, "test/project", "v1.0.0", tmpDir)
	if err == nil {
		t.Error("uploadAsset should reject directories")
	}
}

func TestValidateAssetPath(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Save and restore working directory
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origWd)

	// Change to temp directory for testing
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	tests := []struct {
		name        string
		assetPath   string
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty path",
			assetPath:   "",
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:        "path traversal with ..",
			assetPath:   "../../../etc/passwd",
			wantErr:     true,
			errContains: "path traversal",
		},
		{
			name:        "path traversal mid-path",
			assetPath:   "foo/../../bar",
			wantErr:     true,
			errContains: "path traversal",
		},
		{
			name:        "valid relative path",
			assetPath:   "test.txt",
			wantErr:     false,
			errContains: "",
		},
		{
			name:        "nonexistent file",
			assetPath:   "nonexistent.txt",
			wantErr:     true,
			errContains: "not accessible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validateAssetPath(tt.assetPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validateAssetPath(%q) = %q, want error containing %q", tt.assetPath, result, tt.errContains)
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("validateAssetPath(%q) error = %q, want error containing %q", tt.assetPath, err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("validateAssetPath(%q) unexpected error: %v", tt.assetPath, err)
				}
				if result == "" {
					t.Errorf("validateAssetPath(%q) returned empty path", tt.assetPath)
				}
			}
		})
	}
}

func TestValidateAssetPath_SymlinkAttack(t *testing.T) {
	// Create temp directories
	tmpDir := t.TempDir()
	outsideDir := t.TempDir()

	// Create a file outside the working directory
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0644); err != nil {
		t.Fatalf("failed to create outside file: %v", err)
	}

	// Create a symlink to the outside file
	sneakyLink := filepath.Join(tmpDir, "sneaky-link")
	if err := os.Symlink(outsideFile, sneakyLink); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	// Save and restore working directory
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	// Test that symlink to outside file is rejected
	_, err = validateAssetPath("sneaky-link")
	if err == nil {
		t.Error("validateAssetPath should reject symlink to file outside working directory")
	}
}

func TestValidateAssetPath_DirectoryTraversal(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()

	// Save and restore working directory
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origWd)

	// Change to temp directory for testing
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	// Test various path traversal attempts
	traversalPaths := []string{
		"../secret.txt",
		"../../secret.txt",
		"../../../etc/passwd",
		"foo/../../../bar",
		"./foo/../../bar",
	}

	for _, path := range traversalPaths {
		t.Run(path, func(t *testing.T) {
			_, err := validateAssetPath(path)
			if err == nil {
				t.Errorf("validateAssetPath(%q) should return error for path traversal", path)
			}
		})
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAssetLinkTypes(t *testing.T) {
	p := &GitLabPlugin{}

	raw := map[string]any{
		"asset_links": []any{
			map[string]any{
				"name":      "Other",
				"url":       "https://example.com/other",
				"link_type": "other",
			},
			map[string]any{
				"name":      "Runbook",
				"url":       "https://example.com/runbook",
				"link_type": "runbook",
			},
			map[string]any{
				"name":      "Image",
				"url":       "https://example.com/image",
				"link_type": "image",
			},
			map[string]any{
				"name":      "Package",
				"url":       "https://example.com/package",
				"link_type": "package",
			},
		},
	}

	cfg := p.parseConfig(raw)

	if len(cfg.AssetLinks) != 4 {
		t.Errorf("AssetLinks length = %d, want 4", len(cfg.AssetLinks))
	}

	expectedTypes := []string{"other", "runbook", "image", "package"}
	for i, link := range cfg.AssetLinks {
		if link.LinkType != expectedTypes[i] {
			t.Errorf("AssetLinks[%d].LinkType = %v, want %v", i, link.LinkType, expectedTypes[i])
		}
	}
}
