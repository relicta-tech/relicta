<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="brand/relicta-logo-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="brand/relicta-logo-light.svg">
    <img alt="Relicta" src="brand/relicta-logo-light.svg" width="280">
  </picture>

  <p><strong>AI-powered release management for modern software teams</strong></p>

  <p>
    <a href="https://github.com/relicta-tech/relicta/actions/workflows/ci.yaml"><img src="https://github.com/relicta-tech/relicta/actions/workflows/ci.yaml/badge.svg" alt="CI"></a>
    <a href="https://goreportcard.com/report/github.com/relicta-tech/relicta"><img src="https://goreportcard.com/badge/github.com/relicta-tech/relicta" alt="Go Report Card"></a>
    <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  </p>
</div>

## Features

- **Automatic Version Calculation**: Determines semantic version bumps from conventional commits
- **AI-Powered Release Notes**: Generates professional changelogs and release notes (supports OpenAI, Anthropic, Google Gemini, Azure OpenAI, and Ollama)
- **Plugin System**: Extensible via gRPC plugins (GitHub releases, npm publish, Slack notifications)
- **Approval Workflow**: Review and approve releases before publishing
- **Interactive CLI**: Guided setup and approval process with beautiful terminal output
- **Dry Run Mode**: Preview changes without making any modifications

## Installation

### Homebrew (Recommended)

```bash
brew install relicta-tech/tap/relicta
```

### Download Binary

Download the latest release for your platform:

```bash
# macOS (Apple Silicon)
curl -L https://github.com/relicta-tech/relicta/releases/latest/download/relicta_Darwin_aarch64.tar.gz | tar xz
sudo mv relicta_Darwin_aarch64/relicta /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/relicta-tech/relicta/releases/latest/download/relicta_Darwin_x86_64.tar.gz | tar xz
sudo mv relicta_Darwin_x86_64/relicta /usr/local/bin/

# Linux (x86_64)
curl -L https://github.com/relicta-tech/relicta/releases/latest/download/relicta_Linux_x86_64.tar.gz | tar xz
sudo mv relicta_Linux_x86_64/relicta /usr/local/bin/

# Linux (ARM64)
curl -L https://github.com/relicta-tech/relicta/releases/latest/download/relicta_Linux_aarch64.tar.gz | tar xz
sudo mv relicta_Linux_aarch64/relicta /usr/local/bin/
```

### Using Go

```bash
go install github.com/relicta-tech/relicta/cmd/relicta@latest
```

### From Source

```bash
git clone https://github.com/relicta-tech/relicta.git
cd relicta
make build
sudo mv bin/relicta /usr/local/bin/
```

### GitHub Action (Recommended for CI/CD)

The easiest way to use Relicta in your CI/CD pipeline:

```yaml
- uses: relicta-tech/relicta-action@v2
  with:
    github-token: ${{ secrets.GITHUB_TOKEN }}
```

**No Go, Make, or build tools required!** The action automatically downloads the correct binary, verifies checksums, and handles the entire release workflow. See the [GitHub Action documentation](https://github.com/relicta-tech/relicta-action) for details.

## Quick Start

### One-Command Release (Recommended)

Run the complete release workflow in a single command:

```bash
# Interactive release (prompts for approval)
relicta release

# CI/CD auto-approve
relicta release --yes

# Preview changes without releasing
relicta release --dry-run
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

3. Generate release notes (optionally with AI):

```bash
relicta notes --ai
```

4. Approve the release:

```bash
relicta approve
```

5. Publish the release:

```bash
relicta publish
```

## Configuration

Create a `relicta.config.yaml` in your project root:

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
| `release` | **Run complete release workflow** (plan → bump → notes → approve → publish) |
| `init` | Initialize Relicta with guided setup |
| `plan` | Analyze commits and plan the next release |
| `bump` | Apply version bump (create tag) |
| `notes` | Generate changelog and release notes |
| `approve` | Review and approve the release |
| `publish` | Execute the release (create tag, run plugins) |

### Global Flags

| Flag | Description |
|------|-------------|
| `--config` | Path to config file (default: `relicta.config.yaml`) |
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

Relicta is built with Domain-Driven Design principles:

```
├── cmd/relicta/     # CLI entry point
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

### Pre-commit Hooks

Set up pre-commit hooks to catch issues before pushing (recommended):

```bash
make install-hooks
```

This installs a git hook that runs before each commit:
- **Format check** - Verifies code formatting
- **Vet** - Runs Go static analysis
- **Lint** - Runs golangci-lint
- **Test** - Runs unit tests

To manually run all checks:

```bash
make check
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
