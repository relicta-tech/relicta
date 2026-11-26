# Relicta customer discovery guide

**Purpose:** Test whether release owners have a recurring problem connecting technical repository changes to customer communication and governed release decisions, and whether Relicta solves enough of it to justify adoption.

**Status:** Research instrument only. No external customer evidence has been collected in this repository yet. Local dogfooding now covers one agent-authored release note and dry-run planning in 17 configured repositories; this is product usability evidence, not independent market validation.

## Hypotheses to test

1. Release owners regularly spend meaningful time turning technical changes into customer-facing notes.
2. Existing release-note generators or AI assistants produce output that still requires substantial context gathering, editing, or review.
3. Teams need a reviewable record that ties release notes and approval to the version and changes being shipped.
4. Coding-agent product context can reduce note-writing effort without reducing factual accuracy or human oversight.
5. The problem is frequent and costly enough that a team will adopt a repository-level tool and maintain its workflow/configuration.

Do not present these as facts. Look for examples that disprove them.

## Who to interview

Start with 8–12 people who personally prepare, review, or approve releases. Recruit across:

- Open-source or small product maintainers who publish from GitHub.
- SaaS engineering or product teams that publish customer release notes.
- Teams already using coding agents in repositories and CI.

Record the participant's role, team size band, release frequency, repository/hosting setup, and agent use. Avoid treating one organization or one interviewee as evidence for an entire segment.

### Recruitment invitation

> I'm researching how software teams prepare and review release notes. Would you be open to a
> 30-minute conversation about a recent release and the tools you use? I'm looking for honest
> examples of what works and what is frustrating; this is research, not a sales call. You don't
> need to share code, customer information, or anything confidential. With your permission, I may
> take anonymized notes. I won't record the conversation unless you explicitly agree.

Send this only to people who are relevant to release preparation, review, or approval. Do not
describe a participant as a customer or represent their comments as endorsement without separate
permission.

## 30-minute interview

### 1. Recent release (8 minutes)

- Think about the most recent release you helped ship. Walk me through it from deciding what to release to communicating it.
- Who prepared the release notes? What information did they use?
- What was the hardest or most time-consuming part?
- How long did it take, and who else had to review or correct it?
- Can you show me a sanitized example of the source changes and the final notes?

Ask for a concrete recent example before discussing Relicta. Do not suggest that note writing is painful.

### 2. Existing workaround and impact (7 minutes)

- What do you use today for versioning, changelogs, approvals, and publication?
- Where do you copy information between tools or rewrite it by hand?
- What goes wrong when the notes or release decision are delayed or inaccurate?
- How often has that happened in the last three months?
- What have you already tried to improve it? What did you keep or abandon, and why?

Probe for observed time, rework, missed communications, incidents, or audit effort. Separate measured amounts from estimates.

### 3. Decision and constraints (5 minutes)

- Who would choose and configure a tool for this workflow? Who could block it?
- What security, hosting, data, or approval requirements would it need to meet?
- What would make you unwilling to connect a release tool to a repository or coding agent?
- What tools or existing processes would it need to coexist with?
- Have you paid for or built anything to solve this? What did that cost in money or staff time?

### 4. Concept test (7 minutes)

Only after the open questions, show a short Relicta walkthrough using a disposable repository and clearly label unreleased capabilities. Demonstrate:

1. A plan that reports the proposed version and structured change facts.
2. A customer note drafted using product context already in an agent session.
3. The exact saved draft attached to a release record for human review.
4. The configured approval and publishing boundary.

Then ask:

- Which part, if any, would change your current process?
- What would you stop doing or continue doing?
- What is missing for you to try it on a real repository?
- What would make this less useful than your current setup?
- Would you be willing to schedule a trial on one repository? What would need to happen first?

If the participant already uses GitHub's generated notes, an AI assistant, or a dedicated changelog
tool, use that as the comparison baseline rather than presenting Relicta in isolation. Ask them to
complete the same recent release-note task with their current workflow and with the Relicta draft
flow, using equivalent source facts. Record elapsed time, edits made for incorrect or unsupported
claims, engineering-only changes removed or missed, and whether the final wording can be traced to
the changes under review. Do not claim a win from a faster first draft if review effort or factual
corrections increase.

Do not present drafting inside the coding-agent session as unique: ReleasePad's MCP also supports
drafting, editing, and publishing from coding assistants, with reusable brand-voice instructions.
If a participant uses ReleasePad or a similar changelog service, compare that workflow directly.
Test whether Relicta's release record actually reduces work by linking the final wording to the
version, verified change recommendation, policy result, and approval. Measure whether that link
helps the participant answer who approved what shipped, or whether it adds configuration and review
steps they would rather keep in separate tools. Treat stronger grounding as a hypothesis: count
unsupported claims and missed user-visible changes in both workflows rather than assuming one is
more accurate.

Do not count compliments, hypothetical purchase intent, or feature requests as validation. A concrete follow-up action is stronger evidence.

### 5. Close (3 minutes)

- Who else owns this problem and would give a different perspective?
- May we follow up after you have tried a prototype with one real release?
- Is there anything about the workflow we misunderstood?

## Evidence capture

Get consent before recording or keeping identifiable notes. Store only information the participant agrees to share; redact repository names, customer data, and confidential release content.

For each interview, capture:

| Field | Record |
| --- | --- |
| Interview ID and date |  |
| Segment and participant role |  |
| Release cadence and agent use |  |
| Last real release example |  |
| Current tools and workaround |  |
| Time/rework evidence (measured or estimated) |  |
| Consequence of the problem |  |
| What they tried or paid for |  |
| Adoption constraints and blockers |  |
| Reaction to the walkthrough |  |
| Baseline tool and comparison task |  |
| Draft time, review time, and substantive corrections by workflow |  |
| Unsupported claims, missed user-visible changes, and internal noise by workflow |  |
| Can the notes be traced to the version, change facts, policy decision, and approver? |  |
| Evidence-work saved or added by the integrated release record |  |
| Concrete next step accepted |  |
| Evidence against the hypotheses |  |

## Decision rule

After 8–12 interviews, summarize evidence by segment and revisit each hypothesis. Treat demand as promising only when multiple participants independently describe a recent recurring problem, quantify a meaningful workaround or consequence, and agree to a concrete next step such as a repository trial. If people already solve it cheaply with GitHub notes or a small script, narrow the target or change the value proposition. If interest is limited to agent-generated prose without the governance record, test that as a separate product proposition before expanding the release platform.

This threshold guides the next research decision; it is not statistical proof of market size or willingness to pay. A paid pilot or explicit budget conversation is still needed before making pricing or commercial-demand claims.

## Current evidence boundary

On 2026-11-26, a local Relicta source build and MCP server were dogfooded in the Govee Light Management working repository. A deterministic dry-run plan covered seven committed changes from v3.1.2 to a v3.1.3 patch recommendation and explicitly excluded existing uncommitted dial-feedback work. The agent saved a customer note to the local release record; status confirmed `notes_ready` and `source=agent`, with approval still pending. The initial draft described dependency and security maintenance but also claimed light-control behavior was unchanged, which the recommendation did not establish. Replaced that claim with the narrower, supported statement that the update refreshes project dependencies and improves automated security checks. The release remains `notes_ready`; no version file, Git tag, push, approval, or publication was performed. This is one maintainer's workflow evidence only: time saved was not measured, and it does not establish external demand, adoption, or willingness to pay. The workflow also exposed a sequencing constraint: the local release record had to be advanced to versioned before notes could be saved, despite this repository maintaining version files through a separate release PR.

The same local candidate was then run in dry-run mode across all 17 discovered repositories with their own `.relicta.yaml`. It completed in all 17, including worktree and Nox-config cases that the installed v4.2.0 CLI could not handle. The scan found a release-safety edge case: three checked-out `main` branches were behind or divergent from newer version tags already present in their repositories. Because those tags do not descend from the local HEAD, plans correctly used the newest reachable tag as the commit-range base, but recommended lower versions (`mcp-go` 1.28.3 while v1.30.1 exists, `bolt` 1.7.1 while v3.1.3 exists, and `tokenops` 0.120.2 while v0.126.0 exists). The first scan did not flag the newer unreachable tags. The local candidate has since been updated to name the higher tag and reachable baseline in CLI, JSON, and MCP output. A saved plan now requires explicit acknowledgment, which is retained in its history; dry-runs remain inspectable. Publication can also enforce a fresh remote check with `workflow.require_up_to_date: true`, which fetches the tracking remote and refuses a branch that is behind. These safeguards were exercised locally, but stale or divergent plans still require operator judgment and the full candidate has not received fresh hosted checks. No repository source, tags, or remotes were changed by the original scan.
