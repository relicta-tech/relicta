package manager

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// OfficialRegistryName is the name of the official registry
	OfficialRegistryName = "official"
	// OfficialRegistryURL is the default location of the official plugin registry
	OfficialRegistryURL = "https://raw.githubusercontent.com/relicta-tech/relicta/main/plugins/registry.yaml"
	// RegistryCacheFile is the name of the cached registry file
	RegistryCacheFile = "registry.yaml"
	// RegistryConfigFile is the name of the registry configuration file
	RegistryConfigFile = "registries.yaml"
	// RegistryCacheDuration is how long to cache the registry
	RegistryCacheDuration = 24 * time.Hour
)

// RegistryService manages plugin registries.
type RegistryService struct {
	configDir  string
	cacheDir   string
	httpClient *http.Client
	config     *RegistryConfig
}

// NewRegistryService creates a new registry service.
func NewRegistryService(configDir, cacheDir string) *RegistryService {
	rs := &RegistryService{
		configDir: configDir,
		cacheDir:  cacheDir,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Load or create default config
	rs.loadConfig()

	return rs
}

// loadConfig loads the registry configuration from disk.
func (r *RegistryService) loadConfig() {
	configPath := filepath.Join(r.configDir, RegistryConfigFile)
	data, err := os.ReadFile(configPath) // #nosec G304 -- path from app config dir
	if err != nil {
		// Create default config with official registry
		r.config = r.defaultConfig()
		return
	}

	var config RegistryConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		r.config = r.defaultConfig()
		return
	}

	r.config = &config

	// Ensure official registry is always present
	r.ensureOfficialRegistry()
}

// defaultConfig returns the default registry configuration.
func (r *RegistryService) defaultConfig() *RegistryConfig {
	return &RegistryConfig{
		Version: "1.0",
		Registries: []RegistryEntry{
			{
				Name:     OfficialRegistryName,
				URL:      OfficialRegistryURL,
				Priority: 1000, // Highest priority
				Enabled:  true,
			},
		},
	}
}

// ensureOfficialRegistry ensures the official registry is always in the config.
func (r *RegistryService) ensureOfficialRegistry() {
	for _, reg := range r.config.Registries {
		if reg.Name == OfficialRegistryName {
			return
		}
	}

	// Add official registry at the beginning
	r.config.Registries = append([]RegistryEntry{
		{
			Name:     OfficialRegistryName,
			URL:      OfficialRegistryURL,
			Priority: 1000,
			Enabled:  true,
		},
	}, r.config.Registries...)
}

// saveConfig saves the registry configuration to disk.
func (r *RegistryService) saveConfig() error {
	if err := os.MkdirAll(r.configDir, 0o755); err != nil { // #nosec G301 -- config dirs need exec
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(r.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(r.configDir, RegistryConfigFile)
	if err := os.WriteFile(configPath, data, 0o644); err != nil { // #nosec G306 -- config readable by user
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ListRegistries returns all configured registries.
func (r *RegistryService) ListRegistries() []RegistryEntry {
	// Sort by priority (highest first)
	registries := make([]RegistryEntry, len(r.config.Registries))
	copy(registries, r.config.Registries)

	sort.Slice(registries, func(i, j int) bool {
		return registries[i].Priority > registries[j].Priority
	})

	return registries
}

// AddRegistry adds a new registry.
func (r *RegistryService) AddRegistry(name, url string, priority int) error {
	// Check if name already exists
	for _, reg := range r.config.Registries {
		if reg.Name == name {
			return fmt.Errorf("registry %q already exists", name)
		}
	}

	// Prevent overriding official registry
	if name == OfficialRegistryName {
		return fmt.Errorf("cannot add registry with reserved name %q", OfficialRegistryName)
	}

	r.config.Registries = append(r.config.Registries, RegistryEntry{
		Name:     name,
		URL:      url,
		Priority: priority,
		Enabled:  true,
	})

	return r.saveConfig()
}

// RemoveRegistry removes a registry by name.
func (r *RegistryService) RemoveRegistry(name string) error {
	if name == OfficialRegistryName {
		return fmt.Errorf("cannot remove the official registry")
	}

	for i, reg := range r.config.Registries {
		if reg.Name == name {
			r.config.Registries = append(r.config.Registries[:i], r.config.Registries[i+1:]...)
			return r.saveConfig()
		}
	}

	return fmt.Errorf("registry %q not found", name)
}

// EnableRegistry enables or disables a registry.
func (r *RegistryService) EnableRegistry(name string, enabled bool) error {
	for i, reg := range r.config.Registries {
		if reg.Name == name {
			r.config.Registries[i].Enabled = enabled
			return r.saveConfig()
		}
	}

	return fmt.Errorf("registry %q not found", name)
}

// Fetch retrieves plugins from all enabled registries.
func (r *RegistryService) Fetch(ctx context.Context, forceRefresh bool) (*Registry, error) {
	// Get enabled registries sorted by priority
	registries := r.ListRegistries()

	// Merged registry
	merged := &Registry{
		Version:   "1.0",
		UpdatedAt: time.Now(),
		Plugins:   []PluginInfo{},
	}

	// Track seen plugins (first occurrence wins - higher priority)
	seen := make(map[string]bool)

	for _, regEntry := range registries {
		if !regEntry.Enabled {
			continue
		}

		registry, err := r.fetchRegistry(ctx, regEntry, forceRefresh)
		if err != nil {
			// Log error but continue with other registries
			fmt.Fprintf(os.Stderr, "Warning: failed to fetch registry %q: %v\n", regEntry.Name, err)
			continue
		}

		// Add plugins that haven't been seen yet
		for _, plugin := range registry.Plugins {
			if !seen[plugin.Name] {
				plugin.Source = regEntry.URL
				merged.Plugins = append(merged.Plugins, plugin)
				seen[plugin.Name] = true
			}
		}
	}

	return merged, nil
}

// fetchRegistry fetches a single registry.
func (r *RegistryService) fetchRegistry(ctx context.Context, entry RegistryEntry, forceRefresh bool) (*Registry, error) {
	cachePath := r.getCachePath(entry.Name)

	// Try to use cache if not forcing refresh
	if !forceRefresh {
		if registry, err := r.loadFromCache(cachePath); err == nil {
			// Check if cache is still fresh
			if time.Since(registry.UpdatedAt) < RegistryCacheDuration {
				return registry, nil
			}
		}
	}

	// Fetch from remote
	registry, err := r.fetchFromRemote(ctx, entry.URL)
	if err != nil {
		// If fetch fails, try to use stale cache as fallback
		if cached, cacheErr := r.loadFromCache(cachePath); cacheErr == nil {
			return cached, nil
		}
		return nil, fmt.Errorf("failed to fetch registry from %s: %w", entry.URL, err)
	}

	// Update cache (ignore errors - cache is optional)
	_ = r.saveToCache(cachePath, registry)

	return registry, nil
}

// getCachePath returns the cache file path for a registry.
func (r *RegistryService) getCachePath(name string) string {
	return filepath.Join(r.cacheDir, fmt.Sprintf("registry-%s.yaml", name))
}

// fetchFromRemote downloads the registry from the remote URL.
func (r *RegistryService) fetchFromRemote(ctx context.Context, url string) (*Registry, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var registry Registry
	if err := yaml.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("failed to parse registry: %w", err)
	}

	registry.UpdatedAt = time.Now()
	return &registry, nil
}

// loadFromCache loads the registry from the cache file.
func (r *RegistryService) loadFromCache(cachePath string) (*Registry, error) {
	data, err := os.ReadFile(cachePath) // #nosec G304 -- path from app cache dir
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var registry Registry
	if err := yaml.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("failed to parse cached registry: %w", err)
	}

	return &registry, nil
}

// saveToCache saves the registry to the cache file.
func (r *RegistryService) saveToCache(cachePath string, registry *Registry) error {
	// Ensure cache directory exists
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil { // #nosec G301 -- cache dirs need exec
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	data, err := yaml.Marshal(registry)
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	if err := os.WriteFile(cachePath, data, 0o644); err != nil { // #nosec G306 -- cache readable by user
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// GetPlugin retrieves information about a specific plugin from the registry.
func (r *Registry) GetPlugin(name string) (*PluginInfo, error) {
	for i := range r.Plugins {
		if r.Plugins[i].Name == name {
			return &r.Plugins[i], nil
		}
	}
	return nil, fmt.Errorf("plugin %q not found in registry", name)
}

// ListByCategory returns plugins filtered by category.
func (r *Registry) ListByCategory(category string) []PluginInfo {
	if category == "" {
		return r.Plugins
	}

	var filtered []PluginInfo
	for _, p := range r.Plugins {
		if p.Category == category {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// Categories returns all unique plugin categories.
func (r *Registry) Categories() []string {
	seen := make(map[string]bool)
	var categories []string

	for _, p := range r.Plugins {
		if !seen[p.Category] {
			seen[p.Category] = true
			categories = append(categories, p.Category)
		}
	}

	return categories
}
