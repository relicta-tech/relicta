package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/cgp/policy/library"
)

// The built-in policy templates.
//
// `internal/cgp/policy/library` ships SOC 2 compliance, separation of duties, multi-team
// approval, audit trail, hotfix fast track and the rest — complete, tested, and reachable from
// nothing: no command listed them, no configuration named them, and the package had no importer.
//
// Read-only on purpose, and less than it first appears: policy files are written in the DSL,
// which is parse-only — internal/cgp/policy/dsl has a lexer, a parser and a compiler, and no
// writer. A template is a Go value, so there is no way to emit it as a `.policy` file that the
// loader would read.
//
// So --show prints a rendering of what a template contains, and says so. The first draft of
// this command told the reader to "save it under your policy_dir to put it into effect", which
// would have been false: `relicta policy validate` does not even see a .json file there. That
// is the defect this whole stretch of work has been removing, nearly shipped by the person
// removing it.
//
// Installing a template needs a DSL serializer. That is a real piece of work with a design
// question inside it — whether the DSL round-trips — and it is recorded rather than guessed at.

var policyTemplateShow string

var policyTemplatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "List the built-in policy templates",
	Long: `List the governance policy templates relicta ships.

Templates are starting points, not active policy: nothing here changes how a release is
evaluated. --show prints what a template contains, as a reference for writing your own
policy — it is not a file relicta can load, because policy files are written in the DSL
and there is no writer for it yet.

Examples:
  # See what is available
  relicta policy templates

  # Read what one contains
  relicta policy templates --show enterprise-soc2`,
	RunE: runPolicyTemplates,
}

func init() {
	policyTemplatesCmd.Flags().StringVar(&policyTemplateShow, "show", "",
		"print one template's policy as JSON, by ID")
	policyCmd.AddCommand(policyTemplatesCmd)
}

func runPolicyTemplates(_ *cobra.Command, _ []string) error {
	// The package's own registry, populated in its init. NewRegistry starts empty, which is
	// for callers assembling their own set.
	registry := library.DefaultRegistry

	if policyTemplateShow != "" {
		return showPolicyTemplate(registry, policyTemplateShow)
	}

	templates := registry.List()
	sort.Slice(templates, func(i, j int) bool {
		if templates[i].Category != templates[j].Category {
			return templates[i].Category < templates[j].Category
		}
		return templates[i].ID < templates[j].ID
	})

	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(templatesAsJSON(templates))
	}

	printTitle("Policy Templates")
	fmt.Println()

	category := ""
	for _, t := range templates {
		if t.Category != category {
			category = t.Category
			fmt.Printf("\n  %s\n", category)
		}
		fmt.Printf("    %-24s %s\n", t.ID, t.Name)
		if t.Description != "" {
			printSubtle("      " + t.Description)
		}
	}

	fmt.Println()
	printInfo("Starting points, not active policy. `--show <id>` prints what one contains; " +
		"writing it as a .policy file in your policy_dir is still by hand")
	return nil
}

// showPolicyTemplate prints one template's policy, built with the defaults.
func showPolicyTemplate(registry *library.Registry, id string) error {
	built, err := registry.Build(id, library.DefaultTemplateOptions())
	if err != nil {
		known := make([]string, 0, len(registry.List()))
		for _, t := range registry.List() {
			known = append(known, t.ID)
		}
		sort.Strings(known)
		return fmt.Errorf("%w (available: %v)", err, known)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(built)
}

// templatesAsJSON is the machine-readable listing: what a template is, not what it builds.
func templatesAsJSON(templates []*library.PolicyTemplate) []map[string]any {
	out := make([]map[string]any, 0, len(templates))
	for _, t := range templates {
		out = append(out, map[string]any{
			"id":          t.ID,
			"name":        t.Name,
			"description": t.Description,
			"category":    t.Category,
			"tags":        t.Tags,
		})
	}
	return out
}
