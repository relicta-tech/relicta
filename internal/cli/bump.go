// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/application/versioning"
	"github.com/relicta-tech/relicta/internal/domain/release"
	releaseapp "github.com/relicta-tech/relicta/internal/domain/release/app"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
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
	bumpCmd.Flags().StringVar(&bumpForce, "force", "", "set a specific version (e.g., 2.0.0), bypasses commit analysis")
	bumpCmd.Flags().StringVar(&bumpForce, "version", "", "alias for --force: set a specific version")
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
	// ErrRunNotFound is expected when bump runs standalone without prior plan
	if !dryRun {
		if err := updateReleaseVersion(ctx, app, forcedVersion); err != nil {
			if !errors.Is(err, release.ErrRunNotFound) {
				return fmt.Errorf("failed to update release state: %w", err)
			}
		}
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

	// Show spinner for tag operations (especially if pushing)
	var spinner *Spinner
	if !outputJSON {
		spinnerMsg := "Creating version tag..."
		if bumpPush {
			spinnerMsg = "Creating and pushing version tag..."
		}
		spinner = NewSpinner(spinnerMsg)
		spinner.Start()
	}

	setOutput, err := app.SetVersion().Execute(ctx, setInput)

	if spinner != nil {
		spinner.Stop()
	}

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

	// Check for tag-push mode (HEAD is already tagged)
	mode, existingVersion, err := detectReleaseMode(ctx, app, cfg.Versioning.TagPrefix)
	if err != nil {
		return fmt.Errorf("failed to detect release mode: %w", err)
	}

	if mode == releaseModeTagPush && existingVersion != nil {
		return runBumpTagPush(ctx, app, *existingVersion)
	}

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
	var spinner *Spinner
	if !outputJSON {
		spinner = NewSpinner("Calculating version...")
		spinner.Start()
	}

	calcInput := buildCalculateVersionInput(bumpType, auto)
	calcOutput, err := app.CalculateVersion().Execute(ctx, calcInput)

	if spinner != nil {
		spinner.Stop()
	}

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
	// ErrRunNotFound is expected when bump runs standalone without prior plan
	if err := updateReleaseVersion(ctx, app, nextVersion); err != nil {
		if !errors.Is(err, release.ErrRunNotFound) {
			return fmt.Errorf("failed to update release state: %w", err)
		}
	}

	// Output JSON after operations complete
	if outputJSON {
		return outputBumpJSON(calcOutput.CurrentVersion, nextVersion, calcOutput.BumpType, calcOutput.AutoDetected)
	}

	printBumpNextSteps()
	return nil
}

// runBumpTagPush handles the tag-push scenario where HEAD is already tagged.
// If the calculated version matches the existing tag, it skips creating a new tag.
// If the calculated version is different, it proceeds with the new version.
func runBumpTagPush(ctx context.Context, app cliApp, existingVer version.SemanticVersion) error {
	existingTag := cfg.Versioning.TagPrefix + existingVer.String()

	printInfo(fmt.Sprintf("HEAD is already tagged: %s", existingTag))

	// Calculate what version would be needed based on commits
	calcInput := buildCalculateVersionInput(version.BumpType(""), true)
	calcOutput, err := app.CalculateVersion().Execute(ctx, calcInput)
	if err != nil {
		// Can't calculate - use existing version
		printInfo("Could not calculate version, using existing tag")
		return finishBumpTagPush(ctx, app, existingVer, existingVer, false)
	}

	// Compare calculated version with existing tag
	if calcOutput.NextVersion.Compare(existingVer) == 0 {
		// Versions match - skip creating new tag
		printInfo("Calculated version matches existing tag - skipping bump")
		return finishBumpTagPush(ctx, app, existingVer, existingVer, false)
	}

	// Versions differ - proceed with new version
	printWarning(fmt.Sprintf("Calculated version %s differs from existing tag %s",
		calcOutput.NextVersion.String(), existingVer.String()))
	printInfo(fmt.Sprintf("Proceeding with calculated version: %s", calcOutput.NextVersion.String()))

	return finishBumpTagPush(ctx, app, existingVer, calcOutput.NextVersion, true)
}

// finishBumpTagPush completes the tag-push bump operation.
func finishBumpTagPush(ctx context.Context, app cliApp, existingVer, targetVer version.SemanticVersion, createNewTag bool) error {
	tagName := cfg.Versioning.TagPrefix + targetVer.String()
	existingTagName := cfg.Versioning.TagPrefix + existingVer.String()

	// Create new tag and delete old one if needed
	if createNewTag && !dryRun {
		// Delete the old tag first
		gitAdapter := app.GitAdapter()
		if err := gitAdapter.DeleteTag(ctx, existingTagName); err != nil {
			printWarning(fmt.Sprintf("Could not delete old tag %s: %v", existingTagName, err))
		} else {
			printInfo(fmt.Sprintf("Deleted old tag %s", existingTagName))
		}

		// Create new tag
		setInput := buildSetVersionInput(targetVer, bumpCreateTag, bumpPush, dryRun)
		setOutput, err := app.SetVersion().Execute(ctx, setInput)
		if err != nil {
			return fmt.Errorf("failed to create version tag: %w", err)
		}
		if setOutput.TagCreated {
			printSuccess(fmt.Sprintf("Created tag %s", setOutput.TagName))
		}
		if setOutput.TagPushed {
			printSuccess("Tag pushed to remote")
		}
	}

	// Update release state
	if !dryRun {
		if err := updateReleaseVersion(ctx, app, targetVer); err != nil {
			if errors.Is(err, release.ErrRunNotFound) {
				printInfo("No active release to update")
			} else {
				return fmt.Errorf("failed to update release state: %w", err)
			}
		} else {
			printSuccess(fmt.Sprintf("Release state updated with version %s", targetVer.String()))
		}
	}

	if outputJSON {
		output := map[string]any{
			"mode":            "tag-push",
			"existing_tag":    cfg.Versioning.TagPrefix + existingVer.String(),
			"current_version": existingVer.String(),
			"next_version":    targetVer.String(),
			"tag_name":        tagName,
			"tag_created":     createNewTag,
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(output)
	}

	// Show next steps
	fmt.Println()
	printTitle("Next Steps")
	fmt.Println()
	fmt.Println("  1. Run 'relicta notes' to generate release notes")
	fmt.Println("  2. Run 'relicta approve --yes' to approve the release")
	fmt.Println("  3. Run 'relicta publish --skip-push' to execute the release")
	fmt.Println()

	return nil
}

// updateReleaseVersion updates the active release with the bumped version.
func updateReleaseVersion(ctx context.Context, app cliApp, ver version.SemanticVersion) error {
	gitAdapter := app.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return err
	}

	// Initialize release services if not already done
	if !app.HasReleaseServices() {
		if err := app.InitReleaseServices(ctx, repoInfo.Path); err != nil {
			// Fall back to legacy behavior if init fails
			return updateReleaseVersionLegacy(ctx, app, repoInfo.Path, ver)
		}
	}

	services := app.ReleaseServices()
	if services == nil || services.BumpVersion == nil {
		return updateReleaseVersionLegacy(ctx, app, repoInfo.Path, ver)
	}

	tagName := cfg.Versioning.TagPrefix + ver.String()

	// Use BumpVersionUseCase
	input := releaseapp.BumpVersionInput{
		RepoRoot: repoInfo.Path,
		Actor: ports.ActorInfo{
			Type: "user",
			ID:   "cli",
		},
		Force:           true, // Force since git operations already happened
		OverrideVersion: &ver,
		OverrideTagName: tagName,
	}

	_, err = services.BumpVersion.Execute(ctx, input)
	if err != nil {
		// If the error is because run is not in Planned state, try legacy
		// This handles the case where bump runs standalone without prior plan
		return updateReleaseVersionLegacy(ctx, app, repoInfo.Path, ver)
	}

	return nil
}

// updateReleaseVersionLegacy is the fallback using direct repository access.
func updateReleaseVersionLegacy(ctx context.Context, app cliApp, repoPath string, ver version.SemanticVersion) error {
	releaseRepo := app.ReleaseRepository()
	rel, err := releaseRepo.FindLatest(ctx, repoPath)
	if err != nil {
		return err
	}

	tagName := cfg.Versioning.TagPrefix + ver.String()
	if err := rel.SetVersion(ver, tagName); err != nil {
		return err
	}

	// Transition from Planned to Versioned state
	if err := rel.Bump("cli"); err != nil {
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
