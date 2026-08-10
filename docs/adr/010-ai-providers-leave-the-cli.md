# ADR-010: AI Providers Leave the CLI

## Status

Accepted (2026-08-10), execution phased.

The direction is decided. The three steps below have prerequisites this ADR
itself calls non-negotiable, so Accepted records the decision rather than the
completion. Status of each, measured on this repository:

| Step | State |
|---|---|
| 1. Deterministic artifact is the product surface | Done for `plan --json` and MCP `relicta_plan` (ADR-009). Not on the HTTP API. |
| `notes --ai` removed from the CLI | Done — the flag generated prose the artifact already carried, and its removal was reversible, so it went first. |
| 2. `communicate` becomes a Hub capability | **Blocked.** Hub still lacks metering, rate limiting, API keys, and tenancy, and its JWT auth fails open with no secret configured. Removing `communicate` before Hub can serve it would delete a feature with nothing to replace it. |
| 3. `relicta-ai` gRPC plugin escape hatch | Not started. No prose hook exists yet; the plugin machinery (ADR-008) does. |
| Provider SDKs leave the module | Not started. `go-anthropic` and `go-openai` remain direct dependencies, and `internal/infrastructure/ai` is still in the module. |

The reversible parts were done first deliberately: they reduce surface without
betting on Hub arriving. Nothing further should be removed from the CLI until
step 2 or step 3 can serve what it replaced.

## Date

2026-08-08

## Context

ADR-009 decided that Relicta emits a deterministic recommendation artifact and
leaves prose to the caller. It deliberately did not decide what happens to the
provider code, only that it stops being on the critical path.

This ADR decides that.

### Build tags are not a removal mechanism

**Superseded 2026-08-10: the AI build tags have been removed. Relicta ships one
binary with every provider compiled in.** The measurements below are why that cost
nothing worth keeping, and they are the reason "compile it out" was never the
answer to "remove it". The rest of this ADR — the SDKs leaving the module — is
unaffected.

`internal/infrastructure/ai` was guarded by build tags, so a `relicta_minimal`
binary linked none of it. But that did not remove anything from the supply
chain:

| | Result |
|---|---|
| `go mod tidy` with `-tags relicta_minimal` | `go.mod` unchanged — tidy keeps every tag combination buildable |
| `sbom-source.cdx.json` | all three SDKs present |
| `sbom-binary.cdx.json` | all three SDKs present |

A minimal build still declared `go-openai`, `go-anthropic` and `genai`, still
listed them in both SBOMs, still received Dependabot PRs for them, and still failed
CI on their CVEs. The measured effect of the tags was 54 fewer linked packages and
4 MB of binary — not a smaller dependency surface.

So "compile it out" was never the answer to "remove it". The code has to leave
the module, which is what the decision below says.

Keeping the tags for 4 MB was not worth their cost: 11 files carrying build
constraints, a CI step building and testing four combinations, and — discovered
when they were removed — a set of compile-time interface assertions that
`//go:build relicta_anthropic && relicta_openai` excluded from every configuration,
so they had never been checked once despite a comment claiming the default build
exercised them. One binary is simpler and, in that respect, better tested.

### What the provider code is actually for

Five methods, all prose: changelog, release notes, marketing blurb, summary, raw
completion. Nothing in `internal/cgp`, `internal/application/governance` or
`internal/domain/release` imports a provider. `ai.enabled` defaults to false and a
deterministic fallback already exists. (`notes --ai` was removed on 2026-08-09;
AI notes are requested through `ai.enabled`, which also resolved the "not decided
here" question below.)

The one command where a model earns its place is `relicta communicate`:
audience-aware narratives are a language task a template cannot fake.

## Decision

**Provider SDKs leave the Relicta module. `communicate` becomes a Hub service.
A gRPC plugin is the escape hatch for local prose without Hub.**

Three consequences, in the order they must happen.

### 1. The deterministic artifact is the product surface

Already decided in ADR-009 and implemented for `plan --json` and the MCP
`relicta_plan` tool. Nothing further needed here, but it is the precondition:
prose can only move out once the material for writing it is available in
structured form.

### 2. `communicate` becomes a Hub capability

Fleet-wide voice consistency is an organizational property. A local CLI cannot
deliver it — it needs a central place that knows what the rest of the org sounds
like. That is a reason for Hub to exist that dashboards are not, and it is the
natural home for the accumulated asset: approved terminology, banned phrasing,
prior notes as examples, reviewer edits fed back.

Hub returns prose **plus provenance** — content hash, org template version, model
ID, prompt version — which the CLI records on the run. The audit claim becomes
"this text was produced by org template v7, model X, hash abc": verifiable
provenance rather than reproducibility.

**This step has prerequisites Hub does not currently have.** Measured on
`relicta-hub` main:

| Primitive | State |
|---|---|
| metering / usage / quota / billing | absent |
| rate limiting | absent |
| API keys / rotation | absent |
| plans / subscriptions / tenancy | absent |
| JWT auth | present, but **fail-open** when no secret is configured |

Those were acceptable while Hub was a dashboard. A paid metered endpoint needs
usage records defensible in a billing dispute, and auth that fails closed.
Removing `communicate` from the CLI before Hub can serve it would delete a
feature with nothing to replace it, so the order is not negotiable.

### 3. A gRPC plugin is the escape hatch

Relicta already has the machinery: HashiCorp go-plugin over gRPC, a registry at
`plugins/registry.yaml`, hook-based lifecycle (ADR-008). An `relicta-ai` plugin
puts the SDKs in a separate repository with its own module, release cadence and
CVE surface, and serves anyone who wants local prose without Hub or an agent.

The plugin consumes the ADR-009 artifact and returns prose, which is the same
contract Hub uses. One interface, two implementations.

## Why not the alternatives

**A submodule in this repository** (`internal/infrastructure/ai` → its own
module) is the cheapest real removal, but it means maintaining two modules and
the version relationship between them. That is precisely the pain the
`replace github.com/relicta-tech/relicta => ../relicta` directive already causes
in Hub, where a CLI dependency bump silently untidies Hub's `go.mod`. Adding a
second instance of a problem already in evidence is a poor trade.

**Keeping the SDKs and relying on build tags** is the status quo. It is
defensible if the only goal is binary size, and indefensible if the goal is
supply-chain surface, per the measurements above.

**Deleting prose generation outright with no replacement** would strand humans
who run the CLI without an agent and want more than a template. The plugin
escape hatch exists for them.

## Consequences

### Good

- The Relicta module stops declaring provider SDKs: no Dependabot noise, no CVE
  gating, smaller SBOM, for a capability most users have switched off anyway.
- `ai.enabled` and its configuration surface leave the core config schema.
- The commercial boundary lands where the value is: deterministic judgement free
  and verifiable, organizational voice paid.

### Bad, or at least costly

- `relicta communicate` becomes unavailable in the OSS binary without either Hub
  or a plugin. That is a real capability reduction and needs saying plainly in
  release notes, not buried.
- Hub must build billing primitives it does not have before this can complete.
- A new plugin repository is a new thing to maintain, release and secure.
- Anyone currently using `notes --ai` needs the plugin or Hub. Given the flag
  defaults to false this is likely a small population, but it is not empty.

### Decided since

`relicta notes --ai` is **removed** (2026-08-09). Removing rather than redirecting:
the flag defaulted to false, so it was an opt-in on top of an opt-in, and it left
`notes` and `release` disagreeing — one read a flag, the other read `ai.enabled`.
AI notes are now requested one way, through configuration, which is also the thing
a plugin or Hub can take over later without another flag change.

## Guardrail, restated from ADR-009

Gate language, never judgement. Risk scoring, policy evaluation and the
recommendation artifact stay in the free, locally verifiable core. If assessment
ever moves behind Hub, Relicta stops being a governance tool you can check and
becomes a black box you rent.
