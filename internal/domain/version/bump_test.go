// Package version provides domain types for semantic versioning.
package version

import (
	"testing"
)

func TestBumpType_IsValid(t *testing.T) {
	validTypes := []BumpType{
		BumpMajor,
		BumpMinor,
		BumpPatch,
		BumpPrerelease,
	}

	for _, bt := range validTypes {
		if !bt.IsValid() {
			t.Errorf("IsValid() = false for %s, want true", bt)
		}
	}

	invalidTypes := []BumpType{
		"invalid",
		"",
		"MAJOR",
		"big",
	}

	for _, bt := range invalidTypes {
		if bt.IsValid() {
			t.Errorf("IsValid() = true for %q, want false", bt)
		}
	}
}

func TestBumpType_String(t *testing.T) {
	tests := []struct {
		bt       BumpType
		expected string
	}{
		{BumpMajor, "major"},
		{BumpMinor, "minor"},
		{BumpPatch, "patch"},
		{BumpPrerelease, "prerelease"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.bt.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseBumpType(t *testing.T) {
	tests := []struct {
		input   string
		wantBT  BumpType
		wantErr bool
	}{
		{"major", BumpMajor, false},
		{"minor", BumpMinor, false},
		{"patch", BumpPatch, false},
		{"prerelease", BumpPrerelease, false},
		{"invalid", "", true},
		{"", "", true},
		{"MAJOR", "", true}, // Not case-insensitive
		{"big", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			bt, err := ParseBumpType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseBumpType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && bt != tt.wantBT {
				t.Errorf("ParseBumpType(%q) = %v, want %v", tt.input, bt, tt.wantBT)
			}
		})
	}
}

func TestNewVersionBump(t *testing.T) {
	tests := []BumpType{BumpMajor, BumpMinor, BumpPatch}

	for _, bt := range tests {
		t.Run(string(bt), func(t *testing.T) {
			bump := NewVersionBump(bt)
			if bump.Type() != bt {
				t.Errorf("Type() = %v, want %v", bump.Type(), bt)
			}
			if bump.PrereleaseIdentifier() != "" {
				t.Errorf("PrereleaseIdentifier() = %v, want empty", bump.PrereleaseIdentifier())
			}
		})
	}
}

func TestNewPrereleaseBump(t *testing.T) {
	prereleases := []Prerelease{PrereleaseAlpha, PrereleaseBeta, PrereleaseRC}

	for _, pre := range prereleases {
		t.Run(string(pre), func(t *testing.T) {
			bump := NewPrereleaseBump(pre)
			if bump.Type() != BumpPrerelease {
				t.Errorf("Type() = %v, want prerelease", bump.Type())
			}
			if bump.PrereleaseIdentifier() != pre {
				t.Errorf("PrereleaseIdentifier() = %v, want %v", bump.PrereleaseIdentifier(), pre)
			}
		})
	}
}

func TestVersionBump_Apply_Major(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"simple", "1.2.3", "2.0.0"},
		{"from zero", "0.1.0", "1.0.0"},
		{"with prerelease", "1.2.3-alpha", "2.0.0"},
		{"with metadata", "1.2.3+build", "2.0.0"},
		{"large version", "99.88.77", "100.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := MustParse(tt.version)
			bump := NewVersionBump(BumpMajor)
			got := bump.Apply(v)
			if got.String() != tt.want {
				t.Errorf("Apply(%s) = %v, want %v", tt.version, got.String(), tt.want)
			}
		})
	}
}

func TestVersionBump_Apply_Minor(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"simple", "1.2.3", "1.3.0"},
		{"from zero", "0.0.0", "0.1.0"},
		{"with prerelease", "1.2.3-alpha", "1.3.0"},
		{"with metadata", "1.2.3+build", "1.3.0"},
		{"large version", "1.99.99", "1.100.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := MustParse(tt.version)
			bump := NewVersionBump(BumpMinor)
			got := bump.Apply(v)
			if got.String() != tt.want {
				t.Errorf("Apply(%s) = %v, want %v", tt.version, got.String(), tt.want)
			}
		})
	}
}

func TestVersionBump_Apply_Patch(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"simple", "1.2.3", "1.2.4"},
		{"from zero", "0.0.0", "0.0.1"},
		{"with prerelease - releases it", "1.2.3-alpha", "1.2.3"},
		{"with metadata", "1.2.3+build", "1.2.4"},
		{"large version", "1.2.99", "1.2.100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := MustParse(tt.version)
			bump := NewVersionBump(BumpPatch)
			got := bump.Apply(v)
			if got.String() != tt.want {
				t.Errorf("Apply(%s) = %v, want %v", tt.version, got.String(), tt.want)
			}
		})
	}
}

func TestVersionBump_Apply_Prerelease(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		prerelease Prerelease
		want       string
	}{
		{"add alpha to stable", "1.2.3", PrereleaseAlpha, "1.3.0-alpha"},
		{"add beta to stable", "1.2.3", PrereleaseBeta, "1.3.0-beta"},
		{"add rc to stable", "1.2.3", PrereleaseRC, "1.3.0-rc"},
		{"update alpha to beta", "1.3.0-alpha", PrereleaseBeta, "1.3.0-beta"},
		{"same prerelease", "1.3.0-alpha", PrereleaseAlpha, "1.3.0-alpha"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := MustParse(tt.version)
			bump := NewPrereleaseBump(tt.prerelease)
			got := bump.Apply(v)
			if got.String() != tt.want {
				t.Errorf("Apply(%s) = %v, want %v", tt.version, got.String(), tt.want)
			}
		})
	}
}

func TestVersionBump_Apply_PrereleaseEmpty(t *testing.T) {
	// When prerelease identifier is empty, version should remain unchanged
	v := MustParse("1.2.3")
	bump := VersionBump{bumpType: BumpPrerelease, prerelease: ""}
	got := bump.Apply(v)
	if got.String() != "1.2.3" {
		t.Errorf("Apply() = %v, want 1.2.3 (unchanged)", got.String())
	}
}

func TestVersionBump_Apply_UnknownType(t *testing.T) {
	// Unknown bump type should return version unchanged
	v := MustParse("1.2.3")
	bump := VersionBump{bumpType: BumpType("unknown")}
	got := bump.Apply(v)
	if got.String() != "1.2.3" {
		t.Errorf("Apply() = %v, want 1.2.3 (unchanged)", got.String())
	}
}

func TestBumpMajorVersion(t *testing.T) {
	v := MustParse("1.2.3")
	got := BumpMajorVersion(v)
	if got.String() != "2.0.0" {
		t.Errorf("BumpMajorVersion() = %v, want 2.0.0", got.String())
	}
}

func TestBumpMinorVersion(t *testing.T) {
	v := MustParse("1.2.3")
	got := BumpMinorVersion(v)
	if got.String() != "1.3.0" {
		t.Errorf("BumpMinorVersion() = %v, want 1.3.0", got.String())
	}
}

func TestBumpPatchVersion(t *testing.T) {
	v := MustParse("1.2.3")
	got := BumpPatchVersion(v)
	if got.String() != "1.2.4" {
		t.Errorf("BumpPatchVersion() = %v, want 1.2.4", got.String())
	}
}

func TestNewDefaultVersionCalculator(t *testing.T) {
	calc := NewDefaultVersionCalculator()
	if calc == nil {
		t.Error("NewDefaultVersionCalculator() returned nil")
	}
}

func TestDefaultVersionCalculator_CalculateNextVersion(t *testing.T) {
	calc := NewDefaultVersionCalculator()
	v := MustParse("1.2.3")

	tests := []struct {
		name     string
		bump     BumpType
		expected string
	}{
		{"major", BumpMajor, "2.0.0"},
		{"minor", BumpMinor, "1.3.0"},
		{"patch", BumpPatch, "1.2.4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calc.CalculateNextVersion(v, tt.bump)
			if got.String() != tt.expected {
				t.Errorf("CalculateNextVersion() = %v, want %v", got.String(), tt.expected)
			}
		})
	}
}

func TestDefaultVersionCalculator_DetermineRequiredBump(t *testing.T) {
	calc := NewDefaultVersionCalculator()

	tests := []struct {
		name        string
		hasBreaking bool
		hasFeature  bool
		hasFix      bool
		expected    BumpType
	}{
		{"breaking change", true, false, false, BumpMajor},
		{"breaking with feature", true, true, false, BumpMajor},
		{"breaking with fix", true, false, true, BumpMajor},
		{"breaking with all", true, true, true, BumpMajor},
		{"feature only", false, true, false, BumpMinor},
		{"feature with fix", false, true, true, BumpMinor},
		{"fix only", false, false, true, BumpPatch},
		{"no changes", false, false, false, BumpPatch}, // Default to patch
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calc.DetermineRequiredBump(tt.hasBreaking, tt.hasFeature, tt.hasFix)
			if got != tt.expected {
				t.Errorf("DetermineRequiredBump(%v, %v, %v) = %v, want %v",
					tt.hasBreaking, tt.hasFeature, tt.hasFix, got, tt.expected)
			}
		})
	}
}

func TestVersionBump_Immutability(t *testing.T) {
	// Ensure Apply returns a new version and doesn't modify the original
	original := MustParse("1.2.3")
	bump := NewVersionBump(BumpMajor)

	_ = bump.Apply(original)

	if original.String() != "1.2.3" {
		t.Errorf("Original version was modified: got %v, want 1.2.3", original.String())
	}
}
