package multirepo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The coordinator was constructed with a nil GitAdapter and no implementation of the
// interface existed, so `relicta group plan` did not degrade or refuse — it panicked with a
// nil pointer dereference on the first repository in the group:
//
//	panic: runtime error: invalid memory address or nil pointer dereference
//	  multirepo.(*Coordinator).planRepo(...) coordinator.go:254
//
// Found by following this project's own configuration hint into a group command. The
// interface, the coordinator, the dependency ordering and the config schema were all written
// and tested; nothing could reach them.

// gitRepo builds a repository at dir and returns its path. Uses the git binary rather than
// go-git because the adapter's job is reading real repositories, and a fixture built by the
// same library it reads through can agree with it while both are wrong.
func gitRepo(t *testing.T, tag string, extraCommits int) string {
	t.Helper()

	dir := t.TempDir()
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
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "chore: initial commit")

	if tag != "" {
		run("tag", "-a", tag, "-m", tag)
	}

	for i := range extraCommits {
		name := filepath.Join(dir, "f"+string(rune('a'+i))+".txt")
		if err := os.WriteFile(name, []byte("y\n"), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		run("add", "-A")
		run("commit", "-q", "-m", "feat: change")
	}

	return dir
}

func TestAdapterReadsAReleasedRepository(t *testing.T) {
	repo := gitRepo(t, "v1.2.0", 3)
	adapter := NewGitAdapter("v")
	ctx := context.Background()

	version, err := adapter.GetCurrentVersion(ctx, repo)
	if err != nil {
		t.Fatalf("GetCurrentVersion: %v", err)
	}
	if version != "1.2.0" {
		t.Errorf("GetCurrentVersion = %q, want 1.2.0", version)
	}

	count, err := adapter.GetChangeCount(ctx, repo)
	if err != nil {
		t.Fatalf("GetChangeCount: %v", err)
	}
	if count != 3 {
		t.Errorf("GetChangeCount = %d, want 3 (the commits after the tag)", count)
	}

	changed, err := adapter.HasChanges(ctx, repo)
	if err != nil {
		t.Fatalf("HasChanges: %v", err)
	}
	if !changed {
		t.Error("HasChanges = false for a repository with three commits since its tag")
	}
}

// A repository with nothing since its tag is what makes the coordinator skip it. Reporting
// changes here would plan a release with no content.
func TestAdapterReportsNoChangesAtTheTag(t *testing.T) {
	repo := gitRepo(t, "v2.0.0", 0)
	adapter := NewGitAdapter("v")

	changed, err := adapter.HasChanges(context.Background(), repo)
	if err != nil {
		t.Fatalf("HasChanges: %v", err)
	}
	if changed {
		t.Error("HasChanges = true for a repository sitting exactly on its version tag")
	}
}

// A group can legitimately contain a member that has never shipped. Refusing the whole plan
// for it would make the group unusable until every member had a release.
func TestAnUnreleasedRepositoryIsNotAnError(t *testing.T) {
	repo := gitRepo(t, "", 2)
	adapter := NewGitAdapter("v")
	ctx := context.Background()

	version, err := adapter.GetCurrentVersion(ctx, repo)
	if err != nil {
		t.Fatalf("GetCurrentVersion on an unreleased repository: %v", err)
	}
	if version != "0.0.0" {
		t.Errorf("GetCurrentVersion = %q, want 0.0.0", version)
	}

	count, err := adapter.GetChangeCount(ctx, repo)
	if err != nil {
		t.Fatalf("GetChangeCount: %v", err)
	}
	if count == 0 {
		t.Error("GetChangeCount = 0 for a repository with commits and no tag: with no tag " +
			"to measure from, the whole history is the change set")
	}
}

// The configured prefix has to reach here too, or a group whose members tag "release-1.2.0"
// reads as never released — the same silent emptiness the single-repository path had.
func TestAdapterHonorsTheConfiguredTagPrefix(t *testing.T) {
	repo := gitRepo(t, "release-3.1.0", 1)
	ctx := context.Background()

	version, err := NewGitAdapter("release-").GetCurrentVersion(ctx, repo)
	if err != nil {
		t.Fatalf("GetCurrentVersion: %v", err)
	}
	if version != "3.1.0" {
		t.Errorf("GetCurrentVersion = %q, want 3.1.0", version)
	}

	// And with the wrong prefix it reads as unreleased, which is what makes the
	// configured value matter rather than being decoration.
	if v, err := NewGitAdapter("v").GetCurrentVersion(ctx, repo); err != nil {
		t.Fatalf("GetCurrentVersion: %v", err)
	} else if v != "0.0.0" {
		t.Errorf("with prefix \"v\" a release- tagged repository reported %q; the prefix is "+
			"not being applied", v)
	}
}

// An unreadable path must be an error rather than a panic or a plausible zero.
func TestAMissingRepositoryIsReportedNotGuessed(t *testing.T) {
	adapter := NewGitAdapter("v")
	missing := filepath.Join(t.TempDir(), "not-a-repository")

	if _, err := adapter.GetChangeCount(context.Background(), missing); err == nil {
		t.Error("GetChangeCount on a path that is not a repository returned no error; a " +
			"group member with a wrong path would silently plan as having no changes")
	}
}
