# Product Requirements Document: Relicta Hub

**Version:** 1.0.0
**Status:** Draft
**Date:** 2026-03-21
**Author:** Product Strategy

---

## 1. Executive Summary

Relicta Hub is the commercial SaaS governance control plane that aggregates Change Governance Protocol (CGP) data from Relicta CLI instances running across an organization's repositories. Where the CLI governs individual releases at the repo level — scoring risk, enforcing policy, capturing approvals, and producing audit trails — the Hub surfaces that data org-wide: unified release timelines, aggregated risk dashboards, DORA metric reporting, compliance evidence exports, and a centralized approval queue spanning every team. The Hub does not replace the CLI; it makes the governance work the CLI already does visible, measurable, and auditable at the organizational level.

**Target market:** Engineering organizations with 10 or more repositories operating under compliance requirements, deploying frequently with AI agents or CI automation in the critical path, and lacking a unified view of release risk and decision history. Primary buyers are VP Engineering, CTO, and Security/Compliance officers at SaaS companies, fintech, healthcare technology, and developer tooling companies in the 50–5,000 employee range.

**Revenue model:** SaaS subscription priced per connected repository. Three paid tiers — Team, Business, and Enterprise — layered on top of the free open-source CLI. Revenue expands organically as organizations connect more repositories.

---

## 2. Problem Statement

### 2.1 Governance data is siloed at the repository level

The Relicta CLI produces rich governance data per repository: risk scores, policy decisions, audit trails, actor reputation scores, and compliance reports. Organizations with 20, 50, or 200 repositories cannot aggregate this data today. Each team sees only its own releases. An engineering leader trying to understand the organization's overall release risk, approval bottlenecks, or DORA metrics must manually collect and correlate data from dozens of CLI instances. This is operationally infeasible at scale.

### 2.2 Compliance reporting requires manual aggregation

SOC 2, ISO 27001 Annex A.12, and DORA metric audits require evidence spanning all software changes across the organization, not individual repositories. Currently, generating this evidence requires exporting audit logs from each CLI instance, normalizing formats, and assembling a coherent picture — a process that takes days or weeks. Organizations under active compliance requirements cannot sustain this manually as their repository count grows.

### 2.3 AI agent oversight has no organizational visibility

The CLI already tracks actor reputation for AI agents, CI systems, and human developers at the repo level. But when an organization deploys AI coding assistants that propose changes across dozens of repositories, no centralized system tracks their aggregate behavior, cumulative risk contribution, or cross-repo anomalies. This creates a blind spot precisely where governance oversight is most critical.

### 2.4 Approval workflows are invisible across team boundaries

High-risk releases requiring human approval today are managed through CLI prompts or the embedded per-repo dashboard. When a release manager is responsible for releases across multiple teams, they have no single queue showing pending approvals. Similarly, escalations — a release exceeding the risk budget, an agent proposing a major version autonomously — have no org-level notification path.

### 2.5 Risk intelligence degrades without cross-repo data

The CLI's outcome-based risk calibration (correlating predicted risk scores with post-release incidents) learns from a single repository's release history. Most repositories have too few releases to produce statistically meaningful calibration. Pooling release outcomes across the organization would dramatically improve the accuracy of risk predictions for every connected repository.

---

## 3. Product Vision

Relicta Hub is the organizational governance control plane for software change. It answers the questions that the CLI cannot answer alone: What is our organization's overall release risk right now? Which teams are shipping most frequently and most safely? Are our AI agents behaving within policy boundaries across all repositories? Are we prepared for a SOC 2 audit next quarter?

The relationship between CLI and Hub is deliberately layered. The CLI is free, MIT-licensed, and self-contained — a standalone tool that delivers immediate value to individual teams. The Hub is the paid aggregation layer that multiplies that value across the organization. CLI users who connect to the Hub gain org-level visibility. CLI users who do not connect lose nothing; their local governance continues unchanged. This bottom-up adoption model means the Hub earns its place in an organization only after the CLI has already demonstrated value.

The Hub does not move governance logic to the cloud. Proposals, risk evaluation, and policy enforcement remain local to the CLI. The Hub receives governance events after decisions are made, aggregates them, and surfaces intelligence that only becomes visible at organizational scale.

---

## 4. Target Personas

### 4.1 VP Engineering / CTO

Needs a real-time view of organizational release health. Cares about deployment frequency, change failure rate, mean time to recovery, and whether AI agents are operating within sanctioned boundaries. Does not look at individual release details. Wants weekly trend summaries and exception alerts. Success looks like: "I can see that our change failure rate improved from 8% to 3% this quarter, and I can point to the governance controls that drove it."

### 4.2 Platform Engineering Lead

Responsible for the release infrastructure the organization runs on. Evaluates Relicta Hub for purchase. Needs confidence that the Hub integrates cleanly with existing CI/CD pipelines, does not introduce a critical path dependency, and scales to the organization's full repository count. Cares deeply about the sync protocol, API surface, and operational overhead of running the Hub. Likely a CLI power user already.

### 4.3 Security and Compliance Officer

Needs audit evidence on demand. Responsible for producing SOC 2 Type II control evidence, responding to auditor requests about change management, and maintaining an up-to-date view of supply chain risk. Currently spends 2–3 weeks manually assembling evidence before each audit. Success looks like: "Generate the last 12 months of change management evidence in 10 minutes."

### 4.4 Release Manager

The primary daily user of the Hub dashboard. Monitors pending approvals across all repositories, reviews high-risk releases before they ship, and tracks release outcomes. Often embedded in a specific product area but responsible for coordinating releases across multiple teams. Success looks like: "I wake up, check the approval queue, approve two releases before standup, and flag one that needs a security review."

### 4.5 Engineering Manager

Needs per-team and per-repo visibility into release frequency, risk trends, and actor reliability. Uses the Hub in 1:1s and team retrospectives to understand delivery health. Does not need compliance exports but wants to know which engineers or agents are introducing elevated risk. Success looks like: "I can see that my team's average release risk score has trended up over the last two sprints and drill into why."

---

## 5. User Stories

### 5.1 VP Engineering / CTO Stories

**US-001** — As a VP Engineering, I want to see a single dashboard with deployment frequency, change failure rate, mean time to recovery, and lead time for changes across all of our repositories so that I can report DORA metrics to the board without manually collecting data.

**US-002** — As a CTO, I want to receive an alert when any AI agent in the organization proposes a major version release autonomously so that I can ensure human oversight is maintained for significant changes.

**US-003** — As a VP Engineering, I want to see a week-over-week trend of organizational release risk so that I can identify whether governance is improving or degrading over time.

### 5.2 Platform Engineering Lead Stories

**US-004** — As a Platform Engineering Lead, I want to connect a new repository to the Hub by running a single CLI command so that onboarding a repository takes less than five minutes.

**US-005** — As a Platform Engineering Lead, I want the CLI sync to operate offline-resiliently — queuing events locally when the Hub is unreachable and retrying automatically — so that a Hub outage never blocks a production release.

**US-006** — As a Platform Engineering Lead, I want to manage API keys and rotate them without downtime so that I can follow our organization's key rotation policy without interrupting governance sync.

**US-007** — As a Platform Engineering Lead, I want to configure which governance events are synced to the Hub — proposals, decisions, approvals, outcomes — so that I can comply with data residency requirements by excluding specific content.

### 5.3 Security and Compliance Officer Stories

**US-008** — As a Security and Compliance Officer, I want to generate a SOC 2 Type II change management evidence package for any 12-month period in under 10 minutes so that I can respond to auditor requests without engineering involvement.

**US-009** — As a Security and Compliance Officer, I want to export a DORA metrics report in PDF and CSV formats so that I can include it in board-level and auditor-facing materials.

**US-010** — As a Security and Compliance Officer, I want to verify the cryptographic integrity of the audit chain before exporting compliance evidence so that I can attest to its tamper-free status.

**US-011** — As a Security and Compliance Officer, I want the Hub to alert me when a release is approved outside of its authorized time window so that I can investigate potential policy violations.

### 5.4 Release Manager Stories

**US-012** — As a Release Manager, I want a unified approval queue showing all pending releases across every repository I manage so that I can process approvals from a single interface rather than switching between CLI contexts.

**US-013** — As a Release Manager, I want to see the full risk breakdown, diff summary, and actor reputation score for any pending release before I approve or reject it so that my decision is informed.

**US-014** — As a Release Manager, I want to receive a Slack notification when a release with a risk score above 0.7 enters the approval queue so that I do not miss high-risk releases.

**US-015** — As a Release Manager, I want to configure freeze periods at the organization level that apply to all connected repositories so that I do not need to update each repository's `.relicta.yaml` individually.

### 5.5 Engineering Manager Stories

**US-016** — As an Engineering Manager, I want to see per-actor reliability scores for all human and agent actors in my team's repositories so that I can identify where additional oversight or coaching is needed.

**US-017** — As an Engineering Manager, I want to see a release calendar view showing what my team shipped each week, the risk score of each release, and whether any rollbacks occurred so that I can use real delivery data in retrospectives.

**US-018** — As an Engineering Manager, I want to compare my team's DORA metrics against the organizational average so that I can understand where we are relative to peers.

---

### 5.1 Acceptance Criteria for Top 5 Stories

#### US-001: DORA Metrics Dashboard

- The dashboard renders all four DORA metrics (deployment frequency, lead time for changes, change failure rate, mean time to recovery) using data synced from connected repositories.
- Metrics are filterable by time range (7 days, 30 days, 90 days, 12 months) and by team or repository.
- Deployment frequency is calculated as deploys per day, week, or month, automatically bucketed based on the selected time range.
- Change failure rate is the percentage of releases that resulted in a rollback or incident correlation within 24 hours of publish.
- Lead time is calculated as the time from first commit in a release to the publish timestamp.
- MTTR is calculated as the time from incident correlation event to the next successful release on the same repository.
- All four metrics display their DORA performance band (Elite, High, Medium, Low) using the 2023 State of DevOps benchmarks.

#### US-004: Repository Onboarding via CLI

- Running `relicta sync init --hub https://hub.relicta.dev --api-key <key>` completes in under 60 seconds on a standard development machine.
- The command creates a `.relicta.yaml` stanza for Hub sync without overwriting existing configuration.
- After initialization, all subsequent `relicta publish` executions automatically push governance events to the Hub.
- The repository appears in the Hub dashboard within 60 seconds of the first successful sync.
- A clear error message is displayed if the API key is invalid, the Hub is unreachable, or the repository is already connected.

#### US-008: SOC 2 Evidence Export

- The evidence package includes: a complete release log for the specified period, all governance decisions with actor attribution, all approval records with timestamps and justifications, the risk score for each release, and the audit chain integrity verification result.
- The package is downloadable as a ZIP archive containing a human-readable PDF summary and machine-readable JSON data files.
- The export completes in under 60 seconds for organizations with up to 10,000 releases in the period.
- The PDF includes a cover page with the organization name, period covered, and generation timestamp.
- The system refuses to generate the report if audit chain integrity verification fails, displaying the first entry where tampering was detected.

#### US-012: Unified Approval Queue

- The approval queue displays all pending releases across all repositories the user has permission to approve.
- Each queue entry shows: repository name, release version, risk score with severity band, actor identity, time in queue, and the triggering policy rule.
- Entries are sorted by risk score descending by default, with secondary sort by time in queue ascending.
- Approving or rejecting a release from the Hub sends the decision back to the originating CLI instance via a webhook within 30 seconds.
- The queue updates in real time via WebSocket; a new pending approval appears without page refresh.
- The user can filter the queue by repository, risk severity, actor type (human, agent, CI), and time in queue.

#### US-014: High-Risk Release Alert

- When a release with a risk score at or above the configured threshold (default 0.7, configurable per organization) enters the approval queue, a notification is sent to all configured channels within 60 seconds.
- Supported channels at launch: Slack (via webhook), email.
- The notification includes: repository name, release version, risk score, the highest-weighted risk factors, and a direct link to the approval queue entry in the Hub.
- Notification thresholds and channels are configurable per organization by users with the Admin role.
- A user can mute notifications for a specific release without affecting global alert configuration.

---

## 6. Functional Requirements

### 6.1 Core Platform

#### 6.1.1 Organization and Team Management

- An organization is the top-level tenant. All data, settings, and billing belong to an organization.
- Organizations contain teams. A team is a logical grouping of repositories and users.
- Repositories are associated with one or more teams.
- Users belong to one organization. Users may belong to multiple teams within that organization.
- Organization creation triggers provisioning of an isolated PostgreSQL schema and a default admin user.
- Organizations are deletable. Deletion is a two-step process requiring an explicit confirmation code, and results in permanent deletion of all associated data after a 30-day grace period.

#### 6.1.2 Repository Connection

- Repositories are connected to the Hub via the `relicta sync init` CLI command, which registers the repository and returns a scoped API key.
- Each connected repository stores its Hub configuration in `.relicta.yaml` under a `hub:` stanza.
- The Hub tracks connection health: last sync timestamp, events received in the last 24 hours, and sync error count.
- Repository disconnection is available from the Hub UI. Disconnecting stops accepting new events but preserves historical data.
- A repository may be connected to only one Hub organization at a time.

#### 6.1.3 User Management and RBAC

| Role | Permissions |
|---|---|
| Admin | Full organization management: user management, billing, global settings, all data access, API key management |
| Manager | Read all data, manage approval queues, configure alerts, view reports, generate compliance exports |
| Viewer | Read-only access to dashboards, release history, and analytics |

- Users are invited by email. Invitation links expire after 7 days.
- SAML 2.0 and OIDC SSO is supported on Business and Enterprise tiers. When SSO is enabled, user provisioning is automatic on first login.
- SCIM 2.0 provisioning is supported on the Enterprise tier for automated user lifecycle management.
- MFA is supported via TOTP. MFA enforcement per-organization is configurable by Admins.
- Session tokens expire after 8 hours of inactivity. Session duration is configurable on Business and Enterprise tiers.

#### 6.1.4 SSO and OIDC

- The Hub acts as an OIDC relying party. Any standards-compliant OIDC provider is supported.
- Supported identity providers at launch: Okta, Google Workspace, Microsoft Entra ID, GitHub.
- Attribute mapping (email, name, group-to-team mapping) is configurable per organization.
- SSO is optional; email/password authentication is always available as a fallback for Admins.

### 6.2 Governance Dashboard

#### 6.2.1 Organization-Wide Release Timeline

- A chronological timeline of all releases across all connected repositories, showing version, repository, actor, risk score, and outcome.
- Filterable by repository, team, actor, risk severity, and date range.
- Releases with open approvals are visually distinguished.
- Rollbacks and incident-correlated releases are flagged.

#### 6.2.2 Per-Repository Release Status

- A card or list view of all connected repositories showing: current release state (if a release is in progress), last release version and timestamp, 7-day release count, and 7-day average risk score.
- Clicking a repository opens a detail view showing full release history, actor activity, and governance stats for that repository.

#### 6.2.3 Risk Aggregation View

- Organization-level risk score: the rolling weighted average of risk scores across all releases in the last 30 days.
- Risk budget utilization: consumed vs. configured weekly risk budget.
- Top risk contributors: the five repositories or actors contributing the most to organizational risk.
- Concurrent high-risk releases: an alert banner when two or more releases with risk scores above 0.6 are simultaneously in the pipeline.

#### 6.2.4 Actor Reputation Leaderboard

- A sortable table of all actors (human and agent) who have proposed or approved releases across the organization.
- Columns: actor name, type (human, agent, CI, system), total releases, success rate, average risk score, rollback count, reputation score.
- Filterable by actor type and team.
- Clicking an actor opens a detail view with their release history, risk contribution trend, and flagged anomalies.

#### 6.2.5 Approval Queue

- As specified in US-012 acceptance criteria.
- Bulk approval is supported for up to 10 low-risk (score below 0.3) releases simultaneously.
- Rejected releases display a mandatory rejection reason field. The reason is synced back to the originating CLI and included in the audit trail.

### 6.3 Analytics and Intelligence

#### 6.3.1 DORA Metrics Dashboard

- As specified in US-001 acceptance criteria.
- Trend charts for each metric showing the last 12 months by default with monthly granularity.
- Drill-down from any metric to the specific releases that contributed to it.
- Export to CSV and PDF.

#### 6.3.2 Risk Trend Analysis

- Time-series chart of average risk score per week across the organization.
- Risk factor breakdown: a stacked area chart showing which of the seven CGP risk factors (API changes, blast radius, dependency impact, security impact, historical risk, actor trust, test coverage) have been driving risk over time.
- Anomaly detection: automated flagging of weeks where risk score deviates more than 1.5 standard deviations from the rolling 12-week mean.

#### 6.3.3 Calibration Accuracy Tracking

- For organizations with at least 50 releases and outcome data, display risk score calibration accuracy: the correlation coefficient between predicted risk score and actual outcome (success, rollback, incident).
- A calibration accuracy score below 0.5 triggers a recommendation to run `relicta calibrate` with the accumulated outcome data.
- Trend chart showing calibration accuracy over time as the model learns.

#### 6.3.4 Actor Performance Comparison

- Side-by-side comparison of up to five actors across key metrics: release count, success rate, average risk, lead time, and rollback rate.
- Breakdown by actor type: compare all agents as a cohort against all humans or all CI systems.
- Exportable as CSV for inclusion in performance reviews or team health reports.

#### 6.3.5 Supply Chain Risk Overview

- A table of all dependency changes detected across connected repositories in the last 30 days.
- Each entry shows: repository, dependency name, old version, new version, known CVEs in the new version (from OSV), and the CGP risk score assigned to the change.
- Filterable by severity and repository.
- Integrates with the existing supply chain governance feature in the CLI.

### 6.4 Compliance

#### 6.4.1 DORA Report Generation

- One-click DORA metrics report covering deployment frequency, lead time, change failure rate, and MTTR for a configurable period.
- Report includes methodology notes explaining how each metric is calculated from CGP data.
- Available formats: PDF, CSV, JSON.
- Reports are stored in the Hub for 12 months and accessible to users with Manager or Admin roles.

#### 6.4.2 SOC 2 Evidence Export

- As specified in US-008 acceptance criteria.
- The evidence maps to SOC 2 Type II Common Criteria CC8.1 (change management).
- Evidence package includes: release log, approval records, policy rule configurations at the time of each decision, audit chain verification result, and a signed attestation from the Hub.
- A dedicated SOC 2 evidence section in the Hub UI guides compliance officers through what evidence is available and what the audit chain verification status is.

#### 6.4.3 Custom Report Builder

- A drag-and-drop report builder allowing users to compose custom reports from available data dimensions: repositories, time period, actor filters, risk thresholds, and outcome filters.
- Custom reports are saveable and shareable within the organization.
- Custom reports can be exported as PDF, CSV, or JSON.
- Available on Business and Enterprise tiers.

#### 6.4.4 Scheduled Report Delivery

- Reports — both built-in (DORA, SOC 2) and custom — can be scheduled for automatic delivery.
- Delivery options: email (PDF attachment) and webhook (JSON payload).
- Schedules: daily, weekly, monthly, quarterly.
- Scheduled reports are listed in the organization settings with their last delivery status.

### 6.5 Alerting and Notifications

The following alert types are configurable per organization:

| Alert | Trigger | Default Threshold |
|---|---|---|
| High-risk release | Release enters queue with risk score above threshold | 0.7 |
| Risk budget exceeded | Weekly risk budget consumption exceeds configured limit | 90% of budget |
| Freeze period violation | Release proposed during a configured freeze period | Any proposal |
| Actor reputation change | Actor's reputation score drops more than 0.1 in 7 days | 0.1 drop |
| Incident correlation | A release is automatically correlated with an active incident | Any correlation |
| Agent anomaly | An agent proposes a major version or a release outside its authorized capability scope | Any |
| Audit chain failure | Integrity verification fails for a connected repository's audit chain | Any failure |

- Notification channels: email, Slack (webhook), Microsoft Teams (webhook), and PagerDuty (API key integration) at launch.
- Alert routing is configurable: different alert types can route to different channels.
- Alert suppression windows allow muting specific alert types during known maintenance periods.

### 6.6 CLI Integration

#### 6.6.1 `relicta sync` Command Set

- `relicta sync init` — registers a repository with the Hub, writes Hub configuration to `.relicta.yaml`.
- `relicta sync push` — manually pushes all local governance events since the last successful sync.
- `relicta sync status` — shows the last sync timestamp, pending event count, and connection health.
- `relicta sync disconnect` — removes Hub configuration from `.relicta.yaml` and deregisters the repository.

#### 6.6.2 Automatic Sync

- By default, governance events are pushed to the Hub automatically after each `relicta publish` execution.
- The sync runs as a non-blocking background goroutine. It does not delay or block the publish pipeline.
- Automatic sync is configurable; organizations may require explicit `relicta sync push` instead.

#### 6.6.3 Offline Resilience

- When the Hub is unreachable, governance events are persisted to a local queue in `.relicta/hub-queue/`.
- The queue is drained on the next successful sync attempt, which is retried with exponential backoff (initial delay 30s, maximum delay 1h, maximum queue age 7 days).
- Events older than 7 days are discarded from the queue with a warning. The CLI logs the event IDs of discarded events for manual recovery.
- Queue size is bounded at 10,000 events. When the bound is exceeded, the CLI logs a warning but continues operating.

#### 6.6.4 API Key Authentication

- Each connected repository uses a scoped API key. Keys are associated with a specific repository and cannot be used to access data from other repositories.
- Keys are shown once at creation and then stored as a bcrypt hash in the Hub database.
- Key rotation generates a new key and provides a 24-hour grace period where both old and new keys are valid.
- Keys can be revoked immediately from the Hub UI, after which the CLI will report an authentication error on the next sync attempt.

### 6.7 API

#### 6.7.1 REST API

- A versioned REST API (`/api/v1/`) exposes all Hub data: organizations, repositories, releases, governance decisions, actors, reports, and alerts.
- Authentication: Bearer token (user-scoped) or API key (repository-scoped or organization-scoped service key).
- All endpoints are paginated using cursor-based pagination with a default page size of 50 and a maximum of 200.
- Rate limiting: 1,000 requests per minute per API key, with a 429 response and `Retry-After` header when exceeded.
- The API follows the same resource model as the existing CLI dashboard API (`/api/v1/releases`, `/api/v1/governance/decisions`, etc.) to minimize integration friction.

#### 6.7.2 Webhook Outbound

- The Hub sends signed outbound webhooks for configurable event types.
- Payload is signed with HMAC-SHA256 using a per-organization secret. The signature is provided in the `X-Relicta-Signature` header.
- Webhook delivery uses retry with exponential backoff: up to 5 attempts over 24 hours.
- A webhook delivery log is maintained per endpoint, showing the last 100 delivery attempts with status codes and response times.
- Webhook endpoints are manageable from the Hub UI (Admin role required).

#### 6.7.3 GraphQL (Future)

- A GraphQL API is planned for v2 to support complex cross-entity queries from analytics consumers. Not in scope for MVP or V1.

---

## 7. Non-Functional Requirements

### 7.1 Performance

| Target | Requirement |
|---|---|
| Dashboard load time (p95) | < 2 seconds on a standard broadband connection |
| API response time (p95) | < 200ms for read endpoints; < 500ms for write endpoints |
| Sync ingestion throughput | Sustain 500 events/second per organization |
| Report generation | < 60 seconds for reports covering up to 10,000 releases |
| Real-time approval push | < 30 seconds from Hub approval to CLI acknowledgment |
| WebSocket event delivery | < 5 seconds from event ingestion to dashboard update |

### 7.2 Availability

- Business and Enterprise tiers: 99.9% monthly uptime SLA, excluding scheduled maintenance windows announced 48 hours in advance.
- Team tier: best-effort, no SLA guarantee.
- Scheduled maintenance windows are capped at 2 hours per calendar month and occur during off-peak hours (01:00–04:00 UTC).
- Degraded-mode operation: if the Hub experiences a partial outage, CLI sync queues locally and releases are not blocked. The Hub provides zero blast radius to the release pipeline.

### 7.3 Data Retention

| Data Type | Retention Period |
|---|---|
| Governance events (proposals, decisions, approvals) | 24 months (configurable up to 84 months on Enterprise) |
| Audit trail entries | 84 months (7 years, for SOC 2 and regulatory requirements) |
| Actor reputation data | 24 months rolling |
| Generated compliance reports | 12 months |
| Webhook delivery logs | 30 days |
| Sync queue (local CLI) | 7 days |

- Data export is available at any time for all tiers. Organizations can export all their data as JSON.
- On organization deletion, data is purged from production databases within 30 days and from backups within 90 days.

### 7.4 Security

- All data in transit is encrypted with TLS 1.3. TLS 1.2 is the minimum accepted.
- All data at rest is encrypted using AES-256. Encryption keys are managed using envelope encryption with a key management service.
- Per-organization data isolation: each organization's data resides in a dedicated PostgreSQL schema. Cross-schema access is prevented at the database layer.
- API keys and session tokens are stored as bcrypt hashes.
- The Hub maintains its own append-only audit log for all administrative actions (user management, billing changes, API key operations, report generation).
- Penetration testing is conducted annually. Critical and high findings are remediated within 30 and 90 days respectively.
- The Hub is operated to the SOC 2 Type II standard. The SOC 2 report is available to Enterprise customers under NDA.

### 7.5 Multi-Tenancy Isolation

- Tenant isolation is enforced at the PostgreSQL schema level. Each organization has a dedicated schema with a dedicated database role.
- Application-level query routing enforces that no cross-tenant data access is possible via the API layer.
- Resource quotas per organization prevent noisy-neighbor effects: event ingestion rate limits, storage limits, and API rate limits are enforced per organization.

---

## 8. Technical Architecture

### 8.1 Hub Server

- Written in Go 1.22+, reusing the existing internal service packages from the CLI: `internal/service/governance`, `internal/service/version`, `internal/service/git`, and the compliance report generator.
- Chi HTTP router with middleware for authentication, CORS, rate limiting, and structured logging — identical to the CLI dashboard's existing server stack.
- Modular service layer: each feature area (governance aggregation, actor registry, compliance reports, alerting) is a separate service implementing a clean interface.
- Graceful shutdown with connection draining on SIGTERM.

### 8.2 Frontend

- Vue 3 with TypeScript, reusing and extending the existing `web/` dashboard.
- Pinia for state management, Vue Router for navigation, Tailwind CSS for styling.
- The Hub frontend is a separate deployment from the embedded CLI dashboard but shares component libraries.
- Server-side rendering is not required at launch. SPA with CDN caching.

### 8.3 Database

- PostgreSQL 16 as the primary data store.
- Per-organization schema isolation. Schema provisioning is automated on organization creation.
- Migrations managed with a Go migration library (golang-migrate). Migrations run automatically on Hub startup.
- Read replicas for analytics and report generation queries to avoid contention with the write path.
- Connection pooling via PgBouncer in transaction mode.

### 8.4 Cache

- Redis 7 for: real-time aggregation state (org-level risk scores, live queue counts), session tokens, API rate limiting counters, and WebSocket pub/sub for multi-instance deployments.
- Redis is not used as a primary data store. All data in Redis is reconstructable from PostgreSQL.

### 8.5 Infrastructure

- Container-based deployment. Each service (Hub API, frontend, background workers) runs as a stateless Docker container.
- Kubernetes-orchestrated on managed Kubernetes (EKS or GKE). Horizontal Pod Autoscaler scales the Hub API on CPU and request rate.
- Cloud-agnostic: the Hub runs on any Kubernetes cluster. Enterprise self-hosted deployment is supported via Helm chart.
- Object storage (S3-compatible) for generated report files.
- A background worker process handles: sync event processing, alert evaluation, scheduled report generation, and risk calibration jobs. Worker uses a PostgreSQL-backed queue (no additional message broker required at launch).

### 8.6 CLI Sync Protocol

- HTTPS POST to `/api/v1/sync/events` with a JSON array of governance events.
- Authentication: `Authorization: Bearer <api-key>` header.
- Events are the same CGP audit trail entries the CLI already produces locally, serialized as JSON.
- The Hub validates event schema, verifies the audit chain hash for the submitted events, and stores them in the organization schema.
- Batch size: up to 100 events per request. The CLI splits larger queues into multiple requests.
- Idempotent: events are identified by their audit entry ID. Duplicate submissions are silently deduplicated.

---

## 9. Pricing and Packaging

### 9.1 Tier Definitions

| Feature | Open Source | Team | Business | Enterprise |
|---|---|---|---|---|
| CLI (full feature set) | Free | Free | Free | Free |
| Hub dashboard | — | Up to 5 repos | Unlimited repos | Unlimited repos |
| DORA metrics | — | Basic | Full with drill-down | Full with benchmarks |
| Unified approval queue | — | Included | Included | Included |
| SOC 2 evidence export | — | — | Included | Included |
| Custom report builder | — | — | Included | Included |
| Scheduled report delivery | — | — | Included | Included |
| SSO / OIDC | — | — | Included | Included |
| SCIM provisioning | — | — | — | Included |
| Self-hosted Hub | — | — | — | Included |
| SLA | — | None | 99.9% | 99.9% + custom |
| Support | Community | Email | Email + priority | Dedicated CSM |
| Audit log retention | — | 24 months | 24 months | Up to 84 months |
| Price | Free | $49/repo/month | $149/repo/month | Custom |

### 9.2 Pricing Notes

- A "repo" is one connected repository. Disconnected repositories are not billed.
- Annual billing receives a 20% discount on Team and Business tiers.
- The Team tier is capped at five connected repositories. Exceeding five repositories requires an upgrade to Business.
- Enterprise pricing is negotiated based on repository count, support requirements, and self-hosting needs. Volume discounts apply above 50 repositories.
- Free trial: 14 days of Business tier for any organization, no credit card required. After the trial, the organization is downgraded to Open Source (CLI only) unless a paid tier is activated.
- Non-profit and open source project pricing: 50% discount on Team and Business tiers, subject to verification.

---

## 10. Go-To-Market Strategy

### 10.1 Bottom-Up CLI Adoption

The primary acquisition channel is the CLI itself. Every user of the open-source CLI is a potential Hub subscriber. The CLI surfaces the Hub at natural upgrade moments:

- After the first `relicta release`, the CLI prints: "See this release in the Hub: `relicta sync init`."
- After generating a compliance report locally, the CLI suggests: "Generate and share this report from the Hub with one click."
- When a user runs `relicta history` and has more than 10 releases, the CLI suggests the Hub for trend analysis.

In-product upgrade prompts are opt-out and limited to one per CLI session.

### 10.2 Developer Advocacy

- Maintain an active presence in platform engineering, DevOps, and AI-assisted development communities.
- Regular blog content: case studies showing DORA metric improvements, tutorials on CGP policy authoring, and analysis of AI agent governance patterns in production.
- Conference talks and workshops at KubeCon, PlatformCon, and DevOpsDays targeting Platform Engineering Leads.
- A CGP adopters program: organizations that implement the Change Governance Protocol publicly are featured on the Hub website.

### 10.3 Content Marketing

- SEO-focused content targeting: "DORA metrics automation," "AI agent governance," "SOC 2 change management evidence," "release risk scoring."
- Weekly changelog: every Hub release is announced with a short blog post explaining the customer problem it solves.
- An open policy library: a GitHub repository of community-contributed CGP policy files covering common scenarios (fintech release controls, healthcare change windows, startup velocity policies).

### 10.4 Integration Partnerships

- GitHub: GitHub App for the Hub that displays governance status as a commit check and surfaces the Hub approval queue in GitHub's interface.
- GitLab: GitLab CI integration template that auto-installs the Relicta CLI and configures Hub sync.
- PagerDuty: Bidirectional integration — PagerDuty incidents trigger risk signal ingestion; Hub alerts can create PagerDuty incidents.
- Slack: Official Slack app for approval workflow management directly from Slack.

### 10.5 Enterprise Sales

- Enterprise sales motion activates at organizations with 20+ repositories or compliance requirements.
- Sales cycle entry points: security and compliance officers (SOC 2 use case), platform engineering leads (governance at scale), and engineering VPs (DORA metrics visibility).
- Sales-assisted trial with a dedicated onboarding session for organizations evaluating Business or Enterprise tiers.
- Self-hosted Enterprise option (Helm chart deployment) for organizations with data residency requirements.

---

## 11. Success Metrics

### 11.1 Activation

- CLI-to-Hub conversion rate: percentage of active CLI users (at least 5 releases in the last 30 days) who connect at least one repository to the Hub within 30 days of seeing the first Hub prompt. Target: 8% in Month 6, 15% in Month 12.
- Time-to-first-sync: time from `relicta sync init` to first event visible in the Hub dashboard. Target: under 5 minutes at p95.

### 11.2 Engagement

- Daily active dashboard users per organization: at least one user views the Hub dashboard on 15 of 30 days in a month. Target: 60% of paying organizations at 6 months.
- Approval queue utilization: percentage of pending approvals processed through the Hub (vs. directly via CLI). Target: 50% of approvals in Hub-connected organizations within 3 months of activation.

### 11.3 Revenue

- MRR target: $50,000 by Month 6; $200,000 by Month 12.
- ARPU (average revenue per organization): $490/month at Month 6 (approximately 10 repos per organization on Team tier).
- Expansion revenue: 30% of MRR growth at Month 12 should come from existing customers connecting additional repositories.
- Churn: less than 3% monthly gross revenue churn by Month 6.

### 11.4 Product Outcomes

- DORA improvement: organizations using the Hub for 6+ months show measurable improvement in at least two DORA metrics compared to their baseline at activation. Track via built-in benchmarking.
- Compliance efficiency: Security and Compliance Officer persona reports generate SOC 2 evidence packages in under 10 minutes (measured via support feedback and in-product timing).
- Risk prediction accuracy: the average calibration accuracy score across organizations with sufficient outcome data reaches 0.65 by Month 12.

---

## 12. Risks and Mitigations

### 12.1 Open Source Cannibalization

**Risk:** The CLI's local dashboard and compliance report generator already provide significant value without the Hub. Organizations may resist paying for the Hub when the CLI alone meets their needs.

**Mitigation:** The Hub's value is explicitly organizational — it requires multiple repositories to be meaningfully useful. Organizations with one or two repositories get minimal Hub value, which is expected. The pricing model reflects this: the Team tier starts at $49/repo, which is unattractive for a single-repo organization. The Hub is positioned as a multiplier on CLI value, not a gate on it. The CLI never loses features to create artificial Hub dependency.

### 12.2 Enterprise Self-Hosting Demand

**Risk:** Enterprise customers with strict data residency requirements will demand self-hosted Hub deployments before signing contracts, increasing operational complexity and support burden.

**Mitigation:** Self-hosted Hub is a first-class Enterprise tier offering, delivered via Helm chart and documented with production deployment guides. The Hub is designed to be cloud-agnostic from day one. A dedicated deployment guide and a 30-day assisted onboarding process for Enterprise self-hosted customers reduces support burden.

### 12.3 Competitor Response

**Risk:** GitHub, GitLab, or Atlassian could add governance aggregation and DORA metrics to their existing platforms, commoditizing the Hub's core value proposition.

**Mitigation:** The Hub's defensible differentiation is the CGP protocol and AI agent governance — neither GitHub nor GitLab has a native layer for governing AI-generated releases at the decision level. Invest in CGP adoption as an open standard, making the Hub the reference implementation rather than a proprietary solution. Build depth in AI agent governance that platforms cannot replicate without fundamental product changes.

### 12.4 Data Sensitivity Concerns

**Risk:** Organizations may resist syncing governance data — even metadata about what changed and who approved it — to a SaaS platform, particularly in regulated industries.

**Mitigation:** The sync protocol is designed to be selective: organizations can configure which event types sync, and the diff content, commit messages, and release notes are excluded from sync by default (only metadata). The self-hosted Enterprise tier addresses the most sensitive cases. Publish a clear data processing agreement (DPA) and the SOC 2 Type II report to remove blockers in the sales cycle.

### 12.5 Insufficient CLI Install Base at Launch

**Risk:** The bottom-up adoption model requires an active CLI user base. If CLI adoption is too thin at Hub launch, the conversion funnel has insufficient volume to reach revenue targets.

**Mitigation:** Hub launch should be preceded by at least 3 months of CLI-focused growth: developer advocacy, integrations, and community building. The Hub launch announcement should include case studies from beta customers who have been running the Hub internally. Launch with a generous 14-day Business trial that does not require a credit card.

---

## 13. Roadmap

### 13.1 MVP (Months 1–3): Foundation

**Goal:** Demonstrate the core value proposition to beta customers and validate the CLI-to-Hub conversion funnel.

- `relicta sync` command with API key authentication and offline queue
- Hub organization and team management with email/password authentication
- Repository connection and event ingestion pipeline
- Governance dashboard: release timeline, per-repo status, risk aggregation view
- Basic DORA metrics (deployment frequency and lead time, calculated from sync events)
- Unified approval queue with Slack notification integration
- SOC 2 evidence export (PDF + JSON)
- REST API covering releases, governance decisions, and actors
- Outbound webhooks for approval and alert events

**Success criteria for MVP exit:** 5 beta organizations running in production; Hub-to-CLI sync working reliably at 99%+ delivery rate; at least one customer has used the SOC 2 export in a real audit.

### 13.2 V1 (Months 4–6): Go-to-Market Readiness

**Goal:** Full feature set for Team and Business tiers. Public launch.

- Full DORA metrics dashboard with drill-down and DORA band classification
- SSO/OIDC integration (Okta, Google, Microsoft Entra ID)
- Risk trend analysis with anomaly detection
- Actor reputation leaderboard with cross-repo aggregation
- Custom report builder (Business tier)
- Scheduled report delivery
- Supply chain risk overview
- PagerDuty and email notification channels
- GitHub App integration (governance status as commit check)
- Audit chain integrity verification in compliance exports
- Self-service billing and subscription management
- Public pricing page and free trial flow

**Success criteria for V1 exit:** Public launch; $50,000 MRR within 60 days of launch.

### 13.3 V2 (Months 7–12): Enterprise and Intelligence

**Goal:** Close Enterprise deals, deepen intelligence features, and build the ecosystem.

- SCIM 2.0 provisioning for Enterprise tier
- Self-hosted Enterprise Hub via Helm chart
- Risk calibration data pooling: cross-organization anonymous risk outcome aggregation to improve calibration accuracy
- Org-level risk budget configuration and enforcement
- Multi-agent governance views: cross-repo agent activity, anomaly detection, capability certificate management
- GitLab CI integration template
- Slack app for approval workflow management
- Custom webhook payload templates
- GraphQL API (read-only, analytics use cases)
- External risk signal ingestion: PagerDuty, Datadog, GitHub Security Advisories
- Federated multi-org views for holding companies and platform teams managing multiple business units

---

## 14. Appendices

### 14.1 Glossary

| Term | Definition |
|---|---|
| CGP | Change Governance Protocol. The open protocol defined by Relicta for governing software change decisions. Defines proposal, decision, and execution authorization message types. |
| MCP | Model Context Protocol. Anthropic's open standard for tool use by AI agents. Relicta exposes an MCP server allowing AI agents to interact with the governance workflow. |
| DORA | DevOps Research and Assessment. The research program behind the State of DevOps report. Defines four key software delivery metrics: deployment frequency, lead time for changes, change failure rate, and mean time to recover. |
| Blast Radius | The scope of potential impact if a release causes an issue. One of the seven CGP risk factors. Calculated from files changed, lines changed, and downstream dependency count. |
| Actor | Any entity that can propose or approve a change. CGP recognizes four actor kinds: human, agent (AI coding assistant), CI (CI/CD system), and system (scheduled automation). |
| Actor Reputation | A composite trust score for an actor, calculated from release success rate, risk prediction accuracy, and outcome-weighted historical performance. Stored per-repo in the CLI and aggregated org-wide in the Hub. |
| Release Memory | The CLI's local persistence of governance history. Stored as JSON (default) or PostgreSQL. The Hub aggregates Release Memory data from all connected repositories. |
| Risk Budget | An organization-level configuration that caps the total risk score that can be introduced in a given time period. When the budget is exceeded, releases require override approval. |
| Freeze Period | A configured time window during which releases above a risk threshold are blocked. Can be set in the CLI per-repo or in the Hub org-wide. |
| SCIM | System for Cross-domain Identity Management. An open standard for automating user provisioning and deprovisioning. |
| SLSA | Supply-chain Levels for Software Artifacts. A security framework for hardening the software supply chain. |

### 14.2 Competitive Landscape Summary

| Product | Strengths | Weaknesses | Differentiation |
|---|---|---|---|
| GitHub Advanced Security + Environments | Native integration, large install base | No semantic risk scoring, no AI agent governance | CGP risk model and agent reputation are not available in GitHub's offering |
| LinearB | DORA metrics, engineering manager dashboards | No release governance, no approval workflow | Hub governs decisions, not just measures outcomes |
| Jellyfish | Engineering analytics, DORA metrics | No release governance, expensive | Hub is governance-first; analytics are a byproduct of the governance trail |
| Cortex | Service catalog, DORA metrics, scorecards | Broad but shallow on release governance | Hub has deep CGP integration; governance is the core, not a scorecard |
| OpsLevel | Service maturity, DORA | Similar to Cortex; no release-time governance | Same as above |
| Custom tooling | Fits exact org needs | High build and maintenance cost | Hub is production-ready out of the box; no build required |

No current product combines: (1) per-release AI-powered risk scoring, (2) AI agent governance with reputation tracking, (3) automated SOC 2 evidence generation from governance data, and (4) open CGP protocol with CLI-first adoption.

### 14.3 Technical Dependencies

| Dependency | Version | Purpose | Risk |
|---|---|---|---|
| Go | 1.22+ | Hub server | Low — stable language |
| PostgreSQL | 16 | Primary data store | Low — mature, well-supported |
| Redis | 7 | Cache, pub/sub, rate limiting | Low — mature |
| Vue 3 | 3.4+ | Frontend | Low — existing CLI dashboard shares components |
| Chi | 5 | HTTP router | Low — in use in CLI dashboard today |
| golang-migrate | 4 | Database migrations | Low — standard tooling |
| Docker / Kubernetes | Current stable | Container runtime | Low — industry standard |
| Helm | 3 | Enterprise self-hosted packaging | Low — standard for Kubernetes applications |

---

*This document is maintained alongside the codebase. For questions, contact the product team. The canonical internal version with full commercial sensitivity is available in `docs/internal/hub-prd.md`.*
