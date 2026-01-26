package ciapproval

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Enabled {
		t.Error("Default config should have Enabled=false")
	}
	if cfg.AutoApprove {
		t.Error("Default config should have AutoApprove=false")
	}
	if cfg.MaxRiskScore != 0.3 {
		t.Errorf("MaxRiskScore = %f, want 0.3", cfg.MaxRiskScore)
	}
	if !cfg.BlockBreakingChanges {
		t.Error("Default config should block breaking changes")
	}
	if !cfg.BlockSecurityChanges {
		t.Error("Default config should block security changes")
	}
	if len(cfg.AllowedBumpTypes) != 1 || cfg.AllowedBumpTypes[0] != "patch" {
		t.Errorf("AllowedBumpTypes = %v, want [patch]", cfg.AllowedBumpTypes)
	}
}

func TestFromEnvironment(t *testing.T) {
	// Save and restore environment
	origCI := os.Getenv("CI")
	origAutoApprove := os.Getenv("RELICTA_AUTO_APPROVE")
	origMaxRisk := os.Getenv("RELICTA_MAX_AUTO_APPROVE_RISK")
	defer func() {
		os.Setenv("CI", origCI)
		os.Setenv("RELICTA_AUTO_APPROVE", origAutoApprove)
		os.Setenv("RELICTA_MAX_AUTO_APPROVE_RISK", origMaxRisk)
	}()

	os.Setenv("CI", "true")
	os.Setenv("RELICTA_AUTO_APPROVE", "true")
	os.Setenv("RELICTA_MAX_AUTO_APPROVE_RISK", "0.5")

	cfg := FromEnvironment()

	if !cfg.Enabled {
		t.Error("Should be enabled in CI environment")
	}
	if !cfg.AutoApprove {
		t.Error("AutoApprove should be true from env")
	}
	if cfg.MaxRiskScore != 0.5 {
		t.Errorf("MaxRiskScore = %f, want 0.5", cfg.MaxRiskScore)
	}
}

func TestFromEnvironment_AllowedBumpTypes(t *testing.T) {
	origVal := os.Getenv("RELICTA_ALLOWED_BUMP_TYPES")
	defer os.Setenv("RELICTA_ALLOWED_BUMP_TYPES", origVal)

	os.Setenv("RELICTA_ALLOWED_BUMP_TYPES", "patch,minor")

	cfg := FromEnvironment()

	if len(cfg.AllowedBumpTypes) != 2 {
		t.Errorf("AllowedBumpTypes length = %d, want 2", len(cfg.AllowedBumpTypes))
	}
	if cfg.AllowedBumpTypes[0] != "patch" || cfg.AllowedBumpTypes[1] != "minor" {
		t.Errorf("AllowedBumpTypes = %v, want [patch minor]", cfg.AllowedBumpTypes)
	}
}

func TestApprover_Evaluate_Disabled(t *testing.T) {
	cfg := &Config{
		Enabled:     false,
		AutoApprove: true,
	}
	approver := New(cfg)

	req := &ApprovalRequest{
		ReleaseID: "rel-123",
		Version:   "1.0.1",
		BumpType:  "patch",
		RiskScore: 0.1,
	}

	result, err := approver.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Approved {
		t.Error("Should not be approved when CI approval is disabled")
	}
	if result.Decision != cgp.DecisionApprovalRequired {
		t.Errorf("Decision = %s, want %s", result.Decision, cgp.DecisionApprovalRequired)
	}
}

func TestApprover_Evaluate_Success(t *testing.T) {
	cfg := &Config{
		Enabled:              true,
		AutoApprove:          true,
		MaxRiskScore:         0.3,
		AllowedBumpTypes:     []string{"patch", "minor"},
		AllowedBranches:      []string{"main", "master"},
		BlockBreakingChanges: true,
		BlockSecurityChanges: true,
		TrustedActor:         "ci:test",
		ApprovalTimeout:      time.Hour,
	}
	approver := New(cfg)

	req := &ApprovalRequest{
		ReleaseID:          "rel-123",
		Version:            "1.0.1",
		BumpType:           "patch",
		Branch:             "main",
		RiskScore:          0.1,
		HasBreakingChanges: false,
		HasSecurityChanges: false,
		Timestamp:          time.Now(),
	}

	result, err := approver.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if !result.Approved {
		t.Errorf("Should be approved, reasons: %v", result.Reasons)
	}
	if result.Decision != cgp.DecisionApproved {
		t.Errorf("Decision = %s, want %s", result.Decision, cgp.DecisionApproved)
	}
	if result.Actor.ID != "ci:test" {
		t.Errorf("Actor.ID = %s, want ci:test", result.Actor.ID)
	}
}

func TestApprover_Evaluate_RiskTooHigh(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		AutoApprove:  true,
		MaxRiskScore: 0.3,
	}
	approver := New(cfg)

	req := &ApprovalRequest{
		ReleaseID: "rel-123",
		BumpType:  "patch",
		RiskScore: 0.5, // Above threshold
	}

	result, err := approver.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Approved {
		t.Error("Should not be approved when risk is too high")
	}

	foundReason := false
	for _, r := range result.Reasons {
		if contains(r, "Risk score") && contains(r, "exceeds") {
			foundReason = true
			break
		}
	}
	if !foundReason {
		t.Errorf("Expected risk score reason, got: %v", result.Reasons)
	}
}

func TestApprover_Evaluate_BumpTypeNotAllowed(t *testing.T) {
	cfg := &Config{
		Enabled:          true,
		AutoApprove:      true,
		MaxRiskScore:     0.5,
		AllowedBumpTypes: []string{"patch"},
	}
	approver := New(cfg)

	req := &ApprovalRequest{
		ReleaseID: "rel-123",
		BumpType:  "major", // Not in allowed list
		RiskScore: 0.1,
	}

	result, err := approver.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Approved {
		t.Error("Should not be approved when bump type is not allowed")
	}
}

func TestApprover_Evaluate_BranchNotAllowed(t *testing.T) {
	cfg := &Config{
		Enabled:         true,
		AutoApprove:     true,
		MaxRiskScore:    0.5,
		AllowedBranches: []string{"main", "master"},
	}
	approver := New(cfg)

	req := &ApprovalRequest{
		ReleaseID: "rel-123",
		BumpType:  "patch",
		Branch:    "feature/test", // Not in allowed list
		RiskScore: 0.1,
	}

	result, err := approver.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Approved {
		t.Error("Should not be approved when branch is not allowed")
	}
}

func TestApprover_Evaluate_BreakingChangesBlocked(t *testing.T) {
	cfg := &Config{
		Enabled:              true,
		AutoApprove:          true,
		MaxRiskScore:         0.5,
		BlockBreakingChanges: true,
	}
	approver := New(cfg)

	req := &ApprovalRequest{
		ReleaseID:          "rel-123",
		BumpType:           "major",
		RiskScore:          0.1,
		HasBreakingChanges: true,
	}

	result, err := approver.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Approved {
		t.Error("Should not be approved when breaking changes are blocked")
	}
}

func TestApprover_Evaluate_SecurityChangesBlocked(t *testing.T) {
	cfg := &Config{
		Enabled:              true,
		AutoApprove:          true,
		MaxRiskScore:         0.5,
		BlockSecurityChanges: true,
	}
	approver := New(cfg)

	req := &ApprovalRequest{
		ReleaseID:          "rel-123",
		BumpType:           "patch",
		RiskScore:          0.1,
		HasSecurityChanges: true,
	}

	result, err := approver.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Approved {
		t.Error("Should not be approved when security changes are blocked")
	}
}

func TestApprover_Evaluate_SignedCommitsRequired(t *testing.T) {
	cfg := &Config{
		Enabled:              true,
		AutoApprove:          true,
		MaxRiskScore:         0.5,
		RequireSignedCommits: true,
	}
	approver := New(cfg)

	req := &ApprovalRequest{
		ReleaseID:     "rel-123",
		BumpType:      "patch",
		RiskScore:     0.1,
		CommitCount:   10,
		SignedCommits: 5, // Only half signed
	}

	result, err := approver.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Approved {
		t.Error("Should not be approved when not all commits are signed")
	}
}

func TestApprover_Evaluate_AuditLog(t *testing.T) {
	tmpDir := t.TempDir()
	auditPath := filepath.Join(tmpDir, "audit.json")

	cfg := &Config{
		Enabled:      true,
		AutoApprove:  true,
		MaxRiskScore: 0.5,
		AuditLogPath: auditPath,
		TrustedActor: "ci:test",
	}
	approver := New(cfg)

	req := &ApprovalRequest{
		ReleaseID: "rel-123",
		BumpType:  "patch",
		RiskScore: 0.1,
	}

	_, err := approver.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Check audit log was written
	if _, err := os.Stat(auditPath); os.IsNotExist(err) {
		t.Error("Audit log file was not created")
	}
}

func TestApprover_Evaluate_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	auditPath := filepath.Join(tmpDir, "audit.json")

	cfg := &Config{
		Enabled:      true,
		AutoApprove:  true,
		MaxRiskScore: 0.5,
		AuditLogPath: auditPath,
		DryRun:       true,
	}
	approver := New(cfg)

	req := &ApprovalRequest{
		ReleaseID: "rel-123",
		BumpType:  "patch",
		RiskScore: 0.1,
	}

	result, err := approver.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if !result.DryRun {
		t.Error("Result should indicate dry run")
	}

	// Audit log should NOT be written in dry run
	if _, err := os.Stat(auditPath); !os.IsNotExist(err) {
		t.Error("Audit log should not be written in dry run")
	}
}

func TestMatchBranch(t *testing.T) {
	tests := []struct {
		branch  string
		pattern string
		want    bool
	}{
		{"main", "main", true},
		{"master", "main", false},
		{"release/v1.0", "release/*", true},
		{"release/v2.0", "release/*", true},
		{"feature/test", "release/*", false},
		{"refs/heads/main", "main", true},
		{"main", "main*", true},
		{"main-v2", "main*", true},
	}

	for _, tt := range tests {
		t.Run(tt.branch+"_"+tt.pattern, func(t *testing.T) {
			got := matchBranch(tt.branch, tt.pattern)
			if got != tt.want {
				t.Errorf("matchBranch(%s, %s) = %v, want %v", tt.branch, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestPreApprovedPolicy_Evaluate(t *testing.T) {
	policy := &PreApprovedPolicy{
		Name:        "patch-releases",
		Description: "Auto-approve patch releases",
		Conditions: []PolicyCondition{
			{Field: "bump_type", Operator: "eq", Value: "patch"},
			{Field: "risk_score", Operator: "lte", Value: 0.3},
			{Field: "has_breaking_changes", Operator: "eq", Value: false},
		},
		Enabled:  true,
		Priority: 100,
	}

	// Should match
	req1 := &ApprovalRequest{
		BumpType:           "patch",
		RiskScore:          0.2,
		HasBreakingChanges: false,
	}
	if !EvaluatePolicy(policy, req1) {
		t.Error("Policy should match for low-risk patch release")
	}

	// Should not match - bump type mismatch
	req2 := &ApprovalRequest{
		BumpType:           "minor",
		RiskScore:          0.2,
		HasBreakingChanges: false,
	}
	if EvaluatePolicy(policy, req2) {
		t.Error("Policy should not match for minor release")
	}

	// Should not match - risk too high
	req3 := &ApprovalRequest{
		BumpType:           "patch",
		RiskScore:          0.5,
		HasBreakingChanges: false,
	}
	if EvaluatePolicy(policy, req3) {
		t.Error("Policy should not match for high-risk release")
	}

	// Should not match - has breaking changes
	req4 := &ApprovalRequest{
		BumpType:           "patch",
		RiskScore:          0.1,
		HasBreakingChanges: true,
	}
	if EvaluatePolicy(policy, req4) {
		t.Error("Policy should not match with breaking changes")
	}
}

func TestPreApprovedPolicy_Disabled(t *testing.T) {
	policy := &PreApprovedPolicy{
		Name:    "disabled-policy",
		Enabled: false,
		Conditions: []PolicyCondition{
			{Field: "bump_type", Operator: "eq", Value: "patch"},
		},
	}

	req := &ApprovalRequest{
		BumpType: "patch",
	}

	if EvaluatePolicy(policy, req) {
		t.Error("Disabled policy should not match")
	}
}

func TestCondition_In(t *testing.T) {
	policy := &PreApprovedPolicy{
		Name:    "multi-bump",
		Enabled: true,
		Conditions: []PolicyCondition{
			{Field: "bump_type", Operator: "in", Value: []string{"patch", "minor"}},
		},
	}

	// Patch should match
	if !EvaluatePolicy(policy, &ApprovalRequest{BumpType: "patch"}) {
		t.Error("patch should match 'in' condition")
	}

	// Minor should match
	if !EvaluatePolicy(policy, &ApprovalRequest{BumpType: "minor"}) {
		t.Error("minor should match 'in' condition")
	}

	// Major should not match
	if EvaluatePolicy(policy, &ApprovalRequest{BumpType: "major"}) {
		t.Error("major should not match 'in' condition")
	}
}

func TestGetCIContext_GitHub(t *testing.T) {
	// Save and restore environment
	orig := map[string]string{}
	envVars := []string{"GITHUB_ACTIONS", "GITHUB_REPOSITORY", "GITHUB_REF", "GITHUB_SHA"}
	for _, env := range envVars {
		orig[env] = os.Getenv(env)
	}
	defer func() {
		for env, val := range orig {
			os.Setenv(env, val)
		}
	}()

	os.Setenv("GITHUB_ACTIONS", "true")
	os.Setenv("GITHUB_REPOSITORY", "owner/repo")
	os.Setenv("GITHUB_REF", "refs/heads/main")
	os.Setenv("GITHUB_SHA", "abc123")

	ctx := GetCIContext()

	if ctx["provider"] != "github-actions" {
		t.Errorf("provider = %s, want github-actions", ctx["provider"])
	}
	if ctx["repository"] != "owner/repo" {
		t.Errorf("repository = %s, want owner/repo", ctx["repository"])
	}
	if ctx["ref"] != "refs/heads/main" {
		t.Errorf("ref = %s, want refs/heads/main", ctx["ref"])
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"1", true},
		{"yes", true},
		{"YES", true},
		{"on", true},
		{"false", false},
		{"FALSE", false},
		{"0", false},
		{"no", false},
		{"off", false},
		{"", false},
		{"random", false},
	}

	for _, tt := range tests {
		got := parseBool(tt.input)
		if got != tt.want {
			t.Errorf("parseBool(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseList(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{"a, b, c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{"single", []string{"single"}},
		{"", nil},
	}

	for _, tt := range tests {
		got := parseList(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseList(%q) length = %d, want %d", tt.input, len(got), len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseList(%q)[%d] = %s, want %s", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsAt(s, substr)
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestNewFromEnvironment(t *testing.T) {
	// Save and restore environment
	origCI := os.Getenv("CI")
	defer os.Setenv("CI", origCI)

	os.Setenv("CI", "true")

	approver := NewFromEnvironment()
	if approver == nil {
		t.Fatal("NewFromEnvironment should not return nil")
	}
	if approver.config == nil {
		t.Error("config should not be nil")
	}
}

func TestApprover_Config(t *testing.T) {
	cfg := &Config{
		Enabled:      true,
		MaxRiskScore: 0.5,
	}
	approver := New(cfg)

	got := approver.Config()
	if got != cfg {
		t.Error("Config should return the same config instance")
	}
	if got.MaxRiskScore != 0.5 {
		t.Errorf("MaxRiskScore = %f, want 0.5", got.MaxRiskScore)
	}
}

func TestApprover_IsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{"enabled", true, true},
		{"disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Enabled: tt.enabled}
			approver := New(cfg)
			if got := approver.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApprover_CanAutoApprove(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		autoApprove bool
		want        bool
	}{
		{"both enabled", true, true, true},
		{"enabled only", true, false, false},
		{"auto only", false, true, false},
		{"both disabled", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Enabled: tt.enabled, AutoApprove: tt.autoApprove}
			approver := New(cfg)
			if got := approver.CanAutoApprove(); got != tt.want {
				t.Errorf("CanAutoApprove() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultPreApprovedPolicies(t *testing.T) {
	policies := DefaultPreApprovedPolicies()

	if len(policies) == 0 {
		t.Error("DefaultPreApprovedPolicies should return at least one policy")
	}

	// Check that policies have required fields
	for _, policy := range policies {
		if policy.Name == "" {
			t.Error("Policy name should not be empty")
		}
		if len(policy.Conditions) == 0 {
			t.Errorf("Policy %s should have conditions", policy.Name)
		}
	}
}

func TestEvaluateCondition_GT(t *testing.T) {
	policy := &PreApprovedPolicy{
		Name:    "high-risk",
		Enabled: true,
		Conditions: []PolicyCondition{
			{Field: "risk_score", Operator: "gt", Value: 0.5},
		},
	}

	// Should not match - 0.3 is not > 0.5
	if EvaluatePolicy(policy, &ApprovalRequest{RiskScore: 0.3}) {
		t.Error("0.3 should not be > 0.5")
	}

	// Should match - 0.7 is > 0.5
	if !EvaluatePolicy(policy, &ApprovalRequest{RiskScore: 0.7}) {
		t.Error("0.7 should be > 0.5")
	}
}

func TestEvaluateCondition_GTE(t *testing.T) {
	policy := &PreApprovedPolicy{
		Name:    "threshold",
		Enabled: true,
		Conditions: []PolicyCondition{
			{Field: "risk_score", Operator: "gte", Value: 0.5},
		},
	}

	// Should not match - 0.4 is not >= 0.5
	if EvaluatePolicy(policy, &ApprovalRequest{RiskScore: 0.4}) {
		t.Error("0.4 should not be >= 0.5")
	}

	// Should match - 0.5 is >= 0.5
	if !EvaluatePolicy(policy, &ApprovalRequest{RiskScore: 0.5}) {
		t.Error("0.5 should be >= 0.5")
	}
}

func TestEvaluateCondition_LT(t *testing.T) {
	policy := &PreApprovedPolicy{
		Name:    "low-risk",
		Enabled: true,
		Conditions: []PolicyCondition{
			{Field: "risk_score", Operator: "lt", Value: 0.3},
		},
	}

	// Should match - 0.1 is < 0.3
	if !EvaluatePolicy(policy, &ApprovalRequest{RiskScore: 0.1}) {
		t.Error("0.1 should be < 0.3")
	}

	// Should not match - 0.3 is not < 0.3
	if EvaluatePolicy(policy, &ApprovalRequest{RiskScore: 0.3}) {
		t.Error("0.3 should not be < 0.3")
	}
}

func TestEvaluateCondition_NE(t *testing.T) {
	policy := &PreApprovedPolicy{
		Name:    "not-major",
		Enabled: true,
		Conditions: []PolicyCondition{
			{Field: "bump_type", Operator: "ne", Value: "major"},
		},
	}

	// Should match - patch != major
	if !EvaluatePolicy(policy, &ApprovalRequest{BumpType: "patch"}) {
		t.Error("patch should not equal major")
	}

	// Should not match - major == major
	if EvaluatePolicy(policy, &ApprovalRequest{BumpType: "major"}) {
		t.Error("major should equal major")
	}
}

func TestEvaluateCondition_UnknownField(t *testing.T) {
	policy := &PreApprovedPolicy{
		Name:    "unknown",
		Enabled: true,
		Conditions: []PolicyCondition{
			{Field: "unknown_field", Operator: "eq", Value: "test"},
		},
	}

	// Should not match - unknown field
	if EvaluatePolicy(policy, &ApprovalRequest{BumpType: "patch"}) {
		t.Error("unknown field should not match")
	}
}

func TestEvaluateCondition_NumericOnNumericField(t *testing.T) {
	policy := &PreApprovedPolicy{
		Name:    "numeric-comparison",
		Enabled: true,
		Conditions: []PolicyCondition{
			{Field: "risk_score", Operator: "lte", Value: 0.5},
		},
	}

	// Should match - 0.3 <= 0.5
	if !EvaluatePolicy(policy, &ApprovalRequest{RiskScore: 0.3}) {
		t.Error("0.3 <= 0.5 should match")
	}
}

func TestToFloat64_ciapproval(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  float64
	}{
		{"float64", 1.5, 1.5},
		{"float32", float32(2.5), 2.5},
		{"int", 10, 10.0},
		{"int64", int64(20), 20.0},
		{"string", "invalid", 0}, // returns 0 for invalid types
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toFloat64(tt.input)
			if got != tt.want {
				t.Errorf("toFloat64(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValueIn_ciapproval(t *testing.T) {
	// Test with []any
	list := []any{"patch", "minor"}
	if !valueIn("patch", list) {
		t.Error("\"patch\" should be in list")
	}
	if valueIn("major", list) {
		t.Error("\"major\" should not be in list")
	}

	// Test with []string
	strList := []string{"a", "b"}
	if !valueIn("a", strList) {
		t.Error("\"a\" should be in list")
	}
	if valueIn("c", strList) {
		t.Error("\"c\" should not be in list")
	}
}

func TestNew_NilConfig(t *testing.T) {
	// Save and restore environment
	origCI := os.Getenv("CI")
	defer os.Setenv("CI", origCI)

	os.Setenv("CI", "true")

	// New with nil config should create from environment
	approver := New(nil)
	if approver == nil {
		t.Fatal("New(nil) should not return nil")
	}
	if approver.config == nil {
		t.Error("config should not be nil when created from environment")
	}
}

func TestFromEnvironment_AllOptions(t *testing.T) {
	// Save and restore all environment variables
	vars := map[string]string{
		"CI":                             "",
		"RELICTA_AUTO_APPROVE":           "",
		"RELICTA_MAX_AUTO_APPROVE_RISK":  "",
		"RELICTA_BLOCK_BREAKING":         "",
		"RELICTA_BLOCK_SECURITY":         "",
		"RELICTA_ALLOWED_BUMP_TYPES":     "",
		"RELICTA_ALLOWED_BRANCHES":       "",
		"RELICTA_REQUIRE_SIGNED_COMMITS": "",
		"RELICTA_CI_AUDIT_LOG":           "",
		"RELICTA_DRY_RUN":                "",
	}
	for k := range vars {
		vars[k] = os.Getenv(k)
	}
	defer func() {
		for k, v := range vars {
			os.Setenv(k, v)
		}
	}()

	os.Setenv("CI", "true")
	os.Setenv("RELICTA_AUTO_APPROVE", "true")
	os.Setenv("RELICTA_MAX_AUTO_APPROVE_RISK", "0.4")
	os.Setenv("RELICTA_BLOCK_BREAKING", "false")
	os.Setenv("RELICTA_BLOCK_SECURITY", "false")
	os.Setenv("RELICTA_ALLOWED_BUMP_TYPES", "patch,minor,major")
	os.Setenv("RELICTA_ALLOWED_BRANCHES", "main,develop")
	os.Setenv("RELICTA_REQUIRE_SIGNED_COMMITS", "true")
	os.Setenv("RELICTA_CI_AUDIT_LOG", "/tmp/audit.log")
	os.Setenv("RELICTA_DRY_RUN", "true")

	cfg := FromEnvironment()

	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
	if !cfg.AutoApprove {
		t.Error("AutoApprove should be true")
	}
	if cfg.MaxRiskScore != 0.4 {
		t.Errorf("MaxRiskScore = %f, want 0.4", cfg.MaxRiskScore)
	}
	if cfg.BlockBreakingChanges {
		t.Error("BlockBreakingChanges should be false")
	}
	if cfg.BlockSecurityChanges {
		t.Error("BlockSecurityChanges should be false")
	}
	if len(cfg.AllowedBumpTypes) != 3 {
		t.Errorf("AllowedBumpTypes = %v, want 3 items", cfg.AllowedBumpTypes)
	}
	if len(cfg.AllowedBranches) != 2 {
		t.Errorf("AllowedBranches = %v, want 2 items", cfg.AllowedBranches)
	}
	if !cfg.RequireSignedCommits {
		t.Error("RequireSignedCommits should be true")
	}
	if cfg.AuditLogPath != "/tmp/audit.log" {
		t.Errorf("AuditLogPath = %s, want /tmp/audit.log", cfg.AuditLogPath)
	}
	if !cfg.DryRun {
		t.Error("DryRun should be true")
	}
}

func TestIsCI(t *testing.T) {
	// Save and restore environment
	vars := map[string]string{
		"CI":                  "",
		"GITHUB_ACTIONS":      "",
		"GITLAB_CI":           "",
		"CIRCLECI":            "",
		"JENKINS_URL":         "",
		"TRAVIS":              "",
		"BITBUCKET_PIPELINES": "",
	}
	for k := range vars {
		vars[k] = os.Getenv(k)
	}
	defer func() {
		for k, v := range vars {
			os.Setenv(k, v)
		}
	}()

	// Clear all CI indicators
	for k := range vars {
		os.Unsetenv(k)
	}

	if isCI() {
		t.Error("isCI should be false when no CI env vars are set")
	}

	// Test each CI environment
	testCases := []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI"}
	for _, envVar := range testCases {
		// Clear all
		for k := range vars {
			os.Unsetenv(k)
		}
		os.Setenv(envVar, "true")

		if !isCI() {
			t.Errorf("isCI should be true when %s is set", envVar)
		}
	}
}

func TestGetCIContext_GitLab(t *testing.T) {
	// Save and restore environment
	vars := map[string]string{
		"GITHUB_ACTIONS":     "",
		"GITLAB_CI":          "",
		"CI_PROJECT_PATH":    "",
		"CI_COMMIT_REF_NAME": "",
		"CI_COMMIT_SHA":      "",
	}
	for k := range vars {
		vars[k] = os.Getenv(k)
	}
	defer func() {
		for k, v := range vars {
			os.Setenv(k, v)
		}
	}()

	os.Unsetenv("GITHUB_ACTIONS")
	os.Setenv("GITLAB_CI", "true")
	os.Setenv("CI_PROJECT_PATH", "group/project")
	os.Setenv("CI_COMMIT_REF_NAME", "main")
	os.Setenv("CI_COMMIT_SHA", "def456")

	ctx := GetCIContext()

	// The implementation returns "gitlab" not "gitlab-ci"
	if ctx["provider"] != "gitlab" {
		t.Errorf("provider = %s, want gitlab", ctx["provider"])
	}
	if ctx["repository"] != "group/project" {
		t.Errorf("repository = %s, want group/project", ctx["repository"])
	}
}

func TestDetectCIActor_GitHub(t *testing.T) {
	vars := map[string]string{
		"GITHUB_ACTIONS":          "",
		"GITHUB_ACTOR":            "",
		"GITHUB_TRIGGERING_ACTOR": "",
		"GITLAB_CI":               "",
	}
	for k := range vars {
		vars[k] = os.Getenv(k)
	}
	defer func() {
		for k, v := range vars {
			os.Setenv(k, v)
		}
	}()

	os.Setenv("GITHUB_ACTIONS", "true")
	os.Setenv("GITHUB_ACTOR", "test-user")
	os.Setenv("GITHUB_TRIGGERING_ACTOR", "trigger-user")
	os.Unsetenv("GITLAB_CI")

	actor := detectCIActor()

	if actor == "" {
		t.Error("actor should not be empty")
	}
}

func TestDetectCIActor_GitLab(t *testing.T) {
	vars := map[string]string{
		"GITHUB_ACTIONS":    "",
		"GITLAB_CI":         "",
		"GITLAB_USER_LOGIN": "",
	}
	for k := range vars {
		vars[k] = os.Getenv(k)
	}
	defer func() {
		for k, v := range vars {
			os.Setenv(k, v)
		}
	}()

	os.Unsetenv("GITHUB_ACTIONS")
	os.Setenv("GITLAB_CI", "true")
	os.Setenv("GITLAB_USER_LOGIN", "gitlab-user")

	actor := detectCIActor()

	// The implementation formats the actor as "ci:gitlab:username"
	if actor != "ci:gitlab:gitlab-user" {
		t.Errorf("actor = %s, want ci:gitlab:gitlab-user", actor)
	}
}

func TestApprover_Evaluate_AutoApproveDisabled(t *testing.T) {
	cfg := &Config{
		Enabled:     true,
		AutoApprove: false, // Auto-approve disabled
	}
	approver := New(cfg)

	req := &ApprovalRequest{
		ReleaseID: "rel-123",
		BumpType:  "patch",
		RiskScore: 0.1,
	}

	result, err := approver.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Approved {
		t.Error("Should not be approved when auto-approve is disabled")
	}
}

func TestApprover_Evaluate_SignedCommitsAllSigned(t *testing.T) {
	cfg := &Config{
		Enabled:              true,
		AutoApprove:          true,
		MaxRiskScore:         0.5,
		RequireSignedCommits: true,
	}
	approver := New(cfg)

	req := &ApprovalRequest{
		ReleaseID:     "rel-123",
		BumpType:      "patch",
		RiskScore:     0.1,
		CommitCount:   10,
		SignedCommits: 10, // All signed
	}

	result, err := approver.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if !result.Approved {
		t.Errorf("Should be approved when all commits are signed, reasons: %v", result.Reasons)
	}
}
