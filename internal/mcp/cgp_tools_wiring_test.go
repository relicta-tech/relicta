package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
	"github.com/relicta-tech/relicta/v4/internal/config"
)

// The three cgp_* tools are advertised in tools/list, and every call to them
// failed. They need either WithCGPService or WithEvaluator, and `relicta mcp serve`
// wired neither — so ensureCGPService took its error path on every request, for as
// long as the tools have existed. An agent reading the tool list saw three
// governance tools and could use none of them.
//
// These tests are about the seam being connected, which is the part that was
// missing. The handlers themselves were fine.

func TestCGPToolsRefuseWithoutAnEvaluator(t *testing.T) {
	server, err := NewServer("test")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	_, err = server.handleCGPPropose(context.Background(), CGPProposeToolInput{
		Repository: "owner/repo",
		Summary:    "a change",
	})
	if err == nil {
		t.Fatal("expected a refusal when no evaluator is configured")
	}

	// It has to say why. Before this was a ToolInputError the agent saw only
	// "internal error" and could not tell an unconfigured server from a crash.
	if !strings.Contains(err.Error(), "not available") {
		t.Errorf("the refusal should explain itself; got %v", err)
	}
}

func TestCGPToolsWorkWithAnEvaluator(t *testing.T) {
	server, err := NewServer("test", WithEvaluator(evaluator.New()))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	out, err := server.handleCGPPropose(context.Background(), CGPProposeToolInput{
		Repository:  "owner/repo",
		CommitRange: "HEAD~1..HEAD",
		Summary:     "a change",
		ActorKind:   "agent",
		ActorID:     "agent:test",
	})
	if err != nil {
		t.Fatalf("cgp_propose with an evaluator must not fail: %v", err)
	}
	if out == "" {
		t.Error("expected a governance decision, got empty output")
	}
}

// The reason a bare evaluator is not the fix.
//
// Supplying any evaluator makes the tools respond, which looks like success. But
// an evaluator built here rather than taken from the governance service carries
// default thresholds and no policies, so cgp_propose would answer from different
// rules than relicta_evaluate and `relicta approve` — two governance verdicts for
// one change, with nothing indicating which was authoritative. This test pins the
// difference so "just give it an evaluator" cannot quietly become the fix later.
func TestEvaluatorCarriesItsPolicyEngine(t *testing.T) {
	// A policy that blocks everything, which no default configuration does.
	blockEverything := policy.Policy{
		Name: "test-marker",
		Rules: []policy.Rule{{
			ID: "block-all", Name: "block-all", Enabled: true, Priority: 100,
			Conditions: []policy.Condition{
				{Field: "risk.score", Operator: policy.OperatorGreaterOrEqual, Value: 0.0},
			},
			Actions: []policy.Action{{Type: policy.ActionBlock}},
		}},
		Defaults: policy.Defaults{Decision: "approve"},
	}

	configured := evaluator.New(
		evaluator.WithPolicyEngine(policy.NewEngine([]policy.Policy{blockEverything}, nil)),
	)

	withPolicy, err := NewServer("test", WithEvaluator(configured))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	bare, err := NewServer("test", WithEvaluator(evaluator.New()))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	input := CGPProposeToolInput{
		Repository:  "owner/repo",
		CommitRange: "HEAD~1..HEAD",
		Summary:     "a trivial change",
		ActorKind:   "human",
		ActorID:     "human:dev",
	}

	withPolicyOut, err := withPolicy.handleCGPPropose(context.Background(), input)
	if err != nil {
		t.Fatalf("propose with a policy: %v", err)
	}
	bareOut, err := bare.handleCGPPropose(context.Background(), input)
	if err != nil {
		t.Fatalf("propose without a policy: %v", err)
	}

	if withPolicyOut == bareOut {
		t.Error("a configured evaluator and a bare one produced identical decisions for " +
			"the same proposal, so the policy engine is not reaching cgp_propose — " +
			"which means the protocol tools would decide by different rules than the CLI")
	}
}

// The reload path had the same defect one level down. It refreshed config and
// adapter and nothing else, so the evaluator kept its startup value — and this
// path exists for the case where there was no config at startup, which is exactly
// when that value is nil. `relicta_init` reported "tools are now available" while
// the cgp_* tools stayed broken.
func TestReloadSuppliesTheEvaluator(t *testing.T) {
	// An empty directory, and Force, because handleInit returns early with
	// "config already exists" otherwise and the reloader never runs. The first
	// version of this test ran in the repository root, where .relicta.yaml exists,
	// so it took that early return — and the companion test below passed without
	// reload having happened at all.
	t.Chdir(t.TempDir())

	server, err := NewServer("test",
		WithConfigReloader(func(context.Context) (ReloadedComponents, error) {
			return ReloadedComponents{
				Config:    config.DefaultConfig(),
				Evaluator: evaluator.New(),
			}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Before reload the tools refuse, since nothing supplied an evaluator.
	if _, err := server.handleCGPPropose(context.Background(), CGPProposeToolInput{
		Repository: "owner/repo", Summary: "x",
	}); err == nil {
		t.Fatal("precondition: the tools should refuse before a reload")
	}

	if _, err := server.handleInit(context.Background(), InitToolInput{Force: true}); err != nil {
		t.Fatalf("handleInit: %v", err)
	}

	if _, err := server.handleCGPPropose(context.Background(), CGPProposeToolInput{
		Repository:  "owner/repo",
		CommitRange: "HEAD~1..HEAD",
		Summary:     "a change",
		ActorKind:   "agent",
		ActorID:     "agent:test",
	}); err != nil {
		t.Errorf("after init reported tools available, cgp_propose still fails: %v", err)
	}
}

// A reload must not discard a working component when initialization produced none.
func TestReloadKeepsTheExistingEvaluatorWhenNoneIsSupplied(t *testing.T) {
	t.Chdir(t.TempDir())

	reloaded := false
	server, err := NewServer("test",
		WithEvaluator(evaluator.New()),
		WithConfigReloader(func(context.Context) (ReloadedComponents, error) {
			reloaded = true
			return ReloadedComponents{Config: config.DefaultConfig()}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	if _, err := server.handleInit(context.Background(), InitToolInput{Force: true}); err != nil {
		t.Fatalf("handleInit: %v", err)
	}
	// Without this the test passes whether or not reload ran, which is how its
	// first version passed while taking handleInit's early return.
	if !reloaded {
		t.Fatal("the reloader did not run, so this asserts nothing")
	}

	if _, err := server.handleCGPPropose(context.Background(), CGPProposeToolInput{
		Repository:  "owner/repo",
		CommitRange: "HEAD~1..HEAD",
		Summary:     "a change",
		ActorKind:   "agent",
		ActorID:     "agent:test",
	}); err != nil {
		t.Errorf("a reload that supplies no evaluator must keep the one already set: %v", err)
	}
}
