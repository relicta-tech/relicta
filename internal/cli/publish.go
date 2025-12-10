// Package cli provides the command-line interface for ReleasePilot.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	apprelease "github.com/felixgeelhaar/release-pilot/internal/application/release"
	"github.com/felixgeelhaar/release-pilot/internal/container"
	"github.com/felixgeelhaar/release-pilot/internal/domain/release"
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
		printInfo("Run 'release-pilot approve' to approve the release")
		return fmt.Errorf("release not approved")
	}

	// Check plan exists
	if rel.Plan() == nil {
		printError("Release has no plan")
		printInfo("Run 'release-pilot plan' to create a release plan")
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
	printInfo("Run 'release-pilot plan' to start a new release.")
	fmt.Println()
}

// runPublish implements the publish command.
func runPublish(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	printTitle("Release Publish")
	fmt.Println()

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

	// Display planned actions
	displayPublishActions(nextVersion.String())

	// Dry run check
	if dryRun {
		printWarning("Dry run - no changes will be made")
		return nil
	}

	// Execute publish use case
	input := buildPublishInput(rel)
	output, err := dddContainer.PublishRelease().Execute(ctx, input)
	if err != nil {
		printError(fmt.Sprintf("Failed to publish release: %v", err))
		return fmt.Errorf("failed to publish release: %w", err)
	}

	// Output results
	outputPublishResults(output)
	outputPluginResults(output.PluginResults)
	handleChangelogUpdate(rel)
	printPublishSummary(nextVersion.String(), output.TagName)

	return nil
}

// updateChangelogFile updates the changelog file with new content.
func updateChangelogFile(filename, newContent string) error {
	// Read existing content
	existingContent := ""
	if data, err := os.ReadFile(filename); err == nil {
		existingContent = string(data)
	}

	// Find insertion point (after header, before first version)
	// For keep-a-changelog format, we insert after "## [Unreleased]" or at the beginning
	var finalContent string

	if existingContent == "" {
		// New file
		finalContent = "# Changelog\n\nAll notable changes to this project will be documented in this file.\n\n" + newContent
	} else {
		// Insert new content after the header
		// Simple approach: find "## [" and insert before it
		insertPoint := 0
		lines := []byte(existingContent)

		// Skip past header lines
		for i := 0; i < len(lines); i++ {
			if i+4 < len(lines) && string(lines[i:i+4]) == "## [" {
				insertPoint = i
				break
			}
		}

		if insertPoint > 0 {
			finalContent = existingContent[:insertPoint] + newContent + "\n\n" + existingContent[insertPoint:]
		} else {
			// Append at end if no insertion point found
			finalContent = existingContent + "\n\n" + newContent
		}
	}

	return os.WriteFile(filename, []byte(finalContent), 0o644)
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
