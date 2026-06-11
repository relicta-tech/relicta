// Package monitor provides deployment health monitoring after releases,
// automatically recording outcomes based on observability signals.
package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/relicta-tech/relicta/internal/observability/providers"
)

// MonitorConfig configures the deployment health monitor.
type MonitorConfig struct {
	// Window is how long to monitor after a release (e.g., 30 minutes).
	Window time.Duration `json:"window"`
	// ErrorRateThreshold is the error rate percentage increase that triggers a negative outcome.
	ErrorRateThreshold float64 `json:"error_rate_threshold"`
	// LatencyThreshold is the latency percentage increase that triggers a negative outcome.
	LatencyThreshold float64 `json:"latency_threshold"`
	// ProviderName is the name of the observability provider to query.
	ProviderName string `json:"provider_name"`
	// CheckInterval is how often to poll the provider during the window. Defaults to 30s.
	CheckInterval time.Duration `json:"check_interval"`
}

// DefaultMonitorConfig returns sensible defaults for health monitoring.
func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		Window:             30 * time.Minute,
		ErrorRateThreshold: 5.0,  // 5% error rate increase
		LatencyThreshold:   50.0, // 50% latency increase
		CheckInterval:      30 * time.Second,
	}
}

// HealthStatus represents the current deployment health assessment.
type HealthStatus struct {
	// ReleaseID identifies the release being monitored.
	ReleaseID string `json:"release_id"`
	// Healthy indicates whether the deployment is considered healthy.
	Healthy bool `json:"healthy"`
	// ErrorRate is the current observed error rate percentage.
	ErrorRate float64 `json:"error_rate"`
	// LatencyIncrease is the observed latency increase percentage.
	LatencyIncrease float64 `json:"latency_increase"`
	// Alerts are currently firing alerts detected by the provider.
	Alerts []providers.Alert `json:"alerts,omitempty"`
	// MonitoringUntil is when the health watch window expires.
	MonitoringUntil time.Time `json:"monitoring_until"`
	// CheckedAt is when this status was last computed.
	CheckedAt time.Time `json:"checked_at"`
	// Violations describes which thresholds were crossed.
	Violations []string `json:"violations,omitempty"`
}

// OutcomeRecorder is called when the monitor determines a release outcome.
type OutcomeRecorder func(releaseID string, success bool, details HealthStatus)

// HealthMonitor tracks deployment health after releases.
type HealthMonitor struct {
	config   MonitorConfig
	registry *providers.Registry
	recorder OutcomeRecorder
	logger   *slog.Logger

	mu      sync.RWMutex
	watches map[string]*releaseWatch
}

// releaseWatch tracks the monitoring state for a single release.
type releaseWatch struct {
	releaseID string
	startedAt time.Time
	expiresAt time.Time
	cancel    context.CancelFunc
	status    HealthStatus
}

// NewHealthMonitor creates a new deployment health monitor.
func NewHealthMonitor(cfg MonitorConfig, registry *providers.Registry, recorder OutcomeRecorder) *HealthMonitor {
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 30 * time.Second
	}
	return &HealthMonitor{
		config:   cfg,
		registry: registry,
		recorder: recorder,
		logger:   slog.Default().With("component", "health_monitor"),
		watches:  make(map[string]*releaseWatch),
	}
}

// StartWatch begins health monitoring for a release.
// The monitoring runs in a background goroutine and calls the recorder when complete.
func (hm *HealthMonitor) StartWatch(ctx context.Context, releaseID string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if _, exists := hm.watches[releaseID]; exists {
		return fmt.Errorf("watch already active for release: %s", releaseID)
	}

	watchCtx, cancel := context.WithCancel(ctx) // cancel is stored in watch.cancel and called by StopWatch
	now := time.Now()

	watch := &releaseWatch{
		releaseID: releaseID,
		startedAt: now,
		expiresAt: now.Add(hm.config.Window),
		cancel:    cancel,
		status: HealthStatus{
			ReleaseID:       releaseID,
			Healthy:         true,
			MonitoringUntil: now.Add(hm.config.Window),
			CheckedAt:       now,
		},
	}
	hm.watches[releaseID] = watch

	go hm.runWatch(watchCtx, watch)

	hm.logger.Info("started health watch",
		"release_id", releaseID,
		"window", hm.config.Window,
		"expires_at", watch.expiresAt)

	return nil
}

// StopWatch cancels monitoring for a release.
func (hm *HealthMonitor) StopWatch(releaseID string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if watch, exists := hm.watches[releaseID]; exists {
		watch.cancel()
		delete(hm.watches, releaseID)
	}
}

// GetStatus returns the latest health status for a release.
func (hm *HealthMonitor) GetStatus(releaseID string) (HealthStatus, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	watch, exists := hm.watches[releaseID]
	if !exists {
		return HealthStatus{}, false
	}
	return watch.status, true
}

// GetAllStatuses returns health statuses for all active watches.
func (hm *HealthMonitor) GetAllStatuses() []HealthStatus {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	statuses := make([]HealthStatus, 0, len(hm.watches))
	for _, watch := range hm.watches {
		statuses = append(statuses, watch.status)
	}
	return statuses
}

// CheckHealth performs a single health check for a release without starting a watch.
func (hm *HealthMonitor) CheckHealth(ctx context.Context, releaseID string) (HealthStatus, error) {
	return hm.performCheck(ctx, releaseID)
}

// runWatch is the background goroutine that periodically checks health.
func (hm *HealthMonitor) runWatch(ctx context.Context, watch *releaseWatch) {
	ticker := time.NewTicker(hm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()

			// Window expired without threshold violations — record success.
			if now.After(watch.expiresAt) {
				hm.logger.Info("health watch window expired without violations",
					"release_id", watch.releaseID)
				if hm.recorder != nil {
					hm.recorder(watch.releaseID, true, watch.status)
				}
				hm.mu.Lock()
				delete(hm.watches, watch.releaseID)
				hm.mu.Unlock()
				return
			}

			status, err := hm.performCheck(ctx, watch.releaseID)
			if err != nil {
				hm.logger.Warn("health check failed",
					"release_id", watch.releaseID,
					"error", err)
				continue
			}

			hm.mu.Lock()
			watch.status = status
			hm.mu.Unlock()

			// Threshold crossed — record negative outcome.
			if !status.Healthy {
				hm.logger.Warn("health threshold crossed",
					"release_id", watch.releaseID,
					"violations", status.Violations)
				if hm.recorder != nil {
					hm.recorder(watch.releaseID, false, status)
				}
				hm.mu.Lock()
				delete(hm.watches, watch.releaseID)
				hm.mu.Unlock()
				return
			}
		}
	}
}

// performCheck queries the provider and evaluates health thresholds.
func (hm *HealthMonitor) performCheck(ctx context.Context, releaseID string) (HealthStatus, error) {
	status := HealthStatus{
		ReleaseID:       releaseID,
		Healthy:         true,
		MonitoringUntil: time.Now().Add(hm.config.Window),
		CheckedAt:       time.Now(),
	}

	if hm.config.ProviderName == "" {
		return status, nil
	}

	provider, err := hm.registry.Get(hm.config.ProviderName)
	if err != nil {
		return status, fmt.Errorf("provider not available: %w", err)
	}

	now := time.Now()

	// Query error rate.
	errorSamples, err := provider.QueryMetrics(ctx, providers.MetricQuery{
		MetricName: "http_requests_errors_total",
		Start:      now.Add(-5 * time.Minute),
		End:        now,
		Step:       30 * time.Second,
	})
	if err == nil && len(errorSamples) > 0 {
		status.ErrorRate = errorSamples[len(errorSamples)-1].Value
	}

	// Query latency.
	latencySamples, err := provider.QueryMetrics(ctx, providers.MetricQuery{
		MetricName: "http_request_duration_seconds",
		Start:      now.Add(-5 * time.Minute),
		End:        now,
		Step:       30 * time.Second,
	})
	if err == nil && len(latencySamples) > 0 {
		status.LatencyIncrease = latencySamples[len(latencySamples)-1].Value
	}

	// Query alerts.
	alerts, err := provider.QueryAlerts(ctx, hm.config.Window)
	if err == nil {
		status.Alerts = alerts
	}

	// Evaluate thresholds.
	var violations []string

	if hm.config.ErrorRateThreshold > 0 && status.ErrorRate > hm.config.ErrorRateThreshold {
		violations = append(violations,
			fmt.Sprintf("error rate %.2f%% exceeds threshold %.2f%%",
				status.ErrorRate, hm.config.ErrorRateThreshold))
	}

	if hm.config.LatencyThreshold > 0 && status.LatencyIncrease > hm.config.LatencyThreshold {
		violations = append(violations,
			fmt.Sprintf("latency increase %.2f%% exceeds threshold %.2f%%",
				status.LatencyIncrease, hm.config.LatencyThreshold))
	}

	if len(violations) > 0 {
		status.Healthy = false
		status.Violations = violations
	}

	return status, nil
}
