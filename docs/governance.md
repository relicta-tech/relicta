# Change Governance Protocol (CGP) Guide

This guide covers Relicta's Change Governance Protocol — the framework for risk-aware release decisions, approval workflows, and audit trails.

## Overview

The Change Governance Protocol (CGP) answers the question: **Should this change ship?**

As AI agents and CI systems generate more code, deciding what reaches production requires:

- **Risk Assessment**: Quantified analysis of blast radius, API changes, and security impact
- **Policy Enforcement**: Customizable rules for when releases need approval
- **Audit Trails**: Immutable records of all governance decisions
- **Historical Learning**: Pattern detection from past releases

## Quick Start

### Enable Governance

Add to your `relicta.config.yaml`:

```yaml
governance:
  enabled: true
  policy_paths:
    - .relicta/policies
  memory_enabled: true
```

### Create Your First Policy

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
```

### Evaluate a Release

```bash
# Plan and evaluate risk
relicta plan --analyze

# View the governance decision
relicta evaluate
```

## Policy DSL

Policies are written in Relicta's declarative DSL with `rule` blocks containing `when` conditions and `then` actions.

### Rule Structure

```
rule "rule-name" {
    priority = 100          # Higher = evaluated first (optional)
    description = "..."     # Human-readable description (optional)

    when {
        # Conditions (all must match)
    }

    then {
        # Actions to take
    }
}
```

### Available Conditions

| Condition | Type | Description |
|-----------|------|-------------|
| `risk_score` | float | Overall risk score (0.0 - 1.0) |
| `bump_type` | string | Version bump: "major", "minor", "patch" |
| `has_breaking_changes` | bool | True if breaking changes detected |
| `commit_count` | int | Number of commits in release |
| `files_changed` | int | Number of files modified |
| `lines_changed` | int | Lines added + removed |
| `scope` | string | Primary scope from commits |
| `actor_type` | string | "human", "agent", "ci" |
| `actor_id` | string | Actor identifier |
| `day_of_week` | int | 0=Sunday, 1=Monday, ... 6=Saturday |
| `hour` | int | Hour of day (0-23) |

### Comparison Operators

```
risk_score > 0.5        # Greater than
risk_score >= 0.5       # Greater than or equal
risk_score < 0.3        # Less than
risk_score <= 0.3       # Less than or equal
bump_type == "major"    # Equal
actor_type != "ci"      # Not equal
```

### Available Actions

| Action | Parameters | Description |
|--------|------------|-------------|
| `approve()` | - | Auto-approve the release |
| `block(reason: "...")` | reason | Block the release |
| `require_approval(role: "...")` | role | Require human approval |
| `add_reviewer(team: "...")` | team | Add team as reviewers |
| `warn(message: "...")` | message | Add a warning |
| `set_risk(score: 0.8)` | score | Override risk score |

### Example Policies

#### Block Risky Weekend Releases

```
rule "no-weekend-majors" {
    priority = 100
    description = "Block major releases on weekends"

    when {
        bump_type == "major"
        day_of_week >= 5  # Saturday or Sunday
    }

    then {
        block(reason: "Major releases not allowed on weekends")
    }
}
```

#### Require Security Review for High Risk

```
rule "security-review" {
    priority = 90
    description = "High-risk changes need security team review"

    when {
        risk_score > 0.7
    }

    then {
        require_approval(role: "security-team")
        warn(message: "High risk score detected")
    }
}
```

#### Auto-Approve Low-Risk Patches

```
rule "auto-approve-patches" {
    priority = 50
    description = "Auto-approve low-risk patches from trusted sources"

    when {
        bump_type == "patch"
        risk_score < 0.3
        actor_type != "agent"
    }

    then {
        approve()
    }
}
```

#### Agent Restrictions

```
rule "limit-agent-releases" {
    priority = 80
    description = "AI agents can only release minor versions"

    when {
        actor_type == "agent"
        bump_type == "major"
    }

    then {
        block(reason: "AI agents cannot release major versions autonomously")
    }
}
```

### Policy Defaults

Set default behavior at the top of your policy file:

```
defaults {
    decision = "require_approval"
    required_approvers = 1
}

rule "..." { ... }
```

### Validate Policies

```bash
# Validate a single file
relicta policy validate --file .relicta/policies/security.policy

# Validate all policies in directory
relicta policy validate --dir .relicta/policies

# List all loaded policies
relicta policy list
```

## Risk Scoring

Relicta calculates a risk score (0.0 - 1.0) based on multiple factors:

| Factor | Weight | Description |
|--------|--------|-------------|
| API Changes | 25% | Breaking changes, removed exports |
| Blast Radius | 20% | Files and lines changed |
| Dependency Impact | 15% | Downstream consumer impact |
| Security Impact | 15% | Security-sensitive changes |
| Historical Risk | 10% | Past issues with similar changes |
| Actor Trust | 10% | Track record of the releaser |
| Test Coverage | 5% | Coverage of changed code |

### Risk Severity Levels

| Score | Severity | Typical Action |
|-------|----------|----------------|
| 0.0 - 0.3 | Low | May auto-approve |
| 0.3 - 0.6 | Medium | Standard review |
| 0.6 - 0.8 | High | Extra scrutiny |
| 0.8 - 1.0 | Critical | Block or escalate |

### Customize Weights

```yaml
governance:
  risk:
    weights:
      api_changes: 0.30
      blast_radius: 0.20
      dependency_impact: 0.15
      security_impact: 0.15
      historical_risk: 0.10
      actor_trust: 0.05
      test_coverage: 0.05
```

## Release History

Track release outcomes and learn from historical patterns:

```bash
# View recent releases
relicta history

# View more entries
relicta history --limit 20

# Include risk information
relicta history releases --risk

# JSON output
relicta history --json
```

### Actor Metrics

View reliability metrics for specific actors:

```bash
# View metrics for a human
relicta history actor human:developer-name

# View metrics for an AI agent
relicta history actor agent:github-copilot

# View metrics for CI system
relicta history actor ci:github-actions
```

Output includes:
- **Reliability Score**: Overall success rate
- **Total Releases**: Number of releases by this actor
- **Success Rate**: Percentage of successful releases
- **Rollback Count**: Number of rollbacks
- **Average Risk Score**: Typical risk level
- **High Risk Releases**: Count of high-risk releases

### Risk Patterns

Analyze risk trends for a repository:

```bash
relicta history risk --repo owner/repo
```

Shows:
- Average risk score over time
- Risk trend (increasing/decreasing/stable)
- Common risk factors
- Incident correlations

## Webhooks

Receive notifications for release events via HTTP webhooks.

### Configuration

```yaml
webhooks:
  - name: slack-releases
    url: https://hooks.slack.com/services/...
    events:
      - release.published
      - release.failed
    secret: ${WEBHOOK_SECRET}

  - name: monitoring
    url: https://monitoring.example.com/hooks/release
    events:
      - release.*  # All release events
    headers:
      X-Custom-Header: value
    timeout: 30s
    retry_count: 3
    retry_delay: 5s
```

### Available Events

| Event | Description |
|-------|-------------|
| `release.initialized` | Release workflow started |
| `release.planned` | Version and changes analyzed |
| `release.versioned` | Version number assigned |
| `release.notes_generated` | Release notes created |
| `release.approved` | Release approved |
| `release.publishing_started` | Plugins executing |
| `release.published` | Release completed |
| `release.failed` | Release failed |
| `release.canceled` | Release canceled |
| `plugin.executed` | Plugin hook completed |
| `release.*` | All release events (wildcard) |

### Webhook Payload

```json
{
  "event": "release.published",
  "timestamp": "2024-01-15T12:00:00Z",
  "release_id": "rel-abc123",
  "data": {
    "version": "1.2.0",
    "tag_name": "v1.2.0",
    "release_url": "https://github.com/org/repo/releases/tag/v1.2.0"
  }
}
```

### Signature Verification

Webhooks are signed with HMAC-SHA256. Verify with the `X-Relicta-Signature` header:

```go
import "github.com/relicta-tech/relicta/internal/infrastructure/webhook"

valid := webhook.VerifySignature(
    requestBody,
    request.Header.Get("X-Relicta-Signature"),
    secretKey,
)
```

### Headers Sent

| Header | Description |
|--------|-------------|
| `Content-Type` | `application/json` |
| `User-Agent` | `Relicta-Webhook/1.0` |
| `X-Relicta-Event` | Event name |
| `X-Relicta-Delivery` | Release ID |
| `X-Relicta-Signature` | `sha256=...` (if secret configured) |

## Team-Based Approvals

Configure approval workflows based on teams:

```yaml
governance:
  approval:
    teams:
      release-managers:
        members:
          - alice
          - bob
        required_approvers: 1

      security-team:
        members:
          - carol
          - dave
        required_approvers: 2
```

Reference teams in policies:

```
rule "major-release-approval" {
    when {
        bump_type == "major"
    }

    then {
        require_approval(role: "release-managers")
        add_reviewer(team: "security-team")
    }
}
```

## MCP Integration

AI agents can interact with CGP via the Model Context Protocol:

```bash
# Start MCP server
relicta mcp serve
```

Available tools:
- `relicta.evaluate` - Evaluate release risk
- `relicta.approve` - Approve a pending release
- `relicta.publish` - Execute an approved release

Available resources:
- `relicta://risk-report` - Current risk assessment

See [MCP Integration Guide](mcp.md) for details.

## Audit Trail

All governance decisions are recorded with cryptographic integrity:

```bash
# Verify audit chain integrity
relicta audit verify

# Export audit log
relicta audit export --format json --since 2024-01-01
```

Each entry includes:
- Timestamp
- Actor (who)
- Action (what)
- Resource (on what)
- Outcome (result)
- Hash chain (tamper detection)

## Configuration Reference

Complete governance configuration:

```yaml
governance:
  enabled: true

  # Policy paths
  policy_paths:
    - .relicta/policies
    - .github/relicta/policies

  # Memory/history
  memory_enabled: true
  memory_path: .relicta/memory

  # Risk weights
  risk:
    weights:
      api_changes: 0.25
      blast_radius: 0.20
      dependency_impact: 0.15
      security_impact: 0.15
      historical_risk: 0.10
      actor_trust: 0.10
      test_coverage: 0.05

  # Auto-approval
  auto_approve_threshold: 0.3
  require_human_for_major: true

  # Audit
  audit:
    enabled: true
    retention_days: 365
    store_path: .relicta/audit

# Webhooks
webhooks:
  - name: notifications
    url: https://example.com/hooks/release
    events: ["release.*"]
```

## Best Practices

1. **Start Conservative**: Begin with `require_approval` as default, then add auto-approve rules for trusted patterns.

2. **Use Priorities**: Higher priority rules are evaluated first. Use priority to ensure critical rules (like blocking) run before permissive rules.

3. **Track Metrics**: Enable `memory_enabled` to build historical data for smarter risk assessment.

4. **Secure Webhooks**: Always use `secret` for webhook authentication.

5. **Regular Audits**: Periodically run `relicta audit verify` to ensure audit trail integrity.

6. **Agent Boundaries**: Set clear policies for what AI agents can release autonomously.
