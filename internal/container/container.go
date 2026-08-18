// Package container provides dependency injection for Relicta services.
package container

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	analysisfactory "github.com/relicta-tech/relicta/v4/internal/analysis/factory"
	"github.com/relicta-tech/relicta/v4/internal/analytics"
	"github.com/relicta-tech/relicta/v4/internal/application/blast"
	"github.com/relicta-tech/relicta/v4/internal/application/governance"
	"github.com/relicta-tech/relicta/v4/internal/application/versioning"
	"github.com/relicta-tech/relicta/v4/internal/cgp/audit"
	cgpmemory "github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/communication"
	"github.com/relicta-tech/relicta/v4/internal/domain/integration"
	domainrelease "github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
	"github.com/relicta-tech/relicta/v4/internal/errors"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/ai"
	chronosinfra "github.com/relicta-tech/relicta/v4/internal/infrastructure/chronos"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
	memoryinfra "github.com/relicta-tech/relicta/v4/internal/infrastructure/memory"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/persistence"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/webhook"
	"github.com/relicta-tech/relicta/v4/internal/plugin"
	servicerelease "github.com/relicta-tech/relicta/v4/internal/service/release"
)

// defaultShutdownTimeout is the default timeout for graceful shutdown of components.
const defaultShutdownTimeout = 10 * time.Second

// Closeable represents a component that can be closed/shutdown.
type Closeable interface {
	Close() error
}

// App provides dependency injection for Relicta services.
// It manages infrastructure, application use cases, and service lifecycle.
type App struct {
	config *config.Config
	logger *slog.Logger
	mu     sync.RWMutex
	closed bool

	// repoPath is the repository this container serves, empty for the one relicta was
	// invoked in.
	//
	// Everything else that needs a path derives from c.gitService, so this is the single
	// point that decides which repository a container operates on. It exists because a
	// group release has to drive a member's checkout: without it, git.NewService() opens
	// the working directory, and release services pointed at a member's root would publish
	// the *invoking* repository's tags — silent misrouting on the one path where being
	// wrong is unrecoverable.
	repoPath string

	// allowUntrustedPlugins mirrors the CLI's --allow-untrusted-plugins flag.
	// Default false — plugin manager refuses to load plugins on best-effort
	// sandbox platforms until signing infrastructure ships. CLI sets this
	// before initPluginSystem runs.
	allowUntrustedPlugins bool

	// Infrastructure layer
	gitAdapter *git.Adapter

	// releaseRunStore is the adapter persistence.backend selected, held so the release
	// services get the same one the bridge wraps. Resolved in initInfrastructure; nil only
	// in a container that was never initialized.
	releaseRunStore ports.ReleaseRunRepository

	releaseRepo        domainrelease.Repository
	baseEventPublisher *persistence.InMemoryEventPublisher
	eventPublisher     domainrelease.EventPublisher // Composed publisher chain
	unitOfWorkFactory  *persistence.FileUnitOfWorkFactory
	versionCalc        version.VersionCalculator
	pluginRegistry     integration.PluginRegistry
	pluginExecutor     integration.PluginExecutor
	pluginManager      *plugin.Manager
	memoryStore        cgpmemory.Store

	// governanceID is the repository's governance identity ("owner/repo"), resolved once
	// in initInfrastructure and reused by everything that records against it.
	//
	// Resolved once because it comes from the git remote and two derivations of it
	// eventually disagree — which shows up as one repository's history split in two, and
	// as an audit chain whose entries are filed under a key nothing reads.
	governanceID string

	// Cognitive layer (optional — wired when configured)
	mnemosStore   cgpmemory.Store              // Mnemos-backed memory store (optional)
	chronosClient *chronosinfra.ChronosAdapter // Chronos pattern detection client

	// Services (existing infrastructure)
	gitService   git.Service
	aiService    ai.Service
	blastService blast.Service

	// Application layer use cases
	releaseAnalyzer    *servicerelease.Analyzer
	calculateVersionUC *versioning.CalculateVersionUseCase

	// TagCreator adapter for tag operations in publish step
	tagCreator *TagCreatorAdapter

	// Governance service (CGP)
	governanceService *governance.Service

	// Analytics service captures governance events (risk evaluations,
	// policy decisions, approval outcomes) for trend reporting.
	analyticsService *analytics.Service

	// Release workflow services (domain use cases)
	releaseServices *domainrelease.Services

	// Cleanup tracking
	closeables []Closeable
}

// New creates a new App container with the given configuration.
func New(cfg *config.Config) (*App, error) {
	if cfg == nil {
		return nil, errors.Config("New", "configuration is required")
	}

	return &App{
		config:     cfg,
		logger:     slog.Default(),
		closeables: make([]Closeable, 0),
	}, nil
}

// NewForRepo creates a container that operates on the repository at repoPath rather than the
// process working directory.
//
// For driving a repository that is not the one relicta was invoked in — a group member. An
// empty repoPath behaves exactly like New, so this is not a second code path for the ordinary
// case.
func NewForRepo(cfg *config.Config, repoPath string) (*App, error) {
	app, err := New(cfg)
	if err != nil {
		return nil, err
	}
	app.repoPath = repoPath
	return app, nil
}

// registerCloseable registers a component for cleanup during shutdown.
func (c *App) registerCloseable(closeable Closeable) {
	if closeable != nil {
		c.closeables = append(c.closeables, closeable)
	}
}

// RegisterCloseable allows external components to register for cleanup during shutdown.
// Components are closed in reverse order of registration (LIFO).
// SetAllowUntrustedPlugins records the operator's --allow-untrusted-plugins
// opt-in. Must be called before Initialize() — once initPluginSystem runs the
// plugin manager has already been built with the previous setting.
func (c *App) SetAllowUntrustedPlugins(allow bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.allowUntrustedPlugins = allow
}

func (c *App) RegisterCloseable(closeable Closeable) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.registerCloseable(closeable)
}

// Initialize initializes all layers of the application container.
func (c *App) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return errors.State("Initialize", "container is closed")
	}

	// Initialize infrastructure layer
	if err := c.initInfrastructure(ctx); err != nil {
		return err
	}

	// Initialize application layer
	return c.initApplicationLayer(ctx)
}

// initInfrastructure initializes infrastructure layer components.
func (c *App) initInfrastructure(ctx context.Context) error {
	var err error

	// Initialize existing git service, on this container's repository.
	//
	// Every other path in this container is derived from this service, so scoping it here
	// scopes all of them. Nothing passed means the working directory, which is what every
	// caller but a group release wants.
	gitOpts := []git.ServiceOption{}
	if c.repoPath != "" {
		gitOpts = append(gitOpts, git.WithRepoPath(c.repoPath))
	}
	c.gitService, err = git.NewService(gitOpts...)
	if err != nil {
		return errors.GitWrap(err, "initInfrastructure", "failed to initialize git service")
	}

	// Create git adapter that implements domain interface
	c.gitAdapter = git.NewAdapter(c.gitService)

	// Initialize release repository, anchored to the repository root.
	//
	// This was the relative path ".relicta/releases", resolved against the process
	// working directory. Run from a subdirectory, it therefore pointed at a
	// directory that did not exist, with two visible consequences: `relicta cancel`
	// reported "No release run found" for a repository that had a planned run —
	// while printing the correct root in the same message, because only the message
	// resolved it — and NewFileReleaseRepository's MkdirAll created a stray
	// .relicta/releases in whatever subdirectory the command ran from, so merely
	// invoking relicta littered the working tree.
	//
	// Falling back to the relative path keeps this working outside a repository,
	// where commands that do not need a release store still construct the container.
	// One store. The bridge exposes the release services' repository through the
	// interface the CLI commands already use, so `cancel`, `clean`, `rollback`,
	// `bump` and `approve` read the same runs `plan` wrote — see
	// release_repo_bridge.go for why there were two.
	repoRoot, rootErr := c.gitService.GetRepositoryRoot(ctx)
	if rootErr != nil || repoRoot == "" {
		// Outside a repository. Commands that do not need a release store still
		// construct the container, and the bridge's methods will fail with a clear
		// error if one is reached, rather than reading some directory relative to
		// the caller's cwd.
		repoRoot = ""
	}

	// Initialize event publisher chain:
	// OutcomeTracker → WebhookPublisher → InMemoryEventPublisher
	c.baseEventPublisher = persistence.NewInMemoryEventPublisher()

	// Start with base publisher
	var publisher domainrelease.EventPublisher = c.baseEventPublisher

	// Add webhook publisher if webhooks are configured
	if len(c.config.Webhooks) > 0 {
		webhookPublisher := webhook.NewPublisher(c.config.Webhooks, publisher)
		// Registered so shutdown waits for deliveries in flight. Each command is its own
		// process, so without this the goroutine carrying a delivery is killed when the
		// command returns and the webhook simply never arrives.
		c.registerCloseable(webhookPublisher)
		publisher = webhookPublisher
		c.logger.Debug("webhook publisher initialized", "webhook_count", len(c.config.Webhooks))
	}

	// Add outcome tracker if governance memory is enabled
	if c.config.Governance.MemoryEnabled {
		// Resolve persistence.backend, once, for everything in this container that needs a
		// governance memory store — the outcome tracker here and the governance service in
		// initGovernanceService, which used to open its own.
		//
		// The path this resolves for the file backend is governance.MemoryStorePath. That
		// used to be a cwd-relative ".relicta/memory" here while every reader — `relicta
		// history`, the DORA and SOC 2 reports, the deployment gate, `hub sync` — read
		// ".relicta/governance/memory.json" against the repository root, so the tracker
		// wrote where nothing looked. Reading and writing one store is the whole point of a
		// store, and that is now true of the backend as well as the path.
		memoryStore, memErr := OpenGovernanceMemory(ctx, c.config, repoRoot)
		switch {
		case memErr != nil && c.config.Persistence.Backend != "" &&
			c.config.Persistence.Backend != config.BackendFile:
			// A database the operator asked for and relicta could not open is the
			// command's error. Warning and continuing would leave the tracker unbuilt,
			// so the release would publish, report success, and record nothing at all —
			// the shape of defect ADR-013 exists to remove, with the audit trail as the
			// thing that goes missing.
			return errors.ConfigWrap(memErr, "initInfrastructure",
				"failed to open the governance memory store")
		case memErr != nil:
			// The file backend keeps the behavior it has always had. Nothing was
			// selected here, so nothing fell back: this is a local filesystem refusing
			// the directory relicta would have created itself, and downgrading to no
			// historical tracking is what every previous release did. ADR-013 flips the
			// default on evidence and in its own change, so this branch is deliberately
			// not tightened along with the one above.
			c.logger.Warn("failed to initialize memory store", "error", memErr)
		default:
			c.memoryStore = memoryStore.Store
			// Registered before anything can use it, so a connection is released on
			// shutdown even if a later step of initialization fails. Nil for the file
			// backend, which holds none, and registerCloseable already ignores that.
			if memoryStore.Closer != nil {
				c.registerCloseable(memoryStore.Closer)
			}
			c.logger.Debug("governance memory store opened",
				"backend", string(memoryStore.Backend), "location", memoryStore.Location)
			// The same governance identity the CLI's recordPublishOutcome records
			// against, resolved once here. The tracker's own per-run context cache is
			// empty in a fresh process, so without this a terminal event arriving on its
			// own — which is every `relicta cancel` — produced a record the store
			// rejected for having no repository.
			governanceID := ""
			if repoInfo, infoErr := c.gitAdapter.GetInfo(ctx); infoErr == nil {
				governanceID = repoInfo.GovernanceID()
			}
			c.governanceID = governanceID
			publisher = cgpmemory.NewOutcomeTracker(c.memoryStore, publisher, governanceID)
			c.logger.Debug("outcome tracker initialized",
				"location", memoryStore.Location, "repository", governanceID)

			// The audit chain sits outside the outcome tracker so that it sees the
			// same events, unfiltered, and appends its evidence before the tracker's
			// record is written. Order between the two does not matter for
			// correctness — neither reads the other — but it does for reading the
			// code: the chain is the outermost thing in the release's path, which is
			// what "recorded at the moment it happened" means.
			//
			// Skipped without a governance identity, rather than filed under "".
			// Entries for every repository that failed to resolve one would land in
			// a single chain, verify perfectly, and attribute one project's releases
			// to another.
			if governanceID != "" {
				publisher = audit.NewEventRecorder(c.memoryStore, governanceID, publisher)
				c.logger.Debug("audit chain recorder initialized",
					"repository", governanceID)
			}
		}
	}

	// Initialize the Mnemos client (optional — reachable through MnemosStore()).
	//
	// It used to end with `c.memoryStore = c.mnemosStore`, which made it a second answer to
	// "which governance store", and now that persistence.backend gives the first answer the
	// two cannot both stand. The assignment also never did what its comment claimed: the
	// outcome tracker above was already built around the store resolved before this block, so
	// with mnemos enabled — which is the default — the tracker wrote to the file store while
	// the accessor handed out the Mnemos adapter. Nothing in the tree calls MemoryStore(), so
	// the only thing the line achieved was to make the two disagree, and had it reached the
	// governance service it would have silenced every governance record in the default
	// configuration: the Mnemos adapter logs and drops a write when no daemon answers.
	//
	// Mnemos is unchanged and unaffected — MnemosStore() returns it, HasMnemos() reports it.
	// It is a cognitive backend, not a persistence backend, and persistence.backend is where
	// the governance store is chosen.
	if c.config.Mnemos.Enabled {
		mnemosEndpoint := c.config.Mnemos.Endpoint
		if mnemosEndpoint == "" {
			mnemosEndpoint = "http://localhost:7777"
		}
		mnemosTimeout := c.config.Mnemos.Timeout
		if mnemosTimeout == 0 {
			mnemosTimeout = 10 * time.Second
		}
		mnemosNamespace := c.config.Mnemos.Namespace
		if mnemosNamespace == "" {
			// Derive from repo name if possible
			if repoInfo, err := c.gitAdapter.GetInfo(ctx); err == nil {
				mnemosNamespace = repoInfo.Name
			}
		}
		c.mnemosStore = memoryinfra.NewMnemosStore(
			mnemosEndpoint,
			mnemosNamespace,
			&http.Client{Timeout: mnemosTimeout},
		)
		c.logger.Info("Mnemos memory backend initialized", "endpoint", mnemosEndpoint, "namespace", mnemosNamespace)
	}

	// Initialize Chronos pattern detection client (optional)
	if c.config.Chronos.Enabled {
		chronosEndpoint := c.config.Chronos.Endpoint
		if chronosEndpoint == "" {
			chronosEndpoint = "http://localhost:7778"
		}
		chronosTimeout := c.config.Chronos.Timeout
		if chronosTimeout == 0 {
			chronosTimeout = 10 * time.Second
		}
		chronosThreads := c.config.Chronos.Threads
		if chronosThreads <= 0 {
			chronosThreads = 4
		}
		c.chronosClient = chronosinfra.NewChronosAdapterWithConfig(chronosEndpoint, "relicta", chronosThreads, chronosTimeout)
		c.logger.Info("Chronos pattern detection initialized", "endpoint", chronosEndpoint)
	}

	c.eventPublisher = publisher

	// Resolve persistence.backend, once, for everything in this container that needs a
	// release run repository.
	//
	// This is the point of ADR-013's first phase: the setting had never been read, so a
	// team that configured PostgreSQL for shared governance state got JSON files in each
	// developer's working copy and a command that reported success. It is resolved here
	// rather than at each use site for the same reason the ADR has one conformance suite
	// for three adapters — two selection sites are two answers waiting to disagree, and
	// the way that shows up is `relicta plan` writing to a database while `relicta cancel`
	// reads a file.
	//
	// A failure to open is returned, not logged: falling back to files for someone who
	// asked for postgres is the exact defect this replaces.
	releaseStore, storeErr := persistence.OpenReleaseRunStore(ctx, c.config.Persistence, repoRoot)
	if storeErr != nil {
		return errors.ConfigWrap(storeErr, "initInfrastructure", "failed to open the release run store")
	}
	// Registered before anything can use it, so a connection is released on shutdown even
	// if a later step of initialization fails. Nil for the file backend, which holds none,
	// and registerCloseable already ignores that.
	if releaseStore.Closer != nil {
		c.registerCloseable(releaseStore.Closer)
	}
	c.releaseRunStore = releaseStore.Repository
	c.logger.Debug("release run store opened",
		"backend", string(releaseStore.Backend), "location", releaseStore.Location)

	// The bridge is built here, after the chain, because it carries it: the commands
	// that save through app.ReleaseRepository() — cancel, clean, rollback, bump,
	// approve — must publish their events too. Built before the chain existed, the
	// bridge wrapped a bare repository, so canceling a release recorded nothing and
	// change failure rate never saw a canceled run.
	c.releaseRepo = newReleaseRepoBridge(repoRoot, c.eventPublisher, c.releaseRunStore)

	// Initialize UnitOfWork factory for transactional operations.
	//
	// The composed chain, not the bare in-memory publisher. The unit of work is what
	// publishes a release's events after persisting it, and it was handed
	// baseEventPublisher — so the outcome tracker and the webhook publisher were built,
	// logged as initialized, assigned to c.eventPublisher, and then bypassed by the only
	// code path that emits events. Configured webhooks were never delivered for a
	// release, and no run that failed or was canceled was ever recorded, since the CLI's
	// own recordPublishOutcome covers the publish path alone. Nothing failed; the
	// behavior was simply absent.
	c.unitOfWorkFactory = persistence.NewFileUnitOfWorkFactory(c.releaseRepo, c.eventPublisher)

	// Initialize version calculator
	c.versionCalc = version.NewDefaultVersionCalculator()

	// Initialize plugin system
	if pluginErr := c.initPluginSystem(ctx); pluginErr != nil {
		// Plugin system failure is non-fatal, use empty executor
		c.logger.Warn("plugin system initialization failed, using empty executor", "error", pluginErr)
		c.pluginRegistry = integration.NewInMemoryPluginRegistry()
		c.pluginExecutor = integration.NewSequentialPluginExecutor(c.pluginRegistry)
	}

	// Initialize AI service (optional)
	if c.config.AI.Enabled {
		c.aiService, err = c.initAIService(ctx)
		if err != nil {
			// AI service failure is non-fatal
			c.aiService = nil
		}
	}

	return nil
}

// initAIService initializes the AI service based on configuration.
func (c *App) initAIService(ctx context.Context) (ai.Service, error) {
	// Check for early cancellation
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	provider := c.config.AI.Provider

	// Determine if this provider requires an API key
	// Ollama runs locally and doesn't need authentication
	requiresAPIKey := provider != "ollama"

	apiKey := c.config.AI.APIKey
	if apiKey == "" {
		// Try provider-specific environment variables first, then fall back to OPENAI_API_KEY
		switch provider {
		case "anthropic", "claude":
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		case "gemini":
			apiKey = os.Getenv("GEMINI_API_KEY")
		case "azure-openai":
			apiKey = os.Getenv("AZURE_OPENAI_KEY")
			if apiKey == "" {
				apiKey = os.Getenv("AZURE_OPENAI_API_KEY")
			}
		}
		// Fall back to OPENAI_API_KEY for OpenAI or if provider-specific not set
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
	}

	if requiresAPIKey && apiKey == "" {
		return nil, errors.AI("initAIService", "API key not configured for provider: "+provider)
	}

	opts := []ai.ServiceOption{
		ai.WithProvider(provider),
		ai.WithModel(c.config.AI.Model),
	}

	// Only add API key option if we have one
	if apiKey != "" {
		opts = append(opts, ai.WithAPIKey(apiKey))
	}

	if c.config.AI.BaseURL != "" {
		opts = append(opts, ai.WithBaseURL(c.config.AI.BaseURL))
	}

	if c.config.AI.APIVersion != "" {
		opts = append(opts, ai.WithAPIVersion(c.config.AI.APIVersion))
	}

	if c.config.AI.MaxTokens > 0 {
		opts = append(opts, ai.WithMaxTokens(c.config.AI.MaxTokens))
	}

	if c.config.AI.Temperature > 0 {
		opts = append(opts, ai.WithTemperature(c.config.AI.Temperature))
	}

	if c.config.AI.Timeout > 0 {
		opts = append(opts, ai.WithTimeout(time.Duration(c.config.AI.Timeout)*time.Second))
	}

	// Note: ai.NewService is a pure constructor that only configures the service.
	// No network calls occur during construction; actual API calls happen in Generate()
	// which accepts context for cancellation. Lazy initialization was considered but
	// adds complexity; eager init is acceptable since this only runs when AI is enabled.
	return ai.NewService(opts...)
}

// initPluginSystem initializes the plugin system.
// If plugins are configured, it uses the plugin.Manager with ExecutorAdapter.
// Otherwise, it uses an empty in-memory registry.
func (c *App) initPluginSystem(ctx context.Context) error {
	// If no plugins configured, use empty in-memory implementation
	if len(c.config.Plugins) == 0 {
		c.pluginRegistry = integration.NewInMemoryPluginRegistry()
		c.pluginExecutor = integration.NewSequentialPluginExecutor(c.pluginRegistry)
		return nil
	}

	// Create plugin manager for external gRPC plugins
	c.pluginManager = plugin.NewManager(c.config)

	// Honor operator's --allow-untrusted-plugins opt-in (or equivalent
	// programmatic toggle on App). Default false — the manager's trust gate
	// refuses load on best-effort sandbox platforms until signing ships.
	c.pluginManager.AllowUntrustedPlugins(c.allowUntrustedPlugins)

	// Register plugins for lazy loading (improves startup time)
	// Plugins will be loaded on-demand when hooks are executed
	c.pluginManager.RegisterPlugins()

	// Register manager for cleanup
	c.registerCloseable(c.pluginManager)

	// Create adapter that bridges Manager to PluginExecutor interface
	c.pluginExecutor = plugin.NewExecutorAdapter(c.pluginManager)

	// Use empty in-memory registry (external plugins are managed by Manager)
	c.pluginRegistry = integration.NewInMemoryPluginRegistry()

	return nil
}

// initApplicationLayer initializes application layer use cases.
func (c *App) initApplicationLayer(ctx context.Context) error {
	analysisFactory := analysisfactory.NewFactory(c.aiService)

	// Initialize release analyzer for commit analysis and version calculation
	c.releaseAnalyzer = servicerelease.NewAnalyzer(
		c.gitAdapter,
		c.versionCalc,
		analysisFactory,
	)

	// Initialize CalculateVersionUseCase
	c.calculateVersionUC = versioning.NewCalculateVersionUseCase(
		c.gitAdapter,
		c.versionCalc,
	)

	// Initialize TagCreator adapter for tag operations in publish step
	c.tagCreator = NewTagCreatorAdapter(c.gitAdapter, c.config.Versioning.GitSign)

	// Initialize blast radius service for monorepo analysis
	// Blast analyzes this container's repository, not the caller's. It took "." literally,
	// which for a member-scoped container would have analyzed the wrong tree.
	blastPath := c.repoPath
	if blastPath == "" {
		blastPath = "."
	}
	c.blastService = blast.NewService(
		blast.WithRepoPath(blastPath),
		blast.WithMonorepoConfig(blast.DefaultMonorepoConfig()),
	)

	// Initialize Governance service (CGP) if enabled
	if c.config.Governance.Enabled {
		if err := c.initGovernanceService(ctx); err != nil {
			// Governance failure is non-fatal in advisory mode
			c.logger.Warn("governance service initialization failed", "error", err)
		}
		// Analytics captures governance events for trend reporting. A
		// store failure is non-fatal — capture calls no-op on a nil service.
		if err := c.initAnalyticsService(ctx); err != nil {
			c.logger.Warn("analytics service initialization failed", "error", err)
		}
	}

	return nil
}

// initAnalyticsService initializes the governance analytics service backed by
// a file store under the repository's .relicta/governance/analytics directory.
func (c *App) initAnalyticsService(ctx context.Context) error {
	repoPath := "."
	if c.gitAdapter != nil {
		if info, err := c.gitAdapter.GetInfo(ctx); err == nil {
			repoPath = info.Path
		}
	}
	store, err := analytics.NewFileStore(filepath.Join(repoPath, ".relicta", "governance", "analytics"))
	if err != nil {
		return err
	}
	c.analyticsService = analytics.NewService(store)
	return nil
}

// initGovernanceService initializes the CGP governance service.
func (c *App) initGovernanceService(ctx context.Context) error {
	// Check for early cancellation
	if err := ctx.Err(); err != nil {
		return err
	}

	// Get repository path for memory storage
	repoPath := ""
	if c.gitAdapter != nil {
		info, err := c.gitAdapter.GetInfo(ctx)
		if err == nil {
			repoPath = info.Path
		}
	}

	var err error
	// The store initInfrastructure resolved, rather than one the service opens for
	// itself. persistence.backend is read in one place; a service that built its own
	// would be the second, and the two would disagree the moment the setting was not
	// `file` — the service recording releases in memory.json while the outcome tracker
	// recorded them in the database.
	c.governanceService, err = governance.NewServiceFromConfig(
		ctx,
		&c.config.Governance,
		repoPath,
		c.logger,
		c.memoryStore,
	)
	if err != nil {
		return errors.StateWrap(err, "initGovernanceService", "failed to create governance service")
	}

	c.logger.Info("governance service initialized",
		"strict_mode", c.config.Governance.StrictMode,
		"auto_approve_threshold", c.config.Governance.AutoApproveThreshold,
		"memory_enabled", c.config.Governance.MemoryEnabled,
	)

	return nil
}

// auditChainStore reports where the governance audit chain lives, for the attestation to
// anchor itself to.
//
// Both values or neither. A store with no governance identity has no chain to read: the
// event recorder was skipped for the same reason, so there is nothing recorded under any
// key this could guess, and the attestation should report an empty chain because there
// genuinely is one. Passing the store with an empty repository would instead fail the
// attestation with a configuration error, which is a worse answer to "we could not work
// out what this repository is called".
func (c *App) auditChainStore() (audit.Store, string) {
	if c.governanceID == "" || c.memoryStore == nil {
		return nil, ""
	}
	return c.memoryStore, c.governanceID
}

// initReleaseServices initializes the release workflow services.
func (c *App) initReleaseServices(ctx context.Context, repoRoot string) error {
	// Check for early cancellation
	if err := ctx.Err(); err != nil {
		return err
	}

	// Create port adapters
	notesGenerator := NewNotesGeneratorAdapter(c.aiService, c.gitAdapter, c.changelogRenderOptions())
	// Honor versioning.git_push. WithSkipPush existed and was called from
	// nowhere, so skipPush stayed false and executeTagStep pushed the tag on
	// every publish — including when the config said not to. `relicta publish`
	// printed "Push: false", reported "push_tag": false in --json, and pushed
	// anyway, which is the one action that cannot be undone and the specific
	// thing this setting is for.
	//
	// The CLI's --skip-push folds into the same field before the container is
	// built, so there is one answer to "should this push" rather than a flag and
	// a setting that can disagree.
	// The attestation configuration, which nothing passed.
	//
	// `attestation:` is a documented config section and executeAttestationStep reads
	// a.attestationConfig, so with the option never called the field was always nil and the
	// step returned "Attestation generation skipped (not enabled)" — reporting Success. A
	// user who enabled attestation got a release that said it succeeded and produced no
	// attestation, and `relicta verify`, whose whole purpose is checking one, had nothing to
	// check. Verified before the fix: a full publish with attestation.enabled: true wrote no
	// attestation and said nothing about it.
	//
	// The audit chain travels with it as a store and a repository rather than as a
	// chain. It used to be audit.NewChain() — a fresh, empty, process-local chain that
	// nothing appended to — so every attestation shipped auditChainHash "" and
	// auditEntryCount 0: a supply-chain attestation asserting a governance audit chain
	// and certifying an empty one. Both are nil/empty when governance memory is off,
	// and the attestation then reports an empty chain because there is one.
	publisher := NewPublisherAdapter(c.pluginExecutor, c.gitAdapter, c.tagCreator,
		WithPushTags(c.config.Versioning.GitPush),
		WithAttestationConfig(&c.config.Attestation),
		WithAuditChain(c.auditChainStore()))
	versionWriter := NewVersionWriterAdapter(c.gitAdapter, repoRoot)

	// Configure release services
	cfg := domainrelease.Config{
		RepoRoot:       repoRoot,
		GitAdapter:     c.gitAdapter,
		NotesGenerator: notesGenerator,
		Publisher:      publisher,
		VersionWriter:  versionWriter,
		// The composed chain built in initEventPublishing: outcome tracker, webhook
		// delivery, then the in-memory publisher the dashboard subscribes to.
		EventPublisher: c.eventPublisher,
		// The store persistence.backend selected, resolved once in initInfrastructure.
		// Without it the factory builds its own file adapter, which is how `plan` could
		// write JSON in a container whose bridge was reading a database.
		Repository: c.releaseRunStore,
		// So approval plans the attestation step the publisher knows how to run.
		AttestationEnabled: c.config.Attestation.Enabled,
	}

	var err error
	c.releaseServices, err = domainrelease.NewServices(cfg)
	if err != nil {
		return errors.StateWrap(err, "initReleaseServices", "failed to create release services")
	}

	c.logger.Info("release services initialized", "repo_root", repoRoot)
	return nil
}

// InitReleaseServices initializes release workflow services with a specific repository root.
// This should be called after Initialize() when the repository root is known.
func (c *App) InitReleaseServices(ctx context.Context, repoRoot string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return errors.State("InitReleaseServices", "container is closed")
	}

	return c.initReleaseServices(ctx, repoRoot)
}

// Application layer accessors

// ReleaseAnalyzer returns the release analyzer for commit analysis and version calculation.
func (c *App) ReleaseAnalyzer() *servicerelease.Analyzer {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.releaseAnalyzer
}

// CalculateVersion returns the CalculateVersionUseCase.
func (c *App) CalculateVersion() *versioning.CalculateVersionUseCase {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.calculateVersionUC
}

// TagCreator returns the TagCreatorAdapter for creating git tags.
func (c *App) TagCreator() *TagCreatorAdapter {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tagCreator
}

// GovernanceService returns the CGP governance service.
// Returns nil if governance is not enabled.
func (c *App) GovernanceService() *governance.Service {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.governanceService
}

// HasGovernance returns true if governance is enabled and initialized.
func (c *App) HasGovernance() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.governanceService != nil
}

// Analytics returns the governance analytics service, or nil when analytics
// could not be initialized (e.g. its store directory is unwritable).
func (c *App) Analytics() *analytics.Service {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.analyticsService
}

// MemoryStore returns the CGP memory store for release history.
// Returns nil if memory is not enabled.
func (c *App) MemoryStore() cgpmemory.Store {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.memoryStore
}

// HasMemory returns true if CGP memory is enabled and initialized.
func (c *App) HasMemory() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.memoryStore != nil
}

// MnemosStore returns the Mnemos-backed memory store (optional).
// Returns nil if Mnemos is not enabled or failed to initialize.
func (c *App) MnemosStore() cgpmemory.Store {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.mnemosStore
}

// HasMnemos returns true if Mnemos memory backend is enabled and initialized.
func (c *App) HasMnemos() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.mnemosStore != nil
}

// ChronosClient returns the Chronos pattern detection client (optional).
// Returns nil if Chronos is not enabled or failed to initialize.
func (c *App) ChronosClient() *chronosinfra.ChronosAdapter {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.chronosClient
}

// HasChronos returns true if Chronos pattern detection is enabled and initialized.
func (c *App) HasChronos() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.chronosClient != nil
}

// ReleaseServices returns the release workflow services.
// Returns nil if release services have not been initialized.
// Call InitReleaseServices() first to initialize these services.
func (c *App) ReleaseServices() *domainrelease.Services {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.releaseServices
}

// HasReleaseServices returns true if release services are initialized.
func (c *App) HasReleaseServices() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.releaseServices != nil
}

// Infrastructure layer accessors

// GitAdapter returns the git adapter implementing sourcecontrol.GitRepository.
func (c *App) GitAdapter() sourcecontrol.GitRepository {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.gitAdapter
}

// ReleaseRepository returns the release repository implementing release.Repository.
func (c *App) ReleaseRepository() domainrelease.Repository {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.releaseRepo
}

// EventPublisher returns the event publisher implementing release.EventPublisher.
func (c *App) EventPublisher() domainrelease.EventPublisher {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.eventPublisher
}

// SubscribeToEvents subscribes a handler function to receive domain events.
// The handler will be called for each event published through the base event publisher.
func (c *App) SubscribeToEvents(handler func(domainrelease.DomainEvent)) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.baseEventPublisher != nil {
		c.baseEventPublisher.Subscribe(handler)
	}
}

// UnitOfWork returns a new UnitOfWork for transactional operations.
// It returns an error if the UnitOfWork could not be initialized.
func (c *App) UnitOfWork() (domainrelease.UnitOfWork, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.unitOfWorkFactory.Begin(context.Background())
}

// PluginRegistry returns the plugin registry.
func (c *App) PluginRegistry() integration.PluginRegistry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pluginRegistry
}

// Service layer accessors (existing services)

// Git returns the legacy git service.
func (c *App) Git() git.Service {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.gitService
}

// AI returns the AI service.
func (c *App) AI() ai.Service {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.aiService
}

// HasAI returns true if the AI service is available.
func (c *App) HasAI() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.aiService != nil && c.aiService.IsAvailable()
}

// BlastService returns the blast radius analysis service.
func (c *App) BlastService() blast.Service {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.blastService
}

// HasBlastService returns true if the blast radius service is available.
func (c *App) HasBlastService() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.blastService != nil
}

// Config returns the configuration.
func (c *App) Config() *config.Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// Close gracefully shuts down the container and all its components.
func (c *App) Close() error {
	return c.CloseWithTimeout(defaultShutdownTimeout)
}

// CloseWithTimeout gracefully shuts down the container with a custom timeout.
func (c *App) CloseWithTimeout(timeout time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	c.logger.Debug("initiating container shutdown", "timeout", timeout)

	// Create a context with timeout for shutdown operations
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Close all registered closeables in reverse order (LIFO)
	var errs []error
	for i := len(c.closeables) - 1; i >= 0; i-- {
		closeable := c.closeables[i]
		if err := c.closeWithContext(ctx, closeable); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		c.logger.Warn("some components failed to close cleanly", "error_count", len(errs))
		// Return first error for simplicity
		return errs[0]
	}

	c.logger.Debug("container shutdown completed successfully")
	return nil
}

// closeWithContext closes a component with context cancellation support.
func (c *App) closeWithContext(ctx context.Context, closeable Closeable) error {
	done := make(chan error, 1)
	go func() {
		done <- closeable.Close()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		c.logger.Warn("component close timed out", "error", ctx.Err())
		return ctx.Err()
	}
}

// NewInitialized creates and initializes a new App container.
func NewInitialized(ctx context.Context, cfg *config.Config) (*App, error) {
	c, err := New(cfg)
	if err != nil {
		return nil, err
	}

	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}

	return c, nil
}

// changelogRenderOptions translates the changelog configuration into the domain's rendering
// options.
//
// The translation lives here, at the edge, so the renderer stays a domain value with no
// knowledge of Viper or of the config struct. It exists at all because the entire
// `changelog.*` block — format, group_by, exclude, categories, include_commit_hash,
// include_author, include_date, link_commits, link_issues, issue_url — had no reader outside
// the config package: the defaults described a Keep a Changelog renderer while releases wrote
// a flat list of commit subjects.
//
// changelog.template is the one setting still unread. It names a file this renderer has no way
// to apply: the template engine in internal/infrastructure/template renders a different data
// model (git.CategorizedChanges) than the entry built here, and choosing which shape a user's
// template sees is a public contract, not a wiring gap.
func (c *App) changelogRenderOptions() communication.RenderOptions {
	opts := communication.DefaultRenderOptions()
	if c.config == nil {
		return opts
	}

	cfg := c.config.Changelog

	if format := communication.ChangelogFormat(cfg.Format); format.IsValid() {
		opts.Format = format
	}
	// group_by decided nothing: the renderer always grouped by type, so "type" was
	// accidentally right and "scope" and "none" were silently ignored by a validator that
	// accepts all three.
	if grouping := communication.ChangelogGrouping(cfg.GroupBy); grouping.IsValid() {
		opts.GroupBy = grouping
	}
	// An empty Exclude is a real choice — "include everything" — so it is only the absence
	// of the key that falls back to the default, which config loading has already applied.
	opts.Exclude = cfg.Exclude
	if len(cfg.Categories) > 0 {
		opts.Categories = cfg.Categories
	}
	opts.IncludeCommitHash = cfg.IncludeCommitHash
	opts.IncludeAuthor = cfg.IncludeAuthor
	opts.IncludeDate = cfg.IncludeDate
	opts.LinkCommits = cfg.LinkCommits
	opts.RepositoryURL = strings.TrimSuffix(cfg.RepositoryURL, "/")
	opts.LinkIssues = cfg.LinkIssues
	// Not stripped of a trailing slash the way RepositoryURL is: this one may be a pattern
	// whose placeholder sits at the end, and the renderer trims the separator itself in the
	// base-URL case.
	opts.IssueURL = strings.TrimSpace(cfg.IssueURL)

	return opts
}
