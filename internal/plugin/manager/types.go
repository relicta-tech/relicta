// Package manager provides plugin management functionality for Relicta.
package manager

import (
	"time"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

// Registry represents the central registry of available plugins.
type Registry struct {
	// Version of the registry schema
	Version string `yaml:"version"`
	// Plugins available in the registry
	Plugins []PluginInfo `yaml:"plugins"`
	// UpdatedAt timestamp when registry was last updated
	UpdatedAt time.Time `yaml:"updated_at"`
}

// PluginInfo contains metadata about a plugin from the registry.
type PluginInfo struct {
	// Name of the plugin
	Name string `yaml:"name"`
	// Description of what the plugin does
	Description string `yaml:"description"`
	// Repository where the plugin source code lives (owner/repo)
	Repository string `yaml:"repository"`
	// Path to the plugin within the repository
	Path string `yaml:"path"`
	// Version of the plugin
	Version string `yaml:"version"`
	// Category of the plugin (vcs, notification, package_manager, etc.)
	Category string `yaml:"category"`
	// Hooks that the plugin implements
	Hooks []plugin.Hook `yaml:"hooks"`
	// ConfigSchema defines the configuration options for the plugin
	ConfigSchema map[string]ConfigField `yaml:"config_schema"`
	// Author of the plugin
	Author string `yaml:"author,omitempty"`
	// Homepage URL for the plugin
	Homepage string `yaml:"homepage,omitempty"`
	// License of the plugin
	License string `yaml:"license,omitempty"`
}

// ConfigField defines a configuration field for a plugin.
type ConfigField struct {
	// Type of the field (string, boolean, integer, array, object)
	Type string `yaml:"type"`
	// Required indicates if the field is required
	Required bool `yaml:"required"`
	// Default value for the field
	Default any `yaml:"default,omitempty"`
	// Description of what the field does
	Description string `yaml:"description"`
	// Env is the environment variable to read the value from
	Env string `yaml:"env,omitempty"`
	// Options for enum/select fields
	Options []string `yaml:"options,omitempty"`
	// Validation rules
	Pattern  string `yaml:"pattern,omitempty"`
	MinValue *int   `yaml:"min_value,omitempty"`
	MaxValue *int   `yaml:"max_value,omitempty"`
}

// InstalledPlugin represents a plugin that is installed locally.
type InstalledPlugin struct {
	// Name of the plugin
	Name string `yaml:"name"`
	// Version of the installed plugin
	Version string `yaml:"version"`
	// InstalledAt timestamp when the plugin was installed
	InstalledAt time.Time `yaml:"installed_at"`
	// BinaryPath is the path to the plugin binary
	BinaryPath string `yaml:"binary_path"`
	// Checksum of the plugin binary (SHA256)
	Checksum string `yaml:"checksum"`
	// Enabled indicates if the plugin is enabled in the config
	Enabled bool `yaml:"enabled"`
}

// Manifest tracks all installed plugins.
type Manifest struct {
	// Version of the manifest schema
	Version string `yaml:"version"`
	// Installed plugins
	Installed []InstalledPlugin `yaml:"installed"`
	// UpdatedAt timestamp when manifest was last updated
	UpdatedAt time.Time `yaml:"updated_at"`
}

// PluginStatus represents the status of a plugin.
type PluginStatus string

const (
	// StatusNotInstalled means the plugin is not installed
	StatusNotInstalled PluginStatus = "not_installed"
	// StatusInstalled means the plugin is installed but not enabled
	StatusInstalled PluginStatus = "installed"
	// StatusEnabled means the plugin is installed and enabled
	StatusEnabled PluginStatus = "enabled"
	// StatusUpdateAvailable means a newer version is available
	StatusUpdateAvailable PluginStatus = "update_available"
)

// PluginListEntry combines registry and installation information.
type PluginListEntry struct {
	Info      PluginInfo
	Installed *InstalledPlugin
	Status    PluginStatus
}
