# Relicta Plugins

Plugins extend Relicta's functionality by integrating with external services. All official plugins are distributed as separate binaries via the plugin registry and can be installed with `relicta plugin install <name>`.

## Quick Start

```bash
# List available plugins
relicta plugin list --available

# Install a plugin
relicta plugin install github

# Enable the plugin
relicta plugin enable github

# View plugin details
relicta plugin info github
```

## Available Plugins

Relicta provides 20 official plugins across 5 categories:

### Version Control
- [GitHub](#github) - Create GitHub releases and upload assets
- [GitLab](#gitlab) - Create GitLab releases

### Notifications
- [Slack](#slack) - Send release notifications to Slack
- [Discord](#discord) - Send release notifications to Discord
- [Microsoft Teams](#microsoft-teams) - Send release notifications to Microsoft Teams

### Package Managers
- [npm](#npm) - Publish packages to npm registry
- [Homebrew](#homebrew) - Update Homebrew formula with new release
- [PyPI](#pypi) - Publish packages to PyPI
- [Chocolatey](#chocolatey) - Publish packages to Chocolatey
- [Linux Packages](#linux-packages) - Build Linux packages (deb, rpm, apk)
- [crates.io](#cratesio) - Publish packages to crates.io
- [NuGet](#nuget) - Publish packages to NuGet
- [Maven Central](#maven-central) - Publish packages to Maven Central
- [RubyGems](#rubygems) - Publish gems to RubyGems
- [Hex.pm](#hexpm) - Publish packages to Hex.pm
- [Packagist](#packagist) - Publish packages to Packagist
- [Go Modules](#go-modules) - Publish Go modules

### Project Management
- [Jira](#jira) - Create and link Jira release versions
- [LaunchNotes](#launchnotes) - Create releases in LaunchNotes

### Containers
- [Docker](#docker) - Build and push Docker images

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

## Version Control Plugins

### GitHub

Create GitHub releases and upload build artifacts.

**Installation:**
```bash
relicta plugin install github
relicta plugin enable github
```

**Configuration:**
```yaml
plugins:
  - name: github
    enabled: true
    config:
      owner: your-org              # Repository owner (auto-detected)
      repo: your-repo              # Repository name (auto-detected)
      draft: false                 # Create as draft release
      prerelease: false            # Mark as pre-release
      generate_release_notes: false # Use GitHub's auto-generated notes
      discussion_category: ""      # Create discussion for release
      assets:                      # Files to upload as release assets
        - "dist/*.tar.gz"
        - "dist/*.zip"
        - "dist/checksums.txt"
```

**Environment Variables:**
- `GITHUB_TOKEN` - Required for authentication (auto-set in GitHub Actions)

**Hooks:** `PostPublish`, `OnSuccess`, `OnError`

---

### GitLab

Create GitLab releases with links to build artifacts.

**Installation:**
```bash
relicta plugin install gitlab
relicta plugin enable gitlab
```

**Configuration:**
```yaml
plugins:
  - name: gitlab
    enabled: true
    config:
      project_id: "12345"          # GitLab project ID or path
      url: "https://gitlab.com"    # GitLab instance URL
      assets:
        - name: "Linux Binary"
          url: "https://example.com/myapp-linux.tar.gz"
          type: "package"          # package, image, runbook, or other
```

**Environment Variables:**
- `GITLAB_TOKEN` - Required for authentication
- `CI_PROJECT_ID` - Auto-set in GitLab CI

**Hooks:** `PostPublish`

---

## Notification Plugins

### Slack

Send release notifications to Slack channels.

**Installation:**
```bash
relicta plugin install slack
relicta plugin enable slack
```

**Configuration:**
```yaml
plugins:
  - name: slack
    enabled: true
    config:
      channel: "#releases"         # Channel to post to
      username: "Relicta Bot"      # Bot username
      icon_emoji: ":rocket:"       # Bot icon
      notify_on_success: true      # Send on successful release
      notify_on_error: true        # Send on failure
      mention_users:               # Users to mention
        - "@channel"
```

**Environment Variables:**
- `SLACK_WEBHOOK_URL` - Required Slack webhook URL

**Hooks:** `PostPublish`, `OnError`

---

### Discord

Send release notifications to Discord channels.

**Installation:**
```bash
relicta plugin install discord
relicta plugin enable discord
```

**Configuration:**
```yaml
plugins:
  - name: discord
    enabled: true
    config:
      username: "Relicta Bot"
      avatar_url: "https://example.com/avatar.png"
      color: 3447003              # Embed color (integer)
```

**Environment Variables:**
- `DISCORD_WEBHOOK_URL` - Required Discord webhook URL

**Hooks:** `PostPublish`, `OnError`

---

### Microsoft Teams

Send release notifications to Microsoft Teams channels.

**Installation:**
```bash
relicta plugin install teams
relicta plugin enable teams
```

**Configuration:**
```yaml
plugins:
  - name: teams
    enabled: true
    config:
      title: "New Release"         # Message title template
      theme_color: "0076D7"        # Theme color (hex without #)
      notify_on_success: true
      notify_on_error: true
```

**Environment Variables:**
- `TEAMS_WEBHOOK_URL` - Required Microsoft Teams webhook URL

**Hooks:** `PostPublish`

---

## Package Manager Plugins

### npm

Publish packages to npm registry.

**Installation:**
```bash
relicta plugin install npm
relicta plugin enable npm
```

**Configuration:**
```yaml
plugins:
  - name: npm
    enabled: true
    config:
      registry: "https://registry.npmjs.org"
      access: "public"             # public or restricted
      tag: "latest"                # npm dist-tag
      package_dir: "."             # Directory containing package.json
```

**Environment Variables:**
- `NPM_TOKEN` - Required for authentication
- `NPM_OTP` - One-time password for 2FA (optional)

**Hooks:** `PrePublish`, `PostPublish`

---

### Homebrew

Update Homebrew formula with new release.

**Installation:**
```bash
relicta plugin install homebrew
relicta plugin enable homebrew
```

**Configuration:**
```yaml
plugins:
  - name: homebrew
    enabled: true
    config:
      tap: "your-org/homebrew-tap" # Homebrew tap repository
      formula: "your-app"          # Formula name
      url_template: ""             # URL template (uses {{.Version}})
      sha256: ""                   # SHA256 checksum (auto-calculated)
```

**Environment Variables:**
- `HOMEBREW_GITHUB_TOKEN` - GitHub token for tap repository access

**Hooks:** `PostPublish`

---

### PyPI

Publish packages to PyPI.

**Installation:**
```bash
relicta plugin install pypi
relicta plugin enable pypi
```

**Configuration:**
```yaml
plugins:
  - name: pypi
    enabled: true
    config:
      repository: "https://upload.pypi.org/legacy/"
      dist_dir: "dist"             # Directory containing distribution files
      skip_existing: false         # Skip upload if version exists
```

**Environment Variables:**
- `PYPI_USERNAME` - PyPI username (use `__token__` for API tokens)
- `PYPI_PASSWORD` - PyPI password or API token

**Hooks:** `PrePublish`, `PostPublish`

---

### Chocolatey

Publish packages to Chocolatey.

**Installation:**
```bash
relicta plugin install chocolatey
relicta plugin enable chocolatey
```

**Configuration:**
```yaml
plugins:
  - name: chocolatey
    enabled: true
    config:
      package_id: "your-package"   # Chocolatey package ID
      source: "https://push.chocolatey.org/"
      nuspec_path: ""              # Path to .nuspec file
```

**Environment Variables:**
- `CHOCOLATEY_API_KEY` - Required Chocolatey API key

**Hooks:** `PrePublish`, `PostPublish`

---

### Linux Packages

Build Linux packages (deb, rpm, apk).

**Installation:**
```bash
relicta plugin install linuxpkg
relicta plugin enable linuxpkg
```

**Configuration:**
```yaml
plugins:
  - name: linuxpkg
    enabled: true
    config:
      formats:                     # Package formats to build
        - "deb"
        - "rpm"
        - "apk"
      maintainer: "Your Name <email@example.com>"
      description: "Package description"
      homepage: "https://example.com"
      license: "MIT"
      binaries:                    # Binary files to include
        - "bin/myapp"
      output_dir: "dist"
```

**Hooks:** `PrePublish`, `PostPublish`

---

### crates.io

Publish packages to crates.io (Rust).

**Installation:**
```bash
relicta plugin install crates
relicta plugin enable crates
```

**Configuration:**
```yaml
plugins:
  - name: crates
    enabled: true
    config:
      manifest_path: "Cargo.toml"
      allow_dirty: false           # Allow publishing from dirty directory
      dry_run: false               # Perform dry run without publishing
```

**Environment Variables:**
- `CARGO_REGISTRY_TOKEN` - Required crates.io API token

**Hooks:** `PrePublish`, `PostPublish`

---

### NuGet

Publish packages to NuGet (.NET).

**Installation:**
```bash
relicta plugin install nuget
relicta plugin enable nuget
```

**Configuration:**
```yaml
plugins:
  - name: nuget
    enabled: true
    config:
      source: "https://api.nuget.org/v3/index.json"
      package_path: ""             # Path to .nupkg (auto-detected)
      skip_duplicate: false        # Skip if version exists
```

**Environment Variables:**
- `NUGET_API_KEY` - Required NuGet API key

**Hooks:** `PrePublish`, `PostPublish`

---

### Maven Central

Publish packages to Maven Central (Java).

**Installation:**
```bash
relicta plugin install maven
relicta plugin enable maven
```

**Configuration:**
```yaml
plugins:
  - name: maven
    enabled: true
    config:
      repository_url: "https://oss.sonatype.org/service/local/staging/deploy/maven2/"
      group_id: "com.example"      # Maven group ID
      artifact_id: "my-artifact"   # Maven artifact ID
      pom_path: "pom.xml"
```

**Environment Variables:**
- `MAVEN_USERNAME` - Maven repository username
- `MAVEN_PASSWORD` - Maven repository password
- `GPG_KEY_ID` - GPG key ID for signing (optional)

**Hooks:** `PrePublish`, `PostPublish`

---

### RubyGems

Publish gems to RubyGems.

**Installation:**
```bash
relicta plugin install rubygems
relicta plugin enable rubygems
```

**Configuration:**
```yaml
plugins:
  - name: rubygems
    enabled: true
    config:
      host: "https://rubygems.org"
      gemspec_path: ""             # Path to .gemspec (auto-detected)
      gem_path: ""                 # Path to built .gem file
```

**Environment Variables:**
- `GEM_HOST_API_KEY` - Required RubyGems API key

**Hooks:** `PrePublish`, `PostPublish`

---

### Hex.pm

Publish packages to Hex.pm (Elixir/Erlang).

**Installation:**
```bash
relicta plugin install hex
relicta plugin enable hex
```

**Configuration:**
```yaml
plugins:
  - name: hex
    enabled: true
    config:
      organization: ""             # Hex.pm organization (for private packages)
      replace: false               # Replace existing version
      mix_path: "mix.exs"
```

**Environment Variables:**
- `HEX_API_KEY` - Required Hex.pm API key

**Hooks:** `PrePublish`, `PostPublish`

---

### Packagist

Publish packages to Packagist (PHP).

**Installation:**
```bash
relicta plugin install packagist
relicta plugin enable packagist
```

**Configuration:**
```yaml
plugins:
  - name: packagist
    enabled: true
    config:
      package_name: "vendor/package"
      update_url: "https://packagist.org/api/update-package"
```

**Environment Variables:**
- `PACKAGIST_API_TOKEN` - Required Packagist API token
- `PACKAGIST_USERNAME` - Required Packagist username

**Hooks:** `PostPublish`

---

### Go Modules

Publish Go modules.

**Installation:**
```bash
relicta plugin install gomod
relicta plugin enable gomod
```

**Configuration:**
```yaml
plugins:
  - name: gomod
    enabled: true
    config:
      module_path: "github.com/user/repo"
      proxy_url: "https://proxy.golang.org"
      sumdb_url: "https://sum.golang.org"
      private: false               # Skip proxy notification for private modules
```

**Hooks:** `PrePublish`, `PostPublish`

---

## Project Management Plugins

### Jira

Create and link Jira release versions.

**Installation:**
```bash
relicta plugin install jira
relicta plugin enable jira
```

**Configuration:**
```yaml
plugins:
  - name: jira
    enabled: true
    config:
      url: "https://yourcompany.atlassian.net"
      project: "PROJ"              # Jira project key
      create_version: true         # Create version in Jira
      release_version: true        # Mark version as released
      version_prefix: "v"          # Prefix for version names
      transition_status: "Released"
```

**Environment Variables:**
- `JIRA_USERNAME` - Jira user email
- `JIRA_TOKEN` - Jira API token

**Hooks:** `PostPublish`

**Features:**
- Automatically detects Jira issue keys in commit messages (e.g., PROJ-123)
- Creates a version in Jira for the release
- Transitions issues to specified status
- Adds fix version to issues

---

### LaunchNotes

Create releases in LaunchNotes for customer-facing announcements.

**Installation:**
```bash
relicta plugin install launchnotes
relicta plugin enable launchnotes
```

**Configuration:**
```yaml
plugins:
  - name: launchnotes
    enabled: true
    config:
      project_id: "proj_123"
      auto_publish: false          # Auto-publish announcements
      categories:                  # Map commit types to categories
        feat: "new-features"
        fix: "bug-fixes"
```

**Environment Variables:**
- `LAUNCHNOTES_API_KEY` - Required API key

**Hooks:** `PostPublish`

---

## Container Plugins

### Docker

Build and push Docker images to container registries.

**Installation:**
```bash
relicta plugin install docker
relicta plugin enable docker
```

**Configuration:**
```yaml
plugins:
  - name: docker
    enabled: true
    config:
      registry: "docker.io"        # Container registry URL
      image: "username/myapp"      # Image name
      tags:                        # Tags (supports {{version}}, {{major}}, {{minor}}, {{patch}})
        - "{{version}}"
        - "{{major}}.{{minor}}"
        - "latest"
      dockerfile: "Dockerfile"
      context: "."
      platforms:                   # Multi-arch build targets
        - "linux/amd64"
        - "linux/arm64"
      build_args:
        BUILD_VERSION: "{{version}}"
      push: true                   # Push after building
      labels:
        org.opencontainers.image.source: "https://github.com/user/repo"
      cache_from: []               # Cache source images
      no_cache: false
      target: ""                   # Target build stage
```

**Environment Variables:**
- `DOCKER_USERNAME` - Registry username
- `DOCKER_PASSWORD` - Registry password or token

**Hooks:** `PrePublish`, `PostPublish`

---

## Creating Custom Plugins

Plugins are standalone binaries that communicate with Relicta via gRPC using the [Plugin SDK](https://github.com/relicta-tech/relicta-plugin-sdk).

### Using the Plugin SDK

```bash
# Create a new plugin
relicta plugin create my-plugin
cd my-plugin

# Build and test
go build -o my-plugin
relicta plugin test my-plugin
```

### Plugin Interface

```go
package main

import (
    "context"

    "github.com/relicta-tech/relicta-plugin-sdk/helpers"
    "github.com/relicta-tech/relicta-plugin-sdk/plugin"
)

type MyPlugin struct{}

func (p *MyPlugin) GetInfo() plugin.Info {
    return plugin.Info{
        Name:        "my-plugin",
        Version:     "1.0.0",
        Description: "My custom plugin",
        Author:      "Your Name",
        Hooks:       []plugin.Hook{plugin.HookPostPublish},
        ConfigSchema: `{
            "type": "object",
            "properties": {
                "api_key": {"type": "string", "description": "API key"}
            },
            "required": ["api_key"]
        }`,
    }
}

func (p *MyPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
    parser := helpers.NewConfigParser(req.Config)
    apiKey := parser.GetString("api_key", "MY_PLUGIN_API_KEY", "")

    if req.DryRun {
        return &plugin.ExecuteResponse{
            Success: true,
            Message: "Would execute plugin action",
        }, nil
    }

    // Plugin logic here
    return &plugin.ExecuteResponse{
        Success: true,
        Message: "Plugin executed successfully",
        Outputs: map[string]any{
            "result": "value",
        },
    }, nil
}

func (p *MyPlugin) Validate(ctx context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
    vb := helpers.NewValidationBuilder()
    parser := helpers.NewConfigParser(config)

    if parser.GetString("api_key", "MY_PLUGIN_API_KEY", "") == "" {
        vb.AddError("api_key", "API key is required")
    }

    return vb.Build(), nil
}

func main() {
    plugin.Serve(&MyPlugin{})
}
```

### Plugin Discovery

Relicta discovers plugins in:
1. Official registry (via `relicta plugin install`)
2. `~/.relicta/plugins/` (global)
3. `.relicta/plugins/` (project-local)

### Plugin SDK Resources

- [Plugin SDK Repository](https://github.com/relicta-tech/relicta-plugin-sdk)
- [Official Plugin Examples](https://github.com/relicta-tech/plugin-github)
- [gRPC Protocol](https://github.com/relicta-tech/relicta-plugin-sdk/tree/main/proto)

---

## Troubleshooting

### Plugin Not Found

```
Error: plugin 'github' not found
```

**Solution**: Install the plugin:
```bash
relicta plugin install github
relicta plugin enable github
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
1. Verify plugin binary is executable: `ls -la ~/.relicta/plugins/`
2. Check plugin version compatibility: `relicta plugin info npm`
3. Reinstall the plugin: `relicta plugin install npm --force`

### Plugin Version Mismatch

```
Error: plugin 'github' requires SDK version 2.0, got 1.0
```

**Solution**: Update the plugin:
```bash
relicta plugin update github
```

---

## Best Practices

1. **Use Dry Run** - Test plugin configurations before releasing
   ```bash
   relicta publish --dry-run
   ```

2. **Validate Config** - Check plugin configuration
   ```bash
   relicta validate
   ```

3. **Environment Variables** - Use environment variables for secrets
   ```yaml
   # Good - secrets via environment
   plugins:
     - name: github
       enabled: true

   # Bad - don't commit tokens!
   plugins:
     - name: github
       config:
         token: "ghp_secret123"
   ```

4. **Error Notifications** - Configure notification plugins for error handling
   ```yaml
   plugins:
     - name: slack
       enabled: true
       config:
         notify_on_success: true
         notify_on_error: true  # Get notified of failures
   ```

5. **Incremental Rollout** - Enable plugins one at a time to identify issues

6. **Plugin Versioning** - Pin plugin versions for reproducible releases
   ```bash
   relicta plugin install github@v2.0.0
   ```
