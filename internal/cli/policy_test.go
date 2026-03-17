package cli

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
)

func TestPolicyValidateCmd_NoFiles(t *testing.T) {
	// Create temp directory with no policy files
	tmpDir := t.TempDir()

	// Save and restore flags
	oldDir := policyValidateDir
	oldFile := policyValidateFile
	defer func() {
		policyValidateDir = oldDir
		policyValidateFile = oldFile
	}()

	policyValidateDir = tmpDir
	policyValidateFile = ""

	err := runPolicyValidate(policyValidateCmd, nil)
	if err != nil {
		t.Errorf("expected no error for empty directory, got: %v", err)
	}
}

func TestPolicyValidateCmd_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid policy file using correct DSL syntax
	policyContent := `
rule "check-risk" {
    priority = 100
    description = "Check for high risk"

    when {
        risk_score > 0.5
    }

    then {
        require_approval(role: "reviewer")
    }
}
`
	policyPath := filepath.Join(tmpDir, "test.policy")
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}

	// Save and restore flags
	oldDir := policyValidateDir
	oldFile := policyValidateFile
	defer func() {
		policyValidateDir = oldDir
		policyValidateFile = oldFile
	}()

	policyValidateDir = ""
	policyValidateFile = policyPath

	err := runPolicyValidate(policyValidateCmd, nil)
	if err != nil {
		t.Errorf("expected no error for valid policy, got: %v", err)
	}
}

func TestPolicyValidateCmd_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invalid policy file (syntax error)
	policyContent := `
rule "broken" {
    this is clearly not valid syntax at all
    and neither is this line
}
`
	policyPath := filepath.Join(tmpDir, "invalid.policy")
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}

	// Save and restore flags
	oldDir := policyValidateDir
	oldFile := policyValidateFile
	defer func() {
		policyValidateDir = oldDir
		policyValidateFile = oldFile
	}()

	policyValidateDir = ""
	policyValidateFile = policyPath

	err := runPolicyValidate(policyValidateCmd, nil)
	if err == nil {
		t.Error("expected error for invalid policy, got nil")
	}
}

func TestPolicyValidateCmd_FileNotFound(t *testing.T) {
	// Save and restore flags
	oldDir := policyValidateDir
	oldFile := policyValidateFile
	defer func() {
		policyValidateDir = oldDir
		policyValidateFile = oldFile
	}()

	policyValidateDir = ""
	policyValidateFile = "/nonexistent/path/policy.policy"

	err := runPolicyValidate(policyValidateCmd, nil)
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestPolicyValidateCmd_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple valid policy files
	policies := []struct {
		name    string
		content string
	}{
		{
			name: "security.policy",
			content: `
rule "auth-check" {
    description = "Review auth changes"
    when { scope == "auth" }
    then { require_approval(role: "security") }
}
`,
		},
		{
			name: "risk.policy",
			content: `
rule "high-risk" {
    when { risk_score > 0.8 }
    then { block(reason: "Too risky") }
}
`,
		},
	}

	for _, p := range policies {
		path := filepath.Join(tmpDir, p.name)
		if err := os.WriteFile(path, []byte(p.content), 0o644); err != nil {
			t.Fatalf("failed to write policy file %s: %v", p.name, err)
		}
	}

	// Save and restore flags
	oldDir := policyValidateDir
	oldFile := policyValidateFile
	defer func() {
		policyValidateDir = oldDir
		policyValidateFile = oldFile
	}()

	policyValidateDir = tmpDir
	policyValidateFile = ""

	err := runPolicyValidate(policyValidateCmd, nil)
	if err != nil {
		t.Errorf("expected no error for valid policies, got: %v", err)
	}
}

func TestPolicyListCmd_NoFiles(t *testing.T) {
	// This test just verifies it doesn't panic with no files
	// The actual output goes to stdout
	err := runPolicyList(policyListCmd, nil)
	if err != nil {
		t.Errorf("expected no error for empty list, got: %v", err)
	}
}

func TestValidatePolicyFile_Valid(t *testing.T) {
	tmpDir := t.TempDir()

	policyContent := `
rule "simple" {
    when { risk_score > 0 }
    then { approve() }
}
`
	policyPath := filepath.Join(tmpDir, "simple.policy")
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}

	err := validatePolicyFile(policyPath)
	if err != nil {
		t.Errorf("expected no error for valid policy, got: %v", err)
	}
}

func TestValidatePolicyFile_NotFound(t *testing.T) {
	err := validatePolicyFile("/nonexistent/path/test.policy")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestValidatePolicyFile_Invalid(t *testing.T) {
	tmpDir := t.TempDir()

	// Malformed policy (syntax error)
	policyContent := `
rule "broken" {
    this is not valid syntax
}
`
	policyPath := filepath.Join(tmpDir, "broken.policy")
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}

	err := validatePolicyFile(policyPath)
	if err == nil {
		t.Error("expected error for invalid policy, got nil")
	}
}

func TestValidatePolicyFile_MultipleRules(t *testing.T) {
	tmpDir := t.TempDir()

	policyContent := `
rule "rule-1" {
    priority = 100
    description = "First rule"
    when { risk_score > 0.9 }
    then { block(reason: "Critical risk") }
}

rule "rule-2" {
    priority = 50
    description = "Second rule"
    when { has_breaking_changes == true }
    then { require_approval(role: "tech-lead") }
}

rule "rule-3" {
    when { commit_count > 10 }
    then { require_approval(role: "reviewer") }
}
`
	policyPath := filepath.Join(tmpDir, "multi.policy")
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}

	err := validatePolicyFile(policyPath)
	if err != nil {
		t.Errorf("expected no error for valid policy with multiple rules, got: %v", err)
	}
}

func TestPolicyTestCmd_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	content := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRisk := policyTestRiskScore
	oldRequireApproved := policyTestRequireApproved
	oldBump := policyTestBumpType
	oldActorType := policyTestActorType
	oldActorID := policyTestActorID
	oldRepo := policyTestRepository
	oldBranch := policyTestBranch
	oldJSON := outputJSON
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRiskScore = oldRisk
		policyTestRequireApproved = oldRequireApproved
		policyTestBumpType = oldBump
		policyTestActorType = oldActorType
		policyTestActorID = oldActorID
		policyTestRepository = oldRepo
		policyTestBranch = oldBranch
		outputJSON = oldJSON
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = ""
	policyTestRiskScore = 0.9
	policyTestRequireApproved = false
	policyTestBumpType = "major"
	policyTestActorType = "human"
	policyTestActorID = "human:test"
	policyTestRepository = "owner/repo"
	policyTestBranch = "main"
	outputJSON = true

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runPolicyTest failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	var out struct {
		Decision    string `json:"decision"`
		Blocked     bool   `json:"blocked"`
		BlockReason string `json:"block_reason"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if !out.Blocked {
		t.Fatalf("expected blocked=true, got false")
	}
	if out.Decision == "" {
		t.Fatalf("expected decision in output")
	}
	if out.BlockReason == "" {
		t.Fatalf("expected block_reason in output")
	}
}

func TestPolicyTestCmd_Matrix(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}

	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"low","risk_score":0.2,"bump_type":"patch","actor_type":"human","actor_id":"human:one"},
  {"name":"high","risk_score":0.9,"bump_type":"major","actor_type":"agent","actor_id":"agent:bot"}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRequireApproved := policyTestRequireApproved
	oldJSON := outputJSON
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRequireApproved = oldRequireApproved
		outputJSON = oldJSON
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestRequireApproved = false
	outputJSON = true

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runPolicyTest matrix failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	var out []struct {
		Name   string `json:"name"`
		Output struct {
			Blocked bool `json:"blocked"`
		} `json:"output"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(out))
	}
	if out[0].Output.Blocked {
		t.Fatalf("expected low scenario not blocked")
	}
	if !out[1].Output.Blocked {
		t.Fatalf("expected high scenario blocked")
	}
}

func TestPolicyTestCmd_MatrixYAML(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}

	matrixPath := filepath.Join(tmpDir, "matrix.yaml")
	matrix := `- name: low
  risk_score: 0.2
  bump_type: patch
  actor_type: human
- name: high
  risk_score: 0.9
  bump_type: major
  actor_type: agent
`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRequireApproved := policyTestRequireApproved
	oldJSON := outputJSON
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRequireApproved = oldRequireApproved
		outputJSON = oldJSON
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestRequireApproved = false
	outputJSON = true

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runPolicyTest yaml matrix failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var out []struct {
		Name   string `json:"name"`
		Output struct {
			Blocked bool `json:"blocked"`
		} `json:"output"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(out))
	}
	if out[0].Output.Blocked {
		t.Fatalf("expected low scenario not blocked")
	}
	if !out[1].Output.Blocked {
		t.Fatalf("expected high scenario blocked")
	}
}

func TestPolicyTestCmd_FailOnBlockedSingle(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	content := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRisk := policyTestRiskScore
	oldFail := policyTestFailOnBlocked
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRiskScore = oldRisk
		policyTestFailOnBlocked = oldFail
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = ""
	policyTestRiskScore = 0.9
	policyTestFailOnBlocked = true

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected error with --fail-on-blocked")
	}
}

func TestPolicyTestCmd_FailOnBlockedMatrix(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"ok","risk_score":0.2},
  {"name":"blocked","risk_score":0.95}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldFail := policyTestFailOnBlocked
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestFailOnBlocked = oldFail
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestFailOnBlocked = true

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected error with --fail-on-blocked matrix")
	}
}

func TestPolicyTestCmd_RequireApprovedSingle(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "review.policy")
	content := `
rule "require-review" {
    when { risk_score >= 0.1 }
    then { require_approval(role: "reviewer") }
}
`
	if err := os.WriteFile(policyPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRisk := policyTestRiskScore
	oldRequireApproved := policyTestRequireApproved
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRiskScore = oldRisk
		policyTestRequireApproved = oldRequireApproved
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = ""
	policyTestRiskScore = 0.2
	policyTestRequireApproved = true

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected error with --require-approved when decision is not approved")
	}
}

func TestPolicyTestCmd_RequireApprovedMatrix(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "review.policy")
	policyContent := `
rule "require-review" {
    when { risk_score >= 0.8 }
    then { require_approval(role: "reviewer") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"approved","risk_score":0.2},
  {"name":"needs-review","risk_score":0.95}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRequireApproved := policyTestRequireApproved
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRequireApproved = oldRequireApproved
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestRequireApproved = true

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected error with --require-approved matrix")
	}
}

func TestPolicyTestCmd_InputFromStdinJSON(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	content := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRisk := policyTestRiskScore
	oldRequireApproved := policyTestRequireApproved
	oldJSON := outputJSON
	oldStdin := os.Stdin
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRiskScore = oldRisk
		policyTestRequireApproved = oldRequireApproved
		outputJSON = oldJSON
		os.Stdin = oldStdin
	}()

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdin pipe: %v", err)
	}
	if _, err := stdinW.Write([]byte(`{"risk_score":0.95,"bump_type":"major","actor_type":"agent"}`)); err != nil {
		t.Fatalf("failed to write stdin: %v", err)
	}
	_ = stdinW.Close()
	os.Stdin = stdinR

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = "-"
	policyTestMatrix = ""
	policyTestRiskScore = 0.1
	policyTestRequireApproved = false
	outputJSON = true

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runPolicyTest stdin input failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var out struct {
		Blocked bool `json:"blocked"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if !out.Blocked {
		t.Fatalf("expected stdin scenario to be blocked")
	}
}

func TestPolicyTestCmd_MatrixFromStdinYAML(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRequireApproved := policyTestRequireApproved
	oldJSON := outputJSON
	oldStdin := os.Stdin
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRequireApproved = oldRequireApproved
		outputJSON = oldJSON
		os.Stdin = oldStdin
	}()

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdin pipe: %v", err)
	}
	yaml := `- name: low
  risk_score: 0.2
  bump_type: patch
  actor_type: human
- name: high
  risk_score: 0.9
  bump_type: major
  actor_type: agent
`
	if _, err := stdinW.Write([]byte(yaml)); err != nil {
		t.Fatalf("failed to write stdin: %v", err)
	}
	_ = stdinW.Close()
	os.Stdin = stdinR

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = "-"
	policyTestRequireApproved = false
	outputJSON = true

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runPolicyTest stdin matrix failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var out []struct {
		Name   string `json:"name"`
		Output struct {
			Blocked bool `json:"blocked"`
		} `json:"output"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(out))
	}
	if out[0].Output.Blocked {
		t.Fatalf("expected low scenario not blocked")
	}
	if !out[1].Output.Blocked {
		t.Fatalf("expected high scenario blocked")
	}
}

func TestPolicyTestCmd_MatrixAssertExpectedPass(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"low","risk_score":0.2,"expect":{"decision":"approval_required","blocked":false,"required_approvers":1}},
  {"name":"high","risk_score":0.95,"expect":{"decision":"rejected","blocked":true,"required_approvers":0,"block_reason":"too risky","matched_rules":["high_risk"]}}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRequireApproved := policyTestRequireApproved
	oldAssert := policyTestAssertExpected
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRequireApproved = oldRequireApproved
		policyTestAssertExpected = oldAssert
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestRequireApproved = false
	policyTestAssertExpected = true

	if err := runPolicyTest(policyTestCmd, nil); err != nil {
		t.Fatalf("expected --assert-expected to pass, got error: %v", err)
	}
}

func TestPolicyTestCmd_MatrixAssertExpectedFail(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"high","risk_score":0.95,"expect":{"decision":"rejected","blocked":true,"required_approvers":2}}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRequireApproved := policyTestRequireApproved
	oldAssert := policyTestAssertExpected
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRequireApproved = oldRequireApproved
		policyTestAssertExpected = oldAssert
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestRequireApproved = false
	policyTestAssertExpected = true

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected --assert-expected mismatch to fail")
	}
}

func TestPolicyTestCmd_MatrixAssertExpectedFailMatchedRules(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"high","risk_score":0.95,"expect":{"decision":"rejected","blocked":true,"required_approvers":0,"matched_rules":["different_rule"]}}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRequireApproved := policyTestRequireApproved
	oldAssert := policyTestAssertExpected
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRequireApproved = oldRequireApproved
		policyTestAssertExpected = oldAssert
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestRequireApproved = false
	policyTestAssertExpected = true

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected --assert-expected matched_rules mismatch to fail")
	}
}

func TestPolicyTestCmd_MatrixAssertExpectedFailBlockReason(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"high","risk_score":0.95,"expect":{"decision":"rejected","blocked":true,"required_approvers":0,"block_reason":"different reason","matched_rules":["high_risk"]}}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRequireApproved := policyTestRequireApproved
	oldAssert := policyTestAssertExpected
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRequireApproved = oldRequireApproved
		policyTestAssertExpected = oldAssert
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestRequireApproved = false
	policyTestAssertExpected = true

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected --assert-expected block_reason mismatch to fail")
	}
}

func TestPolicyTestCmd_MatrixAssertExpectedReviewersPass(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "reviewers.policy")
	policyContent := `
rule "needs-reviewers" {
    when { risk_score >= 0.8 }
    then {
        require_approval(count: 1)
        add_reviewer(reviewer: "security-team")
    }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"high","risk_score":0.95,"expect":{"decision":"approval_required","blocked":false,"required_approvers":1,"reviewers":["security-team"],"matched_rules":["needs_reviewers"]}}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRequireApproved := policyTestRequireApproved
	oldAssert := policyTestAssertExpected
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRequireApproved = oldRequireApproved
		policyTestAssertExpected = oldAssert
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestRequireApproved = false
	policyTestAssertExpected = true

	if err := runPolicyTest(policyTestCmd, nil); err != nil {
		t.Fatalf("expected reviewers assertion to pass, got: %v", err)
	}
}

func TestPolicyTestCmd_MatrixAssertExpectedReviewersFail(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "reviewers.policy")
	policyContent := `
rule "needs-reviewers" {
    when { risk_score >= 0.8 }
    then {
        require_approval(count: 1)
        add_reviewer(reviewer: "security-team")
    }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"high","risk_score":0.95,"expect":{"decision":"approval_required","blocked":false,"required_approvers":1,"reviewers":["platform-team"],"matched_rules":["needs_reviewers"]}}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRequireApproved := policyTestRequireApproved
	oldAssert := policyTestAssertExpected
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRequireApproved = oldRequireApproved
		policyTestAssertExpected = oldAssert
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestRequireApproved = false
	policyTestAssertExpected = true

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected reviewers mismatch to fail")
	}
}

func TestPolicyTestCmd_MatrixAssertExpectedRequiredActionsPass(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "actions.policy")
	policyContent := `
rule "needs-action" {
    when { risk_score >= 0.8 }
    then {
        require_approval(count: 1)
    }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"high","risk_score":0.95,"expect":{"decision":"approval_required","blocked":false,"required_approvers":1,"required_actions":[],"matched_rules":["needs_action"]}}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRequireApproved := policyTestRequireApproved
	oldAssert := policyTestAssertExpected
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRequireApproved = oldRequireApproved
		policyTestAssertExpected = oldAssert
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestRequireApproved = false
	policyTestAssertExpected = true

	if err := runPolicyTest(policyTestCmd, nil); err != nil {
		t.Fatalf("expected required_actions assertion to pass, got: %v", err)
	}
}

func TestPolicyTestCmd_MatrixAssertExpectedRequiredActionsFail(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "actions.policy")
	policyContent := `
rule "needs-action" {
    when { risk_score >= 0.8 }
    then {
        require_approval(count: 1)
    }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"high","risk_score":0.95,"expect":{"decision":"approval_required","blocked":false,"required_approvers":1,"required_actions":[{"type":"human_approval","description":"Different"}],"matched_rules":["needs_action"]}}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRequireApproved := policyTestRequireApproved
	oldAssert := policyTestAssertExpected
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRequireApproved = oldRequireApproved
		policyTestAssertExpected = oldAssert
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestRequireApproved = false
	policyTestAssertExpected = true

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected required_actions mismatch to fail")
	}
}

func TestPolicyTestCmd_MatrixAssertExpectedRationaleAndConditionsPass(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "rationale.policy")
	policyContent := `
rule "needs-rationale" {
    when { risk_score >= 0.8 }
    then {
        add_rationale(message: "High risk requires explanation")
        add_condition(type: "time_window", value: "business_hours")
    }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"high","risk_score":0.95,"expect":{"decision":"approved","blocked":false,"required_approvers":0,"rationale":["High risk requires explanation"],"conditions":[{"type":"time_window","value":"business_hours"}],"matched_rules":["needs_rationale"]}}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRequireApproved := policyTestRequireApproved
	oldAssert := policyTestAssertExpected
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRequireApproved = oldRequireApproved
		policyTestAssertExpected = oldAssert
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestRequireApproved = false
	policyTestAssertExpected = true

	if err := runPolicyTest(policyTestCmd, nil); err != nil {
		t.Fatalf("expected rationale/conditions assertions to pass, got: %v", err)
	}
}

func TestPolicyTestCmd_MatrixAssertExpectedRationaleAndConditionsFail(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "rationale.policy")
	policyContent := `
rule "needs-rationale" {
    when { risk_score >= 0.8 }
    then {
        add_rationale(message: "High risk requires explanation")
        add_condition(type: "time_window", value: "business_hours")
    }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"high","risk_score":0.95,"expect":{"decision":"approved","blocked":false,"required_approvers":0,"rationale":["Different rationale"],"conditions":[{"type":"manual_gate","value":"cab"}],"matched_rules":["needs_rationale"]}}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRequireApproved := policyTestRequireApproved
	oldAssert := policyTestAssertExpected
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRequireApproved = oldRequireApproved
		policyTestAssertExpected = oldAssert
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestRequireApproved = false
	policyTestAssertExpected = true

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected rationale/conditions mismatch to fail")
	}
}

func TestPolicyTestCmd_MatrixAssertExpectedFail_JSONIncludesAssertionDiff(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"high","risk_score":0.95,"expect":{"decision":"approved","blocked":false}}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRequireApproved := policyTestRequireApproved
	oldAssert := policyTestAssertExpected
	oldJSON := outputJSON
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRequireApproved = oldRequireApproved
		policyTestAssertExpected = oldAssert
		outputJSON = oldJSON
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestRequireApproved = false
	policyTestAssertExpected = true
	outputJSON = true

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout
	if err == nil {
		t.Fatalf("expected assert-expected failure")
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var out []struct {
		Name          string `json:"name"`
		AssertionDiff *struct {
			Mismatches []struct {
				Field string `json:"field"`
			} `json:"mismatches"`
		} `json:"assertion_diff"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(out))
	}
	if out[0].AssertionDiff == nil {
		t.Fatalf("expected assertion_diff in output")
	}
	if len(out[0].AssertionDiff.Mismatches) == 0 {
		t.Fatalf("expected at least one mismatch in assertion_diff")
	}
}

func TestPolicyTestCmd_MatrixScenarioFilter(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"low","risk_score":0.2},
  {"name":"high","risk_score":0.95}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldScenarios := policyTestScenarios
	oldExcludeScenarios := policyTestExcludeScenarios
	oldExcludePatterns := policyTestExcludePatterns
	oldJSON := outputJSON
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestScenarios = oldScenarios
		policyTestExcludeScenarios = oldExcludeScenarios
		policyTestExcludePatterns = oldExcludePatterns
		outputJSON = oldJSON
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestScenarios = []string{"high"}
	policyTestExcludeScenarios = nil
	policyTestExcludePatterns = nil
	outputJSON = true

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("runPolicyTest matrix with scenario filter failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var out []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(out))
	}
	if out[0].Name != "high" {
		t.Fatalf("expected scenario 'high', got %q", out[0].Name)
	}
}

func TestPolicyTestCmd_MatrixScenarioFilterMissing(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[{"name":"low","risk_score":0.2}]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldScenarios := policyTestScenarios
	oldScenarioPatterns := policyTestScenarioPatterns
	oldExcludeScenarios := policyTestExcludeScenarios
	oldExcludePatterns := policyTestExcludePatterns
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestScenarios = oldScenarios
		policyTestScenarioPatterns = oldScenarioPatterns
		policyTestExcludeScenarios = oldExcludeScenarios
		policyTestExcludePatterns = oldExcludePatterns
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestScenarios = []string{"does-not-exist"}
	policyTestScenarioPatterns = nil
	policyTestExcludeScenarios = nil
	policyTestExcludePatterns = nil

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected missing scenario filter to fail")
	}
}

func TestPolicyTestCmd_MatrixScenarioPatternFilter(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"low-risk","risk_score":0.2},
  {"name":"high-risk","risk_score":0.95},
  {"name":"high-major","risk_score":0.9}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldScenarios := policyTestScenarios
	oldScenarioPatterns := policyTestScenarioPatterns
	oldExcludeScenarios := policyTestExcludeScenarios
	oldExcludePatterns := policyTestExcludePatterns
	oldJSON := outputJSON
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestScenarios = oldScenarios
		policyTestScenarioPatterns = oldScenarioPatterns
		policyTestExcludeScenarios = oldExcludeScenarios
		policyTestExcludePatterns = oldExcludePatterns
		outputJSON = oldJSON
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestScenarios = nil
	policyTestScenarioPatterns = []string{"high-*"}
	policyTestExcludeScenarios = nil
	policyTestExcludePatterns = nil
	outputJSON = true

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("runPolicyTest matrix with scenario-pattern filter failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var out []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(out))
	}
	if out[0].Name != "high-risk" || out[1].Name != "high-major" {
		t.Fatalf("unexpected scenario list: %+v", out)
	}
}

func TestPolicyTestCmd_MatrixScenarioPatternInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[{"name":"high","risk_score":0.95}]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldScenarios := policyTestScenarios
	oldScenarioPatterns := policyTestScenarioPatterns
	oldExcludeScenarios := policyTestExcludeScenarios
	oldExcludePatterns := policyTestExcludePatterns
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestScenarios = oldScenarios
		policyTestScenarioPatterns = oldScenarioPatterns
		policyTestExcludeScenarios = oldExcludeScenarios
		policyTestExcludePatterns = oldExcludePatterns
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestScenarios = nil
	policyTestScenarioPatterns = []string{"["}
	policyTestExcludeScenarios = nil
	policyTestExcludePatterns = nil

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected invalid scenario-pattern to fail")
	}
}

func TestPolicyTestCmd_MatrixExcludeScenario(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"low","risk_score":0.2},
  {"name":"high","risk_score":0.95}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldScenarios := policyTestScenarios
	oldScenarioPatterns := policyTestScenarioPatterns
	oldExcludeScenarios := policyTestExcludeScenarios
	oldExcludePatterns := policyTestExcludePatterns
	oldJSON := outputJSON
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestScenarios = oldScenarios
		policyTestScenarioPatterns = oldScenarioPatterns
		policyTestExcludeScenarios = oldExcludeScenarios
		policyTestExcludePatterns = oldExcludePatterns
		outputJSON = oldJSON
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestScenarios = nil
	policyTestScenarioPatterns = nil
	policyTestExcludeScenarios = []string{"high"}
	policyTestExcludePatterns = nil
	outputJSON = true

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("runPolicyTest matrix with exclude-scenario failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var out []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(out) != 1 || out[0].Name != "low" {
		t.Fatalf("unexpected filtered output: %+v", out)
	}
}

func TestPolicyTestCmd_MatrixExcludeScenarioPattern(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"low-risk","risk_score":0.2},
  {"name":"high-risk","risk_score":0.95},
  {"name":"high-major","risk_score":0.9}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldScenarios := policyTestScenarios
	oldScenarioPatterns := policyTestScenarioPatterns
	oldExcludeScenarios := policyTestExcludeScenarios
	oldExcludePatterns := policyTestExcludePatterns
	oldJSON := outputJSON
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestScenarios = oldScenarios
		policyTestScenarioPatterns = oldScenarioPatterns
		policyTestExcludeScenarios = oldExcludeScenarios
		policyTestExcludePatterns = oldExcludePatterns
		outputJSON = oldJSON
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestScenarios = nil
	policyTestScenarioPatterns = nil
	policyTestExcludeScenarios = nil
	policyTestExcludePatterns = []string{"high-*"}
	outputJSON = true

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("runPolicyTest matrix with exclude-scenario-pattern failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var out []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(out) != 1 || out[0].Name != "low-risk" {
		t.Fatalf("unexpected filtered output: %+v", out)
	}
}

func TestPolicyTestCmd_ListScenariosJSON(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"low","risk_score":0.2},
  {"name":"high","risk_score":0.95}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldScenarios := policyTestScenarios
	oldList := policyTestListScenarios
	oldJSON := outputJSON
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestScenarios = oldScenarios
		policyTestListScenarios = oldList
		outputJSON = oldJSON
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestScenarios = nil
	policyTestListScenarios = true
	outputJSON = true

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("list-scenarios failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var out struct {
		Scenarios []string `json:"scenarios"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(out.Scenarios) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(out.Scenarios))
	}
	if out.Scenarios[0] != "low" || out.Scenarios[1] != "high" {
		t.Fatalf("unexpected scenario list: %v", out.Scenarios)
	}
}

func TestPolicyTestCmd_ListScenariosJSON_WithTagAndShard(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"a","tags":["critical"],"risk_score":0.2},
  {"name":"b","tags":["critical"],"risk_score":0.95},
  {"name":"c","tags":["smoke"],"risk_score":0.5}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldScenarios := policyTestScenarios
	oldScenarioPatterns := policyTestScenarioPatterns
	oldScenarioTags := policyTestScenarioTags
	oldExcludeScenarios := policyTestExcludeScenarios
	oldExcludePatterns := policyTestExcludePatterns
	oldExcludeTags := policyTestExcludeTags
	oldShardIndex := policyTestShardIndex
	oldShardTotal := policyTestShardTotal
	oldList := policyTestListScenarios
	oldJSON := outputJSON
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestScenarios = oldScenarios
		policyTestScenarioPatterns = oldScenarioPatterns
		policyTestScenarioTags = oldScenarioTags
		policyTestExcludeScenarios = oldExcludeScenarios
		policyTestExcludePatterns = oldExcludePatterns
		policyTestExcludeTags = oldExcludeTags
		policyTestShardIndex = oldShardIndex
		policyTestShardTotal = oldShardTotal
		policyTestListScenarios = oldList
		outputJSON = oldJSON
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestScenarios = nil
	policyTestScenarioPatterns = nil
	policyTestScenarioTags = []string{"critical"}
	policyTestExcludeScenarios = nil
	policyTestExcludePatterns = nil
	policyTestExcludeTags = nil
	policyTestShardIndex = 1
	policyTestShardTotal = 2
	policyTestListScenarios = true
	outputJSON = true

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("list-scenarios with tag+shard failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var out struct {
		Scenarios  []string `json:"scenarios"`
		ShardIndex int      `json:"shard_index"`
		ShardTotal int      `json:"shard_total"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if out.ShardIndex != 1 || out.ShardTotal != 2 {
		t.Fatalf("unexpected shard metadata: %+v", out)
	}
	// Only critical-tag scenarios are eligible, and sharding should return a non-empty subset.
	if len(out.Scenarios) == 0 {
		t.Fatalf("expected at least one listed scenario for shard")
	}
	for _, name := range out.Scenarios {
		if name != "a" && name != "b" {
			t.Fatalf("unexpected scenario %q in tagged shard output", name)
		}
	}
}

func TestPolicyTestCmd_ListScenariosRequiresMatrix(t *testing.T) {
	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldList := policyTestListScenarios
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestListScenarios = oldList
	}()

	policyTestFile = ""
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = ""
	policyTestListScenarios = true

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected --list-scenarios without --matrix to fail")
	}
}

func TestPolicyTestCmd_MatrixSummaryJSON(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"low","risk_score":0.2},
  {"name":"high","risk_score":0.95}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldSummary := policyTestSummary
	oldJSON := outputJSON
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestSummary = oldSummary
		outputJSON = oldJSON
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestSummary = true
	outputJSON = true

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("runPolicyTest matrix with --summary failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var out struct {
		Results []struct {
			Name   string `json:"name"`
			Output struct {
				Blocked bool `json:"blocked"`
			} `json:"output"`
		} `json:"results"`
		Summary struct {
			Total      int            `json:"total"`
			Blocked    int            `json:"blocked"`
			Mismatched int            `json:"mismatched"`
			Decisions  map[string]int `json:"decisions"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(out.Results) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(out.Results))
	}
	if out.Summary.Total != 2 {
		t.Fatalf("expected summary.total=2, got %d", out.Summary.Total)
	}
	if out.Summary.Blocked != 1 {
		t.Fatalf("expected summary.blocked=1, got %d", out.Summary.Blocked)
	}
	if out.Summary.Mismatched != 0 {
		t.Fatalf("expected summary.mismatched=0, got %d", out.Summary.Mismatched)
	}
	if out.Summary.Decisions["approval_required"] != 1 || out.Summary.Decisions["rejected"] != 1 {
		t.Fatalf("unexpected decision counts: %+v", out.Summary.Decisions)
	}
}

func TestPolicyTestCmd_MatrixSummaryText(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"low","risk_score":0.2},
  {"name":"high","risk_score":0.95}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldSummary := policyTestSummary
	oldJSON := outputJSON
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestSummary = oldSummary
		outputJSON = oldJSON
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestSummary = true
	outputJSON = false

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("runPolicyTest matrix text with --summary failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("Summary:")) {
		t.Fatalf("expected summary header in text output, got: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte("Total:      2")) {
		t.Fatalf("expected total count in text summary, got: %s", out)
	}
	if !bytes.Contains([]byte(out), []byte("Blocked:    1")) {
		t.Fatalf("expected blocked count in text summary, got: %s", out)
	}
}

func TestPolicyTestCmd_ExplainJSONIncludesRuleTrace(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "trace.policy")
	content := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { require_approval(count: 1) }
}

rule "agent-only" {
    when { actor_type == "agent" }
    then { add_rationale(message: "agent path") }
}
`
	if err := os.WriteFile(policyPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRisk := policyTestRiskScore
	oldActorType := policyTestActorType
	oldRequireApproved := policyTestRequireApproved
	oldJSON := outputJSON
	oldExplain := policyTestExplain
	oldExplainMode := policyTestExplainMode
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRiskScore = oldRisk
		policyTestActorType = oldActorType
		policyTestRequireApproved = oldRequireApproved
		outputJSON = oldJSON
		policyTestExplain = oldExplain
		policyTestExplainMode = oldExplainMode
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = ""
	policyTestRiskScore = 0.9
	policyTestActorType = "human"
	policyTestRequireApproved = false
	outputJSON = true
	policyTestExplain = true
	policyTestExplainMode = "all"

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("runPolicyTest with --explain failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var out struct {
		Decision  string `json:"decision"`
		RuleTrace []struct {
			RuleID     string `json:"rule_id"`
			Matched    bool   `json:"matched"`
			Conditions []struct {
				Field   string `json:"field"`
				Matched bool   `json:"matched"`
			} `json:"conditions"`
		} `json:"rule_trace"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if out.Decision == "" {
		t.Fatalf("expected decision to be present")
	}
	if len(out.RuleTrace) != 2 {
		t.Fatalf("expected 2 rule traces, got %d", len(out.RuleTrace))
	}
	if !out.RuleTrace[0].Matched {
		t.Fatalf("expected first trace rule to match")
	}
	if out.RuleTrace[1].Matched {
		t.Fatalf("expected second trace rule to not match")
	}
	if len(out.RuleTrace[1].Conditions) != 1 || out.RuleTrace[1].Conditions[0].Matched {
		t.Fatalf("expected second rule condition to be unmatched")
	}
}

func TestPolicyTestCmd_ExplainModeMatchedOnly(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "trace.policy")
	content := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { require_approval(count: 1) }
}

rule "agent-only" {
    when { actor_type == "agent" }
    then { add_rationale(message: "agent path") }
}
`
	if err := os.WriteFile(policyPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRisk := policyTestRiskScore
	oldActorType := policyTestActorType
	oldRequireApproved := policyTestRequireApproved
	oldJSON := outputJSON
	oldExplain := policyTestExplain
	oldExplainMode := policyTestExplainMode
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRiskScore = oldRisk
		policyTestActorType = oldActorType
		policyTestRequireApproved = oldRequireApproved
		outputJSON = oldJSON
		policyTestExplain = oldExplain
		policyTestExplainMode = oldExplainMode
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = ""
	policyTestRiskScore = 0.9
	policyTestActorType = "human"
	policyTestRequireApproved = false
	outputJSON = true
	policyTestExplain = true
	policyTestExplainMode = "matched"

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("runPolicyTest with --explain --explain-mode=matched failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var out struct {
		RuleTrace []struct {
			RuleID  string `json:"rule_id"`
			Matched bool   `json:"matched"`
		} `json:"rule_trace"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(out.RuleTrace) != 1 {
		t.Fatalf("expected only 1 matched rule trace, got %d", len(out.RuleTrace))
	}
	if !out.RuleTrace[0].Matched {
		t.Fatalf("expected remaining trace to be matched")
	}
}

func TestPolicyTestCmd_ExplainModeInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "trace.policy")
	content := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { require_approval(count: 1) }
}
`
	if err := os.WriteFile(policyPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldRisk := policyTestRiskScore
	oldRequireApproved := policyTestRequireApproved
	oldExplain := policyTestExplain
	oldExplainMode := policyTestExplainMode
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestRiskScore = oldRisk
		policyTestRequireApproved = oldRequireApproved
		policyTestExplain = oldExplain
		policyTestExplainMode = oldExplainMode
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = ""
	policyTestRiskScore = 0.9
	policyTestRequireApproved = false
	policyTestExplain = true
	policyTestExplainMode = "verbose"

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected invalid explain mode to fail")
	}
}

func TestPolicyTestCmd_MatrixScenarioTagFilter(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"low","tags":["smoke","fast"],"risk_score":0.2},
  {"name":"high","tags":["critical"],"risk_score":0.95}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldScenarios := policyTestScenarios
	oldScenarioPatterns := policyTestScenarioPatterns
	oldScenarioTags := policyTestScenarioTags
	oldExcludeScenarios := policyTestExcludeScenarios
	oldExcludePatterns := policyTestExcludePatterns
	oldExcludeTags := policyTestExcludeTags
	oldShardIndex := policyTestShardIndex
	oldShardTotal := policyTestShardTotal
	oldJSON := outputJSON
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestScenarios = oldScenarios
		policyTestScenarioPatterns = oldScenarioPatterns
		policyTestScenarioTags = oldScenarioTags
		policyTestExcludeScenarios = oldExcludeScenarios
		policyTestExcludePatterns = oldExcludePatterns
		policyTestExcludeTags = oldExcludeTags
		policyTestShardIndex = oldShardIndex
		policyTestShardTotal = oldShardTotal
		outputJSON = oldJSON
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestScenarios = nil
	policyTestScenarioPatterns = nil
	policyTestScenarioTags = []string{"critical"}
	policyTestExcludeScenarios = nil
	policyTestExcludePatterns = nil
	policyTestExcludeTags = nil
	policyTestShardIndex = 0
	policyTestShardTotal = 0
	outputJSON = true

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("runPolicyTest matrix with scenario-tag filter failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var out []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(out) != 1 || out[0].Name != "high" {
		t.Fatalf("unexpected filtered output: %+v", out)
	}
}

func TestPolicyTestCmd_MatrixExcludeScenarioTag(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"low","tags":["smoke"],"risk_score":0.2},
  {"name":"high","tags":["critical"],"risk_score":0.95}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldScenarios := policyTestScenarios
	oldScenarioPatterns := policyTestScenarioPatterns
	oldScenarioTags := policyTestScenarioTags
	oldExcludeScenarios := policyTestExcludeScenarios
	oldExcludePatterns := policyTestExcludePatterns
	oldExcludeTags := policyTestExcludeTags
	oldShardIndex := policyTestShardIndex
	oldShardTotal := policyTestShardTotal
	oldJSON := outputJSON
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestScenarios = oldScenarios
		policyTestScenarioPatterns = oldScenarioPatterns
		policyTestScenarioTags = oldScenarioTags
		policyTestExcludeScenarios = oldExcludeScenarios
		policyTestExcludePatterns = oldExcludePatterns
		policyTestExcludeTags = oldExcludeTags
		policyTestShardIndex = oldShardIndex
		policyTestShardTotal = oldShardTotal
		outputJSON = oldJSON
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestScenarios = nil
	policyTestScenarioPatterns = nil
	policyTestScenarioTags = nil
	policyTestExcludeScenarios = nil
	policyTestExcludePatterns = nil
	policyTestExcludeTags = []string{"critical"}
	policyTestShardIndex = 0
	policyTestShardTotal = 0
	outputJSON = true

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("runPolicyTest matrix with exclude-scenario-tag failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var out []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(out) != 1 || out[0].Name != "low" {
		t.Fatalf("unexpected filtered output: %+v", out)
	}
}

func TestPolicyTestCmd_MatrixShardSelection(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"a","risk_score":0.2},
  {"name":"b","risk_score":0.3},
  {"name":"c","risk_score":0.9},
  {"name":"d","risk_score":0.4}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldScenarios := policyTestScenarios
	oldScenarioPatterns := policyTestScenarioPatterns
	oldScenarioTags := policyTestScenarioTags
	oldExcludeScenarios := policyTestExcludeScenarios
	oldExcludePatterns := policyTestExcludePatterns
	oldExcludeTags := policyTestExcludeTags
	oldShardIndex := policyTestShardIndex
	oldShardTotal := policyTestShardTotal
	oldJSON := outputJSON
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestScenarios = oldScenarios
		policyTestScenarioPatterns = oldScenarioPatterns
		policyTestScenarioTags = oldScenarioTags
		policyTestExcludeScenarios = oldExcludeScenarios
		policyTestExcludePatterns = oldExcludePatterns
		policyTestExcludeTags = oldExcludeTags
		policyTestShardIndex = oldShardIndex
		policyTestShardTotal = oldShardTotal
		outputJSON = oldJSON
	}()

	runShard := func(shardIndex int) []string {
		policyTestFile = policyPath
		policyTestDir = ""
		policyTestInput = ""
		policyTestMatrix = matrixPath
		policyTestScenarios = nil
		policyTestScenarioPatterns = nil
		policyTestScenarioTags = nil
		policyTestExcludeScenarios = nil
		policyTestExcludePatterns = nil
		policyTestExcludeTags = nil
		policyTestShardIndex = shardIndex
		policyTestShardTotal = 2
		outputJSON = true

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := runPolicyTest(policyTestCmd, nil)

		w.Close()
		os.Stdout = oldStdout
		if err != nil {
			t.Fatalf("runPolicyTest matrix with shard %d failed: %v", shardIndex, err)
		}

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		var out []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
			t.Fatalf("invalid JSON output: %v", err)
		}
		names := make([]string, 0, len(out))
		for _, item := range out {
			names = append(names, item.Name)
		}
		return names
	}

	shard1 := runShard(1)
	shard2 := runShard(2)
	if len(shard1) == 0 || len(shard2) == 0 {
		t.Fatalf("expected both shards to contain scenarios, got shard1=%v shard2=%v", shard1, shard2)
	}

	seen := map[string]int{}
	for _, n := range shard1 {
		seen[n]++
	}
	for _, n := range shard2 {
		seen[n]++
	}
	if len(seen) != 4 {
		t.Fatalf("expected union of shards to contain 4 scenarios, got %d (%v)", len(seen), seen)
	}
	for name, count := range seen {
		if count != 1 {
			t.Fatalf("scenario %q appeared %d times across shards", name, count)
		}
	}
}

func TestPolicyTestCmd_MatrixShardInvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[{"name":"a","risk_score":0.2}]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldShardIndex := policyTestShardIndex
	oldShardTotal := policyTestShardTotal
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestShardIndex = oldShardIndex
		policyTestShardTotal = oldShardTotal
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestShardIndex = 1
	policyTestShardTotal = 0

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected invalid shard config to fail")
	}
}

func TestPolicyTestCmd_MatrixJUnitOut(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"low","risk_score":0.2},
  {"name":"high","risk_score":0.95}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}
	junitPath := filepath.Join(tmpDir, "policy-matrix.xml")

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldJUnit := policyTestJUnitOut
	oldFailOnBlocked := policyTestFailOnBlocked
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestJUnitOut = oldJUnit
		policyTestFailOnBlocked = oldFailOnBlocked
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestJUnitOut = junitPath
	policyTestFailOnBlocked = false

	if err := runPolicyTest(policyTestCmd, nil); err != nil {
		t.Fatalf("runPolicyTest with --junit-out failed: %v", err)
	}

	b, err := os.ReadFile(junitPath)
	if err != nil {
		t.Fatalf("failed to read junit file: %v", err)
	}
	var suite struct {
		XMLName  xml.Name `xml:"testsuite"`
		Tests    int      `xml:"tests,attr"`
		Failures int      `xml:"failures,attr"`
		Cases    []struct {
			Name    string `xml:"name,attr"`
			Failure *struct {
				Type    string `xml:"type,attr"`
				Message string `xml:"message,attr"`
			} `xml:"failure"`
		} `xml:"testcase"`
	}
	if err := xml.Unmarshal(b, &suite); err != nil {
		t.Fatalf("failed to parse junit xml: %v", err)
	}
	if suite.Tests != 2 {
		t.Fatalf("expected tests=2, got %d", suite.Tests)
	}
	if suite.Failures != 1 {
		t.Fatalf("expected failures=1 for blocked scenario, got %d", suite.Failures)
	}
}

func TestPolicyTestCmd_MatrixJUnitOut_AssertFailure(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"high","risk_score":0.95,"expect":{"decision":"approved","blocked":false}}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}
	junitPath := filepath.Join(tmpDir, "policy-matrix.xml")

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldJUnit := policyTestJUnitOut
	oldAssert := policyTestAssertExpected
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestJUnitOut = oldJUnit
		policyTestAssertExpected = oldAssert
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestJUnitOut = junitPath
	policyTestAssertExpected = true

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected --assert-expected to fail")
	}

	b, readErr := os.ReadFile(junitPath)
	if readErr != nil {
		t.Fatalf("expected junit file to exist on assert failure: %v", readErr)
	}
	var suite struct {
		Failures int `xml:"failures,attr"`
	}
	if unmarshalErr := xml.Unmarshal(b, &suite); unmarshalErr != nil {
		t.Fatalf("failed to parse junit xml: %v", unmarshalErr)
	}
	if suite.Failures == 0 {
		t.Fatalf("expected junit report to contain failure for assertion diff")
	}
}

func TestPolicyTestCmd_JUnitOutRequiresMatrix(t *testing.T) {
	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldJUnit := policyTestJUnitOut
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestJUnitOut = oldJUnit
	}()

	policyTestFile = ""
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = ""
	policyTestJUnitOut = "report.xml"

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected --junit-out without --matrix to fail")
	}
}

func TestPolicyTestCmd_MatrixSummaryOut(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"low","risk_score":0.2},
  {"name":"high","risk_score":0.95}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}
	summaryPath := filepath.Join(tmpDir, "policy-summary.json")

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldSummaryOut := policyTestSummaryOut
	oldAssert := policyTestAssertExpected
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestSummaryOut = oldSummaryOut
		policyTestAssertExpected = oldAssert
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestSummaryOut = summaryPath
	policyTestAssertExpected = false

	if err := runPolicyTest(policyTestCmd, nil); err != nil {
		t.Fatalf("runPolicyTest with --summary-out failed: %v", err)
	}

	b, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("failed to read summary file: %v", err)
	}
	var report struct {
		Total           int `json:"total"`
		Blocked         int `json:"blocked"`
		Mismatched      int `json:"mismatched"`
		FailedScenarios []struct {
			Name    string `json:"name"`
			Blocked bool   `json:"blocked"`
		} `json:"failed_scenarios"`
	}
	if err := json.Unmarshal(b, &report); err != nil {
		t.Fatalf("invalid summary JSON: %v", err)
	}
	if report.Total != 2 || report.Blocked != 1 || report.Mismatched != 0 {
		t.Fatalf("unexpected summary counts: %+v", report)
	}
	if len(report.FailedScenarios) != 1 || report.FailedScenarios[0].Name != "high" || !report.FailedScenarios[0].Blocked {
		t.Fatalf("unexpected failed_scenarios: %+v", report.FailedScenarios)
	}
}

func TestPolicyTestCmd_MatrixSummaryOut_AssertFailure(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "risk.policy")
	policyContent := `
rule "high-risk" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("failed to write policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[
  {"name":"high","risk_score":0.95,"expect":{"decision":"approved","blocked":false}}
]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}
	summaryPath := filepath.Join(tmpDir, "policy-summary.json")

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldSummaryOut := policyTestSummaryOut
	oldAssert := policyTestAssertExpected
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestSummaryOut = oldSummaryOut
		policyTestAssertExpected = oldAssert
	}()

	policyTestFile = policyPath
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	policyTestSummaryOut = summaryPath
	policyTestAssertExpected = true

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected --assert-expected to fail")
	}

	b, readErr := os.ReadFile(summaryPath)
	if readErr != nil {
		t.Fatalf("expected summary file to exist on assert failure: %v", readErr)
	}
	var report struct {
		Mismatched      int `json:"mismatched"`
		FailedScenarios []struct {
			Name       string `json:"name"`
			Mismatched bool   `json:"mismatched"`
		} `json:"failed_scenarios"`
	}
	if unmarshalErr := json.Unmarshal(b, &report); unmarshalErr != nil {
		t.Fatalf("invalid summary JSON: %v", unmarshalErr)
	}
	if report.Mismatched == 0 {
		t.Fatalf("expected mismatched count in summary report")
	}
	if len(report.FailedScenarios) == 0 || !report.FailedScenarios[0].Mismatched {
		t.Fatalf("expected failed_scenarios with mismatched=true")
	}
}

func TestPolicyTestCmd_SummaryOutRequiresMatrix(t *testing.T) {
	oldFile := policyTestFile
	oldDir := policyTestDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldSummaryOut := policyTestSummaryOut
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		policyTestSummaryOut = oldSummaryOut
	}()

	policyTestFile = ""
	policyTestDir = ""
	policyTestInput = ""
	policyTestMatrix = ""
	policyTestSummaryOut = "summary.json"

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected --summary-out without --matrix to fail")
	}
}

func TestPolicyTestCmd_MatrixCompareBaselineCandidate(t *testing.T) {
	tmpDir := t.TempDir()
	baselinePath := filepath.Join(tmpDir, "baseline.policy")
	baselineContent := `
rule "allow-all" {
    when { risk_score >= 0.0 }
    then { approve() }
}
`
	if err := os.WriteFile(baselinePath, []byte(baselineContent), 0o644); err != nil {
		t.Fatalf("failed to write baseline policy file: %v", err)
	}
	candidatePath := filepath.Join(tmpDir, "candidate.policy")
	candidateContent := `
rule "block-high" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`
	if err := os.WriteFile(candidatePath, []byte(candidateContent), 0o644); err != nil {
		t.Fatalf("failed to write candidate policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	matrix := `[{"name":"high","risk_score":0.95}]`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldFile := policyTestFile
	oldDir := policyTestDir
	oldBaselineFile := policyTestBaselineFile
	oldBaselineDir := policyTestBaselineDir
	oldCandidateFile := policyTestCandidateFile
	oldCandidateDir := policyTestCandidateDir
	oldInput := policyTestInput
	oldMatrix := policyTestMatrix
	oldJSON := outputJSON
	defer func() {
		policyTestFile = oldFile
		policyTestDir = oldDir
		policyTestBaselineFile = oldBaselineFile
		policyTestBaselineDir = oldBaselineDir
		policyTestCandidateFile = oldCandidateFile
		policyTestCandidateDir = oldCandidateDir
		policyTestInput = oldInput
		policyTestMatrix = oldMatrix
		outputJSON = oldJSON
	}()

	policyTestFile = ""
	policyTestDir = ""
	policyTestBaselineFile = baselinePath
	policyTestBaselineDir = ""
	policyTestCandidateFile = candidatePath
	policyTestCandidateDir = ""
	policyTestInput = ""
	policyTestMatrix = matrixPath
	outputJSON = true

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runPolicyTest(policyTestCmd, nil)

	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("runPolicyTest matrix compare failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	var out []struct {
		Name   string `json:"name"`
		Output struct {
			Decision string `json:"decision"`
		} `json:"output"`
		BaselineOutput struct {
			Decision string `json:"decision"`
		} `json:"baseline_output"`
		CandidateOutput struct {
			Decision string `json:"decision"`
		} `json:"candidate_output"`
		Comparison struct {
			Changed   bool   `json:"changed"`
			Direction string `json:"direction"`
			Decision  struct {
				Changed bool `json:"changed"`
			} `json:"decision"`
			Blocked struct {
				Changed bool `json:"changed"`
			} `json:"blocked"`
		} `json:"comparison"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(out))
	}
	if out[0].BaselineOutput.Decision != "approved" {
		t.Fatalf("expected baseline decision approved, got %q", out[0].BaselineOutput.Decision)
	}
	if out[0].CandidateOutput.Decision != "rejected" {
		t.Fatalf("expected candidate decision rejected, got %q", out[0].CandidateOutput.Decision)
	}
	if out[0].Output.Decision != "rejected" {
		t.Fatalf("expected output decision to follow candidate, got %q", out[0].Output.Decision)
	}
	if !out[0].Comparison.Changed {
		t.Fatalf("expected comparison.changed=true")
	}
	if out[0].Comparison.Direction != "stricter" {
		t.Fatalf("expected comparison.direction=stricter, got %q", out[0].Comparison.Direction)
	}
	if !out[0].Comparison.Decision.Changed {
		t.Fatalf("expected comparison.decision.changed=true")
	}
	if !out[0].Comparison.Blocked.Changed {
		t.Fatalf("expected comparison.blocked.changed=true")
	}
}

func TestPolicyTestCmd_CompareRequiresMatrix(t *testing.T) {
	oldBaselineFile := policyTestBaselineFile
	oldCandidateFile := policyTestCandidateFile
	oldMatrix := policyTestMatrix
	defer func() {
		policyTestBaselineFile = oldBaselineFile
		policyTestCandidateFile = oldCandidateFile
		policyTestMatrix = oldMatrix
	}()

	policyTestBaselineFile = "baseline.policy"
	policyTestCandidateFile = "candidate.policy"
	policyTestMatrix = ""

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected compare mode without --matrix to fail")
	}
}

func TestPolicyTestCmd_CompareRequiresCandidate(t *testing.T) {
	tmpDir := t.TempDir()
	baselinePath := filepath.Join(tmpDir, "baseline.policy")
	if err := os.WriteFile(baselinePath, []byte(`
rule "allow-all" {
    when { risk_score >= 0.0 }
    then { approve() }
}
`), 0o644); err != nil {
		t.Fatalf("failed to write baseline policy file: %v", err)
	}

	oldBaselineFile := policyTestBaselineFile
	oldCandidateFile := policyTestCandidateFile
	oldMatrix := policyTestMatrix
	defer func() {
		policyTestBaselineFile = oldBaselineFile
		policyTestCandidateFile = oldCandidateFile
		policyTestMatrix = oldMatrix
	}()

	policyTestBaselineFile = baselinePath
	policyTestCandidateFile = ""
	policyTestMatrix = "matrix.json"

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected compare mode without candidate to fail")
	}
}

func TestPolicyTestCmd_CompareFailOnStricter(t *testing.T) {
	tmpDir := t.TempDir()
	baselinePath := filepath.Join(tmpDir, "baseline.policy")
	if err := os.WriteFile(baselinePath, []byte(`
rule "allow-all" {
    when { risk_score >= 0.0 }
    then { approve() }
}
`), 0o644); err != nil {
		t.Fatalf("failed to write baseline policy file: %v", err)
	}
	candidatePath := filepath.Join(tmpDir, "candidate.policy")
	if err := os.WriteFile(candidatePath, []byte(`
rule "block-high" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`), 0o644); err != nil {
		t.Fatalf("failed to write candidate policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	if err := os.WriteFile(matrixPath, []byte(`[{"name":"high","risk_score":0.95}]`), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldBaselineFile := policyTestBaselineFile
	oldCandidateFile := policyTestCandidateFile
	oldMatrix := policyTestMatrix
	oldFlag := policyTestCompareFailOnStricter
	defer func() {
		policyTestBaselineFile = oldBaselineFile
		policyTestCandidateFile = oldCandidateFile
		policyTestMatrix = oldMatrix
		policyTestCompareFailOnStricter = oldFlag
	}()

	policyTestBaselineFile = baselinePath
	policyTestCandidateFile = candidatePath
	policyTestMatrix = matrixPath
	policyTestCompareFailOnStricter = true

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected compare-fail-on-stricter to fail")
	}
}

func TestPolicyTestCmd_CompareFailOnLooser(t *testing.T) {
	tmpDir := t.TempDir()
	baselinePath := filepath.Join(tmpDir, "baseline.policy")
	if err := os.WriteFile(baselinePath, []byte(`
rule "block-high" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`), 0o644); err != nil {
		t.Fatalf("failed to write baseline policy file: %v", err)
	}
	candidatePath := filepath.Join(tmpDir, "candidate.policy")
	if err := os.WriteFile(candidatePath, []byte(`
rule "allow-all" {
    when { risk_score >= 0.0 }
    then { approve() }
}
`), 0o644); err != nil {
		t.Fatalf("failed to write candidate policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	if err := os.WriteFile(matrixPath, []byte(`[{"name":"high","risk_score":0.95}]`), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldBaselineFile := policyTestBaselineFile
	oldCandidateFile := policyTestCandidateFile
	oldMatrix := policyTestMatrix
	oldFlag := policyTestCompareFailOnLooser
	defer func() {
		policyTestBaselineFile = oldBaselineFile
		policyTestCandidateFile = oldCandidateFile
		policyTestMatrix = oldMatrix
		policyTestCompareFailOnLooser = oldFlag
	}()

	policyTestBaselineFile = baselinePath
	policyTestCandidateFile = candidatePath
	policyTestMatrix = matrixPath
	policyTestCompareFailOnLooser = true

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected compare-fail-on-looser to fail")
	}
}

func TestPolicyTestCmd_CompareMaxStricter(t *testing.T) {
	tmpDir := t.TempDir()
	baselinePath := filepath.Join(tmpDir, "baseline.policy")
	if err := os.WriteFile(baselinePath, []byte(`
rule "allow-all" {
    when { risk_score >= 0.0 }
    then { approve() }
}
`), 0o644); err != nil {
		t.Fatalf("failed to write baseline policy file: %v", err)
	}
	candidatePath := filepath.Join(tmpDir, "candidate.policy")
	if err := os.WriteFile(candidatePath, []byte(`
rule "block-high" {
    when { risk_score >= 0.8 }
    then { block(reason: "too risky") }
}
`), 0o644); err != nil {
		t.Fatalf("failed to write candidate policy file: %v", err)
	}
	matrixPath := filepath.Join(tmpDir, "matrix.json")
	if err := os.WriteFile(matrixPath, []byte(`[{"name":"high","risk_score":0.95}]`), 0o644); err != nil {
		t.Fatalf("failed to write matrix file: %v", err)
	}

	oldBaselineFile := policyTestBaselineFile
	oldCandidateFile := policyTestCandidateFile
	oldMatrix := policyTestMatrix
	oldMax := policyTestCompareMaxStricter
	defer func() {
		policyTestBaselineFile = oldBaselineFile
		policyTestCandidateFile = oldCandidateFile
		policyTestMatrix = oldMatrix
		policyTestCompareMaxStricter = oldMax
	}()

	policyTestBaselineFile = baselinePath
	policyTestCandidateFile = candidatePath
	policyTestMatrix = matrixPath
	policyTestCompareMaxStricter = 0

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected compare-max-stricter threshold to fail")
	}
}

func TestPolicyTestCmd_CompareThresholdFlagsRequireCompareMode(t *testing.T) {
	oldBaselineFile := policyTestBaselineFile
	oldBaselineDir := policyTestBaselineDir
	oldCandidateFile := policyTestCandidateFile
	oldCandidateDir := policyTestCandidateDir
	oldMatrix := policyTestMatrix
	oldMax := policyTestCompareMaxStricter
	defer func() {
		policyTestBaselineFile = oldBaselineFile
		policyTestBaselineDir = oldBaselineDir
		policyTestCandidateFile = oldCandidateFile
		policyTestCandidateDir = oldCandidateDir
		policyTestMatrix = oldMatrix
		policyTestCompareMaxStricter = oldMax
	}()

	policyTestBaselineFile = ""
	policyTestBaselineDir = ""
	policyTestCandidateFile = ""
	policyTestCandidateDir = ""
	policyTestMatrix = ""
	policyTestCompareMaxStricter = 0

	err := runPolicyTest(policyTestCmd, nil)
	if err == nil {
		t.Fatalf("expected compare threshold flags without compare mode to fail")
	}
}
