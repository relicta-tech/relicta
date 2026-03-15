package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	Healthz(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp LivenessResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "alive", resp.Status)
	assert.NotEmpty(t, resp.Timestamp)
}

func TestReadyz_AllHealthy(t *testing.T) {
	// Save and restore global checkers
	original := ReadinessCheckers
	defer func() { ReadinessCheckers = original }()

	ReadinessCheckers = nil
	RegisterReadinessChecker("test-db", func() ComponentStatus {
		return ComponentStatus{Status: "up", Message: "connected"}
	})
	RegisterReadinessChecker("test-cache", func() ComponentStatus {
		return ComponentStatus{Status: "up"}
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	Readyz(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ReadinessResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "ready", resp.Status)
	assert.Len(t, resp.Components, 2)
	assert.Equal(t, "up", resp.Components["test-db"].Status)
	assert.Equal(t, "connected", resp.Components["test-db"].Message)
	assert.Equal(t, "up", resp.Components["test-cache"].Status)
	assert.NotEmpty(t, resp.GoVersion)
	assert.NotEmpty(t, resp.Uptime)
}

func TestReadyz_ComponentDown(t *testing.T) {
	original := ReadinessCheckers
	defer func() { ReadinessCheckers = original }()

	ReadinessCheckers = nil
	RegisterReadinessChecker("healthy-service", func() ComponentStatus {
		return ComponentStatus{Status: "up"}
	})
	RegisterReadinessChecker("broken-db", func() ComponentStatus {
		return ComponentStatus{Status: "down", Message: "connection refused"}
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	Readyz(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var resp ReadinessResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "not_ready", resp.Status)
	assert.Equal(t, "down", resp.Components["broken-db"].Status)
	assert.Equal(t, "connection refused", resp.Components["broken-db"].Message)
}

func TestReadyz_NoCheckers(t *testing.T) {
	original := ReadinessCheckers
	defer func() { ReadinessCheckers = original }()

	ReadinessCheckers = nil

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	Readyz(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ReadinessResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "ready", resp.Status)
	assert.Empty(t, resp.Components)
}

func TestRegisterReadinessChecker(t *testing.T) {
	original := ReadinessCheckers
	defer func() { ReadinessCheckers = original }()

	ReadinessCheckers = nil

	RegisterReadinessChecker("svc-a", func() ComponentStatus {
		return ComponentStatus{Status: "up"}
	})
	RegisterReadinessChecker("svc-b", func() ComponentStatus {
		return ComponentStatus{Status: "up"}
	})

	assert.Len(t, ReadinessCheckers, 2)
	assert.Equal(t, "svc-a", ReadinessCheckers[0].Name)
	assert.Equal(t, "svc-b", ReadinessCheckers[1].Name)
}
