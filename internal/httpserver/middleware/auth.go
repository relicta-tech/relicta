// Package middleware provides HTTP middleware for the dashboard.
package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/security/token"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// UserContextKey is the context key for the authenticated user.
	UserContextKey contextKey = "user"
)

// AuthenticatedUser represents an authenticated dashboard user.
type AuthenticatedUser struct {
	// Name is the friendly name of the user/key.
	Name string
	// Roles is the list of roles the user has.
	Roles []string
}

// HasRole checks if the user has a specific role.
func (u *AuthenticatedUser) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// IsAdmin checks if the user has admin role.
func (u *AuthenticatedUser) IsAdmin() bool {
	return u.HasRole(string(config.DashboardRoleAdmin))
}

// CanApprove checks if the user can approve releases.
func (u *AuthenticatedUser) CanApprove() bool {
	return u.HasRole(string(config.DashboardRoleAdmin)) || u.HasRole(string(config.DashboardRoleApprover))
}

// Auth returns authentication middleware based on the auth config.
// tokenSvc is required for session mode; it may be nil for other modes.
func Auth(cfg config.DashboardAuthConfig, tokenSvc *token.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch cfg.Mode {
			case config.DashboardAuthNone, "":
				// No authentication - pass through with an anonymous, READ-ONLY
				// user. Granting admin here let unauthenticated callers
				// approve/reject releases; viewer keeps no-auth usable for
				// local inspection without exposing mutating actions.
				user := &AuthenticatedUser{
					Name:  "anonymous",
					Roles: []string{string(config.DashboardRoleViewer)},
				}
				ctx := context.WithValue(r.Context(), UserContextKey, user)
				next.ServeHTTP(w, r.WithContext(ctx))

			case config.DashboardAuthAPIKey:
				// API key authentication
				user := validateAPIKey(r, cfg.APIKeys)
				if user == nil {
					http.Error(w, "Unauthorized: invalid or missing API key", http.StatusUnauthorized)
					return
				}
				ctx := context.WithValue(r.Context(), UserContextKey, user)
				next.ServeHTTP(w, r.WithContext(ctx))

			case config.DashboardAuthSession:
				user := validateSession(r, tokenSvc)
				if user == nil {
					http.Error(w, "Unauthorized: invalid or expired token", http.StatusUnauthorized)
					return
				}
				ctx := context.WithValue(r.Context(), UserContextKey, user)
				next.ServeHTTP(w, r.WithContext(ctx))

			case config.DashboardAuthOIDC:
				// OIDC mode reuses JWT session validation — tokens are issued after OIDC callback.
				user := validateSession(r, tokenSvc)
				if user == nil {
					http.Error(w, "Unauthorized: invalid or expired token", http.StatusUnauthorized)
					return
				}
				ctx := context.WithValue(r.Context(), UserContextKey, user)
				next.ServeHTTP(w, r.WithContext(ctx))

			default:
				http.Error(w, "Invalid authentication mode", http.StatusInternalServerError)
			}
		})
	}
}

// validateSession validates a JWT Bearer token from the Authorization header.
func validateSession(r *http.Request, tokenSvc *token.Service) *AuthenticatedUser {
	if tokenSvc == nil {
		return nil
	}

	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return nil
	}

	tokenStr := strings.TrimPrefix(auth, "Bearer ")

	claims, err := tokenSvc.Validate(tokenStr)
	if err != nil {
		return nil
	}

	return &AuthenticatedUser{
		Name:  claims.Name,
		Roles: claims.Roles,
	}
}

// validateAPIKey validates the API key from the request.
func validateAPIKey(r *http.Request, keys []config.DashboardAPIKeyConfig) *AuthenticatedUser {
	// Check X-API-Key header first
	apiKey := r.Header.Get("X-API-Key")

	// Fall back to Authorization header (Bearer token)
	if apiKey == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			apiKey = strings.TrimPrefix(auth, "Bearer ")
		}
	}

	// Fall back to query parameter (for WebSocket upgrade requests only).
	// API keys in URLs are logged by proxies and browsers — restrict to upgrades.
	if apiKey == "" && r.Header.Get("Upgrade") == "websocket" {
		apiKey = r.URL.Query().Get("api_key")
	}

	if apiKey == "" {
		return nil
	}

	// Find matching API key using constant-time comparison to prevent timing attacks
	for _, key := range keys {
		if subtle.ConstantTimeCompare([]byte(key.Key), []byte(apiKey)) == 1 {
			roles := key.Roles
			if len(roles) == 0 {
				roles = []string{string(config.DashboardRoleViewer)}
			}
			return &AuthenticatedUser{
				Name:  key.Name,
				Roles: roles,
			}
		}
	}

	return nil
}

// GetUser retrieves the authenticated user from the request context.
func GetUser(r *http.Request) *AuthenticatedUser {
	user, ok := r.Context().Value(UserContextKey).(*AuthenticatedUser)
	if !ok {
		return nil
	}
	return user
}

// RequireRole returns middleware that requires a specific role.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUser(r)
			if user == nil || !user.HasRole(role) {
				http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
