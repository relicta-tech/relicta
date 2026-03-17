package dsl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoader_LoadFile(t *testing.T) {
	// Create a temporary policy file
	tmpDir, err := os.MkdirTemp("", "policy_loader_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	policyContent := `
rule "high-risk-release" {
    priority = 100
    description = "Block high-risk releases"

    when {
        risk_score > 0.8
    }

    then {
        block(reason: "High risk detected")
    }
}
`
	policyPath := filepath.Join(tmpDir, "security.policy")
	err = os.WriteFile(policyPath, []byte(policyContent), 0644)
	require.NoError(t, err)

	loader := NewLoader(LoaderOptions{})
	policy, err := loader.LoadFile(policyPath)
	require.NoError(t, err)

	assert.Equal(t, "security", policy.Name)
	assert.Len(t, policy.Rules, 1)
	assert.Equal(t, "high_risk_release", policy.Rules[0].ID)
	assert.Equal(t, 100, policy.Rules[0].Priority)
}

func TestLoader_LoadDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "policy_loader_dir_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create multiple policy files
	policy1 := `
rule "rule-1" {
    when { risk_score > 0.5 }
    then { require_approval(role: "reviewer") }
}
`
	policy2 := `
rule "rule-2" {
    when { has_breaking_changes == true }
    then { block(reason: "Breaking changes detected") }
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "policy1.policy"), []byte(policy1), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "policy2.cgp"), []byte(policy2), 0644)
	require.NoError(t, err)

	loader := NewLoader(LoaderOptions{})
	result, err := loader.LoadDir(tmpDir)
	require.NoError(t, err)

	assert.Len(t, result.Policies, 2)
	assert.Len(t, result.Errors, 0)
}

func TestLoader_LoadDir_Recursive(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "policy_loader_recursive_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	err = os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	// Create policy files in both directories
	policy1 := `rule "root-rule" { when { risk_score > 0 } then { approve() } }`
	policy2 := `rule "sub-rule" { when { risk_score < 1 } then { block(reason: "test") } }`

	err = os.WriteFile(filepath.Join(tmpDir, "root.policy"), []byte(policy1), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subDir, "sub.policy"), []byte(policy2), 0644)
	require.NoError(t, err)

	// Non-recursive should only find root policy
	loader := NewLoader(LoaderOptions{Recursive: false})
	result, err := loader.LoadDir(tmpDir)
	require.NoError(t, err)
	assert.Len(t, result.Policies, 1)

	// Recursive should find both policies
	loader = NewLoader(LoaderOptions{Recursive: true})
	result, err = loader.LoadDir(tmpDir)
	require.NoError(t, err)
	assert.Len(t, result.Policies, 2)
}

func TestLoader_LoadDir_NonExistent(t *testing.T) {
	loader := NewLoader(LoaderOptions{})
	result, err := loader.LoadDir("/nonexistent/path")
	require.NoError(t, err) // Should not error, just return empty result
	assert.Len(t, result.Policies, 0)
	assert.Len(t, result.Errors, 0)
}

func TestLoader_LoadDir_IgnoreErrors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "policy_loader_ignore_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create valid and invalid policy files
	validPolicy := `rule "valid" { when { risk_score > 0 } then { approve() } }`
	invalidPolicy := `rule "invalid" { when { syntax error here }`

	err = os.WriteFile(filepath.Join(tmpDir, "valid.policy"), []byte(validPolicy), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "invalid.policy"), []byte(invalidPolicy), 0644)
	require.NoError(t, err)

	// Without IgnoreErrors, should fail on first error
	loader := NewLoader(LoaderOptions{IgnoreErrors: false})
	result, err := loader.LoadDir(tmpDir)
	assert.Error(t, err)
	assert.GreaterOrEqual(t, len(result.Errors), 1)

	// With IgnoreErrors, should continue and load valid file
	loader = NewLoader(LoaderOptions{IgnoreErrors: true})
	result, err = loader.LoadDir(tmpDir)
	require.NoError(t, err)
	assert.Len(t, result.Policies, 1)
	assert.Len(t, result.Errors, 1)
}

func TestLoader_LoadString(t *testing.T) {
	source := `
rule "test-rule" {
    priority = 50
    when { commit_count > 10 }
    then { require_approval(role: "lead") }
}
`
	loader := NewLoader(LoaderOptions{})
	policy, err := loader.LoadString(source, "inline-policy")
	require.NoError(t, err)

	assert.Equal(t, "inline-policy", policy.Name)
	assert.Len(t, policy.Rules, 1)
	assert.Equal(t, "test_rule", policy.Rules[0].ID)
}

func TestValidateFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "policy_validate_file_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Valid policy
	validPolicy := `rule "valid" { when { risk_score > 0 } then { approve() } }`
	validPath := filepath.Join(tmpDir, "valid.policy")
	err = os.WriteFile(validPath, []byte(validPolicy), 0644)
	require.NoError(t, err)

	err = ValidateFile(validPath)
	assert.NoError(t, err)

	// Invalid policy
	invalidPolicy := `rule "invalid" { syntax error }`
	invalidPath := filepath.Join(tmpDir, "invalid.policy")
	err = os.WriteFile(invalidPath, []byte(invalidPolicy), 0644)
	require.NoError(t, err)

	err = ValidateFile(invalidPath)
	assert.Error(t, err)
}

func TestValidateDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "policy_validate_dir_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create mixed valid/invalid policies
	validPolicy := `rule "valid" { when { risk_score > 0 } then { approve() } }`
	invalidPolicy := `rule "invalid" { broken syntax`

	err = os.WriteFile(filepath.Join(tmpDir, "valid.policy"), []byte(validPolicy), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "invalid.policy"), []byte(invalidPolicy), 0644)
	require.NoError(t, err)

	errors, err := ValidateDir(tmpDir)
	require.NoError(t, err)
	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].File, "invalid.policy")
}

func TestValidateString(t *testing.T) {
	// Valid
	err := ValidateString(`rule "test" { when { risk_score > 0 } then { approve() } }`)
	assert.NoError(t, err)

	// Invalid
	err = ValidateString(`rule "test" { broken`)
	assert.Error(t, err)
}

func TestMustLoadDir_Panics(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "policy_must_load_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create invalid policy
	invalidPolicy := `rule "invalid" { broken syntax`
	err = os.WriteFile(filepath.Join(tmpDir, "invalid.policy"), []byte(invalidPolicy), 0644)
	require.NoError(t, err)

	assert.Panics(t, func() {
		MustLoadDir(tmpDir)
	})
}

func TestMustLoadFile_Panics(t *testing.T) {
	assert.Panics(t, func() {
		MustLoadFile("/nonexistent/file.policy")
	})
}

func TestMustLoadDir_Success(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "policy_must_load_success_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	validPolicy := `rule "valid" { when { risk_score > 0 } then { approve() } }`
	err = os.WriteFile(filepath.Join(tmpDir, "valid.policy"), []byte(validPolicy), 0644)
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		policies := MustLoadDir(tmpDir)
		assert.Len(t, policies, 1)
	})
}

func TestIsPolicyFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"policy.policy", true},
		{"policy.POLICY", true},
		{"governance.cgp", true},
		{"governance.CGP", true},
		{"config.yaml", false},
		{"readme.md", false},
		{"script.go", false},
		{".policy", true},
		{".cgp", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, isPolicyFile(tt.path))
		})
	}
}

func TestDefaultPolicyDir(t *testing.T) {
	assert.Equal(t, ".relicta/policies", DefaultPolicyDir())
}

func TestDefaultPolicyPaths(t *testing.T) {
	paths := DefaultPolicyPaths()
	assert.Contains(t, paths, ".relicta/policies")
	assert.Contains(t, paths, ".github/relicta/policies")
	assert.Contains(t, paths, "policies")
}

func TestLoader_LoadDir_EmptyDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "policy_empty_dir_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	loader := NewLoader(LoaderOptions{})
	result, err := loader.LoadDir(tmpDir)
	require.NoError(t, err)
	assert.Len(t, result.Policies, 0)
	assert.Len(t, result.Errors, 0)
}

func TestLoader_LoadDir_IgnoresNonPolicyFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "policy_ignore_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create various files
	err = os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("key: value"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("# Readme"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "valid.policy"), []byte(`rule "test" { when { risk_score > 0 } then { approve() } }`), 0644)
	require.NoError(t, err)

	loader := NewLoader(LoaderOptions{})
	result, err := loader.LoadDir(tmpDir)
	require.NoError(t, err)
	assert.Len(t, result.Policies, 1)
}

func TestLoader_LoadFile_NotFound(t *testing.T) {
	loader := NewLoader(LoaderOptions{})
	_, err := loader.LoadFile("/nonexistent/file.policy")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestLoader_ComplexPolicy(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "policy_complex_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	complexPolicy := `
# Production release governance policy

rule "critical-path-review" {
    priority = 100
    description = "Require senior review for critical path changes"

    when {
        risk_score > 0.6
    }

    then {
        require_approval(role: "senior-engineer")
        add_label(name: "critical-path")
        notify(channel: "slack", target: "#releases")
    }
}

rule "breaking-change-gate" {
    priority = 90
    description = "Block breaking changes without migration plan"

    when {
        has_breaking_changes == true and
        commit_count > 5
    }

    then {
        require_approval(role: "tech-lead")
        require_approval(role: "product-manager")
    }
}

rule "auto-approve-docs" {
    priority = 10
    description = "Auto-approve documentation-only changes"

    when {
        risk_score < 0.2
    }

    then {
        approve()
        add_label(name: "docs-only")
    }
}
`
	policyPath := filepath.Join(tmpDir, "production.policy")
	err = os.WriteFile(policyPath, []byte(complexPolicy), 0644)
	require.NoError(t, err)

	loader := NewLoader(LoaderOptions{})
	policy, err := loader.LoadFile(policyPath)
	require.NoError(t, err)

	assert.Equal(t, "production", policy.Name)
	assert.Len(t, policy.Rules, 3)

	// Verify rules are present
	ruleIDs := make([]string, len(policy.Rules))
	for i, r := range policy.Rules {
		ruleIDs[i] = r.ID
	}
	assert.Contains(t, ruleIDs, "critical_path_review")
	assert.Contains(t, ruleIDs, "breaking_change_gate")
	assert.Contains(t, ruleIDs, "auto_approve_docs")
}
