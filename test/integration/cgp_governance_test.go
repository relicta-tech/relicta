// Package integration provides integration tests for Relicta.
package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/internal/cgp/memory"
	"github.com/relicta-tech/relicta/internal/cgp/policy"
	"github.com/relicta-tech/relicta/internal/cgp/policy/dsl"
	"github.com/relicta-tech/relicta/internal/cgp/risk"
)

func TestCGP_EndToEnd_LowRiskPatch(t *testing.T) {
	RequireGitVersion(t, "2.0.0")

	repo := NewTestRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")
	repo.Tag("v1.0.0")

	// Small patch fix
	repo.WriteFile("main.go", "package main\n\nfunc main() { println(\"hello\") }")
	repo.Commit("fix: typo in output")

	ctx := context.Background()

	// Create actor with trust level
	actor := cgp.NewHumanActor("developer@example.com", "developer")
	actor.TrustLevel = cgp.TrustLevelFull

	// Create proposal with proper API
	proposal := cgp.NewProposal(
		actor,
		cgp.ProposalScope{
			Repository:  repo.Dir,
			CommitRange: "v1.0.0..HEAD",
		},
		cgp.ProposalIntent{
			Summary:       "Fix typo in output",
			SuggestedBump: "patch",
			Confidence:    0.9,
		},
	)

	// Create analysis with proper structure
	analysis := &cgp.ChangeAnalysis{
		Features: 0,
		Fixes:    1,
		Breaking: 0,
		Security: 0,
		BlastRadius: &cgp.BlastRadius{
			FilesChanged: 1,
			LinesChanged: 3,
		},
	}

	// Configure evaluator for auto-approval of low-risk patches
	cfg := evaluator.DefaultConfig()
	cfg.AutoApproveThreshold = 0.3

	eval := evaluator.New(
		evaluator.WithConfig(cfg),
		evaluator.WithRiskCalculator(risk.NewCalculatorWithDefaults()),
	)

	result, err := eval.Evaluate(ctx, proposal, analysis)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Low-risk patch from trusted human should be approved
	if result.Decision.Decision != cgp.DecisionApproved {
		t.Errorf("Expected DecisionApproved for low-risk patch, got %v", result.Decision.Decision)
	}

	if result.RiskAssessment.Score >= 0.3 {
		t.Errorf("Expected risk score < 0.3, got %v", result.RiskAssessment.Score)
	}
}

func TestCGP_EndToEnd_HighRiskMajorVersion(t *testing.T) {
	RequireGitVersion(t, "2.0.0")

	repo := NewTestRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")
	repo.Tag("v1.0.0")

	// Breaking change
	repo.WriteFile("api.go", "package main\n\nfunc NewAPI() {}")
	repo.Commit("feat!: replace old API with new implementation\n\nBREAKING CHANGE: Old API removed")

	ctx := context.Background()

	// Create actor
	actor := cgp.NewHumanActor("developer@example.com", "developer")

	// Create proposal
	proposal := cgp.NewProposal(
		actor,
		cgp.ProposalScope{
			Repository:  repo.Dir,
			CommitRange: "v1.0.0..HEAD",
		},
		cgp.ProposalIntent{
			Summary:       "Major API overhaul",
			SuggestedBump: "major",
			Confidence:    0.8,
		},
	)

	analysis := &cgp.ChangeAnalysis{
		Features: 2,
		Fixes:    0,
		Breaking: 1,
		Security: 0,
		BlastRadius: &cgp.BlastRadius{
			FilesChanged: 5,
			LinesChanged: 800,
		},
	}

	cfg := evaluator.DefaultConfig()
	cfg.RequireHumanForBreaking = true

	eval := evaluator.New(
		evaluator.WithConfig(cfg),
		evaluator.WithRiskCalculator(risk.NewCalculatorWithDefaults()),
	)

	result, err := eval.Evaluate(ctx, proposal, analysis)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Breaking changes should require approval
	if result.Decision.Decision != cgp.DecisionApprovalRequired {
		t.Errorf("Expected DecisionApprovalRequired for breaking change, got %v", result.Decision.Decision)
	}

	// Should have higher risk score
	if result.RiskAssessment.Score < 0.3 {
		t.Errorf("Expected risk score >= 0.3 for major change, got %v", result.RiskAssessment.Score)
	}

	// Check for breaking change rationale
	hasBreakingRationale := false
	for _, r := range result.Decision.Rationale {
		if strings.Contains(strings.ToLower(r), "breaking") {
			hasBreakingRationale = true
			break
		}
	}
	if !hasBreakingRationale {
		t.Error("Expected rationale mentioning breaking changes")
	}
}

func TestCGP_EndToEnd_AgentInitiatedChange(t *testing.T) {
	RequireGitVersion(t, "2.0.0")

	repo := NewTestRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")
	repo.Tag("v1.0.0")

	repo.WriteFile("code.go", "package main")
	repo.Commit("feat: add new feature by AI")

	ctx := context.Background()

	// Agent-initiated change
	actor := cgp.NewAgentActor("github-copilot", "GitHub Copilot", "gpt-4")

	proposal := cgp.NewProposal(
		actor,
		cgp.ProposalScope{
			Repository:  repo.Dir,
			CommitRange: "v1.0.0..HEAD",
		},
		cgp.ProposalIntent{
			Summary:       "AI-generated feature",
			SuggestedBump: "minor",
			Confidence:    0.7,
		},
	)

	analysis := &cgp.ChangeAnalysis{
		Features: 1,
		Fixes:    0,
		Breaking: 0,
		Security: 0,
		BlastRadius: &cgp.BlastRadius{
			FilesChanged: 3,
			LinesChanged: 110,
		},
	}

	cfg := evaluator.DefaultConfig()
	cfg.MaxAutoApproveRisk = 0.2 // Lower threshold for agents

	eval := evaluator.New(
		evaluator.WithConfig(cfg),
		evaluator.WithRiskCalculator(risk.NewCalculatorWithDefaults()),
	)

	result, err := eval.Evaluate(ctx, proposal, analysis)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Agent changes should generally require approval
	if result.Decision.Decision == cgp.DecisionApproved && result.RiskAssessment.Score > cfg.MaxAutoApproveRisk {
		t.Error("Agent change with elevated risk should require approval")
	}
}

func TestCGP_PolicyDSL_LoadAndEvaluate(t *testing.T) {
	// Create temp directory with policy files
	tmpDir := t.TempDir()

	// Write policy file - DSL uses AND/OR operators between conditions
	policyContent := `
rule "block-weekend-releases" {
    priority = 100
    description = "Block major releases on weekends"

    when {
        intent.suggestedBump == "major" AND time.day_of_week >= 5
    }

    then {
        block(reason: "Major releases not allowed on weekends")
    }
}

rule "require-security-review" {
    priority = 90
    description = "High-risk changes need security review"

    when {
        risk.score > 0.7
    }

    then {
        require_approval(role: "security-team")
        add_rationale(message: "High risk score detected")
    }
}

rule "auto-approve-patches" {
    priority = 50
    description = "Auto-approve low-risk patches"

    when {
        intent.suggestedBump == "patch" AND risk.score < 0.3
    }

    then {
        approve()
    }
}
`
	policyPath := filepath.Join(tmpDir, "governance.policy")
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("Failed to write policy file: %v", err)
	}

	// Load policies using the DSL loader
	policies := dsl.MustLoadDir(tmpDir)

	// One policy file = one Policy with 3 rules
	if len(policies) != 1 {
		t.Errorf("Expected 1 policy (one file), got %d", len(policies))
	}

	if len(policies[0].Rules) != 3 {
		t.Errorf("Expected 3 rules in policy, got %d", len(policies[0].Rules))
	}

	// Check rule IDs
	ruleIDs := make(map[string]bool)
	for _, r := range policies[0].Rules {
		ruleIDs[r.ID] = true
	}

	// Note: DSL compiler converts hyphens to underscores in rule IDs
	expectedRules := []string{"block_weekend_releases", "require_security_review", "auto_approve_patches"}
	for _, id := range expectedRules {
		if !ruleIDs[id] {
			t.Errorf("Missing expected rule: %s", id)
		}
	}
}

func TestCGP_PolicyEngine_Evaluation(t *testing.T) {
	ctx := context.Background()

	// Create policies with proper structure using builder pattern
	highRiskPolicy := policy.NewPolicy("high-risk-policy")
	highRiskPolicy.AddRule(policy.Rule{
		ID:          "high-risk-block",
		Name:        "Block High Risk",
		Priority:    100,
		Enabled:     true,
		Description: "Block releases with risk > 0.9",
		Conditions: []policy.Condition{
			{Field: "risk.score", Operator: "gt", Value: 0.9},
		},
		Actions: []policy.Action{
			{Type: "block", Params: map[string]any{"reason": "Risk score too high"}},
		},
	})
	highRiskPolicy.AddRule(policy.Rule{
		ID:          "require-approval-medium",
		Name:        "Require Approval Medium Risk",
		Priority:    50,
		Enabled:     true,
		Description: "Require approval for medium risk",
		Conditions: []policy.Condition{
			{Field: "risk.score", Operator: "gt", Value: 0.5},
		},
		Actions: []policy.Action{
			{Type: "require_approval", Params: map[string]any{"role": "tech-lead"}},
		},
	})

	engine := policy.NewEngine([]policy.Policy{*highRiskPolicy}, nil)

	// Create actor and proposal
	actor := cgp.NewHumanActor("developer@example.com", "developer")

	proposal := cgp.NewProposal(
		actor,
		cgp.ProposalScope{
			Repository:  "/test/repo",
			CommitRange: "abc..def",
		},
		cgp.ProposalIntent{
			Summary:    "Test change",
			Confidence: 0.9,
		},
	)

	analysis := &cgp.ChangeAnalysis{
		BlastRadius: &cgp.BlastRadius{
			FilesChanged: 10,
			LinesChanged: 250,
		},
	}

	tests := []struct {
		name       string
		riskScore  float64
		wantResult cgp.DecisionType
	}{
		{
			name:       "high risk rejected",
			riskScore:  0.95,
			wantResult: cgp.DecisionRejected, // block action maps to rejected
		},
		{
			name:       "medium risk requires approval",
			riskScore:  0.6,
			wantResult: cgp.DecisionApprovalRequired,
		},
		{
			name:       "low risk no match uses default",
			riskScore:  0.2,
			wantResult: cgp.DecisionApprovalRequired, // Engine defaults to require_review when no rules match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Evaluate(ctx, proposal, analysis, tt.riskScore)
			if err != nil {
				t.Fatalf("Evaluate failed: %v", err)
			}

			if result.Decision != tt.wantResult {
				t.Errorf("Decision = %v, want %v", result.Decision, tt.wantResult)
			}
		})
	}
}

func TestCGP_RiskCalculator_Integration(t *testing.T) {
	ctx := context.Background()

	calc := risk.NewCalculatorWithDefaults()

	tests := []struct {
		name          string
		analysis      *cgp.ChangeAnalysis
		actorKind     cgp.ActorKind
		wantMinScore  float64
		wantMaxScore  float64
		wantMinFactor string
	}{
		{
			name: "small patch fix",
			analysis: &cgp.ChangeAnalysis{
				Fixes: 1,
				BlastRadius: &cgp.BlastRadius{
					FilesChanged: 1,
					LinesChanged: 7,
				},
			},
			actorKind:    cgp.ActorKindHuman,
			wantMinScore: 0.0,
			wantMaxScore: 0.3,
		},
		{
			name: "large feature",
			analysis: &cgp.ChangeAnalysis{
				Features: 3,
				BlastRadius: &cgp.BlastRadius{
					FilesChanged: 20,
					LinesChanged: 600,
				},
			},
			actorKind:    cgp.ActorKindHuman,
			wantMinScore: 0.2,
			wantMaxScore: 0.7,
		},
		{
			name: "breaking change",
			analysis: &cgp.ChangeAnalysis{
				Breaking: 2,
				APIChanges: []cgp.APIChange{
					{Type: "removed", Symbol: "OldFunction", Breaking: true},
					{Type: "modified", Symbol: "UpdatedAPI", Breaking: true},
				},
				BlastRadius: &cgp.BlastRadius{
					FilesChanged: 5,
					LinesChanged: 150,
				},
			},
			actorKind:     cgp.ActorKindHuman,
			wantMinScore:  0.15, // With APIChanges weight of 0.25 and BlastRadius of 0.15
			wantMaxScore:  1.0,
			wantMinFactor: "api_change", // Correct category name (no trailing 's')
		},
		{
			name: "security change",
			analysis: &cgp.ChangeAnalysis{
				Security: 1,
				BlastRadius: &cgp.BlastRadius{
					FilesChanged: 3,
					LinesChanged: 70,
				},
			},
			actorKind:     cgp.ActorKindHuman,
			wantMinScore:  0.1, // SecurityImpact weight is only 0.05, realistic expectation
			wantMaxScore:  0.8,
			wantMinFactor: "security_impact",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actor := cgp.NewActor(tt.actorKind, "test")

			proposal := cgp.NewProposal(
				actor,
				cgp.ProposalScope{
					Repository:  "/test/repo",
					CommitRange: "abc..def",
				},
				cgp.ProposalIntent{
					Summary:    "Test change",
					Confidence: 0.9,
				},
			)

			assessment, err := calc.Calculate(ctx, proposal, tt.analysis)
			if err != nil {
				t.Fatalf("Calculate failed: %v", err)
			}

			if assessment.Score < tt.wantMinScore {
				t.Errorf("Score = %v, want >= %v", assessment.Score, tt.wantMinScore)
			}
			if assessment.Score > tt.wantMaxScore {
				t.Errorf("Score = %v, want <= %v", assessment.Score, tt.wantMaxScore)
			}

			if tt.wantMinFactor != "" {
				hasExpectedFactor := false
				for _, factor := range assessment.Factors {
					if factor.Category == tt.wantMinFactor {
						hasExpectedFactor = true
						break
					}
				}
				if !hasExpectedFactor {
					t.Errorf("Expected risk factor %q not found", tt.wantMinFactor)
				}
			}
		})
	}
}

func TestCGP_MemoryStore_ReleaseHistory(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	store, err := memory.NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Record some releases
	developerActor := cgp.NewHumanActor("developer@example.com", "developer")
	ciActor := cgp.NewCIActor("github-actions", "release", "123")

	releases := []*memory.ReleaseRecord{
		{
			ID:           "rel-001",
			Repository:   "owner/repo",
			Version:      "1.0.0",
			ReleasedAt:   time.Now().Add(-72 * time.Hour),
			RiskScore:    0.2,
			Outcome:      memory.OutcomeSuccess,
			Actor:        developerActor,
			FilesChanged: 5,
		},
		{
			ID:           "rel-002",
			Repository:   "owner/repo",
			Version:      "1.1.0",
			ReleasedAt:   time.Now().Add(-24 * time.Hour),
			RiskScore:    0.5,
			Outcome:      memory.OutcomeSuccess,
			Actor:        developerActor,
			FilesChanged: 10,
		},
		{
			ID:              "rel-003",
			Repository:      "owner/repo",
			Version:         "1.2.0",
			ReleasedAt:      time.Now(),
			RiskScore:       0.8,
			Outcome:         memory.OutcomeFailed,
			Actor:           ciActor,
			FilesChanged:    15,
			BreakingChanges: 2,
		},
	}

	for _, release := range releases {
		if err := store.RecordRelease(ctx, release); err != nil {
			t.Fatalf("Failed to record release: %v", err)
		}
	}

	// Get release history
	history, err := store.GetReleaseHistory(ctx, "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory failed: %v", err)
	}

	if len(history) != 3 {
		t.Errorf("Expected 3 releases, got %d", len(history))
	}

	// Get actor metrics - use full actor ID format
	metrics, err := store.GetActorMetrics(ctx, developerActor.ID)
	if err != nil {
		t.Fatalf("GetActorMetrics failed: %v", err)
	}

	if metrics.TotalReleases != 2 {
		t.Errorf("Expected 2 releases for developer, got %d", metrics.TotalReleases)
	}

	if metrics.SuccessfulReleases != 2 {
		t.Errorf("Expected 2 successful releases, got %d", metrics.SuccessfulReleases)
	}
}

func TestCGP_OutcomeTracker_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	store, err := memory.NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create actor using proper constructor
	devActor := cgp.NewHumanActor("developer@example.com", "developer")

	// Record releases directly via store
	successRecord := &memory.ReleaseRecord{
		ID:           "rel-001",
		Repository:   "owner/repo",
		Version:      "v1.0.0",
		ReleasedAt:   time.Now().Add(-1 * time.Hour),
		RiskScore:    0.2,
		Outcome:      memory.OutcomeSuccess,
		Actor:        devActor,
		FilesChanged: 5,
	}

	failedRecord := &memory.ReleaseRecord{
		ID:           "rel-002",
		Repository:   "owner/repo",
		Version:      "v1.1.0",
		ReleasedAt:   time.Now(),
		RiskScore:    0.6,
		Outcome:      memory.OutcomeFailed,
		Actor:        devActor,
		FilesChanged: 8,
		Metadata:     map[string]string{"failure_reason": "tests failed"},
	}

	if err := store.RecordRelease(ctx, successRecord); err != nil {
		t.Fatalf("RecordRelease (success) failed: %v", err)
	}

	if err := store.RecordRelease(ctx, failedRecord); err != nil {
		t.Fatalf("RecordRelease (failed) failed: %v", err)
	}

	// Check history
	history, err := store.GetReleaseHistory(ctx, "owner/repo", 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory failed: %v", err)
	}

	if len(history) != 2 {
		t.Errorf("Expected 2 releases, got %d", len(history))
	}

	successCount := 0
	failureCount := 0
	for _, r := range history {
		switch r.Outcome {
		case memory.OutcomeSuccess:
			successCount++
		case memory.OutcomeFailed:
			failureCount++
		}
	}

	if successCount != 1 {
		t.Errorf("Expected 1 success, got %d", successCount)
	}
	if failureCount != 1 {
		t.Errorf("Expected 1 failure, got %d", failureCount)
	}
}

func TestCGP_FullGovernanceFlow(t *testing.T) {
	RequireGitVersion(t, "2.0.0")

	// Set up git repository
	repo := NewTestRepo(t)
	repo.WriteFile("README.md", "# Test Project")
	repo.Commit("feat: initial setup")
	repo.Tag("v1.0.0")

	// Add changes
	repo.WriteFile("feature.go", "package main\n\nfunc NewFeature() {}")
	repo.Commit("feat: add new feature")

	repo.WriteFile("fix.go", "package main\n\nfunc FixBug() {}")
	repo.Commit("fix: resolve edge case bug")

	// Set up governance components
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create policy file
	policyContent := `
rule "minor-version-check" {
    priority = 100
    description = "Minor versions from humans are approved"

    when {
        intent.suggestedBump == "minor" AND actor.kind == "human" AND risk.score < 0.5
    }

    then {
        approve()
    }
}
`
	policyPath := filepath.Join(tmpDir, "release.policy")
	if err := os.WriteFile(policyPath, []byte(policyContent), 0o644); err != nil {
		t.Fatalf("Failed to write policy file: %v", err)
	}

	// Load policies using the DSL loader
	loadedPolicies := dsl.MustLoadDir(tmpDir)

	// Convert []*policy.Policy to []policy.Policy for NewWithPolicies
	var policies []policy.Policy
	for _, p := range loadedPolicies {
		policies = append(policies, *p)
	}

	// Create memory store
	memDir := filepath.Join(tmpDir, "memory")
	store, err := memory.NewFileStore(memDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create evaluator with loaded policies
	eval := evaluator.NewWithPolicies(policies,
		evaluator.WithRiskCalculator(risk.NewCalculatorWithDefaults()),
	)

	// Create actor with trust level
	actor := cgp.NewHumanActor("developer@example.com", "developer")
	actor.TrustLevel = cgp.TrustLevelFull

	// Create proposal
	proposal := cgp.NewProposal(
		actor,
		cgp.ProposalScope{
			Repository:  repo.Dir,
			CommitRange: "v1.0.0..HEAD",
		},
		cgp.ProposalIntent{
			Summary:       "New feature release",
			SuggestedBump: "minor",
			Confidence:    0.9,
		},
	)

	analysis := &cgp.ChangeAnalysis{
		Features: 1,
		Fixes:    1,
		Breaking: 0,
		Security: 0,
		BlastRadius: &cgp.BlastRadius{
			FilesChanged: 2,
			LinesChanged: 10,
		},
	}

	// Evaluate
	result, err := eval.Evaluate(ctx, proposal, analysis)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	t.Logf("Decision: %v", result.Decision.Decision)
	t.Logf("Risk Score: %v", result.RiskAssessment.Score)
	t.Logf("Matched Rules: %v", result.PolicyResult.MatchedRules)

	// Verify low-risk minor version is handled appropriately
	if result.RiskAssessment.Score >= 0.5 {
		t.Errorf("Expected risk score < 0.5 for simple changes, got %v", result.RiskAssessment.Score)
	}

	// Record outcome
	releaseRecord := &memory.ReleaseRecord{
		ID:           string(proposal.ID),
		Repository:   repo.Dir,
		Version:      "v1.1.0",
		Actor:        actor,
		RiskScore:    result.RiskAssessment.Score,
		Decision:     result.Decision.Decision,
		Outcome:      memory.OutcomeSuccess,
		ReleasedAt:   time.Now(),
		FilesChanged: 2,
		LinesChanged: 10,
	}

	if err := store.RecordRelease(ctx, releaseRecord); err != nil {
		t.Fatalf("RecordRelease failed: %v", err)
	}

	// Verify recorded
	history, err := store.GetReleaseHistory(ctx, repo.Dir, 10)
	if err != nil {
		t.Fatalf("GetReleaseHistory failed: %v", err)
	}

	if len(history) != 1 {
		t.Errorf("Expected 1 release in history, got %d", len(history))
	}
}
