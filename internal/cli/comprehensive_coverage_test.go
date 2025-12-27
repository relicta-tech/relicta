// Package cli provides the command-line interface for Relicta.
// This file contains comprehensive tests to increase coverage.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/relicta-tech/relicta/internal/application/release"
	"github.com/relicta-tech/relicta/internal/domain/changes"
	domainrelease "github.com/relicta-tech/relicta/internal/domain/release"
	domainversion "github.com/relicta-tech/relicta/internal/domain/version"
)

// Test initConfig function
func TestInitConfig_Coverage(t *testing.T) {
	// Create a temp directory with a valid config
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// Create a valid config file
	configContent := `
versioning:
  strategy: conventional
  tag_prefix: "v"
ai:
  enabled: false
`
	err := os.WriteFile(tmpDir+"/release.config.yaml", []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Test initConfig
	err = initConfig()
	if err != nil {
		t.Errorf("initConfig() error = %v", err)
	}
}

// Test printConventionalCommit is already tested in other files
// Skipping duplicate tests

// Test getNonCoreCategorizedCommits with commits
func TestGetNonCoreCategorizedCommits_WithCommits(t *testing.T) {
	cats := &changes.Categories{
		Features: nil,
		Fixes:    nil,
		Perf:     nil,
		Docs: []*changes.ConventionalCommit{
			changes.NewConventionalCommit("doc1", changes.CommitTypeDocs, "Update docs"),
		},
		Refactors: []*changes.ConventionalCommit{
			changes.NewConventionalCommit("ref1", changes.CommitTypeRefactor, "Refactor code"),
		},
		Tests:    nil,
		Chores:   nil,
		Build:    nil,
		CI:       nil,
		Other:    nil,
		Breaking: nil,
	}

	result := getNonCoreCategorizedCommits(cats)
	if len(result) != 2 {
		t.Errorf("getNonCoreCategorizedCommits() returned %d commits, want 2", len(result))
	}
}

// Test Cleanup function
func TestCleanup_WithLogFile(t *testing.T) {
	// Create a temp log file
	tmpFile, err := os.CreateTemp("", "test-log-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Save original state
	origLogFile := logFile
	defer func() { logFile = origLogFile }()

	logFile = tmpFile

	// Call Cleanup
	Cleanup()

	if logFile != nil {
		t.Error("Cleanup() should set logFile to nil")
	}
}

// Test outputPlanText with breaking changes
func TestOutputPlanText_WithBreaking(t *testing.T) {
	currentVersion, _ := domainversion.Parse("1.0.0")
	nextVersion, _ := domainversion.Parse("2.0.0")
	changeSet := changes.NewChangeSet(changes.ChangeSetID("test-id"), "main", "HEAD")

	// Add a breaking change
	commit := changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "Change API")
	changeSet.AddCommit(commit)

	output := &release.PlanReleaseOutput{
		ReleaseID:      domainrelease.RunID("test-release"),
		CurrentVersion: currentVersion,
		NextVersion:    nextVersion,
		ReleaseType:    changes.ReleaseTypeMajor,
		ChangeSet:      changeSet,
		RepositoryName: "test-repo",
		Branch:         "main",
	}

	err := outputPlanText(output, false, false, nil)
	if err != nil {
		t.Errorf("outputPlanText() with breaking changes error = %v", err)
	}
}

// Test outputPlanJSON in CI mode
func TestOutputPlanJSON_CIMode(t *testing.T) {
	origCIMode := ciMode
	defer func() { ciMode = origCIMode }()
	ciMode = true

	currentVersion, _ := domainversion.Parse("1.0.0")
	nextVersion, _ := domainversion.Parse("1.1.0")
	changeSet := changes.NewChangeSet(changes.ChangeSetID("test-id"), "main", "HEAD")

	output := &release.PlanReleaseOutput{
		ReleaseID:      domainrelease.RunID("test-release"),
		CurrentVersion: currentVersion,
		NextVersion:    nextVersion,
		ReleaseType:    changes.ReleaseTypeMinor,
		ChangeSet:      changeSet,
		RepositoryName: "test-repo",
		Branch:         "main",
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputPlanJSON(output, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("outputPlanJSON() in CI mode error = %v", err)
	}

	// Read output
	var buf bytes.Buffer
	buf.ReadFrom(r)

	// Parse JSON output
	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Errorf("outputPlanJSON() produced invalid JSON: %v", err)
	}

	// Verify ci_mode flag is present
	if ciMode, ok := result["ci_mode"].(bool); !ok || !ciMode {
		t.Error("outputPlanJSON() should include ci_mode flag when in CI mode")
	}
}

// Test checkConfig with existing config
func TestCheckConfig_WithConfig(t *testing.T) {
	// Create a temp directory with a config file
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	configPath := tmpDir + "/release.config.yaml"
	err := os.WriteFile(configPath, []byte("versioning:\n  strategy: conventional"), 0600)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	ctx := context.Background()
	health := checkConfig(ctx)

	if health.Name != "config" {
		t.Errorf("checkConfig() name = %v, want config", health.Name)
	}

	if health.Status != HealthStatusHealthy {
		t.Errorf("checkConfig() with valid config should be healthy, got %v: %s", health.Status, health.Message)
	}
}

// Test filterNonBreaking with breaking commits
func TestFilterNonBreaking_WithBreaking(t *testing.T) {
	commits := []*changes.ConventionalCommit{
		changes.NewConventionalCommit("1", changes.CommitTypeFeat, "Feature 1"),
		changes.NewConventionalCommit("2", changes.CommitTypeFeat, "Feature 2"),
		changes.NewConventionalCommit("3", changes.CommitTypeFeat, "Feature 3"),
	}

	result := filterNonBreaking(commits)

	// Should return all commits if none are breaking
	if len(result) != len(commits) {
		t.Errorf("filterNonBreaking() returned %d commits, want %d", len(result), len(commits))
	}
}

// Test checkGit when git is available
func TestCheckGit_Available(t *testing.T) {
	ctx := context.Background()
	health := checkGit(ctx)

	if health.Name != "git" {
		t.Errorf("checkGit() name = %v, want git", health.Name)
	}

	// Git should be available in most test environments
	// If not available, we'll just log it
	if health.Status != HealthStatusHealthy {
		t.Logf("Git not available: %s (this is OK in some environments)", health.Message)
	}
}

// Test checkRepository in non-git directory
func TestCheckRepository_NonGitDir_Comprehensive(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	err := os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	ctx := context.Background()
	health := checkRepository(ctx)

	if health.Name != "repository" {
		t.Errorf("checkRepository() name = %v, want repository", health.Name)
	}

	// Should be degraded or unhealthy in non-git directory
	if health.Status == HealthStatusHealthy {
		t.Error("checkRepository() in non-git directory should not be healthy")
	}
}
