package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/security/token"
)

// testHandler is a simple handler for testing middleware
func testHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
}

// newTestTokenService creates a token.Service for testing.
func newTestTokenService(t *testing.T) *token.Service {
	t.Helper()
	svc, err := token.NewService(token.Config{
		Secret: []byte("test-secret-key-that-is-at-least-32-bytes-long!"),
	})
	require.NoError(t, err)
	return svc
}

// TestAuthenticatedUser_HasRole tests the HasRole method.
func TestAuthenticatedUser_HasRole(t *testing.T) {
	tests := []struct {
		name     string
		user     *AuthenticatedUser
		role     string
		expected bool
	}{
		{
			name:     "has role",
			user:     &AuthenticatedUser{Name: "test", Roles: []string{"admin", "viewer"}},
			role:     "admin",
			expected: true,
		},
		{
			name:     "does not have role",
			user:     &AuthenticatedUser{Name: "test", Roles: []string{"viewer"}},
			role:     "admin",
			expected: false,
		},
		{
			name:     "empty roles",
			user:     &AuthenticatedUser{Name: "test", Roles: []string{}},
			role:     "admin",
			expected: false,
		},
		{
			name:     "nil roles",
			user:     &AuthenticatedUser{Name: "test"},
			role:     "admin",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.user.HasRole(tt.role)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestAuthenticatedUser_IsAdmin tests the IsAdmin method.
func TestAuthenticatedUser_IsAdmin(t *testing.T) {
	tests := []struct {
		name     string
		user     *AuthenticatedUser
		expected bool
	}{
		{
			name:     "is admin",
			user:     &AuthenticatedUser{Name: "admin", Roles: []string{string(config.DashboardRoleAdmin)}},
			expected: true,
		},
		{
			name:     "not admin - viewer",
			user:     &AuthenticatedUser{Name: "viewer", Roles: []string{string(config.DashboardRoleViewer)}},
			expected: false,
		},
		{
			name:     "not admin - approver",
			user:     &AuthenticatedUser{Name: "approver", Roles: []string{string(config.DashboardRoleApprover)}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.user.IsAdmin()
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestAuthenticatedUser_CanApprove tests the CanApprove method.
func TestAuthenticatedUser_CanApprove(t *testing.T) {
	tests := []struct {
		name     string
		user     *AuthenticatedUser
		expected bool
	}{
		{
			name:     "admin can approve",
			user:     &AuthenticatedUser{Name: "admin", Roles: []string{string(config.DashboardRoleAdmin)}},
			expected: true,
		},
		{
			name:     "approver can approve",
			user:     &AuthenticatedUser{Name: "approver", Roles: []string{string(config.DashboardRoleApprover)}},
			expected: true,
		},
		{
			name:     "viewer cannot approve",
			user:     &AuthenticatedUser{Name: "viewer", Roles: []string{string(config.DashboardRoleViewer)}},
			expected: false,
		},
		{
			name:     "multiple roles including approver",
			user:     &AuthenticatedUser{Name: "user", Roles: []string{string(config.DashboardRoleViewer), string(config.DashboardRoleApprover)}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.user.CanApprove()
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestAuth_NoAuth tests the Auth middleware with no authentication.
func TestAuth_NoAuth(t *testing.T) {
	cfg := config.DashboardAuthConfig{Mode: config.DashboardAuthNone}
	handler := Auth(cfg, nil)(testHandler())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestAuth_EmptyMode tests the Auth middleware with empty mode (defaults to no auth).
func TestAuth_EmptyMode(t *testing.T) {
	cfg := config.DashboardAuthConfig{Mode: ""}
	handler := Auth(cfg, nil)(testHandler())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestAuth_APIKey tests the Auth middleware with API key authentication.
func TestAuth_APIKey(t *testing.T) {
	validKey := "test-api-key-12345"
	cfg := config.DashboardAuthConfig{
		Mode: config.DashboardAuthAPIKey,
		APIKeys: []config.DashboardAPIKeyConfig{
			{Key: validKey, Name: "test-key", Roles: []string{string(config.DashboardRoleAdmin)}},
		},
	}
	handler := Auth(cfg, nil)(testHandler())

	tests := []struct {
		name       string
		headerKey  string
		headerVal  string
		queryKey   string
		expectCode int
	}{
		{
			name:       "valid X-API-Key header",
			headerKey:  "X-API-Key",
			headerVal:  validKey,
			expectCode: http.StatusOK,
		},
		{
			name:       "valid Bearer token",
			headerKey:  "Authorization",
			headerVal:  "Bearer " + validKey,
			expectCode: http.StatusOK,
		},
		{
			name:       "valid query parameter (websocket upgrade)",
			queryKey:   validKey,
			headerKey:  "Upgrade",
			headerVal:  "websocket",
			expectCode: http.StatusOK,
		},
		{
			name:       "invalid API key",
			headerKey:  "X-API-Key",
			headerVal:  "invalid-key",
			expectCode: http.StatusUnauthorized,
		},
		{
			name:       "missing API key",
			expectCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/test"
			if tt.queryKey != "" {
				url = "/test?api_key=" + tt.queryKey
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			if tt.headerKey != "" {
				req.Header.Set(tt.headerKey, tt.headerVal)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectCode, rec.Code)
		})
	}
}

// TestAuth_Session_NoToken tests session auth rejects requests without a token.
func TestAuth_Session_NoToken(t *testing.T) {
	svc := newTestTokenService(t)
	cfg := config.DashboardAuthConfig{Mode: config.DashboardAuthSession}
	handler := Auth(cfg, svc)(testHandler())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestAuth_Session_ValidToken tests session auth accepts a valid JWT.
func TestAuth_Session_ValidToken(t *testing.T) {
	svc := newTestTokenService(t)
	cfg := config.DashboardAuthConfig{Mode: config.DashboardAuthSession}

	pair, err := svc.Issue("alice", []string{"admin"})
	require.NoError(t, err)

	var capturedUser *AuthenticatedUser
	handler := Auth(cfg, svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser = GetUser(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, capturedUser)
	assert.Equal(t, "alice", capturedUser.Name)
	assert.Equal(t, []string{"admin"}, capturedUser.Roles)
}

// TestAuth_Session_InvalidToken tests session auth rejects an invalid token.
func TestAuth_Session_InvalidToken(t *testing.T) {
	svc := newTestTokenService(t)
	cfg := config.DashboardAuthConfig{Mode: config.DashboardAuthSession}
	handler := Auth(cfg, svc)(testHandler())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestAuth_Session_NilService tests session auth when token service is nil.
func TestAuth_Session_NilService(t *testing.T) {
	cfg := config.DashboardAuthConfig{Mode: config.DashboardAuthSession}
	handler := Auth(cfg, nil)(testHandler())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer some.token.value")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestAuth_InvalidMode tests that invalid auth mode returns 500.
func TestAuth_InvalidMode(t *testing.T) {
	cfg := config.DashboardAuthConfig{Mode: "invalid"}
	handler := Auth(cfg, nil)(testHandler())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestValidateAPIKey_DefaultRole tests that missing roles default to viewer.
func TestValidateAPIKey_DefaultRole(t *testing.T) {
	keys := []config.DashboardAPIKeyConfig{
		{Key: "test-key", Name: "test", Roles: nil}, // No roles specified
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "test-key")

	user := validateAPIKey(req, keys)

	require.NotNil(t, user)
	assert.Equal(t, "test", user.Name)
	assert.Contains(t, user.Roles, string(config.DashboardRoleViewer))
}

// TestGetUser tests retrieving user from context.
func TestGetUser(t *testing.T) {
	t.Run("user in context", func(t *testing.T) {
		cfg := config.DashboardAuthConfig{Mode: config.DashboardAuthNone}
		var capturedUser *AuthenticatedUser

		handler := Auth(cfg, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedUser = GetUser(r)
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		require.NotNil(t, capturedUser)
		assert.Equal(t, "anonymous", capturedUser.Name)
	})

	t.Run("no user in context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		user := GetUser(req)
		assert.Nil(t, user)
	})
}

// TestRequireRole tests the RequireRole middleware.
func TestRequireRole(t *testing.T) {
	tests := []struct {
		name         string
		requiredRole string
		userRoles    []string
		expectCode   int
	}{
		{
			name:         "user has required role",
			requiredRole: string(config.DashboardRoleAdmin),
			userRoles:    []string{string(config.DashboardRoleAdmin)},
			expectCode:   http.StatusOK,
		},
		{
			name:         "user lacks required role",
			requiredRole: string(config.DashboardRoleAdmin),
			userRoles:    []string{string(config.DashboardRoleViewer)},
			expectCode:   http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authCfg := config.DashboardAuthConfig{
				Mode: config.DashboardAuthAPIKey,
				APIKeys: []config.DashboardAPIKeyConfig{
					{Key: "test-key", Name: "test", Roles: tt.userRoles},
				},
			}

			handler := Auth(authCfg, nil)(RequireRole(tt.requiredRole)(testHandler()))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-API-Key", "test-key")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectCode, rec.Code)
		})
	}
}

// TestRequireRole_NoUser tests RequireRole when no user is authenticated.
func TestRequireRole_NoUser(t *testing.T) {
	handler := RequireRole(string(config.DashboardRoleAdmin))(testHandler())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestSecurityHeaders tests that security headers are set correctly.
func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders()(testHandler())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", rec.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"))
	assert.NotEmpty(t, rec.Header().Get("Permissions-Policy"))
	assert.NotEmpty(t, rec.Header().Get("Content-Security-Policy"))
}

// TestStrictTransportSecurity tests HSTS header setting.
func TestStrictTransportSecurity(t *testing.T) {
	maxAge := 31536000 // 1 year
	handler := StrictTransportSecurity(maxAge)(testHandler())

	t.Run("HTTPS via TLS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.TLS = &tls.ConnectionState{} // Simulate HTTPS
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Header().Get("Strict-Transport-Security"), "max-age=31536000")
	})

	t.Run("HTTPS via X-Forwarded-Proto", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Header().Get("Strict-Transport-Security"), "max-age=31536000")
	})

	t.Run("HTTP - no HSTS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Header().Get("Strict-Transport-Security"))
	})
}

// TestRateLimiter_Allow tests the rate limiter Allow method.
func TestRateLimiter_Allow(t *testing.T) {
	cfg := RateLimiterConfig{
		Rate:     5,
		Burst:    3,
		Interval: time.Second,
	}
	rl := NewRateLimiter(cfg)

	// First request should be allowed
	assert.True(t, rl.Allow("192.168.1.1"))

	// Subsequent requests within burst should be allowed
	assert.True(t, rl.Allow("192.168.1.1"))
	assert.True(t, rl.Allow("192.168.1.1"))

	// Beyond burst should be denied
	assert.False(t, rl.Allow("192.168.1.1"))

	// Different IP should be allowed
	assert.True(t, rl.Allow("192.168.1.2"))
}

// TestRateLimiter_TokenRefill tests that tokens are refilled over time.
func TestRateLimiter_TokenRefill(t *testing.T) {
	cfg := RateLimiterConfig{
		Rate:     100,
		Burst:    2,
		Interval: 100 * time.Millisecond,
	}
	rl := NewRateLimiter(cfg)

	// Exhaust burst
	assert.True(t, rl.Allow("test-ip"))
	assert.True(t, rl.Allow("test-ip"))
	assert.False(t, rl.Allow("test-ip"))

	// Wait for refill
	time.Sleep(150 * time.Millisecond)

	// Should have tokens again
	assert.True(t, rl.Allow("test-ip"))
}

// TestDefaultRateLimiterConfig tests default configuration values.
func TestDefaultRateLimiterConfig(t *testing.T) {
	cfg := DefaultRateLimiterConfig()

	assert.Equal(t, 100, cfg.Rate)
	assert.Equal(t, 20, cfg.Burst)
	assert.Equal(t, time.Minute, cfg.Interval)
}

// TestRateLimit_Middleware tests the rate limiting middleware.
func TestRateLimit_Middleware(t *testing.T) {
	cfg := &RateLimiterConfig{
		Rate:     1,
		Burst:    2,
		Interval: time.Minute,
	}
	handler := RateLimit(cfg)(testHandler())

	// First two requests should be allowed (within burst)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	}

	// Third request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "60", rec.Header().Get("Retry-After"))
}

// TestRateLimit_NilConfig tests that nil config uses defaults.
func TestRateLimit_NilConfig(t *testing.T) {
	handler := RateLimit(nil)(testHandler())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.200:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestLogger tests the logging middleware.
func TestLogger(t *testing.T) {
	handler := Logger()(testHandler())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())
}

// TestLogger_WithErrors tests logging with error status codes.
func TestLogger_WithErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"4xx error", http.StatusBadRequest},
		{"5xx error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			})
			handler := Logger()(errorHandler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.status, rec.Code)
		})
	}
}
