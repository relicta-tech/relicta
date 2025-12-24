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
| Language | Go 1.22+ |
| CLI Framework | Cobra |
| Configuration | Viper (YAML, JSON, env vars) |
| Git Operations | go-git (pure Go) |
| Plugin System | HashiCorp go-plugin (gRPC) |
| Terminal UI | Charmbracelet (bubbletea, lipgloss) |
| AI Clients | go-openai, anthropic-sdk-go, HTTP for Ollama |

## Architecture

```
cmd/relicta/          # Entry point
internal/
├── cli/                    # Cobra commands (init, plan, version, notes, approve, publish)
├── service/                # Core business logic
│   ├── git/               # Git operations, conventional commit parsing
│   ├── version/           # Semver calculations, changelog generation
│   ├── ai/                # AI provider abstraction (OpenAI, Anthropic, Ollama)
│   └── template/          # Go template rendering
├── plugin/                 # Plugin manager, gRPC protocol
├── config/                 # Viper-based config loading
├── state/                  # Release state persistence
└── ui/                     # Terminal UI components
pkg/plugin/                 # Public plugin interface (for plugin authors)
plugins/                    # Official plugins (github, slack, etc.)
```

## Core Commands

| Command | Purpose |
|---------|---------|
| `relicta init` | Set up config and default options |
| `relicta plan` | Analyze changes and assess risk since last release |
| `relicta bump` | Calculate and apply semver version |
| `relicta notes` | Generate AI-powered release notes |
| `relicta approve` | Governance gate with audit trail |
| `relicta publish` | Execute release: tag, changelog, notify, publish |
| `relicta release` | Complete workflow (plan → bump → notes → approve → publish) |

## Plugin System

- Plugins run as separate processes via HashiCorp go-plugin (gRPC)
- Hook-based lifecycle: `PreVersion`, `PostNotes`, `PostPublish`, etc.
- Official plugins: GitHub, GitLab, Slack, Jira

## Configuration

Config file: `release.config.yaml` (also supports JSON, searched in `.` and `~/.config/relicta/`)

Environment variables override config with `RELICTA_` prefix.

## Documentation

- **PRD:** `docs/prd.md` - Product requirements and feature specifications
- **Technical Design:** `docs/technical-design.md` - Architecture, interfaces, and implementation details
