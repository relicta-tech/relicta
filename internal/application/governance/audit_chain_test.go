package governance

// audit_chain_test.go proves the two governance entries no release event can carry.
//
// The rest of the chain comes off the release event stream, which raises nothing for a
// risk evaluation and nothing at all for a rejection — `relicta approve` refuses and
// exits. So the only place a rejected release becomes evidence is here, and a regression
// would not show up as a broken chain: it would show up as a chain in which governance
// never said no to anything.

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/audit"
	"github.com/relicta-tech/relicta/v4/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
	"github.com/relicta-tech/relicta/v4/internal/cgp/risk"
)

const chainRepo = "owner/repo"

func chainService(t *testing.T, store memory.Store, policies []policy.Policy) *Service {
	t.Helper()
	eval := evaluator.New(
		evaluator.WithRiskCalculator(risk.NewCalculatorWithDefaults()),
		evaluator.WithPolicyEngine(policy.NewEngine(policies, nil)),
	)
	return NewService(eval, WithLogger(testLogger()), WithMemoryStore(store))
}

func chainAfterEvaluation(t *testing.T, store memory.Store) []*audit.Entry {
	t.Helper()
	chain, err := audit.LoadChain(context.Background(), store, chainRepo)
	if err != nil {
		t.Fatalf("the chain written by an evaluation does not verify: %v", err)
	}
	return chain.List()
}

func TestAnEvaluationRecordsTheRiskAssessmentAndTheVerdict(t *testing.T) {
	store := memory.NewInMemoryStore()
	svc := chainService(t, store, nil)

	if _, err := svc.EvaluateRelease(context.Background(), EvaluateReleaseInput{
		Release:    createTestRelease(t),
		Actor:      cgp.NewHumanActor("alice@example.com", "Alice"),
		Repository: chainRepo,
	}); err != nil {
		t.Fatalf("EvaluateRelease: %v", err)
	}

	entries := chainAfterEvaluation(t, store)
	if len(entries) != 2 {
		t.Fatalf("one evaluation produced %d chain entries, want 2 (the assessment and "+
			"the verdict)", len(entries))
	}
	if entries[0].EventType != audit.EventEvaluationCompleted {
		t.Errorf("the first entry is %s, want evaluation.completed", entries[0].EventType)
	}
	if entries[1].EventType != audit.EventDecisionMade {
		t.Errorf("the second entry is %s, want decision.made", entries[1].EventType)
	}
	if _, ok := entries[0].Details["riskScore"]; !ok {
		t.Error("the risk assessment entry carries no risk score, so the chain records " +
			"that a release was evaluated and not what the evaluation found")
	}
}

// The entry that exists for no other reason. A rejected release raises no domain event, so
// without this one the chain would contain every approval and no refusal.
func TestARejectedReleaseIsRecordedInTheChain(t *testing.T) {
	store := memory.NewInMemoryStore()

	blockEverything := policy.NewPolicy("blocked")
	blockEverything.Defaults = policy.Defaults{Decision: policy.DecisionReject}
	svc := chainService(t, store, []policy.Policy{*blockEverything})

	out, err := svc.EvaluateRelease(context.Background(), EvaluateReleaseInput{
		Release:    createTestRelease(t),
		Actor:      cgp.NewHumanActor("alice@example.com", "Alice"),
		Repository: chainRepo,
	})
	if err != nil {
		t.Fatalf("EvaluateRelease: %v", err)
	}
	if out.Decision != cgp.DecisionRejected {
		t.Fatalf("the test policy did not reject the release; decision = %s", out.Decision)
	}

	entries := chainAfterEvaluation(t, store)
	decision := entries[len(entries)-1]
	if got := decision.Details["decisionType"]; got != string(cgp.DecisionRejected) {
		t.Errorf("the chain records the verdict as %v, want rejected: governance refused "+
			"a release and left no evidence that it did", got)
	}
}

// `relicta plan` evaluates a run it built to answer "what would this cost" and then throws
// away — a different plan hash, so a run ID no release, attestation or history entry ever
// refers to. Recorded, those entries are an audit trail nobody can join to anything.
func TestAPreviewEvaluationIsNotEvidence(t *testing.T) {
	store := memory.NewInMemoryStore()
	svc := chainService(t, store, nil)

	if _, err := svc.EvaluateRelease(context.Background(), EvaluateReleaseInput{
		Release:    createTestRelease(t),
		Actor:      cgp.NewHumanActor("alice@example.com", "Alice"),
		Repository: chainRepo,
		Preview:    true,
	}); err != nil {
		t.Fatalf("EvaluateRelease: %v", err)
	}

	if entries := chainAfterEvaluation(t, store); len(entries) != 0 {
		t.Errorf("a preview evaluation left %d entries in the chain, want none: the "+
			"evidence names a run that never becomes a release", len(entries))
	}
}

// Governance memory is optional, and a build without it must evaluate releases rather than
// fail them.
func TestAnEvaluationWithoutAStoreStillDecides(t *testing.T) {
	svc := chainService(t, nil, nil)

	out, err := svc.EvaluateRelease(context.Background(), EvaluateReleaseInput{
		Release:    createTestRelease(t),
		Actor:      cgp.NewHumanActor("alice@example.com", "Alice"),
		Repository: chainRepo,
	})
	if err != nil {
		t.Fatalf("evaluating with governance memory disabled: %v", err)
	}
	if out == nil {
		t.Fatal("no decision was produced")
	}
}
