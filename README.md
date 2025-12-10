# ReleasePilot

AI-powered release management CLI for software projects using conventional commits.

[![CI](https://github.com/felixgeelhaar/release-pilot/actions/workflows/ci.yaml/badge.svg)](https://github.com/felixgeelhaar/release-pilot/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/felixgeelhaar/release-pilot)](https://goreportcard.com/report/github.com/felixgeelhaar/release-pilot)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Features

- **Automatic Version Calculation**: Determines semantic version bumps from conventional commits
- **AI-Powered Release Notes**: Generates professional changelogs and release notes using OpenAI
- **Plugin System**: Extensible via gRPC plugins (GitHub releases, npm publish, Slack notifications)
- **Approval Workflow**: Review and approve releases before publishing
- **Interactive CLI**: Guided setup and approval process with beautiful terminal output
- **Dry Run Mode**: Preview changes without making any modifications

## Installation

### Using Go

```bash
go install github.com/felixgeelhaar/release-pilot/cmd/release-pilot@latest
```

### From Source

```bash
git clone https://github.com/felixgeelhaar/release-pilot.git
cd release-pilot
go build -o release-pilot ./cmd/release-pilot
```

### Docker

```bash
docker pull ghcr.io/felixgeelhaar/release-pilot:latest
```

## Quick Start

1. Initialize ReleasePilot in your project:

```bash
release-pilot init
```

2. Plan a release (analyzes commits since last tag):

```bash
release-pilot plan
```

3. Generate release notes (optionally with AI):

```bash
release-pilot notes --ai
```

4. Approve the release:

```bash
release-pilot approve
```

5. Publish the release:

```bash
release-pilot publish
```

## Configuration

Create a `release.config.yaml` in your project root:

```yaml
versioning:
  strategy: conventional
  tag_prefix: v
  git_tag: true
  git_push: true

changelog:
  file: CHANGELOG.md
  format: keep-a-changelog
  repository_url: https://github.com/your-org/your-repo

ai:
  enabled: true
  provider: openai
  model: gpt-4
  tone: professional
  audience: developers

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

## Commands

| Command | Description |
|---------|-------------|
| `init` | Initialize ReleasePilot with guided setup |
| `plan` | Analyze commits and plan the next release |
| `notes` | Generate changelog and release notes |
| `approve` | Review and approve the release |
| `publish` | Execute the release (create tag, run plugins) |

### Global Flags

| Flag | Description |
|------|-------------|
| `--config` | Path to config file (default: `release.config.yaml`) |
| `--dry-run` | Preview changes without making modifications |
| `--verbose` | Enable verbose output |
| `--json` | Output in JSON format |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENAI_API_KEY` | OpenAI API key for AI-powered features |
| `GITHUB_TOKEN` | GitHub token for creating releases |
| `SLACK_WEBHOOK_URL` | Slack webhook URL for notifications |

## Conventional Commits

ReleasePilot follows the [Conventional Commits](https://www.conventionalcommits.org/) specification:

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

ReleasePilot supports plugins for extending functionality:

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

## Architecture

ReleasePilot is built with Domain-Driven Design principles:

```
├── cmd/release-pilot/     # CLI entry point
├── internal/
│   ├── application/       # Use cases
│   ├── domain/           # Business logic
│   │   ├── changes/      # Commit analysis
│   │   ├── release/      # Release aggregate
│   │   └── version/      # Semantic versioning
│   ├── infrastructure/   # External adapters
│   ├── cli/              # Command implementations
│   └── service/          # Application services
├── pkg/plugin/           # Plugin interface
└── plugins/              # Official plugins
```

## Development

### Prerequisites

- Go 1.22+
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

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes using conventional commits
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) for details.
