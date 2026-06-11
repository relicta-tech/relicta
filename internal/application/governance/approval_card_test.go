package governance

import (
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	pkgcgp "github.com/relicta-tech/relicta/v4/pkg/cgp"
)

func TestBuildApprovalCard_NilResultEmitsScaffold(t *testing.T) {
	card := BuildApprovalCard(ApprovalCardInput{
		ReleaseID: "rel-1",
		Version:   "1.0.0",
		Actor:     pkgcgp.Actor{Kind: "human", ID: "alice@example.com"},
	})
	if card.CGPVersion != pkgcgp.ProtocolVersion {
		t.Errorf("CGPVersion: got %q", card.CGPVersion)
	}
	if card.CardID != "card:rel-1" {
		t.Errorf("default CardID: got %q", card.CardID)
	}
	if len(card.AvailableActions) == 0 {
		t.Error("AvailableActions should default to canonical set even when result is nil")
	}
}

func TestBuildApprovalCard_PopulatesRiskBlock(t *testing.T) {
	in := ApprovalCardInput{
		ReleaseID:  "rel-2",
		Version:    "1.4.0",
		Repository: "acme/payments",
		Actor:      pkgcgp.Actor{Kind: "agent", ID: "claude-code-1"},
		Result: &EvaluateReleaseOutput{
			Decision:  cgp.DecisionApprovalRequired,
			RiskScore: 0.65,
			Severity:  cgp.Severity("medium"),
			Rationale: []string{"breaking change requires manual review"},
			RiskFactors: []cgp.RiskFactor{
				{Category: "auth", Description: "touches authentication", Score: 0.8, Severity: cgp.Severity("high")},
				{Category: "scope", Description: "diff small", Score: 0.2, Severity: cgp.Severity("low")},
			},
			RequiredActions: []cgp.RequiredAction{
				{Type: "human_review", Description: "needs human approver"},
			},
		},
	}
	card := BuildApprovalCard(in)

	if card.Risk.Score != 0.65 {
		t.Errorf("score: %v", card.Risk.Score)
	}
	if card.Risk.Tier != "medium" {
		t.Errorf("tier: %q", card.Risk.Tier)
	}
	if card.Risk.Glyph != "▲▲" {
		t.Errorf("glyph: %q", card.Risk.Glyph)
	}
	if len(card.Risk.Factors) != 2 {
		t.Fatalf("factors: %d", len(card.Risk.Factors))
	}
	// Factors must be sorted by score descending.
	if card.Risk.Factors[0].Category != "auth" {
		t.Errorf("highest-score factor first; got %q", card.Risk.Factors[0].Category)
	}
	if len(card.RequiredActions) != 1 {
		t.Errorf("required actions: %d", len(card.RequiredActions))
	}
	if card.Decision != string(cgp.DecisionApprovalRequired) {
		t.Errorf("decision: %q", card.Decision)
	}
	if len(card.Rationale) != 1 {
		t.Errorf("rationale dropped")
	}
}

func TestBuildApprovalCard_TierFromScore(t *testing.T) {
	cases := []struct {
		score float64
		tier  string
		glyph string
	}{
		{0.1, "low", "▲"},
		{0.5, "medium", "▲▲"},
		{0.75, "high", "▲▲▲"},
		{0.9, "critical", "▲▲▲▲"},
	}
	for _, c := range cases {
		card := BuildApprovalCard(ApprovalCardInput{
			ReleaseID: "rel",
			Result:    &EvaluateReleaseOutput{RiskScore: c.score},
		})
		if card.Risk.Tier != c.tier {
			t.Errorf("score %.2f: tier got %q, want %q", c.score, card.Risk.Tier, c.tier)
		}
		if card.Risk.Glyph != c.glyph {
			t.Errorf("score %.2f: glyph got %q, want %q", c.score, card.Risk.Glyph, c.glyph)
		}
	}
}

func TestBuildApprovalCard_SeverityFallsBackToTier(t *testing.T) {
	card := BuildApprovalCard(ApprovalCardInput{
		ReleaseID: "rel",
		Result:    &EvaluateReleaseOutput{RiskScore: 0.75, Severity: cgp.Severity("")},
	})
	if card.Risk.Severity != "high" {
		t.Errorf("expected severity to fall back to tier 'high'; got %q", card.Risk.Severity)
	}
}

func TestBuildApprovalCard_DefaultCardID(t *testing.T) {
	c1 := BuildApprovalCard(ApprovalCardInput{ReleaseID: "rel-x"})
	if c1.CardID != "card:rel-x" {
		t.Errorf("got %q", c1.CardID)
	}
	c2 := BuildApprovalCard(ApprovalCardInput{})
	if c2.CardID != "card:unknown" {
		t.Errorf("expected fallback when releaseID is empty; got %q", c2.CardID)
	}
	c3 := BuildApprovalCard(ApprovalCardInput{CardID: "explicit", ReleaseID: "rel-x"})
	if c3.CardID != "explicit" {
		t.Errorf("explicit ID overrides default; got %q", c3.CardID)
	}
}

func TestConvertRiskFactors_Empty(t *testing.T) {
	out := convertRiskFactors(nil)
	if len(out) != 0 {
		t.Errorf("empty input should produce empty output; got %v", out)
	}
}
