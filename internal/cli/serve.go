package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/container"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/httpserver"
)

var (
	servePort    string
	serveAddress string
	serveAPIKey  string
	serveNoAuth  bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the dashboard web server",
	Long: `Start the self-hosted dashboard web server.

The dashboard provides a web UI for:
  - Release pipeline visualization
  - Governance analytics and risk trends
  - Team performance metrics
  - Approval workflow management
  - Audit trail exploration

The server can be configured via:
  - Command-line flags (--port, --address)
  - Configuration file (dashboard section)
  - Environment variables (RELICTA_DASHBOARD_*)

Examples:
  # Start on default port 8080
  relicta serve

  # Start on custom port
  relicta serve --port 3000

  # Start on specific address
  relicta serve --address localhost:9000

Authentication:
  By default, API key authentication is required. Configure API keys
  in your release.config.yaml:

    dashboard:
      enabled: true
      auth:
        mode: api_key
        api_keys:
          - key: ${RELICTA_DASHBOARD_KEY}
            name: "Admin"
            roles: ["admin"]`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVar(&servePort, "port", "", "Port to listen on (default: 8080)")
	serveCmd.Flags().StringVar(&serveAddress, "address", "", "Address to listen on (e.g., localhost:8080)")
	serveCmd.Flags().StringVar(&serveAPIKey, "api-key", "", "API key for dashboard authentication (enables API key mode)")
	serveCmd.Flags().BoolVar(&serveNoAuth, "no-auth", false, "Disable authentication (not recommended for production)")
}

func runServe(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		// Use default config if none found
		cfg = config.DefaultConfig()
	}

	// Override address from flags
	address := cfg.Dashboard.Address
	if serveAddress != "" {
		address = serveAddress
	} else if servePort != "" {
		address = ":" + servePort
	}
	if address == "" {
		address = ":8080"
	}

	// Update config with resolved address
	dashboardCfg := cfg.Dashboard
	dashboardCfg.Address = address

	// Handle authentication flags
	if serveAPIKey != "" {
		// Enable API key auth with the provided key
		dashboardCfg.Auth.Mode = config.DashboardAuthAPIKey
		dashboardCfg.Auth.APIKeys = []config.DashboardAPIKeyConfig{
			{
				Key:   serveAPIKey,
				Name:  "CLI",
				Roles: []string{string(config.DashboardRoleAdmin)},
			},
		}
	} else if serveNoAuth {
		dashboardCfg.Auth.Mode = config.DashboardAuthNone
	}

	// Warn if API key mode with no keys configured
	if dashboardCfg.Auth.Mode == config.DashboardAuthAPIKey && len(dashboardCfg.Auth.APIKeys) == 0 {
		slog.Warn("No API keys configured. Dashboard will be inaccessible.",
			"hint", "Use --api-key flag or configure api_keys in release.config.yaml")
	}

	// Initialize application container
	var releaseServices *release.Services
	app, err := initializeAppContainer(ctx, cfg)
	if err != nil {
		slog.Warn("Failed to initialize services, running with limited functionality", "error", err)
	} else {
		defer app.Close()
		releaseServices = app.ReleaseServices()
	}

	// Get embedded frontend (nil if not compiled with embed_frontend tag)
	frontend := embeddedFrontend

	// Create server
	server := httpserver.NewServer(httpserver.ServerDeps{
		Config:          dashboardCfg,
		Frontend:        frontend,
		ReleaseServices: releaseServices,
	})

	// Wire up WebSocket event broadcasting
	if app != nil {
		broadcaster := server.EventBroadcaster()
		app.SubscribeToEvents(func(event release.DomainEvent) {
			// Broadcast events asynchronously to WebSocket clients
			broadcaster.PublishAsync(context.Background(), event)
		})
		slog.Debug("WebSocket event broadcasting enabled")
	}

	// Print startup message
	fmt.Printf("Starting Relicta dashboard server on %s\n", address)
	fmt.Printf("Press Ctrl+C to stop\n\n")

	if dashboardCfg.Auth.Mode == config.DashboardAuthNone {
		fmt.Println(styles.Warning.Render("WARNING: Authentication is disabled. Not recommended for production."))
	}

	if frontend == nil {
		fmt.Println(styles.Info.Render("Running in API-only mode (no frontend embedded)"))
	}

	fmt.Printf("\nAPI endpoints:\n")
	fmt.Printf("  Health:     http://%s/health\n", resolveDisplayAddress(address))
	fmt.Printf("  API:        http://%s/api/v1/\n", resolveDisplayAddress(address))
	fmt.Printf("  WebSocket:  ws://%s/api/v1/ws\n", resolveDisplayAddress(address))

	if frontend != nil {
		fmt.Printf("  Dashboard:  http://%s/\n", resolveDisplayAddress(address))
	}
	fmt.Println()

	// Start server (blocks until context is canceled)
	if err := server.Start(ctx); err != nil && ctx.Err() == nil {
		return fmt.Errorf("server error: %w", err)
	}

	fmt.Println("\nServer stopped gracefully")
	return nil
}

// initializeAppContainer initializes the application container with release services.
func initializeAppContainer(ctx context.Context, cfg *config.Config) (*container.App, error) {
	app, err := container.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	if err := app.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize container: %w", err)
	}

	// Initialize release services with current working directory
	repoRoot, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	if err := app.InitReleaseServices(ctx, repoRoot); err != nil {
		slog.Debug("Failed to initialize release services", "error", err)
		// Continue without release services - they may not be needed
	}

	return app, nil
}

// resolveDisplayAddress converts ":8080" to "localhost:8080" for display.
func resolveDisplayAddress(addr string) string {
	if len(addr) > 0 && addr[0] == ':' {
		return "localhost" + addr
	}
	return addr
}
