// Package recommendation builds the deterministic recommendation artifact that
// Relicta emits instead of prose.
//
// The artifact carries the facts Relicta derived, the assessment it computed,
// the verdict it recommends, the obligations that remain, and the provenance
// needed to verify all of it. Turning that into language is the caller's job —
// an agent, a template, or Relicta Hub. See docs/adr/009.
//
// Nothing in this package generates text. Every string is either copied from a
// structured source or formatted from a number, so two runs over the same inputs
// produce byte-identical output. DeterminismTest in the test file asserts that.
package recommendation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// SchemaVersion is the artifact's contract version. It is deliberately
// independent of the CLI version: agents and Hub consume this shape, so it
// changes only when the shape changes.
const SchemaVersion = "1.0.0"

// Artifact is the deterministic recommendation.
type Artifact struct {
	SchemaVersion string    `json:"schema_version"`
	GeneratedAt   time.Time `json:"generated_at"`

	Subject     Subject      `json:"subject"`
	Proposal    *Proposal    `json:"proposal,omitempty"`
	Facts       Facts        `json:"facts"`
	Assessment  *Assessment  `json:"assessment,omitempty"`
	Verdict     *Verdict     `json:"verdict,omitempty"`
	Obligations []Obligation `json:"obligations"`
	Provenance  Provenance   `json:"provenance"`
}

// Subject identifies what was assessed.
type Subject struct {
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	BaseRef    string `json:"base_ref,omitempty"`
	HeadSHA    string `json:"head_sha,omitempty"`
}

// Proposal records who proposed the change and what they claimed.
//
// This is where confidence belongs. A confidence number on a deterministic
// computation invites "confidence in what?"; the proposer's own stated
// confidence is a fact, and policy can already condition on it.
type Proposal struct {
	Actor  ProposalActor  `json:"actor"`
	Intent ProposalIntent `json:"intent"`
}

// ProposalActor is the actor that proposed the change.
type ProposalActor struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// ProposalIntent is what the proposer claimed about their own change.
type ProposalIntent struct {
	Summary       string `json:"summary,omitempty"`
	SuggestedBump string `json:"suggested_bump,omitempty"`
	// DeclaredConfidence is the proposer's own confidence (0.0-1.0). Policy can
	// condition on it via the `intent.confidence` field.
	DeclaredConfidence *float64 `json:"declared_confidence,omitempty"`
}

// Facts is what Relicta derived from the repository. Writer-ready: a caller
// producing release notes should not need a second call to get the material.
type Facts struct {
	CurrentVersion string `json:"current_version"`
	NextVersion    string `json:"next_version"`
	ReleaseType    string `json:"release_type"`
	CommitCount    int    `json:"commit_count"`

	Changes         []Change `json:"changes"`
	BreakingChanges []Change `json:"breaking_changes"`
}

// Change is one classified commit.
type Change struct {
	Type     string `json:"type,omitempty"`
	Scope    string `json:"scope,omitempty"`
	Subject  string `json:"subject"`
	SHA      string `json:"sha,omitempty"`
	Breaking bool   `json:"breaking"`
}

// Assessment is what Relicta computed about the change.
type Assessment struct {
	RiskScore float64      `json:"risk_score"`
	Severity  string       `json:"severity,omitempty"`
	Factors   []RiskFactor `json:"factors"`

	Thresholds *Thresholds `json:"thresholds,omitempty"`

	// Policy is populated only when rule-level detail is available. It is
	// currently omitted: EvaluateReleaseOutput does not surface policy.Result,
	// so the rule trace cannot be reached from here without new plumbing.
	// Omitting it is preferable to inventing it. See docs/adr/009.
	Policy *Policy `json:"policy,omitempty"`
}

// RiskFactor is one contribution to the risk score. Structured deliberately:
// the CLI previously flattened these into formatted strings such as
// "[api_change] description (10%)", discarding the fields a consumer needs.
type RiskFactor struct {
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`
	Severity    string  `json:"severity,omitempty"`
}

// Thresholds is what the score was compared against, so a reader can check the
// verdict rather than take it on trust.
type Thresholds struct {
	AutoApproveBelow        float64 `json:"auto_approve_below"`
	MaxAutoApproveRisk      float64 `json:"max_auto_approve_risk"`
	RequireHumanForBreaking bool    `json:"require_human_for_breaking"`
	RequireHumanForSecurity bool    `json:"require_human_for_security"`
}

// Policy is the rule-level evaluation detail.
type Policy struct {
	MatchedRules      []string `json:"matched_rules"`
	RequiredApprovers int      `json:"required_approvers"`
	Blocked           bool     `json:"blocked"`
	BlockReason       string   `json:"block_reason,omitempty"`
}

// Verdict is what Relicta recommends. It recommends; it does not act.
type Verdict struct {
	Decision           string   `json:"decision"`
	RecommendedVersion string   `json:"recommended_version"`
	CanAutoApprove     bool     `json:"can_auto_approve"`
	Rationale          []string `json:"rationale"`
}

// Obligation is something that must still happen before publishing.
type Obligation struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Blocking    bool   `json:"blocking"`
	Assignee    string `json:"assignee,omitempty"`
}

// Provenance is what a reader needs to verify the artifact.
type Provenance struct {
	ToolVersion  string `json:"tool_version"`
	ConfigHash   string `json:"config_hash,omitempty"`
	PlanHash     string `json:"plan_hash,omitempty"`
	PolicySource string `json:"policy_source,omitempty"`

	// Deterministic is false when any part of the artifact could not be derived
	// reproducibly. It is a claim InputsDigest backs, not decoration.
	Deterministic bool `json:"deterministic"`

	// InputsDigest covers everything that determines Facts, Assessment and
	// Verdict. Two artifacts with the same digest must agree on those three
	// sections; GeneratedAt is deliberately excluded.
	InputsDigest string `json:"inputs_digest"`
}

// DigestInputs are the values that determine the artifact's derived sections.
//
// Tool version is included: a different Relicta may legitimately compute a
// different verdict from identical repository state, and the digest must not
// claim otherwise.
type DigestInputs struct {
	SchemaVersion string
	ToolVersion   string
	Repository    string
	Branch        string
	BaseRef       string
	HeadSHA       string
	ConfigHash    string
	PolicySource  string
}

// Digest computes the inputs digest. The encoding is length-prefixed so that no
// combination of field values can collide by concatenation — "ab"+"c" and
// "a"+"bc" must not produce the same digest.
func (d DigestInputs) Digest() string {
	fields := []string{
		d.SchemaVersion,
		d.ToolVersion,
		d.Repository,
		d.Branch,
		d.BaseRef,
		d.HeadSHA,
		d.ConfigHash,
		d.PolicySource,
	}

	var b strings.Builder
	for _, f := range fields {
		fmt.Fprintf(&b, "%d:%s\n", len(f), f)
	}

	sum := sha256.Sum256([]byte(b.String()))
	return "sha256:" + hex.EncodeToString(sum[:])
}
