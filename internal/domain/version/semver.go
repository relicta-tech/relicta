// Package version provides domain types for semantic versioning.
package version

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// SemanticVersion is a value object representing a semantic version.
// Immutable by design - all operations return new instances.
type SemanticVersion struct {
	major      uint64
	minor      uint64
	patch      uint64
	prerelease Prerelease
	metadata   BuildMetadata
}

// Prerelease represents the prerelease portion of a semantic version.
type Prerelease string

// BuildMetadata represents the build metadata portion of a semantic version.
type BuildMetadata string

// Common prerelease identifiers.
const (
	PrereleaseAlpha Prerelease = "alpha"
	PrereleaseBeta  Prerelease = "beta"
	PrereleaseRC    Prerelease = "rc"
)

var (
	// semverRegex validates semantic version strings.
	semverRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

	// Zero is the zero version (0.0.0).
	Zero = SemanticVersion{major: 0, minor: 0, patch: 0}

	// Initial is the initial version (0.1.0).
	Initial = SemanticVersion{major: 0, minor: 1, patch: 0}
)

// NewSemanticVersion creates a new SemanticVersion value object.
func NewSemanticVersion(major, minor, patch uint64) SemanticVersion {
	return SemanticVersion{
		major: major,
		minor: minor,
		patch: patch,
	}
}

// NewSemanticVersionWithPrerelease creates a new SemanticVersion with prerelease info.
func NewSemanticVersionWithPrerelease(major, minor, patch uint64, prerelease Prerelease) SemanticVersion {
	return SemanticVersion{
		major:      major,
		minor:      minor,
		patch:      patch,
		prerelease: prerelease,
	}
}

// Parse parses a semantic version string into a SemanticVersion value object.
// Returns an error if the string is not a valid semantic version.
func Parse(s string) (SemanticVersion, error) {
	matches := semverRegex.FindStringSubmatch(s)
	if matches == nil {
		return Zero, fmt.Errorf("invalid semantic version: %q", s)
	}

	major, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return Zero, fmt.Errorf("invalid major version: %w", err)
	}

	minor, err := strconv.ParseUint(matches[2], 10, 64)
	if err != nil {
		return Zero, fmt.Errorf("invalid minor version: %w", err)
	}

	patch, err := strconv.ParseUint(matches[3], 10, 64)
	if err != nil {
		return Zero, fmt.Errorf("invalid patch version: %w", err)
	}

	return SemanticVersion{
		major:      major,
		minor:      minor,
		patch:      patch,
		prerelease: Prerelease(matches[4]),
		metadata:   BuildMetadata(matches[5]),
	}, nil
}

// MustParse parses a semantic version string and panics if invalid.
// Use only for known-good version strings.
func MustParse(s string) SemanticVersion {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

// Major returns the major version component.
func (v SemanticVersion) Major() uint64 {
	return v.major
}

// Minor returns the minor version component.
func (v SemanticVersion) Minor() uint64 {
	return v.minor
}

// Patch returns the patch version component.
func (v SemanticVersion) Patch() uint64 {
	return v.patch
}

// Prerelease returns the prerelease identifier.
func (v SemanticVersion) Prerelease() Prerelease {
	return v.prerelease
}

// Metadata returns the build metadata.
func (v SemanticVersion) Metadata() BuildMetadata {
	return v.metadata
}

// IsPrerelease returns true if this is a prerelease version.
func (v SemanticVersion) IsPrerelease() bool {
	return v.prerelease != ""
}

// IsStable returns true if this is a stable release (>= 1.0.0 and no prerelease).
func (v SemanticVersion) IsStable() bool {
	return v.major >= 1 && !v.IsPrerelease()
}

// IsZero returns true if this is the zero version.
func (v SemanticVersion) IsZero() bool {
	return v.major == 0 && v.minor == 0 && v.patch == 0 && v.prerelease == "" && v.metadata == ""
}

// String returns the string representation of the version (without 'v' prefix).
func (v SemanticVersion) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d.%d.%d", v.major, v.minor, v.patch)

	if v.prerelease != "" {
		sb.WriteString("-")
		sb.WriteString(string(v.prerelease))
	}

	if v.metadata != "" {
		sb.WriteString("+")
		sb.WriteString(string(v.metadata))
	}

	return sb.String()
}

// TagString returns the version with 'v' prefix for git tags.
func (v SemanticVersion) TagString() string {
	return "v" + v.String()
}

// WithPrerelease returns a new version with the specified prerelease identifier.
func (v SemanticVersion) WithPrerelease(pre Prerelease) SemanticVersion {
	return SemanticVersion{
		major:      v.major,
		minor:      v.minor,
		patch:      v.patch,
		prerelease: pre,
		metadata:   v.metadata,
	}
}

// WithMetadata returns a new version with the specified build metadata.
func (v SemanticVersion) WithMetadata(meta BuildMetadata) SemanticVersion {
	return SemanticVersion{
		major:      v.major,
		minor:      v.minor,
		patch:      v.patch,
		prerelease: v.prerelease,
		metadata:   meta,
	}
}

// WithoutPrerelease returns a new version without the prerelease identifier.
func (v SemanticVersion) WithoutPrerelease() SemanticVersion {
	return SemanticVersion{
		major:    v.major,
		minor:    v.minor,
		patch:    v.patch,
		metadata: v.metadata,
	}
}

// WithoutMetadata returns a new version without the build metadata.
func (v SemanticVersion) WithoutMetadata() SemanticVersion {
	return SemanticVersion{
		major:      v.major,
		minor:      v.minor,
		patch:      v.patch,
		prerelease: v.prerelease,
	}
}

// Compare compares two versions.
// Returns -1 if v < other, 0 if v == other, 1 if v > other.
// Build metadata is ignored in comparisons per semver spec.
func (v SemanticVersion) Compare(other SemanticVersion) int {
	// Compare major
	if v.major != other.major {
		if v.major < other.major {
			return -1
		}
		return 1
	}

	// Compare minor
	if v.minor != other.minor {
		if v.minor < other.minor {
			return -1
		}
		return 1
	}

	// Compare patch
	if v.patch != other.patch {
		if v.patch < other.patch {
			return -1
		}
		return 1
	}

	// Compare prerelease
	// A version without prerelease has higher precedence than one with prerelease
	if v.prerelease == "" && other.prerelease != "" {
		return 1
	}
	if v.prerelease != "" && other.prerelease == "" {
		return -1
	}
	if v.prerelease < other.prerelease {
		return -1
	}
	if v.prerelease > other.prerelease {
		return 1
	}

	return 0
}

// LessThan returns true if v < other.
func (v SemanticVersion) LessThan(other SemanticVersion) bool {
	return v.Compare(other) < 0
}

// LessThanOrEqual returns true if v <= other.
func (v SemanticVersion) LessThanOrEqual(other SemanticVersion) bool {
	return v.Compare(other) <= 0
}

// GreaterThan returns true if v > other.
func (v SemanticVersion) GreaterThan(other SemanticVersion) bool {
	return v.Compare(other) > 0
}

// GreaterThanOrEqual returns true if v >= other.
func (v SemanticVersion) GreaterThanOrEqual(other SemanticVersion) bool {
	return v.Compare(other) >= 0
}

// Equal returns true if two versions are equal (ignoring metadata).
func (v SemanticVersion) Equal(other SemanticVersion) bool {
	return v.Compare(other) == 0
}

// Equals returns true if two versions are exactly equal (including metadata).
func (v SemanticVersion) Equals(other SemanticVersion) bool {
	return v.major == other.major &&
		v.minor == other.minor &&
		v.patch == other.patch &&
		v.prerelease == other.prerelease &&
		v.metadata == other.metadata
}
