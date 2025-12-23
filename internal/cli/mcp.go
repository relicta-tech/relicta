package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

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
	// Create logger for MCP server
	mcpLogger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create server options
	opts := []mcp.ServerOption{
		mcp.WithLogger(mcpLogger),
	}

	// Add config if loaded
	if cfg != nil {
		opts = append(opts, mcp.WithConfig(cfg))
	}

	// Create and start MCP server
	server, err := mcp.NewServer(versionInfo.Version, opts...)
	if err != nil {
		return err
	}

	// Log startup
	mcpLogger.Info("starting MCP server",
		"version", versionInfo.Version,
		"transport", "stdio",
	)

	// Serve on stdio
	return server.ServeStdio()
}
