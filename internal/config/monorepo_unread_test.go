package config

import (
	"strings"
	"testing"
)

// The monorepo section is the largest unread block in the configuration, and the subsystem behind
// it is the largest unreached code in the tree: every field has zero production readers, and
// internal/domain/monorepo plus internal/application/monorepo hold roughly 3,000 lines of
// implemented, tested code that nothing in the release path calls.
//
// Verified against the built binary before this warning existed. A repository with
// `enabled: true`, `strategy: independent`, `package_paths: ["packages/*"]` and two packages at
// 1.0.0 and 2.0.0 was given one repository-wide version:
//
//	Current version:  0.0.0
//	Next version:     0.1.0
//
// and neither package.json was touched.

func TestAnEnabledMonorepoSectionSaysItDoesNothing(t *testing.T) {
	warnings := warningsFor(t, func(c *Config) { c.Monorepo.Enabled = true })

	var found string
	for _, w := range warnings {
		if strings.Contains(w, "monorepo") {
			found = w
		}
	}
	if found == "" {
		t.Fatalf("no warning for a section nothing reads; warnings were %v.\nA monorepo user "+
			"configures independent per-package versioning and gets one version for the whole "+
			"repository, with no indication that the setting was ignored", warnings)
	}

	// The distinction a reader needs: analysis works, versioning does not. Without it the
	// warning reads as "relicta does not understand monorepos", which is not true.
	if !strings.Contains(found, "blast") {
		t.Errorf("the warning is %q but does not say that `relicta blast` does read packages, "+
			"so it overstates what is missing", found)
	}
}

// A repository that is not a monorepo must hear nothing.
func TestADisabledMonorepoSectionIsSilent(t *testing.T) {
	for _, w := range warningsFor(t, func(*Config) {}) {
		if strings.Contains(w, "monorepo") {
			t.Errorf("warned about monorepo in a repository that did not enable it: %q", w)
		}
	}
}

// A warning, not an error: the project is doing something ineffective, not something invalid.
func TestTheMonorepoSectionDoesNotFailTheRelease(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Monorepo.Enabled = true

	if err := NewValidator().Validate(cfg); err != nil {
		t.Errorf("enabling monorepo fails validation: %v.\nIt has no effect, which is worth "+
			"saying and not worth refusing a release over", err)
	}
}
