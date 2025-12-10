// Package version provides version management for ReleasePilot.
package version

import (
	"context"

	"github.com/felixgeelhaar/release-pilot/internal/service/git"
)

// Service defines the interface for version management operations.
type Service interface {
	// Version operations

	// GetCurrentVersion returns the current version from the configured source.
	GetCurrentVersion(ctx context.Context) (*Version, error)

	// GetCurrentVersionFromTag returns the current version from git tags.
	GetCurrentVersionFromTag(ctx context.Context, prefix string) (*Version, error)

	// GetCurrentVersionFromFile returns the current version from a file.
	GetCurrentVersionFromFile(ctx context.Context, path string) (*Version, error)

	// CalculateNextVersion calculates the next version based on the current version and release type.
	CalculateNextVersion(current *Version, releaseType git.ReleaseType) (*Version, error)

	// BumpVersion applies a version bump and returns the new version.
	BumpVersion(ctx context.Context, opts BumpOptions) (*Version, error)

	// ParseVersion parses a version string.
	ParseVersion(version string) (*Version, error)

	// FormatVersion formats a version with the given options.
	FormatVersion(v *Version, opts FormatOptions) string

	// Changelog operations

	// GenerateChangelog generates a changelog from categorized changes.
	GenerateChangelog(ctx context.Context, changes *git.CategorizedChanges, opts ChangelogOptions) (string, error)

	// UpdateChangelogFile updates the changelog file with new content.
	UpdateChangelogFile(ctx context.Context, path string, content string, version *Version) error

	// ReadChangelogSection reads a specific version's section from the changelog.
	ReadChangelogSection(ctx context.Context, path string, version string) (string, error)

	// Validation

	// ValidateVersion validates a version string.
	ValidateVersion(version string) error

	// CompareVersions compares two versions and returns -1, 0, or 1.
	CompareVersions(v1, v2 *Version) int

	// IsPrerelease returns true if the version is a prerelease.
	IsPrerelease(v *Version) bool
}

// Version represents a semantic version.
type Version struct {
	// Major is the major version number.
	Major uint64 `json:"major"`
	// Minor is the minor version number.
	Minor uint64 `json:"minor"`
	// Patch is the patch version number.
	Patch uint64 `json:"patch"`
	// Prerelease is the prerelease identifier (e.g., "alpha", "beta.1", "rc.2").
	Prerelease string `json:"prerelease,omitempty"`
	// Metadata is the build metadata (e.g., "20240101", "sha.abc123").
	Metadata string `json:"metadata,omitempty"`
	// Original is the original version string as parsed.
	Original string `json:"original,omitempty"`
}

// String returns the version as a string (without prefix).
func (v *Version) String() string {
	return FormatVersionString(v, FormatOptions{})
}

// StringWithPrefix returns the version with a prefix.
func (v *Version) StringWithPrefix(prefix string) string {
	return prefix + v.String()
}

// IsZero returns true if this is the zero version (0.0.0).
func (v *Version) IsZero() bool {
	return v.Major == 0 && v.Minor == 0 && v.Patch == 0 && v.Prerelease == ""
}

// BumpOptions configures version bumping.
type BumpOptions struct {
	// ReleaseType is the type of version bump.
	ReleaseType git.ReleaseType
	// Prerelease sets the prerelease identifier.
	Prerelease string
	// Metadata sets the build metadata.
	Metadata string
	// Prefix is the tag prefix (e.g., "v").
	Prefix string
	// CreateTag creates a git tag.
	CreateTag bool
	// TagMessage is the message for annotated tags.
	TagMessage string
	// PushTag pushes the tag to remote.
	PushTag bool
	// UpdateFile updates a version file.
	UpdateFile string
	// DryRun simulates the bump without making changes.
	DryRun bool
}

// DefaultBumpOptions returns the default bump options.
func DefaultBumpOptions() BumpOptions {
	return BumpOptions{
		ReleaseType: git.ReleaseTypePatch,
		Prefix:      "v",
		CreateTag:   true,
		PushTag:     false,
		DryRun:      false,
	}
}

// FormatOptions configures version formatting.
type FormatOptions struct {
	// IncludePrefix includes the version prefix.
	IncludePrefix bool
	// Prefix is the version prefix (default: "v").
	Prefix string
	// IncludeMetadata includes build metadata.
	IncludeMetadata bool
}

// ChangelogOptions configures changelog generation.
type ChangelogOptions struct {
	// Version is the version being released.
	Version *Version
	// PreviousVersion is the previous version.
	PreviousVersion *Version
	// Date is the release date.
	Date string
	// RepositoryURL is the repository URL for links.
	RepositoryURL string
	// IssueURL is the issue tracker URL pattern.
	IssueURL string
	// Format is the changelog format (keep-a-changelog, conventional, custom).
	Format string
	// GroupBy specifies how to group changes (type, scope, none).
	GroupBy string
	// IncludeCommitHash includes commit hashes.
	IncludeCommitHash bool
	// IncludeAuthor includes author information.
	IncludeAuthor bool
	// LinkCommits creates links to commits.
	LinkCommits bool
	// LinkIssues creates links to issues.
	LinkIssues bool
	// Categories maps commit types to display names.
	Categories map[string]string
	// Exclude lists commit types to exclude.
	Exclude []string
	// Template is a custom template path.
	Template string
	// CompareURL is the URL for version comparison.
	CompareURL string
}

// DefaultChangelogOptions returns the default changelog options.
func DefaultChangelogOptions() ChangelogOptions {
	return ChangelogOptions{
		Format:            "keep-a-changelog",
		GroupBy:           "type",
		IncludeCommitHash: true,
		IncludeAuthor:     false,
		LinkCommits:       true,
		LinkIssues:        true,
		Categories: map[string]string{
			"feat":     "Features",
			"fix":      "Bug Fixes",
			"perf":     "Performance Improvements",
			"refactor": "Code Refactoring",
			"docs":     "Documentation",
			"build":    "Build System",
			"revert":   "Reverts",
		},
		Exclude: []string{"chore", "ci", "test", "style"},
	}
}

// FormatVersionString formats a version as a string.
func FormatVersionString(v *Version, opts FormatOptions) string {
	result := ""

	if opts.IncludePrefix {
		prefix := opts.Prefix
		if prefix == "" {
			prefix = "v"
		}
		result = prefix
	}

	// Base version
	result += formatMajorMinorPatch(v.Major, v.Minor, v.Patch)

	// Prerelease
	if v.Prerelease != "" {
		result += "-" + v.Prerelease
	}

	// Metadata
	if opts.IncludeMetadata && v.Metadata != "" {
		result += "+" + v.Metadata
	}

	return result
}

// formatMajorMinorPatch formats the major.minor.patch portion.
func formatMajorMinorPatch(major, minor, patch uint64) string {
	return formatUint64(major) + "." + formatUint64(minor) + "." + formatUint64(patch)
}

// formatUint64 formats a uint64 as a string.
func formatUint64(n uint64) string {
	if n == 0 {
		return "0"
	}
	// Simple conversion
	digits := make([]byte, 0, 20)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// Parse is a standalone function to parse a version string.
// It delegates to the semver library for actual parsing.
func Parse(version string) (*Version, error) {
	// Use a temporary service instance for parsing
	// This is a bit of a hack but allows standalone parsing without a git service
	svc := &parseOnlyService{}
	return svc.ParseVersion(version)
}

// parseOnlyService is a minimal implementation for parsing only.
type parseOnlyService struct{}

// ParseVersion parses a version string.
func (s *parseOnlyService) ParseVersion(version string) (*Version, error) {
	// Import the semver library - this is done at package level
	// Clean up the version string
	version = trimSpace(version)
	version = trimPrefix(version, "v")
	version = trimPrefix(version, "V")

	// Parse using regexp for basic validation
	// Format: major.minor.patch[-prerelease][+metadata]
	parts := splitVersion(version)
	if parts == nil {
		return nil, &parseError{version: version, message: "invalid version format"}
	}

	return parts, nil
}

// parseError is returned when version parsing fails.
type parseError struct {
	version string
	message string
}

func (e *parseError) Error() string {
	return "parse version " + e.version + ": " + e.message
}

// trimSpace trims whitespace from a string.
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// trimPrefix removes a prefix from a string.
func trimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

// splitVersion splits a version string into components.
func splitVersion(version string) *Version {
	// Split by +, -, and .
	// Format: major.minor.patch[-prerelease][+metadata]

	// Extract metadata
	metadata := ""
	if idx := indexByte(version, '+'); idx >= 0 {
		metadata = version[idx+1:]
		version = version[:idx]
	}

	// Extract prerelease
	prerelease := ""
	if idx := indexByte(version, '-'); idx >= 0 {
		prerelease = version[idx+1:]
		version = version[:idx]
	}

	// Parse major.minor.patch
	parts := splitDots(version)
	if len(parts) < 3 {
		return nil
	}

	major, ok1 := parseUint(parts[0])
	minor, ok2 := parseUint(parts[1])
	patch, ok3 := parseUint(parts[2])

	if !ok1 || !ok2 || !ok3 {
		return nil
	}

	return &Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: prerelease,
		Metadata:   metadata,
		Original:   version,
	}
}

// indexByte returns the index of the first occurrence of c in s, or -1 if not present.
func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// splitDots splits a string by dots.
func splitDots(s string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '.' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return parts
}

// parseUint parses an unsigned integer.
func parseUint(s string) (uint64, bool) {
	if len(s) == 0 {
		return 0, false
	}
	var n uint64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + uint64(c-'0')
	}
	return n, true
}

// ServiceConfig configures the version service.
type ServiceConfig struct {
	// GitService is the git service for tag operations.
	GitService git.Service
	// DefaultPrefix is the default tag prefix.
	DefaultPrefix string
	// VersionSource is where to read the version from (tag, file).
	VersionSource string
	// VersionFile is the file containing the version (if source is file).
	VersionFile string
}

// DefaultServiceConfig returns the default service configuration.
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		DefaultPrefix: "v",
		VersionSource: "tag",
	}
}

// ServiceOption configures the version service.
type ServiceOption func(*ServiceConfig)

// WithGitService sets the git service.
func WithGitService(svc git.Service) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.GitService = svc
	}
}

// WithDefaultPrefix sets the default tag prefix.
func WithDefaultPrefix(prefix string) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.DefaultPrefix = prefix
	}
}

// WithVersionSource sets the version source.
func WithVersionSource(source string) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.VersionSource = source
	}
}

// WithVersionFile sets the version file.
func WithVersionFile(file string) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.VersionFile = file
	}
}
