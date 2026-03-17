package governance

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/config"
)

func TestNewServiceFromConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.GovernanceConfig
		wantErr bool
	}{
		{
			name: "minimal config",
			cfg: &config.GovernanceConfig{
				Enabled:              true,
				AutoApproveThreshold: 0.3,
				MaxAutoApproveRisk:   0.5,
				MemoryEnabled:        false,
			},
			wantErr: false,
		},
		{
			name: "with memory enabled",
			cfg: &config.GovernanceConfig{
				Enabled:              true,
				AutoApproveThreshold: 0.3,
				MaxAutoApproveRisk:   0.5,
				MemoryEnabled:        true,
				MemoryPath:           ".relicta/test/memory.json",
			},
			wantErr: false,
		},
		{
			name: "with custom policies",
			cfg: &config.GovernanceConfig{
				Enabled:              true,
				AutoApproveThreshold: 0.3,
				MaxAutoApproveRisk:   0.5,
				MemoryEnabled:        false,
				Policies: []config.GovernancePolicyConfig{
					{
						Name:        "high-risk-review",
						Description: "Require review for high-risk changes",
						Priority:    100,
						Action:      "require_review",
						Conditions: []config.PolicyConditionConfig{
							{Field: "risk.score", Operator: "gte", Value: 0.7},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp dir for memory store
			tmpDir := t.TempDir()

			logger := slog.Default()
			svc, err := NewServiceFromConfig(tt.cfg, tmpDir, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewServiceFromConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && svc == nil {
				t.Error("NewServiceFromConfig() returned nil service")
			}
		})
	}
}

func TestNewServiceFromConfig_WithMemory(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.GovernanceConfig{
		Enabled:              true,
		AutoApproveThreshold: 0.3,
		MaxAutoApproveRisk:   0.5,
		MemoryEnabled:        true,
		MemoryPath:           "governance/memory.json",
	}

	logger := slog.Default()
	svc, err := NewServiceFromConfig(cfg, tmpDir, logger)
	if err != nil {
		t.Fatalf("NewServiceFromConfig() error = %v", err)
	}
	if svc == nil {
		t.Fatal("NewServiceFromConfig() returned nil service")
	}

	// Verify memory directory was created
	memoryDir := filepath.Join(tmpDir, "governance")
	if _, err := os.Stat(memoryDir); os.IsNotExist(err) {
		t.Errorf("Memory directory was not created: %s", memoryDir)
	}
}

func TestBuildPolicies(t *testing.T) {
	tests := []struct {
		name       string
		policyCfgs []config.GovernancePolicyConfig
		wantCount  int
	}{
		{
			name:       "empty policies",
			policyCfgs: nil,
			wantCount:  0,
		},
		{
			name:       "empty slice",
			policyCfgs: []config.GovernancePolicyConfig{},
			wantCount:  0,
		},
		{
			name: "single policy",
			policyCfgs: []config.GovernancePolicyConfig{
				{
					Name:     "test-policy",
					Priority: 10,
					Action:   "approve",
					Conditions: []config.PolicyConditionConfig{
						{Field: "risk.score", Operator: "lt", Value: 0.3},
					},
				},
			},
			wantCount: 1,
		},
		{
			name: "multiple policies",
			policyCfgs: []config.GovernancePolicyConfig{
				{
					Name:     "low-risk",
					Priority: 10,
					Action:   "approve",
					Conditions: []config.PolicyConditionConfig{
						{Field: "risk.score", Operator: "lt", Value: 0.3},
					},
				},
				{
					Name:     "high-risk",
					Priority: 100,
					Action:   "require_review",
					Conditions: []config.PolicyConditionConfig{
						{Field: "risk.score", Operator: "gte", Value: 0.7},
					},
				},
			},
			wantCount: 1, // All rules go into one policy
		},
		{
			name: "disabled policy excluded",
			policyCfgs: []config.GovernancePolicyConfig{
				{
					Name:     "enabled-policy",
					Priority: 10,
					Action:   "approve",
					Conditions: []config.PolicyConditionConfig{
						{Field: "risk.score", Operator: "lt", Value: 0.3},
					},
				},
				{
					Name:     "disabled-policy",
					Enabled:  boolPtr(false),
					Priority: 100,
					Action:   "deny",
					Conditions: []config.PolicyConditionConfig{
						{Field: "actor.kind", Operator: "eq", Value: "agent"},
					},
				},
			},
			wantCount: 1, // Only one policy with enabled rules
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.Default()
			policies := buildPolicies(tt.policyCfgs, logger)
			if len(policies) != tt.wantCount {
				t.Errorf("buildPolicies() returned %d policies, want %d", len(policies), tt.wantCount)
			}
		})
	}
}

func TestBuildRule(t *testing.T) {
	tests := []struct {
		name       string
		cfg        config.GovernancePolicyConfig
		wantID     string
		wantAction string
	}{
		{
			name: "approve action",
			cfg: config.GovernancePolicyConfig{
				Name:   "approve-low-risk",
				Action: "approve",
				Conditions: []config.PolicyConditionConfig{
					{Field: "risk.score", Operator: "lt", Value: 0.3},
				},
			},
			wantID:     "cfg_approve-low-risk",
			wantAction: "set_decision",
		},
		{
			name: "deny action",
			cfg: config.GovernancePolicyConfig{
				Name:   "deny-high-risk",
				Action: "deny",
				Conditions: []config.PolicyConditionConfig{
					{Field: "risk.score", Operator: "gte", Value: 0.9},
				},
			},
			wantID:     "cfg_deny-high-risk",
			wantAction: "set_decision",
		},
		{
			name: "require_review action",
			cfg: config.GovernancePolicyConfig{
				Name:   "review-medium-risk",
				Action: "require_review",
				Conditions: []config.PolicyConditionConfig{
					{Field: "risk.score", Operator: "gte", Value: 0.5},
				},
			},
			wantID:     "cfg_review-medium-risk",
			wantAction: "set_decision",
		},
		{
			name: "with message",
			cfg: config.GovernancePolicyConfig{
				Name:    "with-message",
				Action:  "require_review",
				Message: "This change requires additional review",
				Conditions: []config.PolicyConditionConfig{
					{Field: "change.breaking", Operator: "eq", Value: true},
				},
			},
			wantID:     "cfg_with-message",
			wantAction: "set_decision",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := buildRule(tt.cfg)
			if rule.ID != tt.wantID {
				t.Errorf("buildRule() ID = %q, want %q", rule.ID, tt.wantID)
			}
			if !rule.Enabled {
				t.Error("buildRule() created disabled rule")
			}
			if len(rule.Actions) == 0 {
				t.Error("buildRule() created rule without actions")
			}
			if rule.Actions[0].Type != tt.wantAction {
				t.Errorf("buildRule() action type = %q, want %q", rule.Actions[0].Type, tt.wantAction)
			}
		})
	}
}

func TestIsActorTrusted(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *config.GovernanceConfig
		actor  cgp.Actor
		wanted bool
	}{
		{
			name: "trusted actor",
			cfg: &config.GovernanceConfig{
				TrustedActors: []string{"user-123", "user-456"},
			},
			actor:  cgp.Actor{ID: "user-123"},
			wanted: true,
		},
		{
			name: "untrusted actor",
			cfg: &config.GovernanceConfig{
				TrustedActors: []string{"user-123", "user-456"},
			},
			actor:  cgp.Actor{ID: "user-789"},
			wanted: false,
		},
		{
			name: "empty trusted list",
			cfg: &config.GovernanceConfig{
				TrustedActors: []string{},
			},
			actor:  cgp.Actor{ID: "user-123"},
			wanted: false,
		},
		{
			name: "nil trusted list",
			cfg: &config.GovernanceConfig{
				TrustedActors: nil,
			},
			actor:  cgp.Actor{ID: "user-123"},
			wanted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsActorTrusted(tt.cfg, tt.actor)
			if got != tt.wanted {
				t.Errorf("IsActorTrusted() = %v, want %v", got, tt.wanted)
			}
		})
	}
}

func TestEvaluatorConfigFromGovernance(t *testing.T) {
	cfg := &config.GovernanceConfig{
		AutoApproveThreshold:    0.25,
		MaxAutoApproveRisk:      0.6,
		RequireHumanForBreaking: true,
		RequireHumanForSecurity: false,
	}

	evalCfg := EvaluatorConfigFromGovernance(cfg)

	if evalCfg.AutoApproveThreshold != 0.25 {
		t.Errorf("AutoApproveThreshold = %v, want 0.25", evalCfg.AutoApproveThreshold)
	}
	if evalCfg.MaxAutoApproveRisk != 0.6 {
		t.Errorf("MaxAutoApproveRisk = %v, want 0.6", evalCfg.MaxAutoApproveRisk)
	}
	if !evalCfg.RequireHumanForBreaking {
		t.Error("RequireHumanForBreaking should be true")
	}
	if evalCfg.RequireHumanForSecurity {
		t.Error("RequireHumanForSecurity should be false")
	}
	if evalCfg.DefaultDecision != cgp.DecisionApproved {
		t.Errorf("DefaultDecision = %v, want approved", evalCfg.DefaultDecision)
	}
}

func boolPtr(b bool) *bool {
	return &b
}
