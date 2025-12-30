# CGP Policy Examples

Example policies for the Change Governance Protocol (CGP). Copy these to `.relicta/policies/` to activate.

## Quick Start

```bash
# Create policies directory
mkdir -p .relicta/policies

# Copy a starter policy
cp examples/policies/starter.policy .relicta/policies/

# Verify policies load correctly
relicta plan --dry-run
```

## Available Policies

| Policy | Description | Use Case |
|--------|-------------|----------|
| `starter.policy` | Basic risk-based governance | Getting started, small teams |
| `agent-aware.policy` | AI agent oversight rules | Teams using AI coding assistants |
| `enterprise.policy` | Comprehensive governance | Regulated industries, critical systems |
| `time-based.policy` | Release windows and freezes | Production environments with SLAs |
| `team-based.policy` | Team ownership rules | Large organizations with domain teams |

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
- `actor.trusted` - Boolean, whether actor is trusted
- `actor.team` - Team name
- `actor.level` - "junior", "senior", "lead", etc.
- `actor.is_member` - Boolean, organization member

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
relicta policy validate .relicta/policies/

# Test with dry-run
relicta plan --dry-run

# View evaluation details
relicta plan --analyze
```
