package monitor

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v5/internal/observability/providers"
)

// --- Mock Provider -------------------------------------------------------

type mockProvider struct {
	name          string
	metrics       []providers.MetricSample
	metricsByName map[string][]providers.MetricSample
	queries       []providers.MetricQuery
	alerts        []providers.Alert
	metricsErr    error
	alertsErr     error
	healthErr     error
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) QueryMetrics(_ context.Context, query providers.MetricQuery) ([]providers.MetricSample, error) {
	m.queries = append(m.queries, query)
	if samples, ok := m.metricsByName[query.Expression]; ok {
		return samples, m.metricsErr
	}
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

func TestHealthMonitor_StartWatch_DuplicateIsIdempotent(t *testing.T) {
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
	if err != nil {
		t.Errorf("duplicate StartWatch() error = %v, want nil", err)
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
	// Was "expected initial status to be healthy". A watch that has just opened has looked at
	// nothing, and reporting health before the first check is a claim about a release nobody
	// has observed yet.
	if status.Measured || status.Healthy {
		t.Errorf("measured=%v healthy=%v before the first check ran",
			status.Measured, status.Healthy)
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
	// Was "expected healthy when no provider configured". A release nobody is watching is
	// not a healthy release; it is an unmeasured one, and saying otherwise puts a result in
	// the record that no observation supports.
	if status.Measured {
		t.Error("a release with no provider configured was reported as measured")
	}
	if status.Healthy {
		t.Error("a release with no provider configured was reported healthy")
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

func TestHealthMonitor_CheckHealth_UsesRatesAndLatencyDelta(t *testing.T) {
	const errorQuery = `sum(rate(http_requests_errors_total[5m])) / sum(rate(http_requests_total[5m])) * 100`
	const latencyQuery = `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))`
	mock := &mockProvider{
		name: "test",
		metricsByName: map[string][]providers.MetricSample{
			errorQuery:   {{Timestamp: time.Now(), Value: 2.0}},
			latencyQuery: {{Timestamp: time.Now().Add(-time.Minute), Value: 0.2}, {Timestamp: time.Now(), Value: 0.28}},
		},
	}
	reg := providers.NewRegistry()
	if err := reg.Register("test", mock); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultMonitorConfig()
	cfg.ProviderName = "test"
	hm := NewHealthMonitor(cfg, reg, nil)

	status, err := hm.CheckHealth(context.Background(), "rel-rate")
	if err != nil {
		t.Fatalf("CheckHealth() error = %v", err)
	}
	if !status.Measured || !status.Healthy {
		t.Fatalf("expected measured healthy status, got %+v", status)
	}
	if status.ErrorRate != 2 || status.LatencyP95 != 0.28 {
		t.Fatalf("got error rate %.2f and p95 %.2f seconds, want 2 and 0.28",
			status.ErrorRate, status.LatencyP95)
	}
	if got := percentIncrease(0.2, status.LatencyP95); got < 39.99 || got > 40.01 {
		t.Fatalf("latency increase = %.2f%%, want 40%%", got)
	}
	if len(mock.queries) != 2 || mock.queries[0].Expression != errorQuery || mock.queries[1].Expression != latencyQuery {
		t.Fatalf("monitor did not use configured PromQL semantics: %+v", mock.queries)
	}
}

func TestHealthMonitor_CheckHealth_UnmeasuredZeroLatencyBaseline(t *testing.T) {
	const latencyQuery = `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))`
	mock := &mockProvider{
		name: "test",
		metricsByName: map[string][]providers.MetricSample{
			latencyQuery: {{Timestamp: time.Now().Add(-time.Minute), Value: 0}, {Timestamp: time.Now(), Value: 0.1}},
		},
		alertsErr: fmt.Errorf("alerts unavailable"),
	}
	reg := providers.NewRegistry()
	if err := reg.Register("test", mock); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultMonitorConfig()
	cfg.ProviderName = "test"
	hm := NewHealthMonitor(cfg, reg, nil)
	status, err := hm.CheckHealth(context.Background(), "rel-no-baseline")
	if err != nil {
		t.Fatalf("CheckHealth() error = %v", err)
	}
	if status.Measured || status.LatencyIncrease != 0 || len(status.Unmeasured) == 0 {
		t.Fatalf("zero latency baseline should be explicitly unmeasured, got %+v", status)
	}
}

func TestPercentIncrease(t *testing.T) {
	if got := percentIncrease(10, 15); got != 50 {
		t.Fatalf("percentIncrease(10, 15) = %v, want 50", got)
	}
	if got := percentIncrease(0, 15); got != 0 {
		t.Fatalf("zero baseline should not produce a percentage, got %v", got)
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

	// Was "expected outcome to be recorded after window expiry", with no provider registered
	// at all. Nothing was watching, so the window expiring says nothing about the release —
	// and a recorded success feeds change failure rate exactly as a real one would.
	// A measured window is covered by TestAMeasuredWindowStillRecordsSuccess.
	if outcomes := collector.get(); len(outcomes) != 0 {
		t.Errorf("recorded %d outcome(s) for a window with no provider to measure it",
			len(outcomes))
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

	// Should not error — a failed query is a failure to measure, not a failure of the run.
	status, err := hm.CheckHealth(context.Background(), "rel-8")
	if err != nil {
		t.Fatalf("CheckHealth() error = %v", err)
	}
	// Was "expected healthy when metrics query fails (graceful degradation)". Degrading to
	// healthy is degrading to a claim: the release is reported as behaving by a check that
	// could not see it. Degrade to unmeasured instead, and record nothing.
	if status.Measured || status.Healthy {
		t.Errorf("measured=%v healthy=%v after every query failed", status.Measured, status.Healthy)
	}
	if len(status.Unmeasured) == 0 {
		t.Error("nothing records why the release could not be measured")
	}
}
