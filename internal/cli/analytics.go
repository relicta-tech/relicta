// Package cli provides the command-line interface for Relicta.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/analytics"
)

var (
	analyticsGranularity string
	analyticsView        string
)

func init() {
	analyticsCmd.Flags().StringVarP(&analyticsView, "view", "w", "all",
		"which view to show: risk | decisions | team | all")
	analyticsCmd.Flags().StringVarP(&analyticsGranularity, "granularity", "g", "week",
		"time bucket for trends: day | week | month")
	analyticsCmd.GroupID = "inspect"
	rootCmd.AddCommand(analyticsCmd)
}

var analyticsCmd = &cobra.Command{
	Use:   "analytics",
	Short: "Show governance analytics (risk trends, decisions, team metrics)",
	Long: `Surface the governance analytics captured during plan/approve/publish:
risk-score trends over time, the distribution of policy decisions, and
per-actor approval/release metrics.

Analytics are captured automatically when governance is enabled; this
command aggregates the stored events.

Examples:
  relicta analytics                       # all views, weekly buckets
  relicta analytics --view risk -g day    # daily risk trend only
  relicta analytics --json                # machine-readable output`,
	RunE: runAnalytics,
}

func runAnalytics(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	app, err := newContainerApp(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer closeApp(app)

	svc := app.Analytics()
	if svc == nil {
		return fmt.Errorf("analytics unavailable: enable governance (governance.enabled) to capture analytics")
	}

	agg := analytics.NewCachedAggregator(svc, 0)
	gran := analytics.ParseGranularity(analyticsGranularity)
	filter := analytics.QueryFilter{}

	risk, err := agg.RiskTrends(ctx, filter, gran)
	if err != nil {
		return fmt.Errorf("risk trends: %w", err)
	}
	decisions, err := agg.Decisions(ctx, filter, gran)
	if err != nil {
		return fmt.Errorf("decisions: %w", err)
	}
	team, err := agg.Team(ctx, filter)
	if err != nil {
		return fmt.Errorf("team metrics: %w", err)
	}

	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"risk_trends": risk,
			"decisions":   decisions,
			"team":        team,
		})
	}

	showAll := analyticsView == "all"
	if showAll || analyticsView == "risk" {
		printRiskTrends(risk)
	}
	if showAll || analyticsView == "decisions" {
		printDecisionDistribution(decisions)
	}
	if showAll || analyticsView == "team" {
		printTeamMetrics(team)
	}
	return nil
}

func printRiskTrends(points []analytics.RiskTrendPoint) {
	printTitle("Risk Trends")
	fmt.Println()
	if len(points) == 0 {
		printInfo("No risk evaluations captured yet.")
		fmt.Println()
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "BUCKET\tAVG\tMIN\tMAX\tCOUNT")
	for _, p := range points {
		fmt.Fprintf(w, "%s\t%.2f\t%.2f\t%.2f\t%d\n", p.Bucket, p.AvgRiskScore, p.MinRiskScore, p.MaxRiskScore, p.Count)
	}
	_ = w.Flush()
	fmt.Println()
}

func printDecisionDistribution(dist []analytics.DecisionDistribution) {
	printTitle("Policy Decisions")
	fmt.Println()
	if len(dist) == 0 {
		printInfo("No policy decisions captured yet.")
		fmt.Println()
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "BUCKET\tAPPROVE\tDENY\tREVIEW\tTOTAL")
	for _, d := range dist {
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\n", d.Bucket, d.Approve, d.Deny, d.RequireReview, d.Total)
	}
	_ = w.Flush()
	fmt.Println()
}

func printTeamMetrics(team []analytics.TeamMetrics) {
	printTitle("Team Metrics")
	fmt.Println()
	if len(team) == 0 {
		printInfo("No approval/release activity captured yet.")
		fmt.Println()
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ACTOR\tAPPROVALS\tREJECTIONS\tRELEASES\tSUCCESS%")
	for _, t := range team {
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%.0f%%\n", t.ActorID, t.ApprovalCount, t.RejectionCount, t.ReleaseCount, t.SuccessRate*100)
	}
	_ = w.Flush()
	fmt.Println()
}
