// Package cli provides the command-line interface for ReleasePilot.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/felixgeelhaar/release-pilot/internal/service/blast"
)

var (
	blastFromRef           string
	blastToRef             string
	blastVerbose           bool
	blastIncludeTests      bool
	blastIncludeDocs       bool
	blastIncludeTransitive bool
	blastGenerateGraph     bool
	blastPackagePaths      []string
	blastExcludePaths      []string
)

func init() {
	blastCmd.Flags().StringVar(&blastFromRef, "from", "", "starting reference (default: latest tag)")
	blastCmd.Flags().StringVar(&blastToRef, "to", "HEAD", "ending reference")
	blastCmd.Flags().BoolVarP(&blastVerbose, "verbose", "V", false, "show verbose output with file details")
	blastCmd.Flags().BoolVar(&blastIncludeTests, "include-tests", false, "include test files in analysis")
	blastCmd.Flags().BoolVar(&blastIncludeDocs, "include-docs", false, "include documentation files in analysis")
	blastCmd.Flags().BoolVar(&blastIncludeTransitive, "transitive", true, "include transitive dependency impacts")
	blastCmd.Flags().BoolVar(&blastGenerateGraph, "graph", false, "generate dependency graph")
	blastCmd.Flags().StringSliceVar(&blastPackagePaths, "package-paths", nil, "custom package paths (glob patterns)")
	blastCmd.Flags().StringSliceVar(&blastExcludePaths, "exclude", nil, "paths to exclude from analysis")

	// Add to root command
	rootCmd.AddCommand(blastCmd)
}

var blastCmd = &cobra.Command{
	Use:   "blast",
	Short: "Analyze blast radius of changes in a monorepo",
	Long: `Analyze the blast radius of changes between two git references.

This command examines changes in a monorepo and identifies:
- Directly affected packages (where files changed)
- Transitively affected packages (dependencies of changed packages)
- Risk assessment for each affected package
- Suggested release types for each package

Example:
  release-pilot blast --from v1.0.0 --to HEAD
  release-pilot blast --from HEAD~10 --verbose
  release-pilot blast --package-paths "packages/*,services/*"`,
	RunE: runBlast,
}

// runBlast implements the blast command.
func runBlast(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	if !outputJSON {
		printTitle("Blast Radius Analysis")
		fmt.Println()
	}

	// Get the repository path
	repoPath := "."

	// Build monorepo config from flags
	monorepoConfig := blast.DefaultMonorepoConfig()
	if len(blastPackagePaths) > 0 {
		monorepoConfig.PackagePaths = blastPackagePaths
	}
	if len(blastExcludePaths) > 0 {
		monorepoConfig.ExcludePaths = append(monorepoConfig.ExcludePaths, blastExcludePaths...)
	}

	// Create service
	svc := blast.NewService(
		blast.WithRepoPath(repoPath),
		blast.WithMonorepoConfig(monorepoConfig),
	)

	// Build analysis options
	opts := &blast.AnalysisOptions{
		FromRef:           blastFromRef,
		ToRef:             blastToRef,
		IncludeTransitive: blastIncludeTransitive,
		CalculateRisk:     true,
		GenerateGraph:     blastGenerateGraph,
		IncludeTests:      blastIncludeTests,
		IncludeDocs:       blastIncludeDocs,
		MonorepoConfig:    monorepoConfig,
	}

	// If no fromRef specified, the service will auto-detect the latest tag

	// Perform analysis
	result, err := svc.AnalyzeBlastRadius(ctx, opts)
	if err != nil {
		return fmt.Errorf("blast radius analysis failed: %w", err)
	}

	// Output results
	if outputJSON {
		return outputBlastJSON(result)
	}

	return outputBlastText(result, blastVerbose)
}

// outputBlastJSON outputs the blast radius analysis as JSON.
func outputBlastJSON(br *blast.BlastRadius) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(br)
}

// outputBlastText outputs the blast radius analysis as text.
func outputBlastText(br *blast.BlastRadius, verbose bool) error {
	outputBlastSummary(br)
	outputBlastRiskFactors(br.Summary.RiskFactors)
	outputBlastImpacts(br.Impacts, verbose)
	outputBlastChangesByCategory(br.Summary.ChangesByCategory, verbose)
	outputBlastLegend()
	return nil
}

// outputBlastSummary outputs the summary section of blast analysis.
func outputBlastSummary(br *blast.BlastRadius) {
	printTitle("Summary")
	fmt.Println()

	s := br.Summary
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  Analysis Range:\t%s → %s\n", br.FromRef, br.ToRef)
	fmt.Fprintf(w, "  Total Packages:\t%d\n", s.TotalPackages)
	fmt.Fprintf(w, "  Files Changed:\t%d (+%d/-%d lines)\n",
		s.TotalFilesChanged, s.TotalInsertions, s.TotalDeletions)
	fmt.Fprintf(w, "  Directly Affected:\t%d\n", s.DirectlyAffected)
	fmt.Fprintf(w, "  Transitively Affected:\t%d\n", s.TransitivelyAffected)
	fmt.Fprintf(w, "  Packages Needing Release:\t%d\n", s.PackagesRequiringRelease)
	w.Flush()
	fmt.Println()

	riskDisplay := formatRiskLevel(s.RiskLevel)
	fmt.Printf("  Risk Level: %s\n", riskDisplay)
}

// formatRiskLevel returns a styled string for the risk level.
func formatRiskLevel(level blast.RiskLevel) string {
	switch level {
	case blast.RiskLevelLow:
		return styles.Success.Render("LOW")
	case blast.RiskLevelMedium:
		return styles.Warning.Render("MEDIUM")
	case blast.RiskLevelHigh:
		return styles.Error.Render("HIGH")
	case blast.RiskLevelCritical:
		return styles.Error.Bold(true).Render("CRITICAL")
	default:
		return string(level)
	}
}

// outputBlastRiskFactors outputs the risk factors section.
func outputBlastRiskFactors(factors []string) {
	if len(factors) == 0 {
		return
	}
	fmt.Println()
	printTitle("Risk Factors")
	fmt.Println()
	for _, factor := range factors {
		fmt.Printf("  • %s\n", factor)
	}
}

// outputBlastImpacts outputs the impacted packages section.
func outputBlastImpacts(impacts []*blast.Impact, verbose bool) {
	if len(impacts) == 0 {
		fmt.Println()
		printSuccess("No packages affected by changes")
		fmt.Println()
		return
	}

	fmt.Println()
	printTitle("Impacted Packages")
	fmt.Println()

	for _, impact := range impacts {
		outputBlastImpactHeader(impact)
		if verbose {
			outputBlastImpactDetails(impact)
		}
		fmt.Println()
	}
}

// outputBlastImpactHeader outputs the header line for a package impact.
func outputBlastImpactHeader(impact *blast.Impact) {
	icon := formatImpactIcon(impact.Level)
	riskBadge := formatRiskBadge(impact.RiskScore)

	fmt.Printf("%s %s ", icon, styles.Bold.Render(impact.Package.Name))
	fmt.Printf("%s ", styles.Subtle.Render(fmt.Sprintf("(%s)", impact.Package.Type)))
	fmt.Printf("%s\n", riskBadge)

	fmt.Printf("    Path: %s\n", styles.Subtle.Render(impact.Package.Path))
	fmt.Printf("    Impact: %s", impact.Level)
	if impact.TransitiveDepth > 0 {
		fmt.Printf(" (depth: %d)", impact.TransitiveDepth)
	}
	fmt.Println()

	if impact.RequiresRelease && impact.ReleaseType != "" {
		fmt.Printf("    Suggested Release: %s\n", styles.Info.Render(impact.ReleaseType))
	}
}

// formatImpactIcon returns the icon for an impact level.
func formatImpactIcon(level blast.ImpactLevel) string {
	switch level {
	case blast.ImpactLevelDirect:
		return styles.Error.Render("●")
	case blast.ImpactLevelTransitive:
		return styles.Warning.Render("○")
	default:
		return "  "
	}
}

// formatRiskBadge returns a styled risk score badge.
func formatRiskBadge(score int) string {
	badge := fmt.Sprintf("[risk: %d]", score)
	switch {
	case score >= 70:
		return styles.Error.Render(badge)
	case score >= 40:
		return styles.Warning.Render(badge)
	default:
		return styles.Success.Render(badge)
	}
}

// outputBlastImpactDetails outputs verbose details for a package impact.
func outputBlastImpactDetails(impact *blast.Impact) {
	outputBlastChangedFiles(impact.DirectChanges)
	outputBlastAffectedDeps(impact.AffectedDependencies)
	outputBlastSuggestedActions(impact.SuggestedActions)
}

// outputBlastChangedFiles outputs the changed files for an impact.
func outputBlastChangedFiles(changes []blast.ChangedFile) {
	if len(changes) == 0 {
		return
	}
	fmt.Println("    Changed Files:")
	for _, change := range changes {
		catStyle := formatFileCategoryStyle(change.Category)
		fmt.Printf("      %s %s (+%d/-%d)\n",
			catStyle.Render(fmt.Sprintf("[%s]", change.Category)),
			change.Path,
			change.Insertions,
			change.Deletions)
	}
}

// formatFileCategoryStyle returns the style for a file category.
func formatFileCategoryStyle(cat blast.FileCategory) lipgloss.Style {
	switch cat {
	case blast.FileCategorySource:
		return styles.Info
	case blast.FileCategoryConfig:
		return styles.Warning
	case blast.FileCategoryTest:
		return styles.Success
	default:
		return styles.Subtle
	}
}

// outputBlastAffectedDeps outputs the affected dependencies.
func outputBlastAffectedDeps(deps []string) {
	if len(deps) == 0 {
		return
	}
	fmt.Println("    Affected Dependencies:")
	for _, dep := range deps {
		fmt.Printf("      → %s\n", dep)
	}
}

// outputBlastSuggestedActions outputs the suggested actions.
func outputBlastSuggestedActions(actions []string) {
	if len(actions) == 0 {
		return
	}
	fmt.Println("    Suggested Actions:")
	for _, action := range actions {
		fmt.Printf("      • %s\n", action)
	}
}

// outputBlastChangesByCategory outputs the changes by category section.
func outputBlastChangesByCategory(categories map[blast.FileCategory]int, verbose bool) {
	if !verbose || len(categories) == 0 {
		return
	}
	printTitle("Changes by Category")
	fmt.Println()
	for cat, count := range categories {
		fmt.Printf("  %s: %d files\n", cat, count)
	}
	fmt.Println()
}

// outputBlastLegend outputs the legend for impact icons.
func outputBlastLegend() {
	fmt.Println(styles.Subtle.Render("Legend: ● Direct impact, ○ Transitive impact"))
	fmt.Println()
}
