// Package cli provides the command-line interface for ReleasePilot.
package cli

import (
	"strings"
	"testing"

	"github.com/felixgeelhaar/release-pilot/internal/config"
	"github.com/felixgeelhaar/release-pilot/internal/domain/version"
)

func TestParseBumpLevel(t *testing.T) {
	tests := []struct {
		name        string
		level       string
		wantType    version.BumpType
		wantAuto    bool
		wantErr     bool
		errContains string
	}{
		{
			name:     "major bump",
			level:    "major",
			wantType: version.BumpMajor,
			wantAuto: false,
			wantErr:  false,
		},
		{
			name:     "minor bump",
			level:    "minor",
			wantType: version.BumpMinor,
			wantAuto: false,
			wantErr:  false,
		},
		{
			name:     "patch bump",
			level:    "patch",
			wantType: version.BumpPatch,
			wantAuto: false,
			wantErr:  false,
		},
		{
			name:     "empty string for auto-detection",
			level:    "",
			wantType: version.BumpType(""),
			wantAuto: true,
			wantErr:  false,
		},
		{
			name:        "invalid level",
			level:       "invalid",
			wantType:    version.BumpType(""),
			wantAuto:    false,
			wantErr:     true,
			errContains: "invalid bump level",
		},
		{
			name:        "typo in major",
			level:       "magor",
			wantType:    version.BumpType(""),
			wantAuto:    false,
			wantErr:     true,
			errContains: "invalid bump level",
		},
		{
			name:        "uppercase not supported",
			level:       "MAJOR",
			wantType:    version.BumpType(""),
			wantAuto:    false,
			wantErr:     true,
			errContains: "invalid bump level",
		},
		{
			name:        "numeric input",
			level:       "1",
			wantType:    version.BumpType(""),
			wantAuto:    false,
			wantErr:     true,
			errContains: "invalid bump level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotAuto, err := parseBumpLevel(tt.level)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseBumpLevel() expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("parseBumpLevel() error = %v, should contain %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("parseBumpLevel() unexpected error: %v", err)
				}
			}

			if gotType != tt.wantType {
				t.Errorf("parseBumpLevel() type = %v, want %v", gotType, tt.wantType)
			}

			if gotAuto != tt.wantAuto {
				t.Errorf("parseBumpLevel() auto = %v, want %v", gotAuto, tt.wantAuto)
			}
		})
	}
}

func TestBumpCommand_FlagsExist(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"level flag", "level"},
		{"prerelease flag", "prerelease"},
		{"build flag", "build"},
		{"force flag", "force"},
		{"tag flag", "tag"},
		{"push flag", "push"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := bumpCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("bump command missing %s flag", tt.flagName)
			}
		})
	}
}

func TestBumpCommand_TagFlagDefaultValue(t *testing.T) {
	flag := bumpCmd.Flags().Lookup("tag")
	if flag == nil {
		t.Fatal("tag flag not found")
	}
	// Default value should be true
	if flag.DefValue != "true" {
		t.Errorf("tag flag default = %v, want true", flag.DefValue)
	}
}

func TestBumpCommand_PushFlagDefaultValue(t *testing.T) {
	flag := bumpCmd.Flags().Lookup("push")
	if flag == nil {
		t.Fatal("push flag not found")
	}
	// Default value should be false
	if flag.DefValue != "false" {
		t.Errorf("push flag default = %v, want false", flag.DefValue)
	}
}

func TestBumpCommand_DescriptionContent(t *testing.T) {
	if bumpCmd.Short == "" {
		t.Error("bump command should have a short description")
	}
	if bumpCmd.Long == "" {
		t.Error("bump command should have a long description")
	}
}

func TestBumpCommand_HasAliases(t *testing.T) {
	if len(bumpCmd.Aliases) == 0 {
		t.Error("bump command should have aliases")
	}

	hasVersionBumpAlias := false
	for _, alias := range bumpCmd.Aliases {
		if alias == "version-bump" {
			hasVersionBumpAlias = true
			break
		}
	}

	if !hasVersionBumpAlias {
		t.Error("bump command should have 'version-bump' alias")
	}
}

func TestBuildSetVersionInput(t *testing.T) {
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

	ver, _ := version.Parse("1.2.3")

	tests := []struct {
		name       string
		ver        version.SemanticVersion
		createTag  bool
		pushTag    bool
		dryRunMode bool
		wantTag    bool
		wantPush   bool
	}{
		{
			name:       "create and push tag",
			ver:        ver,
			createTag:  true,
			pushTag:    true,
			dryRunMode: false,
			wantTag:    true,
			wantPush:   true,
		},
		{
			name:       "create tag only",
			ver:        ver,
			createTag:  true,
			pushTag:    false,
			dryRunMode: false,
			wantTag:    true,
			wantPush:   false,
		},
		{
			name:       "no tag creation",
			ver:        ver,
			createTag:  false,
			pushTag:    false,
			dryRunMode: false,
			wantTag:    false,
			wantPush:   false,
		},
		{
			name:       "dry run mode",
			ver:        ver,
			createTag:  true,
			pushTag:    true,
			dryRunMode: true,
			wantTag:    true,
			wantPush:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := buildSetVersionInput(tt.ver, tt.createTag, tt.pushTag, tt.dryRunMode)

			if input.Version.String() != tt.ver.String() {
				t.Errorf("buildSetVersionInput() version = %v, want %v", input.Version, tt.ver)
			}

			if input.CreateTag != tt.wantTag {
				t.Errorf("buildSetVersionInput() CreateTag = %v, want %v", input.CreateTag, tt.wantTag)
			}

			if input.PushTag != tt.wantPush {
				t.Errorf("buildSetVersionInput() PushTag = %v, want %v", input.PushTag, tt.wantPush)
			}

			if input.DryRun != tt.dryRunMode {
				t.Errorf("buildSetVersionInput() DryRun = %v, want %v", input.DryRun, tt.dryRunMode)
			}

			if input.TagPrefix != "v" {
				t.Errorf("buildSetVersionInput() TagPrefix = %v, want v", input.TagPrefix)
			}

			expectedMessage := "Release 1.2.3"
			if input.TagMessage != expectedMessage {
				t.Errorf("buildSetVersionInput() TagMessage = %v, want %v", input.TagMessage, expectedMessage)
			}
		})
	}
}

func TestBuildCalculateVersionInput(t *testing.T) {
	// Setup test config
	originalCfg := cfg
	defer func() { cfg = originalCfg }()

	cfg = &config.Config{
		Versioning: config.VersioningConfig{
			TagPrefix: "v",
		},
	}

	// Save and restore bumpPrerelease
	originalPrerelease := bumpPrerelease
	defer func() { bumpPrerelease = originalPrerelease }()

	tests := []struct {
		name           string
		bumpType       version.BumpType
		auto           bool
		prerelease     string
		wantPrerelease bool
	}{
		{
			name:           "major bump without prerelease",
			bumpType:       version.BumpMajor,
			auto:           false,
			prerelease:     "",
			wantPrerelease: false,
		},
		{
			name:           "minor bump with prerelease",
			bumpType:       version.BumpMinor,
			auto:           false,
			prerelease:     "beta.1",
			wantPrerelease: true,
		},
		{
			name:           "auto detection",
			bumpType:       version.BumpType(""),
			auto:           true,
			prerelease:     "",
			wantPrerelease: false,
		},
		{
			name:           "patch with alpha prerelease",
			bumpType:       version.BumpPatch,
			auto:           false,
			prerelease:     "alpha",
			wantPrerelease: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bumpPrerelease = tt.prerelease
			input := buildCalculateVersionInput(tt.bumpType, tt.auto)

			if input.BumpType != tt.bumpType {
				t.Errorf("buildCalculateVersionInput() BumpType = %v, want %v", input.BumpType, tt.bumpType)
			}

			if input.Auto != tt.auto {
				t.Errorf("buildCalculateVersionInput() Auto = %v, want %v", input.Auto, tt.auto)
			}

			if input.TagPrefix != "v" {
				t.Errorf("buildCalculateVersionInput() TagPrefix = %v, want v", input.TagPrefix)
			}

			if tt.wantPrerelease {
				if string(input.Prerelease) != tt.prerelease {
					t.Errorf("buildCalculateVersionInput() Prerelease = %v, want %v", input.Prerelease, tt.prerelease)
				}
			} else {
				if input.Prerelease != "" {
					t.Errorf("buildCalculateVersionInput() Prerelease = %v, want empty", input.Prerelease)
				}
			}
		})
	}
}

func TestOutputBumpJSON(t *testing.T) {
	// Setup test config
	originalCfg := cfg
	defer func() { cfg = originalCfg }()

	cfg = &config.Config{
		Versioning: config.VersioningConfig{
			TagPrefix: "v",
		},
	}

	current, _ := version.Parse("1.2.3")
	next, _ := version.Parse("1.3.0")

	tests := []struct {
		name         string
		current      version.SemanticVersion
		next         version.SemanticVersion
		bumpType     version.BumpType
		autoDetected bool
	}{
		{
			name:         "minor bump auto-detected",
			current:      current,
			next:         next,
			bumpType:     version.BumpMinor,
			autoDetected: true,
		},
		{
			name:         "manual major bump",
			current:      current,
			next:         next,
			bumpType:     version.BumpMajor,
			autoDetected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic and returns no error
			err := outputBumpJSON(tt.current, tt.next, tt.bumpType, tt.autoDetected)
			if err != nil {
				t.Errorf("outputBumpJSON() error = %v", err)
			}
		})
	}
}
