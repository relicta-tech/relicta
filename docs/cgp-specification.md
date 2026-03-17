# Change Governance Protocol (CGP) Specification

**Version:** 0.1.0
**Status:** Draft
**Authors:** Relicta Contributors
**License:** MIT

## Abstract

The Change Governance Protocol (CGP) is an open protocol for governing software changes in automated and AI-driven development environments. It defines a structured approach for proposing, evaluating, approving, and executing releases while maintaining security, auditability, and human oversight.

As AI agents and CI/CD systems increasingly generate code autonomously, deciding *what should ship* becomes critical. CGP provides the governance layer that ensures responsible change management.

## Table of Contents

1. [Introduction](#1-introduction)
2. [Terminology](#2-terminology)
3. [Protocol Overview](#3-protocol-overview)
4. [Message Types](#4-message-types)
5. [Actor Model](#5-actor-model)
6. [Risk Assessment](#6-risk-assessment)
7. [Policy Engine](#7-policy-engine)
8. [Approval Workflows](#8-approval-workflows)
9. [Audit Trail](#9-audit-trail)
10. [Transport Bindings](#10-transport-bindings)
11. [Security Considerations](#11-security-considerations)
12. [Implementation Guidelines](#12-implementation-guidelines)

---

## 1. Introduction

### 1.1 Motivation

Modern software development increasingly involves autonomous agents—AI coding assistants, CI/CD systems, and automated bots. These systems can propose changes faster than humans can review them. Without governance:

- Risky changes may reach production unreviewed
- Audit trails become incomplete or inconsistent
- Organizational policies are enforced inconsistently
- Human oversight becomes a bottleneck rather than a checkpoint

CGP addresses these challenges by defining:

- **A structured proposal format** for any actor to request a release
- **Risk assessment criteria** for evaluating change impact
- **Policy enforcement** through declarative rules
- **Audit requirements** for compliance and learning
- **Human-in-the-loop workflows** where appropriate

### 1.2 Design Principles

1. **Actor-Agnostic**: Works with humans, AI agents, CI systems, or any combination
2. **Transport-Independent**: Protocol messages are independent of transport mechanism
3. **Vendor-Neutral**: No specific tooling required; any system can implement CGP
4. **Incrementally Adoptable**: Start simple, add sophistication as needed
5. **Audit-First**: All decisions are traceable and verifiable

### 1.3 Scope

CGP governs the decision to release, not the release mechanics. It answers:

- Should this change be released?
- What risk does it carry?
- Who needs to approve it?
- What conditions must be met?

CGP does **not** define:

- How code is deployed
- How artifacts are built
- How tests are executed
- How infrastructure is managed

---

## 2. Terminology

| Term | Definition |
|------|------------|
| **Actor** | An entity that can propose or approve changes (human, agent, CI system) |
| **Blast Radius** | The potential scope of impact if a change causes issues |
| **Executor** | A system that carries out approved release actions |
| **Governor** | A system implementing CGP that evaluates proposals and renders decisions |
| **Policy** | Declarative rules that influence governance decisions |
| **Proposal** | A structured request to release changes |
| **Risk Score** | A normalized measure (0.0-1.0) of change risk |
| **Trust Level** | The degree of autonomy granted to an actor |

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119).

---

## 3. Protocol Overview

### 3.1 Message Flow

```
┌─────────────┐           ┌─────────────┐           ┌─────────────┐
│   Proposer  │           │   Governor  │           │   Executor  │
│  (Agent/CI) │           │   (CGP)     │           │  (Plugins)  │
└──────┬──────┘           └──────┬──────┘           └──────┬──────┘
       │                         │                         │
       │  1. change.proposal     │                         │
       │────────────────────────>│                         │
       │                         │                         │
       │                         │ [Analyze & Evaluate]    │
       │                         │                         │
       │  2. change.decision     │                         │
       │<────────────────────────│                         │
       │                         │                         │
       │        [If approval_required, wait for human]     │
       │                         │                         │
       │  3. execution_authorized│                         │
       │<────────────────────────│                         │
       │                         │                         │
       │                         │  4. Execute Release     │
       │                         │────────────────────────>│
       │                         │                         │
```

### 3.2 Protocol Version

CGP uses semantic versioning. The current version is **0.1.0**.

All messages MUST include a `cgpVersion` field indicating the protocol version.

---

## 4. Message Types

### 4.1 Change Proposal

A request to evaluate and potentially release changes.

```json
{
  "cgpVersion": "0.1",
  "type": "change.proposal",
  "id": "prop-abc123",
  "timestamp": "2024-01-15T10:30:00Z",

  "actor": {
    "kind": "agent",
    "id": "claude-code",
    "name": "Claude Code Assistant"
  },

  "scope": {
    "repository": "owner/repo",
    "branch": "main",
    "commitRange": "abc123..def456",
    "commits": ["def456", "cde345", "bcd234"],
    "files": ["src/auth.ts", "src/api.ts"]
  },

  "intent": {
    "summary": "Add OAuth2 support with refresh token handling",
    "suggestedBump": "minor",
    "confidence": 0.85,
    "categories": ["feature", "security"],
    "breakingChanges": []
  },

  "context": {
    "issues": [
      {"provider": "github", "id": "123"}
    ]
  }
}
```

#### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `cgpVersion` | string | Protocol version |
| `type` | string | Must be `"change.proposal"` |
| `id` | string | Unique proposal identifier |
| `timestamp` | string | ISO 8601 timestamp |
| `actor` | Actor | Who is proposing the change |
| `scope` | Scope | What changes are included |
| `intent` | Intent | Proposer's understanding of the change |

### 4.2 Governance Decision

The governor's response to a proposal.

```json
{
  "cgpVersion": "0.1",
  "type": "change.decision",
  "id": "dec-xyz789",
  "proposalId": "prop-abc123",
  "timestamp": "2024-01-15T10:30:05Z",

  "decision": "approval_required",
  "recommendedVersion": "1.2.0",

  "riskScore": 0.45,
  "riskFactors": [
    {
      "category": "security",
      "description": "Changes to authentication module",
      "score": 0.6,
      "severity": "medium"
    }
  ],

  "rationale": [
    "Security-sensitive changes require human review",
    "Rule 'security-review' matched"
  ],

  "requiredActions": [
    {
      "type": "human_approval",
      "description": "Security team must approve authentication changes",
      "assignee": "team:security"
    }
  ],

  "analysis": {
    "features": 1,
    "fixes": 0,
    "breaking": 0,
    "security": 1
  }
}
```

#### Decision Types

| Value | Description |
|-------|-------------|
| `approved` | Change is approved for immediate release |
| `approval_required` | Human approval is required before release |
| `rejected` | Change violates policy; cannot proceed |
| `deferred` | Decision deferred; more information needed |

### 4.3 Execution Authorization

Permission to proceed with release execution.

```json
{
  "cgpVersion": "0.1",
  "type": "change.execution_authorized",
  "id": "auth-def456",
  "decisionId": "dec-xyz789",
  "proposalId": "prop-abc123",
  "timestamp": "2024-01-15T11:00:00Z",

  "approvedBy": {
    "kind": "human",
    "id": "alice@example.com",
    "name": "Alice Developer"
  },
  "approvedAt": "2024-01-15T11:00:00Z",

  "version": "1.2.0",
  "tag": "v1.2.0",

  "validUntil": "2024-01-16T11:00:00Z",
  "allowedSteps": ["tag", "changelog", "publish", "notify"],

  "approvalChain": [
    {
      "actor": {"kind": "human", "id": "alice@example.com"},
      "action": "approve",
      "timestamp": "2024-01-15T11:00:00Z",
      "comment": "LGTM - reviewed OAuth implementation"
    }
  ]
}
```

---

## 5. Actor Model

### 5.1 Actor Kinds

CGP recognizes four actor kinds:

| Kind | Description | Examples |
|------|-------------|----------|
| `human` | A human developer or operator | Developer, release manager |
| `agent` | An AI coding assistant | Claude, GitHub Copilot, Cursor |
| `ci` | A CI/CD automation system | GitHub Actions, GitLab CI, Jenkins |
| `system` | An automated internal system | Scheduled jobs, bots |

### 5.2 Actor Identification

```json
{
  "kind": "agent",
  "id": "claude-code-session-123",
  "name": "Claude Code",
  "attributes": {
    "model": "claude-3-opus",
    "session": "abc123"
  }
}
```

Actor IDs SHOULD be stable and traceable. For agents, include session or invocation identifiers. For CI systems, include pipeline/workflow IDs.

### 5.3 Trust Levels

Governors MAY assign trust levels to actors:

| Level | Value | Description |
|-------|-------|-------------|
| Untrusted | 0 | All changes require full review |
| Limited | 1 | Can propose; limited auto-approval for low-risk |
| Trusted | 2 | Can auto-release low and medium risk changes |
| Full | 3 | Equivalent to human approval authority |

Trust levels influence:
- Auto-approval thresholds
- Required number of approvers
- Allowed operations

---

## 6. Risk Assessment

### 6.1 Risk Score

Every governance decision includes a normalized risk score between 0.0 (no risk) and 1.0 (maximum risk).

### 6.2 Risk Factors

Governors SHOULD assess the following factors:

| Factor | Weight (default) | Description |
|--------|------------------|-------------|
| API Changes | 25% | Breaking or significant API modifications |
| Blast Radius | 20% | Scope of files/lines changed |
| Dependency Impact | 15% | Effect on downstream consumers |
| Security Impact | 15% | Changes to security-sensitive code |
| Historical Risk | 10% | Past issues with similar changes |
| Actor Trust | 10% | Track record of the proposer |
| Test Coverage | 5% | Coverage of changed code |

### 6.3 Severity Levels

| Score Range | Severity | Typical Action |
|-------------|----------|----------------|
| 0.0 - 0.3 | Low | May auto-approve |
| 0.3 - 0.6 | Medium | Standard review |
| 0.6 - 0.8 | High | Enhanced scrutiny |
| 0.8 - 1.0 | Critical | Block or escalate |

---

## 7. Policy Engine

### 7.1 Policy Structure

Policies are declarative rules that govern decision-making.

```
rule "block-weekend-majors" {
    priority = 100
    description = "Block major releases on weekends"

    when {
        bump_type == "major"
        day_of_week >= 5
    }

    then {
        block(reason: "Major releases not allowed on weekends")
    }
}
```

### 7.2 Condition Variables

| Variable | Type | Description |
|----------|------|-------------|
| `risk_score` | float | Overall risk (0.0-1.0) |
| `bump_type` | string | "major", "minor", "patch" |
| `has_breaking_changes` | bool | Whether breaking changes exist |
| `commit_count` | int | Number of commits |
| `files_changed` | int | Number of files modified |
| `lines_changed` | int | Lines added + removed |
| `actor_type` | string | "human", "agent", "ci", "system" |
| `actor_id` | string | Actor identifier |
| `day_of_week` | int | 0=Sunday through 6=Saturday |
| `hour` | int | Hour of day (0-23) |

### 7.3 Actions

| Action | Parameters | Description |
|--------|------------|-------------|
| `approve()` | - | Auto-approve the release |
| `block(reason)` | reason: string | Reject the release |
| `require_approval(role)` | role: string | Require human approval |
| `add_reviewer(team)` | team: string | Add additional reviewers |
| `warn(message)` | message: string | Add a warning |
| `set_risk(score)` | score: float | Override risk score |

### 7.4 Default Behavior

Policies SHOULD define a default behavior:

```
defaults {
    decision = "require_approval"
    required_approvers = 1
}
```

If no rules match and no defaults are specified, the decision SHOULD be `approval_required`.

---

## 8. Approval Workflows

### 8.1 Approval States

```
Pending → Approved → Authorized
    ↓
  Rejected
```

### 8.2 Multi-Approver Workflows

When multiple approvers are required:

```json
{
  "requiredActions": [
    {
      "type": "human_approval",
      "assignee": "team:security",
      "minimumApprovers": 1
    },
    {
      "type": "human_approval",
      "assignee": "team:release-managers",
      "minimumApprovers": 2
    }
  ]
}
```

### 8.3 Time Constraints

Authorizations MAY include validity windows:

```json
{
  "validUntil": "2024-01-16T11:00:00Z",
  "conditions": [
    {
      "type": "time_window",
      "value": "weekdays 09:00-17:00 UTC"
    }
  ]
}
```

If an authorization expires, a new approval is required.

---

## 9. Audit Trail

### 9.1 Requirements

Governors MUST maintain an audit trail with:

- **Immutability**: Entries cannot be modified after creation
- **Completeness**: All governance events are recorded
- **Attribution**: Every action is linked to an actor
- **Integrity**: Cryptographic verification of the chain

### 9.2 Event Types

| Event | Description |
|-------|-------------|
| `proposal.received` | New proposal submitted |
| `evaluation.completed` | Analysis finished |
| `decision.made` | Governance decision rendered |
| `approval.requested` | Human approval requested |
| `approval.granted` | Human approved the release |
| `approval.denied` | Human rejected the release |
| `execution.authorized` | Release authorized to proceed |
| `execution.completed` | Release executed successfully |
| `execution.failed` | Release execution failed |

### 9.3 Audit Entry Format

```json
{
  "id": "audit-123",
  "timestamp": "2024-01-15T10:30:00Z",
  "eventType": "decision.made",
  "actor": {
    "kind": "system",
    "id": "cgp-governor"
  },
  "resource": {
    "type": "proposal",
    "id": "prop-abc123"
  },
  "action": "evaluate",
  "outcome": "approval_required",
  "details": {
    "riskScore": 0.45,
    "matchedRules": ["security-review"]
  },
  "previousHash": "sha256:abc...",
  "hash": "sha256:def..."
}
```

### 9.4 Chain Verification

Audit entries form a hash chain:

```
Entry[n].hash = SHA256(
  Entry[n].id +
  Entry[n].timestamp +
  Entry[n].eventType +
  ... +
  Entry[n-1].hash
)
```

Verifiers can validate chain integrity by recomputing hashes.

---

## 10. Transport Bindings

CGP is transport-independent. This section defines standard bindings.

### 10.1 MCP (Model Context Protocol)

CGP integrates with MCP for AI agent communication:

```json
{
  "mcpServers": {
    "cgp": {
      "command": "relicta",
      "args": ["mcp", "serve"]
    }
  }
}
```

MCP tools:
- `cgp.propose` - Submit a change proposal
- `cgp.status` - Get proposal status
- `cgp.approve` - Approve a pending release
- `cgp.reject` - Reject a pending release

### 10.2 HTTP/REST

```
POST /cgp/v1/proposals
Content-Type: application/json

{proposal body}

---

200 OK
Content-Type: application/json

{decision body}
```

### 10.3 gRPC

Protocol buffer definitions available at:
`https://github.com/relicta-tech/cgp-proto`

---

## 11. Security Considerations

### 11.1 Authentication

- Actors MUST be authenticated before submitting proposals
- Credential tokens SHOULD NOT appear in audit logs (use hashes)
- Trust levels SHOULD be verified, not self-declared

### 11.2 Authorization

- Policy enforcement MUST NOT be bypassable
- Approval authority MUST be verified before granting authorization
- Execution authorization SHOULD be time-limited

### 11.3 Audit Integrity

- Audit storage SHOULD be append-only
- Hash chains enable tamper detection
- External audit log backup is RECOMMENDED

### 11.4 Data Protection

- Sensitive fields (secrets, credentials) MUST be excluded from logs
- Policy files SHOULD reference secrets via environment variables
- Transport encryption is REQUIRED for network communications

---

## 12. Implementation Guidelines

### 12.1 Minimal Implementation

A conforming CGP implementation MUST:

1. Accept `change.proposal` messages
2. Return `change.decision` messages
3. Maintain an audit trail
4. Support at least one policy rule

### 12.2 Recommended Extensions

Implementations SHOULD support:

- Risk scoring with configurable weights
- Multiple policy rules with priorities
- Human approval workflows
- MCP transport binding

### 12.3 Compliance Levels

| Level | Requirements |
|-------|--------------|
| Basic | Proposal/decision flow, basic audit |
| Standard | + Risk scoring, policies, approvals |
| Enterprise | + Historical learning, multi-tenant, SSO |

---

## Appendix A: Example Policies

### A.1 Conservative Policy

```
defaults {
    decision = "require_approval"
    required_approvers = 2
}

rule "block-agent-majors" {
    when {
        actor_type == "agent"
        bump_type == "major"
    }
    then {
        block(reason: "AI agents cannot release major versions")
    }
}

rule "security-review" {
    when {
        risk_score > 0.5
    }
    then {
        require_approval(role: "security-team")
    }
}
```

### A.2 Velocity-Optimized Policy

```
defaults {
    decision = "approve"
    required_approvers = 0
}

rule "review-high-risk" {
    when {
        risk_score > 0.7
    }
    then {
        require_approval(role: "on-call")
    }
}

rule "block-critical-risk" {
    when {
        risk_score > 0.9
    }
    then {
        block(reason: "Critical risk requires investigation")
    }
}
```

---

## Appendix B: JSON Schema

Full JSON Schema definitions are available at:
`https://github.com/relicta-tech/cgp-spec/schemas/`

---

## Appendix C: Changelog

### Version 0.1.0 (Draft)

- Initial specification
- Core message types defined
- Actor model established
- Risk assessment framework
- Policy engine specification
- Audit trail requirements

---

## References

- [Model Context Protocol (MCP)](https://modelcontextprotocol.io/)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [Semantic Versioning](https://semver.org/)
- [RFC 2119 - Key Words](https://www.rfc-editor.org/rfc/rfc2119)

---

*This specification is maintained by the Relicta project. Contributions welcome at [github.com/relicta-tech/relicta](https://github.com/relicta-tech/relicta).*
