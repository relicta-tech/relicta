package cli

// policy_init.go: writes a starting policy so governance can be authored from
// the binary rather than from a git checkout.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/cgp/policy/dsl"
	"github.com/relicta-tech/relicta/v4/internal/cli/templates"
)

var (
	policyInitTemplate string
	policyInitDir      string
	policyInitForce    bool
	policyInitList     bool
)

// runPolicyInit writes one embedded example policy into the policy directory.
//
// `policy scaffold` sounds like the command for this and is not: it generates
// test fixtures for policies that already exist, and exits 1 with "no policies
// found for scaffolding" in a repository that has none. So the first step of
// authoring a policy had no command at all — the documented route was to copy a
// file out of examples/, which a released binary does not ship.
func runPolicyInit(cmd *cobra.Command, _ []string) error {
	starters, err := templates.PolicyStarters()
	if err != nil {
		return err
	}

	if policyInitList {
		printPolicyStarters(starters)
		return nil
	}

	starter, err := templates.PolicyStarterByName(policyInitTemplate)
	if err != nil {
		return err
	}

	dir := policyInitDir
	if dir == "" {
		// The first of the directories the loader searches, so the file is live
		// the moment it is written.
		paths := dsl.DefaultPolicyPaths()
		if len(paths) == 0 {
			return fmt.Errorf("no policy search paths are configured")
		}
		dir = paths[0]
	}

	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create policy directory %s: %w", dir, err)
	}

	target := filepath.Join(dir, starter.Filename)
	if _, statErr := os.Stat(target); statErr == nil && !policyInitForce {
		// Overwriting silently would discard rules someone wrote, which is the
		// one thing a governance tool must not do to its own governance.
		return fmt.Errorf("%s already exists; pass --force to overwrite it, or "+
			"--template to write a different one", target)
	}

	if err := os.WriteFile(target, []byte(starter.Content), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}

	printSuccess(fmt.Sprintf("Wrote %s", target))
	fmt.Println()
	fmt.Println("This policy is active immediately — it is in a directory relicta reads.")
	fmt.Println("Next steps:")
	fmt.Printf("  relicta policy list          # confirm it loaded and see its rules\n")
	fmt.Printf("  relicta policy validate      # check it after you edit it\n")
	fmt.Printf("  relicta evaluate             # see the decision it produces for your changes\n")
	fmt.Println()
	fmt.Printf("Other starting points: relicta policy init --list\n")

	_ = cmd
	return nil
}

func printPolicyStarters(starters []templates.PolicyStarter) {
	fmt.Println("Policy templates included in this binary:")
	fmt.Println()
	for _, s := range starters {
		marker := " "
		if s.Name == templates.DefaultPolicyStarter {
			marker = "*"
		}
		fmt.Printf("  %s %-14s %s\n", marker, s.Name, s.Description)
	}
	fmt.Println()
	fmt.Printf("  * written by default\n")
	fmt.Println()
	fmt.Printf("Write one with: relicta policy init --template <name>\n")
}
