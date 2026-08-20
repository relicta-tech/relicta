package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// workflow.auto_commit_changelog defaults to true and was read by nothing, which left a release
// describing itself nowhere it counted. Verified against the shipped binary before the fix: a
// full release left an uncommitted CHANGELOG.md and a modified package.json behind, and
// `git show v0.1.0:package.json` reported 0.0.0 for a release tagged 0.1.0.
//
// Then require_clean_working_tree started being enforced, and the two settings contradicted
// each other outright: `relicta bump` writes the configured version manifests, so by publish
// time package.json was modified — by relicta, one step earlier in the same release — and
// publish refused the dirty tree it had itself created.
//
// So the gate has to ignore exactly the files the release is about to commit, and no others.

func withConfig(t *testing.T) *config.Config {
	t.Helper()

	orig := cfg
	t.Cleanup(func() { cfg = orig })
	cfg = config.DefaultConfig()
	cfg.Changelog.File = "CHANGELOG.md"
	cfg.Versioning.VersionFiles = []config.VersionTarget{{Path: "package.json", Key: "version"}}
	return cfg
}

func TestTheGateIgnoresTheFilesTheReleaseIsAboutToCommit(t *testing.T) {
	withConfig(t)

	kept := withoutReleaseCommitPaths(context.Background(), []string{"package.json", "CHANGELOG.md"})
	if len(kept) != 0 {
		t.Errorf("the gate still reports %v.\nbump wrote those one step earlier in this same "+
			"release and the release commit is about to include them, so counting them as the "+
			"operator's uncommitted work makes version_files and require_clean_working_tree "+
			"refuse every publish", kept)
	}
}

// The gate's actual job, which the exclusion must not weaken.
func TestTheGateStillReportsTheOperatorsOwnWork(t *testing.T) {
	withConfig(t)

	kept := withoutReleaseCommitPaths(context.Background(), []string{"package.json", "internal/server.go"})
	if len(kept) != 1 || kept[0] != "internal/server.go" {
		t.Errorf("kept = %v, want [internal/server.go]: work relicta does not commit is "+
			"exactly what the tag will not contain, which is the reason the gate exists", kept)
	}
}

// With auto-commit off relicta commits nothing, so nothing may be excused. The operator is
// managing these files by hand and their being uncommitted is the truth worth reporting.
func TestNothingIsExcusedWhenRelictaCommitsNothing(t *testing.T) {
	c := withConfig(t)
	c.Workflow.AutoCommitChangelog = false

	kept := withoutReleaseCommitPaths(context.Background(), []string{"package.json", "CHANGELOG.md"})
	if len(kept) != 2 {
		t.Errorf("kept = %v, want both: with auto_commit_changelog off no release commit "+
			"happens, so these stay uncommitted and the gate must say so", kept)
	}
}

// A project that commits .relicta/ — what `git add -A` does once, and how a team keeps its
// governance history in the repository — found its second release blocked by the first one's
// bookkeeping: publish rewrites .relicta/releases/latest, so the tree was dirty before the
// operator had touched anything. Verified end to end: two consecutive releases refused at the
// second until this exclusion existed.
func TestRelictasOwnStoreIsNotTheOperatorsUncommittedWork(t *testing.T) {
	c := withConfig(t)
	// Not about committing, so it must hold with auto-commit off as well.
	c.Workflow.AutoCommitChangelog = false

	kept := withoutReleaseCommitPaths(context.Background(), []string{
		".relicta/releases/latest",
		".relicta/releases/run-3f6023.json",
		"internal/server.go",
	})
	if len(kept) != 1 || kept[0] != "internal/server.go" {
		t.Errorf("kept = %v, want [internal/server.go]: relicta's own run state is not work "+
			"the operator forgot to commit, and counting it blocks every release after the "+
			"first in any repository that tracks .relicta/", kept)
	}
}

func TestTheReleaseCommitCoversTheChangelogAndEveryVersionFile(t *testing.T) {
	withConfig(t)

	paths := releaseCommitPaths(context.Background())
	want := map[string]bool{"CHANGELOG.md": false, "package.json": false}
	for _, p := range paths {
		if _, ok := want[p]; ok {
			want[p] = true
		}
	}
	for path, found := range want {
		if !found {
			t.Errorf("%s is not in the release commit (%v): a tag that omits it points at a "+
				"commit that does not describe the release it names", path, paths)
		}
	}
}

// The changelog is now written before the tag, so a publish that fails afterwards leaves the
// entry behind and the retry must not insert it twice.
func TestARetryDoesNotWriteTheSameChangelogEntryTwice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CHANGELOG.md")
	notes := "## [0.1.0] - 2026-08-14\n\n### Fixed\n\n- a bugfix"

	if err := updateChangelogFile(path, notes); err != nil {
		t.Fatalf("updateChangelogFile: %v", err)
	}
	if !changelogAlreadyContains(path, notes) {
		t.Fatal("the entry just written is not recognized, so a retried publish would " +
			"insert 0.1.0 into the changelog a second time")
	}

	if changelogAlreadyContains(path, "## [0.2.0] - 2026-08-15\n\n### Added\n\n- a feature") {
		t.Error("a different version is reported as already present, which would silence " +
			"the changelog for every release after the first")
	}
}

func TestAnAbsentChangelogIsNotReportedAsAlreadyWritten(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "CHANGELOG.md")

	if changelogAlreadyContains(missing, "## [0.1.0]\n- something") {
		t.Error("a changelog that does not exist reported the entry as present, which would " +
			"skip writing it entirely")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Error("changelogAlreadyContains created the file")
	}
}

// Before this, the changelog file was a flat list of bullets with nothing marking where one
// release ended and the next began — and findVersionEntryPoint, which exists to insert a new
// release above the last one, had no "## [" to find and appended to the bottom instead, so the
// file also ran backwards.
func TestTheChangelogEntryOpensWithAVersionHeading(t *testing.T) {
	rel := newNotesReadyRelease(t, "heading")

	entry := changelogEntryFor(rel)
	if !strings.HasPrefix(entry, "## [1.0.0] - ") {
		t.Errorf("entry begins %q, want a \"## [1.0.0] - <date>\" heading: without one every "+
			"release runs into the previous one", firstLine(entry))
	}
	if !strings.Contains(entry, "Test release notes") {
		t.Error("the notes are missing from the entry")
	}
}

// An AI provider asked to write in Keep a Changelog style supplies its own heading. Adding a
// second one would leave the file with two headings for one release.
func TestNotesThatAlreadyCarryAHeadingAreNotGivenASecond(t *testing.T) {
	rel := newNotesReadyRelease(t, "own-heading")
	if err := rel.UpdateNotesText("## [1.0.0] - 2026-08-15\n\n### Features\n\n- something"); err != nil {
		t.Fatalf("UpdateNotesText: %v", err)
	}

	entry := changelogEntryFor(rel)
	if strings.Count(entry, "## [1.0.0]") != 1 {
		t.Errorf("entry carries %d version headings:\n%s", strings.Count(entry, "## [1.0.0]"), entry)
	}
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return s[:i]
	}
	return s
}
