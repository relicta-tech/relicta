package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/internal/httpserver/dto"
)

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
