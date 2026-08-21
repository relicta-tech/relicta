package observability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/observability/correlation"
	"github.com/relicta-tech/relicta/v4/internal/observability/receiver"
)

// Incidents heard through the webhook, and what becomes of them.
//
// They used to be appended to a slice on the service: lost on restart, and appended to from
// whichever HTTP handler goroutine happened to receive them, which is a data race on a field
// read by the correlations endpoint.
//
// They are now recorded through the governance memory — the same store the health monitor
// writes to and the same one correlation reads release history from — so an incident outlives
// the process that heard it, and there is one place to look.
//
// Attribution happens on arrival, while the labels the scoring uses are still in hand. What is
// stored is the incident, the release it was attributed to, and why. Where a confidence cannot
// be read back it is left at zero and said so, rather than a number being invented for it: the
// rule that governs the rest of this subsystem governs its record too.

// confidenceTag carries the score the correlation engine gave an incident at receipt.
const confidenceTag = "correlation-confidence="

// sourceTag records which system reported the incident.
const sourceTag = "source="

// recordIncident attributes an incident to a release and stores it.
func (s *Service) recordIncident(ctx context.Context, incident receiver.Incident) {
	if s.store == nil {
		return
	}

	record := &memory.IncidentRecord{
		ID:          incidentID(incident),
		Repository:  s.repository,
		Type:        memory.IncidentAvailability,
		Severity:    severityFrom(incident.Severity),
		Description: describeIncident(incident),
		DetectedAt:  detectedAt(incident),
		Tags:        []string{sourceTag + incident.Source},
	}

	// Scored here rather than at query time because the labels the engine reads are on the
	// incident as it arrived, and storage keeps only what IncidentRecord has room for.
	if s.engine != nil {
		if found, err := s.engine.Correlate(ctx, incident); err == nil && len(found) > 0 {
			best := found[0]
			for _, candidate := range found[1:] {
				if candidate.Confidence > best.Confidence {
					best = candidate
				}
			}
			record.ReleaseID = best.ReleaseID
			record.Version = best.Version
			record.RootCause = strings.Join(best.Reasons, "; ")
			record.Tags = append(record.Tags,
				confidenceTag+strconv.FormatFloat(best.Confidence, 'f', 2, 64))
		}
	}

	if err := s.store.RecordIncident(ctx, record); err != nil {
		s.logger().Warn("incident not recorded", "incident_id", incident.ID, "error", err)
	}
}

// incidentID is what the incident is filed under.
//
// Providers do not all supply one — an Alertmanager payload without a fingerprint arrives with
// an empty ID — and the store requires it, so the incident was dropped on the floor with only a
// log line to show for it. Losing a real incident because the sender omitted a field is the
// same failure as recording one that never happened, one step along.
//
// Derived from what the alert does carry, and deterministically, so the same alert redelivered
// lands on the same record rather than a second one.
func incidentID(incident receiver.Incident) string {
	if incident.ID != "" {
		return incident.ID
	}

	sum := sha256.Sum256([]byte(strings.Join([]string{
		incident.Source,
		incident.Name,
		incident.ServiceName,
		detectedAt(incident).UTC().Format(time.RFC3339),
	}, "\x00")))

	return "incident-" + hex.EncodeToString(sum[:])[:16]
}

// storedCorrelations rebuilds what is known about a release's incidents from the store.
//
// Only incidents attributed to this release are returned. Returning every incident the
// repository has ever seen — which is what passing the whole history to the engine would do —
// would claim each of them belongs to the release being asked about.
func (s *Service) storedCorrelations(ctx context.Context, releaseID string) []correlation.ReleaseCorrelation {
	if s.store == nil || releaseID == "" {
		return nil
	}

	history, err := s.store.GetIncidentHistory(ctx, s.repository, incidentHistoryLimit)
	if err != nil {
		return nil
	}

	found := make([]correlation.ReleaseCorrelation, 0, len(history))
	for _, record := range history {
		if record == nil || record.ReleaseID != releaseID {
			continue
		}
		found = append(found, correlation.ReleaseCorrelation{
			ReleaseID:    record.ReleaseID,
			Repository:   record.Repository,
			Version:      record.Version,
			Confidence:   confidenceOf(record),
			Reasons:      reasonsOf(record),
			Incident:     incidentFrom(record),
			CorrelatedAt: record.DetectedAt,
		})
	}
	return found
}

// incidentHistoryLimit bounds how far back a correlation query reads.
const incidentHistoryLimit = 500

// confidenceOf reads back the score recorded at attribution.
//
// Zero when it cannot be read, which the reasons say out loud. A remembered attribution with a
// missing score is still worth reporting; a made-up score is not.
func confidenceOf(record *memory.IncidentRecord) float64 {
	for _, tag := range record.Tags {
		if !strings.HasPrefix(tag, confidenceTag) {
			continue
		}
		if value, err := strconv.ParseFloat(strings.TrimPrefix(tag, confidenceTag), 64); err == nil {
			return value
		}
	}
	return 0
}

// reasonsOf reports why the incident was attributed to its release.
func reasonsOf(record *memory.IncidentRecord) []string {
	if record.RootCause != "" {
		return strings.Split(record.RootCause, "; ")
	}
	return []string{"attributed when the incident was received; the score was not recorded"}
}

// incidentFrom rebuilds what the store kept of the incident.
//
// Labels and the original name are not among them: IncidentRecord has nowhere to put them, and
// the scoring that reads them has already run. The fields here are the ones a reader of a
// correlation needs — when it happened, how bad, and what it said.
func incidentFrom(record *memory.IncidentRecord) receiver.Incident {
	return receiver.Incident{
		ID:          record.ID,
		Source:      sourceOf(record),
		Severity:    string(record.Severity),
		Description: record.Description,
		StartedAt:   record.DetectedAt,
		ReceivedAt:  record.DetectedAt,
	}
}

func sourceOf(record *memory.IncidentRecord) string {
	for _, tag := range record.Tags {
		if strings.HasPrefix(tag, sourceTag) {
			return strings.TrimPrefix(tag, sourceTag)
		}
	}
	return ""
}

// describeIncident keeps the alert's own words, which is what a reader recognizes it by.
func describeIncident(incident receiver.Incident) string {
	switch {
	case incident.Name != "" && incident.Description != "":
		return fmt.Sprintf("%s: %s", incident.Name, incident.Description)
	case incident.Name != "":
		return incident.Name
	default:
		return incident.Description
	}
}

// detectedAt prefers when the alert started firing over when relicta heard about it.
func detectedAt(incident receiver.Incident) time.Time {
	if !incident.StartedAt.IsZero() {
		return incident.StartedAt
	}
	if !incident.ReceivedAt.IsZero() {
		return incident.ReceivedAt
	}
	return time.Now()
}

// severityFrom maps a provider's severity word onto the governance scale.
//
// An unrecognized word becomes medium rather than being dropped or guessed high: the incident
// is real whatever it was called, and neither ignoring it nor inflating it is honest.
func severityFrom(severity string) cgp.Severity {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical", "fatal", "page":
		return cgp.SeverityCritical
	case "high", "error":
		return cgp.SeverityHigh
	case "warning", "warn", "medium":
		return cgp.SeverityMedium
	case "info", "low", "none":
		return cgp.SeverityLow
	default:
		return cgp.SeverityMedium
	}
}
