package cli

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// captureStderrHygiene captures stderr for test assertions.
func captureStderrHygiene(fn func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	fn()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	os.Stderr = old
	return buf.String()
}

func TestReportError_WritesToStderr(t *testing.T) {
	// A non-nil error must surface on stderr so a non-zero exit is never silent.
	errOut := captureStderrHygiene(func() {
		ReportError(errors.New("boom happened"))
	})
	assert.Contains(t, errOut, "boom happened")

	// And it must NOT leak onto stdout, where machine consumers read --json.
	stdOut := captureStdoutCov(func() {
		ReportError(errors.New("boom happened"))
	})
	assert.Empty(t, stdOut)
}

func TestReportError_NilIsNoop(t *testing.T) {
	out := captureStderrHygiene(func() { ReportError(nil) })
	assert.Empty(t, out)
}

func TestPrintError_GoesToStderrNotStdout(t *testing.T) {
	stdOut := captureStdoutCov(func() { printError("diagnostic") })
	assert.Empty(t, stdOut, "errors must not pollute stdout")
}

func TestPrintErrorResult_GoesToStdout(t *testing.T) {
	// In-table failure rows are command output, so they belong on stdout.
	stdOut := captureStdoutCov(func() { printErrorResult("  push: auth failed") })
	assert.Contains(t, stdOut, "push: auth failed")
}

func TestSpinner_DisabledIsInert(t *testing.T) {
	// Under test (stderr is not a TTY) the spinner must emit nothing — no
	// cursor escapes that would leak as "[K" in redirected output.
	origJSON, origCI := outputJSON, ciMode
	defer func() { outputJSON, ciMode = origJSON, origCI }()
	outputJSON, ciMode = false, false

	out := captureStderrHygiene(func() {
		s := NewSpinner("working")
		assert.False(t, s.active, "spinner should be inert on a non-TTY stderr")
		s.Start()
		s.Stop()
	})
	assert.Empty(t, out)
}

func TestSpinnerEnabled_FalseInJSONOrCI(t *testing.T) {
	origJSON, origCI := outputJSON, ciMode
	defer func() { outputJSON, ciMode = origJSON, origCI }()

	outputJSON, ciMode = true, false
	assert.False(t, spinnerEnabled(), "--json must disable the spinner")

	outputJSON, ciMode = false, true
	assert.False(t, spinnerEnabled(), "CI mode must disable the spinner")
}
