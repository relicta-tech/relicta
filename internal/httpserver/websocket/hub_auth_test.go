package websocket

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockTokenValidator is a test implementation of TokenValidator.
type mockTokenValidator struct {
	validTokens map[string]struct {
		name  string
		roles []string
	}
}

func (m *mockTokenValidator) Validate(tokenStr string) (string, []string, error) {
	if entry, ok := m.validTokens[tokenStr]; ok {
		return entry.name, entry.roles, nil
	}
	return "", nil, fmt.Errorf("invalid token")
}

func TestHub_SetTokenValidator(t *testing.T) {
	hub := NewHub([]string{"*"})
	assert.Nil(t, hub.tokenValidator)

	validator := &mockTokenValidator{}
	hub.SetTokenValidator(validator)
	assert.NotNil(t, hub.tokenValidator)
}

func TestHub_HandleConnection_AuthRequired_MissingToken(t *testing.T) {
	hub := NewHub([]string{"*"})
	hub.SetTokenValidator(&mockTokenValidator{
		validTokens: map[string]struct {
			name  string
			roles []string
		}{
			"valid-token": {name: "alice", roles: []string{"admin"}},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()

	hub.HandleConnection(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing token")
}

func TestHub_HandleConnection_AuthRequired_InvalidToken(t *testing.T) {
	hub := NewHub([]string{"*"})
	hub.SetTokenValidator(&mockTokenValidator{
		validTokens: map[string]struct {
			name  string
			roles []string
		}{
			"valid-token": {name: "alice", roles: []string{"admin"}},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/ws?token=bad-token", nil)
	rec := httptest.NewRecorder()

	hub.HandleConnection(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid or expired")
}

func TestHub_HandleConnection_NoValidator_AllowsAll(t *testing.T) {
	hub := NewHub([]string{"*"})
	// No validator set — should attempt WebSocket upgrade (which will fail
	// in test because httptest.ResponseRecorder doesn't support WebSocket,
	// but it should NOT return 401).
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()

	hub.HandleConnection(rec, req)

	// Without a real WebSocket client, the upgrade will fail with a non-401 error.
	// The key assertion is that it did NOT return 401 (auth was skipped).
	assert.NotEqual(t, http.StatusUnauthorized, rec.Code)
}

func TestHub_HandleConnection_AuthRequired_ValidToken(t *testing.T) {
	hub := NewHub([]string{"*"})
	hub.SetTokenValidator(&mockTokenValidator{
		validTokens: map[string]struct {
			name  string
			roles []string
		}{
			"good-token": {name: "bob", roles: []string{"viewer"}},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/ws?token=good-token", nil)
	rec := httptest.NewRecorder()

	hub.HandleConnection(rec, req)

	// Auth passes, but WebSocket upgrade fails (no real WS handshake).
	// The key point: NOT 401.
	assert.NotEqual(t, http.StatusUnauthorized, rec.Code)
}
