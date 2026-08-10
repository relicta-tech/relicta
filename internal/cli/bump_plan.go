package cli

// bump_plan.go: makes `relicta bump` apply the version `relicta plan` decided,
// rather than deriving its own answer to the same question.

import (
	"context"
	"fmt"
	"os"

	"github.com/relicta-tech/relicta/v4/internal/application/versioning"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// plannedBump is the version decision recorded by `relicta plan`.
type plannedBump struct {
	Current version.SemanticVersion
	Next    version.SemanticVersion
	Kind    domain.BumpKind
	RunID   domain.RunID
}

// asCalculateOutput presents the plan in the shape the display and JSON paths
// already expect, so honoring the plan does not fork the output code.
//
// AutoDetected is true because the bump kind was derived from commits when the
// plan was made; the user did not name it here.
func (p plannedBump) asCalculateOutput() *versioning.CalculateVersionOutput {
	return &versioning.CalculateVersionOutput{
		CurrentVersion: p.Current,
		NextVersion:    p.Next,
		BumpType:       version.BumpType(p.Kind),
		AutoDetected:   true,
	}
}

// plannedVersionToApply returns the version `plan` recorded, when bump should
// apply it rather than compute one.
//
// bump computed its own version and forced it onto the run through
// OverrideVersion, even though BumpVersionUseCase already reads run.VersionNext()
// and its input documents OverrideVersion as "if not provided, uses the version
// proposal from planning". Two components answering the same question is how they
// came to disagree: on a first release plan said 0.0.0 -> 0.1.0 and bump said
// 0.1.0 -> 0.2.0 (#264). That particular disagreement was fixed by making the two
// current-version defaults agree, which resolved the symptom while leaving the
// duplication that produced it.
//
// This removes the duplication: the plan is the decision, and bump executes it.
//
// Returns nil when there is nothing to honor, and bump computes as before:
//
//   - no release services or no run — bump is being used standalone
//   - the run is past planning, so its version is already applied
//   - the user named a bump type, a forced version, or build metadata, which is an
//     instruction and outranks the recorded plan
//   - the run is stale: HEAD moved or a release happened since it was planned, so
//     the recorded version was computed against a repository that no longer exists.
//     Applying it silently is the failure this must not introduce, and #230's
//     detector already knows how to see it.
func plannedVersionToApply(ctx context.Context, app cliApp, explicitRequest bool) *plannedBump {
	if explicitRequest {
		return nil
	}

	gitAdapter := app.GitAdapter()
	if gitAdapter == nil {
		return nil
	}
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return nil
	}

	if !app.HasReleaseServices() {
		if initErr := app.InitReleaseServices(ctx, repoInfo.Path); initErr != nil {
			return nil
		}
	}
	services := app.ReleaseServices()
	if services == nil || services.Repository == nil {
		return nil
	}

	run, err := services.Repository.LoadLatest(ctx, repoInfo.Path)
	if err != nil || run == nil {
		return nil
	}

	// Only a run still awaiting its bump has a proposal to apply. A later state
	// already carries the applied version, and re-applying it would be a no-op at
	// best.
	if run.State() != domain.StatePlanned {
		return nil
	}

	next := run.VersionNext()
	if next.IsZero() {
		return nil
	}

	// A plan computed against different commits is not a decision about this
	// repository any more.
	if reasons := detectRunStaleness(ctx, gitAdapter, run, cfg.Versioning.TagPrefix); len(reasons) > 0 {
		printWarning("the recorded plan is stale, so bump is recalculating:")
		// The reasons follow the warning onto the same stream. Written with
		// fmt.Printf they went to stdout unconditionally and corrupted the document
		// in --json mode — the mistake #235 fixed for printWarning itself, repeated
		// one line below it. A diagnostic belongs on stderr; stdout carries only the
		// document.
		out := os.Stdout
		if humanOutputSuppressed() {
			out = os.Stderr
		}
		for _, reason := range reasons {
			fmt.Fprintf(out, "    - %s\n", reason)
		}
		printInfo("Run 'relicta plan' to record a current plan.")
		return nil
	}

	return &plannedBump{
		Current: run.VersionCurrent(),
		Next:    next,
		Kind:    run.BumpKind(),
		RunID:   run.ID(),
	}
}

// bumpRequestIsExplicit reports whether the user named the version themselves.
//
// An explicit instruction outranks the recorded plan — the point of honoring the
// plan is to stop two components disagreeing when nobody asked either of them, not
// to override the person running the command.
//
// Reads every flag that changes the answer, not only the level: --prerelease and
// --channel select a different version from the same commits, and --build and
// --force restate it outright. Missing one would silently discard the user's
// instruction in favor of the plan, which is the same class of surprise this
// change exists to remove, pointing the other way.
func bumpRequestIsExplicit(level, prerelease, build, forced, channel string) bool {
	return level != "" || prerelease != "" || build != "" || forced != "" || channel != ""
}
