package observability

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/observability/monitor"
)

// Recording what the monitor measured.
//
// `auto_record` is the setting that turns an observation into part of the release record, and
// the record is what change failure rate and every governance decision are computed from. So
// what it writes matters more than most settings: an entry here is indistinguishable, later,
// from one a human put there.
//
// Two rules follow from ADR-016, and both are about what is *not* written:
//
//   - Nothing is recorded for an unmeasured release. The monitor already refuses to call a
//     recorder for one, and this refuses again, because a second reader of that rule is
//     cheaper than discovering it was dropped.
//   - A healthy window writes nothing. An incident is evidence that something happened; the
//     absence of one is how a release that behaved is already represented. Writing a
//     "nothing went wrong" record for every watched release would fill the incident history
//     with non-events and change what the existing records mean.

// NewOutcomeRecorder returns a recorder that writes a measured failure to the governance
// memory as an incident against the release.
//
// Returns nil when there is nowhere to write, so the monitor keeps observing and reporting
// without a store — the dashboard still shows health it cannot record.
func NewOutcomeRecorder(store memory.Store, repository string) monitor.OutcomeRecorder {
	if store == nil {
		return nil
	}

	logger := slog.Default().With("component", "observability_recorder")

	return func(releaseID string, success bool, status monitor.HealthStatus) {
		if !status.Measured {
			logger.Warn("refusing to record an unmeasured release",
				"release_id", releaseID, "unmeasured", status.Unmeasured)
			return
		}
		if success {
			// A release that behaved leaves no incident, which is what "behaved" already
			// looks like in this record.
			return
		}

		incident := &memory.IncidentRecord{
			ID:          fmt.Sprintf("incident-health-%s-%d", releaseID, status.CheckedAt.UnixNano()),
			Repository:  repository,
			ReleaseID:   releaseID,
			Type:        incidentTypeFor(status),
			Severity:    cgp.SeverityHigh,
			Description: describeViolations(status),
			DetectedAt:  status.CheckedAt,
		}

		if err := store.RecordIncident(context.Background(), incident); err != nil {
			logger.Error("health incident not recorded",
				"release_id", releaseID, "error", err)
			return
		}
		logger.Info("health incident recorded",
			"release_id", releaseID, "violations", status.Violations)
	}
}

// incidentTypeFor names what was observed, rather than filing everything as "other".
//
// A latency regression and an availability alert are different things to a reader looking for
// patterns, and the violation text already distinguishes them.
func incidentTypeFor(status monitor.HealthStatus) memory.IncidentType {
	if len(status.Alerts) > 0 {
		return memory.IncidentAvailability
	}
	for _, violation := range status.Violations {
		if strings.HasPrefix(violation, "latency") {
			return memory.IncidentPerformance
		}
	}
	return memory.IncidentAvailability
}

// describeViolations says what was measured and when, in the words the thresholds used.
func describeViolations(status monitor.HealthStatus) string {
	if len(status.Violations) == 0 {
		return "deployment health thresholds crossed"
	}
	return fmt.Sprintf("deployment health after release: %s (measured at %s)",
		strings.Join(status.Violations, "; "),
		status.CheckedAt.Format(time.RFC3339))
}
