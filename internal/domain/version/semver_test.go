package version

import (
	"testing"
)

func TestNewSemanticVersion(t *testing.T) {
	tests := []struct {
		name  string
		major uint64
		minor uint64
		patch uint64
		want  string
	}{
		{"zero version", 0, 0, 0, "0.0.0"},
		{"initial version", 0, 1, 0, "0.1.0"},
		{"stable version", 1, 0, 0, "1.0.0"},
		{"patch version", 1, 2, 3, "1.2.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewSemanticVersion(tt.major, tt.minor, tt.patch)
			if got := v.String(); got != tt.want {
				t.Errorf("NewSemanticVersion().String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"simple version", "1.2.3", "1.2.3", false},
		{"with v prefix", "v1.2.3", "1.2.3", false},
		{"with prerelease", "1.2.3-alpha", "1.2.3-alpha", false},
		{"with metadata", "1.2.3+build", "1.2.3+build", false},
		{"with prerelease and metadata", "1.2.3-beta.1+build.123", "1.2.3-beta.1+build.123", false},
		{"zero version", "0.0.0", "0.0.0", false},
		{"large numbers", "100.200.300", "100.200.300", false},
		{"invalid - empty", "", "", true},
		{"invalid - not a version", "foo", "", true},
		{"invalid - missing patch", "1.2", "", true},
		{"invalid - letters in version", "1.a.3", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.String() != tt.want {
				t.Errorf("Parse().String() = %v, want %v", got.String(), tt.want)
			}
		})
	}
}

func TestSemanticVersion_Compare(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v1   string
		v2   string
		want int
	}{
		{"equal versions", "1.0.0", "1.0.0", 0},
		{"major less", "1.0.0", "2.0.0", -1},
		{"major greater", "2.0.0", "1.0.0", 1},
		{"minor less", "1.1.0", "1.2.0", -1},
		{"minor greater", "1.2.0", "1.1.0", 1},
		{"patch less", "1.0.1", "1.0.2", -1},
		{"patch greater", "1.0.2", "1.0.1", 1},
		{"prerelease vs stable", "1.0.0-alpha", "1.0.0", -1},
		{"stable vs prerelease", "1.0.0", "1.0.0-alpha", 1},
		{"prerelease ordering", "1.0.0-alpha", "1.0.0-beta", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v1 := MustParse(tt.v1)
			v2 := MustParse(tt.v2)
			if got := v1.Compare(v2); got != tt.want {
				t.Errorf("Compare() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSemanticVersion_IsStable(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{"stable 1.0.0", "1.0.0", true},
		{"stable 2.0.0", "2.0.0", true},
		{"unstable 0.1.0", "0.1.0", false},
		{"prerelease 1.0.0-alpha", "1.0.0-alpha", false},
		{"zero version", "0.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := MustParse(tt.version)
			if got := v.IsStable(); got != tt.want {
				t.Errorf("IsStable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSemanticVersion_WithPrerelease(t *testing.T) {
	v := MustParse("1.2.3")
	got := v.WithPrerelease(PrereleaseAlpha)

	if got.String() != "1.2.3-alpha" {
		t.Errorf("WithPrerelease() = %v, want 1.2.3-alpha", got.String())
	}

	// Verify original is unchanged (immutability)
	if v.String() != "1.2.3" {
		t.Errorf("Original version modified, got %v, want 1.2.3", v.String())
	}
}

func TestSemanticVersion_WithMetadata(t *testing.T) {
	v := MustParse("1.2.3")
	got := v.WithMetadata("build.123")

	if got.String() != "1.2.3+build.123" {
		t.Errorf("WithMetadata() = %v, want 1.2.3+build.123", got.String())
	}

	// Verify original is unchanged (immutability)
	if v.String() != "1.2.3" {
		t.Errorf("Original version modified, got %v, want 1.2.3", v.String())
	}
}

func TestSemanticVersion_TagString(t *testing.T) {
	v := MustParse("1.2.3")
	if got := v.TagString(); got != "v1.2.3" {
		t.Errorf("TagString() = %v, want v1.2.3", got)
	}
}

func TestSemanticVersion_Accessors(t *testing.T) {
	v := MustParse("1.2.3-alpha+build")

	if v.Major() != 1 {
		t.Errorf("Major() = %v, want 1", v.Major())
	}
	if v.Minor() != 2 {
		t.Errorf("Minor() = %v, want 2", v.Minor())
	}
	if v.Patch() != 3 {
		t.Errorf("Patch() = %v, want 3", v.Patch())
	}
	if v.Prerelease() != "alpha" {
		t.Errorf("Prerelease() = %v, want alpha", v.Prerelease())
	}
	if v.Metadata() != "build" {
		t.Errorf("Metadata() = %v, want build", v.Metadata())
	}
}

func TestSemanticVersion_Comparison(t *testing.T) {
	v1 := MustParse("1.0.0")
	v2 := MustParse("2.0.0")

	if !v1.LessThan(v2) {
		t.Error("LessThan() failed: 1.0.0 should be less than 2.0.0")
	}
	if !v1.LessThanOrEqual(v2) {
		t.Error("LessThanOrEqual() failed")
	}
	if !v2.GreaterThan(v1) {
		t.Error("GreaterThan() failed: 2.0.0 should be greater than 1.0.0")
	}
	if !v2.GreaterThanOrEqual(v1) {
		t.Error("GreaterThanOrEqual() failed")
	}
	if !v1.Equal(MustParse("1.0.0")) {
		t.Error("Equal() failed")
	}
}

func TestSemanticVersion_Equals(t *testing.T) {
	v1 := MustParse("1.0.0+build1")
	v2 := MustParse("1.0.0+build2")

	// Equal() ignores metadata per semver spec
	if !v1.Equal(v2) {
		t.Error("Equal() should return true - metadata is ignored")
	}

	// Equals() includes metadata
	if v1.Equals(v2) {
		t.Error("Equals() should return false - metadata differs")
	}
}

func TestSemanticVersion_IsZero(t *testing.T) {
	zero := Zero
	if !zero.IsZero() {
		t.Error("Zero.IsZero() should return true")
	}

	nonZero := MustParse("0.0.1")
	if nonZero.IsZero() {
		t.Error("0.0.1.IsZero() should return false")
	}
}
