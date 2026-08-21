package monitor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/observability/providers"
)

// No data beats wrong data.
//
// The monitor assumed the opposite at both ends. performCheck started from Healthy: true and
// only ever cleared it when a threshold was crossed, so a provider that could not be reached
// left the error rate and latency at zero and the release was reported healthy. runWatch then
// recorded success when the window expired, unconditionally — including when every single check
// in that window had failed to reach the provider.
//
// A Prometheus that was down for half an hour therefore produced a recorded successful
// deployment, and that number feeds change failure rate. An unmeasured release has to be
// unmeasured: not healthy, not failed, and not recorded.

// unreachableProvider stands for a provider that is configured and not answering.
type unreachableProvider struct{ name string }

func (p *unreachableProvider) Name() string { return p.name }
func (p *unreachableProvider) QueryMetrics(context.Context, providers.MetricQuery) ([]providers.MetricSample, error) {
	return nil, errors.New("connection refused")
}
func (p *unreachableProvider) QueryAlerts(context.Context, time.Duration) ([]providers.Alert, error) {
	return nil, errors.New("connection refused")
}
func (p *unreachableProvider) HealthCheck(context.Context) error {
	return errors.New("connection refused")
}

// silentProvider answers, and has nothing to say: no samples, no alerts.
type silentProvider struct{ name string }

func (p *silentProvider) Name() string { return p.name }
func (p *silentProvider) QueryMetrics(context.Context, providers.MetricQuery) ([]providers.MetricSample, error) {
	return nil, nil
}
func (p *silentProvider) QueryAlerts(context.Context, time.Duration) ([]providers.Alert, error) {
	return nil, nil
}
func (p *silentProvider) HealthCheck(context.Context) error { return nil }

// answeringProvider returns a real error-rate sample.
type answeringProvider struct {
	name  string
	value float64
}

func (p *answeringProvider) Name() string { return p.name }
func (p *answeringProvider) QueryMetrics(_ context.Context, q providers.MetricQuery) ([]providers.MetricSample, error) {
	return []providers.MetricSample{{Timestamp: time.Now(), Value: p.value}}, nil
}
func (p *answeringProvider) QueryAlerts(context.Context, time.Duration) ([]providers.Alert, error) {
	return nil, nil
}
func (p *answeringProvider) HealthCheck(context.Context) error { return nil }

func registryWith(t *testing.T, p providers.Provider) *providers.Registry {
	t.Helper()
	registry := providers.NewRegistry()
	if err := registry.Register(p.Name(), p); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return registry
}

func monitorFor(t *testing.T, p providers.Provider, rec OutcomeRecorder) *HealthMonitor {
	t.Helper()
	cfg := DefaultMonitorConfig()
	cfg.ProviderName = p.Name()
	cfg.Window = time.Minute
	cfg.CheckInterval = 10 * time.Millisecond
	return NewHealthMonitor(cfg, registryWith(t, p), rec)
}

func TestAnUnreachableProviderIsNotAHealthyRelease(t *testing.T) {
	hm := monitorFor(t, &unreachableProvider{name: "prom"}, nil)

	status, _ := hm.performCheck(context.Background(), "run-1")

	if status.Measured {
		t.Error("a release nothing could be measured for was reported as measured")
	}
	if status.Healthy {
		t.Error("a release nothing could be measured for was reported healthy.\nThe provider " +
			"was unreachable, so the error rate stayed at zero and no threshold was crossed")
	}
	if len(status.Unmeasured) == 0 {
		t.Error("nothing says why the release could not be measured")
	}
}

// A provider that answers with no samples is the same case: the question was asked and not
// answered.
func TestAProviderWithNothingToSayLeavesTheReleaseUnmeasured(t *testing.T) {
	hm := monitorFor(t, &silentProvider{name: "prom"}, nil)

	status, _ := hm.performCheck(context.Background(), "run-1")

	if status.Measured || status.Healthy {
		t.Errorf("measured=%v healthy=%v; a provider that returned no samples measured nothing",
			status.Measured, status.Healthy)
	}
}

func TestAnAnsweringProviderMeasuresTheRelease(t *testing.T) {
	hm := monitorFor(t, &answeringProvider{name: "prom", value: 0.5}, nil)

	status, err := hm.performCheck(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("performCheck: %v", err)
	}
	if !status.Measured {
		t.Error("a provider that answered was not treated as a measurement")
	}
	if !status.Healthy {
		t.Errorf("a 0.5%% error rate under a 5%% threshold was reported unhealthy: %v",
			status.Violations)
	}
}

func TestAMeasuredBreachIsStillUnhealthy(t *testing.T) {
	hm := monitorFor(t, &answeringProvider{name: "prom", value: 42}, nil)

	status, _ := hm.performCheck(context.Background(), "run-1")
	if status.Healthy {
		t.Error("a 42% error rate over a 5% threshold was reported healthy")
	}
	if !status.Measured {
		t.Error("a breach was reported as unmeasured, which would stop it being recorded")
	}
}

// The window expiring is only evidence of success if something was measured in it.
func TestAnUnmeasuredWindowRecordsNothing(t *testing.T) {
	var mu sync.Mutex
	var calls []bool
	rec := func(_ string, success bool, _ HealthStatus) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, success)
	}

	cfg := DefaultMonitorConfig()
	cfg.ProviderName = "prom"
	cfg.Window = 20 * time.Millisecond
	cfg.CheckInterval = 5 * time.Millisecond
	hm := NewHealthMonitor(cfg, registryWith(t, &unreachableProvider{name: "prom"}), rec)

	if err := hm.StartWatch(context.Background(), "run-1"); err != nil {
		t.Fatalf("StartWatch: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 0 {
		t.Errorf("recorded %v for a window in which nothing could be measured.\nA provider that "+
			"was down for the whole window would otherwise produce a recorded successful "+
			"deployment, and that number feeds change failure rate", calls)
	}
}

func TestAMeasuredWindowStillRecordsSuccess(t *testing.T) {
	var mu sync.Mutex
	var calls []bool
	rec := func(_ string, success bool, _ HealthStatus) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, success)
	}

	cfg := DefaultMonitorConfig()
	cfg.ProviderName = "prom"
	cfg.Window = 20 * time.Millisecond
	cfg.CheckInterval = 5 * time.Millisecond
	hm := NewHealthMonitor(cfg, registryWith(t, &answeringProvider{name: "prom", value: 0.1}), rec)

	if err := hm.StartWatch(context.Background(), "run-1"); err != nil {
		t.Fatalf("StartWatch: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 1 || !calls[0] {
		t.Errorf("recorded %v, want one success: a window that was measured and stayed under "+
			"every threshold is a real result", calls)
	}
}

// A watch that has just opened has measured nothing yet, and must say so. It used to seed its
// status Healthy, so the dashboard showed a green release from the instant monitoring began —
// before any check had run.
func TestAFreshWatchIsUnmeasured(t *testing.T) {
	hm := monitorFor(t, &answeringProvider{name: "prom", value: 0.1}, nil)

	if err := hm.StartWatch(context.Background(), "run-1"); err != nil {
		t.Fatalf("StartWatch: %v", err)
	}
	defer hm.StopWatch("run-1")

	status, ok := hm.GetStatus("run-1")
	if !ok {
		t.Fatal("the watch reported no status")
	}
	if status.Measured || status.Healthy {
		t.Errorf("measured=%v healthy=%v before the first check ran",
			status.Measured, status.Healthy)
	}
	if len(status.Unmeasured) == 0 {
		t.Error("nothing says the first check has not run yet")
	}
}
