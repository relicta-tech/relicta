package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// workflow.pre_release_hook and workflow.post_release_hook were configured commands that never
// ran. Verified against the shipped binary before the fix: with both set to write a file, a
// full plan → bump → notes → approve → publish tagged v0.1.0, reported "Release completed
// successfully!", and neither file existed afterwards.
//
// A hook that silently does nothing is worse than an unimplemented one, because the operator
// who wrote `pre_release_hook: make test` believes their tests gate the release.

// hookThatWrites returns a shell command writing marker into a fresh file, and that file's path.
func hookThatWrites(t *testing.T, marker string) (command, path string) {
	t.Helper()

	path = filepath.Join(t.TempDir(), "hook-ran.txt")
	return "echo " + marker + " > " + path, path
}

func hookOutput(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path) // #nosec G304 -- test-controlled temp path
	if err != nil {
		t.Fatalf("the hook wrote nothing to %s: %v", path, err)
	}
	return strings.TrimSpace(string(data))
}

func TestAConfiguredPreReleaseHookActuallyRuns(t *testing.T) {
	c := withConfig(t)
	command, path := hookThatWrites(t, "pre-ran")
	c.Workflow.PreReleaseHook = command

	if err := runPreReleaseHook(context.Background(), "1.2.3", "v1.2.3"); err != nil {
		t.Fatalf("runPreReleaseHook: %v", err)
	}
	if got := hookOutput(t, path); got != "pre-ran" {
		t.Errorf("hook wrote %q, want \"pre-ran\": a configured command that leaves no trace "+
			"is the defect this fixes", got)
	}
}

// The whole point of a pre-release hook: it can veto. An operator's `make test` that fails and
// is then ignored is decoration, and worse than nothing because it reads like a safeguard.
func TestAFailingPreReleaseHookRefusesTheRelease(t *testing.T) {
	c := withConfig(t)
	c.Workflow.PreReleaseHook = "exit 3"

	err := runPreReleaseHook(context.Background(), "1.2.3", "v1.2.3")
	if err == nil {
		t.Fatal("a pre-release hook exited non-zero and the release went ahead: the hook is " +
			"where the test suite and the deploy-freeze check live, and ignoring their " +
			"verdict ships the release they refused")
	}
	if !strings.Contains(err.Error(), "pre-release hook failed") {
		t.Errorf("error is %q, which does not say that the hook is what refused: the "+
			"operator has to know whether to fix their command or their release", err)
	}
	if !strings.Contains(err.Error(), "Nothing has been tagged") {
		t.Errorf("error is %q and does not tell the operator the release left no trace, "+
			"which is what decides whether they need to clean up before retrying", err)
	}
}

// The release has already shipped by the time this runs — the tag exists, and with git_push on
// it is on the remote. An error here would make relicta's exit code say the release failed when
// it did not, and the standard reaction to that is a retry that publishes an existing version.
func TestAFailingPostReleaseHookDoesNotUnshipTheRelease(t *testing.T) {
	c := withConfig(t)
	c.Workflow.PostReleaseHook = "exit 7"

	// Compiles to nothing returnable on purpose: the signature itself is the guarantee. If
	// this ever grows an error return, the caller in publish.go must not propagate it.
	runPostReleaseHook(context.Background(), "1.2.3", "v1.2.3")
}

func TestAConfiguredPostReleaseHookActuallyRuns(t *testing.T) {
	c := withConfig(t)
	command, path := hookThatWrites(t, "post-ran")
	c.Workflow.PostReleaseHook = command

	runPostReleaseHook(context.Background(), "1.2.3", "v1.2.3")

	if got := hookOutput(t, path); got != "post-ran" {
		t.Errorf("hook wrote %q, want \"post-ran\"", got)
	}
}

// Hooks are shell, not an argv. `make test && ./scripts/verify.sh`, a pipe, a redirect — split
// on whitespace these run something the operator did not write, and the ones with quoted
// arguments run it silently wrong.
func TestAHookRunsAsAShellCommandRatherThanSplitArguments(t *testing.T) {
	c := withConfig(t)
	path := filepath.Join(t.TempDir(), "chained.txt")
	c.Workflow.PreReleaseHook = "echo first > " + path + " && echo second >> " + path

	if err := runPreReleaseHook(context.Background(), "1.2.3", "v1.2.3"); err != nil {
		t.Fatalf("runPreReleaseHook: %v", err)
	}
	if got := hookOutput(t, path); got != "first\nsecond" {
		t.Errorf("hook produced %q, want \"first\\nsecond\": && and > have to mean what they "+
			"mean in a shell, or every hook more complex than one word runs wrong", got)
	}
}

// Passed so the common hooks — notify a channel, trigger a deploy, upload an artifact — need no
// templating in the config string. Both, because the prefixed and unprefixed forms are
// different strings and deriving one from the other means every hook reimplements tag_prefix.
func TestAHookIsToldTheVersionAndTagItIsRunningFor(t *testing.T) {
	c := withConfig(t)
	path := filepath.Join(t.TempDir(), "env.txt")
	c.Workflow.PreReleaseHook = "echo \"$RELICTA_VERSION $RELICTA_TAG\" > " + path

	if err := runPreReleaseHook(context.Background(), "1.2.3", "v1.2.3"); err != nil {
		t.Fatalf("runPreReleaseHook: %v", err)
	}
	if got := hookOutput(t, path); got != "1.2.3 v1.2.3" {
		t.Errorf("hook saw %q, want \"1.2.3 v1.2.3\": without these a hook cannot name the "+
			"release it is announcing", got)
	}
}

// The overwhelmingly common configuration. Nothing configured must cost nothing and refuse
// nothing.
func TestNoHookConfiguredIsNotAFailure(t *testing.T) {
	withConfig(t)

	if err := runPreReleaseHook(context.Background(), "1.2.3", "v1.2.3"); err != nil {
		t.Errorf("an empty pre_release_hook returned %v, which would refuse every release in "+
			"every project that configures no hooks", err)
	}
	runPostReleaseHook(context.Background(), "1.2.3", "v1.2.3")
}
