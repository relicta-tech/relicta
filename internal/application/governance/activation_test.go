package governance

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
	"github.com/relicta-tech/relicta/v4/internal/cgp/risk"
)

const guardActorID = "human:bad@example.com"

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testEvaluator() *evaluator.Evaluator {
	return evaluator.New(
		evaluator.WithRiskCalculator(risk.NewCalculatorWithDefaults()),
		evaluator.WithPolicyEngine(policy.NewEngine(nil, nil)),
	)
}

// seedIncidents records unresolved incidents for an actor, tanking the
// incident-rate and recovery-speed reputation components.
func seedIncidents(t *testing.T, store *memory.InMemoryStore, actorID, repo string, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		inc := &memory.IncidentRecord{
			ID:         repo + "-inc-" + itoaTest(i),
			ReleaseID:  repo + "-rel-" + itoaTest(i),
			Repository: repo,
			ActorID:    actorID,
			Severity:   cgp.SeverityHigh,
			DetectedAt: time.Now().Add(-time.Duration(i) * time.Hour),
		}
		if err := store.RecordIncident(ctx, inc); err != nil {
			t.Fatalf("seed incident: %v", err)
		}
	}
}

func seedReleases(t *testing.T, store *memory.InMemoryStore, actorID, repo string, outcome memory.ReleaseOutcome, n int) {
	t.Helper()
	ctx := context.Background()
	actor := cgp.Actor{ID: actorID, Kind: cgp.ActorKindHuman, Name: actorID}
	for i := 0; i < n; i++ {
		rec := &memory.ReleaseRecord{
			ID:         repo + "-rel-" + itoaTest(i),
			Repository: repo,
			Version:    "v1.0." + itoaTest(i),
			Actor:      actor,
			RiskScore:  0.3,
			Decision:   cgp.DecisionApproved,
			Outcome:    outcome,
			ReleasedAt: time.Now().Add(-time.Duration(i) * time.Hour),
			Duration:   time.Minute,
		}
		if err := store.RecordRelease(ctx, rec); err != nil {
			t.Fatalf("seed release: %v", err)
		}
	}
}

func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func newGuardService(store *memory.InMemoryStore) *Service {
	return &Service{
		logger:            testLogger(),
		memoryStore:       store,
		reputationEnabled: true,
	}
}

func TestReputationGuard_DowngradesPoorActor(t *testing.T) {
	store := memory.NewInMemoryStore()
	seedReleases(t, store, guardActorID, "owner/repo", memory.OutcomeFailed, 6)
	seedIncidents(t, store, guardActorID, "owner/repo", 6)

	svc := newGuardService(store)
	out := &EvaluateReleaseOutput{Decision: cgp.DecisionApproved, CanAutoApprove: true}
	input := EvaluateReleaseInput{Repository: "owner/repo", Actor: cgp.Actor{ID: guardActorID}}

	svc.applyReputationGuard(context.Background(), input, out)

	if out.Reputation == nil {
		t.Fatal("expected reputation info attached")
	}
	if out.Decision != cgp.DecisionApprovalRequired {
		t.Fatalf("poor actor must be downgraded, got %s", out.Decision)
	}
	if out.CanAutoApprove {
		t.Fatal("poor actor must not auto-approve")
	}
	if !out.Reputation.Guarded {
		t.Fatal("expected Guarded=true")
	}
	if out.Reputation.Overall >= 0.4 {
		t.Fatalf("expected restricted reputation (<0.4), got %.3f", out.Reputation.Overall)
	}
}

func TestReputationGuard_IgnoresInsufficientSamples(t *testing.T) {
	store := memory.NewInMemoryStore()
	// Only 2 records — below minReputationSamples; must NOT penalize.
	seedReleases(t, store, guardActorID, "owner/repo", memory.OutcomeFailed, 2)

	svc := newGuardService(store)
	out := &EvaluateReleaseOutput{Decision: cgp.DecisionApproved, CanAutoApprove: true}
	input := EvaluateReleaseInput{Repository: "owner/repo", Actor: cgp.Actor{ID: guardActorID}}

	svc.applyReputationGuard(context.Background(), input, out)

	if out.Decision != cgp.DecisionApproved {
		t.Fatalf("actor with too few samples must not be downgraded, got %s", out.Decision)
	}
	if out.Reputation != nil && out.Reputation.Guarded {
		t.Fatal("must not guard on insufficient samples")
	}
}

func TestReputationGuard_KeepsGoodActorApproved(t *testing.T) {
	store := memory.NewInMemoryStore()
	seedReleases(t, store, guardActorID, "owner/repo", memory.OutcomeSuccess, 6)

	svc := newGuardService(store)
	out := &EvaluateReleaseOutput{Decision: cgp.DecisionApproved, CanAutoApprove: true}
	input := EvaluateReleaseInput{Repository: "owner/repo", Actor: cgp.Actor{ID: guardActorID}}

	svc.applyReputationGuard(context.Background(), input, out)

	if out.Decision != cgp.DecisionApproved {
		t.Fatalf("good actor must stay approved, got %s", out.Decision)
	}
	if out.Reputation == nil {
		t.Fatal("expected reputation info attached for actor with history")
	}
	if out.Reputation.Guarded {
		t.Fatal("good actor must not be guarded")
	}
}

func TestReputationGuard_NeverLoosens(t *testing.T) {
	store := memory.NewInMemoryStore()
	seedReleases(t, store, guardActorID, "owner/repo", memory.OutcomeFailed, 6)

	svc := newGuardService(store)
	// Decision already requires approval — guard must not flip it to approved.
	out := &EvaluateReleaseOutput{Decision: cgp.DecisionApprovalRequired, CanAutoApprove: false}
	input := EvaluateReleaseInput{Repository: "owner/repo", Actor: cgp.Actor{ID: guardActorID}}

	svc.applyReputationGuard(context.Background(), input, out)

	if out.Decision != cgp.DecisionApprovalRequired {
		t.Fatalf("guard must never loosen; got %s", out.Decision)
	}
}

func TestEnsureCalibrated_RunsOnceAndIsSafe(t *testing.T) {
	store := memory.NewInMemoryStore()
	seedReleases(t, store, guardActorID, "owner/repo", memory.OutcomeFailed, 4)

	svc := &Service{logger: testLogger(), memoryStore: store, evaluator: testEvaluator(), calibrationEnabled: true}

	// Two calls must not panic; sync.Once guarantees a single calibration.
	svc.ensureCalibrated(context.Background(), "owner/repo")
	svc.ensureCalibrated(context.Background(), "owner/repo")
}

func TestEnsureCalibrated_NoHistoryNoOp(t *testing.T) {
	store := memory.NewInMemoryStore()
	svc := &Service{logger: testLogger(), memoryStore: store, evaluator: testEvaluator(), calibrationEnabled: true}
	// No seeded history — must be a safe no-op.
	svc.ensureCalibrated(context.Background(), "owner/repo")
}
