package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadActorBudgets_FileMissing(t *testing.T) {
	set, err := LoadActorBudgets(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("missing file should return empty set, got %v", err)
	}
	if len(set.Budgets) != 0 {
		t.Errorf("expected empty set, got %d budgets", len(set.Budgets))
	}
}

func TestLoadActorBudgets_HappyPath(t *testing.T) {
	yaml := `
budgets:
  - actor_kind: agent
    actor_id: "claude-code-*"
    max_blast_radius: medium
    max_risk_score: 0.4
    requires_cosign: [publish, approve]
    allowed_tools: [plan, evaluate, summarize_diff]
  - actor_kind: human
    actor_id: "*"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "actor-budgets.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	set, err := LoadActorBudgets(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(set.Budgets) != 2 {
		t.Fatalf("expected 2 budgets, got %d", len(set.Budgets))
	}
	if set.Budgets[0].MaxBlastRadius != BlastRadiusMedium {
		t.Errorf("expected medium, got %q", set.Budgets[0].MaxBlastRadius)
	}
	if set.Budgets[0].MaxRiskScore != 0.4 {
		t.Errorf("expected 0.4, got %v", set.Budgets[0].MaxRiskScore)
	}
}

func TestLoadActorBudgets_InvalidYAML(t *testing.T) {
	yaml := `budgets:
  - actor_kind: agent
    max_risk_score: 5.0
`
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadActorBudgets(path); err == nil {
		t.Error("expected validation error for max_risk_score > 1")
	}
}

func TestLoadActorBudgets_UnknownField(t *testing.T) {
	yaml := `budgets:
  - actor_kind: agent
    nonexistent_field: 42
`
	dir := t.TempDir()
	path := filepath.Join(dir, "unknown.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadActorBudgets(path)
	if err == nil {
		t.Error("expected error for unknown field with KnownFields(true)")
	}
	if err != nil && !strings.Contains(err.Error(), "nonexistent_field") {
		t.Errorf("expected error to mention unknown field, got %v", err)
	}
}

func TestResolveBudget_MatchWins(t *testing.T) {
	set := &ActorBudgetSet{Budgets: []ActorBudget{
		{ActorKind: "agent", ActorID: "claude-code-*", MaxRiskScore: 0.5},
	}}
	got := ResolveBudget(set, "agent", "claude-code-1")
	if got == nil {
		t.Fatal("expected match")
	}
	if got.MaxRiskScore != 0.5 {
		t.Errorf("expected 0.5, got %v", got.MaxRiskScore)
	}
}

func TestResolveBudget_AgentDefaultRestrictive(t *testing.T) {
	got := ResolveBudget(nil, "agent", "any-id")
	if got == nil {
		t.Fatal("agent should fall back to default budget")
	}
	if got.MaxRiskScore != 0.4 {
		t.Errorf("agent default should cap risk at 0.4")
	}
	if len(got.RequiresCosign) == 0 {
		t.Errorf("agent default should require cosigner for privileged ops")
	}
}

func TestResolveBudget_HumanDefaultPermissive(t *testing.T) {
	got := ResolveBudget(nil, "human", "alice@example.com")
	if got == nil {
		t.Fatal("human should fall back to permissive default")
	}
	d := got.Evaluate(Operation{
		Tool:        "publish",
		BlastRadius: BlastRadiusCritical,
		RiskScore:   0.99,
	})
	if !d.Allowed {
		t.Errorf("permissive human default should not block; got %+v", d.Violations)
	}
}

func TestResolveBudget_CIFallsBackRestrictive(t *testing.T) {
	got := ResolveBudget(nil, "ci", "github-actions")
	if got == nil {
		t.Fatal("ci should fall back to restrictive default")
	}
	if got.MaxRiskScore != 0.4 {
		t.Errorf("ci should be treated like agent for safety")
	}
}
