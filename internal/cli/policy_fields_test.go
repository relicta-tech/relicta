package cli

import (
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
)

// A condition on a field the evaluator does not provide makes its rule silently
// unmatchable. `policy validate` checked syntax only, so a policy could be
// well-formed and completely inert — which four of the five policies shipped in
// this repository were. These cover the reporting that makes that visible.

func TestUnresolvableFieldsFindsAMistypedField(t *testing.T) {
	pol := &policy.Policy{
		Name: "example",
		Rules: []policy.Rule{{
			ID: "r", Name: "weekend-guard", Enabled: true,
			Conditions: []policy.Condition{
				{Field: "time.is_weekend", Operator: policy.OperatorEqual, Value: true},
			},
		}},
	}

	findings := unresolvableFields("policy \"example\"", pol)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].field != "time.is_weekend" {
		t.Errorf("finding names %q, want time.is_weekend", findings[0].field)
	}
	if findings[0].rule != "weekend-guard" {
		t.Errorf("a finding must name the rule it came from; got %q", findings[0].rule)
	}
}

func TestUnresolvableFieldsAcceptsCorrectFields(t *testing.T) {
	pol := &policy.Policy{
		Name: "example",
		Rules: []policy.Rule{{
			ID: "r", Name: "ok", Enabled: true,
			Conditions: []policy.Condition{
				{Field: "time.isWeekend", Operator: policy.OperatorEqual, Value: true},
				{Field: "risk.score", Operator: policy.OperatorGreaterThan, Value: 0.5},
				{Field: "scope.files", Operator: policy.OperatorContains, Value: "api/"},
			},
		}},
	}

	if findings := unresolvableFields("policy \"example\"", pol); len(findings) != 0 {
		t.Errorf("correct fields must produce no findings; got %v", findings)
	}
}

// The exit status is what CI reads. A warning that never fails is useless in a
// pipeline, and a failure that cannot be turned off blocks a policy written ahead
// of a field a later release provides — hence one of each.
func TestReportUnresolvableFieldsRespectsStrict(t *testing.T) {
	restore := policyValidateStrict
	t.Cleanup(func() { policyValidateStrict = restore })

	findings := []fieldFinding{{source: "policy \"p\"", rule: "r", field: "time.is_weekend"}}

	policyValidateStrict = false
	if err := reportUnresolvableFields(findings); err != nil {
		t.Errorf("without --strict this is a warning, not a failure; got %v", err)
	}

	policyValidateStrict = true
	err := reportUnresolvableFields(findings)
	if err == nil {
		t.Fatal("--strict must fail when a condition can never match")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("the error should say what is wrong; got %v", err)
	}

	// And nothing to report is never a failure, strict or not.
	if err := reportUnresolvableFields(nil); err != nil {
		t.Errorf("no findings must not fail even with --strict; got %v", err)
	}
}

// policy fields is what the validation message tells the reader to run, so it has
// to exist and to list the fields the same enumeration validates against.
func TestPolicyFieldsIsRegisteredAndNonEmpty(t *testing.T) {
	found := false
	for _, c := range policyCmd.Commands() {
		if c.Name() == "fields" {
			found = true
			break
		}
	}
	if !found {
		t.Error("`relicta policy fields` is not registered; the validation message names it")
	}

	fields := policy.KnownFieldPaths()
	if len(fields) == 0 {
		t.Fatal("the evaluator reports no fields")
	}

	// Spot-check the ones the shipped policies depend on, so a change to the
	// evaluator that drops them fails here rather than silently making those
	// policies inert again.
	for _, required := range []string{"risk.score", "change.breaking", "scope.files", "time.freeze.active", "bump_type"} {
		if !policy.IsKnownFieldPath(required) {
			t.Errorf("%q is used by a shipped policy but the evaluator no longer provides it", required)
		}
	}
}

// A container is not a value. Offering `time` alongside `time.isWeekend` would
// invite a comparison against a map, which can never be true.
func TestPolicyFieldsExcludesBareContainers(t *testing.T) {
	fields := policy.KnownFieldPaths()
	for _, container := range []string{"time", "risk", "change", "scope", "actor"} {
		if !isContextContainer(container, fields) {
			t.Errorf("%q should be recognized as a container of other fields", container)
		}
	}
	// A leaf is not a container.
	if isContextContainer("bump_type", fields) {
		t.Error("bump_type is a value, not a container")
	}
}
