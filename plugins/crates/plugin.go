// Package main implements the crates.io plugin for Relicta.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

// Security validation patterns
var (
	// crateNamePattern validates Rust crate names
	crateNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
)

// CratesPlugin implements the crates.io publish plugin.
type CratesPlugin struct{}

// Config represents the crates.io plugin configuration.
type Config struct {
	// Token is the API token for crates.io authentication.
	Token string `json:"token,omitempty"`
	// Registry is the alternative registry URL.
	Registry string `json:"registry,omitempty"`
	// AllowDirty allows publishing with uncommitted changes.
	AllowDirty bool `json:"allow_dirty"`
	// NoVerify skips the crate verification step.
	NoVerify bool `json:"no_verify"`
	// Features are the features to enable when building.
	Features []string `json:"features,omitempty"`
	// AllFeatures enables all features when building.
	AllFeatures bool `json:"all_features"`
	// NoDefaultFeatures disables default features.
	NoDefaultFeatures bool `json:"no_default_features"`
	// PackageDir is the directory containing Cargo.toml.
	PackageDir string `json:"package_dir,omitempty"`
	// UpdateVersion updates version in Cargo.toml.
	UpdateVersion bool `json:"update_version"`
	// Workspace publishes all workspace members.
	Workspace bool `json:"workspace"`
	// Package specifies specific package to publish in workspace.
	Package string `json:"package,omitempty"`
}

// GetInfo returns plugin metadata.
func (p *CratesPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "crates",
		Version:     "1.0.0",
		Description: "Publish packages to crates.io (Rust)",
		Author:      "Relicta Team",
		Hooks: []plugin.Hook{
			plugin.HookPrePublish,
			plugin.HookPostPublish,
		},
		ConfigSchema: `{
			"type": "object",
			"properties": {
				"token": {"type": "string", "description": "crates.io API token"},
				"registry": {"type": "string", "description": "Alternative registry URL"},
				"allow_dirty": {"type": "boolean", "description": "Allow uncommitted changes", "default": false},
				"no_verify": {"type": "boolean", "description": "Skip verification step", "default": false},
				"features": {"type": "array", "items": {"type": "string"}, "description": "Features to enable"},
				"all_features": {"type": "boolean", "description": "Enable all features", "default": false},
				"no_default_features": {"type": "boolean", "description": "Disable default features", "default": false},
				"package_dir": {"type": "string", "description": "Directory containing Cargo.toml"},
				"update_version": {"type": "boolean", "description": "Update version in Cargo.toml", "default": true},
				"workspace": {"type": "boolean", "description": "Publish all workspace members", "default": false},
				"package": {"type": "string", "description": "Specific package to publish in workspace"}
			}
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *CratesPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	cfg := p.parseConfig(req.Config)

	switch req.Hook {
	case plugin.HookPrePublish:
		if cfg.UpdateVersion {
			return p.updateCargoVersion(ctx, cfg, req.Context, req.DryRun)
		}
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Version update disabled",
		}, nil

	case plugin.HookPostPublish:
		return p.publishCrate(ctx, cfg, req.Context, req.DryRun)

	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

// updateCargoVersion updates the version in Cargo.toml.
func (p *CratesPlugin) updateCargoVersion(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	packageDir, err := p.validatePackageDir(cfg.PackageDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid package directory: %v", err),
		}, nil
	}

	cargoPath := filepath.Join(packageDir, "Cargo.toml")

	data, err := os.ReadFile(cargoPath)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to read Cargo.toml: %v", err),
		}, nil
	}

	content := string(data)

	// Find and update version in [package] section
	versionPattern := regexp.MustCompile(`(?m)^version\s*=\s*"([^"]+)"`)
	match := versionPattern.FindStringSubmatch(content)

	oldVersion := ""
	if len(match) > 1 {
		oldVersion = match[1]
	}

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Would update Cargo.toml version from %s to %s", oldVersion, releaseCtx.Version),
			Outputs: map[string]any{
				"old_version": oldVersion,
				"new_version": releaseCtx.Version,
			},
		}, nil
	}

	newContent := versionPattern.ReplaceAllString(content, fmt.Sprintf(`version = "%s"`, releaseCtx.Version))

	if err := os.WriteFile(cargoPath, []byte(newContent), 0644); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to write Cargo.toml: %v", err),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Updated Cargo.toml version to %s", releaseCtx.Version),
		Outputs: map[string]any{
			"old_version": oldVersion,
			"new_version": releaseCtx.Version,
		},
	}, nil
}

// publishCrate publishes the crate to crates.io.
func (p *CratesPlugin) publishCrate(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	packageDir, err := p.validatePackageDir(cfg.PackageDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid package directory: %v", err),
		}, nil
	}

	// Build cargo publish command
	args := []string{"publish"}

	if cfg.Token != "" {
		args = append(args, "--token", cfg.Token)
	}

	if cfg.Registry != "" {
		args = append(args, "--registry", cfg.Registry)
	}

	if cfg.AllowDirty {
		args = append(args, "--allow-dirty")
	}

	if cfg.NoVerify {
		args = append(args, "--no-verify")
	}

	if cfg.AllFeatures {
		args = append(args, "--all-features")
	} else if len(cfg.Features) > 0 {
		args = append(args, "--features", strings.Join(cfg.Features, ","))
	}

	if cfg.NoDefaultFeatures {
		args = append(args, "--no-default-features")
	}

	if cfg.Package != "" {
		args = append(args, "--package", cfg.Package)
	}

	// Build log-safe command string
	logArgs := make([]string, len(args))
	copy(logArgs, args)
	for i := range logArgs {
		if i > 0 && logArgs[i-1] == "--token" {
			logArgs[i] = "[REDACTED]"
		}
	}
	cmdStr := fmt.Sprintf("cargo %s", strings.Join(logArgs, " "))

	if dryRun {
		// Use cargo publish --dry-run for verification
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Would publish to crates.io",
			Outputs: map[string]any{
				"command":     cmdStr,
				"version":     releaseCtx.Version,
				"package_dir": packageDir,
				"registry":    cfg.Registry,
			},
		}, nil
	}

	cmd := exec.CommandContext(ctx, "cargo", args...)
	cmd.Dir = packageDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set token via environment if provided
	if cfg.Token != "" {
		cmd.Env = append(os.Environ(), "CARGO_REGISTRY_TOKEN="+cfg.Token)
	}

	if err := cmd.Run(); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("cargo publish failed: %v\nstderr: %s", err, stderr.String()),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Published version %s to crates.io", releaseCtx.Version),
		Outputs: map[string]any{
			"version":  releaseCtx.Version,
			"registry": cfg.Registry,
			"stdout":   stdout.String(),
		},
	}, nil
}

// validatePackageDir validates and returns the package directory.
func (p *CratesPlugin) validatePackageDir(dir string) (string, error) {
	if dir == "" {
		return ".", nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	cleanPath := filepath.Clean(dir)

	if strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, "/../") {
		return "", fmt.Errorf("path traversal not allowed")
	}

	var absPath string
	if filepath.IsAbs(cleanPath) {
		absPath = cleanPath
	} else {
		absPath = filepath.Join(cwd, cleanPath)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("directory not accessible: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory")
	}

	return absPath, nil
}

// parseConfig parses the plugin configuration.
func (p *CratesPlugin) parseConfig(raw map[string]any) *Config {
	parser := plugin.NewConfigParser(raw)

	return &Config{
		Token:             parser.GetString("token", "CARGO_REGISTRY_TOKEN", "CRATES_TOKEN"),
		Registry:          parser.GetString("registry"),
		AllowDirty:        parser.GetBool("allow_dirty"),
		NoVerify:          parser.GetBool("no_verify"),
		Features:          parser.GetStringSlice("features"),
		AllFeatures:       parser.GetBool("all_features"),
		NoDefaultFeatures: parser.GetBool("no_default_features"),
		PackageDir:        parser.GetString("package_dir"),
		UpdateVersion:     parser.GetBoolDefault("update_version", true),
		Workspace:         parser.GetBool("workspace"),
		Package:           parser.GetString("package"),
	}
}

// Validate validates the plugin configuration.
func (p *CratesPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := plugin.NewValidationBuilder()
	parser := plugin.NewConfigParser(config)

	// Check that cargo is available
	if _, err := exec.LookPath("cargo"); err != nil {
		vb.AddError("", "cargo command not found in PATH", "dependency")
	}

	// Check package directory if provided
	if dir := parser.GetString("package_dir"); dir != "" {
		cargoPath := filepath.Join(dir, "Cargo.toml")
		if _, err := os.Stat(cargoPath); err != nil {
			vb.AddError("package_dir", fmt.Sprintf("Cargo.toml not found at %s", cargoPath), "path")
		}
	}

	// Validate package name if provided
	if pkg := parser.GetString("package"); pkg != "" {
		if !crateNamePattern.MatchString(pkg) {
			vb.AddError("package", "invalid crate name format", "format")
		}
	}

	// Check authentication
	token := parser.GetString("token", "CARGO_REGISTRY_TOKEN", "CRATES_TOKEN")
	if token == "" {
		vb.AddWarning("authentication", "no token configured; using cargo credentials")
	}

	return vb.Build(), nil
}
