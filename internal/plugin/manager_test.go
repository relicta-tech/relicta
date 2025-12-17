// Package plugin provides tests for plugin management.
package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/pkg/plugin"
)

func TestNewManager(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{Name: "test1"},
			{Name: "test2"},
		},
	}

	m := NewManager(cfg)

	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.plugins == nil {
		t.Error("plugins map should be initialized")
	}
	if m.logger == nil {
		t.Error("logger should be initialized")
	}
}

func TestManager_ListPlugins_Empty(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	plugins := m.ListPlugins()
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestValidatePluginName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid lowercase", "myplugin", false},
		{"valid with hyphen", "my-plugin", false},
		{"valid with underscore", "my_plugin", false},
		{"valid with numbers", "plugin123", false},
		{"valid mixed", "My-Plugin_123", false},
		{"empty name", "", true},
		{"too long", string(make([]byte, 65)), true},
		{"invalid char dot", "my.plugin", true},
		{"invalid char slash", "my/plugin", true},
		{"invalid char space", "my plugin", true},
		{"invalid char colon", "my:plugin", true},
		{"path traversal attempt", "../etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePluginName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePluginName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestCollectPluginsForHook(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Manually add some loaded plugins for testing
	m.mu.Lock()
	m.plugins["plugin1"] = &loadedPlugin{
		name:    "plugin1",
		timeout: 30 * time.Second,
		info: plugin.Info{
			Name:  "plugin1",
			Hooks: []plugin.Hook{plugin.HookPostPublish, plugin.HookOnSuccess},
		},
	}
	m.plugins["plugin2"] = &loadedPlugin{
		name:    "plugin2",
		timeout: 30 * time.Second,
		info: plugin.Info{
			Name:  "plugin2",
			Hooks: []plugin.Hook{plugin.HookOnError},
		},
	}
	m.plugins["plugin3"] = &loadedPlugin{
		name:    "plugin3",
		timeout: 30 * time.Second,
		info: plugin.Info{
			Name:  "plugin3",
			Hooks: []plugin.Hook{plugin.HookPostPublish},
		},
	}
	m.mu.Unlock()

	tests := []struct {
		name      string
		hook      plugin.Hook
		wantCount int
	}{
		{"PostPublish hook", plugin.HookPostPublish, 2},
		{"OnSuccess hook", plugin.HookOnSuccess, 1},
		{"OnError hook", plugin.HookOnError, 1},
		{"PreInit hook (none)", plugin.HookPreInit, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collected := m.collectPluginsForHook(tt.hook)
			if len(collected) != tt.wantCount {
				t.Errorf("collectPluginsForHook(%v) = %d plugins, want %d", tt.hook, len(collected), tt.wantCount)
			}
		})
	}
}

func TestCollectPluginsForHook_ConcurrentAccess(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Add a plugin
	m.mu.Lock()
	m.plugins["concurrent-test"] = &loadedPlugin{
		name:    "concurrent-test",
		timeout: 30 * time.Second,
		info: plugin.Info{
			Name:  "concurrent-test",
			Hooks: []plugin.Hook{plugin.HookPostPublish},
		},
	}
	m.mu.Unlock()

	// Test concurrent access doesn't cause race conditions
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.collectPluginsForHook(plugin.HookPostPublish)
		}()
	}

	// Also do some writes concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = m.ListPlugins()
		}(i)
	}

	wg.Wait()
}

func TestPluginSupportsHook(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	lp := &loadedPlugin{
		name: "test",
		info: plugin.Info{
			Hooks: []plugin.Hook{plugin.HookPostPublish, plugin.HookOnSuccess},
		},
	}

	tests := []struct {
		hook     plugin.Hook
		expected bool
	}{
		{plugin.HookPostPublish, true},
		{plugin.HookOnSuccess, true},
		{plugin.HookOnError, false},
		{plugin.HookPreInit, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.hook), func(t *testing.T) {
			result := m.pluginSupportsHook(lp, tt.hook)
			if result != tt.expected {
				t.Errorf("pluginSupportsHook(%v) = %v, want %v", tt.hook, result, tt.expected)
			}
		})
	}
}

func TestManager_Shutdown(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Add a mock plugin (without actual client)
	m.mu.Lock()
	m.plugins["test"] = &loadedPlugin{
		name: "test",
		info: plugin.Info{Name: "test"},
		// Note: client is nil, which is fine for this test
	}
	m.mu.Unlock()

	// Shutdown should clear plugins
	m.Shutdown()

	m.mu.RLock()
	count := len(m.plugins)
	m.mu.RUnlock()

	if count != 0 {
		t.Errorf("expected 0 plugins after shutdown, got %d", count)
	}
}

func TestManager_Close(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	err := m.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestJoinErrors(t *testing.T) {
	tests := []struct {
		name   string
		errs   []string
		expect string
	}{
		{"empty", []string{}, ""},
		{"single", []string{"error1"}, "error1"},
		{"two", []string{"error1", "error2"}, "error1; error2"},
		{"three", []string{"a", "b", "c"}, "a; b; c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinErrors(tt.errs)
			if result != tt.expect {
				t.Errorf("joinErrors(%v) = %q, want %q", tt.errs, result, tt.expect)
			}
		})
	}
}

func TestManager_GetPluginInfo_NotFound(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	_, err := m.GetPluginInfo("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent plugin")
	}
}

func TestManager_GetPluginInfo_Found(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Add a mock plugin
	m.mu.Lock()
	m.plugins["testplugin"] = &loadedPlugin{
		name: "testplugin",
		info: plugin.Info{
			Name:    "testplugin",
			Version: "1.0.0",
			Hooks:   []plugin.Hook{plugin.HookPostPublish},
		},
	}
	m.mu.Unlock()

	info, err := m.GetPluginInfo("testplugin")
	if err != nil {
		t.Errorf("GetPluginInfo() error = %v", err)
	}
	if info.Name != "testplugin" {
		t.Errorf("Name = %v, want testplugin", info.Name)
	}
	if info.Version != "1.0.0" {
		t.Errorf("Version = %v, want 1.0.0", info.Version)
	}
}

func TestManager_ExecuteHook_NoPlugins(t *testing.T) {
	cfg := &config.Config{
		Workflow: config.WorkflowConfig{
			DryRunByDefault: false,
		},
	}
	m := NewManager(cfg)

	ctx := context.Background()
	results, err := m.ExecuteHook(ctx, plugin.HookPostPublish, plugin.ReleaseContext{})

	if err != nil {
		t.Errorf("ExecuteHook() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %v", results)
	}
}

func TestAllowedPluginDirs(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	dirs := m.allowedPluginDirs()
	if len(dirs) == 0 {
		t.Error("allowedPluginDirs() should return at least one directory")
	}

	// Check that standard directories are included
	foundLocal := false
	foundSystem := false
	for _, dir := range dirs {
		if dir == ".relicta/plugins" {
			foundLocal = true
		}
		if dir == "/usr/local/lib/relicta/plugins" {
			foundSystem = true
		}
	}

	if !foundLocal {
		t.Error("expected local plugin directory in allowed dirs")
	}
	if !foundSystem {
		t.Error("expected system plugin directory in allowed dirs")
	}
}

func TestIsPathInAllowedDir(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "path in local plugins dir",
			path:     ".relicta/plugins/test-plugin",
			expected: true,
		},
		{
			name:     "path outside allowed dirs",
			path:     "/tmp/malicious-plugin",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to absolute path
			absPath, err := filepath.Abs(tt.path)
			if err != nil {
				t.Skipf("Cannot resolve absolute path: %v", err)
			}
			result := m.isPathInAllowedDir(absPath)
			if result != tt.expected {
				t.Errorf("isPathInAllowedDir(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestValidatePluginBinary_NonExistent(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	err := m.validatePluginBinary("/nonexistent/path/to/plugin")
	if err == nil {
		t.Error("Expected error for non-existent plugin")
	}
}

func TestFindPluginBinary_InvalidName(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	invalidNames := []string{
		"../../../etc/passwd",
		"plugin/with/slashes",
		"plugin.with.dots",
		"",
	}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			pluginCfg := &config.PluginConfig{Name: name}
			_, err := m.findPluginBinary(pluginCfg)
			if err == nil {
				t.Errorf("Expected error for invalid plugin name: %q", name)
			}
		})
	}
}

func TestFindPluginBinary_NotFound(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	pluginCfg := &config.PluginConfig{Name: "nonexistent-plugin-12345"}
	_, err := m.findPluginBinary(pluginCfg)
	if err == nil {
		t.Error("Expected error for non-existent plugin")
	}
	expectedMsg := "plugin binary not found"
	if err != nil && !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error to contain %q, got %v", expectedMsg, err)
	}
}

func TestLoadPlugins_DisabledPlugin(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{
				Name:    "test",
				Enabled: boolPtr(false),
			},
		},
	}
	m := NewManager(cfg)

	err := m.LoadPlugins(context.Background())
	if err != nil {
		t.Errorf("LoadPlugins() should not error on disabled plugins: %v", err)
	}

	// Verify plugin was not loaded
	m.mu.RLock()
	count := len(m.plugins)
	m.mu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 plugins loaded, got %d", count)
	}
}

func TestLoadPlugins_EmptyConfig(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{},
	}
	m := NewManager(cfg)

	err := m.LoadPlugins(context.Background())
	if err != nil {
		t.Errorf("LoadPlugins() error = %v", err)
	}

	// Verify no plugins loaded
	m.mu.RLock()
	count := len(m.plugins)
	m.mu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 plugins loaded, got %d", count)
	}
}

func TestShutdown_WithNilClient(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Add a plugin with nil client
	m.mu.Lock()
	m.plugins["test-nil-client"] = &loadedPlugin{
		name:   "test-nil-client",
		client: nil, // Nil client
		info:   plugin.Info{Name: "test"},
	}
	m.mu.Unlock()

	// Should not panic
	m.Shutdown()

	m.mu.RLock()
	count := len(m.plugins)
	m.mu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 plugins after shutdown, got %d", count)
	}
}

func TestAllowedPluginDirs_EmptyHome(t *testing.T) {
	// Temporarily clear HOME env var
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", "")
	defer os.Setenv("HOME", oldHome)

	cfg := &config.Config{}
	m := NewManager(cfg)

	dirs := m.allowedPluginDirs()
	// Should still return dirs even with empty HOME (uses /tmp as fallback)
	if len(dirs) == 0 {
		t.Error("allowedPluginDirs() should return directories even with empty HOME")
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}

func TestValidatePluginBinary_Directory(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Create a directory in an allowed location
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set")
	}

	allowedDir := filepath.Join(homeDir, ".relicta", "plugins")
	os.MkdirAll(allowedDir, 0755)
	defer os.RemoveAll(filepath.Join(homeDir, ".relicta"))

	// Create a subdirectory
	testDir := filepath.Join(allowedDir, "test-dir")
	err := os.MkdirAll(testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}

	err = m.validatePluginBinary(testDir)
	if err == nil {
		t.Error("Expected error for directory path")
	}
	expectedMsg := "plugin path is a directory"
	if err != nil && !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error to contain %q, got %v", expectedMsg, err)
	}
}

func TestValidatePluginBinary_NotExecutable(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Create a file in an allowed location
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set")
	}

	allowedDir := filepath.Join(homeDir, ".relicta", "plugins")
	os.MkdirAll(allowedDir, 0755)
	defer os.RemoveAll(filepath.Join(homeDir, ".relicta"))

	// Create a non-executable file
	pluginPath := filepath.Join(allowedDir, "test-not-executable")
	err := os.WriteFile(pluginPath, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = m.validatePluginBinary(pluginPath)
	if err == nil {
		t.Error("Expected error for non-executable file")
	}
	expectedMsg := "not executable"
	if err != nil && !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error to contain %q, got %v", expectedMsg, err)
	}
}

func TestValidatePluginBinary_OutsideAllowedDir(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Create a temp file in /tmp (not an allowed dir)
	tmpFile, err := os.CreateTemp("", "plugin-outside-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Make it executable
	os.Chmod(tmpFile.Name(), 0755)

	err = m.validatePluginBinary(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for plugin outside allowed directory")
	}
	expectedMsg := "not in an allowed directory"
	if err != nil && !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error to contain %q, got %v", expectedMsg, err)
	}
}

func TestFindPluginBinary_WithSpecifiedPath(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Create temp dir in allowed location
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set")
	}

	allowedDir := filepath.Join(homeDir, ".relicta", "plugins")
	os.MkdirAll(allowedDir, 0755)
	defer os.RemoveAll(filepath.Join(homeDir, ".relicta"))

	// Create a mock plugin binary
	pluginPath := filepath.Join(allowedDir, "test-plugin")
	err := os.WriteFile(pluginPath, []byte("#!/bin/bash\necho test"), 0755)
	if err != nil {
		t.Fatalf("Failed to create plugin file: %v", err)
	}

	pluginCfg := &config.PluginConfig{
		Name: "test",
		Path: pluginPath,
	}

	foundPath, err := m.findPluginBinary(pluginCfg)
	if err != nil {
		t.Errorf("findPluginBinary() error = %v", err)
	}
	if foundPath == "" {
		t.Error("Expected non-empty path")
	}
}

func TestFindPluginBinary_SearchAllowedDirs(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Create temp dir in allowed location
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set")
	}

	allowedDir := filepath.Join(homeDir, ".relicta", "plugins")
	os.MkdirAll(allowedDir, 0755)
	defer os.RemoveAll(filepath.Join(homeDir, ".relicta"))

	// Create a mock plugin binary with expected naming
	pluginName := "searchtest"
	pluginPath := filepath.Join(allowedDir, pluginName)
	err := os.WriteFile(pluginPath, []byte("#!/bin/bash\necho test"), 0755)
	if err != nil {
		t.Fatalf("Failed to create plugin file: %v", err)
	}

	pluginCfg := &config.PluginConfig{Name: pluginName}

	foundPath, err := m.findPluginBinary(pluginCfg)
	if err != nil {
		t.Errorf("findPluginBinary() error = %v", err)
	}
	if foundPath == "" {
		t.Error("Expected non-empty path")
	}
}

func TestExecuteHook_WithContextCancellation(t *testing.T) {
	cfg := &config.Config{
		Workflow: config.WorkflowConfig{
			DryRunByDefault: false,
		},
	}
	m := NewManager(cfg)

	// Add a mock plugin
	m.mu.Lock()
	m.plugins["test"] = &loadedPlugin{
		name:    "test",
		timeout: 30 * time.Second,
		info: plugin.Info{
			Name:  "test",
			Hooks: []plugin.Hook{plugin.HookPostPublish},
		},
		// plugin field is nil, which will cause execution to fail
	}
	m.mu.Unlock()

	// Create a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// This should handle the cancellation gracefully
	_, err := m.ExecuteHook(ctx, plugin.HookPostPublish, plugin.ReleaseContext{})
	// Should not panic and may return error or empty results
	_ = err // We don't strictly require an error here
}

func TestLoadPlugin_NonExistentBinary(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	pluginCfg := &config.PluginConfig{
		Name: "nonexistent-plugin-xyz123",
	}

	err := m.loadPlugin(context.Background(), pluginCfg)
	if err == nil {
		t.Error("Expected error for non-existent plugin")
	}
}

func TestIsPathInAllowedDir_EdgeCases(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	tests := []struct {
		name     string
		setup    func() string
		expected bool
	}{
		{
			name: "absolute path outside allowed dirs",
			setup: func() string {
				return "/etc/passwd"
			},
			expected: false,
		},
		{
			name: "path with .. components",
			setup: func() string {
				return ".relicta/plugins/../../etc/passwd"
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			absPath, err := filepath.Abs(path)
			if err != nil {
				t.Skipf("Cannot resolve path: %v", err)
			}

			// Evaluate symlinks if possible
			realPath, err := filepath.EvalSymlinks(absPath)
			if err != nil {
				// If we can't resolve symlinks, use the absolute path
				realPath = absPath
			}

			result := m.isPathInAllowedDir(realPath)
			if result != tt.expected {
				t.Errorf("isPathInAllowedDir(%q) = %v, want %v", path, result, tt.expected)
			}
		})
	}
}

func TestValidatePluginName_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"exactly 64 chars", strings.Repeat("a", 64), false},
		{"65 chars", strings.Repeat("a", 65), true},
		{"starts with hyphen", "-plugin", false},
		{"ends with hyphen", "plugin-", false},
		{"starts with underscore", "_plugin", false},
		{"ends with underscore", "plugin_", false},
		{"only numbers", "123456", false},
		{"mixed case", "MyPlugin", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePluginName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePluginName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestManager_ExecuteHook_DryRun(t *testing.T) {
	cfg := &config.Config{
		Workflow: config.WorkflowConfig{
			DryRunByDefault: true, // Test dry run mode
		},
	}
	m := NewManager(cfg)

	// Test with no plugins - should return empty results
	ctx := context.Background()
	results, err := m.ExecuteHook(ctx, plugin.HookPostPublish, plugin.ReleaseContext{})

	if err != nil {
		t.Errorf("ExecuteHook() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %v", results)
	}
}

func TestJoinErrors_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		errs   []string
		expect string
	}{
		{"very long errors", []string{strings.Repeat("a", 100), strings.Repeat("b", 100)}, strings.Repeat("a", 100) + "; " + strings.Repeat("b", 100)},
		{"single char errors", []string{"a", "b", "c", "d"}, "a; b; c; d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinErrors(tt.errs)
			if result != tt.expect {
				t.Errorf("joinErrors() length = %d, want %d", len(result), len(tt.expect))
			}
		})
	}
}

func TestFindPluginBinary_InvalidPath(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Test with a path that's outside allowed directories
	pluginCfg := &config.PluginConfig{
		Name: "test",
		Path: "/tmp/evil-plugin",
	}

	_, err := m.findPluginBinary(pluginCfg)
	if err == nil {
		t.Error("Expected error for plugin path outside allowed directory")
	}
}

func TestLoadPlugins_ContinueOnError(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{
				Name:            "nonexistent-1",
				ContinueOnError: true,
			},
			{
				Name:            "nonexistent-2",
				ContinueOnError: true,
			},
		},
	}
	m := NewManager(cfg)

	// Should not fail even though plugins don't exist
	err := m.LoadPlugins(context.Background())
	if err != nil {
		t.Errorf("LoadPlugins() should not error with ContinueOnError: %v", err)
	}
}

func TestLoadPlugins_FailOnError(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{
				Name:            "nonexistent-plugin",
				ContinueOnError: false,
			},
		},
	}
	m := NewManager(cfg)

	// Should fail since plugin doesn't exist and ContinueOnError is false
	err := m.LoadPlugins(context.Background())
	if err == nil {
		t.Error("Expected error when plugin loading fails without ContinueOnError")
	}
}

func TestValidatePluginBinary_SymlinkResolution(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set")
	}

	allowedDir := filepath.Join(homeDir, ".relicta", "plugins")
	os.MkdirAll(allowedDir, 0755)
	defer os.RemoveAll(filepath.Join(homeDir, ".relicta"))

	// Create actual plugin file
	pluginPath := filepath.Join(allowedDir, "test-plugin-real")
	err := os.WriteFile(pluginPath, []byte("#!/bin/bash\necho test"), 0755)
	if err != nil {
		t.Fatalf("Failed to create plugin file: %v", err)
	}

	// Create symlink to it
	symlinkPath := filepath.Join(allowedDir, "test-plugin-link")
	err = os.Symlink(pluginPath, symlinkPath)
	if err != nil {
		t.Skipf("Cannot create symlink: %v", err)
	}

	// Should successfully validate the symlink
	err = m.validatePluginBinary(symlinkPath)
	if err != nil {
		t.Errorf("validatePluginBinary() failed for valid symlink: %v", err)
	}
}

func TestIsPathInAllowedDir_SymlinkEscape(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set")
	}

	allowedDir := filepath.Join(homeDir, ".relicta", "plugins")
	os.MkdirAll(allowedDir, 0755)
	defer os.RemoveAll(filepath.Join(homeDir, ".relicta"))

	// Try to create a symlink that points outside allowed dir
	outsidePath := "/tmp/evil-target"
	symlinkPath := filepath.Join(allowedDir, "evil-link")

	// Create the outside file first
	err := os.WriteFile(outsidePath, []byte("evil"), 0644)
	if err != nil {
		t.Skipf("Cannot create test file: %v", err)
	}
	defer os.Remove(outsidePath)

	err = os.Symlink(outsidePath, symlinkPath)
	if err != nil {
		t.Skipf("Cannot create symlink: %v", err)
	}

	// Resolve the symlink
	realPath, err := filepath.EvalSymlinks(symlinkPath)
	if err != nil {
		t.Skipf("Cannot evaluate symlink: %v", err)
	}

	// Should detect that the real path is outside allowed dirs
	result := m.isPathInAllowedDir(realPath)
	if result {
		t.Error("isPathInAllowedDir() should return false for symlink pointing outside allowed dirs")
	}
}

func TestLoadPlugin_PathResolutionError(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Test with invalid path characters that might cause resolution issues
	pluginCfg := &config.PluginConfig{
		Name: "test",
		Path: string([]byte{0, 1, 2}), // Invalid path
	}

	err := m.loadPlugin(context.Background(), pluginCfg)
	if err == nil {
		t.Error("Expected error for invalid path")
	}
}

func TestValidatePluginBinary_AbsolutePathError(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Test with a path that doesn't exist
	err := m.validatePluginBinary("/nonexistent/path/to/plugin/that/does/not/exist")
	if err == nil {
		t.Error("Expected error for non-existent path")
	}
}

func TestIsPathInAllowedDir_RelativePathError(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Test with current directory
	result := m.isPathInAllowedDir(".")
	// Result depends on whether current dir is in allowed dirs
	_ = result
}

func TestExecuteHook_EmptyHook(t *testing.T) {
	cfg := &config.Config{
		Workflow: config.WorkflowConfig{
			DryRunByDefault: false,
		},
	}
	m := NewManager(cfg)

	ctx := context.Background()
	// Test execution with a hook that no plugins support
	results, err := m.ExecuteHook(ctx, plugin.Hook("nonexistent-hook"), plugin.ReleaseContext{})

	if err != nil {
		t.Errorf("ExecuteHook() should not error for hook with no plugins: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestFindPluginBinary_AbsolutePathError(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set")
	}

	allowedDir := filepath.Join(homeDir, ".relicta", "plugins")
	os.MkdirAll(allowedDir, 0755)
	defer os.RemoveAll(filepath.Join(homeDir, ".relicta"))

	// Create a plugin with invalid symlink
	pluginPath := filepath.Join(allowedDir, "broken-link")
	err := os.Symlink("/nonexistent/target", pluginPath)
	if err != nil {
		t.Skipf("Cannot create symlink: %v", err)
	}

	pluginCfg := &config.PluginConfig{
		Name: "test",
		Path: pluginPath,
	}

	_, err = m.findPluginBinary(pluginCfg)
	if err == nil {
		t.Error("Expected error for broken symlink")
	}
}

func TestManager_NewManager_WithLargePluginCount(t *testing.T) {
	// Test that manager pre-allocates map with correct size
	cfg := &config.Config{
		Plugins: make([]config.PluginConfig, 100),
	}

	for i := 0; i < 100; i++ {
		cfg.Plugins[i] = config.PluginConfig{
			Name: fmt.Sprintf("plugin-%d", i),
		}
	}

	m := NewManager(cfg)
	if m == nil {
		t.Fatal("Expected manager to be created")
	}
	if m.plugins == nil {
		t.Error("Expected plugins map to be initialized")
	}
}

func TestShutdown_MultiplePlugins(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Add multiple plugins
	m.mu.Lock()
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("plugin-%d", i)
		m.plugins[name] = &loadedPlugin{
			name:   name,
			client: nil,
			info:   plugin.Info{Name: name},
		}
	}
	m.mu.Unlock()

	// Verify we have plugins
	m.mu.RLock()
	countBefore := len(m.plugins)
	m.mu.RUnlock()

	if countBefore != 10 {
		t.Errorf("Expected 10 plugins, got %d", countBefore)
	}

	// Shutdown should clear all
	m.Shutdown()

	m.mu.RLock()
	countAfter := len(m.plugins)
	m.mu.RUnlock()

	if countAfter != 0 {
		t.Errorf("Expected 0 plugins after shutdown, got %d", countAfter)
	}
}

func TestFindPluginBinary_SearchMultipleDirs(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set")
	}

	// Create plugin in the HOME directory allowed location
	allowedDir := filepath.Join(homeDir, ".relicta", "plugins")
	os.MkdirAll(allowedDir, 0755)
	defer os.RemoveAll(filepath.Join(homeDir, ".relicta"))

	pluginName := "searchtest2"
	pluginPath := filepath.Join(allowedDir, pluginName)
	err := os.WriteFile(pluginPath, []byte("#!/bin/bash\necho test"), 0755)
	if err != nil {
		t.Fatalf("Failed to create plugin file: %v", err)
	}

	pluginCfg := &config.PluginConfig{
		Name: pluginName,
	}

	foundPath, err := m.findPluginBinary(pluginCfg)
	if err != nil {
		t.Errorf("findPluginBinary() error = %v", err)
	}
	if foundPath == "" {
		t.Error("Expected non-empty path")
	}
	// Verify the found path exists
	if _, err := os.Stat(foundPath); err != nil {
		t.Errorf("Found path does not exist: %v", err)
	}
}

func TestIsPathInAllowedDir_MultipleAllowedDirs(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set")
	}

	// Test with path in user's home directory
	allowedDir := filepath.Join(homeDir, ".relicta", "plugins")
	os.MkdirAll(allowedDir, 0755)
	defer os.RemoveAll(filepath.Join(homeDir, ".relicta"))

	testPath := filepath.Join(allowedDir, "test-plugin")
	absPath, err := filepath.Abs(testPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	result := m.isPathInAllowedDir(absPath)
	if !result {
		t.Error("isPathInAllowedDir() should return true for path in allowed directory")
	}
}

func TestValidatePluginName_AllValidChars(t *testing.T) {
	// Test with all valid character types
	validNames := []string{
		"abcdefghijklmnopqrstuvwxyz",
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		"0123456789",
		"plugin-with-hyphens",
		"plugin_with_underscores",
		"Plugin_123-test",
	}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			err := validatePluginName(name)
			if err != nil {
				t.Errorf("validatePluginName(%q) error = %v, want nil", name, err)
			}
		})
	}
}

func TestLoadPlugins_MixedEnabledDisabled(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{
			{
				Name:    "enabled-plugin",
				Enabled: boolPtr(true),
			},
			{
				Name:    "disabled-plugin",
				Enabled: boolPtr(false),
			},
			{
				Name: "default-enabled", // Default is enabled
			},
		},
	}
	m := NewManager(cfg)

	// Should attempt to load enabled plugins (and fail since they don't exist)
	// but skip disabled ones
	err := m.LoadPlugins(context.Background())
	// Will error because the enabled plugins don't actually exist
	if err == nil {
		t.Error("Expected error for non-existent plugins")
	}
}

func TestValidatePluginBinary_SymlinkCircular(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set")
	}

	allowedDir := filepath.Join(homeDir, ".relicta", "plugins")
	os.MkdirAll(allowedDir, 0755)
	defer os.RemoveAll(filepath.Join(homeDir, ".relicta"))

	// Create circular symlinks (link1 -> link2 -> link1)
	link1 := filepath.Join(allowedDir, "link1")
	link2 := filepath.Join(allowedDir, "link2")

	// Create link1 pointing to link2
	err := os.Symlink(link2, link1)
	if err != nil {
		t.Skipf("Cannot create symlink: %v", err)
	}

	// Create link2 pointing to link1 (creates circular reference)
	err = os.Symlink(link1, link2)
	if err != nil {
		t.Skipf("Cannot create symlink: %v", err)
	}

	// Should handle circular symlink gracefully
	err = m.validatePluginBinary(link1)
	if err == nil {
		t.Error("Expected error for circular symlink")
	}
}

func TestIsPathInAllowedDir_LocalPluginDir(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Test with local project plugin directory
	localDir := ".relicta/plugins"
	os.MkdirAll(localDir, 0755)
	defer os.RemoveAll(".relicta")

	testPath := filepath.Join(localDir, "test-plugin")
	absPath, err := filepath.Abs(testPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	result := m.isPathInAllowedDir(absPath)
	if !result {
		t.Error("isPathInAllowedDir() should return true for local plugin directory")
	}
}

func TestFindPluginBinary_LocalProjectDir(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Create plugin in local project directory
	localDir := ".relicta/plugins"
	os.MkdirAll(localDir, 0755)
	defer os.RemoveAll(".relicta")

	pluginName := "localtest"
	pluginPath := filepath.Join(localDir, pluginName)
	err := os.WriteFile(pluginPath, []byte("#!/bin/bash\necho test"), 0755)
	if err != nil {
		t.Fatalf("Failed to create plugin file: %v", err)
	}

	pluginCfg := &config.PluginConfig{
		Name: pluginName,
	}

	foundPath, err := m.findPluginBinary(pluginCfg)
	if err != nil {
		t.Errorf("findPluginBinary() error = %v", err)
	}
	if foundPath == "" {
		t.Error("Expected non-empty path")
	}
}

func TestFindPluginBinary_PathWithSymlinkToAllowedDir(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set")
	}

	allowedDir := filepath.Join(homeDir, ".relicta", "plugins")
	os.MkdirAll(allowedDir, 0755)
	defer os.RemoveAll(filepath.Join(homeDir, ".relicta"))

	// Create actual plugin
	pluginPath := filepath.Join(allowedDir, "test-real-plugin")
	err := os.WriteFile(pluginPath, []byte("#!/bin/bash\necho test"), 0755)
	if err != nil {
		t.Fatalf("Failed to create plugin file: %v", err)
	}

	// Test with the real path
	pluginCfg := &config.PluginConfig{
		Name: "test",
		Path: pluginPath,
	}

	foundPath, err := m.findPluginBinary(pluginCfg)
	if err != nil {
		t.Errorf("findPluginBinary() error = %v", err)
	}
	if foundPath == "" {
		t.Error("Expected non-empty path")
	}
	// Verify it's an absolute path
	if !filepath.IsAbs(foundPath) {
		t.Error("Expected absolute path")
	}
}

func TestNewManager_ZeroPlugins(t *testing.T) {
	cfg := &config.Config{
		Plugins: []config.PluginConfig{},
	}
	m := NewManager(cfg)

	if m == nil {
		t.Fatal("Expected manager to be created")
	}
	// Should allocate with default capacity when no plugins configured
	if m.plugins == nil {
		t.Error("Expected plugins map to be initialized")
	}
}

func TestFindPluginBinary_SystemDir(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Test searching in system directories (will fail unless plugin exists)
	pluginCfg := &config.PluginConfig{
		Name: "nonexistent-system-plugin",
	}

	_, err := m.findPluginBinary(pluginCfg)
	// Should get "not found" error
	if err == nil {
		t.Error("Expected error for non-existent plugin")
	}
	expectedMsg := "plugin binary not found"
	if err != nil && !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error to contain %q, got %v", expectedMsg, err)
	}
}

func TestValidatePluginBinary_Success(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set")
	}

	allowedDir := filepath.Join(homeDir, ".relicta", "plugins")
	os.MkdirAll(allowedDir, 0755)
	defer os.RemoveAll(filepath.Join(homeDir, ".relicta"))

	// Create a valid executable plugin
	pluginPath := filepath.Join(allowedDir, "valid-plugin")
	err := os.WriteFile(pluginPath, []byte("#!/bin/bash\necho test"), 0755)
	if err != nil {
		t.Fatalf("Failed to create plugin file: %v", err)
	}

	// Should successfully validate
	err = m.validatePluginBinary(pluginPath)
	if err != nil {
		t.Errorf("validatePluginBinary() failed for valid plugin: %v", err)
	}
}

func TestFindPluginBinary_SkipInvalidDirs(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Create multiple directories, some with invalid plugins
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		t.Skip("HOME not set")
	}

	allowedDir1 := filepath.Join(homeDir, ".relicta", "plugins")
	os.MkdirAll(allowedDir1, 0755)
	defer os.RemoveAll(filepath.Join(homeDir, ".relicta"))

	// Create a non-executable plugin in first directory (should be skipped)
	pluginName := "skiptest"
	badPluginPath := filepath.Join(allowedDir1, pluginName)
	err := os.WriteFile(badPluginPath, []byte("bad"), 0644) // Not executable
	if err != nil {
		t.Fatalf("Failed to create bad plugin: %v", err)
	}

	pluginCfg := &config.PluginConfig{
		Name: pluginName,
	}

	// Should fail because plugin is not executable
	_, err = m.findPluginBinary(pluginCfg)
	if err == nil {
		t.Error("Expected error for non-executable plugin")
	}
}

func TestIsPathInAllowedDir_AbsolutePathErrors(t *testing.T) {
	cfg := &config.Config{}
	m := NewManager(cfg)

	// Test various paths that should fail
	testPaths := []string{
		"/etc/passwd",
		"/usr/bin/sh",
		"/var/log/test",
	}

	for _, path := range testPaths {
		t.Run(path, func(t *testing.T) {
			result := m.isPathInAllowedDir(path)
			if result {
				t.Errorf("isPathInAllowedDir(%q) should return false", path)
			}
		})
	}
}

// mockPlugin implements the plugin.Plugin interface for testing
type mockPlugin struct {
	executeFunc  func(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error)
	infoFunc     func() plugin.Info
	validateFunc func(ctx context.Context, config map[string]any) (*plugin.ValidateResponse, error)
}

func (m *mockPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, req)
	}
	return &plugin.ExecuteResponse{Success: true}, nil
}

func (m *mockPlugin) GetInfo() plugin.Info {
	if m.infoFunc != nil {
		return m.infoFunc()
	}
	return plugin.Info{Name: "mock"}
}

func (m *mockPlugin) Validate(ctx context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, config)
	}
	return &plugin.ValidateResponse{Valid: true}, nil
}

func TestExecuteHook_PluginReturnsNilResponse(t *testing.T) {
	cfg := &config.Config{
		Workflow: config.WorkflowConfig{
			DryRunByDefault: false,
		},
	}
	m := NewManager(cfg)

	// Add a mock plugin that returns nil response
	mockPlug := &mockPlugin{
		executeFunc: func(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
			return nil, nil
		},
		infoFunc: func() plugin.Info {
			return plugin.Info{
				Name:  "test-nil",
				Hooks: []plugin.Hook{plugin.HookPostPublish},
			}
		},
	}

	m.mu.Lock()
	m.plugins["test-nil"] = &loadedPlugin{
		name:    "test-nil",
		timeout: 30 * time.Second,
		plugin:  mockPlug,
		info: plugin.Info{
			Name:  "test-nil",
			Hooks: []plugin.Hook{plugin.HookPostPublish},
		},
	}
	m.mu.Unlock()

	// Execute hook
	ctx := context.Background()
	responses, err := m.ExecuteHook(ctx, plugin.HookPostPublish, plugin.ReleaseContext{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(responses))
	}

	if !responses[0].Success {
		t.Error("Expected success when plugin returns nil response")
	}
}

func TestExecuteHook_PluginReturnsError(t *testing.T) {
	cfg := &config.Config{
		Workflow: config.WorkflowConfig{
			DryRunByDefault: false,
		},
	}
	m := NewManager(cfg)

	// Add a mock plugin that returns an error
	mockPlug := &mockPlugin{
		executeFunc: func(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
			return nil, fmt.Errorf("plugin execution failed")
		},
		infoFunc: func() plugin.Info {
			return plugin.Info{
				Name:  "test-error",
				Hooks: []plugin.Hook{plugin.HookPostPublish},
			}
		},
	}

	m.mu.Lock()
	m.plugins["test-error"] = &loadedPlugin{
		name:    "test-error",
		timeout: 30 * time.Second,
		plugin:  mockPlug,
		info: plugin.Info{
			Name:  "test-error",
			Hooks: []plugin.Hook{plugin.HookPostPublish},
		},
	}
	m.mu.Unlock()

	// Execute hook
	ctx := context.Background()
	responses, err := m.ExecuteHook(ctx, plugin.HookPostPublish, plugin.ReleaseContext{})
	if err != nil {
		t.Errorf("ExecuteHook should not return error for plugin failures: %v", err)
	}

	if len(responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(responses))
	}

	if responses[0].Success {
		t.Error("Expected failure when plugin returns error")
	}
	if responses[0].Error == "" {
		t.Error("Expected error message in response")
	}
}

func TestExecuteHook_PluginSuccessWithMessage(t *testing.T) {
	cfg := &config.Config{
		Workflow: config.WorkflowConfig{
			DryRunByDefault: true,
		},
	}
	m := NewManager(cfg)

	// Add a mock plugin that returns success with message
	mockPlug := &mockPlugin{
		executeFunc: func(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
			if !req.DryRun {
				t.Error("Expected DryRun to be true based on config")
			}
			return &plugin.ExecuteResponse{
				Success: true,
				Message: "plugin executed successfully",
			}, nil
		},
		infoFunc: func() plugin.Info {
			return plugin.Info{
				Name:  "test-success",
				Hooks: []plugin.Hook{plugin.HookPostPublish},
			}
		},
	}

	m.mu.Lock()
	m.plugins["test-success"] = &loadedPlugin{
		name:    "test-success",
		timeout: 30 * time.Second,
		plugin:  mockPlug,
		info: plugin.Info{
			Name:  "test-success",
			Hooks: []plugin.Hook{plugin.HookPostPublish},
		},
	}
	m.mu.Unlock()

	// Execute hook
	ctx := context.Background()
	responses, err := m.ExecuteHook(ctx, plugin.HookPostPublish, plugin.ReleaseContext{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(responses))
	}

	if !responses[0].Success {
		t.Error("Expected success")
	}
	if responses[0].Message != "plugin executed successfully" {
		t.Errorf("Expected message 'plugin executed successfully', got %q", responses[0].Message)
	}
}

func TestExecuteHook_PluginFailureWithMessage(t *testing.T) {
	cfg := &config.Config{
		Workflow: config.WorkflowConfig{
			DryRunByDefault: false,
		},
	}
	m := NewManager(cfg)

	// Add a mock plugin that returns failure with error message in response
	mockPlug := &mockPlugin{
		executeFunc: func(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
			return &plugin.ExecuteResponse{
				Success: false,
				Error:   "operation failed: permission denied",
			}, nil
		},
		infoFunc: func() plugin.Info {
			return plugin.Info{
				Name:  "test-fail",
				Hooks: []plugin.Hook{plugin.HookOnError},
			}
		},
	}

	m.mu.Lock()
	m.plugins["test-fail"] = &loadedPlugin{
		name:    "test-fail",
		timeout: 30 * time.Second,
		plugin:  mockPlug,
		info: plugin.Info{
			Name:  "test-fail",
			Hooks: []plugin.Hook{plugin.HookOnError},
		},
	}
	m.mu.Unlock()

	// Execute hook
	ctx := context.Background()
	responses, err := m.ExecuteHook(ctx, plugin.HookOnError, plugin.ReleaseContext{})
	if err != nil {
		t.Errorf("ExecuteHook should not return error for plugin failures: %v", err)
	}

	if len(responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(responses))
	}

	if responses[0].Success {
		t.Error("Expected failure")
	}
	if !strings.Contains(responses[0].Error, "permission denied") {
		t.Errorf("Expected error to contain 'permission denied', got %q", responses[0].Error)
	}
}
