package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOIDCConfig_Defaults(t *testing.T) {
	cfg := &OIDCConfig{
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "my-client-id",
		ClientSecret: "my-client-secret",
	}

	cfg.Defaults()

	assert.Equal(t, []string{"openid", "profile", "email"}, cfg.Scopes)
	assert.Equal(t, "groups", cfg.GroupsClaim)
	assert.Equal(t, "viewer", cfg.DefaultRole)
}

func TestOIDCConfig_Defaults_PreservesExisting(t *testing.T) {
	cfg := &OIDCConfig{
		IssuerURL:    "https://login.microsoftonline.com/tenant/v2.0",
		ClientID:     "azure-client",
		ClientSecret: "azure-secret",
		Scopes:       []string{"openid", "profile", "email", "groups"},
		GroupsClaim:  "roles",
		DefaultRole:  "approver",
	}

	cfg.Defaults()

	assert.Equal(t, []string{"openid", "profile", "email", "groups"}, cfg.Scopes)
	assert.Equal(t, "roles", cfg.GroupsClaim)
	assert.Equal(t, "approver", cfg.DefaultRole)
}

func TestOIDCClaimMapping(t *testing.T) {
	mappings := []OIDCClaimMapping{
		{Value: "relicta-admins", Role: "admin"},
		{Value: "relicta-approvers", Role: "approver"},
		{Claim: "email", Value: "admin@example.com", Role: "admin"},
	}

	assert.Equal(t, "relicta-admins", mappings[0].Value)
	assert.Equal(t, "admin", mappings[0].Role)
	assert.Empty(t, mappings[0].Claim) // defaults to GroupsClaim

	assert.Equal(t, "email", mappings[2].Claim) // explicit claim
	assert.Equal(t, "admin@example.com", mappings[2].Value)
}

func TestDashboardAuthConfig_OIDCMode(t *testing.T) {
	cfg := DashboardAuthConfig{
		Mode:          DashboardAuthOIDC,
		SessionSecret: "a-secret-that-is-at-least-32-bytes-long!!",
		OIDC: &OIDCConfig{
			IssuerURL:    "https://accounts.google.com",
			ClientID:     "my-client",
			ClientSecret: "my-secret",
			RedirectURL:  "http://localhost:8080/api/v1/auth/oidc/callback",
			ClaimMappings: []OIDCClaimMapping{
				{Value: "admins", Role: "admin"},
			},
		},
	}

	assert.Equal(t, DashboardAuthOIDC, cfg.Mode)
	assert.NotNil(t, cfg.OIDC)
	assert.Equal(t, "https://accounts.google.com", cfg.OIDC.IssuerURL)
	assert.Len(t, cfg.OIDC.ClaimMappings, 1)

	cfg.OIDC.Defaults()
	assert.Equal(t, "groups", cfg.OIDC.GroupsClaim)
	assert.Equal(t, "viewer", cfg.OIDC.DefaultRole)
}

func TestDashboardAuthOIDC_Constant(t *testing.T) {
	assert.Equal(t, DashboardAuthMode("oidc"), DashboardAuthOIDC)
}
