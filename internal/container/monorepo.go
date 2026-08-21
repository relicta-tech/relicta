package container

import (
	"context"
	"path/filepath"

	analysisfactory "github.com/relicta-tech/relicta/v4/internal/analysis/factory"
	appmonorepo "github.com/relicta-tech/relicta/v4/internal/application/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/config"
	domainrelease "github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/ai"
	infraworkspace "github.com/relicta-tech/relicta/v4/internal/infrastructure/workspace"
)

// NewMonorepoBumper builds the per-package versioning service `relicta bump` uses when
// monorepo.enabled is set.
//
// It lives here for the same reason NewMultirepoCoordinator does: the composition root owns
// the infrastructure choice. Package discovery is a filesystem walk — internal/infrastructure/
// workspace — and the CLI may not import it, which the hexagonal fitness function in
// internal/architecture enforces.
//
// aiService may be nil. The analysis factory then yields an analyzer that classifies commits
// by their conventional-commit prefix alone, which is what an unconfigured repository already
// gets everywhere else; per-package versioning must not require an API key.
func NewMonorepoBumper(gitRepo sourcecontrol.GitRepository, aiService ai.Service) *appmonorepo.BumpService {
	analyzer := appmonorepo.NewMonorepoAnalyzer(
		gitRepo,
		&version.DefaultVersionCalculator{},
		analysisfactory.NewFactory(aiService),
		appmonorepo.NewCompositeVersionWriter(),
	)
	return appmonorepo.NewBumpService(infraworkspace.NewFileDetector(), analyzer, gitRepo)
}

// packageTagResolver answers what a monorepo release should tag, or nil in a repository that
// is not one.
//
// Deferred rather than computed here: `relicta release` bumps and publishes in one command, so
// the manifests this reads are written after the container is built. Resolving at construction
// would tag every package at the version it had before the release.
func (c *App) packageTagResolver(repoRoot string) func(context.Context) ([]appmonorepo.PackageTag, error) {
	if !c.config.Monorepo.Enabled || c.config.Monorepo.Strategy != config.MonorepoStrategyIndependent {
		return nil
	}

	prefixes := make(map[string]string, len(c.config.Monorepo.PackageOverrides))
	skip := make(map[string]bool, len(c.config.Monorepo.PackageOverrides))
	for path, override := range c.config.Monorepo.PackageOverrides {
		if override.TagPrefix != "" {
			prefixes[path] = override.TagPrefix
		}
		if override.SkipVersioning {
			skip[path] = true
		}
	}

	return func(ctx context.Context) ([]appmonorepo.PackageTag, error) {
		tags, err := NewMonorepoBumper(c.gitAdapter, nil).ReleaseTags(ctx, appmonorepo.PlanInput{
			RepoRoot:     repoRoot,
			PackagePaths: c.config.Monorepo.PackagePaths,
			ExcludePaths: c.config.Monorepo.ExcludePaths,
			TagPrefixes:  prefixes,
			Skip:         skip,
		})
		if err != nil {
			return nil, err
		}
		return c.approvedOnly(ctx, repoRoot, tags), nil
	}
}

// approvedOnly keeps the packages whose own release run has been approved.
//
// This is what makes a decision per package mean anything: a package held back at
// `relicta approve --package` must not be tagged by a publish that approved a different one.
// A tag is the release, so tagging an unapproved package would publish it.
//
// A package with no run of its own is kept. Tagging predates per-package runs, a repository
// that has never planned per package still has manifests to tag, and refusing those would make
// this an upgrade that silently stops tagging.
func (c *App) approvedOnly(ctx context.Context, repoRoot string, tags []appmonorepo.PackageTag) []appmonorepo.PackageTag {
	repo := c.ReleaseRepository()
	if repo == nil {
		return tags
	}

	kept := make([]appmonorepo.PackageTag, 0, len(tags))
	for _, pkg := range tags {
		run, err := repo.FindLatest(ctx, filepath.Join(repoRoot, pkg.RelPath))
		if err != nil || run == nil {
			kept = append(kept, pkg)
			continue
		}
		switch run.State() {
		case domainrelease.StateApproved, domainrelease.StatePublishing, domainrelease.StatePublished:
			kept = append(kept, pkg)
		default:
			// Planned, versioned, notes-ready, rejected or canceled: a decision that has
			// not been made is not a decision to release.
		}
	}
	return kept
}
