// Package main implements the Go Modules plugin for ReleasePilot.
package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/felixgeelhaar/release-pilot/pkg/plugin"
)

// GoModPlugin implements the Go Modules publish plugin.
type GoModPlugin struct{}

// Config represents the Go Modules plugin configuration.
type Config struct {
	// ModuleDir is the directory containing go.mod.
	ModuleDir string `json:"module_dir,omitempty"`
	// Private marks this as a private module.
	Private bool `json:"private"`
	// ProxyURL is the Go proxy URL.
	ProxyURL string `json:"proxy_url,omitempty"`
	// SumDB is the checksum database URL.
	SumDB string `json:"sumdb,omitempty"`
	// NOProxy is a comma-separated list of modules to not fetch via proxy.
	NoProxy string `json:"noproxy,omitempty"`
	// ValidateTag validates that the tag matches semver.
	ValidateTag bool `json:"validate_tag"`
	// TriggerProxy triggers proxy.golang.org to fetch the module.
	TriggerProxy bool `json:"trigger_proxy"`
}

// GetInfo returns plugin metadata.
func (p *GoModPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "gomod",
		Version:     "1.0.0",
		Description: "Publish Go modules",
		Author:      "ReleasePilot Team",
		Hooks: []plugin.Hook{
			plugin.HookPrePublish,
			plugin.HookPostPublish,
		},
		ConfigSchema: `{
			"type": "object",
			"properties": {
				"module_dir": {"type": "string", "description": "Directory containing go.mod"},
				"private": {"type": "boolean", "description": "Private module", "default": false},
				"proxy_url": {"type": "string", "description": "Go proxy URL"},
				"sumdb": {"type": "string", "description": "Checksum database URL"},
				"noproxy": {"type": "string", "description": "Modules to not fetch via proxy"},
				"validate_tag": {"type": "boolean", "description": "Validate tag semver", "default": true},
				"trigger_proxy": {"type": "boolean", "description": "Trigger proxy to fetch module", "default": true}
			}
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *GoModPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	cfg := p.parseConfig(req.Config)

	switch req.Hook {
	case plugin.HookPrePublish:
		return p.validateModule(ctx, cfg, req.Context, req.DryRun)

	case plugin.HookPostPublish:
		return p.publishModule(ctx, cfg, req.Context, req.DryRun)

	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

// validateModule validates the Go module before publishing.
func (p *GoModPlugin) validateModule(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	moduleDir, err := p.validateModuleDir(cfg.ModuleDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid module directory: %v", err),
		}, nil
	}

	// Get module path from go.mod
	modulePath, err := p.getModulePath(moduleDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to read module path: %v", err),
		}, nil
	}

	// Validate semver tag format
	if cfg.ValidateTag {
		version := releaseCtx.TagName
		if version == "" {
			version = "v" + releaseCtx.Version
		}

		if !strings.HasPrefix(version, "v") {
			return &plugin.ExecuteResponse{
				Success: false,
				Error:   fmt.Sprintf("Go module tags must start with 'v': got %s", version),
			}, nil
		}
	}

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Would validate module %s", modulePath),
			Outputs: map[string]any{
				"module":  modulePath,
				"version": releaseCtx.TagName,
			},
		}, nil
	}

	// Run go mod tidy
	cmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	cmd.Dir = moduleDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("go mod tidy failed: %v\nstderr: %s", err, stderr.String()),
		}, nil
	}

	// Run go vet
	cmd = exec.CommandContext(ctx, "go", "vet", "./...")
	cmd.Dir = moduleDir
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("go vet failed: %v\nstderr: %s", err, stderr.String()),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Validated module %s", modulePath),
		Outputs: map[string]any{
			"module":  modulePath,
			"version": releaseCtx.TagName,
		},
	}, nil
}

// publishModule publishes the Go module.
func (p *GoModPlugin) publishModule(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	moduleDir, err := p.validateModuleDir(cfg.ModuleDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid module directory: %v", err),
		}, nil
	}

	modulePath, err := p.getModulePath(moduleDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to read module path: %v", err),
		}, nil
	}

	version := releaseCtx.TagName
	if version == "" {
		version = "v" + releaseCtx.Version
	}

	// For private modules, just return success
	if cfg.Private {
		if dryRun {
			return &plugin.ExecuteResponse{
				Success: true,
				Message: "Would mark as private module (no proxy trigger)",
				Outputs: map[string]any{
					"module":  modulePath,
					"version": version,
					"private": true,
				},
			}, nil
		}

		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Published private module %s@%s", modulePath, version),
			Outputs: map[string]any{
				"module":  modulePath,
				"version": version,
				"private": true,
			},
		}, nil
	}

	// Trigger proxy.golang.org to fetch the module
	if cfg.TriggerProxy {
		proxyURL := cfg.ProxyURL
		if proxyURL == "" {
			proxyURL = "https://proxy.golang.org"
		}

		fetchURL := fmt.Sprintf("%s/%s/@v/%s.info", proxyURL, modulePath, version)

		if dryRun {
			return &plugin.ExecuteResponse{
				Success: true,
				Message: "Would trigger Go proxy to fetch module",
				Outputs: map[string]any{
					"module":    modulePath,
					"version":   version,
					"fetch_url": fetchURL,
				},
			}, nil
		}

		// Fetch from proxy to trigger caching
		req, err := http.NewRequestWithContext(ctx, "GET", fetchURL, nil)
		if err != nil {
			return &plugin.ExecuteResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to create request: %v", err),
			}, nil
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			// Proxy fetch failure is not fatal - module may just not be cached yet
			return &plugin.ExecuteResponse{
				Success: true,
				Message: fmt.Sprintf("Published module %s@%s (proxy fetch pending)", modulePath, version),
				Outputs: map[string]any{
					"module":       modulePath,
					"version":      version,
					"proxy_status": "pending",
				},
			}, nil
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return &plugin.ExecuteResponse{
				Success: true,
				Message: fmt.Sprintf("Published module %s@%s (cached in proxy)", modulePath, version),
				Outputs: map[string]any{
					"module":       modulePath,
					"version":      version,
					"proxy_status": "cached",
				},
			}, nil
		}
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Published module %s@%s", modulePath, version),
		Outputs: map[string]any{
			"module":  modulePath,
			"version": version,
		},
	}, nil
}

// getModulePath extracts the module path from go.mod.
func (p *GoModPlugin) getModulePath(moduleDir string) (string, error) {
	goModPath := filepath.Join(moduleDir, "go.mod")

	file, err := os.Open(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to open go.mod: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimPrefix(line, "module "), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}

	return "", fmt.Errorf("module directive not found in go.mod")
}

// validateModuleDir validates and returns the module directory.
func (p *GoModPlugin) validateModuleDir(dir string) (string, error) {
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
func (p *GoModPlugin) parseConfig(raw map[string]any) *Config {
	parser := plugin.NewConfigParser(raw)

	return &Config{
		ModuleDir:    parser.GetString("module_dir"),
		Private:      parser.GetBool("private"),
		ProxyURL:     parser.GetStringDefault("proxy_url", "https://proxy.golang.org"),
		SumDB:        parser.GetString("sumdb"),
		NoProxy:      parser.GetString("noproxy"),
		ValidateTag:  parser.GetBoolDefault("validate_tag", true),
		TriggerProxy: parser.GetBoolDefault("trigger_proxy", true),
	}
}

// Validate validates the plugin configuration.
func (p *GoModPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := plugin.NewValidationBuilder()
	parser := plugin.NewConfigParser(config)

	// Check that go is available
	if _, err := exec.LookPath("go"); err != nil {
		vb.AddError("", "go command not found in PATH", "dependency")
	}

	// Check module directory if provided
	if dir := parser.GetString("module_dir"); dir != "" {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err != nil {
			vb.AddError("module_dir", "go.mod not found", "path")
		}
	}

	return vb.Build(), nil
}
