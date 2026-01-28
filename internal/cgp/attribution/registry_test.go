package attribution

import (
	"testing"

	"github.com/relicta-tech/relicta/internal/cgp"
)

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	actor := cgp.NewAgentActor("claude", "Claude Code", "claude-3")
	config := NewAgentConfig(actor).
		WithCapabilities(CapabilityPropose, CapabilityApproveLowRisk).
		WithMaxAutoApproveRisk(0.3).
		Build()

	err := r.Register(config)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if r.Len() != 1 {
		t.Errorf("Len() = %d, want 1", r.Len())
	}

	// Should not allow duplicate
	err = r.Register(config)
	if err != ErrAgentExists {
		t.Errorf("Expected ErrAgentExists, got %v", err)
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()

	actor := cgp.NewAgentActor("claude", "Claude Code", "claude-3")
	config := NewAgentConfig(actor).Build()
	r.Register(config)

	got, err := r.Get(actor.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Actor.ID != actor.ID {
		t.Errorf("Actor.ID = %s, want %s", got.Actor.ID, actor.ID)
	}

	// Non-existent
	_, err = r.Get("nonexistent")
	if err != ErrAgentNotFound {
		t.Errorf("Expected ErrAgentNotFound, got %v", err)
	}
}

func TestRegistry_Update(t *testing.T) {
	r := NewRegistry()

	actor := cgp.NewAgentActor("claude", "Claude Code", "claude-3")
	config := NewAgentConfig(actor).WithMaxAutoApproveRisk(0.3).Build()
	r.Register(config)

	err := r.Update(actor.ID, func(c *AgentConfig) {
		c.MaxAutoApproveRisk = 0.5
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, _ := r.Get(actor.ID)
	if got.MaxAutoApproveRisk != 0.5 {
		t.Errorf("MaxAutoApproveRisk = %f, want 0.5", got.MaxAutoApproveRisk)
	}

	// Update non-existent
	err = r.Update("nonexistent", func(c *AgentConfig) {})
	if err != ErrAgentNotFound {
		t.Errorf("Expected ErrAgentNotFound, got %v", err)
	}
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry()

	actor := cgp.NewAgentActor("claude", "Claude Code", "claude-3")
	config := NewAgentConfig(actor).Build()
	r.Register(config)

	err := r.Remove(actor.ID)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if r.Len() != 0 {
		t.Errorf("Len() = %d after remove, want 0", r.Len())
	}

	// Remove non-existent
	err = r.Remove(actor.ID)
	if err != ErrAgentNotFound {
		t.Errorf("Expected ErrAgentNotFound, got %v", err)
	}
}

func TestRegistry_EnableDisable(t *testing.T) {
	r := NewRegistry()

	actor := cgp.NewAgentActor("claude", "Claude Code", "claude-3")
	config := NewAgentConfig(actor).Build()
	r.Register(config)

	// Initially enabled
	got, _ := r.Get(actor.ID)
	if !got.Enabled {
		t.Error("Agent should be enabled by default")
	}

	// Disable
	r.Disable(actor.ID)
	got, _ = r.Get(actor.ID)
	if got.Enabled {
		t.Error("Agent should be disabled")
	}

	// Re-enable
	r.Enable(actor.ID)
	got, _ = r.Get(actor.ID)
	if !got.Enabled {
		t.Error("Agent should be enabled again")
	}
}

func TestRegistry_RecordActivity(t *testing.T) {
	r := NewRegistry()

	actor := cgp.NewAgentActor("claude", "Claude Code", "claude-3")
	config := NewAgentConfig(actor).Build()
	r.Register(config)

	// Initially no activity
	got, _ := r.Get(actor.ID)
	if got.LastSeenAt != nil {
		t.Error("LastSeenAt should be nil initially")
	}

	// Record activity
	r.RecordActivity(actor.ID)
	got, _ = r.Get(actor.ID)
	if got.LastSeenAt == nil {
		t.Error("LastSeenAt should not be nil after activity")
	}
}

func TestRegistry_HasCapability(t *testing.T) {
	r := NewRegistry()

	actor := cgp.NewAgentActor("claude", "Claude Code", "claude-3")
	config := NewAgentConfig(actor).
		WithCapabilities(CapabilityPropose, CapabilityApproveAll).
		Build()
	r.Register(config)

	// Direct capability
	if !r.HasCapability(actor.ID, CapabilityPropose) {
		t.Error("Should have Propose capability")
	}

	// Implied capability (ApproveAll implies Approve)
	if !r.HasCapability(actor.ID, CapabilityApprove) {
		t.Error("ApproveAll should imply Approve capability")
	}

	// Missing capability
	if r.HasCapability(actor.ID, CapabilityExecute) {
		t.Error("Should not have Execute capability")
	}

	// Non-existent agent
	if r.HasCapability("nonexistent", CapabilityPropose) {
		t.Error("Non-existent agent should not have any capability")
	}
}

func TestRegistry_CanApprove(t *testing.T) {
	r := NewRegistry()

	actor := cgp.NewAgentActor("claude", "Claude Code", "claude-3")
	config := NewAgentConfig(actor).
		WithCapabilities(CapabilityApprove).
		WithMaxAutoApproveRisk(0.3).
		Build()
	r.Register(config)

	// Within threshold
	if !r.CanApprove(actor.ID, 0.2) {
		t.Error("Should be able to approve risk 0.2")
	}

	// At threshold
	if !r.CanApprove(actor.ID, 0.3) {
		t.Error("Should be able to approve risk 0.3")
	}

	// Above threshold
	if r.CanApprove(actor.ID, 0.5) {
		t.Error("Should not be able to approve risk 0.5")
	}
}

func TestRegistry_CanApproveDisabled(t *testing.T) {
	r := NewRegistry()

	actor := cgp.NewAgentActor("claude", "Claude Code", "claude-3")
	config := NewAgentConfig(actor).
		WithCapabilities(CapabilityApprove).
		WithMaxAutoApproveRisk(0.5).
		Build()
	r.Register(config)
	r.Disable(actor.ID)

	// Disabled agent cannot approve
	if r.CanApprove(actor.ID, 0.1) {
		t.Error("Disabled agent should not be able to approve")
	}
}

func TestRegistry_CanAccessRepository(t *testing.T) {
	r := NewRegistry()

	// Agent with restrictions
	restrictedActor := cgp.NewAgentActor("restricted", "Restricted Agent", "")
	restrictedConfig := NewAgentConfig(restrictedActor).
		WithAllowedRepositories("org/repo1", "org/repo2").
		Build()
	r.Register(restrictedConfig)

	// Agent without restrictions
	unrestrictedActor := cgp.NewAgentActor("unrestricted", "Unrestricted Agent", "")
	unrestrictedConfig := NewAgentConfig(unrestrictedActor).Build()
	r.Register(unrestrictedConfig)

	// Restricted agent - allowed repos
	if !r.CanAccessRepository(restrictedActor.ID, "org/repo1") {
		t.Error("Should access org/repo1")
	}
	if !r.CanAccessRepository(restrictedActor.ID, "org/repo2") {
		t.Error("Should access org/repo2")
	}

	// Restricted agent - disallowed repo
	if r.CanAccessRepository(restrictedActor.ID, "org/repo3") {
		t.Error("Should not access org/repo3")
	}

	// Unrestricted agent - any repo
	if !r.CanAccessRepository(unrestrictedActor.ID, "any/repo") {
		t.Error("Unrestricted agent should access any repo")
	}
}

func TestRegistry_GetByKind(t *testing.T) {
	r := NewRegistry()

	agent1 := cgp.NewAgentActor("agent1", "Agent 1", "model1")
	agent2 := cgp.NewAgentActor("agent2", "Agent 2", "model2")
	human := cgp.NewHumanActor("user@example.com", "User")
	ci := cgp.NewCIActor("github", "release", "123")

	r.Register(NewAgentConfig(agent1).Build())
	r.Register(NewAgentConfig(agent2).Build())
	r.Register(NewAgentConfig(human).Build())
	r.Register(NewAgentConfig(ci).Build())

	agents := r.GetByKind(cgp.ActorKindAgent)
	if len(agents) != 2 {
		t.Errorf("GetByKind(Agent) = %d, want 2", len(agents))
	}

	humans := r.GetByKind(cgp.ActorKindHuman)
	if len(humans) != 1 {
		t.Errorf("GetByKind(Human) = %d, want 1", len(humans))
	}

	cis := r.GetByKind(cgp.ActorKindCI)
	if len(cis) != 1 {
		t.Errorf("GetByKind(CI) = %d, want 1", len(cis))
	}
}

func TestRegistry_ListEnabled(t *testing.T) {
	r := NewRegistry()

	agent1 := cgp.NewAgentActor("agent1", "Agent 1", "model1")
	agent2 := cgp.NewAgentActor("agent2", "Agent 2", "model2")

	r.Register(NewAgentConfig(agent1).Build())
	r.Register(NewAgentConfig(agent2).Disabled().Build())

	enabled := r.ListEnabled()
	if len(enabled) != 1 {
		t.Errorf("ListEnabled() = %d, want 1", len(enabled))
	}
}

func TestRegistry_GetRiskMultiplier(t *testing.T) {
	r := NewRegistry()

	// Agent with custom multiplier
	highRiskActor := cgp.NewAgentActor("highrisk", "High Risk Agent", "")
	highRiskConfig := NewAgentConfig(highRiskActor).
		WithRiskMultiplier(1.5).
		Build()
	r.Register(highRiskConfig)

	// Agent with default multiplier
	normalActor := cgp.NewAgentActor("normal", "Normal Agent", "")
	normalConfig := NewAgentConfig(normalActor).Build()
	r.Register(normalConfig)

	if m := r.GetRiskMultiplier(highRiskActor.ID); m != 1.5 {
		t.Errorf("High risk multiplier = %f, want 1.5", m)
	}

	if m := r.GetRiskMultiplier(normalActor.ID); m != 1.0 {
		t.Errorf("Normal multiplier = %f, want 1.0", m)
	}

	// Non-existent agent defaults to 1.0
	if m := r.GetRiskMultiplier("nonexistent"); m != 1.0 {
		t.Errorf("Non-existent multiplier = %f, want 1.0", m)
	}
}

func TestAgentConfigBuilder(t *testing.T) {
	actor := cgp.NewAgentActor("claude", "Claude Code", "claude-3")

	config := NewAgentConfig(actor).
		WithCapabilities(CapabilityPropose, CapabilityApprove).
		WithRiskMultiplier(1.2).
		WithMaxAutoApproveRisk(0.25).
		WithRequiredReviewers("senior@example.com").
		WithAllowedRepositories("org/repo1").
		WithAllowedBranches("main", "develop").
		WithMetadata("version", "1.0").
		Disabled().
		Build()

	if config.Actor.ID != actor.ID {
		t.Errorf("Actor.ID = %s, want %s", config.Actor.ID, actor.ID)
	}
	if !config.Capabilities.Has(CapabilityPropose) {
		t.Error("Should have Propose capability")
	}
	if config.RiskMultiplier != 1.2 {
		t.Errorf("RiskMultiplier = %f, want 1.2", config.RiskMultiplier)
	}
	if config.MaxAutoApproveRisk != 0.25 {
		t.Errorf("MaxAutoApproveRisk = %f, want 0.25", config.MaxAutoApproveRisk)
	}
	if len(config.RequiredReviewers) != 1 {
		t.Errorf("RequiredReviewers length = %d, want 1", len(config.RequiredReviewers))
	}
	if len(config.AllowedRepositories) != 1 {
		t.Errorf("AllowedRepositories length = %d, want 1", len(config.AllowedRepositories))
	}
	if len(config.AllowedBranches) != 2 {
		t.Errorf("AllowedBranches length = %d, want 2", len(config.AllowedBranches))
	}
	if config.Metadata["version"] != "1.0" {
		t.Errorf("Metadata[version] = %v, want 1.0", config.Metadata["version"])
	}
	if config.Enabled {
		t.Error("Config should be disabled")
	}
}

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(NewAgentConfig(cgp.NewAgentActor("list-1", "Agent One", "model-1")).Build())
	_ = reg.Register(NewAgentConfig(cgp.NewAgentActor("list-2", "Agent Two", "model-2")).Build())

	all := reg.List()
	if len(all) != 2 {
		t.Errorf("List() returned %d agents, want 2", len(all))
	}
}
