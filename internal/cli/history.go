// Package cli provides the command-line interface for Relicta.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/cgp/memory"
)

var (
	historyLimit    int
	historyRepo     string
	historyActorID  string
	historyShowRisk bool
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View release history and CGP metrics",
	Long: `View historical release data and CGP (Change Governance Protocol) metrics.

This command provides insights into:
  - Past releases and their outcomes
  - Actor reliability scores
  - Risk patterns and trends

Examples:
  # View recent release history
  relicta history

  # View history for a specific repository
  relicta history --repo owner/repo

  # View more history entries
  relicta history --limit 20

  # View risk patterns and trends
  relicta history --risk

  # View metrics for a specific actor
  relicta history --actor human:developer-name

  # Output as JSON
  relicta history --json`,
	RunE: runHistory,
}

var historyReleasesCmd = &cobra.Command{
	Use:   "releases",
	Short: "View release history",
	Long:  `View the history of releases for the current or specified repository.`,
	RunE:  runHistoryReleases,
}

var historyActorCmd = &cobra.Command{
	Use:   "actor [actor-id]",
	Short: "View actor metrics",
	Long: `View reliability metrics for a specific actor.

Actor IDs are prefixed with their type:
  - human:username - For human actors
  - agent:name - For AI agents
  - ci:name - For CI systems

Examples:
  relicta history actor human:developer
  relicta history actor agent:github-copilot`,
	Args: cobra.MaximumNArgs(1),
	RunE: runHistoryActor,
}

var historyRiskCmd = &cobra.Command{
	Use:   "risk",
	Short: "View risk patterns and trends",
	Long:  `View historical risk patterns and trends for the repository.`,
	RunE:  runHistoryRisk,
}

func init() {
	rootCmd.AddCommand(historyCmd)
	historyCmd.AddCommand(historyReleasesCmd)
	historyCmd.AddCommand(historyActorCmd)
	historyCmd.AddCommand(historyRiskCmd)

	// Main history command flags
	historyCmd.PersistentFlags().IntVarP(&historyLimit, "limit", "n", 10, "Number of entries to show")
	historyCmd.PersistentFlags().StringVarP(&historyRepo, "repo", "r", "", "Repository to show history for")

	// Subcommand-specific flags
	historyReleasesCmd.Flags().BoolVar(&historyShowRisk, "risk", false, "Include risk information")
	historyActorCmd.Flags().StringVar(&historyActorID, "actor", "", "Actor ID to show metrics for")
}

func runHistory(cmd *cobra.Command, args []string) error {
	// Default behavior: show release history
	return runHistoryReleases(cmd, args)
}

func runHistoryReleases(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	store, err := getMemoryStore()
	if err != nil {
		return fmt.Errorf("failed to access history store: %w", err)
	}

	repo := historyRepo
	if repo == "" {
		// Try to determine repository from git
		repo = getRepositoryName()
	}

	if repo == "" {
		return fmt.Errorf("could not determine repository; use --repo to specify")
	}

	history, err := store.GetReleaseHistory(ctx, repo, historyLimit)
	if err != nil {
		return fmt.Errorf("failed to get release history: %w", err)
	}

	if len(history) == 0 {
		fmt.Println("No release history found for", repo)
		return nil
	}

	if outputJSON {
		return printJSONOutput(history)
	}

	fmt.Printf("Release History for %s\n", repo)
	fmt.Println(strings.Repeat("─", 60))

	for _, record := range history {
		outcomeSymbol := getOutcomeSymbol(record.Outcome)
		fmt.Printf("%s %s - %s\n", outcomeSymbol, record.Version, record.ReleasedAt.Format(time.RFC3339))

		if historyShowRisk || verbose {
			fmt.Printf("   Risk: %.0f%% | Changes: %d files, %d lines\n",
				record.RiskScore*100, record.FilesChanged, record.LinesChanged)
			if record.BreakingChanges > 0 {
				fmt.Printf("   Breaking changes: %d\n", record.BreakingChanges)
			}
		}

		if verbose && len(record.Tags) > 0 {
			fmt.Printf("   Tags: %s\n", strings.Join(record.Tags, ", "))
		}
	}

	// Show summary stats
	stats := calculateReleaseStats(history)
	fmt.Println()
	fmt.Printf("Summary: %d releases, %.0f%% success rate\n",
		stats.total, stats.successRate*100)

	return nil
}

func runHistoryActor(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	store, err := getMemoryStore()
	if err != nil {
		return fmt.Errorf("failed to access history store: %w", err)
	}

	actorID := historyActorID
	if len(args) > 0 {
		actorID = args[0]
	}

	if actorID == "" {
		return fmt.Errorf("actor ID is required; use --actor or provide as argument")
	}

	metrics, err := store.GetActorMetrics(ctx, actorID)
	if err != nil {
		return fmt.Errorf("failed to get actor metrics: %w", err)
	}

	if outputJSON {
		return printJSONOutput(metrics)
	}

	fmt.Printf("Actor Metrics: %s\n", actorID)
	fmt.Println(strings.Repeat("─", 60))

	reliabilityLabel := getReliabilityLabel(metrics.ReliabilityScore)
	fmt.Printf("Reliability Score: %.0f%% (%s)\n", metrics.ReliabilityScore*100, reliabilityLabel)
	fmt.Println()

	fmt.Println("Release Statistics:")
	fmt.Printf("  Total Releases:     %d\n", metrics.TotalReleases)
	fmt.Printf("  Successful:         %d (%.0f%%)\n",
		metrics.SuccessfulReleases, metrics.SuccessRate*100)
	fmt.Printf("  Failed:             %d\n", metrics.FailedReleases)
	fmt.Printf("  Rollbacks:          %d\n", metrics.RollbackCount)
	fmt.Printf("  Incidents:          %d\n", metrics.IncidentCount)
	fmt.Println()

	fmt.Println("Risk Profile:")
	fmt.Printf("  Average Risk Score: %.0f%%\n", metrics.AverageRiskScore*100)
	fmt.Printf("  High Risk Releases: %d\n", metrics.HighRiskReleases)
	fmt.Printf("  Breaking Changes:   %d releases\n", metrics.BreakingChangeReleases)
	fmt.Println()

	if metrics.FirstReleaseAt != nil && metrics.LastReleaseAt != nil {
		fmt.Println("Activity:")
		fmt.Printf("  First Release: %s\n", metrics.FirstReleaseAt.Format(time.RFC3339))
		fmt.Printf("  Last Release:  %s\n", metrics.LastReleaseAt.Format(time.RFC3339))
	}

	return nil
}

func runHistoryRisk(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	store, err := getMemoryStore()
	if err != nil {
		return fmt.Errorf("failed to access history store: %w", err)
	}

	repo := historyRepo
	if repo == "" {
		repo = getRepositoryName()
	}

	if repo == "" {
		return fmt.Errorf("could not determine repository; use --repo to specify")
	}

	patterns, err := store.GetRiskPatterns(ctx, repo)
	if err != nil {
		return fmt.Errorf("failed to get risk patterns: %w", err)
	}

	if outputJSON {
		return printJSONOutput(patterns)
	}

	fmt.Printf("Risk Patterns for %s\n", repo)
	fmt.Println(strings.Repeat("─", 60))

	fmt.Printf("Average Risk Score: %.0f%%\n", patterns.AverageRiskScore*100)
	fmt.Printf("Risk Trend: %s %s\n", getTrendSymbol(patterns.RiskTrend), patterns.RiskTrend)
	fmt.Printf("Releases Analyzed: %d\n", patterns.TotalReleases)
	fmt.Println()

	if len(patterns.CommonRiskFactors) > 0 {
		fmt.Println("Common Risk Factors:")
		for _, factor := range patterns.CommonRiskFactors {
			fmt.Printf("  • %s (%.0f%% of releases)\n",
				factor.Category, factor.Frequency*100)
			if factor.CorrelatedIncidents > 0 {
				fmt.Printf("    Associated incidents: %d\n", factor.CorrelatedIncidents)
			}
		}
		fmt.Println()
	}

	if len(patterns.IncidentCorrelations) > 0 {
		fmt.Println("Incident Correlations:")
		for _, corr := range patterns.IncidentCorrelations {
			fmt.Printf("  • %s: %.0f%% incident probability (n=%d)\n",
				corr.Pattern, corr.IncidentProbability*100, corr.SampleSize)
		}
	}

	return nil
}

func getMemoryStore() (memory.Store, error) {
	// Default to file store in .relicta directory
	storeDir := filepath.Join(".relicta", "memory")

	// Check if directory exists
	if _, err := os.Stat(storeDir); os.IsNotExist(err) {
		// Try home directory
		homeDir, err := os.UserHomeDir()
		if err == nil {
			storeDir = filepath.Join(homeDir, ".relicta", "memory")
		}
	}

	return memory.NewFileStore(storeDir)
}

func getRepositoryName() string {
	// Try to get from git remote
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Look for .git directory
	gitDir := filepath.Join(cwd, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return ""
	}

	// Try to read the remote URL
	configPath := filepath.Join(gitDir, "config")
	content, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	// Simple parsing to extract remote URL
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if strings.Contains(line, "[remote \"origin\"]") && i+1 < len(lines) {
			for j := i + 1; j < len(lines) && !strings.HasPrefix(lines[j], "["); j++ {
				if strings.Contains(lines[j], "url = ") {
					url := strings.TrimSpace(strings.TrimPrefix(lines[j], "url = "))
					return extractRepoFromURL(url)
				}
			}
		}
	}

	return ""
}

func extractRepoFromURL(url string) string {
	// Handle SSH format: git@github.com:owner/repo.git
	if strings.HasPrefix(url, "git@") {
		parts := strings.SplitN(url, ":", 2)
		if len(parts) == 2 {
			repo := strings.TrimSuffix(parts[1], ".git")
			return repo
		}
	}

	// Handle HTTPS format: https://github.com/owner/repo.git
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		parts := strings.Split(url, "/")
		if len(parts) >= 2 {
			owner := parts[len(parts)-2]
			repo := strings.TrimSuffix(parts[len(parts)-1], ".git")
			return owner + "/" + repo
		}
	}

	return ""
}

func getOutcomeSymbol(outcome memory.ReleaseOutcome) string {
	switch outcome {
	case memory.OutcomeSuccess:
		return "✓"
	case memory.OutcomeFailed:
		return "✗"
	case memory.OutcomeRollback:
		return "↩"
	case memory.OutcomePartial:
		return "◐"
	default:
		return "?"
	}
}

func getTrendSymbol(trend memory.RiskTrend) string {
	switch trend {
	case memory.TrendIncreasing:
		return "↑"
	case memory.TrendDecreasing:
		return "↓"
	case memory.TrendStable:
		return "→"
	default:
		return "?"
	}
}

func getReliabilityLabel(score float64) string {
	switch {
	case score >= 0.9:
		return "Excellent"
	case score >= 0.8:
		return "Very Good"
	case score >= 0.7:
		return "Good"
	case score >= 0.6:
		return "Fair"
	case score >= 0.5:
		return "Needs Improvement"
	default:
		return "Poor"
	}
}

type releaseStats struct {
	total       int
	successful  int
	failed      int
	successRate float64
}

func calculateReleaseStats(releases []*memory.ReleaseRecord) releaseStats {
	stats := releaseStats{total: len(releases)}

	for _, r := range releases {
		switch r.Outcome {
		case memory.OutcomeSuccess:
			stats.successful++
		case memory.OutcomeFailed, memory.OutcomeRollback:
			stats.failed++
		}
	}

	if stats.total > 0 {
		stats.successRate = float64(stats.successful) / float64(stats.total)
	}

	return stats
}

func printJSONOutput(v any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
