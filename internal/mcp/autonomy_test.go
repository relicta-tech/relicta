package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/internal/cgp/policy"
)

func TestCheckBudget_NoBudgetSet_FallsBackRestrictive(t *testing.T) {
	s := &Server{}

	// Default restrictive agent budget requires cosign on publish.
	err := s.checkBudget(context.Background(), policy.Operation{Tool: "relicta_publish"})
	if err == nil {
		t.Fatal("expected denial when no budget set + no cosigner present")
	}

	var denial *MCPBudgetDenialError
	if !errors.As(err, &denial) {
		t.Fatalf("expected *MCPBudgetDenialError, got %T", err)
	}
	if !strings.Contains(denial.Error(), "cosigner_required") {
		t.Errorf("expected cosigner violation; got: %s", denial.Error())
	}
}

func TestCheckBudget_WithCosigner_NoCosignBudget_Allowed(t *testing.T) {
	// Custom budget with no cosign requirement → allow without cosigner.
	s := &Server{
		actorBudgets: &policy.ActorBudgetSet{Budgets: []policy.ActorBudget{
			{
				ActorKind:    "agent",
				ActorID:      "*",
				MaxRiskScore: 0.5,
				// No RequiresCosign list.
			},
		}},
	}

	if err := s.checkBudget(context.Background(), policy.Operation{Tool: "relicta_publish"}); err != nil {
		t.Errorf("expected allow, got: %v", err)
	}
}

func TestCheckBudget_DeniedTool(t *testing.T) {
	// Budget configs use logical operation names (no `relicta_` prefix);
	// MCP tool names are normalized before matching.
	s := &Server{
		actorBudgets: &policy.ActorBudgetSet{Budgets: []policy.ActorBudget{
			{
				ActorKind:   "agent",
				ActorID:     "*",
				DeniedTools: []string{"reset"},
			},
		}},
	}

	err := s.checkBudget(context.Background(), policy.Operation{Tool: "relicta_reset"})
	if err == nil {
		t.Fatal("expected denial on denied tool")
	}
	if !strings.Contains(err.Error(), "tool_denied") {
		t.Errorf("expected tool_denied violation, got: %s", err.Error())
	}
}

func TestCheckBudget_AllowedToolWhitelist(t *testing.T) {
	s := &Server{
		actorBudgets: &policy.ActorBudgetSet{Budgets: []policy.ActorBudget{
			{
				ActorKind:    "agent",
				ActorID:      "*",
				AllowedTools: []string{"plan", "evaluate"},
			},
		}},
	}

	// Tool not in whitelist → denied.
	if err := s.checkBudget(context.Background(), policy.Operation{Tool: "relicta_publish"}); err == nil {
		t.Error("expected denial for non-whitelisted tool")
	}

	// Tool in whitelist → allowed (relicta_plan normalizes to plan).
	if err := s.checkBudget(context.Background(), policy.Operation{Tool: "relicta_plan"}); err != nil {
		t.Errorf("expected allow for whitelisted tool; got %v", err)
	}
}

func TestNormalizeToolName(t *testing.T) {
	cases := map[string]string{
		"relicta_publish": "publish",
		"relicta_approve": "approve",
		"publish":         "publish", // already normalized
		"":                "",
		"relicta_":        "relicta_", // exact prefix match — keep as-is
	}
	for in, want := range cases {
		if got := normalizeToolName(in); got != want {
			t.Errorf("normalizeToolName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMCPBudgetDenialError_FormatsAllViolations(t *testing.T) {
	d := policy.Decision{
		Violations: []policy.Violation{
			{Code: "tool_denied", Message: "tool x denied"},
			{Code: "cosigner_required", Message: "publish requires cosigner"},
		},
	}
	err := &MCPBudgetDenialError{Tool: "relicta_publish", Decision: d}

	msg := err.Error()
	if !strings.Contains(msg, "tool_denied") {
		t.Errorf("missing tool_denied in error: %s", msg)
	}
	if !strings.Contains(msg, "cosigner_required") {
		t.Errorf("missing cosigner_required in error: %s", msg)
	}
	if !strings.Contains(msg, "2 violation(s)") {
		t.Errorf("missing violation count: %s", msg)
	}
}

func TestMCPBudgetDenialError_NoViolations(t *testing.T) {
	err := &MCPBudgetDenialError{Tool: "x", Decision: policy.Decision{}}
	if !strings.Contains(err.Error(), `denied tool "x"`) {
		t.Errorf("unexpected error formatting: %s", err.Error())
	}
}

func TestBlastRadiusForRiskScore(t *testing.T) {
	cases := []struct {
		score float64
		want  policy.BlastRadius
	}{
		{0.95, policy.BlastRadiusCritical},
		{0.7, policy.BlastRadiusHigh},
		{0.5, policy.BlastRadiusMedium},
		{0.2, policy.BlastRadiusLow},
		{0.0, policy.BlastRadiusNone},
	}
	for _, c := range cases {
		if got := blastRadiusForRiskScore(c.score); got != c.want {
			t.Errorf("score %.2f: got %q, want %q", c.score, got, c.want)
		}
	}
}

func TestWithActorBudgets_ServerOption(t *testing.T) {
	set := &policy.ActorBudgetSet{Budgets: []policy.ActorBudget{
		{ActorKind: "agent", ActorID: "*"},
	}}

	s := &Server{}
	WithActorBudgets(set)(s)
	if s.actorBudgets != set {
		t.Error("WithActorBudgets did not set the field")
	}
}
