// Package main implements the Maven Central plugin for Relicta.
package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

// MavenPlugin implements the Maven Central publish plugin.
type MavenPlugin struct{}

// Config represents the Maven plugin configuration.
type Config struct {
	// ServerID is the Maven server ID from settings.xml.
	ServerID string `json:"server_id,omitempty"`
	// RepositoryURL is the Maven repository URL.
	RepositoryURL string `json:"repository_url,omitempty"`
	// GPGKeyID is the GPG key ID for signing.
	GPGKeyID string `json:"gpg_key_id,omitempty"`
	// GPGPassphrase is the GPG passphrase.
	GPGPassphrase string `json:"gpg_passphrase,omitempty"`
	// SkipTests skips test execution during publish.
	SkipTests bool `json:"skip_tests"`
	// SkipGPG skips GPG signing.
	SkipGPG bool `json:"skip_gpg"`
	// Profiles are Maven profiles to activate.
	Profiles []string `json:"profiles,omitempty"`
	// Goals are Maven goals to execute.
	Goals []string `json:"goals,omitempty"`
	// ProjectDir is the directory containing pom.xml.
	ProjectDir string `json:"project_dir,omitempty"`
	// UpdateVersion updates version in pom.xml.
	UpdateVersion bool `json:"update_version"`
	// UseGradle uses Gradle instead of Maven.
	UseGradle bool `json:"use_gradle"`
	// MavenCommand is the Maven command to use (mvn or mvnw).
	MavenCommand string `json:"maven_command,omitempty"`
}

// POMProject represents the minimal pom.xml structure needed for version updates.
type POMProject struct {
	XMLName xml.Name `xml:"project"`
	Version string   `xml:"version"`
}

// GetInfo returns plugin metadata.
func (p *MavenPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:        "maven",
		Version:     "1.0.0",
		Description: "Publish packages to Maven Central",
		Author:      "Relicta Team",
		Hooks: []plugin.Hook{
			plugin.HookPrePublish,
			plugin.HookPostPublish,
		},
		ConfigSchema: `{
			"type": "object",
			"properties": {
				"server_id": {"type": "string", "description": "Maven server ID from settings.xml"},
				"repository_url": {"type": "string", "description": "Maven repository URL"},
				"gpg_key_id": {"type": "string", "description": "GPG key ID for signing"},
				"gpg_passphrase": {"type": "string", "description": "GPG passphrase"},
				"skip_tests": {"type": "boolean", "description": "Skip tests during publish", "default": true},
				"skip_gpg": {"type": "boolean", "description": "Skip GPG signing", "default": false},
				"profiles": {"type": "array", "items": {"type": "string"}, "description": "Maven profiles to activate"},
				"goals": {"type": "array", "items": {"type": "string"}, "description": "Maven goals to execute"},
				"project_dir": {"type": "string", "description": "Directory containing pom.xml"},
				"update_version": {"type": "boolean", "description": "Update version in pom.xml", "default": true},
				"use_gradle": {"type": "boolean", "description": "Use Gradle instead of Maven", "default": false},
				"maven_command": {"type": "string", "description": "Maven command (mvn or mvnw)", "default": "mvn"}
			}
		}`,
	}
}

// Execute runs the plugin for a given hook.
func (p *MavenPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
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
		return p.publishArtifact(ctx, cfg, req.Context, req.DryRun)

	default:
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Hook %s not handled", req.Hook),
		}, nil
	}
}

// updateVersion updates the version in pom.xml or build.gradle.
func (p *MavenPlugin) updateVersion(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	projectDir, err := p.validateProjectDir(cfg.ProjectDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid project directory: %v", err),
		}, nil
	}

	if cfg.UseGradle {
		return p.updateGradleVersion(projectDir, releaseCtx.Version, dryRun)
	}

	return p.updatePOMVersion(ctx, cfg, projectDir, releaseCtx.Version, dryRun)
}

// updatePOMVersion updates the version in pom.xml using Maven versions plugin.
func (p *MavenPlugin) updatePOMVersion(ctx context.Context, cfg *Config, projectDir, version string, dryRun bool) (*plugin.ExecuteResponse, error) {
	pomPath := filepath.Join(projectDir, "pom.xml")

	// Read current version from pom.xml
	data, err := os.ReadFile(pomPath)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to read pom.xml: %v", err),
		}, nil
	}

	// Extract current version using regex (more reliable than full XML parsing)
	versionPattern := regexp.MustCompile(`<version>([^<]+)</version>`)
	match := versionPattern.FindStringSubmatch(string(data))

	oldVersion := ""
	if len(match) > 1 {
		oldVersion = match[1]
	}

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Would update pom.xml version from %s to %s", oldVersion, version),
			Outputs: map[string]any{
				"old_version": oldVersion,
				"new_version": version,
			},
		}, nil
	}

	// Use Maven versions plugin to update version
	mvnCmd := cfg.MavenCommand
	if mvnCmd == "" {
		mvnCmd = "mvn"
	}

	args := []string{
		"versions:set",
		fmt.Sprintf("-DnewVersion=%s", version),
		"-DgenerateBackupPoms=false",
	}

	cmd := exec.CommandContext(ctx, mvnCmd, args...)
	cmd.Dir = projectDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Fall back to manual update
		return p.manualPOMUpdate(pomPath, version, oldVersion)
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Updated pom.xml version to %s", version),
		Outputs: map[string]any{
			"old_version": oldVersion,
			"new_version": version,
		},
	}, nil
}

// manualPOMUpdate updates pom.xml version using regex replacement.
func (p *MavenPlugin) manualPOMUpdate(pomPath, version, oldVersion string) (*plugin.ExecuteResponse, error) {
	data, err := os.ReadFile(pomPath)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to read pom.xml: %v", err),
		}, nil
	}

	content := string(data)

	// Replace first occurrence of version in project element
	// This is a simplified approach - full XML parsing would be more robust
	versionPattern := regexp.MustCompile(`(<version>)[^<]+(</version>)`)
	newContent := versionPattern.ReplaceAllStringFunc(content, func(match string) string {
		// Only replace the first match
		if oldVersion != "" {
			content = strings.Replace(content, match, fmt.Sprintf("<version>%s</version>", version), 1)
			oldVersion = "" // Prevent further replacements
			return fmt.Sprintf("<version>%s</version>", version)
		}
		return match
	})

	if err := os.WriteFile(pomPath, []byte(newContent), 0644); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to write pom.xml: %v", err),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Updated pom.xml version to %s (manual)", version),
		Outputs: map[string]any{
			"new_version": version,
		},
	}, nil
}

// updateGradleVersion updates the version in build.gradle or build.gradle.kts.
func (p *MavenPlugin) updateGradleVersion(projectDir, version string, dryRun bool) (*plugin.ExecuteResponse, error) {
	// Try build.gradle.kts first
	gradlePath := filepath.Join(projectDir, "build.gradle.kts")
	if _, err := os.Stat(gradlePath); err != nil {
		// Fall back to build.gradle
		gradlePath = filepath.Join(projectDir, "build.gradle")
	}

	data, err := os.ReadFile(gradlePath)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to read gradle file: %v", err),
		}, nil
	}

	content := string(data)

	// Match version = "x.y.z" pattern
	versionPattern := regexp.MustCompile(`version\s*=\s*["']([^"']+)["']`)
	match := versionPattern.FindStringSubmatch(content)

	oldVersion := ""
	if len(match) > 1 {
		oldVersion = match[1]
	}

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: fmt.Sprintf("Would update Gradle version from %s to %s", oldVersion, version),
			Outputs: map[string]any{
				"old_version": oldVersion,
				"new_version": version,
			},
		}, nil
	}

	newContent := versionPattern.ReplaceAllString(content, fmt.Sprintf(`version = "%s"`, version))

	if err := os.WriteFile(gradlePath, []byte(newContent), 0644); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to write gradle file: %v", err),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Updated Gradle version to %s", version),
		Outputs: map[string]any{
			"old_version": oldVersion,
			"new_version": version,
		},
	}, nil
}

// publishArtifact publishes the artifact to Maven Central.
func (p *MavenPlugin) publishArtifact(ctx context.Context, cfg *Config, releaseCtx plugin.ReleaseContext, dryRun bool) (*plugin.ExecuteResponse, error) {
	projectDir, err := p.validateProjectDir(cfg.ProjectDir)
	if err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid project directory: %v", err),
		}, nil
	}

	var args []string
	var cmdName string

	if cfg.UseGradle {
		cmdName = "./gradlew"
		if _, err := os.Stat(filepath.Join(projectDir, "gradlew")); err != nil {
			cmdName = "gradle"
		}

		args = []string{"publish"}

		if cfg.SkipTests {
			args = append(args, "-x", "test")
		}
	} else {
		cmdName = cfg.MavenCommand
		if cmdName == "" {
			cmdName = "mvn"
		}

		// Default goals for Maven Central deployment
		goals := cfg.Goals
		if len(goals) == 0 {
			goals = []string{"clean", "deploy"}
		}

		args = goals

		if cfg.SkipTests {
			args = append(args, "-DskipTests")
		}

		if cfg.SkipGPG {
			args = append(args, "-Dgpg.skip=true")
		} else if cfg.GPGPassphrase != "" {
			args = append(args, fmt.Sprintf("-Dgpg.passphrase=%s", cfg.GPGPassphrase))
		}

		if cfg.ServerID != "" {
			args = append(args, fmt.Sprintf("-DserverId=%s", cfg.ServerID))
		}

		if cfg.RepositoryURL != "" {
			args = append(args, fmt.Sprintf("-DaltDeploymentRepository=release::default::%s", cfg.RepositoryURL))
		}

		for _, profile := range cfg.Profiles {
			args = append(args, "-P"+profile)
		}
	}

	// Build log-safe command string
	logArgs := make([]string, len(args))
	copy(logArgs, args)
	for i := range logArgs {
		if strings.HasPrefix(logArgs[i], "-Dgpg.passphrase=") {
			logArgs[i] = "-Dgpg.passphrase=[REDACTED]"
		}
	}
	cmdStr := fmt.Sprintf("%s %s", cmdName, strings.Join(logArgs, " "))

	if dryRun {
		return &plugin.ExecuteResponse{
			Success: true,
			Message: "Would publish to Maven Central",
			Outputs: map[string]any{
				"command":     cmdStr,
				"version":     releaseCtx.Version,
				"project_dir": projectDir,
				"use_gradle":  cfg.UseGradle,
			},
		}, nil
	}

	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Dir = projectDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &plugin.ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("publish failed: %v\nstderr: %s", err, stderr.String()),
		}, nil
	}

	return &plugin.ExecuteResponse{
		Success: true,
		Message: fmt.Sprintf("Published version %s to Maven Central", releaseCtx.Version),
		Outputs: map[string]any{
			"version": releaseCtx.Version,
			"stdout":  stdout.String(),
		},
	}, nil
}

// validateProjectDir validates and returns the project directory.
func (p *MavenPlugin) validateProjectDir(dir string) (string, error) {
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
func (p *MavenPlugin) parseConfig(raw map[string]any) *Config {
	parser := plugin.NewConfigParser(raw)

	return &Config{
		ServerID:      parser.GetString("server_id"),
		RepositoryURL: parser.GetString("repository_url"),
		GPGKeyID:      parser.GetString("gpg_key_id"),
		GPGPassphrase: parser.GetString("gpg_passphrase", "GPG_PASSPHRASE"),
		SkipTests:     parser.GetBoolDefault("skip_tests", true),
		SkipGPG:       parser.GetBool("skip_gpg"),
		Profiles:      parser.GetStringSlice("profiles"),
		Goals:         parser.GetStringSlice("goals"),
		ProjectDir:    parser.GetString("project_dir"),
		UpdateVersion: parser.GetBoolDefault("update_version", true),
		UseGradle:     parser.GetBool("use_gradle"),
		MavenCommand:  parser.GetStringDefault("maven_command", "mvn"),
	}
}

// Validate validates the plugin configuration.
func (p *MavenPlugin) Validate(_ context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	vb := plugin.NewValidationBuilder()
	parser := plugin.NewConfigParser(config)

	useGradle := parser.GetBool("use_gradle")

	if useGradle {
		// Check for Gradle
		if _, err := exec.LookPath("gradle"); err != nil {
			if _, err := exec.LookPath("gradlew"); err != nil {
				vb.AddError("", "neither gradle nor gradlew found in PATH", "dependency")
			}
		}
	} else {
		// Check for Maven
		mvnCmd := parser.GetStringDefault("maven_command", "mvn")
		if _, err := exec.LookPath(mvnCmd); err != nil {
			vb.AddError("", fmt.Sprintf("%s command not found in PATH", mvnCmd), "dependency")
		}
	}

	// Check project directory if provided
	if dir := parser.GetString("project_dir"); dir != "" {
		if useGradle {
			// Check for build.gradle or build.gradle.kts
			gradlePath := filepath.Join(dir, "build.gradle")
			gradleKtsPath := filepath.Join(dir, "build.gradle.kts")
			if _, err := os.Stat(gradlePath); err != nil {
				if _, err := os.Stat(gradleKtsPath); err != nil {
					vb.AddError("project_dir", "no build.gradle or build.gradle.kts found", "path")
				}
			}
		} else {
			pomPath := filepath.Join(dir, "pom.xml")
			if _, err := os.Stat(pomPath); err != nil {
				vb.AddError("project_dir", fmt.Sprintf("pom.xml not found at %s", pomPath), "path")
			}
		}
	}

	// Warn if no GPG configured
	if !parser.GetBool("skip_gpg") {
		gpgPassphrase := parser.GetString("gpg_passphrase", "GPG_PASSPHRASE")
		if gpgPassphrase == "" {
			vb.AddWarning("gpg", "no GPG passphrase configured; Maven Central may reject unsigned artifacts")
		}
	}

	return vb.Build(), nil
}
