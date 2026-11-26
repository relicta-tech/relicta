package oidc

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/relicta-tech/relicta/v5/internal/security/token"
)

// Handlers provides HTTP handlers for the OIDC authorization code flow.
type Handlers struct {
	oidcService  *Service
	tokenService *token.Service
	ticketsMu    sync.Mutex
	tickets      map[string]loginTicket
}

type loginTicket struct {
	pair      token.TokenPair
	name      string
	createdAt time.Time
}

const (
	loginTicketTTL  = 30 * time.Second
	maxLoginTickets = 10000
)

// NewHandlers creates OIDC HTTP handlers wired to the OIDC and token services.
func NewHandlers(oidcSvc *Service, tokenSvc *token.Service) *Handlers {
	return &Handlers{
		oidcService:  oidcSvc,
		tokenService: tokenSvc,
		tickets:      make(map[string]loginTicket),
	}
}

// LoginRedirect handles GET /api/v1/auth/oidc/login.
// It generates a CSRF state parameter and redirects the user to the IdP authorization endpoint.
func (h *Handlers) LoginRedirect(w http.ResponseWriter, r *http.Request) {
	var authURL string
	var state string
	var err error
	switch r.URL.Query().Get("response_mode") {
	case "":
		authURL, _, err = h.oidcService.AuthCodeURL()
	case "fragment":
		authURL, state, err = h.oidcService.AuthCodeURLForBrowser()
		if err == nil {
			http.SetCookie(w, &http.Cookie{
				Name: "relicta_oidc_state", Value: state,
				Path: "/api/v1/auth/oidc/callback", MaxAge: int(stateMaxAge.Seconds()),
				HttpOnly: true, Secure: h.oidcService.BrowserCallbackSecure(), SameSite: http.SameSiteLaxMode,
			})
		}
	default:
		http.Error(w, "Unsupported OIDC response mode", http.StatusBadRequest)
		return
	}
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
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Referrer-Policy", "no-referrer")

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
	stateCookie := ""
	if cookie, err := r.Cookie("relicta_oidc_state"); err == nil {
		stateCookie = cookie.Value
	}
	nonce, browserFlow, ok := h.oidcService.ConsumeCallbackState(state, stateCookie)
	if state == "" || !ok {
		http.Error(w, "Invalid or expired state parameter", http.StatusBadRequest)
		return
	}
	if browserFlow {
		http.SetCookie(w, &http.Cookie{
			Name: "relicta_oidc_state", Value: "", Path: "/api/v1/auth/oidc/callback",
			MaxAge: -1, HttpOnly: true, Secure: h.oidcService.BrowserCallbackSecure(), SameSite: http.SameSiteLaxMode,
		})
	}

	// Exchange authorization code for tokens.
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	userInfo, err := h.oidcService.Exchange(r.Context(), code, nonce)
	if err != nil {
		slog.Error("OIDC token exchange failed", "error", err)
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	// Issue a local JWT session token.
	pair, err := h.tokenService.Issue(userInfo.Name, userInfo.Roles)
	if err != nil {
		http.Error(w, "Failed to issue session token", http.StatusInternalServerError)
		return
	}

	if browserFlow {
		ticket, err := h.createLoginTicket(*pair, userInfo.Name)
		if err != nil {
			// Do not leave an unreturnable session valid if the bounded ticket
			// store is full or secure randomness is unavailable.
			_ = h.tokenService.RevokeToken(pair.AccessToken)
			_ = h.tokenService.RevokeToken(pair.RefreshToken)
			http.Error(w, "Could not complete sign-in", http.StatusServiceUnavailable)
			return
		}
		// The opaque ticket is short-lived, one-use, and placed in the URL
		// fragment so it is never sent in an HTTP request or Referer header.
		http.Redirect(w, r, "/login#oidc_ticket="+ticket, http.StatusSeeOther)
		return
	}

	// Return the token pair as JSON for API clients.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_at":    pair.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		"token_type":    "Bearer",
	})
}

type exchangeRequest struct {
	Ticket string `json:"ticket"`
}

// ExchangeTicket consumes a short-lived browser login ticket and returns its
// session token pair. The ticket itself is never a dashboard credential.
func (h *Handlers) ExchangeTicket(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	var req exchangeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil || req.Ticket == "" {
		http.Error(w, "Invalid login ticket request", http.StatusBadRequest)
		return
	}

	h.ticketsMu.Lock()
	ticket, ok := h.tickets[req.Ticket]
	delete(h.tickets, req.Ticket)
	h.ticketsMu.Unlock()
	if !ok || time.Since(ticket.createdAt) >= loginTicketTTL {
		http.Error(w, "Login ticket is invalid or expired", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"access_token":  ticket.pair.AccessToken,
		"refresh_token": ticket.pair.RefreshToken,
		"expires_at":    ticket.pair.ExpiresAt.Format(time.RFC3339),
		"token_type":    "Bearer",
		"user_name":     ticket.name,
	})
}

func (h *Handlers) createLoginTicket(pair token.TokenPair, name string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	key := base64.RawURLEncoding.EncodeToString(raw)
	now := time.Now()
	h.ticketsMu.Lock()
	defer h.ticketsMu.Unlock()
	for existing, ticket := range h.tickets {
		if now.Sub(ticket.createdAt) >= loginTicketTTL {
			delete(h.tickets, existing)
		}
	}
	if len(h.tickets) >= maxLoginTickets {
		return "", http.ErrHandlerTimeout
	}
	h.tickets[key] = loginTicket{pair: pair, name: name, createdAt: now}
	return key, nil
}
