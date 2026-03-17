package cgp

import (
	"fmt"
	"strings"
)

// ActorKind represents the type of actor in CGP.
type ActorKind string

// Actor kinds supported by CGP.
const (
	ActorKindAgent  ActorKind = "agent"  // AI coding agents (Claude, Cursor, etc.)
	ActorKindCI     ActorKind = "ci"     // CI/CD systems (GitHub Actions, GitLab CI, etc.)
	ActorKindHuman  ActorKind = "human"  // Human developers
	ActorKindSystem ActorKind = "system" // Automated systems (relicta itself, webhooks, etc.)
)

// AllActorKinds returns all supported actor kinds.
func AllActorKinds() []ActorKind {
	return []ActorKind{
		ActorKindAgent,
		ActorKindCI,
		ActorKindHuman,
		ActorKindSystem,
	}
}

// String returns the string representation of the actor kind.
func (k ActorKind) String() string {
	return string(k)
}

// IsValid returns true if the actor kind is recognized.
func (k ActorKind) IsValid() bool {
	switch k {
	case ActorKindAgent, ActorKindCI, ActorKindHuman, ActorKindSystem:
		return true
	default:
		return false
	}
}

// Description returns a human-readable description of the actor kind.
func (k ActorKind) Description() string {
	switch k {
	case ActorKindAgent:
		return "AI coding agent"
	case ActorKindCI:
		return "CI/CD system"
	case ActorKindHuman:
		return "Human developer"
	case ActorKindSystem:
		return "Automated system"
	default:
		return "Unknown actor kind"
	}
}

// ParseActorKind parses a string into an ActorKind.
// Returns the actor kind and true if valid, or empty string and false if invalid.
func ParseActorKind(s string) (ActorKind, bool) {
	k := ActorKind(strings.ToLower(strings.TrimSpace(s)))
	if k.IsValid() {
		return k, true
	}
	return "", false
}

// TrustLevel determines how much autonomy an actor has.
type TrustLevel int

// Trust levels from least to most trusted.
const (
	TrustLevelUntrusted TrustLevel = iota // Requires full human review
	TrustLevelLimited                     // Can propose, limited auto-approval
	TrustLevelTrusted                     // Can auto-approve low-risk changes
	TrustLevelFull                        // Full automation (human equivalent)
)

// String returns the string representation of the trust level.
func (l TrustLevel) String() string {
	switch l {
	case TrustLevelUntrusted:
		return "untrusted"
	case TrustLevelLimited:
		return "limited"
	case TrustLevelTrusted:
		return "trusted"
	case TrustLevelFull:
		return "full"
	default:
		return "unknown"
	}
}

// CanAutoApprove returns true if this trust level allows auto-approval.
func (l TrustLevel) CanAutoApprove() bool {
	return l >= TrustLevelTrusted
}

// CanPropose returns true if this trust level allows proposing changes.
func (l TrustLevel) CanPropose() bool {
	return l >= TrustLevelLimited
}

// Actor identifies who is proposing or authorizing a change.
type Actor struct {
	// Kind is the type of actor (agent, ci, human, system).
	Kind ActorKind `json:"kind"`

	// ID is a unique identifier for this actor.
	// Format depends on kind: "agent:cursor", "ci:github-actions", "human:john@example.com".
	ID string `json:"id"`

	// Name is an optional human-readable name for the actor.
	Name string `json:"name,omitempty"`

	// TrustLevel determines how much autonomy this actor has.
	TrustLevel TrustLevel `json:"trustLevel,omitempty"`

	// Attributes contains additional actor-specific metadata.
	// Examples: "model" for agents, "workflow" for CI, "email" for humans.
	Attributes map[string]string `json:"attributes,omitempty"`

	// Credentials holds authentication information (never stored in plain text).
	Credentials *Credentials `json:"credentials,omitempty"`
}

// Credentials for actor authentication.
type Credentials struct {
	// Type is the credential type: "token", "certificate", "oauth".
	Type string `json:"type"`

	// TokenHash is the SHA256 hash of the credential (never store raw tokens).
	TokenHash string `json:"tokenHash,omitempty"`

	// ExpiresAt is the optional expiration timestamp in RFC3339 format.
	ExpiresAt string `json:"expiresAt,omitempty"`
}

// NewActor creates a new actor with the given kind and ID.
func NewActor(kind ActorKind, id string) Actor {
	return Actor{
		Kind: kind,
		ID:   id,
	}
}

// NewAgentActor creates an actor representing an AI agent.
func NewAgentActor(agentID string, agentName string, model string) Actor {
	attrs := make(map[string]string)
	if model != "" {
		attrs["model"] = model
	}
	return Actor{
		Kind:       ActorKindAgent,
		ID:         fmt.Sprintf("agent:%s", agentID),
		Name:       agentName,
		Attributes: attrs,
	}
}

// NewCIActor creates an actor representing a CI/CD system.
func NewCIActor(ciSystem string, workflow string, runID string) Actor {
	attrs := make(map[string]string)
	if workflow != "" {
		attrs["workflow"] = workflow
	}
	if runID != "" {
		attrs["runId"] = runID
	}
	return Actor{
		Kind:       ActorKindCI,
		ID:         fmt.Sprintf("ci:%s", ciSystem),
		Name:       ciSystem,
		Attributes: attrs,
	}
}

// NewHumanActor creates an actor representing a human developer.
func NewHumanActor(email string, name string) Actor {
	attrs := make(map[string]string)
	if email != "" {
		attrs["email"] = email
	}
	return Actor{
		Kind:       ActorKindHuman,
		ID:         fmt.Sprintf("human:%s", email),
		Name:       name,
		Attributes: attrs,
	}
}

// NewSystemActor creates an actor representing an automated system.
func NewSystemActor(systemID string, name string) Actor {
	return Actor{
		Kind: ActorKindSystem,
		ID:   fmt.Sprintf("system:%s", systemID),
		Name: name,
	}
}

// Validate checks if the actor is valid.
func (a Actor) Validate() error {
	if !a.Kind.IsValid() {
		return fmt.Errorf("invalid actor kind: %s", a.Kind)
	}
	if a.ID == "" {
		return fmt.Errorf("actor ID is required")
	}
	return nil
}

// IsAgent returns true if this actor is an AI agent.
func (a Actor) IsAgent() bool {
	return a.Kind == ActorKindAgent
}

// IsCI returns true if this actor is a CI/CD system.
func (a Actor) IsCI() bool {
	return a.Kind == ActorKindCI
}

// IsHuman returns true if this actor is a human.
func (a Actor) IsHuman() bool {
	return a.Kind == ActorKindHuman
}

// IsSystem returns true if this actor is an automated system.
func (a Actor) IsSystem() bool {
	return a.Kind == ActorKindSystem
}

// RequiresHumanReview returns true if changes from this actor kind
// typically require human review.
func (a Actor) RequiresHumanReview() bool {
	return a.Kind == ActorKindAgent || a.Kind == ActorKindSystem
}

// String returns a string representation of the actor.
func (a Actor) String() string {
	if a.Name != "" {
		return fmt.Sprintf("%s (%s)", a.Name, a.ID)
	}
	return a.ID
}
