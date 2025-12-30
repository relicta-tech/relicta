package monorepo

import (
	"context"
)

// Repository defines the interface for persisting monorepo releases.
type Repository interface {
	// Save persists a monorepo release.
	Save(ctx context.Context, release *MonorepoRelease) error

	// FindByID retrieves a release by its ID.
	FindByID(ctx context.Context, id MonorepoReleaseID) (*MonorepoRelease, error)

	// FindActive returns the currently active (non-terminal) release for a repo.
	FindActive(ctx context.Context, repoID string) (*MonorepoRelease, error)

	// FindLatest returns the most recent release for a repo.
	FindLatest(ctx context.Context, repoID string) (*MonorepoRelease, error)

	// FindByState returns all releases in a given state.
	FindByState(ctx context.Context, state MonorepoReleaseState) ([]*MonorepoRelease, error)

	// List returns all releases, optionally filtered.
	List(ctx context.Context, opts ListOptions) ([]*MonorepoRelease, error)

	// Delete removes a release by ID.
	Delete(ctx context.Context, id MonorepoReleaseID) error
}

// ListOptions configures the list operation.
type ListOptions struct {
	// RepoID filters by repository ID.
	RepoID string
	// State filters by release state.
	State MonorepoReleaseState
	// Limit limits the number of results.
	Limit int
	// Offset skips the first N results.
	Offset int
	// IncludeTerminal includes terminal (published/failed/canceled) releases.
	IncludeTerminal bool
}

// EventPublisher publishes domain events.
type EventPublisher interface {
	// Publish publishes domain events.
	Publish(ctx context.Context, events ...DomainEvent) error
}

// PackageDiscoverer discovers packages in a repository.
type PackageDiscoverer interface {
	// DiscoverPackages finds all packages in the repository.
	DiscoverPackages(ctx context.Context, repoPath string, opts DiscoverOptions) ([]*DiscoveredPackage, error)
}

// DiscoverOptions configures package discovery.
type DiscoverOptions struct {
	// Paths are glob patterns for package locations.
	Paths []string
	// ExcludePaths are patterns to exclude.
	ExcludePaths []string
	// IncludeRoot includes the root directory as a package.
	IncludeRoot bool
}

// DiscoveredPackage represents a discovered package.
type DiscoveredPackage struct {
	// Path is the path to the package relative to repo root.
	Path string
	// Name is the package name.
	Name string
	// Type is the package type.
	Type PackageType
	// VersionFile is the file containing the version.
	VersionFile string
	// CurrentVersion is the current version of the package.
	CurrentVersion string
	// Dependencies are internal package dependencies.
	Dependencies []string
}

// VersionFileWriter writes version updates to package files.
type VersionFileWriter interface {
	// CanHandle returns true if this writer can handle the package type.
	CanHandle(pkgType PackageType) bool

	// ReadVersion reads the current version from the package.
	ReadVersion(ctx context.Context, pkgPath string) (string, error)

	// WriteVersion updates the version in the package files.
	WriteVersion(ctx context.Context, pkgPath, version string) error

	// Files returns the files that will be modified.
	Files(pkgPath string) []string
}

// ChangeAnalyzer analyzes changes for packages.
type ChangeAnalyzer interface {
	// AnalyzeChanges analyzes commits affecting packages.
	AnalyzeChanges(ctx context.Context, baseRef, headRef string, packages []*DiscoveredPackage) (*ChangeAnalysis, error)
}

// ChangeAnalysis contains the result of analyzing changes.
type ChangeAnalysis struct {
	// PackageChanges maps package paths to their changes.
	PackageChanges map[string]*PackageChanges
	// SharedChanges are changes to shared directories.
	SharedChanges []string
	// TotalCommits is the total number of commits analyzed.
	TotalCommits int
}

// PackageChanges contains changes for a single package.
type PackageChanges struct {
	// Files are the changed files in this package.
	Files []string
	// Commits are commit messages affecting this package.
	Commits []string
	// BumpType is the inferred bump type from commit messages.
	BumpType BumpType
	// HasBreakingChanges indicates if there are breaking changes.
	HasBreakingChanges bool
}
