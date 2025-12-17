package evaluator

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/policy"
)

func TestNew(t *testing.T) {
	e := New()
	if e == nil {
		t.Fatal("New() should not return nil")
	}
	if e.riskCalculator == nil {
		t.Error("riskCalculator should be initialized")
	}
	if e.policyEngine == nil {
		t.Error("policyEngine should be initialized")
	}
	if e.logger == nil {
		t.Error("logger should be initialized")
	}
}

func TestNewWithPolicies(t *testing.T) {
	p := policy.NewPolicy("test-policy")
	e := NewWithPolicies([]policy.Policy{*p})

	if e == nil {
		t.Fatal("NewWithPolicies() should not return nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DefaultDecision != cgp.DecisionApprovalRequired {
		t.Errorf("DefaultDecision = %v, want %v", cfg.DefaultDecision, cgp.DecisionApprovalRequired)
	}
	if cfg.AutoApproveThreshold <= 0 || cfg.AutoApproveThreshold >= 1 {
		t.Errorf("AutoApproveThreshold = %v, should be between 0 and 1", cfg.AutoApproveThreshold)
	}
	if !cfg.RequireHumanForBreaking {
		t.Error("RequireHumanForBreaking should be true by default")
	}
	if !cfg.RequireHumanForSecurity {
		t.Error("RequireHumanForSecurity should be true by default")
	}
}

func TestEvaluator_Evaluate_NilProposal(t *testing.T) {
	e := New()
	_, err := e.Evaluate(context.Background(), nil, nil)
	if err == nil {
		t.Error("Evaluate() should return error for nil proposal")
	}
}

func TestEvaluator_Evaluate_InvalidProposal(t *testing.T) {
	e := New()
	proposal := &cgp.ChangeProposal{} // Invalid - missing required fields
	_, err := e.Evaluate(context.Background(), proposal, nil)
	if err == nil {
		t.Error("Evaluate() should return error for invalid proposal")
	}
}

func TestEvaluator_Evaluate_BasicProposal(t *testing.T) {
	e := New()
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test change", Confidence: 0.9},
	)

	result, err := e.Evaluate(context.Background(), proposal, nil)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if result == nil {
		t.Fatal("Evaluate() result should not be nil")
	}
	if result.Decision == nil {
		t.Fatal("Evaluate() Decision should not be nil")
	}
	if result.RiskAssessment == nil {
		t.Fatal("Evaluate() RiskAssessment should not be nil")
	}
	if result.PolicyResult == nil {
		t.Fatal("Evaluate() PolicyResult should not be nil")
	}
	if result.Duration <= 0 {
		t.Error("Evaluate() Duration should be positive")
	}
}

func TestEvaluator_Evaluate_WithAnalysis(t *testing.T) {
	e := New()
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Add new feature", Confidence: 0.85},
	)
	analysis := &cgp.ChangeAnalysis{
		Features: 2,
		Fixes:    1,
		Breaking: 0,
		Security: 0,
	}

	result, err := e.Evaluate(context.Background(), proposal, analysis)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if result.Decision.Analysis == nil {
		t.Error("Decision should include analysis")
	}
}

func TestEvaluator_Evaluate_AgentActorHighRisk(t *testing.T) {
	e := New(WithConfig(Config{
		DefaultDecision:         cgp.DecisionApproved,
		MaxAutoApproveRisk:      0.3,
		RequireHumanForBreaking: true,
		RequireHumanForSecurity: true,
		AutoApproveThreshold:    0.2,
	}))

	proposal := cgp.NewProposal(
		cgp.NewAgentActor("cursor", "Cursor", "gpt-4"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Major refactoring", Confidence: 0.6},
	)
	// High-risk analysis
	analysis := &cgp.ChangeAnalysis{
		Features: 0,
		Fixes:    0,
		Breaking: 5,
		Security: 2,
		BlastRadius: &cgp.BlastRadius{
			FilesChanged: 100,
			LinesChanged: 5000,
		},
	}

	result, err := e.Evaluate(context.Background(), proposal, analysis)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	// Agent with high-risk changes should require approval
	if result.Decision.Decision != cgp.DecisionApprovalRequired {
		t.Errorf("High-risk agent change should require approval, got %v", result.Decision.Decision)
	}
}

func TestEvaluator_Evaluate_BreakingChangesRequireReview(t *testing.T) {
	e := New(WithConfig(Config{
		DefaultDecision:         cgp.DecisionApproved,
		RequireHumanForBreaking: true,
		RequireHumanForSecurity: false,
		MaxAutoApproveRisk:      1.0, // Allow high risk for this test
		AutoApproveThreshold:    0.5,
	}))

	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Breaking API change", Confidence: 0.9},
	)
	analysis := &cgp.ChangeAnalysis{
		Breaking: 3,
		APIChanges: []cgp.APIChange{
			{Type: "removed", Symbol: "OldFunc", Breaking: true},
		},
	}

	result, err := e.Evaluate(context.Background(), proposal, analysis)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	// Breaking changes should require approval
	if result.Decision.Decision != cgp.DecisionApprovalRequired {
		t.Errorf("Breaking changes should require approval, got %v", result.Decision.Decision)
	}

	// Should have rationale about breaking changes
	hasBreakingRationale := false
	for _, r := range result.Decision.Rationale {
		if containsSubstring(r, "breaking") {
			hasBreakingRationale = true
			break
		}
	}
	if !hasBreakingRationale {
		t.Error("Decision should have rationale about breaking changes")
	}
}

func TestEvaluator_Evaluate_SecurityChangesRequireReview(t *testing.T) {
	e := New(WithConfig(Config{
		DefaultDecision:         cgp.DecisionApproved,
		RequireHumanForBreaking: false,
		RequireHumanForSecurity: true,
		MaxAutoApproveRisk:      1.0,
		AutoApproveThreshold:    0.5,
	}))

	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Security update", Confidence: 0.9},
	)
	analysis := &cgp.ChangeAnalysis{
		Security: 2,
	}

	result, err := e.Evaluate(context.Background(), proposal, analysis)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	// Security changes should require approval
	if result.Decision.Decision != cgp.DecisionApprovalRequired {
		t.Errorf("Security changes should require approval, got %v", result.Decision.Decision)
	}
}

func TestEvaluator_Evaluate_WithPolicy(t *testing.T) {
	// Create a policy that blocks agent changes
	p := policy.NewPolicy("agent-restriction")
	p.AddRule(*policy.NewRule("block-agents", "Block all agent changes").
		WithPriority(100).
		AddCondition("actor.kind", policy.OperatorEqual, "agent").
		AddAction(policy.ActionBlock, map[string]any{"reason": "Agents not allowed"}))

	e := NewWithPolicies([]policy.Policy{*p})

	proposal := cgp.NewProposal(
		cgp.NewAgentActor("cursor", "Cursor", "gpt-4"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Agent change", Confidence: 0.9},
	)

	result, err := e.Evaluate(context.Background(), proposal, nil)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	// Policy should block agent
	if result.Decision.Decision != cgp.DecisionRejected {
		t.Errorf("Agent should be rejected by policy, got %v", result.Decision.Decision)
	}
}

func TestEvaluator_Evaluate_LowRiskTrustedActor(t *testing.T) {
	e := New(WithConfig(Config{
		DefaultDecision:         cgp.DecisionApprovalRequired,
		RequireHumanForBreaking: true,
		RequireHumanForSecurity: true,
		MaxAutoApproveRisk:      0.5,
		AutoApproveThreshold:    0.3,
	}))

	// Trusted human actor with full trust
	actor := cgp.NewHumanActor("john@example.com", "John")
	actor.TrustLevel = cgp.TrustLevelFull

	proposal := cgp.NewProposal(
		actor,
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Minor fix", Confidence: 0.95},
	)
	// Low-risk analysis
	analysis := &cgp.ChangeAnalysis{
		Fixes: 1,
		BlastRadius: &cgp.BlastRadius{
			FilesChanged: 1,
			LinesChanged: 5,
		},
	}

	result, err := e.Evaluate(context.Background(), proposal, analysis)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	// Low risk + trusted actor should be approved
	if result.Decision.Decision != cgp.DecisionApproved {
		t.Errorf("Low-risk trusted change should be approved, got %v", result.Decision.Decision)
	}
}

func TestEvaluator_EvaluateQuick(t *testing.T) {
	e := New()
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
	)

	assessment, err := e.EvaluateQuick(context.Background(), proposal, nil)
	if err != nil {
		t.Fatalf("EvaluateQuick() error = %v", err)
	}

	if assessment == nil {
		t.Fatal("EvaluateQuick() should return assessment")
	}
	if assessment.Score < 0 || assessment.Score > 1 {
		t.Errorf("Score should be between 0 and 1, got %v", assessment.Score)
	}
}

func TestEvaluator_EvaluateQuick_NilProposal(t *testing.T) {
	e := New()
	_, err := e.EvaluateQuick(context.Background(), nil, nil)
	if err == nil {
		t.Error("EvaluateQuick() should return error for nil proposal")
	}
}

func TestEvaluator_ValidateProposal(t *testing.T) {
	e := New()

	tests := []struct {
		name      string
		proposal  *cgp.ChangeProposal
		expectErr bool
	}{
		{
			name:      "nil proposal",
			proposal:  nil,
			expectErr: true,
		},
		{
			name:      "empty proposal",
			proposal:  &cgp.ChangeProposal{},
			expectErr: true,
		},
		{
			name: "valid proposal",
			proposal: cgp.NewProposal(
				cgp.NewHumanActor("john@example.com", "John"),
				cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
				cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
			),
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := e.ValidateProposal(tt.proposal)
			if (err != nil) != tt.expectErr {
				t.Errorf("ValidateProposal() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestEvaluator_AddPolicy(t *testing.T) {
	e := New()

	// Initially should approve (no policies)
	proposal := cgp.NewProposal(
		cgp.NewAgentActor("cursor", "Cursor", "gpt-4"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
	)

	result1, _ := e.Evaluate(context.Background(), proposal, nil)

	// Add blocking policy
	p := policy.NewPolicy("block-agents")
	p.AddRule(*policy.NewRule("block", "Block agents").
		WithPriority(100).
		AddCondition("actor.kind", policy.OperatorEqual, "agent").
		AddAction(policy.ActionBlock, map[string]any{"reason": "No agents"}))
	e.AddPolicy(*p)

	result2, _ := e.Evaluate(context.Background(), proposal, nil)

	// Should now be rejected
	if result2.Decision.Decision != cgp.DecisionRejected {
		t.Errorf("After adding policy, agent should be rejected, got %v", result2.Decision.Decision)
	}

	// First result should have been different (approved)
	if result1.Decision.Decision == cgp.DecisionRejected {
		t.Error("Before policy, agent should not have been rejected")
	}
}

func TestEvaluator_RiskFactorsInDecision(t *testing.T) {
	e := New()
	proposal := cgp.NewProposal(
		cgp.NewAgentActor("cursor", "Cursor", "gpt-4"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
	)
	analysis := &cgp.ChangeAnalysis{
		Breaking: 2,
		Security: 1,
		BlastRadius: &cgp.BlastRadius{
			FilesChanged: 50,
			LinesChanged: 1000,
		},
	}

	result, err := e.Evaluate(context.Background(), proposal, analysis)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	// Decision should have risk factors
	if len(result.Decision.RiskFactors) == 0 {
		t.Error("Decision should have risk factors")
	}

	// Risk score should be populated
	if result.Decision.RiskScore == 0 {
		t.Error("Decision should have non-zero risk score for this analysis")
	}
}

func TestEvaluator_SuggestedBumpInDecision(t *testing.T) {
	e := New()
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{
			Summary:       "Add new feature",
			Confidence:    0.9,
			SuggestedBump: cgp.BumpTypeMinor,
		},
	)

	result, err := e.Evaluate(context.Background(), proposal, nil)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if result.Decision.RecommendedVersion != string(cgp.BumpTypeMinor) {
		t.Errorf("RecommendedVersion = %v, want %v", result.Decision.RecommendedVersion, cgp.BumpTypeMinor)
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
