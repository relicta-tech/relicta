package policy

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
)

// The DSL documents OR and NOT, the compiler emits them, and the engine had no
// code for either. `a OR b` compiles to one condition with Field "_or" and the two
// sides in Value; the engine called getNestedValue(evalCtx, "_or"), found no such
// field, recorded MissingField and reported the rule as not matched. Every rule
// using OR or NOT was inert — including six across the policies this project
// ships — and nothing said so, because "no field" and "did not apply" produce the
// same output.

func orCondition(left, right []map[string]any) Condition {
	return Condition{
		Field:    compositeFieldOr,
		Operator: "or",
		Value:    map[string]any{"left": left, "right": right},
	}
}

func comparison(field, operator string, value any) map[string]any {
	return map[string]any{"field": field, "operator": operator, "value": value}
}

func ruleWith(conds ...Condition) Policy {
	return Policy{
		Name: "test",
		Rules: []Rule{{
			ID: "r", Name: "r", Enabled: true, Priority: 10,
			Conditions: conds,
			Actions:    []Action{{Type: ActionRequireApproval, Params: map[string]any{"count": 1}}},
		}},
		Defaults: Defaults{Decision: "approve"},
	}
}

func evaluateBump(t *testing.T, pol Policy, bump cgp.BumpType) *Result {
	t.Helper()

	proposal := &cgp.ChangeProposal{
		Actor:  cgp.Actor{Kind: cgp.ActorKindHuman, ID: "human:dev"},
		Scope:  cgp.ProposalScope{Repository: "owner/repo", Files: []string{"main.go"}},
		Intent: cgp.ProposalIntent{SuggestedBump: bump},
	}
	result, err := NewEngine([]Policy{pol}, nil).
		Evaluate(context.Background(), proposal, &cgp.ChangeAnalysis{Fixes: 1}, 0.1)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	return result
}

func TestOrMatchesEitherSide(t *testing.T) {
	pol := ruleWith(orCondition(
		[]map[string]any{comparison("bump_type", OperatorEqual, "major")},
		[]map[string]any{comparison("bump_type", OperatorEqual, "patch")},
	))

	t.Run("left side", func(t *testing.T) {
		if len(evaluateBump(t, pol, cgp.BumpTypeMajor).MatchedRules) == 0 {
			t.Error("major satisfies the left side; the rule must match")
		}
	})

	t.Run("right side", func(t *testing.T) {
		if len(evaluateBump(t, pol, cgp.BumpTypePatch).MatchedRules) == 0 {
			t.Error("patch satisfies the right side; the rule must match")
		}
	})

	// An OR that matches everything would be as wrong as one that matches nothing,
	// and far harder to notice, since a policy that always applies looks strict.
	t.Run("neither side", func(t *testing.T) {
		if len(evaluateBump(t, pol, cgp.BumpTypeMinor).MatchedRules) != 0 {
			t.Error("minor satisfies neither side; the rule must not match")
		}
	})
}

func TestNotInvertsItsOperand(t *testing.T) {
	pol := ruleWith(Condition{
		Field:    compositeFieldNot,
		Operator: "not",
		Value:    []map[string]any{comparison("bump_type", OperatorEqual, "major")},
	})

	if len(evaluateBump(t, pol, cgp.BumpTypePatch).MatchedRules) == 0 {
		t.Error("NOT major must match a patch bump")
	}
	if len(evaluateBump(t, pol, cgp.BumpTypeMajor).MatchedRules) != 0 {
		t.Error("NOT major must not match a major bump")
	}
}

// Operands may themselves be composite. `(a OR b) OR c` nests, and so does the
// OR-chain the shipped policies write across several lines.
func TestNestedOrEvaluates(t *testing.T) {
	inner := orCondition(
		[]map[string]any{comparison("bump_type", OperatorEqual, "major")},
		[]map[string]any{comparison("bump_type", OperatorEqual, "minor")},
	)
	pol := ruleWith(orCondition(
		[]map[string]any{{"field": inner.Field, "operator": inner.Operator, "value": inner.Value}},
		[]map[string]any{comparison("bump_type", OperatorEqual, "patch")},
	))

	for _, bump := range []cgp.BumpType{cgp.BumpTypeMajor, cgp.BumpTypeMinor, cgp.BumpTypePatch} {
		if len(evaluateBump(t, pol, bump).MatchedRules) == 0 {
			t.Errorf("(major OR minor) OR patch must match %s", bump)
		}
	}
}

// A composite is AND within each side: `a AND b OR c` must not match on `a` alone.
func TestOrSideRequiresAllItsConditions(t *testing.T) {
	pol := ruleWith(orCondition(
		[]map[string]any{
			comparison("bump_type", OperatorEqual, "major"),
			comparison("change.fixes", OperatorGreaterThan, 99),
		},
		[]map[string]any{comparison("bump_type", OperatorEqual, "patch")},
	))

	if len(evaluateBump(t, pol, cgp.BumpTypeMajor).MatchedRules) != 0 {
		t.Error("the left side needs both conditions; only one holds, so it must not match")
	}
	if len(evaluateBump(t, pol, cgp.BumpTypePatch).MatchedRules) == 0 {
		t.Error("the right side holds on its own")
	}
}

// A malformed composite must not become a rule that fires on everything. Reading
// "no conditions" as vacuously true would turn a compiler bug into a policy that
// blocks or requires approval for every release.
func TestMalformedCompositeDoesNotMatch(t *testing.T) {
	for name, value := range map[string]any{
		"nil value":      nil,
		"wrong type":     "not a condition set",
		"empty operands": map[string]any{"left": []map[string]any{}, "right": []map[string]any{}},
	} {
		t.Run(name, func(t *testing.T) {
			pol := ruleWith(Condition{Field: compositeFieldOr, Operator: "or", Value: value})
			if len(evaluateBump(t, pol, cgp.BumpTypeMajor).MatchedRules) != 0 {
				t.Error("a malformed OR must not match")
			}
		})
	}
}

// contains had to read a list for path-ownership rules to work; it handled only
// string-in-string, so `scope.files contains "api/"` was false for any file list.
func TestContainsReadsLists(t *testing.T) {
	cases := []struct {
		name  string
		field any
		want  bool
	}{
		{"string field still works", "internal/api/handler.go", true},
		{"list of strings", []string{"README.md", "internal/api/handler.go"}, true},
		{"list after a JSON round trip", []any{"README.md", "internal/api/handler.go"}, true},
		{"list without a match", []string{"README.md", "docs/guide.md"}, false},
		{"empty list", []string{}, false},
		{"nil", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := valueContains(tc.field, "api/"); got != tc.want {
				t.Errorf("valueContains(%#v, %q) = %v, want %v", tc.field, "api/", got, tc.want)
			}
		})
	}
}
