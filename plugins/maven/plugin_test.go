// Package main implements tests for the Maven Central plugin.
package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

func TestGetInfo(t *testing.T) {
	p := &MavenPlugin{}
	info := p.GetInfo()

	if info.Name != "maven" {
		t.Errorf("expected name 'maven', got %s", info.Name)
	}

	if info.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", info.Version)
	}

	if len(info.Hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(info.Hooks))
	}
}

func TestParseConfig(t *testing.T) {
	p := &MavenPlugin{}

	tests := []struct {
		name     string
		config   map[string]any
		expected *Config
	}{
		{
			name: "all fields",
			config: map[string]any{
				"server_id":      "ossrh",
				"repository_url": "https://oss.sonatype.org/",
				"gpg_key_id":     "ABC123",
				"gpg_passphrase": "secret",
				"skip_tests":     true,
				"skip_gpg":       false,
				"profiles":       []any{"release", "deploy"},
				"goals":          []any{"clean", "deploy"},
				"project_dir":    "./java-project",
				"update_version": true,
				"use_gradle":     false,
				"maven_command":  "./mvnw",
			},
			expected: &Config{
				ServerID:      "ossrh",
				RepositoryURL: "https://oss.sonatype.org/",
				GPGKeyID:      "ABC123",
				GPGPassphrase: "secret",
				SkipTests:     true,
				SkipGPG:       false,
				Profiles:      []string{"release", "deploy"},
				Goals:         []string{"clean", "deploy"},
				ProjectDir:    "./java-project",
				UpdateVersion: true,
				UseGradle:     false,
				MavenCommand:  "./mvnw",
			},
		},
		{
			name:   "defaults",
			config: map[string]any{},
			expected: &Config{
				SkipTests:     true,
				UpdateVersion: true,
				MavenCommand:  "mvn",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := p.parseConfig(tt.config)

			if cfg.ServerID != tt.expected.ServerID {
				t.Errorf("ServerID: expected %s, got %s", tt.expected.ServerID, cfg.ServerID)
			}
			if cfg.SkipTests != tt.expected.SkipTests {
				t.Errorf("SkipTests: expected %v, got %v", tt.expected.SkipTests, cfg.SkipTests)
			}
			if cfg.UpdateVersion != tt.expected.UpdateVersion {
				t.Errorf("UpdateVersion: expected %v, got %v", tt.expected.UpdateVersion, cfg.UpdateVersion)
			}
			if cfg.MavenCommand != tt.expected.MavenCommand {
				t.Errorf("MavenCommand: expected %s, got %s", tt.expected.MavenCommand, cfg.MavenCommand)
			}
		})
	}
}

func TestValidateProjectDir(t *testing.T) {
	p := &MavenPlugin{}

	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		dir         string
		expectError bool
	}{
		{
			name:        "empty dir returns current",
			dir:         "",
			expectError: false,
		},
		{
			name:        "valid directory",
			dir:         tmpDir,
			expectError: false,
		},
		{
			name:        "path traversal blocked",
			dir:         "../../../etc",
			expectError: true,
		},
		{
			name:        "nonexistent directory",
			dir:         "/nonexistent/path/to/dir",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.validateProjectDir(tt.dir)
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestUpdateGradleVersion(t *testing.T) {
	p := &MavenPlugin{}

	tmpDir := t.TempDir()
	gradlePath := filepath.Join(tmpDir, "build.gradle.kts")

	// Write initial build.gradle.kts
	initialContent := `plugins {
    kotlin("jvm") version "1.9.0"
}

group = "com.example"
version = "0.1.0"

repositories {
    mavenCentral()
}
`
	if err := os.WriteFile(gradlePath, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Test dry run
	resp, err := p.updateGradleVersion(tmpDir, "1.0.0", true)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	if resp.Outputs["old_version"] != "0.1.0" {
		t.Errorf("unexpected old_version: %v", resp.Outputs["old_version"])
	}

	// Verify file unchanged after dry run
	content, _ := os.ReadFile(gradlePath)
	if string(content) != initialContent {
		t.Error("file was modified during dry run")
	}

	// Test actual update
	resp, err = p.updateGradleVersion(tmpDir, "1.0.0", false)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	// Verify file updated
	content, _ = os.ReadFile(gradlePath)
	if !contains(string(content), `version = "1.0.0"`) {
		t.Errorf("version not updated in file: %s", string(content))
	}
}

func TestManualPOMUpdate(t *testing.T) {
	p := &MavenPlugin{}

	tmpDir := t.TempDir()
	pomPath := filepath.Join(tmpDir, "pom.xml")

	// Write initial pom.xml
	initialContent := `<?xml version="1.0" encoding="UTF-8"?>
<project>
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>my-artifact</artifactId>
    <version>0.1.0</version>
    <packaging>jar</packaging>
</project>
`
	if err := os.WriteFile(pomPath, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	resp, err := p.manualPOMUpdate(pomPath, "1.0.0", "0.1.0")
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	// Verify file updated
	content, _ := os.ReadFile(pomPath)
	if !contains(string(content), "<version>1.0.0</version>") {
		t.Errorf("version not updated in file: %s", string(content))
	}
}

func TestExecuteDryRun(t *testing.T) {
	p := &MavenPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()

	// Create pom.xml
	pomPath := filepath.Join(tmpDir, "pom.xml")
	if err := os.WriteFile(pomPath, []byte(`<?xml version="1.0"?>
<project>
    <version>0.1.0</version>
</project>
`), 0644); err != nil {
		t.Fatal(err)
	}

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPostPublish,
		Config: map[string]any{
			"project_dir": tmpDir,
			"server_id":   "ossrh",
			"skip_tests":  true,
		},
		Context: plugin.ReleaseContext{
			Version: "1.0.0",
			TagName: "v1.0.0",
		},
		DryRun: true,
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	if resp.Message != "Would publish to Maven Central" {
		t.Errorf("unexpected message: %s", resp.Message)
	}

	if resp.Outputs["version"] != "1.0.0" {
		t.Errorf("unexpected version output: %v", resp.Outputs["version"])
	}
}

func TestExecuteUnhandledHook(t *testing.T) {
	p := &MavenPlugin{}
	ctx := context.Background()

	req := plugin.ExecuteRequest{
		Hook:   plugin.HookOnSuccess,
		Config: map[string]any{},
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success for unhandled hook")
	}
}

func TestValidate(t *testing.T) {
	p := &MavenPlugin{}
	ctx := context.Background()

	// Check if mvn or gradle is available
	_, mvnAvailable := exec.LookPath("mvn")
	_, gradleAvailable := exec.LookPath("gradle")

	tests := []struct {
		name        string
		config      map[string]any
		expectValid bool
	}{
		{
			name: "valid maven config",
			config: map[string]any{
				"server_id":      "ossrh",
				"gpg_passphrase": "secret",
			},
			expectValid: mvnAvailable == nil,
		},
		{
			name: "valid gradle config",
			config: map[string]any{
				"use_gradle": true,
			},
			expectValid: gradleAvailable == nil,
		},
		{
			name:        "no gpg passphrase warning",
			config:      map[string]any{},
			expectValid: mvnAvailable == nil, // Only valid if mvn is available (default is maven)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := p.Validate(ctx, tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Valid != tt.expectValid {
				t.Errorf("expected valid=%v, got valid=%v, errors=%v", tt.expectValid, resp.Valid, resp.Errors)
			}
		})
	}
}

func TestExecutePrePublish(t *testing.T) {
	p := &MavenPlugin{}
	ctx := context.Background()

	tmpDir := t.TempDir()
	gradlePath := filepath.Join(tmpDir, "build.gradle.kts")
	if err := os.WriteFile(gradlePath, []byte(`version = "0.1.0"
`), 0644); err != nil {
		t.Fatal(err)
	}

	req := plugin.ExecuteRequest{
		Hook: plugin.HookPrePublish,
		Config: map[string]any{
			"project_dir":    tmpDir,
			"update_version": true,
			"use_gradle":     true,
		},
		Context: plugin.ReleaseContext{
			Version: "1.0.0",
		},
		DryRun: true,
	}

	resp, err := p.Execute(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	if !contains(resp.Message, "Would update Gradle") {
		t.Errorf("unexpected message: %s", resp.Message)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
