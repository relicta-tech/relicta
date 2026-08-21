package cli

import (
	"context"
	"net/http"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/observability/correlation"
	"github.com/relicta-tech/relicta/v4/internal/observability/monitor"
	"github.com/relicta-tech/relicta/v4/internal/observability/providers"
)

// The dashboard's observability routes distinguish "nobody is watching" from "everything is
// healthy" by whether a service was installed at all. That makes the nil case load-bearing:
// a typed nil pointer wrapped in an interface is not nil, and the handlers would then answer
// from a subsystem with no providers — reporting nothing wrong because it is asking nobody.

func TestNoProvidersInstallsNoService(t *testing.T) {
	svc, err := buildObservabilityService(config.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("buildObservabilityService: %v", err)
	}
	if svc != nil {
		t.Error("a repository with no observability configuration got a service. Its empty " +
			"health list renders as a healthy deployment, which is the one answer this must " +
			"never give")
	}
}

func TestAConfiguredProviderInstallsAService(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Observability.Providers = []config.ObservabilityProviderConfig{
		{Name: "prod", Type: "prometheus", Endpoint: "http://localhost:9090"},
	}

	svc, err := buildObservabilityService(cfg, nil)
	if err != nil {
		t.Fatalf("buildObservabilityService: %v", err)
	}
	if svc == nil {
		t.Fatal("a configured provider installed no service")
	}
	if len(svc.GetProviderStatuses()) != 1 {
		t.Errorf("the installed service reports %d providers, want 1",
			len(svc.GetProviderStatuses()))
	}
}

// A misspelled type must stop the server rather than serve a dashboard whose health panel is
// blank for a reason nobody can see.
func TestAnUnknownProviderTypeStopsTheServer(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Observability.Providers = []config.ObservabilityProviderConfig{
		{Name: "prod", Type: "graphite", Endpoint: "http://localhost"},
	}

	if _, err := buildObservabilityService(cfg, nil); err == nil {
		t.Error("an unknown provider type started the server anyway")
	}
}

// watchRecorder stands for the observability service, recording which releases were watched.
type watchRecorder struct{ watched []string }

func (w *watchRecorder) StartWatch(_ context.Context, releaseID string) error {
	w.watched = append(w.watched, releaseID)
	return nil
}
func (w *watchRecorder) GetDeploymentHealth() []monitor.HealthStatus { return nil }
func (w *watchRecorder) GetCorrelations(string) []correlation.ReleaseCorrelation {
	return nil
}
func (w *watchRecorder) GetProviderStatuses() []providers.ProviderStatus { return nil }
func (w *watchRecorder) HandleWebhook(string) http.HandlerFunc           { return nil }

// Only a published release is watched. A plan or an approval has not been deployed, and
// watching one would attribute whatever the metrics show to a release that has not shipped.
func TestOnlyAPublishedReleaseIsWatched(t *testing.T) {
	rec := &watchRecorder{}

	startHealthWatch(rec, &release.RunPublishedEvent{RunID: "run-1"})
	if len(rec.watched) != 1 || rec.watched[0] != "run-1" {
		t.Fatalf("watched %v, want [run-1]", rec.watched)
	}

	startHealthWatch(rec, &release.RunPlannedEvent{RunID: "run-2"})
	if len(rec.watched) != 1 {
		t.Errorf("watched %v after a plan event; a release that has not shipped has nothing "+
			"to observe", rec.watched)
	}
}

// A service that cannot watch — one built without a health monitor — must not panic the
// event subscription it is called from.
func TestAServiceThatCannotWatchIsSkipped(t *testing.T) {
	startHealthWatch(nil, &release.RunPublishedEvent{RunID: "run-1"})
}
