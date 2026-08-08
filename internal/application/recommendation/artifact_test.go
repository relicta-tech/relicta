package recommendation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
)

func testChangeSet(t *testing.T) *changes.ChangeSet {
	t.Helper()

	cs := changes.NewChangeSet("cs-test", "v4.2.0", "HEAD")
	for i, raw := range []string{
		"feat(versioning): support several version manifests",
		"fix(cli): resolve config from any subdirectory",
		"feat(api)!: drop the legacy notes endpoint",
	} {
		c := changes.ParseConventionalCommit(fmt.Sprintf("abc123%d", i), raw)
		if c == nil {
			t.Fatalf("ParseConventionalCommit(%q) returned nil", raw)
		}
		cs.AddCommit(c)
	}
	return cs
}

func testInput(t *testing.T) BuildInput {
	t.Helper()

	conf := 0.62
	return BuildInput{
		Now:            time.Date(2026, 8, 8, 9, 0, 0, 0, time.UTC),
		ToolVersion:    "4.3.0",
		Repository:     "relicta-tech/relicta",
		Branch:         "main",
		HeadSHA:        "1fe781b42990338c3ebcd93d85999be5a3317db7",
		CurrentVersion: "4.2.0",
		NextVersion:    "5.0.0",
		ReleaseType:    "major",
		ChangeSet:      testChangeSet(t),
		ConfigHash:     "sha256:2b1f",
		PlanHash:       "sha256:9ac3",
		PolicySource:   ".relicta/policies/release.policy",
		Proposal: &Proposal{
			Actor:  ProposalActor{Kind: "agent", ID: "claude-code@team-platform"},
			Intent: ProposalIntent{Summary: "gradle support", SuggestedBump: "minor", DeclaredConfidence: &conf},
		},
		Thresholds: &config.GovernanceConfig{
			AutoApproveThreshold:    0.30,
			MaxAutoApproveRisk:      0.50,
			RequireHumanForBreaking: true,
		},
		Governance: &GovernanceInput{
			Decision:  "approval_required",
			RiskScore: 0.34,
			Severity:  "medium",
			// Deliberately out of alphabetical order: the builder must sort.
			RiskFactors: []cgp.RiskFactor{
				{Category: "historical", Description: "no incidents", Score: 0.06, Severity: cgp.SeverityLow},
				{Category: "api_change", Description: "breaking change present", Score: 0.10, Severity: cgp.SeverityHigh},
			},
			Rationale:       []string{"breaking change requires human approval"},
			RequiredActions: []cgp.RequiredAction{{Type: "human_approval", Description: "one approver required"}},
			CanAutoApprove:  false,
		},
	}
}

// TestBuild_NoProseFields is the artifact's central promise: nothing in it was
// written by a model. Every string must be traceable to structured input.
func TestBuild_NoProseFields(t *testing.T) {
	art := Build(testInput(t))

	// Facts carry commit subjects verbatim from the change set.
	if len(art.Facts.Changes) != 3 {
		t.Fatalf("Changes = %d, want 3", len(art.Facts.Changes))
	}
	if got := art.Facts.Changes[0].Subject; got != "support several version manifests" {
		t.Errorf("subject = %q, want it copied verbatim from the commit", got)
	}

	// Rationale comes from governance, not from generation.
	if len(art.Verdict.Rationale) != 1 || art.Verdict.Rationale[0] != "breaking change requires human approval" {
		t.Errorf("Rationale = %v, want the governance rationale verbatim", art.Verdict.Rationale)
	}
}

// TestBuild_IsDeterministic is the claim provenance.deterministic makes: two
// builds over identical inputs must agree on facts, assessment and verdict.
// GeneratedAt is excluded by construction, so the whole artifact should match
// when Now is held equal.
func TestBuild_IsDeterministic(t *testing.T) {
	a := Build(testInput(t))
	b := Build(testInput(t))

	ja, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	jb, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	if !bytes.Equal(ja, jb) {
		t.Errorf("two builds over identical inputs differ:\n a = %s\n b = %s", ja, jb)
	}
	if a.Provenance.InputsDigest != b.Provenance.InputsDigest {
		t.Errorf("inputs digest differs: %q vs %q", a.Provenance.InputsDigest, b.Provenance.InputsDigest)
	}
}

// Factors may arrive in any order; the artifact must not inherit that.
func TestBuild_SortsRiskFactors(t *testing.T) {
	art := Build(testInput(t))

	if len(art.Assessment.Factors) != 2 {
		t.Fatalf("Factors = %d, want 2", len(art.Assessment.Factors))
	}
	if art.Assessment.Factors[0].Category != "api_change" {
		t.Errorf("first factor = %q, want api_change (sorted, despite input order)",
			art.Assessment.Factors[0].Category)
	}
}

// Structure the CLI previously discarded by formatting factors into strings such
// as "[api_change] description (10%)" must survive.
func TestBuild_PreservesRiskFactorStructure(t *testing.T) {
	art := Build(testInput(t))

	f := art.Assessment.Factors[0]
	if f.Category == "" || f.Description == "" || f.Score == 0 || f.Severity == "" {
		t.Errorf("factor lost structure: %+v", f)
	}
	// And it must not have been pre-rendered.
	if strings.Contains(f.Description, "[") || strings.Contains(f.Description, "%") {
		t.Errorf("description %q looks pre-formatted; it should be the raw field", f.Description)
	}
}

// Breaking changes are collected separately so a caller does not have to filter.
func TestBuild_SeparatesBreakingChanges(t *testing.T) {
	art := Build(testInput(t))

	if len(art.Facts.BreakingChanges) != 1 {
		t.Fatalf("BreakingChanges = %d, want 1", len(art.Facts.BreakingChanges))
	}
	if !art.Facts.BreakingChanges[0].Breaking {
		t.Error("collected change is not marked breaking")
	}
	// It also stays in the main list; the two are views, not partitions.
	if len(art.Facts.Changes) != 3 {
		t.Errorf("Changes = %d, want all 3 including the breaking one", len(art.Facts.Changes))
	}
}

// Confidence belongs to the proposer, never the verdict.
func TestBuild_ConfidenceIsOnTheProposalOnly(t *testing.T) {
	art := Build(testInput(t))

	if art.Proposal == nil || art.Proposal.Intent.DeclaredConfidence == nil {
		t.Fatal("proposal confidence missing")
	}
	if got := *art.Proposal.Intent.DeclaredConfidence; got != 0.62 {
		t.Errorf("declared confidence = %v, want 0.62", got)
	}

	// The verdict must have no confidence field at all.
	raw, err := json.Marshal(art.Verdict)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(raw), "confidence") {
		t.Errorf("verdict carries a confidence field: %s", raw)
	}
}

// Governance is optional; an artifact without it must still carry facts.
func TestBuild_WithoutGovernance(t *testing.T) {
	in := testInput(t)
	in.Governance = nil

	art := Build(in)

	if art.Assessment != nil {
		t.Error("Assessment should be omitted when governance did not run")
	}
	if art.Verdict != nil {
		t.Error("Verdict should be omitted when governance did not run")
	}
	if len(art.Facts.Changes) != 3 {
		t.Errorf("facts should survive without governance, got %d changes", len(art.Facts.Changes))
	}
	// Obligations must be an empty array, not null, so consumers can iterate.
	raw, err := json.Marshal(art)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(raw), `"obligations":[]`) {
		t.Error("obligations should marshal as [] rather than null")
	}
}

func TestBuild_NilChangeSetDoesNotPanic(t *testing.T) {
	in := testInput(t)
	in.ChangeSet = nil

	art := Build(in)
	if art.Facts.CommitCount != 0 {
		t.Errorf("CommitCount = %d, want 0", art.Facts.CommitCount)
	}
	if art.Facts.Changes == nil {
		t.Error("Changes should be an empty slice, not nil")
	}
}

// The digest must distinguish inputs that differ, including in ways that naive
// concatenation would collide.
func TestDigest_DistinguishesInputs(t *testing.T) {
	base := DigestInputs{
		SchemaVersion: SchemaVersion,
		ToolVersion:   "4.3.0",
		Repository:    "acme/app",
		Branch:        "main",
		BaseRef:       "v1.0.0",
		HeadSHA:       "abc",
		ConfigHash:    "sha256:1",
		PolicySource:  "p",
	}

	tests := []struct {
		name   string
		mutate func(*DigestInputs)
	}{
		{"tool version", func(d *DigestInputs) { d.ToolVersion = "4.4.0" }},
		{"repository", func(d *DigestInputs) { d.Repository = "acme/other" }},
		{"branch", func(d *DigestInputs) { d.Branch = "develop" }},
		{"base ref", func(d *DigestInputs) { d.BaseRef = "v1.1.0" }},
		{"head sha", func(d *DigestInputs) { d.HeadSHA = "def" }},
		{"config hash", func(d *DigestInputs) { d.ConfigHash = "sha256:2" }},
		{"policy source", func(d *DigestInputs) { d.PolicySource = "q" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutated := base
			tt.mutate(&mutated)
			if base.Digest() == mutated.Digest() {
				t.Errorf("digest unchanged after changing %s", tt.name)
			}
		})
	}

	// Field-boundary collision: moving a character between adjacent fields must
	// change the digest. Length-prefixing is what guarantees this.
	a := DigestInputs{Repository: "ab", Branch: "c"}
	b := DigestInputs{Repository: "a", Branch: "bc"}
	if a.Digest() == b.Digest() {
		t.Error("digest collides across field boundaries; length-prefixing is not working")
	}
}

func TestDigest_StableAcrossCalls(t *testing.T) {
	d := DigestInputs{ToolVersion: "4.3.0", Repository: "acme/app", HeadSHA: "abc"}
	first := d.Digest()
	for i := 0; i < 5; i++ {
		if got := d.Digest(); got != first {
			t.Fatalf("digest changed between calls: %q then %q", first, got)
		}
	}
	if !strings.HasPrefix(first, "sha256:") {
		t.Errorf("digest = %q, want a sha256: prefix", first)
	}
}

// TestBuild_DigestCoversHeadSHA guards a bug this artifact shipped with briefly:
// HeadSHA was never populated by the CLI, so two different HEADs produced the
// same digest while provenance still claimed deterministic: true. A digest that
// omits an input it claims to cover is worse than no digest.
func TestBuild_DigestCoversHeadSHA(t *testing.T) {
	a := testInput(t)
	b := testInput(t)
	b.HeadSHA = "0000000000000000000000000000000000000000"

	da := Build(a).Provenance.InputsDigest
	db := Build(b).Provenance.InputsDigest

	if da == db {
		t.Error("digest is identical for different HEADs; it does not cover head_sha")
	}
}

// An artifact must never claim determinism while omitting a covered input.
func TestBuild_DigestCoversEveryClaimedInput(t *testing.T) {
	base := testInput(t)
	baseDigest := Build(base).Provenance.InputsDigest

	mutations := map[string]func(*BuildInput){
		"tool version":  func(i *BuildInput) { i.ToolVersion = "9.9.9" },
		"repository":    func(i *BuildInput) { i.Repository = "other/repo" },
		"branch":        func(i *BuildInput) { i.Branch = "develop" },
		"head sha":      func(i *BuildInput) { i.HeadSHA = "deadbeef" },
		"config hash":   func(i *BuildInput) { i.ConfigHash = "sha256:different" },
		"policy source": func(i *BuildInput) { i.PolicySource = "elsewhere" },
	}

	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			in := testInput(t)
			mutate(&in)
			if got := Build(in).Provenance.InputsDigest; got == baseDigest {
				t.Errorf("digest unchanged after altering %s", name)
			}
		})
	}
}
