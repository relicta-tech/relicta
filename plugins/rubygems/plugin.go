// Package main implements the RubyGems plugin for Relicta.
package main

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

// RubyGemsPlugin implements the RubyGems publish plugin.
type RubyGemsPlugin struct{}

// Config represents the RubyGems plugin configuration.
type Config struct {
	// APIKey is the RubyGems API key.
	APIKey string `json:"api_key,omitempty"`
	// Host is the RubyGems server URL.
	Host string `json:"host,omitempty"`
	// GemDir is the directory containing the .gemspec file.
	GemDir string `json:"gem_dir,omitempty"`
	// GemName is the name of the gem.
	GemName string `json:"gem_name,omitempty"`
	// UpdateVersion updates version in version.rb or gemspec.
	UpdateVersion bool `json:"update_version"`
	// VersionFile is the path to the version file.
	VersionFile string `json:"version_file,omitempty"`
	// AllowPrerelease allows pushing prerelease versions.
	AllowPrerelease bool `json:"allow_prerelease"`
}

// GetInfo returns plugin metadata.
func (p *RubyGemsPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "rubygems",
		Version:     "1.0.0",
		Description: "Publish gems to RubyGems.org",
		Author:      "Relicta Team",
		Hooks: []plugin.Hook{
			plugin.HookPrePublish,
			plugin.HookPostPublish,
		},
		ConfigSchema: `{
			"type": "object",
			"properties": {
				"api_key": {"type": "string", "description": "RubyGems API key"},
				"host": {"type": "string", "description": "RubyGems server URL"},
				"gem_dir": {"type": "string", "description": "Directory containing .gemspec"},
				"gem_name": {"type": "string", "description": "Gem name"},
				"update_version": {"type": "boolean", "description": "Update version file", "default": true},
				"version_file": {"type": "string", "description": "Path to version.rb file"},
				"allow_prerelease": {"type": "boolean", "description": "Allow prerelease versions", "default": false}
			}
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *RubyGemsPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
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
		return p.publishGem(ctx, cfg, req.Context, req.DryRun)

	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

// updateVersion updates the version in version.rb or gemspec.
func (p *RubyGemsPlugin) updateVersion(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	gemDir, err := p.validateGemDir(cfg.GemDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid gem directory: %v", err),
		}, nil
	}

	// Find version file
	versionFile := cfg.VersionFile
	if versionFile == "" {
		versionFile = p.findVersionFile(gemDir)
	}

	if versionFile == "" {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   "could not find version.rb or gemspec file",
		}, nil
	}

	data, err := os.ReadFile(versionFile)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to read version file: %v", err),
		}, nil
	}

	content := string(data)

	// Pattern for VERSION = "x.y.z"
	versionPattern := regexp.MustCompile(`(VERSION\s*=\s*["'])([^"']+)(["'])`)
	match := versionPattern.FindStringSubmatch(content)

	oldVersion := ""
	if len(match) > 2 {
		oldVersion = match[2]
	}

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Would update version from %s to %s", oldVersion, releaseCtx.Version),
			Outputs: map[string]any{
				"old_version":  oldVersion,
				"new_version":  releaseCtx.Version,
				"version_file": versionFile,
			},
		}, nil
	}

	newContent := versionPattern.ReplaceAllString(content, fmt.Sprintf(`${1}%s${3}`, releaseCtx.Version))

	if err := os.WriteFile(versionFile, []byte(newContent), 0644); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to write version file: %v", err),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Updated version to %s", releaseCtx.Version),
		Outputs: map[string]any{
			"old_version":  oldVersion,
			"new_version":  releaseCtx.Version,
			"version_file": versionFile,
		},
	}, nil
}

// findVersionFile finds the version.rb or gemspec file.
func (p *RubyGemsPlugin) findVersionFile(gemDir string) string {
	// Try lib/**/version.rb first
	matches, _ := filepath.Glob(filepath.Join(gemDir, "lib", "**", "version.rb"))
	if len(matches) > 0 {
		return matches[0]
	}

	// Try lib/*/version.rb
	matches, _ = filepath.Glob(filepath.Join(gemDir, "lib", "*", "version.rb"))
	if len(matches) > 0 {
		return matches[0]
	}

	// Try *.gemspec
	matches, _ = filepath.Glob(filepath.Join(gemDir, "*.gemspec"))
	if len(matches) > 0 {
		return matches[0]
	}

	return ""
}

// publishGem publishes the gem to RubyGems.
func (p *RubyGemsPlugin) publishGem(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	if err := p.validateConfig(cfg); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("configuration validation failed: %v", err),
		}, nil
	}

	gemDir, err := p.validateGemDir(cfg.GemDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid gem directory: %v", err),
		}, nil
	}

	// Find gemspec
	gemspecPath, err := p.findGemspec(gemDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to find gemspec: %v", err),
		}, nil
	}

	// Build gem
	buildArgs := []string{"build", filepath.Base(gemspecPath)}

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Would build and push gem",
			Outputs: map[string]any{
				"build_command": fmt.Sprintf("gem build %s", filepath.Base(gemspecPath)),
				"push_command":  "gem push *.gem",
				"version":       releaseCtx.Version,
				"gem_dir":       gemDir,
			},
		}, nil
	}

	// Execute gem build
	buildCmd := exec.CommandContext(ctx, "gem", buildArgs...)
	buildCmd.Dir = gemDir

	var buildStdout, buildStderr bytes.Buffer
	buildCmd.Stdout = &buildStdout
	buildCmd.Stderr = &buildStderr

	if err := buildCmd.Run(); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("gem build failed: %v\nstderr: %s", err, buildStderr.String()),
		}, nil
	}

	// Find built gem file
	gemName := cfg.GemName
	if gemName == "" {
		// Extract from gemspec filename
		gemName = strings.TrimSuffix(filepath.Base(gemspecPath), ".gemspec")
	}

	gemFile := filepath.Join(gemDir, fmt.Sprintf("%s-%s.gem", gemName, releaseCtx.Version))

	// Push gem
	pushArgs := []string{"push", gemFile}

	if cfg.APIKey != "" {
		pushArgs = append(pushArgs, "--key", cfg.APIKey)
	}

	if cfg.Host != "" {
		pushArgs = append(pushArgs, "--host", cfg.Host)
	}

	pushCmd := exec.CommandContext(ctx, "gem", pushArgs...)
	pushCmd.Dir = gemDir

	var pushStdout, pushStderr bytes.Buffer
	pushCmd.Stdout = &pushStdout
	pushCmd.Stderr = &pushStderr

	// Set GEM_HOST_API_KEY if provided
	if cfg.APIKey != "" {
		pushCmd.Env = append(os.Environ(), "GEM_HOST_API_KEY="+cfg.APIKey)
	}

	if err := pushCmd.Run(); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("gem push failed: %v\nstderr: %s", err, pushStderr.String()),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Published %s-%s to RubyGems", gemName, releaseCtx.Version),
		Outputs: map[string]any{
			"gem_name": gemName,
			"version":  releaseCtx.Version,
			"gem_file": gemFile,
			"stdout":   pushStdout.String(),
		},
	}, nil
}

// findGemspec finds the .gemspec file in the directory.
func (p *RubyGemsPlugin) findGemspec(gemDir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(gemDir, "*.gemspec"))
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no .gemspec file found in %s", gemDir)
	}

	return matches[0], nil
}

// validateConfig validates the plugin configuration.
func (p *RubyGemsPlugin) validateConfig(cfg *Config) error {
	if cfg.Host != "" {
		parsedURL, err := url.Parse(cfg.Host)
		if err != nil {
			return fmt.Errorf("invalid host URL: %w", err)
		}
		if parsedURL.Scheme != "https" {
			hostname := parsedURL.Hostname()
			if parsedURL.Scheme == "http" && (hostname == "localhost" || strings.HasPrefix(hostname, "127.")) {
				// Allow http for local testing
			} else {
				return fmt.Errorf("host must use HTTPS")
			}
		}
	}

	return nil
}

// validateGemDir validates and returns the gem directory.
func (p *RubyGemsPlugin) validateGemDir(dir string) (string, error) {
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
func (p *RubyGemsPlugin) parseConfig(raw map[string]any) *Config {
	parser := plugin.NewConfigParser(raw)

	return &Config{
		APIKey:          parser.GetString("api_key", "GEM_HOST_API_KEY", "RUBYGEMS_API_KEY"),
		Host:            parser.GetString("host"),
		GemDir:          parser.GetString("gem_dir"),
		GemName:         parser.GetString("gem_name"),
		UpdateVersion:   parser.GetBoolDefault("update_version", true),
		VersionFile:     parser.GetString("version_file"),
		AllowPrerelease: parser.GetBool("allow_prerelease"),
	}
}

// Validate validates the plugin configuration.
func (p *RubyGemsPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := plugin.NewValidationBuilder()
	parser := plugin.NewConfigParser(config)

	// Check that gem is available
	if _, err := exec.LookPath("gem"); err != nil {
		vb.AddError("", "gem command not found in PATH", "dependency")
	}

	// Check gem directory if provided
	if dir := parser.GetString("gem_dir"); dir != "" {
		matches, _ := filepath.Glob(filepath.Join(dir, "*.gemspec"))
		if len(matches) == 0 {
			vb.AddError("gem_dir", "no .gemspec file found", "path")
		}
	}

	// Check API key
	apiKey := parser.GetString("api_key", "GEM_HOST_API_KEY", "RUBYGEMS_API_KEY")
	if apiKey == "" {
		vb.AddWarning("api_key", "no API key configured; push may fail")
	}

	return vb.Build(), nil
}
