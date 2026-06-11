package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

func TestNewServer(t *testing.T) {
	cfg := config.DashboardConfig{
		Address:      ":0", // Random port
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		Auth: config.DashboardAuthConfig{
			Mode: config.DashboardAuthNone,
		},
	}

	server := NewServer(ServerDeps{
		Config:   cfg,
		Frontend: nil,
	})

	if server == nil {
		t.Fatal("NewServer returned nil")
	}
	if server.wsHub == nil {
		t.Error("WebSocket hub should be initialized")
	}
	if server.router == nil {
		t.Error("Router should be initialized")
	}
}

func TestHealthEndpoint(t *testing.T) {
	cfg := config.DashboardConfig{
		Address: ":0",
		Auth: config.DashboardAuthConfig{
			Mode: config.DashboardAuthNone,
		},
	}

	server := NewServer(ServerDeps{
		Config:   cfg,
		Frontend: nil,
	})

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	// Serve the request
	server.router.ServeHTTP(rec, req)

	// Check response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// Parse response body
	var response map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", response["status"])
	}
}

func TestHealthEndpointWithAPIPath(t *testing.T) {
	cfg := config.DashboardConfig{
		Address: ":0",
		Auth: config.DashboardAuthConfig{
			Mode: config.DashboardAuthNone,
		},
	}

	server := NewServer(ServerDeps{
		Config:   cfg,
		Frontend: nil,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestAPIAuthenticationRequired(t *testing.T) {
	cfg := config.DashboardConfig{
		Address: ":0",
		Auth: config.DashboardAuthConfig{
			Mode: config.DashboardAuthAPIKey,
			APIKeys: []config.DashboardAPIKeyConfig{
				{Key: "test-key", Name: "Test", Roles: []string{"admin"}},
			},
		},
	}

	server := NewServer(ServerDeps{
		Config:   cfg,
		Frontend: nil,
	})

	// Request without API key should fail
	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 without API key, got %d", rec.Code)
	}

	// Request with valid API key should succeed
	req = httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
	req.Header.Set("X-API-Key", "test-key")
	rec = httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 with valid API key, got %d", rec.Code)
	}
}

func TestAPIAuthenticationWithBearerToken(t *testing.T) {
	cfg := config.DashboardConfig{
		Address: ":0",
		Auth: config.DashboardAuthConfig{
			Mode: config.DashboardAuthAPIKey,
			APIKeys: []config.DashboardAPIKeyConfig{
				{Key: "bearer-test-key", Name: "Test", Roles: []string{"admin"}},
			},
		},
	}

	server := NewServer(ServerDeps{
		Config:   cfg,
		Frontend: nil,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
	req.Header.Set("Authorization", "Bearer bearer-test-key")
	rec := httptest.NewRecorder()

	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 with Bearer token, got %d", rec.Code)
	}
}

func TestServerShutdown(t *testing.T) {
	cfg := config.DashboardConfig{
		Address: ":0",
		Auth: config.DashboardAuthConfig{
			Mode: config.DashboardAuthNone,
		},
	}

	server := NewServer(ServerDeps{
		Config:   cfg,
		Frontend: nil,
	})

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately to test graceful shutdown
	cancel()

	err := server.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}
}

func TestWebSocketHubClientCount(t *testing.T) {
	cfg := config.DashboardConfig{
		Address: ":0",
		Auth: config.DashboardAuthConfig{
			Mode: config.DashboardAuthNone,
		},
	}

	server := NewServer(ServerDeps{
		Config:   cfg,
		Frontend: nil,
	})

	if server.Hub().ClientCount() != 0 {
		t.Error("Expected 0 clients initially")
	}
}

func TestServerAddress(t *testing.T) {
	cfg := config.DashboardConfig{
		Address: "127.0.0.1:9090",
		Auth: config.DashboardAuthConfig{
			Mode: config.DashboardAuthNone,
		},
	}

	server := NewServer(ServerDeps{
		Config:   cfg,
		Frontend: nil,
	})

	if server.Address() != "127.0.0.1:9090" {
		t.Errorf("Address() = %s, want 127.0.0.1:9090", server.Address())
	}
}

func TestServerEventBroadcaster(t *testing.T) {
	cfg := config.DashboardConfig{
		Address: ":0",
		Auth: config.DashboardAuthConfig{
			Mode: config.DashboardAuthNone,
		},
	}

	server := NewServer(ServerDeps{
		Config:   cfg,
		Frontend: nil,
	})

	broadcaster := server.EventBroadcaster()
	if broadcaster == nil {
		t.Error("EventBroadcaster() returned nil")
	}
}

func TestServerTimeoutDefaults(t *testing.T) {
	// Test with zero timeouts to trigger defaults
	cfg := config.DashboardConfig{
		Address: ":0",
		Auth: config.DashboardAuthConfig{
			Mode: config.DashboardAuthNone,
		},
	}

	server := NewServer(ServerDeps{
		Config:   cfg,
		Frontend: nil,
	})

	if server.httpServer.ReadTimeout != 15*time.Second {
		t.Errorf("ReadTimeout = %v, want 15s", server.httpServer.ReadTimeout)
	}
	if server.httpServer.WriteTimeout != 15*time.Second {
		t.Errorf("WriteTimeout = %v, want 15s", server.httpServer.WriteTimeout)
	}
	if server.httpServer.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v, want 60s", server.httpServer.IdleTimeout)
	}
}

func TestServerCustomTimeouts(t *testing.T) {
	cfg := config.DashboardConfig{
		Address:      ":0",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 45 * time.Second,
		IdleTimeout:  120 * time.Second,
		Auth: config.DashboardAuthConfig{
			Mode: config.DashboardAuthNone,
		},
	}

	server := NewServer(ServerDeps{
		Config:   cfg,
		Frontend: nil,
	})

	if server.httpServer.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout = %v, want 30s", server.httpServer.ReadTimeout)
	}
	if server.httpServer.WriteTimeout != 45*time.Second {
		t.Errorf("WriteTimeout = %v, want 45s", server.httpServer.WriteTimeout)
	}
	if server.httpServer.IdleTimeout != 120*time.Second {
		t.Errorf("IdleTimeout = %v, want 120s", server.httpServer.IdleTimeout)
	}
}

func TestServerWithFrontend(t *testing.T) {
	// Test with embedded frontend FS
	frontendFS := fstest.MapFS{
		"index.html":    {Data: []byte("<html>Test</html>")},
		"assets/app.js": {Data: []byte("console.log('test')")},
		"favicon.svg":   {Data: []byte("<svg></svg>")},
	}

	cfg := config.DashboardConfig{
		Address: ":0",
		Auth: config.DashboardAuthConfig{
			Mode: config.DashboardAuthNone,
		},
	}

	server := NewServer(ServerDeps{
		Config:   cfg,
		Frontend: frontendFS,
	})

	// Test root path serves index.html
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Root path: expected status 200, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Root path: expected text/html content type, got %s", rec.Header().Get("Content-Type"))
	}

	// Test SPA fallback (non-API, non-asset path)
	req = httptest.NewRequest(http.MethodGet, "/some/spa/route", nil)
	rec = httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("SPA fallback: expected status 200, got %d", rec.Code)
	}

	// Test API path does not get SPA fallback
	req = httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil)
	rec = httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("API 404: expected status 404, got %d", rec.Code)
	}
}

func TestAPIEndpoints(t *testing.T) {
	cfg := config.DashboardConfig{
		Address: ":0",
		Auth: config.DashboardAuthConfig{
			Mode: config.DashboardAuthNone,
		},
	}

	server := NewServer(ServerDeps{
		Config:   cfg,
		Frontend: nil,
	})

	// Test various API endpoints return 200 (with no services, they return empty data)
	endpoints := []string{
		"/api/v1/releases",
		"/api/v1/governance/decisions",
		"/api/v1/governance/risk-trends",
		"/api/v1/governance/factors",
		"/api/v1/actors",
		"/api/v1/approvals/pending",
		"/api/v1/audit",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, endpoint, nil)
			rec := httptest.NewRecorder()
			server.router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("GET %s: expected status 200, got %d", endpoint, rec.Code)
			}
		})
	}
}
