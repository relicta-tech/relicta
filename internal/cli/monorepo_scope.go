package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	appmonorepo "github.com/relicta-tech/relicta/v4/internal/application/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/container"
)

// The release scope.
//
// A release run is keyed by the directory it is about. In an ordinary repository that is the
// repository root, and there is one run at a time. In an independent monorepo each package is
// released on its own commits, so each package has its own run — its own version, its own
// governance decision, its own audit entry — and the directory that keys it is the package's.
//
// Everything downstream already takes a root: PlanReleaseInput.RepoRoot, ApproveReleaseInput.
// RepoRoot, InitReleaseServices. So per-package governance is not a second release pipeline; it
// is the same one, asked about a different directory.

// releaseScope names one package to act on, set by --package.
var releaseScope string

// scopePath resolves --package to the directory whose run the command should act on.
//
// Returns the repository root when no package is named, which is what every non-monorepo
// command gets and what a monorepo command that is genuinely about the whole repository gets.
func scopePath(ctx context.Context, app cliApp) (string, error) {
	repoInfo, err := app.GitAdapter().GetInfo(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get repository info: %w", err)
	}
	if releaseScope == "" {
		return repoInfo.Path, nil
	}
	if cfg == nil || !cfg.Monorepo.Enabled {
		return "", fmt.Errorf("--package applies to a monorepo; this repository has no " +
			"monorepo section enabled, so it has one release rather than one per package")
	}

	packages, err := discoveredPackages(ctx, app, repoInfo.Path)
	if err != nil {
		return "", err
	}

	// By name or by path, because both are how somebody refers to a package: the table
	// `relicta bump` prints uses paths, and the manifest that names it uses names.
	for _, pkg := range packages {
		if pkg.Name == releaseScope || pkg.RelPath == releaseScope ||
			filepath.ToSlash(pkg.RelPath) == filepath.ToSlash(releaseScope) {
			return filepath.Join(repoInfo.Path, pkg.RelPath), nil
		}
	}

	known := make([]string, 0, len(packages))
	for _, pkg := range packages {
		known = append(known, pkg.Name)
	}
	return "", fmt.Errorf("no package named %q; this repository has %s",
		releaseScope, strings.Join(known, ", "))
}

// discoveredPackages lists the packages the configuration's globs match.
func discoveredPackages(ctx context.Context, app cliApp, repoRoot string) ([]appmonorepo.PackageTag, error) {
	packages, err := container.NewMonorepoBumper(app.GitAdapter(), nil).ReleaseTags(ctx,
		appmonorepo.PlanInput{
			RepoRoot:     repoRoot,
			PackagePaths: cfg.Monorepo.PackagePaths,
			ExcludePaths: cfg.Monorepo.ExcludePaths,
			TagPrefixes:  monorepoTagPrefixes(),
		})
	if err != nil {
		return nil, fmt.Errorf("failed to discover packages: %w", err)
	}
	if len(packages) == 0 {
		return nil, fmt.Errorf("no packages matched monorepo.package_paths %v",
			cfg.Monorepo.PackagePaths)
	}
	return packages, nil
}

// perPackageGovernance reports whether decisions are made per package in this repository.
func perPackageGovernance() bool {
	return cfg != nil && cfg.Monorepo.Enabled
}
