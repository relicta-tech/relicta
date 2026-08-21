# ADR-016: No Data Beats Wrong Data

## Status

Accepted (2026-08-21)

## Date

2026-08-21

## Context

The observability integration was configured by nobody and connected at no point.
`observability.providers` had no production reader, `providers.NewPrometheusProvider` and
`monitor.NewHealthMonitor` had no production caller, `handlers.SetObservabilityService`
was never called, and no implementation of the `ObservabilityService` interface the
handlers declare existed anywhere in the tree.

Building it needed three answers that were not the implementer's to give:

1. When does monitoring start, and for how long?
2. What does `auto_record` write when a threshold is crossed?
3. What counts as unhealthy?

Underneath all three sat one question. Relicta's product is the release record — DORA
change failure rate is computed from it, and governance decisions are justified by it. So
what should the system record when it cannot see?

The code had already answered, everywhere, and answered wrongly:

- `performCheck` started from `Healthy: true` and only ever cleared it when a threshold
  was crossed. A provider that could not be reached left the error rate and latency at
  zero, no threshold was crossed, and the release was reported **healthy**.
- One existing test asserted this on purpose: *"expected healthy when metrics query fails
  (graceful degradation)"*. The degradation was to a claim.
- `runWatch` recorded **success** when the window expired, unconditionally — including
  when every check in that window had failed to reach the provider. A Prometheus that was
  down for half an hour produced a recorded successful deployment.

## Decision

**No data beats wrong data.** A release nothing could observe is *unmeasured*: not
healthy, not failed, and not recorded.

- `HealthStatus` carries `Measured` and `Unmeasured`. `Healthy` is only meaningful when
  `Measured` is true, and is only set when something was actually observed.
- Nothing is recorded for an unmeasured release. The window expiring records success only
  if at least one check in it measured something; a crossed threshold records failure only
  if the crossing was measured.
- **An empty alert list is not evidence of health.** A release can be failing badly with
  no alert rule written for it, so "nothing is firing" measures nothing on its own. A
  *firing* alert is evidence, and evidence of a problem.
- **A repository with no providers gets no service**, not an empty one. The four dashboard
  routes report `not_configured`, which is what distinguishes "nobody is watching" from
  "everything is healthy".
- **An unknown provider type is a startup error**, not a skipped entry. A dashboard that
  reports nothing wrong because it is asking nobody is the failure this ADR exists to
  prevent, and a misspelled type would produce exactly that.
- **The watch runs in the server, not in the CLI.** A window is minutes or hours; `relicta
  publish` exits in seconds. The server starts a watch when it hears a release published,
  so a watch that is shown is a watch that is running.

That answers the three questions. Monitoring starts when the server hears a release
published and runs for the configured window; `auto_record` writes only measured outcomes;
unhealthy means a measured threshold crossing or a firing alert, and nothing else.

## Consequences

Three existing tests asserted the old behavior and were rewritten, with the reasoning kept
where they are, because they are the clearest statement of what changed: a release with no
provider is no longer healthy, a failed query no longer degrades to healthy, and a window
that measured nothing no longer records success.

A dashboard can now show three states where it showed two — healthy, unhealthy, and
unmeasured — and the third is the honest answer far more often than anyone would like.

What this costs: a repository whose provider is flaky will accumulate releases with no
recorded outcome, and its change failure rate will be computed from fewer releases than it
shipped. That is the intended trade. A rate computed from ten measured releases is worth
more than one computed from fifty, forty of which were guesses.

## Amendment: what `auto_record` writes, and how a watch starts (2026-08-21)

Two gaps in the first pass, both of the kind this ADR is about.

**`auto_record` gated a recorder that was nil**, so it did nothing either way — a setting that
looks honored and is not, shipped in the same change that removed several of those. It now
writes a **measured** failure to the governance memory as an incident against the release,
typed by what was observed (a firing alert or a latency regression file differently) and
described in the words the thresholds used.

A healthy window still writes nothing. An incident is evidence that something happened; the
absence of one is already how a release that behaved is represented, and filling the incident
history with non-events would change what the existing records mean.

**The watch was wired to a signal that never arrives.** The server started one when it heard a
release published — but it hears only what its own process raises, and the dashboard publishes
nothing: `relicta publish` is a separate command, usually on another machine. The server now
reads the store the CLI writes to, picking up releases published inside the window, once at
startup and then on an interval. Only inside the window: watching a release from last week
would attribute today's metrics to it, which is the wrong-data failure one step along.

A watch also no longer seeds itself `Healthy: true`. A watch that has just opened has looked at
nothing, and the dashboard showed a green release from the instant monitoring began.

Verified end to end against a stub provider: a 42% error rate against a 5% threshold recorded

    availability | run-5a4f4c26 | high
    deployment health after release: error rate 42.00% exceeds threshold 5.00% (measured at …)

and the same release with the provider down recorded nothing at all, logging that the window
expired with nothing measured.
