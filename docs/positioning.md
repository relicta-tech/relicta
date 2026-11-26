# Relicta product positioning

## Short description

Relicta turns repository changes into reviewed releases, customer-facing notes, and an auditable approval record.

## Who it is for

Relicta is for engineering teams that release software from Git repositories and want a consistent connection between the changes, release communication, policy decision, approval, and published version. It is especially useful when release owners currently assemble this packet by hand or need to explain later why a change was allowed to ship.

## The job it does

For each release, Relicta analyzes repository changes, proposes a version and structured summary, evaluates configured governance policy, records the decision, and carries reviewed notes through approval and publication. The hash-chained audit trail connects governance decisions; `relicta audit verify` checks the chain. Actor types (`human`, `ci`, `agent`, and `system`) let policy distinguish who or what initiated a decision.

The practical value is a repeatable release process and less manual work assembling release notes and approval evidence. Relicta does not prove that the change is correct or safe, and a recorded decision is only as useful as the policy, inputs, and review behind it.

## Customer-facing release notes

Deterministic notes summarize repository history. Configured AI providers can produce prose. In the unreleased v5 source workflow, an MCP-connected coding agent can use product context from its current conversation to draft customer-facing notes from Relicta's version and change recommendation; Relicta stores the supplied text as agent-authored in the regular approval flow. A human remains responsible for checking claims and approving the release. Relicta itself does not fact-check agent-authored text.

This workflow addresses a specific release-owner problem: technical commits often need translation into language customers can understand, and the relevant product context may already be in the agent session. Treat this as a v5 source capability until it is included in a published release.

## Scope and alternatives

| Tool category | Main job | Relicta's relationship |
| --- | --- | --- |
| GRC and AI governance platforms | Inventory, posture, and organization-wide control evidence | Relicta focuses on release-time decisions and can export supported evidence to platforms such as Vanta and Drata. It does not replace their broader governance programs. |
| Agent runtime governance | Observe or constrain agent activity while agents execute | Relicta governs software release decisions; it does not mediate every action an agent takes at runtime. |
| Repository branch protections | Require checks or reviewers before a branch can merge | These provide source-control gates. Relicta records a structured release decision and related evidence that can span the release workflow. |
| Release scripts and changelog generators | Automate versioning, packaging, or change summaries | Relicta combines release preparation with configurable governance, approval records, and publication integrations. |

These categories overlap in some workflows. This table describes their typical focus, not a feature-by-feature evaluation of every product or plan.

## Competitive reality and market hypothesis (checked 2026-11-26)

Relicta should not be positioned as the only tool governing AI-agent releases. Digital.ai announced
Release 26.3 on 2026-11-18 with agent orchestration, an MCP server for coding tools, policies, and
an audit trail across enterprise release workflows. That is direct overlap with the broad
"govern agent releases" story, although Digital.ai describes a larger enterprise orchestration
platform. See [Digital.ai Release 26.3](https://digital.ai/press-releases/digital-ai-release-26-3-brings-ai-agents-into-governed-enterprise-delivery/).

The lower end of the workflow is already covered too: GitHub can generate release notes from pull
requests ([GitHub release documentation](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository)), while
[Release Please](https://github.com/googleapis/release-please) automates changelog updates,
version bumps, release PRs, and GitHub releases from Conventional Commits. A generic promise to
"automate releases" or "write release notes with AI" is not a defensible wedge.

ReleasePad is a direct competitor to the agent-session note-writing use case, not just a generic AI
writer: its September 2026 changelog describes an MCP server that drafts, edits, and publishes
customer notes from coding assistants, alongside reusable brand-voice instructions. Its November
2026 updates further advertise a rebuilt AI writer and in-place post editing. Using the current
agent session, generating polished customer prose, and editing/publishing that prose are therefore
workflow parity, not defensible differentiators. See
[ReleasePad's MCP, brand-voice, and recent product changelog](https://pro.releasepad.io/en/releasepad?page=1).

Price strengthens the free-or-low-cost baseline. GitHub supports manually written or automatically
generated release notes, and Release Please is an Apache-2.0 project that automates version bumps,
changelogs, and GitHub releases. ReleasePad offers a free tier with 20 posts and 20 AI requests;
its Pro plan is $35 per month per product and includes MCP, hosted changelog publishing, analytics,
and unlimited team members. So agent-written notes alone are unlikely to justify a new paid tool.
The buying hypothesis to test is whether teams will adopt and pay for Relicta's repository-level
policy, approval, and audit linkage, or specifically value self-hosting enough to prefer it over
those existing options. Do not make a price or savings claim before comparing actual workflows and
asking about a real budget. Sources: [GitHub release notes](https://docs.github.com/en/repositories/releasing-projects-on-github/about-releases),
[Release Please](https://github.com/googleapis/release-please), and
[ReleasePad pricing](https://www.releasepad.io/pricing/) (checked 2026-11-26).

The sharper product hypothesis is: **for small and mid-sized software teams that ship from Git and
need a lightweight release control, Relicta connects the exact version and change recommendation
to customer notes, an explicit policy decision, human review, and the published outcome in a
self-hostable repository workflow—without adopting an enterprise release orchestration suite.**
Its potential advantage over a dedicated changelog tool is that the saved note is part of the same
reviewed release record as the version and governance decision. Whether release owners value that
link enough to adopt Relicta, rather than keep separate release and changelog tools, is the core
market question. This integrated workflow remains unreleased v5 source; do not market it as
available until shipped and proven with users.

This is a positioning hypothesis, not validated demand. Local MCP dogfooding in the Govee
Light Management working repository saved an agent-authored customer note to a planned patch
release record and confirmed the note was still awaiting approval. The draft was limited to seven
committed changes; existing uncommitted dial-feedback work was excluded. Time saved was not
measured. This provides maintainer workflow evidence only; there are no external customer
interviews, conversion data, retention data, or willingness-to-pay evidence. Market validation
should test whether release owners value the linked decision and approval record enough to adopt
another tool, and whether
agent-session notes materially reduce their editing time versus GitHub's generated notes and
existing AI assistants. A read-only check of the public [issue history](https://github.com/relicta-tech/relicta/issues)
on 2026-11-26 found 42 issues, all opened by the project owner, with no independently opened
customer requests. The public repository showed 0 stars and 0 forks; v4.2.0's five platform
archives totaled 54 downloads. These are weak and ambiguous adoption signals: release asset counts
are not unique users, and the public data omits private use and other install channels. They do not
establish that no market exists, but there is not yet visible evidence of organic pull. Prioritize
external release-owner interviews before investing in broad dashboard or multi-repository expansion.

Local dry-run planning across 17 configured repositories exposed a safety gap for stale or
divergent branches: a plan could use the newest reachable tag even when a higher semantic version
tag existed elsewhere in the repository. The current local candidate now names that higher tag
and the reachable baseline in CLI, JSON, and MCP plan output. When this warning is present, saving
a live plan requires explicit acknowledgment; the acknowledgment and warning are recorded in the
plan history. Separately,
`workflow.require_up_to_date: true` fetches the configured tracking remote and blocks publication
when the branch is behind. Dry-run results remain operator-reviewed recommendations, and neither
warning nor acknowledgment makes a stale checkout safe to publish from. These safeguards have
been exercised locally, but the candidate has not received fresh hosted checks or an external
release-owner review.

### Release-note workflow evidence (checked 2026-11-26)

ReleasePad's product claims include GitHub-based drafting, assistant/MCP access, brand-voice
instructions, draft review, hosted changelog publishing, and analytics. Its MCP supports editing
and publishing as well as drafting, and its November 2026 changelog reports a rebuilt AI writer.
GitHub itself generates release notes from merged pull requests, contributors, and a full
changelog, with label-based categories and exclusions. Sources: [ReleasePad product changelog](https://pro.releasepad.io/en/releasepad?page=1), [GitHub automatically generated release notes](https://docs.github.com/en/repositories/releasing-projects-on-github/automatically-generated-release-notes).

The differentiating claim Relicta can test is therefore the release-record linkage: customer
wording is saved with the plan facts, version, policy decision, and human approval that govern the
release. Grounding, filtering engineering-only changes, and review are necessary quality criteria,
but are not unique by themselves. See ReleasePad's
[release-notes product page](https://www.releasepad.io/solutions/release-notes-tool/) and
[September 2026 MCP changelog](https://pro.releasepad.io/en/releasepad?page=1).

Public discussion offers weak, anecdotal support for the problem: one SaaS founder described release
notes as recurring homework after an eight-month stale changelog, while replies raised filtering
engineering noise, factual completeness, and human approval as the harder parts ([discussion](https://www.reddit.com/r/SaaS/comments/1vjdtx4/i_built_a_changelog-that-writes-itself-from-my/)).
This is self-selected founder discussion, not independent customer validation. A separate question
about how SaaS teams write notes received mixed answers, including ticket-based processes and
existing tools ([discussion](https://www.reddit.com/r/SaaS/comments/1qvkf5h/how_do_you_write_your_saas_release_notes/)).
The practical product test is not just whether an agent can draft prose: measure whether verified
source facts reduce correction work and whether release owners value the governed record enough to
change their existing workflow.

## Compliance evidence limits

Relicta can export records derived from release governance data, including Article 12 supporting material and an Annex IV supporting-evidence draft. These exports can be incomplete, contain placeholders, and omit information required for a particular system or organization. They are not automatically generated records of every AI system use, complete technical documentation, a conformity assessment, or proof of compliance. Review them against applicable requirements with qualified legal and control owners.

## Product claims to keep accurate

- Say “release governance” or “release-time governance,” not general AI governance.
- Describe the audit trail as hash-chained and verifiable; do not imply that every report generation verifies chain integrity.
- Describe compliance exports as supporting evidence or drafts; do not call Relicta “Annex IV ready” or claim regulatory compliance.
- Mark agent-session customer notes as unreleased v5 source until a public release includes them.
- Treat generated customer copy as a draft requiring human review.
- Treat agent-session context as framing only: it may explain why a verified change matters, but it
  cannot add claims about what shipped; omit unclear impact or flag it for human review.

## Evidence and references

- [README](../README.md) for current capabilities and released-versus-unreleased status.
- [MCP release-notes workflow](mcp.md#draft-customer-facing-release-notes-in-your-agent-session) for the agent-session flow.
- [CGP guide](governance.md) for policies, risk scoring, and approval workflows.
- [Deployment evidence](deployment-evidence.md) for recording environment deployments and governance gates.
- [Customer discovery guide](customer-discovery.md) for testing the market hypotheses with release owners.
- [Digital.ai Release 26.3 announcement](https://digital.ai/press-releases/digital-ai-release-26-3-brings-ai-agents-into-governed-enterprise-delivery/) for a current direct category competitor.
- [GitHub release documentation](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository) and [Release Please](https://github.com/googleapis/release-please) for established release-note and release-automation alternatives.
