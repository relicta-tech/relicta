# Product Requirements Document (PRD)

Relicta is transitioning from its v2 release automation roots toward a v3 change-governance foundation. The full requirements, principles, and agentic strategy are recorded in `docs/internal/prd.md`, which is the canonical source for internal stakeholders.

## Public summary

- **Product goal:** Govern every software change (human or AI generated) by capturing explicit decisions, approvals, and outcomes before execution.
- **Focus areas:** risk-aware evaluation, policy enforcement, audit-quality communication, and a governance protocol (CGP) that sits between change generation and deployment.
- **Target audience:** platform and DevOps engineers, product managers, and security/compliance teams who need to understand “what changed, why, and who approved it.”
- **Current state:** v2 delivered AI-assisted release planning; v3 extends that with richer governance, agent controls, and outcome-first comms.
- **Commit intelligence:** supports conventional commits and infers meaning from unstructured history via heuristics, AST analysis, and optional AI, with human review for low-confidence cases.

Refer to the internal PRD for the complete problem framing, solution architecture, and evolution roadmap.
