# ADR-017: One Auto-Approval Path

## Status

Accepted (2026-08-22)

## Date

2026-08-22

## Context

A sweep for packages with no external importer — the same question that found the event
store — turned up `internal/cgp/autoapproval`: 1,107 lines of production code and 1,091
of tests, imported by nothing.

It is not merely unreached. It is a **second implementation of the decision the evaluator
already makes**, with its own model:

- risk bands (`RejectMin`, `RequireReviewMin`, `AutoApproveMax`)
- its own `AutoApprovalPolicy` type and policy list
- `Exemptions` — what may never be auto-approved
- `ActorRules` — per-actor behavior
- its own `AuditConfig`

And its own YAML-tagged configuration, which no configuration file can reach: nothing in
`internal/config` mentions it. A repository cannot switch to it even by accident.

Meanwhile the wired path — `internal/cgp/evaluator`, reached from the governance service —
covers the same ground:

| what autoapproval offered | what the wired path already does |
|---|---|
| auto-approve below a threshold | `riskAssessment.Score < AutoApproveThreshold`, gated on `Actor.TrustLevel.CanAutoApprove()` |
| a ceiling above which nothing auto-approves | `MaxAutoApproveRisk`, applied to agent actors |
| auto-reject | `DecisionRejected`, from policy results and from the freeze and budget checks |
| required approver counts | `policy.RequiredApprovers` |
| exemptions and per-actor rules | the policy engine, including the DSL loaded from `policy_dir` |
| freeze windows | `budget.CheckFreeze`, wired from `governance.freeze_periods` |

## Decision

**Delete `internal/cgp/autoapproval`.** One path decides whether a release is
auto-approved.

The reasoning is ADR-013's, applied to a decision rather than to storage: two
implementations that can disagree are worse than one that cannot. For auto-approval it is
worse still than for a data store, because the disagreement would not be visible. Both
models take a risk score and return a verdict; a repository whose policy said "never
auto-approve a breaking change" would have no way to know which model read that policy,
and the audit record would show an approval with a rationale from whichever one ran.

The specific hazard in wiring it: relicta already has a policy language — the DSL loaded
from `policy_dir`, evaluated by `internal/cgp/policy`. `autoapproval` carries a second
policy type with its own exemptions and actor rules. Wiring it would put two policy
languages in one governance tool, and every future policy feature would have to be
written twice or arbitrarily assigned to one.

## Consequences

2,198 lines removed, and nothing else changed: the package had no importer, so the build
and the whole test suite pass untouched.

What a reader loses is a design that was arguably better in one respect — explicit risk
*bands* read more clearly than two separate thresholds. If band-shaped configuration is
wanted, it belongs in the evaluator's own config, where one model can express it, and not
in a second evaluator.

The other five unreferenced packages — `cgp/ciapproval`, `application/supplychain`,
`cgp/approval`, `infrastructure/hubsync`, `cgp/policy/library` — are recorded in the
backlog and each needs its own decision. This ADR settles one of them, and the test it
sets for the rest is the same: **does something already own this job?** Where the answer
is yes, the unreached copy goes; where it is no, the question is whether the feature is a
commitment, which is how monorepo versioning was decided in ADR-015.
