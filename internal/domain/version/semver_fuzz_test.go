package version

import (
	"testing"
)

// FuzzParse tests the semantic version parser with fuzzing.
// Run with: go test -fuzz=FuzzParse -fuzztime=30s
func FuzzParse(f *testing.F) {
	// Add seed corpus with valid and invalid version strings
	seeds := []string{
		// Valid versions
		"1.0.0",
		"0.0.1",
		"10.20.30",
		"1.2.3-alpha",
		"1.2.3-beta.1",
		"1.2.3-alpha.beta",
		"1.2.3-0.3.7",
		"1.2.3+build",
		"1.2.3+build.123",
		"1.2.3-alpha+build",
		"1.2.3-alpha.1+build.456",
		"v1.0.0",
		"v1.2.3-rc.1",
		// Edge cases
		"0.0.0",
		"999.999.999",
		"1.0.0-alpha.1.2.3.4.5",
		"1.0.0+build.1.2.3.4.5",
		// Invalid versions
		"",
		"v",
		"1",
		"1.0",
		"1.0.0.0",
		"a.b.c",
		"1.a.0",
		"1.0.b",
		"-1.0.0",
		"1.-1.0",
		"1.0.-1",
		"01.0.0",
		"1.00.0",
		"1.0.00",
		"1.0.0-",
		"1.0.0+",
		"1.0.0-+",
		"1.0.0- alpha",
		"1.0.0+build info",
		"1..0",
		".1.0",
		"1.0.",
		// Injection attempts
		"1.0.0; rm -rf /",
		"1.0.0 && cat /etc/passwd",
		"1.0.0$(whoami)",
		"1.0.0`ls`",
		"<script>1.0.0</script>",
		// Unicode
		"1.0.0-α",
		"１.２.３",
		"1.0.0-新版本",
		// Whitespace
		" 1.0.0",
		"1.0.0 ",
		"1 .0.0",
		"1. 0.0",
		"\t1.0.0",
		"1.0.0\n",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, versionStr string) {
		// The function should not panic on any input
		v, err := Parse(versionStr)

		// Invariant checks
		if err == nil {
			// If parsing succeeds, validate the result
			str := v.String()
			if str == "" {
				t.Errorf("parsed version has empty string representation for input: %q", versionStr)
			}

			// Parse the string representation should also work
			reparsed, err2 := Parse(str)
			if err2 != nil {
				t.Errorf("failed to reparse version string %q: %v", str, err2)
			}
			if reparsed.Compare(v) != 0 {
				t.Errorf("reparsed version differs: original=%v, reparsed=%v", v, reparsed)
			}
		}

		// Empty string should always fail
		if versionStr == "" && err == nil {
			t.Errorf("parsing empty string should fail")
		}
	})
}

// FuzzVersionBump_Apply tests version bumping with fuzzing.
func FuzzVersionBump_Apply(f *testing.F) {
	// Seed with various version combinations
	f.Add(uint64(0), uint64(0), uint64(1), "patch")
	f.Add(uint64(1), uint64(0), uint64(0), "major")
	f.Add(uint64(0), uint64(1), uint64(0), "minor")
	f.Add(uint64(999), uint64(999), uint64(999), "patch")
	f.Add(uint64(0), uint64(0), uint64(0), "major")

	f.Fuzz(func(t *testing.T, major, minor, patch uint64, bumpTypeStr string) {
		// Cap values to avoid overflow in edge cases
		if major > 1000000 {
			major = 1000000
		}
		if minor > 1000000 {
			minor = 1000000
		}
		if patch > 1000000 {
			patch = 1000000
		}

		v := NewSemanticVersion(major, minor, patch)

		var bumpType BumpType
		switch bumpTypeStr {
		case "major":
			bumpType = BumpMajor
		case "minor":
			bumpType = BumpMinor
		case "patch":
			bumpType = BumpPatch
		default:
			// Skip invalid bump types
			return
		}

		bump := NewVersionBump(bumpType)
		result := bump.Apply(v)

		// Verify bump behavior
		switch bumpType {
		case BumpMajor:
			if result.Major() != v.Major()+1 {
				t.Errorf("major bump: expected major %d, got %d", v.Major()+1, result.Major())
			}
			if result.Minor() != 0 || result.Patch() != 0 {
				t.Errorf("major bump should reset minor and patch to 0: got %v", result)
			}
		case BumpMinor:
			if result.Major() != v.Major() {
				t.Errorf("minor bump: expected major %d, got %d", v.Major(), result.Major())
			}
			if result.Minor() != v.Minor()+1 {
				t.Errorf("minor bump: expected minor %d, got %d", v.Minor()+1, result.Minor())
			}
			if result.Patch() != 0 {
				t.Errorf("minor bump should reset patch to 0: got %v", result)
			}
		case BumpPatch:
			if result.Major() != v.Major() || result.Minor() != v.Minor() {
				t.Errorf("patch bump should not change major/minor: %v -> %v", v, result)
			}
			if result.Patch() != v.Patch()+1 {
				t.Errorf("patch bump: expected patch %d, got %d", v.Patch()+1, result.Patch())
			}
		}
	})
}
