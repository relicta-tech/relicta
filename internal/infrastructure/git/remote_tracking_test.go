package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// workflow.require_up_to_date defaults to false and was read by no code at all, so an operator
// who switched it on got no gate. Verified against the shipped binary before the fix: with a
// colleague's "fix: urgent fix from someone else" sitting unmerged on origin/main, publish
// tagged v0.1.0 and reported "Release completed successfully!". The tag named the tip of main
// and did not contain the fix.
//
// The distinction these tests exist for is between the answers "no" and "I do not know". A
// repository with no remote determinately cannot trail one; an unreachable remote determinately
// tells us nothing. Collapsing the second into the first is how a gate passes the release it
// was switched on to stop.

// remoteAndClone builds a bare repository and a clone of it, both configured so that git will
// commit without an ambient identity or a signing key.
func remoteAndClone(t *testing.T) (remote, work string, runIn func(dir string, args ...string)) {
	t.Helper()

	base := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(base); err == nil {
		base = resolved
	}

	runIn = func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
	}

	remote = filepath.Join(base, "remote.git")
	work = filepath.Join(base, "work")

	runIn(base, "init", "-q", "--bare", "-b", "main", remote)
	runIn(base, "clone", "-q", remote, work)

	commitIn(t, runIn, work, "README.md", "one\n", "chore: initial commit")
	runIn(work, "push", "-q", "-u", "origin", "main")

	return remote, work, runIn
}

func commitIn(t *testing.T, runIn func(string, ...string), dir, name, body, message string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runIn(dir, "add", "-A")
	runIn(dir, "commit", "-q", "-m", message)
}

// pushFromElsewhere puts a commit on the remote through a second clone, which is what a
// colleague merging while this checkout was not looking actually does. The checkout under test
// is left knowing nothing about it.
func pushFromElsewhere(t *testing.T, remote string, runIn func(string, ...string)) {
	t.Helper()

	other := filepath.Join(t.TempDir(), "other")
	runIn(filepath.Dir(other), "clone", "-q", remote, other)
	commitIn(t, runIn, other, "urgent.txt", "fix\n", "fix: urgent fix from someone else")
	runIn(other, "push", "-q")
}

func freshnessOf(t *testing.T, dir string) RemoteFreshness {
	t.Helper()

	svc, err := NewService(WithRepoPath(dir))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	fresh, err := svc.BehindRemote(context.Background(), nil)
	if err != nil {
		t.Fatalf("BehindRemote: %v", err)
	}
	return fresh
}

func TestABranchLevelWithItsRemoteIsNotBehind(t *testing.T) {
	_, work, _ := remoteAndClone(t)

	fresh := freshnessOf(t, work)
	if fresh.Behind != 0 {
		t.Errorf("a branch that matches its remote is reported %d behind, which would refuse "+
			"every release in an up-to-date repository", fresh.Behind)
	}
	if fresh.RemoteRef != "origin/main" {
		t.Errorf("compared against %q, want origin/main", fresh.RemoteRef)
	}
}

// The defect itself, and the reason the check has to reach the network: the checkout is behind
// and has never been told so. Nothing in this test runs `git fetch` by hand — if BehindRemote
// only read the last-known remote-tracking ref it would report 0 and pass the release.
func TestTheCheckFetchesRatherThanTrustingWhatWasLastFetched(t *testing.T) {
	remote, work, runIn := remoteAndClone(t)
	pushFromElsewhere(t, remote, runIn)

	fresh := freshnessOf(t, work)
	if fresh.Behind != 1 {
		t.Errorf("behind = %d, want 1.\nA colleague's commit is on origin/main and this "+
			"checkout has never fetched it. Reporting 0 means the gate compared against a "+
			"stale ref and would tag a release missing work that is already on the remote",
			fresh.Behind)
	}
}

// Ahead is not behind. Unpushed local work is normal on the commit that is about to be tagged,
// and counting it would refuse every release made before pushing.
func TestUnpushedWorkDoesNotCountAsBehind(t *testing.T) {
	_, work, runIn := remoteAndClone(t)
	commitIn(t, runIn, work, "local.txt", "mine\n", "feat: work not pushed yet")

	if fresh := freshnessOf(t, work); fresh.Behind != 0 {
		t.Errorf("behind = %d for a branch that is ahead of its remote, not behind it: "+
			"relicta commits the changelog itself one step before tagging, so this would "+
			"refuse every release", fresh.Behind)
	}
}

// A repository with no remote determinately cannot trail one. Refusing here would make
// require_up_to_date a permanent publish ban that no operator action could satisfy — and a gate
// that can never be satisfied gets switched off rather than obeyed.
func TestARepositoryWithNoRemoteHasNothingToTrail(t *testing.T) {
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}

	runIn := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runIn("init", "-q", "-b", "main", ".")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runIn("add", "-A")
	runIn("commit", "-q", "-m", "chore: initial commit")

	fresh := freshnessOf(t, dir)
	if fresh.HasRemote() {
		t.Errorf("a repository with no remote reported remote %q", fresh.Remote)
	}
	if fresh.Behind != 0 {
		t.Errorf("behind = %d with no remote configured", fresh.Behind)
	}
}

// Determinate for the same reason: a branch that has never been pushed has no counterpart to
// have fallen behind, so the gate has an answer rather than an unknown.
func TestABranchThatWasNeverPushedHasNothingToTrail(t *testing.T) {
	_, work, runIn := remoteAndClone(t)
	runIn(work, "checkout", "-q", "-b", "release/2.0")
	commitIn(t, runIn, work, "new.txt", "y\n", "feat: on an unpushed branch")

	fresh := freshnessOf(t, work)
	if !fresh.HasRemote() {
		t.Fatal("the repository has a remote and the check did not see it")
	}
	if fresh.IsPublished() {
		t.Errorf("compared against %q, but release/2.0 has never been pushed", fresh.RemoteRef)
	}
	if fresh.Behind != 0 {
		t.Errorf("behind = %d for a branch with no counterpart on the remote", fresh.Behind)
	}
}

// The case that separates this gate from a decorative one: the remote is configured and
// unreachable, so whether the branch is current is genuinely unknown. An error is the only
// honest return, because the caller turns an error into a refusal and would turn a zero count
// into a release.
func TestAnUnreachableRemoteIsAnErrorRatherThanAQuietPass(t *testing.T) {
	remote, work, _ := remoteAndClone(t)
	if err := os.RemoveAll(remote); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	svc, err := NewService(WithRepoPath(work))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	fresh, err := svc.BehindRemote(context.Background(), nil)
	if err == nil {
		t.Fatalf("an unreachable remote returned %+v and no error: the caller reads that as "+
			"up to date and publishes, which is exactly the release the gate exists to stop",
			fresh)
	}
}

// The announcement exists so the operator is told before a network wait they did not ask for,
// and it must not fire in a repository where no network call happens — a message about a fetch
// that never occurs teaches people to ignore the messages.
func TestTheFetchIsAnnouncedOnlyWhenThereIsSomethingToFetchFrom(t *testing.T) {
	_, work, _ := remoteAndClone(t)

	var announced []string
	svc, err := NewService(WithRepoPath(work))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if _, err := svc.BehindRemote(context.Background(), func(r string) {
		announced = append(announced, r)
	}); err != nil {
		t.Fatalf("BehindRemote: %v", err)
	}
	if len(announced) != 1 || announced[0] != "origin" {
		t.Errorf("announced = %v, want [origin]: the operator has to be told which remote is "+
			"being contacted before the wait", announced)
	}
}
