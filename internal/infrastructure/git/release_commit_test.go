package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The release commit exists because a tag has to point at a commit that contains the release:
// the changelog describing it and the version manifests naming it. Before this, publish wrote
// the changelog after creating the tag and committed neither, so the tag described nothing and
// `git show v0.1.0:package.json` still reported the previous version.

func repoForCommit(t *testing.T) (dir string, git func(args ...string)) {
	t.Helper()

	dir = t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}

	git = func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	git("init", "-q", "-b", "main")
	// The ambient user may sign every commit; a temporary repository has no key for it.
	git("config", "commit.gpgsign", "false")
	write(t, dir, "README.md", "x\n")
	git("add", "-A")
	git("commit", "-q", "-m", "chore: initial commit")

	return dir, git
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile %s: %v", name, err)
	}
}

func serviceAt(t *testing.T, dir string) *ServiceImpl {
	t.Helper()
	svc, err := NewService(WithRepoPath(dir))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

func filesIn(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "show", "--stat", "--name-only", "--format=", ref)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git show: %v", err)
	}
	return string(out)
}

// The first release's changelog has never been committed, so the commit has to cover an
// untracked file — which a commit pathspec does not match until it is staged.
func TestAnUntrackedChangelogIsCommitted(t *testing.T) {
	dir, _ := repoForCommit(t)
	write(t, dir, "CHANGELOG.md", "# Changelog\n\n## [0.1.0]\n- a bugfix\n")

	hash, err := serviceAt(t, dir).CommitPaths(context.Background(),
		[]string{"CHANGELOG.md"}, "chore(release): update changelog for 0.1.0")
	if err != nil {
		t.Fatalf("CommitPaths: %v", err)
	}
	if hash == "" {
		t.Fatal("nothing was committed, so the tag would point at a commit with no changelog")
	}

	if !strings.Contains(filesIn(t, dir, "HEAD"), "CHANGELOG.md") {
		t.Error("the commit does not contain CHANGELOG.md")
	}
}

// A release tool that sweeps whatever happened to be in the index into its own commit is worse
// than one that commits nothing: the operator's half-finished work ships under a release
// message they did not write.
func TestWorkTheOperatorStagedIsLeftAlone(t *testing.T) {
	dir, git := repoForCommit(t)
	write(t, dir, "CHANGELOG.md", "## [0.1.0]\n- a bugfix\n")
	write(t, dir, "wip.go", "package wip\n")
	git("add", "wip.go")

	if _, err := serviceAt(t, dir).CommitPaths(context.Background(),
		[]string{"CHANGELOG.md"}, "chore(release): update changelog for 0.1.0"); err != nil {
		t.Fatalf("CommitPaths: %v", err)
	}

	if committed := filesIn(t, dir, "HEAD"); strings.Contains(committed, "wip.go") {
		t.Errorf("the release commit swept in staged work:\n%s", committed)
	}
}

// Publish must not fail because there was nothing to write — a project may keep no changelog,
// and a retry after the commit already landed finds everything in place.
func TestNothingToCommitIsNotAFailure(t *testing.T) {
	dir, _ := repoForCommit(t)

	hash, err := serviceAt(t, dir).CommitPaths(context.Background(),
		[]string{"CHANGELOG.md"}, "chore(release): update changelog for 0.1.0")
	if err != nil {
		t.Fatalf("CommitPaths refused a release with nothing to commit: %v", err)
	}
	if hash != "" {
		t.Errorf("hash = %q, want empty: no commit should have been created", hash)
	}
}

func TestPendingPathsSeesModifiedAndUntrackedFilesOnly(t *testing.T) {
	dir, _ := repoForCommit(t)
	write(t, dir, "CHANGELOG.md", "## [0.1.0]\n")      // untracked
	write(t, dir, "README.md", "changed\n")            // modified, tracked
	write(t, dir, "unrelated.md", "not asked about\n") // untracked, not in the pathspec

	pending, err := serviceAt(t, dir).PendingPaths(context.Background(),
		[]string{"CHANGELOG.md", "README.md"})
	if err != nil {
		t.Fatalf("PendingPaths: %v", err)
	}

	got := strings.Join(pending, " ")
	if !strings.Contains(got, "CHANGELOG.md") || !strings.Contains(got, "README.md") {
		t.Errorf("pending = %v, want both the untracked changelog and the modified README",
			pending)
	}
	if strings.Contains(got, "unrelated.md") {
		t.Errorf("pending = %v includes a file outside the pathspec", pending)
	}
}
