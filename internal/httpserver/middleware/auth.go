// Package middleware provides HTTP middleware for the dashboard.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/relicta-tech/relicta/internal/config"
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
func Auth(cfg config.DashboardAuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch cfg.Mode {
			case config.DashboardAuthNone:
				// No authentication - pass through with anonymous user
				user := &AuthenticatedUser{
					Name:  "anonymous",
					Roles: []string{string(config.DashboardRoleAdmin)}, // Full access when auth disabled
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
				// Session authentication - TODO: implement session validation
				http.Error(w, "Session authentication not yet implemented", http.StatusNotImplemented)

			default:
				http.Error(w, "Invalid authentication mode", http.StatusInternalServerError)
			}
		})
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

	// Fall back to query parameter (for WebSocket connections)
	if apiKey == "" {
		apiKey = r.URL.Query().Get("api_key")
	}

	if apiKey == "" {
		return nil
	}

	// Find matching API key
	for _, key := range keys {
		if key.Key == apiKey {
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
