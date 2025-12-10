# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ReleasePilot is a CLI tool for streamlined software release management. It automates versioning, changelog generation, and release communication using AI and a plugin-based integration system. Built in Go for security, performance, and single-binary distribution.

## Build Commands

```bash
# Build
make build                    # Build binary to bin/release-pilot
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
cmd/release-pilot/          # Entry point
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
| `release-pilot init` | Set up config and default options |
| `release-pilot plan` | Analyze changes since last release |
| `release-pilot version` | Calculate and apply semver bump |
| `release-pilot notes` | Generate internal changelog and public notes |
| `release-pilot approve` | Review/edit notes for final approval |
| `release-pilot publish` | Execute release: tag, changelog, notify, publish |

## Plugin System

- Plugins run as separate processes via HashiCorp go-plugin (gRPC)
- Hook-based lifecycle: `PreVersion`, `PostNotes`, `PostPublish`, etc.
- Official plugins: GitHub, GitLab, Slack, Jira

## Configuration

Config file: `release.config.yaml` (also supports JSON, searched in `.` and `~/.config/release-pilot/`)

Environment variables override config with `RELEASE_PILOT_` prefix.

## Documentation

- **PRD:** `docs/prd.md` - Product requirements and feature specifications
- **Technical Design:** `docs/technical-design.md` - Architecture, interfaces, and implementation details
