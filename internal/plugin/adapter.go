// Package plugin provides plugin management for Relicta.
package plugin

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/integration"
	"github.com/relicta-tech/relicta/pkg/plugin"
)

// ExecutorAdapter adapts the Manager to the integration.PluginExecutor interface.
// This bridges the gap between the internal domain types and the public plugin API.
type ExecutorAdapter struct {
	manager *Manager
}

// NewExecutorAdapter creates a new adapter that wraps a Manager.
func NewExecutorAdapter(manager *Manager) *ExecutorAdapter {
	return &ExecutorAdapter{manager: manager}
}

// ExecuteHook executes all plugins for a given hook, adapting between domain and plugin types.
func (a *ExecutorAdapter) ExecuteHook(ctx context.Context, hook integration.Hook, releaseCtx integration.ReleaseContext) ([]integration.ExecuteResponse, error) {
	// Convert domain hook to plugin hook
	pluginHook := plugin.Hook(hook)

	// Convert domain context to plugin context
	pluginCtx := toPluginReleaseContext(releaseCtx)

	// Execute via manager
	responses, err := a.manager.ExecuteHook(ctx, pluginHook, pluginCtx)
	if err != nil {
		return nil, err
	}

	// Convert responses back to domain types
	return toIntegrationResponses(responses), nil
}

// ExecutePlugin executes a specific plugin.
// Note: Individual plugin execution by ID is not currently supported.
// Use ExecuteHook to execute all plugins for a given hook instead.
func (a *ExecutorAdapter) ExecutePlugin(ctx context.Context, id integration.PluginID, req integration.ExecuteRequest) (*integration.ExecuteResponse, error) {
	return nil, fmt.Errorf("individual plugin execution by ID is not supported; use ExecuteHook instead")
}

// toPluginReleaseContext converts domain ReleaseContext to plugin ReleaseContext.
func toPluginReleaseContext(ctx integration.ReleaseContext) plugin.ReleaseContext {
	result := plugin.ReleaseContext{
		Version:         ctx.Version.String(),
		PreviousVersion: ctx.PreviousVersion.String(),
		ReleaseType:     ctx.ReleaseType.String(),
		RepositoryOwner: ctx.RepositoryOwner,
		RepositoryName:  ctx.RepositoryName,
		Branch:          ctx.Branch,
		TagName:         ctx.TagName,
		Changelog:       ctx.Changelog,
		ReleaseNotes:    ctx.ReleaseNotes,
	}

	// Convert changes if present
	if ctx.Changes != nil {
		result.Changes = toCategorizedChanges(ctx.Changes)
	}

	return result
}

// toCategorizedChanges converts a ChangeSet to plugin CategorizedChanges.
// Note: Plugin API has fewer categories than domain, so Tests, Build, CI, Chores, Reverts
// are merged into the Other category.
func toCategorizedChanges(cs *changes.ChangeSet) *plugin.CategorizedChanges {
	if cs == nil {
		return nil
	}

	cats := cs.Categories()

	// Merge categories that don't have a direct mapping in plugin API into Other
	// Pre-allocate to avoid repeated slice growth during appends
	otherCount := len(cats.Tests) + len(cats.Build) + len(cats.CI) +
		len(cats.Chores) + len(cats.Reverts) + len(cats.Other)
	other := make([]*changes.ConventionalCommit, 0, otherCount)
	other = append(other, cats.Tests...)
	other = append(other, cats.Build...)
	other = append(other, cats.CI...)
	other = append(other, cats.Chores...)
	other = append(other, cats.Reverts...)
	other = append(other, cats.Other...)

	return &plugin.CategorizedChanges{
		Features:    toPluginCommits(cats.Features),
		Fixes:       toPluginCommits(cats.Fixes),
		Breaking:    toPluginCommits(cats.Breaking),
		Performance: toPluginCommits(cats.Perf),
		Refactor:    toPluginCommits(cats.Refactors),
		Docs:        toPluginCommits(cats.Docs),
		Other:       toPluginCommits(other),
	}
}

// toPluginCommits converts domain commits to plugin commits.
func toPluginCommits(commits []*changes.ConventionalCommit) []plugin.ConventionalCommit {
	result := make([]plugin.ConventionalCommit, len(commits))
	for i, c := range commits {
		result[i] = plugin.ConventionalCommit{
			Hash:                c.Hash(),
			Type:                c.Type().String(),
			Scope:               c.Scope(),
			Description:         c.Subject(),
			Body:                c.Body(),
			Breaking:            c.IsBreaking(),
			BreakingDescription: c.BreakingMessage(),
			Issues:              nil, // Domain commits don't track issue references yet
			Author:              c.Author(),
			Date:                c.Date().Format("2006-01-02"),
		}
	}
	return result
}

// toIntegrationResponses converts plugin responses to domain responses.
func toIntegrationResponses(responses []plugin.ExecuteResponse) []integration.ExecuteResponse {
	result := make([]integration.ExecuteResponse, len(responses))
	for i, r := range responses {
		result[i] = integration.ExecuteResponse{
			Success: r.Success,
			Message: r.Message,
			Error:   r.Error,
			Outputs: r.Outputs,
		}

		// Convert artifacts if present
		if len(r.Artifacts) > 0 {
			result[i].Artifacts = make([]integration.Artifact, len(r.Artifacts))
			for j, a := range r.Artifacts {
				result[i].Artifacts[j] = integration.Artifact{
					Name: a.Name,
					Path: a.Path,
					Type: a.Type,
					Size: a.Size,
				}
			}
		}
	}
	return result
}

// toIntegrationResponse converts a single plugin response to domain response.
func toIntegrationResponse(r *plugin.ExecuteResponse) *integration.ExecuteResponse {
	if r == nil {
		return nil
	}

	result := &integration.ExecuteResponse{
		Success: r.Success,
		Message: r.Message,
		Error:   r.Error,
		Outputs: r.Outputs,
	}

	// Convert artifacts if present
	if len(r.Artifacts) > 0 {
		result.Artifacts = make([]integration.Artifact, len(r.Artifacts))
		for i, a := range r.Artifacts {
			result.Artifacts[i] = integration.Artifact{
				Name: a.Name,
				Path: a.Path,
				Type: a.Type,
				Size: a.Size,
			}
		}
	}

	return result
}
