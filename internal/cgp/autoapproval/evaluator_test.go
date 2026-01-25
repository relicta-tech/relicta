package autoapproval

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/risk"
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
