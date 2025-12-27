// Package cli provides the command-line interface for Relicta.
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	apprelease "github.com/relicta-tech/relicta/internal/application/release"
	"github.com/relicta-tech/relicta/internal/domain/communication"
	domainrelease "github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestWriteNotesToFile_Success(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "release-notes.md")

	// Create mock release notes using builder
	ver, _ := version.Parse("1.0.0")
	notes := communication.NewReleaseNotesBuilder(ver).
		WithSummary("Test release summary").
		Build()

	output := &apprelease.GenerateNotesOutput{
		ReleaseNotes: notes,
	}

	// Test writing to file
	err := writeNotesToFile(output, testFile)
	if err != nil {
		t.Fatalf("writeNotesToFile() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("writeNotesToFile() did not create the file")
	}

	// Verify content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	expectedContent := notes.Render()
	if string(content) != expectedContent {
		t.Errorf("writeNotesToFile() content mismatch\ngot: %s\nwant: %s", string(content), expectedContent)
	}
}

func TestWriteNotesToFile_ErrorInvalidPath(t *testing.T) {
	// Try to write to an invalid path (directory doesn't exist)
	invalidPath := "/nonexistent/directory/notes.md"

	ver, _ := version.Parse("1.0.0")
	notes := communication.NewReleaseNotesBuilder(ver).
		WithSummary("Test summary").
		Build()

	output := &apprelease.GenerateNotesOutput{
		ReleaseNotes: notes,
	}

	err := writeNotesToFile(output, invalidPath)
	if err == nil {
		t.Error("writeNotesToFile() expected error for invalid path, got nil")
	}
}

func TestOutputNotesToStdout_WithChangelog(t *testing.T) {
	// Create mock output with both changelog and release notes
	ver, _ := version.Parse("1.0.0")
	changelog := communication.NewChangelog("Changelog", communication.FormatKeepAChangelog)

	notes := communication.NewReleaseNotesBuilder(ver).
		WithSummary("Test summary").
		Build()

	output := &apprelease.GenerateNotesOutput{
		Changelog:    changelog,
		ReleaseNotes: notes,
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Call the function
	outputNotesToStdout(output)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output_str := buf.String()

	// Verify output contains expected sections
	if len(output_str) == 0 {
		t.Error("outputNotesToStdout() produced no output")
	}
}

func TestOutputNotesToStdout_WithoutChangelog(t *testing.T) {
	// Create mock output with only release notes
	ver, _ := version.Parse("1.0.0")
	notes := communication.NewReleaseNotesBuilder(ver).
		WithSummary("Test summary").
		Build()

	output := &apprelease.GenerateNotesOutput{
		Changelog:    nil,
		ReleaseNotes: notes,
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Call the function
	outputNotesToStdout(output)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output_str := buf.String()

	// Verify output is not empty
	if len(output_str) == 0 {
		t.Error("outputNotesToStdout() produced no output")
	}
}

func TestPrintNotesNextStepsOutput(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Call the function
	printNotesNextSteps()

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify output contains expected text
	expectedTexts := []string{
		"Next Steps",
		"approve",
		"publish",
	}

	for _, expected := range expectedTexts {
		if !bytes.Contains([]byte(output), []byte(expected)) {
			t.Errorf("printNotesNextSteps() output missing expected text: %s", expected)
		}
	}
}

func TestOutputNotesJSON_Complete(t *testing.T) {
	// Save and restore original stdout
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	// Create pipe to capture output
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create test data
	rel := domainrelease.NewRelease(domainrelease.RunID("test-release-id"), "main", "test-repo")

	ver, _ := version.Parse("1.0.0")
	changelog := communication.NewChangelog("Changelog", communication.FormatKeepAChangelog)

	notes := communication.NewReleaseNotesBuilder(ver).
		WithSummary("Test summary").
		AIGenerated().
		Build()

	output := &apprelease.GenerateNotesOutput{
		Changelog:    changelog,
		ReleaseNotes: notes,
	}

	// Call function
	err := outputNotesJSON(output, rel)

	// Close writer and restore stdout
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputNotesJSON() error = %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	jsonOutput := buf.String()

	// Verify JSON output contains expected fields
	expectedFields := []string{
		"release_id",
		"state",
		"test-release-id",
	}

	for _, field := range expectedFields {
		if !bytes.Contains([]byte(jsonOutput), []byte(field)) {
			t.Errorf("outputNotesJSON() missing field: %s\nGot: %s", field, jsonOutput)
		}
	}
}
