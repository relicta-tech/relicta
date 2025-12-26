// Package ports defines the interfaces (ports) for the release governance bounded context.
package ports

import (
	"context"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
)

// RepoInspector provides read-only access to repository state.
type RepoInspector interface {
	// HeadSHA returns the current HEAD commit SHA.
	HeadSHA(ctx context.Context) (domain.CommitSHA, error)

	// IsClean returns true if the working tree has no uncommitted changes.
	IsClean(ctx context.Context) (bool, error)

	// ResolveCommits returns the list of commit SHAs between baseRef and headSHA.
	ResolveCommits(ctx context.Context, baseRef string, headSHA domain.CommitSHA) ([]domain.CommitSHA, error)

	// GetRemoteURL returns the remote URL for the repository.
	GetRemoteURL(ctx context.Context) (string, error)

	// GetCurrentBranch returns the current branch name.
	GetCurrentBranch(ctx context.Context) (string, error)

	// GetLatestVersionTag returns the latest version tag matching the prefix.
	GetLatestVersionTag(ctx context.Context, prefix string) (string, error)

	// TagExists checks if a tag already exists.
	TagExists(ctx context.Context, tagName string) (bool, error)

	// ReleaseExists checks if a GitHub/GitLab release already exists for a tag.
	// This is used for idempotency checks.
	ReleaseExists(ctx context.Context, tagName string) (bool, error)
}
