package cli

import (
	"testing"
)

// The cgp_* MCP tools record a governance handshake, and until now those records
// were reachable only over MCP — and only by already knowing a proposal's ID,
// which is exactly what someone auditing a release afterwards does not have.
// `relicta cgp list` and `relicta cgp status` are the reading surface.

// The commands have to be registered, or the reading surface does not exist. A
// correct command that nothing registers is the failure mode this codebase keeps
// producing: `WithAdapterRepo`, `WithGitService`, `RequireRole` and `WithStore`
// were all implemented and attached to nothing.
func TestCGPCommandsAreRegistered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "cgp" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("`relicta cgp` is not registered on the root command")
	}

	want := map[string]bool{"list": false, "status": false}
	for _, c := range cgpCmd.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, registered := range want {
		if !registered {
			t.Errorf("`relicta cgp %s` is not registered", name)
		}
	}
}

// status takes exactly one proposal ID. Without the check cobra would pass an
// empty slice and the command would index args[0] on a nil slice.
func TestCGPStatusRequiresAProposalID(t *testing.T) {
	if cgpStatusCmd.Args == nil {
		t.Fatal("cgp status must declare its argument requirement")
	}
	if err := cgpStatusCmd.Args(cgpStatusCmd, []string{}); err == nil {
		t.Error("cgp status with no argument must be rejected")
	}
	if err := cgpStatusCmd.Args(cgpStatusCmd, []string{"prop_a", "prop_b"}); err == nil {
		t.Error("cgp status takes one proposal, not several")
	}
	if err := cgpStatusCmd.Args(cgpStatusCmd, []string{"prop_a"}); err != nil {
		t.Errorf("one proposal ID must be accepted: %v", err)
	}
}

// These commands read. Proposing and authorizing belong to whoever is making the
// change, and an audit command that could alter the record it reports would
// undermine the record's value as evidence.
func TestCGPCommandsAreReadOnly(t *testing.T) {
	for _, c := range cgpCmd.Commands() {
		switch c.Name() {
		case "list", "status":
			// Expected.
		default:
			t.Errorf("unexpected subcommand %q: `relicta cgp` is deliberately read-only, "+
				"so a mutating verb here needs its own justification", c.Name())
		}
	}
}
