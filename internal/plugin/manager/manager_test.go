package manager

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManager_ListAvailable_Statuses(t *testing.T) {
	pluginDir := t.TempDir()
	configDir := t.TempDir()
	cacheDir := t.TempDir()

	registry := Registry{
		Version: "1.0",
		Plugins: []PluginInfo{
			{Name: "alpha", Version: "1.0.0", Description: "alpha"},
			{Name: "beta", Version: "2.0.0", Description: "beta"},
		},
		UpdatedAt: time.Now(),
	}
	writeRegistryConfig(t, configDir, []RegistryEntry{
		{Name: OfficialRegistryName, URL: OfficialRegistryURL, Priority: 1000, Enabled: false},
		{Name: "local", URL: "https://example.com/registry.yaml", Priority: 10, Enabled: true},
	})
	writeRegistryCache(t, cacheDir, "local", registry)

	manifest := Manifest{
		Version: "1.0",
		Installed: []InstalledPlugin{
			{Name: "alpha", Version: "0.9.0", Enabled: true},
		},
	}
	writeManifest(t, filepath.Join(pluginDir, ManifestFile), manifest)

	mgr := &Manager{
		registry:     NewRegistryService(configDir, cacheDir),
		installer:    NewInstaller(pluginDir),
		pluginDir:    pluginDir,
		cacheDir:     cacheDir,
		manifestPath: filepath.Join(pluginDir, ManifestFile),
	}

	entries, err := mgr.ListAvailable(context.Background(), false)
	if err != nil {
		t.Fatalf("ListAvailable error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}

	var alphaStatus, betaStatus PluginStatus
	for _, entry := range entries {
		switch entry.Info.Name {
		case "alpha":
			alphaStatus = entry.Status
		case "beta":
			betaStatus = entry.Status
		}
	}

	if alphaStatus != StatusUpdateAvailable {
		t.Fatalf("alpha status = %s, want %s", alphaStatus, StatusUpdateAvailable)
	}
	if betaStatus != StatusNotInstalled {
		t.Fatalf("beta status = %s, want %s", betaStatus, StatusNotInstalled)
	}
}

func TestManager_NewManager(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	manager, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}
	if manager == nil {
		t.Fatal("NewManager returned nil manager")
	}

	if _, err := os.Stat(manager.pluginDir); err != nil {
		t.Fatalf("pluginDir not created: %v", err)
	}
	if _, err := os.Stat(manager.cacheDir); err != nil {
		t.Fatalf("cacheDir not created: %v", err)
	}
}

func TestManager_ListInstalled_MissingRegistryEntry(t *testing.T) {
	pluginDir := t.TempDir()
	configDir := t.TempDir()
	cacheDir := t.TempDir()

	writeRegistryConfig(t, configDir, []RegistryEntry{
		{Name: OfficialRegistryName, URL: OfficialRegistryURL, Priority: 1000, Enabled: false},
		{Name: "local", URL: "https://example.com/registry.yaml", Priority: 10, Enabled: true},
	})
	writeRegistryCache(t, cacheDir, "local", Registry{
		Version:   "1.0",
		Plugins:   []PluginInfo{{Name: "alpha", Version: "1.0.0"}},
		UpdatedAt: time.Now(),
	})

	writeManifest(t, filepath.Join(pluginDir, ManifestFile), Manifest{
		Version: "1.0",
		Installed: []InstalledPlugin{
			{Name: "ghost", Version: "0.1.0", Enabled: true},
		},
	})

	mgr := &Manager{
		registry:     NewRegistryService(configDir, cacheDir),
		installer:    NewInstaller(pluginDir),
		pluginDir:    pluginDir,
		cacheDir:     cacheDir,
		manifestPath: filepath.Join(pluginDir, ManifestFile),
	}

	entries, err := mgr.ListInstalled(context.Background())
	if err != nil {
		t.Fatalf("ListInstalled error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(entries))
	}
	if entries[0].Info.Name != "ghost" {
		t.Fatalf("entry name = %s, want ghost", entries[0].Info.Name)
	}
	if entries[0].Status != StatusEnabled {
		t.Fatalf("status = %s, want %s", entries[0].Status, StatusEnabled)
	}
}

func TestManager_GetPluginInfo_InstalledStatus(t *testing.T) {
	pluginDir := t.TempDir()
	configDir := t.TempDir()
	cacheDir := t.TempDir()

	writeRegistryConfig(t, configDir, []RegistryEntry{
		{Name: OfficialRegistryName, URL: OfficialRegistryURL, Priority: 1000, Enabled: false},
		{Name: "local", URL: "https://example.com/registry.yaml", Priority: 10, Enabled: true},
	})
	writeRegistryCache(t, cacheDir, "local", Registry{
		Version:   "1.0",
		Plugins:   []PluginInfo{{Name: "alpha", Version: "1.0.0"}},
		UpdatedAt: time.Now(),
	})

	writeManifest(t, filepath.Join(pluginDir, ManifestFile), Manifest{
		Version: "1.0",
		Installed: []InstalledPlugin{
			{Name: "alpha", Version: "0.9.0", Enabled: false},
		},
	})

	mgr := &Manager{
		registry:     NewRegistryService(configDir, cacheDir),
		installer:    NewInstaller(pluginDir),
		pluginDir:    pluginDir,
		cacheDir:     cacheDir,
		manifestPath: filepath.Join(pluginDir, ManifestFile),
	}

	entry, err := mgr.GetPluginInfo(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("GetPluginInfo error: %v", err)
	}
	if entry.Status != StatusInstalled {
		t.Fatalf("status = %s, want %s", entry.Status, StatusInstalled)
	}
}

func TestManager_EnableDisable_Uninstall(t *testing.T) {
	pluginDir := t.TempDir()
	configDir := t.TempDir()
	cacheDir := t.TempDir()
	manifestPath := filepath.Join(pluginDir, ManifestFile)
	binaryPath := filepath.Join(pluginDir, "plugin-bin")

	if err := os.WriteFile(binaryPath, []byte("bin"), 0o755); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	writeManifest(t, manifestPath, Manifest{
		Version: "1.0",
		Installed: []InstalledPlugin{
			{Name: "alpha", Version: "1.0.0", BinaryPath: binaryPath, Enabled: false},
		},
	})

	mgr := &Manager{
		registry:     NewRegistryService(configDir, cacheDir),
		installer:    NewInstaller(pluginDir),
		pluginDir:    pluginDir,
		cacheDir:     cacheDir,
		manifestPath: manifestPath,
	}

	if err := mgr.Enable(context.Background(), "alpha"); err != nil {
		t.Fatalf("Enable error: %v", err)
	}
	if err := mgr.Disable(context.Background(), "alpha"); err != nil {
		t.Fatalf("Disable error: %v", err)
	}

	if err := mgr.Uninstall(context.Background(), "alpha"); err != nil {
		t.Fatalf("Uninstall error: %v", err)
	}
	if _, err := os.Stat(binaryPath); !os.IsNotExist(err) {
		t.Fatalf("expected binary removed, got err=%v", err)
	}
}

func TestManager_Install_SDKIncompatible(t *testing.T) {
	pluginDir := t.TempDir()
	configDir := t.TempDir()
	cacheDir := t.TempDir()

	writeRegistryConfig(t, configDir, []RegistryEntry{
		{Name: OfficialRegistryName, URL: OfficialRegistryURL, Priority: 1000, Enabled: false},
		{Name: "local", URL: "https://example.com/registry.yaml", Priority: 10, Enabled: true},
	})
	writeRegistryCache(t, cacheDir, "local", Registry{
		Version: "1.0",
		Plugins: []PluginInfo{
			{Name: "alpha", Version: "1.0.0", MinSDKVersion: CurrentSDKVersion + 1},
		},
		UpdatedAt: time.Now(),
	})

	mgr := &Manager{
		registry:     NewRegistryService(configDir, cacheDir),
		installer:    NewInstaller(pluginDir),
		pluginDir:    pluginDir,
		cacheDir:     cacheDir,
		manifestPath: filepath.Join(pluginDir, ManifestFile),
	}

	if err := mgr.Install(context.Background(), "alpha"); err == nil {
		t.Fatal("expected Install to fail for incompatible SDK")
	}
}

func TestManager_Install_Success(t *testing.T) {
	pluginDir := t.TempDir()
	configDir := t.TempDir()
	cacheDir := t.TempDir()

	writeRegistryConfig(t, configDir, []RegistryEntry{
		{Name: OfficialRegistryName, URL: OfficialRegistryURL, Priority: 1000, Enabled: false},
		{Name: "local", URL: "https://example.com/registry.yaml", Priority: 10, Enabled: true},
	})
	writeRegistryCache(t, cacheDir, "local", Registry{
		Version: "1.0",
		Plugins: []PluginInfo{
			{Name: "alpha", Version: "v1.0.0", Repository: "relicta-tech/relicta"},
		},
		UpdatedAt: time.Now(),
	})

	installer := NewInstaller(pluginDir)
	archiveData := createTarGzBytesForTest(t, installer.getBinaryName("alpha"), []byte("binary content"))
	installer.httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(archiveData)),
				Header:     http.Header{"Content-Type": []string{"application/octet-stream"}},
			}, nil
		}),
	}

	mgr := &Manager{
		registry:     NewRegistryService(configDir, cacheDir),
		installer:    installer,
		pluginDir:    pluginDir,
		cacheDir:     cacheDir,
		manifestPath: filepath.Join(pluginDir, ManifestFile),
	}

	if err := mgr.Install(context.Background(), "alpha"); err != nil {
		t.Fatalf("Install error: %v", err)
	}

	manifest, err := mgr.loadManifest()
	if err != nil {
		t.Fatalf("loadManifest error: %v", err)
	}
	if len(manifest.Installed) != 1 {
		t.Fatalf("manifest installed len = %d, want 1", len(manifest.Installed))
	}
}

func TestManager_RegistryActions(t *testing.T) {
	configDir := t.TempDir()
	cacheDir := t.TempDir()

	writeRegistryConfig(t, configDir, []RegistryEntry{
		{Name: OfficialRegistryName, URL: OfficialRegistryURL, Priority: 1000, Enabled: false},
	})

	mgr := &Manager{
		registry:  NewRegistryService(configDir, cacheDir),
		pluginDir: t.TempDir(),
		cacheDir:  cacheDir,
	}

	if len(mgr.ListRegistries()) == 0 {
		t.Fatal("expected ListRegistries to return entries")
	}
	if err := mgr.AddRegistry("custom", "https://example.com/registry.yaml", 10); err != nil {
		t.Fatalf("AddRegistry error: %v", err)
	}
	if err := mgr.EnableRegistry("custom", false); err != nil {
		t.Fatalf("EnableRegistry error: %v", err)
	}
	if err := mgr.RemoveRegistry("custom"); err != nil {
		t.Fatalf("RemoveRegistry error: %v", err)
	}
}

func TestManager_Update_NoUpdateAvailable(t *testing.T) {
	pluginDir := t.TempDir()
	configDir := t.TempDir()
	cacheDir := t.TempDir()

	writeRegistryConfig(t, configDir, []RegistryEntry{
		{Name: OfficialRegistryName, URL: OfficialRegistryURL, Priority: 1000, Enabled: false},
		{Name: "local", URL: "https://example.com/registry.yaml", Priority: 10, Enabled: true},
	})
	writeRegistryCache(t, cacheDir, "local", Registry{
		Version:   "1.0",
		Plugins:   []PluginInfo{{Name: "alpha", Version: "1.0.0"}},
		UpdatedAt: time.Now(),
	})

	writeManifest(t, filepath.Join(pluginDir, ManifestFile), Manifest{
		Version: "1.0",
		Installed: []InstalledPlugin{
			{Name: "alpha", Version: "1.0.0", Enabled: true},
		},
	})

	mgr := &Manager{
		registry:     NewRegistryService(configDir, cacheDir),
		installer:    NewInstaller(pluginDir),
		pluginDir:    pluginDir,
		cacheDir:     cacheDir,
		manifestPath: filepath.Join(pluginDir, ManifestFile),
	}

	result, err := mgr.Update(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if result == nil || result.Updated {
		t.Fatalf("expected Updated=false, got %+v", result)
	}
}

func TestManager_Update_Success(t *testing.T) {
	pluginDir := t.TempDir()
	configDir := t.TempDir()
	cacheDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`version: "1.0"
plugins:
  - name: alpha
    version: "v1.1.0"
    repository: relicta-tech/relicta
updated_at: "` + time.Now().Format(time.RFC3339Nano) + `"`))
	}))
	t.Cleanup(server.Close)

	writeRegistryConfig(t, configDir, []RegistryEntry{
		{Name: OfficialRegistryName, URL: OfficialRegistryURL, Priority: 1000, Enabled: false},
		{Name: "remote", URL: server.URL, Priority: 10, Enabled: true},
	})

	binaryPath := filepath.Join(pluginDir, "alpha")
	if err := os.WriteFile(binaryPath, []byte("old"), 0o755); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	writeManifest(t, filepath.Join(pluginDir, ManifestFile), Manifest{
		Version: "1.0",
		Installed: []InstalledPlugin{
			{Name: "alpha", Version: "v1.0.0", BinaryPath: binaryPath, Enabled: true},
		},
	})

	installer := NewInstaller(pluginDir)
	archiveData := createTarGzBytesForTest(t, installer.getBinaryName("alpha"), []byte("binary content"))
	installer.httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(archiveData)),
				Header:     http.Header{"Content-Type": []string{"application/octet-stream"}},
			}, nil
		}),
	}

	mgr := &Manager{
		registry:     NewRegistryService(configDir, cacheDir),
		installer:    installer,
		pluginDir:    pluginDir,
		cacheDir:     cacheDir,
		manifestPath: filepath.Join(pluginDir, ManifestFile),
	}

	result, err := mgr.Update(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if result == nil || !result.Updated {
		t.Fatalf("expected Updated=true, got %+v", result)
	}
}

func TestManager_UpdateAll_HandlesErrors(t *testing.T) {
	pluginDir := t.TempDir()
	configDir := t.TempDir()
	cacheDir := t.TempDir()

	writeRegistryConfig(t, configDir, []RegistryEntry{
		{Name: OfficialRegistryName, URL: OfficialRegistryURL, Priority: 1000, Enabled: false},
		{Name: "local", URL: "https://example.com/registry.yaml", Priority: 10, Enabled: true},
	})
	writeRegistryCache(t, cacheDir, "local", Registry{
		Version:   "1.0",
		Plugins:   []PluginInfo{{Name: "alpha", Version: "1.0.0"}},
		UpdatedAt: time.Now(),
	})

	writeManifest(t, filepath.Join(pluginDir, ManifestFile), Manifest{
		Version: "1.0",
		Installed: []InstalledPlugin{
			{Name: "alpha", Version: "1.0.0"},
			{Name: "missing", Version: "0.1.0"},
		},
	})

	mgr := &Manager{
		registry:     NewRegistryService(configDir, cacheDir),
		installer:    NewInstaller(pluginDir),
		pluginDir:    pluginDir,
		cacheDir:     cacheDir,
		manifestPath: filepath.Join(pluginDir, ManifestFile),
	}

	results, err := mgr.UpdateAll(context.Background())
	if err != nil {
		t.Fatalf("UpdateAll error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results len = %d, want 2", len(results))
	}

	for _, result := range results {
		if result.Name == "missing" && result.Error == "" {
			t.Fatalf("expected error for missing plugin, got %+v", result)
		}
	}
}

func TestManager_Search(t *testing.T) {
	pluginDir := t.TempDir()
	configDir := t.TempDir()
	cacheDir := t.TempDir()

	writeRegistryConfig(t, configDir, []RegistryEntry{
		{Name: OfficialRegistryName, URL: OfficialRegistryURL, Priority: 1000, Enabled: false},
		{Name: "local", URL: "https://example.com/registry.yaml", Priority: 10, Enabled: true},
	})
	writeRegistryCache(t, cacheDir, "local", Registry{
		Version: "1.0",
		Plugins: []PluginInfo{
			{Name: "alpha", Version: "1.0.0", Description: "Alpha plugin", Category: "vcs"},
			{Name: "beta", Version: "2.0.0", Description: "Beta plugin", Category: "notify"},
		},
		UpdatedAt: time.Now(),
	})

	mgr := &Manager{
		registry:     NewRegistryService(configDir, cacheDir),
		installer:    NewInstaller(pluginDir),
		pluginDir:    pluginDir,
		cacheDir:     cacheDir,
		manifestPath: filepath.Join(pluginDir, ManifestFile),
	}

	results, err := mgr.Search(context.Background(), "notify")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) != 1 || results[0].Name != "beta" {
		t.Fatalf("Search results = %#v, want beta", results)
	}
}

func createTarGzBytesForTest(t *testing.T, filename string, content []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	header := &tar.Header{
		Name: filename,
		Mode: 0o755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("Failed to write tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("Failed to write tar content: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("Failed to close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("Failed to close gzip writer: %v", err)
	}

	return buf.Bytes()
}

func writeManifest(t *testing.T, path string, manifest Manifest) {
	t.Helper()

	data, err := yamlMarshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest error: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write manifest error: %v", err)
	}
}
