package policy

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
)

// ============================================================================
// Policy Engine Benchmarks
// ============================================================================

// BenchmarkEngine_Creation measures engine creation overhead.
func BenchmarkEngine_Creation(b *testing.B) {
	b.ReportAllocs()

	b.Run("empty_policies", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewEngine([]Policy{}, nil)
		}
	})

	b.Run("single_policy", func(b *testing.B) {
		policies := []Policy{*createTestPolicy("test-1", 1)}
		for i := 0; i < b.N; i++ {
			_ = NewEngine(policies, nil)
		}
	})

	b.Run("10_policies", func(b *testing.B) {
		policies := make([]Policy, 10)
		for i := 0; i < 10; i++ {
			policies[i] = *createTestPolicy(fmt.Sprintf("policy-%d", i), 3)
		}
		for i := 0; i < b.N; i++ {
			_ = NewEngine(policies, nil)
		}
	})

	b.Run("50_policies", func(b *testing.B) {
		policies := make([]Policy, 50)
		for i := 0; i < 50; i++ {
			policies[i] = *createTestPolicy(fmt.Sprintf("policy-%d", i), 5)
		}
		for i := 0; i < b.N; i++ {
			_ = NewEngine(policies, nil)
		}
	})
}

// BenchmarkEngine_Evaluate measures policy evaluation performance.
func BenchmarkEngine_Evaluate(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()

	createProposal := func() *cgp.ChangeProposal {
		actor := cgp.NewHumanActor("user@example.com", "Test User")
		scope := cgp.ProposalScope{
			Repository:  "org/repo",
			CommitRange: "abc..def",
		}
		intent := cgp.ProposalIntent{
			Summary:       "Bug fixes",
			SuggestedBump: cgp.BumpTypePatch,
			Confidence:    0.9,
		}
		return cgp.NewProposal(actor, scope, intent)
	}

	b.Run("single_rule_match", func(b *testing.B) {
		policy := NewPolicy("auto-approve-patches")
		policy.AddRule(*NewRule("patch-only", "Patch releases").
			WithPriority(100).
			AddCondition("intent.suggestedBump", OperatorEqual, "patch").
			AddAction(ActionSetDecision, map[string]any{"decision": "approved"}))

		engine := NewEngine([]Policy{*policy}, nil)
		proposal := createProposal()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.Evaluate(ctx, proposal, nil, 0.3)
		}
	})

	b.Run("multiple_rules_first_match", func(b *testing.B) {
		policy := NewPolicy("multi-rule")
		for i := 0; i < 10; i++ {
			policy.AddRule(*NewRule(fmt.Sprintf("rule-%d", i), fmt.Sprintf("Rule %d", i)).
				WithPriority(100-i).
				AddCondition("intent.suggestedBump", OperatorEqual, "patch").
				AddAction(ActionSetDecision, map[string]any{"decision": "approved"}))
		}

		engine := NewEngine([]Policy{*policy}, nil)
		proposal := createProposal()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.Evaluate(ctx, proposal, nil, 0.3)
		}
	})

	b.Run("multiple_rules_last_match", func(b *testing.B) {
		policy := NewPolicy("multi-rule-last")
		for i := 0; i < 9; i++ {
			policy.AddRule(*NewRule(fmt.Sprintf("rule-%d", i), fmt.Sprintf("Rule %d", i)).
				WithPriority(100-i).
				AddCondition("intent.suggestedBump", OperatorEqual, "major"). // Won't match
				AddAction(ActionSetDecision, map[string]any{"decision": "rejected"}))
		}
		policy.AddRule(*NewRule("rule-match", "Matching rule").
			WithPriority(10).
			AddCondition("intent.suggestedBump", OperatorEqual, "patch"). // Will match
			AddAction(ActionSetDecision, map[string]any{"decision": "approved"}))

		engine := NewEngine([]Policy{*policy}, nil)
		proposal := createProposal()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.Evaluate(ctx, proposal, nil, 0.3)
		}
	})

	b.Run("no_match", func(b *testing.B) {
		policy := NewPolicy("no-match")
		policy.AddRule(*NewRule("major-only", "Major releases").
			WithPriority(100).
			AddCondition("intent.suggestedBump", OperatorEqual, "major").
			AddAction(ActionBlock, map[string]any{"reason": "Not allowed"}))

		engine := NewEngine([]Policy{*policy}, nil)
		proposal := createProposal()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.Evaluate(ctx, proposal, nil, 0.3)
		}
	})

	b.Run("complex_conditions", func(b *testing.B) {
		policy := NewPolicy("complex")
		policy.AddRule(*NewRule("complex-rule", "Complex rule").
			WithPriority(100).
			AddCondition("intent.suggestedBump", OperatorIn, []string{"patch", "minor"}).
			AddCondition("risk.score", OperatorLessThan, 0.5).
			AddCondition("actor.kind", OperatorEqual, "human").
			AddAction(ActionSetDecision, map[string]any{"decision": "approved"}))

		engine := NewEngine([]Policy{*policy}, nil)
		proposal := createProposal()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.Evaluate(ctx, proposal, nil, 0.3)
		}
	})
}

// BenchmarkEngine_EvaluateScale measures evaluation at scale.
func BenchmarkEngine_EvaluateScale(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()

	createProposal := func() *cgp.ChangeProposal {
		actor := cgp.NewHumanActor("user@example.com", "Test User")
		scope := cgp.ProposalScope{
			Repository:  "org/repo",
			CommitRange: "abc..def",
		}
		intent := cgp.ProposalIntent{
			Summary:       "New features",
			SuggestedBump: cgp.BumpTypeMinor,
			Confidence:    0.85,
			Categories:    []string{"feature", "api"},
		}
		return cgp.NewProposal(actor, scope, intent)
	}

	b.Run("20_policies_100_rules_total", func(b *testing.B) {
		policies := make([]Policy, 20)
		for i := 0; i < 20; i++ {
			policies[i] = *createTestPolicy(fmt.Sprintf("policy-%d", i), 5)
		}
		engine := NewEngine(policies, nil)
		proposal := createProposal()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.Evaluate(ctx, proposal, nil, 0.5)
		}
	})

	b.Run("50_policies_250_rules_total", func(b *testing.B) {
		policies := make([]Policy, 50)
		for i := 0; i < 50; i++ {
			policies[i] = *createTestPolicy(fmt.Sprintf("policy-%d", i), 5)
		}
		engine := NewEngine(policies, nil)
		proposal := createProposal()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.Evaluate(ctx, proposal, nil, 0.5)
		}
	})
}

// BenchmarkEngine_WithTimeContext measures time context evaluation.
func BenchmarkEngine_WithTimeContext(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	actor := cgp.NewHumanActor("user@example.com", "Test User")
	proposal := cgp.NewProposal(
		actor,
		cgp.ProposalScope{Repository: "org/repo", CommitRange: "abc..def"},
		cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
	)

	b.Run("with_business_hours", func(b *testing.B) {
		policy := NewPolicy("business-hours")
		policy.AddRule(*NewRule("time-check", "Time-based rule").
			WithPriority(100).
			AddAction(ActionSetDecision, map[string]any{"decision": "approved"}))

		engine := NewEngine([]Policy{*policy}, nil).
			SetBusinessHours(BusinessHoursConfig{
				StartHour:     9,
				EndHour:       17,
				Timezone:      "UTC",
				AllowWeekends: false,
			})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.Evaluate(ctx, proposal, nil, 0.3)
		}
	})

	b.Run("with_freeze_periods", func(b *testing.B) {
		policy := NewPolicy("freeze-check")
		policy.AddRule(*NewRule("default", "Default rule").
			WithPriority(100).
			AddAction(ActionSetDecision, map[string]any{"decision": "approved"}))

		engine := NewEngine([]Policy{*policy}, nil).
			AddFreezePeriod(FreezePeriod{
				Name:   "Holiday freeze",
				Start:  time.Now().Add(-time.Hour),
				End:    time.Now().Add(24 * time.Hour),
				Reason: "Holiday season",
			})
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = engine.Evaluate(ctx, proposal, nil, 0.3)
		}
	})
}

// BenchmarkEngine_ConcurrentEvaluation measures concurrent evaluation.
func BenchmarkEngine_ConcurrentEvaluation(b *testing.B) {
	b.ReportAllocs()

	policies := make([]Policy, 10)
	for i := 0; i < 10; i++ {
		policies[i] = *createTestPolicy(fmt.Sprintf("policy-%d", i), 5)
	}
	engine := NewEngine(policies, nil)

	b.Run("parallel_evaluation", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			ctx := context.Background()
			i := 0
			for pb.Next() {
				actor := cgp.NewHumanActor(
					fmt.Sprintf("user-%d@example.com", i%100),
					fmt.Sprintf("User %d", i%100),
				)
				proposal := cgp.NewProposal(
					actor,
					cgp.ProposalScope{Repository: "org/repo", CommitRange: "abc..def"},
					cgp.ProposalIntent{Summary: "Test", Confidence: 0.9},
				)
				_, _ = engine.Evaluate(ctx, proposal, nil, 0.3)
				i++
			}
		})
	})
}

// ============================================================================
// Target Validation Benchmark
// ============================================================================

// BenchmarkEngine_TargetValidation validates the <200ms target for policy evaluation.
func BenchmarkEngine_TargetValidation(b *testing.B) {
	b.ReportAllocs()

	// Create realistic policy set
	policies := make([]Policy, 20)
	for i := 0; i < 20; i++ {
		policies[i] = *createTestPolicy(fmt.Sprintf("policy-%d", i), 10)
	}

	engine := NewEngine(policies, nil).
		SetBusinessHours(BusinessHoursConfig{
			StartHour:     9,
			EndHour:       17,
			Timezone:      "UTC",
			AllowWeekends: false,
		}).
		AddFreezePeriod(FreezePeriod{
			Name:   "Test freeze",
			Start:  time.Now().Add(24 * time.Hour),
			End:    time.Now().Add(48 * time.Hour),
			Reason: "Testing",
		})

	b.Run("full_evaluation_under_200ms", func(b *testing.B) {
		ctx := context.Background()
		for i := 0; i < b.N; i++ {
			start := time.Now()

			actor := cgp.NewHumanActor("developer@example.com", "Developer")
			proposal := cgp.NewProposal(
				actor,
				cgp.ProposalScope{
					Repository:  "org/repository",
					Branch:      "main",
					CommitRange: "abc123..def456",
					Files:       []string{"api/handler.go", "core/service.go"},
				},
				cgp.ProposalIntent{
					Summary:       "Adding new features",
					SuggestedBump: cgp.BumpTypeMinor,
					Confidence:    0.9,
					Categories:    []string{"feature"},
				},
			)

			_, _ = engine.Evaluate(ctx, proposal, nil, 0.5)

			elapsed := time.Since(start)
			if elapsed > 200*time.Millisecond {
				b.Errorf("Policy evaluation took %v, exceeds 200ms target", elapsed)
			}
		}
	})
}

// ============================================================================
// Helper Functions
// ============================================================================

// createTestPolicy creates a test policy with the specified number of rules.
func createTestPolicy(name string, numRules int) *Policy {
	policy := NewPolicy(name)

	operators := []string{OperatorEqual, OperatorNotEqual, OperatorLessThan, OperatorLessOrEqual, OperatorGreaterThan, OperatorGreaterOrEqual, OperatorIn}
	decisions := []string{"approved", "rejected", "require_approval"}

	for i := 0; i < numRules; i++ {
		rule := NewRule(fmt.Sprintf("%s-rule-%d", name, i), fmt.Sprintf("Rule %d", i)).
			WithPriority(100-i).
			AddCondition("intent.suggestedBump", operators[i%len(operators)], "major") // Won't match most

		if i%2 == 0 {
			rule.AddCondition("risk.score", OperatorGreaterThan, 0.9) // High threshold
		}

		rule.AddAction(ActionSetDecision, map[string]any{"decision": decisions[i%len(decisions)]})
		policy.AddRule(*rule)
	}

	return policy
}
