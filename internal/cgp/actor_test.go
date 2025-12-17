package cgp

import (
	"testing"
)

func TestActorKind_String(t *testing.T) {
	tests := []struct {
		name     string
		kind     ActorKind
		expected string
	}{
		{"agent", ActorKindAgent, "agent"},
		{"ci", ActorKindCI, "ci"},
		{"human", ActorKindHuman, "human"},
		{"system", ActorKindSystem, "system"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.kind.String(); got != tt.expected {
				t.Errorf("ActorKind.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestActorKind_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		kind     ActorKind
		expected bool
	}{
		{"agent is valid", ActorKindAgent, true},
		{"ci is valid", ActorKindCI, true},
		{"human is valid", ActorKindHuman, true},
		{"system is valid", ActorKindSystem, true},
		{"empty is invalid", ActorKind(""), false},
		{"unknown is invalid", ActorKind("robot"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.kind.IsValid(); got != tt.expected {
				t.Errorf("ActorKind.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestActorKind_Description(t *testing.T) {
	tests := []struct {
		name     string
		kind     ActorKind
		expected string
	}{
		{"agent", ActorKindAgent, "AI coding agent"},
		{"ci", ActorKindCI, "CI/CD system"},
		{"human", ActorKindHuman, "Human developer"},
		{"system", ActorKindSystem, "Automated system"},
		{"unknown", ActorKind("unknown"), "Unknown actor kind"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.kind.Description(); got != tt.expected {
				t.Errorf("ActorKind.Description() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseActorKind(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    ActorKind
		expectValid bool
	}{
		{"agent lowercase", "agent", ActorKindAgent, true},
		{"agent uppercase", "AGENT", ActorKindAgent, true},
		{"agent with spaces", "  agent  ", ActorKindAgent, true},
		{"ci", "ci", ActorKindCI, true},
		{"human", "human", ActorKindHuman, true},
		{"system", "system", ActorKindSystem, true},
		{"invalid", "robot", ActorKind(""), false},
		{"empty", "", ActorKind(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, valid := ParseActorKind(tt.input)
			if valid != tt.expectValid {
				t.Errorf("ParseActorKind() valid = %v, want %v", valid, tt.expectValid)
			}
			if valid && got != tt.expected {
				t.Errorf("ParseActorKind() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAllActorKinds(t *testing.T) {
	kinds := AllActorKinds()
	if len(kinds) != 4 {
		t.Errorf("AllActorKinds() returned %d kinds, want 4", len(kinds))
	}

	expected := map[ActorKind]bool{
		ActorKindAgent:  true,
		ActorKindCI:     true,
		ActorKindHuman:  true,
		ActorKindSystem: true,
	}

	for _, kind := range kinds {
		if !expected[kind] {
			t.Errorf("Unexpected actor kind: %v", kind)
		}
	}
}

func TestTrustLevel_String(t *testing.T) {
	tests := []struct {
		name     string
		level    TrustLevel
		expected string
	}{
		{"untrusted", TrustLevelUntrusted, "untrusted"},
		{"limited", TrustLevelLimited, "limited"},
		{"trusted", TrustLevelTrusted, "trusted"},
		{"full", TrustLevelFull, "full"},
		{"unknown", TrustLevel(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("TrustLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTrustLevel_CanAutoApprove(t *testing.T) {
	tests := []struct {
		name     string
		level    TrustLevel
		expected bool
	}{
		{"untrusted cannot auto-approve", TrustLevelUntrusted, false},
		{"limited cannot auto-approve", TrustLevelLimited, false},
		{"trusted can auto-approve", TrustLevelTrusted, true},
		{"full can auto-approve", TrustLevelFull, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.CanAutoApprove(); got != tt.expected {
				t.Errorf("TrustLevel.CanAutoApprove() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTrustLevel_CanPropose(t *testing.T) {
	tests := []struct {
		name     string
		level    TrustLevel
		expected bool
	}{
		{"untrusted cannot propose", TrustLevelUntrusted, false},
		{"limited can propose", TrustLevelLimited, true},
		{"trusted can propose", TrustLevelTrusted, true},
		{"full can propose", TrustLevelFull, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.CanPropose(); got != tt.expected {
				t.Errorf("TrustLevel.CanPropose() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewActor(t *testing.T) {
	actor := NewActor(ActorKindAgent, "test-id")
	if actor.Kind != ActorKindAgent {
		t.Errorf("NewActor().Kind = %v, want %v", actor.Kind, ActorKindAgent)
	}
	if actor.ID != "test-id" {
		t.Errorf("NewActor().ID = %v, want test-id", actor.ID)
	}
}

func TestNewAgentActor(t *testing.T) {
	actor := NewAgentActor("cursor", "Cursor", "gpt-4")
	if actor.Kind != ActorKindAgent {
		t.Errorf("NewAgentActor().Kind = %v, want %v", actor.Kind, ActorKindAgent)
	}
	if actor.ID != "agent:cursor" {
		t.Errorf("NewAgentActor().ID = %v, want agent:cursor", actor.ID)
	}
	if actor.Name != "Cursor" {
		t.Errorf("NewAgentActor().Name = %v, want Cursor", actor.Name)
	}
	if actor.Attributes["model"] != "gpt-4" {
		t.Errorf("NewAgentActor().Attributes[model] = %v, want gpt-4", actor.Attributes["model"])
	}
}

func TestNewCIActor(t *testing.T) {
	actor := NewCIActor("github-actions", "release", "12345")
	if actor.Kind != ActorKindCI {
		t.Errorf("NewCIActor().Kind = %v, want %v", actor.Kind, ActorKindCI)
	}
	if actor.ID != "ci:github-actions" {
		t.Errorf("NewCIActor().ID = %v, want ci:github-actions", actor.ID)
	}
	if actor.Attributes["workflow"] != "release" {
		t.Errorf("NewCIActor().Attributes[workflow] = %v, want release", actor.Attributes["workflow"])
	}
	if actor.Attributes["runId"] != "12345" {
		t.Errorf("NewCIActor().Attributes[runId] = %v, want 12345", actor.Attributes["runId"])
	}
}

func TestNewHumanActor(t *testing.T) {
	actor := NewHumanActor("john@example.com", "John Doe")
	if actor.Kind != ActorKindHuman {
		t.Errorf("NewHumanActor().Kind = %v, want %v", actor.Kind, ActorKindHuman)
	}
	if actor.ID != "human:john@example.com" {
		t.Errorf("NewHumanActor().ID = %v, want human:john@example.com", actor.ID)
	}
	if actor.Name != "John Doe" {
		t.Errorf("NewHumanActor().Name = %v, want John Doe", actor.Name)
	}
}

func TestNewSystemActor(t *testing.T) {
	actor := NewSystemActor("relicta", "Relicta Release Manager")
	if actor.Kind != ActorKindSystem {
		t.Errorf("NewSystemActor().Kind = %v, want %v", actor.Kind, ActorKindSystem)
	}
	if actor.ID != "system:relicta" {
		t.Errorf("NewSystemActor().ID = %v, want system:relicta", actor.ID)
	}
}

func TestActor_Validate(t *testing.T) {
	tests := []struct {
		name      string
		actor     Actor
		expectErr bool
	}{
		{
			name:      "valid actor",
			actor:     NewActor(ActorKindAgent, "test-id"),
			expectErr: false,
		},
		{
			name:      "invalid kind",
			actor:     Actor{Kind: ActorKind("invalid"), ID: "test-id"},
			expectErr: true,
		},
		{
			name:      "empty ID",
			actor:     Actor{Kind: ActorKindAgent, ID: ""},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.actor.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("Actor.Validate() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestActor_TypeChecks(t *testing.T) {
	agent := NewAgentActor("cursor", "Cursor", "gpt-4")
	ci := NewCIActor("github-actions", "", "")
	human := NewHumanActor("john@example.com", "John")
	system := NewSystemActor("relicta", "Relicta")

	if !agent.IsAgent() {
		t.Error("Agent actor should return true for IsAgent()")
	}
	if !ci.IsCI() {
		t.Error("CI actor should return true for IsCI()")
	}
	if !human.IsHuman() {
		t.Error("Human actor should return true for IsHuman()")
	}
	if !system.IsSystem() {
		t.Error("System actor should return true for IsSystem()")
	}
}

func TestActor_RequiresHumanReview(t *testing.T) {
	tests := []struct {
		name     string
		actor    Actor
		expected bool
	}{
		{"agent requires review", NewAgentActor("cursor", "Cursor", ""), true},
		{"ci does not require review", NewCIActor("github-actions", "", ""), false},
		{"human does not require review", NewHumanActor("john@example.com", "John"), false},
		{"system requires review", NewSystemActor("relicta", "Relicta"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.actor.RequiresHumanReview(); got != tt.expected {
				t.Errorf("Actor.RequiresHumanReview() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestActor_String(t *testing.T) {
	tests := []struct {
		name     string
		actor    Actor
		expected string
	}{
		{
			name:     "with name",
			actor:    Actor{Kind: ActorKindAgent, ID: "agent:cursor", Name: "Cursor"},
			expected: "Cursor (agent:cursor)",
		},
		{
			name:     "without name",
			actor:    Actor{Kind: ActorKindAgent, ID: "agent:cursor"},
			expected: "agent:cursor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.actor.String(); got != tt.expected {
				t.Errorf("Actor.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}
