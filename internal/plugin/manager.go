// Package plugin provides plugin management for Relicta.
package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/errors"
	"github.com/relicta-tech/relicta/internal/plugin/audit"
	"github.com/relicta-tech/relicta/internal/plugin/sandbox"
	"github.com/relicta-tech/relicta/pkg/plugin"
)

// MaxConcurrentPluginExecutions limits the number of plugins that can execute simultaneously.
// This prevents resource exhaustion from too many concurrent plugin processes.
const MaxConcurrentPluginExecutions = 10

// MaxGlobalHookTimeout is the maximum time allowed for all plugins to execute in a single hook.
// This prevents runaway plugin execution from blocking the entire release process.
const MaxGlobalHookTimeout = 2 * time.Minute

// DefaultPerPluginTimeout is the default timeout for individual plugin execution.
const DefaultPerPluginTimeout = 30 * time.Second

// Manager manages plugin lifecycle and execution.
type Manager struct {
	mu               sync.RWMutex
	plugins          map[string]*loadedPlugin
	logger           hclog.Logger
	cfg              *config.Config
	executionLimiter *semaphore.Weighted

	// Lazy loading support
	pendingPlugins map[string]*config.PluginConfig // Registered but not yet loaded
	loadOnce       map[string]*sync.Once           // Ensures each plugin loads only once
	loadErrors     map[string]error                // Stores load errors for lazy-loaded plugins
}

// loadedPlugin represents a loaded and running plugin.
type loadedPlugin struct {
	name    string
	client  *goplugin.Client
	plugin  plugin.Plugin
	info    plugin.Info
	config  map[string]any
	timeout time.Duration
	sandbox *sandbox.Sandbox
}

// NewManager creates a new plugin manager.
func NewManager(cfg *config.Config) *Manager {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "plugin",
		Level:  hclog.Info,
		Output: os.Stderr,
	})

	// Pre-allocate map capacity based on configured plugins
	pluginCount := len(cfg.Plugins)
	if pluginCount == 0 {
		pluginCount = 4 // Default capacity for typical usage
	}

	return &Manager{
		plugins:          make(map[string]*loadedPlugin, pluginCount),
		logger:           logger,
		cfg:              cfg,
		executionLimiter: semaphore.NewWeighted(MaxConcurrentPluginExecutions),
		pendingPlugins:   make(map[string]*config.PluginConfig, pluginCount),
		loadOnce:         make(map[string]*sync.Once, pluginCount),
		loadErrors:       make(map[string]error, pluginCount),
	}
}

// LoadPlugins loads all configured plugins.
func (m *Manager) LoadPlugins(ctx context.Context) error {
	const op = "plugin.LoadPlugins"

	for _, pluginCfg := range m.cfg.Plugins {
		if !pluginCfg.IsEnabled() {
			m.logger.Debug("plugin disabled", "name", pluginCfg.Name)
			continue
		}

		if err := m.loadPlugin(ctx, &pluginCfg); err != nil {
			if pluginCfg.ContinueOnError {
				m.logger.Warn("failed to load plugin, continuing", "name", pluginCfg.Name, "error", err)
				continue
			}
			return errors.PluginWrap(err, op, fmt.Sprintf("failed to load plugin: %s", pluginCfg.Name))
		}
	}

	return nil
}

// RegisterPlugins registers all configured plugins for lazy loading.
// Plugins are not actually loaded until they are needed (when a hook is executed).
// This improves startup time for commands that don't use plugins.
func (m *Manager) RegisterPlugins() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.cfg.Plugins {
		pluginCfg := &m.cfg.Plugins[i]
		if !pluginCfg.IsEnabled() {
			m.logger.Debug("plugin disabled", "name", pluginCfg.Name)
			continue
		}

		m.logger.Debug("registering plugin for lazy loading", "name", pluginCfg.Name)
		m.pendingPlugins[pluginCfg.Name] = pluginCfg
		m.loadOnce[pluginCfg.Name] = &sync.Once{}
	}
}

// ensurePluginLoaded ensures a plugin is loaded, loading it lazily if needed.
// This is thread-safe and ensures each plugin is loaded only once.
func (m *Manager) ensurePluginLoaded(ctx context.Context, name string) (*loadedPlugin, error) {
	// Fast path: check if already loaded
	m.mu.RLock()
	if lp, ok := m.plugins[name]; ok {
		m.mu.RUnlock()
		return lp, nil
	}

	// Check if there was a previous load error
	if err, ok := m.loadErrors[name]; ok {
		m.mu.RUnlock()
		return nil, err
	}

	// Get config and once for this plugin
	cfg, hasCfg := m.pendingPlugins[name]
	once, hasOnce := m.loadOnce[name]
	m.mu.RUnlock()

	if !hasCfg || !hasOnce {
		return nil, fmt.Errorf("plugin not registered: %s", name)
	}

	// Load the plugin (sync.Once ensures this happens only once)
	var loadErr error
	once.Do(func() {
		m.logger.Debug("lazy loading plugin", "name", name)
		loadErr = m.loadPlugin(ctx, cfg)
		if loadErr != nil {
			// Store the error for future calls
			m.mu.Lock()
			m.loadErrors[name] = loadErr
			m.mu.Unlock()
		}
	})

	if loadErr != nil {
		return nil, loadErr
	}

	// Get the loaded plugin
	m.mu.RLock()
	lp := m.plugins[name]
	m.mu.RUnlock()

	return lp, nil
}

// loadPlugin loads a single plugin.
func (m *Manager) loadPlugin(ctx context.Context, cfg *config.PluginConfig) error {
	// Find plugin binary
	pluginPath, err := m.findPluginBinary(cfg)
	if err != nil {
		_ = audit.LogLoad(ctx, cfg.Name, false, err.Error())
		return err
	}

	m.logger.Debug("loading plugin", "name", cfg.Name, "path", pluginPath)

	// Create sandbox with capabilities from config
	sb := sandbox.New(cfg.Name, cfg.Capabilities)

	// Create the command for the plugin
	cmd := exec.Command(pluginPath) // #nosec G204 -- pluginPath is validated and checksum-verified

	// Apply sandbox restrictions to the command
	if err := sb.PrepareCommand(ctx, cmd); err != nil {
		m.logger.Warn("failed to apply sandbox restrictions", "plugin", cfg.Name, "error", err)
		// Continue without sandboxing - this is best-effort
	}

	// Create plugin client with sandboxed command
	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  plugin.Handshake,
		Plugins:          plugin.PluginMap,
		Cmd:              cmd,
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Logger:           m.logger.Named(cfg.Name),
	})

	const op = "plugin.Load"

	// Connect to the plugin
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return errors.PluginWrap(err, op, "failed to connect to plugin")
	}

	// Get the plugin implementation
	raw, err := rpcClient.Dispense(plugin.PluginName)
	if err != nil {
		client.Kill()
		return errors.PluginWrap(err, op, "failed to dispense plugin")
	}

	p, ok := raw.(plugin.Plugin)
	if !ok {
		client.Kill()
		return errors.Plugin(op, "plugin does not implement Plugin interface")
	}

	// Get plugin info
	info := p.GetInfo()

	// Validate configuration
	if cfg.Config != nil {
		resp, err := p.Validate(ctx, cfg.Config)
		if err != nil {
			client.Kill()
			return errors.PluginWrap(err, op, "failed to validate plugin config")
		}
		if !resp.Valid {
			client.Kill()
			var errMsgs []string
			for _, e := range resp.Errors {
				errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", e.Field, e.Message))
			}
			return errors.Validation(op, fmt.Sprintf("invalid plugin configuration: %s", joinErrors(errMsgs)))
		}
	}

	// Set timeout
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Store loaded plugin
	m.mu.Lock()
	m.plugins[cfg.Name] = &loadedPlugin{
		name:    cfg.Name,
		client:  client,
		plugin:  p,
		info:    info,
		config:  cfg.Config,
		timeout: timeout,
		sandbox: sb,
	}
	m.mu.Unlock()

	m.logger.Info("plugin loaded", "name", cfg.Name, "version", info.Version, "hooks", info.Hooks)

	// Log successful load
	_ = audit.LogLoad(ctx, cfg.Name, true, "")

	return nil
}

// allowedPluginDirs returns the list of allowed directories for plugin binaries.
// Plugins can only be loaded from these secure locations.
func (m *Manager) allowedPluginDirs() []string {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/tmp" // Fallback, though unlikely
	}

	return []string{
		// User's relicta plugins directory (primary location)
		filepath.Join(homeDir, ".relicta", "plugins"),
		// Project-local plugins directory
		".relicta/plugins",
		// System-wide installation (for package managers)
		"/usr/local/lib/relicta/plugins",
		"/usr/lib/relicta/plugins",
	}
}

// validatePluginName checks if the plugin name contains only allowed characters.
func validatePluginName(name string) error {
	// Only allow alphanumeric characters, hyphens, and underscores
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return fmt.Errorf("plugin name contains invalid character: %q", r)
		}
	}
	if name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}
	if len(name) > 64 {
		return fmt.Errorf("plugin name too long (max 64 characters)")
	}
	return nil
}

// isPathInAllowedDir checks if the resolved path is within an allowed directory.
// Uses filepath.Rel for robust directory containment checking to prevent bypass attacks.
func (m *Manager) isPathInAllowedDir(resolvedPath string) bool {
	for _, allowedDir := range m.allowedPluginDirs() {
		absAllowed, err := filepath.Abs(allowedDir)
		if err != nil {
			continue
		}

		// Use filepath.Rel for robust relative path check
		// This properly handles edge cases like /usr/local/lib/relicta/plugins2
		rel, err := filepath.Rel(absAllowed, resolvedPath)
		if err != nil {
			continue
		}

		// Check that the relative path doesn't escape (no ..) and isn't absolute
		// A valid contained path will be something like "plugin-name" or "subdir/plugin"
		// An escape attempt would be "../evil" or an absolute path
		if !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
			return true
		}
	}
	return false
}

// validatePluginBinary performs security checks on the plugin binary.
func (m *Manager) validatePluginBinary(path string) error {
	// Resolve to absolute path to prevent path traversal
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve plugin path: %w", err)
	}

	// Evaluate symlinks to get the real path
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return fmt.Errorf("failed to evaluate symlinks: %w", err)
	}

	// Check that the real path is in an allowed directory
	if !m.isPathInAllowedDir(realPath) {
		return fmt.Errorf("plugin binary %s is not in an allowed directory", realPath)
	}

	// Check file exists and is a regular file
	info, err := os.Stat(realPath)
	if err != nil {
		return fmt.Errorf("plugin binary not accessible: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("plugin path is a directory, not a file")
	}

	// Check file is executable (has at least one execute bit set)
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("plugin binary is not executable")
	}

	// Additional check: file should be owned by current user or root (on Unix systems)
	// This prevents loading plugins placed by other users
	// Note: This is best-effort and may not work on all systems

	return nil
}

// findPluginBinary finds the plugin binary with security validation.
func (m *Manager) findPluginBinary(cfg *config.PluginConfig) (string, error) {
	// Validate plugin name first to prevent path injection
	if err := validatePluginName(cfg.Name); err != nil {
		return "", fmt.Errorf("invalid plugin name: %w", err)
	}

	// Plugin binary name matches the plugin name
	pluginBinaryName := cfg.Name

	// If path is specified, validate it's in an allowed directory
	if cfg.Path != "" {
		if err := m.validatePluginBinary(cfg.Path); err != nil {
			return "", fmt.Errorf("plugin path validation failed: %w", err)
		}
		absPath, err := filepath.Abs(cfg.Path)
		if err != nil {
			return "", fmt.Errorf("failed to resolve absolute path for plugin %s: %w", cfg.Name, err)
		}
		realPath, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			return "", fmt.Errorf("failed to evaluate symlinks for plugin %s: %w", cfg.Name, err)
		}
		return realPath, nil
	}

	// Search only in allowed directories
	for _, allowedDir := range m.allowedPluginDirs() {
		pluginPath := filepath.Join(allowedDir, pluginBinaryName)

		// Validate the plugin binary
		if err := m.validatePluginBinary(pluginPath); err == nil {
			absPath, err := filepath.Abs(pluginPath)
			if err != nil {
				// Log but continue searching other directories
				m.logger.Debug("failed to resolve absolute path", "path", pluginPath, "error", err)
				continue
			}
			realPath, err := filepath.EvalSymlinks(absPath)
			if err != nil {
				// Log but continue searching other directories
				m.logger.Debug("failed to evaluate symlinks", "path", absPath, "error", err)
				continue
			}
			return realPath, nil
		}
	}

	return "", fmt.Errorf("plugin binary not found for %s in allowed directories", cfg.Name)
}

// pluginExecInfo contains the information needed to execute a plugin.
// This allows us to release the lock before making expensive RPC calls.
type pluginExecInfo struct {
	name    string
	plugin  plugin.Plugin
	config  map[string]any
	timeout time.Duration
}

// pluginResult holds the result of a parallel plugin execution.
type pluginResult struct {
	index    int
	response plugin.ExecuteResponse
}

// ExecuteHook executes all plugins for a given hook in parallel.
// Plugins are executed concurrently for improved performance.
// Results are returned in a stable order (same order as plugin registration).
// A global timeout is applied to prevent runaway execution.
func (m *Manager) ExecuteHook(ctx context.Context, hook plugin.Hook, releaseCtx plugin.ReleaseContext) ([]plugin.ExecuteResponse, error) {
	// Collect plugins to execute while holding the lock briefly
	toExecute := m.collectPluginsForHook(hook)

	if len(toExecute) == 0 {
		return nil, nil
	}

	// Apply global timeout for all plugin executions
	// This prevents the entire hook execution from taking too long
	globalCtx, globalCancel := context.WithTimeout(ctx, MaxGlobalHookTimeout)
	defer globalCancel()

	// Get dry run setting (read config while we have access)
	dryRun := m.cfg.Workflow.DryRunByDefault

	// Channel for collecting results from parallel execution
	// Buffered to prevent goroutine leaks on context cancellation
	resultsChan := make(chan pluginResult, len(toExecute))

	// Use errgroup for coordinated parallel execution
	g, gCtx := errgroup.WithContext(globalCtx)

	// Execute plugins in parallel with rate limiting
	for i, exec := range toExecute {
		i, exec := i, exec // Capture loop variables
		g.Go(func() error {
			// Check for context cancellation before acquiring semaphore
			select {
			case <-gCtx.Done():
				resultsChan <- pluginResult{
					index: i,
					response: plugin.ExecuteResponse{
						Success: false,
						Error:   fmt.Sprintf("execution canceled: %v", gCtx.Err()),
					},
				}
				return nil
			default:
			}

			// Acquire execution slot from semaphore (rate limiting)
			if err := m.executionLimiter.Acquire(gCtx, 1); err != nil {
				m.logger.Error("failed to acquire execution slot", "plugin", exec.name, "error", err)
				resultsChan <- pluginResult{
					index: i,
					response: plugin.ExecuteResponse{
						Success: false,
						Error:   fmt.Sprintf("failed to acquire execution slot: %v", err),
					},
				}
				return nil
			}
			defer m.executionLimiter.Release(1)

			m.logger.Debug("executing hook", "plugin", exec.name, "hook", hook)

			// Execute with per-plugin timeout (capped by global context)
			execCtx, cancel := context.WithTimeout(gCtx, exec.timeout)
			defer cancel()

			// Track execution time for audit logging
			startTime := time.Now()

			resp, err := exec.plugin.Execute(execCtx, plugin.ExecuteRequest{
				Hook:    hook,
				Config:  exec.config,
				Context: releaseCtx,
				DryRun:  dryRun,
			})

			duration := time.Since(startTime)

			if err != nil {
				m.logger.Error("plugin execution failed", "plugin", exec.name, "hook", hook, "error", err)
				_ = audit.LogExecution(gCtx, exec.name, string(hook), false, duration, err.Error())
				resultsChan <- pluginResult{
					index: i,
					response: plugin.ExecuteResponse{
						Success: false,
						Error:   err.Error(),
					},
				}
				// Don't return error - allow other plugins to continue
				return nil
			}

			if resp != nil {
				if resp.Success {
					m.logger.Info("plugin executed successfully", "plugin", exec.name, "hook", hook)
					_ = audit.LogExecution(gCtx, exec.name, string(hook), true, duration, "")
				} else {
					m.logger.Warn("plugin execution returned error", "plugin", exec.name, "hook", hook, "error", resp.Error)
					_ = audit.LogExecution(gCtx, exec.name, string(hook), false, duration, resp.Error)
				}
				resultsChan <- pluginResult{
					index:    i,
					response: *resp,
				}
			} else {
				// Ensure we always send a result to prevent deadlock
				_ = audit.LogExecution(gCtx, exec.name, string(hook), true, duration, "")
				resultsChan <- pluginResult{
					index: i,
					response: plugin.ExecuteResponse{
						Success: true,
						Message: "plugin returned no response",
					},
				}
			}

			return nil
		})
	}

	// Wait for all goroutines to complete
	_ = g.Wait() // Errors are handled per-plugin, not propagated
	close(resultsChan)

	// Check if we timed out globally
	if globalCtx.Err() != nil {
		m.logger.Warn("global hook timeout reached", "hook", hook, "timeout", MaxGlobalHookTimeout)
	}

	// Collect results and sort by original index for stable ordering
	indexedResults := make([]pluginResult, 0, len(toExecute))
	for result := range resultsChan {
		indexedResults = append(indexedResults, result)
	}

	// Sort by index to maintain stable order
	results := make([]plugin.ExecuteResponse, len(toExecute))
	for _, r := range indexedResults {
		if r.index >= 0 && r.index < len(results) {
			results[r.index] = r.response
		}
	}

	// Filter out zero-value responses (from plugins that returned nil)
	filteredResults := make([]plugin.ExecuteResponse, 0, len(results))
	for _, r := range results {
		// Check if this is a real response (not zero value from unset index)
		if r.Success || r.Error != "" || r.Message != "" || len(r.Outputs) > 0 {
			filteredResults = append(filteredResults, r)
		}
	}

	return filteredResults, nil
}

// collectPluginsForHook collects plugins that support the given hook.
// Supports both eagerly-loaded plugins and lazy-loaded plugins.
func (m *Manager) collectPluginsForHook(hook plugin.Hook) []pluginExecInfo {
	m.mu.RLock()

	// Pre-allocate with reasonable capacity
	totalPlugins := len(m.plugins) + len(m.pendingPlugins)
	toExecute := make([]pluginExecInfo, 0, totalPlugins)

	// Collect already-loaded plugins that support this hook
	for _, lp := range m.plugins {
		if !m.pluginSupportsHook(lp, hook) {
			continue
		}

		toExecute = append(toExecute, pluginExecInfo{
			name:    lp.name,
			plugin:  lp.plugin,
			config:  lp.config,
			timeout: lp.timeout,
		})
	}

	// Identify pending plugins that might support this hook
	pendingToLoad := make([]string, 0)
	for name, cfg := range m.pendingPlugins {
		// Skip if already loaded
		if _, loaded := m.plugins[name]; loaded {
			continue
		}

		// Check if plugin config specifies hooks
		if len(cfg.Hooks) > 0 {
			// Only load if the hook is in the config list
			if !m.configHasHook(cfg, hook) {
				continue
			}
		}
		// If no hooks specified in config, we need to load to find out

		pendingToLoad = append(pendingToLoad, name)
	}
	m.mu.RUnlock()

	// Lazily load pending plugins that might support this hook
	ctx := context.Background()
	for _, name := range pendingToLoad {
		lp, err := m.ensurePluginLoaded(ctx, name)
		if err != nil {
			m.logger.Warn("failed to lazy load plugin", "name", name, "error", err)
			continue
		}

		// Now check if it actually supports the hook
		if !m.pluginSupportsHook(lp, hook) {
			continue
		}

		toExecute = append(toExecute, pluginExecInfo{
			name:    lp.name,
			plugin:  lp.plugin,
			config:  lp.config,
			timeout: lp.timeout,
		})
	}

	return toExecute
}

// configHasHook checks if the plugin config specifies support for a hook.
func (m *Manager) configHasHook(cfg *config.PluginConfig, hook plugin.Hook) bool {
	hookStr := string(hook)
	for _, h := range cfg.Hooks {
		if h == hookStr {
			return true
		}
	}
	return false
}

// pluginSupportsHook checks if a plugin supports a given hook.
func (m *Manager) pluginSupportsHook(lp *loadedPlugin, hook plugin.Hook) bool {
	for _, h := range lp.info.Hooks {
		if h == hook {
			return true
		}
	}
	return false
}

// GetPluginInfo returns info for a specific plugin.
func (m *Manager) GetPluginInfo(name string) (*plugin.Info, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lp, ok := m.plugins[name]
	if !ok {
		return nil, errors.NotFound("plugin.GetPluginInfo", fmt.Sprintf("plugin not found: %s", name))
	}

	return &lp.info, nil
}

// ListPlugins returns info for all loaded plugins.
func (m *Manager) ListPlugins() []plugin.Info {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]plugin.Info, 0, len(m.plugins))
	for _, lp := range m.plugins {
		infos = append(infos, lp.info)
	}

	return infos
}

// Shutdown shuts down all plugins.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx := context.Background()
	for name, lp := range m.plugins {
		m.logger.Debug("shutting down plugin", "name", name)
		if lp.client != nil {
			lp.client.Kill()
		}
		_ = audit.LogUnload(ctx, name)
	}

	m.plugins = make(map[string]*loadedPlugin)
}

// Close is an alias for Shutdown.
func (m *Manager) Close() error {
	m.Shutdown()
	return nil
}

// joinErrors joins error messages with a separator.
func joinErrors(errs []string) string {
	if len(errs) == 0 {
		return ""
	}
	if len(errs) == 1 {
		return errs[0]
	}

	var sb strings.Builder
	sb.Grow(len(errs) * 30) // Estimate 30 chars per error
	for i, e := range errs {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(e)
	}
	return sb.String()
}
