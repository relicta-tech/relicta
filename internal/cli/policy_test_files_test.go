package cli

import (
	"context"
	"testing"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
)

// `relicta policy test` carried a changed-file *count* and not the paths, so no
// path-conditioned rule could be exercised: neither `scope.files contains "terraform/"`,
// which the shipped policies use, nor the breadth fields derived from the same paths. The
// rules a team is most likely to get wrong were the ones they could not test.
//
// This test exists because I got the wiring wrong in a way a flag test would have caught and
// a unit test of the operator did not: I declared --files, merged it in the --input path, and
// forgot the struct literal that builds the input from flags alone. The flag was accepted,
// reported in --help, and reached nothing. Running the command was what exposed it, so the
// assertion here is on the evaluation, not on the flag being registered.

func policyWithFileRule(field, operator string, value any) []policy.Policy {
	return []policy.Policy{{
		Name:     "paths",
		Defaults: permissiveDefaults(),
		Rules: []policy.Rule{{
			ID: "path-rule", Name: "path rule", Enabled: true, Priority: 1,
			Description: "matched on the changed paths",
			Conditions:  []policy.Condition{{Field: field, Operator: operator, Value: value}},
			Actions: []policy.Action{{
				Type:   policy.ActionAddRationale,
				Params: map[string]any{"message": "matched"},
			}},
		}},
	}}
}

// permissiveDefaults keeps the default outcome out of the way, so only the rule under test
// changes it.
func permissiveDefaults() policy.Defaults {
	return policy.Defaults{Decision: policy.DecisionApprove}
}

func TestChangedPathsReachThePolicyEvaluator(t *testing.T) {
	input := &policyTestInputData{
		RiskScore:  0.1,
		BumpType:   "minor",
		ActorType:  "human",
		ActorID:    "human:alice",
		TrustLevel: "limited",
		Repository: "acme/widget",
		Files:      []string{"internal/a.go", "web/b.ts", "terraform/c.tf"},
	}

	out, err := evaluatePolicyScenario(context.Background(),
		policyWithFileRule("scope.files", policy.OperatorContains, "terraform/c.tf"), input)
	if err != nil {
		t.Fatalf("evaluatePolicyScenario: %v", err)
	}
	if len(out.MatchedRules) != 1 {
		t.Errorf("a rule on scope.files matched %v, want the path rule: the changed paths are "+
			"not reaching the evaluator, so no path-conditioned rule can be tested",
			out.MatchedRules)
	}
}

// The discriminating case for the breadth fields: the same number of files, spread or
// concentrated, must produce different outcomes. If they do not, the field is measuring
// what fileCount already measured.
func TestBreadthRulesSeeTheDifferenceFileCountCannot(t *testing.T) {
	policies := policyWithFileRule("scope.areaCount", policy.OperatorGreaterThan, 2)

	spread := &policyTestInputData{
		RiskScore: 0.1, BumpType: "minor", ActorType: "human", ActorID: "human:alice",
		TrustLevel: "limited", Repository: "acme/widget",
		Files: []string{"internal/a.go", "web/b.ts", "terraform/c.tf"},
	}
	concentrated := &policyTestInputData{
		RiskScore: 0.1, BumpType: "minor", ActorType: "human", ActorID: "human:alice",
		TrustLevel: "limited", Repository: "acme/widget",
		Files: []string{"internal/a.go", "internal/b.go", "internal/c.go"},
	}

	spreadOut, err := evaluatePolicyScenario(context.Background(), policies, spread)
	if err != nil {
		t.Fatalf("evaluatePolicyScenario(spread): %v", err)
	}
	if len(spreadOut.MatchedRules) != 1 {
		t.Errorf("a change across three areas did not match a breadth rule (matched %v)",
			spreadOut.MatchedRules)
	}

	concentratedOut, err := evaluatePolicyScenario(context.Background(), policies, concentrated)
	if err != nil {
		t.Fatalf("evaluatePolicyScenario(concentrated): %v", err)
	}
	if len(concentratedOut.MatchedRules) != 0 {
		t.Errorf("three files inside one area matched a breadth rule (matched %v); the field "+
			"is behaving like fileCount, which is what it exists to avoid",
			concentratedOut.MatchedRules)
	}
}

// The flag has to reach the input, which is a separate question from the input reaching the
// evaluator — and it is the one I got wrong: --files was declared, documented in --help,
// merged in the --input path, and omitted from the struct literal that builds the input from
// flags alone. Every rule depending on it silently did not match.
func TestTheFilesFlagReachesTheInput(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().StringSliceVar(&policyTestFiles, "files", nil, "")
	cmd.Flags().IntVar(&policyTestFilesChanged, "files-changed", 0, "")
	if err := cmd.Flags().Set("files", "internal/a.go,web/b.ts"); err != nil {
		t.Fatalf("set --files: %v", err)
	}
	t.Cleanup(func() { policyTestFiles, policyTestFilesChanged = nil, 0 })

	input, err := resolvePolicyTestInput(cmd)
	if err != nil {
		t.Fatalf("resolvePolicyTestInput: %v", err)
	}

	if len(input.Files) != 2 {
		t.Fatalf("input.Files = %v, want the two paths from --files", input.Files)
	}

	// And the count follows from the paths, so a rule on scope.fileCount and one on
	// scope.files describe the same change rather than disagreeing about its size.
	if input.FilesChanged != 2 {
		t.Errorf("input.FilesChanged = %d, want 2 derived from the paths", input.FilesChanged)
	}
}
