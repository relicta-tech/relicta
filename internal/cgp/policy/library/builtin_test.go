package library

import (
	"testing"
)

func TestSecurityTemplates(t *testing.T) {
	templates := []string{
		"security-basic",
		"security-strict",
		"security-breaking-changes",
		"security-dependency",
	}

	for _, id := range templates {
		t.Run(id, func(t *testing.T) {
			template, ok := DefaultRegistry.Get(id)
			if !ok {
				t.Fatalf("template %s not found", id)
			}

			opts := DefaultTemplateOptions()
			opts.SecurityTeam = "security-team"

			policy := template.Build(opts)
			if policy == nil {
				t.Fatal("Build returned nil")
			}
			if policy.Name == "" {
				t.Error("Policy name is empty")
			}
			if len(policy.Rules) == 0 {
				t.Error("Policy has no rules")
			}

			// Validate the policy
			if err := policy.Validate(); err != nil {
				t.Errorf("Policy validation failed: %v", err)
			}
		})
	}
}

func TestStabilityTemplates(t *testing.T) {
	templates := []string{
		"stability-production",
		"stability-gradual",
		"stability-freeze",
		"stability-confidence",
	}

	for _, id := range templates {
		t.Run(id, func(t *testing.T) {
			template, ok := DefaultRegistry.Get(id)
			if !ok {
				t.Fatalf("template %s not found", id)
			}

			opts := DefaultTemplateOptions()
			opts.FreezeDuringHolidays = true
			opts.RequireBusinessHours = true

			policy := template.Build(opts)
			if policy == nil {
				t.Fatal("Build returned nil")
			}
			if len(policy.Rules) == 0 {
				t.Error("Policy has no rules")
			}

			if err := policy.Validate(); err != nil {
				t.Errorf("Policy validation failed: %v", err)
			}
		})
	}
}

func TestSpeedTemplates(t *testing.T) {
	templates := []string{
		"speed-auto-approve",
		"speed-trusted-actors",
		"speed-hotfix",
		"speed-ci-friendly",
	}

	for _, id := range templates {
		t.Run(id, func(t *testing.T) {
			template, ok := DefaultRegistry.Get(id)
			if !ok {
				t.Fatalf("template %s not found", id)
			}

			opts := DefaultTemplateOptions()
			opts.AllowedActors = []string{"trusted-user-1", "trusted-user-2"}

			policy := template.Build(opts)
			if policy == nil {
				t.Fatal("Build returned nil")
			}
			if len(policy.Rules) == 0 {
				t.Error("Policy has no rules")
			}

			if err := policy.Validate(); err != nil {
				t.Errorf("Policy validation failed: %v", err)
			}
		})
	}
}

func TestEnterpriseTemplates(t *testing.T) {
	templates := []string{
		"enterprise-soc2",
		"enterprise-separation-of-duties",
		"enterprise-multi-team",
		"enterprise-audit-trail",
		"enterprise-complete",
	}

	for _, id := range templates {
		t.Run(id, func(t *testing.T) {
			template, ok := DefaultRegistry.Get(id)
			if !ok {
				t.Fatalf("template %s not found", id)
			}

			opts := DefaultTemplateOptions()
			opts.SecurityTeam = "security"
			opts.LeadTeam = "leads"
			opts.FreezeDuringHolidays = true

			policy := template.Build(opts)
			if policy == nil {
				t.Fatal("Build returned nil")
			}
			if len(policy.Rules) == 0 {
				t.Error("Policy has no rules")
			}

			if err := policy.Validate(); err != nil {
				t.Errorf("Policy validation failed: %v", err)
			}
		})
	}
}

func TestTemplateCustomNames(t *testing.T) {
	template, _ := DefaultRegistry.Get("security-basic")

	customName := "my-custom-security-policy"
	policy := template.Build(TemplateOptions{
		PolicyName: customName,
	})

	if policy.Name != customName {
		t.Errorf("expected name %s, got %s", customName, policy.Name)
	}
}

func TestTemplateCategories(t *testing.T) {
	categories := DefaultRegistry.Categories()
	if len(categories) < 4 {
		t.Errorf("expected at least 4 categories, got %d", len(categories))
	}

	// Check each category has templates
	for _, cat := range categories {
		templates := DefaultRegistry.ListByCategory(cat)
		if len(templates) == 0 {
			t.Errorf("category %s has no templates", cat)
		}
	}
}

func TestSecurityPolicyRulePriorities(t *testing.T) {
	template, _ := DefaultRegistry.Get("security-basic")
	policy := template.Build(DefaultTemplateOptions())

	// Verify rules are sorted by priority (higher = first)
	lastPriority := 10000
	for _, rule := range policy.Rules {
		if rule.Priority > lastPriority {
			t.Errorf("rule %s (priority %d) appears after rule with lower priority %d",
				rule.ID, rule.Priority, lastPriority)
		}
		lastPriority = rule.Priority
	}
}

func TestEnterpriseCompleteHasComprehensiveRules(t *testing.T) {
	template, _ := DefaultRegistry.Get("enterprise-complete")
	policy := template.Build(TemplateOptions{
		SecurityTeam:         "security",
		FreezeDuringHolidays: true,
	})

	// Should have rules covering multiple concerns
	ruleIDs := make(map[string]bool)
	for _, rule := range policy.Rules {
		ruleIDs[rule.ID] = true
	}

	expectedRulePatterns := []string{
		"ent-critical-block", // Security
		"ent-bot-block",      // Automation control
		"ent-cab",            // SOC 2
		"ent-breaking",       // Stability
		"ent-production",     // Production protection
		"ent-sod-bot",        // Separation of duties
		"ent-audit",          // Audit trail
	}

	for _, pattern := range expectedRulePatterns {
		if !ruleIDs[pattern] {
			t.Errorf("enterprise-complete missing expected rule: %s", pattern)
		}
	}
}

func TestBuildAllByCategory(t *testing.T) {
	policies := DefaultRegistry.BuildAll(CategorySecurity, DefaultTemplateOptions())
	if len(policies) == 0 {
		t.Error("BuildAll returned no policies for security category")
	}

	for _, p := range policies {
		if err := p.Validate(); err != nil {
			t.Errorf("policy %s validation failed: %v", p.Name, err)
		}
	}
}
