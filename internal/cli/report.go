// Package cli provides the command-line interface for Relicta.
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/cgp/memory"
	"github.com/relicta-tech/relicta/internal/compliance"
)

var (
	reportType   string
	reportPeriod string
	reportFormat string
	reportRepo   string
	reportOutput string
)

func init() {
	reportCmd.Flags().StringVar(&reportType, "type", "summary", "report type: dora, soc2, summary")
	reportCmd.Flags().StringVar(&reportPeriod, "period", "", "time period (e.g. 2026-Q1 or 2026-03-01:2026-03-31)")
	reportCmd.Flags().StringVar(&reportFormat, "format", "markdown", "output format: markdown, json")
	reportCmd.Flags().StringVar(&reportRepo, "repo", "", "repository filter (default: current repository)")
	reportCmd.Flags().StringVarP(&reportOutput, "output", "o", "", "write report to file instead of stdout")
}

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate compliance reports",
	Long: `Generate compliance and governance reports from release history.

Supported report types:

  dora     DORA metrics (Deployment Frequency, Lead Time,
           MTTR, Change Failure Rate)
  soc2     SOC 2 change management evidence (change log,
           approval evidence, risk assessments, incidents)
  summary  General governance summary with risk distribution,
           approval breakdown, and actor activity

Examples:

  relicta report --type dora --period 2026-Q1
  relicta report --type soc2 --period "2026-03-01:2026-03-31" --format json
  relicta report --type summary --period 2026-Q1 -o report.md`,
	RunE: runReport,
}

func runReport(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	if reportPeriod == "" {
		return fmt.Errorf("--period is required (e.g. 2026-Q1 or 2026-03-01:2026-03-31)")
	}

	period, err := compliance.ParsePeriod(reportPeriod)
	if err != nil {
		return fmt.Errorf("invalid period: %w", err)
	}

	rt := compliance.ReportType(reportType)
	if !rt.IsValid() {
		return fmt.Errorf("invalid report type %q: use dora, soc2, or summary", reportType)
	}

	rf := compliance.ReportFormat(reportFormat)
	if !rf.IsValid() {
		return fmt.Errorf("invalid format %q: use markdown or json", reportFormat)
	}

	// Override format if --json global flag is set
	if outputJSON {
		rf = compliance.FormatJSON
	}

	config := compliance.ReportConfig{
		Type:       rt,
		Format:     rf,
		Period:     period,
		Repository: reportRepo,
	}

	// Create the memory store. In a full integration the store would be
	// injected from the application container. For now we use the in-memory
	// store which can be populated by the governance pipeline.
	store := memory.NewInMemoryStore()

	gen := compliance.NewGenerator(store, nil)

	report, err := gen.Generate(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	output, err := gen.Render(report, rf)
	if err != nil {
		return fmt.Errorf("failed to render report: %w", err)
	}

	if reportOutput != "" {
		if err := os.WriteFile(reportOutput, []byte(output), 0o600); err != nil {
			return fmt.Errorf("failed to write report to %s: %w", reportOutput, err)
		}
		printSuccess(fmt.Sprintf("Report written to %s", reportOutput))
		return nil
	}

	if outputJSON && rf != compliance.FormatJSON {
		// Wrap the markdown in a JSON envelope for consistent JSON output mode
		envelope := map[string]string{"report": output}
		b, _ := json.MarshalIndent(envelope, "", "  ")
		fmt.Println(string(b))
		return nil
	}

	fmt.Print(output)
	return nil
}
