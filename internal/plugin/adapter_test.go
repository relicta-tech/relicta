// Package plugin provides plugin management for ReleasePilot.
package plugin

import (
	"context"
	"strings"
	"testing"

	"github.com/felixgeelhaar/release-pilot/internal/config"
	"github.com/felixgeelhaar/release-pilot/internal/domain/changes"
	"github.com/felixgeelhaar/release-pilot/internal/domain/integration"
	"github.com/felixgeelhaar/release-pilot/internal/domain/version"
	"github.com/felixgeelhaar/release-pilot/pkg/plugin"
)

func TestNewExecutorAdapter(t *testing.T) {
	manager := &Manager{}
	adapter := NewExecutorAdapter(manager)

	if adapter == nil {
		t.Fatal("Expected adapter to be created")
	}
	if adapter.manager != manager {
		t.Error("Expected adapter to wrap the manager")
	}
}

func TestToPluginReleaseContext(t *testing.T) {
	currentVer := version.MustParse("1.0.0")
	nextVer := version.MustParse("1.1.0")

	ctx := integration.ReleaseContext{
		Version:         nextVer,
		PreviousVersion: currentVer,
		ReleaseType:     changes.ReleaseTypeMinor,
		RepositoryOwner: "owner",
		RepositoryName:  "repo",
		Branch:          "main",
		TagName:         "v1.1.0",
		Changelog:       "## Changes\n- Feature 1",
		ReleaseNotes:    "New release",
	}

	result := toPluginReleaseContext(ctx)

	if result.Version != "1.1.0" {
		t.Errorf("Version = %v, want 1.1.0", result.Version)
	}
	if result.PreviousVersion != "1.0.0" {
		t.Errorf("PreviousVersion = %v, want 1.0.0", result.PreviousVersion)
	}
	if result.ReleaseType != "minor" {
		t.Errorf("ReleaseType = %v, want minor", result.ReleaseType)
	}
	if result.RepositoryOwner != "owner" {
		t.Errorf("RepositoryOwner = %v, want owner", result.RepositoryOwner)
	}
	if result.RepositoryName != "repo" {
		t.Errorf("RepositoryName = %v, want repo", result.RepositoryName)
	}
	if result.Branch != "main" {
		t.Errorf("Branch = %v, want main", result.Branch)
	}
	if result.TagName != "v1.1.0" {
		t.Errorf("TagName = %v, want v1.1.0", result.TagName)
	}
	if result.Changelog != "## Changes\n- Feature 1" {
		t.Errorf("Changelog = %v, want ## Changes\\n- Feature 1", result.Changelog)
	}
	if result.ReleaseNotes != "New release" {
		t.Errorf("ReleaseNotes = %v, want New release", result.ReleaseNotes)
	}
	if result.Changes != nil {
		t.Error("Changes should be nil when no ChangeSet provided")
	}
}

func TestToPluginReleaseContext_WithChanges(t *testing.T) {
	currentVer := version.MustParse("1.0.0")
	nextVer := version.MustParse("2.0.0")

	// Create test commits
	commit1 := createTestCommit("abc123", changes.CommitTypeFeat, "api", "add new endpoint", false)
	commit2 := createTestCommit("def456", changes.CommitTypeFix, "", "fix bug", false)
	commit3 := createTestCommit("ghi789", changes.CommitTypeFeat, "", "breaking change", true)

	changeSet := changes.NewChangeSet("test-cs", "v1.0.0", "HEAD")
	changeSet.AddCommits([]*changes.ConventionalCommit{commit1, commit2, commit3})

	ctx := integration.ReleaseContext{
		Version:         nextVer,
		PreviousVersion: currentVer,
		ReleaseType:     changes.ReleaseTypeMajor,
		RepositoryOwner: "owner",
		RepositoryName:  "repo",
		Branch:          "main",
		TagName:         "v2.0.0",
		Changes:         changeSet,
	}

	result := toPluginReleaseContext(ctx)

	if result.Changes == nil {
		t.Fatal("Changes should not be nil")
	}
	if len(result.Changes.Features) != 2 {
		t.Errorf("Features count = %d, want 2", len(result.Changes.Features))
	}
	if len(result.Changes.Fixes) != 1 {
		t.Errorf("Fixes count = %d, want 1", len(result.Changes.Fixes))
	}
	if len(result.Changes.Breaking) != 1 {
		t.Errorf("Breaking count = %d, want 1", len(result.Changes.Breaking))
	}
}

func TestToCategorizedChanges_Nil(t *testing.T) {
	result := toCategorizedChanges(nil)
	if result != nil {
		t.Error("Expected nil result for nil input")
	}
}

func TestToCategorizedChanges_WithCommits(t *testing.T) {
	feat := createTestCommit("abc123", changes.CommitTypeFeat, "ui", "add button", false)
	fix := createTestCommit("def456", changes.CommitTypeFix, "", "fix crash", false)
	perf := createTestCommit("ghi789", changes.CommitTypePerf, "", "improve speed", false)
	refactor := createTestCommit("jkl012", changes.CommitTypeRefactor, "", "clean up code", false)
	docs := createTestCommit("mno345", changes.CommitTypeDocs, "", "update readme", false)
	chore := createTestCommit("pqr678", changes.CommitTypeChore, "", "update deps", false)

	changeSet := changes.NewChangeSet("test-cs", "v1.0.0", "HEAD")
	changeSet.AddCommits([]*changes.ConventionalCommit{feat, fix, perf, refactor, docs, chore})

	result := toCategorizedChanges(changeSet)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if len(result.Features) != 1 {
		t.Errorf("Features count = %d, want 1", len(result.Features))
	}
	if len(result.Fixes) != 1 {
		t.Errorf("Fixes count = %d, want 1", len(result.Fixes))
	}
	if len(result.Performance) != 1 {
		t.Errorf("Performance count = %d, want 1", len(result.Performance))
	}
	if len(result.Refactor) != 1 {
		t.Errorf("Refactor count = %d, want 1", len(result.Refactor))
	}
	if len(result.Docs) != 1 {
		t.Errorf("Docs count = %d, want 1", len(result.Docs))
	}
	if len(result.Other) != 1 {
		t.Errorf("Other count = %d, want 1", len(result.Other))
	}
}

func TestToPluginCommits(t *testing.T) {
	commit := createTestCommit("abc123456", changes.CommitTypeFeat, "api", "add user endpoint", false)

	result := toPluginCommits([]*changes.ConventionalCommit{commit})

	if len(result) != 1 {
		t.Fatalf("Expected 1 commit, got %d", len(result))
	}

	pc := result[0]
	if pc.Hash != "abc123456" {
		t.Errorf("Hash = %v, want abc123456", pc.Hash)
	}
	if pc.Type != "feat" {
		t.Errorf("Type = %v, want feat", pc.Type)
	}
	if pc.Scope != "api" {
		t.Errorf("Scope = %v, want api", pc.Scope)
	}
	if pc.Description != "add user endpoint" {
		t.Errorf("Description = %v, want add user endpoint", pc.Description)
	}
	if pc.Breaking != false {
		t.Errorf("Breaking = %v, want false", pc.Breaking)
	}
}

func TestToPluginCommits_Breaking(t *testing.T) {
	commit := createTestCommit("def456", changes.CommitTypeFeat, "", "breaking change", true)

	result := toPluginCommits([]*changes.ConventionalCommit{commit})

	if len(result) != 1 {
		t.Fatalf("Expected 1 commit, got %d", len(result))
	}

	pc := result[0]
	if !pc.Breaking {
		t.Error("Expected Breaking to be true")
	}
}

func TestToPluginCommits_Empty(t *testing.T) {
	result := toPluginCommits([]*changes.ConventionalCommit{})
	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d items", len(result))
	}
}

func TestToIntegrationResponses(t *testing.T) {
	responses := []plugin.ExecuteResponse{
		{
			Success: true,
			Message: "Success message",
			Outputs: map[string]any{"key": "value"},
		},
		{
			Success: false,
			Error:   "Something failed",
		},
	}

	result := toIntegrationResponses(responses)

	if len(result) != 2 {
		t.Fatalf("Expected 2 responses, got %d", len(result))
	}

	if !result[0].Success {
		t.Error("First response should be successful")
	}
	if result[0].Message != "Success message" {
		t.Errorf("Message = %v, want Success message", result[0].Message)
	}
	if result[0].Outputs["key"] != "value" {
		t.Errorf("Outputs[key] = %v, want value", result[0].Outputs["key"])
	}

	if result[1].Success {
		t.Error("Second response should not be successful")
	}
	if result[1].Error != "Something failed" {
		t.Errorf("Error = %v, want Something failed", result[1].Error)
	}
}

func TestToIntegrationResponses_WithArtifacts(t *testing.T) {
	responses := []plugin.ExecuteResponse{
		{
			Success: true,
			Artifacts: []plugin.Artifact{
				{
					Name: "release.tar.gz",
					Path: "/tmp/release.tar.gz",
					Type: "archive",
					Size: 1024,
				},
			},
		},
	}

	result := toIntegrationResponses(responses)

	if len(result) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(result))
	}
	if len(result[0].Artifacts) != 1 {
		t.Fatalf("Expected 1 artifact, got %d", len(result[0].Artifacts))
	}

	artifact := result[0].Artifacts[0]
	if artifact.Name != "release.tar.gz" {
		t.Errorf("Name = %v, want release.tar.gz", artifact.Name)
	}
	if artifact.Path != "/tmp/release.tar.gz" {
		t.Errorf("Path = %v, want /tmp/release.tar.gz", artifact.Path)
	}
	if artifact.Type != "archive" {
		t.Errorf("Type = %v, want archive", artifact.Type)
	}
	if artifact.Size != 1024 {
		t.Errorf("Size = %v, want 1024", artifact.Size)
	}
}

func TestToIntegrationResponse_Nil(t *testing.T) {
	result := toIntegrationResponse(nil)
	if result != nil {
		t.Error("Expected nil result for nil input")
	}
}

func TestToIntegrationResponse_Success(t *testing.T) {
	response := &plugin.ExecuteResponse{
		Success: true,
		Message: "Done",
		Outputs: map[string]any{"url": "https://example.com"},
	}

	result := toIntegrationResponse(response)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if !result.Success {
		t.Error("Expected success to be true")
	}
	if result.Message != "Done" {
		t.Errorf("Message = %v, want Done", result.Message)
	}
	if result.Outputs["url"] != "https://example.com" {
		t.Errorf("Outputs[url] = %v, want https://example.com", result.Outputs["url"])
	}
}

func TestExecutorAdapter_ExecutePlugin_NotSupported(t *testing.T) {
	adapter := NewExecutorAdapter(&Manager{})

	result, err := adapter.ExecutePlugin(context.Background(), integration.PluginID("test"), integration.ExecuteRequest{})

	if err == nil {
		t.Error("Expected error for unsupported method")
	}
	if result != nil {
		t.Error("Expected nil result for unsupported method")
	}
	expectedMsg := "individual plugin execution by ID is not supported"
	if err != nil && !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error to contain %q, got %v", expectedMsg, err)
	}
}

// createTestCommit creates a test conventional commit using changes.NewConventionalCommit.
func createTestCommit(hash string, commitType changes.CommitType, scope, subject string, breaking bool) *changes.ConventionalCommit {
	var opts []changes.ConventionalCommitOption
	if scope != "" {
		opts = append(opts, changes.WithScope(scope))
	}
	if breaking {
		opts = append(opts, changes.WithBreaking("breaking change"))
	}
	return changes.NewConventionalCommit(hash, commitType, subject, opts...)
}

func TestExecutorAdapter_ExecuteHook_NoPlugins(t *testing.T) {
	// Create a manager with no plugins loaded
	m := &Manager{
		plugins: make(map[string]*loadedPlugin),
	}
	adapter := NewExecutorAdapter(m)

	ctx := context.Background()
	releaseCtx := integration.ReleaseContext{
		Version: version.MustParse("1.0.0"),
	}

	results, err := adapter.ExecuteHook(ctx, integration.HookPostPublish, releaseCtx)
	if err != nil {
		t.Errorf("ExecuteHook() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestToIntegrationResponse_EmptyArtifacts(t *testing.T) {
	response := &plugin.ExecuteResponse{
		Success:   true,
		Message:   "Done",
		Artifacts: []plugin.Artifact{},
	}

	result := toIntegrationResponse(response)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if len(result.Artifacts) != 0 {
		t.Errorf("Expected 0 artifacts, got %d", len(result.Artifacts))
	}
}

func TestExecutorAdapter_ExecuteHook_WithError(t *testing.T) {
	// Create a manager that will return an error
	m := &Manager{
		plugins: make(map[string]*loadedPlugin),
		cfg: &config.Config{
			Workflow: config.WorkflowConfig{
				DryRunByDefault: false,
			},
		},
	}
	adapter := NewExecutorAdapter(m)

	ctx := context.Background()
	releaseCtx := integration.ReleaseContext{
		Version: version.MustParse("1.0.0"),
	}

	// Execute hook - should succeed with no plugins
	results, err := adapter.ExecuteHook(ctx, integration.HookPostPublish, releaseCtx)
	if err != nil {
		t.Errorf("ExecuteHook() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestToIntegrationResponse_WithOutputs(t *testing.T) {
	response := &plugin.ExecuteResponse{
		Success: true,
		Message: "Success",
		Outputs: map[string]any{
			"url":    "https://example.com",
			"status": "published",
			"count":  42,
		},
	}

	result := toIntegrationResponse(response)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Outputs["url"] != "https://example.com" {
		t.Errorf("Outputs[url] = %v, want https://example.com", result.Outputs["url"])
	}
	if result.Outputs["status"] != "published" {
		t.Errorf("Outputs[status] = %v, want published", result.Outputs["status"])
	}
	if result.Outputs["count"] != 42 {
		t.Errorf("Outputs[count] = %v, want 42", result.Outputs["count"])
	}
}

func TestToIntegrationResponse_WithMultipleArtifacts(t *testing.T) {
	response := &plugin.ExecuteResponse{
		Success: true,
		Message: "Success",
		Artifacts: []plugin.Artifact{
			{
				Name: "file1.txt",
				Path: "/path/to/file1.txt",
				Type: "text",
				Size: 100,
			},
			{
				Name: "file2.zip",
				Path: "/path/to/file2.zip",
				Type: "archive",
				Size: 2048,
			},
			{
				Name: "file3.jpg",
				Path: "/path/to/file3.jpg",
				Type: "image",
				Size: 5120,
			},
		},
	}

	result := toIntegrationResponse(response)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if len(result.Artifacts) != 3 {
		t.Fatalf("Expected 3 artifacts, got %d", len(result.Artifacts))
	}

	// Check first artifact
	if result.Artifacts[0].Name != "file1.txt" {
		t.Errorf("Artifact[0].Name = %v, want file1.txt", result.Artifacts[0].Name)
	}
	if result.Artifacts[0].Type != "text" {
		t.Errorf("Artifact[0].Type = %v, want text", result.Artifacts[0].Type)
	}

	// Check second artifact
	if result.Artifacts[1].Size != 2048 {
		t.Errorf("Artifact[1].Size = %v, want 2048", result.Artifacts[1].Size)
	}

	// Check third artifact
	if result.Artifacts[2].Path != "/path/to/file3.jpg" {
		t.Errorf("Artifact[2].Path = %v, want /path/to/file3.jpg", result.Artifacts[2].Path)
	}
}

func TestToIntegrationResponse_FailureWithError(t *testing.T) {
	response := &plugin.ExecuteResponse{
		Success: false,
		Error:   "Plugin execution failed due to network error",
	}

	result := toIntegrationResponse(response)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Success {
		t.Error("Expected Success to be false")
	}
	if result.Error != "Plugin execution failed due to network error" {
		t.Errorf("Error = %v, want 'Plugin execution failed due to network error'", result.Error)
	}
}

func TestToIntegrationResponses_Mixed(t *testing.T) {
	responses := []plugin.ExecuteResponse{
		{
			Success: true,
			Message: "Success 1",
		},
		{
			Success: false,
			Error:   "Error 1",
		},
		{
			Success: true,
			Message: "Success 2",
			Artifacts: []plugin.Artifact{
				{Name: "test.txt", Path: "/test.txt", Type: "text", Size: 42},
			},
		},
	}

	result := toIntegrationResponses(responses)

	if len(result) != 3 {
		t.Fatalf("Expected 3 responses, got %d", len(result))
	}
	if !result[0].Success {
		t.Error("First response should be successful")
	}
	if result[1].Success {
		t.Error("Second response should be unsuccessful")
	}
	if result[1].Error != "Error 1" {
		t.Errorf("Second response error = %v, want 'Error 1'", result[1].Error)
	}
	if len(result[2].Artifacts) != 1 {
		t.Errorf("Third response should have 1 artifact, got %d", len(result[2].Artifacts))
	}
}
