package autoapproval

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/risk"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Enabled {
		t.Error("Enabled should be true by default")
	}
	if cfg.Thresholds.AutoApproveMax != 0.3 {
		t.Errorf("AutoApproveMax = %v, want 0.3", cfg.Thresholds.AutoApproveMax)
	}
	if cfg.Thresholds.RequireReviewMin != 0.5 {
		t.Errorf("RequireReviewMin = %v, want 0.5", cfg.Thresholds.RequireReviewMin)
	}
	if cfg.Thresholds.RejectMin != 0.9 {
		t.Errorf("RejectMin = %v, want 0.9", cfg.Thresholds.RejectMin)
	}
	if len(cfg.Policies) == 0 {
		t.Error("Default policies should not be empty")
	}
}

func TestDefaultPolicies(t *testing.T) {
	policies := DefaultPolicies()

	if len(policies) == 0 {
		t.Fatal("Expected default policies")
	}

	// Check that policies are properly configured
	for _, policy := range policies {
		if policy.ID == "" {
			t.Error("Policy ID should not be empty")
		}
		if policy.Name == "" {
			t.Error("Policy Name should not be empty")
		}
		if len(policy.Conditions) == 0 {
			t.Errorf("Policy %s should have conditions", policy.ID)
		}
	}
}

func TestEvaluator_AutoApproved_LowRisk(t *testing.T) {
	evaluator := New()

	proposal := createTestProposal("patch", cgp.ActorKindCI, cgp.TrustLevelTrusted)
	analysis := createTestAnalysis(0, 0, 0) // No breaking, security, or features
	riskAssessment := &risk.Assessment{
		Score:    0.1,
		Severity: cgp.SeverityLow,
	}

	result, err := evaluator.Evaluate(context.Background(), proposal, analysis, riskAssessment)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionAutoApproved {
		t.Errorf("Decision = %s, want auto_approved", result.Decision)
	}
	if !result.Approved {
		t.Error("Approved should be true")
	}
	if result.MatchedPolicy == nil {
		t.Error("MatchedPolicy should not be nil")
	}
}

func TestEvaluator_RequireReview_HighRisk(t *testing.T) {
	evaluator := New()

	proposal := createTestProposal("minor", cgp.ActorKindHuman, cgp.TrustLevelTrusted)
	analysis := createTestAnalysis(1, 0, 0) // Has features
	riskAssessment := &risk.Assessment{
		Score:    0.6,
		Severity: cgp.SeverityMedium,
	}

	result, err := evaluator.Evaluate(context.Background(), proposal, analysis, riskAssessment)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionRequireReview {
		t.Errorf("Decision = %s, want require_review", result.Decision)
	}
	if result.Approved {
		t.Error("Approved should be false")
	}
	if result.RequiredApprovers == 0 {
		t.Error("RequiredApprovers should be > 0")
	}
}

func TestEvaluator_AutoRejected_VeryHighRisk(t *testing.T) {
	evaluator := New()

	proposal := createTestProposal("major", cgp.ActorKindAgent, cgp.TrustLevelUntrusted)
	analysis := createTestAnalysis(1, 1, 1) // Breaking, security, features
	riskAssessment := &risk.Assessment{
		Score:    0.95,
		Severity: cgp.SeverityCritical,
	}

	result, err := evaluator.Evaluate(context.Background(), proposal, analysis, riskAssessment)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionAutoRejected {
		t.Errorf("Decision = %s, want auto_rejected", result.Decision)
	}
	if result.Approved {
		t.Error("Approved should be false")
	}
}

func TestEvaluator_ExemptionBreakingChanges(t *testing.T) {
	evaluator := New()

	proposal := createTestProposal("minor", cgp.ActorKindCI, cgp.TrustLevelTrusted)
	analysis := createTestAnalysis(0, 1, 0) // Breaking change
	analysis.Breaking = 1
	riskAssessment := &risk.Assessment{
		Score:    0.2,
		Severity: cgp.SeverityLow,
	}

	result, err := evaluator.Evaluate(context.Background(), proposal, analysis, riskAssessment)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionRequireReview {
		t.Errorf("Decision = %s, want require_review (breaking changes exemption)", result.Decision)
	}
	if len(result.ExemptionHits) == 0 {
		t.Error("ExemptionHits should contain breaking changes")
	}
}

func TestEvaluator_ExemptionSecurityChanges(t *testing.T) {
	evaluator := New()

	proposal := createTestProposal("patch", cgp.ActorKindCI, cgp.TrustLevelTrusted)
	analysis := createTestAnalysis(0, 0, 1) // Security change
	analysis.Security = 1
	riskAssessment := &risk.Assessment{
		Score:    0.2,
		Severity: cgp.SeverityLow,
	}

	result, err := evaluator.Evaluate(context.Background(), proposal, analysis, riskAssessment)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionRequireReview {
		t.Errorf("Decision = %s, want require_review (security changes exemption)", result.Decision)
	}
}

func TestEvaluator_ExemptionMajorVersion(t *testing.T) {
	evaluator := New()

	proposal := createTestProposal("major", cgp.ActorKindCI, cgp.TrustLevelTrusted)
	analysis := createTestAnalysis(0, 0, 0)
	riskAssessment := &risk.Assessment{
		Score:    0.2,
		Severity: cgp.SeverityLow,
	}

	result, err := evaluator.Evaluate(context.Background(), proposal, analysis, riskAssessment)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionRequireReview {
		t.Errorf("Decision = %s, want require_review (major version exemption)", result.Decision)
	}
}

func TestEvaluator_AgentRequiresHumanReview(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ActorRules.RequireHumanForAgentChanges = true
	evaluator := New(WithConfig(cfg))

	proposal := createTestProposal("patch", cgp.ActorKindAgent, cgp.TrustLevelTrusted)
	analysis := createTestAnalysis(0, 0, 0)
	riskAssessment := &risk.Assessment{
		Score:    0.1,
		Severity: cgp.SeverityLow,
	}

	result, err := evaluator.Evaluate(context.Background(), proposal, analysis, riskAssessment)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionRequireReview {
		t.Errorf("Decision = %s, want require_review (agent requires human)", result.Decision)
	}
}

func TestEvaluator_TrustedActorBypass(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ActorRules.TrustedActors = []string{"trusted-ci:release-bot"}
	cfg.ActorRules.RequireHumanForAgentChanges = true
	evaluator := New(WithConfig(cfg))

	proposal := &cgp.ChangeProposal{
		ID:        "prop-1",
		Timestamp: time.Now(),
		Actor: cgp.Actor{
			ID:         "trusted-ci:release-bot",
			Kind:       cgp.ActorKindCI,
			Name:       "Release Bot",
			TrustLevel: cgp.TrustLevelTrusted,
		},
		Intent: cgp.ProposalIntent{
			Summary:       "Routine release",
			SuggestedBump: cgp.BumpTypePatch,
			Confidence:    0.9,
		},
		Scope: cgp.ProposalScope{
			Repository: "org/repo",
			Branch:     "main",
		},
	}
	analysis := createTestAnalysis(0, 0, 0)
	riskAssessment := &risk.Assessment{
		Score:    0.1,
		Severity: cgp.SeverityLow,
	}

	result, err := evaluator.Evaluate(context.Background(), proposal, analysis, riskAssessment)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionAutoApproved {
		t.Errorf("Decision = %s, want auto_approved (trusted actor)", result.Decision)
	}
}

func TestEvaluator_Disabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	evaluator := New(WithConfig(cfg))

	proposal := createTestProposal("patch", cgp.ActorKindCI, cgp.TrustLevelTrusted)
	analysis := createTestAnalysis(0, 0, 0)
	riskAssessment := &risk.Assessment{
		Score:    0.1,
		Severity: cgp.SeverityLow,
	}

	result, err := evaluator.Evaluate(context.Background(), proposal, analysis, riskAssessment)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionRequireReview {
		t.Errorf("Decision = %s, want require_review (disabled)", result.Decision)
	}
}

func TestEvaluator_PolicyPriority(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Policies = []AutoApprovalPolicy{
		{
			ID:       "low-priority",
			Name:     "Low Priority",
			Enabled:  true,
			Priority: 10,
			Conditions: []PolicyCondition{
				{Field: "risk.score", Operator: "lte", Value: 0.5},
			},
		},
		{
			ID:       "high-priority",
			Name:     "High Priority",
			Enabled:  true,
			Priority: 100,
			Conditions: []PolicyCondition{
				{Field: "risk.score", Operator: "lte", Value: 0.3},
			},
		},
	}
	cfg.Exemptions.BreakingChanges = false
	cfg.Exemptions.SecurityChanges = false
	cfg.Exemptions.MajorVersions = false
	evaluator := New(WithConfig(cfg))

	proposal := createTestProposal("patch", cgp.ActorKindCI, cgp.TrustLevelTrusted)
	analysis := createTestAnalysis(0, 0, 0)
	riskAssessment := &risk.Assessment{
		Score:    0.2,
		Severity: cgp.SeverityLow,
	}

	result, err := evaluator.Evaluate(context.Background(), proposal, analysis, riskAssessment)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.MatchedPolicy == nil {
		t.Fatal("MatchedPolicy should not be nil")
	}
	if result.MatchedPolicy.ID != "high-priority" {
		t.Errorf("MatchedPolicy.ID = %s, want high-priority", result.MatchedPolicy.ID)
	}
}

func TestEvaluator_ApproverCountByRisk(t *testing.T) {
	evaluator := New()

	tests := []struct {
		riskScore         float64
		wantApprovers     int
		wantRequireReview bool
	}{
		{0.1, 0, false}, // Low risk, auto-approved
		{0.35, 1, true}, // Medium-low risk
		{0.55, 2, true}, // Medium risk
		{0.75, 3, true}, // High risk
	}

	for _, tt := range tests {
		proposal := createTestProposal("minor", cgp.ActorKindHuman, cgp.TrustLevelTrusted)
		analysis := createTestAnalysis(1, 0, 0)
		riskAssessment := &risk.Assessment{
			Score:    tt.riskScore,
			Severity: cgp.SeverityMedium,
		}

		result, err := evaluator.Evaluate(context.Background(), proposal, analysis, riskAssessment)
		if err != nil {
			t.Fatalf("Evaluate failed for risk %.2f: %v", tt.riskScore, err)
		}

		if tt.wantRequireReview {
			if result.Decision != DecisionRequireReview {
				t.Errorf("Risk %.2f: Decision = %s, want require_review", tt.riskScore, result.Decision)
			}
			if result.RequiredApprovers != tt.wantApprovers {
				t.Errorf("Risk %.2f: RequiredApprovers = %d, want %d",
					tt.riskScore, result.RequiredApprovers, tt.wantApprovers)
			}
		}
	}
}

func TestEvaluator_NilProposal(t *testing.T) {
	evaluator := New()

	riskAssessment := &risk.Assessment{
		Score:    0.1,
		Severity: cgp.SeverityLow,
	}

	result, err := evaluator.Evaluate(context.Background(), nil, nil, riskAssessment)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Should still work with nil proposal
	if result.RiskScore != 0.1 {
		t.Errorf("RiskScore = %v, want 0.1", result.RiskScore)
	}
}

func TestEvaluator_NilRiskAssessment(t *testing.T) {
	evaluator := New()

	proposal := createTestProposal("patch", cgp.ActorKindCI, cgp.TrustLevelTrusted)
	analysis := createTestAnalysis(0, 0, 0)

	result, err := evaluator.Evaluate(context.Background(), proposal, analysis, nil)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.RiskScore != 0 {
		t.Errorf("RiskScore = %v, want 0", result.RiskScore)
	}
}

func TestResult_ToGovernanceDecision(t *testing.T) {
	result := &Result{
		Decision:          DecisionAutoApproved,
		CGPDecision:       cgp.DecisionApproved,
		Approved:          true,
		RiskScore:         0.2,
		Rationale:         []string{"Low risk", "Policy matched"},
		RequiredApprovers: 0,
		MatchedPolicy: &AutoApprovalPolicy{
			ID:   "test-policy",
			Name: "Test Policy",
		},
	}

	decision := result.ToGovernanceDecision("proposal-123")

	if decision.ProposalID != "proposal-123" {
		t.Errorf("ProposalID = %s, want proposal-123", decision.ProposalID)
	}
	if decision.Decision != cgp.DecisionApproved {
		t.Errorf("Decision = %s, want approved", decision.Decision)
	}
	if decision.RiskScore != 0.2 {
		t.Errorf("RiskScore = %v, want 0.2", decision.RiskScore)
	}
	if len(decision.Rationale) == 0 {
		t.Error("Rationale should not be empty")
	}
}

func TestEvaluator_LargeChangeExemption(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Exemptions.LargeChanges = &LargeChangeConfig{
		MaxFiles: 10,
		MaxLines: 100,
	}
	evaluator := New(WithConfig(cfg))

	proposal := createTestProposal("patch", cgp.ActorKindCI, cgp.TrustLevelTrusted)
	analysis := &cgp.ChangeAnalysis{
		Features: 0,
		Breaking: 0,
		Security: 0,
		BlastRadius: &cgp.BlastRadius{
			FilesChanged: 50, // Exceeds limit
			LinesChanged: 500,
		},
	}
	riskAssessment := &risk.Assessment{
		Score:    0.1,
		Severity: cgp.SeverityLow,
	}

	result, err := evaluator.Evaluate(context.Background(), proposal, analysis, riskAssessment)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionRequireReview {
		t.Errorf("Decision = %s, want require_review (large change)", result.Decision)
	}
	if len(result.ExemptionHits) == 0 {
		t.Error("ExemptionHits should contain large change info")
	}
}

func TestStrictConfig(t *testing.T) {
	cfg := StrictConfig()

	if cfg.Thresholds.AutoApproveMax >= 0.3 {
		t.Error("StrictConfig should have lower AutoApproveMax")
	}
	if !cfg.Exemptions.NewDependencies {
		t.Error("StrictConfig should block new dependencies")
	}
	if cfg.ActorRules.CIAutoApprove {
		t.Error("StrictConfig should not allow CI auto-approve")
	}
}

func TestPermissiveConfig(t *testing.T) {
	cfg := PermissiveConfig()

	if cfg.Thresholds.AutoApproveMax <= 0.3 {
		t.Error("PermissiveConfig should have higher AutoApproveMax")
	}
	if cfg.Exemptions.BreakingChanges {
		t.Error("PermissiveConfig should allow breaking changes")
	}
	if !cfg.ActorRules.AgentAutoApprove {
		t.Error("PermissiveConfig should allow agent auto-approve")
	}
}

func TestConditionOperators(t *testing.T) {
	tests := []struct {
		name     string
		cond     PolicyCondition
		ctx      map[string]any
		expected bool
	}{
		{
			name:     "eq string",
			cond:     PolicyCondition{Field: "actor.kind", Operator: "eq", Value: "ci"},
			ctx:      map[string]any{"actor": map[string]any{"kind": "ci"}},
			expected: true,
		},
		{
			name:     "ne string",
			cond:     PolicyCondition{Field: "actor.kind", Operator: "ne", Value: "agent"},
			ctx:      map[string]any{"actor": map[string]any{"kind": "ci"}},
			expected: true,
		},
		{
			name:     "lte numeric",
			cond:     PolicyCondition{Field: "risk.score", Operator: "lte", Value: 0.5},
			ctx:      map[string]any{"risk": map[string]any{"score": 0.3}},
			expected: true,
		},
		{
			name:     "gt numeric",
			cond:     PolicyCondition{Field: "change.features", Operator: "gt", Value: 0},
			ctx:      map[string]any{"change": map[string]any{"features": 5}},
			expected: true,
		},
		{
			name:     "in array",
			cond:     PolicyCondition{Field: "intent.suggestedBump", Operator: "in", Value: []string{"patch", "minor"}},
			ctx:      map[string]any{"intent": map[string]any{"suggestedBump": "patch"}},
			expected: true,
		},
		{
			name:     "not_in array",
			cond:     PolicyCondition{Field: "intent.suggestedBump", Operator: "not_in", Value: []string{"major"}},
			ctx:      map[string]any{"intent": map[string]any{"suggestedBump": "patch"}},
			expected: true,
		},
		{
			name:     "contains string",
			cond:     PolicyCondition{Field: "scope.branch", Operator: "contains", Value: "release"},
			ctx:      map[string]any{"scope": map[string]any{"branch": "release/v1.0"}},
			expected: true,
		},
		{
			name:     "matches regex",
			cond:     PolicyCondition{Field: "scope.branch", Operator: "matches", Value: "^main$"},
			ctx:      map[string]any{"scope": map[string]any{"branch": "main"}},
			expected: true,
		},
	}

	evaluator := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, _ := evaluator.evaluateCondition(tt.cond, tt.ctx)
			if matched != tt.expected {
				t.Errorf("evaluateCondition() = %v, want %v", matched, tt.expected)
			}
		})
	}
}

// Helper functions

func createTestProposal(bumpType string, actorKind cgp.ActorKind, trustLevel cgp.TrustLevel) *cgp.ChangeProposal {
	return &cgp.ChangeProposal{
		ID:        "prop-test",
		Timestamp: time.Now(),
		Actor: cgp.Actor{
			ID:         "test-actor",
			Kind:       actorKind,
			Name:       "Test Actor",
			TrustLevel: trustLevel,
		},
		Intent: cgp.ProposalIntent{
			Summary:       "Test change",
			SuggestedBump: cgp.BumpType(bumpType),
			Confidence:    0.9,
		},
		Scope: cgp.ProposalScope{
			Repository: "org/repo",
			Branch:     "main",
		},
	}
}

func createTestAnalysis(features, breaking, security int) *cgp.ChangeAnalysis {
	return &cgp.ChangeAnalysis{
		Features: features,
		Breaking: breaking,
		Security: security,
	}
}

func TestWithLogger(t *testing.T) {
	logger := slog.Default()
	evaluator := New(WithLogger(logger))

	if evaluator.logger != logger {
		t.Error("WithLogger should set the logger")
	}
}

func TestCheckTimeConstraints(t *testing.T) {
	evaluator := New()

	// Test business hours only
	tc := &TimeConstraints{
		BusinessHoursOnly: true,
	}
	// This test depends on current time, so we just verify it runs without panic
	blocked, _ := evaluator.checkTimeConstraints(tc)
	_ = blocked // Result depends on current time

	// Test blocked dates
	today := time.Now().Format("2006-01-02")
	tc = &TimeConstraints{
		BlockedDates: []string{today},
	}
	blocked, reason := evaluator.checkTimeConstraints(tc)
	if !blocked {
		t.Error("should be blocked on today's date")
	}
	if reason == "" {
		t.Error("should have a reason")
	}

	// Test allowed days (block all days except non-existent day 10)
	tc = &TimeConstraints{
		AllowedDays: []int{10}, // Day 10 doesn't exist (weekdays are 0-6)
	}
	blocked, _ = evaluator.checkTimeConstraints(tc)
	if !blocked {
		t.Error("should be blocked when today is not in allowed days")
	}

	// Test allowed days - today should pass
	tc = &TimeConstraints{
		AllowedDays: []int{int(time.Now().Weekday())},
	}
	blocked, _ = evaluator.checkTimeConstraints(tc)
	if blocked {
		t.Error("should not be blocked when today is in allowed days")
	}

	// Test freeze windows
	tc = &TimeConstraints{
		FreezeWindows: []FreezeWindow{
			{
				Name:   "Current freeze",
				Start:  time.Now().Add(-time.Hour),
				End:    time.Now().Add(time.Hour),
				Reason: "Testing",
			},
		},
	}
	blocked, reason = evaluator.checkTimeConstraints(tc)
	if !blocked {
		t.Error("should be blocked during freeze window")
	}
	if !strings.Contains(reason, "Current freeze") {
		t.Errorf("reason should mention freeze window name, got: %s", reason)
	}

	// Test no constraints trigger
	tc = &TimeConstraints{}
	blocked, reason = evaluator.checkTimeConstraints(tc)
	if blocked {
		t.Errorf("empty constraints should not block, got reason: %s", reason)
	}
}

func TestCheckActorConstraints(t *testing.T) {
	evaluator := New()

	// Test blocked actor
	ac := &ActorConstraints{
		BlockedActorIDs: []string{"blocked-user"},
	}
	actor := cgp.Actor{ID: "blocked-user", Kind: cgp.ActorKindHuman}
	blocked, reason := evaluator.checkActorConstraints(ac, actor)
	if !blocked {
		t.Error("blocked actor should be blocked")
	}
	if !strings.Contains(reason, "blocked-user") {
		t.Errorf("reason should mention actor ID, got: %s", reason)
	}

	// Test allowed actors list
	ac = &ActorConstraints{
		AllowedActorIDs: []string{"allowed-user"},
	}
	actor = cgp.Actor{ID: "not-allowed-user", Kind: cgp.ActorKindHuman}
	blocked, _ = evaluator.checkActorConstraints(ac, actor)
	if !blocked {
		t.Error("actor not in allowed list should be blocked")
	}

	// Test allowed actor in list
	actor = cgp.Actor{ID: "allowed-user", Kind: cgp.ActorKindHuman}
	blocked, _ = evaluator.checkActorConstraints(ac, actor)
	if blocked {
		t.Error("actor in allowed list should not be blocked")
	}

	// Test allowed actor kinds
	ac = &ActorConstraints{
		AllowedActorKinds: []cgp.ActorKind{cgp.ActorKindCI},
	}
	actor = cgp.Actor{ID: "user", Kind: cgp.ActorKindHuman}
	blocked, _ = evaluator.checkActorConstraints(ac, actor)
	if !blocked {
		t.Error("actor kind not in allowed list should be blocked")
	}

	// Test actor kind in allowed list
	actor = cgp.Actor{ID: "ci-bot", Kind: cgp.ActorKindCI}
	blocked, _ = evaluator.checkActorConstraints(ac, actor)
	if blocked {
		t.Error("actor kind in allowed list should not be blocked")
	}

	// Test minimum trust level
	ac = &ActorConstraints{
		MinTrustLevel: cgp.TrustLevelTrusted,
	}
	actor = cgp.Actor{ID: "user", Kind: cgp.ActorKindHuman, TrustLevel: cgp.TrustLevelUntrusted}
	blocked, reason = evaluator.checkActorConstraints(ac, actor)
	if !blocked {
		t.Error("actor with low trust level should be blocked")
	}
	if !strings.Contains(reason, "trust level") {
		t.Errorf("reason should mention trust level, got: %s", reason)
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		value   string
		pattern string
		want    bool
	}{
		{"main", "main", true},
		{"main", "release/*", false},
		{"release/v1.0", "release/*", true},
		{"release/v2.0", "release/*", true},
		{"develop", "release/*", false},
		{"feature/test", "feature/*", true},
		{"main", "*", true},
	}

	for _, tt := range tests {
		t.Run(tt.value+"/"+tt.pattern, func(t *testing.T) {
			got := matchPattern(tt.value, tt.pattern)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.value, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestCompareFloatEqual(t *testing.T) {
	tests := []struct {
		name string
		a    float64
		b    any
		want bool
	}{
		{"float64 equal", 0.5, 0.5, true},
		{"float64 not equal", 0.5, 0.6, false},
		{"int equal", 1.0, 1, true},
		{"int64 equal", 1.0, int64(1), true},
		{"string", 1.0, "1.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareFloatEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareFloatEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		want   float64
		wantOk bool
	}{
		{"float64", 1.5, 1.5, true},
		{"float32", float32(1.5), 1.5, true},
		{"int", 10, 10.0, true},
		{"int64", int64(20), 20.0, true},
		{"int32", int32(30), 30.0, true},
		{"string", "1.5", 0, false},
		{"bool", true, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat64(tt.input)
			if ok != tt.wantOk {
				t.Errorf("toFloat64(%v) ok = %v, want %v", tt.input, ok, tt.wantOk)
			}
			if ok && got != tt.want {
				t.Errorf("toFloat64(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValuesEqual_BoolAndOther(t *testing.T) {
	// Test bool equality
	if !valuesEqual(true, true) {
		t.Error("true == true should be true")
	}
	if valuesEqual(true, false) {
		t.Error("true == false should be false")
	}
	// Note: valuesEqual uses fmt.Sprintf fallback, so true == "true" is actually true
	if !valuesEqual(true, "true") {
		t.Error("true == \"true\" should be true due to sprintf fallback")
	}

	// Test int64 path
	if !valuesEqual(int64(5), 5) {
		t.Error("int64(5) == 5 should be true")
	}

	// Test fallback to fmt.Sprintf comparison
	if !valuesEqual("5", "5") {
		t.Error("\"5\" == \"5\" should be true")
	}
}

func TestProtectedBranchesExemption(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Exemptions.ProtectedBranches = []string{"main", "release/*"}
	cfg.Exemptions.BreakingChanges = false
	cfg.Exemptions.SecurityChanges = false
	cfg.Exemptions.MajorVersions = false
	evaluator := New(WithConfig(cfg))

	// Test protected branch
	proposal := createTestProposal("patch", cgp.ActorKindCI, cgp.TrustLevelTrusted)
	proposal.Scope.Branch = "main"
	analysis := createTestAnalysis(0, 0, 0)
	riskAssessment := &risk.Assessment{Score: 0.1, Severity: cgp.SeverityLow}

	result, err := evaluator.Evaluate(context.Background(), proposal, analysis, riskAssessment)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionRequireReview {
		t.Errorf("Decision = %s, want require_review (protected branch)", result.Decision)
	}
	found := false
	for _, exemption := range result.ExemptionHits {
		if strings.Contains(exemption, "protected") {
			found = true
			break
		}
	}
	if !found {
		t.Error("ExemptionHits should mention protected branch")
	}
}

func TestNewDependenciesExemption(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Exemptions.NewDependencies = true
	cfg.Exemptions.BreakingChanges = false
	cfg.Exemptions.SecurityChanges = false
	cfg.Exemptions.MajorVersions = false
	evaluator := New(WithConfig(cfg))

	proposal := createTestProposal("patch", cgp.ActorKindCI, cgp.TrustLevelTrusted)
	analysis := &cgp.ChangeAnalysis{
		Features:     0,
		Breaking:     0,
		Security:     0,
		Dependencies: 3, // New dependencies
	}
	riskAssessment := &risk.Assessment{Score: 0.1, Severity: cgp.SeverityLow}

	result, err := evaluator.Evaluate(context.Background(), proposal, analysis, riskAssessment)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionRequireReview {
		t.Errorf("Decision = %s, want require_review (new dependencies)", result.Decision)
	}
}

func TestPolicyWithTimeConstraints(t *testing.T) {
	cfg := DefaultConfig()
	// Create a policy that would match but has time constraints blocking it
	cfg.Policies = []AutoApprovalPolicy{
		{
			ID:       "test-policy",
			Name:     "Test Policy",
			Enabled:  true,
			Priority: 100,
			Conditions: []PolicyCondition{
				{Field: "risk.score", Operator: "lte", Value: 0.5},
			},
			TimeConstraints: &TimeConstraints{
				BlockedDates: []string{time.Now().Format("2006-01-02")},
			},
		},
	}
	cfg.Exemptions.BreakingChanges = false
	cfg.Exemptions.SecurityChanges = false
	cfg.Exemptions.MajorVersions = false
	evaluator := New(WithConfig(cfg))

	proposal := createTestProposal("patch", cgp.ActorKindCI, cgp.TrustLevelTrusted)
	analysis := createTestAnalysis(0, 0, 0)
	riskAssessment := &risk.Assessment{Score: 0.1, Severity: cgp.SeverityLow}

	result, err := evaluator.Evaluate(context.Background(), proposal, analysis, riskAssessment)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionRequireReview {
		t.Errorf("Decision = %s, want require_review (time constraints)", result.Decision)
	}
}

func TestPolicyWithActorConstraints(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Policies = []AutoApprovalPolicy{
		{
			ID:       "test-policy",
			Name:     "Test Policy",
			Enabled:  true,
			Priority: 100,
			Conditions: []PolicyCondition{
				{Field: "risk.score", Operator: "lte", Value: 0.5},
			},
			ActorConstraints: &ActorConstraints{
				BlockedActorIDs: []string{"test-actor"},
			},
		},
	}
	cfg.Exemptions.BreakingChanges = false
	cfg.Exemptions.SecurityChanges = false
	cfg.Exemptions.MajorVersions = false
	evaluator := New(WithConfig(cfg))

	proposal := createTestProposal("patch", cgp.ActorKindCI, cgp.TrustLevelTrusted)
	analysis := createTestAnalysis(0, 0, 0)
	riskAssessment := &risk.Assessment{Score: 0.1, Severity: cgp.SeverityLow}

	result, err := evaluator.Evaluate(context.Background(), proposal, analysis, riskAssessment)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionRequireReview {
		t.Errorf("Decision = %s, want require_review (actor constraints)", result.Decision)
	}
}

func TestValueInWithAnySlice(t *testing.T) {
	// Test with []any slice
	list := []any{"patch", "minor"}
	if !valueIn("patch", list) {
		t.Error("\"patch\" should be in list")
	}
	if valueIn("major", list) {
		t.Error("\"major\" should not be in list")
	}
}

func TestValueContainsEdgeCases(t *testing.T) {
	// Test with non-string field
	if valueContains(123, "1") {
		t.Error("non-string field should return false")
	}
	// Test with non-string search
	if valueContains("hello", 123) {
		t.Error("non-string search should return false")
	}
}

func TestValueMatchesEdgeCases(t *testing.T) {
	// Test with non-string field
	if valueMatches(123, ".*") {
		t.Error("non-string field should return false")
	}
	// Test with non-string pattern
	if valueMatches("hello", 123) {
		t.Error("non-string pattern should return false")
	}
	// Test with invalid regex
	if valueMatches("hello", "[invalid") {
		t.Error("invalid regex should return false")
	}
}

func TestGetNestedValueEdgeCases(t *testing.T) {
	data := map[string]any{
		"level1": map[string]any{
			"level2": "value",
		},
		"string": "not a map",
	}

	// Test valid path
	if getNestedValue(data, "level1.level2") != "value" {
		t.Error("should get nested value")
	}

	// Test non-existent key
	if getNestedValue(data, "level1.notexist") != nil {
		t.Error("non-existent key should return nil")
	}

	// Test path through non-map
	if getNestedValue(data, "string.child") != nil {
		t.Error("path through non-map should return nil")
	}
}

func TestToGovernanceDecisionWithRequiredApprovers(t *testing.T) {
	result := &Result{
		Decision:          DecisionRequireReview,
		CGPDecision:       cgp.DecisionApprovalRequired,
		Approved:          false,
		RiskScore:         0.6,
		Rationale:         []string{"High risk"},
		RequiredApprovers: 2,
		MatchedPolicy:     nil,
	}

	decision := result.ToGovernanceDecision("proposal-123")

	if decision.ProposalID != "proposal-123" {
		t.Errorf("ProposalID = %s, want proposal-123", decision.ProposalID)
	}
	if decision.Decision != cgp.DecisionApprovalRequired {
		t.Errorf("Decision = %s, want approval_required", decision.Decision)
	}
	// Check that required action was added
	found := false
	for _, action := range decision.RequiredActions {
		if action.Type == "human_approval" {
			found = true
			break
		}
	}
	if !found {
		t.Error("should have human_approval required action")
	}
}
