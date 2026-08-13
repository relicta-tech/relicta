package memory

import (
	"strconv"
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
)

// Reconcile detects an ungoverned deployment after it happened. This answers the
// same question before it happens: a deployer asks whether a version may reach an
// environment, and relicta answers from what it governed.
//
// That difference matters. Reconcile turns an ungoverned deployment into a finding
// someone reads later; this turns it into a deployment that did not occur. Detection
// is what you build when you cannot prevent, and until a deployer could ask, relicta
// could only detect. See ADR-012.
//
// Deliberately not an integration with any deployer. The question and the answer are
// a documented shape, so a GitOps controller, a CI step with curl, or a script can
// all ask — the same reasoning that keeps the deployment-evidence endpoint generic.

// AuthorizationRequest is a deployer asking whether it may proceed.
type AuthorizationRequest struct {
	// Action is what is being attempted. "probe" is a readiness check that decides
	// nothing, so a caller can verify reachability without recording a deploy.
	Action string

	// Environment is where the version is going. Empty means unknown, which is
	// treated as non-production — see the production rule in Authorize.
	Environment string

	// Version is what is being deployed. This is the field the answer turns on.
	Version string

	// TargetRef identifies the destination in the deployer's own terms. Carried
	// through to the evidence so an auditor can tie the decision to the thing
	// deployed, but not used to decide.
	TargetRef string

	// Actor is who is deploying, when the deployer knows.
	Actor cgp.Actor
}

// AuthorizationDecision is relicta's answer.
type AuthorizationDecision struct {
	Allowed bool `json:"allowed"`

	// Reason is written for whoever has to act on a refusal, not for a log.
	Reason string `json:"reason,omitempty"`

	// Evidence records what the decision was based on, so an audit trail says why a
	// deployment was permitted rather than only that it was.
	Evidence map[string]string `json:"evidence,omitempty"`
}

// AuthorizeAction is the action value a deployer sends for a real request.
const AuthorizeAction = "apply"

// ProbeAction is a readiness check. It decides nothing and must never be recorded as
// a deployment decision: a caller checking that governance is reachable has not
// deployed anything.
const ProbeAction = "probe"

// Authorize decides whether a version may reach an environment.
//
// The rule: a version reaching production must have a release record showing it was
// governed and approved. Anything else is refused, because a version with no record
// reaching production is precisely the finding a governance tool exists to produce —
// and refusing is better than reporting it afterwards.
//
// Non-production environments are allowed regardless. A version legitimately reaches
// staging *before* it is released: that is what staging is for, and requiring a
// release record there would refuse every pre-release deploy and teach people to
// switch the gate off. Turning it off is how a gate stops protecting production too.
//
// productionEnvironment names which environment is production. When it is empty
// nothing can be identified as production, so nothing is refused — guessing would
// either gate every environment or none, and both are wrong in a way the operator
// cannot see. Callers that want the gate must declare the environment.
func Authorize(req AuthorizationRequest, releases []*ReleaseRecord, productionEnvironment string) AuthorizationDecision {
	if strings.TrimSpace(req.Action) == ProbeAction {
		return AuthorizationDecision{
			Allowed:  true,
			Reason:   "readiness probe: governance is reachable and decided nothing",
			Evidence: map[string]string{"probe": "true"},
		}
	}

	version := strings.TrimSpace(req.Version)
	environment := strings.TrimSpace(req.Environment)

	if productionEnvironment == "" {
		return AuthorizationDecision{
			Allowed: true,
			Reason: "no production environment is declared, so no deployment can be " +
				"identified as reaching production; declare one to enable this gate",
			Evidence: map[string]string{"gate": "inactive"},
		}
	}

	if environment != productionEnvironment {
		return AuthorizationDecision{
			Allowed: true,
			Reason: "environment " + describeEnvironment(environment) + " is not production, " +
				"where a version is expected to arrive before it is released",
			Evidence: map[string]string{
				"environment": environment,
				"gate":        "production-only",
			},
		}
	}

	// Past here the deployment is heading for production, so it needs a record.
	if version == "" {
		return AuthorizationDecision{
			Allowed: false,
			Reason: "a production deployment must name the version it is deploying; " +
				"without it relicta cannot tell whether the change was governed",
			Evidence: map[string]string{"environment": environment},
		}
	}

	record := findRelease(releases, version)
	if record == nil {
		return AuthorizationDecision{
			Allowed: false,
			Reason: "relicta has no release record for version " + version + ": it was not " +
				"governed, so it must not reach production. Release it through relicta first",
			Evidence: map[string]string{
				"environment": environment,
				"version":     version,
			},
		}
	}

	evidence := map[string]string{
		"environment": environment,
		"version":     version,
		"release_id":  record.ID,
		"decision":    string(record.Decision),
		"outcome":     string(record.Outcome),
		"released_by": record.Actor.ID,
		"released_at": record.ReleasedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if record.RiskScore > 0 {
		evidence["risk_score"] = formatScore(record.RiskScore)
	}
	if req.TargetRef != "" {
		evidence["target_ref"] = req.TargetRef
	}

	// A record exists, which is not the same as a record that permits deployment. A
	// release blocked by policy or still waiting on a human is exactly the case a gate
	// is for: without this check, proposing a release would be enough to deploy it, and
	// the approval step would decide nothing.
	switch record.Decision {
	case cgp.DecisionRejected:
		return AuthorizationDecision{
			Allowed:  false,
			Reason:   "release " + version + " was rejected by governance and must not be deployed",
			Evidence: evidence,
		}
	case cgp.DecisionApprovalRequired:
		return AuthorizationDecision{
			Allowed: false,
			Reason: "release " + version + " is still awaiting approval; approve it in relicta " +
				"before deploying to production",
			Evidence: evidence,
		}
	case cgp.DecisionDeferred:
		return AuthorizationDecision{
			Allowed: false,
			Reason: "release " + version + " was deferred pending more information and has not " +
				"been cleared for production",
			Evidence: evidence,
		}
	}

	return AuthorizationDecision{
		Allowed:  true,
		Reason:   "release " + version + " was governed and approved",
		Evidence: evidence,
	}
}

// findRelease returns the record for a version, tolerating a leading "v".
//
// A deployer usually reads its version off an image tag, where "v1.2.3" and "1.2.3"
// are both common, while relicta stores whatever the release used. Refusing on that
// difference would report a governed release as ungoverned — a false alarm that
// teaches people the gate is broken, which is worse than no gate.
func findRelease(releases []*ReleaseRecord, version string) *ReleaseRecord {
	want := strings.TrimPrefix(version, "v")
	for _, r := range releases {
		if r == nil {
			continue
		}
		if strings.TrimPrefix(r.Version, "v") == want {
			return r
		}
	}
	return nil
}

// describeEnvironment names an environment for a human, including when it has no name.
func describeEnvironment(environment string) string {
	if environment == "" {
		return "(unspecified)"
	}
	return environment
}

// formatScore renders a risk score compactly for the evidence map.
func formatScore(score float64) string {
	return strconv.FormatFloat(score, 'f', -1, 64)
}
