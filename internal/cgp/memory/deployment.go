package memory

import (
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
)

// A release is a tag being published. A deployment is a change reaching an
// environment. Relicta recorded only the first, so the evidence chain ended at the
// tag — "approved and released" rather than "approved, released, and running in
// production since 14:22".
//
// The DORA report already computed deployment metrics from release records, which
// measures the wrong event: deployment frequency counted tags, lead time measured
// commit-to-tag rather than commit-to-running, and change failure rate could not be
// computed at all, because a failed deployment of a good release was invisible —
// exactly what that metric asks about. So these records are a missing input to
// numbers already being reported. See ADR-012.
//
// A deployment is its own record rather than a field on a release: one release
// deploys to several environments at different times with independent outcomes, and
// a rollback is a deployment too.

// DeploymentRecord is one version reaching one environment.
type DeploymentRecord struct {
	// ID uniquely identifies this deployment.
	ID string `json:"id"`

	// Repository is the canonical governance identity of the repository.
	Repository string `json:"repository"`

	// Environment is the declared environment this version reached.
	Environment string `json:"environment"`

	// Version is the version that was deployed.
	Version string `json:"version"`

	// Actor identifies who or what performed the deployment. A GitOps controller
	// is a system actor; a pipeline step is CI.
	Actor cgp.Actor `json:"actor"`

	// Outcome is what happened. A failed deployment is the record that makes change
	// failure rate computable, so it matters as much as a successful one.
	Outcome DeploymentOutcome `json:"outcome"`

	// DeployedAt is when the deployment completed.
	DeployedAt time.Time `json:"deployedAt"`

	// Duration is how long the deployment took, when the reporter knows.
	Duration time.Duration `json:"duration,omitempty"`

	// Provenance records what observed this deployment.
	//
	// Not decoration: a controller reporting its own successful sync and a record
	// inferred from a manifest commit are different qualities of evidence, and an
	// auditor has to be able to weigh them differently. Without this they look
	// identical.
	Provenance DeploymentProvenance `json:"provenance"`

	// Reference points at the deploying system's own record — a CI run URL, a
	// rollout ID — so a reader can follow the claim back to its source.
	Reference string `json:"reference,omitempty"`

	// ReleaseID links to the relicta release run, when the deployer knows it. Empty
	// is meaningful: a deployment with no release is a version that reached an
	// environment without passing through governance.
	ReleaseID string `json:"releaseId,omitempty"`

	// Metadata carries reporter-specific detail.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// DeploymentOutcome is what happened to a deployment.
type DeploymentOutcome string

const (
	// DeploymentSucceeded means the version is running in the environment.
	DeploymentSucceeded DeploymentOutcome = "succeeded"

	// DeploymentFailed means the deployment did not complete. Recorded rather than
	// discarded: change failure rate is computed from these.
	DeploymentFailed DeploymentOutcome = "failed"

	// DeploymentRolledBack means the version was deployed and then withdrawn.
	// Distinct from failed — it ran, and then something made it stop being wanted,
	// which is a different signal about the change.
	DeploymentRolledBack DeploymentOutcome = "rolled_back"
)

// IsValid reports whether the outcome is one this store understands.
//
// Checked rather than accepted, because these records arrive from outside: an
// unrecognized outcome silently stored would be counted as neither a success nor a
// failure and would quietly bias every rate computed from it.
func (o DeploymentOutcome) IsValid() bool {
	switch o {
	case DeploymentSucceeded, DeploymentFailed, DeploymentRolledBack:
		return true
	default:
		return false
	}
}

// DeploymentProvenance says what observed a deployment.
type DeploymentProvenance string

const (
	// ProvenanceReported means the deploying system reported its own outcome. The
	// strongest evidence available, since the thing that did the work said so.
	ProvenanceReported DeploymentProvenance = "reported"

	// ProvenanceInferred means the deployment was deduced from desired state — a
	// manifest commit, a registry tag. Evidence that a deployment was *requested*,
	// not that it succeeded.
	ProvenanceInferred DeploymentProvenance = "inferred"

	// ProvenanceManual means a person recorded it by hand.
	ProvenanceManual DeploymentProvenance = "manual"
)

// IsValid reports whether the provenance is one this store understands.
func (p DeploymentProvenance) IsValid() bool {
	switch p {
	case ProvenanceReported, ProvenanceInferred, ProvenanceManual:
		return true
	default:
		return false
	}
}
