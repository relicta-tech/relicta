package compliance

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

// Canceled runs became recordable once release domain events reached the outcome tracker.
// They are in the store for audit and must stay out of every rate computed over releases:
// a run nobody shipped is neither a deployment nor a change whose failure rate this
// measures. Left in the denominator, each cancellation quietly improves change failure rate
// and depresses deployment frequency — a team that reviews carefully would score worse than
// one that ships everything.

func releaseRecordAt(id string, outcome memory.ReleaseOutcome, at time.Time) *memory.ReleaseRecord {
	return &memory.ReleaseRecord{
		ID:         id,
		Repository: "acme/widget",
		Version:    "1.0.0",
		Actor:      cgp.Actor{Kind: cgp.ActorKindHuman, ID: "human:alice"},
		Outcome:    outcome,
		ReleasedAt: at,
	}
}

func TestCanceledRunsLeaveTheDORAPopulation(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()

	// Two real releases, one of which failed, plus six cancellations.
	if err := store.RecordRelease(ctx, releaseRecordAt("ok-1", memory.OutcomeSuccess, now.Add(-72*time.Hour))); err != nil {
		t.Fatalf("RecordRelease: %v", err)
	}
	if err := store.RecordRelease(ctx, releaseRecordAt("bad-1", memory.OutcomeFailed, now.Add(-48*time.Hour))); err != nil {
		t.Fatalf("RecordRelease: %v", err)
	}
	for i := range 6 {
		id := "canceled-" + string(rune('a'+i))
		if err := store.RecordRelease(ctx, releaseRecordAt(id, memory.OutcomeCanceled, now.Add(-24*time.Hour))); err != nil {
			t.Fatalf("RecordRelease(%s): %v", id, err)
		}
	}

	// No deployments reported, so both metrics fall back to releases — the path where
	// the denominator matters.
	dora := generateDORA(t, store, "production")

	// One of two releases failed: 50%. With the cancellations counted it would be
	// 1 in 8 — 12.5%, and a different DORA classification.
	if got := dora.ChangeFailureRate.Rate * 100; got < 49 || got > 51 {
		t.Errorf("ChangeFailureRate = %.1f%%, want ~50%%: six canceled runs were counted as "+
			"changes, so canceling releases improved the failure rate", got)
	}

	if dora.DeploymentFrequency.TotalDeployments != 2 {
		t.Errorf("TotalDeployments = %d, want 2: canceled runs were counted as deployments "+
			"even though nothing reached users", dora.DeploymentFrequency.TotalDeployments)
	}
}

// The actor summary in the governance summary report has the same denominator, and an actor who only
// ever canceled should not be listed as having released anything.
func TestCanceledRunsDoNotDepressAnActorsSuccessRate(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()

	if err := store.RecordRelease(ctx, releaseRecordAt("ok-1", memory.OutcomeSuccess, now.Add(-48*time.Hour))); err != nil {
		t.Fatalf("RecordRelease: %v", err)
	}
	if err := store.RecordRelease(ctx, releaseRecordAt("canceled-1", memory.OutcomeCanceled, now.Add(-24*time.Hour))); err != nil {
		t.Fatalf("RecordRelease: %v", err)
	}

	report, err := NewGenerator(store, nil).Generate(ctx, ReportConfig{
		Type:       ReportSummary,
		Format:     FormatJSON,
		Period:     Period{Start: now.Add(-30 * 24 * time.Hour), End: now.Add(time.Hour)},
		Repository: "acme/widget",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(report.Summary.ActorActivity) != 1 {
		t.Fatalf("got %d actors, want 1", len(report.Summary.ActorActivity))
	}
	actor := report.Summary.ActorActivity[0]
	if actor.ReleaseCount != 1 {
		t.Errorf("ReleaseCount = %d, want 1: the cancellation was counted as a release",
			actor.ReleaseCount)
	}
	if actor.SuccessRate != 1 {
		t.Errorf("SuccessRate = %v, want 1: the cancellation sat in the denominator, so "+
			"declining to ship looked like a failure to ship", actor.SuccessRate)
	}
}
