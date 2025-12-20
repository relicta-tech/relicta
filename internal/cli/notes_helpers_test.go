package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apprelease "github.com/relicta-tech/relicta/internal/application/release"
	"github.com/relicta-tech/relicta/internal/domain/communication"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

func captureNotesStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}
	_ = r.Close()
	os.Stdout = old

	return buf.String()
}

func TestParseNoteToneDefaults(t *testing.T) {
	if got := parseNoteTone(""); got != communication.ToneProfessional {
		t.Fatalf("unexpected default tone: %v", got)
	}
	if got := parseNoteTone("friendly"); got != communication.ToneFriendly {
		t.Fatalf("unexpected tone: %v", got)
	}
}

func TestParseNoteAudienceDefaults(t *testing.T) {
	if got := parseNoteAudience(""); got != communication.AudienceDevelopers {
		t.Fatalf("unexpected default audience: %v", got)
	}
	if got := parseNoteAudience("public"); got != communication.AudiencePublic {
		t.Fatalf("unexpected audience: %v", got)
	}
}

func TestWriteNotesToFileCreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "notes.md")

	output := &apprelease.GenerateNotesOutput{
		ReleaseNotes: communication.NewReleaseNotesBuilder(version.MustParse("0.1.0")).
			WithSummary("notes").
			Build(),
	}

	if err := writeNotesToFile(output, path); err != nil {
		t.Fatalf("writeNotesToFile failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if !strings.Contains(string(data), "notes") {
		t.Fatalf("unexpected contents: %q", data)
	}
}

func TestOutputNotesToStdoutAndNextSteps(t *testing.T) {
	output := &apprelease.GenerateNotesOutput{
		Changelog: communication.NewChangelog("Changelog", communication.FormatKeepAChangelog),
		ReleaseNotes: communication.NewReleaseNotesBuilder(version.MustParse("0.1.0")).
			WithSummary("relnotes").
			Build(),
	}

	notesOutput = ""
	defer func() { notesOutput = "" }()

	notesTone = ""
	notesAudience = ""

	stdout := captureNotesStdout(t, func() {
		outputNotesToStdout(output)
		printNotesNextSteps()
	})

	if !strings.Contains(stdout, "Release Notes") {
		t.Fatalf("expected release notes block, got %q", stdout)
	}
	if !strings.Contains(stdout, "Run 'relicta approve'") {
		t.Fatalf("expected next steps, got %q", stdout)
	}
}

func TestOutputNotesJSON(t *testing.T) {
	tmp := newTestRelease(t, "notes-json")
	output := &apprelease.GenerateNotesOutput{
		ReleaseNotes: communication.NewReleaseNotesBuilder(version.MustParse("0.1.0")).
			WithSummary("notes-json").
			Build(),
		Changelog: communication.NewChangelog("Changelog", communication.FormatKeepAChangelog),
	}

	stdout := captureNotesStdout(t, func() {
		if err := outputNotesJSON(output, tmp); err != nil {
			t.Fatalf("outputNotesJSON failed: %v", err)
		}
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}
	if parsed["release_id"] != string(tmp.ID()) {
		t.Fatalf("unexpected release_id: %v", parsed["release_id"])
	}
	if parsed["state"] != string(tmp.State()) {
		t.Fatalf("unexpected state: %v", parsed["state"])
	}
}
