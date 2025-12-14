# Relicta Plugins

Plugins extend Relicta's functionality by integrating with external services. Plugins run at specific lifecycle hooks during the release process.

## Available Plugins

- [GitHub](#github) - Create GitHub releases and upload assets
- [GitLab](#gitlab) - Create GitLab releases
- [npm](#npm) - Publish packages to npm registry
- [Slack](#slack) - Send release notifications to Slack
- [Discord](#discord) - Send release notifications to Discord
- [Jira](#jira) - Update Jira tickets and create release notes
- [LaunchNotes](#launchnotes) - Sync release notes to LaunchNotes

## Plugin Lifecycle Hooks

Plugins can execute at different stages of the release process:

```
PreInit → PostInit → PrePlan → PostPlan → PreVersion → PostVersion
  → PreNotes → PostNotes → PreApprove → PostApprove → PrePublish
  → PostPublish → OnSuccess → OnError
```

## Configuration

Plugins are configured in `release.config.yaml`:

```yaml
plugins:
  - name: plugin-name
    enabled: true
    config:
      key: value
```

---

## GitHub

Create GitHub releases and upload build artifacts.

### Configuration

```yaml
plugins:
  - name: github
    enabled: true
    config:
      owner: your-org        # Repository owner (defaults to current repo)
      repo: your-repo        # Repository name (defaults to current repo)
      draft: false           # Create as draft release
      prerelease: false      # Mark as pre-release
      generate_release_notes: false  # Use GitHub's auto-generated notes
      assets:                # Files to upload as release assets
        - "dist/*.tar.gz"
        - "dist/*.zip"
        - "dist/checksums.txt"
```

### Environment Variables

- `GITHUB_TOKEN` - Required for authentication (auto-set in GitHub Actions)

### Hooks

- `PostPublish` - Creates the release and uploads assets

### Example

```yaml
plugins:
  - name: github
    enabled: true
    config:
      assets:
        - "build/myapp-*.tar.gz"
        - "build/myapp-*.zip"
```

---

## GitLab

Create GitLab releases with links to build artifacts.

### Configuration

```yaml
plugins:
  - name: gitlab
    enabled: true
    config:
      project_id: "12345"    # GitLab project ID
      assets:
        - name: "Linux Binary"
          url: "https://example.com/myapp-linux.tar.gz"
          type: "package"    # package, image, runbook, or other
```

### Environment Variables

- `GITLAB_TOKEN` - Required for authentication
- `CI_PROJECT_ID` - Auto-set in GitLab CI (used if project_id not specified)

### Hooks

- `PostPublish` - Creates the release

---

## npm

Publish packages to npm registry.

### Configuration

```yaml
plugins:
  - name: npm
    enabled: true
    config:
      registry: "https://registry.npmjs.org"  # npm registry URL
      access: "public"       # public or restricted
      tag: "latest"          # npm dist-tag
      package_dir: "."       # Directory containing package.json
```

### Environment Variables

- `NPM_TOKEN` - Required for authentication

### Hooks

- `PostPublish` - Publishes the package

### Example

```yaml
plugins:
  - name: npm
    enabled: true
    config:
      access: public
      tag: latest
```

---

## Slack

Send release notifications to Slack channels.

### Configuration

```yaml
plugins:
  - name: slack
    enabled: true
    config:
      channel: "#releases"   # Channel to post to
      username: "Relicta Bot"  # Bot username
      icon_emoji: ":rocket:" # Bot icon
      mention_users:         # Users to mention
        - "@channel"
```

### Environment Variables

- `SLACK_WEBHOOK_URL` - Required Slack webhook URL

### Hooks

- `PostPublish` - Sends notification
- `OnError` - Sends error notification

### Example Message

```
🚀 New Release: v1.2.0

📝 Release Notes:
• Added user authentication
• Fixed login bug
• Updated dependencies

🔗 https://github.com/org/repo/releases/tag/v1.2.0
```

---

## Discord

Send release notifications to Discord channels.

### Configuration

```yaml
plugins:
  - name: discord
    enabled: true
    config:
      username: "Relicta Bot"
      avatar_url: "https://example.com/avatar.png"
      color: 3447003        # Embed color (integer)
```

### Environment Variables

- `DISCORD_WEBHOOK_URL` - Required Discord webhook URL

### Hooks

- `PostPublish` - Sends notification
- `OnError` - Sends error notification

---

## Jira

Update Jira issues and create release notes.

### Configuration

```yaml
plugins:
  - name: jira
    enabled: true
    config:
      host: "https://yourcompany.atlassian.net"
      project: "PROJ"        # Jira project key
      version_prefix: "v"    # Prefix for version names
      transition_status: "Released"  # Status to transition issues to
```

### Environment Variables

- `JIRA_EMAIL` - Jira user email
- `JIRA_API_TOKEN` - Jira API token

### Hooks

- `PostVersion` - Creates version in Jira
- `PostPublish` - Transitions issues and creates release

### Features

- Automatically detects Jira issue keys in commit messages (e.g., PROJ-123)
- Creates a version in Jira for the release
- Transitions issues to specified status
- Adds fix version to issues

---

## LaunchNotes

Sync release notes to LaunchNotes for customer-facing announcements.

### Configuration

```yaml
plugins:
  - name: launchnotes
    enabled: true
    config:
      project_id: "proj_123"
      auto_publish: false    # Auto-publish announcements
      categories:            # Map commit types to categories
        feat: "new-features"
        fix: "bug-fixes"
```

### Environment Variables

- `LAUNCHNOTES_API_KEY` - Required API key

### Hooks

- `PostNotes` - Creates announcement draft
- `PostPublish` - Publishes announcement (if auto_publish: true)

---

## Creating Custom Plugins

Plugins are standalone binaries that communicate with Relicta via gRPC.

### Plugin Interface

```go
package main

import (
    "github.com/relicta-tech/relicta/pkg/plugin"
)

type MyPlugin struct{}

func (p *MyPlugin) GetInfo() plugin.Info {
    return plugin.Info{
        Name:        "my-plugin",
        Version:     "1.0.0",
        Description: "My custom plugin",
        Author:      "Your Name",
        Hooks:       []plugin.Hook{plugin.HookPostPublish},
    }
}

func (p *MyPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
    // Plugin logic here
    return &plugin.ExecuteResponse{
        Success: true,
        Message: "Plugin executed successfully",
    }, nil
}

func (p *MyPlugin) Validate(ctx context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
    // Validate configuration
    return &plugin.ValidateResponse{Valid: true}, nil
}

func main() {
    plugin.Serve(&MyPlugin{})
}
```

### Plugin Discovery

Relicta discovers plugins in:
1. `~/.relicta/plugins/` (global)
2. `.relicta/plugins/` (project-local)

Plugin binaries must be named: `relicta-plugin-{name}`

### Testing Plugins

```bash
# Test plugin locally
relicta plugin list
relicta plugin test my-plugin
```

---

## Troubleshooting

### Plugin Not Found

```
Error: plugin 'github' not found
```

**Solution**: Install the plugin binary:
```bash
# Download from releases
curl -L https://github.com/relicta-tech/relicta/releases/latest/download/github_linux_x86_64 \
  -o ~/.relicta/plugins/relicta-plugin-github
chmod +x ~/.relicta/plugins/relicta-plugin-github
```

### Plugin Failed to Execute

```
Error: plugin 'slack' failed: missing SLACK_WEBHOOK_URL
```

**Solution**: Set required environment variables:
```bash
export SLACK_WEBHOOK_URL="https://hooks.slack.com/..."
```

### Plugin Communication Error

```
Error: failed to communicate with plugin 'npm'
```

**Solution**:
1. Verify plugin binary is executable
2. Check plugin version compatibility
3. Review plugin logs in `.relicta/logs/`

---

## Best Practices

1. **Use Dry Run** - Test plugin configurations with `--dry-run`
   ```bash
   relicta publish --dry-run
   ```

2. **Validate Config** - Check plugin configuration before releasing
   ```bash
   relicta validate
   ```

3. **Environment Variables** - Use environment variables for secrets, never commit to config
   ```yaml
   # ✅ Good
   plugins:
     - name: github
       enabled: true

   # ❌ Bad - Don't commit tokens!
   plugins:
     - name: github
       config:
         token: "ghp_secret123"
   ```

4. **Error Handling** - Use OnError hook for notifications
   ```yaml
   plugins:
     - name: slack
       enabled: true
       hooks:
         - PostPublish
         - OnError  # Get notified of failures
   ```

5. **Incremental Rollout** - Enable plugins one at a time to identify issues

---

## Plugin Development Resources

- [Plugin SDK Documentation](https://github.com/relicta-tech/relicta/tree/main/pkg/plugin)
- [Example Plugins](https://github.com/relicta-tech/relicta/tree/main/plugins)
- [gRPC Protocol](https://github.com/relicta-tech/relicta/tree/main/internal/plugin/proto)
