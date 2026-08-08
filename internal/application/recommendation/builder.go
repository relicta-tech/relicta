package recommendation

import (
	"sort"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
)

// BuildInput carries everything needed to assemble an artifact. Every field is
// already computed elsewhere; this package only reshapes it.
type BuildInput struct {
	// Now is injected so the artifact is testable. Only GeneratedAt uses it, and
	// GeneratedAt is excluded from InputsDigest.
	Now time.Time

	ToolVersion string

	Repository string
	Branch     string
	HeadSHA    string

	CurrentVersion string
	NextVersion    string
	ReleaseType    string
	ChangeSet      *changes.ChangeSet

	// Governance is the evaluation outcome, when governance ran. Nil is normal:
	// governance is optional, and an artifact without it still carries facts.
	Governance *GovernanceInput

	// Proposal describes who proposed the change, when known.
	Proposal *Proposal

	// Thresholds come from configuration, so a reader can check the verdict.
	Thresholds *config.GovernanceConfig

	ConfigHash   string
	PlanHash     string
	PolicySource string
}

// GovernanceInput is the subset of governance.EvaluateReleaseOutput this package
// needs. Declared locally to avoid importing the governance application service
// into a presentation concern, and to keep the dependency pointing inward.
type GovernanceInput struct {
	Decision        string
	RiskScore       float64
	Severity        string
	RiskFactors     []cgp.RiskFactor
	Rationale       []string
	RequiredActions []cgp.RequiredAction
	CanAutoApprove  bool
}

// Build assembles the artifact.
func Build(in BuildInput) *Artifact {
	digest := DigestInputs{
		SchemaVersion: SchemaVersion,
		ToolVersion:   in.ToolVersion,
		Repository:    in.Repository,
		Branch:        in.Branch,
		BaseRef:       baseRef(in.ChangeSet),
		HeadSHA:       in.HeadSHA,
		ConfigHash:    in.ConfigHash,
		PolicySource:  in.PolicySource,
	}

	art := &Artifact{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   in.Now.UTC(),
		Subject: Subject{
			Repository: in.Repository,
			Branch:     in.Branch,
			BaseRef:    digest.BaseRef,
			HeadSHA:    in.HeadSHA,
		},
		Proposal:    in.Proposal,
		Facts:       buildFacts(in),
		Obligations: []Obligation{},
		Provenance: Provenance{
			ToolVersion:   in.ToolVersion,
			ConfigHash:    in.ConfigHash,
			PlanHash:      in.PlanHash,
			PolicySource:  in.PolicySource,
			Deterministic: true,
			InputsDigest:  digest.Digest(),
		},
	}

	if in.Governance != nil {
		art.Assessment = buildAssessment(in)
		art.Verdict = buildVerdict(in)
		art.Obligations = buildObligations(in.Governance.RequiredActions)
	}

	return art
}

// baseRef reports what the change set was computed against.
func baseRef(cs *changes.ChangeSet) string {
	if cs == nil {
		return ""
	}
	return cs.FromRef()
}

// buildFacts flattens the change set into a stable, writer-ready list.
//
// Commits are emitted in change-set order rather than by category, so that the
// output is a faithful sequence rather than a presentation choice, and breaking
// changes are additionally collected so a caller does not have to filter.
func buildFacts(in BuildInput) Facts {
	f := Facts{
		CurrentVersion:  in.CurrentVersion,
		NextVersion:     in.NextVersion,
		ReleaseType:     in.ReleaseType,
		Changes:         []Change{},
		BreakingChanges: []Change{},
	}

	if in.ChangeSet == nil {
		return f
	}

	f.CommitCount = in.ChangeSet.CommitCount()

	for _, c := range in.ChangeSet.Commits() {
		if c == nil {
			continue
		}
		change := Change{
			Type:     string(c.Type()),
			Scope:    c.Scope(),
			Subject:  c.Subject(),
			SHA:      c.ShortHash(),
			Breaking: c.IsBreaking(),
		}
		f.Changes = append(f.Changes, change)
		if change.Breaking {
			f.BreakingChanges = append(f.BreakingChanges, change)
		}
	}

	return f
}

// buildAssessment reshapes the risk evaluation, preserving the structure the CLI
// previously discarded by formatting factors into strings.
func buildAssessment(in BuildInput) *Assessment {
	g := in.Governance

	a := &Assessment{
		RiskScore: g.RiskScore,
		Severity:  g.Severity,
		Factors:   make([]RiskFactor, 0, len(g.RiskFactors)),
	}

	for _, rf := range g.RiskFactors {
		a.Factors = append(a.Factors, RiskFactor{
			Category:    rf.Category,
			Description: rf.Description,
			Score:       rf.Score,
			Severity:    string(rf.Severity),
		})
	}

	// Factors arrive from a map in some code paths, so order is not guaranteed.
	// Sort by category to keep the artifact byte-stable across runs.
	sort.SliceStable(a.Factors, func(i, j int) bool {
		if a.Factors[i].Category != a.Factors[j].Category {
			return a.Factors[i].Category < a.Factors[j].Category
		}
		return a.Factors[i].Description < a.Factors[j].Description
	})

	if in.Thresholds != nil {
		a.Thresholds = &Thresholds{
			AutoApproveBelow:        in.Thresholds.AutoApproveThreshold,
			MaxAutoApproveRisk:      in.Thresholds.MaxAutoApproveRisk,
			RequireHumanForBreaking: in.Thresholds.RequireHumanForBreaking,
			RequireHumanForSecurity: in.Thresholds.RequireHumanForSecurity,
		}
	}

	return a
}

// buildVerdict reshapes the decision. It recommends; it does not act.
func buildVerdict(in BuildInput) *Verdict {
	g := in.Governance

	rationale := g.Rationale
	if rationale == nil {
		rationale = []string{}
	}

	return &Verdict{
		Decision:           g.Decision,
		RecommendedVersion: in.NextVersion,
		CanAutoApprove:     g.CanAutoApprove,
		Rationale:          rationale,
	}
}

// buildObligations reshapes required actions into what a human must still do.
func buildObligations(actions []cgp.RequiredAction) []Obligation {
	out := make([]Obligation, 0, len(actions))
	for _, a := range actions {
		out = append(out, Obligation{
			Type:        a.Type,
			Description: a.Description,
			// Every required action blocks by definition: it is something that
			// must happen before execution.
			Blocking: true,
			Assignee: a.Assignee,
		})
	}
	return out
}
