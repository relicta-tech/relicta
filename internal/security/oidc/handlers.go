package oidc

import (
	"net/http"

	"github.com/relicta-tech/relicta/internal/security/token"
)

// Handlers provides HTTP handlers for the OIDC authorization code flow.
type Handlers struct {
	oidcService  *Service
	tokenService *token.Service
}

// NewHandlers creates OIDC HTTP handlers wired to the OIDC and token services.
func NewHandlers(oidcSvc *Service, tokenSvc *token.Service) *Handlers {
	return &Handlers{
		oidcService:  oidcSvc,
		tokenService: tokenSvc,
	}
}

// LoginRedirect handles GET /api/v1/auth/oidc/login.
// It generates a CSRF state parameter and redirects the user to the IdP authorization endpoint.
func (h *Handlers) LoginRedirect(w http.ResponseWriter, r *http.Request) {
	authURL, _, err := h.oidcService.AuthCodeURL()
	if err != nil {
		http.Error(w, "Failed to generate authorization URL", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

// Callback handles GET /api/v1/auth/oidc/callback.
// It validates the state parameter, exchanges the authorization code for tokens,
// verifies the ID token, maps claims to roles, and issues a local JWT session.
func (h *Handlers) Callback(w http.ResponseWriter, r *http.Request) {
	// Check for error response from IdP.
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		desc := r.URL.Query().Get("error_description")
		if desc == "" {
			desc = errParam
		}
		http.Error(w, "IdP error: "+desc, http.StatusBadRequest)
		return
	}

	// Validate state parameter for CSRF protection.
	state := r.URL.Query().Get("state")
	if state == "" || !h.oidcService.ValidateState(state) {
		http.Error(w, "Invalid or expired state parameter", http.StatusBadRequest)
		return
	}

	// Exchange authorization code for tokens.
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	userInfo, err := h.oidcService.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "Token exchange failed: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Issue a local JWT session token.
	pair, err := h.tokenService.Issue(userInfo.Name, userInfo.Roles)
	if err != nil {
		http.Error(w, "Failed to issue session token", http.StatusInternalServerError)
		return
	}

	// Return the token pair as JSON.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"access_token":"` + pair.AccessToken +
		`","refresh_token":"` + pair.RefreshToken +
		`","expires_at":"` + pair.ExpiresAt.Format("2006-01-02T15:04:05Z07:00") +
		`","token_type":"Bearer"}`))
}
