package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The deployment endpoint records that something happened; this one decides whether
// it may. These tests cover the HTTP contract — the decision logic itself is tested
// in internal/cgp/memory. What matters here is that a caller can tell a refusal from
// a breakage, because a caller that fails closed treats both as a block and only one
// of them should be shown to a human as a policy answer.

func postAuthorize(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/authorize", strings.NewReader(body))
	rec := httptest.NewRecorder()
	Authorize(rec, req)
	return rec
}

// A malformed body is the caller's fault and must say so, rather than being answered
// with a decision that was not made.
func TestAuthorizeRejectsAnUnparseableRequest(t *testing.T) {
	rec := postAuthorize(t, `{not json`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for an unparseable request body", rec.Code)
	}
	// Specifically: not a 200 carrying allowed=false, which a caller would read as a
	// governance refusal and report to a human as a policy decision.
	var decision map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decision); err == nil {
		if _, hasAllowed := decision["allowed"]; hasAllowed {
			t.Error("a malformed request was answered with a decision: a caller cannot then " +
				"distinguish its own bad payload from a governance refusal")
		}
	}
}

// A configured secret must not be satisfied by omitting the header, or the protection
// is opt-out by the caller.
func TestAuthorizeRequiresASignatureWhenConfigured(t *testing.T) {
	t.Setenv("RELICTA_WEBHOOK_SECRET", "s3cret")

	rec := postAuthorize(t, `{"action":"apply","environment":"production","version":"1.0.0"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 when a secret is configured and the request is unsigned", rec.Code)
	}
}

// The endpoint never answers "allowed" because it could not check.
//
// This runs outside a repository, so the store cannot be resolved — the same shape as
// a broken or unreadable store in production. The answer must be an error, because a
// caller cannot distinguish allow-because-governed from allow-because-broken, and
// answering "allowed" here would silently disable the gate.
func TestAuthorizeWillNotAllowWhatItCannotCheck(t *testing.T) {
	t.Setenv("RELICTA_WEBHOOK_SECRET", "")
	t.Chdir(t.TempDir()) // not a repository: nothing to read

	rec := postAuthorize(t, `{"action":"apply","environment":"production","version":"9.9.9"}`)

	if rec.Code == http.StatusOK {
		var decision struct {
			Allowed bool `json:"allowed"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &decision); err == nil && decision.Allowed {
			t.Fatal("the endpoint allowed a deployment it could not check: a gate that opens " +
				"when its own store is unreadable is absent exactly when something is wrong")
		}
	}
	if rec.Code < 500 && rec.Code != http.StatusOK {
		t.Errorf("status = %d, want a 5xx so a caller can tell a broken governor from a "+
			"refusing one", rec.Code)
	}
}

// The actor defaults matter for the audit trail: a deployment arriving over HTTP was
// performed by a machine unless the caller says otherwise.
func TestAuthorizationActorDefaults(t *testing.T) {
	actor := authorizationActor(authorizationRequest{})

	if actor.Kind == "" {
		t.Error("Actor.Kind must default rather than being recorded empty")
	}
	if actor.ID == "" {
		t.Error("Actor.ID must default rather than being recorded empty: an evidence entry " +
			"attributing a decision to nobody cannot be audited")
	}

	named := authorizationActor(authorizationRequest{ActorID: "pipeline-7", ActorKind: "ci"})
	if named.ID != "pipeline-7" || string(named.Kind) != "ci" {
		t.Errorf("a caller's own attribution must survive: got %+v", named)
	}
}
