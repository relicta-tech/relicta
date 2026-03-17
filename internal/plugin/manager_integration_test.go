//go:build integration

// Package plugin provides integration tests for plugin management.
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
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/relicta-tech/relicta/internal/config"
	pmgr "github.com/relicta-tech/relicta/internal/plugin/manager"
)

func TestManager_LoadPlugin_Success_Integration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("plugin build test not supported on Windows")
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
	buildTestPluginBinaryIntegration(t, binaryPath, "alpha")

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
	writeManifestYAMLIntegration(t, filepath.Join(pluginDir, pmgr.ManifestFile), manifest)

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

func TestManager_LoadPlugin_ValidateError_Integration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("plugin build test not supported on Windows")
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
	buildTestPluginBinaryIntegration(t, binaryPath, "beta")

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
	writeManifestYAMLIntegration(t, filepath.Join(pluginDir, pmgr.ManifestFile), manifest)

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

func TestManager_LoadPlugin_InvalidConfig_Integration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("plugin build test not supported on Windows")
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
	buildTestPluginBinaryIntegration(t, binaryPath, "gamma")

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
	writeManifestYAMLIntegration(t, filepath.Join(pluginDir, pmgr.ManifestFile), manifest)

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

func writeManifestYAMLIntegration(t *testing.T, path string, manifest pmgr.Manifest) {
	t.Helper()

	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("yaml marshal error: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
}

func buildTestPluginBinaryIntegration(t *testing.T, outputPath, name string) {
	t.Helper()

	repoRoot := findRepoRootIntegration(t)
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

func findRepoRootIntegration(t *testing.T) string {
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
