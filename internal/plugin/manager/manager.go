package manager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultPluginDir is the default directory for plugins
	DefaultPluginDir = ".release-pilot/plugins"
	// DefaultCacheDir is the default directory for cache
	DefaultCacheDir = ".release-pilot/cache"
	// ManifestFile is the name of the manifest file
	ManifestFile = "manifest.yaml"
)

// Manager coordinates plugin management operations.
type Manager struct {
	registry   *RegistryService
	pluginDir  string
	cacheDir   string
	manifestPath string
}

// NewManager creates a new plugin manager.
func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	pluginDir := filepath.Join(home, DefaultPluginDir)
	cacheDir := filepath.Join(home, DefaultCacheDir)

	// Ensure directories exist
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create plugin directory: %w", err)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Manager{
		registry:     NewRegistryService(cacheDir),
		pluginDir:    pluginDir,
		cacheDir:     cacheDir,
		manifestPath: filepath.Join(pluginDir, ManifestFile),
	}, nil
}

// ListAvailable returns all plugins available in the registry.
func (m *Manager) ListAvailable(ctx context.Context, forceRefresh bool) ([]PluginListEntry, error) {
	registry, err := m.registry.Fetch(ctx, forceRefresh)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}

	manifest, err := m.loadManifest()
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Build map of installed plugins for quick lookup
	installedMap := make(map[string]*InstalledPlugin)
	if manifest != nil {
		for i := range manifest.Installed {
			installedMap[manifest.Installed[i].Name] = &manifest.Installed[i]
		}
	}

	// Combine registry and installation info
	var entries []PluginListEntry
	for _, pluginInfo := range registry.Plugins {
		installed := installedMap[pluginInfo.Name]
		status := m.determineStatus(pluginInfo, installed)

		entries = append(entries, PluginListEntry{
			Info:      pluginInfo,
			Installed: installed,
			Status:    status,
		})
	}

	return entries, nil
}

// ListInstalled returns only installed plugins.
func (m *Manager) ListInstalled(ctx context.Context) ([]PluginListEntry, error) {
	manifest, err := m.loadManifest()
	if err != nil {
		if os.IsNotExist(err) {
			return []PluginListEntry{}, nil
		}
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	registry, err := m.registry.Fetch(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}

	var entries []PluginListEntry
	for _, installed := range manifest.Installed {
		pluginInfo, err := registry.GetPlugin(installed.Name)
		if err != nil {
			// Plugin no longer in registry, create minimal info
			pluginInfo = &PluginInfo{
				Name:    installed.Name,
				Version: installed.Version,
			}
		}

		status := m.determineStatus(*pluginInfo, &installed)
		entries = append(entries, PluginListEntry{
			Info:      *pluginInfo,
			Installed: &installed,
			Status:    status,
		})
	}

	return entries, nil
}

// GetPluginInfo retrieves information about a specific plugin.
func (m *Manager) GetPluginInfo(ctx context.Context, name string) (*PluginListEntry, error) {
	registry, err := m.registry.Fetch(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}

	pluginInfo, err := registry.GetPlugin(name)
	if err != nil {
		return nil, err
	}

	manifest, _ := m.loadManifest()
	var installed *InstalledPlugin
	if manifest != nil {
		for i := range manifest.Installed {
			if manifest.Installed[i].Name == name {
				installed = &manifest.Installed[i]
				break
			}
		}
	}

	status := m.determineStatus(*pluginInfo, installed)

	return &PluginListEntry{
		Info:      *pluginInfo,
		Installed: installed,
		Status:    status,
	}, nil
}

// determineStatus determines the status of a plugin based on registry and installation info.
func (m *Manager) determineStatus(info PluginInfo, installed *InstalledPlugin) PluginStatus {
	if installed == nil {
		return StatusNotInstalled
	}

	if installed.Enabled {
		// Check if update is available
		if installed.Version != info.Version {
			return StatusUpdateAvailable
		}
		return StatusEnabled
	}

	return StatusInstalled
}

// loadManifest loads the plugin manifest from disk.
func (m *Manager) loadManifest() (*Manifest, error) {
	data, err := os.ReadFile(m.manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

// saveManifest saves the plugin manifest to disk.
func (m *Manager) saveManifest(manifest *Manifest) error {
	manifest.UpdatedAt = time.Now()

	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := os.WriteFile(m.manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}

// getOrCreateManifest loads the manifest or creates a new one if it doesn't exist.
func (m *Manager) getOrCreateManifest() (*Manifest, error) {
	manifest, err := m.loadManifest()
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{
				Version:   "1.0",
				Installed: []InstalledPlugin{},
			}, nil
		}
		return nil, err
	}
	return manifest, nil
}
