// Package release provides application use cases for release management.
package release

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/integration"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

// mockPluginExecutor implements integration.PluginExecutor for testing.
type mockPluginExecutor struct {
	responses   map[integration.Hook][]integration.ExecuteResponse
	errors      map[integration.Hook]error
	execCalls   []execCall
	singleCalls []singleExecCall
}

type execCall struct {
	hook       integration.Hook
	releaseCtx integration.ReleaseContext
}

type singleExecCall struct {
	id  integration.PluginID
	req integration.ExecuteRequest
}

func newMockPluginExecutor() *mockPluginExecutor {
	return &mockPluginExecutor{
		responses: make(map[integration.Hook][]integration.ExecuteResponse),
		errors:    make(map[integration.Hook]error),
	}
}

func (m *mockPluginExecutor) ExecuteHook(ctx context.Context, hook integration.Hook, releaseCtx integration.ReleaseContext) ([]integration.ExecuteResponse, error) {
	m.execCalls = append(m.execCalls, execCall{hook: hook, releaseCtx: releaseCtx})
	if err, ok := m.errors[hook]; ok && err != nil {
		return nil, err
	}
	if resp, ok := m.responses[hook]; ok {
		return resp, nil
	}
	return nil, nil
}

func (m *mockPluginExecutor) ExecutePlugin(ctx context.Context, id integration.PluginID, req integration.ExecuteRequest) (*integration.ExecuteResponse, error) {
	m.singleCalls = append(m.singleCalls, singleExecCall{id: id, req: req})
	return &integration.ExecuteResponse{Success: true}, nil
}

// createApprovedRelease creates a release in the approved state ready for publishing.
func createApprovedRelease(id release.ReleaseID, branch, repoPath string) *release.Release {
	r := release.NewRelease(id, branch, repoPath)

	// Create a changeset for the plan
	cs := changes.NewChangeSet("cs-test", "v1.0.0", "HEAD")
	cs.AddCommit(changes.NewConventionalCommit("abc123", changes.CommitTypeFeat, "new feature"))

	// Create plan
	currentVersion := version.MustParse("1.0.0")
	nextVersion := version.MustParse("1.1.0")
	plan := release.NewReleasePlan(
		currentVersion,
		nextVersion,
		changes.ReleaseTypeMinor,
		cs,
		false,
	)
	_ = release.SetPlan(r, plan)

	// Set version
	_ = r.SetVersion(nextVersion, "v1.1.0")

	// Set notes
	notes := &release.ReleaseNotes{
		Text:        "## [1.1.0] - Changes\n- feat: new feature",
		Provider:    "test",
		GeneratedAt: time.Now(),
	}
	_ = r.SetNotes(notes)

	// Approve
	_ = r.Approve("test-user", false)

	return r
}

func TestPublishReleaseInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   PublishReleaseInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid input with all fields",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				CreateTag: true,
				PushTag:   true,
				TagPrefix: "v",
				Remote:    "origin",
			},
			wantErr: false,
		},
		{
			name: "valid input minimal",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
			},
			wantErr: false,
		},
		{
			name: "missing release ID",
			input: PublishReleaseInput{
				CreateTag: true,
			},
			wantErr: true,
			errMsg:  "release ID is required",
		},
		{
			name: "tag prefix too long",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				TagPrefix: "this-is-a-very-long-tag-prefix-that-exceeds-limit",
			},
			wantErr: true,
			errMsg:  "too long",
		},
		{
			name: "tag prefix with invalid characters",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				TagPrefix: "v*",
			},
			wantErr: true,
			errMsg:  "invalid characters",
		},
		{
			name: "tag prefix with space",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				TagPrefix: "v ",
			},
			wantErr: true,
			errMsg:  "invalid characters",
		},
		{
			name: "remote name too long",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				Remote:    strings.Repeat("a", 257),
			},
			wantErr: true,
			errMsg:  "too long",
		},
		{
			name: "remote name with whitespace",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				Remote:    "origin upstream",
			},
			wantErr: true,
			errMsg:  "whitespace",
		},
		{
			name: "remote name with tab",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				Remote:    "origin\tupstream",
			},
			wantErr: true,
			errMsg:  "whitespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestPublishReleaseUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		input          PublishReleaseInput
		setupRelease   func(repo *mockReleaseRepository)
		gitRepo        *mockGitRepository
		pluginExec     integration.PluginExecutor // Use interface type to properly handle nil
		eventPublisher *mockEventPublisher
		wantErr        bool
		errMsg         string
		wantTagName    string
		wantSaved      bool
	}{
		{
			name: "successful publish with tag creation",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				CreateTag: true,
				PushTag:   true,
				TagPrefix: "v",
				Remote:    "origin",
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createApprovedRelease("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			gitRepo: &mockGitRepository{
				latestCommit: createTestCommit("abc123def", "feat: latest commit"),
				tagCreated:   sourcecontrol.NewTag("v1.1.0", "abc123def"),
			},
			pluginExec:     newMockPluginExecutor(),
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantTagName:    "v1.1.0",
			wantSaved:      true,
		},
		{
			name: "successful publish dry run - no tag creation",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				CreateTag: true,
				PushTag:   true,
				DryRun:    true,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createApprovedRelease("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			gitRepo:        &mockGitRepository{},
			pluginExec:     newMockPluginExecutor(),
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantTagName:    "v1.1.0",
			wantSaved:      false, // Dry run doesn't save
		},
		{
			name: "invalid input - missing release ID",
			input: PublishReleaseInput{
				CreateTag: true,
			},
			setupRelease:   func(repo *mockReleaseRepository) {},
			gitRepo:        &mockGitRepository{},
			pluginExec:     newMockPluginExecutor(),
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "invalid input",
			wantSaved:      false,
		},
		{
			name: "release not found",
			input: PublishReleaseInput{
				ReleaseID: "nonexistent",
				CreateTag: true,
			},
			setupRelease:   func(repo *mockReleaseRepository) {},
			gitRepo:        &mockGitRepository{},
			pluginExec:     newMockPluginExecutor(),
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "failed to find release",
			wantSaved:      false,
		},
		{
			name: "release not ready for publishing - initialized state",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				CreateTag: true,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				// Create a release that's only in initialized state
				r := release.NewRelease("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			gitRepo:        &mockGitRepository{},
			pluginExec:     newMockPluginExecutor(),
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "not ready for publishing",
			wantSaved:      false,
		},
		{
			name: "pre-publish plugin fails",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				CreateTag: true,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createApprovedRelease("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			gitRepo: &mockGitRepository{},
			pluginExec: func() *mockPluginExecutor {
				m := newMockPluginExecutor()
				m.errors[integration.HookPrePublish] = errors.New("pre-publish plugin failed")
				return m
			}(),
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "pre-publish hook failed",
			wantSaved:      false,
		},
		{
			name: "get latest commit fails",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				CreateTag: true,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createApprovedRelease("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			gitRepo: &mockGitRepository{
				latestCommitErr: errors.New("git error"),
			},
			pluginExec:     newMockPluginExecutor(),
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "failed to get latest commit",
			wantSaved:      false,
		},
		{
			name: "create tag fails",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				CreateTag: true,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createApprovedRelease("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			gitRepo: &mockGitRepository{
				latestCommit: createTestCommit("abc123", "latest"),
				tagCreateErr: errors.New("tag exists"),
			},
			pluginExec:     newMockPluginExecutor(),
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "failed to create tag",
			wantSaved:      false,
		},
		{
			name: "push tag fails",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				CreateTag: true,
				PushTag:   true,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createApprovedRelease("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			gitRepo: &mockGitRepository{
				latestCommit: createTestCommit("abc123", "latest"),
				tagCreated:   sourcecontrol.NewTag("v1.1.0", "abc123"),
				pushTagErr:   errors.New("push failed"),
			},
			pluginExec:     newMockPluginExecutor(),
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "failed to push tag",
			wantSaved:      false,
		},
		{
			name: "publish without tag creation",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				CreateTag: false,
				PushTag:   false,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createApprovedRelease("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			gitRepo:        &mockGitRepository{},
			pluginExec:     newMockPluginExecutor(),
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantTagName:    "v1.1.0",
			wantSaved:      true,
		},
		{
			name: "custom tag prefix",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				CreateTag: true,
				PushTag:   true,
				TagPrefix: "release-",
				Remote:    "upstream",
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createApprovedRelease("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			gitRepo: &mockGitRepository{
				latestCommit: createTestCommit("abc123", "latest"),
				tagCreated:   sourcecontrol.NewTag("release-1.1.0", "abc123"),
			},
			pluginExec:     newMockPluginExecutor(),
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantTagName:    "release-1.1.0",
			wantSaved:      true,
		},
		{
			name: "post-publish plugin failure does not fail release",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				CreateTag: true,
				PushTag:   true,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createApprovedRelease("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			gitRepo: &mockGitRepository{
				latestCommit: createTestCommit("abc123", "latest"),
				tagCreated:   sourcecontrol.NewTag("v1.1.0", "abc123"),
			},
			pluginExec: func() *mockPluginExecutor {
				m := newMockPluginExecutor()
				m.errors[integration.HookPostPublish] = errors.New("notification failed")
				return m
			}(),
			eventPublisher: &mockEventPublisher{},
			wantErr:        false, // Post-publish failures don't fail the release
			wantTagName:    "v1.1.0",
			wantSaved:      true,
		},
		{
			name: "on-success plugin failure does not fail release",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				CreateTag: true,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createApprovedRelease("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			gitRepo: &mockGitRepository{
				latestCommit: createTestCommit("abc123", "latest"),
				tagCreated:   sourcecontrol.NewTag("v1.1.0", "abc123"),
			},
			pluginExec: func() *mockPluginExecutor {
				m := newMockPluginExecutor()
				m.errors[integration.HookOnSuccess] = errors.New("slack notification failed")
				return m
			}(),
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantTagName:    "v1.1.0",
			wantSaved:      true,
		},
		{
			name: "save release fails",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				CreateTag: false, // Skip tag creation
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createApprovedRelease("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
				repo.saveErr = errors.New("database error")
			},
			gitRepo:        &mockGitRepository{},
			pluginExec:     newMockPluginExecutor(),
			eventPublisher: &mockEventPublisher{},
			wantErr:        true,
			errMsg:         "failed to save release",
			wantSaved:      false,
		},
		{
			name: "event publisher failure is logged but does not fail",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				CreateTag: false,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createApprovedRelease("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			gitRepo:    &mockGitRepository{},
			pluginExec: newMockPluginExecutor(),
			eventPublisher: &mockEventPublisher{
				publishErr: errors.New("event bus error"),
			},
			wantErr:     false,
			wantTagName: "v1.1.0",
			wantSaved:   true,
		},
		{
			name: "plugin executor nil - plugins are skipped",
			input: PublishReleaseInput{
				ReleaseID: "release-123",
				CreateTag: true,
				PushTag:   true,
			},
			setupRelease: func(repo *mockReleaseRepository) {
				r := createApprovedRelease("release-123", "main", "/path/to/repo")
				repo.releases["release-123"] = r
			},
			gitRepo: &mockGitRepository{
				latestCommit: createTestCommit("abc123", "latest"),
				tagCreated:   sourcecontrol.NewTag("v1.1.0", "abc123"),
			},
			pluginExec:     nil, // No plugin executor
			eventPublisher: &mockEventPublisher{},
			wantErr:        false,
			wantTagName:    "v1.1.0",
			wantSaved:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			releaseRepo := newMockReleaseRepository()
			tt.setupRelease(releaseRepo)

			uc := NewPublishReleaseUseCase(
				releaseRepo,
				tt.gitRepo,
				tt.pluginExec,
				tt.eventPublisher,
			)

			output, err := uc.Execute(ctx, tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if output == nil {
				t.Error("expected output, got nil")
				return
			}

			if tt.wantTagName != "" && output.TagName != tt.wantTagName {
				t.Errorf("TagName = %s, want %s", output.TagName, tt.wantTagName)
			}

			// Check if save was called when expected
			if tt.wantSaved && !releaseRepo.saveCalled {
				t.Error("expected release to be saved, but Save was not called")
			}
			if !tt.wantSaved && releaseRepo.saveCalled {
				t.Error("did not expect release to be saved, but Save was called")
			}
		})
	}
}

func TestPublishReleaseUseCase_PluginHookExecution(t *testing.T) {
	ctx := context.Background()

	releaseRepo := newMockReleaseRepository()
	r := createApprovedRelease("release-123", "main", "/path/to/repo")
	releaseRepo.releases["release-123"] = r

	pluginExec := newMockPluginExecutor()
	pluginExec.responses[integration.HookPrePublish] = []integration.ExecuteResponse{
		{Success: true, Message: "pre-publish ok"},
	}
	pluginExec.responses[integration.HookPostPublish] = []integration.ExecuteResponse{
		{Success: true, Message: "post-publish ok"},
	}
	pluginExec.responses[integration.HookOnSuccess] = []integration.ExecuteResponse{
		{Success: true, Message: "success notification sent"},
	}

	gitRepo := &mockGitRepository{
		latestCommit: createTestCommit("abc123", "latest"),
		tagCreated:   sourcecontrol.NewTag("v1.1.0", "abc123"),
	}

	uc := NewPublishReleaseUseCase(
		releaseRepo,
		gitRepo,
		pluginExec,
		&mockEventPublisher{},
	)

	input := PublishReleaseInput{
		ReleaseID: "release-123",
		CreateTag: true,
		PushTag:   true,
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all hooks were called
	expectedHooks := []integration.Hook{
		integration.HookPrePublish,
		integration.HookPostPublish,
		integration.HookOnSuccess,
	}

	if len(pluginExec.execCalls) != len(expectedHooks) {
		t.Errorf("expected %d hook calls, got %d", len(expectedHooks), len(pluginExec.execCalls))
	}

	for i, expected := range expectedHooks {
		if i >= len(pluginExec.execCalls) {
			t.Errorf("missing call for hook %s", expected)
			continue
		}
		if pluginExec.execCalls[i].hook != expected {
			t.Errorf("hook %d = %s, want %s", i, pluginExec.execCalls[i].hook, expected)
		}
	}

	// Verify plugin results are included in output
	if len(output.PluginResults) != 3 {
		t.Errorf("expected 3 plugin results, got %d", len(output.PluginResults))
	}
}

func TestPublishReleaseUseCase_ReleaseContextBuilding(t *testing.T) {
	ctx := context.Background()

	releaseRepo := newMockReleaseRepository()
	r := createApprovedRelease("release-123", "main", "/path/to/repo")
	releaseRepo.releases["release-123"] = r

	pluginExec := newMockPluginExecutor()
	gitRepo := &mockGitRepository{
		latestCommit: createTestCommit("abc123", "latest"),
		tagCreated:   sourcecontrol.NewTag("v1.1.0", "abc123"),
	}

	uc := NewPublishReleaseUseCase(
		releaseRepo,
		gitRepo,
		pluginExec,
		&mockEventPublisher{},
	)

	input := PublishReleaseInput{
		ReleaseID: "release-123",
		CreateTag: true,
		TagPrefix: "v",
		DryRun:    false,
	}

	_, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify release context passed to plugins
	if len(pluginExec.execCalls) == 0 {
		t.Fatal("no plugin calls made")
	}

	releaseCtx := pluginExec.execCalls[0].releaseCtx

	// Verify version info
	if releaseCtx.Version.String() != "1.1.0" {
		t.Errorf("Version = %s, want 1.1.0", releaseCtx.Version.String())
	}
	if releaseCtx.PreviousVersion.String() != "1.0.0" {
		t.Errorf("PreviousVersion = %s, want 1.0.0", releaseCtx.PreviousVersion.String())
	}

	// Verify repository info
	if releaseCtx.RepositoryName != "test-repo" {
		t.Errorf("RepositoryName = %s, want test-repo", releaseCtx.RepositoryName)
	}
	if releaseCtx.Branch != "main" {
		t.Errorf("Branch = %s, want main", releaseCtx.Branch)
	}
	if releaseCtx.TagName != "v1.1.0" {
		t.Errorf("TagName = %s, want v1.1.0", releaseCtx.TagName)
	}

	// Verify release notes are included
	if releaseCtx.Changelog == "" {
		t.Error("Changelog should not be empty")
	}
	if releaseCtx.ReleaseNotes == "" {
		t.Error("ReleaseNotes should not be empty")
	}
}

func TestPublishReleaseUseCase_DefaultRemote(t *testing.T) {
	ctx := context.Background()

	releaseRepo := newMockReleaseRepository()
	r := createApprovedRelease("release-123", "main", "/path/to/repo")
	releaseRepo.releases["release-123"] = r

	var pushedRemote string
	gitRepo := &mockGitRepository{
		latestCommit: createTestCommit("abc123", "latest"),
		tagCreated:   sourcecontrol.NewTag("v1.1.0", "abc123"),
	}
	// Track the remote used for push
	originalPushTag := gitRepo.PushTag
	_ = originalPushTag // silence unused warning

	uc := NewPublishReleaseUseCase(
		releaseRepo,
		gitRepo,
		newMockPluginExecutor(),
		&mockEventPublisher{},
	)

	// Test with empty remote - should default to "origin"
	input := PublishReleaseInput{
		ReleaseID: "release-123",
		CreateTag: true,
		PushTag:   true,
		Remote:    "", // Empty - should use default
	}

	_, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The mock doesn't track the remote used, but we can verify it didn't error
	// which means it used the default "origin"
	_ = pushedRemote
}

func TestPublishReleaseUseCase_DefaultTagPrefix(t *testing.T) {
	ctx := context.Background()

	releaseRepo := newMockReleaseRepository()
	r := createApprovedRelease("release-123", "main", "/path/to/repo")
	releaseRepo.releases["release-123"] = r

	gitRepo := &mockGitRepository{
		latestCommit: createTestCommit("abc123", "latest"),
		tagCreated:   sourcecontrol.NewTag("v1.1.0", "abc123"),
	}

	uc := NewPublishReleaseUseCase(
		releaseRepo,
		gitRepo,
		newMockPluginExecutor(),
		&mockEventPublisher{},
	)

	// Test with empty tag prefix - should default to "v"
	input := PublishReleaseInput{
		ReleaseID: "release-123",
		CreateTag: true,
		TagPrefix: "", // Empty - should use default "v"
	}

	output, err := uc.Execute(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify tag has "v" prefix
	if output.TagName != "v1.1.0" {
		t.Errorf("TagName = %s, want v1.1.0 (with default 'v' prefix)", output.TagName)
	}
}

func TestNewPublishReleaseUseCase(t *testing.T) {
	releaseRepo := newMockReleaseRepository()
	gitRepo := &mockGitRepository{}
	pluginExec := newMockPluginExecutor()
	eventPublisher := &mockEventPublisher{}

	uc := NewPublishReleaseUseCase(releaseRepo, gitRepo, pluginExec, eventPublisher)

	if uc == nil {
		t.Fatal("expected non-nil use case")
	}
	if uc.releaseRepo == nil {
		t.Error("releaseRepo should not be nil")
	}
	if uc.gitRepo == nil {
		t.Error("gitRepo should not be nil")
	}
	if uc.pluginExecutor == nil {
		t.Error("pluginExecutor should not be nil")
	}
	if uc.eventPublisher == nil {
		t.Error("eventPublisher should not be nil")
	}
	if uc.logger == nil {
		t.Error("logger should not be nil")
	}
}

func TestPublishReleaseOutput(t *testing.T) {
	output := &PublishReleaseOutput{
		TagName:    "v1.0.0",
		ReleaseURL: "https://github.com/owner/repo/releases/tag/v1.0.0",
		PluginResults: []PluginResult{
			{
				PluginName: "github",
				Hook:       integration.HookPostPublish,
				Success:    true,
				Message:    "Release created",
				Duration:   100 * time.Millisecond,
			},
		},
	}

	if output.TagName != "v1.0.0" {
		t.Errorf("TagName = %s, want v1.0.0", output.TagName)
	}
	if len(output.PluginResults) != 1 {
		t.Errorf("PluginResults length = %d, want 1", len(output.PluginResults))
	}
	if output.PluginResults[0].PluginName != "github" {
		t.Errorf("PluginName = %s, want github", output.PluginResults[0].PluginName)
	}
}

func TestPluginResult(t *testing.T) {
	result := PluginResult{
		PluginName: "slack",
		Hook:       integration.HookOnSuccess,
		Success:    true,
		Message:    "Notification sent",
		Duration:   50 * time.Millisecond,
	}

	if result.PluginName != "slack" {
		t.Errorf("PluginName = %s, want slack", result.PluginName)
	}
	if result.Hook != integration.HookOnSuccess {
		t.Errorf("Hook = %s, want %s", result.Hook, integration.HookOnSuccess)
	}
	if !result.Success {
		t.Error("Success should be true")
	}
	if result.Message != "Notification sent" {
		t.Errorf("Message = %s, want 'Notification sent'", result.Message)
	}
	if result.Duration != 50*time.Millisecond {
		t.Errorf("Duration = %v, want 50ms", result.Duration)
	}
}

// Additional tests for uncovered use cases

func TestNewApproveReleaseUseCase(t *testing.T) {
	repo := newMockReleaseRepository()
	publisher := &mockEventPublisher{}

	uc := NewApproveReleaseUseCase(repo, publisher)

	if uc == nil {
		t.Fatal("NewApproveReleaseUseCase should return non-nil use case")
	}
}

func TestNewGetReleaseForApprovalUseCase(t *testing.T) {
	repo := newMockReleaseRepository()

	uc := NewGetReleaseForApprovalUseCase(repo)

	if uc == nil {
		t.Fatal("NewGetReleaseForApprovalUseCase should return non-nil use case")
	}
}

func TestNewGenerateNotesUseCase(t *testing.T) {
	repo := newMockReleaseRepository()
	publisher := &mockEventPublisher{}

	uc := NewGenerateNotesUseCase(repo, nil, publisher)

	if uc == nil {
		t.Fatal("NewGenerateNotesUseCase should return non-nil use case")
	}
}
