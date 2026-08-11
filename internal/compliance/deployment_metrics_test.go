package compliance

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

// DORA metrics were computed from release records. A release is a tag being
// published; a deployment is a change reaching an environment. So deployment
// frequency counted tags — wrong in both directions for a project that tags weekly
// and deploys daily — and change failure rate could not be computed at all, because
// a failed deployment of a good release was invisible. That is exactly what the
// metric asks about. See ADR-012.

func deploymentAt(env, version string, outcome memory.DeploymentOutcome, at time.Time) *memory.DeploymentRecord {
	return &memory.DeploymentRecord{
		ID:          env + "-" + version + "-" + at.Format(time.RFC3339Nano),
		Repository:  "acme/widget",
		Environment: env,
		Version:     version,
		Outcome:     outcome,
		Provenance:  memory.ProvenanceReported,
		DeployedAt:  at,
	}
}

func generateDORA(t *testing.T, store memory.Store, productionEnv string) *DORAReport {
	t.Helper()

	now := time.Now().UTC()
	report, err := NewGenerator(store, nil).Generate(context.Background(), ReportConfig{
		Type:                  ReportDORA,
		Format:                FormatJSON,
		Period:                Period{Start: now.Add(-30 * 24 * time.Hour), End: now.Add(time.Hour)},
		Repository:            "acme/widget",
		ProductionEnvironment: productionEnv,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return report.DORA
}

func TestDeploymentFrequencyCountsProductionDeployments(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()

	records := []*memory.DeploymentRecord{
		deploymentAt("production", "1.0.0", memory.DeploymentSucceeded, now.Add(-3*time.Hour)),
		deploymentAt("production", "1.1.0", memory.DeploymentSucceeded, now.Add(-2*time.Hour)),
		// Staging must not count: deployment frequency means changes reaching users,
		// and a project deploying to three environments per change would otherwise
		// appear to deploy three times as often as it does.
		deploymentAt("staging", "1.2.0", memory.DeploymentSucceeded, now.Add(-time.Hour)),
		// A failed production deployment did not reach users either.
		deploymentAt("production", "1.3.0", memory.DeploymentFailed, now.Add(-30*time.Minute)),
	}
	for _, r := range records {
		if err := store.RecordDeployment(ctx, r); err != nil {
			t.Fatalf("RecordDeployment: %v", err)
		}
	}

	dora := generateDORA(t, store, "production")

	if dora.DeploymentFrequency.CountedFrom != "deployments" {
		t.Errorf("CountedFrom = %q, want deployments", dora.DeploymentFrequency.CountedFrom)
	}
	if got := dora.DeploymentFrequency.TotalDeployments; got != 2 {
		t.Errorf("TotalDeployments = %d, want 2 (two successful production deployments; "+
			"staging and the failed one must not count)", got)
	}
}

// The metric that could not be computed before deployments existed.
func TestChangeFailureRateUsesDeployments(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()

	for _, r := range []*memory.DeploymentRecord{
		deploymentAt("production", "1.0.0", memory.DeploymentSucceeded, now.Add(-4*time.Hour)),
		deploymentAt("production", "1.1.0", memory.DeploymentSucceeded, now.Add(-3*time.Hour)),
		deploymentAt("production", "1.2.0", memory.DeploymentFailed, now.Add(-2*time.Hour)),
		// A rollback counts as a failure: the change reached users and had to be
		// withdrawn, which is the outcome this metric exists to surface.
		deploymentAt("production", "1.3.0", memory.DeploymentRolledBack, now.Add(-time.Hour)),
	} {
		if err := store.RecordDeployment(ctx, r); err != nil {
			t.Fatalf("RecordDeployment: %v", err)
		}
	}

	dora := generateDORA(t, store, "production")

	if got := dora.ChangeFailureRate.TotalChanges; got != 4 {
		t.Errorf("TotalChanges = %d, want 4 production deployments", got)
	}
	if got := dora.ChangeFailureRate.FailedChanges; got != 2 {
		t.Errorf("FailedChanges = %d, want 2 (one failed, one rolled back)", got)
	}
	if got := dora.ChangeFailureRate.Rate; got < 0.49 || got > 0.51 {
		t.Errorf("Rate = %.2f, want 0.50", got)
	}
}

// Without deployments the metrics fall back to releases — and say so, because
// otherwise the same figure means two different things and a reader cannot tell
// which. A report claiming "12 deployments" that counted tags is the failure this
// label prevents.
func TestFallsBackToReleasesAndLabelsIt(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()

	for i := 0; i < 3; i++ {
		if err := store.RecordRelease(ctx, &memory.ReleaseRecord{
			ID: "run-" + string(rune('a'+i)), Repository: "acme/widget",
			Version: "1.0.0", Outcome: memory.OutcomeSuccess,
			ReleasedAt: now.Add(-time.Duration(i) * time.Hour),
		}); err != nil {
			t.Fatalf("RecordRelease: %v", err)
		}
	}

	dora := generateDORA(t, store, "production")

	if dora.DeploymentFrequency.CountedFrom != "releases" {
		t.Errorf("CountedFrom = %q, want releases when nothing reports deployments",
			dora.DeploymentFrequency.CountedFrom)
	}
	if got := dora.DeploymentFrequency.TotalDeployments; got != 3 {
		t.Errorf("TotalDeployments = %d, want the 3 releases", got)
	}
}

// An undeclared production environment cannot be guessed. Counting every
// environment would inflate the figure by however many environments a change passes
// through, so the report falls back and labels it rather than reading high.
func TestWithoutAProductionEnvironmentItFallsBack(t *testing.T) {
	store := memory.NewInMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()

	for _, r := range []*memory.DeploymentRecord{
		deploymentAt("production", "1.0.0", memory.DeploymentSucceeded, now.Add(-2*time.Hour)),
		deploymentAt("staging", "1.0.0", memory.DeploymentSucceeded, now.Add(-time.Hour)),
	} {
		if err := store.RecordDeployment(ctx, r); err != nil {
			t.Fatalf("RecordDeployment: %v", err)
		}
	}

	dora := generateDORA(t, store, "")

	if dora.DeploymentFrequency.CountedFrom != "releases" {
		t.Errorf("CountedFrom = %q: with no production environment declared the report "+
			"must not count deployments across all environments",
			dora.DeploymentFrequency.CountedFrom)
	}
	if got := dora.DeploymentFrequency.TotalDeployments; got != 0 {
		t.Errorf("TotalDeployments = %d, want 0 — there are no releases to fall back to", got)
	}
}
