// Package communication provides domain types for release communication.
package communication

import (
	"strings"
	"testing"
	"time"

	"github.com/felixgeelhaar/release-pilot/internal/domain/changes"
	"github.com/felixgeelhaar/release-pilot/internal/domain/version"
)

func TestChangelogFormat_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		format ChangelogFormat
		valid  bool
	}{
		{"keep-a-changelog", FormatKeepAChangelog, true},
		{"conventional", FormatConventional, true},
		{"simple", FormatSimple, true},
		{"invalid", ChangelogFormat("invalid"), false},
		{"empty", ChangelogFormat(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.format.IsValid(); got != tt.valid {
				t.Errorf("ChangelogFormat(%q).IsValid() = %v, want %v", tt.format, got, tt.valid)
			}
		})
	}
}

func TestChangelogFormatConstants(t *testing.T) {
	tests := []struct {
		format ChangelogFormat
		want   string
	}{
		{FormatKeepAChangelog, "keep-a-changelog"},
		{FormatConventional, "conventional"},
		{FormatSimple, "simple"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.format) != tt.want {
				t.Errorf("ChangelogFormat constant = %v, want %v", tt.format, tt.want)
			}
		})
	}
}

func TestNewChangelog(t *testing.T) {
	cl := NewChangelog("Changelog", FormatKeepAChangelog)

	if cl.Title() != "Changelog" {
		t.Errorf("Title = %v, want Changelog", cl.Title())
	}
	if cl.Format() != FormatKeepAChangelog {
		t.Errorf("Format = %v, want keep-a-changelog", cl.Format())
	}
	if cl.Description() != "" {
		t.Errorf("Description should be empty, got %v", cl.Description())
	}
	if len(cl.Entries()) != 0 {
		t.Errorf("Entries should be empty, got %d entries", len(cl.Entries()))
	}
}

func TestChangelog_SetDescription(t *testing.T) {
	cl := NewChangelog("Changelog", FormatSimple)
	cl.SetDescription("All notable changes to this project")

	if cl.Description() != "All notable changes to this project" {
		t.Errorf("Description = %v, want All notable changes to this project", cl.Description())
	}
}

func TestChangelog_AddEntry(t *testing.T) {
	cl := NewChangelog("Changelog", FormatConventional)

	entry1 := ChangelogEntry{
		Version: version.MustParse("1.0.0"),
		Date:    time.Now(),
	}
	entry2 := ChangelogEntry{
		Version: version.MustParse("2.0.0"),
		Date:    time.Now(),
	}

	cl.AddEntry(entry1)
	cl.AddEntry(entry2)

	entries := cl.Entries()
	if len(entries) != 2 {
		t.Fatalf("Entries length = %v, want 2", len(entries))
	}

	// Newest first (entry2 added last, should be first)
	if entries[0].Version.String() != "2.0.0" {
		t.Errorf("First entry version = %v, want 2.0.0", entries[0].Version.String())
	}
	if entries[1].Version.String() != "1.0.0" {
		t.Errorf("Second entry version = %v, want 1.0.0", entries[1].Version.String())
	}
}

func TestChangelog_LatestEntry(t *testing.T) {
	cl := NewChangelog("Changelog", FormatSimple)

	// Empty changelog
	if cl.LatestEntry() != nil {
		t.Error("LatestEntry should be nil for empty changelog")
	}

	// Add entry
	entry := ChangelogEntry{
		Version: version.MustParse("1.0.0"),
	}
	cl.AddEntry(entry)

	latest := cl.LatestEntry()
	if latest == nil {
		t.Fatal("LatestEntry should not be nil")
	}
	if latest.Version.String() != "1.0.0" {
		t.Errorf("LatestEntry version = %v, want 1.0.0", latest.Version.String())
	}
}

func TestChangelogEntry_Fields(t *testing.T) {
	now := time.Now()
	entry := ChangelogEntry{
		Version: version.MustParse("1.5.0"),
		Date:    now,
		Sections: []ChangelogSection{
			{Title: "Features", Items: []ChangelogItem{{Description: "New feature"}}},
		},
		CompareURL:   "https://github.com/owner/repo/compare/v1.4.0...v1.5.0",
		IsUnreleased: false,
	}

	if entry.Version.String() != "1.5.0" {
		t.Errorf("Version = %v, want 1.5.0", entry.Version.String())
	}
	if !entry.Date.Equal(now) {
		t.Errorf("Date = %v, want %v", entry.Date, now)
	}
	if len(entry.Sections) != 1 {
		t.Errorf("Sections length = %v, want 1", len(entry.Sections))
	}
	if entry.CompareURL != "https://github.com/owner/repo/compare/v1.4.0...v1.5.0" {
		t.Errorf("CompareURL unexpected value")
	}
	if entry.IsUnreleased {
		t.Error("IsUnreleased should be false")
	}
}

func TestChangelogEntry_Unreleased(t *testing.T) {
	entry := ChangelogEntry{
		IsUnreleased: true,
	}

	if !entry.IsUnreleased {
		t.Error("IsUnreleased should be true")
	}
}

func TestChangelogSection_Fields(t *testing.T) {
	section := ChangelogSection{
		Title: "Bug Fixes",
		Items: []ChangelogItem{
			{Description: "Fix crash", CommitHash: "abc1234"},
			{Description: "Fix memory leak", Scope: "core"},
		},
	}

	if section.Title != "Bug Fixes" {
		t.Errorf("Title = %v, want Bug Fixes", section.Title)
	}
	if len(section.Items) != 2 {
		t.Errorf("Items length = %v, want 2", len(section.Items))
	}
}

func TestChangelogItem_AllFields(t *testing.T) {
	item := ChangelogItem{
		Description: "Add new API endpoint",
		Scope:       "api",
		CommitHash:  "abc1234",
		Author:      "johndoe",
		IssueRefs:   []string{"#123", "#456"},
		PRRefs:      []string{"#100"},
	}

	if item.Description != "Add new API endpoint" {
		t.Errorf("Description = %v, want Add new API endpoint", item.Description)
	}
	if item.Scope != "api" {
		t.Errorf("Scope = %v, want api", item.Scope)
	}
	if item.CommitHash != "abc1234" {
		t.Errorf("CommitHash = %v, want abc1234", item.CommitHash)
	}
	if item.Author != "johndoe" {
		t.Errorf("Author = %v, want johndoe", item.Author)
	}
	if len(item.IssueRefs) != 2 {
		t.Errorf("IssueRefs length = %v, want 2", len(item.IssueRefs))
	}
	if len(item.PRRefs) != 1 {
		t.Errorf("PRRefs length = %v, want 1", len(item.PRRefs))
	}
}

func TestCreateEntryFromChangeSet(t *testing.T) {
	ver := version.MustParse("1.0.0")

	// Create test commits
	feat := changes.NewConventionalCommit("abc1234567", changes.CommitTypeFeat, "add feature", changes.WithScope("api"))
	fix := changes.NewConventionalCommit("def4567890", changes.CommitTypeFix, "fix bug")
	breaking := changes.NewConventionalCommit("ghi7891234", changes.CommitTypeFeat, "change api", changes.WithBreaking("API signature changed"))
	perf := changes.NewConventionalCommit("jkl0123456", changes.CommitTypePerf, "improve speed")

	cs := changes.NewChangeSet("test", "v0.9.0", "HEAD")
	cs.AddCommits([]*changes.ConventionalCommit{feat, fix, breaking, perf})

	entry := CreateEntryFromChangeSet(ver, cs, "https://github.com/owner/repo")

	if entry.Version.String() != "1.0.0" {
		t.Errorf("Version = %v, want 1.0.0", entry.Version.String())
	}
	if entry.Date.IsZero() {
		t.Error("Date should not be zero")
	}
	if entry.CompareURL == "" {
		t.Error("CompareURL should not be empty")
	}
	if !strings.Contains(entry.CompareURL, "v0.9.0...v1.0.0") {
		t.Errorf("CompareURL should contain version range, got %v", entry.CompareURL)
	}

	// Should have 4 sections
	if len(entry.Sections) != 4 {
		t.Errorf("Sections length = %v, want 4", len(entry.Sections))
	}
}

func TestCreateEntryFromChangeSet_BreakingMessageUsed(t *testing.T) {
	ver := version.MustParse("2.0.0")

	breaking := changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "breaking change",
		changes.WithBreaking("This is the breaking change description"))

	cs := changes.NewChangeSet("test", "v1.0.0", "HEAD")
	cs.AddCommits([]*changes.ConventionalCommit{breaking})

	entry := CreateEntryFromChangeSet(ver, cs, "")

	// First section should be breaking changes
	if len(entry.Sections) < 1 {
		t.Fatal("Should have at least one section")
	}

	breakingSection := entry.Sections[0]
	if breakingSection.Title != "⚠ BREAKING CHANGES" {
		t.Errorf("First section title = %v, want ⚠ BREAKING CHANGES", breakingSection.Title)
	}
	if len(breakingSection.Items) != 1 {
		t.Fatal("Breaking section should have 1 item")
	}
	// Should use breaking message, not subject
	if breakingSection.Items[0].Description != "This is the breaking change description" {
		t.Errorf("Breaking description = %v, want This is the breaking change description", breakingSection.Items[0].Description)
	}
}

func TestCreateEntryFromChangeSet_NoRepoURL(t *testing.T) {
	ver := version.MustParse("1.0.0")
	cs := changes.NewChangeSet("test", "v0.9.0", "HEAD")

	entry := CreateEntryFromChangeSet(ver, cs, "")

	if entry.CompareURL != "" {
		t.Errorf("CompareURL should be empty when no repo URL, got %v", entry.CompareURL)
	}
}

func TestChangelog_Render(t *testing.T) {
	cl := NewChangelog("Changelog", FormatKeepAChangelog)
	cl.SetDescription("All notable changes to this project")

	entry := ChangelogEntry{
		Version: version.MustParse("1.0.0"),
		Date:    time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Sections: []ChangelogSection{
			{
				Title: "Features",
				Items: []ChangelogItem{
					{Description: "Add new feature", Scope: "api", CommitHash: "abc1234"},
				},
			},
			{
				Title: "Bug Fixes",
				Items: []ChangelogItem{
					{Description: "Fix crash"},
				},
			},
		},
	}
	cl.AddEntry(entry)

	rendered := cl.Render()

	// Check title
	if !strings.Contains(rendered, "# Changelog") {
		t.Error("Rendered output should contain title")
	}

	// Check description
	if !strings.Contains(rendered, "All notable changes to this project") {
		t.Error("Rendered output should contain description")
	}

	// Check version header
	if !strings.Contains(rendered, "## [1.0.0] - 2024-01-15") {
		t.Error("Rendered output should contain version header with date")
	}

	// Check sections
	if !strings.Contains(rendered, "### Features") {
		t.Error("Rendered output should contain Features section")
	}
	if !strings.Contains(rendered, "### Bug Fixes") {
		t.Error("Rendered output should contain Bug Fixes section")
	}

	// Check items with scope and hash
	if !strings.Contains(rendered, "**api:**") {
		t.Error("Rendered output should contain scope in bold")
	}
	if !strings.Contains(rendered, "(abc1234)") {
		t.Error("Rendered output should contain commit hash in parentheses")
	}
}

func TestChangelog_Render_Unreleased(t *testing.T) {
	cl := NewChangelog("Changelog", FormatSimple)

	entry := ChangelogEntry{
		IsUnreleased: true,
		Sections: []ChangelogSection{
			{Title: "Features", Items: []ChangelogItem{{Description: "WIP feature"}}},
		},
	}
	cl.AddEntry(entry)

	rendered := cl.Render()

	if !strings.Contains(rendered, "## [Unreleased]") {
		t.Error("Rendered output should contain [Unreleased] header")
	}
}

func TestChangelog_Render_Empty(t *testing.T) {
	cl := NewChangelog("CHANGELOG", FormatConventional)

	rendered := cl.Render()

	if !strings.Contains(rendered, "# CHANGELOG") {
		t.Error("Rendered output should contain title even when empty")
	}
}

func TestChangelog_Render_MultipleEntries(t *testing.T) {
	cl := NewChangelog("Changelog", FormatSimple)

	entry1 := ChangelogEntry{
		Version: version.MustParse("1.0.0"),
		Date:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	entry2 := ChangelogEntry{
		Version: version.MustParse("2.0.0"),
		Date:    time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
	}

	cl.AddEntry(entry1)
	cl.AddEntry(entry2)

	rendered := cl.Render()

	// Check both versions are present
	if !strings.Contains(rendered, "[1.0.0]") {
		t.Error("Rendered output should contain 1.0.0")
	}
	if !strings.Contains(rendered, "[2.0.0]") {
		t.Error("Rendered output should contain 2.0.0")
	}

	// 2.0.0 should appear before 1.0.0 (newest first)
	idx1 := strings.Index(rendered, "[1.0.0]")
	idx2 := strings.Index(rendered, "[2.0.0]")
	if idx2 > idx1 {
		t.Error("2.0.0 should appear before 1.0.0 in rendered output")
	}
}

func TestChangelog_RenderEntries_NoHeader(t *testing.T) {
	cl := NewChangelog("Changelog", FormatKeepAChangelog)

	entry := ChangelogEntry{
		Version: version.MustParse("1.0.0"),
		Date:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Sections: []ChangelogSection{
			{
				Title: "Features",
				Items: []ChangelogItem{
					{Description: "New feature", CommitHash: "abc1234"},
				},
			},
		},
	}

	cl.AddEntry(entry)

	// RenderEntries should NOT include the "# Changelog" header
	rendered := cl.RenderEntries()

	if strings.Contains(rendered, "# Changelog") {
		t.Error("RenderEntries should not include '# Changelog' header")
	}

	if !strings.Contains(rendered, "## [1.0.0]") {
		t.Error("RenderEntries should include version entry")
	}

	if !strings.Contains(rendered, "New feature") {
		t.Error("RenderEntries should include features")
	}
}

func TestChangelog_RenderEntries_VsRender(t *testing.T) {
	cl := NewChangelog("Changelog", FormatSimple)

	entry := ChangelogEntry{
		Version: version.MustParse("1.0.0"),
		Date:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	cl.AddEntry(entry)

	// Render() should include header
	full := cl.Render()
	if !strings.HasPrefix(full, "# Changelog") {
		t.Error("Render() should start with '# Changelog'")
	}

	// RenderEntries() should NOT include header (starts with "# " for h1)
	// but it WILL start with "## " for h2 version entries
	entries := cl.RenderEntries()
	if strings.HasPrefix(entries, "# ") && !strings.HasPrefix(entries, "## ") {
		t.Error("RenderEntries() should not start with '# ' (h1 header)")
	}

	// Both should contain the version entry
	if !strings.Contains(full, "## [1.0.0]") || !strings.Contains(entries, "## [1.0.0]") {
		t.Error("Both should contain the version entry")
	}
}
