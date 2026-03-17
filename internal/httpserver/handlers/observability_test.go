package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/observability/correlation"
	"github.com/relicta-tech/relicta/internal/observability/monitor"
	"github.com/relicta-tech/relicta/internal/observability/providers"
	"github.com/relicta-tech/relicta/internal/observability/receiver"
)

// mockObservabilityService implements ObservabilityService for testing.
type mockObservabilityService struct {
	healthStatuses   []monitor.HealthStatus
	correlations     map[string][]correlation.ReleaseCorrelation
	providerStatuses []providers.ProviderStatus
}

func (m *mockObservabilityService) GetDeploymentHealth() []monitor.HealthStatus {
	return m.healthStatuses
}

func (m *mockObservabilityService) GetCorrelations(releaseID string) []correlation.ReleaseCorrelation {
	if m.correlations == nil {
		return nil
	}
	return m.correlations[releaseID]
}

func (m *mockObservabilityService) GetProviderStatuses() []providers.ProviderStatus {
	return m.providerStatuses
}

func (m *mockObservabilityService) HandleWebhook(provider string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"received": 1})
	}
}

func TestGetObservabilityHealth_ServiceNil(t *testing.T) {
	original := observabilitySvc
	defer func() { observabilitySvc = original }()
	observabilitySvc = nil

	req := httptest.NewRequest(http.MethodGet, "/api/v1/observability/health", nil)
	w := httptest.NewRecorder()

	GetObservabilityHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	health, ok := resp["health"].([]any)
	if !ok || len(health) != 0 {
		t.Error("expected empty health array")
	}
}

func TestGetObservabilityHealth_WithStatuses(t *testing.T) {
	original := observabilitySvc
	defer func() { observabilitySvc = original }()

	svc := &mockObservabilityService{
		healthStatuses: []monitor.HealthStatus{
			{
				ReleaseID:       "rel-1",
				Healthy:         true,
				MonitoringUntil: time.Now().Add(30 * time.Minute),
				CheckedAt:       time.Now(),
			},
		},
	}
	observabilitySvc = svc

	req := httptest.NewRequest(http.MethodGet, "/api/v1/observability/health", nil)
	w := httptest.NewRecorder()

	GetObservabilityHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	health := resp["health"].([]any)
	if len(health) != 1 {
		t.Errorf("expected 1 health status, got %d", len(health))
	}
}

func TestGetObservabilityCorrelations_MissingReleaseID(t *testing.T) {
	original := observabilitySvc
	defer func() { observabilitySvc = original }()

	observabilitySvc = &mockObservabilityService{}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/observability/correlations", nil)
	w := httptest.NewRecorder()

	GetObservabilityCorrelations(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetObservabilityCorrelations_Success(t *testing.T) {
	original := observabilitySvc
	defer func() { observabilitySvc = original }()

	svc := &mockObservabilityService{
		correlations: map[string][]correlation.ReleaseCorrelation{
			"rel-1": {
				{
					ReleaseID:  "rel-1",
					Repository: "my-repo",
					Version:    "1.0.0",
					Confidence: 0.85,
					Reasons:    []string{"time proximity"},
					Incident: receiver.Incident{
						Name:     "HighErrorRate",
						Severity: "critical",
					},
					CorrelatedAt: time.Now(),
				},
			},
		},
	}
	observabilitySvc = svc

	req := httptest.NewRequest(http.MethodGet, "/api/v1/observability/correlations?release_id=rel-1", nil)
	w := httptest.NewRecorder()

	GetObservabilityCorrelations(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	corr := resp["correlations"].([]any)
	if len(corr) != 1 {
		t.Errorf("expected 1 correlation, got %d", len(corr))
	}
}

func TestGetObservabilityCorrelations_ServiceNil(t *testing.T) {
	original := observabilitySvc
	defer func() { observabilitySvc = original }()
	observabilitySvc = nil

	req := httptest.NewRequest(http.MethodGet, "/api/v1/observability/correlations?release_id=x", nil)
	w := httptest.NewRecorder()

	GetObservabilityCorrelations(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetObservabilityProviders_Success(t *testing.T) {
	original := observabilitySvc
	defer func() { observabilitySvc = original }()

	now := time.Now()
	svc := &mockObservabilityService{
		providerStatuses: []providers.ProviderStatus{
			{
				Name:        "prod-prometheus",
				Type:        "prometheus",
				Endpoint:    "http://prom:9090",
				Healthy:     true,
				LastChecked: &now,
			},
		},
	}
	observabilitySvc = svc

	req := httptest.NewRequest(http.MethodGet, "/api/v1/observability/providers", nil)
	w := httptest.NewRecorder()

	GetObservabilityProviders(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	prov := resp["providers"].([]any)
	if len(prov) != 1 {
		t.Errorf("expected 1 provider, got %d", len(prov))
	}
}

func TestGetObservabilityProviders_ServiceNil(t *testing.T) {
	original := observabilitySvc
	defer func() { observabilitySvc = original }()
	observabilitySvc = nil

	req := httptest.NewRequest(http.MethodGet, "/api/v1/observability/providers", nil)
	w := httptest.NewRecorder()

	GetObservabilityProviders(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestObservabilityWebhook_ServiceNil(t *testing.T) {
	original := observabilitySvc
	defer func() { observabilitySvc = original }()
	observabilitySvc = nil

	req := makeWebhookRequest(http.MethodPost, "/api/v1/observability/webhook/alertmanager",
		map[string]string{"provider": "alertmanager"})
	w := httptest.NewRecorder()

	ObservabilityWebhook(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestObservabilityWebhook_MissingProvider(t *testing.T) {
	original := observabilitySvc
	defer func() { observabilitySvc = original }()

	observabilitySvc = &mockObservabilityService{}

	req := makeWebhookRequest(http.MethodPost, "/api/v1/observability/webhook/",
		map[string]string{})
	w := httptest.NewRecorder()

	ObservabilityWebhook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestObservabilityWebhook_Success(t *testing.T) {
	original := observabilitySvc
	defer func() { observabilitySvc = original }()

	observabilitySvc = &mockObservabilityService{}

	req := makeWebhookRequest(http.MethodPost, "/api/v1/observability/webhook/alertmanager",
		map[string]string{"provider": "alertmanager"})
	w := httptest.NewRecorder()

	ObservabilityWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSetObservabilityService(t *testing.T) {
	original := observabilitySvc
	defer func() { observabilitySvc = original }()

	svc := &mockObservabilityService{}
	SetObservabilityService(svc)

	if observabilitySvc != svc {
		t.Error("SetObservabilityService did not set the service")
	}
}
