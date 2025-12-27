// Package cli provides the command-line interface for Relicta.
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/relicta-tech/relicta/internal/application/release"
	"github.com/relicta-tech/relicta/internal/application/versioning"
	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/changes"
	domainrelease "github.com/relicta-tech/relicta/internal/domain/release"
	domainversion "github.com/relicta-tech/relicta/internal/domain/version"
)

func TestOutputPlanJSON_Coverage(t *testing.T) {
	// Create test data
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
		t.Errorf("outputPlanJSON() error = %v", err)
	}

	// Read output
	var buf bytes.Buffer
	buf.ReadFrom(r)

	// Parse JSON output
	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Errorf("outputPlanJSON() produced invalid JSON: %v", err)
	}

	// Verify fields
	if result["release_id"] != "test-release" {
		t.Errorf("outputPlanJSON() release_id = %v, want test-release", result["release_id"])
	}
	if result["current_version"] != "1.0.0" {
		t.Errorf("outputPlanJSON() current_version = %v, want 1.0.0", result["current_version"])
	}
	if result["next_version"] != "1.1.0" {
		t.Errorf("outputPlanJSON() next_version = %v, want 1.1.0", result["next_version"])
	}
}

func TestOutputPlanText_Coverage(t *testing.T) {
	// Create test data
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

	// Save original state
	origDryRun := dryRun
	defer func() { dryRun = origDryRun }()
	dryRun = false

	// Test minimal output
	err := outputPlanText(output, false, true, nil)
	if err != nil {
		t.Errorf("outputPlanText() minimal error = %v", err)
	}

	// Test full output
	err = outputPlanText(output, false, false, nil)
	if err != nil {
		t.Errorf("outputPlanText() full error = %v", err)
	}

	// Test show all
	err = outputPlanText(output, true, false, nil)
	if err != nil {
		t.Errorf("outputPlanText() show all error = %v", err)
	}
}

func TestOutputSetVersionJSON_Coverage(t *testing.T) {
	// Create test data
	version, _ := domainversion.Parse("2.0.0")

	output := &versioning.SetVersionOutput{
		Version:    version,
		TagCreated: true,
		TagPushed:  true,
		TagName:    "v2.0.0",
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputSetVersionJSON(output)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("outputSetVersionJSON() error = %v", err)
	}

	// Read output
	var buf bytes.Buffer
	buf.ReadFrom(r)

	// Parse JSON output
	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Errorf("outputSetVersionJSON() produced invalid JSON: %v", err)
	}

	if result["version"] != "2.0.0" {
		t.Errorf("outputSetVersionJSON() version = %v, want 2.0.0", result["version"])
	}
	if result["tag_created"] != true {
		t.Errorf("outputSetVersionJSON() tag_created = %v, want true", result["tag_created"])
	}
}

func TestOutputSetVersionResult_Coverage(t *testing.T) {
	version, _ := domainversion.Parse("1.2.3")
	cfg = &config.Config{
		Versioning: config.VersioningConfig{
			TagPrefix: "v",
		},
	}

	// Test with tag created
	output := &versioning.SetVersionOutput{
		Version:    version,
		TagCreated: true,
		TagPushed:  false,
		TagName:    "v1.2.3",
	}

	// Just verify it doesn't panic
	outputSetVersionResult(output)

	// Test with tag pushed
	output.TagPushed = true
	outputSetVersionResult(output)
}

func TestPrintBumpNextSteps(t *testing.T) {
	printBumpNextSteps()
	// Just verify it doesn't panic
}

func TestPrintNotesNextSteps(t *testing.T) {
	printNotesNextSteps()
	// Just verify it doesn't panic
}

func TestOutputCalculatedVersionText_Coverage(t *testing.T) {
	currentVersion, _ := domainversion.Parse("1.0.0")
	nextVersion, _ := domainversion.Parse("1.1.0")

	calcOutput := &versioning.CalculateVersionOutput{
		CurrentVersion: currentVersion,
		NextVersion:    nextVersion,
		BumpType:       domainversion.BumpMinor,
		AutoDetected:   true,
	}

	// Just verify it doesn't panic
	outputCalculatedVersionText(calcOutput, nextVersion)
}
