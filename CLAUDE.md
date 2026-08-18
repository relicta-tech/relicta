# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Relicta is the governance layer for software change. As AI agents and CI systems generate more code, deciding what should ship becomes the hardest problem. Relicta governs change — before it reaches production.

**Today**, it's a production-ready CLI that automates semantic versioning, release notes, approvals, and publishing. **Tomorrow**, it's the decision layer for risk-aware releases in an AI-driven world.

Built in Go for security, performance, and single-binary distribution. Features the Change Governance Protocol (CGP) for risk assessment, audit trails, and approval workflows.

## Build Commands

```bash
# Build
make build                    # Build binary to bin/relicta
make install                  # Install to $GOPATH/bin

# Test
make test                     # Run unit tests with race detection
make test-integration         # Run integration tests

# Lint
make lint                     # Run golangci-lint

# Release
make release-snapshot         # Build snapshot release (no publish)
make release                  # Full release with goreleaser
```

## Technical Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.25+ |
| CLI Framework | Cobra |
| Configuration | Viper (YAML, JSON, env vars) |
| Git Operations | go-git (pure Go) |
| Plugin System | HashiCorp go-plugin (gRPC) |
| Terminal UI | Charmbracelet (bubbletea, lipgloss) |
| HTTP Server | chi router |
| Persistence | `persistence.backend`: `file` (default), `sqlite` via modernc.org/sqlite, or `postgres` via pgx/v5 |
| AI Clients | go-openai, anthropic-sdk-go, HTTP for Gemini and Ollama |

## Architecture

```
cmd/relicta/                # Entry point
internal/
├── cli/                    # Cobra commands (36 files, ~13K LOC)
├── domain/                 # Hexagonal core (release aggregate, value objects, ports)
│   ├── release/            # ReleaseRun aggregate, state machine, events
│   ├── changes/            # Change classification
│   ├── communication/      # Audience-aware narratives
│   ├── monorepo/           # Workspace versioning
│   ├── multirepo/          # Repo group governance
│   ├── sourcecontrol/      # Git port
│   ├── version/            # Semver
│   └── workspace/          # Workspace detection
├── application/            # Use-case orchestration (governance, blast, supplychain, monorepo, multirepo, versioning)
├── infrastructure/         # Adapters (git, ai, persistence, webhook, template, workspace, observability)
│   ├── ai/                 # OpenAI/Anthropic/Gemini/Ollama provider abstraction (5,914 LOC)
│   ├── git/                # go-git adapter
│   ├── persistence/        # File event store + PostgreSQL adapter
│   ├── webhook/            # Outbound delivery queue + retry
│   └── observability/      # Prometheus metrics + inbound webhook receiver
├── cgp/                    # Change Governance Protocol — risk, policy, audit, autoapproval, reputation, memory, attribution, identity, dsl, evaluator
├── compliance/             # SOC2 + DORA report generation (templates + markdown/JSON output)
├── httpserver/             # Dashboard REST API (chi router) + WebSocket + middleware (JWT, OIDC, RBAC)
├── mcp/                    # MCP server for AI agent integration (~9.5K LOC)
├── security/               # Token (JWT/HS256), OIDC, attestation
├── observability/          # Prometheus + receiver wiring
├── plugin/                 # gRPC plugin manager + sandbox (best-effort on darwin)
├── analytics/              # Risk trends, decision distribution, team metrics
├── config/                 # Viper-based config loading
├── service/                # Release service composition root
├── ui/                     # Terminal UI components (lipgloss)
└── container/              # Dependency injection wiring

pkg/cgp/                    # Public CGP SDK — wire format, message types, validation
plugins/                    # Plugin registry (separate plugin repos referenced via registry.yaml)
web/                        # Vue 3 + Vite + Tailwind dashboard frontend
```

## Core Commands

| Command | Purpose |
|---------|---------|
| `relicta init` | Set up config and default options (8-step wizard) |
| `relicta plan` | Analyze changes and assess risk since last release |
| `relicta bump` | Calculate and apply semver version |
| `relicta notes` | Generate AI-powered release notes |
| `relicta evaluate` | CGP risk evaluation against policy |
| `relicta approve` | Governance gate with audit trail |
| `relicta publish` | Execute release: tag, changelog, notify, publish |
| `relicta release` | Complete workflow (plan → bump → notes → approve → publish) |
| `relicta promote` | Promote between channels (alpha/beta/rc/stable) |
| `relicta communicate` | Audience-aware release narratives |
| `relicta blast` | Blast radius analysis |
| `relicta verify` | Verify release attestations and signatures |
| `relicta rollback` | Roll back to previous version |
| `relicta group` | Multi-repo group operations |
| `relicta report` | Generate SOC2 / DORA / summary reports |
| `relicta mcp serve` | MCP server for AI agent integration |
| `relicta server` | Standalone dashboard API server |
| `relicta db migrate` | PostgreSQL migration runner |
| `relicta db import` | Import the `.relicta` history — release runs and governance memory — into the configured backend |
| `relicta plugin` | Plugin management (list/create/dev/registry/search) |

## Plugin System

- Plugins run as separate processes via HashiCorp go-plugin (gRPC)
- Hook-based lifecycle: `PreVersion`, `PostNotes`, `PostPublish`, etc.
- Plugin registry at `plugins/registry.yaml` references external plugin repositories (each plugin is its own Go module/repo)
- Sandbox: best-effort on darwin (RLIMIT_AS unenforced on Apple Silicon — see `internal/plugin/sandbox/sandbox_darwin.go`)

## Configuration

Config file: `.relicta.yaml` (also supports JSON, searched in `.` and `~/.config/relicta/`)

Environment variables override config with `RELICTA_` prefix.

## Documentation

- **PRD:** `docs/internal/prd.md` - Product requirements and feature specifications
- **Technical Design:** `docs/internal/technical-design.md` - Architecture, interfaces, and implementation details
