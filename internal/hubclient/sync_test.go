package hubclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

func sampleRecord() *memory.ReleaseRecord {
	released := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	return &memory.ReleaseRecord{
		ID:              "run-abc123",
		Repository:      "acme/widget",
		Version:         "1.4.0",
		Actor:           cgp.Actor{Kind: cgp.ActorKindHuman, ID: "alice@example.com"},
		RiskScore:       0.42,
		Decision:        cgp.DecisionApproved,
		BreakingChanges: 2,
		SecurityChanges: 1,
		FilesChanged:    9,
		LinesChanged:    120,
		Outcome:         memory.OutcomeSuccess,
		Duration:        90 * time.Second,
		ReleasedAt:      released,
		FirstCommitAt:   released.Add(-72 * time.Hour),
	}
}

func eventByType(t *testing.T, events []Event, eventType string) Event {
	t.Helper()
	for _, e := range events {
		if e.Type == eventType {
			return e
		}
	}
	t.Fatalf("no %s event in %d event(s)", eventType, len(events))
	return Event{}
}

// Hub builds its release row from release.planned — the only branch that reads the
// risk score, commit count and breaking flag — then completes it with the outcome.
// Sending only the outcome produced a row with no risk data at all, which is most of
// what the governance dashboard exists to show.
func TestEachRecordSendsAPlanAndAnOutcome(t *testing.T) {
	events := EventsFromReleases("org-1", []*memory.ReleaseRecord{sampleRecord()})

	if len(events) != 2 {
		t.Fatalf("got %d events for one record, want 2 (planned + outcome)", len(events))
	}

	plan := eventByType(t, events, "release.planned")
	if plan.Data["risk_score"] != 0.42 {
		t.Errorf("planned risk_score = %v, want 0.42: Hub reads the risk score only from "+
			"the planned event, so an outcome-only sync leaves every release unscored",
			plan.Data["risk_score"])
	}
	if plan.Data["commit_count"] != 9 {
		t.Errorf("planned commit_count = %v, want 9", plan.Data["commit_count"])
	}
	if plan.Data["has_breaking"] != true {
		t.Errorf("planned has_breaking = %v, want true (record has 2 breaking changes)",
			plan.Data["has_breaking"])
	}
	if plan.Data["next_version"] != "1.4.0" {
		t.Errorf("planned next_version = %v, want 1.4.0: Hub reads the version from "+
			"next_version on the planned event, not version", plan.Data["next_version"])
	}

	outcome := eventByType(t, events, "release.published")
	if outcome.Data["version"] != "1.4.0" {
		t.Errorf("outcome version = %v, want 1.4.0", outcome.Data["version"])
	}
}

// Hub keys its release row on release_id. An event without one materializes a row
// with an empty ID that no later event can find again, so the outcome silently fails
// to attach and the release shows as permanently planned.
func TestEveryEventCarriesTheReleaseID(t *testing.T) {
	events := EventsFromReleases("org-1", []*memory.ReleaseRecord{sampleRecord()})

	for _, e := range events {
		if got := e.Data["release_id"]; got != "run-abc123" {
			t.Errorf("%s: release_id = %v, want run-abc123", e.Type, got)
		}
	}
}

// Hub's lead time is PublishedAt - PlannedAt, and PlannedAt is this event's
// timestamp. Dating the plan from the oldest commit is what makes that subtraction
// measure how long a change waited to reach users, rather than how long relicta's own
// release command took to run — the exact defect already fixed in the local DORA
// report. Both sides must answer the same question about the same release.
func TestThePlanIsDatedFromTheOldestCommit(t *testing.T) {
	rec := sampleRecord()
	events := EventsFromReleases("org-1", []*memory.ReleaseRecord{rec})

	plan := eventByType(t, events, "release.planned")
	if !plan.Timestamp.Equal(rec.FirstCommitAt) {
		t.Errorf("planned timestamp = %s, want %s (the oldest commit): dating it from the "+
			"release run makes Hub's lead time measure the release process, not the change's wait",
			plan.Timestamp, rec.FirstCommitAt)
	}

	published := eventByType(t, events, "release.published")
	if !published.Timestamp.Equal(rec.ReleasedAt) {
		t.Errorf("published timestamp = %s, want %s", published.Timestamp, rec.ReleasedAt)
	}
}

// A record from before FirstCommitAt was tracked has a zero value. Sending that as
// the plan date would make lead time the age of the Unix epoch and rate every
// project catastrophically slow, so it falls back to the release date (lead time
// zero — understated, but not nonsense).
func TestAMissingFirstCommitFallsBackToTheReleaseDate(t *testing.T) {
	rec := sampleRecord()
	rec.FirstCommitAt = time.Time{}

	plan := eventByType(t, EventsFromReleases("org-1", []*memory.ReleaseRecord{rec}), "release.planned")
	if !plan.Timestamp.Equal(rec.ReleasedAt) {
		t.Errorf("planned timestamp = %s, want the release date %s for a record with no "+
			"recorded first commit", plan.Timestamp, rec.ReleasedAt)
	}
}

// IDs are derived from the record, never generated: Hub's SaveEvent is ON CONFLICT
// (id) DO NOTHING, so a stable ID is what makes re-syncing the same history a no-op
// instead of duplicating every release on each run.
func TestSyncingTwiceDerivesTheSameIDs(t *testing.T) {
	first := EventsFromReleases("org-1", []*memory.ReleaseRecord{sampleRecord()})
	second := EventsFromReleases("org-1", []*memory.ReleaseRecord{sampleRecord()})

	if len(first) != len(second) {
		t.Fatalf("event counts differ between runs: %d then %d", len(first), len(second))
	}
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Errorf("event %d ID differs between runs: %q then %q — a generated ID would "+
				"duplicate the whole history on every sync", i, first[i].ID, second[i].ID)
		}
	}

	// The plan and the outcome of one release must not collide, or one overwrites the other.
	if first[0].ID == first[1].ID {
		t.Errorf("plan and outcome share the ID %q, so Hub's idempotency drops one of them", first[0].ID)
	}
}

// A rollback reached users and was withdrawn. Reporting it as published would count
// it as a success in every rate derived from it — change failure rate most of all,
// which exists to count exactly this.
func TestAWithdrawnReleaseIsNotReportedAsPublished(t *testing.T) {
	for _, outcome := range []memory.ReleaseOutcome{
		memory.OutcomeFailed, memory.OutcomeRollback, memory.OutcomePartial,
	} {
		rec := sampleRecord()
		rec.Outcome = outcome

		events := EventsFromReleases("org-1", []*memory.ReleaseRecord{rec})
		if got := eventByType(t, events, "release.failed"); got.ID == "" {
			t.Errorf("outcome %q produced no release.failed event", outcome)
		}
		for _, e := range events {
			if e.Type == "release.published" {
				t.Errorf("outcome %q was reported as release.published", outcome)
			}
		}
	}
}

// The wire contract: Hub rejects an unknown non-empty X-CGP-Version with 412 rather
// than storing a mis-encoded batch, so the header has to actually be sent.
func TestSyncSendsTheVersionHeaderAndBearerToken(t *testing.T) {
	var gotVersion, gotAuth string
	var gotEvents []Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.Header.Get("X-CGP-Version")
		gotAuth = r.Header.Get("Authorization")

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotEvents)

		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"accepted":1,"received":1,"results":[{"id":"run-abc123:release.planned","status":"accepted"}]}`))
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	events := EventsFromReleases("org-1", []*memory.ReleaseRecord{sampleRecord()})

	if _, err := client.SyncEvents(context.Background(), "token-xyz", events); err != nil {
		t.Fatalf("SyncEvents: %v", err)
	}

	if gotVersion != cgpWireVersion {
		t.Errorf("X-CGP-Version = %q, want %q: an unsent header makes Hub fall back to its "+
			"alpha-era tolerance instead of negotiating", gotVersion, cgpWireVersion)
	}
	if gotAuth != "Bearer token-xyz" {
		t.Errorf("Authorization = %q, want the bearer token", gotAuth)
	}
	if len(gotEvents) != 2 {
		t.Errorf("Hub received %d events, want 2", len(gotEvents))
	}
}

// A 207 means Hub kept some events and refused others. Reporting it as success would
// hide a rejected event behind a green checkmark, which for an audit trail is the
// difference between a complete record and a plausible one.
func TestAPartialSyncSurfacesTheRejections(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`{"accepted":1,"received":2,"results":[
			{"id":"a","status":"accepted"},
			{"id":"b","status":"rejected","error":"org_id does not match the authenticated organization"}
		]}`))
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}

	result, err := client.SyncEvents(context.Background(), "token-xyz",
		EventsFromReleases("org-1", []*memory.ReleaseRecord{sampleRecord()}))
	if err != nil {
		t.Fatalf("SyncEvents on 207: %v", err)
	}

	rejected := result.Rejected()
	if len(rejected) != 1 {
		t.Fatalf("Rejected() returned %d entries, want 1: %+v", len(rejected), result.Results)
	}
	if !containsAll(rejected[0], "org_id") {
		t.Errorf("the rejection %q does not carry Hub's reason, so the operator cannot tell "+
			"why the record is incomplete", rejected[0])
	}
	if result.Accepted != 1 {
		t.Errorf("Accepted = %d, want 1", result.Accepted)
	}
}

// A version mismatch is the one error the CLI can act on by upgrading, so it must
// not arrive as a generic failure.
func TestAVersionMismatchIsReportedAsSuch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-CGP-Supported-Versions", "2.0")
		w.WriteHeader(http.StatusPreconditionFailed)
		_, _ = w.Write([]byte(`{"error":"unsupported X-CGP-Version: 1.0","supported_versions":["2.0"]}`))
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}

	_, err := client.SyncEvents(context.Background(), "token-xyz",
		EventsFromReleases("org-1", []*memory.ReleaseRecord{sampleRecord()}))
	if err == nil {
		t.Fatal("a 412 returned no error, so a mis-encoded sync looks like a successful one")
	}
	if !containsAll(err.Error(), "version", "2.0") {
		t.Errorf("error %q does not name the version problem or the supported version, so the "+
			"operator cannot tell an upgrade would fix it", err)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// A canceled run must not reach Hub at all.
//
// Hub's event vocabulary is planned/published/failed, and eventTypeFor's default is
// release.published — so once OutcomeCanceled existed locally, every cancellation would have
// arrived at Hub as a successful release. That is the one outcome the local report goes out
// of its way to exclude from deployment frequency, so reporting it as a deployment would make
// Hub's numbers disagree with relicta's own about the same repository.
func TestACanceledRunIsNotReportedToHub(t *testing.T) {
	canceled := sampleRecord()
	canceled.Outcome = memory.OutcomeCanceled

	events := EventsFromReleases("org-1", []*memory.ReleaseRecord{canceled})

	if len(events) != 0 {
		var types []string
		for _, e := range events {
			types = append(types, e.Type)
		}
		t.Errorf("a canceled run produced %v; Hub has no term for a run that never shipped, "+
			"so any event here is a false statement about it", types)
	}
}

// And a canceled run among real ones must not take them with it.
func TestACanceledRunDoesNotSuppressTheReleasesAroundIt(t *testing.T) {
	canceled := sampleRecord()
	canceled.ID = "run-canceled"
	canceled.Outcome = memory.OutcomeCanceled

	events := EventsFromReleases("org-1", []*memory.ReleaseRecord{
		canceled, sampleRecord(),
	})

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 (the plan and outcome of the one real release)", len(events))
	}
	for _, e := range events {
		if e.Data["release_id"] == "run-canceled" {
			t.Errorf("the canceled run leaked into the batch as %s", e.Type)
		}
	}
}
