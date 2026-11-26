<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="brand/relicta-logo-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="brand/relicta-logo-light.svg">
    <img alt="Relicta" src="brand/relicta-logo-light.svg" width="280">
  </picture>

  <p><strong>Turn repository changes into reviewed releases, customer-facing notes, and an auditable approval record.</strong></p>

  <p>
    <a href="https://github.com/relicta-tech/relicta/actions/workflows/ci.yaml"><img src="https://github.com/relicta-tech/relicta/actions/workflows/ci.yaml/badge.svg" alt="CI"></a>
    <a href="https://github.com/relicta-tech/relicta/actions/workflows/codeql.yaml"><img src="brand/nox-badge.svg" alt="nox"></a>
    <a href="https://github.com/relicta-tech/relicta/actions/workflows/ci.yaml"><img src="https://img.shields.io/badge/coverage-gated_by_CI-blue" alt="Coverage enforced in CI"></a>
    <a href="https://goreportcard.com/report/github.com/relicta-tech/relicta"><img src="https://goreportcard.com/badge/github.com/relicta-tech/relicta" alt="Go Report Card"></a>
    <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  </p>
</div>

Relicta helps release owners turn technical changes into a reviewable release packet: a proposed
version, structured change summary, release notes, governance decision, and approval record. In
the unreleased v5 workflow, a coding agent can draft customer-facing notes from that summary using
product context already present in its session; a human reviews the saved text before approval.

**Availability:** This repository documents the unreleased v5 source. The latest public release is
[v4.2.0](https://github.com/relicta-tech/relicta/releases/tag/v4.2.0); capabilities described below
may not be included in that release. See [Installation](#installation) for the currently published
version and verified install options.

Relicta records release proposals and governance decisions in a hash-chained audit trail with actor-typed governance (`human` / `ci` / `agent`), per-actor autonomy budgets, and exportable evidence. Agent-authored release notes are stored as supplied and are not fact-checked by Relicta. These reports can support an organization’s compliance work; they do not establish compliance or replace legal and control-owner review.

## What Relicta is (and isn't)

Relicta governs the **software release lifecycle**: it analyzes repository changes, proposes a
version, prepares release communication, applies release policy, records approvals, and publishes
artifacts. Its records connect the code changes and release decision to the resulting release.

Relicta is not a general compliance-monitoring platform and does not mediate every action an AI
agent takes while it is running. It can export release evidence to GRC platforms such as
[Vanta](https://www.vanta.com/products/ai-governance) and [Drata](https://drata.com/products/agent-governance),
which cover broader governance programs and describe agent inventory or control capabilities.
Runtime controls such as [Microsoft's Agent Governance Toolkit](https://github.com/microsoft/agent-governance-toolkit)
focus on enforcing policies around agent execution. These tools address adjacent parts of the
software governance lifecycle; Relicta focuses on the release decision and its evidence.

## Features

### EU AI Act evidence exports (supporting material)

- **Article 12 supporting evidence**: Export release-governance decisions, CGP protocol version, change metadata, and recorded rationale as JSONL or CSV. Decisions are discovered through release records dated in the selected period; standalone decisions without a matching release record are not enumerated. Approval-event identities and audit-chain hashes are not included, and report generation does not verify chain integrity. These records are not automatically generated logs of each AI system use and cannot by themselves establish that Article 12 logging duties are met. The report shows a suggested date six months after its period ends. Articles 19(1) and 26(6) set role-specific minimum retention periods for certain logs, subject to intended purpose and other applicable law; confirm actual duties. Relicta does not determine role-specific duties or enforce storage retention or deletion.
- **Annex IV supporting-evidence draft**: Generate an eight-section draft from available release-governance data, with missing provider documentation marked for completion. It is not complete target-system technical documentation or a conformity assessment. The conformity-declaration section is a scaffold requiring completion and review by the system provider and its legal/control owners. Article 18(1) sets a ten-year period for specified provider documentation from placing the system on the market or putting it into service. Relicta does not collect that event date or calculate a statutory deadline, and does not enforce storage retention or deletion.

These exports are supporting material for an organization’s compliance work. They cover only data Relicta has recorded, can contain gaps or placeholders, and do not establish that a system complies with the EU AI Act. Review them against the applicable requirements with qualified legal and control owners before relying on or submitting them.

### Core Governance

- **Change Governance Protocol (CGP)**: Public SDK (`pkg/cgp/`) for risk assessment, actor trust scoring, and security impact analysis
- **Hash-Chained Audit Trail**: SHA-256 links over canonical CGP decisions; verify them with `relicta audit verify`. Compliance report generation does not check chain integrity.
- **Actor-Typed Governance**: First-class `human` / `ci` / `agent` / `system` actor kinds drive policy decisions
- **Agent Autonomy Profiles**: Per-actor budgets cap blast radius, risk score, dollar cost, allowed/denied tools, time windows, and require human cosigners for privileged operations
- **Policy DSL**: Declarative policy rules with what-if comparison and matrix testing
- **Blast Radius Analysis**: Quantify the scope and impact of changes before they ship
- **Approval Workflow**: Review and approve releases with full audit trails
- **Release Attestation & Signing**: Generate and verify governance attestations; release artifact signing is provided by the repository's release workflow
- **MCP Server**: Claude / GPT / Cursor / Devin agents drive releases through the same governance gates as humans

See the [CGP Guide](docs/governance.md) for policy DSL, risk scoring, and approval workflows, [Deployment Evidence](docs/deployment-evidence.md) for recording that a release reached an environment and gating deployments on governance, the [CGP Specification](docs/cgp-specification.md) for the complete protocol definition, [custom changelog templates](docs/changelog-templates.md) for release entry customization, and the [Skill Publishing Checklist](docs/skill-publishing-checklist.md) for sharing the Relicta governance skill publicly.

### Compliance Integrations (Push, don't replace)

- **Vanta** evidence push: `relicta integrations vanta push --type article12|soc2` maps Relicta release-governance artefacts to Vanta custom evidence.
- **Drata** evidence push: `relicta integrations drata push` exports supported evidence types to Drata.

### Release Automation

- **Automatic Version Calculation**: Semantic version bumps from conventional commits
- **Smart Commit Intelligence**: Infers intent from unstructured histories via heuristics and optional AI
- **Optional AI-Enhanced Release Notes**: Enable a configured provider (OpenAI, Anthropic, Gemini, Azure, or Ollama) for AI-written prose; deterministic notes remain the default when AI is disabled.
- **Agent-Session Customer Notes (v5 source, not yet released)**: Draft release notes with the product context already in your MCP agent conversation; Relicta supplies the version and change recommendation, stores the draft as agent-authored, and keeps it in the normal review and approval flow. The `release-summary` MCP prompt fetches a read-only recommendation before drafting; saving notes is a separate step, and approval and publication remain separate. The current public v4.2.0 release does not include this workflow. See the [MCP release-notes workflow](docs/mcp.md#draft-customer-facing-release-notes-in-your-agent-session).
- **Communication Narratives**: Audience-specific release communication (engineering, product, executive, external) with AI generation when configured and template-based fallback
- **Pre-release Channels**: Alpha, beta, and RC versions with channel promotion workflow (`--channel` flag). The built-in RC channel is named `next`.
- **Promotion governance**: `relicta promote` creates the target channel tag without a per-channel approval gate; review promotions through your existing release process.
- **Plugin System**: Extensible via gRPC plugins (verified: GitHub, GitLab, Slack, Jira, npm; community-listed entries pending audit)

### Multi-Project Support

- **Monorepo**: Analyze configured package paths and version changed packages independently. Package-level lockstep, hybrid, and `monorepo.release_groups` strategies are not supported; configuration using them is rejected.
- **Multi-repo**: Separately, `repository_groups` can plan and publish already-approved member releases in dependency order. Cross-repository CGP federation is not wired into the group workflow yet.

### Authentication & Access Control

- **Dashboard authentication**: API-key, password-session, and OIDC sign-in in the embedded UI, with configurable role mapping
- **Role-Based Access**: Admin, Approver, and Viewer roles

The embedded UI stores API keys persistently and session tokens for the current browser tab. OIDC browser sign-in uses a short-lived, single-use ticket before establishing the session; multi-replica deployments need session affinity during OIDC login, callback, and ticket exchange because those temporary handoff records are process-local. See the [dashboard authentication guide](docs/dashboard.md#authentication-modes) for supported server modes and configuration.

### Persistence

- **File-based** (default): Zero-dependency local state
- **SQLite** (opt-in): One database per repository, no server and no cgo
- **PostgreSQL** (opt-in): Shared state for teams with `db migrate` and `db status` commands
- **Migration is explicit**: switching `persistence.backend` leaves the existing history where
  it is — `relicta db import` moves it, and never deletes the JSON tree it read

### Dashboard & API

- **REST API v1**: OpenAPI-specified endpoints with health and readiness probes
- **Real-time Events**: WebSocket and SSE streaming for live release status
- **Standalone Frontend Container**: Nginx-served dashboard UI that proxies API and WebSocket traffic to a separately configured Relicta server; see the [dashboard deployment guide](docs/dashboard.md#standalone-frontend-container)

### Observability

- **Prometheus Monitoring**: Optional metrics endpoint and configurable Prometheus queries for post-release health checks
- **Alert Webhooks**: Accept configured alert events from Alertmanager, PagerDuty, Datadog, and generic senders
- **Deployment Health Monitoring**: Watch configured post-release error-rate and latency signals
- **Release-Incident Correlation**: Find potential links using time proximity, service names, and deployment labels; review the reported confidence and reasons

## Installation

### Homebrew (Linux)

```bash
brew install relicta-tech/tap/relicta
```

Homebrew installation is currently supported on Linux. The current v4.2.0 release includes
unsigned macOS binaries, which Gatekeeper may block. Apple signing and notarization credentials
are not configured for the next release, so new releases will include Linux and Windows binaries
only until those credentials are configured. The existing v4.2.0 tap Formula still offers its
unsigned Darwin archives; avoid Homebrew on macOS and use `go install` or build from source.

### Download Binary

For Linux on x86_64 or ARM64, use the [verified direct-download steps](docs/quick-start.md#installation). They require Cosign and GitHub CLI, verify the signed checksum manifest and archive digest, and check GitHub build provenance before installation. For Windows amd64, follow the [PowerShell installation steps](docs/quick-start.md#verified-windows-binary-amd64), which perform the same checks before extracting the executable.

### Using Go

The current public v4.2.0 module requires Go 1.25 or newer. Install that published version
until the v5 module is released; v5 requires Go 1.26 or newer. On Windows, add Go's bin
directory to the current PowerShell session if `relicta` is not already on `PATH`:

```bash
go install github.com/relicta-tech/relicta/v4/cmd/relicta@v4.2.0
```

The v5 source in this checkout is not published yet. After v5 is released, use
`go install github.com/relicta-tech/relicta/v5/cmd/relicta@latest`.

This builds the CLI without the embedded Vue dashboard. The current release configuration embeds
the dashboard in release binaries; until a release built with that configuration is published,
use `make build-with-frontend` from a source checkout when you need the web UI.

```powershell
$goBin = Join-Path (go env GOPATH) "bin"
$env:Path = "$env:Path;$goBin"
relicta version
```

### From Source

```bash
git clone https://github.com/relicta-tech/relicta.git
cd relicta
make build
sudo mv bin/relicta /usr/local/bin/
```

### GitHub Actions

For a read-only release plan in CI, install a pinned Relicta module version with Go:

```yaml
name: Relicta plan

on: workflow_dispatch

permissions:
  contents: read

jobs:
  plan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v6
        with:
          fetch-depth: 0
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v6
        with:
          go-version: '1.26'
      - run: go install github.com/relicta-tech/relicta/v4/cmd/relicta@v4.2.0
      - run: relicta plan --dry-run --json
```

The current [Relicta GitHub Action](https://github.com/relicta-tech/relicta-action) release warns
and continues when binary checksum verification fails, so pinning its source commit does not make
the downloaded binary trustworthy. Use the Go module path above or the verified
[direct-download procedure](docs/quick-start.md#installation) for production workflows. Wait for a
fail-closed Action release before using that Action to execute Relicta in CI.

## Quick Start

> **New to Relicta?** See the [Quick Start Guide](docs/quick-start.md) for a comprehensive walkthrough.

### One-Command Release

Preview first, then review the proposed version and configured version-file changes. The dry run
does not generate release notes or evaluate governance. A live release can write changelog/version
files, create a local tag, run publisher plugins, and push if configured. Run it only when you
intend to execute that release, and review its generated notes before approving:

```bash
# Execute an explicitly reviewed release; prompts for approval
relicta release

# Preview changes without releasing
relicta release --dry-run

# CI/CD only when unattended approval is an intentional policy
relicta release --yes --ci

# Pre-release to a channel
relicta release --channel beta
```

### Step-by-Step Workflow

For more control, use individual commands:

1. Initialize Relicta in your project:

```bash
relicta init
```

2. Plan a release (analyzes commits since last tag):

```bash
relicta plan
```

3. Evaluate risk using the Change Governance Protocol:

```bash
relicta evaluate
```

4. Calculate and apply the version:

```bash
relicta bump
```

5. Generate release notes (optionally with AI):

```bash
# Optional provider-generated notes follow the project configuration.
# For agent-authored notes, use the MCP flow: plan, bump, draft from the
# recommendation in this agent session, then pass the text to relicta_notes.
relicta notes
```

Planning, bumping, and saving notes prepare local release state; saving notes does not publish.
Review and approve the release before `relicta publish`, which creates the configured tag and
runs publisher plugins (and pushes the tag when configured).

6. Approve the release:

```bash
relicta approve
```

7. Publish the release:

```bash
relicta publish
```

## Configuration

Create a `.relicta.yaml` in your project root:

```yaml
versioning:
  strategy: conventional
  tag_prefix: v
  git_tag: true
  git_push: false # Local tag by default; set true to push and trigger tag-based CI

changelog:
  file: CHANGELOG.md
  format: keep-a-changelog
  repository_url: https://github.com/your-org/your-repo

ai:
  enabled: true
  provider: openai
  model: gpt-6.2-sol
  tone: professional
  audience: developers

persistence:
  backend: file            # file | sqlite | postgres — selects the release run store
  connection_string: ""    # DATABASE_URL for postgres
  migration_mode: manual   # postgres only: manual runs `relicta db migrate`, auto migrates on start

channels:
  enabled: true
  default: stable

communication:
  default_audience: engineering
  # Omit `audiences` to use Relicta's built-in audience profiles.

observability:
  providers:
    - name: production
      type: prometheus
      endpoint: https://prometheus.example.com
      bearer_token: ${PROMETHEUS_TOKEN}
  health_check:
    provider_name: production
    window: 30m
    error_rate_threshold: 5 # absolute error rate percent
    latency_threshold: 50 # p95 percentage increase from the first measured check
    # Override these if your Prometheus metrics use different names/labels.
    error_rate_query: 'sum(rate(http_requests_errors_total[5m])) / sum(rate(http_requests_total[5m])) * 100'
    latency_query: 'histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))'
  auto_record: false # opt in only after validating metrics and governance-memory storage

repository_groups:
  - name: platform
    repositories:
      - name: relicta
        path: ../relicta
      - name: relicta-action
        path: ../relicta-action
    strategy: coordinated

plugins:
  - name: github
    enabled: true
    config:
      draft: false
  - name: slack
    enabled: true
    config:
      webhook: ${SLACK_WEBHOOK_URL}

workflow:
  require_approval: true
  allowed_branches:
    - main
```

Relicta defaults to creating a local tag without pushing it. Set `versioning.git_push: true`
when you want Relicta to push tags; a push can start workflows in your repository. The GitHub
plugin creates GitHub Releases and needs a token with repository release permissions. Avoid
configuring multiple workflows to publish the same version on tag push. The enabled GitHub and
Slack plugin examples above are separate processes and are unsigned; commands that load them
require the explicit `--allow-untrusted-plugins` flag. See [Plugin Security](#plugins)
before enabling third-party plugins.

## Commands

| Command | Description |
|---------|-------------|
| `release` | **Run complete release workflow** (plan, bump, notes, approve, publish) |
| `init` | Create repository-aware defaults; use `init --guided` for interactive setup |
| `plan` | Analyze commits and plan the next release |
| `bump` | Apply the selected version to configured version files |
| `notes` | Generate changelog and release notes |
| `evaluate` | Evaluate risk using CGP (Change Governance Protocol) |
| `approve` | Review and approve the release |
| `publish` | Create the configured Git tag, run publisher plugins, and record the release |
| `verify` | Verify release attestations and signatures |
| `promote` | Promote a release between channels (e.g., beta to stable) |
| `communicate` | Generate audience-specific release narratives |
| `rollback` | Roll back to a previous version |
| `group plan` | Plan releases across a repository group |
| `group release` | Release across a repository group with dependency ordering |
| `group status` | View status of all repos in a group |
| `db migrate` | Run database migrations (PostgreSQL persistence) |
| `db status` | Show current migration status |
| `db import` | Import an existing `.relicta/releases` history into the configured backend |
| `server` | Start standalone API server (dashboard backend) |
| `mcp serve` | Start MCP server for AI agent integration |
| `eval run` | Run AI eval harness against the embedded golden corpus (gates model bumps) |
| `eval list` | List embedded golden cases |
| `integrations vanta push` | Push scoped release-governance evidence mapped to Article 12 concepts / SOC 2 evidence to Vanta |
| `integrations drata push` | Push scoped release-governance evidence mapped to Article 12 concepts / SOC 2 evidence to Drata |
| `plugin sandbox-status` | Show plugin enforcement limits, signature status, and platform caveats |
| `report --type eu-ai-act-article-12` | Release-governance evidence mapped to EU AI Act Article 12 concepts (jsonl / csv / markdown) |
| `report --type eu-ai-act-annex-iv` | Draft release-governance evidence mapped to EU AI Act Annex IV; provider documentation and assessment remain to be completed |

### Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | `-c` | Path to config file (default: `.relicta.yaml`) |
| `--dry-run` | | Preview changes without making modifications |
| `--verbose` | `-v` | Enable verbose output |
| `--json` | | Output in JSON format |
| `--ci` | | CI/CD mode: auto-approve, JSON output, non-interactive |
| `--model` | | AI model to use (format: `provider/model`) |
| `--redact` | | Redact secrets and API keys from output |
| `--allow-untrusted-plugins` | | Required to load enabled plugins; signatures are not verified and sandbox enforcement varies by platform. Review `relicta plugin sandbox-status` first |
| `--no-color` | | Disable colored output |
| `--channel` | | Target release channel (alpha, beta, rc/next, stable) |

### Command-Specific Flags

Most commands support short flags for convenience. Here are the commonly used ones:

| Command | Flag | Short | Description |
|---------|------|-------|-------------|
| `plan` | `--analyze` | `-a` | Include detailed commit analysis |
| `plan` | `--from` | `-f` | Starting point for analysis |
| `bump` | `--level` | `-l` | Override bump type (major/minor/patch) |
| `notes` | `--audience` | `-a` | Set the target audience for configured AI notes |
| `notes` | `--tone` | `-t` | Set the tone for configured AI notes |
| `notes` | `--language` | `-l` | Set the language for configured AI notes |
| `notes` | `--output` | `-o` | Write generated notes to a file |
| `notes` | `--emoji` | | Include emojis in generated notes |
| `approve` | `--yes` | `-y` | Auto-approve without prompting |
| `approve` | `--edit` | `-e` | Edit release notes before approving |
| `publish` | `--skip-approval` | `-A` | Skip the approval check |
| `publish` | `--skip-push` | `-P` | Skip pushing to remote |
| `publish` | `--skip-tag` | `-T` | Skip git tag creation |
| `publish` | `--skip-plugins` | `-G` | Skip running configured plugins |
| `release` | `--yes` | `-y` | Auto-approve releases |
| `cancel` | `--force` | `-f` | Force cancel without confirmation |
| `clean` | `--keep` | `-k` | Keep the last N release runs |
| `clean` | `--older-than` | `-o` | Remove runs older than duration |
| `blast` | `--from` | `-f` | Starting git reference |
| `blast` | `--to` | `-t` | Ending git reference |
| `communicate` | `--audiences` | | Select one or more narrative audiences |
| `promote` | `--from` | `-f` | Source channel |
| `promote` | `--to` | `-t` | Destination channel |
| `serve` | `--port` | `-p` | Port to listen on |
| `serve` | `--address` | `-a` | Address to listen on |

Run `relicta <command> --help` for the complete list of flags for any command.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENAI_API_KEY` | OpenAI API key for AI-powered features |
| `ANTHROPIC_API_KEY` | Anthropic API key for Claude-powered features |
| `GEMINI_API_KEY` | Google Gemini API key |
| `OLLAMA_HOST` | Ollama server address (default: `http://localhost:11434`) |
| `GITHUB_TOKEN` | GitHub token for creating releases |
| `SLACK_WEBHOOK_URL` | Slack webhook URL for notifications |
| `DATABASE_URL` | PostgreSQL connection string for persistent storage |
| `RELICTA_SERVER_MODE` | Enable standalone server mode (`true`/`false`) |
| `RELICTA_ALLOWED_ORIGINS` | Comma-separated CORS origins for the dashboard API |

## Conventional Commits

Relicta follows the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Types and Version Bumps

| Type | Version Bump | Description |
|------|-------------|-------------|
| `feat` | Minor | New features |
| `fix` | Patch | Bug fixes |
| `perf` | Patch | Performance improvements |
| `BREAKING CHANGE` | Major | Breaking changes |
| `docs` | - | Documentation only |
| `style` | - | Code style changes |
| `refactor` | - | Code refactoring |
| `test` | - | Test changes |
| `chore` | - | Maintenance tasks |

## Plugins

Relicta supports plugins for extending functionality:

Plugins are unsigned and execute as separate processes. Before enabling one, review
`relicta plugin sandbox-status`. Loading an enabled plugin requires
`--allow-untrusted-plugins` on Linux, macOS, and Windows; signatures are not verified on
any platform. The flag records this trust decision and does not verify the publisher or provide
a complete security sandbox.

### GitHub Plugin

Creates GitHub releases with release notes:

```yaml
plugins:
  - name: github
    config:
      draft: false
      prerelease: false
```

### npm Plugin

Publishes packages to npm registry:

```yaml
plugins:
  - name: npm
    config:
      access: public
      registry: https://registry.npmjs.org
```

### Slack Plugin

Sends release notifications to Slack:

```yaml
plugins:
  - name: slack
    config:
      webhook: ${SLACK_WEBHOOK_URL}
      notify_on_success: true
      notify_on_error: true
```

## MCP Integration

Relicta includes a [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server, enabling AI agents like Claude and GPT to manage releases directly. See the [MCP Guide](docs/mcp.md) for detailed integration instructions.

### Starting the MCP Server

```bash
# Stdio transport (default, for Claude Desktop)
relicta mcp serve

# Local HTTP transport (loopback only; remote use needs a trusted auth/TLS proxy)
relicta mcp serve --port 8080
```

### Available Tools

| Tool | Description |
|------|-------------|
| `relicta_status` | Get current release state and pending actions |
| `relicta_init` | Initialize Relicta configuration |
| `relicta_plan` | Analyze commits and plan the next release; returns a deterministic recommendation for agent-authored notes |
| `relicta_bump` | Set the release version before generating notes |
| `relicta_notes` | Generate notes or persist a draft written in the agent session |
| `relicta_evaluate` | Evaluate release risk using CGP (Change Governance Protocol) |
| `relicta_approve` | Approve the release for publishing |
| `relicta_publish` | Execute the release (create tags, run plugins) |
| `relicta_cancel` | Cancel an in-progress release |
| `relicta_reset` | Reset a failed or canceled release |
| `relicta_blast_radius` | Analyze impacted packages in a monorepo |
| `relicta_infer_version` | Infer a version without starting a full release plan |
| `relicta_summarize_diff` | Summarize changes for a selected audience |
| `relicta_validate_release` | Run release pre-flight checks |
| `cgp_propose` | Submit a change proposal for CGP evaluation |
| `cgp_authorize` | Authorize a change through CGP policy gates |
| `cgp_status` | Query CGP evaluation and authorization status |

For agent-authored customer notes, call `relicta_plan`, use its deterministic
`recommendation` as the source of release facts, call `relicta_bump`, then write
the customer-facing draft in the agent session and pass it to `relicta_notes`
using the `draft` field. Relicta persists that draft on the release for the
normal review and approval flow; it does not call an AI provider for this path.
For Relicta's own tag-triggered release workflow, commit the reviewed copy as
`docs/release-notes/<version>.public.md` before planning; CI then presents the
draft for protected review and publishes the approved text.

### Resources

| Resource | Description |
|----------|-------------|
| `relicta://state` | Current release state machine status |
| `relicta://config` | Relicta configuration |
| `relicta://commits` | Recent commits since last release |
| `relicta://changelog` | Generated changelog |
| `relicta://risk-report` | CGP risk assessment |
| `relicta://policy/current` | Active governance policy context |
| `relicta://risk-budget` | Default and per-actor risk budgets |
| `relicta://risk-budget/{actor_id}` | Risk budget for a specific actor |
| `relicta://recent-incidents` | Recent governance incidents |
| `relicta://compliance-frameworks` | Configured compliance framework context |
| `relicta://approval` | Approval requirements for the current change |

### Claude Desktop Integration

Add to your Claude Desktop `claude_desktop_config.json`:

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

### Client SDK

For AI agent developers, Relicta provides a Go client SDK:

```go
import "github.com/relicta-tech/relicta/internal/mcp"

// Create client with stdio transport
transport := mcp.NewStdioClientTransport(reader, writer)
client := mcp.NewClient(transport)

// Initialize and use
client.Initialize(ctx)

// Plan a release
plan, err := client.Plan(ctx, true, "")

// Get status
status, err := client.Status(ctx)
```

### Features

- **Streaming Support**: Progress updates for long-running operations
- **Multi-Repository**: Manage releases across multiple repositories
- **Plugin Integration**: List, inspect, and execute plugins via MCP

## Architecture

Relicta is built with Domain-Driven Design principles:

```
├── cmd/relicta/                  # CLI entry point
├── internal/
│   ├── application/              # Use cases
│   ├── domain/                   # Business logic
│   │   ├── changes/              # Commit analysis
│   │   ├── release/              # Release aggregate
│   │   ├── version/              # Semantic versioning
│   │   ├── communication/        # Audience-specific narratives
│   │   ├── multirepo/            # Multi-repo group coordination
│   │   └── workspace/            # Monorepo workspace detection
│   ├── analytics/                # Release analytics and metrics
│   ├── infrastructure/           # External adapters (persistence, APIs)
│   ├── observability/
│   │   └── providers/            # Prometheus, webhook receivers
│   ├── security/
│   │   └── oidc/                 # OIDC/SSO provider integration
│   ├── cli/                      # Command implementations
│   ├── mcp/                      # MCP server & client SDK
│   ├── cgp/                      # Change Governance Protocol (internal)
│   └── service/                  # Application services
├── pkg/
│   ├── cgp/                      # CGP public SDK
│   └── plugin/                   # Plugin interface
└── plugins/                      # Official plugins
```

## Development

### Prerequisites

- Go 1.26+
- Git

### Build

```bash
make build
```

### Test

```bash
make test
```

### Lint

```bash
make lint
```

### Pre-commit Hooks

Set up pre-commit hooks to catch issues before pushing (recommended):

```bash
make install-hooks
```

This installs a git hook that runs before each commit:
- **Format check** (`gofmt`) - Verifies code formatting
- **Vet** (`go vet`) - Runs Go static analysis
- **Lint** (`golangci-lint` + `gocritic`) - Static analysis and code quality
- **Test compile** - Compiles unit tests without executing them
- **Coverage** (`coverctl`) - Checks test coverage thresholds
- **Security** (`nox`) - Runs security scanning

To manually run fast pre-commit checks:

```bash
make check
```

To run full CI-equivalent checks locally:

```bash
make check-ci
```

To skip the hook temporarily (use sparingly):

```bash
git commit --no-verify
```

## Contributing

1. Fork the repository
2. Set up pre-commit hooks: `make install-hooks`
3. Create a feature branch (`git checkout -b feature/amazing-feature`)
4. Commit your changes using conventional commits
5. Push to the branch (`git push origin feature/amazing-feature`)
6. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) for details.
