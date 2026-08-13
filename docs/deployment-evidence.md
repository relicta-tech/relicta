# Deployment evidence and the deployment gate

Relicta governs a release: it decides what ships, records why, and publishes a tag.
Whether that version then reached an environment is a separate fact, and relicta
cannot observe it — a tag being published is not a change reaching users.

Two features close that gap, and they are not alternatives:

| | question | when |
|---|---|---|
| **Evidence** | did version V reach environment E? | after the deployment |
| **The gate** | may version V reach environment E? | before the deployment |

Evidence turns an ungoverned production deployment into a finding someone reads. The
gate turns it into a deployment that did not happen. Detection is what you have when
you cannot prevent, and not every deployer can be made to ask — so both exist, and
evidence is the fallback wherever the gate is not configured.

See [ADR-012](adr/012-deployment-evidence-over-a-protocol.md) for the reasoning,
including why this crosses a protocol rather than a dependency.

## Declare your environments

Both features need to know which environment counts as production:

```yaml
# .relicta.yaml
environments:
  - name: staging
    description: pre-release verification
  - name: production
    production: true
```

Exactly one environment should carry `production: true`. Deployment frequency means
*changes reaching users*, so a project deploying to three environments per change
would otherwise appear to deploy three times as often as it does.

**Empty means off.** A project that declares nothing is not tracking deployments and
must not be told it is missing evidence it never promised — and the gate stays
inactive rather than guessing which environment matters.

## Recording deployments

### From a machine that has a checkout

```bash
relicta deploy record --environment production --version 1.4.0
relicta deploy record --environment production --version 1.4.1 --outcome failed
relicta deploy list
```

### From a machine that does not

A GitOps controller reconciling in a cluster has no checkout and cannot run the CLI,
so it posts instead:

```bash
curl -X POST https://relicta.internal/api/v1/webhooks/deployments \
  -H 'Content-Type: application/json' \
  -H "X-Relicta-Signature: sha256=$SIGNATURE" \
  -d '{
    "environment": "production",
    "version": "1.4.0",
    "outcome": "succeeded",
    "reference": "rollout-8812",
    "release_id": "run-7",
    "provenance": "reported"
  }'
```

Deliberately generic: any deployer posting this schema is a first-class client — a CI
step with curl, a controller, a script. Only `environment` and `version` are required.

- `outcome` — `succeeded` (default), `failed`, `rolled_back`. An unrecognized value is
  refused rather than stored, because a value counted as neither success nor failure
  biases every rate derived from it.
- `provenance` — `reported` (default), `inferred`, `manual`. A reporter deducing a
  deployment from desired state should say `inferred`, so an auditor can weigh a
  controller's own report differently from a guess.
- `release_id` — links to the relicta run when the reporter knows it. **Absent is
  meaningful, not missing:** a deployment with no release is a version that reached an
  environment without passing through governance.
- `reference` — points back at the reporter's own record, so a claim can be followed
  to its source.

The repository is resolved server-side and is **not** accepted from the payload. A
reporter authenticated for one server must not be able to write governance records
attributed to a different repository.

Sign the body with HMAC-SHA256 and send it as `X-Relicta-Signature` when
`RELICTA_WEBHOOK_SECRET` is set. With no secret configured the endpoint accepts
unsigned requests — that is a deployment decision, not a default to rely on, because
this writes governance evidence.

## The gate

A deployer asks before deploying, and relicta answers from what it governed:

```bash
curl -X POST https://relicta.internal/api/v1/webhooks/authorize \
  -H 'Content-Type: application/json' \
  -H "X-Relicta-Signature: sha256=$SIGNATURE" \
  -d '{"action":"apply","environment":"production","version":"1.4.0","target_ref":"k8s/prod/api"}'
```

```jsonc
// governed and approved
{
  "allowed": true,
  "reason": "release 1.4.0 was governed and approved",
  "evidence": {
    "release_id": "run-7", "version": "1.4.0", "environment": "production",
    "decision": "approved", "risk_score": "4.5",
    "released_by": "human:felix", "released_at": "2026-08-11T09:14:00Z"
  }
}

// nothing governed this
{
  "allowed": false,
  "reason": "relicta has no release record for version 9.9.9: it was not governed, so it must not reach production. Release it through relicta first",
  "evidence": {"version": "9.9.9", "environment": "production"}
}
```

`evidence` exists so an audit trail says *why* a deployment was permitted — the
release, the decision, the risk score, who released it — rather than only that it was.

Send `"action": "probe"` for a readiness check. It is always allowed and decides
nothing, so a caller can verify the gate is reachable without recording a decision.

### What it refuses, and what it deliberately does not

**Production only.** A version legitimately reaches staging *before* it is released —
that is what staging is for. Requiring a release record there would refuse every
pre-release deploy and teach people to switch the gate off, and a gate that is off
does not protect production either. Non-production requests are allowed and say so in
`evidence.gate`.

**A record is not permission.** A release that was rejected, deferred, or is still
awaiting approval is refused. Without that, proposing a release would be enough to
deploy it and the approval step would decide nothing.

**A `v` prefix is not a different version.** Deployers usually read a version off an
image tag, where `v1.2.3` and `1.2.3` are both common. Refusing on that difference
would report a governed release as ungoverned — a false alarm that teaches people the
gate is broken, which is worse than no gate.

**It never answers "allowed" because it could not check.** An unreadable store or
history returns `5xx`, not a permissive decision, because a caller cannot distinguish
allow-because-governed from allow-because-broken.

Callers should **fail closed**: treat a refusal, an error, and an unreachable relicta
as "do not deploy". "Governance not requested" and "governance requested but
unavailable" are different states, and a gate that evaporates on a bad network is
absent exactly when a rushed deploy is most likely.

## Auditing after the fact

```bash
relicta deploy audit           # report discrepancies
relicta deploy audit --strict  # exit non-zero on a severe one
```

Two discrepancies, and they are not equally serious:

- **An ungoverned deployment** — a version running with no release record behind it.
  This is severe and fails `--strict`. Until deployments were recorded it was not
  merely unreported but *undetectable*, because relicta had no way to know what was
  running.
- **An undeployed release** — governed, released, never reached production. Reported,
  not severe: a release awaiting rollout is a normal state on any given afternoon, and
  failing on it would train people to ignore the command — at which point it stops
  reporting the serious case too.

## What this fixes in the DORA report

Deployment frequency counted release records, which meant it counted tags. For a
project that tags weekly and deploys daily that is wrong in both directions. Change
failure rate could not be computed at all, because a failed deployment of a good
release was invisible — and that is precisely what the metric asks about.

With deployments recorded, both measure deployments. Without them the report falls
back to releases and **labels which it used** (`countedFrom`), because the same figure
otherwise means two different things and a reader cannot tell which.

Lead time for changes was worse than either. It averaged the release process's own
runtime — a few seconds or minutes — and compared that against DORA's 24-hour "elite"
threshold, so **every project scored elite for publishing quickly**, no matter how long
its changes had actually waited. A metric that always returns the best answer measures
nothing.

It now measures commit → production deployment, and says which interval it used:

| `measuredFrom` | meaning |
|---|---|
| `commit-to-production` | the DORA definition: earliest commit in the release → production deploy |
| `release-to-production` | commit dates unknown, so measured from the release. Reads low by exactly the time a change waited to be released |
| `unavailable` | nothing reached production in the period, so there is no arrival to measure to. Reported as unknown rather than as a number |

The interval starts at the **oldest** commit in a release, not the newest: a release
containing a three-week-old commit and one from this morning has a three-week lead
time, and reporting the morning's would describe the fastest change in the batch
instead of the one the metric asks about.

An `unavailable` lead time contributes no vote to the overall DORA rating. Scoring it
"low" would rate a project poorly for not reporting deployments, which punishes the
honest state rather than measuring delivery.
