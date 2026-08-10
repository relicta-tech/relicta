package templates

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy/dsl"
)

// Parsing is not the property that matters. Every shipped policy parsed cleanly
// while referencing fields the evaluator does not provide, so all of them loaded,
// all of them reported their rules as enabled, and none of them could ever fire.
//
// The worst case was time-based.policy: with it installed, a release scoring 0.95
// risk on a major bump came back `approved`, rationale "Applied default policy" —
// while the policy contained a rule named "freeze-period-block". The conditions
// said time.is_freeze, time.is_weekend, time.day_of_week; the evaluator provides
// time.freeze.active, time.isWeekend, time.weekday.
//
// These tests pin the clock and assert the rules actually change the decision.
// A test that only parsed would have passed throughout.

func loadEmbedded(t *testing.T, name string) policy.Policy {
	t.Helper()

	starter, err := PolicyStarterByName(name)
	if err != nil {
		t.Fatalf("PolicyStarterByName(%q): %v", name, err)
	}
	pol, err := dsl.NewLoader(dsl.LoaderOptions{}).LoadString(starter.Content, starter.Filename)
	if err != nil {
		t.Fatalf("load %s: %v", starter.Filename, err)
	}
	return *pol
}

// proposalAt is a low-risk patch by a human — deliberately the least alarming
// change possible, so anything that matches does so because of the rule under
// test and not because the change itself is risky.
func proposalAt(files ...string) (*cgp.ChangeProposal, *cgp.ChangeAnalysis) {
	proposal := &cgp.ChangeProposal{
		Actor: cgp.Actor{Kind: cgp.ActorKindHuman, ID: "human:dev", Name: "Dev"},
		Scope: cgp.ProposalScope{
			Repository:  "owner/repo",
			Branch:      "main",
			CommitRange: "HEAD~1..HEAD",
			Files:       files,
		},
		Intent: cgp.ProposalIntent{Summary: "small change", SuggestedBump: cgp.BumpTypePatch},
	}
	analysis := &cgp.ChangeAnalysis{
		Fixes:       1,
		BlastRadius: &cgp.BlastRadius{Score: 0.05, FilesChanged: len(files), LinesChanged: 10},
	}
	return proposal, analysis
}

func matched(result *policy.Result, ruleID string) bool {
	for _, id := range result.MatchedRules {
		if id == ruleID || strings.ReplaceAll(id, "_", "-") == ruleID {
			return true
		}
	}
	return false
}

func TestTimeBasedPolicyFires(t *testing.T) {
	// A Saturday afternoon, and a Friday at 16:00, in a fixed zone so the test
	// does not depend on where it runs.
	saturday := time.Date(2026, 8, 8, 14, 0, 0, 0, time.UTC)
	friday := time.Date(2026, 8, 7, 16, 0, 0, 0, time.UTC)
	tuesdayMidday := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name     string
		at       time.Time
		wantRule string
	}{
		{"weekend release needs senior approval", saturday, "weekend-approval"},
		{"friday afternoon needs care", friday, "friday-afternoon-caution"},
	}

	pol := loadEmbedded(t, "time-based")
	proposal, analysis := proposalAt("README.md")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			engine := policy.NewEngine([]policy.Policy{pol}, nil).
				WithTimeContext(policy.DefaultTimeContext().WithTime(tc.at))

			result, err := engine.Evaluate(context.Background(), proposal, analysis, 0.05)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if !matched(result, tc.wantRule) {
				t.Errorf("expected rule %q to match at %s; matched %v (decision %s)",
					tc.wantRule, tc.at.Format(time.RFC1123), result.MatchedRules, result.Decision)
			}
			if result.Decision == cgp.DecisionApproved {
				t.Errorf("a matched approval rule must change the decision; got %s", result.Decision)
			}
		})
	}

	// The negative case matters as much: a Tuesday midday release must not trip
	// the weekend or Friday rules, or the policy is merely noisy rather than wrong.
	t.Run("an ordinary weekday midday release is untouched", func(t *testing.T) {
		engine := policy.NewEngine([]policy.Policy{pol}, nil).
			WithTimeContext(policy.DefaultTimeContext().WithTime(tuesdayMidday))

		result, err := engine.Evaluate(context.Background(), proposal, analysis, 0.05)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		for _, unwanted := range []string{"weekend-approval", "friday-afternoon-caution", "freeze-period-block"} {
			if matched(result, unwanted) {
				t.Errorf("rule %q matched on a Tuesday at midday", unwanted)
			}
		}
	})
}

// The rule the original defect was most visible in: a policy containing
// "freeze-period-block" that blocked nothing.
func TestFreezePeriodBlocks(t *testing.T) {
	pol := loadEmbedded(t, "time-based")
	during := time.Date(2026, 12, 24, 11, 0, 0, 0, time.UTC)

	engine := policy.NewEngine([]policy.Policy{pol}, nil).
		WithTimeContext(policy.DefaultTimeContext().WithTime(during)).
		AddFreezePeriod(policy.FreezePeriod{
			Name:     "year-end",
			Reason:   "change freeze",
			Severity: "hard",
			Start:    time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC),
			End:      time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC),
		})

	proposal, analysis := proposalAt("README.md")
	result, err := engine.Evaluate(context.Background(), proposal, analysis, 0.05)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if !matched(result, "freeze-period-block") {
		t.Fatalf("a release inside a hard freeze must match freeze-period-block; matched %v", result.MatchedRules)
	}
	if !result.Blocked {
		t.Errorf("freeze-period-block calls block(); the result must be blocked, got decision %s", result.Decision)
	}
}

// Path-ownership rules were inexpressible: the evaluator exposed only
// scope.fileCount, so `scope.files contains "terraform/"` had nothing to read,
// and `contains` could not match inside a list even once the field existed.
func TestPathOwnershipRulesMatchChangedFiles(t *testing.T) {
	pol := loadEmbedded(t, "team-based")

	t.Run("an infrastructure change reaches the platform rule", func(t *testing.T) {
		proposal, analysis := proposalAt("terraform/main.tf", "README.md")
		result, err := policy.NewEngine([]policy.Policy{pol}, nil).
			Evaluate(context.Background(), proposal, analysis, 0.05)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if !matched(result, "platform-ownership") {
			t.Errorf("a change to terraform/ must match platform-ownership; matched %v", result.MatchedRules)
		}
	})

	t.Run("an unrelated change does not", func(t *testing.T) {
		proposal, analysis := proposalAt("docs/guide.md")
		result, err := policy.NewEngine([]policy.Policy{pol}, nil).
			Evaluate(context.Background(), proposal, analysis, 0.05)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if matched(result, "platform-ownership") {
			t.Error("a docs-only change must not match platform-ownership")
		}
	})
}

// agent-aware.policy's "trusted-human" rule carried `actor.trusted == true`,
// which the evaluator did not provide, so the clause was deleted to stop the rule
// lying about what it checked (PR #249). With actor.trusted exposed the clause is
// back, and this pins what it does: an actor who has earned trust reaches the
// rule, an actor who has not falls through to the policy defaults instead. The
// difference is visible in the recorded rationale, which is what an audit reads.
func TestAgentAwarePolicyReadsEarnedTrust(t *testing.T) {
	pol := loadEmbedded(t, "agent-aware")
	engine := policy.NewEngine([]policy.Policy{pol}, nil)

	evaluate := func(t *testing.T, level cgp.TrustLevel) *policy.Result {
		t.Helper()
		proposal, analysis := proposalAt("README.md")
		proposal.Actor.TrustLevel = level
		result, err := engine.Evaluate(context.Background(), proposal, analysis, 0.05)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		return result
	}

	t.Run("an actor who has earned trust reaches the rule", func(t *testing.T) {
		result := evaluate(t, cgp.TrustLevelTrusted)
		if !matched(result, "trusted-human") {
			t.Fatalf("a trusted human's low-risk change must match trusted-human; matched %v", result.MatchedRules)
		}
		for _, line := range result.Rationale {
			if strings.Contains(line, "Applied default policy") {
				t.Error("a matched rule must supply the rationale, not the defaults")
			}
		}
	})

	t.Run("an actor who has not earned trust does not", func(t *testing.T) {
		result := evaluate(t, cgp.TrustLevelLimited)
		if matched(result, "trusted-human") {
			t.Fatalf("trusted-human must not match an actor that has not earned trust; matched %v", result.MatchedRules)
		}
		if len(result.MatchedRules) != 0 {
			t.Fatalf("no rule should match a low-risk human patch; matched %v", result.MatchedRules)
		}
		var fellThrough bool
		for _, line := range result.Rationale {
			if strings.Contains(line, "Applied default policy") {
				fellThrough = true
			}
		}
		if !fellThrough {
			t.Errorf("expected the defaults to decide; rationale was %v", result.Rationale)
		}
	})
}

// The guarantee behind `relicta policy init`: nothing it writes may contain a
// condition the evaluator cannot resolve. This is the check that turns the class
// of defect into a build failure rather than a surprise during a release.
func TestNoEmbeddedPolicyReferencesAnUnknownField(t *testing.T) {
	starters, err := PolicyStarters()
	if err != nil {
		t.Fatalf("PolicyStarters: %v", err)
	}

	loader := dsl.NewLoader(dsl.LoaderOptions{})
	for _, s := range starters {
		pol, err := loader.LoadString(s.Content, s.Filename)
		if err != nil {
			t.Fatalf("load %s: %v", s.Filename, err)
		}
		for _, rule := range pol.Rules {
			for _, field := range policy.UnknownFields(rule.Conditions) {
				suggestion := ""
				if sug, ok := policy.SuggestFieldPath(field); ok {
					suggestion = " (did you mean " + sug + "?)"
				}
				t.Errorf("%s rule %q references %q, which the evaluator does not provide%s — "+
					"the rule can never match", s.Filename, rule.Name, field, suggestion)
			}
		}
	}
}
