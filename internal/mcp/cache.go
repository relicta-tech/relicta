package mcp

import (
	"sync"
	"time"
)

// ResourceCache provides caching for MCP resource reads.
// Resources are cached with configurable TTLs and can be invalidated
// when tool operations modify state.
type ResourceCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttls    map[string]time.Duration
	enabled bool
}

// cacheEntry holds a cached resource result.
type cacheEntry struct {
	result    *ReadResourceResult
	expiresAt time.Time
}

// Default TTLs for different resource types.
const (
	// StateTTL is short since state changes frequently during release workflow.
	StateTTL = 5 * time.Second

	// ConfigTTL is longer since config rarely changes during a session.
	ConfigTTL = 5 * time.Minute

	// CommitsTTL is medium since commits don't change but new releases may be planned.
	CommitsTTL = 30 * time.Second

	// ChangelogTTL is medium since notes may be regenerated.
	ChangelogTTL = 30 * time.Second

	// RiskReportTTL is medium since evaluation results may change.
	RiskReportTTL = 30 * time.Second
)

// NewResourceCache creates a new resource cache with default TTLs.
func NewResourceCache() *ResourceCache {
	return &ResourceCache{
		entries: make(map[string]*cacheEntry),
		ttls: map[string]time.Duration{
			"relicta://state":       StateTTL,
			"relicta://config":      ConfigTTL,
			"relicta://commits":     CommitsTTL,
			"relicta://changelog":   ChangelogTTL,
			"relicta://risk-report": RiskReportTTL,
		},
		enabled: true,
	}
}

// Get retrieves a cached resource result if available and not expired.
// Returns nil if the resource is not cached or has expired.
func (c *ResourceCache) Get(uri string) *ReadResourceResult {
	if !c.enabled {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[uri]
	if !ok {
		return nil
	}

	if time.Now().After(entry.expiresAt) {
		// Entry expired, will be cleaned up on next Set
		return nil
	}

	return entry.result
}

// Set caches a resource result with the configured TTL for that resource type.
func (c *ResourceCache) Set(uri string, result *ReadResourceResult) {
	if !c.enabled || result == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	ttl := c.ttls[uri]
	if ttl == 0 {
		// Unknown resource type, use default TTL
		ttl = 10 * time.Second
	}

	c.entries[uri] = &cacheEntry{
		result:    result,
		expiresAt: time.Now().Add(ttl),
	}
}

// Invalidate removes a specific resource from the cache.
func (c *ResourceCache) Invalidate(uri string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, uri)
}

// InvalidateAll removes all resources from the cache.
func (c *ResourceCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
}

// InvalidateStateDependent invalidates resources that depend on release state.
// This should be called after tools that modify state (plan, bump, notes, approve, publish).
func (c *ResourceCache) InvalidateStateDependent() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// These resources depend on release state
	stateDependent := []string{
		"relicta://state",
		"relicta://commits",
		"relicta://changelog",
		"relicta://risk-report",
	}

	for _, uri := range stateDependent {
		delete(c.entries, uri)
	}
}

// SetTTL configures the TTL for a specific resource URI.
func (c *ResourceCache) SetTTL(uri string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ttls[uri] = ttl
}

// SetEnabled enables or disables caching.
func (c *ResourceCache) SetEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = enabled
	if !enabled {
		c.entries = make(map[string]*cacheEntry)
	}
}

// IsEnabled returns whether caching is enabled.
func (c *ResourceCache) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

// Stats returns cache statistics.
func (c *ResourceCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CacheStats{
		Enabled:    c.enabled,
		EntryCount: len(c.entries),
		Entries:    make(map[string]CacheEntryStats),
	}

	now := time.Now()
	for uri, entry := range c.entries {
		stats.Entries[uri] = CacheEntryStats{
			ExpiresIn: entry.expiresAt.Sub(now),
			Expired:   now.After(entry.expiresAt),
		}
	}

	return stats
}

// CacheStats provides information about the cache state.
type CacheStats struct {
	Enabled    bool
	EntryCount int
	Entries    map[string]CacheEntryStats
}

// CacheEntryStats provides information about a single cache entry.
type CacheEntryStats struct {
	ExpiresIn time.Duration
	Expired   bool
}

// Cleanup removes expired entries from the cache.
// This can be called periodically to prevent memory growth.
func (c *ResourceCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for uri, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, uri)
		}
	}
}
