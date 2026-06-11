package cgp

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRiskTierForScore(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{0.0, "low"},
		{0.39, "low"},
		{0.4, "medium"},
		{0.69, "medium"},
		{0.7, "high"},
		{0.84, "high"},
		{0.85, "critical"},
		{1.0, "critical"},
	}
	for _, c := range cases {
		if got := RiskTierForScore(c.score); got != c.want {
			t.Errorf("score %.2f: got %q, want %q", c.score, got, c.want)
		}
	}
}

func TestRiskGlyphForTier_DistinctPerTier(t *testing.T) {
	tiers := []string{"low", "medium", "high", "critical"}
	seen := make(map[string]string)
	for _, tier := range tiers {
		g := RiskGlyphForTier(tier)
		if existing, dup := seen[g]; dup {
			t.Errorf("glyph %q used by both %q and %q", g, existing, tier)
		}
		seen[g] = tier
	}
}

func TestCanonicalActions_StableOrder(t *testing.T) {
	a := CanonicalActions()
	b := CanonicalActions()
	if len(a) != len(b) {
		t.Fatalf("non-deterministic length")
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Errorf("order drift at %d: %s vs %s", i, a[i].ID, b[i].ID)
		}
	}

	wantOrder := []string{"approve", "reject", "edit_notes", "request_changes"}
	for i, want := range wantOrder {
		if i >= len(a) || a[i].ID != want {
			t.Errorf("expected action %d to be %q; got %q", i, want, a[i].ID)
		}
	}
}

func TestApprovalCard_RoundTripJSON(t *testing.T) {
	card := ApprovalCard{
		CGPVersion: "0.1",
		CardID:     "card-001",
		ReleaseID:  "rel-001",
		Version:    "1.4.1",
		Repository: "acme/payments",
		Risk: RiskBlock{
			Score:    0.65,
			Tier:     RiskTierForScore(0.65),
			Severity: "MEDIUM",
			Glyph:    RiskGlyphForTier("medium"),
			Factors: []RiskFactor{
				{Category: "auth", Description: "touches authentication", Score: 0.6, Severity: "high"},
			},
		},
		Actor:            Actor{Kind: "agent", ID: "claude-code-1"},
		Decision:         "approval_required",
		Rationale:        []string{"breaking change requires manual review"},
		AvailableActions: CanonicalActions(),
		Frameworks:       []string{"SOC2", "EU-AI-Act"},
		CreatedAt:        time.Now().UTC(),
	}

	b, err := json.Marshal(&card)
	if err != nil {
		t.Fatal(err)
	}

	var got ApprovalCard
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}

	if got.CardID != card.CardID {
		t.Errorf("cardId drift: got %q, want %q", got.CardID, card.CardID)
	}
	if got.Risk.Tier != "medium" {
		t.Errorf("tier: got %q", got.Risk.Tier)
	}
	if len(got.AvailableActions) != len(card.AvailableActions) {
		t.Errorf("actions length drift: %d vs %d", len(got.AvailableActions), len(card.AvailableActions))
	}
}

func TestApprovalCard_OmitemptyFieldsWhenAbsent(t *testing.T) {
	minimal := ApprovalCard{
		CGPVersion:       "0.1",
		CardID:           "x",
		ReleaseID:        "y",
		Risk:             RiskBlock{Score: 0.1, Tier: "low", Severity: "LOW"},
		Actor:            Actor{Kind: "human", ID: "alice@example.com"},
		Decision:         "approved",
		AvailableActions: CanonicalActions(),
		CreatedAt:        time.Now().UTC(),
	}
	b, _ := json.Marshal(&minimal)
	body := string(b)

	for _, mustOmit := range []string{"diffSummary", "verifiers", "rationale", "requiredActions", "frameworks", "auditChainHash"} {
		if contains(body, `"`+mustOmit+`":`) {
			t.Errorf("expected %q to be omitted when empty", mustOmit)
		}
	}
}

func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
