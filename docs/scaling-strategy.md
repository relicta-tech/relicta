# Scaling Strategy: From CLI to Governance Platform

## Executive Summary

Relicta's scaling path follows three strategic themes from the vision: **Risk-Awareness**, **Agent Identity**, and **Universal Ledger**. Each theme builds on the existing production-ready CLI and extends it toward an organization-wide governance platform. This document maps the concrete steps from where Relicta is today to where it's going.

---

## Current Foundation

What's already built and production-ready:

| Capability | Status | Scale Readiness |
|------------|--------|-----------------|
| CGP v0.1.0 (local) | Shipped | Protocol defined, needs wire format standardization |
| Policy DSL with testing | Shipped | Matrix testing, sharding, what-if comparison done |
| 7-factor risk scoring | Shipped | Needs cross-repo aggregation |
| Release memory (file + PostgreSQL) | Shipped | Needs org-level federation |
| MCP server (stdio + HTTP) | Shipped | Needs multi-agent orchestration |
| Multi-repo federation | Shipped | Needs org-level coordination hub |
| Monorepo workspace detection | Shipped | Go, npm, Cargo supported |
| SLSA attestation + Sigstore | Shipped | Needs attestation registry |
| Dashboard (REST + WebSocket + SSE) | Shipped | Needs org-level views |
| Plugin system (30 official) | Shipped | Needs marketplace |
| OIDC/SSO | Shipped | Needs org-level RBAC |

---

## Theme 1: Risk-Awareness

**From "pass/fail" tests to probabilistic risk scoring.**

### Phase 1: Enhanced Risk Intelligence (Q2 2026)

**Goal:** Risk scores that predict outcomes, not just measure inputs.

#### 1.1 Outcome-Based Risk Calibration

The risk calculator currently uses static weights (API Changes 25%, Blast Radius 20%, etc.). Calibrate weights against actual outcomes by correlating risk scores with post-release incidents.

```
Current:  risk_score = weighted_sum(7 static factors)
Target:   risk_score = calibrated_model(7 factors, historical_outcomes)
```

**Implementation:**
- Add `release_outcome` field to Release Memory records (success, rollback, incident, hotfix)
- Build calibration pipeline: collect (score, outcome) pairs → adjust weights via logistic regression
- Run calibration weekly or on-demand via `relicta calibrate`
- Store calibrated weights in `.relicta/models/risk-weights.json`
- Fall back to defaults when insufficient data (<50 releases)

**Success metric:** Risk scores >0.7 should correlate with >60% of actual incidents.

#### 1.2 Predictive Risk Patterns

Use Release Memory to detect risk patterns that static factors miss.

**Patterns to detect:**
- **Friday deployments** — correlate day-of-week with incident rate
- **Actor fatigue** — high release frequency by single actor correlates with increased risk
- **Dependency chain risk** — changes to deeply-depended-on packages have outsized blast radius
- **Seasonal risk** — end-of-quarter, holiday freezes, team capacity changes
- **Cascading failures** — releases that follow other recent releases in dependent repos

**Implementation:**
- Add `PatternDetector` service that runs during `plan` phase
- Surface patterns as warnings in CLI output and MCP responses
- Feed patterns into risk calculator as an 8th factor: "contextual risk"

#### 1.3 Risk Dashboards

Extend the dashboard with risk-focused views.

**Views:**
- Risk distribution over time (histogram of risk scores across releases)
- Actor risk profiles (per-actor risk score trends)
- Package risk heatmap (which packages consistently carry higher risk)
- Risk vs. outcome scatter plot (calibration visualization)

### Phase 2: Organization-Wide Risk Intelligence (Q3 2026)

#### 2.1 Cross-Repo Risk Aggregation

When multiple repos are releasing simultaneously, aggregate risk across the organization.

```
Org risk = max(individual risks) + correlation_penalty(concurrent releases)
```

**Implementation:**
- Extend multi-repo federation to share risk scores via CGP messages
- Add `relicta org risk` command for org-level risk assessment
- Surface org-level risk in dashboard

#### 2.2 Risk Budgets

Allow organizations to set risk budgets per time period.

```yaml
governance:
  risk_budget:
    weekly_limit: 5.0    # total risk score budget per week
    concurrent_limit: 2  # max concurrent releases with risk > 0.5
    freeze_periods:
      - start: "Friday 16:00"
        end: "Monday 09:00"
        max_risk: 0.3
```

**Implementation:**
- Track cumulative risk score per org/team per period
- Block releases that would exceed budget (require override approval)
- Surface remaining budget in CLI and dashboard

#### 2.3 External Risk Signal Integration

Ingest external signals into risk calculation.

**Sources:**
- **PagerDuty/Opsgenie** — active incidents increase release risk
- **Datadog/Grafana** — anomalous metrics during change window
- **GitHub Security Advisories** — known vulnerabilities in dependencies
- **Calendar/HR systems** — team capacity, on-call rotation

**Implementation:**
- Webhook receiver already exists — extend with risk signal parsing
- Add `external_risk_signals` factor to risk calculator
- MCP tool: `relicta.ingest_signal` for programmatic signal injection

---

## Theme 2: Agent Identity

**Cryptographically signing changes with agent identities.**

### Phase 1: Actor Trust Framework (Q2 2026)

**Goal:** Every actor (human or agent) has a verifiable identity and earned trust level.

#### 3.1 Actor Identity Registry

Currently, actor trust is tracked per-repo in Release Memory. Scale to organization-level identity.

```yaml
actors:
  - id: "claude-code@team-platform"
    type: agent
    trust_score: 0.87
    total_releases: 142
    success_rate: 0.96
    permissions:
      - auto_approve_patch: true
      - auto_approve_minor: risk_score < 0.3
      - auto_approve_major: false
```

**Implementation:**
- Actor registry as a shared data store (PostgreSQL or dedicated service)
- Actor identity format: `name@scope` (e.g., `claude@org`, `ci-bot@team-platform`)
- Trust scores aggregate across repos (weighted by recency)
- OIDC token claims map to actor identity

#### 3.2 Agent Capability Certificates

Issue short-lived certificates to agents that encode their approved capabilities.

```
Certificate:
  subject: claude-code@team-platform
  issuer: relicta-governance
  capabilities:
    - plan: always
    - bump: patch, minor
    - notes: always
    - approve: risk_score < 0.4
    - publish: never (requires human)
  valid_until: 2026-04-01T00:00:00Z
  signature: <sigstore-signed>
```

**Implementation:**
- `relicta agent issue-cert` command for admins
- Certificate validation in CGP proposal handler
- Short-lived certs (24h-7d) to limit blast radius of compromised agents
- Revocation list for emergency agent deauthorization

#### 3.3 Agent Audit Trail

Every agent action must be attributable and traceable.

**Implementation:**
- Extend CGP audit chain with agent identity fields
- Agent actions include: tool invocations, decisions, approvals, overrides
- Dashboard view: "Agent Activity" showing per-agent action timeline
- Alert on anomalous agent behavior (e.g., agent approving its own changes)

### Phase 2: Cross-Agent Governance (Q4 2026)

#### 3.4 Multi-Agent Orchestration

As MCP v2 enables multi-agent communication, Relicta becomes the coordination point.

**Scenarios:**
- Agent A writes code → Agent B reviews → Relicta governs the release
- Multiple agents propose changes to the same package → Relicta serializes
- Agent proposes release → Relicta queries security agent for vulnerability scan → decision

**Implementation:**
- MCP v2 agent-to-agent messaging via Relicta as hub
- CGP proposal includes `upstream_agents` field for chain-of-custody
- Conflict resolution for concurrent agent proposals
- Agent collaboration patterns: sequential pipeline, parallel review, consensus

#### 3.5 Agent Reputation System

Trust scores based on verifiable outcomes, not just configuration.

**Metrics per agent:**
- Release success rate (no rollbacks within 24h)
- Risk prediction accuracy (predicted risk vs actual outcome)
- Time-to-detection (how quickly agent detects issues)
- Code quality signal (downstream test pass rates)

**Implementation:**
- Continuous scoring from Release Memory data
- Reputation decay for inactive agents
- Reputation boost for agents that catch issues early
- Organization can set minimum reputation thresholds for auto-approval

---

## Theme 3: Universal Ledger

**A decentralized audit log of every change decision.**

### Phase 1: Org-Level Governance Ledger (Q3 2026)

**Goal:** Single source of truth for all governance decisions across the organization.

#### 4.1 Centralized Governance Store

Aggregate all release governance data from individual repos into a shared store.

**Implementation:**
- PostgreSQL backend (already exists) as the central store
- `relicta sync` pushes local governance data to central store
- Dashboard connects to central store for org-level views
- API for querying governance history across repos

#### 4.2 Governance Analytics

Organization-wide analytics on governance patterns.

**Metrics:**
- Mean time to release (from plan to publish)
- Approval bottleneck analysis (which approvers are slowest)
- Policy effectiveness (which policies block the most, which are never triggered)
- Risk trend analysis (is the org shipping riskier over time?)
- Compliance coverage (% of releases with full governance trail)

**Dashboard views:**
- Org-level release calendar
- Team comparison (release frequency, risk, success rate)
- Policy impact analysis
- Compliance report generation

#### 4.3 Governance Compliance Reports

Auto-generate compliance reports from governance data.

**Formats:**
- SOC 2 Type II evidence (change management controls)
- ISO 27001 Annex A.12 (change management)
- DORA metrics (deployment frequency, lead time, MTTR, change failure rate)
- Custom report templates

**Implementation:**
- `relicta report generate --format soc2 --period 2026-Q1`
- Template-based report generation from governance data
- Export to PDF, Markdown, JSON

### Phase 2: Federated Governance (2027)

#### 4.4 Cross-Organization CGP Federation

Enable governance across organizational boundaries — e.g., vendor releases that affect your supply chain.

**Implementation:**
- CGP messages include organization identity
- Trust federation: Org A trusts Org B's governance decisions for shared dependencies
- Selective disclosure: share governance metadata without revealing internal policies
- Public attestation registry: organizations publish attestations for their releases

#### 4.5 Immutable Governance Ledger

Move from centralized PostgreSQL to a tamper-evident, append-only ledger.

**Options:**
- **Merkle tree log** (similar to Certificate Transparency) — append-only, cryptographically verifiable
- **Rekor-compatible transparency log** — leverage existing Sigstore infrastructure
- **Git-based ledger** — governance events as signed git commits in a dedicated repo

**Recommended approach:** Rekor integration, since Relicta already uses Sigstore for attestation.

**Implementation:**
- Each governance decision produces a signed log entry
- Entries submitted to a transparency log (Rekor or self-hosted)
- Verification: any party can prove a governance decision existed at a specific time
- Inclusion proofs for auditors

#### 4.6 Supply Chain Governance

Extend governance to cover dependency updates, not just internal changes.

**Implementation:**
- Monitor dependency updates (Dependabot, Renovate alerts)
- Apply CGP risk assessment to dependency changes
- Policy: "dependency updates with CVE fixes auto-approve; major dependency bumps require human approval"
- SBOM diff: compare SBOM before/after to quantify supply chain change

---

## Scaling Infrastructure

### Deployment Models

| Model | Target | When |
|-------|--------|------|
| **CLI (current)** | Individual developers, small teams | Now |
| **CLI + PostgreSQL** | Teams sharing state | Now |
| **CLI + Dashboard** | Teams needing visibility | Now |
| **Org Hub** | Central governance for multi-team | Phase 1 |
| **Federated Hub** | Cross-org governance | Phase 2 |

### Org Hub Architecture

```
┌─────────────────────────────────────────────────────┐
│                    Org Hub                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │ Governance│  │   Risk   │  │  Actor   │          │
│  │   Store   │  │ Aggregator│ │ Registry │          │
│  └──────────┘  └──────────┘  └──────────┘          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │ Dashboard │  │ Compliance│ │  Alert   │          │
│  │   (org)   │  │  Reports │ │  Engine  │          │
│  └──────────┘  └──────────┘  └──────────┘          │
├─────────────────────────────────────────────────────┤
│                    CGP / MCP                         │
├─────────────────────────────────────────────────────┤
│  Repo A CLI    Repo B CLI    Repo C CLI    ...      │
└─────────────────────────────────────────────────────┘
```

### Growth Metrics

Track these to measure scaling progress:

| Metric | Current | Phase 1 Target | Phase 2 Target |
|--------|---------|----------------|----------------|
| Repos governed | 1-10 | 50-200 | 500+ |
| Releases/month | 10-50 | 500-2000 | 10,000+ |
| Active agents | 1-2 | 5-20 | 50+ |
| Risk prediction accuracy | Baseline | 60% | 80% |
| Compliance coverage | Manual | 90% automated | 99% automated |
| Mean time to release | Variable | <10min | <5min |

---

## Go-To-Market Scaling

### Open Source (Current)

- MIT-licensed CLI
- Community plugins
- Public CGP specification
- Documentation site

### Community (Phase 1)

- CGP adopters program — help teams implement the protocol
- Plugin marketplace — community-contributed plugins
- Policy library — shared governance policies for common scenarios
- Integration guides — CI/CD, cloud providers, observability tools

### Enterprise (Phase 2)

- Org Hub (self-hosted or managed)
- Advanced analytics and compliance reporting
- Priority support and SLAs
- Custom policy development
- Training and certification

### Ecosystem (Phase 3)

- CGP SDK for third-party implementations
- Agent certification program
- Governance-as-a-Service API
- Industry-specific policy packs (fintech, healthcare, government)
- Academic research partnerships

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| CGP doesn't get adopted as a standard | Keep CGP tightly coupled to Relicta value prop; standard is a multiplier, not a dependency |
| MCP v2 changes break integration | Maintain backward compat; Relicta's MCP layer is thin and adaptable |
| Enterprise features dilute OSS community | Core CLI stays MIT; enterprise features are additive, not gated |
| Risk models overfit to small datasets | Require minimum sample size (50+ releases); fall back to static weights |
| Agent identity becomes a compliance burden | Start simple (OIDC claims); evolve to certificates only when demand exists |
