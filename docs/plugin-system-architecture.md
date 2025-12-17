# Plugin System Architecture Design

**Version:** 1.0
**Date:** 2025-12-17
**Status:** Design Proposal

---

## Executive Summary

This document proposes a comprehensive redesign of Relicta's plugin system to address current architectural inconsistencies, improve developer experience, and establish a sustainable foundation for plugin ecosystem growth. The design draws from industry best practices (HashiCorp, Kubernetes, Homebrew) while maintaining Relicta's security and simplicity requirements.

### Current Problems

1. **Two Separate "Managers"** - Runtime plugin loading (`internal/plugin/manager.go`) and CLI plugin management (`internal/plugin/manager/`) don't integrate well
2. **No Plugin SDK** - Authors must reverse-engineer examples; no standardized development kit
3. **Inconsistent Naming** - Plugin binary names, discovery, and versioning lack conventions
4. **Missing Metadata** - No standardized plugin metadata, versioning, or compatibility checking
5. **No Distribution Strategy** - Unclear how plugins should be distributed, versioned, and discovered
6. **Weak Security Model** - Path validation exists but no checksums, signatures, or provenance

---

## Design Principles

| Principle | Rationale |
|-----------|-----------|
| **Plugin SDK First** | Plugin authors should have a clear, documented SDK to build against |
| **Secure by Default** | All plugins verified via checksums; isolated execution via go-plugin |
| **Backward Compatibility** | Versioned protocol allows host-plugin compatibility checking |
| **Developer Experience** | Simple local development with `relicta plugin dev` support |
| **Distribution Neutral** | Support multiple distribution channels (registry, local, git) |
| **Observable & Debuggable** | Built-in logging, tracing, and error reporting for plugin execution |

---

## 1. Plugin SDK

### 1.1 SDK Structure

```
github.com/relicta-tech/relicta-sdk-go/
├── plugin/                   # Core plugin interface (re-export from pkg/plugin)
│   ├── interface.go         # Plugin interface
│   ├── types.go             # Core types
│   └── helpers.go           # Helper utilities
├── testing/                 # Testing utilities
│   ├── mock.go              # Mock implementations
│   └── fixture.go           # Test fixtures
├── examples/                # Example plugins
│   ├── hello/              # Minimal example
│   ├── notification/       # Notification plugin example
│   └── publisher/          # Publishing plugin example
├── template/               # Plugin template generator
│   └── generate.go         # Template generator
└── tools/                  # Development tools
    ├── validate.go         # Config validator
    └── build.go            # Build helper
```

### 1.2 SDK Package (What Plugin Authors Import)

**Recommended Import Path:**
```go
import (
    "github.com/relicta-tech/relicta/pkg/plugin"
)
```

**Current State:** ✅ Already well-designed in `pkg/plugin/`
- Clean interface definition
- Comprehensive types (ReleaseContext, ExecuteRequest, etc.)
- Hook lifecycle well-defined

**Enhancement Needed:** Documentation and helper utilities

```go
// pkg/plugin/helpers.go - NEW

package plugin

import (
    "fmt"
    "os"
    "path/filepath"
)

// ConfigParser helps parse plugin configuration safely
type ConfigParser struct {
    raw map[string]any
}

// NewConfigParser creates a config parser
func NewConfigParser(raw map[string]any) *ConfigParser {
    return &ConfigParser{raw: raw}
}

// GetString safely retrieves a string config value with env var fallback
func (p *ConfigParser) GetString(key string, envVars ...string) string {
    if v, ok := p.raw[key].(string); ok && v != "" {
        return v
    }
    for _, envVar := range envVars {
        if v := os.Getenv(envVar); v != "" {
            return v
        }
    }
    return ""
}

// GetBool safely retrieves a boolean config value
func (p *ConfigParser) GetBool(key string) bool {
    if v, ok := p.raw[key].(bool); ok {
        return v
    }
    return false
}

// GetStringSlice safely retrieves a string slice
func (p *ConfigParser) GetStringSlice(key string) []string {
    if v, ok := p.raw[key].([]any); ok {
        result := make([]string, 0, len(v))
        for _, item := range v {
            if s, ok := item.(string); ok {
                result = append(result, s)
            }
        }
        return result
    }
    return nil
}

// ValidationBuilder helps build validation responses
type ValidationBuilder struct {
    errors []ValidationError
}

// NewValidationBuilder creates a validation builder
func NewValidationBuilder() *ValidationBuilder {
    return &ValidationBuilder{errors: []ValidationError{}}
}

// AddError adds a validation error
func (vb *ValidationBuilder) AddError(field, message, code string) {
    vb.errors = append(vb.errors, ValidationError{
        Field:   field,
        Message: message,
        Code:    code,
    })
}

// ValidateRequired checks if a required field exists
func (vb *ValidationBuilder) ValidateRequired(config map[string]any, field string) {
    if _, ok := config[field]; !ok {
        vb.AddError(field, fmt.Sprintf("%s is required", field), "required")
    }
}

// ValidateStringSlice validates a string slice field
func (vb *ValidationBuilder) ValidateStringSlice(config map[string]any, field string) {
    if v, ok := config[field]; ok {
        if _, ok := v.([]any); !ok {
            vb.AddError(field, fmt.Sprintf("%s must be an array", field), "invalid_type")
        }
    }
}

// Build returns the validation response
func (vb *ValidationBuilder) Build() *ValidateResponse {
    return &ValidateResponse{
        Valid:  len(vb.errors) == 0,
        Errors: vb.errors,
    }
}

// ValidateAssetPath validates and sanitizes an asset path
func ValidateAssetPath(assetPath string) (string, error) {
    // Get current working directory
    cwd, err := os.Getwd()
    if err != nil {
        return "", fmt.Errorf("failed to get working directory: %w", err)
    }

    // Resolve to absolute path
    absPath := assetPath
    if !filepath.IsAbs(assetPath) {
        absPath = filepath.Join(cwd, assetPath)
    }

    // Resolve symlinks and clean path
    realPath, err := filepath.EvalSymlinks(absPath)
    if err != nil {
        return "", fmt.Errorf("failed to resolve path: %w", err)
    }

    // Ensure the resolved path is within the working directory
    rel, err := filepath.Rel(cwd, realPath)
    if err != nil {
        return "", fmt.Errorf("failed to get relative path: %w", err)
    }

    // Check for path traversal
    if filepath.IsAbs(rel) || len(rel) > 0 && rel[0] == '.' && len(rel) > 1 && rel[1] == '.' {
        return "", fmt.Errorf("path traversal detected: %s", assetPath)
    }

    return realPath, nil
}
```

### 1.3 Plugin Template Generator

**Command:** `relicta plugin create <name>`

```bash
$ relicta plugin create my-notifier
Creating plugin: my-notifier
✓ Created directory: my-notifier/
✓ Generated main.go
✓ Generated plugin.go
✓ Generated go.mod
✓ Generated README.md
✓ Generated .goreleaser.yaml

Next steps:
  cd my-notifier
  go mod tidy
  relicta plugin dev
```

**Generated Structure:**

```
my-notifier/
├── main.go              # Plugin entry point
├── plugin.go            # Plugin implementation
├── go.mod               # Go module
├── README.md            # Documentation template
├── .goreleaser.yaml     # Release configuration
└── .relicta-plugin.yaml # Plugin metadata
```

---

## 2. Plugin Protocol & Lifecycle

### 2.1 Protocol Versioning

**Current:** Uses HashiCorp go-plugin with gRPC (✅ Good foundation)

**Enhancement:** Add protocol version negotiation

```go
// pkg/plugin/protocol.go - NEW

package plugin

const (
    // ProtocolVersion is the current plugin protocol version
    // Increment for breaking changes to plugin interface
    ProtocolVersion = 1

    // MinCompatibleVersion is the minimum protocol version we support
    MinCompatibleVersion = 1
)

// ProtocolInfo describes the protocol capabilities
type ProtocolInfo struct {
    Version            int      `json:"version"`
    MinCompatibleVersion int    `json:"min_compatible_version"`
    SupportedHooks     []Hook   `json:"supported_hooks"`
}

// IsCompatible checks if two protocol versions are compatible
func IsCompatible(hostVersion, pluginVersion int) bool {
    return pluginVersion >= MinCompatibleVersion &&
           pluginVersion <= ProtocolVersion
}
```

### 2.2 Plugin Metadata Standard

**File:** `.relicta-plugin.yaml` (in plugin repository root)

```yaml
# .relicta-plugin.yaml
name: github
version: 1.2.0
protocol_version: 1
description: Create GitHub releases and upload assets
author: Relicta Team
homepage: https://github.com/relicta-tech/plugin-github
license: MIT

# Minimum relicta version required
relicta_min_version: 2.0.0

# Dependencies on other plugins (optional)
dependencies:
  - name: git
    version: ^1.0.0

# Supported hooks
hooks:
  - post-publish
  - on-success
  - on-error

# Configuration schema (JSON Schema)
config_schema:
  type: object
  properties:
    owner:
      type: string
      description: Repository owner
    repo:
      type: string
      description: Repository name
    token:
      type: string
      description: GitHub token (or use GITHUB_TOKEN env)
    draft:
      type: boolean
      default: false
    assets:
      type: array
      items:
        type: string

# Platforms supported
platforms:
  - linux/amd64
  - linux/arm64
  - darwin/amd64
  - darwin/arm64
  - windows/amd64

# Installation instructions (optional)
install:
  # How to install this plugin
  registry: official  # or: community, git, local
  # For git-based plugins:
  # git: https://github.com/relicta-tech/plugin-github
```

### 2.3 Plugin Lifecycle Hooks

**Current:** ✅ Well-defined in `pkg/plugin/interface.go`

```
Lifecycle:
  PreInit → PostInit →
  PrePlan → PostPlan →
  PreVersion → PostVersion →
  PreNotes → PostNotes →
  PreApprove → PostApprove →
  PrePublish → PostPublish →
  OnSuccess / OnError
```

**No changes needed** - current hook design is excellent.

---

## 3. Plugin Discovery & Naming

### 3.1 Binary Naming Convention

**Standard:** `<name>` (without `relicta-plugin-` prefix)

**Examples:**
- ✅ `github` (not `relicta-plugin-github`)
- ✅ `slack` (not `relicta-plugin-slack`)
- ✅ `npm` (not `relicta-plugin-npm`)

**Rationale:** Simpler names, consistent with kubectl plugin conventions

### 3.2 Discovery Locations (Priority Order)

```
1. Project-local:        .relicta/plugins/<name>
2. User-level:           ~/.relicta/plugins/<name>
3. System-level:         /usr/local/lib/relicta/plugins/<name>
                         /usr/lib/relicta/plugins/<name>
```

**Current Implementation:** ✅ Already follows this pattern in `internal/plugin/manager.go`

### 3.3 Plugin Resolution Algorithm

```
For plugin "github":
  1. Check config for explicit path: plugins[].path
  2. If not found, search in discovery locations
  3. Validate binary (security checks)
  4. Verify protocol compatibility
  5. Load and cache metadata
```

---

## 4. Plugin Distribution & Registry

### 4.1 Distribution Channels

| Channel | Use Case | Trust Model |
|---------|----------|-------------|
| **Official Registry** | Relicta-maintained plugins | High - signed by Relicta |
| **Community Registry** | Third-party verified plugins | Medium - community review |
| **Git Repository** | Development, custom plugins | Low - user responsibility |
| **Local Path** | Development, testing | Low - user responsibility |

### 4.2 Registry Structure

**Registry File:** `https://registry.relicta.tech/plugins.json` (or GitHub-hosted)

```json
{
  "version": "1.0",
  "updated_at": "2025-12-17T10:00:00Z",
  "plugins": [
    {
      "name": "github",
      "version": "1.2.0",
      "protocol_version": 1,
      "description": "Create GitHub releases",
      "category": "publisher",
      "tags": ["github", "release", "official"],
      "homepage": "https://github.com/relicta-tech/plugin-github",
      "repository": "https://github.com/relicta-tech/plugin-github",
      "license": "MIT",
      "author": "Relicta Team",

      "platforms": {
        "linux/amd64": {
          "url": "https://github.com/relicta-tech/plugin-github/releases/download/v1.2.0/github_1.2.0_linux_amd64.tar.gz",
          "checksum": "sha256:abc123...",
          "size": 5242880
        },
        "darwin/amd64": {
          "url": "https://github.com/relicta-tech/plugin-github/releases/download/v1.2.0/github_1.2.0_darwin_amd64.tar.gz",
          "checksum": "sha256:def456...",
          "size": 5500000
        }
      },

      "relicta_min_version": "2.0.0",
      "dependencies": [],

      "verified": true,
      "signature": "base64-encoded-signature"
    }
  ]
}
```

**Current State:** ⚠️ Partial - registry fetching exists in `internal/plugin/manager/registry.go` but needs enhancement

### 4.3 Plugin Installation Flow

```
relicta plugin install github
  ↓
1. Fetch registry (cache for 24h)
  ↓
2. Find plugin "github" in registry
  ↓
3. Check relicta version compatibility
  ↓
4. Download binary for current platform
  ↓
5. Verify checksum (MUST match)
  ↓
6. Verify signature (if available)
  ↓
7. Extract to ~/.relicta/plugins/github
  ↓
8. Validate protocol compatibility
  ↓
9. Update manifest (~/.relicta/plugins/manifest.yaml)
  ↓
10. Mark as enabled (or prompt user)
```

### 4.4 Manifest File

**Location:** `~/.relicta/plugins/manifest.yaml`

**Current Implementation:** ✅ Exists in `internal/plugin/manager/types.go`

**Enhancement:** Add checksum and signature tracking

```yaml
version: "1.0"
updated_at: 2025-12-17T10:30:00Z

installed:
  - name: github
    version: 1.2.0
    protocol_version: 1
    enabled: true
    installed_at: 2025-12-15T14:20:00Z
    source: registry  # or: git, local
    path: /Users/user/.relicta/plugins/github
    checksum: sha256:abc123...
    # signature: <base64>  # Optional

  - name: slack
    version: 0.9.0
    protocol_version: 1
    enabled: false
    installed_at: 2025-12-16T09:15:00Z
    source: git
    repository: https://github.com/my-org/relicta-slack
    path: /Users/user/.relicta/plugins/slack
    checksum: sha256:xyz789...
```

---

## 5. Plugin Configuration

### 5.1 Configuration Schema

**Current:** ✅ Plugins already return JSON Schema in `GetInfo().ConfigSchema`

**Enhancement:** Add schema validation helper

```go
// pkg/plugin/config.go - NEW

package plugin

import (
    "encoding/json"
    "fmt"

    "github.com/xeipuuv/gojsonschema"
)

// ValidateConfigAgainstSchema validates config against JSON schema
func ValidateConfigAgainstSchema(config map[string]any, schemaJSON string) error {
    if schemaJSON == "" {
        return nil // No schema = no validation
    }

    schemaLoader := gojsonschema.NewStringLoader(schemaJSON)
    configJSON, _ := json.Marshal(config)
    documentLoader := gojsonschema.NewBytesLoader(configJSON)

    result, err := gojsonschema.Validate(schemaLoader, documentLoader)
    if err != nil {
        return fmt.Errorf("schema validation error: %w", err)
    }

    if !result.Valid() {
        var errs []string
        for _, err := range result.Errors() {
            errs = append(errs, err.String())
        }
        return fmt.Errorf("config validation failed: %v", errs)
    }

    return nil
}
```

### 5.2 User Configuration

**Location:** `release.config.yaml`

**Current:** ✅ Already well-designed

```yaml
plugins:
  - name: github
    config:
      draft: false
      assets:
        - dist/*.tar.gz
        - dist/*.zip
    timeout: 60s
    continue_on_error: false

  - name: slack
    enabled: false  # Installed but not active
    config:
      webhook: ${SLACK_WEBHOOK}
      channel: "#releases"
```

---

## 6. Host-Plugin Communication

### 6.1 Current Architecture

```
Relicta CLI (Host)
      ↓
  go-plugin (gRPC)
      ↓
  Plugin Process (Separate Binary)
```

**Current:** ✅ Secure isolation via go-plugin

### 6.2 Data Flow

```
Host                          Plugin
  |                              |
  |-- ExecuteRequest ----------->|
  |    (Hook, Config, Context)   |
  |                              |
  |                         Execute()
  |                              |
  |<---------- ExecuteResponse --|
  |    (Success, Message, Error) |
```

### 6.3 Error Handling Strategy

**Principle:** Fail gracefully, continue if possible

```go
// internal/plugin/manager.go - Current approach ✅

func (m *Manager) ExecuteHook(ctx context.Context, hook plugin.Hook, releaseCtx plugin.ReleaseContext) ([]plugin.ExecuteResponse, error) {
    // Parallel execution with errgroup
    // Individual plugin failures don't stop others
    // Respect continue_on_error configuration
}
```

**Enhancement:** Add structured error types

```go
// pkg/plugin/errors.go - NEW

package plugin

type ErrorType string

const (
    ErrorTypeConfig     ErrorType = "config"      // Configuration error
    ErrorTypeAuth       ErrorType = "auth"        // Authentication failed
    ErrorTypeNetwork    ErrorType = "network"     // Network/API error
    ErrorTypeValidation ErrorType = "validation"  // Data validation error
    ErrorTypeInternal   ErrorType = "internal"    // Internal plugin error
)

type ExecutionError struct {
    Type    ErrorType `json:"type"`
    Message string    `json:"message"`
    Code    string    `json:"code,omitempty"`
    Details any       `json:"details,omitempty"`
}
```

### 6.4 Logging & Observability

**Current:** ✅ Uses hclog with per-plugin namespacing

**Enhancement:** Add structured execution tracking

```go
// internal/plugin/telemetry.go - NEW

type ExecutionMetrics struct {
    PluginName string
    Hook       Hook
    StartTime  time.Time
    Duration   time.Duration
    Success    bool
    Error      string
}

// Track execution metrics for observability
func (m *Manager) recordExecution(metrics ExecutionMetrics) {
    m.logger.Info("plugin execution completed",
        "plugin", metrics.PluginName,
        "hook", metrics.Hook,
        "duration_ms", metrics.Duration.Milliseconds(),
        "success", metrics.Success,
    )
}
```

---

## 7. Security Model

### 7.1 Current Security Features ✅

1. **Path Validation** - Only load from approved directories
2. **Process Isolation** - Plugins run as separate processes
3. **gRPC Communication** - Structured, type-safe communication
4. **Symlink Resolution** - Prevent path traversal

### 7.2 Security Enhancements

**Add Checksum Verification:**

```go
// internal/plugin/security.go - NEW

func verifyChecksum(pluginPath, expectedChecksum string) error {
    file, err := os.Open(pluginPath)
    if err != nil {
        return err
    }
    defer file.Close()

    h := sha256.New()
    if _, err := io.Copy(h, file); err != nil {
        return err
    }

    actualChecksum := fmt.Sprintf("sha256:%x", h.Sum(nil))
    if actualChecksum != expectedChecksum {
        return fmt.Errorf("checksum mismatch: expected %s, got %s",
            expectedChecksum, actualChecksum)
    }

    return nil
}
```

**Add Signature Verification (Optional):**

```go
// Verify plugin signature using Relicta's public key
func verifySignature(pluginPath, signature, publicKeyPath string) error {
    // Use crypto/ed25519 or similar
    // Verify that plugin binary was signed by trusted key
}
```

### 7.3 Security Checklist

- [x] Path traversal prevention
- [x] Process isolation (go-plugin)
- [x] Plugin binary validation
- [ ] Checksum verification on install
- [ ] Signature verification (optional)
- [ ] Plugin capability restrictions (future)
- [ ] Audit logging of plugin actions

---

## 8. Developer Experience

### 8.1 Plugin Development Workflow

```bash
# 1. Create plugin from template
relicta plugin create my-notifier

# 2. Implement plugin
cd my-notifier
vim plugin.go

# 3. Test locally with live reload
relicta plugin dev --watch

# 4. Build for release
goreleaser build --snapshot

# 5. Test installation
relicta plugin install --path ./dist/my-notifier_linux_amd64

# 6. Release to community
relicta plugin publish --registry community
```

### 8.2 Plugin Testing Utilities

```go
// pkg/plugin/testing/mock.go - NEW

package testing

import "context"

// MockPlugin implements Plugin for testing
type MockPlugin struct {
    InfoFunc     func() plugin.Info
    ExecuteFunc  func(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error)
    ValidateFunc func(ctx context.Context, config map[string]any) (*plugin.ValidateResponse, error)
}

func (m *MockPlugin) GetInfo() plugin.Info {
    if m.InfoFunc != nil {
        return m.InfoFunc()
    }
    return plugin.Info{Name: "mock"}
}

// ... rest of mock implementation
```

### 8.3 Plugin Documentation Template

**Generated README.md:**

```markdown
# My Notifier Plugin

Sends release notifications to custom webhook endpoints.

## Installation

```bash
relicta plugin install my-notifier
```

## Configuration

```yaml
plugins:
  - name: my-notifier
    config:
      webhook_url: https://example.com/webhook
      format: json
```

## Supported Hooks

- `post-publish` - Send notification after successful publish
- `on-error` - Send alert on release failure

## Configuration Schema

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `webhook_url` | string | yes | Webhook endpoint URL |
| `format` | string | no | Payload format (json, text) |

## Development

```bash
# Run in development mode
relicta plugin dev --watch

# Build
make build

# Test
make test
```
```

---

## 9. Implementation Roadmap

### Phase 1: Foundation (Week 1-2)

**Goal:** Establish SDK and improve existing code

- [ ] Create `pkg/plugin/helpers.go` with ConfigParser, ValidationBuilder
- [ ] Add `pkg/plugin/protocol.go` with versioning support
- [ ] Enhance manifest with checksum tracking
- [ ] Add checksum verification to installer
- [ ] Document plugin development guide

### Phase 2: Unification (Week 3-4)

**Goal:** Merge runtime and CLI managers

- [ ] Refactor `internal/plugin/manager.go` to use manifest from `manager/`
- [ ] Consolidate plugin discovery logic
- [ ] Add protocol compatibility checking
- [ ] Improve error handling with structured types

### Phase 3: Developer Tools (Week 5-6)

**Goal:** Improve plugin author experience

- [ ] Create `relicta plugin create` template generator
- [ ] Add `relicta plugin dev` development mode
- [ ] Create testing utilities in SDK
- [ ] Write comprehensive plugin development guide

### Phase 4: Distribution (Week 7-8)

**Goal:** Establish plugin registry and distribution

- [ ] Design registry format and API
- [ ] Implement registry client in installer
- [ ] Add `relicta plugin search`, `relicta plugin install <name>`
- [ ] Add signature verification (optional but recommended)

### Phase 5: Ecosystem (Week 9-10)

**Goal:** Grow plugin ecosystem

- [ ] Migrate all official plugins to new standards
- [ ] Create plugin submission process
- [ ] Publish registry with initial plugins
- [ ] Launch plugin marketplace website

---

## 10. Recommended Changes Summary

### High Priority (Must Have)

1. **Create Plugin SDK Helpers** (`pkg/plugin/helpers.go`)
   - ConfigParser for safe config reading
   - ValidationBuilder for validation responses
   - ValidateAssetPath for security

2. **Add Protocol Versioning** (`pkg/plugin/protocol.go`)
   - Define ProtocolVersion constant
   - Add compatibility checking
   - Include in handshake

3. **Enhance Manifest** (`internal/plugin/manager/types.go`)
   - Add checksum field
   - Track protocol_version
   - Add source field (registry/git/local)

4. **Add Checksum Verification** (`internal/plugin/manager/installer.go`)
   - Verify checksums on install
   - Store checksums in manifest
   - Validate on load

5. **Unify Plugin Managers**
   - Merge runtime and CLI concerns
   - Single source of truth for installed plugins
   - Shared plugin discovery logic

### Medium Priority (Should Have)

6. **Plugin Template Generator** (`cmd/relicta/plugin_create.go`)
   - Generate boilerplate with `relicta plugin create`
   - Include all best practices
   - Ready-to-publish structure

7. **Enhanced Registry** (`internal/plugin/manager/registry.go`)
   - Support multiple registries
   - Add plugin search/filter
   - Include plugin categories/tags

8. **Development Mode** (`cmd/relicta/plugin_dev.go`)
   - `relicta plugin dev --watch` for local testing
   - Hot reload on plugin changes
   - Detailed execution logging

9. **Plugin Documentation**
   - SDK documentation
   - Best practices guide
   - Example plugins repository

### Low Priority (Nice to Have)

10. **Signature Verification**
    - Sign official plugins
    - Verify signatures on install
    - Trust model for community plugins

11. **Dependency Management**
    - Track plugin dependencies
    - Automatic dependency installation
    - Dependency conflict detection

12. **Plugin Marketplace**
    - Web interface for browsing plugins
    - Plugin ratings/reviews
    - Usage statistics

---

## 11. Breaking Changes & Migration

### Breaking Changes

None - all changes are additive and backward compatible.

**Existing plugins continue to work** without modification.

### Migration Path for Plugin Authors

**Old (still works):**
```go
import "github.com/relicta-tech/relicta/pkg/plugin"

type MyPlugin struct{}

func (p *MyPlugin) GetInfo() plugin.Info { ... }
func (p *MyPlugin) Execute(...) { ... }
func (p *MyPlugin) Validate(...) { ... }
```

**New (recommended):**
```go
import "github.com/relicta-tech/relicta/pkg/plugin"

type MyPlugin struct{}

func (p *MyPlugin) GetInfo() plugin.Info { ... }

func (p *MyPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
    // Use new helpers
    parser := plugin.NewConfigParser(req.Config)
    webhookURL := parser.GetString("webhook_url", "WEBHOOK_URL")

    // ... rest of implementation
}

func (p *MyPlugin) Validate(ctx context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
    vb := plugin.NewValidationBuilder()
    vb.ValidateRequired(config, "webhook_url")
    return vb.Build(), nil
}
```

### Migration Timeline

- **v2.0.0** - Add SDK helpers, protocol versioning (non-breaking)
- **v2.1.0** - Add plugin template generator, dev mode
- **v2.2.0** - Launch plugin registry
- **v3.0.0** - Deprecate old patterns (if needed)

---

## 12. Comparison with Industry Standards

### HashiCorp go-plugin ✅

**What We Adopt:**
- gRPC-based plugin system
- Process isolation
- Structured RPC protocol

**What We Add:**
- SDK helpers for plugin authors
- Registry and distribution
- Template generator

### Kubernetes kubectl Plugins

**What We Adopt:**
- Simple binary naming (`<name>` not `kubectl-<name>`)
- PATH-based discovery
- Minimal overhead

**What We Skip:**
- kubectl uses PATH search (security risk)
- We restrict to specific directories

### Homebrew Formula Pattern

**What We Adopt:**
- Registry-based distribution
- Checksum verification
- Version management

**What We Add:**
- Protocol versioning
- Hook-based lifecycle
- gRPC instead of shell

---

## 13. Success Metrics

| Metric | Target | Timeline |
|--------|--------|----------|
| Official plugins migrated to new SDK | 100% | Month 1 |
| Plugin development guide published | 1.0 | Month 1 |
| Community plugins using SDK | 3+ | Month 2 |
| Plugin registry operational | ✅ | Month 2 |
| Plugin template downloads | 50+ | Month 3 |
| Plugin marketplace launched | ✅ | Month 3 |

---

## 14. Open Questions

1. **Registry Hosting:** GitHub-hosted JSON file vs. dedicated API?
   - **Recommendation:** Start with GitHub-hosted, migrate to API later

2. **Signature Verification:** Required or optional?
   - **Recommendation:** Optional for community, required for official

3. **Plugin Versioning:** Semantic versioning enforcement?
   - **Recommendation:** Yes - validate semver in registry

4. **Breaking Changes:** How to handle protocol version bumps?
   - **Recommendation:** Min/max compatibility range

5. **Plugin Dependencies:** Support or defer?
   - **Recommendation:** Defer to v3.0 - adds significant complexity

---

## 15. References

### Industry Examples Studied

- **HashiCorp go-plugin** - https://github.com/hashicorp/go-plugin
  - Process isolation, gRPC protocol, versioning

- **Kubernetes kubectl plugins** - https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/
  - Simple naming, PATH-based discovery

- **Krew (kubectl plugin manager)** - https://krew.sigs.k8s.io/
  - Registry format, installation flow

- **Homebrew** - https://brew.sh/
  - Formula structure, checksum verification

- **Terraform Providers** - https://developer.hashicorp.com/terraform/plugin
  - Protocol versioning, provider registry

### Relicta-Specific Documents

- PRD: `docs/prd.md`
- Technical Design: `docs/technical-design.md`
- Current Plugin Interface: `pkg/plugin/interface.go`
- Current Runtime Manager: `internal/plugin/manager.go`
- Current CLI Manager: `internal/plugin/manager/manager.go`

---

## Appendix A: Plugin Binary Naming Examples

| Plugin Name | Binary Name | Install Location |
|-------------|-------------|------------------|
| github | `github` | `~/.relicta/plugins/github` |
| slack | `slack` | `~/.relicta/plugins/slack` |
| npm | `npm` | `~/.relicta/plugins/npm` |
| my-custom | `my-custom` | `~/.relicta/plugins/my-custom` |

**Pattern:** Simple name, no prefix, matches plugin name exactly.

---

## Appendix B: Complete Plugin Example

**Full plugin implementation using new SDK helpers:**

```go
package main

import (
    "context"
    "fmt"

    "github.com/relicta-tech/relicta/pkg/plugin"
)

type NotifierPlugin struct{}

func (p *NotifierPlugin) GetInfo() plugin.Info {
    return plugin.Info{
        Name:        "notifier",
        Version:     "1.0.0",
        Description: "Send custom webhook notifications",
        Author:      "My Team",
        Hooks: []plugin.Hook{
            plugin.HookPostPublish,
            plugin.HookOnError,
        },
        ConfigSchema: `{
            "type": "object",
            "required": ["webhook_url"],
            "properties": {
                "webhook_url": {"type": "string"},
                "format": {"type": "string", "enum": ["json", "text"]}
            }
        }`,
    }
}

func (p *NotifierPlugin) Execute(ctx context.Context, req plugin.ExecuteRequest) (*plugin.ExecuteResponse, error) {
    parser := plugin.NewConfigParser(req.Config)
    webhookURL := parser.GetString("webhook_url", "NOTIFIER_WEBHOOK")
    format := parser.GetString("format")
    if format == "" {
        format = "json"
    }

    if req.DryRun {
        return &plugin.ExecuteResponse{
            Success: true,
            Message: fmt.Sprintf("Would send notification to %s", webhookURL),
        }, nil
    }

    // Send notification (implementation omitted)
    // ...

    return &plugin.ExecuteResponse{
        Success: true,
        Message: fmt.Sprintf("Notification sent to %s", webhookURL),
    }, nil
}

func (p *NotifierPlugin) Validate(ctx context.Context, config map[string]any) (*plugin.ValidateResponse, error) {
    vb := plugin.NewValidationBuilder()
    vb.ValidateRequired(config, "webhook_url")

    parser := plugin.NewConfigParser(config)
    if format := parser.GetString("format"); format != "" {
        if format != "json" && format != "text" {
            vb.AddError("format", "format must be 'json' or 'text'", "invalid_value")
        }
    }

    return vb.Build(), nil
}

func main() {
    plugin.Serve(&NotifierPlugin{})
}
```

---

**End of Document**
