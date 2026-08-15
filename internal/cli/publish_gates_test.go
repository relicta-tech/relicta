package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// `relicta release` reached PublishRelease.Execute without passing a single one of the gates
// `relicta publish` applies. Verified against the shipped binary in four throwaway repositories:
// a modified tracked file, a branch one commit behind origin/main, a `pre_release_hook: exit 3`
// and an actor budget requiring a cosigner each refused `relicta publish`, and each sailed
// through `relicta release --yes`, which tagged v0.1.0 and reported success.
//
// Two kinds of test here, because the defect had two halves. The behavior tests below check
// that the shared sequence refuses what it should and in the order it should; the wiring tests
// at the bottom check that both commands actually call it, which is the half no unit test of a
// working component can catch.

// repoWithUncommittedWork creates a repository holding one commit and one modified tracked
// file, and makes it the working directory — the gates read the repository they are run in.
func repoWithUncommittedWork(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "--initial-branch=main")
	git("config", "user.name", "Test User")
	git("config", "user.email", "test@example.com")
	git("config", "commit.gpgsign", "false")
	git("config", "tag.gpgsign", "false")

	tracked := filepath.Join(dir, "server.go")
	writeFile(t, tracked, "package main\n")
	git("add", "-A")
	git("commit", "-m", "chore: initial commit")
	writeFile(t, tracked, "package main\n\n// work the tag would not contain\n")

	t.Chdir(dir)
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// budgetDenyingPublish writes an actor budget that refuses to publish without a cosigner, and
// points the config at it.
func budgetDenyingPublish(t *testing.T) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "budgets.yaml")
	writeFile(t, path, "budgets:\n  - actor_id: \"*\"\n    requires_cosign: [publish]\n")
	cfg.Governance.ActorBudgetPath = path
}

// budgetPermittingPublish makes the actor a human, whose default budget allows a publish.
//
// Without it these tests inherit whatever environment they run in: CI sets CI=true and
// GITHUB_ACTIONS=true, the actor resolves as a bot, and the restrictive default budget refuses
// the publish before the gate under test is ever reached. That is a correct refusal for the
// wrong reason — it passed locally and failed in CI, which is the tell.
//
// A test about one gate has to hold the others still.
func budgetPermittingPublish(t *testing.T) {
	t.Helper()

	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "JENKINS_URL", "BUILDKITE", "CIRCLECI", "GITHUB_ACTOR"} {
		t.Setenv(key, "")
	}
	t.Setenv("USER", "alice")
}

func TestUncommittedWorkRefusesEveryPathToATag(t *testing.T) {
	withConfig(t)
	budgetPermittingPublish(t)
	repoWithUncommittedWork(t)

	err := enforcePrePublishGates(context.Background(), 0)
	if err == nil {
		t.Fatal("the gates passed a tree with a modified tracked file: the tag points at a " +
			"commit, so that file is precisely what the release does not contain, and the " +
			"author will believe it does")
	}
	if !strings.Contains(err.Error(), "require_clean_working_tree") {
		t.Errorf("error is %q and does not name the setting that refused, so the operator "+
			"cannot tell which gate to satisfy or switch off", err)
	}
}

// Order, not arrangement: an actor who may not publish should be told so before relicta reads
// their working tree or opens a connection to their remote on the way to refusing anyway.
func TestTheAutonomyBudgetIsAskedBeforeTheGatesThatReadDiskAndNetwork(t *testing.T) {
	withConfig(t)
	repoWithUncommittedWork(t)
	budgetDenyingPublish(t)

	err := enforcePrePublishGates(context.Background(), 0)
	if err == nil {
		t.Fatal("an actor the autonomy budget denies was allowed to publish")
	}
	if !strings.Contains(err.Error(), "autonomy budget") {
		t.Errorf("the first refusal was %q, but the budget denies this actor outright: the "+
			"budget has to answer before the gates that cost a disk read and a network round "+
			"trip, or relicta spends both to reach a verdict it already had", err)
	}
}

// The clean-tree gate is local and instant; the freshness gate fetches. Refusing a dirty tree
// after paying for a network round trip is the wrong way round, and on an unreachable remote it
// also reports the wrong problem.
func TestTheWorkingTreeIsCheckedBeforeTheBranchIsFetched(t *testing.T) {
	c := withConfig(t)
	budgetPermittingPublish(t)
	c.Workflow.RequireUpToDate = true
	dir := repoWithUncommittedWork(t)

	// A remote that cannot be reached, so the freshness gate is guaranteed to fail if it runs.
	cmd := exec.Command("git", "remote", "add", "origin", filepath.Join(dir, "no-such-remote.git"))
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}

	err := enforcePrePublishGates(context.Background(), 0)
	if err == nil {
		t.Fatal("the gates passed a dirty tree")
	}
	if !strings.Contains(err.Error(), "require_clean_working_tree") {
		t.Errorf("the first refusal was %q, but the tree is dirty: the local check has to come "+
			"first, or every refused release pays for a fetch and the operator is told about "+
			"their remote when the problem is on their disk", err)
	}
}

// The reason the pre-release hook sits where it does: a veto is only free while there is
// nothing to undo. The hook is where `make test` and the deploy-freeze check live, and its
// refusal has to land before relicta writes the changelog and commits it.
func TestAVetoingPreReleaseHookStopsTheReleaseBeforeTheChangelogIsWritten(t *testing.T) {
	c := withConfig(t)
	changelog := filepath.Join(t.TempDir(), "CHANGELOG.md")
	c.Changelog.File = changelog
	c.Workflow.PreReleaseHook = "exit 3"

	err := prepareReleaseForPublish(context.Background(), commandTestApp{gitRepo: stubGitRepo{}},
		"1.2.3", "v1.2.3")
	if err == nil {
		t.Fatal("a pre-release hook exited non-zero and the release carried on to commit and " +
			"tag: a veto that is logged and ignored is decoration")
	}
	if !strings.Contains(err.Error(), "pre-release hook failed") {
		t.Errorf("error is %q, which does not say the hook is what refused", err)
	}
	if _, statErr := os.Stat(changelog); !os.IsNotExist(statErr) {
		t.Error("the changelog was written despite the hook's veto: the hook must run while " +
			"there is still nothing for the operator to undo")
	}
}

// The wiring half. Reading the source rather than exercising it, for the reason
// mcp_wiring_test.go gives: exercising needs a repository, a container and a live release per
// case, while the failure that actually shipped was "nobody calls this".

// gateCallsThatMustNotBeInlined names the steps of the shared sequence and what re-inlining one
// costs. A path that calls a gate itself has started keeping its own copy of the order, which
// is how `relicta release` came to have a copy containing none of them.
var gateCallsThatMustNotBeInlined = map[string]string{
	"enforceActorBudget":      "the autonomy budget",
	"enforceCleanWorkingTree": "workflow.require_clean_working_tree",
	"enforceUpToDate":         "workflow.require_up_to_date",
	"runPreReleaseHook":       "workflow.pre_release_hook",
	"commitReleaseArtifacts":  "the release commit that puts the changelog inside the tag",
}

func TestTheOneShotReleaseGoesThroughTheSameSequenceAsPublish(t *testing.T) {
	source := readSource(t, "release.go")

	required := map[string]string{
		"enforcePrePublishGates(": "the autonomy budget, require_clean_working_tree and " +
			"require_up_to_date are all skipped, so `relicta release` tags from a dirty tree, " +
			"a stale branch, and for an actor the budget denies",
		"prepareReleaseForPublish(": "the pre-release hook never runs and the changelog is " +
			"never committed, so the tag points at a commit that describes none of the release",
		"runPostReleaseHook(": "workflow.post_release_hook is configured, documented, and " +
			"silently does nothing on the command CI runs",
	}
	for call, consequence := range required {
		if !strings.Contains(source, call) {
			t.Errorf("internal/cli/release.go never calls %s — %s", call, consequence)
		}
	}
}

func TestNeitherPublishingPathKeepsItsOwnCopyOfTheGates(t *testing.T) {
	for _, file := range []string{"publish.go", "release.go"} {
		source := readSource(t, file)
		for gate, what := range gateCallsThatMustNotBeInlined {
			if callsOutsideItsOwnDeclaration(source, gate) {
				t.Errorf("internal/cli/%s calls %s directly instead of going through the "+
					"shared sequence in publish_gates.go. Two copies of the order is how %s "+
					"came to be enforced on one publishing path and not the other", file, gate, what)
			}
		}
	}
}

// The gates must be answered before the dry-run return, or `relicta release --dry-run` reports
// success for a release the real command refuses — which is the one thing a dry run must never
// do. Nothing else in the workflow depends on their position, so the position needs guarding.
func TestTheGatesAreAnsweredBeforeTheDryRunReturns(t *testing.T) {
	source := readSource(t, "release.go")

	gates := strings.Index(source, "enforcePrePublishGates(")
	dryRunReturn := strings.Index(source, "Dry run - skipping actual publish")
	if gates < 0 || dryRunReturn < 0 {
		t.Fatalf("release.go no longer contains the gate call (%d) or the dry-run return (%d)",
			gates, dryRunReturn)
	}
	if gates > dryRunReturn {
		t.Error("the gates are applied after `relicta release --dry-run` has already returned, " +
			"so a dry run reports a release that require_clean_working_tree, " +
			"require_up_to_date or the autonomy budget would refuse")
	}
}

// The pre-release hook and the release commit must stay below it, for the opposite reason: a
// dry run promises to change nothing, and a hook is somebody else's code.
func TestTheHooksAndTheReleaseCommitStayBelowTheDryRunReturn(t *testing.T) {
	source := readSource(t, "release.go")

	dryRunReturn := strings.Index(source, "Dry run - skipping actual publish")
	prepare := strings.Index(source, "prepareReleaseForPublish(ctx")
	if dryRunReturn < 0 || prepare < 0 {
		t.Fatalf("release.go no longer contains the dry-run return (%d) or the call that runs "+
			"the hook and the release commit (%d)", dryRunReturn, prepare)
	}
	if prepare < dryRunReturn {
		t.Error("`relicta release --dry-run` would run the pre-release hook and make the " +
			"release commit, which is exactly what a dry run promises not to do")
	}
}

// callsOutsideItsOwnDeclaration reports whether source calls name somewhere other than in the
// func declaration that defines it — the declaration contains the same "name(" text.
func callsOutsideItsOwnDeclaration(source, name string) bool {
	return strings.Count(source, name+"(") > strings.Count(source, "func "+name+"(")
}
