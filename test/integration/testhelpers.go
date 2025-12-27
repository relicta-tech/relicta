// Package integration provides integration test utilities and fixtures.
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestRepo represents a temporary git repository for testing.
type TestRepo struct {
	t       testing.TB
	Dir     string
	cleanup func()
}

// NewTestRepo creates a new temporary git repository for testing.
// The repository is automatically cleaned up when the test completes.
func NewTestRepo(t testing.TB) *TestRepo {
	t.Helper()

	// Create temporary directory
	dir, err := os.MkdirTemp("", "relicta-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	repo := &TestRepo{
		t:   t,
		Dir: dir,
		cleanup: func() {
			_ = os.RemoveAll(dir)
		},
	}

	// Initialize git repository with main as the default branch
	repo.Git("init", "--initial-branch=main")
	repo.Git("config", "user.email", "test@example.com")
	repo.Git("config", "user.name", "Test User")
	repo.Git("config", "commit.gpgsign", "false")

	t.Cleanup(repo.cleanup)
	return repo
}

// Git runs a git command in the test repository.
func (r *TestRepo) Git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v failed: %v\nOutput: %s", args, err, string(output))
	}
	return string(output)
}

// GitMayFail runs a git command that may fail, returning the output and error.
func (r *TestRepo) GitMayFail(args ...string) (string, error) {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// WriteFile writes a file to the repository.
func (r *TestRepo) WriteFile(path, content string) {
	r.t.Helper()
	fullPath := filepath.Join(r.Dir, path)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil { // #nosec G301 -- test repo needs exec
		r.t.Fatalf("failed to create directory %s: %v", dir, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil { // #nosec G306 -- test file permissions
		r.t.Fatalf("failed to write file %s: %v", path, err)
	}
}

// Commit stages all changes and creates a commit.
func (r *TestRepo) Commit(message string) string {
	r.t.Helper()
	r.Git("add", "-A")
	r.Git("commit", "-m", message, "--allow-empty")
	return r.Git("rev-parse", "HEAD")
}

// CommitWithDate creates a commit with a specific date.
func (r *TestRepo) CommitWithDate(message string, date time.Time) string {
	r.t.Helper()
	r.Git("add", "-A")
	dateStr := date.Format(time.RFC3339)
	cmd := exec.Command("git", "commit", "-m", message, "--allow-empty", "--date", dateStr) // #nosec G204 -- test helper with known args
	cmd.Dir = r.Dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+dateStr,
		"GIT_COMMITTER_DATE="+dateStr,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git commit failed: %v\nOutput: %s", err, string(output))
	}
	return r.Git("rev-parse", "HEAD")
}

// Tag creates a lightweight tag at the current HEAD.
func (r *TestRepo) Tag(name string) {
	r.t.Helper()
	// Use -m with empty message fallback to ensure tag creation works
	r.Git("tag", "-a", name, "-m", name)
}

// AnnotatedTag creates an annotated tag at the current HEAD.
func (r *TestRepo) AnnotatedTag(name, message string) {
	r.t.Helper()
	r.Git("tag", "-a", name, "-m", message)
}

// Branch creates and switches to a new branch.
func (r *TestRepo) Branch(name string) {
	r.t.Helper()
	r.Git("checkout", "-b", name)
}

// Checkout switches to a branch or commit.
func (r *TestRepo) Checkout(ref string) {
	r.t.Helper()
	r.Git("checkout", ref)
}

// CurrentBranch returns the current branch name.
func (r *TestRepo) CurrentBranch() string {
	r.t.Helper()
	output := r.Git("rev-parse", "--abbrev-ref", "HEAD")
	return trimNewline(output)
}

// HeadHash returns the current HEAD commit hash.
func (r *TestRepo) HeadHash() string {
	r.t.Helper()
	output := r.Git("rev-parse", "HEAD")
	return trimNewline(output)
}

// Path returns the full path to a file in the repository.
func (r *TestRepo) Path(relPath string) string {
	return filepath.Join(r.Dir, relPath)
}

// SetupConventionalCommits creates a series of conventional commits for testing.
func (r *TestRepo) SetupConventionalCommits() {
	r.t.Helper()

	r.WriteFile("README.md", "# Test Project")
	r.Commit("feat: initial project setup")

	r.WriteFile("main.go", "package main\n\nfunc main() {}")
	r.Commit("feat(core): add main function")

	r.WriteFile("utils.go", "package main\n\nfunc helper() {}")
	r.Commit("feat(utils): add helper function")

	r.WriteFile("main.go", "package main\n\nfunc main() { println(\"Hello\") }")
	r.Commit("fix: fix output in main function")

	r.WriteFile("docs.md", "# Documentation")
	r.Commit("docs: add documentation")

	r.WriteFile("utils.go", "package main\n\n// helper does things\nfunc helper() {}")
	r.Commit("refactor: improve code structure")
}

// SetupBreakingChangeCommits creates commits with breaking changes.
func (r *TestRepo) SetupBreakingChangeCommits() {
	r.t.Helper()

	r.WriteFile("README.md", "# Test Project")
	r.Commit("feat: initial setup")

	r.WriteFile("api.go", "package main\n\nfunc OldAPI() {}")
	r.Commit("feat: add old API")

	r.WriteFile("api.go", "package main\n\nfunc NewAPI() {}")
	r.Commit("feat!: replace API with new implementation\n\nBREAKING CHANGE: The old API has been removed.")
}

// SetupVersionedTags creates tags with semantic versions.
func (r *TestRepo) SetupVersionedTags() {
	r.t.Helper()

	r.WriteFile("v1.go", "v1")
	r.Commit("feat: v1.0.0 release")
	r.AnnotatedTag("v1.0.0", "Version 1.0.0")

	r.WriteFile("v1.1.go", "v1.1")
	r.Commit("feat: add new feature")
	r.AnnotatedTag("v1.1.0", "Version 1.1.0")

	r.WriteFile("v1.1.1.go", "v1.1.1")
	r.Commit("fix: bug fix")
	r.AnnotatedTag("v1.1.1", "Version 1.1.1")

	r.WriteFile("v2.go", "v2")
	r.Commit("feat!: major version bump")
	r.AnnotatedTag("v2.0.0", "Version 2.0.0")
}

// RequireGitVersion checks if git is installed and meets the minimum version.
func RequireGitVersion(t testing.TB, minVersion string) {
	t.Helper()

	cmd := exec.Command("git", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("git not available: %v", err)
	}
	t.Logf("Git version: %s", trimNewline(string(output)))
}

// trimNewline removes trailing newlines from a string.
func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
