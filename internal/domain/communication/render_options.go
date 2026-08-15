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

	// GroupBy decides what an entry's sections are: one per commit type, one per commit
	// scope, or none at all. The zero value groups by type, which is the shipped default.
	GroupBy ChangelogGrouping

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

	// LinkIssues puts the issue references a commit declares beside it. It governs whether
	// they appear at all, not merely whether they are hyperlinks: nothing else in the
	// configuration asks for issue references, so with this off an entry looks exactly as
	// it did before.
	LinkIssues bool

	// RepositoryURL is the base URL used for commit links and the compare link.
	RepositoryURL string

	// IssueURL is where an issue reference points. Either a pattern containing
	// IssueIDPlaceholder, or a base URL the issue number is appended to. Without it the
	// references render as plain text, because a link to nothing is worse than no link.
	IssueURL string
}

// ChangelogGrouping selects what an entry's sections are.
type ChangelogGrouping string

const (
	// GroupByType gives each commit type its own section: Features, Bug Fixes, and so on.
	GroupByType ChangelogGrouping = "type"
	// GroupByScope gives each commit scope its own section, for projects whose readers
	// think in components rather than in kinds of change.
	GroupByScope ChangelogGrouping = "scope"
	// GroupByNone puts every change in one unheaded list.
	GroupByNone ChangelogGrouping = "none"
)

// IsValid reports whether the grouping is one this renderer knows.
func (g ChangelogGrouping) IsValid() bool {
	switch g {
	case GroupByType, GroupByScope, GroupByNone:
		return true
	default:
		return false
	}
}

// IssueIDPlaceholder is what an issue_url pattern puts where the issue number goes.
//
// The setting ships described as "the issue tracker URL pattern", and nothing consumed it, so
// the convention had to be chosen from what the tree already spells. Two spellings appear in
// its fixtures — "{id}" and a printf "%s" — and this is the one honored: it names what it
// stands for, and it cannot be produced by accident the way a stray percent sign can.
//
// A value with no placeholder is the tracker's base URL and the number is appended to it. That
// is the form the config validation's own tests use, and it is what someone types when the
// description says "URL".
const IssueIDPlaceholder = "{id}"

// otherChangesTitle heads the catch-all section: an unnamed commit type when grouping by type,
// and the commits that named no scope when grouping by scope.
//
// Unscoped commits cannot simply be dropped — a change with no scope is still a change — and
// inventing a scope would put a component name in the changelog that exists nowhere in the
// project. Reusing the heading an unnamed type already gets means a reader meets one catch-all
// rather than two.
const otherChangesTitle = "Other Changes"

// DefaultRenderOptions returns the rendering the config defaults describe, so a caller with no
// configuration to hand still produces a Keep a Changelog entry rather than a flat list.
func DefaultRenderOptions() RenderOptions {
	return RenderOptions{
		Format:            FormatKeepAChangelog,
		GroupBy:           GroupByType,
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
// including one the project has chosen to exclude. That holds under every grouping. GroupBy
// says how the ordinary changes are organized; it is not a statement that the reader has
// stopped caring what breaks, so even "none" — a deliberately flat list — keeps the breaking
// section ahead of it. Folding those items namelessly into the list would leave a
// compatibility break rendered indistinguishably from a typo fix.
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
	byGroup := make(map[string][]ChangelogItem)

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

		// Exclude still names commit types under every grouping: a chore is a chore
		// whether the sections are types, scopes, or absent.
		key, item := groupKeyFor(kind, commit, opts)
		byGroup[key] = append(byGroup[key], item)
	}

	if len(breaking) > 0 {
		entry.Sections = append(entry.Sections, ChangelogSection{
			Title: breakingSectionTitle,
			Items: breaking,
		})
	}

	for _, key := range orderedGroups(byGroup, opts.GroupBy) {
		entry.Sections = append(entry.Sections, ChangelogSection{
			Title: groupTitle(key, opts),
			Items: byGroup[key],
		})
	}

	return entry
}

// groupKeyFor decides which section a commit belongs to, and hands back the item to put there.
func groupKeyFor(kind string, commit *changes.ConventionalCommit, opts RenderOptions) (string, ChangelogItem) {
	item := itemFor(commit, opts)

	switch opts.GroupBy {
	case GroupByScope:
		// The heading names the scope, so repeating "**api:**" on every item beneath it
		// is noise the reader has to look past on every line.
		scope := item.Scope
		item.Scope = ""
		return strings.TrimSpace(scope), item
	case GroupByNone:
		return "", item
	default:
		// GroupByType, the zero value, and anything a future config could name that this
		// renderer does not know: grouping by type is the shipped default, and falling
		// back to it costs a reader nothing.
		return kind, item
	}
}

// orderedGroups lists the sections to render, in the order they appear.
func orderedGroups(byGroup map[string][]ChangelogItem, grouping ChangelogGrouping) []string {
	if grouping == GroupByNone {
		if len(byGroup) == 0 {
			return nil
		}
		return []string{""}
	}
	if grouping == GroupByScope {
		return orderedScopes(byGroup)
	}
	return orderedTypes(byGroup)
}

// orderedScopes lists the scopes present alphabetically, with the unscoped catch-all last.
//
// Alphabetically because a project's scopes have no inherent ranking the way commit types do —
// nothing says "api" matters more than "cli" — and an arbitrary but stable order at least
// means two releases of the same project read the same way. The catch-all goes last because a
// named component tells a reader more than "everything else" does.
func orderedScopes(byGroup map[string][]ChangelogItem) []string {
	scopes := make([]string, 0, len(byGroup))
	unscoped := false

	for scope := range byGroup {
		if scope == "" {
			unscoped = true
			continue
		}
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)

	if unscoped {
		scopes = append(scopes, "")
	}
	return scopes
}

// groupTitle names a section for the grouping in force.
func groupTitle(key string, opts RenderOptions) string {
	switch opts.GroupBy {
	case GroupByScope:
		if key == "" {
			return otherChangesTitle
		}
		// Verbatim: Categories maps commit types, and a scope that happens to be named
		// "feat" is a component, not a kind of change. Scopes are identifiers a project
		// chose — "api", "cli" — and capitalizing them would print a name the project
		// does not use.
		return key
	case GroupByNone:
		// No heading at all. RenderSections leaves the "### " line out for an empty
		// title, which is what makes the flat list flat.
		return ""
	default:
		return sectionTitle(key, opts.Categories)
	}
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
		return otherChangesTitle
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
	// Gated on the setting rather than always parsed, because link_issues is the only thing
	// in the configuration that asks for issue references at all: with it off, an entry has
	// to look exactly as it did before.
	if opts.LinkIssues {
		item.IssueRefs = commit.IssueRefs()
	}
	return item
}
