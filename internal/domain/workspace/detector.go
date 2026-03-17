package workspace

import (
	"context"
)

// Detector defines the interface for workspace detection.
// Implementations should be able to detect various workspace configurations
// from filesystem markers and configuration files.
type Detector interface {
	// Detect attempts to detect a workspace starting from the given path.
	// It will search upward from the path to find workspace root indicators.
	Detect(ctx context.Context, startPath string, opts DetectionOptions) (*DetectionResult, error)

	// DetectFromRoot detects workspace configuration assuming the path is already the root.
	DetectFromRoot(ctx context.Context, rootPath string, opts DetectionOptions) (*DetectionResult, error)

	// DetectType returns just the workspace type without full package discovery.
	// This is a lightweight operation for quick checks.
	DetectType(ctx context.Context, path string) (WorkspaceType, error)

	// DiscoverPackages discovers all packages within a detected workspace.
	DiscoverPackages(ctx context.Context, workspace *Workspace) ([]*Package, error)

	// ValidateWorkspace checks if a workspace configuration is valid and consistent.
	ValidateWorkspace(ctx context.Context, workspace *Workspace) error
}

// ConfigResolver resolves workspace configuration from various sources.
type ConfigResolver interface {
	// ResolveConfig finds and parses the workspace configuration file.
	ResolveConfig(ctx context.Context, rootPath string) (*WorkspaceConfig, error)

	// MergeConfig merges detected configuration with explicit user configuration.
	MergeConfig(detected, explicit *WorkspaceConfig) *WorkspaceConfig
}

// WorkspaceConfig represents the configuration for a workspace.
// This can be loaded from .relicta.yaml or detected from workspace markers.
type WorkspaceConfig struct {
	// Enabled indicates if workspace mode is explicitly enabled.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Type is the workspace type (can be explicitly set or auto-detected).
	Type WorkspaceType `json:"type,omitempty" yaml:"type,omitempty"`

	// Strategy is the versioning strategy.
	Strategy WorkspaceStrategy `json:"strategy" yaml:"strategy"`

	// PackagePaths are glob patterns for finding packages.
	PackagePaths []string `json:"packagePaths" yaml:"packagePaths"`

	// ExcludePaths are glob patterns for paths to exclude.
	ExcludePaths []string `json:"excludePaths,omitempty" yaml:"excludePaths,omitempty"`

	// ReleaseGroups defines groups of packages released together.
	ReleaseGroups map[string][]string `json:"releaseGroups,omitempty" yaml:"releaseGroups,omitempty"`

	// DependencyCoordination configures automatic internal dependency updates.
	DependencyCoordination DependencyCoordination `json:"dependencyCoordination" yaml:"dependencyCoordination"`
}

// DependencyCoordination configures how internal dependencies are coordinated.
type DependencyCoordination struct {
	// AutoUpdate enables automatic updating of internal dependency versions.
	AutoUpdate bool `json:"autoUpdate" yaml:"autoUpdate"`

	// VersionPrefix is the prefix for internal dependency versions (e.g., "^", "~", "workspace:").
	VersionPrefix string `json:"versionPrefix" yaml:"versionPrefix"`
}

// DefaultWorkspaceConfig returns a sensible default workspace configuration.
func DefaultWorkspaceConfig() *WorkspaceConfig {
	return &WorkspaceConfig{
		Enabled:  true,
		Strategy: StrategyIndependent,
		PackagePaths: []string{
			"packages/*",
			"apps/*",
			"libs/*",
		},
		ExcludePaths: []string{
			"**/node_modules/**",
			"**/vendor/**",
			"**/.git/**",
		},
		DependencyCoordination: DependencyCoordination{
			AutoUpdate:    true,
			VersionPrefix: "workspace:",
		},
	}
}

// MarkerFile represents a file that indicates a specific workspace type.
type MarkerFile struct {
	// Name is the filename to look for.
	Name string

	// Type is the workspace type this marker indicates.
	Type WorkspaceType

	// PackageManager is the package manager this marker indicates.
	PackageManager PackageManagerType

	// Priority determines which marker takes precedence (higher = more specific).
	Priority int

	// RequiresContent indicates if the file content must be parsed to confirm.
	RequiresContent bool
}

// DefaultMarkerFiles returns the default marker files for workspace detection.
func DefaultMarkerFiles() []MarkerFile {
	return []MarkerFile{
		// pnpm workspace (highest priority for Node.js)
		{Name: "pnpm-workspace.yaml", Type: WorkspaceTypePnpm, PackageManager: PackageManagerPnpm, Priority: 100},

		// Lerna
		{Name: "lerna.json", Type: WorkspaceTypeLerna, PackageManager: PackageManagerNpm, Priority: 90},

		// Nx
		{Name: "nx.json", Type: WorkspaceTypeNx, PackageManager: PackageManagerNpm, Priority: 85},

		// Turborepo
		{Name: "turbo.json", Type: WorkspaceTypeTurborepo, PackageManager: PackageManagerNpm, Priority: 85},

		// npm/yarn workspaces (requires parsing package.json)
		{Name: "package.json", Type: WorkspaceTypeNpm, PackageManager: PackageManagerNpm, Priority: 50, RequiresContent: true},

		// Go module workspace
		{Name: "go.work", Type: WorkspaceTypeGoModule, PackageManager: PackageManagerGo, Priority: 100},

		// Cargo workspace
		{Name: "Cargo.toml", Type: WorkspaceTypeCargo, PackageManager: PackageManagerCargo, Priority: 80, RequiresContent: true},

		// Maven multi-module
		{Name: "pom.xml", Type: WorkspaceTypeMaven, PackageManager: PackageManagerMaven, Priority: 70, RequiresContent: true},

		// Gradle multi-project
		{Name: "settings.gradle", Type: WorkspaceTypeGradle, PackageManager: PackageManagerGradle, Priority: 75},
		{Name: "settings.gradle.kts", Type: WorkspaceTypeGradle, PackageManager: PackageManagerGradle, Priority: 75},

		// Lock files for package manager detection
		{Name: "pnpm-lock.yaml", Type: WorkspaceTypeNone, PackageManager: PackageManagerPnpm, Priority: 40},
		{Name: "yarn.lock", Type: WorkspaceTypeNone, PackageManager: PackageManagerYarn, Priority: 40},
		{Name: "package-lock.json", Type: WorkspaceTypeNone, PackageManager: PackageManagerNpm, Priority: 30},
		{Name: "bun.lockb", Type: WorkspaceTypeNone, PackageManager: PackageManagerBun, Priority: 40},
	}
}
