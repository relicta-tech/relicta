package main

import (
	"testing"

	"github.com/relicta-tech/relicta/v5/internal/cgp/audit"
)

func TestVerifyReleaseEvidenceBindsDecisionAndHumanApproval(t *testing.T) {
	tests := []struct {
		name         string
		decision     string
		decisionHash string
		planHash     string
		reviewer     string
		autoApprove  bool
		wantErr      bool
	}{
		{name: "matching protected approval", decision: "approved", decisionHash: "plan-123", planHash: "plan-123", reviewer: "alice"},
		{name: "unpermitted decision", decision: "deferred", decisionHash: "plan-123", planHash: "plan-123", reviewer: "alice", wantErr: true},
		{name: "decision belongs to another plan", decision: "approved", decisionHash: "old-plan", planHash: "plan-123", reviewer: "alice", wantErr: true},
		{name: "approval belongs to another plan", decision: "approved", decisionHash: "plan-123", planHash: "other-plan", reviewer: "alice", wantErr: true},
		{name: "wrong reviewer", decision: "approved", decisionHash: "plan-123", planHash: "plan-123", reviewer: "bob", wantErr: true},
		{name: "automatic approval", decision: "approved", decisionHash: "plan-123", planHash: "plan-123", reviewer: "alice", autoApprove: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := audit.NewChain()
			decision := audit.NewEntry("release-1:decision", audit.EventDecisionMade).
				WithProposal("release-1").
				WithDetail("decisionType", tt.decision).
				WithDetail("planHash", tt.decisionHash).
				Build()
			if err := chain.Append(decision); err != nil {
				t.Fatalf("append decision: %v", err)
			}
			approval := audit.NewEntry("release-1:approval", audit.EventApprovalGranted).
				WithProposal("release-1").
				WithActor("github:alice", "human").
				WithDetail("planHash", tt.planHash).
				WithDetail("autoApproved", tt.autoApprove).
				Build()
			if err := chain.Append(approval); err != nil {
				t.Fatalf("append approval: %v", err)
			}

			_, err := verifyReleaseEvidence(chain, "release-1", map[string]struct{}{
				"approved": {}, "approval_required": {},
			}, "plan-123", tt.reviewer)
			if (err != nil) != tt.wantErr {
				t.Fatalf("verifyReleaseEvidence() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}
