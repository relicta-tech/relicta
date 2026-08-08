package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// `relicta --help` is the first thing most people read, and it had drifted into
// the opposite of what its own comment claimed ("a structured menu rather than a
// 30-item flat list"). Only 17 of 32 commands carried a GroupID, so Cobra filed
// the rest under "Additional Commands" — a 19-item bucket holding `status`,
// `evaluate`, `health`, `history`, `verify` and `rollback` next to `completion`
// and `demo`, while "Governance:" listed `policy` by itself.
//
// These tests are cheap and they protect a property nothing else would notice
// breaking: a command added without a GroupID looks fine and quietly lands in the
// bucket again.

// cobraBuiltins are added by Cobra itself and cannot be grouped from here.
var cobraBuiltins = map[string]bool{
	"completion": true,
	"help":       true,
}

func TestEveryCommandIsGrouped(t *testing.T) {
	var ungrouped []string
	for _, c := range rootCmd.Commands() {
		if cobraBuiltins[c.Name()] {
			continue
		}
		if c.GroupID == "" {
			ungrouped = append(ungrouped, c.Name())
		}
	}
	if len(ungrouped) > 0 {
		t.Errorf("these commands have no GroupID and will appear under "+
			"\"Additional Commands\" instead of a real section: %s",
			strings.Join(ungrouped, ", "))
	}
}

// A GroupID that matches no registered group is worse than none: Cobra drops the
// command from the help listing entirely rather than showing it somewhere odd.
func TestEveryGroupIDIsRegistered(t *testing.T) {
	registered := make(map[string]bool)
	for _, g := range rootCmd.Groups() {
		registered[g.ID] = true
	}
	for _, c := range rootCmd.Commands() {
		if c.GroupID != "" && !registered[c.GroupID] {
			t.Errorf("command %q has GroupID %q, which is not a registered group — "+
				"it will be omitted from --help", c.Name(), c.GroupID)
		}
	}
}

// TestVersionFlagExists covers the near-universal first thing anyone types.
// `relicta --version` used to fail with "unknown flag", and `-v` is --verbose, so
// the obvious shorthand guess failed too.
func TestVersionFlagExists(t *testing.T) {
	// Cobra will not register the flag at all when Version is empty, so this is
	// the actual precondition rather than a proxy for it.
	if rootCmd.Version == "" {
		t.Fatal("rootCmd.Version is empty, so Cobra registers no --version flag")
	}

	// Registration happens lazily inside Execute, which a unit test does not run.
	// Calling it directly checks that the flag really materializes instead of
	// trusting that a non-empty Version is sufficient.
	rootCmd.InitDefaultVersionFlag()
	if f := rootCmd.Flags().Lookup("version"); f == nil {
		t.Error("--version flag is not registered even with Version set")
	}
}

// -v must stay --verbose. Handing it to --version would silently change what
// every existing `-v` invocation does.
func TestShorthandVDoesNotBecomeVersion(t *testing.T) {
	f := rootCmd.PersistentFlags().ShorthandLookup("v")
	if f == nil {
		t.Fatal("-v shorthand is gone; it should still be --verbose")
	}
	if f.Name != "verbose" {
		t.Errorf("-v now means --%s; it must remain --verbose", f.Name)
	}
}

// The dashboard used to be two top-level commands one letter apart, `serve` and
// `server`, where `server`'s own help called itself "an enhanced alias for
// 'relicta serve'". One command with an alias keeps both spellings working while
// showing a single entry in --help.
func TestServeIsAnAliasNotASecondCommand(t *testing.T) {
	var serverCommands []*cobra.Command
	for _, c := range rootCmd.Commands() {
		if c.Name() == "serve" || c.Name() == "server" {
			serverCommands = append(serverCommands, c)
		}
	}
	if len(serverCommands) != 1 {
		names := make([]string, 0, len(serverCommands))
		for _, c := range serverCommands {
			names = append(names, c.Name())
		}
		t.Fatalf("expected one dashboard command, found %d (%s)", len(serverCommands), strings.Join(names, ", "))
	}

	cmd := serverCommands[0]
	if cmd.Name() != "server" {
		t.Errorf("expected the canonical name to be 'server', got %q", cmd.Name())
	}

	// The old spelling has to keep resolving, or this is a breaking rename.
	found, _, err := rootCmd.Find([]string{"serve"})
	if err != nil {
		t.Fatalf("'relicta serve' no longer resolves: %v", err)
	}
	if found.Name() != "server" {
		t.Errorf("'serve' resolved to %q, want the server command", found.Name())
	}

	// And it must still accept the flags it always had.
	for _, flag := range []string{"port", "address", "api-key", "no-auth"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("--%s is missing; 'relicta serve --%s' used to work", flag, flag)
		}
	}
}

// eval and evaluate do unrelated things one letter apart. Grouping separates
// them, but a reader scanning --help sees Short strings, so each must point at
// the other rather than leaving the reader to guess.
func TestEvalAndEvaluateDisambiguateThemselves(t *testing.T) {
	shorts := map[string]string{}
	for _, c := range rootCmd.Commands() {
		if c.Name() == "eval" || c.Name() == "evaluate" {
			shorts[c.Name()] = c.Short
		}
	}
	if len(shorts) != 2 {
		t.Fatalf("expected both eval and evaluate to exist, got %v", shorts)
	}
	if !strings.Contains(shorts["eval"], "evaluate") {
		t.Errorf("eval's summary should name 'evaluate' so the two are told apart: %q", shorts["eval"])
	}
	if !strings.Contains(shorts["evaluate"], "eval") {
		t.Errorf("evaluate's summary should name 'eval': %q", shorts["evaluate"])
	}
}
