// Package plugin provides the public interface for Relicta plugins.
package plugin

import (
	"testing"
)

func TestAllHooks(t *testing.T) {
	hooks := AllHooks()

	// Verify all hooks are returned
	expectedHooks := []Hook{
		HookPreInit, HookPostInit,
		HookPrePlan, HookPostPlan,
		HookPreVersion, HookPostVersion,
		HookPreNotes, HookPostNotes,
		HookPreApprove, HookPostApprove,
		HookPrePublish, HookPostPublish,
		HookOnSuccess, HookOnError,
	}

	if len(hooks) != len(expectedHooks) {
		t.Errorf("AllHooks() returned %d hooks, want %d", len(hooks), len(expectedHooks))
	}

	for i, hook := range hooks {
		if hook != expectedHooks[i] {
			t.Errorf("AllHooks()[%d] = %v, want %v", i, hook, expectedHooks[i])
		}
	}
}

func TestAllHooks_Order(t *testing.T) {
	hooks := AllHooks()

	// Verify hooks are in execution order (pre before post)
	prePostPairs := [][2]Hook{
		{HookPreInit, HookPostInit},
		{HookPrePlan, HookPostPlan},
		{HookPreVersion, HookPostVersion},
		{HookPreNotes, HookPostNotes},
		{HookPreApprove, HookPostApprove},
		{HookPrePublish, HookPostPublish},
	}

	hookIndex := make(map[Hook]int)
	for i, h := range hooks {
		hookIndex[h] = i
	}

	for _, pair := range prePostPairs {
		preIdx := hookIndex[pair[0]]
		postIdx := hookIndex[pair[1]]
		if preIdx >= postIdx {
			t.Errorf("Hook %v should come before %v", pair[0], pair[1])
		}
	}
}

func TestHookConstants(t *testing.T) {
	tests := []struct {
		hook Hook
		want string
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
		t.Run(string(tt.hook), func(t *testing.T) {
			if string(tt.hook) != tt.want {
				t.Errorf("Hook constant = %v, want %v", tt.hook, tt.want)
			}
		})
	}
}

func TestInfo_Fields(t *testing.T) {
	info := Info{
		Name:         "test-plugin",
		Version:      "1.0.0",
		Description:  "A test plugin",
		Author:       "Test Author",
		Hooks:        []Hook{HookPostPublish, HookOnSuccess},
		ConfigSchema: `{"type": "object"}`,
	}

	if info.Name != "test-plugin" {
		t.Errorf("Info.Name = %v, want test-plugin", info.Name)
	}
	if info.Version != "1.0.0" {
		t.Errorf("Info.Version = %v, want 1.0.0", info.Version)
	}
	if info.Description != "A test plugin" {
		t.Errorf("Info.Description = %v, want A test plugin", info.Description)
	}
	if info.Author != "Test Author" {
		t.Errorf("Info.Author = %v, want Test Author", info.Author)
	}
	if len(info.Hooks) != 2 {
		t.Errorf("Info.Hooks length = %v, want 2", len(info.Hooks))
	}
	if info.ConfigSchema != `{"type": "object"}` {
		t.Errorf("Info.ConfigSchema = %v, want {\"type\": \"object\"}", info.ConfigSchema)
	}
}

func TestExecuteRequest_Fields(t *testing.T) {
	req := ExecuteRequest{
		Hook:   HookPostPublish,
		Config: map[string]any{"key": "value"},
		Context: ReleaseContext{
			Version: "1.0.0",
			TagName: "v1.0.0",
		},
		DryRun: true,
	}

	if req.Hook != HookPostPublish {
		t.Errorf("ExecuteRequest.Hook = %v, want post-publish", req.Hook)
	}
	if req.Config["key"] != "value" {
		t.Errorf("ExecuteRequest.Config[key] = %v, want value", req.Config["key"])
	}
	if req.Context.Version != "1.0.0" {
		t.Errorf("ExecuteRequest.Context.Version = %v, want 1.0.0", req.Context.Version)
	}
	if !req.DryRun {
		t.Error("ExecuteRequest.DryRun should be true")
	}
}

func TestExecuteResponse_Success(t *testing.T) {
	resp := ExecuteResponse{
		Success: true,
		Message: "Operation completed",
		Outputs: map[string]any{"url": "https://example.com"},
		Artifacts: []Artifact{
			{Name: "release.tar.gz", Path: "/tmp/release.tar.gz", Type: "file"},
		},
	}

	if !resp.Success {
		t.Error("ExecuteResponse.Success should be true")
	}
	if resp.Message != "Operation completed" {
		t.Errorf("ExecuteResponse.Message = %v, want Operation completed", resp.Message)
	}
	if resp.Outputs["url"] != "https://example.com" {
		t.Errorf("ExecuteResponse.Outputs[url] = %v, want https://example.com", resp.Outputs["url"])
	}
	if len(resp.Artifacts) != 1 {
		t.Errorf("ExecuteResponse.Artifacts length = %v, want 1", len(resp.Artifacts))
	}
}

func TestExecuteResponse_Error(t *testing.T) {
	resp := ExecuteResponse{
		Success: false,
		Error:   "Something went wrong",
	}

	if resp.Success {
		t.Error("ExecuteResponse.Success should be false")
	}
	if resp.Error != "Something went wrong" {
		t.Errorf("ExecuteResponse.Error = %v, want Something went wrong", resp.Error)
	}
}

func TestReleaseContext_AllFields(t *testing.T) {
	ctx := ReleaseContext{
		Version:         "2.0.0",
		PreviousVersion: "1.9.0",
		TagName:         "v2.0.0",
		ReleaseType:     "major",
		RepositoryURL:   "https://github.com/owner/repo",
		RepositoryOwner: "owner",
		RepositoryName:  "repo",
		Branch:          "main",
		CommitSHA:       "abc123def456",
		Changelog:       "## Changes\n- Feature 1",
		ReleaseNotes:    "New major version",
		Changes: &CategorizedChanges{
			Features: []ConventionalCommit{{Hash: "abc123", Type: "feat", Description: "new feature"}},
		},
		Environment: map[string]string{"CI": "true"},
	}

	if ctx.Version != "2.0.0" {
		t.Errorf("ReleaseContext.Version = %v, want 2.0.0", ctx.Version)
	}
	if ctx.PreviousVersion != "1.9.0" {
		t.Errorf("ReleaseContext.PreviousVersion = %v, want 1.9.0", ctx.PreviousVersion)
	}
	if ctx.TagName != "v2.0.0" {
		t.Errorf("ReleaseContext.TagName = %v, want v2.0.0", ctx.TagName)
	}
	if ctx.ReleaseType != "major" {
		t.Errorf("ReleaseContext.ReleaseType = %v, want major", ctx.ReleaseType)
	}
	if ctx.RepositoryURL != "https://github.com/owner/repo" {
		t.Errorf("ReleaseContext.RepositoryURL = %v, want https://github.com/owner/repo", ctx.RepositoryURL)
	}
	if ctx.RepositoryOwner != "owner" {
		t.Errorf("ReleaseContext.RepositoryOwner = %v, want owner", ctx.RepositoryOwner)
	}
	if ctx.RepositoryName != "repo" {
		t.Errorf("ReleaseContext.RepositoryName = %v, want repo", ctx.RepositoryName)
	}
	if ctx.Branch != "main" {
		t.Errorf("ReleaseContext.Branch = %v, want main", ctx.Branch)
	}
	if ctx.CommitSHA != "abc123def456" {
		t.Errorf("ReleaseContext.CommitSHA = %v, want abc123def456", ctx.CommitSHA)
	}
	if ctx.Changelog != "## Changes\n- Feature 1" {
		t.Errorf("ReleaseContext.Changelog = %v, want ## Changes\\n- Feature 1", ctx.Changelog)
	}
	if ctx.ReleaseNotes != "New major version" {
		t.Errorf("ReleaseContext.ReleaseNotes = %v, want New major version", ctx.ReleaseNotes)
	}
	if ctx.Changes == nil {
		t.Error("ReleaseContext.Changes should not be nil")
	}
	if ctx.Environment["CI"] != "true" {
		t.Errorf("ReleaseContext.Environment[CI] = %v, want true", ctx.Environment["CI"])
	}
}

func TestCategorizedChanges_AllCategories(t *testing.T) {
	changes := &CategorizedChanges{
		Features:    []ConventionalCommit{{Hash: "1", Type: "feat"}},
		Fixes:       []ConventionalCommit{{Hash: "2", Type: "fix"}},
		Breaking:    []ConventionalCommit{{Hash: "3", Type: "feat", Breaking: true}},
		Performance: []ConventionalCommit{{Hash: "4", Type: "perf"}},
		Refactor:    []ConventionalCommit{{Hash: "5", Type: "refactor"}},
		Docs:        []ConventionalCommit{{Hash: "6", Type: "docs"}},
		Other:       []ConventionalCommit{{Hash: "7", Type: "chore"}},
	}

	if len(changes.Features) != 1 {
		t.Errorf("CategorizedChanges.Features length = %v, want 1", len(changes.Features))
	}
	if len(changes.Fixes) != 1 {
		t.Errorf("CategorizedChanges.Fixes length = %v, want 1", len(changes.Fixes))
	}
	if len(changes.Breaking) != 1 {
		t.Errorf("CategorizedChanges.Breaking length = %v, want 1", len(changes.Breaking))
	}
	if len(changes.Performance) != 1 {
		t.Errorf("CategorizedChanges.Performance length = %v, want 1", len(changes.Performance))
	}
	if len(changes.Refactor) != 1 {
		t.Errorf("CategorizedChanges.Refactor length = %v, want 1", len(changes.Refactor))
	}
	if len(changes.Docs) != 1 {
		t.Errorf("CategorizedChanges.Docs length = %v, want 1", len(changes.Docs))
	}
	if len(changes.Other) != 1 {
		t.Errorf("CategorizedChanges.Other length = %v, want 1", len(changes.Other))
	}
}

func TestConventionalCommit_AllFields(t *testing.T) {
	commit := ConventionalCommit{
		Hash:                "abc123456789",
		Type:                "feat",
		Scope:               "api",
		Description:         "add new endpoint",
		Body:                "This adds a new endpoint for user management",
		Breaking:            true,
		BreakingDescription: "API signature changed",
		Issues:              []string{"#123", "#456"},
		Author:              "John Doe",
		Date:                "2024-01-15",
	}

	if commit.Hash != "abc123456789" {
		t.Errorf("ConventionalCommit.Hash = %v, want abc123456789", commit.Hash)
	}
	if commit.Type != "feat" {
		t.Errorf("ConventionalCommit.Type = %v, want feat", commit.Type)
	}
	if commit.Scope != "api" {
		t.Errorf("ConventionalCommit.Scope = %v, want api", commit.Scope)
	}
	if commit.Description != "add new endpoint" {
		t.Errorf("ConventionalCommit.Description = %v, want add new endpoint", commit.Description)
	}
	if commit.Body != "This adds a new endpoint for user management" {
		t.Errorf("ConventionalCommit.Body unexpected value")
	}
	if !commit.Breaking {
		t.Error("ConventionalCommit.Breaking should be true")
	}
	if commit.BreakingDescription != "API signature changed" {
		t.Errorf("ConventionalCommit.BreakingDescription = %v, want API signature changed", commit.BreakingDescription)
	}
	if len(commit.Issues) != 2 {
		t.Errorf("ConventionalCommit.Issues length = %v, want 2", len(commit.Issues))
	}
	if commit.Author != "John Doe" {
		t.Errorf("ConventionalCommit.Author = %v, want John Doe", commit.Author)
	}
	if commit.Date != "2024-01-15" {
		t.Errorf("ConventionalCommit.Date = %v, want 2024-01-15", commit.Date)
	}
}

func TestArtifact_AllFields(t *testing.T) {
	artifact := Artifact{
		Name:     "app-linux-amd64.tar.gz",
		Path:     "/tmp/dist/app-linux-amd64.tar.gz",
		Type:     "archive",
		Size:     1048576,
		Checksum: "sha256:abc123def456",
	}

	if artifact.Name != "app-linux-amd64.tar.gz" {
		t.Errorf("Artifact.Name = %v, want app-linux-amd64.tar.gz", artifact.Name)
	}
	if artifact.Path != "/tmp/dist/app-linux-amd64.tar.gz" {
		t.Errorf("Artifact.Path = %v, want /tmp/dist/app-linux-amd64.tar.gz", artifact.Path)
	}
	if artifact.Type != "archive" {
		t.Errorf("Artifact.Type = %v, want archive", artifact.Type)
	}
	if artifact.Size != 1048576 {
		t.Errorf("Artifact.Size = %v, want 1048576", artifact.Size)
	}
	if artifact.Checksum != "sha256:abc123def456" {
		t.Errorf("Artifact.Checksum = %v, want sha256:abc123def456", artifact.Checksum)
	}
}

func TestValidateResponse_Valid(t *testing.T) {
	resp := ValidateResponse{
		Valid:  true,
		Errors: nil,
	}

	if !resp.Valid {
		t.Error("ValidateResponse.Valid should be true")
	}
	if resp.Errors != nil {
		t.Error("ValidateResponse.Errors should be nil for valid response")
	}
}

func TestValidateResponse_Invalid(t *testing.T) {
	resp := ValidateResponse{
		Valid: false,
		Errors: []ValidationError{
			{Field: "api_token", Message: "required field", Code: "REQUIRED"},
			{Field: "webhook_url", Message: "invalid URL format", Code: "INVALID_FORMAT"},
		},
	}

	if resp.Valid {
		t.Error("ValidateResponse.Valid should be false")
	}
	if len(resp.Errors) != 2 {
		t.Errorf("ValidateResponse.Errors length = %v, want 2", len(resp.Errors))
	}

	// Check first error
	if resp.Errors[0].Field != "api_token" {
		t.Errorf("ValidationError[0].Field = %v, want api_token", resp.Errors[0].Field)
	}
	if resp.Errors[0].Message != "required field" {
		t.Errorf("ValidationError[0].Message = %v, want required field", resp.Errors[0].Message)
	}
	if resp.Errors[0].Code != "REQUIRED" {
		t.Errorf("ValidationError[0].Code = %v, want REQUIRED", resp.Errors[0].Code)
	}

	// Check second error
	if resp.Errors[1].Field != "webhook_url" {
		t.Errorf("ValidationError[1].Field = %v, want webhook_url", resp.Errors[1].Field)
	}
}

func TestValidationError_Fields(t *testing.T) {
	err := ValidationError{
		Field:   "timeout",
		Message: "must be a positive integer",
		Code:    "POSITIVE_INT",
	}

	if err.Field != "timeout" {
		t.Errorf("ValidationError.Field = %v, want timeout", err.Field)
	}
	if err.Message != "must be a positive integer" {
		t.Errorf("ValidationError.Message = %v, want must be a positive integer", err.Message)
	}
	if err.Code != "POSITIVE_INT" {
		t.Errorf("ValidationError.Code = %v, want POSITIVE_INT", err.Code)
	}
}
