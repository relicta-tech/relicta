package governance

import (
	"sort"
	"time"

	"github.com/relicta-tech/relicta/internal/cgp"
	pkgcgp "github.com/relicta-tech/relicta/pkg/cgp"
)

// ApprovalCardInput collects the surrounding context BuildApprovalCard needs.
//
// Kept as a small struct rather than positional args so callers (CLI, MCP,
// Hub) can grow the input over time without churning every call site.
type ApprovalCardInput struct {
	// Result is the governance evaluation output to render.
	Result *EvaluateReleaseOutput

	// CardID is a stable per-card identifier (e.g. "card:<release_id>:<decision_id>").
	// Empty falls back to "card:<release_id>".
	CardID string

	// ReleaseID ties the card to a release run.
	ReleaseID string

	// Version is the proposed version, optional.
	Version string

	// Repository identifies the system, optional.
	Repository string

	// Actor is the proposing actor (publically-shaped pkg/cgp.Actor).
	Actor pkgcgp.Actor

	// DiffSummary is an optional change summary; renderers show it above
	// AvailableActions.
	DiffSummary string

	// Verifiers lists humans who have already cosigned (multi-level approval).
	Verifiers []pkgcgp.Actor

	// Frameworks lists compliance frameworks this card contributes evidence to.
	// Renderers use this for badge rows; pass nil to omit.
	Frameworks []string

	// AuditChainHash anchors the card content to the hash chain.
	AuditChainHash string
}

// BuildApprovalCard converts an EvaluateReleaseOutput into the canonical
// pkg/cgp.ApprovalCard wire format used by every Relicta surface
// (CLI TUI, web dashboard, MCP tool dispatch, Hub UI).
//
// Keeping the conversion in one place prevents drift: every renderer
// consumes the same shape, so changes in the governance output flow to
// every UI without per-renderer touch-ups.
func BuildApprovalCard(in ApprovalCardInput) pkgcgp.ApprovalCard {
	r := in.Result
	if r == nil {
		// Defensive: empty card with the available identifiers populated.
		return pkgcgp.ApprovalCard{
			CGPVersion:       pkgcgp.ProtocolVersion,
			CardID:           cardIDOrDefault(in.CardID, in.ReleaseID),
			ReleaseID:        in.ReleaseID,
			Version:          in.Version,
			Repository:       in.Repository,
			Actor:            in.Actor,
			AvailableActions: pkgcgp.CanonicalActions(),
			CreatedAt:        time.Now().UTC(),
		}
	}

	tier := pkgcgp.RiskTierForScore(r.RiskScore)

	risk := pkgcgp.RiskBlock{
		Score:    r.RiskScore,
		Tier:     tier,
		Severity: severityToString(r.Severity, tier),
		Glyph:    pkgcgp.RiskGlyphForTier(tier),
		Factors:  convertRiskFactors(r.RiskFactors),
	}

	return pkgcgp.ApprovalCard{
		CGPVersion:       pkgcgp.ProtocolVersion,
		CardID:           cardIDOrDefault(in.CardID, in.ReleaseID),
		ReleaseID:        in.ReleaseID,
		Version:          in.Version,
		Repository:       in.Repository,
		Risk:             risk,
		DiffSummary:      in.DiffSummary,
		Actor:            in.Actor,
		Verifiers:        in.Verifiers,
		Decision:         string(r.Decision),
		Rationale:        append([]string(nil), r.Rationale...),
		RequiredActions:  convertRequiredActions(r.RequiredActions),
		AvailableActions: pkgcgp.CanonicalActions(),
		Frameworks:       in.Frameworks,
		AuditChainHash:   in.AuditChainHash,
		CreatedAt:        time.Now().UTC(),
	}
}

// convertRiskFactors maps the internal cgp.RiskFactor (typed Severity) to
// the public pkg/cgp.RiskFactor (string Severity), sorting by score desc.
func convertRiskFactors(in []cgp.RiskFactor) []pkgcgp.RiskFactor {
	out := make([]pkgcgp.RiskFactor, 0, len(in))
	for _, f := range in {
		out = append(out, pkgcgp.RiskFactor{
			Category:    f.Category,
			Description: f.Description,
			Score:       f.Score,
			Severity:    string(f.Severity),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

// convertRequiredActions maps internal RequiredAction → public ApprovalAction
// for the card's RequiredActions list. Mostly a Description→Description shuttle;
// the public ApprovalAction shape carries more metadata than required-action
// records typically have, so unset fields stay zero.
func convertRequiredActions(in []cgp.RequiredAction) []pkgcgp.ApprovalAction {
	if len(in) == 0 {
		return nil
	}
	out := make([]pkgcgp.ApprovalAction, 0, len(in))
	for _, a := range in {
		out = append(out, pkgcgp.ApprovalAction{
			ID:          a.Type,
			Label:       a.Type,
			Description: a.Description,
			Type:        "secondary",
		})
	}
	return out
}

// severityToString converts the typed cgp.Severity to its lower-case string.
// Falls back to the canonical tier when severity is the zero value.
func severityToString(s cgp.Severity, tier string) string {
	str := string(s)
	if str == "" {
		// Tier is "low|medium|high|critical" — match severity convention.
		return tier
	}
	return str
}

// cardIDOrDefault returns the explicit card ID or a stable default derived
// from the release ID.
func cardIDOrDefault(cardID, releaseID string) string {
	if cardID != "" {
		return cardID
	}
	if releaseID == "" {
		return "card:unknown"
	}
	return "card:" + releaseID
}
