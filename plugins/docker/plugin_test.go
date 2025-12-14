// Package main implements tests for the Docker plugin.
package main

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

func TestGetInfo(t *testing.T) {
	p := &DockerPlugin{}
	info := p.GetInfo()

	if info.Name != "docker" {
		t.Errorf("expected name 'docker', got %s", info.Name)
	}

	if info.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", info.Version)
	}

	if len(info.Hooks) != 1 || info.Hooks[0] != plugin.HookPostPublish {
		t.Errorf("expected HookPostPublish, got %v", info.Hooks)
	}
}

func TestParseConfig(t *testing.T) {
	p := &DockerPlugin{}

	tests := []struct {
		name     string
		config   map[string]any
		expected *Config
	}{
		{
			name: "all fields",
			config: map[string]any{
				"registry":   "ghcr.io",
				"image":      "user/app",
				"tags":       []any{"{{version}}", "latest"},
				"dockerfile": "Dockerfile.prod",
				"context":    "./app",
				"build_args": map[string]any{"ENV": "prod"},
				"platforms":  []any{"linux/amd64", "linux/arm64"},
				"username":   "user",
				"password":   "pass",
				"push":       true,
				"labels":     map[string]any{"maintainer": "user"},
				"cache_from": []any{"user/app:cache"},
				"no_cache":   false,
				"target":     "production",
			},
			expected: &Config{
				Registry:   "ghcr.io",
				Image:      "user/app",
				Tags:       []string{"{{version}}", "latest"},
				Dockerfile: "Dockerfile.prod",
				Context:    "./app",
				BuildArgs:  map[string]string{"ENV": "prod"},
				Platforms:  []string{"linux/amd64", "linux/arm64"},
				Username:   "user",
				Password:   "pass",
				Push:       true,
				Labels:     map[string]string{"maintainer": "user"},
				CacheFrom:  []string{"user/app:cache"},
				NoCache:    false,
				Target:     "production",
			},
		},
		{
			name:   "defaults",
			config: map[string]any{},
			expected: &Config{
				Registry:   "docker.io",
				Dockerfile: "Dockerfile",
				Context:    ".",
				Push:       true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := p.parseConfig(tt.config)

			if cfg.Registry != tt.expected.Registry {
				t.Errorf("Registry: expected %s, got %s", tt.expected.Registry, cfg.Registry)
			}
			if cfg.Image != tt.expected.Image {
				t.Errorf("Image: expected %s, got %s", tt.expected.Image, cfg.Image)
			}
			if cfg.Dockerfile != tt.expected.Dockerfile {
				t.Errorf("Dockerfile: expected %s, got %s", tt.expected.Dockerfile, cfg.Dockerfile)
			}
			if cfg.Context != tt.expected.Context {
				t.Errorf("Context: expected %s, got %s", tt.expected.Context, cfg.Context)
			}
			if cfg.Push != tt.expected.Push {
				t.Errorf("Push: expected %v, got %v", tt.expected.Push, cfg.Push)
			}
			if cfg.Target != tt.expected.Target {
				t.Errorf("Target: expected %s, got %s", tt.expected.Target, cfg.Target)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	p := &DockerPlugin{}
	ctx := context.Background()

	tests := []struct {
		name        string
		config      map[string]any
		expectValid bool
	}{
		{
			name: "valid config",
			config: map[string]any{
				"image":    "user/app",
				"username": "user",
				"password": "pass",
			},
			expectValid: true,
		},
		{
			name:        "missing image",
			config:      map[string]any{},
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
	p := &DockerPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"image": "user/app",
			"tags":  []any{"{{version}}", "latest", "v{{major}}"},
		},
		Context: plugin.ReleaseContext{
			Version:        "1.2.3",
			TagName:        "v1.2.3",
			RepositoryName: "app",
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

	if resp.Message != "Would build and push Docker image" {
		t.Errorf("unexpected message: %s", resp.Message)
	}

	tags := resp.Outputs["tags"].([]string)
	expectedTags := []string{"1.2.3", "latest", "v1"}
	if len(tags) != len(expectedTags) {
		t.Errorf("expected %d tags, got %d", len(expectedTags), len(tags))
	}
	for i, tag := range expectedTags {
		if tags[i] != tag {
			t.Errorf("tag[%d]: expected %s, got %s", i, tag, tags[i])
		}
	}
}

func TestExecuteUnhandledHook(t *testing.T) {
	p := &DockerPlugin{}
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

func TestTagResolution(t *testing.T) {
	p := &DockerPlugin{}
	ctx := context.Background()

	tests := []struct {
		name         string
		version      string
		tags         []any
		expectedTags []string
	}{
		{
			name:         "all placeholders",
			version:      "1.2.3",
			tags:         []any{"{{version}}", "{{major}}.{{minor}}", "{{major}}"},
			expectedTags: []string{"1.2.3", "1.2", "1"},
		},
		{
			name:         "static tags",
			version:      "2.0.0",
			tags:         []any{"latest", "stable"},
			expectedTags: []string{"latest", "stable"},
		},
		{
			name:         "default tags when empty",
			version:      "1.0.0",
			tags:         nil,
			expectedTags: []string{"1.0.0", "latest"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := plugin.ExecuteRequest{
				Hook: plugin.HookPostPublish,
				Config: map[string]any{
					"image": "user/app",
					"tags":  tt.tags,
				},
				Context: plugin.ReleaseContext{
					Version: tt.version,
				},
				DryRun: true,
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tags := resp.Outputs["tags"].([]string)
			if len(tags) != len(tt.expectedTags) {
				t.Fatalf("expected %d tags, got %d: %v", len(tt.expectedTags), len(tags), tags)
			}
			for i, expected := range tt.expectedTags {
				if tags[i] != expected {
					t.Errorf("tag[%d]: expected %s, got %s", i, expected, tags[i])
				}
			}
		})
	}
}

func TestImageNameConstruction(t *testing.T) {
	p := &DockerPlugin{}
	ctx := context.Background()

	tests := []struct {
		name           string
		registry       string
		image          string
		expectedPrefix string
	}{
		{
			name:           "docker hub default",
			registry:       "docker.io",
			image:          "user/app",
			expectedPrefix: "user/app",
		},
		{
			name:           "ghcr.io",
			registry:       "ghcr.io",
			image:          "user/app",
			expectedPrefix: "ghcr.io/user/app",
		},
		{
			name:           "custom registry",
			registry:       "registry.example.com",
			image:          "project/app",
			expectedPrefix: "registry.example.com/project/app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := plugin.ExecuteRequest{
				Hook: plugin.HookPostPublish,
				Config: map[string]any{
					"registry": tt.registry,
					"image":    tt.image,
					"tags":     []any{"1.0.0"},
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

			// The image name should start with the expected prefix
			image := resp.Outputs["image"].(string)
			if image != tt.image {
				t.Errorf("expected image %s, got %s", tt.image, image)
			}
		})
	}
}

func TestBuildArgsInConfig(t *testing.T) {
	p := &DockerPlugin{}

	config := map[string]any{
		"image": "user/app",
		"build_args": map[string]any{
			"NODE_ENV":   "production",
			"BUILD_DATE": "2025-01-01",
		},
	}

	cfg := p.parseConfig(config)

	if len(cfg.BuildArgs) != 2 {
		t.Errorf("expected 2 build args, got %d", len(cfg.BuildArgs))
	}

	if cfg.BuildArgs["NODE_ENV"] != "production" {
		t.Errorf("expected NODE_ENV=production, got %s", cfg.BuildArgs["NODE_ENV"])
	}
}

func TestPlatformsConfig(t *testing.T) {
	p := &DockerPlugin{}

	config := map[string]any{
		"image":     "user/app",
		"platforms": []any{"linux/amd64", "linux/arm64", "linux/arm/v7"},
	}

	cfg := p.parseConfig(config)

	if len(cfg.Platforms) != 3 {
		t.Errorf("expected 3 platforms, got %d", len(cfg.Platforms))
	}

	expectedPlatforms := []string{"linux/amd64", "linux/arm64", "linux/arm/v7"}
	for i, platform := range expectedPlatforms {
		if cfg.Platforms[i] != platform {
			t.Errorf("platform[%d]: expected %s, got %s", i, platform, cfg.Platforms[i])
		}
	}
}
