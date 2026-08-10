package governance

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
	"github.com/relicta-tech/relicta/v4/internal/cgp/reputation"
	"github.com/relicta-tech/relicta/v4/internal/cgp/risk"
)

// testScore builds a reputation score with the fields attachActorReputation reads.
func testScore(overall float64, samples int, trend reputation.Trend) reputation.Score {
	return reputation.Score{Overall: overall, SampleSize: samples, Trend: trend}
}

// reputationPolicy requires review for an actor whose computed reputation is
// weak. Its defaults approve, so the rule is the only thing that can raise the
// requirement — if actor.reputation.overall does not reach the evaluator, the
// release comes back approved and this test fails.
func reputationPolicy() policy.Policy {
	pol := policy.NewPolicy("reputation-aware")
	pol.Defaults = policy.Defaults{Decision: policy.DecisionApprove}
	pol.AddRule(*policy.NewRule("weak-record-review", "Review releases from actors with a weak record").
		WithPriority(50).
		AddCondition("actor.reputation.overall", policy.OperatorLessThan, 0.6).
		AddAction(policy.ActionRequireApproval, map[string]any{"count": float64(1)}))
	return *pol
}

func reputationService(t *testing.T, store memory.Store, opts ...ServiceOption) *Service {
	t.Helper()
	eval := evaluator.New(
		evaluator.WithRiskCalculator(risk.NewCalculatorWithDefaults()),
		evaluator.WithPolicyEngine(policy.NewEngine([]policy.Policy{reputationPolicy()}, nil)),
	)
	return NewService(eval, append([]ServiceOption{WithLogger(testLogger())}, opts...)...)
}

// TestEvaluateRelease_PolicyCanConditionOnReputation is the wiring proof. The
// evaluator is built from configuration and never sees the memory store, so a
// reputation that is computed but not attached to the proposal is a reputation no
// policy can read — the rule would load, report itself enabled, and never fire.
func TestEvaluateRelease_PolicyCanConditionOnReputation(t *testing.T) {
	store := memory.NewInMemoryStore()
	actor := cgp.NewHumanActor("shaky@example.com", "Shaky")
	// A record of failures: enough samples for a real score, and a poor one.
	seedReleases(t, store, actor.ID, "owner/repo", memory.OutcomeFailed, 12)

	svc := reputationService(t, store, WithMemoryStore(store), WithReputation(true))
	out, err := svc.EvaluateRelease(context.Background(), EvaluateReleaseInput{
		Release:    createTestRelease(t),
		Actor:      actor,
		Repository: "owner/repo",
	})
	if err != nil {
		t.Fatalf("EvaluateRelease() error = %v", err)
	}
	if out.Decision != cgp.DecisionApprovalRequired {
		t.Fatalf("a policy rule on actor.reputation.overall must require review for a weak record; got %s, rationale %v",
			out.Decision, out.Rationale)
	}
}

// The same release with reputation switched off must NOT be treated as a weak
// record. This is the failure mode the absent-versus-zero distinction exists for:
// a zero score exposed where nothing was computed would make the rule fire for
// every actor in a deployment that never computes reputation.
func TestEvaluateRelease_ReputationAbsentWhenDisabled(t *testing.T) {
	store := memory.NewInMemoryStore()
	actor := cgp.NewHumanActor("shaky@example.com", "Shaky")
	seedReleases(t, store, actor.ID, "owner/repo", memory.OutcomeFailed, 12)

	// Memory store present, but neither reputation guarding nor earned trust is on.
	svc := reputationService(t, store, WithMemoryStore(store))
	out, err := svc.EvaluateRelease(context.Background(), EvaluateReleaseInput{
		Release:    createTestRelease(t),
		Actor:      actor,
		Repository: "owner/repo",
	})
	if err != nil {
		t.Fatalf("EvaluateRelease() error = %v", err)
	}
	if out.Decision != cgp.DecisionApproved {
		t.Fatalf("with reputation not computed the rule must not fire; got %s, rationale %v",
			out.Decision, out.Rationale)
	}
}

func TestGoverningActorReputation_RequiresStoreAndFlag(t *testing.T) {
	store := memory.NewInMemoryStore()
	actorID := "human:steady@example.com"
	seedReleases(t, store, actorID, "owner/repo", memory.OutcomeSuccess, 12)

	proposal := cgp.NewProposal(
		cgp.Actor{ID: actorID, Kind: cgp.ActorKindHuman},
		cgp.ProposalScope{Repository: "owner/repo"},
		cgp.ProposalIntent{},
	)

	cases := []struct {
		name string
		svc  *Service
		want bool
	}{
		{"no memory store", &Service{logger: testLogger(), reputationEnabled: true}, false},
		{"store but neither feature enabled", &Service{logger: testLogger(), memoryStore: store}, false},
		{"reputation guarding enabled", &Service{logger: testLogger(), memoryStore: store, reputationEnabled: true}, true},
		{"earned trust enabled", &Service{logger: testLogger(), memoryStore: store, earnedTrustEnabled: true}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := tc.svc.governingActorReputation(context.Background(), "owner/repo", proposal)
			if ok != tc.want {
				t.Fatalf("governingActorReputation ok = %v, want %v", ok, tc.want)
			}
		})
	}
}

func TestAttachActorReputation_SurvivesExistingContext(t *testing.T) {
	proposal := cgp.NewProposal(
		cgp.Actor{ID: "human:dev", Kind: cgp.ActorKindHuman},
		cgp.ProposalScope{Repository: "owner/repo"},
		cgp.ProposalIntent{},
	)
	// Attribution and trust escalation already write to the proposal context; the
	// reputation must join them rather than replace what they recorded.
	proposal.AddMetadata("attribution.method", "trailer")

	attachActorReputation(proposal, testScore(0.72, 9, "improving"))

	if proposal.Context.ActorReputation == nil {
		t.Fatal("reputation was not attached")
	}
	if got := proposal.Context.ActorReputation.Level; got != "reliable" {
		t.Errorf("level = %q, want reliable", got)
	}
	if got := proposal.Context.ActorReputation.SampleSize; got != 9 {
		t.Errorf("sampleSize = %d, want 9", got)
	}
	if proposal.Context.Metadata["attribution.method"] != "trailer" {
		t.Error("attaching reputation dropped existing proposal metadata")
	}
}
