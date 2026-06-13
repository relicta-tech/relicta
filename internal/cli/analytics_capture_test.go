package cli

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/analytics"
	"github.com/relicta-tech/relicta/v4/internal/application/governance"
	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/config"
)

func newTestAnalytics(t *testing.T) (*analytics.Service, func() []analytics.Event) {
	t.Helper()
	store, err := analytics.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := analytics.NewService(store)
	dump := func() []analytics.Event {
		ev, err := svc.Query(context.Background(), analytics.QueryFilter{})
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		return ev
	}
	return svc, dump
}

func TestCaptureGovernanceAnalytics(t *testing.T) {
	svc, dump := newTestAnalytics(t)
	app := commandTestApp{analyticsSvc: svc}

	gov := &governance.EvaluateReleaseOutput{
		Decision:       cgp.DecisionApproved,
		RiskScore:      0.65,
		Rationale:      []string{"touches auth"},
		CanAutoApprove: true,
	}
	captureGovernanceAnalytics(context.Background(), app, "rel-1", gov)

	events := dump()
	if len(events) != 2 {
		t.Fatalf("expected risk + policy events, got %d", len(events))
	}
	types := map[analytics.EventType]bool{}
	for _, e := range events {
		types[e.Type] = true
	}
	if !types[analytics.EventRiskEvaluation] || !types[analytics.EventPolicyDecision] {
		t.Errorf("missing expected event types: %v", types)
	}
}

func TestCaptureGovernanceAnalytics_NilSafe(t *testing.T) {
	// No analytics service and nil gov result must not panic.
	captureGovernanceAnalytics(context.Background(), commandTestApp{}, "rel", nil)
	svc, dump := newTestAnalytics(t)
	captureGovernanceAnalytics(context.Background(), commandTestApp{analyticsSvc: svc}, "rel", nil)
	if len(dump()) != 0 {
		t.Error("nil gov result must capture nothing")
	}
}

func TestCaptureApprovalOutcome(t *testing.T) {
	origCfg := cfg
	t.Cleanup(func() { cfg = origCfg })
	cfg = config.DefaultConfig()

	svc, dump := newTestAnalytics(t)
	app := commandTestApp{analyticsSvc: svc}

	captureApprovalOutcome(context.Background(), app, "rel-2", "approved", 1500)

	events := dump()
	if len(events) != 1 || events[0].Type != analytics.EventApprovalOutcome {
		t.Fatalf("expected one approval_outcome event, got %+v", events)
	}
}

func TestCaptureReleaseDuration(t *testing.T) {
	svc, dump := newTestAnalytics(t)
	app := commandTestApp{analyticsSvc: svc}

	captureReleaseDuration(context.Background(), app, "rel-3", "1.2.0", 4200, true)

	events := dump()
	if len(events) != 1 || events[0].Type != analytics.EventReleaseDuration {
		t.Fatalf("expected one release_duration event, got %+v", events)
	}
}

func TestRiskLevelForScore(t *testing.T) {
	cases := map[float64]string{0.9: "critical", 0.7: "high", 0.5: "medium", 0.1: "low", 0: "none"}
	for score, want := range cases {
		if got := riskLevelForScore(score); got != want {
			t.Errorf("riskLevelForScore(%v) = %q, want %q", score, got, want)
		}
	}
}
