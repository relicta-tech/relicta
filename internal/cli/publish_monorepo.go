package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	appmonorepo "github.com/relicta-tech/relicta/v4/internal/application/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/container"
	"github.com/relicta-tech/relicta/v4/internal/domain/communication"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// Per-package changelogs.
//
// The repository's CHANGELOG.md describes the release as a whole and is written from the
// release notes, which are generated once. A package's changelog describes that package, and is
// rendered from that package's own commits — the same analysis that decided its version, so the
// heading and the tag cannot disagree.
//
// No AI call per package. Rendering from conventional commits is deterministic and free, and a
// per-package changelog that required an API key would make monorepo releases fail for everyone
// without one.

// packageChangelogFile is where a package's changelog lives.
//
// `monorepo.package_overrides.<path>.changelog_file` names it; otherwise CHANGELOG.md beside the
// package's manifest, which is where a reader of that package looks.
func packageChangelogFile(repoRoot, relPath string) string {
	if override, ok := cfg.Monorepo.PackageOverrides[relPath]; ok && override.ChangelogFile != "" {
		if filepath.IsAbs(override.ChangelogFile) {
			return override.ChangelogFile
		}
		return filepath.Join(repoRoot, relPath, override.ChangelogFile)
	}
	return filepath.Join(repoRoot, relPath, "CHANGELOG.md")
}

// writePackageChangelogs renders one changelog entry per changed package.
//
// Returns the files it wrote so the release commit can carry them: a changelog committed after
// the tag describes a release the tag does not contain.
func writePackageChangelogs(ctx context.Context, app cliApp, repoRoot string) []string {
	if cfg == nil || !cfg.Monorepo.Enabled || !cfg.Monorepo.Changelog.PerPackage {
		return nil
	}

	bumper := container.NewMonorepoBumper(app.GitAdapter(), nil)
	plan, err := bumper.Plan(ctx, appmonorepo.PlanInput{
		RepoRoot:     repoRoot,
		PackagePaths: cfg.Monorepo.PackagePaths,
		ExcludePaths: cfg.Monorepo.ExcludePaths,
		TagPrefixes:  monorepoTagPrefixes(),
		Skip:         monorepoSkipped(),
		FromRef:      lastReleaseTag(ctx, app),
	})
	if err != nil {
		printWarning(fmt.Sprintf("Could not render per-package changelogs: %v", err))
		return nil
	}

	// The heading version is the one the package is being tagged at — the version its manifest
	// claims right now — not the plan's Next.
	//
	// By publish time bump has already written the manifests, so a plan computed here reports
	// the version *after* this release: the changelog said 1.6.0 above a release tagged
	// api-v1.5.0. The plan is still what carries each package's commits, so the two are joined
	// rather than one replacing the other.
	tags, err := bumper.ReleaseTags(ctx, appmonorepo.PlanInput{
		RepoRoot:     repoRoot,
		PackagePaths: cfg.Monorepo.PackagePaths,
		ExcludePaths: cfg.Monorepo.ExcludePaths,
		TagPrefixes:  monorepoTagPrefixes(),
		Skip:         monorepoSkipped(),
	})
	if err != nil {
		printWarning(fmt.Sprintf("Could not read the package versions to render changelogs: %v", err))
		return nil
	}
	releasing := make(map[string]version.SemanticVersion, len(tags))
	for _, tag := range tags {
		releasing[tag.RelPath] = tag.Version
	}

	opts := container.ChangelogRenderOptions(cfg)
	written := make([]string, 0, len(plan.Packages))

	for _, pkg := range plan.Packages {
		if pkg.Changes == nil {
			continue
		}

		relPath := displayPath(pkg.Path, repoRoot)
		ver, ok := releasing[relPath]
		if !ok {
			continue
		}

		// A held package gets no entry, for the same reason it gets no tag: the entry would
		// describe a release that did not happen, in the file its readers trust to say what
		// shipped.
		if !packageIsShipping(ctx, app, repoRoot, relPath) {
			continue
		}

		entry := communication.BuildEntry(ver, pkg.Changes, opts)
		sections := communication.RenderSections(entry, opts)
		if sections == "" {
			// Every commit that touched the package was excluded by changelog.exclude. An
			// empty section list under a version heading says less than no entry at all.
			continue
		}

		body := communication.RenderVersionHeading(entry) + "\n\n" + sections
		file := packageChangelogFile(repoRoot, displayPath(pkg.Path, repoRoot))

		if changelogAlreadyContains(file, body) {
			continue
		}
		if err := updateChangelogFile(file, body); err != nil {
			printWarning(fmt.Sprintf("Failed to update %s: %v", displayPath(file, repoRoot), err))
			continue
		}
		written = append(written, file)
	}

	if len(written) > 0 {
		printSuccess(fmt.Sprintf("Updated %d package changelog%s", len(written), plural(len(written))))
	}
	return written
}

// packageIsShipping reports whether this package's own decision allows it to be released.
//
// A package with no run of its own is shipping: per-package runs postdate per-package tagging,
// and a repository that has never planned one still has manifests to release.
func packageIsShipping(ctx context.Context, app cliApp, repoRoot, relPath string) bool {
	repo := app.ReleaseRepository()
	if repo == nil {
		return true
	}
	run, err := repo.FindLatest(ctx, filepath.Join(repoRoot, relPath))
	if err != nil || run == nil {
		return true
	}
	return packageWillShip(run.State())
}

// packageChangelogPaths lists where per-package changelogs live, whether or not this release
// writes one.
//
// The release commit and the clean-tree gate both work from a path list rather than from what
// was written, so the list has to be answerable before the writing happens.
func packageChangelogPaths(ctx context.Context) []string {
	if cfg == nil || !cfg.Monorepo.Enabled || !cfg.Monorepo.Changelog.PerPackage {
		return nil
	}

	root, err := os.Getwd()
	if err != nil {
		return nil
	}

	tags, err := container.NewMonorepoBumper(nil, nil).ReleaseTags(ctx, appmonorepo.PlanInput{
		RepoRoot:     root,
		PackagePaths: cfg.Monorepo.PackagePaths,
		ExcludePaths: cfg.Monorepo.ExcludePaths,
	})
	if err != nil {
		return nil
	}

	paths := make([]string, 0, len(tags))
	for _, pkg := range tags {
		file := packageChangelogFile(root, pkg.RelPath)
		if rel, relErr := filepath.Rel(root, file); relErr == nil {
			paths = append(paths, rel)
		}
	}
	return paths
}
