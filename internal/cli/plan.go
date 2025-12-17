// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/application/governance"
	apprelease "github.com/relicta-tech/relicta/internal/application/release"
	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/container"
	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release"
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

	if dryRun {
		printDryRunBanner()
	}

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
	input := apprelease.PlanReleaseInput{
		RepositoryPath: repoInfo.Path,
		Branch:         repoInfo.CurrentBranch,
		FromRef:        planFromRef,
		ToRef:          planToRef,
		DryRun:         dryRun,
		TagPrefix:      cfg.Versioning.TagPrefix,
	}

	// Execute use case with spinner (unless JSON output)
	var spinner *Spinner
	if !outputJSON {
		spinner = NewSpinner("Analyzing commits...")
		spinner.Start()
	}

	output, err := dddContainer.PlanRelease().Execute(ctx, input)

	if spinner != nil {
		spinner.Stop()
	}

	if err != nil {
		return fmt.Errorf("failed to plan release: %w", err)
	}

	// Get governance risk preview if enabled
	var riskPreview *governanceRiskPreview
	if dddContainer.HasGovernance() {
		riskPreview = getGovernanceRiskPreview(ctx, dddContainer, output, repoInfo.RemoteURL)
	}

	// Output results
	if outputJSON {
		return outputPlanJSON(output, riskPreview)
	}

	return outputPlanText(output, planShowAll, planMinimal, riskPreview)
}

// governanceRiskPreview holds the risk assessment preview for the plan.
type governanceRiskPreview struct {
	RiskScore      float64
	Severity       string
	Decision       string
	CanAutoApprove bool
	RiskFactors    []string
}

// outputPlanJSON outputs the plan as JSON.
func outputPlanJSON(output *apprelease.PlanReleaseOutput, riskPreview *governanceRiskPreview) error {
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

	// Add governance risk preview if available
	if riskPreview != nil {
		result["governance"] = map[string]any{
			"risk_score":       riskPreview.RiskScore,
			"severity":         riskPreview.Severity,
			"decision":         riskPreview.Decision,
			"can_auto_approve": riskPreview.CanAutoApprove,
			"risk_factors":     riskPreview.RiskFactors,
		}
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// outputPlanText outputs the plan as text.
func outputPlanText(output *apprelease.PlanReleaseOutput, showAll, minimal bool, riskPreview *governanceRiskPreview) error {
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

	// Governance risk preview (if enabled)
	if riskPreview != nil {
		printTitle("Governance Risk Preview")
		fmt.Println()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  Risk Score:\t%s\n", formatRiskScoreDisplay(riskPreview.RiskScore, riskPreview.Severity))
		fmt.Fprintf(w, "  Decision:\t%s\n", formatDecisionDisplay(riskPreview.Decision))
		fmt.Fprintf(w, "  Auto-Approve:\t%s\n", formatAutoApproveDisplay(riskPreview.CanAutoApprove))
		w.Flush()

		if len(riskPreview.RiskFactors) > 0 {
			fmt.Println()
			fmt.Println("  Risk Factors:")
			for _, factor := range riskPreview.RiskFactors {
				fmt.Printf("    - %s\n", factor)
			}
		}

		fmt.Println()
	}

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
	fmt.Printf("  1. Run 'relicta bump' to bump to %s\n", output.NextVersion.String())
	fmt.Println("  2. Run 'relicta notes' to generate release notes")
	fmt.Println("  3. Run 'relicta approve' to review and approve")
	fmt.Println("  4. Run 'relicta publish' to execute the release")
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

// getGovernanceRiskPreview performs a quick governance risk assessment for plan preview.
func getGovernanceRiskPreview(ctx context.Context, dddContainer *container.DDDContainer, output *apprelease.PlanReleaseOutput, repoURL string) *governanceRiskPreview {
	govService := dddContainer.GovernanceService()
	if govService == nil {
		return nil
	}

	// Create a temporary release from plan output (works in dry-run mode)
	rel := release.NewRelease(output.ReleaseID, output.Branch, "")
	plan := release.NewReleasePlan(
		output.CurrentVersion,
		output.NextVersion,
		output.ReleaseType,
		output.ChangeSet,
		dryRun,
	)
	if err := rel.SetPlan(plan); err != nil {
		return nil
	}

	// Create actor (similar to approve.go)
	actor := createCGPActorForPlan()

	// Evaluate
	input := governance.EvaluateReleaseInput{
		Release:    rel,
		Actor:      actor,
		Repository: repoURL,
	}

	result, err := govService.EvaluateRelease(ctx, input)
	if err != nil {
		// Don't fail the plan command if governance fails
		return nil
	}

	// Extract risk factors
	var riskFactors []string
	for _, factor := range result.RiskFactors {
		riskFactors = append(riskFactors, fmt.Sprintf("[%s] %s (%.0f%%)", factor.Category, factor.Description, factor.Score*100))
	}

	return &governanceRiskPreview{
		RiskScore:      result.RiskScore,
		Severity:       string(result.Severity),
		Decision:       string(result.Decision),
		CanAutoApprove: result.CanAutoApprove,
		RiskFactors:    riskFactors,
	}
}

// createCGPActorForPlan creates a CGP actor for plan preview.
func createCGPActorForPlan() cgp.Actor {
	// Simple actor for preview - just uses local user
	user := os.Getenv("USER")
	if user == "" {
		user = "unknown"
	}
	return cgp.NewHumanActor(user, user)
}

// formatRiskScoreDisplay formats the risk score with severity label.
func formatRiskScoreDisplay(score float64, severity string) string {
	percent := fmt.Sprintf("%.1f%%", score*100)

	switch severity {
	case "critical", "high":
		return styles.Error.Render(fmt.Sprintf("%s (%s)", percent, severity))
	case "medium":
		return styles.Warning.Render(fmt.Sprintf("%s (%s)", percent, severity))
	default:
		return styles.Success.Render(fmt.Sprintf("%s (%s)", percent, severity))
	}
}

// formatDecisionDisplay formats the decision with appropriate styling.
func formatDecisionDisplay(decision string) string {
	switch decision {
	case "approved":
		return styles.Success.Render(decision)
	case "approval_required":
		return styles.Warning.Render("requires approval")
	case "rejected":
		return styles.Error.Render(decision)
	default:
		return styles.Subtle.Render(decision)
	}
}

// formatAutoApproveDisplay formats the auto-approve status.
func formatAutoApproveDisplay(canAutoApprove bool) string {
	if canAutoApprove {
		return styles.Success.Render("yes")
	}
	return styles.Warning.Render("no (manual review required)")
}
