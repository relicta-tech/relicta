// Package cli provides the command-line interface for ReleasePilot.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/felixgeelhaar/release-pilot/internal/application/release"
	"github.com/felixgeelhaar/release-pilot/internal/container"
	"github.com/felixgeelhaar/release-pilot/internal/domain/changes"
)

var (
	planFromRef string
	planToRef   string
	planShowAll bool
	planMinimal bool
)

func init() {
	planCmd.Flags().StringVar(&planFromRef, "from", "", "starting reference (default: latest tag)")
	planCmd.Flags().StringVar(&planToRef, "to", "HEAD", "ending reference")
	planCmd.Flags().BoolVar(&planShowAll, "all", false, "show all commits including non-conventional")
	planCmd.Flags().BoolVar(&planMinimal, "minimal", false, "show minimal output")
}

// runPlan implements the plan command.
func runPlan(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	printTitle("Release Plan")
	fmt.Println()

	// Initialize container
	dddContainer, err := container.NewInitializedDDDContainer(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer dddContainer.Close()

	// Get repository info for the path
	gitAdapter := dddContainer.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	// Prepare input
	input := release.PlanReleaseInput{
		RepositoryPath: repoInfo.Path,
		Branch:         repoInfo.CurrentBranch,
		FromRef:        planFromRef,
		ToRef:          planToRef,
		DryRun:         dryRun,
		TagPrefix:      cfg.Versioning.TagPrefix,
	}

	// Execute use case
	output, err := dddContainer.PlanRelease().Execute(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to plan release: %w", err)
	}

	// Output results
	if outputJSON {
		return outputPlanJSON(output)
	}

	return outputPlanText(output, planShowAll, planMinimal)
}

// outputPlanJSON outputs the plan as JSON.
func outputPlanJSON(output *release.PlanReleaseOutput) error {
	cats := output.ChangeSet.Categories()
	result := map[string]any{
		"release_id":      string(output.ReleaseID),
		"current_version": output.CurrentVersion.String(),
		"next_version":    output.NextVersion.String(),
		"release_type":    output.ReleaseType.String(),
		"repository_name": output.RepositoryName,
		"branch":          output.Branch,
		"ci_mode":         ciMode,
		"summary": map[string]int{
			"total":            output.ChangeSet.CommitCount(),
			"features":         len(cats.Features),
			"fixes":            len(cats.Fixes),
			"breaking_changes": len(cats.Breaking),
		},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// outputPlanText outputs the plan as text.
func outputPlanText(output *release.PlanReleaseOutput, showAll, minimal bool) error {
	// Summary
	printTitle("Summary")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  Current version:\t%s\n", output.CurrentVersion.String())
	fmt.Fprintf(w, "  Next version:\t%s\n", output.NextVersion.String())
	fmt.Fprintf(w, "  Release type:\t%s\n", releaseTypeDisplay(output.ReleaseType))
	fmt.Fprintf(w, "  Total commits:\t%d\n", output.ChangeSet.CommitCount())
	fmt.Fprintf(w, "  Repository:\t%s\n", output.RepositoryName)
	fmt.Fprintf(w, "  Branch:\t%s\n", output.Branch)
	w.Flush()

	fmt.Println()

	if !minimal {
		cats := output.ChangeSet.Categories()

		// Breaking changes
		if len(cats.Breaking) > 0 {
			printTitle("⚠ Breaking Changes")
			fmt.Println()
			for _, commit := range cats.Breaking {
				printConventionalCommit(commit)
			}
			fmt.Println()
		}

		// Features (non-breaking)
		nonBreakingFeatures := filterNonBreaking(cats.Features)
		if len(nonBreakingFeatures) > 0 {
			printTitle("✨ Features")
			fmt.Println()
			for _, commit := range nonBreakingFeatures {
				printConventionalCommit(commit)
			}
			fmt.Println()
		}

		// Bug Fixes
		if len(cats.Fixes) > 0 {
			printTitle("🐛 Bug Fixes")
			fmt.Println()
			for _, commit := range cats.Fixes {
				printConventionalCommit(commit)
			}
			fmt.Println()
		}

		// Performance
		if len(cats.Perf) > 0 {
			printTitle("⚡ Performance")
			fmt.Println()
			for _, commit := range cats.Perf {
				printConventionalCommit(commit)
			}
			fmt.Println()
		}

		// Other changes (if showAll)
		if showAll {
			other := getNonCoreCategorizedCommits(cats)
			if len(other) > 0 {
				printTitle("Other Changes")
				fmt.Println()
				for _, commit := range other {
					printConventionalCommit(commit)
				}
				fmt.Println()
			}
		}
	}

	// Next steps
	printTitle("Next Steps")
	fmt.Println()
	fmt.Printf("  1. Run 'release-pilot bump' to bump to %s\n", output.NextVersion.String())
	fmt.Println("  2. Run 'release-pilot notes' to generate release notes")
	fmt.Println("  3. Run 'release-pilot approve' to review and approve")
	fmt.Println("  4. Run 'release-pilot publish' to execute the release")
	fmt.Println()

	if !dryRun {
		printSuccess(fmt.Sprintf("Release plan saved with ID: %s", output.ReleaseID))
	}

	return nil
}

// printConventionalCommit prints a conventional commit.
func printConventionalCommit(commit *changes.ConventionalCommit) {
	scope := ""
	if commit.Scope() != "" {
		scope = fmt.Sprintf("(%s) ", commit.Scope())
	}

	hash := styles.Subtle.Render(commit.ShortHash())
	desc := commit.Subject()

	if commit.IsBreaking() {
		desc = styles.Error.Render("BREAKING: " + desc)
	}

	fmt.Printf("  %s %s%s\n", hash, scope, desc)
}

// releaseTypeDisplay returns a styled display string for the release type.
func releaseTypeDisplay(rt changes.ReleaseType) string {
	switch rt {
	case changes.ReleaseTypeMajor:
		return styles.Error.Render("major (breaking changes)")
	case changes.ReleaseTypeMinor:
		return styles.Info.Render("minor (new features)")
	case changes.ReleaseTypePatch:
		return styles.Success.Render("patch (bug fixes)")
	default:
		return styles.Subtle.Render("none")
	}
}

// filterNonBreaking filters out breaking commits from a slice.
func filterNonBreaking(commits []*changes.ConventionalCommit) []*changes.ConventionalCommit {
	var result []*changes.ConventionalCommit
	for _, c := range commits {
		if !c.IsBreaking() {
			result = append(result, c)
		}
	}
	return result
}

// getNonCoreCategorizedCommits returns commits that are not feat, fix, or perf from categories.
func getNonCoreCategorizedCommits(cats *changes.Categories) []*changes.ConventionalCommit {
	var result []*changes.ConventionalCommit
	result = append(result, cats.Docs...)
	result = append(result, cats.Refactors...)
	result = append(result, cats.Tests...)
	result = append(result, cats.Chores...)
	result = append(result, cats.Build...)
	result = append(result, cats.CI...)
	result = append(result, cats.Other...)
	return result
}
