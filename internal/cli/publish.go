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
	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/domain/release"
	releaseapp "github.com/relicta-tech/relicta/internal/domain/release/app"
	releasedomain "github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
)

var (
	publishSkipApproval bool
	publishSkipTag      bool
	publishSkipPush     bool
	publishSkipPlugins  bool
)

func init() {
	publishCmd.Flags().BoolVarP(&publishSkipApproval, "skip-approval", "A", false, "skip approval check")
	publishCmd.Flags().BoolVarP(&publishSkipTag, "skip-tag", "T", false, "skip git tag creation")
	publishCmd.Flags().BoolVarP(&publishSkipPush, "skip-push", "P", false, "skip pushing to remote")
	publishCmd.Flags().BoolVarP(&publishSkipPlugins, "skip-plugins", "G", false, "skip running plugins")
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

// outputStepResults outputs the results of step executions.
func outputStepResults(results []releaseapp.StepResult) {
	if len(results) == 0 {
		return
	}

	fmt.Println()
	printTitle("Step Results")
	fmt.Println()
	for _, result := range results {
		if result.Skipped {
			printInfo(fmt.Sprintf("  %s: skipped", result.StepName))
		} else if result.Success {
			printSuccess(fmt.Sprintf("  %s: %s", result.StepName, result.Output))
		} else {
			printError(fmt.Sprintf("  %s: %s", result.StepName, result.Error))
		}
	}
}

// handleChangelogUpdate updates the changelog file if configured.
func handleChangelogUpdate(rel *release.ReleaseRun) {
	if cfg.Changelog.File == "" || rel.Notes() == nil || rel.Notes().Text == "" {
		return
	}

	printInfo(fmt.Sprintf("Updating %s...", cfg.Changelog.File))
	if err := updateChangelogFile(cfg.Changelog.File, rel.Notes().Text); err != nil {
		printWarning(fmt.Sprintf("Failed to update changelog: %v", err))
	} else {
		printSuccess(fmt.Sprintf("Updated %s", cfg.Changelog.File))
	}
}

// printPublishSummary prints the final release summary.
func printPublishSummary(nextVersion, tagName string, remoteURL string) {
	fmt.Println()
	printTitle("Release Summary")
	fmt.Println()
	fmt.Printf("  Version:    %s%s\n", cfg.Versioning.TagPrefix, nextVersion)
	fmt.Printf("  Tag:        %s\n", tagName)
	fmt.Printf("  Status:     published\n")
	fmt.Printf("  Published:  %s\n", time.Now().Format(time.RFC3339))

	printSuccess("Release completed successfully!")

	// Show helpful hints for creating platform releases
	if !hasPlugin(cfg, "github") && isGitHubRemote(remoteURL) {
		fmt.Println()
		printInfo("To create a GitHub Release, either:")
		printSubtle("  • Run: relicta plugin install github")
		printSubtle("  • Or manually: gh release create " + tagName + " --generate-notes")
	}
	if !hasPlugin(cfg, "gitlab") && isGitLabRemote(remoteURL) {
		fmt.Println()
		printInfo("To create a GitLab Release, run: relicta plugin install gitlab")
	}

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
	app, err := newContainerApp(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer closeApp(app)

	// Get repository info for domain services
	gitAdapter := app.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	// Initialize domain services
	if err := app.InitReleaseServices(ctx, repoInfo.Path); err != nil {
		return fmt.Errorf("failed to initialize release services: %w", err)
	}
	if !app.HasReleaseServices() {
		return fmt.Errorf("release services not available")
	}
	services := app.ReleaseServices()
	if services == nil || services.PublishRelease == nil {
		return fmt.Errorf("PublishRelease use case not available")
	}

	return runPublishWithServices(ctx, app, repoInfo.Path, repoInfo.RemoteURL)
}

// runPublishWithServices publishes using the PublishReleaseUseCase.
func runPublishWithServices(ctx context.Context, app cliApp, repoPath, remoteURL string) error {
	services := app.ReleaseServices()

	// Load release from repository to get version
	run, err := services.Repository.LoadLatest(ctx, repoPath)
	if err != nil {
		return fmt.Errorf("failed to load release: %w", err)
	}

	nextVersion := run.VersionNext().String()

	// Output JSON if requested
	if outputJSON {
		return outputPublishJSONFromServices(run)
	}

	// Get governance evaluation for outcome tracking (if enabled)
	var govResult *governance.EvaluateReleaseOutput
	if app.HasGovernance() {
		// Load legacy release for governance (it reads from same path)
		if rel, err := getLatestRelease(ctx, app); err == nil {
			govResult, _ = evaluateGovernanceForPublish(ctx, app, rel)
			if govResult != nil && cfg.Governance.StrictMode && govResult.Decision == cgp.DecisionRejected {
				printError("Release blocked by governance policy")
				return fmt.Errorf("release denied by governance")
			}
		}
	}

	// Display planned actions
	displayPublishActions(nextVersion)

	// Dry run - skip actual changes
	if dryRun {
		return nil
	}

	// Track publish start time for duration recording
	publishStart := time.Now()

	// Execute publish use case with spinner
	spinner := NewSpinner("Publishing release...")
	spinner.Start()

	input := releaseapp.PublishReleaseInput{
		RepoRoot: repoPath,
		RunID:    run.ID(),
		Actor: ports.ActorInfo{
			Type: "user",
			ID:   "cli",
		},
		Force:  true, // Force since we already validated
		DryRun: false,
	}

	output, err := services.PublishRelease.Execute(ctx, input)

	spinner.Stop()

	if err != nil {
		printError(fmt.Sprintf("Failed to publish release: %v", err))
		// Record failure outcome to Release Memory
		if govResult != nil {
			if rel, relErr := getLatestRelease(ctx, app); relErr == nil {
				recordPublishOutcome(ctx, app, rel, govResult, false, time.Since(publishStart))
			}
		}
		return fmt.Errorf("failed to publish release: %w", err)
	}

	// Record success outcome to Release Memory
	if govResult != nil {
		if rel, relErr := getLatestRelease(ctx, app); relErr == nil {
			recordPublishOutcome(ctx, app, rel, govResult, true, time.Since(publishStart))
		}
	}

	// Output step results
	outputStepResults(output.StepResults)

	// Handle changelog update
	if rel, relErr := getLatestRelease(ctx, app); relErr == nil {
		handleChangelogUpdate(rel)
	}

	// Determine tag name from version
	tagName := cfg.Versioning.TagPrefix + nextVersion
	printPublishSummary(nextVersion, tagName, remoteURL)

	return nil
}

// evaluateGovernanceForPublish evaluates the release for governance tracking.
func evaluateGovernanceForPublish(ctx context.Context, app cliApp, rel *release.ReleaseRun) (*governance.EvaluateReleaseOutput, error) {
	govService := app.GovernanceService()
	if govService == nil {
		return nil, fmt.Errorf("governance service not available")
	}

	gitAdapter := app.GitAdapter()
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
func recordPublishOutcome(ctx context.Context, app cliApp, rel *release.ReleaseRun, govResult *governance.EvaluateReleaseOutput, success bool, duration time.Duration) {
	govService := app.GovernanceService()
	if govService == nil {
		return
	}

	gitAdapter := app.GitAdapter()
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
	if plan := release.GetPlan(rel); plan != nil && plan.HasChangeSet() {
		cats := plan.GetChangeSet().Categories()
		breakingChanges = len(cats.Breaking)
		filesChanged = plan.GetChangeSet().Summary().TotalCommits
	}

	input := governance.RecordOutcomeInput{
		ReleaseID:       rel.ID(),
		Repository:      repoInfo.Path,
		Version:         rel.Summary().VersionNext,
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

	return os.WriteFile(filename, []byte(finalContent), filePermReadable)
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

// outputPublishJSONFromServices outputs publish information as JSON from domain services.
func outputPublishJSONFromServices(run *releasedomain.ReleaseRun) error {
	output := map[string]any{
		"release_id":   string(run.ID()),
		"version":      run.VersionNext().String(),
		"tag_name":     cfg.Versioning.TagPrefix + run.VersionNext().String(),
		"state":        string(run.State()),
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

	if run.Notes() != nil && run.Notes().Text != "" {
		output["release_notes"] = run.Notes().Text
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
