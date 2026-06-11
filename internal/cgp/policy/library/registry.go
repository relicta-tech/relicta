// Package library provides built-in policy templates and a registry for managing policies.
package library

import (
	"fmt"
	"sync"

	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
)

// PolicyTemplate represents a pre-built policy template.
type PolicyTemplate struct {
	// ID is the unique identifier for this template.
	ID string

	// Name is the human-readable name.
	Name string

	// Description explains the template's purpose and use case.
	Description string

	// Category groups related templates (security, stability, speed, enterprise).
	Category string

	// Tags for searchability.
	Tags []string

	// Build creates a configured policy from this template.
	Build func(opts TemplateOptions) *policy.Policy
}

// TemplateOptions configures policy template generation.
type TemplateOptions struct {
	// PolicyName overrides the default policy name.
	PolicyName string

	// RiskThreshold sets the risk score threshold (0.0-1.0).
	RiskThreshold float64

	// RequiredApprovers sets the default number of required approvers.
	RequiredApprovers int

	// AllowedActors limits who can propose changes.
	AllowedActors []string

	// BlockedBranches specifies branches that require extra scrutiny.
	BlockedBranches []string

	// ProductionBranches identifies production branches.
	ProductionBranches []string

	// SecurityTeam is the team name for security reviews.
	SecurityTeam string

	// LeadTeam is the team name for lead approvals.
	LeadTeam string

	// FreezeDuringHolidays enables freeze period blocking.
	FreezeDuringHolidays bool

	// RequireBusinessHours restricts releases to business hours.
	RequireBusinessHours bool

	// MaxFilesWithoutReview limits files changed before requiring review.
	MaxFilesWithoutReview int

	// MaxLinesWithoutReview limits lines changed before requiring review.
	MaxLinesWithoutReview int

	// Custom allows template-specific custom options.
	Custom map[string]any
}

// DefaultTemplateOptions returns sensible defaults for template options.
func DefaultTemplateOptions() TemplateOptions {
	return TemplateOptions{
		RiskThreshold:         0.7,
		RequiredApprovers:     1,
		ProductionBranches:    []string{"main", "master", "production"},
		MaxFilesWithoutReview: 10,
		MaxLinesWithoutReview: 500,
	}
}

// Registry manages available policy templates.
type Registry struct {
	mu         sync.RWMutex
	templates  map[string]*PolicyTemplate
	byCategory map[string][]*PolicyTemplate
}

// NewRegistry creates a new policy registry.
func NewRegistry() *Registry {
	return &Registry{
		templates:  make(map[string]*PolicyTemplate),
		byCategory: make(map[string][]*PolicyTemplate),
	}
}

// Register adds a template to the registry.
func (r *Registry) Register(t *PolicyTemplate) error {
	if t.ID == "" {
		return fmt.Errorf("template ID is required")
	}
	if t.Build == nil {
		return fmt.Errorf("template Build function is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.templates[t.ID]; exists {
		return fmt.Errorf("template %s already registered", t.ID)
	}

	r.templates[t.ID] = t
	r.byCategory[t.Category] = append(r.byCategory[t.Category], t)
	return nil
}

// Get retrieves a template by ID.
func (r *Registry) Get(id string) (*PolicyTemplate, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.templates[id]
	return t, ok
}

// List returns all registered templates.
func (r *Registry) List() []*PolicyTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*PolicyTemplate, 0, len(r.templates))
	for _, t := range r.templates {
		result = append(result, t)
	}
	return result
}

// ListByCategory returns templates in a specific category.
func (r *Registry) ListByCategory(category string) []*PolicyTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()

	templates := r.byCategory[category]
	result := make([]*PolicyTemplate, len(templates))
	copy(result, templates)
	return result
}

// Categories returns all available categories.
func (r *Registry) Categories() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, 0, len(r.byCategory))
	for cat := range r.byCategory {
		result = append(result, cat)
	}
	return result
}

// Build creates a policy from a template with the given options.
func (r *Registry) Build(templateID string, opts TemplateOptions) (*policy.Policy, error) {
	t, ok := r.Get(templateID)
	if !ok {
		return nil, fmt.Errorf("template %s not found", templateID)
	}
	return t.Build(opts), nil
}

// BuildAll creates policies from all templates in a category.
func (r *Registry) BuildAll(category string, opts TemplateOptions) []*policy.Policy {
	templates := r.ListByCategory(category)
	policies := make([]*policy.Policy, 0, len(templates))
	for _, t := range templates {
		policies = append(policies, t.Build(opts))
	}
	return policies
}

// DefaultRegistry is the global registry with built-in templates.
var DefaultRegistry = NewRegistry()

// mustRegister registers a template and panics on error.
// Used for built-in templates where registration failure is a programmer error.
func mustRegister(r *Registry, t *PolicyTemplate) {
	if err := r.Register(t); err != nil {
		panic(fmt.Sprintf("failed to register template %s: %v", t.ID, err))
	}
}

// Category constants for organizing templates.
const (
	CategorySecurity   = "security"
	CategoryStability  = "stability"
	CategorySpeed      = "speed"
	CategoryEnterprise = "enterprise"
	CategoryCompliance = "compliance"
)

// RegisterBuiltins registers all built-in templates to a registry.
func RegisterBuiltins(r *Registry) {
	// Security templates
	registerSecurityTemplates(r)

	// Stability templates
	registerStabilityTemplates(r)

	// Speed templates
	registerSpeedTemplates(r)

	// Enterprise templates
	registerEnterpriseTemplates(r)
}

func init() {
	RegisterBuiltins(DefaultRegistry)
}
