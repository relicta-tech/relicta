package risk

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/internal/cgp"
)

type mockHistoryProvider struct {
	rollbackRate float64
}

func (m *mockHistoryProvider) GetRecentIncidents(ctx context.Context, repository string, limit int) ([]Incident, error) {
	return nil, nil
}

func (m *mockHistoryProvider) GetRollbackRate(ctx context.Context, repository string) (float64, error) {
	if m.rollbackRate < 0 {
		return 0, errors.New("rollback lookup failed")
	}
	return m.rollbackRate, nil
}

func (m *mockHistoryProvider) GetActorHistory(ctx context.Context, actorID string) (*ActorHistory, error) {
	return &ActorHistory{
		TotalReleases:    10,
		SuccessfulCount:  9,
		RollbackCount:    1,
		IncidentCount:    0,
		AverageRiskScore: 0.2,
	}, nil
}

func TestCalculator_Calculate_WithHistoryProvider(t *testing.T) {
	history := &mockHistoryProvider{rollbackRate: 0.25}
	calc := NewCalculatorWithDefaults().WithHistory(history)

	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "History check", Confidence: 0.9},
	)
	analysis := &cgp.ChangeAnalysis{
		Security: 1,
	}

	assessment, err := calc.Calculate(context.Background(), proposal, analysis)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}

	found := false
	for _, factor := range assessment.Factors {
		if factor.Category == "historical_risk" {
			found = true
			if !strings.Contains(factor.Description, "rollback") {
				t.Fatalf("unexpected historical description %q", factor.Description)
			}
		}
	}
	if !found {
		t.Fatal("expected historical risk factor")
	}
	if !strings.Contains(assessment.Summary, "risk") || !strings.Contains(assessment.Summary, "high-severity") {
		t.Fatalf("summary should mention risk and high-severity factors, got %q", assessment.Summary)
	}
}

func TestCalculator_Calculate_WithHistoryError(t *testing.T) {
	history := &mockHistoryProvider{rollbackRate: -1} // triggers error
	calc := NewCalculatorWithDefaults().WithHistory(history)

	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "History error check", Confidence: 0.9},
	)

	assessment, err := calc.Calculate(context.Background(), proposal, nil)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}

	// Should not have historical risk factor when history errors
	for _, factor := range assessment.Factors {
		if factor.Category == "historical_risk" {
			t.Fatal("should not have historical_risk factor when history errors")
		}
	}
}

func TestCalculator_Calculate_ActorTypes(t *testing.T) {
	calc := NewCalculatorWithDefaults()

	actors := []struct {
		kind cgp.ActorKind
	}{
		{cgp.ActorKindHuman},
		{cgp.ActorKindCI},
		{cgp.ActorKindSystem},
		{cgp.ActorKindAgent},
		{cgp.ActorKind("unknown")},
	}

	for _, tt := range actors {
		t.Run(string(tt.kind), func(t *testing.T) {
			proposal := cgp.NewProposal(
				cgp.Actor{Kind: tt.kind, ID: "test"},
				cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
				cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
			)

			assessment, err := calc.Calculate(context.Background(), proposal, nil)
			if err != nil {
				t.Fatalf("Calculate() error = %v", err)
			}

			found := false
			for _, factor := range assessment.Factors {
				if factor.Category == "actor_trust" {
					found = true
					break
				}
			}
			if !found {
				t.Error("should have actor_trust factor")
			}
		})
	}
}

func TestCalculator_Calculate_ModerateHistoricalRisk(t *testing.T) {
	history := &mockHistoryProvider{rollbackRate: 0.15} // moderate
	calc := NewCalculatorWithDefaults().WithHistory(history)

	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Moderate history check", Confidence: 0.9},
	)

	assessment, err := calc.Calculate(context.Background(), proposal, nil)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}

	for _, factor := range assessment.Factors {
		if factor.Category == "historical_risk" {
			if !strings.Contains(factor.Description, "Moderate") {
				t.Errorf("expected moderate description, got %q", factor.Description)
			}
			return
		}
	}
	t.Fatal("expected historical_risk factor")
}

func TestCalculator_Calculate_LowHistoricalRisk(t *testing.T) {
	history := &mockHistoryProvider{rollbackRate: 0.05} // low
	calc := NewCalculatorWithDefaults().WithHistory(history)

	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Low history check", Confidence: 0.9},
	)

	assessment, err := calc.Calculate(context.Background(), proposal, nil)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}

	for _, factor := range assessment.Factors {
		if factor.Category == "historical_risk" {
			if !strings.Contains(factor.Description, "Low") {
				t.Errorf("expected Low description, got %q", factor.Description)
			}
			return
		}
	}
	t.Fatal("expected historical_risk factor")
}

func TestCalculator_Calculate_APIChanges(t *testing.T) {
	calc := NewCalculatorWithDefaults()

	proposal := cgp.NewProposal(
		cgp.NewHumanActor("john@example.com", "John"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "API changes", Confidence: 0.9},
	)

	analysis := &cgp.ChangeAnalysis{
		APIChanges: []cgp.APIChange{
			{Type: "removed", Symbol: "GetUsers", Location: "api.go", Breaking: true},
			{Type: "added", Symbol: "ListOrders", Location: "api.go"},
		},
		DependencyImpact: &cgp.DependencyImpact{
			DirectDependents:     5,
			TransitiveDependents: 20,
		},
		BlastRadius: &cgp.BlastRadius{
			FilesChanged: 50,
			LinesChanged: 500,
			Score:        0.7,
		},
	}

	assessment, err := calc.Calculate(context.Background(), proposal, analysis)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}

	if assessment.Score <= 0 {
		t.Error("should have non-zero risk score with API and breaking changes")
	}
}

func TestCalculator_Calculate_APIChanges_AllTypes(t *testing.T) {
	calc := NewCalculatorWithDefaults()

	proposal := cgp.NewProposal(
		cgp.NewHumanActor("dev@example.com", "Dev"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "API changes all types", Confidence: 0.9},
	)

	analysis := &cgp.ChangeAnalysis{
		APIChanges: []cgp.APIChange{
			{Type: "modified", Symbol: "UpdateUser", Location: "api.go", Breaking: true},
			{Type: "modified", Symbol: "GetConfig", Location: "api.go", Breaking: false},
			{Type: "deprecated", Symbol: "OldMethod", Location: "api.go"},
			{Type: "added", Symbol: "NewMethod", Location: "api.go"},
		},
	}

	assessment, err := calc.Calculate(context.Background(), proposal, analysis)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}

	if assessment.Score <= 0 {
		t.Error("should have non-zero risk score")
	}

	// Should have api_change factor
	found := false
	for _, f := range assessment.Factors {
		if f.Category == "api_change" {
			found = true
			if f.Severity != cgp.SeverityHigh {
				t.Errorf("Severity = %s, want high (has breaking changes)", f.Severity)
			}
		}
	}
	if !found {
		t.Error("expected api_change risk factor")
	}
}

func TestCalculator_Calculate_APIChanges_NonBreakingMedium(t *testing.T) {
	calc := NewCalculatorWithDefaults()

	proposal := cgp.NewProposal(
		cgp.NewHumanActor("dev@example.com", "Dev"),
		cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Non-breaking API changes", Confidence: 0.9},
	)

	// All deprecated → high normalized score (0.2 each, > 0.5 normalized) → medium severity
	analysis := &cgp.ChangeAnalysis{
		APIChanges: []cgp.APIChange{
			{Type: "deprecated", Symbol: "Method1", Location: "api.go"},
			{Type: "deprecated", Symbol: "Method2", Location: "api.go"},
			{Type: "deprecated", Symbol: "Method3", Location: "api.go"},
		},
	}

	assessment, err := calc.Calculate(context.Background(), proposal, analysis)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}

	if assessment.Score <= 0 {
		t.Error("should have non-zero risk score")
	}
}

func TestClampOutOfRange(t *testing.T) {
	if got := clamp(-1, 0, 1); got != 0 {
		t.Fatalf("clamp below range = %v, want 0", got)
	}
	if got := clamp(2, 0, 1); got != 1 {
		t.Fatalf("clamp above range = %v, want 1", got)
	}
}

func TestGenerateSummaryHighRisk(t *testing.T) {
	factors := []cgp.RiskFactor{
		{Severity: cgp.SeverityHigh},
		{Severity: cgp.SeverityCritical},
	}
	high := generateSummary(0.85, factors)
	if !strings.Contains(high, string(cgp.SeverityCritical)+" risk") {
		t.Fatalf("expected severity string in summary, got %q", high)
	}
	if !strings.Contains(high, "high-severity") {
		t.Fatalf("expected high-severity mention, got %q", high)
	}
}
