// Package container provides dependency injection for Relicta services.
package container

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	analysisfactory "github.com/relicta-tech/relicta/v4/internal/analysis/factory"
	"github.com/relicta-tech/relicta/v4/internal/analytics"
	"github.com/relicta-tech/relicta/v4/internal/application/blast"
	"github.com/relicta-tech/relicta/v4/internal/application/governance"
	"github.com/relicta-tech/relicta/v4/internal/application/versioning"
	cgpmemory "github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/integration"
	domainrelease "github.com/relicta-tech/relicta/v4/internal/domain/release"
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

	// allowUntrustedPlugins mirrors the CLI's --allow-untrusted-plugins flag.
	// Default false — plugin manager refuses to load plugins on best-effort
	// sandbox platforms until signing infrastructure ships. CLI sets this
	// before initPluginSystem runs.
	allowUntrustedPlugins bool

	// Infrastructure layer
	gitAdapter         *git.Adapter
	releaseRepo        *persistence.FileReleaseRepository
	baseEventPublisher *persistence.InMemoryEventPublisher
	eventPublisher     domainrelease.EventPublisher // Composed publisher chain
	unitOfWorkFactory  *persistence.FileUnitOfWorkFactory
	versionCalc        version.VersionCalculator
	pluginRegistry     integration.PluginRegistry
	pluginExecutor     integration.PluginExecutor
	pluginManager      *plugin.Manager
	memoryStore        cgpmemory.Store

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

	// Initialize existing git service
	c.gitService, err = git.NewService()
	if err != nil {
		return errors.GitWrap(err, "initInfrastructure", "failed to initialize git service")
	}

	// Create git adapter that implements domain interface
	c.gitAdapter = git.NewAdapter(c.gitService)

	// Initialize release repository
	repoPath := ".relicta/releases"
	c.releaseRepo, err = persistence.NewFileReleaseRepository(repoPath)
	if err != nil {
		return errors.StateWrap(err, "initInfrastructure", "failed to initialize release repository")
	}

	// Initialize event publisher chain:
	// OutcomeTracker → WebhookPublisher → InMemoryEventPublisher
	c.baseEventPublisher = persistence.NewInMemoryEventPublisher()

	// Start with base publisher
	var publisher domainrelease.EventPublisher = c.baseEventPublisher

	// Add webhook publisher if webhooks are configured
	if len(c.config.Webhooks) > 0 {
		publisher = webhook.NewPublisher(c.config.Webhooks, publisher)
		c.logger.Debug("webhook publisher initialized", "webhook_count", len(c.config.Webhooks))
	}

	// Add outcome tracker if governance memory is enabled
	if c.config.Governance.MemoryEnabled {
		memoryPath := ".relicta/memory"
		c.memoryStore, err = cgpmemory.NewFileStore(memoryPath)
		if err != nil {
			c.logger.Warn("failed to initialize memory store", "error", err)
		} else {
			publisher = cgpmemory.NewOutcomeTracker(c.memoryStore, publisher)
			c.logger.Debug("outcome tracker initialized", "path", memoryPath)
		}
	}

	// Initialize Mnemos memory backend (optional — replaces file store when enabled)
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
		// Make Mnemos the primary governance memory backend when enabled.
		c.memoryStore = c.mnemosStore
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

	// Initialize UnitOfWork factory for transactional operations
	c.unitOfWorkFactory = persistence.NewFileUnitOfWorkFactory(c.releaseRepo, c.baseEventPublisher)

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
	c.tagCreator = NewTagCreatorAdapter(c.gitAdapter)

	// Initialize blast radius service for monorepo analysis
	c.blastService = blast.NewService(
		blast.WithRepoPath("."),
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
	c.governanceService, err = governance.NewServiceFromConfig(
		ctx,
		&c.config.Governance,
		repoPath,
		c.logger,
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

// initReleaseServices initializes the release workflow services.
func (c *App) initReleaseServices(ctx context.Context, repoRoot string) error {
	// Check for early cancellation
	if err := ctx.Err(); err != nil {
		return err
	}

	// Create port adapters
	notesGenerator := NewNotesGeneratorAdapter(c.aiService, c.gitAdapter)
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
	publisher := NewPublisherAdapter(c.pluginExecutor, c.gitAdapter, c.tagCreator,
		WithPushTags(c.config.Versioning.GitPush))
	versionWriter := NewVersionWriterAdapter(c.gitAdapter, repoRoot)

	// Configure release services
	cfg := domainrelease.Config{
		RepoRoot:       repoRoot,
		GitAdapter:     c.gitAdapter,
		NotesGenerator: notesGenerator,
		Publisher:      publisher,
		VersionWriter:  versionWriter,
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
