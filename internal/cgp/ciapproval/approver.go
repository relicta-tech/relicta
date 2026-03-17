package ciapproval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
)

// ApprovalRequest contains the data needed for CI approval evaluation.
type ApprovalRequest struct {
	// ReleaseID is the unique identifier for the release.
	ReleaseID string `json:"releaseId"`

	// Version is the version being released.
	Version string `json:"version"`

	// BumpType is the semantic version bump type (major, minor, patch).
	BumpType string `json:"bumpType"`

	// Branch is the git branch being released from.
	Branch string `json:"branch"`

	// RiskScore is the evaluated risk score (0.0-1.0).
	RiskScore float64 `json:"riskScore"`

	// HasBreakingChanges indicates if there are breaking changes.
	HasBreakingChanges bool `json:"hasBreakingChanges"`

	// HasSecurityChanges indicates if there are security-related changes.
	HasSecurityChanges bool `json:"hasSecurityChanges"`

	// CommitCount is the number of commits in this release.
	CommitCount int `json:"commitCount"`

	// SignedCommits is the number of signed commits.
	SignedCommits int `json:"signedCommits"`

	// Timestamp is when the approval was requested.
	Timestamp time.Time `json:"timestamp"`

	// Metadata contains additional context.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ApprovalResult contains the outcome of a CI approval evaluation.
type ApprovalResult struct {
	// Approved indicates if the release was approved.
	Approved bool `json:"approved"`

	// Decision is the CGP decision type.
	Decision cgp.DecisionType `json:"decision"`

	// Reasons lists why the approval succeeded or failed.
	Reasons []string `json:"reasons"`

	// Actor is the CI actor that performed the approval.
	Actor cgp.Actor `json:"actor"`

	// Timestamp is when the approval was made.
	Timestamp time.Time `json:"timestamp"`

	// ValidUntil is when the approval expires.
	ValidUntil time.Time `json:"validUntil"`

	// AuditID is a unique identifier for audit tracking.
	AuditID string `json:"auditId"`

	// CIContext contains CI environment information.
	CIContext map[string]string `json:"ciContext,omitempty"`

	// DryRun indicates if this was a dry run.
	DryRun bool `json:"dryRun"`
}

// Approver handles CI-safe approval logic.
type Approver struct {
	config *Config
}

// New creates a new CI-safe Approver.
func New(cfg *Config) *Approver {
	if cfg == nil {
		cfg = FromEnvironment()
	}
	return &Approver{config: cfg}
}

// NewFromEnvironment creates an Approver configured from environment variables.
func NewFromEnvironment() *Approver {
	return New(FromEnvironment())
}

// Config returns the current configuration.
func (a *Approver) Config() *Config {
	return a.config
}

// IsEnabled returns true if CI approval is enabled.
func (a *Approver) IsEnabled() bool {
	return a.config.Enabled
}

// CanAutoApprove returns true if auto-approval is possible.
func (a *Approver) CanAutoApprove() bool {
	return a.config.Enabled && a.config.AutoApprove
}

// Evaluate checks if a release can be auto-approved in CI.
func (a *Approver) Evaluate(ctx context.Context, req *ApprovalRequest) (*ApprovalResult, error) {
	result := &ApprovalResult{
		Approved:   false,
		Decision:   cgp.DecisionApprovalRequired,
		Reasons:    []string{},
		Timestamp:  time.Now().UTC(),
		ValidUntil: time.Now().UTC().Add(a.config.ApprovalTimeout),
		AuditID:    generateAuditID(),
		CIContext:  GetCIContext(),
		DryRun:     a.config.DryRun,
	}

	// Create actor
	result.Actor = a.createActor()

	// Check if CI approval is enabled
	if !a.config.Enabled {
		result.Reasons = append(result.Reasons, "CI approval is not enabled")
		return result, nil
	}

	// Check if auto-approve is enabled
	if !a.config.AutoApprove {
		result.Reasons = append(result.Reasons, "Auto-approve is not enabled in CI")
		return result, nil
	}

	// Run all checks
	checks := []struct {
		name  string
		check func(*ApprovalRequest) (bool, string)
	}{
		{"risk_score", a.checkRiskScore},
		{"bump_type", a.checkBumpType},
		{"branch", a.checkBranch},
		{"breaking_changes", a.checkBreakingChanges},
		{"security_changes", a.checkSecurityChanges},
		{"signed_commits", a.checkSignedCommits},
	}

	allPassed := true
	for _, c := range checks {
		passed, reason := c.check(req)
		if !passed {
			allPassed = false
			result.Reasons = append(result.Reasons, reason)
		}
	}

	if allPassed {
		result.Approved = true
		result.Decision = cgp.DecisionApproved
		result.Reasons = append(result.Reasons, "All CI approval checks passed")

		// Add bypass reason if provided
		if a.config.BypassReason != "" {
			result.Reasons = append(result.Reasons, fmt.Sprintf("Bypass reason: %s", a.config.BypassReason))
		}
	}

	// Write audit log
	if a.config.AuditLogPath != "" && !a.config.DryRun {
		if err := a.writeAuditLog(req, result); err != nil {
			// Don't fail on audit log error, but include it in reasons
			result.Reasons = append(result.Reasons, fmt.Sprintf("Warning: failed to write audit log: %v", err))
		}
	}

	return result, nil
}

// checkRiskScore validates the risk score is below threshold.
func (a *Approver) checkRiskScore(req *ApprovalRequest) (bool, string) {
	if req.RiskScore > a.config.MaxRiskScore {
		return false, fmt.Sprintf("Risk score %.2f exceeds maximum %.2f for auto-approval",
			req.RiskScore, a.config.MaxRiskScore)
	}
	return true, ""
}

// checkBumpType validates the bump type is allowed.
func (a *Approver) checkBumpType(req *ApprovalRequest) (bool, string) {
	if len(a.config.AllowedBumpTypes) == 0 {
		return true, ""
	}

	bumpType := strings.ToLower(req.BumpType)
	for _, allowed := range a.config.AllowedBumpTypes {
		if strings.EqualFold(allowed, bumpType) {
			return true, ""
		}
	}

	return false, fmt.Sprintf("Bump type '%s' is not in allowed list: %v",
		req.BumpType, a.config.AllowedBumpTypes)
}

// checkBranch validates the branch is allowed.
func (a *Approver) checkBranch(req *ApprovalRequest) (bool, string) {
	if len(a.config.AllowedBranches) == 0 {
		return true, ""
	}

	for _, pattern := range a.config.AllowedBranches {
		if matchBranch(req.Branch, pattern) {
			return true, ""
		}
	}

	return false, fmt.Sprintf("Branch '%s' is not in allowed list: %v",
		req.Branch, a.config.AllowedBranches)
}

// checkBreakingChanges validates breaking changes policy.
func (a *Approver) checkBreakingChanges(req *ApprovalRequest) (bool, string) {
	if a.config.BlockBreakingChanges && req.HasBreakingChanges {
		return false, "Breaking changes detected - requires manual approval"
	}
	return true, ""
}

// checkSecurityChanges validates security changes policy.
func (a *Approver) checkSecurityChanges(req *ApprovalRequest) (bool, string) {
	if a.config.BlockSecurityChanges && req.HasSecurityChanges {
		return false, "Security-related changes detected - requires manual approval"
	}
	return true, ""
}

// checkSignedCommits validates commit signing requirements.
func (a *Approver) checkSignedCommits(req *ApprovalRequest) (bool, string) {
	if !a.config.RequireSignedCommits {
		return true, ""
	}

	if req.CommitCount == 0 {
		return true, ""
	}

	if req.SignedCommits < req.CommitCount {
		return false, fmt.Sprintf("Only %d of %d commits are signed - all commits must be signed",
			req.SignedCommits, req.CommitCount)
	}

	return true, ""
}

// createActor creates a CGP actor for the CI approver.
func (a *Approver) createActor() cgp.Actor {
	actorID := a.config.TrustedActor
	if actorID == "" {
		actorID = detectCIActor()
	}

	// Build attributes from CI context
	ciContext := GetCIContext()
	attrs := make(map[string]string)
	for k, v := range ciContext {
		attrs[k] = v
	}

	return cgp.Actor{
		ID:         actorID,
		Kind:       cgp.ActorKindCI,
		Name:       extractActorName(actorID),
		TrustLevel: cgp.TrustLevelTrusted,
		Attributes: attrs,
	}
}

// extractActorName extracts a display name from the actor ID.
func extractActorName(actorID string) string {
	parts := strings.Split(actorID, ":")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return actorID
}

// matchBranch checks if a branch matches a pattern.
// Supports wildcards: main, master, release/*, feature/*.
func matchBranch(branch, pattern string) bool {
	// Exact match
	if branch == pattern {
		return true
	}

	// Wildcard match
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		if strings.HasPrefix(branch, prefix) {
			return true
		}
	}

	// Refs prefix handling
	branch = strings.TrimPrefix(branch, "refs/heads/")
	return branch == pattern
}

// AuditEntry represents a single audit log entry.
type AuditEntry struct {
	AuditID   string           `json:"auditId"`
	Timestamp time.Time        `json:"timestamp"`
	Request   *ApprovalRequest `json:"request"`
	Result    *ApprovalResult  `json:"result"`
	Config    *Config          `json:"config"`
}

// writeAuditLog writes an audit entry to the configured log file.
func (a *Approver) writeAuditLog(req *ApprovalRequest, result *ApprovalResult) error {
	entry := &AuditEntry{
		AuditID:   result.AuditID,
		Timestamp: time.Now().UTC(),
		Request:   req,
		Result:    result,
		Config:    a.config,
	}

	// Ensure directory exists
	dir := filepath.Dir(a.config.AuditLogPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create audit log directory: %w", err)
	}

	// Read existing entries
	var entries []AuditEntry
	data, err := os.ReadFile(a.config.AuditLogPath)
	if err == nil {
		_ = json.Unmarshal(data, &entries)
	}

	// Append new entry
	entries = append(entries, *entry)

	// Write back
	data, err = json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal audit entries: %w", err)
	}

	if err := os.WriteFile(a.config.AuditLogPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write audit log: %w", err)
	}

	return nil
}

// generateAuditID generates a unique audit identifier.
func generateAuditID() string {
	return fmt.Sprintf("ci-audit-%d", time.Now().UnixNano())
}

// PreApprovedPolicy represents a pre-configured approval policy.
type PreApprovedPolicy struct {
	// Name is the policy name.
	Name string `json:"name"`

	// Description explains what this policy allows.
	Description string `json:"description"`

	// Conditions are the conditions that must be met.
	Conditions []PolicyCondition `json:"conditions"`

	// Enabled indicates if this policy is active.
	Enabled bool `json:"enabled"`

	// Priority determines evaluation order (higher = first).
	Priority int `json:"priority"`
}

// PolicyCondition represents a single condition in a pre-approved policy.
type PolicyCondition struct {
	// Field is the field to check (e.g., "bump_type", "risk_score", "branch").
	Field string `json:"field"`

	// Operator is the comparison operator (eq, ne, lt, lte, gt, gte, in, matches).
	Operator string `json:"operator"`

	// Value is the value to compare against.
	Value any `json:"value"`
}

// DefaultPreApprovedPolicies returns common pre-approved policies.
func DefaultPreApprovedPolicies() []PreApprovedPolicy {
	return []PreApprovedPolicy{
		{
			Name:        "patch-releases",
			Description: "Auto-approve patch releases with low risk",
			Conditions: []PolicyCondition{
				{Field: "bump_type", Operator: "eq", Value: "patch"},
				{Field: "risk_score", Operator: "lte", Value: 0.3},
				{Field: "has_breaking_changes", Operator: "eq", Value: false},
			},
			Enabled:  true,
			Priority: 100,
		},
		{
			Name:        "documentation-only",
			Description: "Auto-approve documentation-only changes",
			Conditions: []PolicyCondition{
				{Field: "bump_type", Operator: "eq", Value: "patch"},
				{Field: "risk_score", Operator: "lte", Value: 0.1},
			},
			Enabled:  true,
			Priority: 90,
		},
		{
			Name:        "dependency-updates",
			Description: "Auto-approve minor dependency updates",
			Conditions: []PolicyCondition{
				{Field: "bump_type", Operator: "in", Value: []string{"patch", "minor"}},
				{Field: "risk_score", Operator: "lte", Value: 0.4},
				{Field: "has_security_changes", Operator: "eq", Value: false},
			},
			Enabled:  true,
			Priority: 80,
		},
	}
}

// EvaluatePolicy checks if a request matches a pre-approved policy.
func EvaluatePolicy(policy *PreApprovedPolicy, req *ApprovalRequest) bool {
	if !policy.Enabled {
		return false
	}

	for _, cond := range policy.Conditions {
		if !evaluateCondition(cond, req) {
			return false
		}
	}

	return true
}

// evaluateCondition evaluates a single policy condition.
func evaluateCondition(cond PolicyCondition, req *ApprovalRequest) bool {
	var fieldValue any

	switch cond.Field {
	case "bump_type":
		fieldValue = strings.ToLower(req.BumpType)
	case "risk_score":
		fieldValue = req.RiskScore
	case "branch":
		fieldValue = req.Branch
	case "has_breaking_changes":
		fieldValue = req.HasBreakingChanges
	case "has_security_changes":
		fieldValue = req.HasSecurityChanges
	case "commit_count":
		fieldValue = req.CommitCount
	default:
		return false
	}

	switch cond.Operator {
	case "eq":
		return fieldValue == cond.Value
	case "ne":
		return fieldValue != cond.Value
	case "lt":
		return compareNumeric(fieldValue, cond.Value) < 0
	case "lte":
		return compareNumeric(fieldValue, cond.Value) <= 0
	case "gt":
		return compareNumeric(fieldValue, cond.Value) > 0
	case "gte":
		return compareNumeric(fieldValue, cond.Value) >= 0
	case "in":
		return valueIn(fieldValue, cond.Value)
	case "matches":
		return matchBranch(fmt.Sprintf("%v", fieldValue), fmt.Sprintf("%v", cond.Value))
	default:
		return false
	}
}

// compareNumeric compares two numeric values.
func compareNumeric(a, b any) int {
	af := toFloat64(a)
	bf := toFloat64(b)
	if af < bf {
		return -1
	}
	if af > bf {
		return 1
	}
	return 0
}

// toFloat64 converts a value to float64.
func toFloat64(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	default:
		return 0
	}
}

// valueIn checks if a value is in a list.
func valueIn(value any, list any) bool {
	switch l := list.(type) {
	case []string:
		s := fmt.Sprintf("%v", value)
		for _, item := range l {
			if item == s {
				return true
			}
		}
	case []any:
		for _, item := range l {
			if item == value {
				return true
			}
		}
	}
	return false
}
