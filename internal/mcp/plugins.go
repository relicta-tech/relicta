package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/relicta-tech/relicta/pkg/plugin"
)

// PluginManager interface for plugin operations.
// This allows for testing and decoupling from the concrete implementation.
type PluginManager interface {
	// ListPlugins returns info for all loaded plugins.
	ListPlugins() []plugin.Info
	// GetPluginInfo returns info for a specific plugin.
	GetPluginInfo(name string) (*plugin.Info, error)
	// ExecuteHook executes all plugins for a given hook.
	ExecuteHook(ctx context.Context, hook plugin.Hook, releaseCtx plugin.ReleaseContext) ([]plugin.ExecuteResponse, error)
}

// PluginServer extends Server with plugin management capabilities.
type PluginServer struct {
	*Server
	pluginManager PluginManager
	mu            sync.RWMutex
}

// NewPluginServer creates a server with plugin support.
func NewPluginServer(version string, pluginManager PluginManager, opts ...ServerOption) (*PluginServer, error) {
	server, err := NewServer(version, opts...)
	if err != nil {
		return nil, err
	}

	return &PluginServer{
		Server:        server,
		pluginManager: pluginManager,
	}, nil
}

// SetPluginManager sets the plugin manager.
func (s *PluginServer) SetPluginManager(pm PluginManager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pluginManager = pm
}

// PluginManager returns the current plugin manager.
func (s *PluginServer) PluginManager() PluginManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pluginManager
}

// RegisterPluginTools registers plugin-specific tools and resources.
func (s *PluginServer) RegisterPluginTools() {
	// Tool: plugins/list - List all plugins
	s.tools["relicta.plugins.list"] = s.toolPluginsList

	// Tool: plugins/info - Get plugin info
	s.tools["relicta.plugins.info"] = s.toolPluginsInfo

	// Tool: plugins/execute - Execute a plugin hook
	s.tools["relicta.plugins.execute"] = s.toolPluginsExecute

	// Tool: plugins/hooks - List available hooks
	s.tools["relicta.plugins.hooks"] = s.toolPluginsHooks

	// Resource: relicta://plugins - Plugin registry
	s.resources["relicta://plugins"] = s.resourcePlugins
}

// PluginListResult contains the list of plugins.
type PluginListResult struct {
	Plugins []PluginInfoResult `json:"plugins"`
	Count   int                `json:"count"`
}

// PluginInfoResult contains plugin info for JSON output.
type PluginInfoResult struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Description  string   `json:"description"`
	Author       string   `json:"author"`
	Hooks        []string `json:"hooks"`
	ConfigSchema string   `json:"config_schema,omitempty"`
}

func (s *PluginServer) toolPluginsList(_ context.Context, _ map[string]any) (*CallToolResult, error) {
	s.mu.RLock()
	pm := s.pluginManager
	s.mu.RUnlock()

	if pm == nil {
		return NewToolResultError("plugin manager not initialized"), nil
	}

	infos := pm.ListPlugins()
	result := PluginListResult{
		Plugins: make([]PluginInfoResult, 0, len(infos)),
		Count:   len(infos),
	}

	for _, info := range infos {
		hooks := make([]string, len(info.Hooks))
		for i, h := range info.Hooks {
			hooks[i] = string(h)
		}

		result.Plugins = append(result.Plugins, PluginInfoResult{
			Name:         info.Name,
			Version:      info.Version,
			Description:  info.Description,
			Author:       info.Author,
			Hooks:        hooks,
			ConfigSchema: info.ConfigSchema,
		})
	}

	return NewToolResultJSON(result)
}

func (s *PluginServer) toolPluginsInfo(_ context.Context, args map[string]any) (*CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return NewToolResultError("plugin name is required"), nil
	}

	s.mu.RLock()
	pm := s.pluginManager
	s.mu.RUnlock()

	if pm == nil {
		return NewToolResultError("plugin manager not initialized"), nil
	}

	info, err := pm.GetPluginInfo(name)
	if err != nil {
		return NewToolResultError(fmt.Sprintf("plugin not found: %s", name)), nil
	}

	hooks := make([]string, len(info.Hooks))
	for i, h := range info.Hooks {
		hooks[i] = string(h)
	}

	result := PluginInfoResult{
		Name:         info.Name,
		Version:      info.Version,
		Description:  info.Description,
		Author:       info.Author,
		Hooks:        hooks,
		ConfigSchema: info.ConfigSchema,
	}

	return NewToolResultJSON(result)
}

// PluginExecuteArgs contains arguments for plugin execution.
type PluginExecuteArgs struct {
	Hook    string         `json:"hook"`
	Context ReleaseContext `json:"context,omitempty"`
	DryRun  bool           `json:"dry_run,omitempty"`
}

// ReleaseContext mirrors the plugin release context for MCP.
type ReleaseContext struct {
	Version         string            `json:"version,omitempty"`
	PreviousVersion string            `json:"previous_version,omitempty"`
	TagName         string            `json:"tag_name,omitempty"`
	ReleaseType     string            `json:"release_type,omitempty"`
	RepositoryURL   string            `json:"repository_url,omitempty"`
	RepositoryOwner string            `json:"repository_owner,omitempty"`
	RepositoryName  string            `json:"repository_name,omitempty"`
	Branch          string            `json:"branch,omitempty"`
	CommitSHA       string            `json:"commit_sha,omitempty"`
	Changelog       string            `json:"changelog,omitempty"`
	ReleaseNotes    string            `json:"release_notes,omitempty"`
	Environment     map[string]string `json:"environment,omitempty"`
}

// PluginExecuteResult contains the results of plugin execution.
type PluginExecuteResult struct {
	Hook      string             `json:"hook"`
	Executed  int                `json:"executed"`
	Succeeded int                `json:"succeeded"`
	Failed    int                `json:"failed"`
	Results   []PluginHookResult `json:"results"`
}

// PluginHookResult contains the result of a single plugin hook execution.
type PluginHookResult struct {
	Success   bool           `json:"success"`
	Message   string         `json:"message,omitempty"`
	Error     string         `json:"error,omitempty"`
	Outputs   map[string]any `json:"outputs,omitempty"`
	Artifacts []Artifact     `json:"artifacts,omitempty"`
}

// Artifact represents a file or resource created by a plugin.
type Artifact struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Size     int64  `json:"size,omitempty"`
	Checksum string `json:"checksum,omitempty"`
}

func (s *PluginServer) toolPluginsExecute(ctx context.Context, args map[string]any) (*CallToolResult, error) {
	hookStr, ok := args["hook"].(string)
	if !ok || hookStr == "" {
		return NewToolResultError("hook is required"), nil
	}

	// Validate hook
	hook := plugin.Hook(hookStr)
	if !isValidHook(hook) {
		return NewToolResultError(fmt.Sprintf("invalid hook: %s", hookStr)), nil
	}

	s.mu.RLock()
	pm := s.pluginManager
	s.mu.RUnlock()

	if pm == nil {
		return NewToolResultError("plugin manager not initialized"), nil
	}

	// Build release context from args
	releaseCtx := plugin.ReleaseContext{}
	if ctxArg, ok := args["context"].(map[string]any); ok {
		if v, ok := ctxArg["version"].(string); ok {
			releaseCtx.Version = v
		}
		if v, ok := ctxArg["previous_version"].(string); ok {
			releaseCtx.PreviousVersion = v
		}
		if v, ok := ctxArg["tag_name"].(string); ok {
			releaseCtx.TagName = v
		}
		if v, ok := ctxArg["release_type"].(string); ok {
			releaseCtx.ReleaseType = v
		}
		if v, ok := ctxArg["repository_url"].(string); ok {
			releaseCtx.RepositoryURL = v
		}
		if v, ok := ctxArg["repository_owner"].(string); ok {
			releaseCtx.RepositoryOwner = v
		}
		if v, ok := ctxArg["repository_name"].(string); ok {
			releaseCtx.RepositoryName = v
		}
		if v, ok := ctxArg["branch"].(string); ok {
			releaseCtx.Branch = v
		}
		if v, ok := ctxArg["commit_sha"].(string); ok {
			releaseCtx.CommitSHA = v
		}
		if v, ok := ctxArg["changelog"].(string); ok {
			releaseCtx.Changelog = v
		}
		if v, ok := ctxArg["release_notes"].(string); ok {
			releaseCtx.ReleaseNotes = v
		}
		if v, ok := ctxArg["environment"].(map[string]any); ok {
			releaseCtx.Environment = make(map[string]string)
			for k, val := range v {
				if strVal, ok := val.(string); ok {
					releaseCtx.Environment[k] = strVal
				}
			}
		}
	}

	// Execute hook
	responses, err := pm.ExecuteHook(ctx, hook, releaseCtx)
	if err != nil {
		return NewToolResultError(fmt.Sprintf("hook execution failed: %v", err)), nil
	}

	// Build result
	result := PluginExecuteResult{
		Hook:     hookStr,
		Executed: len(responses),
		Results:  make([]PluginHookResult, 0, len(responses)),
	}

	for _, resp := range responses {
		hookResult := PluginHookResult{
			Success: resp.Success,
			Message: resp.Message,
			Error:   resp.Error,
			Outputs: resp.Outputs,
		}

		// Convert artifacts
		if len(resp.Artifacts) > 0 {
			hookResult.Artifacts = make([]Artifact, len(resp.Artifacts))
			for i, a := range resp.Artifacts {
				hookResult.Artifacts[i] = Artifact{
					Name:     a.Name,
					Path:     a.Path,
					Type:     a.Type,
					Size:     a.Size,
					Checksum: a.Checksum,
				}
			}
		}

		result.Results = append(result.Results, hookResult)

		if resp.Success {
			result.Succeeded++
		} else {
			result.Failed++
		}
	}

	return NewToolResultJSON(result)
}

// HooksListResult contains the list of available hooks.
type HooksListResult struct {
	Hooks []HookInfo `json:"hooks"`
	Count int        `json:"count"`
}

// HookInfo describes a hook.
type HookInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Phase       string `json:"phase"`
}

func (s *PluginServer) toolPluginsHooks(_ context.Context, _ map[string]any) (*CallToolResult, error) {
	hooks := []HookInfo{
		{Name: "pre-init", Description: "Runs before initialization", Phase: "init"},
		{Name: "post-init", Description: "Runs after initialization", Phase: "init"},
		{Name: "pre-plan", Description: "Runs before planning", Phase: "plan"},
		{Name: "post-plan", Description: "Runs after planning", Phase: "plan"},
		{Name: "pre-version", Description: "Runs before version bump", Phase: "version"},
		{Name: "post-version", Description: "Runs after version bump", Phase: "version"},
		{Name: "pre-notes", Description: "Runs before notes generation", Phase: "notes"},
		{Name: "post-notes", Description: "Runs after notes generation", Phase: "notes"},
		{Name: "pre-approve", Description: "Runs before approval", Phase: "approve"},
		{Name: "post-approve", Description: "Runs after approval", Phase: "approve"},
		{Name: "pre-publish", Description: "Runs before publishing", Phase: "publish"},
		{Name: "post-publish", Description: "Runs after publishing", Phase: "publish"},
		{Name: "on-success", Description: "Runs when release succeeds", Phase: "lifecycle"},
		{Name: "on-error", Description: "Runs when release fails", Phase: "lifecycle"},
	}

	return NewToolResultJSON(HooksListResult{
		Hooks: hooks,
		Count: len(hooks),
	})
}

func (s *PluginServer) resourcePlugins(_ context.Context, _ string) (*ReadResourceResult, error) {
	s.mu.RLock()
	pm := s.pluginManager
	s.mu.RUnlock()

	var result PluginListResult
	if pm != nil {
		infos := pm.ListPlugins()
		result.Plugins = make([]PluginInfoResult, 0, len(infos))
		result.Count = len(infos)

		for _, info := range infos {
			hooks := make([]string, len(info.Hooks))
			for i, h := range info.Hooks {
				hooks[i] = string(h)
			}

			result.Plugins = append(result.Plugins, PluginInfoResult{
				Name:         info.Name,
				Version:      info.Version,
				Description:  info.Description,
				Author:       info.Author,
				Hooks:        hooks,
				ConfigSchema: info.ConfigSchema,
			})
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}

	return &ReadResourceResult{
		Contents: []ResourceContent{
			{
				URI:      "relicta://plugins",
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

// isValidHook checks if a hook is valid.
func isValidHook(hook plugin.Hook) bool {
	validHooks := map[plugin.Hook]bool{
		plugin.HookPreInit:     true,
		plugin.HookPostInit:    true,
		plugin.HookPrePlan:     true,
		plugin.HookPostPlan:    true,
		plugin.HookPreVersion:  true,
		plugin.HookPostVersion: true,
		plugin.HookPreNotes:    true,
		plugin.HookPostNotes:   true,
		plugin.HookPreApprove:  true,
		plugin.HookPostApprove: true,
		plugin.HookPrePublish:  true,
		plugin.HookPostPublish: true,
		plugin.HookOnSuccess:   true,
		plugin.HookOnError:     true,
	}
	return validHooks[hook]
}

// PluginClient extends Client with plugin operations.
type PluginClient struct {
	*Client
}

// NewPluginClient creates a client with plugin support.
func NewPluginClient(transport ClientTransport, opts ...ClientOption) *PluginClient {
	return &PluginClient{
		Client: NewClient(transport, opts...),
	}
}

// ListPlugins lists all plugins.
func (c *PluginClient) ListPlugins(ctx context.Context) (*PluginListResult, error) {
	var result PluginListResult
	if err := c.CallToolTyped(ctx, "relicta.plugins.list", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetPluginInfo gets info for a specific plugin.
func (c *PluginClient) GetPluginInfo(ctx context.Context, name string) (*PluginInfoResult, error) {
	args := map[string]any{"name": name}

	var result PluginInfoResult
	if err := c.CallToolTyped(ctx, "relicta.plugins.info", args, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ExecuteHook executes a plugin hook.
func (c *PluginClient) ExecuteHook(ctx context.Context, hook string, releaseCtx *ReleaseContext) (*PluginExecuteResult, error) {
	args := map[string]any{
		"hook": hook,
	}

	if releaseCtx != nil {
		args["context"] = map[string]any{
			"version":          releaseCtx.Version,
			"previous_version": releaseCtx.PreviousVersion,
			"tag_name":         releaseCtx.TagName,
			"release_type":     releaseCtx.ReleaseType,
			"repository_url":   releaseCtx.RepositoryURL,
			"repository_owner": releaseCtx.RepositoryOwner,
			"repository_name":  releaseCtx.RepositoryName,
			"branch":           releaseCtx.Branch,
			"commit_sha":       releaseCtx.CommitSHA,
			"changelog":        releaseCtx.Changelog,
			"release_notes":    releaseCtx.ReleaseNotes,
			"environment":      releaseCtx.Environment,
		}
	}

	var result PluginExecuteResult
	if err := c.CallToolTyped(ctx, "relicta.plugins.execute", args, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListHooks lists available hooks.
func (c *PluginClient) ListHooks(ctx context.Context) (*HooksListResult, error) {
	var result HooksListResult
	if err := c.CallToolTyped(ctx, "relicta.plugins.hooks", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetPlugins reads the plugins resource.
func (c *PluginClient) GetPlugins(ctx context.Context) (*PluginListResult, error) {
	var result PluginListResult
	if err := c.ReadResourceTyped(ctx, "relicta://plugins", &result); err != nil {
		return nil, err
	}
	return &result, nil
}
