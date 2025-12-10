package plugin

import (
	"context"
	"testing"

	"google.golang.org/grpc"
)

func TestProtoHookToHook(t *testing.T) {
	tests := []struct {
		name      string
		protoHook HookProto
		want      Hook
	}{
		{"pre_init", HookProto_HOOK_PRE_INIT, HookPreInit},
		{"post_init", HookProto_HOOK_POST_INIT, HookPostInit},
		{"pre_plan", HookProto_HOOK_PRE_PLAN, HookPrePlan},
		{"post_plan", HookProto_HOOK_POST_PLAN, HookPostPlan},
		{"pre_version", HookProto_HOOK_PRE_VERSION, HookPreVersion},
		{"post_version", HookProto_HOOK_POST_VERSION, HookPostVersion},
		{"pre_notes", HookProto_HOOK_PRE_NOTES, HookPreNotes},
		{"post_notes", HookProto_HOOK_POST_NOTES, HookPostNotes},
		{"pre_approve", HookProto_HOOK_PRE_APPROVE, HookPreApprove},
		{"post_approve", HookProto_HOOK_POST_APPROVE, HookPostApprove},
		{"pre_publish", HookProto_HOOK_PRE_PUBLISH, HookPrePublish},
		{"post_publish", HookProto_HOOK_POST_PUBLISH, HookPostPublish},
		{"on_success", HookProto_HOOK_ON_SUCCESS, HookOnSuccess},
		{"on_error", HookProto_HOOK_ON_ERROR, HookOnError},
		{"unspecified", HookProto_HOOK_UNSPECIFIED, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := protoHookToHook(tt.protoHook)
			if got != tt.want {
				t.Errorf("protoHookToHook(%v) = %v, want %v", tt.protoHook, got, tt.want)
			}
		})
	}
}

func TestHookToProtoHook(t *testing.T) {
	tests := []struct {
		name string
		hook Hook
		want HookProto
	}{
		{"pre_init", HookPreInit, HookProto_HOOK_PRE_INIT},
		{"post_init", HookPostInit, HookProto_HOOK_POST_INIT},
		{"pre_plan", HookPrePlan, HookProto_HOOK_PRE_PLAN},
		{"post_plan", HookPostPlan, HookProto_HOOK_POST_PLAN},
		{"pre_version", HookPreVersion, HookProto_HOOK_PRE_VERSION},
		{"post_version", HookPostVersion, HookProto_HOOK_POST_VERSION},
		{"pre_notes", HookPreNotes, HookProto_HOOK_PRE_NOTES},
		{"post_notes", HookPostNotes, HookProto_HOOK_POST_NOTES},
		{"pre_approve", HookPreApprove, HookProto_HOOK_PRE_APPROVE},
		{"post_approve", HookPostApprove, HookProto_HOOK_POST_APPROVE},
		{"pre_publish", HookPrePublish, HookProto_HOOK_PRE_PUBLISH},
		{"post_publish", HookPostPublish, HookProto_HOOK_POST_PUBLISH},
		{"on_success", HookOnSuccess, HookProto_HOOK_ON_SUCCESS},
		{"on_error", HookOnError, HookProto_HOOK_ON_ERROR},
		{"unknown", Hook("unknown"), HookProto_HOOK_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hookToProtoHook(tt.hook)
			if got != tt.want {
				t.Errorf("hookToProtoHook(%v) = %v, want %v", tt.hook, got, tt.want)
			}
		})
	}
}

func TestConvertProtoChanges(t *testing.T) {
	protoChanges := &CategorizedChangesProto{
		Features: []*ConventionalCommitProto{
			{
				Hash:        "abc123",
				Type:        "feat",
				Scope:       "api",
				Description: "add new endpoint",
				Body:        "detailed description",
				Breaking:    false,
				Issues:      []string{"#123"},
				Author:      "John Doe",
				Date:        "2024-01-01",
			},
		},
		Fixes: []*ConventionalCommitProto{
			{
				Hash:        "def456",
				Type:        "fix",
				Description: "fix bug",
			},
		},
		Breaking: []*ConventionalCommitProto{
			{
				Hash:                "ghi789",
				Type:                "feat",
				Description:         "breaking change",
				Breaking:            true,
				BreakingDescription: "this breaks things",
			},
		},
	}

	result := convertProtoChanges(protoChanges)

	if len(result.Features) != 1 {
		t.Errorf("Features len = %d, want 1", len(result.Features))
	}
	if result.Features[0].Hash != "abc123" {
		t.Errorf("Features[0].Hash = %s, want abc123", result.Features[0].Hash)
	}
	if result.Features[0].Type != "feat" {
		t.Errorf("Features[0].Type = %s, want feat", result.Features[0].Type)
	}
	if len(result.Features[0].Issues) != 1 {
		t.Errorf("Features[0].Issues len = %d, want 1", len(result.Features[0].Issues))
	}

	if len(result.Fixes) != 1 {
		t.Errorf("Fixes len = %d, want 1", len(result.Fixes))
	}
	if result.Fixes[0].Hash != "def456" {
		t.Errorf("Fixes[0].Hash = %s, want def456", result.Fixes[0].Hash)
	}

	if len(result.Breaking) != 1 {
		t.Errorf("Breaking len = %d, want 1", len(result.Breaking))
	}
	if result.Breaking[0].BreakingDescription != "this breaks things" {
		t.Errorf("Breaking[0].BreakingDescription = %s, want 'this breaks things'", result.Breaking[0].BreakingDescription)
	}
}

func TestConvertChangesToProto(t *testing.T) {
	changes := &CategorizedChanges{
		Features: []ConventionalCommit{
			{
				Hash:        "abc123",
				Type:        "feat",
				Scope:       "api",
				Description: "add new endpoint",
			},
		},
		Fixes: []ConventionalCommit{
			{
				Hash:        "def456",
				Type:        "fix",
				Description: "fix bug",
			},
		},
		Performance: []ConventionalCommit{
			{
				Hash:        "perf123",
				Type:        "perf",
				Description: "improve speed",
			},
		},
		Refactor: []ConventionalCommit{
			{
				Hash:        "ref123",
				Type:        "refactor",
				Description: "refactor code",
			},
		},
		Docs: []ConventionalCommit{
			{
				Hash:        "doc123",
				Type:        "docs",
				Description: "update docs",
			},
		},
		Other: []ConventionalCommit{
			{
				Hash:        "other123",
				Type:        "chore",
				Description: "update deps",
			},
		},
	}

	result := convertChangesToProto(changes)

	if len(result.Features) != 1 {
		t.Errorf("Features len = %d, want 1", len(result.Features))
	}
	if result.Features[0].Hash != "abc123" {
		t.Errorf("Features[0].Hash = %s, want abc123", result.Features[0].Hash)
	}

	if len(result.Fixes) != 1 {
		t.Errorf("Fixes len = %d, want 1", len(result.Fixes))
	}

	if len(result.Performance) != 1 {
		t.Errorf("Performance len = %d, want 1", len(result.Performance))
	}

	if len(result.Refactor) != 1 {
		t.Errorf("Refactor len = %d, want 1", len(result.Refactor))
	}

	if len(result.Docs) != 1 {
		t.Errorf("Docs len = %d, want 1", len(result.Docs))
	}

	if len(result.Other) != 1 {
		t.Errorf("Other len = %d, want 1", len(result.Other))
	}
}

func TestConvertProtoCommits(t *testing.T) {
	protoCommits := []*ConventionalCommitProto{
		{
			Hash:        "abc123",
			Type:        "feat",
			Scope:       "api",
			Description: "add feature",
			Body:        "body text",
			Breaking:    true,
			Issues:      []string{"#1", "#2"},
			Author:      "Jane Doe",
			Date:        "2024-01-01",
		},
	}

	result := convertProtoCommits(protoCommits)

	if len(result) != 1 {
		t.Fatalf("convertProtoCommits len = %d, want 1", len(result))
	}

	if result[0].Hash != "abc123" {
		t.Errorf("Hash = %s, want abc123", result[0].Hash)
	}
	if result[0].Type != "feat" {
		t.Errorf("Type = %s, want feat", result[0].Type)
	}
	if result[0].Scope != "api" {
		t.Errorf("Scope = %s, want api", result[0].Scope)
	}
	if !result[0].Breaking {
		t.Error("Breaking should be true")
	}
	if len(result[0].Issues) != 2 {
		t.Errorf("Issues len = %d, want 2", len(result[0].Issues))
	}
}

func TestConvertCommitsToProto(t *testing.T) {
	commits := []ConventionalCommit{
		{
			Hash:                "abc123",
			Type:                "feat",
			Scope:               "api",
			Description:         "add feature",
			Body:                "body text",
			Breaking:            true,
			BreakingDescription: "breaks API",
			Issues:              []string{"#1", "#2"},
			Author:              "Jane Doe",
			Date:                "2024-01-01",
		},
	}

	result := convertCommitsToProto(commits)

	if len(result) != 1 {
		t.Fatalf("convertCommitsToProto len = %d, want 1", len(result))
	}

	if result[0].Hash != "abc123" {
		t.Errorf("Hash = %s, want abc123", result[0].Hash)
	}
	if result[0].BreakingDescription != "breaks API" {
		t.Errorf("BreakingDescription = %s, want 'breaks API'", result[0].BreakingDescription)
	}
}

func TestGRPCClient_GetInfo_Timeout(t *testing.T) {
	// This test verifies that GetInfo uses a timeout context
	// We create a mock client that would hang indefinitely without timeout
	client := &GRPCClient{
		client: &mockPluginClient{
			hangOnGetInfo: true,
		},
	}

	// This should return empty Info due to timeout, not hang forever
	info := client.GetInfo()

	// Verify it returns empty info on error
	if info.Name != "" {
		t.Error("Expected empty Name on timeout")
	}
}

// mockPluginClient is a test mock for PluginClient
type mockPluginClient struct {
	UnimplementedPluginServer
	hangOnGetInfo bool
}

func (m *mockPluginClient) GetInfo(ctx context.Context, req *Empty, opts ...grpc.CallOption) (*PluginInfo, error) {
	if m.hangOnGetInfo {
		// Wait for context cancellation
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &PluginInfo{
		Name:    "test",
		Version: "1.0.0",
	}, nil
}

func (m *mockPluginClient) Execute(ctx context.Context, req *ExecuteRequestProto, opts ...grpc.CallOption) (*ExecuteResponseProto, error) {
	return &ExecuteResponseProto{Success: true}, nil
}

func (m *mockPluginClient) Validate(ctx context.Context, req *ValidateRequestProto, opts ...grpc.CallOption) (*ValidateResponseProto, error) {
	return &ValidateResponseProto{Valid: true}, nil
}

func TestGRPCClient_Execute(t *testing.T) {
	client := &GRPCClient{
		client: &mockPluginClient{},
	}

	req := ExecuteRequest{
		Hook: HookPrePublish,
		Config: map[string]any{
			"key": "value",
		},
		Context: ReleaseContext{
			Version:      "1.0.0",
			TagName:      "v1.0.0",
			ReleaseNotes: "Test notes",
			Changes: &CategorizedChanges{
				Features: []ConventionalCommit{
					{
						Hash:        "abc123",
						Type:        "feat",
						Description: "new feature",
					},
				},
			},
		},
		DryRun: true,
	}

	resp, err := client.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !resp.Success {
		t.Error("Execute() Success = false")
	}
}

func TestGRPCClient_Validate(t *testing.T) {
	client := &GRPCClient{
		client: &mockPluginClient{},
	}

	config := map[string]any{
		"key": "value",
	}

	resp, err := client.Validate(context.Background(), config)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if !resp.Valid {
		t.Error("Validate() Valid = false")
	}
}

func TestGRPCServer_GetInfo(t *testing.T) {
	mockPlugin := &mockPlugin{}
	server := &GRPCServer{Impl: mockPlugin}

	resp, err := server.GetInfo(context.Background(), &Empty{})
	if err != nil {
		t.Fatalf("GetInfo() error = %v", err)
	}

	if resp.Name != "test-plugin" {
		t.Errorf("GetInfo().Name = %s, want test-plugin", resp.Name)
	}

	if len(resp.Hooks) != 1 {
		t.Errorf("GetInfo().Hooks len = %d, want 1", len(resp.Hooks))
	}
}

func TestGRPCServer_Execute(t *testing.T) {
	mockPlugin := &mockPlugin{}
	server := &GRPCServer{Impl: mockPlugin}

	req := &ExecuteRequestProto{
		Hook:   HookProto_HOOK_PRE_PUBLISH,
		Config: `{"key":"value"}`,
		Context: &ReleaseContextProto{
			Version: "1.0.0",
			TagName: "v1.0.0",
		},
		DryRun: false,
	}

	resp, err := server.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !resp.Success {
		t.Error("Execute() Success = false")
	}
}

func TestGRPCServer_Execute_InvalidJSON(t *testing.T) {
	mockPlugin := &mockPlugin{}
	server := &GRPCServer{Impl: mockPlugin}

	req := &ExecuteRequestProto{
		Hook:   HookProto_HOOK_PRE_PUBLISH,
		Config: `{invalid json}`,
	}

	resp, err := server.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if resp.Success {
		t.Error("Execute() should fail with invalid JSON")
	}

	if resp.Error == "" {
		t.Error("Execute() Error should not be empty for invalid JSON")
	}
}

func TestGRPCServer_Validate(t *testing.T) {
	mockPlugin := &mockPlugin{}
	server := &GRPCServer{Impl: mockPlugin}

	req := &ValidateRequestProto{
		Config: `{"key":"value"}`,
	}

	resp, err := server.Validate(context.Background(), req)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if !resp.Valid {
		t.Error("Validate() Valid = false")
	}
}

func TestGRPCServer_Validate_InvalidJSON(t *testing.T) {
	mockPlugin := &mockPlugin{}
	server := &GRPCServer{Impl: mockPlugin}

	req := &ValidateRequestProto{
		Config: `{invalid`,
	}

	resp, err := server.Validate(context.Background(), req)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if resp.Valid {
		t.Error("Validate() should return invalid for bad JSON")
	}

	if len(resp.Errors) == 0 {
		t.Error("Validate() should have errors for invalid JSON")
	}
}

// mockPlugin is a test implementation of the Plugin interface
type mockPlugin struct{}

func (m *mockPlugin) GetInfo() Info {
	return Info{
		Name:    "test-plugin",
		Version: "1.0.0",
		Hooks:   []Hook{HookPrePublish},
	}
}

func (m *mockPlugin) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	return &ExecuteResponse{
		Success: true,
		Message: "test success",
		Outputs: map[string]any{"key": "value"},
		Artifacts: []Artifact{
			{Name: "test.zip", Path: "/path/test.zip", Type: "file", Size: 100},
		},
	}, nil
}

func (m *mockPlugin) Validate(ctx context.Context, config map[string]any) (*ValidateResponse, error) {
	return &ValidateResponse{
		Valid:  true,
		Errors: []ValidationError{},
	}, nil
}
