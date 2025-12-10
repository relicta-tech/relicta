// Package cli provides the command-line interface for ReleasePilot.
package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/felixgeelhaar/release-pilot/internal/config"
)

func TestNewOptions(t *testing.T) {
	opts := NewOptions()

	if opts == nil {
		t.Fatal("NewOptions returned nil")
	}
	if opts.Logger == nil {
		t.Error("Logger should be initialized")
	}
	if opts.Stdout == nil {
		t.Error("Stdout should be initialized")
	}
	if opts.Stderr == nil {
		t.Error("Stderr should be initialized")
	}
	if opts.Stdin == nil {
		t.Error("Stdin should be initialized")
	}
}

func TestDefaultStyles(t *testing.T) {
	styles := DefaultStyles()

	// Verify styles render content correctly
	// Styles.Render() applies styling to text
	if styles.Title.Render("test") == "" {
		t.Error("Title style should render content")
	}
	if styles.Success.Render("test") == "" {
		t.Error("Success style should render content")
	}
	if styles.Error.Render("test") == "" {
		t.Error("Error style should render content")
	}
	if styles.Warning.Render("test") == "" {
		t.Error("Warning style should render content")
	}
	if styles.Info.Render("test") == "" {
		t.Error("Info style should render content")
	}
	if styles.Subtle.Render("test") == "" {
		t.Error("Subtle style should render content")
	}
	if styles.Bold.Render("test") == "" {
		t.Error("Bold style should render content")
	}
}

func TestOptions_SetVersion(t *testing.T) {
	opts := NewOptions()
	opts.SetVersion("1.0.0", "abc123", "2024-01-01")

	if opts.Version.Version != "1.0.0" {
		t.Errorf("Version = %v, want 1.0.0", opts.Version.Version)
	}
	if opts.Version.Commit != "abc123" {
		t.Errorf("Commit = %v, want abc123", opts.Version.Commit)
	}
	if opts.Version.Date != "2024-01-01" {
		t.Errorf("Date = %v, want 2024-01-01", opts.Version.Date)
	}
}

func TestOptions_IsCI(t *testing.T) {
	opts := NewOptions()

	if opts.IsCI() {
		t.Error("IsCI() should return false by default")
	}

	opts.CIMode = true
	if !opts.IsCI() {
		t.Error("IsCI() should return true when CIMode is true")
	}
}

func TestOptions_IsJSON(t *testing.T) {
	opts := NewOptions()

	if opts.IsJSON() {
		t.Error("IsJSON() should return false by default")
	}

	opts.JSONOutput = true
	if !opts.IsJSON() {
		t.Error("IsJSON() should return true when JSONOutput is true")
	}
}

func TestOptions_IsDryRun(t *testing.T) {
	t.Run("flag only", func(t *testing.T) {
		opts := NewOptions()
		if opts.IsDryRun() {
			t.Error("IsDryRun() should return false by default")
		}

		opts.DryRun = true
		if !opts.IsDryRun() {
			t.Error("IsDryRun() should return true when DryRun flag is true")
		}
	})

	t.Run("config only", func(t *testing.T) {
		opts := NewOptions()
		opts.Config = &config.Config{
			Workflow: config.WorkflowConfig{
				DryRunByDefault: true,
			},
		}

		if !opts.IsDryRun() {
			t.Error("IsDryRun() should return true when config DryRunByDefault is true")
		}
	})

	t.Run("flag overrides config", func(t *testing.T) {
		opts := NewOptions()
		opts.DryRun = true
		opts.Config = &config.Config{
			Workflow: config.WorkflowConfig{
				DryRunByDefault: false,
			},
		}

		if !opts.IsDryRun() {
			t.Error("IsDryRun() should return true when flag is true")
		}
	})
}

func TestOptions_IsVerbose(t *testing.T) {
	t.Run("flag only", func(t *testing.T) {
		opts := NewOptions()
		if opts.IsVerbose() {
			t.Error("IsVerbose() should return false by default")
		}

		opts.Verbose = true
		if !opts.IsVerbose() {
			t.Error("IsVerbose() should return true when Verbose flag is true")
		}
	})

	t.Run("config only", func(t *testing.T) {
		opts := NewOptions()
		opts.Config = &config.Config{
			Output: config.OutputConfig{
				Verbose: true,
			},
		}

		if !opts.IsVerbose() {
			t.Error("IsVerbose() should return true when config Verbose is true")
		}
	})
}

func TestOptions_Cleanup(t *testing.T) {
	t.Run("nil log file", func(t *testing.T) {
		opts := NewOptions()

		// Should not panic when LogFile is nil
		opts.Cleanup()

		// LogFile should remain nil
		if opts.LogFile != nil {
			t.Error("LogFile should be nil after cleanup")
		}
	})

	t.Run("with log file", func(t *testing.T) {
		opts := NewOptions()

		// Create a temporary file to use as LogFile
		tmpFile, err := os.CreateTemp("", "test-log-*.log")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		opts.LogFile = tmpFile

		// Cleanup should close the log file
		opts.Cleanup()

		// LogFile should be nil after cleanup
		if opts.LogFile != nil {
			t.Error("LogFile should be nil after cleanup")
		}
	})
}

func TestOptions_PrintMethods(t *testing.T) {
	tests := []struct {
		name     string
		method   func(*Options, string)
		expected string
	}{
		{"PrintSuccess", (*Options).PrintSuccess, "✓"},
		{"PrintError", (*Options).PrintError, "✗"},
		{"PrintWarning", (*Options).PrintWarning, "⚠"},
		{"PrintInfo", (*Options).PrintInfo, "ℹ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			opts := NewOptions()
			opts.Stdout = &buf

			tt.method(opts, "test message")

			output := buf.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("Output should contain %q, got: %s", tt.expected, output)
			}
			if !strings.Contains(output, "test message") {
				t.Errorf("Output should contain message, got: %s", output)
			}
		})
	}
}

func TestOptions_PrintTitle(t *testing.T) {
	var buf bytes.Buffer
	opts := NewOptions()
	opts.Stdout = &buf

	opts.PrintTitle("Test Title")

	output := buf.String()
	if !strings.Contains(output, "Test Title") {
		t.Errorf("Output should contain title, got: %s", output)
	}
}

func TestOptions_PrintSubtle(t *testing.T) {
	var buf bytes.Buffer
	opts := NewOptions()
	opts.Stdout = &buf

	opts.PrintSubtle("subtle text")

	output := buf.String()
	if !strings.Contains(output, "subtle text") {
		t.Errorf("Output should contain subtle text, got: %s", output)
	}
}

func TestOptions_NilStdout(t *testing.T) {
	opts := NewOptions()
	opts.Stdout = nil

	// Should not panic
	opts.PrintSuccess("test")
	opts.PrintError("test")
	opts.PrintWarning("test")
	opts.PrintInfo("test")
	opts.PrintTitle("test")
	opts.PrintSubtle("test")
}

func TestPlanOptions(t *testing.T) {
	opts := NewOptions()
	planOpts := PlanOptions{
		CommandOptions: CommandOptions{Options: opts},
		FromRef:        "v1.0.0",
		ToRef:          "HEAD",
		ShowAll:        true,
		Minimal:        false,
	}

	if planOpts.FromRef != "v1.0.0" {
		t.Errorf("FromRef = %v, want v1.0.0", planOpts.FromRef)
	}
	if planOpts.ToRef != "HEAD" {
		t.Errorf("ToRef = %v, want HEAD", planOpts.ToRef)
	}
	if !planOpts.ShowAll {
		t.Error("ShowAll should be true")
	}
	if planOpts.Minimal {
		t.Error("Minimal should be false")
	}

	// Verify embedded Options is accessible
	if planOpts.Options == nil {
		t.Error("Options should be accessible through embedding")
	}
}

func TestBumpOptions(t *testing.T) {
	opts := NewOptions()
	bumpOpts := BumpOptions{
		CommandOptions: CommandOptions{Options: opts},
		ReleaseType:    "minor",
		Prerelease:     "beta",
		SkipTag:        true,
		ForceVersion:   "2.0.0",
	}

	if bumpOpts.ReleaseType != "minor" {
		t.Errorf("ReleaseType = %v, want minor", bumpOpts.ReleaseType)
	}
	if bumpOpts.Prerelease != "beta" {
		t.Errorf("Prerelease = %v, want beta", bumpOpts.Prerelease)
	}
	if !bumpOpts.SkipTag {
		t.Error("SkipTag should be true")
	}
	if bumpOpts.ForceVersion != "2.0.0" {
		t.Errorf("ForceVersion = %v, want 2.0.0", bumpOpts.ForceVersion)
	}
}

func TestNotesOptions(t *testing.T) {
	opts := NewOptions()
	notesOpts := NotesOptions{
		CommandOptions: CommandOptions{Options: opts},
		NoAI:           true,
		OutputFile:     "RELEASE_NOTES.md",
		Format:         "markdown",
	}

	if !notesOpts.NoAI {
		t.Error("NoAI should be true")
	}
	if notesOpts.OutputFile != "RELEASE_NOTES.md" {
		t.Errorf("OutputFile = %v, want RELEASE_NOTES.md", notesOpts.OutputFile)
	}
	if notesOpts.Format != "markdown" {
		t.Errorf("Format = %v, want markdown", notesOpts.Format)
	}
}

func TestApproveOptions(t *testing.T) {
	opts := NewOptions()
	approveOpts := ApproveOptions{
		CommandOptions: CommandOptions{Options: opts},
		Edit:           true,
	}

	if !approveOpts.Edit {
		t.Error("Edit should be true")
	}
}

func TestPublishOptions(t *testing.T) {
	opts := NewOptions()
	publishOpts := PublishOptions{
		CommandOptions: CommandOptions{Options: opts},
		SkipPlugins:    []string{"slack"},
		OnlyPlugins:    []string{"github"},
		Force:          true,
	}

	if len(publishOpts.SkipPlugins) != 1 || publishOpts.SkipPlugins[0] != "slack" {
		t.Errorf("SkipPlugins = %v, want [slack]", publishOpts.SkipPlugins)
	}
	if len(publishOpts.OnlyPlugins) != 1 || publishOpts.OnlyPlugins[0] != "github" {
		t.Errorf("OnlyPlugins = %v, want [github]", publishOpts.OnlyPlugins)
	}
	if !publishOpts.Force {
		t.Error("Force should be true")
	}
}

func TestBlastOptions(t *testing.T) {
	opts := NewOptions()
	blastOpts := BlastOptions{
		CommandOptions: CommandOptions{Options: opts},
		Paths:          []string{"pkg/core", "pkg/api"},
		IncludeShared:  true,
		OutputFormat:   "json",
	}

	if len(blastOpts.Paths) != 2 {
		t.Errorf("Paths length = %d, want 2", len(blastOpts.Paths))
	}
	if !blastOpts.IncludeShared {
		t.Error("IncludeShared should be true")
	}
	if blastOpts.OutputFormat != "json" {
		t.Errorf("OutputFormat = %v, want json", blastOpts.OutputFormat)
	}
}

func TestVersionInfo(t *testing.T) {
	info := VersionInfo{
		Version: "1.2.3",
		Commit:  "abc123def",
		Date:    "2024-06-15",
	}

	if info.Version != "1.2.3" {
		t.Errorf("Version = %v, want 1.2.3", info.Version)
	}
	if info.Commit != "abc123def" {
		t.Errorf("Commit = %v, want abc123def", info.Commit)
	}
	if info.Date != "2024-06-15" {
		t.Errorf("Date = %v, want 2024-06-15", info.Date)
	}
}
