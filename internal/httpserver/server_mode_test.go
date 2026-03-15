package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/httpserver/handlers"
)

// --- Server Mode Tests ---

func TestServerMode_EmbeddedWithFrontend(t *testing.T) {
	frontendFS := fstest.MapFS{
		"index.html": {Data: []byte("<html>Dashboard</html>")},
	}

	server := NewServer(ServerDeps{
		Config: config.DashboardConfig{
			Address:    ":0",
			ServerMode: config.ServerModeEmbedded,
			Auth:       config.DashboardAuthConfig{Mode: config.DashboardAuthNone},
		},
		Frontend: frontendFS,
	})

	// Root path should serve the frontend
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Dashboard")
}

func TestServerMode_APIOnlyNoFrontend(t *testing.T) {
	server := NewServer(ServerDeps{
		Config: config.DashboardConfig{
			Address:    ":0",
			ServerMode: config.ServerModeAPI,
			Auth:       config.DashboardAuthConfig{Mode: config.DashboardAuthNone},
		},
		Frontend: nil, // API-only mode: no frontend
	})

	// Root path should 404 (no frontend served)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	// API endpoints should still work
	req = httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec = httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestServerMode_DefaultIsEmpty(t *testing.T) {
	server := NewServer(ServerDeps{
		Config: config.DashboardConfig{
			Address: ":0",
			Auth:    config.DashboardAuthConfig{Mode: config.DashboardAuthNone},
		},
		Frontend: nil,
	})

	// With no mode set and no frontend, API should still work
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- CORS Tests ---

func TestCORS_WithAllowedOrigins(t *testing.T) {
	server := NewServer(ServerDeps{
		Config: config.DashboardConfig{
			Address:     ":0",
			CORSOrigins: []string{"http://localhost:5173", "https://dashboard.example.com"},
			Auth:        config.DashboardAuthConfig{Mode: config.DashboardAuthNone},
		},
		Frontend: nil,
	})

	// Preflight request from allowed origin
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, "http://localhost:5173", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_WithWildcard(t *testing.T) {
	server := NewServer(ServerDeps{
		Config: config.DashboardConfig{
			Address:     ":0",
			CORSOrigins: []string{"*"},
			Auth:        config.DashboardAuthConfig{Mode: config.DashboardAuthNone},
		},
		Frontend: nil,
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://any-site.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_RejectsUnknownOrigin(t *testing.T) {
	server := NewServer(ServerDeps{
		Config: config.DashboardConfig{
			Address:     ":0",
			CORSOrigins: []string{"http://localhost:5173"},
			Auth:        config.DashboardAuthConfig{Mode: config.DashboardAuthNone},
		},
		Frontend: nil,
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://evil.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	// go-chi/cors does not set the header for disallowed origins
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

// --- Health Probe Tests ---

func TestHealthzEndpoint(t *testing.T) {
	server := NewServer(ServerDeps{
		Config: config.DashboardConfig{
			Address: ":0",
			Auth:    config.DashboardAuthConfig{Mode: config.DashboardAuthNone},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp handlers.LivenessResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "alive", resp.Status)
}

func TestReadyzEndpoint(t *testing.T) {
	// Save and restore
	original := handlers.ReadinessCheckers
	defer func() { handlers.ReadinessCheckers = original }()
	handlers.ReadinessCheckers = nil

	server := NewServer(ServerDeps{
		Config: config.DashboardConfig{
			Address: ":0",
			Auth:    config.DashboardAuthConfig{Mode: config.DashboardAuthNone},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp handlers.ReadinessResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "ready", resp.Status)
}

// --- SSE Endpoint Tests ---

func TestSSEEndpoint_RequiresAuth(t *testing.T) {
	server := NewServer(ServerDeps{
		Config: config.DashboardConfig{
			Address: ":0",
			Auth: config.DashboardAuthConfig{
				Mode: config.DashboardAuthAPIKey,
				APIKeys: []config.DashboardAPIKeyConfig{
					{Key: "test-key", Name: "Test", Roles: []string{"admin"}},
				},
			},
		},
	})

	// Without auth -> 401
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/stream", nil)
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// --- SSE Hub Integration ---

func TestServerSSEHub(t *testing.T) {
	server := NewServer(ServerDeps{
		Config: config.DashboardConfig{
			Address: ":0",
			Auth:    config.DashboardAuthConfig{Mode: config.DashboardAuthNone},
		},
	})

	hub := server.SSEHub()
	require.NotNil(t, hub)
	assert.Equal(t, 0, hub.ClientCount())
}
