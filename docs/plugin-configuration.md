# Plugin configuration

Configuration notes for first-party plugins under `plugins[].config`. The plugin
registry changes independently of this documentation. Use
`relicta plugin list --available` to see the current catalog and
`relicta plugin info <name>` for authoritative fields, required values, and
credential environment variables for an installed registry entry.

```yaml
plugins:
  - name: github
    enabled: true
    config:
      draft: false
```

Relicta passes this block through to the plugin verbatim — it is a `map[string]any` on the
wire, and the plugin validates it. **Each plugin repository is authoritative**; the tables
below are a convenience and can lag behind a plugin release.

Plugins execute as separate processes, but Relicta does not verify plugin signatures. Before
loading an enabled plugin, review `relicta plugin sandbox-status`. Unsigned plugins require
the explicit `--allow-untrusted-plugins` flag on Linux and macOS, where enforcement is
best-effort, and on Windows, where resource enforcement is unavailable. This flag is an
operator trust decision; it does not make an unsigned plugin trusted or sandboxed.

This used to be six Go structs in `internal/config/schema.go` that nothing referenced: not the
loader, not the validator, not the plugin manager. They could not validate a plugin's config
because nothing ever built one, and they showed up in every unread-configuration sweep as forty
fields with no reader. Documentation is what they were; documentation is where they now live.

## GitHub

GitHubPluginConfig is the configuration for the GitHub plugin.

| key | type | meaning |
|---|---|---|
| `owner` | `string` | Owner is the repository owner. |
| `repo` | `string` | Repo is the repository name. |
| `token` | `string` | Token is the GitHub token (can use environment variable expansion). |
| `draft` | `bool` | Draft creates the release as a draft. |
| `prerelease` | `bool` | Prerelease marks the release as a prerelease. |
| `generate_release_notes` | `bool` | GenerateReleaseNotes uses GitHub's auto-generated release notes. |
| `assets` | `[]string` | Assets is a list of files to upload as release assets. |
| `discussion_category` | `string` | DiscussionCategory creates a discussion for the release. |

## NPM

NPMPluginConfig is the configuration for the npm plugin.

| key | type | meaning |
|---|---|---|
| `registry` | `string` | Registry is the npm registry URL. |
| `tag` | `string` | Tag is the npm dist-tag to use. |
| `access` | `string` | Access is the package access level (public, restricted). |
| `otp` | `string` | OTP is the one-time password for 2FA. |
| `dry_run` | `bool` | DryRun performs a dry-run publish. |
| `package_dir` | `string` | PackageDir is the directory containing package.json. |

## Slack

SlackPluginConfig is the configuration for the Slack plugin.

| key | type | meaning |
|---|---|---|
| `webhook` | `string` | WebhookURL is the Slack webhook URL. |
| `channel` | `string` | Channel is the channel to post to (overrides webhook default). |
| `username` | `string` | Username is the bot username. |
| `icon_emoji` | `string` | IconEmoji is the bot icon emoji. |
| `icon_url` | `string` | IconURL is the bot icon URL. |
| `notify_on_success` | `bool` | NotifyOnSuccess sends notification on successful release. |
| `notify_on_error` | `bool` | NotifyOnError sends notification on failed release. |
| `include_changelog` | `bool` | IncludeChangelog includes changelog in the notification. |
| `mentions` | `[]string` | Mentions is a list of users/groups to mention. |

## Discord

DiscordPluginConfig is the configuration for the Discord plugin.

| key | type | meaning |
|---|---|---|
| `webhook` | `string` | WebhookURL is the Discord webhook URL (https://discord.com/api/webhooks/...). |
| `username` | `string` | Username is the bot username (overrides webhook default). |
| `avatar_url` | `string` | AvatarURL is the bot avatar URL (overrides webhook default). |
| `notify_on_success` | `bool` | NotifyOnSuccess sends notification on successful release. |
| `notify_on_error` | `bool` | NotifyOnError sends notification on failed release. |
| `include_changelog` | `bool` | IncludeChangelog includes changelog in the notification. |
| `mentions` | `[]string` | Mentions is a list of users/roles to mention (format: <@user_id> or <@&role_id>). |
| `thread_id` | `string` | ThreadID posts to a specific thread within the channel (optional). |
| `color` | `int` | Color is the embed color in decimal (default varies by status). |

## GitLab

GitLabPluginConfig is the configuration for the GitLab plugin.

| key | type | meaning |
|---|---|---|
| `base_url` | `string` | BaseURL is the GitLab instance URL (default: https://gitlab.com). |
| `project_id` | `string` | ProjectID is the GitLab project ID or path (e.g., "group/project"). |
| `token` | `string` | Token is the GitLab personal access token (can use environment variable expansion). |
| `name` | `string` | Name is the release name (default: "Release {version}"). |
| `description` | `string` | Description is the release description (uses release notes if empty). |
| `ref` | `string` | Ref is the tag ref for the release. |
| `released_at` | `string` | ReleasedAt is the release date (ISO 8601 format). |
| `milestones` | `[]string` | Milestones is a list of milestones to associate with the release. |
| `assets` | `[]string` | Assets is a list of files to upload as release assets. |
| `asset_links` | list of `{name, url, filepath, link_type}` | AssetLinks is a list of external asset links. |

## Jira

JiraPluginConfig is the configuration for the Jira plugin.

| key | type | meaning |
|---|---|---|
| `base_url` | `string` | BaseURL is the Jira instance URL (e.g., "https://your-domain.atlassian.net"). |
| `username` | `string` | Username is the Jira username (email for Jira Cloud). |
| `token` | `string` | Token is the Jira API token (can use environment variable expansion). |
| `project_key` | `string` | ProjectKey is the Jira project key (e.g., "PROJ"). |
| `issue_pattern` | `string` | IssuePattern is a regex pattern to extract issue keys from commits (default: `[A-Z][A-Z0-9]*-\d+`). |
| `create_version` | `bool` | CreateVersion creates a version in Jira for the release. |
| `release_version` | `bool` | ReleaseVersion marks the version as released. |
| `update_fix_version` | `bool` | UpdateFixVersion adds the version to fix version of linked issues. |
| `transition_issues` | `bool` | TransitionIssues transitions issues to a specified status. |
| `transition_name` | `string` | TransitionName is the name of the transition to apply (e.g., "Done", "Released"). |
| `add_comment` | `bool` | AddComment adds a comment to linked issues. |
| `comment_template` | `string` | CommentTemplate is the comment template (supports {{.Version}}, {{.Repository}}, {{.ReleaseURL}}). |
| `version_prefix` | `string` | VersionPrefix is a prefix for the Jira version name (e.g., "v"). |
| `version_description` | `string` | VersionDescription is a description template for the Jira version. |

### GitLab asset links

Each entry of `asset_links`:

| key | type | meaning |
|---|---|---|
| `name` | `string` | Display name for the link. |
| `url` | `string` | URL of the asset. |
| `filepath` | `string` | Direct asset path within the release. |
| `link_type` | `string` | One of `other`, `runbook`, `image`, `package`. |

## Telegram

Registry configuration for the Telegram notification plugin. Confirm optional
field names and defaults with `relicta plugin info telegram` for the version
you install.

| key | type | meaning |
|---|---|---|
| `bot_token` | `string` | Telegram bot token; prefer `TELEGRAM_BOT_TOKEN`. |
| `chat_id` | `string` | Chat ID or channel username; prefer `TELEGRAM_CHAT_ID`. |
| `parse_mode` | `string` | Message format: `MarkdownV2` or `HTML`. |
| `disable_web_page_preview` | `bool` | Disable link previews. |
| `include_changelog` | `bool` | Include changelog text. |
| `message_thread_id` | `int` | Topic thread ID for groups. |
| `notify_on_success` | `bool` | Send a success notification. |
| `notify_on_error` | `bool` | Send an error notification. |

## Helm

| key | type | meaning |
|---|---|---|
| `chart_path` | `string` | Required path to the chart directory. |
| `repository_url` | `string` | Required Helm repository URL. |
| `repository_type` | `string` | Repository type: `oci`, `http`, or `chartmuseum`. |
| `update_chart_version` | `bool` | Update the version in `Chart.yaml`. |
| `lint` | `bool` | Lint the chart before publishing. |
| `username` | `string` | Repository username; may use `HELM_REPO_USERNAME`. |
| `password` | `string` | Repository password; may use `HELM_REPO_PASSWORD`. |

## Dart and Flutter (`pub`)

| key | type | meaning |
|---|---|---|
| `pubspec_path` | `string` | Path to `pubspec.yaml`. |
| `update_version` | `bool` | Update the package version. |
| `run_tests` | `bool` | Run tests before publishing. |
| `dry_run` | `bool` | Validate without publishing. |

## Swift Package Manager (`swift-pm`)

| key | type | meaning |
|---|---|---|
| `package_path` | `string` | Path containing `Package.swift`. |
| `registry_url` | `string` | Swift Package Registry URL. |
| `build` | `bool` | Build the package before publishing. |
| `test` | `bool` | Run tests before publishing. |

## Windows Package Manager (`winget`)

| key | type | meaning |
|---|---|---|
| `package_id` | `string` | Required package identifier, such as `Publisher.AppName`. |
| `installer_url` | `string` | Required installer download URL template. |
| `installer_type` | `string` | Installer type: `exe`, `msi`, `msix`, or `zip`. |
| `create_pr` | `bool` | Create a pull request in `winget-pkgs`. |
| `github_token` | `string` | GitHub token; prefer `GITHUB_TOKEN`. |

## Linear

| key | type | meaning |
|---|---|---|
| `api_key` | `string` | Required Linear API key; prefer `LINEAR_API_KEY`. |
| `team_id` | `string` | Linear team UUID. |
| `team_key` | `string` | Linear team key, such as `ENG`. |
| `update_linked_issues` | `bool` | Update linked issues after release. |
| `create_release_issue` | `bool` | Create a release tracking issue. |
| `issue_prefix` | `string` | Issue prefix pattern found in commits. |
| `project_id` | `string` | Linear project UUID. |
| `released_state` | `string` | Workflow state to apply after release. |

## Container registries

The `acr`, `ecr`, and `gcr` plugins each require `image`, `source_image`, and
`tags`. Registry authentication is provider-specific; confirm the current
plugin metadata and provider credential chain before publishing.

| plugin | additional required keys | other settings |
|---|---|---|
| `acr` | `registry` | `repository`, `auth_method`, `tenant_id`, `client_id`, `client_secret` |
| `ecr` | `registry` | `region` (also accepts `AWS_REGION`) |
| `gcr` | `registry`, `project` | `location`; project also accepts `GCP_PROJECT` |
