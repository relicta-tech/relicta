// Package adapters provides infrastructure adapters for the release governance bounded context.
package adapters

import (
	"context"

	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
)

// GitRepoInspector adapts the sourcecontrol.GitRepository interface to ports.RepoInspector.
type GitRepoInspector struct {
	git sourcecontrol.GitRepository
}

// NewGitRepoInspector creates a new GitRepoInspector adapter.
func NewGitRepoInspector(git sourcecontrol.GitRepository) *GitRepoInspector {
	return &GitRepoInspector{git: git}
}

// HeadSHA returns the current HEAD commit SHA.
func (g *GitRepoInspector) HeadSHA(ctx context.Context) (domain.CommitSHA, error) {
	branch, err := g.git.GetCurrentBranch(ctx)
	if err != nil {
		return "", err
	}

	commit, err := g.git.GetLatestCommit(ctx, branch)
	if err != nil {
		return "", err
	}

	return domain.CommitSHA(commit.Hash().String()), nil
}

// IsClean returns true if the working tree has no uncommitted changes.
func (g *GitRepoInspector) IsClean(ctx context.Context) (bool, error) {
	isDirty, err := g.git.IsDirty(ctx)
	if err != nil {
		return false, err
	}
	return !isDirty, nil
}

// ResolveCommits returns the list of commit SHAs between baseRef and headSHA.
func (g *GitRepoInspector) ResolveCommits(ctx context.Context, baseRef string, headSHA domain.CommitSHA) ([]domain.CommitSHA, error) {
	commits, err := g.git.GetCommitsBetween(ctx, baseRef, string(headSHA))
	if err != nil {
		return nil, err
	}

	shas := make([]domain.CommitSHA, len(commits))
	for i, c := range commits {
		shas[i] = domain.CommitSHA(c.Hash().String())
	}
	return shas, nil
}

// GetRemoteURL returns the remote URL for the repository.
func (g *GitRepoInspector) GetRemoteURL(ctx context.Context) (string, error) {
	info, err := g.git.GetInfo(ctx)
	if err != nil {
		return "", err
	}
	return info.RemoteURL, nil
}

// GetCurrentBranch returns the current branch name.
func (g *GitRepoInspector) GetCurrentBranch(ctx context.Context) (string, error) {
	info, err := g.git.GetInfo(ctx)
	if err != nil {
		return "", err
	}
	return info.CurrentBranch, nil
}

// GetLatestVersionTag returns the latest version tag matching the prefix.
func (g *GitRepoInspector) GetLatestVersionTag(ctx context.Context, prefix string) (string, error) {
	tags, err := g.git.GetTags(ctx)
	if err != nil {
		return "", err
	}

	// Filter by prefix and find the latest version tag
	filtered := tags.FilterByPrefix(prefix).VersionTags()
	if len(filtered) == 0 {
		// No version tags found
		return "", nil
	}

	// Find latest by version comparison
	latest := filtered[0]
	for _, t := range filtered[1:] {
		latestVer := latest.Version()
		tVer := t.Version()
		if latestVer != nil && tVer != nil && tVer.GreaterThan(*latestVer) {
			latest = t
		}
	}

	return latest.Name(), nil
}

// TagExists checks if a tag already exists.
func (g *GitRepoInspector) TagExists(ctx context.Context, tagName string) (bool, error) {
	tags, err := g.git.GetTags(ctx)
	if err != nil {
		return false, err
	}

	for _, t := range tags {
		if t.Name() == tagName {
			return true, nil
		}
	}
	return false, nil
}

// ReleaseExists checks if a GitHub/GitLab release already exists for a tag.
// This is used for idempotency checks.
// Note: This is a placeholder - actual implementation would need to call GitHub/GitLab API.
func (g *GitRepoInspector) ReleaseExists(ctx context.Context, tagName string) (bool, error) {
	// For now, just check if the tag exists
	// A more complete implementation would query the GitHub/GitLab API
	return g.TagExists(ctx, tagName)
}

// Ensure GitRepoInspector implements ports.RepoInspector
var _ ports.RepoInspector = (*GitRepoInspector)(nil)
