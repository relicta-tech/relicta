# Landing Page Enrichment — Relicta

Copy-ready sections to add to relicta.tech. Each section includes heading, body copy, and structured data for implementation.

---

## Section 1: Competitive Comparison Table

**Placement:** After "Production Ready" section, before "The Evolution"

### Why Relicta?

Other tools automate releases. Relicta governs them.

| Capability | semantic-release | release-please | goreleaser | LaunchDarkly | ServiceNow | Relicta |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| Semantic versioning | Yes | Yes | Yes | — | — | Yes |
| AI release notes | — | — | — | — | — | 5 providers |
| Multi-audience narratives | — | — | — | — | — | 4 audiences |
| Risk scoring | — | — | — | Basic | Basic | 7-factor + learning |
| Approval workflows | — | — | — | — | ITIL | Policy DSL |
| Audit trail | — | — | — | Flags only | Yes | Cryptographic |
| MCP server (AI agents) | — | — | — | — | — | Native |
| Blast radius analysis | — | — | — | — | — | Yes |
| Monorepo support | Plugin | Yes | Yes | — | — | Yes + dependency graph |
| Multi-repo federation | — | — | — | — | — | Yes |
| Supply chain attestation | — | — | Partial | — | — | in-toto + Sigstore |
| Single binary | — | — | Yes | — | — | Yes |
| Open source | MIT | Apache | MIT | — | — | MIT |

---

## Section 2: The 8 Things Only Relicta Does

**Placement:** New section after comparison table

### What no other tool offers

#### 1. Change Governance Protocol (CGP)

The first standardized, vendor-neutral protocol for governing software change. Actor-agnostic — works with humans, AI agents, and CI systems simultaneously. Transport-independent — speaks MCP, HTTP, and gRPC.

```yaml
# Policy DSL example
rule: high-risk-release
  when: risk_score > 0.7 AND has_breaking_changes
  then: require_approval(count: 2, team: "platform")
```

#### 2. MCP-Native AI Agent Integration

The only release tool that's an MCP server. Claude, GPT, and custom agents can plan, assess risk, approve, and publish releases — with governance guardrails built in.

```
You: "What's the release risk for this project?"

Agent: Risk score: 0.62 (medium). 3 breaking API changes detected
       across 2 packages. Blast radius: auth-service, api-gateway.
       Recommending v2.0.0 with 2-approver gate.

You: "Approve and publish."

Agent: Release v2.0.0 published. GitHub release created,
       Slack notification sent, attestation signed.
```

#### 3. Multi-Factor Risk Scoring with Learning

Seven weighted factors — API changes, blast radius, dependency impact, security, historical risk, actor trust, and test coverage — that learn from past releases and incidents. Gets smarter every time you ship.

#### 4. Blast Radius Analysis

Before you release, Relicta maps changed files to impacted packages, builds dependency graphs, and quantifies the scope of change across your monorepo. Language-aware for Go, Python, npm, and Rust.

#### 5. Audience-Specific Narratives

One release, four stories. Engineering gets the technical changelog. Product gets feature highlights. Executives get business impact. Customers get the upgrade guide. AI-generated or template-based.

#### 6. Release Memory

Relicta remembers every release, every incident, every actor. It tracks reliability scores, correlates incidents to releases, and surfaces risk patterns over time. Your release intelligence compounds.

#### 7. SLSA Governance Attestation

The first tool to couple governance decisions with in-toto v1 attestation statements, signed via Sigstore. Proves not just what was built, but why it was approved and by whom — with cryptographic verification.

#### 8. Policy DSL

A full rules engine for governance decisions. Composable conditions, team-based approvals, time windows, auto-approval for low-risk changes. Write policies as code, version them with your repo.

---

## Section 3: How It Works (Expanded)

**Placement:** Replace or expand the existing "Production Ready" section

### From commit to production in one command

```bash
# Install
brew install relicta-tech/tap/relicta

# Initialize
relicta init

# Complete governed release
relicta release
```

#### What happens under the hood

```
relicta release
  |
  |-- plan      Analyze 47 commits since v1.3.0
  |             Conventional commit parsing + heuristic fallback
  |             Detect 2 breaking changes, 8 features, 12 fixes
  |             Calculate risk score: 0.58 (medium)
  |             Blast radius: 3 of 7 packages impacted
  |             Suggest: v2.0.0 (major)
  |
  |-- bump      Apply v2.0.0
  |             Create git tag
  |
  |-- notes     Generate release notes via Claude/GPT/Ollama
  |             Engineering changelog + product summary
  |             Migration guide for breaking changes
  |
  |-- approve   Policy check: risk > 0.5 requires approval
  |             Auto-approved: actor trust score 0.94
  |             Audit trail: SHA-256 hash chain entry
  |
  |-- publish   Create GitHub release
  |             Push to npm registry
  |             Notify #releases on Slack
  |             Sign in-toto attestation via Sigstore
  |
  Done. v2.0.0 released with full governance trail.
```

---

## Section 4: Trust & Security

**Placement:** New section before segments

### Enterprise-grade security, developer-grade UX

- **Zero data leaves your environment.** Relicta runs locally or in your CI. No SaaS. No cloud dependency. Your code, your keys, your infrastructure.

- **Cryptographic audit trails.** Every governance decision is recorded in an immutable SHA-256 hash chain. Tamper-evident by design.

- **SLSA attestation.** in-toto v1 statements signed via Sigstore prove what was released, why it was approved, and by whom.

- **SBOM generation.** Full software bill of materials for supply chain transparency.

- **Plugin sandboxing.** Plugins run in isolated processes with capability-based restrictions on filesystem, network, and environment access.

- **Secret masking.** API keys and tokens are automatically redacted from all output and logs.

---

## Section 5: MCP Integration (Expanded)

**Placement:** Expand the existing "AI-Native" section

### The first release tool built for AI agents

MCP (Model Context Protocol) is the industry standard for connecting AI agents to tools. Relicta is a native MCP server — not a wrapper, not a plugin.

#### What agents can do

| Tool | What it does |
|------|-------------|
| `relicta.plan` | Analyze commits, assess risk, suggest version |
| `relicta.bump` | Apply semantic version |
| `relicta.notes` | Generate AI-powered release notes |
| `relicta.evaluate` | CGP risk evaluation with blast radius |
| `relicta.approve` | Governance gate with audit trail |
| `relicta.publish` | Tag, changelog, plugins, attestation |
| `relicta.blast_radius` | Monorepo change impact analysis |
| `relicta.infer_version` | Lightweight version inference |
| `relicta.summarize_diff` | Audience-tailored change summaries |
| `relicta.validate_release` | Pre-flight release validation |

#### What agents can read

| Resource | Content |
|----------|---------|
| `relicta://state` | Current release state machine |
| `relicta://config` | Project configuration |
| `relicta://commits` | Commits since last release |
| `relicta://changelog` | Generated changelog |
| `relicta://risk-report` | CGP risk assessment |

#### Works with

- Claude Desktop & Claude Code
- GPT-based agents via MCP bridge
- Custom agents via stdio or HTTP transport
- CI pipelines via GitHub Action

```json
{
  "mcpServers": {
    "relicta": {
      "command": "relicta",
      "args": ["mcp", "serve"]
    }
  }
}
```

---

## Section 6: Governance for Scale

**Placement:** Expand or replace "Segments" section

### Platform Engineers

Standardize release workflows across hundreds of services. One config, consistent governance, full observability.

- Monorepo support with independent, lockstep, and hybrid versioning
- Multi-repo federation with dependency-aware coordination
- Prometheus metrics for release pipeline monitoring

### Release Managers

Regain visibility without slowing anyone down. Risk scores surface what needs attention; auto-approval handles what doesn't.

- Risk-based approval gates — low risk ships automatically
- Actor trust scoring — earn autonomy through track record
- Real-time dashboard with WebSocket streaming

### Security & Compliance

Audit trails and attestation built into the pipeline, not bolted on.

- Immutable hash-chain audit log
- SLSA in-toto attestation with Sigstore signing
- SBOM generation for supply chain transparency
- OIDC/SSO with role-based access (Okta, Azure AD, Google)

### Teams Adopting AI Agents

Safe guardrails for AI-generated code. The agent proposes; the protocol decides.

- CGP protocol governs agent-initiated changes
- Policy DSL defines what agents can auto-approve
- Actor trust scores track agent reliability over time
- Full audit trail of every agent action

---

## Section 7: Social Proof / Numbers

**Placement:** After hero section or before CTA

### The numbers

- **14** governance hooks in the plugin lifecycle
- **7** weighted risk factors with historical learning
- **5** AI providers (OpenAI, Anthropic, Gemini, Azure, Ollama)
- **4** audience-specific narrative types
- **1** binary. Zero dependencies. MIT licensed.

---

## Section 8: Bottom CTA

**Placement:** Page footer, before links

### Start governing change today

```bash
brew install relicta-tech/tap/relicta
relicta init
relicta release
```

One binary. Zero cloud dependencies. Full governance trail.

[Get Started](https://docs.relicta.tech) | [View on GitHub](https://github.com/relicta-tech/relicta) | [Read the CGP Spec](https://docs.relicta.tech/cgp/)

---

## Implementation Notes

### Missing from current landing page

1. **No competitive comparison** — visitors can't see why Relicta vs semantic-release
2. **No feature depth** — CGP, Policy DSL, blast radius, release memory are not mentioned
3. **MCP section is thin** — doesn't show the full tool/resource inventory
4. **No security story** — attestation, SBOM, sandboxing not highlighted
5. **No "how it works" flow** — the plan→bump→notes→approve→publish pipeline isn't visualized
6. **No numbers** — no concrete stats that create credibility

### Recommended page structure

```
1. Hero (existing — strong)
2. Problem/Gap (existing — strong)
3. Solution overview (existing — needs depth)
4. How it works (NEW — pipeline visualization)
5. The 8 differentiators (NEW — what only Relicta does)
6. Competitive comparison table (NEW)
7. Trust & Security (NEW)
8. MCP / AI agents (EXPAND existing)
9. Segments (EXPAND existing)
10. Numbers (NEW)
11. CTA (EXPAND existing)
```

### SEO keywords to target

- "release governance tool"
- "AI release management"
- "MCP server release"
- "semantic versioning with risk assessment"
- "change governance protocol"
- "release approval workflow CLI"
- "monorepo release governance"
- "AI agent release automation"
- "SLSA release attestation"
- "developer-friendly change management"
