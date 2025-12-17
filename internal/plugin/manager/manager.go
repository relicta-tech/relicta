package manager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultPluginDir is the default directory for plugins
	DefaultPluginDir = ".relicta/plugins"
	// DefaultCacheDir is the default directory for cache
	DefaultCacheDir = ".relicta/cache"
	// DefaultConfigDir is the default directory for configuration
	DefaultConfigDir = ".relicta"
	// ManifestFile is the name of the manifest file
	ManifestFile = "manifest.yaml"
)

// Manager coordinates plugin management operations.
type Manager struct {
	registry     *RegistryService
	installer    *Installer
	pluginDir    string
	cacheDir     string
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
	configDir := filepath.Join(home, DefaultConfigDir)

	// Ensure directories exist
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create plugin directory: %w", err)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	return &Manager{
		registry:     NewRegistryService(configDir, cacheDir),
		installer:    NewInstaller(pluginDir),
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

// Install installs a plugin from the registry.
func (m *Manager) Install(ctx context.Context, name string) error {
	// Get plugin info from registry
	registry, err := m.registry.Fetch(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to fetch registry: %w", err)
	}

	pluginInfo, err := registry.GetPlugin(name)
	if err != nil {
		return err
	}

	// Check if already installed
	manifest, err := m.getOrCreateManifest()
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	for _, installed := range manifest.Installed {
		if installed.Name == name {
			return fmt.Errorf("plugin %q is already installed (version %s)", name, installed.Version)
		}
	}

	// Install the plugin
	installed, err := m.installer.Install(ctx, *pluginInfo)
	if err != nil {
		return fmt.Errorf("failed to install plugin: %w", err)
	}

	// Update manifest
	manifest.Installed = append(manifest.Installed, *installed)
	if err := m.saveManifest(manifest); err != nil {
		// Clean up installed binary on manifest save failure
		_ = m.installer.Uninstall(*installed)
		return fmt.Errorf("failed to update manifest: %w", err)
	}

	return nil
}

// Uninstall removes a plugin.
func (m *Manager) Uninstall(ctx context.Context, name string) error {
	manifest, err := m.loadManifest()
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	// Find the plugin
	var found bool
	var toUninstall InstalledPlugin
	var updatedInstalled []InstalledPlugin

	for _, installed := range manifest.Installed {
		if installed.Name == name {
			found = true
			toUninstall = installed
		} else {
			updatedInstalled = append(updatedInstalled, installed)
		}
	}

	if !found {
		return fmt.Errorf("plugin %q is not installed", name)
	}

	// Uninstall the binary
	if err := m.installer.Uninstall(toUninstall); err != nil {
		return fmt.Errorf("failed to uninstall plugin: %w", err)
	}

	// Update manifest
	manifest.Installed = updatedInstalled
	if err := m.saveManifest(manifest); err != nil {
		return fmt.Errorf("failed to update manifest: %w", err)
	}

	return nil
}

// Enable enables an installed plugin.
func (m *Manager) Enable(ctx context.Context, name string) error {
	manifest, err := m.loadManifest()
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	var found bool
	for i := range manifest.Installed {
		if manifest.Installed[i].Name == name {
			found = true
			manifest.Installed[i].Enabled = true
			break
		}
	}

	if !found {
		return fmt.Errorf("plugin %q is not installed", name)
	}

	return m.saveManifest(manifest)
}

// Disable disables an installed plugin.
func (m *Manager) Disable(ctx context.Context, name string) error {
	manifest, err := m.loadManifest()
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	var found bool
	for i := range manifest.Installed {
		if manifest.Installed[i].Name == name {
			found = true
			manifest.Installed[i].Enabled = false
			break
		}
	}

	if !found {
		return fmt.Errorf("plugin %q is not installed", name)
	}

	return m.saveManifest(manifest)
}

// ListRegistries returns all configured plugin registries.
func (m *Manager) ListRegistries() []RegistryEntry {
	return m.registry.ListRegistries()
}

// AddRegistry adds a new plugin registry.
func (m *Manager) AddRegistry(name, url string, priority int) error {
	return m.registry.AddRegistry(name, url, priority)
}

// RemoveRegistry removes a plugin registry by name.
func (m *Manager) RemoveRegistry(name string) error {
	return m.registry.RemoveRegistry(name)
}

// EnableRegistry enables or disables a registry.
func (m *Manager) EnableRegistry(name string, enabled bool) error {
	return m.registry.EnableRegistry(name, enabled)
}

// Update updates an installed plugin to the latest version.
func (m *Manager) Update(ctx context.Context, name string) (*UpdateResult, error) {
	// Load manifest to check if plugin is installed
	manifest, err := m.loadManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Find installed plugin
	var installedPlugin *InstalledPlugin
	var installedIndex int
	for i, p := range manifest.Installed {
		if p.Name == name {
			installedPlugin = &manifest.Installed[i]
			installedIndex = i
			break
		}
	}

	if installedPlugin == nil {
		return nil, fmt.Errorf("plugin %q is not installed", name)
	}

	// Get latest version from registry
	registry, err := m.registry.Fetch(ctx, true) // Force refresh
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}

	pluginInfo, err := registry.GetPlugin(name)
	if err != nil {
		return nil, fmt.Errorf("plugin %q not found in registry", name)
	}

	// Check if update is available
	if pluginInfo.Version == installedPlugin.Version {
		return &UpdateResult{
			Name:           name,
			CurrentVersion: installedPlugin.Version,
			LatestVersion:  pluginInfo.Version,
			Updated:        false,
		}, nil
	}

	// Preserve enabled state
	wasEnabled := installedPlugin.Enabled

	// Uninstall old version
	if err := m.installer.Uninstall(*installedPlugin); err != nil {
		return nil, fmt.Errorf("failed to uninstall old version: %w", err)
	}

	// Install new version
	newInstalled, err := m.installer.Install(ctx, *pluginInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to install new version: %w", err)
	}

	// Restore enabled state
	newInstalled.Enabled = wasEnabled

	// Update manifest
	manifest.Installed[installedIndex] = *newInstalled
	if err := m.saveManifest(manifest); err != nil {
		return nil, fmt.Errorf("failed to update manifest: %w", err)
	}

	return &UpdateResult{
		Name:           name,
		CurrentVersion: installedPlugin.Version,
		LatestVersion:  pluginInfo.Version,
		Updated:        true,
	}, nil
}

// UpdateAll updates all installed plugins to their latest versions.
func (m *Manager) UpdateAll(ctx context.Context) ([]UpdateResult, error) {
	manifest, err := m.loadManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	var results []UpdateResult
	for _, installed := range manifest.Installed {
		result, err := m.Update(ctx, installed.Name)
		if err != nil {
			results = append(results, UpdateResult{
				Name:           installed.Name,
				CurrentVersion: installed.Version,
				Error:          err.Error(),
			})
			continue
		}
		results = append(results, *result)
	}

	return results, nil
}

// Search searches for plugins matching the query.
func (m *Manager) Search(ctx context.Context, query string) ([]PluginInfo, error) {
	registry, err := m.registry.Fetch(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}

	query = strings.ToLower(query)
	var matches []PluginInfo

	for _, plugin := range registry.Plugins {
		// Search in name, description, and category
		if strings.Contains(strings.ToLower(plugin.Name), query) ||
			strings.Contains(strings.ToLower(plugin.Description), query) ||
			strings.Contains(strings.ToLower(plugin.Category), query) {
			matches = append(matches, plugin)
		}
	}

	return matches, nil
}
