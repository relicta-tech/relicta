// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/application/versioning"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

var (
	bumpLevel      string
	bumpPrerelease string
	bumpBuild      string
	bumpForce      string
	bumpCreateTag  bool
	bumpPush       bool
)

func init() {
	bumpCmd.Flags().StringVar(&bumpLevel, "level", "", "bump level (major, minor, patch) - overrides auto-detection")
	bumpCmd.Flags().StringVar(&bumpPrerelease, "prerelease", "", "prerelease identifier (e.g., alpha, beta, rc.1)")
	bumpCmd.Flags().StringVar(&bumpBuild, "build", "", "build metadata")
	bumpCmd.Flags().StringVar(&bumpForce, "force", "", "force a specific version (e.g., 2.0.0)")
	bumpCmd.Flags().BoolVar(&bumpCreateTag, "tag", true, "create git tag")
	bumpCmd.Flags().BoolVar(&bumpPush, "push", false, "push tag to remote")
}

// parseBumpLevel parses the bump level flag and returns the bump type and whether auto-detection should be used.
func parseBumpLevel(level string) (version.BumpType, bool, error) {
	switch level {
	case "major":
		return version.BumpMajor, false, nil
	case "minor":
		return version.BumpMinor, false, nil
	case "patch":
		return version.BumpPatch, false, nil
	case "":
		return version.BumpType(""), true, nil // Will auto-detect from commits
	default:
		return version.BumpType(""), false, fmt.Errorf("invalid bump level: %s (expected major, minor, or patch)", level)
	}
}

// buildSetVersionInput creates the input for the SetVersion use case.
func buildSetVersionInput(ver version.SemanticVersion, createTag, pushTag, dryRunMode bool) versioning.SetVersionInput {
	return versioning.SetVersionInput{
		Version:    ver,
		TagPrefix:  cfg.Versioning.TagPrefix,
		CreateTag:  createTag && cfg.Versioning.GitTag,
		PushTag:    pushTag && cfg.Versioning.GitPush,
		Remote:     "origin",
		TagMessage: fmt.Sprintf("Release %s", ver.String()),
		DryRun:     dryRunMode,
	}
}

// outputSetVersionResult outputs the result of a SetVersion operation as text.
func outputSetVersionResult(output *versioning.SetVersionOutput) {
	printInfo(fmt.Sprintf("Version set to: %s%s", cfg.Versioning.TagPrefix, output.Version.String()))
	if output.TagCreated {
		printSuccess(fmt.Sprintf("Created tag %s", output.TagName))
	}
	if output.TagPushed {
		printSuccess("Tag pushed to remote")
	}
}

// handleForcedVersion handles the --force flag to set a specific version.
func handleForcedVersion(ctx context.Context, app cliApp, forcedVersionStr string) error {
	forcedVersion, err := version.Parse(forcedVersionStr)
	if err != nil {
		return fmt.Errorf("invalid version format: %w", err)
	}

	setInput := buildSetVersionInput(forcedVersion, bumpCreateTag, bumpPush, dryRun)

	output, err := app.SetVersion().Execute(ctx, setInput)
	if err != nil {
		return fmt.Errorf("failed to set version: %w", err)
	}

	// Update release state if there's an active release (same as normal bump flow)
	// Not fatal if it fails - just means there's no release to update
	if !dryRun {
		_ = updateReleaseVersion(ctx, app, forcedVersion)
	}

	if outputJSON {
		return outputSetVersionJSON(output)
	}

	outputSetVersionResult(output)
	return nil
}

// buildCalculateVersionInput creates the input for the CalculateVersion use case.
func buildCalculateVersionInput(bumpType version.BumpType, auto bool) versioning.CalculateVersionInput {
	input := versioning.CalculateVersionInput{
		TagPrefix: cfg.Versioning.TagPrefix,
		BumpType:  bumpType,
		Auto:      auto,
	}

	if bumpPrerelease != "" {
		input.Prerelease = version.Prerelease(bumpPrerelease)
	}

	return input
}

// outputCalculatedVersionText outputs the calculated version information as text.
func outputCalculatedVersionText(calcOutput *versioning.CalculateVersionOutput, nextVersion version.SemanticVersion) {
	printInfo(fmt.Sprintf("Current version: %s%s", cfg.Versioning.TagPrefix, calcOutput.CurrentVersion.String()))
	printInfo(fmt.Sprintf("Next version:    %s%s", cfg.Versioning.TagPrefix, nextVersion.String()))
	printInfo(fmt.Sprintf("Bump type:       %s", calcOutput.BumpType.String()))
	if calcOutput.AutoDetected {
		printInfo("Bump type was auto-detected from commits")
	}
	fmt.Println()
}

// applyVersionTag creates and optionally pushes the version tag.
func applyVersionTag(ctx context.Context, app cliApp, nextVersion version.SemanticVersion) error {
	if !bumpCreateTag || !cfg.Versioning.GitTag {
		return nil
	}

	setInput := buildSetVersionInput(nextVersion, true, bumpPush, false)

	setOutput, err := app.SetVersion().Execute(ctx, setInput)
	if err != nil {
		return fmt.Errorf("failed to set version: %w", err)
	}

	if setOutput.TagCreated {
		printSuccess(fmt.Sprintf("Created tag %s", setOutput.TagName))
	}
	if setOutput.TagPushed {
		printSuccess("Tag pushed to remote")
	}

	return nil
}

// printBumpNextSteps prints the next steps after a version bump.
func printBumpNextSteps() {
	fmt.Println()
	printTitle("Next Steps")
	fmt.Println()
	fmt.Println("  1. Run 'relicta notes' to generate release notes")
	fmt.Println("  2. Run 'relicta approve' to review and approve")
	fmt.Println("  3. Run 'relicta publish' to execute the release")
	fmt.Println()
}

// runVersion implements the version/bump command.
func runVersion(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	printTitle("Version Bump")
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

	// Parse bump type from flag
	bumpType, auto, err := parseBumpLevel(bumpLevel)
	if err != nil {
		return err
	}

	// Handle forced version separately
	if bumpForce != "" {
		return handleForcedVersion(ctx, app, bumpForce)
	}

	// Calculate version
	calcInput := buildCalculateVersionInput(bumpType, auto)
	calcOutput, err := app.CalculateVersion().Execute(ctx, calcInput)
	if err != nil {
		return fmt.Errorf("failed to calculate version: %w", err)
	}

	// Apply build metadata if provided
	nextVersion := calcOutput.NextVersion
	if bumpBuild != "" {
		nextVersion = nextVersion.WithMetadata(version.BuildMetadata(bumpBuild))
	}

	// Output text results (skip for JSON mode)
	if !outputJSON {
		outputCalculatedVersionText(calcOutput, nextVersion)
	}

	// Dry run - skip actual changes but still output JSON if requested
	if dryRun {
		if outputJSON {
			return outputBumpJSON(calcOutput.CurrentVersion, nextVersion, calcOutput.BumpType, calcOutput.AutoDetected)
		}
		return nil
	}

	// Apply version tag
	if err := applyVersionTag(ctx, app, nextVersion); err != nil {
		return err
	}

	// Update release state if there's an active release
	// Not fatal if it fails - just means there's no release to update
	// This can happen when bump is run standalone
	_ = updateReleaseVersion(ctx, app, nextVersion)

	// Output JSON after operations complete
	if outputJSON {
		return outputBumpJSON(calcOutput.CurrentVersion, nextVersion, calcOutput.BumpType, calcOutput.AutoDetected)
	}

	printBumpNextSteps()
	return nil
}

// updateReleaseVersion updates the active release with the bumped version.
func updateReleaseVersion(ctx context.Context, app cliApp, ver version.SemanticVersion) error {
	gitAdapter := app.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return err
	}

	releaseRepo := app.ReleaseRepository()
	rel, err := releaseRepo.FindLatest(ctx, repoInfo.Path)
	if err != nil {
		return err
	}

	tagName := cfg.Versioning.TagPrefix + ver.String()
	if err := rel.SetVersion(ver, tagName); err != nil {
		return err
	}

	return releaseRepo.Save(ctx, rel)
}

// outputBumpJSON outputs the version bump as JSON.
func outputBumpJSON(current, next version.SemanticVersion, bumpType version.BumpType, autoDetected bool) error {
	output := map[string]any{
		"current_version": current.String(),
		"next_version":    next.String(),
		"bump_type":       bumpType.String(),
		"auto_detected":   autoDetected,
		"tag_name":        cfg.Versioning.TagPrefix + next.String(),
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// outputSetVersionJSON outputs the set version result as JSON.
func outputSetVersionJSON(output *versioning.SetVersionOutput) error {
	result := map[string]any{
		"version":     output.Version.String(),
		"tag_name":    output.TagName,
		"tag_created": output.TagCreated,
		"tag_pushed":  output.TagPushed,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
