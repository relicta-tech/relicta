# Video Tutorial 5: Custom Plugin Development with SDK

**Duration:** 8-10 minutes
**Demo Repo:** https://github.com/relicta-tech/relicta-demo

## Overview

Build a custom Relicta plugin using the official SDK. This tutorial walks through creating, testing, and publishing your own plugin.

## Prerequisites

- Go 1.22+
- Relicta installed
- Basic Go knowledge

## Script

### Scene 1: Introduction (30s)

```
"Sometimes you need an integration that doesn't exist yet.
Maybe it's an internal tool, a niche platform, or a custom workflow.

Relicta's Plugin SDK makes it easy to build your own plugins.
Let's create one from scratch."
```

### Scene 2: Using the Plugin Generator (1 min)

```bash
# Create a new plugin project
relicta plugin create my-notifier --author "Your Name"

cd my-notifier
ls -la
```

**Show generated files:**
```
my-notifier/
├── main.go              # Plugin entry point
├── plugin.go            # Implementation skeleton
├── go.mod               # Module definition with SDK
├── README.md            # Documentation template
└── .github/workflows/   # CI/CD templates
```

**Narration:**
```
"The plugin generator creates a complete project structure
with all the boilerplate handled for you."
```

### Scene 3: Understanding the Plugin Interface (1.5 min)

```go
// plugin.go - The core interface
type Plugin interface {
    // Plugin metadata
    GetInfo() *PluginInfo

    // Configuration validation
    Validate(config map[string]interface{}) *ValidationResult

    // Lifecycle hooks - implement what you need
    PreVersion(ctx context.Context, info *ReleaseInfo) error
    PostNotes(ctx context.Context, info *ReleaseInfo, notes string) error
    PostPublish(ctx context.Context, info *ReleaseInfo, url string) error
    OnSuccess(ctx context.Context, info *ReleaseInfo) error
    OnError(ctx context.Context, info *ReleaseInfo, err error) error
}
```

**Narration:**
```
"Every plugin implements this interface.
You only need to implement the hooks you care about -
the SDK provides sensible defaults for the rest."
```

### Scene 4: Implementing GetInfo (45s)

```go
func (p *MyNotifierPlugin) GetInfo() *plugin.PluginInfo {
    return &plugin.PluginInfo{
        Name:        "my-notifier",
        Version:     "1.0.0",
        Description: "Sends release notifications to my custom service",
        Author:      "Your Name",
        Homepage:    "https://github.com/yourname/relicta-plugin-my-notifier",
        Hooks: []string{
            "post-publish",
            "on-success",
            "on-error",
        },
        ConfigSchema: map[string]plugin.ConfigField{
            "api_url": {
                Type:        "string",
                Required:    true,
                Description: "API endpoint URL",
                EnvVar:      "MY_NOTIFIER_API_URL",
            },
            "api_key": {
                Type:        "string",
                Required:    true,
                Description: "API authentication key",
                EnvVar:      "MY_NOTIFIER_API_KEY",
                Secret:      true,
            },
        },
    }
}
```

### Scene 5: Implementing Validate (1 min)

```go
func (p *MyNotifierPlugin) Validate(raw map[string]interface{}) *plugin.ValidationResult {
    parser := helpers.NewConfigParser(raw)
    vb := helpers.NewValidationBuilder()

    // Get config values with environment fallbacks
    apiURL := parser.GetString("api_url", "MY_NOTIFIER_API_URL", "")
    apiKey := parser.GetString("api_key", "MY_NOTIFIER_API_KEY", "")

    // Validate required fields
    if apiURL == "" {
        vb.AddError("api_url", "API URL is required")
    }
    if apiKey == "" {
        vb.AddError("api_key", "API key is required")
    }

    // Store for later use
    p.apiURL = apiURL
    p.apiKey = apiKey

    return vb.Build()
}
```

**Narration:**
```
"The Validate method ensures configuration is correct before
the plugin runs. The helpers package makes it easy to parse
config and check for required fields."
```

### Scene 6: Implementing PostPublish Hook (1.5 min)

```go
func (p *MyNotifierPlugin) PostPublish(
    ctx context.Context,
    info *plugin.ReleaseInfo,
    url string,
) error {
    // Build notification payload
    payload := map[string]interface{}{
        "version":     info.Version,
        "release_url": url,
        "repository":  info.Repository,
        "timestamp":   time.Now().Format(time.RFC3339),
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("failed to marshal payload: %w", err)
    }

    // Send to API
    req, err := http.NewRequestWithContext(
        ctx, "POST", p.apiURL, bytes.NewReader(body))
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+p.apiKey)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return fmt.Errorf("failed to send notification: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return fmt.Errorf("API returned error: %d", resp.StatusCode)
    }

    return nil
}
```

### Scene 7: Building and Testing (1 min)

```bash
# Build the plugin
go build -o my-notifier .

# Install locally for testing
relicta plugin install ./my-notifier
relicta plugin enable my-notifier

# Test with dry-run
relicta publish --dry-run
```

**Show plugin loading in output:**
```
plugin: plugin loaded: name=my-notifier version=1.0.0 hooks=["post-publish"]
```

### Scene 8: Development Mode (45s)

```bash
# Watch mode - auto-rebuilds on changes
relicta plugin dev --watch

# In another terminal, edit your code
# Changes are automatically rebuilt and reloaded
```

**Narration:**
```
"Development mode with --watch makes iteration fast.
Every time you save a file, the plugin is rebuilt
and reinstalled automatically."
```

### Scene 9: Publishing Your Plugin (1 min)

```bash
# Initialize git and push
git init
git add .
git commit -m "feat: initial release"
git remote add origin https://github.com/yourname/relicta-plugin-my-notifier.git
git push -u origin main

# Create a release
git tag v1.0.0
git push origin v1.0.0
```

**Show GitHub release workflow triggering and creating assets**

**Narration:**
```
"Once you push a tag, the CI workflow builds your plugin
for all platforms and creates a GitHub release.
Others can now install it with relicta plugin install!"
```

### Scene 10: Summary (30s)

```
"You've learned how to:
- Generate a plugin project with the CLI
- Implement the Plugin interface
- Validate configuration
- Hook into the release lifecycle
- Test and develop locally
- Publish for others to use

The Plugin SDK opens up endless possibilities
for customizing your release workflow.

Thanks for watching the Relicta tutorial series!"
```

## Complete Plugin Example

```go
package main

import (
    "context"
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/relicta-tech/relicta-plugin-sdk/plugin"
    "github.com/relicta-tech/relicta-plugin-sdk/helpers"
)

func main() {
    plugin.Serve(&MyNotifierPlugin{})
}

type MyNotifierPlugin struct {
    apiURL string
    apiKey string
}

func (p *MyNotifierPlugin) GetInfo() *plugin.PluginInfo {
    return &plugin.PluginInfo{
        Name:        "my-notifier",
        Version:     "1.0.0",
        Description: "Custom notification service integration",
        Author:      "Your Name",
        Hooks:       []string{"post-publish", "on-success", "on-error"},
        ConfigSchema: map[string]plugin.ConfigField{
            "api_url": {Type: "string", Required: true, EnvVar: "MY_NOTIFIER_API_URL"},
            "api_key": {Type: "string", Required: true, EnvVar: "MY_NOTIFIER_API_KEY", Secret: true},
        },
    }
}

func (p *MyNotifierPlugin) Validate(raw map[string]interface{}) *plugin.ValidationResult {
    parser := helpers.NewConfigParser(raw)
    vb := helpers.NewValidationBuilder()

    p.apiURL = parser.GetString("api_url", "MY_NOTIFIER_API_URL", "")
    p.apiKey = parser.GetString("api_key", "MY_NOTIFIER_API_KEY", "")

    if p.apiURL == "" {
        vb.AddError("api_url", "required")
    }
    if p.apiKey == "" {
        vb.AddError("api_key", "required")
    }

    return vb.Build()
}

func (p *MyNotifierPlugin) PostPublish(ctx context.Context, info *plugin.ReleaseInfo, url string) error {
    payload, _ := json.Marshal(map[string]string{
        "version": info.Version,
        "url":     url,
    })

    req, _ := http.NewRequestWithContext(ctx, "POST", p.apiURL, bytes.NewReader(payload))
    req.Header.Set("Authorization", "Bearer "+p.apiKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return fmt.Errorf("API error: %d", resp.StatusCode)
    }
    return nil
}

// Implement other hooks as needed...
func (p *MyNotifierPlugin) PreVersion(ctx context.Context, info *plugin.ReleaseInfo) error { return nil }
func (p *MyNotifierPlugin) PostNotes(ctx context.Context, info *plugin.ReleaseInfo, notes string) error { return nil }
func (p *MyNotifierPlugin) OnSuccess(ctx context.Context, info *plugin.ReleaseInfo) error { return nil }
func (p *MyNotifierPlugin) OnError(ctx context.Context, info *plugin.ReleaseInfo, err error) error { return nil }
```

## End Card

- SDK docs: docs.relicta.tech/sdk
- Plugin registry: docs.relicta.tech/plugins
- GitHub: github.com/relicta-tech/relicta
- Thank you for watching!
