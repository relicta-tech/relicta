package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

var (
	serverMode           string
	serverAllowedOrigins string
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the dashboard API server with deployment mode control",
	Long: `Start the dashboard server with explicit control over the deployment mode.

This is an enhanced alias for 'relicta serve' that adds deployment mode flags
for separating the frontend from the backend API.

Modes:
  embedded  (default) Serves the embedded frontend alongside the API.
  api       API-only mode — no frontend is served. Set CORS origins
            to allow an external frontend to connect.

Examples:
  # Embedded mode (same as 'relicta serve')
  relicta server

  # API-only mode for standalone frontend deployment
  relicta server --mode api --allowed-origins "http://localhost:5173,https://dashboard.example.com"

  # Using environment variables
  RELICTA_SERVER_MODE=api RELICTA_ALLOWED_ORIGINS="https://dashboard.example.com" relicta server

Authentication:
  All authentication modes from 'relicta serve' are supported.
  See 'relicta serve --help' for details.`,
	RunE: runServer,
}

func init() {
	rootCmd.AddCommand(serverCmd)

	// Inherit serve flags
	serverCmd.Flags().StringVarP(&servePort, "port", "p", "", "Port to listen on (default: 8080)")
	serverCmd.Flags().StringVarP(&serveAddress, "address", "a", "", "Address to listen on (e.g., localhost:8080)")
	serverCmd.Flags().StringVarP(&serveAPIKey, "api-key", "k", "", "API key for dashboard authentication")
	serverCmd.Flags().BoolVarP(&serveNoAuth, "no-auth", "n", false, "Disable authentication (not recommended for production)")

	// Server-specific flags
	serverCmd.Flags().StringVar(&serverMode, "mode", "", "Server mode: embedded (default) or api")
	serverCmd.Flags().StringVar(&serverAllowedOrigins, "allowed-origins", "", "Comma-separated CORS allowed origins (e.g., http://localhost:5173)")
}

func runServer(cmd *cobra.Command, args []string) error {
	// Resolve server mode from flag, env var, or default
	mode := resolveServerMode()

	// Resolve allowed origins from flag or env var
	origins := resolveAllowedOrigins()

	// Store in a context-like mechanism for runServe to pick up
	serverModeOverride = mode
	serverOriginsOverride = origins

	return runServe(cmd, args)
}

// resolveServerMode returns the server mode from flag, env, or default.
func resolveServerMode() config.ServerMode {
	// Flag takes precedence
	if serverMode != "" {
		return config.ServerMode(serverMode)
	}

	// Then env var
	if env := os.Getenv("RELICTA_SERVER_MODE"); env != "" {
		return config.ServerMode(env)
	}

	// Default
	return config.ServerModeEmbedded
}

// resolveAllowedOrigins returns allowed origins from flag or env var.
func resolveAllowedOrigins() []string {
	raw := serverAllowedOrigins
	if raw == "" {
		raw = os.Getenv("RELICTA_ALLOWED_ORIGINS")
	}
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}
