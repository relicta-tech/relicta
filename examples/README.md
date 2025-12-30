# Relicta Configuration Examples

This directory contains example configurations for common use cases. Each example is a complete `.relicta.yaml` file that you can use as a starting point for your project.

## Quick Start

1. Choose an example that matches your use case
2. Copy it to your project root as `.relicta.yaml`
3. Customize the configuration for your project
4. Set required environment variables
5. Run `relicta init` to verify configuration

## Available Examples

### 1. GitHub Release (`github-release.yaml`)

**Use case:** Basic open-source project with GitHub releases

**Features:**
- Automatic semantic versioning from conventional commits
- Changelog generation in `CHANGELOG.md`
- GitHub release creation with build artifacts
- Git tag creation and push

**Required environment variables:**
- `GITHUB_TOKEN` - GitHub personal access token or Actions token

**Perfect for:**
- Open-source projects
- Simple release workflows
- Projects with binary distributions

---

### 2. NPM Package Publishing (`npm-publish.yaml`)

**Use case:** Publishing TypeScript/JavaScript packages to npm

**Features:**
- Automatic version bumping in `package.json`
- npm registry publishing
- GitHub releases
- AI-powered release notes
- Changelog generation

**Required environment variables:**
- `NPM_TOKEN` - npm authentication token
- `GITHUB_TOKEN` - GitHub authentication token
- `OPENAI_API_KEY` - OpenAI API key (for AI features)

**Perfect for:**
- Node.js libraries
- TypeScript packages
- React/Vue component libraries
- CLI tools published to npm

---

### 3. Team Notifications (`notifications.yaml`)

**Use case:** Notify team via Slack and Discord when releases happen

**Features:**
- GitHub releases
- Slack channel notifications
- Discord channel notifications
- AI-powered friendly release notes
- Error notifications to team

**Required environment variables:**
- `GITHUB_TOKEN` - GitHub authentication token
- `SLACK_WEBHOOK_URL` - Slack incoming webhook URL
- `DISCORD_WEBHOOK_URL` - Discord webhook URL
- `OPENAI_API_KEY` - OpenAI API key

**Perfect for:**
- Teams using Slack or Discord
- Projects needing release announcements
- Internal tool releases

**Setup instructions:**
- **Slack:** Go to your Slack workspace → Apps → Incoming Webhooks → Add New Webhook
- **Discord:** Server Settings → Integrations → Webhooks → New Webhook

---

### 4. Multi-Platform CLI (`multi-platform-cli.yaml`)

**Use case:** Go CLI tool with cross-platform binaries

**Features:**
- Support for multiple platforms (Linux, macOS, Windows)
- Multiple architectures (amd64, arm64)
- GitHub releases with all binary artifacts
- Checksum files for verification
- Slack notifications
- AI-powered release notes

**Required environment variables:**
- `GITHUB_TOKEN` - GitHub authentication token
- `SLACK_WEBHOOK_URL` - Slack webhook (optional)
- `OPENAI_API_KEY` - OpenAI API key

**Perfect for:**
- Go CLI tools
- Cross-platform desktop applications
- Tools distributed as binaries

**Expected artifacts in `dist/`:**
```
dist/
├── myapp_linux_amd64.tar.gz
├── myapp_linux_arm64.tar.gz
├── myapp_darwin_amd64.tar.gz
├── myapp_darwin_arm64.tar.gz
├── myapp_windows_amd64.zip
├── checksums.txt
└── checksums.sha256
```

**Works great with:** [GoReleaser](https://goreleaser.com/)

---

### 5. Jira Integration (`jira-integration.yaml`)

**Use case:** Enterprise team using Jira for issue tracking

**Features:**
- Automatic Jira ticket detection in commits
- Create version in Jira for release
- Transition tickets to "Released" status
- Add fix version to tickets
- GitHub releases
- Slack notifications to product team
- AI-powered release notes

**Required environment variables:**
- `GITHUB_TOKEN` - GitHub authentication token
- `JIRA_EMAIL` - Your Jira account email
- `JIRA_API_TOKEN` - Jira API token
- `SLACK_WEBHOOK_URL` - Slack webhook (optional)
- `OPENAI_API_KEY` - OpenAI API key

**Perfect for:**
- Enterprise development teams
- Projects using Jira for tracking
- Teams needing audit trails

**Commit message format:**
```bash
feat(PROJ-123): add user authentication
fix(PROJ-456): resolve login timeout
docs(PROJ-789): update API documentation
```

**Setup instructions:**
1. Generate Jira API token: [Atlassian API Tokens](https://id.atlassian.com/manage-profile/security/api-tokens)
2. Find your project key in Jira (e.g., "PROJ", "API", "WEB")
3. Configure transition status name (e.g., "Released", "Done")

---

### 6. Monorepo (`monorepo.yaml`)

**Use case:** Monorepo with multiple packages released independently

**Features:**
- Package-scoped versioning and tags
- Independent changelog per package
- NPM publishing for each package
- Package-scoped GitHub releases
- Path-based commit filtering
- Slack notifications

**Required environment variables:**
- `GITHUB_TOKEN` - GitHub authentication token
- `NPM_TOKEN` - npm authentication token
- `SLACK_WEBHOOK_URL` - Slack webhook (optional)
- `OPENAI_API_KEY` - OpenAI API key

**Perfect for:**
- Monorepo projects (Turborepo, Nx, Lerna)
- Multiple npm packages in one repo
- Independently versioned components

**Directory structure:**
```
monorepo/
├── packages/
│   ├── api/
│   │   ├── .relicta.yaml
│   │   ├── package.json
│   │   └── CHANGELOG.md
│   └── web/
│       ├── .relicta.yaml
│       ├── package.json
│       └── CHANGELOG.md
└── README.md
```

**Usage:**
```bash
# Release the API package
cd packages/api
relicta plan
relicta publish

# Release the web package
cd packages/web
relicta plan
relicta publish
```

**Tag format:** `@scope/package@version` (e.g., `@myorg/api@1.2.0`)

---

## General Setup Guide

### 1. Choose Your Example

Select the example that best matches your project type and workflow.

### 2. Copy Configuration

```bash
# Copy example to your project
cp examples/github-release.yaml .relicta.yaml

# Or use init command and customize
relicta init
```

### 3. Customize Configuration

Edit `.relicta.yaml` and update:
- `repository_url` - Your GitHub repository URL
- `plugins` - Enable/disable plugins based on your needs
- `workflow.allowed_branches` - Branches allowed for releases
- Plugin-specific configurations

### 4. Set Environment Variables

```bash
# GitHub (required for most examples)
export GITHUB_TOKEN="ghp_your_token_here"

# npm (for npm publishing)
export NPM_TOKEN="npm_your_token_here"

# OpenAI (for AI features)
export OPENAI_API_KEY="sk-your_key_here"

# Slack (for notifications)
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/..."

# Discord (for notifications)
export DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/..."

# Jira (for Jira integration)
export JIRA_EMAIL="your.email@company.com"
export JIRA_API_TOKEN="your_jira_api_token"
```

### 5. Test Configuration

```bash
# Validate configuration
relicta validate

# Test with dry-run
relicta plan --dry-run
relicta publish --dry-run
```

### 6. Create First Release

```bash
# Plan the release
relicta plan

# Generate release notes
relicta notes

# Review and approve
relicta approve

# Publish the release
relicta publish
```

---

## Configuration Options Reference

### Versioning Strategies

- `conventional` - Semantic versioning from conventional commits (recommended)
- `manual` - Manually specify version

### Changelog Formats

- `conventional` - Standard conventional commits format
- `keep-a-changelog` - Keep a Changelog format

### AI Providers

- `openai` - OpenAI GPT models
- `anthropic` - Claude models
- `ollama` - Local Ollama models

### AI Tones

- `professional` - Formal, technical tone
- `friendly` - Casual, approachable tone
- `concise` - Brief, to-the-point
- `detailed` - Comprehensive explanations

### AI Audiences

- `developers` - Technical audience
- `team` - Internal team members
- `customers` - End users and customers
- `product-team` - Product managers and stakeholders

---

## Plugin Configuration

### Available Plugins

| Plugin | Purpose | Required Env Vars |
|--------|---------|-------------------|
| `github` | Create GitHub releases | `GITHUB_TOKEN` |
| `gitlab` | Create GitLab releases | `GITLAB_TOKEN` |
| `npm` | Publish to npm registry | `NPM_TOKEN` |
| `slack` | Slack notifications | `SLACK_WEBHOOK_URL` |
| `discord` | Discord notifications | `DISCORD_WEBHOOK_URL` |
| `jira` | Update Jira tickets | `JIRA_EMAIL`, `JIRA_API_TOKEN` |
| `launchnotes` | Sync to LaunchNotes | `LAUNCHNOTES_API_KEY` |

See [PLUGINS.md](../docs/PLUGINS.md) for detailed plugin documentation.

---

## Advanced Scenarios

### GitHub Actions

Use the Relicta GitHub Action for zero-setup CI/CD:

```yaml
name: Release

on:
  push:
    branches:
      - main

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: relicta-tech/relicta-action@v1
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
```

See [relicta-action](https://github.com/relicta-tech/relicta-action) for details.

### Private npm Registry

```yaml
plugins:
  - name: npm
    config:
      registry: https://npm.yourcompany.com
      access: restricted
```

### Pre-release Versions

```yaml
versioning:
  strategy: conventional
  prerelease: true
  prerelease_prefix: beta  # Creates versions like 1.0.0-beta.1

plugins:
  - name: github
    config:
      prerelease: true
```

### Custom Commit Types

```yaml
versioning:
  strategy: conventional
  commit_types:
    enhancement: minor
    security: patch
    breaking: major
```

---

## Troubleshooting

See [TROUBLESHOOTING.md](../docs/TROUBLESHOOTING.md) for common issues and solutions.

### Common Issues

**"No commits found since last release"**
```bash
# Check if there are commits
git log $(git describe --tags --abbrev=0)..HEAD

# Make a commit if needed
git commit -m "feat: add new feature"
```

**"Plugin not found"**
```bash
# Download plugin binary
curl -L https://github.com/relicta-tech/relicta/releases/latest/download/github_linux_x86_64 \
  -o ~/.relicta/plugins/relicta-plugin-github
chmod +x ~/.relicta/plugins/relicta-plugin-github
```

**"Invalid config"**
```bash
# Validate configuration
relicta validate

# Check YAML syntax
yamllint .relicta.yaml
```

---

## Additional Resources

- [Main README](../README.md) - Project overview and installation
- [Plugin Guide](../docs/PLUGINS.md) - Detailed plugin documentation
- [Troubleshooting](../docs/TROUBLESHOOTING.md) - Common issues and solutions
- [GitHub Action](https://github.com/relicta-tech/relicta-action) - CI/CD integration
- [Conventional Commits](https://www.conventionalcommits.org/) - Commit message format

---

## Contributing

Have a configuration example that would benefit others? Please contribute!

1. Create a new example file following the naming convention
2. Include clear comments and documentation
3. Test the configuration in a real project
4. Add it to this README with a description
5. Submit a pull request

---

## License

MIT License - see [LICENSE](../LICENSE) for details.
