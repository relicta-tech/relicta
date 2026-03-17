package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/relicta-tech/relicta/internal/observability/correlation"
	"github.com/relicta-tech/relicta/internal/observability/monitor"
	"github.com/relicta-tech/relicta/internal/observability/providers"
	"github.com/relicta-tech/relicta/internal/observability/receiver"
)

// ObservabilityService defines the interface required by observability handlers.
type ObservabilityService interface {
	// GetDeploymentHealth returns the current deployment health status.
	GetDeploymentHealth() []monitor.HealthStatus
	// GetCorrelations returns incident correlations for a release.
	GetCorrelations(releaseID string) []correlation.ReleaseCorrelation
	// GetProviderStatuses returns the status of all configured providers.
	GetProviderStatuses() []providers.ProviderStatus
	// HandleWebhook returns an HTTP handler for the given provider.
	HandleWebhook(provider string) http.HandlerFunc
}

// observabilitySvc is the service used by observability handlers.
var observabilitySvc ObservabilityService

// SetObservabilityService sets the observability service for handlers.
func SetObservabilityService(svc ObservabilityService) {
	observabilitySvc = svc
}

// GetObservabilityHealth handles GET /api/v1/observability/health.
// Returns current deployment health statuses for all active watches.
func GetObservabilityHealth(w http.ResponseWriter, r *http.Request) {
	if observabilitySvc == nil {
		respondJSON(w, http.StatusOK, map[string]any{
			"health": []monitor.HealthStatus{},
		})
		return
	}

	statuses := observabilitySvc.GetDeploymentHealth()
	if statuses == nil {
		statuses = []monitor.HealthStatus{}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"health": statuses,
	})
}

// GetObservabilityCorrelations handles GET /api/v1/observability/correlations?release_id=X.
// Returns incident correlations for a specific release.
func GetObservabilityCorrelations(w http.ResponseWriter, r *http.Request) {
	if observabilitySvc == nil {
		respondJSON(w, http.StatusOK, map[string]any{
			"correlations": []correlation.ReleaseCorrelation{},
		})
		return
	}

	releaseID := r.URL.Query().Get("release_id")
	if releaseID == "" {
		writeError(w, r, http.StatusBadRequest, ErrCodeMissingField,
			"release_id query parameter is required", nil)
		return
	}

	correlations := observabilitySvc.GetCorrelations(releaseID)
	if correlations == nil {
		correlations = []correlation.ReleaseCorrelation{}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"correlations": correlations,
	})
}

// GetObservabilityProviders handles GET /api/v1/observability/providers.
// Returns the status of all configured observability providers.
func GetObservabilityProviders(w http.ResponseWriter, r *http.Request) {
	if observabilitySvc == nil {
		respondJSON(w, http.StatusOK, map[string]any{
			"providers": []providers.ProviderStatus{},
		})
		return
	}

	statuses := observabilitySvc.GetProviderStatuses()
	if statuses == nil {
		statuses = []providers.ProviderStatus{}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"providers": statuses,
	})
}

// ObservabilityWebhook handles POST /api/v1/observability/webhook/:provider.
// Receives alerts from external systems.
func ObservabilityWebhook(w http.ResponseWriter, r *http.Request) {
	if observabilitySvc == nil {
		writeError(w, r, http.StatusServiceUnavailable, ErrCodeServiceUnavailable,
			"observability service not available", nil)
		return
	}

	providerName := chi.URLParam(r, "provider")
	if providerName == "" {
		writeError(w, r, http.StatusBadRequest, ErrCodeMissingField,
			"provider path parameter is required", nil)
		return
	}

	handler := observabilitySvc.HandleWebhook(providerName)
	handler(w, r)
}

// --- In-memory implementation for wiring ---------------------------------

// DefaultObservabilityService is a simple in-memory implementation
// for wiring the dashboard endpoints.
type DefaultObservabilityService struct {
	Monitor         *monitor.HealthMonitor
	Registry        *providers.Registry
	Correlations    map[string][]correlation.ReleaseCorrelation
	WebhookReceiver *receiver.WebhookReceiver
}

// GetDeploymentHealth returns all active health statuses.
func (s *DefaultObservabilityService) GetDeploymentHealth() []monitor.HealthStatus {
	if s.Monitor == nil {
		return nil
	}
	return s.Monitor.GetAllStatuses()
}

// GetCorrelations returns correlations for a release.
func (s *DefaultObservabilityService) GetCorrelations(releaseID string) []correlation.ReleaseCorrelation {
	if s.Correlations == nil {
		return nil
	}
	return s.Correlations[releaseID]
}

// GetProviderStatuses returns provider health statuses.
func (s *DefaultObservabilityService) GetProviderStatuses() []providers.ProviderStatus {
	if s.Registry == nil {
		return nil
	}
	return s.Registry.HealthCheckAll(context.TODO())
}

// HandleWebhook returns a webhook handler for the provider.
func (s *DefaultObservabilityService) HandleWebhook(provider string) http.HandlerFunc {
	if s.WebhookReceiver == nil {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "webhook receiver not configured", http.StatusServiceUnavailable)
		}
	}
	return s.WebhookReceiver.HandleWebhook(provider)
}
