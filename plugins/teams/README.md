# Microsoft Teams Plugin

Send rich notifications to Microsoft Teams channels when releases are published.

## Features

- 🎨 **Rich Adaptive Cards** - Beautiful, interactive notifications using Microsoft Adaptive Cards
- 🔔 **Smart Notifications** - Configurable success and error notifications
- 🎯 **@Mentions** - Notify specific users or groups
- 📊 **Release Details** - Automatic facts display (version, type, branch, tag, changes)
- 📝 **Changelog Integration** - Optional changelog inclusion in notifications
- 🎨 **Customizable Themes** - Configure card colors to match your brand
- 🔒 **Security Hardened** - TLS 1.2+, SSRF protection, redirect prevention
- 🧪 **Dry-Run Support** - Test notifications without sending

## Installation

The Teams plugin is included with Relicta. No separate installation required.

## Configuration

Add the Teams plugin to your `release.config.yaml`:

```yaml
plugins:
  - name: teams
    enabled: true
    config:
      webhook_url: ${TEAMS_WEBHOOK_URL}
      notify_on_success: true
      notify_on_error: true
      include_changelog: false
      theme_color: "28a745"  # Green for success, red for errors
      mentions:
        - "@user1"
        - "@team-leads"
```

### Configuration Options

| Option | Type | Required | Default | Description |
|--------|------|----------|---------|-------------|
| `webhook_url` | string | Yes | - | Teams incoming webhook URL or `${TEAMS_WEBHOOK_URL}` env var |
| `notify_on_success` | boolean | No | `true` | Send notification on successful release |
| `notify_on_error` | boolean | No | `true` | Send notification on release failure |
| `include_changelog` | boolean | No | `false` | Include full changelog in notification (truncated at 2000 chars) |
| `theme_color` | string | No | `"28a745"` | Hex color for card accent (6 digits, with or without #) |
| `mentions` | array | No | `[]` | List of users/groups to @mention |

## Setting Up Teams Webhook

1. **Open Microsoft Teams** and navigate to the channel where you want notifications
2. **Click the three dots** (⋯) next to the channel name
3. **Select "Connectors"** or "Workflows" (depending on Teams version)
4. **Configure "Incoming Webhook"**:
   - Name: `Relicta`
   - Icon: Upload your project logo (optional)
   - Click "Create"
5. **Copy the webhook URL** - it will look like:
   - `https://outlook.office.com/webhook/...`
   - `https://outlook.office365.com/webhook/...`
   - `https://*.webhook.office.com/webhookb2/...`
6. **Set environment variable**:
   ```bash
   export TEAMS_WEBHOOK_URL="https://outlook.office.com/webhook/YOUR_WEBHOOK_HERE"
   ```

## Examples

### Basic Configuration

Minimal setup with just the webhook:

```yaml
plugins:
  - name: teams
    enabled: true
    config:
      webhook_url: ${TEAMS_WEBHOOK_URL}
```

### Success Notifications Only

Only notify on successful releases:

```yaml
plugins:
  - name: teams
    enabled: true
    config:
      webhook_url: ${TEAMS_WEBHOOK_URL}
      notify_on_success: true
      notify_on_error: false
```

### With Changelog and Mentions

Include full changelog and notify team leads:

```yaml
plugins:
  - name: teams
    enabled: true
    config:
      webhook_url: ${TEAMS_WEBHOOK_URL}
      include_changelog: true
      theme_color: "0078d4"  # Microsoft blue
      mentions:
        - "@engineering"
        - "@product-managers"
```

### Custom Theme Colors

Match your brand colors:

```yaml
plugins:
  - name: teams
    enabled: true
    config:
      webhook_url: ${TEAMS_WEBHOOK_URL}
      theme_color: "ff6b35"  # Custom brand color
```

**Suggested Colors:**
- Success: `28a745` (green)
- Error: `dc3545` (red)
- Info: `17a2b8` (teal)
- Warning: `ffc107` (yellow)
- Microsoft Blue: `0078d4`

### Multiple Teams Channels

Send notifications to different channels:

```yaml
plugins:
  # Production releases
  - name: teams
    enabled: true
    config:
      webhook_url: ${TEAMS_PROD_WEBHOOK_URL}
      notify_on_success: true
      notify_on_error: false

  # Error alerts
  - name: teams
    enabled: true
    config:
      webhook_url: ${TEAMS_ALERTS_WEBHOOK_URL}
      notify_on_success: false
      notify_on_error: true
      theme_color: "dc3545"
      mentions:
        - "@on-call-engineer"
```

## Notification Content

### Success Notification

When a release is published successfully, Teams receives an Adaptive Card with:

- **Title**: "🚀 Release {version} Published!"
- **Facts**:
  - Version (e.g., `1.2.3`)
  - Release Type (e.g., `Major`, `Minor`, `Patch`)
  - Branch (e.g., `main`)
  - Tag (e.g., `v1.2.3`)
  - Changes Summary (e.g., `3 features, 2 fixes, 1 breaking change`)
- **Changelog** (if `include_changelog: true`)
  - Full release notes (truncated at 2000 characters)
- **Color**: Green (`28a745`) or custom color

### Error Notification

When a release fails:

- **Title**: "❌ Release {version} Failed"
- **Facts**:
  - Version
  - Branch
- **Color**: Red (`dc3545`) or custom color
- **Mentions**: All configured users/groups to alert them

## Message Size Limits

Microsoft Teams has a **28KB message size limit**. The plugin automatically:
- Truncates changelog at 2000 characters if `include_changelog: true`
- Validates total message size before sending
- Returns clear error if message exceeds limit

## Security

The Teams plugin implements multiple security measures:

### SSRF Protection
- Only allows HTTPS connections
- Validates webhook URLs against allowed Teams hosts:
  - `outlook.office.com`
  - `outlook.office365.com`
  - `*.webhook.office.com`
- Blocks redirects to non-Teams hosts

### TLS Security
- Requires TLS 1.2 or higher
- Prevents downgrade attacks

### Redirect Protection
- Limits redirect chain to 3 hops
- Blocks redirects to HTTP
- Prevents redirect away from Teams hosts

### Input Validation
- Validates webhook URL format
- Validates theme color format (6-digit hex)
- Sanitizes all user inputs

## Testing

### Dry Run Mode

Test your configuration without sending actual notifications:

```bash
relicta publish --dry-run
```

The plugin will validate configuration and show what would be sent:

```
✓ Would send Teams success notification
  version: 1.2.3
```

### Manual Testing

Test your webhook manually:

```bash
curl -H "Content-Type: application/json" -d '{
  "@type": "MessageCard",
  "@context": "https://schema.org/extensions",
  "summary": "Test",
  "themeColor": "28a745",
  "sections": [{
    "activityTitle": "Test Notification",
    "text": "This is a test from Relicta"
  }]
}' https://outlook.office.com/webhook/YOUR_WEBHOOK_HERE
```

## Troubleshooting

### Webhook URL Not Working

**Error**: `Teams webhook URL is required`

**Solution**: Set the environment variable:
```bash
export TEAMS_WEBHOOK_URL="https://outlook.office.com/webhook/..."
```

Or configure directly in YAML (not recommended for security):
```yaml
webhook_url: "https://outlook.office.com/webhook/..."
```

### Invalid Webhook URL

**Error**: `URL must be a valid Teams webhook host`

**Cause**: Using an invalid or suspicious webhook URL

**Solution**: Ensure your webhook URL:
- Uses HTTPS
- Is from `outlook.office.com`, `outlook.office365.com`, or `*.webhook.office.com`
- Doesn't contain malicious redirects

### Message Too Large

**Error**: `message size exceeds Teams 28KB limit`

**Cause**: Changelog or release notes are too large

**Solutions**:
1. Disable changelog: `include_changelog: false`
2. Shorten release notes
3. Use summary instead of full changelog

### Notifications Not Appearing

**Possible Causes**:
1. Webhook expired or deleted in Teams
2. Channel deleted or renamed
3. Permissions changed
4. Incorrect webhook URL

**Solutions**:
1. Recreate webhook in Teams channel
2. Update `TEAMS_WEBHOOK_URL` environment variable
3. Test with dry-run mode first
4. Check Teams channel settings

### Theme Color Not Applied

**Error**: `theme color must be a 6-digit hex color`

**Cause**: Invalid color format

**Solutions**:
- Use 6 digits: `28a745` ✓
- With or without #: `#28a745` ✓
- Not 3 digits: `fff` ✗
- Not 8 digits: `28a74512` ✗

## Advanced Usage

### Conditional Notifications

Only notify for major releases:

```yaml
plugins:
  - name: teams
    enabled: true
    config:
      webhook_url: ${TEAMS_WEBHOOK_URL}
      # Use workflow rules to conditionally enable
```

### Multiple Environments

Different webhooks for different environments:

```yaml
# production.config.yaml
plugins:
  - name: teams
    enabled: true
    config:
      webhook_url: ${TEAMS_PROD_WEBHOOK_URL}
      theme_color: "28a745"

# staging.config.yaml
plugins:
  - name: teams
    enabled: true
    config:
      webhook_url: ${TEAMS_STAGING_WEBHOOK_URL}
      theme_color: "ffc107"  # Yellow for staging
```

## Adaptive Cards Reference

The Teams plugin uses [Adaptive Cards 1.4](https://adaptivecards.io/):

- **Card Type**: MessageCard with Adaptive Card attachment
- **Elements Used**:
  - TextBlock (title, changelog)
  - FactSet (version details)
  - Container (layout)

### Example Card Structure

```json
{
  "@type": "MessageCard",
  "@context": "https://schema.org/extensions",
  "themeColor": "28a745",
  "summary": "🚀 Release 1.2.3 Published!",
  "attachments": [{
    "contentType": "application/vnd.microsoft.card.adaptive",
    "content": {
      "type": "AdaptiveCard",
      "version": "1.4",
      "$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
      "body": [
        {
          "type": "TextBlock",
          "text": "🚀 Release 1.2.3 Published!",
          "size": "large",
          "weight": "bolder"
        },
        {
          "type": "FactSet",
          "facts": [
            {"title": "Version", "value": "1.2.3"},
            {"title": "Release Type", "value": "Minor"},
            {"title": "Branch", "value": "main"},
            {"title": "Tag", "value": "v1.2.3"}
          ]
        }
      ]
    }
  }]
}
```

## Integration with Other Plugins

Teams works seamlessly with other notification plugins:

```yaml
plugins:
  # Slack for engineering team
  - name: slack
    enabled: true
    config:
      webhook: ${SLACK_WEBHOOK_URL}

  # Teams for product team
  - name: teams
    enabled: true
    config:
      webhook_url: ${TEAMS_WEBHOOK_URL}

  # Discord for community
  - name: discord
    enabled: true
    config:
      webhook: ${DISCORD_WEBHOOK_URL}
```

## License

MIT License - Part of Relicta

## Support

- 🐛 [Report Issues](https://github.com/relicta-tech/relicta/issues)
- 📖 [Documentation](https://github.com/relicta-tech/relicta)
- 💬 [Discussions](https://github.com/relicta-tech/relicta/discussions)
