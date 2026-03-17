package version

import (
	"sort"
	"testing"
)

func TestComparePrerelease_SemverSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v1   string
		v2   string
		want int
	}{
		// Basic ordering: alpha < beta < rc < stable
		{"alpha < beta", "1.0.0-alpha", "1.0.0-beta", -1},
		{"beta < rc", "1.0.0-beta", "1.0.0-rc", -1},
		{"rc < stable", "1.0.0-rc", "1.0.0", -1},
		{"alpha < stable", "1.0.0-alpha", "1.0.0", -1},

		// Reverse ordering
		{"beta > alpha", "1.0.0-beta", "1.0.0-alpha", 1},
		{"stable > rc", "1.0.0", "1.0.0-rc", 1},

		// Dotted numeric identifiers
		{"alpha.1 < alpha.2", "1.0.0-alpha.1", "1.0.0-alpha.2", -1},
		{"alpha.2 < alpha.10", "1.0.0-alpha.2", "1.0.0-alpha.10", -1},
		{"alpha.1 < alpha.11", "1.0.0-alpha.1", "1.0.0-alpha.11", -1},
		{"rc.1 < rc.2", "1.0.0-rc.1", "1.0.0-rc.2", -1},
		{"beta.1 < beta.2", "1.0.0-beta.1", "1.0.0-beta.2", -1},

		// Mixed identifiers
		{"alpha.1 < beta.1", "1.0.0-alpha.1", "1.0.0-beta.1", -1},
		{"beta.5 < rc.1", "1.0.0-beta.5", "1.0.0-rc.1", -1},
		{"alpha.99 < beta.1", "1.0.0-alpha.99", "1.0.0-beta.1", -1},

		// Numeric vs alphanumeric precedence
		// Per semver spec: numeric identifiers have lower precedence than alphanumeric
		{"numeric < alpha", "1.0.0-1", "1.0.0-alpha", -1},

		// Shorter set < longer set when all preceding are equal
		{"alpha < alpha.1", "1.0.0-alpha", "1.0.0-alpha.1", -1},
		{"alpha.1 > alpha", "1.0.0-alpha.1", "1.0.0-alpha", 1},

		// Equal pre-releases
		{"equal alpha", "1.0.0-alpha", "1.0.0-alpha", 0},
		{"equal alpha.1", "1.0.0-alpha.1", "1.0.0-alpha.1", 0},
		{"equal rc.3", "1.0.0-rc.3", "1.0.0-rc.3", 0},

		// Different major.minor.patch with pre-releases
		{"1.0.0-alpha < 1.0.1-alpha", "1.0.0-alpha", "1.0.1-alpha", -1},
		{"1.0.0-rc.1 < 1.1.0-alpha.1", "1.0.0-rc.1", "1.1.0-alpha.1", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v1 := MustParse(tt.v1)
			v2 := MustParse(tt.v2)
			got := v1.Compare(v2)
			if got != tt.want {
				t.Errorf("Compare(%s, %s) = %d, want %d", tt.v1, tt.v2, got, tt.want)
			}
		})
	}
}

func TestSortPrerelease(t *testing.T) {
	t.Parallel()

	// Verify sort ordering matches semver specification
	versions := []string{
		"1.0.0",
		"1.0.0-rc.2",
		"1.0.0-alpha.1",
		"1.0.0-beta.1",
		"1.0.0-alpha.2",
		"1.0.0-rc.1",
		"1.0.0-beta.2",
		"1.0.0-alpha",
	}

	expected := []string{
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0-alpha.2",
		"1.0.0-beta.1",
		"1.0.0-beta.2",
		"1.0.0-rc.1",
		"1.0.0-rc.2",
		"1.0.0",
	}

	parsed := make([]SemanticVersion, len(versions))
	for i, v := range versions {
		parsed[i] = MustParse(v)
	}

	sort.Slice(parsed, func(i, j int) bool {
		return parsed[i].Compare(parsed[j]) < 0
	})

	for i, v := range parsed {
		if v.String() != expected[i] {
			t.Errorf("sorted[%d] = %s, want %s", i, v.String(), expected[i])
		}
	}
}

func TestBumpPreRelease(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		preType Prerelease
		want    string
	}{
		// From stable: bump minor and start at .1
		{"stable to alpha", "1.2.3", PrereleaseAlpha, "1.3.0-alpha.1"},
		{"stable to beta", "1.2.3", PrereleaseBeta, "1.3.0-beta.1"},
		{"stable to rc", "1.2.3", PrereleaseRC, "1.3.0-rc.1"},

		// Same prerelease type: increment counter
		{"alpha.1 to alpha.2", "1.3.0-alpha.1", PrereleaseAlpha, "1.3.0-alpha.2"},
		{"alpha.5 to alpha.6", "1.3.0-alpha.5", PrereleaseAlpha, "1.3.0-alpha.6"},
		{"beta.1 to beta.2", "1.3.0-beta.1", PrereleaseBeta, "1.3.0-beta.2"},
		{"rc.1 to rc.2", "1.3.0-rc.1", PrereleaseRC, "1.3.0-rc.2"},
		{"rc.9 to rc.10", "1.3.0-rc.9", PrereleaseRC, "1.3.0-rc.10"},

		// Different prerelease type: start at .1 with same version
		{"alpha to beta", "1.3.0-alpha.2", PrereleaseBeta, "1.3.0-beta.1"},
		{"beta to rc", "1.3.0-beta.3", PrereleaseRC, "1.3.0-rc.1"},
		{"alpha to rc", "1.3.0-alpha.5", PrereleaseRC, "1.3.0-rc.1"},

		// From zero version
		{"zero to alpha", "0.0.0", PrereleaseAlpha, "0.1.0-alpha.1"},

		// Empty preType returns unchanged
		{"empty preType", "1.2.3", "", "1.2.3"},

		// Prerelease without counter
		{"alpha without counter to alpha", "1.3.0-alpha", PrereleaseAlpha, "1.3.0-alpha.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := MustParse(tt.version)
			got := v.BumpPreRelease(tt.preType)
			if got.String() != tt.want {
				t.Errorf("BumpPreRelease(%s, %s) = %s, want %s", tt.version, tt.preType, got.String(), tt.want)
			}
		})
	}
}

func TestBumpPreRelease_Immutability(t *testing.T) {
	t.Parallel()

	original := MustParse("1.2.3")
	_ = original.BumpPreRelease(PrereleaseAlpha)

	if original.String() != "1.2.3" {
		t.Errorf("original version was modified: got %s, want 1.2.3", original.String())
	}
}

func TestPromoteToRelease(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"alpha to release", "1.3.0-alpha.1", "1.3.0"},
		{"beta to release", "1.3.0-beta.2", "1.3.0"},
		{"rc to release", "1.3.0-rc.1", "1.3.0"},
		{"already stable", "1.3.0", "1.3.0"},
		{"canary to release", "2.0.0-canary.5", "2.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := MustParse(tt.version)
			got := v.PromoteToRelease()
			if got.String() != tt.want {
				t.Errorf("PromoteToRelease(%s) = %s, want %s", tt.version, got.String(), tt.want)
			}
			// Ensure it's not a prerelease
			if got.IsPrerelease() {
				t.Errorf("PromoteToRelease(%s) should not be a prerelease", tt.version)
			}
		})
	}
}

func TestPrereleaseType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    Prerelease
	}{
		{"alpha.1", "1.0.0-alpha.1", PrereleaseAlpha},
		{"beta.2", "1.0.0-beta.2", PrereleaseBeta},
		{"rc.1", "1.0.0-rc.1", PrereleaseRC},
		{"alpha", "1.0.0-alpha", PrereleaseAlpha},
		{"stable", "1.0.0", ""},
		{"canary.3", "1.0.0-canary.3", "canary"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := MustParse(tt.version)
			got := v.PrereleaseType()
			if got != tt.want {
				t.Errorf("PrereleaseType(%s) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}

func TestPrereleaseNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    uint64
	}{
		{"alpha.1", "1.0.0-alpha.1", 1},
		{"alpha.10", "1.0.0-alpha.10", 10},
		{"rc.3", "1.0.0-rc.3", 3},
		{"no counter", "1.0.0-alpha", 0},
		{"stable", "1.0.0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := MustParse(tt.version)
			got := v.PrereleaseNumber()
			if got != tt.want {
				t.Errorf("PrereleaseNumber(%s) = %d, want %d", tt.version, got, tt.want)
			}
		})
	}
}

func TestBumpPreRelease_FullWorkflow(t *testing.T) {
	t.Parallel()

	// Simulate a full pre-release workflow:
	// 1.2.3 -> 1.3.0-alpha.1 -> alpha.2 -> beta.1 -> beta.2 -> rc.1 -> rc.2 -> 1.3.0
	v := MustParse("1.2.3")

	// Start alpha
	v = v.BumpPreRelease(PrereleaseAlpha)
	assertVersion(t, v, "1.3.0-alpha.1")

	// More alpha
	v = v.BumpPreRelease(PrereleaseAlpha)
	assertVersion(t, v, "1.3.0-alpha.2")

	// Promote to beta
	v = v.BumpPreRelease(PrereleaseBeta)
	assertVersion(t, v, "1.3.0-beta.1")

	// More beta
	v = v.BumpPreRelease(PrereleaseBeta)
	assertVersion(t, v, "1.3.0-beta.2")

	// Promote to rc
	v = v.BumpPreRelease(PrereleaseRC)
	assertVersion(t, v, "1.3.0-rc.1")

	// More rc
	v = v.BumpPreRelease(PrereleaseRC)
	assertVersion(t, v, "1.3.0-rc.2")

	// Promote to stable
	v = v.PromoteToRelease()
	assertVersion(t, v, "1.3.0")

	// Verify ordering through the workflow
	versions := []SemanticVersion{
		MustParse("1.3.0"),
		MustParse("1.3.0-rc.2"),
		MustParse("1.3.0-rc.1"),
		MustParse("1.3.0-beta.2"),
		MustParse("1.3.0-beta.1"),
		MustParse("1.3.0-alpha.2"),
		MustParse("1.3.0-alpha.1"),
		MustParse("1.2.3"),
	}

	for i := 0; i < len(versions)-1; i++ {
		if !versions[i].GreaterThan(versions[i+1]) {
			t.Errorf("expected %s > %s", versions[i].String(), versions[i+1].String())
		}
	}
}

func assertVersion(t *testing.T, v SemanticVersion, expected string) {
	t.Helper()
	if v.String() != expected {
		t.Errorf("got %s, want %s", v.String(), expected)
	}
}
