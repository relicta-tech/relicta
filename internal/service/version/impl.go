// Package version provides version management for ReleasePilot.
package version

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"

	rperrors "github.com/felixgeelhaar/release-pilot/internal/errors"
	"github.com/felixgeelhaar/release-pilot/internal/service/git"
)

// Pre-compiled regex patterns for changelog parsing.
var (
	// versionHeaderRegex matches version headers like "## [1.0.0]" or "## 1.0.0"
	versionHeaderRegex = regexp.MustCompile(`(?m)^## \[?\d+\.\d+\.\d+`)
	// unreleasedRegex matches the Unreleased section header
	unreleasedRegex = regexp.MustCompile(`(?m)^## \[?Unreleased\]?`)
	// nextVersionRegex matches the next version header (without multiline flag for line-by-line scanning)
	nextVersionRegex = regexp.MustCompile(`^## \[?\d+\.\d+\.\d+`)
)

// Ensure ServiceImpl implements Service.
var _ Service = (*ServiceImpl)(nil)

// ServiceImpl is the implementation of the version service.
type ServiceImpl struct {
	cfg        ServiceConfig
	gitService git.Service
}

// NewService creates a new version service.
func NewService(opts ...ServiceOption) (*ServiceImpl, error) {
	cfg := DefaultServiceConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.GitService == nil {
		return nil, rperrors.Version("version.NewService", "git service is required")
	}

	return &ServiceImpl{
		cfg:        cfg,
		gitService: cfg.GitService,
	}, nil
}

// GetCurrentVersion returns the current version from the configured source.
func (s *ServiceImpl) GetCurrentVersion(ctx context.Context) (*Version, error) {
	switch s.cfg.VersionSource {
	case "tag":
		return s.GetCurrentVersionFromTag(ctx, s.cfg.DefaultPrefix)
	case "file":
		return s.GetCurrentVersionFromFile(ctx, s.cfg.VersionFile)
	default:
		return s.GetCurrentVersionFromTag(ctx, s.cfg.DefaultPrefix)
	}
}

// GetCurrentVersionFromTag returns the current version from git tags.
func (s *ServiceImpl) GetCurrentVersionFromTag(ctx context.Context, prefix string) (*Version, error) {
	const op = "version.GetCurrentVersionFromTag"

	tag, err := s.gitService.GetLatestVersionTag(ctx, prefix)
	if err != nil {
		// No tags found, return initial version
		if rperrors.IsKind(err, rperrors.KindNotFound) {
			return &Version{
				Major:    0,
				Minor:    0,
				Patch:    0,
				Original: "0.0.0",
			}, nil
		}
		return nil, rperrors.VersionWrap(err, op, "failed to get latest version tag")
	}

	// Parse the version from the tag name
	versionStr := strings.TrimPrefix(tag.Name, prefix)
	return s.ParseVersion(versionStr)
}

// GetCurrentVersionFromFile returns the current version from a file.
func (s *ServiceImpl) GetCurrentVersionFromFile(_ context.Context, path string) (*Version, error) {
	const op = "version.GetCurrentVersionFromFile"

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, rperrors.NotFound(op, fmt.Sprintf("version file not found: %s", path))
		}
		return nil, rperrors.IOWrap(err, op, "failed to read version file")
	}

	// Try to parse as JSON (package.json style)
	if strings.HasSuffix(path, ".json") {
		var pkg struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(data, &pkg); err == nil && pkg.Version != "" {
			return s.ParseVersion(pkg.Version)
		}
	}

	// Try to parse as plain text version
	versionStr := strings.TrimSpace(string(data))
	return s.ParseVersion(versionStr)
}

// CalculateNextVersion calculates the next version based on the current version and release type.
func (s *ServiceImpl) CalculateNextVersion(current *Version, releaseType git.ReleaseType) (*Version, error) {
	const op = "version.CalculateNextVersion"

	if current == nil {
		return nil, rperrors.Validation(op, "current version is required")
	}

	next := &Version{
		Major:    current.Major,
		Minor:    current.Minor,
		Patch:    current.Patch,
		Original: "",
	}

	switch releaseType {
	case git.ReleaseTypeMajor:
		next.Major++
		next.Minor = 0
		next.Patch = 0
	case git.ReleaseTypeMinor:
		next.Minor++
		next.Patch = 0
	case git.ReleaseTypePatch:
		next.Patch++
	case git.ReleaseTypeNone:
		// No change
		return current, nil
	default:
		return nil, rperrors.Validation(op, fmt.Sprintf("unknown release type: %s", releaseType))
	}

	return next, nil
}

// BumpVersion applies a version bump and returns the new version.
func (s *ServiceImpl) BumpVersion(ctx context.Context, opts BumpOptions) (*Version, error) {
	const op = "version.BumpVersion"

	// Get current version
	current, err := s.GetCurrentVersion(ctx)
	if err != nil {
		return nil, rperrors.VersionWrap(err, op, "failed to get current version")
	}

	// Calculate next version
	next, err := s.CalculateNextVersion(current, opts.ReleaseType)
	if err != nil {
		return nil, rperrors.VersionWrap(err, op, "failed to calculate next version")
	}

	// Apply prerelease and metadata
	if opts.Prerelease != "" {
		next.Prerelease = opts.Prerelease
	}
	if opts.Metadata != "" {
		next.Metadata = opts.Metadata
	}

	if opts.DryRun {
		return next, nil
	}

	// Update version file if configured
	if opts.UpdateFile != "" {
		if err := s.updateVersionFile(opts.UpdateFile, next); err != nil {
			return nil, rperrors.VersionWrap(err, op, "failed to update version file")
		}
	}

	// Create tag if configured
	if opts.CreateTag {
		tagName := next.StringWithPrefix(opts.Prefix)
		tagMessage := opts.TagMessage
		if tagMessage == "" {
			tagMessage = fmt.Sprintf("Release %s", tagName)
		}

		tagOpts := git.DefaultTagOptions()
		if err := s.gitService.CreateTag(ctx, tagName, tagMessage, tagOpts); err != nil {
			return nil, rperrors.VersionWrap(err, op, "failed to create tag")
		}

		// Push tag if configured
		if opts.PushTag {
			pushOpts := git.DefaultPushOptions()
			if err := s.gitService.PushTag(ctx, tagName, pushOpts); err != nil {
				return nil, rperrors.VersionWrap(err, op, "failed to push tag")
			}
		}
	}

	return next, nil
}

// updateVersionFile updates a version file with the new version.
func (s *ServiceImpl) updateVersionFile(path string, v *Version) error {
	const op = "version.updateVersionFile"

	// Read existing file
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return rperrors.IOWrap(err, op, "failed to read version file")
	}

	var newData []byte

	if strings.HasSuffix(path, ".json") {
		// Update JSON file
		var content map[string]any
		if len(data) > 0 {
			if unmarshalErr := json.Unmarshal(data, &content); unmarshalErr != nil {
				return rperrors.IOWrap(unmarshalErr, op, "failed to parse JSON file")
			}
		} else {
			content = make(map[string]any)
		}
		content["version"] = v.String()
		newData, err = json.MarshalIndent(content, "", "  ")
		if err != nil {
			return rperrors.IOWrap(err, op, "failed to marshal JSON")
		}
		newData = append(newData, '\n')
	} else {
		// Plain text file
		newData = []byte(v.String() + "\n")
	}

	if err := os.WriteFile(path, newData, 0o644); err != nil {
		return rperrors.IOWrap(err, op, "failed to write version file")
	}

	return nil
}

// ParseVersion parses a version string.
func (s *ServiceImpl) ParseVersion(version string) (*Version, error) {
	const op = "version.ParseVersion"

	// Clean up the version string
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")

	sv, err := semver.NewVersion(version)
	if err != nil {
		return nil, rperrors.ValidationWrap(err, op, fmt.Sprintf("invalid version: %s", version))
	}

	return &Version{
		Major:      sv.Major(),
		Minor:      sv.Minor(),
		Patch:      sv.Patch(),
		Prerelease: sv.Prerelease(),
		Metadata:   sv.Metadata(),
		Original:   version,
	}, nil
}

// FormatVersion formats a version with the given options.
func (s *ServiceImpl) FormatVersion(v *Version, opts FormatOptions) string {
	return FormatVersionString(v, opts)
}

// GenerateChangelog generates a changelog from categorized changes.
func (s *ServiceImpl) GenerateChangelog(_ context.Context, changes *git.CategorizedChanges, opts ChangelogOptions) (string, error) {
	const op = "version.GenerateChangelog"

	if changes == nil {
		return "", rperrors.Validation(op, "changes are required")
	}

	var sb strings.Builder

	versionStr := s.resolveVersionString(opts)
	dateStr := s.resolveDateString(opts)

	s.writeChangelogHeader(&sb, versionStr, dateStr, opts.Format)
	s.writeBreakingChanges(&sb, changes.Breaking, opts)
	s.writeCategorizedChanges(&sb, changes, opts)
	s.writeOtherChanges(&sb, changes.Other, opts)
	s.writeCompareURL(&sb, versionStr, opts)

	return sb.String(), nil
}

// resolveVersionString returns the version string for the changelog header.
func (s *ServiceImpl) resolveVersionString(opts ChangelogOptions) string {
	if opts.Version != nil {
		return opts.Version.String()
	}
	return "Unreleased"
}

// resolveDateString returns the date string for the changelog header.
func (s *ServiceImpl) resolveDateString(opts ChangelogOptions) string {
	if opts.Date != "" {
		return opts.Date
	}
	return time.Now().Format("2006-01-02")
}

// writeChangelogHeader writes the changelog header based on format.
func (s *ServiceImpl) writeChangelogHeader(sb *strings.Builder, version, date, format string) {
	switch format {
	case "keep-a-changelog":
		fmt.Fprintf(sb, "## [%s] - %s\n\n", version, date)
	case "conventional":
		fmt.Fprintf(sb, "# %s (%s)\n\n", version, date)
	default:
		fmt.Fprintf(sb, "## %s - %s\n\n", version, date)
	}
}

// writeBreakingChanges writes the breaking changes section.
func (s *ServiceImpl) writeBreakingChanges(sb *strings.Builder, breaking []git.ConventionalCommit, opts ChangelogOptions) {
	if len(breaking) == 0 {
		return
	}

	sb.WriteString("### ⚠ BREAKING CHANGES\n\n")
	for _, commit := range breaking {
		s.writeChangelogEntry(sb, commit, opts)
	}
	sb.WriteString("\n")
}

// changelogCategory represents a category of changes for the changelog.
type changelogCategory struct {
	commits []git.ConventionalCommit
	name    string
	ctype   git.CommitType
}

// writeCategorizedChanges writes all categorized change sections.
func (s *ServiceImpl) writeCategorizedChanges(sb *strings.Builder, changes *git.CategorizedChanges, opts ChangelogOptions) {
	categories := []changelogCategory{
		{changes.Features, "Features", git.CommitTypeFeat},
		{changes.Fixes, "Bug Fixes", git.CommitTypeFix},
		{changes.Performance, "Performance Improvements", git.CommitTypePerf},
		{changes.Refactoring, "Code Refactoring", git.CommitTypeRefactor},
		{changes.Documentation, "Documentation", git.CommitTypeDocs},
	}

	for _, cat := range categories {
		s.writeCategorySection(sb, cat, opts)
	}
}

// writeCategorySection writes a single category section.
func (s *ServiceImpl) writeCategorySection(sb *strings.Builder, cat changelogCategory, opts ChangelogOptions) {
	if len(cat.commits) == 0 {
		return
	}

	if slices.Contains(opts.Exclude, string(cat.ctype)) {
		return
	}

	displayName := s.getCategoryDisplayName(cat, opts)
	fmt.Fprintf(sb, "### %s\n\n", displayName)

	for _, commit := range cat.commits {
		if commit.Breaking {
			continue // Skip if already in breaking changes
		}
		s.writeChangelogEntry(sb, commit, opts)
	}
	sb.WriteString("\n")
}

// getCategoryDisplayName returns the display name for a category.
func (s *ServiceImpl) getCategoryDisplayName(cat changelogCategory, opts ChangelogOptions) string {
	if name, ok := opts.Categories[string(cat.ctype)]; ok {
		return name
	}
	return cat.name
}

// writeOtherChanges writes the other changes section if applicable.
func (s *ServiceImpl) writeOtherChanges(sb *strings.Builder, other []git.ConventionalCommit, opts ChangelogOptions) {
	if len(other) == 0 || slices.Contains(opts.Exclude, "other") {
		return
	}

	if !s.hasNonExcludedOtherChanges(other, opts) {
		return
	}

	sb.WriteString("### Other Changes\n\n")
	for _, commit := range other {
		if slices.Contains(opts.Exclude, string(commit.Type)) {
			continue
		}
		s.writeChangelogEntry(sb, commit, opts)
	}
	sb.WriteString("\n")
}

// hasNonExcludedOtherChanges checks if there are any non-excluded other changes.
func (s *ServiceImpl) hasNonExcludedOtherChanges(other []git.ConventionalCommit, opts ChangelogOptions) bool {
	for _, commit := range other {
		if !slices.Contains(opts.Exclude, string(commit.Type)) {
			return true
		}
	}
	return false
}

// writeCompareURL writes the compare URL if available.
func (s *ServiceImpl) writeCompareURL(sb *strings.Builder, version string, opts ChangelogOptions) {
	if opts.CompareURL != "" && opts.PreviousVersion != nil {
		fmt.Fprintf(sb, "[%s]: %s\n", version, opts.CompareURL)
	}
}

// writeChangelogEntry writes a single changelog entry.
func (s *ServiceImpl) writeChangelogEntry(sb *strings.Builder, commit git.ConventionalCommit, opts ChangelogOptions) {
	// Format: * **scope:** description (hash) - author
	sb.WriteString("* ")

	// Scope
	if commit.Scope != "" {
		fmt.Fprintf(sb, "**%s:** ", commit.Scope)
	}

	// Description
	sb.WriteString(commit.Description)

	// Commit hash
	if opts.IncludeCommitHash {
		hash := commit.Commit.ShortHash
		if opts.LinkCommits && opts.RepositoryURL != "" {
			fmt.Fprintf(sb, " ([%s](%s/commit/%s))", hash, opts.RepositoryURL, commit.Commit.Hash)
		} else {
			fmt.Fprintf(sb, " (%s)", hash)
		}
	}

	// Author
	if opts.IncludeAuthor {
		fmt.Fprintf(sb, " - %s", commit.Commit.Author.Name)
	}

	// Issue references
	if opts.LinkIssues && len(commit.References) > 0 {
		for _, ref := range commit.References {
			if opts.IssueURL != "" {
				issueURL := strings.ReplaceAll(opts.IssueURL, "{id}", ref.ID)
				fmt.Fprintf(sb, ", [#%s](%s)", ref.ID, issueURL)
			} else if opts.RepositoryURL != "" {
				fmt.Fprintf(sb, ", [#%s](%s/issues/%s)", ref.ID, opts.RepositoryURL, ref.ID)
			}
		}
	}

	sb.WriteString("\n")
}

// UpdateChangelogFile updates the changelog file with new content.
func (s *ServiceImpl) UpdateChangelogFile(_ context.Context, path string, content string, version *Version) error {
	const op = "version.UpdateChangelogFile"

	// Read existing changelog
	existingData, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return rperrors.IOWrap(err, op, "failed to read changelog file")
	}

	var newContent string

	if len(existingData) == 0 {
		// Create new changelog
		newContent = fmt.Sprintf("# Changelog\n\nAll notable changes to this project will be documented in this file.\n\nThe format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),\nand this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).\n\n%s", content)
	} else {
		// Insert new content after header
		existing := string(existingData)
		insertPos := findChangelogInsertPosition(existing)
		newContent = existing[:insertPos] + content + existing[insertPos:]
	}

	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return rperrors.IOWrap(err, op, "failed to write changelog file")
	}

	return nil
}

// findChangelogInsertPosition finds the position to insert new changelog content.
func findChangelogInsertPosition(content string) int {
	// Look for the first version header (## [x.x.x] or ## x.x.x)
	loc := versionHeaderRegex.FindStringIndex(content)
	if loc != nil {
		return loc[0]
	}

	// Look for "[Unreleased]" section
	loc = unreleasedRegex.FindStringIndex(content)
	if loc != nil {
		// Find the end of the Unreleased section
		remaining := content[loc[1]:]
		nextHeaderLoc := versionHeaderRegex.FindStringIndex(remaining)
		if nextHeaderLoc != nil {
			return loc[1] + nextHeaderLoc[0]
		}
		// No next header, insert at the end
		return len(content)
	}

	// No existing structure, insert after the header
	// Look for a blank line after the title
	lines := strings.Split(content, "\n")
	pos := 0
	inHeader := true
	for i, line := range lines {
		if inHeader {
			if strings.TrimSpace(line) == "" && i > 0 {
				inHeader = false
			}
		} else if strings.TrimSpace(line) != "" {
			// Found first non-blank line after header
			return pos
		}
		pos += len(line) + 1
	}

	return len(content)
}

// ReadChangelogSection reads a specific version's section from the changelog.
func (s *ServiceImpl) ReadChangelogSection(_ context.Context, path string, version string) (string, error) {
	const op = "version.ReadChangelogSection"

	file, err := os.Open(path)
	if err != nil {
		return "", rperrors.IOWrap(err, op, "failed to open changelog file")
	}
	defer file.Close()

	var section strings.Builder
	inSection := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if !inSection {
			if matchesVersionHeader(line, version) {
				inSection = true
				section.WriteString(line)
				section.WriteString("\n")
			}
		} else {
			// Check if we've hit the next version section
			if nextVersionRegex.MatchString(line) && !matchesVersionHeader(line, version) {
				break
			}
			section.WriteString(line)
			section.WriteString("\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return "", rperrors.IOWrap(err, op, "failed to read changelog file")
	}

	if !inSection {
		return "", rperrors.NotFound(op, fmt.Sprintf("version %s not found in changelog", version))
	}

	return strings.TrimSpace(section.String()), nil
}

// matchesVersionHeader checks if a line matches a version header.
// Matches formats: "## [1.0.0]" or "## 1.0.0"
// Uses string operations instead of regex for performance.
func matchesVersionHeader(line, version string) bool {
	if !strings.HasPrefix(line, "## ") {
		return false
	}
	rest := line[3:] // Skip "## "

	// Check for bracketed version: "[1.0.0]"
	if strings.HasPrefix(rest, "[") {
		return strings.HasPrefix(rest[1:], version) &&
			(len(rest) > len(version)+1 && rest[len(version)+1] == ']')
	}

	// Check for unbracketed version: "1.0.0"
	return strings.HasPrefix(rest, version) &&
		(len(rest) == len(version) || rest[len(version)] == ' ' || rest[len(version)] == ']')
}

// ValidateVersion validates a version string.
func (s *ServiceImpl) ValidateVersion(version string) error {
	_, err := s.ParseVersion(version)
	return err
}

// CompareVersions compares two versions and returns -1, 0, or 1.
func (s *ServiceImpl) CompareVersions(v1, v2 *Version) int {
	if v1.Major != v2.Major {
		if v1.Major < v2.Major {
			return -1
		}
		return 1
	}
	if v1.Minor != v2.Minor {
		if v1.Minor < v2.Minor {
			return -1
		}
		return 1
	}
	if v1.Patch != v2.Patch {
		if v1.Patch < v2.Patch {
			return -1
		}
		return 1
	}

	// Compare prerelease
	if v1.Prerelease == "" && v2.Prerelease != "" {
		return 1 // No prerelease is greater
	}
	if v1.Prerelease != "" && v2.Prerelease == "" {
		return -1
	}
	if v1.Prerelease < v2.Prerelease {
		return -1
	}
	if v1.Prerelease > v2.Prerelease {
		return 1
	}

	return 0
}

// IsPrerelease returns true if the version is a prerelease.
func (s *ServiceImpl) IsPrerelease(v *Version) bool {
	return v.Prerelease != ""
}
