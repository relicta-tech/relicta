package receiver

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// collectingHandler collects incidents for test assertion.
type collectingHandler struct {
	mu        sync.Mutex
	incidents []Incident
}

func (h *collectingHandler) handle(inc Incident) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.incidents = append(h.incidents, inc)
}

func (h *collectingHandler) get() []Incident {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]Incident, len(h.incidents))
	copy(result, h.incidents)
	return result
}

func signBody(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookReceiver_Alertmanager(t *testing.T) {
	collector := &collectingHandler{}
	recv := NewWebhookReceiver("", collector.handle)

	payload := alertmanagerPayload{
		Alerts: []alertmanagerAlert{
			{
				Status:      "firing",
				Labels:      map[string]string{"alertname": "HighErrorRate", "severity": "critical", "service": "api"},
				Annotations: map[string]string{"summary": "Error rate exceeded 5%"},
				StartsAt:    time.Now().Add(-5 * time.Minute),
				Fingerprint: "fp-123",
			},
			{
				Status: "resolved",
				Labels: map[string]string{"alertname": "OldAlert"},
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/observability/webhook/alertmanager", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler := recv.HandleWebhook("alertmanager")
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	incidents := collector.get()
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].Name != "HighErrorRate" {
		t.Errorf("expected HighErrorRate, got %s", incidents[0].Name)
	}
	if incidents[0].Severity != "critical" {
		t.Errorf("expected critical, got %s", incidents[0].Severity)
	}
	if incidents[0].ServiceName != "api" {
		t.Errorf("expected service 'api', got %s", incidents[0].ServiceName)
	}
	if incidents[0].Source != "alertmanager" {
		t.Errorf("expected source 'alertmanager', got %s", incidents[0].Source)
	}
}

func TestWebhookReceiver_PagerDuty(t *testing.T) {
	collector := &collectingHandler{}
	recv := NewWebhookReceiver("", collector.handle)

	payload := pagerDutyPayload{
		Messages: []pagerDutyMessage{
			{
				Event: "incident.trigger",
				Incident: pagerDutyIncident{
					ID:        "pd-1",
					Title:     "Service Down",
					Urgency:   "high",
					CreatedAt: time.Now().Add(-2 * time.Minute),
					Service:   pagerDutyService{Name: "payment-service"},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhook/pagerduty", bytes.NewReader(body))
	w := httptest.NewRecorder()

	recv.HandleWebhook("pagerduty")(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	incidents := collector.get()
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].Severity != "critical" {
		t.Errorf("expected critical (high urgency), got %s", incidents[0].Severity)
	}
	if incidents[0].ServiceName != "payment-service" {
		t.Errorf("expected payment-service, got %s", incidents[0].ServiceName)
	}
}

func TestWebhookReceiver_Datadog(t *testing.T) {
	collector := &collectingHandler{}
	recv := NewWebhookReceiver("", collector.handle)

	payload := datadogPayload{
		ID:           "dd-1",
		Title:        "High Latency",
		AlertType:    "error",
		DateHappened: time.Now().Unix(),
		Tags:         []string{"service:checkout", "env:prod"},
		Body:         "P99 latency > 500ms",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhook/datadog", bytes.NewReader(body))
	w := httptest.NewRecorder()

	recv.HandleWebhook("datadog")(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	incidents := collector.get()
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].Severity != "critical" {
		t.Errorf("expected critical for error alert, got %s", incidents[0].Severity)
	}
	if incidents[0].ServiceName != "checkout" {
		t.Errorf("expected service 'checkout', got %s", incidents[0].ServiceName)
	}
}

func TestWebhookReceiver_Generic(t *testing.T) {
	collector := &collectingHandler{}
	recv := NewWebhookReceiver("", collector.handle)

	payload := genericPayload{
		ID:          "gen-1",
		Name:        "CustomAlert",
		Severity:    "warning",
		Description: "Something happened",
		Service:     "auth-service",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhook/generic", bytes.NewReader(body))
	w := httptest.NewRecorder()

	recv.HandleWebhook("generic")(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	incidents := collector.get()
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].Name != "CustomAlert" {
		t.Errorf("expected CustomAlert, got %s", incidents[0].Name)
	}
}

func TestWebhookReceiver_GenericMissingName(t *testing.T) {
	recv := NewWebhookReceiver("", nil)

	payload := genericPayload{ID: "x"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhook/generic", bytes.NewReader(body))
	w := httptest.NewRecorder()

	recv.HandleWebhook("generic")(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing name, got %d", w.Code)
	}
}

func TestWebhookReceiver_SignatureValidation(t *testing.T) {
	secret := "test-secret"
	collector := &collectingHandler{}
	recv := NewWebhookReceiver(secret, collector.handle)

	payload := genericPayload{
		ID:       "sig-1",
		Name:     "SignedAlert",
		Severity: "info",
	}
	body, _ := json.Marshal(payload)

	t.Run("valid signature", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/generic", bytes.NewReader(body))
		req.Header.Set("X-Webhook-Signature", signBody(body, secret))
		w := httptest.NewRecorder()

		recv.HandleWebhook("generic")(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/generic", bytes.NewReader(body))
		req.Header.Set("X-Webhook-Signature", "sha256=invalid")
		w := httptest.NewRecorder()

		recv.HandleWebhook("generic")(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("missing signature", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/generic", bytes.NewReader(body))
		w := httptest.NewRecorder()

		recv.HandleWebhook("generic")(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})
}

func TestWebhookReceiver_MethodNotAllowed(t *testing.T) {
	recv := NewWebhookReceiver("", nil)
	req := httptest.NewRequest(http.MethodGet, "/webhook/alertmanager", nil)
	w := httptest.NewRecorder()

	recv.HandleWebhook("alertmanager")(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestWebhookReceiver_InvalidJSON(t *testing.T) {
	recv := NewWebhookReceiver("", nil)
	req := httptest.NewRequest(http.MethodPost, "/webhook/alertmanager", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	recv.HandleWebhook("alertmanager")(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestWebhookReceiver_UnknownProvider(t *testing.T) {
	collector := &collectingHandler{}
	recv := NewWebhookReceiver("", collector.handle)

	// Unknown provider falls back to generic format.
	payload := genericPayload{
		ID:   "u-1",
		Name: "GenericFallback",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/webhook/unknown", bytes.NewReader(body))
	w := httptest.NewRecorder()

	recv.HandleWebhook("unknown")(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestExtractService(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{"service label", map[string]string{"service": "api"}, "api"},
		{"job label", map[string]string{"job": "worker"}, "worker"},
		{"app label", map[string]string{"app": "frontend"}, "frontend"},
		{"no match", map[string]string{"foo": "bar"}, ""},
		{"empty labels", map[string]string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractService(tt.labels)
			if got != tt.want {
				t.Errorf("extractService() = %q, want %q", got, tt.want)
			}
		})
	}
}
