// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/domain/release"
)

var (
	cleanKeepLast   int
	cleanOlderThan  string
	cleanAll        bool
	cleanDryRunFlag bool
)

func init() {
	cleanCmd.Flags().IntVarP(&cleanKeepLast, "keep", "k", 10, "keep the last N release runs (default: 10)")
	cleanCmd.Flags().StringVarP(&cleanOlderThan, "older-than", "o", "", "remove runs older than duration (e.g., 7d, 30d)")
	cleanCmd.Flags().BoolVarP(&cleanAll, "all", "a", false, "remove all release runs except the latest")
	cleanCmd.Flags().BoolVar(&cleanDryRunFlag, "dry-run", false, "show what would be deleted without deleting")

	rootCmd.AddCommand(cleanCmd)
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up old release runs",
	Long: `Clean up old or stale release runs from the .relicta/releases directory.

This command helps manage disk space and reduce clutter by removing
old release runs that are no longer needed. By default, it keeps the
last 10 runs and removes older ones.

Examples:
  relicta clean                     # Keep last 10 runs, remove others
  relicta clean --keep 5            # Keep last 5 runs
  relicta clean --older-than 30d    # Remove runs older than 30 days
  relicta clean --all               # Remove all except the latest
  relicta clean --dry-run           # Show what would be deleted

The command will never delete an active (in-progress) release run.
Use 'relicta cancel' to cancel an active release first.`,
	RunE: runClean,
}

// cleanResult holds the result of the clean operation.
type cleanResult struct {
	TotalRuns     int      `json:"total_runs"`
	DeletedCount  int      `json:"deleted_count"`
	DeletedIDs    []string `json:"deleted_ids"`
	KeptCount     int      `json:"kept_count"`
	KeptIDs       []string `json:"kept_ids"`
	SkippedActive int      `json:"skipped_active"`
	DryRun        bool     `json:"dry_run"`
}

func runClean(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	printTitle("Clean Release Runs")
	fmt.Println()

	// Use dry-run from flag or global setting
	isDryRun := cleanDryRunFlag || dryRun
	if isDryRun {
		printDryRunBanner()
	}

	// Parse older-than duration if provided
	var olderThanDuration time.Duration
	if cleanOlderThan != "" {
		var err error
		olderThanDuration, err = parseDuration(cleanOlderThan)
		if err != nil {
			return fmt.Errorf("invalid --older-than value: %w", err)
		}
	}

	// Initialize container
	app, err := newContainerApp(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer closeApp(app)

	// Get repository info
	gitAdapter := app.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	// List all release runs
	releaseRepo := app.ReleaseRepository()
	runIDs, err := releaseRepo.List(ctx, repoInfo.Path)
	if err != nil {
		printInfo("No release runs found")
		return nil
	}

	if len(runIDs) == 0 {
		printInfo("No release runs found")
		return nil
	}

	printInfo(fmt.Sprintf("Found %d release run(s)", len(runIDs)))
	fmt.Println()

	// Determine which runs to delete
	result, err := determineRunsToDelete(ctx, releaseRepo, runIDs, olderThanDuration, isDryRun)
	if err != nil {
		return err
	}

	// Nothing to delete
	if len(result.DeletedIDs) == 0 {
		printInfo("No release runs to clean up")
		if outputJSON {
			return outputCleanJSON(result)
		}
		return nil
	}

	// Perform deletion
	if !isDryRun {
		for _, id := range result.DeletedIDs {
			if err := releaseRepo.Delete(ctx, release.RunID(id)); err != nil {
				printWarning(fmt.Sprintf("Failed to delete %s: %v", shortenID(id), err))
			} else {
				printSuccess(fmt.Sprintf("Deleted %s", shortenID(id)))
			}
		}
	} else {
		for _, id := range result.DeletedIDs {
			printInfo(fmt.Sprintf("Would delete %s", shortenID(id)))
		}
	}

	fmt.Println()

	if outputJSON {
		return outputCleanJSON(result)
	}

	// Summary
	if isDryRun {
		printInfo(fmt.Sprintf("Would delete %d run(s), keeping %d", result.DeletedCount, result.KeptCount))
	} else {
		printSuccess(fmt.Sprintf("Deleted %d run(s), kept %d", result.DeletedCount, result.KeptCount))
	}

	if result.SkippedActive > 0 {
		printWarning(fmt.Sprintf("Skipped %d active run(s) - use 'relicta cancel' first", result.SkippedActive))
	}

	return nil
}

// determineRunsToDelete determines which runs to delete based on the configured options.
func determineRunsToDelete(ctx context.Context, releaseRepo release.Repository, runIDs []release.RunID, olderThan time.Duration, isDryRun bool) (*cleanResult, error) {
	result := &cleanResult{
		TotalRuns:  len(runIDs),
		DeletedIDs: []string{},
		KeptIDs:    []string{},
		DryRun:     isDryRun,
	}

	now := time.Now()

	// Load all runs to check their state and age
	type runInfo struct {
		id       release.RunID
		state    release.RunState
		updated  time.Time
		isActive bool
	}

	runs := make([]runInfo, 0, len(runIDs))
	for _, id := range runIDs {
		rel, err := releaseRepo.FindByID(ctx, id)
		if err != nil {
			continue
		}

		isActive := !rel.State().IsFinal()
		runs = append(runs, runInfo{
			id:       id,
			state:    rel.State(),
			updated:  rel.UpdatedAt(),
			isActive: isActive,
		})
	}

	// Determine which runs to delete based on criteria
	for i, run := range runs {
		shouldDelete := false
		shouldKeep := false

		// Never delete active runs
		if run.isActive {
			result.SkippedActive++
			shouldKeep = true
		}

		// Check age if --older-than specified
		if olderThan > 0 && !shouldKeep {
			age := now.Sub(run.updated)
			if age > olderThan {
				shouldDelete = true
			}
		}

		// Check --all flag (keep only latest)
		if cleanAll && !shouldKeep {
			if i > 0 { // Keep the first (latest) run
				shouldDelete = true
			}
		}

		// Check --keep flag (default: keep last 10)
		if !cleanAll && !shouldKeep && olderThan == 0 {
			if i >= cleanKeepLast {
				shouldDelete = true
			}
		}

		// When using --older-than with --keep, apply both criteria
		if olderThan > 0 && !cleanAll && !shouldKeep {
			if i < cleanKeepLast {
				shouldDelete = false // Keep even if old
			}
		}

		if shouldDelete && !shouldKeep {
			result.DeletedIDs = append(result.DeletedIDs, string(run.id))
		} else {
			result.KeptIDs = append(result.KeptIDs, string(run.id))
		}
	}

	result.DeletedCount = len(result.DeletedIDs)
	result.KeptCount = len(result.KeptIDs)

	return result, nil
}

// parseDuration parses a duration string like "7d", "30d", "2w".
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("duration too short: %s", s)
	}

	unit := s[len(s)-1]
	valueStr := s[:len(s)-1]

	var value int
	_, err := fmt.Sscanf(valueStr, "%d", &value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration value: %s", valueStr)
	}

	switch unit {
	case 'd', 'D':
		return time.Duration(value) * 24 * time.Hour, nil
	case 'w', 'W':
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	case 'h', 'H':
		return time.Duration(value) * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %c (use d for days, w for weeks, h for hours)", unit)
	}
}

// shortenID shortens a run ID for display.
func shortenID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// outputCleanJSON outputs the clean result as JSON.
func outputCleanJSON(result *cleanResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
