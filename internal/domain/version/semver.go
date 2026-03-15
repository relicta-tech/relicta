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

// PrereleaseType returns the type portion of the prerelease identifier (e.g., "alpha" from "alpha.3").
// Returns an empty Prerelease if this is not a prerelease version.
func (v SemanticVersion) PrereleaseType() Prerelease {
	if v.prerelease == "" {
		return ""
	}
	parts := strings.Split(string(v.prerelease), ".")
	return Prerelease(parts[0])
}

// PrereleaseNumber returns the numeric counter from the prerelease identifier (e.g., 3 from "alpha.3").
// Returns 0 if there is no numeric counter or this is not a prerelease version.
func (v SemanticVersion) PrereleaseNumber() uint64 {
	if v.prerelease == "" {
		return 0
	}
	parts := strings.Split(string(v.prerelease), ".")
	if len(parts) < 2 {
		return 0
	}
	n, err := strconv.ParseUint(parts[len(parts)-1], 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// BumpPreRelease increments the pre-release counter for the given pre-release type.
// If the version has no prerelease or has a different prerelease type, it starts at 1.
// If the version already has the same prerelease type, it increments the counter.
// Examples:
//   - 1.2.3 + alpha -> 1.3.0-alpha.1 (bumps minor, starts at .1)
//   - 1.3.0-alpha.1 + alpha -> 1.3.0-alpha.2
//   - 1.3.0-alpha.2 + beta -> 1.3.0-beta.1
//   - 1.3.0-beta.1 + rc -> 1.3.0-rc.1
func (v SemanticVersion) BumpPreRelease(preType Prerelease) SemanticVersion {
	if preType == "" {
		return v
	}

	if v.IsPrerelease() {
		currentType := v.PrereleaseType()
		if currentType == preType {
			// Same prerelease type: increment counter
			nextNum := v.PrereleaseNumber() + 1
			return SemanticVersion{
				major:      v.major,
				minor:      v.minor,
				patch:      v.patch,
				prerelease: Prerelease(fmt.Sprintf("%s.%d", preType, nextNum)),
			}
		}
		// Different prerelease type: start at 1 with same major.minor.patch
		return SemanticVersion{
			major:      v.major,
			minor:      v.minor,
			patch:      v.patch,
			prerelease: Prerelease(fmt.Sprintf("%s.%d", preType, 1)),
		}
	}

	// No prerelease: bump minor and start at .1
	return SemanticVersion{
		major:      v.major,
		minor:      v.minor + 1,
		patch:      0,
		prerelease: Prerelease(fmt.Sprintf("%s.%d", preType, 1)),
	}
}

// PromoteToRelease strips the pre-release suffix, producing the stable release version.
// If the version is not a prerelease, it returns the version unchanged.
// Example: 1.3.0-rc.2 -> 1.3.0
func (v SemanticVersion) PromoteToRelease() SemanticVersion {
	return v.WithoutPrerelease()
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
// Pre-release identifiers are compared per the semver 2.0.0 specification:
// identifiers are split by dots and compared left-to-right. Numeric identifiers
// are compared as integers; alphanumeric identifiers are compared lexically.
// A version without prerelease has higher precedence than one with prerelease.
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

	return comparePrerelease(v.prerelease, other.prerelease)
}

// comparePrerelease compares two prerelease identifiers per semver 2.0.0 spec.
// Identifiers are split by dots and compared left-to-right.
// Numeric identifiers are compared as integers; alphanumeric identifiers lexically.
// Numeric identifiers always have lower precedence than alphanumeric identifiers.
// A shorter set of identifiers has lower precedence than a longer set when all
// preceding identifiers are equal.
func comparePrerelease(a, b Prerelease) int {
	if a == b {
		return 0
	}

	partsA := strings.Split(string(a), ".")
	partsB := strings.Split(string(b), ".")

	minLen := len(partsA)
	if len(partsB) < minLen {
		minLen = len(partsB)
	}

	for i := 0; i < minLen; i++ {
		cmp := comparePrereleaseIdentifier(partsA[i], partsB[i])
		if cmp != 0 {
			return cmp
		}
	}

	// All compared identifiers are equal; shorter set has lower precedence
	if len(partsA) < len(partsB) {
		return -1
	}
	if len(partsA) > len(partsB) {
		return 1
	}
	return 0
}

// comparePrereleaseIdentifier compares two individual prerelease identifier segments.
func comparePrereleaseIdentifier(a, b string) int {
	numA, errA := strconv.ParseUint(a, 10, 64)
	numB, errB := strconv.ParseUint(b, 10, 64)

	switch {
	case errA == nil && errB == nil:
		// Both numeric: compare as integers
		if numA < numB {
			return -1
		}
		if numA > numB {
			return 1
		}
		return 0
	case errA == nil:
		// Only a is numeric: numeric has lower precedence
		return -1
	case errB == nil:
		// Only b is numeric: numeric has lower precedence
		return 1
	default:
		// Both alphanumeric: compare lexically
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	}
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
