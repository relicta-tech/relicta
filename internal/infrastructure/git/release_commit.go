package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// PendingPaths returns which of the given repo-relative paths differ from HEAD,
// counting untracked files: a changelog that has never been committed is exactly
// the case a release has to commit.
//
// Used to decide whether there is anything to commit at all. `git commit` with
// nothing staged exits non-zero, and a release must not fail because the
// changelog happened to be unchanged.
func (s *ServiceImpl) PendingPaths(ctx context.Context, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	repoRoot, err := s.GetRepositoryRoot(ctx)
	if err != nil {
		return nil, err
	}

	args := append([]string{"status", "--porcelain", "--"}, paths...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoRoot
	cmd.Env = filteredGitEnv()
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}

	pending := make([]string, 0, len(paths))
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		// Two status columns, then the path — the same shape parsed in clean_tree.go.
		if len(line) > 2 {
			pending = append(pending, strings.TrimSpace(line[2:]))
		}
	}
	return pending, nil
}

// CommitPaths commits exactly the given repo-relative paths and returns the new
// commit hash. It returns an empty hash and no error when none of them differ
// from HEAD.
//
// Two properties this has to hold, both of which shape the implementation:
//
// Only these paths. The pathspec on `git commit` means work the user had staged
// for their own reasons stays staged rather than being swept into a release
// commit they did not write. A release tool that quietly commits whatever
// happened to be in the index is worse than one that commits nothing.
//
// The user's identity. This shells out rather than using go-git because a commit
// is not just a tree write: user.name and user.email may come from an includeIf
// section, commit.gpgsign may require a signature, and pre-commit hooks may have
// something to say. go-git implements none of that and would need an author
// synthesized here — producing a release commit attributed differently from
// every other commit in the repository, permanently, in a way nobody would think
// to check. git's own resolution is the only correct one.
func (s *ServiceImpl) CommitPaths(ctx context.Context, paths []string, message string) (string, error) {
	pending, err := s.PendingPaths(ctx, paths)
	if err != nil {
		return "", err
	}
	if len(pending) == 0 {
		return "", nil
	}

	repoRoot, err := s.GetRepositoryRoot(ctx)
	if err != nil {
		return "", err
	}

	run := func(args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = repoRoot
		cmd.Env = filteredGitEnv()
		out, runErr := cmd.CombinedOutput()
		if runErr != nil {
			return "", fmt.Errorf("git %s failed: %w: %s",
				args[0], runErr, strings.TrimSpace(string(out)))
		}
		return strings.TrimSpace(string(out)), nil
	}

	// Staged first because a path that is untracked — the usual state of a
	// changelog on a first release — does not match a commit pathspec until it
	// is in the index.
	if _, err := run(append([]string{"add", "--"}, pending...)...); err != nil {
		return "", err
	}
	if _, err := run(append([]string{"commit", "-m", message, "--"}, pending...)...); err != nil {
		return "", err
	}

	hash, err := run("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return hash, nil
}
