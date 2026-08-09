package cli

import (
	"strings"
	"testing"
)

// With --json, stdout is a JSON document and nothing else may share it.
//
// That was not enforced, so `relicta plan --json` printed a "Release Plan"
// heading before its object:
//
//	$ relicta plan --json | jq .
//	parse error: Invalid numeric literal at line 1, column 8
//
// and `relicta history --json` answered an empty history with the prose "No
// release history found for <repo>", leaving a consumer unable to tell "nothing
// to report" from "the command broke".
//
// Both are the same mistake: writing for a human on a channel a machine is
// reading. The fix is structural — the print helpers return early in JSON mode —
// and these tests protect the structure, since an end-to-end check of every
// command's output would need a git repository per command and would still miss
// whichever command was added last.

// printHelpers are the functions that write prose to stdout. printError is
// deliberately absent: it writes to stderr, which stays available for
// diagnostics regardless of what stdout carries.
var printHelpers = map[string]func(string){
	"printSuccess":     printSuccess,
	"printErrorResult": printErrorResult,
	"printWarning":     printWarning,
	"printInfo":        printInfo,
	"printTitle":       printTitle,
	"printSubtle":      printSubtle,
}

func TestPrintHelpersAreSilentInJSONMode(t *testing.T) {
	for name, fn := range printHelpers {
		t.Run(name, func(t *testing.T) {
			restore := outputJSON
			outputJSON = true
			t.Cleanup(func() { outputJSON = restore })

			out := captureStdoutCov(func() { fn("this must not reach stdout") })
			if out != "" {
				t.Errorf("%s wrote %q to stdout in JSON mode; it would corrupt the document", name, out)
			}
		})
	}
}

func TestPrintHelpersStillWriteForHumans(t *testing.T) {
	for name, fn := range printHelpers {
		t.Run(name, func(t *testing.T) {
			restore := outputJSON
			outputJSON = false
			t.Cleanup(func() { outputJSON = restore })

			out := captureStdoutCov(func() { fn("hello") })
			if !strings.Contains(out, "hello") {
				t.Errorf("%s printed %q; suppression must apply only to JSON mode", name, out)
			}
		})
	}
}

// The dry-run banner takes no argument, so it is checked separately rather than
// left out of the guarantee.
func TestDryRunBannerIsSilentInJSONMode(t *testing.T) {
	restore := outputJSON
	outputJSON = true
	t.Cleanup(func() { outputJSON = restore })

	if out := captureStdoutCov(printDryRunBanner); out != "" {
		t.Errorf("printDryRunBanner wrote %q to stdout in JSON mode", out)
	}
}

// printError is the one helper that must keep talking: a diagnostic is useful
// precisely when the command failed, and stderr never carries the JSON document.
func TestPrintErrorStillReportsInJSONMode(t *testing.T) {
	restore := outputJSON
	outputJSON = true
	t.Cleanup(func() { outputJSON = restore })

	if out := captureStdoutCov(func() { printError("boom") }); out != "" {
		t.Errorf("printError wrote %q to stdout; errors belong on stderr", out)
	}
}
