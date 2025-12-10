// Package main implements the npm plugin for ReleasePilot.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/felixgeelhaar/release-pilot/pkg/plugin"
)

// Security validation patterns
var (
	// tagPattern validates npm dist-tags (alphanumeric, hyphens, underscores, dots)
	tagPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	// otpPattern validates OTP codes (6-8 digits)
	otpPattern = regexp.MustCompile(`^\d{6,8}$`)
	// allowedAccessLevels are the only valid npm access levels
	allowedAccessLevels = map[string]bool{"public": true, "restricted": true, "": true}
)

// NpmPlugin implements the npm publish plugin.
type NpmPlugin struct{}

// Config represents the npm plugin configuration.
type Config struct {
	// Registry is the npm registry URL.
	Registry string `json:"registry,omitempty"`
	// Tag is the npm dist-tag to use.
	Tag string `json:"tag,omitempty"`
	// Access is the package access level (public, restricted).
	Access string `json:"access,omitempty"`
	// OTP is the one-time password for 2FA.
	OTP string `json:"otp,omitempty"`
	// DryRun performs a dry-run publish.
	DryRun bool `json:"dry_run"`
	// PackageDir is the directory containing package.json.
	PackageDir string `json:"package_dir,omitempty"`
	// UpdateVersion updates package.json version before publishing.
	UpdateVersion bool `json:"update_version"`
}

// PackageJSON represents a package.json file.
type PackageJSON struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Private bool   `json:"private"`
}

// GetInfo returns plugin metadata.
func (p *NpmPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "npm",
		Version:     "1.0.0",
		Description: "Publish packages to npm registry",
		Author:      "ReleasePilot Team",
		Hooks: []plugin.Hook{
			plugin.HookPrePublish,
			plugin.HookPostPublish,
		},
		ConfigSchema: `{
			"type": "object",
			"properties": {
				"registry": {"type": "string", "description": "npm registry URL"},
				"tag": {"type": "string", "description": "dist-tag for the package", "default": "latest"},
				"access": {"type": "string", "enum": ["public", "restricted"], "description": "Package access level"},
				"otp": {"type": "string", "description": "OTP for 2FA"},
				"dry_run": {"type": "boolean", "description": "Perform dry-run", "default": false},
				"package_dir": {"type": "string", "description": "Directory containing package.json"},
				"update_version": {"type": "boolean", "description": "Update package.json version", "default": true}
			}
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *NpmPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	cfg := p.parseConfig(req.Config)

	switch req.Hook {
	case plugin.HookPrePublish:
		if cfg.UpdateVersion {
			return p.updatePackageVersion(ctx, cfg, req.Context, req.DryRun)
		}
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Version update disabled",
		}, nil

	case plugin.HookPostPublish:
		return p.publishPackage(ctx, cfg, req.Context, req.DryRun || cfg.DryRun)

	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

// updatePackageVersion updates the version in package.json.
func (p *NpmPlugin) updatePackageVersion(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	// Validate and sanitize package directory (security check)
	packageDir, err := validatePackageDir(cfg.PackageDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid package directory: %v", err),
		}, nil
	}

	packagePath := filepath.Join(packageDir, "package.json")

	// Read existing package.json
	data, err := os.ReadFile(packagePath)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to read package.json: %v", err),
		}, nil
	}

	var pkg map[string]any
	if err := json.Unmarshal(data, &pkg); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to parse package.json: %v", err),
		}, nil
	}

	oldVersion := pkg["version"]

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Would update package.json version from %v to %s", oldVersion, releaseCtx.Version),
		}, nil
	}

	// Update version
	pkg["version"] = releaseCtx.Version

	// Write back
	newData, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to marshal package.json: %v", err),
		}, nil
	}

	if err := os.WriteFile(packagePath, newData, 0644); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to write package.json: %v", err),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Updated package.json version to %s", releaseCtx.Version),
		Outputs: map[string]any{
			"old_version": oldVersion,
			"new_version": releaseCtx.Version,
		},
	}, nil
}

// validateRegistry validates and sanitizes npm registry URL.
func validateRegistry(registry string) error {
	if registry == "" {
		return nil
	}

	parsedURL, err := url.Parse(registry)
	if err != nil {
		return fmt.Errorf("invalid registry URL: %w", err)
	}

	// Only allow https URLs (or http for localhost during development)
	if parsedURL.Scheme != "https" {
		if parsedURL.Scheme == "http" && (parsedURL.Host == "localhost" || strings.HasPrefix(parsedURL.Host, "127.0.0.1") || strings.HasPrefix(parsedURL.Host, "localhost:")) {
			// Allow http for local development registries
		} else {
			return fmt.Errorf("registry must use HTTPS (got %s)", parsedURL.Scheme)
		}
	}

	// Prevent injection via URL components
	if strings.ContainsAny(registry, "\n\r\t") {
		return fmt.Errorf("registry URL contains invalid characters")
	}

	return nil
}

// validateTag validates npm dist-tag format.
func validateTag(tag string) error {
	if tag == "" {
		return nil
	}
	if len(tag) > 128 {
		return fmt.Errorf("tag too long (max 128 characters)")
	}
	if !tagPattern.MatchString(tag) {
		return fmt.Errorf("tag contains invalid characters (must be alphanumeric, hyphens, underscores, dots)")
	}
	return nil
}

// validateAccess validates npm access level.
func validateAccess(access string) error {
	if !allowedAccessLevels[access] {
		return fmt.Errorf("access must be 'public' or 'restricted'")
	}
	return nil
}

// validateOTP validates one-time password format.
func validateOTP(otp string) error {
	if otp == "" {
		return nil
	}
	if !otpPattern.MatchString(otp) {
		return fmt.Errorf("OTP must be 6-8 digits")
	}
	return nil
}

// validatePackageDir validates and sanitizes package directory path.
// It ensures the path doesn't escape the current working directory.
func validatePackageDir(dir string) (string, error) {
	if dir == "" {
		return ".", nil
	}

	// Get current working directory as the security boundary
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// Clean the path to normalize it
	cleanPath := filepath.Clean(dir)

	// Prevent obvious path traversal attempts
	if strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, "/../") {
		return "", fmt.Errorf("path traversal not allowed in package_dir")
	}

	// Resolve to absolute path
	var absPath string
	if filepath.IsAbs(cleanPath) {
		absPath = cleanPath
	} else {
		absPath = filepath.Join(cwd, cleanPath)
	}

	// Ensure the resolved path doesn't escape the working directory
	// Use EvalSymlinks to resolve any symlinks that might bypass the check
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If EvalSymlinks fails, the path might not exist yet or have permission issues
		// Fall back to checking the absolute path
		resolvedPath = absPath
	}

	resolvedCwd, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		resolvedCwd = cwd
	}

	// Verify the resolved path is within the working directory
	// Add trailing separator to prevent partial path matches (e.g., /home/user2 vs /home/user)
	if !strings.HasPrefix(resolvedPath+string(filepath.Separator), resolvedCwd+string(filepath.Separator)) &&
		resolvedPath != resolvedCwd {
		return "", fmt.Errorf("package_dir must be within the current working directory")
	}

	// Verify the directory exists
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("package directory not accessible: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("package_dir is not a directory")
	}

	return resolvedPath, nil
}

// validateConfig performs security validation on all config fields.
func (p *NpmPlugin) validateConfig(cfg *Config) error {
	if err := validateRegistry(cfg.Registry); err != nil {
		return fmt.Errorf("registry validation failed: %w", err)
	}
	if err := validateTag(cfg.Tag); err != nil {
		return fmt.Errorf("tag validation failed: %w", err)
	}
	if err := validateAccess(cfg.Access); err != nil {
		return fmt.Errorf("access validation failed: %w", err)
	}
	if err := validateOTP(cfg.OTP); err != nil {
		return fmt.Errorf("OTP validation failed: %w", err)
	}
	return nil
}

// publishPackage publishes the package to npm.
func (p *NpmPlugin) publishPackage(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	// Validate all config fields first (security check)
	if err := p.validateConfig(cfg); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("configuration validation failed: %v", err),
		}, nil
	}

	// Validate and sanitize package directory
	packageDir, err := validatePackageDir(cfg.PackageDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid package directory: %v", err),
		}, nil
	}

	// Check if package.json exists
	packagePath := filepath.Join(packageDir, "package.json")
	data, err := os.ReadFile(packagePath)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to read package.json: %v", err),
		}, nil
	}

	var pkg PackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to parse package.json: %v", err),
		}, nil
	}

	if pkg.Private {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Package is private, skipping npm publish",
		}, nil
	}

	// Build npm publish command with validated arguments
	args := []string{"publish"}

	if cfg.Registry != "" {
		args = append(args, "--registry", cfg.Registry)
	}

	if cfg.Tag != "" {
		args = append(args, "--tag", cfg.Tag)
	}

	if cfg.Access != "" {
		args = append(args, "--access", cfg.Access)
	}

	if cfg.OTP != "" {
		args = append(args, "--otp", cfg.OTP)
	}

	if dryRun {
		args = append(args, "--dry-run")
	}

	// Log command (redact OTP in logs)
	logArgs := make([]string, len(args))
	copy(logArgs, args)
	for i := range logArgs {
		if i > 0 && logArgs[i-1] == "--otp" {
			logArgs[i] = "[REDACTED]"
		}
	}
	cmdStr := fmt.Sprintf("npm %s", strings.Join(logArgs, " "))

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Would run: %s (in %s)", cmdStr, packageDir),
			Outputs: map[string]any{
				"package":     pkg.Name,
				"version":     releaseCtx.Version,
				"command":     cmdStr,
				"package_dir": packageDir,
			},
		}, nil
	}

	// Execute npm publish
	cmd := exec.CommandContext(ctx, "npm", args...)
	cmd.Dir = packageDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("npm publish failed: %v\nstderr: %s", err, stderr.String()),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Published %s@%s to npm", pkg.Name, releaseCtx.Version),
		Outputs: map[string]any{
			"package":  pkg.Name,
			"version":  releaseCtx.Version,
			"registry": cfg.Registry,
			"tag":      cfg.Tag,
			"stdout":   stdout.String(),
		},
	}, nil
}

// parseConfig parses the plugin configuration using the shared ConfigParser.
func (p *NpmPlugin) parseConfig(raw map[string]any) *Config {
	parser := plugin.NewConfigParser(raw)

	tag := parser.GetString("tag")
	if tag == "" {
		tag = "latest"
	}

	return &Config{
		Registry:      parser.GetString("registry"),
		Tag:           tag,
		Access:        parser.GetString("access"),
		OTP:           parser.GetString("otp"),
		DryRun:        parser.GetBool("dry_run"),
		PackageDir:    parser.GetString("package_dir"),
		UpdateVersion: parser.GetBoolDefault("update_version", true),
	}
}

// Validate validates the plugin configuration using the shared ValidationBuilder.
func (p *NpmPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := plugin.NewValidationBuilder()

	// Check access level if provided
	vb.ValidateEnum(config, "access", []string{"public", "restricted"})

	// Verify npm is available
	if _, err := exec.LookPath("npm"); err != nil {
		vb.AddError("", "npm command not found in PATH", "dependency")
	}

	// Check package_dir exists if provided
	parser := plugin.NewConfigParser(config)
	if dir := parser.GetString("package_dir"); dir != "" {
		packagePath := filepath.Join(dir, "package.json")
		if _, err := os.Stat(packagePath); err != nil {
			vb.AddError("package_dir", fmt.Sprintf("package.json not found at %s", packagePath), "path")
		}
	}

	return vb.Build(), nil
}
