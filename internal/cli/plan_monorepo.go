package cli

import (
	"context"
	"fmt"

	appmonorepo "github.com/relicta-tech/relicta/v4/internal/application/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/container"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/monorepo"
	releaseapp "github.com/relicta-tech/relicta/v4/internal/domain/release/app"
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
			RepoID:   repoInfo.RemoteURL,
			BaseRef:  pkg.BaseRef,
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
