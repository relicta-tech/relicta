// Package blast provides blast radius analysis for monorepos.
// It analyzes which parts of the codebase are affected by changes,
// helping identify impacted packages, modules, and downstream dependencies.
package blast

import (
	"time"
)

// PackageType represents the type of package/module in a monorepo.
type PackageType string

const (
	// PackageTypeGoModule is a Go module.
	PackageTypeGoModule PackageType = "go_module"
	// PackageTypeNPM is an npm package.
	PackageTypeNPM PackageType = "npm"
	// PackageTypePython is a Python package.
	PackageTypePython PackageType = "python"
	// PackageTypeCargo is a Rust Cargo package.
	PackageTypeCargo PackageType = "cargo"
	// PackageTypeGradle is a Gradle project.
	PackageTypeGradle PackageType = "gradle"
	// PackageTypeMaven is a Maven project.
	PackageTypeMaven PackageType = "maven"
	// PackageTypeDirectory is a generic directory-based package.
	PackageTypeDirectory PackageType = "directory"
	// PackageTypeUnknown is an unknown package type.
	PackageTypeUnknown PackageType = "unknown"
)

// ImpactLevel represents how severely a package is affected by changes.
type ImpactLevel string

const (
	// ImpactLevelDirect means the package itself was directly changed.
	ImpactLevelDirect ImpactLevel = "direct"
	// ImpactLevelTransitive means the package depends on something that changed.
	ImpactLevelTransitive ImpactLevel = "transitive"
	// ImpactLevelNone means the package is not affected.
	ImpactLevelNone ImpactLevel = "none"
)

// Package represents a package/module in the monorepo.
type Package struct {
	// Name is the package name.
	Name string `json:"name"`
	// Path is the relative path from the repository root.
	Path string `json:"path"`
	// Type is the package type (go_module, npm, etc.).
	Type PackageType `json:"type"`
	// Version is the current version (if available).
	Version string `json:"version,omitempty"`
	// Dependencies is the list of packages this package depends on.
	Dependencies []string `json:"dependencies,omitempty"`
	// DevDependencies is the list of development dependencies.
	DevDependencies []string `json:"dev_dependencies,omitempty"`
	// Metadata contains type-specific metadata.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Impact represents the impact of changes on a specific package.
type Impact struct {
	// Package is the affected package.
	Package *Package `json:"package"`
	// Level is the impact level.
	Level ImpactLevel `json:"level"`
	// DirectChanges lists files directly changed in this package.
	DirectChanges []ChangedFile `json:"direct_changes,omitempty"`
	// AffectedDependencies lists dependencies that were changed.
	AffectedDependencies []string `json:"affected_dependencies,omitempty"`
	// TransitiveDepth is the depth in the dependency graph (0 = direct).
	TransitiveDepth int `json:"transitive_depth"`
	// RiskScore is a calculated risk score (0-100).
	RiskScore int `json:"risk_score"`
	// SuggestedActions lists recommended actions for this package.
	SuggestedActions []string `json:"suggested_actions,omitempty"`
	// RequiresRelease indicates if this package needs a new release.
	RequiresRelease bool `json:"requires_release"`
	// ReleaseType is the suggested release type (if release is needed).
	ReleaseType string `json:"release_type,omitempty"`
}

// ChangedFile represents a file that was changed.
type ChangedFile struct {
	// Path is the file path relative to repo root.
	Path string `json:"path"`
	// Status is the change status (added, modified, deleted, renamed).
	Status string `json:"status"`
	// OldPath is the old path (for renamed files).
	OldPath string `json:"old_path,omitempty"`
	// Insertions is the number of lines added.
	Insertions int `json:"insertions"`
	// Deletions is the number of lines removed.
	Deletions int `json:"deletions"`
	// IsBinary indicates if this is a binary file.
	IsBinary bool `json:"is_binary"`
	// Category categorizes the file (source, test, config, docs, etc.).
	Category FileCategory `json:"category"`
}

// FileCategory categorizes types of files.
type FileCategory string

const (
	// FileCategorySource is production source code.
	FileCategorySource FileCategory = "source"
	// FileCategoryTest is test code.
	FileCategoryTest FileCategory = "test"
	// FileCategoryConfig is configuration files.
	FileCategoryConfig FileCategory = "config"
	// FileCategoryDocs is documentation files.
	FileCategoryDocs FileCategory = "docs"
	// FileCategoryBuild is build-related files.
	FileCategoryBuild FileCategory = "build"
	// FileCategoryCI is CI/CD related files.
	FileCategoryCI FileCategory = "ci"
	// FileCategoryDependency is dependency manifest files.
	FileCategoryDependency FileCategory = "dependency"
	// FileCategoryAsset is static assets.
	FileCategoryAsset FileCategory = "asset"
	// FileCategoryGenerated is generated code.
	FileCategoryGenerated FileCategory = "generated"
	// FileCategoryOther is any other file type.
	FileCategoryOther FileCategory = "other"
)

// BlastRadius contains the complete blast radius analysis results.
type BlastRadius struct {
	// Packages is the list of all packages in the repository.
	Packages []*Package `json:"packages"`
	// Impacts lists the impact on each affected package.
	Impacts []*Impact `json:"impacts"`
	// ChangedFiles lists all changed files.
	ChangedFiles []ChangedFile `json:"changed_files"`
	// Summary provides a high-level summary.
	Summary *Summary `json:"summary"`
	// DependencyGraph is the dependency graph for visualization.
	DependencyGraph *DependencyGraph `json:"dependency_graph,omitempty"`
	// AnalyzedAt is when the analysis was performed.
	AnalyzedAt time.Time `json:"analyzed_at"`
	// FromRef is the starting reference for the analysis.
	FromRef string `json:"from_ref"`
	// ToRef is the ending reference for the analysis.
	ToRef string `json:"to_ref"`
}

// Summary provides a high-level summary of the blast radius.
type Summary struct {
	// TotalPackages is the total number of packages.
	TotalPackages int `json:"total_packages"`
	// DirectlyAffected is the number of directly affected packages.
	DirectlyAffected int `json:"directly_affected"`
	// TransitivelyAffected is the number of transitively affected packages.
	TransitivelyAffected int `json:"transitively_affected"`
	// TotalAffected is the total number of affected packages.
	TotalAffected int `json:"total_affected"`
	// HighRiskCount is the number of high-risk impacts.
	HighRiskCount int `json:"high_risk_count"`
	// PackagesRequiringRelease is the count of packages needing a release.
	PackagesRequiringRelease int `json:"packages_requiring_release"`
	// TotalFilesChanged is the total number of changed files.
	TotalFilesChanged int `json:"total_files_changed"`
	// TotalInsertions is the total lines added.
	TotalInsertions int `json:"total_insertions"`
	// TotalDeletions is the total lines removed.
	TotalDeletions int `json:"total_deletions"`
	// ChangesByCategory breaks down changes by file category.
	ChangesByCategory map[FileCategory]int `json:"changes_by_category"`
	// AffectedByType breaks down affected packages by type.
	AffectedByType map[PackageType]int `json:"affected_by_type"`
	// RiskLevel is the overall risk level of the changes.
	RiskLevel RiskLevel `json:"risk_level"`
	// RiskFactors lists the main risk factors identified.
	RiskFactors []string `json:"risk_factors,omitempty"`
}

// RiskLevel represents the overall risk level.
type RiskLevel string

const (
	// RiskLevelLow indicates low risk changes.
	RiskLevelLow RiskLevel = "low"
	// RiskLevelMedium indicates medium risk changes.
	RiskLevelMedium RiskLevel = "medium"
	// RiskLevelHigh indicates high risk changes.
	RiskLevelHigh RiskLevel = "high"
	// RiskLevelCritical indicates critical risk changes.
	RiskLevelCritical RiskLevel = "critical"
)

// DependencyGraph represents the package dependency graph.
type DependencyGraph struct {
	// Nodes are the packages in the graph.
	Nodes []GraphNode `json:"nodes"`
	// Edges are the dependency relationships.
	Edges []GraphEdge `json:"edges"`
}

// GraphNode represents a node in the dependency graph.
type GraphNode struct {
	// ID is the unique node ID (usually package path).
	ID string `json:"id"`
	// Label is the display label.
	Label string `json:"label"`
	// Type is the package type.
	Type PackageType `json:"type"`
	// Affected indicates if this node is affected by changes.
	Affected bool `json:"affected"`
	// ImpactLevel is the impact level if affected.
	ImpactLevel ImpactLevel `json:"impact_level,omitempty"`
}

// GraphEdge represents an edge in the dependency graph.
type GraphEdge struct {
	// Source is the source node ID.
	Source string `json:"source"`
	// Target is the target node ID.
	Target string `json:"target"`
	// Type describes the dependency type.
	Type string `json:"type"`
}

// MonorepoConfig configures monorepo-specific settings.
type MonorepoConfig struct {
	// PackagePaths is a list of glob patterns for package locations.
	// Examples: "packages/*", "services/*", "libs/**"
	PackagePaths []string `json:"package_paths,omitempty"`
	// ExcludePaths is a list of paths to exclude from analysis.
	ExcludePaths []string `json:"exclude_paths,omitempty"`
	// SharedDirs lists directories containing shared code.
	SharedDirs []string `json:"shared_dirs,omitempty"`
	// RootPackage indicates if the root directory is also a package.
	RootPackage bool `json:"root_package"`
	// CustomPatterns maps file patterns to categories.
	CustomPatterns map[string]FileCategory `json:"custom_patterns,omitempty"`
	// IgnoreDevDependencies excludes dev dependencies from analysis.
	IgnoreDevDependencies bool `json:"ignore_dev_dependencies"`
	// MaxTransitiveDepth limits transitive dependency depth (0 = unlimited).
	MaxTransitiveDepth int `json:"max_transitive_depth"`
}

// DefaultMonorepoConfig returns sensible defaults for monorepo analysis.
func DefaultMonorepoConfig() *MonorepoConfig {
	return &MonorepoConfig{
		PackagePaths: []string{
			"packages/*",
			"plugins/*",
			"services/*",
			"libs/*",
			"apps/*",
			"modules/*",
		},
		ExcludePaths: []string{
			"node_modules",
			"vendor",
			".git",
			"dist",
			"build",
			"coverage",
			".next",
			"__pycache__",
			".pytest_cache",
			"target",
		},
		SharedDirs: []string{
			"shared",
			"common",
			"core",
			"internal",
			"pkg",
		},
		RootPackage:           false,
		IgnoreDevDependencies: true,
		MaxTransitiveDepth:    0, // unlimited
	}
}

// AnalysisOptions configures the blast radius analysis.
type AnalysisOptions struct {
	// FromRef is the starting reference (tag, commit, branch).
	FromRef string
	// ToRef is the ending reference (default: HEAD).
	ToRef string
	// IncludeTransitive includes transitive dependency impacts.
	IncludeTransitive bool
	// CalculateRisk calculates risk scores.
	CalculateRisk bool
	// GenerateGraph generates the dependency graph.
	GenerateGraph bool
	// IncludeTests includes test files in the analysis.
	IncludeTests bool
	// IncludeDocs includes documentation files in the analysis.
	IncludeDocs bool
	// MonorepoConfig is the monorepo-specific configuration.
	MonorepoConfig *MonorepoConfig
}

// DefaultAnalysisOptions returns the default analysis options.
func DefaultAnalysisOptions() *AnalysisOptions {
	return &AnalysisOptions{
		ToRef:             "HEAD",
		IncludeTransitive: true,
		CalculateRisk:     true,
		GenerateGraph:     false,
		IncludeTests:      false,
		IncludeDocs:       false,
		MonorepoConfig:    DefaultMonorepoConfig(),
	}
}
