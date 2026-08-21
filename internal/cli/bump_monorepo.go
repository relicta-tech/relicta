package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	appmonorepo "github.com/relicta-tech/relicta/v4/internal/application/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/container"
	"github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
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
	Tag     string `json:"tag"`
	BaseRef string `json:"base_ref,omitempty"`
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
		TagPrefixes:  monorepoTagPrefixes(),
		Skip:         monorepoSkipped(),
		FromRef:      fromRef,
	})
	if err != nil {
		return err
	}

	if len(plan.Packages) == 0 {
		if outputJSON {
			return emitMonorepoBumpJSON(plan, repoRoot, nil)
		}
		printInfo(fmt.Sprintf("No package changed since its last release — %d discovered, none to bump",
			plan.Discovered))
		return nil
	}

	var written []string
	if !dryRun {
		if written, err = bumper.Apply(ctx, plan); err != nil {
			return err
		}
	}

	if err := advanceRepositoryRun(ctx, app); err != nil {
		return err
	}

	// And each package's own run, which is what `approve --package` acts on.
	if !dryRun {
		bumpPackageRuns(ctx, app, repoRoot, plan.Packages)
	}

	if outputJSON {
		return emitMonorepoBumpJSON(plan, repoRoot, written)
	}
	printMonorepoBumpText(plan, repoRoot, written)
	return nil
}

// advanceRepositoryRun moves the repository's own release run to a version, so the rest of the
// flow still runs.
//
// Per-package versioning replaces the repository's *manifest* version, not its release record.
// The run is what `notes`, `approve` and `publish` act on, and what carries the governance
// decision and the audit chain; a monorepo bump that left it in `planned` produced
//
//	✗ cannot generate notes: release run is in 'planned' state. Run 'relicta bump' first
//
// from a user who had just run bump — the per-package work succeeded and the release could
// never be published. Reproduced against the shipped binary.
//
// Quiet on purpose: the package table above is this command's answer, and a second version
// printed beside it invites the reader to think one of the packages is going there.
func advanceRepositoryRun(ctx context.Context, app cliApp) error {
	if dryRun {
		return nil
	}

	current, err := configuredCurrentVersion(ctx, app)
	if err != nil {
		return err
	}

	calcOutput, err := app.CalculateVersion().Execute(ctx,
		buildCalculateVersionInput(version.BumpType(""), true, current))
	if err != nil {
		return fmt.Errorf("failed to calculate the repository version: %w", err)
	}

	if err := updateReleaseVersion(ctx, app, calcOutput.NextVersion); err != nil {
		if errors.Is(err, release.ErrRunNotFound) {
			// No plan yet. `relicta bump` before `relicta plan` is a legitimate order for
			// somebody who only wants the manifests written.
			return nil
		}
		return fmt.Errorf("failed to update release state: %w", err)
	}
	return nil
}

func printMonorepoBumpText(plan *appmonorepo.BumpPlan, repoRoot string, written []string) {
	// No repository-wide "since" in the heading: each package is measured from its own last
	// release, and one ref at the top would be wrong for every package that does not share it.
	printSubtitle(fmt.Sprintf("Packages (%d of %d changed)", len(plan.Packages), plan.Discovered))
	fmt.Println()

	for _, pkg := range plan.Packages {
		fmt.Printf("  %-24s %s → %-8s %-18s (%s, %d commit%s since %s)\n",
			displayPath(pkg.Path, repoRoot),
			pkg.Current.String(), pkg.Next.String(), pkg.Tag,
			pkg.Bump, pkg.Commits, plural(pkg.Commits), refDisplay(pkg.BaseRef))
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
			Tag:     pkg.Tag,
			BaseRef: pkg.BaseRef,
		})
	}
	for _, file := range written {
		out.Written = append(out.Written, displayPath(file, repoRoot))
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}

// monorepoTagPrefixes reduces the package overrides to the one field tag naming needs.
//
// The application layer takes a plain map rather than the config type: it should not have to
// import internal/config to name a tag, and the other override fields are not its business.
func monorepoTagPrefixes() map[string]string {
	if len(cfg.Monorepo.PackageOverrides) == 0 {
		return nil
	}
	prefixes := make(map[string]string, len(cfg.Monorepo.PackageOverrides))
	for path, override := range cfg.Monorepo.PackageOverrides {
		if override.TagPrefix != "" {
			prefixes[path] = override.TagPrefix
		}
	}
	return prefixes
}

// monorepoSkipped is the set of packages monorepo.package_overrides.<path>.skip_versioning
// excludes.
func monorepoSkipped() map[string]bool {
	if len(cfg.Monorepo.PackageOverrides) == 0 {
		return nil
	}
	skip := make(map[string]bool, len(cfg.Monorepo.PackageOverrides))
	for path, override := range cfg.Monorepo.PackageOverrides {
		if override.SkipVersioning {
			skip[path] = true
		}
	}
	return skip
}

// lastReleaseTag is the repository-wide fallback for the base of the commit range.
//
// Each package prefers its own last tag; this is what a package that has never been released
// under one falls back to, which is every package before the first per-package publish. A
// repository with no tags at all analyzes everything, which is what a first release should do.
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
// `relicta bump` versions each package, and `publish` tags each and writes each a changelog,
// but the plan and the governance decision are still one per repository. Saying so is the whole point: the defect
// this subsystem was an instance of is a setting that looks honored and is not, and a monorepo
// user whose bump produced two package versions has every reason to ask what `approve` just
// approved.
func warnRepositoryWideInAMonorepo(command string) {
	if !cfg.Monorepo.Enabled {
		return
	}
	printInfo(fmt.Sprintf("monorepo: `relicta %s` acts on the release as a whole. Each package "+
		"carries its own version, tag, changelog and decision — approve one with "+
		"`relicta approve --package <name>`", command))
}
