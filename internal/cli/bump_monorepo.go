package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	appmonorepo "github.com/relicta-tech/relicta/v4/internal/application/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/container"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/ai"
)

// Per-package versioning for `monorepo.enabled` repositories.
//
// The repository-wide path answers one question — what does this repository become — and a
// monorepo has that question once per package. Before this, a repository with two packages at
// 1.0.0 and 2.0.0 was told its next version was 0.1.0 and neither manifest was touched, because
// no field of the monorepo section was read by anything.
//
// Only `strategy: independent` arrives here; config validation refuses lockstep, hybrid and
// release_groups rather than serving them as something else.

// monorepoBumpJSON is the machine-readable form, kept separate from the aggregate's own JSON
// because it answers about packages rather than about a release.
type monorepoBumpJSON struct {
	Strategy   string                `json:"strategy"`
	FromRef    string                `json:"from_ref,omitempty"`
	ToRef      string                `json:"to_ref"`
	Discovered int                   `json:"packages_discovered"`
	DryRun     bool                  `json:"dry_run"`
	Packages   []monorepoPackageJSON `json:"packages"`
	Written    []string              `json:"files_written,omitempty"`
}

type monorepoPackageJSON struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Type    string `json:"type"`
	Current string `json:"current_version"`
	Next    string `json:"next_version"`
	Bump    string `json:"bump"`
	Commits int    `json:"commits"`
}

// runMonorepoBump versions each package from its own commits.
func runMonorepoBump(ctx context.Context, app cliApp, repoRoot string) error {
	// --force and --level name one version, and this repository has one per package. Applying
	// either to every package at once would move packages no commit touched.
	if bumpForce != "" {
		return fmt.Errorf("--force sets a single version, which a monorepo does not have; " +
			"set the version in the package's own manifest instead")
	}
	if bumpLevel != "" {
		return fmt.Errorf("--level applies one bump to every package; in an independent monorepo "+
			"each package's own commits decide its bump, so %q would move packages nothing "+
			"changed", bumpLevel)
	}

	// AI is optional: with no provider configured the analysis factory yields the
	// conventional-commit classifier, which is what the rest of the release path falls back to.
	var aiService ai.Service
	if app.HasAI() {
		aiService = app.AI()
	}
	bumper := container.NewMonorepoBumper(app.GitAdapter(), aiService)

	fromRef := lastReleaseTag(ctx, app)

	plan, err := bumper.Plan(ctx, appmonorepo.PlanInput{
		RepoRoot:     repoRoot,
		PackagePaths: cfg.Monorepo.PackagePaths,
		ExcludePaths: cfg.Monorepo.ExcludePaths,
		FromRef:      fromRef,
	})
	if err != nil {
		return err
	}

	if len(plan.Packages) == 0 {
		if outputJSON {
			return emitMonorepoBumpJSON(plan, repoRoot, nil)
		}
		printInfo(fmt.Sprintf("No package changed since %s — %d packages discovered, none to bump",
			refDisplay(fromRef), plan.Discovered))
		return nil
	}

	var written []string
	if !dryRun {
		if written, err = bumper.Apply(ctx, plan); err != nil {
			return err
		}
	}

	if outputJSON {
		return emitMonorepoBumpJSON(plan, repoRoot, written)
	}
	printMonorepoBumpText(plan, repoRoot, written)
	return nil
}

func printMonorepoBumpText(plan *appmonorepo.BumpPlan, repoRoot string, written []string) {
	printSubtitle(fmt.Sprintf("Packages (%d of %d changed since %s)",
		len(plan.Packages), plan.Discovered, refDisplay(plan.FromRef)))
	fmt.Println()

	for _, pkg := range plan.Packages {
		fmt.Printf("  %-24s %s → %s  (%s, %d commit%s)\n",
			displayPath(pkg.Path, repoRoot),
			pkg.Current.String(), pkg.Next.String(),
			pkg.Bump, pkg.Commits, plural(pkg.Commits))
	}
	fmt.Println()

	if dryRun {
		printInfo("Dry run — no manifest was written")
		return
	}
	printSuccess(fmt.Sprintf("Updated %d manifest%s", len(written), plural(len(written))))
	for _, file := range written {
		fmt.Printf("  %s\n", displayPath(file, repoRoot))
	}
}

func emitMonorepoBumpJSON(plan *appmonorepo.BumpPlan, repoRoot string, written []string) error {
	out := monorepoBumpJSON{
		Strategy:   string(cfg.Monorepo.Strategy),
		FromRef:    plan.FromRef,
		ToRef:      plan.ToRef,
		Discovered: plan.Discovered,
		DryRun:     dryRun,
		Packages:   make([]monorepoPackageJSON, 0, len(plan.Packages)),
	}
	for _, pkg := range plan.Packages {
		out.Packages = append(out.Packages, monorepoPackageJSON{
			Name:    pkg.Name,
			Path:    displayPath(pkg.Path, repoRoot),
			Type:    string(pkg.Type),
			Current: pkg.Current.String(),
			Next:    pkg.Next.String(),
			Bump:    string(pkg.Bump),
			Commits: pkg.Commits,
		})
	}
	for _, file := range written {
		out.Written = append(out.Written, displayPath(file, repoRoot))
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}

// lastReleaseTag is the base of the commit range.
//
// The repository's own last tag, not the package's: nothing creates per-package tags yet, so
// there is no `api-v1.4.0` to measure from. Reading the repository tag keeps a monorepo that
// has released before from re-counting its whole history, and a repository with no tags at all
// analyses everything, which is what a first release should do.
func lastReleaseTag(ctx context.Context, app cliApp) string {
	tag, err := app.GitAdapter().GetLatestVersionTag(ctx, cfg.Versioning.TagPrefix)
	if err != nil || tag == nil {
		return ""
	}
	return tag.Name()
}

func refDisplay(ref string) string {
	if ref == "" {
		return "the start of history"
	}
	return ref
}

// displayPath prefers the repository-relative form: absolute paths are what the writers need
// and not what an operator reading a table wants to compare.
func displayPath(path, repoRoot string) string {
	if rel, err := filepath.Rel(repoRoot, path); err == nil {
		return rel
	}
	return path
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// warnRepositoryWideInAMonorepo says which question the command about to run is answering.
//
// `relicta bump` versions each package; plan, publish and release still work on the repository
// as a whole, because a per-package tag, changelog and governance decision are not implemented
// yet. Saying so is the whole point: the defect this subsystem was an instance of is a setting
// that looks honored and is not, and a monorepo user whose `bump` produced two package versions
// has every reason to expect `publish` to tag two packages.
func warnRepositoryWideInAMonorepo(command string) {
	if !cfg.Monorepo.Enabled {
		return
	}
	printWarning(fmt.Sprintf("monorepo: `relicta %s` acts on the repository as a whole. "+
		"Per-package versioning applies to `relicta bump`; per-package tags, changelogs and "+
		"approvals are not implemented yet", command))
}
