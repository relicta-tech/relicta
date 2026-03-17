package library

import (
	"github.com/relicta-tech/relicta/internal/cgp/policy"
)

// registerSecurityTemplates adds security-focused policy templates.
func registerSecurityTemplates(r *Registry) {
	mustRegister(r, &PolicyTemplate{
		ID:          "security-basic",
		Name:        "Basic Security",
		Description: "Essential security controls for any project. Blocks high-risk changes and requires review for security-related commits.",
		Category:    CategorySecurity,
		Tags:        []string{"security", "basic", "starter"},
		Build:       buildBasicSecurityPolicy,
	})

	mustRegister(r, &PolicyTemplate{
		ID:          "security-strict",
		Name:        "Strict Security",
		Description: "Comprehensive security controls with mandatory security team review for sensitive changes.",
		Category:    CategorySecurity,
		Tags:        []string{"security", "strict", "enterprise"},
		Build:       buildStrictSecurityPolicy,
	})

	mustRegister(r, &PolicyTemplate{
		ID:          "security-breaking-changes",
		Name:        "Breaking Change Protection",
		Description: "Requires additional approvals for breaking changes that could impact downstream consumers.",
		Category:    CategorySecurity,
		Tags:        []string{"security", "breaking", "api"},
		Build:       buildBreakingChangesPolicy,
	})

	mustRegister(r, &PolicyTemplate{
		ID:          "security-dependency",
		Name:        "Dependency Security",
		Description: "Controls around dependency updates to prevent supply chain attacks.",
		Category:    CategorySecurity,
		Tags:        []string{"security", "dependencies", "supply-chain"},
		Build:       buildDependencySecurityPolicy,
	})
}

func buildBasicSecurityPolicy(opts TemplateOptions) *policy.Policy {
	name := opts.PolicyName
	if name == "" {
		name = "basic-security"
	}

	threshold := opts.RiskThreshold
	if threshold == 0 {
		threshold = 0.7
	}

	p := policy.NewPolicy(name)
	p.Description = "Essential security controls for release governance"

	// Rule: Block extremely high-risk changes
	p.AddRule(*policy.NewRule("sec-block-critical", "Block Critical Risk").
		WithPriority(1000).
		WithDescription("Blocks changes with risk score above 0.9").
		AddCondition("risk.score", policy.OperatorGreaterThan, 0.9).
		AddAction(policy.ActionBlock, map[string]any{
			"reason": "Risk score exceeds critical threshold (>0.9). Manual intervention required.",
		}))

	// Rule: Require review for high-risk changes
	p.AddRule(*policy.NewRule("sec-high-risk", "High Risk Review").
		WithPriority(900).
		WithDescription("Requires review for changes with elevated risk").
		AddCondition("risk.score", policy.OperatorGreaterThan, threshold).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       2,
			"description": "High risk change requires additional review",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Risk score exceeds threshold, requiring additional approval",
		}))

	// Rule: Require review for security-related changes
	p.AddRule(*policy.NewRule("sec-security-changes", "Security Changes Review").
		WithPriority(850).
		WithDescription("Requires review for security-related commits").
		AddCondition("change.security", policy.OperatorGreaterThan, 0).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Security-related changes require review",
		}))

	// Rule: Flag breaking changes
	p.AddRule(*policy.NewRule("sec-breaking", "Breaking Change Notice").
		WithPriority(800).
		WithDescription("Flags breaking changes for awareness").
		AddCondition("intent.hasBreaking", policy.OperatorEqual, true).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Breaking changes require explicit approval",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "This release contains breaking changes",
		}))

	// Set defaults
	p.Defaults = policy.Defaults{
		Decision:          policy.DecisionApprove,
		RequiredApprovers: opts.RequiredApprovers,
	}

	return p
}

func buildStrictSecurityPolicy(opts TemplateOptions) *policy.Policy {
	name := opts.PolicyName
	if name == "" {
		name = "strict-security"
	}

	p := policy.NewPolicy(name)
	p.Description = "Comprehensive security controls with mandatory team reviews"

	// Rule: Block high-risk from bots
	p.AddRule(*policy.NewRule("strict-bot-block", "Bot High Risk Block").
		WithPriority(1000).
		WithDescription("Blocks high-risk changes from automated systems").
		AddCondition("actor.kind", policy.OperatorEqual, "bot").
		AddCondition("risk.score", policy.OperatorGreaterThan, 0.5).
		AddAction(policy.ActionBlock, map[string]any{
			"reason": "Automated systems cannot propose high-risk changes",
		}))

	// Rule: Block critical risk
	p.AddRule(*policy.NewRule("strict-critical", "Block Critical Risk").
		WithPriority(950).
		WithDescription("Blocks changes with critical risk score").
		AddCondition("risk.score", policy.OperatorGreaterThan, 0.85).
		AddAction(policy.ActionBlock, map[string]any{
			"reason": "Risk score exceeds critical threshold. Requires manual review outside standard flow.",
		}))

	// Rule: Security team review for security changes
	if opts.SecurityTeam != "" {
		p.AddRule(*policy.NewRule("strict-security-team", "Security Team Review").
			WithPriority(900).
			WithDescription("Requires security team review for security-related changes").
			AddCondition("change.security", policy.OperatorGreaterThan, 0).
			AddAction(policy.ActionRequireTeamReview, map[string]any{
				"team":  opts.SecurityTeam,
				"count": 1,
			}))
	}

	// Rule: Breaking changes require lead approval
	p.AddRule(*policy.NewRule("strict-breaking-lead", "Breaking Changes Lead Review").
		WithPriority(850).
		WithDescription("Breaking changes require team lead approval").
		AddCondition("intent.hasBreaking", policy.OperatorEqual, true).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       2,
			"description": "Breaking changes require lead approval",
		}))

	// Rule: API changes require extra review
	p.AddRule(*policy.NewRule("strict-api-changes", "API Changes Review").
		WithPriority(800).
		WithDescription("API changes require additional scrutiny").
		AddCondition("change.hasAPIChange", policy.OperatorEqual, true).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       2,
			"description": "API changes affect consumers and require review",
		}))

	// Rule: Large blast radius
	maxFiles := opts.MaxFilesWithoutReview
	if maxFiles == 0 {
		maxFiles = 10
	}
	p.AddRule(*policy.NewRule("strict-blast-radius", "Large Blast Radius Review").
		WithPriority(750).
		WithDescription("Large changes require additional review").
		AddCondition("blastRadius.filesChanged", policy.OperatorGreaterThan, float64(maxFiles)).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       2,
			"description": "Large change scope requires additional review",
		}))

	// Rule: Always require at least one review
	p.AddRule(*policy.NewRule("strict-default-review", "Default Review").
		WithPriority(100).
		WithDescription("All changes require at least one review").
		AddCondition("risk.score", policy.OperatorGreaterOrEqual, 0).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Standard review required",
		}))

	p.Defaults = policy.Defaults{
		Decision:          policy.DecisionRequireReview,
		RequiredApprovers: 1,
	}

	return p
}

func buildBreakingChangesPolicy(opts TemplateOptions) *policy.Policy {
	name := opts.PolicyName
	if name == "" {
		name = "breaking-changes"
	}

	p := policy.NewPolicy(name)
	p.Description = "Protection for breaking changes affecting downstream consumers"

	// Rule: Major version bump from breaking changes
	p.AddRule(*policy.NewRule("break-major", "Major Version Breaking").
		WithPriority(900).
		WithDescription("Major version changes with breaking require multi-approval").
		AddCondition("intent.suggestedBump", policy.OperatorEqual, "major").
		AddCondition("change.breaking", policy.OperatorGreaterThan, 0).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       3,
			"description": "Major version with breaking changes requires extensive review",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Major version bump with breaking changes - ensure migration documentation exists",
		}))

	// Rule: Breaking API changes
	p.AddRule(*policy.NewRule("break-api", "Breaking API Changes").
		WithPriority(850).
		WithDescription("API breaking changes require additional approval").
		AddCondition("intent.hasBreaking", policy.OperatorEqual, true).
		AddCondition("change.hasAPIChange", policy.OperatorEqual, true).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       2,
			"description": "Breaking API changes affect consumers",
		}))

	// Rule: Any breaking changes
	p.AddRule(*policy.NewRule("break-any", "Any Breaking Changes").
		WithPriority(800).
		WithDescription("All breaking changes require review").
		AddCondition("intent.hasBreaking", policy.OperatorEqual, true).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Breaking changes require explicit approval",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Contains breaking changes - verify changelog documents the impact",
		}))

	p.Defaults = policy.Defaults{
		Decision:          policy.DecisionApprove,
		RequiredApprovers: 0,
	}

	return p
}

func buildDependencySecurityPolicy(opts TemplateOptions) *policy.Policy {
	name := opts.PolicyName
	if name == "" {
		name = "dependency-security"
	}

	p := policy.NewPolicy(name)
	p.Description = "Supply chain security controls for dependency updates"

	// Rule: High-risk with dependencies
	p.AddRule(*policy.NewRule("dep-high-risk", "High Risk Dependencies").
		WithPriority(900).
		WithDescription("Dependency changes with high risk require extra review").
		AddCondition("change.dependencies", policy.OperatorGreaterThan, 0).
		AddCondition("risk.score", policy.OperatorGreaterThan, 0.6).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       2,
			"description": "High-risk dependency changes require security review",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Dependency changes with elevated risk - verify no vulnerable packages",
		}))

	// Rule: Many dependencies changed
	p.AddRule(*policy.NewRule("dep-many-changes", "Many Dependency Changes").
		WithPriority(850).
		WithDescription("Large dependency updates require review").
		AddCondition("change.dependencies", policy.OperatorGreaterThan, 5).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Multiple dependency changes require review",
		}))

	// Rule: Bot dependency updates
	p.AddRule(*policy.NewRule("dep-bot-updates", "Bot Dependency Updates").
		WithPriority(800).
		WithDescription("Automated dependency updates require human review").
		AddCondition("actor.kind", policy.OperatorEqual, "bot").
		AddCondition("change.dependencies", policy.OperatorGreaterThan, 0).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Automated dependency updates require human approval",
		}))

	p.Defaults = policy.Defaults{
		Decision:          policy.DecisionApprove,
		RequiredApprovers: 0,
	}

	return p
}
