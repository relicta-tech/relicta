// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/cgp/memory"
	"github.com/relicta-tech/relicta/internal/compliance"
	"github.com/relicta-tech/relicta/internal/integrations/drata"
	"github.com/relicta-tech/relicta/internal/integrations/vanta"
)

var (
	vantaToken   string
	vantaBaseURL string
	vantaPeriod  string
	vantaRepo    string
	vantaEvType  string
	vantaDryRun  bool

	drataToken       string
	drataBaseURL     string
	drataWorkspaceID string
	drataPeriod      string
	drataRepo        string
	drataEvType      string
	drataDryRun      bool
)

func init() {
	vantaPushCmd.Flags().StringVar(&vantaToken, "token", "", "Vanta API token (defaults to VANTA_API_TOKEN env)")
	vantaPushCmd.Flags().StringVar(&vantaBaseURL, "base-url", "", "Vanta API base URL (default: https://api.vanta.com/v1)")
	vantaPushCmd.Flags().StringVar(&vantaPeriod, "period", "", "reporting period (e.g. 2026-Q1)")
	vantaPushCmd.Flags().StringVar(&vantaRepo, "repo", "", "repository identifier (system identifier)")
	vantaPushCmd.Flags().StringVar(&vantaEvType, "type", "article12", "evidence type: article12, soc2")
	vantaPushCmd.Flags().BoolVar(&vantaDryRun, "dry-run", false, "render evidence payloads without pushing to Vanta")

	vantaCmd.AddCommand(vantaPushCmd)
	integrationsCmd.AddCommand(vantaCmd)

	drataPushCmd.Flags().StringVar(&drataToken, "token", "", "Drata API token (defaults to DRATA_API_TOKEN env)")
	drataPushCmd.Flags().StringVar(&drataBaseURL, "base-url", "", "Drata API base URL (default: https://api.drata.com/public-api/v1)")
	drataPushCmd.Flags().StringVar(&drataWorkspaceID, "workspace-id", "", "Drata workspace ID (defaults to DRATA_WORKSPACE_ID env)")
	drataPushCmd.Flags().StringVar(&drataPeriod, "period", "", "reporting period (e.g. 2026-Q1)")
	drataPushCmd.Flags().StringVar(&drataRepo, "repo", "", "repository identifier (system identifier)")
	drataPushCmd.Flags().StringVar(&drataEvType, "type", "article12", "evidence type: article12, soc2")
	drataPushCmd.Flags().BoolVar(&drataDryRun, "dry-run", false, "render evidence payloads without pushing to Drata")

	drataCmd.AddCommand(drataPushCmd)
	integrationsCmd.AddCommand(drataCmd)
}

var integrationsCmd = &cobra.Command{
	Use:   "integrations",
	Short: "Third-party integrations (Vanta, Drata)",
	Long: `Push Relicta governance evidence to external compliance platforms.

Currently supported:

  vanta   Vanta REST API — push Article 12 logs and SOC 2 evidence as
          custom evidence artifacts. Vanta released its remote MCP server
          and Claude plugin in April 2026; Relicta complements Vanta by
          providing upstream cryptographically attested release evidence.
  drata   Drata REST API — same evidence shape, different transport. Use
          when your org runs Drata for SOC 2 / ISO 27001 / HIPAA / PCI
          continuous monitoring.`,
}

var vantaCmd = &cobra.Command{
	Use:   "vanta",
	Short: "Vanta evidence push integration",
}

var vantaPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push evidence to Vanta",
	Long: `Generate compliance evidence and push it to Vanta as custom evidence.

Examples:

  # Push Article 12 log entries for Q1 2026 (one evidence record per entry)
  relicta integrations vanta push --period 2026-Q1 --type article12

  # Push SOC 2 aggregated evidence (3 records: change log, approvals, risks)
  VANTA_API_TOKEN=secret relicta integrations vanta push --period 2026-Q1 --type soc2

  # Dry-run: render JSON payloads without API calls
  relicta integrations vanta push --period 2026-Q1 --type article12 --dry-run`,
	RunE: runVantaPush,
}

func runVantaPush(cmd *cobra.Command, args []string) error {
	if vantaPeriod == "" {
		return errors.New("--period is required (e.g. 2026-Q1)")
	}

	period, err := compliance.ParsePeriod(vantaPeriod)
	if err != nil {
		return fmt.Errorf("invalid period: %w", err)
	}

	token := vantaToken
	if token == "" {
		token = os.Getenv("VANTA_API_TOKEN")
	}
	if token == "" && !vantaDryRun {
		return errors.New("Vanta API token required: pass --token or set VANTA_API_TOKEN")
	}

	repo := vantaRepo
	if repo == "" {
		return errors.New("--repo is required (system identifier shown in Vanta UI)")
	}

	store := memory.NewInMemoryStore()
	gen := compliance.NewGenerator(store, nil)

	var evidence []vanta.Evidence

	switch vantaEvType {
	case "article12", "eu-ai-act-article-12":
		report, err := gen.Generate(cmd.Context(), compliance.ReportConfig{
			Type:       compliance.ReportEUAIActArticle12,
			Format:     compliance.FormatJSONL,
			Period:     period,
			Repository: repo,
		})
		if err != nil {
			return fmt.Errorf("generate Article 12 report: %w", err)
		}
		evidence = vanta.MapArticle12LogEntries(report.Article12)
	case "soc2":
		report, err := gen.Generate(cmd.Context(), compliance.ReportConfig{
			Type:       compliance.ReportSOC2,
			Format:     compliance.FormatJSON,
			Period:     period,
			Repository: repo,
		})
		if err != nil {
			return fmt.Errorf("generate SOC 2 report: %w", err)
		}
		evidence = vanta.MapSOC2(report)
	default:
		return fmt.Errorf("unknown --type %q: use article12 or soc2", vantaEvType)
	}

	if len(evidence) == 0 {
		printSuccess("No evidence to push (no governance events in period).")
		return nil
	}

	printInfo(fmt.Sprintf("Prepared %d evidence record(s) for Vanta.", len(evidence)))

	if vantaDryRun {
		for i, ev := range evidence {
			printInfo(fmt.Sprintf("  [%d] %s — %s (%s)", i+1, ev.Title, ev.Type, ev.SystemIdentifier))
		}
		printSuccess("Dry-run complete. No data sent to Vanta.")
		return nil
	}

	client, err := vanta.NewClient(vanta.ClientConfig{
		APIToken: token,
		BaseURL:  vantaBaseURL,
	})
	if err != nil {
		return fmt.Errorf("init Vanta client: %w", err)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
	defer cancel()

	ids, err := client.PushBatch(ctx, evidence)
	if err != nil {
		printInfo(fmt.Sprintf("Pushed %d/%d records before failure.", len(ids), len(evidence)))
		return fmt.Errorf("push batch: %w", err)
	}

	printSuccess(fmt.Sprintf("Pushed %d evidence record(s) to Vanta. Vanta IDs: %v", len(ids), ids))
	return nil
}

var drataCmd = &cobra.Command{
	Use:   "drata",
	Short: "Drata evidence push integration",
}

var drataPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push evidence to Drata",
	Long: `Generate compliance evidence and push it to Drata as evidence artifacts.

Examples:

  # Push Article 12 log entries for Q1 2026
  relicta integrations drata push --period 2026-Q1 --type article12 --repo acme/payments

  # Push SOC 2 aggregated evidence
  DRATA_API_TOKEN=secret relicta integrations drata push --period 2026-Q1 --type soc2 --repo acme/payments

  # Dry-run: render JSON payloads without API calls
  relicta integrations drata push --period 2026-Q1 --type article12 --repo acme/payments --dry-run`,
	RunE: runDrataPush,
}

func runDrataPush(cmd *cobra.Command, args []string) error {
	if drataPeriod == "" {
		return errors.New("--period is required (e.g. 2026-Q1)")
	}

	period, err := compliance.ParsePeriod(drataPeriod)
	if err != nil {
		return fmt.Errorf("invalid period: %w", err)
	}

	token := drataToken
	if token == "" {
		token = os.Getenv("DRATA_API_TOKEN")
	}
	if token == "" && !drataDryRun {
		return errors.New("Drata API token required: pass --token or set DRATA_API_TOKEN")
	}

	workspaceID := drataWorkspaceID
	if workspaceID == "" {
		workspaceID = os.Getenv("DRATA_WORKSPACE_ID")
	}

	repo := drataRepo
	if repo == "" {
		return errors.New("--repo is required (system identifier shown in Drata UI)")
	}

	store := memory.NewInMemoryStore()
	gen := compliance.NewGenerator(store, nil)

	var evidence []drata.Evidence

	switch drataEvType {
	case "article12", "eu-ai-act-article-12":
		report, err := gen.Generate(cmd.Context(), compliance.ReportConfig{
			Type:       compliance.ReportEUAIActArticle12,
			Format:     compliance.FormatJSONL,
			Period:     period,
			Repository: repo,
		})
		if err != nil {
			return fmt.Errorf("generate Article 12 report: %w", err)
		}
		evidence = drata.MapArticle12LogEntries(report.Article12)
	case "soc2":
		report, err := gen.Generate(cmd.Context(), compliance.ReportConfig{
			Type:       compliance.ReportSOC2,
			Format:     compliance.FormatJSON,
			Period:     period,
			Repository: repo,
		})
		if err != nil {
			return fmt.Errorf("generate SOC 2 report: %w", err)
		}
		evidence = drata.MapSOC2(report)
	default:
		return fmt.Errorf("unknown --type %q: use article12 or soc2", drataEvType)
	}

	if len(evidence) == 0 {
		printSuccess("No evidence to push (no governance events in period).")
		return nil
	}

	printInfo(fmt.Sprintf("Prepared %d evidence record(s) for Drata.", len(evidence)))

	if drataDryRun {
		for i, ev := range evidence {
			printInfo(fmt.Sprintf("  [%d] %s — %s (%s)", i+1, ev.Title, ev.Type, ev.SystemIdentifier))
		}
		printSuccess("Dry-run complete. No data sent to Drata.")
		return nil
	}

	client, err := drata.NewClient(drata.ClientConfig{
		APIToken:    token,
		BaseURL:     drataBaseURL,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return fmt.Errorf("init Drata client: %w", err)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
	defer cancel()

	ids, err := client.PushBatch(ctx, evidence)
	if err != nil {
		printInfo(fmt.Sprintf("Pushed %d/%d records before failure.", len(ids), len(evidence)))
		return fmt.Errorf("push batch: %w", err)
	}

	printSuccess(fmt.Sprintf("Pushed %d evidence record(s) to Drata. Drata IDs: %v", len(ids), ids))
	return nil
}
