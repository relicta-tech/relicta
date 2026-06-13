// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
)

var (
	rollbackToVersion string
	rollbackToTag     string
	rollbackDryRun    bool
)

func init() {
	rollbackCmd.Flags().StringVar(&rollbackToVersion, "to-version", "", "target version to roll back to (e.g., 1.2.3)")
	rollbackCmd.Flags().StringVar(&rollbackToTag, "to-tag", "", "target git tag to roll back to (e.g., v1.2.3)")
	rollbackCmd.Flags().BoolVar(&rollbackDryRun, "dry-run", false, "simulate the rollback without making changes")

	rootCmd.AddCommand(rollbackCmd)
}

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Roll back to a previous release version",
	Long: `Roll back to a previous release version by creating a revert tag.

This command validates that the target version exists as a git tag,
creates a new tag pointing to the same commit as the target version,
and records the rollback event in the audit trail.

Examples:
  relicta rollback --to-version 1.2.3
  relicta rollback --to-tag v1.2.3
  relicta rollback --to-version 1.2.3 --dry-run`,
	RunE: runRollback,
}

// runRollback implements the rollback command.
func runRollback(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	printTitle("Rollback Release")
	fmt.Println()

	// Validate flags
	targetTag, err := resolveRollbackTarget()
	if err != nil {
		return err
	}

	effectiveDryRun := dryRun || rollbackDryRun
	if effectiveDryRun {
		printDryRunBanner()
	}

	// Enforce the per-actor autonomy budget for the real (non-dry-run)
	// rollback. Rollback mutates production state and has no per-run risk
	// score in hand, so it is gated as a critical-blast-radius operation —
	// restrictive (agent/CI) budgets block it, permissive (human) allow.
	if !effectiveDryRun {
		if err := enforceActorBudget("rollback", 1.0); err != nil {
			return err
		}
	}

	// Initialize container
	app, err := newContainerApp(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer closeApp(app)

	// Execute rollback
	result, err := executeRollback(ctx, app, targetTag, effectiveDryRun)
	if err != nil {
		return err
	}

	if outputJSON {
		return outputRollbackJSON(result)
	}

	if effectiveDryRun {
		printInfo(fmt.Sprintf("Would roll back to %s (commit %s)", result.TargetTag, result.TargetCommit))
		printInfo(fmt.Sprintf("Would create revert tag: %s", result.RevertTag))
		return nil
	}

	printSuccess(fmt.Sprintf("Rolled back to %s", result.TargetTag))
	printInfo(fmt.Sprintf("Created revert tag: %s", result.RevertTag))
	printInfo(fmt.Sprintf("Target commit: %s", result.TargetCommit))
	fmt.Println()
	printTitle("Next Steps")
	fmt.Println()
	fmt.Println("  • Push the revert tag: git push origin " + result.RevertTag)
	fmt.Println("  • Verify the rollback: relicta status")
	fmt.Println()

	return nil
}

// resolveRollbackTarget determines the target tag from flags.
func resolveRollbackTarget() (string, error) {
	if rollbackToVersion == "" && rollbackToTag == "" {
		return "", fmt.Errorf("either --to-version or --to-tag must be specified")
	}

	if rollbackToVersion != "" && rollbackToTag != "" {
		return "", fmt.Errorf("--to-version and --to-tag are mutually exclusive")
	}

	if rollbackToTag != "" {
		return rollbackToTag, nil
	}

	// Convert version to tag format
	return "v" + rollbackToVersion, nil
}

// rollbackResult holds the result of a rollback operation.
type rollbackResult struct {
	TargetTag    string `json:"target_tag"`
	TargetCommit string `json:"target_commit"`
	RevertTag    string `json:"revert_tag"`
	DryRun       bool   `json:"dry_run"`
	RolledBackBy string `json:"rolled_back_by"`
	RolledBackAt string `json:"rolled_back_at"`
}

// executeRollback performs the rollback operation.
func executeRollback(ctx context.Context, app cliApp, targetTag string, isDryRun bool) (*rollbackResult, error) {
	gitAdapter := app.GitAdapter()

	// Validate target tag exists
	tag, err := gitAdapter.GetTag(ctx, targetTag)
	if err != nil {
		return nil, fmt.Errorf("failed to find tag %q: %w", targetTag, err)
	}
	if tag == nil {
		return nil, fmt.Errorf("tag %q does not exist", targetTag)
	}

	targetCommit := string(tag.Hash())

	// Generate the revert tag name
	revertTag := fmt.Sprintf("rollback-to-%s-%s", targetTag, time.Now().UTC().Format("20060102-150405"))

	// Get current user for audit trail
	user := getCurrentUser()

	result := &rollbackResult{
		TargetTag:    targetTag,
		TargetCommit: targetCommit,
		RevertTag:    revertTag,
		DryRun:       isDryRun,
		RolledBackBy: user,
		RolledBackAt: time.Now().UTC().Format(time.RFC3339),
	}

	if isDryRun {
		return result, nil
	}

	// Create the revert tag pointing to the target commit
	message := fmt.Sprintf("Rollback to %s by %s at %s", targetTag, user, result.RolledBackAt)
	_, err = gitAdapter.CreateTag(ctx, revertTag, sourcecontrol.CommitHash(targetCommit), message)
	if err != nil {
		return nil, fmt.Errorf("failed to create revert tag: %w", err)
	}

	// Record the rollback in the release audit trail if possible
	recordRollbackAudit(ctx, app, result)

	return result, nil
}

// recordRollbackAudit records the rollback event in the release repository.
func recordRollbackAudit(ctx context.Context, app cliApp, result *rollbackResult) {
	releaseRepo := app.ReleaseRepository()
	if releaseRepo == nil {
		return
	}

	// Try to find the latest release to record the rollback
	gitAdapter := app.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		logger.Warn("failed to get repo info for rollback audit", "error", err)
		return
	}

	rel, err := releaseRepo.FindLatest(ctx, repoInfo.Path)
	if err != nil {
		// No active release to annotate - this is fine
		logger.Debug("no active release to record rollback event", "error", err)
		return
	}

	// Cancel the current release with rollback reason
	reason := fmt.Sprintf("rolled back to %s by %s", result.TargetTag, result.RolledBackBy)
	if err := rel.Cancel(reason, result.RolledBackBy); err != nil {
		logger.Debug("could not cancel release during rollback", "error", err)
		return
	}

	if err := releaseRepo.Save(ctx, rel); err != nil {
		logger.Warn("failed to save rollback audit event", "error", err)
	}
}

// outputRollbackJSON outputs the rollback result as JSON.
func outputRollbackJSON(result *rollbackResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
