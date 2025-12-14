// Package main implements the NuGet plugin for Relicta.
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

// NuGetPlugin implements the NuGet publish plugin.
type NuGetPlugin struct{}

// Config represents the NuGet plugin configuration.
type Config struct {
	// APIKey is the NuGet API key.
	APIKey string `json:"api_key,omitempty"`
	// Source is the NuGet server URL.
	Source string `json:"source,omitempty"`
	// SymbolSource is the symbol server URL.
	SymbolSource string `json:"symbol_source,omitempty"`
	// SkipDuplicate skips pushing if package version exists.
	SkipDuplicate bool `json:"skip_duplicate"`
	// NoSymbols skips symbol package push.
	NoSymbols bool `json:"no_symbols"`
	// ProjectDir is the directory containing .csproj/.fsproj.
	ProjectDir string `json:"project_dir,omitempty"`
	// Configuration is the build configuration (Release, Debug).
	Configuration string `json:"configuration,omitempty"`
	// UpdateVersion updates version in .csproj file.
	UpdateVersion bool `json:"update_version"`
	// PackageID is the package identifier.
	PackageID string `json:"package_id,omitempty"`
	// OutputDir is the directory for .nupkg files.
	OutputDir string `json:"output_dir,omitempty"`
}

// GetInfo returns plugin metadata.
func (p *NuGetPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "nuget",
		Version:     "1.0.0",
		Description: "Publish packages to NuGet (.NET)",
		Author:      "Relicta Team",
		Hooks: []plugin.Hook{
			plugin.HookPrePublish,
			plugin.HookPostPublish,
		},
		ConfigSchema: `{
			"type": "object",
			"properties": {
				"api_key": {"type": "string", "description": "NuGet API key"},
				"source": {"type": "string", "description": "NuGet server URL", "default": "https://api.nuget.org/v3/index.json"},
				"symbol_source": {"type": "string", "description": "Symbol server URL"},
				"skip_duplicate": {"type": "boolean", "description": "Skip if version exists", "default": false},
				"no_symbols": {"type": "boolean", "description": "Skip symbol push", "default": false},
				"project_dir": {"type": "string", "description": "Directory containing .csproj"},
				"configuration": {"type": "string", "description": "Build configuration", "default": "Release"},
				"update_version": {"type": "boolean", "description": "Update version in .csproj", "default": true},
				"package_id": {"type": "string", "description": "Package identifier"},
				"output_dir": {"type": "string", "description": "Output directory for .nupkg"}
			}
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *NuGetPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
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

// updateVersion updates the version in .csproj file.
func (p *NuGetPlugin) updateVersion(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	projectDir, err := p.validateProjectDir(cfg.ProjectDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid project directory: %v", err),
		}, nil
	}

	// Find .csproj or .fsproj file
	csprojPath, err := p.findProjectFile(projectDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to find project file: %v", err),
		}, nil
	}

	data, err := os.ReadFile(csprojPath)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to read project file: %v", err),
		}, nil
	}

	content := string(data)

	// Find and update version
	versionPattern := regexp.MustCompile(`<Version>([^<]+)</Version>`)
	match := versionPattern.FindStringSubmatch(content)

	oldVersion := ""
	if len(match) > 1 {
		oldVersion = match[1]
	}

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Would update .csproj version from %s to %s", oldVersion, releaseCtx.Version),
			Outputs: map[string]any{
				"old_version": oldVersion,
				"new_version": releaseCtx.Version,
				"file":        csprojPath,
			},
		}, nil
	}

	newContent := versionPattern.ReplaceAllString(content, fmt.Sprintf("<Version>%s</Version>", releaseCtx.Version))

	// If no version tag exists, add it in PropertyGroup
	if !versionPattern.MatchString(content) {
		propGroupPattern := regexp.MustCompile(`(<PropertyGroup[^>]*>)`)
		newContent = propGroupPattern.ReplaceAllString(content, fmt.Sprintf("$1\n    <Version>%s</Version>", releaseCtx.Version))
	}

	if err := os.WriteFile(csprojPath, []byte(newContent), 0644); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to write project file: %v", err),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Updated .csproj version to %s", releaseCtx.Version),
		Outputs: map[string]any{
			"old_version": oldVersion,
			"new_version": releaseCtx.Version,
			"file":        csprojPath,
		},
	}, nil
}

// findProjectFile finds .csproj or .fsproj in the directory.
func (p *NuGetPlugin) findProjectFile(dir string) (string, error) {
	patterns := []string{"*.csproj", "*.fsproj", "*.vbproj"}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err == nil && len(matches) > 0 {
			return matches[0], nil
		}
	}

	return "", fmt.Errorf("no .csproj, .fsproj, or .vbproj file found in %s", dir)
}

// publishPackage publishes the package to NuGet.
func (p *NuGetPlugin) publishPackage(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	if err := p.validateConfig(cfg); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("configuration validation failed: %v", err),
		}, nil
	}

	projectDir, err := p.validateProjectDir(cfg.ProjectDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid project directory: %v", err),
		}, nil
	}

	// Build pack command
	packArgs := []string{"pack"}

	if cfg.Configuration != "" {
		packArgs = append(packArgs, "-c", cfg.Configuration)
	}

	outputDir := cfg.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(projectDir, "bin", "Release")
	}
	packArgs = append(packArgs, "-o", outputDir)

	// Build push command
	pushArgs := []string{"nuget", "push"}

	nupkgPattern := filepath.Join(outputDir, "*.nupkg")
	pushArgs = append(pushArgs, nupkgPattern)

	source := cfg.Source
	if source == "" {
		source = "https://api.nuget.org/v3/index.json"
	}
	pushArgs = append(pushArgs, "-s", source)

	if cfg.APIKey != "" {
		pushArgs = append(pushArgs, "-k", cfg.APIKey)
	}

	if cfg.SkipDuplicate {
		pushArgs = append(pushArgs, "--skip-duplicate")
	}

	if cfg.NoSymbols {
		pushArgs = append(pushArgs, "--no-symbols")
	}

	// Build log-safe command strings
	packCmdStr := fmt.Sprintf("dotnet %s", strings.Join(packArgs, " "))

	logPushArgs := make([]string, len(pushArgs))
	copy(logPushArgs, pushArgs)
	for i := range logPushArgs {
		if i > 0 && logPushArgs[i-1] == "-k" {
			logPushArgs[i] = "[REDACTED]"
		}
	}
	pushCmdStr := fmt.Sprintf("dotnet %s", strings.Join(logPushArgs, " "))

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Would build and push to NuGet",
			Outputs: map[string]any{
				"pack_command": packCmdStr,
				"push_command": pushCmdStr,
				"version":      releaseCtx.Version,
				"source":       source,
				"project_dir":  projectDir,
			},
		}, nil
	}

	// Execute pack
	packCmd := exec.CommandContext(ctx, "dotnet", packArgs...)
	packCmd.Dir = projectDir

	var packStdout, packStderr bytes.Buffer
	packCmd.Stdout = &packStdout
	packCmd.Stderr = &packStderr

	if err := packCmd.Run(); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("dotnet pack failed: %v\nstderr: %s", err, packStderr.String()),
		}, nil
	}

	// Execute push
	pushCmd := exec.CommandContext(ctx, "dotnet", pushArgs...)
	pushCmd.Dir = projectDir

	var pushStdout, pushStderr bytes.Buffer
	pushCmd.Stdout = &pushStdout
	pushCmd.Stderr = &pushStderr

	if err := pushCmd.Run(); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("dotnet nuget push failed: %v\nstderr: %s", err, pushStderr.String()),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Published version %s to NuGet", releaseCtx.Version),
		Outputs: map[string]any{
			"version": releaseCtx.Version,
			"source":  source,
			"stdout":  pushStdout.String(),
		},
	}, nil
}

// validateConfig validates the plugin configuration.
func (p *NuGetPlugin) validateConfig(cfg *Config) error {
	if cfg.Source != "" {
		parsedURL, err := url.Parse(cfg.Source)
		if err != nil {
			return fmt.Errorf("invalid source URL: %w", err)
		}
		if parsedURL.Scheme != "https" {
			hostname := parsedURL.Hostname()
			if parsedURL.Scheme == "http" && (hostname == "localhost" || strings.HasPrefix(hostname, "127.")) {
				// Allow http for local testing
			} else {
				return fmt.Errorf("source must use HTTPS")
			}
		}
	}

	return nil
}

// validateProjectDir validates and returns the project directory.
func (p *NuGetPlugin) validateProjectDir(dir string) (string, error) {
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
func (p *NuGetPlugin) parseConfig(raw map[string]any) *Config {
	parser := plugin.NewConfigParser(raw)

	return &Config{
		APIKey:        parser.GetString("api_key", "NUGET_API_KEY"),
		Source:        parser.GetStringDefault("source", "https://api.nuget.org/v3/index.json"),
		SymbolSource:  parser.GetString("symbol_source"),
		SkipDuplicate: parser.GetBool("skip_duplicate"),
		NoSymbols:     parser.GetBool("no_symbols"),
		ProjectDir:    parser.GetString("project_dir"),
		Configuration: parser.GetStringDefault("configuration", "Release"),
		UpdateVersion: parser.GetBoolDefault("update_version", true),
		PackageID:     parser.GetString("package_id"),
		OutputDir:     parser.GetString("output_dir"),
	}
}

// Validate validates the plugin configuration.
func (p *NuGetPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := plugin.NewValidationBuilder()
	parser := plugin.NewConfigParser(config)

	// Check that dotnet is available
	if _, err := exec.LookPath("dotnet"); err != nil {
		vb.AddError("", "dotnet command not found in PATH", "dependency")
	}

	// Check project directory if provided
	if dir := parser.GetString("project_dir"); dir != "" {
		// Check for .csproj, .fsproj, or .vbproj
		patterns := []string{"*.csproj", "*.fsproj", "*.vbproj"}
		found := false

		for _, pattern := range patterns {
			matches, err := filepath.Glob(filepath.Join(dir, pattern))
			if err == nil && len(matches) > 0 {
				found = true
				break
			}
		}

		if !found {
			vb.AddError("project_dir", "no .csproj, .fsproj, or .vbproj file found", "path")
		}
	}

	// Check API key
	apiKey := parser.GetString("api_key", "NUGET_API_KEY")
	if apiKey == "" {
		vb.AddWarning("api_key", "no API key configured; push may fail")
	}

	return vb.Build(), nil
}
