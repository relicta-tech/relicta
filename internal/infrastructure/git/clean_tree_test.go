package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// workflow.require_clean_working_tree defaults to true and was read by no code at all, so
// every user had this gate switched on while relicta published from dirty trees regardless.
// Verified against the shipped binary: with a modified tracked file present it tagged v0.1.0
// and reported "Release completed successfully!".
//
// The distinction this file exists for is what counts as dirty. IsClean counts untracked
// files, and relicta's own .relicta/ directory is untracked in any repository that has not
// ignored it — so a gate built on IsClean would refuse every release, which is how a safety
// gate gets switched off rather than obeyed.
//
// The release's question is narrower: a tag points at a commit, so untracked files cannot be
// in it. Modified, staged and deleted tracked files can — they are exactly what the tag will
// not contain despite sitting on the author's disk.

func repoForStatus(t *testing.T) (dir string, run func(args ...string)) {
	t.Helper()

	dir = t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}

	run = func(args ...string) {
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

	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "chore: initial commit")

	return dir, run
}

func modifiedIn(t *testing.T, dir string) []string {
	t.Helper()

	svc, err := NewService(WithRepoPath(dir))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	modified, err := svc.ModifiedTrackedFiles(context.Background())
	if err != nil {
		t.Fatalf("ModifiedTrackedFiles: %v", err)
	}
	return modified
}

func TestACleanTreeReportsNothing(t *testing.T) {
	dir, _ := repoForStatus(t)

	if modified := modifiedIn(t, dir); len(modified) != 0 {
		t.Errorf("a freshly committed tree reported %v as modified", modified)
	}
}

func TestAModifiedTrackedFileIsReported(t *testing.T) {
	dir, _ := repoForStatus(t)

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	modified := modifiedIn(t, dir)
	if len(modified) != 1 || modified[0] != "README.md" {
		t.Errorf("modified = %v, want [README.md]: this is the change the tag would not "+
			"contain, which is the reason the gate exists", modified)
	}
}

// The case that decides whether the gate is usable at all.
func TestAnUntrackedFileIsNotReported(t *testing.T) {
	dir, _ := repoForStatus(t)

	// Exactly what relicta itself leaves behind.
	if err := os.MkdirAll(filepath.Join(dir, ".relicta", "releases"), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".relicta", "releases", "run.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if modified := modifiedIn(t, dir); len(modified) != 0 {
		t.Errorf("untracked files reported as modified: %v.\nrelicta creates .relicta/ in "+
			"every repository, so counting untracked files would refuse every release — and "+
			"a gate that always refuses gets switched off rather than obeyed", modified)
	}
}

// A staged but uncommitted file is still not in the commit the tag will point at.
func TestAStagedFileIsReported(t *testing.T) {
	dir, run := repoForStatus(t)

	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("y\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run("add", "new.txt")

	modified := modifiedIn(t, dir)
	if len(modified) != 1 || modified[0] != "new.txt" {
		t.Errorf("modified = %v, want [new.txt]: staged work is not in the commit either",
			modified)
	}
}

// A deleted tracked file is a change the release would not carry.
func TestADeletedTrackedFileIsReported(t *testing.T) {
	dir, _ := repoForStatus(t)

	if err := os.Remove(filepath.Join(dir, "README.md")); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if modified := modifiedIn(t, dir); len(modified) != 1 {
		t.Errorf("modified = %v, want the deleted file", modified)
	}
}
