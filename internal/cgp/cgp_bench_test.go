package cgp

import (
	"fmt"
	"testing"
	"time"
)

// ============================================================================
// Core CGP Type Benchmarks
// ============================================================================

// BenchmarkProposal_Creation measures proposal creation overhead.
func BenchmarkProposal_Creation(b *testing.B) {
	b.ReportAllocs()

	actor := NewHumanActor("user@example.com", "Test User")
	scope := ProposalScope{
		Repository:  "org/repo",
		CommitRange: "abc123..def456",
	}
	intent := ProposalIntent{
		Summary:       "Adding new features",
		SuggestedBump: BumpTypeMinor,
		Confidence:    0.9,
	}

	b.Run("minimal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewProposal(actor, scope, intent)
		}
	})

	b.Run("with_context", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			p := NewProposal(actor, scope, intent)
			p.WithContext(&ProposalContext{
				AgentSession: "session-123",
			})
		}
	})

	b.Run("with_issues", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			p := NewProposal(actor, scope, intent)
			p.AddIssue("github", "123", "https://github.com/org/repo/issues/123")
			p.AddIssue("jira", "PROJ-456", "https://jira.example.com/PROJ-456")
		}
	})

	b.Run("with_metadata", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			p := NewProposal(actor, scope, intent)
			p.AddMetadata("source", "ci")
			p.AddMetadata("pipeline", "main")
			p.AddMetadata("build", 12345)
		}
	})
}

// BenchmarkProposal_Validation measures proposal validation overhead.
func BenchmarkProposal_Validation(b *testing.B) {
	b.ReportAllocs()

	actor := NewHumanActor("user@example.com", "Test User")
	scope := ProposalScope{
		Repository:  "org/repo",
		CommitRange: "abc123..def456",
	}
	intent := ProposalIntent{
		Summary:    "Bug fixes",
		Confidence: 0.8,
	}

	b.Run("valid_proposal", func(b *testing.B) {
		p := NewProposal(actor, scope, intent)
		for i := 0; i < b.N; i++ {
			_ = p.Validate()
		}
	})

	b.Run("with_breaking_changes", func(b *testing.B) {
		intentBreaking := ProposalIntent{
			Summary:         "Major refactor",
			SuggestedBump:   BumpTypeMajor,
			Confidence:      0.95,
			BreakingChanges: []string{"Removed deprecated API", "Changed config format"},
		}
		p := NewProposal(actor, scope, intentBreaking)
		for i := 0; i < b.N; i++ {
			_ = p.Validate()
		}
	})
}

// BenchmarkActor_Creation measures actor creation overhead.
func BenchmarkActor_Creation(b *testing.B) {
	b.ReportAllocs()

	b.Run("human_actor", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewHumanActor("user@example.com", "Test User")
		}
	})

	b.Run("ci_actor", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewCIActor("github-actions", "release", "12345")
		}
	})

	b.Run("agent_actor", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewAgentActor("claude-agent", "Claude Code Agent", "claude-3")
		}
	})
}

// BenchmarkActor_Validation measures actor validation overhead.
func BenchmarkActor_Validation(b *testing.B) {
	b.ReportAllocs()

	human := NewHumanActor("user@example.com", "Test User")
	ci := NewCIActor("github-actions", "release", "12345")
	agent := NewAgentActor("claude-agent", "Claude Code Agent", "claude-3")

	b.Run("human", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = human.Validate()
		}
	})

	b.Run("ci", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = ci.Validate()
		}
	})

	b.Run("agent", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = agent.Validate()
		}
	})
}

// ============================================================================
// Decision Benchmarks
// ============================================================================

// BenchmarkDecision_Creation measures decision creation overhead.
func BenchmarkDecision_Creation(b *testing.B) {
	b.ReportAllocs()

	b.Run("approved", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewDecision(fmt.Sprintf("proposal-%d", i), DecisionApproved)
		}
	})

	b.Run("with_risk_score", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			d := NewDecision(fmt.Sprintf("proposal-%d", i), DecisionApproved)
			d.WithRiskScore(0.25)
		}
	})

	b.Run("with_rationale", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			d := NewDecision(fmt.Sprintf("proposal-%d", i), DecisionApproved)
			d.AddRationale("Low risk change with comprehensive testing")
			d.AddRationale("All CI checks passed")
			d.AddRationale("No breaking changes detected")
		}
	})

	b.Run("with_risk_factors", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			d := NewDecision(fmt.Sprintf("proposal-%d", i), DecisionApprovalRequired)
			d.WithRiskScore(0.55)
			d.AddRiskFactor("scope", "Large change affecting multiple components", 0.6, SeverityMedium)
			d.AddRiskFactor("dependencies", "Updates critical dependency", 0.5, SeverityMedium)
			d.AddRiskFactor("security", "No security-related changes", 0.1, SeverityLow)
		}
	})

	b.Run("full_decision", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			d := NewDecision(fmt.Sprintf("proposal-%d", i), DecisionApprovalRequired)
			d.WithRiskScore(0.65)
			d.AddRationale("Medium-risk change requiring review")
			d.AddRationale("Multiple components affected")
			d.AddRiskFactor("scope", "Large blast radius", 0.7, SeverityHigh)
			d.AddRiskFactor("complexity", "High cyclomatic complexity", 0.6, SeverityMedium)
			d.AddCondition("review", "Requires senior engineer review")
			d.AddCondition("testing", "Integration tests must pass")
		}
	})
}

// ============================================================================
// Authorization Benchmarks
// ============================================================================

// BenchmarkAuthorization_Creation measures authorization creation overhead.
func BenchmarkAuthorization_Creation(b *testing.B) {
	b.ReportAllocs()

	actor := NewHumanActor("user@example.com", "Test User")

	b.Run("minimal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewAuthorization("decision-123", "proposal-456", actor, "1.2.0")
		}
	})

	b.Run("with_validity", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			auth := NewAuthorization("decision-123", "proposal-456", actor, "1.2.0")
			auth.WithValidity(48 * time.Hour)
		}
	})

	b.Run("with_release_notes", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			auth := NewAuthorization("decision-123", "proposal-456", actor, "1.2.0")
			auth.WithReleaseNotes("## What's Changed\n- Feature A\n- Bug fix B")
			auth.WithChangelog("v1.2.0 - 2025-01-25\n\n### Features\n- Added feature A")
		}
	})

	b.Run("full_authorization", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			auth := NewAuthorization("decision-123", "proposal-456", actor, "1.2.0")
			auth.WithValidity(24 * time.Hour)
			auth.WithReleaseNotes("## What's Changed\n- Feature A")
			auth.WithChangelog("v1.2.0 - 2025-01-25")
			auth.WithAllowedSteps(ExecutionStepTag, ExecutionStepChangelog, ExecutionStepNotify)
			auth.AddRestriction("No production deploy before Monday")
			auth.RecordApproval(actor, ApprovalActionApprove, "LGTM")
		}
	})
}

// BenchmarkAuthorization_Validation measures authorization validation overhead.
func BenchmarkAuthorization_Validation(b *testing.B) {
	b.ReportAllocs()

	actor := NewHumanActor("user@example.com", "Test User")
	auth := NewAuthorization("decision-123", "proposal-456", actor, "1.2.0")
	auth.WithValidity(24 * time.Hour)
	auth.WithReleaseNotes("## What's Changed\n- Feature A")
	auth.RecordApproval(actor, ApprovalActionApprove, "Approved after review")

	b.Run("validate", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = auth.Validate()
		}
	})

	b.Run("is_valid", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = auth.IsValid()
		}
	})

	b.Run("is_step_allowed", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = auth.IsStepAllowed(ExecutionStepTag)
		}
	})

	b.Run("has_approval_from", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = auth.HasApprovalFrom("human:user@example.com")
		}
	})

	b.Run("has_human_approval", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = auth.HasHumanApproval()
		}
	})
}

// ============================================================================
// Scale Benchmarks
// ============================================================================

// BenchmarkDecision_ManyRiskFactors measures performance with many risk factors.
func BenchmarkDecision_ManyRiskFactors(b *testing.B) {
	b.ReportAllocs()

	categories := []string{"scope", "security", "complexity", "dependencies", "testing", "history"}
	severities := []Severity{SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical}

	b.Run("10_factors", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			d := NewDecision("proposal-test", DecisionApprovalRequired)
			d.WithRiskScore(0.65)
			for j := 0; j < 10; j++ {
				d.AddRiskFactor(
					categories[j%len(categories)],
					fmt.Sprintf("Risk factor %d description", j),
					float64(j)/20.0+0.2,
					severities[j%len(severities)],
				)
			}
		}
	})

	b.Run("50_factors", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			d := NewDecision("proposal-test", DecisionApprovalRequired)
			d.WithRiskScore(0.75)
			for j := 0; j < 50; j++ {
				d.AddRiskFactor(
					categories[j%len(categories)],
					fmt.Sprintf("Risk factor %d description with more detail about the risk", j),
					float64(j)/100.0+0.2,
					severities[j%len(severities)],
				)
			}
		}
	})
}

// BenchmarkDecision_ManyConditions measures performance with many conditions.
func BenchmarkDecision_ManyConditions(b *testing.B) {
	b.ReportAllocs()

	b.Run("10_conditions", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			d := NewDecision("proposal-test", DecisionApprovalRequired)
			for j := 0; j < 10; j++ {
				d.AddCondition(
					fmt.Sprintf("condition_%d", j),
					fmt.Sprintf("Condition %d must be satisfied", j),
				)
			}
		}
	})

	b.Run("50_conditions", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			d := NewDecision("proposal-test", DecisionApprovalRequired)
			for j := 0; j < 50; j++ {
				d.AddCondition(
					fmt.Sprintf("condition_%d", j),
					fmt.Sprintf("Condition %d: detailed requirement for approval of this change", j),
				)
			}
		}
	})
}

// BenchmarkProposal_LargeScope measures performance with large scope data.
func BenchmarkProposal_LargeScope(b *testing.B) {
	b.ReportAllocs()

	actor := NewHumanActor("user@example.com", "Test User")
	intent := ProposalIntent{
		Summary:    "Large refactor",
		Confidence: 0.85,
	}

	b.Run("many_files", func(b *testing.B) {
		files := make([]string, 100)
		for i := 0; i < 100; i++ {
			files[i] = fmt.Sprintf("src/component-%d/file.go", i)
		}
		scope := ProposalScope{
			Repository:  "org/repo",
			CommitRange: "abc..def",
			Files:       files,
		}
		for i := 0; i < b.N; i++ {
			_ = NewProposal(actor, scope, intent)
		}
	})

	b.Run("many_commits", func(b *testing.B) {
		commits := make([]string, 50)
		for i := 0; i < 50; i++ {
			commits[i] = fmt.Sprintf("abc%04d", i)
		}
		scope := ProposalScope{
			Repository: "org/repo",
			Commits:    commits,
		}
		for i := 0; i < b.N; i++ {
			_ = NewProposal(actor, scope, intent)
		}
	})
}

// ============================================================================
// Concurrent Access Benchmarks
// ============================================================================

// BenchmarkDecision_ConcurrentCreation measures concurrent decision creation.
func BenchmarkDecision_ConcurrentCreation(b *testing.B) {
	b.ReportAllocs()

	b.Run("parallel_creation", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				d := NewDecision(fmt.Sprintf("proposal-%d", i), DecisionApproved)
				d.WithRiskScore(0.25)
				d.AddRationale("Approved after review")
				i++
			}
		})
	})
}

// BenchmarkProposal_ConcurrentCreation measures concurrent proposal creation.
func BenchmarkProposal_ConcurrentCreation(b *testing.B) {
	b.ReportAllocs()

	b.Run("parallel_creation", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				actor := NewHumanActor(
					fmt.Sprintf("user-%d@example.com", i%100),
					fmt.Sprintf("User %d", i%100),
				)
				scope := ProposalScope{
					Repository:  fmt.Sprintf("org/repo-%d", i%10),
					CommitRange: "abc..def",
				}
				intent := ProposalIntent{
					Summary:    "Bug fix",
					Confidence: 0.8,
				}
				_ = NewProposal(actor, scope, intent)
				i++
			}
		})
	})
}

// ============================================================================
// Target Validation Benchmark
// ============================================================================

// BenchmarkCGP_FullEvaluationOverhead validates the <200ms target.
// This measures all CGP infrastructure overhead without actual policy execution.
func BenchmarkCGP_FullEvaluationOverhead(b *testing.B) {
	b.ReportAllocs()

	b.Run("full_cgp_infrastructure", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			start := time.Now()

			// 1. Create actor
			actor := NewHumanActor("developer@example.com", "Developer")

			// 2. Create proposal with full data
			scope := ProposalScope{
				Repository:  "org/repository",
				Branch:      "main",
				CommitRange: "abc123..def456",
				Files:       []string{"api/handler.go", "core/service.go", "db/repository.go"},
			}
			intent := ProposalIntent{
				Summary:       "Adding new API endpoints and improving performance",
				SuggestedBump: BumpTypeMinor,
				Confidence:    0.9,
				Categories:    []string{"feature", "performance"},
			}
			proposal := NewProposal(actor, scope, intent)
			proposal.AddIssue("github", "123", "https://github.com/org/repo/issues/123")
			proposal.AddMetadata("environment", "staging")

			// 3. Validate proposal
			_ = proposal.Validate()

			// 4. Create authorization
			auth := NewAuthorization("decision-123", proposal.ID, actor, "1.0.0")
			auth.WithValidity(24 * time.Hour)
			auth.RecordApproval(actor, ApprovalActionApprove, "LGTM")
			_ = auth.IsValid()
			_ = auth.IsStepAllowed(ExecutionStepTag)

			// 5. Create decision with risk factors
			decision := NewDecision(proposal.ID, DecisionApprovalRequired)
			decision.WithRiskScore(0.45)
			decision.AddRationale("Moderate risk change requiring standard review")
			decision.AddRiskFactor("scope", "Multiple components affected", 0.5, SeverityMedium)
			decision.AddRiskFactor("complexity", "New API endpoints added", 0.4, SeverityMedium)
			decision.AddRiskFactor("testing", "Good test coverage", 0.2, SeverityLow)
			decision.AddCondition("review", "Requires team lead approval")
			decision.AddCondition("testing", "Integration tests must pass")

			// 6. Verify we're well under 200ms
			elapsed := time.Since(start)
			if elapsed > 200*time.Millisecond {
				b.Errorf("CGP infrastructure took %v, exceeds 200ms target", elapsed)
			}
		}
	})
}
