// Package changes provides domain types for analyzing commit changes.
package changes

import (
	"fmt"
	"strings"

	"github.com/felixgeelhaar/release-pilot/internal/domain/version"
)

// ReleaseType represents the type of release based on changes.
type ReleaseType string

const (
	// ReleaseTypeMajor indicates a major release with breaking changes.
	ReleaseTypeMajor ReleaseType = "major"
	// ReleaseTypeMinor indicates a minor release with new features.
	ReleaseTypeMinor ReleaseType = "minor"
	// ReleaseTypePatch indicates a patch release with bug fixes.
	ReleaseTypePatch ReleaseType = "patch"
	// ReleaseTypeNone indicates no release is needed.
	ReleaseTypeNone ReleaseType = "none"
)

// String returns the string representation of the release type.
func (r ReleaseType) String() string {
	return string(r)
}

// IsValid returns true if the release type is valid.
func (r ReleaseType) IsValid() bool {
	switch r {
	case ReleaseTypeMajor, ReleaseTypeMinor, ReleaseTypePatch, ReleaseTypeNone:
		return true
	default:
		return false
	}
}

// ToBumpType converts a ReleaseType to a version.BumpType.
func (r ReleaseType) ToBumpType() version.BumpType {
	switch r {
	case ReleaseTypeMajor:
		return version.BumpMajor
	case ReleaseTypeMinor:
		return version.BumpMinor
	case ReleaseTypePatch:
		return version.BumpPatch
	default:
		return version.BumpPatch
	}
}

// Description returns a human-readable description.
func (r ReleaseType) Description() string {
	switch r {
	case ReleaseTypeMajor:
		return "Major release with breaking changes"
	case ReleaseTypeMinor:
		return "Minor release with new features"
	case ReleaseTypePatch:
		return "Patch release with bug fixes"
	case ReleaseTypeNone:
		return "No release needed"
	default:
		return "Unknown release type"
	}
}

// ParseReleaseType parses a string into a ReleaseType.
func ParseReleaseType(s string) (ReleaseType, error) {
	r := ReleaseType(strings.ToLower(strings.TrimSpace(s)))
	if !r.IsValid() {
		return "", fmt.Errorf("invalid release type: %q", s)
	}
	return r, nil
}

// ReleaseTypeFromCommitType determines the release type based on commit type.
func ReleaseTypeFromCommitType(ct CommitType, isBreaking bool) ReleaseType {
	if isBreaking {
		return ReleaseTypeMajor
	}

	switch ct {
	case CommitTypeFeat:
		return ReleaseTypeMinor
	case CommitTypeFix, CommitTypePerf:
		return ReleaseTypePatch
	default:
		return ReleaseTypeNone
	}
}

// MaxReleaseType returns the higher precedence release type.
// Major > Minor > Patch > None
func MaxReleaseType(a, b ReleaseType) ReleaseType {
	if a == ReleaseTypeMajor || b == ReleaseTypeMajor {
		return ReleaseTypeMajor
	}
	if a == ReleaseTypeMinor || b == ReleaseTypeMinor {
		return ReleaseTypeMinor
	}
	if a == ReleaseTypePatch || b == ReleaseTypePatch {
		return ReleaseTypePatch
	}
	return ReleaseTypeNone
}
