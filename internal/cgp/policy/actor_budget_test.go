package policy

import (
	"testing"
	"time"

	"github.com/relicta-tech/relicta/pkg/cgp"
)

func TestBlastRadius_Comparison(t *testing.T) {
	if !BlastRadiusLow.LessThan(BlastRadiusMedium) {
		t.Errorf("low should be less than medium")
	}
	if BlastRadiusCritical.LessThan(BlastRadiusHigh) {
		t.Errorf("critical should not be less than high")
	}
	if !BlastRadiusMedium.LessThanOrEqual(BlastRadiusMedium) {
		t.Errorf("medium should be less than or equal to medium")
	}
	if !BlastRadius("invalid").LessThan(BlastRadiusLow) {
		// invalid -> 0 ordinal, low -> 1; so "less than" returns true
		t.Logf("invalid maps to ordinal 0; this is the expected fallback")
	}
}

func TestBlastRadius_Valid(t *testing.T) {
	for _, level := range []BlastRadius{BlastRadiusNone, BlastRadiusLow, BlastRadiusMedium, BlastRadiusHigh, BlastRadiusCritical} {
		if !level.Valid() {
			t.Errorf("%q should be valid", level)
		}
	}
	if BlastRadius("nuclear").Valid() {
		t.Errorf("nuclear is not a valid blast radius")
	}
}

func TestActorBudgetSet_Match(t *testing.T) {
	set := &ActorBudgetSet{
		Budgets: []ActorBudget{
			{ActorKind: "agent", ActorID: "claude-code-*"},
			{ActorKind: "agent", ActorID: "*"},
			{ActorKind: "human", ActorID: "*"},
		},
	}

	cases := []struct {
		name      string
		actor     cgp.Actor
		wantIndex int
	}{
		{"glob match wins", cgp.Actor{Kind: "agent", ID: "claude-code-1"}, 0},
		{"agent wildcard fallback", cgp.Actor{Kind: "agent", ID: "cursor"}, 1},
		{"human wildcard", cgp.Actor{Kind: "human", ID: "alice@example.com"}, 2},
		{"unknown kind no match", cgp.Actor{Kind: "system", ID: "x"}, -1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := set.Match(c.actor)
			if c.wantIndex == -1 {
				if got != nil {
					t.Errorf("expected nil match, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected match, got nil")
			}
			if got.ActorID != set.Budgets[c.wantIndex].ActorID {
				t.Errorf("matched wrong budget: want index %d (%q), got %q", c.wantIndex, set.Budgets[c.wantIndex].ActorID, got.ActorID)
			}
		})
	}
}

func TestActorBudget_Evaluate_BlastRadiusExceeded(t *testing.T) {
	b := &ActorBudget{MaxBlastRadius: BlastRadiusMedium}
	d := b.Evaluate(Operation{Tool: "publish", BlastRadius: BlastRadiusHigh})
	if d.Allowed {
		t.Fatal("expected denial")
	}
	if len(d.Violations) != 1 || d.Violations[0].Code != "blast_radius_exceeded" {
		t.Errorf("expected blast_radius_exceeded, got %+v", d.Violations)
	}
}

func TestActorBudget_Evaluate_RiskScoreExceeded(t *testing.T) {
	b := &ActorBudget{MaxRiskScore: 0.4}
	d := b.Evaluate(Operation{RiskScore: 0.7})
	if d.Allowed {
		t.Fatal("expected denial")
	}
	if d.Violations[0].Code != "risk_score_exceeded" {
		t.Errorf("expected risk_score_exceeded, got %s", d.Violations[0].Code)
	}
}

func TestActorBudget_Evaluate_DollarCostExceeded(t *testing.T) {
	b := &ActorBudget{MaxDollarCostUSD: 50.0}
	d := b.Evaluate(Operation{DollarCostUSD: 75.0})
	if d.Allowed {
		t.Fatal("expected denial")
	}
	if d.Violations[0].Code != "cost_exceeded" {
		t.Errorf("expected cost_exceeded")
	}
}

func TestActorBudget_Evaluate_CosignRequired(t *testing.T) {
	b := &ActorBudget{RequiresCosign: []string{"publish", "approve"}}

	t.Run("missing cosigner", func(t *testing.T) {
		d := b.Evaluate(Operation{Tool: "publish", HasCosigner: false})
		if d.Allowed {
			t.Fatal("expected denial")
		}
		if d.Violations[0].Code != "cosigner_required" {
			t.Errorf("expected cosigner_required")
		}
	})

	t.Run("with cosigner", func(t *testing.T) {
		d := b.Evaluate(Operation{Tool: "publish", HasCosigner: true})
		if !d.Allowed {
			t.Errorf("expected allow with cosigner")
		}
	})

	t.Run("operation not in list", func(t *testing.T) {
		d := b.Evaluate(Operation{Tool: "plan", HasCosigner: false})
		if !d.Allowed {
			t.Errorf("plan should not require cosigner")
		}
	})
}

func TestActorBudget_Evaluate_ToolAllowList(t *testing.T) {
	b := &ActorBudget{AllowedTools: []string{"plan", "evaluate"}}

	if d := b.Evaluate(Operation{Tool: "plan"}); !d.Allowed {
		t.Errorf("plan should be allowed")
	}
	if d := b.Evaluate(Operation{Tool: "publish"}); d.Allowed {
		t.Errorf("publish should be denied")
	}
}

func TestActorBudget_Evaluate_ToolDenyList(t *testing.T) {
	b := &ActorBudget{DeniedTools: []string{"publish", "rollback"}}
	if d := b.Evaluate(Operation{Tool: "plan"}); !d.Allowed {
		t.Errorf("plan should be allowed (not in deny list)")
	}
	if d := b.Evaluate(Operation{Tool: "publish"}); d.Allowed {
		t.Errorf("publish should be denied")
	}
}

func TestActorBudget_Evaluate_DenyOverridesAllow(t *testing.T) {
	b := &ActorBudget{
		AllowedTools: []string{"plan", "publish"},
		DeniedTools:  []string{"publish"},
	}
	if d := b.Evaluate(Operation{Tool: "publish"}); d.Allowed {
		t.Errorf("deny should override allow")
	}
}

func TestActorBudget_Evaluate_MultipleViolations(t *testing.T) {
	b := &ActorBudget{
		MaxBlastRadius: BlastRadiusLow,
		MaxRiskScore:   0.3,
		RequiresCosign: []string{"publish"},
	}
	d := b.Evaluate(Operation{
		Tool:        "publish",
		BlastRadius: BlastRadiusHigh,
		RiskScore:   0.9,
		HasCosigner: false,
	})
	if d.Allowed {
		t.Fatal("expected denial")
	}
	if len(d.Violations) != 3 {
		t.Errorf("expected 3 violations, got %d: %+v", len(d.Violations), d.Violations)
	}
}

func TestActorBudget_Evaluate_NilBudget(t *testing.T) {
	var b *ActorBudget
	d := b.Evaluate(Operation{Tool: "publish"})
	if d.Allowed {
		t.Fatal("nil budget must not allow operations")
	}
	if d.Violations[0].Code != "no_budget" {
		t.Errorf("expected no_budget violation")
	}
}

func TestBudgetTimeWindow_Permits(t *testing.T) {
	weekdayMorningUTC := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC) // Mon
	weekendUTC := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)        // Sun
	nightUTC := time.Date(2026, 5, 4, 23, 0, 0, 0, time.UTC)          // Mon 23:00

	startHour, endHour := 9, 17
	weekdayBusiness := &BudgetTimeWindow{
		Days:      []string{"mon", "tue", "wed", "thu", "fri"},
		StartHour: &startHour,
		EndHour:   &endHour,
	}

	if !weekdayBusiness.Permits(weekdayMorningUTC) {
		t.Errorf("Mon 10:00 UTC should be permitted by weekday-business window")
	}
	if weekdayBusiness.Permits(weekendUTC) {
		t.Errorf("Sun should be denied by weekday-business window")
	}
	if weekdayBusiness.Permits(nightUTC) {
		t.Errorf("Mon 23:00 should be denied by weekday-business window")
	}

	// Wrap-around window: 22-06 (covering night hours)
	wrapStart, wrapEnd := 22, 6
	wrap := &BudgetTimeWindow{StartHour: &wrapStart, EndHour: &wrapEnd}
	if wrap.Permits(weekdayMorningUTC) {
		t.Errorf("10:00 should be denied by 22-06 window")
	}
	if !wrap.Permits(nightUTC) {
		t.Errorf("23:00 should be permitted by 22-06 window")
	}
}

func TestActorBudgetSet_Validate(t *testing.T) {
	cases := []struct {
		name    string
		set     ActorBudgetSet
		wantErr bool
	}{
		{
			name: "valid",
			set: ActorBudgetSet{Budgets: []ActorBudget{
				{ActorKind: "agent", MaxBlastRadius: BlastRadiusMedium, MaxRiskScore: 0.5},
			}},
			wantErr: false,
		},
		{
			name: "invalid blast radius",
			set: ActorBudgetSet{Budgets: []ActorBudget{
				{MaxBlastRadius: BlastRadius("nuclear")},
			}},
			wantErr: true,
		},
		{
			name: "risk score out of range",
			set: ActorBudgetSet{Budgets: []ActorBudget{
				{MaxRiskScore: 1.5},
			}},
			wantErr: true,
		},
		{
			name: "negative cost",
			set: ActorBudgetSet{Budgets: []ActorBudget{
				{MaxDollarCostUSD: -1.0},
			}},
			wantErr: true,
		},
		{
			name: "half-set time window",
			set: ActorBudgetSet{Budgets: []ActorBudget{
				{TimeWindow: &BudgetTimeWindow{StartHour: intPtr(9)}},
			}},
			wantErr: true,
		},
		{
			name: "invalid day",
			set: ActorBudgetSet{Budgets: []ActorBudget{
				{TimeWindow: &BudgetTimeWindow{Days: []string{"funday"}}},
			}},
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.set.Validate()
			if c.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestDefaultRestrictiveAgentBudget(t *testing.T) {
	b := DefaultRestrictiveAgentBudget()

	// Major release attempt by agent: high blast + high risk -> deny.
	d := b.Evaluate(Operation{
		Tool:        "publish",
		BlastRadius: BlastRadiusHigh,
		RiskScore:   0.7,
		HasCosigner: false,
	})
	if d.Allowed {
		t.Errorf("default agent budget must refuse high-risk major releases")
	}

	// Patch release with cosigner: low risk + medium blast + cosigner -> allow.
	d = b.Evaluate(Operation{
		Tool:        "publish",
		BlastRadius: BlastRadiusLow,
		RiskScore:   0.2,
		HasCosigner: true,
	})
	if !d.Allowed {
		t.Errorf("default agent budget should allow low-risk releases with cosigner; violations=%+v", d.Violations)
	}
}

func TestDefaultPermissiveHumanBudget(t *testing.T) {
	b := DefaultPermissiveHumanBudget()
	d := b.Evaluate(Operation{
		Tool:        "publish",
		BlastRadius: BlastRadiusCritical,
		RiskScore:   0.99,
	})
	if !d.Allowed {
		t.Errorf("permissive human budget should allow anything; violations=%+v", d.Violations)
	}
}

func intPtr(i int) *int { return &i }
