// Package integration provides domain types for plugin integration.
package integration

import (
	"context"
	"time"

	"github.com/felixgeelhaar/release-pilot/internal/domain/changes"
	"github.com/felixgeelhaar/release-pilot/internal/domain/version"
)

// PluginID uniquely identifies a plugin.
type PluginID string

// PluginInfo represents plugin metadata.
type PluginInfo struct {
	ID           PluginID
	Name         string
	Version      string
	Description  string
	Author       string
	Hooks        []Hook
	ConfigSchema string
}

// PluginConfig represents plugin configuration.
type PluginConfig map[string]any

// ReleaseContext provides context to plugins during execution.
type ReleaseContext struct {
	// Version info
	Version         version.SemanticVersion
	PreviousVersion version.SemanticVersion
	ReleaseType     changes.ReleaseType

	// Repository info
	RepositoryOwner string
	RepositoryName  string
	RepositoryPath  string
	Branch          string
	TagName         string

	// Changes info
	Changes      *changes.ChangeSet
	Changelog    string
	ReleaseNotes string

	// Metadata
	DryRun    bool
	Timestamp time.Time
}

// ExecuteRequest represents a plugin execution request.
type ExecuteRequest struct {
	Hook    Hook
	Context ReleaseContext
	Config  PluginConfig
	DryRun  bool
}

// ExecuteResponse represents a plugin execution response.
type ExecuteResponse struct {
	Success   bool
	Message   string
	Error     string
	Outputs   map[string]any
	Artifacts []Artifact
}

// Artifact represents an artifact produced by a plugin.
type Artifact struct {
	Name string
	Path string
	Type string
	Size int64
	URL  string
}

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
	Code    string
}

// ValidateResponse represents a configuration validation response.
type ValidateResponse struct {
	Valid  bool
	Errors []ValidationError
}

// Plugin defines the interface that all plugins must implement.
// This is a port in hexagonal architecture terms.
type Plugin interface {
	// GetInfo returns plugin metadata.
	GetInfo() PluginInfo

	// Execute runs the plugin for a given hook.
	Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error)

	// Validate validates the plugin configuration.
	Validate(config PluginConfig) (*ValidateResponse, error)
}

// PluginRegistry manages plugin registration and lookup.
type PluginRegistry interface {
	// Register registers a plugin.
	Register(plugin Plugin) error

	// Unregister removes a plugin.
	Unregister(id PluginID) error

	// Get retrieves a plugin by ID.
	Get(id PluginID) (Plugin, error)

	// GetByHook retrieves all plugins that handle a specific hook.
	GetByHook(hook Hook) []Plugin

	// List returns all registered plugins.
	List() []Plugin

	// Has returns true if a plugin is registered.
	Has(id PluginID) bool
}

// PluginLoader loads plugins from various sources.
type PluginLoader interface {
	// Load loads a plugin from the specified path.
	Load(ctx context.Context, path string) (Plugin, error)

	// LoadAll loads all plugins from a directory.
	LoadAll(ctx context.Context, dir string) ([]Plugin, error)

	// Unload unloads a plugin.
	Unload(id PluginID) error
}

// PluginExecutor executes plugins at hook points.
type PluginExecutor interface {
	// ExecuteHook executes all plugins for a given hook.
	ExecuteHook(ctx context.Context, hook Hook, releaseCtx ReleaseContext) ([]ExecuteResponse, error)

	// ExecutePlugin executes a specific plugin.
	ExecutePlugin(ctx context.Context, id PluginID, req ExecuteRequest) (*ExecuteResponse, error)
}

// PluginState represents the state of a plugin.
type PluginState string

const (
	// PluginStateLoading indicates the plugin is loading.
	PluginStateLoading PluginState = "loading"
	// PluginStateReady indicates the plugin is ready.
	PluginStateReady PluginState = "ready"
	// PluginStateError indicates the plugin has an error.
	PluginStateError PluginState = "error"
	// PluginStateDisabled indicates the plugin is disabled.
	PluginStateDisabled PluginState = "disabled"
)

// PluginInstance represents a running plugin instance.
type PluginInstance struct {
	Plugin   Plugin
	Info     PluginInfo
	Config   PluginConfig
	State    PluginState
	Error    error
	LoadedAt time.Time
}

// IsHealthy returns true if the plugin is healthy and ready.
func (pi *PluginInstance) IsHealthy() bool {
	return pi.State == PluginStateReady && pi.Error == nil
}

// SupportsHook returns true if the plugin supports the given hook.
func (pi *PluginInstance) SupportsHook(hook Hook) bool {
	for _, h := range pi.Info.Hooks {
		if h == hook {
			return true
		}
	}
	return false
}
