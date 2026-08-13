package policy

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
)

// The trust machinery decided things no policy could see: actor.trusted was
// written in the shipped agent-aware policy, the evaluator provided no such
// field, so the rule could never fire and nothing said so. These tests hold the
// two properties that matter — the fields resolve, and a rule conditioning on
// them changes the decision. A test that only asserted the field exists would
// have passed while the rule stayed inert.

func trustProposal(level cgp.TrustLevel) *cgp.ChangeProposal {
	return cgp.NewProposal(
		cgp.Actor{Kind: cgp.ActorKindHuman, ID: "human:dev", Name: "Dev", TrustLevel: level},
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "small change", SuggestedBump: cgp.BumpTypePatch, Confidence: 1.0},
	)
}

// trustedAutoApprovePolicy is the rule the backlog entry describes: auto-approve
// low-risk changes from actors who have earned it. Its defaults require review,
// so the rule is the only thing that can produce an approval — if the condition
// cannot resolve, the decision stays approval_required and the test fails.
func trustedAutoApprovePolicy() Policy {
	pol := NewPolicy("earned-trust")
	pol.Defaults = Defaults{Decision: DecisionRequireReview, RequiredApprovers: 1}
	pol.AddRule(*NewRule("trusted-auto-approve", "Auto-approve low-risk changes from trusted actors").
		WithPriority(50).
		AddCondition("actor.trusted", OperatorEqual, true).
		AddCondition("risk.score", OperatorLessThan, 0.3).
		AddAction(ActionSetDecision, map[string]any{"decision": "approve"}))
	return *pol
}

func TestActorTrustFieldsAreResolvable(t *testing.T) {
	for _, field := range []string{
		"actor.trusted",
		"actor.trustLevel",
		"actor.reputation",
		"actor.reputation.overall",
		"actor.reputation.level",
		"actor.reputation.samples",
		"actor.reputation.trend",
	} {
		if !IsKnownFieldPath(field) {
			t.Errorf("IsKnownFieldPath(%q) = false; a policy condition on it would be reported as unknown", field)
		}
	}
}

// The property the whole change exists for: earned trust flips a decision.
func TestTrustedActorChangesTheDecision(t *testing.T) {
	engine := NewEngine([]Policy{trustedAutoApprovePolicy()}, nil)

	trusted, err := engine.Evaluate(context.Background(), trustProposal(cgp.TrustLevelTrusted), nil, 0.1)
	if err != nil {
		t.Fatalf("evaluate trusted actor: %v", err)
	}
	if trusted.Decision != cgp.DecisionApproved {
		t.Errorf("a trusted actor's low-risk change must be approved; got %s (matched %v)",
			trusted.Decision, trusted.MatchedRules)
	}
	if len(trusted.MatchedRules) != 1 || trusted.MatchedRules[0] != "trusted-auto-approve" {
		t.Errorf("expected trusted-auto-approve to match; matched %v", trusted.MatchedRules)
	}

	// Same change, same policy, an actor who has not earned trust: the rule must
	// not match and the defaults must stand. Without this half the test would pass
	// against a field hardcoded to true.
	limited, err := engine.Evaluate(context.Background(), trustProposal(cgp.TrustLevelLimited), nil, 0.1)
	if err != nil {
		t.Fatalf("evaluate limited actor: %v", err)
	}
	if limited.Decision != cgp.DecisionApprovalRequired {
		t.Errorf("an actor who has not earned trust must still need review; got %s", limited.Decision)
	}
	if len(limited.MatchedRules) != 0 {
		t.Errorf("trusted-auto-approve must not match a limited actor; matched %v", limited.MatchedRules)
	}
}

// Full autonomy is above Trusted, so it satisfies actor.trusted too — a rule
// written for trusted actors must not exclude the most trusted ones.
func TestTrustLevelNamesAreExposed(t *testing.T) {
	cases := map[cgp.TrustLevel]struct {
		name    string
		trusted bool
	}{
		cgp.TrustLevelUntrusted: {"untrusted", false},
		cgp.TrustLevelLimited:   {"limited", false},
		cgp.TrustLevelTrusted:   {"trusted", true},
		cgp.TrustLevelFull:      {"full", true},
	}

	for level, want := range cases {
		ctx := buildEvalContext(trustProposal(level), nil, 0.1, DefaultTimeContext(), DefaultTeamContext(), "human:dev")
		actor, ok := ctx["actor"].(map[string]any)
		if !ok {
			t.Fatal("actor context missing")
		}
		if got := actor["trustLevel"]; got != want.name {
			t.Errorf("trustLevel for %s = %v, want %q", level, got, want.name)
		}
		if got := actor["trusted"]; got != want.trusted {
			t.Errorf("trusted for %s = %v, want %v", level, got, want.trusted)
		}
	}
}

// A reputation that was never computed must be ABSENT, not zero. Zero is the
// score of an actor with a demonstrably bad record, so reporting it where nothing
// was computed would make `actor.reputation.overall < 0.5` fire for every actor in
// a deployment that does not compute reputation at all — the rule would look like
// it was working.
func TestReputationIsAbsentWhenNotComputed(t *testing.T) {
	proposal := trustProposal(cgp.TrustLevelLimited)

	ctx := buildEvalContext(proposal, nil, 0.1, DefaultTimeContext(), DefaultTeamContext(), "human:dev")
	actor := ctx["actor"].(map[string]any)
	if _, present := actor["reputation"]; present {
		t.Fatalf("reputation must be absent when none was computed; got %v", actor["reputation"])
	}
	if _, ok := getNestedValue(ctx, "actor.reputation.overall"); ok {
		t.Error("actor.reputation.overall resolved without a computed reputation")
	}

	proposal.Context = &cgp.ProposalContext{ActorReputation: &cgp.ActorReputation{
		Overall: 0.91, Level: "trusted", SampleSize: 20, Trend: "improving",
	}}
	ctx = buildEvalContext(proposal, nil, 0.1, DefaultTimeContext(), DefaultTeamContext(), "human:dev")
	value, ok := getNestedValue(ctx, "actor.reputation.overall")
	if !ok {
		t.Fatal("actor.reputation.overall must resolve once a reputation is attached")
	}
	if value != 0.91 {
		t.Errorf("actor.reputation.overall = %v, want 0.91", value)
	}
	for path, want := range map[string]any{
		"actor.reputation.level":   "trusted",
		"actor.reputation.samples": 20,
		"actor.reputation.trend":   "improving",
	} {
		got, ok := getNestedValue(ctx, path)
		if !ok {
			t.Errorf("%s did not resolve", path)
			continue
		}
		if got != want {
			t.Errorf("%s = %v, want %v", path, got, want)
		}
	}
}

// A condition on an absent reputation must report itself as a missing field
// rather than as a comparison that simply did not hold — that trace is the only
// signal a policy author gets that the rule cannot fire here.
func TestReputationConditionReportsMissingField(t *testing.T) {
	pol := NewPolicy("reputation-guard")
	pol.Defaults = Defaults{Decision: DecisionApprove}
	pol.AddRule(*NewRule("weak-record-review", "Review changes from actors with a weak record").
		WithPriority(50).
		AddCondition("actor.reputation.overall", OperatorLessThan, 0.5).
		AddAction(ActionRequireApproval, map[string]any{"count": float64(1)}))

	engine := NewEngine([]Policy{*pol}, nil)
	result, err := engine.Evaluate(context.Background(), trustProposal(cgp.TrustLevelLimited), nil, 0.1)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(result.MatchedRules) != 0 {
		t.Errorf("a rule reading an uncomputed reputation must not match; matched %v", result.MatchedRules)
	}

	var reported bool
	for _, rule := range result.RuleTrace {
		for _, cond := range rule.Conditions {
			if cond.Field == "actor.reputation.overall" && cond.MissingField {
				reported = true
			}
		}
	}
	if !reported {
		t.Error("the trace must mark actor.reputation.overall as missing, not merely unmatched")
	}
}

// A weak record must tighten the decision once reputation IS computed, which is
// the mirror of the trusted case: the same policy, the same change, one field.
func TestReputationChangesTheDecisionWhenComputed(t *testing.T) {
	pol := NewPolicy("reputation-guard")
	pol.Defaults = Defaults{Decision: DecisionApprove}
	pol.AddRule(*NewRule("weak-record-review", "Review changes from actors with a weak record").
		WithPriority(50).
		AddCondition("actor.reputation.overall", OperatorLessThan, 0.5).
		AddAction(ActionRequireApproval, map[string]any{"count": float64(1)}))
	engine := NewEngine([]Policy{*pol}, nil)

	weak := trustProposal(cgp.TrustLevelLimited)
	weak.Context = &cgp.ProposalContext{ActorReputation: &cgp.ActorReputation{
		Overall: 0.2, Level: "restricted", SampleSize: 12, Trend: "declining",
	}}
	result, err := engine.Evaluate(context.Background(), weak, nil, 0.1)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Decision != cgp.DecisionApprovalRequired {
		t.Errorf("a weak record must require review; got %s (matched %v)", result.Decision, result.MatchedRules)
	}

	strong := trustProposal(cgp.TrustLevelLimited)
	strong.Context = &cgp.ProposalContext{ActorReputation: &cgp.ActorReputation{
		Overall: 0.95, Level: "trusted", SampleSize: 60, Trend: "stable",
	}}
	result, err = engine.Evaluate(context.Background(), strong, nil, 0.1)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Decision != cgp.DecisionApproved {
		t.Errorf("a strong record must stay approved; got %s (matched %v)", result.Decision, result.MatchedRules)
	}
}
