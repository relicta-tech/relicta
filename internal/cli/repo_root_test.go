package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Commands kept answering "which repository am I in" with "." or os.Getwd(), which is the
// working directory rather than the repository. It has been fixed one command at a time — the
// release store (#246, which reported "no release run found" while printing the correct
// root), the governance memory store, the container — and `relicta blast` was still doing it.
//
// That one mattered more than most, because blast radius feeds the risk score. Run from a
// subdirectory of a monorepo where a package had changed, it reported:
//
//	Total Packages:      0
//	Directly Affected:   0
//	Risk Level: LOW
//
// against the root's 2 packages and 1 affected. Not an error — a confident answer computed
// over the wrong file set.
//
// git works from anywhere in a tree, and so should this.

func repoWithSubdirectory(t *testing.T) (root, subdir string) {
	t.Helper()

	root = t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}

	subdir = filepath.Join(root, "packages", "api")
	if err := os.MkdirAll(subdir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(subdir, "package.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "chore: initial commit")

	return root, subdir
}

func TestTheRepositoryRootIsFoundFromASubdirectory(t *testing.T) {
	root, subdir := repoWithSubdirectory(t)

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	got := repositoryRoot(context.Background())
	if got != root {
		t.Errorf("repositoryRoot() = %q from %q, want the repository root %q.\n"+
			"A command using this to find packages, changed files or the release store "+
			"would operate on the subdirectory and report a confident answer about the "+
			"wrong tree.", got, subdir, root)
	}
	if got == subdir {
		t.Error("repositoryRoot() returned the working directory, which is the defect")
	}
}

// Outside a repository the answer has to be something a caller can use rather than an error:
// several commands run happily without one.
func TestTheRepositoryRootFallsBackOutsideARepository(t *testing.T) {
	dir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	if got := repositoryRoot(context.Background()); got != "." {
		t.Errorf("repositoryRoot() = %q outside a repository, want \".\"", got)
	}
}
