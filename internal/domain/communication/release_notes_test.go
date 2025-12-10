// Package communication provides domain types for release communication.
package communication

import (
	"strings"
	"testing"

	"github.com/felixgeelhaar/release-pilot/internal/domain/changes"
	"github.com/felixgeelhaar/release-pilot/internal/domain/version"
)

func TestNoteTone_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		tone  NoteTone
		valid bool
	}{
		{"technical", ToneTechnical, true},
		{"friendly", ToneFriendly, true},
		{"professional", ToneProfessional, true},
		{"marketing", ToneMarketing, true},
		{"invalid", NoteTone("invalid"), false},
		{"empty", NoteTone(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tone.IsValid(); got != tt.valid {
				t.Errorf("NoteTone(%q).IsValid() = %v, want %v", tt.tone, got, tt.valid)
			}
		})
	}
}

func TestNoteToneConstants(t *testing.T) {
	tests := []struct {
		tone NoteTone
		want string
	}{
		{ToneTechnical, "technical"},
		{ToneFriendly, "friendly"},
		{ToneProfessional, "professional"},
		{ToneMarketing, "marketing"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.tone) != tt.want {
				t.Errorf("NoteTone constant = %v, want %v", tt.tone, tt.want)
			}
		})
	}
}

func TestNoteAudience_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		audience NoteAudience
		valid    bool
	}{
		{"developers", AudienceDevelopers, true},
		{"users", AudienceUsers, true},
		{"stakeholders", AudienceStakeholders, true},
		{"public", AudiencePublic, true},
		{"invalid", NoteAudience("invalid"), false},
		{"empty", NoteAudience(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.audience.IsValid(); got != tt.valid {
				t.Errorf("NoteAudience(%q).IsValid() = %v, want %v", tt.audience, got, tt.valid)
			}
		})
	}
}

func TestNoteAudienceConstants(t *testing.T) {
	tests := []struct {
		audience NoteAudience
		want     string
	}{
		{AudienceDevelopers, "developers"},
		{AudienceUsers, "users"},
		{AudienceStakeholders, "stakeholders"},
		{AudiencePublic, "public"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.audience) != tt.want {
				t.Errorf("NoteAudience constant = %v, want %v", tt.audience, tt.want)
			}
		})
	}
}

func TestNewReleaseNotesBuilder(t *testing.T) {
	ver := version.MustParse("1.0.0")
	builder := NewReleaseNotesBuilder(ver)

	if builder == nil {
		t.Fatal("NewReleaseNotesBuilder returned nil")
	}
	if builder.notes == nil {
		t.Fatal("builder.notes is nil")
	}
	if builder.notes.version.String() != "1.0.0" {
		t.Errorf("Version = %v, want 1.0.0", builder.notes.version.String())
	}
	// Check defaults
	if builder.notes.tone != ToneTechnical {
		t.Errorf("Default tone = %v, want technical", builder.notes.tone)
	}
	if builder.notes.audience != AudienceDevelopers {
		t.Errorf("Default audience = %v, want developers", builder.notes.audience)
	}
}

func TestReleaseNotesBuilder_Fluent(t *testing.T) {
	ver := version.MustParse("2.0.0")

	notes := NewReleaseNotesBuilder(ver).
		WithTitle("Release 2.0.0").
		WithSummary("This is a major release").
		WithHighlights([]string{"Feature A", "Feature B"}).
		AddSection(NotesSection{Title: "Breaking", Items: []string{"Change 1"}}).
		WithContributors([]Contributor{{Name: "John", Commits: 5}}).
		WithTone(ToneFriendly).
		WithAudience(AudienceUsers).
		AIGenerated().
		Build()

	if notes.Version().String() != "2.0.0" {
		t.Errorf("Version = %v, want 2.0.0", notes.Version().String())
	}
	if notes.Title() != "Release 2.0.0" {
		t.Errorf("Title = %v, want Release 2.0.0", notes.Title())
	}
	if notes.Summary() != "This is a major release" {
		t.Errorf("Summary = %v, want This is a major release", notes.Summary())
	}
	if len(notes.Highlights()) != 2 {
		t.Errorf("Highlights length = %v, want 2", len(notes.Highlights()))
	}
	if len(notes.Sections()) != 1 {
		t.Errorf("Sections length = %v, want 1", len(notes.Sections()))
	}
	if len(notes.Contributors()) != 1 {
		t.Errorf("Contributors length = %v, want 1", len(notes.Contributors()))
	}
	if notes.Tone() != ToneFriendly {
		t.Errorf("Tone = %v, want friendly", notes.Tone())
	}
	if notes.Audience() != AudienceUsers {
		t.Errorf("Audience = %v, want users", notes.Audience())
	}
	if !notes.IsAIGenerated() {
		t.Error("IsAIGenerated should be true")
	}
	if notes.GeneratedAt().IsZero() {
		t.Error("GeneratedAt should not be zero")
	}
}

func TestReleaseNotes_Render(t *testing.T) {
	ver := version.MustParse("1.0.0")

	notes := NewReleaseNotesBuilder(ver).
		WithTitle("Release 1.0.0").
		WithSummary("A new release").
		WithHighlights([]string{"New feature"}).
		AddSection(NotesSection{
			Title:   "Features",
			Content: "New features added",
			Items:   []string{"Feature 1", "Feature 2"},
		}).
		WithContributors([]Contributor{
			{Username: "johndoe"},
			{Name: "Jane Smith"},
		}).
		Build()

	rendered := notes.Render()

	// Check title
	if !strings.Contains(rendered, "# Release 1.0.0") {
		t.Error("Rendered output should contain title")
	}

	// Check summary
	if !strings.Contains(rendered, "A new release") {
		t.Error("Rendered output should contain summary")
	}

	// Check highlights
	if !strings.Contains(rendered, "## Highlights") {
		t.Error("Rendered output should contain Highlights section")
	}
	if !strings.Contains(rendered, "- New feature") {
		t.Error("Rendered output should contain highlight items")
	}

	// Check section
	if !strings.Contains(rendered, "## Features") {
		t.Error("Rendered output should contain Features section")
	}
	if !strings.Contains(rendered, "- Feature 1") {
		t.Error("Rendered output should contain feature items")
	}

	// Check contributors
	if !strings.Contains(rendered, "## Contributors") {
		t.Error("Rendered output should contain Contributors section")
	}
	if !strings.Contains(rendered, "@johndoe") {
		t.Error("Rendered output should contain username with @")
	}
	if !strings.Contains(rendered, "Jane Smith") {
		t.Error("Rendered output should contain name")
	}
}

func TestReleaseNotes_Render_Empty(t *testing.T) {
	ver := version.MustParse("0.1.0")
	notes := NewReleaseNotesBuilder(ver).
		WithTitle("Release 0.1.0").
		Build()

	rendered := notes.Render()

	if !strings.Contains(rendered, "# Release 0.1.0") {
		t.Error("Rendered output should contain title")
	}
}

func TestNotesSection_Fields(t *testing.T) {
	section := NotesSection{
		Title:    "Features",
		Icon:     "✨",
		Content:  "New features",
		Items:    []string{"Feature 1"},
		Priority: 1,
	}

	if section.Title != "Features" {
		t.Errorf("Title = %v, want Features", section.Title)
	}
	if section.Icon != "✨" {
		t.Errorf("Icon = %v, want ✨", section.Icon)
	}
	if section.Content != "New features" {
		t.Errorf("Content = %v, want New features", section.Content)
	}
	if len(section.Items) != 1 {
		t.Errorf("Items length = %v, want 1", len(section.Items))
	}
	if section.Priority != 1 {
		t.Errorf("Priority = %v, want 1", section.Priority)
	}
}

func TestContributor_Fields(t *testing.T) {
	contributor := Contributor{
		Name:     "John Doe",
		Username: "johndoe",
		Email:    "john@example.com",
		Commits:  10,
	}

	if contributor.Name != "John Doe" {
		t.Errorf("Name = %v, want John Doe", contributor.Name)
	}
	if contributor.Username != "johndoe" {
		t.Errorf("Username = %v, want johndoe", contributor.Username)
	}
	if contributor.Email != "john@example.com" {
		t.Errorf("Email = %v, want john@example.com", contributor.Email)
	}
	if contributor.Commits != 10 {
		t.Errorf("Commits = %v, want 10", contributor.Commits)
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		count    int
		singular string
		want     string
	}{
		{1, "feature", "1 feature"},
		{2, "feature", "2 features"},
		{0, "bug", "0 bugs"},
		{5, "change", "5 changes"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := pluralize(tt.count, tt.singular); got != tt.want {
				t.Errorf("pluralize(%d, %q) = %v, want %v", tt.count, tt.singular, got, tt.want)
			}
		})
	}
}

func TestCreateFromChangeSet(t *testing.T) {
	ver := version.MustParse("1.1.0")

	// Create test commits
	feat := changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "add feature")
	fix := changes.NewConventionalCommit("def456", changes.CommitTypeFix, "fix bug")
	breaking := changes.NewConventionalCommit("ghi789", changes.CommitTypeFeat, "breaking", changes.WithBreaking("API changed"))
	perf := changes.NewConventionalCommit("jkl012", changes.CommitTypePerf, "improve speed")

	cs := changes.NewChangeSet("test", "v1.0.0", "HEAD")
	cs.AddCommits([]*changes.ConventionalCommit{feat, fix, breaking, perf})

	notes := CreateFromChangeSet(ver, cs)

	if notes.Version().String() != "1.1.0" {
		t.Errorf("Version = %v, want 1.1.0", notes.Version().String())
	}
	if notes.Title() != "Release 1.1.0" {
		t.Errorf("Title = %v, want Release 1.1.0", notes.Title())
	}

	// Should have summary
	if notes.Summary() == "" {
		t.Error("Summary should not be empty")
	}

	// Should have 4 sections (breaking, features, fixes, perf)
	sections := notes.Sections()
	if len(sections) != 4 {
		t.Errorf("Sections length = %v, want 4", len(sections))
	}
}

func TestCreateFromChangeSet_WithOptions(t *testing.T) {
	ver := version.MustParse("1.0.0")
	cs := changes.NewChangeSet("test", "v0.9.0", "HEAD")

	notes := CreateFromChangeSet(ver, cs, func(b *ReleaseNotesBuilder) {
		b.WithTone(ToneMarketing)
		b.WithAudience(AudiencePublic)
	})

	if notes.Tone() != ToneMarketing {
		t.Errorf("Tone = %v, want marketing", notes.Tone())
	}
	if notes.Audience() != AudiencePublic {
		t.Errorf("Audience = %v, want public", notes.Audience())
	}
}
