package library

import (
	"github.com/relicta-tech/relicta/internal/cgp/policy"
)

// registerStabilityTemplates adds stability-focused policy templates.
func registerStabilityTemplates(r *Registry) {
	mustRegister(r, &PolicyTemplate{
		ID:          "stability-production",
		Name:        "Production Protection",
		Description: "Protects production branches with additional review requirements and time-based controls.",
		Category:    CategoryStability,
		Tags:        []string{"stability", "production", "protection"},
		Build:       buildProductionProtectionPolicy,
	})

	mustRegister(r, &PolicyTemplate{
		ID:          "stability-gradual",
		Name:        "Gradual Rollout",
		Description: "Encourages incremental releases by requiring review for large changes.",
		Category:    CategoryStability,
		Tags:        []string{"stability", "gradual", "incremental"},
		Build:       buildGradualRolloutPolicy,
	})

	mustRegister(r, &PolicyTemplate{
		ID:          "stability-freeze",
		Name:        "Release Freeze",
		Description: "Implements release freezes during critical periods or holidays.",
		Category:    CategoryStability,
		Tags:        []string{"stability", "freeze", "holidays"},
		Build:       buildReleaseFreezePolicy,
	})

	mustRegister(r, &PolicyTemplate{
		ID:          "stability-confidence",
		Name:        "Confidence Threshold",
		Description: "Requires higher confidence for automated releases.",
		Category:    CategoryStability,
		Tags:        []string{"stability", "confidence", "automation"},
		Build:       buildConfidenceThresholdPolicy,
	})
}

func buildProductionProtectionPolicy(opts TemplateOptions) *policy.Policy {
	name := opts.PolicyName
	if name == "" {
		name = "production-protection"
	}

	p := policy.NewPolicy(name)
	p.Description = "Protection rules for production branches"

	// Rule: Production branch requires review
	branches := opts.ProductionBranches
	if len(branches) == 0 {
		branches = []string{"main", "master", "production"}
	}

	for i, branch := range branches {
		p.AddRule(*policy.NewRule(
			"prod-branch-"+branch,
			"Production Branch: "+branch,
		).
			WithPriority(900-i).
			WithDescription("Releases to "+branch+" require approval").
			AddCondition("scope.branch", policy.OperatorEqual, branch).
			AddAction(policy.ActionRequireApproval, map[string]any{
				"count":       1,
				"description": "Production branch release requires approval",
			}))
	}

	// Rule: High-risk to production blocked
	p.AddRule(*policy.NewRule("prod-high-risk-block", "Block High Risk to Production").
		WithPriority(1000).
		WithDescription("Blocks high-risk changes to production branches").
		AddCondition("scope.branch", policy.OperatorIn, branches).
		AddCondition("risk.score", policy.OperatorGreaterThan, 0.8).
		AddAction(policy.ActionBlock, map[string]any{
			"reason": "High-risk changes to production require manual intervention",
		}))

	// Rule: Large changes to production require more approvers
	maxFiles := opts.MaxFilesWithoutReview
	if maxFiles == 0 {
		maxFiles = 10
	}
	p.AddRule(*policy.NewRule("prod-large-change", "Large Production Change").
		WithPriority(850).
		WithDescription("Large changes to production require additional review").
		AddCondition("scope.branch", policy.OperatorIn, branches).
		AddCondition("scope.fileCount", policy.OperatorGreaterThan, float64(maxFiles)).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       2,
			"description": "Large production change requires additional review",
		}))

	p.Defaults = policy.Defaults{
		Decision:          policy.DecisionApprove,
		RequiredApprovers: 0,
	}

	return p
}

func buildGradualRolloutPolicy(opts TemplateOptions) *policy.Policy {
	name := opts.PolicyName
	if name == "" {
		name = "gradual-rollout"
	}

	maxFiles := opts.MaxFilesWithoutReview
	if maxFiles == 0 {
		maxFiles = 10
	}
	maxLines := opts.MaxLinesWithoutReview
	if maxLines == 0 {
		maxLines = 500
	}

	p := policy.NewPolicy(name)
	p.Description = "Encourages smaller, incremental releases"

	// Rule: Many files changed
	p.AddRule(*policy.NewRule("grad-many-files", "Many Files Changed").
		WithPriority(800).
		WithDescription("Large file count changes require review").
		AddCondition("blastRadius.filesChanged", policy.OperatorGreaterThan, float64(maxFiles)).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Consider splitting into smaller releases",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Large change scope - consider incremental releases for easier rollback",
		}))

	// Rule: Many lines changed
	p.AddRule(*policy.NewRule("grad-many-lines", "Many Lines Changed").
		WithPriority(750).
		WithDescription("Large line count changes require review").
		AddCondition("blastRadius.linesChanged", policy.OperatorGreaterThan, float64(maxLines)).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Large change requires additional review",
		}))

	// Rule: Too many features at once
	p.AddRule(*policy.NewRule("grad-many-features", "Many Features").
		WithPriority(700).
		WithDescription("Many features in one release require review").
		AddCondition("change.features", policy.OperatorGreaterThan, 5).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Many features - consider separate releases",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Multiple features bundled - harder to identify issues if problems arise",
		}))

	// Rule: Mixed breaking and features
	p.AddRule(*policy.NewRule("grad-mixed-breaking", "Mixed Breaking and Features").
		WithPriority(650).
		WithDescription("Breaking changes with features require review").
		AddCondition("change.breaking", policy.OperatorGreaterThan, 0).
		AddCondition("change.features", policy.OperatorGreaterThan, 0).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Breaking changes should be separate from features",
		}))

	p.Defaults = policy.Defaults{
		Decision:          policy.DecisionApprove,
		RequiredApprovers: 0,
	}

	return p
}

func buildReleaseFreezePolicy(opts TemplateOptions) *policy.Policy {
	name := opts.PolicyName
	if name == "" {
		name = "release-freeze"
	}

	p := policy.NewPolicy(name)
	p.Description = "Implements release freezes for critical periods"

	// Rule: Block during freeze periods
	if opts.FreezeDuringHolidays {
		p.AddRule(*policy.NewRule("freeze-holiday", "Holiday Freeze").
			WithPriority(1000).
			WithDescription("Blocks releases during holiday freeze periods").
			AddCondition("time.inFreeze", policy.OperatorEqual, true).
			AddAction(policy.ActionBlock, map[string]any{
				"reason": "Releases blocked during freeze period",
			}))
	}

	// Rule: Restrict to business hours
	if opts.RequireBusinessHours {
		p.AddRule(*policy.NewRule("freeze-hours", "Business Hours Only").
			WithPriority(950).
			WithDescription("Restricts releases to business hours").
			AddCondition("time.isBusinessHours", policy.OperatorEqual, false).
			AddAction(policy.ActionRequireApproval, map[string]any{
				"count":       2,
				"description": "Out-of-hours release requires additional approval",
			}).
			AddAction(policy.ActionAddRationale, map[string]any{
				"message": "Release requested outside business hours - ensure on-call coverage",
			}))
	}

	// Rule: Weekend releases require approval
	p.AddRule(*policy.NewRule("freeze-weekend", "Weekend Release").
		WithPriority(900).
		WithDescription("Weekend releases require approval").
		AddCondition("time.isWeekend", policy.OperatorEqual, true).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Weekend release requires approval",
		}))

	// Rule: End of day releases get flagged
	p.AddRule(*policy.NewRule("freeze-eod", "End of Day Release").
		WithPriority(800).
		WithDescription("End of day releases should be reviewed").
		AddCondition("time.hour", policy.OperatorGreaterThan, 16).
		AddCondition("time.isWeekday", policy.OperatorEqual, true).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Late day release - consider deferring to morning for better support coverage",
		}))

	p.Defaults = policy.Defaults{
		Decision:          policy.DecisionApprove,
		RequiredApprovers: 0,
	}

	return p
}

func buildConfidenceThresholdPolicy(opts TemplateOptions) *policy.Policy {
	name := opts.PolicyName
	if name == "" {
		name = "confidence-threshold"
	}

	p := policy.NewPolicy(name)
	p.Description = "Requires confidence thresholds for automated releases"

	// Rule: Low confidence requires review
	p.AddRule(*policy.NewRule("conf-low", "Low Confidence Review").
		WithPriority(800).
		WithDescription("Low confidence changes require review").
		AddCondition("intent.confidence", policy.OperatorLessThan, 0.7).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Low confidence change requires human review",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Analysis confidence below threshold - human review recommended",
		}))

	// Rule: Very low confidence blocked
	p.AddRule(*policy.NewRule("conf-very-low", "Very Low Confidence Block").
		WithPriority(900).
		WithDescription("Very low confidence changes require explicit approval").
		AddCondition("intent.confidence", policy.OperatorLessThan, 0.4).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       2,
			"description": "Very low confidence requires additional review",
		}))

	// Rule: Bot with low confidence
	p.AddRule(*policy.NewRule("conf-bot-low", "Bot Low Confidence").
		WithPriority(850).
		WithDescription("Automated changes with low confidence require review").
		AddCondition("actor.kind", policy.OperatorEqual, "bot").
		AddCondition("intent.confidence", policy.OperatorLessThan, 0.8).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Bot change with lower confidence requires human check",
		}))

	p.Defaults = policy.Defaults{
		Decision:          policy.DecisionApprove,
		RequiredApprovers: 0,
	}

	return p
}
