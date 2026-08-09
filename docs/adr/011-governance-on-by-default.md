# ADR-011: Governance On By Default

## Status

Proposed

## Date

2026-08-09

## Context

Relicta describes itself as "the governance layer for software change". Out of the
box it is not one: `governance.enabled` defaults to `false`, so a project that runs
`relicta init` gets semantic versioning, changelogs and publishing, and no
governance at all. `relicta evaluate` and `relicta analytics` refuse to run, and
`relicta approve` asks nothing of anybody.

Two changes have removed the accidental reasons this was hard to notice. The
setting is now written into the generated config with a comment explaining what it
does, and the error that used to say "enable governance in .relicta.yaml" — about a
section the file did not contain — now carries the YAML to add. What remains is the
deliberate question: should the default be off?

### What "on" now means

Until recently the answer did not matter much, because governance did not work.
Over the last two days:

| | before | now |
|---|---|---|
| `relicta evaluate` | failed on every release, in every repository | returns risk, factors, severity, verdict |
| the approval gate | `approve --ci` approved what governance reserved for a human | refuses, with an auditable `--override-governance` |
| audit attribution | pipeline approvals recorded as human decisions | records `ci` vs `human`, and override reasons |
| `relicta_evaluate` over MCP | `-32603 "internal error"` | returns the verdict to the agent |

So "on" is now a meaningful state rather than a broken one, which is what makes
this worth deciding.

### What the defaults actually do

With `enabled: true` and nothing else configured, on a breaking change:

```
Decision:       approval_required
Auto-Approve:   false
Required Actions:
  - [human_approval] Review breaking changes before release
Rationale:
  - No policy rules configured; built-in governance rules apply
  - 1 breaking changes detected - human review required
```

A benign `fix:` auto-approves. The gates come from `require_human_for_breaking`
and `require_human_for_security`, both true by default, and
`auto_approve_threshold` at 0.3. No policy file is needed for any of that.

## Decision

**Not taken here.** This ADR sets out the options and the cost of each; the choice
belongs to the product owner.

### Option A — on by default

`governance.enabled` defaults to `true`. A new project gets risk scoring and the
breaking-change gate without knowing the setting exists.

The product claim becomes true on first run, which is the strongest argument. The
name of the tool is "the governance layer"; a governance layer that has to be
switched on is a versioning tool with an optional extra.

Cost, and it is real: existing projects upgrading get a behaviour change they did
not ask for. A pipeline that runs `relicta approve --ci` on a release with a
breaking change starts failing. That is arguably the correct failure — those
pipelines believed they were governed and were not — but it arrives at upgrade
time rather than when someone chose it, which is the wrong moment for a surprise.

Mitigations: gate it on a config schema version so only new configs get the new
default; or emit a one-time notice when an existing config has no explicit
`governance` block; or both.

### Option B — off by default, loudly

Keep `false`, but make the absence visible. `relicta plan` and `relicta publish`
could report "governance is off; this release is not being assessed" once per run,
the way the config now warns about `git_push`.

Nothing breaks for anyone. The cost is that the product's headline capability stays
opt-in, and a notice on every run is the kind of thing people learn to skip.

### Option C — on by default for new projects only

`relicta init` writes `enabled: true`; the schema default stays `false`. New
projects are governed, existing ones are untouched, and nobody's pipeline changes
until they choose to.

The cost is a split brain: the same field means different things depending on
whether a config was generated before or after this change, and someone reading
the schema default gets the wrong answer about what their colleague's project
does. It also does nothing for the projects most likely to want governance —
established ones with real release risk.

## Recommendation

Option C first, then A behind a schema version.

C is the only option that improves the first-run experience without changing
anyone's behaviour under their feet, and it is reversible. It also produces the
evidence A needs: if governed-by-default projects turn it off, that is worth
knowing before making it universal.

The split-brain cost is mitigated by the config now being written explicitly — a
generated file states `enabled: true` in the file rather than relying on a default,
so reading the config still tells the truth about that project.

## Consequences

### If A or C is adopted

- The product claim holds without configuration, which is the point of the ADR.
- Breaking and security-related changes require human approval before publish.
  Teams that release breaking changes through automation will feel this first.
- `relicta evaluate` and `relicta analytics` start working out of the box, which
  makes the risk data and DORA metrics available without a setup step.

### Either way

- The governance defaults deserve their own review. `auto_approve_threshold: 0.3`
  and the risk weights were not chosen against real release data, and a governance
  tool's thresholds are a claim about what is safe. Worth calibrating separately
  from this decision.
- `severity` reads "low" for a change carrying a 50% `security_impact` factor
  because the aggregate score stays under the threshold. That is defensible
  arithmetic and a confusing thing to show a human. Also worth its own look.

### Not decided here

Whether `strict_mode` should follow. Today it turns a governance evaluation
failure from a warning into a hard error; with governance on by default, a
misconfigured policy file would block releases. That is a separate trade between
failing closed and failing usable.
