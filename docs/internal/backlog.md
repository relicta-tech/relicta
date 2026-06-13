# Backlog

## Completed

### Policy Decision Explainability — DONE

Add first-class explainability for policy evaluation so teams can see which rules/conditions drove a decision, with CLI output suitable for CI and human review.

- `--explain` flag on `policy test`
- `--explain-mode all|matched` controls verbosity
- `RuleTrace` + `ConditionTrace` structs capture per-rule/per-condition evaluation
- Output shows field, operator, expected vs actual, matched status

### Matrix Tagging and Sharding — DONE

Extend policy matrix scenarios with tags and add filtering/sharding flags so large policy suites can run in parallel CI jobs with deterministic subsets.

- Tags field on matrix scenarios (`tags: [low-risk, seed]`)
- `--scenario-tag` / `--exclude-scenario-tag` for filtering
- `--shard-index` / `--shard-total` for deterministic FNV-1a sharding
- `--list-scenarios` to preview selected scenarios

### Policy Test CI Artifacts — DONE

Add policy test report exporters (JUnit and compact JSON summary) to integrate matrix outcomes and assertion diffs into CI dashboards and test reporting.

- `--junit-out path` writes JUnit XML with assertion mismatches as failures
- `--summary-out path` writes compact JSON summary (totals, blocked, decisions)
- Both integrate into CI dashboards

### What-If Policy Comparison — DONE

Allow comparing decisions between two policy sets (current vs candidate) across the same matrix to surface governance impact before rollout.

- `--baseline-file` / `--baseline-dir` + `--candidate-file` / `--candidate-dir`
- Per-scenario comparison with strictness ranking
- `--compare-fail-on-stricter` / `--compare-fail-on-looser` threshold flags
- `--compare-max-stricter N` / `--compare-max-looser N` count limits

### Policy Fixture Scaffolding — DONE

Add a command to scaffold policy test fixtures (single input + matrix templates) from existing policies to accelerate adoption and reduce manual setup.

- `relicta policy scaffold` command with `--dir`, `--file`, `--input-out`, `--matrix-out`, `--force`, `--max-rule-scenarios`
- Generates low-risk and high-risk seed scenarios plus per-rule derived scenarios
- Supports JSON and YAML output formats

## Monorepo Workspace Orchestration

Add first-class monorepo support for governing changes across multiple packages in a single repository. Detect workspace structure (Go modules, npm workspaces, Cargo workspaces), analyze per-package changes independently, support independent versioning per package, and coordinate cross-package releases. This addresses the PRD's "Monorepo support gaps" risk (rated High) and is critical for enterprise adoption where monorepos are standard.

---

## CGP Protocol Wire Format

Formalize the Change Governance Protocol (CGP) wire format as described in PRD Section 17. Implement CGP message types (change.proposal, change.decision, change.execution_authorized) as structured JSON with versioning. Add CGP endpoints to the MCP server so AI agents can propose changes via the protocol. Create a CGP SDK package that other tools can import to become CGP-compliant. This positions Relicta as the reference implementation of an open governance standard.

---

## Audience-Aware Release Communication

Implement the stakeholder communication layer from PRD Section 11. Generate audience-specific release narratives (engineering, product, executive, external) derived from approved governance decisions. Support bundling changes across components into coherent narratives. Add communication templates with configurable tone and detail level. Integrate with existing AI providers for narrative generation. This moves Relicta from "output" (changelogs) to "outcomes" (stakeholder communication).

---

## Release Memory and Learning

Build the historical context system from PRD Section 16.4. Track past incidents, rollbacks, and their root causes. Identify patterns of risky changes across releases. Feed historical outcomes back into risk scoring so future evaluations improve over time. Store release memory in the persistence layer (file or PostgreSQL). Surface insights via the dashboard analytics. This enables Relicta to continuously improve beyond what stateless tools can achieve.

---

## Dashboard Frontend Separation

Extract the embedded Vue dashboard into a standalone deployable frontend as described in PRD Section 14.1. Support CDN-friendly deployment (Vercel, CloudFront, S3). Add real-time updates via WebSocket for release progress, approval notifications, and analytics. Build dashboard views for the new analytics data (risk trends, decision distribution, team metrics). The Go backend serves as a standalone API service alongside the CLI.

---

## Pre-release and Release Channel Support

Add pre-release versioning (alpha, beta, rc) and release channels (stable, canary, next). Support promoting pre-releases through channels with governance gates at each stage. Track channel-specific policies (e.g., canary requires lower approval threshold than stable). This addresses the PRD's "Missing critical workflows (pre-release)" gap identified in the product strategy review.

---

## Multi-Repository Governance

Coordinate release governance across multiple separate repositories that form a product or platform. Define repository groups with dependency relationships, synchronize version constraints across repos, enforce cross-repo release policies (e.g., "service-api must release before service-client"), and provide a unified view of multi-repo release state. Support both centralized governance (one Relicta instance governs all repos) and federated governance (each repo has Relicta, coordinated via CGP messages). Integrate with the existing CGP protocol for cross-repo proposal/decision exchange.

---

## Runtime Observability Integration

Connect post-deployment outcomes back to the release memory system for continuous learning. Integrate with observability platforms (Prometheus, Datadog, PagerDuty, Sentry) to automatically detect incidents, error rate spikes, and rollbacks that correlate with specific releases. Auto-record outcomes in the release memory when deployment metrics cross configured thresholds. Surface deployment health in the dashboard with release-correlated metrics. This closes the feedback loop so Relicta's risk scoring improves with real production data rather than relying solely on code analysis.

---
