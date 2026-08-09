package cli

import (
	"errors"
	"fmt"
	"os"
)

// errGovernanceDisabled is returned by the commands that need the Change
// Governance Protocol when it is switched off.
//
// Short and lowercase because it gets wrapped; the remedy lives in
// governanceEnableHint and is written to stderr by governanceDisabled(). Putting
// the YAML in the error string itself is what the linter objects to, and rightly:
// "failed to evaluate: governance is not enabled.\n\nAdd this to..." reads badly
// once something wraps it.
var errGovernanceDisabled = errors.New("governance is not enabled")

// governanceEnableHint tells the reader what to add, rather than naming a key.
//
// `relicta init` writes 6 of the schema's 21 top-level sections and governance is
// not among them, so the previous message — "enable governance in
// .relicta.yaml" — sent people looking for a setting that was not in the file,
// and left them to guess the nesting. Three commands said this three different
// ways, none of them actionable. Verified end to end: adding exactly this makes
// `relicta evaluate` work.
const governanceEnableHint = `
Add this to .relicta.yaml:

  governance:
    enabled: true

Governance computes the risk score and policy verdict used by 'relicta evaluate',
'relicta approve' and 'relicta analytics'. It is off by default because it changes
whether a release can be approved automatically.`

// governanceDisabled writes the remedy to stderr and returns the error.
//
// stderr keeps the hint off stdout, which may be carrying a JSON document, and
// means it still appears when a caller is parsing output.
func governanceDisabled() error {
	fmt.Fprintln(os.Stderr, governanceEnableHint)
	return errGovernanceDisabled
}
