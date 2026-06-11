package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/security/attestation"
)

// TestResolveAttestationPath_FromFile returns the file directly when --file is set.
func TestResolveAttestationPath_FromFile(t *testing.T) {
	origFile := verifyFile
	defer func() { verifyFile = origFile }()

	verifyFile = "/some/explicit/attestation.jsonl"

	path, err := resolveAttestationPath()
	if err != nil {
		t.Fatalf("resolveAttestationPath() error = %v", err)
	}
	if path != "/some/explicit/attestation.jsonl" {
		t.Errorf("path = %v, want /some/explicit/attestation.jsonl", path)
	}
}

// TestResolveAttestationPath_FromRunID finds attestation in standard location.
func TestResolveAttestationPath_FromRunID(t *testing.T) {
	origRunID := verifyRunID
	origFile := verifyFile
	origWD, _ := os.Getwd()
	defer func() {
		verifyRunID = origRunID
		verifyFile = origFile
		os.Chdir(origWD)
	}()

	// Create a temporary directory simulating the .relicta/releases/<runID> structure.
	tmpDir := t.TempDir()
	runID := "test-run-123"
	attDir := filepath.Join(tmpDir, ".relicta", "releases", runID)
	if err := os.MkdirAll(attDir, 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}
	attFile := filepath.Join(attDir, "attestation.intoto.jsonl")
	if err := os.WriteFile(attFile, []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Change to the tmpDir so resolveAttestationPath can use os.Getwd().
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	verifyFile = ""
	verifyRunID = runID

	path, err := resolveAttestationPath()
	if err != nil {
		t.Fatalf("resolveAttestationPath() error = %v", err)
	}
	// Compare filename only since the path computation uses Getwd() inside.
	if filepath.Base(filepath.Dir(path)) != runID {
		t.Errorf("path %v should be under run dir %v", path, runID)
	}
}

// TestResolveAttestationPath_RunIDNotFound returns error when file doesn't exist.
func TestResolveAttestationPath_RunIDNotFound(t *testing.T) {
	origRunID := verifyRunID
	origFile := verifyFile
	defer func() {
		verifyRunID = origRunID
		verifyFile = origFile
	}()

	tmpDir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer os.Chdir(origWD)

	verifyFile = ""
	verifyRunID = "nonexistent-run"

	_, err := resolveAttestationPath()
	if err == nil {
		t.Error("expected error for nonexistent run ID")
	}
}

// TestResolveAttestationPath_NoReleasesDir returns error when releases dir doesn't exist.
func TestResolveAttestationPath_NoReleasesDir(t *testing.T) {
	origRunID := verifyRunID
	origFile := verifyFile
	defer func() {
		verifyRunID = origRunID
		verifyFile = origFile
	}()

	tmpDir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer os.Chdir(origWD)

	verifyFile = ""
	verifyRunID = ""

	_, err := resolveAttestationPath()
	if err == nil {
		t.Error("expected error when no releases directory exists")
	}
}

// TestResolveAttestationPath_FindsLatest finds the most recent attestation without run-id.
func TestResolveAttestationPath_FindsLatest(t *testing.T) {
	origRunID := verifyRunID
	origFile := verifyFile
	origWD, _ := os.Getwd()
	defer func() {
		verifyRunID = origRunID
		verifyFile = origFile
		os.Chdir(origWD)
	}()

	tmpDir := t.TempDir()
	// Create two run directories; the last one alphabetically should be found.
	for _, runID := range []string{"run-aaa", "run-bbb"} {
		runDir := filepath.Join(tmpDir, ".relicta", "releases", runID)
		if err := os.MkdirAll(runDir, 0o755); err != nil {
			t.Fatalf("mkdir error: %v", err)
		}
		attFile := filepath.Join(runDir, "attestation.intoto.jsonl")
		if err := os.WriteFile(attFile, []byte("{}"), 0o644); err != nil {
			t.Fatalf("writefile error: %v", err)
		}
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir error: %v", err)
	}

	verifyFile = ""
	verifyRunID = ""

	path, err := resolveAttestationPath()
	if err != nil {
		t.Fatalf("resolveAttestationPath() error = %v", err)
	}
	// Should find run-bbb (last alphabetically, walked in reverse).
	if filepath.Base(filepath.Dir(path)) != "run-bbb" {
		t.Errorf("expected run-bbb directory, got %v", filepath.Dir(path))
	}
}

// TestLoadAttestation_ValidJSON returns attestation from a valid JSON file.
func TestLoadAttestation_ValidJSON(t *testing.T) {
	att := attestation.SignedAttestation{
		PayloadType: "application/vnd.in-toto+json",
		Payload:     []byte(`{"_type":"test"}`),
	}
	data, err := json.Marshal(att)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	tmpFile := filepath.Join(t.TempDir(), "attestation.intoto.jsonl")
	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		t.Fatalf("writefile error: %v", err)
	}

	result, err := loadAttestation(tmpFile)
	if err != nil {
		t.Fatalf("loadAttestation() error = %v", err)
	}
	if result.PayloadType != "application/vnd.in-toto+json" {
		t.Errorf("PayloadType = %v, want application/vnd.in-toto+json", result.PayloadType)
	}
}

// TestLoadAttestation_InvalidJSON returns error for malformed JSON.
func TestLoadAttestation_InvalidJSON(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "attestation.intoto.jsonl")
	if err := os.WriteFile(tmpFile, []byte("not-json{{{}"), 0o644); err != nil {
		t.Fatalf("writefile error: %v", err)
	}

	_, err := loadAttestation(tmpFile)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestLoadAttestation_FileNotFound returns error for missing file.
func TestLoadAttestation_FileNotFound(t *testing.T) {
	_, err := loadAttestation("/nonexistent/path/attestation.jsonl")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// TestOutputVerifyJSON writes JSON output to stdout.
func TestOutputVerifyJSON(t *testing.T) {
	output := &VerifyOutput{
		Valid:      true,
		File:       "test.jsonl",
		Version:    "1.0.0",
		RiskScore:  0.3,
		Governance: "passed",
	}

	// Capture stdout.
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputVerifyJSON(output)

	w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("outputVerifyJSON() error = %v", err)
	}

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	got := string(buf[:n])

	var decoded VerifyOutput
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("failed to decode JSON output: %v", err)
	}
	if !decoded.Valid {
		t.Error("decoded Valid should be true")
	}
	if decoded.File != "test.jsonl" {
		t.Errorf("decoded File = %v, want test.jsonl", decoded.File)
	}
}

// TestOutputVerifyText_Valid tests text output for a valid attestation.
func TestOutputVerifyText_Valid(t *testing.T) {
	output := &VerifyOutput{
		Valid:      true,
		File:       "test.jsonl",
		Version:    "1.0.0",
		Decision:   "approved",
		RiskScore:  0.3,
		Approvals:  2,
		Repository: "owner/repo",
		CommitSHA:  "abc123",
		Governance: "passed",
	}

	// Should not return error for valid attestation.
	err := outputVerifyText(output)
	if err != nil {
		t.Errorf("outputVerifyText() error = %v", err)
	}
}

// TestOutputVerifyText_WithError tests text output when verification failed.
func TestOutputVerifyText_WithError(t *testing.T) {
	output := &VerifyOutput{
		Valid:       false,
		File:        "test.jsonl",
		ErrorDetail: "signature verification failed",
	}

	err := outputVerifyText(output)
	if err == nil {
		t.Error("expected error for failed verification")
	}
}

// TestOutputVerifyText_GovernanceFailed tests text output when governance fails.
func TestOutputVerifyText_GovernanceFailed(t *testing.T) {
	output := &VerifyOutput{
		Valid:      true,
		File:       "test.jsonl",
		Governance: "failed",
		GovError:   "risk score 0.8 exceeds max 0.5",
	}

	err := outputVerifyText(output)
	if err == nil {
		t.Error("expected error for governance failure")
	}
}

// TestOutputVerifyText_Unsigned tests text output for unsigned attestation.
func TestOutputVerifyText_Unsigned(t *testing.T) {
	output := &VerifyOutput{
		Valid:    true,
		Unsigned: true,
		File:     "test.jsonl",
	}

	err := outputVerifyText(output)
	if err != nil {
		t.Errorf("outputVerifyText() for unsigned error = %v", err)
	}
}

// TestOutputVerifyText_SignedBy tests text output when signed by known entity.
func TestOutputVerifyText_SignedBy(t *testing.T) {
	output := &VerifyOutput{
		Valid:    true,
		SignedBy: "alice@example.com",
		File:     "test.jsonl",
	}

	err := outputVerifyText(output)
	if err != nil {
		t.Errorf("outputVerifyText() error = %v", err)
	}
}

// TestOutputVerifyResult_JSON routes to JSON when outputJSON is true.
func TestOutputVerifyResult_JSON(t *testing.T) {
	origOutputJSON := outputJSON
	outputJSON = true
	defer func() { outputJSON = origOutputJSON }()

	// Capture stdout to avoid noise.
	origStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		w.Close()
		os.Stdout = origStdout
	}()

	output := &VerifyOutput{Valid: true, File: "test.jsonl"}
	err := outputVerifyResult(output)
	if err != nil {
		t.Errorf("outputVerifyResult() error = %v", err)
	}
}

// TestOutputVerifyResult_Text routes to text when outputJSON is false.
func TestOutputVerifyResult_Text(t *testing.T) {
	origOutputJSON := outputJSON
	outputJSON = false
	defer func() { outputJSON = origOutputJSON }()

	output := &VerifyOutput{Valid: true, File: "test.jsonl"}
	err := outputVerifyResult(output)
	if err != nil {
		t.Errorf("outputVerifyResult() error = %v", err)
	}
}
