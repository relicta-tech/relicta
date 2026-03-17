// Package receiver provides HTTP handlers for ingesting alerts from external
// observability systems (Prometheus Alertmanager, PagerDuty, Datadog, etc.)
// and mapping them to internal incident types.
package receiver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Incident is the internal representation of an alert from an external system.
type Incident struct {
	// ID is a unique identifier for this incident.
	ID string `json:"id"`
	// Source identifies the originating system (e.g., "alertmanager", "pagerduty").
	Source string `json:"source"`
	// Name is the alert/incident name.
	Name string `json:"name"`
	// Severity (critical, warning, info).
	Severity string `json:"severity"`
	// Description provides details about the incident.
	Description string `json:"description"`
	// StartedAt is when the alert started firing.
	StartedAt time.Time `json:"started_at"`
	// Labels are key-value pairs from the source system.
	Labels map[string]string `json:"labels,omitempty"`
	// ServiceName is the affected service, extracted from labels.
	ServiceName string `json:"service_name,omitempty"`
	// ReceivedAt is when Relicta received this incident.
	ReceivedAt time.Time `json:"received_at"`
}

// IncidentHandler is called when a new incident is parsed from a webhook.
type IncidentHandler func(incident Incident)

// WebhookReceiver handles incoming alert webhooks from multiple providers.
type WebhookReceiver struct {
	secret  string
	handler IncidentHandler
	logger  *slog.Logger
}

// NewWebhookReceiver creates a new WebhookReceiver.
// secret is the HMAC secret for webhook signature validation (empty to skip).
// handler is called for each parsed incident.
func NewWebhookReceiver(secret string, handler IncidentHandler) *WebhookReceiver {
	return &WebhookReceiver{
		secret:  secret,
		handler: handler,
		logger:  slog.Default().With("component", "observability_receiver"),
	}
}

// HandleWebhook returns an http.HandlerFunc that parses webhooks by provider.
// The provider name is expected as a path parameter.
func (wr *WebhookReceiver) HandleWebhook(provider string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 5*1024*1024))
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		// Validate HMAC signature if secret is configured.
		if wr.secret != "" {
			sig := r.Header.Get("X-Webhook-Signature")
			if sig == "" {
				sig = r.Header.Get("X-Relicta-Signature")
			}
			if !wr.verifySignature(body, sig) {
				http.Error(w, "invalid signature", http.StatusUnauthorized)
				return
			}
		}

		var incidents []Incident
		switch strings.ToLower(provider) {
		case "alertmanager":
			incidents, err = parseAlertmanager(body)
		case "pagerduty":
			incidents, err = parsePagerDuty(body)
		case "datadog":
			incidents, err = parseDatadog(body)
		default:
			incidents, err = parseGeneric(body)
		}

		if err != nil {
			wr.logger.Warn("failed to parse webhook",
				"provider", provider,
				"error", err)
			http.Error(w, fmt.Sprintf("failed to parse webhook: %s", err), http.StatusBadRequest)
			return
		}

		for i := range incidents {
			incidents[i].ReceivedAt = time.Now()
			incidents[i].Source = provider
			if wr.handler != nil {
				wr.handler(incidents[i])
			}
		}

		wr.logger.Info("processed webhook",
			"provider", provider,
			"incidents", len(incidents))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"received": len(incidents),
		})
	}
}

// verifySignature validates the HMAC-SHA256 signature.
func (wr *WebhookReceiver) verifySignature(body []byte, signature string) bool {
	if signature == "" {
		return false
	}
	signature = strings.TrimPrefix(signature, "sha256=")
	mac := hmac.New(sha256.New, []byte(wr.secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// --- Alertmanager format -------------------------------------------------

type alertmanagerPayload struct {
	Alerts []alertmanagerAlert `json:"alerts"`
}

type alertmanagerAlert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	Fingerprint string            `json:"fingerprint"`
}

func parseAlertmanager(body []byte) ([]Incident, error) {
	var payload alertmanagerPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid alertmanager payload: %w", err)
	}

	var incidents []Incident
	for _, a := range payload.Alerts {
		if a.Status != "firing" {
			continue
		}
		severity := a.Labels["severity"]
		if severity == "" {
			severity = "warning"
		}
		inc := Incident{
			ID:          a.Fingerprint,
			Name:        a.Labels["alertname"],
			Severity:    severity,
			Description: a.Annotations["summary"],
			StartedAt:   a.StartsAt,
			Labels:      a.Labels,
			ServiceName: extractService(a.Labels),
		}
		if inc.Description == "" {
			inc.Description = a.Annotations["description"]
		}
		incidents = append(incidents, inc)
	}
	return incidents, nil
}

// --- PagerDuty format ----------------------------------------------------

type pagerDutyPayload struct {
	Messages []pagerDutyMessage `json:"messages"`
}

type pagerDutyMessage struct {
	Event    string            `json:"event"`
	Incident pagerDutyIncident `json:"incident"`
}

type pagerDutyIncident struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Urgency   string           `json:"urgency"`
	CreatedAt time.Time        `json:"created_at"`
	Service   pagerDutyService `json:"service"`
}

type pagerDutyService struct {
	Name string `json:"name"`
}

func parsePagerDuty(body []byte) ([]Incident, error) {
	var payload pagerDutyPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid pagerduty payload: %w", err)
	}

	var incidents []Incident
	for _, msg := range payload.Messages {
		if msg.Event != "incident.trigger" && msg.Event != "incident.acknowledge" {
			continue
		}
		severity := "warning"
		if msg.Incident.Urgency == "high" {
			severity = "critical"
		}
		incidents = append(incidents, Incident{
			ID:          msg.Incident.ID,
			Name:        msg.Incident.Title,
			Severity:    severity,
			Description: msg.Incident.Title,
			StartedAt:   msg.Incident.CreatedAt,
			ServiceName: msg.Incident.Service.Name,
			Labels: map[string]string{
				"urgency": msg.Incident.Urgency,
				"service": msg.Incident.Service.Name,
			},
		})
	}
	return incidents, nil
}

// --- Datadog format ------------------------------------------------------

type datadogPayload struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	AlertType    string   `json:"alert_type"`
	DateHappened int64    `json:"date_happened"`
	Tags         []string `json:"tags"`
	Body         string   `json:"body"`
}

func parseDatadog(body []byte) ([]Incident, error) {
	var payload datadogPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid datadog payload: %w", err)
	}

	severity := "warning"
	switch payload.AlertType {
	case "error":
		severity = "critical"
	case "warning":
		severity = "warning"
	case "info", "success":
		severity = "info"
	}

	labels := make(map[string]string)
	var serviceName string
	for _, tag := range payload.Tags {
		parts := strings.SplitN(tag, ":", 2)
		if len(parts) == 2 {
			labels[parts[0]] = parts[1]
			if parts[0] == "service" {
				serviceName = parts[1]
			}
		}
	}

	startedAt := time.Unix(payload.DateHappened, 0)
	if payload.DateHappened == 0 {
		startedAt = time.Now()
	}

	incidents := []Incident{
		{
			ID:          payload.ID,
			Name:        payload.Title,
			Severity:    severity,
			Description: payload.Body,
			StartedAt:   startedAt,
			Labels:      labels,
			ServiceName: serviceName,
		},
	}
	return incidents, nil
}

// --- Generic format ------------------------------------------------------

type genericPayload struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Severity    string            `json:"severity"`
	Description string            `json:"description"`
	StartedAt   string            `json:"started_at"`
	Labels      map[string]string `json:"labels,omitempty"`
	Service     string            `json:"service,omitempty"`
}

func parseGeneric(body []byte) ([]Incident, error) {
	var payload genericPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid generic payload: %w", err)
	}

	if payload.Name == "" {
		return nil, fmt.Errorf("name is required in generic payload")
	}

	severity := payload.Severity
	if severity == "" {
		severity = "warning"
	}

	startedAt := time.Now()
	if payload.StartedAt != "" {
		if t, err := time.Parse(time.RFC3339, payload.StartedAt); err == nil {
			startedAt = t
		}
	}

	incidents := []Incident{
		{
			ID:          payload.ID,
			Name:        payload.Name,
			Severity:    severity,
			Description: payload.Description,
			StartedAt:   startedAt,
			Labels:      payload.Labels,
			ServiceName: payload.Service,
		},
	}
	return incidents, nil
}

// extractService attempts to extract a service name from alert labels.
func extractService(labels map[string]string) string {
	for _, key := range []string{"service", "job", "app", "application", "namespace"} {
		if v, ok := labels[key]; ok && v != "" {
			return v
		}
	}
	return ""
}
