// Package chronos – integration tests with a stub Chronos server.
package chronos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/memory"
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
		var req ChronosIngestRequest
		json.NewDecoder(r.Body).Decode(&req)
		s.ingested = append(s.ingested, req)
		w.WriteHeader(http.StatusNoContent)

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
		Actor:       cgp.Actor{ID: "alice", Kind: cgp.ActorKindHuman},
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
