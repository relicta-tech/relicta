package cli

import (
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/config"
)

// workflow.allowed_branches was validated, defaulted, and enforced by nothing: a repository
// restricting releases to main could release from anywhere.
//
// It hid from the unread-configuration sweep because the MCP server assigns to the field, and a
// write counts as a use. The only implementation of the check lived in internal/cgp/ciapproval,
// which nothing imports.

func TestBranchMatching(t *testing.T) {
	cases := []struct {
		allowed []string
		branch  string
		want    bool
	}{
		{[]string{"main"}, "main", true},
		{[]string{"main"}, "feature/x", false},
		{[]string{"main", "release/*"}, "release/1.0", true},
		{[]string{"release/*"}, "release/1.0/hotfix", false}, // path.Match does not cross /
		{[]string{"main", "master"}, "master", true},
	}

	for _, c := range cases {
		if got := branchIsAllowed(c.allowed, c.branch); got != c.want {
			t.Errorf("branchIsAllowed(%v, %q) = %v, want %v", c.allowed, c.branch, got, c.want)
		}
	}
}

// No list means no restriction, which is the default and what every repository has today.
func TestNoListMeansNoRestriction(t *testing.T) {
	orig := cfg
	t.Cleanup(func() { cfg = orig })

	cfg = config.DefaultConfig()
	cfg.Workflow.AllowedBranches = nil
	if !branchIsAllowed(cfg.Workflow.AllowedBranches, "anything") {
		t.Error("an empty list refused a branch; the default must not start refusing releases")
	}
}

// The message has to name the setting and the branch, or the operator cannot act on it.
func TestTheRefusalNamesTheSettingAndTheBranch(t *testing.T) {
	msg := branchRefusal([]string{"main", "release/*"}, "feature/x")

	for _, want := range []string{"workflow.allowed_branches", "feature/x", "main", "release/*"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal does not mention %q: %s", want, msg)
		}
	}
}
