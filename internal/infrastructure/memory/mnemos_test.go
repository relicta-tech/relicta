// Test suite for MnemosAdapter – uses an in‑memory HTTP stub.
package memory

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

// simple in‑memory event store used by the test server
type eventStore struct {
	events []MnemosEvent
}

func (s *eventStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		// Expect POST /v1/events (Mnemos default path)
		var payload struct {
			Events []MnemosEvent `json:"events"`
		}
		json.NewDecoder(r.Body).Decode(&payload)
		s.events = append(s.events, payload.Events...)
		w.WriteHeader(http.StatusCreated)
	case http.MethodGet:
		// Return stored events as MnemosQueryResponse
		resp := MnemosQueryResponse{Events: s.events, Total: len(s.events)}
		json.NewEncoder(w).Encode(resp)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func newTestServer() (*httptest.Server, *eventStore) {
	store := &eventStore{}
	srv := httptest.NewServer(store)
	return srv, store
}

func TestMnemosAdapter_RecordRelease_And_GetReleaseHistory(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	// Use the test server URL as baseURL (Mnemos expects /v1/events endpoint)
	adapter := NewMnemosStore(srv.URL+"/v1", "test-run", nil)

	// Build a minimal ReleaseRecord matching the memory schema.
	rec := &memory.ReleaseRecord{
		ID:              "rel-1",
		Version:         "v1.0.0",
		Repository:      "github.com/example/repo",
		Actor:           cgp.Actor{ID: "alice", Kind: cgp.ActorKindHuman},
		RiskScore:       0.12,
		Decision:        cgp.DecisionApproved,
		Outcome:         memory.OutcomeSuccess,
		BreakingChanges: 0,
		FilesChanged:    5,
		LinesChanged:    123,
		ReleasedAt:      time.Now().UTC(),
	}

	if err := adapter.RecordRelease(context.Background(), rec); err != nil {
		t.Fatalf("RecordRelease failed: %v", err)
	}

	// Retrieve history – limit 10 should return the single record.
	history, err := adapter.GetReleaseHistory(context.Background(), "github.com/example/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory error: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 record, got %d", len(history))
	}
	if history[0].ID != "release-rel-1" {
		t.Fatalf("unexpected record ID: %s", history[0].ID)
	}
}
