// Package main implements the PyPI plugin for ReleasePilot.
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

	"github.com/felixgeelhaar/release-pilot/pkg/plugin"
)

// Security validation patterns
var (
	// packageNamePattern validates Python package names (PEP 503)
	packageNamePattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$`)
)

// PyPIPlugin implements the PyPI publish plugin.
type PyPIPlugin struct{}

// Config represents the PyPI plugin configuration.
type Config struct {
	// Repository is the PyPI repository URL (default: pypi.org).
	Repository string `json:"repository,omitempty"`
	// RepositoryURL is an alias for Repository.
	RepositoryURL string `json:"repository_url,omitempty"`
	// Username for PyPI authentication.
	Username string `json:"username,omitempty"`
	// Password for PyPI authentication.
	Password string `json:"password,omitempty"`
	// Token is the API token for PyPI authentication.
	Token string `json:"token,omitempty"`
	// DistDir is the directory containing distribution files.
	DistDir string `json:"dist_dir,omitempty"`
	// SkipExisting skips uploading if version already exists.
	SkipExisting bool `json:"skip_existing"`
	// UseTwine uses twine for uploading (recommended).
	UseTwine bool `json:"use_twine"`
	// BuildCommand is the command to build distributions.
	BuildCommand string `json:"build_command,omitempty"`
	// UpdateVersion updates version in pyproject.toml/setup.py.
	UpdateVersion bool `json:"update_version"`
	// PackageDir is the directory containing the Python package.
	PackageDir string `json:"package_dir,omitempty"`
}

// GetInfo returns plugin metadata.
func (p *PyPIPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "pypi",
		Version:     "1.0.0",
		Description: "Publish packages to PyPI (Python Package Index)",
		Author:      "ReleasePilot Team",
		Hooks: []plugin.Hook{
			plugin.HookPrePublish,
			plugin.HookPostPublish,
		},
		ConfigSchema: `{
			"type": "object",
			"properties": {
				"repository": {"type": "string", "description": "PyPI repository name (pypi or testpypi)"},
				"repository_url": {"type": "string", "description": "Custom PyPI repository URL"},
				"username": {"type": "string", "description": "PyPI username (use __token__ for API tokens)"},
				"password": {"type": "string", "description": "PyPI password"},
				"token": {"type": "string", "description": "PyPI API token"},
				"dist_dir": {"type": "string", "description": "Directory containing dist files", "default": "dist"},
				"skip_existing": {"type": "boolean", "description": "Skip if version exists", "default": false},
				"use_twine": {"type": "boolean", "description": "Use twine for upload", "default": true},
				"build_command": {"type": "string", "description": "Command to build distributions"},
				"update_version": {"type": "boolean", "description": "Update version in pyproject.toml", "default": true},
				"package_dir": {"type": "string", "description": "Package directory"}
			}
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *PyPIPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	cfg := p.parseConfig(req.Config)

	switch req.Hook {
	case plugin.HookPrePublish:
		return p.preparePackage(ctx, cfg, req.Context, req.DryRun)

	case plugin.HookPostPublish:
		return p.publishPackage(ctx, cfg, req.Context, req.DryRun)

	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

// preparePackage prepares the package for publishing (update version, build).
func (p *PyPIPlugin) preparePackage(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	packageDir, err := p.validatePackageDir(cfg.PackageDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid package directory: %v", err),
		}, nil
	}

	outputs := make(map[string]any)

	// Update version if enabled
	if cfg.UpdateVersion {
		versionUpdated, err := p.updateVersion(packageDir, releaseCtx.Version, dryRun)
		if err != nil {
			return &plugin.ExecuteResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to update version: %v", err),
			}, nil
		}
		outputs["version_updated"] = versionUpdated
	}

	// Build distributions if build command provided
	if cfg.BuildCommand != "" {
		// Validate build command for dangerous patterns (defense in depth)
		// Note: BuildCommand comes from config file, which is trusted, but validate defensively
		if err := p.validateBuildCommand(cfg.BuildCommand); err != nil {
			return &plugin.ExecuteResponse{
				Success: false,
				Error:   fmt.Sprintf("invalid build command: %v", err),
			}, nil
		}

		if dryRun {
			outputs["build_command"] = cfg.BuildCommand
			return &plugin.ExecuteResponse{
				Success: true,
				Message: fmt.Sprintf("Would update version to %s and run: %s", releaseCtx.Version, cfg.BuildCommand),
				Outputs: outputs,
			}, nil
		}

		cmd := exec.CommandContext(ctx, "sh", "-c", cfg.BuildCommand)
		cmd.Dir = packageDir

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return &plugin.ExecuteResponse{
				Success: false,
				Error:   fmt.Sprintf("build failed: %v\nstderr: %s", err, stderr.String()),
			}, nil
		}
		outputs["build_output"] = stdout.String()
	}

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Would prepare package version %s", releaseCtx.Version),
			Outputs: outputs,
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Prepared package version %s", releaseCtx.Version),
		Outputs: outputs,
	}, nil
}

// updateVersion updates the version in pyproject.toml or setup.py.
func (p *PyPIPlugin) updateVersion(packageDir, version string, dryRun bool) (string, error) {
	// Try pyproject.toml first
	pyprojectPath := filepath.Join(packageDir, "pyproject.toml")
	if _, err := os.Stat(pyprojectPath); err == nil {
		return p.updatePyprojectVersion(pyprojectPath, version, dryRun)
	}

	// Fall back to setup.py
	setupPath := filepath.Join(packageDir, "setup.py")
	if _, err := os.Stat(setupPath); err == nil {
		return p.updateSetupPyVersion(setupPath, version, dryRun)
	}

	return "", fmt.Errorf("no pyproject.toml or setup.py found in %s", packageDir)
}

// updatePyprojectVersion updates version in pyproject.toml.
func (p *PyPIPlugin) updatePyprojectVersion(path, version string, dryRun bool) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read pyproject.toml: %w", err)
	}

	content := string(data)

	// Match version in [project] section
	versionPattern := regexp.MustCompile(`(?m)^version\s*=\s*["']([^"']+)["']`)
	match := versionPattern.FindStringSubmatch(content)

	oldVersion := ""
	if len(match) > 1 {
		oldVersion = match[1]
	}

	if dryRun {
		return fmt.Sprintf("%s -> %s", oldVersion, version), nil
	}

	// Replace version
	newContent := versionPattern.ReplaceAllString(content, fmt.Sprintf(`version = "%s"`, version))

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write pyproject.toml: %w", err)
	}

	return fmt.Sprintf("%s -> %s", oldVersion, version), nil
}

// updateSetupPyVersion updates version in setup.py.
func (p *PyPIPlugin) updateSetupPyVersion(path, version string, dryRun bool) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read setup.py: %w", err)
	}

	content := string(data)

	// Match version argument
	versionPattern := regexp.MustCompile(`version\s*=\s*["']([^"']+)["']`)
	match := versionPattern.FindStringSubmatch(content)

	oldVersion := ""
	if len(match) > 1 {
		oldVersion = match[1]
	}

	if dryRun {
		return fmt.Sprintf("%s -> %s", oldVersion, version), nil
	}

	// Replace version
	newContent := versionPattern.ReplaceAllString(content, fmt.Sprintf(`version="%s"`, version))

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write setup.py: %w", err)
	}

	return fmt.Sprintf("%s -> %s", oldVersion, version), nil
}

// publishPackage publishes the package to PyPI.
func (p *PyPIPlugin) publishPackage(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	// Validate configuration
	if err := p.validateConfig(cfg); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("configuration validation failed: %v", err),
		}, nil
	}

	packageDir, err := p.validatePackageDir(cfg.PackageDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid package directory: %v", err),
		}, nil
	}

	distDir := cfg.DistDir
	if distDir == "" {
		distDir = "dist"
	}
	distPath := filepath.Join(packageDir, distDir)

	// Check dist directory exists
	if _, err := os.Stat(distPath); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("distribution directory not found: %s", distPath),
		}, nil
	}

	// Build upload command
	var args []string
	var cmd *exec.Cmd

	if cfg.UseTwine {
		args = []string{"upload"}

		// Add repository options
		if cfg.Repository != "" {
			args = append(args, "--repository", cfg.Repository)
		} else if cfg.RepositoryURL != "" {
			args = append(args, "--repository-url", cfg.RepositoryURL)
		}

		// Add authentication
		if cfg.Token != "" {
			args = append(args, "--username", "__token__")
			args = append(args, "--password", cfg.Token)
		} else {
			if cfg.Username != "" {
				args = append(args, "--username", cfg.Username)
			}
			if cfg.Password != "" {
				args = append(args, "--password", cfg.Password)
			}
		}

		if cfg.SkipExisting {
			args = append(args, "--skip-existing")
		}

		// Add dist files
		args = append(args, filepath.Join(distDir, "*"))

		cmd = exec.CommandContext(ctx, "twine", args...)
	} else {
		// Use pip/python -m twine as fallback
		args = []string{"-m", "twine", "upload"}

		if cfg.Repository != "" {
			args = append(args, "--repository", cfg.Repository)
		} else if cfg.RepositoryURL != "" {
			args = append(args, "--repository-url", cfg.RepositoryURL)
		}

		if cfg.Token != "" {
			args = append(args, "--username", "__token__")
			args = append(args, "--password", cfg.Token)
		} else {
			if cfg.Username != "" {
				args = append(args, "--username", cfg.Username)
			}
			if cfg.Password != "" {
				args = append(args, "--password", cfg.Password)
			}
		}

		if cfg.SkipExisting {
			args = append(args, "--skip-existing")
		}

		args = append(args, filepath.Join(distDir, "*"))

		cmd = exec.CommandContext(ctx, "python", args...)
	}

	cmd.Dir = packageDir

	// Build log-safe command string (redact credentials)
	logArgs := make([]string, len(args))
	copy(logArgs, args)
	for i := range logArgs {
		if i > 0 && (logArgs[i-1] == "--password" || logArgs[i-1] == "--token") {
			logArgs[i] = "[REDACTED]"
		}
	}

	cmdName := "twine"
	if !cfg.UseTwine {
		cmdName = "python"
	}
	cmdStr := fmt.Sprintf("%s %s", cmdName, strings.Join(logArgs, " "))

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Would publish to PyPI",
			Outputs: map[string]any{
				"command":     cmdStr,
				"version":     releaseCtx.Version,
				"dist_dir":    distPath,
				"repository":  cfg.Repository,
				"package_dir": packageDir,
			},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set environment for non-interactive mode
	cmd.Env = append(os.Environ(), "TWINE_NON_INTERACTIVE=1")

	if err := cmd.Run(); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("PyPI upload failed: %v\nstderr: %s", err, stderr.String()),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Published version %s to PyPI", releaseCtx.Version),
		Outputs: map[string]any{
			"version":    releaseCtx.Version,
			"repository": cfg.Repository,
			"stdout":     stdout.String(),
		},
	}, nil
}

// validateConfig validates the plugin configuration.
func (p *PyPIPlugin) validateConfig(cfg *Config) error {
	// Validate repository URL if provided
	if cfg.RepositoryURL != "" {
		parsedURL, err := url.Parse(cfg.RepositoryURL)
		if err != nil {
			return fmt.Errorf("invalid repository URL: %w", err)
		}
		if parsedURL.Scheme != "https" {
			hostname := parsedURL.Hostname()
			if parsedURL.Scheme == "http" && (hostname == "localhost" || strings.HasPrefix(hostname, "127.")) {
				// Allow http for local testing
			} else {
				return fmt.Errorf("repository URL must use HTTPS")
			}
		}
	}

	// Validate repository name if provided
	if cfg.Repository != "" {
		validRepos := map[string]bool{"pypi": true, "testpypi": true}
		if !validRepos[cfg.Repository] && !packageNamePattern.MatchString(cfg.Repository) {
			return fmt.Errorf("invalid repository name: %s", cfg.Repository)
		}
	}

	return nil
}

// validatePackageDir validates and returns the package directory.
func (p *PyPIPlugin) validatePackageDir(dir string) (string, error) {
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

// validateBuildCommand validates build command for dangerous patterns (defense in depth).
// Note: BuildCommand comes from config file, which is trusted, but validate defensively.
func (p *PyPIPlugin) validateBuildCommand(cmd string) error {
	if cmd == "" {
		return nil
	}

	// Check for dangerous shell metacharacters that could enable command injection
	// Even though config is trusted, defense in depth is important
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "<", ">", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(cmd, char) {
			return fmt.Errorf("build command contains dangerous character: %q", char)
		}
	}

	// Check for path traversal attempts
	if strings.Contains(cmd, "..") {
		return fmt.Errorf("build command contains path traversal pattern")
	}

	// Check for suspicious commands that shouldn't be in a build command
	suspiciousPatterns := []string{
		"rm ", "rmdir", "del ",
		"curl ", "wget ",
		"nc ", "netcat",
		"eval ", "exec ",
		"/dev/", "/proc/",
	}
	lowerCmd := strings.ToLower(cmd)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerCmd, pattern) {
			return fmt.Errorf("build command contains suspicious pattern: %q", pattern)
		}
	}

	// Validate that command starts with a reasonable build tool
	// This is a whitelist approach - only allow known-safe build commands
	allowedPrefixes := []string{
		"python -m build",
		"python3 -m build",
		"poetry build",
		"pip install",
		"pip3 install",
		"python setup.py",
		"python3 setup.py",
		"flit build",
		"hatch build",
		"pdm build",
	}

	hasValidPrefix := false
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(cmd, prefix) {
			hasValidPrefix = true
			break
		}
	}

	if !hasValidPrefix {
		return fmt.Errorf("build command must start with a known build tool (python -m build, poetry build, etc.)")
	}

	return nil
}

// parseConfig parses the plugin configuration.
func (p *PyPIPlugin) parseConfig(raw map[string]any) *Config {
	parser := plugin.NewConfigParser(raw)

	return &Config{
		Repository:    parser.GetString("repository"),
		RepositoryURL: parser.GetString("repository_url"),
		Username:      parser.GetString("username", "PYPI_USERNAME", "TWINE_USERNAME"),
		Password:      parser.GetString("password", "PYPI_PASSWORD", "TWINE_PASSWORD"),
		Token:         parser.GetString("token", "PYPI_TOKEN", "PYPI_API_TOKEN"),
		DistDir:       parser.GetStringDefault("dist_dir", "dist"),
		SkipExisting:  parser.GetBool("skip_existing"),
		UseTwine:      parser.GetBoolDefault("use_twine", true),
		BuildCommand:  parser.GetString("build_command"),
		UpdateVersion: parser.GetBoolDefault("update_version", true),
		PackageDir:    parser.GetString("package_dir"),
	}
}

// Validate validates the plugin configuration.
func (p *PyPIPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := plugin.NewValidationBuilder()
	parser := plugin.NewConfigParser(config)

	// Check that twine is available
	if _, err := exec.LookPath("twine"); err != nil {
		if _, err := exec.LookPath("python"); err != nil {
			vb.AddError("", "neither twine nor python found in PATH", "dependency")
		}
	}

	// Check package directory if provided
	if dir := parser.GetString("package_dir"); dir != "" {
		// Check for pyproject.toml or setup.py
		pyprojectPath := filepath.Join(dir, "pyproject.toml")
		setupPath := filepath.Join(dir, "setup.py")

		hasPyproject := false
		hasSetup := false

		if _, err := os.Stat(pyprojectPath); err == nil {
			hasPyproject = true
		}
		if _, err := os.Stat(setupPath); err == nil {
			hasSetup = true
		}

		if !hasPyproject && !hasSetup {
			vb.AddError("package_dir", "no pyproject.toml or setup.py found", "path")
		}
	}

	// Validate authentication
	token := parser.GetString("token", "PYPI_TOKEN", "PYPI_API_TOKEN")
	username := parser.GetString("username", "PYPI_USERNAME", "TWINE_USERNAME")
	password := parser.GetString("password", "PYPI_PASSWORD", "TWINE_PASSWORD")

	if token == "" && (username == "" || password == "") {
		vb.AddWarning("authentication", "no credentials configured; using .pypirc or environment")
	}

	return vb.Build(), nil
}
