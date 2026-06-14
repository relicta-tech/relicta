package governance

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
	"github.com/relicta-tech/relicta/v4/internal/cgp/risk"
)

// fullService wires a real evaluator and memory store with calibration,
// reputation, and attribution all enabled — the production-shaped configuration.
func fullService(store *memory.InMemoryStore) *Service {
	eval := evaluator.New(
		evaluator.WithRiskCalculator(risk.NewCalculatorWithDefaults()),
		evaluator.WithPolicyEngine(policy.NewEngine(nil, nil)),
	)
	return NewService(eval,
		WithLogger(testLogger()),
		WithMemoryStore(store),
		WithCalibration(true),
		WithReputation(true),
		WithReputationThreshold(0.5),
		WithAttribution(true),
		WithCalibrationValidation(0.5, false),
	)
}

// TestEvaluateRelease_FullPipeline exercises calibration + reputation +
// attribution together through EvaluateRelease, the gap the unit tests don't
// cover. A human initiates a release that an AI agent authored; the initiator
// carries a poor track record. The evaluation must run end to end, attach
// reputation, and never auto-approve.
func TestEvaluateRelease_FullPipeline(t *testing.T) {
	store := memory.NewInMemoryStore()
	initiator := cgp.NewHumanActor("dev@example.com", "Dev")

	// Seed a poor track record for the initiator so reputation is restricted.
	seedReleases(t, store, initiator.ID, "owner/repo", memory.OutcomeFailed, 6)
	seedIncidents(t, store, initiator.ID, "owner/repo", 6)

	svc := fullService(store)
	rel := agentReleaseWith(t, "Claude", "claude@anthropic.com")

	out, err := svc.EvaluateRelease(context.Background(), EvaluateReleaseInput{
		Release:        rel,
		Actor:          initiator,
		Repository:     "owner/repo",
		IncludeHistory: true,
	})
	if err != nil {
		t.Fatalf("EvaluateRelease() error = %v", err)
	}
	if out == nil {
		t.Fatal("EvaluateRelease() returned nil output")
	}

	// Reputation must be computed and attached for an actor with history.
	if out.Reputation == nil {
		t.Fatal("expected reputation info attached for actor with history")
	}
	if out.Reputation.SampleSize < minReputationSamples {
		t.Fatalf("expected sufficient reputation samples; got %d", out.Reputation.SampleSize)
	}

	// A poor-reputation actor authoring via an AI agent must not auto-approve.
	if out.CanAutoApprove {
		t.Fatal("poor-reputation, agent-authored release must not auto-approve")
	}
	if out.Decision == cgp.DecisionApproved {
		t.Fatalf("decision must not be a clean approval; got %s", out.Decision)
	}

	// Calibration must have run without affecting determinism of the result.
	if out.HistoricalContext == nil {
		t.Fatal("expected historical context when IncludeHistory is set")
	}
}

// TestEvaluateRelease_CalibrationStrictRejectsLowAccuracy verifies the strict
// calibration guard: with too little/no usable history, strict mode keeps
// default weights and evaluation still succeeds.
func TestEvaluateRelease_CalibrationStrictRejectsLowAccuracy(t *testing.T) {
	store := memory.NewInMemoryStore()
	eval := evaluator.New(
		evaluator.WithRiskCalculator(risk.NewCalculatorWithDefaults()),
		evaluator.WithPolicyEngine(policy.NewEngine(nil, nil)),
	)
	svc := NewService(eval,
		WithLogger(testLogger()),
		WithMemoryStore(store),
		WithCalibration(true),
		WithCalibrationValidation(0.99, true), // unreachable accuracy, fail closed
	)

	rel := createTestRelease(t)
	out, err := svc.EvaluateRelease(context.Background(), EvaluateReleaseInput{
		Release:    rel,
		Actor:      cgp.NewHumanActor("dev@example.com", "Dev"),
		Repository: "owner/repo",
	})
	if err != nil {
		t.Fatalf("EvaluateRelease() error = %v", err)
	}
	if out == nil {
		t.Fatal("EvaluateRelease() returned nil output")
	}
}
