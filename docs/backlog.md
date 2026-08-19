
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

PARTLY CLOSED SINCE: the executor is a planner now, so the NEXT column is filled —
`infrastructure/multirepo.Planner` reads each member's commits since its last version tag and
applies the same rule the single-repository path applies. Deliberately the same rule: a group
that planned by different arithmetic would produce a different version for the same commits
depending on which command ran, and neither answer would be wrong on its own terms. (I wrote a
test asserting chores produce no version, which is not this tool's rule — DetectReleaseType
treats any commit as at least a patch — and corrected the test rather than the behavior.)

SINCE: the refusal now reports the group instead of the implementation. `relicta group
release` checks every member's stored run first and prints what each one needs — no plan, not
approved, already published, canceled, failed — with the repository named, refusing the group
as a whole. A coordinated release that publishes two of four repositories and then stops
leaves the group in a state nobody chose, so readiness is checked before anything runs.

That check reads stored runs and nothing else: no container, no git service, no config. Each
of those resolves against the process working directory somewhere, and a component that
silently answered for the invoking repository instead of the member would be worse than the
refusal it replaced.

DONE: a group release publishes its members. Both blockers are cleared.

The decision, confirmed rather than assumed: a group release **publishes what was already
approved and approves nothing itself**. Otherwise adding a repository to a group would be a
way around its own policy — the release would run under an approval nobody gave. Readiness
reports which members need approval; the executor refuses again at publish time, because the
two checks are separated by however long the earlier members took.

The risk, removed: every path in a container derives from its git service, and that service
took no path, so it opened the process working directory. Pointing release services at a
member's root while the git adapter still pointed at the invoking repository would have
published the *invoking* repository's tags — silently, and unrecoverably. `NewForRepo` scopes
the whole container; `blast.WithRepoPath(".")` is scoped with it, and the third site was only
a fallback for when the git adapter fails, so it followed. Containers built with `New` behave
exactly as before.

Verified with three repositories — a caller and two members, both approved, `group release`
run from the caller:

    gm-caller: v1.0.0            (unchanged — no tag from someone else's release)
    gm-a:      v1.0.0 v1.1.0
    gm-b:      v1.0.0 v1.1.0

and with one member left unapproved, where nothing published at all and no tag moved. A test
asserts the scoping directly, and reverting it reproduces the misrouting: the container
resolves the caller instead of the member.

Two smaller decisions inside it. The publish is not forced — `relicta publish` forces because
an operator has just been shown the plan and confirmed it, while here nobody is watching this
particular member, so a repository whose HEAD moved since approval stops rather than shipping
something unreviewed. And the audit trail records the actor as the group release rather than
"cli", since the run's own approval already records who authorized it.

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

DONE: the read-side join. `relicta audit` reports both records as one timeline, joined on the
version an ExecutionAuthorization granted — so an agent's proposal appears on the same line as
the release that carried it out. The records stay separate, which remains the right modeling;
this is a read-side view over them, not a merge.

Either half stands alone when it has no counterpart, and the output says what a dash means: a
release with no proposal was driven directly, and a proposal with no release was never
authorized or has not shipped. `--version` accepts a tag name, since that is where a reader
copies one from.

One thing worth keeping: the first version keyed its "already paired" set by version, so
pairing one release suppressed every other release carrying that version — and a repository
routinely has several, a canceled 0.1.0 and the published 0.1.0 that followed it being the
ordinary case. The cancellation vanished from the timeline. Running the command on a
repository that had both is what found it; reading the loop had not. It claims by release ID
now, and only a release that counts as one is a candidate for pairing — an authorization
paired with a run that never happened would misstate what it led to.

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

## DONE: Hub represents a canceled run

`release.canceled` existed in Hub's event vocabulary and was materialized nowhere, so the CLI
skipped canceled records rather than send an event that would vanish — or worse, be reported
as a release, since eventTypeFor's default is release.published.

Both halves moved together, which was the condition this entry set. Hub materializes the
event into a row with state and outcome "canceled" and excludes it from every rate through
`Release.CountsAsRelease`, the mirror of `ReleaseOutcome.CountsAsRelease` here; `hub sync`
sends it. Verified end to end: a repository with one canceled and one published run shows
both at /releases while /analytics/dora counts one deployment, and the CLI reports the same
split locally.

The reason the halves could not move separately is worth keeping: a Hub that counted
cancellations would disagree with relicta about the same repository, and a disagreement
between two views of one governance record reads as a reporting bug rather than a difference
of opinion.

## The observability integration is not wired at all

Configuring `observability.providers` does nothing. The subsystem is complete in parts and
connected at none of them:

- `ObservabilityProviderConfig` — name, type, endpoint, basic auth, bearer token — is
  consumed by **no code anywhere**. Nothing reads `cfg.Observability.Providers`.
- `providers.NewPrometheusProvider` has no production caller, so its auth options
  (`WithBasicAuth`, `WithBearerToken`) are unreachable too.
- `monitor.NewHealthMonitor` has no production caller, so "deployment health monitoring after
  releases" and `auto_record` never run.
- `handlers.SetObservabilityService` has no production caller, so `observabilitySvc` is always
  nil and the four `/api/v1/observability/*` routes answer from that branch every time.
- No implementation of the `ObservabilityService` interface exists in the tree. The interface
  is declared by the handlers and satisfied by nothing.

FIXED HERE, because it is wrong regardless of the feature: the three read routes returned a
bare empty collection with 200, so a caller could not tell "no providers configured" from
"everything healthy" — and an empty health list reads as the second. They now report
`status: not_configured` alongside the empty collection. The webhook route already answered
503 honestly.

WHAT BUILDING IT NEEDS, and why it was not done in the same pass: a provider factory and
registry from config, an `ObservabilityService` implementation, and the health monitor wired
to run after a deployment. The last carries decisions that are not the implementer's to make:

1. When does monitoring start and for how long — the release, the deployment record, or a
   configured window?
2. What does `auto_record` write when a threshold is crossed? A deployment outcome of failed
   is the obvious candidate, and it feeds change failure rate, so getting it wrong misreports
   DORA rather than merely missing a feature.
3. What counts as unhealthy — error rate, latency, a firing alert, or a combination — and does
   an unhealthy window roll the release back or only record it?

Until those are answered, the honest state is the one now shipped: the routes say the
subsystem is not configured rather than implying health.

## Sign git tags, or stop offering the setting

`versioning.git_sign: true` produced an ordinary unsigned tag and said nothing. The chain was
dead at every link: `versioning.git_sign` was read by no code at all, `TagOptions.Sign` is
declared and never read by `CreateTag`, and `ServiceConfig.GPGSign` is written only by
`WithGPGSign`, which nothing calls. Confirmed against the shipped binary — with the setting
on, publish created v0.1.0 and `git tag -v` answered "error: no signature found".

FIXED HERE by refusing: with `git_sign` set, tagging fails and names the setting, rather than
producing an unsigned tag under a policy that asks for a signed one. For most settings
silently doing nothing is a bug; for this one it is a false integrity claim, because the
signature is the evidence. It affects only someone who deliberately set it — the default is
false and the wizard writes false — which is exactly who was being misled.

STILL OPEN: actually signing. go-git's `CreateTag` accepts a `SignKey *openpgp.Entity`, so the
mechanism is a key-loading problem rather than a git one, and the attestation config already
has the shape for it (`signing_mode`, `key_path`). What it needs:

1. Where the key comes from — a keyring path, a GPG agent, or the same key attestation signing
   uses. Sharing one key for both is the tidier story and should be a deliberate choice, not
   an accident of whichever was implemented first.
2. Passphrase handling in a non-interactive release. A prompt is impossible in CI, and an
   environment variable holding a passphrase is a decision with its own consequences.
3. Whether verification belongs in `relicta verify` alongside the attestation check, so a
   signed tag and a signed attestation are reported together rather than by two commands.

Until then the refusal is the honest state: a release either has the signature its policy
asked for, or it does not happen.

## Config fields that nothing reads

A sweep of every `mapstructure` field against its readers found 68 with no consumer outside
the config package and the wizard. Most are plugin settings — a GitHub or Slack plugin reads
its own config through the plugin interface, so "unread here" is correct for those.

The rest are settings the CLI is expected to honor and does not. Now fixed: `versioning.git_sign`,
`workflow.require_clean_working_tree` (#302), `workflow.auto_commit_changelog` with
`changelog_commit_message` (#303), the changelog rendering block (#304, #307 — `format`,
`exclude`, `categories`, `include_commit_hash`, `include_author`, `include_date`,
`link_commits`, `group_by`, `link_issues`/`issue_url`), `workflow.require_up_to_date` and the
pre/post release hooks (#308), and `versioning.bump_from` (#306).

These remain:

- `versioning.prerelease_suffix` — **resolved: it warns.** Two wirings were tried and rejected.
  Read literally, every bump becomes a prerelease and a project can never cut a stable release
  through `bump` again (measured: `1.3.0-beta.1` → `1.3.0-beta.2`, never `1.3.0`). Wired as the
  default for a bare `--prerelease`, it needs pflag's `NoOptDefVal`, which silently stops
  `--prerelease beta` and `-p beta` from binding their value — an unread setting would have
  become one that overrides an explicit flag, which is worse than the defect. The capability is
  already reachable through `--prerelease`, `--channel` and `promote`, so config validation now
  warns that the setting has no effect and names those. The package-override copy in the monorepo
  block is still unread, with the rest of that block.
- `changelog.template` **and** `changelog.format` — one piece of work, not two. There is a
  template engine in `internal/infrastructure/template` (embedded `changelog.tmpl`, custom-dir
  loader, func map, execution timeout) whose only non-test importer is a benchmark. The blocker
  is a public contract, not wiring: **which data model does a user's template receive?** Its
  `ChangelogData` is built from `git.CategorizedChanges`, a different shape from the
  `ChangelogEntry` that `group_by`, `categories` and `exclude` now shape. Choosing
  `CategorizedChanges` silently stops those three applying to template users; choosing
  `ChangelogEntry` stops the three shipped `.tmpl` files parsing. Two adjacent facts: `format:
  custom` is accepted by `validate.go` but unknown to `ChangelogFormat.IsValid()`, so it is
  silently downgraded today; and `RenderOptions.Format` is translated from config and never
  branched on, so all formats currently render identically.
- `versioning.build_metadata` — unread. (The monorepo block has its own entry below.)

## Monorepo support is implemented and unreachable

The largest instance of this defect class in the tree, and the one that needs a product decision
rather than wiring.

**Every field of the `monorepo:` section has zero production readers** — `enabled`, `strategy`,
`package_paths`, `exclude_paths`, `package_overrides`, `version_files`, `release_groups` — and
`internal/domain/monorepo` plus `internal/application/monorepo` hold ~3,000 lines of implemented,
tested code (aggregate, package release, analyzer, orchestrator, version writers) that nothing in
the release path calls.

Verified against the built binary. A repository with `enabled: true`, `strategy: independent`,
`package_paths: ["packages/*"]` and two packages at 1.0.0 and 2.0.0:

    Current version:  0.0.0
    Next version:     0.1.0

One repository-wide version, and neither `package.json` touched.

The distinction that matters: **package analysis works, package versioning does not.** `relicta
blast` found both packages and the one affected — but it reads `blast_radius`, not `monorepo`. So
a monorepo user is not wrong that relicta understands their layout; they are wrong about which
part of it versions their packages.

Config validation now warns when the section is enabled, so it stops silently doing nothing. That
is the honest interim state, not the fix.

**The decision needed: is per-package versioning a product commitment?** If yes, the work is
wiring an existing subsystem rather than writing one — the orchestrator and version writers exist
and are tested — and it needs answers for how a monorepo release interacts with the single
`ReleaseRun` aggregate, with tags (one per package? `app-v1.2.3`?), with the changelog, and with
the governance record, which is currently one decision per release rather than per package. If
no, ~3,000 lines and a config section should be deleted rather than left looking like a feature.
Either answer is better than the present one. (`versioning.version_files` at the top level
  *is* honored — `relicta bump` writes every configured manifest; the earlier note here was
  wrong, corrected after checking against a real package.json.)
- `git.ssh_key_path` / `ssh_key_password` — the git service has WithAuthToken and
  WithAuthUsername with no callers either, so authenticated push relies entirely on ambient
  credentials.
- `attestation.rekor_url` / `fulcio_url` — keyless signing endpoints, unused because keyless
  signing is not implemented.
- `telemetry` — a whole section with no reader. Belongs with the observability entry above,
  which carries the decisions it needs.

## Gates apply to `publish` but not to `release`

`relicta release`, the one-shot workflow, has its own publish step in `runReleasePublish` that
calls the use case directly. It applies **none** of the gates: not `require_clean_working_tree`
(#302), not `require_up_to_date` (#308), not the actor budget, and it runs neither release
hook. Every gate added to `publish` has to be added twice or moved somewhere both paths reach,
and right now the one-shot path is the unguarded one — which is the path CI is most likely to
use.

The same shape once more: `internal/mcp/adapters.go` ignores `bump_from` at three `AnalyzeInput`
sites, and two of them do not set `TagPrefix` either. The adapter holds no config; closing it is
a container-wiring decision rather than a patch.

## DONE: three backends behind one contract (ADR-013)

The decision that blocked this is taken and written down: **the system of record is the release
run and the governance record, not an event log.** Wiring the event store without answering that
would have produced a third copy of the truth beside the run JSON and the CGP audit chain.

Landed: ADR-013 and the conformance suite (#310), the SQLite adapter (#311), the PostgreSQL run
repository (#312). All three implementations pass one shared suite — 45 cases across the three —
and `file` remains the default until parity is proven, per the ADR.

The suite earned itself on the first day. It caught the file and sqlite adapters returning
different orderings for the same repository (`[older, newer]` versus `[newer, older]`) before
either could be selected, and correcting that exposed two more mistakes: the port documented
"creation time" when no implementation had ever done that, and my own first ordering case pinned
when a *row was written* rather than when the *run changed* — which the sqlite adapter was right
to fail. One repository with two histories depending on a config key is exactly the drift three
adapters produce unsupervised.

`persistence.backend` now selects (#324), and #325 fixed what selection exposed: `relicta group
release` read every member through a file repository, so a member whose approved run sat in SQLite
was reported as "no release has been planned" — a confident wrong answer, not a gap. Underneath it,
`config.LoadFromDirectory(dir)` did not load from `dir`: the process working directory was searched
first and the ancestor walk started there too, so all three callers that name another repository
were answered about the invoking one.

Still open on the storage work:
- `relicta db import`, to read an existing `.relicta/` tree into a database. ADR-013 requires
  migration to be explicit and non-destructive: relicta must not move an operator's audit trail
  because they edited a config key, and it does not delete the JSON afterwards.
- The other four stores. Release runs have a port and now three adapters; governance memory,
  analytics, the CGP protocol store and attestations each still invent their own persistence
  under `.relicta/`, so a backend setting cannot yet mean "all of it". Governance memory is the
  one that matters next, because a run and the record it produces should be writable in one
  transaction — which is the atomicity argument the ADR makes and the file backend cannot offer.
- Flipping the default, which the ADR says happens on evidence: the conformance suite passing on
  all three adapters (done) plus an importer with a round trip test (not done).

## DONE: incident recording agrees across all four backends

Found while implementing the PostgreSQL governance memory store, which declined to reproduce two
file-store behaviors. Both were defects rather than choices, and both corrupted the numbers
reputation and the autonomy budget read:

1. `RecordIncident` appended unconditionally while `RecordRelease` upserted by ID — an asymmetry
   inside one implementation. A retried incident left two rows and counted twice.
2. An incident counted only `if metrics, exists := s.actors[actorID]; exists`, so one recorded
   before that actor's first release never counted — even after later releases made them known.
   The count depended on arrival order rather than on what happened.

Fixed as one change across all four implementations, driven by two new contract cases, with
`UpsertIncidentRecord` added beside `UpsertReleaseRecord` so the rule has one definition.

Worth recording how the second case reached its final shape. It first asserted that an incident
*alone* should conjure an actor — that an actor known only by an incident gets metrics. The
PostgreSQL store's own test asserted the opposite and caught it, and the reference agrees with
the test: an actor nobody has seen release anything is unknown rather than clean, and
`GetActorMetrics` erroring is how callers tell those apart. The case now pins arrival-order
independence and nothing more. It was my reasoning against the code's, which is the rule this
suite exists to enforce and the one I broke writing it.

The corrected case also exposed a latent panic in the SQLite store: `releases[len(releases)-1]`
indexed unconditionally to read the actor's kind, reachable as soon as an incident could precede
a release.

## The event store is still unreachable, and now needs a smaller decision

ADR-013 answered the question this entry used to be blocked on: **the event store is not the
system of record.** The release run and the governance record are. So what remains is narrower
than it was.

Nothing constructs an event store of either kind:

- `persistence.NewEventStore` — the factory that selects postgres by config: no caller.
- `adapters.NewFileEventStore` — the other implementation: no caller either.
- `LoadEvents` / `LoadAllEvents` — no caller outside the implementations, so nothing reads an
  event stream back.

`EventPublishingConfig.EventStore` is therefore always nil and the event-sourcing layer never
runs. Both implementations satisfy `ports.EventStore` and the factory compiles, so it is not far
from working — and that is the trap, because wiring it today adds writes nothing reads.

**The decision left is whether the event log is worth keeping at all.** Its plausible value is
an append-only record for auditors, distinct from the run aggregate's current state: what
happened in order, rather than where the release ended up. If that is worth having, wire it and
give something a reason to read it — `relicta audit` is the obvious candidate. If it is not, the
factory, both implementations and the `events` table are dead weight that reads as a feature,
and deleting them is the honest outcome. Either answer is better than the present one.

Fixed alongside this entry: the `db` command is now registered (it was documented in CLAUDE.md
and absent from the binary), and `persistence` has defaults (`DefaultPersistenceConfig` existed
and was called from nowhere, so the backend was "" and the pool size 0).

The method is worth keeping: extract every field with a `mapstructure` tag, count references
to the Go field name outside `internal/config`, `internal/ui/wizard` and
`internal/cli/templates`, and read what is left. A setting that does nothing is worse than a
missing one, because the user believes it is in force — `require_clean_working_tree` defaults
to **true**, so every user had a safety gate switched on that never ran.
