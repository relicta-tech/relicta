// Package oidc implements OpenID Connect authorization code flow for Relicta dashboard SSO.
package oidc

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/relicta-tech/relicta/v5/internal/config"
)

// Service handles OIDC provider discovery, authorization code flow, and claim-to-role mapping.
type Service struct {
	provider     *gooidc.Provider
	verifier     *gooidc.IDTokenVerifier
	oauth2Config oauth2.Config
	oidcConfig   config.OIDCConfig

	mu              sync.RWMutex
	states          map[string]stateEntry // CSRF state parameter store
	stateAddCount   uint16
	lastStatePruned time.Time
}

// BrowserCallbackSecure reports whether the configured callback uses HTTPS,
// which controls the Secure attribute on the browser-flow state cookie.
func (s *Service) BrowserCallbackSecure() bool {
	u, err := url.Parse(s.oidcConfig.RedirectURL)
	return err == nil && u.Scheme == "https"
}

func validateIssuerURL(raw string) error {
	issuerURL, err := url.Parse(raw)
	if err != nil || !issuerURL.IsAbs() || issuerURL.Hostname() == "" || issuerURL.User != nil || issuerURL.RawQuery != "" || issuerURL.ForceQuery || issuerURL.Fragment != "" || strings.Contains(raw, "#") {
		return errors.New("oidc: issuer_url must be an absolute URL without credentials, query parameters, or a fragment")
	}

	loopback := strings.EqualFold(issuerURL.Hostname(), "localhost")
	if ip := net.ParseIP(issuerURL.Hostname()); ip != nil && ip.IsLoopback() {
		loopback = true
	}
	secureScheme := strings.EqualFold(issuerURL.Scheme, "https")
	developmentScheme := strings.EqualFold(issuerURL.Scheme, "http") && loopback
	if !secureScheme && !developmentScheme {
		return errors.New("oidc: issuer_url must use HTTPS except for loopback development providers")
	}

	return nil
}

// stateEntry tracks an OIDC state parameter with expiry for cleanup.
type stateEntry struct {
	createdAt time.Time
	nonce     string
	browser   bool
}

// stateMaxAge is how long a state parameter remains valid.
const stateMaxAge = 10 * time.Minute

// maxPendingStates caps the number of concurrent in-flight OIDC login requests.
const maxPendingStates = 10000

// UserInfo holds the extracted user information from an OIDC ID token.
type UserInfo struct {
	// Subject is the unique identifier from the IdP (sub claim).
	Subject string
	// Email is the user's email address.
	Email string
	// Name is the user's display name.
	Name string
	// Roles are the Relicta roles derived from claim mappings.
	Roles []string
}

// NewService creates an OIDC service by performing provider discovery against the issuer URL.
// The context is used for the initial discovery HTTP request.
func NewService(ctx context.Context, cfg config.OIDCConfig) (*Service, error) {
	if cfg.IssuerURL == "" {
		return nil, errors.New("oidc: issuer_url is required")
	}
	if err := validateIssuerURL(cfg.IssuerURL); err != nil {
		return nil, err
	}
	if cfg.ClientID == "" {
		return nil, errors.New("oidc: client_id is required")
	}
	if cfg.ClientSecret == "" {
		return nil, errors.New("oidc: client_secret is required")
	}
	if cfg.RedirectURL == "" {
		return nil, errors.New("oidc: redirect_url is required")
	}
	redirectURL, err := url.Parse(cfg.RedirectURL)
	if err != nil || !redirectURL.IsAbs() || redirectURL.Hostname() == "" || redirectURL.User != nil || redirectURL.Fragment != "" {
		return nil, errors.New("oidc: redirect_url must be an absolute URL without credentials or a fragment")
	}
	loopback := strings.EqualFold(redirectURL.Hostname(), "localhost")
	if ip := net.ParseIP(redirectURL.Hostname()); ip != nil && ip.IsLoopback() {
		loopback = true
	}
	secureScheme := redirectURL.Scheme == "https"
	developmentScheme := redirectURL.Scheme == "http" && loopback
	if !secureScheme && !developmentScheme {
		return nil, errors.New("oidc: redirect_url must use HTTPS except for loopback development URLs")
	}

	cfg.Defaults()

	provider, err := gooidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc: provider discovery failed: %w", err)
	}

	verifier := provider.Verifier(&gooidc.Config{
		ClientID: cfg.ClientID,
	})

	oauth2Cfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.Scopes,
	}

	return &Service{
		provider:     provider,
		verifier:     verifier,
		oauth2Config: oauth2Cfg,
		oidcConfig:   cfg,
		states:       make(map[string]stateEntry),
	}, nil
}

// AuthCodeURL generates an authorization URL with a random state parameter for CSRF protection.
// The state is stored internally and must be validated in the callback via ValidateState.
func (s *Service) AuthCodeURL() (authURL, state string, err error) {
	return s.authCodeURL(false)
}

// AuthCodeURLForBrowser marks the flow for a same-origin one-time-ticket redirect
// to the embedded dashboard rather than a JSON API response.
func (s *Service) AuthCodeURLForBrowser() (authURL, state string, err error) {
	return s.authCodeURL(true)
}

func (s *Service) authCodeURL(browser bool) (authURL, state string, err error) {
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", "", fmt.Errorf("oidc: generate state: %w", err)
	}
	state = base64.RawURLEncoding.EncodeToString(stateBytes)
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", "", fmt.Errorf("oidc: generate nonce: %w", err)
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)

	now := time.Now()
	s.mu.Lock()
	s.stateAddCount++
	// Amortize map cleanup across login requests instead of starting a goroutine
	// that scans the full state map for every login. At capacity, allow a sweep
	// at most once per minute so expired entries can be reclaimed without making
	// repeated rejected requests scan the map continuously.
	if s.stateAddCount%256 == 0 || (len(s.states) >= maxPendingStates && now.Sub(s.lastStatePruned) >= time.Minute) {
		s.cleanExpiredStatesLocked(now)
	}
	if len(s.states) >= maxPendingStates {
		s.mu.Unlock()
		return "", "", errors.New("oidc: too many pending login requests")
	}
	s.states[state] = stateEntry{createdAt: now, nonce: nonce, browser: browser}
	s.mu.Unlock()

	return s.oauth2Config.AuthCodeURL(state, oauth2.SetAuthURLParam("nonce", nonce)), state, nil
}

// ValidateState checks whether the given state parameter was issued by this service
// and has not expired. It is consumed on validation (one-time use).
func (s *Service) ValidateState(state string) bool {
	_, ok := s.ConsumeState(state)
	return ok
}

// ConsumeState validates and consumes a state value, returning its paired OIDC nonce.
// The nonce must be checked against the ID token returned by the code exchange.
func (s *Service) ConsumeState(state string) (string, bool) {
	nonce, _, ok := s.ConsumeStateContext(state)
	return nonce, ok
}

// ConsumeStateContext validates and consumes state, returning its nonce and
// whether the login was initiated by the embedded browser.
func (s *Service) ConsumeStateContext(state string) (string, bool, bool) {
	return s.consumeStateContext(state, "", false)
}

// ConsumeCallbackState validates browser state against its same-site cookie
// before consuming it, preventing a forged callback from invalidating a login.
func (s *Service) ConsumeCallbackState(state, browserCookie string) (string, bool, bool) {
	return s.consumeStateContext(state, browserCookie, true)
}

func (s *Service) consumeStateContext(state, browserCookie string, validateBrowserCookie bool) (string, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.states[state]
	if !ok {
		return "", false, false
	}
	if validateBrowserCookie && entry.browser && subtle.ConstantTimeCompare([]byte(browserCookie), []byte(state)) != 1 {
		return "", true, false
	}
	delete(s.states, state)

	if time.Since(entry.createdAt) >= stateMaxAge {
		return "", false, false
	}
	return entry.nonce, entry.browser, true
}

// Exchange trades an authorization code for tokens, verifies the ID token,
// extracts claims, and maps them to Relicta roles.
func (s *Service) Exchange(ctx context.Context, code, expectedNonce string) (*UserInfo, error) {
	if expectedNonce == "" {
		return nil, errors.New("oidc: expected nonce is required")
	}
	oauth2Token, err := s.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oidc: token exchange failed: %w", err)
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("oidc: no id_token in token response")
	}

	idToken, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("oidc: id_token verification failed: %w", err)
	}
	if idToken.Nonce == "" || idToken.Nonce != expectedNonce {
		return nil, errors.New("oidc: id_token nonce does not match login request")
	}

	// Extract standard claims plus any custom claims into a map.
	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("oidc: extract claims: %w", err)
	}

	info := &UserInfo{
		Subject: idToken.Subject,
		Email:   claimString(claims, "email"),
		Name:    claimString(claims, "name"),
	}

	// Determine display name fallback.
	if info.Name == "" {
		info.Name = info.Email
	}
	if info.Name == "" {
		info.Name = info.Subject
	}

	info.Roles = s.mapClaimsToRoles(claims)

	return info, nil
}

// mapClaimsToRoles applies the configured claim-to-role mappings.
// If no mappings match, the default role is assigned.
func (s *Service) mapClaimsToRoles(claims map[string]any) []string {
	roleSet := make(map[string]struct{})

	for _, mapping := range s.oidcConfig.ClaimMappings {
		claimName := mapping.Claim
		if claimName == "" {
			claimName = s.oidcConfig.GroupsClaim
		}

		claimVal, ok := claims[claimName]
		if !ok {
			continue
		}

		if matchesClaim(claimVal, mapping.Value) {
			roleSet[mapping.Role] = struct{}{}
		}
	}

	if len(roleSet) == 0 {
		return []string{s.oidcConfig.DefaultRole}
	}

	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}
	return roles
}

// matchesClaim checks whether a claim value matches the target.
// Supports string claims and string-slice claims (e.g., groups: ["a", "b"]).
func matchesClaim(claimVal any, target string) bool {
	switch v := claimVal.(type) {
	case string:
		return v == target
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s == target {
				return true
			}
		}
	case []string:
		for _, s := range v {
			if s == target {
				return true
			}
		}
	}
	return false
}

// claimString extracts a string claim value, returning "" if not present or not a string.
func claimString(claims map[string]any, key string) string {
	v, ok := claims[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// cleanExpiredStatesLocked removes state entries older than stateMaxAge.
// The caller must hold s.mu.
func (s *Service) cleanExpiredStatesLocked(now time.Time) {
	s.lastStatePruned = now
	for state, entry := range s.states {
		if now.Sub(entry.createdAt) >= stateMaxAge {
			delete(s.states, state)
		}
	}
}
