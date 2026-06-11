// Package memory provides the Release Memory store for CGP.
package memory

import (
	"context"
	"log/slog"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/observability/receiver"
)

// WebhookCorrelatorAdapter adapts the IncidentCorrelator to the webhook
// receiver's IncidentHandler callback. It converts incoming webhook incidents
// to memory IncidentRecords and runs correlation automatically.
type WebhookCorrelatorAdapter struct {
	correlator *IncidentCorrelator
	// repoResolver maps a service name to a repository path. When an incident
	// arrives with a ServiceName but no Repository, this function is called to
	// determine which repository the service belongs to.
	repoResolver func(serviceName string) string
	logger       *slog.Logger
}

// WebhookAdapterOption configures a WebhookCorrelatorAdapter.
type WebhookAdapterOption func(*WebhookCorrelatorAdapter)

// WithRepoResolver sets a function that maps service names to repository paths.
func WithRepoResolver(fn func(serviceName string) string) WebhookAdapterOption {
	return func(a *WebhookCorrelatorAdapter) {
		a.repoResolver = fn
	}
}

// NewWebhookCorrelatorAdapter creates an adapter that bridges the webhook
// receiver to the incident correlator.
func NewWebhookCorrelatorAdapter(correlator *IncidentCorrelator, opts ...WebhookAdapterOption) *WebhookCorrelatorAdapter {
	a := &WebhookCorrelatorAdapter{
		correlator: correlator,
		logger:     slog.Default().With("component", "webhook_correlator_adapter"),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Handle returns an IncidentHandler that can be passed to the WebhookReceiver.
// Each incoming webhook incident is converted to a memory IncidentRecord and
// correlated with recent releases.
func (a *WebhookCorrelatorAdapter) Handle() receiver.IncidentHandler {
	return func(incident receiver.Incident) {
		repository := ""
		if incident.ServiceName != "" && a.repoResolver != nil {
			repository = a.repoResolver(incident.ServiceName)
		}
		if repository == "" && incident.ServiceName != "" {
			// Fall back to using the service name as the repository identifier.
			repository = incident.ServiceName
		}
		if repository == "" {
			a.logger.Warn("cannot correlate incident without repository or service name",
				"incident_id", incident.ID,
				"source", incident.Source)
			return
		}

		severity := mapSeverity(incident.Severity)
		incidentType := mapIncidentType(incident.Labels)

		record := IncidentRecord{
			ID:          incident.ID,
			Repository:  repository,
			Type:        incidentType,
			Severity:    severity,
			Description: incident.Description,
			DetectedAt:  incident.StartedAt,
		}

		releaseID, err := a.correlator.Correlate(context.Background(), record)
		if err != nil {
			a.logger.Error("incident correlation failed",
				"incident_id", incident.ID,
				"error", err)
			return
		}

		if releaseID != "" {
			a.logger.Info("webhook incident correlated with release",
				"incident_id", incident.ID,
				"release_id", releaseID,
				"source", incident.Source)
		}
	}
}

// mapSeverity converts a webhook severity string to a CGP Severity.
func mapSeverity(s string) cgp.Severity {
	switch s {
	case "critical":
		return cgp.SeverityCritical
	case "high", "error":
		return cgp.SeverityHigh
	case "warning", "medium":
		return cgp.SeverityMedium
	default:
		return cgp.SeverityLow
	}
}

// mapIncidentType attempts to infer an incident type from labels.
func mapIncidentType(labels map[string]string) IncidentType {
	if labels == nil {
		return IncidentOther
	}

	if t, ok := labels["type"]; ok {
		switch t {
		case "rollback":
			return IncidentRollback
		case "performance":
			return IncidentPerformance
		case "security":
			return IncidentSecurity
		case "availability":
			return IncidentAvailability
		case "data":
			return IncidentDataIssue
		}
	}

	// Infer from alert name patterns.
	if name, ok := labels["alertname"]; ok {
		switch {
		case contains(name, "latency", "slow", "timeout"):
			return IncidentPerformance
		case contains(name, "down", "unavailable", "5xx"):
			return IncidentAvailability
		case contains(name, "security", "auth", "cve"):
			return IncidentSecurity
		}
	}

	return IncidentOther
}

// contains checks if s contains any of the substrings.
func contains(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
