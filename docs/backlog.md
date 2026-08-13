
## DONE: the two file-based release repositories are consolidated

One implementation survives — `adapters.FileReleaseRunRepository`, the writer that
round-trips — reached by the callers of the other interface through `releaseRepoBridge`.
`persistence.FileReleaseRepository` is constructed nowhere; the only remaining mention of it
in container.go is a comment explaining what it used to do. `app.ReleaseRepository()` returns
the bridge, so cancel, clean, rollback, bump and approve read the runs plan wrote.

The acceptance asked for a round trip proving commits, head SHA, base ref and changeset all
survive save/load, and that was the part still missing:
`TestARunRoundTripsWithEverythingGovernanceNeeds` now asserts all four together. Base ref had
no assertion anywhere, and it was the field the lossy loader filled from the branch — wrong
rather than empty, which is the failure mode that reads as data instead of absence. Verified
by dropping the stored value on load: the test fails naming the field.

## DONE: versioning.tag_prefix works for any prefix

Verified end to end rather than by reading the code, in a repository configured with
`tag_prefix: "release-"` and tagged `release-1.4.0`:

    Current version:  1.4.0
    Next version:     1.5.0 (minor)
    Total commits:    1

and, after `release-1.5.0` was created elsewhere, `relicta status` reports the run stale with
"planned against 1.4.0, but the repository is now at 1.5.0" — the staleness detector this
entry said could not see non-"v" releases. It now takes `cfg.Versioning.TagPrefix` at both
call sites, and `plan` passes `input.tagPrefixOrDefault()` rather than a hardcoded "v".

The domain gained the prefix-aware pair the entry asked for: `Tag.VersionWithPrefix` and
`TagList.VersionTagsWithPrefix`, which do the selecting and the stripping together, because
the prefix that selects a tag is the prefix that must be removed to read its version.
`FilterByPrefix` and `VersionTags` remain, each documenting that chaining them drops every
tag whose prefix `version.Parse` does not already understand — the trap this entry recorded.

The monorepo tag model decision it was waiting on was resolved as plain prefix stripping
rather than a component-qualified type. `app-v1.2.3` is what the documentation already
promised, and it parses.

## DONE: what init writes is decided, and every key mentioned has somewhere to look

DECISION: option 1. `relicta init` keeps a curated file — now 7 sections, governance having
been added because it is the capability the product exists for — and every message that names
a config section carries the YAML to add. Writing all 21 sections would be several hundred
lines of advanced settings at their defaults, which is its own usability problem.

The three cases the entry named now print a paste-able block instead of a key name:
environments (deploy), dashboard.api_keys (serve), repository_groups (group). `configHint`
holds them, following the shape errGovernanceDisabled established, and the group command's
help text shows the same YAML rather than only naming the section.

ENFORCED, since the failure is silent — the message looks helpful and only someone following
it discovers there is nothing to edit. `TestEveryConfigKeyMentionedHasSomewhereToLook` reads
the sections out of a file `WriteDefaultConfig` actually produces, so adding or removing one
changes the expectation automatically, and requires each hint to show its section nested and
name the file. `TestActionableMessagesDoNotJustNameAKey` fails on a one-line "configure X in
.relicta.yaml" string, which is what all three of these were.

Both hints were verified by pasting them into a config and running the command, which is how
two further defects surfaced:

1. The repository_groups YAML in the first version of the hint was wrong — `repositories`
   takes entries with their own name and path, and a list of strings fails to load. A hint
   that does not work is worse than a key name, because the reader now has two problems.

2. Following the corrected hint reached `relicta group plan`, which **panicked**:
   `NewCoordinator(nil, nil)` in internal/cli/multirepo.go, dereferenced immediately by
   planRepo. No implementation of the application's GitAdapter interface existed anywhere, so
   the command had never worked. Fixed with infrastructure/multirepo.GitAdapter, which opens
   each member repository at its configured path and honors the configured tag prefix; group
   plan now reports both members' current versions and change counts.

STILL OPEN, and the reason this entry is not entirely closed: the release executor is still
nil, so `relicta group release` refuses rather than releasing, and the NEXT column in a group
plan is blank because the next version comes from the executor's own Plan. Running a full
release inside another checkout — its config, its plugins, its approval state — is a feature
rather than a wiring fix. Coordinator.Execute now says so instead of panicking.

## DONE: the cgp_* MCP tools are wired, and the RELATED audit was partly wrong

The three tools work. `relicta mcp serve` supplies the governance service's own evaluator
(option 1), so an agent calling cgp_propose is governed by the same thresholds, policies and
freeze periods as a human running `relicta evaluate` — a fresh `evaluator.New()` would have
produced a second verdict for the same change with nothing saying which was authoritative.
Verified over stdio against a real repository: cgp_propose returns a GovernanceDecision with
risk score 0.6, `approval_required`, and the rationale "Agent-initiated change with elevated
risk requires human review".

THE RELATED LIST WAS PARTLY WRONG, and the correction matters more than the fix:

- `WithRiskCalculator` and `WithCache` are override seams over fields `NewServer` already
  defaults (`risk.NewCalculatorWithDefaults()`, `NewResourceCache()`). The claim that
  "WithRiskCalculator gates the fallback risk path in handleEvaluate, which therefore cannot
  run either" was false — that path runs. Deleting the option, which the entry invited, would
  have removed a working seam and broken four call sites. Confirmed by removing it and reading
  the compiler's answer, after a grep for the wrong field name (`riskCalculator` rather than
  `riskCalc`) had suggested it was dead.

- `WithCacheDisabled` is therefore meaningful: it turns off a cache that is on.

- `WithPolicyEngine` was already wired.

TWO WERE GENUINELY UNWIRED, and both are fixed:

- `WithReleaseRepository`. Five resources — release state, active runs, history, the run's
  recommendation — answered `{"status": "no release repository configured"}` while the
  container held the repository all along. To a caller that is indistinguishable from having
  no release, so an agent asking what is in progress was told nothing was. Now wired;
  `relicta://state` returns the real run (`"state": "versioned", "version": "0.1.0"`).

- `WithActorBudgets`. A configured `governance.actor_budget_path` gated the CLI and was
  ignored by the MCP surface — the surface agents actually use, and the reason per-actor
  budgets exist. `ResolveBudget`'s fallback is the restrictive default, so nothing was unsafe;
  what went missing was the operator's own policy, in either direction. The loader is now
  shared with the CLI gate rather than duplicated, and `mcp serve` refuses to start on an
  unreadable budget file instead of silently applying defaults the operator does not expect.

WHAT THE TESTS COVER, and why in two places: internal/mcp asserts the behavior each option
unlocks, and internal/cli asserts that mcp.go passes them. Only the second was missing, and
it is the half that was broken — an option nothing calls is the defect this file keeps
recording. The budget test needed rewriting for the same reason: its first version configured
a *restrictive* budget and asserted refusal, which passed even with the option turned into a
no-op, because the default refuses that operation too. It now configures a permissive budget
and asserts the operation is allowed, with the unconfigured server as a control.

## Policy conditions for data the evaluator does not yet carry

Five conditions written in the shipped example policies had no field behind them and were
removed rather than left looking active (PR #249). Two are now closed.

5. ~~Membership-of-nothing~~ — DONE. `is_empty` and `size` exist as operators, in the schema,
   the DSL (lexer, parser, compiler) and the engine, so `actor.teams is_empty true` compiles
   and fires. `NOT actor.is_member` can be written again. `size` came with it because a
   length comparison is the same question asked arithmetically, and it applies to
   `scope.files` and `actor.roles` too.

   Two things worth carrying forward. A field the context does not carry does not match, for
   every operator including this one — I assumed absence should read as empty and asserted it
   in a test before checking, and it is the wrong answer twice: a rule about unknown data
   should not fire in a governance tool, and one operator disagreeing with the other nine is
   a special case the policy author cannot see. It costs nothing here, because
   buildEvalContext always sets `actor.teams` and `actor.roles`, so they are present-and-empty
   rather than absent. And `is_empty` on a non-collection is an error rather than a silent
   false: a policy comparing the emptiness of a risk score is a mistake, and hiding it means
   the rule never fires and nobody learns why.

3. `change.scope_count` — PARTLY DONE, and deliberately renamed. `scope.areas` /
   `scope.areaCount` (distinct first path segments) and `scope.directories` /
   `scope.directoryCount` are derived from the paths the proposal already carries, so
   "touches more than two areas" is expressible today: with `size` and `>` it is the rule
   this entry wanted, and it distinguishes a three-file change across three areas from a
   three-file change inside one — the distinction fileCount cannot make, which is why the
   entry rejected approximating it that way.

   Named for what they measure rather than "domains" or "packages" on purpose. True package
   attribution needs the workspace layout, which the policy engine is not given: it receives
   a proposal and an analysis, and injecting a workspace resolver into it is a larger change
   than this. Calling a path prefix a "package" would be the same approximation the entry
   rejected, with better marketing. **Still open:** real package attribution for monorepos,
   reusing the workspace detection, if someone needs boundaries rather than breadth.

1 and 2 (`time.is_holiday`, `actor.level`) remain configuration surfaces and can wait for
someone to ask.

ALSO FIXED HERE: `relicta policy test` carried a changed-file count and not the paths, so no
path-conditioned rule could be exercised at all — neither the breadth fields nor
`scope.files contains "terraform/"`, which the shipped policies use. `--files` supplies them,
and the count is derived from them unless given explicitly, so a rule on `scope.fileCount`
and one on `scope.files` cannot disagree about the size of the same change. The rules a team
is most likely to get wrong were the ones they could not test, which is the same reason
`--trust-level` was added.

## DONE: CGP protocol records are durable, readable, and kept on purpose

The durability landed in PR #267 (FileProposalStore, wired through the WithStore option that
had never been called outside tests). The two things this entry left open are now closed.

`relicta cgp list` and `relicta cgp status <id>` exist, so the records are auditable by a
person rather than only over MCP. Verified against a proposal made through the cgp_propose
tool: list shows its state, risk score, actor and summary; status shows the full handshake.

RETENTION, decided rather than left as an accident: records are kept indefinitely. These are
the evidence that a change was governed, and an audit trail that expires cannot answer a
question asked after it expired. Removing them is a deliberate act — delete the files — and
`cgp list` reports how many are held, so growth is visible instead of silent. The command's
help says all of this, which is the part the entry asked for.

FOUND WHILE VERIFYING IT: both renderings composed "<kind>:<id>" unconditionally, while the
IDs agents send are already qualified — the convention pkg/cgp uses and what NewAgentActor
produces. So a proposal from "agent:probe" was listed as "agent:agent:probe". Nothing
downstream breaks on it, which is why it survived: it is wrong only on the screen, in the
command whose purpose is letting a person audit what an agent did. Running the command found
it; reading the formatting code had not.

STILL OPEN, unchanged: the read-side join. The protocol records and the release audit trail
are separate views of governance activity — correct modeling, since a CGP proposal is not a
release run (see the correction below) — but a reader asking "what governed this change?" has
to consult both. One command or endpoint reporting both is the useful thing left to build.

## Correction: CGP proposals are not release runs

Supersedes the recommendation in "CGP protocol proposals do not survive the session". That entry proposed, as its preferred option, routing CGP proposals through the ReleaseRun aggregate so there would be one governance record rather than two — by analogy to the duplicate release store consolidated in PR #247. I wrote that recommendation and it is wrong.

WHY IT IS WRONG: a cgpsdk.ChangeProposal carries an actor, a scope, an intent and metadata. That is all. It has no version proposal, no changeset, no bump kind, and no release state machine. CGP governs change in general; not every proposal is a release. Mapping every proposal onto a ReleaseRun would force the aggregate to hold data it has no meaning for, and would require inventing release fields for proposals that are not releases.

The #247 analogy only holds when two records describe the same thing. There, two stores held the same release run in different DTO shapes, and a run written by one came back from the other missing its changeset and HEAD. Here the two records describe different things, so having two is correct modeling rather than duplication.

WHAT WAS DONE INSTEAD (PR #267): a durable FileProposalStore under .relicta/cgp/{proposals,decisions,authorizations}, wired through the WithStore option that already existed and had never been called outside tests. The handshake now survives across processes — verified across four separate `relicta mcp serve` invocations, ending at state "authorized" with the decision and authorization both readable.

WHAT REMAINS, and is genuinely open:
1. Nothing prunes .relicta/cgp/. Proposals accumulate indefinitely. Decide a retention rule, or decide deliberately that governance records are never deleted — which is a defensible position for an audit trail and should then be stated rather than left as an accident.
2. The protocol records and the release audit trail are still separate views of governance activity. That is correct modeling, but a reader asking "what governed this change?" has to consult both. A read-side join — one command or endpoint that reports both — is the useful thing to build, and it does not require merging the aggregates.
3. There is no CLI surface for the protocol records at all: they are reachable only over MCP. `relicta cgp status <id>` would let a person audit what an agent did, which is the point of recording it.

Item 3 is the smallest and the most valuable of the three, because records nobody can read are only marginally better than records that do not exist.

---

## DONE: Release domain events are now published (was: the outcome tracker and release webhooks are unreachable)

Fixed. Kept here because the shape of the bug is worth remembering, and because two of the
things found on the way out are decisions rather than fixes.

WHAT IT WAS: the only production caller of `release.EventPublisher.Publish` was
`FileUnitOfWork.Commit`, and nothing constructed a unit of work outside `container_test.go`.
So the container assembled OutcomeTracker → WebhookPublisher → InMemoryEventPublisher,
logged it as initialized, and no release ever published an event.

THE SEAM CHOSEN: the repository's Save, decorated. Every use case already persists through
it — ten calls across plan, bump, notes, approve, publish and retry — so one seam cannot be
forgotten by the eleventh. Both paths to the aggregate are decorated: the release services,
and `app.ReleaseRepository()` through the bridge, which is what cancel, clean, rollback and
bump use. Decorating only the first left canceling silent, which is how that half was found.

WHAT WAS BLOCKING IT, once events flowed:

1. The tracker's per-run context cache is per-process, and this CLI is one process per
   command. `relicta cancel` raises a lone RunCanceledEvent, so there was no cached
   repository and the store rejected the record with "repository is required". The
   governance identity is now supplied by the container — the same value
   recordPublishOutcome uses, because a second derivation from the run's path would produce
   "local:checkout" where publish produces "acme/widget" and split one repository's history.

2. Terminal events carried no version, for the same reason: bump raised RunVersionedEvent in
   a process that has since exited. RunFailedEvent and RunCanceledEvent now carry it.

3. Two writers, one release. The tracker records during the publish use case and
   recordPublishOutcome records after it returns, both keyed on the run ID, and
   RecordRelease appended unconditionally — so every publish would have produced two
   records. RecordRelease now replaces by ID, which also makes retrying a publish
   idempotent. A replacement rebuilds the affected actors' metrics, because Accumulate
   keeps a running average that cannot be un-added.

DECISIONS TAKEN, both of which changed numbers:

- A cancellation is recorded as `OutcomeCanceled` and excluded from every rate computed
  over releases (`CountsAsRelease`). It was recorded as OutcomePartial, which IsNegative
  counts as a problem and Accumulate counts as a failed release — so declining to ship
  lowered the actor's reliability score and raised change failure rate. The governance gate
  working as intended must not read as a defect.

- Event names are the documented `release.*` rather than `run.*`. The webhook configuration
  documented `release.published` and offered `release.*` as its wildcard example, so a user
  who configured exactly what was described received nothing, and there was nothing to log
  because the filter simply matched no event. Both event-store deserializers accept the
  historical spelling (`CanonicalEventName`), so history written before the rename still
  loads.

- Webhook deliveries were sent with a bare `go` and nothing waited. In a process that exits
  when the command returns, delivery was a race against teardown that left no trace when it
  lost. The publisher now tracks in-flight sends and the container waits on shutdown.

STILL OPEN: nothing constructs a unit of work, so `FileUnitOfWork` and `App.UnitOfWork()`
remain unreachable. They are now redundant rather than load-bearing — the decorator does the
publishing — so the open question is whether to delete them or adopt them for the
transactional boundary they were written for.

## Hub cannot represent a canceled run

`relicta hub sync` skips records whose outcome is `canceled`, because Hub's event vocabulary
is release.planned / release.published / release.failed and none of them is true of a run that
never shipped. Sending one would mean choosing between two false statements, and
`eventTypeFor`'s default would have made it the worse one — every cancellation arriving as a
successful release, inflating the deployment frequency the local report is careful to exclude
it from.

Skipping keeps Hub's numbers agreeing with relicta's own about the same repository, at the
cost of Hub not knowing a release was deliberately stopped — which is governance-relevant:
"we decided not to ship this" is a decision an org-wide view should show.

Closing it needs a `release.canceled` event type in Hub, materialized into a row that is
recorded but excluded from every rate, mirroring `ReleaseOutcome.CountsAsRelease` on the CLI
side. Both halves of that predicate have to move together, or the two sides will disagree
about which records are releases — which reads as a reporting bug rather than a difference of
opinion.
