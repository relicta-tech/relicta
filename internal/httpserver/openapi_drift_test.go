package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/httpserver/openapi"
)

// The published OpenAPI specification described 18 paths while the router served
// considerably more. Nothing reported the gap, so it widened with every endpoint added —
// including the deployment endpoints from ADR-012, which are the ones an external
// deployer most needs a contract for.
//
// A specification that omits endpoints is worse than an obviously incomplete one: a
// client trusts it, and an absent path reads as "this API has no such feature".
//
// This test does not demand the backlog be paid off at once. It pins the current gap and
// fails on a new one, so an endpoint added from here must either be documented or
// consciously listed below — which is a decision someone makes rather than an omission
// nobody notices.
//
// To satisfy it after adding a route: document the path in
// internal/httpserver/openapi/openapi.json. Adding it to undocumentedByDecision is the
// escape hatch, not the default.
var undocumentedByDecision = map[string]string{
	// Real-time transports. Neither is an HTTP request/response pair, and OpenAPI 3.0 has
	// no vocabulary for either.
	"/ws":            "WebSocket upgrade, not an HTTP operation",
	"/events/stream": "Server-Sent Events stream, not an HTTP operation",

	// Pre-existing debt: 20 of the API's 40 routes are undocumented. Listed individually
	// rather than waved past with a count, so the gap is legible and shrinking it is a
	// series of small deletions from this map.
	"/webhooks/{id}/deliveries":                        "webhook delivery inspection",
	"/webhooks/{id}/deliveries/{deliveryId}/redeliver": "webhook redelivery",
	"/releases/{id}/recommendation":                    "ADR-009 recommendation artifact",
	"/analytics/risk-trends":                           "analytics surface",
	"/analytics/decisions":                             "analytics surface",
	"/analytics/team":                                  "analytics surface",
	"/analytics/outcomes":                              "analytics surface",
	"/analytics/risk-factors":                          "analytics surface",
	"/analytics/calibration":                           "analytics surface",
	"/memory/insights":                                 "learning surface",
	"/memory/trends":                                   "learning surface",
	"/groups":                                          "multi-repo group surface",
	"/groups/{name}/status":                            "multi-repo group surface",
	"/groups/{name}/graph":                             "multi-repo group surface",
	"/observability/health":                            "observability surface",
	"/observability/providers":                         "observability surface",
	"/observability/correlations":                      "observability surface",
	"/observability/webhook/{provider}":                "inbound incident receiver",
}

// documentedPaths reads the paths the specification claims to describe.
func documentedPaths(t *testing.T) map[string]bool {
	t.Helper()
	var doc struct {
		Paths map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(openapi.Spec(), &doc); err != nil {
		t.Fatalf("the embedded specification is not valid JSON: %v", err)
	}
	out := make(map[string]bool, len(doc.Paths))
	for p := range doc.Paths {
		out[p] = true
	}
	return out
}

// servedPaths walks the router for every route it actually serves, normalized to the
// form the specification uses: without the /api/v1 prefix, with chi's {param} syntax.
func servedPaths(t *testing.T) map[string]bool {
	t.Helper()
	srv := NewServer(ServerDeps{Config: config.DashboardConfig{Address: ":0"}})

	out := map[string]bool{}
	err := chi.Walk(srv.router, func(_ string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		route = strings.TrimSuffix(route, "/*")
		// chi registers a sub-router's index route with a trailing slash ("/actors/")
		// while the specification names it without one. Same endpoint either way.
		if route != "/" {
			route = strings.TrimSuffix(route, "/")
		}
		if !strings.HasPrefix(route, "/api/v1") {
			// Unversioned infrastructure endpoints (static assets, the dashboard itself).
			return nil
		}
		route = strings.TrimPrefix(route, "/api/v1")
		if route == "" {
			return nil
		}
		out[route] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walking the router: %v", err)
	}
	return out
}

func TestEveryServedRouteIsDocumentedOrKnown(t *testing.T) {
	documented := documentedPaths(t)
	served := servedPaths(t)

	var undocumented []string
	for route := range served {
		if documented[route] {
			continue
		}
		if _, known := undocumentedByDecision[route]; known {
			continue
		}
		undocumented = append(undocumented, route)
	}

	if len(undocumented) > 0 {
		t.Errorf("these routes are served but absent from the OpenAPI specification: %v\n\n"+
			"Document them in internal/httpserver/openapi/openapi.json. A published contract "+
			"that omits an endpoint is worse than an obviously incomplete one, because a "+
			"client trusts it. If a route genuinely cannot be expressed, add it to "+
			"undocumentedByDecision with the reason.", undocumented)
	}
}

// The debt list must not outlive the debt. An entry for a route that no longer exists, or
// one that has since been documented, makes the list look like a real constraint while
// hiding that it is empty.
func TestTheUndocumentedListHasNoStaleEntries(t *testing.T) {
	documented := documentedPaths(t)
	served := servedPaths(t)

	for route, reason := range undocumentedByDecision {
		if !served[route] {
			t.Errorf("undocumentedByDecision lists %q (%s) but the router does not serve it: "+
				"remove the entry", route, reason)
		}
		if documented[route] {
			t.Errorf("undocumentedByDecision lists %q (%s) but it is now documented: remove "+
				"the entry so the list keeps meaning something", route, reason)
		}
	}
}

// The two endpoints an external deployer integrates against. Named explicitly because
// they are the contract another product builds on, and a generic drift test would let
// them slip back out as long as the total count stayed put.
func TestTheDeploymentEndpointsAreDocumented(t *testing.T) {
	documented := documentedPaths(t)

	for _, path := range []string{"/webhooks/deployments", "/webhooks/authorize"} {
		if !documented[path] {
			t.Errorf("%s is not in the OpenAPI specification: it is the documented wire "+
				"contract a deployer integrates against, so leaving it out of the published "+
				"spec asks every integrator to read our source instead", path)
		}
	}
}
