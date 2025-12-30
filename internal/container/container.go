// Package container provides dependency injection for Relicta services.
package container

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	analysisfactory "github.com/relicta-tech/relicta/internal/analysis/factory"
	"github.com/relicta-tech/relicta/internal/application/blast"
	"github.com/relicta-tech/relicta/internal/application/governance"
	"github.com/relicta-tech/relicta/internal/application/versioning"
	"github.com/relicta-tech/relicta/internal/cgp/memory"
	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/integration"
	domainrelease "github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/domain/version"
	"github.com/relicta-tech/relicta/internal/errors"
	"github.com/relicta-tech/relicta/internal/infrastructure/ai"
	"github.com/relicta-tech/relicta/internal/infrastructure/git"
	"github.com/relicta-tech/relicta/internal/infrastructure/persistence"
	"github.com/relicta-tech/relicta/internal/infrastructure/webhook"
	"github.com/relicta-tech/relicta/internal/plugin"
	servicerelease "github.com/relicta-tech/relicta/internal/service/release"
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
	memoryStore        memory.Store

	// Services (existing infrastructure)
	gitService   git.Service
	aiService    ai.Service
	blastService blast.Service

	// Application layer use cases
	releaseAnalyzer    *servicerelease.Analyzer
	calculateVersionUC *versioning.CalculateVersionUseCase
	setVersionUC       *versioning.SetVersionUseCase

	// Governance service (CGP)
	governanceService *governance.Service

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
		c.memoryStore, err = memory.NewFileStore(memoryPath)
		if err != nil {
			c.logger.Warn("failed to initialize memory store", "error", err)
		} else {
			publisher = memory.NewOutcomeTracker(c.memoryStore, publisher)
			c.logger.Debug("outcome tracker initialized", "path", memoryPath)
		}
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
	//nolint:contextcheck // Constructor is pure configuration; context used in method calls
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

	// Initialize SetVersionUseCase
	c.setVersionUC = versioning.NewSetVersionUseCase(c.gitAdapter)

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
	}

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
	publisher := NewPublisherAdapter(c.pluginExecutor, c.gitAdapter)
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

// SetVersion returns the SetVersionUseCase.
func (c *App) SetVersion() *versioning.SetVersionUseCase {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.setVersionUC
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

// MemoryStore returns the CGP memory store for release history.
// Returns nil if memory is not enabled.
func (c *App) MemoryStore() memory.Store {
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
