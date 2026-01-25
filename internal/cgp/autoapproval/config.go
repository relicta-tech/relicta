// Package autoapproval provides configurable policy-based auto-approval for CGP.
// It enables automatic approval of low-risk changes while routing high-risk
// changes to human review based on configurable thresholds and policies.
package autoapproval

import (
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
)

// Config configures policy-based auto-approval behavior.
type Config struct {
	// Enabled controls whether auto-approval is active.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Thresholds configures risk-based routing.
	Thresholds ThresholdConfig `json:"thresholds" yaml:"thresholds"`

	// Policies are the auto-approval policies to evaluate.
	Policies []AutoApprovalPolicy `json:"policies" yaml:"policies"`

	// Exemptions define what cannot be auto-approved.
	Exemptions ExemptionConfig `json:"exemptions" yaml:"exemptions"`

	// ActorRules configure actor-specific auto-approval behavior.
	ActorRules ActorConfig `json:"actorRules" yaml:"actorRules"`

	// AuditConfig configures auto-approval audit logging.
	AuditConfig AuditConfig `json:"audit" yaml:"audit"`
}

// ThresholdConfig defines risk thresholds for auto-approval decisions.
type ThresholdConfig struct {
	// AutoApproveMax is the maximum risk score for auto-approval.
	// Changes with risk <= this value may be auto-approved if policies allow.
	AutoApproveMax float64 `json:"autoApproveMax" yaml:"autoApproveMax"`

	// RequireReviewMin is the minimum risk score that requires human review.
	// Changes with risk >= this value always require review.
	RequireReviewMin float64 `json:"requireReviewMin" yaml:"requireReviewMin"`

	// RejectMin is the minimum risk score for automatic rejection.
	// Changes with risk >= this value are rejected without review.
	RejectMin float64 `json:"rejectMin" yaml:"rejectMin"`

	// ApproverCountByRisk maps risk ranges to required approver counts.
	// Key format: "0.3-0.5" for range, value is approver count.
	ApproverCountByRisk map[string]int `json:"approverCountByRisk" yaml:"approverCountByRisk"`
}

// AutoApprovalPolicy defines conditions under which changes are auto-approved.
type AutoApprovalPolicy struct {
	// ID is the unique policy identifier.
	ID string `json:"id" yaml:"id"`

	// Name is a human-readable policy name.
	Name string `json:"name" yaml:"name"`

	// Description explains what this policy does.
	Description string `json:"description" yaml:"description"`

	// Enabled controls whether this policy is active.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Priority determines evaluation order (higher = first).
	Priority int `json:"priority" yaml:"priority"`

	// Conditions that must all be true for auto-approval.
	Conditions []PolicyCondition `json:"conditions" yaml:"conditions"`

	// TimeConstraints limit when this policy applies.
	TimeConstraints *TimeConstraints `json:"timeConstraints,omitempty" yaml:"timeConstraints,omitempty"`

	// ActorConstraints limit who this policy applies to.
	ActorConstraints *ActorConstraints `json:"actorConstraints,omitempty" yaml:"actorConstraints,omitempty"`

	// MaxApprovals is the maximum auto-approvals per time window (0 = unlimited).
	MaxApprovals int `json:"maxApprovals,omitempty" yaml:"maxApprovals,omitempty"`

	// ApprovalWindow is the time window for MaxApprovals.
	ApprovalWindow time.Duration `json:"approvalWindow,omitempty" yaml:"approvalWindow,omitempty"`
}

// PolicyCondition is a single condition in an auto-approval policy.
type PolicyCondition struct {
	// Field is the path to the field to check.
	// Supports: risk.score, change.breaking, change.security, intent.suggestedBump,
	// scope.branch, actor.kind, actor.trustLevel
	Field string `json:"field" yaml:"field"`

	// Operator is the comparison operator.
	// Supports: eq, ne, lt, lte, gt, gte, in, not_in, contains, matches
	Operator string `json:"operator" yaml:"operator"`

	// Value is the value to compare against.
	Value any `json:"value" yaml:"value"`
}

// TimeConstraints limit when a policy applies.
type TimeConstraints struct {
	// BusinessHoursOnly restricts to business hours.
	BusinessHoursOnly bool `json:"businessHoursOnly" yaml:"businessHoursOnly"`

	// AllowedDays restricts to specific days (0=Sunday, 6=Saturday).
	AllowedDays []int `json:"allowedDays,omitempty" yaml:"allowedDays,omitempty"`

	// BlockedDates lists dates when policy is inactive (YYYY-MM-DD format).
	BlockedDates []string `json:"blockedDates,omitempty" yaml:"blockedDates,omitempty"`

	// FreezeWindows define periods when auto-approval is disabled.
	FreezeWindows []FreezeWindow `json:"freezeWindows,omitempty" yaml:"freezeWindows,omitempty"`
}

// FreezeWindow is a period when auto-approval is disabled.
type FreezeWindow struct {
	// Name identifies the freeze period.
	Name string `json:"name" yaml:"name"`

	// Start is when the freeze begins.
	Start time.Time `json:"start" yaml:"start"`

	// End is when the freeze ends.
	End time.Time `json:"end" yaml:"end"`

	// Reason explains why the freeze exists.
	Reason string `json:"reason" yaml:"reason"`
}

// ActorConstraints limit who a policy applies to.
type ActorConstraints struct {
	// AllowedActorKinds restricts which actor types can use this policy.
	AllowedActorKinds []cgp.ActorKind `json:"allowedActorKinds,omitempty" yaml:"allowedActorKinds,omitempty"`

	// MinTrustLevel is the minimum trust level required (0=Untrusted to 3=Full).
	MinTrustLevel cgp.TrustLevel `json:"minTrustLevel,omitempty" yaml:"minTrustLevel,omitempty"`

	// AllowedActorIDs explicitly allows specific actors.
	AllowedActorIDs []string `json:"allowedActorIds,omitempty" yaml:"allowedActorIds,omitempty"`

	// BlockedActorIDs explicitly blocks specific actors.
	BlockedActorIDs []string `json:"blockedActorIds,omitempty" yaml:"blockedActorIds,omitempty"`
}

// ExemptionConfig defines what cannot be auto-approved.
type ExemptionConfig struct {
	// BreakingChanges prevents auto-approval of breaking changes.
	BreakingChanges bool `json:"breakingChanges" yaml:"breakingChanges"`

	// SecurityChanges prevents auto-approval of security changes.
	SecurityChanges bool `json:"securityChanges" yaml:"securityChanges"`

	// MajorVersions prevents auto-approval of major version bumps.
	MajorVersions bool `json:"majorVersions" yaml:"majorVersions"`

	// NewDependencies prevents auto-approval of new dependency additions.
	NewDependencies bool `json:"newDependencies" yaml:"newDependencies"`

	// LargeChanges prevents auto-approval above file/line thresholds.
	LargeChanges *LargeChangeConfig `json:"largeChanges,omitempty" yaml:"largeChanges,omitempty"`

	// AffectedComponents lists components that cannot be auto-approved.
	AffectedComponents []string `json:"affectedComponents,omitempty" yaml:"affectedComponents,omitempty"`

	// ProtectedBranches lists branches where auto-approval is disabled.
	ProtectedBranches []string `json:"protectedBranches,omitempty" yaml:"protectedBranches,omitempty"`
}

// LargeChangeConfig defines thresholds for "large" changes.
type LargeChangeConfig struct {
	// MaxFiles is the maximum number of changed files.
	MaxFiles int `json:"maxFiles" yaml:"maxFiles"`

	// MaxLines is the maximum number of changed lines.
	MaxLines int `json:"maxLines" yaml:"maxLines"`

	// MaxCommits is the maximum number of commits.
	MaxCommits int `json:"maxCommits" yaml:"maxCommits"`
}

// ActorConfig configures actor-specific auto-approval rules.
type ActorConfig struct {
	// TrustedActors can always auto-approve (override exemptions).
	TrustedActors []string `json:"trustedActors,omitempty" yaml:"trustedActors,omitempty"`

	// AgentAutoApprove allows AI agents to trigger auto-approval.
	AgentAutoApprove bool `json:"agentAutoApprove" yaml:"agentAutoApprove"`

	// CIAutoApprove allows CI systems to trigger auto-approval.
	CIAutoApprove bool `json:"ciAutoApprove" yaml:"ciAutoApprove"`

	// RequireHumanForAgentChanges requires human review for agent proposals.
	RequireHumanForAgentChanges bool `json:"requireHumanForAgentChanges" yaml:"requireHumanForAgentChanges"`
}

// AuditConfig configures auto-approval audit logging.
type AuditConfig struct {
	// LogAllDecisions logs all auto-approval decisions.
	LogAllDecisions bool `json:"logAllDecisions" yaml:"logAllDecisions"`

	// LogDeniedDetails logs detailed reasons for denial.
	LogDeniedDetails bool `json:"logDeniedDetails" yaml:"logDeniedDetails"`

	// IncludeProposalDetails includes full proposal in audit.
	IncludeProposalDetails bool `json:"includeProposalDetails" yaml:"includeProposalDetails"`
}

// DefaultConfig returns sensible defaults for auto-approval.
func DefaultConfig() *Config {
	return &Config{
		Enabled: true,
		Thresholds: ThresholdConfig{
			AutoApproveMax:   0.3, // Auto-approve if risk <= 0.3
			RequireReviewMin: 0.5, // Require review if risk >= 0.5
			RejectMin:        0.9, // Auto-reject if risk >= 0.9
			ApproverCountByRisk: map[string]int{
				"0.3-0.5": 1, // 1 approver for medium-low risk
				"0.5-0.7": 2, // 2 approvers for medium risk
				"0.7-0.9": 3, // 3 approvers for high risk
			},
		},
		Policies: DefaultPolicies(),
		Exemptions: ExemptionConfig{
			BreakingChanges: true,
			SecurityChanges: true,
			MajorVersions:   true,
			NewDependencies: false,
			LargeChanges: &LargeChangeConfig{
				MaxFiles:   50,
				MaxLines:   1000,
				MaxCommits: 20,
			},
		},
		ActorRules: ActorConfig{
			AgentAutoApprove:            false, // Agents can't auto-approve by default
			CIAutoApprove:               true,  // CI can auto-approve
			RequireHumanForAgentChanges: true,  // Agent changes need human review
		},
		AuditConfig: AuditConfig{
			LogAllDecisions:        true,
			LogDeniedDetails:       true,
			IncludeProposalDetails: false,
		},
	}
}

// DefaultPolicies returns the default auto-approval policies.
func DefaultPolicies() []AutoApprovalPolicy {
	return []AutoApprovalPolicy{
		{
			ID:          "patch-releases",
			Name:        "Patch Release Auto-Approval",
			Description: "Auto-approve patch releases with low risk and no breaking changes",
			Enabled:     true,
			Priority:    100,
			Conditions: []PolicyCondition{
				{Field: "intent.suggestedBump", Operator: "eq", Value: "patch"},
				{Field: "risk.score", Operator: "lte", Value: 0.3},
				{Field: "change.breaking", Operator: "eq", Value: 0},
				{Field: "change.security", Operator: "eq", Value: 0},
			},
		},
		{
			ID:          "docs-only",
			Name:        "Documentation-Only Changes",
			Description: "Auto-approve changes that only affect documentation",
			Enabled:     true,
			Priority:    90,
			Conditions: []PolicyCondition{
				{Field: "risk.score", Operator: "lte", Value: 0.1},
				{Field: "change.features", Operator: "eq", Value: 0},
				{Field: "change.fixes", Operator: "eq", Value: 0},
				{Field: "change.breaking", Operator: "eq", Value: 0},
			},
		},
		{
			ID:          "trusted-ci",
			Name:        "Trusted CI Pipeline",
			Description: "Auto-approve low-risk changes from trusted CI systems",
			Enabled:     true,
			Priority:    80,
			Conditions: []PolicyCondition{
				{Field: "actor.kind", Operator: "eq", Value: "ci"},
				{Field: "actor.trustLevel", Operator: "gte", Value: 2}, // Trusted or higher
				{Field: "risk.score", Operator: "lte", Value: 0.4},
			},
		},
		{
			ID:          "minor-features-trusted",
			Name:        "Minor Features from Trusted Sources",
			Description: "Auto-approve minor feature additions from trusted actors with low risk",
			Enabled:     true,
			Priority:    70,
			Conditions: []PolicyCondition{
				{Field: "intent.suggestedBump", Operator: "in", Value: []string{"patch", "minor"}},
				{Field: "actor.trustLevel", Operator: "gte", Value: 3}, // Verified or higher
				{Field: "risk.score", Operator: "lte", Value: 0.25},
				{Field: "change.breaking", Operator: "eq", Value: 0},
			},
			ActorConstraints: &ActorConstraints{
				AllowedActorKinds: []cgp.ActorKind{cgp.ActorKindHuman, cgp.ActorKindCI},
			},
		},
		{
			ID:          "dependency-updates",
			Name:        "Routine Dependency Updates",
			Description: "Auto-approve minor/patch dependency updates with no security impact",
			Enabled:     true,
			Priority:    60,
			Conditions: []PolicyCondition{
				{Field: "change.dependencies", Operator: "gt", Value: 0},
				{Field: "change.features", Operator: "eq", Value: 0},
				{Field: "change.fixes", Operator: "eq", Value: 0},
				{Field: "change.breaking", Operator: "eq", Value: 0},
				{Field: "change.security", Operator: "eq", Value: 0},
				{Field: "risk.score", Operator: "lte", Value: 0.3},
			},
		},
	}
}

// StrictConfig returns a more restrictive configuration.
func StrictConfig() *Config {
	cfg := DefaultConfig()
	cfg.Thresholds.AutoApproveMax = 0.2
	cfg.Thresholds.RequireReviewMin = 0.3
	cfg.Exemptions.NewDependencies = true
	cfg.ActorRules.CIAutoApprove = false
	cfg.ActorRules.RequireHumanForAgentChanges = true
	return cfg
}

// PermissiveConfig returns a more permissive configuration.
func PermissiveConfig() *Config {
	cfg := DefaultConfig()
	cfg.Thresholds.AutoApproveMax = 0.5
	cfg.Thresholds.RequireReviewMin = 0.7
	cfg.Exemptions.BreakingChanges = false
	cfg.ActorRules.AgentAutoApprove = true
	return cfg
}
