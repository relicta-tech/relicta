package manager

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistryService_LoadConfig_Defaults(t *testing.T) {
	service := NewRegistryService(t.TempDir(), t.TempDir())

	registries := service.ListRegistries()
	if len(registries) == 0 {
		t.Fatal("expected default registries to be present")
	}

	found := false
	for _, reg := range registries {
		if reg.Name == OfficialRegistryName {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %s registry to be present", OfficialRegistryName)
	}
}

func TestRegistryService_EnsureOfficialRegistry(t *testing.T) {
	configDir := t.TempDir()
	cacheDir := t.TempDir()

	writeRegistryConfig(t, configDir, []RegistryEntry{
		{Name: "custom", URL: "https://example.com/registry.yaml", Priority: 1, Enabled: true},
	})

	service := NewRegistryService(configDir, cacheDir)
	registries := service.ListRegistries()
	if len(registries) < 2 {
		t.Fatalf("expected official registry to be injected, got %d", len(registries))
	}
	if registries[0].Name != OfficialRegistryName {
		t.Fatalf("expected official registry first, got %s", registries[0].Name)
	}
}

func TestRegistryService_AddRemoveEnable(t *testing.T) {
	configDir := t.TempDir()
	cacheDir := t.TempDir()

	writeRegistryConfig(t, configDir, []RegistryEntry{
		{Name: OfficialRegistryName, URL: OfficialRegistryURL, Priority: 1000, Enabled: false},
	})

	service := NewRegistryService(configDir, cacheDir)
	if err := service.AddRegistry("custom", "https://example.com/registry.yaml", 10); err != nil {
		t.Fatalf("AddRegistry error: %v", err)
	}
	if err := service.EnableRegistry("custom", false); err != nil {
		t.Fatalf("EnableRegistry error: %v", err)
	}
	if err := service.RemoveRegistry("custom"); err != nil {
		t.Fatalf("RemoveRegistry error: %v", err)
	}

	if err := service.AddRegistry(OfficialRegistryName, "https://example.com", 1); err == nil {
		t.Fatal("expected AddRegistry to reject official name")
	}
	if err := service.RemoveRegistry(OfficialRegistryName); err == nil {
		t.Fatal("expected RemoveRegistry to reject official registry removal")
	}
	if err := service.EnableRegistry("missing", true); err == nil {
		t.Fatal("expected EnableRegistry to fail for missing registry")
	}
}

func TestRegistryService_Fetch_UsesCache(t *testing.T) {
	configDir := t.TempDir()
	cacheDir := t.TempDir()

	writeRegistryConfig(t, configDir, []RegistryEntry{
		{Name: OfficialRegistryName, URL: OfficialRegistryURL, Priority: 1000, Enabled: false},
		{Name: "local", URL: "https://example.com/registry.yaml", Priority: 1, Enabled: true},
	})

	writeRegistryCache(t, cacheDir, "local", Registry{
		Version: "1.0",
		Plugins: []PluginInfo{{Name: "alpha", Version: "1.0.0"}},
	})

	service := NewRegistryService(configDir, cacheDir)
	registry, err := service.Fetch(context.Background(), false)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(registry.Plugins) != 1 {
		t.Fatalf("plugins len = %d, want 1", len(registry.Plugins))
	}
	if registry.Plugins[0].Source != "https://example.com/registry.yaml" {
		t.Fatalf("plugin Source = %s, want registry URL", registry.Plugins[0].Source)
	}
}

func TestRegistryService_Fetch_Remote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write([]byte(`version: "1.0"
plugins:
  - name: alpha
    version: "1.0.0"
updated_at: "` + time.Now().Format(time.RFC3339Nano) + `"`))
	}))
	t.Cleanup(server.Close)

	configDir := t.TempDir()
	cacheDir := t.TempDir()

	writeRegistryConfig(t, configDir, []RegistryEntry{
		{Name: OfficialRegistryName, URL: OfficialRegistryURL, Priority: 1000, Enabled: false},
		{Name: "remote", URL: server.URL, Priority: 1, Enabled: true},
	})

	service := NewRegistryService(configDir, cacheDir)
	registry, err := service.Fetch(context.Background(), true)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(registry.Plugins) != 1 {
		t.Fatalf("plugins len = %d, want 1", len(registry.Plugins))
	}
	if registry.Plugins[0].Source != server.URL {
		t.Fatalf("plugin Source = %s, want %s", registry.Plugins[0].Source, server.URL)
	}
}

func TestRegistry_ListHelpers(t *testing.T) {
	registry := &Registry{
		Plugins: []PluginInfo{
			{Name: "alpha", Category: "vcs"},
			{Name: "beta", Category: "vcs"},
			{Name: "gamma", Category: "notify"},
		},
	}

	if _, err := registry.GetPlugin("missing"); err == nil {
		t.Fatal("expected GetPlugin to return error for missing plugin")
	}
	if got := registry.ListByCategory("vcs"); len(got) != 2 {
		t.Fatalf("ListByCategory len = %d, want 2", len(got))
	}
	if got := registry.ListByCategory(""); len(got) != 3 {
		t.Fatalf("ListByCategory all len = %d, want 3", len(got))
	}
	if got := registry.Categories(); len(got) != 2 {
		t.Fatalf("Categories len = %d, want 2", len(got))
	}
}

func TestRegistryService_SaveAndLoadCache(t *testing.T) {
	cacheDir := t.TempDir()
	service := NewRegistryService(t.TempDir(), cacheDir)

	registry := &Registry{
		Version: "1.0",
		Plugins: []PluginInfo{
			{Name: "alpha", Version: "1.0.0"},
		},
		UpdatedAt: time.Now(),
	}

	cachePath := service.getCachePath("local")
	if err := service.saveToCache(cachePath, registry); err != nil {
		t.Fatalf("saveToCache error: %v", err)
	}

	loaded, err := service.loadFromCache(cachePath)
	if err != nil {
		t.Fatalf("loadFromCache error: %v", err)
	}
	if len(loaded.Plugins) != 1 || loaded.Plugins[0].Name != "alpha" {
		t.Fatalf("loaded registry = %#v", loaded)
	}
}

func TestRegistryService_SaveConfig_Error(t *testing.T) {
	tmpDir := t.TempDir()
	blocker := filepath.Join(tmpDir, "config-blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	service := NewRegistryService(tmpDir, t.TempDir())
	service.configDir = blocker

	if err := service.saveConfig(); err == nil {
		t.Fatal("expected saveConfig to fail with file as directory")
	}
}
