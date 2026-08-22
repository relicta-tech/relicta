package cli

import (
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// The whole telemetry block was read by nothing. A repository that configured a metrics port
// and path got 9090 and /metrics regardless, so a Prometheus scraping the configured address
// found nothing there — and the address it was told to scrape was the one in the config file.

func TestTheMetricsPathComesFromTheConfiguration(t *testing.T) {
	orig := cfg
	t.Cleanup(func() { cfg = orig })

	cfg = config.DefaultConfig()
	if got := metricsPath(); got != "/metrics" {
		t.Errorf("metricsPath = %q, want the conventional /metrics that every scrape config "+
			"already assumes", got)
	}

	cfg.Telemetry.Metrics.Endpoint = "/internal/metrics"
	if got := metricsPath(); got != "/internal/metrics" {
		t.Errorf("metricsPath = %q, want the configured path", got)
	}

	// A path written without its leading slash is what somebody means, not a different path.
	cfg.Telemetry.Metrics.Endpoint = "internal/metrics"
	if got := metricsPath(); got != "/internal/metrics" {
		t.Errorf("metricsPath = %q, want a leading slash added", got)
	}

	cfg.Telemetry.Metrics.Endpoint = "   "
	if got := metricsPath(); got != "/metrics" {
		t.Errorf("metricsPath = %q, want the default for a blank setting", got)
	}
}

// No configuration at all still serves somewhere: this runs before the config is loaded in
// some paths, and a nil dereference there would take out the command.
func TestTheMetricsPathSurvivesNoConfiguration(t *testing.T) {
	orig := cfg
	t.Cleanup(func() { cfg = orig })

	cfg = nil
	if got := metricsPath(); got != "/metrics" {
		t.Errorf("metricsPath = %q with no configuration loaded", got)
	}
}
