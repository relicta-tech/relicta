package memory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

// NewMnemosStore returns this adapter, and the container assigns it as the memory store
// whenever Mnemos is configured — so its read side is on the live path, not a stub
// nobody reaches. It returned records carrying only an ID, which meant DORA metrics
// computed from nothing, reconcile unable to match a deployment to a release, an empty
// `relicta history`, reputation reading a history with no actors, and the deployment gate
// refusing a legitimate release as ungoverned because it looks releases up by version.
//
// Nothing was missing from storage: RecordRelease already writes every field into
// Metadata. These tests hold the read side to what the write side puts there.

// mnemosServer captures what the adapter writes and replays it on query, so a record can
// be round-tripped through the real HTTP paths rather than through a hand-built event.
func mnemosServer(t *testing.T) (*httptest.Server, *[]MnemosEvent) {
	t.Helper()
	stored := &[]MnemosEvent{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var payload struct {
				Events []MnemosEvent `json:"events"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				// Some clients wrap differently; fall back to a bare array.
				w.WriteHeader(http.StatusOK)
				return
			}
			*stored = append(*stored, payload.Events...)
			w.WriteHeader(http.StatusOK)
		default:
			_ = json.NewEncoder(w).Encode(MnemosQueryResponse{Events: *stored, Total: len(*stored)})
		}
	}))
	t.Cleanup(srv.Close)
	return srv, stored
}

func TestAReleaseSurvivesTheRoundTrip(t *testing.T) {
	srv, stored := mnemosServer(t)
	adapter := NewMnemosStore(srv.URL, "run-ns", srv.Client())
	ctx := context.Background()

	releasedAt := time.Date(2026, 8, 11, 9, 14, 0, 0, time.UTC)
	original := &memory.ReleaseRecord{
		ID:              "run-7",
		Repository:      "acme/widget",
		Version:         "1.4.0",
		Actor:           cgp.Actor{ID: "human:felix", Kind: cgp.ActorKindHuman},
		RiskScore:       0.42,
		Decision:        cgp.DecisionApproved,
		Outcome:         memory.OutcomeSuccess,
		BreakingChanges: 2,
		FilesChanged:    17,
		LinesChanged:    431,
		ReleasedAt:      releasedAt,
	}

	if err := adapter.RecordRelease(ctx, original); err != nil {
		t.Fatalf("RecordRelease: %v", err)
	}
	if len(*stored) != 1 {
		t.Fatalf("the adapter stored %d events, want 1", len(*stored))
	}

	records, err := adapter.GetReleaseHistory(ctx, "acme/widget", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("read back %d records, want 1", len(records))
	}
	got := records[0]

	// The ID must come back as it went in. Readers correlate on it, and the write side
	// prefixes it — returning "release-run-7" would match nothing.
	if got.ID != "run-7" {
		t.Errorf("ID = %q, want run-7 (the stored SourceID prefix must be stripped)", got.ID)
	}
	// The field the deployment gate matches on.
	if got.Version != "1.4.0" {
		t.Errorf("Version = %q, want 1.4.0; empty means the gate would refuse this release "+
			"as ungoverned", got.Version)
	}
	if got.Outcome != memory.OutcomeSuccess {
		t.Errorf("Outcome = %q, want success; empty means every DORA rate counts it as "+
			"neither a success nor a failure", got.Outcome)
	}
	if !got.ReleasedAt.Equal(releasedAt) {
		t.Errorf("ReleasedAt = %v, want %v; a zero time puts the release outside every "+
			"report period", got.ReleasedAt, releasedAt)
	}
	if got.Actor.ID != "human:felix" || got.Actor.Kind != cgp.ActorKindHuman {
		t.Errorf("Actor = %+v, want human:felix/human; without it reputation and earned "+
			"trust read a history that attributes nothing", got.Actor)
	}
	if got.Decision != cgp.DecisionApproved {
		t.Errorf("Decision = %q, want approved", got.Decision)
	}
	// Numbers survive the JSON round trip, where every number becomes a float64. Read
	// as the wrong type these silently become zero — the stub's failure, reached subtly.
	if got.RiskScore < 0.41 || got.RiskScore > 0.43 {
		t.Errorf("RiskScore = %v, want 0.42", got.RiskScore)
	}
	if got.BreakingChanges != 2 || got.FilesChanged != 17 || got.LinesChanged != 431 {
		t.Errorf("counts = %d/%d/%d, want 2/17/431: a JSON round trip makes every number a "+
			"float64, and asserting to int yields zero", got.BreakingChanges, got.FilesChanged,
			got.LinesChanged)
	}
}

func TestAnIncidentSurvivesTheRoundTrip(t *testing.T) {
	srv, _ := mnemosServer(t)
	adapter := NewMnemosStore(srv.URL, "run-ns", srv.Client())
	ctx := context.Background()

	detectedAt := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	if err := adapter.RecordIncident(ctx, &memory.IncidentRecord{
		ID:         "inc-3",
		Repository: "acme/widget",
		ReleaseID:  "run-7",
		Version:    "1.4.0",
		Type:       memory.IncidentRollback,
		Severity:   cgp.SeverityHigh,
		RootCause:  "a migration ran twice",
		ActorID:    "human:felix",
		DetectedAt: detectedAt,
	}); err != nil {
		t.Fatalf("RecordIncident: %v", err)
	}

	incidents, err := adapter.GetIncidentHistory(ctx, "acme/widget", 10)
	if err != nil {
		t.Fatalf("GetIncidentHistory: %v", err)
	}
	if len(incidents) != 1 {
		t.Fatalf("read back %d incidents, want 1", len(incidents))
	}
	got := incidents[0]

	if got.ID != "inc-3" {
		t.Errorf("ID = %q, want inc-3", got.ID)
	}
	if got.ReleaseID != "run-7" {
		t.Errorf("ReleaseID = %q, want run-7; without it an incident cannot be tied to the "+
			"release that caused it, which is what change failure rate asks", got.ReleaseID)
	}
	if got.Severity != cgp.SeverityHigh {
		t.Errorf("Severity = %q, want high", got.Severity)
	}
	if !got.DetectedAt.Equal(detectedAt) {
		t.Errorf("DetectedAt = %v, want %v; a zero time makes MTTR meaningless",
			got.DetectedAt, detectedAt)
	}
}

// A malformed or absent timestamp yields the zero time, not now. Substituting now would
// date a years-old release to this moment and quietly corrupt every interval computed
// from it — a wrong answer is worse here than an obviously missing one.
func TestAMalformedTimestampDoesNotBecomeNow(t *testing.T) {
	for _, ts := range []string{"", "yesterday", "2026-13-45"} {
		if got := metaTime(ts); !got.IsZero() {
			t.Errorf("metaTime(%q) = %v, want the zero time", ts, got)
		}
	}
}
