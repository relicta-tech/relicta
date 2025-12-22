// Package plugin provides the public interface for Relicta plugins.
package plugin

import (
	"context"
	"testing"
)

func TestHookProtoConstants(t *testing.T) {
	tests := []struct {
		name  string
		hook  HookProto
		value int32
	}{
		{"HOOK_UNSPECIFIED", HookProto_HOOK_UNSPECIFIED, 0},
		{"HOOK_PRE_INIT", HookProto_HOOK_PRE_INIT, 1},
		{"HOOK_POST_INIT", HookProto_HOOK_POST_INIT, 2},
		{"HOOK_PRE_PLAN", HookProto_HOOK_PRE_PLAN, 3},
		{"HOOK_POST_PLAN", HookProto_HOOK_POST_PLAN, 4},
		{"HOOK_PRE_VERSION", HookProto_HOOK_PRE_VERSION, 5},
		{"HOOK_POST_VERSION", HookProto_HOOK_POST_VERSION, 6},
		{"HOOK_PRE_NOTES", HookProto_HOOK_PRE_NOTES, 7},
		{"HOOK_POST_NOTES", HookProto_HOOK_POST_NOTES, 8},
		{"HOOK_PRE_APPROVE", HookProto_HOOK_PRE_APPROVE, 9},
		{"HOOK_POST_APPROVE", HookProto_HOOK_POST_APPROVE, 10},
		{"HOOK_PRE_PUBLISH", HookProto_HOOK_PRE_PUBLISH, 11},
		{"HOOK_POST_PUBLISH", HookProto_HOOK_POST_PUBLISH, 12},
		{"HOOK_ON_SUCCESS", HookProto_HOOK_ON_SUCCESS, 13},
		{"HOOK_ON_ERROR", HookProto_HOOK_ON_ERROR, 14},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int32(tt.hook) != tt.value {
				t.Errorf("HookProto %s = %d, want %d", tt.name, tt.hook, tt.value)
			}
		})
	}
}

func TestEmpty(t *testing.T) {
	// Verify Empty struct can be instantiated
	e := Empty{}
	_ = e // Just ensure it compiles
}

func TestPluginInfo_Fields(t *testing.T) {
	info := PluginInfo{
		Name:         "test",
		Version:      "1.0.0",
		Description:  "Test plugin",
		Author:       "Test Author",
		Hooks:        []string{"post-publish"},
		ConfigSchema: `{"type": "object"}`,
	}

	if info.Name != "test" {
		t.Errorf("PluginInfo.Name = %v, want test", info.Name)
	}
	if info.Version != "1.0.0" {
		t.Errorf("PluginInfo.Version = %v, want 1.0.0", info.Version)
	}
	if info.Description != "Test plugin" {
		t.Errorf("PluginInfo.Description = %v, want Test plugin", info.Description)
	}
	if info.Author != "Test Author" {
		t.Errorf("PluginInfo.Author = %v, want Test Author", info.Author)
	}
	if len(info.Hooks) != 1 {
		t.Errorf("PluginInfo.Hooks length = %v, want 1", len(info.Hooks))
	}
}

func TestExecuteRequestProto_Fields(t *testing.T) {
	req := ExecuteRequestProto{
		Hook:   HookProto_HOOK_POST_PUBLISH,
		Config: `{"key": "value"}`,
		Context: &ReleaseContextProto{
			Version: "1.0.0",
			TagName: "v1.0.0",
		},
		DryRun: true,
	}

	if req.Hook != HookProto_HOOK_POST_PUBLISH {
		t.Errorf("ExecuteRequestProto.Hook = %v, want HOOK_POST_PUBLISH", req.Hook)
	}
	if req.Config != `{"key": "value"}` {
		t.Errorf("ExecuteRequestProto.Config = %v, want {\"key\": \"value\"}", req.Config)
	}
	if req.Context == nil {
		t.Error("ExecuteRequestProto.Context should not be nil")
	}
	if !req.DryRun {
		t.Error("ExecuteRequestProto.DryRun should be true")
	}
}

func TestExecuteResponseProto_Success(t *testing.T) {
	resp := ExecuteResponseProto{
		Success: true,
		Message: "Done",
		Outputs: `{"url": "https://example.com"}`,
		Artifacts: []*ArtifactProto{
			{Name: "release.tar.gz", Path: "/tmp/release.tar.gz", Type: "file"},
		},
	}

	if !resp.Success {
		t.Error("ExecuteResponseProto.Success should be true")
	}
	if resp.Message != "Done" {
		t.Errorf("ExecuteResponseProto.Message = %v, want Done", resp.Message)
	}
	if len(resp.Artifacts) != 1 {
		t.Errorf("ExecuteResponseProto.Artifacts length = %v, want 1", len(resp.Artifacts))
	}
}

func TestExecuteResponseProto_Error(t *testing.T) {
	resp := ExecuteResponseProto{
		Success: false,
		Error:   "Failed to execute",
	}

	if resp.Success {
		t.Error("ExecuteResponseProto.Success should be false")
	}
	if resp.Error != "Failed to execute" {
		t.Errorf("ExecuteResponseProto.Error = %v, want Failed to execute", resp.Error)
	}
}

func TestReleaseContextProto_AllFields(t *testing.T) {
	ctx := ReleaseContextProto{
		Version:         "1.0.0",
		PreviousVersion: "0.9.0",
		TagName:         "v1.0.0",
		ReleaseType:     "minor",
		RepositoryUrl:   "https://github.com/owner/repo",
		RepositoryOwner: "owner",
		RepositoryName:  "repo",
		Branch:          "main",
		CommitSha:       "abc123",
		Changelog:       "## Changes",
		ReleaseNotes:    "New release",
		Changes:         &CategorizedChangesProto{},
		Environment:     map[string]string{"CI": "true"},
	}

	if ctx.Version != "1.0.0" {
		t.Errorf("ReleaseContextProto.Version = %v, want 1.0.0", ctx.Version)
	}
	if ctx.PreviousVersion != "0.9.0" {
		t.Errorf("ReleaseContextProto.PreviousVersion = %v, want 0.9.0", ctx.PreviousVersion)
	}
	if ctx.TagName != "v1.0.0" {
		t.Errorf("ReleaseContextProto.TagName = %v, want v1.0.0", ctx.TagName)
	}
	if ctx.ReleaseType != "minor" {
		t.Errorf("ReleaseContextProto.ReleaseType = %v, want minor", ctx.ReleaseType)
	}
	if ctx.RepositoryUrl != "https://github.com/owner/repo" {
		t.Errorf("ReleaseContextProto.RepositoryUrl unexpected value")
	}
	if ctx.RepositoryOwner != "owner" {
		t.Errorf("ReleaseContextProto.RepositoryOwner = %v, want owner", ctx.RepositoryOwner)
	}
	if ctx.RepositoryName != "repo" {
		t.Errorf("ReleaseContextProto.RepositoryName = %v, want repo", ctx.RepositoryName)
	}
	if ctx.Branch != "main" {
		t.Errorf("ReleaseContextProto.Branch = %v, want main", ctx.Branch)
	}
	if ctx.CommitSha != "abc123" {
		t.Errorf("ReleaseContextProto.CommitSha = %v, want abc123", ctx.CommitSha)
	}
	if ctx.Environment["CI"] != "true" {
		t.Errorf("ReleaseContextProto.Environment[CI] = %v, want true", ctx.Environment["CI"])
	}
}

func TestCategorizedChangesProto_AllCategories(t *testing.T) {
	changes := CategorizedChangesProto{
		Features:    []*ConventionalCommitProto{{Hash: "1"}},
		Fixes:       []*ConventionalCommitProto{{Hash: "2"}},
		Breaking:    []*ConventionalCommitProto{{Hash: "3"}},
		Performance: []*ConventionalCommitProto{{Hash: "4"}},
		Refactor:    []*ConventionalCommitProto{{Hash: "5"}},
		Docs:        []*ConventionalCommitProto{{Hash: "6"}},
		Other:       []*ConventionalCommitProto{{Hash: "7"}},
	}

	if len(changes.Features) != 1 {
		t.Errorf("CategorizedChangesProto.Features length = %v, want 1", len(changes.Features))
	}
	if len(changes.Fixes) != 1 {
		t.Errorf("CategorizedChangesProto.Fixes length = %v, want 1", len(changes.Fixes))
	}
	if len(changes.Breaking) != 1 {
		t.Errorf("CategorizedChangesProto.Breaking length = %v, want 1", len(changes.Breaking))
	}
	if len(changes.Performance) != 1 {
		t.Errorf("CategorizedChangesProto.Performance length = %v, want 1", len(changes.Performance))
	}
	if len(changes.Refactor) != 1 {
		t.Errorf("CategorizedChangesProto.Refactor length = %v, want 1", len(changes.Refactor))
	}
	if len(changes.Docs) != 1 {
		t.Errorf("CategorizedChangesProto.Docs length = %v, want 1", len(changes.Docs))
	}
	if len(changes.Other) != 1 {
		t.Errorf("CategorizedChangesProto.Other length = %v, want 1", len(changes.Other))
	}
}

func TestProtoMessageHelpers(t *testing.T) {
	req := &ExecuteRequestProto{Hook: HookProto_HOOK_PRE_PLAN}
	req.Reset()
	_ = req.String()
	req.ProtoMessage()

	resp := &ExecuteResponseProto{Success: true}
	resp.Reset()
	_ = resp.String()
	resp.ProtoMessage()

	ctx := &ReleaseContextProto{Version: "1.0.0"}
	ctx.Reset()
	_ = ctx.String()
	ctx.ProtoMessage()

	changes := &CategorizedChangesProto{}
	changes.Reset()
	_ = changes.String()
	changes.ProtoMessage()

	commit := &ConventionalCommitProto{Type: "feat", Description: "add"}
	commit.Reset()
	_ = commit.String()
	commit.ProtoMessage()

	artifact := &ArtifactProto{Name: "file"}
	artifact.Reset()
	_ = artifact.String()
	artifact.ProtoMessage()

	valReq := &ValidateRequestProto{Config: "{}"}
	valReq.Reset()
	_ = valReq.String()
	valReq.ProtoMessage()

	valResp := &ValidateResponseProto{Valid: true}
	valResp.Reset()
	_ = valResp.String()
	valResp.ProtoMessage()

	valErr := &ValidationErrorProto{Field: "f", Message: "m"}
	valErr.Reset()
	_ = valErr.String()
	valErr.ProtoMessage()

	empty := &Empty{}
	empty.Reset()
	_ = empty.String()
	empty.ProtoMessage()

	info := &PluginInfo{Name: "p", Version: "1.0.0"}
	info.Reset()
	_ = info.String()
	info.ProtoMessage()
}

func TestConventionalCommitProto_AllFields(t *testing.T) {
	commit := ConventionalCommitProto{
		Hash:                "abc123",
		Type:                "feat",
		Scope:               "api",
		Description:         "add endpoint",
		Body:                "Detailed description",
		Breaking:            true,
		BreakingDescription: "Breaking change",
		Issues:              []string{"#123"},
		Author:              "John Doe",
		Date:                "2024-01-15",
	}

	if commit.Hash != "abc123" {
		t.Errorf("ConventionalCommitProto.Hash = %v, want abc123", commit.Hash)
	}
	if commit.Type != "feat" {
		t.Errorf("ConventionalCommitProto.Type = %v, want feat", commit.Type)
	}
	if commit.Scope != "api" {
		t.Errorf("ConventionalCommitProto.Scope = %v, want api", commit.Scope)
	}
	if commit.Description != "add endpoint" {
		t.Errorf("ConventionalCommitProto.Description = %v, want add endpoint", commit.Description)
	}
	if commit.Body != "Detailed description" {
		t.Errorf("ConventionalCommitProto.Body = %v, want Detailed description", commit.Body)
	}
	if !commit.Breaking {
		t.Error("ConventionalCommitProto.Breaking should be true")
	}
	if commit.BreakingDescription != "Breaking change" {
		t.Errorf("ConventionalCommitProto.BreakingDescription = %v, want Breaking change", commit.BreakingDescription)
	}
	if len(commit.Issues) != 1 {
		t.Errorf("ConventionalCommitProto.Issues length = %v, want 1", len(commit.Issues))
	}
	if commit.Author != "John Doe" {
		t.Errorf("ConventionalCommitProto.Author = %v, want John Doe", commit.Author)
	}
	if commit.Date != "2024-01-15" {
		t.Errorf("ConventionalCommitProto.Date = %v, want 2024-01-15", commit.Date)
	}
}

func TestArtifactProto_AllFields(t *testing.T) {
	artifact := ArtifactProto{
		Name:     "release.tar.gz",
		Path:     "/tmp/release.tar.gz",
		Type:     "archive",
		Size:     1024,
		Checksum: "sha256:abc123",
	}

	if artifact.Name != "release.tar.gz" {
		t.Errorf("ArtifactProto.Name = %v, want release.tar.gz", artifact.Name)
	}
	if artifact.Path != "/tmp/release.tar.gz" {
		t.Errorf("ArtifactProto.Path = %v, want /tmp/release.tar.gz", artifact.Path)
	}
	if artifact.Type != "archive" {
		t.Errorf("ArtifactProto.Type = %v, want archive", artifact.Type)
	}
	if artifact.Size != 1024 {
		t.Errorf("ArtifactProto.Size = %v, want 1024", artifact.Size)
	}
	if artifact.Checksum != "sha256:abc123" {
		t.Errorf("ArtifactProto.Checksum = %v, want sha256:abc123", artifact.Checksum)
	}
}

func TestValidateRequestProto_Config(t *testing.T) {
	req := ValidateRequestProto{
		Config: `{"api_token": "test", "timeout": 30}`,
	}

	if req.Config != `{"api_token": "test", "timeout": 30}` {
		t.Errorf("ValidateRequestProto.Config unexpected value")
	}
}

func TestValidateResponseProto_Valid(t *testing.T) {
	resp := ValidateResponseProto{
		Valid:  true,
		Errors: nil,
	}

	if !resp.Valid {
		t.Error("ValidateResponseProto.Valid should be true")
	}
	if resp.Errors != nil {
		t.Error("ValidateResponseProto.Errors should be nil")
	}
}

func TestValidateResponseProto_Invalid(t *testing.T) {
	resp := ValidateResponseProto{
		Valid: false,
		Errors: []*ValidationErrorProto{
			{Field: "token", Message: "required", Code: "REQUIRED"},
		},
	}

	if resp.Valid {
		t.Error("ValidateResponseProto.Valid should be false")
	}
	if len(resp.Errors) != 1 {
		t.Errorf("ValidateResponseProto.Errors length = %v, want 1", len(resp.Errors))
	}
	if resp.Errors[0].Field != "token" {
		t.Errorf("ValidationErrorProto.Field = %v, want token", resp.Errors[0].Field)
	}
	if resp.Errors[0].Message != "required" {
		t.Errorf("ValidationErrorProto.Message = %v, want required", resp.Errors[0].Message)
	}
	if resp.Errors[0].Code != "REQUIRED" {
		t.Errorf("ValidationErrorProto.Code = %v, want REQUIRED", resp.Errors[0].Code)
	}
}

func TestValidationErrorProto_Fields(t *testing.T) {
	err := ValidationErrorProto{
		Field:   "webhook_url",
		Message: "invalid URL",
		Code:    "INVALID_URL",
	}

	if err.Field != "webhook_url" {
		t.Errorf("ValidationErrorProto.Field = %v, want webhook_url", err.Field)
	}
	if err.Message != "invalid URL" {
		t.Errorf("ValidationErrorProto.Message = %v, want invalid URL", err.Message)
	}
	if err.Code != "INVALID_URL" {
		t.Errorf("ValidationErrorProto.Code = %v, want INVALID_URL", err.Code)
	}
}

func TestUnimplementedPluginServer_GetInfo(t *testing.T) {
	srv := UnimplementedPluginServer{}
	info, err := srv.GetInfo(context.TODO(), nil)
	if info != nil {
		t.Error("UnimplementedPluginServer.GetInfo should return nil info")
	}
	if err != nil {
		t.Error("UnimplementedPluginServer.GetInfo should return nil error")
	}
}

func TestUnimplementedPluginServer_Execute(t *testing.T) {
	srv := UnimplementedPluginServer{}
	resp, err := srv.Execute(context.TODO(), nil)
	if resp != nil {
		t.Error("UnimplementedPluginServer.Execute should return nil response")
	}
	if err != nil {
		t.Error("UnimplementedPluginServer.Execute should return nil error")
	}
}

func TestUnimplementedPluginServer_Validate(t *testing.T) {
	srv := UnimplementedPluginServer{}
	resp, err := srv.Validate(context.TODO(), nil)
	if resp != nil {
		t.Error("UnimplementedPluginServer.Validate should return nil response")
	}
	if err != nil {
		t.Error("UnimplementedPluginServer.Validate should return nil error")
	}
}
