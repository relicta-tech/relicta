package policy

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
)

// Five conditions written in the shipped example policies had no field or operator behind
// them and were removed rather than left looking active. Two of the gaps are closed here.
//
// "This list is empty" could not be expressed at all, so a rule meaning "the actor belongs to
// no team" — the case for requiring a reviewer from somewhere — had to be deleted. And the
// breadth of a change could only be asked as fileCount, which the backlog rejected as
// backwards: a large single-area change fires such a rule and a small cross-cutting one does
// not, which is the opposite of what a reviewer cares about.

func conditionMatches(t *testing.T, field, operator string, value any, ctx map[string]any) bool {
	t.Helper()

	engine := NewEngine([]Policy{{
		Name:     "test",
		Defaults: Defaults{Decision: DecisionApprove},
		Rules: []Rule{{
			ID: "r1", Name: "rule", Enabled: true, Priority: 1,
			Conditions: []Condition{{Field: field, Operator: operator, Value: value}},
			Actions:    []Action{{Type: ActionAddRationale, Params: map[string]any{"text": "matched"}}},
		}},
	}}, nil)

	matched, _, err := engine.evaluateRuleWithTrace(context.Background(), engine.policies[0].Rules[0], ctx)
	if err != nil {
		t.Fatalf("evaluating %s %s %v: %v", field, operator, value, err)
	}
	return matched
}

func TestIsEmptyExpressesMembershipOfNothing(t *testing.T) {
	noTeams := map[string]any{"actor": map[string]any{"teams": []string{}}}
	someTeams := map[string]any{"actor": map[string]any{"teams": []string{"platform"}}}

	if !conditionMatches(t, "actor.teams", OperatorIsEmpty, true, noTeams) {
		t.Error("actor.teams is_empty true did not match an actor with no teams — this is the " +
			"rule that had to be deleted for want of an operator")
	}
	if conditionMatches(t, "actor.teams", OperatorIsEmpty, true, someTeams) {
		t.Error("actor.teams is_empty true matched an actor who belongs to a team")
	}

	// And the negation, so a rule can require membership rather than only its absence.
	if !conditionMatches(t, "actor.teams", OperatorIsEmpty, false, someTeams) {
		t.Error("actor.teams is_empty false did not match an actor who belongs to a team")
	}
}

// A field the context does not carry does not match, which is the engine's existing
// convention for every operator and is recorded as MissingField in the rule trace.
//
// I expected is_empty to treat absent as empty, and wrote this test asserting that before
// checking. It is the wrong answer twice over. A rule about a field that does not exist
// should not fire in a governance tool, and making one operator disagree with the other nine
// would be a special case the policy author cannot see. What made the assumption look
// reasonable — "the actor belongs to no team must be true when no teams are configured" — is
// not actually at stake: buildEvalContext always sets actor.teams and actor.roles from the
// team context, so they are present-and-empty rather than absent, and that is the case
// is_empty answers.
func TestAnAbsentFieldDoesNotMatch(t *testing.T) {
	if conditionMatches(t, "actor.teams", OperatorIsEmpty, true, map[string]any{"actor": map[string]any{}}) {
		t.Error("a condition on a field the context does not carry matched; a rule about " +
			"unknown data must not fire")
	}

	// And the field really is always there, which is what makes the convention harmless
	// for the rule this operator exists for.
	ctx := buildEvalContext(
		&cgp.ChangeProposal{Actor: cgp.Actor{Kind: cgp.ActorKindHuman, ID: "human:nobody"}},
		nil, 0.1, DefaultTimeContext(), DefaultTeamContext(), "human:nobody",
	)
	actor, ok := ctx["actor"].(map[string]any)
	if !ok {
		t.Fatal("no actor in the evaluation context")
	}
	if _, present := actor["teams"]; !present {
		t.Error("actor.teams is absent from the context, so `actor.teams is_empty true` would " +
			"never fire and the rule this operator was added for still cannot be written")
	}
}

func TestSizeComparesCollectionLength(t *testing.T) {
	ctx := map[string]any{"scope": map[string]any{"files": []string{"a.go", "b.go", "c.go"}}}

	if !conditionMatches(t, "scope.files", OperatorSize, 3, ctx) {
		t.Error("scope.files size 3 did not match a three-file change")
	}
	if conditionMatches(t, "scope.files", OperatorSize, 2, ctx) {
		t.Error("scope.files size 2 matched a three-file change")
	}
}

// A non-collection must produce an error rather than a silent false: a rule comparing the
// size of a number is a mistake in the policy, and hiding it means the rule never fires and
// nobody learns why.
func TestSizeOfANonCollectionIsAnError(t *testing.T) {
	engine := NewEngine(nil, nil)
	if _, err := compareValues(0.7, OperatorSize, 3); err == nil {
		t.Error("size of a float returned no error; a policy mistake would then read as a " +
			"rule that simply does not apply")
	}
	_ = engine
}

func TestIsEmptyNeedsABoolean(t *testing.T) {
	if _, err := compareValues([]string{}, OperatorIsEmpty, "yes"); err == nil {
		t.Error(`is_empty with "yes" returned no error; the operator takes true or false, and ` +
			`accepting anything else would make a typo silently mean false`)
	}

	// A quoted boolean is accepted, because YAML makes the distinction invisible to the
	// policy author.
	if ok, err := compareValues([]string{}, OperatorIsEmpty, "true"); err != nil || !ok {
		t.Errorf(`is_empty "true" on an empty list = (%v, %v), want (true, nil)`, ok, err)
	}
}

// The breadth fields, which is the half of change.scope_count that can be answered from the
// paths the proposal already carries.
func TestChangeBreadthIsExposedSeparatelyFromFileCount(t *testing.T) {
	proposal := &cgp.ChangeProposal{
		Actor: cgp.Actor{Kind: cgp.ActorKindHuman, ID: "human:alice"},
		Scope: cgp.ProposalScope{
			Repository: "acme/widget",
			Files: []string{
				"internal/cgp/policy/engine.go",
				"internal/cgp/policy/policy.go",
				"web/src/App.vue",
				"terraform/main.tf",
				"go.mod",
			},
		},
	}

	ctx := buildEvalContext(proposal, nil, 0.5, DefaultTimeContext(), DefaultTeamContext(), "human:alice")
	scope, ok := ctx["scope"].(map[string]any)
	if !ok {
		t.Fatal("no scope in the evaluation context")
	}

	// Four areas: internal, web, terraform, and the root (go.mod).
	if got := scope["areaCount"]; got != 4 {
		t.Errorf("areaCount = %v, want 4 (internal, web, terraform, root)", got)
	}

	// Distinguishable from fileCount, which is the whole point: five files, four areas.
	if got := scope["fileCount"]; got != 5 {
		t.Errorf("fileCount = %v, want 5", got)
	}

	// Directories are finer: the two policy files share one.
	if got := scope["directoryCount"]; got != 4 {
		t.Errorf("directoryCount = %v, want 4 (the two policy files share a directory)", got)
	}
}

// A concentrated change and a spread one with the same file count must be distinguishable,
// because that is the distinction fileCount cannot make and the reason this field exists.
func TestBreadthDistinguishesConcentratedFromSpreadChanges(t *testing.T) {
	concentrated := []string{"internal/a/one.go", "internal/a/two.go", "internal/a/three.go"}
	spread := []string{"internal/a/one.go", "web/two.ts", "terraform/three.tf"}

	if got := len(topLevelAreas(concentrated)); got != 1 {
		t.Errorf("a change inside one area reported %d areas, want 1", got)
	}
	if got := len(topLevelAreas(spread)); got != 3 {
		t.Errorf("a change across three areas reported %d areas, want 3", got)
	}
}

// The rule the operator and the field together make expressible: a change spread across
// several areas needs a second reader, regardless of how small it is.
func TestACrossCuttingChangeCanBeRequiredToHaveAReviewer(t *testing.T) {
	engine := NewEngine([]Policy{{
		Name:     "breadth",
		Defaults: Defaults{Decision: DecisionApprove},
		Rules: []Rule{{
			ID: "cross-cutting", Name: "cross-cutting change", Enabled: true, Priority: 10,
			Description: "a change touching several areas needs review",
			Conditions: []Condition{
				{Field: "scope.areaCount", Operator: OperatorGreaterThan, Value: 2},
			},
			Actions: []Action{{Type: ActionSetDecision, Params: map[string]any{"decision": "require_review"}}},
		}},
	}}, nil)

	spread := &cgp.ChangeProposal{
		Actor: cgp.Actor{Kind: cgp.ActorKindAgent, ID: "agent:probe"},
		Scope: cgp.ProposalScope{
			Repository: "acme/widget",
			Files:      []string{"internal/a.go", "web/b.ts", "terraform/c.tf"},
		},
	}

	result, err := engine.Evaluate(context.Background(), spread, nil, 0.1)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.Decision != cgp.DecisionApprovalRequired {
		t.Errorf("decision = %q, want approval_required: a three-line change across three "+
			"areas is exactly what this rule is for, and it could not be written before",
			result.Decision)
	}
}
