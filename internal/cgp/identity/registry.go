// Package identity provides an organization-level actor identity registry
// for the Change Governance Protocol (CGP). It manages actor identities,
// trust aggregation, and capability-based access control across teams.
package identity

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
)

// ActorIdentity represents an org-level actor with earned trust.
type ActorIdentity struct {
	// ID is the unique identifier in "name@scope" format,
	// e.g. "claude-code@team-platform".
	ID string `json:"id"`

	// Kind is the type of actor (agent, ci, human, system).
	Kind cgp.ActorKind `json:"kind"`

	// Organization is the organization identifier.
	Organization string `json:"organization"`

	// Team is the team within the organization.
	Team string `json:"team"`

	// TrustScore is the aggregated trust score (0.0-1.0).
	TrustScore float64 `json:"trustScore"`

	// Capabilities defines what this actor is permitted to do.
	Capabilities []Capability `json:"capabilities"`

	// Metadata contains custom attributes for the actor.
	Metadata map[string]string `json:"metadata,omitempty"`

	// CreatedAt is when this identity was first registered.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when this identity was last modified.
	UpdatedAt time.Time `json:"updatedAt"`
}

// Capability defines what an actor is permitted to do.
type Capability struct {
	// Action is the operation: "plan", "bump", "notes", "approve", "publish".
	Action string `json:"action"`

	// Scope constrains the action: "patch", "minor", "major", "all".
	Scope string `json:"scope"`

	// Condition is an optional guard expression,
	// e.g. "risk_score < 0.3".
	Condition string `json:"condition,omitempty"`
}

// RegistryStore persists actor identities.
type RegistryStore interface {
	// LoadAll loads all actor identities from the store.
	LoadAll(ctx context.Context) ([]*ActorIdentity, error)

	// Save persists an actor identity.
	Save(ctx context.Context, actor *ActorIdentity) error

	// Delete removes an actor identity by ID.
	Delete(ctx context.Context, id string) error
}

// Registry manages actor identities and trust aggregation.
type Registry struct {
	mu     sync.RWMutex
	actors map[string]*ActorIdentity
	store  RegistryStore
	logger *slog.Logger
}

// RegistryOption configures the Registry.
type RegistryOption func(*Registry)

// WithLogger sets a custom logger for the registry.
func WithLogger(logger *slog.Logger) RegistryOption {
	return func(r *Registry) {
		r.logger = logger
	}
}

// NewRegistry creates a new actor identity registry backed by the given store.
// It loads all existing identities from the store on creation.
func NewRegistry(store RegistryStore, opts ...RegistryOption) (*Registry, error) {
	if store == nil {
		return nil, fmt.Errorf("registry store is required")
	}

	r := &Registry{
		actors: make(map[string]*ActorIdentity),
		store:  store,
		logger: slog.Default(),
	}

	for _, opt := range opts {
		opt(r)
	}

	// Load existing identities from store.
	ctx := context.Background()
	actors, err := store.LoadAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading identities from store: %w", err)
	}

	for _, a := range actors {
		r.actors[a.ID] = a
	}

	r.logger.Info("identity registry initialized", "actor_count", len(r.actors))
	return r, nil
}

// Register adds or updates an actor identity in the registry.
func (r *Registry) Register(ctx context.Context, identity *ActorIdentity) error {
	if identity == nil {
		return fmt.Errorf("identity is required")
	}
	if err := validateIdentity(identity); err != nil {
		return fmt.Errorf("invalid identity: %w", err)
	}

	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.actors[identity.ID]
	if exists {
		// Preserve creation timestamp on update.
		identity.CreatedAt = existing.CreatedAt
	} else {
		identity.CreatedAt = now
	}
	identity.UpdatedAt = now

	if err := r.store.Save(ctx, identity); err != nil {
		return fmt.Errorf("saving identity: %w", err)
	}

	r.actors[identity.ID] = identity

	r.logger.Info("actor registered",
		"id", identity.ID,
		"kind", identity.Kind,
		"team", identity.Team,
		"updated", exists,
	)

	return nil
}

// Get returns an actor identity by ID.
func (r *Registry) Get(ctx context.Context, id string) (*ActorIdentity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	actor, exists := r.actors[id]
	if !exists {
		return nil, fmt.Errorf("actor not found: %s", id)
	}

	// Return a copy to prevent mutation.
	clone := *actor
	clone.Capabilities = make([]Capability, len(actor.Capabilities))
	copy(clone.Capabilities, actor.Capabilities)
	if actor.Metadata != nil {
		clone.Metadata = make(map[string]string, len(actor.Metadata))
		for k, v := range actor.Metadata {
			clone.Metadata[k] = v
		}
	}

	return &clone, nil
}

// GetByTeam returns all actor identities belonging to a team.
func (r *Registry) GetByTeam(ctx context.Context, team string) ([]*ActorIdentity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*ActorIdentity
	for _, actor := range r.actors {
		if actor.Team == team {
			clone := *actor
			result = append(result, &clone)
		}
	}

	return result, nil
}

// UpdateTrust recalculates trust for an actor from their metrics.
// It uses the same reliability formula as memory.ActorMetrics.CalculateReliabilityScore
// with additional recency weighting: recent releases carry more weight.
func (r *Registry) UpdateTrust(ctx context.Context, id string, metrics memory.ActorMetrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	actor, exists := r.actors[id]
	if !exists {
		return fmt.Errorf("actor not found: %s", id)
	}

	actor.TrustScore = calculateTrustFromMetrics(&metrics)
	actor.UpdatedAt = time.Now()

	if err := r.store.Save(ctx, actor); err != nil {
		return fmt.Errorf("saving updated trust: %w", err)
	}

	r.logger.Info("trust updated",
		"id", id,
		"trust_score", actor.TrustScore,
		"total_releases", metrics.TotalReleases,
	)

	return nil
}

// CheckCapability evaluates whether an actor can perform the requested action
// at the given scope, considering the current risk score. It returns whether the
// action is allowed and a human-readable reason.
func (r *Registry) CheckCapability(ctx context.Context, id, action, scope string, riskScore float64) (bool, string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	actor, exists := r.actors[id]
	if !exists {
		return false, fmt.Sprintf("actor not found: %s", id)
	}

	if len(actor.Capabilities) == 0 {
		return false, fmt.Sprintf("actor %s has no capabilities defined", id)
	}

	for _, cap := range actor.Capabilities {
		if !matchAction(cap.Action, action) {
			continue
		}

		if !matchScope(cap.Scope, scope) {
			continue
		}

		if cap.Condition != "" {
			allowed, err := evaluateCondition(cap.Condition, riskScore)
			if err != nil {
				return false, fmt.Sprintf("condition evaluation error: %v", err)
			}
			if !allowed {
				return false, fmt.Sprintf("condition not met: %s (risk_score=%.2f)", cap.Condition, riskScore)
			}
		}

		return true, fmt.Sprintf("capability granted: %s:%s", cap.Action, cap.Scope)
	}

	return false, fmt.Sprintf("no matching capability for action=%s scope=%s", action, scope)
}

// List returns all registered actor identities.
func (r *Registry) List(ctx context.Context) ([]*ActorIdentity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ActorIdentity, 0, len(r.actors))
	for _, actor := range r.actors {
		clone := *actor
		result = append(result, &clone)
	}

	return result, nil
}

// calculateTrustFromMetrics computes a trust score from actor metrics.
// It mirrors memory.ActorMetrics.CalculateReliabilityScore but adds
// recency weighting so that recent release outcomes matter more.
func calculateTrustFromMetrics(m *memory.ActorMetrics) float64 {
	if m.TotalReleases == 0 {
		return 0.5 // Neutral for unknown actors.
	}

	// Base reliability score using the same formula as memory package.
	baseScore := m.CalculateReliabilityScore()

	// Apply recency weighting: if the last release was recent, the score
	// has more confidence; if stale, regress toward 0.5 (neutral).
	recencyFactor := 1.0
	if m.LastReleaseAt != nil {
		daysSince := time.Since(*m.LastReleaseAt).Hours() / 24
		// Exponential decay: half-life of 90 days.
		recencyFactor = math.Exp(-0.0077 * daysSince) // ln(2)/90 ~ 0.0077
	}

	// Blend base score with neutral using recency factor.
	neutral := 0.5
	score := neutral + (baseScore-neutral)*recencyFactor

	// Clamp to [0.0, 1.0].
	if score < 0.0 {
		score = 0.0
	}
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// matchAction checks if the capability action matches the requested action.
func matchAction(capAction, requestedAction string) bool {
	return strings.EqualFold(capAction, requestedAction)
}

// matchScope checks if the capability scope covers the requested scope.
// "all" matches everything. The semver hierarchy is: patch < minor < major.
func matchScope(capScope, requestedScope string) bool {
	capScope = strings.ToLower(capScope)
	requestedScope = strings.ToLower(requestedScope)

	if capScope == "all" || capScope == requestedScope {
		return true
	}

	// Scope hierarchy: major > minor > patch.
	scopeOrder := map[string]int{
		"patch": 1,
		"minor": 2,
		"major": 3,
	}

	capLevel, capOk := scopeOrder[capScope]
	reqLevel, reqOk := scopeOrder[requestedScope]

	if !capOk || !reqOk {
		return false
	}

	// A higher scope capability covers lower scope requests.
	return capLevel >= reqLevel
}

// evaluateCondition evaluates a simple numeric condition against risk_score.
// Supported forms: "risk_score < 0.3", "risk_score <= 0.5", "risk_score > 0.7",
// "risk_score >= 0.8".
func evaluateCondition(condition string, riskScore float64) (bool, error) {
	condition = strings.TrimSpace(condition)

	// Parse the condition: expect "risk_score <op> <value>".
	parts := strings.Fields(condition)
	if len(parts) != 3 {
		return false, fmt.Errorf("unsupported condition format: %q", condition)
	}

	variable := strings.ToLower(parts[0])
	operator := parts[1]
	valueStr := parts[2]

	if variable != "risk_score" {
		return false, fmt.Errorf("unsupported variable: %q (only risk_score is supported)", variable)
	}

	threshold, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return false, fmt.Errorf("invalid threshold value %q: %w", valueStr, err)
	}

	switch operator {
	case "<":
		return riskScore < threshold, nil
	case "<=":
		return riskScore <= threshold, nil
	case ">":
		return riskScore > threshold, nil
	case ">=":
		return riskScore >= threshold, nil
	case "==":
		return riskScore == threshold, nil
	case "!=":
		return riskScore != threshold, nil
	default:
		return false, fmt.Errorf("unsupported operator: %q", operator)
	}
}

// validateIdentity checks that an ActorIdentity has all required fields.
func validateIdentity(identity *ActorIdentity) error {
	if identity.ID == "" {
		return fmt.Errorf("identity ID is required")
	}
	if !identity.Kind.IsValid() {
		return fmt.Errorf("invalid actor kind: %s", identity.Kind)
	}
	if identity.Organization == "" {
		return fmt.Errorf("organization is required")
	}
	if identity.TrustScore < 0.0 || identity.TrustScore > 1.0 {
		return fmt.Errorf("trust score must be between 0.0 and 1.0, got %.2f", identity.TrustScore)
	}
	return nil
}
