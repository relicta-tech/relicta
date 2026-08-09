package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	domainrelease "github.com/relicta-tech/relicta/v4/internal/domain/release"
	releaseapp "github.com/relicta-tech/relicta/v4/internal/domain/release/app"
	releasedomain "github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// =============================================================================
// Mock port implementations for testing server handlers with a real adapter
// =============================================================================

// mockPortsRepo implements ports.ReleaseRunRepository for use case testing.
type mockPortsRepo struct {
	runs map[releasedomain.RunID]*releasedomain.ReleaseRun
	// latestByRepo maps repoRoot to the latest run
	latestByRepo map[string]releasedomain.RunID
	saveErr      error
	deleteErr    error
}

func newMockPortsRepo() *mockPortsRepo {
	return &mockPortsRepo{
		runs:         make(map[releasedomain.RunID]*releasedomain.ReleaseRun),
		latestByRepo: make(map[string]releasedomain.RunID),
	}
}

func (m *mockPortsRepo) Load(_ context.Context, runID releasedomain.RunID) (*releasedomain.ReleaseRun, error) {
	if r, ok := m.runs[runID]; ok {
		return r, nil
	}
	return nil, domainrelease.ErrRunNotFound
}

func (m *mockPortsRepo) LoadBatch(_ context.Context, _ string, runIDs []releasedomain.RunID) (map[releasedomain.RunID]*releasedomain.ReleaseRun, error) {
	result := make(map[releasedomain.RunID]*releasedomain.ReleaseRun)
	for _, id := range runIDs {
		if r, ok := m.runs[id]; ok {
			result[id] = r
		}
	}
	return result, nil
}

func (m *mockPortsRepo) LoadLatest(_ context.Context, repoRoot string) (*releasedomain.ReleaseRun, error) {
	if id, ok := m.latestByRepo[repoRoot]; ok {
		if r, ok2 := m.runs[id]; ok2 {
			return r, nil
		}
	}
	// Return any run if only one exists (test convenience)
	for _, r := range m.runs {
		return r, nil
	}
	return nil, domainrelease.ErrRunNotFound
}

func (m *mockPortsRepo) List(_ context.Context, _ string) ([]releasedomain.RunID, error) {
	var ids []releasedomain.RunID
	for id := range m.runs {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *mockPortsRepo) Save(_ context.Context, run *releasedomain.ReleaseRun) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.runs[run.ID()] = run
	return nil
}

func (m *mockPortsRepo) SetLatest(_ context.Context, repoRoot string, runID releasedomain.RunID) error {
	m.latestByRepo[repoRoot] = runID
	return nil
}

func (m *mockPortsRepo) Delete(_ context.Context, runID releasedomain.RunID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.runs, runID)
	return nil
}

func (m *mockPortsRepo) FindByState(_ context.Context, _ string, state releasedomain.RunState) ([]*releasedomain.ReleaseRun, error) {
	var result []*releasedomain.ReleaseRun
	for _, r := range m.runs {
		if r.State() == state {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *mockPortsRepo) FindActive(_ context.Context, _ string) ([]*releasedomain.ReleaseRun, error) {
	var result []*releasedomain.ReleaseRun
	for _, r := range m.runs {
		if !r.State().IsFinal() {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *mockPortsRepo) FindByPlanHash(_ context.Context, _ string, _ string) (*releasedomain.ReleaseRun, error) {
	return nil, nil
}

// addRun adds a release run and sets it as latest.
func (m *mockPortsRepo) addRun(run *releasedomain.ReleaseRun, repoRoot string) {
	m.runs[run.ID()] = run
	m.latestByRepo[repoRoot] = run.ID()
}

// mockRepoInspector implements ports.RepoInspector.
type mockRepoInspector struct {
	headSHA   releasedomain.CommitSHA
	isClean   bool
	remoteURL string
	branch    string
}

func (m *mockRepoInspector) HeadSHA(_ context.Context) (releasedomain.CommitSHA, error) {
	return m.headSHA, nil
}

func (m *mockRepoInspector) IsClean(_ context.Context) (bool, error) {
	return m.isClean, nil
}

func (m *mockRepoInspector) ResolveCommits(_ context.Context, _ string, _ releasedomain.CommitSHA) ([]releasedomain.CommitSHA, error) {
	return nil, nil
}

func (m *mockRepoInspector) GetRemoteURL(_ context.Context) (string, error) {
	return m.remoteURL, nil
}

func (m *mockRepoInspector) GetCurrentBranch(_ context.Context) (string, error) {
	return m.branch, nil
}

func (m *mockRepoInspector) GetLatestVersionTag(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *mockRepoInspector) TagExists(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (m *mockRepoInspector) ReleaseExists(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// Compile-time interface checks.
var (
	_ ports.ReleaseRunRepository = (*mockPortsRepo)(nil)
	_ ports.RepoInspector        = (*mockRepoInspector)(nil)
)

// =============================================================================
// Helper: create release runs at various states
// =============================================================================

func createPlannedRelease(id string, repoRoot string) *releasedomain.ReleaseRun {
	rel := domainrelease.NewReleaseRunForTest(domainrelease.RunID(id), "main", repoRoot)
	v, _ := version.Parse("1.0.0")
	nextV, _ := version.Parse("1.1.0")
	plan := domainrelease.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = domainrelease.SetPlan(rel, plan)
	return rel
}

func createCanceledRelease(id string, repoRoot string) *releasedomain.ReleaseRun {
	rel := createPlannedRelease(id, repoRoot)
	_ = rel.Cancel("test cancel", "tester")
	return rel
}

// =============================================================================
// Test setup helper
// =============================================================================

type testSetup struct {
	server    *Server
	adapter   *Adapter
	repo      *mockPortsRepo
	inspector *mockRepoInspector
}

func newTestSetup(repoRoot string) *testSetup {
	repo := newMockPortsRepo()
	inspector := &mockRepoInspector{
		headSHA: releasedomain.CommitSHA("abc123"),
		isClean: true,
		branch:  "main",
	}

	getStatus := releaseapp.NewGetStatusUseCase(repo, inspector)

	services := &domainrelease.Services{
		GetStatus: getStatus,
	}

	adapter := NewAdapter(
		WithReleaseServices(services),
		WithAdapterRepo(&mockPortsRepoLegacyAdapter{inner: repo}),
		WithRepoRoot(repoRoot),
	)

	srv, _ := NewServer("test-version",
		WithAdapter(adapter),
	)

	return &testSetup{
		server:    srv,
		adapter:   adapter,
		repo:      repo,
		inspector: inspector,
	}
}

// mockPortsRepoLegacyAdapter bridges mockPortsRepo to the legacy domainrelease.Repository interface.
type mockPortsRepoLegacyAdapter struct {
	inner *mockPortsRepo
}

func (m *mockPortsRepoLegacyAdapter) Save(ctx context.Context, rel *domainrelease.ReleaseRun) error {
	return m.inner.Save(ctx, rel)
}

func (m *mockPortsRepoLegacyAdapter) FindByID(ctx context.Context, id domainrelease.RunID) (*domainrelease.ReleaseRun, error) {
	return m.inner.Load(ctx, id)
}

func (m *mockPortsRepoLegacyAdapter) FindLatest(ctx context.Context, repoPath string) (*domainrelease.ReleaseRun, error) {
	return m.inner.LoadLatest(ctx, repoPath)
}

func (m *mockPortsRepoLegacyAdapter) FindActive(ctx context.Context) ([]*domainrelease.ReleaseRun, error) {
	return m.inner.FindActive(ctx, "")
}

func (m *mockPortsRepoLegacyAdapter) FindByState(ctx context.Context, state domainrelease.RunState) ([]*domainrelease.ReleaseRun, error) {
	return m.inner.FindByState(ctx, "", state)
}

func (m *mockPortsRepoLegacyAdapter) FindBySpecification(_ context.Context, spec domainrelease.Specification) ([]*domainrelease.ReleaseRun, error) {
	var result []*domainrelease.ReleaseRun
	for _, r := range m.inner.runs {
		if spec.IsSatisfiedBy(r) {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *mockPortsRepoLegacyAdapter) Delete(ctx context.Context, id domainrelease.RunID) error {
	return m.inner.Delete(ctx, id)
}

func (m *mockPortsRepoLegacyAdapter) List(ctx context.Context, repoPath string) ([]domainrelease.RunID, error) {
	return m.inner.List(ctx, repoPath)
}

var _ domainrelease.Repository = (*mockPortsRepoLegacyAdapter)(nil)

// =============================================================================
// JSON helper
// =============================================================================

// parseJSONMap decodes a tool handler result into a map. Handlers may return a
// raw JSON string or a typed struct value; both are handled.
func parseJSONMap(t *testing.T, result any) map[string]any {
	t.Helper()
	var data []byte
	if s, ok := result.(string); ok {
		data = []byte(s)
	} else {
		b, err := json.Marshal(result)
		require.NoError(t, err, "failed to marshal handler result")
		data = b
	}
	var out map[string]any
	err := json.Unmarshal(data, &out)
	require.NoError(t, err, "failed to parse JSON: %s", string(data))
	return out
}

// =============================================================================
// Server handler tests WITH adapter (adapter path coverage)
// =============================================================================

func TestHandleStatusWithAdapterPath(t *testing.T) {
	repoRoot := "/test/repo"
	ts := newTestSetup(repoRoot)
	ctx := context.Background()

	t.Run("no active release returns no_active_release", func(t *testing.T) {
		result, err := ts.server.handleStatus(ctx, StatusInput{})
		require.NoError(t, err)
		parsed := parseJSONMap(t, result)
		assert.Equal(t, "no_active_release", parsed["status"])
	})

	t.Run("with active planned release returns status details", func(t *testing.T) {
		rel := createPlannedRelease("status-test-1", repoRoot)
		ts.repo.addRun(rel, repoRoot)

		result, err := ts.server.handleStatus(ctx, StatusInput{})
		require.NoError(t, err)
		parsed := parseJSONMap(t, result)
		assert.Equal(t, "planned", parsed["state"])
		assert.NotEmpty(t, parsed["release_id"])
		assert.Equal(t, "bump", parsed["next_action"])
		assert.NotEmpty(t, parsed["created"])
		assert.NotEmpty(t, parsed["updated"])
	})
}

func TestHandleStatusWithAdapterVersionField(t *testing.T) {
	repoRoot := "/test/repo"
	ts := newTestSetup(repoRoot)

	rel := createPlannedRelease("version-test", repoRoot)
	nextV, _ := version.Parse("1.1.0")
	_ = rel.SetVersion(nextV, "v1.1.0")
	_ = rel.Bump("system")
	ts.repo.addRun(rel, repoRoot)

	ctx := context.Background()
	result, err := ts.server.handleStatus(ctx, StatusInput{})
	require.NoError(t, err)
	parsed := parseJSONMap(t, result)
	assert.Equal(t, "1.1.0", parsed["version"])
	assert.Equal(t, "versioned", parsed["state"])
}

func TestHandleStatusWithAdapterNotesReadyState(t *testing.T) {
	repoRoot := "/test/repo"
	ts := newTestSetup(repoRoot)

	rel := createPlannedRelease("notes-ready-test", repoRoot)
	nextV, _ := version.Parse("1.1.0")
	_ = rel.SetVersion(nextV, "v1.1.0")
	_ = rel.Bump("system")
	notes := &domainrelease.ReleaseNotes{Text: "## Changes"}
	_ = rel.GenerateNotes(notes, "hash", "system")
	ts.repo.addRun(rel, repoRoot)

	ctx := context.Background()
	result, err := ts.server.handleStatus(ctx, StatusInput{})
	require.NoError(t, err)
	parsed := parseJSONMap(t, result)
	assert.Equal(t, "notes_ready", parsed["state"])
	assert.Equal(t, true, parsed["can_approve"])
	assert.Equal(t, "approve", parsed["next_action"])
}

func TestHandleStatusWithAdapterHeadDrift(t *testing.T) {
	repoRoot := "/test/repo"
	ts := newTestSetup(repoRoot)

	rel := createPlannedRelease("drift-test", repoRoot)
	ts.repo.addRun(rel, repoRoot)
	// Set inspector HEAD to different SHA to trigger drift warning
	ts.inspector.headSHA = "different-sha-999"

	ctx := context.Background()
	result, err := ts.server.handleStatus(ctx, StatusInput{})
	require.NoError(t, err)
	parsed := parseJSONMap(t, result)
	assert.Equal(t, "planned", parsed["state"])
}

func TestHandlePlanWithAdapterPath_NotConfigured(t *testing.T) {
	repoRoot := "/test/repo"
	ts := newTestSetup(repoRoot)

	ctx := context.Background()
	result, err := ts.server.handlePlan(ctx, PlanToolInput{})
	require.NoError(t, err)
	parsed := parseJSONMap(t, result)
	assert.Equal(t, "not_configured", parsed["status"])
}

func TestHandleNotesWithAdapterPath_NoActiveRelease(t *testing.T) {
	repoRoot := "/test/repo"
	ts := newTestSetup(repoRoot)

	ctx := context.Background()
	_, err := ts.server.handleNotes(ctx, NotesToolInput{})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "no active release")
}

func TestHandleApproveWithAdapterPath_NoActiveRelease(t *testing.T) {
	repoRoot := "/test/repo"
	ts := newTestSetup(repoRoot)

	ctx := context.Background()
	_, err := ts.server.handleApprove(ctx, ApproveToolInput{})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "no active release")
}

func TestHandlePublishWithAdapterPath_NoActiveRelease(t *testing.T) {
	repoRoot := "/test/repo"
	ts := newTestSetup(repoRoot)

	ctx := context.Background()
	_, err := ts.server.handlePublish(ctx, PublishToolInput{})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "no active release")
}

func TestHandleCancelWithAdapterPath(t *testing.T) {
	repoRoot := "/test/repo"

	t.Run("no active release returns error", func(t *testing.T) {
		ts := newTestSetup(repoRoot)
		ctx := context.Background()

		_, err := ts.server.handleCancel(ctx, CancelToolInput{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no active release to cancel")
	})

	t.Run("cancel planned release succeeds", func(t *testing.T) {
		ts := newTestSetup(repoRoot)
		rel := createPlannedRelease("cancel-ok", repoRoot)
		ts.repo.addRun(rel, repoRoot)

		ctx := context.Background()
		result, err := ts.server.handleCancel(ctx, CancelToolInput{Reason: "test reason"})
		require.NoError(t, err)
		parsed := parseJSONMap(t, result)
		assert.Equal(t, "planned", parsed["previous_state"])
		assert.Equal(t, "canceled", parsed["new_state"])
		assert.Equal(t, "test reason", parsed["reason"])
		assert.Contains(t, parsed["message"], "canceled successfully")
	})

	t.Run("cancel with default reason", func(t *testing.T) {
		ts := newTestSetup(repoRoot)
		rel := createPlannedRelease("cancel-default", repoRoot)
		ts.repo.addRun(rel, repoRoot)

		ctx := context.Background()
		result, err := ts.server.handleCancel(ctx, CancelToolInput{})
		require.NoError(t, err)
		parsed := parseJSONMap(t, result)
		assert.Equal(t, "canceled via MCP", parsed["reason"])
	})

	t.Run("cancel already canceled release returns terminal message", func(t *testing.T) {
		ts := newTestSetup(repoRoot)
		rel := createCanceledRelease("cancel-terminal", repoRoot)
		ts.repo.addRun(rel, repoRoot)

		ctx := context.Background()
		result, err := ts.server.handleCancel(ctx, CancelToolInput{})
		require.NoError(t, err)
		parsed := parseJSONMap(t, result)
		assert.Contains(t, parsed["message"], "terminal state")
	})

	t.Run("cancel published release returns error", func(t *testing.T) {
		ts := newTestSetup(repoRoot)
		rel := createPlannedRelease("cancel-published", repoRoot)
		nextV, _ := version.Parse("1.1.0")
		_ = rel.SetVersion(nextV, "v1.1.0")
		_ = rel.Bump("system")
		notes := &domainrelease.ReleaseNotes{Text: "## Changes"}
		_ = rel.GenerateNotes(notes, "hash", "system")
		_ = rel.Approve("approver", false)
		rel.SetExecutionPlan([]releasedomain.StepPlan{
			{Name: "tag", Type: releasedomain.StepTypeTag},
		})
		_ = rel.StartPublishing("publisher")
		_ = rel.MarkStepDone("tag", "v1.1.0")
		_ = rel.MarkPublished("publisher")
		ts.repo.addRun(rel, repoRoot)

		ctx := context.Background()
		_, err := ts.server.handleCancel(ctx, CancelToolInput{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot cancel a published release")
	})
}

func TestHandleResetWithAdapterPath(t *testing.T) {
	repoRoot := "/test/repo"

	t.Run("no active release returns message", func(t *testing.T) {
		ts := newTestSetup(repoRoot)
		ctx := context.Background()

		result, err := ts.server.handleReset(ctx, ResetToolInput{})
		require.NoError(t, err)
		parsed := parseJSONMap(t, result)
		assert.Contains(t, parsed["message"], "nothing to reset")
	})

	t.Run("reset canceled release succeeds with force", func(t *testing.T) {
		ts := newTestSetup(repoRoot)
		rel := createCanceledRelease("reset-canceled", repoRoot)
		ts.repo.addRun(rel, repoRoot)

		ctx := context.Background()
		result, err := ts.server.handleReset(ctx, ResetToolInput{Force: true})
		require.NoError(t, err)
		parsed := parseJSONMap(t, result)
		assert.Equal(t, true, parsed["deleted"])
		assert.Contains(t, parsed["message"], "reset successfully")
	})

	t.Run("reset in-progress release without force suggests cancel", func(t *testing.T) {
		ts := newTestSetup(repoRoot)
		rel := createPlannedRelease("reset-inprogress", repoRoot)
		ts.repo.addRun(rel, repoRoot)

		ctx := context.Background()
		result, err := ts.server.handleReset(ctx, ResetToolInput{Force: false})
		require.NoError(t, err)
		parsed := parseJSONMap(t, result)
		assert.Contains(t, parsed["message"], "cancel first")
	})

	t.Run("reset in-progress release with force deletes it", func(t *testing.T) {
		ts := newTestSetup(repoRoot)
		rel := createPlannedRelease("reset-force", repoRoot)
		ts.repo.addRun(rel, repoRoot)

		ctx := context.Background()
		result, err := ts.server.handleReset(ctx, ResetToolInput{Force: true})
		require.NoError(t, err)
		parsed := parseJSONMap(t, result)
		assert.Equal(t, true, parsed["deleted"])
	})

	t.Run("reset published release returns error", func(t *testing.T) {
		ts := newTestSetup(repoRoot)
		rel := createPlannedRelease("reset-published", repoRoot)
		nextV, _ := version.Parse("1.1.0")
		_ = rel.SetVersion(nextV, "v1.1.0")
		_ = rel.Bump("system")
		notes := &domainrelease.ReleaseNotes{Text: "## Changes"}
		_ = rel.GenerateNotes(notes, "hash", "system")
		_ = rel.Approve("approver", false)
		rel.SetExecutionPlan([]releasedomain.StepPlan{
			{Name: "tag", Type: releasedomain.StepTypeTag},
		})
		_ = rel.StartPublishing("publisher")
		_ = rel.MarkStepDone("tag", "v1.1.0")
		_ = rel.MarkPublished("publisher")
		ts.repo.addRun(rel, repoRoot)

		ctx := context.Background()
		_, err := ts.server.handleReset(ctx, ResetToolInput{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "published releases cannot be reset")
	})
}

func TestHandleBlastRadiusWithAdapterPath_NotConfigured(t *testing.T) {
	repoRoot := "/test/repo"
	ts := newTestSetup(repoRoot)

	ctx := context.Background()
	result, err := ts.server.handleBlastRadius(ctx, BlastRadiusToolInput{})
	require.NoError(t, err)
	parsed := parseJSONMap(t, result)
	assert.Equal(t, "not_configured", parsed["status"])
}

func TestHandleInferVersionWithAdapterPath_NotConfigured(t *testing.T) {
	repoRoot := "/test/repo"
	ts := newTestSetup(repoRoot)

	ctx := context.Background()
	result, err := ts.server.handleInferVersion(ctx, InferVersionToolInput{})
	require.NoError(t, err)
	parsed := parseJSONMap(t, result)
	assert.Equal(t, "not_configured", parsed["status"])
}

func TestHandleSummarizeDiffWithAdapterPath_NotConfigured(t *testing.T) {
	repoRoot := "/test/repo"
	ts := newTestSetup(repoRoot)

	ctx := context.Background()
	result, err := ts.server.handleSummarizeDiff(ctx, SummarizeDiffToolInput{})
	require.NoError(t, err)
	parsed := parseJSONMap(t, result)
	assert.Equal(t, "not_configured", parsed["status"])
}

func TestHandleValidateReleaseWithAdapterPath(t *testing.T) {
	repoRoot := "/test/repo"

	t.Run("basic validation with git and plugin checks", func(t *testing.T) {
		ts := newTestSetup(repoRoot)
		ctx := context.Background()

		result, err := ts.server.handleValidateRelease(ctx, ValidateReleaseToolInput{
			CheckGit:     true,
			CheckPlugins: true,
		})
		require.NoError(t, err)
		parsed := parseJSONMap(t, result)
		assert.Equal(t, true, parsed["valid"])
		assert.Equal(t, true, parsed["can_proceed"])
		assert.Contains(t, parsed["recommendation"], "All checks passed")
		// Should have checks array
		checks, ok := parsed["checks"].([]any)
		require.True(t, ok)
		assert.GreaterOrEqual(t, len(checks), 2)
	})

	t.Run("validation picks up active release ID automatically", func(t *testing.T) {
		ts := newTestSetup(repoRoot)
		rel := createPlannedRelease("validate-active", repoRoot)
		ts.repo.addRun(rel, repoRoot)

		ctx := context.Background()
		result, err := ts.server.handleValidateRelease(ctx, ValidateReleaseToolInput{})
		require.NoError(t, err)
		parsed := parseJSONMap(t, result)
		assert.Equal(t, true, parsed["valid"])
		// Should have the release_exists check
		if checks, ok := parsed["checks"].([]any); ok {
			found := false
			for _, c := range checks {
				cm := c.(map[string]any)
				if cm["name"] == "release_exists" {
					found = true
					assert.Equal(t, "passed", cm["status"])
				}
			}
			assert.True(t, found, "Expected release_exists check")
		}
	})

	t.Run("validation with nonexistent release ID", func(t *testing.T) {
		ts := newTestSetup(repoRoot)
		ctx := context.Background()

		result, err := ts.server.handleValidateRelease(ctx, ValidateReleaseToolInput{
			ReleaseID: "nonexistent",
		})
		require.NoError(t, err)
		parsed := parseJSONMap(t, result)
		assert.Equal(t, false, parsed["valid"])
		assert.Equal(t, false, parsed["can_proceed"])

		blockingIssues, ok := parsed["blocking_issues"].([]any)
		require.True(t, ok)
		assert.Contains(t, blockingIssues[0], "Release not found")
	})

	t.Run("validation with governance check", func(t *testing.T) {
		ts := newTestSetup(repoRoot)
		ctx := context.Background()

		result, err := ts.server.handleValidateRelease(ctx, ValidateReleaseToolInput{
			CheckGovernance: true,
		})
		require.NoError(t, err)
		parsed := parseJSONMap(t, result)
		assert.Equal(t, true, parsed["valid"])
		// Governance check won't appear since no governance service on adapter
	})
}

func TestHandleEvaluateWithAdapterPath_NoGovernance(t *testing.T) {
	repoRoot := "/test/repo"
	ts := newTestSetup(repoRoot)

	ctx := context.Background()
	// handleEvaluate requires HasGovernanceService AND HasReleaseServices
	// Our setup has release services but no governance, so falls through to riskCalc
	result, err := ts.server.handleEvaluate(ctx, EvaluateToolInput{})
	require.NoError(t, err)
	// Falls through to basic risk calculation (NewServer creates a default riskCalc)
	parsed := parseJSONMap(t, result)
	assert.Contains(t, parsed, "score")
}

// =============================================================================
// Additional adapter-level tests to increase adapters.go coverage
// =============================================================================

func TestAdapterCancelSuccessPath(t *testing.T) {
	rel := createPlannedRelease("cancel-success", "/test/repo")
	repo := &mockReleaseRepository{releases: []*domainrelease.ReleaseRun{rel}}
	adapter := NewAdapter(WithAdapterRepo(repo))

	ctx := context.Background()
	output, err := adapter.Cancel(ctx, CancelInput{
		ReleaseID: "cancel-success",
		Reason:    "test cancel",
	})
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, "cancel-success", output.ReleaseID)
	assert.Equal(t, "planned", output.PreviousState)
	assert.Equal(t, "canceled", output.NewState)
}

func TestAdapterNotesWithServicesButNoGenerateNotesUseCase(t *testing.T) {
	services := &domainrelease.Services{}
	adapter := NewAdapter(WithReleaseServices(services))

	ctx := context.Background()
	_, err := adapter.Notes(ctx, NotesInput{ReleaseID: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generate notes use case not configured")
}

func TestAdapterApproveWithServicesButNoApproveUseCase(t *testing.T) {
	services := &domainrelease.Services{}
	adapter := NewAdapter(WithReleaseServices(services))

	ctx := context.Background()
	_, err := adapter.Approve(ctx, ApproveInput{ReleaseID: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "approve release use case not configured")
}

func TestAdapterPublishWithServicesButNoPublishUseCase(t *testing.T) {
	services := &domainrelease.Services{}
	adapter := NewAdapter(WithReleaseServices(services))

	ctx := context.Background()
	_, err := adapter.Publish(ctx, PublishInput{ReleaseID: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "publish release use case not configured")
}

func TestAdapterGetStatusWithServicesButNoGetStatusUseCase(t *testing.T) {
	services := &domainrelease.Services{}
	adapter := NewAdapter(WithReleaseServices(services))

	ctx := context.Background()
	_, err := adapter.GetStatus(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get status use case not configured")
}

func TestAdapterSetAndGetRepoRootPath(t *testing.T) {
	adapter := NewAdapter()
	assert.Equal(t, "", adapter.GetRepoRoot())

	adapter.SetRepoRoot("/new/path")
	assert.Equal(t, "/new/path", adapter.GetRepoRoot())
}

func TestAdapterBumpRepoPathFallbackToDefault(t *testing.T) {
	services := &domainrelease.Services{
		BumpVersion: nil,
	}
	adapter := NewAdapter(WithReleaseServices(services))

	ctx := context.Background()
	_, err := adapter.Bump(ctx, BumpInput{})
	require.Error(t, err)
	// Error from nil BumpVersion but the path defaulting logic executed
	assert.Contains(t, err.Error(), "release services not configured")
}

func TestAdapterBumpWithEmptyRepoPath(t *testing.T) {
	// Test path where RepositoryPath is empty and repoRoot is also empty
	services := &domainrelease.Services{
		BumpVersion: nil,
	}
	adapter := NewAdapter(
		WithReleaseServices(services),
		WithRepoRoot(""), // empty
	)

	ctx := context.Background()
	_, err := adapter.Bump(ctx, BumpInput{
		RepositoryPath: "", // empty
	})
	require.Error(t, err)
}
