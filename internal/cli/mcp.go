package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/container"
	"github.com/relicta-tech/relicta/internal/mcp"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Model Context Protocol (MCP) server commands",
	Long: `Manage the MCP server for AI agent integration.

The Model Context Protocol allows AI agents to interact with Relicta
through a standardized protocol, enabling automated release management.`,
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	Long: `Start the MCP server for AI agent communication.

The server uses stdio transport by default, allowing integration with
AI clients that support the Model Context Protocol (MCP).

Core Tools:
  - relicta.status:   Get current release state
  - relicta.init:     Initialize configuration file
  - relicta.plan:     Analyze commits and plan release
  - relicta.bump:     Calculate and set version
  - relicta.notes:    Generate release notes
  - relicta.evaluate: CGP risk evaluation
  - relicta.approve:  Approve the release
  - relicta.publish:  Execute the release

AI Agent Tools:
  - relicta.blast_radius:     Analyze monorepo change impact
  - relicta.infer_version:    Lightweight version inference
  - relicta.summarize_diff:   Audience-tailored change summaries
  - relicta.validate_release: Pre-flight release validation

Resources available:
  - relicta://state:       Current release state
  - relicta://config:      Configuration settings
  - relicta://commits:     Recent commits
  - relicta://changelog:   Generated changelog
  - relicta://risk-report: CGP risk assessment`,
	RunE: runMCPServe,
}

func init() {
	mcpCmd.AddCommand(mcpServeCmd)
}

func runMCPServe(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Create logger for MCP server
	mcpLogger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Load config if not already loaded
	if cfg == nil {
		if err := initConfig(); err != nil {
			// Config loading is optional for MCP - use defaults
			mcpLogger.Warn("config not loaded, using defaults", "error", err)
		}
	}

	// Create server options
	opts := []mcp.ServerOption{
		mcp.WithLogger(mcpLogger),
	}

	// Add config if loaded
	if cfg != nil {
		opts = append(opts, mcp.WithConfig(cfg))
	}

	// Initialize container to get use cases
	var app *container.App
	if cfg != nil {
		var err error
		app, err = container.NewInitialized(ctx, cfg)
		if err != nil {
			mcpLogger.Warn("failed to initialize container, tools will return stubs", "error", err)
		} else {
			defer func() {
				if closeErr := app.Close(); closeErr != nil {
					mcpLogger.Warn("failed to close container", "error", closeErr)
				}
			}()

			// Initialize release services with current working directory
			// This enables the DDD release workflow (plan, bump, notes, approve, publish)
			repoRoot, err := os.Getwd()
			if err != nil {
				mcpLogger.Warn("failed to get working directory for release services", "error", err)
			} else {
				if err := app.InitReleaseServices(ctx, repoRoot); err != nil {
					mcpLogger.Warn("failed to initialize release services", "error", err)
				}
			}

			// Create adapter with use cases from container
			adapter := createMCPAdapter(app)
			opts = append(opts, mcp.WithAdapter(adapter))
		}
	}

	// Create and start MCP server
	server, err := mcp.NewServer(versionInfo.Version, opts...)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	// Log startup
	hasAdapter := app != nil
	mcpLogger.Info("starting MCP server",
		"version", versionInfo.Version,
		"transport", "stdio",
		"tools_wired", hasAdapter,
	)

	// Serve on stdio
	return server.ServeStdio()
}

// createMCPAdapter creates an MCP adapter wired to the container's services.
// ADR-007: All interfaces must use application services layer.
func createMCPAdapter(app *container.App) *mcp.Adapter {
	opts := []mcp.AdapterOption{}

	// Wire release analyzer for planning
	if analyzer := app.ReleaseAnalyzer(); analyzer != nil {
		opts = append(opts, mcp.WithReleaseAnalyzer(analyzer))
	}

	// Wire DDD release services (ADR-007 compliant)
	// This provides plan, bump, notes, approve, publish functionality
	if app.HasReleaseServices() {
		opts = append(opts, mcp.WithReleaseServices(app.ReleaseServices()))
	}

	// Wire governance service for CGP evaluation
	if svc := app.GovernanceService(); svc != nil {
		opts = append(opts, mcp.WithGovernanceService(svc))
	}

	// Wire blast radius service for monorepo analysis
	if app.HasBlastService() {
		opts = append(opts, mcp.WithBlastService(app.BlastService()))
	}

	// Wire AI service for diff summarization
	if app.HasAI() {
		opts = append(opts, mcp.WithAIService(app.AI()))
	}

	return mcp.NewAdapter(opts...)
}
