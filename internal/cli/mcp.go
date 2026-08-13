package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/container"
	"github.com/relicta-tech/relicta/v4/internal/mcp"
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
AI clients that support the Model Context Protocol (MCP). You can also
run over HTTP transport for remote/custom integrations.

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

var (
	mcpServeTransport string
	mcpServePort      string
	mcpServeAddress   string
)

func init() {
	mcpCmd.AddCommand(mcpServeCmd)
	mcpServeCmd.Flags().StringVar(&mcpServeTransport, "transport", "stdio", "transport to use: stdio or http")
	mcpServeCmd.Flags().StringVar(&mcpServePort, "port", "", "port for HTTP transport (implies --transport=http)")
	mcpServeCmd.Flags().StringVar(&mcpServeAddress, "address", "", "address for HTTP transport, e.g. :8080 or 127.0.0.1:8080")
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

	// initContainerAndAdapter initializes the container and creates an MCP adapter.
	// Used both at startup and for hot-reload after relicta_init (fixes #83).
	initContainerAndAdapter := func(c *config.Config) (*container.App, *mcp.Adapter, error) {
		newApp, err := container.NewInitialized(ctx, c)
		if err != nil {
			return nil, nil, err
		}

		repoRoot, err := os.Getwd()
		if err != nil {
			mcpLogger.Warn("failed to get working directory for release services", "error", err)
		} else {
			if err := newApp.InitReleaseServices(ctx, repoRoot); err != nil {
				mcpLogger.Warn("failed to initialize release services", "error", err)
			}
		}

		adapter := createMCPAdapter(newApp)
		return newApp, adapter, nil
	}

	// Initialize container to get use cases
	var app *container.App
	if cfg != nil {
		var adapter *mcp.Adapter
		var err error
		app, adapter, err = initContainerAndAdapter(cfg)
		if err != nil {
			mcpLogger.Warn("failed to initialize container, tools will return stubs", "error", err)
		} else {
			opts = append(opts, mcp.WithAdapter(adapter))

			// ensureRepoPath resolves the repository root through this service and
			// falls back to "." without it. WithGitService was never called, so the
			// fallback was always taken: started from a subdirectory, the MCP server
			// reported "No active release found" for a repository that had a planned
			// release — silently wrong rather than an error. The comment on
			// ensureRepoPath cites issue #35 as fixed, and it was, apart from the
			// service it depends on never being supplied.
			if gitSvc := app.Git(); gitSvc != nil {
				opts = append(opts, mcp.WithGitService(gitSvc))
			}

			// The three cgp_* protocol tools are advertised in tools/list and every
			// call to them failed: they need either WithCGPService or WithEvaluator,
			// and neither was ever wired, so ensureCGPService took its error path on
			// every request. An agent reading the tool list saw three governance
			// tools and could use none of them.
			//
			// Handing over the governance service's own evaluator rather than a
			// fresh one is the point. A fresh evaluator carries default thresholds
			// and no policies, so cgp_propose would have decided by different rules
			// than relicta_evaluate and `relicta approve` — two governance verdicts
			// for one change, with nothing saying which was authoritative. This way
			// the protocol surface and the CLI share one set of rules.
			if govSvc := app.GovernanceService(); govSvc != nil {
				if eval := govSvc.Evaluator(); eval != nil {
					opts = append(opts, mcp.WithEvaluator(eval))
				}
			}

			// Five resources — release state, active runs, history, the run's
			// recommendation — answered "not available" because nothing supplied the
			// repository, though the container had it all along. An agent asking what
			// release is in progress got a stub, which reads as "no release" rather
			// than "this server was not wired".
			if repo := app.ReleaseRepository(); repo != nil {
				opts = append(opts, mcp.WithReleaseRepository(repo))
			}

			// The operator's per-actor autonomy budgets. WithActorBudgets was never
			// called, so a configured governance.actor_budget_path gated the CLI and
			// was ignored here — on the surface agents actually use, which is what
			// per-actor budgets are for. ResolveBudget's fallback is restrictive, so
			// nothing was unsafe by default; what went missing was the operator's own
			// policy, whether it widened a trusted agent's budget or tightened one
			// past the default.
			if budgets, budgetErr := loadActorBudgetSet(); budgetErr != nil {
				// Refused rather than ignored: continuing would apply defaults while
				// the operator believes their file is in force.
				return fmt.Errorf("failed to load actor budgets for the MCP server: %w", budgetErr)
			} else if budgets != nil {
				opts = append(opts, mcp.WithActorBudgets(budgets))
			}
		}
	}

	// Defer cleanup — uses app pointer which may be updated by the reloader.
	// Use CloseWithTimeout directly to satisfy contextcheck linter.
	defer func() {
		if app != nil {
			if closeErr := app.CloseWithTimeout(10 * time.Second); closeErr != nil {
				mcpLogger.Warn("failed to close container", "error", closeErr)
			}
		}
	}()

	// Config reloader for hot-reload after relicta_init (fixes #83).
	// When init creates .relicta.yaml mid-session, this reloads config and
	// reinitializes the container so subsequent commands work immediately.
	opts = append(opts, mcp.WithConfigReloader(func(reloadCtx context.Context) (mcp.ReloadedComponents, error) {
		mcpLogger.Info("reloading config after init")

		// Re-load config from disk
		newCfg, err := config.NewLoader().Load()
		if err != nil {
			return mcp.ReloadedComponents{}, fmt.Errorf("failed to load config: %w", err)
		}

		// Close old container if it exists
		if app != nil {
			if closeErr := app.CloseWithTimeout(10 * time.Second); closeErr != nil {
				mcpLogger.Warn("failed to close old container during reload", "error", closeErr)
			}
		}

		// Initialize new container and adapter
		newApp, newAdapter, err := initContainerAndAdapter(newCfg)
		if err != nil {
			return mcp.ReloadedComponents{Config: newCfg}, fmt.Errorf("config loaded but container init failed: %w", err)
		}

		// Update the outer app reference for cleanup on shutdown
		app = newApp
		cfg = newCfg

		// The evaluator has to come back too. This reload path exists for the case
		// where no config existed at startup, which is exactly the case where the
		// evaluator was nil — so without this, `relicta_init` reported that tools
		// were available while the three cgp_* tools stayed broken.
		reloaded := mcp.ReloadedComponents{Config: newCfg, Adapter: newAdapter}
		if govSvc := newApp.GovernanceService(); govSvc != nil {
			reloaded.Evaluator = govSvc.Evaluator()
		}
		return reloaded, nil
	}))

	// Create and start MCP server
	server, err := mcp.NewServer(versionInfo.Version, opts...)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	// Log startup
	hasAdapter := app != nil
	transport, address, err := resolveMCPServeTransport()
	if err != nil {
		return err
	}

	switch transport {
	case "stdio":
		mcpLogger.Info("starting MCP server",
			"version", versionInfo.Version,
			"transport", "stdio",
			"tools_wired", hasAdapter,
		)
		return server.ServeStdio()
	case "http":
		mcpLogger.Info("starting MCP server",
			"version", versionInfo.Version,
			"transport", "http",
			"address", address,
			"tools_wired", hasAdapter,
		)
		return server.ServeHTTP(ctx, address)
	default:
		return fmt.Errorf("unsupported MCP transport: %s", transport)
	}
}

func resolveMCPServeTransport() (transport string, address string, err error) {
	transport = mcpServeTransport
	if transport == "" {
		transport = "stdio"
	}

	// Port/address imply HTTP transport.
	if (mcpServePort != "" || mcpServeAddress != "") && transport == "stdio" {
		transport = "http"
	}

	switch transport {
	case "stdio":
		if mcpServeAddress != "" || mcpServePort != "" {
			return "", "", fmt.Errorf("--address/--port can only be used with --transport=http")
		}
		return "stdio", "", nil
	case "http":
		if mcpServeAddress != "" {
			return "http", mcpServeAddress, nil
		}
		if mcpServePort != "" {
			return "http", ":" + mcpServePort, nil
		}
		return "http", ":8080", nil
	default:
		return "", "", fmt.Errorf("invalid transport %q (supported: stdio, http)", transport)
	}
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

	// Adapter.Evaluate needs the repository to find the run by ID, and refuses
	// with "release repository not configured" without it. WithAdapterRepo was
	// defined and called from nowhere, so relicta_evaluate could not run on any
	// repository — and because tool errors were being redacted, an agent saw only
	// "internal error" and never learned why.
	if repo := app.ReleaseRepository(); repo != nil {
		opts = append(opts, mcp.WithAdapterRepo(repo))
	}

	// Wire blast radius service for monorepo analysis
	if app.HasBlastService() {
		opts = append(opts, mcp.WithBlastService(app.BlastService()))
	}

	// Wire AI service for diff summarization
	if app.HasAI() {
		opts = append(opts, mcp.WithAIService(app.AI()))
	}

	// Report the real version in recommendation provenance, so an artifact can be
	// attributed to the binary that produced it.
	opts = append(opts, mcp.WithToolVersion(versionInfo.Version))

	// Without this the MCP surface plans against "v" regardless of configuration.
	opts = append(opts, mcp.WithTagPrefix(configuredTagPrefix()))

	return mcp.NewAdapter(opts...)
}
