package memory

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	release "github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// Three defects, all of which appeared the moment release domain events actually reached
// this tracker:
//
//  1. RecordRelease appended unconditionally, so one release run could produce two
//     records — the tracker's, from the event, and the CLI's recordPublishOutcome, from
//     the same run — inflating deployment frequency and counting the actor twice.
//
//  2. A cancellation was recorded as OutcomePartial, which IsNegative treats as a problem
//     and Accumulate counts as a failed release. Deciding not to ship therefore damaged
//     the actor's reliability and raised change failure rate: the governance gate working
//     as intended looked like a defect.
//
//  3. The per-run context cache is per-process, and the CLI runs one command per process.
//     `relicta cancel` raises a lone RunCanceledEvent, so there was no cached repository
//     and the store rejected every such record with "repository is required" — a warning
//     in a log nobody reads, and no record at all.

func recordFor(id, repo string, outcome ReleaseOutcome, actor string) *ReleaseRecord {
	return &ReleaseRecord{
		ID:         id,
		Repository: repo,
		Version:    "1.0.0",
		Actor:      cgp.Actor{Kind: cgp.ActorKindHuman, ID: actor},
		Outcome:    outcome,
		RiskScore:  0.2,
		ReleasedAt: time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC),
	}
}

func TestRecordingTheSameRunTwiceKeepsOneRecord(t *testing.T) {
	for name, store := range map[string]Store{
		"in-memory": NewInMemoryStore(),
		"file":      newFileStoreIn(t),
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			first := recordFor("run-1", "acme/widget", OutcomeSuccess, "human:alice")
			if err := store.RecordRelease(ctx, first); err != nil {
				t.Fatalf("RecordRelease(first): %v", err)
			}

			// The same run, recorded again with the richer data the CLI resolves after
			// publishing. This is the real sequence: the tracker writes during the
			// publish use case, recordPublishOutcome writes after it returns.
			second := recordFor("run-1", "acme/widget", OutcomeSuccess, "human:alice")
			second.RiskScore = 0.55
			if err := store.RecordRelease(ctx, second); err != nil {
				t.Fatalf("RecordRelease(second): %v", err)
			}

			records, err := store.GetReleaseHistory(ctx, "acme/widget", 10)
			if err != nil {
				t.Fatalf("GetReleaseHistory: %v", err)
			}
			if len(records) != 1 {
				t.Fatalf("found %d records for one run, want 1: two writers for one release "+
					"double every rate derived from it", len(records))
			}
			if records[0].RiskScore != 0.55 {
				t.Errorf("RiskScore = %v, want 0.55: the later, richer record should win",
					records[0].RiskScore)
			}

			// Metrics must not have counted the release twice.
			metrics, err := store.GetActorMetrics(ctx, "human:alice")
			if err != nil {
				t.Fatalf("GetActorMetrics: %v", err)
			}
			if metrics.TotalReleases != 1 {
				t.Errorf("TotalReleases = %d, want 1: the replaced record was folded in on top "+
					"of the one it replaced", metrics.TotalReleases)
			}
		})
	}
}

func TestACanceledRunIsNotCountedAsARelease(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	if err := store.RecordRelease(ctx, recordFor("run-ok", "acme/widget", OutcomeSuccess, "human:alice")); err != nil {
		t.Fatalf("RecordRelease(success): %v", err)
	}
	if err := store.RecordRelease(ctx, recordFor("run-cancel", "acme/widget", OutcomeCanceled, "human:alice")); err != nil {
		t.Fatalf("RecordRelease(canceled): %v", err)
	}

	// Still in the history: it is an audit record of a decision someone made.
	records, err := store.GetReleaseHistory(ctx, "acme/widget", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("found %d records, want 2: the cancellation should be recorded, just not counted",
			len(records))
	}

	metrics, err := store.GetActorMetrics(ctx, "human:alice")
	if err != nil {
		t.Fatalf("GetActorMetrics: %v", err)
	}
	if metrics.TotalReleases != 1 {
		t.Errorf("TotalReleases = %d, want 1: a canceled run entered the release population",
			metrics.TotalReleases)
	}
	if metrics.FailedReleases != 0 {
		t.Errorf("FailedReleases = %d, want 0: canceling a release was counted as failing one, "+
			"so using the governance gate lowered the actor's reliability",
			metrics.FailedReleases)
	}
	if metrics.SuccessRate != 1 {
		t.Errorf("SuccessRate = %v, want 1: the cancellation stayed in the denominator",
			metrics.SuccessRate)
	}
}

func TestCanceledIsNotANegativeOutcome(t *testing.T) {
	if OutcomeCanceled.IsNegative() {
		t.Error("OutcomeCanceled.IsNegative() = true: change failure rate counts negative " +
			"outcomes, and a release nobody shipped is not a failure")
	}
	if OutcomeCanceled.CountsAsRelease() {
		t.Error("OutcomeCanceled.CountsAsRelease() = true: it would land in the denominator " +
			"of every rate computed over releases")
	}
	for _, o := range []ReleaseOutcome{OutcomeSuccess, OutcomeFailed, OutcomeRollback, OutcomePartial} {
		if !o.CountsAsRelease() {
			t.Errorf("%q.CountsAsRelease() = false, want true", o)
		}
	}
	if !OutcomeCanceled.IsValid() {
		t.Error("OutcomeCanceled.IsValid() = false: a value the tracker writes must validate")
	}
}

// A lone terminal event is the normal case for this CLI, not an edge case: every command
// is its own process, so the process that cancels a run never saw it created.
func TestALoneCancelEventIsRecordedInAFreshProcess(t *testing.T) {
	store := NewInMemoryStore()
	tracker := NewOutcomeTracker(store, nil, "acme/widget")

	err := tracker.Publish(context.Background(), &release.RunCanceledEvent{
		RunID:   "run-lonely",
		Reason:  "changed my mind",
		By:      "alice",
		At:      time.Now().UTC(),
		Version: "2.3.0",
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	records, err := store.GetReleaseHistory(context.Background(), "acme/widget", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("found %d records, want 1: with no cached context the record had no "+
			"repository and the store rejected it, so canceling recorded nothing at all",
			len(records))
	}

	got := records[0]
	if got.Outcome != OutcomeCanceled {
		t.Errorf("Outcome = %q, want %q", got.Outcome, OutcomeCanceled)
	}
	if got.Version != "2.3.0" {
		t.Errorf("Version = %q, want 2.3.0: the event carries it because the process that "+
			"calculated the version has already exited", got.Version)
	}
	if got.Actor.ID != "human:alice" {
		t.Errorf("Actor.ID = %q, want human:alice: recorded bare, this actor is a second "+
			"identity holding half of alice's history", got.Actor.ID)
	}
}

// A failed run must name the version that failed, in a fresh process too — that half of
// the history is what change failure rate is computed from.
func TestALoneFailureEventKeepsItsVersion(t *testing.T) {
	store := NewInMemoryStore()
	tracker := NewOutcomeTracker(store, nil, "acme/widget")

	err := tracker.Publish(context.Background(), &release.RunFailedEvent{
		RunID:   "run-failed",
		Reason:  "npm publish rejected the tarball",
		At:      time.Now().UTC(),
		Version: "4.1.0",
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	records, err := store.GetReleaseHistory(context.Background(), "acme/widget", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("found %d records, want 1", len(records))
	}
	if records[0].Version != "4.1.0" {
		t.Errorf("Version = %q, want 4.1.0", records[0].Version)
	}
	if records[0].Outcome != OutcomeFailed {
		t.Errorf("Outcome = %q, want failed", records[0].Outcome)
	}
}

// The cached context still wins when there is one: it came from this run's own created
// event, and the fallback is a single value for the whole process.
func TestTheRunsOwnRepositoryBeatsTheFallback(t *testing.T) {
	store := NewInMemoryStore()
	tracker := NewOutcomeTracker(store, nil, "acme/fallback")

	ctx := context.Background()
	if err := tracker.Publish(ctx,
		&release.RunCreatedEvent{RunID: "run-x", RepoID: "https://github.com/acme/real.git", At: time.Now().Add(-time.Hour)},
		&release.RunPublishedEvent{RunID: "run-x", Version: mustParseVersion(t, "1.0.0"), At: time.Now()},
	); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if records, err := store.GetReleaseHistory(ctx, "acme/real", 10); err != nil {
		t.Fatalf("GetReleaseHistory: %v", err)
	} else if len(records) != 1 {
		t.Errorf("found %d records under the run's own repository, want 1", len(records))
	}
	if records, err := store.GetReleaseHistory(ctx, "acme/fallback", 10); err != nil {
		t.Fatalf("GetReleaseHistory(fallback): %v", err)
	} else if len(records) != 0 {
		t.Errorf("%d records were filed under the process-wide fallback instead of the run's "+
			"own repository", len(records))
	}
}

func mustParseVersion(t *testing.T, v string) version.SemanticVersion {
	t.Helper()
	parsed, err := version.Parse(v)
	if err != nil {
		t.Fatalf("version.Parse(%q): %v", v, err)
	}
	return parsed
}

func newFileStoreIn(t *testing.T) *FileStore {
	t.Helper()
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return store
}
