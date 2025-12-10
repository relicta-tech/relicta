// Package sourcecontrol provides domain types for source control operations.
package sourcecontrol

import (
	"context"

	"github.com/felixgeelhaar/release-pilot/internal/domain/version"
)

// RepositoryInfo represents repository metadata.
type RepositoryInfo struct {
	Path          string
	Name          string
	Owner         string
	RemoteURL     string
	DefaultBranch string
	CurrentBranch string
	IsDirty       bool
}

// RemoteInfo represents a git remote.
type RemoteInfo struct {
	Name string
	URL  string
}

// BranchInfo represents a git branch.
type BranchInfo struct {
	Name      string
	IsRemote  bool
	IsCurrent bool
	Hash      CommitHash
	Upstream  string
}

// RepositoryInfoReader provides read access to repository metadata.
// Use this interface when you only need to read repository information.
type RepositoryInfoReader interface {
	GetInfo(ctx context.Context) (*RepositoryInfo, error)
	GetRemotes(ctx context.Context) ([]RemoteInfo, error)
	GetBranches(ctx context.Context) ([]BranchInfo, error)
	GetCurrentBranch(ctx context.Context) (string, error)
}

// CommitReader provides read access to commits.
// Use this interface when you only need to read commit history.
type CommitReader interface {
	GetCommit(ctx context.Context, hash CommitHash) (*Commit, error)
	GetCommitsBetween(ctx context.Context, from, to string) ([]*Commit, error)
	GetCommitsSince(ctx context.Context, ref string) ([]*Commit, error)
	GetLatestCommit(ctx context.Context, branch string) (*Commit, error)
}

// TagReader provides read access to tags.
// Use this interface when you only need to read tag information.
type TagReader interface {
	GetTags(ctx context.Context) (TagList, error)
	GetTag(ctx context.Context, name string) (*Tag, error)
	GetLatestVersionTag(ctx context.Context, prefix string) (*Tag, error)
}

// TagWriter provides write access to tags.
// Use this interface when you need to create, delete, or push tags.
type TagWriter interface {
	CreateTag(ctx context.Context, name string, hash CommitHash, message string) (*Tag, error)
	DeleteTag(ctx context.Context, name string) error
	PushTag(ctx context.Context, name string, remote string) error
}

// TagManager combines read and write access to tags.
// Use this interface when you need full tag management capabilities.
type TagManager interface {
	TagReader
	TagWriter
}

// WorkingTreeInspector provides methods to inspect the working tree status.
// Use this interface when you only need to check working tree state.
type WorkingTreeInspector interface {
	IsDirty(ctx context.Context) (bool, error)
	GetStatus(ctx context.Context) (*WorkingTreeStatus, error)
}

// RemoteOperator provides operations for interacting with remote repositories.
// Use this interface when you need to sync with remote repositories.
type RemoteOperator interface {
	Fetch(ctx context.Context, remote string) error
	Pull(ctx context.Context, remote, branch string) error
	Push(ctx context.Context, remote, branch string) error
}

// GitRepository defines the full interface for git operations.
// This is a repository interface in DDD - implemented in infrastructure layer.
// For more focused use cases, consider using the smaller interfaces:
// - RepositoryInfoReader: for reading repository metadata
// - CommitReader: for reading commit history
// - TagReader/TagWriter/TagManager: for tag operations
// - WorkingTreeInspector: for checking working tree status
// - RemoteOperator: for remote synchronization
type GitRepository interface {
	RepositoryInfoReader
	CommitReader
	TagManager
	WorkingTreeInspector
	RemoteOperator
}

// WorkingTreeStatus represents the status of the working tree.
type WorkingTreeStatus struct {
	IsClean   bool
	Staged    []FileChange
	Unstaged  []FileChange
	Untracked []string
}

// FileChange represents a file change.
type FileChange struct {
	Path   string
	Status FileStatus
}

// FileStatus represents the status of a file change.
type FileStatus string

const (
	FileStatusAdded    FileStatus = "added"
	FileStatusModified FileStatus = "modified"
	FileStatusDeleted  FileStatus = "deleted"
	FileStatusRenamed  FileStatus = "renamed"
	FileStatusCopied   FileStatus = "copied"
)

// VersionDiscovery provides methods for discovering versions from tags.
type VersionDiscovery struct {
	tagPrefix string
}

// NewVersionDiscovery creates a new VersionDiscovery.
func NewVersionDiscovery(tagPrefix string) *VersionDiscovery {
	return &VersionDiscovery{tagPrefix: tagPrefix}
}

// DiscoverCurrentVersion finds the current version from tags.
func (vd *VersionDiscovery) DiscoverCurrentVersion(ctx context.Context, repo GitRepository) (version.SemanticVersion, error) {
	tag, err := repo.GetLatestVersionTag(ctx, vd.tagPrefix)
	if err != nil {
		return version.Zero, err
	}

	if tag == nil || tag.Version() == nil {
		return version.Initial, nil
	}

	return *tag.Version(), nil
}

// DiscoverAllVersions finds all versions from tags.
func (vd *VersionDiscovery) DiscoverAllVersions(ctx context.Context, repo GitRepository) ([]version.SemanticVersion, error) {
	tags, err := repo.GetTags(ctx)
	if err != nil {
		return nil, err
	}

	versionTags := tags.FilterByPrefix(vd.tagPrefix).VersionTags()
	versions := make([]version.SemanticVersion, 0, len(versionTags))
	for _, t := range versionTags {
		if t.Version() != nil {
			versions = append(versions, *t.Version())
		}
	}

	return versions, nil
}
