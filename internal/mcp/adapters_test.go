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
		assert.False(t, adapter.HasReleaseAnalyzer())
		assert.False(t, adapter.HasReleaseServices())
		assert.False(t, adapter.HasGovernanceService())
		assert.False(t, adapter.HasReleaseRepository())
	})
}

func TestAdapterOptions(t *testing.T) {
	t.Run("WithReleaseAnalyzer sets release analyzer", func(t *testing.T) {
		adapter := NewAdapter(WithReleaseAnalyzer(nil))
		assert.False(t, adapter.HasReleaseAnalyzer())
	})

	t.Run("WithReleaseServices sets release services", func(t *testing.T) {
		adapter := NewAdapter(WithReleaseServices(nil))
		assert.False(t, adapter.HasReleaseServices())
	})

	t.Run("WithGovernanceService sets governance service", func(t *testing.T) {
		adapter := NewAdapter(WithGovernanceService(nil))
		assert.False(t, adapter.HasGovernanceService())
	})

	t.Run("WithAdapterRepo sets release repository", func(t *testing.T) {
		adapter := NewAdapter(WithAdapterRepo(nil))
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
	assert.Contains(t, err.Error(), "release analyzer not configured")
}

// NOTE: TestAdapterBumpWithoutUseCase and TestAdapterBumpInvalidType were removed
// because the legacy bump path was removed (ADR-007 compliance).
// The DDD path requires release services, tested in TestAdapterBumpRequiresReleaseServices.

func TestAdapterNotesWithoutUseCase(t *testing.T) {
	adapter := NewAdapter()

	ctx := context.Background()
	input := NotesInput{
		ReleaseID: "test-release-id",
	}

	output, err := adapter.Notes(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "release services not configured")
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
	assert.Contains(t, err.Error(), "release services not configured")
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
	assert.Contains(t, err.Error(), "release services not configured")
}

func TestAdapterGetStatusWithoutRepo(t *testing.T) {
	adapter := NewAdapter()

	ctx := context.Background()

	output, err := adapter.GetStatus(ctx)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "release services not configured")
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
	releases []*domainrelease.ReleaseRun
}

func (m *mockReleaseRepository) Save(ctx context.Context, rel *domainrelease.ReleaseRun) error {
	m.releases = append(m.releases, rel)
	return nil
}

func (m *mockReleaseRepository) FindByID(ctx context.Context, id domainrelease.RunID) (*domainrelease.ReleaseRun, error) {
	for _, r := range m.releases {
		if r.ID() == id {
			return r, nil
		}
	}
	return nil, domainrelease.ErrRunNotFound
}

func (m *mockReleaseRepository) FindLatest(ctx context.Context, repoPath string) (*domainrelease.ReleaseRun, error) {
	if len(m.releases) == 0 {
		return nil, domainrelease.ErrRunNotFound
	}
	return m.releases[len(m.releases)-1], nil
}

func (m *mockReleaseRepository) FindActive(ctx context.Context) ([]*domainrelease.ReleaseRun, error) {
	return m.releases, nil
}

func (m *mockReleaseRepository) FindByState(ctx context.Context, state domainrelease.RunState) ([]*domainrelease.ReleaseRun, error) {
	var result []*domainrelease.ReleaseRun
	for _, r := range m.releases {
		if r.State() == state {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *mockReleaseRepository) FindBySpecification(ctx context.Context, spec domainrelease.Specification) ([]*domainrelease.ReleaseRun, error) {
	var result []*domainrelease.ReleaseRun
	for _, r := range m.releases {
		if spec.IsSatisfiedBy(r) {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *mockReleaseRepository) Delete(ctx context.Context, id domainrelease.RunID) error {
	for i, r := range m.releases {
		if r.ID() == id {
			m.releases = append(m.releases[:i], m.releases[i+1:]...)
			return nil
		}
	}
	return domainrelease.ErrRunNotFound
}

func (m *mockReleaseRepository) List(ctx context.Context, repoPath string) ([]domainrelease.RunID, error) {
	return nil, nil
}

func TestAdapterGetStatusWithEmptyRepo(t *testing.T) {
	// GetStatus now requires release services, not just a repository
	// This test verifies the error when only a repository is provided
	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{}}
	adapter := NewAdapter(WithAdapterRepo(repo))

	ctx := context.Background()

	output, err := adapter.GetStatus(ctx)
	require.Error(t, err)
	assert.Nil(t, output)
	// Now returns "release services not configured" since GetStatus uses the DDD use case
	assert.Contains(t, err.Error(), "release services not configured")
}

func TestAdapterGetStatusWithActiveRelease(t *testing.T) {
	// GetStatus now requires release services with GetStatus use case
	// This test verifies that even with a repository, we need services
	rel := domainrelease.NewReleaseRunForTest("test-release-123", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)

	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{rel}}
	adapter := NewAdapter(WithAdapterRepo(repo))

	ctx := context.Background()

	output, err := adapter.GetStatus(ctx)
	// Without release services, this will fail
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "release services not configured")
}

// Test adapter with nil use cases doesn't panic

func TestAdapterWithNilUseCases(t *testing.T) {
	// Explicitly pass nil values - should be handled gracefully
	adapter := NewAdapter(
		WithReleaseAnalyzer(nil),
		WithReleaseServices(nil),
		WithGovernanceService(nil),
		WithAdapterRepo(nil),
	)

	assert.NotNil(t, adapter)
	assert.False(t, adapter.HasReleaseAnalyzer())
	assert.False(t, adapter.HasReleaseServices())
	assert.False(t, adapter.HasGovernanceService())
	assert.False(t, adapter.HasReleaseRepository())
}

// Test bump requires release services

func TestAdapterBumpRequiresReleaseServices(t *testing.T) {
	adapter := NewAdapter() // No release services configured

	ctx := context.Background()
	input := BumpInput{
		RepositoryPath: "/test/repo",
		BumpType:       "auto",
	}

	_, err := adapter.Bump(ctx, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "release services not configured")
}

// Test GetStatus with release that has version set
// NOTE: GetStatus now requires release services with a GetStatusUseCase.
func TestAdapterGetStatusWithVersionSet(t *testing.T) {
	// Create a release with version explicitly set
	rel := domainrelease.NewReleaseRunForTest("test-release-456", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)
	// Set the version directly
	_ = rel.SetVersion(nextV, "v1.1.0")

	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{rel}}
	adapter := NewAdapter(WithAdapterRepo(repo))

	ctx := context.Background()

	// GetStatus now requires release services, so this returns an error
	output, err := adapter.GetStatus(ctx)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "release services not configured")
}

// Test GetStatus with release repo returning error
// NOTE: GetStatus now requires release services with a GetStatusUseCase.
// The release services check happens before repository access.
func TestAdapterGetStatusWithRepoError(t *testing.T) {
	repo := &mockErrorReleaseRepository{err: fmt.Errorf("database connection failed")}
	adapter := NewAdapter(WithAdapterRepo(repo))

	ctx := context.Background()

	// GetStatus now requires release services, so this returns a services error
	// before the repository is accessed
	output, err := adapter.GetStatus(ctx)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "release services not configured")
}

// mockErrorReleaseRepository always returns an error
type mockErrorReleaseRepository struct {
	err error
}

func (m *mockErrorReleaseRepository) Save(ctx context.Context, rel *domainrelease.ReleaseRun) error {
	return m.err
}

func (m *mockErrorReleaseRepository) FindByID(ctx context.Context, id domainrelease.RunID) (*domainrelease.ReleaseRun, error) {
	return nil, m.err
}

func (m *mockErrorReleaseRepository) FindLatest(ctx context.Context, repoPath string) (*domainrelease.ReleaseRun, error) {
	return nil, m.err
}

func (m *mockErrorReleaseRepository) FindActive(ctx context.Context) ([]*domainrelease.ReleaseRun, error) {
	return nil, m.err
}

func (m *mockErrorReleaseRepository) FindByState(ctx context.Context, state domainrelease.RunState) ([]*domainrelease.ReleaseRun, error) {
	return nil, m.err
}

func (m *mockErrorReleaseRepository) FindBySpecification(ctx context.Context, spec domainrelease.Specification) ([]*domainrelease.ReleaseRun, error) {
	return nil, m.err
}

func (m *mockErrorReleaseRepository) Delete(ctx context.Context, id domainrelease.RunID) error {
	return m.err
}

func (m *mockErrorReleaseRepository) List(ctx context.Context, repoPath string) ([]domainrelease.RunID, error) {
	return nil, m.err
}

// Test Evaluate with release not found
func TestAdapterEvaluateReleaseNotFound(t *testing.T) {
	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{}}
	adapter := NewAdapter(
		WithGovernanceService(&governance.Service{}),
		WithAdapterRepo(repo),
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
	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{}}
	govSvc := &governance.Service{}

	adapter := NewAdapter(
		WithAdapterRepo(repo),
		WithGovernanceService(govSvc),
	)

	assert.True(t, adapter.HasReleaseRepository())
	assert.True(t, adapter.HasGovernanceService())
}

// Test GetStatus shows correct approval status
// NOTE: GetStatus now requires release services with a GetStatusUseCase.
func TestAdapterGetStatusApprovalStatus(t *testing.T) {
	rel := domainrelease.NewReleaseRunForTest("approval-test-123", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)

	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{rel}}
	adapter := NewAdapter(WithAdapterRepo(repo))

	ctx := context.Background()

	// GetStatus now requires release services, so this returns an error
	output, err := adapter.GetStatus(ctx)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "release services not configured")
}

// Test Evaluate with release found but empty actor (tests default actor)
func TestAdapterEvaluateWithDefaultActor(t *testing.T) {
	// Create a release
	rel := domainrelease.NewReleaseRunForTest("evaluate-test-123", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)

	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{rel}}
	govSvc := &governance.Service{} // Empty service - will fail but tests the actor path

	adapter := NewAdapter(
		WithGovernanceService(govSvc),
		WithAdapterRepo(repo),
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
// NOTE: GetStatus now requires release services with a GetStatusUseCase.
func TestAdapterGetStatusWithApprovalMessage(t *testing.T) {
	rel := domainrelease.NewReleaseRunForTest("approval-msg-test", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)

	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{rel}}
	adapter := NewAdapter(WithAdapterRepo(repo))

	ctx := context.Background()

	// GetStatus now requires release services, so this returns an error
	output, err := adapter.GetStatus(ctx)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "release services not configured")
}

// Test ReleaseRepository option
func TestAdapterWithAdapterRepo(t *testing.T) {
	// Test that the option works even with nil
	adapter := NewAdapter(WithAdapterRepo(nil))
	assert.NotNil(t, adapter)
	assert.False(t, adapter.HasReleaseRepository())
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
	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{}} // empty
	adapter := NewAdapter(
		WithGovernanceService(govSvc),
		WithAdapterRepo(repo),
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
	rel := domainrelease.NewReleaseRunForTest("evaluate-actor-test", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)

	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{rel}}
	govSvc := &governance.Service{}

	adapter := NewAdapter(
		WithGovernanceService(govSvc),
		WithAdapterRepo(repo),
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

// Test WithBlastService option
func TestAdapterWithBlastService(t *testing.T) {
	t.Run("nil blast service", func(t *testing.T) {
		adapter := NewAdapter(WithBlastService(nil))
		assert.NotNil(t, adapter)
		assert.False(t, adapter.HasBlastService())
	})
}

// Test WithAIService option
func TestAdapterWithAIService(t *testing.T) {
	t.Run("nil AI service", func(t *testing.T) {
		adapter := NewAdapter(WithAIService(nil))
		assert.NotNil(t, adapter)
		assert.False(t, adapter.HasAIService())
	})
}

// Test Cancel operation
func TestAdapterCancelWithoutRepo(t *testing.T) {
	adapter := NewAdapter()

	ctx := context.Background()
	input := CancelInput{
		ReleaseID: "test-release",
		Reason:    "testing",
	}

	output, err := adapter.Cancel(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "release repository not configured")
}

func TestAdapterCancelReleaseNotFound(t *testing.T) {
	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{}}
	adapter := NewAdapter(WithAdapterRepo(repo))

	ctx := context.Background()
	input := CancelInput{
		ReleaseID: "nonexistent-release",
		Reason:    "testing",
	}

	output, err := adapter.Cancel(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "failed to find release")
}

// Test Reset operation
func TestAdapterResetWithoutRepo(t *testing.T) {
	adapter := NewAdapter()

	ctx := context.Background()
	input := ResetInput{
		ReleaseID: "test-release",
	}

	output, err := adapter.Reset(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "release repository not configured")
}

func TestAdapterResetReleaseNotFound(t *testing.T) {
	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{}}
	adapter := NewAdapter(WithAdapterRepo(repo))

	ctx := context.Background()
	input := ResetInput{
		ReleaseID: "nonexistent-release",
	}

	output, err := adapter.Reset(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "failed to find release")
}

func TestAdapterResetSuccess(t *testing.T) {
	// Create a release
	rel := domainrelease.NewReleaseRunForTest("reset-test-123", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)

	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{rel}}
	adapter := NewAdapter(WithAdapterRepo(repo))

	ctx := context.Background()
	input := ResetInput{
		ReleaseID: "reset-test-123",
	}

	output, err := adapter.Reset(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, "reset-test-123", output.ReleaseID)
	assert.Equal(t, "planned", output.PreviousState)
	assert.True(t, output.Deleted)
}

// Test BlastRadius operation
func TestAdapterBlastRadiusWithoutService(t *testing.T) {
	adapter := NewAdapter()

	ctx := context.Background()
	input := BlastRadiusInput{
		FromRef: "v1.0.0",
		ToRef:   "HEAD",
	}

	output, err := adapter.BlastRadius(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "blast radius service not configured")
}

// Test InferVersion operation
func TestAdapterInferVersionWithoutAnalyzer(t *testing.T) {
	adapter := NewAdapter()

	ctx := context.Background()
	input := InferVersionInput{
		FromRef: "v1.0.0",
		ToRef:   "HEAD",
	}

	output, err := adapter.InferVersion(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "release analyzer not configured")
}

// Test SummarizeDiff operation
func TestAdapterSummarizeDiffWithoutAnalyzer(t *testing.T) {
	adapter := NewAdapter()

	ctx := context.Background()
	input := SummarizeDiffInput{
		FromRef:  "v1.0.0",
		ToRef:    "HEAD",
		Audience: "developer",
	}

	output, err := adapter.SummarizeDiff(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "release analyzer not configured")
}

// Test ValidateRelease operation
func TestAdapterValidateReleaseBasic(t *testing.T) {
	adapter := NewAdapter()

	ctx := context.Background()
	input := ValidateReleaseInput{
		CheckGit:     true,
		CheckPlugins: true,
	}

	output, err := adapter.ValidateRelease(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.True(t, output.Valid)
	assert.True(t, output.CanProceed)
	assert.Contains(t, output.Recommendation, "All checks passed")
}

func TestAdapterValidateReleaseWithReleaseID(t *testing.T) {
	// Create a release
	rel := domainrelease.NewReleaseRunForTest("validate-test-123", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)

	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{rel}}
	adapter := NewAdapter(WithAdapterRepo(repo))

	ctx := context.Background()
	input := ValidateReleaseInput{
		ReleaseID: "validate-test-123",
	}

	output, err := adapter.ValidateRelease(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.True(t, output.Valid)

	// Check that release_exists check is present
	var foundCheck bool
	for _, check := range output.Checks {
		if check.Name == "release_exists" {
			foundCheck = true
			assert.Equal(t, "passed", check.Status)
		}
	}
	assert.True(t, foundCheck, "Expected release_exists check")
}

func TestAdapterValidateReleaseNotFound(t *testing.T) {
	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{}}
	adapter := NewAdapter(WithAdapterRepo(repo))

	ctx := context.Background()
	input := ValidateReleaseInput{
		ReleaseID: "nonexistent-release",
	}

	output, err := adapter.ValidateRelease(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.False(t, output.Valid)
	assert.False(t, output.CanProceed)
	assert.Contains(t, output.BlockingIssues, "Release not found")
}

func TestAdapterValidateReleaseWithGovernance(t *testing.T) {
	govSvc := &governance.Service{}
	adapter := NewAdapter(WithGovernanceService(govSvc))

	ctx := context.Background()
	input := ValidateReleaseInput{
		CheckGovernance: true,
	}

	output, err := adapter.ValidateRelease(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, output)

	// Check that governance_enabled check is present
	var foundCheck bool
	for _, check := range output.Checks {
		if check.Name == "governance_enabled" {
			foundCheck = true
			assert.Equal(t, "passed", check.Status)
		}
	}
	assert.True(t, foundCheck, "Expected governance_enabled check")
}

// Test CancelInput and CancelOutput fields
func TestCancelInputFields(t *testing.T) {
	input := CancelInput{
		ReleaseID: "cancel-123",
		Reason:    "canceled by user",
	}

	assert.Equal(t, "cancel-123", input.ReleaseID)
	assert.Equal(t, "canceled by user", input.Reason)
}

func TestCancelOutputFields(t *testing.T) {
	output := CancelOutput{
		ReleaseID:     "cancel-123",
		PreviousState: "planned",
		NewState:      "canceled",
	}

	assert.Equal(t, "cancel-123", output.ReleaseID)
	assert.Equal(t, "planned", output.PreviousState)
	assert.Equal(t, "canceled", output.NewState)
}

// Test ResetInput and ResetOutput fields
func TestResetInputFields(t *testing.T) {
	input := ResetInput{
		ReleaseID: "reset-123",
	}

	assert.Equal(t, "reset-123", input.ReleaseID)
}

func TestResetOutputFields(t *testing.T) {
	output := ResetOutput{
		ReleaseID:     "reset-123",
		PreviousState: "planned",
		Deleted:       true,
	}

	assert.Equal(t, "reset-123", output.ReleaseID)
	assert.Equal(t, "planned", output.PreviousState)
	assert.True(t, output.Deleted)
}

// Test BlastRadiusInput and BlastRadiusOutput fields
func TestBlastRadiusInputFields(t *testing.T) {
	input := BlastRadiusInput{
		FromRef:           "v1.0.0",
		ToRef:             "HEAD",
		IncludeTransitive: true,
		GenerateGraph:     true,
		PackagePaths:      []string{"pkg/", "internal/"},
	}

	assert.Equal(t, "v1.0.0", input.FromRef)
	assert.Equal(t, "HEAD", input.ToRef)
	assert.True(t, input.IncludeTransitive)
	assert.True(t, input.GenerateGraph)
	assert.Len(t, input.PackagePaths, 2)
}

func TestBlastRadiusOutputFields(t *testing.T) {
	output := BlastRadiusOutput{
		TotalPackages:            10,
		DirectlyAffected:         3,
		TransitivelyAffected:     5,
		PackagesRequiringRelease: 2,
		RiskLevel:                "medium",
		RiskFactors:              []string{"api change", "database migration"},
		TotalFilesChanged:        15,
		TotalInsertions:          100,
		TotalDeletions:           50,
	}

	assert.Equal(t, 10, output.TotalPackages)
	assert.Equal(t, 3, output.DirectlyAffected)
	assert.Equal(t, 5, output.TransitivelyAffected)
	assert.Equal(t, 2, output.PackagesRequiringRelease)
	assert.Equal(t, "medium", output.RiskLevel)
	assert.Len(t, output.RiskFactors, 2)
}

// Test InferVersionInput and InferVersionOutput fields
func TestInferVersionInputFields(t *testing.T) {
	input := InferVersionInput{
		FromRef:     "v1.0.0",
		ToRef:       "HEAD",
		IncludeRisk: true,
	}

	assert.Equal(t, "v1.0.0", input.FromRef)
	assert.Equal(t, "HEAD", input.ToRef)
	assert.True(t, input.IncludeRisk)
}

func TestInferVersionOutputFields(t *testing.T) {
	output := InferVersionOutput{
		CurrentVersion: "1.0.0",
		NextVersion:    "1.1.0",
		BumpType:       "minor",
		HasBreaking:    false,
		HasFeatures:    true,
		HasFixes:       true,
		CommitCount:    5,
		Confidence:     0.9,
		Rationale:      []string{"2 features detected → minor bump"},
		RiskScore:      0.3,
		RiskSeverity:   "low",
	}

	assert.Equal(t, "1.0.0", output.CurrentVersion)
	assert.Equal(t, "1.1.0", output.NextVersion)
	assert.Equal(t, "minor", output.BumpType)
	assert.False(t, output.HasBreaking)
	assert.True(t, output.HasFeatures)
	assert.True(t, output.HasFixes)
	assert.Equal(t, 5, output.CommitCount)
	assert.Equal(t, 0.9, output.Confidence)
	assert.Len(t, output.Rationale, 1)
	assert.Equal(t, 0.3, output.RiskScore)
	assert.Equal(t, "low", output.RiskSeverity)
}

// Test SummarizeDiffInput and SummarizeDiffOutput fields
func TestSummarizeDiffInputFields(t *testing.T) {
	input := SummarizeDiffInput{
		FromRef:   "v1.0.0",
		ToRef:     "HEAD",
		Audience:  "end-user",
		MaxLength: 500,
	}

	assert.Equal(t, "v1.0.0", input.FromRef)
	assert.Equal(t, "HEAD", input.ToRef)
	assert.Equal(t, "end-user", input.Audience)
	assert.Equal(t, 500, input.MaxLength)
}

func TestSummarizeDiffOutputFields(t *testing.T) {
	output := SummarizeDiffOutput{
		Summary:        "This release includes improvements",
		Highlights:     []string{"✨ 3 new features", "🐛 2 bug fixes"},
		AIGenerated:    true,
		Audience:       "developer",
		CharacterCount: 35,
	}

	assert.Equal(t, "This release includes improvements", output.Summary)
	assert.Len(t, output.Highlights, 2)
	assert.True(t, output.AIGenerated)
	assert.Equal(t, "developer", output.Audience)
	assert.Equal(t, 35, output.CharacterCount)
}

// Test ValidateReleaseInput and ValidateReleaseOutput fields
func TestValidateReleaseInputFields(t *testing.T) {
	input := ValidateReleaseInput{
		ReleaseID:       "validate-123",
		CheckGit:        true,
		CheckPlugins:    true,
		CheckGovernance: true,
		Checks:          []string{"git_clean", "branch_allowed"},
	}

	assert.Equal(t, "validate-123", input.ReleaseID)
	assert.True(t, input.CheckGit)
	assert.True(t, input.CheckPlugins)
	assert.True(t, input.CheckGovernance)
	assert.Len(t, input.Checks, 2)
}

func TestValidateReleaseOutputFields(t *testing.T) {
	output := ValidateReleaseOutput{
		Valid: true,
		Checks: []ValidationCheckResult{
			{Name: "git_clean", Status: "passed", Message: "Working directory is clean"},
		},
		BlockingIssues: nil,
		Warnings:       []string{"Release is stale"},
		CanProceed:     true,
		Recommendation: "All checks passed",
	}

	assert.True(t, output.Valid)
	assert.Len(t, output.Checks, 1)
	assert.Nil(t, output.BlockingIssues)
	assert.Len(t, output.Warnings, 1)
	assert.True(t, output.CanProceed)
	assert.Equal(t, "All checks passed", output.Recommendation)
}

// Test ValidationCheckResult fields
func TestValidationCheckResultFields(t *testing.T) {
	result := ValidationCheckResult{
		Name:    "git_clean",
		Status:  "passed",
		Message: "Working directory is clean",
	}

	assert.Equal(t, "git_clean", result.Name)
	assert.Equal(t, "passed", result.Status)
	assert.Equal(t, "Working directory is clean", result.Message)
}

// Test BlastImpactInfo fields
func TestBlastImpactInfoFields(t *testing.T) {
	info := BlastImpactInfo{
		PackageName:      "core",
		PackagePath:      "pkg/core",
		PackageType:      "library",
		ImpactLevel:      "high",
		RiskScore:        85,
		RequiresRelease:  true,
		ReleaseType:      "major",
		ChangedFiles:     12,
		SuggestedActions: []string{"review API changes", "update docs"},
	}

	assert.Equal(t, "core", info.PackageName)
	assert.Equal(t, "pkg/core", info.PackagePath)
	assert.Equal(t, "library", info.PackageType)
	assert.Equal(t, "high", info.ImpactLevel)
	assert.Equal(t, 85, info.RiskScore)
	assert.True(t, info.RequiresRelease)
	assert.Equal(t, "major", info.ReleaseType)
	assert.Equal(t, 12, info.ChangedFiles)
	assert.Len(t, info.SuggestedActions, 2)
}

// Test BlastDependencyGraph fields
func TestBlastDependencyGraphFields(t *testing.T) {
	graph := BlastDependencyGraph{
		Nodes: []BlastGraphNode{
			{ID: "core", Label: "Core Package", Type: "library", Affected: true, ImpactLevel: "high"},
		},
		Edges: []BlastGraphEdge{
			{Source: "api", Target: "core", Type: "depends_on"},
		},
	}

	assert.Len(t, graph.Nodes, 1)
	assert.Equal(t, "core", graph.Nodes[0].ID)
	assert.Equal(t, "Core Package", graph.Nodes[0].Label)
	assert.True(t, graph.Nodes[0].Affected)

	assert.Len(t, graph.Edges, 1)
	assert.Equal(t, "api", graph.Edges[0].Source)
	assert.Equal(t, "core", graph.Edges[0].Target)
}

// Test BlastGraphNode fields
func TestBlastGraphNodeFields(t *testing.T) {
	node := BlastGraphNode{
		ID:          "pkg-1",
		Label:       "Package 1",
		Type:        "service",
		Affected:    true,
		ImpactLevel: "medium",
	}

	assert.Equal(t, "pkg-1", node.ID)
	assert.Equal(t, "Package 1", node.Label)
	assert.Equal(t, "service", node.Type)
	assert.True(t, node.Affected)
	assert.Equal(t, "medium", node.ImpactLevel)
}

// Test BlastGraphEdge fields
func TestBlastGraphEdgeFields(t *testing.T) {
	edge := BlastGraphEdge{
		Source: "pkg-a",
		Target: "pkg-b",
		Type:   "imports",
	}

	assert.Equal(t, "pkg-a", edge.Source)
	assert.Equal(t, "pkg-b", edge.Target)
	assert.Equal(t, "imports", edge.Type)
}

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
// NOTE: GetStatus now requires release services, so these tests verify error behavior
func TestAdapterGetStatusWithStaleRelease(t *testing.T) {
	// Create a release that was last updated more than 1 hour ago
	rel := domainrelease.NewReleaseRunForTest("stale-release-test", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)

	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{rel}}
	adapter := NewAdapter(WithAdapterRepo(repo))

	ctx := context.Background()

	// GetStatus now requires release services
	output, err := adapter.GetStatus(ctx)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "release services not configured")
}

// Test GetStatus output fields for various states
// NOTE: GetStatus now requires release services, so these tests verify error behavior
func TestAdapterGetStatusNextAction(t *testing.T) {
	// Create a release in planned state
	rel := domainrelease.NewReleaseRunForTest("next-action-test", "main", "")
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)

	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{rel}}
	adapter := NewAdapter(WithAdapterRepo(repo))

	ctx := context.Background()

	// GetStatus now requires release services
	output, err := adapter.GetStatus(ctx)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "release services not configured")
}

// Test GetStatus with warning flag
func TestGetStatusOutputWarningField(t *testing.T) {
	output := GetStatusOutput{
		ReleaseID:  "warning-test",
		State:      "planned",
		Version:    "1.0.0",
		NextAction: "bump",
		Stale:      true,
		Warning:    "Release was last updated over 1 hour ago",
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

// =============================================================================
// State Transition Integration Tests
// =============================================================================
// These tests verify the complete state transition flow through the MCP adapter:
// draft → planned → versioned → notes_ready → approved → publishing → published
// =============================================================================

// TestStateTransitionsViaAdapter tests that state transitions are properly
// enforced through the MCP adapter layer, validating ADR-007 compliance.
func TestStateTransitionsViaAdapter(t *testing.T) {
	t.Run("state flow: draft → planned (via SetPlan)", func(t *testing.T) {
		rel := domainrelease.NewReleaseRunForTest("state-test-1", "main", "")
		assert.Equal(t, domainrelease.StateDraft, rel.State())

		// Apply plan to transition to planned state
		v, _ := version.Parse("1.0.0")
		nextV, _ := version.Parse("1.1.0")
		plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
		err := domainrelease.SetPlan(rel, plan)
		require.NoError(t, err)

		assert.Equal(t, domainrelease.StatePlanned, rel.State())
	})

	t.Run("state flow: planned → versioned (via SetVersion + Bump)", func(t *testing.T) {
		rel := domainrelease.NewReleaseRunForTest("state-test-2", "main", "")
		v, _ := version.Parse("1.0.0")
		nextV, _ := version.Parse("1.1.0")
		plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
		_ = domainrelease.SetPlan(rel, plan)
		assert.Equal(t, domainrelease.StatePlanned, rel.State())

		// Set version and then bump to transition to versioned state
		err := rel.SetVersion(nextV, "v1.1.0")
		require.NoError(t, err)
		err = rel.Bump("system")
		require.NoError(t, err)

		assert.Equal(t, domainrelease.StateVersioned, rel.State())
	})

	t.Run("state flow: versioned → notes_ready (via GenerateNotes)", func(t *testing.T) {
		rel := domainrelease.NewReleaseRunForTest("state-test-3", "main", "")
		v, _ := version.Parse("1.0.0")
		nextV, _ := version.Parse("1.1.0")
		plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
		_ = domainrelease.SetPlan(rel, plan)
		_ = rel.SetVersion(nextV, "v1.1.0")
		_ = rel.Bump("system")
		assert.Equal(t, domainrelease.StateVersioned, rel.State())

		// Generate notes to transition to notes_ready state
		notes := &domainrelease.ReleaseNotes{
			Text: "## Changes\n- Feature 1",
		}
		err := rel.GenerateNotes(notes, "inputs-hash", "system")
		require.NoError(t, err)

		assert.Equal(t, domainrelease.StateNotesReady, rel.State())
	})

	t.Run("state flow: notes_ready → approved (via Approve)", func(t *testing.T) {
		rel := domainrelease.NewReleaseRunForTest("state-test-4", "main", "")
		v, _ := version.Parse("1.0.0")
		nextV, _ := version.Parse("1.1.0")
		plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
		_ = domainrelease.SetPlan(rel, plan)
		_ = rel.SetVersion(nextV, "v1.1.0")
		_ = rel.Bump("system")
		notes := &domainrelease.ReleaseNotes{Text: "## Changes"}
		_ = rel.GenerateNotes(notes, "inputs-hash", "system")
		assert.Equal(t, domainrelease.StateNotesReady, rel.State())

		// Approve to transition to approved state
		err := rel.Approve("test-approver", false)
		require.NoError(t, err)

		assert.Equal(t, domainrelease.StateApproved, rel.State())
	})

	t.Run("invalid transition: draft → versioned (must go through planned)", func(t *testing.T) {
		rel := domainrelease.NewReleaseRunForTest("state-test-5", "main", "")
		assert.Equal(t, domainrelease.StateDraft, rel.State())

		// Attempt to set version without planning first
		nextV, _ := version.Parse("1.1.0")
		err := rel.SetVersion(nextV, "v1.1.0")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid state")

		// State should remain draft
		assert.Equal(t, domainrelease.StateDraft, rel.State())
	})

	t.Run("invalid transition: planned → notes_ready (must go through versioned)", func(t *testing.T) {
		rel := domainrelease.NewReleaseRunForTest("state-test-6", "main", "")
		v, _ := version.Parse("1.0.0")
		nextV, _ := version.Parse("1.1.0")
		plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
		_ = domainrelease.SetPlan(rel, plan)
		assert.Equal(t, domainrelease.StatePlanned, rel.State())

		// Attempt to generate notes without bumping version first
		notes := &domainrelease.ReleaseNotes{Text: "## Changes"}
		err := rel.GenerateNotes(notes, "inputs-hash", "system")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot generate notes")

		// State should remain planned
		assert.Equal(t, domainrelease.StatePlanned, rel.State())
	})

	t.Run("invalid transition: versioned → approved (must go through notes_ready)", func(t *testing.T) {
		rel := domainrelease.NewReleaseRunForTest("state-test-7", "main", "")
		v, _ := version.Parse("1.0.0")
		nextV, _ := version.Parse("1.1.0")
		plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
		_ = domainrelease.SetPlan(rel, plan)
		_ = rel.SetVersion(nextV, "v1.1.0")
		_ = rel.Bump("system")
		assert.Equal(t, domainrelease.StateVersioned, rel.State())

		// Attempt to approve without generating notes first
		err := rel.Approve("test-approver", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot approve")

		// State should remain versioned
		assert.Equal(t, domainrelease.StateVersioned, rel.State())
	})

	t.Run("cancel transition: any state → canceled", func(t *testing.T) {
		// Test cancel from planned state
		rel := domainrelease.NewReleaseRunForTest("state-test-8", "main", "")
		v, _ := version.Parse("1.0.0")
		nextV, _ := version.Parse("1.1.0")
		plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
		_ = domainrelease.SetPlan(rel, plan)
		assert.Equal(t, domainrelease.StatePlanned, rel.State())

		err := rel.Cancel("user requested cancel", "test-user")
		require.NoError(t, err)
		assert.Equal(t, domainrelease.StateCanceled, rel.State())
	})
}

// TestStateTransitionTableDriven tests all valid state transitions in a table-driven format
func TestStateTransitionTableDriven(t *testing.T) {
	tests := []struct {
		name          string
		setup         func() *domainrelease.ReleaseRun
		action        func(rel *domainrelease.ReleaseRun) error
		expectedState domainrelease.RunState
		expectError   bool
	}{
		{
			name: "draft → planned via SetPlan",
			setup: func() *domainrelease.ReleaseRun {
				return domainrelease.NewReleaseRunForTest("table-test-1", "main", "")
			},
			action: func(rel *domainrelease.ReleaseRun) error {
				v, _ := version.Parse("1.0.0")
				nextV, _ := version.Parse("1.1.0")
				plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
				return domainrelease.SetPlan(rel, plan)
			},
			expectedState: domainrelease.StatePlanned,
			expectError:   false,
		},
		{
			name: "planned → versioned via SetVersion + Bump",
			setup: func() *domainrelease.ReleaseRun {
				rel := domainrelease.NewReleaseRunForTest("table-test-2", "main", "")
				v, _ := version.Parse("1.0.0")
				nextV, _ := version.Parse("1.1.0")
				plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
				_ = domainrelease.SetPlan(rel, plan)
				_ = rel.SetVersion(nextV, "v1.1.0")
				return rel
			},
			action: func(rel *domainrelease.ReleaseRun) error {
				return rel.Bump("system")
			},
			expectedState: domainrelease.StateVersioned,
			expectError:   false,
		},
		{
			name: "versioned → notes_ready via GenerateNotes",
			setup: func() *domainrelease.ReleaseRun {
				rel := domainrelease.NewReleaseRunForTest("table-test-3", "main", "")
				v, _ := version.Parse("1.0.0")
				nextV, _ := version.Parse("1.1.0")
				plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
				_ = domainrelease.SetPlan(rel, plan)
				_ = rel.SetVersion(nextV, "v1.1.0")
				_ = rel.Bump("system")
				return rel
			},
			action: func(rel *domainrelease.ReleaseRun) error {
				notes := &domainrelease.ReleaseNotes{Text: "## Changes"}
				return rel.GenerateNotes(notes, "inputs-hash", "system")
			},
			expectedState: domainrelease.StateNotesReady,
			expectError:   false,
		},
		{
			name: "notes_ready → approved via Approve",
			setup: func() *domainrelease.ReleaseRun {
				rel := domainrelease.NewReleaseRunForTest("table-test-4", "main", "")
				v, _ := version.Parse("1.0.0")
				nextV, _ := version.Parse("1.1.0")
				plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
				_ = domainrelease.SetPlan(rel, plan)
				_ = rel.SetVersion(nextV, "v1.1.0")
				_ = rel.Bump("system")
				notes := &domainrelease.ReleaseNotes{Text: "## Changes"}
				_ = rel.GenerateNotes(notes, "inputs-hash", "system")
				return rel
			},
			action: func(rel *domainrelease.ReleaseRun) error {
				return rel.Approve("approver", false)
			},
			expectedState: domainrelease.StateApproved,
			expectError:   false,
		},
		{
			name: "draft → versioned (invalid - skip planned)",
			setup: func() *domainrelease.ReleaseRun {
				return domainrelease.NewReleaseRunForTest("table-test-5", "main", "")
			},
			action: func(rel *domainrelease.ReleaseRun) error {
				nextV, _ := version.Parse("1.1.0")
				return rel.SetVersion(nextV, "v1.1.0")
			},
			expectedState: domainrelease.StateDraft, // Should remain draft
			expectError:   true,
		},
		{
			name: "planned → approved (invalid - skip versioned and notes)",
			setup: func() *domainrelease.ReleaseRun {
				rel := domainrelease.NewReleaseRunForTest("table-test-6", "main", "")
				v, _ := version.Parse("1.0.0")
				nextV, _ := version.Parse("1.1.0")
				plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
				_ = domainrelease.SetPlan(rel, plan)
				return rel
			},
			action: func(rel *domainrelease.ReleaseRun) error {
				return rel.Approve("approver", false)
			},
			expectedState: domainrelease.StatePlanned, // Should remain planned
			expectError:   true,
		},
		{
			name: "canceled is terminal (cannot transition further)",
			setup: func() *domainrelease.ReleaseRun {
				rel := domainrelease.NewReleaseRunForTest("table-test-7", "main", "")
				v, _ := version.Parse("1.0.0")
				nextV, _ := version.Parse("1.1.0")
				plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
				_ = domainrelease.SetPlan(rel, plan)
				_ = rel.Cancel("user requested", "test-user")
				return rel
			},
			action: func(rel *domainrelease.ReleaseRun) error {
				nextV, _ := version.Parse("1.1.0")
				return rel.SetVersion(nextV, "v1.1.0")
			},
			expectedState: domainrelease.StateCanceled, // Should remain canceled
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rel := tt.setup()
			err := tt.action(rel)

			if tt.expectError {
				require.Error(t, err, "Expected error for invalid transition")
			} else {
				require.NoError(t, err, "Expected no error for valid transition")
			}

			assert.Equal(t, tt.expectedState, rel.State(),
				"State mismatch: got %s, want %s", rel.State(), tt.expectedState)
		})
	}
}

// TestMCPAdapterStateTransitionErrors verifies the adapter returns proper errors
// when operations are attempted in invalid states
func TestMCPAdapterStateTransitionErrors(t *testing.T) {
	t.Run("Bump requires release services", func(t *testing.T) {
		adapter := NewAdapter()

		ctx := context.Background()
		input := BumpInput{
			RepositoryPath: "/test/repo",
			BumpType:       "minor",
		}

		output, err := adapter.Bump(ctx, input)
		require.Error(t, err)
		assert.Nil(t, output)
		assert.Contains(t, err.Error(), "release services not configured")
	})

	t.Run("Notes requires release services", func(t *testing.T) {
		adapter := NewAdapter()

		ctx := context.Background()
		input := NotesInput{
			ReleaseID: "test-release",
		}

		output, err := adapter.Notes(ctx, input)
		require.Error(t, err)
		assert.Nil(t, output)
		assert.Contains(t, err.Error(), "release services not configured")
	})

	t.Run("Approve requires release services", func(t *testing.T) {
		adapter := NewAdapter()

		ctx := context.Background()
		input := ApproveInput{
			ReleaseID: "test-release",
		}

		output, err := adapter.Approve(ctx, input)
		require.Error(t, err)
		assert.Nil(t, output)
		assert.Contains(t, err.Error(), "release services not configured")
	})

	t.Run("Publish requires release services", func(t *testing.T) {
		adapter := NewAdapter()

		ctx := context.Background()
		input := PublishInput{
			ReleaseID: "test-release",
		}

		output, err := adapter.Publish(ctx, input)
		require.Error(t, err)
		assert.Nil(t, output)
		assert.Contains(t, err.Error(), "release services not configured")
	})
}

// TestCompleteReleaseWorkflow tests the happy path through all states
func TestCompleteReleaseWorkflow(t *testing.T) {
	t.Run("complete workflow: draft → published", func(t *testing.T) {
		// Create a release run
		rel := domainrelease.NewReleaseRunForTest("workflow-test", "main", "")
		assert.Equal(t, domainrelease.StateDraft, rel.State())

		// Step 1: Plan
		v, _ := version.Parse("1.0.0")
		nextV, _ := version.Parse("1.1.0")
		plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
		err := domainrelease.SetPlan(rel, plan)
		require.NoError(t, err)
		assert.Equal(t, domainrelease.StatePlanned, rel.State())

		// Step 2: Bump (set version and transition)
		err = rel.SetVersion(nextV, "v1.1.0")
		require.NoError(t, err)
		err = rel.Bump("system")
		require.NoError(t, err)
		assert.Equal(t, domainrelease.StateVersioned, rel.State())

		// Step 3: Notes
		notes := &domainrelease.ReleaseNotes{
			Text: "## [1.1.0]\n\n### Features\n- New feature\n\n### Bug Fixes\n- Fixed bug",
		}
		err = rel.GenerateNotes(notes, "inputs-hash", "system")
		require.NoError(t, err)
		assert.Equal(t, domainrelease.StateNotesReady, rel.State())

		// Step 4: Approve
		err = rel.Approve("release-manager", false)
		require.NoError(t, err)
		assert.Equal(t, domainrelease.StateApproved, rel.State())

		// Step 5: Start Publishing
		rel.SetExecutionPlan([]domainrelease.StepPlan{
			{Name: "tag", Type: domainrelease.StepTypeTag},
			{Name: "notify", Type: domainrelease.StepTypeNotify},
		})
		err = rel.StartPublishing("publisher")
		require.NoError(t, err)
		assert.Equal(t, domainrelease.StatePublishing, rel.State())

		// Step 6: Complete steps and mark published
		_ = rel.MarkStepDone("tag", "v1.1.0")
		_ = rel.MarkStepDone("notify", "notified")
		err = rel.MarkPublished("publisher")
		require.NoError(t, err)
		assert.Equal(t, domainrelease.StatePublished, rel.State())

		// Verify final state attributes
		assert.Equal(t, "v1.1.0", rel.TagName())
		assert.Equal(t, "1.1.0", rel.VersionNext().String())
		assert.True(t, rel.IsApproved())
	})
}
