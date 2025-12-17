package risk

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/internal/cgp"
)

func TestDefaultWeights(t *testing.T) {
	weights := DefaultWeights()

	// Verify weights are set
	if weights.APIChanges == 0 {
		t.Error("APIChanges weight should not be 0")
	}
	if weights.DependencyImpact == 0 {
		t.Error("DependencyImpact weight should not be 0")
	}
	if weights.BlastRadius == 0 {
		t.Error("BlastRadius weight should not be 0")
	}

	// Verify weights approximately sum to 1
	total := weights.APIChanges + weights.DependencyImpact + weights.BlastRadius +
		weights.CodeComplexity + weights.TestCoverage + weights.ActorTrust +
		weights.HistoricalRisk + weights.SecurityImpact

	if total < 0.9 || total > 1.1 {
		t.Errorf("Weights should sum to approximately 1.0, got %v", total)
	}
}

func TestNewCalculator(t *testing.T) {
	calc := NewCalculator(DefaultWeights())
	if calc == nil {
		t.Error("NewCalculator() should not return nil")
	}
}

func TestNewCalculatorWithDefaults(t *testing.T) {
	calc := NewCalculatorWithDefaults()
	if calc == nil {
		t.Error("NewCalculatorWithDefaults() should not return nil")
	}
}

func TestCalculator_Calculate_NoAnalysis(t *testing.T) {
	calc := NewCalculatorWithDefaults()
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
	)

	assessment, err := calc.Calculate(context.Background(), proposal, nil)
	if err != nil {
		t.Errorf("Calculate() error = %v", err)
	}
	if assessment == nil {
		t.Fatal("Calculate() should return assessment")
	}
	// With no analysis, only actor trust should contribute
	if assessment.Score < 0 || assessment.Score > 1 {
		t.Errorf("Score should be between 0 and 1, got %v", assessment.Score)
	}
}

func TestCalculator_Calculate_WithAPIChanges(t *testing.T) {
	calc := NewCalculatorWithDefaults()
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
	)
	analysis := &cgp.ChangeAnalysis{
		APIChanges: []cgp.APIChange{
			{Type: "removed", Symbol: "OldFunc", Breaking: true},
			{Type: "modified", Symbol: "UpdatedFunc", Breaking: true},
		},
	}

	assessment, err := calc.Calculate(context.Background(), proposal, analysis)
	if err != nil {
		t.Errorf("Calculate() error = %v", err)
	}

	// Should have API change factor
	hasAPIFactor := false
	for _, factor := range assessment.Factors {
		if factor.Category == "api_change" {
			hasAPIFactor = true
			if factor.Severity != cgp.SeverityHigh {
				t.Errorf("API change factor severity should be high for breaking changes, got %v", factor.Severity)
			}
		}
	}
	if !hasAPIFactor {
		t.Error("Should have API change risk factor")
	}
}

func TestCalculator_Calculate_WithBlastRadius(t *testing.T) {
	calc := NewCalculatorWithDefaults()
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
	)
	analysis := &cgp.ChangeAnalysis{
		BlastRadius: &cgp.BlastRadius{
			FilesChanged: 100,
			LinesChanged: 5000,
		},
	}

	assessment, err := calc.Calculate(context.Background(), proposal, analysis)
	if err != nil {
		t.Errorf("Calculate() error = %v", err)
	}

	// Should have blast radius factor
	hasBlastFactor := false
	for _, factor := range assessment.Factors {
		if factor.Category == "blast_radius" {
			hasBlastFactor = true
			// Large blast radius should be high severity
			if factor.Score < 0.5 {
				t.Errorf("Large blast radius should have high score, got %v", factor.Score)
			}
		}
	}
	if !hasBlastFactor {
		t.Error("Should have blast radius risk factor")
	}
}

func TestCalculator_Calculate_WithSecurityImpact(t *testing.T) {
	calc := NewCalculatorWithDefaults()
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
	)
	analysis := &cgp.ChangeAnalysis{
		Security: 3,
	}

	assessment, err := calc.Calculate(context.Background(), proposal, analysis)
	if err != nil {
		t.Errorf("Calculate() error = %v", err)
	}

	// Should have security factor
	hasSecurityFactor := false
	for _, factor := range assessment.Factors {
		if factor.Category == "security_impact" {
			hasSecurityFactor = true
		}
	}
	if !hasSecurityFactor {
		t.Error("Should have security impact risk factor")
	}
}

func TestCalculator_Calculate_DependencyImpact(t *testing.T) {
	calc := NewCalculatorWithDefaults()
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
	)
	analysis := &cgp.ChangeAnalysis{
		DependencyImpact: &cgp.DependencyImpact{
			DirectDependents:     150,
			TransitiveDependents: 2000,
		},
	}

	assessment, err := calc.Calculate(context.Background(), proposal, analysis)
	if err != nil {
		t.Errorf("Calculate() error = %v", err)
	}

	// Should have dependency factor
	hasDepFactor := false
	for _, factor := range assessment.Factors {
		if factor.Category == "dependency_impact" {
			hasDepFactor = true
			// High dependency count should result in high score
			if factor.Score < 0.8 {
				t.Errorf("High dependency count should have high score, got %v", factor.Score)
			}
		}
	}
	if !hasDepFactor {
		t.Error("Should have dependency impact risk factor")
	}
}

func TestCalculator_ActorTrust(t *testing.T) {
	calc := NewCalculatorWithDefaults()

	tests := []struct {
		name             string
		actor            cgp.Actor
		expectedRange    [2]float64 // [min, max] for score
		expectedSeverity cgp.Severity
	}{
		{
			name:             "human actor",
			actor:            cgp.NewHumanActor("john@example.com", "John"),
			expectedRange:    [2]float64{0.0, 0.2},
			expectedSeverity: cgp.SeverityLow,
		},
		{
			name:             "ci actor",
			actor:            cgp.NewCIActor("github-actions", "release", "123"),
			expectedRange:    [2]float64{0.1, 0.3},
			expectedSeverity: cgp.SeverityLow,
		},
		{
			name:             "agent actor",
			actor:            cgp.NewAgentActor("cursor", "Cursor", "gpt-4"),
			expectedRange:    [2]float64{0.5, 0.7},
			expectedSeverity: cgp.SeverityMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proposal := cgp.NewProposal(
				tt.actor,
				cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
				cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
			)

			assessment, err := calc.Calculate(context.Background(), proposal, nil)
			if err != nil {
				t.Errorf("Calculate() error = %v", err)
			}

			// Find actor trust factor
			for _, factor := range assessment.Factors {
				if factor.Category == "actor_trust" {
					if factor.Score < tt.expectedRange[0] || factor.Score > tt.expectedRange[1] {
						t.Errorf("Actor trust score = %v, expected between %v and %v",
							factor.Score, tt.expectedRange[0], tt.expectedRange[1])
					}
					if factor.Severity != tt.expectedSeverity {
						t.Errorf("Actor trust severity = %v, want %v", factor.Severity, tt.expectedSeverity)
					}
					return
				}
			}
			t.Error("Actor trust factor not found")
		})
	}
}

func TestScoreSeverity(t *testing.T) {
	tests := []struct {
		score    float64
		expected cgp.Severity
	}{
		{0.9, cgp.SeverityCritical},
		{0.8, cgp.SeverityCritical},
		{0.7, cgp.SeverityHigh},
		{0.6, cgp.SeverityHigh},
		{0.5, cgp.SeverityMedium},
		{0.4, cgp.SeverityMedium},
		{0.3, cgp.SeverityLow},
		{0.1, cgp.SeverityLow},
		{0.0, cgp.SeverityLow},
	}

	for _, tt := range tests {
		result := scoreSeverity(tt.score)
		if result != tt.expected {
			t.Errorf("scoreSeverity(%v) = %v, want %v", tt.score, result, tt.expected)
		}
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		value, min, max float64
		expected        float64
	}{
		{0.5, 0.0, 1.0, 0.5},
		{-0.5, 0.0, 1.0, 0.0},
		{1.5, 0.0, 1.0, 1.0},
		{0.0, 0.0, 1.0, 0.0},
		{1.0, 0.0, 1.0, 1.0},
	}

	for _, tt := range tests {
		result := clamp(tt.value, tt.min, tt.max)
		if result != tt.expected {
			t.Errorf("clamp(%v, %v, %v) = %v, want %v",
				tt.value, tt.min, tt.max, result, tt.expected)
		}
	}
}

func TestGenerateSummary(t *testing.T) {
	tests := []struct {
		name    string
		score   float64
		factors []cgp.RiskFactor
		expect  string
	}{
		{
			name:    "no factors",
			score:   0,
			factors: []cgp.RiskFactor{},
			expect:  "No risk factors",
		},
		{
			name:  "high severity factor",
			score: 0.8,
			factors: []cgp.RiskFactor{
				{Category: "api", Severity: cgp.SeverityHigh},
			},
			expect: "1 high-severity",
		},
		{
			name:  "low severity factors",
			score: 0.3,
			factors: []cgp.RiskFactor{
				{Category: "api", Severity: cgp.SeverityLow},
				{Category: "blast", Severity: cgp.SeverityLow},
			},
			expect: "2 factors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateSummary(tt.score, tt.factors)
			if tt.expect != "" && !containsSubstring(result, tt.expect) {
				t.Errorf("generateSummary() = %v, should contain %v", result, tt.expect)
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && containsSubstringHelper(s, substr)))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestCalculator_HighRiskScenario(t *testing.T) {
	calc := NewCalculatorWithDefaults()
	proposal := cgp.NewProposal(
		cgp.NewAgentActor("cursor", "Cursor", "gpt-4"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Major refactoring", Confidence: 0.5},
	)
	analysis := &cgp.ChangeAnalysis{
		Features: 0,
		Fixes:    0,
		Breaking: 5,
		Security: 2,
		APIChanges: []cgp.APIChange{
			{Type: "removed", Symbol: "Func1", Breaking: true},
			{Type: "removed", Symbol: "Func2", Breaking: true},
			{Type: "removed", Symbol: "Func3", Breaking: true},
		},
		BlastRadius: &cgp.BlastRadius{
			FilesChanged: 200,
			LinesChanged: 10000,
		},
		DependencyImpact: &cgp.DependencyImpact{
			DirectDependents:     200,
			TransitiveDependents: 5000,
		},
	}

	assessment, err := calc.Calculate(context.Background(), proposal, analysis)
	if err != nil {
		t.Errorf("Calculate() error = %v", err)
	}

	// High risk scenario should have high score
	if assessment.Score < 0.6 {
		t.Errorf("High risk scenario should have score >= 0.6, got %v", assessment.Score)
	}

	// Should have multiple risk factors
	if len(assessment.Factors) < 4 {
		t.Errorf("High risk scenario should have multiple factors, got %d", len(assessment.Factors))
	}

	// Should be high or critical severity
	if assessment.Severity != cgp.SeverityHigh && assessment.Severity != cgp.SeverityCritical {
		t.Errorf("High risk scenario should be high/critical severity, got %v", assessment.Severity)
	}
}

func TestCalculator_LowRiskScenario(t *testing.T) {
	calc := NewCalculatorWithDefaults()
	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Minor fix", Confidence: 0.95},
	)
	analysis := &cgp.ChangeAnalysis{
		Features: 0,
		Fixes:    1,
		Breaking: 0,
		Security: 0,
		BlastRadius: &cgp.BlastRadius{
			FilesChanged: 2,
			LinesChanged: 10,
		},
	}

	assessment, err := calc.Calculate(context.Background(), proposal, analysis)
	if err != nil {
		t.Errorf("Calculate() error = %v", err)
	}

	// Low risk scenario should have low score
	if assessment.Score > 0.4 {
		t.Errorf("Low risk scenario should have score <= 0.4, got %v", assessment.Score)
	}

	// Should be low or medium severity
	if assessment.Severity != cgp.SeverityLow && assessment.Severity != cgp.SeverityMedium {
		t.Errorf("Low risk scenario should be low/medium severity, got %v", assessment.Severity)
	}
}
