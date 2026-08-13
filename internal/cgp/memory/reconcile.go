package memory

// Relicta records what it governed and, since deployment records exist, what
// actually reached an environment. Holding both makes a question answerable that
// neither could answer alone: does what is running match what was governed?
//
// Two discrepancies, and they are not equally serious.
//
// A released version that never deployed is a delivery gap — the change was
// governed and then did not ship. Worth surfacing, rarely alarming, and often just
// a release that has not been rolled out yet.
//
// A deployed version with no release record is the other thing entirely: something
// reached an environment without passing through governance. That is the finding a
// governance tool exists to produce, and until deployments were recorded it was not
// merely unreported — it was undetectable, because relicta had no way to know what
// was running.

// DiscrepancyKind classifies a mismatch between governance and reality.
type DiscrepancyKind string

const (
	// UngovernedDeployment is a version running in an environment with no release
	// record behind it. Someone deployed around the governance.
	UngovernedDeployment DiscrepancyKind = "ungoverned_deployment"

	// UndeployedRelease is a governed release that never reached the production
	// environment.
	UndeployedRelease DiscrepancyKind = "undeployed_release"
)

// Discrepancy is one mismatch, with enough context to act on it.
type Discrepancy struct {
	Kind        DiscrepancyKind `json:"kind"`
	Version     string          `json:"version"`
	Environment string          `json:"environment,omitempty"`

	// Detail explains the finding in the terms a reader needs, rather than making
	// them infer it from the kind.
	Detail string `json:"detail"`

	// Actor is who deployed or released, when known — the first question asked about
	// an ungoverned deployment.
	Actor string `json:"actor,omitempty"`

	// Reference points at the deployer's own record, so an ungoverned deployment can
	// be traced to the system that performed it.
	Reference string `json:"reference,omitempty"`
}

// Severe reports whether a discrepancy should fail a check.
//
// Only ungoverned deployments do. A release awaiting rollout is a normal state on
// any given afternoon, and failing on it would train people to ignore the command —
// at which point it stops reporting the serious case too.
func (d Discrepancy) Severe() bool {
	return d.Kind == UngovernedDeployment
}

// Reconcile compares deployments against releases and returns what does not line up.
//
// Matching is by release ID first and version second. A deployer that knows the
// release ID gives an exact answer; one that knows only the version — a controller
// reading an image tag — still matches, because requiring the ID would report every
// deployment as ungoverned and make the check useless where it matters most.
//
// Only production deployments are considered for the undeployed-release direction:
// a release that reached staging and stopped has not been delivered, and treating
// staging as delivery would hide exactly that.
func Reconcile(releases []*ReleaseRecord, deployments []*DeploymentRecord, productionEnvironment string) []Discrepancy {
	releasedVersions := make(map[string]*ReleaseRecord, len(releases))
	releasedIDs := make(map[string]*ReleaseRecord, len(releases))
	for _, r := range releases {
		if r.Version != "" {
			releasedVersions[r.Version] = r
		}
		if r.ID != "" {
			releasedIDs[r.ID] = r
		}
	}

	var found []Discrepancy

	// Deployments with nothing behind them.
	deployedVersions := make(map[string]bool, len(deployments))
	for _, dep := range deployments {
		if dep.Outcome != DeploymentSucceeded {
			// A failed deployment did not reach users, so it is not evidence that
			// something ungoverned is running. It stays in the failure-rate data.
			continue
		}
		if productionEnvironment != "" && dep.Environment == productionEnvironment {
			deployedVersions[dep.Version] = true
		}

		if dep.ReleaseID != "" {
			if _, ok := releasedIDs[dep.ReleaseID]; ok {
				continue
			}
		}
		if _, ok := releasedVersions[dep.Version]; ok {
			continue
		}

		found = append(found, Discrepancy{
			Kind:        UngovernedDeployment,
			Version:     dep.Version,
			Environment: dep.Environment,
			Actor:       dep.Actor.ID,
			Reference:   dep.Reference,
			Detail: "this version is running but relicta has no release record for it: " +
				"it reached the environment without passing through governance",
		})
	}

	// Releases that never arrived. Skipped entirely without a production
	// environment, because "deployed" would otherwise mean "deployed anywhere", and
	// a release sitting in staging would be reported as delivered.
	if productionEnvironment != "" {
		for _, rel := range releases {
			if rel.Version == "" || deployedVersions[rel.Version] {
				continue
			}
			found = append(found, Discrepancy{
				Kind:        UndeployedRelease,
				Version:     rel.Version,
				Environment: productionEnvironment,
				Actor:       rel.Actor.ID,
				Detail: "this version was released and governed but has no successful " +
					"deployment to " + productionEnvironment,
			})
		}
	}

	return found
}
