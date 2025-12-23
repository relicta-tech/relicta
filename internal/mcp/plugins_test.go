package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

// mockPluginManager implements PluginManager for testing.
type mockPluginManager struct {
	plugins       []plugin.Info
	getInfoErr    error
	executeErr    error
	executeResult []plugin.ExecuteResponse
}

func (m *mockPluginManager) ListPlugins() []plugin.Info {
	return m.plugins
}

func (m *mockPluginManager) GetPluginInfo(name string) (*plugin.Info, error) {
	if m.getInfoErr != nil {
		return nil, m.getInfoErr
	}

	for i, p := range m.plugins {
		if p.Name == name {
			return &m.plugins[i], nil
		}
	}
	return nil, errors.New("plugin not found")
}

func (m *mockPluginManager) ExecuteHook(_ context.Context, _ plugin.Hook, _ plugin.ReleaseContext) ([]plugin.ExecuteResponse, error) {
	if m.executeErr != nil {
		return nil, m.executeErr
	}
	return m.executeResult, nil
}

func newMockPluginManager() *mockPluginManager {
	return &mockPluginManager{
		plugins: []plugin.Info{
			{
				Name:        "github",
				Version:     "1.0.0",
				Description: "GitHub release integration",
				Author:      "Relicta Team",
				Hooks: []plugin.Hook{
					plugin.HookPostPublish,
					plugin.HookOnSuccess,
				},
				ConfigSchema: `{"type":"object"}`,
			},
			{
				Name:        "slack",
				Version:     "1.2.0",
				Description: "Slack notifications",
				Author:      "Relicta Team",
				Hooks: []plugin.Hook{
					plugin.HookPostPublish,
					plugin.HookOnError,
				},
			},
		},
	}
}

func TestNewPluginServer(t *testing.T) {
	pm := newMockPluginManager()
	server, err := NewPluginServer("1.0.0", pm)

	require.NoError(t, err)
	assert.NotNil(t, server)
	assert.NotNil(t, server.Server)
	assert.Same(t, pm, server.PluginManager())
}

func TestPluginServer_SetPluginManager(t *testing.T) {
	server, err := NewPluginServer("1.0.0", nil)
	require.NoError(t, err)

	pm := newMockPluginManager()
	server.SetPluginManager(pm)

	assert.Same(t, pm, server.PluginManager())
}

func TestPluginServer_RegisterPluginTools(t *testing.T) {
	pm := newMockPluginManager()
	server, _ := NewPluginServer("1.0.0", pm)
	server.RegisterPluginTools()

	// Verify tools are registered
	assert.Contains(t, server.tools, "relicta.plugins.list")
	assert.Contains(t, server.tools, "relicta.plugins.info")
	assert.Contains(t, server.tools, "relicta.plugins.execute")
	assert.Contains(t, server.tools, "relicta.plugins.hooks")

	// Verify resource is registered
	assert.Contains(t, server.resources, "relicta://plugins")
}

func TestPluginServer_ToolPluginsList(t *testing.T) {
	pm := newMockPluginManager()
	server, _ := NewPluginServer("1.0.0", pm)
	server.RegisterPluginTools()

	result, err := server.toolPluginsList(context.Background(), nil)

	require.NoError(t, err)
	assert.False(t, result.IsError)

	var listResult PluginListResult
	err = json.Unmarshal([]byte(result.Content[0].Text), &listResult)
	require.NoError(t, err)
	assert.Equal(t, 2, listResult.Count)
	assert.Len(t, listResult.Plugins, 2)

	// Verify plugin data
	var github, slack PluginInfoResult
	for _, p := range listResult.Plugins {
		if p.Name == "github" {
			github = p
		}
		if p.Name == "slack" {
			slack = p
		}
	}
	assert.Equal(t, "1.0.0", github.Version)
	assert.Equal(t, "GitHub release integration", github.Description)
	assert.Equal(t, "1.2.0", slack.Version)
}

func TestPluginServer_ToolPluginsList_NoManager(t *testing.T) {
	server, _ := NewPluginServer("1.0.0", nil)
	server.RegisterPluginTools()

	result, err := server.toolPluginsList(context.Background(), nil)

	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "plugin manager not initialized")
}

func TestPluginServer_ToolPluginsInfo(t *testing.T) {
	pm := newMockPluginManager()
	server, _ := NewPluginServer("1.0.0", pm)
	server.RegisterPluginTools()

	result, err := server.toolPluginsInfo(context.Background(), map[string]any{
		"name": "github",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)

	var infoResult PluginInfoResult
	err = json.Unmarshal([]byte(result.Content[0].Text), &infoResult)
	require.NoError(t, err)
	assert.Equal(t, "github", infoResult.Name)
	assert.Equal(t, "1.0.0", infoResult.Version)
	assert.Equal(t, "Relicta Team", infoResult.Author)
	assert.Contains(t, infoResult.Hooks, "post-publish")
}

func TestPluginServer_ToolPluginsInfo_MissingName(t *testing.T) {
	pm := newMockPluginManager()
	server, _ := NewPluginServer("1.0.0", pm)
	server.RegisterPluginTools()

	result, err := server.toolPluginsInfo(context.Background(), map[string]any{})

	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "plugin name is required")
}

func TestPluginServer_ToolPluginsInfo_NotFound(t *testing.T) {
	pm := newMockPluginManager()
	server, _ := NewPluginServer("1.0.0", pm)
	server.RegisterPluginTools()

	result, err := server.toolPluginsInfo(context.Background(), map[string]any{
		"name": "nonexistent",
	})

	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "plugin not found")
}

func TestPluginServer_ToolPluginsExecute(t *testing.T) {
	pm := newMockPluginManager()
	pm.executeResult = []plugin.ExecuteResponse{
		{
			Success: true,
			Message: "GitHub release created",
			Outputs: map[string]any{
				"release_url": "https://github.com/org/repo/releases/v1.0.0",
			},
		},
		{
			Success: true,
			Message: "Slack notification sent",
		},
	}

	server, _ := NewPluginServer("1.0.0", pm)
	server.RegisterPluginTools()

	result, err := server.toolPluginsExecute(context.Background(), map[string]any{
		"hook": "post-publish",
		"context": map[string]any{
			"version":  "1.0.0",
			"tag_name": "v1.0.0",
			"branch":   "main",
		},
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)

	var execResult PluginExecuteResult
	err = json.Unmarshal([]byte(result.Content[0].Text), &execResult)
	require.NoError(t, err)
	assert.Equal(t, "post-publish", execResult.Hook)
	assert.Equal(t, 2, execResult.Executed)
	assert.Equal(t, 2, execResult.Succeeded)
	assert.Equal(t, 0, execResult.Failed)
	assert.Len(t, execResult.Results, 2)
}

func TestPluginServer_ToolPluginsExecute_WithFailures(t *testing.T) {
	pm := newMockPluginManager()
	pm.executeResult = []plugin.ExecuteResponse{
		{
			Success: true,
			Message: "GitHub release created",
		},
		{
			Success: false,
			Error:   "Slack API error: rate limited",
		},
	}

	server, _ := NewPluginServer("1.0.0", pm)
	server.RegisterPluginTools()

	result, err := server.toolPluginsExecute(context.Background(), map[string]any{
		"hook": "post-publish",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)

	var execResult PluginExecuteResult
	err = json.Unmarshal([]byte(result.Content[0].Text), &execResult)
	require.NoError(t, err)
	assert.Equal(t, 2, execResult.Executed)
	assert.Equal(t, 1, execResult.Succeeded)
	assert.Equal(t, 1, execResult.Failed)
}

func TestPluginServer_ToolPluginsExecute_WithArtifacts(t *testing.T) {
	pm := newMockPluginManager()
	pm.executeResult = []plugin.ExecuteResponse{
		{
			Success: true,
			Message: "Build completed",
			Artifacts: []plugin.Artifact{
				{
					Name:     "app-linux-amd64",
					Path:     "/dist/app-linux-amd64",
					Type:     "binary",
					Size:     1024000,
					Checksum: "sha256:abc123",
				},
			},
		},
	}

	server, _ := NewPluginServer("1.0.0", pm)
	server.RegisterPluginTools()

	result, err := server.toolPluginsExecute(context.Background(), map[string]any{
		"hook": "post-version",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)

	var execResult PluginExecuteResult
	err = json.Unmarshal([]byte(result.Content[0].Text), &execResult)
	require.NoError(t, err)
	require.Len(t, execResult.Results, 1)
	require.Len(t, execResult.Results[0].Artifacts, 1)
	assert.Equal(t, "app-linux-amd64", execResult.Results[0].Artifacts[0].Name)
	assert.Equal(t, int64(1024000), execResult.Results[0].Artifacts[0].Size)
}

func TestPluginServer_ToolPluginsExecute_MissingHook(t *testing.T) {
	pm := newMockPluginManager()
	server, _ := NewPluginServer("1.0.0", pm)
	server.RegisterPluginTools()

	result, err := server.toolPluginsExecute(context.Background(), map[string]any{})

	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "hook is required")
}

func TestPluginServer_ToolPluginsExecute_InvalidHook(t *testing.T) {
	pm := newMockPluginManager()
	server, _ := NewPluginServer("1.0.0", pm)
	server.RegisterPluginTools()

	result, err := server.toolPluginsExecute(context.Background(), map[string]any{
		"hook": "invalid-hook",
	})

	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "invalid hook")
}

func TestPluginServer_ToolPluginsExecute_ExecutionError(t *testing.T) {
	pm := newMockPluginManager()
	pm.executeErr = errors.New("execution failed")

	server, _ := NewPluginServer("1.0.0", pm)
	server.RegisterPluginTools()

	result, err := server.toolPluginsExecute(context.Background(), map[string]any{
		"hook": "post-publish",
	})

	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "hook execution failed")
}

func TestPluginServer_ToolPluginsHooks(t *testing.T) {
	pm := newMockPluginManager()
	server, _ := NewPluginServer("1.0.0", pm)
	server.RegisterPluginTools()

	result, err := server.toolPluginsHooks(context.Background(), nil)

	require.NoError(t, err)
	assert.False(t, result.IsError)

	var hooksResult HooksListResult
	err = json.Unmarshal([]byte(result.Content[0].Text), &hooksResult)
	require.NoError(t, err)
	assert.Equal(t, 14, hooksResult.Count)
	assert.Len(t, hooksResult.Hooks, 14)

	// Verify some hooks
	hookNames := make(map[string]HookInfo)
	for _, h := range hooksResult.Hooks {
		hookNames[h.Name] = h
	}

	assert.Contains(t, hookNames, "pre-init")
	assert.Contains(t, hookNames, "post-publish")
	assert.Contains(t, hookNames, "on-success")
	assert.Contains(t, hookNames, "on-error")

	// Verify phases
	assert.Equal(t, "init", hookNames["pre-init"].Phase)
	assert.Equal(t, "publish", hookNames["post-publish"].Phase)
	assert.Equal(t, "lifecycle", hookNames["on-success"].Phase)
}

func TestPluginServer_ResourcePlugins(t *testing.T) {
	pm := newMockPluginManager()
	server, _ := NewPluginServer("1.0.0", pm)
	server.RegisterPluginTools()

	result, err := server.resourcePlugins(context.Background(), "relicta://plugins")

	require.NoError(t, err)
	require.Len(t, result.Contents, 1)
	assert.Equal(t, "relicta://plugins", result.Contents[0].URI)
	assert.Equal(t, "application/json", result.Contents[0].MIMEType)

	var listResult PluginListResult
	err = json.Unmarshal([]byte(result.Contents[0].Text), &listResult)
	require.NoError(t, err)
	assert.Equal(t, 2, listResult.Count)
}

func TestPluginServer_ResourcePlugins_NoManager(t *testing.T) {
	server, _ := NewPluginServer("1.0.0", nil)
	server.RegisterPluginTools()

	result, err := server.resourcePlugins(context.Background(), "relicta://plugins")

	require.NoError(t, err)
	require.Len(t, result.Contents, 1)

	var listResult PluginListResult
	err = json.Unmarshal([]byte(result.Contents[0].Text), &listResult)
	require.NoError(t, err)
	assert.Equal(t, 0, listResult.Count)
	assert.Empty(t, listResult.Plugins)
}

func TestIsValidHook(t *testing.T) {
	tests := []struct {
		hook  plugin.Hook
		valid bool
	}{
		{plugin.HookPreInit, true},
		{plugin.HookPostInit, true},
		{plugin.HookPrePlan, true},
		{plugin.HookPostPlan, true},
		{plugin.HookPreVersion, true},
		{plugin.HookPostVersion, true},
		{plugin.HookPreNotes, true},
		{plugin.HookPostNotes, true},
		{plugin.HookPreApprove, true},
		{plugin.HookPostApprove, true},
		{plugin.HookPrePublish, true},
		{plugin.HookPostPublish, true},
		{plugin.HookOnSuccess, true},
		{plugin.HookOnError, true},
		{"invalid-hook", false},
		{"", false},
		{"pre", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.hook), func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidHook(tt.hook))
		})
	}
}

func TestNewPluginClient(t *testing.T) {
	transport := newMockTransport()
	client := NewPluginClient(transport)

	assert.NotNil(t, client)
	assert.NotNil(t, client.Client)
}

func TestPluginClient_ListPlugins(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"plugins": [
				{"name": "github", "version": "1.0.0", "hooks": ["post-publish"]},
				{"name": "slack", "version": "1.2.0", "hooks": ["post-publish", "on-error"]}
			],
			"count": 2
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewPluginClient(transport)
	listResult, err := client.ListPlugins(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, listResult.Count)
	assert.Len(t, listResult.Plugins, 2)
}

func TestPluginClient_GetPluginInfo(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"name": "github",
			"version": "1.0.0",
			"description": "GitHub release integration",
			"author": "Relicta Team",
			"hooks": ["post-publish", "on-success"]
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewPluginClient(transport)
	infoResult, err := client.GetPluginInfo(context.Background(), "github")

	require.NoError(t, err)
	assert.Equal(t, "github", infoResult.Name)
	assert.Equal(t, "1.0.0", infoResult.Version)
	assert.Equal(t, "Relicta Team", infoResult.Author)
}

func TestPluginClient_ExecuteHook(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"hook": "post-publish",
			"executed": 2,
			"succeeded": 2,
			"failed": 0,
			"results": [
				{"success": true, "message": "Release created"},
				{"success": true, "message": "Notification sent"}
			]
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewPluginClient(transport)
	execResult, err := client.ExecuteHook(context.Background(), "post-publish", &ReleaseContext{
		Version: "1.0.0",
		TagName: "v1.0.0",
		Branch:  "main",
	})

	require.NoError(t, err)
	assert.Equal(t, "post-publish", execResult.Hook)
	assert.Equal(t, 2, execResult.Executed)
	assert.Equal(t, 2, execResult.Succeeded)
	assert.Equal(t, 0, execResult.Failed)
}

func TestPluginClient_ExecuteHook_NilContext(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"hook": "pre-init",
			"executed": 0,
			"succeeded": 0,
			"failed": 0,
			"results": []
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewPluginClient(transport)
	execResult, err := client.ExecuteHook(context.Background(), "pre-init", nil)

	require.NoError(t, err)
	assert.Equal(t, "pre-init", execResult.Hook)
}

func TestPluginClient_ListHooks(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"hooks": [
				{"name": "pre-init", "description": "Runs before initialization", "phase": "init"},
				{"name": "post-publish", "description": "Runs after publishing", "phase": "publish"}
			],
			"count": 2
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewPluginClient(transport)
	hooksResult, err := client.ListHooks(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, hooksResult.Count)
	assert.Len(t, hooksResult.Hooks, 2)
}

func TestPluginClient_GetPlugins(t *testing.T) {
	result := ReadResourceResult{
		Contents: []ResourceContent{{
			URI:      "relicta://plugins",
			MIMEType: "application/json",
			Text: `{
				"plugins": [
					{"name": "github", "version": "1.0.0", "hooks": ["post-publish"]}
				],
				"count": 1
			}`,
		}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewPluginClient(transport)
	pluginsResult, err := client.GetPlugins(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, pluginsResult.Count)
	assert.Len(t, pluginsResult.Plugins, 1)
	assert.Equal(t, "github", pluginsResult.Plugins[0].Name)
}

func TestPluginInfoResult_JSON(t *testing.T) {
	info := PluginInfoResult{
		Name:         "test-plugin",
		Version:      "2.0.0",
		Description:  "A test plugin",
		Author:       "Test Author",
		Hooks:        []string{"pre-init", "post-publish"},
		ConfigSchema: `{"type":"object","properties":{}}`,
	}

	data, err := json.Marshal(info)
	require.NoError(t, err)

	var decoded PluginInfoResult
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "test-plugin", decoded.Name)
	assert.Equal(t, "2.0.0", decoded.Version)
	assert.Len(t, decoded.Hooks, 2)
	assert.NotEmpty(t, decoded.ConfigSchema)
}

func TestPluginExecuteResult_JSON(t *testing.T) {
	execResult := PluginExecuteResult{
		Hook:      "post-publish",
		Executed:  2,
		Succeeded: 1,
		Failed:    1,
		Results: []PluginHookResult{
			{
				Success: true,
				Message: "Success",
				Outputs: map[string]any{"url": "https://example.com"},
			},
			{
				Success: false,
				Error:   "Failed to connect",
			},
		},
	}

	data, err := json.Marshal(execResult)
	require.NoError(t, err)

	var decoded PluginExecuteResult
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "post-publish", decoded.Hook)
	assert.Equal(t, 2, decoded.Executed)
	assert.Equal(t, 1, decoded.Succeeded)
	assert.Equal(t, 1, decoded.Failed)
	assert.Len(t, decoded.Results, 2)
}

func TestReleaseContext_JSON(t *testing.T) {
	ctx := ReleaseContext{
		Version:         "1.0.0",
		PreviousVersion: "0.9.0",
		TagName:         "v1.0.0",
		ReleaseType:     "minor",
		RepositoryURL:   "https://github.com/org/repo",
		RepositoryOwner: "org",
		RepositoryName:  "repo",
		Branch:          "main",
		CommitSHA:       "abc123",
		Changelog:       "## Changes\n- Feature 1",
		ReleaseNotes:    "Release notes here",
		Environment: map[string]string{
			"CI":    "true",
			"BUILD": "123",
		},
	}

	data, err := json.Marshal(ctx)
	require.NoError(t, err)

	var decoded ReleaseContext
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "1.0.0", decoded.Version)
	assert.Equal(t, "0.9.0", decoded.PreviousVersion)
	assert.Equal(t, "v1.0.0", decoded.TagName)
	assert.Equal(t, "minor", decoded.ReleaseType)
	assert.Equal(t, "main", decoded.Branch)
	assert.Len(t, decoded.Environment, 2)
}

func TestHookInfo_JSON(t *testing.T) {
	hookInfo := HookInfo{
		Name:        "post-publish",
		Description: "Runs after publishing",
		Phase:       "publish",
	}

	data, err := json.Marshal(hookInfo)
	require.NoError(t, err)

	var decoded HookInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "post-publish", decoded.Name)
	assert.Equal(t, "publish", decoded.Phase)
}

func TestArtifact_JSON(t *testing.T) {
	artifact := Artifact{
		Name:     "app-binary",
		Path:     "/dist/app",
		Type:     "binary",
		Size:     1024000,
		Checksum: "sha256:abc123",
	}

	data, err := json.Marshal(artifact)
	require.NoError(t, err)

	var decoded Artifact
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "app-binary", decoded.Name)
	assert.Equal(t, int64(1024000), decoded.Size)
	assert.Equal(t, "sha256:abc123", decoded.Checksum)
}

func TestPluginServer_ConcurrentAccess(t *testing.T) {
	pm := newMockPluginManager()
	pm.executeResult = []plugin.ExecuteResponse{
		{Success: true, Message: "OK"},
	}

	server, _ := NewPluginServer("1.0.0", pm)
	server.RegisterPluginTools()

	// Concurrent operations
	done := make(chan bool, 10)

	// Concurrent list operations
	for i := 0; i < 5; i++ {
		go func() {
			_, _ = server.toolPluginsList(context.Background(), nil)
			done <- true
		}()
	}

	// Concurrent execute operations
	for i := 0; i < 5; i++ {
		go func() {
			_, _ = server.toolPluginsExecute(context.Background(), map[string]any{
				"hook": "post-publish",
			})
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestPluginServer_ExecuteWithFullContext(t *testing.T) {
	pm := newMockPluginManager()
	pm.executeResult = []plugin.ExecuteResponse{
		{Success: true},
	}

	server, _ := NewPluginServer("1.0.0", pm)
	server.RegisterPluginTools()

	result, err := server.toolPluginsExecute(context.Background(), map[string]any{
		"hook": "post-publish",
		"context": map[string]any{
			"version":          "1.0.0",
			"previous_version": "0.9.0",
			"tag_name":         "v1.0.0",
			"release_type":     "minor",
			"repository_url":   "https://github.com/org/repo",
			"repository_owner": "org",
			"repository_name":  "repo",
			"branch":           "main",
			"commit_sha":       "abc123def456",
			"changelog":        "## Changelog",
			"release_notes":    "Release notes",
			"environment": map[string]any{
				"CI":     "true",
				"RUNNER": "github",
			},
		},
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
}
