// Package chronos – integration tests with a stub Chronos server.
package chronos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v5/internal/cgp"
	"github.com/relicta-tech/relicta/v5/internal/cgp/memory"
)

// stubChronos captures ingest requests and returns empty signals.
type stubChronos struct {
	ingested []ChronosIngestRequest
}

func (s *stubChronos) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/v1/ingest":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var reqs []ChronosIngestRequest
		if err := json.NewDecoder(r.Body).Decode(&reqs); err == nil {
			s.ingested = append(s.ingested, reqs...)
		}
		w.WriteHeader(http.StatusCreated)

	case "/v1/signals":
		resp := ChronosSignalsResponse{Signals: []ChronosSignal{}, Count: 0}
		json.NewEncoder(w).Encode(resp)

	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func TestChronosAdapter_RecordRelease(t *testing.T) {
	stub := &stubChronos{}
	srv := httptest.NewServer(stub)
	defer srv.Close()

	adapter := NewChronosAdapter(srv.URL, "test-scope")

	rec := &memory.ReleaseRecord{
		ID:         "rel-1",
		Repository: "github.com/example/repo",
		Version:    "v1.0.0",
		Actor:      cgp.Actor{ID: "alice", Kind: cgp.ActorKindHuman},
		RiskScore:  0.25,
		Decision:   cgp.DecisionApproved,
		Outcome:    memory.OutcomeSuccess,
		ReleasedAt: time.Now().UTC(),
	}

	if err := adapter.RecordRelease(context.Background(), rec); err != nil {
		t.Fatalf("RecordRelease failed: %v", err)
	}

	if len(stub.ingested) != 1 {
		t.Fatalf("expected 1 ingest request, got %d", len(stub.ingested))
	}
}

func TestChronosAdapter_GetReleaseHistory_NotSupported(t *testing.T) {
	stub := &stubChronos{}
	srv := httptest.NewServer(stub)
	defer srv.Close()

	adapter := NewChronosAdapter(srv.URL, "test-scope")

	_, err := adapter.GetReleaseHistory(context.Background(), "any", 10)
	if err == nil {
		t.Fatal("expected error for unsupported GetReleaseHistory")
	}
}

func TestChronosAdapter_RecordIncident(t *testing.T) {
	stub := &stubChronos{}
	srv := httptest.NewServer(stub)
	defer srv.Close()

	adapter := NewChronosAdapter(srv.URL, "test-scope")
	err := adapter.RecordIncident(context.Background(), &memory.IncidentRecord{
		ID: "incident-1", Repository: "example/repo", ReleaseID: "release-1",
		Version: "v1.2.3", Type: memory.IncidentSecurity, Severity: cgp.SeverityHigh,
		ActorID: "alice", DetectedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("RecordIncident: %v", err)
	}
	if len(stub.ingested) != 1 {
		t.Fatalf("ingested %d requests, want 1", len(stub.ingested))
	}
	got := stub.ingested[0]
	if got.EntityID != "incident-1" || got.ScopeID != "test-scope" || len(got.Features) != 1 || got.Features[0] != 1 {
		t.Errorf("unexpected incident ingest: %+v", got)
	}
	if len(got.Labels) != 2 || got.Labels[0] != "incident" || got.Labels[1] != string(memory.IncidentSecurity) {
		t.Errorf("incident labels = %v", got.Labels)
	}
}

func TestChronosAdapter_GetRiskPatternsMapsSignals(t *testing.T) {
	start, end := time.Now().Add(-time.Hour).UTC(), time.Now().UTC()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/signals" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(ChronosSignalsResponse{Signals: []ChronosSignal{
			{Pattern: "trend", Strength: 0.8},
			{Pattern: "spike", Window: ChronosWindow{Start: start, End: end}},
			{Pattern: "stall"}, {Pattern: "anomaly"}, {Pattern: "trend", Strength: 0.1},
		}})
	}))
	defer srv.Close()

	patterns, err := NewChronosAdapter(srv.URL, "scope").GetRiskPatterns(context.Background(), "example/repo")
	if err != nil {
		t.Fatalf("GetRiskPatterns: %v", err)
	}
	if patterns.Repository != "example/repo" || patterns.RiskTrend != memory.TrendStable {
		t.Errorf("unexpected mapped patterns: %+v", patterns)
	}
	if len(patterns.HighRiskPeriods) != 1 || !patterns.HighRiskPeriods[0].Start.Equal(start) || !patterns.HighRiskPeriods[0].End.Equal(end) {
		t.Errorf("high risk periods = %+v", patterns.HighRiskPeriods)
	}
}

func TestChronosAdapter_GetRiskPatternsDegradesOnUnavailableService(t *testing.T) {
	adapter := NewChronosAdapter("http://127.0.0.1:1", "scope")
	patterns, err := adapter.GetRiskPatterns(context.Background(), "example/repo")
	if err != nil {
		t.Fatalf("GetRiskPatterns should degrade gracefully: %v", err)
	}
	if patterns.Repository != "example/repo" || len(patterns.HighRiskPeriods) != 0 {
		t.Errorf("unexpected fallback patterns: %+v", patterns)
	}
}

func TestChronosAdapterUnsupportedStoreOperationsAreExplicit(t *testing.T) {
	adapter := NewChronosAdapter("", "scope")
	ctx := context.Background()
	checks := []struct {
		name string
		call func() error
	}{
		{"incident history", func() error { _, err := adapter.GetIncidentHistory(ctx, "repo", 10); return err }},
		{"decision lookup", func() error { _, err := adapter.GetDecision(ctx, "decision"); return err }},
		{"proposal decisions", func() error { _, err := adapter.GetDecisionsByProposal(ctx, "proposal"); return err }},
		{"authorization lookup", func() error { _, err := adapter.GetAuthorization(ctx, "authorization"); return err }},
		{"decision authorizations", func() error { _, err := adapter.GetAuthorizationsByDecision(ctx, "decision"); return err }},
		{"audit trail", func() error { _, err := adapter.GetAuditTrail(ctx, "proposal"); return err }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.call(); err == nil {
				t.Fatal("expected unsupported operation error")
			}
		})
	}

	if err := adapter.RecordDecision(ctx, nil); err != nil {
		t.Errorf("RecordDecision no-op: %v", err)
	}
	if err := adapter.RecordAuthorization(ctx, nil); err != nil {
		t.Errorf("RecordAuthorization no-op: %v", err)
	}
	if err := adapter.UpdateActorMetrics(ctx, "alice", memory.OutcomeSuccess); err != nil {
		t.Errorf("UpdateActorMetrics no-op: %v", err)
	}
	metrics, err := adapter.GetActorMetrics(ctx, "alice")
	if err != nil || metrics.ActorID != "alice" || metrics.ActorKind != cgp.ActorKindHuman {
		t.Errorf("GetActorMetrics = %+v, %v", metrics, err)
	}
}
