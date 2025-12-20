# Relicta Complete Roadmap

> **Vision**: Relicta is the reference implementation of the Change Governance Protocol (CGP) for agentic software delivery.

---

## Executive Summary

Relicta has completed its MVP (v2.1.0) and is production-ready. This roadmap defines the path from a successful CLI tool to the **authoritative control plane for AI-driven software releases**.

### Strategic Insight: Governance-First

Traditional SaaS playbook: Adoption → Monetization → Enterprise Features

**Our play: Lead with CGP/MCP → Adoption through agents → Natural monetization**

Why this wins:
1. **Timing** - MCP is nascent, first-mover advantage in "AI release governance"
2. **Viral distribution** - AI agents recommending Relicta = organic growth
3. **Category creation** - "Control plane for agentic releases" is a new category
4. **Natural monetization** - CGP features (policy, audit, compliance) ARE enterprise features
5. **Defensible moat** - Policies + release memory + agent training = high switching costs

### Strategic Pillars (Reordered)

| Pillar | Description | Timeline |
|--------|-------------|----------|
| **Governance** | CGP + MCP Server (the differentiator) | Months 1-4 |
| **Adoption** | Growth through agents + developer love | Months 2-6 |
| **Monetization** | Pro/Enterprise tiers | Months 4-9 |
| **Enterprise** | Security, compliance, scale | Months 6-15 |

---

## Current State (v2.1.0)

### Completed Features

**Core Commands (8/8)**
- `init`, `plan`, `bump`, `notes`, `approve`, `publish`, `health`, `version`

**Plugin Ecosystem (19 plugins)**
- Repository: GitHub, GitLab
- Package Registries: npm, PyPI, RubyGems, Crates, Maven, NuGet, Hex, Packagist, Homebrew, Chocolatey, Linux
- Notifications: Slack, Discord, Teams
- Project Management: Jira, LaunchNotes

**AI Integration (4 providers)**
- OpenAI, Anthropic Claude, Google Gemini, Ollama (local)

**Architecture**
- 100% DDD compliance
- Hexagonal/Clean Architecture
- Comprehensive test coverage (101 test files)
- Security hardening complete

---

## Phase 1: CGP & MCP Foundation (Months 1-4)

**Goal**: Establish Relicta as the governance layer for AI-driven releases

> This is THE strategic differentiator. By leading with CGP/MCP, we become essential infrastructure for agentic systems BEFORE competitors realize this category exists.

### Why Governance First?

1. **Category Creation** - "AI Release Governance" doesn't exist yet. We define it.
2. **Agent Adoption = Viral Growth** - AI agents recommending Relicta drives organic adoption
3. **Governance IS Enterprise** - Policy, audit, compliance are monetizable from day one
4. **High Switching Costs** - Policies + release memory = defensive moat
5. **Press & Thought Leadership** - "First tool AI agents must respect" is a compelling story

### 1.1 CGP Message Types & Protocol (Month 1-2)

**Protocol Implementation**:

```go
// Core CGP message types
type ChangeProposal struct {
    CGPVersion string
    Actor      Actor           // Who is proposing (agent, CI, human)
    Scope      ProposalScope   // Repository, commit range
    Intent     ProposalIntent  // Summary, suggested bump, confidence
}

type GovernanceDecision struct {
    Decision           DecisionType  // approved, approval_required, rejected
    RecommendedVersion string
    RiskScore          float64
    RiskFactors        []RiskFactor
    Rationale          []string
    RequiredActions    []RequiredAction
}

type ExecutionAuthorization struct {
    ApprovedBy    Actor
    Version       string
    ValidUntil    time.Time
    AllowedSteps  []string
    ApprovalChain []ApprovalRecord
}
```

**Deliverables**:
- CGP message schemas and Go types
- Basic policy engine
- Simple risk scoring (weighted factors)
- File-based audit trail
- CLI integration (`relicta cgp evaluate`)

### 1.2 MCP Server Implementation (Month 2-3)

**Model Context Protocol Server**:

```json
{
  "mcpServers": {
    "relicta-cgp": {
      "command": "relicta",
      "args": ["mcp", "serve"]
    }
  }
}
```

**MCP Tools** (available to AI agents):
- `cgp_propose_change` - Submit release proposal
- `cgp_get_decision` - Get governance decision
- `cgp_approve` - Human approval
- `cgp_reject` - Reject proposal
- `cgp_get_policy` - Query policies
- `cgp_list_pending` - List pending proposals
- `release_plan` - Analyze commits and plan release
- `release_execute` - Execute approved release

**MCP Resources**:
- `cgp://policies` - Current governance policies
- `cgp://pending` - Proposals awaiting approval
- `cgp://audit` - Recent decisions
- `cgp://history` - Release history

**Why MCP Matters**:
- Claude Code, Cursor, Windsurf, Cline all use MCP
- First MCP server for release governance = instant distribution
- AI agents discover and recommend tools that work well
- Organic adoption through agent recommendations

### 1.3 Policy Engine (Month 3-4)

**Policy Configuration** (YAML):

```yaml
version: "1.0"
name: "Standard Release Policy"

rules:
  - id: block-autonomous-major
    name: "Block Autonomous Major Releases"
    conditions:
      - field: "actor.kind"
        operator: "eq"
        value: "agent"
      - field: "analysis.breaking"
        operator: "gt"
        value: 0
    actions:
      - type: set_decision
        params:
          decision: approval_required
      - type: require_approval
        params:
          approvers: ["team:release-managers"]

  - id: auto-approve-trusted-patches
    name: "Auto-approve Trusted Patches"
    conditions:
      - field: "risk.score"
        operator: "lt"
        value: 0.3
      - field: "intent.suggestedBump"
        operator: "eq"
        value: "patch"
    actions:
      - type: set_decision
        params:
          decision: approved
```

### 1.4 Risk Scoring & Semantic Analysis (Month 4)

**Go AST Analysis**:
- Public API change detection (added, removed, modified)
- Breaking change identification
- Blast radius calculation

**Risk Factors**:

| Factor | Weight | Description |
|--------|--------|-------------|
| API Changes | 0.25 | Public interface modifications |
| Dependency Impact | 0.20 | Downstream consumer effects |
| Blast Radius | 0.15 | Files/lines/components changed |
| Code Complexity | 0.10 | Cyclomatic complexity |
| Test Coverage | 0.10 | Test coverage delta |
| Actor Trust | 0.05 | Historical actor reliability |
| Historical Risk | 0.10 | Past patterns and incidents |
| Security Impact | 0.05 | Security-critical components |

**Multi-Language Support** (Phase 5):
- TypeScript/JavaScript
- Python
- Rust

### Phase 1 Success Metrics (Month 4)

| Metric | Target |
|--------|--------|
| MCP Server Published | Yes |
| Agent Integrations | 3+ (Claude Code, Cursor, etc.) |
| CGP Proposals Processed | 100+ |
| Policy Rules Defined | 10+ example policies |
| First External Adoption | 5+ organizations |

---

## Phase 2: Adoption & Developer Experience (Months 2-6)

**Goal**: Grow to 1,000+ active users through exceptional DX and agent recommendations

> Runs in parallel with Phase 1. While building governance, we polish the developer experience to capture users driven by agent adoption.

### 2.1 Developer Experience Polish (Month 2-3)

| Feature | Effort | Impact |
|---------|--------|--------|
| Enhanced error messages with actionable guidance | 1 week | High |
| Progress indicators and streaming output | 1 week | Medium |
| Shell completion (bash, zsh, fish) | 3 days | Medium |
| Better dry-run visualization | 3 days | High |

### 2.2 Release Templates & Quick Start (Month 3-4)

| Template | Target Audience |
|----------|-----------------|
| Open Source Library | Go, Rust, Python library maintainers |
| SaaS Application | Next.js, React, Node.js teams |
| CLI Tool | Multi-platform binary distribution |
| API Service | REST/GraphQL with Docker |
| Mobile App | iOS/Android release workflows |
| NPM Package | JavaScript/TypeScript packages |

**Deliverable**: `relicta init` wizard with template selection

### 2.3 Documentation & Onboarding (Month 4-5)

- Video tutorials (2-min, 5-min, 10-min)
- Step-by-step recipes for common workflows
- FAQ and troubleshooting expansion
- Plugin development guide with examples
- **MCP setup guide for AI agents** (critical for adoption)

### 2.4 Community Building (Month 4-6)

- Discord/Slack community
- GitHub Discussions enabled
- Weekly office hours
- Plugin contribution guidelines
- **MCP integration showcase** (agents using Relicta)

### Phase 2 Success Metrics (Month 6)

| Metric | Target |
|--------|--------|
| Active Users | 1,000+ |
| GitHub Stars | 5,000+ |
| Template Usage | 80%+ |
| AI Feature Adoption | 30%+ |
| MCP Installations | 500+ |

---

## Phase 3: Pro Tier & Monetization (Months 4-9)

**Goal**: First revenue, validate pricing, reach $10K MRR

> Monetization follows naturally from governance. CGP features (policy UI, audit exports, advanced risk) become Pro features.

### 3.1 Backend Infrastructure (Month 4-5)

```
┌─────────────┐     ┌──────────────┐     ┌────────────┐
│   CLI       │────▶│  API Server  │────▶│ PostgreSQL │
│  (Go)       │◀────│  (Go + gRPC) │◀────│            │
└─────────────┘     └──────────────┘     └────────────┘
                           │
                    ┌──────┴──────┐
                    ▼             ▼
             ┌─────────────┐  ┌─────────┐
             │  Web UI     │  │ Redis   │
             │  (Next.js)  │  │ (Cache) │
             └─────────────┘  └─────────┘
```

**Components**:
- Go API server (REST + gRPC)
- PostgreSQL for persistence
- Redis for caching and rate limiting
- OAuth2 authentication
- License validation system
- Usage tracking

### 3.2 Managed AI (Month 5-6)

| Feature | Free | Pro |
|---------|------|-----|
| AI Provider | BYOK | Managed (no API key needed) |
| Usage Limit | N/A | 500 releases/month |
| Provider Selection | Manual | Auto-optimized |
| Response Caching | No | Yes |

**Implementation**: CLI checks license, routes to managed AI service

### 3.3 Web UI Dashboard (Month 6-7)

**MVP Features**:
- Release history timeline
- Visual commit analyzer
- Web-based approval workflow
- Basic analytics (releases/week, success rate)
- Plugin execution status
- **CGP policy editor** (visual policy builder)
- **Pending approvals dashboard**

**Tech Stack**: Next.js 14, React, Tailwind, Recharts

### 3.4 Analytics & DORA Metrics (Month 7-8)

**DORA Metrics**:
- Deployment Frequency
- Lead Time for Changes
- Change Failure Rate
- Time to Restore Service

**Additional Metrics**:
- Commit velocity per release
- Breaking change frequency
- Plugin success rates
- AI usage and savings
- **CGP decision analytics** (approval rates, risk trends)

### 3.5 Release Simulation & Advanced Features (Month 8-9)

- Visual diff viewer with syntax highlighting
- AI impact analysis and risk scoring
- Dependency impact visualization
- Pre-flight checks (tests, security, licenses)
- CHANGELOG.md preview
- One-command rollback

### Pricing Strategy

| Tier | Price | Features |
|------|-------|----------|
| **Free** | $0 | Core CLI, BYOK AI, all plugins, basic CGP, MCP Server |
| **Pro** | $49-99/month | Managed AI, Web UI, Analytics, Advanced CGP policies, Audit exports |

### Phase 3 Success Metrics (Month 9)

| Metric | Target |
|--------|--------|
| Pro Customers | 50-100 |
| MRR | $5-10K |
| Conversion Rate | 10%+ |
| Churn Rate | <5% |

---

## Phase 4: Enterprise (Months 6-15)

**Goal**: Enterprise revenue, $50K+ MRR

> Enterprise features build directly on CGP. Audit, compliance, and advanced governance are natural extensions of the governance-first architecture.

### 4.1 Advanced CGP Features (Month 6-9)

| Feature | Description |
|---------|-------------|
| **Immutable Audit Trail** | Cryptographic hash chain, tamper-evident logging |
| **Release Memory** | Historical outcomes, pattern detection, risk prediction |
| **Actor Performance Tracking** | Reliability scores for agents and humans |
| **Multi-Team Policies** | Organization-wide + team-specific rules |
| **Custom Approval Flows** | Configurable approval chains per risk level |

### 4.2 Security & Compliance (Month 9-12)

| Feature | Description |
|---------|-------------|
| **SBOM Generation** | CycloneDX/SPDX software bill of materials |
| **Supply Chain Security** | Dependency vulnerability scanning, SLSA compliance |
| **Compliance Reporting** | SOC2, HIPAA, FDA 21 CFR Part 11 audit exports |
| **Secret Scanning** | Detect leaked credentials in commits |
| **7+ Year Audit Retention** | SIEM integration, regulatory compliance |

### 4.3 Access Control (Month 10-12)

| Feature | Description |
|---------|-------------|
| **SSO/SAML** | Okta, Azure AD, OneLogin integration |
| **Advanced RBAC** | Custom roles, fine-grained permissions |
| **MFA Enforcement** | TOTP, WebAuthn support |
| **API Key Management** | Rotation policies, scoped permissions |
| **IP Allowlisting** | Network-level access control |

### 4.4 Enterprise Governance Workflows (Month 12-15)

| Feature | Description |
|---------|-------------|
| **CAB Integration** | Change Advisory Board workflows |
| **Release Windows** | Enforce release schedules, blackout periods |
| **Emergency Releases** | Expedited workflow for hotfixes |
| **ServiceNow/Jira SM** | ITSM tool integration |
| **Agent Quotas** | Limit autonomous release frequency |

### 4.5 AI Release Assistant (Month 13-15)

**Slack/Discord Bot**:
```
User: @relicta what's in the next release?
Bot: 📦 Next release for api-service:
     - 8 commits (5 features, 3 fixes)
     - Suggested version: v2.4.0 (minor)
     - Risk score: 0.35 (low)
     - Ready to release? Reply "approve" or "show details"
```

**Natural Language Commands**:
- "Release the API service with all fixes since Tuesday"
- "Show me what's in the next release"
- "Schedule a release for Friday at 2pm if tests pass"
- "Who approved the last release?"
- "What did Claude agent propose for the auth service?"

### 4.6 Multi-Repo Orchestration (Month 15-18)

| Feature | Description |
|---------|-------------|
| **Dependent Releases** | Release service B after A |
| **Atomic Multi-Repo** | All-or-nothing releases |
| **Cross-Repo Changelog** | Combined release notes |
| **Version Coordination** | Auto-update dependent versions |
| **Rollback Coordination** | Rollback multiple repos |
| **Blast Radius Calculation** | Service dependency impact |

### Enterprise Pricing

| Tier | Price | Key Features |
|------|-------|--------------|
| **Pro** | $49-99/month | Web UI, Analytics, Managed AI |
| **Enterprise** | $500-2000/month | CGP, SSO, SBOM, Compliance, AI Assistant |

### Enterprise Success Metrics

| Metric | Target |
|--------|--------|
| Enterprise Customers | 10-20 |
| Enterprise MRR | $20-40K |
| Average Contract Value | $12K-24K/year |
| Net Revenue Retention | 120%+ |

---

## Phase 5: Market Leadership (Months 18+)

### 5.1 CGP as Open Standard

- Publish CGP specification
- Community governance for protocol evolution
- Reference implementations in multiple languages
- Certification program for CGP-compliant tools

### 5.2 Advanced Intelligence

| Feature | Description |
|---------|-------------|
| **ML Risk Prediction** | Learn from release outcomes |
| **Anomaly Detection** | Identify unusual release patterns |
| **Proactive Recommendations** | Suggest optimal release timing |
| **Auto-Remediation** | Automated rollback triggers |

### 5.3 Platform Expansion

| Area | Description |
|------|-------------|
| **Infrastructure Changes** | CGP for Terraform, Kubernetes |
| **Configuration Rollouts** | Feature flags, config changes |
| **Data Migrations** | Database schema changes |
| **AI Model Updates** | ML model deployment governance |

### 5.4 IDE Integrations

| IDE | Features |
|-----|----------|
| **VSCode** | Release sidebar, command palette, status bar |
| **JetBrains** | IntelliJ, GoLand, WebStorm plugins |
| **Neovim** | CLI integration for power users |

---

## Technical Architecture Evolution

### Current (v2.1.0)
```
┌─────────────┐
│   CLI       │
│  (Go binary)│
└─────────────┘
    │
    ├── Git (go-git)
    ├── AI (OpenAI, Anthropic, Gemini, Ollama)
    └── Plugins (gRPC, go-plugin)
```

### Phase 1 (CGP + MCP)
```
┌─────────────┐     ┌─────────────┐
│   CLI       │◄───▶│ MCP Server  │◄──── AI Agents (Claude, Cursor, etc.)
│  (Go)       │     │  (stdio)    │
└─────────────┘     └─────────────┘
       │                   │
       │              ┌────┴────┐
       │              ▼         ▼
       │        ┌──────────┐  ┌──────────┐
       │        │ CGP      │  │ Policy   │
       │        │ Governor │  │ Engine   │
       │        └──────────┘  └──────────┘
       │
       └── Git, AI, Plugins (existing)
```

### Phase 3-4 (Pro + Enterprise)
```
┌─────────────┐     ┌──────────────┐     ┌────────────┐
│   CLI       │────▶│  API Server  │────▶│ PostgreSQL │
│  (Go)       │     │  (Go)        │     │            │
└─────────────┘     └──────────────┘     └────────────┘
                           │
      ┌────────────────────┼────────────────────┐
      ▼                    ▼                    ▼
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Web UI     │     │ CGP Governor│     │ MCP Server  │
│  (Next.js)  │     │             │     │             │
└─────────────┘     └─────────────┘     └─────────────┘
```

### Phase 4+ (Enterprise)
```
┌─────────────────────────────────────────────────────────────────┐
│                      Relicta Platform                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │   CLI       │  │   Web UI    │  │  AI Bot     │             │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘             │
│         │                │                │                     │
│         └────────────────┴────────────────┘                     │
│                          │                                       │
│  ┌───────────────────────▼───────────────────────┐              │
│  │              API Gateway                       │              │
│  │  (Auth, Rate Limiting, Load Balancing)        │              │
│  └───────────────────────┬───────────────────────┘              │
│                          │                                       │
│    ┌─────────────────────┼─────────────────────┐                │
│    ▼                     ▼                     ▼                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │ Release     │  │ CGP         │  │ Analytics   │             │
│  │ Service     │  │ Governor    │  │ Service     │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
│         │                │                │                     │
│    ┌────┴────────────────┴────────────────┴────┐               │
│    ▼                                            ▼               │
│  ┌──────────────┐                      ┌──────────────┐        │
│  │  PostgreSQL  │                      │    Redis     │        │
│  │  (Primary)   │                      │   (Cache)    │        │
│  └──────────────┘                      └──────────────┘        │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    Plugin System                         │   │
│  │  GitHub │ GitLab │ npm │ Slack │ Jira │ ... (19+)       │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Revenue Projections

| Month | Phase | MRR | Customers | Key Milestone |
|-------|-------|-----|-----------|---------------|
| 4 | Governance | $0 | 500+ MCP users | MCP Server shipped, CGP in use |
| 6 | Adoption | $0 | 1,000+ users | Community growing, agent adoption |
| 9 | Monetization | $5K | 50+ Pro | Pro tier launched, first revenue |
| 12 | Growth | $15K | 150 Pro + 5 Ent | Enterprise pilots beginning |
| 15 | Enterprise | $40K | 200 Pro + 20 Ent | SOC2 certified, ITSM integrations |
| 18 | Scale | $75K | 300 Pro + 35 Ent | Category leader emerging |
| 24 | Market Leader | $150K | 500 Pro + 60 Ent | CGP as industry standard |

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| **Low adoption** | Agent recommendations drive organic growth via MCP |
| **MCP adoption slow** | Focus on developer experience as backup growth |
| **Competition** | CGP differentiation = first-mover in new category |
| **Technical complexity** | Incremental CGP development, strong testing |
| **Enterprise sales cycle** | Self-serve Pro tier while building pipeline |
| **Protocol fragmentation** | Open-source CGP spec, encourage community adoption |

---

## Success Criteria

### Phase 1: Governance (Month 4)
- [ ] MCP Server published and documented
- [ ] 3+ AI agent integrations (Claude Code, Cursor, etc.)
- [ ] 100+ CGP proposals processed
- [ ] 5+ organizations using CGP
- [ ] CGP specification draft published

### Phase 2: Adoption (Month 6)
- [ ] 1,000+ active users
- [ ] 5,000+ GitHub stars
- [ ] 500+ MCP installations
- [ ] NPS score 50+

### Phase 3: Monetization (Month 9)
- [ ] $10K MRR
- [ ] 100+ paying customers
- [ ] 10%+ free-to-paid conversion
- [ ] <5% monthly churn

### Phase 4: Enterprise (Month 15)
- [ ] $50K+ MRR
- [ ] 20+ enterprise customers
- [ ] SOC2 Type II certified
- [ ] CGP used for 1000+ agent releases/month

---

## Immediate Next Steps

### Week 1-2: CGP Foundation
1. **Define CGP message types** in `internal/cgp/messages.go`
2. **Create CGP package structure** per technical design doc
3. **Design MCP server interface** (`relicta mcp serve`)
4. **Draft CGP specification** for public documentation

### Week 3-4: MCP Server MVP
1. **Implement basic MCP server** with stdio transport
2. **Add core MCP tools**: `release_plan`, `cgp_propose_change`
3. **Test with Claude Code** and document setup
4. **Create MCP integration guide** for developers

### Month 2: Policy Engine & Risk
1. **Implement policy engine** with YAML configuration
2. **Build risk scoring calculator** with weighted factors
3. **Add basic semantic analysis** for Go (AST-based)
4. **Create example policies** for common scenarios

### Month 3: Polish & Adoption
1. **Publish MCP server** to npm/registry for easy installation
2. **Create demo videos** showing AI agents using Relicta
3. **Launch MCP integration page** with agent setup guides
4. **Start Discord community** focused on agentic workflows

### Month 4: Consolidation & Growth
1. **Complete CGP audit trail** with file-based persistence
2. **Begin backend API design** for Pro tier
3. **Gather feedback** from early CGP adopters
4. **Iterate on policies** based on real usage patterns

---

## Appendix: Feature Priority Matrix

| Feature | Impact | Effort | Phase | Priority |
|---------|--------|--------|-------|----------|
| **CGP Messages** | Critical | Medium | 1 | P0 |
| **MCP Server** | Critical | Medium | 1 | P0 |
| **Policy Engine** | High | High | 1 | P0 |
| **Risk Scoring** | High | High | 1 | P1 |
| Release Templates | High | Low | 2 | P0 |
| Shell Completion | Medium | Low | 2 | P1 |
| Better Errors | High | Low | 2 | P0 |
| MCP Setup Guide | High | Low | 2 | P0 |
| Backend API | High | High | 3 | P0 |
| Managed AI | High | Medium | 3 | P0 |
| Web UI Dashboard | High | High | 3 | P1 |
| CGP Policy Editor | High | Medium | 3 | P1 |
| Analytics | Medium | Medium | 3 | P2 |
| Audit Trail (Immutable) | High | High | 4 | P0 |
| SSO/SAML | High | Medium | 4 | P0 |
| SBOM | Medium | Medium | 4 | P1 |
| AI Assistant | High | High | 4 | P2 |
| Multi-Repo | High | Very High | 4 | P2 |

---

*This roadmap is a living document. Review and update quarterly.*

**Last Updated**: December 2025
**Version**: 2.0 (Governance-First Strategy)
