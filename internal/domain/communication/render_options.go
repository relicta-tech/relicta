package communication

import (
	"sort"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// RenderOptions describes how a changelog entry is built and rendered.
//
// Every field here corresponds to a `changelog.*` setting that shipped with a default and was
// read by nothing: format, exclude, categories, include_commit_hash, include_author,
// include_date, link_commits. The defaults describe a Keep a Changelog renderer precisely —
// and what a release actually wrote was a flat list of commit subjects with no version
// heading, so consecutive releases ran together in one undifferentiated file.
//
// A domain value rather than the config struct: the translation from configuration happens at
// the edge, and the renderer stays testable without one.
type RenderOptions struct {
	// Format selects the rendering style. Only the entry layout differs; the section
	// contents are the same.
	Format ChangelogFormat

	// Exclude lists commit types omitted from the changelog entirely, such as "chore" and
	// "ci". A breaking change is never excluded — see BuildEntry.
	Exclude []string

	// Categories maps a commit type to its section title ("feat" -> "Features"). Types
	// absent from the map fall back to a readable form of the type itself, so a project
	// using a type nobody anticipated still sees its commits.
	Categories map[string]string

	// IncludeCommitHash appends the short hash to each item.
	IncludeCommitHash bool

	// IncludeAuthor appends the commit author to each item.
	IncludeAuthor bool

	// IncludeDate puts the release date beside the version heading.
	IncludeDate bool

	// LinkCommits renders the hash as a link into RepositoryURL. Ignored without one,
	// because a link needs somewhere to point.
	LinkCommits bool

	// RepositoryURL is the base URL used for commit links and the compare link.
	RepositoryURL string
}

// DefaultRenderOptions returns the rendering the config defaults describe, so a caller with no
// configuration to hand still produces a Keep a Changelog entry rather than a flat list.
func DefaultRenderOptions() RenderOptions {
	return RenderOptions{
		Format:            FormatKeepAChangelog,
		Exclude:           []string{"chore", "ci", "docs", "style", "test"},
		Categories:        DefaultCategories(),
		IncludeCommitHash: true,
		IncludeDate:       true,
	}
}

// DefaultCategories returns the commit type to section title mapping.
func DefaultCategories() map[string]string {
	return map[string]string{
		"feat":     "Features",
		"fix":      "Bug Fixes",
		"perf":     "Performance Improvements",
		"refactor": "Code Refactoring",
		"revert":   "Reverts",
		"build":    "Build System",
	}
}

// breakingSectionTitle heads the section for changes that break compatibility.
const breakingSectionTitle = "⚠ BREAKING CHANGES"

// sectionOrder is the order sections appear in, most consequential first. Types outside it
// follow, alphabetically, so the output is stable whatever a project's commit conventions are.
var sectionOrder = []string{"feat", "fix", "perf", "refactor", "revert", "build"}

// BuildEntry turns a changeset into one changelog entry, honoring the rendering options.
//
// Breaking changes come first and are never dropped by Exclude: a compatibility break is the
// single thing a reader cannot afford to miss, and it can arrive under any commit type —
// including one the project has chosen to exclude.
func BuildEntry(ver version.SemanticVersion, cs *changes.ChangeSet, opts RenderOptions) ChangelogEntry {
	entry := ChangelogEntry{Version: ver}
	if opts.IncludeDate {
		entry.Date = time.Now()
	}

	if cs == nil {
		return entry
	}

	if opts.RepositoryURL != "" && cs.FromRef() != "" {
		entry.CompareURL = opts.RepositoryURL + "/compare/" + cs.FromRef() + "..." + ver.TagString()
	}

	excluded := make(map[string]struct{}, len(opts.Exclude))
	for _, kind := range opts.Exclude {
		excluded[strings.ToLower(strings.TrimSpace(kind))] = struct{}{}
	}

	breaking := make([]ChangelogItem, 0)
	byType := make(map[string][]ChangelogItem)

	for _, commit := range cs.Commits() {
		if commit == nil {
			continue
		}

		if commit.IsBreaking() {
			item := itemFor(commit, opts)
			// The footer when the author wrote one, the subject otherwise. `feat!: drop
			// the v1 API` carries no footer, and a section reading "- breaking change"
			// tells a reader nothing about what broke.
			if msg := commit.BreakingMessage(); msg != "" {
				item.Description = msg
			}
			breaking = append(breaking, item)
			continue
		}

		kind := strings.ToLower(string(commit.Type()))
		if _, skip := excluded[kind]; skip {
			continue
		}
		byType[kind] = append(byType[kind], itemFor(commit, opts))
	}

	if len(breaking) > 0 {
		entry.Sections = append(entry.Sections, ChangelogSection{
			Title: breakingSectionTitle,
			Items: breaking,
		})
	}

	for _, kind := range orderedTypes(byType) {
		entry.Sections = append(entry.Sections, ChangelogSection{
			Title: sectionTitle(kind, opts.Categories),
			Items: byType[kind],
		})
	}

	return entry
}

// orderedTypes lists the commit types present, known ones first in sectionOrder, the rest
// alphabetically after them.
func orderedTypes(byType map[string][]ChangelogItem) []string {
	ordered := make([]string, 0, len(byType))
	seen := make(map[string]struct{}, len(byType))

	for _, kind := range sectionOrder {
		if _, present := byType[kind]; present {
			ordered = append(ordered, kind)
			seen[kind] = struct{}{}
		}
	}

	rest := make([]string, 0, len(byType))
	for kind := range byType {
		if _, already := seen[kind]; !already {
			rest = append(rest, kind)
		}
	}
	sort.Strings(rest)

	return append(ordered, rest...)
}

// sectionTitle names a section, falling back to the capitalized commit type so a project using
// an unanticipated type still gets a readable heading rather than a dropped section.
func sectionTitle(kind string, categories map[string]string) string {
	if title, ok := categories[kind]; ok && title != "" {
		return title
	}
	if kind == "" {
		return "Other Changes"
	}
	return strings.ToUpper(kind[:1]) + kind[1:]
}

// itemFor renders one commit into an item, carrying only what the options ask for.
func itemFor(commit *changes.ConventionalCommit, opts RenderOptions) ChangelogItem {
	item := ChangelogItem{
		Description: commit.Subject(),
		Scope:       commit.Scope(),
	}
	if opts.IncludeCommitHash {
		item.CommitHash = commit.ShortHash()
	}
	if opts.IncludeAuthor {
		item.Author = commit.Author()
	}
	return item
}
