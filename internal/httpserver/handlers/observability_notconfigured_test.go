package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The three read routes returned a bare empty collection with 200, so a caller could not tell
// "no providers are configured" from "everything is healthy" — and an empty health list reads
// as the second. That distinction is the whole value of a health endpoint.
//
// It is not a hypothetical state. Nothing constructs an ObservabilityService: no
// implementation of the interface exists anywhere, ObservabilityProviderConfig is consumed by
// nothing, and NewHealthMonitor has no caller. Every one of these routes gives this answer to
// every caller today, which is why saying so matters more than it would for a rare branch.
//
// The empty collection stays in the payload so existing consumers keep parsing.
func TestUnconfiguredObservabilityRoutesSaySoRatherThanLookingHealthy(t *testing.T) {
	original := observabilitySvc
	t.Cleanup(func() { observabilitySvc = original })
	observabilitySvc = nil

	for name, tc := range map[string]struct {
		handler    http.HandlerFunc
		target     string
		collection string
	}{
		"health":       {GetObservabilityHealth, "/api/v1/observability/health", "health"},
		"correlations": {GetObservabilityCorrelations, "/api/v1/observability/correlations?release_id=r1", "correlations"},
		"providers":    {GetObservabilityProviders, "/api/v1/observability/providers", "providers"},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			w := httptest.NewRecorder()
			tc.handler(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: an unconfigured subsystem is not an error", w.Code)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode: %v", err)
			}

			if resp["status"] != "not_configured" {
				t.Errorf("status = %v, want not_configured: an empty %s with no explanation "+
					"reads as a clean bill of health for a subsystem that is not running",
					resp["status"], tc.collection)
			}
			if _, ok := resp[tc.collection]; !ok {
				t.Errorf("the %s key is missing; it stays in the payload so existing "+
					"consumers keep parsing", tc.collection)
			}
			if resp["details"] == nil || resp["details"] == "" {
				t.Error("no details telling the reader what to configure")
			}
		})
	}
}
