// Package container provides dependency injection for ReleasePilot services.
package container

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/felixgeelhaar/release-pilot/internal/application/release"
	"github.com/felixgeelhaar/release-pilot/internal/application/versioning"
	"github.com/felixgeelhaar/release-pilot/internal/config"
	"github.com/felixgeelhaar/release-pilot/internal/domain/integration"
	domainrelease "github.com/felixgeelhaar/release-pilot/internal/domain/release"
	"github.com/felixgeelhaar/release-pilot/internal/domain/sourcecontrol"
	"github.com/felixgeelhaar/release-pilot/internal/domain/version"
	"github.com/felixgeelhaar/release-pilot/internal/errors"
	gitadapter "github.com/felixgeelhaar/release-pilot/internal/infrastructure/git"
	"github.com/felixgeelhaar/release-pilot/internal/infrastructure/persistence"
	"github.com/felixgeelhaar/release-pilot/internal/plugin"
	"github.com/felixgeelhaar/release-pilot/internal/service/ai"
	"github.com/felixgeelhaar/release-pilot/internal/service/git"
)

// defaultShutdownTimeout is the default timeout for graceful shutdown of components.
const defaultShutdownTimeout = 10 * time.Second

// Closeable represents a component that can be closed/shutdown.
type Closeable interface {
	Close() error
}

// DDDContainer provides dependency injection for the DDD architecture.
type DDDContainer struct {
	config *config.Config
	logger *slog.Logger
	mu     sync.RWMutex
	closed bool

	// Infrastructure layer
	gitAdapter     *gitadapter.Adapter
	releaseRepo    *persistence.FileReleaseRepository
	eventPublisher *persistence.InMemoryEventPublisher
	versionCalc    version.VersionCalculator
	pluginRegistry integration.PluginRegistry
	pluginExecutor integration.PluginExecutor
	pluginManager  *plugin.Manager

	// Services (existing infrastructure)
	gitService git.Service
	aiService  ai.Service

	// Application layer use cases
	planReleaseUC      *release.PlanReleaseUseCase
	generateNotesUC    *release.GenerateNotesUseCase
	approveReleaseUC   *release.ApproveReleaseUseCase
	publishReleaseUC   *release.PublishReleaseUseCase
	calculateVersionUC *versioning.CalculateVersionUseCase
	setVersionUC       *versioning.SetVersionUseCase

	// Cleanup tracking
	closeables []Closeable
}

// NewDDDContainer creates a new DDD container with the given configuration.
func NewDDDContainer(cfg *config.Config) (*DDDContainer, error) {
	if cfg == nil {
		return nil, errors.Config("NewDDDContainer", "configuration is required")
	}

	return &DDDContainer{
		config:     cfg,
		logger:     slog.Default(),
		closeables: make([]Closeable, 0),
	}, nil
}

// registerCloseable registers a component for cleanup during shutdown.
func (c *DDDContainer) registerCloseable(closeable Closeable) {
	if closeable != nil {
		c.closeables = append(c.closeables, closeable)
	}
}

// RegisterCloseable allows external components to register for cleanup during shutdown.
// Components are closed in reverse order of registration (LIFO).
func (c *DDDContainer) RegisterCloseable(closeable Closeable) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.registerCloseable(closeable)
}

// Initialize initializes all layers of the DDD container.
func (c *DDDContainer) Initialize(ctx context.Context) error {
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
	if err := c.initApplicationLayer(); err != nil {
		return err
	}

	return nil
}

// initInfrastructure initializes infrastructure layer components.
func (c *DDDContainer) initInfrastructure(ctx context.Context) error {
	var err error

	// Initialize existing git service
	c.gitService, err = git.NewService()
	if err != nil {
		return errors.GitWrap(err, "initInfrastructure", "failed to initialize git service")
	}

	// Create git adapter that implements domain interface
	c.gitAdapter = gitadapter.NewAdapter(c.gitService)

	// Initialize release repository
	repoPath := ".release-pilot/releases"
	c.releaseRepo, err = persistence.NewFileReleaseRepository(repoPath)
	if err != nil {
		return errors.StateWrap(err, "initInfrastructure", "failed to initialize release repository")
	}

	// Initialize event publisher
	c.eventPublisher = persistence.NewInMemoryEventPublisher()

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
		c.aiService, err = c.initAIService()
		if err != nil {
			// AI service failure is non-fatal
			c.aiService = nil
		}
	}

	return nil
}

// initAIService initializes the AI service based on configuration.
func (c *DDDContainer) initAIService() (ai.Service, error) {
	apiKey := c.config.AI.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	if apiKey == "" {
		return nil, errors.AI("initAIService", "OpenAI API key not configured")
	}

	opts := []ai.ServiceOption{
		ai.WithAPIKey(apiKey),
		ai.WithProvider(c.config.AI.Provider),
		ai.WithModel(c.config.AI.Model),
	}

	if c.config.AI.BaseURL != "" {
		opts = append(opts, ai.WithBaseURL(c.config.AI.BaseURL))
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

	return ai.NewService(opts...)
}

// initPluginSystem initializes the plugin system.
// If plugins are configured, it uses the plugin.Manager with ExecutorAdapter.
// Otherwise, it uses an empty in-memory registry.
func (c *DDDContainer) initPluginSystem(ctx context.Context) error {
	// If no plugins configured, use empty in-memory implementation
	if len(c.config.Plugins) == 0 {
		c.pluginRegistry = integration.NewInMemoryPluginRegistry()
		c.pluginExecutor = integration.NewSequentialPluginExecutor(c.pluginRegistry)
		return nil
	}

	// Create plugin manager for external gRPC plugins
	c.pluginManager = plugin.NewManager(c.config)

	// Load configured plugins
	if err := c.pluginManager.LoadPlugins(ctx); err != nil {
		return err
	}

	// Register manager for cleanup
	c.registerCloseable(c.pluginManager)

	// Create adapter that bridges Manager to PluginExecutor interface
	c.pluginExecutor = plugin.NewExecutorAdapter(c.pluginManager)

	// Use empty in-memory registry (external plugins are managed by Manager)
	c.pluginRegistry = integration.NewInMemoryPluginRegistry()

	return nil
}

// initApplicationLayer initializes application layer use cases.
func (c *DDDContainer) initApplicationLayer() error {
	// Initialize PlanReleaseUseCase
	c.planReleaseUC = release.NewPlanReleaseUseCase(
		c.releaseRepo,
		c.gitAdapter,
		c.versionCalc,
		c.eventPublisher,
	)

	// Initialize GenerateNotesUseCase
	// Note: AINotesGenerator is nil for now, can be set later
	c.generateNotesUC = release.NewGenerateNotesUseCase(
		c.releaseRepo,
		nil, // AINotesGenerator - optional
		c.eventPublisher,
	)

	// Initialize ApproveReleaseUseCase
	c.approveReleaseUC = release.NewApproveReleaseUseCase(
		c.releaseRepo,
		c.eventPublisher,
	)

	// Initialize PublishReleaseUseCase
	c.publishReleaseUC = release.NewPublishReleaseUseCase(
		c.releaseRepo,
		c.gitAdapter,
		c.pluginExecutor,
		c.eventPublisher,
	)

	// Initialize CalculateVersionUseCase
	c.calculateVersionUC = versioning.NewCalculateVersionUseCase(
		c.gitAdapter,
		c.versionCalc,
	)

	// Initialize SetVersionUseCase
	c.setVersionUC = versioning.NewSetVersionUseCase(c.gitAdapter)

	return nil
}

// Application layer accessors

// PlanRelease returns the PlanReleaseUseCase.
func (c *DDDContainer) PlanRelease() *release.PlanReleaseUseCase {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.planReleaseUC
}

// GenerateNotes returns the GenerateNotesUseCase.
func (c *DDDContainer) GenerateNotes() *release.GenerateNotesUseCase {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.generateNotesUC
}

// ApproveRelease returns the ApproveReleaseUseCase.
func (c *DDDContainer) ApproveRelease() *release.ApproveReleaseUseCase {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.approveReleaseUC
}

// PublishRelease returns the PublishReleaseUseCase.
func (c *DDDContainer) PublishRelease() *release.PublishReleaseUseCase {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.publishReleaseUC
}

// CalculateVersion returns the CalculateVersionUseCase.
func (c *DDDContainer) CalculateVersion() *versioning.CalculateVersionUseCase {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.calculateVersionUC
}

// SetVersion returns the SetVersionUseCase.
func (c *DDDContainer) SetVersion() *versioning.SetVersionUseCase {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.setVersionUC
}

// Infrastructure layer accessors

// GitAdapter returns the git adapter implementing sourcecontrol.GitRepository.
func (c *DDDContainer) GitAdapter() sourcecontrol.GitRepository {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.gitAdapter
}

// ReleaseRepository returns the release repository implementing release.Repository.
func (c *DDDContainer) ReleaseRepository() domainrelease.Repository {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.releaseRepo
}

// EventPublisher returns the event publisher implementing release.EventPublisher.
func (c *DDDContainer) EventPublisher() domainrelease.EventPublisher {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.eventPublisher
}

// PluginRegistry returns the plugin registry.
func (c *DDDContainer) PluginRegistry() integration.PluginRegistry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pluginRegistry
}

// Service layer accessors (existing services)

// Git returns the legacy git service.
func (c *DDDContainer) Git() git.Service {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.gitService
}

// AI returns the AI service.
func (c *DDDContainer) AI() ai.Service {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.aiService
}

// HasAI returns true if the AI service is available.
func (c *DDDContainer) HasAI() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.aiService != nil && c.aiService.IsAvailable()
}

// Config returns the configuration.
func (c *DDDContainer) Config() *config.Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// Close gracefully shuts down the container and all its components.
func (c *DDDContainer) Close() error {
	return c.CloseWithTimeout(defaultShutdownTimeout)
}

// CloseWithTimeout gracefully shuts down the container with a custom timeout.
func (c *DDDContainer) CloseWithTimeout(timeout time.Duration) error {
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
func (c *DDDContainer) closeWithContext(ctx context.Context, closeable Closeable) error {
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

// NewInitializedDDDContainer creates and initializes a new DDD container.
func NewInitializedDDDContainer(ctx context.Context, cfg *config.Config) (*DDDContainer, error) {
	c, err := NewDDDContainer(cfg)
	if err != nil {
		return nil, err
	}

	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}

	return c, nil
}
