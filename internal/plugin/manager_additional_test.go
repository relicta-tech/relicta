// Package plugin provides tests for plugin management.
package plugin

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/relicta-tech/relicta/internal/config"
	pmgr "github.com/relicta-tech/relicta/internal/plugin/manager"
	"github.com/relicta-tech/relicta/pkg/plugin"
)

func TestManager_RegisterPlugins_TracksEnabled(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "enabled", Enabled: boolPtr(true)},
			{Name: "disabled", Enabled: boolPtr(false)},
		},
	}

	m := NewManager(cfg)
	m.RegisterPlugins()

	if _, ok := m.pendingPlugins["enabled"]; !ok {
		t.Fatal("enabled plugin missing from pendingPlugins")
	}
	if _, ok := m.pendingPlugins["disabled"]; ok {
		t.Fatal("disabled plugin should not be registered")
	}
	if _, ok := m.loadOnce["enabled"]; !ok {
		t.Fatal("loadOnce not set for enabled plugin")
	}
}

func TestManager_EnsurePluginLoaded_Errors(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	if _, err := m.ensurePluginLoaded(context.Background(), "missing"); err == nil {
		t.Fatal("expected error for unregistered plugin")
	}

	m.loadErrors["broken"] = fmt.Errorf("failed to load")
	if _, err := m.ensurePluginLoaded(context.Background(), "broken"); err == nil {
		t.Fatal("expected stored error for broken plugin")
	}
}

func TestManager_EnsurePluginLoaded_AlreadyLoaded(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	m.plugins["loaded"] = &loadedPlugin{name: "loaded"}
	lp, err := m.ensurePluginLoaded(context.Background(), "loaded")
	if err != nil {
		t.Fatalf("ensurePluginLoaded error: %v", err)
	}
	if lp == nil || lp.name != "loaded" {
		t.Fatalf("expected loaded plugin, got %+v", lp)
	}
}

func TestManager_ConfigHasHook(t *testing.T) {
	cfg := &config.PluginConfig{
		Name:  "test",
		Hooks: []string{string(plugin.HookPostPublish)},
	}

	m := NewManager(&config.Config{})
	if !m.configHasHook(cfg, plugin.HookPostPublish) {
		t.Fatal("expected configHasHook to match")
	}
	if m.configHasHook(cfg, plugin.HookOnError) {
		t.Fatal("expected configHasHook to return false for missing hook")
	}
}

func TestManager_VerifyPluginBinary(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	manifestPath := filepath.Join(tmpDir, ".relicta", "plugins", pmgr.ManifestFile)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	binaryPath := filepath.Join(tmpDir, "plugin-bin")
	if err := os.WriteFile(binaryPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	checksum := sha256.Sum256([]byte("binary"))
	manifest := pmgr.Manifest{
		Version: "1.0",
		Installed: []pmgr.InstalledPlugin{
			{Name: "alpha", Checksum: fmt.Sprintf("%x", checksum[:])},
		},
	}
	writeManifestYAML(t, manifestPath, manifest)

	m := NewManager(&config.Config{})
	if err := m.verifyPluginBinary("alpha", binaryPath); err != nil {
		t.Fatalf("verifyPluginBinary error: %v", err)
	}

	if err := m.verifyPluginBinary("missing", binaryPath); err != nil {
		t.Fatalf("verifyPluginBinary should skip missing plugin: %v", err)
	}
}

func TestManager_VerifyPluginBinary_Mismatch(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	manifestPath := filepath.Join(tmpDir, ".relicta", "plugins", pmgr.ManifestFile)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	binaryPath := filepath.Join(tmpDir, "plugin-bin")
	if err := os.WriteFile(binaryPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	manifest := pmgr.Manifest{
		Version: "1.0",
		Installed: []pmgr.InstalledPlugin{
			{Name: "alpha", Checksum: strings.Repeat("0", 64)},
		},
	}
	writeManifestYAML(t, manifestPath, manifest)

	m := NewManager(&config.Config{})
	if err := m.verifyPluginBinary("alpha", binaryPath); err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestManager_VerifyPluginBinary_NoManifest(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	binaryPath := filepath.Join(tmpDir, "plugin-bin")
	if err := os.WriteFile(binaryPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	m := NewManager(&config.Config{})
	if err := m.verifyPluginBinary("alpha", binaryPath); err != nil {
		t.Fatalf("verifyPluginBinary error: %v", err)
	}
}

func TestManager_VerifyPluginBinary_NoChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	manifestPath := filepath.Join(tmpDir, ".relicta", "plugins", pmgr.ManifestFile)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	binaryPath := filepath.Join(tmpDir, "plugin-bin")
	if err := os.WriteFile(binaryPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	manifest := pmgr.Manifest{
		Version: "1.0",
		Installed: []pmgr.InstalledPlugin{
			{Name: "alpha", Checksum: ""},
		},
	}
	writeManifestYAML(t, manifestPath, manifest)

	m := NewManager(&config.Config{})
	if err := m.verifyPluginBinary("alpha", binaryPath); err != nil {
		t.Fatalf("verifyPluginBinary error: %v", err)
	}
}

func TestManager_EnsurePluginLoaded_UsesLoadOnce(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{{Name: "alpha", Enabled: boolPtr(true)}},
	}
	m := NewManager(cfg)
	m.pendingPlugins["alpha"] = &cfg.Plugins[0]
	m.loadOnce["alpha"] = &sync.Once{}

	// Inject a load error to avoid executing real plugins.
	m.loadErrors["alpha"] = fmt.Errorf("load failed")
	if _, err := m.ensurePluginLoaded(context.Background(), "alpha"); err == nil {
		t.Fatal("expected load error for alpha")
	}
}

func TestManager_EnsurePluginLoaded_LoadOnceErrorStored(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	badCfg := &config.PluginConfig{Name: "bad/name", Enabled: boolPtr(true)}
	m.pendingPlugins["bad"] = badCfg
	m.loadOnce["bad"] = &sync.Once{}

	if _, err := m.ensurePluginLoaded(context.Background(), "bad"); err == nil {
		t.Fatal("expected error for bad plugin")
	}
	if _, ok := m.loadErrors["bad"]; !ok {
		t.Fatal("expected loadErrors to store error")
	}
}

func TestManager_LoadPlugin_InvalidName(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	err := m.loadPlugin(context.Background(), &config.PluginConfig{Name: "bad/name"})
	if err == nil {
		t.Fatal("expected error for invalid plugin name")
	}
}

func TestManager_LoadPlugin_VerifyFailure(t *testing.T) {
	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	pluginDir := filepath.Join(workDir, ".relicta", "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(workDir, ".relicta")) })

	binaryPath := filepath.Join(pluginDir, "alpha")
	if err := os.WriteFile(binaryPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	manifestPath := filepath.Join(pluginDir, pmgr.ManifestFile)
	manifest := pmgr.Manifest{
		Version: "1.0",
		Installed: []pmgr.InstalledPlugin{
			{Name: "alpha", Checksum: strings.Repeat("0", 64)},
		},
	}
	writeManifestYAML(t, manifestPath, manifest)

	cfg := &config.Config{}
	m := NewManager(cfg)

	err = m.loadPlugin(context.Background(), &config.PluginConfig{Name: "alpha", Path: binaryPath})
	if err == nil {
		t.Fatal("expected loadPlugin to fail on checksum mismatch")
	}
}

func TestManager_LoadPlugin_InvalidHandshake(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test not supported on Windows")
	}

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	pluginDir := filepath.Join(workDir, ".relicta", "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(workDir, ".relicta")) })

	scriptPath := filepath.Join(pluginDir, "alpha")
	script := []byte("#!/bin/sh\nexit 1\n")
	if err := os.WriteFile(scriptPath, script, 0o755); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	checksum := sha256.Sum256(script)
	manifest := pmgr.Manifest{
		Version: "1.0",
		Installed: []pmgr.InstalledPlugin{
			{Name: "alpha", Checksum: fmt.Sprintf("%x", checksum[:])},
		},
	}
	writeManifestYAML(t, filepath.Join(pluginDir, pmgr.ManifestFile), manifest)

	m := NewManager(&config.Config{})
	err = m.loadPlugin(context.Background(), &config.PluginConfig{Name: "alpha", Path: scriptPath})
	if err == nil {
		t.Fatal("expected loadPlugin to fail for invalid plugin handshake")
	}
}

func TestManager_LoadPlugin_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("plugin build test not supported on Windows")
	}
	if testing.Short() {
		t.Skip("skipping plugin process test in short mode")
	}

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	pluginDir := filepath.Join(workDir, ".relicta", "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(workDir, ".relicta")) })

	binaryPath := filepath.Join(pluginDir, "alpha")
	buildTestPluginBinary(t, binaryPath, "alpha")

	binaryBytes, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	checksum := sha256.Sum256(binaryBytes)
	manifest := pmgr.Manifest{
		Version: "1.0",
		Installed: []pmgr.InstalledPlugin{
			{Name: "alpha", Checksum: fmt.Sprintf("%x", checksum[:])},
		},
	}
	writeManifestYAML(t, filepath.Join(pluginDir, pmgr.ManifestFile), manifest)

	m := NewManager(&config.Config{})
	t.Cleanup(func() { _ = m.Close() })

	err = m.loadPlugin(context.Background(), &config.PluginConfig{
		Name:   "alpha",
		Path:   binaryPath,
		Config: map[string]any{"mode": "ok"},
	})
	if err != nil {
		t.Fatalf("loadPlugin error: %v", err)
	}

	m.mu.RLock()
	lp := m.plugins["alpha"]
	m.mu.RUnlock()
	if lp == nil {
		t.Fatal("expected loaded plugin to be stored")
	}
	if lp.info.Name != "alpha" {
		t.Fatalf("expected info name alpha, got %s", lp.info.Name)
	}
	if lp.timeout != 30*time.Second {
		t.Fatalf("expected default timeout, got %s", lp.timeout)
	}
}

func TestManager_LoadPlugin_ValidateError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("plugin build test not supported on Windows")
	}
	if testing.Short() {
		t.Skip("skipping plugin process test in short mode")
	}

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	pluginDir := filepath.Join(workDir, ".relicta", "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(workDir, ".relicta")) })

	binaryPath := filepath.Join(pluginDir, "beta")
	buildTestPluginBinary(t, binaryPath, "beta")

	binaryBytes, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	checksum := sha256.Sum256(binaryBytes)
	manifest := pmgr.Manifest{
		Version: "1.0",
		Installed: []pmgr.InstalledPlugin{
			{Name: "beta", Checksum: fmt.Sprintf("%x", checksum[:])},
		},
	}
	writeManifestYAML(t, filepath.Join(pluginDir, pmgr.ManifestFile), manifest)

	m := NewManager(&config.Config{})
	t.Cleanup(func() { _ = m.Close() })

	err = m.loadPlugin(context.Background(), &config.PluginConfig{
		Name:   "beta",
		Path:   binaryPath,
		Config: map[string]any{"error": true},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid plugin configuration") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestManager_LoadPlugin_InvalidConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("plugin build test not supported on Windows")
	}
	if testing.Short() {
		t.Skip("skipping plugin process test in short mode")
	}

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	pluginDir := filepath.Join(workDir, ".relicta", "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(workDir, ".relicta")) })

	binaryPath := filepath.Join(pluginDir, "gamma")
	buildTestPluginBinary(t, binaryPath, "gamma")

	binaryBytes, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	checksum := sha256.Sum256(binaryBytes)
	manifest := pmgr.Manifest{
		Version: "1.0",
		Installed: []pmgr.InstalledPlugin{
			{Name: "gamma", Checksum: fmt.Sprintf("%x", checksum[:])},
		},
	}
	writeManifestYAML(t, filepath.Join(pluginDir, pmgr.ManifestFile), manifest)

	m := NewManager(&config.Config{})
	t.Cleanup(func() { _ = m.Close() })

	err = m.loadPlugin(context.Background(), &config.PluginConfig{
		Name:   "gamma",
		Path:   binaryPath,
		Config: map[string]any{"invalid": true},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid plugin configuration") {
		t.Fatalf("expected invalid config error, got %v", err)
	}
}

func TestManager_CollectPluginsForHook(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "loaded", Enabled: boolPtr(true)},
			{Name: "skip", Enabled: boolPtr(true), Hooks: []string{"pre-plan"}},
			{Name: "lazy", Enabled: boolPtr(true)},
		},
	}

	m := NewManager(cfg)
	m.plugins["loaded"] = &loadedPlugin{
		name:   "loaded",
		plugin: stubPlugin{},
		info: plugin.Info{
			Name:  "loaded",
			Hooks: []plugin.Hook{plugin.HookPostPublish},
		},
		timeout: DefaultPerPluginTimeout,
	}

	m.pendingPlugins["skip"] = &cfg.Plugins[1]
	m.pendingPlugins["lazy"] = &cfg.Plugins[2]
	m.loadOnce["lazy"] = &sync.Once{}
	m.loadErrors["lazy"] = fmt.Errorf("load failed")

	execList := m.collectPluginsForHook(plugin.HookPostPublish)
	if len(execList) != 1 {
		t.Fatalf("execList len = %d, want 1", len(execList))
	}
	if execList[0].name != "loaded" {
		t.Fatalf("execList name = %s, want loaded", execList[0].name)
	}
}

type stubPlugin struct{}

func (stubPlugin) GetInfo() plugin.Info { return plugin.Info{Name: "stub"} }

func (stubPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	return &plugin.ExecuteResponse{Success: true}, nil
}

func (stubPlugin) Validate(ctx context.Context, cfg map[string]any) (*plugin.ValidateResponse, error) {
	return &plugin.ValidateResponse{Valid: true}, nil
}

func writeManifestYAML(t *testing.T, path string, manifest pmgr.Manifest) {
	t.Helper()

	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("yaml marshal error: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
}

func buildTestPluginBinary(t *testing.T, outputPath, name string) {
	t.Helper()

	repoRoot := findRepoRoot(t)
	buildDir, err := os.MkdirTemp(repoRoot, "tmp-plugin-*")
	if err != nil {
		t.Fatalf("MkdirTemp error: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(buildDir) })

	source := fmt.Sprintf(`package main

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

type testPlugin struct{}

func (testPlugin) GetInfo() plugin.Info {
	return plugin.Info{
		Name:    %q,
		Version: "1.0.0",
		Hooks:   []plugin.Hook{plugin.HookPostPublish},
	}
}

func (testPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	return &plugin.ExecuteResponse{Success: true, Message: "ok"}, nil
}

func (testPlugin) Validate(ctx context.Context, cfg map[string]any) (*plugin.ValidateResponse, error) {
	if cfg["error"] == true {
		return nil, fmt.Errorf("validation error")
	}
	if cfg["invalid"] == true {
		return &plugin.ValidateResponse{
			Valid: false,
			Errors: []plugin.ValidationError{
				{Field: "mode", Message: "invalid"},
			},
		}, nil
	}
	return &plugin.ValidateResponse{Valid: true}, nil
}

func main() {
	plugin.Serve(testPlugin{})
}
`, name)

	if err := os.WriteFile(filepath.Join(buildDir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	relDir, err := filepath.Rel(repoRoot, buildDir)
	if err != nil {
		t.Fatalf("Rel error: %v", err)
	}

	cmd := exec.Command("go", "build", "-o", outputPath, "./"+filepath.ToSlash(relDir))
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build error: %v\n%s", err, output)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found in parent dirs")
		}
		dir = parent
	}
}
