package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v5/internal/domain/changes"
	"github.com/relicta-tech/relicta/v5/internal/domain/version"
	servicerelease "github.com/relicta-tech/relicta/v5/internal/service/release"
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

func TestPlanJSONReportsWorkingTreeScope(t *testing.T) {
	previousJSON := outputJSON
	outputJSON = true
	t.Cleanup(func() { outputJSON = previousJSON })

	output := &servicerelease.AnalyzeOutput{
		CurrentVersion: version.MustParse("1.0.0"),
		NextVersion:    version.MustParse("1.0.1"),
		ReleaseType:    changes.ReleaseTypePatch,
		ChangeSet:      changes.NewChangeSet("test", "v1.0.0", "HEAD"),
	}
	stdout := captureStdoutCov(func() {
		if err := outputPlanJSON(output, persistedRun{}, nil, true); err != nil {
			t.Errorf("outputPlanJSON: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON output %q: %v", stdout, err)
	}
	workingTree, ok := result["working_tree"].(map[string]any)
	if !ok {
		t.Fatalf("working_tree missing or invalid: %#v", result["working_tree"])
	}
	if workingTree["clean"] != false || workingTree["uncommitted_changes_included"] != false {
		t.Errorf("working_tree = %#v, want dirty and excluded", workingTree)
	}
}

// TestPrintWarningGoesToStderrInJSONMode covers the one helper that must keep
// speaking in JSON mode, just on a different stream.
//
// Dropping warnings entirely lost information that mattered: when governance
// evaluation failed during `relicta approve --ci`, a warning was the only sign
// the release was proceeding ungoverned, and JSON mode swallowed it.
func TestPrintWarningGoesToStderrInJSONMode(t *testing.T) {
	restore := outputJSON
	outputJSON = true
	t.Cleanup(func() { outputJSON = restore })

	if out := captureStdoutCov(func() { printWarning("governance evaluation failed") }); out != "" {
		t.Errorf("printWarning wrote %q to stdout, which must carry only the JSON document", out)
	}

	stderr := captureStderr(t, func() { printWarning("governance evaluation failed") })
	if !strings.Contains(stderr, "governance evaluation failed") {
		t.Errorf("printWarning must still report on stderr in JSON mode; got %q", stderr)
	}
}

// captureStderr swaps os.Stderr for the duration of fn.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read: %v", err)
	}
	return buf.String()
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
