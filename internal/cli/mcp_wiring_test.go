package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every defect in this area has been a working option that nothing called: the three cgp_*
// tools were advertised and always failed because neither WithCGPService nor WithEvaluator
// was wired; five resources answered "no release repository configured" because
// WithReleaseRepository was wired nowhere; and a configured actor_budget_path gated the CLI
// while the MCP surface — where agents actually operate — ignored it.
//
// The option implementations are covered in internal/mcp, by tests that drive the behavior
// each option unlocks. What no test covered is the part that was actually broken: whether
// this file passes them. That is asserted here by reading the construction, following
// release_repo_path_test.go, because building a real container needs a git repository and a
// working-directory dance while the property at issue is simply "this option is passed".
//
// A source scan cannot prove the option reaches a live server, so it is deliberately paired
// with the behavioral tests in internal/mcp rather than standing in for them.
func TestMCPServeWiresTheOptionsItsFeaturesDependOn(t *testing.T) {
	source, err := os.ReadFile(filepath.Clean("mcp.go"))
	if err != nil {
		t.Fatalf("read mcp.go: %v", err)
	}
	s := string(source)

	for _, w := range []struct {
		call string
		why  string
	}{
		{
			call: "mcp.WithEvaluator(",
			why: "the three cgp_* tools are advertised in tools/list and need an evaluator; " +
				"without one every call returns a refusal",
		},
		{
			call: "mcp.WithReleaseRepository(",
			why: "release state, active runs, history and the run recommendation are served " +
				"from the repository; without it each answers \"no release repository " +
				"configured\", which a caller cannot tell apart from having no release",
		},
		{
			call: "mcp.WithActorBudgets(",
			why: "the operator's per-actor autonomy budgets must apply on the surface agents " +
				"use, not only in the CLI",
		},
		{
			call: "mcp.WithGitService(",
			why: "resolving the repository path; without it the server reads whatever " +
				"directory it was started from",
		},
	} {
		if !strings.Contains(s, w.call) {
			t.Errorf("mcp serve does not call %s — %s", w.call, w.why)
		}
	}
}

// The evaluator has to be the governance service's own, not a fresh one. A fresh evaluator
// carries default thresholds and no policies, so cgp_propose would decide by different
// rules than relicta_evaluate and `relicta approve` — two governance verdicts for one
// change, with nothing saying which is authoritative.
func TestTheMCPEvaluatorIsTheGovernanceServices(t *testing.T) {
	source, err := os.ReadFile(filepath.Clean("mcp.go"))
	if err != nil {
		t.Fatalf("read mcp.go: %v", err)
	}
	s := string(source)

	if strings.Contains(s, "mcp.WithEvaluator(evaluator.New(") {
		t.Error("mcp serve builds a fresh evaluator: an agent would then be governed by " +
			"default thresholds and no policies, disagreeing with the CLI on the same change")
	}
	if !strings.Contains(s, "govSvc.Evaluator()") {
		t.Error("expected the evaluator to come from the governance service, so the protocol " +
			"surface and the CLI share one set of rules")
	}
}
