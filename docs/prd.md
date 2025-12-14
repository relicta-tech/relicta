# Product Requirements Document (PRD): AI-Assisted Release Management CLI

## 1. Overview

**Product Name:** Relicta (Working Title)

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

- `release init` – Set up config and default options.
- `release plan` – Analyze changes since last release.
- `release version` – Calculate and apply semver bump.
- `release notes` – Generate internal changelog and public notes.
- `release approve` – Review/edit notes for final approval.
- `release publish` – Execute release: tag, changelog, notify, publish.

### AI Integration

- Summarize commits/PRs into changelogs.
- Generate public-friendly release notes.
- Support tone presets (technical, friendly, excited).
- Support OpenAI API, Anthropic, or local models (e.g., Ollama).

### Plugin Ecosystem

- Plugins are standalone npm packages or executables.
- Hook-based lifecycle: `preVersion`, `postNotes`, `onPublish`, etc.
- Official plugins for:
  - GitHub/GitLab Releases
  - npm/yarn/pnpm publish
  - Slack/Teams notifications
  - LaunchNotes, AnnounceKit
  - Jira/Confluence update
  - Email (SMTP/SendGrid)

### Configuration

- Single config file: `release.config.json` or `.release.yml`
- Define:
  - Versioning strategy
  - AI model/key
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
- Plugin Loader: Go plugins + PATH-based executables via HashiCorp go-plugin
- AI Integration: pluggable prompt-to-output model (OpenAI, Anthropic, Ollama)
- Git Integration: go-git library for pure Go git operations
- Configuration: Viper for flexible config management (YAML, JSON, env vars)

---

## 8. MVP Scope

### Must-Have

- Core commands (`init`, `version`, `notes`, `publish`)
- Basic AI integration (OpenAI)
- GitHub & GitLab plugins
- Slack & Discord notification plugins
- Jira integration plugin
- JSON/YAML config

### Package Registry Plugins (Language Support)

- npm (JavaScript/Node.js)
- PyPI (Python)
- crates.io (Rust)
- Maven Central (Java)
- NuGet (.NET)
- RubyGems (Ruby)
- Packagist (PHP)
- Go Modules (Go)
- Hex (Elixir/Erlang)
- Cargo (Rust alternative registries)

### Nice-to-Have

- Multi-package support
- Local AI runner (Ollama)
- LaunchNotes plugin
- Tone/style templates for AI
- Homebrew formula publishing
- Docker Hub / Container registries
- Chocolatey (Windows)
- APT/YUM repository publishing

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

## 11. Strategic Moat: Model Context Protocol (MCP) Integration

The Model Context Protocol (MCP) is a foundational pillar of Relicta's long-term defensibility and a key differentiator compared to legacy release automation tools. MCP enables Relicta to become the **semantic intelligence layer** of modern DevOps workflows by allowing AI models to access structured, authoritative system context—securely, consistently, and without brittle custom integrations.

### 1. Contextual Intelligence Beyond Deterministic Tools

While existing tools rely exclusively on commit heuristics or strict formatting rules, Relicta uses MCP to retrieve real system context from providers such as Jira, Linear, GitHub, Nx/Turborepo, and CI pipelines. This context allows Relicta's AI engine to **understand the intent behind changes**, not just the diff, resulting in:

- More accurate version inference (understanding semantic impact vs. textual diff)
- High-quality changelogs that capture business value, not just technical changes
- Reliable risk modeling based on deployment history, test coverage, and issue context

**Example:** Instead of simply parsing "fix: button color," MCP allows Relicta to query Jira and understand this fix resolves a P0 accessibility issue affecting 10,000+ users, adjusting changelog tone and priority accordingly.

### 2. Secure, Enterprise-Ready Architecture

MCP provides a **zero-trust, capability-scoped interface** between Relicta and external systems:

- **Credentials never pass through Relicta or the LLM** - Authentication happens via MCP servers
- **Sanitized, structured data exposure** - Only release-relevant data flows to AI models
- **Granular permission control** - Enterprises can scope MCP access to specific repos, issues, or CI jobs
- **Audit trail compliance** - MCP interactions are logged for SOC2/ISO compliance

This security posture makes Relicta uniquely suitable for regulated industries (finance, healthcare, defense) and privacy-sensitive environments where traditional API integrations require excessive privileged access.

### 3. Future-Proof Integration Layer

By relying on **MCP providers instead of bespoke API clients**, Relicta becomes vendor-agnostic:

- **No custom integration maintenance** - As tools adopt MCP, Relicta gains compatibility automatically
- **Ecosystem network effects** - MCP adoption by Jira, GitHub, Linear, etc. directly benefits Relicta
- **Plugin obsolescence prevention** - MCP standardization reduces plugin maintenance burden
- **Cost efficiency** - Single MCP integration replaces dozens of custom API clients

**Timeline advantage:** While competitors maintain brittle REST/GraphQL integrations, Relicta leverages the growing MCP ecosystem with zero marginal integration cost.

### 4. Foundation for Multi-Agent DevOps Automation

MCP enables Relicta to act not only as a **client** but eventually as an **MCP Provider**—exposing its own tools such as:

- `getBlastRadius(changeId)` - Analyze deployment risk and affected components
- `inferVersion(commits, context)` - Semantic versioning with business context
- `summarizeDiff(before, after, audience)` - Generate audience-tailored summaries
- `validateRelease(releaseId)` - Pre-flight checks across systems

This allows **external AI agents** (IDE copilots, CI agents, terminal assistants, PR reviewers) to query Relicta for semantic reasoning:

- **IDE Copilot:** "What's the blast radius of this change?" → Queries Relicta MCP server
- **CI Agent:** "Should I block this release?" → Consults Relicta's risk model
- **PR Reviewer:** "Generate release notes preview" → Streams from Relicta

As agentic development environments grow (GitHub Copilot Workspace, Cursor Composer, Replit Agent), Relicta becomes the **"release brain"** these agents rely on, creating:

- **Strong network effect** - More agents = more Relicta queries = more value
- **Lock-in via ecosystem position** - Switching costs increase as agents integrate
- **Platform moat** - Relicta becomes infrastructure, not just a tool

### 5. Defensible Competitive Advantage

No existing release automation tool offers MCP integration or agent-aware capabilities:

| Tool | MCP Support | AI Context | Agent Interop | Secure Multi-Tenant |
|------|-------------|------------|---------------|---------------------|
| **Relicta** | ✅ Full | ✅ Rich | ✅ Provider + Client | ✅ Enterprise-ready |
| semantic-release | ❌ | ❌ | ❌ | ❌ |
| release-it | ❌ | ❌ | ❌ | ❌ |
| GoReleaser | ❌ | ❌ | ❌ | ❌ |
| Changesets | ❌ | ❌ | ❌ | ❌ |

**MCP gives Relicta a durable moat by combining:**

1. **High-fidelity semantic context** - Understands *why* changes matter, not just *what* changed
2. **AI-grounded reasoning** - LLMs make better decisions with real system context
3. **Secure, standardized integrations** - Enterprise-grade without custom API sprawl
4. **Future agent compatibility** - Positioned as infrastructure for AI-native DevOps

**Strategic Positioning:** Relicta is not merely a release tool but a **core intelligence node** in the emerging AI-driven DevOps stack, uniquely positioned at the intersection of:

- **Developer workflows** (commits, PRs, CI/CD)
- **Product workflows** (issues, roadmaps, changelog communication)
- **AI automation** (agentic coding, release decision-making)

### 6. Implementation Roadmap

**Phase 1 (Current):** MCP Client Integration
- Jira MCP server (issue context for changelog enrichment)
- GitHub MCP server (PR metadata, commit context)
- Linear MCP server (issue tracking for modern teams)

**Phase 2 (6-12 months):** Advanced Context Providers
- Nx/Turborepo MCP servers (monorepo change analysis)
- CI/CD MCP servers (test coverage, deployment history)
- Error tracking MCP servers (Sentry release annotations)

**Phase 3 (12-18 months):** MCP Provider Development
- Expose Relicta tools to external AI agents
- SDK for third-party MCP client integration
- Agent orchestration layer for multi-step release workflows

**Phase 4 (18-24 months):** Agent Ecosystem Platform
- Marketplace for release automation agents
- Agent-to-agent workflows (e.g., CI agent → Relicta → Slack agent)
- Enterprise agent deployment and monitoring

This positions Relicta to evolve from **a CLI tool** → **a release intelligence platform** → **the semantic layer for DevOps agents**, creating compounding competitive advantages as the agent ecosystem matures.

---

## 12. Risks & Mitigation

| Risk                                      | Mitigation                                     |
| ----------------------------------------- | ---------------------------------------------- |
| AI inaccuracies in summaries              | Require human approval step before publishing  |
| Misconfigurations cause versioning errors | Implement dry-run + detailed logs              |
| Plugin compatibility breaks               | Versioned plugin API + official plugin support |
| Performance bottlenecks in large repos    | Caching + incremental changelog generation     |

---

## 13. Timeline (Post-Approval)

**Week 1–2:** Design CLI structure, command syntax, scaffolding.

**Week 3–5:** Implement versioning logic, changelog generation, AI module.

**Week 6–8:** Build plugin system, implement GitHub + npm + Slack plugins.

**Week 9–10:** Interactive flow, dry-run, approval UX.

**Week 11–12:** Documentation, examples, test suite, beta release.

---

## 14. Stakeholders

- Product Engineering
- DevOps / Platform Team
- Developer Experience (DX) Lead
- Documentation / Technical Writers

---

## 15. Go To Market Strategy

### Freemium Model Overview

Relicta will adopt a freemium model to drive adoption while monetizing advanced capabilities and enterprise integrations.

### Free Tier (Developer Tier)

- Full access to CLI commands
- Basic AI summarization (e.g. GPT-3.5 or local-only models)
- GitHub, npm, and Slack plugins
- Markdown changelog generation
- Plugin framework and local-only plugin support

### Pro CLI License (Self-hosted)

- Advanced AI (GPT-4/5 access, tone/style presets)
- Multi-project/monorepo support
- Approval workflows
- Audit trail and release logs
- Plugin chaining, lifecycle control, and script hooks
- Role-based CLI usage (per team member license)

**Pricing Model:**

- Monthly or annual license (per seat or team)
- CLI license key distributed via environment variables or config

### SaaS Dashboard (Optional Add-On)

- Release history dashboard and analytics
- Collaborative changelog editor with approval workflows
- Secrets and plugin credential management
- Slack/email notifications from release flow
- GitHub/GitLab sync and audit log

**Pricing Model:**

- Free for individuals and open source
- Paid tiers by team size or feature unlock

### Paid Plugins & Marketplace

- LaunchNotes, Jira, Confluence, Notion integrations (Pro-only)
- Plugin bundles (e.g., Product Ops Pack, Enterprise Pack)
- Developer plugin marketplace with revenue share model

**Complete Plugin Tier Matrix:**

| Plugin | Free | Pro | SaaS | Notes |
| ------ | ---- | --- | ---- | ----- |
| **Version Control** |||||
| GitHub Releases | ✅ | ✅ | ✅ | Core integration |
| GitLab Releases | ✅ | ✅ | ✅ | Core integration |
| Bitbucket | ✅ | ✅ | ✅ | |
| **Package Registries** |||||
| npm (JavaScript) | ✅ | ✅ | ✅ | Core - language agnostic |
| PyPI (Python) | ✅ | ✅ | ✅ | Core - language agnostic |
| crates.io (Rust) | ✅ | ✅ | ✅ | Core - language agnostic |
| Maven Central (Java) | ✅ | ✅ | ✅ | Core - language agnostic |
| NuGet (.NET) | ✅ | ✅ | ✅ | Core - language agnostic |
| RubyGems (Ruby) | ✅ | ✅ | ✅ | Core - language agnostic |
| Packagist (PHP) | ✅ | ✅ | ✅ | Core - language agnostic |
| Go Modules (Go) | ✅ | ✅ | ✅ | Core - language agnostic |
| Hex (Elixir) | ✅ | ✅ | ✅ | Core - language agnostic |
| Homebrew | ✅ | ✅ | ✅ | |
| Docker Hub | ✅ | ✅ | ✅ | |
| **Basic Notifications** |||||
| Slack | ✅ | ✅ | ✅ | Core integration |
| Discord | ✅ | ✅ | ✅ | Core integration |
| Webhooks (generic) | ✅ | ✅ | ✅ | Custom integrations |
| **Enterprise Notifications** |||||
| Microsoft Teams | ❌ | ✅ | ✅ | Enterprise focus |
| Email (SMTP) | ❌ | ✅ | ✅ | |
| SendGrid | ❌ | ✅ | ✅ | |
| Postmark | ❌ | ✅ | ✅ | |
| Mailchimp | ❌ | ✅ | ✅ | Newsletter campaigns |
| **Issue Tracking** |||||
| Jira | ❌ | ✅ | ✅ | Enterprise integration |
| Linear | ❌ | ✅ | ✅ | Modern teams |
| Asana | ❌ | ✅ | ✅ | |
| Monday.com | ❌ | ✅ | ✅ | |
| ClickUp | ❌ | ✅ | ✅ | |
| **Product Announcements** |||||
| LaunchNotes | ❌ | ✅ | ✅ | Premium changelog |
| AnnounceKit | ❌ | ✅ | ✅ | |
| Canny | ❌ | ✅ | ✅ | |
| Beamer | ❌ | ✅ | ✅ | |
| Headway | ❌ | ✅ | ✅ | |
| **Documentation** |||||
| Confluence | ❌ | ✅ | ✅ | Enterprise wiki |
| Notion | ❌ | ✅ | ✅ | |
| GitBook | ❌ | ✅ | ✅ | |
| ReadMe | ❌ | ✅ | ✅ | API docs |
| **Social & Marketing** |||||
| Twitter/X | ❌ | ✅ | ✅ | Social automation |
| LinkedIn | ❌ | ✅ | ✅ | |
| Dev.to | ❌ | ✅ | ✅ | |
| Medium | ❌ | ✅ | ✅ | |
| Reddit | ❌ | ✅ | ✅ | |
| **Customer Platforms** |||||
| Intercom | ❌ | ❌ | ✅ | SaaS-only |
| Zendesk | ❌ | ❌ | ✅ | SaaS-only |
| **Monitoring & Observability** |||||
| Sentry | ❌ | ✅ | ✅ | Release annotations |
| Datadog | ❌ | ✅ | ✅ | |
| New Relic | ❌ | ✅ | ✅ | |
| Grafana | ❌ | ✅ | ✅ | |
| PagerDuty | ❌ | ❌ | ✅ | SaaS-only |
| Opsgenie | ❌ | ❌ | ✅ | SaaS-only |
| **CI/CD Integration** |||||
| Jenkins | ❌ | ✅ | ✅ | Pipeline triggers |
| CircleCI | ❌ | ✅ | ✅ | |
| Azure DevOps | ❌ | ✅ | ✅ | |
| Bitbucket Pipelines | ❌ | ✅ | ✅ | |
| **Cloud Platforms** |||||
| Vercel | ❌ | ✅ | ✅ | Deployment triggers |
| Netlify | ❌ | ✅ | ✅ | |
| Railway | ❌ | ✅ | ✅ | |
| Fly.io | ❌ | ✅ | ✅ | |
| AWS CodePipeline | ❌ | ❌ | ✅ | Enterprise cloud |
| Google Cloud Deploy | ❌ | ❌ | ✅ | Enterprise cloud |
| Azure Pipelines | ❌ | ❌ | ✅ | Enterprise cloud |
| **Mobile App Stores** |||||
| Apple App Store Connect | ❌ | ✅ | ✅ | iOS releases |
| Google Play Console | ❌ | ✅ | ✅ | Android releases |
| TestFlight | ❌ | ✅ | ✅ | Beta distribution |
| Firebase App Distribution | ❌ | ✅ | ✅ | |
| **Feature Flags** |||||
| LaunchDarkly | ❌ | ❌ | ✅ | SaaS integration |
| Split.io | ❌ | ❌ | ✅ | |
| Flagsmith | ❌ | ✅ | ✅ | Open source option |
| Unleash | ❌ | ✅ | ✅ | Open source option |

---

## 16. Appendix

- Based on competitive and feasibility research [Deep Research Report Ref].
- Inspired by Changesets, semantic-release, Auto, LaunchNotes.
- Plugin architecture modeled after semantic-release and kubectl.
