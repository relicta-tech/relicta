// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/application/governance"
	apprelease "github.com/relicta-tech/relicta/internal/application/release"
	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/container"
	"github.com/relicta-tech/relicta/internal/domain/release"
)

var (
	publishSkipApproval bool
	publishSkipTag      bool
	publishSkipPush     bool
	publishSkipPlugins  bool
)

func init() {
	publishCmd.Flags().BoolVar(&publishSkipApproval, "skip-approval", false, "skip approval check")
	publishCmd.Flags().BoolVar(&publishSkipTag, "skip-tag", false, "skip git tag creation")
	publishCmd.Flags().BoolVar(&publishSkipPush, "skip-push", false, "skip pushing to remote")
	publishCmd.Flags().BoolVar(&publishSkipPlugins, "skip-plugins", false, "skip running plugins")
}

// validateReleaseForPublish validates that the release is ready for publishing.
func validateReleaseForPublish(rel *release.Release) error {
	// Check approval
	if cfg.Workflow.RequireApproval && !rel.IsApproved() && !publishSkipApproval {
		printError("Release not approved")
		printInfo("Run 'relicta approve' to approve the release")
		return fmt.Errorf("release not approved")
	}

	// Check plan exists
	if rel.Plan() == nil {
		printError("Release has no plan")
		printInfo("Run 'relicta plan' to create a release plan")
		return fmt.Errorf("no release plan found")
	}

	return nil
}

// shouldCreateTag returns whether a tag should be created.
func shouldCreateTag() bool {
	return !publishSkipTag && cfg.Versioning.GitTag
}

// shouldPushTag returns whether the tag should be pushed.
func shouldPushTag() bool {
	return !publishSkipPush && cfg.Versioning.GitPush
}

// shouldRunPlugins returns whether plugins should be executed.
func shouldRunPlugins() bool {
	return !publishSkipPlugins && len(cfg.Plugins) > 0
}

// displayPublishActions displays what actions will be performed.
func displayPublishActions(nextVersion string) {
	fmt.Println()
	printTitle("Release Actions")
	fmt.Println()
	fmt.Printf("  Version:    %s%s\n", cfg.Versioning.TagPrefix, nextVersion)
	fmt.Printf("  Create tag: %v\n", shouldCreateTag())
	fmt.Printf("  Push:       %v\n", shouldPushTag())
	fmt.Printf("  Plugins:    %v\n", shouldRunPlugins())
	fmt.Println()
}

// buildPublishInput creates the input for the PublishRelease use case.
func buildPublishInput(rel *release.Release) apprelease.PublishReleaseInput {
	return apprelease.PublishReleaseInput{
		ReleaseID: rel.ID(),
		DryRun:    dryRun,
		CreateTag: shouldCreateTag(),
		PushTag:   shouldPushTag(),
		TagPrefix: cfg.Versioning.TagPrefix,
		Remote:    "origin",
	}
}

// outputPublishResults outputs the results of the publish operation.
func outputPublishResults(output *apprelease.PublishReleaseOutput) {
	if output.TagName != "" {
		printSuccess(fmt.Sprintf("Created tag %s", output.TagName))
	}

	if output.ReleaseURL != "" {
		printSuccess(fmt.Sprintf("Release URL: %s", output.ReleaseURL))
	}
}

// outputPluginResults outputs the results of plugin executions.
func outputPluginResults(results []apprelease.PluginResult) {
	if len(results) == 0 {
		return
	}

	fmt.Println()
	printTitle("Plugin Results")
	fmt.Println()
	for _, result := range results {
		if result.Success {
			printSuccess(fmt.Sprintf("  %s: %s", result.PluginName, result.Message))
		} else {
			printError(fmt.Sprintf("  %s: %s", result.PluginName, result.Message))
		}
	}
}

// handleChangelogUpdate updates the changelog file if configured.
func handleChangelogUpdate(rel *release.Release) {
	if cfg.Changelog.File == "" || rel.Notes() == nil || rel.Notes().Changelog == "" {
		return
	}

	printInfo(fmt.Sprintf("Updating %s...", cfg.Changelog.File))
	if err := updateChangelogFile(cfg.Changelog.File, rel.Notes().Changelog); err != nil {
		printWarning(fmt.Sprintf("Failed to update changelog: %v", err))
	} else {
		printSuccess(fmt.Sprintf("Updated %s", cfg.Changelog.File))
	}
}

// printPublishSummary prints the final release summary.
func printPublishSummary(nextVersion, tagName string) {
	fmt.Println()
	printTitle("Release Summary")
	fmt.Println()
	fmt.Printf("  Version:    %s%s\n", cfg.Versioning.TagPrefix, nextVersion)
	fmt.Printf("  Tag:        %s\n", tagName)
	fmt.Printf("  Status:     published\n")
	fmt.Printf("  Published:  %s\n", time.Now().Format(time.RFC3339))

	printSuccess("Release completed successfully!")
	fmt.Println()
	printInfo("Run 'relicta plan' to start a new release.")
	fmt.Println()
}

// runPublish implements the publish command.
func runPublish(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	printTitle("Release Publish")
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

	// Get latest release (reuse helper from approve.go)
	rel, err := getLatestRelease(ctx, dddContainer)
	if err != nil {
		return err
	}

	// Validate release state
	if err := validateReleaseForPublish(rel); err != nil {
		return err
	}

	nextVersion := rel.Plan().NextVersion

	// Output JSON if requested
	if outputJSON {
		return outputPublishJSON(rel)
	}

	// Get governance evaluation for outcome tracking (if enabled)
	var govResult *governance.EvaluateReleaseOutput
	if dddContainer.HasGovernance() {
		govResult, err = evaluateGovernanceForPublish(ctx, dddContainer, rel)
		if err != nil {
			// Non-fatal - just log warning
			printWarning(fmt.Sprintf("Governance evaluation failed: %v", err))
		} else if cfg.Governance.StrictMode && govResult.Decision == cgp.DecisionRejected {
			// In strict mode, block rejected releases even at publish time
			printError("Release blocked by governance policy")
			return fmt.Errorf("release denied by governance")
		}
	}

	// Display planned actions
	displayPublishActions(nextVersion.String())

	// Dry run - skip actual changes
	if dryRun {
		return nil
	}

	// Track publish start time for duration recording
	publishStart := time.Now()

	// Execute publish use case with spinner
	spinner := NewSpinner("Publishing release...")
	spinner.Start()

	input := buildPublishInput(rel)
	output, err := dddContainer.PublishRelease().Execute(ctx, input)

	spinner.Stop()

	if err != nil {
		printError(fmt.Sprintf("Failed to publish release: %v", err))
		// Record failure outcome to Release Memory
		recordPublishOutcome(ctx, dddContainer, rel, govResult, false, time.Since(publishStart))
		return fmt.Errorf("failed to publish release: %w", err)
	}

	// Record success outcome to Release Memory
	recordPublishOutcome(ctx, dddContainer, rel, govResult, true, time.Since(publishStart))

	// Output results
	outputPublishResults(output)
	outputPluginResults(output.PluginResults)
	handleChangelogUpdate(rel)
	printPublishSummary(nextVersion.String(), output.TagName)

	return nil
}

// evaluateGovernanceForPublish evaluates the release for governance tracking.
func evaluateGovernanceForPublish(ctx context.Context, dddContainer *container.DDDContainer, rel *release.Release) (*governance.EvaluateReleaseOutput, error) {
	govService := dddContainer.GovernanceService()
	if govService == nil {
		return nil, fmt.Errorf("governance service not available")
	}

	gitAdapter := dddContainer.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	actor := createCGPActor()

	input := governance.EvaluateReleaseInput{
		Release:        rel,
		Actor:          actor,
		Repository:     repoInfo.Path,
		IncludeHistory: cfg.Governance.MemoryEnabled,
	}

	return govService.EvaluateRelease(ctx, input)
}

// recordPublishOutcome records the actual publish outcome to Release Memory.
func recordPublishOutcome(ctx context.Context, dddContainer *container.DDDContainer, rel *release.Release, govResult *governance.EvaluateReleaseOutput, success bool, duration time.Duration) {
	govService := dddContainer.GovernanceService()
	if govService == nil {
		return
	}

	gitAdapter := dddContainer.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return
	}

	// Determine outcome
	outcome := governance.OutcomeSuccess
	if !success {
		outcome = governance.OutcomeFailure
	}

	actor := createCGPActor()

	// Get risk info from governance result or use defaults
	var riskScore float64
	var decision cgp.DecisionType
	var breakingChanges, securityChanges, filesChanged int

	if govResult != nil {
		riskScore = govResult.RiskScore
		decision = govResult.Decision
	}

	// Extract change metrics from release plan
	if rel.Plan() != nil && rel.Plan().HasChangeSet() {
		cats := rel.Plan().GetChangeSet().Categories()
		breakingChanges = len(cats.Breaking)
		filesChanged = rel.Plan().GetChangeSet().Summary().TotalCommits
	}

	input := governance.RecordOutcomeInput{
		ReleaseID:       rel.ID(),
		Repository:      repoInfo.Path,
		Version:         rel.Summary().NextVersion,
		Actor:           actor,
		RiskScore:       riskScore,
		Decision:        decision,
		BreakingChanges: breakingChanges,
		SecurityChanges: securityChanges,
		FilesChanged:    filesChanged,
		Outcome:         outcome,
		Duration:        duration,
	}

	if err := govService.RecordReleaseOutcome(ctx, input); err != nil {
		printWarning(fmt.Sprintf("Failed to record publish outcome: %v", err))
	}
}

// updateChangelogFile updates the changelog file with new content.
func updateChangelogFile(filename, newContent string) error {
	// Strip any "# Changelog" header from the new content if present
	// This handles cases where the content was generated with a header
	newContent = stripChangelogHeader(newContent)

	// Read existing content
	existingContent := ""
	if data, err := os.ReadFile(filename); err == nil { // #nosec G304 -- user-specified changelog path
		existingContent = string(data)
	}

	var finalContent string

	if existingContent == "" {
		// New file - create with standard header
		finalContent = "# Changelog\n\nAll notable changes to this project will be documented in this file.\n\n" + newContent + "\n"
	} else {
		// Find the first version entry (## [x.y.z] or ## [Unreleased])
		// Insert new content before it
		insertPoint := findVersionEntryPoint(existingContent)

		if insertPoint > 0 {
			finalContent = existingContent[:insertPoint] + newContent + "\n\n" + existingContent[insertPoint:]
		} else {
			// No existing version entries found, append after header
			finalContent = existingContent + "\n\n" + newContent + "\n"
		}
	}

	return os.WriteFile(filename, []byte(finalContent), 0o644) // #nosec G306 -- changelog readable by user
}

// stripChangelogHeader removes any "# Changelog" header from the content.
func stripChangelogHeader(content string) string {
	lines := strings.Split(content, "\n")
	startIdx := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip "# Changelog" or similar headers
		if strings.HasPrefix(trimmed, "# ") && strings.Contains(strings.ToLower(trimmed), "changelog") {
			startIdx = i + 1
			continue
		}
		// Skip empty lines after the header
		if startIdx > 0 && trimmed == "" && i == startIdx {
			startIdx = i + 1
			continue
		}
		// Found actual content
		if trimmed != "" {
			break
		}
	}

	if startIdx > 0 && startIdx < len(lines) {
		return strings.Join(lines[startIdx:], "\n")
	}
	return content
}

// findVersionEntryPoint finds the byte position of the first version entry in the changelog.
func findVersionEntryPoint(content string) int {
	lines := strings.Split(content, "\n")
	pos := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Look for version entries: "## [x.y.z]" or "## [Unreleased]"
		if strings.HasPrefix(trimmed, "## [") {
			return pos
		}
		pos += len(line) + 1 // +1 for newline
	}

	return 0
}

// outputPublishJSON outputs the publish information as JSON.
func outputPublishJSON(rel *release.Release) error {
	plan := rel.Plan()

	output := map[string]any{
		"release_id":   string(rel.ID()),
		"version":      plan.NextVersion.String(),
		"tag_name":     cfg.Versioning.TagPrefix + plan.NextVersion.String(),
		"approved":     rel.IsApproved(),
		"state":        rel.State().String(),
		"dry_run":      dryRun,
		"ci_mode":      ciMode,
		"skip_tag":     publishSkipTag,
		"skip_push":    publishSkipPush,
		"skip_plugins": publishSkipPlugins,
		"actions": map[string]bool{
			"create_tag":  !publishSkipTag && cfg.Versioning.GitTag,
			"push_tag":    !publishSkipPush && cfg.Versioning.GitPush,
			"run_plugins": !publishSkipPlugins,
		},
	}

	if rel.Notes() != nil && rel.Notes().Changelog != "" {
		output["release_notes"] = rel.Notes().Changelog
	}

	if len(cfg.Plugins) > 0 {
		var plugins []string
		for _, p := range cfg.Plugins {
			if p.IsEnabled() {
				plugins = append(plugins, p.Name)
			}
		}
		output["plugins"] = plugins
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
