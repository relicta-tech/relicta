// Package cli provides the command-line interface for Relicta.
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/relicta-tech/relicta/internal/application/versioning"
	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestBuildCalculateVersionInputExtended(t *testing.T) {
	// Setup test config
	originalCfg := cfg
	defer func() { cfg = originalCfg }()

	cfg = &config.Config{
		Versioning: config.VersioningConfig{
			TagPrefix: "v",
		},
	}

	// Save and restore flag variables
	originalPrerelease := bumpPrerelease
	defer func() { bumpPrerelease = originalPrerelease }()

	tests := []struct {
		name         string
		bumpType     version.BumpType
		auto         bool
		prerelease   string
		wantAuto     bool
		wantBumpType version.BumpType
	}{
		{
			name:         "manual major bump",
			bumpType:     version.BumpMajor,
			auto:         false,
			prerelease:   "",
			wantAuto:     false,
			wantBumpType: version.BumpMajor,
		},
		{
			name:         "auto detection",
			bumpType:     version.BumpType(""),
			auto:         true,
			prerelease:   "",
			wantAuto:     true,
			wantBumpType: version.BumpType(""),
		},
		{
			name:         "minor with prerelease",
			bumpType:     version.BumpMinor,
			auto:         false,
			prerelease:   "beta.1",
			wantAuto:     false,
			wantBumpType: version.BumpMinor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bumpPrerelease = tt.prerelease

			input := buildCalculateVersionInput(tt.bumpType, tt.auto)

			if input.Auto != tt.wantAuto {
				t.Errorf("buildCalculateVersionInput() Auto = %v, want %v", input.Auto, tt.wantAuto)
			}

			if input.BumpType != tt.wantBumpType {
				t.Errorf("buildCalculateVersionInput() BumpType = %v, want %v", input.BumpType, tt.wantBumpType)
			}

			if input.TagPrefix != "v" {
				t.Errorf("buildCalculateVersionInput() TagPrefix = %v, want v", input.TagPrefix)
			}

			if tt.prerelease != "" && string(input.Prerelease) != tt.prerelease {
				t.Errorf("buildCalculateVersionInput() Prerelease = %v, want %v", input.Prerelease, tt.prerelease)
			}
		})
	}
}

func TestOutputCalculatedVersionText(t *testing.T) {
	// Setup test config
	originalCfg := cfg
	defer func() { cfg = originalCfg }()

	cfg = &config.Config{
		Versioning: config.VersioningConfig{
			TagPrefix: "v",
		},
	}

	currentVer, _ := version.Parse("1.0.0")
	nextVer, _ := version.Parse("1.1.0")

	calcOutput := &versioning.CalculateVersionOutput{
		CurrentVersion: currentVer,
		NextVersion:    nextVer,
		BumpType:       version.BumpMinor,
		AutoDetected:   false,
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputCalculatedVersionText(calcOutput, nextVer)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify output contains version information
	expectedStrings := []string{
		"1.0.0",
		"1.1.0",
		"minor",
	}

	for _, expected := range expectedStrings {
		if !bytes.Contains([]byte(output), []byte(expected)) {
			t.Errorf("outputCalculatedVersionText() missing expected text: %s", expected)
		}
	}
}

func TestOutputCalculatedVersionText_AutoDetected(t *testing.T) {
	// Setup test config
	originalCfg := cfg
	defer func() { cfg = originalCfg }()

	cfg = &config.Config{
		Versioning: config.VersioningConfig{
			TagPrefix: "v",
		},
	}

	currentVer, _ := version.Parse("1.0.0")
	nextVer, _ := version.Parse("2.0.0")

	calcOutput := &versioning.CalculateVersionOutput{
		CurrentVersion: currentVer,
		NextVersion:    nextVer,
		BumpType:       version.BumpMajor,
		AutoDetected:   true,
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputCalculatedVersionText(calcOutput, nextVer)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify auto-detection message is present
	if !bytes.Contains([]byte(output), []byte("auto-detected")) {
		t.Error("outputCalculatedVersionText() missing auto-detected message")
	}
}

func TestPrintBumpNextStepsOutput(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printBumpNextSteps()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify output contains expected next steps
	expectedTexts := []string{
		"Next Steps",
		"notes",
		"approve",
		"publish",
	}

	for _, expected := range expectedTexts {
		if !bytes.Contains([]byte(output), []byte(expected)) {
			t.Errorf("printBumpNextSteps() missing expected text: %s", expected)
		}
	}
}

func TestOutputBumpJSONExtended(t *testing.T) {
	// Setup test config
	originalCfg := cfg
	defer func() { cfg = originalCfg }()

	cfg = &config.Config{
		Versioning: config.VersioningConfig{
			TagPrefix: "v",
		},
	}

	currentVer, _ := version.Parse("1.0.0")
	nextVer, _ := version.Parse("1.1.0")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputBumpJSON(currentVer, nextVer, version.BumpMinor, true)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputBumpJSON() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	// Parse JSON output
	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Verify JSON fields
	if result["current_version"] != "1.0.0" {
		t.Errorf("outputBumpJSON() current_version = %v, want 1.0.0", result["current_version"])
	}

	if result["next_version"] != "1.1.0" {
		t.Errorf("outputBumpJSON() next_version = %v, want 1.1.0", result["next_version"])
	}

	if result["bump_type"] != "minor" {
		t.Errorf("outputBumpJSON() bump_type = %v, want minor", result["bump_type"])
	}

	if result["auto_detected"] != true {
		t.Errorf("outputBumpJSON() auto_detected = %v, want true", result["auto_detected"])
	}

	if result["tag_name"] != "v1.1.0" {
		t.Errorf("outputBumpJSON() tag_name = %v, want v1.1.0", result["tag_name"])
	}
}

func TestOutputSetVersionJSON(t *testing.T) {
	ver, _ := version.Parse("1.2.3")
	output := &versioning.SetVersionOutput{
		Version:    ver,
		TagName:    "v1.2.3",
		TagCreated: true,
		TagPushed:  false,
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputSetVersionJSON(output)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputSetVersionJSON() error = %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	// Parse JSON output
	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Verify JSON fields
	if result["version"] != "1.2.3" {
		t.Errorf("outputSetVersionJSON() version = %v, want 1.2.3", result["version"])
	}

	if result["tag_name"] != "v1.2.3" {
		t.Errorf("outputSetVersionJSON() tag_name = %v, want v1.2.3", result["tag_name"])
	}

	if result["tag_created"] != true {
		t.Errorf("outputSetVersionJSON() tag_created = %v, want true", result["tag_created"])
	}

	if result["tag_pushed"] != false {
		t.Errorf("outputSetVersionJSON() tag_pushed = %v, want false", result["tag_pushed"])
	}
}

func TestOutputSetVersionResult(t *testing.T) {
	// Setup test config
	originalCfg := cfg
	defer func() { cfg = originalCfg }()

	cfg = &config.Config{
		Versioning: config.VersioningConfig{
			TagPrefix: "v",
		},
	}

	ver, _ := version.Parse("2.0.0")
	output := &versioning.SetVersionOutput{
		Version:    ver,
		TagName:    "v2.0.0",
		TagCreated: true,
		TagPushed:  true,
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputSetVersionResult(output)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	result := buf.String()

	// Verify output
	expectedStrings := []string{
		"v2.0.0",
		"Created tag",
		"pushed",
	}

	for _, expected := range expectedStrings {
		if !bytes.Contains([]byte(result), []byte(expected)) {
			t.Errorf("outputSetVersionResult() missing expected text: %s", expected)
		}
	}
}

func TestBuildSetVersionInputExtended(t *testing.T) {
	// Setup test config
	originalCfg := cfg
	defer func() { cfg = originalCfg }()

	cfg = &config.Config{
		Versioning: config.VersioningConfig{
			TagPrefix: "v",
			GitTag:    true,
			GitPush:   true,
		},
	}

	ver, _ := version.Parse("1.5.0")

	tests := []struct {
		name          string
		createTag     bool
		pushTag       bool
		dryRun        bool
		wantCreateTag bool
		wantPushTag   bool
		wantDryRun    bool
	}{
		{
			name:          "create and push tag",
			createTag:     true,
			pushTag:       true,
			dryRun:        false,
			wantCreateTag: true,
			wantPushTag:   true,
			wantDryRun:    false,
		},
		{
			name:          "create only",
			createTag:     true,
			pushTag:       false,
			dryRun:        false,
			wantCreateTag: true,
			wantPushTag:   false,
			wantDryRun:    false,
		},
		{
			name:          "dry run mode",
			createTag:     true,
			pushTag:       true,
			dryRun:        true,
			wantCreateTag: true,
			wantPushTag:   true,
			wantDryRun:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := buildSetVersionInput(ver, tt.createTag, tt.pushTag, tt.dryRun)

			if input.Version != ver {
				t.Errorf("buildSetVersionInput() Version = %v, want %v", input.Version, ver)
			}

			if input.CreateTag != tt.wantCreateTag {
				t.Errorf("buildSetVersionInput() CreateTag = %v, want %v", input.CreateTag, tt.wantCreateTag)
			}

			if input.PushTag != tt.wantPushTag {
				t.Errorf("buildSetVersionInput() PushTag = %v, want %v", input.PushTag, tt.wantPushTag)
			}

			if input.DryRun != tt.wantDryRun {
				t.Errorf("buildSetVersionInput() DryRun = %v, want %v", input.DryRun, tt.wantDryRun)
			}

			if input.Remote != "origin" {
				t.Errorf("buildSetVersionInput() Remote = %v, want origin", input.Remote)
			}

			if input.TagPrefix != "v" {
				t.Errorf("buildSetVersionInput() TagPrefix = %v, want v", input.TagPrefix)
			}
		})
	}
}

func TestBuildSetVersionInput_ConfigDisabled(t *testing.T) {
	// Setup test config with git features disabled
	originalCfg := cfg
	defer func() { cfg = originalCfg }()

	cfg = &config.Config{
		Versioning: config.VersioningConfig{
			TagPrefix: "release-",
			GitTag:    false,
			GitPush:   false,
		},
	}

	ver, _ := version.Parse("1.0.0")
	input := buildSetVersionInput(ver, true, true, false)

	// Even though we request createTag and pushTag, config disables them
	if input.CreateTag != false {
		t.Errorf("buildSetVersionInput() CreateTag should be false when GitTag is disabled")
	}

	if input.PushTag != false {
		t.Errorf("buildSetVersionInput() PushTag should be false when GitPush is disabled")
	}
}
