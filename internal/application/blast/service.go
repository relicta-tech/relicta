// Package blast provides blast radius analysis for monorepos.
package blast

import (
	"context"
)

// Service defines the interface for blast radius analysis.
type Service interface {
	// DiscoverPackages discovers all packages in the repository.
	DiscoverPackages(ctx context.Context, opts *AnalysisOptions) ([]*Package, error)

	// AnalyzeBlastRadius performs a complete blast radius analysis.
	AnalyzeBlastRadius(ctx context.Context, opts *AnalysisOptions) (*BlastRadius, error)

	// GetChangedFiles returns files changed between two refs.
	GetChangedFiles(ctx context.Context, fromRef, toRef string) ([]ChangedFile, error)

	// GetImpactedPackages returns packages impacted by changes.
	GetImpactedPackages(ctx context.Context, changedFiles []ChangedFile, packages []*Package) ([]*Impact, error)

	// BuildDependencyGraph builds the dependency graph for packages.
	BuildDependencyGraph(ctx context.Context, packages []*Package) (*DependencyGraph, error)

	// CalculateRiskScore calculates the risk score for an impact.
	CalculateRiskScore(impact *Impact) int

	// SuggestReleaseType suggests a release type based on changes.
	SuggestReleaseType(impact *Impact) string
}

// ServiceConfig configures the blast radius service.
type ServiceConfig struct {
	// RepoPath is the path to the repository.
	RepoPath string
	// MonorepoConfig is the monorepo-specific configuration.
	MonorepoConfig *MonorepoConfig
}

// DefaultServiceConfig returns the default service configuration.
func DefaultServiceConfig() *ServiceConfig {
	return &ServiceConfig{
		RepoPath:       ".",
		MonorepoConfig: DefaultMonorepoConfig(),
	}
}

// ServiceOption configures the blast radius service.
type ServiceOption func(*ServiceConfig)

// WithRepoPath sets the repository path.
func WithRepoPath(path string) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.RepoPath = path
	}
}

// WithMonorepoConfig sets the monorepo configuration.
func WithMonorepoConfig(config *MonorepoConfig) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.MonorepoConfig = config
	}
}
