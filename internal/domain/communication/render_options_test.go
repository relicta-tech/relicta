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
