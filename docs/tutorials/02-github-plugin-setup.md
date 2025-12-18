# Video Tutorial 2: GitHub Plugin Setup

**Duration:** 4-6 minutes
**Demo Repo:** https://github.com/relicta-tech/relicta-demo

## Overview

This tutorial shows how to install and configure the GitHub plugin to automatically create GitHub releases with binaries and release notes.

## Prerequisites

- Relicta installed
- GitHub repository
- GitHub personal access token (for local) or `GITHUB_TOKEN` (for CI)

## Script

### Scene 1: Introduction (30s)

```
"Relicta plugins extend your release workflow with powerful integrations.

In this tutorial, we'll set up the GitHub plugin to automatically
create GitHub releases, upload binaries, and generate release notes
directly on your repository."
```

### Scene 2: Installing the Plugin (1 min)

```bash
# List available plugins
relicta plugin list --available

# Install the GitHub plugin
relicta plugin install github

# Enable it
relicta plugin enable github

# Verify installation
relicta plugin list
```

**Narration:**
```
"Installing plugins with Relicta is straightforward.
The plugin is downloaded from the official registry,
checksums are verified for security, and it's ready to use."
```

### Scene 3: Configuration (1.5 min)

```yaml
# relicta.config.yaml
plugins:
  - name: github
    enabled: true
    config:
      # Owner and repo (auto-detected from git remote)
      # owner: your-org
      # repository: your-repo

      # Release settings
      draft: false           # Create as draft first
      prerelease: false      # Mark as prerelease
      generate_notes: true   # Use GitHub's auto-notes

      # Assets to upload
      assets:
        - "dist/*.tar.gz"
        - "dist/*.zip"
        - "dist/checksums.txt"
```

**Key Points:**
- Explain auto-detection of owner/repo
- Show draft vs published releases
- Demonstrate asset glob patterns

### Scene 4: Authentication (1 min)

```bash
# Option 1: Environment variable
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"

# Option 2: GitHub CLI (recommended for local dev)
gh auth status

# Option 3: .env file
echo "GITHUB_TOKEN=ghp_xxxx" >> .env
```

**Narration:**
```
"For authentication, you have several options:
- Use a personal access token in an environment variable
- Leverage the GitHub CLI's existing authentication
- Store it in a .env file for local development

In CI/CD, the workflow's GITHUB_TOKEN is automatically available."
```

### Scene 5: Running a Release (1.5 min)

```bash
# Build your release assets
make release

# Run the full workflow
relicta plan
relicta bump
relicta notes
relicta approve --yes
relicta publish
```

**Show:**
- Plugin loading message
- GitHub release being created
- Assets being uploaded
- Final release URL

**Narration:**
```
"When you run publish, the GitHub plugin:
1. Creates a new release on your repository
2. Uploads all matching assets
3. Sets the release notes from your changelog
4. Makes it available for download

Check your GitHub releases page to see the result!"
```

### Scene 6: CI/CD Integration (30s)

```yaml
# .github/workflows/release.yaml
- name: Publish Release
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  run: |
    relicta plan
    relicta bump
    relicta notes
    relicta approve --yes
    relicta publish
```

**Narration:**
```
"In GitHub Actions, the GITHUB_TOKEN is automatically available
with the permissions you've configured. No additional secrets needed!"
```

### Scene 7: Summary (30s)

```
"You've now set up the GitHub plugin to:
- Automatically create GitHub releases
- Upload binary assets
- Generate release notes

In the next video, we'll explore multi-platform publishing
to npm, Docker Hub, and more."
```

## Common Issues

1. **401 Unauthorized**: Check token permissions (needs `contents: write`)
2. **Asset not found**: Verify glob patterns match your build output
3. **Rate limiting**: Use authenticated requests to increase limits

## End Card

- Plugin docs: docs.relicta.tech/plugins/github
- Next video: Multi-Platform Publishing
