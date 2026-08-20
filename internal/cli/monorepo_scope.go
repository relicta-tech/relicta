package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	appmonorepo "github.com/relicta-tech/relicta/v4/internal/application/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/container"
	"github.com/relicta-tech/relicta/v4/internal/domain/release"
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

// printPackageDecisions reports where each package stands.
//
// A monorepo release has two levels: the release itself, which `relicta approve` decides, and
// each package in it, which `relicta approve --package <name>` decides. Publish tags only the
// packages that were decided, so a package nobody approved simply does not ship — and that has
// to be visible at the moment of deciding, not discovered afterwards from the tag list.
func printPackageDecisions(ctx context.Context, app cliApp) {
	if !perPackageGovernance() || outputJSON {
		return
	}

	repoInfo, err := app.GitAdapter().GetInfo(ctx)
	if err != nil {
		return
	}
	packages, err := discoveredPackages(ctx, app, repoInfo.Path)
	if err != nil {
		return
	}

	repo := app.ReleaseRepository()
	if repo == nil {
		return
	}

	type row struct{ path, version, state string }
	rows := make([]row, 0, len(packages))
	held := 0
	for _, pkg := range packages {
		run, err := repo.FindLatest(ctx, filepath.Join(repoInfo.Path, pkg.RelPath))
		if err != nil || run == nil {
			continue
		}
		state := string(run.State())
		if !packageWillShip(run.State()) {
			state += " — held, will not be tagged"
			held++
		}
		rows = append(rows, row{pkg.RelPath, run.VersionNext().String(), state})
	}
	if len(rows) == 0 {
		return
	}

	fmt.Println()
	printSubtitle("Package decisions")
	fmt.Println()
	for _, r := range rows {
		fmt.Printf("  %-24s %-10s %s\n", r.path, r.version, r.state)
	}
	if held > 0 {
		fmt.Println()
		printInfo("Decide a held package with `relicta approve --package <name>`")
	}
}

// packageWillShip reports whether a package's run has been decided in favor of releasing it.
func packageWillShip(state release.RunState) bool {
	switch state {
	case release.StateApproved, release.StatePublishing, release.StatePublished:
		return true
	default:
		return false
	}
}
