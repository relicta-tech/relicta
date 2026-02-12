package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/relicta-tech/relicta/internal/config"
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

// Login handles POST /api/v1/auth/login.
// Validates username/password against configured users and returns a JWT token pair.
func Login(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.TokenService == nil {
		respondError(w, http.StatusInternalServerError, "Session authentication not configured", "")
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body", "")
		return
	}

	if req.Username == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "Username and password are required", "")
		return
	}

	// Find matching user and validate password.
	user := authenticateUser(req.Username, req.Password, ctx.AuthConfig.Users)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Invalid username or password", "")
		return
	}

	roles := user.Roles
	if len(roles) == 0 {
		roles = []string{string(config.DashboardRoleViewer)}
	}

	pair, err := ctx.TokenService.Issue(user.Username, roles)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to issue token", "")
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
		respondError(w, http.StatusInternalServerError, "Session authentication not configured", "")
		return
	}

	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body", "")
		return
	}

	if req.RefreshToken == "" {
		respondError(w, http.StatusBadRequest, "Refresh token is required", "")
		return
	}

	pair, err := ctx.TokenService.Refresh(req.RefreshToken)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Invalid or expired refresh token", "")
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
// Revokes the access token from the Authorization header.
func Logout(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.TokenService == nil {
		respondError(w, http.StatusInternalServerError, "Session authentication not configured", "")
		return
	}

	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		respondError(w, http.StatusBadRequest, "Missing Authorization header", "")
		return
	}

	tokenStr := strings.TrimPrefix(auth, "Bearer ")

	if err := ctx.TokenService.RevokeToken(tokenStr); err != nil {
		respondError(w, http.StatusUnauthorized, "Invalid token", "")
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
