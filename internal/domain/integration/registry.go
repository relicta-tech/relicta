// Package integration provides domain types for plugin integration.
package integration

import (
	"context"
	"sync"
)

// InMemoryPluginRegistry implements PluginRegistry with in-memory storage.
type InMemoryPluginRegistry struct {
	mu      sync.RWMutex
	plugins map[PluginID]Plugin
}

// NewInMemoryPluginRegistry creates a new in-memory plugin registry.
func NewInMemoryPluginRegistry() *InMemoryPluginRegistry {
	return &InMemoryPluginRegistry{
		plugins: make(map[PluginID]Plugin),
	}
}

// Register registers a plugin.
func (r *InMemoryPluginRegistry) Register(plugin Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info := plugin.GetInfo()
	if info.ID == "" {
		return ErrInvalidPlugin
	}

	if _, exists := r.plugins[info.ID]; exists {
		return ErrPluginAlreadyRegistered
	}

	r.plugins[info.ID] = plugin
	return nil
}

// Unregister removes a plugin.
func (r *InMemoryPluginRegistry) Unregister(id PluginID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[id]; !exists {
		return ErrPluginNotFound
	}

	delete(r.plugins, id)
	return nil
}

// Get retrieves a plugin by ID.
func (r *InMemoryPluginRegistry) Get(id PluginID) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, exists := r.plugins[id]
	if !exists {
		return nil, ErrPluginNotFound
	}

	return plugin, nil
}

// GetByHook retrieves all plugins that handle a specific hook.
func (r *InMemoryPluginRegistry) GetByHook(hook Hook) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Pre-allocate assuming ~25% of plugins handle each hook on average
	result := make([]Plugin, 0, len(r.plugins)/4+1)
	for _, plugin := range r.plugins {
		info := plugin.GetInfo()
		for _, h := range info.Hooks {
			if h == hook {
				result = append(result, plugin)
				break
			}
		}
	}
	return result
}

// List returns all registered plugins.
func (r *InMemoryPluginRegistry) List() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Plugin, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		result = append(result, plugin)
	}
	return result
}

// Has returns true if a plugin is registered.
func (r *InMemoryPluginRegistry) Has(id PluginID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.plugins[id]
	return exists
}

// SequentialPluginExecutor executes plugins sequentially.
type SequentialPluginExecutor struct {
	registry PluginRegistry
	configs  map[PluginID]PluginConfig
	mu       sync.RWMutex
}

// NewSequentialPluginExecutor creates a new sequential plugin executor.
func NewSequentialPluginExecutor(registry PluginRegistry) *SequentialPluginExecutor {
	return &SequentialPluginExecutor{
		registry: registry,
		configs:  make(map[PluginID]PluginConfig),
	}
}

// SetPluginConfig sets the configuration for a plugin.
func (e *SequentialPluginExecutor) SetPluginConfig(id PluginID, config PluginConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.configs[id] = config
}

// GetPluginConfig gets the configuration for a plugin.
func (e *SequentialPluginExecutor) GetPluginConfig(id PluginID) PluginConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.configs[id]
}

// ExecuteHook executes all plugins for a given hook.
func (e *SequentialPluginExecutor) ExecuteHook(ctx context.Context, hook Hook, releaseCtx ReleaseContext) ([]ExecuteResponse, error) {
	plugins := e.registry.GetByHook(hook)
	if len(plugins) == 0 {
		return nil, nil
	}

	var responses []ExecuteResponse
	for _, plugin := range plugins {
		info := plugin.GetInfo()
		config := e.GetPluginConfig(info.ID)

		req := ExecuteRequest{
			Hook:    hook,
			Context: releaseCtx,
			Config:  config,
			DryRun:  releaseCtx.DryRun,
		}

		resp, err := plugin.Execute(ctx, req)
		if err != nil {
			responses = append(responses, ExecuteResponse{
				Success: false,
				Error:   err.Error(),
			})
			continue
		}

		responses = append(responses, *resp)
	}

	return responses, nil
}

// ExecutePlugin executes a specific plugin.
func (e *SequentialPluginExecutor) ExecutePlugin(ctx context.Context, id PluginID, req ExecuteRequest) (*ExecuteResponse, error) {
	plugin, err := e.registry.Get(id)
	if err != nil {
		return nil, err
	}

	// Use stored config if not provided in request
	if req.Config == nil {
		req.Config = e.GetPluginConfig(id)
	}

	return plugin.Execute(ctx, req)
}

// ParallelPluginExecutor executes plugins in parallel.
type ParallelPluginExecutor struct {
	registry PluginRegistry
	configs  map[PluginID]PluginConfig
	mu       sync.RWMutex
}

// NewParallelPluginExecutor creates a new parallel plugin executor.
func NewParallelPluginExecutor(registry PluginRegistry) *ParallelPluginExecutor {
	return &ParallelPluginExecutor{
		registry: registry,
		configs:  make(map[PluginID]PluginConfig),
	}
}

// SetPluginConfig sets the configuration for a plugin.
func (e *ParallelPluginExecutor) SetPluginConfig(id PluginID, config PluginConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.configs[id] = config
}

// GetPluginConfig gets the configuration for a plugin.
func (e *ParallelPluginExecutor) GetPluginConfig(id PluginID) PluginConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.configs[id]
}

// ExecuteHook executes all plugins for a given hook in parallel.
func (e *ParallelPluginExecutor) ExecuteHook(ctx context.Context, hook Hook, releaseCtx ReleaseContext) ([]ExecuteResponse, error) {
	plugins := e.registry.GetByHook(hook)
	if len(plugins) == 0 {
		return nil, nil
	}

	var wg sync.WaitGroup
	responses := make([]ExecuteResponse, len(plugins))
	errs := make([]error, len(plugins))

	for i, plugin := range plugins {
		wg.Add(1)
		go func(idx int, p Plugin) {
			defer wg.Done()

			// Check for context cancellation before executing
			select {
			case <-ctx.Done():
				responses[idx] = ExecuteResponse{
					Success: false,
					Error:   ctx.Err().Error(),
				}
				errs[idx] = ctx.Err()
				return
			default:
			}

			info := p.GetInfo()
			config := e.GetPluginConfig(info.ID)

			req := ExecuteRequest{
				Hook:    hook,
				Context: releaseCtx,
				Config:  config,
				DryRun:  releaseCtx.DryRun,
			}

			resp, err := p.Execute(ctx, req)
			if err != nil {
				responses[idx] = ExecuteResponse{
					Success: false,
					Error:   err.Error(),
				}
				errs[idx] = err
				return
			}

			responses[idx] = *resp
		}(i, plugin)
	}

	wg.Wait()
	return responses, nil
}

// ExecutePlugin executes a specific plugin.
func (e *ParallelPluginExecutor) ExecutePlugin(ctx context.Context, id PluginID, req ExecuteRequest) (*ExecuteResponse, error) {
	plugin, err := e.registry.Get(id)
	if err != nil {
		return nil, err
	}

	if req.Config == nil {
		req.Config = e.GetPluginConfig(id)
	}

	return plugin.Execute(ctx, req)
}
