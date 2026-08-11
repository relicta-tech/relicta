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

// GetActorMetrics counted releases and stopped, under "Simplified - full implementation
// would parse all fields". So an actor with nothing but failures showed a flawless
// record — and these numbers decide whether that actor's next change is auto-approved.
func TestActorMetricsCountOutcomes(t *testing.T) {
	srv, _ := mnemosServer(t)
	adapter := NewMnemosStore(srv.URL, "run-ns", srv.Client())
	ctx := context.Background()

	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	outcomes := []memory.ReleaseOutcome{
		memory.OutcomeSuccess,
		memory.OutcomeSuccess,
		memory.OutcomeFailed,
		memory.OutcomeRollback,
	}
	for i, outcome := range outcomes {
		if err := adapter.RecordRelease(ctx, &memory.ReleaseRecord{
			ID:         "run-" + string(rune('a'+i)),
			Repository: "acme/widget",
			Version:    "1.0." + string(rune('0'+i)),
			Actor:      cgp.Actor{ID: "agent:codex", Kind: cgp.ActorKindAgent},
			Outcome:    outcome,
			RiskScore:  0.8,
			ReleasedAt: base.Add(time.Duration(i) * time.Hour),
		}); err != nil {
			t.Fatalf("RecordRelease: %v", err)
		}
	}

	metrics, err := adapter.GetActorMetrics(ctx, "agent:codex")
	if err != nil {
		t.Fatalf("GetActorMetrics: %v", err)
	}

	if metrics.TotalReleases != 4 {
		t.Fatalf("TotalReleases = %d, want 4", metrics.TotalReleases)
	}
	if metrics.SuccessfulReleases != 2 {
		t.Errorf("SuccessfulReleases = %d, want 2", metrics.SuccessfulReleases)
	}
	// A rollback counts as a failure too: the change reached users and was withdrawn.
	if metrics.FailedReleases != 2 {
		t.Errorf("FailedReleases = %d, want 2 (one failed, one rolled back); 0 means outcomes "+
			"are still not parsed and a failing actor reads as flawless", metrics.FailedReleases)
	}
	if metrics.RollbackCount != 1 {
		t.Errorf("RollbackCount = %d, want 1", metrics.RollbackCount)
	}
	if metrics.SuccessRate < 0.49 || metrics.SuccessRate > 0.51 {
		t.Errorf("SuccessRate = %.2f, want 0.50; this feeds the governance decision's "+
			"historical context", metrics.SuccessRate)
	}
	if metrics.HighRiskReleases != 4 {
		t.Errorf("HighRiskReleases = %d, want 4 (all at 0.8)", metrics.HighRiskReleases)
	}
	// The attribution a governance audit exists to make legible: an agent recorded as a
	// human is the wrong answer, and it used to be hardcoded.
	if metrics.ActorKind != cgp.ActorKindAgent {
		t.Errorf("ActorKind = %q, want agent; it was defaulted to human regardless of what "+
			"the records said", metrics.ActorKind)
	}
}

// GetRiskPatterns returned a hardcoded zero struct, which fed
// HistoricalContext.AverageRiskScore and RiskTrend straight into risk evaluation. A
// fabricated zero asserts this repository has never shipped anything risky.
func TestRiskPatternsAreComputed(t *testing.T) {
	srv, _ := mnemosServer(t)
	adapter := NewMnemosStore(srv.URL, "run-ns", srv.Client())
	ctx := context.Background()

	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	// Risk climbing over time: 0.1, 0.2, 0.8, 0.9.
	for i, score := range []float64{0.1, 0.2, 0.8, 0.9} {
		if err := adapter.RecordRelease(ctx, &memory.ReleaseRecord{
			ID:         "run-" + string(rune('a'+i)),
			Repository: "acme/widget",
			Version:    "2.0." + string(rune('0'+i)),
			Actor:      cgp.Actor{ID: "human:felix", Kind: cgp.ActorKindHuman},
			Outcome:    memory.OutcomeSuccess,
			RiskScore:  score,
			ReleasedAt: base.Add(time.Duration(i) * time.Hour),
		}); err != nil {
			t.Fatalf("RecordRelease: %v", err)
		}
	}

	patterns, err := adapter.GetRiskPatterns(ctx, "acme/widget")
	if err != nil {
		t.Fatalf("GetRiskPatterns: %v", err)
	}

	if patterns.TotalReleases != 4 {
		t.Fatalf("TotalReleases = %d, want 4; 0 was returned unconditionally before",
			patterns.TotalReleases)
	}
	if patterns.AverageRiskScore < 0.49 || patterns.AverageRiskScore > 0.51 {
		t.Errorf("AverageRiskScore = %.2f, want 0.50", patterns.AverageRiskScore)
	}
	// Risk rose from ~0.15 to ~0.85, and the answer must not depend on which order the
	// history happened to arrive in.
	if patterns.RiskTrend != memory.TrendIncreasing {
		t.Errorf("RiskTrend = %q, want increasing: risk climbed from 0.1 to 0.9 and a report "+
			"that calls that stable is telling an operator the opposite of the truth",
			patterns.RiskTrend)
	}
}
