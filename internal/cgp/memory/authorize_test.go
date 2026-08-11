package memory

import (
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
)

// Reconcile detects an ungoverned deployment after it happened; Authorize refuses it
// before it does. Every test here is about a way the gate could fail to refuse — or
// could refuse something legitimate, which is how a gate gets switched off and then
// protects nothing at all.

func governedRelease(version string, decision cgp.DecisionType) *ReleaseRecord {
	return &ReleaseRecord{
		ID:         "run-" + version,
		Repository: "acme/widget",
		Version:    version,
		Actor:      cgp.NewHumanActor("felix", "felix"),
		Decision:   decision,
		Outcome:    OutcomeSuccess,
		RiskScore:  4.5,
		ReleasedAt: time.Unix(1_700_000_000, 0).UTC(),
	}
}

func productionRequest(version string) AuthorizationRequest {
	return AuthorizationRequest{
		Action:      AuthorizeAction,
		Environment: "production",
		Version:     version,
		TargetRef:   "k8s/prod/api",
	}
}

// The whole point: a version nothing governed must not reach production.
func TestAnUngovernedVersionIsRefused(t *testing.T) {
	d := Authorize(
		productionRequest("9.9.9"),
		[]*ReleaseRecord{governedRelease("1.0.0", cgp.DecisionApproved)},
		"production",
	)

	if d.Allowed {
		t.Fatal("a version with no release record reached production: this is the finding a " +
			"governance tool exists to produce, and refusing beats reporting it afterwards")
	}
	if !strings.Contains(d.Reason, "no release record") {
		t.Errorf("Reason = %q, want it to say there is no release record so an operator "+
			"knows the fix is to release through relicta", d.Reason)
	}
}

func TestAGovernedApprovedReleaseIsAllowed(t *testing.T) {
	d := Authorize(
		productionRequest("1.0.0"),
		[]*ReleaseRecord{governedRelease("1.0.0", cgp.DecisionApproved)},
		"production",
	)

	if !d.Allowed {
		t.Fatalf("an approved release must deploy: %s", d.Reason)
	}
	// The evidence is the reason this is worth recording rather than a bare yes.
	for field, want := range map[string]string{
		"release_id":  "run-1.0.0",
		"version":     "1.0.0",
		"environment": "production",
		"decision":    "approved",
		"released_by": "human:felix", // the namespaced actor ID, as recorded everywhere else
		"risk_score":  "4.5",
	} {
		if got := d.Evidence[field]; got != want {
			t.Errorf("Evidence[%q] = %q, want %q — an audit trail must say why a deployment "+
				"was permitted, not only that it was", field, got, want)
		}
	}
}

// A record existing is not a record that permits deployment. Without this, proposing
// a release would be enough to deploy it and the approval step would decide nothing.
func TestARecordIsNotAutomaticallyPermission(t *testing.T) {
	for _, tc := range []struct {
		decision cgp.DecisionType
		expect   string
	}{
		{cgp.DecisionRejected, "rejected"},
		{cgp.DecisionApprovalRequired, "awaiting approval"},
		{cgp.DecisionDeferred, "deferred"},
	} {
		d := Authorize(
			productionRequest("1.0.0"),
			[]*ReleaseRecord{governedRelease("1.0.0", tc.decision)},
			"production",
		)
		if d.Allowed {
			t.Errorf("decision %q was treated as permission to deploy", tc.decision)
		}
		if !strings.Contains(d.Reason, tc.expect) {
			t.Errorf("for %q: Reason = %q, want it to mention %q", tc.decision, d.Reason, tc.expect)
		}
	}
}

// A version legitimately reaches staging before it is released — that is what staging
// is for. Refusing there would block every pre-release deploy and teach people to
// switch the gate off, which is how it stops protecting production too.
func TestStagingIsNotGated(t *testing.T) {
	req := productionRequest("9.9.9")
	req.Environment = "staging"

	d := Authorize(req, nil, "production")

	if !d.Allowed {
		t.Fatalf("an unreleased version must still reach staging: %s", d.Reason)
	}
	if d.Evidence["gate"] != "production-only" {
		t.Errorf("Evidence[gate] = %q, want production-only so the answer explains itself",
			d.Evidence["gate"])
	}
}

// Nothing can be identified as production without a declaration, and guessing would
// either gate every environment or none — both wrong in a way the operator cannot see.
func TestWithoutADeclaredProductionEnvironmentNothingIsRefused(t *testing.T) {
	d := Authorize(productionRequest("9.9.9"), nil, "")

	if !d.Allowed {
		t.Fatalf("with no production environment declared the gate cannot apply: %s", d.Reason)
	}
	if d.Evidence["gate"] != "inactive" {
		t.Errorf("Evidence[gate] = %q, want inactive — a gate that is not deciding must say "+
			"so, or an operator will believe they are protected", d.Evidence["gate"])
	}
}

// A production deploy that cannot name its version cannot be checked, and allowing it
// would be a way around the gate: omit the field and deploy anything.
func TestAProductionDeployMustNameItsVersion(t *testing.T) {
	req := productionRequest("")

	if d := Authorize(req, []*ReleaseRecord{governedRelease("1.0.0", cgp.DecisionApproved)}, "production"); d.Allowed {
		t.Error("a production deployment with no version was allowed: omitting the field " +
			"must not be a way past the gate")
	}
}

// A deployer usually reads its version off an image tag, where "v1.2.3" and "1.2.3"
// are both common. Refusing on that difference would report a governed release as
// ungoverned — a false alarm that teaches people the gate is broken.
func TestAVPrefixDoesNotLookLikeAnUngovernedVersion(t *testing.T) {
	releases := []*ReleaseRecord{governedRelease("1.0.0", cgp.DecisionApproved)}

	if d := Authorize(productionRequest("v1.0.0"), releases, "production"); !d.Allowed {
		t.Errorf("a deployer reporting v1.0.0 for release 1.0.0 was refused: %s", d.Reason)
	}

	tagged := []*ReleaseRecord{governedRelease("v2.0.0", cgp.DecisionApproved)}
	if d := Authorize(productionRequest("2.0.0"), tagged, "production"); !d.Allowed {
		t.Errorf("a release stored as v2.0.0 deployed as 2.0.0 was refused: %s", d.Reason)
	}
}

// A probe proves reachability and decides nothing. It must be allowed regardless of
// what is or is not released, or a caller could never check a gate that is working.
func TestAProbeDecidesNothing(t *testing.T) {
	req := productionRequest("")
	req.Action = ProbeAction

	d := Authorize(req, nil, "production")

	if !d.Allowed {
		t.Fatalf("a readiness probe must be answered, not refused: %s", d.Reason)
	}
	if d.Evidence["probe"] != "true" {
		t.Errorf("Evidence[probe] = %q, want true so a probe is never mistaken for a "+
			"deployment decision in an audit trail", d.Evidence["probe"])
	}
}
