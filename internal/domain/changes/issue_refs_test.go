package changes

import (
	"strings"
	"testing"
)

// ChangelogItem carries an IssueRefs field that nothing populated, so changelog.link_issues and
// changelog.issue_url — both shipped, both validated — had nothing to render. These tests are
// about what counts as a reference, because that is the whole risk: a reference that is not one
// sends a reader to the wrong ticket.

func refsOf(t *testing.T, message string) []string {
	t.Helper()

	commit := ParseConventionalCommit("abcdef1234567890", message, WithRawMessage(message))
	if commit == nil {
		t.Fatalf("ParseConventionalCommit(%q) returned nil", message)
	}
	return commit.IssueRefs()
}

func TestATrailerDeclaresAnIssueReference(t *testing.T) {
	refs := refsOf(t, "feat(api): add cursor pagination\n\nCloses: #123\n")

	if len(refs) != 1 || refs[0] != "#123" {
		t.Errorf("refs = %v, want [#123]: \"Closes: #123\" is the author saying this commit "+
			"acts on that issue", refs)
	}
}

// The convention makes the colon optional, and "Fixes #45" is how most people write it.
func TestATrailerWithoutAColonIsStillATrailer(t *testing.T) {
	refs := refsOf(t, "fix: reject expired tokens\n\nFixes #45\n")

	if len(refs) != 1 || refs[0] != "#45" {
		t.Errorf("refs = %v, want [#45]", refs)
	}
}

func TestOneTrailerCanNameSeveralIssues(t *testing.T) {
	refs := refsOf(t, "feat(cli): add --json to status\n\nRefs #7, #8\n")

	if strings.Join(refs, ",") != "#7,#8" {
		t.Errorf("refs = %v, want [#7 #8]: dropping the second reference loses an issue the "+
			"author listed", refs)
	}
}

// The specific over-eagerness this guards against: a "#" followed by digits in a paragraph is
// discussion, not a claim that the release resolved anything.
func TestProseThatMentionsANumberIsNotAnIssueReference(t *testing.T) {
	refs := refsOf(t, "fix: stop panicking on an empty changeset\n\n"+
		"The retry budget of #999 is unrelated prose that mentions a number.\n")

	if len(refs) != 0 {
		t.Errorf("refs = %v, want none: a number mentioned in prose is not an issue this "+
			"commit closed, and linking it points the reader at the wrong ticket", refs)
	}
}

// A squash merge leaves the pull request number in the subject. It is already in front of the
// reader as text, and issue_url points at an issue tracker, which for a project tracking work
// outside its forge is not the same place.
func TestAPullRequestNumberInTheSubjectIsNotAnIssueReference(t *testing.T) {
	refs := refsOf(t, "fix(api): reject expired tokens (#45)")

	if len(refs) != 0 {
		t.Errorf("refs = %v, want none: the number is already rendered in the item text, so "+
			"linking it prints #45 twice on one line", refs)
	}
}

func TestAnIssueNamedTwiceIsListedOnce(t *testing.T) {
	refs := refsOf(t, "fix: correct the retry budget\n\nFixes #12\nCloses: #12\n")

	if len(refs) != 1 {
		t.Errorf("refs = %v, want [#12] once", refs)
	}
}

// Commits rebuilt from the state file carry their parsed parts rather than a raw message, and a
// changelog is rendered from a run that was planned earlier.
func TestReferencesAreFoundWithoutARawMessage(t *testing.T) {
	commit := NewConventionalCommit("abcdef1234567890", CommitTypeFix, "reject expired tokens",
		WithFooter("Closes: #123"))

	if refs := commit.IssueRefs(); len(refs) != 1 || refs[0] != "#123" {
		t.Errorf("refs = %v, want [#123]: a run reloaded from disk must render the same "+
			"changelog as one held in memory", refs)
	}
}

func TestACommitWithNoReferencesHasNone(t *testing.T) {
	if refs := refsOf(t, "perf: cache the tag lookup"); len(refs) != 0 {
		t.Errorf("refs = %v, want none", refs)
	}
}
