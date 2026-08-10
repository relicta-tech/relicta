package cli

// policy_fields.go: lists the condition fields a policy may test.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
)

// runPolicyFields prints the fields the evaluator can resolve, grouped by
// context.
//
// The DSL's vocabulary was documented only in prose and only in part, so writing a
// condition meant guessing a name and getting no feedback when the guess was
// wrong — an unresolvable field makes a rule silently unmatchable rather than
// failing. Every policy shipped in this repository had at least one, and the
// mistakes were all of the same kind: is_weekend for isWeekend, day_of_week for
// weekday, change.files for scope.files. A list the tool prints from the evaluator
// itself cannot drift from what the evaluator accepts.
func runPolicyFields(cmd *cobra.Command, _ []string) error {
	fields := policy.KnownFieldPaths()
	if len(fields) == 0 {
		return fmt.Errorf("no fields reported by the evaluator")
	}

	// Group by top-level context, dropping the bare context names — `time` on its
	// own is a container, not something to compare.
	const topLevel = ""
	groups := make(map[string][]string)
	var order []string
	remember := func(root string) {
		if _, seen := groups[root]; !seen {
			order = append(order, root)
		}
	}

	for _, field := range fields {
		if i := strings.Index(field, "."); i > 0 {
			root := field[:i]
			remember(root)
			groups[root] = append(groups[root], field)
			continue
		}
		if isContextContainer(field, fields) {
			continue
		}
		remember(topLevel)
		groups[topLevel] = append(groups[topLevel], field)
	}
	sort.Strings(order)

	fmt.Println("Fields a policy condition can test:")
	for _, root := range order {
		members := groups[root]
		if len(members) == 0 {
			continue
		}
		fmt.Println()
		if root == "" {
			fmt.Println("  (top level)")
		} else {
			fmt.Printf("  %s\n", root)
		}
		for _, field := range members {
			fmt.Printf("    %s\n", field)
		}
	}

	fmt.Println()
	fmt.Println("Conditions on team.teams.<name> and team.roles.<name> are also valid;")
	fmt.Println("those names come from your team configuration.")
	fmt.Println()
	fmt.Println("Check a policy against this list with: relicta policy validate")

	_ = cmd
	return nil
}

// isContextContainer reports whether a top-level name is a container for other
// fields rather than a value in its own right, so `time` is not offered as
// something to compare while `bump_type` is.
func isContextContainer(name string, all []string) bool {
	prefix := name + "."
	for _, field := range all {
		if strings.HasPrefix(field, prefix) {
			return true
		}
	}
	return false
}
