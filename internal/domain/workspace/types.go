// Package workspace provides workspace detection and management for monorepos.
// It supports auto-detection of various workspace configurations including
// pnpm, npm, yarn workspaces, Go modules, and custom configurations.
package workspace

import (
	"time"
)

// WorkspaceType represents the type of workspace detected.
type WorkspaceType string

const (
	// WorkspaceTypeNone indicates no workspace structure (single project).
	WorkspaceTypeNone WorkspaceType = "none"

	// WorkspaceTypePnpm indicates a pnpm workspace.
	WorkspaceTypePnpm WorkspaceType = "pnpm"

	// WorkspaceTypeNpm indicates an npm workspace.
	WorkspaceTypeNpm WorkspaceType = "npm"

	// WorkspaceTypeYarn indicates a yarn workspace.
	WorkspaceTypeYarn WorkspaceType = "yarn"

	// WorkspaceTypeLerna indicates a Lerna monorepo.
	WorkspaceTypeLerna WorkspaceType = "lerna"

	// WorkspaceTypeNx indicates an Nx workspace.
	WorkspaceTypeNx WorkspaceType = "nx"

	// WorkspaceTypeTurborepo indicates a Turborepo monorepo.
	WorkspaceTypeTurborepo WorkspaceType = "turborepo"

	// WorkspaceTypeGoModule indicates a Go module workspace.
	WorkspaceTypeGoModule WorkspaceType = "go_module"

	// WorkspaceTypeCargo indicates a Cargo/Rust workspace.
	WorkspaceTypeCargo WorkspaceType = "cargo"

	// WorkspaceTypeMaven indicates a Maven multi-module project.
	WorkspaceTypeMaven WorkspaceType = "maven"

	// WorkspaceTypeGradle indicates a Gradle multi-project build.
	WorkspaceTypeGradle WorkspaceType = "gradle"

	// WorkspaceTypeCustom indicates a custom workspace configuration.
	WorkspaceTypeCustom WorkspaceType = "custom"
)

// String returns the string representation of the workspace type.
func (wt WorkspaceType) String() string {
	return string(wt)
}

// IsMonorepo returns true if the workspace type indicates a monorepo structure.
func (wt WorkspaceType) IsMonorepo() bool {
	return wt != WorkspaceTypeNone
}

// PackageManagerType represents the package manager used in the workspace.
type PackageManagerType string

const (
	PackageManagerUnknown PackageManagerType = "unknown"
	PackageManagerNpm     PackageManagerType = "npm"
	PackageManagerPnpm    PackageManagerType = "pnpm"
	PackageManagerYarn    PackageManagerType = "yarn"
	PackageManagerBun     PackageManagerType = "bun"
	PackageManagerGo      PackageManagerType = "go"
	PackageManagerCargo   PackageManagerType = "cargo"
	PackageManagerMaven   PackageManagerType = "maven"
	PackageManagerGradle  PackageManagerType = "gradle"
	PackageManagerPip     PackageManagerType = "pip"
	PackageManagerPoetry  PackageManagerType = "poetry"
)

// String returns the string representation of the package manager type.
func (pm PackageManagerType) String() string {
	return string(pm)
}

// WorkspaceStrategy defines how packages in a workspace are versioned.
type WorkspaceStrategy string

const (
	// StrategyIndependent means each package has its own independent version.
	StrategyIndependent WorkspaceStrategy = "independent"

	// StrategyLockstep means all packages share the same version.
	StrategyLockstep WorkspaceStrategy = "lockstep"

	// StrategyHybrid means packages are grouped with shared versions within groups.
	StrategyHybrid WorkspaceStrategy = "hybrid"
)

// String returns the string representation of the workspace strategy.
func (ws WorkspaceStrategy) String() string {
	return string(ws)
}

// Package represents a package discovered within a workspace.
type Package struct {
	// Name is the package name (e.g., "@scope/package").
	Name string `json:"name" yaml:"name"`

	// Path is the relative path from workspace root to the package.
	Path string `json:"path" yaml:"path"`

	// Version is the current version of the package.
	Version string `json:"version" yaml:"version"`

	// Private indicates if the package is private (not published).
	Private bool `json:"private" yaml:"private"`

	// Dependencies lists the package's dependencies (name -> version constraint).
	Dependencies map[string]string `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`

	// DevDependencies lists the package's dev dependencies.
	DevDependencies map[string]string `json:"devDependencies,omitempty" yaml:"devDependencies,omitempty"`

	// InternalDependencies lists dependencies on other workspace packages.
	InternalDependencies []string `json:"internalDependencies,omitempty" yaml:"internalDependencies,omitempty"`
}

// Workspace represents a detected workspace configuration.
type Workspace struct {
	// ID is a unique identifier for the workspace (usually derived from root path).
	ID string `json:"id" yaml:"id"`

	// RootPath is the absolute path to the workspace root.
	RootPath string `json:"rootPath" yaml:"rootPath"`

	// Type is the detected workspace type.
	Type WorkspaceType `json:"type" yaml:"type"`

	// PackageManager is the detected package manager.
	PackageManager PackageManagerType `json:"packageManager" yaml:"packageManager"`

	// Strategy is the versioning strategy for the workspace.
	Strategy WorkspaceStrategy `json:"strategy" yaml:"strategy"`

	// PackagePaths contains glob patterns for package locations.
	PackagePaths []string `json:"packagePaths" yaml:"packagePaths"`

	// ExcludePaths contains glob patterns for paths to exclude.
	ExcludePaths []string `json:"excludePaths,omitempty" yaml:"excludePaths,omitempty"`

	// Packages contains the discovered packages.
	Packages []*Package `json:"packages,omitempty" yaml:"packages,omitempty"`

	// ConfigFile is the path to the workspace configuration file.
	ConfigFile string `json:"configFile,omitempty" yaml:"configFile,omitempty"`

	// DetectedAt is when the workspace was detected.
	DetectedAt time.Time `json:"detectedAt" yaml:"detectedAt"`

	// Confidence is the detection confidence (0.0 - 1.0).
	Confidence float64 `json:"confidence" yaml:"confidence"`

	// Markers lists the files/directories that indicated this workspace type.
	Markers []string `json:"markers,omitempty" yaml:"markers,omitempty"`
}

// IsEmpty returns true if no workspace was detected.
func (w *Workspace) IsEmpty() bool {
	return w == nil || w.Type == WorkspaceTypeNone
}

// PackageCount returns the number of packages in the workspace.
func (w *Workspace) PackageCount() int {
	if w == nil {
		return 0
	}
	return len(w.Packages)
}

// FindPackage finds a package by name.
func (w *Workspace) FindPackage(name string) *Package {
	if w == nil {
		return nil
	}
	for _, pkg := range w.Packages {
		if pkg.Name == name {
			return pkg
		}
	}
	return nil
}

// FindPackageByPath finds a package by its path.
func (w *Workspace) FindPackageByPath(path string) *Package {
	if w == nil {
		return nil
	}
	for _, pkg := range w.Packages {
		if pkg.Path == path {
			return pkg
		}
	}
	return nil
}

// DetectionResult contains the result of workspace detection.
type DetectionResult struct {
	// Workspace is the detected workspace (nil if detection failed).
	Workspace *Workspace `json:"workspace,omitempty" yaml:"workspace,omitempty"`

	// Error is set if detection failed.
	Error error `json:"error,omitempty" yaml:"error,omitempty"`

	// Duration is how long detection took.
	Duration time.Duration `json:"duration" yaml:"duration"`

	// CheckedPaths lists paths that were checked during detection.
	CheckedPaths []string `json:"checkedPaths,omitempty" yaml:"checkedPaths,omitempty"`
}

// Success returns true if detection succeeded.
func (r *DetectionResult) Success() bool {
	return r.Error == nil && r.Workspace != nil
}

// DetectionOptions configures workspace detection behavior.
type DetectionOptions struct {
	// MaxDepth limits how deep to search for workspace markers.
	MaxDepth int `json:"maxDepth" yaml:"maxDepth"`

	// IncludePackages determines whether to discover packages.
	IncludePackages bool `json:"includePackages" yaml:"includePackages"`

	// PreferredTypes limits detection to specific workspace types.
	PreferredTypes []WorkspaceType `json:"preferredTypes,omitempty" yaml:"preferredTypes,omitempty"`

	// CustomMarkers allows defining custom marker files for detection.
	CustomMarkers map[string]WorkspaceType `json:"customMarkers,omitempty" yaml:"customMarkers,omitempty"`

	// FollowSymlinks determines whether to follow symbolic links.
	FollowSymlinks bool `json:"followSymlinks" yaml:"followSymlinks"`
}

// DefaultDetectionOptions returns sensible default detection options.
func DefaultDetectionOptions() DetectionOptions {
	return DetectionOptions{
		MaxDepth:        3,
		IncludePackages: true,
		FollowSymlinks:  false,
	}
}
