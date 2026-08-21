package observability

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/observability/receiver"
)

// Incidents used to be appended to a slice on the service: lost when the process ended, and
// appended to from whichever HTTP handler goroutine received the webhook while the
// correlations endpoint read it. They now go through the governance memory — the same store
// the health monitor writes to and correlation reads release history from.

func serviceWithStore(t *testing.T) (*Service, *memory.InMemoryStore) {
	t.Helper()

	store := memory.NewInMemoryStore()
	svc, err := NewService(config.ObservabilityConfig{
		Providers: []config.ObservabilityProviderConfig{
			{Name: "prod", Type: "prometheus", Endpoint: "http://localhost:9090"},
		},
	}, store, "github.com/acme/api")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc, store
}

func TestAnIncidentSurvivesTheProcessThatHeardIt(t *testing.T) {
	svc, store := serviceWithStore(t)

	svc.recordIncident(context.Background(), receiver.Incident{
		ID:          "alert-1",
		Source:      "alertmanager",
		Name:        "HighErrorRate",
		Severity:    "critical",
		Description: "error rate above 10% for 5m",
		StartedAt:   time.Now(),
	})

	history, err := store.GetIncidentHistory(context.Background(), "github.com/acme/api", 10)
	if err != nil {
		t.Fatalf("GetIncidentHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("stored %d incidents, want 1", len(history))
	}
	if history[0].Severity != cgp.SeverityCritical {
		t.Errorf("severity = %q, want critical", history[0].Severity)
	}
	if got := history[0].Description; got != "HighErrorRate: error rate above 10% for 5m" {
		t.Errorf("description = %q, which is not what the alert said", got)
	}
}

// An unrecognized severity is neither dropped nor inflated: the incident is real whatever the
// provider called it.
func TestAnUnknownSeverityBecomesMedium(t *testing.T) {
	// A slice rather than a map because two of these differ only in case and padding, which
	// is the point: providers do not agree on how to spell a severity.
	cases := []struct {
		input string
		want  cgp.Severity
	}{
		{"critical", cgp.SeverityCritical},
		{"warning", cgp.SeverityMedium},
		{"info", cgp.SeverityLow},
		{"P2", cgp.SeverityMedium},
		{"", cgp.SeverityMedium},
		{"CRITICAL\t", cgp.SeverityCritical},
	}
	for _, c := range cases {
		if got := severityFrom(c.input); got != c.want {
			t.Errorf("severityFrom(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// Correlations are read from the store and filtered to the release asked about. Handing the
// whole history to the engine would claim every incident the repository has ever seen belongs
// to this one release.
func TestOnlyIncidentsAttributedToTheReleaseAreReturned(t *testing.T) {
	svc, store := serviceWithStore(t)
	ctx := context.Background()

	for _, rec := range []*memory.IncidentRecord{
		{ID: "a", Repository: "github.com/acme/api", ReleaseID: "run-1", DetectedAt: time.Now(),
			Description: "belongs here", Tags: []string{confidenceTag + "0.82"}},
		{ID: "b", Repository: "github.com/acme/api", ReleaseID: "run-2", DetectedAt: time.Now(),
			Description: "another release"},
		{ID: "c", Repository: "github.com/acme/api", DetectedAt: time.Now(),
			Description: "never attributed"},
	} {
		if err := store.RecordIncident(ctx, rec); err != nil {
			t.Fatalf("RecordIncident: %v", err)
		}
	}

	found := svc.GetCorrelations("run-1")
	if len(found) != 1 {
		t.Fatalf("returned %d correlations, want 1: %+v", len(found), found)
	}
	if found[0].Incident.ID != "a" {
		t.Errorf("returned incident %q, want a", found[0].Incident.ID)
	}
	if found[0].Confidence != 0.82 {
		t.Errorf("confidence = %v, want the 0.82 recorded at attribution", found[0].Confidence)
	}
}

// A missing score is reported as missing. A remembered attribution is worth reporting; an
// invented number for it is not.
func TestAMissingConfidenceIsSaidRatherThanInvented(t *testing.T) {
	svc, store := serviceWithStore(t)

	if err := store.RecordIncident(context.Background(), &memory.IncidentRecord{
		ID: "a", Repository: "github.com/acme/api", ReleaseID: "run-1", DetectedAt: time.Now(),
	}); err != nil {
		t.Fatalf("RecordIncident: %v", err)
	}

	found := svc.GetCorrelations("run-1")
	if len(found) != 1 {
		t.Fatalf("returned %d correlations, want 1", len(found))
	}
	if found[0].Confidence != 0 {
		t.Errorf("confidence = %v for an incident that recorded none", found[0].Confidence)
	}
	if len(found[0].Reasons) == 0 {
		t.Error("nothing says why this incident is attributed to the release")
	}
}

// The webhook arrives on whichever HTTP goroutine serves it, while the correlations endpoint
// reads. The slice this replaced had no lock at all; the store is what makes that safe.
func TestConcurrentIncidentsAndReadsAreSafe(t *testing.T) {
	svc, _ := serviceWithStore(t)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			svc.recordIncident(context.Background(), receiver.Incident{
				ID: string(rune('a'+n%26)) + "-incident", Source: "alertmanager",
				Name: "Alert", Severity: "warning", StartedAt: time.Now(),
			})
		}(i)
		go func() {
			defer wg.Done()
			_ = svc.GetCorrelations("run-1")
		}()
	}
	wg.Wait()
}

// A provider that sends no ID must not cost us the incident. An Alertmanager payload without a
// fingerprint arrives with an empty ID, the store requires one, and the incident was dropped
// with only a log line to show for it — verified against the running server before this.
func TestAnIncidentWithNoIDIsStillRecorded(t *testing.T) {
	svc, store := serviceWithStore(t)
	started := time.Now()

	svc.recordIncident(context.Background(), receiver.Incident{
		Source: "alertmanager", Name: "HighErrorRate", Severity: "critical", StartedAt: started,
	})

	history, err := store.GetIncidentHistory(context.Background(), "github.com/acme/api", 10)
	if err != nil {
		t.Fatalf("GetIncidentHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("stored %d incidents, want 1: an alert without a fingerprint was dropped",
			len(history))
	}
	if history[0].ID == "" {
		t.Error("the stored incident has no ID")
	}
}

// The same alert redelivered must land on the same record rather than a second one.
func TestADerivedIDIsStableForTheSameAlert(t *testing.T) {
	started := time.Now()
	incident := receiver.Incident{
		Source: "alertmanager", Name: "HighErrorRate", ServiceName: "api", StartedAt: started,
	}

	first, second := incidentID(incident), incidentID(incident)
	if first != second {
		t.Errorf("the same alert derived %q then %q, so a redelivery would duplicate it",
			first, second)
	}

	other := incident
	other.Name = "LatencyHigh"
	if incidentID(incident) == incidentID(other) {
		t.Error("two different alerts derived the same ID, so one would overwrite the other")
	}
}
