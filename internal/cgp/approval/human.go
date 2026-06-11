// Package approval provides human-in-the-loop approval workflows for CGP.
// It enables interactive approval gates with rationale capture, policy display,
// and proper audit record creation.
package approval

import (
	"context"
	"fmt"
	"time"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
)

// ApprovalStatus represents the status of an approval request.
type ApprovalStatus string

// Approval statuses.
const (
	StatusPending  ApprovalStatus = "pending"
	StatusApproved ApprovalStatus = "approved"
	StatusRejected ApprovalStatus = "rejected"
	StatusDeferred ApprovalStatus = "deferred"
	StatusExpired  ApprovalStatus = "expired"
)

// ApprovalRequest represents a request for human approval.
type ApprovalRequest struct {
	// ID uniquely identifies this approval request.
	ID string `json:"id"`

	// ProposalID links to the change proposal being reviewed.
	ProposalID string `json:"proposalId"`

	// ReleaseID links to the release being approved.
	ReleaseID string `json:"releaseId"`

	// Version is the version being released.
	Version string `json:"version"`

	// Actor is who is requesting approval.
	Requester cgp.Actor `json:"requester"`

	// RiskAssessment contains the evaluated risk.
	RiskAssessment *RiskSummary `json:"riskAssessment,omitempty"`

	// PolicyEvaluations shows which policies were evaluated.
	PolicyEvaluations []PolicyEvaluation `json:"policyEvaluations,omitempty"`

	// RequiredApprovers lists actors who must approve.
	RequiredApprovers []string `json:"requiredApprovers,omitempty"`

	// Context provides additional context for the reviewer.
	Context *ApprovalContext `json:"context,omitempty"`

	// ExpiresAt is when this approval request expires.
	ExpiresAt time.Time `json:"expiresAt"`

	// CreatedAt is when this request was created.
	CreatedAt time.Time `json:"createdAt"`

	// Metadata contains additional request data.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// RiskSummary contains risk information for the approval request.
type RiskSummary struct {
	// OverallScore is the combined risk score (0.0-1.0).
	OverallScore float64 `json:"overallScore"`

	// Severity is the risk severity classification.
	Severity string `json:"severity"`

	// Factors are the individual risk contributions.
	Factors []RiskFactorSummary `json:"factors,omitempty"`

	// Recommendations are suggested actions.
	Recommendations []string `json:"recommendations,omitempty"`
}

// RiskFactorSummary represents a single risk factor.
type RiskFactorSummary struct {
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`
	Severity    string  `json:"severity"`
}

// PolicyEvaluation shows the result of evaluating a policy.
type PolicyEvaluation struct {
	// PolicyID is the policy identifier.
	PolicyID string `json:"policyId"`

	// PolicyName is the human-readable name.
	PolicyName string `json:"policyName"`

	// Matched indicates if the policy matched.
	Matched bool `json:"matched"`

	// Decision is the policy's decision.
	Decision string `json:"decision"`

	// Rationale explains why this policy matched.
	Rationale string `json:"rationale,omitempty"`
}

// ApprovalContext provides context for the reviewer.
type ApprovalContext struct {
	// Repository is the repository being released.
	Repository string `json:"repository"`

	// Branch is the branch being released.
	Branch string `json:"branch"`

	// CommitCount is the number of commits.
	CommitCount int `json:"commitCount"`

	// BreakingChanges lists breaking change descriptions.
	BreakingChanges []string `json:"breakingChanges,omitempty"`

	// SecurityChanges lists security-related changes.
	SecurityChanges []string `json:"securityChanges,omitempty"`

	// AffectedComponents lists impacted components.
	AffectedComponents []string `json:"affectedComponents,omitempty"`

	// HistoricalContext provides historical release data.
	HistoricalContext *HistoricalContext `json:"historicalContext,omitempty"`
}

// HistoricalContext provides historical context for the decision.
type HistoricalContext struct {
	// RecentReleases is the count of recent releases.
	RecentReleases int `json:"recentReleases"`

	// SuccessRate is the historical success rate.
	SuccessRate float64 `json:"successRate"`

	// RollbackRate is the historical rollback rate.
	RollbackRate float64 `json:"rollbackRate"`

	// SimilarChangesOutcome summarizes outcomes of similar changes.
	SimilarChangesOutcome string `json:"similarChangesOutcome,omitempty"`
}

// ApprovalResponse represents a human's approval decision.
type ApprovalResponse struct {
	// RequestID links to the approval request.
	RequestID string `json:"requestId"`

	// Status is the approval status.
	Status ApprovalStatus `json:"status"`

	// Approver is who made the decision.
	Approver cgp.Actor `json:"approver"`

	// Rationale explains why the decision was made.
	Rationale string `json:"rationale"`

	// RationaleType categorizes the rationale.
	RationaleType RationaleType `json:"rationaleType"`

	// Conditions are any conditions attached to the approval.
	Conditions []string `json:"conditions,omitempty"`

	// ReviewedFactors lists which risk factors were reviewed.
	ReviewedFactors []string `json:"reviewedFactors,omitempty"`

	// Timestamp is when the decision was made.
	Timestamp time.Time `json:"timestamp"`

	// ValidUntil is when this approval expires.
	ValidUntil time.Time `json:"validUntil,omitempty"`

	// Metadata contains additional response data.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// RationaleType categorizes the approval rationale.
type RationaleType string

// Rationale types.
const (
	RationaleTypeReviewed     RationaleType = "reviewed"      // Full review conducted
	RationaleTypeTrusted      RationaleType = "trusted"       // Trusted source/actor
	RationaleTypeEmergency    RationaleType = "emergency"     // Emergency bypass
	RationaleTypePolicyBased  RationaleType = "policy_based"  // Policy-driven decision
	RationaleTypeRiskAccepted RationaleType = "risk_accepted" // Known risks accepted
	RationaleTypeOther        RationaleType = "other"         // Other/custom reason
)

// Approver handles human approval workflow.
type Approver struct {
	config *Config
}

// Config configures the human approval workflow.
type Config struct {
	// DefaultTimeout is the default approval timeout.
	DefaultTimeout time.Duration `json:"defaultTimeout"`

	// RequireRationale requires a rationale for all approvals.
	RequireRationale bool `json:"requireRationale"`

	// MinRationaleLength is the minimum rationale length.
	MinRationaleLength int `json:"minRationaleLength"`

	// RequireFactorReview requires reviewing risk factors.
	RequireFactorReview bool `json:"requireFactorReview"`

	// AllowEmergencyBypass allows emergency bypasses.
	AllowEmergencyBypass bool `json:"allowEmergencyBypass"`

	// EmergencyBypassAudit logs emergency bypasses.
	EmergencyBypassAudit bool `json:"emergencyBypassAudit"`

	// PreApprovedActors can approve without rationale.
	PreApprovedActors []string `json:"preApprovedActors,omitempty"`
}

// DefaultConfig returns sensible defaults for human approval.
func DefaultConfig() *Config {
	return &Config{
		DefaultTimeout:       24 * time.Hour,
		RequireRationale:     true,
		MinRationaleLength:   10,
		RequireFactorReview:  false,
		AllowEmergencyBypass: false,
		EmergencyBypassAudit: true,
	}
}

// New creates a new human approver.
func New(cfg *Config) *Approver {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Approver{config: cfg}
}

// CreateRequest creates a new approval request.
func (a *Approver) CreateRequest(
	proposalID, releaseID, version string,
	requester cgp.Actor,
	risk *RiskSummary,
	policies []PolicyEvaluation,
	ctx *ApprovalContext,
) *ApprovalRequest {
	return &ApprovalRequest{
		ID:                generateRequestID(),
		ProposalID:        proposalID,
		ReleaseID:         releaseID,
		Version:           version,
		Requester:         requester,
		RiskAssessment:    risk,
		PolicyEvaluations: policies,
		Context:           ctx,
		ExpiresAt:         time.Now().UTC().Add(a.config.DefaultTimeout),
		CreatedAt:         time.Now().UTC(),
		Metadata:          make(map[string]any),
	}
}

// ValidateResponse validates an approval response.
func (a *Approver) ValidateResponse(resp *ApprovalResponse) error {
	if resp.RequestID == "" {
		return fmt.Errorf("request ID is required")
	}

	if resp.Status == "" {
		return fmt.Errorf("status is required")
	}

	if err := resp.Approver.Validate(); err != nil {
		return fmt.Errorf("invalid approver: %w", err)
	}

	// Check rationale requirement
	if a.config.RequireRationale && resp.Status != StatusDeferred {
		if resp.Rationale == "" {
			return fmt.Errorf("rationale is required for approval/rejection")
		}
		if len(resp.Rationale) < a.config.MinRationaleLength {
			return fmt.Errorf("rationale must be at least %d characters", a.config.MinRationaleLength)
		}
	}

	// Validate emergency bypass
	if resp.RationaleType == RationaleTypeEmergency && !a.config.AllowEmergencyBypass {
		return fmt.Errorf("emergency bypass is not allowed")
	}

	return nil
}

// ProcessApproval processes an approval and creates the CGP decision.
func (a *Approver) ProcessApproval(
	ctx context.Context,
	req *ApprovalRequest,
	resp *ApprovalResponse,
) (*cgp.GovernanceDecision, error) {
	// Validate response
	if err := a.ValidateResponse(resp); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}

	// Check if request is expired
	if time.Now().After(req.ExpiresAt) {
		return nil, fmt.Errorf("approval request has expired")
	}

	// Check if approver is authorized
	if len(req.RequiredApprovers) > 0 {
		authorized := false
		for _, required := range req.RequiredApprovers {
			if resp.Approver.ID == required {
				authorized = true
				break
			}
		}
		if !authorized {
			return nil, fmt.Errorf("approver %s is not authorized", resp.Approver.ID)
		}
	}

	// Create CGP decision
	var decisionType cgp.DecisionType
	switch resp.Status {
	case StatusApproved:
		decisionType = cgp.DecisionApproved
	case StatusRejected:
		decisionType = cgp.DecisionRejected
	case StatusDeferred:
		decisionType = cgp.DecisionDeferred
	default:
		decisionType = cgp.DecisionApprovalRequired
	}

	decision := cgp.NewDecision(req.ProposalID, decisionType)
	decision.AddRationale(resp.Rationale)

	// Add risk information
	if req.RiskAssessment != nil {
		decision.WithRiskScore(req.RiskAssessment.OverallScore)

		// Add risk factors
		for _, factor := range req.RiskAssessment.Factors {
			decision.AddRiskFactor(
				factor.Category,
				factor.Description,
				factor.Score,
				cgp.Severity(factor.Severity),
			)
		}
	}

	// Add conditions
	for _, condition := range resp.Conditions {
		decision.AddCondition("human_approval_condition", condition)
	}

	// Add rationale metadata as additional rationale entries
	decision.AddRationale(fmt.Sprintf("Approval request: %s", req.ID))
	decision.AddRationale(fmt.Sprintf("Rationale type: %s", resp.RationaleType))
	decision.AddRationale(fmt.Sprintf("Approved by: %s", resp.Approver.ID))

	return decision, nil
}

// IsExpired checks if a request has expired.
func (req *ApprovalRequest) IsExpired() bool {
	return time.Now().After(req.ExpiresAt)
}

// NeedsHumanReview checks if the request requires human review.
func (req *ApprovalRequest) NeedsHumanReview() bool {
	// Always needs review if there are required approvers
	if len(req.RequiredApprovers) > 0 {
		return true
	}

	// Check risk threshold
	if req.RiskAssessment != nil && req.RiskAssessment.OverallScore > 0.4 {
		return true
	}

	// Check for blocking policies
	for _, pol := range req.PolicyEvaluations {
		if pol.Matched && (pol.Decision == "rejected" || pol.Decision == "approval_required") {
			return true
		}
	}

	return false
}

// HasBreakingChanges checks if there are breaking changes.
func (req *ApprovalRequest) HasBreakingChanges() bool {
	if req.Context == nil {
		return false
	}
	return len(req.Context.BreakingChanges) > 0
}

// HasSecurityChanges checks if there are security changes.
func (req *ApprovalRequest) HasSecurityChanges() bool {
	if req.Context == nil {
		return false
	}
	return len(req.Context.SecurityChanges) > 0
}

// RiskLevel returns a human-readable risk level.
func (req *ApprovalRequest) RiskLevel() string {
	if req.RiskAssessment == nil {
		return "unknown"
	}
	score := req.RiskAssessment.OverallScore
	switch {
	case score >= 0.7:
		return "high"
	case score >= 0.4:
		return "medium"
	case score >= 0.2:
		return "low"
	default:
		return "minimal"
	}
}

// generateRequestID generates a unique approval request ID.
func generateRequestID() string {
	return fmt.Sprintf("approval_%d", time.Now().UnixNano())
}

// SuggestedRationales returns suggested rationales based on the request.
func SuggestedRationales(req *ApprovalRequest) []string {
	suggestions := []string{}

	// Low risk suggestions
	if req.RiskLevel() == "low" || req.RiskLevel() == "minimal" {
		suggestions = append(suggestions,
			"Reviewed changes - low risk, standard release process",
			"Routine update with minimal impact",
		)
	}

	// Breaking change suggestions
	if req.HasBreakingChanges() {
		suggestions = append(suggestions,
			"Breaking changes reviewed and migration path confirmed",
			"API changes coordinated with downstream consumers",
		)
	}

	// Security change suggestions
	if req.HasSecurityChanges() {
		suggestions = append(suggestions,
			"Security changes reviewed by security team",
			"Vulnerability fix verified and tested",
		)
	}

	// High risk suggestions
	if req.RiskLevel() == "high" {
		suggestions = append(suggestions,
			"High-risk changes reviewed with extended testing",
			"Rollback plan prepared and tested",
		)
	}

	// Default suggestions
	if len(suggestions) == 0 {
		suggestions = append(suggestions,
			"Changes reviewed and approved for release",
			"Release meets quality and testing requirements",
		)
	}

	return suggestions
}

// RejectionRationales returns common rejection rationales.
func RejectionRationales() []string {
	return []string{
		"Changes require additional testing",
		"Breaking changes not properly documented",
		"Security concerns need to be addressed",
		"Missing required approvals from other teams",
		"Performance impact needs investigation",
		"Deployment timing not appropriate",
		"Technical debt needs to be addressed first",
	}
}
