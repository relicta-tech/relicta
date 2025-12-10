// Package git provides git operations for ReleasePilot.
package git

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"

	rperrors "github.com/felixgeelhaar/release-pilot/internal/errors"
)

// errStopIteration is a sentinel error used to signal early termination of commit iteration.
var errStopIteration = errors.New("stop iteration")

// Ensure ServiceImpl implements Service.
var _ Service = (*ServiceImpl)(nil)

// repoInfoCacheTTL is how long repository info is cached before refreshing.
// Using a short TTL ensures data freshness while avoiding repeated syscalls
// during typical release operations that call GetRepositoryInfo multiple times.
const repoInfoCacheTTL = 5 * time.Second

// repoInfoCache holds cached repository information.
type repoInfoCache struct {
	info      *RepositoryInfo
	expiresAt time.Time
}

// ServiceImpl is the go-git implementation of the git service.
type ServiceImpl struct {
	cfg      ServiceConfig
	repo     *git.Repository
	worktree *git.Worktree
	auth     transport.AuthMethod

	// Repository info cache with TTL
	repoInfoMu    sync.RWMutex
	repoInfoCache *repoInfoCache
}

// NewService creates a new git service.
func NewService(opts ...ServiceOption) (*ServiceImpl, error) {
	cfg := DefaultServiceConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	absPath, err := filepath.Abs(cfg.RepoPath)
	if err != nil {
		return nil, rperrors.GitWrap(err, "git.NewService", "failed to get absolute path")
	}

	repo, err := git.PlainOpen(absPath)
	if err != nil {
		return nil, rperrors.GitWrap(err, "git.NewService", "failed to open repository")
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, rperrors.GitWrap(err, "git.NewService", "failed to get worktree")
	}

	return &ServiceImpl{
		cfg:      cfg,
		repo:     repo,
		worktree: worktree,
	}, nil
}

// GetRepositoryRoot returns the absolute path to the repository root.
func (s *ServiceImpl) GetRepositoryRoot(_ context.Context) (string, error) {
	return s.worktree.Filesystem.Root(), nil
}

// GetRepositoryInfo returns information about the repository.
// Results are cached for repoInfoCacheTTL to avoid repeated syscalls
// during operations that query repository info multiple times.
func (s *ServiceImpl) GetRepositoryInfo(ctx context.Context) (*RepositoryInfo, error) {
	// Try to return cached info first (optimistic read)
	s.repoInfoMu.RLock()
	if s.repoInfoCache != nil && time.Now().Before(s.repoInfoCache.expiresAt) {
		info := s.repoInfoCache.info
		s.repoInfoMu.RUnlock()
		return info, nil
	}
	s.repoInfoMu.RUnlock()

	// Cache miss or expired, fetch fresh data
	info, err := s.fetchRepositoryInfo(ctx)
	if err != nil {
		return nil, err
	}

	// Update cache
	s.repoInfoMu.Lock()
	s.repoInfoCache = &repoInfoCache{
		info:      info,
		expiresAt: time.Now().Add(repoInfoCacheTTL),
	}
	s.repoInfoMu.Unlock()

	return info, nil
}

// InvalidateRepoInfoCache forces the repository info cache to be refreshed
// on the next GetRepositoryInfo call. Use after operations that modify
// repository state (e.g., creating tags, switching branches).
func (s *ServiceImpl) InvalidateRepoInfoCache() {
	s.repoInfoMu.Lock()
	s.repoInfoCache = nil
	s.repoInfoMu.Unlock()
}

// fetchRepositoryInfo retrieves fresh repository information from git.
func (s *ServiceImpl) fetchRepositoryInfo(ctx context.Context) (*RepositoryInfo, error) {
	const op = "git.GetRepositoryInfo"

	root, err := s.GetRepositoryRoot(ctx)
	if err != nil {
		return nil, rperrors.GitWrap(err, op, "failed to get repository root")
	}

	branch, err := s.GetCurrentBranch(ctx)
	if err != nil {
		branch = "" // Detached HEAD
	}

	defaultBranch, err := s.GetDefaultBranch(ctx)
	if err != nil {
		defaultBranch = "main"
	}

	isDirty, err := s.IsClean(ctx)
	if err != nil {
		isDirty = false
	}

	head, err := s.GetHeadCommit(ctx)
	headHash := ""
	if err == nil {
		headHash = head.Hash
	}

	remotes, err := s.repo.Remotes()
	if err != nil {
		remotes = nil
	}

	remoteInfos := make([]RemoteInfo, 0, len(remotes))
	for _, remote := range remotes {
		cfg := remote.Config()
		info := RemoteInfo{
			Name: cfg.Name,
		}
		if len(cfg.URLs) > 0 {
			info.URL = cfg.URLs[0]
		}
		remoteInfos = append(remoteInfos, info)
	}

	return &RepositoryInfo{
		Root:          root,
		CurrentBranch: branch,
		DefaultBranch: defaultBranch,
		Remotes:       remoteInfos,
		IsDirty:       !isDirty,
		HeadCommit:    headHash,
	}, nil
}

// IsClean returns true if the working tree has no uncommitted changes.
func (s *ServiceImpl) IsClean(_ context.Context) (bool, error) {
	const op = "git.IsClean"

	status, err := s.worktree.Status()
	if err != nil {
		return false, rperrors.GitWrap(err, op, "failed to get worktree status")
	}

	return status.IsClean(), nil
}

// GetCommit returns a specific commit by hash.
func (s *ServiceImpl) GetCommit(_ context.Context, hash string) (*Commit, error) {
	const op = "git.GetCommit"

	commitObj, err := s.repo.CommitObject(plumbing.NewHash(hash))
	if err != nil {
		return nil, rperrors.GitWrap(err, op, "failed to get commit")
	}

	return s.convertCommit(commitObj), nil
}

// GetCommitsSince returns all commits since the given reference.
func (s *ServiceImpl) GetCommitsSince(ctx context.Context, ref string) ([]Commit, error) {
	const op = "git.GetCommitsSince"

	// Get the reference commit
	refHash, err := s.resolveRef(ref)
	if err != nil {
		return nil, rperrors.GitWrap(err, op, fmt.Sprintf("failed to resolve reference %s", ref))
	}

	// Get HEAD
	head, err := s.repo.Head()
	if err != nil {
		return nil, rperrors.GitWrap(err, op, "failed to get HEAD")
	}

	return s.getCommitsBetweenHashes(ctx, refHash, head.Hash())
}

// GetCommitsBetween returns all commits between two references.
func (s *ServiceImpl) GetCommitsBetween(ctx context.Context, from, to string) ([]Commit, error) {
	const op = "git.GetCommitsBetween"

	fromHash, err := s.resolveRef(from)
	if err != nil {
		return nil, rperrors.GitWrap(err, op, fmt.Sprintf("failed to resolve from reference %s", from))
	}

	toHash, err := s.resolveRef(to)
	if err != nil {
		return nil, rperrors.GitWrap(err, op, fmt.Sprintf("failed to resolve to reference %s", to))
	}

	return s.getCommitsBetweenHashes(ctx, fromHash, toHash)
}

// getCommitsBetweenHashes returns commits between two hashes.
func (s *ServiceImpl) getCommitsBetweenHashes(ctx context.Context, from, to plumbing.Hash) ([]Commit, error) {
	const op = "git.getCommitsBetweenHashes"
	const estimatedCommitsPerRelease = 50 // Pre-allocate for typical release size

	// Get iterator from 'to' commit
	iter, err := s.repo.Log(&git.LogOptions{
		From:  to,
		Order: git.LogOrderCommitterTime,
	})
	if err != nil {
		return nil, rperrors.GitWrap(err, op, "failed to get log iterator")
	}
	defer iter.Close()

	commits := make([]Commit, 0, estimatedCommitsPerRelease)
	err = iter.ForEach(func(c *object.Commit) error {
		// Check for context cancellation during iteration
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if c.Hash == from {
			return errStopIteration
		}
		commits = append(commits, *s.convertCommit(c))
		return nil
	})
	if err != nil && !errors.Is(err, errStopIteration) {
		// Return context error with proper wrapping
		if ctx.Err() != nil {
			return nil, rperrors.GitWrap(ctx.Err(), op, "operation canceled")
		}
		return nil, rperrors.GitWrap(err, op, "failed to iterate commits")
	}

	return commits, nil
}

// GetHeadCommit returns the current HEAD commit.
func (s *ServiceImpl) GetHeadCommit(_ context.Context) (*Commit, error) {
	const op = "git.GetHeadCommit"

	head, err := s.repo.Head()
	if err != nil {
		return nil, rperrors.GitWrap(err, op, "failed to get HEAD")
	}

	commit, err := s.repo.CommitObject(head.Hash())
	if err != nil {
		return nil, rperrors.GitWrap(err, op, "failed to get HEAD commit")
	}

	return s.convertCommit(commit), nil
}

// GetBranchCommit returns the latest commit on a specific branch.
func (s *ServiceImpl) GetBranchCommit(_ context.Context, branch string) (*Commit, error) {
	const op = "git.GetBranchCommit"

	// Resolve the branch to a hash
	hash, err := s.resolveRef(branch)
	if err != nil {
		return nil, rperrors.GitWrap(err, op, fmt.Sprintf("failed to resolve branch %s", branch))
	}

	commit, err := s.repo.CommitObject(hash)
	if err != nil {
		return nil, rperrors.GitWrap(err, op, fmt.Sprintf("failed to get commit for branch %s", branch))
	}

	return s.convertCommit(commit), nil
}

// GetLatestTag returns the most recent tag.
func (s *ServiceImpl) GetLatestTag(ctx context.Context) (*Tag, error) {
	tags, err := s.ListTags(ctx)
	if err != nil {
		return nil, err
	}

	if len(tags) == 0 {
		return nil, rperrors.NotFound("git.GetLatestTag", "no tags found")
	}

	// Tags are sorted by date, newest first
	return &tags[0], nil
}

// GetLatestVersionTag returns the most recent version tag matching the prefix.
func (s *ServiceImpl) GetLatestVersionTag(ctx context.Context, prefix string) (*Tag, error) {
	tags, err := s.ListVersionTags(ctx, prefix)
	if err != nil {
		return nil, err
	}

	if len(tags) == 0 {
		return nil, rperrors.NotFound("git.GetLatestVersionTag", "no version tags found")
	}

	return &tags[0], nil
}

// ListTags returns all tags in the repository.
func (s *ServiceImpl) ListTags(ctx context.Context) ([]Tag, error) {
	const op = "git.ListTags"
	const estimatedTags = 20 // Pre-allocate for typical tag count

	tags := make([]Tag, 0, estimatedTags)

	iter, err := s.repo.Tags()
	if err != nil {
		return nil, rperrors.GitWrap(err, op, "failed to get tags iterator")
	}
	defer iter.Close()

	err = iter.ForEach(func(ref *plumbing.Reference) error {
		// Check for context cancellation during iteration
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		tag, convertErr := s.convertTag(ref)
		if convertErr != nil {
			return convertErr
		}
		tags = append(tags, *tag)
		return nil
	})
	if err != nil {
		// Return context error with proper wrapping
		if ctx.Err() != nil {
			return nil, rperrors.GitWrap(ctx.Err(), op, "operation canceled")
		}
		return nil, rperrors.GitWrap(err, op, "failed to iterate tags")
	}

	// Sort by date, newest first
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Date.After(tags[j].Date)
	})

	return tags, nil
}

// versionTagCache holds a tag with its pre-parsed semver version.
type versionTagCache struct {
	tag     Tag
	version *semver.Version
}

// ListVersionTags returns all version tags matching the prefix.
func (s *ServiceImpl) ListVersionTags(ctx context.Context, prefix string) ([]Tag, error) {
	allTags, err := s.ListTags(ctx)
	if err != nil {
		return nil, err
	}

	// Parse semver once and cache to avoid repeated parsing
	cache := make([]versionTagCache, 0, len(allTags))
	for _, tag := range allTags {
		name := tag.Name
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}

		versionStr := strings.TrimPrefix(name, prefix)
		if v, err := semver.NewVersion(versionStr); err == nil {
			cache = append(cache, versionTagCache{tag: tag, version: v})
		}
	}

	// Sort by semver using cached versions, newest first
	sort.Slice(cache, func(i, j int) bool {
		return cache[i].version.GreaterThan(cache[j].version)
	})

	// Extract tags from cache
	versionTags := make([]Tag, len(cache))
	for i, c := range cache {
		versionTags[i] = c.tag
	}

	return versionTags, nil
}

// GetTag returns a specific tag by name, or an error if not found.
func (s *ServiceImpl) GetTag(_ context.Context, name string) (*Tag, error) {
	const op = "git.GetTag"

	tagRef, err := s.repo.Tag(name)
	if err != nil {
		return nil, rperrors.Git(op, fmt.Sprintf("tag not found: %s", name))
	}

	// Resolve tag object
	obj, err := s.repo.TagObject(tagRef.Hash())
	if err != nil {
		// Might be a lightweight tag
		commit, err := s.repo.CommitObject(tagRef.Hash())
		if err != nil {
			return nil, rperrors.GitWrap(err, op, "failed to resolve tag")
		}
		return &Tag{
			Name:    name,
			Hash:    tagRef.Hash().String(),
			Message: "",
			Date:    commit.Committer.When,
		}, nil
	}

	return &Tag{
		Name:    name,
		Hash:    obj.Hash.String(),
		Message: obj.Message,
		Date:    obj.Tagger.When,
	}, nil
}

// CreateTag creates a new tag.
func (s *ServiceImpl) CreateTag(_ context.Context, name, message string, opts TagOptions) error {
	const op = "git.CreateTag"

	// Resolve the reference
	ref := opts.Ref
	if ref == "" {
		ref = "HEAD"
	}

	hash, err := s.resolveRef(ref)
	if err != nil {
		return rperrors.GitWrap(err, op, fmt.Sprintf("failed to resolve reference %s", ref))
	}

	if opts.Annotated {
		// Create annotated tag
		_, err = s.repo.CreateTag(name, hash, &git.CreateTagOptions{
			Message: message,
			Tagger: &object.Signature{
				Name:  "ReleasePilot",
				Email: "release-pilot@localhost",
				When:  time.Now(),
			},
		})
	} else {
		// Create lightweight tag
		refName := plumbing.NewTagReferenceName(name)
		tagRef := plumbing.NewHashReference(refName, hash)
		err = s.repo.Storer.SetReference(tagRef)
	}

	if err != nil {
		return rperrors.GitWrap(err, op, fmt.Sprintf("failed to create tag %s", name))
	}

	// Invalidate cache since repository state changed
	s.InvalidateRepoInfoCache()

	return nil
}

// DeleteTag deletes a tag.
func (s *ServiceImpl) DeleteTag(_ context.Context, name string) error {
	const op = "git.DeleteTag"

	refName := plumbing.NewTagReferenceName(name)
	err := s.repo.Storer.RemoveReference(refName)
	if err != nil {
		return rperrors.GitWrap(err, op, fmt.Sprintf("failed to delete tag %s", name))
	}

	// Invalidate cache since repository state changed
	s.InvalidateRepoInfoCache()

	return nil
}

// PushTag pushes a tag to the remote.
func (s *ServiceImpl) PushTag(_ context.Context, name string, opts PushOptions) error {
	const op = "git.PushTag"

	if opts.DryRun {
		return nil // Dry run, don't actually push
	}

	remote := opts.Remote
	if remote == "" {
		remote = s.cfg.DefaultRemote
	}

	refSpec := config.RefSpec(fmt.Sprintf("refs/tags/%s:refs/tags/%s", name, name))

	err := s.repo.Push(&git.PushOptions{
		RemoteName: remote,
		RefSpecs:   []config.RefSpec{refSpec},
		Auth:       s.auth,
		Force:      opts.Force,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return rperrors.GitWrap(err, op, fmt.Sprintf("failed to push tag %s", name))
	}

	return nil
}

// GetCurrentBranch returns the current branch name.
func (s *ServiceImpl) GetCurrentBranch(_ context.Context) (string, error) {
	const op = "git.GetCurrentBranch"

	head, err := s.repo.Head()
	if err != nil {
		return "", rperrors.GitWrap(err, op, "failed to get HEAD")
	}

	if !head.Name().IsBranch() {
		return "", rperrors.Git(op, "HEAD is not on a branch (detached HEAD)")
	}

	return head.Name().Short(), nil
}

// GetDefaultBranch returns the default branch name (main/master).
func (s *ServiceImpl) GetDefaultBranch(_ context.Context) (string, error) {
	// Try to get from remote HEAD reference
	remote, err := s.repo.Remote(s.cfg.DefaultRemote)
	if err == nil {
		refs, err := remote.List(&git.ListOptions{Auth: s.auth})
		if err == nil {
			for _, ref := range refs {
				if ref.Name() == plumbing.HEAD {
					target := ref.Target()
					if target.IsBranch() {
						return target.Short(), nil
					}
				}
			}
		}
	}

	// Fallback to common defaults
	for _, name := range []string{"main", "master"} {
		ref, err := s.repo.Reference(plumbing.NewBranchReferenceName(name), true)
		if err == nil && ref != nil {
			return name, nil
		}
	}

	return "main", nil
}

// ListBranches returns all branches in the repository.
func (s *ServiceImpl) ListBranches(ctx context.Context) ([]Branch, error) {
	const op = "git.ListBranches"
	const estimatedBranches = 10 // Pre-allocate for typical branch count

	branches := make([]Branch, 0, estimatedBranches)

	iter, err := s.repo.Branches()
	if err != nil {
		return nil, rperrors.GitWrap(err, op, "failed to get branches iterator")
	}
	defer iter.Close()

	err = iter.ForEach(func(ref *plumbing.Reference) error {
		// Check for context cancellation during iteration
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		branches = append(branches, Branch{
			Name:     ref.Name().Short(),
			Hash:     ref.Hash().String(),
			IsRemote: ref.Name().IsRemote(),
		})
		return nil
	})
	if err != nil {
		// Return context error with proper wrapping
		if ctx.Err() != nil {
			return nil, rperrors.GitWrap(ctx.Err(), op, "operation canceled")
		}
		return nil, rperrors.GitWrap(err, op, "failed to iterate branches")
	}

	return branches, nil
}

// GetRemoteURL returns the URL of the specified remote.
func (s *ServiceImpl) GetRemoteURL(_ context.Context, name string) (string, error) {
	const op = "git.GetRemoteURL"

	remote, err := s.repo.Remote(name)
	if err != nil {
		return "", rperrors.GitWrap(err, op, fmt.Sprintf("failed to get remote %s", name))
	}

	cfg := remote.Config()
	if len(cfg.URLs) == 0 {
		return "", rperrors.NotFound(op, fmt.Sprintf("remote %s has no URLs", name))
	}

	return cfg.URLs[0], nil
}

// Push pushes changes to the remote.
func (s *ServiceImpl) Push(_ context.Context, opts PushOptions) error {
	const op = "git.Push"

	if opts.DryRun {
		return nil
	}

	remote := opts.Remote
	if remote == "" {
		remote = s.cfg.DefaultRemote
	}

	pushOpts := &git.PushOptions{
		RemoteName: remote,
		Auth:       s.auth,
		Force:      opts.Force,
	}

	if opts.RefSpec != "" {
		pushOpts.RefSpecs = []config.RefSpec{config.RefSpec(opts.RefSpec)}
	}

	err := s.repo.Push(pushOpts)
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return rperrors.GitWrap(err, op, "failed to push")
	}

	return nil
}

// Fetch fetches from the remote.
func (s *ServiceImpl) Fetch(_ context.Context, opts FetchOptions) error {
	const op = "git.Fetch"

	remote := opts.Remote
	if remote == "" {
		remote = s.cfg.DefaultRemote
	}

	fetchOpts := &git.FetchOptions{
		RemoteName: remote,
		Auth:       s.auth,
		Prune:      opts.Prune,
	}

	if opts.Tags {
		fetchOpts.Tags = git.AllTags
	}

	if opts.Depth > 0 {
		fetchOpts.Depth = opts.Depth
	}

	err := s.repo.Fetch(fetchOpts)
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return rperrors.GitWrap(err, op, "failed to fetch")
	}

	return nil
}

// Pull pulls changes from the remote and merges them.
func (s *ServiceImpl) Pull(_ context.Context, opts PullOptions) error {
	const op = "git.Pull"

	remote := opts.Remote
	if remote == "" {
		remote = s.cfg.DefaultRemote
	}

	pullOpts := &git.PullOptions{
		RemoteName: remote,
		Auth:       s.auth,
	}

	if opts.Branch != "" {
		pullOpts.ReferenceName = plumbing.NewBranchReferenceName(opts.Branch)
	}

	if opts.Depth > 0 {
		pullOpts.Depth = opts.Depth
	}

	err := s.worktree.Pull(pullOpts)
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return rperrors.GitWrap(err, op, "failed to pull")
	}

	return nil
}

// GetDiffStats returns statistics about changes between two refs.
func (s *ServiceImpl) GetDiffStats(_ context.Context, from, to string) (*DiffStats, error) {
	const op = "git.GetDiffStats"

	fromHash, err := s.resolveRef(from)
	if err != nil {
		return nil, rperrors.GitWrap(err, op, fmt.Sprintf("failed to resolve from reference %s", from))
	}

	toHash, err := s.resolveRef(to)
	if err != nil {
		return nil, rperrors.GitWrap(err, op, fmt.Sprintf("failed to resolve to reference %s", to))
	}

	fromCommit, err := s.repo.CommitObject(fromHash)
	if err != nil {
		return nil, rperrors.GitWrap(err, op, "failed to get from commit")
	}

	toCommit, err := s.repo.CommitObject(toHash)
	if err != nil {
		return nil, rperrors.GitWrap(err, op, "failed to get to commit")
	}

	fromTree, err := fromCommit.Tree()
	if err != nil {
		return nil, rperrors.GitWrap(err, op, "failed to get from tree")
	}

	toTree, err := toCommit.Tree()
	if err != nil {
		return nil, rperrors.GitWrap(err, op, "failed to get to tree")
	}

	changes, err := fromTree.Diff(toTree)
	if err != nil {
		return nil, rperrors.GitWrap(err, op, "failed to compute diff")
	}

	stats := &DiffStats{
		FilesChanged: len(changes),
	}

	for _, change := range changes {
		patch, err := change.Patch()
		if err != nil {
			continue
		}

		for _, fileStat := range patch.Stats() {
			stats.Insertions += fileStat.Addition
			stats.Deletions += fileStat.Deletion
			stats.Files = append(stats.Files, FileStats{
				Path:       fileStat.Name,
				Insertions: fileStat.Addition,
				Deletions:  fileStat.Deletion,
			})
		}
	}

	return stats, nil
}

// ParseConventionalCommit parses a commit message as a conventional commit.
func (s *ServiceImpl) ParseConventionalCommit(message string) (*ConventionalCommit, error) {
	return ParseConventionalCommit(message)
}

// ParseConventionalCommits parses multiple commits as conventional commits.
func (s *ServiceImpl) ParseConventionalCommits(commits []Commit, opts ParseOptions) ([]ConventionalCommit, error) {
	conventionalCommits := make([]ConventionalCommit, 0, len(commits))

	for _, commit := range commits {
		cc, err := ParseConventionalCommitWithOptions(commit, opts)
		if err != nil {
			if opts.StrictMode {
				return nil, err
			}
			// In non-strict mode, include non-conventional commits
			cc = &ConventionalCommit{
				Commit:         commit,
				Type:           CommitTypeUnknown,
				Description:    commit.Subject,
				Body:           commit.Body,
				IsConventional: false,
			}
		}
		conventionalCommits = append(conventionalCommits, *cc)
	}

	return conventionalCommits, nil
}

// DetectReleaseType determines the release type based on conventional commits.
func (s *ServiceImpl) DetectReleaseType(commits []ConventionalCommit) ReleaseType {
	return DetectReleaseType(commits)
}

// CategorizeCommits groups commits by their type.
func (s *ServiceImpl) CategorizeCommits(commits []ConventionalCommit) *CategorizedChanges {
	return CategorizeCommits(commits)
}

// FilterCommits filters commits based on criteria.
func (s *ServiceImpl) FilterCommits(commits []ConventionalCommit, filter CommitFilter) []ConventionalCommit {
	return FilterCommits(commits, filter)
}

// Helper methods

// resolveRef resolves a reference (tag, branch, or commit hash) to a hash.
func (s *ServiceImpl) resolveRef(ref string) (plumbing.Hash, error) {
	// Try as a hash first
	if plumbing.IsHash(ref) {
		return plumbing.NewHash(ref), nil
	}

	// Try as a reference
	resolved, err := s.repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to resolve reference %s: %w", ref, err)
	}

	return *resolved, nil
}

// convertCommit converts a go-git commit to our Commit type.
func (s *ServiceImpl) convertCommit(c *object.Commit) *Commit {
	subject, body := splitMessage(c.Message)

	// Pre-allocate parents slice
	parents := make([]string, 0, len(c.ParentHashes))
	for _, parent := range c.ParentHashes {
		parents = append(parents, parent.String())
	}

	// Convert hash to string once to avoid duplicate conversion
	hashStr := c.Hash.String()

	return &Commit{
		Hash:      hashStr,
		ShortHash: hashStr[:7],
		Message:   c.Message,
		Subject:   subject,
		Body:      body,
		Author: Author{
			Name:  c.Author.Name,
			Email: c.Author.Email,
		},
		Committer: Author{
			Name:  c.Committer.Name,
			Email: c.Committer.Email,
		},
		Date:    c.Author.When,
		Parents: parents,
	}
}

// convertTag converts a go-git tag reference to our Tag type.
func (s *ServiceImpl) convertTag(ref *plumbing.Reference) (*Tag, error) {
	tag := &Tag{
		Name: ref.Name().Short(),
		Hash: ref.Hash().String(),
	}

	// Try to get annotated tag object
	tagObj, err := s.repo.TagObject(ref.Hash())
	if err == nil {
		// Annotated tag
		tag.Message = tagObj.Message
		tag.IsAnnotated = true
		tag.Date = tagObj.Tagger.When
		tag.Tagger = &Author{
			Name:  tagObj.Tagger.Name,
			Email: tagObj.Tagger.Email,
		}
		// Get the commit hash that the tag points to
		commit, err := tagObj.Commit()
		if err == nil {
			tag.Hash = commit.Hash.String()
		}
	} else {
		// Lightweight tag - get commit date
		commit, err := s.repo.CommitObject(ref.Hash())
		if err == nil {
			tag.Date = commit.Author.When
		} else {
			tag.Date = time.Now()
		}
	}

	return tag, nil
}

// splitMessage splits a commit message into subject and body.
func splitMessage(message string) (subject, body string) {
	lines := strings.SplitN(strings.TrimSpace(message), "\n", 2)
	subject = strings.TrimSpace(lines[0])
	if len(lines) > 1 {
		body = strings.TrimSpace(lines[1])
	}
	return subject, body
}
