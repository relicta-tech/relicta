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
	"log/slog"
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

	// store is where incidents are recorded, and where correlation reads release history
	// from. nil when the repository has no governance memory, in which case incidents are
	// still received and answered — they are simply not remembered past this process.
	store      memory.Store
	repository string
}

// logger for the paths that can only report a problem rather than return it.
func (s *Service) logger() *slog.Logger {
	return slog.Default().With("component", "observability")
}

// NewService builds the subsystem a repository's configuration describes.
//
// Returns nil when no provider is configured, which is not an error: a repository that has not
// asked for observability has none, and the dashboard routes say `not_configured` rather than
// answering from an empty subsystem that looks like a healthy one.
//
// An unknown provider type is an error rather than a skip. Silently ignoring it would leave
// somebody watching a dashboard that reports nothing wrong because it is asking nobody.
func NewService(cfg config.ObservabilityConfig, store memory.Store, repository string) (*Service, error) {
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

	svc := &Service{registry: registry, store: store, repository: repository}

	if store != nil {
		svc.engine = correlation.NewEngine(store, correlation.DefaultEngineConfig())
	}

	// Recorded rather than accumulated in memory. The slice this replaces was appended to
	// from whichever HTTP handler goroutine received the webhook — a data race with the
	// correlations endpoint reading it — and it was lost on restart besides.
	svc.receiver = receiver.NewWebhookReceiver(cfg.WebhookSecret, func(incident receiver.Incident) {
		svc.recordIncident(context.Background(), incident)
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

// GetCorrelations reports the incidents attributed to a release.
//
// Read from the store, so a restart does not forget them, and filtered to the release asked
// about: the whole history handed to the engine would claim every incident the repository has
// ever seen belongs to this one release.
func (s *Service) GetCorrelations(releaseID string) []correlation.ReleaseCorrelation {
	if s == nil {
		return nil
	}
	return s.storedCorrelations(context.Background(), releaseID)
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
