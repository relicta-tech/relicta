// Package integration provides domain types for plugin integration.
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHookString tests the Hook.String method.
func TestHookString(t *testing.T) {
	tests := []struct {
		hook     Hook
		expected string
	}{
		{HookPreInit, "pre-init"},
		{HookPostInit, "post-init"},
		{HookPrePlan, "pre-plan"},
		{HookPostPlan, "post-plan"},
		{HookPreVersion, "pre-version"},
		{HookPostVersion, "post-version"},
		{HookPreNotes, "pre-notes"},
		{HookPostNotes, "post-notes"},
		{HookPreApprove, "pre-approve"},
		{HookPostApprove, "post-approve"},
		{HookPrePublish, "pre-publish"},
		{HookPostPublish, "post-publish"},
		{HookOnSuccess, "on-success"},
		{HookOnError, "on-error"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.hook.String())
		})
	}
}

// TestHookIsValid tests the Hook.IsValid method.
func TestHookIsValid(t *testing.T) {
	// All defined hooks should be valid
	for _, hook := range AllHooks() {
		t.Run(hook.String(), func(t *testing.T) {
			assert.True(t, hook.IsValid())
		})
	}

	// Invalid hooks should return false
	invalidHooks := []Hook{
		"invalid",
		"",
		"pre-unknown",
	}
	for _, hook := range invalidHooks {
		t.Run(string(hook), func(t *testing.T) {
			assert.False(t, hook.IsValid())
		})
	}
}

// TestHookIsPre tests the Hook.IsPre method.
func TestHookIsPre(t *testing.T) {
	preHooks := []Hook{
		HookPreInit, HookPrePlan, HookPreVersion,
		HookPreNotes, HookPreApprove, HookPrePublish,
	}
	for _, hook := range preHooks {
		t.Run(hook.String(), func(t *testing.T) {
			assert.True(t, hook.IsPre())
		})
	}

	nonPreHooks := []Hook{
		HookPostInit, HookPostPlan, HookPostVersion,
		HookPostNotes, HookPostApprove, HookPostPublish,
		HookOnSuccess, HookOnError,
	}
	for _, hook := range nonPreHooks {
		t.Run(hook.String(), func(t *testing.T) {
			assert.False(t, hook.IsPre())
		})
	}
}

// TestHookIsPost tests the Hook.IsPost method.
func TestHookIsPost(t *testing.T) {
	postHooks := []Hook{
		HookPostInit, HookPostPlan, HookPostVersion,
		HookPostNotes, HookPostApprove, HookPostPublish,
	}
	for _, hook := range postHooks {
		t.Run(hook.String(), func(t *testing.T) {
			assert.True(t, hook.IsPost())
		})
	}

	nonPostHooks := []Hook{
		HookPreInit, HookPrePlan, HookPreVersion,
		HookPreNotes, HookPreApprove, HookPrePublish,
		HookOnSuccess, HookOnError,
	}
	for _, hook := range nonPostHooks {
		t.Run(hook.String(), func(t *testing.T) {
			assert.False(t, hook.IsPost())
		})
	}
}

// TestHookIsLifecycle tests the Hook.IsLifecycle method.
func TestHookIsLifecycle(t *testing.T) {
	assert.True(t, HookOnSuccess.IsLifecycle())
	assert.True(t, HookOnError.IsLifecycle())

	for _, hook := range PreHooks() {
		assert.False(t, hook.IsLifecycle())
	}
	for _, hook := range PostHooks() {
		assert.False(t, hook.IsLifecycle())
	}
}

// TestHookPair tests the Hook.HookPair method.
func TestHookPair(t *testing.T) {
	tests := []struct {
		hook    Hook
		expPre  Hook
		expPost Hook
	}{
		{HookPreInit, HookPreInit, HookPostInit},
		{HookPostInit, HookPreInit, HookPostInit},
		{HookPrePlan, HookPrePlan, HookPostPlan},
		{HookPostPlan, HookPrePlan, HookPostPlan},
		{HookPreVersion, HookPreVersion, HookPostVersion},
		{HookPostVersion, HookPreVersion, HookPostVersion},
		{HookPreNotes, HookPreNotes, HookPostNotes},
		{HookPostNotes, HookPreNotes, HookPostNotes},
		{HookPreApprove, HookPreApprove, HookPostApprove},
		{HookPostApprove, HookPreApprove, HookPostApprove},
		{HookPrePublish, HookPrePublish, HookPostPublish},
		{HookPostPublish, HookPrePublish, HookPostPublish},
		// Lifecycle hooks return themselves
		{HookOnSuccess, HookOnSuccess, HookOnSuccess},
		{HookOnError, HookOnError, HookOnError},
	}

	for _, tt := range tests {
		t.Run(tt.hook.String(), func(t *testing.T) {
			pre, post := tt.hook.HookPair()
			assert.Equal(t, tt.expPre, pre)
			assert.Equal(t, tt.expPost, post)
		})
	}
}

// TestHookDescription tests the Hook.Description method.
func TestHookDescription(t *testing.T) {
	// All valid hooks should have descriptions
	for _, hook := range AllHooks() {
		t.Run(hook.String(), func(t *testing.T) {
			desc := hook.Description()
			assert.NotEmpty(t, desc)
			assert.NotEqual(t, "Unknown hook", desc)
		})
	}

	// Invalid hook should return "Unknown hook"
	invalidHook := Hook("invalid")
	assert.Equal(t, "Unknown hook", invalidHook.Description())
}

// TestAllHooks tests the AllHooks function.
func TestAllHooks(t *testing.T) {
	hooks := AllHooks()
	assert.Len(t, hooks, 14) // 6 pre + 6 post + 2 lifecycle

	// Verify order
	assert.Equal(t, HookPreInit, hooks[0])
	assert.Equal(t, HookPostInit, hooks[1])
	assert.Equal(t, HookOnError, hooks[len(hooks)-1])
}

// TestPreHooks tests the PreHooks function.
func TestPreHooks(t *testing.T) {
	hooks := PreHooks()
	assert.Len(t, hooks, 6)
	for _, hook := range hooks {
		assert.True(t, hook.IsPre())
	}
}

// TestPostHooks tests the PostHooks function.
func TestPostHooks(t *testing.T) {
	hooks := PostHooks()
	assert.Len(t, hooks, 6)
	for _, hook := range hooks {
		assert.True(t, hook.IsPost())
	}
}

// TestPluginInstanceIsHealthy tests the PluginInstance.IsHealthy method.
func TestPluginInstanceIsHealthy(t *testing.T) {
	tests := []struct {
		name     string
		state    PluginState
		err      error
		expected bool
	}{
		{"ready and no error", PluginStateReady, nil, true},
		{"ready with error", PluginStateReady, ErrPluginExecutionFailed, false},
		{"loading", PluginStateLoading, nil, false},
		{"error state", PluginStateError, ErrPluginLoadFailed, false},
		{"disabled", PluginStateDisabled, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pi := &PluginInstance{
				State: tt.state,
				Error: tt.err,
			}
			assert.Equal(t, tt.expected, pi.IsHealthy())
		})
	}
}

// TestPluginInstanceSupportsHook tests the PluginInstance.SupportsHook method.
func TestPluginInstanceSupportsHook(t *testing.T) {
	pi := &PluginInstance{
		Info: PluginInfo{
			Hooks: []Hook{HookPostPublish, HookOnSuccess, HookOnError},
		},
	}

	assert.True(t, pi.SupportsHook(HookPostPublish))
	assert.True(t, pi.SupportsHook(HookOnSuccess))
	assert.True(t, pi.SupportsHook(HookOnError))
	assert.False(t, pi.SupportsHook(HookPreInit))
	assert.False(t, pi.SupportsHook(HookPrePublish))
}

// mockPlugin is a simple mock plugin for testing.
type mockPlugin struct {
	info PluginInfo
}

func (m *mockPlugin) GetInfo() PluginInfo {
	return m.info
}

func (m *mockPlugin) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	return &ExecuteResponse{Success: true, Message: "executed"}, nil
}

func (m *mockPlugin) Validate(config PluginConfig) (*ValidateResponse, error) {
	return &ValidateResponse{Valid: true}, nil
}

// TestInMemoryPluginRegistryRegister tests the InMemoryPluginRegistry.Register method.
func TestInMemoryPluginRegistryRegister(t *testing.T) {
	registry := NewInMemoryPluginRegistry()

	plugin := &mockPlugin{
		info: PluginInfo{
			ID:   "test-plugin",
			Name: "Test Plugin",
		},
	}

	// First registration should succeed
	err := registry.Register(plugin)
	require.NoError(t, err)

	// Second registration should fail
	err = registry.Register(plugin)
	assert.Equal(t, ErrPluginAlreadyRegistered, err)

	// Invalid plugin (empty ID) should fail
	invalidPlugin := &mockPlugin{
		info: PluginInfo{
			ID:   "",
			Name: "Invalid Plugin",
		},
	}
	err = registry.Register(invalidPlugin)
	assert.Equal(t, ErrInvalidPlugin, err)
}

// TestInMemoryPluginRegistryUnregister tests the InMemoryPluginRegistry.Unregister method.
func TestInMemoryPluginRegistryUnregister(t *testing.T) {
	registry := NewInMemoryPluginRegistry()

	plugin := &mockPlugin{
		info: PluginInfo{
			ID:   "test-plugin",
			Name: "Test Plugin",
		},
	}

	// Register first
	err := registry.Register(plugin)
	require.NoError(t, err)

	// Unregister should succeed
	err = registry.Unregister("test-plugin")
	require.NoError(t, err)

	// Unregister again should fail
	err = registry.Unregister("test-plugin")
	assert.Equal(t, ErrPluginNotFound, err)
}

// TestInMemoryPluginRegistryGet tests the InMemoryPluginRegistry.Get method.
func TestInMemoryPluginRegistryGet(t *testing.T) {
	registry := NewInMemoryPluginRegistry()

	plugin := &mockPlugin{
		info: PluginInfo{
			ID:   "test-plugin",
			Name: "Test Plugin",
		},
	}

	// Register
	err := registry.Register(plugin)
	require.NoError(t, err)

	// Get should return the plugin
	retrieved, err := registry.Get("test-plugin")
	require.NoError(t, err)
	assert.Equal(t, plugin, retrieved)

	// Get non-existent should fail
	_, err = registry.Get("non-existent")
	assert.Equal(t, ErrPluginNotFound, err)
}

// TestInMemoryPluginRegistryGetByHook tests the InMemoryPluginRegistry.GetByHook method.
func TestInMemoryPluginRegistryGetByHook(t *testing.T) {
	registry := NewInMemoryPluginRegistry()

	// Register plugins with different hooks
	publishPlugin := &mockPlugin{
		info: PluginInfo{
			ID:    "publish-plugin",
			Name:  "Publish Plugin",
			Hooks: []Hook{HookPostPublish},
		},
	}
	initPlugin := &mockPlugin{
		info: PluginInfo{
			ID:    "init-plugin",
			Name:  "Init Plugin",
			Hooks: []Hook{HookPreInit, HookPostInit},
		},
	}
	allHooksPlugin := &mockPlugin{
		info: PluginInfo{
			ID:    "all-hooks-plugin",
			Name:  "All Hooks Plugin",
			Hooks: []Hook{HookPreInit, HookPostPublish},
		},
	}

	require.NoError(t, registry.Register(publishPlugin))
	require.NoError(t, registry.Register(initPlugin))
	require.NoError(t, registry.Register(allHooksPlugin))

	// GetByHook for PostPublish should return 2 plugins
	plugins := registry.GetByHook(HookPostPublish)
	assert.Len(t, plugins, 2)

	// GetByHook for PreInit should return 2 plugins
	plugins = registry.GetByHook(HookPreInit)
	assert.Len(t, plugins, 2)

	// GetByHook for a hook with no plugins should return empty
	plugins = registry.GetByHook(HookOnError)
	assert.Empty(t, plugins)
}

// TestInMemoryPluginRegistryList tests the InMemoryPluginRegistry.List method.
func TestInMemoryPluginRegistryList(t *testing.T) {
	registry := NewInMemoryPluginRegistry()

	// Empty registry
	plugins := registry.List()
	assert.Empty(t, plugins)

	// Add plugins
	plugin1 := &mockPlugin{info: PluginInfo{ID: "plugin-1"}}
	plugin2 := &mockPlugin{info: PluginInfo{ID: "plugin-2"}}
	require.NoError(t, registry.Register(plugin1))
	require.NoError(t, registry.Register(plugin2))

	plugins = registry.List()
	assert.Len(t, plugins, 2)
}

// TestInMemoryPluginRegistryHas tests the InMemoryPluginRegistry.Has method.
func TestInMemoryPluginRegistryHas(t *testing.T) {
	registry := NewInMemoryPluginRegistry()

	plugin := &mockPlugin{info: PluginInfo{ID: "test-plugin"}}
	require.NoError(t, registry.Register(plugin))

	assert.True(t, registry.Has("test-plugin"))
	assert.False(t, registry.Has("non-existent"))
}

// TestSequentialPluginExecutorExecuteHook tests the SequentialPluginExecutor.ExecuteHook method.
func TestSequentialPluginExecutorExecuteHook(t *testing.T) {
	registry := NewInMemoryPluginRegistry()

	plugin := &mockPlugin{
		info: PluginInfo{
			ID:    "test-plugin",
			Hooks: []Hook{HookPostPublish},
		},
	}
	require.NoError(t, registry.Register(plugin))

	executor := NewSequentialPluginExecutor(registry)

	ctx := context.Background()
	releaseCtx := ReleaseContext{
		RepositoryName: "test-repo",
		Timestamp:      time.Now(),
	}

	// Execute hook with registered plugins
	responses, err := executor.ExecuteHook(ctx, HookPostPublish, releaseCtx)
	require.NoError(t, err)
	assert.Len(t, responses, 1)
	assert.True(t, responses[0].Success)

	// Execute hook with no registered plugins
	responses, err = executor.ExecuteHook(ctx, HookPreInit, releaseCtx)
	require.NoError(t, err)
	assert.Nil(t, responses)
}

// TestSequentialPluginExecutorPluginConfig tests the SequentialPluginExecutor config methods.
func TestSequentialPluginExecutorPluginConfig(t *testing.T) {
	registry := NewInMemoryPluginRegistry()
	executor := NewSequentialPluginExecutor(registry)

	config := PluginConfig{"key": "value"}
	executor.SetPluginConfig("test-plugin", config)

	retrieved := executor.GetPluginConfig("test-plugin")
	assert.Equal(t, config, retrieved)

	// Non-existent should return nil
	nonExistent := executor.GetPluginConfig("non-existent")
	assert.Nil(t, nonExistent)
}

// TestSequentialPluginExecutorExecutePlugin tests the SequentialPluginExecutor.ExecutePlugin method.
func TestSequentialPluginExecutorExecutePlugin(t *testing.T) {
	registry := NewInMemoryPluginRegistry()

	plugin := &mockPlugin{
		info: PluginInfo{
			ID:    "test-plugin",
			Hooks: []Hook{HookPostPublish},
		},
	}
	require.NoError(t, registry.Register(plugin))

	executor := NewSequentialPluginExecutor(registry)
	executor.SetPluginConfig("test-plugin", PluginConfig{"key": "value"})

	ctx := context.Background()
	req := ExecuteRequest{
		Hook: HookPostPublish,
	}

	// Execute registered plugin
	response, err := executor.ExecutePlugin(ctx, "test-plugin", req)
	require.NoError(t, err)
	assert.True(t, response.Success)

	// Execute non-existent plugin
	_, err = executor.ExecutePlugin(ctx, "non-existent", req)
	assert.Equal(t, ErrPluginNotFound, err)
}

// TestParallelPluginExecutorExecuteHook tests the ParallelPluginExecutor.ExecuteHook method.
func TestParallelPluginExecutorExecuteHook(t *testing.T) {
	registry := NewInMemoryPluginRegistry()

	// Register multiple plugins
	for i := 0; i < 5; i++ {
		plugin := &mockPlugin{
			info: PluginInfo{
				ID:    PluginID("plugin-" + string(rune('a'+i))),
				Hooks: []Hook{HookPostPublish},
			},
		}
		require.NoError(t, registry.Register(plugin))
	}

	executor := NewParallelPluginExecutor(registry)

	ctx := context.Background()
	releaseCtx := ReleaseContext{
		RepositoryName: "test-repo",
		Timestamp:      time.Now(),
	}

	// Execute hook with registered plugins
	responses, err := executor.ExecuteHook(ctx, HookPostPublish, releaseCtx)
	require.NoError(t, err)
	assert.Len(t, responses, 5)

	for _, resp := range responses {
		assert.True(t, resp.Success)
	}
}

// TestParallelPluginExecutorContextCancellation tests context cancellation handling.
func TestParallelPluginExecutorContextCancellation(t *testing.T) {
	registry := NewInMemoryPluginRegistry()

	plugin := &mockPlugin{
		info: PluginInfo{
			ID:    "test-plugin",
			Hooks: []Hook{HookPostPublish},
		},
	}
	require.NoError(t, registry.Register(plugin))

	executor := NewParallelPluginExecutor(registry)

	// Create a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	releaseCtx := ReleaseContext{
		RepositoryName: "test-repo",
		Timestamp:      time.Now(),
	}

	// Execute hook with canceled context
	responses, err := executor.ExecuteHook(ctx, HookPostPublish, releaseCtx)
	require.NoError(t, err)
	assert.Len(t, responses, 1)
	assert.False(t, responses[0].Success)
	assert.Contains(t, responses[0].Error, "context canceled")
}

// TestParallelPluginExecutorPluginConfig tests the ParallelPluginExecutor config methods.
func TestParallelPluginExecutorPluginConfig(t *testing.T) {
	registry := NewInMemoryPluginRegistry()
	executor := NewParallelPluginExecutor(registry)

	config := PluginConfig{"key": "value"}
	executor.SetPluginConfig("test-plugin", config)

	retrieved := executor.GetPluginConfig("test-plugin")
	assert.Equal(t, config, retrieved)
}

// TestParallelPluginExecutorExecutePlugin tests the ParallelPluginExecutor.ExecutePlugin method.
func TestParallelPluginExecutorExecutePlugin(t *testing.T) {
	registry := NewInMemoryPluginRegistry()

	plugin := &mockPlugin{
		info: PluginInfo{
			ID:    "test-plugin",
			Hooks: []Hook{HookPostPublish},
		},
	}
	require.NoError(t, registry.Register(plugin))

	executor := NewParallelPluginExecutor(registry)

	ctx := context.Background()
	req := ExecuteRequest{
		Hook: HookPostPublish,
	}

	// Execute registered plugin
	response, err := executor.ExecutePlugin(ctx, "test-plugin", req)
	require.NoError(t, err)
	assert.True(t, response.Success)

	// Execute non-existent plugin
	_, err = executor.ExecutePlugin(ctx, "non-existent", req)
	assert.Equal(t, ErrPluginNotFound, err)
}

// TestPluginInfoFields tests PluginInfo field access.
func TestPluginInfoFields(t *testing.T) {
	info := PluginInfo{
		ID:           "test-id",
		Name:         "Test Name",
		Version:      "1.0.0",
		Description:  "Test Description",
		Author:       "Test Author",
		Hooks:        []Hook{HookPostPublish},
		ConfigSchema: `{"type": "object"}`,
	}

	assert.Equal(t, PluginID("test-id"), info.ID)
	assert.Equal(t, "Test Name", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Equal(t, "Test Description", info.Description)
	assert.Equal(t, "Test Author", info.Author)
	assert.Len(t, info.Hooks, 1)
	assert.NotEmpty(t, info.ConfigSchema)
}

// TestExecuteResponseFields tests ExecuteResponse field access.
func TestExecuteResponseFields(t *testing.T) {
	resp := ExecuteResponse{
		Success: true,
		Message: "Completed",
		Outputs: map[string]any{"url": "https://example.com"},
		Artifacts: []Artifact{
			{
				Name: "release.tar.gz",
				Path: "/tmp/release.tar.gz",
				Type: "archive",
				Size: 1024,
				URL:  "https://example.com/release.tar.gz",
			},
		},
	}

	assert.True(t, resp.Success)
	assert.Equal(t, "Completed", resp.Message)
	assert.Equal(t, "https://example.com", resp.Outputs["url"])
	assert.Len(t, resp.Artifacts, 1)
	assert.Equal(t, "release.tar.gz", resp.Artifacts[0].Name)
}

// TestValidationErrorFields tests ValidationError field access.
func TestValidationErrorFields(t *testing.T) {
	verr := ValidationError{
		Field:   "webhook_url",
		Message: "must be a valid URL",
		Code:    "INVALID_URL",
	}

	assert.Equal(t, "webhook_url", verr.Field)
	assert.Equal(t, "must be a valid URL", verr.Message)
	assert.Equal(t, "INVALID_URL", verr.Code)
}

// TestReleaseContextFields tests ReleaseContext field access.
func TestReleaseContextFields(t *testing.T) {
	ctx := ReleaseContext{
		RepositoryOwner: "owner",
		RepositoryName:  "repo",
		RepositoryPath:  "/path/to/repo",
		Branch:          "main",
		TagName:         "v1.0.0",
		Changelog:       "# Changelog",
		ReleaseNotes:    "## Release Notes",
		DryRun:          true,
		Timestamp:       time.Now(),
	}

	assert.Equal(t, "owner", ctx.RepositoryOwner)
	assert.Equal(t, "repo", ctx.RepositoryName)
	assert.Equal(t, "/path/to/repo", ctx.RepositoryPath)
	assert.Equal(t, "main", ctx.Branch)
	assert.Equal(t, "v1.0.0", ctx.TagName)
	assert.Equal(t, "# Changelog", ctx.Changelog)
	assert.Equal(t, "## Release Notes", ctx.ReleaseNotes)
	assert.True(t, ctx.DryRun)
}
