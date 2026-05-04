package cgp

import "time"

// ApprovalCard is the canonical JSON contract for rendering a release-approval
// view across every Relicta surface (CLI TUI, web dashboard, Hub UI, MCP
// resource, third-party tooling). Defining it here in pkg/cgp ensures
// external consumers and Vue/TS clients import the same shape Go renders.
//
// Three surfaces drift today; one schema fixes the drift:
//   - CLI: internal/cli/approve.go's displayGovernanceResult
//   - Web: relicta/web/src/views/ApprovalWorkflow.vue
//   - MCP: tool dispatch + relicta://approval resource
//
// All fields use the same camelCase JSON wire encoding as ChangeProposal /
// GovernanceDecision so renderers can pipe the same byte stream.
type ApprovalCard struct {
	// CGPVersion is the wire-format version. Always "0.1" for v0.1 cards.
	CGPVersion string `json:"cgpVersion"`

	// CardID uniquely identifies this card render. Stable per (release, decision).
	CardID string `json:"cardId"`

	// ReleaseID ties the card back to the underlying release run.
	ReleaseID string `json:"releaseId"`

	// Version is the proposed version under approval.
	Version string `json:"version,omitempty"`

	// Repository identifies the system (org/repo) being released.
	Repository string `json:"repository,omitempty"`

	// Risk is the headline decision-driving signal. Render with highest
	// visual weight (Von Restorff).
	Risk RiskBlock `json:"risk"`

	// DiffSummary is the change summary string shown above the actions.
	// Optional — empty when not yet generated.
	DiffSummary string `json:"diffSummary,omitempty"`

	// Actor identifies who proposed the release. Drives autonomy budget
	// enforcement and audit attribution.
	Actor Actor `json:"actor"`

	// Verifiers lists humans who have already cosigned (when multi-level
	// approval is active). Empty for single-approver flows.
	Verifiers []Actor `json:"verifiers,omitempty"`

	// Decision is the current governance state ("approved", "approval_required",
	// "rejected", "deferred").
	Decision string `json:"decision"`

	// Rationale is the ordered list of reasoning bullets supporting the decision.
	Rationale []string `json:"rationale,omitempty"`

	// RequiredActions lists items the approver must complete before publish.
	RequiredActions []ApprovalAction `json:"requiredActions,omitempty"`

	// AvailableActions lists buttons / commands the approver may invoke.
	// Order is canonical: every renderer presents them in this order.
	AvailableActions []ApprovalAction `json:"availableActions"`

	// Frameworks lists the compliance frameworks this approval contributes
	// evidence to (SOC 2, EU AI Act Article 12, ISO 27001, etc.).
	Frameworks []string `json:"frameworks,omitempty"`

	// AuditChainHash is the tamper-evidence anchor for this card content.
	// Empty when the underlying audit chain is not yet finalized.
	AuditChainHash string `json:"auditChainHash,omitempty"`

	// CreatedAt is when the card content was rendered.
	CreatedAt time.Time `json:"createdAt"`
}

// RiskBlock is the headline risk signal of an ApprovalCard.
//
// Score is the canonical 0.0–1.0 number; Tier is a stable enum so renderers
// don't each implement their own tier mapping (drift-prevention).
type RiskBlock struct {
	// Score is the CGP risk score [0.0, 1.0].
	Score float64 `json:"score"`

	// Tier categorizes the score: "low" | "medium" | "high" | "critical".
	// The stable mapping is: <0.4 low, <0.7 medium, <0.85 high, ≥0.85 critical.
	Tier string `json:"tier"`

	// Severity is a human label echoing the tier in upper case for legacy
	// renderers that key off "HIGH" / "MEDIUM" strings. Keep both fields in
	// sync; tier is canonical.
	Severity string `json:"severity"`

	// Factors are the per-category contributions to the score, sorted by
	// score descending. Render top-3 by default.
	Factors []RiskFactor `json:"factors,omitempty"`

	// Glyph is the visual severity indicator for color-blind / NO_COLOR use.
	// Format: 1-4 ▲ characters.
	Glyph string `json:"glyph,omitempty"`
}

// ApprovalAction is one button or command the approver may invoke.
//
// IDs are stable across surfaces so a UI button click and an MCP tool invocation
// produce the same audit-trail entry.
type ApprovalAction struct {
	// ID is the stable action identifier. Examples: "approve", "reject",
	// "edit_notes", "request_changes", "rollback".
	ID string `json:"id"`

	// Label is the human-readable text shown in UIs.
	Label string `json:"label"`

	// Description optionally explains what the action does.
	Description string `json:"description,omitempty"`

	// Type categorizes the action: "primary" | "secondary" | "danger".
	// Renderers use this to assign visual treatment.
	Type string `json:"type"`

	// RequiresConfirmation hints UIs to add a confirm step before invoking.
	// True for any destructive or irreversible action.
	RequiresConfirmation bool `json:"requiresConfirmation,omitempty"`

	// RequiresCosigner hints that the autonomy budget for the calling actor
	// requires a cosigner before the action can succeed.
	RequiresCosigner bool `json:"requiresCosigner,omitempty"`
}

// CanonicalActions returns the ordered list of standard approval actions.
// Renderers use this as the default AvailableActions ordering so users see
// the same button layout in every surface (Jakob's Law).
func CanonicalActions() []ApprovalAction {
	return []ApprovalAction{
		{ID: "approve", Label: "Approve", Type: "primary", Description: "Approve the release for publishing."},
		{ID: "reject", Label: "Reject", Type: "danger", Description: "Reject the release; sends it back for revision.", RequiresConfirmation: true},
		{ID: "edit_notes", Label: "Edit Notes", Type: "secondary", Description: "Edit the release notes before approving."},
		{ID: "request_changes", Label: "Request Changes", Type: "secondary", Description: "Request specific changes before approval."},
	}
}

// RiskTierForScore maps a numeric risk score to a canonical tier string.
// Renderers and Hub UIs MUST use this mapping to avoid drift.
func RiskTierForScore(score float64) string {
	switch {
	case score >= 0.85:
		return "critical"
	case score >= 0.7:
		return "high"
	case score >= 0.4:
		return "medium"
	default:
		return "low"
	}
}

// RiskGlyphForTier returns the Unicode severity glyph for a tier.
// Used by terminal renderers and any UI that wants color-blind safety.
func RiskGlyphForTier(tier string) string {
	switch tier {
	case "critical":
		return "▲▲▲▲"
	case "high":
		return "▲▲▲"
	case "medium":
		return "▲▲"
	default:
		return "▲"
	}
}
