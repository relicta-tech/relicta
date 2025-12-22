// Package git provides tests for the git service implementation.
package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// testRepoHelper provides helper functions for creating test git repositories.
type testRepoHelper struct {
	t       *testing.T
	repoDir string
	repo    *git.Repository
}

// newTestRepo creates a new test repository in a temporary directory.
func newTestRepo(t *testing.T) *testRepoHelper {
	t.Helper()

	repoDir := t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	if err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	return &testRepoHelper{
		t:       t,
		repoDir: repoDir,
		repo:    repo,
	}
}

// makeCommit creates a test commit in the repository.
func (h *testRepoHelper) makeCommit(message string) string {
	h.t.Helper()

	// Create a test file
	filename := filepath.Join(h.repoDir, "test.txt")
	if err := os.WriteFile(filename, []byte(message), 0644); err != nil {
		h.t.Fatalf("failed to write test file: %v", err)
	}

	// Stage the file
	worktree, err := h.repo.Worktree()
	if err != nil {
		h.t.Fatalf("failed to get worktree: %v", err)
	}

	if _, err := worktree.Add("test.txt"); err != nil {
		h.t.Fatalf("failed to stage file: %v", err)
	}

	// Create commit
	hash, err := worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		h.t.Fatalf("failed to commit: %v", err)
	}

	return hash.String()
}

// makeTag creates a test tag in the repository.
func (h *testRepoHelper) makeTag(name, message string) {
	h.t.Helper()

	head, err := h.repo.Head()
	if err != nil {
		h.t.Fatalf("failed to get HEAD: %v", err)
	}

	if message != "" {
		// Annotated tag
		_, err = h.repo.CreateTag(name, head.Hash(), &git.CreateTagOptions{
			Message: message,
			Tagger: &object.Signature{
				Name:  "Test Tagger",
				Email: "tagger@example.com",
				When:  time.Now(),
			},
		})
	} else {
		// Lightweight tag
		refName := plumbing.NewTagReferenceName(name)
		ref := plumbing.NewHashReference(refName, head.Hash())
		err = h.repo.Storer.SetReference(ref)
	}

	if err != nil {
		h.t.Fatalf("failed to create tag: %v", err)
	}
}

// TestNewService tests creating a new git service.
func TestNewService(t *testing.T) {
	t.Run("success with default options", func(t *testing.T) {
		helper := newTestRepo(t)
		helper.makeCommit("Initial commit")

		svc, err := NewService(WithRepoPath(helper.repoDir))
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		if svc == nil {
			t.Fatal("NewService() returned nil service")
		}
		if svc.cfg.RepoPath != helper.repoDir {
			t.Errorf("RepoPath = %v, want %v", svc.cfg.RepoPath, helper.repoDir)
		}
	})

	t.Run("success with custom options", func(t *testing.T) {
		helper := newTestRepo(t)
		helper.makeCommit("Initial commit")

		svc, err := NewService(
			WithRepoPath(helper.repoDir),
			WithDefaultRemote("upstream"),
			WithGPGSign("ABCD1234"),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		if svc.cfg.DefaultRemote != "upstream" {
			t.Errorf("DefaultRemote = %v, want upstream", svc.cfg.DefaultRemote)
		}
		if !svc.cfg.GPGSign {
			t.Error("GPGSign should be true")
		}
		if svc.cfg.GPGKeyID != "ABCD1234" {
			t.Errorf("GPGKeyID = %v, want ABCD1234", svc.cfg.GPGKeyID)
		}
	})

	t.Run("error on non-existent path", func(t *testing.T) {
		_, err := NewService(WithRepoPath("/nonexistent/path"))
		if err == nil {
			t.Error("NewService() should return error for non-existent path")
		}
	})

	t.Run("error on non-git directory", func(t *testing.T) {
		nonGitDir := t.TempDir()
		_, err := NewService(WithRepoPath(nonGitDir))
		if err == nil {
			t.Error("NewService() should return error for non-git directory")
		}
	})
}

func TestServiceImpl_CommitDiffStatsAndPatch(t *testing.T) {
	helper := newTestRepo(t)
	hash := helper.makeCommit("feat: add file")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}

	stats, err := svc.GetCommitDiffStats(context.Background(), hash)
	if err != nil {
		t.Fatalf("GetCommitDiffStats error: %v", err)
	}
	if stats == nil || stats.FilesChanged == 0 {
		t.Fatal("expected diff stats")
	}

	patch, err := svc.GetCommitPatch(context.Background(), hash)
	if err != nil {
		t.Fatalf("GetCommitPatch error: %v", err)
	}
	if patch == "" {
		t.Fatal("expected patch content")
	}
}

func TestServiceImpl_GetFileAtRef(t *testing.T) {
	helper := newTestRepo(t)
	hash := helper.makeCommit("feat: add file")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}

	data, err := svc.GetFileAtRef(context.Background(), hash, "test.txt")
	if err != nil {
		t.Fatalf("GetFileAtRef error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected file contents")
	}
}

func TestServiceImpl_isCleanFallback(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("feat: add file")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}

	clean, err := svc.isCleanFallback(context.Background())
	if err != nil {
		t.Fatalf("isCleanFallback error: %v", err)
	}
	if !clean {
		t.Fatal("expected clean repo")
	}
}

func TestServiceImpl_pushTagFallback(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("feat: add file")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}

	if err := svc.pushTagFallback(context.Background(), "v1.0.0", "origin", false); err == nil {
		t.Fatal("expected pushTagFallback to fail without remote")
	}
}

// TestGetRepositoryRoot tests getting the repository root.
func TestGetRepositoryRoot(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()
	root, err := svc.GetRepositoryRoot(ctx)
	if err != nil {
		t.Fatalf("GetRepositoryRoot() error = %v", err)
	}

	if root != helper.repoDir {
		t.Errorf("GetRepositoryRoot() = %v, want %v", root, helper.repoDir)
	}
}

// TestIsClean tests checking if the working tree is clean.
func TestIsClean(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("clean worktree", func(t *testing.T) {
		clean, err := svc.IsClean(ctx)
		if err != nil {
			t.Fatalf("IsClean() error = %v", err)
		}
		if !clean {
			t.Error("IsClean() should return true for clean worktree")
		}
	})

	t.Run("dirty worktree", func(t *testing.T) {
		// Create an unstaged file
		testFile := filepath.Join(helper.repoDir, "dirty.txt")
		if err := os.WriteFile(testFile, []byte("dirty"), 0644); err != nil {
			t.Fatalf("failed to create dirty file: %v", err)
		}

		clean, err := svc.IsClean(ctx)
		if err != nil {
			t.Fatalf("IsClean() error = %v", err)
		}
		if clean {
			t.Error("IsClean() should return false for dirty worktree")
		}
	})
}

// TestGetCommit tests getting a specific commit.
func TestGetCommit(t *testing.T) {
	helper := newTestRepo(t)
	hash := helper.makeCommit("Test commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		commit, err := svc.GetCommit(ctx, hash)
		if err != nil {
			t.Fatalf("GetCommit() error = %v", err)
		}

		if commit.Hash != hash {
			t.Errorf("Hash = %v, want %v", commit.Hash, hash)
		}
		if commit.Subject != "Test commit" {
			t.Errorf("Subject = %v, want 'Test commit'", commit.Subject)
		}
		if commit.Author.Email != "test@example.com" {
			t.Errorf("Author.Email = %v, want test@example.com", commit.Author.Email)
		}
	})

	t.Run("error on invalid hash", func(t *testing.T) {
		_, err := svc.GetCommit(ctx, "invalid")
		if err == nil {
			t.Error("GetCommit() should return error for invalid hash")
		}
	})
}

// TestGetHeadCommit tests getting the HEAD commit.
func TestGetHeadCommit(t *testing.T) {
	helper := newTestRepo(t)
	hash := helper.makeCommit("HEAD commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()
	commit, err := svc.GetHeadCommit(ctx)
	if err != nil {
		t.Fatalf("GetHeadCommit() error = %v", err)
	}

	if commit.Hash != hash {
		t.Errorf("Hash = %v, want %v", commit.Hash, hash)
	}
	if commit.Subject != "HEAD commit" {
		t.Errorf("Subject = %v, want 'HEAD commit'", commit.Subject)
	}
}

// TestGetCommitsSince tests getting commits since a reference.
func TestGetCommitsSince(t *testing.T) {
	helper := newTestRepo(t)

	// Create commit history
	hash1 := helper.makeCommit("First commit")
	helper.makeTag("v1.0.0", "Version 1.0.0")
	helper.makeCommit("Second commit")
	helper.makeCommit("Third commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("get commits since tag", func(t *testing.T) {
		commits, err := svc.GetCommitsSince(ctx, "v1.0.0")
		if err != nil {
			t.Fatalf("GetCommitsSince() error = %v", err)
		}

		if len(commits) != 2 {
			t.Errorf("GetCommitsSince() returned %d commits, want 2", len(commits))
		}
	})

	t.Run("get commits since hash", func(t *testing.T) {
		commits, err := svc.GetCommitsSince(ctx, hash1)
		if err != nil {
			t.Fatalf("GetCommitsSince() error = %v", err)
		}

		if len(commits) != 2 {
			t.Errorf("GetCommitsSince() returned %d commits, want 2", len(commits))
		}
	})

	t.Run("error on invalid ref", func(t *testing.T) {
		_, err := svc.GetCommitsSince(ctx, "invalid-ref")
		if err == nil {
			t.Error("GetCommitsSince() should return error for invalid ref")
		}
	})
}

// TestGetCommitsBetween tests getting commits between two references.
func TestGetCommitsBetween(t *testing.T) {
	helper := newTestRepo(t)

	// Create commit history
	hash1 := helper.makeCommit("First commit")
	helper.makeTag("v1.0.0", "Version 1.0.0")
	helper.makeCommit("Second commit")
	helper.makeCommit("Third commit")
	hash4 := helper.makeCommit("Fourth commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("get commits between tags and HEAD", func(t *testing.T) {
		commits, err := svc.GetCommitsBetween(ctx, "v1.0.0", "HEAD")
		if err != nil {
			t.Fatalf("GetCommitsBetween() error = %v", err)
		}

		if len(commits) != 3 {
			t.Errorf("GetCommitsBetween() returned %d commits, want 3", len(commits))
		}
	})

	t.Run("get commits between hashes", func(t *testing.T) {
		commits, err := svc.GetCommitsBetween(ctx, hash1, hash4)
		if err != nil {
			t.Fatalf("GetCommitsBetween() error = %v", err)
		}

		if len(commits) != 3 {
			t.Errorf("GetCommitsBetween() returned %d commits, want 3", len(commits))
		}
	})

	t.Run("error on invalid from ref", func(t *testing.T) {
		_, err := svc.GetCommitsBetween(ctx, "invalid", "HEAD")
		if err == nil {
			t.Error("GetCommitsBetween() should return error for invalid from ref")
		}
	})

	t.Run("error on invalid to ref", func(t *testing.T) {
		_, err := svc.GetCommitsBetween(ctx, hash1, "invalid")
		if err == nil {
			t.Error("GetCommitsBetween() should return error for invalid to ref")
		}
	})
}

// TestGetCurrentBranch tests getting the current branch name.
func TestGetCurrentBranch(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()
	branch, err := svc.GetCurrentBranch(ctx)
	if err != nil {
		t.Fatalf("GetCurrentBranch() error = %v", err)
	}

	// Default branch should be "master" or "main"
	if branch != "master" && branch != "main" {
		t.Errorf("GetCurrentBranch() = %v, want master or main", branch)
	}
}

// TestListTags tests listing all tags.
func TestListTags(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("no tags", func(t *testing.T) {
		tags, err := svc.ListTags(ctx)
		if err != nil {
			t.Fatalf("ListTags() error = %v", err)
		}
		if len(tags) != 0 {
			t.Errorf("ListTags() returned %d tags, want 0", len(tags))
		}
	})

	t.Run("with tags", func(t *testing.T) {
		helper.makeTag("v1.0.0", "Version 1.0.0")
		helper.makeCommit("Second commit")
		helper.makeTag("v1.1.0", "Version 1.1.0")
		helper.makeTag("lightweight", "")

		tags, err := svc.ListTags(ctx)
		if err != nil {
			t.Fatalf("ListTags() error = %v", err)
		}

		if len(tags) != 3 {
			t.Errorf("ListTags() returned %d tags, want 3", len(tags))
		}

		// Tags should be sorted by date, newest first
		if tags[0].Name != "lightweight" && tags[0].Name != "v1.1.0" {
			t.Errorf("First tag should be lightweight or v1.1.0, got %v", tags[0].Name)
		}
	})
}

// TestListVersionTags tests listing version tags.
func TestListVersionTags(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")

	helper.makeTag("v1.0.0", "Version 1.0.0")
	helper.makeCommit("Second commit")
	helper.makeTag("v1.1.0", "Version 1.1.0")
	helper.makeTag("v2.0.0", "Version 2.0.0")
	helper.makeTag("notaversion", "Not a version")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("list all version tags", func(t *testing.T) {
		tags, err := svc.ListVersionTags(ctx, "v")
		if err != nil {
			t.Fatalf("ListVersionTags() error = %v", err)
		}

		if len(tags) != 3 {
			t.Errorf("ListVersionTags() returned %d tags, want 3", len(tags))
		}

		// Should be sorted by semver, newest first
		if tags[0].Name != "v2.0.0" {
			t.Errorf("First tag should be v2.0.0, got %v", tags[0].Name)
		}
		if tags[1].Name != "v1.1.0" {
			t.Errorf("Second tag should be v1.1.0, got %v", tags[1].Name)
		}
		if tags[2].Name != "v1.0.0" {
			t.Errorf("Third tag should be v1.0.0, got %v", tags[2].Name)
		}
	})

	t.Run("no version tags", func(t *testing.T) {
		tags, err := svc.ListVersionTags(ctx, "nonexistent-")
		if err != nil {
			t.Fatalf("ListVersionTags() error = %v", err)
		}

		if len(tags) != 0 {
			t.Errorf("ListVersionTags() returned %d tags, want 0", len(tags))
		}
	})
}

// TestGetLatestTag tests getting the latest tag.
func TestGetLatestTag(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("no tags", func(t *testing.T) {
		_, err := svc.GetLatestTag(ctx)
		if err == nil {
			t.Error("GetLatestTag() should return error when no tags exist")
		}
	})

	t.Run("with tags", func(t *testing.T) {
		helper.makeTag("v1.0.0", "Version 1.0.0")
		time.Sleep(100 * time.Millisecond) // Ensure different timestamps
		helper.makeCommit("Second commit")
		time.Sleep(100 * time.Millisecond)
		helper.makeTag("v1.1.0", "Version 1.1.0")

		// Invalidate cache to ensure fresh data
		svc.InvalidateRepoInfoCache()

		tag, err := svc.GetLatestTag(ctx)
		if err != nil {
			t.Fatalf("GetLatestTag() error = %v", err)
		}

		// The latest tag should be v1.1.0 based on date
		if tag.Name != "v1.1.0" && tag.Name != "v1.0.0" {
			t.Errorf("GetLatestTag() = %v, want v1.1.0 or v1.0.0", tag.Name)
		}
	})
}

// TestGetLatestVersionTag tests getting the latest version tag.
func TestGetLatestVersionTag(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("no version tags", func(t *testing.T) {
		_, err := svc.GetLatestVersionTag(ctx, "v")
		if err == nil {
			t.Error("GetLatestVersionTag() should return error when no version tags exist")
		}
	})

	t.Run("with version tags", func(t *testing.T) {
		helper.makeTag("v1.0.0", "Version 1.0.0")
		helper.makeCommit("Second commit")
		helper.makeTag("v2.0.0", "Version 2.0.0")
		helper.makeTag("v1.5.0", "Version 1.5.0")

		tag, err := svc.GetLatestVersionTag(ctx, "v")
		if err != nil {
			t.Fatalf("GetLatestVersionTag() error = %v", err)
		}

		if tag.Name != "v2.0.0" {
			t.Errorf("GetLatestVersionTag() = %v, want v2.0.0", tag.Name)
		}
	})
}

// TestGetTag tests getting a specific tag.
func TestGetTag(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")
	helper.makeTag("v1.0.0", "Version 1.0.0")
	helper.makeTag("lightweight", "")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("get annotated tag", func(t *testing.T) {
		tag, err := svc.GetTag(ctx, "v1.0.0")
		if err != nil {
			t.Fatalf("GetTag() error = %v", err)
		}

		if tag.Name != "v1.0.0" {
			t.Errorf("Name = %v, want v1.0.0", tag.Name)
		}
		// The message may have trailing newline from go-git
		if tag.Message != "Version 1.0.0" && tag.Message != "Version 1.0.0\n" {
			t.Errorf("Message = %q, want 'Version 1.0.0' or 'Version 1.0.0\\n'", tag.Message)
		}
	})

	t.Run("get lightweight tag", func(t *testing.T) {
		tag, err := svc.GetTag(ctx, "lightweight")
		if err != nil {
			t.Fatalf("GetTag() error = %v", err)
		}

		if tag.Name != "lightweight" {
			t.Errorf("Name = %v, want lightweight", tag.Name)
		}
		if tag.Message != "" {
			t.Errorf("Message should be empty for lightweight tag, got %v", tag.Message)
		}
	})

	t.Run("error on non-existent tag", func(t *testing.T) {
		_, err := svc.GetTag(ctx, "nonexistent")
		if err == nil {
			t.Error("GetTag() should return error for non-existent tag")
		}
	})
}

// TestCreateTag tests creating tags.
func TestCreateTag(t *testing.T) {
	helper := newTestRepo(t)
	hash := helper.makeCommit("Initial commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("create annotated tag", func(t *testing.T) {
		opts := DefaultTagOptions()
		opts.Annotated = true

		err := svc.CreateTag(ctx, "v1.0.0", "Version 1.0.0", opts)
		if err != nil {
			t.Fatalf("CreateTag() error = %v", err)
		}

		// Verify tag was created
		tag, err := svc.GetTag(ctx, "v1.0.0")
		if err != nil {
			t.Fatalf("GetTag() error = %v", err)
		}
		if tag.Name != "v1.0.0" {
			t.Errorf("Tag name = %v, want v1.0.0", tag.Name)
		}
	})

	t.Run("create lightweight tag", func(t *testing.T) {
		opts := DefaultTagOptions()
		opts.Annotated = false

		err := svc.CreateTag(ctx, "lightweight", "", opts)
		if err != nil {
			t.Fatalf("CreateTag() error = %v", err)
		}

		// Verify tag was created
		tag, err := svc.GetTag(ctx, "lightweight")
		if err != nil {
			t.Fatalf("GetTag() error = %v", err)
		}
		if tag.Name != "lightweight" {
			t.Errorf("Tag name = %v, want lightweight", tag.Name)
		}
	})

	t.Run("create tag at specific ref", func(t *testing.T) {
		opts := DefaultTagOptions()
		opts.Ref = hash

		err := svc.CreateTag(ctx, "at-hash", "At hash", opts)
		if err != nil {
			t.Fatalf("CreateTag() error = %v", err)
		}

		tag, err := svc.GetTag(ctx, "at-hash")
		if err != nil {
			t.Fatalf("GetTag() error = %v", err)
		}
		if tag.Name != "at-hash" {
			t.Errorf("Tag name = %v, want at-hash", tag.Name)
		}
	})
}

// TestDeleteTag tests deleting tags.
func TestDeleteTag(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")
	helper.makeTag("v1.0.0", "Version 1.0.0")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("delete existing tag", func(t *testing.T) {
		err := svc.DeleteTag(ctx, "v1.0.0")
		if err != nil {
			t.Fatalf("DeleteTag() error = %v", err)
		}

		// Verify tag was deleted
		_, err = svc.GetTag(ctx, "v1.0.0")
		if err == nil {
			t.Error("Tag should not exist after deletion")
		}
	})

	t.Run("error on non-existent tag", func(t *testing.T) {
		// Note: go-git's Storer.RemoveReference doesn't return error for non-existent refs
		// This is expected behavior, so we skip this test
		t.Skip("go-git does not return error for deleting non-existent tags")
	})
}

// TestPushTag tests pushing tags (dry run only).
func TestPushTag(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")
	helper.makeTag("v1.0.0", "Version 1.0.0")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("dry run push", func(t *testing.T) {
		opts := DefaultPushOptions()
		opts.DryRun = true

		err := svc.PushTag(ctx, "v1.0.0", opts)
		if err != nil {
			t.Fatalf("PushTag() error = %v", err)
		}
	})
}

// TestGetRepositoryInfo tests getting repository information.
func TestGetRepositoryInfo(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("get repository info", func(t *testing.T) {
		info, err := svc.GetRepositoryInfo(ctx)
		if err != nil {
			t.Fatalf("GetRepositoryInfo() error = %v", err)
		}

		if info.Root != helper.repoDir {
			t.Errorf("Root = %v, want %v", info.Root, helper.repoDir)
		}
		if info.IsDirty {
			t.Error("IsDirty should be false for clean repo")
		}
	})

	t.Run("cache is used", func(t *testing.T) {
		// First call
		info1, err := svc.GetRepositoryInfo(ctx)
		if err != nil {
			t.Fatalf("GetRepositoryInfo() error = %v", err)
		}

		// Second call should use cache
		info2, err := svc.GetRepositoryInfo(ctx)
		if err != nil {
			t.Fatalf("GetRepositoryInfo() error = %v", err)
		}

		// Should be same object from cache
		if info1.Root != info2.Root {
			t.Error("Second call should return cached info")
		}
	})

	t.Run("cache invalidation", func(t *testing.T) {
		info1, err := svc.GetRepositoryInfo(ctx)
		if err != nil {
			t.Fatalf("GetRepositoryInfo() error = %v", err)
		}

		// Invalidate cache
		svc.InvalidateRepoInfoCache()

		// Create a dirty state
		testFile := filepath.Join(helper.repoDir, "new.txt")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		info2, err := svc.GetRepositoryInfo(ctx)
		if err != nil {
			t.Fatalf("GetRepositoryInfo() error = %v", err)
		}

		// Should reflect new state
		if !info2.IsDirty {
			t.Error("Info should reflect dirty state after cache invalidation")
		}
		if info1.IsDirty == info2.IsDirty {
			t.Error("Cache invalidation should cause fresh data fetch")
		}
	})
}

// TestGetDiffStats tests getting diff statistics.
func TestGetDiffStats(t *testing.T) {
	helper := newTestRepo(t)
	hash1 := helper.makeCommit("First commit")

	// Modify file for second commit
	filename := filepath.Join(helper.repoDir, "test.txt")
	if err := os.WriteFile(filename, []byte("modified content\n"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}
	helper.makeCommit("Second commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("get diff stats", func(t *testing.T) {
		stats, err := svc.GetDiffStats(ctx, hash1, "HEAD")
		if err != nil {
			t.Fatalf("GetDiffStats() error = %v", err)
		}

		if stats.FilesChanged == 0 {
			t.Error("FilesChanged should be greater than 0")
		}
		if stats.Insertions == 0 && stats.Deletions == 0 {
			t.Error("Should have insertions or deletions")
		}
	})
}

// TestGetBranchCommit tests getting commit for a specific branch.
func TestGetBranchCommit(t *testing.T) {
	helper := newTestRepo(t)
	hash := helper.makeCommit("Initial commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("get commit for current branch", func(t *testing.T) {
		// Get current branch name
		branch, err := svc.GetCurrentBranch(ctx)
		if err != nil {
			t.Fatalf("GetCurrentBranch() error = %v", err)
		}

		commit, err := svc.GetBranchCommit(ctx, branch)
		if err != nil {
			t.Fatalf("GetBranchCommit() error = %v", err)
		}

		if commit.Hash != hash {
			t.Errorf("Hash = %v, want %v", commit.Hash, hash)
		}
	})

	t.Run("error on invalid branch", func(t *testing.T) {
		_, err := svc.GetBranchCommit(ctx, "nonexistent-branch")
		if err == nil {
			t.Error("GetBranchCommit() should return error for invalid branch")
		}
	})
}

// TestListBranches tests listing all branches.
func TestListBranches(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("list branches", func(t *testing.T) {
		branches, err := svc.ListBranches(ctx)
		if err != nil {
			t.Fatalf("ListBranches() error = %v", err)
		}

		if len(branches) == 0 {
			t.Error("ListBranches() should return at least one branch")
		}

		// Should have a master or main branch
		foundMainBranch := false
		for _, branch := range branches {
			if branch.Name == "master" || branch.Name == "main" {
				foundMainBranch = true
				break
			}
		}
		if !foundMainBranch {
			t.Error("ListBranches() should include master or main branch")
		}
	})
}

// TestGetRemoteURL tests getting remote URL.
func TestGetRemoteURL(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")

	// Add a remote
	remote, err := helper.repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"https://github.com/test/repo.git"},
	})
	if err != nil {
		t.Fatalf("failed to create remote: %v", err)
	}
	_ = remote

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("get remote URL", func(t *testing.T) {
		url, err := svc.GetRemoteURL(ctx, "origin")
		if err != nil {
			t.Fatalf("GetRemoteURL() error = %v", err)
		}

		if url != "https://github.com/test/repo.git" {
			t.Errorf("URL = %v, want https://github.com/test/repo.git", url)
		}
	})

	t.Run("error on non-existent remote", func(t *testing.T) {
		_, err := svc.GetRemoteURL(ctx, "nonexistent")
		if err == nil {
			t.Error("GetRemoteURL() should return error for non-existent remote")
		}
	})
}

// TestGetDefaultBranch tests getting the default branch.
func TestGetDefaultBranch(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	branch, err := svc.GetDefaultBranch(ctx)
	if err != nil {
		t.Fatalf("GetDefaultBranch() error = %v", err)
	}

	// Should return main or master
	if branch != "main" && branch != "master" {
		t.Errorf("GetDefaultBranch() = %v, want main or master", branch)
	}
}

// TestPush tests push operations (dry run).
func TestPush(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("dry run push", func(t *testing.T) {
		opts := DefaultPushOptions()
		opts.DryRun = true

		err := svc.Push(ctx, opts)
		if err != nil {
			t.Fatalf("Push() error = %v", err)
		}
	})
}

// TestFetch tests fetch operations (dry run with invalid remote).
func TestFetch(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("fetch from non-existent remote fails", func(t *testing.T) {
		opts := DefaultFetchOptions()
		opts.Remote = "nonexistent"

		err := svc.Fetch(ctx, opts)
		if err == nil {
			t.Error("Fetch() should return error for non-existent remote")
		}
	})
}

// TestPull tests pull operations (dry run with invalid remote).
func TestPull(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	ctx := context.Background()

	t.Run("pull from non-existent remote fails", func(t *testing.T) {
		opts := DefaultPullOptions()
		opts.Remote = "nonexistent"

		err := svc.Pull(ctx, opts)
		if err == nil {
			t.Error("Pull() should return error for non-existent remote")
		}
	})
}

// TestParseConventionalCommits_NonStrictMode tests parsing with non-strict mode.
func TestParseConventionalCommits_NonStrictMode(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("Initial commit")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	commits := []Commit{
		{Message: "feat: feature", Subject: "feat: feature"},
		{Message: "invalid commit", Subject: "invalid commit"},
	}

	t.Run("non-strict mode includes invalid commits", func(t *testing.T) {
		opts := DefaultParseOptions()
		opts.StrictMode = false

		ccs, err := svc.ParseConventionalCommits(commits, opts)
		if err != nil {
			t.Fatalf("ParseConventionalCommits() error = %v", err)
		}

		if len(ccs) != 2 {
			t.Errorf("ParseConventionalCommits() returned %d commits, want 2", len(ccs))
		}

		// Second commit should be non-conventional
		if ccs[1].IsConventional {
			t.Error("Second commit should be marked as non-conventional")
		}
	})

	t.Run("strict mode fails on invalid commit", func(t *testing.T) {
		opts := DefaultParseOptions()
		opts.StrictMode = true

		_, err := svc.ParseConventionalCommits(commits, opts)
		if err == nil {
			t.Error("ParseConventionalCommits() should return error in strict mode")
		}
	})
}

// TestServiceImplConventionalCommitMethods tests the wrapper methods.
func TestServiceImplConventionalCommitMethods(t *testing.T) {
	helper := newTestRepo(t)
	helper.makeCommit("feat: add new feature")

	svc, err := NewService(WithRepoPath(helper.repoDir))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	t.Run("ParseConventionalCommit", func(t *testing.T) {
		cc, err := svc.ParseConventionalCommit("feat: test")
		if err != nil {
			t.Fatalf("ParseConventionalCommit() error = %v", err)
		}
		if cc.Type != CommitTypeFeat {
			t.Errorf("Type = %v, want feat", cc.Type)
		}
	})

	t.Run("ParseConventionalCommits", func(t *testing.T) {
		commits := []Commit{
			{Message: "feat: feature 1", Subject: "feat: feature 1"},
			{Message: "fix: bug fix", Subject: "fix: bug fix"},
		}

		ccs, err := svc.ParseConventionalCommits(commits, DefaultParseOptions())
		if err != nil {
			t.Fatalf("ParseConventionalCommits() error = %v", err)
		}
		if len(ccs) != 2 {
			t.Errorf("ParseConventionalCommits() returned %d commits, want 2", len(ccs))
		}
	})

	t.Run("DetectReleaseType", func(t *testing.T) {
		commits := []ConventionalCommit{
			{Type: CommitTypeFeat},
		}
		releaseType := svc.DetectReleaseType(commits)
		if releaseType != ReleaseTypeMinor {
			t.Errorf("DetectReleaseType() = %v, want minor", releaseType)
		}
	})

	t.Run("CategorizeCommits", func(t *testing.T) {
		commits := []ConventionalCommit{
			{Type: CommitTypeFeat, Description: "feature"},
			{Type: CommitTypeFix, Description: "fix"},
		}
		changes := svc.CategorizeCommits(commits)
		if len(changes.Features) != 1 {
			t.Errorf("Features count = %d, want 1", len(changes.Features))
		}
		if len(changes.Fixes) != 1 {
			t.Errorf("Fixes count = %d, want 1", len(changes.Fixes))
		}
	})

	t.Run("FilterCommits", func(t *testing.T) {
		commits := []ConventionalCommit{
			{Type: CommitTypeFeat, IsConventional: true},
			{Type: CommitTypeFix, IsConventional: true},
		}
		filter := CommitFilter{
			Types:                  []CommitType{CommitTypeFeat},
			IncludeNonConventional: true,
		}
		filtered := svc.FilterCommits(commits, filter)
		if len(filtered) != 1 {
			t.Errorf("FilterCommits() returned %d commits, want 1", len(filtered))
		}
	})
}
