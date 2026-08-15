package communication

import (
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// The whole changelog.* configuration block — format, exclude, categories, include_commit_hash,
// include_author, include_date, link_commits — had no reader outside the config package. Its
// defaults describe a Keep a Changelog renderer exactly, and a renderer producing that already
// existed here with no caller, while every release wrote a flat list of commit subjects with no
// version heading, so consecutive releases ran together in one undifferentiated file.

func commit(kind changes.CommitType, subject string, opts ...changes.ConventionalCommitOption) *changes.ConventionalCommit {
	return changes.NewConventionalCommit("abcdef1234567890", kind, subject, opts...)
}

func changeSetOf(commits ...*changes.ConventionalCommit) *changes.ChangeSet {
	cs := changes.NewChangeSet("test", "v0.9.0", "HEAD")
	cs.AddCommits(commits)
	return cs
}

func sectionTitles(entry ChangelogEntry) []string {
	titles := make([]string, 0, len(entry.Sections))
	for _, section := range entry.Sections {
		titles = append(titles, section.Title)
	}
	return titles
}

func TestExcludedTypesAreLeftOutOfTheChangelog(t *testing.T) {
	cs := changeSetOf(
		commit(changes.CommitTypeFeat, "add pagination"),
		commit(changes.CommitTypeChore, "bump the linter"),
		commit(changes.CommitTypeDocs, "expand the readme"),
	)

	entry := BuildEntry(version.MustParse("1.0.0"), cs, DefaultRenderOptions())

	titles := strings.Join(sectionTitles(entry), " | ")
	if !strings.Contains(titles, "Features") {
		t.Errorf("sections = %v, want the features section", titles)
	}
	if strings.Contains(titles, "Chore") || strings.Contains(titles, "Doc") {
		t.Errorf("sections = %v: changelog.exclude lists chore and docs by default", titles)
	}
}

// A compatibility break is the one thing a reader cannot afford to miss, and it can arrive
// under a type the project excludes.
func TestABreakingChangeSurvivesAnExcludedType(t *testing.T) {
	cs := changeSetOf(commit(changes.CommitTypeChore, "retire the old runtime",
		changes.WithBreaking("the v1 runtime is gone")))

	entry := BuildEntry(version.MustParse("2.0.0"), cs, DefaultRenderOptions())

	if len(entry.Sections) != 1 || entry.Sections[0].Title != breakingSectionTitle {
		t.Fatalf("sections = %v, want only the breaking section: excluding \"chore\" must "+
			"never hide a change that breaks compatibility", sectionTitles(entry))
	}
	if got := entry.Sections[0].Items[0].Description; got != "the v1 runtime is gone" {
		t.Errorf("description = %q, want the breaking message", got)
	}
}

// `feat!: drop the v1 API` carries no footer. Falling back to the subject is what stops the
// section reading "- breaking change" for every entry.
func TestABreakingChangeWithNoFooterUsesItsSubject(t *testing.T) {
	cs := changeSetOf(commit(changes.CommitTypeFeat, "drop the v1 API",
		changes.WithBreaking("")))

	entry := BuildEntry(version.MustParse("2.0.0"), cs, DefaultRenderOptions())

	if len(entry.Sections) == 0 {
		t.Fatal("no sections")
	}
	if got := entry.Sections[0].Items[0].Description; got != "drop the v1 API" {
		t.Errorf("description = %q, want the subject", got)
	}
}

func TestCategoriesNameTheSections(t *testing.T) {
	opts := DefaultRenderOptions()
	opts.Categories = map[string]string{"feat": "Neuerungen", "fix": "Fehlerbehebungen"}

	entry := BuildEntry(version.MustParse("1.0.0"), changeSetOf(
		commit(changes.CommitTypeFeat, "add pagination"),
		commit(changes.CommitTypeFix, "reject expired tokens"),
	), opts)

	titles := sectionTitles(entry)
	if len(titles) != 2 || titles[0] != "Neuerungen" || titles[1] != "Fehlerbehebungen" {
		t.Errorf("titles = %v, want the configured names in feat, fix order", titles)
	}
}

// A project using a commit type nobody anticipated must still see its commits: a missing
// category is a naming gap, not a reason to drop the change.
func TestAnUnknownTypeStillGetsASection(t *testing.T) {
	opts := DefaultRenderOptions()
	opts.Exclude = nil

	entry := BuildEntry(version.MustParse("1.0.0"),
		changeSetOf(commit(changes.CommitType("security"), "patch the parser")), opts)

	titles := sectionTitles(entry)
	if len(titles) != 1 || titles[0] != "Security" {
		t.Errorf("titles = %v, want [Security]: an unmapped type falls back to its own "+
			"name rather than vanishing", titles)
	}
}

func TestTheCommitHashAndAuthorAppearOnlyWhenAskedFor(t *testing.T) {
	cs := changeSetOf(commit(changes.CommitTypeFeat, "add pagination",
		changes.WithAuthor("Ada", "ada@example.com")))
	ver := version.MustParse("1.0.0")

	bare := DefaultRenderOptions()
	bare.IncludeCommitHash = false
	bare.IncludeAuthor = false
	if got := RenderSections(BuildEntry(ver, cs, bare), bare); strings.Contains(got, "(") {
		t.Errorf("rendered %q, want no trailing detail when both are off", got)
	}

	full := DefaultRenderOptions()
	full.IncludeAuthor = true
	got := RenderSections(BuildEntry(ver, cs, full), full)
	if !strings.Contains(got, "abcdef1") || !strings.Contains(got, "Ada") {
		t.Errorf("rendered %q, want the short hash and the author", got)
	}
}

func TestCommitsLinkIntoTheRepositoryWhenAsked(t *testing.T) {
	opts := DefaultRenderOptions()
	opts.LinkCommits = true
	opts.RepositoryURL = "https://github.com/owner/repo"

	got := RenderSections(BuildEntry(version.MustParse("1.0.0"),
		changeSetOf(commit(changes.CommitTypeFeat, "add pagination")), opts), opts)

	if !strings.Contains(got, "(https://github.com/owner/repo/commit/") {
		t.Errorf("rendered %q, want the hash linked into the repository", got)
	}
}

// Without a repository there is nowhere for a link to point, and a markdown link to nothing is
// worse than plain text.
func TestCommitsAreNotLinkedWithoutARepositoryURL(t *testing.T) {
	opts := DefaultRenderOptions()
	opts.LinkCommits = true

	got := RenderSections(BuildEntry(version.MustParse("1.0.0"),
		changeSetOf(commit(changes.CommitTypeFeat, "add pagination")), opts), opts)

	if strings.Contains(got, "](") {
		t.Errorf("rendered %q, want no link when no repository URL is configured", got)
	}
}

func TestSectionsAppearMostConsequentialFirst(t *testing.T) {
	opts := DefaultRenderOptions()
	opts.Exclude = nil

	entry := BuildEntry(version.MustParse("1.0.0"), changeSetOf(
		commit(changes.CommitTypeChore, "tidy up"),
		commit(changes.CommitTypeFix, "reject expired tokens"),
		commit(changes.CommitTypeFeat, "add pagination"),
		commit(changes.CommitTypeFeat, "retire v1", changes.WithBreaking("v1 is gone")),
	), opts)

	titles := sectionTitles(entry)
	if len(titles) < 4 {
		t.Fatalf("titles = %v, want four sections", titles)
	}
	if titles[0] != breakingSectionTitle || titles[1] != "Features" || titles[2] != "Bug Fixes" {
		t.Errorf("titles = %v, want breaking, features, fixes first", titles)
	}
	if titles[len(titles)-1] != "Chore" {
		t.Errorf("titles = %v, want the unranked type last", titles)
	}
}

// The heading is what makes a changelog file a sequence of releases rather than one long list.
func TestTheVersionHeadingCarriesTheVersionAndDate(t *testing.T) {
	entry := BuildEntry(version.MustParse("1.2.3"), nil, DefaultRenderOptions())

	heading := RenderVersionHeading(entry)
	if !strings.HasPrefix(heading, "## [1.2.3] - ") {
		t.Errorf("heading = %q, want \"## [1.2.3] - <date>\"", heading)
	}
}

func TestTheDateIsOmittedWhenNotWanted(t *testing.T) {
	opts := DefaultRenderOptions()
	opts.IncludeDate = false

	if got := RenderVersionHeading(BuildEntry(version.MustParse("1.2.3"), nil, opts)); got != "## [1.2.3]" {
		t.Errorf("heading = %q, want \"## [1.2.3]\" with include_date off", got)
	}
}

// changelog.group_by shipped with a default of "type", three documented values, and validation
// that accepts all three — while the renderer always grouped by type. "type" was accidentally
// right; "scope" and "none" were silently ignored.

func TestGroupingByScopeGivesEachComponentItsOwnSection(t *testing.T) {
	opts := DefaultRenderOptions()
	opts.GroupBy = GroupByScope

	entry := BuildEntry(version.MustParse("1.0.0"), changeSetOf(
		commit(changes.CommitTypeFeat, "add pagination", changes.WithScope("api")),
		commit(changes.CommitTypeFix, "reject expired tokens", changes.WithScope("api")),
		commit(changes.CommitTypeFeat, "add --json", changes.WithScope("cli")),
	), opts)

	titles := sectionTitles(entry)
	if len(titles) != 2 || titles[0] != "api" || titles[1] != "cli" {
		t.Fatalf("titles = %v, want [api cli]: group_by scope asks for one section per "+
			"component, in a stable order", titles)
	}
	if len(entry.Sections[0].Items) != 2 {
		t.Errorf("api section has %d items, want both api commits regardless of their type",
			len(entry.Sections[0].Items))
	}
}

// The heading names the scope, so repeating "**api:**" on every line beneath it is noise.
func TestGroupingByScopeDropsTheRepeatedScopePrefix(t *testing.T) {
	opts := DefaultRenderOptions()
	opts.GroupBy = GroupByScope

	got := RenderSections(BuildEntry(version.MustParse("1.0.0"), changeSetOf(
		commit(changes.CommitTypeFeat, "add pagination", changes.WithScope("api")),
	), opts), opts)

	if strings.Contains(got, "**api:**") {
		t.Errorf("rendered %q, want no scope prefix under the scope's own heading", got)
	}
}

// A change with no scope is still a change. Dropping it would lose it, and inventing a scope
// would print a component name the project does not use.
func TestUnscopedChangesGetTheCatchAllSectionLast(t *testing.T) {
	opts := DefaultRenderOptions()
	opts.GroupBy = GroupByScope

	entry := BuildEntry(version.MustParse("1.0.0"), changeSetOf(
		commit(changes.CommitTypeFix, "stop panicking"),
		commit(changes.CommitTypeFeat, "add pagination", changes.WithScope("api")),
	), opts)

	titles := sectionTitles(entry)
	if len(titles) != 2 || titles[0] != "api" || titles[1] != otherChangesTitle {
		t.Fatalf("titles = %v, want [api %q]: the unscoped change must appear, and after the "+
			"named components", titles, otherChangesTitle)
	}
	if got := entry.Sections[1].Items[0].Description; got != "stop panicking" {
		t.Errorf("catch-all item = %q, want the unscoped commit", got)
	}
}

// Exclude names commit types, and it goes on naming them whatever the sections are made of.
func TestExcludedTypesStayOutWhenGroupingByScope(t *testing.T) {
	opts := DefaultRenderOptions()
	opts.GroupBy = GroupByScope

	entry := BuildEntry(version.MustParse("1.0.0"), changeSetOf(
		commit(changes.CommitTypeChore, "bump the linter", changes.WithScope("deps")),
		commit(changes.CommitTypeFeat, "add pagination", changes.WithScope("api")),
	), opts)

	if titles := sectionTitles(entry); len(titles) != 1 || titles[0] != "api" {
		t.Errorf("titles = %v, want [api]: a chore is a chore whether sections are types or "+
			"scopes", titles)
	}
}

func TestNoGroupingRendersOneUnheadedList(t *testing.T) {
	opts := DefaultRenderOptions()
	opts.GroupBy = GroupByNone

	entry := BuildEntry(version.MustParse("1.0.0"), changeSetOf(
		commit(changes.CommitTypeFeat, "add pagination"),
		commit(changes.CommitTypeFix, "reject expired tokens"),
		commit(changes.CommitTypePerf, "cache the tag lookup"),
	), opts)

	if len(entry.Sections) != 1 {
		t.Fatalf("sections = %v, want one flat list", sectionTitles(entry))
	}
	if len(entry.Sections[0].Items) != 3 {
		t.Errorf("items = %d, want all three: a flat list still lists everything",
			len(entry.Sections[0].Items))
	}

	if got := RenderSections(entry, opts); strings.Contains(got, "###") {
		t.Errorf("rendered %q, want no section heading with group_by none", got)
	}
}

// group_by says how the ordinary changes are organized. It is not a statement that the reader
// stopped caring what breaks, so the guarantee that a break leads and is never dropped holds
// under every grouping — folding those items namelessly into a flat list would leave a
// compatibility break rendered indistinguishably from a typo fix.
func TestABreakingChangeStillLeadsUnderEveryGrouping(t *testing.T) {
	for _, grouping := range []ChangelogGrouping{GroupByType, GroupByScope, GroupByNone} {
		t.Run(string(grouping), func(t *testing.T) {
			opts := DefaultRenderOptions()
			opts.GroupBy = grouping

			entry := BuildEntry(version.MustParse("2.0.0"), changeSetOf(
				commit(changes.CommitTypeFeat, "add pagination", changes.WithScope("api")),
				commit(changes.CommitTypeChore, "retire the old runtime",
					changes.WithBreaking("the v1 runtime is gone")),
			), opts)

			titles := sectionTitles(entry)
			if len(titles) == 0 || titles[0] != breakingSectionTitle {
				t.Fatalf("titles = %v, want the breaking section first with group_by %q",
					titles, grouping)
			}
			if got := entry.Sections[0].Items[0].Description; got != "the v1 runtime is gone" {
				t.Errorf("description = %q, want the breaking message: excluding \"chore\" "+
					"must not hide a break under any grouping", got)
			}
		})
	}
}

// changelog.link_issues and changelog.issue_url were both validated and both unread, because
// ChangelogItem.IssueRefs was populated by nothing.

func issueCommit(t *testing.T) *changes.ConventionalCommit {
	t.Helper()

	message := "feat(api): add cursor pagination\n\nCloses: #123\n"
	parsed := changes.ParseConventionalCommit("abcdef1234567890", message,
		changes.WithRawMessage(message))
	if parsed == nil {
		t.Fatal("ParseConventionalCommit returned nil")
	}
	return parsed
}

func TestIssueReferencesLinkIntoTheIssueTracker(t *testing.T) {
	opts := DefaultRenderOptions()
	opts.LinkIssues = true
	opts.IssueURL = "https://github.com/owner/repo/issues"

	got := RenderSections(BuildEntry(version.MustParse("1.0.0"),
		changeSetOf(issueCommit(t)), opts), opts)

	if !strings.Contains(got, "[#123](https://github.com/owner/repo/issues/123)") {
		t.Errorf("rendered %q, want #123 linked into the tracker: an issue_url with no "+
			"placeholder is the tracker's base URL and the number is appended", got)
	}
}

// The setting is described as a URL pattern, and "{id}" is the placeholder the codebase
// already spells. A tracker whose issue number is not at the end of the path needs it.
func TestAnIssueURLPatternPutsTheNumberWhereItBelongs(t *testing.T) {
	opts := DefaultRenderOptions()
	opts.LinkIssues = true
	opts.IssueURL = "https://jira.example.com/browse/PROJ-" + IssueIDPlaceholder + "?src=changelog"

	got := RenderSections(BuildEntry(version.MustParse("1.0.0"),
		changeSetOf(issueCommit(t)), opts), opts)

	want := "[#123](https://jira.example.com/browse/PROJ-123?src=changelog)"
	if !strings.Contains(got, want) {
		t.Errorf("rendered %q, want %q: the number goes where the pattern puts it, not at the "+
			"end of the URL", got, want)
	}
}

// Validation rejects this combination, but a renderer handed it anyway must not emit a link to
// nothing — a reader who sees "#123" can find the issue, while "[#123]()" wastes a click.
func TestIssueReferencesStayPlainTextWithoutATrackerURL(t *testing.T) {
	opts := DefaultRenderOptions()
	opts.LinkIssues = true

	got := RenderSections(BuildEntry(version.MustParse("1.0.0"),
		changeSetOf(issueCommit(t)), opts), opts)

	if !strings.Contains(got, "#123") {
		t.Errorf("rendered %q, want the reference kept as text", got)
	}
	if strings.Contains(got, "[#123](") {
		t.Errorf("rendered %q, want no link when no issue URL is configured", got)
	}
}

// link_issues is the only setting that asks for issue references at all, so with it off an
// entry has to look exactly as it did before any of this existed.
func TestNoIssueReferencesAppearWhenLinkIssuesIsOff(t *testing.T) {
	opts := DefaultRenderOptions()
	opts.IssueURL = "https://github.com/owner/repo/issues"

	got := RenderSections(BuildEntry(version.MustParse("1.0.0"),
		changeSetOf(issueCommit(t)), opts), opts)

	if strings.Contains(got, "#123") {
		t.Errorf("rendered %q, want no issue reference with link_issues off", got)
	}
}
