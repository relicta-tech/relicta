package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// `relicta release` carried two gaps that `relicta publish` and `relicta bump` did not, and
// both were verified against the built binary before being fixed here.
//
// --skip-push was the worse one. It selected a success message and nothing else, so the command
// printed "Created 0.1.0 locally (push skipped)" and "Run 'git push origin --tags' to publish
// when ready" while the tag was already on the remote. Not merely ignored: it asserted the
// opposite of what it had done, about the one action that cannot be taken back, and told the
// operator to repeat it.
//
// These read the source rather than driving a release, because what was wrong is whether the
// flag reaches the configuration the publisher is built from at all — a container is
// constructed once, from cfg, before any of this runs.

// runReleaseBumpBody returns just that function, so an assertion about it cannot be satisfied
// by matching text somewhere else in a 700-line file.
func runReleaseBumpBody(t *testing.T, source string) string {
	t.Helper()

	start := strings.Index(source, "func runReleaseBump(")
	if start < 0 {
		t.Fatal("runReleaseBump is gone; this test is out of date")
	}
	end := strings.Index(source[start:], "\n}\n")
	if end < 0 {
		t.Fatal("could not find the end of runReleaseBump")
	}
	return source[start : start+end]
}

func releaseSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean("release.go"))
	if err != nil {
		t.Fatalf("read release.go: %v", err)
	}
	return string(data)
}

func TestSkipPushReachesTheConfigThePublisherIsBuiltFrom(t *testing.T) {
	source := releaseSource(t)

	if !strings.Contains(source, "cfg.Versioning.GitPush = false") {
		t.Error("--skip-push never clears cfg.Versioning.GitPush, so it only changes what is " +
			"printed: the publisher is configured from that field at construction, and the " +
			"tag is pushed anyway while the summary says it was not")
	}

	fold := strings.Index(source, "cfg.Versioning.GitPush = false")
	container := strings.Index(source, "newContainerApp(ctx, cfg)")
	if fold < 0 || container < 0 || fold > container {
		t.Error("the flag is folded into the config after the container is built, so the " +
			"publisher was already constructed from the unmodified setting")
	}
}

func TestTheOneShotReleaseWritesTheConfiguredManifests(t *testing.T) {
	source := releaseSource(t)

	if !strings.Contains(source, "applyVersionFiles(ctx, app, bumpOutput.Version)") {
		t.Error("the release workflow never writes version_files, so a repository that " +
			"configures package.json is tagged with a version its manifest does not state — " +
			"the commit the tag names does not carry the version it is tagged as")
	}

	apply := strings.Index(source, "applyVersionFiles(ctx, app, bumpOutput.Version)")
	notes := strings.Index(source, "Generating release notes")
	if apply < 0 || notes < 0 || apply > notes {
		t.Error("the manifests are written after the notes step; they have to land before " +
			"publish makes the release commit, or the tag will not contain them")
	}
}

// The fix must clear the setting only when the flag is given, never unconditionally.
//
// This test first asserted that versioning.git_push defaults to true, on my assumption rather
// than the code: it defaults to *false*, and pushing is opt-in. So the risk the fix carries is
// not "pushing silently stops" — a project that never enabled it was never pushing — it is that
// the fold could clear the setting for a release that did not pass the flag.
func TestTheFoldIsGuardedByTheFlag(t *testing.T) {
	source := releaseSource(t)

	fold := strings.Index(source, "cfg.Versioning.GitPush = false")
	if fold < 0 {
		t.Fatal("the fold is gone; --skip-push cannot reach the publisher")
	}

	guard := strings.LastIndex(source[:fold], "if releaseSkipPush {")
	if guard < 0 {
		t.Error("cfg.Versioning.GitPush is cleared without checking the flag, which disables " +
			"pushing for every release rather than the ones that asked")
	}
	if between := source[guard:fold]; strings.Contains(between, "\n\t}") {
		t.Error("the guard closes before the assignment, so the clear is unconditional")
	}

	// And the setting a project opts into is untouched by default.
	if config.DefaultConfig().Versioning.GitPush {
		t.Error("versioning.git_push now defaults to true: pushing a tag is irreversible, so " +
			"it is opt-in, and a default flip would push from every repository that never " +
			"asked")
	}
}

// `relicta release --dry-run` died at step 2 of 5 with "release run not found" — the preview
// failing on its own earlier step. Step 1 deliberately does not persist a run when previewing,
// and steps 2 through 4 all went through use cases that load one.
//
// That made the flag useless on the command it matters most for: --dry-run is what an operator
// runs *before* doing something they cannot take back, so the safe path was the broken one.
func TestTheDryRunReachesTheEndOfTheWorkflow(t *testing.T) {
	source := releaseSource(t)

	// The bump step's state update is the first thing that needed a stored run.
	//
	// Scoped to that function's own body. This first searched the whole file before the
	// call site, which passed while the guard was absent because the plan step further up
	// also contains `if !dryRun {` — a revert-check caught it only because I asserted the
	// revert had actually changed the file before believing the result.
	body := runReleaseBumpBody(t, source)
	if !strings.Contains(body, "updateReleaseVersion(ctx, c, ver)") {
		t.Fatal("the bump step no longer updates release state; this test is out of date")
	}
	if !strings.Contains(body, "if !dryRun {") {
		t.Error("the bump step updates release state unconditionally, so a preview asks for " +
			"a run step 1 deliberately did not write and the workflow dies at step 2 of 5")
	}

	// Steps 3 and 4 are described rather than executed, matching what publish already does.
	if !strings.Contains(source, "Not generated in a dry run") {
		t.Error("the notes step still runs its use case under --dry-run, which loads the " +
			"stored release a preview does not create")
	}
	if !strings.Contains(source, "Not evaluated in a dry run") {
		t.Error("the approval step still runs under --dry-run for the same reason")
	}
}

// A preview that says what it did not do is honest; one that silently reports nothing invites
// the reader to believe the notes were checked.
func TestTheDryRunSaysWhatItDidNotDo(t *testing.T) {
	source := releaseSource(t)

	for _, want := range []string{
		"notes are written against the stored release",
		"relicta evaluate",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("the dry run does not explain %q, so an operator cannot tell a step that "+
				"was skipped from one that found nothing to report", want)
		}
	}
}
