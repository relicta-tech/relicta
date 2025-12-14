// Package communication provides domain types for release communication.
package communication

import (
	"fmt"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// NoteTone represents the tone of the release notes.
type NoteTone string

const (
	// ToneTechnical uses technical language for developers.
	ToneTechnical NoteTone = "technical"
	// ToneFriendly uses friendly language for general users.
	ToneFriendly NoteTone = "friendly"
	// ToneProfessional uses professional language for stakeholders.
	ToneProfessional NoteTone = "professional"
	// ToneMarketing uses marketing language for announcements.
	ToneMarketing NoteTone = "marketing"
)

// IsValid returns true if the tone is valid.
func (t NoteTone) IsValid() bool {
	switch t {
	case ToneTechnical, ToneFriendly, ToneProfessional, ToneMarketing:
		return true
	default:
		return false
	}
}

// NoteAudience represents the target audience for release notes.
type NoteAudience string

const (
	// AudienceDevelopers targets developers and technical users.
	AudienceDevelopers NoteAudience = "developers"
	// AudienceUsers targets end users.
	AudienceUsers NoteAudience = "users"
	// AudienceStakeholders targets business stakeholders.
	AudienceStakeholders NoteAudience = "stakeholders"
	// AudiencePublic targets the general public.
	AudiencePublic NoteAudience = "public"
)

// IsValid returns true if the audience is valid.
func (a NoteAudience) IsValid() bool {
	switch a {
	case AudienceDevelopers, AudienceUsers, AudienceStakeholders, AudiencePublic:
		return true
	default:
		return false
	}
}

// ReleaseNotes is a value object representing release notes.
type ReleaseNotes struct {
	version      version.SemanticVersion
	title        string
	summary      string
	highlights   []string
	sections     []NotesSection
	contributors []Contributor
	generatedAt  time.Time
	aiGenerated  bool
	tone         NoteTone
	audience     NoteAudience
}

// NotesSection represents a section in release notes.
type NotesSection struct {
	Title    string
	Icon     string
	Content  string
	Items    []string
	Priority int
}

// Contributor represents a contributor to the release.
type Contributor struct {
	Name     string
	Username string
	Email    string
	Commits  int
}

// ReleaseNotesBuilder builds ReleaseNotes using the builder pattern.
type ReleaseNotesBuilder struct {
	notes *ReleaseNotes
}

// NewReleaseNotesBuilder creates a new builder.
func NewReleaseNotesBuilder(ver version.SemanticVersion) *ReleaseNotesBuilder {
	return &ReleaseNotesBuilder{
		notes: &ReleaseNotes{
			version:     ver,
			generatedAt: time.Now(),
			tone:        ToneTechnical,
			audience:    AudienceDevelopers,
		},
	}
}

// WithTitle sets the title.
func (b *ReleaseNotesBuilder) WithTitle(title string) *ReleaseNotesBuilder {
	b.notes.title = title
	return b
}

// WithSummary sets the summary.
func (b *ReleaseNotesBuilder) WithSummary(summary string) *ReleaseNotesBuilder {
	b.notes.summary = summary
	return b
}

// WithHighlights sets the highlights.
func (b *ReleaseNotesBuilder) WithHighlights(highlights []string) *ReleaseNotesBuilder {
	b.notes.highlights = highlights
	return b
}

// AddSection adds a section.
func (b *ReleaseNotesBuilder) AddSection(section NotesSection) *ReleaseNotesBuilder {
	b.notes.sections = append(b.notes.sections, section)
	return b
}

// WithContributors sets the contributors.
func (b *ReleaseNotesBuilder) WithContributors(contributors []Contributor) *ReleaseNotesBuilder {
	b.notes.contributors = contributors
	return b
}

// WithTone sets the tone.
func (b *ReleaseNotesBuilder) WithTone(tone NoteTone) *ReleaseNotesBuilder {
	b.notes.tone = tone
	return b
}

// WithAudience sets the audience.
func (b *ReleaseNotesBuilder) WithAudience(audience NoteAudience) *ReleaseNotesBuilder {
	b.notes.audience = audience
	return b
}

// AIGenerated marks the notes as AI-generated.
func (b *ReleaseNotesBuilder) AIGenerated() *ReleaseNotesBuilder {
	b.notes.aiGenerated = true
	return b
}

// Build creates the ReleaseNotes.
func (b *ReleaseNotesBuilder) Build() *ReleaseNotes {
	return b.notes
}

// Version returns the version.
func (n *ReleaseNotes) Version() version.SemanticVersion {
	return n.version
}

// Title returns the title.
func (n *ReleaseNotes) Title() string {
	return n.title
}

// Summary returns the summary.
func (n *ReleaseNotes) Summary() string {
	return n.summary
}

// Highlights returns the highlights.
func (n *ReleaseNotes) Highlights() []string {
	return n.highlights
}

// Sections returns the sections.
func (n *ReleaseNotes) Sections() []NotesSection {
	return n.sections
}

// Contributors returns the contributors.
func (n *ReleaseNotes) Contributors() []Contributor {
	return n.contributors
}

// GeneratedAt returns when the notes were generated.
func (n *ReleaseNotes) GeneratedAt() time.Time {
	return n.generatedAt
}

// IsAIGenerated returns true if AI generated the notes.
func (n *ReleaseNotes) IsAIGenerated() bool {
	return n.aiGenerated
}

// Tone returns the tone.
func (n *ReleaseNotes) Tone() NoteTone {
	return n.tone
}

// Audience returns the audience.
func (n *ReleaseNotes) Audience() NoteAudience {
	return n.audience
}

// CreateFromChangeSet creates release notes from a changeset.
func CreateFromChangeSet(ver version.SemanticVersion, cs *changes.ChangeSet, opts ...func(*ReleaseNotesBuilder)) *ReleaseNotes {
	builder := NewReleaseNotesBuilder(ver)

	summary := cs.Summary()
	title := "Release " + ver.String()
	builder.WithTitle(title)

	// Create summary text
	var summaryParts []string
	if summary.Breaking > 0 {
		summaryParts = append(summaryParts, pluralize(summary.Breaking, "breaking change"))
	}
	if summary.Features > 0 {
		summaryParts = append(summaryParts, pluralize(summary.Features, "new feature"))
	}
	if summary.Fixes > 0 {
		summaryParts = append(summaryParts, pluralize(summary.Fixes, "bug fix"))
	}
	if summary.Performance > 0 {
		summaryParts = append(summaryParts, pluralize(summary.Performance, "performance improvement"))
	}

	if len(summaryParts) > 0 {
		builder.WithSummary("This release includes " + strings.Join(summaryParts, ", ") + ".")
	}

	// Add sections from categories
	cats := cs.Categories()

	if len(cats.Breaking) > 0 {
		var items []string
		for _, c := range cats.Breaking {
			items = append(items, c.FormattedSubject())
		}
		builder.AddSection(NotesSection{
			Title:    "⚠️ Breaking Changes",
			Items:    items,
			Priority: 1,
		})
	}

	if len(cats.Features) > 0 {
		var items []string
		for _, c := range cats.Features {
			items = append(items, c.FormattedSubject())
		}
		builder.AddSection(NotesSection{
			Title:    "✨ New Features",
			Items:    items,
			Priority: 2,
		})
	}

	if len(cats.Fixes) > 0 {
		var items []string
		for _, c := range cats.Fixes {
			items = append(items, c.FormattedSubject())
		}
		builder.AddSection(NotesSection{
			Title:    "🐛 Bug Fixes",
			Items:    items,
			Priority: 3,
		})
	}

	if len(cats.Perf) > 0 {
		var items []string
		for _, c := range cats.Perf {
			items = append(items, c.FormattedSubject())
		}
		builder.AddSection(NotesSection{
			Title:    "⚡ Performance Improvements",
			Items:    items,
			Priority: 4,
		})
	}

	// Apply options
	for _, opt := range opts {
		opt(builder)
	}

	return builder.Build()
}

// pluralize returns a pluralized string.
func pluralize(count int, singular string) string {
	if count == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %ss", count, singular)
}

// Render renders the release notes to markdown.
func (n *ReleaseNotes) Render() string {
	var sb strings.Builder
	// Pre-allocate estimated size: title + summary + highlights + sections
	estimatedSize := len(n.title) + len(n.summary) + 100
	for _, h := range n.highlights {
		estimatedSize += len(h) + 5
	}
	for _, section := range n.sections {
		estimatedSize += len(section.Title) + len(section.Content) + 50
		for _, item := range section.Items {
			estimatedSize += len(item) + 5
		}
	}
	sb.Grow(estimatedSize)

	// Title
	sb.WriteString("# ")
	sb.WriteString(n.title)
	sb.WriteString("\n\n")

	// Summary
	if n.summary != "" {
		sb.WriteString(n.summary)
		sb.WriteString("\n\n")
	}

	// Highlights
	if len(n.highlights) > 0 {
		sb.WriteString("## Highlights\n\n")
		for _, h := range n.highlights {
			sb.WriteString("- ")
			sb.WriteString(h)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Sections
	for _, section := range n.sections {
		sb.WriteString("## ")
		sb.WriteString(section.Title)
		sb.WriteString("\n\n")

		if section.Content != "" {
			sb.WriteString(section.Content)
			sb.WriteString("\n\n")
		}

		for _, item := range section.Items {
			sb.WriteString("- ")
			sb.WriteString(item)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Contributors
	if len(n.contributors) > 0 {
		sb.WriteString("## Contributors\n\n")
		sb.WriteString("Thanks to all our contributors for this release:\n\n")
		for _, c := range n.contributors {
			sb.WriteString("- ")
			if c.Username != "" {
				sb.WriteString("@")
				sb.WriteString(c.Username)
			} else {
				sb.WriteString(c.Name)
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
