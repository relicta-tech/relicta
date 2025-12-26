// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/domain/release"
)

var (
	cancelReason string
	cancelForce  bool
)

func init() {
	cancelCmd.Flags().StringVar(&cancelReason, "reason", "", "reason for canceling the release")
	cancelCmd.Flags().BoolVar(&cancelForce, "force", false, "force cancel even if in publishing state (not recommended)")

	resetCmd.Flags().BoolVar(&cancelForce, "force", false, "force reset even if release is in progress")
}

var cancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel the current release",
	Long: `Cancel the current in-progress release.

This command cancels a release that is in progress, allowing you to
start fresh with a new release cycle. Use this when:

  • You need to abort a release before publishing
  • The release has issues that require starting over
  • You want to discard the current release plan

After canceling, you can run 'relicta reset' to prepare for a new release,
or simply run 'relicta plan' to start a fresh release cycle.

Note: You cannot cancel a release that is currently being published.
To handle a failed publish, use 'relicta reset' instead.`,
	RunE: runCancel,
}

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset a failed or canceled release",
	Long: `Reset a release to allow starting fresh.

This command resets a release that has failed or been canceled,
clearing the error state and preparing for a new release attempt.

Use this when:
  • A publish operation failed and you want to retry
  • You canceled a release and want to start over
  • The release state is stuck and needs to be cleared

After resetting, run 'relicta plan' to start a new release cycle.`,
	RunE: runReset,
}

// runCancel implements the cancel command.
func runCancel(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	printTitle("Cancel Release")
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

	// Find the current release
	rel, err := findCurrentRelease(ctx, app)
	if err != nil {
		return err
	}

	// Validate the release can be canceled
	if err := validateCancelState(rel); err != nil {
		return err
	}

	// Get reason
	reason := cancelReason
	if reason == "" {
		reason = "canceled by user"
	}

	// Get current user for audit trail
	canceledBy := getCurrentUser()

	if dryRun {
		printInfo(fmt.Sprintf("Would cancel release %s", rel.ID()))
		printInfo(fmt.Sprintf("Reason: %s", reason))
		if outputJSON {
			return outputCancelJSON(rel, reason, true)
		}
		return nil
	}

	// Cancel the release
	if err := rel.Cancel(reason, canceledBy); err != nil {
		return fmt.Errorf("failed to cancel release: %w", err)
	}

	// Save the updated release
	releaseRepo := app.ReleaseRepository()
	if err := releaseRepo.Save(ctx, rel); err != nil {
		return fmt.Errorf("failed to save release state: %w", err)
	}

	if outputJSON {
		return outputCancelJSON(rel, reason, false)
	}

	printSuccess("Release canceled")
	printInfo(fmt.Sprintf("Reason: %s", reason))
	fmt.Println()
	printTitle("Next Steps")
	fmt.Println()
	fmt.Println("  • Run 'relicta reset' to prepare for a new release")
	fmt.Println("  • Or run 'relicta plan' to start fresh")
	fmt.Println()

	return nil
}

// runReset implements the reset command.
func runReset(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	printTitle("Reset Release")
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

	// Find the current release
	rel, err := findCurrentRelease(ctx, app)
	if err != nil {
		return err
	}

	// Validate the release can be reset
	if err := validateResetState(rel); err != nil {
		return err
	}

	previousState := rel.State()

	if dryRun {
		printInfo(fmt.Sprintf("Would reset release %s from state '%s'", rel.ID(), previousState))
		if outputJSON {
			return outputResetJSON(rel, previousState, true)
		}
		return nil
	}

	// Reset the release (retry from failed state)
	if err := rel.RetryPublish("cli"); err != nil {
		return fmt.Errorf("failed to reset release: %w", err)
	}

	// Save the updated release
	releaseRepo := app.ReleaseRepository()
	if err := releaseRepo.Save(ctx, rel); err != nil {
		return fmt.Errorf("failed to save release state: %w", err)
	}

	if outputJSON {
		return outputResetJSON(rel, previousState, false)
	}

	printSuccess(fmt.Sprintf("Release reset from '%s' to '%s'", previousState, rel.State()))
	fmt.Println()
	printTitle("Next Steps")
	fmt.Println()
	fmt.Println("  • Run 'relicta plan' to start a new release")
	fmt.Println()

	return nil
}

// findCurrentRelease finds the current/latest release for the repository.
func findCurrentRelease(ctx context.Context, app cliApp) (*release.Release, error) {
	gitAdapter := app.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	releaseRepo := app.ReleaseRepository()
	rel, err := releaseRepo.FindLatest(ctx, repoInfo.Path)
	if err != nil {
		if errors.Is(err, release.ErrReleaseNotFound) {
			printInfo("No active release found")
			printInfo("Nothing to cancel - there is no release in progress")
			return nil, err
		}
		return nil, fmt.Errorf("failed to find release: %w", err)
	}

	return rel, nil
}

// validateCancelState validates that the release can be canceled.
func validateCancelState(rel *release.Release) error {
	state := rel.State()

	switch state {
	case release.StatePublishing:
		if !cancelForce {
			printError("Cannot cancel a release that is currently being published")
			printInfo("Wait for publishing to complete, or use --force to override (not recommended)")
			return fmt.Errorf("release in state '%s' cannot be safely canceled", state)
		}
		printWarning("Force-canceling a release during publishing - this may leave things in an inconsistent state")
		return nil

	case release.StatePublished:
		printError("Release has already been published")
		printInfo("Published releases cannot be canceled")
		return fmt.Errorf("release in state '%s' is already complete", state)

	case release.StateFailed:
		printInfo("Release is already in failed state")
		printInfo("Use 'relicta reset' to prepare for a new release attempt")
		return fmt.Errorf("release is already failed - use 'relicta reset' instead")

	case release.StateCanceled:
		printInfo("Release is already canceled")
		printInfo("Use 'relicta reset' to prepare for a new release")
		return fmt.Errorf("release is already canceled - use 'relicta reset' instead")

	default:
		// All other states can be canceled
		return nil
	}
}

// validateResetState validates that the release can be reset.
func validateResetState(rel *release.Release) error {
	state := rel.State()

	switch state {
	case release.StateFailed, release.StateCanceled:
		// These states can be reset
		return nil

	case release.StatePublished:
		printError("Published releases cannot be reset")
		printInfo("Start a new release with 'relicta plan'")
		return fmt.Errorf("release in state '%s' is complete and cannot be reset", state)

	case release.StatePublishing:
		if !cancelForce {
			printError("Cannot reset a release that is currently being published")
			printInfo("Wait for publishing to complete or fail")
			return fmt.Errorf("release in state '%s' is currently being published", state)
		}
		printWarning("Force-resetting a release during publishing")
		return nil

	default:
		// For in-progress releases, suggest cancel instead
		printInfo(fmt.Sprintf("Release is in state '%s'", state))
		printInfo("Use 'relicta cancel' to cancel an in-progress release")
		printInfo("Then use 'relicta reset' to prepare for a new release")
		return fmt.Errorf("only failed or canceled releases can be reset - use 'relicta cancel' first")
	}
}

// getCurrentUser gets the current user for audit purposes.
func getCurrentUser() string {
	// Try common environment variables
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}
	if user := os.Getenv("GITHUB_ACTOR"); user != "" {
		return user
	}
	return "unknown"
}

// outputCancelJSON outputs the cancel result as JSON.
func outputCancelJSON(rel *release.Release, reason string, wasDryRun bool) error {
	output := map[string]any{
		"action":     "cancel",
		"release_id": string(rel.ID()),
		"state":      string(rel.State()),
		"reason":     reason,
		"dry_run":    wasDryRun,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// outputResetJSON outputs the reset result as JSON.
func outputResetJSON(rel *release.Release, previousState release.ReleaseState, wasDryRun bool) error {
	output := map[string]any{
		"action":         "reset",
		"release_id":     string(rel.ID()),
		"previous_state": string(previousState),
		"new_state":      string(rel.State()),
		"dry_run":        wasDryRun,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
