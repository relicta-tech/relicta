package manager

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultRegistryURL is the default location of the plugin registry
	DefaultRegistryURL = "https://raw.githubusercontent.com/relicta-tech/relicta/main/plugins/registry.yaml"
	// RegistryCacheFile is the name of the cached registry file
	RegistryCacheFile = "registry.yaml"
	// RegistryCacheDuration is how long to cache the registry
	RegistryCacheDuration = 24 * time.Hour
)

// RegistryService manages the plugin registry.
type RegistryService struct {
	registryURL string
	cacheDir    string
	httpClient  *http.Client
}

// NewRegistryService creates a new registry service.
func NewRegistryService(cacheDir string) *RegistryService {
	return &RegistryService{
		registryURL: DefaultRegistryURL,
		cacheDir:    cacheDir,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Fetch retrieves the plugin registry, using cache if available and fresh.
func (r *RegistryService) Fetch(ctx context.Context, forceRefresh bool) (*Registry, error) {
	cachePath := filepath.Join(r.cacheDir, RegistryCacheFile)

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
	registry, err := r.fetchFromRemote(ctx)
	if err != nil {
		// If fetch fails, try to use stale cache as fallback
		if cached, cacheErr := r.loadFromCache(cachePath); cacheErr == nil {
			return cached, nil
		}
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}

	// Update cache (ignore errors - cache is optional)
	_ = r.saveToCache(cachePath, registry)

	return registry, nil
}

// fetchFromRemote downloads the registry from the remote URL.
func (r *RegistryService) fetchFromRemote(ctx context.Context) (*Registry, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", r.registryURL, nil)
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
	data, err := os.ReadFile(cachePath)
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
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	data, err := yaml.Marshal(registry)
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	if err := os.WriteFile(cachePath, data, 0o644); err != nil {
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
