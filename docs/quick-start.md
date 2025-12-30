# Quick Start Guide

Get from zero to your first release in under 5 minutes.

## Prerequisites

- Git repository with at least one commit
- [Conventional Commits](https://www.conventionalcommits.org/) (recommended) or any commit history

## Installation

Choose your preferred method:

```bash
# Homebrew (macOS/Linux)
brew install relicta-tech/tap/relicta

# Go install
go install github.com/relicta-tech/relicta/cmd/relicta@latest

# Direct download (see README for all platforms)
curl -L https://github.com/relicta-tech/relicta/releases/latest/download/relicta_$(uname -s)_$(uname -m).tar.gz | tar xz
sudo mv relicta_*/relicta /usr/local/bin/
```

Verify installation:

```bash
relicta version
```

## Your First Release

### Option 1: One Command (Fastest)

Run the complete workflow with a single command:

```bash
cd your-project

# Preview what will happen
relicta release --dry-run

# Execute the release
relicta release
```

This runs the complete workflow: analyze → version → notes → approve → publish.

### Option 2: Step by Step (Learn the Flow)

For more control and understanding, run each step:

```bash
# 1. Initialize Relicta (creates config file)
relicta init

# 2. Analyze commits and plan the release
relicta plan

# 3. Calculate and apply the version
relicta bump

# 4. Generate release notes
relicta notes

# 5. Review and approve
relicta approve

# 6. Publish the release
relicta publish
```

## Understanding the Workflow

Relicta uses a state machine to track releases through their lifecycle:

```
Draft → Planned → Versioned → NotesReady → Approved → Published
                                    ↓
                              (or) Canceled/Failed
```

Each command transitions the release to the next state:

| Command | Transition | What Happens |
|---------|------------|--------------|
| `plan` | Draft → Planned | Analyzes commits, calculates risk score |
| `bump` | Planned → Versioned | Determines version bump, creates git tag |
| `notes` | Versioned → NotesReady | Generates changelog and release notes |
| `approve` | NotesReady → Approved | Governance check, human approval |
| `publish` | Approved → Published | Executes plugins (GitHub, npm, etc.) |

## Configuration

The `relicta init` wizard creates a `relicta.config.yaml`:

```yaml
# Minimal config - Relicta works with sensible defaults
versioning:
  strategy: conventional  # or: smart (for non-conventional commits)
  tag_prefix: v
  git_tag: true
  git_push: true

changelog:
  file: CHANGELOG.md
  format: keep-a-changelog
```

### Enable AI-Powered Release Notes

```yaml
ai:
  enabled: true
  provider: openai  # openai, anthropic, gemini, azure, ollama
  model: gpt-4
  tone: professional
  audience: developers
```

Set your API key:

```bash
export OPENAI_API_KEY="your-key"
```

Then generate notes:

```bash
relicta notes --ai
```

### Enable the GitHub Plugin

```yaml
plugins:
  - name: github
    enabled: true
    config:
      draft: false
```

Set your token:

```bash
export GITHUB_TOKEN="your-token"
```

## Governance (CGP)

The Change Governance Protocol helps you decide **what should ship** by assessing risk and enforcing policies.

### Enable Governance

Add to your config:

```yaml
governance:
  enabled: true
  policy_paths:
    - .relicta/policies
```

### Create a Policy

Create `.relicta/policies/default.policy`:

```
rule "require-approval-for-major" {
    description = "Major versions need human approval"

    when {
        bump_type == "major"
    }

    then {
        require_approval(role: "release-manager")
    }
}

rule "auto-approve-patches" {
    description = "Auto-approve low-risk patches"

    when {
        bump_type == "patch"
        risk_score < 0.3
    }

    then {
        approve()
    }
}
```

### View Risk Assessment

```bash
relicta plan --analyze
```

Output shows:
- Risk score (0.0 - 1.0)
- Contributing factors
- Recommended action

See the [CGP Guide](governance.md) for full policy DSL documentation.

## Common Patterns

### CI/CD Auto-Release

For automated releases in CI:

```bash
relicta release --yes
```

The `--yes` flag auto-approves releases (subject to governance policies).

### Tag-Triggered Releases

If you push tags manually and want Relicta to handle the rest:

```bash
git tag v1.2.0
git push origin v1.2.0

# Relicta detects the tag and uses it
relicta release --yes
```

### Preview Without Changes

Always available with `--dry-run`:

```bash
relicta release --dry-run
relicta bump --dry-run
relicta notes --dry-run
```

### Skip Tag Push

For local testing or when CI handles pushes:

```bash
relicta publish --skip-push
```

### Clean Up Stale Releases

If you have old release runs that weren't completed:

```bash
# See what would be deleted
relicta clean --dry-run

# Keep last 5 releases, delete older
relicta clean --keep 5

# Delete releases older than 30 days
relicta clean --older-than 30d
```

## GitHub Actions

The simplest CI/CD integration:

```yaml
name: Release

on:
  push:
    branches: [main]

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: relicta-tech/relicta-action@v2
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
```

For AI-powered notes, add your API key:

```yaml
      - uses: relicta-tech/relicta-action@v2
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
```

## MCP Integration (AI Agents)

Let AI agents like Claude manage releases:

```bash
# Start the MCP server
relicta mcp serve
```

Add to Claude Desktop's config:

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

Now Claude can:
- Plan and analyze releases
- Generate release notes
- Approve and publish (with governance guardrails)

See [MCP Integration Guide](mcp.md) for details.

## Troubleshooting

### "No commits found since last release"

Your repository needs at least one commit after the last tag:

```bash
git log --oneline $(git describe --tags --abbrev=0)..HEAD
```

### "No previous tag found"

For first releases, Relicta uses `0.0.0` as the base:

```bash
relicta plan  # Will suggest 0.1.0 or 1.0.0
```

### Reset a Failed Release

If a release gets stuck:

```bash
relicta cancel  # Cancel current release
relicta clean   # Clean up stale runs
relicta release # Start fresh
```

### View Current State

```bash
relicta status
```

Shows:
- Current release state
- Pending actions
- Version information

## Next Steps

- **[Configuration Reference](README.md#configuration)** - Full config options
- **[CGP Guide](governance.md)** - Policy DSL, risk scoring, approvals
- **[Plugin System](PLUGINS.md)** - GitHub, npm, Slack, and more
- **[MCP Integration](mcp.md)** - AI agent support
- **[Troubleshooting](TROUBLESHOOTING.md)** - Common issues and solutions

## Command Reference

| Command | Description |
|---------|-------------|
| `relicta release` | Complete workflow in one command |
| `relicta init` | Initialize with guided setup |
| `relicta plan` | Analyze commits and plan release |
| `relicta bump` | Apply version bump |
| `relicta notes` | Generate release notes |
| `relicta approve` | Review and approve |
| `relicta publish` | Execute the release |
| `relicta status` | View current state |
| `relicta cancel` | Cancel active release |
| `relicta clean` | Remove stale releases |
| `relicta mcp serve` | Start MCP server |
