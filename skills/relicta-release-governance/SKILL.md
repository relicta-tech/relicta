---
name: relicta-release-governance
description: Operate Relicta release workflows end-to-end with governance and safety checks. Use when working in repositories that use `relicta` and `.relicta.yaml` for version planning, bumping, notes generation, approval, publishing, plugin setup, MCP usage, policy checks, dashboard operations, and release troubleshooting.
---

# Relicta Release Governance

## Overview
Drive Relicta from intent to published release with predictable command order, explicit guardrails, and fast recovery for common failure modes.

## Quick Start
Start with read-only checks and a dry-run preview in the target repository:

```bash
relicta version
relicta health
relicta status
relicta plan --analyze --dry-run
relicta release --dry-run
```

Inspect the plan, proposed version, and repository diff before changing release state. The
`relicta bump`, `relicta approve`, `relicta publish`, and `relicta release` commands can change
files, create tags, push to remotes, or publish artifacts depending on configuration. Run them only
when the user explicitly asks to execute that release and has reviewed the exact version, notes, approval, and
publish destinations. Never infer publish authorization from a general request to plan or prepare a
release.

Use one-command mode for previews only:

```bash
relicta release --dry-run
```

Before a live release, confirm the approved version and notes, configured destinations, signing
and provenance checks, and whether pushing a tag will trigger external CI. If any of these are
unclear, stop at the dry-run and report what needs review.

## Workflow
1. Validate environment.
2. Establish or confirm config.
3. Plan and inspect risk.
4. Preview the version and notes.
5. Approve and publish only after explicit user authorization for the live release.
6. Verify outputs and recover if needed.

## Step 1: Validate Environment
Run:

```bash
relicta version
relicta health
git status --short
```

Expect:
- `relicta` available and healthy.
- Repository is correct.
- Working tree state is understood before release actions.

## Step 2: Confirm Configuration
Use `.relicta.yaml` as the canonical config file name.

If missing, explain the config creation choices and ask before initializing because `init` writes
to the target repository:

```bash
relicta init
```

Confirm critical sections:
- `versioning`
- `workflow`
- `plugins`
- `governance`
- `dashboard` (if serving UI)

## Step 3: Plan and Risk Review
Run:

```bash
relicta plan --analyze --dry-run
relicta status
```

Focus on:
- Current and next version.
- Commit count and release type.
- Governance/risk signals before bump/publish.

Policy simulation gates (before merge/release):

```bash
relicta policy validate
relicta policy test --matrix examples/policies/policy-matrix.yaml --json
relicta policy test --matrix examples/policies/policy-matrix.yaml --fail-on-blocked
relicta policy test --matrix examples/policies/policy-matrix.yaml --require-approved
```

Stdin mode is supported for automation:

```bash
cat examples/policies/policy-matrix.yaml | relicta policy test --matrix -
```

If no changes are detected, stop release flow.

## Step 4: Preview the Release Version
Run:

```bash
relicta release --dry-run
```

The release preview reports the proposed version and configured version-file changes without
writing them. It does not generate release notes or evaluate governance. Use `relicta plan
--analyze --dry-run` for commit facts. Run `relicta evaluate` only after a plan has been persisted.
After the user authorizes a live release, save the version, draft or generate notes, review them,
and complete the normal approval and publish steps. For agent-authored notes, use the MCP sequence
in `docs/mcp.md`: plan, bump, draft in the agent session, then call `relicta_notes` with `draft`.

Optional provider-generated notes (only in the authorized live release flow): enable a provider in
`.relicta.yaml` with `ai.enabled: true`, then run:

```bash
relicta notes
```

The standalone `bump --dry-run` previews version-file writes, but needs a planned release for the
version recommendation. `notes --dry-run` still needs a versioned release and does not generate a
persistable draft. Do not run these after a dry-run plan, which intentionally creates no release
state.

If bump type must be overridden:

```bash
relicta bump --level patch --dry-run
```

## Step 5: Approval and Publish (Explicit Authorization Required)

The following commands perform release actions. Do not run them for a planning, review, or
documentation request. First show the user the version, customer-facing notes, files to be changed,
approval requirements, and destinations. Proceed only after the user explicitly authorizes this
release.
Interactive/human-in-the-loop:

```bash
relicta approve
relicta publish
```

CI/non-interactive:

```bash
relicta release --yes --ci
```

If explicitly authorized to create artifacts locally without pushing:

```bash
relicta publish --skip-push
```

## Step 6: Verify and Recover
Run:

```bash
relicta status
relicta history
```

Recovery commands:

```bash
relicta cancel
relicta reset
relicta clean
```

Use these when state is failed/canceled/stale.

## Plugins
Discover and install:

```bash
relicta plugin list --available
relicta plugin install <name>
relicta plugin enable <name>
relicta plugin configure <name>
```

Confirm plugin status:

```bash
relicta plugin list
relicta plugin info <name>
```

## Governance and Policy
Policy lifecycle:

```bash
relicta policy list
relicta policy validate
```

Use `plan --analyze` before approval to surface governance signals.

## MCP and Dashboard
MCP server:

```bash
relicta mcp serve
```

Remote transport mode:

```bash
relicta mcp serve --transport http --port 8080
```

Dashboard server:

```bash
relicta serve
```

Security rule:
- Treat dashboard auth mode `none` as local-development only.
- For shared environments, configure `dashboard.auth.mode: api_key` in `.relicta.yaml`.

## Troubleshooting Playbook
- Error: configuration not found.
  Action: create or point to `.relicta.yaml` via `--config`.
- Error: workflow state mismatch across commands.
  Action: inspect `relicta status`; then `relicta reset` or `relicta clean` and rerun flow.
- Error: plugin not executing.
  Action: verify install (`plugin list`), enable/configure, then retry publish.
- Error: publish blocked by governance.
  Action: inspect `plan --analyze`, adjust policy or perform required approval.

## Output Standards
When assisting users with Relicta tasks:
- Report exact commands run.
- Summarize state transitions and version/tag outcomes.
- Call out whether run was dry-run, local-only, or published.
- Prefer deterministic, auditable steps over speculative advice.
