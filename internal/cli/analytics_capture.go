// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"

	"github.com/relicta-tech/relicta/v4/internal/analytics"
	"github.com/relicta-tech/relicta/v4/internal/application/governance"
)

// captureGovernanceAnalytics records the risk-evaluation and policy-decision
// events from a governance result into the analytics store. Previously the
// analytics pipeline had no producers, so the dashboard/CLI trend views
// served empty data; this is the live capture point. It no-ops when the
// analytics service is unavailable and never blocks the release flow.
func captureGovernanceAnalytics(ctx context.Context, app cliApp, releaseID string, gov *governance.EvaluateReleaseOutput) {
	if app == nil || gov == nil {
		return
	}
	svc := app.Analytics()
	if svc == nil {
		return
	}

	_ = svc.Capture(ctx, analytics.EventRiskEvaluation, releaseID, analytics.RiskEvaluationPayload{
		RiskScore: gov.RiskScore,
		RiskLevel: riskLevelForScore(gov.RiskScore),
		Factors:   gov.Rationale,
	})

	_ = svc.Capture(ctx, analytics.EventPolicyDecision, releaseID, analytics.PolicyDecisionPayload{
		Decision:     string(gov.Decision),
		RiskScore:    gov.RiskScore,
		AutoApproved: gov.CanAutoApprove,
	})
}

// captureApprovalOutcome records an approval/rejection for actor and
// bottleneck analytics.
func captureApprovalOutcome(ctx context.Context, app cliApp, releaseID, outcome string, durationMs int64) {
	if app == nil {
		return
	}
	svc := app.Analytics()
	if svc == nil {
		return
	}
	actor := createCGPActor()
	_ = svc.Capture(ctx, analytics.EventApprovalOutcome, releaseID, analytics.ApprovalOutcomePayload{
		Outcome:    outcome,
		ActorID:    actor.ID,
		ActorKind:  actor.Kind.String(),
		DurationMs: durationMs,
	})
}

// captureReleaseDuration records the wall-clock duration and success of a
// publish for DORA-style reporting.
func captureReleaseDuration(ctx context.Context, app cliApp, releaseID, version string, durationMs int64, success bool) {
	if app == nil {
		return
	}
	svc := app.Analytics()
	if svc == nil {
		return
	}
	_ = svc.Capture(ctx, analytics.EventReleaseDuration, releaseID, analytics.ReleaseDurationPayload{
		DurationMs: durationMs,
		Success:    success,
		Version:    version,
	})
}

// riskLevelForScore maps a numeric risk score to a coarse level label.
func riskLevelForScore(score float64) string {
	switch {
	case score >= 0.8:
		return "critical"
	case score >= 0.6:
		return "high"
	case score >= 0.4:
		return "medium"
	case score > 0:
		return "low"
	default:
		return "none"
	}
}
