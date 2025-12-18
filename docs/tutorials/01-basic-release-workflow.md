# Video Tutorial 1: Basic Release Workflow

**Duration:** 5-7 minutes
**Demo Repo:** https://github.com/relicta-tech/relicta-demo

## Overview

This tutorial introduces the core Relicta workflow: `plan → bump → notes → approve → publish`. Perfect for developers new to Relicta who want to automate their release process.

## Prerequisites

- Go project with git history
- Conventional commits (recommended)
- Relicta installed (`brew install relicta-tech/tap/relicta`)

## Script

### Scene 1: Introduction (30s)

```
"Welcome to Relicta - AI-powered release management for modern software teams.

In this tutorial, we'll walk through the basic release workflow that takes you
from code changes to a published release in just a few commands.

Let's get started with a demo project."
```

### Scene 2: Setup (1 min)

```bash
# Clone the demo repository
git clone https://github.com/relicta-tech/relicta-demo.git
cd relicta-demo

# Show the project structure
tree -L 2

# Initialize relicta (if needed)
relicta init
```

**Key Points:**
- Show the `relicta.config.yaml` file
- Explain versioning strategy (conventional commits)
- Point out the plugin configuration section

### Scene 3: The Plan Command (1 min)

```bash
# See what changes have occurred since last release
relicta plan
```

**Narration:**
```
"The plan command analyzes your git history since the last release.
It identifies:
- The type of version bump needed (major, minor, patch)
- All commits that will be included
- Any breaking changes that need attention"
```

**Show Output:**
- Version bump type (e.g., "minor" for feat commits)
- Commit summary
- Risk assessment (if CGP enabled)

### Scene 4: The Bump Command (45s)

```bash
# Apply the version bump
relicta bump
```

**Narration:**
```
"The bump command calculates and applies the semantic version bump.
It updates version files, creates the version tag, and prepares
for the changelog generation."
```

**Key Points:**
- Show the version change (e.g., v0.1.0 → v0.2.0)
- Explain how conventional commits drive the version

### Scene 5: The Notes Command (1 min)

```bash
# Generate release notes
relicta notes
```

**Narration:**
```
"The notes command generates a human-readable changelog from your commits.
It categorizes changes into features, fixes, and other types,
making it easy for users to understand what's new."
```

**Show Output:**
- Generated CHANGELOG.md content
- Feature groupings
- Mention AI enhancement option

### Scene 6: The Approve Command (45s)

```bash
# Review and approve the release
relicta approve
```

**Narration:**
```
"The approve command shows you a summary of the release for final review.
You can edit the notes, review the changes, and approve when ready.
This is your last chance to catch any issues before publishing."
```

**Key Points:**
- Interactive TUI for reviewing changes
- Ability to edit release notes
- `--yes` flag for CI/CD automation

### Scene 7: The Publish Command (1 min)

```bash
# Publish the release
relicta publish
```

**Narration:**
```
"The publish command executes the release:
- Creates the git tag
- Updates the changelog
- Triggers configured plugins (like GitHub releases)

And that's it! Your release is live."
```

**Show Output:**
- Tag creation
- GitHub release creation (if configured)
- Success message with release URL

### Scene 8: Summary (30s)

```bash
# The complete workflow in one view
relicta plan
relicta bump
relicta notes
relicta approve --yes
relicta publish
```

**Narration:**
```
"That's the basic Relicta workflow:
1. Plan - analyze changes
2. Bump - calculate version
3. Notes - generate changelog
4. Approve - review release
5. Publish - ship it!

In the next video, we'll set up the GitHub plugin for
automated GitHub releases."
```

## B-Roll Suggestions

- Terminal with commands running
- GitHub releases page showing the created release
- CHANGELOG.md file updates
- Version tags in git history

## End Card

- Link to documentation: docs.relicta.tech
- Next video: GitHub Plugin Setup
- GitHub: github.com/relicta-tech/relicta
