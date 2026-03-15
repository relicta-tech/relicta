// Package oidc implements OpenID Connect authorization code flow for Relicta dashboard SSO.
package oidc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/relicta-tech/relicta/internal/config"
)

// Service handles OIDC provider discovery, authorization code flow, and claim-to-role mapping.
type Service struct {
	provider     *gooidc.Provider
	verifier     *gooidc.IDTokenVerifier
	oauth2Config oauth2.Config
	oidcConfig   config.OIDCConfig

	mu     sync.RWMutex
	states map[string]stateEntry // CSRF state parameter store
}

// stateEntry tracks an OIDC state parameter with expiry for cleanup.
type stateEntry struct {
	createdAt time.Time
}

// stateMaxAge is how long a state parameter remains valid.
const stateMaxAge = 10 * time.Minute

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
	if cfg.ClientID == "" {
		return nil, errors.New("oidc: client_id is required")
	}
	if cfg.ClientSecret == "" {
		return nil, errors.New("oidc: client_secret is required")
	}
	if cfg.RedirectURL == "" {
		return nil, errors.New("oidc: redirect_url is required")
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
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", "", fmt.Errorf("oidc: generate state: %w", err)
	}
	state = base64.URLEncoding.EncodeToString(stateBytes)

	s.mu.Lock()
	s.states[state] = stateEntry{createdAt: time.Now()}
	s.mu.Unlock()

	// Trigger background cleanup of expired states.
	go s.cleanExpiredStates()

	return s.oauth2Config.AuthCodeURL(state), state, nil
}

// ValidateState checks whether the given state parameter was issued by this service
// and has not expired. It is consumed on validation (one-time use).
func (s *Service) ValidateState(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.states[state]
	if !ok {
		return false
	}
	delete(s.states, state)

	return time.Since(entry.createdAt) < stateMaxAge
}

// Exchange trades an authorization code for tokens, verifies the ID token,
// extracts claims, and maps them to Relicta roles.
func (s *Service) Exchange(ctx context.Context, code string) (*UserInfo, error) {
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

// cleanExpiredStates removes state entries older than stateMaxAge.
func (s *Service) cleanExpiredStates() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for state, entry := range s.states {
		if now.Sub(entry.createdAt) >= stateMaxAge {
			delete(s.states, state)
		}
	}
}
