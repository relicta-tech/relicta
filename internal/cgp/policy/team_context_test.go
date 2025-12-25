package policy

import (
	"context"
	"testing"

	"github.com/relicta-tech/relicta/internal/cgp"
)

func TestDefaultTeamContext(t *testing.T) {
	tc := DefaultTeamContext()

	if tc == nil {
		t.Fatal("DefaultTeamContext returned nil")
	}

	if tc.Teams == nil {
		t.Error("Teams map should not be nil")
	}

	if tc.Roles == nil {
		t.Error("Roles map should not be nil")
	}
}

func TestTeamContext_AddTeam(t *testing.T) {
	tc := DefaultTeamContext()

	team := NewTeam("platform", "Platform Engineering").
		WithMembers("alice", "bob").
		WithLeads("carol")

	tc.AddTeam(team)

	if len(tc.Teams) != 1 {
		t.Errorf("expected 1 team, got %d", len(tc.Teams))
	}

	if !tc.IsTeamMember("alice", "platform") {
		t.Error("alice should be a member of platform team")
	}

	if !tc.IsTeamMember("carol", "platform") {
		t.Error("carol should be a member of platform team (as lead)")
	}

	if !tc.IsTeamLead("carol", "platform") {
		t.Error("carol should be a lead of platform team")
	}

	if tc.IsTeamLead("alice", "platform") {
		t.Error("alice should not be a lead of platform team")
	}
}

func TestTeamContext_AddRole(t *testing.T) {
	tc := DefaultTeamContext()

	role := NewApproverRole("release-manager", "Can approve releases")
	tc.AddRole(role)

	if len(tc.Roles) != 1 {
		t.Errorf("expected 1 role, got %d", len(tc.Roles))
	}

	r, ok := tc.GetRole("release-manager")
	if !ok {
		t.Error("role not found")
	}

	if !r.CanApprove {
		t.Error("release-manager should be able to approve")
	}
}

func TestTeamContext_AssignRole(t *testing.T) {
	tc := DefaultTeamContext()

	tc.AddRole(NewApproverRole("approver", "Approver role"))
	tc.AssignRole("alice", "approver")

	if !tc.HasRole("alice", "approver") {
		t.Error("alice should have approver role")
	}

	if tc.HasRole("bob", "approver") {
		t.Error("bob should not have approver role")
	}

	if !tc.CanApprove("alice") {
		t.Error("alice should be able to approve")
	}
}

func TestTeamContext_GetActorTeams(t *testing.T) {
	tc := DefaultTeamContext()

	tc.AddTeam(NewTeam("frontend", "Frontend Team").WithMembers("alice", "bob"))
	tc.AddTeam(NewTeam("backend", "Backend Team").WithMembers("alice", "carol"))

	aliceTeams := tc.GetActorTeams("alice")
	if len(aliceTeams) != 2 {
		t.Errorf("alice should be in 2 teams, got %d", len(aliceTeams))
	}

	bobTeams := tc.GetActorTeams("bob")
	if len(bobTeams) != 1 {
		t.Errorf("bob should be in 1 team, got %d", len(bobTeams))
	}
}

func TestTeamContext_HasPermission(t *testing.T) {
	tc := DefaultTeamContext()

	// Add role with wildcard permission
	adminRole := &Role{
		Name:        "admin",
		Permissions: []string{"*"},
	}
	tc.AddRole(adminRole)
	tc.AssignRole("admin-user", "admin")

	// Add role with specific permissions
	devRole := &Role{
		Name:        "developer",
		Permissions: []string{"release.view", "release.create"},
	}
	tc.AddRole(devRole)
	tc.AssignRole("dev-user", "developer")

	// Add role with prefix wildcard
	releaseRole := &Role{
		Name:        "release-manager",
		Permissions: []string{"release.*"},
	}
	tc.AddRole(releaseRole)
	tc.AssignRole("release-user", "release-manager")

	tests := []struct {
		actor      string
		permission string
		expected   bool
	}{
		{"admin-user", "release.approve", true},   // wildcard matches
		{"admin-user", "anything.else", true},     // wildcard matches
		{"dev-user", "release.view", true},        // exact match
		{"dev-user", "release.create", true},      // exact match
		{"dev-user", "release.approve", false},    // no match
		{"release-user", "release.approve", true}, // prefix wildcard matches
		{"release-user", "release.publish", true}, // prefix wildcard matches
		{"release-user", "admin.delete", false},   // no match
	}

	for _, tt := range tests {
		t.Run(tt.actor+"_"+tt.permission, func(t *testing.T) {
			result := tc.HasPermission(tt.actor, tt.permission)
			if result != tt.expected {
				t.Errorf("HasPermission(%s, %s) = %v, want %v",
					tt.actor, tt.permission, result, tt.expected)
			}
		})
	}
}

func TestTeamContext_RequiredApprovers(t *testing.T) {
	tc := DefaultTeamContext()

	// Add security role
	secRole := NewSecurityReviewerRole("security", "Security reviewer")
	tc.AddRole(secRole)
	tc.AssignRole("sec-alice", "security")
	tc.AssignRole("sec-bob", "security")

	// Add architect role
	archRole := NewArchitectRole("architect", "Architect")
	tc.AddRole(archRole)
	tc.AssignRole("arch-carol", "architect")

	// Check required approvers for security
	secApprovers := tc.GetRequiredApproversForSecurity()
	if len(secApprovers) != 2 {
		t.Errorf("expected 2 security approvers, got %d", len(secApprovers))
	}

	// Check required approvers for breaking changes
	breakingApprovers := tc.GetRequiredApproversForBreaking()
	if len(breakingApprovers) != 1 {
		t.Errorf("expected 1 breaking change approver, got %d", len(breakingApprovers))
	}
}

func TestTeamContext_ToEvalContext(t *testing.T) {
	tc := DefaultTeamContext()

	tc.AddTeam(NewTeam("platform", "Platform Team").
		WithMembers("alice", "bob").
		WithLeads("carol"))
	tc.AddRole(NewApproverRole("approver", "Approver"))
	tc.AssignRole("alice", "approver")

	ctx := tc.ToEvalContext("alice")

	// Check teams are present
	if _, ok := ctx["teams"]; !ok {
		t.Error("teams should be in context")
	}

	// Check actor-specific fields
	if ctx["canApprove"] != true {
		t.Error("alice should be able to approve")
	}

	actorTeams, ok := ctx["actorTeams"].([]string)
	if !ok {
		t.Fatal("actorTeams should be []string")
	}
	if len(actorTeams) != 1 {
		t.Errorf("alice should be in 1 team, got %d", len(actorTeams))
	}
}

func TestEngine_TeamBasedApproval(t *testing.T) {
	t.Run("require team review", func(t *testing.T) {
		policy := NewPolicy("team-policy")
		policy.AddRule(*NewRule("require-platform-review", "Require platform team review").
			WithPriority(100).
			WithDescription("Platform changes require team review").
			AddCondition("actor.kind", OperatorEqual, "agent").
			AddAction(ActionRequireTeamReview, map[string]any{"team": "platform", "count": float64(2)}))

		engine := NewEngine([]Policy{*policy}, nil)

		// Add platform team
		engine.AddTeam(NewTeam("platform", "Platform Team").
			WithMembers("alice", "bob", "carol").
			WithLeads("diana"))

		proposal := cgp.NewProposal(
			cgp.NewAgentActor("cursor", "Cursor", "gpt-4"),
			cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
			cgp.ProposalIntent{Summary: "Platform change", Confidence: 0.9},
		)

		result, err := engine.Evaluate(context.Background(), proposal, nil, 0.5)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}

		if result.Decision != cgp.DecisionApprovalRequired {
			t.Errorf("Decision = %v, want %v", result.Decision, cgp.DecisionApprovalRequired)
		}

		if result.RequiredApprovers != 2 {
			t.Errorf("RequiredApprovers = %d, want 2", result.RequiredApprovers)
		}

		// Should include all team members as potential reviewers
		if len(result.Reviewers) != 4 {
			t.Errorf("Reviewers count = %d, want 4", len(result.Reviewers))
		}

		// Check required action
		found := false
		for _, action := range result.RequiredActions {
			if action.Type == "team_approval" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected team_approval required action")
		}
	})

	t.Run("require role review", func(t *testing.T) {
		policy := NewPolicy("role-policy")
		policy.AddRule(*NewRule("require-security-review", "Require security review").
			WithPriority(100).
			WithDescription("Security changes require security role review").
			AddCondition("change.security", OperatorGreaterThan, 0).
			AddAction(ActionRequireRoleReview, map[string]any{"role": "security-reviewer"}))

		engine := NewEngine([]Policy{*policy}, nil)

		// Add security role and assign to users
		engine.AddRole(NewSecurityReviewerRole("security-reviewer", "Security Reviewer"))
		engine.AssignActorRole("sec-alice", "security-reviewer")
		engine.AssignActorRole("sec-bob", "security-reviewer")

		proposal := cgp.NewProposal(
			cgp.NewHumanActor("dev@example.com", "Developer"),
			cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
			cgp.ProposalIntent{Summary: "Security fix", Confidence: 0.9},
		)

		analysis := &cgp.ChangeAnalysis{
			Security: 2,
		}

		result, err := engine.Evaluate(context.Background(), proposal, analysis, 0.5)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}

		if result.Decision != cgp.DecisionApprovalRequired {
			t.Errorf("Decision = %v, want %v", result.Decision, cgp.DecisionApprovalRequired)
		}

		// Should include security reviewers
		if len(result.Reviewers) != 2 {
			t.Errorf("Reviewers count = %d, want 2", len(result.Reviewers))
		}

		// Check required action
		found := false
		for _, action := range result.RequiredActions {
			if action.Type == "role_approval" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected role_approval required action")
		}
	})

	t.Run("require team lead approval", func(t *testing.T) {
		policy := NewPolicy("lead-policy")
		policy.AddRule(*NewRule("require-lead-approval", "Require team lead approval").
			WithPriority(100).
			WithDescription("Breaking changes require team lead approval").
			AddCondition("change.breaking", OperatorGreaterThan, 0).
			AddAction(ActionRequireTeamLead, map[string]any{"team": "core"}))

		engine := NewEngine([]Policy{*policy}, nil)

		// Add core team with leads
		engine.AddTeam(NewTeam("core", "Core Team").
			WithMembers("dev1", "dev2").
			WithLeads("lead1", "lead2"))

		proposal := cgp.NewProposal(
			cgp.NewHumanActor("dev@example.com", "Developer"),
			cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
			cgp.ProposalIntent{Summary: "Breaking API change", Confidence: 0.9},
		)

		analysis := &cgp.ChangeAnalysis{
			Breaking: 1,
		}

		result, err := engine.Evaluate(context.Background(), proposal, analysis, 0.7)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}

		if result.Decision != cgp.DecisionApprovalRequired {
			t.Errorf("Decision = %v, want %v", result.Decision, cgp.DecisionApprovalRequired)
		}

		// Should only include team leads, not all members
		if len(result.Reviewers) != 2 {
			t.Errorf("Reviewers count = %d, want 2 (only leads)", len(result.Reviewers))
		}

		// Verify only leads are included
		leadMap := map[string]bool{"lead1": true, "lead2": true}
		for _, r := range result.Reviewers {
			if !leadMap[r] {
				t.Errorf("unexpected reviewer: %s (expected only leads)", r)
			}
		}

		// Check required action
		found := false
		for _, action := range result.RequiredActions {
			if action.Type == "team_lead_approval" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected team_lead_approval required action")
		}
	})

	t.Run("actor team membership in conditions", func(t *testing.T) {
		policy := NewPolicy("membership-policy")
		policy.AddRule(*NewRule("can-approve-check", "Check if actor can approve").
			WithPriority(100).
			WithDescription("Actors with approval permission can approve").
			AddCondition("actor.canApprove", OperatorEqual, true).
			AddAction(ActionSetDecision, map[string]any{"decision": "approve"}))

		engine := NewEngine([]Policy{*policy}, nil)

		// Add approver role and assign to an actor
		// Note: Actor IDs are prefixed with kind (e.g., "human:approver-user")
		engine.AddRole(NewApproverRole("approver", "Can approve"))
		engine.AssignActorRole("human:approver-user", "approver")

		// Test with actor who can approve
		proposal := cgp.NewProposal(
			cgp.NewHumanActor("approver-user", "Approver"),
			cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
			cgp.ProposalIntent{Summary: "Platform update", Confidence: 0.9},
		)

		result, err := engine.Evaluate(context.Background(), proposal, nil, 0.3)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}

		if result.Decision != cgp.DecisionApproved {
			t.Errorf("Decision = %v, want %v (approver should be auto-approved)", result.Decision, cgp.DecisionApproved)
		}

		// Test with actor who cannot approve
		proposal2 := cgp.NewProposal(
			cgp.NewHumanActor("regular-user", "Regular User"),
			cgp.ProposalScope{Repository: "owner/repo", CommitRange: "abc..def"},
			cgp.ProposalIntent{Summary: "Platform update", Confidence: 0.9},
		)

		result2, err := engine.Evaluate(context.Background(), proposal2, nil, 0.3)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}

		// Should not auto-approve for user without approval role
		if result2.Decision == cgp.DecisionApproved && len(result2.MatchedRules) > 0 {
			t.Error("non-approver should not match the auto-approve rule")
		}
	})
}

func TestNewTeam(t *testing.T) {
	team := NewTeam("engineering", "Engineering Team").
		WithMembers("alice", "bob").
		WithLeads("carol").
		WithPermissions("release.view", "release.create").
		WithParent("company")

	if team.Name != "engineering" {
		t.Errorf("Name = %s, want engineering", team.Name)
	}

	if len(team.Members) != 2 {
		t.Errorf("Members count = %d, want 2", len(team.Members))
	}

	if len(team.Leads) != 1 {
		t.Errorf("Leads count = %d, want 1", len(team.Leads))
	}

	if len(team.Permissions) != 2 {
		t.Errorf("Permissions count = %d, want 2", len(team.Permissions))
	}

	if team.ParentTeam != "company" {
		t.Errorf("ParentTeam = %s, want company", team.ParentTeam)
	}
}

func TestRoleHelpers(t *testing.T) {
	t.Run("NewApproverRole", func(t *testing.T) {
		role := NewApproverRole("approver", "Can approve releases")
		if !role.CanApprove {
			t.Error("approver role should be able to approve")
		}
		if role.CanPublish {
			t.Error("approver role should not be able to publish")
		}
	})

	t.Run("NewPublisherRole", func(t *testing.T) {
		role := NewPublisherRole("publisher", "Can publish releases")
		if !role.CanApprove {
			t.Error("publisher role should be able to approve")
		}
		if !role.CanPublish {
			t.Error("publisher role should be able to publish")
		}
	})

	t.Run("NewSecurityReviewerRole", func(t *testing.T) {
		role := NewSecurityReviewerRole("security", "Security reviewer")
		if !role.RequiredForSecurity {
			t.Error("security role should be required for security")
		}
	})

	t.Run("NewArchitectRole", func(t *testing.T) {
		role := NewArchitectRole("architect", "Architect")
		if !role.RequiredForBreaking {
			t.Error("architect role should be required for breaking changes")
		}
	})
}
