package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/internal/cgp/policy"
)

func TestHandleResourcePolicyCurrent_NoEngine(t *testing.T) {
	s := &Server{}
	res, err := s.handleResourcePolicyCurrent(context.Background(), "relicta://policy/current", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.MimeType != "application/json" {
		t.Errorf("mime: %q", res.MimeType)
	}
	if !strings.Contains(res.Text, "no_policy_engine") {
		t.Errorf("expected no_policy_engine status; got %s", res.Text)
	}
}

func TestHandleResourceRiskBudgetAll_NoBudgets_ReturnsDefaults(t *testing.T) {
	s := &Server{}
	res, err := s.handleResourceRiskBudgetAll(context.Background(), "relicta://risk-budget", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Text, "DefaultRestrictiveAgentBudget") {
		t.Errorf("expected default-budget hint; got %s", res.Text)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(res.Text), &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if payload["status"] != "no_explicit_budgets" {
		t.Errorf("expected no_explicit_budgets; got %v", payload["status"])
	}
}

func TestHandleResourceRiskBudgetAll_WithBudgets(t *testing.T) {
	s := &Server{
		actorBudgets: &policy.ActorBudgetSet{Budgets: []policy.ActorBudget{
			{ActorKind: "agent", ActorID: "*", MaxRiskScore: 0.5},
		}},
	}
	res, _ := s.handleResourceRiskBudgetAll(context.Background(), "relicta://risk-budget", nil)
	var payload map[string]any
	if err := json.Unmarshal([]byte(res.Text), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["status"] != "configured" {
		t.Errorf("expected configured status; got %v", payload["status"])
	}
}

func TestHandleResourceRiskBudgetSingle_FromParams(t *testing.T) {
	s := &Server{}
	res, _ := s.handleResourceRiskBudgetSingle(
		context.Background(),
		"relicta://risk-budget/claude-code-1",
		map[string]string{"actor_id": "claude-code-1"},
	)

	var payload map[string]any
	if err := json.Unmarshal([]byte(res.Text), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["actor_id"] != "claude-code-1" {
		t.Errorf("actor_id: %v", payload["actor_id"])
	}
	if payload["budget"] == nil {
		t.Errorf("expected budget field present")
	}
}

func TestHandleResourceRiskBudgetSingle_FromURIFallback(t *testing.T) {
	s := &Server{}
	res, _ := s.handleResourceRiskBudgetSingle(
		context.Background(),
		"relicta://risk-budget/cursor-1",
		nil, // no params — must derive from URI
	)
	if !strings.Contains(res.Text, "cursor-1") {
		t.Errorf("expected actor_id from URI; got %s", res.Text)
	}
}

func TestHandleResourceRiskBudgetSingle_MissingActor(t *testing.T) {
	s := &Server{}
	res, _ := s.handleResourceRiskBudgetSingle(
		context.Background(),
		"relicta://risk-budget",
		nil,
	)
	if !strings.Contains(res.Text, "missing_actor_id") {
		t.Errorf("expected missing_actor_id status; got %s", res.Text)
	}
}

func TestHandleResourceRecentIncidents(t *testing.T) {
	s := &Server{}
	res, _ := s.handleResourceRecentIncidents(context.Background(), "relicta://recent-incidents", nil)

	var payload map[string]any
	if err := json.Unmarshal([]byte(res.Text), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["status"] != "ok" {
		t.Errorf("status: %v", payload["status"])
	}
	window, _ := payload["window"].(map[string]any)
	if window["since"] == "" || window["until"] == "" {
		t.Errorf("expected since/until in window: %+v", window)
	}
}

func TestHandleResourceComplianceFrameworks(t *testing.T) {
	s := &Server{}
	res, _ := s.handleResourceComplianceFrameworks(context.Background(), "relicta://compliance-frameworks", nil)

	var payload map[string]any
	if err := json.Unmarshal([]byte(res.Text), &payload); err != nil {
		t.Fatal(err)
	}
	frameworks, ok := payload["frameworks"].([]any)
	if !ok {
		t.Fatal("expected frameworks array")
	}

	var names []string
	for _, f := range frameworks {
		m := f.(map[string]any)
		names = append(names, m["id"].(string))
	}

	for _, want := range []string{"soc2", "eu-ai-act-article-12", "eu-ai-act-annex-iv", "iso-27001", "iso-42001", "hipaa", "pci-dss"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing framework %q", want)
		}
	}
}

func TestHandleResourceApprovalCard_NoReleaseRepo(t *testing.T) {
	s := &Server{}
	res, err := s.handleResourceApprovalCard(context.Background(), "relicta://approval", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Text, "no_release_repo") {
		t.Errorf("expected no_release_repo status; got %s", res.Text)
	}
}

func TestSeverityFromTier(t *testing.T) {
	cases := map[string]string{
		"critical": "CRITICAL",
		"high":     "HIGH",
		"medium":   "MEDIUM",
		"low":      "LOW",
		"":         "LOW",
	}
	for in, want := range cases {
		if got := severityFromTier(in); got != want {
			t.Errorf("severityFromTier(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestJSONResource_HandlesMarshal(t *testing.T) {
	res, err := jsonResource("relicta://x", map[string]string{"k": "v"})
	if err != nil {
		t.Fatal(err)
	}
	if res.MimeType != "application/json" {
		t.Errorf("mime: %q", res.MimeType)
	}
	if !strings.Contains(res.Text, `"k": "v"`) {
		t.Errorf("expected payload in text: %s", res.Text)
	}
}
