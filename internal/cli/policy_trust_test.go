package cli

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy/dsl"
)

// A field a policy author cannot exercise is barely better than one that does not
// exist: `relicta policy test` builds its own actor, so without --trust-level
// every scenario would be Limited and a rule on actor.trusted would report
// itself as never matching for reasons the author cannot change.
const trustDSL = `
rule "trusted-auto-approve" {
  priority = 50
  description = "Auto-approve low-risk changes from trusted actors"

  when {
    actor.trusted == true AND risk.score < 0.3
  }

  then {
    approve()
  }
}

defaults {
  decision = "require_review"
  required_approvers = 1
}
`

func loadTrustPolicy(t *testing.T) []policy.Policy {
	t.Helper()
	pol, err := dsl.NewLoader(dsl.LoaderOptions{}).LoadString(trustDSL, "trust.policy")
	if err != nil {
		t.Fatalf("load trust policy: %v", err)
	}
	return []policy.Policy{*pol}
}

func TestPolicyTestScenario_TrustLevelChangesDecision(t *testing.T) {
	policies := loadTrustPolicy(t)

	cases := []struct {
		trustLevel   string
		wantDecision cgp.DecisionType
		wantMatch    bool
	}{
		{"trusted", cgp.DecisionApproved, true},
		{"full", cgp.DecisionApproved, true},
		{"limited", cgp.DecisionApprovalRequired, false},
		{"untrusted", cgp.DecisionApprovalRequired, false},
	}

	for _, tc := range cases {
		t.Run(tc.trustLevel, func(t *testing.T) {
			input := applyPolicyTestDefaults(policyTestInputData{
				RiskScore:  0.2,
				ActorType:  "human",
				TrustLevel: tc.trustLevel,
			})
			out, err := evaluatePolicyScenario(context.Background(), policies, &input)
			if err != nil {
				t.Fatalf("evaluatePolicyScenario: %v", err)
			}
			if out.Decision != tc.wantDecision {
				t.Errorf("decision = %s, want %s (matched %v)", out.Decision, tc.wantDecision, out.MatchedRules)
			}
			if got := len(out.MatchedRules) > 0; got != tc.wantMatch {
				t.Errorf("matched = %v, want %v (rules %v)", got, tc.wantMatch, out.MatchedRules)
			}
		})
	}
}

// The default has to be the un-elevated level, so a scenario that says nothing
// about trust is not silently granted it.
func TestPolicyTestScenario_DefaultTrustLevelIsLimited(t *testing.T) {
	input := applyPolicyTestDefaults(policyTestInputData{RiskScore: 0.2, ActorType: "human"})
	if input.TrustLevel != cgp.TrustLevelLimited.String() {
		t.Fatalf("default trust level = %q, want limited", input.TrustLevel)
	}

	out, err := evaluatePolicyScenario(context.Background(), loadTrustPolicy(t), &input)
	if err != nil {
		t.Fatalf("evaluatePolicyScenario: %v", err)
	}
	if out.Decision != cgp.DecisionApprovalRequired {
		t.Fatalf("an actor with default trust must not auto-approve; got %s", out.Decision)
	}
}

// The reputation half of the same property: a scenario that says nothing about
// reputation must not fabricate one, and a scenario that states one must be able
// to fire a rule that reads it.
func TestPolicyTestScenario_ReputationIsOptionalAndUsable(t *testing.T) {
	src := `
rule "weak-record-review" {
  priority = 50
  description = "Review changes from actors with a weak record"

  when {
    actor.reputation.overall < 0.5
  }

  then {
    require_approval(count: 1)
  }
}

defaults {
  decision = "approve"
  required_approvers = 0
}
`
	pol, err := dsl.NewLoader(dsl.LoaderOptions{}).LoadString(src, "reputation.policy")
	if err != nil {
		t.Fatalf("load reputation policy: %v", err)
	}
	policies := []policy.Policy{*pol}

	weak, strong := 0.2, 0.95
	cases := []struct {
		name         string
		reputation   *float64
		wantDecision cgp.DecisionType
	}{
		{"no reputation stated", nil, cgp.DecisionApproved},
		{"weak record", &weak, cgp.DecisionApprovalRequired},
		{"strong record", &strong, cgp.DecisionApproved},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := applyPolicyTestDefaults(policyTestInputData{
				RiskScore:  0.2,
				ActorType:  "human",
				Reputation: tc.reputation,
			})
			out, err := evaluatePolicyScenario(context.Background(), policies, &input)
			if err != nil {
				t.Fatalf("evaluatePolicyScenario: %v", err)
			}
			if out.Decision != tc.wantDecision {
				t.Errorf("decision = %s, want %s (matched %v)", out.Decision, tc.wantDecision, out.MatchedRules)
			}
		})
	}
}

func TestPolicyTestScenario_ReputationInputIsValidated(t *testing.T) {
	tooHigh, negative := 1.5, -0.1
	cases := []struct {
		name  string
		input policyTestInputData
	}{
		{"above one", policyTestInputData{ActorType: "human", TrustLevel: "limited", BumpType: "patch", Reputation: &tooHigh}},
		{"below zero", policyTestInputData{ActorType: "human", TrustLevel: "limited", BumpType: "patch", Reputation: &negative}},
		{"unknown trend", policyTestInputData{
			ActorType: "human", TrustLevel: "limited", BumpType: "patch",
			Reputation: &tooHigh, ReputationTrend: "sideways",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := evaluatePolicyScenario(context.Background(), loadTrustPolicy(t), &tc.input); err == nil {
				t.Fatal("expected an error for an out-of-range reputation input")
			}
		})
	}
}

// The band must be derived from the score, so a scenario cannot claim 0.1 overall
// and a "trusted" level — a rule reading one field and a human reading the other
// would disagree about the same scenario.
func TestPolicyTestReputationContext_DerivesLevel(t *testing.T) {
	cases := map[float64]string{0.95: "trusted", 0.7: "reliable", 0.45: "probation", 0.1: "restricted"}
	for overall, wantLevel := range cases {
		score := overall
		got, err := policyTestReputationContext(&policyTestInputData{Reputation: &score, ReputationSamples: 10})
		if err != nil {
			t.Fatalf("policyTestReputationContext(%v): %v", overall, err)
		}
		if got.Level != wantLevel {
			t.Errorf("level for %v = %q, want %q", overall, got.Level, wantLevel)
		}
		if got.Trend != "stable" {
			t.Errorf("trend for %v = %q, want stable", overall, got.Trend)
		}
	}
}

// An unrecognized trust level is rejected rather than silently read as untrusted:
// a typo that quietly downgraded the actor would make a rule look inert.
func TestPolicyTestScenario_InvalidTrustLevelIsRejected(t *testing.T) {
	input := policyTestInputData{RiskScore: 0.2, ActorType: "human", TrustLevel: "very-trusted"}
	if _, err := evaluatePolicyScenario(context.Background(), loadTrustPolicy(t), &input); err == nil {
		t.Fatal("expected an error for an unrecognized trust level")
	}
}
