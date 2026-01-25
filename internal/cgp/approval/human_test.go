package approval

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DefaultTimeout != 24*time.Hour {
		t.Errorf("DefaultTimeout = %v, want 24h", cfg.DefaultTimeout)
	}
	if !cfg.RequireRationale {
		t.Error("RequireRationale should be true by default")
	}
	if cfg.MinRationaleLength != 10 {
		t.Errorf("MinRationaleLength = %d, want 10", cfg.MinRationaleLength)
	}
}

func TestApprover_CreateRequest(t *testing.T) {
	approver := New(nil)
	requester := cgp.NewHumanActor("test@example.com", "Test User")

	risk := &RiskSummary{
		OverallScore: 0.3,
		Severity:     "low",
	}

	policies := []PolicyEvaluation{
		{
			PolicyID:   "policy-1",
			PolicyName: "Low Risk Auto",
			Matched:    true,
			Decision:   "approved",
		},
	}

	ctx := &ApprovalContext{
		Repository:  "org/repo",
		Branch:      "main",
		CommitCount: 5,
	}

	req := approver.CreateRequest(
		"proposal-123",
		"release-456",
		"1.2.3",
		requester,
		risk,
		policies,
		ctx,
	)

	if req.ID == "" {
		t.Error("ID should not be empty")
	}
	if req.ProposalID != "proposal-123" {
		t.Errorf("ProposalID = %s, want proposal-123", req.ProposalID)
	}
	if req.ReleaseID != "release-456" {
		t.Errorf("ReleaseID = %s, want release-456", req.ReleaseID)
	}
	if req.Version != "1.2.3" {
		t.Errorf("Version = %s, want 1.2.3", req.Version)
	}
	if req.RiskAssessment.OverallScore != 0.3 {
		t.Errorf("RiskScore = %f, want 0.3", req.RiskAssessment.OverallScore)
	}
	if len(req.PolicyEvaluations) != 1 {
		t.Errorf("PolicyEvaluations length = %d, want 1", len(req.PolicyEvaluations))
	}
	if req.ExpiresAt.Before(time.Now()) {
		t.Error("ExpiresAt should be in the future")
	}
}

func TestApprover_ValidateResponse_Valid(t *testing.T) {
	approver := New(&Config{
		RequireRationale:   true,
		MinRationaleLength: 10,
	})

	resp := &ApprovalResponse{
		RequestID:     "approval-123",
		Status:        StatusApproved,
		Approver:      cgp.NewHumanActor("approver@example.com", "Approver"),
		Rationale:     "Reviewed and approved - all tests pass",
		RationaleType: RationaleTypeReviewed,
		Timestamp:     time.Now(),
	}

	err := approver.ValidateResponse(resp)
	if err != nil {
		t.Errorf("ValidateResponse failed: %v", err)
	}
}

func TestApprover_ValidateResponse_MissingRationale(t *testing.T) {
	approver := New(&Config{
		RequireRationale: true,
	})

	resp := &ApprovalResponse{
		RequestID:     "approval-123",
		Status:        StatusApproved,
		Approver:      cgp.NewHumanActor("approver@example.com", "Approver"),
		Rationale:     "", // Missing
		RationaleType: RationaleTypeReviewed,
		Timestamp:     time.Now(),
	}

	err := approver.ValidateResponse(resp)
	if err == nil {
		t.Error("ValidateResponse should fail with missing rationale")
	}
}

func TestApprover_ValidateResponse_ShortRationale(t *testing.T) {
	approver := New(&Config{
		RequireRationale:   true,
		MinRationaleLength: 20,
	})

	resp := &ApprovalResponse{
		RequestID:     "approval-123",
		Status:        StatusApproved,
		Approver:      cgp.NewHumanActor("approver@example.com", "Approver"),
		Rationale:     "Too short", // Less than 20 chars
		RationaleType: RationaleTypeReviewed,
		Timestamp:     time.Now(),
	}

	err := approver.ValidateResponse(resp)
	if err == nil {
		t.Error("ValidateResponse should fail with short rationale")
	}
}

func TestApprover_ValidateResponse_EmergencyBypassBlocked(t *testing.T) {
	approver := New(&Config{
		AllowEmergencyBypass: false,
	})

	resp := &ApprovalResponse{
		RequestID:     "approval-123",
		Status:        StatusApproved,
		Approver:      cgp.NewHumanActor("approver@example.com", "Approver"),
		Rationale:     "Emergency hotfix required",
		RationaleType: RationaleTypeEmergency, // Emergency not allowed
		Timestamp:     time.Now(),
	}

	err := approver.ValidateResponse(resp)
	if err == nil {
		t.Error("ValidateResponse should fail when emergency bypass is not allowed")
	}
}

func TestApprover_ValidateResponse_DeferredNoRationale(t *testing.T) {
	approver := New(&Config{
		RequireRationale: true,
	})

	// Deferred status should not require rationale
	resp := &ApprovalResponse{
		RequestID:     "approval-123",
		Status:        StatusDeferred,
		Approver:      cgp.NewHumanActor("approver@example.com", "Approver"),
		Rationale:     "", // Empty is OK for deferred
		RationaleType: RationaleTypeOther,
		Timestamp:     time.Now(),
	}

	err := approver.ValidateResponse(resp)
	if err != nil {
		t.Errorf("ValidateResponse should not fail for deferred without rationale: %v", err)
	}
}

func TestApprover_ProcessApproval_Success(t *testing.T) {
	approver := New(&Config{
		RequireRationale:   true,
		MinRationaleLength: 10,
	})

	req := &ApprovalRequest{
		ID:         "approval-123",
		ProposalID: "proposal-456",
		ReleaseID:  "release-789",
		Version:    "1.0.0",
		RiskAssessment: &RiskSummary{
			OverallScore: 0.25,
			Severity:     "low",
			Factors: []RiskFactorSummary{
				{Category: "scope", Description: "Small change", Score: 0.2},
			},
		},
		ExpiresAt: time.Now().Add(time.Hour),
	}

	resp := &ApprovalResponse{
		RequestID:     "approval-123",
		Status:        StatusApproved,
		Approver:      cgp.NewHumanActor("approver@example.com", "Approver"),
		Rationale:     "Reviewed and approved - all tests pass",
		RationaleType: RationaleTypeReviewed,
		Conditions:    []string{"Must deploy during maintenance window"},
		Timestamp:     time.Now(),
	}

	decision, err := approver.ProcessApproval(context.Background(), req, resp)
	if err != nil {
		t.Fatalf("ProcessApproval failed: %v", err)
	}

	if decision.Decision != cgp.DecisionApproved {
		t.Errorf("Decision = %s, want approved", decision.Decision)
	}
	if len(decision.Rationale) == 0 {
		t.Error("Rationale should not be empty")
	}
	if decision.RiskScore != 0.25 {
		t.Errorf("RiskScore = %f, want 0.25", decision.RiskScore)
	}
	if len(decision.Conditions) != 1 {
		t.Errorf("Conditions length = %d, want 1", len(decision.Conditions))
	}
}

func TestApprover_ProcessApproval_Rejected(t *testing.T) {
	approver := New(&Config{
		RequireRationale:   true,
		MinRationaleLength: 10,
	})

	req := &ApprovalRequest{
		ID:         "approval-123",
		ProposalID: "proposal-456",
		ExpiresAt:  time.Now().Add(time.Hour),
	}

	resp := &ApprovalResponse{
		RequestID:     "approval-123",
		Status:        StatusRejected,
		Approver:      cgp.NewHumanActor("approver@example.com", "Approver"),
		Rationale:     "Security concerns need to be addressed before release",
		RationaleType: RationaleTypeReviewed,
		Timestamp:     time.Now(),
	}

	decision, err := approver.ProcessApproval(context.Background(), req, resp)
	if err != nil {
		t.Fatalf("ProcessApproval failed: %v", err)
	}

	if decision.Decision != cgp.DecisionRejected {
		t.Errorf("Decision = %s, want rejected", decision.Decision)
	}
}

func TestApprover_ProcessApproval_Expired(t *testing.T) {
	approver := New(nil)

	req := &ApprovalRequest{
		ID:         "approval-123",
		ProposalID: "proposal-456",
		ExpiresAt:  time.Now().Add(-time.Hour), // Already expired
	}

	resp := &ApprovalResponse{
		RequestID:     "approval-123",
		Status:        StatusApproved,
		Approver:      cgp.NewHumanActor("approver@example.com", "Approver"),
		Rationale:     "Approved after expiration",
		RationaleType: RationaleTypeReviewed,
		Timestamp:     time.Now(),
	}

	_, err := approver.ProcessApproval(context.Background(), req, resp)
	if err == nil {
		t.Error("ProcessApproval should fail for expired request")
	}
}

func TestApprover_ProcessApproval_UnauthorizedApprover(t *testing.T) {
	approver := New(&Config{
		RequireRationale:   true,
		MinRationaleLength: 10,
	})

	req := &ApprovalRequest{
		ID:                "approval-123",
		ProposalID:        "proposal-456",
		RequiredApprovers: []string{"human:authorized@example.com"},
		ExpiresAt:         time.Now().Add(time.Hour),
	}

	resp := &ApprovalResponse{
		RequestID:     "approval-123",
		Status:        StatusApproved,
		Approver:      cgp.NewHumanActor("unauthorized@example.com", "Unauthorized"),
		Rationale:     "Trying to approve without authorization",
		RationaleType: RationaleTypeReviewed,
		Timestamp:     time.Now(),
	}

	_, err := approver.ProcessApproval(context.Background(), req, resp)
	if err == nil {
		t.Error("ProcessApproval should fail for unauthorized approver")
	}
}

func TestApprovalRequest_IsExpired(t *testing.T) {
	// Expired
	expired := &ApprovalRequest{
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	if !expired.IsExpired() {
		t.Error("Request should be expired")
	}

	// Not expired
	valid := &ApprovalRequest{
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if valid.IsExpired() {
		t.Error("Request should not be expired")
	}
}

func TestApprovalRequest_NeedsHumanReview(t *testing.T) {
	// High risk needs review
	highRisk := &ApprovalRequest{
		RiskAssessment: &RiskSummary{OverallScore: 0.6},
	}
	if !highRisk.NeedsHumanReview() {
		t.Error("High risk should need review")
	}

	// Low risk doesn't need review
	lowRisk := &ApprovalRequest{
		RiskAssessment: &RiskSummary{OverallScore: 0.2},
	}
	if lowRisk.NeedsHumanReview() {
		t.Error("Low risk should not need review")
	}

	// Required approvers need review
	withApprovers := &ApprovalRequest{
		RequiredApprovers: []string{"human:approver@example.com"},
	}
	if !withApprovers.NeedsHumanReview() {
		t.Error("Required approvers should need review")
	}

	// Blocking policy needs review
	blockedByPolicy := &ApprovalRequest{
		PolicyEvaluations: []PolicyEvaluation{
			{Matched: true, Decision: "approval_required"},
		},
	}
	if !blockedByPolicy.NeedsHumanReview() {
		t.Error("Blocking policy should need review")
	}
}

func TestApprovalRequest_RiskLevel(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{0.0, "minimal"},
		{0.1, "minimal"},
		{0.2, "low"},
		{0.3, "low"},
		{0.4, "medium"},
		{0.5, "medium"},
		{0.7, "high"},
		{1.0, "high"},
	}

	for _, tt := range tests {
		req := &ApprovalRequest{
			RiskAssessment: &RiskSummary{OverallScore: tt.score},
		}
		got := req.RiskLevel()
		if got != tt.want {
			t.Errorf("RiskLevel(%.1f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestApprovalRequest_HasBreakingChanges(t *testing.T) {
	// No context
	noCtx := &ApprovalRequest{}
	if noCtx.HasBreakingChanges() {
		t.Error("No context should not have breaking changes")
	}

	// Empty breaking changes
	empty := &ApprovalRequest{
		Context: &ApprovalContext{
			BreakingChanges: []string{},
		},
	}
	if empty.HasBreakingChanges() {
		t.Error("Empty breaking changes should return false")
	}

	// Has breaking changes
	hasBreaking := &ApprovalRequest{
		Context: &ApprovalContext{
			BreakingChanges: []string{"Removed deprecated API"},
		},
	}
	if !hasBreaking.HasBreakingChanges() {
		t.Error("Should detect breaking changes")
	}
}

func TestSuggestedRationales(t *testing.T) {
	// Low risk
	lowRisk := &ApprovalRequest{
		RiskAssessment: &RiskSummary{OverallScore: 0.1},
	}
	suggestions := SuggestedRationales(lowRisk)
	if len(suggestions) == 0 {
		t.Error("Should have suggestions for low risk")
	}

	// Breaking changes
	breaking := &ApprovalRequest{
		RiskAssessment: &RiskSummary{OverallScore: 0.3},
		Context: &ApprovalContext{
			BreakingChanges: []string{"API change"},
		},
	}
	suggestions = SuggestedRationales(breaking)
	found := false
	for _, s := range suggestions {
		if contains(s, "Breaking") || contains(s, "breaking") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Should have breaking change rationale suggestion")
	}

	// Security changes
	security := &ApprovalRequest{
		RiskAssessment: &RiskSummary{OverallScore: 0.3},
		Context: &ApprovalContext{
			SecurityChanges: []string{"Fix XSS vulnerability"},
		},
	}
	suggestions = SuggestedRationales(security)
	found = false
	for _, s := range suggestions {
		if contains(s, "Security") || contains(s, "security") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Should have security rationale suggestion")
	}
}

func TestRejectionRationales(t *testing.T) {
	rationales := RejectionRationales()
	if len(rationales) == 0 {
		t.Error("Should have rejection rationales")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
