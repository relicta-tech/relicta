// Package cli provides the command-line interface for Relicta.
package cli

import (
	"fmt"
	"os"
)

// The config hints.
//
// `relicta init` writes 7 of the schema's 21 top-level sections, which is deliberate: all
// 21 would be several hundred lines of advanced settings at their defaults, and the current
// file is short enough to read. The cost is that a message telling someone to configure a
// setting frequently names a section their file does not contain — they open it, the section
// is absent, and nothing says what nesting to add.
//
// The rule adopted instead of writing every section: a message that names a config key
// carries the YAML to add. errGovernanceDisabled established the shape and this generalizes
// it, so the three remaining cases stop being three different half-answers.
// TestEveryConfigKeyMentionedHasSomewhereToLook enforces it.

// configHint is a remedy the user can paste, printed to stderr.
//
// stderr rather than stdout because stdout may be carrying a JSON document, and because the
// hint should still appear when a caller is parsing output.
type configHint struct {
	// what the reader is trying to achieve, one line.
	purpose string
	// yaml is the block to add, written to paste at the top level of .relicta.yaml.
	//
	// Not indented. These blocks used to carry a two-space indent, which made them work in
	// exactly one situation — an empty config file — and silently mis-nest in the one they
	// are printed for. A hint appears because you already have a config, so pasting an
	// indented `repository_groups:` after an existing `ai:` block made it a child of ai,
	// and the command that printed the hint then reported "no repository groups are
	// declared" to someone who had just followed its instructions. Verified against the
	// shipped binary, both ways round.
	//
	// Every field these hints name is top level. A hint for a nested setting must include
	// its parent key, as the dashboard one does, so that the block is still complete on its
	// own.
	yaml string
	// note is optional context — why the setting is off by default, what it changes.
	note string
}

func (h configHint) String() string {
	out := "\n" + h.purpose + "\n\nAdd this to .relicta.yaml:\n\n" + h.yaml + "\n"
	if h.note != "" {
		out += "\n" + h.note + "\n"
	}
	return out
}

// print writes the hint to stderr.
func (h configHint) print() {
	fmt.Fprint(os.Stderr, h.String())
}

// hintEnvironments is the remedy for a deployment naming an undeclared environment.
//
// Deployments are refused rather than recorded under a free-form name, because "prod",
// "production" and "Production" would become three environments each holding part of the
// audit history, with nothing reporting it as wrong.
var hintEnvironments = configHint{
	purpose: "Deployment environments have to be declared before a deployment can name one.",
	yaml: `environments:
  - name: staging
  - name: production
    production: true`,
	note: `Exactly one environment marked 'production: true' is what DORA deployment
frequency and lead time measure against — without it, every environment a version
passes through counts as a deployment.`,
}

// hintDashboardAuth is the remedy for running the dashboard in API-key mode with no keys.
var hintDashboardAuth = configHint{
	purpose: "The dashboard is in API-key mode with no keys configured, so every request will be refused.",
	yaml: `dashboard:
  auth:
    mode: api_key
    api_keys:
      - name: ci
        key: "${RELICTA_DASHBOARD_KEY}"
        roles: [viewer]`,
	note: `Each key is an entry with its own name and value, not a bare string — a plain
list of strings fails to load. name appears in the audit log, and roles is 'viewer'
or 'admin', defaulting to viewer. The value expands from the environment, so the key
itself need not be committed.

Pass --api-key to supply one for a single run instead. Use --no-auth only on a
loopback address: it serves this repository's governance record to anyone who can
reach the port.`,
}

// hintRepositoryGroups is the remedy for group commands with no groups declared.
var hintRepositoryGroups = configHint{
	purpose: "Repository groups have to be declared before a group command can act on one.",
	yaml: `repository_groups:
  - name: platform
    strategy: coordinated
    repositories:
      - name: service-a
        path: ../service-a
      - name: service-b
        path: ../service-b
        dependencies: [service-a]`,
	note: `Each repository is an entry with its own name and local path, not a bare
string — 'repositories: [owner/repo]' fails to load. strategy is 'independent' to
release each separately, or 'coordinated' to release in dependency order.`,
}
