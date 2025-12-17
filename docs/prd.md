# Product Requirements Document (PRD): AI-Assisted Release Management CLI

## 1. Overview

**Product Name:** Relicta

**Summary:** Relicta is a CLI tool designed to streamline software release management for developers and product teams. It automates versioning, changelog generation, and public-facing release communication using an AI engine and a plugin-based integration system. The tool improves developer experience (DX) by supporting structured workflows, offering both cloud and local AI generation, and integrating with CI/CD and common platforms via plugins.

---

## 2. Problem Statement

Modern software teams face significant friction in the release process:

- Writing release notes is manual, tedious, and inconsistent.
- Release workflows are fragmented between developer and product/marketing teams.
- Developers waste time translating commits into changelogs or announcements.
- Teams use multiple disconnected tools for versioning, changelog updates, and release communication.

---

## 3. Goals & Objectives

- Automate release versioning, changelog creation, and release note writing.
- Support developer workflows (semver, multi-package repos, commit parsing).
- Generate audience-tailored content (internal changelogs, public notes, marketing blurbs).
- Provide a plugin system for publishing to GitHub Releases, npm, Slack, LaunchNotes, etc.
- Offer AI integration (cloud-based and local) for generating content.
- Improve consistency and quality of release communication.

---

## 4. Target Users

- Developers (working on CI/CD, monorepos, package management)
- DevOps/Platform Engineers (managing release pipelines)
- Product Managers (reviewing and publishing release content)
- Open Source Maintainers
- Teams releasing on GitHub, GitLab, npm, Docker Hub, etc.

---

## 5. User Stories

### Developer:

- As a developer, I want to bump version and auto-generate changelog from commits.
- As a developer, I want to generate release notes with AI help.
- As a developer, I want to preview and edit release notes before publishing.

### Product Manager:

- As a PM, I want to receive a human-friendly draft of the release announcement.
- As a PM, I want to approve or adjust the final text before publishing.

### DevOps Engineer:

- As a DevOps engineer, I want the CLI to run in CI and publish releases automatically.
- As a DevOps engineer, I want to integrate release notes with our changelog/notification systems.

---

## 6. Features

### Core CLI Workflow

- `relicta init` – Set up config and default options.
- `relicta plan` – Analyze changes since last release.
- `relicta bump` – Calculate and apply semver bump.
- `relicta notes` – Generate internal changelog and public notes.
- `relicta approve` – Review/edit notes for final approval.
- `relicta publish` – Execute release: tag, changelog, notify, publish.

### AI Integration

- Summarize commits/PRs into changelogs.
- Generate public-friendly release notes.
- Support tone presets (technical, friendly, excited).
- Support OpenAI API, Anthropic, Google Gemini, or local models (e.g., Ollama).

### Plugin Ecosystem

- Plugins are standalone executables using HashiCorp go-plugin (gRPC).
- Hook-based lifecycle: `PreVersion`, `PostNotes`, `PostPublish`, etc.
- Official plugins for:
  - GitHub/GitLab Releases
  - npm publish
  - Slack/Discord/Teams notifications
  - LaunchNotes
  - Jira updates

### Configuration

- Single config file: `relicta.config.yaml` or `relicta.config.json`
- Define:
  - Versioning strategy
  - AI provider and model
  - Enabled plugins
  - Template paths

### Templates and Output

- Markdown templates for:
  - Internal changelog
  - Public notes
  - Social/marketing blurbs

- User can override defaults or supply custom templates.

### Safety & Usability

- Dry-run support for previewing changes.
- Approval gates (interactive or in CI).
- Rollback or undo guidance.
- Editor integration for final note edits.

---

## 7. Technical Architecture

- Language: Go (security, single binary distribution, minimal dependencies)
- CLI Framework: Cobra (industry standard for Go CLIs)
- Plugin Loader: HashiCorp go-plugin (gRPC-based secure plugin execution)
- AI Integration: Pluggable provider system (OpenAI, Anthropic, Google Gemini, Ollama)
- Git Integration: go-git library for pure Go git operations
- Configuration: Viper for flexible config management (YAML, JSON, env vars)

---

## 8. MVP Scope

### Must-Have

- Core commands (`init`, `plan`, `bump`, `notes`, `approve`, `publish`)
- AI integration (OpenAI, Anthropic, Google Gemini, Ollama)
- GitHub & GitLab plugins
- Slack & Discord notification plugins
- Jira integration plugin
- YAML/JSON config

### Nice-to-Have

- Multi-package/monorepo support
- LaunchNotes plugin
- Tone/style templates for AI
- Homebrew formula publishing
- Docker Hub / Container registries

---

## 9. Success Metrics

- Time saved on release documentation (avg minutes/release)
- Reduction in bugs/errors related to manual versioning
- % of releases with AI-generated notes used
- Plugin ecosystem growth (number of plugins installed)
- Developer satisfaction (feedback surveys)

---

## 10. Future Opportunities

### Platform & Infrastructure
- SaaS dashboard for managing drafts, release analytics
- Plugin marketplace or registry
- Visual editor for release planning & summaries
- Auto-localization of notes (AI-generated translations)
- Self-hosted enterprise server option
- Webhook system for custom integrations

### Product Announcement & Changelog Platforms
- AnnounceKit - Changelog widget with user reactions
- Canny - Feedback + changelog + public roadmaps
- Beamer - In-app notifications and changelog widget
- Headway - Changelog widget with segmentation
- ProductBoard - Product management releases
- ReleaseNotes.io - Embeddable changelog

### Communication & Collaboration
- Microsoft Teams - Enterprise notifications
- Intercom - Customer messaging releases
- Zendesk - Support ticket release updates
- Linear - Modern issue tracking
- Asana - Project management
- Monday.com - Work OS integration
- ClickUp - All-in-one project management
- Basecamp - Team communication

### Documentation & Knowledge Base
- GitBook - Developer documentation
- ReadMe - API documentation updates
- Docusaurus - Static docs generation

### Social & Marketing
- Twitter/X - Social announcements
- LinkedIn - Professional updates
- Dev.to - Developer community posts
- Hashnode - Developer blogging
- Medium - Blog publishing
- Reddit - Subreddit announcements
- Hacker News - Show HN submissions

### Email & Newsletter
- SendGrid - Transactional email
- Mailchimp - Newsletter campaigns
- Postmark - Developer email
- Resend - Modern email API
- ConvertKit - Creator newsletters

### Monitoring & Observability
- Sentry - Error tracking release annotations
- Datadog - APM release markers
- New Relic - Performance release tracking
- PagerDuty - Incident management
- Opsgenie - Alert management
- Grafana - Dashboard annotations

### CI/CD Integration
- Jenkins - Pipeline triggers
- CircleCI - Build integration
- Travis CI - CI automation
- Azure DevOps - Microsoft CI/CD
- Bitbucket Pipelines - Atlassian CI/CD
- Buildkite - CI/CD at scale

### Cloud & Deployment Platforms
- AWS CodePipeline - AWS release management
- Google Cloud Deploy - GCP delivery
- Azure Release Pipelines - Microsoft releases
- Vercel - Frontend deployments
- Netlify - JAMstack deployments
- Railway - App deployment
- Fly.io - Edge deployments
- Render - Cloud platform
- Heroku - PaaS deployments

### Mobile App Stores
- Apple App Store Connect - iOS releases
- Google Play Console - Android releases
- TestFlight - iOS beta distribution
- Firebase App Distribution - Cross-platform beta

### Feature Flags & Experimentation
- LaunchDarkly - Feature flag management
- Split.io - Feature delivery
- Flagsmith - Open source flags
- Unleash - Feature toggles
- GrowthBook - A/B testing

---

## 11. Relicta in Agentic Systems (Strategic Positioning)

### 11.1 Context: The Rise of Agentic Software Development

Modern software development is rapidly transitioning from human-driven workflows to agentic systems — AI agents that autonomously analyze code, open pull requests, modify infrastructure, and initiate releases.

While these agents dramatically increase velocity, they introduce a new class of risk:

- Autonomous actions without clear accountability
- Implicit decision-making hidden in prompts or logs
- Lack of organizational context (business criticality, timing, policy)
- Fragmented automation across multiple independent agents

Existing release tooling was not designed for this paradigm. Deterministic tools (e.g., semantic-release) and CI pipelines assume either:

- Perfectly structured inputs, or
- Direct human oversight

Neither assumption holds in agentic environments.

### 11.2 Problem: No Trusted Boundary Between AI and Production

In agentic systems, the release process becomes the highest-risk surface:

- Agents can generate, modify, and merge code faster than humans can review it
- Release decisions (versioning, timing, communication) are often implicit
- There is no shared authority enforcing when and how AI-driven changes may ship

This results in:

- Silent breaking changes
- Unexplainable releases
- Policy violations
- Erosion of trust in automation

### 11.3 Relicta's Role: The Agentic Release Authority

Relicta is designed to act as the authoritative control plane between agentic systems and production releases.

Rather than replacing agents, Relicta:

- Constrains them with explicit policy
- Interprets their intent into human-meaningful decisions
- Records rationale, context, and outcomes
- Enforces organizational standards consistently

In an agentic environment, all release actions must pass through Relicta.

Relicta becomes:

- The final decision boundary before production
- A shared source of truth for release intent and risk
- The long-term memory of how changes impacted systems over time

### 11.4 Key Capabilities for Agentic Systems

**Agent-Aware Semantic Analysis**

Relicta evaluates changes based on code semantics and dependency impact, not commit conventions, enabling safe releases even when changes are authored by autonomous agents.

**Policy Enforcement for AI-Initiated Changes**

Relicta enforces release policies such as:

- Approval requirements based on risk level
- Restrictions on autonomous major releases
- Time-based or context-based deployment rules
- Organization-wide release governance

**Human-in-the-Loop by Design**

Relicta introduces a deliberate, inspectable checkpoint where humans can:

- Review agent-proposed releases
- Override versioning decisions
- Modify release narratives
- Block unsafe releases

**Release Memory & Learning**

Relicta maintains historical context across releases:

- Past incidents and rollbacks
- Patterns of risky changes
- Agent behavior over time

This enables Relicta to continuously improve risk assessment beyond what stateless agents can achieve.

### 11.5 Strategic Differentiation

Relicta is uniquely positioned as a neutral coordination layer across heterogeneous AI agents and tools.

Because Relicta integrates via the Model Context Protocol (MCP):

- It is model-agnostic
- It supports multi-agent environments
- It avoids vendor lock-in
- It remains future-proof as agent ecosystems evolve

Relicta is not a tool agents replace — it is the system agents must respect.

---

## 12. Change Governance Protocol (CGP)

### 12.1 Overview

The Change Governance Protocol (CGP) is a neutral, open protocol that defines how autonomous systems, CI pipelines, and human operators propose, evaluate, approve, and execute production changes in a controlled, auditable manner.

CGP exists to solve a fundamental problem in modern software development:
**AI agents can create change faster than organizations can safely govern it.**

Relicta implements CGP as its core interaction model and serves as the reference implementation for CGP-compliant release governance.

### 12.2 Motivation: Governance in an Agentic World

As software delivery becomes increasingly agent-driven, traditional release tooling fails to address:

- Autonomous actions without explicit approval boundaries
- Implicit decisions hidden inside agent prompts
- Inconsistent application of organizational policy
- Lack of explainability and accountability for AI-initiated changes

CGP introduces a standardized decision boundary between agents and production systems.

Under CGP:

- Agents may propose changes
- Governance systems decide
- Execution systems act

This separation is foundational to safe, scalable agentic workflows.

### 12.3 Core Principles

**Intent ≠ Execution**
Proposing a change does not imply permission to execute it.

**Governance Is Explicit**
All decisions are structured, inspectable, and auditable.

**Policy Over Prompting**
Organizational rules are enforced by protocol, not AI instructions.

**Human Override Is First-Class**
Autonomous behavior must always be interruptible.

**Model & Vendor Neutrality**
CGP is compatible with any AI system via MCP.

### 12.4 CGP Interaction Model

#### Actors

CGP defines three actor types:

| Actor | Description |
|-------|-------------|
| **Proposers** | AI agents, CI systems, or humans proposing a change |
| **Governors** | Systems implementing CGP (e.g., Relicta) that evaluate proposals |
| **Executors** | Systems that carry out approved actions (CI/CD, registries, infra tools) |

Actors may be combined but responsibilities remain distinct.

#### Canonical Flow

```
Proposer → CGP Governor → Decision → (Human Review?) → Executor
```

CGP standardizes this flow regardless of tooling or agent implementation.

### 12.5 CGP Message Types (Protocol Specification)

#### Change Proposal

A proposer submits a structured change intent.

```json
{
  "cgpVersion": "0.1",
  "type": "change.proposal",
  "actor": {
    "kind": "agent",
    "id": "ai-refactor-agent"
  },
  "scope": {
    "repository": "relicta-tech/relicta-core",
    "commitRange": "abc123..def456"
  },
  "intent": {
    "summary": "Refactor authentication middleware",
    "confidence": 0.74
  }
}
```

#### Governance Evaluation

The governor evaluates the proposal against:

- Code semantics and API changes
- Dependency and blast radius analysis
- Organizational policy
- Historical outcomes (optional)

#### Governance Decision

```json
{
  "cgpVersion": "0.1",
  "type": "change.decision",
  "decision": "approval_required",
  "recommendedVersion": "major",
  "riskScore": 0.82,
  "rationale": [
    "Public authentication interface modified",
    "Three downstream services affected",
    "Component classified as security-critical"
  ],
  "requiredActions": [
    "human_approval",
    "release_note_review"
  ]
}
```

#### Execution Authorization

Only after governance approval may execution occur.

```json
{
  "type": "change.execution_authorized",
  "approvedBy": "release-manager",
  "timestamp": "2025-12-15T17:45:00Z"
}
```

### 12.6 Relicta as CGP Reference Implementation

Relicta implements CGP as its foundational architecture and provides:

- A CGP-compliant CLI and TUI
- Policy enforcement for agent-initiated changes
- Semantic version inference and blast-radius analysis
- Human-in-the-loop governance workflows
- Immutable audit trails for all decisions

**Relicta does not replace agents — it governs them.**

### 12.7 Strategic Advantage

By separating CGP from the Relicta product:

- CGP can become an industry standard
- Other tools may adopt the protocol
- Relicta becomes the default governance layer
- Lock-in occurs at the decision layer, not the execution layer

This positions Relicta as a long-term control plane for agentic systems.

### 12.8 Future Evolution

CGP is intentionally extensible and may later govern:

- Infrastructure changes
- Data migrations
- Configuration rollouts
- AI model updates

Releases are the first and most critical use case.

### 12.9 Positioning Statement

> **Relicta is the reference implementation of the Change Governance Protocol (CGP) for agentic software delivery.**

---

## 13. Risks & Mitigation

| Risk                                      | Mitigation                                     |
| ----------------------------------------- | ---------------------------------------------- |
| AI inaccuracies in summaries              | Require human approval step before publishing  |
| Misconfigurations cause versioning errors | Implement dry-run + detailed logs              |
| Plugin compatibility breaks               | Versioned plugin API + official plugin support |
| Performance bottlenecks in large repos    | Caching + incremental changelog generation     |

---

## 14. Timeline (Post-Approval)

**Week 1–2:** Design CLI structure, command syntax, scaffolding.

**Week 3–5:** Implement versioning logic, changelog generation, AI module.

**Week 6–8:** Build plugin system, implement GitHub + npm + Slack plugins.

**Week 9–10:** Interactive flow, dry-run, approval UX.

**Week 11–12:** Documentation, examples, test suite, beta release.

---

## 15. Stakeholders

- Product Engineering
- DevOps / Platform Team
- Developer Experience (DX) Lead
- Documentation / Technical Writers

---

## 16. Appendix

- Inspired by Changesets, semantic-release, Auto, LaunchNotes.
- Plugin architecture modeled after semantic-release and kubectl.
