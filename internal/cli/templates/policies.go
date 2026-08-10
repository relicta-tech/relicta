package templates

import (
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Policies ship inside the binary because governance has to be reachable from an
// installed relicta, not only from a git checkout.
//
// Five well-commented example policies existed in examples/policies/, and the
// documentation told people to `cp examples/policies/starter.policy
// .relicta/policies/`. Someone who installed relicta with `go install` or a
// release archive has no examples/ directory, so the only instruction for
// authoring a policy pointed at a path they did not have. `relicta policy list`
// named the directories to create files in and the extensions to use, but not
// the grammar that goes inside them — and the grammar is a DSL, not YAML, so
// there is nothing to guess correctly.
//
// Embedding them makes `relicta policy init` able to write a working policy on
// any machine that has the binary.
//
//go:embed data/policies/*.policy
var policyFiles embed.FS

const policyDataDir = "data/policies"

// PolicyStarter is an example policy that ships with the binary.
type PolicyStarter struct {
	// Name is the identifier used to select the starter, e.g. "starter".
	Name string
	// Filename is the name to write on disk, e.g. "starter.policy".
	Filename string
	// Description is a one-line summary of when to reach for this policy.
	Description string
	// Content is the policy source.
	Content string
}

// policyDescriptions keeps the summaries next to each other so `policy init
// --list` reads as a menu rather than five unrelated sentences. They mirror the
// table in examples/policies/README.md.
var policyDescriptions = map[string]string{
	"starter":     "Basic risk-based governance — a good first policy",
	"agent-aware": "Oversight rules for changes authored by AI agents",
	"enterprise":  "Comprehensive governance for regulated or critical systems",
	"team-based":  "Rules that vary by the team or owner making the change",
	"time-based":  "Release windows and change freezes",
}

// PolicyStarters returns every embedded policy, ordered so the one to start with
// comes first and the rest are alphabetical.
func PolicyStarters() ([]PolicyStarter, error) {
	entries, err := policyFiles.ReadDir(policyDataDir)
	if err != nil {
		return nil, fmt.Errorf("read embedded policies: %w", err)
	}

	starters := make([]PolicyStarter, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".policy") {
			continue
		}

		content, err := policyFiles.ReadFile(filepath.Join(policyDataDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read embedded policy %s: %w", entry.Name(), err)
		}

		name := strings.TrimSuffix(entry.Name(), ".policy")
		starters = append(starters, PolicyStarter{
			Name:        name,
			Filename:    entry.Name(),
			Description: policyDescriptions[name],
			Content:     string(content),
		})
	}

	sort.Slice(starters, func(i, j int) bool {
		// "starter" first: it is the answer to "which one do I pick", and a list
		// that opens with agent-aware makes the reader choose before they know
		// the vocabulary.
		if (starters[i].Name == DefaultPolicyStarter) != (starters[j].Name == DefaultPolicyStarter) {
			return starters[i].Name == DefaultPolicyStarter
		}
		return starters[i].Name < starters[j].Name
	})

	return starters, nil
}

// DefaultPolicyStarter is the policy `relicta policy init` writes when no
// template is named.
const DefaultPolicyStarter = "starter"

// PolicyStarterByName returns one embedded policy, naming the alternatives when
// the requested one does not exist — a bare "not found" leaves the caller with
// no way to discover the valid values.
func PolicyStarterByName(name string) (PolicyStarter, error) {
	starters, err := PolicyStarters()
	if err != nil {
		return PolicyStarter{}, err
	}

	available := make([]string, 0, len(starters))
	for _, s := range starters {
		if s.Name == name {
			return s, nil
		}
		available = append(available, s.Name)
	}

	return PolicyStarter{}, fmt.Errorf("unknown policy template %q; available: %s",
		name, strings.Join(available, ", "))
}
