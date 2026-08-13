package memory

import (
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
)

// Holding both release and deployment records makes a question answerable that
// neither could answer alone: does what is running match what was governed?
//
// The serious half is a deployed version with no release behind it — something
// reached an environment without passing through governance. Until deployments were
// recorded that was not merely unreported, it was undetectable, because relicta had
// no way to know what was running.

func releaseRecord(id, version string) *ReleaseRecord {
	return &ReleaseRecord{
		ID: id, Repository: "acme/widget", Version: version,
		Actor: cgp.NewHumanActor("dev", "dev"), Outcome: OutcomeSuccess,
		ReleasedAt: time.Unix(1_700_000_000, 0).UTC(),
	}
}

func deployment(version, env string, outcome DeploymentOutcome) *DeploymentRecord {
	return &DeploymentRecord{
		ID: "deploy-" + version, Repository: "acme/widget", Environment: env,
		Version: version, Outcome: outcome, Provenance: ProvenanceReported,
		Actor: cgp.NewHumanActor("dev", "dev"), DeployedAt: time.Unix(1_700_000_100, 0).UTC(),
	}
}

func kindsOf(found []Discrepancy) map[DiscrepancyKind]int {
	out := make(map[DiscrepancyKind]int)
	for _, d := range found {
		out[d.Kind]++
	}
	return out
}

func TestReconcileFindsAnUngovernedDeployment(t *testing.T) {
	releases := []*ReleaseRecord{releaseRecord("run-1", "1.0.0")}
	deployments := []*DeploymentRecord{
		deployment("1.0.0", "production", DeploymentSucceeded),
		// Nothing governed this one.
		deployment("9.9.9", "production", DeploymentSucceeded),
	}

	found := Reconcile(releases, deployments, "production")

	counts := kindsOf(found)
	if counts[UngovernedDeployment] != 1 {
		t.Fatalf("found %d ungoverned deployments, want 1: %+v", counts[UngovernedDeployment], found)
	}
	for _, d := range found {
		if d.Kind == UngovernedDeployment {
			if d.Version != "9.9.9" {
				t.Errorf("named version %q, want 9.9.9", d.Version)
			}
			if !d.Severe() {
				t.Error("an ungoverned deployment must be severe: it is the finding a " +
					"governance tool exists to produce")
			}
		}
	}
}

// A failed deployment did not reach users, so it is not evidence that something
// ungoverned is running. It stays in the failure-rate data instead.
func TestReconcileIgnoresFailedDeployments(t *testing.T) {
	found := Reconcile(
		[]*ReleaseRecord{releaseRecord("run-1", "1.0.0")},
		[]*DeploymentRecord{deployment("9.9.9", "production", DeploymentFailed)},
		"production",
	)

	if counts := kindsOf(found); counts[UngovernedDeployment] != 0 {
		t.Errorf("a failed deployment of an ungoverned version is not something running: %+v", found)
	}
}

// A deployer that knows the release ID gives an exact answer; one that knows only
// the version — a controller reading an image tag — must still match, or every
// deployment would be reported as ungoverned and the check would be useless exactly
// where it matters.
func TestReconcileMatchesByReleaseIDOrVersion(t *testing.T) {
	releases := []*ReleaseRecord{releaseRecord("run-1", "1.0.0")}

	byID := deployment("whatever-the-tag-says", "production", DeploymentSucceeded)
	byID.ReleaseID = "run-1"
	if found := Reconcile(releases, []*DeploymentRecord{byID}, "production"); kindsOf(found)[UngovernedDeployment] != 0 {
		t.Errorf("a deployment naming its release must match: %+v", found)
	}

	byVersion := deployment("1.0.0", "production", DeploymentSucceeded)
	if found := Reconcile(releases, []*DeploymentRecord{byVersion}, "production"); kindsOf(found)[UngovernedDeployment] != 0 {
		t.Errorf("a deployment matching on version must match: %+v", found)
	}
}

func TestReconcileFindsAnUndeployedRelease(t *testing.T) {
	found := Reconcile(
		[]*ReleaseRecord{releaseRecord("run-1", "1.0.0"), releaseRecord("run-2", "1.1.0")},
		[]*DeploymentRecord{deployment("1.0.0", "production", DeploymentSucceeded)},
		"production",
	)

	counts := kindsOf(found)
	if counts[UndeployedRelease] != 1 {
		t.Fatalf("found %d undeployed releases, want 1 (1.1.0): %+v", counts[UndeployedRelease], found)
	}
	for _, d := range found {
		if d.Kind == UndeployedRelease && d.Severe() {
			t.Error("a release awaiting rollout is a normal state and must not be severe: " +
				"failing on it would train people to ignore the check, and then it stops " +
				"reporting the serious case too")
		}
	}
}

// A release that reached staging and stopped has not been delivered. Treating any
// environment as delivery would hide exactly that.
func TestReconcileDoesNotTreatStagingAsDelivery(t *testing.T) {
	found := Reconcile(
		[]*ReleaseRecord{releaseRecord("run-1", "1.0.0")},
		[]*DeploymentRecord{deployment("1.0.0", "staging", DeploymentSucceeded)},
		"production",
	)

	if counts := kindsOf(found); counts[UndeployedRelease] != 1 {
		t.Errorf("a release deployed only to staging has not reached production: %+v", found)
	}
}

// Without a declared production environment the delivery direction cannot be
// answered, and guessing would report every release as undeployed. The ungoverned
// direction still works, because it does not depend on knowing which environment is
// production.
func TestReconcileWithoutAProductionEnvironment(t *testing.T) {
	found := Reconcile(
		[]*ReleaseRecord{releaseRecord("run-1", "1.0.0")},
		[]*DeploymentRecord{deployment("9.9.9", "production", DeploymentSucceeded)},
		"",
	)

	counts := kindsOf(found)
	if counts[UndeployedRelease] != 0 {
		t.Errorf("with no production environment declared, delivery cannot be judged: %+v", found)
	}
	if counts[UngovernedDeployment] != 1 {
		t.Errorf("an ungoverned deployment is detectable regardless: %+v", found)
	}
}

func TestReconcileOnAQuietRepository(t *testing.T) {
	if found := Reconcile(nil, nil, "production"); len(found) != 0 {
		t.Errorf("nothing released and nothing deployed is not a discrepancy: %+v", found)
	}
}
