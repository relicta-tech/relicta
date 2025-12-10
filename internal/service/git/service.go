// Package git provides git operations for ReleasePilot.
package git

import (
	"context"
)

// Service defines the interface for git operations.
type Service interface {
	// Repository information

	// GetRepositoryRoot returns the absolute path to the repository root.
	GetRepositoryRoot(ctx context.Context) (string, error)

	// GetRepositoryInfo returns information about the repository.
	GetRepositoryInfo(ctx context.Context) (*RepositoryInfo, error)

	// IsClean returns true if the working tree has no uncommitted changes.
	IsClean(ctx context.Context) (bool, error)

	// Commit operations

	// GetCommit returns a specific commit by hash.
	GetCommit(ctx context.Context, hash string) (*Commit, error)

	// GetCommitsSince returns all commits since the given reference.
	GetCommitsSince(ctx context.Context, ref string) ([]Commit, error)

	// GetCommitsBetween returns all commits between two references.
	GetCommitsBetween(ctx context.Context, from, to string) ([]Commit, error)

	// GetHeadCommit returns the current HEAD commit.
	GetHeadCommit(ctx context.Context) (*Commit, error)

	// GetBranchCommit returns the latest commit on a specific branch.
	GetBranchCommit(ctx context.Context, branch string) (*Commit, error)

	// Tag operations

	// GetLatestTag returns the most recent tag.
	GetLatestTag(ctx context.Context) (*Tag, error)

	// GetLatestVersionTag returns the most recent version tag matching the prefix.
	GetLatestVersionTag(ctx context.Context, prefix string) (*Tag, error)

	// ListTags returns all tags in the repository.
	ListTags(ctx context.Context) ([]Tag, error)

	// ListVersionTags returns all version tags matching the prefix.
	ListVersionTags(ctx context.Context, prefix string) ([]Tag, error)

	// GetTag returns a specific tag by name, or an error if not found.
	GetTag(ctx context.Context, name string) (*Tag, error)

	// CreateTag creates a new tag.
	CreateTag(ctx context.Context, name, message string, opts TagOptions) error

	// DeleteTag deletes a tag.
	DeleteTag(ctx context.Context, name string) error

	// PushTag pushes a tag to the remote.
	PushTag(ctx context.Context, name string, opts PushOptions) error

	// Branch operations

	// GetCurrentBranch returns the current branch name.
	GetCurrentBranch(ctx context.Context) (string, error)

	// GetDefaultBranch returns the default branch name (main/master).
	GetDefaultBranch(ctx context.Context) (string, error)

	// ListBranches returns all branches in the repository.
	ListBranches(ctx context.Context) ([]Branch, error)

	// Remote operations

	// GetRemoteURL returns the URL of the specified remote.
	GetRemoteURL(ctx context.Context, name string) (string, error)

	// Push pushes changes to the remote.
	Push(ctx context.Context, opts PushOptions) error

	// Pull pulls changes from the remote and merges them.
	Pull(ctx context.Context, opts PullOptions) error

	// Fetch fetches from the remote.
	Fetch(ctx context.Context, opts FetchOptions) error

	// Diff operations

	// GetDiffStats returns statistics about changes between two refs.
	GetDiffStats(ctx context.Context, from, to string) (*DiffStats, error)

	// Conventional commit operations

	// ParseConventionalCommit parses a commit message as a conventional commit.
	ParseConventionalCommit(message string) (*ConventionalCommit, error)

	// ParseConventionalCommits parses multiple commits as conventional commits.
	ParseConventionalCommits(commits []Commit, opts ParseOptions) ([]ConventionalCommit, error)

	// DetectReleaseType determines the release type based on conventional commits.
	DetectReleaseType(commits []ConventionalCommit) ReleaseType

	// CategorizeCommits groups commits by their type.
	CategorizeCommits(commits []ConventionalCommit) *CategorizedChanges

	// FilterCommits filters commits based on criteria.
	FilterCommits(commits []ConventionalCommit, filter CommitFilter) []ConventionalCommit
}

// TagOptions configures tag creation.
type TagOptions struct {
	// Annotated creates an annotated tag (vs lightweight).
	Annotated bool
	// Sign signs the tag with GPG.
	Sign bool
	// Force overwrites an existing tag.
	Force bool
	// Ref is the reference to tag (default: HEAD).
	Ref string
}

// DefaultTagOptions returns the default tag options.
func DefaultTagOptions() TagOptions {
	return TagOptions{
		Annotated: true,
		Sign:      false,
		Force:     false,
		Ref:       "HEAD",
	}
}

// PushOptions configures push operations.
type PushOptions struct {
	// Remote is the remote name (default: "origin").
	Remote string
	// Force enables force push.
	Force bool
	// Tags pushes tags.
	Tags bool
	// DryRun simulates the push.
	DryRun bool
	// RefSpec is the refspec to push.
	RefSpec string
}

// DefaultPushOptions returns the default push options.
func DefaultPushOptions() PushOptions {
	return PushOptions{
		Remote: "origin",
		Force:  false,
		Tags:   false,
		DryRun: false,
	}
}

// FetchOptions configures fetch operations.
type FetchOptions struct {
	// Remote is the remote name (default: "origin").
	Remote string
	// Tags fetches all tags.
	Tags bool
	// Prune removes remote-tracking references that no longer exist.
	Prune bool
	// Depth limits the fetch depth (0 = full).
	Depth int
}

// DefaultFetchOptions returns the default fetch options.
func DefaultFetchOptions() FetchOptions {
	return FetchOptions{
		Remote: "origin",
		Tags:   true,
		Prune:  false,
		Depth:  0,
	}
}

// PullOptions configures pull operations.
type PullOptions struct {
	// Remote is the remote name (default: "origin").
	Remote string
	// Branch is the branch to pull (default: current branch).
	Branch string
	// Rebase uses rebase instead of merge.
	Rebase bool
	// Depth limits the fetch depth (0 = full).
	Depth int
}

// DefaultPullOptions returns the default pull options.
func DefaultPullOptions() PullOptions {
	return PullOptions{
		Remote: "origin",
		Rebase: false,
		Depth:  0,
	}
}

// CommitOptions configures commit creation.
type CommitOptions struct {
	// Message is the commit message.
	Message string
	// Author is the commit author.
	Author *Author
	// AllowEmpty allows creating an empty commit.
	AllowEmpty bool
	// Sign signs the commit with GPG.
	Sign bool
	// Amend amends the previous commit.
	Amend bool
}

// ServiceConfig configures the git service.
type ServiceConfig struct {
	// RepoPath is the path to the repository.
	RepoPath string
	// DefaultRemote is the default remote name.
	DefaultRemote string
	// GPGSign enables GPG signing by default.
	GPGSign bool
	// GPGKeyID is the GPG key ID to use for signing.
	GPGKeyID string
}

// DefaultServiceConfig returns the default service configuration.
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		RepoPath:      ".",
		DefaultRemote: "origin",
		GPGSign:       false,
	}
}

// ServiceOption configures the git service.
type ServiceOption func(*ServiceConfig)

// WithRepoPath sets the repository path.
func WithRepoPath(path string) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.RepoPath = path
	}
}

// WithDefaultRemote sets the default remote.
func WithDefaultRemote(remote string) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.DefaultRemote = remote
	}
}

// WithGPGSign enables GPG signing.
func WithGPGSign(keyID string) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.GPGSign = true
		cfg.GPGKeyID = keyID
	}
}
