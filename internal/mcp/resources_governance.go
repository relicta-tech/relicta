package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mcplib "go.klarlabs.de/mcp"

	"github.com/relicta-tech/relicta/internal/cgp/policy"
	"github.com/relicta-tech/relicta/internal/domain/release"
	pkgcgp "github.com/relicta-tech/relicta/pkg/cgp"
)

// Governance-as-context MCP resources.
//
// These resources let coding agents (Claude Code, Cursor, Devin, etc.) plan
// WITH governance context BEFORE proposing changes — shift-left from
// post-facto governance evaluation. Pairs with the existing tool surface
// (relicta_evaluate, relicta_approve) which is post-facto by definition.
//
// Resource URIs:
//   relicta://policy/current             — active policy snapshot
//   relicta://risk-budget                — autonomy budget set
//   relicta://risk-budget/{actor_id}     — single actor's resolved budget
//   relicta://recent-incidents           — last-30d incident feed
//   relicta://compliance-frameworks      — frameworks Relicta enforces
//
// blast-radius/{service} is intentionally NOT registered here — it requires
// a service-graph dependency that the adapter does not yet expose. Track in
// follow-up Sprint 5.21b.

// registerGovernanceContextResources adds the reverse-MCP resources that turn
// Relicta into a context provider for coding agents. Called from
// registerResources to keep server.go focused.
func (s *Server) registerGovernanceContextResources() {
	s.server.Resource("relicta://policy/current").
		Name("Active Policy").
		Description("Current CGP governance policy in effect — agents read this BEFORE proposing changes to align their drafts with org rules.").
		MimeType("application/json").
		Handler(s.handleResourcePolicyCurrent)

	s.server.Resource("relicta://risk-budget").
		Name("Actor Autonomy Budgets").
		Description("All configured per-actor governance budgets. Use risk-budget/{actor_id} for a single actor's resolved budget.").
		MimeType("application/json").
		Handler(s.handleResourceRiskBudgetAll)

	s.server.Resource("relicta://risk-budget/{actor_id}").
		Name("Actor Autonomy Budget").
		Description("Resolved per-actor budget (max blast radius, max risk, allowed tools, cosign requirements).").
		MimeType("application/json").
		Handler(s.handleResourceRiskBudgetSingle)

	s.server.Resource("relicta://recent-incidents").
		Name("Recent Incidents").
		Description("Production incidents in the last 30 days correlated with releases. Agents use this to weigh blast-radius decisions.").
		MimeType("application/json").
		Handler(s.handleResourceRecentIncidents)

	s.server.Resource("relicta://compliance-frameworks").
		Name("Compliance Frameworks").
		Description("Compliance frameworks Relicta enforces (SOC 2, EU AI Act, ISO 27001/42001, HIPAA, PCI-DSS, NIST, OWASP).").
		MimeType("application/json").
		Handler(s.handleResourceComplianceFrameworks)

	s.server.Resource("relicta://approval").
		Name("Approval Card").
		Description("Canonical ApprovalCard for the active release. Same shape consumed by the CLI TUI, web dashboard, and Hub UI — agents read this to render or reason about pending approvals.").
		MimeType("application/json").
		Handler(s.handleResourceApprovalCard)
}

// handleResourcePolicyCurrent returns the active CGP policy snapshot.
//
// Agents that read this before drafting a change can pre-align with rules
// (e.g. avoid touching files the policy blocks, scope changes to fit
// auto-approve thresholds).
func (s *Server) handleResourcePolicyCurrent(_ context.Context, uri string, _ map[string]string) (*mcplib.ResourceContent, error) {
	if s.policyEngine == nil {
		return jsonResource(uri, map[string]any{
			"status":  "no_policy_engine",
			"message": "policy engine not configured for this server instance",
		})
	}

	// The Engine does not currently expose its loaded policies via a public
	// accessor, so we surface a stable summary that agents can rely on.
	// When a public Policies() accessor is added, this can return the full
	// rule list verbatim.
	return jsonResource(uri, map[string]any{
		"status": "active",
		"hint":   "policy engine is active; use the relicta_evaluate tool to dry-run a change against current rules",
		"interaction_pattern": []string{
			"1. Draft your proposed change",
			"2. Call relicta_evaluate with the change scope",
			"3. Read returned governance decision + required actions",
			"4. Iterate until policy permits",
		},
	})
}

// handleResourceRiskBudgetAll returns the full configured budget set.
func (s *Server) handleResourceRiskBudgetAll(_ context.Context, uri string, _ map[string]string) (*mcplib.ResourceContent, error) {
	if s.actorBudgets == nil {
		return jsonResource(uri, map[string]any{
			"status":   "no_explicit_budgets",
			"fallback": "DefaultRestrictiveAgentBudget applies to all agents",
			"defaults": map[string]any{
				"agent": policy.DefaultRestrictiveAgentBudget(),
				"human": policy.DefaultPermissiveHumanBudget(),
			},
		})
	}
	return jsonResource(uri, map[string]any{
		"status":  "configured",
		"budgets": s.actorBudgets.Budgets,
	})
}

// handleResourceRiskBudgetSingle returns the resolved budget for a specific actor.
//
// The URI follows the template `relicta://risk-budget/{actor_id}`. The MCP
// transport surfaces the trailing path segment as the `actor_id` parameter.
// When the param is missing (older transports) we fall back to parsing the URI.
func (s *Server) handleResourceRiskBudgetSingle(_ context.Context, uri string, params map[string]string) (*mcplib.ResourceContent, error) {
	actorID := params["actor_id"]
	if actorID == "" {
		// Fallback: derive from URI suffix.
		const prefix = "relicta://risk-budget/"
		if strings.HasPrefix(uri, prefix) {
			actorID = strings.TrimPrefix(uri, prefix)
		}
	}
	if actorID == "" {
		return jsonResource(uri, map[string]any{
			"status":  "missing_actor_id",
			"message": "URI must include trailing actor_id (e.g. relicta://risk-budget/claude-code-1)",
		})
	}

	// Treat the actor as an agent for resolution purposes (humans are the
	// expected callers of relicta CLI directly; MCP traffic is agent-shaped).
	budget := policy.ResolveBudget(s.actorBudgets, "agent", actorID)
	return jsonResource(uri, map[string]any{
		"status":   "resolved",
		"actor_id": actorID,
		"budget":   budget,
	})
}

// handleResourceRecentIncidents returns the last 30 days of incidents.
//
// When no incident store is wired the resource returns an empty list with
// a helpful status string rather than failing — agents should still be able
// to read the resource even in alpha-development setups.
func (s *Server) handleResourceRecentIncidents(_ context.Context, uri string, _ map[string]string) (*mcplib.ResourceContent, error) {
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	return jsonResource(uri, map[string]any{
		"status": "ok",
		"window": map[string]any{
			"since": cutoff.UTC().Format(time.RFC3339),
			"until": time.Now().UTC().Format(time.RFC3339),
		},
		"incidents": []any{},
		"note":      "incident store integration is pending; once wired, this returns IncidentRecord summaries from internal/cgp/memory",
	})
}

// handleResourceComplianceFrameworks lists the frameworks Relicta enforces.
//
// Agents can read this to understand which control IDs may be cited in
// approval rationales and which evidence types Relicta can produce.
func (s *Server) handleResourceComplianceFrameworks(_ context.Context, uri string, _ map[string]string) (*mcplib.ResourceContent, error) {
	frameworks := []map[string]any{
		{"id": "soc2", "name": "SOC 2 Type II", "controls": []string{"CC6.1", "CC7.1", "CC7.4", "CC8.1"}, "evidence_types": []string{"change_management", "access_review", "risk_assessment"}},
		{"id": "eu-ai-act-article-12", "name": "EU AI Act — Article 12 Record-Keeping", "retention": "6 months minimum (Art 26(6))", "evidence_types": []string{"audit_log"}},
		{"id": "eu-ai-act-annex-iv", "name": "EU AI Act — Annex IV Technical Documentation", "retention": "10 years (Art 11)", "sections": 8, "evidence_types": []string{"technical_documentation"}},
		{"id": "iso-27001", "name": "ISO/IEC 27001:2022", "controls": []string{"A.5.10", "A.8.32"}, "evidence_types": []string{"change_management"}},
		{"id": "iso-42001", "name": "ISO/IEC 42001:2023 (AI Management)", "evidence_types": []string{"ai_governance"}},
		{"id": "hipaa", "name": "HIPAA", "controls": []string{"164.312(b)"}, "evidence_types": []string{"audit_log"}},
		{"id": "pci-dss", "name": "PCI-DSS", "controls": []string{"6.4"}, "evidence_types": []string{"change_management"}},
		{"id": "nist-800-53", "name": "NIST SP 800-53", "controls": []string{"AU-3", "AU-12"}, "evidence_types": []string{"audit_log"}},
		{"id": "owasp-llm-top10", "name": "OWASP LLM Top 10 (2025)", "controls": []string{"LLM06"}},
		{"id": "owasp-agentic-top10", "name": "OWASP Agentic AI Top 10 (2026)", "controls": []string{"AG04"}, "status": "partial"},
	}

	return jsonResource(uri, map[string]any{
		"status":     "active",
		"frameworks": frameworks,
	})
}

// handleResourceApprovalCard emits the canonical pkg/cgp.ApprovalCard for
// the active release run. Agents read this to render or reason about the
// pending approval without reimplementing the schema in MCP-land.
//
// When no release is active, returns a status payload rather than an empty
// card so callers can branch on "no active release" cleanly.
func (s *Server) handleResourceApprovalCard(ctx context.Context, uri string, _ map[string]string) (*mcplib.ResourceContent, error) {
	if s.releaseRepo == nil {
		return jsonResource(uri, map[string]any{
			"status":  "no_release_repo",
			"message": "release repository not configured for this MCP server instance",
		})
	}

	releases, err := s.releaseRepo.FindActive(ctx)
	if err != nil || len(releases) == 0 {
		return jsonResource(uri, map[string]any{
			"status":  "no_active_release",
			"message": "no active release run; run 'relicta plan' to start one",
		})
	}

	rel := releases[0]
	card := approvalCardFromActiveRun(rel)
	return jsonResource(uri, card)
}

// approvalCardFromActiveRun synthesizes a canonical ApprovalCard from the
// active ReleaseRun's state. When governance evaluation has run, the risk
// score and decision are populated; otherwise decision="not_evaluated" so
// agents can branch.
//
// Lightweight builder — does NOT trigger evaluation. For evaluated cards
// callers should invoke `relicta_evaluate` first.
func approvalCardFromActiveRun(rel *release.ReleaseRun) pkgcgp.ApprovalCard {
	versionStr := ""
	if v := rel.VersionNext(); !v.IsZero() {
		versionStr = v.String()
	}

	riskScore := rel.RiskScore()
	tier := pkgcgp.RiskTierForScore(riskScore)

	decision := "not_evaluated"
	if rel.State().String() == "approved" || rel.State().String() == "publishing" || rel.State().String() == "published" {
		decision = "approved"
	} else if rel.State().String() == "failed" || rel.State().String() == "canceled" {
		decision = "rejected"
	} else if riskScore > 0 {
		decision = "approval_required"
	}

	return pkgcgp.ApprovalCard{
		CGPVersion: pkgcgp.ProtocolVersion,
		CardID:     "card:" + string(rel.ID()),
		ReleaseID:  string(rel.ID()),
		Version:    versionStr,
		Repository: rel.RepoID(),
		Risk: pkgcgp.RiskBlock{
			Score:    riskScore,
			Tier:     tier,
			Severity: severityFromTier(tier),
			Glyph:    pkgcgp.RiskGlyphForTier(tier),
		},
		Actor: pkgcgp.Actor{
			Kind: string(rel.ActorType()),
			ID:   rel.ActorID(),
		},
		Decision:         decision,
		AvailableActions: pkgcgp.CanonicalActions(),
		CreatedAt:        rel.CreatedAt(),
	}
}

// severityFromTier returns the upper-case severity string matching the
// canonical tier mapping. Mirrors the CLI lipgloss + Vue rendering.
func severityFromTier(tier string) string {
	switch tier {
	case "critical":
		return "CRITICAL"
	case "high":
		return "HIGH"
	case "medium":
		return "MEDIUM"
	default:
		return "LOW"
	}
}

// jsonResource is a small helper that marshals any payload to a JSON
// MCP ResourceContent and returns it. Marshal failures fall back to a
// plain status payload so the resource read never fails outright.
func jsonResource(uri string, payload any) (*mcplib.ResourceContent, error) {
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return &mcplib.ResourceContent{
			URI:      uri,
			MimeType: "application/json",
			Text:     fmt.Sprintf(`{"error": %q}`, err.Error()),
		}, nil
	}
	return &mcplib.ResourceContent{
		URI:      uri,
		MimeType: "application/json",
		Text:     string(b),
	}, nil
}
