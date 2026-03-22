package memory

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
)

func TestIncidentCorrelator_WithinWindow(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Record a release 30 minutes ago.
	releaseTime := time.Now().Add(-30 * time.Minute)
	err := store.RecordRelease(ctx, &ReleaseRecord{
		ID:         "rel-1",
		Repository: "owner/repo",
		Version:    "1.0.0",
		Outcome:    OutcomeSuccess,
		ReleasedAt: releaseTime,
		Actor:      cgp.NewHumanActor("dev", "Developer"),
		Metadata:   map[string]string{},
	})
	if err != nil {
		t.Fatalf("RecordRelease failed: %v", err)
	}

	correlator := NewIncidentCorrelator(store)

	// Incident started 10 minutes ago (20 min after release, within default 60 min window).
	incident := IncidentRecord{
		ID:         "inc-1",
		Repository: "owner/repo",
		Severity:   cgp.SeverityHigh,
		Type:       IncidentAvailability,
		DetectedAt: time.Now().Add(-10 * time.Minute),
	}

	releaseID, err := correlator.Correlate(ctx, incident)
	if err != nil {
		t.Fatalf("Correlate failed: %v", err)
	}
	if releaseID != "rel-1" {
		t.Errorf("expected release ID %q, got %q", "rel-1", releaseID)
	}

	// Verify the release outcome was updated to reflect the incident.
	history, err := store.GetReleaseHistory(ctx, "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory failed: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 release, got %d", len(history))
	}
	if history[0].Outcome != OutcomeRollback {
		t.Errorf("expected outcome %q after incident correlation, got %q", OutcomeRollback, history[0].Outcome)
	}
}

func TestIncidentCorrelator_OutsideWindow(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Record a release 2 hours ago.
	releaseTime := time.Now().Add(-2 * time.Hour)
	err := store.RecordRelease(ctx, &ReleaseRecord{
		ID:         "rel-old",
		Repository: "owner/repo",
		Version:    "1.0.0",
		Outcome:    OutcomeSuccess,
		ReleasedAt: releaseTime,
		Actor:      cgp.NewHumanActor("dev", "Developer"),
		Metadata:   map[string]string{},
	})
	if err != nil {
		t.Fatalf("RecordRelease failed: %v", err)
	}

	correlator := NewIncidentCorrelator(store) // default 60 min window

	// Incident started 10 minutes ago (release was 2 hours ago, outside window).
	incident := IncidentRecord{
		ID:         "inc-2",
		Repository: "owner/repo",
		Severity:   cgp.SeverityHigh,
		Type:       IncidentAvailability,
		DetectedAt: time.Now().Add(-10 * time.Minute),
	}

	releaseID, err := correlator.Correlate(ctx, incident)
	if err != nil {
		t.Fatalf("Correlate failed: %v", err)
	}
	if releaseID != "" {
		t.Errorf("expected empty release ID for out-of-window incident, got %q", releaseID)
	}
}

func TestIncidentCorrelator_MultipleReleases_PicksClosest(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Record two releases within the window.
	err := store.RecordRelease(ctx, &ReleaseRecord{
		ID:         "rel-older",
		Repository: "owner/repo",
		Version:    "1.0.0",
		Outcome:    OutcomeSuccess,
		ReleasedAt: time.Now().Add(-50 * time.Minute),
		Actor:      cgp.NewHumanActor("dev", "Developer"),
		Metadata:   map[string]string{},
	})
	if err != nil {
		t.Fatalf("RecordRelease failed: %v", err)
	}

	err = store.RecordRelease(ctx, &ReleaseRecord{
		ID:         "rel-closer",
		Repository: "owner/repo",
		Version:    "1.1.0",
		Outcome:    OutcomeSuccess,
		ReleasedAt: time.Now().Add(-15 * time.Minute),
		Actor:      cgp.NewHumanActor("dev", "Developer"),
		Metadata:   map[string]string{},
	})
	if err != nil {
		t.Fatalf("RecordRelease failed: %v", err)
	}

	correlator := NewIncidentCorrelator(store)

	incident := IncidentRecord{
		ID:         "inc-3",
		Repository: "owner/repo",
		Severity:   cgp.SeverityCritical,
		Type:       IncidentAvailability,
		DetectedAt: time.Now().Add(-5 * time.Minute),
	}

	releaseID, err := correlator.Correlate(ctx, incident)
	if err != nil {
		t.Fatalf("Correlate failed: %v", err)
	}
	if releaseID != "rel-closer" {
		t.Errorf("expected closest release %q, got %q", "rel-closer", releaseID)
	}
}

func TestIncidentCorrelator_ServiceNameMatching(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Record releases for different repos with service metadata.
	err := store.RecordRelease(ctx, &ReleaseRecord{
		ID:         "rel-svc-a",
		Repository: "owner/service-a",
		Version:    "1.0.0",
		Outcome:    OutcomeSuccess,
		ReleasedAt: time.Now().Add(-20 * time.Minute),
		Actor:      cgp.NewHumanActor("dev", "Developer"),
		Metadata:   map[string]string{"service": "payment-service"},
	})
	if err != nil {
		t.Fatalf("RecordRelease failed: %v", err)
	}

	err = store.RecordRelease(ctx, &ReleaseRecord{
		ID:         "rel-svc-b",
		Repository: "owner/service-b",
		Version:    "2.0.0",
		Outcome:    OutcomeSuccess,
		ReleasedAt: time.Now().Add(-10 * time.Minute),
		Actor:      cgp.NewHumanActor("dev", "Developer"),
		Metadata:   map[string]string{"service": "order-service"},
	})
	if err != nil {
		t.Fatalf("RecordRelease failed: %v", err)
	}

	correlator := NewIncidentCorrelator(store)

	// Incident for payment-service should match rel-svc-a, not rel-svc-b.
	incident := IncidentRecord{
		ID:         "inc-svc",
		Repository: "owner/service-a",
		Severity:   cgp.SeverityHigh,
		Type:       IncidentAvailability,
		DetectedAt: time.Now().Add(-5 * time.Minute),
	}

	releaseID, err := correlator.Correlate(ctx, incident)
	if err != nil {
		t.Fatalf("Correlate failed: %v", err)
	}
	if releaseID != "rel-svc-a" {
		t.Errorf("expected %q for service match, got %q", "rel-svc-a", releaseID)
	}
}

func TestIncidentCorrelator_NoReleases(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	correlator := NewIncidentCorrelator(store)

	incident := IncidentRecord{
		ID:         "inc-lonely",
		Repository: "owner/empty-repo",
		Severity:   cgp.SeverityLow,
		Type:       IncidentOther,
		DetectedAt: time.Now(),
	}

	releaseID, err := correlator.Correlate(ctx, incident)
	if err != nil {
		t.Fatalf("Correlate failed: %v", err)
	}
	if releaseID != "" {
		t.Errorf("expected empty release ID when no releases exist, got %q", releaseID)
	}
}

func TestIncidentCorrelator_CustomWindow(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Record a release 45 minutes ago.
	err := store.RecordRelease(ctx, &ReleaseRecord{
		ID:         "rel-window",
		Repository: "owner/repo",
		Version:    "1.0.0",
		Outcome:    OutcomeSuccess,
		ReleasedAt: time.Now().Add(-45 * time.Minute),
		Actor:      cgp.NewHumanActor("dev", "Developer"),
		Metadata:   map[string]string{},
	})
	if err != nil {
		t.Fatalf("RecordRelease failed: %v", err)
	}

	// With a 30-minute window the release is outside the window.
	correlator := NewIncidentCorrelator(store, WithCorrelationWindow(30*time.Minute))

	incident := IncidentRecord{
		ID:         "inc-window",
		Repository: "owner/repo",
		Severity:   cgp.SeverityMedium,
		Type:       IncidentBugIntro,
		DetectedAt: time.Now(),
	}

	releaseID, err := correlator.Correlate(ctx, incident)
	if err != nil {
		t.Fatalf("Correlate failed: %v", err)
	}
	if releaseID != "" {
		t.Errorf("expected empty release ID with 30min window, got %q", releaseID)
	}

	// With a 60-minute window the same release should match.
	correlator2 := NewIncidentCorrelator(store, WithCorrelationWindow(60*time.Minute))

	releaseID2, err := correlator2.Correlate(ctx, incident)
	if err != nil {
		t.Fatalf("Correlate failed: %v", err)
	}
	if releaseID2 != "rel-window" {
		t.Errorf("expected %q with 60min window, got %q", "rel-window", releaseID2)
	}
}

func TestIncidentCorrelator_ReleaseMustBeBeforeIncident(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Record a release in the future relative to the incident.
	err := store.RecordRelease(ctx, &ReleaseRecord{
		ID:         "rel-future",
		Repository: "owner/repo",
		Version:    "1.0.0",
		Outcome:    OutcomeSuccess,
		ReleasedAt: time.Now().Add(10 * time.Minute),
		Actor:      cgp.NewHumanActor("dev", "Developer"),
		Metadata:   map[string]string{},
	})
	if err != nil {
		t.Fatalf("RecordRelease failed: %v", err)
	}

	correlator := NewIncidentCorrelator(store)

	incident := IncidentRecord{
		ID:         "inc-before-release",
		Repository: "owner/repo",
		Severity:   cgp.SeverityHigh,
		Type:       IncidentAvailability,
		DetectedAt: time.Now(),
	}

	releaseID, err := correlator.Correlate(ctx, incident)
	if err != nil {
		t.Fatalf("Correlate failed: %v", err)
	}
	if releaseID != "" {
		t.Errorf("expected no match when release is after incident, got %q", releaseID)
	}
}

func TestIncidentCorrelator_RecordsIncidentInStore(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	err := store.RecordRelease(ctx, &ReleaseRecord{
		ID:         "rel-inc-test",
		Repository: "owner/repo",
		Version:    "1.0.0",
		Outcome:    OutcomeSuccess,
		ReleasedAt: time.Now().Add(-20 * time.Minute),
		Actor:      cgp.NewHumanActor("dev", "Developer"),
		Metadata:   map[string]string{},
	})
	if err != nil {
		t.Fatalf("RecordRelease failed: %v", err)
	}

	correlator := NewIncidentCorrelator(store)

	incident := IncidentRecord{
		ID:          "inc-recorded",
		Repository:  "owner/repo",
		Severity:    cgp.SeverityCritical,
		Type:        IncidentAvailability,
		Description: "service down",
		DetectedAt:  time.Now().Add(-5 * time.Minute),
	}

	_, err = correlator.Correlate(ctx, incident)
	if err != nil {
		t.Fatalf("Correlate failed: %v", err)
	}

	// Verify incident was recorded in the store.
	incidents, err := store.GetIncidentHistory(ctx, "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetIncidentHistory failed: %v", err)
	}
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident recorded, got %d", len(incidents))
	}
	if incidents[0].ReleaseID != "rel-inc-test" {
		t.Errorf("expected incident linked to %q, got %q", "rel-inc-test", incidents[0].ReleaseID)
	}
	if incidents[0].Description != "service down" {
		t.Errorf("expected description %q, got %q", "service down", incidents[0].Description)
	}
}
