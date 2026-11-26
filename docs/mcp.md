# MCP Integration Guide

Relicta provides a [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server that enables AI agents like Claude and GPT to manage software releases directly. This guide covers setup, configuration, and usage.

**Release availability:** the agent-session customer-notes workflow described here is in the
unreleased v5 source. The current public release is v4.2.0 and does not include this workflow;
use a v5 source build until v5 is published.

## Overview

The MCP integration allows AI agents to:

- **Plan releases** by analyzing commits since the last release
- **Calculate versions** using semantic versioning rules
- **Draft release notes in the agent session** using the deterministic recommendation returned by `relicta_plan`, then persist that draft on the release
- **Generate deterministic or configured AI notes** when a draft is not supplied
- **Evaluate risk** using the Change Governance Protocol (CGP)
- **Approve and publish releases** with full plugin support

## Quick Start

### Starting the Server

```bash
# Default: stdio transport (for Claude Desktop)
relicta mcp serve

# HTTP transport (for custom integrations)
relicta mcp serve --port 8080
```

### Claude Desktop Configuration

Add to `~/.config/claude/claude_desktop_config.json` (macOS/Linux) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "relicta": {
      "command": "relicta",
      "args": ["mcp", "serve"],
      "cwd": "/path/to/your/project"
    }
  }
}
```

### Codex CLI and IDE extension

Codex uses the same MCP server configuration in its CLI and IDE extension. For
a single repository, or when every Codex session starts in the same repository,
register a user-level local stdio server:

```bash
codex mcp add relicta -- relicta mcp serve
codex mcp list
```

This registers the server in the user-level Codex configuration. Start or
restart Codex, then use `/mcp` in the Codex CLI to confirm that `relicta` is
connected. Make sure the `relicta` executable is on the environment's `PATH`;
use its absolute path in the command if Codex cannot find it. Unless configured
otherwise, the server inherits its working directory when it starts, so a
user-level entry is easy to misdirect when you work across repositories. See
the [Codex MCP documentation](https://developers.openai.com/codex/mcp/) for
CLI, IDE, and configuration-file options.

#### Use Relicta across multiple repositories

A Relicta server instance is scoped to one repository working directory. For
Codex, configure the server in each repository's trusted `.codex/config.toml`
and set `cwd` to that checkout's absolute path. Keep this machine-specific file
out of shared commits (for example, add it to `.git/info/exclude`), and do not
also register a user-level Relicta server. Codex loads project configuration
only for trusted repositories. This keeps each repository's plans, notes, and
release state attached to the intended checkout.

```toml
[mcp_servers.relicta]
command = "/absolute/path/to/relicta"
args = ["mcp", "serve"]
cwd = "/absolute/path/to/your/repository"
```

Repeat this setup in each repository where you want the Relicta tools. Confirm
the active checkout and release version from `relicta_plan` before saving a
plan or notes. Codex supports project-scoped MCP configuration; see its
[project configuration guide](https://developers.openai.com/codex/config-advanced/)
for trust and loading behavior.

Relicta's MCP tools can change release state and publish a release. Keep those
actions subject to the normal Relicta approval policy and your Codex tool
approval settings; do not grant unattended publishing unless that is an
intentional repository policy.

## Tools Reference

### relicta_status

Get the current release state and pending actions.

Pass the returned `release_id` to `relicta_notes`, `relicta_approve`, or
`relicta_publish` to bind the action to the release you inspected. Those tools
also accept an explicit ID directly; when omitted, they use the active release.

When notes have been saved, the structured result includes their exact text in
`notes`, allowing an agent to read back and review the persisted draft before
approval. `notes_source` identifies how they were created. If a technical
changelog fallback was saved, `notes_warning` explains that it should be
rewritten as customer-facing prose. For agent-authored notes, the warning says
Relicta does not fact-check them and every claim must be checked against the
reviewed changes before approval. These fields are omitted when there are no
saved notes or no warning.

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "release_id": {
      "type": "string",
      "description": "Release ID to inspect. Uses the active release if omitted."
    }
  }
}
```

**Response:**
```json
{
  "release_id": "rel-123",
  "state": "planned",
  "version": "1.2.0",
  "created": "2024-01-15T10:00:00Z",
  "updated": "2024-01-15T10:30:00Z",
  "can_approve": false,
  "next_action": "bump"
}
```

For agent-authored notes, status returns the provenance and factual-review
warning:

```json
{
  "release_id": "rel-123",
  "state": "notes_ready",
  "version": "1.2.0",
  "can_approve": true,
  "next_action": "approve",
  "notes": "This update refreshes project dependencies and improves automated security checks.",
  "notes_source": "agent",
  "notes_warning": "These release notes were supplied by an agent and are not fact-checked by Relicta. Verify every claim against the reviewed changes before approval."
}
```

When notes were saved from a technical changelog fallback, the warning instead
asks for customer-facing prose:

```json
{
  "release_id": "rel-123",
  "state": "notes_ready",
  "version": "1.2.0",
  "can_approve": true,
  "next_action": "approve",
  "notes": "feat(auth): add OAuth support",
  "notes_source": "changelog-fallback",
  "notes_warning": "Saved release notes use a technical changelog fallback. Review or replace them with customer-facing prose before approval."
}
```

### relicta_plan

Analyze commits since the last release and suggest a version bump.

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "from": {
      "type": "string",
      "description": "Starting reference for commit analysis (tag or commit SHA). Leave empty for automatic detection from the latest version tag."
    },
    "to": {
      "type": "string",
      "description": "Ending reference for commit analysis. Defaults to HEAD."
    },
    "analyze": {
      "type": "boolean",
      "description": "Include detailed commit analysis"
    },
    "dry_run": {
      "type": "boolean",
      "description": "Return analysis and recommendation without persisting a release plan"
    },
    "allow_diverged_version_line": {
      "type": "boolean",
      "description": "Acknowledge a higher version tag outside the target history and allow persisting this plan. Set true only after confirming an intentional maintenance line."
    },
    "no_ai": {
      "type": "boolean",
      "description": "Disable AI-powered commit classification and use conventional commit parsing only"
    },
    "min_confidence": {
      "type": "number",
      "description": "Minimum confidence threshold (0.0-1.0) for AI classifications; defaults to 0.7"
    }
  }
}
```

**Response:**
```json
{
  "release_id": "rel-123",
  "current_version": "1.1.0",
  "next_version": "1.2.0",
  "release_type": "minor",
  "warnings": [],
  "commit_count": 15,
  "has_breaking": false,
  "has_features": true,
  "has_fixes": true,
  "recommendation": {
    "schema_version": "1.0.0",
    "subject": {"repository": "owner/repo", "branch": "main"},
    "facts": {
      "current_version": "1.1.0",
      "next_version": "1.2.0",
      "release_type": "minor",
  "warnings": [],
  "version_line_acknowledged": false,
      "commit_count": 15,
      "changes": [{"type": "feat", "scope": "auth", "subject": "Add OAuth support", "breaking": false}],
      "breaking_changes": []
    },
    "obligations": [],
    "provenance": {"tool_version": "<installed-version>", "deterministic": true, "inputs_digest": "sha256:…"}
  },
  "recommendation_persisted": true
}
```

The recommendation is the factual input for customer-facing prose. The MCP
caller can compose notes from it in the current agent session and persist them
with `relicta_notes`; Relicta stores the supplied text on the active release so
the normal approval workflow reviews the same draft. Relicta also stores the
recommendation beside the release when the configured repository supports it.
Check `recommendation_persisted`; when it is false, `recommendation_warning`
explains why the artifact is available only in the plan response and not in
later audit/API reads.

If `warnings` contains a higher version tag that is not an ancestor of the
target, inspect the repository's branch and tag history before proceeding. The
commit analysis uses the newest reachable tag as its range boundary, but the
repository may be stale or may intentionally be on a maintenance line. A
non-dry-run plan is refused until the caller repeats it with
`allow_diverged_version_line: true`; that acknowledgment is stored in the plan
transition history and returned as `version_line_acknowledged: true`. Dry-runs
can always be used to inspect the warning without saving a plan.

#### Draft customer-facing release notes in your agent session

To inspect the recommendation before creating release state, call
`relicta_plan` with `dry_run: true`. This is read-only: it returns the
recommendation but does not create a release record, so it cannot be followed
directly by `relicta_bump` or `relicta_notes`. Use the live plan below when you
are ready to save and review a draft.

The release state machine requires a version before notes can be saved. Use
this order when drafting notes in the agent session:

1. Call `relicta_plan` with `dry_run: true` and review its `recommendation`
   and commit facts. Draft customer-facing prose from those facts, keeping claims tied to
   the listed changes and leaving out internal implementation details customers
   do not need. `relicta_notes` persists the exact agent draft, including
   surrounding whitespace, for approval; it does not fact-check or edit
   agent-authored prose.
2. If tag-triggered CI should publish this text, save it as
   `docs/release-notes/<version>.public.md` and commit that file before planning.
3. Call `relicta_plan` without `dry_run` to create the release plan. Recheck the
   recommendation against the dry-run facts because the analysis may change if
   commits changed since the preview. Before bumping or saving notes, verify that
   the version, release type, classified changes, and supported caveats still
   match the draft. If any of those facts changed, discard the stale draft and
   rewrite it from the live recommendation.
4. Call `relicta_bump` to set the version, then call `relicta_notes` with the
   prose in `draft`.
5. Review the saved draft. Evaluating governance, approving, and publishing are
   separate downstream release steps; drafting and saving the notes does not
   require approval or publication.

The MCP release state is local and ignored by Git. Relicta's own tag-triggered
workflow looks for `docs/release-notes/<version>.public.md`; CI saves that copy
onto the release run, presents it for protected human review, then reads the
approved text from the release record and supplies it to GoReleaser. The file
must be committed before planning because Relicta binds release actions to the
planned HEAD. Other repositories can use `relicta notes --draft-file <path>`
and wire their CI to publish notes from the approved release record.

`relicta_bump` applies the selected version to release state and configured
version files; it does not create a tag or publish. Saving a draft with
`relicta_notes` also does not approve or publish. `relicta_publish` is the
external side-effect boundary: after approval, it can create and push the Git
tag and run configured publisher plugins. Treat that call as a release action,
not as part of drafting or review.

MCP bumping and note generation verify that repository `HEAD` still matches
the commit analyzed by the plan. If either call reports that the repository
changed, plan again and draft from the new recommendation; do not reuse notes
based on the stale plan.

For technical commit histories, use a prompt like this in the calling agent
session:

> Write customer-facing release notes for version `<version>` from the Relicta
> plan recommendation below. Use the product purpose, audience, and vocabulary
> already established in this agent session to frame the notes, while grounding
> every release claim in the recommendation. Explain user-visible outcomes in
> plain language.
> Group related changes, omit internal refactors and implementation names, and
> do not infer behavior or benefits that the recommendation does not support.
> If product or audience context is unavailable, do not guess.
> If a change has no clear customer impact, omit it. Preserve any important
> compatibility, migration, security, or operational caveats. Return only the
> release-note text, with no preamble.
>
> Recommendation: `<paste the recommendation JSON>`

Before saving, compare every claim against the recommendation and check that the
notes describe the right product and audience. Do not infer that a capability or
type of change is absent just because the recommendation contains no matching
classified commits; make negative claims only when the reviewed change set proves
them. Relicta stores the draft as provided, including surrounding whitespace, and
does not review its claims.

### Govee maintenance example

This workflow is useful when commit messages describe implementation work but
the release needs product-facing language. A Govee Light Management v3.1.3
dry-run recommendation contained seven commits and no feature-classified
changes: dependency updates, security-scanning and workflow maintenance, and
internal cleanup. One dependency commit says it clears a high-severity npm
audit finding, but the available release facts do not identify affected shipped
code or customer exposure. Keep that severity out of customer-facing claims
unless the release owner verifies the product impact. A grounded maintenance
draft is:

> ## Maintenance update
>
> This maintenance update refreshes project dependencies and automated security
> and build workflows.

The example omits internal implementation detail without claiming that unclassified changes or
features are absent. It also avoids turning an audit severity in a commit message into a claim
about customer exposure. The recommendation was generated read-only from the current Govee
checkout; it excludes the checkout's uncommitted changes. This is a drafting example, not an
approved Govee release announcement.

### relicta_bump

Apply the planned version by default. Set `level` to `major`, `minor`, or
`patch` to override the plan's bump kind, or set `version` to an explicit
semantic version. Bumping writes configured version files and advances the
release to its versioned state.

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "release_id": {
      "type": "string",
      "description": "Release ID to bump. Uses the latest planned release if omitted."
    },
    "level": {
      "type": "string",
      "description": "Version bump type: major, minor, patch, or auto",
      "default": "auto"
    },
    "version": {
      "type": "string",
      "description": "Explicit semantic version; overrides level and planned version"
    },
    "prerelease": {
      "type": "string",
      "description": "Validated prerelease identifier, such as beta or rc.1"
    },
    "build": {
      "type": "string",
      "description": "Validated build metadata to append to the selected version"
    }
  }
}
```

**Response:**
```json
{
  "release_id": "rel_abc123",
  "current_version": "1.1.0",
  "next_version": "1.2.0",
  "bump_type": "minor",
  "auto_detected": true
}
```

The MCP bump advances the selected release (or the latest planned release when
`release_id` is omitted), returns the selected ID, and writes configured version
files. Pass the same `release_id` returned by status to keep a multi-run workflow
bound to one release. It does not create a Git tag; tag creation happens during
publish.

### relicta_notes

Generate changelog and release notes for the active release, or for the release
named by `release_id`.

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "release_id": {
      "type": "string",
      "description": "Release ID to update. Uses the active release if omitted."
    },
    "draft": {
      "type": "string",
      "description": "Customer-facing release notes written by the calling agent. When supplied, Relicta stores this text without calling an AI provider."
    },
    "ai": {
      "type": "boolean",
      "description": "Use the configured AI provider when no draft is supplied"
    },
    "audience": {
      "type": "string",
      "enum": ["developers", "users", "public", "stakeholders"]
    },
    "tone": {
      "type": "string",
      "enum": ["technical", "friendly", "professional", "marketing"]
    },
    "language": {
      "type": "string",
      "description": "Output language for generated notes; defaults to English"
    },
    "emoji": {
      "type": "boolean",
      "description": "Include emojis in generated notes"
    }
  }
}
```

**Response:**
```json
{
  "release_id": "rel_abc123",
  "version": "1.2.0",
  "state": "notes_ready",
  "notes": "Sign in more reliably with refreshed authentication, and recover gracefully when a session expires.",
  "summary": "Sign in more reliably with refreshed authentication, and recover gracefully when a session expires.",
  "ai_generated": false,
  "agent_authored": true,
  "source": "agent",
  "warning": "Agent-authored notes were saved exactly as supplied and are not fact-checked by Relicta. Verify every claim against the release recommendation before approval."
}
```

For an agent-authored draft, `changelog` is omitted because Relicta has not
generated a separate technical changelog. The exact saved customer-facing text
is returned once in `notes`.

Pass the agent-written text in `draft`:

```json
{
  "draft": "Sign in more reliably with refreshed authentication, and recover gracefully when a session expires."
}
```

When `draft` is present, it takes precedence over `ai`, is stored with the
selected release, and is returned with `agent_authored: true`. This path does not
call a model provider. The agent should use the `recommendation` from
`relicta_plan` as its source of release facts and avoid adding unsupported
claims. Since `relicta_notes` requires a versioned release, call
`relicta_bump` after planning and before saving the draft. The saved notes are
then available to `relicta_approve` and the normal publish workflow. The
response returns the persisted text in `notes`, with the release ID, version,
and resulting `notes_ready` state so the caller can verify what was saved. The
tool advertises an output schema and provides the same fields through MCP's
structured content, so clients do not need to parse the text block as JSON.
The response's `source` identifies the stored text as `agent`, `basic`, the
configured AI provider, or `changelog-fallback`. When AI prose is unavailable,
`warning` explains that the saved text is only a changelog and should be
rewritten for customers before approval. `ai_generated` reflects the stored
content source; a changelog fallback can be AI-generated while still lacking
the customer-facing narrative.

### relicta_evaluate

Evaluate release risk using the Change Governance Protocol (CGP).

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "release_id": {
      "type": "string",
      "description": "Release ID to evaluate. Uses the latest active release if omitted."
    }
  }
}
```

**Response:**
```json
{
  "decision": "approve",
  "risk_score": 0.35,
  "severity": "low",
  "can_auto_approve": true,
  "required_actions": [],
  "risk_factors": [
    {"name": "blast_radius", "score": 0.2, "description": "5 files changed"},
    {"name": "api_changes", "score": 0.4, "description": "New endpoints added"}
  ],
  "rationale": "Low-risk release with minor feature additions"
}
```

### relicta_approve

Approve the release for publishing.

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "release_id": {
      "type": "string",
      "description": "Release ID to approve. Uses the active release if omitted."
    },
    "notes": {
      "type": "string",
      "description": "Replace the saved release notes before approval. The exact supplied text is persisted and becomes the approved draft."
    },
    "message": {
      "type": "string",
      "description": "Approval message or reason recorded in the audit trail."
    }
  }
}
```

**Response:**
```json
{
  "approved": true,
  "approved_by": "mcp-cosigner",
  "version": "1.2.0"
}
```

MCP bump, notes, approval, and publish calls verify that repository `HEAD`
still matches the planned commit. If the repository changed, plan again before
continuing. When `notes` is supplied, approval first saves that replacement via
the notes workflow; if saving it fails, approval does not proceed.
With the default restrictive agent budget, approval also requires an accepted
human elicitation. That manual approval is recorded and satisfies the cosigner
requirement for publish. The confirmation includes the release risk score and
the notes that will be approved (including a supplied replacement draft). A
dry-run publish can be previewed without a cosigner. Notes edits serialize on
the release lock; approval verifies the reviewed text hash while holding that
same lock and asks for a fresh review if the draft changed in the meantime.
The approval confirmation includes the note-quality warning returned by
`relicta_status` for the saved notes. When replacement notes are supplied with
approval, it instead warns that the replacement was supplied by an agent and
is not fact-checked. Technical changelog fallbacks are labeled when they are
the notes being approved.

### relicta_publish

Execute the release by creating tags and running plugins.

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "release_id": {
      "type": "string",
      "description": "Release ID to publish. Uses the active release if omitted."
    },
    "dry_run": {
      "type": "boolean",
      "description": "Simulate release without making changes"
    },
    "skip_push": {
      "type": "boolean",
      "description": "Skip pushing Git tags to the remote repository"
    },
    "skip_tag": {
      "type": "boolean",
      "description": "Skip creating the Git tag when it already exists"
    },
    "skip_plugins": {
      "type": "boolean",
      "description": "Skip running configured publishing and notification plugins"
    }
  }
}
```

**Response:**
```json
{
  "release_id": "rel_abc123",
  "tag_name": "v1.2.0",
  "release_url": "https://github.com/org/repo/releases/tag/v1.2.0",
  "dry_run": false,
  "plugin_results": [
    {"plugin": "github", "hook": "post-publish", "success": true, "message": "Release created"}
  ]
}
```

Pass the `release_id` returned by `relicta_status` to publish the approved run.
The response repeats the selected ID so callers can verify which run was
published or previewed.

### relicta_cancel

Cancel an in-progress release. Pass `release_id` to bind cancellation to the
run you inspected; when omitted, Relicta uses the latest active run.

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "release_id": {"type": "string"},
    "reason": {"type": "string"},
    "force": {"type": "boolean"}
  }
}
```

### relicta_reset

Delete a failed or canceled release run. Pass `release_id` to bind reset to
that run; `force` allows deleting an in-progress run.

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "release_id": {"type": "string"},
    "force": {"type": "boolean"}
  }
}
```

## Resources Reference

### relicta://state

Current release state machine status.

```json
{
  "state": "approved",
  "version": "1.2.0",
  "created_at": "2024-01-15T10:00:00Z",
  "updated_at": "2024-01-15T11:00:00Z"
}
```

### relicta://config

Current Relicta configuration.

```json
{
  "product_name": "MyApp",
  "ai_enabled": true,
  "ai_provider": "openai",
  "versioning_strategy": "conventional"
}
```

### relicta://commits

Recent commits since last release.

```json
{
  "commits": [
    {
      "sha": "abc123",
      "type": "feat",
      "scope": "auth",
      "description": "Add OAuth support",
      "breaking": false
    }
  ],
  "count": 15
}
```

### relicta://changelog

Generated changelog for current release.

```markdown
## [1.2.0] - 2024-01-15

### Features

- **auth:** Add OAuth support (abc123)

### Bug Fixes

- **api:** Fix rate limiting issue (def456)
```

### relicta://risk-report

CGP risk assessment for current release.

```json
{
  "score": 0.35,
  "severity": "low",
  "factors": [
    {"name": "blast_radius", "score": 0.2},
    {"name": "api_changes", "score": 0.4}
  ],
  "summary": "Low-risk release"
}
```

## Advanced Features

### HTTP Transport

Run the MCP server over HTTP for remote/custom integrations:

```bash
relicta mcp serve --transport http --port 8080
```

HTTP transport binds to `127.0.0.1` by default. The MCP HTTP endpoint does not
provide authentication itself, so keep it local unless a trusted reverse proxy
provides TLS and authentication and network policy prevents direct access. To
bind it to a non-loopback address, opt in explicitly:

```bash
relicta mcp serve --transport http --address 0.0.0.0:8080 --allow-insecure-http
```

The opt-in only permits the listener; it does not add authentication or TLS.
Do not expose that listener directly to an untrusted network. Keep the upstream
listener private behind the proxy or an equivalent network boundary.

For Docker deployments, loopback is inside the Relicta container. Put Relicta
and the authenticating TLS proxy on a private Docker network, publish only the
proxy's host port, and bind Relicta to the private interface. For example:

```yaml
services:
  relicta:
    image: ${RELICTA_IMAGE:?Set RELICTA_IMAGE to a verified Relicta image digest}
    command: ["mcp", "serve", "--transport", "http", "--address", "0.0.0.0:8080", "--allow-insecure-http"]
    expose:
      - "8080"
    networks: [mcp_private]
  proxy:
    image: ${PROXY_IMAGE:?Set PROXY_IMAGE to a verified proxy image digest}
    ports:
      - "443:443"
    networks: [mcp_private]
    # Configure TLS and client authentication here; proxy only to relicta:8080.

networks:
  mcp_private:
    internal: true
```

The example is a topology, not a complete proxy configuration: configure
certificate management and client authentication for the chosen proxy before
deployment. Set both `RELICTA_IMAGE` and `PROXY_IMAGE` in the deployment
environment to exact image digests that have passed the [container provenance
and vulnerability checks](security/supply-chain-security.md#container-images);
do not use mutable tags. The currently published v4.2.0 Relicta image has
documented high and critical vulnerabilities and is not suitable for production.
Until a fixed image is published and verified, build the current source locally
for evaluation or wait for the verified release image before deploying this
example.

You can also bind explicitly:

```bash
relicta mcp serve --transport http --address 127.0.0.1:8080
```

### Streaming Support

Long-running operations may emit progress updates depending on MCP client support.

## Client Integration

Relicta exposes a standard MCP server. Use any MCP-compatible client with one of:

- `stdio` transport: `relicta mcp serve`
- local `http` transport: `relicta mcp serve --transport http --port 8080` (loopback only by default)

Most users integrate via Claude Desktop or custom MCP clients that speak JSON-RPC 2.0 over MCP transport.

## Prompts

The MCP server provides reusable prompts for AI agents. Most provide writing
instructions; supply the relevant release recommendation, commit details, or
governance result as context when using them. `release-summary` first instructs
the agent to call `relicta_plan` with `dry_run: true`, then draft notes from its
recommendation. It does not create a release plan or save notes. The agent should
use `relicta_notes` separately only when the user asks to save the draft.

| Prompt | Purpose | Arguments |
|--------|---------|-----------|
| `release-summary` | Draft notes from a read-only release recommendation | `style`: brief, detailed, or technical |
| `risk-analysis` | Explain release risk factors using CGP context | None |
| `commit-review` | Review commit quality or security | `focus`: compliance, quality, or security |
| `breaking-changes` | Document breaking changes and their impact | None |
| `migration-guide` | Draft upgrade instructions | `audience`: developer, operator, or end-user |
| `release-announcement` | Draft a release announcement for a channel | `channel`: GitHub, blog, social, or email |
| `approval-decision` | Help assess an approval using CGP analysis | None |

## Error Handling

All tools return structured errors:

```json
{
  "isError": true,
  "content": [
    {"type": "text", "text": "Error: No active release found"}
  ]
}
```

Common error codes:
- `-32700` - Parse error
- `-32600` - Invalid request
- `-32601` - Method not found
- `-32602` - Invalid params
- `-32603` - Internal error

## Security Considerations

1. **Authentication**: Stdio inherits the local client's trust boundary. MCP HTTP has no built-in authentication or TLS; it binds to loopback by default. Use a trusted TLS/authenticating proxy and network controls before enabling a non-loopback bind with `--allow-insecure-http`.

2. **Dry Run Mode**: Always use `dry_run: true` when testing to prevent unintended releases.

3. **Plugin Execution**: Plugins are executed with the same permissions as the MCP server process.

4. **Environment Variables**: Sensitive values (API keys, tokens) should be set via environment variables, not passed through MCP tools.

## Troubleshooting

### Server not responding

```bash
# Check if server is running
relicta --log-level debug mcp serve

# Test with JSON-RPC ping
echo '{"jsonrpc":"2.0","id":1,"method":"ping"}' | relicta mcp serve
```

### Claude Desktop not connecting

1. Verify config path is correct
2. Ensure `relicta` is in PATH
3. Check Claude Desktop logs for errors

### Missing dependencies

```bash
# Run health check
relicta health --json
```

## Examples

### Complete Release Workflow

```
User: Prepare customer-facing release notes for my project

Agent: I'll inspect the release facts, then draft and save customer-facing notes for review.

[calls relicta_status]

Agent: There's no active release. Let me analyze your commits.

[calls relicta_plan with dry_run: true]

Agent: I found 15 commits since v1.1.0:
- 3 features
- 8 bug fixes
- 4 chores

The recommendation suggests a minor release. I’ll use only its verified changes as the source for the draft.

User: Yes, proceed

Agent: I’ll create the planned release and save the draft notes.

[calls relicta_plan]
[calls relicta_bump]
[writes concise customer-facing prose from recommendation, omitting unsupported claims]
[calls relicta_notes with draft: "..."]
[calls relicta_evaluate]
[presents the saved notes and governance result for human review]

Agent: The release is ready for approval:
- Version: 1.2.0
- Risk Score: 0.35 (low)
- Agent-authored release notes saved for approval

Review the notes and governance result. I will not approve or publish until you confirm.

User: I approve these notes and authorize publication

[calls relicta_approve after the user's confirmation]

Agent: Release v1.2.0 is approved. Do you want me to publish it now?

User: Publish it

[calls relicta_publish]

Agent: Release v1.2.0 has been published!
- GitHub release: https://github.com/org/repo/releases/tag/v1.2.0
- Changelog updated
- Slack notification sent
```
