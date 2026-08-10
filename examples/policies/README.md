# CGP Policy Examples

Example policies for the Change Governance Protocol (CGP).

Every policy here also ships **inside the relicta binary**, so you do not need this
repository to use one — `relicta policy init` writes it for you. The copies here are
for reading and diffing on the web; the embedded copies are what the command writes,
and a test asserts the two never diverge.

## Quick Start

```bash
# Write a starting policy into the first directory relicta searches
relicta policy init

# See the other starting points included in your binary
relicta policy init --list
relicta policy init --template enterprise

# Confirm it loaded, and see the rules it contributes
relicta policy list

# See the decision it produces for the changes you have
relicta evaluate
```

Governance is active without any policy file — it runs on built-in defaults. A policy
is how you add rules those defaults do not cover.

Editing a policy? `relicta policy validate` checks it, and `relicta policy test
--risk-score 0.6` shows what it decides before you rely on it.

## Available Policies

| Policy | Description | Use Case |
|--------|-------------|----------|
| `starter.policy` | Basic risk-based governance | Getting started, small teams |
| `agent-aware.policy` | AI agent oversight rules | Teams using AI coding assistants |
| `enterprise.policy` | Comprehensive governance | Regulated industries, critical systems |
| `time-based.policy` | Release windows and freezes | Production environments with SLAs |
| `team-based.policy` | Team ownership rules | Large organizations with domain teams |

## Test Fixtures

Use these with `relicta policy test` to validate policy behavior quickly:

| Fixture | Format | Purpose |
|--------|--------|---------|
| `policy-input.json` | JSON object | Single scenario input |
| `policy-matrix.json` | JSON array | Multi-scenario regression matrix |
| `policy-matrix.yaml` | YAML array | Multi-scenario matrix in YAML |

## Policy Syntax

CGP uses a simple, readable DSL:

```cgp
# Comments start with # or //

rule "rule-name" {
  priority = 100              # Higher = evaluated first
  description = "..."         # Human-readable description
  enabled = true              # Can disable rules

  when {
    # Conditions (all must match)
    risk.score > 0.5
    actor.kind == "agent"
    change.breaking == true
  }

  then {
    # Actions to take
    require_approval(count: 2)
    add_reviewer(team: "security")
    add_rationale(message: "...")
    block()
  }
}

defaults {
  decision = "approve"        # or "require_approval" or "block"
  required_approvers = 1
}
```

## Available Fields

### Risk
- `risk.score` - Risk score from 0.0 to 1.0

### Actor
- `actor.kind` - "human", "agent", "bot", "ci", "automation"
- `actor.trusted` - Boolean, true at trust level "trusted" or above
- `actor.trustLevel` - "untrusted", "limited", "trusted", "full"
- `actor.teams` / `actor.roles` - Lists from your team configuration (use with `contains`)
- `actor.reputation.overall` / `.level` / `.samples` / `.trend` - The actor's computed
  track record. Present only when governance computes reputation
  (`governance.memory_enabled` plus `reputation_enabled` or `earned_trust_enabled`);
  where it is absent a condition on it never matches.

`actor.team`, `actor.level` and `actor.is_member` are NOT provided — run
`relicta policy fields` for the list the evaluator actually resolves, and
`relicta policy validate` to check a policy against it.

### Change
- `change.breaking` - Boolean, breaking change detected
- `change.files` - Files changed (use with `contains`)
- `change.bump_kind` - "major", "minor", "patch"
- `change.scope_count` - Number of scopes touched

### Time (requires time context)
- `time.is_freeze` - Boolean, freeze period active
- `time.is_business_hours` - Boolean
- `time.is_weekend` - Boolean
- `time.is_holiday` - Boolean
- `time.day_of_week` - "monday", "tuesday", etc.
- `time.hour` - 0-23

### Canonical vs Legacy Field Names

Use canonical dotted fields in policies whenever possible:

- `risk.score` (canonical) over `risk_score` (legacy alias)
- `actor.kind` (canonical) over `actor_type` (legacy alias)
- `change.bump_kind` (canonical) over `bump_type` (legacy alias)
- `change.breaking` (canonical) over `has_breaking_changes` (legacy alias)

Legacy names remain supported for compatibility with older policy files.

## Operators

| Operator | Example | Description |
|----------|---------|-------------|
| `==` | `actor.kind == "agent"` | Equals |
| `!=` | `actor.kind != "bot"` | Not equals |
| `>` | `risk.score > 0.5` | Greater than |
| `<` | `risk.score < 0.3` | Less than |
| `>=` | `risk.score >= 0.5` | Greater or equal |
| `<=` | `risk.score <= 0.3` | Less or equal |
| `AND` | `a > 0.5 AND b == true` | Both conditions |
| `OR` | `a > 0.9 OR b == true` | Either condition |
| `NOT` | `NOT actor.trusted` | Negation |
| `in` | `actor.kind in ("a", "b")` | In list |
| `contains` | `change.files contains "api/"` | String contains |
| `matches` | `change.files matches "*.go"` | Pattern match |

## Actions

| Action | Parameters | Description |
|--------|------------|-------------|
| `require_approval` | `count: N` | Require N approvals |
| `add_reviewer` | `team: "name"` | Request review from team |
| `add_rationale` | `message: "..."` | Add explanation to decision |
| `block` | none | Block the release |

## Combining Policies

Multiple policies can be combined. Rules are evaluated in priority order (highest first). The most restrictive decision wins:

1. `block` - Release cannot proceed
2. `require_approval` - Needs human approval
3. `approve` - Auto-approve allowed

## Testing Policies

```bash
# Validate policy syntax
relicta policy validate --dir .relicta/policies

# Test with dry-run
relicta plan --dry-run

# Scaffold starter fixtures from policy rules
relicta policy scaffold --file .relicta/policies/starter.policy --input-out policy-input.json --matrix-out policy-matrix.yaml

# View evaluation details
relicta plan --analyze

# Evaluate a single fixture
relicta policy test --file .relicta/policies/starter.policy --input examples/policies/policy-input.json --json

# Evaluate a full matrix
relicta policy test --file .relicta/policies/starter.policy --matrix examples/policies/policy-matrix.json --json

# Include aggregate matrix summary (total/blocked/mismatched/decisions)
relicta policy test --file .relicta/policies/starter.policy --matrix examples/policies/policy-matrix.json --summary --json

# Show per-rule/per-condition evaluation trace
relicta policy test --file .relicta/policies/starter.policy --input examples/policies/policy-input.json --explain --json

# Only include matched rules in trace output
relicta policy test --file .relicta/policies/starter.policy --input examples/policies/policy-input.json --explain --explain-mode matched --json

# Contract-test matrix expectations (`expect.decision` / `expect.blocked` / `expect.block_reason` / `expect.reviewers` / `expect.required_approvers` / `expect.required_actions` / `expect.rationale` / `expect.conditions` / `expect.matched_rules`)
relicta policy test --file .relicta/policies/starter.policy --matrix examples/policies/policy-matrix.yaml --assert-expected

# Discover matrix scenario names for filtering/sharding
relicta policy test --matrix examples/policies/policy-matrix.yaml --list-scenarios --json

# Select scenarios by glob pattern
relicta policy test --matrix examples/policies/policy-matrix.yaml --scenario-pattern "high-*" --json

# Select scenarios by tag (if matrix scenarios define `tags`)
relicta policy test --matrix examples/policies/policy-matrix.yaml --scenario-tag critical --json

# Exclude selected scenarios
relicta policy test --matrix examples/policies/policy-matrix.yaml --exclude-scenario low-risk-patch --json

# Exclude scenarios by tag
relicta policy test --matrix examples/policies/policy-matrix.yaml --exclude-scenario-tag flaky --json

# Run one deterministic shard for CI parallelization
relicta policy test --matrix examples/policies/policy-matrix.yaml --shard-index 2 --shard-total 4 --json

# Write JUnit XML report for CI test dashboards
relicta policy test --file .relicta/policies/starter.policy --matrix examples/policies/policy-matrix.yaml --assert-expected --junit-out policy-matrix.xml

# Write compact JSON summary artifact for CI uploads or annotations
relicta policy test --file .relicta/policies/starter.policy --matrix examples/policies/policy-matrix.yaml --assert-expected --summary-out policy-matrix-summary.json

# Compare baseline and candidate policy files with one matrix run
relicta policy test --matrix examples/policies/policy-matrix.yaml --baseline-file examples/policies/starter.policy --candidate-file examples/policies/enterprise.policy --json

# Compare-mode JSON includes `baseline_output`, `candidate_output`, and `comparison` deltas

# Enforce compare regression thresholds in CI
relicta policy test --matrix examples/policies/policy-matrix.yaml --baseline-file examples/policies/starter.policy --candidate-file examples/policies/enterprise.policy --compare-max-stricter 2 --compare-max-looser 0

# Canonical dotted-field policy example (uses risk.score, actor.kind, change.bump_kind in rules)
relicta policy test --file examples/policies/agent-aware.policy --risk-score 0.92 --bump-type major --actor-type agent --json

# Fail CI if any matrix case is blocked
relicta policy test --file .relicta/policies/starter.policy --matrix examples/policies/policy-matrix.yaml --fail-on-blocked

# Stream matrix from stdin (JSON or YAML auto-detected)
cat examples/policies/policy-matrix.yaml | relicta policy test --file .relicta/policies/starter.policy --matrix -
```

## Scaffold Iteration Loop

Use scaffolding to keep policy contract tests in sync with rule changes:

1. Bootstrap fixtures:
```bash
relicta policy scaffold --file .relicta/policies/starter.policy --input-out policy-input.json --matrix-out policy-matrix.yaml
```
2. Evaluate and inspect:
```bash
relicta policy test --matrix policy-matrix.yaml --json
```
3. Gate expected behavior:
```bash
relicta policy test --matrix policy-matrix.yaml --assert-expected
```
4. After policy edits, regenerate with `--force` and update expectations intentionally.

## Explainability Output

Use `--explain` to include `rule_trace` in JSON output for each decision:

- `rule_trace[].rule_id`, `rule_name`, `priority`, `matched`
- `rule_trace[].conditions[]` with `field`, `operator`, `expected`, `actual`, `matched`
- Optional condition diagnostics: `missing_field`, `error`

`--explain-mode all` includes all evaluated rules.  
`--explain-mode matched` includes only matched rules.

For CI, a practical flow is:

1. Run matrix gating without trace for speed.
2. Re-run failed subset with `--explain --explain-mode matched --json`.
3. Upload the JSON output as an artifact for review.

## CI Integration Example

Use both artifact outputs in CI:

```bash
go run ./cmd/relicta policy test \
  --file examples/policies/starter.policy \
  --matrix examples/policies/policy-matrix.yaml \
  --assert-expected \
  --junit-out policy-matrix.xml \
  --summary-out policy-matrix-summary.json \
  --json
```
