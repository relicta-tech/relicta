package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewServiceOpensLinkedWorktreeWithWorktreeConfigExtension(t *testing.T) {
	root := t.TempDir()
	mainRepo := filepath.Join(root, "main")
	linkedRepo := filepath.Join(root, "linked")
	testGit(t, root, "init", "-b", "main", mainRepo)
	testGit(t, mainRepo, "config", "user.name", "Relicta Test")
	testGit(t, mainRepo, "config", "user.email", "relicta@example.test")
	if err := os.WriteFile(filepath.Join(mainRepo, "README.md"), []byte("first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	testGit(t, mainRepo, "add", "README.md")
	testGit(t, mainRepo, "commit", "-m", "feat: initial commit")
	testGit(t, mainRepo, "worktree", "add", "-b", "linked", linkedRepo)
	testGit(t, mainRepo, "config", "extensions.worktreeConfig", "true")
	testGit(t, linkedRepo, "config", "--worktree", "codex.localEnvironmentConfigPath", "worktree-local")
	if err := os.WriteFile(filepath.Join(linkedRepo, "change.txt"), []byte("second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	testGit(t, linkedRepo, "add", "change.txt")
	testGit(t, linkedRepo, "commit", "-m", "fix: linked worktree commit")

	svc, err := NewService(WithRepoPath(linkedRepo))
	if err != nil {
		t.Fatalf("NewService() failed: %v", err)
	}

	branch, err := svc.GetCurrentBranch(t.Context())
	if err != nil {
		t.Fatalf("GetCurrentBranch() failed: %v", err)
	}
	if branch != "linked" {
		t.Fatalf("GetCurrentBranch() = %q, want linked", branch)
	}
	commits, err := svc.GetCommitsBetween(t.Context(), "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("GetCommitsBetween() failed: %v", err)
	}
	if len(commits) != 1 || commits[0].Subject != "fix: linked worktree commit" {
		t.Fatalf("GetCommitsBetween() = %#v, want the linked worktree commit", commits)
	}

	storedConfig, err := svc.repo.Config()
	if err != nil {
		t.Fatalf("repo.Config() failed: %v", err)
	}
	if err := svc.repo.Storer.SetConfig(storedConfig); err != nil {
		t.Fatalf("repo.SetConfig() failed: %v", err)
	}
	if got := testGit(t, linkedRepo, "config", "--get", "extensions.worktreeConfig"); got != "true" {
		t.Fatalf("extensions.worktreeConfig after config write = %q, want true", got)
	}
}

func testGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
