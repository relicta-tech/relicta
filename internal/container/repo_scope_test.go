package container

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// Every path a container resolves derives from its git service, and that service took no path
// — so it opened the process working directory. Pointing release services at another
// repository's root while the git adapter still pointed at the invoking one would have
// published the invoking repository's tags.
//
// That is the failure this test exists for, and it is why `relicta group release` refused to
// publish anything until the container could be scoped: silent misrouting on the publish path
// is unrecoverable, and the operator would find out from `git tag` in the wrong repository.

func gitRepoAt(t *testing.T, name string) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
	}

	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "chore: initial commit")

	return dir
}

// A container built for one repository must resolve that repository, from anywhere.
func TestAContainerBuiltForARepositoryDoesNotFollowTheWorkingDirectory(t *testing.T) {
	caller := gitRepoAt(t, "caller")
	member := gitRepoAt(t, "member")

	// Run from the caller, the way a group release does.
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(caller); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	app, err := NewForRepo(config.DefaultConfig(), member)
	if err != nil {
		t.Fatalf("NewForRepo: %v", err)
	}
	ctx := context.Background()
	if err := app.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })

	info, err := app.GitAdapter().GetInfo(ctx)
	if err != nil {
		t.Fatalf("GetInfo: %v", err)
	}

	if info.Path != member {
		t.Errorf("the container resolved %q, want the member %q", info.Path, member)
	}
	if info.Path == caller {
		t.Error("the container resolved the INVOKING repository: a release driven through it " +
			"would tag the caller instead of the member, which is the failure this scoping " +
			"exists to prevent")
	}
}

// The ordinary case must be untouched: with no repository given, a container follows the
// working directory exactly as before.
func TestAContainerWithNoRepositoryStillFollowsTheWorkingDirectory(t *testing.T) {
	repo := gitRepoAt(t, "cwd")

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	app, err := New(config.DefaultConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	if err := app.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })

	info, err := app.GitAdapter().GetInfo(ctx)
	if err != nil {
		t.Fatalf("GetInfo: %v", err)
	}
	if info.Path != repo {
		t.Errorf("a container with no repository resolved %q, want the working directory %q",
			info.Path, repo)
	}
}

// A group release publishes what was already approved and approves nothing itself. Without
// that, adding a repository to a group would be a way around its own policy — the release
// would run under an approval nobody gave.
func TestTheGroupExecutorRefusesAMemberThatIsNotApproved(t *testing.T) {
	member := gitRepoAt(t, "unapproved")

	executor := NewGroupExecutor("v", nil)
	_, err := executor.Release(context.Background(), member)
	if err == nil {
		t.Fatal("Release published a repository with no approved run")
	}
	if !strings.Contains(err.Error(), "plan") && !strings.Contains(err.Error(), "approve") {
		t.Errorf("refusal %q says neither that there is no plan nor that approval is needed", err)
	}
}
