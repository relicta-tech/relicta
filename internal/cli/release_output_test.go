package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestPrintSubtitleOutputsText(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printSubtitle("Release Preview")

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = r.Close()

	if !strings.Contains(buf.String(), "Release Preview") {
		t.Fatalf("expected subtitle output, got: %s", buf.String())
	}
}

func TestIsTerminalFalseForFile(t *testing.T) {
	old := os.Stdout
	tmpFile, err := os.CreateTemp("", "stdout")
	if err != nil {
		t.Fatalf("CreateTemp error: %v", err)
	}
	t.Cleanup(func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		os.Stdout = old
	})

	os.Stdout = tmpFile
	if isTerminal() {
		t.Fatal("expected isTerminal to be false for file-backed stdout")
	}
}
