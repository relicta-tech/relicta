package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Six defects this week were the same shape: a constructor or option that exists,
// is correct, and is called from nowhere. FindByPlanHash, WithSkipPush,
// WithAdapterRepo, WithGitService, hub's Dockerfile, hub's web/ CI. Unit tests
// missed every one of them, because the component under test worked — what was
// missing was the call.
//
// A sweep found 40 of 87 With* constructors never called outside tests. Most are
// legitimate test seams (WithNowFunc, WithHTTPClient). The dangerous ones are
// those the production path silently degrades without, and the MCP server has
// several: without WithGitService, ensureRepoPath falls back to "." and the server
// reports "no active release" for a repository that has one.
//
// This test reads the wiring rather than exercising it, deliberately. Exercising
// would need a repository, a container and a live server per option; reading
// catches "nobody passes this" — which is the actual failure — at negligible cost.

// mcpServerOptionsThatMustBeWired are Server options whose absence changes
// behavior silently rather than loudly. Each entry says what breaks, so a failure
// explains itself instead of just naming a symbol.
var mcpServerOptionsThatMustBeWired = map[string]string{
	"WithGitService": "ensureRepoPath falls back to \".\", so the server resolves the " +
		"wrong repository when started from a subdirectory and reports no active release",
}

func TestMCPServerWiresOptionsItDependsOn(t *testing.T) {
	source := readSource(t, "mcp.go")

	for option, consequence := range mcpServerOptionsThatMustBeWired {
		if !strings.Contains(source, "mcp."+option+"(") {
			t.Errorf("internal/cli/mcp.go never calls mcp.%s — %s", option, consequence)
		}
	}
}

// mcpAdapterOptionsThatMustBeWired is the same guarantee for the Adapter.
var mcpAdapterOptionsThatMustBeWired = map[string]string{
	"WithAdapterRepo": "Adapter.Evaluate refuses with \"release repository not configured\", " +
		"so relicta_evaluate cannot run",
	"WithReleaseServices": "the plan/bump/notes/approve/publish tools fall back to stubs",
	"WithGovernanceService": "relicta_evaluate has no governance to call and the " +
		"recommendation artifact carries no verdict",
	"WithToolVersion": "recommendation provenance reports \"unknown\" instead of the " +
		"binary that produced the artifact",
}

func TestMCPAdapterWiresOptionsItDependsOn(t *testing.T) {
	source := readSource(t, "mcp.go")

	for option, consequence := range mcpAdapterOptionsThatMustBeWired {
		if !strings.Contains(source, "mcp."+option+"(") {
			t.Errorf("internal/cli/mcp.go never calls mcp.%s — %s", option, consequence)
		}
	}
}

// TestMCPEvaluateRepositoryIsPopulated guards a subtler omission than a missing
// option: governance validates that a proposal names a repository, and
// EvaluateInput.Repository was left empty, so every relicta_evaluate call failed
// with "repository is required" while every option was correctly wired.
func TestMCPEvaluateRepositoryIsPopulated(t *testing.T) {
	source := readSource(t, "../mcp/server_tools.go")

	// The handler must both keep ensureRepoPath's return value and pass it on.
	if !regexp.MustCompile(`repoPath\s*:?=\s*s\.ensureRepoPath\(ctx\)`).MatchString(source) {
		t.Error("handleEvaluate discards ensureRepoPath's return value; the resolved " +
			"root is needed for EvaluateInput.Repository")
	}
	if !strings.Contains(source, "Repository:     repoPath") {
		t.Error("EvaluateInput.Repository is not set from the resolved repository root; " +
			"governance rejects the proposal with \"repository is required\"")
	}
}

func readSource(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}
