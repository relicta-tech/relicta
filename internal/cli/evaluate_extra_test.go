package cli

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/application/governance"
	"github.com/relicta-tech/relicta/internal/cgp"
)

func TestPrintEvaluateNextStep(t *testing.T) {
	tests := []struct {
		name     string
		decision cgp.DecisionType
	}{
		{"approved decision", cgp.DecisionApproved},
		{"approval required decision", cgp.DecisionApprovalRequired},
		{"deferred decision", cgp.DecisionDeferred},
		{"rejected decision", cgp.DecisionRejected},
		{"unknown decision", cgp.DecisionType("unknown")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &governance.EvaluateReleaseOutput{
				Decision: tt.decision,
			}
			// Should not panic for any decision type
			printEvaluateNextStep(result)
		})
	}
}
