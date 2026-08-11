# ADR-012: Deployment Evidence Crosses a Protocol, Not a Dependency

## Status

Accepted (2026-08-11)

## Date

2026-08-11

## Context

Relicta records that a release was governed and published. It records nothing
about that release reaching an environment, so the evidence chain ends at the tag:

```
proposed → risk assessed → approved → tagged & published → ???
```

For a tool whose product is audit evidence, the missing step is the one an auditor
asks about. "Approved and released" is a weaker claim than "approved, released, and
running in production since 14:22, deployed by this actor".

This is not only a gap in the record. The DORA report already computes deployment
metrics **from release records**, which measures the wrong event:

- deployment frequency counts tags, so a project that tags weekly and deploys
  daily is wrong in both directions
- lead time for changes measures commit → tag, not commit → running, which is the
  metric's definition
- change failure rate is uncomputable, because a failed deployment of a good
  release is invisible — and that is exactly what the metric asks about
- MTTR has no anchor, because an incident cannot be tied to the deployment that
  caused it

So deployment records are a missing *input* to numbers already being reported, not
a new feature bolted alongside them.

### The deployer is a separate product

The concrete case driving this: releases are published by relicta through GitHub,
and deployed by **Rollops**, a GitOps rollout controller — a separate product with
its own repository, release cadence and users. Nothing in relicta's release
pipeline knows when a deployment happened, because by then the pipeline has exited
and a controller is reconciling desired state on its own schedule.

Both products already have the sockets this needs, and one of them is dormant:

| Rollops has | shape |
|---|---|
| `internal/notify` webhook | POSTs `{kind, target_ref, rollout_id, detail}` on `promoted`, `failed`, `rolled_back`, `approval_needed`, signed HMAC-SHA256 in `X-Rollops-Signature` |
| `internal/governance` | `Provider interface { Evaluate(Request) (Decision, error) }` with `Decision{Allowed, Reason, Evidence map[string]string}`; `Hook` returns `Allowed: true` when no provider is set |

Nothing in Rollops supplies a `Provider` and nothing calls `Evaluate`, so the gate
is designed and unused. Rollops also states the boundary explicitly in
`internal/condition/condition.go` — "there is no bespoke DSL, and relicta's
governance DSL stays separate" — so neither side reimplements the other's policy
language.

### The constraint

**Rollops must not depend on relicta, and relicta must not depend on Rollops.**
They are separate products that must remain independently buildable, releasable and
usable. A user of either must never be obliged to adopt the other.

That rules out the obvious implementations. A relicta-specific `Provider` living in
Rollops' tree would import relicta. A Rollops-specific receiver in relicta would
import Rollops' event types. Either makes one product's release cadence a
constraint on the other's, and makes a CVE in one a CVE surface for the other.

## Decision

**Deployment evidence and deployment gating cross a documented wire protocol.
Neither product names the other in code, in configuration defaults, or in its
module graph.**

### 1. Both flows are initiated by the deployer

```
Rollops ──(1) may I deploy version V to environment E? ──▶ relicta
        ◀──────── decision + evidence ────────────────────
        ──(2) version V reached environment E at T ──────▶ relicta
```

Rollops initiates both. Relicta never reaches into a cluster, needs no credentials
for one, and does not poll. This also settles a fidelity question: a manifest
commit is a *request* to deploy, while the controller reporting healthy is the
*fact*. Only the deployer knows the fact, so only the deployer can report it.

### 2. Neither side is written against the other

- Rollops ships a **generic HTTP governance provider**: a URL, a signing secret, a
  timeout. Not "the relicta provider". Anything answering the documented contract
  works, and Rollops with no URL configured behaves exactly as it does today.
- Relicta ships a **generic deployment receiver**: a documented event schema on an
  authenticated endpoint. Not "the Rollops webhook". Any deployer — a CI step, a
  hand-rolled script, a different controller — can post to it.
- Anything that genuinely needs both sides' internals is a **plugin in its own
  repository**, which is already Rollops' established pattern (`pkg/target` is
  public precisely so third-party plugins can implement it, and
  `rollops-plugin-datadog` and `rollops-plugin-unleash` live outside the core).

### 3. The governance answer travels as CGP

The decision flow uses the Change Governance Protocol, which exists for this
purpose: `pkg/cgp` is already a public SDK with a versioned wire format
(`cgpVersion`), message types and validation, and ADR-009 already made a versioned
artifact the contract every interface returns.

Rollops implements the wire format, not the Go package. The subset it needs is
small — a request naming the version, environment and actor, and a decision
carrying allowed/reason/evidence — and hand-rolling that subset is preferable to a
module dependency. If the SDK is ever worth sharing as Go code, it must be
published as a **standalone module** so that importing the protocol does not mean
importing relicta; that is a precondition, not an implementation detail.

`Decision.Evidence map[string]string` maps cleanly onto what relicta can prove:
the release run ID, the risk score, the approver, the policy that decided, and the
recommendation artifact's digest. Rollops then records *why* a deployment was
allowed rather than merely that it was.

### 4. Deployment records are their own entity

A deployment is not a field on a release. One release deploys to several
environments at different times with independent outcomes, and a rollback is a
deployment too. Records are keyed by the canonical repository identity (see the
governance identity work) and stored beside release records, so the report path
finds them with no further plumbing.

Environments are **declared** in configuration with exactly one marked as
production. Free-form environment names let `prod`, `production` and `Production`
become three environments in an audit report, and deployment frequency has to mean
"reaching users" or it counts staging and reads high.

### 5. A configured gate fails closed

Rollops' `Hook` returns `Allowed: true` when no provider is configured. That is
correct: a user who has not asked for governance must not be blocked by it.

But once a provider **is** configured and relicta is unreachable, the answer must
be deny, not allow. Governance that evaporates when the network is bad is
governance that is absent exactly when a rushed deploy is most likely. This mirrors
the same decision made for Hub's authentication, which used to fail open with only
a startup banner as protection.

The distinction is between "no governance requested" and "governance requested and
unavailable". The first is a configuration; the second is a failure.

### 6. Two risk scores, two questions

Relicta computes CGP risk; Rollops computes a decision-kit blast-radius score. Two
components producing one number for one change is the shape that caused three
separate defects in relicta — a duplicate release store, a bare-versus-configured
evaluator, and plan disagreeing with bump.

These are genuinely different questions and both should exist:

- **relicta scores the change** — what is in it: breaking commits, security
  touches, blast radius over the changed files, actor trust
- **Rollops scores the rollout** — where it is going: which target, what traffic
  share, what depends on it, what a failure there costs

Neither may recompute the other's number. When Rollops asks relicta for a
decision, relicta's score travels in `Evidence` as a fact Rollops records, not as
an input Rollops re-derives.

## Why not the alternatives

**A Go dependency in either direction.** Simplest to write and the reason this ADR
exists. It couples release cadences, makes each product's dependency graph the
other's problem, and forces a user of one to acquire the other. The independence
constraint is a product decision, not a technical preference.

**Relicta polls the cluster or the GitOps repository.** Needs cluster credentials
relicta has no business holding, and infers the fact from the intent: a manifest
commit says a deployment was requested, not that it succeeded. Useful later as a
*reconciliation* source — a version running with no deployment record is a finding
— but wrong as the primary signal.

**A shared library both import.** Moves the coupling rather than removing it: a
change to the shared library still forces coordinated releases. A wire format
versioned independently of both products does not.

**Recording only, no gate.** This was the smaller option and it is not enough on
its own. Recording detects an ungoverned deployment after it has shipped; the gate
prevents it. Both are in scope, and the record is the fallback for when the gate is
not configured.

## Consequences

Easier:

- deployment frequency, lead time and change failure rate measure deployments, so
  the DORA report stops answering a question it was not asked
- the evidence chain reaches the environment, which is what SOC 2 and EU AI Act
  Article 12 evidence is actually asked to show
- an **ungoverned deployment becomes detectable**: a version running with no
  relicta release record means someone deployed around the governance, and that is
  the finding a governance tool should care most about
- any deployer can integrate, because the contract is a documented schema rather
  than a Go interface — a CI step with `curl` is a first-class client

Harder:

- two wire contracts now need versioning and compatibility discipline; a protocol
  is a public API even when only two products speak it
- an integration test needs both products, so it belongs in neither repository —
  a third harness, or documented manual verification, and saying which
- a self-reported deployment is only as good as the reporter. The record must carry
  provenance naming what observed it, so a controller-observed deployment and a
  manifest-inferred one are not weighed equally by an auditor

Invariant this ADR is worth nothing without, and which is checkable rather than
aspirational:

> Each repository must build, test and pass CI with the other absent, and neither
> `go.mod` may reference the other. A test asserting the absence of that dependency
> is cheap; without it, this decision decays the first time someone reaches for a
> convenient import.
