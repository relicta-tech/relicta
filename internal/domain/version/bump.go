// Package version provides domain types for semantic versioning.
package version

import (
	"fmt"
)

// BumpType represents the type of version bump to apply.
type BumpType string

const (
	// BumpMajor indicates a major version bump (breaking changes).
	BumpMajor BumpType = "major"
	// BumpMinor indicates a minor version bump (new features).
	BumpMinor BumpType = "minor"
	// BumpPatch indicates a patch version bump (bug fixes).
	BumpPatch BumpType = "patch"
	// BumpPrerelease indicates a prerelease version bump.
	BumpPrerelease BumpType = "prerelease"
)

// IsValid returns true if the bump type is valid.
func (b BumpType) IsValid() bool {
	switch b {
	case BumpMajor, BumpMinor, BumpPatch, BumpPrerelease:
		return true
	default:
		return false
	}
}

// String returns the string representation of the bump type.
func (b BumpType) String() string {
	return string(b)
}

// ParseBumpType parses a string into a BumpType.
func ParseBumpType(s string) (BumpType, error) {
	bt := BumpType(s)
	if !bt.IsValid() {
		return "", fmt.Errorf("invalid bump type: %q (must be major, minor, patch, or prerelease)", s)
	}
	return bt, nil
}

// VersionBump is a value object representing a version bump operation.
type VersionBump struct {
	bumpType   BumpType
	prerelease Prerelease
}

// NewVersionBump creates a new VersionBump for the specified type.
func NewVersionBump(bumpType BumpType) VersionBump {
	return VersionBump{bumpType: bumpType}
}

// NewPrereleaseBump creates a new VersionBump for a prerelease version.
func NewPrereleaseBump(prerelease Prerelease) VersionBump {
	return VersionBump{
		bumpType:   BumpPrerelease,
		prerelease: prerelease,
	}
}

// Type returns the bump type.
func (b VersionBump) Type() BumpType {
	return b.bumpType
}

// PrereleaseIdentifier returns the prerelease identifier for prerelease bumps.
func (b VersionBump) PrereleaseIdentifier() Prerelease {
	return b.prerelease
}

// Apply applies the version bump to a semantic version and returns the new version.
func (b VersionBump) Apply(v SemanticVersion) SemanticVersion {
	switch b.bumpType {
	case BumpMajor:
		return SemanticVersion{
			major: v.major + 1,
			minor: 0,
			patch: 0,
		}

	case BumpMinor:
		return SemanticVersion{
			major: v.major,
			minor: v.minor + 1,
			patch: 0,
		}

	case BumpPatch:
		// If current version has a prerelease, just remove it (releasing the prerelease)
		if v.IsPrerelease() {
			return v.WithoutPrerelease()
		}
		return SemanticVersion{
			major: v.major,
			minor: v.minor,
			patch: v.patch + 1,
		}

	case BumpPrerelease:
		// Prerelease bump logic:
		// - If no prerelease, increment patch and add prerelease
		// - If same prerelease type, increment prerelease number
		// - If different prerelease type, use new type with .1
		if b.prerelease != "" {
			if v.IsPrerelease() {
				// Already a prerelease, update the identifier
				return v.WithPrerelease(b.prerelease)
			}
			// Not a prerelease, bump minor and add prerelease
			return SemanticVersion{
				major:      v.major,
				minor:      v.minor + 1,
				patch:      0,
				prerelease: b.prerelease,
			}
		}
		return v

	default:
		return v
	}
}

// BumpMajorVersion returns a new version with the major component incremented.
func BumpMajorVersion(v SemanticVersion) SemanticVersion {
	return NewVersionBump(BumpMajor).Apply(v)
}

// BumpMinorVersion returns a new version with the minor component incremented.
func BumpMinorVersion(v SemanticVersion) SemanticVersion {
	return NewVersionBump(BumpMinor).Apply(v)
}

// BumpPatchVersion returns a new version with the patch component incremented.
func BumpPatchVersion(v SemanticVersion) SemanticVersion {
	return NewVersionBump(BumpPatch).Apply(v)
}

// VersionCalculator defines the interface for calculating version bumps.
type VersionCalculator interface {
	// CalculateNextVersion determines the next version based on current version and bump type.
	CalculateNextVersion(current SemanticVersion, bump BumpType) SemanticVersion

	// DetermineRequiredBump analyzes changes and determines the required bump type.
	// This is typically implemented in the changes domain and used here.
	DetermineRequiredBump(hasBreaking, hasFeature, hasFix bool) BumpType
}

// DefaultVersionCalculator provides standard version calculation logic.
type DefaultVersionCalculator struct{}

// NewDefaultVersionCalculator creates a new DefaultVersionCalculator.
func NewDefaultVersionCalculator() *DefaultVersionCalculator {
	return &DefaultVersionCalculator{}
}

// CalculateNextVersion determines the next version based on current version and bump type.
func (c *DefaultVersionCalculator) CalculateNextVersion(current SemanticVersion, bump BumpType) SemanticVersion {
	return NewVersionBump(bump).Apply(current)
}

// DetermineRequiredBump analyzes changes and determines the required bump type.
func (c *DefaultVersionCalculator) DetermineRequiredBump(hasBreaking, hasFeature, hasFix bool) BumpType {
	if hasBreaking {
		return BumpMajor
	}
	if hasFeature {
		return BumpMinor
	}
	if hasFix {
		return BumpPatch
	}
	return BumpPatch // Default to patch for any other changes
}
