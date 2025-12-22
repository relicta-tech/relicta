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
