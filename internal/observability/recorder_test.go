package observability

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/observability/monitor"
	"github.com/relicta-tech/relicta/v4/internal/observability/providers"
)

// auto_record turns an observation into part of the release record, and that record is what
// change failure rate and every governance decision are computed from. An entry written here is
// indistinguishable, later, from one a human put there — so what it declines to write matters
// as much as what it writes.

func recorderOver(t *testing.T) (monitor.OutcomeRecorder, *memory.InMemoryStore) {
	t.Helper()
	store := memory.NewInMemoryStore()
	return NewOutcomeRecorder(store, "github.com/acme/api"), store
}

func incidents(t *testing.T, store *memory.InMemoryStore) []*memory.IncidentRecord {
	t.Helper()
	found, err := store.GetIncidentHistory(context.Background(), "github.com/acme/api", 100)
	if err != nil {
		t.Fatalf("GetIncidentHistory: %v", err)
	}
	return found
}

func TestAMeasuredFailureIsRecordedAsAnIncident(t *testing.T) {
	record, store := recorderOver(t)

	record("run-1", false, monitor.HealthStatus{
		ReleaseID:  "run-1",
		Measured:   true,
		Violations: []string{"error rate 12.00% exceeds threshold 5.00%"},
		CheckedAt:  time.Now(),
	})

	found := incidents(t, store)
	if len(found) != 1 {
		t.Fatalf("recorded %d incidents, want 1", len(found))
	}
	if found[0].ReleaseID != "run-1" {
		t.Errorf("incident release = %q, want run-1", found[0].ReleaseID)
	}
	if !strings.Contains(found[0].Description, "error rate 12.00%") {
		t.Errorf("the incident does not say what was measured: %q", found[0].Description)
	}
}

// The rule this whole area is built on, read twice: the monitor refuses to call a recorder for
// an unmeasured release, and the recorder refuses again. A second reader of that rule is
// cheaper than discovering it was dropped.
func TestAnUnmeasuredReleaseIsNeverRecorded(t *testing.T) {
	record, store := recorderOver(t)

	record("run-1", false, monitor.HealthStatus{
		ReleaseID:  "run-1",
		Measured:   false,
		Unmeasured: []string{"error rate: connection refused"},
		CheckedAt:  time.Now(),
	})

	if found := incidents(t, store); len(found) != 0 {
		t.Errorf("recorded %d incidents for a release nothing could measure: %+v",
			len(found), found)
	}
}

// A release that behaved leaves no incident, which is what "behaved" already looks like here.
// Writing a non-event for every watched release would change what the existing records mean.
func TestAHealthyWindowWritesNothing(t *testing.T) {
	record, store := recorderOver(t)

	record("run-1", true, monitor.HealthStatus{ReleaseID: "run-1", Measured: true, CheckedAt: time.Now()})

	if found := incidents(t, store); len(found) != 0 {
		t.Errorf("a healthy release wrote %d incidents", len(found))
	}
}

// A firing alert and a latency regression are different things to somebody reading for
// patterns, so they are filed as different kinds.
func TestTheIncidentIsTypedByWhatWasObserved(t *testing.T) {
	if got := incidentTypeFor(monitor.HealthStatus{
		Alerts: []providers.Alert{{Name: "HighErrorRate"}},
	}); got != memory.IncidentAvailability {
		t.Errorf("a firing alert was filed as %q, want availability", got)
	}

	if got := incidentTypeFor(monitor.HealthStatus{
		Violations: []string{"latency increase 80.00% exceeds threshold 50.00%"},
	}); got != memory.IncidentPerformance {
		t.Errorf("a latency regression was filed as %q, want performance", got)
	}
}

// Without somewhere to write, the monitor still observes and reports — the dashboard shows
// health that nothing records, which is better than refusing to look.
func TestNoStoreMeansNoRecorderRatherThanNoMonitoring(t *testing.T) {
	if NewOutcomeRecorder(nil, "repo") != nil {
		t.Error("a recorder was built with nowhere to write")
	}
}
