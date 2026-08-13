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

func TestEarnedTrustLevel_Tiers(t *testing.T) {
	tests := []struct {
		name      string
		score     reputation.Score
		wantTrust cgp.TrustLevel
	}{
		{
			name:      "strong record earns full",
			score:     reputation.Score{Overall: 0.96, SampleSize: 60, Trend: reputation.TrendStable},
			wantTrust: cgp.TrustLevelFull,
		},
		{
			name:      "full reputation but declining trend caps at trusted",
			score:     reputation.Score{Overall: 0.96, SampleSize: 60, Trend: reputation.TrendDeclining},
			wantTrust: cgp.TrustLevelTrusted,
		},
		{
			name:      "good record earns trusted",
			score:     reputation.Score{Overall: 0.82, SampleSize: 15, Trend: reputation.TrendStable},
			wantTrust: cgp.TrustLevelTrusted,
		},
		{
			name:      "enough reputation but too few samples earns nothing",
			score:     reputation.Score{Overall: 0.9, SampleSize: 5, Trend: reputation.TrendStable},
			wantTrust: cgp.TrustLevelUntrusted,
		},
		{
			name:      "many samples but weak reputation earns nothing",
			score:     reputation.Score{Overall: 0.5, SampleSize: 100, Trend: reputation.TrendStable},
			wantTrust: cgp.TrustLevelUntrusted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := earnedTrustLevel(tt.score, defaultEarnedTrustMinSamples, defaultEarnedTrustFullSamples)
			if got != tt.wantTrust {
				t.Fatalf("earnedTrustLevel = %s, want %s", got, tt.wantTrust)
			}
		})
	}
}

func TestApplyEarnedTrust_RaisesOnly(t *testing.T) {
	store := memory.NewInMemoryStore()
	actorID := "human:steady@example.com"
	seedReleases(t, store, actorID, "owner/repo", memory.OutcomeSuccess, 15)

	svc := &Service{logger: testLogger(), memoryStore: store, earnedTrustEnabled: true}

	// Actor constructed Untrusted; a strong record should raise to Trusted.
	proposal := cgp.NewProposal(
		cgp.Actor{ID: actorID, Kind: cgp.ActorKindHuman, TrustLevel: cgp.TrustLevelUntrusted},
		cgp.ProposalScope{Repository: "owner/repo"},
		cgp.ProposalIntent{},
	)

	score, ok := svc.governingActorReputation(context.Background(), "owner/repo", proposal)
	if !ok {
		t.Fatal("expected a reputation to be computed with a memory store and earned trust enabled")
	}

	info := svc.applyEarnedTrust(proposal, score)
	if info == nil {
		t.Fatal("expected escalation for strong record")
	}
	if proposal.Actor.TrustLevel < cgp.TrustLevelTrusted {
		t.Fatalf("trust must be raised to at least Trusted; got %s", proposal.Actor.TrustLevel)
	}
	if info.FromLevel != "untrusted" {
		t.Fatalf("expected FromLevel untrusted; got %s", info.FromLevel)
	}
}

func TestApplyEarnedTrust_NeverLowers(t *testing.T) {
	store := memory.NewInMemoryStore()
	actorID := "human:weak@example.com"
	// Few records, failed: earns nothing.
	seedReleases(t, store, actorID, "owner/repo", memory.OutcomeFailed, 2)

	svc := &Service{logger: testLogger(), memoryStore: store, earnedTrustEnabled: true}

	// Actor already constructed Full — escalation must not lower it, and must
	// not produce an escalation record.
	proposal := cgp.NewProposal(
		cgp.Actor{ID: actorID, Kind: cgp.ActorKindHuman, TrustLevel: cgp.TrustLevelFull},
		cgp.ProposalScope{Repository: "owner/repo"},
		cgp.ProposalIntent{},
	)

	score, ok := svc.governingActorReputation(context.Background(), "owner/repo", proposal)
	if !ok {
		t.Fatal("expected a reputation to be computed with a memory store and earned trust enabled")
	}

	info := svc.applyEarnedTrust(proposal, score)
	if info != nil {
		t.Fatalf("must not escalate a weak record; got %#v", info)
	}
	if proposal.Actor.TrustLevel != cgp.TrustLevelFull {
		t.Fatalf("must not lower existing trust; got %s", proposal.Actor.TrustLevel)
	}
}

// TestEvaluateRelease_EarnedTrustUnlocksAutoApprove is the end-to-end proof that
// a strong, verifiable track record unlocks low-risk auto-approval a fresh actor
// would not get.
func TestEvaluateRelease_EarnedTrustUnlocksAutoApprove(t *testing.T) {
	store := memory.NewInMemoryStore()
	actor := cgp.NewHumanActor("steady@example.com", "Steady")
	seedReleases(t, store, actor.ID, "owner/repo", memory.OutcomeSuccess, 15)

	eval := evaluator.New(
		evaluator.WithRiskCalculator(risk.NewCalculatorWithDefaults()),
		evaluator.WithPolicyEngine(policy.NewEngine(nil, nil)),
	)
	svc := NewService(eval,
		WithLogger(testLogger()),
		WithMemoryStore(store),
		WithEarnedTrust(true),
	)

	// Baseline: same release without earned trust does not auto-approve.
	baseline := NewService(eval, WithLogger(testLogger()), WithMemoryStore(store))
	relBase := createTestRelease(t)
	baseOut, err := baseline.EvaluateRelease(context.Background(), EvaluateReleaseInput{
		Release: relBase, Actor: actor, Repository: "owner/repo",
	})
	if err != nil {
		t.Fatalf("baseline EvaluateRelease() error = %v", err)
	}

	rel := createTestRelease(t)
	out, err := svc.EvaluateRelease(context.Background(), EvaluateReleaseInput{
		Release: rel, Actor: actor, Repository: "owner/repo",
	})
	if err != nil {
		t.Fatalf("EvaluateRelease() error = %v", err)
	}

	if out.EarnedTrust == nil {
		t.Fatal("expected earned-trust escalation for strong record")
	}
	if out.EarnedTrust.ToLevel != "trusted" && out.EarnedTrust.ToLevel != "full" {
		t.Fatalf("expected escalation to trusted/full; got %s", out.EarnedTrust.ToLevel)
	}
	if !out.CanAutoApprove {
		t.Fatalf("earned-trusted actor must auto-approve a low-risk change; baseline was %v", baseOut.CanAutoApprove)
	}
}
