package library

import (
	"github.com/relicta-tech/relicta/internal/cgp/policy"
)

// registerEnterpriseTemplates adds enterprise-focused policy templates.
func registerEnterpriseTemplates(r *Registry) {
	mustRegister(r, &PolicyTemplate{
		ID:          "enterprise-soc2",
		Name:        "SOC 2 Compliance",
		Description: "Policy controls aligned with SOC 2 Type II requirements for change management.",
		Category:    CategoryEnterprise,
		Tags:        []string{"enterprise", "compliance", "soc2", "audit"},
		Build:       buildSOC2Policy,
	})

	mustRegister(r, &PolicyTemplate{
		ID:          "enterprise-separation-of-duties",
		Name:        "Separation of Duties",
		Description: "Enforces separation between change proposers and approvers.",
		Category:    CategoryEnterprise,
		Tags:        []string{"enterprise", "sod", "governance"},
		Build:       buildSeparationOfDutiesPolicy,
	})

	mustRegister(r, &PolicyTemplate{
		ID:          "enterprise-multi-team",
		Name:        "Multi-Team Approval",
		Description: "Requires approvals from multiple teams for cross-cutting changes.",
		Category:    CategoryEnterprise,
		Tags:        []string{"enterprise", "teams", "collaboration"},
		Build:       buildMultiTeamApprovalPolicy,
	})

	mustRegister(r, &PolicyTemplate{
		ID:          "enterprise-audit-trail",
		Name:        "Audit Trail",
		Description: "Ensures comprehensive audit trail for all changes.",
		Category:    CategoryEnterprise,
		Tags:        []string{"enterprise", "audit", "compliance"},
		Build:       buildAuditTrailPolicy,
	})

	mustRegister(r, &PolicyTemplate{
		ID:          "enterprise-complete",
		Name:        "Enterprise Complete",
		Description: "Comprehensive enterprise policy combining security, stability, and compliance.",
		Category:    CategoryEnterprise,
		Tags:        []string{"enterprise", "complete", "comprehensive"},
		Build:       buildEnterpriseCompletePolicy,
	})
}

func buildSOC2Policy(opts TemplateOptions) *policy.Policy {
	name := opts.PolicyName
	if name == "" {
		name = "soc2-compliance"
	}

	p := policy.NewPolicy(name)
	p.Description = "SOC 2 Type II aligned change management controls"

	// CC6.1 - Logical and Physical Access Controls
	// Changes require authorized approval
	p.AddRule(*policy.NewRule("soc2-auth-approval", "Authorized Approval Required").
		WithPriority(900).
		WithDescription("All changes require authorized approval (CC6.1)").
		AddCondition("risk.score", policy.OperatorGreaterOrEqual, 0).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "SOC 2 CC6.1 - Authorized approval required",
		}))

	// CC7.1 - System Monitoring
	// Breaking changes require additional review
	p.AddRule(*policy.NewRule("soc2-breaking-review", "Breaking Change Review").
		WithPriority(850).
		WithDescription("Breaking changes require additional review (CC7.1)").
		AddCondition("intent.hasBreaking", policy.OperatorEqual, true).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       2,
			"description": "SOC 2 CC7.1 - Breaking changes require enhanced review",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "CC7.1 - System change with breaking impact logged",
		}))

	// CC7.4 - Incident Response
	// Security changes flagged for incident tracking
	p.AddRule(*policy.NewRule("soc2-security-tracking", "Security Change Tracking").
		WithPriority(800).
		WithDescription("Security changes tracked for incident response (CC7.4)").
		AddCondition("change.security", policy.OperatorGreaterThan, 0).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "SOC 2 CC7.4 - Security change requires tracking",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "CC7.4 - Security-related change logged for incident tracking",
		}))

	// CC8.1 - Change Management
	// High-risk changes require change advisory board
	p.AddRule(*policy.NewRule("soc2-cab-review", "Change Advisory Board").
		WithPriority(950).
		WithDescription("High-risk changes require CAB review (CC8.1)").
		AddCondition("risk.score", policy.OperatorGreaterThan, 0.7).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       3,
			"description": "SOC 2 CC8.1 - Change Advisory Board review required",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "CC8.1 - High-risk change escalated to CAB",
		}))

	// Block extremely high risk
	p.AddRule(*policy.NewRule("soc2-block-critical", "Block Critical Risk").
		WithPriority(1000).
		WithDescription("Critical risk changes blocked pending review (CC8.1)").
		AddCondition("risk.score", policy.OperatorGreaterThan, 0.9).
		AddAction(policy.ActionBlock, map[string]any{
			"reason": "SOC 2 CC8.1 - Critical risk change requires out-of-band review",
		}))

	p.Defaults = policy.Defaults{
		Decision:          policy.DecisionRequireReview,
		RequiredApprovers: 1,
	}

	return p
}

func buildSeparationOfDutiesPolicy(opts TemplateOptions) *policy.Policy {
	name := opts.PolicyName
	if name == "" {
		name = "separation-of-duties"
	}

	p := policy.NewPolicy(name)
	p.Description = "Enforces separation between proposers and approvers"

	// Rule: Proposers cannot approve their own changes
	// This is a documentation rule - actual enforcement is in approval flow
	p.AddRule(*policy.NewRule("sod-self-approve", "No Self-Approval").
		WithPriority(1000).
		WithDescription("Proposers cannot approve their own changes").
		AddCondition("risk.score", policy.OperatorGreaterOrEqual, 0).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Separation of duties: Proposer cannot be sole approver",
		}))

	// Rule: CI/Bot changes require human approval
	p.AddRule(*policy.NewRule("sod-bot-human", "Bot Requires Human").
		WithPriority(900).
		WithDescription("Automated changes require human approval").
		AddCondition("actor.kind", policy.OperatorIn, []string{"ci", "bot"}).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Automated change requires human approval",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Separation of duties: Bot/CI change requires human review",
		}))

	// Rule: High-risk changes require different team reviewer
	p.AddRule(*policy.NewRule("sod-cross-team", "Cross-Team Review").
		WithPriority(850).
		WithDescription("High-risk changes require cross-team review").
		AddCondition("risk.score", policy.OperatorGreaterThan, 0.6).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       2,
			"description": "High-risk change requires cross-team review",
		}))

	// Rule: Breaking changes require non-author review
	p.AddRule(*policy.NewRule("sod-breaking", "Breaking Change Review").
		WithPriority(800).
		WithDescription("Breaking changes require non-author review").
		AddCondition("intent.hasBreaking", policy.OperatorEqual, true).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Breaking change requires non-author approval",
		}))

	p.Defaults = policy.Defaults{
		Decision:          policy.DecisionRequireReview,
		RequiredApprovers: 1,
	}

	return p
}

func buildMultiTeamApprovalPolicy(opts TemplateOptions) *policy.Policy {
	name := opts.PolicyName
	if name == "" {
		name = "multi-team-approval"
	}

	p := policy.NewPolicy(name)
	p.Description = "Requires approvals from multiple teams for cross-cutting changes"

	// Rule: API changes require API team
	p.AddRule(*policy.NewRule("multi-api-team", "API Team Review").
		WithPriority(800).
		WithDescription("API changes require API team review").
		AddCondition("change.hasAPIChange", policy.OperatorEqual, true).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "API change requires API team approval",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "API change flagged for API team review",
		}))

	// Rule: Security changes require security team
	if opts.SecurityTeam != "" {
		p.AddRule(*policy.NewRule("multi-security-team", "Security Team Review").
			WithPriority(850).
			WithDescription("Security changes require security team review").
			AddCondition("change.security", policy.OperatorGreaterThan, 0).
			AddAction(policy.ActionRequireTeamReview, map[string]any{
				"team":  opts.SecurityTeam,
				"count": 1,
			}))
	}

	// Rule: Large changes require lead approval
	if opts.LeadTeam != "" {
		maxFiles := opts.MaxFilesWithoutReview
		if maxFiles == 0 {
			maxFiles = 20
		}
		p.AddRule(*policy.NewRule("multi-lead-large", "Lead Review for Large Changes").
			WithPriority(750).
			WithDescription("Large changes require lead approval").
			AddCondition("blastRadius.filesChanged", policy.OperatorGreaterThan, float64(maxFiles)).
			AddAction(policy.ActionRequireTeamLead, map[string]any{
				"team": opts.LeadTeam,
			}))
	}

	// Rule: Breaking changes require multiple teams
	p.AddRule(*policy.NewRule("multi-breaking", "Multi-Team Breaking Review").
		WithPriority(900).
		WithDescription("Breaking changes require multiple team approvals").
		AddCondition("intent.hasBreaking", policy.OperatorEqual, true).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       2,
			"description": "Breaking change requires multi-team approval",
		}))

	p.Defaults = policy.Defaults{
		Decision:          policy.DecisionRequireReview,
		RequiredApprovers: 1,
	}

	return p
}

func buildAuditTrailPolicy(opts TemplateOptions) *policy.Policy {
	name := opts.PolicyName
	if name == "" {
		name = "audit-trail"
	}

	p := policy.NewPolicy(name)
	p.Description = "Ensures comprehensive audit trail for all changes"

	// Rule: All changes logged with rationale
	p.AddRule(*policy.NewRule("audit-all-changes", "Audit All Changes").
		WithPriority(100).
		WithDescription("All changes logged with rationale").
		AddCondition("risk.score", policy.OperatorGreaterOrEqual, 0).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Change proposal logged for audit trail",
		}))

	// Rule: Breaking changes documented
	p.AddRule(*policy.NewRule("audit-breaking", "Breaking Change Audit").
		WithPriority(800).
		WithDescription("Breaking changes require explicit documentation").
		AddCondition("intent.hasBreaking", policy.OperatorEqual, true).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "AUDIT: Breaking change - requires changelog entry and migration docs",
		}).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Breaking change audit approval",
		}))

	// Rule: Security changes audited
	p.AddRule(*policy.NewRule("audit-security", "Security Change Audit").
		WithPriority(850).
		WithDescription("Security changes require audit documentation").
		AddCondition("change.security", policy.OperatorGreaterThan, 0).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "AUDIT: Security-related change - requires security review documentation",
		}).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Security change audit approval",
		}))

	// Rule: High-risk changes require extensive audit
	p.AddRule(*policy.NewRule("audit-high-risk", "High Risk Audit").
		WithPriority(900).
		WithDescription("High-risk changes require extensive audit trail").
		AddCondition("risk.score", policy.OperatorGreaterThan, 0.7).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "AUDIT: High-risk change - full audit trail required",
		}).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       2,
			"description": "High-risk audit requires multiple approvals",
		}))

	// Rule: Major version changes audited
	p.AddRule(*policy.NewRule("audit-major", "Major Version Audit").
		WithPriority(750).
		WithDescription("Major version changes require audit documentation").
		AddCondition("intent.suggestedBump", policy.OperatorEqual, "major").
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "AUDIT: Major version bump - requires release notes and upgrade guide",
		}))

	p.Defaults = policy.Defaults{
		Decision:          policy.DecisionRequireReview,
		RequiredApprovers: 1,
	}

	return p
}

func buildEnterpriseCompletePolicy(opts TemplateOptions) *policy.Policy {
	name := opts.PolicyName
	if name == "" {
		name = "enterprise-complete"
	}

	p := policy.NewPolicy(name)
	p.Description = "Comprehensive enterprise policy combining security, stability, and compliance"

	// CRITICAL: Block extremely high risk
	p.AddRule(*policy.NewRule("ent-critical-block", "Block Critical Risk").
		WithPriority(1000).
		WithDescription("Blocks critical risk changes").
		AddCondition("risk.score", policy.OperatorGreaterThan, 0.9).
		AddAction(policy.ActionBlock, map[string]any{
			"reason": "Critical risk change requires out-of-band review process",
		}))

	// SECURITY: Block bots from high-risk
	p.AddRule(*policy.NewRule("ent-bot-block", "Block Bot High Risk").
		WithPriority(980).
		WithDescription("Automated systems cannot propose high-risk changes").
		AddCondition("actor.kind", policy.OperatorIn, []string{"ci", "bot"}).
		AddCondition("risk.score", policy.OperatorGreaterThan, 0.6).
		AddAction(policy.ActionBlock, map[string]any{
			"reason": "Automated systems blocked from high-risk changes",
		}))

	// SOC 2: High-risk requires CAB
	p.AddRule(*policy.NewRule("ent-cab", "Change Advisory Board").
		WithPriority(950).
		WithDescription("High-risk changes require CAB review").
		AddCondition("risk.score", policy.OperatorGreaterThan, 0.7).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       3,
			"description": "CAB review required for high-risk change",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Escalated to Change Advisory Board",
		}))

	// SECURITY: Security team for security changes
	if opts.SecurityTeam != "" {
		p.AddRule(*policy.NewRule("ent-security-team", "Security Team Review").
			WithPriority(900).
			WithDescription("Security changes require security team").
			AddCondition("change.security", policy.OperatorGreaterThan, 0).
			AddAction(policy.ActionRequireTeamReview, map[string]any{
				"team":  opts.SecurityTeam,
				"count": 1,
			}))
	}

	// STABILITY: Breaking changes require extensive review
	p.AddRule(*policy.NewRule("ent-breaking", "Breaking Change Review").
		WithPriority(850).
		WithDescription("Breaking changes require extensive review").
		AddCondition("intent.hasBreaking", policy.OperatorEqual, true).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       2,
			"description": "Breaking change requires multi-approver review",
		}).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Breaking change - verify migration docs and changelog",
		}))

	// STABILITY: Production branch protection
	branches := opts.ProductionBranches
	if len(branches) == 0 {
		branches = []string{"main", "master", "production"}
	}
	p.AddRule(*policy.NewRule("ent-production", "Production Protection").
		WithPriority(800).
		WithDescription("Production branches require approval").
		AddCondition("scope.branch", policy.OperatorIn, branches).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Production branch requires approval",
		}))

	// STABILITY: Weekend/freeze restrictions
	if opts.FreezeDuringHolidays {
		p.AddRule(*policy.NewRule("ent-freeze", "Freeze Period Block").
			WithPriority(960).
			WithDescription("Blocks releases during freeze").
			AddCondition("time.inFreeze", policy.OperatorEqual, true).
			AddAction(policy.ActionBlock, map[string]any{
				"reason": "Releases blocked during freeze period",
			}))
	}

	// SoD: Bot requires human
	p.AddRule(*policy.NewRule("ent-sod-bot", "Bot Requires Human").
		WithPriority(750).
		WithDescription("Automated changes require human approval").
		AddCondition("actor.kind", policy.OperatorIn, []string{"ci", "bot"}).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Automated change requires human approval",
		}))

	// AUDIT: All changes logged
	p.AddRule(*policy.NewRule("ent-audit", "Audit Trail").
		WithPriority(100).
		WithDescription("All changes logged for audit").
		AddCondition("risk.score", policy.OperatorGreaterOrEqual, 0).
		AddAction(policy.ActionAddRationale, map[string]any{
			"message": "Enterprise policy: Change logged for audit compliance",
		}))

	// SPEED: Allow low-risk patches through faster
	p.AddRule(*policy.NewRule("ent-fast-patch", "Fast Track Patches").
		WithPriority(600).
		WithDescription("Low-risk patches can proceed faster").
		AddCondition("intent.suggestedBump", policy.OperatorEqual, "patch").
		AddCondition("risk.score", policy.OperatorLessThan, 0.3).
		AddCondition("change.breaking", policy.OperatorEqual, 0).
		AddCondition("change.security", policy.OperatorEqual, 0).
		AddAction(policy.ActionRequireApproval, map[string]any{
			"count":       1,
			"description": "Low-risk patch - standard approval",
		}))

	p.Defaults = policy.Defaults{
		Decision:          policy.DecisionRequireReview,
		RequiredApprovers: 1,
	}

	return p
}
