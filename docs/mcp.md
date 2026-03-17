# MCP Integration Guide

Relicta provides a [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server that enables AI agents like Claude and GPT to manage software releases directly. This guide covers setup, configuration, and usage.

## Overview

The MCP integration allows AI agents to:

- **Plan releases** by analyzing commits since the last release
- **Calculate versions** using semantic versioning rules
- **Generate release notes** with optional AI enhancement
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

## Tools Reference

### relicta_status

Get the current release state and pending actions.

**Input Schema:**
```json
{
  "type": "object"
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
  "can_approve": true
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
      "description": "Starting point: tag, commit SHA, or 'auto'",
      "default": "auto"
    },
    "analyze": {
      "type": "boolean",
      "description": "Include detailed commit analysis"
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
  "commit_count": 15,
  "has_breaking": false,
  "has_features": true,
  "has_fixes": true
}
```

### relicta_bump

Calculate and set the next version based on commits.

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "bump": {
      "type": "string",
      "description": "Version bump type: major, minor, patch, or auto",
      "default": "auto"
    },
    "version": {
      "type": "string",
      "description": "Explicit version to set (overrides bump type)"
    }
  }
}
```

**Response:**
```json
{
  "current_version": "1.1.0",
  "next_version": "1.2.0",
  "bump_type": "minor",
  "auto_detected": true,
  "tag_name": "v1.2.0",
  "tag_created": true
}
```

### relicta_notes

Generate changelog and release notes for the current release.

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "ai": {
      "type": "boolean",
      "description": "Use AI to enhance release notes"
    }
  }
}
```

**Response:**
```json
{
  "summary": "This release adds new authentication features...",
  "changelog": "## [1.2.0] - 2024-01-15\n\n### Features\n...",
  "ai_generated": true
}
```

### relicta_evaluate

Evaluate release risk using the Change Governance Protocol (CGP).

**Input Schema:**
```json
{
  "type": "object"
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
    "notes": {
      "type": "string",
      "description": "Updated release notes (optional)"
    }
  }
}
```

**Response:**
```json
{
  "approved": true,
  "approved_by": "mcp-agent",
  "version": "1.2.0"
}
```

### relicta_publish

Execute the release by creating tags and running plugins.

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "dry_run": {
      "type": "boolean",
      "description": "Simulate release without making changes"
    }
  }
}
```

**Response:**
```json
{
  "tag_name": "v1.2.0",
  "release_url": "https://github.com/org/repo/releases/tag/v1.2.0",
  "dry_run": false,
  "plugin_results": [
    {"plugin": "github", "hook": "post-publish", "success": true, "message": "Release created"}
  ]
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

You can also bind explicitly:

```bash
relicta mcp serve --transport http --address 127.0.0.1:8080
```

### Streaming Support

Long-running operations may emit progress updates depending on MCP client support.

## Client Integration

Relicta exposes a standard MCP server. Use any MCP-compatible client with one of:

- `stdio` transport: `relicta mcp serve`
- `http` transport: `relicta mcp serve --transport http --port 8080`

Most users integrate via Claude Desktop or custom MCP clients that speak JSON-RPC 2.0 over MCP transport.

## Prompts

The MCP server provides prompts for AI agents:

### release-summary

Generate a summary of the upcoming release.

**Arguments:**
- `style`: "brief", "detailed", or "technical"

### risk-analysis

Analyze and explain the risk factors for the current release.

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

1. **Authentication**: The MCP server does not implement authentication. Run it in trusted environments only.

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
User: Plan and execute a release for my project

Agent: I'll help you create a release. Let me check the current state.

[calls relicta_status]

Agent: There's no active release. Let me analyze your commits.

[calls relicta_plan with analyze: true]

Agent: I found 15 commits since v1.1.0:
- 3 features
- 8 bug fixes
- 4 chores

This suggests a minor version bump to v1.2.0. Should I proceed?

User: Yes, proceed

Agent: Creating the release...

[calls relicta_bump]
[calls relicta_notes with ai: true]
[calls relicta_evaluate]

Agent: The release has been prepared:
- Version: 1.2.0
- Risk Score: 0.35 (low)
- Release notes generated

Ready to publish?

User: Yes, publish it

[calls relicta_publish]

Agent: Release v1.2.0 has been published!
- GitHub release: https://github.com/org/repo/releases/tag/v1.2.0
- Changelog updated
- Slack notification sent
```
