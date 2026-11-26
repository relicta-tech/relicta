package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/relicta-tech/relicta/v5/internal/config"
)

// loginRequest is the request body for POST /api/v1/auth/login.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// tokenResponse is the response body for login and refresh endpoints.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
	TokenType    string `json:"token_type"`
}

// refreshRequest is the request body for POST /api/v1/auth/refresh.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// AuthMode reports the configured dashboard authentication mode so the
// embedded UI can present the matching sign-in method without exposing secrets.
func AuthMode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	ctx := GetContext()
	if ctx == nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Authentication configuration unavailable", nil)
		return
	}
	mode := ctx.AuthConfig.Mode
	if mode == "" {
		mode = config.DashboardAuthNone
	}
	respondJSON(w, http.StatusOK, map[string]string{"mode": string(mode)})
}

// Login handles POST /api/v1/auth/login.
// Validates username/password against configured users and returns a JWT token pair.
func Login(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.TokenService == nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeAuthNotConfigured, "Session authentication not configured", nil)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, ErrCodeInvalidJSON, "Invalid request body", nil)
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, r, http.StatusBadRequest, ErrCodeMissingField, "Username and password are required", nil)
		return
	}

	// Find matching user and validate password.
	user := authenticateUser(req.Username, req.Password, ctx.AuthConfig.Users)
	if user == nil {
		writeError(w, r, http.StatusUnauthorized, ErrCodeInvalidCredentials, "Invalid username or password", nil)
		return
	}

	roles := user.Roles
	if len(roles) == 0 {
		roles = []string{string(config.DashboardRoleViewer)}
	}

	pair, err := ctx.TokenService.Issue(user.Username, roles)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to issue token", nil)
		return
	}

	respondJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresAt:    pair.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		TokenType:    "Bearer",
	})
}

// Refresh handles POST /api/v1/auth/refresh.
// Validates a refresh token and returns a new token pair.
func Refresh(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.TokenService == nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeAuthNotConfigured, "Session authentication not configured", nil)
		return
	}

	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, ErrCodeInvalidJSON, "Invalid request body", nil)
		return
	}

	if req.RefreshToken == "" {
		writeError(w, r, http.StatusBadRequest, ErrCodeMissingField, "Refresh token is required", nil)
		return
	}

	pair, err := ctx.TokenService.Refresh(req.RefreshToken)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, ErrCodeTokenInvalid, "Invalid or expired refresh token", nil)
		return
	}

	respondJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresAt:    pair.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		TokenType:    "Bearer",
	})
}

// Logout handles POST /api/v1/auth/logout.
// It revokes both tokens when the refresh token is supplied in the request body.
func Logout(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.TokenService == nil {
		writeError(w, r, http.StatusInternalServerError, ErrCodeAuthNotConfigured, "Session authentication not configured", nil)
		return
	}

	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		writeError(w, r, http.StatusBadRequest, ErrCodeMissingField, "Missing Authorization header", nil)
		return
	}

	tokenStr := strings.TrimPrefix(auth, "Bearer ")
	var req logoutRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			writeError(w, r, http.StatusBadRequest, ErrCodeInvalidJSON, "Invalid request body", nil)
			return
		}
	}

	var err error
	if req.RefreshToken != "" {
		err = ctx.TokenService.RevokePair(tokenStr, req.RefreshToken)
	} else {
		err = ctx.TokenService.RevokeToken(tokenStr)
	}
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, ErrCodeTokenInvalid, "Invalid token", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// authenticateUser validates credentials against the configured user list.
func authenticateUser(username, password string, users []config.DashboardUserConfig) *config.DashboardUserConfig {
	for i := range users {
		if users[i].Username != username {
			continue
		}
		if err := bcrypt.CompareHashAndPassword([]byte(users[i].PasswordHash), []byte(password)); err != nil {
			return nil
		}
		return &users[i]
	}
	return nil
}
