// Package main implements the Docker Hub / Container registry plugin for Relicta.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

// DockerPlugin implements the Docker container registry plugin.
type DockerPlugin struct{}

// Config represents the Docker plugin configuration.
type Config struct {
	// Registry is the container registry URL (default: docker.io for Docker Hub).
	Registry string `json:"registry,omitempty"`
	// Image is the image name (e.g., "user/image" or "ghcr.io/user/image").
	Image string `json:"image,omitempty"`
	// Tags is a list of tags to apply (supports {{version}}, {{major}}, {{minor}}, {{patch}}).
	Tags []string `json:"tags,omitempty"`
	// Dockerfile is the path to the Dockerfile (default: "Dockerfile").
	Dockerfile string `json:"dockerfile,omitempty"`
	// Context is the build context path (default: ".").
	Context string `json:"context,omitempty"`
	// BuildArgs is a map of build arguments.
	BuildArgs map[string]string `json:"build_args,omitempty"`
	// Platforms is a list of target platforms for multi-arch builds.
	Platforms []string `json:"platforms,omitempty"`
	// Username is the registry username.
	Username string `json:"username,omitempty"`
	// Password is the registry password/token.
	Password string `json:"password,omitempty"`
	// Push indicates whether to push after building (default: true).
	Push bool `json:"push"`
	// Labels is a map of labels to add to the image.
	Labels map[string]string `json:"labels,omitempty"`
	// CacheFrom is a list of images to use as cache sources.
	CacheFrom []string `json:"cache_from,omitempty"`
	// NoCache disables build cache.
	NoCache bool `json:"no_cache"`
	// Target is the target build stage.
	Target string `json:"target,omitempty"`
}

// GetInfo returns plugin metadata.
func (p *DockerPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "docker",
		Version:     "1.0.0",
		Description: "Build and push Docker images to container registries",
		Author:      "Relicta Team",
		Hooks: []plugin.Hook{
			plugin.HookPostPublish,
		},
		ConfigSchema: `{
			"type": "object",
			"properties": {
				"registry": {"type": "string", "description": "Container registry URL", "default": "docker.io"},
				"image": {"type": "string", "description": "Image name (e.g., user/image)"},
				"tags": {"type": "array", "items": {"type": "string"}, "description": "Tags to apply (supports {{version}})"},
				"dockerfile": {"type": "string", "description": "Dockerfile path", "default": "Dockerfile"},
				"context": {"type": "string", "description": "Build context", "default": "."},
				"build_args": {"type": "object", "description": "Build arguments"},
				"platforms": {"type": "array", "items": {"type": "string"}, "description": "Target platforms"},
				"username": {"type": "string", "description": "Registry username (or use DOCKER_USERNAME env)"},
				"password": {"type": "string", "description": "Registry password (or use DOCKER_PASSWORD env)"},
				"push": {"type": "boolean", "description": "Push after building", "default": true},
				"labels": {"type": "object", "description": "Image labels"},
				"cache_from": {"type": "array", "items": {"type": "string"}, "description": "Cache source images"},
				"no_cache": {"type": "boolean", "description": "Disable build cache"},
				"target": {"type": "string", "description": "Target build stage"}
			},
			"required": ["image"]
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *DockerPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	cfg := p.parseConfig(req.Config)

	switch req.Hook {
	case plugin.HookPostPublish:
		return p.buildAndPush(ctx, cfg, req.Context, req.DryRun)
	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

// buildAndPush builds the Docker image and pushes it to the registry.
func (p *DockerPlugin) buildAndPush(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	version := strings.TrimPrefix(releaseCtx.Version, "v")
	parts := strings.Split(version, ".")

	major, minor, patch := "", "", ""
	if len(parts) >= 1 {
		major = parts[0]
	}
	if len(parts) >= 2 {
		minor = parts[1]
	}
	if len(parts) >= 3 {
		patch = parts[2]
	}

	// Resolve tags
	tags := cfg.Tags
	if len(tags) == 0 {
		// Default tags: version and latest
		tags = []string{"{{version}}", "latest"}
	}

	resolvedTags := make([]string, 0, len(tags))
	for _, tag := range tags {
		resolved := tag
		resolved = strings.ReplaceAll(resolved, "{{version}}", version)
		resolved = strings.ReplaceAll(resolved, "{{major}}", major)
		resolved = strings.ReplaceAll(resolved, "{{minor}}", minor)
		resolved = strings.ReplaceAll(resolved, "{{patch}}", patch)
		resolvedTags = append(resolvedTags, resolved)
	}

	// Build full image names
	imageNames := make([]string, 0, len(resolvedTags))
	for _, tag := range resolvedTags {
		imageName := cfg.Image
		if cfg.Registry != "" && cfg.Registry != "docker.io" {
			imageName = fmt.Sprintf("%s/%s", cfg.Registry, cfg.Image)
		}
		imageNames = append(imageNames, fmt.Sprintf("%s:%s", imageName, tag))
	}

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Would build and push Docker image",
			Outputs: map[string]any{
				"image":    cfg.Image,
				"tags":     resolvedTags,
				"registry": cfg.Registry,
			},
		}, nil
	}

	// Login to registry if credentials provided
	if cfg.Username != "" && cfg.Password != "" {
		if err := p.dockerLogin(ctx, cfg); err != nil {
			return &plugin.ExecuteResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to login to registry: %v", err),
			}, nil
		}
	}

	// Build image
	if err := p.dockerBuild(ctx, cfg, imageNames, releaseCtx); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to build image: %v", err),
		}, nil
	}

	// Push if enabled
	if cfg.Push {
		for _, imageName := range imageNames {
			if err := p.dockerPush(ctx, imageName); err != nil {
				return &plugin.ExecuteResponse{
					Success: false,
					Error:   fmt.Sprintf("failed to push image %s: %v", imageName, err),
				}, nil
			}
		}
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Built and pushed Docker image with %d tags", len(resolvedTags)),
		Outputs: map[string]any{
			"image":  cfg.Image,
			"tags":   resolvedTags,
			"pushed": cfg.Push,
		},
	}, nil
}

// dockerLogin authenticates with the container registry.
func (p *DockerPlugin) dockerLogin(ctx context.Context, cfg *Config) error {
	registry := cfg.Registry
	if registry == "" || registry == "docker.io" {
		registry = ""
	}

	args := []string{"login"}
	if registry != "" {
		args = append(args, registry)
	}
	args = append(args, "-u", cfg.Username, "--password-stdin")

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = strings.NewReader(cfg.Password)
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// dockerBuild builds the Docker image.
func (p *DockerPlugin) dockerBuild(ctx context.Context, cfg *Config, imageNames []string, releaseCtx plugin.ReleaseContext) error {
	args := []string{"build"}

	// Add tags
	for _, name := range imageNames {
		args = append(args, "-t", name)
	}

	// Dockerfile
	dockerfile := cfg.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	args = append(args, "-f", dockerfile)

	// Build args
	for key, value := range cfg.BuildArgs {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", key, value))
	}

	// Add version as build arg
	args = append(args, "--build-arg", fmt.Sprintf("VERSION=%s", releaseCtx.Version))

	// Platforms for multi-arch
	if len(cfg.Platforms) > 0 {
		args = append(args, "--platform", strings.Join(cfg.Platforms, ","))
	}

	// Labels
	for key, value := range cfg.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", key, value))
	}

	// Cache options
	for _, cache := range cfg.CacheFrom {
		args = append(args, "--cache-from", cache)
	}
	if cfg.NoCache {
		args = append(args, "--no-cache")
	}

	// Target stage
	if cfg.Target != "" {
		args = append(args, "--target", cfg.Target)
	}

	// Build context
	buildContext := cfg.Context
	if buildContext == "" {
		buildContext = "."
	}
	args = append(args, buildContext)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// dockerPush pushes an image to the registry.
func (p *DockerPlugin) dockerPush(ctx context.Context, imageName string) error {
	cmd := exec.CommandContext(ctx, "docker", "push", imageName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// parseConfig parses the plugin configuration.
func (p *DockerPlugin) parseConfig(raw map[string]any) *Config {
	parser := plugin.NewConfigParser(raw)

	return &Config{
		Registry:   parser.GetStringDefault("registry", "docker.io"),
		Image:      parser.GetString("image"),
		Tags:       parser.GetStringSlice("tags"),
		Dockerfile: parser.GetStringDefault("dockerfile", "Dockerfile"),
		Context:    parser.GetStringDefault("context", "."),
		BuildArgs:  parser.GetStringMap("build_args"),
		Platforms:  parser.GetStringSlice("platforms"),
		Username:   parser.GetString("username", "DOCKER_USERNAME"),
		Password:   parser.GetString("password", "DOCKER_PASSWORD", "DOCKER_TOKEN"),
		Push:       parser.GetBoolDefault("push", true),
		Labels:     parser.GetStringMap("labels"),
		CacheFrom:  parser.GetStringSlice("cache_from"),
		NoCache:    parser.GetBool("no_cache"),
		Target:     parser.GetString("target"),
	}
}

// Validate validates the plugin configuration.
func (p *DockerPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := plugin.NewValidationBuilder()
	parser := plugin.NewConfigParser(config)

	// Validate image name
	image := parser.GetString("image")
	if image == "" {
		vb.AddError("image", "Docker image name is required", "required")
	}

	// Validate dockerfile exists (warning only)
	dockerfile := parser.GetStringDefault("dockerfile", "Dockerfile")
	if _, err := os.Stat(dockerfile); os.IsNotExist(err) {
		vb.AddWarning("dockerfile", fmt.Sprintf("Dockerfile '%s' not found", dockerfile))
	}

	// Validate credentials if push is enabled
	push := parser.GetBoolDefault("push", true)
	if push {
		username := parser.GetString("username", "DOCKER_USERNAME")
		password := parser.GetString("password", "DOCKER_PASSWORD", "DOCKER_TOKEN")
		if username == "" || password == "" {
			vb.AddWarning("credentials", "Docker credentials not configured - push may fail")
		}
	}

	return vb.Build(), nil
}
