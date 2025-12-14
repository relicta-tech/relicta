// Package changes provides domain types for analyzing commit changes.
package changes

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestReleaseType_String(t *testing.T) {
	tests := []struct {
		rt       ReleaseType
		expected string
	}{
		{ReleaseTypeMajor, "major"},
		{ReleaseTypeMinor, "minor"},
		{ReleaseTypePatch, "patch"},
		{ReleaseTypeNone, "none"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.rt.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestReleaseType_IsValid(t *testing.T) {
	validTypes := []ReleaseType{
		ReleaseTypeMajor,
		ReleaseTypeMinor,
		ReleaseTypePatch,
		ReleaseTypeNone,
	}

	for _, rt := range validTypes {
		if !rt.IsValid() {
			t.Errorf("IsValid() = false for %s, want true", rt)
		}
	}

	invalidTypes := []ReleaseType{
		"invalid",
		"",
		"MAJOR",
		"big",
	}

	for _, rt := range invalidTypes {
		if rt.IsValid() {
			t.Errorf("IsValid() = true for %q, want false", rt)
		}
	}
}

func TestReleaseType_Description(t *testing.T) {
	tests := []struct {
		rt       ReleaseType
		expected string
	}{
		{ReleaseTypeMajor, "Major release with breaking changes"},
		{ReleaseTypeMinor, "Minor release with new features"},
		{ReleaseTypePatch, "Patch release with bug fixes"},
		{ReleaseTypeNone, "No release needed"},
	}

	for _, tt := range tests {
		t.Run(string(tt.rt), func(t *testing.T) {
			if got := tt.rt.Description(); got != tt.expected {
				t.Errorf("Description() = %q, want %q", got, tt.expected)
			}
		})
	}

	// Test unknown type
	unknown := ReleaseType("unknown")
	if got := unknown.Description(); got != "Unknown release type" {
		t.Errorf("Description() for unknown = %q, want 'Unknown release type'", got)
	}
}

func TestReleaseType_ToBumpType(t *testing.T) {
	tests := []struct {
		rt       ReleaseType
		expected version.BumpType
	}{
		{ReleaseTypeMajor, version.BumpMajor},
		{ReleaseTypeMinor, version.BumpMinor},
		{ReleaseTypePatch, version.BumpPatch},
		{ReleaseTypeNone, version.BumpPatch}, // Default to patch
	}

	for _, tt := range tests {
		t.Run(string(tt.rt), func(t *testing.T) {
			if got := tt.rt.ToBumpType(); got != tt.expected {
				t.Errorf("ToBumpType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseReleaseType(t *testing.T) {
	tests := []struct {
		input   string
		wantRT  ReleaseType
		wantErr bool
	}{
		{"major", ReleaseTypeMajor, false},
		{"minor", ReleaseTypeMinor, false},
		{"patch", ReleaseTypePatch, false},
		{"none", ReleaseTypeNone, false},
		{"MAJOR", ReleaseTypeMajor, false},     // Case insensitive
		{"  minor  ", ReleaseTypeMinor, false}, // Trim spaces
		{"  PATCH  ", ReleaseTypePatch, false}, // Both
		{"invalid", "", true},
		{"", "", true},
		{"big", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			rt, err := ParseReleaseType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseReleaseType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && rt != tt.wantRT {
				t.Errorf("ParseReleaseType(%q) = %v, want %v", tt.input, rt, tt.wantRT)
			}
		})
	}
}

func TestReleaseTypeFromCommitType(t *testing.T) {
	tests := []struct {
		name       string
		commitType CommitType
		isBreaking bool
		expected   ReleaseType
	}{
		// Breaking changes always result in major
		{"breaking feat", CommitTypeFeat, true, ReleaseTypeMajor},
		{"breaking fix", CommitTypeFix, true, ReleaseTypeMajor},
		{"breaking docs", CommitTypeDocs, true, ReleaseTypeMajor},
		{"breaking chore", CommitTypeChore, true, ReleaseTypeMajor},

		// Non-breaking changes by type
		{"feat", CommitTypeFeat, false, ReleaseTypeMinor},
		{"fix", CommitTypeFix, false, ReleaseTypePatch},
		{"perf", CommitTypePerf, false, ReleaseTypePatch},
		{"docs", CommitTypeDocs, false, ReleaseTypeNone},
		{"style", CommitTypeStyle, false, ReleaseTypeNone},
		{"refactor", CommitTypeRefactor, false, ReleaseTypeNone},
		{"test", CommitTypeTest, false, ReleaseTypeNone},
		{"build", CommitTypeBuild, false, ReleaseTypeNone},
		{"ci", CommitTypeCI, false, ReleaseTypeNone},
		{"chore", CommitTypeChore, false, ReleaseTypeNone},
		{"revert", CommitTypeRevert, false, ReleaseTypeNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReleaseTypeFromCommitType(tt.commitType, tt.isBreaking)
			if got != tt.expected {
				t.Errorf("ReleaseTypeFromCommitType(%s, %v) = %v, want %v",
					tt.commitType, tt.isBreaking, got, tt.expected)
			}
		})
	}
}

func TestMaxReleaseType(t *testing.T) {
	tests := []struct {
		name     string
		a        ReleaseType
		b        ReleaseType
		expected ReleaseType
	}{
		// Major wins over all
		{"major vs minor", ReleaseTypeMajor, ReleaseTypeMinor, ReleaseTypeMajor},
		{"major vs patch", ReleaseTypeMajor, ReleaseTypePatch, ReleaseTypeMajor},
		{"major vs none", ReleaseTypeMajor, ReleaseTypeNone, ReleaseTypeMajor},
		{"minor vs major", ReleaseTypeMinor, ReleaseTypeMajor, ReleaseTypeMajor},
		{"patch vs major", ReleaseTypePatch, ReleaseTypeMajor, ReleaseTypeMajor},
		{"none vs major", ReleaseTypeNone, ReleaseTypeMajor, ReleaseTypeMajor},

		// Minor wins over patch and none
		{"minor vs patch", ReleaseTypeMinor, ReleaseTypePatch, ReleaseTypeMinor},
		{"minor vs none", ReleaseTypeMinor, ReleaseTypeNone, ReleaseTypeMinor},
		{"patch vs minor", ReleaseTypePatch, ReleaseTypeMinor, ReleaseTypeMinor},
		{"none vs minor", ReleaseTypeNone, ReleaseTypeMinor, ReleaseTypeMinor},

		// Patch wins over none
		{"patch vs none", ReleaseTypePatch, ReleaseTypeNone, ReleaseTypePatch},
		{"none vs patch", ReleaseTypeNone, ReleaseTypePatch, ReleaseTypePatch},

		// Same types
		{"major vs major", ReleaseTypeMajor, ReleaseTypeMajor, ReleaseTypeMajor},
		{"minor vs minor", ReleaseTypeMinor, ReleaseTypeMinor, ReleaseTypeMinor},
		{"patch vs patch", ReleaseTypePatch, ReleaseTypePatch, ReleaseTypePatch},
		{"none vs none", ReleaseTypeNone, ReleaseTypeNone, ReleaseTypeNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaxReleaseType(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("MaxReleaseType(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}
