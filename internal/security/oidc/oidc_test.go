package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/security/token"
)

// mockOIDCProvider sets up a httptest.Server that implements the OIDC discovery,
// JWKS, token, and authorization endpoints needed for integration tests.
type mockOIDCProvider struct {
	server     *httptest.Server
	privateKey *rsa.PrivateKey
	keyID      string

	// tokenClaims lets tests override the claims returned in the ID token.
	tokenClaims map[string]any

	// tokenError makes the token endpoint return an error.
	tokenError bool
}

func newMockOIDCProvider(t *testing.T) *mockOIDCProvider {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	m := &mockOIDCProvider{
		privateKey: privateKey,
		keyID:      "test-key-1",
		tokenClaims: map[string]any{
			"sub":    "user-123",
			"email":  "alice@example.com",
			"name":   "Alice",
			"groups": []string{"relicta-admins", "developers"},
		},
	}

	mux := http.NewServeMux()
	// We need a reference to the server URL in handlers, so create the server first
	// then register routes that reference it.
	m.server = httptest.NewServer(mux)

	mux.HandleFunc("/.well-known/openid-configuration", m.handleDiscovery)
	mux.HandleFunc("/keys", m.handleJWKS)
	mux.HandleFunc("/token", m.handleToken)
	mux.HandleFunc("/authorize", m.handleAuthorize)

	return m
}

func (m *mockOIDCProvider) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	disc := map[string]any{
		"issuer":                                m.server.URL,
		"authorization_endpoint":                m.server.URL + "/authorize",
		"token_endpoint":                        m.server.URL + "/token",
		"jwks_uri":                              m.server.URL + "/keys",
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"subject_types_supported":               []string{"public"},
		"response_types_supported":              []string{"code"},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(disc)
}

func (m *mockOIDCProvider) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	jwk := jose.JSONWebKey{
		Key:       &m.privateKey.PublicKey,
		KeyID:     m.keyID,
		Algorithm: "RS256",
		Use:       "sig",
	}
	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jwks)
}

func (m *mockOIDCProvider) handleToken(w http.ResponseWriter, r *http.Request) {
	if m.tokenError {
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
		return
	}

	// Build ID token JWT with the configured claims.
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": m.server.URL,
		"aud": "test-client-id",
		"exp": now.Add(time.Hour).Unix(),
		"iat": now.Unix(),
	}
	for k, v := range m.tokenClaims {
		claims[k] = v
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = m.keyID
	idToken, err := tok.SignedString(m.privateKey)
	if err != nil {
		http.Error(w, fmt.Sprintf("sign id_token: %v", err), http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"access_token": "mock-access-token",
		"token_type":   "Bearer",
		"expires_in":   3600,
		"id_token":     idToken,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (m *mockOIDCProvider) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	// In a real flow, the IdP would show a login page. Here we just redirect back
	// with a code and the state parameter.
	state := r.URL.Query().Get("state")
	redirectURI := r.URL.Query().Get("redirect_uri")
	http.Redirect(w, r, redirectURI+"?code=mock-auth-code&state="+state, http.StatusFound)
}

func (m *mockOIDCProvider) close() {
	m.server.Close()
}

func (m *mockOIDCProvider) issuerURL() string {
	return m.server.URL
}

// newTestOIDCConfig returns an OIDCConfig pointed at the mock provider.
func newTestOIDCConfig(mock *mockOIDCProvider) config.OIDCConfig {
	cfg := config.OIDCConfig{
		IssuerURL:    mock.issuerURL(),
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  mock.issuerURL() + "/callback",
		Scopes:       []string{"openid", "profile", "email"},
		GroupsClaim:  "groups",
		DefaultRole:  "viewer",
		ClaimMappings: []config.OIDCClaimMapping{
			{Claim: "groups", Value: "relicta-admins", Role: "admin"},
			{Claim: "groups", Value: "relicta-approvers", Role: "approver"},
		},
	}
	cfg.Defaults()
	return cfg
}

// newTestTokenService creates a token.Service for testing.
func newTestTokenService(t *testing.T) *token.Service {
	t.Helper()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatalf("generate secret: %v", err)
	}
	svc, err := token.NewService(token.Config{
		Secret: secret,
	})
	if err != nil {
		t.Fatalf("create token service: %v", err)
	}
	return svc
}

// --- Service Tests ---

func TestNewService_ProviderDiscovery(t *testing.T) {
	mock := newMockOIDCProvider(t)
	defer mock.close()

	cfg := newTestOIDCConfig(mock)
	svc, err := NewService(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if svc == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestNewService_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*config.OIDCConfig)
		wantErr string
	}{
		{
			name:    "missing issuer_url",
			modify:  func(c *config.OIDCConfig) { c.IssuerURL = "" },
			wantErr: "issuer_url is required",
		},
		{
			name:    "missing client_id",
			modify:  func(c *config.OIDCConfig) { c.ClientID = "" },
			wantErr: "client_id is required",
		},
		{
			name:    "missing client_secret",
			modify:  func(c *config.OIDCConfig) { c.ClientSecret = "" },
			wantErr: "client_secret is required",
		},
		{
			name:    "missing redirect_url",
			modify:  func(c *config.OIDCConfig) { c.RedirectURL = "" },
			wantErr: "redirect_url is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.OIDCConfig{
				IssuerURL:    "https://example.com",
				ClientID:     "id",
				ClientSecret: "secret",
				RedirectURL:  "https://example.com/callback",
			}
			tt.modify(&cfg)
			_, err := NewService(context.Background(), cfg)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got := err.Error(); !contains(got, tt.wantErr) {
				t.Errorf("error = %q, want substring %q", got, tt.wantErr)
			}
		})
	}
}

func TestNewService_InvalidIssuerURL(t *testing.T) {
	cfg := config.OIDCConfig{
		IssuerURL:    "https://invalid.example.test:1",
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURL:  "https://example.com/callback",
	}
	cfg.Defaults()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := NewService(ctx, cfg)
	if err == nil {
		t.Fatal("expected error for unreachable issuer")
	}
}

func TestAuthCodeURL_GeneratesUniqueState(t *testing.T) {
	mock := newMockOIDCProvider(t)
	defer mock.close()

	svc, err := NewService(context.Background(), newTestOIDCConfig(mock))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	url1, state1, err := svc.AuthCodeURL()
	if err != nil {
		t.Fatalf("AuthCodeURL: %v", err)
	}
	url2, state2, err := svc.AuthCodeURL()
	if err != nil {
		t.Fatalf("AuthCodeURL: %v", err)
	}

	if state1 == state2 {
		t.Error("expected unique state parameters, got identical values")
	}
	if url1 == "" || url2 == "" {
		t.Error("expected non-empty auth URLs")
	}
}

func TestValidateState_Success(t *testing.T) {
	mock := newMockOIDCProvider(t)
	defer mock.close()

	svc, err := NewService(context.Background(), newTestOIDCConfig(mock))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, state, _ := svc.AuthCodeURL()

	if !svc.ValidateState(state) {
		t.Error("ValidateState() = false for valid state")
	}

	// Second use should fail (one-time consumption).
	if svc.ValidateState(state) {
		t.Error("ValidateState() = true for already-consumed state")
	}
}

func TestValidateState_InvalidState(t *testing.T) {
	mock := newMockOIDCProvider(t)
	defer mock.close()

	svc, err := NewService(context.Background(), newTestOIDCConfig(mock))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if svc.ValidateState("bogus-state") {
		t.Error("ValidateState() = true for unknown state")
	}
}

func TestExchange_Success(t *testing.T) {
	mock := newMockOIDCProvider(t)
	defer mock.close()

	svc, err := NewService(context.Background(), newTestOIDCConfig(mock))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	info, err := svc.Exchange(context.Background(), "valid-code")
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}

	if info.Subject != "user-123" {
		t.Errorf("Subject = %q, want %q", info.Subject, "user-123")
	}
	if info.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", info.Email, "alice@example.com")
	}
	if info.Name != "Alice" {
		t.Errorf("Name = %q, want %q", info.Name, "Alice")
	}
}

func TestExchange_TokenEndpointError(t *testing.T) {
	mock := newMockOIDCProvider(t)
	defer mock.close()
	mock.tokenError = true

	svc, err := NewService(context.Background(), newTestOIDCConfig(mock))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = svc.Exchange(context.Background(), "any-code")
	if err == nil {
		t.Fatal("expected error from token endpoint")
	}
}

// --- Claim-to-Role Mapping Tests ---

func TestMapClaimsToRoles(t *testing.T) {
	tests := []struct {
		name      string
		claims    map[string]any
		mappings  []config.OIDCClaimMapping
		wantRoles []string
	}{
		{
			name: "admin via groups claim",
			claims: map[string]any{
				"groups": []any{"relicta-admins", "developers"},
			},
			mappings: []config.OIDCClaimMapping{
				{Claim: "groups", Value: "relicta-admins", Role: "admin"},
			},
			wantRoles: []string{"admin"},
		},
		{
			name: "multiple role matches",
			claims: map[string]any{
				"groups": []any{"relicta-admins", "relicta-approvers"},
			},
			mappings: []config.OIDCClaimMapping{
				{Claim: "groups", Value: "relicta-admins", Role: "admin"},
				{Claim: "groups", Value: "relicta-approvers", Role: "approver"},
			},
			wantRoles: []string{"admin", "approver"},
		},
		{
			name: "no matching claims falls back to default",
			claims: map[string]any{
				"groups": []any{"unrelated-group"},
			},
			mappings: []config.OIDCClaimMapping{
				{Claim: "groups", Value: "relicta-admins", Role: "admin"},
			},
			wantRoles: []string{"viewer"},
		},
		{
			name:      "no claim mappings configured falls back to default",
			claims:    map[string]any{"groups": []any{"anything"}},
			mappings:  nil,
			wantRoles: []string{"viewer"},
		},
		{
			name: "string claim (email mapping)",
			claims: map[string]any{
				"email": "admin@example.com",
			},
			mappings: []config.OIDCClaimMapping{
				{Claim: "email", Value: "admin@example.com", Role: "admin"},
			},
			wantRoles: []string{"admin"},
		},
		{
			name: "empty claim name defaults to groups_claim",
			claims: map[string]any{
				"groups": []any{"relicta-admins"},
			},
			mappings: []config.OIDCClaimMapping{
				{Claim: "", Value: "relicta-admins", Role: "admin"},
			},
			wantRoles: []string{"admin"},
		},
		{
			name: "missing claim in token",
			claims: map[string]any{
				"email": "user@example.com",
			},
			mappings: []config.OIDCClaimMapping{
				{Claim: "groups", Value: "relicta-admins", Role: "admin"},
			},
			wantRoles: []string{"viewer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &Service{
				oidcConfig: config.OIDCConfig{
					GroupsClaim:   "groups",
					DefaultRole:   "viewer",
					ClaimMappings: tt.mappings,
				},
			}

			roles := svc.mapClaimsToRoles(tt.claims)

			if !equalStringSliceUnordered(roles, tt.wantRoles) {
				t.Errorf("mapClaimsToRoles() = %v, want %v", roles, tt.wantRoles)
			}
		})
	}
}

// --- Handler Tests ---

func TestLoginRedirect_Handler(t *testing.T) {
	mock := newMockOIDCProvider(t)
	defer mock.close()

	svc, err := NewService(context.Background(), newTestOIDCConfig(mock))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	tokenSvc := newTestTokenService(t)
	h := NewHandlers(svc, tokenSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/login", nil)
	rr := httptest.NewRecorder()

	h.LoginRedirect(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusFound)
	}

	loc := rr.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header in redirect")
	}
	if !contains(loc, "/authorize") {
		t.Errorf("Location = %q, expected to contain /authorize", loc)
	}
	if !contains(loc, "state=") {
		t.Errorf("Location = %q, expected to contain state parameter", loc)
	}
}

func TestCallback_Handler_Success(t *testing.T) {
	mock := newMockOIDCProvider(t)
	defer mock.close()

	cfg := newTestOIDCConfig(mock)
	svc, err := NewService(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	tokenSvc := newTestTokenService(t)
	h := NewHandlers(svc, tokenSvc)

	// Generate a valid state.
	_, state, _ := svc.AuthCodeURL()

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oidc/callback?code=valid-code&state="+state, nil)
	rr := httptest.NewRecorder()

	h.Callback(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["access_token"] == "" {
		t.Error("expected non-empty access_token")
	}
	if resp["refresh_token"] == "" {
		t.Error("expected non-empty refresh_token")
	}
	if resp["token_type"] != "Bearer" {
		t.Errorf("token_type = %q, want Bearer", resp["token_type"])
	}

	// Validate the issued access token contains the right roles.
	claims, err := tokenSvc.Validate(resp["access_token"])
	if err != nil {
		t.Fatalf("validate issued token: %v", err)
	}
	if claims.Name != "Alice" {
		t.Errorf("token name = %q, want Alice", claims.Name)
	}
	// Default mock claims include "relicta-admins" which maps to admin.
	if !containsStr(claims.Roles, "admin") {
		t.Errorf("roles = %v, expected to contain admin", claims.Roles)
	}
}

func TestCallback_Handler_InvalidState(t *testing.T) {
	mock := newMockOIDCProvider(t)
	defer mock.close()

	svc, err := NewService(context.Background(), newTestOIDCConfig(mock))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	h := NewHandlers(svc, newTestTokenService(t))

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oidc/callback?code=valid-code&state=invalid-state", nil)
	rr := httptest.NewRecorder()

	h.Callback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCallback_Handler_MissingState(t *testing.T) {
	mock := newMockOIDCProvider(t)
	defer mock.close()

	svc, err := NewService(context.Background(), newTestOIDCConfig(mock))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	h := NewHandlers(svc, newTestTokenService(t))

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oidc/callback?code=valid-code", nil)
	rr := httptest.NewRecorder()

	h.Callback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCallback_Handler_MissingCode(t *testing.T) {
	mock := newMockOIDCProvider(t)
	defer mock.close()

	svc, err := NewService(context.Background(), newTestOIDCConfig(mock))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	h := NewHandlers(svc, newTestTokenService(t))

	_, state, _ := svc.AuthCodeURL()

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oidc/callback?state="+state, nil)
	rr := httptest.NewRecorder()

	h.Callback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCallback_Handler_IdPError(t *testing.T) {
	mock := newMockOIDCProvider(t)
	defer mock.close()

	svc, err := NewService(context.Background(), newTestOIDCConfig(mock))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	h := NewHandlers(svc, newTestTokenService(t))

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oidc/callback?error=access_denied&error_description=User+denied+access", nil)
	rr := httptest.NewRecorder()

	h.Callback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if !contains(rr.Body.String(), "User denied access") {
		t.Errorf("body = %q, expected to contain error description", rr.Body.String())
	}
}

func TestCallback_Handler_TokenExchangeFailure(t *testing.T) {
	mock := newMockOIDCProvider(t)
	defer mock.close()
	mock.tokenError = true

	svc, err := NewService(context.Background(), newTestOIDCConfig(mock))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	h := NewHandlers(svc, newTestTokenService(t))

	_, state, _ := svc.AuthCodeURL()

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oidc/callback?code=bad-code&state="+state, nil)
	rr := httptest.NewRecorder()

	h.Callback(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestCallback_RoleMappingIntegration(t *testing.T) {
	// Test that the full flow produces correct roles in the JWT.
	mock := newMockOIDCProvider(t)
	defer mock.close()

	// User is in relicta-approvers but not relicta-admins.
	mock.tokenClaims = map[string]any{
		"sub":    "user-456",
		"email":  "bob@example.com",
		"name":   "Bob",
		"groups": []string{"relicta-approvers", "developers"},
	}

	cfg := newTestOIDCConfig(mock)
	svc, err := NewService(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	tokenSvc := newTestTokenService(t)
	h := NewHandlers(svc, tokenSvc)

	_, state, _ := svc.AuthCodeURL()

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oidc/callback?code=valid-code&state="+state, nil)
	rr := httptest.NewRecorder()

	h.Callback(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&resp)

	claims, err := tokenSvc.Validate(resp["access_token"])
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}

	if claims.Name != "Bob" {
		t.Errorf("name = %q, want Bob", claims.Name)
	}
	if !containsStr(claims.Roles, "approver") {
		t.Errorf("roles = %v, want approver", claims.Roles)
	}
	if containsStr(claims.Roles, "admin") {
		t.Errorf("roles = %v, should not contain admin", claims.Roles)
	}
}

func TestCallback_UnmappedUserGetsDefaultRole(t *testing.T) {
	mock := newMockOIDCProvider(t)
	defer mock.close()

	// User has no matching groups.
	mock.tokenClaims = map[string]any{
		"sub":    "user-789",
		"email":  "carol@example.com",
		"name":   "Carol",
		"groups": []string{"other-team"},
	}

	cfg := newTestOIDCConfig(mock)
	svc, err := NewService(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	tokenSvc := newTestTokenService(t)
	h := NewHandlers(svc, tokenSvc)

	_, state, _ := svc.AuthCodeURL()

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oidc/callback?code=valid-code&state="+state, nil)
	rr := httptest.NewRecorder()

	h.Callback(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&resp)

	claims, err := tokenSvc.Validate(resp["access_token"])
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if len(claims.Roles) != 1 || claims.Roles[0] != "viewer" {
		t.Errorf("roles = %v, want [viewer]", claims.Roles)
	}
}

func TestCallback_NameFallback(t *testing.T) {
	// When name is missing, should fall back to email.
	mock := newMockOIDCProvider(t)
	defer mock.close()

	mock.tokenClaims = map[string]any{
		"sub":   "user-no-name",
		"email": "noname@example.com",
	}

	cfg := newTestOIDCConfig(mock)
	svc, err := NewService(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	tokenSvc := newTestTokenService(t)
	h := NewHandlers(svc, tokenSvc)

	_, state, _ := svc.AuthCodeURL()

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/auth/oidc/callback?code=valid-code&state="+state, nil)
	rr := httptest.NewRecorder()

	h.Callback(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&resp)

	claims, err := tokenSvc.Validate(resp["access_token"])
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if claims.Name != "noname@example.com" {
		t.Errorf("name = %q, want email fallback noname@example.com", claims.Name)
	}
}

// --- matchesClaim Tests ---

func TestMatchesClaim(t *testing.T) {
	tests := []struct {
		name     string
		claimVal any
		target   string
		want     bool
	}{
		{"string match", "admin", "admin", true},
		{"string no match", "user", "admin", false},
		{"slice match", []any{"a", "b", "admin"}, "admin", true},
		{"slice no match", []any{"a", "b"}, "admin", false},
		{"string slice match", []string{"x", "admin"}, "admin", true},
		{"string slice no match", []string{"x", "y"}, "admin", false},
		{"int type no match", 42, "42", false},
		{"nil no match", nil, "admin", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesClaim(tt.claimVal, tt.target); got != tt.want {
				t.Errorf("matchesClaim(%v, %q) = %v, want %v", tt.claimVal, tt.target, got, tt.want)
			}
		})
	}
}

// --- helpers ---

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func equalStringSliceUnordered(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]int)
	for _, v := range a {
		m[v]++
	}
	for _, v := range b {
		m[v]--
		if m[v] < 0 {
			return false
		}
	}
	return true
}
