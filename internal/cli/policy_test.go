package cli

import (
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
