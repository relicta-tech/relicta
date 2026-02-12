package token

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSecret = []byte("test-secret-key-that-is-at-least-32-bytes-long!")

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, err := NewService(Config{Secret: testSecret})
	require.NoError(t, err)
	return svc
}

func TestNewService_SecretTooShort(t *testing.T) {
	_, err := NewService(Config{Secret: []byte("short")})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least 32 bytes")
}

func TestNewService_Defaults(t *testing.T) {
	svc := newTestService(t)
	assert.Equal(t, 15*time.Minute, svc.cfg.AccessTTL)
	assert.Equal(t, 24*time.Hour, svc.cfg.RefreshTTL)
	assert.Equal(t, "relicta", svc.cfg.Issuer)
}

func TestNewService_CustomConfig(t *testing.T) {
	svc, err := NewService(Config{
		Secret:     testSecret,
		AccessTTL:  5 * time.Minute,
		RefreshTTL: 1 * time.Hour,
		Issuer:     "custom",
	})
	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, svc.cfg.AccessTTL)
	assert.Equal(t, 1*time.Hour, svc.cfg.RefreshTTL)
	assert.Equal(t, "custom", svc.cfg.Issuer)
}

func TestIssue(t *testing.T) {
	svc := newTestService(t)
	pair, err := svc.Issue("alice", []string{"admin", "approver"})
	require.NoError(t, err)

	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	assert.NotEqual(t, pair.AccessToken, pair.RefreshToken)
	assert.WithinDuration(t, time.Now().Add(15*time.Minute), pair.ExpiresAt, 5*time.Second)
}

func TestValidate_AccessToken(t *testing.T) {
	svc := newTestService(t)
	pair, err := svc.Issue("bob", []string{"viewer"})
	require.NoError(t, err)

	claims, err := svc.Validate(pair.AccessToken)
	require.NoError(t, err)

	assert.Equal(t, "bob", claims.Name)
	assert.Equal(t, "bob", claims.Subject)
	assert.Equal(t, []string{"viewer"}, claims.Roles)
	assert.Equal(t, "relicta", claims.Issuer)
	assert.NotEmpty(t, claims.ID)
}

func TestValidate_RefreshToken(t *testing.T) {
	svc := newTestService(t)
	pair, err := svc.Issue("carol", []string{"approver"})
	require.NoError(t, err)

	claims, err := svc.Validate(pair.RefreshToken)
	require.NoError(t, err)

	assert.Equal(t, "carol", claims.Name)
	assert.Equal(t, []string{"approver"}, claims.Roles)
}

func TestValidate_ExpiredToken(t *testing.T) {
	svc, err := NewService(Config{
		Secret:    testSecret,
		AccessTTL: 1 * time.Millisecond,
	})
	require.NoError(t, err)

	pair, err := svc.Issue("dave", []string{"viewer"})
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	_, err = svc.Validate(pair.AccessToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token")
}

func TestValidate_MalformedToken(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Validate("not.a.valid.token")
	assert.Error(t, err)
}

func TestValidate_WrongSecret(t *testing.T) {
	svc1, err := NewService(Config{Secret: testSecret})
	require.NoError(t, err)

	svc2, err := NewService(Config{Secret: []byte("different-secret-that-is-at-least-32-bytes!!")})
	require.NoError(t, err)

	pair, err := svc1.Issue("eve", []string{"admin"})
	require.NoError(t, err)

	_, err = svc2.Validate(pair.AccessToken)
	assert.Error(t, err)
}

func TestRevoke(t *testing.T) {
	svc := newTestService(t)
	pair, err := svc.Issue("frank", []string{"viewer"})
	require.NoError(t, err)

	// Token valid before revocation.
	claims, err := svc.Validate(pair.AccessToken)
	require.NoError(t, err)

	// Revoke.
	svc.Revoke(claims.ID, claims.ExpiresAt.Time)

	// Token rejected after revocation.
	_, err = svc.Validate(pair.AccessToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revoked")
}

func TestRevokeToken(t *testing.T) {
	svc := newTestService(t)
	pair, err := svc.Issue("grace", []string{"admin"})
	require.NoError(t, err)

	err = svc.RevokeToken(pair.AccessToken)
	require.NoError(t, err)

	_, err = svc.Validate(pair.AccessToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revoked")
}

func TestRevokeToken_InvalidToken(t *testing.T) {
	svc := newTestService(t)
	err := svc.RevokeToken("garbage")
	assert.Error(t, err)
}

func TestRefresh(t *testing.T) {
	svc := newTestService(t)
	original, err := svc.Issue("heidi", []string{"approver", "viewer"})
	require.NoError(t, err)

	refreshed, err := svc.Refresh(original.RefreshToken)
	require.NoError(t, err)

	assert.NotEmpty(t, refreshed.AccessToken)
	assert.NotEqual(t, original.AccessToken, refreshed.AccessToken)
	assert.NotEqual(t, original.RefreshToken, refreshed.RefreshToken)

	// New access token is valid.
	claims, err := svc.Validate(refreshed.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "heidi", claims.Name)
	assert.Equal(t, []string{"approver", "viewer"}, claims.Roles)

	// Old refresh token is revoked.
	_, err = svc.Validate(original.RefreshToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revoked")
}

func TestRefresh_InvalidToken(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Refresh("not.a.valid.token")
	assert.Error(t, err)
}

func TestCleanExpired(t *testing.T) {
	svc := newTestService(t)

	// Revoke a token with an expiry in the past.
	past := time.Now().Add(-1 * time.Second)
	svc.Revoke("expired-jti", past)

	svc.mu.RLock()
	assert.Len(t, svc.revoked, 1)
	svc.mu.RUnlock()

	svc.CleanExpired()

	svc.mu.RLock()
	assert.Empty(t, svc.revoked)
	svc.mu.RUnlock()
}

func TestCleanExpired_KeepsActive(t *testing.T) {
	svc := newTestService(t)
	pair, err := svc.Issue("judy", []string{"admin"})
	require.NoError(t, err)

	claims, err := svc.Validate(pair.AccessToken)
	require.NoError(t, err)
	svc.Revoke(claims.ID, claims.ExpiresAt.Time)

	// Clean should not remove active entries.
	svc.CleanExpired()

	svc.mu.RLock()
	assert.Len(t, svc.revoked, 1)
	svc.mu.RUnlock()
}

func TestConcurrentAccess(t *testing.T) {
	svc := newTestService(t)
	done := make(chan struct{})

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			pair, err := svc.Issue("concurrent", []string{"viewer"})
			if err != nil {
				return
			}
			_, _ = svc.Validate(pair.AccessToken)
			_ = svc.RevokeToken(pair.AccessToken)
			svc.CleanExpired()
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
