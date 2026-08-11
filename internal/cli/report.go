// Package cli provides the command-line interface for Relicta.
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/compliance"
)

var (
	reportType   string
	reportPeriod string
	reportFormat string
	reportRepo   string
	reportOutput string
)

func init() {
	reportCmd.Flags().StringVar(&reportType, "type", "summary", "report type: dora, soc2, summary, eu-ai-act-article-12, eu-ai-act-annex-iv")
	reportCmd.Flags().StringVar(&reportPeriod, "period", "", "time period (e.g. 2026-Q1 or 2026-03-01:2026-03-31)")
	reportCmd.Flags().StringVar(&reportFormat, "format", "markdown", "output format: markdown, json, jsonl, csv (jsonl/csv require --type eu-ai-act-article-12)")
	reportCmd.Flags().StringVar(&reportRepo, "repo", "", "repository filter (default: current repository)")
	reportCmd.Flags().StringVarP(&reportOutput, "output", "o", "", "write report to file instead of stdout")
}

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate compliance reports",
	Long: `Generate compliance and governance reports from release history.

Supported report types:

  dora                    DORA metrics (Deployment Frequency, Lead Time,
                          MTTR, Change Failure Rate)
  soc2                    SOC 2 change management evidence (change log,
                          approval evidence, risk assessments, incidents)
  summary                 General governance summary with risk distribution,
                          approval breakdown, and actor activity
  eu-ai-act-article-12    EU AI Act Article 12 record-keeping log bundle —
                          one entry per governance decision with use period,
                          reference data, input data, verifiers, and audit
                          chain anchors. 6-month retention enforced.
  eu-ai-act-annex-iv      EU AI Act Annex IV technical documentation —
                          eight-section system documentation: general
                          description, detailed elements, monitoring/control,
                          risk management, lifecycle changes, harmonized
                          standards, conformity declaration scaffold, and
                          post-market monitoring. 10-year retention enforced.

Examples:

  relicta report --type dora --period 2026-Q1
  relicta report --type soc2 --period "2026-03-01:2026-03-31" --format json
  relicta report --type summary --period 2026-Q1 -o report.md
  relicta report --type eu-ai-act-article-12 --period 2026-Q1 --format jsonl -o art12.jsonl
  relicta report --type eu-ai-act-article-12 --period 2026-Q1 --format csv -o art12.csv
  relicta report --type eu-ai-act-annex-iv --period 2026-Q1 -o annex-iv.md`,
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
		return fmt.Errorf("invalid report type %q: use dora, soc2, summary, eu-ai-act-article-12, or eu-ai-act-annex-iv", reportType)
	}

	rf := compliance.ReportFormat(reportFormat)
	if !rf.IsValid() {
		return fmt.Errorf("invalid format %q: use markdown, json, jsonl, or csv", reportFormat)
	}

	// Override format if --json global flag is set
	if outputJSON {
		rf = compliance.FormatJSON
	}

	// The repository the report is about, defaulting to this one under the same
	// identity every other governance read and write uses. Without it the report
	// filtered on a different key than the releases were recorded under.
	repo := reportRepo
	if repo == "" {
		repo = getRepositoryName(ctx)
	}

	config := compliance.ReportConfig{
		Type:       rt,
		Format:     rf,
		Period:     period,
		Repository: repo,
	}

	// The persisted governance store, not a fresh one.
	//
	// This constructed memory.NewInMemoryStore() and handed it straight to the
	// generator, with a comment saying it "can be populated by the governance
	// pipeline" — nothing populated it. So every report was generated from zero
	// records: `relicta report --type dora` said "Total Deployments: 0" for a
	// repository with twelve published releases that `relicta history` listed.
	//
	// Empty is not the worst of it. SOC 2 and EU AI Act Article 12 reports are
	// artifacts someone hands to an auditor, and a clean one — no incidents, no
	// failures — asserted from data that was never read is an affirmative false
	// statement, not a missing feature. A report that cannot see the history has to
	// say so rather than report zeros.
	store, err := getMemoryStore()
	if err != nil {
		return fmt.Errorf("failed to open the governance history store: %w", err)
	}

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
