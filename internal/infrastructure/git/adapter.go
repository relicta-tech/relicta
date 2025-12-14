// Package git provides infrastructure adapters for git operations.
package git

import (
	"context"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
)

// Default timeouts for git operations to prevent hangs on slow/unreachable remotes.
const (
	// DefaultLocalTimeout is the timeout for local git operations (read-only).
	DefaultLocalTimeout = 30 * time.Second

	// DefaultRemoteTimeout is the timeout for remote git operations (network calls).
	DefaultRemoteTimeout = 60 * time.Second
)

// withLocalTimeout applies a timeout for local git operations.
func withLocalTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	// Don't override if context already has a shorter deadline
	if deadline, ok := ctx.Deadline(); ok {
		if time.Until(deadline) < DefaultLocalTimeout {
			return ctx, func() {}
		}
	}
	return context.WithTimeout(ctx, DefaultLocalTimeout)
}

// withRemoteTimeout applies a timeout for remote git operations.
func withRemoteTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	// Don't override if context already has a shorter deadline
	if deadline, ok := ctx.Deadline(); ok {
		if time.Until(deadline) < DefaultRemoteTimeout {
			return ctx, func() {}
		}
	}
	return context.WithTimeout(ctx, DefaultRemoteTimeout)
}

// Adapter adapts the existing git service to the domain interface.
type Adapter struct {
	svc Service
}

// NewAdapter creates a new git adapter.
func NewAdapter(svc Service) *Adapter {
	return &Adapter{svc: svc}
}

// GetInfo retrieves repository information.
func (a *Adapter) GetInfo(ctx context.Context) (*sourcecontrol.RepositoryInfo, error) {
	info, err := a.svc.GetRepositoryInfo(ctx)
	if err != nil {
		return nil, err
	}

	result := &sourcecontrol.RepositoryInfo{
		Path:          info.Root,
		Name:          extractRepoName(info.Root),
		CurrentBranch: info.CurrentBranch,
		DefaultBranch: info.DefaultBranch,
		IsDirty:       info.IsDirty,
	}

	if len(info.Remotes) > 0 {
		result.RemoteURL = info.Remotes[0].URL
		result.Owner = extractOwner(info.Remotes[0].URL)
	}

	return result, nil
}

// GetRemotes retrieves remote information.
func (a *Adapter) GetRemotes(ctx context.Context) ([]sourcecontrol.RemoteInfo, error) {
	info, err := a.svc.GetRepositoryInfo(ctx)
	if err != nil {
		return nil, err
	}

	remotes := make([]sourcecontrol.RemoteInfo, len(info.Remotes))
	for i, r := range info.Remotes {
		remotes[i] = sourcecontrol.RemoteInfo{
			Name: r.Name,
			URL:  r.URL,
		}
	}
	return remotes, nil
}

// GetBranches retrieves branch information.
func (a *Adapter) GetBranches(ctx context.Context) ([]sourcecontrol.BranchInfo, error) {
	branches, err := a.svc.ListBranches(ctx)
	if err != nil {
		return nil, err
	}

	currentBranch, _ := a.svc.GetCurrentBranch(ctx)

	result := make([]sourcecontrol.BranchInfo, len(branches))
	for i, b := range branches {
		result[i] = sourcecontrol.BranchInfo{
			Name:      b.Name,
			IsRemote:  b.IsRemote,
			IsCurrent: b.Name == currentBranch,
			Hash:      sourcecontrol.CommitHash(b.Hash),
		}
	}
	return result, nil
}

// GetCurrentBranch retrieves the current branch name.
func (a *Adapter) GetCurrentBranch(ctx context.Context) (string, error) {
	return a.svc.GetCurrentBranch(ctx)
}

// GetCommit retrieves a specific commit.
func (a *Adapter) GetCommit(ctx context.Context, hash sourcecontrol.CommitHash) (*sourcecontrol.Commit, error) {
	commit, err := a.svc.GetCommit(ctx, string(hash))
	if err != nil {
		return nil, err
	}
	return convertCommit(commit), nil
}

// GetCommitsBetween retrieves commits between two references.
func (a *Adapter) GetCommitsBetween(ctx context.Context, from, to string) ([]*sourcecontrol.Commit, error) {
	ctx, cancel := withLocalTimeout(ctx)
	defer cancel()

	commits, err := a.svc.GetCommitsBetween(ctx, from, to)
	if err != nil {
		return nil, err
	}
	return convertCommits(commits), nil
}

// GetCommitsSince retrieves commits since a reference.
func (a *Adapter) GetCommitsSince(ctx context.Context, ref string) ([]*sourcecontrol.Commit, error) {
	ctx, cancel := withLocalTimeout(ctx)
	defer cancel()

	commits, err := a.svc.GetCommitsSince(ctx, ref)
	if err != nil {
		return nil, err
	}
	return convertCommits(commits), nil
}

// GetLatestCommit retrieves the latest commit on a branch.
func (a *Adapter) GetLatestCommit(ctx context.Context, branch string) (*sourcecontrol.Commit, error) {
	var commit *Commit
	var err error

	// If branch is empty or "HEAD", get the HEAD commit
	if branch == "" || branch == "HEAD" {
		commit, err = a.svc.GetHeadCommit(ctx)
	} else {
		commit, err = a.svc.GetBranchCommit(ctx, branch)
	}

	if err != nil {
		return nil, err
	}
	return convertCommit(commit), nil
}

// GetTags retrieves all tags.
func (a *Adapter) GetTags(ctx context.Context) (sourcecontrol.TagList, error) {
	tags, err := a.svc.ListTags(ctx)
	if err != nil {
		return nil, err
	}

	result := make(sourcecontrol.TagList, len(tags))
	for i, t := range tags {
		result[i] = sourcecontrol.NewTag(
			t.Name,
			sourcecontrol.CommitHash(t.Hash),
		)
	}
	return result, nil
}

// GetTag retrieves a specific tag.
func (a *Adapter) GetTag(ctx context.Context, name string) (*sourcecontrol.Tag, error) {
	tag, err := a.svc.GetTag(ctx, name)
	if err != nil {
		return nil, err
	}
	if tag == nil {
		return nil, sourcecontrol.ErrTagNotFound
	}
	return sourcecontrol.NewTag(tag.Name, sourcecontrol.CommitHash(tag.Hash)), nil
}

// GetLatestVersionTag retrieves the latest version tag.
func (a *Adapter) GetLatestVersionTag(ctx context.Context, prefix string) (*sourcecontrol.Tag, error) {
	tag, err := a.svc.GetLatestVersionTag(ctx, prefix)
	if err != nil {
		return nil, err
	}
	if tag == nil {
		return nil, nil
	}
	return sourcecontrol.NewTag(tag.Name, sourcecontrol.CommitHash(tag.Hash)), nil
}

// CreateTag creates a new tag.
func (a *Adapter) CreateTag(ctx context.Context, name string, hash sourcecontrol.CommitHash, message string) (*sourcecontrol.Tag, error) {
	opts := TagOptions{
		Annotated: message != "",
		Ref:       string(hash),
	}

	err := a.svc.CreateTag(ctx, name, message, opts)
	if err != nil {
		return nil, err
	}

	// Retrieve the created tag
	return a.GetTag(ctx, name)
}

// DeleteTag deletes a tag.
func (a *Adapter) DeleteTag(ctx context.Context, name string) error {
	return a.svc.DeleteTag(ctx, name)
}

// PushTag pushes a tag to a remote.
func (a *Adapter) PushTag(ctx context.Context, name string, remote string) error {
	ctx, cancel := withRemoteTimeout(ctx)
	defer cancel()

	opts := PushOptions{
		Remote:  remote,
		Tags:    true,
		RefSpec: "refs/tags/" + name,
	}
	return a.svc.PushTag(ctx, name, opts)
}

// IsDirty checks if the working tree is dirty.
func (a *Adapter) IsDirty(ctx context.Context) (bool, error) {
	clean, err := a.svc.IsClean(ctx)
	if err != nil {
		return false, err
	}
	return !clean, nil
}

// GetStatus retrieves the working tree status.
func (a *Adapter) GetStatus(ctx context.Context) (*sourcecontrol.WorkingTreeStatus, error) {
	clean, err := a.svc.IsClean(ctx)
	if err != nil {
		return nil, err
	}
	return &sourcecontrol.WorkingTreeStatus{
		IsClean: clean,
	}, nil
}

// Fetch fetches from a remote.
func (a *Adapter) Fetch(ctx context.Context, remote string) error {
	ctx, cancel := withRemoteTimeout(ctx)
	defer cancel()

	opts := FetchOptions{
		Remote: remote,
		Tags:   true,
	}
	return a.svc.Fetch(ctx, opts)
}

// Pull pulls from a remote.
func (a *Adapter) Pull(ctx context.Context, remote, branch string) error {
	ctx, cancel := withRemoteTimeout(ctx)
	defer cancel()

	opts := PullOptions{
		Remote: remote,
		Branch: branch,
	}
	return a.svc.Pull(ctx, opts)
}

// Push pushes to a remote.
func (a *Adapter) Push(ctx context.Context, remote, branch string) error {
	ctx, cancel := withRemoteTimeout(ctx)
	defer cancel()

	opts := PushOptions{
		Remote: remote,
	}
	return a.svc.Push(ctx, opts)
}

// Helper functions

func convertCommit(c *Commit) *sourcecontrol.Commit {
	if c == nil {
		return nil
	}
	commit := sourcecontrol.NewCommit(
		sourcecontrol.CommitHash(c.Hash),
		c.Message,
		sourcecontrol.Author{Name: c.Author.Name, Email: c.Author.Email},
		c.Date,
	)
	commit.SetCommitter(sourcecontrol.Author{Name: c.Committer.Name, Email: c.Committer.Email})
	return commit
}

func convertCommits(commits []Commit) []*sourcecontrol.Commit {
	result := make([]*sourcecontrol.Commit, len(commits))
	for i := range commits {
		result[i] = convertCommit(&commits[i])
	}
	return result
}

func extractRepoName(path string) string {
	parts := splitPath(path)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func extractOwner(url string) string {
	// Extract owner from remote URL
	// Supports formats:
	// - https://github.com/owner/repo.git
	// - git@github.com:owner/repo.git
	// - ssh://git@github.com/owner/repo.git

	url = strings.TrimSuffix(url, ".git")

	// Handle SSH format: git@github.com:owner/repo
	if strings.Contains(url, "@") && strings.Contains(url, ":") {
		// Find the colon after the host
		atIdx := strings.Index(url, "@")
		colonIdx := strings.Index(url[atIdx:], ":")
		if colonIdx != -1 {
			path := url[atIdx+colonIdx+1:]
			parts := strings.Split(path, "/")
			if len(parts) >= 1 {
				return parts[0]
			}
		}
	}

	// Handle HTTPS/SSH URL format: https://github.com/owner/repo
	parts := splitPath(url)
	// Look for the owner after the host (github.com, gitlab.com, etc.)
	for i, part := range parts {
		if strings.Contains(part, ".") && i+1 < len(parts) {
			// Found a domain, next part is the owner
			return parts[i+1]
		}
	}

	// Fallback: if we have at least 2 parts, assume second-to-last is owner
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}

	return ""
}

func splitPath(path string) []string {
	// Use strings.FieldsFunc for O(n) performance instead of string concatenation
	return strings.FieldsFunc(path, func(c rune) bool {
		return c == '/' || c == '\\'
	})
}
