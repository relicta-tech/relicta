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
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/analysis"
	"github.com/relicta-tech/relicta/v4/internal/application/governance"
	"github.com/relicta-tech/relicta/v4/internal/application/recommendation"
	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/release"
	releaseapp "github.com/relicta-tech/relicta/v4/internal/domain/release/app"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
	servicerelease "github.com/relicta-tech/relicta/v4/internal/service/release"
)

var (
	planFromRef        string
	planToRef          string
	planShowAll        bool
	planMinimal        bool
	planAnalyze        bool
	planReview         bool
	planMinConfidence  float64
	planDisableAI      bool
	planSkipCognitive  bool
	planChronosThreads int
	planForce          bool
)

func init() {
	planCmd.Flags().StringVarP(&planFromRef, "from", "f", "", "starting reference (default: latest tag)")
	planCmd.Flags().StringVarP(&planToRef, "to", "t", "HEAD", "ending reference")
	planCmd.Flags().BoolVarP(&planShowAll, "all", "a", false, "show all commits including non-conventional")
	planCmd.Flags().BoolVarP(&planMinimal, "minimal", "m", false, "show minimal output")
	planCmd.Flags().BoolVar(&planAnalyze, "analyze", false, "analyze commit classifications and stop")
	planCmd.Flags().BoolVarP(&planReview, "review", "r", false, "review and adjust commit classifications before planning")
	planCmd.Flags().Float64Var(&planMinConfidence, "min-confidence", 0, "minimum confidence to accept classifications")
	planCmd.Flags().BoolVar(&planDisableAI, "no-ai", false, "disable AI classification")
	planCmd.Flags().BoolVar(&planSkipCognitive, "skip-cognitive", false, "skip Mnemos & Chronos cognitive backends")
	planCmd.Flags().IntVar(&planChronosThreads, "chronos-threads", 0, "max concurrent Chronos ingest requests (overrides config)")
	planCmd.Flags().BoolVar(&planForce, "force", false, "discard an existing release run for these same commits, including its approval")
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

	if planSkipCognitive {
		cfg.Mnemos.Enabled = false
		cfg.Chronos.Enabled = false
	}

	if planChronosThreads > 0 {
		cfg.Chronos.Threads = planChronosThreads
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
	var persisted persistedRun
	if !dryRun {
		persisted, err = persistReleaseRun(ctx, app, output, repoInfo)
		if err != nil {
			printWarning(fmt.Sprintf("release run persistence failed: %v", err))
		}
	}

	// Get governance risk preview if enabled
	var riskPreview *governanceRiskPreview
	if app.HasGovernance() {
		riskPreview = getGovernanceRiskPreview(ctx, app, output, repoInfo.RemoteURL)
	}

	storeRecommendation(app, output, riskPreview, persisted, repoInfo.Path)

	// Output results
	if outputJSON {
		return outputPlanJSON(output, persisted, riskPreview)
	}

	return outputPlanText(output, persisted, planShowAll, planMinimal, riskPreview)
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
	for _, t := range tags.VersionTagsWithPrefix(cfg.Versioning.TagPrefix) {
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
	// Use tag-push mode to transition directly to versioned state
	var releaseID string
	if !dryRun {
		var persisted persistedRun
		persisted, err = persistReleaseRunWithOptions(ctx, app, output, repoInfo, persistReleaseRunOptions{
			TagPushMode: true,
			TagName:     tagName,
		})
		releaseID = persisted.ID
		if err != nil {
			printWarning(fmt.Sprintf("release run persistence failed: %v", err))
		}
	}

	// Get governance risk preview if enabled
	var riskPreview *governanceRiskPreview
	if app.HasGovernance() {
		riskPreview = getGovernanceRiskPreview(ctx, app, output, repoInfo.RemoteURL)
	}

	storeRecommendation(app, output, riskPreview, persistedRun{ID: releaseID}, repoInfo.Path)

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
	var persisted persistedRun
	if !dryRun {
		var releaseID string
		releaseID, err = persistReleaseRunFromApp(ctx, app, output)
		if err != nil {
			printWarning(fmt.Sprintf("release run persistence failed: %v", err))
		}
		persisted = persistedRun{ID: releaseID}
	}

	var riskPreview *governanceRiskPreview
	if app.HasGovernance() {
		riskPreview = getGovernanceRiskPreview(ctx, app, output, repoURL)
	}

	if outputJSON {
		return outputPlanJSON(output, persisted, riskPreview)
	}

	return outputPlanText(output, persisted, planShowAll, planMinimal, riskPreview)
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
//
// RiskFactors is pre-formatted for text output. The structured fields below it
// carry the same data unflattened, because the recommendation artifact needs the
// categories, scores and severities that formatting discards (ADR-009).
type governanceRiskPreview struct {
	RiskScore      float64
	Severity       string
	Decision       string
	CanAutoApprove bool
	RiskFactors    []string

	Factors         []cgp.RiskFactor
	Rationale       []string
	RequiredActions []cgp.RequiredAction
}

// outputPlanJSON outputs the plan as JSON.
func outputPlanJSON(output *servicerelease.AnalyzeOutput, persisted persistedRun, riskPreview *governanceRiskPreview) error {
	cats := output.ChangeSet.Categories()
	result := map[string]any{
		"release_id":      persisted.ID,
		"current_version": output.CurrentVersion.String(),
		"next_version":    output.NextVersion.String(),
		"release_type":    output.ReleaseType.String(),
		"repository_name": output.RepositoryName,
		"branch":          output.Branch,
		"ci_mode":         ciMode,
		// A consumer needs this to read the commit count correctly: on a first
		// release it covers the whole history, not the commits since a tag.
		"first_release": persisted.FirstRelease,
		"summary": map[string]int{
			"total":            output.ChangeSet.CommitCount(),
			"features":         len(cats.Features),
			"fixes":            len(cats.Fixes),
			"breaking_changes": len(cats.Breaking),
		},
		// The deterministic recommendation artifact (ADR-009). Emitted alongside
		// the flattened keys above so existing consumers keep working.
		"recommendation": buildRecommendationArtifact(output, riskPreview),
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
func outputPlanText(output *servicerelease.AnalyzeOutput, persisted persistedRun, showAll, minimal bool, riskPreview *governanceRiskPreview) error {
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
	if persisted.FirstRelease {
		// Names the baseline rather than leaving the reader to infer it from a
		// current version of 0.0.0. Planning used to fail outright here, so this is
		// also the line that says the situation was understood rather than guessed.
		fmt.Fprintf(w, "  Baseline:\tno previous release — all %d commits\n",
			output.ChangeSet.CommitCount())
	}
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
	if persisted.AlreadyExisted {
		// The run is mid-flight, so the fixed bump/notes/approve/publish list is
		// wrong — it would tell someone holding an approved release to start over.
		// getNextSteps is what `status` uses, so both commands give the same
		// answer for the same state.
		for i, step := range getNextSteps(persisted.ExistingState) {
			fmt.Printf("  %d. %s\n", i+1, step)
		}
	} else {
		fmt.Printf("  1. Run 'relicta bump' to bump to %s\n", output.NextVersion.String())
		fmt.Println("  2. Run 'relicta notes' to generate release notes")
		fmt.Println("  3. Run 'relicta approve' to review and approve")
		fmt.Println("  4. Run 'relicta publish' to execute the release")
	}
	fmt.Println()

	if !dryRun && persisted.ID != "" {
		if persisted.AlreadyExisted {
			// Nothing was written. Saying "saved" here is how an in-progress
			// release used to get quietly overwritten without anyone noticing.
			printSuccess(fmt.Sprintf("Existing release run %s is already at %s — nothing changed",
				persisted.ID, persisted.ExistingState))
			fmt.Println()
			fmt.Printf("  This plan matches that run exactly, so it was left untouched.\n")
			fmt.Printf("  Continue it with 'relicta status', or discard it with 'relicta cancel'.\n")
		} else {
			printSuccess(fmt.Sprintf("Release plan saved with ID: %s", persisted.ID))
		}
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
		// Governance is enabled by default (ADR-011), so its absence from the output
		// is a claim that needs a reason. Both failure paths here returned nil
		// silently, which made "governance is disabled", "governance failed" and
		// "governance found nothing" the same observable outcome: no governance
		// section at all. A reader could not tell an ungoverned release from a
		// governed one.
		printWarning(fmt.Sprintf("governance preview unavailable: could not build a "+
			"release to evaluate: %v", err))
		return nil
	}

	// Create actor (similar to approve.go)
	actor := createCGPActorForPlan()

	// Evaluate.
	//
	// A CGP proposal requires a repository identifier, and this passed the git
	// remote URL straight through. A repository with no remote therefore produced
	// "invalid scope: repository is required", governance never ran, and the error
	// was discarded — so every plan in a local-only repository came back with no
	// risk assessment while governance was enabled and reporting nothing wrong.
	//
	// The plan use case already has the convention for this: prefer the remote,
	// fall back to the path. Same order here, with the repository name in between
	// because it is the more readable identifier and is what the artifact's subject
	// shows.
	input := governance.EvaluateReleaseInput{
		Release:    rel,
		Actor:      actor,
		Repository: firstNonEmpty(repoURL, output.RepositoryName, "local"),
	}

	result, err := govService.EvaluateRelease(ctx, input)
	if err != nil {
		// Still not fatal — a plan is useful without a risk preview, and failing the
		// command would make governance a single point of failure for planning. But
		// it is said out loud, on stderr, so a pipeline that believes it is governed
		// finds out that this plan was not.
		printWarning(fmt.Sprintf("governance preview unavailable: %v", err))
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

		Factors:         result.RiskFactors,
		Rationale:       result.Rationale,
		RequiredActions: result.RequiredActions,
	}
}

// firstNonEmpty returns the first non-empty string, or "" if all are empty.
//
// The final fallback matters: an identifier that is merely readable is better than
// one that is absent, because absent means governance does not run at all.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
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

	persisted, err := persistReleaseRun(ctx, app, output, repoInfo)
	return persisted.ID, err
}

// persistReleaseRunOptions contains optional parameters for persistReleaseRun.
type persistReleaseRunOptions struct {
	TagPushMode bool   // If true, transition directly to versioned state
	TagName     string // The existing tag name (required if TagPushMode is true)
}

// persistReleaseRun stores the release run with pre-computed analysis data.
// This enables subsequent commands (bump, notes, approve, publish) to operate on the release.
// persistedRun describes what persisting the plan actually did.
//
// It carries more than the run ID because "saved a new plan" and "found the run
// you already have, mid-release" need different words. Returning only the ID
// made those indistinguishable, so plan reported success either way.
type persistedRun struct {
	ID             string
	AlreadyExisted bool
	ExistingState  domain.RunState

	// FirstRelease reports that no previous release was found, so the changeset is
	// the whole history. Said out loud because it changes what the numbers mean: a
	// reader who cannot tell this from an ordinary release sees a large commit count
	// as unusual activity rather than as the baseline.
	FirstRelease bool
}

func persistReleaseRun(ctx context.Context, app cliApp, output *servicerelease.AnalyzeOutput, repoInfo *sourcecontrol.RepositoryInfo) (persistedRun, error) {
	return persistReleaseRunWithOptions(ctx, app, output, repoInfo, persistReleaseRunOptions{})
}

// storeRecommendation writes the ADR-009 artifact next to the run it describes.
//
// Called after the governance preview rather than inside persistReleaseRun,
// because the run is saved before governance runs and an artifact stored at that
// point would carry no assessment — the lossy version this exists to avoid.
//
// A failure is reported and not fatal. The release is already planned and the
// artifact is a record of it; refusing to continue because the record could not
// be written would turn a storage problem into a blocked release. Silence would
// be worse than either, since the HTTP endpoint would then 404 with no
// explanation of why.
func storeRecommendation(app cliApp, output *servicerelease.AnalyzeOutput,
	riskPreview *governanceRiskPreview, persisted persistedRun, repoRoot string,
) {
	if persisted.ID == "" || !app.HasReleaseServices() {
		return
	}
	services := app.ReleaseServices()
	if services == nil || services.Repository == nil {
		return
	}

	artifact := buildRecommendationArtifact(output, riskPreview)
	if _, err := recommendation.Persist(services.Repository, repoRoot,
		domain.RunID(persisted.ID), artifact); err != nil {
		printWarning(fmt.Sprintf("recommendation artifact not stored: %v", err))
	}
}

// persistReleaseRunWithOptions stores the release run with optional tag-push mode support.
func persistReleaseRunWithOptions(ctx context.Context, app cliApp, output *servicerelease.AnalyzeOutput, repoInfo *sourcecontrol.RepositoryInfo, opts persistReleaseRunOptions) (persistedRun, error) {
	if err := app.InitReleaseServices(ctx, repoInfo.Path); err != nil {
		return persistedRun{}, fmt.Errorf("failed to initialize release services: %w", err)
	}

	if !app.HasReleaseServices() {
		return persistedRun{}, fmt.Errorf("release services not available")
	}

	services := app.ReleaseServices()
	if services == nil || services.PlanRelease == nil {
		return persistedRun{}, fmt.Errorf("PlanRelease use case not available")
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
		Force: true, // supersede stale runs whose plan hash no longer matches
		// Only an explicit `--force` discards a run that already exists for
		// these exact commits — see PlanReleaseInput.DiscardExisting.
		DiscardExisting: planForce,
		ChangeSet:       output.ChangeSet,
		CurrentVersion:  &output.CurrentVersion,
		NextVersion:     &output.NextVersion,
		BumpKind:        &bumpKind,
		TagPrefix:       configuredTagPrefix(),
		Confidence:      1.0, // Legacy analysis is authoritative
		TagPushMode:     opts.TagPushMode,
		TagName:         opts.TagName,
	}

	planOutput, err := services.PlanRelease.Execute(ctx, input)
	if err != nil {
		return persistedRun{}, err
	}
	return persistedRun{
		ID:             string(planOutput.RunID),
		AlreadyExisted: planOutput.AlreadyExisted,
		ExistingState:  planOutput.ExistingState,
		FirstRelease:   planOutput.FirstRelease,
	}, nil
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

// buildRecommendationArtifact assembles the deterministic recommendation
// artifact for JSON output (ADR-009).
//
// It is emitted alongside the existing keys rather than replacing them, so
// nothing consuming today's output breaks. Whether it eventually replaces the
// flattened fields is a separate decision.
func buildRecommendationArtifact(
	output *servicerelease.AnalyzeOutput,
	riskPreview *governanceRiskPreview,
) *recommendation.Artifact {
	in := recommendation.BuildInput{
		Now:            time.Now(),
		ToolVersion:    versionInfo.Version,
		Repository:     output.RepositoryName,
		Branch:         output.Branch,
		CurrentVersion: output.CurrentVersion.String(),
		NextVersion:    output.NextVersion.String(),
		ReleaseType:    output.ReleaseType.String(),
		ChangeSet:      output.ChangeSet,
	}

	// The digest claims to cover HEAD, so it has to be populated — otherwise two
	// different HEADs would share a digest and the determinism claim would be
	// false. The newest commit in the change set identifies what was analyzed;
	// ChangeSet.ToRef may be a symbolic name like "HEAD" rather than a SHA.
	if output.ChangeSet != nil {
		if commits := output.ChangeSet.Commits(); len(commits) > 0 && commits[0] != nil {
			in.HeadSHA = commits[0].Hash()
		}
	}

	if cfg != nil {
		in.Thresholds = &cfg.Governance
		in.PolicySource = cfg.Governance.PolicyDir
	}

	if riskPreview != nil {
		in.Governance = &recommendation.GovernanceInput{
			Decision:        riskPreview.Decision,
			RiskScore:       riskPreview.RiskScore,
			Severity:        riskPreview.Severity,
			RiskFactors:     riskPreview.Factors,
			Rationale:       riskPreview.Rationale,
			RequiredActions: riskPreview.RequiredActions,
			CanAutoApprove:  riskPreview.CanAutoApprove,
		}
	}

	return recommendation.Build(in)
}
