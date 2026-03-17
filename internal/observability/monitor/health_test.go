package monitor

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/observability/providers"
)

// --- Mock Provider -------------------------------------------------------

type mockProvider struct {
	name       string
	metrics    []providers.MetricSample
	alerts     []providers.Alert
	metricsErr error
	alertsErr  error
	healthErr  error
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) QueryMetrics(_ context.Context, _ providers.MetricQuery) ([]providers.MetricSample, error) {
	return m.metrics, m.metricsErr
}

func (m *mockProvider) QueryAlerts(_ context.Context, _ time.Duration) ([]providers.Alert, error) {
	return m.alerts, m.alertsErr
}

func (m *mockProvider) HealthCheck(_ context.Context) error {
	return m.healthErr
}

// --- Outcome Recorder ----------------------------------------------------

type recordedOutcome struct {
	releaseID string
	success   bool
	status    HealthStatus
}

type outcomeCollector struct {
	mu       sync.Mutex
	outcomes []recordedOutcome
}

func (c *outcomeCollector) record(releaseID string, success bool, status HealthStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.outcomes = append(c.outcomes, recordedOutcome{releaseID, success, status})
}

func (c *outcomeCollector) get() []recordedOutcome {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]recordedOutcome, len(c.outcomes))
	copy(result, c.outcomes)
	return result
}

// --- Tests ---------------------------------------------------------------

func TestDefaultMonitorConfig(t *testing.T) {
	cfg := DefaultMonitorConfig()

	if cfg.Window != 30*time.Minute {
		t.Errorf("expected 30m window, got %v", cfg.Window)
	}
	if cfg.ErrorRateThreshold != 5.0 {
		t.Errorf("expected 5%% error rate threshold, got %f", cfg.ErrorRateThreshold)
	}
	if cfg.LatencyThreshold != 50.0 {
		t.Errorf("expected 50%% latency threshold, got %f", cfg.LatencyThreshold)
	}
}

func TestHealthMonitor_StartWatch_DuplicateRejected(t *testing.T) {
	reg := providers.NewRegistry()
	hm := NewHealthMonitor(DefaultMonitorConfig(), reg, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := hm.StartWatch(ctx, "rel-1")
	if err != nil {
		t.Fatalf("first StartWatch failed: %v", err)
	}
	defer hm.StopWatch("rel-1")

	err = hm.StartWatch(ctx, "rel-1")
	if err == nil {
		t.Error("expected error on duplicate watch")
	}
}

func TestHealthMonitor_StopWatch(t *testing.T) {
	reg := providers.NewRegistry()
	hm := NewHealthMonitor(DefaultMonitorConfig(), reg, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = hm.StartWatch(ctx, "rel-2")
	hm.StopWatch("rel-2")

	_, exists := hm.GetStatus("rel-2")
	if exists {
		t.Error("expected watch to be removed after stop")
	}
}

func TestHealthMonitor_GetStatus(t *testing.T) {
	reg := providers.NewRegistry()
	hm := NewHealthMonitor(DefaultMonitorConfig(), reg, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = hm.StartWatch(ctx, "rel-3")
	defer hm.StopWatch("rel-3")

	status, exists := hm.GetStatus("rel-3")
	if !exists {
		t.Fatal("expected watch to exist")
	}
	if status.ReleaseID != "rel-3" {
		t.Errorf("expected release ID rel-3, got %s", status.ReleaseID)
	}
	if !status.Healthy {
		t.Error("expected initial status to be healthy")
	}
}

func TestHealthMonitor_GetAllStatuses(t *testing.T) {
	reg := providers.NewRegistry()
	hm := NewHealthMonitor(DefaultMonitorConfig(), reg, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = hm.StartWatch(ctx, "rel-a")
	_ = hm.StartWatch(ctx, "rel-b")
	defer hm.StopWatch("rel-a")
	defer hm.StopWatch("rel-b")

	statuses := hm.GetAllStatuses()
	if len(statuses) != 2 {
		t.Errorf("expected 2 statuses, got %d", len(statuses))
	}
}

func TestHealthMonitor_CheckHealth_NoProvider(t *testing.T) {
	reg := providers.NewRegistry()
	cfg := DefaultMonitorConfig()
	cfg.ProviderName = "" // No provider configured.
	hm := NewHealthMonitor(cfg, reg, nil)

	status, err := hm.CheckHealth(context.Background(), "rel-4")
	if err != nil {
		t.Fatalf("CheckHealth() error = %v", err)
	}
	if !status.Healthy {
		t.Error("expected healthy when no provider configured")
	}
}

func TestHealthMonitor_CheckHealth_ProviderNotFound(t *testing.T) {
	reg := providers.NewRegistry()
	cfg := DefaultMonitorConfig()
	cfg.ProviderName = "nonexistent"
	hm := NewHealthMonitor(cfg, reg, nil)

	_, err := hm.CheckHealth(context.Background(), "rel-5")
	if err == nil {
		t.Error("expected error for missing provider")
	}
}

func TestHealthMonitor_CheckHealth_ThresholdCrossed(t *testing.T) {
	mock := &mockProvider{
		name: "test",
		metrics: []providers.MetricSample{
			{Timestamp: time.Now(), Value: 10.0}, // Above 5% threshold.
		},
		alerts: []providers.Alert{},
	}

	reg := providers.NewRegistry()
	_ = reg.Register("test", mock)

	cfg := DefaultMonitorConfig()
	cfg.ProviderName = "test"
	cfg.ErrorRateThreshold = 5.0

	hm := NewHealthMonitor(cfg, reg, nil)

	status, err := hm.CheckHealth(context.Background(), "rel-6")
	if err != nil {
		t.Fatalf("CheckHealth() error = %v", err)
	}
	if status.Healthy {
		t.Error("expected unhealthy when error rate exceeds threshold")
	}
	if len(status.Violations) == 0 {
		t.Error("expected violations to be populated")
	}
}

func TestHealthMonitor_CheckHealth_WithinThreshold(t *testing.T) {
	mock := &mockProvider{
		name: "test",
		metrics: []providers.MetricSample{
			{Timestamp: time.Now(), Value: 2.0}, // Below 5% threshold.
		},
	}

	reg := providers.NewRegistry()
	_ = reg.Register("test", mock)

	cfg := DefaultMonitorConfig()
	cfg.ProviderName = "test"
	cfg.ErrorRateThreshold = 5.0

	hm := NewHealthMonitor(cfg, reg, nil)

	status, err := hm.CheckHealth(context.Background(), "rel-7")
	if err != nil {
		t.Fatalf("CheckHealth() error = %v", err)
	}
	if !status.Healthy {
		t.Error("expected healthy when within threshold")
	}
}

func TestHealthMonitor_WatchWindowExpiry(t *testing.T) {
	reg := providers.NewRegistry()
	collector := &outcomeCollector{}

	cfg := MonitorConfig{
		Window:        100 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
	}
	hm := NewHealthMonitor(cfg, reg, collector.record)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := hm.StartWatch(ctx, "rel-expire")
	if err != nil {
		t.Fatalf("StartWatch() error = %v", err)
	}

	// Wait for the watch to expire.
	time.Sleep(300 * time.Millisecond)

	outcomes := collector.get()
	if len(outcomes) == 0 {
		t.Fatal("expected outcome to be recorded after window expiry")
	}
	if !outcomes[0].success {
		t.Error("expected successful outcome when no thresholds crossed")
	}
	if outcomes[0].releaseID != "rel-expire" {
		t.Errorf("expected release ID rel-expire, got %s", outcomes[0].releaseID)
	}
}

func TestHealthMonitor_WatchThresholdCrossed(t *testing.T) {
	mock := &mockProvider{
		name: "test",
		metrics: []providers.MetricSample{
			{Timestamp: time.Now(), Value: 15.0}, // Way above 5% threshold.
		},
	}

	reg := providers.NewRegistry()
	_ = reg.Register("test", mock)

	collector := &outcomeCollector{}

	cfg := MonitorConfig{
		Window:             2 * time.Second,
		CheckInterval:      50 * time.Millisecond,
		ErrorRateThreshold: 5.0,
		ProviderName:       "test",
	}
	hm := NewHealthMonitor(cfg, reg, collector.record)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := hm.StartWatch(ctx, "rel-fail")
	if err != nil {
		t.Fatalf("StartWatch() error = %v", err)
	}

	// Wait for the watch to detect the threshold crossing.
	time.Sleep(300 * time.Millisecond)

	outcomes := collector.get()
	if len(outcomes) == 0 {
		t.Fatal("expected negative outcome to be recorded")
	}
	if outcomes[0].success {
		t.Error("expected failure outcome when threshold crossed")
	}
}

func TestHealthMonitor_CheckHealth_MetricsError(t *testing.T) {
	mock := &mockProvider{
		name:       "test",
		metricsErr: fmt.Errorf("query timeout"),
	}

	reg := providers.NewRegistry()
	_ = reg.Register("test", mock)

	cfg := DefaultMonitorConfig()
	cfg.ProviderName = "test"

	hm := NewHealthMonitor(cfg, reg, nil)

	// Should not error — metrics errors are non-fatal.
	status, err := hm.CheckHealth(context.Background(), "rel-8")
	if err != nil {
		t.Fatalf("CheckHealth() error = %v", err)
	}
	if !status.Healthy {
		t.Error("expected healthy when metrics query fails (graceful degradation)")
	}
}
