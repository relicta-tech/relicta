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
