// Package cli provides the command-line interface for Relicta.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/analysis"
	"github.com/relicta-tech/relicta/internal/application/governance"
	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release"
	releaseapp "github.com/relicta-tech/relicta/internal/domain/release/app"
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/domain/version"
	servicerelease "github.com/relicta-tech/relicta/internal/service/release"
)

var (
	planFromRef       string
	planToRef         string
	planShowAll       bool
	planMinimal       bool
	planAnalyze       bool
	planReview        bool
	planMinConfidence float64
	planDisableAI     bool
)

func init() {
	planCmd.Flags().StringVar(&planFromRef, "from", "", "starting reference (default: latest tag)")
	planCmd.Flags().StringVar(&planToRef, "to", "HEAD", "ending reference")
	planCmd.Flags().BoolVar(&planShowAll, "all", false, "show all commits including non-conventional")
	planCmd.Flags().BoolVar(&planMinimal, "minimal", false, "show minimal output")
	planCmd.Flags().BoolVar(&planAnalyze, "analyze", false, "analyze commit classifications and stop")
	planCmd.Flags().BoolVar(&planReview, "review", false, "review and adjust commit classifications before planning")
	planCmd.Flags().Float64Var(&planMinConfidence, "min-confidence", 0, "minimum confidence to accept classifications")
	planCmd.Flags().BoolVar(&planDisableAI, "no-ai", false, "disable AI classification")
}

// runPlan implements the plan command.
func runPlan(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	if planAnalyze && planReview {
		return fmt.Errorf("use either --analyze or --review, not both")
	}

	if planReview && outputJSON {
		return fmt.Errorf("--review is not supported with --json output")
	}

	printTitle("Release Plan")
	fmt.Println()

	if dryRun {
		printDryRunBanner()
	}

	// Initialize container
	app, err := newContainerApp(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer closeApp(app)

	// Check for tag-push mode (HEAD is already tagged)
	mode, existingVersion, err := detectReleaseMode(ctx, app, cfg.Versioning.TagPrefix)
	if err != nil {
		return fmt.Errorf("failed to detect release mode: %w", err)
	}

	if mode == releaseModeTagPush && existingVersion != nil {
		return runPlanTagPush(ctx, app, *existingVersion)
	}

	// Get repository info for the path
	gitAdapter := app.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	// Prepare input
	input := servicerelease.AnalyzeInput{
		RepositoryPath: repoInfo.Path,
		Branch:         repoInfo.CurrentBranch,
		FromRef:        planFromRef,
		ToRef:          planToRef,
		TagPrefix:      cfg.Versioning.TagPrefix,
	}

	minConfidenceSet := cmd.Flags().Changed("min-confidence")
	analysisConfig, hasAnalysisConfig := buildPlanAnalysisConfig(minConfidenceSet)
	if hasAnalysisConfig {
		input.AnalysisConfig = &analysisConfig
	}

	if planAnalyze {
		return runPlanAnalyze(ctx, app, input)
	}

	if planReview {
		return runPlanReview(ctx, app, input, repoInfo.RemoteURL)
	}

	// Execute use case with spinner (unless JSON output)
	var spinner *Spinner
	if !outputJSON {
		spinner = NewSpinner("Analyzing commits...")
		spinner.Start()
	}

	output, err := app.ReleaseAnalyzer().Analyze(ctx, input)

	if spinner != nil {
		spinner.Stop()
	}

	if err != nil {
		return fmt.Errorf("failed to plan release: %w", err)
	}

	// Persist release run for subsequent commands (bump, notes, approve, publish)
	var releaseID string
	if !dryRun {
		releaseID, err = persistReleaseRun(ctx, app, output, repoInfo)
		if err != nil {
			printWarning(fmt.Sprintf("release run persistence failed: %v", err))
		}
	}

	// Get governance risk preview if enabled
	var riskPreview *governanceRiskPreview
	if app.HasGovernance() {
		riskPreview = getGovernanceRiskPreview(ctx, app, output, repoInfo.RemoteURL)
	}

	// Output results
	if outputJSON {
		return outputPlanJSON(output, releaseID, riskPreview)
	}

	return outputPlanText(output, releaseID, planShowAll, planMinimal, riskPreview)
}

func buildPlanAnalysisConfig(minConfidenceSet bool) (analysis.AnalyzerConfig, bool) {
	cfg := analysis.DefaultConfig()
	updated := planAnalyze || planReview
	if minConfidenceSet {
		cfg.MinConfidence = planMinConfidence
		updated = true
	}
	if planDisableAI {
		cfg.EnableAI = false
		updated = true
	}

	return cfg, updated
}

// runPlanTagPush handles the tag-push scenario where HEAD is already tagged.
// It executes the plan use case with the existing tag to create release state,
// enabling subsequent commands (notes, approve, publish) to work.
func runPlanTagPush(ctx context.Context, app cliApp, ver version.SemanticVersion) error {
	tagName := cfg.Versioning.TagPrefix + ver.String()

	printInfo(fmt.Sprintf("HEAD is already tagged: %s", tagName))
	printInfo("Running in tag-push mode")
	fmt.Println()

	// Get repository info
	gitAdapter := app.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	// Find previous version tag
	tags, err := gitAdapter.GetTags(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tags: %w", err)
	}

	var prevTagName string
	var prevVersion *version.SemanticVersion
	for _, t := range tags.FilterByPrefix(cfg.Versioning.TagPrefix).VersionTags() {
		tagVer := t.Version()
		if tagVer != nil && tagVer.LessThan(ver) {
			if prevVersion == nil || tagVer.GreaterThan(*prevVersion) {
				prevTagName = t.Name()
				prevVersion = tagVer
			}
		}
	}

	// Execute analysis to create release state
	planInput := servicerelease.AnalyzeInput{
		RepositoryPath: repoInfo.Path,
		Branch:         repoInfo.CurrentBranch,
		FromRef:        prevTagName,
		ToRef:          tagName,
		TagPrefix:      cfg.Versioning.TagPrefix,
	}

	// Execute with spinner (unless JSON output)
	var spinner *Spinner
	if !outputJSON {
		spinner = NewSpinner("Analyzing commits...")
		spinner.Start()
	}

	output, err := app.ReleaseAnalyzer().Analyze(ctx, planInput)

	if spinner != nil {
		spinner.Stop()
	}

	if err != nil {
		return fmt.Errorf("failed to plan release: %w", err)
	}

	// Override next version to match existing tag
	output.NextVersion = ver

	// Persist release run for subsequent commands
	var releaseID string
	if !dryRun {
		releaseID, err = persistReleaseRun(ctx, app, output, repoInfo)
		if err != nil {
			printWarning(fmt.Sprintf("release run persistence failed: %v", err))
		}
	}

	// Get governance risk preview if enabled
	var riskPreview *governanceRiskPreview
	if app.HasGovernance() {
		riskPreview = getGovernanceRiskPreview(ctx, app, output, repoInfo.RemoteURL)
	}

	// Output results
	if outputJSON {
		return outputPlanTagPushJSON(output, releaseID, riskPreview)
	}

	return outputPlanTagPushText(output, releaseID, riskPreview)
}

// outputPlanTagPushJSON outputs the tag-push plan as JSON.
func outputPlanTagPushJSON(output *servicerelease.AnalyzeOutput, releaseID string, riskPreview *governanceRiskPreview) error {
	cats := output.ChangeSet.Categories()
	result := map[string]any{
		"mode":            "tag-push",
		"release_id":      releaseID,
		"current_version": output.CurrentVersion.String(),
		"next_version":    output.NextVersion.String(),
		"release_type":    output.ReleaseType.String(),
		"repository_name": output.RepositoryName,
		"branch":          output.Branch,
		"summary": map[string]int{
			"total":            output.ChangeSet.CommitCount(),
			"features":         len(cats.Features),
			"fixes":            len(cats.Fixes),
			"breaking_changes": len(cats.Breaking),
		},
	}

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

// outputPlanTagPushText outputs the tag-push plan as text.
func outputPlanTagPushText(output *servicerelease.AnalyzeOutput, releaseID string, riskPreview *governanceRiskPreview) error {
	// Summary
	printTitle("Tag-Push Mode Summary")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  Previous version:\t%s\n", output.CurrentVersion.String())
	fmt.Fprintf(w, "  Current version:\t%s\n", output.NextVersion.String())
	fmt.Fprintf(w, "  Total commits:\t%d\n", output.ChangeSet.CommitCount())
	fmt.Fprintf(w, "  Repository:\t%s\n", output.RepositoryName)
	fmt.Fprintf(w, "  Branch:\t%s\n", output.Branch)
	_ = w.Flush()

	fmt.Println()

	// Governance risk preview (if enabled)
	if riskPreview != nil {
		printTitle("Governance Risk Preview")
		fmt.Println()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  Risk Score:\t%s\n", formatRiskScoreDisplay(riskPreview.RiskScore, riskPreview.Severity))
		fmt.Fprintf(w, "  Decision:\t%s\n", formatDecisionDisplay(riskPreview.Decision))
		fmt.Fprintf(w, "  Auto-Approve:\t%s\n", formatAutoApproveDisplay(riskPreview.CanAutoApprove))
		_ = w.Flush()

		if len(riskPreview.RiskFactors) > 0 {
			fmt.Println()
			fmt.Println("  Risk Factors:")
			for _, factor := range riskPreview.RiskFactors {
				fmt.Printf("    - %s\n", factor)
			}
		}

		fmt.Println()
	}

	// Next steps for tag-push mode
	printTitle("Next Steps")
	fmt.Println()
	fmt.Println("  Since HEAD is already tagged, bump is not needed:")
	fmt.Println("  1. Run 'relicta notes' to generate release notes")
	fmt.Println("  2. Run 'relicta approve --yes' to approve the release")
	fmt.Println("  3. Run 'relicta publish --skip-push' to execute the release")
	fmt.Println()
	fmt.Println("  Or use 'relicta release --yes' to run all steps automatically.")
	fmt.Println()

	if !dryRun && releaseID != "" {
		printSuccess(fmt.Sprintf("Release plan saved with ID: %s", releaseID))
	}

	return nil
}

func runPlanAnalyze(ctx context.Context, app cliApp, input servicerelease.AnalyzeInput) error {
	result, commitInfos, err := app.ReleaseAnalyzer().AnalyzeCommits(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to analyze commits: %w", err)
	}

	if outputJSON {
		return outputAnalysisJSON(result, commitInfos)
	}

	return outputAnalysisText(result, commitInfos)
}

func runPlanReview(ctx context.Context, app cliApp, input servicerelease.AnalyzeInput, repoURL string) error {
	if ciMode {
		return fmt.Errorf("--review is not supported in CI mode")
	}

	result, commitInfos, err := app.ReleaseAnalyzer().AnalyzeCommits(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to analyze commits: %w", err)
	}

	classifications, err := reviewCommitClassifications(result, commitInfos)
	if err != nil {
		return err
	}

	input.CommitClassifications = classifications

	var spinner *Spinner
	if !outputJSON {
		spinner = NewSpinner("Planning release...")
		spinner.Start()
	}

	output, err := app.ReleaseAnalyzer().Analyze(ctx, input)

	if spinner != nil {
		spinner.Stop()
	}

	if err != nil {
		return fmt.Errorf("failed to plan release: %w", err)
	}

	// Persist release run for subsequent commands
	var releaseID string
	if !dryRun {
		releaseID, err = persistReleaseRunFromApp(ctx, app, output)
		if err != nil {
			printWarning(fmt.Sprintf("release run persistence failed: %v", err))
		}
	}

	var riskPreview *governanceRiskPreview
	if app.HasGovernance() {
		riskPreview = getGovernanceRiskPreview(ctx, app, output, repoURL)
	}

	if outputJSON {
		return outputPlanJSON(output, releaseID, riskPreview)
	}

	return outputPlanText(output, releaseID, planShowAll, planMinimal, riskPreview)
}

func outputAnalysisJSON(result *analysis.AnalysisResult, commitInfos []analysis.CommitInfo) error {
	commits := make([]map[string]any, 0, len(commitInfos))
	for _, info := range commitInfos {
		classification := result.Classifications[info.Hash]
		entry := map[string]any{
			"hash":    info.Hash.String(),
			"subject": info.Subject,
		}
		if classification != nil {
			entry["type"] = string(classification.Type)
			entry["scope"] = classification.Scope
			entry["method"] = classification.Method.String()
			entry["confidence"] = classification.Confidence
			entry["reasoning"] = classification.Reasoning
			entry["is_breaking"] = classification.IsBreaking
			entry["breaking_reason"] = classification.BreakingReason
			entry["should_skip"] = classification.ShouldSkip
			entry["skip_reason"] = classification.SkipReason
		}
		commits = append(commits, entry)
	}

	payload := map[string]any{
		"stats":         result.Stats,
		"commits":       commits,
		"total_commits": result.Stats.TotalCommits,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func outputAnalysisText(result *analysis.AnalysisResult, commitInfos []analysis.CommitInfo) error {
	printTitle("Commit Analysis")
	fmt.Println()

	fmt.Printf("  Analyzed %d commits\n", result.Stats.TotalCommits)
	fmt.Printf("  Average confidence: %.2f\n", result.Stats.AverageConfidence)
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  Conventional:\t%d\n", result.Stats.ConventionalCount)
	fmt.Fprintf(w, "  Heuristics:\t%d\n", result.Stats.HeuristicCount)
	fmt.Fprintf(w, "  AST:\t%d\n", result.Stats.ASTCount)
	fmt.Fprintf(w, "  AI:\t%d\n", result.Stats.AICount)
	fmt.Fprintf(w, "  Skipped:\t%d\n", result.Stats.SkippedCount)
	fmt.Fprintf(w, "  Low confidence:\t%d\n", result.Stats.LowConfidenceCount)
	_ = w.Flush()

	if len(result.Stats.LowConfidenceCommits) > 0 {
		fmt.Println()
		fmt.Println("  Low confidence commits:")
		for _, hash := range result.Stats.LowConfidenceCommits {
			fmt.Printf("    - %s\n", hash.Short())
		}
	}

	fmt.Println()
	printTitle("Commit Breakdown")
	fmt.Println()

	for _, info := range commitInfos {
		classification := result.Classifications[info.Hash]
		if classification == nil {
			fmt.Printf("  %s  unknown  ?    0.00  %s\n", info.Hash.Short(), info.Subject)
			continue
		}

		commitType := string(classification.Type)
		if classification.ShouldSkip {
			commitType = "skip"
		} else if commitType == "" {
			commitType = "unknown"
		}

		fmt.Printf("  %s  %-8s  %-4s  %.2f  %s\n",
			info.Hash.Short(),
			commitType,
			classification.Method.ShortString(),
			classification.Confidence,
			info.Subject,
		)

		if classification.Reasoning != "" {
			fmt.Printf("        reason: %s\n", classification.Reasoning)
		}
		if classification.ShouldSkip && classification.SkipReason != "" {
			fmt.Printf("        skip: %s\n", classification.SkipReason)
		}
		if classification.IsBreaking && classification.BreakingReason != "" {
			fmt.Printf("        breaking: %s\n", classification.BreakingReason)
		}
	}

	fmt.Println()
	fmt.Println("  Run 'relicta plan' to create the release plan.")
	return nil
}

func reviewCommitClassifications(result *analysis.AnalysisResult, commitInfos []analysis.CommitInfo) (map[sourcecontrol.CommitHash]*analysis.CommitClassification, error) {
	reader := bufio.NewReader(os.Stdin)
	classifications := make(map[sourcecontrol.CommitHash]*analysis.CommitClassification, len(commitInfos))

	for idx, info := range commitInfos {
		classification := result.Classifications[info.Hash]
		if classification == nil {
			classification = &analysis.CommitClassification{
				CommitHash: info.Hash,
				Method:     analysis.MethodHeuristic,
				Confidence: 0,
				Reasoning:  "unclassified",
			}
		}

		fmt.Println()
		fmt.Printf("[%d/%d] %s  %s\n", idx+1, len(commitInfos), info.Hash.Short(), info.Subject)
		fmt.Printf("  Detected: %s (%s, %.2f)\n", classificationTypeLabel(classification), classification.Method.String(), classification.Confidence)
		if len(info.Files) > 0 {
			fmt.Printf("  Files: %s\n", strings.Join(trimList(info.Files, 6), ", "))
		}
		if classification.Reasoning != "" {
			fmt.Printf("  Reason: %s\n", classification.Reasoning)
		}

		for {
			fmt.Print("  Override? (enter=accept, type[/!], skip) > ")
			line, err := reader.ReadString('\n')
			if err != nil {
				return nil, err
			}
			line = strings.TrimSpace(line)
			if line == "" {
				classifications[info.Hash] = classification
				break
			}

			updated, err := parseClassificationOverride(line, classification)
			if err != nil {
				fmt.Printf("  %s\n", err.Error())
				continue
			}
			classifications[info.Hash] = updated
			break
		}
	}

	return classifications, nil
}

func parseClassificationOverride(input string, current *analysis.CommitClassification) (*analysis.CommitClassification, error) {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "skip" || lower == "s" {
		return &analysis.CommitClassification{
			CommitHash: current.CommitHash,
			Method:     analysis.MethodManual,
			Confidence: 1.0,
			ShouldSkip: true,
			SkipReason: "manual skip",
			Reasoning:  "manual override",
		}, nil
	}

	isBreaking := false
	if strings.HasSuffix(lower, "!") {
		isBreaking = true
		lower = strings.TrimSuffix(lower, "!")
	}

	commitType, ok := changes.ParseCommitType(lower)
	if !ok {
		return nil, fmt.Errorf("unknown type: %s", input)
	}

	updated := *current
	updated.Type = commitType
	updated.Method = analysis.MethodManual
	updated.Confidence = 1.0
	updated.Reasoning = "manual override"
	updated.ShouldSkip = false
	updated.SkipReason = ""
	updated.IsBreaking = isBreaking
	if isBreaking {
		updated.BreakingReason = "manual override"
	} else {
		updated.BreakingReason = ""
	}

	return &updated, nil
}

func classificationTypeLabel(classification *analysis.CommitClassification) string {
	if classification == nil {
		return "unknown"
	}
	if classification.ShouldSkip {
		return "skip"
	}
	if classification.Type == "" {
		return "unknown"
	}
	return string(classification.Type)
}

func trimList(items []string, limit int) []string {
	if len(items) <= limit {
		return items
	}
	return append(items[:limit], "...")
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
func outputPlanJSON(output *servicerelease.AnalyzeOutput, releaseID string, riskPreview *governanceRiskPreview) error {
	cats := output.ChangeSet.Categories()
	result := map[string]any{
		"release_id":      releaseID,
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
func outputPlanText(output *servicerelease.AnalyzeOutput, releaseID string, showAll, minimal bool, riskPreview *governanceRiskPreview) error {
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
	_ = w.Flush() // Ignore flush error for stdout display

	fmt.Println()

	// Governance risk preview (if enabled)
	if riskPreview != nil {
		printTitle("Governance Risk Preview")
		fmt.Println()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  Risk Score:\t%s\n", formatRiskScoreDisplay(riskPreview.RiskScore, riskPreview.Severity))
		fmt.Fprintf(w, "  Decision:\t%s\n", formatDecisionDisplay(riskPreview.Decision))
		fmt.Fprintf(w, "  Auto-Approve:\t%s\n", formatAutoApproveDisplay(riskPreview.CanAutoApprove))
		_ = w.Flush() // Ignore flush error for stdout display

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

	if !dryRun && releaseID != "" {
		printSuccess(fmt.Sprintf("Release plan saved with ID: %s", releaseID))
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
func getGovernanceRiskPreview(ctx context.Context, app cliApp, output *servicerelease.AnalyzeOutput, repoURL string) *governanceRiskPreview {
	govService := app.GovernanceService()
	if govService == nil {
		return nil
	}

	// Create a temporary release from plan output (works in dry-run mode)
	rel := release.NewReleaseRun(
		"",            // repoID
		"",            // repoRoot
		output.Branch, // baseRef
		"",            // headSHA
		nil,           // commits
		"",            // configHash
		"",            // pluginPlanHash
	)
	plan := release.NewReleasePlan(
		output.CurrentVersion,
		output.NextVersion,
		output.ReleaseType,
		output.ChangeSet,
		dryRun,
	)
	if err := release.SetPlan(rel, plan); err != nil {
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

// actorID is the identifier used for CLI-initiated actions.
const actorID = "cli"

// persistReleaseRunFromApp persists the release run by first obtaining repository info.
func persistReleaseRunFromApp(ctx context.Context, app cliApp, output *servicerelease.AnalyzeOutput) (string, error) {
	gitAdapter := app.GitAdapter()
	if gitAdapter == nil {
		return "", fmt.Errorf("git adapter not available")
	}

	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get repository info: %w", err)
	}

	return persistReleaseRun(ctx, app, output, repoInfo)
}

// persistReleaseRun stores the release run with pre-computed analysis data.
// This enables subsequent commands (bump, notes, approve, publish) to operate on the release.
func persistReleaseRun(ctx context.Context, app cliApp, output *servicerelease.AnalyzeOutput, repoInfo *sourcecontrol.RepositoryInfo) (string, error) {
	if err := app.InitReleaseServices(ctx, repoInfo.Path); err != nil {
		return "", fmt.Errorf("failed to initialize release services: %w", err)
	}

	if !app.HasReleaseServices() {
		return "", fmt.Errorf("release services not available")
	}

	services := app.ReleaseServices()
	if services == nil || services.PlanRelease == nil {
		return "", fmt.Errorf("PlanRelease use case not available")
	}

	bumpKind := convertReleaseTypeToBumpKind(output.ReleaseType)

	input := releaseapp.PlanReleaseInput{
		RepoRoot: repoInfo.Path,
		RepoID:   repoInfo.RemoteURL,
		BaseRef:  "", // Auto-detect from tags
		Actor: ports.ActorInfo{
			Type: "user",
			ID:   actorID,
		},
		Force:          true, // Force to replace any existing run from legacy
		ChangeSet:      output.ChangeSet,
		CurrentVersion: &output.CurrentVersion,
		NextVersion:    &output.NextVersion,
		BumpKind:       &bumpKind,
		Confidence:     1.0, // Legacy analysis is authoritative
	}

	planOutput, err := services.PlanRelease.Execute(ctx, input)
	if err != nil {
		return "", err
	}
	return string(planOutput.RunID), nil
}

// convertReleaseTypeToBumpKind converts ReleaseType to the domain BumpKind.
func convertReleaseTypeToBumpKind(rt changes.ReleaseType) domain.BumpKind {
	switch rt {
	case changes.ReleaseTypeMajor:
		return domain.BumpMajor
	case changes.ReleaseTypeMinor:
		return domain.BumpMinor
	case changes.ReleaseTypePatch:
		return domain.BumpPatch
	default:
		return domain.BumpNone
	}
}
