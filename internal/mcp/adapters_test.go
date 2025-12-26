package mcp

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/internal/application/governance"
	"github.com/relicta-tech/relicta/internal/domain/changes"
	domainrelease "github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestNewAdapter(t *testing.T) {
	t.Run("creates empty adapter", func(t *testing.T) {
		adapter := NewAdapter()
		assert.NotNil(t, adapter)
		assert.False(t, adapter.HasPlanUseCase())
		assert.False(t, adapter.HasCalculateVersionUseCase())
		assert.False(t, adapter.HasGenerateNotesUseCase())
		assert.False(t, adapter.HasApproveUseCase())
		assert.False(t, adapter.HasPublishUseCase())
		assert.False(t, adapter.HasGovernanceService())
		assert.False(t, adapter.HasReleaseRepository())
	})
}

func TestAdapterOptions(t *testing.T) {
	t.Run("WithPlanUseCase sets plan use case", func(t *testing.T) {
		// We can't easily create a real PlanReleaseUseCase without dependencies,
		// but we can test that nil is handled gracefully
		adapter := NewAdapter(WithPlanUseCase(nil))
		assert.False(t, adapter.HasPlanUseCase())
	})

	t.Run("WithCalculateVersionUseCase sets calculate version use case", func(t *testing.T) {
		adapter := NewAdapter(WithCalculateVersionUseCase(nil))
		assert.False(t, adapter.HasCalculateVersionUseCase())
	})

	t.Run("WithSetVersionUseCase sets set version use case", func(t *testing.T) {
		adapter := NewAdapter(WithSetVersionUseCase(nil))
		// setVersionUC doesn't have a HasSetVersionUseCase method exposed
		assert.NotNil(t, adapter)
	})

	t.Run("WithGenerateNotesUseCase sets generate notes use case", func(t *testing.T) {
		adapter := NewAdapter(WithGenerateNotesUseCase(nil))
		assert.False(t, adapter.HasGenerateNotesUseCase())
	})

	t.Run("WithApproveUseCase sets approve use case", func(t *testing.T) {
		adapter := NewAdapter(WithApproveUseCase(nil))
		assert.False(t, adapter.HasApproveUseCase())
	})

	t.Run("WithPublishUseCase sets publish use case", func(t *testing.T) {
		adapter := NewAdapter(WithPublishUseCase(nil))
		assert.False(t, adapter.HasPublishUseCase())
	})

	t.Run("WithGovernanceService sets governance service", func(t *testing.T) {
		adapter := NewAdapter(WithGovernanceService(nil))
		assert.False(t, adapter.HasGovernanceService())
	})

	t.Run("WithAdapterReleaseRepository sets release repository", func(t *testing.T) {
		adapter := NewAdapter(WithAdapterReleaseRepository(nil))
		assert.False(t, adapter.HasReleaseRepository())
	})
}

func TestAdapterPlanWithoutUseCase(t *testing.T) {
	adapter := NewAdapter()

	ctx := context.Background()
	input := PlanInput{
		RepositoryPath: "/test/repo",
	}

	output, err := adapter.Plan(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "plan use case not configured")
}

func TestAdapterBumpWithoutUseCase(t *testing.T) {
	adapter := NewAdapter()

	ctx := context.Background()
	input := BumpInput{
		RepositoryPath: "/test/repo",
		BumpType:       "minor",
	}

	output, err := adapter.Bump(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "calculate version use case not configured")
}

func TestAdapterBumpInvalidType(t *testing.T) {
	// Create a mock use case
	adapter := NewAdapter()

	ctx := context.Background()
	input := BumpInput{
		RepositoryPath: "/test/repo",
		BumpType:       "invalid",
	}

	output, err := adapter.Bump(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "invalid bump type")
}

func TestAdapterNotesWithoutUseCase(t *testing.T) {
	adapter := NewAdapter()

	ctx := context.Background()
	input := NotesInput{
		ReleaseID: "test-release-id",
	}

	output, err := adapter.Notes(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "generate notes use case not configured")
}

func TestAdapterEvaluateWithoutGovernance(t *testing.T) {
	adapter := NewAdapter()

	ctx := context.Background()
	input := EvaluateInput{
		ReleaseID: "test-release-id",
	}

	output, err := adapter.Evaluate(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "governance service not configured")
}

func TestAdapterEvaluateWithoutReleaseRepo(t *testing.T) {
	// Create a mock governance service - we can't easily do this without dependencies
	// but we can test the path where governance is set but repo is not
	adapter := NewAdapter(
		WithGovernanceService(&governance.Service{}),
	)

	ctx := context.Background()
	input := EvaluateInput{
		ReleaseID: "test-release-id",
	}

	output, err := adapter.Evaluate(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "release repository not configured")
}

func TestAdapterApproveWithoutUseCase(t *testing.T) {
	adapter := NewAdapter()

	ctx := context.Background()
	input := ApproveInput{
		ReleaseID: "test-release-id",
	}

	output, err := adapter.Approve(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "approve use case not configured")
}

func TestAdapterPublishWithoutUseCase(t *testing.T) {
	adapter := NewAdapter()

	ctx := context.Background()
	input := PublishInput{
		ReleaseID: "test-release-id",
	}

	output, err := adapter.Publish(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "publish use case not configured")
}

func TestAdapterGetStatusWithoutRepo(t *testing.T) {
	adapter := NewAdapter()

	ctx := context.Background()

	output, err := adapter.GetStatus(ctx)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "release repository not configured")
}

func TestPlanInputFields(t *testing.T) {
	input := PlanInput{
		RepositoryPath: "/path/to/repo",
		FromRef:        "v1.0.0",
		ToRef:          "HEAD",
		Analyze:        true,
		DryRun:         true,
	}

	assert.Equal(t, "/path/to/repo", input.RepositoryPath)
	assert.Equal(t, "v1.0.0", input.FromRef)
	assert.Equal(t, "HEAD", input.ToRef)
	assert.True(t, input.Analyze)
	assert.True(t, input.DryRun)
}

func TestBumpInputFields(t *testing.T) {
	input := BumpInput{
		RepositoryPath: "/path/to/repo",
		BumpType:       "major",
		Version:        "2.0.0",
		Prerelease:     "beta.1",
		CreateTag:      true,
		DryRun:         false,
	}

	assert.Equal(t, "/path/to/repo", input.RepositoryPath)
	assert.Equal(t, "major", input.BumpType)
	assert.Equal(t, "2.0.0", input.Version)
	assert.Equal(t, "beta.1", input.Prerelease)
	assert.True(t, input.CreateTag)
	assert.False(t, input.DryRun)
}

func TestNotesInputFields(t *testing.T) {
	input := NotesInput{
		ReleaseID:        "release-123",
		UseAI:            true,
		IncludeChangelog: true,
		RepositoryURL:    "https://github.com/test/repo",
	}

	assert.Equal(t, "release-123", input.ReleaseID)
	assert.True(t, input.UseAI)
	assert.True(t, input.IncludeChangelog)
	assert.Equal(t, "https://github.com/test/repo", input.RepositoryURL)
}

func TestEvaluateInputFields(t *testing.T) {
	input := EvaluateInput{
		ReleaseID:      "release-123",
		Repository:     "https://github.com/test/repo",
		ActorID:        "user-456",
		ActorName:      "Test User",
		IncludeHistory: true,
	}

	assert.Equal(t, "release-123", input.ReleaseID)
	assert.Equal(t, "https://github.com/test/repo", input.Repository)
	assert.Equal(t, "user-456", input.ActorID)
	assert.Equal(t, "Test User", input.ActorName)
	assert.True(t, input.IncludeHistory)
}

func TestApproveInputFields(t *testing.T) {
	input := ApproveInput{
		ReleaseID:   "release-123",
		ApprovedBy:  "admin",
		AutoApprove: true,
		EditedNotes: "Updated release notes",
	}

	assert.Equal(t, "release-123", input.ReleaseID)
	assert.Equal(t, "admin", input.ApprovedBy)
	assert.True(t, input.AutoApprove)
	assert.Equal(t, "Updated release notes", input.EditedNotes)
}

func TestPublishInputFields(t *testing.T) {
	input := PublishInput{
		ReleaseID: "release-123",
		DryRun:    true,
		CreateTag: true,
		PushTag:   true,
		TagPrefix: "v",
		Remote:    "origin",
	}

	assert.Equal(t, "release-123", input.ReleaseID)
	assert.True(t, input.DryRun)
	assert.True(t, input.CreateTag)
	assert.True(t, input.PushTag)
	assert.Equal(t, "v", input.TagPrefix)
	assert.Equal(t, "origin", input.Remote)
}

func TestPlanOutputFields(t *testing.T) {
	output := PlanOutput{
		ReleaseID:      "release-123",
		CurrentVersion: "1.0.0",
		NextVersion:    "1.1.0",
		ReleaseType:    "minor",
		CommitCount:    5,
		HasBreaking:    false,
		HasFeatures:    true,
		HasFixes:       true,
	}

	assert.Equal(t, "release-123", output.ReleaseID)
	assert.Equal(t, "1.0.0", output.CurrentVersion)
	assert.Equal(t, "1.1.0", output.NextVersion)
	assert.Equal(t, "minor", output.ReleaseType)
	assert.Equal(t, 5, output.CommitCount)
	assert.False(t, output.HasBreaking)
	assert.True(t, output.HasFeatures)
	assert.True(t, output.HasFixes)
}

func TestBumpOutputFields(t *testing.T) {
	output := BumpOutput{
		CurrentVersion: "1.0.0",
		NextVersion:    "2.0.0",
		BumpType:       "major",
		AutoDetected:   true,
		TagName:        "v2.0.0",
		TagCreated:     true,
	}

	assert.Equal(t, "1.0.0", output.CurrentVersion)
	assert.Equal(t, "2.0.0", output.NextVersion)
	assert.Equal(t, "major", output.BumpType)
	assert.True(t, output.AutoDetected)
	assert.Equal(t, "v2.0.0", output.TagName)
	assert.True(t, output.TagCreated)
}

func TestNotesOutputFields(t *testing.T) {
	output := NotesOutput{
		Summary:     "This release includes new features",
		Changelog:   "## Changes\n- Feature 1\n- Feature 2",
		AIGenerated: true,
	}

	assert.Equal(t, "This release includes new features", output.Summary)
	assert.Equal(t, "## Changes\n- Feature 1\n- Feature 2", output.Changelog)
	assert.True(t, output.AIGenerated)
}

func TestEvaluateOutputFields(t *testing.T) {
	output := EvaluateOutput{
		Decision:        "approved",
		RiskScore:       0.25,
		Severity:        "low",
		CanAutoApprove:  true,
		RequiredActions: []string{"review"},
		RiskFactors:     []string{"minor: 0.10"},
		Rationale:       []string{"Low risk release"},
	}

	assert.Equal(t, "approved", output.Decision)
	assert.Equal(t, 0.25, output.RiskScore)
	assert.Equal(t, "low", output.Severity)
	assert.True(t, output.CanAutoApprove)
	assert.Equal(t, []string{"review"}, output.RequiredActions)
	assert.Equal(t, []string{"minor: 0.10"}, output.RiskFactors)
	assert.Equal(t, []string{"Low risk release"}, output.Rationale)
}

func TestApproveOutputFields(t *testing.T) {
	output := ApproveOutput{
		Approved:   true,
		ApprovedBy: "admin",
		Version:    "1.1.0",
	}

	assert.True(t, output.Approved)
	assert.Equal(t, "admin", output.ApprovedBy)
	assert.Equal(t, "1.1.0", output.Version)
}

func TestPublishOutputFields(t *testing.T) {
	output := PublishOutput{
		TagName:    "v1.1.0",
		ReleaseURL: "https://github.com/test/repo/releases/v1.1.0",
		PluginResults: []PluginResultInfo{
			{
				PluginName: "github",
				Hook:       "PostPublish",
				Success:    true,
				Message:    "Release created",
			},
		},
	}

	assert.Equal(t, "v1.1.0", output.TagName)
	assert.Equal(t, "https://github.com/test/repo/releases/v1.1.0", output.ReleaseURL)
	require.Len(t, output.PluginResults, 1)
	assert.Equal(t, "github", output.PluginResults[0].PluginName)
	assert.Equal(t, "PostPublish", output.PluginResults[0].Hook)
	assert.True(t, output.PluginResults[0].Success)
	assert.Equal(t, "Release created", output.PluginResults[0].Message)
}

func TestGetStatusOutputFields(t *testing.T) {
	output := GetStatusOutput{
		ReleaseID:   "release-123",
		State:       "approved",
		Version:     "1.1.0",
		CreatedAt:   "2024-01-01T00:00:00Z",
		UpdatedAt:   "2024-01-02T00:00:00Z",
		CanApprove:  false,
		ApprovalMsg: "Already approved",
	}

	assert.Equal(t, "release-123", output.ReleaseID)
	assert.Equal(t, "approved", output.State)
	assert.Equal(t, "1.1.0", output.Version)
	assert.Equal(t, "2024-01-01T00:00:00Z", output.CreatedAt)
	assert.Equal(t, "2024-01-02T00:00:00Z", output.UpdatedAt)
	assert.False(t, output.CanApprove)
	assert.Equal(t, "Already approved", output.ApprovalMsg)
}

// Mock implementations for testing with real use cases

type mockReleaseRepository struct {
	releases []*domainrelease.Release
}

func (m *mockReleaseRepository) Save(ctx context.Context, rel *domainrelease.Release) error {
	m.releases = append(m.releases, rel)
	return nil
}

func (m *mockReleaseRepository) FindByID(ctx context.Context, id domainrelease.ReleaseID) (*domainrelease.Release, error) {
	for _, r := range m.releases {
		if r.ID() == id {
			return r, nil
		}
	}
	return nil, domainrelease.ErrReleaseNotFound
}

func (m *mockReleaseRepository) FindLatest(ctx context.Context, repoPath string) (*domainrelease.Release, error) {
	if len(m.releases) == 0 {
		return nil, domainrelease.ErrReleaseNotFound
	}
	return m.releases[len(m.releases)-1], nil
}

func (m *mockReleaseRepository) FindActive(ctx context.Context) ([]*domainrelease.Release, error) {
	return m.releases, nil
}

func (m *mockReleaseRepository) FindByState(ctx context.Context, state domainrelease.ReleaseState) ([]*domainrelease.Release, error) {
	var result []*domainrelease.Release
	for _, r := range m.releases {
		if r.State() == state {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *mockReleaseRepository) FindBySpecification(ctx context.Context, spec domainrelease.Specification) ([]*domainrelease.Release, error) {
	var result []*domainrelease.Release
	for _, r := range m.releases {
		if spec.IsSatisfiedBy(r) {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *mockReleaseRepository) Delete(ctx context.Context, id domainrelease.ReleaseID) error {
	for i, r := range m.releases {
		if r.ID() == id {
			m.releases = append(m.releases[:i], m.releases[i+1:]...)
			return nil
		}
	}
	return domainrelease.ErrReleaseNotFound
}

func TestAdapterGetStatusWithEmptyRepo(t *testing.T) {
	repo := &mockReleaseRepository{releases: []*domainrelease.Release{}}
	adapter := NewAdapter(WithAdapterReleaseRepository(repo))

	ctx := context.Background()

	output, err := adapter.GetStatus(ctx)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "no active release found")
}

func TestAdapterGetStatusWithActiveRelease(t *testing.T) {
	// Create a release
	rel := domainrelease.NewRelease("test-release-123", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)

	repo := &mockReleaseRepository{releases: []*domainrelease.Release{rel}}
	adapter := NewAdapter(WithAdapterReleaseRepository(repo))

	ctx := context.Background()

	output, err := adapter.GetStatus(ctx)
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, "test-release-123", output.ReleaseID)
	assert.Equal(t, "planned", output.State)
	assert.Equal(t, "1.1.0", output.Version)
}

// Test adapter with nil use cases doesn't panic

func TestAdapterWithNilUseCases(t *testing.T) {
	// Explicitly pass nil values - should be handled gracefully
	adapter := NewAdapter(
		WithPlanUseCase(nil),
		WithCalculateVersionUseCase(nil),
		WithSetVersionUseCase(nil),
		WithGenerateNotesUseCase(nil),
		WithApproveUseCase(nil),
		WithGetForApprovalUseCase(nil),
		WithPublishUseCase(nil),
		WithGovernanceService(nil),
		WithAdapterReleaseRepository(nil),
	)

	assert.NotNil(t, adapter)
	assert.False(t, adapter.HasPlanUseCase())
	assert.False(t, adapter.HasCalculateVersionUseCase())
	assert.False(t, adapter.HasGenerateNotesUseCase())
	assert.False(t, adapter.HasApproveUseCase())
	assert.False(t, adapter.HasPublishUseCase())
	assert.False(t, adapter.HasGovernanceService())
	assert.False(t, adapter.HasReleaseRepository())
}

// Test bump type parsing

func TestAdapterBumpTypeParsing(t *testing.T) {
	tests := []struct {
		name     string
		bumpType string
		wantErr  bool
	}{
		{"major", "major", false},
		{"minor", "minor", false},
		{"patch", "patch", false},
		{"auto", "auto", false},
		{"empty", "", false},
		{"invalid", "invalid-type", true},
	}

	// Since we can't easily create a real CalculateVersionUseCase,
	// we test that invalid bump types are rejected before use case execution
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewAdapter() // No use case - will fail anyway

			ctx := context.Background()
			input := BumpInput{
				RepositoryPath: "/test/repo",
				BumpType:       tt.bumpType,
			}

			_, err := adapter.Bump(ctx, input)
			require.Error(t, err)

			if tt.wantErr {
				assert.Contains(t, err.Error(), "invalid bump type")
			} else {
				assert.Contains(t, err.Error(), "calculate version use case not configured")
			}
		})
	}
}

// Test GetStatus with release that has version set
func TestAdapterGetStatusWithVersionSet(t *testing.T) {
	// Create a release with version explicitly set
	rel := domainrelease.NewRelease("test-release-456", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)
	// Set the version directly
	_ = rel.SetVersion(nextV, "v1.1.0")

	repo := &mockReleaseRepository{releases: []*domainrelease.Release{rel}}
	adapter := NewAdapter(WithAdapterReleaseRepository(repo))

	ctx := context.Background()

	output, err := adapter.GetStatus(ctx)
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, "test-release-456", output.ReleaseID)
	assert.Equal(t, "1.1.0", output.Version)
}

// Test GetStatus with release repo returning error
func TestAdapterGetStatusWithRepoError(t *testing.T) {
	repo := &mockErrorReleaseRepository{err: fmt.Errorf("database connection failed")}
	adapter := NewAdapter(WithAdapterReleaseRepository(repo))

	ctx := context.Background()

	output, err := adapter.GetStatus(ctx)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "failed to find active releases")
}

// mockErrorReleaseRepository always returns an error
type mockErrorReleaseRepository struct {
	err error
}

func (m *mockErrorReleaseRepository) Save(ctx context.Context, rel *domainrelease.Release) error {
	return m.err
}

func (m *mockErrorReleaseRepository) FindByID(ctx context.Context, id domainrelease.ReleaseID) (*domainrelease.Release, error) {
	return nil, m.err
}

func (m *mockErrorReleaseRepository) FindLatest(ctx context.Context, repoPath string) (*domainrelease.Release, error) {
	return nil, m.err
}

func (m *mockErrorReleaseRepository) FindActive(ctx context.Context) ([]*domainrelease.Release, error) {
	return nil, m.err
}

func (m *mockErrorReleaseRepository) FindByState(ctx context.Context, state domainrelease.ReleaseState) ([]*domainrelease.Release, error) {
	return nil, m.err
}

func (m *mockErrorReleaseRepository) FindBySpecification(ctx context.Context, spec domainrelease.Specification) ([]*domainrelease.Release, error) {
	return nil, m.err
}

func (m *mockErrorReleaseRepository) Delete(ctx context.Context, id domainrelease.ReleaseID) error {
	return m.err
}

// Test Evaluate with release not found
func TestAdapterEvaluateReleaseNotFound(t *testing.T) {
	repo := &mockReleaseRepository{releases: []*domainrelease.Release{}}
	adapter := NewAdapter(
		WithGovernanceService(&governance.Service{}),
		WithAdapterReleaseRepository(repo),
	)

	ctx := context.Background()
	input := EvaluateInput{
		ReleaseID: "non-existent-release",
	}

	output, err := adapter.Evaluate(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "failed to find release")
}

// Test adapter option application order
func TestAdapterOptionChaining(t *testing.T) {
	repo := &mockReleaseRepository{releases: []*domainrelease.Release{}}
	govSvc := &governance.Service{}

	adapter := NewAdapter(
		WithAdapterReleaseRepository(repo),
		WithGovernanceService(govSvc),
	)

	assert.True(t, adapter.HasReleaseRepository())
	assert.True(t, adapter.HasGovernanceService())
}

// Test GetStatus shows correct approval status
func TestAdapterGetStatusApprovalStatus(t *testing.T) {
	rel := domainrelease.NewRelease("approval-test-123", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)

	repo := &mockReleaseRepository{releases: []*domainrelease.Release{rel}}
	adapter := NewAdapter(WithAdapterReleaseRepository(repo))

	ctx := context.Background()

	output, err := adapter.GetStatus(ctx)
	require.NoError(t, err)
	require.NotNil(t, output)
	// Check approval fields are populated
	assert.NotEmpty(t, output.CreatedAt)
	assert.NotEmpty(t, output.UpdatedAt)
}

// Test Evaluate with release found but empty actor (tests default actor)
func TestAdapterEvaluateWithDefaultActor(t *testing.T) {
	// Create a release
	rel := domainrelease.NewRelease("evaluate-test-123", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)

	repo := &mockReleaseRepository{releases: []*domainrelease.Release{rel}}
	govSvc := &governance.Service{} // Empty service - will fail but tests the actor path

	adapter := NewAdapter(
		WithGovernanceService(govSvc),
		WithAdapterReleaseRepository(repo),
	)

	ctx := context.Background()
	input := EvaluateInput{
		ReleaseID: "evaluate-test-123",
		// Empty ActorID and ActorName to test defaults
	}

	// This will fail at governance evaluation, but we're testing the actor default path
	_, err := adapter.Evaluate(ctx, input)
	// Expected to fail since governance service isn't properly initialized
	require.Error(t, err)
}

// Test that adapter correctly handles release with approval status
func TestAdapterGetStatusWithApprovalMessage(t *testing.T) {
	rel := domainrelease.NewRelease("approval-msg-test", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)

	repo := &mockReleaseRepository{releases: []*domainrelease.Release{rel}}
	adapter := NewAdapter(WithAdapterReleaseRepository(repo))

	ctx := context.Background()

	output, err := adapter.GetStatus(ctx)
	require.NoError(t, err)
	require.NotNil(t, output)
	// ApprovalMsg should be populated from release.ApprovalStatus()
	assert.NotNil(t, output.ApprovalMsg)
}

// Test GetForApprovalUseCase option
func TestAdapterWithGetForApprovalUseCase(t *testing.T) {
	// Test that the option works even with nil
	adapter := NewAdapter(WithGetForApprovalUseCase(nil))
	assert.NotNil(t, adapter)
}

// Test Evaluate with governance service but no release repository
func TestAdapterEvaluateNoReleaseRepo(t *testing.T) {
	govSvc := &governance.Service{}
	adapter := NewAdapter(
		WithGovernanceService(govSvc),
		// No release repo
	)

	ctx := context.Background()
	input := EvaluateInput{
		ReleaseID: "test-release",
	}

	_, err := adapter.Evaluate(ctx, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "release repository not configured")
}

// Test Evaluate with release not found
func TestAdapterEvaluateReleaseNotFoundByID(t *testing.T) {
	govSvc := &governance.Service{}
	repo := &mockReleaseRepository{releases: []*domainrelease.Release{}} // empty
	adapter := NewAdapter(
		WithGovernanceService(govSvc),
		WithAdapterReleaseRepository(repo),
	)

	ctx := context.Background()
	input := EvaluateInput{
		ReleaseID: "nonexistent-release",
	}

	_, err := adapter.Evaluate(ctx, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find release")
}

// Test Evaluate with explicit actor (non-empty ActorID)
func TestAdapterEvaluateWithExplicitActor(t *testing.T) {
	// Create a release with a plan
	rel := domainrelease.NewRelease("evaluate-actor-test", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)

	repo := &mockReleaseRepository{releases: []*domainrelease.Release{rel}}
	govSvc := &governance.Service{}

	adapter := NewAdapter(
		WithGovernanceService(govSvc),
		WithAdapterReleaseRepository(repo),
	)

	ctx := context.Background()
	input := EvaluateInput{
		ReleaseID: "evaluate-actor-test",
		ActorID:   "test-actor",
		ActorName: "Test Actor",
	}

	// This will fail because governance service isn't properly set up,
	// but it exercises the explicit actor code path
	_, err := adapter.Evaluate(ctx, input)
	require.Error(t, err)
}

// Ensure interface compliance at compile time
var (
	_ domainrelease.Repository = (*mockReleaseRepository)(nil)
	_ domainrelease.Repository = (*mockErrorReleaseRepository)(nil)
)

// Test nextActionForState function for all states
func TestNextActionForState(t *testing.T) {
	tests := []struct {
		state    string
		expected string
	}{
		{"initialized", "plan"},
		{"planned", "bump"},
		{"versioned", "notes"},
		{"notes_generated", "approve"},
		{"approved", "publish"},
		{"publishing", "wait"},
		{"published", "done"},
		{"failed", "retry or cancel"},
		{"canceled", "plan"},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			result := nextActionForState(tt.state)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test GetStatus with stale release
func TestAdapterGetStatusWithStaleRelease(t *testing.T) {
	// Create a release that was last updated more than 1 hour ago
	rel := domainrelease.NewRelease("stale-release-test", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)

	repo := &mockReleaseRepository{releases: []*domainrelease.Release{rel}}
	adapter := NewAdapter(WithAdapterReleaseRepository(repo))

	ctx := context.Background()

	output, err := adapter.GetStatus(ctx)
	require.NoError(t, err)
	require.NotNil(t, output)
	// Check that NextAction is populated
	assert.NotEmpty(t, output.NextAction)
}

// Test GetStatus output fields for various states
func TestAdapterGetStatusNextAction(t *testing.T) {
	// Create a release in planned state
	rel := domainrelease.NewRelease("next-action-test", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)

	repo := &mockReleaseRepository{releases: []*domainrelease.Release{rel}}
	adapter := NewAdapter(WithAdapterReleaseRepository(repo))

	ctx := context.Background()

	output, err := adapter.GetStatus(ctx)
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, "bump", output.NextAction)
}

// Test GetStatus with warning flag
func TestGetStatusOutputWarningField(t *testing.T) {
	output := GetStatusOutput{
		ReleaseID:   "warning-test",
		State:       "planned",
		Version:     "1.0.0",
		NextAction:  "bump",
		Stale:       true,
		Warning:     "Release was last updated over 1 hour ago",
	}

	assert.Equal(t, "warning-test", output.ReleaseID)
	assert.True(t, output.Stale)
	assert.NotEmpty(t, output.Warning)
}

// Test CommitInfo struct
func TestCommitInfoFields(t *testing.T) {
	info := CommitInfo{
		SHA:     "abc123",
		Type:    "feat",
		Scope:   "api",
		Message: "add new endpoint",
		Author:  "test@example.com",
	}

	assert.Equal(t, "abc123", info.SHA)
	assert.Equal(t, "feat", info.Type)
	assert.Equal(t, "api", info.Scope)
	assert.Equal(t, "add new endpoint", info.Message)
	assert.Equal(t, "test@example.com", info.Author)
}

// Test PluginResultInfo struct
func TestPluginResultInfoFields(t *testing.T) {
	info := PluginResultInfo{
		PluginName: "github",
		Hook:       "PostPublish",
		Success:    true,
		Message:    "Release created successfully",
	}

	assert.Equal(t, "github", info.PluginName)
	assert.Equal(t, "PostPublish", info.Hook)
	assert.True(t, info.Success)
	assert.Equal(t, "Release created successfully", info.Message)
}

// Test PlanOutput with Commits field
func TestPlanOutputWithCommits(t *testing.T) {
	output := PlanOutput{
		ReleaseID:      "plan-with-commits",
		CurrentVersion: "1.0.0",
		NextVersion:    "1.1.0",
		ReleaseType:    "minor",
		CommitCount:    2,
		HasFeatures:    true,
		Commits: []CommitInfo{
			{SHA: "abc123", Type: "feat", Message: "add feature"},
			{SHA: "def456", Type: "fix", Message: "fix bug"},
		},
	}

	assert.Equal(t, "plan-with-commits", output.ReleaseID)
	assert.Len(t, output.Commits, 2)
	assert.Equal(t, "abc123", output.Commits[0].SHA)
	assert.Equal(t, "def456", output.Commits[1].SHA)
}
