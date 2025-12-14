// Package main implements the Packagist plugin for Relicta.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

// PackagistPlugin implements the Packagist publish plugin.
type PackagistPlugin struct{}

// Config represents the Packagist plugin configuration.
type Config struct {
	// APIToken is the Packagist API token.
	APIToken string `json:"api_token,omitempty"`
	// Username is the Packagist username.
	Username string `json:"username,omitempty"`
	// PackageName is the full package name (vendor/package).
	PackageName string `json:"package_name,omitempty"`
	// ProjectDir is the directory containing composer.json.
	ProjectDir string `json:"project_dir,omitempty"`
	// UpdateVersion updates version in composer.json.
	UpdateVersion bool `json:"update_version"`
	// PackagistURL is the Packagist API URL.
	PackagistURL string `json:"packagist_url,omitempty"`
	// WebhookURL is the webhook URL to trigger updates.
	WebhookURL string `json:"webhook_url,omitempty"`
}

// ComposerJSON represents the composer.json structure.
type ComposerJSON struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// GetInfo returns plugin metadata.
func (p *PackagistPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "packagist",
		Version:     "1.0.0",
		Description: "Publish packages to Packagist (PHP)",
		Author:      "Relicta Team",
		Hooks: []plugin.Hook{
			plugin.HookPrePublish,
			plugin.HookPostPublish,
		},
		ConfigSchema: `{
			"type": "object",
			"properties": {
				"api_token": {"type": "string", "description": "Packagist API token"},
				"username": {"type": "string", "description": "Packagist username"},
				"package_name": {"type": "string", "description": "Package name (vendor/package)"},
				"project_dir": {"type": "string", "description": "Directory containing composer.json"},
				"update_version": {"type": "boolean", "description": "Update version in composer.json", "default": true},
				"packagist_url": {"type": "string", "description": "Packagist API URL", "default": "https://packagist.org"},
				"webhook_url": {"type": "string", "description": "Webhook URL for updates"}
			}
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *PackagistPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
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
		return p.notifyPackagist(ctx, cfg, req.Context, req.DryRun)

	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

// updateVersion updates the version in composer.json.
func (p *PackagistPlugin) updateVersion(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	projectDir, err := p.validateProjectDir(cfg.ProjectDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid project directory: %v", err),
		}, nil
	}

	composerPath := filepath.Join(projectDir, "composer.json")

	data, err := os.ReadFile(composerPath)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to read composer.json: %v", err),
		}, nil
	}

	var composer map[string]any
	if err := json.Unmarshal(data, &composer); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to parse composer.json: %v", err),
		}, nil
	}

	oldVersion := ""
	if v, ok := composer["version"].(string); ok {
		oldVersion = v
	}

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Would update composer.json version from %s to %s", oldVersion, releaseCtx.Version),
			Outputs: map[string]any{
				"old_version": oldVersion,
				"new_version": releaseCtx.Version,
			},
		}, nil
	}

	composer["version"] = releaseCtx.Version

	newData, err := json.MarshalIndent(composer, "", "    ")
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to marshal composer.json: %v", err),
		}, nil
	}

	if err := os.WriteFile(composerPath, newData, 0644); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to write composer.json: %v", err),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Updated composer.json version to %s", releaseCtx.Version),
		Outputs: map[string]any{
			"old_version": oldVersion,
			"new_version": releaseCtx.Version,
		},
	}, nil
}

// notifyPackagist triggers Packagist to update the package.
func (p *PackagistPlugin) notifyPackagist(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
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

	// Get package name from composer.json if not provided
	packageName := cfg.PackageName
	if packageName == "" {
		composerPath := filepath.Join(projectDir, "composer.json")
		data, err := os.ReadFile(composerPath)
		if err != nil {
			return &plugin.ExecuteResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to read composer.json: %v", err),
			}, nil
		}

		var composer ComposerJSON
		if err := json.Unmarshal(data, &composer); err != nil {
			return &plugin.ExecuteResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to parse composer.json: %v", err),
			}, nil
		}

		packageName = composer.Name
	}

	if packageName == "" {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   "package name not found in composer.json and not provided in config",
		}, nil
	}

	packagistURL := cfg.PackagistURL
	if packagistURL == "" {
		packagistURL = "https://packagist.org"
	}

	// Build update URL
	updateURL := cfg.WebhookURL
	if updateURL == "" {
		updateURL = fmt.Sprintf("%s/api/update-package", packagistURL)
	}

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Would notify Packagist to update package",
			Outputs: map[string]any{
				"package_name": packageName,
				"version":      releaseCtx.Version,
				"update_url":   updateURL,
			},
		}, nil
	}

	// Prepare request body
	reqBody := map[string]string{
		"repository": fmt.Sprintf("https://github.com/%s", packageName),
	}

	if cfg.APIToken != "" && cfg.Username != "" {
		reqBody["username"] = cfg.Username
		reqBody["apiToken"] = cfg.APIToken
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to marshal request: %v", err),
		}, nil
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", updateURL, bytes.NewReader(jsonBody))
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to create request: %v", err),
		}, nil
	}

	req.Header.Set("Content-Type", "application/json")

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to notify Packagist: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("Packagist returned status %d", resp.StatusCode),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Notified Packagist to update %s version %s", packageName, releaseCtx.Version),
		Outputs: map[string]any{
			"package_name": packageName,
			"version":      releaseCtx.Version,
			"status":       resp.StatusCode,
		},
	}, nil
}

// validateConfig validates the plugin configuration.
func (p *PackagistPlugin) validateConfig(cfg *Config) error {
	if cfg.PackagistURL != "" {
		parsedURL, err := url.Parse(cfg.PackagistURL)
		if err != nil {
			return fmt.Errorf("invalid Packagist URL: %w", err)
		}
		if parsedURL.Scheme != "https" {
			hostname := parsedURL.Hostname()
			if parsedURL.Scheme == "http" && (hostname == "localhost" || strings.HasPrefix(hostname, "127.")) {
				// Allow http for local testing
			} else {
				return fmt.Errorf("packagist URL must use HTTPS")
			}
		}
	}

	return nil
}

// validateProjectDir validates and returns the project directory.
func (p *PackagistPlugin) validateProjectDir(dir string) (string, error) {
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
func (p *PackagistPlugin) parseConfig(raw map[string]any) *Config {
	parser := plugin.NewConfigParser(raw)

	return &Config{
		APIToken:      parser.GetString("api_token", "PACKAGIST_TOKEN"),
		Username:      parser.GetString("username", "PACKAGIST_USERNAME"),
		PackageName:   parser.GetString("package_name"),
		ProjectDir:    parser.GetString("project_dir"),
		UpdateVersion: parser.GetBoolDefault("update_version", true),
		PackagistURL:  parser.GetStringDefault("packagist_url", "https://packagist.org"),
		WebhookURL:    parser.GetString("webhook_url"),
	}
}

// Validate validates the plugin configuration.
func (p *PackagistPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := plugin.NewValidationBuilder()
	parser := plugin.NewConfigParser(config)

	// Check project directory if provided
	if dir := parser.GetString("project_dir"); dir != "" {
		composerPath := filepath.Join(dir, "composer.json")
		if _, err := os.Stat(composerPath); err != nil {
			vb.AddError("project_dir", "composer.json not found", "path")
		}
	}

	// Check API credentials
	apiToken := parser.GetString("api_token", "PACKAGIST_TOKEN")
	username := parser.GetString("username", "PACKAGIST_USERNAME")

	if apiToken == "" || username == "" {
		vb.AddWarning("credentials", "API token or username not configured; update notification may fail")
	}

	return vb.Build(), nil
}
