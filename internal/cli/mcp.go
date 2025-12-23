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

Tools available via MCP:
  - relicta.status:   Get current release state
  - relicta.plan:     Analyze commits and plan release
  - relicta.bump:     Calculate and set version
  - relicta.notes:    Generate release notes
  - relicta.evaluate: CGP risk evaluation
  - relicta.approve:  Approve the release
  - relicta.publish:  Execute the release

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

// createMCPAdapter creates an MCP adapter wired to the container's use cases.
func createMCPAdapter(app *container.App) *mcp.Adapter {
	opts := []mcp.AdapterOption{}

	// Wire plan use case
	if uc := app.PlanRelease(); uc != nil {
		opts = append(opts, mcp.WithPlanUseCase(uc))
	}

	// Wire calculate version use case
	if uc := app.CalculateVersion(); uc != nil {
		opts = append(opts, mcp.WithCalculateVersionUseCase(uc))
	}

	// Wire set version use case
	if uc := app.SetVersion(); uc != nil {
		opts = append(opts, mcp.WithSetVersionUseCase(uc))
	}

	// Wire generate notes use case
	if uc := app.GenerateNotes(); uc != nil {
		opts = append(opts, mcp.WithGenerateNotesUseCase(uc))
	}

	// Wire approve use case
	if uc := app.ApproveRelease(); uc != nil {
		opts = append(opts, mcp.WithApproveUseCase(uc))
	}

	// Wire publish use case
	if uc := app.PublishRelease(); uc != nil {
		opts = append(opts, mcp.WithPublishUseCase(uc))
	}

	// Wire governance service
	if svc := app.GovernanceService(); svc != nil {
		opts = append(opts, mcp.WithGovernanceService(svc))
	}

	// Wire release repository
	if repo := app.ReleaseRepository(); repo != nil {
		opts = append(opts, mcp.WithAdapterReleaseRepository(repo))
	}

	return mcp.NewAdapter(opts...)
}
