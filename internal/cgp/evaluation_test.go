package cgp

import (
	"testing"
	"time"
)

func TestNewEvaluation(t *testing.T) {
	eval := NewEvaluation("proposal-123")

	if eval.CGPVersion != Version {
		t.Errorf("CGPVersion = %s, want %s", eval.CGPVersion, Version)
	}
	if eval.Type != MessageTypeEvaluation {
		t.Errorf("Type = %s, want %s", eval.Type, MessageTypeEvaluation)
	}
	if eval.ProposalID != "proposal-123" {
		t.Errorf("ProposalID = %s, want proposal-123", eval.ProposalID)
	}
	if eval.Status != EvaluationStatusComplete {
		t.Errorf("Status = %s, want %s", eval.Status, EvaluationStatusComplete)
	}
	if eval.ID == "" {
		t.Error("ID should not be empty")
	}
	if eval.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestEvaluationStatus_IsValid(t *testing.T) {
	tests := []struct {
		status EvaluationStatus
		valid  bool
	}{
		{EvaluationStatusComplete, true},
		{EvaluationStatusPartial, true},
		{EvaluationStatusFailed, true},
		{EvaluationStatusTimeout, true},
		{EvaluationStatus("unknown"), false},
		{EvaluationStatus(""), false},
	}

	for _, tt := range tests {
		if got := tt.status.IsValid(); got != tt.valid {
			t.Errorf("%s.IsValid() = %v, want %v", tt.status, got, tt.valid)
		}
	}
}

func TestGovernanceEvaluation_Validate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *GovernanceEvaluation
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid evaluation",
			setup: func() *GovernanceEvaluation {
				return NewEvaluation("proposal-123")
			},
			wantErr: false,
		},
		{
			name: "missing CGP version",
			setup: func() *GovernanceEvaluation {
				e := NewEvaluation("proposal-123")
				e.CGPVersion = ""
				return e
			},
			wantErr: true,
			errMsg:  "CGP version is required",
		},
		{
			name: "invalid message type",
			setup: func() *GovernanceEvaluation {
				e := NewEvaluation("proposal-123")
				e.Type = MessageTypeDecision
				return e
			},
			wantErr: true,
			errMsg:  "invalid message type",
		},
		{
			name: "missing evaluation ID",
			setup: func() *GovernanceEvaluation {
				e := NewEvaluation("proposal-123")
				e.ID = ""
				return e
			},
			wantErr: true,
			errMsg:  "evaluation ID is required",
		},
		{
			name: "missing proposal ID",
			setup: func() *GovernanceEvaluation {
				e := NewEvaluation("")
				return e
			},
			wantErr: true,
			errMsg:  "proposal ID is required",
		},
		{
			name: "invalid status",
			setup: func() *GovernanceEvaluation {
				e := NewEvaluation("proposal-123")
				e.Status = EvaluationStatus("invalid")
				return e
			},
			wantErr: true,
			errMsg:  "invalid evaluation status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eval := tt.setup()
			err := eval.Validate()

			if tt.wantErr {
				if err == nil {
					t.Error("Validate() expected error, got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want containing %v", err, tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestGovernanceEvaluation_AddWarning(t *testing.T) {
	eval := NewEvaluation("proposal-123").
		AddWarning("W001", "Minor style issue", "formatting")

	if len(eval.Warnings) != 1 {
		t.Fatalf("Warnings count = %d, want 1", len(eval.Warnings))
	}

	if eval.Warnings[0].Code != "W001" {
		t.Errorf("Warning.Code = %s, want W001", eval.Warnings[0].Code)
	}
	if !eval.HasWarnings() {
		t.Error("HasWarnings() should return true")
	}
}

func TestGovernanceEvaluation_AddError(t *testing.T) {
	eval := NewEvaluation("proposal-123").
		AddError("E001", "Critical issue", "security", true)

	if len(eval.Errors) != 1 {
		t.Fatalf("Errors count = %d, want 1", len(eval.Errors))
	}

	if eval.Errors[0].Code != "E001" {
		t.Errorf("Error.Code = %s, want E001", eval.Errors[0].Code)
	}
	if !eval.Errors[0].Fatal {
		t.Error("Error.Fatal should be true")
	}
	if !eval.HasErrors() {
		t.Error("HasErrors() should return true")
	}
	if !eval.HasFatalErrors() {
		t.Error("HasFatalErrors() should return true")
	}
}

func TestGovernanceEvaluation_CanProceed(t *testing.T) {
	// Complete with no errors
	eval1 := NewEvaluation("proposal-123")
	if !eval1.CanProceed() {
		t.Error("Should be able to proceed when complete with no errors")
	}

	// Complete with non-fatal error
	eval2 := NewEvaluation("proposal-123").
		AddError("E001", "Non-fatal issue", "warning", false)
	if !eval2.CanProceed() {
		t.Error("Should be able to proceed with non-fatal errors")
	}

	// Complete with fatal error
	eval3 := NewEvaluation("proposal-123").
		AddError("E002", "Fatal issue", "security", true)
	if eval3.CanProceed() {
		t.Error("Should not be able to proceed with fatal errors")
	}

	// Failed status
	eval4 := NewEvaluation("proposal-123")
	eval4.Status = EvaluationStatusFailed
	if eval4.CanProceed() {
		t.Error("Should not be able to proceed when status is failed")
	}
}

func TestGovernanceEvaluation_SuggestedDecision(t *testing.T) {
	// Fatal error -> Rejected
	evalFatal := NewEvaluation("proposal-123").
		AddError("E001", "Fatal", "security", true)
	if evalFatal.SuggestedDecision() != DecisionRejected {
		t.Errorf("With fatal error, suggested = %s, want %s", evalFatal.SuggestedDecision(), DecisionRejected)
	}

	// No risk assessment -> Deferred
	evalNoRisk := NewEvaluation("proposal-123")
	if evalNoRisk.SuggestedDecision() != DecisionDeferred {
		t.Errorf("Without risk assessment, suggested = %s, want %s", evalNoRisk.SuggestedDecision(), DecisionDeferred)
	}

	// High risk -> Approval required
	evalHighRisk := NewEvaluation("proposal-123").
		WithRiskAssessment(&RiskAssessment{OverallScore: 0.8})
	if evalHighRisk.SuggestedDecision() != DecisionApprovalRequired {
		t.Errorf("With high risk, suggested = %s, want %s", evalHighRisk.SuggestedDecision(), DecisionApprovalRequired)
	}

	// Low risk -> Approved
	evalLowRisk := NewEvaluation("proposal-123").
		WithRiskAssessment(&RiskAssessment{OverallScore: 0.2})
	if evalLowRisk.SuggestedDecision() != DecisionApproved {
		t.Errorf("With low risk, suggested = %s, want %s", evalLowRisk.SuggestedDecision(), DecisionApproved)
	}

	// Medium risk -> Approval required
	evalMedRisk := NewEvaluation("proposal-123").
		WithRiskAssessment(&RiskAssessment{OverallScore: 0.5})
	if evalMedRisk.SuggestedDecision() != DecisionApprovalRequired {
		t.Errorf("With medium risk, suggested = %s, want %s", evalMedRisk.SuggestedDecision(), DecisionApprovalRequired)
	}

	// Policy requires approval
	evalPolicyApproval := NewEvaluation("proposal-123").
		WithRiskAssessment(&RiskAssessment{OverallScore: 0.2}).
		AddPolicyResult(PolicyResult{
			PolicyID: "policy-1",
			Matched:  true,
			Decision: DecisionApprovalRequired,
		})
	if evalPolicyApproval.SuggestedDecision() != DecisionApprovalRequired {
		t.Errorf("With approval policy, suggested = %s, want %s", evalPolicyApproval.SuggestedDecision(), DecisionApprovalRequired)
	}

	// Policy rejects
	evalPolicyReject := NewEvaluation("proposal-123").
		WithRiskAssessment(&RiskAssessment{OverallScore: 0.2}).
		AddPolicyResult(PolicyResult{
			PolicyID: "policy-1",
			Matched:  true,
			Decision: DecisionRejected,
		})
	if evalPolicyReject.SuggestedDecision() != DecisionRejected {
		t.Errorf("With reject policy, suggested = %s, want %s", evalPolicyReject.SuggestedDecision(), DecisionRejected)
	}
}

func TestGovernanceEvaluation_WithDuration(t *testing.T) {
	duration := 500 * time.Millisecond
	eval := NewEvaluation("proposal-123").WithDuration(duration)

	if eval.Duration != duration {
		t.Errorf("Duration = %v, want %v", eval.Duration, duration)
	}
}

func TestGovernanceEvaluation_WithMetadata(t *testing.T) {
	eval := NewEvaluation("proposal-123").
		WithMetadata("key1", "value1").
		WithMetadata("key2", 42)

	if eval.Metadata["key1"] != "value1" {
		t.Errorf("Metadata[key1] = %v, want value1", eval.Metadata["key1"])
	}
	if eval.Metadata["key2"] != 42 {
		t.Errorf("Metadata[key2] = %v, want 42", eval.Metadata["key2"])
	}
}

func TestVersionAnalysis(t *testing.T) {
	eval := NewEvaluation("proposal-123").
		WithVersionAnalysis(&VersionAnalysis{
			CurrentVersion:     "1.2.3",
			RecommendedVersion: "1.3.0",
			BumpType:           BumpTypeMinor,
			Reasoning:          []string{"New features added"},
		})

	if eval.VersionAnalysis == nil {
		t.Fatal("VersionAnalysis should not be nil")
	}
	if eval.VersionAnalysis.CurrentVersion != "1.2.3" {
		t.Errorf("CurrentVersion = %s, want 1.2.3", eval.VersionAnalysis.CurrentVersion)
	}
	if eval.VersionAnalysis.RecommendedVersion != "1.3.0" {
		t.Errorf("RecommendedVersion = %s, want 1.3.0", eval.VersionAnalysis.RecommendedVersion)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
