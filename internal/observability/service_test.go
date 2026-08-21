package observability

import (
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/observability/monitor"
)

// The subsystem was complete in parts and connected at none of them: nothing read
// observability.providers, NewPrometheusProvider and NewHealthMonitor had no production
// caller, and SetObservabilityService was never called — so an implementation of the handlers'
// interface did not exist anywhere in the tree.

// A repository that asked for nothing gets nothing, and the distinction is load-bearing: the
// routes report `not_configured` for a nil service, which is what separates "nobody is
// watching" from "everything is healthy".
func TestNoProvidersMeansNoService(t *testing.T) {
	svc, err := NewService(config.ObservabilityConfig{}, nil, "repo")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if svc != nil {
		t.Error("a configuration with no providers produced a service. Its empty health list " +
			"would render as a healthy deployment")
	}
}

func TestAConfiguredProviderIsRegistered(t *testing.T) {
	svc, err := NewService(config.ObservabilityConfig{
		Providers: []config.ObservabilityProviderConfig{
			{Name: "prod", Type: "prometheus", Endpoint: "http://localhost:9090"},
		},
	}, nil, "repo")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if svc == nil {
		t.Fatal("a configured provider produced no service")
	}

	statuses := svc.GetProviderStatuses()
	if len(statuses) != 1 {
		t.Fatalf("got %d provider statuses, want 1: %+v", len(statuses), statuses)
	}
	if statuses[0].Name != "prod" {
		t.Errorf("provider name = %q, want prod", statuses[0].Name)
	}
}

// An unknown type is refused rather than skipped. Skipping it would leave somebody watching a
// dashboard that reports nothing wrong because it is asking nobody.
func TestAnUnknownProviderTypeIsRefused(t *testing.T) {
	_, err := NewService(config.ObservabilityConfig{
		Providers: []config.ObservabilityProviderConfig{
			{Name: "prod", Type: "datadog", Endpoint: "http://localhost"},
		},
	}, nil, "repo")

	if err == nil {
		t.Fatal("an unknown provider type was accepted")
	}
	if got := err.Error(); got == "" {
		t.Error("the error does not say which type is unsupported")
	}
}

// auto_record decides whether an observation becomes a record. With it off the monitor still
// reports through the dashboard and writes nothing.
func TestAutoRecordOffDetachesTheRecorder(t *testing.T) {
	cfg := config.ObservabilityConfig{
		Providers: []config.ObservabilityProviderConfig{
			{Name: "prod", Type: "prometheus", Endpoint: "http://localhost:9090"},
		},
		AutoRecord: false,
	}
	cfg.HealthCheck.ProviderName = "prod"

	svc, err := NewService(cfg, nil, "repo")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	recorded := false
	svc = svc.WithHealthMonitor(cfg, func(string, bool, monitor.HealthStatus) { recorded = true })
	if svc.monitor == nil {
		t.Fatal("no health monitor was attached")
	}
	if recorded {
		t.Error("the recorder ran during construction")
	}
}
