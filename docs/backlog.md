
## Consolidate the two file-based release repositories

There are two independent file-based implementations of the ReleaseRun aggregate, with incompatible on-disk schemas, and commands disagree about which to use.

WHAT THEY ARE

1. internal/domain/release/adapters/FileReleaseRunRepository — wired by internal/domain/release/factory.go:60 and used by the release services (plan, bump, notes, approve, publish) and by `relicta status`. Writes the changeset at the top level of the run JSON and restores it on load.

2. internal/infrastructure/persistence/FileReleaseRepository — wired at internal/container/container.go:169, returned by app.ReleaseRepository(), and used by the governance paths. Expects the changeset nested under plan.changeset, and reconstructs runs lossily.

THE CONSEQUENCE, OBSERVED

`relicta evaluate` failed on every release, in every repository, with:

  governance evaluation failed: failed to evaluate proposal: invalid proposal:
  invalid scope: either commitRange or commits is required

`plan` wrote the run with (1); `evaluate` read it with (2); (2) found no changeset because it looks in a different place, so governance had no commit range and refused the proposal. A core command — the one that computes the risk score and policy verdict the product is built around — could not succeed at all.

WHAT (2) LOSES ON LOAD (internal/infrastructure/persistence/release_repository.go:739-741)

  BaseRef  <- dto.Branch      (the branch, not the base ref)
  HeadSHA  <- ""              (empty)
  Commits  <- nil             (dropped)
  ChangeSet                   (not found: different schema location)

So a run loaded through (2) cannot support anything that needs commits, HEAD, or the base ref — which is most of governance.

IMMEDIATE MITIGATION ALREADY LANDED

getLatestReleaseForEvaluate now loads through the release services' repository, the same one status reads and plan wrote. That makes evaluate work without touching the duplication. It is a redirect, not a fix: app.ReleaseRepository() still returns the lossy implementation and other callers still use it.

WHY THIS IS NOT A SMALL CHANGE

- app.ReleaseRepository() has other callers that would need auditing for the same class of bug
- the two schemas differ, so consolidating means deciding whether to migrate existing .relicta/releases/*.json or read both shapes during a transition
- domainrelease.Repository and ports.ReleaseRunRepository are different interfaces over the same concept, so consolidation is an interface decision as well as an implementation one
- publish and approve both take governance paths that would change behaviour

SUGGESTED APPROACH

1. Inventory every caller of app.ReleaseRepository() and record which are affected by the lossy load
2. Decide the single owning implementation — (1) is the better candidate: it is the writer, it round-trips the changeset, and the release services already depend on it
3. Give the surviving type the union of the two interfaces, or narrow callers to the interface they actually need
4. Handle existing on-disk runs: either read both schema shapes for a release or two, or migrate on load
5. Delete the loser, and add a test that a run written by the release services round-trips with commits, HEAD SHA, base ref and changeset intact — the absence of that test is why this survived

ACCEPTANCE

One implementation. A round-trip test proving commits, head_sha, base_ref and changeset all survive save/load. evaluate, approve and publish all read runs through it.

---

## Make versioning.tag_prefix work for non-"v" prefixes

Tracked on GitHub as relicta-tech/relicta#231; logged here so it stays in the backlog.

versioning.tag_prefix is configurable, TagList.FilterByPrefix exists, and monorepo docs describe app-v1.2.3 style tags. But a tag with any prefix other than "v" is never recognised as a version tag, so the setting has no effect beyond its default.

CAUSE

sourcecontrol.NewTag (internal/domain/sourcecontrol/tag.go:23) decides version-ness by parsing the whole tag name, and version.Parse accepts only bare semver or a leading "v":

  Parse("1.5.0")         -> 1.5.0
  Parse("v1.5.0")        -> 1.5.0
  Parse("release-1.5.0") -> invalid semantic version
  Parse("rel/1.5.0")     -> invalid semantic version
  Parse("app-v1.5.0")    -> invalid semantic version

Such tags are dropped by TagList.VersionTags() BEFORE FilterByPrefix can be applied — the usual call order is FilterByPrefix(prefix).VersionTags(), and the second step discards what the first selected.

CONSEQUENCES for any project not using "v"

- plan cannot find the previous version tag, so it computes a baseline of "no previous release" and a changeset spanning the whole history
- status cannot detect that a release happened since a run was planned
- monorepo-style prefixes do not work at all, though app-v1.2.3 is the documented pattern for blast and workspace versioning

The failure is silent: nothing reports that the prefix was ignored, so the project simply appears to have no releases.

RELATED

internal/domain/release/app/plan.go:131 separately hardcodes the prefix:

  tag, err := uc.repoInspector.GetLatestVersionTag(ctx, "v")

even though cfg.Versioning.TagPrefix is threaded into the CLI's AnalyzeInput elsewhere. Both need fixing; that line alone would still hit the parse limitation.

WHY IT NEEDS A DECISION FIRST

The right shape depends on what a monorepo tag is meant to be: "prefix app-v plus semver", or a component-qualified tag with its own type carrying component and version separately. blast and workspace versioning both depend on the answer, so deciding that comes before implementing.

SUGGESTED APPROACH

1. Decide the monorepo tag model (plain prefix vs component-qualified type)
2. Teach the tag domain to strip a configured prefix before parsing, or add a prefix-aware constructor
3. Pass cfg.Versioning.TagPrefix in plan.go instead of "v"
4. Revisit detectRunStaleness in internal/cli/status.go, which currently cannot see non-"v" releases

EXISTING TEST

TestDetectRunStaleness_NonVTagPrefixesAreNotDetected (internal/cli/status_staleness_test.go) pins the current behavior and FAILS once this is fixed, as a prompt to revisit the staleness detector at the same time.

---

## Decide what relicta init should write, and make error messages match

`relicta init` writes 6 of the schema's 21 top-level config sections, so error messages that tell a user to edit a setting frequently name a key the generated file does not contain.

WRITTEN: versioning, changelog, ai, plugins, workflow, output

MISSING: attestation, channels, chronos, communication, dashboard, git, governance, mnemos, monorepo, observability, persistence, plugin_security, repository_groups, telemetry, webhooks

MESSAGES THAT POINT AT ABSENT KEYS

  internal/cli/evaluate.go   "enable governance in .relicta.yaml"        (governance absent)
  internal/cli/multirepo.go  "defined in .relicta.yaml under 'repository_groups'"  (absent)
  internal/cli/serve.go      "configure api_keys in .relicta.yaml"       (dashboard absent)

A user follows the instruction, opens the file, and the section is not there — with no indication of the nesting to add.

MITIGATION ALREADY LANDED

The governance case now returns errGovernanceDisabled (internal/cli/governance_disabled.go), which prints the YAML to add rather than naming a key:

  Add this to .relicta.yaml:

    governance:
      enabled: true

Verified end to end: adding exactly that makes `relicta evaluate` work. The multirepo and dashboard messages have not been given the same treatment.

THE DECISION THIS NEEDS

Writing all 21 sections would produce a file of several hundred lines, most of it advanced settings at their defaults, which is its own usability problem — the current 99-line output is readable and the annotations for the dangerous settings stand out. Three plausible answers:

1. Keep a curated subset, and require every "configure X in .relicta.yaml" message to carry its own YAML snippet. Cheapest, keeps the file readable, puts the burden on error messages.
2. Write all sections, with comments, and accept the length. Most discoverable, worst to read.
3. Write the curated subset plus a commented-out block for each remaining section, so the shape is discoverable without being active. Middle ground; the annotation mechanism in internal/config/loader.go could be extended to emit these.

Whichever is chosen, the invariant worth enforcing with a test is: no user-facing message names a config key unless that key (or a commented example of it) appears in what init writes.

SUGGESTED APPROACH

1. Pick 1, 2 or 3
2. If 1: audit every message mentioning .relicta.yaml and give it a snippet, following errGovernanceDisabled
3. Add a test that scans CLI strings for ".relicta.yaml" references and checks the named section appears in WriteDefaultConfig output
4. Consider whether governance in particular deserves to be written regardless, since it gates the risk scoring the product is built around and is the section users most need to find

---

## Wire or withdraw the cgp_* MCP tools

The three CGP protocol tools are advertised in tools/list and cannot work. An agent enumerates them, calls one, and gets a failure on every attempt.

  cgp_propose
  cgp_authorize
  cgp_status

CAUSE

handleCGP* calls ensureCGPService (internal/mcp/server.go:708), which needs either s.cgpService or s.evaluator. `relicta mcp serve` wires neither: WithCGPService and WithEvaluator are both defined and called only from tests. So ensureCGPService always takes its error path.

Found by sweeping for option constructors that are never called in production — the same pattern as WithAdapterRepo, WithSkipPush and FindByPlanHash. Of 87 With* constructors in the tree, 40 are never called outside tests; most of those are legitimate test seams, these are not.

MITIGATION ALREADY LANDED

The failure now returns a ToolInputError explaining that no evaluator is configured and pointing at relicta_evaluate, instead of the redacted "internal error" an agent used to receive. The tools still do not work.

THE DECISION THIS NEEDS

Supplying `evaluator.New()` in createMCPServer would make the tools respond, and that is what the tests do. But a bare evaluator is not the same as the CLI's governance service: the container builds that from cfg.Governance — thresholds, policies, freeze periods, trusted actors, reputation. An agent evaluating a proposal through cgp_* would get a different verdict than a human running `relicta evaluate` on the same change, which for a governance tool is worse than the tool being absent.

Three options:

1. Expose the container's configured evaluator (or the governance service's) and wire that. Correct, and makes agent and CLI agree. Needs an accessor the container does not currently have, and a decision about whether the CGP protocol path should share governance's configuration or have its own.

2. Withdraw the three tools from tools/list until they can be wired. Honest, and stops agents wasting calls. Loses the CGP protocol surface, which pkg/cgp exists to expose.

3. Keep them advertised but make the unconfigured response a documented, structured "not available here" that an agent can branch on. Cheapest; currently implemented as an isError, which is close.

Option 1 is the real answer if the CGP protocol surface is meant to be part of the product. Option 2 is right if it is not.

ACCEPTANCE

Either the three tools return real results computed with the same configuration the CLI uses, or they are not advertised. A test should assert whichever is chosen, since the current state — advertised and non-functional — is what nothing was checking.

RELATED

Other Server options never wired by `relicta mcp serve`: WithReleaseRepository, WithPolicyEngine, WithActorBudgets, WithRiskCalculator, WithCache, WithCacheDisabled. WithRiskCalculator in particular gates the fallback risk path in handleEvaluate, which therefore cannot run either. Each needs the same question asked: wire it, or remove the option and the code that depends on it.

---

## Policy conditions for data the evaluator does not yet carry

Five conditions written in the shipped example policies had no field behind them and were removed rather than left looking active (PR #249). Each is a real governance need, and each needs the evaluator to carry data it does not have today.

CAUSE: buildEvalContext exposes risk, change counts, blast radius, scope, intent, actor identity, team configuration and a clock. It carries nothing about holidays, actor seniority, how many domains a change touches, or an actor's earned trust. `relicta policy fields` now prints exactly what exists, so the gaps are visible instead of guessed at.

CONSEQUENCE: rules a team would reasonably want cannot be expressed. Specifically:

1. `time.is_holiday` — no holiday calendar. Workaround today: configure holidays as freeze periods, which `time.freeze.active` covers. A real calendar would need per-region holiday sets and config to select them.
2. `actor.level` / seniority — expressible only as a configured role (`actor.roles contains "junior"`), which pushes the concept into team config. A first-class seniority field would need a source of truth for it.
3. `change.scope_count` — no count of distinct domains or packages touched. Deliberately NOT approximated with scope.fileCount: a large single-domain change would fire the rule and a small cross-cutting one would not, which is backwards. Needs domain/package attribution over the changed file set — the monorepo package detection may already supply most of it.
4. ~~`actor.trusted`~~ — DONE. buildEvalContext now exposes `actor.trusted` and `actor.trustLevel` from proposal.Actor.TrustLevel (so it reflects whichever source granted the trust: the trusted_actors allowlist, an identity-registry grant, or earned trust), plus `actor.reputation.overall` / `.level` / `.samples` / `.trend` from the reputation the governance service computes. The clause is restored in agent-aware.policy. Two things worth carrying forward: `actor.reputation.*` is absent rather than zero when reputation is not being computed, because a zero score means a bad record and would have made every such rule fire; and `relicta policy test --trust-level` exists so a trust-conditioned rule can be exercised at all.
5. Membership-of-nothing — `NOT actor.is_member` was removed because there is no operator for "this list is empty". `actor.teams` exists; an `is_empty` / `size` operator would express it, and would apply to `scope.files` and `actor.roles` too.

OPTIONS: (5) next, which is an operator rather than new data. (3) after that if monorepo attribution can be reused. (1) and (2) are configuration surfaces and can wait for someone to ask.

STILL OPEN from (4): the shipped `trusted-human` rule takes no actions, so with agent-aware's `decision = "approve"` defaults, matching it changes the rationale but not the verdict. The rule that would pay for itself is the agent equivalent — a trusted agent skipping the human approval `agent-requires-human` demands — but that loosens a shipped template's governance, so it wants a deliberate decision rather than being folded into exposing the field.

Whatever is added must go into buildEvalContext, so KnownFieldPaths picks it up automatically and `policy validate` stops reporting it — the enumeration is derived, not hand-listed, specifically so this stays in one place.

---

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
