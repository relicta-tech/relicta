package supplychain

import (
	"testing"
)

func TestAnalyzer_CVEFix_AutoApprove(t *testing.T) {
	a := NewAnalyzer(nil)

	changes := []DependencyChange{
		{
			Name:       "golang.org/x/crypto",
			Ecosystem:  "go",
			OldVersion: "v0.17.0",
			NewVersion: "v0.18.0",
			ChangeType: ChangeMinor,
			HasCVEFix:  true,
			CVEs:       []string{"CVE-2024-1234"},
		},
	}

	result := a.Analyze(changes)

	if result.RiskScore != 0.1 {
		t.Errorf("expected risk score 0.1 for CVE fix, got %f", result.RiskScore)
	}
	if result.Recommendation != "auto-approve" {
		t.Errorf("expected auto-approve for CVE fix, got %s", result.Recommendation)
	}
}

func TestAnalyzer_PatchUpdate_LowRisk(t *testing.T) {
	a := NewAnalyzer(nil)

	changes := []DependencyChange{
		{
			Name:       "github.com/stretchr/testify",
			Ecosystem:  "go",
			OldVersion: "v1.8.3",
			NewVersion: "v1.8.4",
			ChangeType: ChangePatch,
		},
	}

	result := a.Analyze(changes)

	if result.RiskScore != 0.15 {
		t.Errorf("expected risk score 0.15 for patch update, got %f", result.RiskScore)
	}
	if result.Recommendation != "auto-approve" {
		t.Errorf("expected auto-approve for patch update, got %s", result.Recommendation)
	}
}

func TestAnalyzer_MajorUpdate_HighRisk(t *testing.T) {
	a := NewAnalyzer(nil)

	changes := []DependencyChange{
		{
			Name:       "google.golang.org/grpc",
			Ecosystem:  "go",
			OldVersion: "v1.60.0",
			NewVersion: "v2.0.0",
			ChangeType: ChangeMajor,
		},
	}

	result := a.Analyze(changes)

	if result.RiskScore != 0.7 {
		t.Errorf("expected risk score 0.7 for major update, got %f", result.RiskScore)
	}
	if result.Recommendation != "block" {
		t.Errorf("expected block for major update, got %s", result.Recommendation)
	}
}

func TestAnalyzer_MultipleMajors_Penalty(t *testing.T) {
	a := NewAnalyzer(nil)

	changes := []DependencyChange{
		{
			Name:       "google.golang.org/grpc",
			Ecosystem:  "go",
			OldVersion: "v1.60.0",
			NewVersion: "v2.0.0",
			ChangeType: ChangeMajor,
		},
		{
			Name:       "github.com/go-redis/redis",
			Ecosystem:  "go",
			OldVersion: "v8.0.0",
			NewVersion: "v9.0.0",
			ChangeType: ChangeMajor,
		},
	}

	result := a.Analyze(changes)

	// Two major updates: (0.7 + 0.7 + 0.1 penalty) / 2 = 0.75
	expectedScore := 0.75
	if result.RiskScore != expectedScore {
		t.Errorf("expected risk score %f for two major updates with penalty, got %f",
			expectedScore, result.RiskScore)
	}
	if result.Recommendation != "block" {
		t.Errorf("expected block for multiple major updates, got %s", result.Recommendation)
	}
}

func TestAnalyzer_TransitiveDeps_LowerRisk(t *testing.T) {
	a := NewAnalyzer(nil)

	changes := []DependencyChange{
		{
			Name:         "golang.org/x/sys",
			Ecosystem:    "go",
			OldVersion:   "v0.15.0",
			NewVersion:   "v0.16.0",
			ChangeType:   ChangeMinor,
			IsTransitive: true,
		},
	}

	result := a.Analyze(changes)

	// Minor (0.3) * transitive multiplier (0.5) = 0.15
	expectedScore := 0.15
	if result.RiskScore != expectedScore {
		t.Errorf("expected risk score %f for transitive minor update, got %f",
			expectedScore, result.RiskScore)
	}
}

func TestAnalyzer_MixedChanges_WeightedAverage(t *testing.T) {
	a := NewAnalyzer(nil)

	changes := []DependencyChange{
		{
			Name:       "github.com/stretchr/testify",
			Ecosystem:  "go",
			OldVersion: "v1.8.3",
			NewVersion: "v1.8.4",
			ChangeType: ChangePatch,
		},
		{
			Name:       "google.golang.org/grpc",
			Ecosystem:  "go",
			OldVersion: "v1.60.0",
			NewVersion: "v2.0.0",
			ChangeType: ChangeMajor,
		},
	}

	result := a.Analyze(changes)

	// (0.15 + 0.7) / 2 = 0.425
	expectedScore := 0.425
	if result.RiskScore != expectedScore {
		t.Errorf("expected risk score %f for mixed changes, got %f",
			expectedScore, result.RiskScore)
	}
	if result.Recommendation != "review" {
		t.Errorf("expected review for mixed changes, got %s", result.Recommendation)
	}
}

func TestAnalyzer_NewDependency_MediumHighRisk(t *testing.T) {
	a := NewAnalyzer(nil)

	changes := []DependencyChange{
		{
			Name:       "github.com/new/package",
			Ecosystem:  "go",
			NewVersion: "v1.0.0",
			ChangeType: ChangeNew,
		},
	}

	result := a.Analyze(changes)

	if result.RiskScore != 0.5 {
		t.Errorf("expected risk score 0.5 for new dependency, got %f", result.RiskScore)
	}
	if result.Recommendation != "review" {
		t.Errorf("expected review for new dependency, got %s", result.Recommendation)
	}
}

func TestAnalyzer_RemovedDependency_MediumRisk(t *testing.T) {
	a := NewAnalyzer(nil)

	changes := []DependencyChange{
		{
			Name:       "github.com/old/package",
			Ecosystem:  "go",
			OldVersion: "v1.0.0",
			ChangeType: ChangeRemoved,
		},
	}

	result := a.Analyze(changes)

	if result.RiskScore != 0.4 {
		t.Errorf("expected risk score 0.4 for removed dependency, got %f", result.RiskScore)
	}
	if result.Recommendation != "review" {
		t.Errorf("expected review for removed dependency, got %s", result.Recommendation)
	}
}

func TestAnalyzer_EmptyChanges(t *testing.T) {
	a := NewAnalyzer(nil)

	result := a.Analyze(nil)

	if result.RiskScore != 0.0 {
		t.Errorf("expected risk score 0.0 for empty changes, got %f", result.RiskScore)
	}
	if result.Recommendation != "auto-approve" {
		t.Errorf("expected auto-approve for empty changes, got %s", result.Recommendation)
	}
}

func TestAnalyzer_TransitiveCVEFix(t *testing.T) {
	a := NewAnalyzer(nil)

	changes := []DependencyChange{
		{
			Name:         "golang.org/x/net",
			Ecosystem:    "go",
			OldVersion:   "v0.17.0",
			NewVersion:   "v0.18.0",
			ChangeType:   ChangeMinor,
			HasCVEFix:    true,
			CVEs:         []string{"CVE-2023-9999"},
			IsTransitive: true,
		},
	}

	result := a.Analyze(changes)

	// CVE fix = 0.1 (transitive multiplier does not apply to CVE fixes)
	if result.RiskScore != 0.1 {
		t.Errorf("expected risk score 0.1 for transitive CVE fix, got %f", result.RiskScore)
	}
}

func TestAnalyzer_ThreeMajors_IncreasedPenalty(t *testing.T) {
	a := NewAnalyzer(nil)

	changes := []DependencyChange{
		{Name: "pkg/a", Ecosystem: "go", OldVersion: "v1.0.0", NewVersion: "v2.0.0", ChangeType: ChangeMajor},
		{Name: "pkg/b", Ecosystem: "go", OldVersion: "v3.0.0", NewVersion: "v4.0.0", ChangeType: ChangeMajor},
		{Name: "pkg/c", Ecosystem: "go", OldVersion: "v5.0.0", NewVersion: "v6.0.0", ChangeType: ChangeMajor},
	}

	result := a.Analyze(changes)

	// 3 majors: (0.7 + 0.7 + 0.7 + 0.2 penalty) / 3 = 0.7666...
	expectedMin := 0.76
	expectedMax := 0.77
	if result.RiskScore < expectedMin || result.RiskScore > expectedMax {
		t.Errorf("expected risk score between %f and %f for three major updates, got %f",
			expectedMin, expectedMax, result.RiskScore)
	}
}

func TestPolicy_DefaultPolicy(t *testing.T) {
	p := DefaultPolicy()

	if !p.AutoApproveCVEFixes {
		t.Error("expected AutoApproveCVEFixes to be true")
	}
	if !p.AutoApprovePatch {
		t.Error("expected AutoApprovePatch to be true")
	}
	if p.AutoApproveMinor {
		t.Error("expected AutoApproveMinor to be false")
	}
	if !p.RequireReviewForMajor {
		t.Error("expected RequireReviewForMajor to be true")
	}
	if p.MaxRiskThreshold != 0.5 {
		t.Errorf("expected MaxRiskThreshold 0.5, got %f", p.MaxRiskThreshold)
	}
}

func TestPolicy_Evaluate_CVEFixAutoApproved(t *testing.T) {
	p := DefaultPolicy()
	a := NewAnalyzer(nil)

	changes := []DependencyChange{
		{
			Name:       "golang.org/x/crypto",
			Ecosystem:  "go",
			OldVersion: "v0.17.0",
			NewVersion: "v0.18.0",
			ChangeType: ChangeMinor,
			HasCVEFix:  true,
		},
	}

	analysis := a.Analyze(changes)
	decision := p.Evaluate(analysis)

	if decision.Action != "approve" {
		t.Errorf("expected approve for CVE fix, got %s", decision.Action)
	}
}

func TestPolicy_Evaluate_PatchAutoApproved(t *testing.T) {
	p := DefaultPolicy()
	a := NewAnalyzer(nil)

	changes := []DependencyChange{
		{
			Name:       "github.com/stretchr/testify",
			Ecosystem:  "go",
			OldVersion: "v1.8.3",
			NewVersion: "v1.8.4",
			ChangeType: ChangePatch,
		},
	}

	analysis := a.Analyze(changes)
	decision := p.Evaluate(analysis)

	if decision.Action != "approve" {
		t.Errorf("expected approve for patch update, got %s", decision.Action)
	}
}

func TestPolicy_Evaluate_MajorRequiresReview(t *testing.T) {
	p := DefaultPolicy()
	a := NewAnalyzer(nil)

	changes := []DependencyChange{
		{
			Name:       "google.golang.org/grpc",
			Ecosystem:  "go",
			OldVersion: "v1.60.0",
			NewVersion: "v2.0.0",
			ChangeType: ChangeMajor,
		},
	}

	analysis := a.Analyze(changes)
	decision := p.Evaluate(analysis)

	if decision.Action != "block" {
		t.Errorf("expected block for major update (risk 0.7 > threshold 0.5), got %s", decision.Action)
	}
}

func TestPolicy_Evaluate_NilAnalysis(t *testing.T) {
	p := DefaultPolicy()
	decision := p.Evaluate(nil)

	if decision.Action != "approve" {
		t.Errorf("expected approve for nil analysis, got %s", decision.Action)
	}
}

func TestPolicy_Evaluate_HighRiskBlocked(t *testing.T) {
	p := DefaultPolicy()
	p.MaxRiskThreshold = 0.3

	a := NewAnalyzer(nil)
	changes := []DependencyChange{
		{
			Name:       "github.com/new/package",
			Ecosystem:  "go",
			NewVersion: "v1.0.0",
			ChangeType: ChangeNew,
		},
	}

	analysis := a.Analyze(changes)
	decision := p.Evaluate(analysis)

	if decision.Action != "block" {
		t.Errorf("expected block when risk exceeds threshold, got %s", decision.Action)
	}
}
