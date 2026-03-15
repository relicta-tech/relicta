package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/internal/analytics"
)

// setupAnalyticsAggregator creates a CachedAggregator with seeded data for testing.
func setupAnalyticsAggregator(t *testing.T) func() {
	t.Helper()

	store, err := analytics.NewFileStore(filepath.Join(t.TempDir(), "analytics"))
	require.NoError(t, err)

	svc := analytics.NewService(store)
	ctx := context.Background()

	// Fixed clock for deterministic timestamps
	baseTime := time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
	svc = svc.WithClock(func() time.Time { return baseTime })

	// Seed risk evaluation events across multiple days
	for i, input := range []struct {
		ts    time.Time
		score float64
	}{
		{baseTime, 0.3},
		{baseTime.Add(2 * time.Hour), 0.5},
		{baseTime.AddDate(0, 0, 1), 0.7},
		{baseTime.AddDate(0, 0, 2), 0.2},
	} {
		svc = svc.WithClock(func() time.Time { return input.ts })
		err := svc.Capture(ctx, analytics.EventRiskEvaluation, "rel-"+string(rune('1'+i)),
			analytics.RiskEvaluationPayload{RiskScore: input.score, RiskLevel: "test"})
		require.NoError(t, err)
	}

	// Seed policy decision events
	for _, input := range []struct {
		ts       time.Time
		decision string
	}{
		{baseTime, "approve"},
		{baseTime.Add(time.Hour), "deny"},
		{baseTime.AddDate(0, 0, 1), "approve"},
		{baseTime.AddDate(0, 0, 1), "require_review"},
	} {
		svc = svc.WithClock(func() time.Time { return input.ts })
		err := svc.Capture(ctx, analytics.EventPolicyDecision, "",
			analytics.PolicyDecisionPayload{Decision: input.decision, RiskScore: 0.5})
		require.NoError(t, err)
	}

	// Seed approval outcome events
	for _, input := range []struct {
		ts      time.Time
		actorID string
		outcome string
	}{
		{baseTime, "alice", "approved"},
		{baseTime.Add(time.Hour), "bob", "rejected"},
		{baseTime.AddDate(0, 0, 1), "alice", "approved"},
	} {
		svc = svc.WithClock(func() time.Time { return input.ts })
		err := svc.Capture(ctx, analytics.EventApprovalOutcome, "",
			analytics.ApprovalOutcomePayload{Outcome: input.outcome, ActorID: input.actorID, ActorKind: "human"})
		require.NoError(t, err)
	}

	agg := analytics.NewCachedAggregator(svc, 5*time.Minute)
	SetAnalyticsAggregator(agg)

	return func() { SetAnalyticsAggregator(nil) }
}

// =============================================================================
// Risk Trends Endpoint Tests
// =============================================================================

func TestGetAnalyticsRiskTrends_NoAggregator(t *testing.T) {
	SetAnalyticsAggregator(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/risk-trends", nil)
	rec := httptest.NewRecorder()

	GetAnalyticsRiskTrends(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	trends, ok := resp["trends"].([]any)
	require.True(t, ok)
	assert.Empty(t, trends)
}

func TestGetAnalyticsRiskTrends_WithData(t *testing.T) {
	cleanup := setupAnalyticsAggregator(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/risk-trends", nil)
	rec := httptest.NewRecorder()

	GetAnalyticsRiskTrends(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string][]analytics.RiskTrendPoint
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	trends := resp["trends"]
	assert.NotEmpty(t, trends)
}

func TestGetAnalyticsRiskTrends_WithDateRange(t *testing.T) {
	cleanup := setupAnalyticsAggregator(t)
	defer cleanup()

	from := "2026-03-10T00:00:00Z"
	to := "2026-03-10T23:59:59Z"

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/risk-trends?from="+from+"&to="+to, nil)
	rec := httptest.NewRecorder()

	GetAnalyticsRiskTrends(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string][]analytics.RiskTrendPoint
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	trends := resp["trends"]
	// Should only have data from March 10
	for _, point := range trends {
		assert.Equal(t, "2026-03-10", point.Bucket)
	}
}

func TestGetAnalyticsRiskTrends_WeeklyGranularity(t *testing.T) {
	cleanup := setupAnalyticsAggregator(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/risk-trends?granularity=week", nil)
	rec := httptest.NewRecorder()

	GetAnalyticsRiskTrends(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string][]analytics.RiskTrendPoint
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	// All events are in the same week, so should have 1 bucket
	assert.Len(t, resp["trends"], 1)
}

func TestGetAnalyticsRiskTrends_MonthlyGranularity(t *testing.T) {
	cleanup := setupAnalyticsAggregator(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/risk-trends?granularity=month", nil)
	rec := httptest.NewRecorder()

	GetAnalyticsRiskTrends(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string][]analytics.RiskTrendPoint
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Len(t, resp["trends"], 1)
	assert.Equal(t, "2026-03", resp["trends"][0].Bucket)
}

// =============================================================================
// Decisions Endpoint Tests
// =============================================================================

func TestGetAnalyticsDecisions_NoAggregator(t *testing.T) {
	SetAnalyticsAggregator(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/decisions", nil)
	rec := httptest.NewRecorder()

	GetAnalyticsDecisions(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	decisions, ok := resp["decisions"].([]any)
	require.True(t, ok)
	assert.Empty(t, decisions)
}

func TestGetAnalyticsDecisions_WithData(t *testing.T) {
	cleanup := setupAnalyticsAggregator(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/decisions", nil)
	rec := httptest.NewRecorder()

	GetAnalyticsDecisions(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string][]analytics.DecisionDistribution
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	decisions := resp["decisions"]
	assert.NotEmpty(t, decisions)

	// Check that we have expected decision counts
	totalApprove := 0
	totalDeny := 0
	totalReview := 0
	for _, d := range decisions {
		totalApprove += d.Approve
		totalDeny += d.Deny
		totalReview += d.RequireReview
	}
	assert.Equal(t, 2, totalApprove)
	assert.Equal(t, 1, totalDeny)
	assert.Equal(t, 1, totalReview)
}

func TestGetAnalyticsDecisions_WithDateFilter(t *testing.T) {
	cleanup := setupAnalyticsAggregator(t)
	defer cleanup()

	from := "2026-03-11T00:00:00Z"
	to := "2026-03-11T23:59:59Z"

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/decisions?from="+from+"&to="+to, nil)
	rec := httptest.NewRecorder()

	GetAnalyticsDecisions(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string][]analytics.DecisionDistribution
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	for _, d := range resp["decisions"] {
		assert.Equal(t, "2026-03-11", d.Bucket)
	}
}

// =============================================================================
// Team Endpoint Tests
// =============================================================================

func TestGetAnalyticsTeam_NoAggregator(t *testing.T) {
	SetAnalyticsAggregator(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/team", nil)
	rec := httptest.NewRecorder()

	GetAnalyticsTeam(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	team, ok := resp["team"].([]any)
	require.True(t, ok)
	assert.Empty(t, team)
}

func TestGetAnalyticsTeam_WithData(t *testing.T) {
	cleanup := setupAnalyticsAggregator(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/team", nil)
	rec := httptest.NewRecorder()

	GetAnalyticsTeam(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string][]analytics.TeamMetrics
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	team := resp["team"]
	assert.NotEmpty(t, team)

	// Find alice
	var alice *analytics.TeamMetrics
	for i := range team {
		if team[i].ActorID == "alice" {
			alice = &team[i]
		}
	}
	require.NotNil(t, alice)
	assert.Equal(t, 2, alice.ApprovalCount)
	assert.Equal(t, 0, alice.RejectionCount)
}

func TestGetAnalyticsTeam_WithDateFilter(t *testing.T) {
	cleanup := setupAnalyticsAggregator(t)
	defer cleanup()

	from := "2026-03-10T00:00:00Z"
	to := "2026-03-10T12:00:00Z"

	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/team?from="+from+"&to="+to, nil)
	rec := httptest.NewRecorder()

	GetAnalyticsTeam(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// =============================================================================
// Integration Test: Full Router
// =============================================================================

func TestAnalyticsRoutes_Integration(t *testing.T) {
	cleanup := setupAnalyticsAggregator(t)
	defer cleanup()

	r := chi.NewRouter()
	r.Route("/api/v1/analytics", func(r chi.Router) {
		r.Get("/risk-trends", GetAnalyticsRiskTrends)
		r.Get("/decisions", GetAnalyticsDecisions)
		r.Get("/team", GetAnalyticsTeam)
	})

	endpoints := []string{
		"/api/v1/analytics/risk-trends",
		"/api/v1/analytics/decisions",
		"/api/v1/analytics/team",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, endpoint, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			var resp map[string]any
			err := json.NewDecoder(rec.Body).Decode(&resp)
			require.NoError(t, err)
		})
	}
}

func TestAnalyticsRoutes_WithQueryParams(t *testing.T) {
	cleanup := setupAnalyticsAggregator(t)
	defer cleanup()

	r := chi.NewRouter()
	r.Route("/api/v1/analytics", func(r chi.Router) {
		r.Get("/risk-trends", GetAnalyticsRiskTrends)
		r.Get("/decisions", GetAnalyticsDecisions)
		r.Get("/team", GetAnalyticsTeam)
	})

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "risk trends with all params",
			url:  "/api/v1/analytics/risk-trends?from=2026-03-01T00:00:00Z&to=2026-03-31T23:59:59Z&granularity=day",
		},
		{
			name: "decisions weekly",
			url:  "/api/v1/analytics/decisions?granularity=week",
		},
		{
			name: "team with date range",
			url:  "/api/v1/analytics/team?from=2026-03-01T00:00:00Z&to=2026-03-31T23:59:59Z",
		},
		{
			name: "risk trends monthly",
			url:  "/api/v1/analytics/risk-trends?granularity=month",
		},
		{
			name: "invalid from param is ignored gracefully",
			url:  "/api/v1/analytics/risk-trends?from=invalid-date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

// =============================================================================
// parseTimeRange Tests
// =============================================================================

func TestParseTimeRange(t *testing.T) {
	t.Run("valid from and to", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?from=2026-03-10T00:00:00Z&to=2026-03-15T23:59:59Z", nil)
		from, to := parseTimeRange(req)
		require.NotNil(t, from)
		require.NotNil(t, to)
		assert.Equal(t, 2026, from.Year())
		assert.Equal(t, 2026, to.Year())
	})

	t.Run("no params", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		from, to := parseTimeRange(req)
		assert.Nil(t, from)
		assert.Nil(t, to)
	})

	t.Run("invalid from ignored", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?from=not-a-date", nil)
		from, _ := parseTimeRange(req)
		assert.Nil(t, from)
	})

	t.Run("only to param", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?to=2026-03-15T00:00:00Z", nil)
		from, to := parseTimeRange(req)
		assert.Nil(t, from)
		require.NotNil(t, to)
	})
}
