package templates

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp/policy/dsl"
)

// These assertions used to live in internal/cgp/policy/dsl/examples_test.go,
// where they parsed the .policy files in examples/. That checked a directory
// users of a released binary do not have. The same checks run here against the
// copies that ship inside the binary, which is what `relicta policy init`
// actually writes — so a starter that does not compile now fails the build
// instead of failing on someone's first attempt at governance.

func TestEmbeddedPoliciesCompile(t *testing.T) {
	starters, err := PolicyStarters()
	if err != nil {
		t.Fatalf("PolicyStarters: %v", err)
	}
	if len(starters) == 0 {
		t.Fatal("no policies are embedded; `relicta policy init` would have nothing to write")
	}

	loader := dsl.NewLoader(dsl.LoaderOptions{})

	for _, s := range starters {
		t.Run(s.Name, func(t *testing.T) {
			pol, err := loader.LoadString(s.Content, s.Filename)
			if err != nil {
				t.Fatalf("embedded policy %s does not parse: %v", s.Filename, err)
			}
			if len(pol.Rules) == 0 {
				t.Errorf("%s has no rules, so it governs nothing", s.Filename)
			}
			for _, rule := range pol.Rules {
				if rule.ID == "" {
					t.Errorf("%s: a rule has no ID", s.Filename)
				}
				if rule.Name == "" {
					t.Errorf("%s: a rule has no name", s.Filename)
				}
				if len(rule.Conditions) == 0 {
					t.Errorf("%s: rule %q has no conditions, so it matches nothing",
						s.Filename, rule.Name)
				}
			}
			// Every starter is also a teaching document — it is the only place a
			// user sees the DSL before writing their own.
			if s.Description == "" {
				t.Errorf("%s has no description; `policy init --list` would show a blank row", s.Filename)
			}
		})
	}
}

// The embedded copies and the ones in examples/ are the same files in two
// places: examples/ is browsable on GitHub and linked from the docs, and the
// embedded set is what an installed binary can reach. Duplication is the
// tradeoff for both being available; this test is what keeps it honest, so an
// edit to one cannot silently leave the other stale.
func TestEmbeddedPoliciesMatchTheDocumentedExamples(t *testing.T) {
	examplesDir := filepath.Join("..", "..", "..", "examples", "policies")
	if _, err := os.Stat(examplesDir); os.IsNotExist(err) {
		t.Skip("examples/policies not present (running outside a source checkout)")
	}

	starters, err := PolicyStarters()
	if err != nil {
		t.Fatalf("PolicyStarters: %v", err)
	}

	for _, s := range starters {
		onDisk, err := os.ReadFile(filepath.Join(examplesDir, s.Filename))
		if err != nil {
			t.Errorf("%s is embedded but missing from examples/policies: %v", s.Filename, err)
			continue
		}
		if string(onDisk) != s.Content {
			t.Errorf("examples/policies/%s and the embedded copy have diverged; "+
				"update both (the embedded copy is what `relicta policy init` writes)", s.Filename)
		}
	}

	// The other direction: an example added to examples/ and not embedded is
	// invisible to anyone using a released binary.
	onDisk, err := filepath.Glob(filepath.Join(examplesDir, "*.policy"))
	if err != nil {
		t.Fatalf("glob examples: %v", err)
	}
	for _, path := range onDisk {
		name := filepath.Base(path)
		found := false
		for _, s := range starters {
			if s.Filename == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("examples/policies/%s is not embedded, so `relicta policy init` "+
				"cannot offer it to anyone who installed the binary", name)
		}
	}
}
