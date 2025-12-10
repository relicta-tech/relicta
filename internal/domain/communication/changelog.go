// Package communication provides domain types for release communication.
package communication

import (
	"fmt"
	"strings"
	"time"

	"github.com/felixgeelhaar/release-pilot/internal/domain/changes"
	"github.com/felixgeelhaar/release-pilot/internal/domain/version"
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

// CreateEntryFromChangeSet creates a changelog entry from a changeset.
func CreateEntryFromChangeSet(ver version.SemanticVersion, cs *changes.ChangeSet, repoURL string) ChangelogEntry {
	entry := ChangelogEntry{
		Version: ver,
		Date:    time.Now(),
	}

	if repoURL != "" && cs.FromRef() != "" {
		entry.CompareURL = fmt.Sprintf("%s/compare/%s...%s", repoURL, cs.FromRef(), ver.TagString())
	}

	cats := cs.Categories()

	// Breaking changes
	if len(cats.Breaking) > 0 {
		section := ChangelogSection{Title: "⚠ BREAKING CHANGES"}
		for _, commit := range cats.Breaking {
			item := ChangelogItem{
				Description: commit.Subject(),
				Scope:       commit.Scope(),
				CommitHash:  commit.ShortHash(),
			}
			if commit.BreakingMessage() != "" {
				item.Description = commit.BreakingMessage()
			}
			section.Items = append(section.Items, item)
		}
		entry.Sections = append(entry.Sections, section)
	}

	// Features
	if len(cats.Features) > 0 {
		section := ChangelogSection{Title: "Features"}
		for _, commit := range cats.Features {
			section.Items = append(section.Items, ChangelogItem{
				Description: commit.Subject(),
				Scope:       commit.Scope(),
				CommitHash:  commit.ShortHash(),
			})
		}
		entry.Sections = append(entry.Sections, section)
	}

	// Bug Fixes
	if len(cats.Fixes) > 0 {
		section := ChangelogSection{Title: "Bug Fixes"}
		for _, commit := range cats.Fixes {
			section.Items = append(section.Items, ChangelogItem{
				Description: commit.Subject(),
				Scope:       commit.Scope(),
				CommitHash:  commit.ShortHash(),
			})
		}
		entry.Sections = append(entry.Sections, section)
	}

	// Performance
	if len(cats.Perf) > 0 {
		section := ChangelogSection{Title: "Performance Improvements"}
		for _, commit := range cats.Perf {
			section.Items = append(section.Items, ChangelogItem{
				Description: commit.Subject(),
				Scope:       commit.Scope(),
				CommitHash:  commit.ShortHash(),
			})
		}
		entry.Sections = append(entry.Sections, section)
	}

	return entry
}

// Render renders the changelog to a string.
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

// renderEntry renders a single changelog entry.
func (c *Changelog) renderEntry(sb *strings.Builder, entry ChangelogEntry) {
	// Version header
	if entry.IsUnreleased {
		sb.WriteString("## [Unreleased]")
	} else {
		sb.WriteString("## [")
		sb.WriteString(entry.Version.String())
		sb.WriteString("]")
		if !entry.Date.IsZero() {
			sb.WriteString(" - ")
			sb.WriteString(entry.Date.Format("2006-01-02"))
		}
	}
	sb.WriteString("\n\n")

	// Sections
	for _, section := range entry.Sections {
		sb.WriteString("### ")
		sb.WriteString(section.Title)
		sb.WriteString("\n\n")

		for _, item := range section.Items {
			sb.WriteString("- ")
			if item.Scope != "" {
				sb.WriteString("**")
				sb.WriteString(item.Scope)
				sb.WriteString(":** ")
			}
			sb.WriteString(item.Description)
			if item.CommitHash != "" {
				sb.WriteString(" (")
				sb.WriteString(item.CommitHash)
				sb.WriteString(")")
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
}
