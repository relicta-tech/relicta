package cli

import (
	"context"
	"fmt"
	"path/filepath"

	appmonorepo "github.com/relicta-tech/relicta/v4/internal/application/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/container"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/monorepo"
	releaseapp "github.com/relicta-tech/relicta/v4/internal/domain/release/app"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
)

// One release run per package.
//
// A package released on its own commits carries its own decision: its own risk, its own
// approval, its own audit entry. The run is keyed by the package's directory, which is what
// PlanReleaseInput.RepoRoot already means — so this is the existing pipeline asked about each
// package in turn, not a second one.
//
// The repository's own run is still planned alongside. It is what the repository-wide commands
// act on and what the release marker is measured from; a monorepo with none would have every
// one of them counting from the start of history forever.

// plannedPackage is one package's run, for reporting.
type plannedPackage struct {
	Name    string
	RelPath string
	Version string
	RunID   string
	Existed bool
}

// planPackageRuns plans a release run for every package with changes.
func planPackageRuns(ctx context.Context, app cliApp, repoInfo *sourcecontrol.RepositoryInfo) ([]plannedPackage, error) {
	bumper := container.NewMonorepoBumper(app.GitAdapter(), nil)

	plan, err := bumper.Plan(ctx, appmonorepo.PlanInput{
		RepoRoot:     repoInfo.Path,
		PackagePaths: cfg.Monorepo.PackagePaths,
		ExcludePaths: cfg.Monorepo.ExcludePaths,
		TagPrefixes:  monorepoTagPrefixes(),
		FromRef:      lastReleaseTag(ctx, app),
	})
	if err != nil {
		return nil, err
	}

	planned := make([]plannedPackage, 0, len(plan.Packages))
	for _, pkg := range plan.Packages {
		// Services are rebuilt per package because they are built around one root. The cost
		// is a few adapters per package; the alternative is a run written to the repository's
		// store under a package's name, which is the kind of near-miss that reads as working.
		if err := app.InitReleaseServices(ctx, pkg.Path); err != nil {
			return nil, fmt.Errorf("failed to initialize release services for %s: %w", pkg.Name, err)
		}
		services := app.ReleaseServices()
		if services == nil || services.PlanRelease == nil {
			return nil, fmt.Errorf("PlanRelease use case not available")
		}

		bumpKind := convertReleaseTypeToBumpKind(releaseTypeFor(pkg.Bump))
		current, next := pkg.Current, pkg.Next

		out, err := services.PlanRelease.Execute(ctx, releaseapp.PlanReleaseInput{
			RepoRoot: pkg.Path,
			// The releasable unit, not just the repository: a run's ID is derived from its
			// plan hash, which covers repoID, base, head, commits and version — and for two
			// packages of one repository every one of those could match. All three runs then
			// shared an ID, so the decisions were not distinguishable from each other.
			//
			// Carried in repoID rather than in a new persisted field so that repositories
			// that are not monorepos keep the exact IDs they have. Including repoRoot in the
			// hash unconditionally would give every existing repository new IDs, and the next
			// `plan` — which supersedes runs whose hash no longer matches — would cancel an
			// in-flight approved release on upgrade.
			RepoID:  packageRepoID(repoInfo, displayPath(pkg.Path, repoInfo.Path)),
			Commits: commitSHAsOf(pkg),
			BaseRef: pkg.BaseRef,
			Actor: ports.ActorInfo{
				Type: "user",
				ID:   actorID,
			},
			Force:           true,
			DiscardExisting: planForce,
			ChangeSet:       pkg.Changes,
			CurrentVersion:  &current,
			NextVersion:     &next,
			BumpKind:        &bumpKind,
			// The package's own prefix, so the run names the tag publish will create.
			TagPrefix:  appmonorepo.TagPrefixFor(displayPath(pkg.Path, repoInfo.Path), monorepoTagPrefixes()),
			Confidence: 1.0,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", pkg.Name, err)
		}

		planned = append(planned, plannedPackage{
			Name:    pkg.Name,
			RelPath: displayPath(pkg.Path, repoInfo.Path),
			Version: next.String(),
			RunID:   string(out.RunID),
			Existed: out.AlreadyExisted,
		})
	}

	return planned, nil
}

// packageRepoID identifies the releasable unit: this repository, this package.
func packageRepoID(repoInfo *sourcecontrol.RepositoryInfo, relPath string) string {
	base := repoInfo.RemoteURL
	if base == "" {
		base = repoInfo.Path
	}
	return base + "#" + filepath.ToSlash(relPath)
}

// commitSHAsOf lists the commits that touched the package, which are the ones its run is
// about — a subset of the range between the base ref and HEAD.
func commitSHAsOf(pkg appmonorepo.PackageBump) []domain.CommitSHA {
	if pkg.Changes == nil {
		return nil
	}
	commits := pkg.Changes.Commits()
	shas := make([]domain.CommitSHA, 0, len(commits))
	for _, commit := range commits {
		if commit == nil {
			continue
		}
		shas = append(shas, domain.CommitSHA(commit.Hash()))
	}
	return shas
}

// releaseTypeFor maps a package's bump to the release type the run records.
func releaseTypeFor(bump monorepo.BumpType) changes.ReleaseType {
	switch bump {
	case monorepo.BumpTypeMajor:
		return changes.ReleaseTypeMajor
	case monorepo.BumpTypeMinor:
		return changes.ReleaseTypeMinor
	case monorepo.BumpTypePatch:
		return changes.ReleaseTypePatch
	default:
		return changes.ReleaseTypeNone
	}
}

// printPlannedPackages reports what each package will release, and how to decide on it.
func printPlannedPackages(planned []plannedPackage) {
	if len(planned) == 0 {
		return
	}

	fmt.Println()
	printSubtitle(fmt.Sprintf("Package releases (%d)", len(planned)))
	fmt.Println()
	for _, pkg := range planned {
		fmt.Printf("  %-24s %-10s %s\n", pkg.RelPath, pkg.Version, shortenID(pkg.RunID))
	}
	fmt.Println()
	printInfo("Each package carries its own decision — approve one with " +
		"`relicta approve --package <name>`")
}

// bumpPackageRuns moves each package's run to its own version.
//
// The manifests are already written by the time this runs; this is the release record catching
// up with them. Without it a package's run stays in `planned` and `relicta approve --package`
// refuses with "run 'relicta bump' first" to somebody who just did.
func bumpPackageRuns(ctx context.Context, app cliApp, repoRoot string, packages []appmonorepo.PackageBump) {
	for _, pkg := range packages {
		if err := app.InitReleaseServices(ctx, pkg.Path); err != nil {
			printWarning(fmt.Sprintf("%s: release services unavailable: %v", pkg.Name, err))
			continue
		}
		services := app.ReleaseServices()
		if services == nil || services.BumpVersion == nil {
			continue
		}

		next := pkg.Next
		_, err := services.BumpVersion.Execute(ctx, releaseapp.BumpVersionInput{
			RepoRoot: pkg.Path,
			Actor: ports.ActorInfo{
				Type: "user",
				ID:   actorID,
			},
			Force:           true,
			OverrideVersion: &next,
			// The package's own tag, so the run names what publish will create rather than
			// the repository-wide `v1.5.0` that belongs to a different release.
			OverrideTagName: pkg.Tag,
		})
		if err != nil {
			printWarning(fmt.Sprintf("%s: release run not versioned: %v", pkg.Name, err))
		}
	}

	// Point the services back at the repository, since the caller's next step is about it.
	if err := app.InitReleaseServices(ctx, repoRoot); err != nil {
		printWarning(fmt.Sprintf("release services not restored: %v", err))
	}
}

// generatePackageNotes writes each package's release notes into its own run.
//
// The notes come from that package's changeset, which its run already carries, so a package's
// notes describe its own commits. Approval reads them, and a package whose run has none cannot
// be approved.
func generatePackageNotes(ctx context.Context, app cliApp, repoRoot string) {
	if !perPackageGovernance() {
		return
	}

	packages, err := discoveredPackages(ctx, app, repoRoot)
	if err != nil {
		return
	}

	for _, pkg := range packages {
		scope := filepath.Join(repoRoot, pkg.RelPath)
		if err := app.InitReleaseServices(ctx, scope); err != nil {
			continue
		}
		services := app.ReleaseServices()
		if services == nil || services.GenerateNotes == nil {
			continue
		}

		if _, err := services.GenerateNotes.Execute(ctx, releaseapp.GenerateNotesInput{
			RepoRoot: scope,
			Actor: ports.ActorInfo{
				Type: "user",
				ID:   actorID,
			},
			Force: true,
		}); err != nil {
			// A package with no run of its own is not an error here: only packages with
			// changes were planned, and the rest have nothing to describe.
			continue
		}
	}

	if err := app.InitReleaseServices(ctx, repoRoot); err != nil {
		printWarning(fmt.Sprintf("release services not restored: %v", err))
	}
}
