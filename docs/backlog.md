
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
   attribution now comes from workspace detection in the governance service. Changed paths
   map to the most specific detected package and are exposed as `scope.packages` and
   `scope.packageCount`. Those fields are omitted when package ownership cannot be established;
   path-derived breadth remains available separately.

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

The protocol service now fails closed if it cannot persist either the incoming proposal or its
decision. Previously it logged those write failures and still returned a governance decision,
which let an MCP caller proceed with an outcome Relicta could not later audit. If saving the
decision fails after the proposal is written, status exposes that incomplete state and the
caller receives an error rather than a successful verdict. Status also distinguishes a missing
decision or authorization from unreadable/corrupt records, and refuses to report an authorization
whose decision record is absent.

The three follow-up items from the original entry are now resolved elsewhere in this backlog:
retention is explicitly indefinite, `relicta cgp list` / `status` make protocol records readable,
and `relicta audit` joins protocol activity with release outcomes. The remaining storage gap is
backend selection for these records, tracked under the storage work below.

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
  lost. The publisher waits for configured retries in its legacy in-memory mode and adds a
  stable `X-Relicta-Delivery-ID` for receiver deduplication. The production container now uses
  the file-backed queue: events are persisted before forwarding to local governance/analytics,
  and a background worker resumes retries after restart. Queue files are atomically written
  with owner-only permissions; malformed records and unsafe IDs fail closed. Delivery is
  at-least-once, so receivers must deduplicate by delivery ID. `relicta webhook queue` reports
  pending/in-flight/dead-lettered records without exposing endpoint URLs, and `relicta webhook
  replay <id>` retries a dead-lettered record only when that webhook is configured and enabled.
  A file lock allows one process per local repository to recover and deliver queue items at a
  time; other processes can enqueue and inspect without taking over in-flight work. The file
  queue is intended for local filesystems, not shared network filesystems.

The worker starts only after application initialization succeeds. Container shutdown stops the
worker and drains queued work for up to five seconds; anything still pending remains on disk.
Retry failures and dead-letter transitions are logged by webhook and event.

FOLLOW-UP: `FileUnitOfWorkFactory` is still constructed by the container and exposed through
`App.UnitOfWork()`; a container test exercises creation and rollback. It has no production
callers, and its file-backed commit applies writes one at a time, so it does not provide an
atomic transaction. Do not describe it as protecting release operations. Either remove this
unused API and its implementation or design a backend-specific transactional contract before
introducing production callers. The separate cross-record consistency issue remains the
best-effort event pipeline described under storage above.

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

## SUPERSEDED by ADR-016: the observability integration is not wired at all

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

## DONE: sign git tags when configured

`versioning.git_sign: true` asks the installed Git client to create a signed annotated tag using
the repository/user signing configuration (`user.signingkey`, `gpg.format`, agent, and signing
program). This supports Git's configured OpenPGP and SSH signing backends without loading
private key material into Relicta. A missing key, unavailable signing program, or failed signer
returns an error; Relicta does not fall back to an unsigned tag. The command is context-bounded
so a blocked signer cannot hang publish indefinitely.

`relicta verify --tag <tag>` can check the Git signature alongside the attestation. JSON includes
`tag_signature.valid`, the peeled commit, and whether it matches the attested commit; a failed
signature or binding check exits non-zero. The attestation now hashes the post-artifact commit
that publish tags, while retaining the planned HEAD as the release's analyzed range. Git's local
trust configuration determines which signing identities are trusted, so CI must provision that
trust store deliberately.

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
- `changelog.format` now selects built-in Keep a Changelog, conventional, and simple layouts;
  `changelog.format: custom` renders a Go `text/template` whose input is `ChangelogEntry`, so
  `group_by`, `categories`, and `exclude` apply before the template runs. Template parse and
  execution errors stop notes generation with the file and phase identified. **Open:** richer
  format-specific section vocabularies and templates for whole changelog files; the current
  template contract renders one release entry's sections, while Relicta owns its version heading.
- `versioning.build_metadata` — DONE. It supplies build metadata to normal, forced and
  independent monorepo bumps; `--build` overrides the configured value, and package tags are
  recomputed from the metadata-bearing versions.
- `versioning.version_files` at the top level *is* honored — `relicta bump` writes every
  configured manifest. The earlier note here was wrong, corrected after checking against a real
  package.json.
- `git.ssh_key_path` / `ssh_key_password` — the git service has WithAuthToken and
  WithAuthUsername with no callers either, so authenticated push relies entirely on ambient
  credentials.
- `attestation.rekor_url` / `fulcio_url` — keyless signing endpoints, unused because keyless
  signing is not implemented.
- `telemetry` — a whole section with no reader. Belongs with the observability entry above,
  which carries the decisions it needs.

## Two packages nothing imports — four settled, two to go

**`cgp/autoapproval` is deleted (ADR-017).** It was not merely unreached: it was a second
implementation of the decision the evaluator already makes, with its own risk bands, its own
policy type, its own exemptions and actor rules, and its own YAML config that no configuration
file could reach. The wired path covers every one of those — the ADR maps them feature by
feature — and relicta already has a policy language in the DSL loaded from `policy_dir`. Wiring
the second one would have put two policy languages in one governance tool, and two things able
to auto-approve a release with no defined precedence.

**`cgp/ciapproval` is deleted too**, but only after the one thing it uniquely did got a proper
owner: `workflow.allowed_branches` was validated, defaulted, and enforced by nothing, so a
repository restricting releases to `main` could release from any branch. `enforceAllowedBranch`
now sits with the other pre-publish gates. That setting had hidden from the config sweep because
the MCP server assigns to the field, and a write counts as a use — the second time these two
sweeps have each caught something the other could not.

**`cgp/approval` is deleted** on the same test: requests and processing are the ApproveRelease
use case, emergency bypass is `--override-governance --reason` (which refuses without one), and
the rationale it captured is `ApproveReleaseInput.Justification`, already carried into the run
and read by the attestation generator.

**One control it had is not owned, and is not being added here: requiring a rationale on every
approval.** Only an override produces one today, so an ordinary approval records who and when
and not why. That is a real gap for a tool whose product is the audit trail — and no config key
promises it, so building it would be inventing a feature rather than keeping a promise. That
distinction is what separated this deletion from `allowed_branches`, which had to be enforced
before its only implementation could go. If wanted: a `governance.require_approval_rationale`
setting, a prompt or `--reason` on the ordinary path, and the field the use case already has.

**`cgp/policy/library` is the first of the six that was not a duplicate.** It ships SOC 2,
separation of duties, multi-team approval, audit trail, hotfix fast track and more, built as
`*policy.Policy` values for the engine that *is* wired — and nothing offered them: no command,
no config key, no importer. `relicta policy templates` now lists the catalogue and `--show <id>`
prints what one contains.

**And it is less than it first appears, which is worth recording.** Policy files are written in
the DSL, and `internal/cgp/policy/dsl` is parse-only — a lexer, a parser, a compiler, and no
writer. A template is a Go value, so there is no way to emit it as a `.policy` file the loader
would read. The first draft of the command told the reader to "save it under your policy_dir to
put it into effect", which was false: `relicta policy validate` does not even see a `.json` file
there. That is the defect this whole stretch has been removing, and it was nearly shipped by the
work removing it — caught by round-tripping the output through the validator rather than trusting
that it would fit.

**Installing a template needs a DSL serializer**, which carries a design question — whether the
DSL round-trips — and is left open rather than guessed at.

The test for the remaining two: **does something already own this job?** Where the
answer is yes, the unreached copy goes. Where it is no, the question is whether the feature is a
commitment — which is how monorepo versioning was decided in ADR-015.

## Six packages nothing imports — 3,606 lines, and each needs the same decision

The config sweep is closed, so the same technique went to the next dimension: exported symbols
resolved through type information, aggregated per package. The question that produced something
was not "which symbol is unused" — that reports thousands and means nothing — but **which
packages have no external user at all**. That is the shape the event store had.

Eleven packages qualify. Five are explained: `internal/benchmark` and the two `conformance`
packages exist for tests, `internal/version` is set through ldflags, and
`internal/infrastructure/template` is already an entry here, blocked on which data model a user's
template receives.

Six are not explained, and every one is a feature:

| package | production | tests | what it is |
|---|---|---|---|
| ~~`cgp/autoapproval`~~ | ~~1,107~~ | ~~1,091~~ | **deleted, ADR-017** — a second implementation of a decision the evaluator already makes |
| `cgp/ciapproval` | 873 | 1,277 | approval for non-interactive CI environments |
| `application/supplychain` | 552 | 750 | dependency change analysis and governance |
| ~~`cgp/approval`~~ | ~~493~~ | ~~579~~ | **deleted, ADR-017 amendment** — owned by the ApproveRelease use case, `--override-governance --reason`, and the justification the run already carries |
| `infrastructure/hubsync` | 474 | 319 | ships governance events to a Relicta Hub |
| `cgp/policy/library` | 107 | 444 | **now reachable** — `relicta policy templates` lists them; installing one needs a DSL writer |

3,606 lines of production code and 4,460 of tests, imported by nothing.

**`cgp/autoapproval` is the one to look at first**, because it is not merely unreached — it is a
second implementation. The evaluator already auto-approves, with one number:
`riskAssessment.Score < config.AutoApproveThreshold`. This package has a full policy model, and
its own YAML-tagged config that no config file can reach: nothing in `internal/config` mentions
it. So a repository cannot turn it on even by accident, and the simpler path silently wins.

**Each needs the decision the event store needed and the monorepo subsystem needed: wire it, or
delete it.** That is a product question, not a wiring one — is richer auto-approval a
commitment? does the Hub sync ship? — and the two precedents in this repository went opposite
ways for good reasons: the event store was deleted because the audit chain already owned its
job, and monorepo versioning was wired because nothing else did.

**On the method, and its false starts.** Both dimensions of this sweep needed two attempts
before their numbers meant anything. The config sweep first matched by field name, and a name
shared with another struct read as "used" — that hid nine settings. This one first keyed method
declarations as `Type.Method` while marking uses as `Method`, and reported all 3,034 exported
symbols as dead; then, corrected, still reported 2,322, because "unused symbol" is the wrong
question. Per-package is the right one, and it gives eleven.

A measurement that produces a big number is usually measuring itself.

## DONE: the last two dead chains, and what the sweep now says

Following the type-aware sweep's second category — fields read *only* inside `internal/config` —
needed one more refinement to be trustworthy. Marking a reader "reachable" only when production
names it directly reported 24 dead chains, almost all of them wrong: `validateMonorepo` is called
by `Validate`, which the loader calls, which production calls. With transitive closure inside the
package, 24 became **three**.

**`git.use_cli_fallback`** — `GitConfig.UseCLI()` existed to answer it and nothing called it, so
the git service kept its own default of true and shelled out to the git CLI whenever go-git
failed. Including in the environments that had turned the fallback off deliberately, which is
the only reason anyone sets it. Now passed at construction, beside the auth options.

**`channels.definitions[].require_approval` and `auto_approve`** — `relicta promote` builds its
registry from name, stability, tag_pattern, promotes_to and prerelease, and not from these;
`ChannelDefinitionConfig.NeedsApproval()` has no caller. Warned rather than refused, because
promote has **no approval step at all** to attach them to: this is a feature that was never
built, not a setting wired to the wrong place, and refusing would stop a promotion that works
today over a gate that never existed.

**A per-channel approval gate for `promote` is the feature this leaves open.** It needs the
same decisions the rest of the governance path already made once: whether promoting creates a
run, what it is evaluated against, and where the decision is recorded.

Where the sweep stands: 17 fields read by nothing, all on structs already refused at load or
already logged; 27 read within `internal/config`, of which 24 are reached through `Validate` or
`Load` and three were these. The unread-configuration thread is, for now, closed.

## DONE: a freeze that froze nothing, found by fixing the sweep's method

The previous entry ended by noting that the config-field sweep counts references by Go field
name, so a field whose name also exists on a struct that is read looks read. Replacing grep with
a type-aware pass — resolve every selector to the struct it is on, using `go/packages` type
information — changed the answer from 19 fields to **20 read by nothing at all** and 27 more read
only inside `internal/config`, which is a different question worth asking separately.

Two of the twenty mattered.

**`governance.freeze_periods` was enforced by an evaluator nobody told about it.** The evaluator
calls `budget.CheckFreeze` on every evaluation and holds `FreezePeriods` and `RiskBudget` fields
for exactly this — and neither of the two places that build its config passed them, so both
stayed at their zero values. A configured release freeze permitted every release.

For a governance tool this is the worst place for the defect: the control reports itself as
configured, the evaluation says approved, and the record shows a release that a policy said
should not happen. Verified against the binary, all three ways: inside a window with
`max_risk: 0.0` the decision is now `approval_required` naming the freeze; with `max_risk: 0.9`
and a 0.10 release it passes; with no freeze configured nothing changes.

**`git.default_remote` had no reader**, so every tag push went to `origin` — the usual name, and
not always the name. A repository whose publishing remote is `upstream` or `release` had the
setting to say so and the push went elsewhere.

The rest of the twenty are fields on structs already refused at load (`ReleaseGroupConfig`,
`VersionFileConfig`, the monorepo overrides).

**The method, for next time:** the sweep tool lives in the scratchpad rather than the repo,
because it needs `golang.org/x/tools` and this is not a dependency worth adding for something run
by hand every few months. What matters is the technique — resolve the selector, do not match the
name — and the two categories it produces: *never read*, and *read only within `internal/config`*,
which is where a dead helper chain hides.

## DONE: the last unread settings, and a sweep blind spot worth remembering

`output.quiet` had no reader, so a repository that asked for quiet output got the full chrome on
every command. It now gates the same seam `--json` does; warnings and errors stay, because they
are diagnostics rather than "non-essential output".

`output.plugin_audit_log` was worse than unread. `audit.Initialize` was never called, so the
package's global logger stayed nil and every `audit.LogLoad` and `audit.LogExecution` in the
plugin manager returned nil without writing anything. For a governance tool: the calls to record
plugin loads and executions were all in place, and the audit trail was empty. Initialized now
when the path is set.

`plugin_security.auto_install` is refused rather than implemented. Installing a plugin means
fetching and executing code that is not in the repository, and a release tool that starts
downloading executables because a config key looked plausible is not a decision to make on
somebody's behalf.

`versioning.version_files[].update_format` belongs to the monorepo `version_files` map, already
refused in the monorepo pass.

**And the one that was hiding: the whole `blast_radius` section.** `BlastRadiusConfig` was a type
and not a field on `Config`, so the section went nowhere and `relicta blast` ran on
`DefaultMonorepoConfig()` plus two flags. Nine settings. It is now a field, with defaults
matching the previous behavior, read by both construction sites — verified against the binary in
both directions: configuring `packages/*` finds the package, configuring only another path makes
it disappear, and no section at all behaves exactly as before.

**The method note that matters more than the fix:** the config-field sweep counts references by
Go field name, so a field whose name also exists on a struct that *is* read looks read.
`PackagePaths`, `ExcludePaths` and `RootPackage` all collide with the monorepo config. Only
`IgnoreDevDependencies` was unique, and it is the single thread that pulled out the other eight.
The next sweep should count by struct and field, not by name alone.

## DONE: the plugin config structs were documentation, and now they are documentation

Re-running the config-field sweep after the last few fixes turned up 53 fields with no reader —
and roughly forty of them belonged to six structs that nothing referenced at all:
`GitHubPluginConfig`, `GitLabPluginConfig`, `JiraPluginConfig`, `SlackPluginConfig`,
`DiscordPluginConfig`, `NPMPluginConfig`, plus `GitLabAssetLink`.

Not the loader, not the validator, not the plugin manager. A plugin's configuration reaches it as
`plugins[].config`, a `map[string]any` passed through verbatim over gRPC, so these types could
never have validated anything — nothing ever built one. They were documentation in the shape of
schema, and they will drift from the plugin repositories that actually define their config,
because those are separate modules.

Deleted, 124 lines, and every one of the 60 keys they documented is now in
`docs/plugin-configuration.md` with its type and meaning, under a note saying the plugin
repository is authoritative. No behavior changes: a user's `plugins[].config` reached the plugin
before and reaches it now.

The sweep reads 19 fields after this, from 53, and the remainder is almost entirely settings
already refused or warned about: the monorepo ones (#352), the Sigstore endpoints (#358),
`telemetry.tracing.insecure` (#357), and `prerelease_suffix`.

**Left for the next pass**, five settings with no reader and nothing yet said about them:
`output.quiet`, `output.plugin_audit_log`, `plugins.security.auto_install`,
`versioning.version_files[].update_format`, and `blast_radius.ignore_dev_dependencies`.

Fixed alongside: a flaky test of my own. `TestAMeasuredWindowStillRecordsSuccess` slept 200ms and
asserted; it passes alone and fails in a full parallel run, where the watch goroutine may not be
scheduled in time. Both timing tests now wait on the condition — one for the outcome, one for the
watch to actually end, which also stops the "records nothing" case from passing trivially when
the window never closed. A test that fails randomly teaches people to re-run rather than to read.

## DONE (in part): attestation signing fails at startup rather than mid-release

`attestation.rekor_url` and `fulcio_url` had no readers, which is the least of it. The signing
configuration was accepted whole and then failed *during publish*, from inside the attestation
step:

- `signing_mode: keyless` → "keyless signing requires sigstore-go"
- `signing_mode: local` with no `key_path` → "key_path is required for local signing mode"

By then the tag exists and the release is half done. Worse, an attestation failure is only fatal
when `attestation.required` is set — so the ordinary case is a release that ships **unattested**,
under a policy that asked for signatures, with one line of log saying why.

All of it now fails at config load, with the message naming what to use instead: `local` with a
`key_path`, or `none` for an unsigned attestation, which is still a provenance record and is the
default. `rekor_url` and `fulcio_url` warn that they configure the mode nothing implements.

Gated on `attestation.enabled`, so a repository that has not asked for attestations is not told
about signing modes.

**STILL OPEN: implementing keyless.** This is not closed by adding `sigstore-go` alone. The
current `SignedAttestation` stores only payload/signature/certificate fields, and `Verifier`
checks a signature only against an explicitly configured static public key. A production
keyless format must persist the Sigstore bundle (certificate, transparency evidence, and
related verification material), then verify it against an explicit OIDC issuer and certificate
identity policy plus the Sigstore trusted root. `attestation.rekor_url` and `fulcio_url` must
either configure supported instances end to end or fail validation; they cannot silently imply
support. Define the on-disk compatibility/versioning contract before changing the envelope.
Tag signing and verification are handled separately through Git's configured signing backend.

## DONE (in part): telemetry is read — metrics are served, tracing says what it cannot do

`telemetry` was a whole section with no reader, and it is two different situations wearing one
name.

**Metrics were real and unreachable from their own configuration.** `internal/observability`
has a complete Prometheus implementation and `relicta metrics` serves it — on a port from a
flag, at a hardcoded `/metrics`, with `telemetry.metrics.port` and `.endpoint` read by nothing.
A repository that configured an address was scraped at the address it configured, and found
nothing there. The command now takes its port and path from the configuration, with a typed
flag still winning, and `telemetry.metrics.enabled` exposes the same handler on the dashboard's
own port — so a deployment already running the server does not need a second process to be
scraped. Outside the dashboard's auth, because a Prometheus scrape carries no session, and only
when asked for, because release counts on an open port is the operator's decision.

**Tracing describes an export nothing performs.** `InitTracer` had no production caller, and
what it builds is a logging tracer — OTLP is a comment saying it can be added later. So
`endpoint`, `headers`, `insecure` and `sample_rate` configure a connection no code makes.
`tracing.enabled` now installs the tracer, so the setting does something, and config validation
warns that the OTLP settings do not. A warning rather than an error, on the reasoning the
original monorepo warning used: a project with these set is not misconfigured, it is expecting
something that has not been built, and refusing to start would take away the tracing it does
get.

**What OTLP export would need**, when somebody wants it: `go.opentelemetry.io/otel/exporters/
otlp/otlptracegrpc` as a direct dependency, a real tracer implementation behind the existing
`Tracer` interface, and a decision about what a release tool should trace — the command, the
release, or each step — because the answer decides whether the spans are useful or merely
numerous.

## DONE: git.auth is read, and an unset credential is refused rather than sent

`git.auth` is a complete, documented block — `type`, `token`, `username`, `password`,
`ssh_key_path`, `ssh_key_password`, each promising environment-variable expansion — and nothing
read any of it. The service had the mechanism too: `ServiceConfig.AuthToken` becomes an
`http.BasicAuth` used by every push and remote listing, and `WithAuthToken` / `WithAuthUsername`
existed to set it with no caller anywhere.

So a repository that configured a token pushed with whatever ambient credential the machine
happened to have, or failed. The first of those is the dangerous one: it succeeds, as somebody
else.

Now honored: `auto` (the default, unchanged — the credential helper and SSH agent every
repository already relies on), `token`, `basic`, and `ssh`, the last via go-git's
`NewPublicKeysFromFile`, which the service had no path for at all.

**The finding that made this dangerous rather than merely broken:** `config.ExpandEnvVars`
leaves an unset variable as the literal `${NAME}` — correct for a template, wrong for a
credential. A config naming an absent variable would have pushed with the text
`"${GITHUB_TOKEN}"` as its password: a request that fails against a forge, and succeeds against
anything that accepts any password. An unexpanded placeholder now reads as absent, and absent is
refused at startup with the setting named.

Refused at startup rather than at push time throughout — a credential problem discovered at push
is discovered after the tag exists and half the release has happened. Also fixed: `~` in
`ssh_key_path` was not expanded, so the most natural spelling of the setting failed with "cannot
be read".

Verified against the shipped binary: the secret never appears in output, an unset variable is
refused by name, an absent key file and an unknown type each stop startup, and a repository with
no `git.auth` behaves exactly as before.

## DONE: the observability integration is wired, on "no data beats wrong data" (ADR-016)

The entry below asked three questions before this could be built — when monitoring starts, what
`auto_record` writes, and what counts as unhealthy. One answer settles all three: **a release
nothing could observe is unmeasured, and nothing is recorded for it.**

What the code was doing instead, at both ends:

- `performCheck` started from `Healthy: true` and only cleared it on a crossed threshold, so an
  unreachable provider left every number at zero and the release was reported healthy. An
  existing test asserted this deliberately — *"expected healthy when metrics query fails
  (graceful degradation)"* — and the degradation was to a claim.
- `runWatch` recorded success when the window expired, unconditionally, including when every
  check in it had failed. A Prometheus down for half an hour produced a recorded successful
  deployment, and that number feeds change failure rate.

Now wired: providers are built from `observability.providers` (prometheus; an unknown type is a
startup error, not a skipped entry), the `ObservabilityService` the handlers declare has an
implementation, `SetObservabilityService` is called, and the server starts a health watch when
it hears a release published. The watch lives in the server because a window is minutes or hours
and `relicta publish` exits in seconds — a watch shown but not running is the same lie one level
up.

A repository with no providers still gets **no service**, so the routes keep reporting
`not_configured`: the distinction between "nobody is watching" and "everything is healthy" is
the whole point.

Two gaps closed straight after, both the same shape as the rest: `auto_record` gated a recorder
that was nil, so it did nothing either way; and the watch was started from a domain-event
subscription the server can never receive, because the dashboard publishes nothing and
`relicta publish` runs in another process. The recorder now writes a measured failure as an
incident against the release, and the server picks up releases published inside the window from
the store the CLI writes to.

Incidents are now recorded through the governance memory rather than accumulated in a slice —
which was lost on restart and written from HTTP handler goroutines while the correlations
endpoint read it, a data race. They are attributed to a release on arrival, while the labels the
scoring reads are still in hand, and read back filtered to the release asked about. Two smaller
findings on the way: a score that cannot be read back is reported as absent rather than invented,
and an Alertmanager payload with no fingerprint — which the store refused and the service dropped
with only a log line — now gets a deterministic derived ID.

The health metric contract is now explicit and implemented: error-rate queries return an absolute
percentage, and the default PromQL derives it from error/request counter rates; p95 latency is
compared with the first measured sample in the release watch, not compared as seconds against a
percentage threshold. Operators can override both PromQL expressions for their metric names and
labels. A positive outcome requires both metric signals; zero traffic, missing samples, invalid
latency baselines, or a missing signal leave the watch unmeasured instead of recording success.
README's configuration example now matches the schema that the service actually reads.

Left for later: the alert's labels are not kept, because `IncidentRecord` has nowhere to put
them. Nothing that is used today is missing, but re-scoring a correlation from storage would
need them.

## DONE: the persistence default stays `file`, and stays configurable

Three backends behind one conformance suite (ADR-013), with SQLite benchmarking 2.6–6× faster,
made flipping the default look obvious. It stays `file`: the default decides where an existing
repository's history lives, and flipping it moves every repository that has not opted out, on
upgrade, silently. `persistence.backend` remains the choice a team makes when they outgrow the
default. Recorded as an ADR-013 amendment.

## DONE: the monorepo settings that still did nothing

Found by the config-field sweep — extract every `mapstructure` tag, count production readers of
its Go field outside `internal/config`, `internal/ui/wizard` and `internal/cli/templates`, read
what is left. 57 fields have no reader; most belong to entries already here or to plugin blocks
external repositories read. Six were in the monorepo section this session had just built around,
which makes them the worst kind: a feature that works, beside settings for it that do not.

Honored:

- **`package_overrides.<path>.skip_versioning`** — the package is now excluded from everything,
  not just versioning. A package relicta does not version has no version to tag and no release
  to describe, so tagging it or writing it a changelog would claim a release nobody asked for.
  Applied at discovery so every later step sees one list; a filter applied per step is one a
  step will eventually forget.
- **`changelog.root_changelog: false`** — now stops the repository-wide changelog. It defaulted
  to true and was read by nothing, so it was accidentally right when set and silently ignored
  when cleared.

Refused, because nothing implements them: `version_files` (only when customized — its default
agrees with what the writers detect), `dependency_coordination`, `changelog.include_package_links`,
and `package_overrides.<path>.version_file` / `version_field`.

**Two of those defaulted to on**, which is the trap this pass nearly walked into: refusing a
setting whose default asks for it fails every configuration in existence. `dependency_coordination`
and `include_package_links` now default off. A default that asks for a feature nothing performs is
the same lie as a setting nothing reads, and a louder one.

## DONE: three AI settings that nothing read

Found by counting production callers of the `With*` option constructors: 33 of 90 have none.
Most are legitimately test-only affordances (`WithNowFunc`, `WithHTTPClient`), but three were
the other thing — a documented, defaulted setting with a complete mechanism behind it and no
link from the configuration to the code.

- **`ai.custom_prompts`** — six fields for replacing the prompts relicta sends. Every provider
  applies them (`prompts.applyCustomPrompts(cfg.CustomPrompts)` in openai.go, anthropic.go,
  gemini.go and ollama.go) and `ai.WithCustomPrompts` exists to carry them, but the container
  never called it. A team that rewrote its release-notes prompt got the default prompt, with
  nothing said.
- **`ai.retry_attempts`** — the resilience layer reads `cfg.RetryAttempts` and
  `ai.WithRetryAttempts` sets it; nothing called that either, so a project asking for one
  attempt or for ten got the library's own default.
- **`ai.include_emoji`** — dead at four links at once. The prompt templates have honored an
  `IncludeEmoji` option since they were written, but `ports.NotesOptions` carried no such
  field, the notes adapter set none, the config key had no production reader, and
  `relicta notes --emoji` registered a flag variable that was read nowhere. Two ways to ask,
  neither of which did anything.

The option-building is now its own function (`aiServiceOptions`) so the translation can be
asserted; a zero or empty value is still left out rather than sent, because for these settings
"not configured" has to stay distinguishable from "configured to nothing". `--emoji` folds into
`ai.include_emoji` the way `--skip-push` folds into `versioning.git_push`, and an explicit
`--emoji=false` still beats a config that asks for them.

The method is worth repeating on the rest of the list. What it does not catch is the shape
found earlier this session — a setting read by code that is itself unreachable — so a caller
count of one is not proof of anything.

## DONE: `--skip-tag` and `versioning.git_tag` now reach the tag

`relicta publish --skip-tag` printed `Create tag: false`, reported `"create_tag": false` in
`--json`, and created the tag anyway. `versioning.git_tag` had no production reader at all, so a
repository that set it to `false` was tagged on every release. Reproduced against the shipped
binary, both before and after.

The reason the flag had nothing to act on: the tag step is added to the run's execution plan at
*approve* time (`ensureTagStep`), so by the time publish ran there was no decision left to make —
only a step to execute. The gate has to live where the step runs, which is
`PublisherAdapter.executeTagStep`.

Fixed the same way `--skip-push` was, one flag along: `WithTagging(enabled)` on the publisher,
wired from `versioning.git_tag`, with `--skip-tag` folded into that config before the container
reads it. The option is phrased positively so the zero value is the harmless one — a publisher
built without options creates nothing. That matters more than it sounds: every construction site
forgot the push option, which is how that defect survived.

When tagging is off the step still runs and still records itself, reporting that the tag was not
created and why. Dropping the step from the plan would leave the run's record silent about a
decision somebody made.

## DONE (in part): monorepo `independent` versioning is wired; tags and governance are not

The largest instance of this defect class in the tree. Every field of the `monorepo:` section had
zero production readers, while `internal/domain/monorepo` and `internal/application/monorepo` held
~3,050 lines of implemented, tested code that nothing in the release path called — plus 3,149
lines of tests for it. Verified against the built binary: a repository with `enabled: true`,
`strategy: independent`, `package_paths: ["packages/*"]` and two packages at 1.0.0 and 2.0.0 was
told its next version was 0.1.0, and neither `package.json` was touched.

The product decision it was waiting on is recorded in ADR-015: **per-package versioning is a
commitment, for `independent` only.** `relicta bump` now analyzes each package's own commits,
computes its own version and writes its own manifest; `lockstep`, `hybrid` and `release_groups`
are refused at config load rather than served as something else.

Wiring it surfaced three defects that only a real repository would have shown, all in code that
had passed its tests for as long as nothing could call it:

- `package.json` was decoded into a `map[string]interface{}` and re-marshalled, and Go marshals
  map keys alphabetically. Every bump would have permanently reordered the manifest.
- `Cargo.toml` and `pyproject.toml` were rewritten with `^(\s*version\s*=\s*)"[^"]+"` over the
  whole file, so a dependency declared as its own table had the package's version written over
  its own. Silent corruption of a file the build reads.
- Manifests were rewritten `0644`, widening one somebody had kept private.

Two more were found by running the thing rather than reading it: packages were typed from the
workspace-wide `PackageManager`, so `packages/*` holding an npm package beside a Go module read
one manifest and wrote another; and the base version came from the working tree — the file the
previous bump had written — so bumping twice off one commit took a package 2.1.3 → 3.0.0 → 4.0.0.

**Tags landed next** (ADR-015 amendment). Each package carries its own — `api-v1.5.0`, or the
configured `package_overrides.<path>.tag_prefix` — created by `publish` alongside the
repository's own tag, and read back by `bump` so each package measures from its own last release.
Two more defects fell out of running it: the release commit did not cover the packages'
manifests, so the tag pointed at a commit that did not contain the versions it claimed and the
clean-tree gate refused the publish; and `bump` left the run in `planned`, so `notes`, `approve`
and `publish` all told a user who had just run bump to run bump.

**Changelogs landed next.** Each package gets its own `CHANGELOG.md`, rendered from its own
commits through the repository's `changelog.*` settings and carried by the release commit, so the
package's tag contains the entry describing it. Three more defects fell out: the `monorepo:`
section had no viper defaults registered, so a config naming only `enabled` and `package_paths`
was refused for an empty strategy; the analyzer put the raw git subject into the changeset, so
entries read `- fix: correct the status code` under a `### Bug Fixes` heading; and its fallback
classifier was a prefix match that dropped scopes, read `perf`/`build`/`ci`/`style`/`revert` as
chores, and called `fixup! ...` a fix.

**Governance landed last** (ADR-015 amendment). Each package carries its own run — version,
notes, risk, approval, audit entry — decided with `relicta approve --package <name>`, and publish
tags only the packages that were decided. A monorepo release has two levels: `approve` decides the
release, `--package` decides what is in it, and the decision table is printed at both.

Two defects found by running it: all three runs shared one ID, because the plan hash covers
repoID, base, head, commits and version, every one of which can match between two packages of one
repository — the package is now part of `repoID`, which leaves existing repositories' IDs
untouched; and each package's run recorded the repository's whole commit range rather than its
own, so its risk was assessed on changes it does not contain.

Package aggregates now run their own publish use case after the repository publish creates
package tags. Their tag step is idempotent, while package-specific plugins, attestations, audit
events and state transitions remain on the package run. Held packages are skipped, and package
runs do not advance when tagging is disabled. If a package step fails, the repository release
remains recorded as successful, the command reports the package failure, and a later publish can
resume the package aggregate.

The tag step also resumes partial side effects: an existing local tag no longer short-circuits a
failed push or unfinished package tags. Existing package tags are pushed again on retry, which
lets a publish recover after a partial local tag/push failure without recreating tags.

## DONE: one-shot release shares publication gates

`release` and `publish` now pass through the shared pre-publication gate and shared hook and
artifact preparation paths. The one-shot workflow no longer bypasses branch, working-tree,
up-to-date, or actor-budget policy.

## DONE: MCP analysis honors configured version sources

MCP analysis resolves the configured `versioning.bump_from` source from the repository on each
request and passes that version into plan, inference, and diff analysis. Long-lived agent sessions
therefore see a version changed by `relicta_bump` or an external manifest edit. The same analyses
also use the configured tag prefix. A configured but unreadable version source fails the request
with a configuration-specific error instead of silently falling back to tags.

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

`relicta db import` now covers release runs, governance memory, and CGP protocol handshakes. It reads the
existing `.relicta/` files without changing them, supports `--dry-run`, reports partial writes,
and is safe to rerun. The release-run importer has a file-to-SQLite round trip; the governance
importer has round-trip, idempotency, source-preservation, dry-run and partial-failure coverage.
Deployments are reported as not migrated because neither database store has a deployment table.

Still open on the storage work:
- One backend setting does not yet cover the full product record. Release runs and governance
  memory can use file, SQLite or PostgreSQL, and audit chains have database adapters. CGP protocol
  proposals/decisions now follow `persistence.backend` across MCP, `relicta cgp` and `relicta audit`.
  `relicta db import` now also copies CGP proposals and their associated decisions and
  authorizations from `.relicta/cgp/`, and configured identity grants, without changing their
  source trees. Analytics now follows the selected backend, with migration 007 for SQLite and
  PostgreSQL; `relicta db import` copies the existing JSONL events by ID and can be rerun after
  interruption. Reputation scores are computed per evaluation from governance release and
  incident history; the production path uses a no-op score store and does not persist score
  history. Attestations are release artifacts written beside their run, not records currently
  imported by `relicta db import`; retention and migration need an explicit artifact contract
  before moving them into a backend is meaningful.
- The public `App.UnitOfWork()` currently constructs a `FileUnitOfWork` for every backend, but
  a source scan found no production callers. Release commands save through the selected run
  repository directly. The actual cross-record gap is that `EventPublishingRepository.Save`
  persists the run first and publishes governance/webhook events afterward. Audit-chain and
  outcome-store failures now return errors after logging and continue forwarding the batch, so
  the wrapper retains the events on the live aggregate rather than clearing them as delivered.
  Webhook delivery likewise waits synchronously for configured in-process retries and returns
  an error when exhausted. This remains best-effort across process exits because pending events
  are not in the run payload; the wrapper also cannot know whether every derived history write
  completed. A durable outbox and idempotent consumer contract should be designed around this
  live path, rather than treating the unused unit-of-work facade as if it already protected
  release operations.
- Flipping the default remains deferred. The release-run conformance suite and importer
  round-trip evidence now exist, but backend parity for the rest of the product state does not.

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

## DONE: the event store is deleted, and the audit chain is the append-only record

ADR-013 established that the event store is not the system of record — the release run and the
governance record are. ADR-014 then gave the append-only record a real owner: the audit chain,
which is appended as decisions happen and verified on read, and which attestations carry a hash
of.

That answered the question this entry was left holding. The event log's only plausible value was
"an append-only record for auditors, distinct from the run aggregate's current state" — and the
audit chain is exactly that, already wired, already read. Keeping a second one that nothing wrote
to and nothing read from was two answers to one question, with the wrong one shipped in the
package layout.

Deleted, 760 lines:

- `ports.EventStore` (38) — the interface with no production implementation reached.
- `adapters.FileEventStore` (368) and `persistence/postgres.Store` (296) — both implementations.
- `persistence.NewEventStore` (58) — the factory that selected between them, called by nothing.

`EventPublishingConfig.EventStore` went with them; `Save` now publishes the aggregate's events and
clears them, which is what every caller already relied on since the store field was always nil.

Kept: `migrations/001_create_events`, because it has shipped. The table is created and stays
empty. Removing a migration from an ordered sequence rewrites history for anyone who already ran
it, which costs more than an unused table.

Fixed alongside this entry: the `db` command is now registered (it was documented in CLAUDE.md
and absent from the binary), and `persistence` has defaults (`DefaultPersistenceConfig` existed
and was called from nowhere, so the backend was "" and the pool size 0).

The method is worth keeping: extract every field with a `mapstructure` tag, count references
to the Go field name outside `internal/config`, `internal/ui/wizard` and
`internal/cli/templates`, and read what is left. A setting that does nothing is worse than a
missing one, because the user believes it is in force — `require_clean_working_tree` defaults
to **true**, so every user had a safety gate switched on that never ran.

## DONE: the unauthenticated MCP HTTP transport binds locally by default

`relicta mcp serve --port 8080` previously listened on every interface, while the MCP HTTP
transport had no authentication or TLS. That made the documented quick-start command expose
release tools to any network client that could reach the host. HTTP now defaults to
`127.0.0.1`; a non-loopback address is refused unless the operator passes
`--allow-insecure-http`, and the CLI error says that the flag only permits the listener. The
operator must still put a trusted TLS/authentication proxy in front and block direct access.
Stdio is unchanged.

Verified: focused CLI tests cover IPv4/IPv6 loopback, localhost, wildcard/public binds and the
explicit opt-in; running `relicta mcp serve --transport http --address :8099` exits before config
or repository initialization with the refusal message. `go test ./internal/cli ./internal/mcp
./internal/httpserver`, `git diff --check`, and `make fmt-check` passed.

## DONE: MCP release notes use the calling agent's session

For the Govee workflow, technical commit titles are evidence, not customer copy. Sending those
commits to Relicta's separately configured AI provider meant another model call with less of the
conversation's product context, and made a correctly framed first-party agent session unusable
for the prose step.

The MCP path returns a deterministic `recommendation` from `relicta_plan`, including the version,
classified changes and breaking changes. The calling agent can draft customer-facing notes in its
own session and pass them as `draft` to `relicta_notes`. Relicta persists the text through
`GenerateNotesUseCase`, marks it as agent-authored, and does not call its AI provider for that path.
The submitted text, including surrounding whitespace, is preserved and hashed as the draft
content. The saved notes then flow through the existing governance, approval and publish
workflow. `docs/mcp.md` and the README document the plan → bump → draft → notes sequence; the MCP
guide now includes a reusable customer-facing prompt and a claim-review checklist for converting
technical commit histories into product outcomes. Handler coverage verifies the draft is
persisted with `agent` provenance and the requested audience.

This gives the Govee workflow its desired handoff: Relicta supplies auditable release facts and
state, while the agent session supplies the product context and customer-facing wording.
The reusable MCP release-summary prompt now explicitly uses product purpose, audience, vocabulary,
and tone already established in the calling agent's session for framing, while requiring every
release claim to stay grounded in the Relicta recommendation and verified facts. This removes a
prompt-level conflict that could suppress the same session context the feature is meant to use.

## DONE: changelog format and custom template configuration is applied

The built-in renderer now distinguishes Keep a Changelog section names from conventional names,
and `simple` flattens sections to a concise list. Custom templates receive the rendered
`communication.ChangelogEntry` (`Version`, `Date`, `Sections`, `CompareURL`, `IsUnreleased`);
each section has `Title` and `Items`, and each item has `Description`, `Scope`, `CommitHash`,
`Author`, and `IssueRefs`. The same renderer is used by basic release notes and monorepo package
changelogs. Template errors fail the operation rather than silently falling back.

## DONE: publish retries validate existing tag targets

When a previous publish created a local tag but failed before pushing it, a retry now resumes the
push only if the tag resolves to the current release commit. Existing package tags receive the
same check. Conflicting or stale tags fail closed with both commit hashes in the error, instead
of being treated as successful idempotency. Annotated tags resolve through their target object to
the commit, matching lightweight tags and nested annotated tag references.

## DONE: CLI release-note output identifies its authoring source

The notes stored on a release already retain their provider (`basic`, configured AI provider,
or `agent`), and JSON output already exposed it. The human-readable `relicta notes` output did
not, making a deterministic template look like customer-ready prose and hiding whether an AI
provider had drafted the content. The terminal now labels the displayed notes as a deterministic
template, calling-agent draft, or AI-generated with the provider name. The label stays outside
the note body and output file so copied release text remains clean.

## DONE: quick-start direct download matches release assets

The quick-start used raw `uname` values in the archive URL (`Linux_x86_64`, `Darwin_arm64`)
while GoReleaser publishes lowercase OS names and `amd64`/`arm64`. It then looked for the binary
inside a `relicta_*` directory even though tar extracts the named binary at the archive root.
The documented Linux/macOS path now maps OS and architecture to published asset names, fails on
unsupported platforms, downloads with HTTP failure/retry handling, verifies the published SHA-256 checksum, and
optionally verifies the Sigstore checksum signature against the exact tag-triggered release
workflow when Cosign is installed. Without Cosign it warns that the checksum detects corruption
but does not authenticate the publisher. The script cleans up its temporary directory on failure
or success, extracts only the executable, and installs it with executable permissions. The
published v4.2.0 checksum bundle was verified locally against the workflow identity and GitHub
Actions OIDC issuer.

## DONE: release archive configuration and tool version are deterministic

GoReleaser's `goreleaser check` reported the archive `builds` field as deprecated; it now uses
`ids`. The release workflow pins GoReleaser to the locally validated v2.20.3 instead of resolving
any v2 release at runtime. Removed the optional UPX configuration: the release runner did not
install UPX, so CI published uncompressed archives while a maintainer with UPX installed could
produce compressed artifacts locally. Release archives now have one predictable build path.

**Open packaging migration:** GoReleaser deprecated its `brews` publisher in favor of
`homebrew_casks`, while Relicta is an open-source command-line tool that Homebrew's current policy
says normally belongs in a Formula. Homebrew permits Casks on Linux when their artifact type is
supported, and GoReleaser Casks can install binaries, so a cross-platform Cask is technically
possible. That does not make it the preferred packaging model here: it would require a reviewed
Formula-to-Cask tap migration, could create token ambiguity with the existing Formula, and the
macOS binary still needs Apple's trust. A live check of the v4.2.0 macOS ARM64 binary found
an ad-hoc linker signature (`TeamIdentifier` absent), and `spctl --assess --type execute` rejected
it. Cask migration should follow Developer ID signing and notarization so Gatekeeper accepts the
binary. GoReleaser can publish Casks without the deprecated formula publisher, but its suggested
unsigned-binary quarantine-removal hook bypasses Gatekeeper and is not appropriate as the default
customer path. The config supports cross-platform signing and notarization for the `relicta`
binary, but none of the five required Apple Developer ID / App Store Connect GitHub secrets are
configured in this environment, so releases remain unsigned until account setup is completed.
The GoReleaser notarization block now activates only when all five values are present. The tag
workflow rejects partial setup, uses the full cross-platform config when all five are present,
and otherwise publishes Linux and Windows only. This preserves those release channels without
publishing macOS binaries that Gatekeeper rejects. Manual snapshot builds remain unsigned for
review and do not publish. Configuring the Apple developer account and migrating Homebrew to a
Cask remain required before macOS is a supported production install path.
An initial inspection on 2026-11-25 found only intermediate artifacts in the repository's ignored
`dist/` directory (87 MiB of Linux binaries, metadata, and resolved config), with 1.9 GiB available.
That output was not a complete snapshot. A later isolated GoReleaser run using a temporary output
directory completed successfully and preserved `dist/`; its archive, SBOM, checksum, Formula, and
embedded dashboard verification is recorded in the snapshot workflow section below. Run the
snapshot workflow on a GitHub-hosted runner and inspect its uploaded artifact; the tagged release's
download URLs, attestation subject set, and tap publication remain unverified.
The tap still contains a Formula, and Linux Homebrew is an advertised install path. A Cask is a poor
fit for a command-line-only tool under Homebrew's package policy. Cross-platform Casks are
technically possible, but the generated Cask could not be verified and the Formula-to-Cask token
migration is unresolved. The release workflow now publishes the maintained Linux Formula through
the checksum-derived custom publisher rather than GoReleaser's deprecated `brews` publisher; the
snapshot validator checks that same generated Formula. The tap migration remains separate. A
read-only check of `relicta-tech/homebrew-tap/tap_migrations.json` returned 404, so no migration is
present. A snapshot attempt with the Cask config exhausted the workstation's temporary build space
before generating the Cask. Do not migrate the tap until it has a verified `tap_migrations.json`, a
plan for Linux Homebrew users, and a generated Cask whose install behavior has been reviewed.
Configuring the Apple developer account and completing a supported macOS distribution path remain
required. Sigstore signatures on the checksum manifest do not replace Apple's executable signing.
**Rechecked 2026-11-25:** pinned GoReleaser v2.20.3 parses both release configs but `goreleaser check`
exited nonzero for each because the `brews` publisher was deprecated. The live tap still has
only `Formula/relicta.rb`; its v4.2.0 formula advertises both Darwin archives as well as Linux,
while this repository's README says Homebrew is supported on Linux only. The tap has neither
`tap_migrations.json` nor `Casks/relicta.rb` (both GitHub content lookups returned 404). This is a
real install-path mismatch: the documentation change does not stop a macOS user following the
existing tap formula from receiving an unsigned binary that Gatekeeper rejects. Resolve the tap
formula and migration before claiming the macOS Homebrew path is retired; do not silently publish a
second package under the same token.
**Packaging direction update (2026-11-25):** the earlier assumption that a Cask could not preserve
Linux Homebrew installs is incorrect. Homebrew's [Cask platform policy](https://docs.brew.sh/Acceptable-Casks)
allows Linux when the artifact type is supported; GoReleaser's
[Cask publisher](https://www.goreleaser.com/customization/publish/homebrew_casks/) supports binary
artifacts. However, Homebrew says open-source command-line-only software normally belongs in
`homebrew/core` as a Formula ([package type policy](https://docs.brew.sh/Acceptable-Casks#appropriate-package-type)).
Do not switch this tap to a Cask solely to avoid a deprecated GoReleaser integration. First decide
between migrating Relicta to a maintained Formula distribution path (including a possible
`homebrew-core` submission) or coordinating a tap-specific publisher/migration that preserves
Linux support, avoids ambiguous `relicta` Formula/Cask tokens, and provides signed/notarized macOS
artifacts. Validate the chosen generated manifest and install behavior on both operating systems
before changing the advertised install command.

The release hook uses `go mod tidy -diff` so a release build checks module-file consistency
without mutating `go.mod` or `go.sum`; `go mod verify` checks the downloaded module cache.

## DONE: remove GoReleaser's deprecated Homebrew publisher

Both release configs no longer declare the deprecated GoReleaser `brews` publisher. The release
workflow already skips that publisher and updates the existing Linux Formula only after the
checksum manifest is attested; the snapshot validator now checks the exact formula rendered by that
same custom publisher instead of requiring GoReleaser to emit a second formula. This removes the
deprecated integration without changing the tap's Formula token or Homebrew install path. The
macOS-only Cask migration remains gated on Apple trust and external tap coordination. Both configs
pass `goreleaser check` with v2.20.3 after this change; the hosted snapshot workflow still needs to
confirm the resulting artifact layout in GitHub Actions.
On 2026-11-25, an isolated GoReleaser v2.20.3 snapshot built Linux amd64/arm64 and Windows amd64
archives without generating a second GoReleaser Homebrew formula. The repository snapshot validator
passed against that output: the dashboard assets are embedded in all three binaries, all three SPDX
SBOMs parse, six checksums match their artifacts, and the custom Linux formula's URLs and hashes
match the checksum manifest. The build and validation output stayed outside the worktree; a hosted
snapshot and tap install remain unverified.

## IN PROGRESS: release workflow gets a read-only snapshot path

Manual dispatch previously shared the release job's write permissions and exposed a
`dry-run: false` option even though GoReleaser can only publish a tag-triggered release in this
workflow. The working-tree edit routes `workflow_dispatch` to a dedicated snapshot job with only
`contents: read`; the job builds and uploads archives without Cosign identity, release, tap, or
attestation permissions. It explicitly selects `.goreleaser.non-macos.yaml` and passes
`--skip=sign`, so the uploaded snapshot is the Linux/Windows build and does not require Apple
credentials or OIDC signing. Tag pushes alone run plan validation and the publishing job.
`actionlint` passes. GoReleaser parses the fallback configuration, then exits nonzero solely
because the retained Linux Formula uses deprecated `brews`. Local signing could not be exercised
because the workstation has no GitHub Actions OIDC identity; the snapshot job intentionally does
not sign or publish. The isolated local snapshot and archive checks are recorded below; the hosted
workflow and its artifact upload remain to be verified.
The working-tree dispatch path selects the Linux/Windows config when Apple credentials are absent,
rejects partial Apple credential configuration, and bounds GoReleaser parallelism to two workers.
This workflow change is not present at `origin/fix/dependency-vulnerability-updates` yet. That
remote commit still uses the older dispatch path, whose `release` job can run on manual dispatch;
do not trigger a remote Release dispatch until the read-only workflow change is committed and
available at the selected ref. `goreleaser check` is not a required workflow gate: GoReleaser exits
nonzero solely because the retained Linux Formula uses deprecated `brews`, even when that
configuration is otherwise valid.
The hosted snapshot job validates its output before upload: Linux amd64/arm64 and Windows amd64
archives, an SBOM for each archive, the checksum manifest, and every checksum entry must be
present and valid. It extracts each platform binary and checks that the dashboard index and built
JavaScript/CSS assets are embedded. The validator also requires the generated Linux Homebrew
Formula to include both Linux archive targets and reject Darwin entries, preventing the fallback
config from advertising unsigned macOS binaries.

An isolated GoReleaser v2.20.3 snapshot completed locally using a temporary output directory, so
the existing ignored `dist/` artifacts were preserved. It generated all three expected archives,
SBOMs, checksums, and `homebrew/Formula/relicta.rb`; every checksum verified. Inspection of all
three platform binaries confirmed that the dashboard index, JavaScript, and CSS are embedded.
The workflow validator was corrected to check the actual Formula path. `actionlint` and
`git diff --check` pass. The GitHub-hosted workflow itself has not yet run.

Snapshot validation now lives in `scripts/validate-release-snapshot.sh` and is shared by the
workflow and `make validate-release-snapshot [DIST_DIR=...]`. It checks the expected Linux/Windows
archives, embedded dashboard files, parseable SPDX SBOM documents, the Linux-only generated
Formula, and every SHA-256 manifest entry. Running it against the isolated local GoReleaser output
passed all checks; this does not substitute for the still-pending GitHub-hosted workflow run.

On 2026-11-25, a fresh `make release-local` initially failed the validator because the Makefile
used `.goreleaser.yaml`, which includes Darwin assets in the Homebrew formula, while the release
workflow correctly selects `.goreleaser.non-macos.yaml` until signed/notarized macOS distribution is
available. The local `release-local` and `release-snapshot` targets now default to that same
non-macOS config and permit an explicit `GORELEASER_CONFIG` override. The first attempt also
exposed stale module checksums: `go mod tidy -diff` found missing checksums for the current JSON
schema dependency and obsolete entries for the former MCP version. `go mod tidy` reconciled the
lock data. A fresh local snapshot then built the Linux amd64/arm64 and Windows amd64 archives,
generated all three SPDX SBOMs and the Linux-only Formula, and passed the complete snapshot validator,
including every checksum entry and embedded-dashboard check. No publication occurred; the hosted
release workflow remains unverified.
The packaged Linux amd64 executable was then launched in an amd64 container: `version`, `health`,
and top-level help completed successfully. The health report was appropriately degraded in an empty
directory because it was outside a Git repository and had no configuration; Git and the binary
itself were detected. Native macOS execution of the Linux artifact was not possible, so the smoke
check used an existing amd64 Linux image.
After the module tidy, `govulncheck ./...` reported no reachable Go vulnerabilities, and both
`app/` and `web/` production dependency audits reported zero vulnerabilities.

Release artifact review found that both GoReleaser configs compiled without the `embed_frontend`
tag, so CLI archives would omit the dashboard even though `make build-with-frontend` supported it.
Both configs now run `make frontend` as a before hook and compile with `embed_frontend`; the release
and read-only snapshot jobs set up Node.js 24 for that hook. `make build-with-frontend` passed, and
the isolated GoReleaser snapshot verified the dashboard files in the Linux archive binary.
`actionlint .github/workflows/release.yaml` and `git diff --check` pass. The hosted snapshot still
needs to confirm the embedded UI and archive layout in GitHub Actions.

The multi-platform container build had the same omission: its Go compile did not build the Vue
dashboard or use the `embed_frontend` tag. The root `Dockerfile` now builds the dashboard in a
pinned Node 24 stage, copies it into the Go build stage, and embeds it in the resulting image
binary. Host and Docker builds using `embed_frontend` succeeded. The ARM64 image built in about two
minutes and the emulated AMD64 image in about 7.5 minutes. Both report the expected CLI version,
run as UID 1000 `appuser`, and contain the dashboard index and built JavaScript asset markers.
The ARM64 image then started healthy with API-key authentication; an HTTP request returned the
embedded dashboard shell and hashed bundle reference, and the auth-mode endpoint reported
`api_key`. The smoke container was stopped and removed after inspection.
GoReleaser's isolated Linux/Windows snapshot passed for amd64/arm64 Linux and amd64 Windows,
including checksum/SBOM checks, and all three binaries contained the frontend embed markers. This
verifies both configured local container targets; the GitHub-hosted multi-platform image build,
scan, and attestation workflow still needs to run before the release pipeline is production
verified.

**Operational release freshness:** GitHub currently exposes `v4.2.0` as latest (published
2026-07-10); a live GitHub API check on 2026-11-25 confirmed it remains latest. The installer and
Homebrew tap therefore point to that release. The repository currently has `HOMEBREW_TAP_TOKEN`
but none of the five Apple signing/notarization secrets, confirmed by listing secret names only.
A fresh read on 2026-11-25 still shows v4.2.0 as latest and only `HOMEBREW_TAP_TOKEN` configured.
README now labels Homebrew as Linux-supported and points macOS users to `go install` or source
builds until notarized binaries are released. macOS binary signing, notarization and the Homebrew
Cask migration remain macOS distribution blockers. Decide whether Relicta is ready for a new product release after this work is reviewed; do not make a tag
or publish artifacts from this audit branch automatically.

**Separate CI-action launch blocker:** inspection of the published `relicta-tech/relicta-action`
v2.5.0 implementation on 2026-11-25 found that `auto-approve` defaults to `true`. Its checksum
verification catches missing manifests, missing asset entries, download errors, and digest
mismatches as warnings and continues to execute the downloaded binary. That does not meet a
fail-closed supply-chain gate, and checksum verification alone does not authenticate the
publisher. README now documents the current behavior and the signed direct-download path for
fail-closed verification. The separate action repository still publishes v2.5.0 with fail-open
checksum handling. Open draft PR [#7](https://github.com/relicta-tech/relicta-action/pull/7), head
`9db954cd54de8d98ca10e967e5aecac7adbb7932` as checked on 2026-11-25, changes the
installer to stop before extraction when checksum download, lookup, format, or digest validation
fails; regenerates the bundled action; and refreshes semver-safe dependency locks. Its local lint
and bundle build pass, and npm audit has no critical/high findings after the lock refresh. CI exposed
two portability issues in its formatting/lint scripts: generated declarations were included in the
format check, then a quoted glob was passed literally on Windows. The branch now runs Prettier and
ESLint over the `src` directory with an explicit TypeScript extension for ESLint. Local format,
lint, build, and diff checks pass. A live read on 2026-11-25 shows all six Ubuntu, macOS, and
Windows unit/end-to-end CI jobs passed. The PR remains draft and review is required; existing
released action versions remain fail-open until a fixed version is reviewed, merged, and released.
The latest GitHub read on 2026-11-25 confirms PR #7 is still open at the same head, remains a
draft, and has no submitted reviews. Its two workflow runs completed successfully across all six
Ubuntu/macOS/Windows jobs. Those checks cover head `9db954cd54de8d98ca10e967e5aecac7adbb7932`,
which still declares the Node 20 Action runtime and checksum-only fail-closed verification. The
separate local Action checkout has uncommitted changes that upgrade to Node 24 and authenticate the
checksum manifest with Sigstore. Those newer changes are not in PR #7 and have no hosted CI evidence
yet; local format, lint, bundle build, production dependency audit, and diff checks pass. No PR
review, merge, or publish was performed.
Checksum validation still does not authenticate the manifest publisher. Preserve the existing
`auto-approve: true` default in v2 to avoid breaking noninteractive `full` workflows; the safer
default should be evaluated as a v3 behavior change with migration guidance. A separate local
action checkout now has fail-closed checksum parsing, exact-version-only caching, a plan-only
least-permission quick start, and updated v2 compatibility guidance. Its format, lint, build, diff,
and production-dependency audit checks pass; it is local work and has not been pushed. The v2.5.0
production dependency audit reports zero findings; 30 moderate findings remain in development and
test tooling and need separate toolchain review.
The latest local validation on 2026-11-25 reconfirmed TypeScript lint, Prettier,
the bundled action build, production-only npm audit, and diff checks. Tests were
not run in this pass. The fix remains uncommitted and unpublished in the Action
worktree and requires review before it can be recommended for production
publishing workflows. Its checksum check establishes integrity against the
downloaded manifest but does not authenticate the manifest publisher; signed
attestation verification remains a separate supply-chain requirement.
This pass also changed requested plugin installation failures from warnings to
fatal errors, so the Action will not continue without an explicitly requested
plugin. The Action lint, format, bundle, production dependency audit, and diff
checks passed again after that change.
The action bundle also stopped publishing generated source maps, which contained
absolute local checkout paths and made build output machine-specific. TypeScript
declaration maps and the bundled JavaScript map were removed; a rebuilt `dist`
contains no local workspace path references. The final local validation passed
lint, format, bundle build, production npm audit (zero findings), and diff checks.
**Current runtime and publisher-authentication gap (2026-11-25):** GitHub retired the Node 20
Actions runtime on 2026-09-23, so the published `v2.5.0` action's `runs.using: node20` is no
longer compatible with GitHub-hosted runners. The local PR worktree now targets `node24` and
authenticates `checksums.txt` with the release's Sigstore bundle before checking an archive digest.
The verifier pins the OIDC issuer and the Relicta tagged release workflow identity. An integration
check against the published v4.2.0 checksum manifest verified and returned the expected workflow
identity. The Action bundle builds; lint, formatting, full npm audit, production-only npm audit,
and diff checks pass with zero reported vulnerabilities. This work remains local and unpushed;
the live PR still points to its earlier head and remains draft. GitHub Actions documents Node 24
support beginning with runner v2.328.0, so the action setup guidance requires Node 24-capable
runners.
Relicta v4.2.0 does publish `relicta_windows_amd64.zip`, confirmed in the live GitHub Release
assets on 2026-11-25. The action installer maps Windows amd64 to that archive name, so the prior
Windows-runner blocker is closed for amd64; Windows arm64 remains unsupported because no matching
archive is published. This only establishes platform/archive compatibility. The action repository
still needs fail-closed checksum verification before its publishing path is recommended for
production. Preserve the existing `auto-approve: true` default in v2 for noninteractive `full`
workflow compatibility; evaluate a safer default as a v3 behavior change with migration guidance.

The in-repository direct installer had a related but distinct fail-open edge: when the signed
manifest omitted the selected archive, `awk` emitted no line and `sha256sum`/`shasum` could
succeed on empty input. The quick-start installer and supply-chain verification examples now
require exactly one matching checksum entry before checking the digest. README describes the
required Cosign and GitHub CLI verification accurately.

On 2026-11-25, verified the published v4.2.0 checksum bundle with Cosign against the documented
GitHub release workflow identity and OIDC issuer, checked the Darwin ARM64 archive against its
unique checksum entry, and verified its GitHub artifact attestation with `gh attestation verify`.

The published v4.2.0 Windows amd64 archive was also downloaded and verified on 2026-11-25: its
checksum manifest signature matched the release workflow identity, its digest matched the unique
manifest entry, and GitHub artifact provenance verified against the release workflow. Quick-start
now documents a pinned PowerShell install that performs those checks before extracting and
installing the binary; README and the supply-chain guide link to it. The PowerShell runtime is not
installed on the current macOS host, so the procedure was manually reviewed but not executed in
Windows PowerShell. Windows ARM64 remains unavailable because the release publishes no matching
archive.

The adjacent action worktree now combines that signed-manifest verification with the Node 24
runtime, immutable-version-only caching, and fatal handling for requested plugin installation
failures. On 2026-11-25, its bundled entrypoint rebuilt successfully; lint, formatting, production
dependency audit (zero findings), JavaScript syntax checks, private-checkout-path scan, and diff
checks passed. The action README examples now pin the currently published Relicta v4.2.0 instead
of the stale v1.3.0. Local Action commit `a56ad3c` contains these changes, but it has not been
pushed; live PR #7 remains draft at head `9db954cd54de8d98ca10e967e5aecac7adbb7932`, so its
checks do not cover these changes and the production GitHub Action remains a launch blocker.
The Action's own `sigstore` 5.0.1 verifier was also run against the published v4.2.0 checksum
manifest and bundle using the installer policy; it accepted the bundle and returned the expected
release workflow identity and GitHub Actions OIDC issuer.
Review then found pinned versions accepted a valid signer identity from any `v*` tag, allowing
another authentic release's manifest and archive to be replayed alongside a requested pinned
version. The local Action now binds the Sigstore certificate identity to the exact requested tag
for pinned versions, while preserving tagged-release matching for `latest`. Its documentation now
states that distinction. The bundled entrypoint rebuilt, formatting and lint passed, production npm
audit found zero vulnerabilities, and `git diff --check` passed. This follow-up remains local and
the draft PR still does not contain it; no tests were run.

The market-facing setup docs had two contradictory examples: quick-start set `git_push: true`
although the product defaults to local-only tags to avoid unexpectedly triggering release CI,
and README's repository group used an unsupported `repos` key, string members, and `lockstep`
strategy. Both examples now match the shipped config schema and defaults. The quick-start and
README explain that pushing is opt-in, can trigger tag-based CI, and needs a configured publisher
for a GitHub Release.

The README, quick-start, examples, and troubleshooting guides now use a pinned Go module install
for read-only CI plans. They no longer present the current fail-open Action release as a production
execution path. The docs state that production workflows should wait for a fail-closed Action
release or use the verified direct-download procedure. The Action PR's six Ubuntu, macOS, and
Windows CI checks passed for head `9db954cd54de8d98ca10e967e5aecac7adbb7932`, but PR #7 remains
draft and no fixed Action release is published, so this external launch blocker remains open.
The CI examples pin checkout/setup-go actions, fetch complete Git history for planning, set
`persist-credentials: false`, and grant only `contents: read`. The manual Action smoke workflow also
no longer passes its read-only GitHub token to the Action step.

**Relicta self-dogfood, 2026-11-25:** read-only `plan --analyze --dry-run` classified 168 commits
since v4.2.0 as conventional with confidence 1. A minimal dry-run plan recommends 5.0.0, with one
breaking change, 41 features, and 68 fixes. No release run was created; status remained empty, and
no bump, notes, tag, or publish was performed. The run also showed `plan --minimal` only affects
text output, so CLI help and its generated reference now state that `--json` ignores the flag. This
is backlog evidence for release review, not approval to ship the proposed major release.

**Release-note dogfood:** `docs/release-notes/5.0.0.md` is a customer-facing draft based on the
recommendation artifact and verified feature commits. It explains the single planned breaking
change (`relicta notes --ai` → `ai.enabled`) and is explicitly marked unapproved. The audit also
found that the published v4.2.0 release had no corresponding section in `CHANGELOG.md`; the
customer-readable highlights from the GitHub release are now recorded there with a compare link.
The GitHub release itself was not edited.
The same checkout reports `healthy` from `relicta health` and `No active release found` from
`relicta status`; the draft therefore remains a document for review rather than persisted release
state. The installed dogfood binary identifies as `v4.2.0-168-g72d31c6-dirty`.

## DONE: dashboard WebSocket credentials are exchanged for one-use tickets

The dashboard frontend previously appended its `localStorage` API key to the WebSocket URL.
The API now exchanges dashboard authentication for a cryptographically random, 30-second,
single-use ticket; the hub stores only its digest, expires stale tickets, and bounds outstanding
tickets. API-key query authentication has been removed. The browser obtains a ticket with its
configured API-key or session bearer header and passes only that ticket to the WebSocket upgrade. Ticket requests are
authenticated by the same configured middleware as REST and rate-limited; responses disable
caching. Dashboard docs and the client follow this flow. Remote dashboard deployments still
require TLS. A live check on 2026-11-25 found the repository's `copilot` GitHub environment has no
required reviewers, so it cannot act as a release approval gate. This was the state before the
later governed tag-release workflow below. It is retained as historical audit context; the current
workflow now validates the planned decision, waits for the protected `release-approval` review,
records the reviewer through `relicta approve`, and verifies the resulting audit chain before
publication. The current required environment is still absent in GitHub, so publication remains
blocked until it is configured.

## IN PROGRESS: embedded dashboard OIDC login uses a one-time handoff

The embedded UI supports API-key and password-session sign-in, including token refresh and
sign-out. Browser OIDC now validates state against a SameSite, HttpOnly cookie, requires HTTPS for
non-loopback callback URLs, stores the token pair behind a bounded 30-second one-use ticket, and
redirects the ticket in a URL fragment. The UI removes the fragment before exchanging the ticket
and stores session tokens in session storage; failed exchanges return to the OIDC sign-in button
for a retry, and fragment cleanup preserves router history state. API clients retain the JSON
callback. The OIDC state and ticket stores are process-local; the full multi-replica session
requirements are documented below.

The OIDC and HTTP server package builds and Vite production build pass. The first combined Go
build after the final change hit the workstation's temporary disk limit; a targeted
`go build ./internal/httpserver` subsequently passed with the available cache. `actionlint` and
`git diff --check` pass. The login screen, callback, and ticket exchange have not been exercised
against a live identity provider; this flow remains unverified for production.
OIDC discovery now rejects remote issuers that use plaintext HTTP, and rejects issuer URLs with
credentials, query parameters, or fragments before making the discovery request. HTTPS remains
required outside loopback; localhost and loopback HTTP providers remain available for development.
The dashboard deployment guide describes these constraints. The OIDC and HTTP server packages
build and pass `go vet`; no tests were run.
The state store no longer launches a cleanup goroutine and scans the full map for every login;
it now amortizes cleanup over 256 additions, with a one-minute throttle for capacity sweeps. The
OIDC and HTTP server packages build and pass `go vet` after this change. The store remains
process-local and its multi-replica deployment requirement is unchanged.
Deployment review found the token service's revocation and refresh-rotation denylist was
process-local. Session sign-out previously removed tokens only from browser storage, and the API revoked only
the access token; a copied refresh token could still rotate into a new session. The dashboard now
sends the access/refresh pair to logout, and the token service revokes both under one lock after
validating that they belong to the same subject. Refresh rotation also claims a refresh token
under that lock, so concurrent refresh requests cannot both rotate it. If a refresh response races
with sign-out, the client discards it and attempts to revoke the newly issued pair. Builds, `go vet`,
the frontend production build, JSON parsing, and `git diff --check` passed at that stage. Tests
were not run. At that point the denylist remained in memory, so a restart could restore the
validity of previously revoked tokens until expiration; the dashboard guide stated the default
15-minute access and 24-hour refresh exposure windows. A follow-up auth review found that token purpose was not encoded: access
tokens could be rotated, and refresh tokens could authorize API requests. New JWTs now carry an
access/refresh purpose and shared session ID, middleware accepts access tokens only, refresh
rotation accepts refresh tokens only, and logout checks the pair. Untyped tokens from older releases
are classified by their signed expiration lifetime during the compatibility window. The OpenAPI
response now matches the actual `expires_at` response field. The dashboard CLI now opens a durable revocation store for `session` and `oidc` modes.
The default `file` and explicit `sqlite` backends use `.relicta/relicta.db`; PostgreSQL uses a
new `token_revocations` table. Refresh-token claims are unique inserts and pair logout is
transactional, so separate server instances sharing PostgreSQL observe the same revocations.
Pair logout is idempotent when refresh rotation has already claimed the refresh token, so the
access-token revocation still commits during a refresh/sign-out race.
Database read/write failures reject authentication rather than falling back to process memory.
SQLite and PostgreSQL migrations are embedded as migration 008; PostgreSQL manual mode must be
migrated before startup. The HTTP server's library constructor retains its in-memory fallback for
embedders that do not supply a store. Server shutdown and every `Start` return path now stop the
token cleanup worker before the CLI closes its persistence connection, and repeated close calls
are safe. `go build ./...` and `go vet` for the token, persistence, HTTP server, and CLI packages
pass; tests and live multi-process persistence checks were not run.
OIDC state and browser exchange tickets remain process-local, so session affinity is still needed
for the login, callback, and ticket-exchange portion of OIDC. `docs/dashboard.md` now distinguishes
that short-lived OIDC requirement from durable access-token revocation.


The `build-with-frontend` target previously selected `embed_frontend` without any tagged Go source
embedding `cmd/relicta/frontend`, leaving the shipped binary without the UI. A tagged embed file
now mounts the copied Vite output into the CLI server. `make build-with-frontend` succeeds, and a
local run of that binary served the dashboard index with HTTP 200 and returned the configured auth
mode with `Cache-Control: no-store`. No public release was created.

## DONE: container releases follow the verified binary release

The GHCR workflow previously published every `v*` tag independently of the binary release. The
container build now runs as a downstream job in `release.yaml`, so it starts only after GoReleaser
has published the release assets and their artifact attestations successfully. It pushes the
multi-platform image by digest without release tags, scans that exact registry digest for HIGH and CRITICAL
OS and library vulnerabilities on both architectures, publishes GitHub provenance and a Syft SPDX
SBOM attestation for the digest, then adds the customer-facing release tags. The supply-chain guide
documents `gh attestation verify` for consumers. A failed scan can leave an untagged registry
manifest, but no release tag points to it. The plan command is validation only in this repository;
CGP policy approval is not configured here.

A live check on 2026-11-25 confirmed `ghcr.io/relicta-tech/relicta:4.2.0` exists for linux/amd64
and linux/arm64, but `gh attestation verify oci://...:4.2.0` returns 404 because that image
predates this workflow's GitHub attestation steps. The supply-chain guide now states that the
verification command applies only after these controls are merged and used for a release; the
existing image is not retroactively attested.

The first local build also exposed missing binary build metadata: the container reported
`relicta dev` even when built from a release tag. Docker builds now accept `VERSION`, `COMMIT`,
and `DATE` arguments and apply the same linker variables as GoReleaser; the tag workflow passes
the stripped semantic version, source SHA, and UTC build time. A local image built with explicit
metadata reports `relicta 4.2.0`. The quick-start now includes a GHCR invocation that mounts the
current repository, runs as the host UID/GID, and asks operators to pin a reviewed version for
production.

## DONE: public positioning describes the release-lifecycle boundary

The README compared Relicta with Vanta, Drata, Microsoft AGT, and semantic-release using a static
feature matrix, and claimed Vanta/Drata could not produce release provenance and that Microsoft
AGT covered runtime only. Current vendor materials describe AI agent visibility/control in Vanta
and Drata and runtime governance in Microsoft's toolkit, so the blanket gaps were no longer a
credible distinction. The README now explains Relicta's release-lifecycle scope, names the
adjacent platforms as complementary, and links their current product descriptions. Removed the
unsubstantiated "60% of PRs" lead-in and unsupported 99.1% / stale 80.7% coverage claims; the
audit-chain feature is described by its implementation and report-time integrity check instead,
and the coverage badge now points to CI enforcement without implying a fixed percentage. Removed
the repeated Vanta capability-gap claim from the integrations list as well.

## DONE: dashboard local startup matches its documented security guard

The server refused its own default command: configuration selected `:8080` with authentication
disabled, while the production guard correctly rejects unauthenticated non-loopback binds. The
documented `relicta serve` quick start therefore failed despite being described as a local
dashboard. The default address and `--port` now bind to `127.0.0.1`; external exposure requires
an explicit address and configured authentication. Dashboard docs now describe all four auth
modes (none, API key, session, OIDC), the loopback default, and the refusal of unauthenticated
remote binds. Config, CLI, and HTTP server package checks pass.

## DONE: workflow summaries report the result CI actually observed

The integration job's summary checked `$?` after writing its heading, which always succeeds;
failed integration tests could therefore be described as passed. The summary now reads the
test step's recorded outcome. Benchmark pipelines now enable `pipefail`, so a failed benchmark
command cannot be hidden by `tee`; the binary-size comparison uses arithmetic, and trend
history uses timezone-aware UTC timestamps. Workflow shell outputs are quoted. `actionlint`
passes across `.github/workflows/*.yaml`.

Removed the withdrawn AI-022 entry from `.nox/baseline.json`. A fresh Nox 1.70.0 scan on
2026-11-25 completed its code, dependency, IaC, AI, data, and supply-chain scopes, but exited
nonzero because one check did not complete; its 35 findings include 28 potential dependency
vulnerabilities and one OIDC-permission observation. The scan does not establish reachability,
and its static base-image inventory still reports OpenSSL/zlib from the pinned Alpine digests even
when later Dockerfile layers upgrade them. The CLI Dockerfile now upgrades those packages in
both the build and runtime stages. On 2026-11-25, Trivy 0.76.2 scanned the local ARM64 runtime
image `relicta-readiness:local` against its cached vulnerability database (updated 2026-11-24
19:04 UTC) and found zero HIGH/CRITICAL OS or Go-binary findings; package inventory confirms
OpenSSL 3.5.9-r1, zlib 1.4-r0, and libxml2 2.13.9-r3. The same scan found zero HIGH/CRITICAL OS
findings in the local ARM64 frontend image `relicta-web-readiness:local`, whose Dockerfile includes
the runtime package upgrades. These local image results do not cover the release workflow's
linux/amd64 artifact. The updated builder stage's `apk upgrade` layer completed in a subsequent
build, but BuildKit failed to commit its next layer with a read-only metadata error while disk
space was exhausted. Only 221 MiB remained during the current audit, so the rebuilt builder image
and its package inventory still need verification. Official Alpine 3.25 and corresponding language
images were unavailable in the registry during the check, so the base digests remain pinned to
3.24. The security and CI workflows now both pin Nox 1.70.0 by a SHA-256 taken from its checksum
manifest, which was verified against the tagged GitHub Actions release identity with Cosign. CI
had been using Nox 1.26.0 and piping its download directly into `tar`; it now verifies the pinned
archive before extraction. Both workflow gates match the existing `make security-scan` policy and
fail on new HIGH/CRITICAL findings; the
tracked-tree scan with that threshold exits successfully with zero findings (143 findings were
filtered or suppressed). The unfiltered local scan still reports medium/low findings for review.
The baseline records why the five `workflow_dispatch` checks are non-production/manual and why
the release image's OIDC permission is needed for digest attestations. `tools/gendocs` no longer
accepts arbitrary output paths; it writes only to its documented `docs/cli` destination. Local
Nox also reported 16 optional plugins already installed on this workstation but not declared in
the project's required plugin list; those checks were skipped, so `--fail-on-degraded` is not
enabled for clean CI runners. `go build ./...`, `go vet ./...`, and `govulncheck ./...` completed
successfully in the earlier readiness pass; no tests were run in this pass.

A fresh full tracked-tree scan on 2026-11-25 with Nox 1.70.0, `--severity-threshold high`, and
the repository baseline completed successfully: the console reported zero findings and 143
suppressed. The JSON report classifies 120 entries as baselined, one as suppressed, and 22 with
unknown status (21 informational and one low severity). Nox reported 783 dependencies and 9 AI
components. Optional API-abuse, third-party plugin, dependency-freshness,
dependency-confusion, and exposed-endpoint checks did not run; live attack checks also remain
outside this static scan. This is evidence for the configured CI gate, not evidence that all
optional or runtime checks pass.

On 2026-11-25, a fresh full Nox scan reported 158 findings across 797 dependencies and 9 AI
components. It identified a new low-severity IAC-123 on the frontend asset copy in the root
Dockerfile. That build-stage copy now explicitly sets owner `0:0`. A follow-up IaC-only scan no
longer reports that line; it still reports two active low-severity infrastructure findings among
39 results, with 37 suppressed. That narrow scan did not assess code, dependency, AI, data,
secrets, or supply-chain risks, and the analysis-capability report shows call-graph, taint,
reachability, and runtime questions were not answered. This closes one finding only; it is not a
clean security assessment. `docker build --check .` passes with no warnings. A fresh local AMD64
image build completed the edited copy layer and frontend build but stalled with the Go compile
idle for over five minutes; that build was stopped, so it does not verify a complete fresh image.

## DONE: zero-config AI selects a model for the detected provider

`Load()` seeds Viper with the general `gpt-6.2-sol` model default before detecting credentials.
The provider auto-detection branches then checked whether `ai.model` was empty, so those checks
never passed during a normal load. An OpenAI key therefore missed its intended fast model, and
other providers could inherit the general OpenAI model. Auto-detection now sets provider-specific
fallbacks unconditionally; environment overrides retain precedence. Azure OpenAI no longer guesses
a deployment name and reports that `ai.model` or `RELICTA_AI_MODEL` is required. Defaults and
examples now use current OpenAI, Anthropic, and Gemini model IDs; reasoning models omit the
unsupported temperature parameter. The existing auto-detection unit tests exercise the helper
without `setDefaults()`, so the full loader path still needs behavioral coverage. Tests were not
run in this pass.

## DONE: Azure OpenAI auto-detection uses the current v1 endpoint

Azure resource-root endpoints discovered from the environment now normalize to `/openai/v1`,
which uses standard OpenAI routing without a dated API version. Existing legacy dated Azure mode
remains available for manually configured resource-root endpoints. Legacy deployment-shaped
environment endpoints are reduced to their resource root before v1 normalization to avoid the SDK
duplicating `/deployments/{model}`. Provider setup guidance now describes deployment-name
configuration and the current endpoint shape. Build and `git diff --check` passed; tests were not
run in this pass.

## DONE: AI provider guidance reflects current setup choices

The provider guide now recommends using the selected provider's current model catalog and
deployment names instead of carrying stale model IDs, price/context comparisons, and unsupported
privacy or compliance claims. Setup examples and wizard/security guidance clarify environment
key references and plaintext config risks. Sources were checked against official provider docs.

## DONE: release skill defaults to a reviewable dry run

The `relicta-release-governance` skill's quick start previously ran `init`, `bump`, `approve`, and
`publish` directly, and presented a live `relicta release` command before its dry-run alternative.
That could modify a customer's repository or publish artifacts when the agent was only asked to
plan. The quick start now starts with read-only checks and `plan --analyze --dry-run` plus
`release --dry-run`; live release commands are explicitly authorization-gated, and config
initialization is identified as a write requiring confirmation. The release dry-run's limitations
(no generated notes and no governance evaluation) are stated, and non-persistent dry-run plans are
not followed by stateful `bump` or `notes` commands. A static check rejects unsafe live release
commands in the quick-start or outside the authorized section and runs in `make skill-preflight`.
The full skill preflight passed on 2026-11-25, including policy validation and policy-matrix
assertions.

## DONE: first-release guides separate preview from execution

The README called `release --yes` a CI/CD example directly beside its dry run, while the first-time
guide said to execute `release` immediately after preview. The CLI help confirms `--yes` bypasses
approval. Both guides now tell users to review the proposed version, customer-facing notes, changed
files, and configured destinations before running a live release; they describe possible local and
external effects. CI guidance uses `plan` for routine read-only checks and reserves `--yes --ci`
for an intentional unattended-approval policy. The README also clarifies that action input
`auto-approve: false` only helps when a runner can provide interactive approval. CLI help was
checked against `bin/relicta release --help`; documentation diff checks passed.

The README's step-by-step release flow also skipped `relicta bump` before notes and approval, even
though the MCP `relicta_notes` tool requires a versioned release. The sequence now applies the
version before notes and tells users to pass an agent-authored draft after planning and bumping.
The order matches the documented MCP state-machine requirement. A full workflow review also found
`release --dry-run` called the version-file writer; it now uses the existing non-writing planner.
The README and quick-start accurately state that release dry-run does not generate notes or
evaluate governance. The CLI package builds, the skill safety check passes, and targeted diff
checks pass; tests were not run. The skill preflight's static check also guards the dry-run branch
so it must call the version-file planner and writer only on the real-release path.

## DONE: Govee release-note dogfood from technical commits

On 2026-11-25, a read-only Relicta analysis of `govee-light-management` at tag `v3.1.2` found nine
commits and recommended patch `3.1.3`. The changes are security and dependency maintenance, a
security-scan workflow update, and removal of an unused music-mode helper; the recommendation
contains no new features and no customer-facing light-control change. The Relicta MCP flow was
then dogfooded end to end against a fresh temporary clone using a binary built from this worktree:
`relicta_plan` returned a structured recommendation, `relicta_bump` selected `3.1.3`, and
`relicta_notes` saved this agent-authored draft:

> ## Maintenance update
>
> This release updates a networking dependency to address a reported security issue. It also
> refreshes the project’s security and build tooling. There are no new light controls or changes
> to saved profiles in this update.

The notes response returned the matching release ID and version, `state: notes_ready`, and
`agent_authored: true`; a following `relicta_status` returned the exact saved text. The checkout
had no tracked changes after the flow. No approval, tag, or publication was performed. The flow
works for this maintenance-only range and shows the value of session context: Relicta supplies
versioned, auditable facts, while the agent turns them into concise customer copy and can omit
internal commits. The draft remains a local dogfood artifact, not an approved Govee release note.

## DONE: current Go and dashboard production builds pass

On 2026-11-25, `go build ./...` and `go vet ./...` completed successfully in the current
worktree. The dashboard production command `npm run build` also passed its `vue-tsc --noEmit`
check and Vite production bundle. This confirms the current Go packages and embedded-dashboard
source compile together at their respective build boundaries; it does not verify runtime behavior,
release signing, or external publishing. Tests were not run.
Refreshed validation on 2026-11-25 after the latest readiness changes again passed `go build
./...`, `go vet ./...`, the web dashboard's `npm run build`, the MCP app's `npm run build`,
`actionlint .github/workflows/*.yaml`, and `git diff --check`. No tests were run.

## DONE: supply-chain example uses the published archive name

A live read of the `v4.2.0` GitHub Release asset list showed that the Linux archive is named
`relicta_linux_amd64.tar.gz` (lowercase OS). The supply-chain guide's example used
`relicta_Linux_amd64.tar.gz`, which would make the copy-pasted download fail. Updated the example
to match the published asset. The exact asset name was confirmed against the release, and the
edited documentation passes `git diff --check`.

## VERIFIED: published Linux archive passes the documented provenance checks

On 2026-11-25, downloaded the published `v4.2.0` Linux amd64 archive, checksum manifest, and
Sigstore bundle to a temporary directory. The guide's Cosign identity and issuer constraints
returned `Verified OK`; the unique archive checksum passed using the documented macOS
`shasum -a 256 -c -` variant; and `gh attestation verify <archive> --repo relicta-tech/relicta`
completed successfully. Re-ran the GitHub attestation checks with
`--signer-workflow relicta-tech/relicta/.github/workflows/release.yaml`; both the archive and
checksum manifest verified against that exact workflow. The quick-start and supply-chain guide
now constrain provenance verification to that workflow. The first local checksum attempt used
this macOS host's `sha256sum`, which is not GNU-compatible and rejected `--check`; the guide's
platform-specific `shasum` instruction worked. No archive was extracted or installed.

## DONE: release tags must match Relicta's proposed version

The release workflow's prior `plan_valid` gate ran `relicta plan --json` and then set the output
true unconditionally if the command exited successfully. It did not compare the plan to the tag,
and the current release tag was already visible to Git, so Relicta could see no unreleased commits.
The gate now checks the tag's semantic-version shape, deletes only that tag's local ref in the
throwaway GitHub runner, runs a non-writing JSON plan against the preceding release, and fails
unless `next_version` exactly matches the pushed tag. It still explicitly distinguishes version
validation from CGP approval because this repository has no governance policy configured.
`actionlint .github/workflows/release.yaml` and `git diff --check` passed. The live publication path
was not run.

The release-tag gate was also exercised in an isolated local clone: created a lightweight `v5.0.0`
tag at HEAD, deleted that local tag exactly as the workflow does, and ran the same Relicta
`plan --dry-run --json` path. It reported `current_version: 4.2.0`, `next_version: 5.0.0`, and
`release_type: major`; the clone had no `.relicta` release-state files afterward. This validates
the CLI/tag interaction and dry-run state behavior locally. GitHub Actions execution is still needed
to validate hosted-runner integration.

## DONE: container verification gates binary publication

The release workflow previously published the binary GitHub Release before building and scanning
its container image. A failed image build or scan could therefore leave customers with a partially
published version. The workflow now builds the image by digest, scans both supported architectures,
and publishes provenance and SBOM attestations before the binary release job can start. The
customer-facing image tags are added only after both image verification and the binary release
succeed. A failed later binary release can leave an untagged, attested image digest in GHCR, but no
release version tag points to it. `actionlint .github/workflows/release.yaml` and
`git diff --check` passed; live publication was not run.

## DONE: quick-start sets an accurate expectation for publishing

The quick-start promised a first release "in under 5 minutes," although a hosted publication can
require publisher setup, credentials, generated-note review, and approval. Changed the opening to
promise the actual quick-start outcome—setup, plan review, and understanding the lifecycle—and to
state that hosted publishing depends on a configured publisher and credentials. The zero-config
init and first-run plan succeeded in a fresh temporary Git repository without AI credentials;
`git diff --check` passed.

## FIXED: dry-run plans report first-release baselines

A fresh repository with no version tags produced `first_release: false` under `plan --dry-run`, even
though a persisted plan correctly reported `true`. The CLI had derived this descriptive field from
the persistence result, which is intentionally empty in dry-run mode. The release analyzer now
carries whether it resolved a prior version tag, and both text and JSON previews combine that with
persisted state. The relevant Go packages build and the diff is clean; the updated runtime output
was not exercised and no tests were run.

## DONE: MCP plan preview and bump inputs match their behavior

`relicta_plan` exposes its existing non-persisting analysis path as
`dry_run: true`; the recommendation remains available in the response while
release state and durable recommendation data are not written. Its persistent
default is unchanged. The MCP guide explains that a later persisted plan
re-analyzes the repository and should be compared with the preview before
saving notes.

The MCP `relicta_bump` schema advertised `level` and `version`, but its adapter
silently ignored them. Explicit semantic versions now reach the bump use case.
Explicit major/minor/patch levels calculate a version from the planned current
version and update the aggregate's recorded bump kind before versioning; `auto`
keeps the plan's recommendation. The guide now matches the actual `level` key
and states that MCP bump writes configured version files and advances release
state but does not create a Git tag. The same audit found that the advertised
`prerelease` and `build` inputs were also dropped. They now pass through the
handler, use the shared security validators, apply prerelease identifiers with
the CLI's simple-channel counter behavior, and append build metadata.
Build-only overrides retain the planned version on first releases, whose
current baseline is `0.0.0`.

`go build ./...`, `go vet ./...`, `make fmt-check`, and `git diff --check` passed.
Tests and a runtime release flow were not run, so persistence and version-writer
effects still need isolated runtime verification before production use.

## DONE: MCP agent-authored release notes follow approval

The MCP release workflow now supports the use case where technical commit history
is converted into customer-facing prose by the calling agent: `relicta_plan`
returns deterministic recommendation facts, `relicta_notes` persists the draft,
and approval can replace the saved text before approving it. Replacement notes
use the normal notes use case, remain in `notes_ready`, and are saved before the
approval transition, so a failed notes write cannot approve the prior draft by
mistake. MCP bump, notes, approval, and publish also enforce the plan's pinned
HEAD; a changed repository must be replanned.

MCP bump uses configured version-file targets through the existing renderer.
The release-summary prompt and MCP workflow example now describe agent-authored,
customer-facing notes rather than assuming provider-generated technical notes.
The `relicta_notes` result returns the persisted note text together with its
release ID, version, and resulting state, so an agent can confirm which draft
was saved before presenting it for review. The tool now advertises a matching
MCP output schema and returns typed structured content in addition to the text
representation. The release ID and version in that result come from the saved
aggregate, not from the pre-write status snapshot.
`relicta_status` also exposes the saved notes when present, so an agent can
read back and review the persisted draft before approval.
The end-to-end MCP guide example now presents the saved notes and governance
result for review, calls `relicta_approve` only after the user's approval, and
requests separate authorization before publishing. This aligns the example
with the approval state and documented human-elicitation gate.

`go build ./...`, `go vet ./...`, and `git diff --check` passed during the
implementation pass. The 2026-11-25 Govee dogfood recorded above exercised the
agent-authored plan→bump→notes→status workflow against a fresh clone using the
current binary and confirmed that the exact draft persisted with structured
release ID, version, state, and authorship fields. That verifies draft creation
and readback for a maintenance-only range; approval and publication remain
intentionally unperformed.

The default restrictive MCP autonomy budget requires a human cosigner for
approval and publish. Accepted approval elicitation is now recorded as a manual
human approval and satisfies the publish cosigner check; without elicitation,
the actor budget still controls whether agent approval is permitted. Publish
dry-runs do not require a cosigner because they make no release changes. The
live Govee dogfood exercised the stdio MCP client path for planning, bumping,
agent-authored notes, and status readback. It did not invoke approval. At that
stage the human approval elicitation and publish-cosigner transition remained
unverified; the later isolated stdio approval check below closes the elicitation
gap, while publish-cosigner behavior remains unverified.
The approval confirmation now includes the risk score and exact notes text
(using a supplied replacement draft when present), so the cosigner sees the
content that will be published before accepting.
Notes persistence now uses the same per-release lock as approval, without
holding it during remote note generation. Before saving, notes reload the
aggregate under lock and reject a changed release snapshot. Approval reloads
the aggregate after acquiring that lock. MCP human cosigns pass a SHA-256
of the reviewed text; approval checks it under lock and rejects the operation if
another draft replaced the reviewed one. This closes the stale-snapshot and
concurrent-notes-update race in the review-to-approval boundary. The Go build,
vet, and diff checks pass. The Govee dogfood did not exercise approval
elicitation, the text-hash cosign boundary, or concurrent notes/approval
requests. Later isolated stdio work verified elicitation and cosigner denial;
the hash-mismatch and concurrent-update branches remain unverified before
production use.

## VERIFIED: production dependency advisories are clear

On 2026-11-25, `govulncheck ./...` reported no vulnerabilities in reachable Go
code. `npm audit --omit=dev --audit-level=high` reported zero production
dependency vulnerabilities for both `app/` and `web/`. These checks cover the
current dependency advisories, not behavioral correctness, development tooling,
container base images, or the still-open keyless signing and MCP runtime
verification work above.
Re-running all three commands on 2026-11-25 against the current worktree again
reported no reachable Go vulnerabilities and zero high/critical production npm
dependency findings in either frontend package.
On 2026-11-25, reran `govulncheck ./...` and `npm audit --omit=dev
--audit-level=high` in both `app/` and `web/` against the current worktree;
all three reported zero findings.

The same readiness pass rebuilt and ran the ARM64 container image. It reported
the expected version, served the embedded dashboard, and ran as UID 1000. The
image now carries the verified security contact in OCI metadata and copies the
CLI into the runtime stage with `appuser` ownership. `actionlint`, shell syntax
validation, and `git diff --check` passed. A Nox scan reported 159 total items:
36 informational dependency notices, three low infrastructure notices, and
120 suppressed or baselined items, with no active medium, high, or critical
findings. The remaining low notices flag root-owned `COPY` instructions in
build-only Docker stages; those files are consumed by the build and are not
copied as root-owned application files into the runtime image. Nox's optional
Intelligence endpoint timed out in the earlier scan, and several optional
reachability and dynamic checks were unavailable or inconclusive, so this is
not a comprehensive security certification.

On 2026-11-25, the complete current worktree passed `go build ./...`, `go vet
./...`, the Vue dashboard production build, and the desktop app bundle build.
`actionlint .github/workflows/release.yaml`, `git diff --check`, and
`scripts/validate-release-snapshot.sh` also passed. The snapshot check validated
the Linux amd64/arm64 and Windows amd64 archives, embedded dashboard assets,
SPDX SBOMs, generated Linux Homebrew formula, and all six checksum entries.
These are build and artifact checks; tests were not run in this pass, and the
hosted release workflow has not yet been exercised against the unpushed worktree.

## DONE: MCP actions can stay bound to the reviewed release

The MCP workflow previously looked up the active release for each operation.
Agents could not carry the run identity they had inspected in `relicta_status`
into note drafting, approval, or publication. Those tools now accept an
optional `release_id`; status accepts one too, and returns the selected ID. The
ID is passed to the existing run-scoped use cases, so the normal release lock,
plan snapshot, HEAD, approval, and publish validations still apply. The legacy
repository fallback honors the same status selection and returns the ID and
saved notes. The MCP guide documents the workflow.

`go build ./...`, `go vet ./...`, and `git diff --check` passed. The Govee
dogfood verified the single active-run path and status readback, but did not
exercise multiple simultaneous release runs or explicit `release_id` selection.
That targeting flow still needs isolated runtime verification before production
use.

The follow-up fallback-provenance change is now carried through the application
status output and MCP response as `notes_source` plus `notes_warning`. Human
approval confirmation repeats the warning when the saved fallback draft is the
text being approved. The MCP guide places that behavior under the approval tool
and shows the warning-bearing status response. Package builds and diff checks
pass. The earlier runtime gap was closed by the isolated stdio check below.

## VERIFIED: MCP stdio approval and fallback-note warning

Follow-up stdio verification on 2026-11-25 supersedes the earlier runtime gap.
The MCP dependency is now v1.30.1, whose stdio transport supports elicitation
requests. In an isolated temporary Git repository, a live JSON-RPC stdio
session planned and bumped a release, saved deterministic fallback notes with
no AI credentials, read back `notes_source: basic` and the customer-facing
rewrite warning through status, delivered that warning with the exact notes in
the approval elicitation, and accepted the human confirmation. A second session
without elicitation capability confirmed the cosigner-required denial is
returned as a readable MCP tool error and leaves the release unapproved.
Budget-denial paths for approve, publish, cancel, and reset now use the same
user-facing error mapping. `go build ./internal/mcp ./internal/cli` and focused
`git diff --check` pass. These checks were manual isolated runtime checks, not
automated tests; actual publish side effects and multi-run approval/publish
targeting through the MCP JSON-RPC handlers remain unverified.

## FIXED: MCP bump accepts an explicit release ID

The run-targeting workflow previously skipped `relicta_bump`: status, notes,
approval, and publish accepted `release_id`, but bump always advanced the latest
run. Bump now accepts the selected release ID, uses that run when calculating
an override, passes it into the versioning use case, and returns the ID it
advanced. The MCP guide documents the same binding. `go build ./internal/mcp
./internal/cli`, `go vet` for both packages, and focused diff checks pass. A
disposable runtime smoke check created two planned runs, bumped the older one by
explicit ID, and verified the other stayed planned. The response returned the
selected ID and current/next versions; the repository's latest pointer followed
the run just updated. A follow-up disposable runtime check also saved agent
notes to the selected versioned run and queried status for both IDs, confirming
the second run remained planned and note-free. This validates multi-run
selection for bump, notes, and status through the application adapter. A
follow-up below covers approval, dry-run publish, cancel, and reset.

## VERIFIED: MCP lifecycle actions target the selected run

Cancellation and reset now accept `release_id`. Cancellation persists through
the same release-services repository that supplied the run, and reset loads and
deletes from that repository instead of requiring the separate legacy store.
Publish results now repeat the selected release ID. `relicta_evaluate` also
accepts a release ID and evaluates the run selected through status.

A disposable runtime harness created four planned runs. It verified that a
stale notes hash rejects approval without changing state, the reviewed hash
approves only the selected run as `mcp-cosigner`, and publish dry-run targets
that approved run without publishing. It then canceled and reset a separately
selected run while a fourth stayed planned. A separate live stdio JSON-RPC
session in an isolated Git repository discovered the registered tools, planned
a release, bumped it by explicit ID, read the selected run's status, and
canceled that same run; each response carried the same run ID. A second
JSON-RPC session started with two persisted planned runs, bumped one by ID,
verified the other remained planned, and canceled both by their IDs. The second
run was seeded directly in the isolated file store because `relicta_plan`
correctly returns the existing active run instead of creating another one. The
JSON-RPC run-selection path is verified for status, bump, and cancel; multi-run
approval/publish targeting and actual publish side effects remain unverified.
`go build ./...`, `go vet ./...`, and `git diff --check` pass. Temporary
harnesses and repositories were kept outside the worktree.

## IN PROGRESS: publish binary releases only after provenance succeeds

The tagged release job previously let GoReleaser publish a public GitHub release
before the following artifact-provenance step. It now asks GoReleaser to create
a draft (reusing that draft on workflow reruns), skips its Homebrew publisher,
attaches provenance to the archives and checksum manifest, and only then clears
the draft flag. This prevents customers
from downloading a public binary release when provenance generation fails.
GoReleaser publishes the Homebrew formula during the same publish phase, which
would otherwise point the tap at assets still hidden in a draft; the existing
tap is now updated only after the GitHub release is public. The dedicated
publisher derives the Linux amd64/arm64 formula URLs and SHA-256 values from the
same attested checksum manifest and replaces the existing formula through the
tap's contents API. This avoids a second build and removes the macOS formula
entries until signed/notarized binaries are available. Container tags remain
downstream of successful binary release publication. The workflow has not been
run on GitHub, and the formula publisher has not been exercised against the live
tap, so hosted credentials, API update behavior, and Homebrew install behavior
still need validation. The workflow checks that the tap token is configured
before starting GoReleaser, so missing credentials cannot leave a newly
published binary release paired with a stale formula. The formula update now
runs in a separate downstream job, receiving only the checksum manifest; a tap
API outage no longer makes the core release job fail or prevents verified
container tags from being published. The Homebrew job only runs for stable
semantic-version tags, preserving the existing tap behavior that skips
pre-releases and preventing build metadata from entering Homebrew's version.
Before writing to the tap, the downstream publisher verifies the transferred
checksum manifest's GitHub provenance, requiring both the release workflow
identity and exact source tag. The formula generator supports a local `--dry-run`
that renders the proposed Ruby without contacting GitHub, and a successful tap
update records the commit link in the Actions summary.
The local dry-run initially failed on macOS's system Bash 3.2 because the
publisher used Bash 4's `mapfile`; checksum collection now uses a compatible
read loop. On this macOS host, the dry-run produced a syntactically valid Ruby
formula for both Linux architectures, and each generated SHA-256 matched the
validated release snapshot manifest. `bash -n`, `ruby -c`, snapshot validation,
and `git diff --check` passed. No tap write or hosted release was performed.
Review also found the custom publisher omitted Homebrew's explicit Linux-only
platform requirement, unlike the GoReleaser-generated formula. The publisher
now emits `depends_on :linux`, and the snapshot validator requires it so a
future formula cannot silently lose that platform restriction. Homebrew's
current `brew audit` CLI refused a direct path argument, so package-manager
audit of the generated custom formula remains unverified. The generated file
now includes Homebrew's typed/frozen-string headers and `brew style` passes with
no offenses. The snapshot validator also renders the custom publisher's exact
formula path and checks its Linux-only declaration, asset URLs, and SHA-256
values against the manifest, ensuring the hosted snapshot job covers the file
that the downstream tap job will publish. The enhanced validator passes on the
current snapshot. On 2026-11-25, reran it against the existing GoReleaser
snapshot: Linux amd64, Linux arm64, and Windows amd64 archives all contained
the embedded dashboard; all three SPDX SBOMs parsed; six manifest entries
matched their artifacts; and the custom Homebrew formula's URLs and hashes
matched the manifest. This remains a local snapshot check, not a hosted
workflow run or tap install.

On 2026-11-25, reran the workflow's pinned GoReleaser v2.20.3 snapshot locally
into an isolated temporary directory, preserving the existing `dist/` artifacts.
The first attempt hit a missing entry in the shared Go compiler cache; a retry
with a fresh temporary `GOCACHE` completed in 3m9s. The hosted snapshot
validator then confirmed the embedded dashboard in Linux amd64, Linux arm64,
and Windows amd64 binaries; parsed all three SPDX SBOMs; matched all six
checksums; and verified the generated Linux Homebrew formula against the
manifest. The formula dry-run output passes Ruby syntax checking. Both
GoReleaser configurations pass `goreleaser check`, all GitHub Actions workflows
pass `actionlint`, and focused `git diff --check` passes. This verifies local
build and artifact contents only; GitHub-hosted Actions, provenance
attestations, release publication, and tap installation remain unverified.

## DONE: direct-download provenance is bound to the selected release tag

The verified install examples previously checked that archives came from the
Relicta release workflow without binding that provenance to the tag whose assets
were downloaded. The pinned Windows instructions now require the checksum
signature identity and archive provenance to match the selected `$version`.
The Linux instructions resolve the latest release tag once, download every
asset from that tag, and bind both the checksum signature identity and archive
provenance to its exact source ref. This closes cross-tag substitution between
otherwise valid Relicta release artifacts. `git diff --check` passed; the shell
example syntax was reviewed, but no release was downloaded or installed.

## DONE: supply-chain guide pins archive and image provenance to release refs

The supply-chain guide's standalone archive verifier still accepted any release
tag signed by the release workflow. It now validates a stable version tag,
matches the checksum signature identity to that exact tag, and requires the
archive and checksum-manifest GitHub attestations to use the same source ref.
The image instructions now distinguish Docker's version tag from the Git tag
and pin attestation verification to the corresponding `refs/tags/v...` source.
`git diff --check` passed. The commands have not been run against a newly
published release; the current v4.2.0 container still has no provenance.

## FIXED: plugin troubleshooting uses the registry installer

The “GitHub plugin not found” troubleshooting section linked to guessed
`relicta` release asset names such as `github_linux_x86_64` and
`github_darwin_aarch64`. Those are not the documented plugin installation
path and can yield a Relicta CLI archive under a plugin filename. The guidance
now uses `relicta plugin list --available`, `plugin install`, and `plugin enable`,
then explains the required operator review and `--allow-untrusted-plugins` flag
for loading unsigned plugins. `git diff --check` passed; no plugin was installed
or executed.

## FIXED: remove an unsupported plugin-disable environment variable

Troubleshooting suggested `RELICTA_PLUGINS=""` to skip plugin execution, but a
repository-wide source search found no production reader for that variable. The
instructions now show the supported `.relicta.yaml` setting and note that the
same change can be made in a file selected with `--config`. `git diff --check`
passed; no release or plugin command was run.

## DONE: plugin catalog matches the live registry

A read-only `relicta plugin list --available --refresh` check found 29 plugins
across five categories; the plugin guide listed 20 and omitted nine registry
entries. The catalog now links all 29 and includes setup notes for Telegram,
Helm, Dart/Flutter, Swift Package Manager, winget, Linear, Azure Container
Registry, Amazon ECR, and Google Artifact Registry. The configuration reference
now records their settings and credential variables. `plugin info` was checked
for each addition, and `git diff --check` passed. No plugins were installed,
enabled, or executed.

## VERIFIED: agent-session customer notes round-trip over MCP

Dogfooded the complete stdio MCP flow against a disposable HueHouse repository
with a tagged baseline and two technical conventional commits. `relicta_plan`
returned the deterministic recommendation (minor release, sunset scheduling
feature, restart-persistence fix); `relicta_bump` selected 1.1.0 without
creating a tag; the calling agent then wrote two customer-facing bullets from
those facts and passed them to `relicta_notes`. The tool returned
`agent_authored: true`, `source: agent`, and the exact draft. `relicta_status`
read back the same text in `notes_ready`. No AI provider, publish operation,
remote write, or change to the user's repository was involved. The current Go
binary build and `git diff --check` passed; tests were not run.

## VERIFIED: local GitHub Action bundle is clean; PR remains stale

On 2026-11-25, rebuilt the adjacent `relicta-action` worktree at local commit
`57c2e13` and confirmed the tracked tree stayed clean. `npm run lint`,
`npm run build`, `npm audit --omit=dev --audit-level=high` (zero findings),
`git diff --check`, and a generated-bundle scan for source maps and local
checkout paths passed. A fresh GitHub read still shows PR #7 open and draft at
`9db954cd54de8d98ca10e967e5aecac7adbb7932`; that hosted head does not contain
this local bundle or the newer Node 24 and Sigstore changes. No push, PR update,
review, merge, or release was performed. The published Action remains a
production launch blocker until the updated branch is reviewed and released.
The current PR diff also contains checked-in source maps whose `sources` entries
expose private temporary checkout paths. The maintained local build already
omits source-map generation; its bundle normalization step now fails if `.map`
files appear anywhere under `dist` (including nested output). A fresh local
bundle build, lint, recursive map-file scan, temporary-path scan, and
`git diff --check` passed. The guard is still local and
must be included in the reviewed PR update before that path-leak finding is
closed.

On 2026-11-25, a fresh GitHub read confirmed PR #7 remains open and draft at
`9db954cd54de8d98ca10e967e5aecac7adbb7932`. The separate local Action branch is
at `57c2e13ea95347d27bff3b12c2dd6769b8476c44`; its source-map guard remains an
uncommitted local change. After that change, lint, formatting, bundle build,
production-only dependency audit (zero findings), recursive source-map scan,
private-checkout-path scan, and `git diff --check` pass. These results do not
cover the hosted PR head. No push, PR update, review, merge, or release was
performed.

## VERIFIED: CGP dogfood on Relicta's own release plan

Enabled strict CGP governance in the local `.relicta.yaml` with human review
required for breaking and security changes, and rebuilt the current CLI. A
read-only `relicta plan --dry-run --json --no-ai --skip-cognitive` against this
worktree produced a deterministic recommendation for 5.0.0 and a critical
`approval_required` verdict (risk 0.8). The preview identified 168 commits, one
breaking change, 15 security-related changes, and a blocking human-approval
obligation. The versioned release policy is loaded from
`.github/relicta/policies`; its major, breaking-change, and high-risk rules all
match this plan. This validates local plan-time CGP visibility. The dry-run did not
persist a release run or create release artifacts. The release workflow now
fails closed for rejected, missing, or unknown CGP decisions, and the publishing
job is bound to the `release-approval` GitHub Environment. A read-only GitHub
settings check found only the unprotected `copilot` environment;
`release-approval` does not exist yet. The release job now reads the environment
configuration through GitHub's Actions API and fails closed unless it finds at
least one required reviewer with self-review disabled. Its `GITHUB_TOKEN` is
limited to Actions read for that check; publication permissions remain separate.
The environment still provides the approval pause before the job starts, while
the API check rejects a missing or unprotected environment before release
credentials or publication steps are used. The workflow uses this environment
for every release, including decisions CGP marks approved, so the reviewer gate
is unconditional. `actionlint` and focused `git diff --check` pass. No settings
were changed and no hosted run was performed.

## VERIFIED: production Docker image builds and serves the embedded dashboard

On 2026-11-25, `docker build --progress=plain --platform linux/amd64 -t
relicta-readiness:local .` completed successfully; the emulated Go compile took
345 seconds. The resulting image reports `linux/amd64`, starts as UID/GID
1000 (`appuser:appgroup`), and `relicta version` returns `relicta dev`. Running
the image with API-key authentication served the dashboard HTML from `/` over
HTTP, confirming the frontend is embedded. Startup logged the expected limited
functionality warning because the container had no Git repository mounted; the
container was removed after the smoke check. `docker build --check .` also
passes. This is local AMD64 image evidence only; the hosted container release
workflow and published image remain unverified.

## FIXED: Docker build-stage copies declare ownership

A fresh full Nox scan on 2026-11-25 reported 157 findings across 797
dependencies: 38 active (36 informational dependency advisories and two low
confidence IAC findings), with no critical or high findings. The two IAC
findings were the frontend package and source copies in the Docker build stage.
Both now use explicit `COPY --chown=0:0`, consistent with the root-owned build
steps. A follow-up IAC-only scan reports no active low findings, and
`docker build --check .` passes without warnings. The scoped follow-up does not
recheck other security areas. In the full scan, call-graph analysis was
inconclusive for 74 findings and attacker reachability, dynamic verification,
and dependency reachability were not evaluated; this is not a claim that the
application is vulnerability-free.

## VERIFIED: external release gates still need configuration and publication

Read-only GitHub checks on 2026-11-25 found `v4.2.0` (published 2026-07-10)
is still Relicta's latest release. The repository exposes only the `copilot`
Actions environment; the configured repository secret-name listing contains
only `HOMEBREW_TAP_TOKEN`, so the required `release-approval` environment and
Apple signing/notarization credentials are absent. In `relicta-action`, PR #7
is still open and draft at `9db954cd54de8d98ca10e967e5aecac7adbb7932`, and
the latest published Action release is still v2.5.0 (published 2026-03-15).
These are live external launch blockers: local workflow validation cannot
configure GitHub protections, provide Apple credentials, or publish the fixed
Action. No repository settings, PR state, or releases were changed.

A follow-up on 2026-11-25 confirmed the `release-approval` environment endpoint
returns 404; the environment preflight will therefore fail closed before any
release credential or publishing step. PR #404 remains draft at
`00854edd39280c637818a0c29ab2c75b69b64184`, with its October 28 Nox Security
Scan and CI Security Scan failing; dependency PR #405 remains mergeable with
its hosted checks passing. The local Linux/Windows GoReleaser snapshot now
completes from the current source, and its dashboard embedding, SPDX SBOMs,
checksums, and generated Homebrew formula validate. In the separate Action
checkout, Node 24 source formatting, lint, bundle generation, production npm
audit, workflow lint, and bundle path/source-map checks pass locally; however,
PR #7 still points at its older Node 20 head, remains draft, and its passing
hosted checks do not cover those newer changes. Nothing was pushed, reviewed,
merged, or published.
The supply-chain guide now documents the required reviewer-protected
environment, least-privilege Homebrew token, and all-or-none Apple secrets so
maintainers can configure the external release gates accurately.

## VERIFIED: Relicta dogfood policy is tracked and validated

Added `.github/relicta/policies/release.policy` so the repository's strict
release-manager approval rules are versioned with the project and discovered by
`relicta policy list`; `.relicta.yaml` points `policy_dir` at that tracked
directory. The breaking-change policy checks the analyzed count
(`change.breaking > 0`), since the field is numeric. Strict policy validation
passes. The dry-run still recommends 5.0.0 and requires approval at critical
risk (0.8), with one
breaking change among 168 commits. Its policy verdict reports all three
matching rules: major release, breaking change, and high risk. This remains a
non-persistent preview, not an approved or published release.

## FIXED: release-note CLI guidance matches the available workflow

The README advertised an unsupported `notes --save` flag, and the quick-start
example called `-a` without its required audience value. Removed the phantom
flag, documented the actual audience, tone, language, and output flags, and
corrected the example to `relicta notes -a public`. The CLI help and generated
command reference now explain that CLI AI prose requires `ai.enabled` plus a
configured provider, while an MCP caller can supply customer-facing notes from
its existing product context. `relicta notes --help`, `go build ./...`, focused
`git diff --check`, and a search for the stale flag/example passed. No tests
were added or run.

## FIXED: release-notes recording scripts use supported commands

Updated `cmd-notes.sh` to pass an audience value, write file copies with
`--output`, and show JSON output instead of unsupported save and dry-run flags.
It now states that AI notes require an enabled, configured provider and cleans
up its temporary output. Updated the messy-history demo to run the real notes
command without hiding failures behind fabricated customer prose; its summary
now describes optional configured AI notes without promising specific claims.
`bash -n` and focused `git diff --check` pass. The scripts were not executed
against the worktree.

## FIXED: recording demos distinguish AI output from deterministic fallback

The tutorial that demonstrates provider-backed notes now requires an
`OPENAI_API_KEY` and checks Relicta's reported source before calling the output
AI-generated. The notes command demo no longer attempts an AI request against
the default AI-disabled fixture. The old duplicate AI-versus-template script
now delegates to the maintained deterministic/MCP workflow demo. Shell syntax
checks and `git diff --check` pass; no recording scripts or tests were run.

## FIXED: release communication and preview guidance matches behavior

The README now describes `communicate` as using AI when configured with
template-based fallback. The quick-start no longer lists `notes --dry-run` as a
read-only preview; it explains that notes generation saves release-note state.
The publish recording script's `--skip-tag` flag was verified against the CLI
definition and is supported; its flag summary now also includes skip-approval
and skip-plugins. Documentation and script syntax checks pass; no tests or
recording scripts were run.

The README command flag table also had `communicate --audience -a`, while the
CLI accepts `--audiences` without a short form. Corrected that entry, documented
the notes emoji flag, and included the publish skip controls. The flag names
were checked against their Cobra definitions.

The README described AI-written “professional changelogs” as a core default,
but fresh configuration disables AI. Reworded this as optional AI-enhanced
release-note prose and clarified the deterministic default in the quick-start.
The default was checked in `DefaultConfig`.

The dashboard README described OIDC sign-in without its multi-replica
constraint. It now states that session affinity is required during login,
callback, and ticket exchange because the temporary OIDC handoff state is
process-local; this matches the deployment guide.

## FIXED: unsigned-attestation override cannot bless an unverifiable signature

The verifier previously treated `--allow-unsigned` as permission to accept a
signed attestation without a public key, then reported it as valid. The option
now applies only when the attestation has no signature; signed attestations
without a configured key fail closed. The existing regression case and CLI
reference now describe that boundary. Source review and `git diff --check`
passed; tests were not run.

## FIXED: EU AI Act retention guidance no longer invents deadlines

Primary-source review found that the Annex IV export calculated a ten-year
"deadline" from the report period, while Article 18(1) measures the provider's
specified documentation period from placing the system on the market or
putting it into service. Relicta does not collect that event date. Replaced the
calculated deadline with explicit Article 18(1) guidance and the limitation.
Article 12 exports now label period-end plus six months as a suggested export
date, not a legal deadline, and identify the distinct provider/deployer rules
in Articles 19(1) and 26(6), including intended-purpose and other-law caveats.
Updated Markdown, CLI help/reference, README, MCP framework metadata, and
existing source assertions/setup. Checked against the official [Regulation
(EU) 2024/1689, Articles 18, 19 and 26](https://eur-lex.europa.eu/legal-content/EN/TXT/?uri=CELEX:32024R1689).
`go build ./internal/compliance ./internal/cli ./internal/mcp` and
`git diff --check` pass. Tests were not run.

## FIXED: Article 12 exports disclose their evidence scope

A product-scope review found that the Article 12 report and GRC integrations
called release-governance decisions "Article 12 logs" and described proposal
and decision timestamps as AI-system use periods. The underlying records do not
capture every AI-system use or automatically generated operational logs. The
report now labels the data as release-governance supporting evidence and states
that it cannot alone establish Article 12 logging duties. Timestamps and change
metadata are described according to what Relicta actually records. The scope
warning travels with each JSONL/CSV row and each Vanta/Drata payload; the
integration descriptions and framework metadata now identify the mapping as
limited supporting evidence. README and CLI documentation use the same product
claim. Existing source assertions were updated but tests were not run. The
compliance, CLI, MCP, Vanta, and Drata packages build; `git diff --check` passes.

## FIXED: Annex IV draft no longer invents target-system compliance claims

The Annex IV generator populated customer reports with unverified target-system
purpose, architecture, monitoring, risk controls, standards status, and a
conformity declaration date. Those assertions were not sourced from customer
configuration or evidence. The generator now includes only observed version,
CGP protocol version, release lifecycle records, and risk categories inferred
from governance rationales; it leaves provider documentation and standards
status for completion and verification. The Markdown draft marks missing
fields TODO and explains that post-market metrics describe Relicta release
history rather than monitoring of the target AI system. Existing source
assertions were updated. Formatting, builds, and vet are checked separately;
tests are not run.

Annex IV follow-up: the JSON report now carries the same explicit evidence
scope in a top-level `evidenceScope` field as the Markdown report, so automated
consumers do not have to infer the limitation from generation notes. When no
risk categories are inferred from governance rationales, Markdown now states
that this does not establish absence of risk. Existing source assertions were
updated; no tests were run.

## FIXED: Annex IV entry points describe a supporting-evidence draft

Although the report now discloses its evidence limits, the README, CLI help,
MCP framework catalog, and report headings still called it full Annex IV
technical documentation. Updated these entry points to identify it as a draft
of release-governance evidence mapped to Annex IV, and stated that it is not a
complete target-system technical or conformity assessment. MCP metadata now
publishes that scope and uses `release_governance_record` as the evidence type.
The full local Go build, focused vet, and diff checks pass; tests were not run.

## FIXED: Article 12 timestamps name the release records they describe

The scoped Article 12 export still exposed `startedAt` / `endedAt`, described
as proposal-to-decision timestamps even though the values came from recorded
release duration and release or decision timestamps. That naming could still
be mistaken for an AI-system use interval. Renamed JSON fields and CSV columns
to `releaseProcessStartedAt` / `releaseRecordedAt`, changed Markdown table
labels, corrected comments and CLI guidance, and updated Vanta/Drata payloads.
The evidence-scope text now matches the recorded values. The planned 5.0.0
customer release-notes draft calls out the schema change. Source assertions were
updated; tests were not run.

## FIXED: Article 12 export no longer self-asserts chain verification or approvals

A deeper source-to-output trace found the report initialized
`auditChainIntegrityVerified` to true without loading the audit chain, never
populated approval identities or record hashes, and rendered missing verifier
data as an autonomous decision in both GRC integrations. The report now
returns `auditChainIntegrityStatus: not_checked`, points users to
`relicta audit verify`, describes empty verifier data as unavailable, and
states that approval events and chain hashes are not included. CSV no longer
advertises empty policy, model, verifier, or hash columns; the JSON/Markdown,
README, CLI help, and Vanta/Drata descriptions use the same scope. The planned
5.0.0 release-notes draft documents the schema changes. Source assertions were
updated; tests were not run.

## FIXED: background event workers follow their owning context

The repository-wide linter identified background webhook and dashboard event
work that was detached from its owning context. Persistent webhook workers now
derive their cancellation from the container or command context, and dashboard
event publishing receives the server context. Also resolved the remaining
repository-wide lint findings in OIDC URL checks, JWT date access, header-copy
naming, and deprecated compatibility comments. Formatting, `go vet ./...`, and
`golangci-lint run ./...` pass. Go and npm dependency scans found no high or
critical vulnerabilities; tests were not run.

## FIXED: release artifacts are bound to the reviewed commit

The tag-triggered workflow checked the recommended version but relied on a
movable tag ref for later job checkouts. Jobs now check out the immutable event
SHA. A shared release-tag guard verifies that both the local and remote tag
still resolve to that SHA during governance validation and before binary,
GitHub Release, Homebrew, and container-tag publication. This prevents a tag
move between planning and publication from relabeling artifacts built from a
different commit. `actionlint`, ShellCheck, shell syntax validation, and
`git diff --check` pass; publishing was not run.

## FIXED: Action CI builds with the declared Node runtime and verifies formatting

The reusable Relicta Action declares Node 24, but its CI and release workflows
still selected Node 20. Both now use Node 24. The example workflow quotes its
computed Git revision in the tag command. Action CI now checks formatting
without rewriting generated files and fails on unexpected dirty bundle output.
`actionlint`, the Action lint/build/format checks, `git diff --check`, and a
production dependency audit pass locally. These workflow changes are not
published; the existing Action PR remains open and draft. Tests were not run.

## FIXED: provider health checks follow the HTTP request context

The observability provider endpoint called every configured provider with a
non-cancellable context, allowing a slow health check to outlive a disconnected
request. The handler now passes `r.Context()` through the service interface to
the provider registry. Updated implementations and compile-time call sites;
`go build ./...`, focused `go vet`, and `git diff --check` pass. Tests were not
run.

## FIXED: webhook incident processing follows request cancellation

Webhook incident callbacks now receive the incoming request context, and the
observability service and CGP correlator pass it through to storage and
correlation work. This prevents slow persistence or correlation from continuing
after the sender disconnects. Updated callback implementations and call sites;
`go build ./...`, focused `go vet`, and `git diff --check` pass. Tests were not
run.

## FIXED: webhook failures no longer receive a success acknowledgment

The receiver previously returned HTTP 200 after parsing even when incident
persistence or correlation failed, which could cause alert providers to discard
incidents Relicta had not recorded. Incident callbacks now return errors; the
receiver logs failures and responds HTTP 500 so senders can retry. Persistence
errors and CGP correlation errors are returned by their respective handlers, and
a focused regression test asserts the retryable status. `go build ./...`, focused
`go vet`, and `git diff --check` pass. Tests were not run.

## FIXED: incident processing failures request sender retries

A follow-up review found that the webhook receiver acknowledged parsed alerts
with HTTP 200 even when its callback failed. `IncidentHandler` now returns an
error, the receiver responds HTTP 500 on processing failure, and the observability
store and CGP correlation adapter propagate their failures. Added a regression
test for the retry response. Build, focused vet, and diff checks pass; tests were
not run.

## FIXED: provider retries reuse an incident identity

Webhook providers may omit incident IDs. The observability store derived a
fallback, but the CGP callback received an empty ID and failed; retries could
also receive a new ID when a missing start time defaulted to the current time.
The receiver now derives a stable ID from provider, alert content, labels, and
provider-supplied start time, excluding delivery time. Generic payloads preserve
a missing start time as unknown and reject malformed supplied timestamps. Added
a retry-identity regression test. `go build ./...`, focused `go vet`, and
`git diff --check` pass. Tests were not run.

## FIXED: webhook requests enforce the configured body limit

The receiver read at most 5 MiB but did not distinguish an oversized body from
one exactly at the limit. It now reads one byte beyond the configured maximum
and returns HTTP 413 when exceeded, so oversized payloads are rejected rather
than truncated and misclassified as malformed. Datadog alerts with no event
timestamp now keep the timestamp unknown, preserving stable derived retry IDs.
Added a body-limit regression test. `go build ./...`, focused `go vet`, and
`git diff --check` pass. Tests were not run.

## FIXED: observability configuration fails fast on unusable providers

A typo in `health_check.provider_name` previously allowed startup but left every
release watch unmeasured. The service now verifies that the selected provider
is registered, and Prometheus endpoints must be absolute HTTP(S) URLs without
embedded credentials (which could otherwise appear in status output). Added
regression tests for unknown provider names and invalid endpoints. `go build
./...`, focused `go vet`, and `git diff --check` pass. Tests were not run.

## FIXED: provider status checks run concurrently with a bounded timeout

The providers endpoint checked configured backends sequentially. Each Prometheus
client can wait 30 seconds, so one unavailable backend could hold the endpoint
past its server write deadline and prevent later providers from being checked.
The registry now checks providers concurrently, returns statuses in stable name
order, and bounds each check to 10 seconds while still honoring request
cancellation. `go build ./...`, focused `go vet`, and `git diff --check` pass.
Tests were not run.

## FIXED: health-watch recovery is idempotent and cancellation cleans up state

The dashboard starts watches from both in-process publish events and a recovery
poll, so the same release could produce duplicate-start warnings. Duplicate
starts now preserve the active watch and return success. A watch that exits
because its context is cancelled removes its own active entry, allowing a later
recovery attempt to restart it. Updated the existing duplicate-start assertion.
`go build ./...`, focused `go vet`, `golangci-lint`, and `git diff --check` pass;
tests were not run.

## FIXED: recovered health watches keep the original release window

When the dashboard restarted, it started a fresh monitoring window for a release
that had already been published, allowing unrelated later metrics to be
attributed to it. Both recovery polling and in-process publish events now anchor
the watch and its displayed expiry to the publication timestamp, and the first
health check runs immediately. Releases whose monitoring window has already
expired are not restarted. `go build ./...`, `go vet ./...`, and
`git diff --check` pass; tests were not run.

## VERIFIED: Govee customer notes round-trip through an agent session

Dogfooded the MCP workflow against a disposable clone of Govee Light
Management's committed history; its source checkout and uncommitted user changes
were left untouched. The deterministic plan recommended patch 3.1.3 from
v3.1.2 across seven commits, with no breaking changes or new features. After
persisting the plan and bumping without creating a tag, the calling agent wrote
a concise user-facing note centered on dependency and automated security
maintenance. `relicta_notes` returned `agent_authored: true` and `notes_ready`,
and `relicta_status` read back the same draft bound to the release. No approval,
tag, or publish was run. This validates the intended “technical changes in,
customer notes out” workflow on a real project history.

## FIXED: dashboard lint gate passes cleanly

The dashboard CI lint step failed on an unused router-mode assignment and
reported 52 template-formatting warnings in the login and settings views.
Simplified the auth-mode guard and formatted those two views. `npm run lint:check`
now exits successfully with no warnings, and `npm run build` passes its Vue type
check and Vite production bundle. The production-only npm audits for both the
dashboard and MCP app report zero vulnerabilities; the MCP app typecheck passes.
Tests were not run.

## VERIFIED: current production dependency and repository security gates

The dashboard and MCP app production-only npm audits report zero
vulnerabilities. Nox 1.70.0 scanned the current tree and 797 dependencies at
the CI high-severity threshold, reporting zero active findings and 286
baseline-suppressed findings. Core checks covered secrets, code, dependencies,
infrastructure, AI integrations, sensitive-data handling, supply chain,
authentication/authorization, and webhook signatures. The pinned
`nox/depconfusion@0.2.4` and `nox/attack-surface@0.3.0` plugins also completed
with no high-severity unsuppressed findings. Explicit public npm registry
settings were added to both app workspaces, removing all dependency-confusion
findings for their scoped packages. The scan still has 167 unsuppressed
below-threshold leads (101 informational and 66 medium), chiefly endpoint
exposure and configuration findings. Router review confirms the dashboard API
routes sit inside the configured auth middleware group; the static endpoint
plugin does not model that Chi group boundary and reports potential exposure.
The intentionally public health, auth, docs, and ticket-gated WebSocket routes
also appear in that list. Dependency freshness and live adversarial testing
were not run. The API-abuse plugin was tried but fails on this repository's
large single-line generated MCP HTML (`bufio.Scanner: token too long`), even
when the generated directory is excluded from the project scan; it is not
required in CI until that plugin can handle the input. These results are scoped
to the checks listed and do not establish that all security risks are absent.

The project config now queries OSV.dev directly and uses Nox's signed bundled
slopsquat feed, so a scan does not depend on Klarlabs Intelligence being
reachable or transmit dependency metadata there. Re-ran the expanded scan with
`--fail-on-degraded` under an isolated Nox home to model a clean GitHub runner;
it exited successfully after all configured checks completed. The regular
developer machine has extra globally installed plugins that are not enabled
for this repository, so strict mode reports those as advisory local coverage
gaps. Both per-change CI and the scheduled security workflow now fail if an
enabled check is incomplete.

## FIXED: dashboard deployment guidance sets an explicit availability boundary

The dashboard guide described the embedded dashboard as suitable for production
enterprise use after mentioning OIDC and PostgreSQL, while separately noting
that OIDC state and browser exchange tickets are process-local. Those statements
could lead operators to infer that shared PostgreSQL supports horizontally
scaled OIDC. The guide now states that Relicta does not provide a fully highly
available dashboard deployment. PostgreSQL-backed session revocation can be
shared across replicas, while OIDC handoff state and live WebSocket/SSE
broadcasts remain instance-local; OIDC requests need affinity and the complete
experience is best run as one instance. It also calls out the TLS proxy and
network restriction expectations for remote access. This aligns the
market-facing deployment promise with current runtime behavior.

## FIXED: cognitive health reports initialized adapters

`/health/cognitive` returned `enabled` for Mnemos and Chronos unconditionally,
including when either integration was disabled or no adapter was initialized.
The serving CLI now passes the container's initialized-adapter state into the
HTTP server, and the endpoint reports `disabled` or `enabled` from that state.
The dashboard guide clarifies that `enabled` means initialized, not reachable;
the adapters intentionally degrade gracefully, so this endpoint does not claim
an external connectivity check. `go build ./internal/httpserver/... ./internal/cli/...`
and `git diff --check` pass. Tests were not run.

## FIXED: standalone dashboard image proxies the Relicta API

The frontend Docker image previously served only static files even though the
README advertised it as a deployable standalone dashboard. Browser API and
WebSocket requests had no route to a Relicta server. The image now expands a
runtime `RELICTA_API_URL` into its Nginx template and proxies `/api/` traffic,
including WebSocket upgrades, to that backend. The deployment guide documents
the separate authenticated backend requirement and a container run example.
`docker build --check` and a full frontend image build pass; a local container
smoke check served the dashboard and health endpoint and confirmed the runtime
backend URL was rendered into the active Nginx configuration. No automated
tests were run.


## FIXED: readiness audit dates no longer point into the future

Several entries in this log were stamped 2026-11-26 even though the current
date is 2026-11-25. Corrected those verification dates so the release-readiness
record does not claim future checks. The live GitHub release and PR states were
rechecked on 2026-11-25 during this review.

## VERIFIED: the 5.0.0 review PR still fails its hosted security gates

On 2026-11-25, the open draft release-governance PR #404 remained at
`00854edd39280c637818a0c29ab2c75b69b64184`. Its latest CI and Security
Analysis runs are from 2026-10-28 and fail both Nox scans with 71 dependency
findings against that older commit. The dependency-update PR #405 changes Go,
web, and app lockfiles plus the Nox baseline; its hosted checks pass and GitHub
reports a clean merge state. The release-governance work therefore still needs
to be brought onto the dependency-updated base and its security gates rerun
before it can be treated as release-ready. Neither PR was changed or merged.

## RECHECKED: current UI builds pass; hosted publication remains stale

On 2026-11-25, the MCP app's full build (`vue-tsc` plus all six bundled apps)
and the dashboard's typecheck and Vite production build both passed. Their
production-only npm audits each reported zero vulnerabilities, and the current
worktree passes `git diff --check`. A fresh GitHub read still shows v4.2.0 as
the latest Relicta release, PR #404 open as a draft at
`00854edd39280c637818a0c29ab2c75b69b64184`, and PR #405 at
`72d31c6b83744f6f11100dd8f41b15145c23cc13`—the dependency-update base, not
this worktree's uncommitted readiness work. These local checks do not validate
the hosted release workflow or publish a new version. No tests, PR updates,
merges, or releases were performed.

## VERIFIED: release snapshot and artifacts validate on three targets

On 2026-11-25, `make release-local` completed from the current worktree using
`.goreleaser.non-macos.yaml`, producing Linux amd64, Linux arm64, and Windows
amd64 archives and SPDX SBOMs. `make validate-release-snapshot` verified that
all three binaries embed the dashboard, parsed each SBOM, matched all six
checksum entries, and confirmed the Linux Homebrew formula URLs and digests
match the archives. `git diff --check` passed. This excludes macOS signing and
notarization, Sigstore publication, hosted approval, and upload; those gates
remain separate. No tests or release publication were performed.

The tag-triggered release workflow previously skipped this validator and moved
from GoReleaser directly to provenance attestation. It now runs
`scripts/validate-release-snapshot.sh dist` before attestation, so required
archives, embedded dashboard files, SPDX SBOMs, checksums, and the Linux formula
are validated in the actual publish path. `actionlint .github/workflows/release.yaml`,
the local snapshot validator, and `git diff --check` pass; hosted workflow
execution remains pending.

The release job now also verifies the checksum signature against the exact
tag-specific GitHub Actions identity and verifies GitHub provenance for every
archive and `checksums.txt` before making the draft public. The command forms
were exercised against the public v4.2.0 checksum bundle and Linux archive:
Cosign accepted the exact workflow identity, the archive digest matched the
signed manifest, and `gh attestation verify` accepted both the archive and
manifest for the release workflow and tag. `actionlint` and `git diff --check`
pass. This validates the verification commands against an existing release;
the new release workflow itself still needs a hosted run.

## RECHECKED: GitHub Action bundle is clean; hosted PR is older

On 2026-11-25, the adjacent `relicta-action` checkout at local commit
`57c2e13` rebuilt its ncc bundle successfully, with no generated bundle diff.
Prettier source checks, ESLint, `actionlint`, and `git diff --check` passed;
the production-only npm audit found zero vulnerabilities. The recursive bundle
scan found no source maps or local worktree paths. The local tree includes the
source-map rejection guard and Node 24 workflow updates. GitHub still reports
PR #7 as an open draft at `9db954cd54de8d98ca10e967e5aecac7adbb7932`, so its
hosted checks do not cover those local changes. No tests, push, PR update,
review, merge, or Action release was performed.

## FIXED: release demo wrapper no longer forwards unused shell arguments

A low-threshold local Nox scan flagged the compatibility wrapper as passing
untrusted arguments to `exec`. The maintained child demo accepts no arguments,
so forwarding them had no useful behavior; the wrapper now invokes it without
`$@`. A code-only Nox scan of the wrapper reports zero findings, `bash -n` and
`git diff --check` pass. No tests were run. The same scan found three medium
Alpine advisories for `expat` 2.10.0-r0 in the pinned
`nginx:1.30.8-alpine3.24` image. The package inventory shows `libexpat` is
pulled in only through `fontconfig`, which is required by `libgd` for the
optional `nginx-module-image-filter`; the dashboard Nginx configuration does
not load or use that module. The Docker build now removes the unused module,
allowing APK to remove the expat dependency chain. The final image package
inventory and vulnerability scan still need a successful image build; Docker's
local daemon was unavailable in this environment. Debian's own tracker also
marks the underlying expat CVE vulnerable, so switching distributions would not
clear the issue. Alpine v3.24 currently provides expat 2.10.0-r0, while its
fixed 2.11.0-r0 package is on a different branch.
The full low-threshold scan also emitted route-exposure leads for health probes
and authenticated API routes; router inspection confirms the release, group,
governance, webhook-management, and observability APIs are inside the shared
authentication middleware group, while the public health endpoints intentionally
serve probe responses.

The earlier local dashboard build and smoke check covered the API proxy change,
before the later `apk del nginx-module-image-filter` layer was added. That latest
package-removal change still needs a rebuilt image and final package inventory;
the local Docker daemon is currently unavailable. The Dockerfile now fails its
image build if `libexpat` remains installed after module removal. The dashboard's
web CI builds the image and scans its OS and library dependencies for HIGH and
CRITICAL vulnerabilities on web pull requests and main-branch updates.
`actionlint` and `git diff --check` pass locally; the hosted workflow has not run
on this change.

## RECHECKED: local release gates pass; publication branches remain separate

On 2026-11-25, the current Relicta worktree passed `go build ./...`,
`go vet ./...`, the dashboard and MCP app production builds, `actionlint` for
all workflows, and `git diff --check`. No automated tests were run in this
pass. The dashboard container workflow and its `libexpat` removal assertion are
local changes; the Docker daemon was unavailable, so that image was not built.

GitHub currently reports PR #405 head `72d31c6b83744f6f11100dd8f41b15145c23cc13`
with CI, Security Analysis, Web Accessibility, Docs, Skill Quality, and
Benchmark checks passing. PR #404 remains a draft at
`00854edd39280c637818a0c29ab2c75b69b64184`; its October CI and security runs
fail because Nox 1.26.0 reports 71 dependency findings. The newer dependency
base includes the passing Nox 1.70.0 gate, but PR #404 has not been updated to
it.

The adjacent Relicta Action worktree at `57c2e13` contains local Node 24,
Sigstore-authenticated checksum, and exact-tag verification changes beyond
public PR #7's head `9db954cd54de8d98ca10e967e5aecac7adbb7932`. Its current
local formatting, lint, bundle build, workflow syntax checks, and production
dependency audit pass; the production audit reports zero vulnerabilities. The
PR's hosted unit and end-to-end jobs pass only for its older head. These local
Action changes and the Relicta readiness work remain unpublished, so neither
project's customer release gates have validated the complete changeset.

## FIXED: patched Alpine packages are applied in every container build stage

The current Nox 1.70.0 security scan reported a high OpenSSL advisory against
the pinned Alpine base packages. The CLI and dashboard Dockerfiles now upgrade
the full installed package set from their maintained Alpine v3.24 repositories
in both build and runtime stages. The dashboard continues to remove the unused
image-filter module first and fails if its vulnerable expat dependency remains.
The `relicta eval` fallback message also no longer resembles a printed API
token to secret scanners; it names provider families while keeping the actual
environment-variable configuration documented elsewhere.

The Nox 1.70.0 scan with the reviewed `.nox/baseline.json` reports zero new
findings across 797 dependencies; `go build ./...`, `go vet ./...`, and
`git diff --check` pass. The strict scan with an empty baseline still reports
known synthetic credentials in test fixtures and the Docker base-image OpenSSL
advisory, which are outside the reviewed baseline; no credential was exposed
by that scan. `docker build --check` could not connect because the local Docker
engine is unavailable, so the APK upgrade and final image package inventory
still require a hosted or daemon-backed build.

The release workflow already builds and scans both published architectures,
and the dashboard has a pull-request image gate. The main Relicta Dockerfile
previously had no pre-merge image build; `.github/workflows/container-security.yaml`
now builds it and fails on HIGH or CRITICAL OS/library vulnerabilities when
container inputs change. `actionlint`, Nox 1.70.0 with the reviewed baseline,
and `git diff --check` pass. The hosted build and Trivy scan have not run on
this local workflow change.

The release PR and dependency PR were rechecked against GitHub on 2026-11-25:
PR #404 is still a draft at `00854edd39280c637818a0c29ab2c75b69b64184`, and
PR #405 remains open at `72d31c6b83744f6f11100dd8f41b15145c23cc13`. The
current container gate, package-upgrade changes, and governance fixes remain
local and are not covered by either hosted PR's checks.

The latest Nox review also removed one obsolete AI-007 baseline exception for
`internal/cli/eval.go`: the provider fallback text that triggered it was changed,
and a strict scan confirmed the finding no longer exists at that location. The
normal baseline scan still reports zero new findings across 797 dependencies
(288 existing findings remain suppressed). No other baseline entries were
changed in this cleanup.

## RECHECKED: local release snapshot and production gates remain clean

On 2026-11-25, reran `scripts/validate-release-snapshot.sh dist` against the
current isolated snapshot. The Linux amd64/arm64 and Windows amd64 archives all
contain the embedded dashboard, all three SPDX SBOMs parse, all six checksum
entries match, and the generated Linux Homebrew formula matches the manifest.
`go build ./...`, `go vet ./...`, `actionlint .github/workflows/*.yaml`, and
`git diff --check` also pass. No tests were run.

A fresh GitHub read confirms Relicta PR #404 remains a draft, PR #405 remains
open, and Relicta Action PR #7 remains a draft. The current readiness changes
are still local and are not covered by those hosted checks. Docker Engine is
unavailable on this workstation, so the latest local APK package upgrades and
container security workflow still need a daemon-backed or hosted run. The OIDC
browser flow still lacks validation against a real configured identity provider,
and the provenance-gated release workflow still lacks a hosted release run.

## FIXED: frontend build stages use patched npm packages

A direct Trivy scan of the pinned Node 24.22.3 Alpine image found six HIGH
findings in npm's bundled `brace-expansion` 5.0.9 and `undici` 6.28.0 packages.
These were present only in the frontend builder stage and were not copied into
either runtime image, but they were still part of the release build toolchain.
Both Dockerfiles now install npm 12.2.3 before running `npm ci`; its Node
engine range includes the pinned Node 24.22.3 runtime, and its bundled copies
are `brace-expansion` 5.0.12 and `undici` 6.29.1. Trivy reports zero
HIGH/CRITICAL findings in the npm package archive, the installed npm 12.2.3
package tree, and `web/`'s dependency lockfile. The same global npm upgrade
command used in the Dockerfiles was exercised in an isolated prefix; npm 12.2.3
then completed `npm ci` and the production dashboard build successfully (host
Node 26; the pinned Node 24.22.3 version is within npm's declared engine range).
The CLI container security workflow and dashboard workflow now build and scan
both their frontend toolchain stages and their final runtime images for HIGH and
CRITICAL OS/library findings; `actionlint`, Nox 1.70.0 with the reviewed
baseline (zero new findings across 797 dependencies), and `git diff --check`
pass. The Docker daemon is unavailable, so neither hosted container build nor
scan has run on this change yet.

## RECHECKED: agent-session notes work through the MCP stdio server

On 2026-11-25, exercised the built Relicta binary through a real MCP
initialize/`tools/call` exchange. `relicta_plan` with `dry_run: true` returned
valid structured content with the 5.0.0 recommendation, all 168 release
change records, and no tool error. The recommendation was 22,720 bytes; the
dry run returned an empty release ID and `recommendation_persisted: false`,
confirming that this inspection did not create release state. The same
commit-based governance assessment reported 520 touched paths across the 168
commits since v4.2.0. The net range diff is 512 paths; governance counts old
and new rename paths and cumulative changes, so the larger scope is expected
and is not counting the uncommitted worktree.

The existing [5.0.0 preparation notes](release-notes/5.0.0.md) cover the
recommendation's strongest customer outcomes: independent monorepo releases,
broader version-file support, persistent release/governance history, and
customer-facing drafts from the agent's session. The MCP draft path remains
available for the planned release flow; no persistent plan, version bump,
approval, or publication was performed during this dry-run check. No automated
tests were run.

The dependency PR description also reported a build failure from TypeScript 7
with `vue-tsc`. The current local app dependencies now use TypeScript 6.0.3;
the resolved Volar integration declares a TypeScript `<7` peer range.
`npm run build --prefix app` passes `vue-tsc --noEmit` and builds all six MCP
apps. This local fix and the generated bundles have not yet been covered by
hosted PR checks. No automated tests were run.
`npm audit --prefix app --json` reports zero vulnerabilities across 211
resolved dependencies.
The current tree also passes `GOMAXPROCS=4 go build ./...`,
`GOMAXPROCS=4 go vet ./...`, and `git diff --check`.

## RECHECKED: hardened local CLI image and dashboard smoke

On 2026-11-25, Docker Engine became available on this workstation, superseding
the earlier local-daemon-unavailable notes above. `docker build --pull
-t relicta-readiness:local .` completed successfully for `linux/arm64`; the
resulting CLI reports `relicta dev` and runs as the unprivileged
`appuser:appgroup`. Trivy's HIGH/CRITICAL scan of OS and library packages
reported zero findings in both the Alpine 3.24.4 image and the Go binary.

Ran that image with a read-only root filesystem, a bounded `/tmp` tmpfs,
dropped capabilities, `no-new-privileges`, PID/memory/CPU limits, and a
loopback-only published port. The dashboard returned 200 for `/healthz`,
`/readyz`, and `/`; `/api/v1/releases` returned 401 without a key and 200 with
the configured API key (empty release list, as expected for this isolated
container). The embedded dashboard HTML was served successfully. The service
logged that no repository was configured and continued in limited-function
mode, which is expected for this isolated smoke configuration. The temporary
container was stopped and removed. Hosted amd64/arm64 builds and their workflow
scans, and a live repository-backed dashboard run, remain outstanding; no
automated tests were run.

## RECHECKED: standalone dashboard image builds and runs hardened

On 2026-11-25, refreshed the pinned Nginx 1.30.8 Alpine base digest in
`web/docker/Dockerfile` and built the standalone dashboard image locally. The
frontend typecheck and production build passed, npm reported zero vulnerabilities,
and Trivy reported zero OS/library vulnerabilities in the final image. A fresh
Nox scan reported the upstream Nginx base's `libexpat` advisory; inspection of
the Docker build confirmed `nginx-module-image-filter` and `libexpat` are
removed before the final image is assembled. The final image package inventory
contains no expat package. The Nox scan's dependency-advisory comparison was
incomplete because the prior scan did not query dependency advisories, so its
reported “new” status is not used as a before/after conclusion.

Ran the dashboard container as UID 101 (`nginx`) with a read-only root,
capabilities dropped, `no-new-privileges`, and bounded resources. Writable
tmpfs mounts for `/run`, `/var/cache/nginx`, and `/etc/nginx/conf.d` are needed
for the stock Nginx entrypoint's PID, cache, and generated proxy config. On a
Docker network with a resolvable `relicta` backend, the static dashboard and
`/nginx-health` both returned HTTP 200. The temporary containers and network
were removed. Added this verified launch configuration to `docs/dashboard.md`
and corrected stale demo and cognitive-health descriptions in
`docs/cli-new-features.md` and `docs/dashboard.md`.

The full Nox scan reports 288 findings (170 active, 118 suppressed), including
68 medium-severity, one low-severity, and 101 informational findings, with no
high or critical findings. This is not a clean security assessment: attacker
reachability, dynamic verification, and dependency reachability were available
but not established by that scan. Separate `npm audit` runs for both `app/`
and `web/` report zero vulnerabilities. Live GitHub reads on this date show
release PR #404 still draft with its October Nox Security Scan and Security Scan
checks failing, while dependency PR #405 remains open with its October checks
passing. Neither includes the current local changes; hosted workflow evidence
and release review remain outstanding. No automated tests were run.

The repository-backed CLI dashboard check was also completed on 2026-11-25
using a throwaway Git repository (one commit and one version tag) mounted into
the hardened `linux/arm64` CLI image. The server initialized without the
limited-function warning, `/healthz` returned 200, and `/api/v1/releases`
returned 401 without a key and 200 with the configured key. The isolated
fixture correctly had no saved release runs. Its temporary container was
stopped. This closes the local repository-initialization smoke gap, but does
not substitute for running against an operator's repository, PostgreSQL, or a
real OIDC provider.

The failed main-branch Security Analysis run `45122265990` (2026-11-23,
commit `247b693b00fd761e607697d7ba38f03accc93c87`) was inspected from its
uploaded findings artifact. It ran Nox 1.26.0 and failed the scan step on 23
unbaselined dependency advisories: 22 high and one critical across the then
committed app/web lockfiles and Go modules. SARIF upload and artifact upload
both succeeded, so this was a finding-gate failure, not a reporting failure.
The current local security and CI workflows pin checksum-verified Nox 1.70.0,
fail only on new high/critical findings, and use the updated lockfiles and
reviewed baseline; the tracked-tree scan passes that gate locally. This
explains the stale hosted failure but does not replace a hosted run on the
current commit. No workflow was dispatched during this check.

## FIXED: release approval now precedes publication-capable jobs

The `release-approval` environment previously gated only the binary release
job, while the dependent image-verification job had already pushed an untagged
GHCR digest and published its attestations. A new least-privilege approval job
now runs after CGP planning and before both image and binary publication jobs;
it verifies that the environment has a required reviewer and prevents
self-review. The release job no longer repeats this gate. The release workflow
also now blocks CGP's `deferred` decision, which means more information is
needed, instead of allowing a generic environment approval to override it.
Tag validation explicitly passes `--no-ai`, so the release version gate does
not rely on optional model credentials or service availability; deterministic
CGP risk evaluation remains enabled. The supply-chain guide describes the new
order. `actionlint` and
`git diff --check` pass. The external `release-approval` environment is still
not configured, so this new hosted path has not run; no image or release was
published and no tests were run.

## FIXED: `plan --no-ai` works without configured provider credentials

Read-only release previews across the 16 configured first-party repositories
found that both Nox checkouts failed before planning: their project config
enables AI and references `OPENAI_API_KEY`, even when the operator explicitly
passes `--no-ai`. Configuration validation now applies that command option
before validating the AI provider settings. A fresh build and dry-run preview
of the Open Source Nox checkout succeeded without an API key, returning a
non-persisted `1.39.2` plan. Across the full matrix, 15 repositories produced
dry-run results and Git status was unchanged in every checkout. Briefkasten
previously failed to initialize because its ordinary `.git` directory enables
Git's `extensions.worktreeConfig`, which go-git's extension-name handling
rejects after lowercasing it before a case-sensitive allowlist check. The local
opener now resolves both linked worktrees and ordinary repositories with the
extension, removes it only from go-git's in-memory config, and restores it if
go-git writes config. The implementation compiles, Relicta's own dry-run still
succeeds, and a temporary ordinary repository with this extension enabled
produced a dry-run plan while its config and working tree remained unchanged.
Briefkasten's checkout is not present at its earlier recorded path, so that
exact repository remains unavailable for direct verification. Do not edit a
user's repository config to make Relicta open it. No plan was persisted and no
automated test was run.

## RECHECKED: dashboard dependency fix builds cleanly; hosted release remains gated

On 2026-11-25, a clean `npm ci --ignore-scripts --no-audit --no-fund` followed
by `npm run build` in `app/` completed successfully with the current TypeScript
6 lockfile, including `vue-tsc --noEmit` and all six bundled MCP apps. Generated
build outputs did not appear in Git status. The live GitHub dependency PR #405
head (`72d31c6`) has passing CI, tests, docs, security/Nox, accessibility,
dependency-review, and skill-quality checks; this validates that PR head, not
the larger local worktree. The latest published release is still v4.2.0 from
2026-07-10. The `release-approval` GitHub environment still returns 404, so the
new protected release workflow cannot yet run successfully in hosted Actions.
No hosted release or publication was triggered.

## FIXED LOCALLY: Relicta Action reauthenticates cached-version downloads

The separate Action checkout previously returned an exact-version binary from
the runner tool cache without rechecking its signed checksum manifest. On a
persistent self-hosted runner, a modified cache entry could therefore bypass
the Sigstore identity and archive-digest checks. The Action now downloads,
authenticates, and verifies every invocation; its user guide describes this
behavior, and the packaged `dist/index.js` was regenerated. Local format, lint,
bundle build, production-only npm audit, workflow lint, and `git diff --check`
pass. The Action's build and end-to-end workflows now explicitly use a
read-only `contents` token; its release example retains write access because it
can perform a real release. The live Action PR #7 remains an older Node 20
draft head (`9db954c`) and does not include this fix; its green checks do not
cover the current local branch. No test suite was run, and no branch was
pushed or published.

## FIXED LOCALLY: Git repositories with worktreeConfig extension open in Relicta

The Git opener supports both linked worktrees and ordinary repositories with
Git's `extensions.worktreeConfig`, without changing repository config. It
resolves private and common gitdirs through the Git CLI, lets go-git use its
standard filesystem layout, strips the unsupported extension from the
in-memory config, and preserves it on config writes. `go build`, `git diff
--check`, Relicta's own read-only dry-run, and a dry-run against a temporary
ordinary repository with this extension enabled all succeeded. The temporary
repository's config and working tree remained unchanged. The Briefkasten
checkout recorded in the earlier investigation is unavailable at its old
path, so verification against that specific repository remains outstanding.
No automated tests were run.

## FIXED LOCALLY: tag-triggered release cannot bypass Relicta approval

The initial correction stopped accepting `approval_required` from the plan preview,
but that treated Relicta's protected GitHub reviewer as separate from the CGP approval
it was meant to provide. The current workflow keeps `approval_required` as a valid
preview outcome only when the protected environment review runs, then calls
`relicta approve` as that verified human and checks the persisted release run and
hash-linked CGP decision before publication. `deferred` and `rejected` previews still
stop before the environment gate. The workflow also transfers the verified chain so
the publication job checks it again. See “FIXED LOCALLY: release approval transfers
verifiable governance history” below. The GitHub environment is still absent, so the
hosted release remains blocked. No hosted settings were changed.

## RECHECKED: local release gates pass; hosted launch remains unconfigured

On 2026-11-25, a clean sample Git repository completed the documented quick
start's `relicta init` and `relicta plan --dry-run` path: init created
`.relicta.yaml`, the plan returned `0.1.0`, and no changelog or persisted
release mutation was created. `actionlint` passed for the GitHub workflows,
both GoReleaser configurations passed `goreleaser check`, `go vet ./...`
completed cleanly, and `scripts/validate-release-snapshot.sh dist` verified
the existing ignored snapshot's Linux and Windows archives, embedded
dashboards, SPDX SBOMs, six checksum entries, and Linux Homebrew formula.
This validates the snapshot currently in the worktree; it is not a fresh
build of the current dirty tree.
The latest public GitHub release remains v4.2.0 (published 2026-07-10). The
hosted release workflow still cannot publish until the repository has a
`release-approval` environment with required reviewer protection; the
documented Homebrew credential and optional Apple signing configuration also
need to be present for their respective distribution paths. No release was
created or published, and no automated tests were run.

The Homebrew formula generator's description still called Relicta "AI-powered",
which no longer matches the product's release-governance positioning. It now
describes Relicta as release governance for AI-native software teams. The
generated Linux-only formula passed a dry-run against the current checksum
snapshot and the release snapshot validator. The public tap still has its
v4.2.0 formula until the next stable release updates it; no tap change was
published.

The planned 5.0.0 release also requires the Go module's major-version suffix
to move from `/v4` to `/v5`; otherwise Go consumers could not resolve the new
major through the module path. The module declaration, all Go source imports,
protobuf package metadata, lint configuration, and current `@latest` install
guidance now use `/v5`. Examples pinned to the published v4.2.0 remain on
`/v4`. The customer-facing 5.0.0 draft explains the import migration and the
now-enforced branch and freeze settings. `go build ./...`, `go vet ./...`, and
`git diff --check` pass; `go install ./cmd/relicta` installs a local v5-path
build that runs and reports `relicta dev`. No old `/v4` references remain in
Go source or build metadata. No automated tests were run. This source
migration remains local and unpublished; the public latest release is still
v4.2.0.

## FIXED LOCALLY: Relicta Action market copy matches current behavior

The Action marketplace metadata and README still described Relicta as
"AI-powered release management" and claimed pinned binaries were cached. The
current product is positioned around release governance, and the installer
deliberately downloads and authenticates every binary on every run. Updated
the Action description and README copy to match both the product positioning
and actual behavior. The Action format check, lint, bundle build, production
dependency audit (zero vulnerabilities), and `git diff --check` pass. The
current Relicta tree also passes `go build ./...`, `go vet ./...`, `go mod
tidy -diff`, `go mod verify`, both GoReleaser configuration checks,
`actionlint`, and the dashboard MCP-app build. No automated tests were run;
changes remain local and unpublished.

## FIXED LOCALLY: metrics listener defaults to loopback

A fresh Nox 1.70.0 scan of the current working tree found 170 findings (118
suppressed) across 797 dependencies. The review surfaced a concrete exposure:
`relicta metrics` bound to `0.0.0.0` by default and serves Prometheus data
without authentication. The CLI now binds to `127.0.0.1` by default; remote
scraping requires an explicit host plus firewall or authenticated-proxy
controls. Updated the generated `relicta metrics` reference accordingly. Nox
also reported medium findings for documented public health/probe routes and
an expat package named by the pinned Nginx base; the image's removal step and
final-image scan are documented above, while the health routes remain public
for probes and the repository-group status route is under authentication.
The local Nox invocation exited 2 because `--fail-on-degraded` found a
degraded check; its report identifies optional locally installed plugins not
declared by this repository, and `nox doctor` reports a missing local
freshness plugin binary. Core scan findings were still emitted, but the scan
is not recorded as a clean, fully complete assessment. The standalone
metrics-host change builds successfully, and `relicta metrics --help` now
confirms the loopback default and remote-binding guidance. A local runtime
smoke on port 19090 confirmed the listener was `127.0.0.1:19090`, `/health`
and `/metrics` returned successfully, and Ctrl-C shut the server down cleanly.
No automated tests were run.

## FIXED LOCALLY: customer-notes workflow is discoverable and accurate

A source check found that the earlier pass changed only generated Markdown,
while the live Cobra help still displayed stale "AI-powered insights" and
"production-ready CLI" claims. The notes command's source help was also
truncated after "customer-", and the generated reference inherited that
truncation. The command source now positions Relicta around agent-session
customer notes and completes the plan → bump → draft handoff; generated CLI
references were rebuilt from Cobra. Root help and `notes --help` were run
from the current source and show the corrected copy. The MCP Govee maintenance
example is under its own heading, labeled as an unapproved dogfood draft, and
the README and CLI reference links resolve. A Go build, `gofmt`,
`git diff --check`, and local link/anchor checks pass. No automated tests were
run.

## FIXED LOCALLY: release approval transfers verifiable governance history

The release workflow already binds its protected GitHub reviewer to the
release run, version, commit, and Relicta plan hash, then records that reviewer
through Relicta's normal approval path. The remaining evidence gap was that the
artifact transferred the release run but omitted the governance memory file
containing the hash-linked evaluation and decision. The approval job could
therefore record a reviewer while leaving the decision history behind.

Planning uses an advisory governance preview and intentionally does not write
a decision audit entry. The first implementation looked for that entry before
approval, so it could never pass on a clean runner. The workflow now checks the
preview for a publishable decision, records the protected human approval through
Relicta, then verifies the real decision entry emitted by that approval. It
transfers only the target repository's hash-linked chain with the approved run;
the publication job receives both and verifies the approved release state.
The publication job reads the governance memory at the same repository-relative
path produced by the artifact download; paths outside `.relicta/` are rejected
so the verifier cannot silently point to state that was not transferred.
`actionlint`, the verifier build, and `git diff --check` pass. An isolated
runtime smoke confirmed the verifier accepts a valid `approval_required`
entry and its hash-linked chain. This remains unverified on GitHub; the required
reviewer environment and hosted release credentials need operational
validation. No hosted settings or release state were changed.
This change has not run on GitHub; the repository's required reviewer
environment and hosted release credentials still need operational validation.
No hosted settings or release state were changed.

## FIXED LOCALLY: Codex MCP setup is documented for agent-session notes

The MCP guide documented Claude Desktop setup but not Codex, despite the
agent-session customer-notes workflow being one of Relicta's primary dogfood
paths. Added Codex CLI registration and connection checks, the working-directory
requirement for selecting the target repository, links to the official Codex
MCP guide, and a reminder that Relicta's MCP surface includes approval and
publishing actions. Checked the command syntax against the installed Codex CLI
help and confirmed the configuration and `/mcp` verification steps against
OpenAI Docs. `git diff --check` passes; no tests were run and no user MCP
configuration was changed.

## RECHECKED: public distribution still reflects the pre-dogfood release

Read-only GitHub checks on 2026-11-25 confirm the latest public release is
still v4.2.0 (published 2026-07-10). The repository currently has a `copilot`
environment but no `release-approval` environment; the only visible repository
secret name is `HOMEBREW_TAP_TOKEN`. Therefore the configured release workflow
cannot pass its required reviewer-protection check, and new macOS signing and
notarization are unavailable. The current v5 source, MCP customer-notes
workflow, and release-governance improvements remain local and are not
represented by the published artifact. No remote settings, refs, or releases
were changed.

## FIXED LOCALLY: release lifecycle docs place Git tagging at publish

A check against `BumpVersionUseCase` and the publisher adapter found two public
workflow tables saying `relicta bump` creates the Git tag. Bumping stores the
selected version and writes configured version files; `publish` creates the
tag, pushes it only when configured, and runs the release steps. Corrected the
README command summary and Quick Start state-transition table to match the
implemented lifecycle. The separate MCP reference already described this
accurately. `git diff --check` passes; no tests or release actions were run.

## FIXED LOCALLY: install instructions point to the published module version

The repository's current Go module declares `/v5`, but the public remote has no
v5 tag; its latest stable tag is still v4.2.0. The README and Quick Start
previously instructed new users to install `/v5@latest`, which cannot resolve
until that major is published. Both guides now install `/v4@v4.2.0` and label
the v5 command as the post-release path. Confirmed against `go.mod`, local tags,
and `git ls-remote --tags origin`; `git diff --check` passes. No tests or
publication actions were run.

## FIXED LOCALLY: MCP release actions distinguish drafts from publication

The MCP handlers show that bumping updates release/version-file state, saving
notes persists an editable draft, and publishing can create/push a tag and run
publisher plugins. Tool descriptions did not make that boundary explicit to
agents at call time. Updated the bump, notes, and publish descriptions plus the
MCP guide and README workflow summary to identify the local preparation steps,
approval step, and publication side effects. `git diff --check` passes; no
automated tests or release actions were run.

## FIXED LOCALLY: customer-notes availability matches the public release

The README and Quick Start documented agent-session notes as available while
the public v4.2.0 binary predates that workflow. The module proxy lists no
published `/v5` versions, while `/v4` ends at v4.2.0. Labeled the workflow in
the README, MCP guide, and Quick Start as available in the unreleased v5 source
only, and directed users to a v5 source build until publication. The current
CLI builds with `go build ./cmd/relicta`; `git diff --check` passes. No tests or
release actions were run.

## FIXED LOCALLY: plugin startup fails when configured restrictions cannot apply

`Manager.loadPlugin` previously warned and continued when command restrictions
failed. Linux and Windows did not implement requested network/filesystem
isolation, and macOS continued unconfined when `sandbox-exec` was absent. The
loader now refuses to start a plugin when requested isolation is unsupported,
macOS reports missing `sandbox-exec` as an error, and configured Linux
resource-limit failures kill the just-started process and abort loading. The
plugin guide documents the platform behavior and remaining Linux post-start
race and the per-command trust opt-in. `.roady/plan.json` now separates the
completed fail-closed behavior from remaining sandbox-hardening work. `go build
./cmd/relicta` and cross-builds for macOS arm64 and Windows amd64 pass;
`git diff --check` passes. No tests were run.

## FIXED LOCALLY: concurrent plugin loaders receive the cached load error

The lazy loader shared a `sync.Once`, but kept the result in a local variable.
When the first attempt failed, a concurrent waiter could finish `once.Do`, see
no loaded plugin, and return `(nil, nil)` because only the first caller had the
error value. The loader now reads the plugin and cached error together after
initialization, and reports an invariant error if neither exists. `go build
./cmd/relicta` and targeted `git diff --check` pass. No tests were run.

## FIXED LOCALLY: lazy plugin startup observes hook cancellation

`ExecuteHook` passed its caller's context into plugin execution, but lazy plugin
startup used `context.Background()`. A canceled release could therefore wait
for process startup and perform plugin work after its caller had stopped
waiting. Hook collection now receives the caller context, passes it into lazy
startup, and returns cancellation even when the plugin is configured with
`continue_on_error`. `go build ./cmd/relicta` and targeted `git diff --check`
pass. No tests were run.

## FIXED LOCALLY: canceled lazy loads can be retried

After lazy startup began observing the hook context, its `sync.Once` cache
would have made a transient cancellation permanent for the lifetime of the
manager. Replaced the separate once/error maps with a per-attempt load state.
Concurrent callers retain the outcome they joined, permanent startup failures
remain cached, and a later request replaces a canceled or timed-out attempt
with a fresh state. `go build ./cmd/relicta` and targeted `git diff --check`
pass. No tests were run.

## PREPARED LOCALLY: Relicta Action fails closed on checksum verification

A fresh public checkout of `relicta-tech/relicta-action` confirmed that its
installer caught checksum download, lookup, and mismatch errors, warned, then
continued to extract and cache the binary. Prepared a change on local branch
`codex/fail-closed-checksum` in `/tmp/relicta-action-readiness`: missing or
malformed SHA-256 entries and mismatches now stop installation; both plain and
`sha256sum`-marked filenames are accepted. Rebuilt the committed `dist`
bundle. `npm run lint`, `npm run build`, and `git diff --check` pass; tests were
not run. The connected GitHub account does not have this repository installed,
so no remote branch, PR, or release was changed. Until the fix is merged and
released, the public Action retains the documented fail-open behavior.

## PREPARED LOCALLY: Relicta Action refreshes the moving `latest` version

The Action used `latest` as a persistent runner tool-cache key, so a
self-hosted runner could continue to execute an older binary after a newer
release became available. The same local Action branch now bypasses cache
lookup and storage for `latest`, while retaining caching for pinned versions;
the README now correctly describes this as the runner tool cache. Rebuilt the
committed `dist` bundle. `npm run lint`, `npm run build`, `node --check
dist/index.js`, and `git diff --check` pass. Tests were not run, and no remote
branch, PR, or release was changed.

## RECHECKED: Relicta Action security work is consolidated on its maintained branch

The Action worktree `fix/fail-closed-checksum-and-approval-default` already
contains a stronger implementation than the separate fresh-clone patch above:
it authenticates the checksum manifest with Sigstore against the expected
Relicta release workflow identity, requires exactly one well-formed checksum
for the selected archive, and downloads/verifies on every run so a mutable
self-hosted runner cache cannot bypass authentication. It also treats requested
plugin installation failures as fatal, moves the Action to Node 24, and makes
the CI formatting check non-mutating. The README and Action metadata describe
the release-governance positioning and preserve the v2 `auto-approve` default.

Rechecked the maintained worktree with `npm run lint`, `npm run format-check`,
`npm run build`, `node --check dist/index.js`, and `git diff --check`; all pass.
Automated tests were not run. The changes remain local and unreviewed; no remote
branch, PR, or release was changed. The public Action therefore retains its
current behavior until this work is reviewed, merged, and released.

## VERIFIED: current-source dogfood across nine repositories and Govee MCP notes

Built the current Relicta source into an isolated binary and ran read-only
`plan --dry-run --json --no-ai` in nine configured repositories. Each returned
a structured recommendation without changing its release state:

- Govee Light Management: 3.1.2 → 3.1.3 patch, 7 commits.
- Roady: 0.27.3 → 0.28.0 minor, 5 commits.
- Specular: 1.11.0 → 1.11.1 patch, 9 commits.
- Nox: 1.39.1 → 1.39.2 patch, 3 commits.
- mcp-rs: first release 0.1.0, 3 commits on its current feature branch.
- TokenOps: 0.120.1 → 0.120.2 patch, 2 commits.
- Dispatch: 0.2.0 → 0.3.0 minor, 36 commits.
- Nomi: 0.3.0 → 0.4.0 major, 1 commit.
- Relicta: 4.2.0 → 5.0.0 major, 168 commits; CGP correctly returns
  `approval_required`, critical risk, and `can_auto_approve: false`.

The umbrella `Developer/open_source/relicta` directory also contains a
`.relicta.yaml`, but it is a Go workspace with no Git repository of its own;
running `plan` there correctly fails with “repository does not exist.” Treat
its nested repositories as the release targets rather than interpreting this
workspace-level config as a supported single-repository project.

For Govee, exercised the MCP server over stdio in an isolated full clone: plan,
bump, agent-authored `relicta_notes`, then `relicta_status`. The saved 3.1.3
draft was returned with `notes_source: agent` and state `notes_ready`; it
described the high-severity dependency audit fix, refreshed security and build
tooling, removed unused music-mode code, and stated that no light controls or
saved-profile changes were included. No approval or publish tool was called.
The pre-existing uncommitted Govee working tree remained unchanged.

An initial Nox attempt used the stale checked-in `bin/relicta` and failed on
its AI-key validation. Rebuilding from current source resolved it: `--no-ai`
works without AI credentials, consistent with the current CLI's config-loading
path. No automated tests were run. No repository release, tag, or publication
was changed.

## INVESTIGATED: GitHub exposes protected-environment reviewer identity

The GitHub Actions REST API has `GET /repos/{owner}/{repo}/actions/runs/{run_id}/approvals`.
Its review objects include the approval state, environment names, and reviewing user. The local
release workflow now queries this endpoint after its `release-approval` gate, requires exactly
one approved review for that environment, and writes the named reviewer to the job output and
summary rather than attributing approval to `github.actor`. The documented response does not
include a review timestamp. The follow-up implementation below carries the persisted run through
the protected job and binds the human approval to its release version, tag, commit, and plan hash.
See GitHub's [workflow-run review history API](https://docs.github.com/en/rest/actions/workflow-runs#get-the-review-history-for-a-workflow-run).

## VERIFIED: dependency scans and reviewer capture are clean locally

On 2026-11-25, `govulncheck ./...` reported no Go vulnerabilities. Production-only
`npm audit --omit=dev --audit-level=low` reported zero vulnerabilities for both `web/` and
`app/`. The changed release workflow passes `actionlint`, and `git diff --check` passes for the
workflow and this backlog update. `go vet ./...` passed; `web/` and `app/` both completed their
production builds, with the MCP app refreshing its six embedded UI bundles. These local results
do not verify the hosted environment's reviewer configuration or an actual tag-triggered release.
`go build ./...` and `actionlint .github/workflows/*.yaml` also passed against the current
worktree. No automated tests or release actions were run.

## VERIFIED: current Relicta plan blocks an unapproved critical release

On 2026-11-25, a local read-only `plan --dry-run --json --no-ai` from this
worktree recommended `4.2.0 → 5.0.0` and returned `approval_required`, critical
severity, risk score `0.8`, and `can_auto_approve: false`. The tag-triggered
workflow now creates and persists the plan, waits for the protected reviewer,
records that human approval into the run, and verifies the plan hash, version,
tag, and commit before publication jobs. The manual dispatch path remains a
read-only GoReleaser snapshot and does not publish. Local workflow and CLI
validation pass; hosted environment configuration and a real tag-triggered
approval remain unverified. No automated tests or release action were run.

The current Nox offline scan reports zero unsuppressed findings across
secrets, code, dependencies, infrastructure, and supply-chain checks (797
dependencies). Three exact Expat fingerprints from the pinned nginx base are
recorded with a rationale because the Docker build removes the module that
brings Expat in and the built final image confirms `libexpat` is absent. This
offline scan does not query current vulnerability advisories; the separate
online scan still needs a clean confirmation after the baseline change.

## VERIFIED: online Nox findings are build-stage or removed-package matches

The online Nox scan on 2026-11-25 queried OSV for 797 dependencies and reported
37 findings (93 suppressed): six root-image OpenSSL advisories and one zlib
advisory are attached to the pinned Alpine base at the source Dockerfile line,
while the final CLI image runs `apk upgrade` and copies only a statically linked
Go binary from the builder. Replayed the final runtime stage's exact Alpine
package-install and upgrade commands directly against the pinned digest; the
only matching crypto/compression package in the final package database is
`zlib 1.4-r0` (the fixed version), with no OpenSSL or Expat installed. The web
image build separately removes `nginx-module-image-filter`, asserts Expat is
absent, and its built final image reports no `libexpat` package. Nox cannot
model these later Docker stage and package-removal operations, so its remaining
medium Expat finding is a source-level base-image match. The exact advisory
fingerprint has a narrow baseline entry with image evidence; other findings
are already baselined or informational. No new dependency or runtime package
upgrade was needed based on this verification.

After reconciling that exact Expat fingerprint with the existing baseline, the
online scan reports 36 findings (94 suppressed) and no issues needing attention;
six low-priority observations remain. A fresh no-cache dashboard image build
also succeeded: the final image has no `libexpat`, `nginx -t` passes, and the
container serves the dashboard index. The scan excludes AI integrations and
sensitive-data checks, so these results cover only the requested secrets,
code, dependency, IaC, and supply-chain scan scope.

Limit: this validates the Alpine package state produced by the pinned base and
current repositories at scan time, not the complete hosted release image,
registry provenance, or production runtime deployment. The full CLI container
build did not finish in Docker Desktop's Go compile stage; host `go build`,
`go vet`, and the isolated runtime package replay pass. No automated tests or
release actions were run.

### Follow-up: CLI runtime image verified with a Linux build artifact

A full multi-stage Docker build remained inside its Go compilation step, so I
built the CLI for Linux on the host and used an isolated temporary Dockerfile
with the production Dockerfile's exact pinned Alpine digest, package install,
upgrade, non-root user, work directory, and health check. The image built and
reported `libcrypto3 3.5.9-r1`, `libssl3 3.5.9-r1`, `zlib 1.4-r0`, and
`libexpat 2.11.0-r0`; the CLI runs as uid 1000 and `relicta version` succeeds.
Keeping the one-shot CLI container alive for the configured Docker probe
produced `healthy` with `relicta dev` output. This validates the final runtime
layer and executable, but does not replace a full multi-stage build or hosted
image provenance check.

## IN PROGRESS: full-source security scan and authenticated cognitive status

A full tracked-source Nox scan now includes the AI and sensitive-data checks
that the narrower dependency-focused scan omitted. It examined 9 AI components
and 797 dependencies. After reviewing the ATTACK-003 route matches, the
cognitive backend configuration endpoint was moved from the public probe paths
to authenticated `GET /api/v1/health/cognitive`; the dashboard guide, feature
reference, and OpenAPI document now describe its authenticated path.

The latest full scan reports 161 findings (126 suppressed), including 159
lower-priority observations and one medium ATTACK-003 issue with two route
occurrences: the public `/api/v1/health` probe and `/api/v1/observability/health`,
which is inside `middleware.Auth`. Nox assigns both the same file-and-endpoint
fingerprint, so its baseline suppresses one occurrence but leaves the other
visible. The unversioned `/health` and `/healthz` probes are intentionally
public; remote unauthenticated dashboard binding is rejected unless an
operator explicitly enables `allow_insecure_no_auth`. Keep this finding open
until the duplicate-route fingerprint can be handled without hiding a real
exposure. Optional API-abuse, dependency-freshness, and live attack checks were
not enabled or authorized by this scan.

## IMPLEMENTED LOCALLY: CI records the protected reviewer in Relicta

`relicta approve --ci` previously recorded the runner as a CI actor, even when
the CI job had waited for a protected human reviewer. Added paired
`--actor-id` and `--actor-type` options for trusted integrations. They are
accepted only with `--ci`, must be supplied together, and validate the actor
type. They also require `--reason` so the approval source is preserved with
the decision. A human actor is stored as a human approval rather than an
automated approval. The CLI records the supplied identity; the integration
remains responsible for verifying the reviewer and binding the review to the
exact release plan.

The release workflow now creates a persisted plan before the environment gate,
transfers that run to the protected job, records the uniquely identified GitHub
reviewer through `relicta approve`, and carries the approved state to the
publication job. Both jobs check the release ID, plan hash, version, and commit;
the approval job also checks the human actor, non-automatic status, and workflow
run in its justification. CGP `approval_required` is satisfied only after the
protected review, while `deferred` and `rejected` still stop the workflow.
The GitHub review-history API returns reviewer identity but no approval-time
field, so Relicta records the approval timestamp when the post-gate approval is
written. This is a local workflow implementation. An authenticated, read-only
GitHub API lookup on 2026-11-25 returned 404 for
`relicta-tech/relicta`'s `release-approval` environment, so the required
protected reviewer configuration is currently absent. The workflow will fail
closed until an administrator creates that environment with at least one
required reviewer and `prevent_self_review` enabled. No hosted settings were
changed; an actual tag-triggered run remains unverified.

## RECHECKED: current Nox results need route-context review

On 2026-11-25, a fresh full scan of this worktree reported 288 findings across
797 dependencies and 9 AI components; 126 are suppressed and 162 remain active.
The active medium findings are mostly heuristic endpoint candidates in
`internal/httpserver/router.go`. The scanner does not resolve Chi route groups:
the `/api/v1` handlers are registered inside `middleware.Auth`, while health,
login, OpenAPI, and static asset routes are intentionally public. The two
`/health` matches still share one fingerprint; scanning code alone suppresses
it, but scanning all scopes exposes the duplicate again. Nox's own capability
report says attacker reachability and dynamic verification were not asked, so
these results do not establish an unauthenticated runtime path. Keep the
duplicate health match documented rather than changing public API routes to
work around the fingerprint.

The scan also reports 36 dependency advisories at informational severity,
attributed to the Node frontend build stage and marked `image_stage_shipped:
false`; the final runtime images do not derive from that stage. The local
frontend image build succeeded, but Trivy could not complete its image scan
because the temporary filesystem ran out of space. Docker then refused image
inspection/removal because its storage was read-only, so exact installed
package versions remain unverified in this pass. No finding was waived on
that basis. The new container-security workflow's same-ref cancellation was
confirmed safe because it only builds and scans; an inline Nox rationale now
records that intent, and the IaC scan reports it as waived.

## FIXED LOCALLY: release snapshots require checksums for every supported archive

The release snapshot validator checked each manifest entry it received, but did
not require the manifest to contain every supported release artifact. A missing
archive or SBOM checksum could therefore pass validation and proceed to signing
and attestation. The validator now requires checksum entries for the Linux
amd64 and arm64 archives, the Windows amd64 archive, and all three corresponding
SPDX SBOMs. It also rejects duplicate filenames and malformed SHA-256 digests,
and continues to verify every listed artifact. A new isolated GoReleaser
snapshot from the current worktree built all three binaries and SPDX SBOMs; the
validator verified embedded dashboards, all six checksums, SBOM documents, and
generated Linux Homebrew formula hashes. The
GoReleaser hooks `make frontend`, `go mod tidy -diff`, and `go mod verify` also
passed. This local run used Go 1.27.4, while hosted release jobs pin Go 1.26, so
the hosted snapshot remains necessary. Shell syntax validation,
`actionlint .github/workflows/release.yaml`, and `git diff --check` pass. No
automated tests or release actions were run.

## RECHECKED LOCALLY: release workflow plan handoff and approval sequence

An isolated CLI flow found that release IDs already include the `run-` prefix,
so the workflow's extra prefix looked for a nonexistent `run-run-*.json`. The
workflow now uses the release ID as the persisted filename. It also advances
the run through `bump` and deterministic user-audience `notes` before review;
without those transitions, `approve` correctly refuses a merely planned run.
The CI approval gate now evaluates the externally verified GitHub reviewer as
the actor, matching the human identity that Relicta records, instead of
blocking that review under the runner's restrictive CI autonomy budget.

An isolated temporary Git repository completed plan, bump, notes, approval,
and governance-chain verification, including checking the reviewer and audit
record. `actionlint`, `go build ./...`, `go vet ./...`, and `git diff --check`
pass. No automated tests, hosted settings, commits, pushes, or releases were
run. The actual GitHub `release-approval` environment is still absent, so a
hosted review and publication remain outstanding.

## FIXED LOCALLY: Relicta Action workflows use Node 24 runtimes

The maintained Action worktree had moved its own `action.yaml` and Node matrix
to Node 24, but its release, test, example, and end-to-end workflows still
pinned `actions/checkout` v4 and `actions/setup-node` v4. The release workflow
also pinned `softprops/action-gh-release` v2. GitHub removed Node 20 from
Actions runners on 2026-09-23, leaving those workflows dependent on retired
runtimes. Updated the pinned actions to verified Node 24 releases:
`actions/checkout` v7.0.1, `actions/setup-node` v7.1.0, and
`softprops/action-gh-release` v3.0.3.

In `/Users/felixgeelhaar/.codex/worktrees/deps-vuln-remediation/relicta-action`,
`actionlint`, Prettier format-check, ESLint, bundle build, `node --check`,
production-only npm audit (zero findings), and `git diff --check` pass. Tests
were not run. The live PR #7 ref remains at `9db954cd54de8d98ca10e967e5aecac7adbb7932`,
so this local Node 24 migration and signed-manifest implementation still need
hosted CI and review before they are recommended or released.

## RECHECKED: current release checkout builds cleanly; hosted checks are stale

On 2026-11-25, the current checkout passes `go build ./...` and `go vet ./...`
with Go 1.27.4, and `app/npm run build` passes Vue type checking and bundles all
six MCP apps into `internal/mcp/dist`. This resolves the earlier local
TypeScript/vue-tsc build failure recorded on PR #405's description for the
current dependency set; the current PR head still needs these changes and a
fresh hosted run.

A live GitHub read confirms PR #405 is open, non-draft, and mergeable at
`72d31c6b83744f6f11100dd8f41b15145c23cc13`. Its successful CI, security,
documentation, benchmark, accessibility, and skill checks ran on 2026-10-28,
so they do not verify today's local tree. No automated tests, commits, pushes,
merges, hosted configuration changes, or releases were performed in this
recheck. The `release-approval` environment, fresh checks against the current
revision, and governed publication remain launch gates.

## DIAGNOSED: PR #404 security failures are fixed by dependency PR #405

Read the archived logs and `nox-scan-results` artifact for PR #404 run
`40641186058` (head `00854edd39280c637818a0c29ab2c75b69b64184`, 2026-10-28).
Its scan found 71 unbaselined dependency advisories: 1 critical, 22 high, 42
medium, and 6 low. The critical advisory was `GHSA-jqcg-44mw-7w3h` in
`proxy-addr@2.0.7`, fixed in 2.0.8. Findings also covered old Go and npm
versions in `go.mod`, `app/package-lock.json`, and `web/package-lock.json`.
The separate Nox Security Scan job failed on the same dependency finding set.

The current checkout is at PR #405's dependency-update commit
`72d31c6b83744f6f11100dd8f41b15145c23cc13`; a fresh local Nox scan reports 36
active findings, all informational and in the non-shipping Docker `frontend`
build stage. It reports 797 dependencies and 9 AI components. This scan is
not complete: required `attack-surface` and `depconfusion` plugin binaries are
missing, and attacker reachability/dynamic verification were not evaluated.
Do not treat that as a clean security certification. The correct order is to
land the dependency fix, rebase PR #404, and rerun its hosted security checks;
do not suppress the 71 advisories to make the old branch green.

## DIAGNOSED: PR #422 security checks ran against an outdated dependency tree

Downloaded the `nox-scan-results` artifact from PR #422's 2026-11-15 CI run
`43720929638` and reviewed its findings. Both the Nox Security Scan and CI
Security Scan failed because Nox found 82 unbaselined dependency advisories in
the merge commit: 1 critical, 22 high, 49 medium, 6 low, and 4 informational,
across `app/package-lock.json`, `web/package-lock.json`, and `go.mod`. The merge
commit was `ef229b75`, built from base `247b693` plus the Dependabot `qs` update;
it did not include the later local dependency state. The scan artifact confirms
this PR run's failures, not the current worktree's vulnerability status.

The current checkout has since advanced representative dependencies beyond
those flagged versions: `proxy-addr` 2.0.8, MCP SDK 1.32.2, Vite 8.3.8,
`ip-address` 10.7.6, chi 5.4.0, gRPC 1.83.2, `x/crypto` 0.59.0, and `x/net`
0.62.0. The fresh local Nox result above reports 36 informational findings in
the non-shipping Docker frontend build stage and no active medium-or-higher
findings, with the documented plugin coverage limitations. PR #422 itself is
still open with the old merge result; it needs to be refreshed and rescanned
before its hosted checks can establish the current dependency posture. No
baseline suppression was added for this PR's findings.

## RECHECKED: Relicta Action hosted checks still run the obsolete Node matrix

Live GitHub state on 2026-11-25 shows Action PR #7 remains a draft at
`9db954cd54de8d98ca10e967e5aecac7adbb7932`. Its latest hosted test and
end-to-end checks all passed across Ubuntu, macOS, and Windows, but the test
matrix is explicitly Node 20.x. These checks predate and do not include the
maintained worktree at `57c2e13ea95347d27bff3b12c2dd6769b8476c44`, which
switches the Action to Node 24, authenticates the checksum manifest, and moves
its own workflows to Node 24-compatible pinned action versions.

Revalidated the maintained worktree locally: `actionlint`, Prettier format
check, ESLint, ncc bundle build, `node --check`, production-only npm audit
(zero findings), and `git diff --check` all pass. Tests were not run. Hosted
checks on the updated revision, review, and a fixed major release are still
required before recommending the Action for production release pipelines.

## FIXED LOCALLY: Go module installs report the module version

Installing the current public module with the documented `go install ...@v4.2.0`
command succeeded but `relicta version` reported `dev`. The command entry point
now falls back to `runtime/debug.ReadBuildInfo` when no linker version is set,
using the tagged module version embedded by Go. Release linker flags still take
precedence, and local source builds retain `dev`. `go build ./...` and an
isolated local-binary version check passed; no tests were run. The next tagged
module release should verify the resulting displayed version end to end.

## VERIFIED: current Docker runtime images have no HIGH/CRITICAL findings

Built the CLI image from the current repository and the web image from `web/`,
using `docker build --pull --no-cache` for both. The Dockerfiles upgraded the
CLI image's `libcrypto3`/`libssl3` to `3.5.9-r1` and zlib to `1.4-r0`; the web
image removed `nginx-module-image-filter` and `libexpat`, then upgraded zlib to
`1.4-r0`. Confirmed those packages in the built images. A fresh Trivy database
scan, without excluding unfixed advisories, found zero HIGH/CRITICAL OS or Go
binary vulnerabilities in the CLI image and zero HIGH/CRITICAL OS
vulnerabilities in the web image. Docker build-time npm audits also found zero
vulnerabilities. Updated only the matching Nox baseline rationales to include
this evidence. The Nox scan still has 36 active informational findings and did
not answer attacker-reachability or dynamic-verification questions; the
release workflow's scan of the exact pushed digest remains required. No
automated tests, commits, pushes, or releases were performed.

## FIXED: dashboard security headers now reach cached routes

A runtime smoke of the dashboard image showed that Nginx's location-level
cache headers suppressed server-level security headers on `/` and static
assets. Set `add_header_inherit merge` (supported by the pinned Nginx 1.30.8)
so the security headers are preserved alongside route-specific cache headers.
Rebuilt the web image with `--pull --no-cache`; verified HTTP 200 and all four
security headers on `/`, `/index.html`, a hashed JavaScript asset, and
`/nginx-health`. A full HIGH/CRITICAL Trivy scan of that rebuilt image found
zero vulnerabilities. This is local verification; the release scan of the
exact pushed image digest remains required.

## FIXED: release-governance skill no longer documents a removed notes flag

Dogfooding the repository's release workflow exposed stale guidance in the
distributed `relicta-release-governance` skill: it recommended
`relicta notes --ai`, removed in v5. The skill now directs users to set
`ai.enabled: true` in `.relicta.yaml` and run `relicta notes`, matching the
current CLI and quick-start documentation. A read-only local plan against the
current checkout still recommends v5.0.0 from v4.2.0; its 168-commit,
high-risk recommendation is approval-required, as expected. The plan used
`--dry-run --no-ai --skip-cognitive` and created no release state. This is
local dogfood evidence only; it does not substitute for hosted release checks.

## FIXED: current source had two Go lint blockers

Ran static checks against the current full worktree. `golangci-lint` found that
the release-governance verifier's printf-style helper was named `fatal`, and
that plugin lazy loading wrapped a function that always returned in a loop.
Renamed the helper to `fatalf` and removed the redundant loop; cancellation
still resets the cached load state so a later request can retry. The current
source now passes `golangci-lint run --timeout=5m`, `go build ./...`, and
`go vet ./...`. `actionlint .github/workflows/*.yaml` and `git diff --check`
also pass. Automated tests were not run. Hosted checks on the current revision
remain outstanding.

## FIXED LOCALLY: Relicta Action `latest` now verifies the resolved release tag

Reviewing the Action's installer found that `version: latest` downloaded from
GitHub's mutable `/releases/latest` alias while accepting a signature from any
Relicta version tag. An older, still-valid signed release could therefore be
replayed as latest. The Action now resolves GitHub's latest stable release to a
validated `vMAJOR.MINOR.PATCH` tag, downloads assets from that immutable tag,
and requires the checksum manifest signature to match that exact tag. Updated
the README security behavior to match. The Action source and generated bundle
pass Prettier, ESLint, TypeScript checking, bundle build, `node --check`,
`actionlint`, and `git diff --check`; automated tests were not run. Hosted checks
on the updated revision remain outstanding.

## RECHECKED: shipped dependency advisories remain clear locally

Refreshed runtime dependency checks against the current worktrees: `govulncheck
./...` reports no Go vulnerabilities, and `npm audit --omit=dev` reports zero
vulnerabilities in both the app and web packages and in the Relicta Action.
These checks cover the current local manifests and source, not the deployed
images or the exact hosted release commit. The release pipeline's digest-bound
security scan and fresh hosted checks remain required.

## RECHECKED: current source builds as an embedded-dashboard CLI

Built `./cmd/relicta` from the current checkout with the production
`embed_frontend` build tag into a temporary path. The binary starts, reports
`relicta dev` for this untagged local source build, and returns root help
successfully. This verifies the current Go source and available embedded
dashboard assets build together without modifying the ignored release snapshot.
It does not replace building and validating release archives, signing them, or
running the hosted release pipeline.

## FIXED LOCALLY: reviewed agent notes now reach the published release

The release governance job previously regenerated deterministic notes, while
GoReleaser had `changelog.disable: true`; this meant customer copy drafted in
the agent session could not reliably reach the published GitHub release. Added
`relicta notes --draft-file` for CI and the convention
`docs/release-notes/<version>.public.md` for Relicta's own tag workflow. The
workflow attaches that committed text to the release run, includes it in the
protected review summary, and the publish job extracts `.notes.text` from the
approved run and passes it to GoReleaser. Repositories using Relicta can use
the same CLI option and wire their publisher to the approved release record.
`go build ./...`, `go vet ./...`, `actionlint`, both `goreleaser check`
configurations, `relicta notes --help`, and scoped `git diff --check` passed.
Automated tests and hosted publication were not run; hosted environment and
release checks remain necessary.

## RECHECKED: local release source and shipped dependencies remain clean

On 2026-11-25, reran the current Go static analysis and vulnerability checks:
`go build ./...`, `go vet ./...`, and `golangci-lint run --timeout=5m` all
passed, with lint reporting 0 issues. `govulncheck ./...` found no
vulnerabilities, and `npm audit --omit=dev --audit-level=high` found zero
production vulnerabilities in the dashboard, MCP app, and Relicta Action.
`actionlint .github/workflows/*.yaml` and repository-wide `git diff --check`
also pass. These checks validate the local uncommitted tree; they do not update
the old PR checks or certify the current workflow on GitHub.

A fresh read-only GitHub check confirms PR #405 remains open and mergeable at
`72d31c6b83744f6f11100dd8f41b15145c23cc13`; its latest successful checks are
still from 2026-10-28. The repository's `release-approval` environment endpoint
still returns 404. Hosted release validation therefore remains a production
gate. No tests, commits, pushes, settings changes, merges, or releases were
performed.

The README opening now states the user outcome in release-owner terms: a
reviewable packet from repository changes, followed by the agent-session notes
workflow (explicitly marked unreleased v5) and a human review. It also says
Relicta does not fact-check agent-authored prose, so the market copy does not
imply automatic correctness. A follow-up source-to-claim audit confirmed the
observability subsystem is wired in `relicta serve`; its old package comments
still claimed it was unused. Corrected those comments and clarified the README
that Prometheus signals require configuration and incident correlations are
potential matches with confidence/reasons, not definitive causal findings.

## FIXED LOCALLY: `make install` resolves Go's actual binary directory

The install target expanded the shell `GOPATH` variable directly. Go permits
GOPATH to be unset, so on a clean shell this could resolve to `/bin/relicta`
instead of the user's Go bin directory. The target now resolves `GOBIN` through
`go env`, falls back to `go env GOPATH` plus `/bin`, creates that directory,
and reports the same behavior in `make help`. `make -n install` resolved to
`/Users/felixgeelhaar/Developer/go/bin/relicta` in this environment; a separate
temporary CLI build reported its injected version correctly, and scoped
`git diff --check` passed. Tests were not run. This does not verify package
manager installation or hosted release artifacts.

## RECHECKED: first-use and agent-authored notes path run locally

On 2026-11-25, exercised the documented first-use path with a disposable Git
repository: `relicta init` created `.relicta.yaml`; `plan --ci --json
--no-ai --skip-cognitive` returned a deterministic recommendation and version;
`bump --ci --version` applied it; and `notes --ci --json` produced the
expected deterministic changelog. Re-ran the flow with `notes --draft-file`
and confirmed the JSON response returned the same release ID, `provider:
agent`, and the supplied customer-facing text unchanged. The run reached
`notes_ready`, ready for human review. This exercised the CLI draft-file path,
not an MCP host/client session, and did not approve or publish. Hosted
integration and release validation remain outstanding.

## VERIFIED LOCALLY: MCP stdio customer-notes workflow

Ran the actual `relicta mcp serve` stdio process from a disposable initialized
repository and sent MCP `initialize`, `tools/list`, and `tools/call` requests.
The server exposed `relicta_plan`, `relicta_bump`, `relicta_notes`, and
`relicta_status`. A dry-run plan returned its deterministic recommendation
without persisting a release; a normal plan persisted a release ID; bumping
and submitting a customer draft through `relicta_notes` reached `notes_ready`.
The response identified `source: agent`, `agent_authored: true`, and returned
the supplied notes unchanged. A subsequent status call returned the same
release ID, version, notes, and `notes_source: agent`. This validates the real
local MCP wire path and structured output, but not a third-party agent host or
product context quality. No approval or publishing was attempted.

## VERIFIED LOCALLY: release-mode embedded dashboard serves at runtime

Built the CLI with the GoReleaser `embed_frontend` tag into `/tmp`, started it
from a disposable repository on loopback, and fetched `/`. The server returned
HTTP 200 with the expected Content Security Policy and other security headers;
the referenced JavaScript asset also returned HTTP 200 (61,791 bytes). The
server stopped cleanly. Both `.goreleaser.yaml` and
`.goreleaser.non-macos.yaml` pass `goreleaser check`; ShellCheck passes for the
snapshot validator and Homebrew formula publisher. This verifies the local
embedded UI path, not release archives or hosted signatures. A fresh read-only
GitHub check still shows PR #405's checks dated 2026-10-28 and the
`release-approval` environment endpoint returning 404, so hosted release
validation remains outstanding.

## RECHECKED: hosted launch prerequisites and local snapshot checksums

Read-only GitHub inspection on 2026-11-25 confirms the latest public release is
still v4.2.0 (published 2026-07-10). The release repository secret-name list
contains `HOMEBREW_TAP_TOKEN`; no macOS signing/notarization secret names are
present. The `release-approval` environment endpoint returns 404, while the
release workflow requires a reviewer-protected environment with self-review
prevented. Repository Actions settings currently allow all actions and do not
enforce SHA pinning; all 107 external `uses:` references currently present in
`.github/workflows` are nevertheless pinned to full 40-character commit SHAs.
The workflow remains unverified on the current local tree, and no hosted
settings or secrets were changed.

Separately, verified every entry in the existing ignored local `dist/checksums.txt`
with `sha256sum -c`: Linux amd64/arm64 and Windows amd64 archives and all three
SBOMs matched. These local artifacts are not hosted release assets and do not
prove provenance or publication.

## FIXED LOCALLY: Action release identity matching escapes exact tags

A follow-up review of the local `latest` tag-binding fix found that the regular
expression used to escape the resolved version was malformed and left dots in
semantic versions unescaped. That could let a pinned release identity pattern
match a different tag. Replaced it with the standard regex metacharacter escape,
rebuilt the checked-in bundle, and confirmed `v5.0.0` becomes `v5\\.0\\.0`.
The Action bundle build, Prettier check, ESLint, and `git diff --check` pass.
Automated tests were not run. This change remains local and has no hosted CI
coverage.

## VERIFIED LOCALLY: fresh v5 source release snapshot

The existing ignored `dist/` artifacts were from an earlier source state, so a
fresh snapshot was built from an isolated copy of the current tracked and
untracked tree into `/tmp/relicta-current-snapshot.yBwpVZ`. The first attempt
in the shared checkout stopped in `make frontend` when `npm ci` hit macOS
`ENOTEMPTY` while replacing an existing dependency directory; no release
artifacts were written to the preserved `dist/`. The isolated build completed
all release hooks, compiled Linux amd64/arm64 and Windows amd64 binaries,
created the three archives and SPDX SBOMs, and wrote six checksum entries.
`scripts/validate-release-snapshot.sh` passed against the fresh artifacts,
including embedded dashboard checks and the generated Linux Homebrew formula.
The snapshot is labeled `5.0.0-snapshot`, based on local source revision
`72d31c6b83744f6f11100dd8f41b15145c23cc13`; it is not a signed or published
release and does not provide hosted release-gate evidence. Automated tests were
not run.

## VERIFIED LOCALLY: fresh v5 Linux package first-use flow

Ran the freshly built `5.0.0-snapshot` Linux amd64 archive inside an Alpine
container. The packaged binary reports the snapshot version, root help shows
the current release-governance and agent-session notes positioning, and `init`
creates `.relicta.yaml` with local-only publishing defaults. In a disposable
Git repository, `plan --dry-run --json --no-ai --skip-cognitive` returned
`next_version: 0.1.0`, an `approved` governance decision for the trivial demo
change, and an empty `release_id`, confirming the dry-run did not persist a
release. The container was removed automatically. This validates the Linux
amd64 package's basic customer path, not hosted publication, other platforms,
or a production repository's policy decisions. Automated tests were not run.

The Action's exact-tag verifier received an additional live-artifact check on
2026-11-25: its local Sigstore library accepted the published v4.2.0 checksum
bundle for identity `refs/tags/v4.2.0` and rejected the same bundle when the
identity was constrained to `refs/tags/v4.2.1`. Rebuilt the checked-in Action
bundle afterward; format check, ESLint, Node syntax check, production-only npm
audit (zero findings), and `git diff --check` pass. This confirms local source
behavior against the current published checksum bundle. It does not update the
draft PR or establish hosted CI evidence.

## PREPARED LOCALLY: focused Action update applies to draft PR #7

Created a separate review worktree at PR #7's current hosted head
`9db954cd54de8d98ca10e967e5aecac7adbb7932`, then layered the current Action
source, release verification, Node 24 workflow updates, docs, and generated
bundle on that exact base. The resulting patch is available at
`/tmp/relicta-action-pr7-focused.patch`; `git apply --check` succeeded against
a second clean worktree at the PR head. In the review worktree, the bundle
build, Prettier check, ESLint, JavaScript syntax check, production-only npm
audit (zero vulnerabilities), `actionlint`, and `git diff --check` passed.
The patch contains 22 Action files and no dependency-install directory. No
commit, push, PR update, review, or release was made. PR #7 remains draft, so
hosted CI and maintainer review still need to happen before the public Action
can be recommended for production.

## PREPARED LOCALLY: v5 customer release notes now match the release workflow

The tag workflow looks for `docs/release-notes/<version>.public.md`, but the
repository had only `docs/release-notes/5.0.0.md`, an internal draft that
explicitly says it is not approved or published. Added
`docs/release-notes/5.0.0.public.md` as customer-facing copy with the monorepo,
persistence, agent-session notes, version-file, and policy changes plus the Go
module and CLI migration guidance. Kept the internal draft unchanged. Exercised
the exact public-draft path with the freshly built `5.0.0-snapshot` Linux
archive in a disposable Git repository: `init`, CI plan, bump, and
`notes --ci --draft-file` completed, returned provider `agent`, the same release
ID, and the supplied 2,749-character notes. An earlier attempt with the old
pre-existing `bin/relicta` failed because that binary predates `--draft-file`;
the fresh artifact succeeds. `git diff --check` passes. The public draft is
prepared for review, not approved or published.

## FIXED LOCALLY: troubleshooting AI setup uses the supported configuration

The AI troubleshooting examples disabled AI under `changelog.ai`, which the
configuration loader does not read, and described non-AI release notes as
limited to conventional commits. Updated the instructions to use the actual
top-level `ai.enabled` setting and to describe deterministic notes based on
classified changes, matching the quick-start guide and configuration schema.
The shown YAML path is confirmed by `AIConfig` at the root `ai` key and the
loader's `ai.enabled` setting. `git diff --check` passes; no tests were run.

## FIXED LOCALLY: tag-triggered release instructions distinguish CI from local publishing

The quick-start's tag example pushed a version tag and then showed
`relicta release --yes` without saying that command runs in the tag-triggered
workflow. That could lead an operator to run the release twice or configure two
publishers for the same tag. Clarified that Relicta's tag-push mode runs inside
the workflow triggered by the pushed tag, while local releases should use
`relicta release` and allow Relicta to create its local tag. The instructions
now call out full Git history and a single publishing workflow. Checked against
the CLI's tag-push mode and default local-tag behavior; `git diff --check`
passes. No tests were run.

## FIXED LOCALLY: README opens the agent-notes workflow directly

The README's v5 agent-session notes feature linked to `docs/mcp.md#quick-start`,
which lands on server setup rather than the instructions for drafting notes from
the release recommendation and saving them for human review. Added a dedicated
"Draft customer-facing release notes in your agent session" heading at the
workflow instructions and updated the README link to its generated anchor.
Manually confirmed the target heading and link text match; `git diff --check`
passes. This is a documentation correction only; the feature remains v5 source
until the governed release is published.

## FIXED LOCALLY: quick-start describes dry-run release gates

The first-release preview was described as previewing version-file changes without mentioning
that `relicta release --dry-run` enforces the same pre-publish prerequisites as a live release.
Clarified that it checks the actor budget, allowed branch, clean working tree, and branch freshness,
while skipping file writes, release notes, and governance evaluation. Confirmed the gate order in
`internal/cli/release.go` and `internal/cli/publish_gates.go`; `git diff --check` passes. No tests
were run.

## VERIFIED LOCALLY: MCP customer-notes test fixtures match pinned planning state

Running `go test ./internal/mcp` exposed three notes-handler tests whose synthetic release runs
had an empty planned HEAD while the repository inspector returned `abc123`. The use case correctly
refused to save notes against a different planning snapshot. Updated the shared test fixture to
create a planned release with the matching pinned HEAD and adjusted the cancel test to use the
fixture's derived release ID. Also narrowed a JSON omission assertion so it checks the exact
`recommendation` key instead of matching `recommendation_persisted`. The full MCP package test now
passes. This corrects invalid fixtures and a false-positive assertion; no production source was
changed in this fix.

The same turn's fresh Nox scan completed with 158 total findings across 797 dependencies and 9 AI
components. The current active findings list contains 36 informational advisories, all in the
non-shipping frontend/builder stages of `Dockerfile` and `web/docker/Dockerfile`; the tool explains
that these stages do not feed the final images. No active medium-or-higher findings appeared, but
the scan remains incomplete: `nox/attack-surface@0.3.0` and `nox/depconfusion@0.2.4` binaries are
unavailable, and attacker reachability and dynamic verification were not exercised. The hosted
PR #422 remains open with failing security scans on its 2026-11-15 merge tree; PR #405 remains
open with successful but stale 2026-10-28 checks. Public latest remains v4.2.0. These local and
historical checks are not release clearance.

## DOGFOODED LOCALLY: agent-session notes in Govee

Exercised the MCP customer-notes path against a clean disposable clone of Govee Light Management;
the user's original checkout and its 13 pending entries were left untouched. Relicta planned
3.1.3 from seven commits since v3.1.2: one dependency fix, no features, and no breaking changes.
The agent drafted a short public maintenance note based on the recommendation, then MCP
`relicta_bump` and `relicta_notes` saved it under the exact release ID. The tool response and
`relicta_status` both returned the identical draft with `notes_source: agent`; the run stopped at
`notes_ready` and was not approved or published. The scratch clone remained Git-clean, and no tag
was created. This confirms the workflow can carry product context from an agent session into a
reviewable release record. Product follow-up: make MCP tool availability and repository-specific
notes instructions straightforward to discover across coding-agent clients, while preserving the
recommendation as the factual boundary and human review as the approval gate.

## VERIFIED LOCALLY: MCP app production build passes on current dependency updates

The dependency PR description reported a TypeScript/Vue app build failure in its earlier local
validation. On the current `fix/dependency-vulnerability-updates` worktree, `npm run build` now
passes: `vue-tsc --noEmit` succeeds and the build script produces all six MCP apps in `app/dist`
and `internal/mcp/dist`. GitHub Actions metadata confirms PR #405's latest CI and Security Analysis
runs passed on head `72d31c6b83744f6f11100dd8f41b15145c23cc13`. This validates the current local
build but does not replace rerunning hosted checks against any later changes or establish a
production release.
# VERIFIED LOCALLY: production-readiness Go suite and backend parity

The full Go suite exposed and resolved several current-tree gaps. Added the missing OpenAPI
contracts for `/auth/mode` and `/ws/ticket`, plus narrowly justified CLI infrastructure allowlist
entries. Corrected the webhook shutdown test to exercise an in-flight synchronous publish and
shutdown drain without waiting through retry timeouts; its cancellation test now asserts propagated
context errors. Recommendation artifacts now fall back to the established byte-preserving file
sidecar when release runs use SQLite or PostgreSQL, so MCP/HTTP consumers retain the same ADR-009
artifact across backends; the differential command test passes for both SQL backends. Corrected
stale compliance-scope, plugin `ContinueOnError`, Annex IV heading, and localhost metrics test
fixtures/expectations. `go test ./...` now passes, including the PostgreSQL Testcontainers and
differential release suites. `git diff --check` passes. This is local evidence; hosted CI and
security checks still need to run on the final release tree.

## RECHECKED: hosted readiness references and current scan (2026-11-25)

Live GitHub metadata confirms PR #405 remains open, non-draft, and mergeable at
`72d31c6b83744f6f11100dd8f41b15145c23cc13`. Its latest hosted lint/test, build, dependency,
Nox, and security checks succeeded on that commit. The current local worktree contains additional
uncommitted changes, so the hosted checks do not validate that full tree. PR #422 remains open and
mergeable with historical Nov 15 security failures (82 findings); PR #404 remains open and draft
with historical Oct 28 Nox failures (71 findings). Both need fresh hosted scans on the proposed
release tree.

A fresh Nox 1.70.0 scan now completes all configured scopes with zero degradations: 288 total
findings over 797 dependencies and 9 AI components, with 161 active (60 medium, 101 informational)
and 127 suppressed/baselined. The 60 medium findings are attack-surface heuristics for routes and
WebSockets. Code inspection confirms the read APIs are registered within `middleware.Auth`, the
WebSocket upgrade consumes a short-lived single-use ticket, and `/health`, `/healthz`, `/readyz`,
and auth endpoints are intentionally public. This is static code inspection, not runtime proof:
attacker-reachability and dynamic-verification capabilities answered zero questions, and the
offline attack planner had no scenarios for the active rule IDs. The 36 active dependency findings
are informational. Thirteen Docker build-stage advisory instances moved to the reviewed baseline;
that is a suppression change, not a package fix. Current Docker runtime images were separately
scanned with zero HIGH/CRITICAL findings in the previous check. Keep hosted digest scanning as a
release gate.

Corrected the roadmap's repository references: `relicta-tech/relicta#7` is the merged CGP
implementation, while `relicta-tech/relicta-action#7` is the still-open draft to fail closed on
binary checksum verification. The Action repository's latest release remains v2.5.0 (Mar 15).
`relicta#227`, merged Aug 8, is the main repository's Node 24 CI migration. Keep the Action
checksum issue as a separate customer-pipeline readiness item; it needs hosted review and checks
on the current Action PR before recommending a fixed release. The repaired analysis plugins are
verified installed, but scan results still do not exercise attacker reachability or dynamic tests.

## VERIFIED LOCALLY: release snapshot package and Linux formula validate (2026-11-25)

Ran `make validate-release-snapshot` against the current `dist/` release snapshot. The
Linux amd64, Linux arm64, and Windows amd64 archives each contain the expected embedded
dashboard assets; all three adjacent SPDX JSON SBOMs parse; all six checksum entries
match their files; and the generated Linux-only Homebrew formula uses the matching
archive URLs and SHA-256 values. The validator passed. This validates the existing local
snapshot only: it does not establish that hosted release jobs, final published digests,
macOS packaging, or protected environment reviewer settings are ready.

## VERIFIED LOCALLY: current release workflows and snapshot scripts pass static checks (2026-11-25)

Ran `actionlint .github/workflows/*.yaml` against every checked-in GitHub Actions workflow;
all passed. Ran ShellCheck on `scripts/validate-release-snapshot.sh`,
`scripts/publish-homebrew-formula.sh`, and `scripts/verify-release-tag.sh`; all passed.
`git diff --check` also passed. These checks validate local syntax and common workflow
hazards only. They do not validate hosted GitHub environment protections, secrets,
permissions as enforced by repository settings, or a release run on the hosted runner.

## RECHECKED: hosted dependency PR checks identify the remaining security gate (2026-11-25)

Read current GitHub check diagnostics for PR #405, PR #422, and
`relicta-action#7`. PR #405's checks all passed at its hosted head
`72d31c6b83744f6f11100dd8f41b15145c23cc13`; they do not cover the additional
uncommitted release work in this checkout. PR #422's current head
`08442b2ad671be909353f09e004a92de8fcd2784` now passes build, lint/tests,
dependency review, policy, and SBOM jobs, while both Nox jobs fail. Their logs
show Nox 1.26.0 reporting 82 dependency findings (305 suppressed), so this is a
real hosted release gate, not only the earlier route-heuristic concern. The
current worktree pins Nox 1.70.0 in CI and security workflows; hosted results
for that version on the final release tree are still needed before interpreting
whether the newer scan clears the gate. The Action PR's six hosted matrix and
end-to-end jobs pass at `9db954cd54de8d98ca10e967e5aecac7adbb7932`, but the
PR remains draft and those checks still identify Node 20.x matrix entries.
No remote refs or PR metadata were changed.

## RECONCILED: Dependabot security failure is from scanning the pre-update base (2026-11-25)

Downloaded and compared the actual `nox-scan-results` artifacts for PR #405
(`72d31c6b83744f6f11100dd8f41b15145c23cc13`) and PR #422
(`08442b2ad671be909353f09e004a92de8fcd2784`). Both used Nox 1.26.0. PR #422's
artifact has 82 active findings, and every one is `VULN-001` in `app/package-lock.json`,
`web/package-lock.json`, or `go.mod`; it contains no active route or source-code
finding. None of those 82 advisory IDs occur in PR #405's artifact, which reports zero
active findings. The two PRs were independently based on the same older `main`, so #422's
security failure is explained by its tree not containing #405's dependency updates. The
appropriate integration sequence is to land/update the dependency tree before refreshing
#422's scan, rather than baselining these advisories.

Also ran the current workflow command locally (`NOX_FINGERPRINT_VERSION=2 nox scan .
--severity-threshold high --fail-on-degraded`). Nox 1.70.0 reported no findings at or
above HIGH among 797 dependencies and 9 AI components, but exited non-zero because the
operator installation has 14 optional plugins installed globally that are intentionally
not declared for this repository; those plugins were not run. The JSON report identifies
that as an advisory plugin degradation. A clean hosted runner should have only the two
repository-required plugins, but a hosted Nox 1.70 run on the final tree remains the
stronger confirmation. The local result must not be described as a fully completed scan.

## VERIFIED LOCALLY: clean-runner Nox plugin verification (2026-11-25)

Reproduced the CI security scan in a disposable `golang:1.26.11-bookworm` Linux
container with the repository mounted read-only. Verified the downloaded Nox 1.70.0
archive against the workflow SHA-256 and installed Cosign v3.0.6 with its verified
SHA-256. With Cosign available, Nox successfully verified and auto-installed both
required community plugins (`depconfusion@0.2.4` and `attack-surface@0.3.0`), then
completed the workflow scan with `--severity-threshold high --fail-on-degraded`:
zero findings, 188 suppressed findings, 234 dependencies, 8 AI components, and no
reported degradations. The current CI and security workflows now install Cosign using
the repository's existing pinned `sigstore/cosign-installer` action before scanning.
`actionlint` and `git diff --check` pass. This reproduces the clean-runner Nox setup
locally; hosted checks on the final proposed tree remain necessary.

## VERIFIED LOCALLY: market copy and docs dependency refresh (2026-11-26)

In the separate `relicta-site` checkout, corrected homepage and blog claims that blended v5
source features into the public v4.2.0 offer or implied Relicta governs arbitrary agent activity.
The Vision page now identifies itself as roadmap direction, limits its MCP governance claim to
actions routed through Relicta, and links to the existing CGP specification. Documentation now
states the release boundary, supported installation paths, v4.2.0 MCP HTTP security limits, and
that governance records support rather than establish compliance. Added a custom docs 404 page.

The docs site had 30 npm audit findings (one critical) in its locked Astro 5 / Starlight 0.37
dependency tree. Updated to Astro 7.4.13 and Starlight 0.43.1, migrated to Starlight's current
content loaders, and removed the obsolete postinstall source patch. A clean `npm ci` now reports
zero findings; `npm run build` succeeds with all 11 docs pages. The marketing site also builds;
Vite still warns that a JS chunk exceeds 500 kB. `git diff --check` passes. These are local source
and build results only: the site has not been deployed, and the larger marketing-site bundle has
not been optimized.

## VERIFIED LOCALLY: application and Go source build checks (2026-11-26)

On the current working tree, `npm run build` succeeds in both `app/` (MCP Apps)
and `web/` (dashboard), including their TypeScript checks. `go build ./...` and
`go vet ./...` also pass, and `git diff --check` is clean. `relicta status
--json` confirms there is no active release run. These checks do not replace
hosted CI/security results for the full uncommitted tree; the release approval
environment and an independent reviewer are still not provisioned.

## VERIFIED LOCALLY: release artifacts and market-site dependency readiness (2026-11-26)

Built the exact current source candidate in an isolated temporary copy with GoReleaser's
non-macOS snapshot configuration. The release validator passed for Linux amd64, Linux arm64,
and Windows amd64 archives: each binary contains the dashboard, each archive has a valid SPDX
SBOM, all six archive/SBOM checksums match, and the generated Linux Homebrew formula matches
the release checksums. The existing `dist/` directory in the working checkout was preserved.

In the separate `relicta-site` checkout, removed the unused Mermaid integration and its client
bundle/dependency chain after confirming no built page or current content used it. This also
removed the remaining low-severity npm findings; `npm audit` now reports zero vulnerabilities.
The marketing site passes `astro check` with zero errors, warnings, or hints and builds all seven
pages without the previous oversized-chunk warning. The documentation site passes `astro check`
with zero diagnostics, builds all eleven pages, and has zero npm audit findings. Its TypeScript
config now excludes the sibling docs output from marketing-site checks; documented content schema
uses Astro's current Zod entrypoint, and the remaining legacy diagram component diagnostics were
cleaned up.

The source repository's workflow lint, both GoReleaser config validations, Go formatting, and
golangci-lint pass. The full uncommitted candidate still needs hosted CI/security runs and the
protected release environment plus independent reviewer before publication.

After that checkpoint, the marketing homepage copy was tightened to describe release governance
specifically, remove unsupported monorepo strategy promises, qualify plugin verification status,
and mark MCP Apps as unreleased v5 concepts. The separate site checkout again passes Astro checks
with zero diagnostics, builds all seven pages, reports zero npm vulnerabilities, and passes
`git diff --check`.

## VERIFIED LOCALLY: clean-environment security scan on current worktree (2026-11-26)

Re-ran the repository's CI Nox gate against the current full worktree with Nox 1.70.0 and a
temporary isolated home directory. Only the two plugins declared in `.nox.yaml` were installed;
the scan completed without plugin or other degradations: zero HIGH or CRITICAL findings, 797
dependencies, and 9 AI components. This supersedes the earlier local scan note that was
incomplete because of unrelated globally installed plugins. GitHub PR #405 also reports passing
build, lint/test, dependency review, and Nox checks for commit `72d31c6`, but that commit is the
branch base and does not include the current uncommitted candidate. Hosted checks on the candidate,
protected release-environment readiness, and independent review remain outstanding.

## VERIFIED LOCALLY: disposable v5 release plan and MCP-authored notes flow (2026-11-26)

Assembled the exact current 833-entry worktree state into a disposable Git worktree, committed the
snapshot only there so the clean-tree requirement could be evaluated, and built the v5 CLI from
that snapshot. The dry-run plan and local release flow both recommended `5.0.0` from `4.2.0`:
169 commits, 42 features, and 68 fixes. Governance correctly required a release manager because
the recommendation was critical risk (0.8), including 1006 files / 112255 lines in its measured
release blast radius and 15 security-related changes.

In that disposable repository, `relicta plan`, `relicta bump`, and `relicta notes --draft-file`
completed. Then a direct MCP stdio client called `relicta_notes` with the same public draft; the
tool returned the same release ID, version `5.0.0`, `source: agent`, and `state: notes_ready`.
This verifies the draft persists through the product's MCP flow without an AI-provider request.
The isolated run was left unapproved and unpublished, and its temporary worktree was removed.
It does not replace validation on the eventual merged `main` commit or hosted CI on that commit.

## RECONCILED LOCALLY: production-hardening worktree against current v5 candidate

The separate `codex/relicta-production-hardening` worktree is based on the current candidate's
base but contains v4 source. Its release approval, external publication evidence, container build,
linked-worktree, and MCP documentation changes were compared with this v5 candidate. The current
candidate already contains the compatible release workflow, persisted publisher evidence,
container hardening, linked-worktree support, and MCP notes guidance in its v5 implementation.
The older CLI-side approval verifier is superseded by the protected GitHub workflow's reviewer
identity check and was not transplanted. The independent hardening worktree's CLI, release-domain,
Git, and MCP package tests pass. The current candidate's corresponding v5 CLI, release-domain,
Git, and MCP package suites also pass. A focused MCP regression test confirms that changing
agent-authored customer notes changes the persisted notes integrity hash while leaving the release
in `notes_ready`.

## IMPLEMENTED LOCALLY: verified human approval and external publication outcome (2026-11-26)

Compared the hardening branch against the current v5 candidate's actual protected release workflow.
The candidate already gates publication behind the `release-approval` GitHub environment, confirms
that the environment requires reviewers and prevents self-review, queries this workflow run's
approved reviewer, binds the approval to the persisted plan hash and exact commit, and validates
the approved release state before the privileged release job proceeds. The branch's new CLI-side
GitHub approval verifier would duplicate this workflow-specific verification and is built against
older API/domain surfaces, so it is not safe to copy as-is.

The candidate's release workflow previously stopped after GoReleaser, attestations, GitHub release
visibility, and Homebrew publication without recording those outcomes in Relicta. Added
`relicta publish --external-evidence-file`, persisted canonical publisher evidence in the release
aggregate and every release-run storage backend, exposed it through `relicta status`, and added a
workflow job that reports GitHub Release, verified container publication (including the digest),
and Homebrew success/failure/skip state. Successful finalization requires the exact approved tag
and commit, the GitHub Release and container publishers to succeed, and Homebrew to succeed or be
skipped only for prereleases. Failed publisher reports fail the release run while retaining partial
outcomes. External finalization rejects execution plans with plugins or other unexecuted steps.

The feature compiles and the edited workflow passes actionlint. A read-only GitHub environment query
on 2026-11-26 returned only the unprotected `copilot` environment; the required protected
`release-approval` environment is not configured. The workflow has not yet run in GitHub Actions;
validate artifact transfer, publisher-result expressions, and the final persisted state on a hosted
tag workflow before treating this release workflow as production proven.

## VERIFIED LOCALLY: current candidate passes full CI gate (2026-11-26)

`make check-ci` completed successfully on the current worktree: formatting, `go vet`, golangci-lint,
the full Go test suite, and all 27 configured coverage domains passed. CLI coverage is 54.0% against
its 54.0% minimum; infrastructure is 76.2% against 76.0%. The final Nox step emitted a degraded
environment warning because 14 globally registered optional plugins have no usable binaries and
are not in `.nox.yaml`'s required list. The separate clean-home Nox scan recorded above is the
non-degraded security-scan evidence. This local result does not cover the candidate in hosted CI.

## CORRECTED: 5.0 monorepo release notes match governance scope (2026-11-26)

Review against the implemented monorepo flow and `docs/design/monorepo-versioning.md` found that
the draft release notes claimed a separate governance decision for each package. Relicta currently
calculates package versions, changelogs, and tags independently, but plans and approves at the
repository level. Updated both internal and public 5.0.0 notes to state this boundary explicitly.

## CORRECTION SUPERSEDED: package approvals are implemented (2026-11-26)

Further review of the current CLI and monorepo release flow showed that the preceding correction
was wrong: each changed package has its own release run, policy evaluation, approval, and audit
entry, while the repository release separately coordinates the overall workflow. The per-package
approval path is exposed through `relicta approve --package`. Updated the 5.0.0 notes and the
monorepo design status to reflect both decision layers. For packages with their own release run,
the package approval gate controls whether they are included; retain the no-run compatibility path
when making claims about package tags. See `internal/cli/monorepo_scope.go`,
`internal/cli/plan_monorepo.go`, and ADR-015.

## MARKET COPY REFINED: policy logic claim grounded in evaluator (2026-11-26)

The local `relicta-site` candidate still described the policy language as having "full logic
support," which is broader than a useful product claim. Replaced it with the concrete supported
AND, OR, and NOT conditions, verified against the evaluator guide. The marketing site's Astro
check reports zero diagnostics, its production build generates all seven routes, and
`scripts/verify-build.mjs src dist` passes. Committed locally on `codex/site-production-readiness`
as `647bd44`; no PR or deployment was created. Public site accuracy remains unresolved until
the reviewed site changes are published.

## DOCUMENTED: safe GitHub-hosted snapshot release check (2026-11-26)

The release workflow has a manual `workflow_dispatch` job that builds and validates snapshot
archives without signing or publishing. Added the exact Actions UI path and what that run does and
does not verify to the supply-chain guide. This gives maintainers a hosted packaging check before
creating a version tag while keeping the distinction from the protected live-release path clear.

## VERIFIED LOCALLY: market site checks and production build pass (2026-11-26)

Rechecked the separate `relicta-site` working copy after its market-copy updates. `npm run check`
reported zero errors, warnings, or hints, and `npm run build` generated all seven routes. The built
product page contains the corrected release governance, approval, and notes claims; its v5
interfaces remain labeled as unreleased. `npm audit --omit=dev --audit-level=high` found zero
production dependency vulnerabilities, and `git diff --check` passes. This validates the current
local site working copy only; it contains pre-existing uncommitted edits and was not deployed.

## UPDATED AND VERIFIED LOCALLY: MCP onboarding covers agent-written customer notes (2026-11-26)

The separate `relicta-site` documentation site previously said the newer agent-session customer-note
workflow was absent from v4.2.0, but did not explain how it works. Added a concise v5-source workflow
to the MCP guide: inspect a dry-run recommendation, create the live plan and version, draft in the
existing agent session, save through `relicta_notes`, then review. The page states that the path is
not in public v4.2.0 and Relicta does not fact-check the supplied prose. `npm run check` reports zero
diagnostics, the docs build generates all 11 routes, the rendered MCP page contains these claims,
and `git diff --check` passes. This remains local documentation work in a pre-existing dirty site
checkout; it has not been deployed.

## VERIFIED LOCALLY: production container build, restricted smoke, and vulnerability scan (2026-11-26)

Built the production `Dockerfile` from the current candidate worktree as local image
`relicta:readiness` (image ID `sha256:27b0a176330e4e01dfc862e053ba827c9e16c4339d53bcb533929c8f927995c6`).
The release workflow's `scripts/smoke-container.sh` passed in its restricted runtime: initialization,
release planning, health, authenticated dashboard serving, and API auth enforcement all succeeded.
Trivy scanned this exact local image with a fresh current database for OS and Go-library HIGH/CRITICAL
findings (including unfixed) and reported zero. The installed Trivy 0.76.2 identified `--vuln-type`
as deprecated, so all three repository image-scanning workflows now use `--pkg-types`; scanning the
same image with the replacement produced zero findings without a deprecation warning. `actionlint`
passes for all three edited workflows and `git diff --check` passes. This is single-platform local
evidence; it does not establish registry digest, multi-platform image results, hosted attestations,
or published-release readiness.

## RECHECKED: public install state, verified Homebrew hashes, and image severity range (2026-11-26)

Live GitHub still lists `v4.2.0` as the latest stable Relicta release; its assets include Linux
amd64/arm64, Windows amd64, and unsigned Darwin amd64/arm64 archives. The live Homebrew formula
still points at `v4.2.0`. All four formula archive hashes match the release checksum manifest;
`cosign verify-blob` accepted the manifest signature for `release.yaml@refs/tags/v4.2.0`, and
`gh attestation verify` accepted its provenance for the same repository, workflow, and tag. The
current locally built candidate container returned zero Trivy findings across MEDIUM, HIGH, and
CRITICAL OS and Go-library severities. GitHub still lists only the unprotected `copilot` environment,
so protected approval and hosted release validation remain unavailable until repository setup and
candidate publication are completed.

## HARDENED LOCALLY: market-site production deployment workflow (2026-11-26)

In the separate `relicta-site` checkout, pinned `actions/checkout`, `actions/setup-node`, and
`cloudflare/wrangler-action` to immutable upstream commits and pinned Wrangler to the exact current
published CLI version. Both Cloudflare Pages production jobs now run only for `refs/heads/main`;
manual dispatch from another branch cannot deploy to the production project. Separate non-canceling
concurrency groups serialize deployments per Pages project so a running deploy is not interrupted
and the latest queued main deployment wins. `actionlint` and `git diff --check` pass, and the built
marketing/docs outputs pass their existing static verifiers. The configured Cloudflare secret names
are present in GitHub, but no deploy was run; hosted workflow execution and Pages production state
remain unverified.

## VERIFIED LOCALLY: current candidate clears the fresh Nox dependency gate (2026-11-26)

The live `main` scheduled Security Analysis run `45122265990` failed on 2026-11-23: Nox 1.26.0
reported 85 unsuppressed findings, including known vulnerabilities in dependencies recorded in
`app/package-lock.json`. Scanned the current candidate with Nox 1.70.0 using a fresh isolated
`NOX_HOME` so stale machine plugin registrations could not degrade the result. The exact CI gate
(`nox scan . --severity-threshold high --fail-on-degraded -format json,sarif`) exited successfully:
797 dependencies checked, zero new HIGH/CRITICAL findings, and no degraded checks. Nox still emits
36 informational VULN-001 records for packages in intermediate Docker build stages; the existing
production image scan independently reports zero MEDIUM/HIGH/CRITICAL OS and Go-library findings.
This confirms the current local dependency updates address the hosted findings, but the dirty
candidate has not been pushed and hosted CI has not validated it.

## VERIFIED LOCALLY: current-worktree release snapshot and platform credentials (2026-11-26)

Ran the pinned GoReleaser v2.20.3 snapshot flow against the current working tree, directing output
to a temporary path so the pre-existing ignored `dist/` artifacts were preserved. The `make
frontend`, `go mod tidy -diff`, and `go mod verify` hooks passed. GoReleaser built Linux amd64,
Linux arm64, and Windows amd64 archives; `scripts/validate-release-snapshot.sh` verified the
embedded dashboard in each binary, all three SPDX SBOMs, six checksum entries, and the Linux-only
Homebrew formula. No publish or signing step ran. The ignored current `dist/` folder remains the
older snapshot from clean commit `72d31c6`; do not use it as candidate evidence. Live GitHub secret
names show `HOMEBREW_TAP_TOKEN` is configured and no Apple signing/notarization secrets are present,
so the release workflow will choose its non-macOS config. README and install docs already explain
the macOS limitation and direct users to Go installation/source builds. This validates local
packaging, not hosted tag workflow, protected approval, or publication.

## IMPROVED: Quick Start surfaces agent-authored customer notes (2026-11-26)

The Quick Start described the standard deterministic notes command but did not surface the
agent-session workflow near the release lifecycle. Added an adjacent subsection showing when to use
the MCP handoff, how the recommendation grounds the prose, when to save it, and the human review
boundary. It links directly to the full MCP procedure and warns that Relicta stores but does not
fact-check agent-authored text. This makes the primary customer-notes use case discoverable during
first-use onboarding. `git diff --check` passes; documentation build not run.

## REGENERATED: CLI reference now matches the candidate commands (2026-11-26)

Ran the repository's `make docs-cli` generator against the current candidate. It refreshed 69
existing references and created 22 references for current commands that were missing from the
published documentation tree, including `audit`, `cgp`, database, deployment, policy, Hub, and
webhook commands. The generated notes reference now documents `--draft-file` and the MCP handoff;
the MCP server reference documents its loopback default and the guard required for non-loopback
HTTP. `make docs-cli` completed successfully and `git diff --check` passes. The change is generated
from the current candidate CLI and has not been checked into a release or hosted docs site.

## VERIFIED: generated command reference links resolve (2026-11-26)

After refreshing the CLI docs, checked all relative Markdown targets in the 92 pages under
`docs/cli/`. All local file targets exist, including the 22 newly generated command references.
`git diff --check` passes. This was a link-target audit, not a rendered documentation build or
hosted docs test.

## CORRECTED: public docs CLI guide matches published v4.2.0 (2026-11-26)

The separate Starlight docs site's CLI page still advertised removed `relicta release --ai` and
`--save` flags, used the obsolete `relicta.config.yaml` filename, and did not distinguish published
v4.2.0 from v5 source. Rewrote it against the actual `v4.2.0` tag: clarified that `--ai` belongs to
`relicta notes`, explained the approval implications of `release --yes`, linked to the versioned
command reference, and marked the MCP agent-session notes workflow as unreleased. `npm run check`
reported zero diagnostics; `npm run build` generated all 11 docs routes. The rendered CLI page
contains v4.2.0 and correct `notes --ai` guidance, and no longer contains the obsolete flags or
config filename. The docs checkout has other pre-existing uncommitted work and no deployment was
run.

## CORRECTED: public workflow guide uses v4.2.0 command semantics (2026-11-26)

Comparing the public docs site's workflow page with the `v4.2.0` tag found more obsolete flags:
`notes -a` was presented as AI (it is audience), `--save` was unsupported, and approval flags
`--by`/`--note` were not present in the release. Rewrote the page from the tagged command references,
added the published-version boundary, and clarified that `--yes`/`--skip-approval` bypass
interactive approval and require equivalent policy controls. Docs `npm run check` reports zero
diagnostics; the build generates all 11 routes; rendered workflow output includes the supported
v4.2.0 flags and excludes the stale ones. `git diff --check` passes. No deploy was run.

## VERIFIED AND IMPROVED: public Linux install path is release-pinned (2026-11-26)

The docs site's install page sent readers to verification commands on mutable `main`. Replaced the
Linux pointer with a self-contained v4.2.0 installer that verifies the tag-specific Sigstore-signed
checksum manifest, archive SHA-256, and GitHub workflow provenance before extracting or installing.
Against the live Linux amd64 archive, Cosign returned `Verified OK`, the manifest hash check returned
`OK`, and `gh attestation verify` succeeded for `refs/tags/v4.2.0`. The documented shell snippet
passes `bash -n`; Starlight check reports zero diagnostics and the docs build generates all 11
routes. This verifies v4.2.0's Linux amd64 archive and the snippet syntax; it is not a test of the
ARM64 branch or a production deployment.

## VERIFIED: published Windows archive matches its release evidence (2026-11-26)

Downloaded `relicta_windows_amd64.zip` from v4.2.0 and verified the release checksum bundle with
Cosign (`Verified OK`), matched the archive's SHA-256 manifest entry (`OK`), and verified the
archive's GitHub attestation for the release workflow and `refs/tags/v4.2.0`. Added the pinned
PowerShell installation procedure directly to the docs site's install page, removing dependence on
mutable `main` instructions. The docs site check reports zero diagnostics and the build generates
all 11 routes; rendered output includes the archive, hash, attestation, and extraction commands.
PowerShell was unavailable in this environment, so the script was not executed; artifact evidence
was verified independently. No deployment was run.

## CORRECTED: public configuration, governance, CI/CD, and MCP guides reconciled to v4.2.0 (2026-11-26)

The separate Relicta docs site contained additional stale claims: an obsolete config filename and flags, unverified CGP risk weights, action inputs/approval behavior presented without clear source, and MCP capabilities and security guidance mixed with unreleased functionality. Updated the configuration, governance, CI/CD, and MCP guides against the published v4.2.0 source and current action metadata. Corrected the tutorial's unsupported `release --ai` example. The docs check reports 0 errors/warnings/hints; the static build generates all 11 routes; `git diff --check` passes. The site was not deployed. The action input summary links to its live metadata because that interface can change independently of the CLI release.

## HARDENED: public CI example pins GitHub Actions (2026-11-26)

The production CI/CD example used mutable major-version references for both checkout and the Relicta action, and its action metadata link targeted mutable `main`. Replaced these with reviewed commit SHAs and linked the matching Relicta action metadata revision. Verified `relicta-tech/relicta-action` v2 resolves to `0268e9a3001e9483874e5501d59fa929a42130f3` and `actions/checkout` v4 resolves to `11d5960a326750d5838078e36cf38b85af677262` at this review. Astro check reports zero diagnostics; build generates all 11 docs routes; `git diff --check` passes. Recheck pins before updating the example. No deployment.

## DOGFOOD VERIFIED: agent-session notes recommendation on Govee maintenance release (2026-11-26)

Built the current Relicta candidate and ran `plan --dry-run --skip-cognitive --json` from the existing Govee Light Management checkout. It produced a deterministic patch recommendation for 3.1.3 from seven commits since v3.1.2, with zero features and one dependency fix; inspected the dependency commit, whose message says it clears a high-severity npm audit finding. Used those facts to refine the Govee MCP example into customer-facing maintenance notes and explicitly disclosed that the read-only recommendation excludes 13 uncommitted checkout changes. No live plan, bump, saved release notes, tag, or publish was performed, preserving the repository's manual release process. Targeted `go test ./internal/mcp ./internal/domain/release/app` passed. This validates recommendation generation and unit-level draft persistence, not an end-to-end persisted release or approved Govee announcement.

## VERIFIED END TO END: Govee notes draft persists in isolated release run (2026-11-26)

To test the complete safe authoring path without touching Govee's dirty working tree, created a clean local clone at committed `main` and used the current candidate binary: `plan` created a v3.1.3 patch run from seven commits, `bump` versioned that run, and `notes --draft-file` saved the customer-facing draft. The notes response returned the exact text, provider `agent`, release ID, and `notes_ready`; a subsequent `status` reported the same release ID and `stale: false`. The clone used Govee's config with Git tag/push and GitHub publishing disabled; approval and publishing were not called. Existing MCP/app tests pass. The artifact was a controlled local exercise, not an approved Govee release or hosted environment validation.

## VERIFIED: dashboard dependency cleanup and complete npm audit (2026-11-26)

Removed the unused deprecated `lucide-vue-next` package. A clean install then exposed moderate
findings in the dashboard's ESLint/minimatch development toolchain; updated the compatible lockfile
versions to ESLint 10.15.0, typescript-eslint 8.74.0, and minimatch 11.0.9. From the updated lock,
`npm ci --ignore-scripts`, `npm run build`, `npm run lint:check`, and the full `npm audit` all pass;
npm reports zero vulnerabilities. This is local dashboard dependency and build evidence, not hosted
CI or deployment validation.

## FIXED: dashboard accessibility gate now tests the intended routes (2026-11-26)

A local run exposed that Playwright reused another app already listening on port 5173, so the
accessibility suite could pass against an unrelated login screen. The governance test also opened
`/governance/analytics`, which is not a Relicta route. Updated the runner to use a strict dedicated
port (overridable with `PORT`/`BASE_URL`) and never reuse an existing server; added visible-content
assertions and corrected the governance path. On isolated port 5183, all seven Chromium/axe tests
pass against Relicta's dashboard, governance, release detail, and approval views. This validates the
local frontend routes; backend integration and hosted workflow execution remain separate checks.

## RECHECKED: dashboard and hosted release readiness (2026-11-26)

After the dashboard dependency and accessibility-gate updates, `npm run build`, `npm run lint:check`,
`npm audit`, the seven-test isolated Chromium/axe suite, and `git diff --check` pass locally. A
read-only GitHub recheck confirms PR #405 at `72d31c6` now has six passing PR-triggered workflows;
these validate that hosted commit only, not this 848-path dirty worktree. PR #404 remains draft at
`00854ed`; PR #422 remains at `08442b2` with its historical Nov 15 Nox failure (82 dependency
findings). Public `main` remains `247b693`. The current local Nox route heuristic assigns the same
medium `ATTACK-003` fingerprint to `/health` and `/api/v1/observability/health`; the latter is under
the authenticated route group, and the public handler returns only healthy status, timestamp,
uptime, and Go version. Treat this as a low-evidence heuristic, not proof that observability data is
public; no live attack verification ran. Protected approval configuration, fresh hosted checks on
the candidate, and actual release artifacts remain launch gates; no remote state was changed.

## VERIFIED: dashboard images rebuilt after dependency cleanup (2026-11-26)

Rebuilt the current dashboard builder and runtime images from the updated lockfile. Both build
successfully; the image's `npm ci` reports zero audit vulnerabilities. A fresh Trivy scan at
MEDIUM/HIGH/CRITICAL found zero OS and library findings in the final nginx runtime image. The
Node/npm builder image has two moderate findings in npm 12.2.3's bundled `glob` 13.0.6 and
`minimatch` 10.2.5; Trivy identifies fixes only in the next major versions (glob 14 and minimatch
11). Registry metadata confirms npm 12.2.3 is the current latest release and still declares those
versions. Since this tool runs only in the build stage and forcing incompatible major overrides
would alter npm's own dependency graph without upstream support, leave it visible as a medium build
image residual and recheck when npm publishes an updated release. Runtime image: zero M/H/C.

## HARDENED: Relicta GitHub Action installer and test gate (2026-11-26)

In the separate local `relicta-action` checkout, added installer tests for stable `latest` tag
resolution, exact release-tag Sigstore identity, prerelease rejection, checksum mismatch before
extraction, and duplicate checksum entries. The tests exposed and fixed an unescaped `.github`
segment in the trusted workflow identity regex. A clean `npm ci`, format check, lint, all four Jest
tests, ncc build, `npm audit` (zero vulnerabilities), and `git diff --check` pass. The generated
bundle includes the corrected identity. This remains local-only: the draft Action PR's successful
hosted runs cover a different remote commit, so fresh hosted checks are still required before
merging. No push or publication was performed.

## TIGHTENED: release audit approval is bound to the reviewed plan (2026-11-26)

The release workflow's transferred audit-chain check previously accepted any permitted
decision for a release ID, without requiring the chain to contain the matching protected
human approval. The verifier now requires an `approval.granted` event for the same release,
exact plan hash, GitHub reviewer identity, and `autoApproved=false`, in addition to the
permitted decision. Both the approval job and publication job pass those expected values.
This is a local source change; hosted workflow execution has not validated it.

## RECHECKED: candidate CI and release snapshot (2026-11-26)

The targeted release-audit regression suite passes (`go test ./scripts
./internal/application/governance`), as do `actionlint .github/workflows/release.yaml` and
`git diff --check`. Every component of `make check-ci` passes locally: formatting, `go vet`,
golangci-lint, the full unit-test set, all 27 coverctl domains, and the Nox 1.70 CI-equivalent
scan from a clean plugin home (796 dependencies checked, no findings and no degraded checks).
The first coverage attempt stopped at 122 MB free; after clearing a separate unused, rebuildable
1.5 GB Go cache, the serial coverage run completed. The existing local release snapshot passes
`make validate-release-snapshot`: all three supported Linux/Windows
archives contain the embedded dashboard, their SPDX SBOMs validate, six checksum entries match,
and the Homebrew formula matches the manifest. This validates the existing snapshot artifacts,
not freshly built artifacts from the current dirty tree. The 848-path worktree remains local and
needs fresh hosted checks before release.

## FIXED: dirty-tree scope is explicit in release plans (2026-11-26)

Dogfooding the v5.0.0 recommendation exposed that `plan` analyzed committed history while leaving
848 local changes outside the recommendation without a clear notice. The CLI now warns that local
edits are excluded and reports `working_tree.clean` plus
`working_tree.uncommitted_changes_included: false` in JSON for normal, reviewed, and tag-push
plans. Added a JSON contract regression test. `go test ./internal/cli` and `git diff --check` pass.
The recommendation remains commit-based; users must commit intended release content before planning
again. This is local-only validation, and the dirty candidate still needs fresh hosted checks.

## VERIFIED: protected observability routes reject unauthenticated requests (2026-11-26)

The remaining Nox ATTACK-003 health-route match shares a fingerprint with the intentional public
`/health` probes because the scanner flattens Chi route groups. The matched observability handlers
can expose deployment health and provider status, so added an HTTP integration regression covering
unauthenticated health, correlations, provider-status, and webhook requests. All four return 401;
the public versioned health probe remains available. The focused `internal/httpserver` test passes.
This verifies route behavior in-process; it does not show that Nox understands middleware grouping,
and the heuristic remains visible for review.

## CORRECTED LOCALLY: product page describes the approval boundary (2026-11-26)

The public product page said approval happened before all release side effects. That was inaccurate:
planning, version updates, and note drafting precede approval, while configured approval governs
publication. Corrected the local product page to state that sequence. The separate Astro site passes
`npm run check` with zero diagnostics and builds all seven pages. The live page still serves older
copy, so this correction remains local pending the site's normal reviewed deployment path.

## RECHECKED: current candidate local Go preflight (2026-11-26)

On the current worktree, `make check` passes: formatting, `go vet ./...`, golangci-lint (0 issues),
and test compilation across `internal/...` and `pkg/...`. The dirty-tree CLI contract test and the
unauthenticated observability integration test also pass individually. This preflight does not run
the full Go suite, coverage gate, security scan, or hosted workflows; those results remain scoped to
their own recorded runs and candidate revisions.

## CORRECTED LOCALLY: market metrics and plugin registry count (2026-11-26)

Reconciled the marketing copy against the v4.2.0 tag. CGP calculates eight potential risk factors
(seven baseline factors and historical risk when history is available), so the homepage now says
"8 potential release-risk factors" rather than implying there are seven. The AI provider count is
five and `relicta communicate` supports four audiences. The product page lists four featured
plugins plus 25 additional registry entries; its count now matches the displayed list and calls
them registry entries because only five are marked verified in `plugins/registry.yaml`. The
separate market site's Astro check and production build pass. This verifies local source only;
the live Pages site has not been deployed.

## RECHECKED LOCALLY: release workflow and helper syntax (2026-11-26)

Reviewed the current tag-release path from governance through human approval, digest-scanned
container publication, attested binary release, Homebrew update, and outcome recording. The
workflow requires a protected `release-approval` environment with a required reviewer and
self-review prevention before it will publish. `actionlint .github/workflows/release.yaml`,
ShellCheck on the release verification, snapshot validation, Homebrew publisher, and restricted
container smoke scripts, and `git diff --check` all pass. This is static/local evidence; the
workflow has not run on the current candidate in GitHub, and the required environment is absent.

## UPDATED: market positioning accounts for direct agent-release competition (2026-11-26)

Checked current alternatives against primary product sources. Digital.ai Release 26.3 now announces
agent orchestration, a Release MCP server, policy, and audit trails; GitHub already generates
release notes, and Release Please automates changelogs, version bumps, release PRs, and releases.
Updated `docs/positioning.md` to avoid a uniqueness claim, focus Relicta on repository-level
release decisions and auditable approvals for teams that do not want an enterprise orchestration
suite, and label this as a market hypothesis. Recorded that external interviews, conversion,
retention, and willingness-to-pay evidence are still missing. Sources and date are in the brief;
`git diff --check` passes. This is a desk-research comparison, not validated customer demand.

## PREPARED: customer discovery guide for Relicta demand validation (2026-11-26)

Added `docs/customer-discovery.md` with a 30-minute, evidence-led interview, recruit segments,
questions about recent release behavior and current workarounds, a neutral prototype walkthrough,
per-interview evidence fields, and decision criteria. It distinguishes internal Govee dogfooding
from external customer validation and avoids counting compliments or hypothetical purchase intent
as demand. Linked it from the positioning brief. No external participants have been contacted and
no interviews are represented as completed; the next evidence step requires real release owners.

## FIXED LOCALLY: tag releases enforce allowed-branch policy (2026-11-26)

The hosted tag-triggered release workflow validated tag identity and governance but published
through dedicated jobs without calling the CLI's normal pre-publish branch gate. Added a verifier
that loads `workflow.allowed_branches` and requires the tagged commit to be reachable from a
matching `origin` branch before a release plan can proceed. The verifier lives in its own Go
command package so the repository-wide Go checks can compile it, and ignores Git's symbolic
`origin/HEAD` ref to match the CLI's real branch names. The disposable release repository accepted
a commit on `origin/main` and rejected a child commit reachable only from an unauthorized feature
branch. Added regression coverage for ancestry, symbolic refs, and glob matching; the focused Go
test, repository `make check`, `actionlint`, and `git diff --check` pass. This closes the local
policy gap; fresh hosted checks on the candidate and the protected GitHub release environment
remain outstanding.

## RECHECKED: current candidate CI and nine-repository dogfood (2026-11-26)

After the tag-branch verifier change, `make check-ci` passed formatting, `go vet`, golangci-lint,
the full Go and docs test suites, and all 27 configured coverage domains. The Nox step exited
successfully but warned that 14 globally installed optional plugins were not declared and did not
run. Repeated the exact high-severity, fail-on-degraded Nox gate with a fresh `NOX_HOME`: Nox 1.70.0
reported zero findings, 288 suppressed records, 796 dependencies, 9 AI components, and no degraded
checks. Rebuilt the CLI from this candidate and repeated read-only
`plan --dry-run --json --no-ai` across nine configured repositories. All returned structured
recommendations: Govee 3.1.2→3.1.3 patch (7
commits), Roady 0.27.3→0.28.0 minor (5), Specular 1.11.0→1.11.1 patch (9), Nox 1.70.0→1.71.0
minor (25), mcp-rs 0.0.0→0.1.0 minor (5), TokenOps 0.120.1→0.120.2 patch (2), Dispatch
0.2.0→0.3.0 minor (36), Nomi 0.3.0→0.4.0 major (1), and Relicta 4.2.0→5.0.0 major (168).
Relicta's recommendation remains `approval_required`, risk 0.8, and not auto-approvable. Seven
repositories reported dirty working trees, with JSON explicitly stating that uncommitted changes
were excluded; mcp-rs and Nomi were clean. No release state, tag, or publication was changed.
These are local candidate results; hosted checks on the candidate and external customer validation
remain outstanding.

## CHECKED: public inbound-demand signal remains absent (2026-11-26)

Read the public Relicta issue history through GitHub: all 42 issues currently returned were opened
by the project owner; there were no independently opened customer requests. The repository showed
0 stars and 0 forks, while the v4.2.0 platform archives showed 54 downloads in total. Added these
as weak, ambiguous adoption signals to `docs/positioning.md`: downloads are not unique users, and
the public metrics omit private use and other install channels. They do not prove that a market
does not exist, but organic pull is not demonstrated. External release-owner interviews and
willingness-to-pay evidence remain the market validation gate.

## RECHECKED LOCALLY: release snapshot builds and artifact validation passes (2026-11-26)

Built a fresh non-macOS snapshot from the current candidate into an isolated distribution folder.
GoReleaser completed its frontend, module-integrity, cross-platform build, archive, and SPDX SBOM
steps. `scripts/validate-release-snapshot.sh` verified all three Linux/Windows archives contain
the embedded dashboard, all SBOMs are valid SPDX JSON, the six checksum entries match, and the
Linux-only Homebrew formula matches the release checksums. The generated snapshot was removed
after validation; the committed/source worktree diff passes `git diff --check`.

Read-only GitHub checks on the same date show that the operational release gates remain absent:
the repository has only an unprotected `copilot` environment, not the workflow's required
`release-approval` environment, and `main` has no required pull-request reviews or status checks.
PR #405 remains open at the checkout's committed SHA `72d31c6`; the current local candidate
changes are uncommitted and therefore are not covered by those hosted checks. No remote settings,
branches, or PR state were changed. A fresh hosted run and maintainer review remain required before
production release.

## VERIFIED: published v4.2.0 Linux integrity and provenance (2026-11-26)

Downloaded the public Linux amd64 archive, checksum manifest, Sigstore bundle, and SPDX SBOM for
v4.2.0. The archive SHA-256 matches the published manifest. `cosign verify-blob` accepts the
manifest signature for the exact Relicta release workflow identity and GitHub Actions OIDC issuer.
`gh attestation verify` accepts the archive's SLSA provenance when constrained to
`relicta-tech/relicta` and `.github/workflows/release.yaml`, with a Rekor transparency-log
timestamp. This confirms the integrity and provenance path for one current published artifact; it
does not verify every platform artifact or the unreleased local candidate's hosted provenance.

Follow-up platform audit downloaded all five v4.2.0 archives and SBOMs. Each archive's SHA-256
matches the published checksum manifest, and each SBOM parses as SPDX JSON. GitHub attestation
verification constrained to the Relicta repository and release workflow succeeds for Linux amd64,
Linux arm64, Windows amd64, and both macOS architectures. The Windows executable in v4.2.0 does
not contain the candidate's embedded-dashboard markers; README installation guidance already
explains the current public release's dashboard limitation and build-from-source workaround. The
new local candidate's snapshot validator separately enforces embedded dashboard presence for the
next release. Artifact integrity does not provide Apple code signing or notarization, which remain
unconfigured and are disclosed in the install guidance.

## CORRECTED LOCALLY: dashboard availability in market-site copy (2026-11-26)

The market-site homepage said release managers could review risk and approvals in the dashboard
without qualifying that capability by release. Inspected the published v4.2.0 Linux amd64 and
Windows amd64 binaries; neither contains the embedded dashboard assets. Updated the homepage to
describe the CLI review path and disclose that the v4.2.0 binaries omit the dashboard, with a
source-build route for teams that need it. `npm run check` reports zero diagnostics and
`npm run build` produces all seven pages. This corrects local site source only; no production
deployment was made.

## RECHECKED: live market site and install guide still serve stale claims (2026-11-26)

Read the public homepage and `docs.relicta.tech/install` on 2026-11-26. The live homepage still
claims Relicta is the only MCP release tool, advertises seven risk factors, describes a real-time
dashboard without release qualifications, and presents several v5 capabilities as current. The
live install page recommends a generic Homebrew install, gives `Darwin_aarch64`/`Linux_x86_64`
archive names that do not match the published v4.2.0 assets, and uses an unversioned module path;
it also omits the warning that published macOS archives are unsigned. The current local site and
docs source already contain the corrected v4.2.0 install paths, provenance checks, dashboard and
v5 availability boundaries, and platform warnings. Both source sites pass `npm run check` with
zero diagnostics; their builds produce seven marketing pages and 11 documentation pages. The live
copy remains stale because no deployment was run; production site accuracy is not complete until
the reviewed local changes are deployed.

## UPDATED: MCP release-note prompts bound product context to verified changes (2026-11-26)

Revised the read-only `release-summary` MCP prompt so Relicta plan facts remain the only source for
what changed, while agent-session product context can explain relevance without adding release
claims. The prompt directs the agent to omit unclear customer impact or flag it for human review,
and to focus on customer impact of verified changes. Added regression assertions for these prompt
requirements. `go test ./internal/mcp` and `git diff --check` pass. Updated `docs/positioning.md` with current evidence that ReleasePad already offers
release-note drafting via AI assistant/MCP, plus caveated public anecdotes that identify filtering
engineering noise and factual completeness as user concerns. This sharpens the testable product
hypothesis around grounded notes attached to a reviewed release decision; it does not establish
market demand. No saved draft, release approval, publication, site deployment, or remote change
was performed. A follow-up update to `docs/customer-discovery.md` now compares the same release-note
task against each participant's existing workflow and records draft/review time, factual
corrections, missed user-visible changes, and engineering noise. The focused MCP test passed.
`make check-ci` passed formatting, vet, and lint, and unit tests ran, but the coverage phase could
not link several test binaries because the machine had only 668 MB free; Go reported `no space left
on device`. The run's temporary build directory is gone. Disk inspection found 3.1 GB under the
user's shared temp directory, including large tool caches and temp directories; none were deleted.

## UPDATED: release setup guide includes default-branch protections (2026-11-26)

Extended `docs/security/supply-chain-security.md` with a prerequisite for pull requests, independent
review, review of the latest push, resolved conversations, observed CI/security/release checks,
and force-push/deletion restrictions. The instructions require selecting check contexts from a
successful candidate run and verifying that failures and missing reviews block merges; they do not
guess at status names. Linked GitHub's current documentation for rulesets and protected branches.
`actionlint` passes for the tag-release workflow; `shellcheck` and `bash -n` pass for the four
release helper scripts. This is documented setup only; repository protections and the
`release-approval` environment remain external configuration, unchanged by this work.

## RECHECKED: sequential MCP-domain coverage passes (2026-11-26)


Re-ran `coverctl check` with Go test package parallelism limited to one after the parallel run
failed to link binaries under disk pressure. Every Go package and the integration suite passed;
the configured MCP coverage domain measured 62.7%, above its 45% minimum. The Go test output
reported 55.0% for the `internal/mcp` package itself, which is a narrower figure than the
domain aggregate. The selected-domain report's six warnings mean 99 other
directories are outside this domain-filtered report, not that those directories failed; this is
not a substitute for the full coverage gate. The temporary profile was removed. `main` branch
protection was rechecked through GitHub: force pushes and deletions are blocked, but required
reviews and status checks are both unset. The only environment is `copilot`; `release-approval`
is still absent. PR #405 remains open at `72d31c6`, with its last successful hosted checks from
2026-10-28, so local candidate changes still lack fresh hosted evidence. No remote state changed.

## BLOCKED: all-domain coverage run (2026-11-26)

Attempted the full configured coverage gate sequentially with a fresh profile. Unit packages and
`test/integration` completed, but `test/differential` failed while starting its PostgreSQL
Testcontainer: Docker reported the container log as read-only, and the readiness check timed out
before observing PostgreSQL's second ready message. The run therefore did not produce a full
coverage-gate result. It left a 248 MB profile, which was removed; free disk space returned to
about 452 MB. This failure is separate from the successful MCP-domain run above and does not
establish whether the other coverage domains pass. Retry the complete gate after Docker's writable
storage is restored. A read-only follow-up confirmed Docker 29.9.3 is using `overlay2`; both
containers left by that Testcontainers session carry its matching session label. Removing only
those two identified test containers also failed because their overlay filesystems report
`read-only file system`. The other shared Docker containers were left untouched. Restoring Docker's
writable storage is required before retrying the full gate; no Docker restart was attempted because
it would interrupt unrelated running services.

## CORRECTED: customer-note example avoids unsupported absence claims (2026-11-26)

Reviewed the Govee maintenance-note example against Relicta's recommendation. It asserted that
no new lighting controls were included solely because the recommendation had no feature commits;
that classification does not prove the absence of such changes. Removed the unsupported negative
claim and clarified the review guidance: verify absence claims against the reviewed change set,
not only the presence or absence of matching classified commits. The sample now retains only the
supported maintenance and npm audit facts. Documentation-only change; no tests or release actions
were run.

## CLARIFIED: MCP customer-note drafts must match the live plan (2026-11-26)

The documented workflow can commit a customer-facing notes file between a read-only dry run and
the persisted plan. That commit can change the recommendation, while the original instructions
only said to recheck it. Added a required freshness review of version, release type, classified
changes, and supported caveats; the guide now directs the agent to discard and rewrite the draft
when those release facts differ. This reinforces the plan-bound notes workflow without changing
the state machine. Documentation-only change; no tests or release actions were run.

## REFINED: customer-note market wedge accounts for ReleasePad MCP (2026-11-26)

Rechecked ReleasePad's own September 2026 changelog. It describes MCP drafting, editing, and
publishing from coding assistants, plus reusable brand-voice instructions. The previous market
wording treated agent-session drafting as a possible differentiator even though that workflow is
already offered by a direct competitor. Updated `docs/positioning.md` to treat session-based
drafting as parity and frame Relicta's testable wedge as linking the exact customer note to the
version, plan facts, policy decision, and human approval in one self-hostable release record. The
segment and willingness-to-adopt claim remain hypotheses pending external interviews. Evidence:
[ReleasePad MCP and brand-voice changelog](https://pro.releasepad.io/en/releasepad?page=1). No
product availability or release claims were changed.

## UPDATED: customer discovery directly tests release-record linkage (2026-11-26)

Extended the interview comparison protocol to treat coding-agent draft creation as table stakes
where ReleasePad or similar MCP-enabled changelog tools are in use. The side-by-side trial now
explicitly measures whether Relicta's version, verified change facts, policy result, and approver
link save evidence work or add unwanted configuration. It also compares unsupported claims and
missed user-visible changes rather than assuming Relicta's plan is more accurate. Added evidence
capture fields for traceability and evidence-work saved/added. This makes the current market
hypothesis falsifiable; it is not a substitute for interviews. No external participants were
contacted and no tests were run.

## CORRECTED: Govee release-note sample does not imply customer exposure (2026-11-26)

Rechecked the dogfood evidence behind the customer-note example. The commit message records a
high-severity npm audit finding, but the available release recommendation does not identify
affected shipped code or customer exposure. Removed the severity from the sample customer copy and
added guidance to verify product impact before making security claims to customers. The example
now states only that dependencies and automated security/build workflows were updated. This
corrects the earlier sample without asserting that the finding was or was not customer-impacting.
Documentation-only change; no tests or release actions were run.

## HARDENED LOCALLY: Relicta Action tag release verifies its runtime bundle (2026-11-26)

In the existing local `relicta-action` worktree, the release workflow builds the JavaScript action
bundle and uploads build outputs, but did not check that the tag's checked-in `dist/index.js` (the
file consumers execute through `action.yaml`) matched the source at that tag. Added a pre-release
parity check matching the pull-request workflow, so a stale or divergent bundle fails before the
GitHub release is created. Disabled checkout credential persistence so `npm ci` and the bundle
build do not inherit the workflow's write-capable GitHub credential from `.git/config`. The
workflow passes `actionlint` and `git diff --check`; the existing bundle includes the local
signed-manifest verification path. No tests were run. A fresh read-only GitHub check confirms
Relicta PR #405 remains open at `72d31c6` with October hosted checks, while Action PR #7 remains a
draft at `9db954c`. The Action worktree contains local security changes beyond PR #7, including
publisher authentication, exact-tag verification, and the new release workflow checks. Nothing
was pushed or released. The v2 published Action still has the fail-open checksum behavior
documented above, so production use remains gated on review, fresh hosted checks, and an explicitly
authorized release.

## VERIFIED: market site risk claims and static build (2026-11-26)

Recovered the interrupted homepage edit after disk space returned and removed only the duplicated
trailing style closure. The page now describes risk signals as configured, labels outcome
calibration as roadmap work, and narrows the audience language to teams shipping from Git. Astro's
`npm run check` reports zero errors, warnings, or hints across 16 files; `npm run build` generates
all seven static routes; and `git diff --check` passes. This validates the local site source only:
no deployment was performed, and the public homepage remains unchanged until an authorized
release of the site.

## CLARIFIED: MCP market demo labels synthetic results (2026-11-26)

The homepage's MCP conversation showed exact risk scores, affected packages, a proposed version,
and approval count without the caveat already attached to the CLI walkthrough. Added an adjacent
label explaining that those findings and approvals depend on the repository and configured policy.
The homepage still separates public v4.2.0 behavior from unreleased v5 agent-session notes and MCP
Apps. Re-ran Astro check (0 errors, warnings, or hints across 16 files), the seven-route static
build, and `scripts/verify-build.mjs`; all passed, as did `git diff --check`. No deployment was
performed.

## RECHECKED: current hosted PR signals are green but review is absent (2026-11-26)

Read-only GitHub checks show all six PR-triggered workflows associated with Relicta PR #405's
current hosted head completed successfully, and both Action PR #7 test workflows completed
successfully. PR #405 has no submitted reviews and remains on `72d31c6`; its 44.21 MB binary-size
comment is an advisory against the workflow's 20 MB optimization target, not a failed required
check. Action PR #7 remains a draft at `9db954c`; the signed-publisher changes and release-workflow
hardening in its local worktree are not part of that hosted revision. No remote state changed.

## REBUILT: local Action bundle matches signed-manifest installer source (2026-11-26)

Rebuilt `dist/index.js` from the current Action worktree with `npm run build` (3.286 MB output).
`actionlint` passes on the release, test, and E2E workflows, and `git diff --check` passes. The
local checkout still has additional installer, test, and workflow edits beyond hosted Action PR #7;
this build is local evidence only. No tests were run and nothing was pushed or released.

## RECHECKED: Docker daemon is unavailable for full coverage (2026-11-26)

After disk space was restored, a read-only `docker info` check reported that the local Docker API socket
is absent, so the differential PostgreSQL Testcontainers run cannot currently start. I did not start
or restart Docker because the host may run unrelated services. The full all-domain coverage gate
remains unverified until Docker is available; the earlier MCP-domain result does not replace it.

## CLARIFIED: Product page identifies the current release on entry (2026-11-26)

Added the v4.2.0 availability label to the standalone Product page hero so visitors arriving directly
there can distinguish the shipped product before reaching the later evolution and v5 concept sections.
Astro check reports zero errors, warnings, or hints across 16 files; the build emits all seven routes;
the static output verifier passes; and `git diff --check` passes. No deployment was performed.

## REVERIFIED: published v4.2.0 assets match local install guide (2026-11-26)

Queried GitHub's latest stable release and its v4.2.0 tag. Both identify v4.2.0 as the latest
non-draft, non-prerelease release (published 2026-07-10). The release contains signed checksum
metadata and archives for Linux amd64/arm64, Windows amd64, and macOS amd64/arm64. The local
install guide names these targets and correctly warns that macOS archives are unsigned, while
providing a Go module/source-build alternative. No install-guide edit was needed. This confirms
published artifact availability only; it does not change the separately recorded fact that the
corrected local site/docs sources have not been deployed, or satisfy the remaining production
gates.


## RECHECKED: current public site still blocks market readiness (2026-11-26)

Reopened the public homepage and install guide. The public homepage still advertises unsupported
exclusive/first claims, current risk learning and MCP Apps, and unqualified dashboard and v5
capabilities. The public install guide still recommends unsigned macOS downloads, outdated archive
filenames, and an unpinned Go module path. Local marketing and docs source now correct these claims;
`npm run check`, `npm run build`, the marketing and docs static-output verifiers, and `git diff
--check` all pass (7 marketing pages and 11 docs pages). The checked-in deployment workflow is
main-only, SHA-pinned, and verifies both build outputs before deploying. No production deployment
was made. GitHub API access returned an empty public ruleset list; branch-protection details are
not readable to the connected integration. The public-facing mismatch remains open until changes
are reviewed, merged, deployed, and confirmed on the live domains.


## DOGFOODED: agent-authored release notes saved through MCP (2026-11-26)

Built the local Relicta candidate and used its loopback MCP server against the Govee Light
Management repository. `relicta_plan` returned a deterministic dry-run recommendation from v3.1.2
through HEAD: seven committed changes and a v3.1.3 patch; the output explicitly excluded current
uncommitted dial-feedback edits. The existing local run was advanced from planned to versioned via
MCP `relicta_bump`; the repository config has no version file and disables Git tagging/pushing, and
the response confirmed `tag_created: false`. The active agent then supplied customer-facing text to
`relicta_notes`. MCP returned and persisted it as `agent_authored: true`, `source: agent`, with state
`notes_ready`; `relicta_status` read it back and reported approval as the next action. The initial
draft mentioned selected dependency/security updates and improved automated checks, but also said
lighting controls were unchanged, a claim the recommendation alone did not establish. On
2026-11-26, rebuilt the current local candidate and replaced that sentence through
`relicta notes --draft-file` with the supported statement that the update refreshes project
dependencies and improves automated security checks. A dry run showed the exact replacement before
saving; `relicta_status` and the persisted release JSON confirmed the same text and `notes_ready`
state. No approval, Git tag, push, or publication was performed, and the Govee tracked working
changes remain unchanged. This dogfood both validates the plan-to-agent-draft path and exposes a
manual-release friction: the record must be versioned before an agent-authored note can be saved.
No draft/review time measurement or external customer evidence was collected.

## PRODUCT SAFETY FINDING: stale branches can plan below an existing version tag (2026-11-26)

Built the current local candidate and ran `plan --dry-run --minimal --no-ai --skip-cognitive
--no-color --json` in all 17 discovered repositories with their own `.relicta.yaml`. All 17
completed. Relative to installed v4.2.0, the candidate also handles the two worktree repositories
whose Git config includes `worktreeconfig` and validates both Nox configurations successfully.
These are dry-run compatibility results only; they do not establish release readiness for each
repository.

Three plans need operator attention before any release:

| Repository | Checked-out `main` | Highest local version tag | Candidate plan | Why it differs |
| --- | --- | --- | --- | --- |
| `klarlabs/oss/mcp-go` | 10 commits behind `origin/main` | v1.30.1 | v1.28.2 → v1.28.3 | Newer tags are not ancestors of local HEAD; config permits planning from an outdated checkout. |
| `klarlabs/oss/bolt` | 1 commit behind `origin/main` | v3.1.3 | v1.7.0 → v1.7.1 | v3.1.3 is on a divergent history, not an ancestor of local `main`. |
| `open_source/tokenops` | 48 commits behind `origin/main` | v0.126.0 | v0.120.1 → v0.120.2 | Newer release tags are ahead of local `main`; config requires an up-to-date branch at publish time but dry-run planning still reports the lower proposal. |

Confirmed with `git merge-base --is-ancestor`: for each repository, the newest tag used by the
candidate is reachable from HEAD, while the higher version tag and `origin/main` are not. Therefore
the commit-range selection itself is coherent; the risk was that output did not call out the higher
existing tag or stale/divergent branch. **Implemented locally:** the analyzer now attaches a warning
when a higher matching version tag is not an ancestor of the target; CLI text, CLI JSON and MCP plan
responses carry the warning, naming the higher tag, target and reachable baseline. The CLI command
guide and MCP guide explain how to interpret it. Built the CLI and re-ran dry-run planning in the
three affected repositories; each preserved its original branch-local version recommendation and
now reports the correct warning. `git diff --check` passes. No tests were added or run; the code has
only build and live dry-run verification so far.

**Acknowledgment implemented and exercised locally:** a non-dry-run CLI plan with this warning
stops unless `--allow-diverged-version-line` is supplied; MCP requires
`allow_diverged_version_line: true`. Dry-runs remain inspectable without acknowledgment. In an
isolated Bolt clone, the CLI refused without the flag and wrote no release run. With the flag it
planned from the reachable `v1.7.0` baseline, reported `version_line_acknowledged: true`, and stored
the acknowledgment plus exact warning in the `PLAN` transition. An actual stdio MCP
initialize/`tools/call` exchange likewise persisted an acknowledged plan from `v1.7.0` and returned
the warning and acknowledgment in structured output. This exposed and fixed a related persistence
bug: CLI and MCP now pass the analyzer's exact `ChangeSet.FromRef()` into `PlanRelease`, rather than
re-detecting the highest tag and failing to persist plans on a divergent maintenance line. Both
guides document the operator action. `go build -o /tmp/relicta-ack-gate ./cmd/relicta` and
`git diff --check` pass. No tests were added or run, no approval/tag/push/publish occurred, and the
dogfooded Govee repository was not changed.

**Still open before production release:** confirm policy for intentional maintenance lines and
separately assess behind-remote branches, which may need an up-to-date checkout even when their
reachable baseline is coherent. These are release decisions; the warning and acknowledgment gate
make them explicit but do not decide them automatically.

Other candidate results from the 17-repository scan (base → next): dispatch 0.2.0→0.3.0; nomi
0.3.0→0.4.0; agent-go 0.16.1→0.16.2; briefkasten 0.38.0→0.39.0; scout 1.16.3→1.16.4;
KlarLabs Nox 1.70.0→1.71.0; coverctl 1.20.0→1.20.1; open-source Nox 1.39.1→1.39.2;
specular 1.11.0→1.11.1; mcp-rs 0.2.0→0.2.1; Govee 3.1.2→3.1.3; Relicta 4.2.0→5.0.0;
Roady 0.27.3→0.28.0. These are branch-local recommendations, not approved release decisions.
`mcp-go` is listed in the table above. No plan or state was persisted by these dry runs.

## RECHECKED: dependency tree builds cleanly; hosted review is still stale (2026-11-26)

In the current dependency-updates worktree, `npm ci` installed the lockfile tree (164 packages),
reported zero vulnerabilities, and `npm run build` passed `vue-tsc --noEmit` and rebuilt all six
MCP apps. `go build ./...` also passed. This resolves the TypeScript/Vue build failure described in
PR #405's original body for the current local tree. It does not establish that the hosted PR has
these edits: GitHub still reports PR #405 open at its October head `72d31c6`, with no reviews.

Dependabot PR #422 is still open at `08442b2`; its build and lint checks pass, while its Nox and
Security Scan jobs fail. Read-only diagnostics show those jobs ran on 2026-11-15 against merge base
`247b693` and Nox 1.26.0 reported 82 dependency findings. The existing artifact comparison records
that these findings are in the dependency files on the pre-update base, absent from PR #405's scan;
the current local candidate's clean-environment Nox 1.70.0 scan found zero HIGH/CRITICAL findings.
Refresh hosted checks only after the dependency update is integrated into the Dependabot tree; no
remote PR, branch, or workflow state was changed here. No tests were run in this recheck.

## SAFER: MCP HTTP deployment example requires reviewed image digests (2026-11-26)

Rechecked GitHub's latest stable release endpoint: v4.2.0 remains the newest published release.
The MCP HTTP Docker Compose example used `ghcr.io/relicta-tech/relicta:latest`, which resolves to
the currently published image documented in the security guide as having 60 HIGH/CRITICAL
findings. Replaced both that mutable reference and the proxy's mutable `caddy:2` reference with
required `RELICTA_IMAGE` and `PROXY_IMAGE` deployment variables, and linked the image verification
procedure. The example now states that v4.2.0 is not suitable for production and directs users to
build locally for evaluation or wait for a verified fixed release. `git diff --check` passes; no
tests were run and no hosted state changed.

## IMPROVED: Codex MCP onboarding supports multiple repositories (2026-11-26)

The MCP guide now distinguishes the user-level `codex mcp add` setup for a single repository from
project-scoped `.codex/config.toml` entries for multi-repository use. Each project entry binds the
server to an absolute checkout path, and the guide explains how to keep that machine-specific path
out of shared commits. It also warns against leaving a second user-level Relicta server enabled and
asks agents to verify the target repository and version before saving state. These instructions
match Codex's current MCP and trusted project-configuration behavior. `git diff --check` passes;
documentation-only change, with no tests or release actions run.

## SAFER: agent-authored MCP notes carry a fact-check warning (2026-11-26)

Review of the persisted Govee v3.1.3 dogfood draft found an unsupported sentence claiming light
control behavior was unchanged; the release recommendation excluded uncommitted dial-feedback
work and could not establish that negative claim. Replaced it in the local Relicta release record
with the supported statement that the update refreshes project dependencies and improves
automated security checks. `relicta_status` remained `notes_ready`, with approval pending and no
tag or tracked Govee changes. Relicta MCP status now attaches a warning to agent-authored notes
that they are not fact-checked and every claim must be checked against reviewed changes; the
existing approval elicitation includes this warning. Rebuilt the CLI and verified the exact warning
in a direct MCP stdio status call against the saved record. `git diff --check` passes; no
approval, publication, tests, or remote changes were performed.

## RECHECKED: live market copy still overstates shipped behavior (2026-11-26)

Opened the public homepage after the local copy corrections. The deployed page still labels the
product "Production Ready", describes risk scoring as having historical learning, claims Relicta
is the only release tool with native MCP, and presents a completed approval-and-publish transcript
with generated notes, auto-approval, GitHub/npm publication, Slack notification, and Sigstore
attestation. The local site source has already been narrowed to distinguish current v4.2.0 behavior
from unreleased capabilities and synthetic examples, but the deployed page has not caught up. The
local marketing-site edits are present in its worktree; no deploy was run. This remains a launch
blocker until the corrected copy is reviewed, deployed, and checked on the live domain.

`go vet ./...` completed successfully on the current Relicta source tree. This is static-analysis
evidence only and does not replace automated test coverage or hosted checks. No tests were run and
no remote state changed. Revalidated the corrected local marketing site: `npm run check` reports
zero errors, warnings, or hints across 16 files, and `npm run build` generates all seven pages.
`git diff --check` passes in that worktree. The generated site remains local and undeployed.
The built homepage was also checked directly: it contains the v4.2.0 availability label, the
unreleased v5 caveats for agent-authored notes and MCP Apps, and the illustrative-workflow caveat;
`scripts/verify-build.mjs src dist` passes. An apparent duplicate paragraph was only an artifact of
overlapping source excerpts; no source change was needed.

Dogfooded the current candidate's release decision path with `plan --dry-run --minimal --no-ai
--skip-cognitive --no-color --json`. From v4.2.0, it proposes v5.0.0 across 168 commits (41
features, 68 fixes, one breaking change), reports critical risk at 0.80, and requires human
approval. The breaking change is removal of the `--ai` flag in favor of `ai.enabled`; the output
explicitly says uncommitted work is excluded. The dry run created no release ID or persisted plan.
This validates the candidate's conservative release gate against Relicta's own major-change
history; it does not validate the uncommitted changes or hosted release pipeline.

## SAFER: public v5 release-note draft states agent prose is not fact-checked (2026-11-26)

The customer-facing `5.0.0.public.md` draft described agent-session note drafting and review, but
omitted that Relicta does not verify the prose itself. Added a concise instruction to check every
claim against the changes before approval, matching the existing MCP status warning and workflow
documentation. `git diff --check` passes. The draft remains unapproved and unpublished.

## SAFER: MCP approval warns on inline replacement notes (2026-11-26)

The approval elicitation included `relicta_status` note warnings only when the approval request did
not also provide replacement notes. An agent replacing the draft in the approval call could
therefore present unverified prose without the fact-check warning. The confirmation now uses a
specific agent-authored warning for inline replacement notes, keeps the status warning for saved
notes, and labels technical fallbacks only when that fallback is what reviewers will approve. The
MCP guide documents the behavior. `go build -o /tmp/relicta-readiness-notes-warning ./cmd/relicta`
and `go vet ./internal/mcp` pass. The default restrictive agent budget continues to deny approval
when human elicitation is unavailable; a repository may explicitly configure a different
cosigning policy. `git diff --check` passes. No tests were added or run; no approval or publication
occurred.

## VERIFIED LOCALLY: full source and production container build (2026-11-26)

On the current dependency-remediation worktree, `go build ./...` and `go vet ./...` both pass.
`goreleaser check --config .goreleaser.yaml` validates the release configuration. A fresh
`docker build --tag relicta:readiness-local .` completed, including the embedded dashboard build;
the frontend dependency audit reported zero vulnerabilities. Image inspection confirms the
container runs as `appuser:appgroup`; the local image is approximately 74 MB. `git diff --check`
passes. This verifies compilation, static analysis, release configuration syntax, and image
construction. Follow-up verification with Trivy's current vulnerability database found zero HIGH
or CRITICAL findings in the Alpine OS packages and embedded Go binary. `scripts/smoke-container.sh`
passed against the local image: initialization, patch planning from one feature commit, healthy
status, dashboard response, unauthenticated API rejection (401), authenticated API access (200),
and the container health check all behaved as expected under a read-only filesystem with dropped
capabilities and resource limits. The script initially lacked its executable bit; fixed the mode
and reran it directly, with the same pass. The hosted release remains v4.2.0 (published
2026-07-10); PR #405 is still open on the
baseline commit, so these local changes have not reached hosted CI or production. No release,
approval, tag, push, deployment, or publication occurred; the image remains local.

## VERIFIED LOCALLY: race-enabled CI coverage domains pass (2026-11-26)

The first full `coverctl check --race` run passed all packages but failed the CLI domain by two
covered statements: 4,739 of 8,781 (53.97%) against the configured 54% minimum. Added a regression
test for `relicta verify` returning a clear error when its configured public key cannot be read.
The focused test passes under the race detector, and the full race-enabled coverage gate now passes
all 27 configured domains, with CLI coverage at 54.1%. `git diff --check` passes. This is local
verification only; it does not represent hosted CI or a release.

## VERIFIED LOCALLY: dashboard and MCP app production checks (2026-11-26)

On the current worktree, `web` type checking, ESLint, and the Vite production build pass. The seven
Playwright/axe accessibility tests pass when run on an unused local port. The browser harness has no
API backend, so pages that request live data emit expected 404/403 console messages; the tests verify
accessibility with empty or mocked content, not live dashboard integration. The MCP companion app
also passes `npm run build`, including TypeScript checking and generation of all six embedded HTML
apps. No source files were changed by these builds. `git diff --check` passes.

## VERIFIED LOCALLY: dashboard image build, scan, and serving smoke check (2026-11-26)

Built both dashboard CI Docker targets from `web/docker/Dockerfile`. Trivy's current database reports
zero HIGH or CRITICAL vulnerabilities in the nginx runtime image and in the frontend builder image
(including npm libraries). The runtime container serves `/` and `/nginx-health` successfully as the
`nginx` user when configured with `RELICTA_API_URL=http://127.0.0.1:8080`. Running the image alone
with its default `RELICTA_API_URL=http://relicta:8080` exits because that compose service name is not
resolvable outside the app network; deployment must provide the app service or an explicit API URL.
The standalone smoke container was removed after verification. `git diff --check` passes. This is
local verification and does not replace the hosted workflow or an integrated end-to-end deployment.

## RECHECKED: hosted PR checks and local dashboard image (2026-11-26)

GitHub still reports PR #405 open on `72d31c6b83744f6f11100dd8f41b15145c23cc13`. Its current
workflow runs pass CI (lint/tests, Linux build, policy and dependency review, Nox, SBOM), web
accessibility, docs, benchmarks including the binary-size job, security, and skill preflight. This
is evidence for the committed PR head only; the 851-path local worktree is not represented in those
checks. The dashboard runtime and frontend-builder Docker images also build locally and pass the
same Trivy HIGH/CRITICAL vulnerability scan used by the dashboard workflow. The runtime serves its
static UI and health endpoint as the non-root nginx user when given a resolvable API URL. The PR
remains unmerged and its description still contains an outdated frontend-build failure note.

## ROADY PLAN RECONCILED: stale duplicate features removed (2026-11-26)

The repository's Roady specification described session authentication as unimplemented even
though session and OIDC flows are present, and duplicated the `cgp-protocol-wire-format` and
`audience-aware-release-communication` feature IDs. Updated the stale capability text, removed
the later duplicate entries while retaining the original richer definitions, and regenerated
the plan. `roady spec validate` now passes and the plan has 65 tasks (down from 67). The six
remaining pending findings include five whose code appears fixed in this worktree and a policy
conditions gap that needs implementation/product decisions. Their requirements have no Roady
acceptance checks, so Roady cannot verify them yet; no completion statuses were changed.
`roady drift detect` identified spec/plan mismatch before regeneration. `git diff --check` passes.

## TARGETED REGRESSION SUITES PASS (2026-11-26)

`go test ./internal/domain/sourcecontrol ./internal/domain/release/adapters ./internal/container
./internal/mcp ./internal/cli` passes. This covers non-`v` tag prefixes, release data round trips,
shared repository wiring, MCP proposal persistence, and the CLI config-help changes. In
particular, `internal/container/release_run_backend_test.go` verifies the bridge and release
services share one configured store, `internal/domain/release/adapters/run_round_trip_test.go`
checks commits, HEAD, base ref, and changeset persistence, and `internal/mcp/cgp_tools_wiring_test.go`
checks proposals are written durably. These tests show the old Roady descriptions are stale for
those fixes, though the full original release-repository acceptance (migrating all old file
shapes and proving every approve/publish path) still needs a dedicated acceptance definition.

## POLICY CONDITIONS ROADY ENTRY RECONCILED (2026-11-26)

The Roady description was stale: it said `actor.trusted`, empty-list operators, and breadth
fields were missing, although the current evaluator exposes trust and conditional reputation,
`is_empty`/`size`, path-derived area/directory breadth, and detected package breadth. Updated the
feature to state the actual capability and split it into checkable requirements. The executable
acceptance check passed via `roady task check` for evaluator and CLI policy regression tests; the
corresponding task was started, completed, and verified with that evidence. Holiday calendars
and first-class actor seniority remain deferred: shipped policies use freeze periods for
holiday windows and configurable roles for team-defined levels, and no current discovery evidence
justifies adding a separate calendar or seniority source. That deferral remains a manual
acceptance item pending customer discovery. Expanded `docs/governance.md` with area/directory
fields and collection operators. No new policy behavior was needed. `roady spec validate`,
`roady drift detect`, focused tests, and `git diff --check` pass.

## FIXED AND VERIFIED: reproducible local Nox security gate (2026-11-26)

`make security-scan` previously ran Nox with the user's ambient plugin registry. In this
workspace that caused a degraded failure because 14 unrelated plugins happened to be installed
but were not part of this repository's declared scan. The target now uses a fresh temporary
`NOX_HOME`, installs only the two plugins declared in `.nox.yaml`, and fails on degraded checks,
matching the CI gate without depending on machine-specific plugin state. The temporary registry
is removed when the target exits.

`make security-scan` passes with pinned Nox 1.70.0 on this full local candidate: 796 dependencies
checked, zero unsuppressed findings at HIGH or CRITICAL, and no degraded checks. The first run
also demonstrated the ambient-plugin failure, confirming why isolation matters. The full local
`make check-ci` target passes: formatting, vet, lint, tests, coverage policy, and security scan;
`go test ./...` also passes across all packages, including PostgreSQL and integration packages.
`git diff --check` passes.

## CGP MCP TOOL WIRING VERIFIED LOCALLY (2026-11-26)

The remaining Roady entry for `cgp_propose`, `cgp_authorize`, and `cgp_status` was stale:
`relicta mcp serve` now supplies the evaluator built by the configured governance service and
opens the configured durable proposal store. The `relicta_init` reload path refreshes both,
preserving the same policy behavior and persistence after initialization. No bare evaluator is
substituted when governance is unavailable; the tools return an explicit availability error.

`go test ./internal/mcp ./internal/cli -run
'Test(CGP|TheMCP|MCPServerWires|Reload|EvaluatorCarries)'` passes, covering configured policy
parity, production wiring, reload, and durable protocol storage. Updated the Roady requirement
with this executable check. Hosted CI has not run for this local candidate.
The earlier entry above records an independent isolated scan of this candidate; hosted CI still
needs to validate the eventual pushed change.

## PUBLIC SITE IS STALE; PRODUCTION DEPLOYMENT LACKS A MERGE GATE (2026-11-26)

Read-only inspection of `https://relicta.tech` and `https://docs.relicta.tech/install` found the
public site still advertises unsupported exclusivity, four audience variants, "zero data leaves
your environment," immutable governance records, and unauthenticated direct macOS archive
installation. These claims conflict with the current v4.2.0 product and the corrected local
`relicta-site` source. The corrected marketing build passes `npm run check`, `npm run build`, and
`scripts/verify-build.mjs src dist`; the docs build and `scripts/verify-build.mjs docs/src docs/dist`
also pass. The live site remains unchanged because no deployment was run.

The `relicta-site` GitHub repository is public and its deploy workflow runs for pushes to `main`.
The connected GitHub API initially reported no branch protection or repository rulesets. I added
and re-read active ruleset `Protect production site deployments` (ID 28140665) for `main`: direct
pushes, deletion and force-pushes are blocked; merges require a pull request and the
`Build Marketing Site` and `Build Documentation Site` checks. It requires no reviewer approval to
preserve a workable solo-maintainer path, and has no bypass actors. The local CI workflow changes
must still reach a PR and confirm these exact contexts on a hosted run. No site deployment occurred.

The core `relicta` repository also lacked branch protection. Added and re-read active ruleset
`Protect production release branch` (ID 28140753), requiring a PR and ten core CI/security status
checks, with strict up-to-date checks, no reviewer count and no bypass actors. PR #405 is open,
clean, and currently green for every required context, but those checks still cover only its
committed dependency-update head, not this 855-path local candidate. Neither ruleset merges or
publishes changes.

## ACTION INSTALLER TESTED LOCALLY; HOSTED PR STILL NEEDS THE UPDATE (2026-11-26)

Committed Action installer hardening and regression coverage locally as `630a66d` on
`fix/fail-closed-checksum-and-approval-default`. The `latest` alias is resolved to a stable exact
release tag before downloading, and the checksum signature is constrained to that same tag. The
installer no longer trusts a persistent tool cache, and the bundle build rejects source maps. Four
focused tests cover latest-tag binding, prerelease rejection, checksum mismatch, and duplicate
checksum entries. The GitHub Actions matrix now runs those tests on Node 24 for Ubuntu, macOS, and
Windows; its release workflow also checks that the committed bundle matches the source.

Local evidence passes: Prettier, ESLint, all four Jest tests, bundle generation, `npm audit` (zero
findings), and `git diff --check`. The branch is clean after the local commit. No push occurred.
GitHub PR #7 remains open and draft at older head `9db954cd54de8d98ca10e967e5aecac7adbb7932`,
with its older Node 20 checks; it does not include `630a66d`. The cross-platform Node 24 run and the
updated release workflow therefore still need hosted validation before the Action can be considered
production ready.

## CONFIGURED VERSION TAG PREFIXES VERIFIED LOCALLY (2026-11-26)

The Roady description for non-`v` tag prefixes still described an unimplemented defect even
though the candidate already had prefix-aware semantic-version parsing and latest-tag lookup,
including monorepo tags and stale-plan detection. Reconciled the description with the shipped
behavior and added an executable acceptance check across sourcecontrol, release adapters, Git,
monorepo, and CLI packages.

The prefix acceptance suite passes for `release-`, `rel/`, `app-v`, and the default `v` prefix,
and verifies a different component's prefix does not make a plan stale. Roady's acceptance task
and feature task are verified. This is local evidence only; hosted CI has not validated the full
candidate.

## INIT CONFIG GUIDANCE VERIFIED LOCALLY (2026-11-26)

The `relicta init` roadmap item still treated missing config sections as an unresolved design
decision, while the implementation had converged on a concise generated file plus complete YAML
hints for actionable errors. Updated the Roady spec to describe that behavior and added an
executable acceptance check. The focused CLI/config suite passes, covering the generated config,
the governance, environment, dashboard-auth, and repository-group hints, and detection of
actionable messages that only name `.relicta.yaml`. Roady marks both the acceptance requirement
and feature task verified. Hosted CI has not run for the local candidate.

## CGP PROTOCOL HISTORY SURVIVES RESTARTS LOCALLY

Reconciled the Roady item that still claimed MCP protocol records existed only in memory. The
production MCP command opens the configured file, SQLite, or PostgreSQL protocol store, and the
protocol suite verifies proposals plus decision/authorization round trips through separate store
instances. Targeted tests across protocol, persistence backends, and MCP wiring pass; both the
acceptance task and the feature task are verified locally.

Protocol payloads remain separate from the release-run aggregate because a CGP proposal is not
inherently a release. Production MCP now also appends protocol proposal, decision, and
authorization events to the shared hash-chained audit store selected by `persistence.backend`,
when governance memory is enabled. The focused protocol, MCP, and CLI suites pass; the new test
reconstructs and verifies the chain. This is audit evidence of CGP actions, not evidence that a
release was approved through `relicta approve` or published. Hosted CI has not run for the full
candidate.

## CURRENT CORE CI GATE PASSES LOCALLY (2026-11-26)

Re-ran `make check-ci` against the current worktree after the latest compatibility and roadmap
updates. The first run caught a `staticcheck` De Morgan diagnostic in legacy commit-hash
recognition; simplified the character check using `strings.ContainsRune` and reran the gate.
Formatting, `go vet`, `golangci-lint`, the full Go test suite (including PostgreSQL test
containers), all 27 configured coverage domains, and the isolated Nox security scan now pass.
`git diff --check` passes as well. This is current local evidence only; the 856-path candidate
remains uncommitted and has not been validated by hosted CI or released.

## VERIFIED LOCALLY: agent-authored MCP notes avoid duplicate changelog output (2026-11-26)

When a coding agent submitted customer-facing copy through `relicta_notes`, the response repeated
the same prose in both `notes` and `changelog`, incorrectly suggesting a separate technical
changelog had been generated. Agent-authored drafts now return the exact text once in `notes`, with
their provenance warning; the MCP reference reflects that response. Focused agent-draft, exact-hash,
and whitespace-draft tests pass, as does `git diff --check`. This is local evidence only; hosted CI
has not validated the full candidate.

## RECHECKED: corrected site builds; live marketing and install pages remain stale (2026-11-26)

Re-read the public release page, homepage, and install guide. GitHub still lists v4.2.0 as latest;
the live homepage continues to claim Relicta is the only MCP release tool, promises four customer
narratives, and says zero data leaves the environment, while the install page still presents a
direct macOS archive download without the current source's verification safeguards. The corrected
site source is on local branch `codex/site-production-readiness` at `e322ecf`, not on the remote.
Fresh marketing and docs `astro check`, production builds, and both `scripts/verify-build.mjs`
checks pass; the site checkout remains clean. This proves the local artifacts are buildable, not
that they are live. No push or deployment occurred.

## RELEASE APPROVAL SETUP RECHECKED (2026-11-26)

The hosted release workflow passes `actionlint`, but the GitHub `release-approval` environment
still returns 404. The repository collaborator list contains only the project owner, and the
organization has no reviewer team available. The active `main` ruleset requires a pull request and
ten status checks but requires zero approving reviews. The release workflow itself requires a
separate environment reviewer with self-review disabled, so a tagged release cannot currently
complete. The security setup guide already documents the missing environment. The owner must
choose between adding a trusted independent reviewer and changing the release policy to permit
maintainer self-review before this gate can be configured. No GitHub settings were changed.

## CURRENT CANDIDATE FULL CI RECHECK (2026-11-26)

Re-ran `make check-ci` after the MCP agent-note warning and duplicate-changelog fix. Formatting,
`go vet`, `golangci-lint`, the full Go test suite, all 27 configured coverage domains, and the Nox
security scan passed. This confirms local candidate quality gates, not hosted execution: the
856-path worktree is still uncommitted and GitHub PR #405 remains at its older head.

## NON-MACOS RELEASE SNAPSHOT VALIDATED LOCALLY (2026-11-26)

Ran the pinned GoReleaser v2.20.3 workflow configuration for Linux amd64/arm64 and Windows
amd64 on macOS arm64 using a non-publishing snapshot. The release completed, and
`scripts/validate-release-snapshot.sh dist` verified all three embedded dashboards, three SPDX
SBOMs, six checksum entries, and the custom Homebrew formula against the release checksums. This
validates the local packaging path only; hosted release approval and publication remain blocked
by the missing reviewer environment described above.

## CURRENT READINESS CANDIDATE COMMITTED LOCALLY (2026-11-26)

Committed the reviewed local candidate as `1d7e470` on `codex/production-readiness` (858 files,
25,748 insertions, 7,067 deletions). The worktree is clean; `roady spec validate` and
`roady drift detect` pass. The commit includes the prior local full-CI and release-snapshot
evidence, but has not been pushed, so no hosted checks have run for this candidate. Open PR #405
still points at `72d31c6`; its existing green checks are from October and do not validate this
candidate. The separately corrected site remains local on `codex/site-production-readiness`, now
including commit `4e988fc`; no site PR or deployment exists.
