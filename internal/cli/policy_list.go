package cli

// policy_list.go: extracted from policy.go.

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/cgp/policy/dsl"
)

// runPolicyList prints all loadable policies + rules from the configured
// policy directories. Useful for an at-a-glance audit of governance posture.
func runPolicyList(cmd *cobra.Command, args []string) error {
	dirs := dsl.DefaultPolicyPaths()

	loader := dsl.NewLoader(dsl.LoaderOptions{
		IgnoreErrors: true,
		Recursive:    true,
	})

	var totalPolicies int
	var totalRules int

	for _, dir := range dirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			absDir = dir
		}

		result, err := loader.LoadDir(absDir)
		if err != nil || result == nil || len(result.Policies) == 0 {
			continue
		}

		fmt.Printf("\nPolicies from %s:\n", dir)
		fmt.Println(strings.Repeat("-", 50))

		for _, pol := range result.Policies {
			totalPolicies++
			ruleCount := len(pol.Rules)
			totalRules += ruleCount

			fmt.Printf("\n  %s (%d rules)\n", pol.Name, ruleCount)
			if pol.Description != "" {
				fmt.Printf("    Description: %s\n", pol.Description)
			}

			for _, rule := range pol.Rules {
				status := "enabled"
				if !rule.Enabled {
					status = "disabled"
				}
				fmt.Printf("    - %s (priority: %d, %s)\n",
					rule.Name, rule.Priority, status)
				if rule.Description != "" {
					fmt.Printf("      %s\n", rule.Description)
				}
			}
		}
	}

	if totalPolicies == 0 {
		// This message used to name the directories and the file extensions and
		// stop there. Both are true and neither is enough: policies are written in
		// a DSL, so a reader who now knows where to put the file and what to call
		// it still has no way to know what goes inside it. The command that writes
		// a working one is the useful next step.
		fmt.Println("No policies found.")
		fmt.Println("\nGovernance is still active — it runs on built-in defaults.")
		fmt.Println("Policies are how you add rules those defaults do not cover.")
		fmt.Println("\nWrite one to start from:")
		fmt.Println("  relicta policy init          # a basic risk-based policy")
		fmt.Println("  relicta policy init --list   # other starting points")
		fmt.Println("\nOr create .policy / .cgp files yourself in one of these directories:")
		for _, dir := range dirs {
			fmt.Printf("  - %s\n", dir)
		}
		return nil
	}

	fmt.Printf("\n%s\n", strings.Repeat("-", 50))
	fmt.Printf("Total: %d policies, %d rules\n", totalPolicies, totalRules)
	_ = cmd
	return nil
}
