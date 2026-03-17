package attribution

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
)

// ErrAgentNotFound indicates the agent is not registered.
var ErrAgentNotFound = errors.New("agent not registered")

// ErrAgentExists indicates the agent is already registered.
var ErrAgentExists = errors.New("agent already registered")

// AgentConfig represents the configuration for a registered agent.
type AgentConfig struct {
	// Actor is the base actor information.
	Actor cgp.Actor `json:"actor"`

	// Capabilities defines what the agent can do.
	Capabilities *CapabilitySet `json:"capabilities"`

	// RiskMultiplier adjusts risk scoring for this agent (1.0 = no change).
	RiskMultiplier float64 `json:"riskMultiplier"`

	// MaxAutoApproveRisk is the maximum risk score for auto-approval.
	MaxAutoApproveRisk float64 `json:"maxAutoApproveRisk"`

	// RequiredReviewers specifies reviewers required for this agent's changes.
	RequiredReviewers []string `json:"requiredReviewers,omitempty"`

	// AllowedRepositories restricts which repos the agent can propose to.
	AllowedRepositories []string `json:"allowedRepositories,omitempty"`

	// AllowedBranches restricts which branches the agent can target.
	AllowedBranches []string `json:"allowedBranches,omitempty"`

	// Metadata contains additional agent-specific configuration.
	Metadata map[string]any `json:"metadata,omitempty"`

	// CreatedAt is when the agent was registered.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the configuration was last modified.
	UpdatedAt time.Time `json:"updatedAt"`

	// LastSeenAt is when the agent was last active.
	LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`

	// Enabled indicates if the agent is currently active.
	Enabled bool `json:"enabled"`
}

// Registry manages registered agents and their configurations.
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*AgentConfig
}

// NewRegistry creates a new agent registry.
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]*AgentConfig),
	}
}

// Register adds a new agent to the registry.
func (r *Registry) Register(config *AgentConfig) error {
	if err := config.Actor.Validate(); err != nil {
		return fmt.Errorf("invalid actor: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[config.Actor.ID]; exists {
		return ErrAgentExists
	}

	now := time.Now().UTC()
	config.CreatedAt = now
	config.UpdatedAt = now

	if config.Capabilities == nil {
		config.Capabilities = NewCapabilitySet()
	}

	if config.RiskMultiplier == 0 {
		config.RiskMultiplier = 1.0
	}

	r.agents[config.Actor.ID] = config
	return nil
}

// Update modifies an existing agent configuration.
func (r *Registry) Update(actorID string, fn func(*AgentConfig)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	config, exists := r.agents[actorID]
	if !exists {
		return ErrAgentNotFound
	}

	fn(config)
	config.UpdatedAt = time.Now().UTC()
	return nil
}

// Get retrieves an agent configuration by ID.
func (r *Registry) Get(actorID string) (*AgentConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	config, exists := r.agents[actorID]
	if !exists {
		return nil, ErrAgentNotFound
	}
	return config, nil
}

// GetByKind returns all agents of a specific kind.
func (r *Registry) GetByKind(kind cgp.ActorKind) []*AgentConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*AgentConfig
	for _, config := range r.agents {
		if config.Actor.Kind == kind {
			result = append(result, config)
		}
	}
	return result
}

// List returns all registered agents.
func (r *Registry) List() []*AgentConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*AgentConfig, 0, len(r.agents))
	for _, config := range r.agents {
		result = append(result, config)
	}
	return result
}

// ListEnabled returns all enabled agents.
func (r *Registry) ListEnabled() []*AgentConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*AgentConfig
	for _, config := range r.agents {
		if config.Enabled {
			result = append(result, config)
		}
	}
	return result
}

// Remove unregisters an agent.
func (r *Registry) Remove(actorID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[actorID]; !exists {
		return ErrAgentNotFound
	}

	delete(r.agents, actorID)
	return nil
}

// Enable activates an agent.
func (r *Registry) Enable(actorID string) error {
	return r.Update(actorID, func(config *AgentConfig) {
		config.Enabled = true
	})
}

// Disable deactivates an agent.
func (r *Registry) Disable(actorID string) error {
	return r.Update(actorID, func(config *AgentConfig) {
		config.Enabled = false
	})
}

// RecordActivity updates the last seen timestamp.
func (r *Registry) RecordActivity(actorID string) error {
	return r.Update(actorID, func(config *AgentConfig) {
		now := time.Now().UTC()
		config.LastSeenAt = &now
	})
}

// HasCapability checks if an agent has a specific capability.
func (r *Registry) HasCapability(actorID string, capability Capability) bool {
	config, err := r.Get(actorID)
	if err != nil {
		return false
	}
	return config.Capabilities.Allows(capability)
}

// CanApprove checks if an agent can approve changes at a given risk level.
func (r *Registry) CanApprove(actorID string, riskScore float64) bool {
	config, err := r.Get(actorID)
	if err != nil {
		return false
	}

	if !config.Enabled {
		return false
	}

	if !config.Capabilities.Allows(CapabilityApprove) {
		return false
	}

	return riskScore <= config.MaxAutoApproveRisk
}

// CanAccessRepository checks if an agent is allowed to access a repository.
func (r *Registry) CanAccessRepository(actorID, repository string) bool {
	config, err := r.Get(actorID)
	if err != nil {
		return false
	}

	// If no restrictions, allow all
	if len(config.AllowedRepositories) == 0 {
		return true
	}

	for _, allowed := range config.AllowedRepositories {
		if allowed == repository || allowed == "*" {
			return true
		}
	}
	return false
}

// GetRiskMultiplier returns the risk multiplier for an agent.
func (r *Registry) GetRiskMultiplier(actorID string) float64 {
	config, err := r.Get(actorID)
	if err != nil {
		return 1.0 // Default multiplier
	}
	return config.RiskMultiplier
}

// Len returns the number of registered agents.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}

// AgentConfigBuilder provides a fluent API for creating agent configurations.
type AgentConfigBuilder struct {
	config *AgentConfig
}

// NewAgentConfig creates a new agent configuration builder.
func NewAgentConfig(actor cgp.Actor) *AgentConfigBuilder {
	return &AgentConfigBuilder{
		config: &AgentConfig{
			Actor:          actor,
			Capabilities:   NewCapabilitySet(),
			RiskMultiplier: 1.0,
			Metadata:       make(map[string]any),
			Enabled:        true,
		},
	}
}

// WithCapabilities sets the agent's capabilities.
func (b *AgentConfigBuilder) WithCapabilities(caps ...Capability) *AgentConfigBuilder {
	for _, c := range caps {
		b.config.Capabilities.Add(c)
	}
	return b
}

// WithRiskMultiplier sets the risk multiplier.
func (b *AgentConfigBuilder) WithRiskMultiplier(multiplier float64) *AgentConfigBuilder {
	b.config.RiskMultiplier = multiplier
	return b
}

// WithMaxAutoApproveRisk sets the maximum auto-approve risk threshold.
func (b *AgentConfigBuilder) WithMaxAutoApproveRisk(threshold float64) *AgentConfigBuilder {
	b.config.MaxAutoApproveRisk = threshold
	return b
}

// WithRequiredReviewers sets required reviewers.
func (b *AgentConfigBuilder) WithRequiredReviewers(reviewers ...string) *AgentConfigBuilder {
	b.config.RequiredReviewers = reviewers
	return b
}

// WithAllowedRepositories sets allowed repositories.
func (b *AgentConfigBuilder) WithAllowedRepositories(repos ...string) *AgentConfigBuilder {
	b.config.AllowedRepositories = repos
	return b
}

// WithAllowedBranches sets allowed branches.
func (b *AgentConfigBuilder) WithAllowedBranches(branches ...string) *AgentConfigBuilder {
	b.config.AllowedBranches = branches
	return b
}

// WithMetadata adds metadata.
func (b *AgentConfigBuilder) WithMetadata(key string, value any) *AgentConfigBuilder {
	b.config.Metadata[key] = value
	return b
}

// Disabled marks the agent as disabled initially.
func (b *AgentConfigBuilder) Disabled() *AgentConfigBuilder {
	b.config.Enabled = false
	return b
}

// Build returns the constructed configuration.
func (b *AgentConfigBuilder) Build() *AgentConfig {
	return b.config
}
