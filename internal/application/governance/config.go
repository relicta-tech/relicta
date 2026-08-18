// Package governance provides CGP (Change Governance Protocol) integration for release workflows.
package governance

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/v4/internal/cgp/identity"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy/dsl"
	"github.com/relicta-tech/relicta/v4/internal/config"
)

// DefaultMemoryStorePath is where governance history lives inside a repository.
const DefaultMemoryStorePath = ".relicta/governance/memory.json"

// MemoryStorePath resolves the governance memory store's location.
//
// Exposed because two commands were reading two different stores. The governance
// service writes release outcomes to .relicta/governance/memory.json, and
// `relicta history` opened .relicta/memory — falling back to ~/.relicta/memory
// when that did not exist. So publishing recorded an outcome and history read an
// empty store, in a different directory, and reported "no release history found"
// for every repository.
//
// The relative default is resolved against the repository root rather than the
// process working directory: a relative store path would make the history depend
// on which subdirectory a command ran from, and the home-directory fallback turned
// that into silently reading a global store instead.
func MemoryStorePath(configured, repoRoot string) string {
	path := configured
	if path == "" {
		path = DefaultMemoryStorePath
	}
	if !filepath.IsAbs(path) && repoRoot != "" {
		path = filepath.Join(repoRoot, path)
	}
	return path
}

// NewServiceFromConfig creates a governance service from configuration.
// It sets up the evaluator and policy engine from the provided configuration.
// Construction reads from disk (identity registry), so ctx governs that I/O
// rather than being started afresh here.
//
// memoryStore is the governance store the composition root resolved, and nil means
// "no historical tracking" — the state a container with governance.memory_enabled
// false has always been in.
//
// It is a parameter rather than something built here, and that is the point of the
// change that introduced it. This function used to call memory.NewFileStore on
// MemoryStorePath itself, which made it a second answer to "where does governance
// memory live". Two answers were already visibly wrong: with mnemos enabled the
// container's store was the Mnemos adapter and this one was still a file, and with
// the file backend the two were separate FileStore instances over one memory.json,
// each holding the whole document in memory and rewriting all of it on every write —
// so whichever wrote last erased what the other had recorded. Once persistence.backend
// selects a governance store the same duplication becomes the ADR-013 defect itself:
// the service would keep writing JSON while the tracker wrote to the database.
func NewServiceFromConfig(
	ctx context.Context,
	cfg *config.GovernanceConfig,
	repoPath string,
	logger *slog.Logger,
	memoryStore memory.Store,
) (*Service, error) {
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

	// Load policies from DSL files
	dslPolicies := loadDSLPolicies(cfg.PolicyDir, repoPath, logger)
	policies = append(policies, dslPolicies...)

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
		WithCalibration(cfg.CalibrationEnabled),
		WithReputation(cfg.ReputationEnabled),
		WithReputationThreshold(cfg.ReputationProbationThreshold),
		WithAttribution(cfg.AttributionEnabled),
		WithCalibrationValidation(cfg.CalibrationMinAccuracy, cfg.CalibrationStrict),
		WithEarnedTrust(cfg.EarnedTrustEnabled),
		WithEarnedTrustSamples(cfg.EarnedTrustMinSamples, cfg.EarnedTrustFullSamples),
	}

	// Historical tracking runs on the store the caller resolved, and on nothing else.
	//
	// Both conditions are checked because they answer different questions and can
	// disagree: memory_enabled is the operator's intent, and a nil store is the
	// composition root reporting that it could not open one. Attaching a nil store
	// would turn every read into a nil dereference on a path — reputation, calibration,
	// earned trust — that is meant to degrade to "no history" instead.
	if cfg.MemoryEnabled && memoryStore != nil {
		opts = append(opts, WithMemoryStore(memoryStore))
	}

	// Set up identity registry if configured
	if cfg.IdentityRegistryPath != "" {
		regDir := cfg.IdentityRegistryPath
		if !filepath.IsAbs(regDir) && repoPath != "" {
			regDir = filepath.Join(repoPath, regDir)
		}

		store, err := identity.NewFileStore(regDir)
		if err != nil {
			logger.Warn("failed to create identity registry store, proceeding without identity grants",
				"error", err, "path", regDir)
		} else if registry, err := identity.NewRegistry(ctx, store, identity.WithLogger(logger)); err != nil {
			logger.Warn("failed to load identity registry, proceeding without identity grants",
				"error", err, "path", regDir)
		} else {
			opts = append(opts, WithIdentityRegistry(registry))
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
//
// An entry matches either the actor's full kind-scoped ID ("human:alice",
// "ci:deploy-bot") or its bare name ("alice"). The scoped form lets operators
// grant trust to a human without also trusting a CI actor that happens to run
// under the same identity (e.g. GITHUB_ACTOR=alice in a workflow); the bare
// form is kept for backwards compatibility with existing allowlists.
func IsActorTrusted(cfg *config.GovernanceConfig, actor cgp.Actor) bool {
	for _, trusted := range cfg.TrustedActors {
		if trusted == "" {
			continue
		}
		if actor.ID == trusted || (actor.Name != "" && actor.Name == trusted) {
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

// loadDSLPolicies loads policy files from the configured policy directory.
// It searches for .policy and .cgp files in the policy directory and
// all default policy paths.
func loadDSLPolicies(policyDir, repoPath string, logger *slog.Logger) []policy.Policy {
	var allPolicies []policy.Policy

	loader := dsl.NewLoader(dsl.LoaderOptions{
		IgnoreErrors: true, // Continue loading even if some files fail
		Recursive:    true, // Search subdirectories
	})

	// Collect all paths to search
	var searchPaths []string

	// Add configured policy directory
	if policyDir != "" {
		searchPaths = append(searchPaths, policyDir)
	}

	// Add default policy paths
	searchPaths = append(searchPaths, dsl.DefaultPolicyPaths()...)

	// Search each path
	seenPaths := make(map[string]bool)
	for _, dir := range searchPaths {
		// Make path absolute if relative
		absDir := dir
		if !filepath.IsAbs(absDir) && repoPath != "" {
			absDir = filepath.Join(repoPath, dir)
		}

		// Skip if already searched
		if seenPaths[absDir] {
			continue
		}
		seenPaths[absDir] = true

		// Load policies from this directory
		result, err := loader.LoadDir(absDir)
		if err != nil {
			logger.Debug("failed to load policies from directory",
				"path", absDir,
				"error", err,
			)
			continue
		}

		// Log any errors from individual files
		for _, loadErr := range result.Errors {
			logger.Warn("failed to load policy file",
				"file", loadErr.File,
				"error", loadErr.Error,
			)
		}

		// Add successfully loaded policies
		for _, pol := range result.Policies {
			allPolicies = append(allPolicies, *pol)
			logger.Debug("loaded policy from DSL file",
				"name", pol.Name,
				"rules", len(pol.Rules),
			)
		}
	}

	if len(allPolicies) > 0 {
		logger.Info("loaded DSL policies",
			"count", len(allPolicies),
		)
	}

	return allPolicies
}
