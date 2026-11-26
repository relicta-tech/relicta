package cli

import (
	"strings"
	"testing"

	"github.com/relicta-tech/relicta/v5/internal/analytics"
)

func TestAnalyticsReportsRenderEmptyAndPopulatedViews(t *testing.T) {
	out := captureImportReportOutput(t, func() {
		printRiskTrends(nil)
		printRiskTrends([]analytics.RiskTrendPoint{{Bucket: "2026-W48", AvgRiskScore: 0.42, MinRiskScore: 0.2, MaxRiskScore: 0.8, Count: 4}})
		printDecisionDistribution(nil)
		printDecisionDistribution([]analytics.DecisionDistribution{{Bucket: "2026-W48", Approve: 4, Deny: 1, RequireReview: 2, Total: 7}})
		printTeamMetrics(nil)
		printTeamMetrics([]analytics.TeamMetrics{{ActorID: "agent:release-bot", ApprovalCount: 4, RejectionCount: 1, ReleaseCount: 3, SuccessRate: 0.75}})
	})
	for _, want := range []string{
		"No risk evaluations captured yet", "2026-W48", "0.42", "No policy decisions captured yet",
		"APPROVE", "No approval/release activity captured yet", "agent:release-bot", "75%",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("analytics output omitted %q:\n%s", want, out)
		}
	}
}
