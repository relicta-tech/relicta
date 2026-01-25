package library

import (
	"github.com/relicta-tech/relicta/internal/cgp/policy"
)

// registerSpeedTemplates adds speed-focused policy templates for fast releases.
func registerSpeedTemplates(r *Registry) {
	mustRegister(r, &PolicyTemplate{
		ID:          "speed-auto-approve",
		Name:        "Auto-Approve Low Risk",
		Description: "Automatically approves low-risk changes to enable fast, continuous releases.",
		Category:    CategorySpeed,
		Tags:        []string{"speed", "auto-approve", "low-risk"},
		Build:       buildAutoApproveLowRiskPolicy,
	})

	mustRegister(r, &PolicyTemplate{
		ID:          "speed-trusted-actors",
		Name:        "Trusted Actors",
		Description: "Allows trusted actors and team leads to skip reviews for routine changes.",
		Category:    CategorySpeed,
		Tags:        []string{"speed", "trusted", "bypass"},
		Build:       buildTrustedActorsPolicy,
	})

	mustRegister(r, &PolicyTemplate{
		ID:          "speed-hotfix",
		Name:        "Hotfix Fast Track",
		Description: "Expedites hotfixes and critical patches with minimal friction.",
		Category:    CategorySpeed,
		Tags:        []string{"speed", "hotfix", "patch"},
		Build:       buildHotfixFastTrackPolicy,
	})

	mustRegister(r, &PolicyTemplate{
		ID:          "speed-ci-friendly",
		Name:        "CI-Friendly",
		Description: "Optimized for CI/CD pipelines with automated approvals for safe changes.",
		Category:    CategorySpeed,
		Tags:        []string{"speed", "ci", "automation"},
		Build:       buildCIFriendlyPolicy,
	})
}

func buildAutoApproveLowRiskPolicy(opts TemplateOptions) *policy.Policy {
	name := opts.PolicyName
	if name == "" {
		name = "auto-approve-low-risk"
	}

	threshold := opts.RiskThreshold
	if threshold == 0 {
		threshold = 0.3 // Lower threshold for auto-approve
	}

	p := policy.NewPolicy(name)
	p.Description = "Automatically approves low-risk changes for fast releases"

	// Rule: Auto-approve very low risk
	p.AddRule(*policy.NewRule("auto-approve-low", "Auto-Approve Low Risk").
		WithPriority(500).
		WithDescription("Automatically approves changes with very low risk").
		AddCondition("risk.score", policy.OperatorLessThan, threshold).
		AddCondition("change.breaking", policy.OperatorEqual, 0).
		AddCondition("change.security", policy.OperatorEqual, 0).
		AddAction(policy.ActionSetDecision, map[string]any{
			"decision": "approve",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Low-risk change auto-approved",
		}))

	// Rule: Auto-approve patches with fixes only
	p.AddRule(*policy.NewRule("auto-approve-patches", "Auto-Approve Patches").
		WithPriority(450).
		WithDescription("Automatically approves patch releases with only fixes").
		AddCondition("intent.suggestedBump", policy.OperatorEqual, "patch").
		AddCondition("change.features", policy.OperatorEqual, 0).
		AddCondition("change.breaking", policy.OperatorEqual, 0).
		AddCondition("risk.score", policy.OperatorLessThan, 0.5).
		AddAction(policy.ActionSetDecision, map[string]any{
			"decision": "approve",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Patch release with fixes only auto-approved",
		}))

	// Rule: Small changes auto-approved
	maxFiles := opts.MaxFilesWithoutReview
	if maxFiles == 0 {
		maxFiles = 3
	}
	p.AddRule(*policy.NewRule("auto-approve-small", "Auto-Approve Small Changes").
		WithPriority(400).
		WithDescription("Automatically approves small, safe changes").
		AddCondition("blastRadius.filesChanged", policy.OperatorLessOrEqual, float64(maxFiles)).
		AddCondition("change.breaking", policy.OperatorEqual, 0).
		AddCondition("risk.score", policy.OperatorLessThan, 0.4).
		AddAction(policy.ActionSetDecision, map[string]any{
			"decision": "approve",
		}))

	// Still block high risk
	p.AddRule(*policy.NewRule("auto-block-high", "Block High Risk").
		WithPriority(1000).
		WithDescription("Still blocks high-risk changes").
		AddCondition("risk.score", policy.OperatorGreaterThan, 0.8).
		AddAction(policy.ActionBlock, map[string]any{
			"reason": "High-risk change requires manual review",
		}))

	p.Defaults = policy.Defaults{
		Decision:          policy.DecisionRequireReview,
		RequiredApprovers: 1,
	}

	return p
}

func buildTrustedActorsPolicy(opts TemplateOptions) *policy.Policy {
	name := opts.PolicyName
	if name == "" {
		name = "trusted-actors"
	}

	p := policy.NewPolicy(name)
	p.Description = "Allows trusted actors to fast-track releases"

	// Rule: Team leads can approve their own low-risk changes
	p.AddRule(*policy.NewRule("trust-lead-approve", "Team Lead Self-Approve").
		WithPriority(700).
		WithDescription("Team leads can self-approve low-risk changes").
		AddCondition("actor.isTeamLead", policy.OperatorEqual, true).
		AddCondition("risk.score", policy.OperatorLessThan, 0.5).
		AddCondition("change.breaking", policy.OperatorEqual, 0).
		AddAction(policy.ActionSetDecision, map[string]any{
			"decision": "approve",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Team lead self-approved low-risk change",
		}))

	// Rule: Actors with publish permission get expedited review
	p.AddRule(*policy.NewRule("trust-publisher", "Publisher Fast Track").
		WithPriority(650).
		WithDescription("Publishers get expedited review for routine changes").
		AddCondition("actor.canPublish", policy.OperatorEqual, true).
		AddCondition("risk.score", policy.OperatorLessThan, 0.6).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1, // Reduced from default
			"description": "Expedited review for trusted publisher",
		}))

	// Rule: Allowed actors list
	if len(opts.AllowedActors) > 0 {
		p.AddRule(*policy.NewRule("trust-allowed-list", "Allowed Actors").
			WithPriority(600).
			WithDescription("Allowed actors get expedited approval").
			AddCondition("actor.id", policy.OperatorIn, opts.AllowedActors).
			AddCondition("risk.score", policy.OperatorLessThan, 0.5).
			AddAction(policy.ActionSetDecision, map[string]any{
				"decision": "approve",
			}))
	}

	// Still require review for high-risk from anyone
	p.AddRule(*policy.NewRule("trust-high-risk", "High Risk Review").
		WithPriority(900).
		WithDescription("High-risk changes always require review").
		AddCondition("risk.score", policy.OperatorGreaterThan, 0.7).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       2,
			"description": "High-risk change requires review regardless of actor",
		}))

	p.Defaults = policy.Defaults{
		Decision:          policy.DecisionRequireReview,
		RequiredApprovers: 1,
	}

	return p
}

func buildHotfixFastTrackPolicy(opts TemplateOptions) *policy.Policy {
	name := opts.PolicyName
	if name == "" {
		name = "hotfix-fast-track"
	}

	p := policy.NewPolicy(name)
	p.Description = "Expedites hotfixes and critical patches"

	// Rule: Security fixes get expedited approval
	p.AddRule(*policy.NewRule("hotfix-security", "Security Hotfix").
		WithPriority(900).
		WithDescription("Security fixes get expedited approval").
		AddCondition("change.security", policy.OperatorGreaterThan, 0).
		AddCondition("change.features", policy.OperatorEqual, 0).
		AddCondition("change.breaking", policy.OperatorEqual, 0).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Security hotfix - expedited approval",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Security hotfix fast-tracked",
		}))

	// Rule: Pure bug fixes get fast approval
	p.AddRule(*policy.NewRule("hotfix-bugfix", "Bug Fix Hotfix").
		WithPriority(850).
		WithDescription("Pure bug fixes get expedited approval").
		AddCondition("change.fixes", policy.OperatorGreaterThan, 0).
		AddCondition("change.features", policy.OperatorEqual, 0).
		AddCondition("change.breaking", policy.OperatorEqual, 0).
		AddCondition("change.total", policy.OperatorLessOrEqual, 3).
		AddAction(policy.ActionSetDecision, map[string]any{
			"decision": "approve",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Bug fix hotfix auto-approved",
		}))

	// Rule: Small patches fast-tracked
	p.AddRule(*policy.NewRule("hotfix-small-patch", "Small Patch").
		WithPriority(800).
		WithDescription("Small patch releases expedited").
		AddCondition("intent.suggestedBump", policy.OperatorEqual, "patch").
		AddCondition("blastRadius.filesChanged", policy.OperatorLessOrEqual, 5).
		AddCondition("change.breaking", policy.OperatorEqual, 0).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Small patch expedited",
		}))

	// Rule: Hotfix during freeze allowed with approval
	p.AddRule(*policy.NewRule("hotfix-freeze-override", "Freeze Override for Hotfix").
		WithPriority(950).
		WithDescription("Critical hotfixes can bypass freeze with approval").
		AddCondition("time.inFreeze", policy.OperatorEqual, true).
		AddCondition("change.security", policy.OperatorGreaterThan, 0).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       2,
			"description": "Security hotfix during freeze requires 2 approvals",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Security hotfix allowed during freeze with additional approval",
		}))

	p.Defaults = policy.Defaults{
		Decision:          policy.DecisionRequireReview,
		RequiredApprovers: 1,
	}

	return p
}

func buildCIFriendlyPolicy(opts TemplateOptions) *policy.Policy {
	name := opts.PolicyName
	if name == "" {
		name = "ci-friendly"
	}

	p := policy.NewPolicy(name)
	p.Description = "Optimized for CI/CD pipelines with automated approvals"

	// Rule: CI can auto-release low-risk patches
	p.AddRule(*policy.NewRule("ci-auto-patch", "CI Auto-Patch").
		WithPriority(700).
		WithDescription("CI can automatically release low-risk patches").
		AddCondition("actor.kind", policy.OperatorEqual, "ci").
		AddCondition("intent.suggestedBump", policy.OperatorEqual, "patch").
		AddCondition("risk.score", policy.OperatorLessThan, 0.3).
		AddCondition("change.breaking", policy.OperatorEqual, 0).
		AddAction(policy.ActionSetDecision, map[string]any{
			"decision": "approve",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "CI auto-approved low-risk patch",
		}))

	// Rule: CI minor releases need one human
	p.AddRule(*policy.NewRule("ci-minor-review", "CI Minor Review").
		WithPriority(600).
		WithDescription("CI minor releases need human approval").
		AddCondition("actor.kind", policy.OperatorEqual, "ci").
		AddCondition("intent.suggestedBump", policy.OperatorEqual, "minor").
		AddCondition("risk.score", policy.OperatorLessThan, 0.5).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "CI minor release requires one human approval",
		}))

	// Rule: CI major releases need more review
	p.AddRule(*policy.NewRule("ci-major-review", "CI Major Review").
		WithPriority(500).
		WithDescription("CI major releases need additional review").
		AddCondition("actor.kind", policy.OperatorEqual, "ci").
		AddCondition("intent.suggestedBump", policy.OperatorEqual, "major").
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       2,
			"description": "CI major release requires human review",
		}))

	// Rule: Bot updates with high confidence auto-approved
	p.AddRule(*policy.NewRule("ci-bot-confident", "Bot High Confidence").
		WithPriority(650).
		WithDescription("Bot updates with high confidence auto-approved").
		AddCondition("actor.kind", policy.OperatorEqual, "bot").
		AddCondition("intent.confidence", policy.OperatorGreaterThan, 0.9).
		AddCondition("risk.score", policy.OperatorLessThan, 0.3).
		AddCondition("change.breaking", policy.OperatorEqual, 0).
		AddAction(policy.ActionSetDecision, map[string]any{
			"decision": "approve",
		}))

	// Block CI from high-risk
	p.AddRule(*policy.NewRule("ci-high-risk-block", "CI High Risk Block").
		WithPriority(1000).
		WithDescription("Blocks CI from proposing high-risk changes").
		AddCondition("actor.kind", policy.OperatorIn, []string{"ci", "bot"}).
		AddCondition("risk.score", policy.OperatorGreaterThan, 0.7).
		AddAction(policy.ActionBlock, map[string]any{
			"reason": "Automated systems cannot propose high-risk changes",
		}))

	p.Defaults = policy.Defaults{
		Decision:          policy.DecisionRequireReview,
		RequiredApprovers: 1,
	}

	return p
}
