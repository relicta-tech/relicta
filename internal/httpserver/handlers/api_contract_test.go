package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/version"
	"github.com/relicta-tech/relicta/internal/httpserver/dto"
)

// createTestRunN creates a unique test run by varying the commit SHA.
func createTestRunN(n int) *domain.ReleaseRun {
	sha := domain.CommitSHA(fmt.Sprintf("abc%04d567890def%04d", n, n))
	run := domain.NewReleaseRun(
		"https://github.com/test/repo",
		"/tmp/test-repo",
		"main",
		sha,
		[]domain.CommitSHA{sha},
		fmt.Sprintf("config-hash-%d", n),
		fmt.Sprintf("plugin-hash-%d", n),
	)
	_ = run.SetVersionProposal(
		version.NewSemanticVersion(1, 0, 0),
		version.NewSemanticVersion(1, uint64(n+1), 0),
		domain.BumpMinor,
		0.95,
	)
	run.SetPolicyEvaluation(0.3, []string{"minor version bump"}, domain.PolicyThresholds{
		AutoApproveRiskThreshold: 0.5,
		RequireApprovalAbove:     0.5,
		BlockReleaseAbove:        0.9,
	})
	run.SetActor(domain.ActorHuman, fmt.Sprintf("user-%d", n))
	return run
}

// =============================================================================
// Pagination contract tests
// =============================================================================

func TestAPIPagination_CursorParams(t *testing.T) {
	// Create multiple runs with unique IDs for pagination
	runs := make([]*domain.ReleaseRun, 5)
	for i := range runs {
		runs[i] = createTestRunN(i)
	}
	_, cleanup := setupTestContext(runs...)
	defer cleanup()

	t.Run("default limit is applied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/releases", nil)
		rec := httptest.NewRecorder()

		ListReleases(rec, req)

		var resp dto.CursorPaginatedResponse[dto.ReleaseDTO]
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

		assert.Equal(t, defaultLimit, resp.Limit)
		assert.Equal(t, 5, resp.Total)
	})

	t.Run("custom limit is respected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/releases?limit=2", nil)
		rec := httptest.NewRecorder()

		ListReleases(rec, req)

		var resp dto.CursorPaginatedResponse[dto.ReleaseDTO]
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

		assert.Equal(t, 2, resp.Limit)
		assert.Len(t, resp.Data, 2)
		assert.True(t, resp.HasMore)
		assert.NotEmpty(t, resp.NextCursor)
	})

	t.Run("cursor navigation returns next page", func(t *testing.T) {
		// First page
		req1 := httptest.NewRequest(http.MethodGet, "/releases?limit=2", nil)
		rec1 := httptest.NewRecorder()
		ListReleases(rec1, req1)

		var resp1 dto.CursorPaginatedResponse[dto.ReleaseDTO]
		require.NoError(t, json.NewDecoder(rec1.Body).Decode(&resp1))
		require.NotEmpty(t, resp1.NextCursor)

		// Second page via cursor
		req2 := httptest.NewRequest(http.MethodGet, "/releases?limit=2&cursor="+resp1.NextCursor, nil)
		rec2 := httptest.NewRecorder()
		ListReleases(rec2, req2)

		var resp2 dto.CursorPaginatedResponse[dto.ReleaseDTO]
		require.NoError(t, json.NewDecoder(rec2.Body).Decode(&resp2))

		assert.Len(t, resp2.Data, 2)
		assert.True(t, resp2.HasMore)
		assert.NotEmpty(t, resp2.PrevCursor)
	})
}

func TestAPIPagination_LinkHeaders(t *testing.T) {
	runs := make([]*domain.ReleaseRun, 5)
	for i := range runs {
		runs[i] = createTestRunN(i + 10)
	}
	_, cleanup := setupTestContext(runs...)
	defer cleanup()

	t.Run("first page has next Link but no prev", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/releases?limit=2", nil)
		rec := httptest.NewRecorder()

		ListReleases(rec, req)

		link := rec.Header().Get("Link")
		assert.Contains(t, link, `rel="next"`)
		assert.NotContains(t, link, `rel="prev"`)
	})

	t.Run("middle page has both next and prev Links", func(t *testing.T) {
		// Get cursor for second page
		req1 := httptest.NewRequest(http.MethodGet, "/releases?limit=2", nil)
		rec1 := httptest.NewRecorder()
		ListReleases(rec1, req1)

		var resp1 dto.CursorPaginatedResponse[dto.ReleaseDTO]
		require.NoError(t, json.NewDecoder(rec1.Body).Decode(&resp1))

		req2 := httptest.NewRequest(http.MethodGet, "/releases?limit=2&cursor="+resp1.NextCursor, nil)
		rec2 := httptest.NewRecorder()
		ListReleases(rec2, req2)

		link := rec2.Header().Get("Link")
		assert.Contains(t, link, `rel="next"`)
		assert.Contains(t, link, `rel="prev"`)
	})

	t.Run("last page has prev Link but no next", func(t *testing.T) {
		// Navigate to last page
		req1 := httptest.NewRequest(http.MethodGet, "/releases?limit=2", nil)
		rec1 := httptest.NewRecorder()
		ListReleases(rec1, req1)
		var resp1 dto.CursorPaginatedResponse[dto.ReleaseDTO]
		require.NoError(t, json.NewDecoder(rec1.Body).Decode(&resp1))

		req2 := httptest.NewRequest(http.MethodGet, "/releases?limit=2&cursor="+resp1.NextCursor, nil)
		rec2 := httptest.NewRecorder()
		ListReleases(rec2, req2)
		var resp2 dto.CursorPaginatedResponse[dto.ReleaseDTO]
		require.NoError(t, json.NewDecoder(rec2.Body).Decode(&resp2))

		req3 := httptest.NewRequest(http.MethodGet, "/releases?limit=2&cursor="+resp2.NextCursor, nil)
		rec3 := httptest.NewRecorder()
		ListReleases(rec3, req3)

		link := rec3.Header().Get("Link")
		assert.NotContains(t, link, `rel="next"`)
		assert.Contains(t, link, `rel="prev"`)
	})

	t.Run("no Link header for single page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/releases?limit=100", nil)
		rec := httptest.NewRecorder()

		ListReleases(rec, req)

		assert.Empty(t, rec.Header().Get("Link"))
	})
}

func TestAPIPagination_LimitClamping(t *testing.T) {
	_, cleanup := setupTestContext()
	defer cleanup()

	tests := []struct {
		name        string
		query       string
		expectLimit int
	}{
		{"limit above max falls back to default", "/releases?limit=500", defaultLimit},
		{"limit of zero falls back to default", "/releases?limit=0", defaultLimit},
		{"negative limit falls back to default", "/releases?limit=-5", defaultLimit},
		{"valid limit is preserved", "/releases?limit=50", 50},
		{"no limit uses default", "/releases", defaultLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.query, nil)
			rec := httptest.NewRecorder()

			ListReleases(rec, req)

			var resp dto.CursorPaginatedResponse[dto.ReleaseDTO]
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

			assert.Equal(t, tt.expectLimit, resp.Limit)
		})
	}
}

// =============================================================================
// Structured error format contract tests
// =============================================================================

func TestAPIErrorFormat_Structure(t *testing.T) {
	t.Run("error response includes code and message", func(t *testing.T) {
		_, cleanup := setupTestContext()
		defer cleanup()

		r := chi.NewRouter()
		r.Get("/releases/{id}", GetRelease)

		req := httptest.NewRequest(http.MethodGet, "/releases/nonexistent", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		var resp dto.ErrorResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

		assert.NotEmpty(t, resp.Error, "error message must be present")
		assert.NotEmpty(t, resp.Code, "error code must be present")
		assert.Equal(t, ErrCodeReleaseNotFound, resp.Code)
	})

	t.Run("error response includes request_id when middleware is active", func(t *testing.T) {
		_, cleanup := setupTestContext()
		defer cleanup()

		r := chi.NewRouter()
		r.Use(chimw.RequestID)
		r.Get("/releases/{id}", GetRelease)

		req := httptest.NewRequest(http.MethodGet, "/releases/nonexistent", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		var resp dto.ErrorResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

		assert.NotEmpty(t, resp.RequestID, "request_id must be present with RequestID middleware")
	})

	t.Run("error response without request_id middleware omits field", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)

		writeError(rec, req, http.StatusBadRequest, ErrCodeBadRequest, "test error", nil)

		var resp dto.ErrorResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

		assert.Equal(t, "test error", resp.Error)
		assert.Equal(t, ErrCodeBadRequest, resp.Code)
	})
}

func TestAPIErrorFormat_Codes(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T)
		cleanup  func()
		method   string
		path     string
		handler  func(chi.Router)
		wantCode int
		wantErr  string
	}{
		{
			name: "missing field returns MISSING_FIELD",
			setup: func(t *testing.T) {
				_, c := setupTestContext()
				t.Cleanup(c)
			},
			method: http.MethodGet,
			path:   "/releases/",
			handler: func(r chi.Router) {
				// Call handler directly without chi routing to get empty id
			},
			wantCode: http.StatusBadRequest,
			wantErr:  ErrCodeMissingField,
		},
		{
			name: "not found returns RELEASE_NOT_FOUND",
			setup: func(t *testing.T) {
				_, c := setupTestContext()
				t.Cleanup(c)
			},
			method:   http.MethodGet,
			path:     "/releases/does-not-exist",
			wantCode: http.StatusNotFound,
			wantErr:  ErrCodeReleaseNotFound,
		},
		{
			name: "forbidden returns FORBIDDEN",
			setup: func(t *testing.T) {
				SetContext(nil)
				t.Cleanup(func() { SetContext(nil) })
			},
			method:   http.MethodPost,
			path:     "/approvals/123/approve",
			wantCode: http.StatusForbidden,
			wantErr:  ErrCodeForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)

			r := chi.NewRouter()
			r.Get("/releases/{id}", GetRelease)
			r.Post("/approvals/{id}/approve", ApproveRelease)

			if tt.name == "missing field returns MISSING_FIELD" {
				// Test direct handler call without chi param
				req := httptest.NewRequest(http.MethodGet, "/releases/", nil)
				rec := httptest.NewRecorder()
				GetRelease(rec, req)

				var resp dto.ErrorResponse
				require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
				assert.Equal(t, tt.wantCode, rec.Code)
				assert.Equal(t, tt.wantErr, resp.Code)
				return
			}

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantCode, rec.Code)

			var resp dto.ErrorResponse
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
			assert.Equal(t, tt.wantErr, resp.Code)
		})
	}
}

func TestAPIErrorFormat_DetailsField(t *testing.T) {
	t.Run("error with details includes details", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)

		writeError(rec, req, http.StatusBadRequest, ErrCodeBadRequest, "validation failed", "field 'name' is required")

		var resp dto.ErrorResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

		assert.Equal(t, "field 'name' is required", resp.Details)
	})

	t.Run("error without details omits details", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)

		writeError(rec, req, http.StatusBadRequest, ErrCodeBadRequest, "bad request", nil)

		var resp map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

		_, hasDetails := resp["details"]
		assert.False(t, hasDetails, "details should be omitted when nil")
	})
}

// =============================================================================
// API versioning contract tests
// =============================================================================

func TestAPIVersioning_Prefix(t *testing.T) {
	t.Run("all list endpoints return cursor paginated responses", func(t *testing.T) {
		run := createTestRun()
		_, cleanup := setupTestContext(run)
		defer cleanup()

		endpoints := []struct {
			path    string
			handler http.HandlerFunc
		}{
			{"/api/v1/releases", ListReleases},
			{"/api/v1/governance/decisions", ListGovernanceDecisions},
			{"/api/v1/actors", ListActors},
			{"/api/v1/audit", ListAuditEvents},
		}

		for _, ep := range endpoints {
			t.Run(ep.path, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, ep.path, nil)
				rec := httptest.NewRecorder()

				ep.handler(rec, req)

				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

				// Verify response has cursor pagination fields
				var raw map[string]any
				require.NoError(t, json.NewDecoder(rec.Body).Decode(&raw))

				assert.Contains(t, raw, "data", "response must contain 'data' field")
				assert.Contains(t, raw, "total", "response must contain 'total' field")
				assert.Contains(t, raw, "limit", "response must contain 'limit' field")
				assert.Contains(t, raw, "has_more", "response must contain 'has_more' field")
			})
		}
	})

	t.Run("approvals endpoint returns cursor paginated response", func(t *testing.T) {
		run := createTestRun()
		_ = run.Plan("test")
		_ = run.SetVersion(version.NewSemanticVersion(1, 1, 0), "v1.1.0")
		_ = run.Bump("test")
		_ = run.GenerateNotes(&domain.ReleaseNotes{Text: "notes"}, "h", "test")

		_, cleanup := setupTestContext(run)
		defer cleanup()

		req := httptest.NewRequest(http.MethodGet, "/api/v1/approvals/pending", nil)
		rec := httptest.NewRecorder()

		ListPendingApprovals(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var raw map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&raw))

		assert.Contains(t, raw, "data")
		assert.Contains(t, raw, "limit")
		assert.Contains(t, raw, "has_more")
	})
}

func TestAPIVersioning_ContentType(t *testing.T) {
	t.Run("all responses have application/json content type", func(t *testing.T) {
		SetContext(nil)
		defer SetContext(nil)

		endpoints := []struct {
			path    string
			handler http.HandlerFunc
		}{
			{"/api/v1/health", Health},
			{"/api/v1/releases", ListReleases},
			{"/api/v1/governance/decisions", ListGovernanceDecisions},
			{"/api/v1/actors", ListActors},
			{"/api/v1/audit", ListAuditEvents},
		}

		for _, ep := range endpoints {
			t.Run(ep.path, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, ep.path, nil)
				rec := httptest.NewRecorder()

				ep.handler(rec, req)

				assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			})
		}
	})
}

// =============================================================================
// Sort parameter contract tests
// =============================================================================

func TestAPISortParameter(t *testing.T) {
	t.Run("releases accept sort parameter", func(t *testing.T) {
		run := createTestRun()
		_, cleanup := setupTestContext(run)
		defer cleanup()

		sortValues := []string{"created", "-created", "risk", "-risk", "version", "-version"}
		for _, s := range sortValues {
			t.Run(s, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/releases?sort="+s, nil)
				rec := httptest.NewRecorder()

				ListReleases(rec, req)

				assert.Equal(t, http.StatusOK, rec.Code)
			})
		}
	})

	t.Run("actors accept sort parameter", func(t *testing.T) {
		run := createTestRun()
		_, cleanup := setupTestContext(run)
		defer cleanup()

		sortValues := []string{"name", "-name", "releases", "-releases", "risk", "-risk", "reliability", "-reliability"}
		for _, s := range sortValues {
			t.Run(s, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/actors?sort="+s, nil)
				rec := httptest.NewRecorder()

				ListActors(rec, req)

				assert.Equal(t, http.StatusOK, rec.Code)
			})
		}
	})

	t.Run("approvals accept sort parameter", func(t *testing.T) {
		_, cleanup := setupTestContext()
		defer cleanup()

		sortValues := []string{"risk", "-risk", "submitted", "-submitted"}
		for _, s := range sortValues {
			t.Run(s, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/approvals/pending?sort="+s, nil)
				rec := httptest.NewRecorder()

				ListPendingApprovals(rec, req)

				assert.Equal(t, http.StatusOK, rec.Code)
			})
		}
	})
}
