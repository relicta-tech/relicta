package cli

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	cgpmemory "github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/container"
	"github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/httpserver"
	"github.com/relicta-tech/relicta/v4/internal/httpserver/handlers"
	"github.com/relicta-tech/relicta/v4/internal/observability"
)

var (
	servePort    string
	serveAddress string
	serveAPIKey  string
	serveNoAuth  bool

	// serverModeOverride and serverOriginsOverride are set by the server
	// command to pass mode/origin configuration into runServe.
	serverModeOverride    config.ServerMode
	serverOriginsOverride []string
)

// The dashboard server is a single command, `relicta server`, with `serve` kept
// as an alias. There used to be two top-level commands one letter apart:
// `server`'s own help called it "an enhanced alias for 'relicta serve'", its
// flags bound the same variables, and runServer only set two overrides before
// calling runServe. Two names for one thing, differing by a letter, is a trap
// with nothing to gain. runServe stays here as the implementation.

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

	// Apply server mode override (from 'relicta server' command)
	if serverModeOverride != "" {
		dashboardCfg.ServerMode = serverModeOverride
	}

	// Apply allowed origins override (from 'relicta server' command)
	if len(serverOriginsOverride) > 0 {
		dashboardCfg.CORSOrigins = serverOriginsOverride
	}

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
		slog.Warn("No API keys configured. Dashboard will be inaccessible.")
		// Naming api_keys was not actionable: `relicta init` writes no dashboard
		// section, so there was nothing to edit and no indication of the nesting.
		hintDashboardAuth.print()
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

	// Determine frontend: nil in API-only mode or when not compiled with embed tag
	var frontend fs.FS
	if dashboardCfg.ServerMode != config.ServerModeAPI {
		frontend = embeddedFrontend
	}

	// Build the observability subsystem from the repository's configuration. nil when no
	// provider is configured, which the handlers report as `not_configured` rather than
	// answering from an empty subsystem that reads as a healthy one.
	observabilitySvc, obsErr := buildObservabilityService(cfg, app)
	if obsErr != nil {
		return obsErr
	}

	// Create server
	// telemetry.tracing.enabled installs the tracer, which had no production caller: spans
	// were started against a global that nothing ever configured. What it installs today is a
	// logging tracer — config validation warns that the OTLP settings describe an export
	// nothing performs — but "enabled" now does something rather than nothing.
	if cfg.Telemetry.Tracing.Enabled {
		if _, err := observability.InitTracer(observability.TracerConfig{
			Enabled:     true,
			ServiceName: tracingServiceName(cfg),
			Endpoint:    cfg.Telemetry.Tracing.Endpoint,
			SampleRate:  cfg.Telemetry.Tracing.SampleRate,
		}); err != nil {
			slog.Warn("tracing not initialized", "error", err)
		} else {
			defer func() {
				if err := observability.ShutdownTracer(context.Background()); err != nil {
					slog.Debug("tracer shutdown", "error", err)
				}
			}()
		}
	}

	// Metrics on the dashboard's own port when telemetry.metrics.enabled, so a deployment
	// that already runs the server does not need a second process to be scraped.
	var metricsHandler http.Handler
	if cfg.Telemetry.Metrics.Enabled {
		metricsHandler = observability.InitGlobal(versionInfo.Version).Handler()
	}

	server := httpserver.NewServer(httpserver.ServerDeps{
		Config:          dashboardCfg,
		Frontend:        frontend,
		ReleaseServices: releaseServices,
		Observability:   observabilitySvc,
		Metrics:         metricsHandler,
		MetricsPath:     metricsPath(),
	})

	// Wire up WebSocket event broadcasting
	if app != nil {
		broadcaster := server.EventBroadcaster()
		app.SubscribeToEvents(func(event release.DomainEvent) {
			// Broadcast events asynchronously to WebSocket clients
			broadcaster.PublishAsync(context.Background(), event)

			// Start the health watch here rather than in the release path. A window is
			// minutes or hours long and `relicta publish` exits in seconds, so a watch
			// started there would be killed before it observed anything — and a watch that
			// dies silently is worse than none, because the dashboard shows a release being
			// monitored that nobody is monitoring.
			startHealthWatch(observabilitySvc, event)
		})
		slog.Debug("WebSocket event broadcasting enabled")
	}

	// Watch releases published by other processes too.
	//
	// The event subscription above only hears what this process raises, and this process
	// never publishes — `relicta publish` is a separate command, usually on a developer's
	// machine or in CI. Without this the health watch would have been wired to a signal that
	// never arrives, which is the defect the whole observability pass exists to remove.
	if observabilitySvc != nil && releaseServices != nil {
		go watchPublishedReleases(ctx, observabilitySvc, releaseServices, repositoryRoot(ctx), cfg.Observability)
	}

	// Print startup message
	fmt.Printf("Starting Relicta dashboard server on %s\n", address)
	fmt.Printf("Press Ctrl+C to stop\n\n")

	if dashboardCfg.Auth.Mode == config.DashboardAuthNone {
		fmt.Println(styles.Warning.Render("WARNING: Authentication is disabled. Not recommended for production."))
	}

	if dashboardCfg.ServerMode == config.ServerModeAPI {
		fmt.Println(styles.Info.Render("Running in API-only mode (frontend disabled via --mode api)"))
	} else if frontend == nil {
		fmt.Println(styles.Info.Render("Running in API-only mode (no frontend embedded)"))
	}

	fmt.Printf("\nAPI endpoints:\n")
	fmt.Printf("  Health:     http://%s/healthz\n", resolveDisplayAddress(address))
	fmt.Printf("  Readiness:  http://%s/readyz\n", resolveDisplayAddress(address))
	fmt.Printf("  API:        http://%s/api/v1/\n", resolveDisplayAddress(address))
	fmt.Printf("  WebSocket:  ws://%s/api/v1/ws\n", resolveDisplayAddress(address))
	fmt.Printf("  SSE:        http://%s/api/v1/events/stream\n", resolveDisplayAddress(address))

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

	// The repository root, not the working directory.
	//
	// Started from a subdirectory, os.Getwd() rooted the release store there — so the
	// dashboard served an empty release history for a repository that had one, exactly the
	// way `relicta cancel` reported "no release run found" before #246. The git service the
	// container already built answers this correctly from anywhere in the tree.
	repoRoot := ""
	if info, infoErr := app.GitAdapter().GetInfo(ctx); infoErr == nil {
		repoRoot = info.Path
	}
	if repoRoot == "" {
		// Outside a repository the dashboard still starts; release services simply have
		// nothing to serve, which the caller below already tolerates.
		var err error
		repoRoot, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
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

// buildObservabilityService assembles the observability subsystem the configuration describes.
//
// Returns a nil interface when no provider is configured. That nil has to survive the trip
// into ServerDeps: a typed nil pointer wrapped in an interface is not nil, and the handlers
// would then answer from a subsystem with no providers — reporting nothing wrong because it is
// asking nobody, which is the one outcome this whole area is meant to avoid.
//
// A misconfigured provider is a startup error rather than a warning. Serving a dashboard whose
// health panel is blank because a provider name was misspelled is worse than not starting.
func buildObservabilityService(appConfig *config.Config, app *container.App) (handlers.ObservabilityService, error) {
	if appConfig == nil {
		return nil, nil
	}

	// Correlation needs the governance memory to know which releases an incident could
	// belong to. Without it the engine is left nil and correlations come back empty, which
	// the route already distinguishes from "no correlations found".
	var store cgpmemory.Store
	if app != nil && app.HasMemory() {
		store = app.MemoryStore()
	}

	repository := governanceRepository(appConfig, app)

	svc, err := observability.NewService(appConfig.Observability, store, repository)
	if err != nil {
		return nil, fmt.Errorf("observability: %w", err)
	}
	if svc == nil {
		return nil, nil
	}

	// The recorder writes a measured failure into the governance memory as an incident
	// against the release. Passing nil here would have made `auto_record` a setting that
	// gates nothing — the exact shape this work exists to remove — since WithHealthMonitor
	// only ever clears a recorder it was given.
	//
	// nil when there is no governance memory to write to: the monitor then keeps observing
	// and reporting, and the dashboard shows health that nothing records.
	recorder := observability.NewOutcomeRecorder(store, repository)
	return svc.WithHealthMonitor(appConfig.Observability, recorder), nil
}

// governanceRepository names the repository incidents are filed under, so a health incident
// and the release it belongs to end up under one name.
func governanceRepository(appConfig *config.Config, app *container.App) string {
	if app != nil {
		if info, err := app.GitAdapter().GetInfo(context.Background()); err == nil {
			return info.GovernanceID()
		}
	}
	if appConfig != nil {
		return appConfig.Changelog.RepositoryURL
	}
	return ""
}

// startHealthWatch begins monitoring a release the server just heard about.
//
// Only for a published release: a plan or an approval has not been deployed, and watching one
// would attribute whatever the metrics show to a release that has not shipped.
func startHealthWatch(svc handlers.ObservabilityService, event release.DomainEvent) {
	watcher, ok := svc.(interface {
		StartWatch(context.Context, string) error
	})
	if !ok || watcher == nil {
		return
	}

	published, ok := event.(*release.RunPublishedEvent)
	if !ok {
		return
	}

	if err := watcher.StartWatch(context.Background(), string(published.RunID)); err != nil {
		slog.Warn("health watch not started", "release_id", published.RunID, "error", err)
	}
}

// watchPublishedReleases starts a health watch for releases published recently, wherever they
// were published from.
//
// Polls rather than subscribes because the publish happens in another process: `relicta
// publish` writes the run to the store this server reads. The poll is cheap — a state query
// against runs the server already has open — and the alternative is a health watch that only
// covers releases published inside the dashboard, which publishes none.
//
// Only releases published within the monitoring window are picked up. Watching a release from
// last week would attribute today's metrics to it, which is the wrong-data failure one level
// along from the one ADR-016 is about.
func watchPublishedReleases(
	ctx context.Context,
	svc handlers.ObservabilityService,
	services *release.Services,
	repoRoot string,
	cfg config.ObservabilityConfig,
) {
	watcher, ok := svc.(interface {
		StartWatch(context.Context, string) error
	})
	if !ok || services == nil || services.Repository == nil {
		return
	}

	window := cfg.HealthCheck.Window
	if window <= 0 {
		window = 30 * time.Minute
	}

	// Every interval, not every publish: a release published while the server was down is
	// still worth watching if it is inside the window.
	interval := time.Minute
	if interval > window {
		interval = window
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	watched := make(map[string]struct{})

	// Once before the first tick. A release published moments before the server started is
	// the most likely one to want watching, and waiting a full interval can age it out of the
	// window entirely — with a short window, every release would be missed.
	pickUpPublishedReleases(ctx, watcher, services, repoRoot, window, watched)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pickUpPublishedReleases(ctx, watcher, services, repoRoot, window, watched)
		}
	}
}

// pickUpPublishedReleases starts a watch for each release published inside the window that is
// not already being watched.
func pickUpPublishedReleases(
	ctx context.Context,
	watcher interface {
		StartWatch(context.Context, string) error
	},
	services *release.Services,
	repoRoot string,
	window time.Duration,
	watched map[string]struct{},
) {
	runs, err := services.Repository.FindByState(ctx, repoRoot, release.StatePublished)
	if err != nil {
		slog.Debug("could not list published releases to watch", "error", err)
		return
	}

	for _, run := range runs {
		id := string(run.ID())
		publishedAt := run.PublishedAt()
		if publishedAt == nil || time.Since(*publishedAt) > window {
			continue
		}
		if _, already := watched[id]; already {
			continue
		}
		watched[id] = struct{}{}
		if err := watcher.StartWatch(ctx, id); err != nil {
			slog.Debug("health watch not started", "release_id", id, "error", err)
		} else {
			slog.Info("watching published release", "release_id", id)
		}
	}
}

// tracingServiceName is what spans are attributed to.
//
// There is no service_name setting, so this is the tool's own name. Named rather than inlined
// because a repository-specific name is the obvious next thing somebody will want, and this is
// where it goes.
func tracingServiceName(*config.Config) string {
	return "relicta"
}
