package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/version"
	"github.com/relicta-tech/relicta/internal/httpserver/dto"
)

// mockRepository is an in-memory implementation of ports.ReleaseRunRepository for testing.
type mockRepository struct {
	runs       map[domain.RunID]*domain.ReleaseRun
	latestRuns map[string]domain.RunID
	saveErr    error
	loadErr    error
	findErr    error
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		runs:       make(map[domain.RunID]*domain.ReleaseRun),
		latestRuns: make(map[string]domain.RunID),
	}
}

func (m *mockRepository) Save(_ context.Context, run *domain.ReleaseRun) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.runs[run.ID()] = run
	return nil
}

func (m *mockRepository) Load(_ context.Context, runID domain.RunID) (*domain.ReleaseRun, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	run, ok := m.runs[runID]
	if !ok {
		return nil, errors.New("run not found")
	}
	return run, nil
}

func (m *mockRepository) LoadBatch(_ context.Context, _ string, runIDs []domain.RunID) (map[domain.RunID]*domain.ReleaseRun, error) {
	result := make(map[domain.RunID]*domain.ReleaseRun)
	for _, id := range runIDs {
		if run, ok := m.runs[id]; ok {
			result[id] = run
		}
	}
	return result, nil
}

func (m *mockRepository) LoadLatest(ctx context.Context, repoRoot string) (*domain.ReleaseRun, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	runID, ok := m.latestRuns[repoRoot]
	if !ok {
		return nil, errors.New("no latest run")
	}
	return m.Load(ctx, runID)
}

func (m *mockRepository) SetLatest(_ context.Context, repoRoot string, runID domain.RunID) error {
	m.latestRuns[repoRoot] = runID
	return nil
}

func (m *mockRepository) List(_ context.Context, _ string) ([]domain.RunID, error) {
	ids := make([]domain.RunID, 0, len(m.runs))
	for id := range m.runs {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *mockRepository) Delete(_ context.Context, runID domain.RunID) error {
	delete(m.runs, runID)
	return nil
}

func (m *mockRepository) FindByState(_ context.Context, _ string, state domain.RunState) ([]*domain.ReleaseRun, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var runs []*domain.ReleaseRun
	for _, run := range m.runs {
		if run.State() == state {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

func (m *mockRepository) FindActive(_ context.Context, _ string) ([]*domain.ReleaseRun, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var runs []*domain.ReleaseRun
	for _, run := range m.runs {
		if !run.State().IsFinal() {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

func (m *mockRepository) FindByPlanHash(_ context.Context, _ string, planHash string) (*domain.ReleaseRun, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	for _, run := range m.runs {
		if run.PlanHash() == planHash {
			return run, nil
		}
	}
	return nil, nil
}

// createTestRun creates a ReleaseRun for testing with common defaults.
func createTestRun() *domain.ReleaseRun {
	run := domain.NewReleaseRun(
		"https://github.com/test/repo",
		"/tmp/test-repo",
		"main",
		domain.CommitSHA("abc1234567890def"),
		[]domain.CommitSHA{"abc1234567890def", "def4567890abc123"},
		"config-hash-123",
		"plugin-hash-456",
	)
	_ = run.SetVersionProposal(
		version.NewSemanticVersion(1, 0, 0),
		version.NewSemanticVersion(1, 1, 0),
		domain.BumpMinor,
		0.95,
	)
	run.SetPolicyEvaluation(0.3, []string{"minor version bump", "2 commits"}, domain.PolicyThresholds{
		AutoApproveRiskThreshold: 0.5,
		RequireApprovalAbove:     0.5,
		BlockReleaseAbove:        0.9,
	})
	run.SetActor(domain.ActorHuman, "test-user")
	return run
}

// setupTestContext creates a handler Context with a mock repository containing the given runs.
func setupTestContext(runs ...*domain.ReleaseRun) (*mockRepository, func()) {
	repo := newMockRepository()
	for _, run := range runs {
		repo.runs[run.ID()] = run
	}

	svc := &release.Services{
		Repository: repo,
	}
	SetContext(&Context{ReleaseServices: svc})
	return repo, func() { SetContext(nil) }
}

// TestHealth tests the health endpoint.
func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	Health(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp HealthResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "healthy", resp.Status)
	assert.NotEmpty(t, resp.Timestamp)
	assert.NotEmpty(t, resp.Uptime)
	assert.NotEmpty(t, resp.GoVersion)
}

// TestHealthResponse_Fields tests all fields of HealthResponse.
func TestHealthResponse_Fields(t *testing.T) {
	resp := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Uptime:    "1h0m0s",
		Version:   "v1.0.0",
		GoVersion: "go1.22.0",
	}

	assert.Equal(t, "healthy", resp.Status)
	assert.NotEmpty(t, resp.Timestamp)
	assert.Equal(t, "1h0m0s", resp.Uptime)
	assert.Equal(t, "v1.0.0", resp.Version)
	assert.Equal(t, "go1.22.0", resp.GoVersion)
}

// TestSetContext tests setting and getting context.
func TestSetContext(t *testing.T) {
	// Reset context after test
	defer SetContext(nil)

	assert.Nil(t, GetContext())

	ctx := &Context{}
	SetContext(ctx)

	assert.Equal(t, ctx, GetContext())
}

// TestGetContext_Nil tests GetContext when not initialized.
func TestGetContext_Nil(t *testing.T) {
	// Reset context
	SetContext(nil)
	assert.Nil(t, GetContext())
}

// TestListReleases_NoContext tests ListReleases with no context.
func TestListReleases_NoContext(t *testing.T) {
	// Reset context
	SetContext(nil)

	req := httptest.NewRequest(http.MethodGet, "/releases", nil)
	rec := httptest.NewRecorder()

	ListReleases(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.PaginatedResponse[dto.ReleaseDTO]
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Empty(t, resp.Data)
	assert.Equal(t, 0, resp.Total)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.PageSize)
}

// TestGetActiveRelease_NoContext tests GetActiveRelease with no context.
func TestGetActiveRelease_NoContext(t *testing.T) {
	// Reset context
	SetContext(nil)

	req := httptest.NewRequest(http.MethodGet, "/releases/active", nil)
	rec := httptest.NewRecorder()

	GetActiveRelease(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Nil(t, resp["release"])
}

// TestGetRelease_NoContext tests GetRelease with no context.
func TestGetRelease_NoContext(t *testing.T) {
	// Reset context
	SetContext(nil)

	req := httptest.NewRequest(http.MethodGet, "/releases/123", nil)
	rec := httptest.NewRecorder()

	// Use chi router to properly set URL params
	r := chi.NewRouter()
	r.Get("/releases/{id}", GetRelease)
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestGetRelease_MissingID tests GetRelease with missing ID.
// Note: Context with nil ReleaseServices returns 404 before checking ID
func TestGetRelease_MissingID(t *testing.T) {
	SetContext(&Context{}) // ReleaseServices is nil
	defer SetContext(nil)

	// Create request without chi context (no URL param)
	req := httptest.NewRequest(http.MethodGet, "/releases/", nil)
	rec := httptest.NewRecorder()

	GetRelease(rec, req)

	// Handler checks ReleaseServices first, returns 404 when nil
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestGetReleaseEvents_NoContext tests GetReleaseEvents with no context.
func TestGetReleaseEvents_NoContext(t *testing.T) {
	// Reset context
	SetContext(nil)

	req := httptest.NewRequest(http.MethodGet, "/releases/123/events", nil)
	rec := httptest.NewRecorder()

	GetReleaseEvents(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	events, ok := resp["events"].([]any)
	require.True(t, ok)
	assert.Empty(t, events)
}

// TestGetReleaseEvents_MissingID tests GetReleaseEvents with missing ID.
// Note: Context with nil ReleaseServices returns 200 with empty events
func TestGetReleaseEvents_MissingID(t *testing.T) {
	SetContext(&Context{}) // ReleaseServices is nil
	defer SetContext(nil)

	req := httptest.NewRequest(http.MethodGet, "/releases//events", nil)
	rec := httptest.NewRecorder()

	GetReleaseEvents(rec, req)

	// Handler checks ReleaseServices first, returns empty events when nil
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Empty(t, resp["events"])
}

// TestGetRiskLevel tests risk level calculation.
func TestGetRiskLevel(t *testing.T) {
	tests := []struct {
		score    float64
		expected string
	}{
		{0.0, "low"},
		{0.3, "low"},
		{0.39, "low"},
		{0.4, "medium"},
		{0.5, "medium"},
		{0.69, "medium"},
		{0.7, "high"},
		{0.8, "high"},
		{1.0, "high"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := getRiskLevel(tt.score)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestRespondJSON tests JSON response helper.
func TestRespondJSON(t *testing.T) {
	rec := httptest.NewRecorder()

	data := map[string]string{"message": "hello"}
	respondJSON(rec, http.StatusOK, data)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp map[string]string
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "hello", resp["message"])
}

// TestListActors_NoContext tests ListActors with no context.
func TestListActors_NoContext(t *testing.T) {
	SetContext(nil)

	req := httptest.NewRequest(http.MethodGet, "/actors", nil)
	rec := httptest.NewRecorder()

	ListActors(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.PaginatedResponse[dto.ActorDTO]
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Empty(t, resp.Data)
	assert.Equal(t, 0, resp.Total)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.PageSize)
}

// TestGetActor_NoContext tests GetActor with no context.
func TestGetActor_NoContext(t *testing.T) {
	SetContext(nil)

	req := httptest.NewRequest(http.MethodGet, "/actors/test-actor", nil)
	rec := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Get("/actors/{id}", GetActor)
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestListPendingApprovals_NoContext tests ListPendingApprovals with no context.
func TestListPendingApprovals_NoContext(t *testing.T) {
	SetContext(nil)

	req := httptest.NewRequest(http.MethodGet, "/approvals", nil)
	rec := httptest.NewRecorder()

	ListPendingApprovals(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.PaginatedResponse[dto.ApprovalDTO]
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Empty(t, resp.Data)
	assert.Equal(t, 0, resp.Total)
}

// TestApproveRelease_NoAuth tests ApproveRelease without authentication.
func TestApproveRelease_NoAuth(t *testing.T) {
	SetContext(nil)

	req := httptest.NewRequest(http.MethodPost, "/approvals/123/approve", nil)
	rec := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/approvals/{id}/approve", ApproveRelease)
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestRejectRelease_NoAuth tests RejectRelease without authentication.
func TestRejectRelease_NoAuth(t *testing.T) {
	SetContext(nil)

	req := httptest.NewRequest(http.MethodPost, "/approvals/123/reject", nil)
	rec := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/approvals/{id}/reject", RejectRelease)
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestListGovernanceDecisions_NoContext tests ListGovernanceDecisions with no context.
func TestListGovernanceDecisions_NoContext(t *testing.T) {
	SetContext(nil)

	req := httptest.NewRequest(http.MethodGet, "/governance/decisions", nil)
	rec := httptest.NewRecorder()

	ListGovernanceDecisions(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.PaginatedResponse[dto.GovernanceDecisionDTO]
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Empty(t, resp.Data)
	assert.Equal(t, 0, resp.Total)
}

// TestGetRiskTrends_NoContext tests GetRiskTrends with no context.
func TestGetRiskTrends_NoContext(t *testing.T) {
	SetContext(nil)

	req := httptest.NewRequest(http.MethodGet, "/governance/risk-trends", nil)
	rec := httptest.NewRecorder()

	GetRiskTrends(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	trends, ok := resp["trends"].([]any)
	require.True(t, ok)
	assert.Empty(t, trends)
}

// TestGetFactorDistribution_NoContext tests GetFactorDistribution with no context.
func TestGetFactorDistribution_NoContext(t *testing.T) {
	SetContext(nil)

	req := httptest.NewRequest(http.MethodGet, "/governance/factors", nil)
	rec := httptest.NewRecorder()

	GetFactorDistribution(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	factors, ok := resp["factors"].([]any)
	require.True(t, ok)
	assert.Empty(t, factors)
}

// TestListAuditEvents_NoContext tests ListAuditEvents with no context.
func TestListAuditEvents_NoContext(t *testing.T) {
	SetContext(nil)

	req := httptest.NewRequest(http.MethodGet, "/audit", nil)
	rec := httptest.NewRecorder()

	ListAuditEvents(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.PaginatedResponse[dto.AuditEventDTO]
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Empty(t, resp.Data)
	assert.Equal(t, 0, resp.Total)
}

// TestRespondError tests error response helper.
func TestRespondError(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		message string
		details string
	}{
		{
			name:    "with details",
			status:  http.StatusBadRequest,
			message: "Bad request",
			details: "Missing required field",
		},
		{
			name:    "without details",
			status:  http.StatusInternalServerError,
			message: "Internal error",
			details: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			respondError(rec, tt.status, tt.message, tt.details)

			assert.Equal(t, tt.status, rec.Code)

			var resp dto.ErrorResponse
			err := json.NewDecoder(rec.Body).Decode(&resp)
			require.NoError(t, err)

			assert.Equal(t, tt.message, resp.Error)
			if tt.details != "" {
				assert.Equal(t, tt.details, resp.Details)
			}
		})
	}
}

// =============================================================================
// Tests with real Context and ReleaseServices
// =============================================================================

func TestMapReleaseToDTO(t *testing.T) {
	run := createTestRun()

	d := mapReleaseToDTO(run)

	assert.Equal(t, string(run.ID()), d.ID)
	assert.Equal(t, "draft", d.State)
	assert.Equal(t, "main", d.BaseRef)
	assert.Equal(t, "abc1234567890def", d.HeadRef)
	assert.Equal(t, 2, d.CommitCount)
	assert.Equal(t, "1.0.0", d.Version)
	assert.Equal(t, "1.1.0", d.NextVersion)
	assert.Equal(t, "minor", d.BumpType)
	assert.InDelta(t, 0.3, d.RiskScore, 0.001)
	assert.Equal(t, "low", d.RiskLevel)
	assert.Nil(t, d.ApprovedAt)
	assert.Empty(t, d.ApprovedBy)
	assert.Nil(t, d.PublishedAt)
	assert.Equal(t, []string{"minor version bump", "2 commits"}, d.ChangeTypes)
	assert.Empty(t, d.ReleaseNotes)
}

func TestMapReleaseToDTO_WithNotes(t *testing.T) {
	run := createTestRun()
	// Transition to planned, then versioned, then generate notes
	_ = run.Plan("test")
	_ = run.SetVersion(version.NewSemanticVersion(1, 1, 0), "v1.1.0")
	_ = run.Bump("test")
	_ = run.GenerateNotes(&domain.ReleaseNotes{
		Text:     "## Release Notes\n- Feature A",
		Provider: "openai",
		Model:    "gpt-4",
	}, "inputs-hash", "test")

	d := mapReleaseToDTO(run)

	assert.Equal(t, "notes_ready", d.State)
	assert.Equal(t, "## Release Notes\n- Feature A", d.ReleaseNotes)
}

func TestMapReleaseToDTO_WithApproval(t *testing.T) {
	run := createTestRun()
	_ = run.Plan("test")
	_ = run.SetVersion(version.NewSemanticVersion(1, 1, 0), "v1.1.0")
	_ = run.Bump("test")
	_ = run.GenerateNotes(&domain.ReleaseNotes{Text: "notes"}, "h", "test")
	_ = run.Approve("approver-user", false)

	d := mapReleaseToDTO(run)

	assert.Equal(t, "approved", d.State)
	assert.NotNil(t, d.ApprovedAt)
	assert.Equal(t, "approver-user", d.ApprovedBy)
}

func TestMapReleaseToDTO_HighRisk(t *testing.T) {
	run := createTestRun()
	run.SetPolicyEvaluation(0.8, []string{"major change"}, domain.PolicyThresholds{})

	d := mapReleaseToDTO(run)
	assert.Equal(t, "high", d.RiskLevel)
}

func TestMapReleaseToDTO_MediumRisk(t *testing.T) {
	run := createTestRun()
	run.SetPolicyEvaluation(0.5, []string{"moderate change"}, domain.PolicyThresholds{})

	d := mapReleaseToDTO(run)
	assert.Equal(t, "medium", d.RiskLevel)
}

func TestListReleases_WithContext(t *testing.T) {
	run := createTestRun()
	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/releases", nil)
	rec := httptest.NewRecorder()

	ListReleases(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.PaginatedResponse[dto.ReleaseDTO]
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, 1, resp.Total)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, string(run.ID()), resp.Data[0].ID)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.PageSize)
	assert.Equal(t, 1, resp.TotalPages)
}

func TestListReleases_Pagination(t *testing.T) {
	run := createTestRun()
	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/releases?page=1&page_size=5", nil)
	rec := httptest.NewRecorder()

	ListReleases(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.PaginatedResponse[dto.ReleaseDTO]
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, 5, resp.PageSize)
}

func TestListReleases_EmptyRepo(t *testing.T) {
	_, cleanup := setupTestContext()
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/releases", nil)
	rec := httptest.NewRecorder()

	ListReleases(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.PaginatedResponse[dto.ReleaseDTO]
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, 0, resp.Total)
	assert.Empty(t, resp.Data)
}

func TestGetActiveRelease_WithLatest(t *testing.T) {
	run := createTestRun()
	repo, cleanup := setupTestContext(run)
	defer cleanup()

	// Set as latest using cwd
	cwd := mustGetwd(t)
	repo.latestRuns[cwd] = run.ID()

	req := httptest.NewRequest(http.MethodGet, "/releases/active", nil)
	rec := httptest.NewRecorder()

	GetActiveRelease(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.NotNil(t, resp["release"])
}

func TestGetActiveRelease_NoLatest(t *testing.T) {
	_, cleanup := setupTestContext()
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/releases/active", nil)
	rec := httptest.NewRecorder()

	GetActiveRelease(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Nil(t, resp["release"])
}

func TestGetRelease_WithContext(t *testing.T) {
	run := createTestRun()
	_, cleanup := setupTestContext(run)
	defer cleanup()

	r := chi.NewRouter()
	r.Get("/releases/{id}", GetRelease)

	req := httptest.NewRequest(http.MethodGet, "/releases/"+string(run.ID()), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.ReleaseDTO
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, string(run.ID()), resp.ID)
}

func TestGetRelease_NotFound(t *testing.T) {
	_, cleanup := setupTestContext()
	defer cleanup()

	r := chi.NewRouter()
	r.Get("/releases/{id}", GetRelease)

	req := httptest.NewRequest(http.MethodGet, "/releases/nonexistent", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetRelease_MissingIDWithServices(t *testing.T) {
	_, cleanup := setupTestContext()
	defer cleanup()

	// Call directly without chi router (no URL param set)
	req := httptest.NewRequest(http.MethodGet, "/releases/", nil)
	rec := httptest.NewRecorder()

	GetRelease(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetReleaseEvents_WithContext(t *testing.T) {
	run := createTestRun()
	// Add some history by transitioning
	_ = run.Plan("test-actor")

	_, cleanup := setupTestContext(run)
	defer cleanup()

	r := chi.NewRouter()
	r.Get("/releases/{id}/events", GetReleaseEvents)

	req := httptest.NewRequest(http.MethodGet, "/releases/"+string(run.ID())+"/events", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	events, ok := resp["events"].([]any)
	require.True(t, ok)
	assert.NotEmpty(t, events)
}

func TestGetReleaseEvents_NotFound(t *testing.T) {
	_, cleanup := setupTestContext()
	defer cleanup()

	r := chi.NewRouter()
	r.Get("/releases/{id}/events", GetReleaseEvents)

	req := httptest.NewRequest(http.MethodGet, "/releases/nonexistent/events", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetReleaseEvents_MissingIDWithServices(t *testing.T) {
	_, cleanup := setupTestContext()
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/releases//events", nil)
	rec := httptest.NewRecorder()

	GetReleaseEvents(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListGovernanceDecisions_WithContext(t *testing.T) {
	run := createTestRun()
	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/governance/decisions", nil)
	rec := httptest.NewRecorder()

	ListGovernanceDecisions(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.PaginatedResponse[dto.GovernanceDecisionDTO]
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, 1, resp.Total)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "pending", resp.Data[0].Decision)
	assert.InDelta(t, 0.3, resp.Data[0].RiskScore, 0.001)
	assert.Equal(t, "low", resp.Data[0].RiskLevel)
	assert.Equal(t, "test-user", resp.Data[0].ActorID)
	assert.Equal(t, "human", resp.Data[0].ActorKind)
}

func TestListGovernanceDecisions_ApprovedRun(t *testing.T) {
	run := createTestRun()
	_ = run.Plan("test")
	_ = run.SetVersion(version.NewSemanticVersion(1, 1, 0), "v1.1.0")
	_ = run.Bump("test")
	_ = run.GenerateNotes(&domain.ReleaseNotes{Text: "notes"}, "h", "test")
	_ = run.Approve("approver", false)

	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/governance/decisions", nil)
	rec := httptest.NewRecorder()

	ListGovernanceDecisions(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.PaginatedResponse[dto.GovernanceDecisionDTO]
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	require.Len(t, resp.Data, 1)
	assert.Equal(t, "approve", resp.Data[0].Decision)
}

func TestListGovernanceDecisions_CanceledRun(t *testing.T) {
	run := createTestRun()
	_ = run.Cancel("too risky", "admin")

	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/governance/decisions", nil)
	rec := httptest.NewRecorder()

	ListGovernanceDecisions(rec, req)

	var resp dto.PaginatedResponse[dto.GovernanceDecisionDTO]
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	require.Len(t, resp.Data, 1)
	assert.Equal(t, "deny", resp.Data[0].Decision)
}

func TestGetRiskTrends_WithContext(t *testing.T) {
	run := createTestRun()
	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/governance/risk-trends?days=30", nil)
	rec := httptest.NewRecorder()

	GetRiskTrends(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	trends, ok := resp["trends"].([]any)
	require.True(t, ok)
	assert.Len(t, trends, 1)
}

func TestGetRiskTrends_CustomDays(t *testing.T) {
	run := createTestRun()
	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/governance/risk-trends?days=7", nil)
	rec := httptest.NewRecorder()

	GetRiskTrends(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGetFactorDistribution_WithContext(t *testing.T) {
	run := createTestRun()
	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/governance/factors", nil)
	rec := httptest.NewRecorder()

	GetFactorDistribution(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	factors, ok := resp["factors"].([]any)
	require.True(t, ok)
	assert.Len(t, factors, 2) // "minor version bump" and "2 commits"
}

func TestListActors_WithContext(t *testing.T) {
	run := createTestRun()
	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/actors", nil)
	rec := httptest.NewRecorder()

	ListActors(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.PaginatedResponse[dto.ActorDTO]
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, 1, resp.Total)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "test-user", resp.Data[0].ID)
	assert.Equal(t, "human", resp.Data[0].Kind)
	assert.Equal(t, 1, resp.Data[0].ReleaseCount)
	// reliability = (0 success * 0.6) + ((1 - 0.3 risk) * 0.4) = 0.28 < 0.5 = probation
	assert.Equal(t, "probation", resp.Data[0].TrustLevel)
}

func TestGetActor_WithContext(t *testing.T) {
	run := createTestRun()
	_, cleanup := setupTestContext(run)
	defer cleanup()

	r := chi.NewRouter()
	r.Get("/actors/{id}", GetActor)

	req := httptest.NewRequest(http.MethodGet, "/actors/test-user", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.ActorDTO
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "test-user", resp.ID)
	assert.Equal(t, "human", resp.Kind)
	assert.Equal(t, 1, resp.ReleaseCount)
}

func TestGetActor_NotFound(t *testing.T) {
	_, cleanup := setupTestContext()
	defer cleanup()

	r := chi.NewRouter()
	r.Get("/actors/{id}", GetActor)

	req := httptest.NewRequest(http.MethodGet, "/actors/nonexistent", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetActor_MissingIDWithServices(t *testing.T) {
	_, cleanup := setupTestContext()
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/actors/", nil)
	rec := httptest.NewRecorder()

	GetActor(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListPendingApprovals_WithContext(t *testing.T) {
	_, cleanup := setupTestContext()
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/approvals", nil)
	rec := httptest.NewRecorder()

	ListPendingApprovals(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.PaginatedResponse[dto.ApprovalDTO]
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	// No runs in NotesReady state
	assert.Empty(t, resp.Data)
}

func TestListPendingApprovals_WithPendingRun(t *testing.T) {
	run := createTestRun()
	_ = run.Plan("test")
	_ = run.SetVersion(version.NewSemanticVersion(1, 1, 0), "v1.1.0")
	_ = run.Bump("test")
	_ = run.GenerateNotes(&domain.ReleaseNotes{Text: "notes"}, "h", "test")
	// Now in NotesReady state

	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/approvals", nil)
	rec := httptest.NewRecorder()

	ListPendingApprovals(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.PaginatedResponse[dto.ApprovalDTO]
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	require.Len(t, resp.Data, 1)
	assert.Equal(t, string(run.ID()), resp.Data[0].ReleaseID)
	assert.Equal(t, "1.1.0", resp.Data[0].Version)
	assert.Equal(t, "low", resp.Data[0].RiskLevel)
}

func TestListAuditEvents_WithContext(t *testing.T) {
	run := createTestRun()
	_ = run.Plan("test-actor")

	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/audit", nil)
	rec := httptest.NewRecorder()

	ListAuditEvents(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.PaginatedResponse[dto.AuditEventDTO]
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Greater(t, resp.Total, 0)
	assert.NotEmpty(t, resp.Data)
	assert.Equal(t, "PLAN", resp.Data[0].Type)
}

func TestListAuditEvents_WithFilters(t *testing.T) {
	run := createTestRun()
	_ = run.Plan("test-actor")

	_, cleanup := setupTestContext(run)
	defer cleanup()

	// Filter by actor
	req := httptest.NewRequest(http.MethodGet, "/audit?actor=test-actor", nil)
	rec := httptest.NewRecorder()

	ListAuditEvents(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp dto.PaginatedResponse[dto.AuditEventDTO]
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	assert.Greater(t, resp.Total, 0)
}

func TestListAuditEvents_FilterByEventType(t *testing.T) {
	run := createTestRun()
	_ = run.Plan("test-actor")

	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/audit?event_type=PLAN", nil)
	rec := httptest.NewRecorder()

	ListAuditEvents(rec, req)

	var resp dto.PaginatedResponse[dto.AuditEventDTO]
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	for _, ev := range resp.Data {
		assert.Equal(t, "PLAN", ev.Type)
	}
}

func TestListAuditEvents_FilterByTimeRange(t *testing.T) {
	run := createTestRun()
	_ = run.Plan("test-actor")

	_, cleanup := setupTestContext(run)
	defer cleanup()

	from := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	to := time.Now().Add(1 * time.Hour).Format(time.RFC3339)

	req := httptest.NewRequest(http.MethodGet, "/audit?from="+from+"&to="+to, nil)
	rec := httptest.NewRecorder()

	ListAuditEvents(rec, req)

	var resp dto.PaginatedResponse[dto.AuditEventDTO]
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	assert.Greater(t, resp.Total, 0)
}

func TestListAuditEvents_LimitAndOffset(t *testing.T) {
	run := createTestRun()
	_ = run.Plan("test-actor")

	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/audit?limit=1&offset=0", nil)
	rec := httptest.NewRecorder()

	ListAuditEvents(rec, req)

	var resp dto.PaginatedResponse[dto.AuditEventDTO]
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	assert.LessOrEqual(t, len(resp.Data), 1)
}

func TestListAuditEvents_FilterByReleaseID(t *testing.T) {
	run := createTestRun()
	_ = run.Plan("test-actor")

	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/audit?release_id="+string(run.ID()), nil)
	rec := httptest.NewRecorder()

	ListAuditEvents(rec, req)

	var resp dto.PaginatedResponse[dto.AuditEventDTO]
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	assert.Greater(t, resp.Total, 0)
}

func TestListGovernanceDecisions_RequiresReview(t *testing.T) {
	run := createTestRun()
	// Set high risk so RequiresApproval() returns true
	run.SetPolicyEvaluation(0.7, []string{"breaking change"}, domain.PolicyThresholds{
		AutoApproveRiskThreshold: 0.3,
		RequireApprovalAbove:     0.3,
		BlockReleaseAbove:        0.9,
	})

	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/governance/decisions", nil)
	rec := httptest.NewRecorder()

	ListGovernanceDecisions(rec, req)

	var resp dto.PaginatedResponse[dto.GovernanceDecisionDTO]
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	require.Len(t, resp.Data, 1)
	assert.Equal(t, "require_review", resp.Data[0].Decision)
	assert.True(t, resp.Data[0].RequiresReview)
}

func TestListActors_UnknownActor(t *testing.T) {
	run := domain.NewReleaseRun(
		"https://github.com/test/repo",
		"/tmp/test-repo",
		"main",
		domain.CommitSHA("abc1234567890def"),
		nil,
		"", "",
	)
	// ActorID is empty by default

	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/actors", nil)
	rec := httptest.NewRecorder()

	ListActors(rec, req)

	var resp dto.PaginatedResponse[dto.ActorDTO]
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	require.Len(t, resp.Data, 1)
	assert.Equal(t, "unknown", resp.Data[0].ID)
}

func TestListReleases_PaginationBeyondTotal(t *testing.T) {
	run := createTestRun()
	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/releases?page=100&page_size=20", nil)
	rec := httptest.NewRecorder()

	ListReleases(rec, req)

	var resp dto.PaginatedResponse[dto.ReleaseDTO]
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	assert.Equal(t, 1, resp.Total)
	assert.Empty(t, resp.Data) // page 100 is beyond the data
}

func TestListGovernanceDecisions_Pagination(t *testing.T) {
	run := createTestRun()
	_, cleanup := setupTestContext(run)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/governance/decisions?page=1&page_size=10", nil)
	rec := httptest.NewRecorder()

	ListGovernanceDecisions(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// mustGetwd is a test helper that returns the current working directory.
func mustGetwd(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	require.NoError(t, err)
	return cwd
}
