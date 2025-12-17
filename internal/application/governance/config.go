// Package governance provides CGP (Change Governance Protocol) integration for release workflows.
package governance

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/internal/cgp/memory"
	"github.com/relicta-tech/relicta/internal/cgp/policy"
	"github.com/relicta-tech/relicta/internal/config"
)

// NewServiceFromConfig creates a governance service from configuration.
// It sets up the evaluator, policy engine, and optionally the memory store
// based on the provided configuration.
func NewServiceFromConfig(cfg *config.GovernanceConfig, repoPath string, logger *slog.Logger) (*Service, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Create evaluator config from governance config
	evalCfg := evaluator.Config{
		DefaultDecision:         cgp.DecisionApproved,
		AutoApproveThreshold:    cfg.AutoApproveThreshold,
		RequireHumanForBreaking: cfg.RequireHumanForBreaking,
		RequireHumanForSecurity: cfg.RequireHumanForSecurity,
		MaxAutoApproveRisk:      cfg.MaxAutoApproveRisk,
	}

	// Build policies from config
	policies := buildPolicies(cfg.Policies, logger)

	// Create policy engine with policies
	policyEngine := policy.NewEngine(policies, logger)

	// Create evaluator with config and policy engine
	eval := evaluator.New(
		evaluator.WithConfig(evalCfg),
		evaluator.WithPolicyEngine(policyEngine),
		evaluator.WithLogger(logger),
	)

	// Create service with evaluator
	opts := []ServiceOption{
		WithLogger(logger),
	}

	// Set up memory store if enabled
	if cfg.MemoryEnabled {
		memoryPath := cfg.MemoryPath
		if memoryPath == "" {
			memoryPath = ".relicta/governance/memory.json"
		}

		// Make path absolute if relative
		if !filepath.IsAbs(memoryPath) && repoPath != "" {
			memoryPath = filepath.Join(repoPath, memoryPath)
		}

		// Create FileStore for persistence
		store, err := memory.NewFileStore(filepath.Dir(memoryPath))
		if err != nil {
			logger.Warn("failed to create memory store, proceeding without historical tracking",
				"error", err,
				"path", memoryPath,
			)
		} else {
			opts = append(opts, WithMemoryStore(store))
		}
	}

	return NewService(eval, opts...), nil
}

// buildPolicies creates policy.Policy objects from config.
func buildPolicies(policyCfgs []config.GovernancePolicyConfig, logger *slog.Logger) []policy.Policy {
	if len(policyCfgs) == 0 {
		return nil
	}

	// Group rules into a single policy from config
	p := policy.NewPolicy("config")
	p.Description = "Policies loaded from configuration"
	p.Defaults.Decision = policy.DecisionRequireReview

	for _, cfg := range policyCfgs {
		if !cfg.IsPolicyEnabled() {
			logger.Debug("skipping disabled policy", "name", cfg.Name)
			continue
		}

		rule := buildRule(cfg)
		p.AddRule(rule)
	}

	if len(p.Rules) == 0 {
		return nil
	}

	return []policy.Policy{*p}
}

// buildRule creates a policy.Rule from config.
func buildRule(cfg config.GovernancePolicyConfig) policy.Rule {
	rule := policy.Rule{
		ID:          fmt.Sprintf("cfg_%s", cfg.Name),
		Name:        cfg.Name,
		Description: cfg.Description,
		Priority:    cfg.Priority,
		Enabled:     true,
		Conditions:  make([]policy.Condition, 0, len(cfg.Conditions)),
		Actions:     make([]policy.Action, 0),
	}

	// Convert conditions
	for _, condCfg := range cfg.Conditions {
		cond := policy.Condition{
			Field:    condCfg.Field,
			Operator: condCfg.Operator,
			Value:    condCfg.Value,
		}
		rule.Conditions = append(rule.Conditions, cond)
	}

	// Convert action string to policy action
	action := policy.Action{
		Type:   policy.ActionSetDecision,
		Params: map[string]any{},
	}

	switch cfg.Action {
	case "approve":
		action.Params["decision"] = policy.DecisionApprove
	case "deny", "reject":
		action.Params["decision"] = policy.DecisionReject
	case "require_review":
		action.Params["decision"] = policy.DecisionRequireReview
	default:
		// Default to require review
		action.Params["decision"] = policy.DecisionRequireReview
	}

	rule.Actions = append(rule.Actions, action)

	// Add message as rationale if provided
	if cfg.Message != "" {
		rule.Actions = append(rule.Actions, policy.Action{
			Type: policy.ActionAddRationale,
			Params: map[string]any{
				"message": cfg.Message,
			},
		})
	}

	return rule
}

// IsActorTrusted checks if an actor is in the trusted actors list.
func IsActorTrusted(cfg *config.GovernanceConfig, actor cgp.Actor) bool {
	for _, trusted := range cfg.TrustedActors {
		if actor.ID == trusted {
			return true
		}
	}
	return false
}

// EvaluatorConfigFromGovernance converts governance config to evaluator config.
func EvaluatorConfigFromGovernance(cfg *config.GovernanceConfig) evaluator.Config {
	return evaluator.Config{
		DefaultDecision:         cgp.DecisionApproved,
		AutoApproveThreshold:    cfg.AutoApproveThreshold,
		RequireHumanForBreaking: cfg.RequireHumanForBreaking,
		RequireHumanForSecurity: cfg.RequireHumanForSecurity,
		MaxAutoApproveRisk:      cfg.MaxAutoApproveRisk,
	}
}
