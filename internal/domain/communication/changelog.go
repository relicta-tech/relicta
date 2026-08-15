// Package communication provides domain types for release communication.
package communication

import (
	"strings"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// ChangelogFormat represents the format of a changelog.
type ChangelogFormat string

const (
	// FormatKeepAChangelog follows the Keep a Changelog format.
	FormatKeepAChangelog ChangelogFormat = "keep-a-changelog"
	// FormatConventional follows the Conventional Changelog format.
	FormatConventional ChangelogFormat = "conventional"
	// FormatSimple uses a simple markdown format.
	FormatSimple ChangelogFormat = "simple"
)

// IsValid returns true if the format is valid.
func (f ChangelogFormat) IsValid() bool {
	switch f {
	case FormatKeepAChangelog, FormatConventional, FormatSimple:
		return true
	default:
		return false
	}
}

// ChangelogEntry represents a single entry in the changelog.
type ChangelogEntry struct {
	Version      version.SemanticVersion
	Date         time.Time
	Sections     []ChangelogSection
	CompareURL   string
	IsUnreleased bool
}

// ChangelogSection represents a section within a changelog entry.
type ChangelogSection struct {
	Title string
	Items []ChangelogItem
}

// ChangelogItem represents a single item in a changelog section.
type ChangelogItem struct {
	Description string
	Scope       string
	CommitHash  string
	Author      string
	IssueRefs   []string
	PRRefs      []string
}

// Changelog is a value object representing a complete changelog.
type Changelog struct {
	title       string
	description string
	entries     []ChangelogEntry
	format      ChangelogFormat
}

// NewChangelog creates a new Changelog.
func NewChangelog(title string, format ChangelogFormat) *Changelog {
	return &Changelog{
		title:   title,
		format:  format,
		entries: make([]ChangelogEntry, 0),
	}
}

// Title returns the changelog title.
func (c *Changelog) Title() string {
	return c.title
}

// Description returns the changelog description.
func (c *Changelog) Description() string {
	return c.description
}

// SetDescription sets the changelog description.
func (c *Changelog) SetDescription(desc string) {
	c.description = desc
}

// Format returns the changelog format.
func (c *Changelog) Format() ChangelogFormat {
	return c.format
}

// Entries returns all changelog entries.
func (c *Changelog) Entries() []ChangelogEntry {
	return c.entries
}

// AddEntry adds a new entry to the changelog.
func (c *Changelog) AddEntry(entry ChangelogEntry) {
	// Insert at the beginning (newest first)
	c.entries = append([]ChangelogEntry{entry}, c.entries...)
}

// LatestEntry returns the most recent entry.
func (c *Changelog) LatestEntry() *ChangelogEntry {
	if len(c.entries) == 0 {
		return nil
	}
	return &c.entries[0]
}

// Render renders the changelog to a string including header.
func (c *Changelog) Render() string {
	var sb strings.Builder
	// Pre-allocate estimated size: header + description + entries
	estimatedSize := len(c.title) + len(c.description) + 100
	for _, entry := range c.entries {
		estimatedSize += 100 + len(entry.Version.String())
		for _, section := range entry.Sections {
			estimatedSize += 50 + len(section.Title)
			for _, item := range section.Items {
				estimatedSize += len(item.Description) + 10
			}
		}
	}
	sb.Grow(estimatedSize)

	// Header
	sb.WriteString("# ")
	sb.WriteString(c.title)
	sb.WriteString("\n\n")

	if c.description != "" {
		sb.WriteString(c.description)
		sb.WriteString("\n\n")
	}

	// Entries
	for _, entry := range c.entries {
		c.renderEntry(&sb, entry)
		sb.WriteString("\n")
	}

	return sb.String()
}

// RenderEntries renders only the changelog entries without the header.
// Use this when inserting into an existing changelog file.
func (c *Changelog) RenderEntries() string {
	var sb strings.Builder
	// Pre-allocate estimated size for entries only
	estimatedSize := 0
	for _, entry := range c.entries {
		estimatedSize += 100 + len(entry.Version.String())
		for _, section := range entry.Sections {
			estimatedSize += 50 + len(section.Title)
			for _, item := range section.Items {
				estimatedSize += len(item.Description) + 10
			}
		}
	}
	sb.Grow(estimatedSize)

	// Entries only, no header
	for i, entry := range c.entries {
		c.renderEntry(&sb, entry)
		if i < len(c.entries)-1 {
			sb.WriteString("\n")
		}
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

// renderEntry renders a single changelog entry.
func (c *Changelog) renderEntry(sb *strings.Builder, entry ChangelogEntry) {
	sb.WriteString(RenderVersionHeading(entry))
	sb.WriteString("\n\n")
	sb.WriteString(RenderSections(entry, RenderOptions{}))
}

// RenderVersionHeading renders the "## [1.2.0] - 2026-08-15" line that separates one release
// from the next.
//
// Separated from the sections because the two belong to different places: the notes describe
// what changed and are shown wherever a release is announced, while the heading is what makes
// a changelog *file* a sequence of releases rather than one long list. Publish writes the
// heading when it inserts an entry, so notes written by an AI provider get one too.
func RenderVersionHeading(entry ChangelogEntry) string {
	if entry.IsUnreleased {
		return "## [Unreleased]"
	}

	heading := "## [" + entry.Version.String() + "]"
	if !entry.Date.IsZero() {
		heading += " - " + entry.Date.Format("2006-01-02")
	}
	return heading
}

// RenderSections renders an entry's sections without its version heading.
func RenderSections(entry ChangelogEntry, opts RenderOptions) string {
	var sb strings.Builder

	for _, section := range entry.Sections {
		// An untitled section is the flat list group_by: none asks for. Writing "### "
		// with nothing after it would leave an empty heading in the markdown.
		if section.Title != "" {
			sb.WriteString("### ")
			sb.WriteString(section.Title)
			sb.WriteString("\n\n")
		}

		for _, item := range section.Items {
			sb.WriteString("- ")
			if item.Scope != "" {
				sb.WriteString("**")
				sb.WriteString(item.Scope)
				sb.WriteString(":** ")
			}
			sb.WriteString(item.Description)
			sb.WriteString(renderItemSuffix(item, opts))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderItemSuffix renders the trailing "(hash, author, #123)" an item carries, linking the
// hash and the issue references when asked and when there is somewhere for a link to point.
func renderItemSuffix(item ChangelogItem, opts RenderOptions) string {
	parts := make([]string, 0, 2+len(item.IssueRefs))

	if item.CommitHash != "" {
		hash := item.CommitHash
		if opts.LinkCommits && opts.RepositoryURL != "" {
			hash = "[" + hash + "](" + opts.RepositoryURL + "/commit/" + item.CommitHash + ")"
		}
		parts = append(parts, hash)
	}
	if item.Author != "" {
		parts = append(parts, item.Author)
	}
	for _, ref := range item.IssueRefs {
		parts = append(parts, renderIssueRef(ref, opts.IssueURL))
	}

	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

// renderIssueRef renders one "#123", as a link when there is a tracker to link into.
//
// The plain-text fallback matters because link_issues without issue_url is a configuration the
// validator rejects but the renderer can still be handed — and a reader who sees "#123" can
// find the issue, while "[#123]()" only wastes a click.
func renderIssueRef(ref, issueURL string) string {
	if issueURL == "" {
		return ref
	}

	number := strings.TrimPrefix(ref, "#")
	target := strings.ReplaceAll(issueURL, IssueIDPlaceholder, number)
	if target == issueURL {
		// No placeholder, so the setting is the tracker's base URL rather than a
		// pattern: "https://github.com/owner/repo/issues" + "/123".
		target = strings.TrimSuffix(issueURL, "/") + "/" + number
	}
	return "[" + ref + "](" + target + ")"
}
