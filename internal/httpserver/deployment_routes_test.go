package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// The deployment endpoints (ADR-012) were tested by calling their handler functions
// directly, which proves the logic and nothing about reachability. A handler that is
// correct but unroutable — wrong path, wrong method, or refused by middleware before it
// runs — is the failure this file exists to catch, and it is the one that would leave a
// deployer unable to report or ask anything while every unit test stayed green.

func newDeploymentServer(t *testing.T) *Server {
	t.Helper()
	// Auth mode none: the default, and the mode a deployer posting with only an HMAC
	// signature relies on. If these routes ever move behind a mode that requires a
	// bearer token, that is a deliberate decision and this test should be the thing
	// that forces the conversation.
	return NewServer(ServerDeps{Config: config.DashboardConfig{Address: ":0"}})
}

func postJSON(t *testing.T, srv *Server, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	return rec
}

// Both routes must exist. A 404 or 405 here means the handler is unreachable however
// well it behaves in isolation.
func TestDeploymentRoutesAreRoutable(t *testing.T) {
	srv := newDeploymentServer(t)

	for _, route := range []struct {
		name string
		path string
		body map[string]any
	}{
		{"evidence", "/api/v1/webhooks/deployments", map[string]any{
			"environment": "production", "version": "1.0.0",
		}},
		{"gate", "/api/v1/webhooks/authorize", map[string]any{
			"action": "probe",
		}},
	} {
		t.Run(route.name, func(t *testing.T) {
			rec := postJSON(t, srv, route.path, route.body)

			if rec.Code == http.StatusNotFound {
				t.Fatalf("POST %s returned 404: the route is not registered, so no deployer "+
					"can reach it no matter how the handler behaves", route.path)
			}
			if rec.Code == http.StatusMethodNotAllowed {
				t.Fatalf("POST %s returned 405: the route exists but not for POST, which is "+
					"the only method a reporter uses", route.path)
			}
			if rec.Code == http.StatusUnauthorized {
				t.Fatalf("POST %s returned 401 with auth mode none: middleware refuses the "+
					"request before the handler runs, so the HMAC signature the endpoint "+
					"documents can never be the credential", route.path)
			}
		})
	}
}

// A probe must be answered with a decision, not merely not-404. This is the request a
// caller makes to check the gate is working, so it has to survive the whole stack:
// routing, middleware, handler, store resolution and JSON encoding.
func TestTheGateAnswersAProbeThroughTheFullStack(t *testing.T) {
	rec := postJSON(t, newDeploymentServer(t), "/api/v1/webhooks/authorize", map[string]any{
		"action":     "probe",
		"target_ref": "k8s/prod/api",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var decision struct {
		Allowed  bool              `json:"allowed"`
		Reason   string            `json:"reason"`
		Evidence map[string]string `json:"evidence"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&decision); err != nil {
		t.Fatalf("the response is not a decision a caller can parse: %v", err)
	}
	if !decision.Allowed {
		t.Errorf("a readiness probe was refused (%q); a caller could then never distinguish "+
			"a working gate from a broken one", decision.Reason)
	}
	if decision.Evidence["probe"] != "true" {
		t.Errorf("Evidence[probe] = %q, want true so a probe is never recorded as a "+
			"deployment decision", decision.Evidence["probe"])
	}
}

// A signature is required once a secret is configured, and that has to hold at the
// route level too — not only in the function that checks it.
func TestAConfiguredSecretIsEnforcedOnTheRoutes(t *testing.T) {
	t.Setenv("RELICTA_WEBHOOK_SECRET", "s3cret")
	srv := newDeploymentServer(t)

	for _, path := range []string{
		"/api/v1/webhooks/deployments",
		"/api/v1/webhooks/authorize",
	} {
		rec := postJSON(t, srv, path, map[string]any{"action": "probe", "version": "1.0.0"})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("POST %s unsigned returned %d, want 401: a configured secret must not be "+
				"satisfied by omitting the header", path, rec.Code)
		}
	}
}
