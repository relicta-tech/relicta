# ADR-009: Relicta Emits a Deterministic Recommendation, Not Prose

## Status

Accepted (2026-08-10)

The decision is settled and the artifact is the contract that ADR-010 and
ADR-011 build on, so leaving it Proposed misrepresented it as still open.

Implemented for `relicta plan --json` and the MCP `relicta_plan` tool, with
tests asserting the two properties the decision rests on: `TestBuild_NoProseFields`
and `TestBuild_IsDeterministic`, plus `TestDigest_StableAcrossCalls` for
provenance.

**Not yet on every interface.** The decision text says "CLI JSON output, MCP tool
results, HTTP API"; the HTTP API does not emit the artifact. That is outstanding
work against an accepted decision, not a reason to keep the decision provisional
— but it does mean a Hub reading the HTTP API today gets a different shape than an
agent reading MCP.

## Date

2026-08-08

## Relationship to earlier ADRs

The index lists ADR-005 "Multi-provider AI Integration" and ADR-006 "Model
Context Protocol for AI Agent Integration" as Accepted, but neither file exists
in the repository, and git history shows they were never committed — ADR-001
through ADR-006 are index rows pointing at nothing.

That matters here rather than being a filing complaint: AI's role in Relicta was
never actually written down and argued. This ADR is the first record of that
decision, so it supersedes nothing — it fills a gap. If the reasoning behind
ADR-005 exists somewhere outside the repository, it should be reconciled with
this before this is accepted.

## Context

Relicta currently does two different kinds of work behind one interface.

**Deterministic work.** Analyze commits, compute the next version, score risk,
evaluate policy, record an audit trail. This is reproducible: the same inputs
produce the same outputs, and `internal/cgp/risk/calculator.go` is weighted
arithmetic over classified change facts. Nothing in `internal/cgp`,
`internal/application/governance` or `internal/domain/release` imports an AI
provider.

**Non-deterministic work.** Write a changelog, release notes, a marketing
blurb, an audience-aware narrative. This lives in
`internal/infrastructure/ai` (3,301 LOC, four provider SDKs) and is reached
only from `NotesGeneratorAdapter` and `relicta communicate`.

The second kind is already optional and already off by default:

| Setting | Default |
|---|---|
| `ai.enabled` | `false` |
| `relicta notes --ai` | `false` — flag removed 2026-08-09; AI notes now follow `ai.enabled` |
| Deterministic fallback (`generateBasicNotes`) | exists, used whenever AI is off |
| Build tags to compile providers out (`relicta_minimal`) | exist |

So the architecture has already drifted toward "deterministic core, optional
prose". Two problems remain.

### Prose in an audit trail cannot be re-derived

Relicta's claim is a governance record: this change was assessed, scored and
approved for these reasons. That claim depends on the record being
reproducible. AI-generated text is not. The codebase half-acknowledges this by
persisting `Notes.Provider` and `Notes.Model`, but recording *which* model
produced unreproducible text does not make it reproducible.

### The MCP path calls a second model that knows less than the first

An agent calls `relicta_notes` over MCP; relicta then calls its own provider.
The caller is already a language model holding the repository, the diff, the
issue tracker and the conversation. Relicta's provider call receives a
categorized changeset. The caller is strictly better placed to write the prose.

`Adapter.HasAIService()` exists in `internal/mcp/adapters.go` and is never
called outside tests, which is a fair summary of how load-bearing AI is on that
path.

## Decision

**Relicta emits a deterministic recommendation artifact. It does not emit
prose.**

Every interface — CLI JSON output, MCP tool results, HTTP API — returns the
same artifact: the facts it derived, the assessment it computed, the verdict it
recommends, the obligations that remain, and the provenance needed to verify
all of it. Rendering that into language is the caller's job: an agent, a
template, or Relicta Hub.

### The artifact

`schema_version` is independent of the CLI version, because this is a contract
consumed by agents and by Hub.

```json
{
  "schema_version": "1.0.0",
  "generated_at": "2026-08-08T09:00:00Z",

  "subject": {
    "repository": "relicta-tech/relicta",
    "branch": "main",
    "base_ref": "v4.2.0",
    "head_sha": "1fe781b42990338c3ebcd93d85999be5a3317db7"
  },

  "proposal": {
    "actor": { "kind": "agent", "id": "claude-code@team-platform" },
    "intent": {
      "summary": "add gradle version property support",
      "suggested_bump": "minor",
      "declared_confidence": 0.62
    }
  },

  "facts": {
    "current_version": "4.2.0",
    "next_version": "4.3.0",
    "release_type": "minor",
    "commit_count": 23,
    "changes": [
      {
        "type": "feat",
        "scope": "versioning",
        "subject": "support several version manifests in different formats",
        "sha": "7787c6b",
        "breaking": false
      },
      {
        "type": "fix",
        "scope": "cli",
        "subject": "resolve repository and config from any subdirectory",
        "sha": "1dca2f4",
        "breaking": false
      }
    ],
    "breaking_changes": [],
    "blast_radius": {
      "from_ref": "v4.2.0",
      "to_ref": "1fe781b",
      "packages": 7,
      "changed_files": 23,
      "impacts": []
    }
  },

  "assessment": {
    "risk_score": 0.34,
    "severity": "medium",
    "factors": [
      { "category": "api_change",   "score": 0.10, "severity": "low",    "description": "no exported signatures changed" },
      { "category": "blast_radius", "score": 0.18, "severity": "medium", "description": "7 packages touched" },
      { "category": "historical",   "score": 0.06, "severity": "low",    "description": "no incidents in the last 10 releases" }
    ],
    "thresholds": {
      "auto_approve_below": 0.30,
      "max_auto_approve_risk": 0.50,
      "require_human_for_breaking": true
    },
    "policy": {
      "matched_rules": ["cfg_review-low-confidence-proposals"],
      "required_approvers": 1,
      "blocked": false,
      "rule_trace": [
        {
          "rule_id": "cfg_review-low-confidence-proposals",
          "rule_name": "review-low-confidence-proposals",
          "priority": 100,
          "matched": true,
          "conditions": [
            { "field": "intent.confidence", "operator": "lt", "expected": 0.70, "actual": 0.62, "matched": true }
          ]
        }
      ]
    }
  },

  "verdict": {
    "decision": "approval_required",
    "recommended_version": "4.3.0",
    "rationale": [
      "proposer declared confidence 0.62, below the 0.70 review threshold",
      "policy rule cfg_review-low-confidence-proposals requires 1 approver"
    ]
  },

  "obligations": [
    {
      "type": "human_approval",
      "description": "one approver required before publish",
      "blocking": true
    }
  ],

  "provenance": {
    "tool_version": "4.3.0",
    "config_hash": "sha256:2b1f…",
    "plan_hash": "sha256:9ac3…",
    "policy_source": ".relicta/policies/release.policy",
    "deterministic": true,
    "inputs_digest": "sha256:71de…"
  }
}
```

### Rules the artifact obeys

1. **No prose fields.** `description`, `subject` and `rationale` are
   fixed strings derived from data, not generated text. A consumer can render
   them verbatim or rewrite them; nothing in the artifact was written by a
   model.
2. **`facts` is writer-ready.** Changes arrive grouped, typed, scoped, with
   breaking changes separated. A caller writing release notes should not need a
   second call to Relicta to get the material.
3. **`deterministic: true` is a claim the artifact backs.** `inputs_digest`
   covers head SHA, base ref, config hash and policy source. Two runs agreeing
   on `inputs_digest` must agree on `facts`, `assessment` and `verdict`. This
   is testable, and should be tested.
4. **`verdict` recommends; it does not act.** Publishing stays a separate,
   explicitly invoked step.
5. **Confidence belongs to the proposer, not the verdict.** There is no
   `verdict.confidence`. A confidence number attached to a deterministic
   computation invites "confidence in what?", and the honest answer is that the
   arithmetic has no uncertainty — the uncertainty is in the weights, which are
   configuration.

   What *is* meaningful is `proposal.intent.declared_confidence`: how sure the
   proposing actor said it was. That is an input, and it is already a governance
   signal — `buildEvalContext` (`cgp/policy/engine.go:511`) puts
   `intent.confidence` into the policy evaluation context, so a rule can already
   say "require review when the proposer is less than 70% sure".

   Surfacing it makes the most interesting sentence in an AI-driven release
   auditable: *the agent said it was 62% sure this was a minor bump, and policy
   required a human because of it.* That is the product.

### Mapping to existing types

Most of this already exists and is a serialization exercise, not new logic.

| Artifact section | Source |
|---|---|
| `proposal` | `cgp.ChangeProposal` — `Actor`, `Intent.Summary`, `Intent.SuggestedBump`, `Intent.Confidence` |
| `facts` | `service/release.AnalyzeOutput`, `domain/changes.ChangeSet` |
| `facts.blast_radius` | `application/blast.BlastRadius` (`Packages`, `ChangedFiles`, `Impacts`, `FromRef`, `ToRef`) |
| `assessment.risk_score`, `.factors` | `cgp/risk.Assessment`, `cgp.RiskFactor` |
| `assessment.thresholds` | `config.GovernanceConfig` |
| `assessment.policy` | `cgp/policy.Result`, including its existing `RuleTrace` / `ConditionTrace` — already JSON-tagged with `expected`/`actual`/`matched` |
| `verdict`, `obligations` | `cgp.GovernanceDecision`, `cgp.RequiredAction` |
| `provenance.plan_hash`, `config_hash` | already on `domain/release.ReleaseRun` |

`cgp.GovernanceDecision` in the public `pkg/cgp` SDK already carries
`Decision`, `RecommendedVersion`, `RiskScore`, `RiskFactors`, `Rationale` and
`RequiredActions`. The artifact wraps it with the facts that justified it and
the provenance that lets someone check it.

New work is limited to: `provenance.inputs_digest`, `assessment.thresholds`
(currently implicit in config), and the envelope itself.

Note that `Confidence` lives on `ProposalIntent`, not on `GovernanceDecision` —
it is the proposer's declared confidence, which is why it appears under
`proposal` rather than `verdict`.

## Consequences

### Good

- The audit trail becomes reproducible, which is the product's actual claim.
- The MCP surface stops making a redundant model call with less context than
  its caller.
- The default binary can drop four provider SDKs by making `relicta_minimal`
  the default build.
- The artifact is a clean commercial boundary: the deterministic verdict stays
  free and verifiable; turning it into house-styled prose is a service. See
  ADR-010 if that is pursued.

### Bad, or at least costly

- `relicta notes` output changes shape for anyone consuming today's prose.
  Mitigation: keep the deterministic template renderer as the default text
  output, and gate the artifact behind `--json` initially.
- `relicta communicate` becomes the one command that is explicitly
  non-deterministic. It should say so, and its output should be excluded from
  the audit trail or recorded with provenance rather than as fact.
- Someone must maintain the template renderer that currently only exists as
  `generateBasicNotes`.

### Explicitly not decided here

Whether AI leaves the CLI entirely. This ADR removes it from the *critical
path* and from the *audit trail*. Whether the provider code is deleted,
demoted to a plugin, or moved to Hub is a separate decision.

## Guardrail

Gate language, never judgement. Risk scoring, policy evaluation and this
artifact stay free and locally verifiable. A governance tool whose assessment
you cannot reproduce yourself is a black box you rent, which is the opposite of
what Relicta is for.
