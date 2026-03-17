// Package ai provides AI-powered content generation for Relicta.
package ai

import (
	"fmt"
	"sync"
)

// ProviderFactory creates an AI service for a specific provider.
type ProviderFactory func(cfg ServiceConfig) (Service, error)

var (
	providerRegistry = make(map[string]ProviderFactory)
	registryMu       sync.RWMutex
)

// RegisterProvider registers a provider factory.
// This is typically called in init() functions of provider-specific files.
func RegisterProvider(name string, factory ProviderFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	providerRegistry[name] = factory
}

// GetProvider returns the factory for a provider, or nil if not registered.
func GetProvider(name string) ProviderFactory {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return providerRegistry[name]
}

// IsProviderAvailable checks if a provider is registered.
func IsProviderAvailable(name string) bool {
	return GetProvider(name) != nil
}

// ListProviders returns a list of registered provider names.
func ListProviders() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	providers := make([]string, 0, len(providerRegistry))
	for name := range providerRegistry {
		providers = append(providers, name)
	}
	return providers
}

// ProviderNotAvailableError is returned when a provider is not compiled in.
type ProviderNotAvailableError struct {
	Provider string
}

func (e ProviderNotAvailableError) Error() string {
	return fmt.Sprintf("AI provider %q is not available in this build; available providers: %v", e.Provider, ListProviders())
}
