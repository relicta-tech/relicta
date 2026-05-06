# Change Governance Protocol (CGP) Guide

This guide covers Relicta's Change Governance Protocol — the framework for risk-aware release decisions, approval workflows, and audit trails.

> **Looking for the formal specification?** See the [CGP Specification](cgp-specification.md) for the complete protocol definition.

## Overview

The Change Governance Protocol (CGP) answers the question: **Should this change ship?**

As AI agents and CI systems generate more code, deciding what reaches production requires:

- **Risk Assessment**: Quantified analysis of blast radius, API changes, and security impact
- **Policy Enforcement**: Customizable rules for when releases need approval
- **Audit Trails**: Immutable records of all governance decisions
- **Historical Learning**: Pattern detection from past releases

## Quick Start

### Enable Governance

Add to your `.relicta.yaml`:

```yaml
governance:
  enabled: true
  policy_dir: .relicta/policies
  memory_enabled: true
```

### Mnemos and Chronos Backends (Enabled by Default)

Mnemos (memory) and Chronos (pattern detection) are **enabled by default** in v4.0.1+.
Relicta works without external services running - operations gracefully degrade if
the backends are not available.

To opt-out, set `enabled: false` in your config:

```yaml
mnemos:
  enabled: false  # Disable Mnemos memory backend

chronos:
  enabled: false  # Disable Chronos pattern detection
```

To use the backends for enhanced features (historical queries, pattern detection),
run them as separate services:

```bash
# Install and start Mnemos (memory backend)
go install github.com/felixgeelhaar/mnemos/cmd/mnemos@latest
mnemos serve  # defaults to localhost:7777

# Install and start Chronos (pattern detection)
go install github.com/felixgeelhaar/chronos/cmd/chronos@latest
chronos serve  # defaults to localhost:7778
```

For full setup and run instructions, see [Cognitive Backends](cognitive-backends.md).

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

# Scaffold starter fixtures from existing policy rules
relicta policy scaffold --file .relicta/policies/security.policy --input-out policy-input.json --matrix-out policy-matrix.yaml

# Test policy behavior with simulated input
relicta policy test --risk-score 0.85 --bump-type major --actor-type agent

# Test multiple scenarios from one matrix file (JSON or YAML)
relicta policy test --matrix policy-matrix.yaml --json

# Test one scenario from input file (JSON or YAML)
relicta policy test --input policy-input.json --json

# Use in CI: fail if any scenario is blocked
relicta policy test --matrix policy-matrix.json --fail-on-blocked

# Strict CI gate: require all scenarios to be approved
relicta policy test --matrix policy-matrix.json --require-approved

# Contract test gate: assert each scenario's expected decision/block/block_reason/reviewers/required_approvers/required_actions/rationale/conditions/matched_rules
relicta policy test --matrix policy-matrix.json --assert-expected

# Include per-scenario assertion diffs in JSON output on failure
relicta policy test --matrix policy-matrix.json --assert-expected --json

# Include aggregate matrix summary (JSON: {"results":[...],"summary":{...}})
relicta policy test --matrix policy-matrix.json --summary --json

# Include per-rule and per-condition trace in output
relicta policy test --risk-score 0.85 --bump-type major --explain --json

# Trace verbosity: include only matched rules in trace output
relicta policy test --risk-score 0.85 --bump-type major --explain --explain-mode matched --json

# Run only selected scenarios from a matrix (repeat flag to select multiple)
relicta policy test --matrix policy-matrix.json --scenario high-risk-major --scenario medium-risk-minor --json

# Run scenario subsets by glob pattern
relicta policy test --matrix policy-matrix.json --scenario-pattern "high-*" --json

# Run scenarios by tag (repeat flag to include multiple tags)
relicta policy test --matrix policy-matrix.json --scenario-tag critical --scenario-tag smoke --json

# Exclude specific scenarios or patterns
relicta policy test --matrix policy-matrix.json --exclude-scenario flaky-case --exclude-scenario-pattern "experimental-*" --json

# Exclude scenarios by tag
relicta policy test --matrix policy-matrix.json --exclude-scenario-tag flaky --json

# Deterministic matrix sharding for CI parallel jobs
relicta policy test --matrix policy-matrix.json --shard-index 1 --shard-total 4 --json

# Export matrix results as JUnit XML for CI test reports
relicta policy test --matrix policy-matrix.json --assert-expected --junit-out policy-matrix.xml

# Write compact JSON summary artifact for CI step summaries
relicta policy test --matrix policy-matrix.json --assert-expected --summary-out policy-matrix-summary.json

# Compare baseline vs candidate policy sets on the same matrix
relicta policy test --matrix policy-matrix.json --baseline-file policy-current.policy --candidate-file policy-next.policy --json

# Compare output includes per-scenario `comparison` with changed fields (decision, blocked, required_approvers)

# Fail if candidate is stricter in any scenario
relicta policy test --matrix policy-matrix.json --baseline-file policy-current.policy --candidate-file policy-next.policy --compare-fail-on-stricter

# Fail if stricter/looser scenario counts exceed thresholds
relicta policy test --matrix policy-matrix.json --baseline-file policy-current.policy --candidate-file policy-next.policy --compare-max-stricter 3 --compare-max-looser 0

# List available matrix scenario names
relicta policy test --matrix policy-matrix.json --list-scenarios --json

# List selected scenarios for a CI shard
relicta policy test --matrix policy-matrix.json --scenario-tag critical --shard-index 1 --shard-total 4 --list-scenarios --json

# Read scenario matrix from stdin ("-" accepts JSON or YAML)
cat policy-matrix.yaml | relicta policy test --matrix - --json
```

### Scaffold Workflow

Use `relicta policy scaffold` to bootstrap contract tests quickly:

1. Generate fixtures from current rules:
```bash
relicta policy scaffold --dir .relicta/policies --input-out policy-input.json --matrix-out policy-matrix.yaml
```
2. Review generated scenarios (`low-risk-seed`, `high-risk-seed`, and `rule-*`).
3. Run matrix and inspect outcomes:
```bash
relicta policy test --matrix policy-matrix.yaml --json
```
4. Lock in behavior in CI:
```bash
relicta policy test --matrix policy-matrix.yaml --assert-expected
```
5. Re-run scaffold with `--force` when rules change, then reconcile updated expectations.

### Explainability Output Contract

When `--explain` is enabled, each decision output includes `rule_trace`:

- `rule_trace[].rule_id`: stable rule identifier from policy.
- `rule_trace[].rule_name`: human-readable rule name.
- `rule_trace[].priority`: evaluated priority (higher runs first).
- `rule_trace[].matched`: whether the rule matched.
- `rule_trace[].conditions[]`: per-condition trace details:
- `field`, `operator`, `expected`, `actual`, `matched`, and optional `missing_field` / `error`.

`--explain-mode all` keeps traces for every evaluated rule.  
`--explain-mode matched` keeps only matched rules in `rule_trace`.

Example JSON fragment:

```json
{
  "decision": "approval_required",
  "matched_rules": ["high-risk-major"],
  "rule_trace": [
    {
      "rule_id": "high-risk-major",
      "rule_name": "High risk major changes",
      "priority": 90,
      "matched": true,
      "conditions": [
        {"field": "risk.score", "operator": "gte", "expected": 0.8, "actual": 0.85, "matched": true},
        {"field": "change.bump_kind", "operator": "eq", "expected": "major", "actual": "major", "matched": true}
      ]
    }
  ]
}
```

CI guidance:

- Keep default runs lean (`--json` only) for fast checks.
- Enable `--explain --explain-mode matched` on failing shards to capture compact diagnostics.
- Persist explain output as an artifact for approvals/audits when policy behavior changes.

### CI Artifact Example (GitHub Actions)

```yaml
- name: Run policy matrix with artifacts
  run: |
    go run ./cmd/relicta policy test \
      --file examples/policies/starter.policy \
      --matrix examples/policies/policy-matrix.yaml \
      --assert-expected \
      --junit-out policy-matrix.xml \
      --summary-out policy-matrix-summary.json \
      --json

- name: Upload policy artifacts
  uses: actions/upload-artifact@v4
  with:
    name: policy-matrix-artifacts
    path: |
      policy-matrix.xml
      policy-matrix-summary.json
```

Use canonical dotted fields in policy rules (`risk.score`, `actor.kind`, `change.bump_kind`) for new files. Legacy aliases like `risk_score`, `actor_type`, and `bump_type` are still supported for compatibility.

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

### Risk Tuning

```yaml
governance:
  # Auto-approve very low risk changes
  auto_approve_threshold: 0.30
  # Never auto-approve above this score
  max_auto_approve_risk: 0.50
  # Always require humans for high-impact areas
  require_human_for_breaking: true
  require_human_for_security: true
  # Optional trust list for low-risk automation
  trusted_actors:
    - github-actions
    - ci-release-bot
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

  # Policy directory (contains .policy files)
  policy_dir: .relicta/policies

  # Memory/history
  memory_enabled: true
  memory_path: .relicta/governance/memory.json

  # Auto-approval
  auto_approve_threshold: 0.3
  max_auto_approve_risk: 0.5
  require_human_for_breaking: true
  require_human_for_security: true
  trusted_actors: []

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

5. **Review History Regularly**: Use `relicta history` to validate governance outcomes over time.

6. **Agent Boundaries**: Set clear policies for what AI agents can release autonomously.
