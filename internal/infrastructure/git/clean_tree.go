package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
)

// ModifiedTrackedFiles returns the tracked files with uncommitted changes.
//
// Deliberately not IsClean. That one counts untracked files, and relicta's own .relicta/
// directory is untracked in any repository that has not gitignored it — so a release gate
// built on IsClean would refuse every release, which is how a safety gate gets switched off
// rather than obeyed.
//
// The question a release asks is narrower: is anything that will be in the tag still
// uncommitted? A tag points at a commit, so untracked files cannot be in it. Modified, staged
// and deleted tracked files can — they are what the tag will not contain despite being on the
// author's disk.
func (s *ServiceImpl) ModifiedTrackedFiles(ctx context.Context) ([]string, error) {
	status, err := s.worktree.Status()
	if err != nil {
		// Same fallback as IsClean: go-git cannot read some newer index formats.
		return s.modifiedTrackedFilesFallback(ctx)
	}

	modified := make([]string, 0, len(status))
	for path, entry := range status {
		if entry == nil {
			continue
		}
		if entry.Staging == git.Untracked && entry.Worktree == git.Untracked {
			continue
		}
		modified = append(modified, path)
	}
	return modified, nil
}

// modifiedTrackedFilesFallback shells out when go-git cannot read the index.
func (s *ServiceImpl) modifiedTrackedFilesFallback(ctx context.Context) ([]string, error) {
	repoRoot, err := s.GetRepositoryRoot(ctx)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = repoRoot
	cmd.Env = filteredGitEnv()
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}

	modified := make([]string, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" || strings.HasPrefix(line, "??") {
			// "??" is untracked, excluded for the reason above.
			continue
		}
		// Porcelain is exactly two status columns followed by the path. Slicing at 3
		// ate the path's first character — "README.md" arrived as "EADME.md" — because
		// the separator is one space only when both columns are occupied.
		if len(line) > 2 {
			modified = append(modified, strings.TrimSpace(line[2:]))
		}
	}
	return modified, nil
}
