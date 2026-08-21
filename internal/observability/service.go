// Package observability wires the observability subsystem: the providers a repository
// configures, the health monitor that watches a release after it ships, the correlation engine
// that ties incidents back to releases, and the webhook receiver that hears about them.
//
// The parts were complete and connected at none of them. `observability.providers` had no
// reader at all, `NewPrometheusProvider` and `NewHealthMonitor` had no production caller, and
// `handlers.SetObservabilityService` was never called — so the four dashboard routes answered
// from a nil service every time and no implementation of their interface existed.
//
// The rule this is built on: no data beats wrong data. A release nothing could observe is
// unmeasured, not healthy, and nothing is recorded for it. See ADR-016.
package observability

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/observability/correlation"
	"github.com/relicta-tech/relicta/v4/internal/observability/monitor"
	"github.com/relicta-tech/relicta/v4/internal/observability/providers"
	"github.com/relicta-tech/relicta/v4/internal/observability/receiver"
)

// Service is the observability subsystem, assembled from configuration.
type Service struct {
	registry *providers.Registry
	monitor  *monitor.HealthMonitor
	engine   *correlation.Engine
	receiver *receiver.WebhookReceiver

	// incidents holds what the webhook receiver has heard, so a correlation query has
	// something to correlate against.
	incidents []receiver.Incident
}

// NewService builds the subsystem a repository's configuration describes.
//
// Returns nil when no provider is configured, which is not an error: a repository that has not
// asked for observability has none, and the dashboard routes say `not_configured` rather than
// answering from an empty subsystem that looks like a healthy one.
//
// An unknown provider type is an error rather than a skip. Silently ignoring it would leave
// somebody watching a dashboard that reports nothing wrong because it is asking nobody.
func NewService(cfg config.ObservabilityConfig, store memory.Store) (*Service, error) {
	if len(cfg.Providers) == 0 {
		return nil, nil
	}

	registry := providers.NewRegistry()
	for _, p := range cfg.Providers {
		provider, err := buildProvider(p)
		if err != nil {
			return nil, err
		}
		if err := registry.Register(p.Name, provider); err != nil {
			return nil, fmt.Errorf("observability.providers: %w", err)
		}
	}

	svc := &Service{registry: registry}

	if store != nil {
		svc.engine = correlation.NewEngine(store, correlation.DefaultEngineConfig())
	}

	svc.receiver = receiver.NewWebhookReceiver(cfg.WebhookSecret, func(incident receiver.Incident) {
		svc.incidents = append(svc.incidents, incident)
	})

	return svc, nil
}

// buildProvider constructs one configured provider.
func buildProvider(cfg config.ObservabilityProviderConfig) (providers.Provider, error) {
	switch strings.ToLower(cfg.Type) {
	case "prometheus":
		opts := make([]providers.PrometheusOption, 0, 2)
		if cfg.BasicAuthUser != "" || cfg.BasicAuthPass != "" {
			opts = append(opts, providers.WithBasicAuth(cfg.BasicAuthUser, cfg.BasicAuthPass))
		}
		if cfg.BearerToken != "" {
			opts = append(opts, providers.WithBearerToken(cfg.BearerToken))
		}
		return providers.NewPrometheusProvider(cfg.Endpoint, opts...), nil
	default:
		return nil, fmt.Errorf("observability.providers[%s]: unknown type %q; supported: prometheus",
			cfg.Name, cfg.Type)
	}
}

// WithHealthMonitor attaches a health monitor built from the health-check configuration.
//
// Separate from NewService because the monitor needs somewhere to record outcomes, and that
// belongs to the caller. Without a recorder it still watches and reports; it simply writes
// nothing, which is the honest behavior when `auto_record` is off.
func (s *Service) WithHealthMonitor(cfg config.ObservabilityConfig, recorder monitor.OutcomeRecorder) *Service {
	if s == nil {
		return nil
	}

	monitorCfg := monitor.DefaultMonitorConfig()
	if cfg.HealthCheck.Window > 0 {
		monitorCfg.Window = cfg.HealthCheck.Window
	}
	if cfg.HealthCheck.ErrorRateThreshold > 0 {
		monitorCfg.ErrorRateThreshold = cfg.HealthCheck.ErrorRateThreshold
	}
	if cfg.HealthCheck.LatencyThreshold > 0 {
		monitorCfg.LatencyThreshold = cfg.HealthCheck.LatencyThreshold
	}
	monitorCfg.ProviderName = cfg.HealthCheck.ProviderName

	// auto_record is what decides whether anything is written. With it off the monitor still
	// reports through the dashboard; it just does not turn an observation into a record.
	if !cfg.AutoRecord {
		recorder = nil
	}

	s.monitor = monitor.NewHealthMonitor(monitorCfg, s.registry, recorder)
	return s
}

// StartWatch begins monitoring a release, if monitoring is configured.
func (s *Service) StartWatch(ctx context.Context, releaseID string) error {
	if s == nil || s.monitor == nil {
		return nil
	}
	return s.monitor.StartWatch(ctx, releaseID)
}

// GetDeploymentHealth reports what the monitor has observed.
func (s *Service) GetDeploymentHealth() []monitor.HealthStatus {
	if s == nil || s.monitor == nil {
		return nil
	}
	return s.monitor.GetAllStatuses()
}

// GetCorrelations ties incidents heard so far back to a release.
func (s *Service) GetCorrelations(releaseID string) []correlation.ReleaseCorrelation {
	if s == nil || s.engine == nil || len(s.incidents) == 0 {
		return nil
	}
	found, err := s.engine.CorrelateForRelease(context.Background(), releaseID, s.incidents)
	if err != nil {
		return nil
	}
	return found
}

// GetProviderStatuses health-checks every configured provider.
func (s *Service) GetProviderStatuses() []providers.ProviderStatus {
	if s == nil || s.registry == nil {
		return nil
	}
	return s.registry.HealthCheckAll(context.Background())
}

// HandleWebhook returns the handler for a provider's alert webhook.
func (s *Service) HandleWebhook(provider string) http.HandlerFunc {
	if s == nil || s.receiver == nil {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "observability is not configured", http.StatusServiceUnavailable)
		}
	}
	return s.receiver.HandleWebhook(provider)
}
