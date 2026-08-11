// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/application/governance"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	gitservice "github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
)

var (
	historyLimit    int
	historyRepo     string
	historyActorID  string
	historyShowRisk bool

	// getMemoryStoreFunc is the function used to get the memory store.
	// It can be overridden in tests.
	getMemoryStoreFunc = getMemoryStore
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
	store, err := getMemoryStoreFunc()
	if err != nil {
		return fmt.Errorf("failed to access history store: %w", err)
	}

	repo := historyRepo
	if repo == "" {
		// Try to determine repository from git
		repo = getRepositoryName(ctx)
	}

	if repo == "" {
		return fmt.Errorf("could not determine repository; use --repo to specify")
	}

	// Records written before the identity was made canonical are keyed by the
	// repository's checkout path, and are invisible to a read under the canonical
	// key. Adopting them here rather than dropping them silently: a store that looks
	// healthy and contains nothing about this repository's past is the failure the
	// canonical identity was introduced to fix, repeated at the migration boundary.
	adoptLegacyGovernanceRecords(ctx, store, repo)

	history, err := store.GetReleaseHistory(ctx, repo, historyLimit)
	if err != nil {
		return fmt.Errorf("failed to get release history: %w", err)
	}

	// The empty case has to answer in the caller's language too. Printing prose
	// here meant `relicta history --json` emitted "No release history found for
	// <repo>" on stdout, so a consumer could not tell an empty history from a
	// broken command — both were unparseable.
	if outputJSON {
		return printJSONOutput(history)
	}

	if len(history) == 0 {
		fmt.Println("No release history found for", repo)
		return nil
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
	store, err := getMemoryStoreFunc()
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
	store, err := getMemoryStoreFunc()
	if err != nil {
		return fmt.Errorf("failed to access history store: %w", err)
	}

	repo := historyRepo
	if repo == "" {
		repo = getRepositoryName(ctx)
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

// getMemoryStore opens the same governance store the release path writes to.
//
// It used to open .relicta/memory, falling back to ~/.relicta/memory when that
// directory did not exist. The governance service writes release outcomes to
// .relicta/governance/memory.json, so publishing recorded an outcome and this read
// an empty store in a different directory — `relicta history` reported "no release
// history found" for every repository, always.
//
// Two further problems came with the old resolution. The path was relative to the
// process working directory, so the history depended on which subdirectory the
// command ran from; and the home-directory fallback turned that into silently
// reading a global store, which would have mixed several repositories' governance
// history together had anything ever been written there.
//
// One resolver now answers where the store is, and both sides call it.
func getMemoryStore() (memory.Store, error) {
	return getMemoryStoreCtx(context.Background())
}

// getMemoryStoreCtx is the context-taking form, so a caller inside a request or
// command scope does not silently start a detached one.
func getMemoryStoreCtx(ctx context.Context) (memory.Store, error) {
	repoRoot := ""
	if svc, err := gitservice.NewService(); err == nil {
		if info, infoErr := gitservice.NewAdapter(svc).GetInfo(ctx); infoErr == nil {
			repoRoot = info.Path
		}
	}

	configured := ""
	if cfg != nil {
		configured = cfg.Governance.MemoryPath
	}

	return memory.NewFileStore(filepath.Dir(governance.MemoryStorePath(configured, repoRoot)))
}

// getRepositoryName resolves the current repository as "owner/name".
//
// It goes through the same git adapter that plan, health and the rest of the
// CLI use, rather than parsing .git/config by hand. The previous hand-rolled
// version never resolved anything: git config indents its keys with a tab, so
// stripping the "url = " prefix left the key in place and the URL failed every
// scheme check downstream. It also only looked at ./.git, so it could not work
// from a subdirectory of the repository.
// getRepositoryName returns the canonical governance identity for the current
// repository.
//
// It used to compose info.Owner with info.Name, and those come from different
// sources: Owner is parsed from the remote URL while Name is the last segment of
// the checkout path. On a repository whose remote is github.com/acme/widget.git
// checked out at /tmp/tmp.6fPqrJakiQ, that produced "acme/tmp.6fPqrJakiQ" — a
// plausible-looking identity belonging to no repository, queried against records
// that publish had stored under the absolute path. `relicta history` was empty in
// every repository as a result.
// adoptLegacyGovernanceRecords moves path-keyed records onto the canonical
// identity, once, when the canonical key has none.
//
// Best-effort and quiet on failure: this runs on the read path of an informational
// command, and a migration problem must not stop someone from reading the history
// that is reachable. It reports what it moved, because silently rewriting stored
// governance records is not something to do without saying so.
func adoptLegacyGovernanceRecords(ctx context.Context, store memory.Store, canonicalID string) {
	adopter, ok := store.(interface {
		AdoptLegacyRepositoryKey(context.Context, string, string) (int, error)
	})
	if !ok {
		return
	}

	svc, err := gitservice.NewService()
	if err != nil {
		return
	}
	info, err := gitservice.NewAdapter(svc).GetInfo(ctx)
	if err != nil || info.Path == "" {
		return
	}

	adopted, err := adopter.AdoptLegacyRepositoryKey(ctx, canonicalID, info.Path)
	if err != nil {
		printWarning(fmt.Sprintf("could not migrate legacy governance records: %v", err))
		return
	}
	if adopted > 0 {
		printInfo(fmt.Sprintf("Migrated %d governance record(s) to the repository identity %q",
			adopted, canonicalID))
	}
}

func getRepositoryName(ctx context.Context) string {
	svc, err := gitservice.NewService()
	if err != nil {
		return ""
	}

	info, err := gitservice.NewAdapter(svc).GetInfo(ctx)
	if err != nil {
		return ""
	}
	return info.GovernanceID()
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
