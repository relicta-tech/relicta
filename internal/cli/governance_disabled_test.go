package cli

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func captureCommandOutput(t *testing.T, run func() error) (stdout, stderr string, runErr error) {
	t.Helper()

	oldStdout, oldStderr := os.Stdout, os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		stdoutReader.Close()
		stdoutWriter.Close()
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	t.Cleanup(func() {
		os.Stdout, os.Stderr = oldStdout, oldStderr
	})

	runErr = run()
	stdoutWriter.Close()
	stderrWriter.Close()
	os.Stdout, os.Stderr = oldStdout, oldStderr

	stdoutBytes, err := io.ReadAll(stdoutReader)
	stdoutReader.Close()
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	stderrBytes, err := io.ReadAll(stderrReader)
	stderrReader.Close()
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return string(stdoutBytes), string(stderrBytes), runErr
}

func TestGovernanceDisabledExplainsHowToEnableGovernance(t *testing.T) {
	gotStdout, gotStderr, gotErr := captureCommandOutput(t, governanceDisabled)

	if !errors.Is(gotErr, errGovernanceDisabled) {
		t.Fatalf("governanceDisabled() error = %v, want %v", gotErr, errGovernanceDisabled)
	}
	if gotStdout != "" {
		t.Fatalf("governanceDisabled() wrote %q to stdout, want no output", gotStdout)
	}
	for _, want := range []string{"governance:", "enabled: true", "relicta evaluate", "off by default"} {
		if !strings.Contains(gotStderr, want) {
			t.Errorf("stderr %q does not contain %q", gotStderr, want)
		}
	}
}

func TestGovernanceDisabledErrorIsSafeToWrap(t *testing.T) {
	_, _, gotErr := captureCommandOutput(t, governanceDisabled)
	wrapped := errors.Join(errors.New("release evaluation failed"), gotErr)
	if !errors.Is(wrapped, errGovernanceDisabled) {
		t.Fatalf("wrapped error %v does not preserve governance-disabled identity", wrapped)
	}
}
