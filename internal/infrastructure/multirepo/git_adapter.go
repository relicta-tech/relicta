// Package multirepo provides infrastructure adapters for multi-repository governance.
package multirepo

import (
	"context"
	"fmt"
	"strings"

	appmultirepo "github.com/relicta-tech/relicta/v4/internal/application/multirepo"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
)

// GitAdapter answers the coordinator's questions about a repository it does not own.
//
// The coordinator was constructed with a nil GitAdapter — `NewCoordinator(nil, nil)` in the
// CLI — and no implementation of the interface existed anywhere. So `relicta group plan`
// did not degrade or refuse: it panicked with a nil pointer dereference on the first
// repository in the group, for as long as the command has existed. The interface, the
// coordinator, the dependency ordering and the config schema were all written and tested;
// nothing could reach them.
//
// Each call opens the repository at the given path, because a group's repositories are
// separate checkouts and the process's own git service is bound to the one relicta was
// invoked in.
type GitAdapter struct {
	// tagPrefix is the configured version tag prefix, so a group whose members tag
	// "release-1.2.0" is read the same way the single-repository path reads it.
	tagPrefix string
}

// NewGitAdapter returns an adapter that reads each repository at its configured path.
func NewGitAdapter(tagPrefix string) *GitAdapter {
	if tagPrefix == "" {
		tagPrefix = "v"
	}
	return &GitAdapter{tagPrefix: tagPrefix}
}

var _ appmultirepo.GitAdapter = (*GitAdapter)(nil)

// HasChanges reports whether the repository has commits since its last version tag.
func (a *GitAdapter) HasChanges(ctx context.Context, repoPath string) (bool, error) {
	count, err := a.GetChangeCount(ctx, repoPath)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetCurrentVersion returns the repository's latest version, or "0.0.0" when it has never
// been released.
//
// A repository with no version tag is not an error: a group can legitimately contain a
// member that has not shipped yet, and refusing the whole plan for it would make the group
// unusable until every member had a release.
func (a *GitAdapter) GetCurrentVersion(ctx context.Context, repoPath string) (string, error) {
	svc, err := git.NewService(git.WithRepoPath(repoPath))
	if err != nil {
		return "", fmt.Errorf("opening repository %s: %w", repoPath, err)
	}

	tag, err := svc.GetLatestVersionTag(ctx, a.tagPrefix)
	if err != nil || tag == nil {
		return "0.0.0", nil
	}
	return strings.TrimPrefix(tag.Name, a.tagPrefix), nil
}

// GetChangeCount returns the number of commits since the repository's last version tag, or
// its whole history when it has no version tag.
func (a *GitAdapter) GetChangeCount(ctx context.Context, repoPath string) (int, error) {
	svc, err := git.NewService(git.WithRepoPath(repoPath))
	if err != nil {
		return 0, fmt.Errorf("opening repository %s: %w", repoPath, err)
	}

	// An empty ref means "everything", which is what an unreleased repository needs.
	ref := ""
	if tag, tagErr := svc.GetLatestVersionTag(ctx, a.tagPrefix); tagErr == nil && tag != nil {
		ref = tag.Name
	}

	commits, err := svc.GetCommitsSince(ctx, ref)
	if err != nil {
		return 0, fmt.Errorf("counting commits in %s: %w", repoPath, err)
	}
	return len(commits), nil
}
