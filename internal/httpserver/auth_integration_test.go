package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/httpserver/handlers"
)

// tokenResponse mirrors handlers.tokenResponse for test decoding.
type authTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
	TokenType    string `json:"token_type"`
}

func newSessionServer(t *testing.T) *Server {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
	require.NoError(t, err)

	srv := NewServer(ServerDeps{
		Config: config.DashboardConfig{
			Address: ":0",
			Auth: config.DashboardAuthConfig{
				Mode:          config.DashboardAuthSession,
				SessionSecret: "integration-test-secret-that-is-at-least-32-bytes-long!!",
				SessionMaxAge: 1 * time.Hour,
				Users: []config.DashboardUserConfig{
					{
						Username:     "admin",
						PasswordHash: string(hash),
						Roles:        []string{"admin"},
					},
					{
						Username:     "viewer",
						PasswordHash: string(hash),
						Roles:        []string{"viewer"},
					},
				},
			},
		},
	})

	t.Cleanup(func() { handlers.SetContext(nil) })

	return srv
}

func TestIntegration_LoginFlow(t *testing.T) {
	srv := newSessionServer(t)

	// Step 1: Login with valid credentials.
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "test-password",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var loginResp authTokenResponse
	err := json.NewDecoder(rec.Body).Decode(&loginResp)
	require.NoError(t, err)
	assert.NotEmpty(t, loginResp.AccessToken)
	assert.NotEmpty(t, loginResp.RefreshToken)
	assert.Equal(t, "Bearer", loginResp.TokenType)

	// Step 2: Access a protected endpoint with the token.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	rec = httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Step 3: Access without token should fail.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
	rec = httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestIntegration_RefreshFlow(t *testing.T) {
	srv := newSessionServer(t)

	// Login.
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "test-password",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var loginResp authTokenResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&loginResp))

	// Refresh the token.
	refreshBody, _ := json.Marshal(map[string]string{
		"refresh_token": loginResp.RefreshToken,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(refreshBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var refreshResp authTokenResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&refreshResp))

	assert.NotEmpty(t, refreshResp.AccessToken)
	assert.NotEqual(t, loginResp.AccessToken, refreshResp.AccessToken)
	assert.NotEqual(t, loginResp.RefreshToken, refreshResp.RefreshToken)

	// New token should work.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
	req.Header.Set("Authorization", "Bearer "+refreshResp.AccessToken)
	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Old refresh token should be revoked (can't refresh again).
	refreshBody, _ = json.Marshal(map[string]string{
		"refresh_token": loginResp.RefreshToken,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(refreshBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestIntegration_LogoutFlow(t *testing.T) {
	srv := newSessionServer(t)

	// Login.
	loginBody, _ := json.Marshal(map[string]string{
		"username": "viewer",
		"password": "test-password",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var loginResp authTokenResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&loginResp))

	// Access works before logout.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Logout.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Access fails after logout (token revoked).
	req = httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestIntegration_LoginInvalidCredentials(t *testing.T) {
	srv := newSessionServer(t)

	body, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "wrong-password",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestIntegration_HealthUnauthenticated(t *testing.T) {
	srv := newSessionServer(t)

	// Health endpoint should work without auth.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIntegration_AuthEndpointsUnauthenticated(t *testing.T) {
	srv := newSessionServer(t)

	// Auth endpoints should be accessible without a token.
	body, _ := json.Marshal(map[string]string{
		"refresh_token": "invalid",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	// Should get 401 (invalid token), not 401 from auth middleware.
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestIntegration_RevokedTokenRejected(t *testing.T) {
	srv := newSessionServer(t)

	// Login.
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "test-password",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var loginResp authTokenResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&loginResp))

	// Revoke the token directly via service.
	claims, err := srv.tokenService.Validate(loginResp.AccessToken)
	require.NoError(t, err)
	srv.tokenService.Revoke(claims.ID, claims.ExpiresAt.Time)

	// Access should fail with revoked token.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestIntegration_RoleMappingFromLogin(t *testing.T) {
	srv := newSessionServer(t)

	// Login as admin.
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "test-password",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var loginResp authTokenResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&loginResp))

	// Validate claims have the correct role from config.
	claims, err := srv.tokenService.Validate(loginResp.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "admin", claims.Name)
	assert.Equal(t, []string{"admin"}, claims.Roles)
}
