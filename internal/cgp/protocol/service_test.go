package protocol

import (
	"context"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp/evaluator"
	cgpsdk "github.com/relicta-tech/relicta/v4/pkg/cgp"
)

func validProposal() *cgpsdk.ChangeProposal {
	return &cgpsdk.ChangeProposal{
		CGPVersion: cgpsdk.ProtocolVersion,
		Type:       cgpsdk.TypeChangeProposal,
		ID:         "prop_test_001",
		Timestamp:  time.Now().UTC(),
		Actor:      cgpsdk.Actor{Kind: "agent", ID: "agent:claude", Name: "Claude"},
		Scope: cgpsdk.Scope{
			Repository:  "relicta-tech/relicta",
			CommitRange: "v1.0.0..HEAD",
		},
		Intent: cgpsdk.Intent{
			Summary:    "Add user authentication feature",
			Confidence: 0.85,
			Categories: []string{"feature"},
		},
	}
}

func TestEvaluateProposal(t *testing.T) {
	eval := evaluator.New()
	svc := NewService(eval)

	ctx := context.Background()
	proposal := validProposal()

	decision, err := svc.EvaluateProposal(ctx, proposal)
	if err != nil {
		t.Fatalf("EvaluateProposal failed: %v", err)
	}

	if decision == nil {
		t.Fatal("expected non-nil decision")
	}

	if decision.CGPVersion != cgpsdk.ProtocolVersion {
		t.Errorf("CGPVersion = %q, want %q", decision.CGPVersion, cgpsdk.ProtocolVersion)
	}

	if decision.Type != cgpsdk.TypeGovernanceDecision {
		t.Errorf("Type = %q, want %q", decision.Type, cgpsdk.TypeGovernanceDecision)
	}

	if decision.ProposalID != proposal.ID {
		t.Errorf("ProposalID = %q, want %q", decision.ProposalID, proposal.ID)
	}

	validDecisions := map[string]bool{
		"approved": true, "denied": true, "approval_required": true, "deferred": true,
	}
	if !validDecisions[decision.Decision] {
		t.Errorf("unexpected decision value: %q", decision.Decision)
	}

	if decision.RiskScore < 0 || decision.RiskScore > 1 {
		t.Errorf("RiskScore %f out of range [0,1]", decision.RiskScore)
	}
}

func TestEvaluateProposal_InvalidProposal(t *testing.T) {
	eval := evaluator.New()
	svc := NewService(eval)

	ctx := context.Background()

	// Missing required fields
	invalid := &cgpsdk.ChangeProposal{
		Type: cgpsdk.TypeChangeProposal,
	}

	_, err := svc.EvaluateProposal(ctx, invalid)
	if err == nil {
		t.Fatal("expected error for invalid proposal")
	}
}

func TestEvaluateProposal_NilProposal(t *testing.T) {
	eval := evaluator.New()
	svc := NewService(eval)

	_, err := svc.EvaluateProposal(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil proposal")
	}
}

func TestGetStatus_ProposedState(t *testing.T) {
	eval := evaluator.New()
	svc := NewService(eval)

	ctx := context.Background()
	proposal := validProposal()

	// Store proposal directly through the service evaluation
	_, err := svc.EvaluateProposal(ctx, proposal)
	if err != nil {
		t.Fatalf("EvaluateProposal failed: %v", err)
	}

	status, err := svc.GetStatus(ctx, proposal.ID)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status.ProposalID != proposal.ID {
		t.Errorf("ProposalID = %q, want %q", status.ProposalID, proposal.ID)
	}

	// After evaluation, the state should be "decided" since we have both proposal and decision.
	if status.State != "decided" {
		t.Errorf("State = %q, want %q", status.State, "decided")
	}

	if status.Proposal == nil {
		t.Error("expected non-nil Proposal in status")
	}

	if status.Decision == nil {
		t.Error("expected non-nil Decision in status after evaluation")
	}
}

func TestGetStatus_NotFound(t *testing.T) {
	eval := evaluator.New()
	svc := NewService(eval)

	_, err := svc.GetStatus(context.Background(), "nonexistent_id")
	if err == nil {
		t.Fatal("expected error for nonexistent proposal")
	}
}

func TestRecordAuthorization(t *testing.T) {
	eval := evaluator.New()
	svc := NewService(eval)

	ctx := context.Background()
	proposal := validProposal()

	// Evaluate first to get a proposal and decision stored.
	decision, err := svc.EvaluateProposal(ctx, proposal)
	if err != nil {
		t.Fatalf("EvaluateProposal failed: %v", err)
	}

	now := time.Now().UTC()
	auth := &cgpsdk.ExecutionAuthorization{
		CGPVersion: cgpsdk.ProtocolVersion,
		Type:       cgpsdk.TypeExecutionAuthorization,
		ID:         "auth_test_001",
		ProposalID: proposal.ID,
		DecisionID: decision.ID,
		Timestamp:  now,
		ApprovedBy: cgpsdk.Actor{Kind: "human", ID: "human:alice@example.com"},
		Version:    "1.1.0",
		ValidUntil: now.Add(24 * time.Hour),
		Scope:      []string{"tag", "publish"},
	}

	// Skip if the decision was "denied" -- the authorization should fail.
	if decision.Decision == "denied" {
		err = svc.RecordAuthorization(ctx, auth)
		if err == nil {
			t.Fatal("expected error when authorizing a denied proposal")
		}
		return
	}

	err = svc.RecordAuthorization(ctx, auth)
	if err != nil {
		t.Fatalf("RecordAuthorization failed: %v", err)
	}

	// Verify the status is now "authorized"
	status, err := svc.GetStatus(ctx, proposal.ID)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status.State != "authorized" {
		t.Errorf("State = %q, want %q", status.State, "authorized")
	}

	if status.Authorization == nil {
		t.Error("expected non-nil Authorization in status")
	}
}

func TestRecordAuthorization_InvalidAuth(t *testing.T) {
	eval := evaluator.New()
	svc := NewService(eval)

	// Missing required fields
	auth := &cgpsdk.ExecutionAuthorization{
		Type: cgpsdk.TypeExecutionAuthorization,
	}

	err := svc.RecordAuthorization(context.Background(), auth)
	if err == nil {
		t.Fatal("expected error for invalid authorization")
	}
}

func TestRecordAuthorization_NoProposal(t *testing.T) {
	eval := evaluator.New()
	svc := NewService(eval)

	now := time.Now().UTC()
	auth := &cgpsdk.ExecutionAuthorization{
		CGPVersion: cgpsdk.ProtocolVersion,
		Type:       cgpsdk.TypeExecutionAuthorization,
		ID:         "auth_test_noprop",
		ProposalID: "nonexistent_proposal",
		DecisionID: "dec_x",
		Timestamp:  now,
		ApprovedBy: cgpsdk.Actor{Kind: "human", ID: "human:bob@example.com"},
		Version:    "1.0.0",
		ValidUntil: now.Add(time.Hour),
	}

	err := svc.RecordAuthorization(context.Background(), auth)
	if err == nil {
		t.Fatal("expected error when proposal does not exist")
	}
}

func TestConversions(t *testing.T) {
	// Verify toInternalProposal converts fields correctly.
	sdkProposal := validProposal()
	internal := toInternalProposal(sdkProposal)

	if string(internal.Actor.Kind) != sdkProposal.Actor.Kind {
		t.Errorf("Actor.Kind = %q, want %q", internal.Actor.Kind, sdkProposal.Actor.Kind)
	}
	if internal.Actor.ID != sdkProposal.Actor.ID {
		t.Errorf("Actor.ID = %q, want %q", internal.Actor.ID, sdkProposal.Actor.ID)
	}
	if internal.Scope.Repository != sdkProposal.Scope.Repository {
		t.Errorf("Scope.Repository = %q, want %q", internal.Scope.Repository, sdkProposal.Scope.Repository)
	}
	if internal.Intent.Summary != sdkProposal.Intent.Summary {
		t.Errorf("Intent.Summary = %q, want %q", internal.Intent.Summary, sdkProposal.Intent.Summary)
	}
}
