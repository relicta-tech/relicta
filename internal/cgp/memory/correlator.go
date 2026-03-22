// Package memory provides the Release Memory store for CGP.
package memory

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const defaultCorrelationWindow = 60 * time.Minute

// IncidentCorrelator links incoming incidents to recent releases based on
// temporal proximity and service name matching. When a correlation is found,
// the release outcome is updated to reflect the incident and the incident
// record is persisted in the memory store.
type IncidentCorrelator struct {
	store  Store
	window time.Duration
	logger *slog.Logger
}

// CorrelatorOption configures an IncidentCorrelator.
type CorrelatorOption func(*IncidentCorrelator)

// WithCorrelationWindow sets the maximum time window between a release and an
// incident for them to be considered correlated.
func WithCorrelationWindow(d time.Duration) CorrelatorOption {
	return func(c *IncidentCorrelator) {
		if d > 0 {
			c.window = d
		}
	}
}

// WithLogger sets the logger for the correlator.
func WithLogger(l *slog.Logger) CorrelatorOption {
	return func(c *IncidentCorrelator) {
		if l != nil {
			c.logger = l
		}
	}
}

// NewIncidentCorrelator creates a new IncidentCorrelator with the given store
// and options.
func NewIncidentCorrelator(store Store, opts ...CorrelatorOption) *IncidentCorrelator {
	c := &IncidentCorrelator{
		store:  store,
		window: defaultCorrelationWindow,
		logger: slog.Default().With("component", "incident_correlator"),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Correlate links an incident to the most recent release within the configured
// time window and records the outcome. It returns the correlated release ID,
// or an empty string if no matching release was found.
func (c *IncidentCorrelator) Correlate(ctx context.Context, incident IncidentRecord) (string, error) {
	repository := incident.Repository
	if repository == "" {
		return "", fmt.Errorf("incident repository is required for correlation")
	}

	// Fetch recent releases for this repository. A generous limit ensures we
	// cover the correlation window even for high-frequency release repos.
	releases, err := c.store.GetReleaseHistory(ctx, repository, 100)
	if err != nil {
		return "", fmt.Errorf("failed to fetch release history: %w", err)
	}

	matched := c.findClosestRelease(releases, incident.DetectedAt)
	if matched == nil {
		c.logger.Debug("no correlated release found",
			"incident_id", incident.ID,
			"repository", repository)
		return "", nil
	}

	c.logger.Info("correlated incident with release",
		"incident_id", incident.ID,
		"release_id", matched.ID,
		"release_version", matched.Version,
		"time_since_release", incident.DetectedAt.Sub(matched.ReleasedAt))

	// Update the release outcome to indicate the incident.
	if matched.Outcome == OutcomeSuccess {
		matched.Outcome = OutcomeRollback
	}

	// Link the incident to the release and persist it.
	incident.ReleaseID = matched.ID
	incident.Version = matched.Version
	if incident.ActorID == "" {
		incident.ActorID = matched.Actor.ID
	}
	if !matched.ReleasedAt.IsZero() && !incident.DetectedAt.IsZero() {
		incident.TimeToDetect = incident.DetectedAt.Sub(matched.ReleasedAt)
	}

	if err := c.store.RecordIncident(ctx, &incident); err != nil {
		return matched.ID, fmt.Errorf("failed to record correlated incident: %w", err)
	}

	return matched.ID, nil
}

// findClosestRelease returns the most recent release that occurred before the
// incident and within the correlation window. Returns nil if none found.
func (c *IncidentCorrelator) findClosestRelease(releases []*ReleaseRecord, incidentTime time.Time) *ReleaseRecord {
	windowStart := incidentTime.Add(-c.window)

	var closest *ReleaseRecord
	for _, r := range releases {
		// Release must be before the incident.
		if !r.ReleasedAt.Before(incidentTime) {
			continue
		}
		// Release must be within the correlation window.
		if r.ReleasedAt.Before(windowStart) {
			continue
		}
		// Pick the release closest to the incident (most recent before it).
		if closest == nil || r.ReleasedAt.After(closest.ReleasedAt) {
			closest = r
		}
	}
	return closest
}
