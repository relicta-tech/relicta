# ADR-013: One Store Behind a Backend, and SQLite Is the Shape of It

## Status

Accepted (2026-08-15)

## Date

2026-08-15

## Context

`.relicta.yaml` has offered a `persistence` section since before this ADR:

```yaml
persistence:
  backend: postgres          # file | postgres
  connection_string: "${DATABASE_URL}"
  pool_size: 10
  migration_mode: auto
```

Nothing read it. Verified against the shipped binary with a deliberately unreachable
connection string: `relicta plan` reported *"Release plan saved"* and wrote
`.relicta/releases/run-*.json`. A team that configured PostgreSQL for shared governance
state believed its release history was in the database; it was in each developer's
working copy.

Wiring the setting as it stood would not have fixed that, which is the reason this is
an ADR and not a patch. Three facts decide the shape of the answer.

**Relicta does not have "a file backend". It has five stores that each invented their
own persistence**, all under `.relicta/`:

| store | on disk | port |
|---|---|---|
| release runs | `releases/run-<id>.json`, `.state.json`, `latest` | `ports.ReleaseRunRepository` |
| governance memory | `governance/memory.json` | `cgpmemory.Store` |
| analytics | `governance/analytics/` | — |
| CGP protocol records | `protocol/` file store | — |
| attestations | `releases/<id>/attestation.intoto.jsonl` | — |

Only two have a port. A `backend:` setting can only mean something once they share one.

**The PostgreSQL support that exists was never going to be the system of record.** Its
schema is a single `events` table serving `ports.EventStore` — an interface with no
production caller on either implementation (both were deleted; see the amendment
below), and whose `LoadEvents` and `LoadAllEvents` are called by nothing at all. Fully wired, it would have added a write-only event log
beside the JSON runs.

**The strongest case for a database is a problem that exists locally, with one user.**
Every query in `ports.RunQuery` — `FindByState`, `FindActive`, `FindByPlanHash` — is
`os.ReadDir` followed by parsing every file, and `history`, `audit`, `report` and the
analytics service all walk that tree. `file_repository.go` carries a comment recording
a bug where a sibling `.json` artifact was returned as a run and rendered on the
dashboard as `{"id":"-decision"}` with a zero risk score. A schema makes that
unrepresentable.

The five stores also cannot be updated together. A crash between writing the run and
updating governance memory leaves them disagreeing, and for a tool whose product is
audit evidence, "the two records disagree" is the worst available failure.

## Decision

**One port set, three adapters, selected by the `persistence.backend` setting that
already exists.** `file` stays the default until parity is proven by the conformance
suite below; `sqlite` is the intended destination for local and CI use; `postgres` is
for teams sharing governance state.

**SQLite uses `modernc.org/sqlite`, the pure-Go driver.** Not a preference —
`.goreleaser.yaml` sets `CGO_ENABLED=0` so a single binary cross-compiles to every
target, which rules out `mattn/go-sqlite3` entirely. Anything requiring cgo would trade
the distribution story for the storage one.

**The system of record is the release run and the governance record — not an event
log.** Events remain a publication mechanism for webhooks and subscribers. This is the
question that blocked the work for as long as it was unasked: wiring `EventStore`
without answering it would have produced a third copy of the truth, and three copies
that can disagree are worse than one that cannot.

**One conformance suite runs against all three adapters.** With three implementations
of one port, a shared suite executed three times is the only thing that keeps them
honest; anything else lets them drift until a backend switch changes behaviour nobody
predicted.

**Migration is explicit, not automatic.** `relicta db import` reads an existing
`.relicta/` tree into the configured backend. Relicta does not silently move an
operator's audit trail because they edited a config key, and it does not delete the
JSON afterwards — the tree stays as an export until the operator removes it.

## Consequences

**The setting stops lying.** `persistence.backend` selects something, and a value the
build cannot honor is refused at load rather than ignored.

**Queries become queries.** `FindByState` stops meaning "load every run and filter in
Go". This matters most for `report` and the dashboard, which are the paths that read
the whole history.

> **Amended 2026-08-19, after measuring it.** True, but by less than this implies:
> `FindByState` gains ~3–5×, not the order of magnitude `List` gains, because the index
> removes the scan and not the parse. See "What the measurement changed" below.

**Atomicity becomes available.** A run and the governance record it produces can be
written in one transaction under sqlite and postgres. The file backend cannot offer
that, which is a reason to move rather than a reason not to have the file backend —
it is the compatibility path, and it keeps the guarantees it already had.

**Three adapters is more surface.** Accepted, because the alternative — one backend,
chosen for everyone — either denies teams shared state or forces a database on someone
releasing from a laptop. The conformance suite is the cost of the choice, and it is
also the thing that makes the choice safe.

**`file` staying the default means the payoff is deferred.** Deliberately. Flipping the
default migrates every existing user's audit trail, and that is a decision to make on
evidence — a conformance suite passing on all three adapters, and an importer with a
round trip test — rather than on the day the code lands.

**The `events` table is not the schema.** It survives for the event log; the runs and
governance tables are new. Migration `001_create_events` stays where it is.

## What the measurement changed (2026-08-19)

The Context argues that a database helps a single local user because every query walks
the tree and parses every file. That was asserted three times in this document and its
commit messages before anyone measured it. It is now measured, by
`BenchmarkReleaseRunQueries` in the sqlite package, at `-benchtime 10x -count 2`:

| runs | `List` (file → sqlite) | `FindByState` (file → sqlite) |
|---|---|---|
| 100 | 0.74ms → 0.13ms (~6×) | 4.4–5.8ms → 1.3–1.5ms (~3.5×) |
| 500 | 4.2ms → 0.70ms (~6×) | 24.5–26.8ms → 9.6–10.2ms (~2.6×) |
| 2000 | 15.8ms → 1.6ms (~10×) | 534–797ms → 132–135ms (~4–6×) |

**The direction holds. The shape of the win is not what the Context implies.**

`List` improves by roughly an order of magnitude, and for the stated reason: it answers
from directory entries or an index without materializing anything.

`FindByState` improves by three to five times — real, but visibly less. The index removes
the *scan*; it does not remove the *parse*. Both backends still deserialize every
matching run, so that shared work sets a floor no schema can lift. If this query ever
needs to be dramatically faster, the change is to the port rather than any adapter:
filtering runs does not require materializing them, so `FindByState` could return
identities, or select the projected columns and load documents only for what the caller
keeps. That moves all four implementations together, which is what the conformance suite
exists to make possible.

### A correction to this correction

The first version of this section, written earlier the same day, claimed `FindByState`
was *not faster at all* and at small histories was *slower*. That was measured with a
single untimed call per backend against cold caches — one sample, no warmup — and it was
wrong. The benchmark disagrees consistently across sizes and repeats.

Both mistakes have the same cause and it is worth naming, because this document is where
someone will come looking for justification: a number asserted from reasoning is a guess,
and a number from one unrepeated sample is barely better. The benchmark is committed for
that reason, and it is a benchmark rather than a test because it measures rather than
asserts — a threshold that fails on a busy machine teaches people to ignore it.

Reproduce with:

    go test ./internal/infrastructure/persistence/sqlite/ \
      -bench BenchmarkReleaseRunQueries -run '^$' -benchtime 10x -count 2

### What this does not change

The case for this decision was never mainly speed, and the parts that carry it are
untouched: one schema instead of five ad-hoc encodings, a run and its governance record
writable in one transaction, and a store a team can share. Those remain the reasons.

## Amendment: the event store was deleted rather than wired

This ADR settled that the event log is not the system of record but left it in the tree,
unwired. That was the wrong resting place: an interface, two implementations and a
factory that nothing constructed read as a feature to anyone opening the package.

ADR-014 then gave the append-only-record job to the audit chain, which is appended as
decisions happen, verified on read, and hashed into attestations. With that owner named,
the event store had no remaining claim, and all 760 lines were removed.

`migrations/001_create_events` stays. It has shipped, and renumbering an ordered
migration sequence breaks every operator who already ran it — a cost far above an unused
empty table.
