package mcp

import (
	"testing"
	"time"
)

func TestNewResourceCache(t *testing.T) {
	cache := NewResourceCache()
	if cache == nil {
		t.Fatal("NewResourceCache returned nil")
	}
	if !cache.IsEnabled() {
		t.Error("cache should be enabled by default")
	}
}

func TestResourceCache_GetSet(t *testing.T) {
	cache := NewResourceCache()

	// Get on empty cache returns nil
	result := cache.Get("relicta://state")
	if result != nil {
		t.Error("expected nil for missing key")
	}

	// Set and get
	expected := &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://state", Text: "test"}},
	}
	cache.Set("relicta://state", expected)

	result = cache.Get("relicta://state")
	if result == nil {
		t.Fatal("expected cached result")
	}
	if result.Contents[0].Text != "test" {
		t.Error("cached content mismatch")
	}
}

func TestResourceCache_Expiration(t *testing.T) {
	cache := NewResourceCache()

	// Set a very short TTL
	cache.SetTTL("relicta://state", 1*time.Millisecond)

	cache.Set("relicta://state", &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://state", Text: "test"}},
	})

	// Should be available immediately
	if cache.Get("relicta://state") == nil {
		t.Error("expected cached result")
	}

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	// Should be expired now
	if cache.Get("relicta://state") != nil {
		t.Error("expected expired result to return nil")
	}
}

func TestResourceCache_Invalidate(t *testing.T) {
	cache := NewResourceCache()

	cache.Set("relicta://state", &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://state", Text: "test"}},
	})
	cache.Set("relicta://config", &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://config", Text: "config"}},
	})

	// Invalidate one key
	cache.Invalidate("relicta://state")

	if cache.Get("relicta://state") != nil {
		t.Error("expected invalidated key to return nil")
	}
	if cache.Get("relicta://config") == nil {
		t.Error("expected non-invalidated key to remain")
	}
}

func TestResourceCache_InvalidateAll(t *testing.T) {
	cache := NewResourceCache()

	cache.Set("relicta://state", &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://state", Text: "test"}},
	})
	cache.Set("relicta://config", &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://config", Text: "config"}},
	})

	cache.InvalidateAll()

	if cache.Get("relicta://state") != nil {
		t.Error("expected all keys to be invalidated")
	}
	if cache.Get("relicta://config") != nil {
		t.Error("expected all keys to be invalidated")
	}
}

func TestResourceCache_InvalidateStateDependent(t *testing.T) {
	cache := NewResourceCache()

	// Set all resource types
	cache.Set("relicta://state", &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://state", Text: "state"}},
	})
	cache.Set("relicta://config", &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://config", Text: "config"}},
	})
	cache.Set("relicta://commits", &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://commits", Text: "commits"}},
	})
	cache.Set("relicta://changelog", &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://changelog", Text: "changelog"}},
	})
	cache.Set("relicta://risk-report", &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://risk-report", Text: "risk"}},
	})

	cache.InvalidateStateDependent()

	// Config should remain (not state-dependent)
	if cache.Get("relicta://config") == nil {
		t.Error("config should not be invalidated")
	}

	// State-dependent resources should be invalidated
	if cache.Get("relicta://state") != nil {
		t.Error("state should be invalidated")
	}
	if cache.Get("relicta://commits") != nil {
		t.Error("commits should be invalidated")
	}
	if cache.Get("relicta://changelog") != nil {
		t.Error("changelog should be invalidated")
	}
	if cache.Get("relicta://risk-report") != nil {
		t.Error("risk-report should be invalidated")
	}
}

func TestResourceCache_SetEnabled(t *testing.T) {
	cache := NewResourceCache()

	cache.Set("relicta://state", &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://state", Text: "test"}},
	})

	// Disable cache
	cache.SetEnabled(false)
	if cache.IsEnabled() {
		t.Error("cache should be disabled")
	}

	// Should not return cached values when disabled
	if cache.Get("relicta://state") != nil {
		t.Error("disabled cache should return nil")
	}

	// Should not cache new values when disabled
	cache.Set("relicta://config", &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://config", Text: "config"}},
	})

	// Re-enable and check
	cache.SetEnabled(true)
	if cache.Get("relicta://config") != nil {
		t.Error("value set while disabled should not be cached")
	}
}

func TestResourceCache_Stats(t *testing.T) {
	cache := NewResourceCache()

	cache.Set("relicta://state", &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://state", Text: "test"}},
	})
	cache.Set("relicta://config", &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://config", Text: "config"}},
	})

	stats := cache.Stats()
	if stats.EntryCount != 2 {
		t.Errorf("expected 2 entries, got %d", stats.EntryCount)
	}
	if !stats.Enabled {
		t.Error("stats should show cache enabled")
	}
	if _, ok := stats.Entries["relicta://state"]; !ok {
		t.Error("stats should include state entry")
	}
}

func TestResourceCache_Cleanup(t *testing.T) {
	cache := NewResourceCache()

	// Set with short TTL
	cache.SetTTL("relicta://state", 1*time.Millisecond)
	cache.Set("relicta://state", &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://state", Text: "test"}},
	})

	// Set with long TTL
	cache.SetTTL("relicta://config", 1*time.Hour)
	cache.Set("relicta://config", &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://config", Text: "config"}},
	})

	// Wait for first to expire
	time.Sleep(5 * time.Millisecond)

	// Cleanup should remove expired entries
	cache.Cleanup()

	stats := cache.Stats()
	if stats.EntryCount != 1 {
		t.Errorf("expected 1 entry after cleanup, got %d", stats.EntryCount)
	}
	if _, ok := stats.Entries["relicta://config"]; !ok {
		t.Error("config should remain after cleanup")
	}
}

func TestResourceCache_NilResult(t *testing.T) {
	cache := NewResourceCache()

	// Setting nil should not add to cache
	cache.Set("relicta://state", nil)

	stats := cache.Stats()
	if stats.EntryCount != 0 {
		t.Error("nil result should not be cached")
	}
}

func TestResourceCache_UnknownResourceTTL(t *testing.T) {
	cache := NewResourceCache()

	// Unknown resource type should use default TTL
	cache.Set("relicta://unknown", &ReadResourceResult{
		Contents: []ResourceContent{{URI: "relicta://unknown", Text: "test"}},
	})

	// Should be cached
	if cache.Get("relicta://unknown") == nil {
		t.Error("unknown resource should be cached with default TTL")
	}
}
