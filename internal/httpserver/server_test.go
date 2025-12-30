package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/config"
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
