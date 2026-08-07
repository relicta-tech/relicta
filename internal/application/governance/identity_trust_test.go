package governance

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/v4/internal/cgp/identity"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
	"github.com/relicta-tech/relicta/v4/internal/cgp/risk"
)

func TestTrustScoreToLevel(t *testing.T) {
	tests := []struct {
		score float64
		want  cgp.TrustLevel
	}{
		{0.95, cgp.TrustLevelFull},
		{0.9, cgp.TrustLevelFull},
		{0.8, cgp.TrustLevelTrusted},
		{0.7, cgp.TrustLevelTrusted},
		{0.6, cgp.TrustLevelLimited},
		{0.5, cgp.TrustLevelLimited},
		{0.4, cgp.TrustLevelUntrusted},
		{0.0, cgp.TrustLevelUntrusted},
	}
	for _, tt := range tests {
		if got := trustScoreToLevel(tt.score); got != tt.want {
			t.Errorf("trustScoreToLevel(%.2f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestActorAndIdentityLocalName(t *testing.T) {
	if got := actorLocalName("agent:claude-code"); got != "claude-code" {
		t.Errorf("actorLocalName = %s, want claude-code", got)
	}
	if got := actorLocalName("noprefix"); got != "noprefix" {
		t.Errorf("actorLocalName(noprefix) = %s", got)
	}
	if got := identityLocalName("claude-code@team-platform"); got != "claude-code" {
		t.Errorf("identityLocalName = %s, want claude-code", got)
	}
	if got := identityLocalName("nnoscope"); got != "nnoscope" {
		t.Errorf("identityLocalName(nnoscope) = %s", got)
	}
}

// testRegistry builds a registry seeded with the given identities.
func testRegistry(t *testing.T, identities ...*identity.ActorIdentity) *identity.Registry {
	t.Helper()
	store, err := identity.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}
	reg, err := identity.NewRegistry(context.Background(), store)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	for _, id := range identities {
		if err := reg.Register(context.Background(), id); err != nil {
			t.Fatalf("register %s: %v", id.ID, err)
		}
	}
	return reg
}

func agentIdentity(id string, score float64) *identity.ActorIdentity {
	return &identity.ActorIdentity{
		ID:           id,
		Kind:         cgp.ActorKindAgent,
		Organization: "acme",
		TrustScore:   score,
	}
}

func TestApplyIdentityTrust_GrantsRegisteredTrust(t *testing.T) {
	reg := testRegistry(t, agentIdentity("claude-code@team-platform", 0.95))
	svc := &Service{logger: testLogger(), identityRegistry: reg}

	proposal := cgp.NewProposal(
		cgp.Actor{ID: "agent:claude-code", Kind: cgp.ActorKindAgent, TrustLevel: cgp.TrustLevelUntrusted},
		cgp.ProposalScope{}, cgp.ProposalIntent{},
	)

	info := svc.applyIdentityTrust(context.Background(), proposal)
	if info == nil {
		t.Fatal("expected identity grant")
	}
	if proposal.Actor.TrustLevel != cgp.TrustLevelFull {
		t.Fatalf("score 0.95 must grant full; got %s", proposal.Actor.TrustLevel)
	}
	if info.IdentityID != "claude-code@team-platform" {
		t.Fatalf("unexpected identity matched: %s", info.IdentityID)
	}
}

func TestApplyIdentityTrust_NeverLowers(t *testing.T) {
	// Registered score maps to Trusted, but the actor is already Full.
	reg := testRegistry(t, agentIdentity("claude-code@team", 0.7))
	svc := &Service{logger: testLogger(), identityRegistry: reg}

	proposal := cgp.NewProposal(
		cgp.Actor{ID: "agent:claude-code", Kind: cgp.ActorKindAgent, TrustLevel: cgp.TrustLevelFull},
		cgp.ProposalScope{}, cgp.ProposalIntent{},
	)

	if info := svc.applyIdentityTrust(context.Background(), proposal); info != nil {
		t.Fatalf("must not lower trust; got %#v", info)
	}
	if proposal.Actor.TrustLevel != cgp.TrustLevelFull {
		t.Fatalf("trust must stay full; got %s", proposal.Actor.TrustLevel)
	}
}

func TestApplyIdentityTrust_NoMatchNoGrant(t *testing.T) {
	reg := testRegistry(t, agentIdentity("other-agent@team", 0.95))
	svc := &Service{logger: testLogger(), identityRegistry: reg}

	proposal := cgp.NewProposal(
		cgp.Actor{ID: "agent:claude-code", Kind: cgp.ActorKindAgent, TrustLevel: cgp.TrustLevelUntrusted},
		cgp.ProposalScope{}, cgp.ProposalIntent{},
	)

	if info := svc.applyIdentityTrust(context.Background(), proposal); info != nil {
		t.Fatalf("no matching identity must not grant; got %#v", info)
	}
}

func TestApplyIdentityTrust_KindMustMatch(t *testing.T) {
	// Same local name but registered as CI, governing actor is an agent.
	reg := testRegistry(t, &identity.ActorIdentity{
		ID: "release-bot@team", Kind: cgp.ActorKindCI, Organization: "acme", TrustScore: 0.95,
	})
	svc := &Service{logger: testLogger(), identityRegistry: reg}

	proposal := cgp.NewProposal(
		cgp.Actor{ID: "agent:release-bot", Kind: cgp.ActorKindAgent, TrustLevel: cgp.TrustLevelUntrusted},
		cgp.ProposalScope{}, cgp.ProposalIntent{},
	)

	if info := svc.applyIdentityTrust(context.Background(), proposal); info != nil {
		t.Fatalf("kind mismatch must not grant; got %#v", info)
	}
}

// TestEvaluateRelease_IdentityGrantUnlocksAutoApprove proves an org-granted
// identity raises trust enough to auto-approve a low-risk change end to end.
func TestEvaluateRelease_IdentityGrantUnlocksAutoApprove(t *testing.T) {
	reg := testRegistry(t, &identity.ActorIdentity{
		ID: "alice@acme", Kind: cgp.ActorKindHuman, Organization: "acme", TrustScore: 0.95,
	})
	eval := evaluator.New(
		evaluator.WithRiskCalculator(risk.NewCalculatorWithDefaults()),
		evaluator.WithPolicyEngine(policy.NewEngine(nil, nil)),
	)
	svc := NewService(eval, WithLogger(testLogger()), WithIdentityRegistry(reg))

	actor := cgp.Actor{ID: "human:alice", Kind: cgp.ActorKindHuman, Name: "alice"}
	out, err := svc.EvaluateRelease(context.Background(), EvaluateReleaseInput{
		Release: createTestRelease(t), Actor: actor, Repository: "owner/repo",
	})
	if err != nil {
		t.Fatalf("EvaluateRelease() error = %v", err)
	}
	if out.IdentityTrust == nil {
		t.Fatal("expected identity grant recorded")
	}
	if !out.CanAutoApprove {
		t.Fatal("identity-granted actor must auto-approve a low-risk change")
	}
}
