package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPolicyScaffold_GeneratesFixtures(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "scaffold.policy")
	inputOut := filepath.Join(tmpDir, "policy-input.json")
	matrixOut := filepath.Join(tmpDir, "policy-matrix.yaml")

	content := `
rule "high-risk-major" {
    priority = 100
    when {
        risk_score >= 0.8
    }
    then {
        require_approval(role: "senior")
    }
}

rule "low-risk-patch" {
    priority = 10
    when {
        risk_score < 0.3
    }
    then {
        approve()
    }
}
`
	if err := os.WriteFile(policyPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write policy file: %v", err)
	}

	restore := withPolicyScaffoldFlags(t, policyScaffoldState{
		file:             policyPath,
		inputOut:         inputOut,
		matrixOut:        matrixOut,
		force:            false,
		maxRuleScenarios: 8,
	})
	defer restore()

	if err := runPolicyScaffold(policyScaffoldCmd, nil); err != nil {
		t.Fatalf("runPolicyScaffold failed: %v", err)
	}

	inputData, err := os.ReadFile(inputOut)
	if err != nil {
		t.Fatalf("read input fixture: %v", err)
	}
	if !strings.Contains(string(inputData), "\"risk_score\"") {
		t.Fatalf("expected input fixture JSON to contain risk_score field, got: %s", string(inputData))
	}

	matrixData, err := os.ReadFile(matrixOut)
	if err != nil {
		t.Fatalf("read matrix fixture: %v", err)
	}
	var matrix []policyTestMatrixCase
	if err := yaml.Unmarshal(matrixData, &matrix); err != nil {
		t.Fatalf("unmarshal matrix fixture: %v", err)
	}
	if len(matrix) < 3 {
		t.Fatalf("expected at least 3 scenarios, got %d", len(matrix))
	}

	var hasLow bool
	var hasHigh bool
	var hasRuleScenario bool
	for _, c := range matrix {
		if c.Name == "low-risk-seed" {
			hasLow = true
		}
		if c.Name == "high-risk-seed" {
			hasHigh = true
			if c.RiskScore < 0.8 {
				t.Fatalf("expected high-risk-seed risk_score >= 0.8, got %f", c.RiskScore)
			}
		}
		if strings.HasPrefix(c.Name, "rule-") {
			hasRuleScenario = true
		}
		if c.Expect.Decision == nil || c.Expect.Blocked == nil {
			t.Fatalf("expected seeded scenario %s to include expect.decision and expect.blocked", c.Name)
		}
	}
	if !hasLow || !hasHigh {
		t.Fatalf("expected low-risk-seed and high-risk-seed scenarios, got: %+v", matrix)
	}
	if !hasRuleScenario {
		t.Fatal("expected at least one per-rule scenario")
	}
}

func TestPolicyScaffold_NoOverwriteWithoutForce(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "scaffold.policy")
	inputOut := filepath.Join(tmpDir, "policy-input.json")
	matrixOut := filepath.Join(tmpDir, "policy-matrix.yaml")

	content := `
rule "base" {
    when { risk_score > 0.5 }
    then { require_approval(role: "reviewer") }
}
`
	if err := os.WriteFile(policyPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write policy file: %v", err)
	}
	if err := os.WriteFile(inputOut, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("seed input fixture: %v", err)
	}
	if err := os.WriteFile(matrixOut, []byte("[]\n"), 0o644); err != nil {
		t.Fatalf("seed matrix fixture: %v", err)
	}

	restore := withPolicyScaffoldFlags(t, policyScaffoldState{
		file:             policyPath,
		inputOut:         inputOut,
		matrixOut:        matrixOut,
		force:            false,
		maxRuleScenarios: 4,
	})
	defer restore()

	err := runPolicyScaffold(policyScaffoldCmd, nil)
	if err == nil {
		t.Fatal("expected runPolicyScaffold to fail when output files already exist")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected overwrite error, got: %v", err)
	}
}

func TestPolicyScaffold_MaxRuleScenariosZero(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "scaffold.policy")
	matrixOut := filepath.Join(tmpDir, "policy-matrix.yaml")

	content := `
rule "one" {
    when { risk_score > 0.4 }
    then { require_approval(role: "reviewer") }
}
rule "two" {
    when { actor_type == "agent" }
    then { require_approval(role: "reviewer") }
}
`
	if err := os.WriteFile(policyPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write policy file: %v", err)
	}

	restore := withPolicyScaffoldFlags(t, policyScaffoldState{
		file:             policyPath,
		inputOut:         filepath.Join(tmpDir, "policy-input.json"),
		matrixOut:        matrixOut,
		force:            false,
		maxRuleScenarios: 0,
	})
	defer restore()

	if err := runPolicyScaffold(policyScaffoldCmd, nil); err != nil {
		t.Fatalf("runPolicyScaffold failed: %v", err)
	}

	matrixData, err := os.ReadFile(matrixOut)
	if err != nil {
		t.Fatalf("read matrix fixture: %v", err)
	}
	var matrix []policyTestMatrixCase
	if err := yaml.Unmarshal(matrixData, &matrix); err != nil {
		t.Fatalf("unmarshal matrix fixture: %v", err)
	}
	if len(matrix) != 2 {
		t.Fatalf("expected only low/high seed scenarios with --max-rule-scenarios=0, got %d", len(matrix))
	}
}

type policyScaffoldState struct {
	dir              string
	file             string
	inputOut         string
	matrixOut        string
	force            bool
	maxRuleScenarios int
}

func withPolicyScaffoldFlags(t *testing.T, state policyScaffoldState) func() {
	t.Helper()
	old := policyScaffoldState{
		dir:              policyScaffoldDir,
		file:             policyScaffoldFile,
		inputOut:         policyScaffoldInputOut,
		matrixOut:        policyScaffoldMatrixOut,
		force:            policyScaffoldForce,
		maxRuleScenarios: policyScaffoldMaxRuleScenarios,
	}

	policyScaffoldDir = state.dir
	policyScaffoldFile = state.file
	policyScaffoldInputOut = state.inputOut
	policyScaffoldMatrixOut = state.matrixOut
	policyScaffoldForce = state.force
	policyScaffoldMaxRuleScenarios = state.maxRuleScenarios

	return func() {
		policyScaffoldDir = old.dir
		policyScaffoldFile = old.file
		policyScaffoldInputOut = old.inputOut
		policyScaffoldMatrixOut = old.matrixOut
		policyScaffoldForce = old.force
		policyScaffoldMaxRuleScenarios = old.maxRuleScenarios
	}
}
