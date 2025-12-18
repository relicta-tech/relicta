# Video Tutorial 3: Multi-Platform Publishing

**Duration:** 6-8 minutes
**Demo Repo:** https://github.com/relicta-tech/relicta-demo

## Overview

Publish your releases to multiple platforms simultaneously: GitHub, npm, Docker Hub, PyPI, and more. This tutorial shows how to configure multiple plugins for a unified release workflow.

## Prerequisites

- Relicta installed with GitHub plugin
- Accounts on target platforms (npm, Docker Hub, etc.)
- API tokens/credentials for each platform

## Script

### Scene 1: Introduction (30s)

```
"Modern software often needs to be published to multiple platforms:
binaries on GitHub, packages on npm, images on Docker Hub.

Relicta's plugin system lets you publish to all of them
with a single workflow. Let's set it up."
```

### Scene 2: Available Plugins Overview (1 min)

```bash
# List all available plugins
relicta plugin list --available

# Categories of plugins:
# - Version Control: github, gitlab
# - Notifications: slack, discord, teams
# - Package Managers: npm, pypi, homebrew, chocolatey
# - Containers: docker
# - Project Management: jira, launchnotes
```

**Show the 20 available plugins organized by category**

### Scene 3: Installing Multiple Plugins (1 min)

```bash
# Install the plugins you need
relicta plugin install npm
relicta plugin install docker
relicta plugin install slack

# Enable them
relicta plugin enable npm
relicta plugin enable docker
relicta plugin enable slack

# Verify
relicta plugin list
```

**Narration:**
```
"Each plugin runs independently, allowing you to publish
to multiple destinations in a single release workflow."
```

### Scene 4: Configuring npm Plugin (1 min)

```yaml
plugins:
  - name: npm
    enabled: true
    config:
      # Package configuration
      package_path: "."           # Directory with package.json
      tag: "latest"               # npm dist-tag
      access: "public"            # public or restricted

      # Registry (default: npmjs.com)
      # registry: "https://npm.pkg.github.com"

      # Authentication via NPM_TOKEN env var
```

```bash
# Set npm token
export NPM_TOKEN="npm_xxxxxxxxxxxxxx"
```

### Scene 5: Configuring Docker Plugin (1.5 min)

```yaml
plugins:
  - name: docker
    enabled: true
    config:
      # Image configuration
      image: "myorg/myapp"
      registry: "docker.io"       # or ghcr.io, etc.

      # Build settings
      dockerfile: "Dockerfile"
      context: "."
      platforms:
        - "linux/amd64"
        - "linux/arm64"

      # Tags (${VERSION} is replaced automatically)
      tags:
        - "latest"
        - "${VERSION}"
        - "${VERSION_MAJOR}"

      # Push on publish
      push: true
```

```bash
# Docker Hub authentication
docker login
# or
export DOCKER_USERNAME="myuser"
export DOCKER_PASSWORD="mytoken"
```

### Scene 6: Configuring Slack Notifications (1 min)

```yaml
plugins:
  - name: slack
    enabled: true
    config:
      webhook_url: "${SLACK_WEBHOOK_URL}"
      channel: "#releases"

      # Message customization
      username: "Relicta Bot"
      icon_emoji: ":rocket:"

      # Include in notification
      include_changelog: true
      include_version: true
```

**Narration:**
```
"Notification plugins let your team know when releases happen.
Slack, Discord, and Teams are all supported."
```

### Scene 7: Running Multi-Platform Release (1.5 min)

```bash
# Full release workflow
relicta plan
relicta bump
relicta notes
relicta approve --yes
relicta publish
```

**Show the output:**
```
Publishing release v1.2.0...
  ✓ github: Release created at github.com/org/repo/releases/v1.2.0
  ✓ npm: Published to npm as @org/package@1.2.0
  ✓ docker: Pushed myorg/myapp:1.2.0 to docker.io
  ✓ slack: Notification sent to #releases

Release complete!
```

### Scene 8: Plugin Lifecycle Hooks (30s)

```
"Plugins can hook into different stages of the release:

- PreVersion: Before version calculation
- PostNotes: After changelog generation
- PostPublish: After publishing
- OnSuccess/OnError: Based on outcome

This allows plugins to run at exactly the right time."
```

### Scene 9: Summary (30s)

```
"With Relicta's plugin system, you can:
- Publish to 20+ platforms simultaneously
- Configure each destination independently
- Get notified across multiple channels

One workflow, many destinations.

In the next video, we'll explore AI-powered release notes
using OpenAI or Anthropic."
```

## Complete Configuration Example

```yaml
# relicta.config.yaml - Full multi-platform setup
versioning:
  strategy: conventional

plugins:
  - name: github
    enabled: true
    config:
      draft: false
      assets:
        - "dist/*.tar.gz"
        - "dist/*.zip"

  - name: npm
    enabled: true
    config:
      access: public

  - name: docker
    enabled: true
    config:
      image: "myorg/myapp"
      platforms:
        - "linux/amd64"
        - "linux/arm64"
      tags:
        - "latest"
        - "${VERSION}"

  - name: slack
    enabled: true
    config:
      webhook_url: "${SLACK_WEBHOOK_URL}"
      channel: "#releases"
```

## End Card

- Plugin catalog: docs.relicta.tech/plugins
- Next video: AI-Powered Release Notes
