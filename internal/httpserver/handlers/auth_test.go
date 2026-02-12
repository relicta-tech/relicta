package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/security/token"
)

var testSecret = []byte("test-secret-key-that-is-at-least-32-bytes-long!")

func setupAuthContext(t *testing.T) *token.Service {
	t.Helper()
	svc, err := token.NewService(token.Config{Secret: testSecret})
	require.NoError(t, err)

	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	require.NoError(t, err)

	t.Cleanup(func() { SetContext(nil) })

	SetContext(&Context{
		TokenService: svc,
		AuthConfig: config.DashboardAuthConfig{
			Mode:          config.DashboardAuthSession,
			SessionSecret: string(testSecret),
			Users: []config.DashboardUserConfig{
				{
					Username:     "alice",
					PasswordHash: string(hash),
					Roles:        []string{"admin", "approver"},
				},
				{
					Username:     "bob",
					PasswordHash: string(hash),
					Roles:        nil, // defaults to viewer
				},
			},
		},
	})
	return svc
}

func TestLogin_Success(t *testing.T) {
	setupAuthContext(t)

	body, _ := json.Marshal(loginRequest{Username: "alice", Password: "correct-password"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	Login(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp tokenResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.NotEmpty(t, resp.ExpiresAt)
	assert.Equal(t, "Bearer", resp.TokenType)
}

func TestLogin_DefaultRole(t *testing.T) {
	svc := setupAuthContext(t)

	body, _ := json.Marshal(loginRequest{Username: "bob", Password: "correct-password"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	Login(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp tokenResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	// Validate that the token has the viewer role (default).
	claims, err := svc.Validate(resp.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "bob", claims.Name)
	assert.Equal(t, []string{"viewer"}, claims.Roles)
}

func TestLogin_WrongPassword(t *testing.T) {
	setupAuthContext(t)

	body, _ := json.Marshal(loginRequest{Username: "alice", Password: "wrong-password"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	Login(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestLogin_UnknownUser(t *testing.T) {
	setupAuthContext(t)

	body, _ := json.Marshal(loginRequest{Username: "nobody", Password: "password"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	Login(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestLogin_MissingFields(t *testing.T) {
	setupAuthContext(t)

	body, _ := json.Marshal(loginRequest{Username: "", Password: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	Login(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestLogin_InvalidJSON(t *testing.T) {
	setupAuthContext(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()

	Login(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestLogin_NoTokenService(t *testing.T) {
	t.Cleanup(func() { SetContext(nil) })
	SetContext(&Context{TokenService: nil})

	body, _ := json.Marshal(loginRequest{Username: "alice", Password: "password"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	Login(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRefresh_Success(t *testing.T) {
	svc := setupAuthContext(t)

	// Issue a token pair to get a valid refresh token.
	pair, err := svc.Issue("alice", []string{"admin"})
	require.NoError(t, err)

	body, _ := json.Marshal(refreshRequest{RefreshToken: pair.RefreshToken})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	Refresh(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp tokenResponse
	err = json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEqual(t, pair.AccessToken, resp.AccessToken)
}

func TestRefresh_InvalidToken(t *testing.T) {
	setupAuthContext(t)

	body, _ := json.Marshal(refreshRequest{RefreshToken: "invalid.token.value"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	Refresh(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRefresh_MissingToken(t *testing.T) {
	setupAuthContext(t)

	body, _ := json.Marshal(refreshRequest{RefreshToken: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	Refresh(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestLogout_Success(t *testing.T) {
	svc := setupAuthContext(t)

	pair, err := svc.Issue("alice", []string{"admin"})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rec := httptest.NewRecorder()

	Logout(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Token should now be revoked.
	_, err = svc.Validate(pair.AccessToken)
	assert.Error(t, err)
}

func TestLogout_MissingHeader(t *testing.T) {
	setupAuthContext(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()

	Logout(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestLogout_InvalidToken(t *testing.T) {
	setupAuthContext(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer invalid.token")
	rec := httptest.NewRecorder()

	Logout(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRefresh_NoTokenService(t *testing.T) {
	t.Cleanup(func() { SetContext(nil) })
	SetContext(&Context{TokenService: nil})

	body, _ := json.Marshal(refreshRequest{RefreshToken: "some.token"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	Refresh(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRefresh_InvalidJSON(t *testing.T) {
	setupAuthContext(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader([]byte("{bad")))
	rec := httptest.NewRecorder()

	Refresh(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestLogout_NoTokenService(t *testing.T) {
	t.Cleanup(func() { SetContext(nil) })
	SetContext(&Context{TokenService: nil})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer some.token")
	rec := httptest.NewRecorder()

	Logout(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestLogin_NilContext(t *testing.T) {
	t.Cleanup(func() { SetContext(nil) })
	SetContext(nil)

	body, _ := json.Marshal(loginRequest{Username: "alice", Password: "pass"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	Login(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRefresh_NilContext(t *testing.T) {
	t.Cleanup(func() { SetContext(nil) })
	SetContext(nil)

	body, _ := json.Marshal(refreshRequest{RefreshToken: "some.token"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	Refresh(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestLogout_NilContext(t *testing.T) {
	t.Cleanup(func() { SetContext(nil) })
	SetContext(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer some.token")
	rec := httptest.NewRecorder()

	Logout(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAuthenticateUser(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	require.NoError(t, err)

	users := []config.DashboardUserConfig{
		{Username: "alice", PasswordHash: string(hash), Roles: []string{"admin"}},
	}

	t.Run("valid credentials", func(t *testing.T) {
		user := authenticateUser("alice", "secret", users)
		require.NotNil(t, user)
		assert.Equal(t, "alice", user.Username)
	})

	t.Run("wrong password", func(t *testing.T) {
		user := authenticateUser("alice", "wrong", users)
		assert.Nil(t, user)
	})

	t.Run("unknown user", func(t *testing.T) {
		user := authenticateUser("eve", "secret", users)
		assert.Nil(t, user)
	})
}
