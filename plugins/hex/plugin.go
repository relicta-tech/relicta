// Package main implements the Hex plugin for ReleasePilot.
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

	"github.com/felixgeelhaar/release-pilot/pkg/plugin"
)

// HexPlugin implements the Hex publish plugin.
type HexPlugin struct{}

// Config represents the Hex plugin configuration.
type Config struct {
	// APIKey is the Hex.pm API key.
	APIKey string `json:"api_key,omitempty"`
	// Organization is the Hex organization name.
	Organization string `json:"organization,omitempty"`
	// ProjectDir is the directory containing mix.exs.
	ProjectDir string `json:"project_dir,omitempty"`
	// UpdateVersion updates version in mix.exs.
	UpdateVersion bool `json:"update_version"`
	// Replace replaces an existing package version.
	Replace bool `json:"replace"`
	// Revert reverts to a previous version if publish fails.
	Revert bool `json:"revert"`
}

// GetInfo returns plugin metadata.
func (p *HexPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "hex",
		Version:     "1.0.0",
		Description: "Publish packages to Hex.pm (Elixir/Erlang)",
		Author:      "ReleasePilot Team",
		Hooks: []plugin.Hook{
			plugin.HookPrePublish,
			plugin.HookPostPublish,
		},
		ConfigSchema: `{
			"type": "object",
			"properties": {
				"api_key": {"type": "string", "description": "Hex.pm API key"},
				"organization": {"type": "string", "description": "Hex organization name"},
				"project_dir": {"type": "string", "description": "Directory containing mix.exs"},
				"update_version": {"type": "boolean", "description": "Update version in mix.exs", "default": true},
				"replace": {"type": "boolean", "description": "Replace existing version", "default": false},
				"revert": {"type": "boolean", "description": "Revert on failure", "default": false}
			}
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *HexPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	cfg := p.parseConfig(req.Config)

	switch req.Hook {
	case plugin.HookPrePublish:
		if cfg.UpdateVersion {
			return p.updateVersion(ctx, cfg, req.Context, req.DryRun)
		}
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Version update disabled",
		}, nil

	case plugin.HookPostPublish:
		return p.publishPackage(ctx, cfg, req.Context, req.DryRun)

	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

// updateVersion updates the version in mix.exs.
func (p *HexPlugin) updateVersion(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	projectDir, err := p.validateProjectDir(cfg.ProjectDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid project directory: %v", err),
		}, nil
	}

	mixPath := filepath.Join(projectDir, "mix.exs")

	data, err := os.ReadFile(mixPath)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to read mix.exs: %v", err),
		}, nil
	}

	content := string(data)

	// Pattern for version: "x.y.z"
	versionPattern := regexp.MustCompile(`(version:\s*["'])([^"']+)(["'])`)
	match := versionPattern.FindStringSubmatch(content)

	oldVersion := ""
	if len(match) > 2 {
		oldVersion = match[2]
	}

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Would update mix.exs version from %s to %s", oldVersion, releaseCtx.Version),
			Outputs: map[string]any{
				"old_version": oldVersion,
				"new_version": releaseCtx.Version,
			},
		}, nil
	}

	newContent := versionPattern.ReplaceAllString(content, fmt.Sprintf(`${1}%s${3}`, releaseCtx.Version))

	if err := os.WriteFile(mixPath, []byte(newContent), 0644); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to write mix.exs: %v", err),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Updated mix.exs version to %s", releaseCtx.Version),
		Outputs: map[string]any{
			"old_version": oldVersion,
			"new_version": releaseCtx.Version,
		},
	}, nil
}

// publishPackage publishes the package to Hex.pm.
func (p *HexPlugin) publishPackage(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	projectDir, err := p.validateProjectDir(cfg.ProjectDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid project directory: %v", err),
		}, nil
	}

	// Build hex.publish command
	args := []string{"hex.publish"}

	if cfg.Organization != "" {
		args = append(args, "--organization", cfg.Organization)
	}

	if cfg.Replace {
		args = append(args, "--replace")
	}

	if cfg.Revert {
		args = append(args, "--revert")
	}

	// Always pass --yes for non-interactive mode
	args = append(args, "--yes")

	cmdStr := fmt.Sprintf("mix %s", strings.Join(args, " "))

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Would publish to Hex.pm",
			Outputs: map[string]any{
				"command":      cmdStr,
				"version":      releaseCtx.Version,
				"project_dir":  projectDir,
				"organization": cfg.Organization,
			},
		}, nil
	}

	cmd := exec.CommandContext(ctx, "mix", args...)
	cmd.Dir = projectDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set HEX_API_KEY if provided
	if cfg.APIKey != "" {
		cmd.Env = append(os.Environ(), "HEX_API_KEY="+cfg.APIKey)
	}

	if err := cmd.Run(); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("mix hex.publish failed: %v\nstderr: %s", err, stderr.String()),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Published version %s to Hex.pm", releaseCtx.Version),
		Outputs: map[string]any{
			"version":      releaseCtx.Version,
			"organization": cfg.Organization,
			"stdout":       stdout.String(),
		},
	}, nil
}

// validateProjectDir validates and returns the project directory.
func (p *HexPlugin) validateProjectDir(dir string) (string, error) {
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
func (p *HexPlugin) parseConfig(raw map[string]any) *Config {
	parser := plugin.NewConfigParser(raw)

	return &Config{
		APIKey:        parser.GetString("api_key", "HEX_API_KEY"),
		Organization:  parser.GetString("organization"),
		ProjectDir:    parser.GetString("project_dir"),
		UpdateVersion: parser.GetBoolDefault("update_version", true),
		Replace:       parser.GetBool("replace"),
		Revert:        parser.GetBool("revert"),
	}
}

// Validate validates the plugin configuration.
func (p *HexPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := plugin.NewValidationBuilder()
	parser := plugin.NewConfigParser(config)

	// Check that mix is available
	if _, err := exec.LookPath("mix"); err != nil {
		vb.AddError("", "mix command not found in PATH", "dependency")
	}

	// Check project directory if provided
	if dir := parser.GetString("project_dir"); dir != "" {
		mixPath := filepath.Join(dir, "mix.exs")
		if _, err := os.Stat(mixPath); err != nil {
			vb.AddError("project_dir", "mix.exs not found", "path")
		}
	}

	// Check API key
	apiKey := parser.GetString("api_key", "HEX_API_KEY")
	if apiKey == "" {
		vb.AddWarning("api_key", "no API key configured; publish may fail")
	}

	return vb.Build(), nil
}
