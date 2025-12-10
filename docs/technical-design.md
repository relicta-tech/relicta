# Technical Design Document: ReleasePilot

## 1. Executive Summary

ReleasePilot is a CLI tool that automates software release management through AI-powered changelog generation, semantic versioning, and a plugin-based publishing system. Built in Go for security, performance, and single-binary distribution.

---

## 2. System Architecture

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              ReleasePilot CLI                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │    init     │  │    plan     │  │   version   │  │    notes    │        │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘        │
│         │                │                │                │                │
│  ┌──────┴──────┐  ┌──────┴──────┐  ┌──────┴──────┐  ┌──────┴──────┐        │
│  │   approve   │  │   publish   │  │   config    │  │   plugins   │        │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘        │
│         │                │                │                │                │
│         └────────────────┴────────────────┴────────────────┘                │
│                                    │                                         │
├────────────────────────────────────┼────────────────────────────────────────┤
│                              Core Services                                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │     Git     │  │   Version   │  │     AI      │  │   Plugin    │        │
│  │   Service   │  │   Service   │  │   Service   │  │   Manager   │        │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘        │
│                                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │  Template   │  │   Config    │  │   State     │  │   Logger    │        │
│  │   Engine    │  │   Manager   │  │   Manager   │  │             │        │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘        │
├─────────────────────────────────────────────────────────────────────────────┤
│                             External Integrations                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │   OpenAI    │  │  Anthropic  │  │   Ollama    │  │   GitHub    │        │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘        │
│                                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │   GitLab    │  │    Slack    │  │    Jira     │  │ LaunchNotes │        │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘        │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 Design Principles

| Principle | Implementation |
|-----------|----------------|
| **Single Binary** | All core functionality compiled into one executable |
| **Minimal Dependencies** | Leverage Go stdlib, carefully vetted external packages |
| **Plugin Isolation** | Plugins run as separate processes via go-plugin |
| **Fail-Safe** | Dry-run by default, explicit confirmation for destructive actions |
| **Offline-First** | Core functionality works without network; AI is optional |

---

## 3. Technology Stack

### 3.1 Core Technologies

| Technology | Version | Purpose |
|------------|---------|---------|
| Go | 1.22+ | Primary language |
| Cobra | v1.8+ | CLI framework |
| Viper | v1.18+ | Configuration management |
| go-git | v5 | Pure Go git implementation |
| go-plugin | v1.6+ | HashiCorp plugin system |

### 3.2 Core Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI command framework |
| `github.com/spf13/viper` | Configuration loading (YAML, JSON, env) |
| `github.com/go-git/go-git/v5` | Git operations without shelling out |
| `github.com/hashicorp/go-plugin` | Secure plugin execution over RPC |
| `github.com/Masterminds/semver/v3` | Semantic version parsing and manipulation |
| `github.com/charmbracelet/bubbletea` | Terminal UI components |
| `github.com/charmbracelet/lipgloss` | Terminal styling |
| `github.com/charmbracelet/log` | Structured logging |
| `text/template` | Go stdlib template rendering |
| `gopkg.in/yaml.v3` | YAML parsing |

### 3.3 AI Provider Clients

| Package | Purpose |
|---------|---------|
| `github.com/sashabaranov/go-openai` | OpenAI API client |
| `github.com/anthropics/anthropic-sdk-go` | Anthropic Claude client |
| HTTP client | Ollama REST API (no SDK needed) |

### 3.4 Build & Development

| Tool | Purpose |
|------|---------|
| `goreleaser` | Cross-platform binary releases |
| `golangci-lint` | Comprehensive linting |
| `gotestsum` | Test runner with better output |
| `mockery` | Interface mock generation |

---

## 4. Project Structure

```
release-pilot/
├── cmd/
│   └── release-pilot/
│       └── main.go                 # Entry point
│
├── internal/
│   ├── cli/                        # Cobra commands
│   │   ├── root.go
│   │   ├── init.go
│   │   ├── plan.go
│   │   ├── version.go
│   │   ├── notes.go
│   │   ├── approve.go
│   │   ├── publish.go
│   │   └── plugins.go
│   │
│   ├── service/                    # Core business logic
│   │   ├── git/
│   │   │   ├── service.go
│   │   │   ├── commit.go
│   │   │   ├── tag.go
│   │   │   └── conventional.go
│   │   ├── version/
│   │   │   ├── service.go
│   │   │   ├── semver.go
│   │   │   └── changelog.go
│   │   ├── ai/
│   │   │   ├── service.go
│   │   │   ├── provider.go
│   │   │   ├── openai.go
│   │   │   ├── anthropic.go
│   │   │   ├── ollama.go
│   │   │   └── prompts/
│   │   │       ├── changelog.go
│   │   │       ├── releasenotes.go
│   │   │       └── marketing.go
│   │   └── template/
│   │       ├── service.go
│   │       └── funcs.go
│   │
│   ├── plugin/                     # Plugin system
│   │   ├── manager.go
│   │   ├── registry.go
│   │   ├── loader.go
│   │   ├── hooks.go
│   │   └── proto/
│   │       └── plugin.proto        # gRPC plugin protocol
│   │
│   ├── config/                     # Configuration
│   │   ├── config.go
│   │   ├── schema.go
│   │   └── validate.go
│   │
│   ├── state/                      # Release state persistence
│   │   ├── manager.go
│   │   └── schema.go
│   │
│   └── ui/                         # Terminal UI components
│       ├── prompt.go
│       ├── spinner.go
│       ├── table.go
│       └── diff.go
│
├── pkg/                            # Public API for plugins
│   └── plugin/
│       ├── interface.go
│       ├── context.go
│       └── hooks.go
│
├── plugins/                        # Official plugins (separate modules)
│   ├── github/
│   │   ├── go.mod
│   │   ├── main.go
│   │   └── plugin.go
│   ├── gitlab/
│   ├── npm/
│   ├── slack/
│   └── jira/
│
├── templates/                      # Default templates
│   ├── changelog.tmpl
│   ├── release-notes.tmpl
│   └── marketing.tmpl
│
├── test/
│   ├── integration/
│   ├── e2e/
│   └── fixtures/
│
├── docs/
│   ├── prd.md
│   └── technical-design.md
│
├── go.mod
├── go.sum
├── Makefile
├── .goreleaser.yaml
└── .golangci.yaml
```

---

## 5. Core Domain Types

### 5.1 Commit Types

```go
// internal/service/git/commit.go

package git

import "time"

// Commit represents a git commit
type Commit struct {
    Hash      string
    ShortHash string
    Message   string
    Body      string
    Author    Author
    Date      time.Time
    Files     []string
}

// Author represents commit author information
type Author struct {
    Name  string
    Email string
}

// ConventionalCommit extends Commit with parsed conventional commit data
type ConventionalCommit struct {
    Commit
    Type       CommitType
    Scope      string
    Subject    string
    Breaking   bool
    References []Reference
}

// CommitType represents conventional commit types
type CommitType string

const (
    CommitTypeFeat     CommitType = "feat"
    CommitTypeFix      CommitType = "fix"
    CommitTypeDocs     CommitType = "docs"
    CommitTypeStyle    CommitType = "style"
    CommitTypeRefactor CommitType = "refactor"
    CommitTypePerf     CommitType = "perf"
    CommitTypeTest     CommitType = "test"
    CommitTypeChore    CommitType = "chore"
    CommitTypeCI       CommitType = "ci"
    CommitTypeBuild    CommitType = "build"
)

// Reference represents an issue/PR reference
type Reference struct {
    Type   string // "issue", "pr", "closes"
    Number int
    URL    string
}

// ReleaseType represents semantic version bump type
type ReleaseType string

const (
    ReleaseTypeMajor ReleaseType = "major"
    ReleaseTypeMinor ReleaseType = "minor"
    ReleaseTypePatch ReleaseType = "patch"
    ReleaseTypeNone  ReleaseType = "none"
)
```

### 5.2 Release Types

```go
// internal/state/schema.go

package state

import "time"

// ReleaseState represents the current state of an in-progress release
type ReleaseState struct {
    ID              string                 `json:"id"`
    StartedAt       time.Time              `json:"started_at"`
    CurrentStage    Stage                  `json:"current_stage"`
    PreviousVersion string                 `json:"previous_version"`
    NextVersion     string                 `json:"next_version"`
    ReleaseType     git.ReleaseType        `json:"release_type"`
    Commits         []git.Commit           `json:"commits"`
    Changes         CategorizedChanges     `json:"changes"`
    Changelog       string                 `json:"changelog,omitempty"`
    ReleaseNotes    string                 `json:"release_notes,omitempty"`
    Approved        bool                   `json:"approved"`
    ApprovedAt      *time.Time             `json:"approved_at,omitempty"`
    PluginStates    map[string]interface{} `json:"plugin_states,omitempty"`
    Artifacts       []Artifact             `json:"artifacts,omitempty"`
}

// Stage represents release lifecycle stages
type Stage string

const (
    StageInit     Stage = "init"
    StagePlan     Stage = "plan"
    StageVersion  Stage = "version"
    StageNotes    Stage = "notes"
    StageApprove  Stage = "approve"
    StagePublish  Stage = "publish"
    StageComplete Stage = "complete"
    StageFailed   Stage = "failed"
)

// CategorizedChanges groups commits by type
type CategorizedChanges struct {
    Breaking     []git.ConventionalCommit `json:"breaking,omitempty"`
    Features     []git.ConventionalCommit `json:"features,omitempty"`
    Fixes        []git.ConventionalCommit `json:"fixes,omitempty"`
    Performance  []git.ConventionalCommit `json:"performance,omitempty"`
    Other        []git.ConventionalCommit `json:"other,omitempty"`
}

// Artifact represents a release artifact
type Artifact struct {
    Name     string `json:"name"`
    Path     string `json:"path"`
    URL      string `json:"url,omitempty"`
    Checksum string `json:"checksum,omitempty"`
}
```

---

## 6. Service Interfaces

### 6.1 Git Service

```go
// internal/service/git/service.go

package git

import "context"

// Service defines git operations
type Service interface {
    // Repository information
    GetRepositoryRoot(ctx context.Context) (string, error)
    GetCurrentBranch(ctx context.Context) (string, error)
    GetRemoteURL(ctx context.Context) (string, error)

    // Commit operations
    GetCommitsSince(ctx context.Context, ref string) ([]Commit, error)
    GetCommitsBetween(ctx context.Context, from, to string) ([]Commit, error)
    ParseConventionalCommit(message string) (*ConventionalCommit, error)

    // Tag operations
    GetLatestTag(ctx context.Context) (string, error)
    GetAllTags(ctx context.Context) ([]Tag, error)
    CreateTag(ctx context.Context, name, message string) error

    // Release detection
    DetectReleaseType(commits []ConventionalCommit) ReleaseType
    CategorizeCommits(commits []ConventionalCommit) CategorizedChanges
}

// Tag represents a git tag
type Tag struct {
    Name    string
    Hash    string
    Message string
    Date    time.Time
}
```

### 6.2 Version Service

```go
// internal/service/version/service.go

package version

import "context"

// Service defines versioning operations
type Service interface {
    // Version calculations
    GetCurrentVersion(ctx context.Context) (string, error)
    CalculateNextVersion(current string, releaseType git.ReleaseType) (string, error)
    BumpVersion(ctx context.Context, releaseType git.ReleaseType, opts BumpOptions) (string, error)

    // Changelog management
    GenerateChangelog(commits []git.ConventionalCommit, opts ChangelogOptions) (string, error)
    UpdateChangelogFile(ctx context.Context, content string) error
}

// BumpOptions configures version bump behavior
type BumpOptions struct {
    PreID         string // Prerelease identifier (alpha, beta, rc)
    DryRun        bool
    GitTag        bool
    GitCommit     bool
    CommitMessage string
}

// ChangelogOptions configures changelog generation
type ChangelogOptions struct {
    Format            ChangelogFormat
    Template          string
    GroupBy           GroupBy
    IncludeCommitHash bool
    IncludeAuthor     bool
    LinkCommits       bool
    LinkIssues        bool
    RepositoryURL     string
}

type ChangelogFormat string

const (
    FormatConventional   ChangelogFormat = "conventional"
    FormatKeepAChangelog ChangelogFormat = "keep-a-changelog"
    FormatCustom         ChangelogFormat = "custom"
)

type GroupBy string

const (
    GroupByType  GroupBy = "type"
    GroupByScope GroupBy = "scope"
    GroupByNone  GroupBy = "none"
)
```

### 6.3 AI Service

```go
// internal/service/ai/service.go

package ai

import "context"

// Service defines AI content generation operations
type Service interface {
    // Provider management
    SetProvider(provider Provider)
    GetProvider() Provider

    // Content generation
    GenerateChangelog(ctx context.Context, commits []git.ConventionalCommit, opts Options) (string, error)
    GenerateReleaseNotes(ctx context.Context, changelog string, opts Options) (string, error)
    GenerateMarketingBlurb(ctx context.Context, releaseNotes string, opts Options) (string, error)

    // Summarization
    SummarizeCommits(ctx context.Context, commits []git.Commit) ([]CommitSummary, error)
}

// Provider defines the interface for AI providers
type Provider interface {
    Name() string
    Generate(ctx context.Context, prompt string, opts ProviderOptions) (string, error)
    IsAvailable(ctx context.Context) bool
}

// Options configures AI generation
type Options struct {
    Tone               Tone
    Audience           Audience
    MaxLength          int
    Language           string
    CustomInstructions string
}

type Tone string

const (
    ToneTechnical    Tone = "technical"
    ToneFriendly     Tone = "friendly"
    ToneExcited      Tone = "excited"
    ToneProfessional Tone = "professional"
)

type Audience string

const (
    AudienceDevelopers   Audience = "developers"
    AudienceUsers        Audience = "users"
    AudienceStakeholders Audience = "stakeholders"
    AudiencePublic       Audience = "public"
)

// ProviderOptions configures the AI provider
type ProviderOptions struct {
    Model       string
    Temperature float64
    MaxTokens   int
}

// CommitSummary represents a summarized commit
type CommitSummary struct {
    Commit  git.Commit
    Summary string
    Impact  string
}
```

---

## 7. Plugin Architecture

### 7.1 Plugin Interface

```go
// pkg/plugin/interface.go

package plugin

import "context"

// Plugin defines the interface all plugins must implement
type Plugin interface {
    // Metadata
    Name() string
    Version() string
    Description() string

    // Lifecycle
    Init(ctx context.Context, config map[string]interface{}) error
    Shutdown() error

    // Hooks - return nil to skip
    Hooks() Hooks
}

// Hooks defines all available lifecycle hooks
type Hooks struct {
    // Pre-hooks can modify context
    PreInit    func(ctx context.Context, c *PreInitContext) error
    PrePlan    func(ctx context.Context, c *PrePlanContext) error
    PreVersion func(ctx context.Context, c *PreVersionContext) error
    PreNotes   func(ctx context.Context, c *PreNotesContext) error
    PreApprove func(ctx context.Context, c *PreApproveContext) error
    PrePublish func(ctx context.Context, c *PrePublishContext) error

    // Post-hooks react to results
    PostInit    func(ctx context.Context, c *PostInitContext) error
    PostPlan    func(ctx context.Context, c *PostPlanContext) error
    PostVersion func(ctx context.Context, c *PostVersionContext) error
    PostNotes   func(ctx context.Context, c *PostNotesContext) error
    PostApprove func(ctx context.Context, c *PostApproveContext) error
    PostPublish func(ctx context.Context, c *PostPublishContext) error

    // Event hooks
    OnError   func(ctx context.Context, stage string, err error) error
    OnSuccess func(ctx context.Context, result *ReleaseResult) error
}
```

### 7.2 Hook Contexts

```go
// pkg/plugin/context.go

package plugin

// PreVersionContext is passed to PreVersion hooks
type PreVersionContext struct {
    Commits              []Commit
    SuggestedReleaseType string
    CurrentVersion       string

    // Plugins can set these to override
    OverrideReleaseType *string
    OverrideVersion     *string
}

// PostPublishContext is passed to PostPublish hooks
type PostPublishContext struct {
    Version      string
    Changelog    string
    ReleaseNotes string
    Tag          string
    Commits      []Commit
    Artifacts    []Artifact
    Repository   RepositoryInfo
}

// RepositoryInfo contains repository metadata
type RepositoryInfo struct {
    Owner    string
    Name     string
    URL      string
    Provider string // "github", "gitlab", "bitbucket"
}

// ReleaseResult contains the final release outcome
type ReleaseResult struct {
    Version      string
    Tag          string
    Changelog    string
    ReleaseNotes string
    Artifacts    []Artifact
    Duration     time.Duration
}
```

### 7.3 Plugin Manager

```go
// internal/plugin/manager.go

package plugin

import (
    "context"
    "os/exec"

    goplugin "github.com/hashicorp/go-plugin"
)

// Manager handles plugin loading and lifecycle
type Manager struct {
    plugins  map[string]*loadedPlugin
    registry *Registry
    logger   *slog.Logger
}

type loadedPlugin struct {
    client  *goplugin.Client
    plugin  Plugin
    config  map[string]interface{}
}

// Load discovers and loads all configured plugins
func (m *Manager) Load(ctx context.Context, configs []PluginConfig) error {
    for _, cfg := range configs {
        plugin, err := m.loadPlugin(ctx, cfg)
        if err != nil {
            return fmt.Errorf("failed to load plugin %s: %w", cfg.Name, err)
        }
        m.plugins[cfg.Name] = plugin
    }
    return nil
}

// ExecuteHook runs a specific hook across all plugins
func (m *Manager) ExecuteHook(ctx context.Context, hookName string, hookCtx interface{}) error {
    for name, p := range m.plugins {
        if err := m.executePluginHook(ctx, p, hookName, hookCtx); err != nil {
            m.logger.Error("plugin hook failed",
                "plugin", name,
                "hook", hookName,
                "error", err,
            )
            // Continue or fail based on plugin configuration
            if p.config["required"] == true {
                return err
            }
        }
    }
    return nil
}
```

### 7.4 Plugin Protocol (gRPC)

```protobuf
// internal/plugin/proto/plugin.proto

syntax = "proto3";

package plugin;

option go_package = "github.com/releasepilot/release-pilot/internal/plugin/proto";

service ReleasePlugin {
    rpc Init(InitRequest) returns (InitResponse);
    rpc Shutdown(Empty) returns (Empty);

    // Hooks
    rpc PreVersion(PreVersionRequest) returns (PreVersionResponse);
    rpc PostPublish(PostPublishRequest) returns (PostPublishResponse);
    // ... other hooks
}

message InitRequest {
    map<string, string> config = 1;
}

message InitResponse {
    bool success = 1;
    string error = 2;
}

message PreVersionRequest {
    repeated Commit commits = 1;
    string suggested_release_type = 2;
    string current_version = 3;
}

message PreVersionResponse {
    optional string override_release_type = 1;
    optional string override_version = 2;
}

message PostPublishRequest {
    string version = 1;
    string changelog = 2;
    string release_notes = 3;
    string tag = 4;
    RepositoryInfo repository = 5;
}

message PostPublishResponse {
    bool success = 1;
    string error = 2;
    repeated Artifact artifacts = 3;
}

message Commit {
    string hash = 1;
    string message = 2;
    string author_name = 3;
    string author_email = 4;
    int64 timestamp = 5;
}

message RepositoryInfo {
    string owner = 1;
    string name = 2;
    string url = 3;
    string provider = 4;
}

message Artifact {
    string name = 1;
    string path = 2;
    string url = 3;
}

message Empty {}
```

### 7.5 Example Plugin: GitHub

```go
// plugins/github/plugin.go

package main

import (
    "context"
    "fmt"

    "github.com/google/go-github/v60/github"
    "github.com/releasepilot/release-pilot/pkg/plugin"
)

type GitHubPlugin struct {
    client     *github.Client
    owner      string
    repo       string
    draft      bool
    prerelease bool
}

func (p *GitHubPlugin) Name() string        { return "github" }
func (p *GitHubPlugin) Version() string     { return "1.0.0" }
func (p *GitHubPlugin) Description() string { return "Publish releases to GitHub" }

func (p *GitHubPlugin) Init(ctx context.Context, config map[string]interface{}) error {
    token, _ := config["token"].(string)
    if token == "" {
        token = os.Getenv("GITHUB_TOKEN")
    }
    if token == "" {
        return fmt.Errorf("github token required")
    }

    p.client = github.NewClient(nil).WithAuthToken(token)
    p.owner, _ = config["owner"].(string)
    p.repo, _ = config["repo"].(string)
    p.draft, _ = config["draft"].(bool)
    p.prerelease, _ = config["prerelease"].(bool)

    return nil
}

func (p *GitHubPlugin) Hooks() plugin.Hooks {
    return plugin.Hooks{
        PostPublish: p.postPublish,
    }
}

func (p *GitHubPlugin) postPublish(ctx context.Context, c *plugin.PostPublishContext) error {
    owner := p.owner
    repo := p.repo
    if owner == "" {
        owner = c.Repository.Owner
    }
    if repo == "" {
        repo = c.Repository.Name
    }

    release := &github.RepositoryRelease{
        TagName:    github.String(c.Tag),
        Name:       github.String(fmt.Sprintf("v%s", c.Version)),
        Body:       github.String(c.ReleaseNotes),
        Draft:      github.Bool(p.draft),
        Prerelease: github.Bool(p.prerelease),
    }

    _, _, err := p.client.Repositories.CreateRelease(ctx, owner, repo, release)
    return err
}

func (p *GitHubPlugin) Shutdown() error {
    return nil
}
```

---

## 8. Configuration System

### 8.1 Configuration Schema

```go
// internal/config/schema.go

package config

// Config represents the complete configuration
type Config struct {
    Versioning VersioningConfig `mapstructure:"versioning"`
    Changelog  ChangelogConfig  `mapstructure:"changelog"`
    AI         AIConfig         `mapstructure:"ai"`
    Templates  TemplatesConfig  `mapstructure:"templates"`
    Plugins    []PluginConfig   `mapstructure:"plugins"`
    Workflow   WorkflowConfig   `mapstructure:"workflow"`
}

type VersioningConfig struct {
    Strategy      string   `mapstructure:"strategy" default:"conventional"`
    TagPrefix     string   `mapstructure:"tag_prefix" default:"v"`
    PackageFiles  []string `mapstructure:"package_files"`
    CommitMessage string   `mapstructure:"commit_message" default:"chore(release): {{version}}"`
    GitTag        bool     `mapstructure:"git_tag" default:"true"`
    GitPush       bool     `mapstructure:"git_push" default:"false"`
}

type ChangelogConfig struct {
    File        string            `mapstructure:"file" default:"CHANGELOG.md"`
    Format      string            `mapstructure:"format" default:"conventional"`
    Template    string            `mapstructure:"template"`
    GroupBy     string            `mapstructure:"group_by" default:"type"`
    CommitTypes map[string]string `mapstructure:"commit_types"`
}

type AIConfig struct {
    Enabled  bool   `mapstructure:"enabled" default:"true"`
    Provider string `mapstructure:"provider" default:"openai"`
    Model    string `mapstructure:"model"`
    APIKey   string `mapstructure:"api_key"`
    BaseURL  string `mapstructure:"base_url"`
    Tone     string `mapstructure:"tone" default:"professional"`
    Audience string `mapstructure:"audience" default:"developers"`
}

type TemplatesConfig struct {
    Changelog     string `mapstructure:"changelog"`
    ReleaseNotes  string `mapstructure:"release_notes"`
    MarketingBlurb string `mapstructure:"marketing_blurb"`
}

type PluginConfig struct {
    Name     string                 `mapstructure:"name"`
    Enabled  bool                   `mapstructure:"enabled" default:"true"`
    Required bool                   `mapstructure:"required" default:"false"`
    Config   map[string]interface{} `mapstructure:"config"`
}

type WorkflowConfig struct {
    RequireApproval  bool     `mapstructure:"require_approval" default:"false"`
    DryRunByDefault  bool     `mapstructure:"dry_run_by_default" default:"false"`
    AllowedBranches  []string `mapstructure:"allowed_branches"`
}
```

### 8.2 Configuration File Example

```yaml
# release.config.yaml

versioning:
  strategy: conventional
  tag_prefix: v
  git_tag: true
  git_push: true
  commit_message: "chore(release): {{version}} [skip ci]"

changelog:
  file: CHANGELOG.md
  format: keep-a-changelog
  group_by: type
  commit_types:
    feat: "Added"
    fix: "Fixed"
    perf: "Performance"
    security: "Security"
    deprecated: "Deprecated"
    removed: "Removed"

ai:
  enabled: true
  provider: openai
  model: gpt-4
  tone: professional
  audience: developers

plugins:
  - name: github
    config:
      draft: false
      prerelease: false

  - name: slack
    config:
      webhook: ${SLACK_WEBHOOK_URL}
      channel: "#releases"

workflow:
  require_approval: true
  dry_run_by_default: false
  allowed_branches:
    - main
    - "release/*"
```

### 8.3 Configuration Loading

```go
// internal/config/config.go

package config

import (
    "github.com/spf13/viper"
)

// Load reads configuration from file and environment
func Load() (*Config, error) {
    v := viper.New()

    // Config file search paths
    v.SetConfigName("release.config")
    v.SetConfigType("yaml")
    v.AddConfigPath(".")
    v.AddConfigPath("$HOME/.config/release-pilot")

    // Environment variable support
    v.SetEnvPrefix("RELEASE_PILOT")
    v.AutomaticEnv()

    // Allow ${VAR} expansion in config values
    v.SetConfigPermissions(0600)

    if err := v.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            return nil, fmt.Errorf("reading config: %w", err)
        }
        // No config file is OK, use defaults
    }

    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil {
        return nil, fmt.Errorf("unmarshaling config: %w", err)
    }

    // Expand environment variables in sensitive fields
    cfg.AI.APIKey = os.ExpandEnv(cfg.AI.APIKey)
    for i := range cfg.Plugins {
        cfg.Plugins[i].Config = expandEnvInMap(cfg.Plugins[i].Config)
    }

    if err := Validate(&cfg); err != nil {
        return nil, fmt.Errorf("validating config: %w", err)
    }

    return &cfg, nil
}
```

---

## 9. Command Implementation

### 9.1 Root Command

```go
// internal/cli/root.go

package cli

import (
    "github.com/spf13/cobra"
)

var (
    cfgFile string
    verbose bool
    dryRun  bool
)

func NewRootCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "release-pilot",
        Short: "AI-powered release management",
        Long: `ReleasePilot automates software releases with semantic versioning,
AI-generated changelogs, and plugin-based publishing.`,
        SilenceUsage: true,
    }

    // Global flags
    cmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path")
    cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
    cmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "preview without making changes")

    // Add subcommands
    cmd.AddCommand(
        newInitCmd(),
        newPlanCmd(),
        newVersionCmd(),
        newNotesCmd(),
        newApproveCmd(),
        newPublishCmd(),
        newPluginsCmd(),
    )

    return cmd
}
```

### 9.2 Plan Command

```go
// internal/cli/plan.go

package cli

import (
    "context"
    "fmt"

    "github.com/spf13/cobra"
)

func newPlanCmd() *cobra.Command {
    var (
        fromRef string
        toRef   string
        jsonOut bool
    )

    cmd := &cobra.Command{
        Use:   "plan",
        Short: "Analyze changes and suggest version bump",
        Long: `Analyze commits since the last release and suggest a semantic version bump.
Shows commits grouped by type and highlights breaking changes.`,
        Example: `  release-pilot plan
  release-pilot plan --from v1.0.0 --to HEAD
  release-pilot plan --json`,
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := cmd.Context()
            return runPlan(ctx, fromRef, toRef, jsonOut)
        },
    }

    cmd.Flags().StringVar(&fromRef, "from", "", "starting reference (tag, commit, or branch)")
    cmd.Flags().StringVar(&toRef, "to", "HEAD", "ending reference")
    cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")

    return cmd
}

func runPlan(ctx context.Context, from, to string, jsonOut bool) error {
    cfg, err := config.Load()
    if err != nil {
        return err
    }

    gitSvc := git.NewService()

    // Get starting point
    if from == "" {
        from, err = gitSvc.GetLatestTag(ctx)
        if err != nil {
            return fmt.Errorf("no previous tag found, use --from to specify starting point")
        }
    }

    // Get commits
    commits, err := gitSvc.GetCommitsBetween(ctx, from, to)
    if err != nil {
        return fmt.Errorf("getting commits: %w", err)
    }

    // Parse conventional commits
    var conventionalCommits []git.ConventionalCommit
    for _, c := range commits {
        if cc, err := gitSvc.ParseConventionalCommit(c.Message); err == nil {
            conventionalCommits = append(conventionalCommits, *cc)
        }
    }

    // Detect release type
    releaseType := gitSvc.DetectReleaseType(conventionalCommits)
    changes := gitSvc.CategorizeCommits(conventionalCommits)

    if jsonOut {
        return outputJSON(map[string]interface{}{
            "from":         from,
            "to":           to,
            "commits":      len(commits),
            "release_type": releaseType,
            "changes":      changes,
        })
    }

    // Pretty print
    printPlanSummary(from, to, releaseType, changes)
    return nil
}
```

### 9.3 Version Command

```go
// internal/cli/version.go

package cli

func newVersionCmd() *cobra.Command {
    var (
        bump   string
        preID  string
        noTag  bool
    )

    cmd := &cobra.Command{
        Use:   "version [VERSION]",
        Short: "Calculate and apply version bump",
        Long: `Calculate the next version based on conventional commits or set an explicit version.
Optionally creates a git tag and updates package files.`,
        Example: `  release-pilot version --bump minor
  release-pilot version 2.0.0
  release-pilot version --bump prerelease --preid beta`,
        Args: cobra.MaximumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := cmd.Context()
            var explicitVersion string
            if len(args) > 0 {
                explicitVersion = args[0]
            }
            return runVersion(ctx, explicitVersion, bump, preID, noTag, dryRun)
        },
    }

    cmd.Flags().StringVar(&bump, "bump", "", "bump type: major, minor, patch, premajor, preminor, prepatch, prerelease")
    cmd.Flags().StringVar(&preID, "preid", "", "prerelease identifier (alpha, beta, rc)")
    cmd.Flags().BoolVar(&noTag, "no-git-tag", false, "skip creating git tag")

    return cmd
}
```

---

## 10. AI Integration

### 10.1 OpenAI Provider

```go
// internal/service/ai/openai.go

package ai

import (
    "context"

    "github.com/sashabaranov/go-openai"
)

type OpenAIProvider struct {
    client *openai.Client
    model  string
}

func NewOpenAIProvider(apiKey, baseURL, model string) *OpenAIProvider {
    config := openai.DefaultConfig(apiKey)
    if baseURL != "" {
        config.BaseURL = baseURL
    }

    if model == "" {
        model = openai.GPT4
    }

    return &OpenAIProvider{
        client: openai.NewClientWithConfig(config),
        model:  model,
    }
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) Generate(ctx context.Context, prompt string, opts ProviderOptions) (string, error) {
    model := p.model
    if opts.Model != "" {
        model = opts.Model
    }

    temp := float32(0.7)
    if opts.Temperature > 0 {
        temp = float32(opts.Temperature)
    }

    maxTokens := 2000
    if opts.MaxTokens > 0 {
        maxTokens = opts.MaxTokens
    }

    resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: model,
        Messages: []openai.ChatCompletionMessage{
            {Role: openai.ChatMessageRoleUser, Content: prompt},
        },
        Temperature: temp,
        MaxTokens:   maxTokens,
    })
    if err != nil {
        return "", fmt.Errorf("openai completion: %w", err)
    }

    if len(resp.Choices) == 0 {
        return "", fmt.Errorf("no response from openai")
    }

    return resp.Choices[0].Message.Content, nil
}

func (p *OpenAIProvider) IsAvailable(ctx context.Context) bool {
    _, err := p.client.ListModels(ctx)
    return err == nil
}
```

### 10.2 Ollama Provider (Local)

```go
// internal/service/ai/ollama.go

package ai

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
)

type OllamaProvider struct {
    baseURL string
    model   string
    client  *http.Client
}

func NewOllamaProvider(baseURL, model string) *OllamaProvider {
    if baseURL == "" {
        baseURL = "http://localhost:11434"
    }
    if model == "" {
        model = "llama3"
    }

    return &OllamaProvider{
        baseURL: baseURL,
        model:   model,
        client:  &http.Client{Timeout: 120 * time.Second},
    }
}

func (p *OllamaProvider) Name() string { return "ollama" }

func (p *OllamaProvider) Generate(ctx context.Context, prompt string, opts ProviderOptions) (string, error) {
    model := p.model
    if opts.Model != "" {
        model = opts.Model
    }

    reqBody := map[string]interface{}{
        "model":  model,
        "prompt": prompt,
        "stream": false,
    }

    body, _ := json.Marshal(reqBody)
    req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/generate", bytes.NewReader(body))
    if err != nil {
        return "", err
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.client.Do(req)
    if err != nil {
        return "", fmt.Errorf("ollama request: %w", err)
    }
    defer resp.Body.Close()

    var result struct {
        Response string `json:"response"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", fmt.Errorf("decoding ollama response: %w", err)
    }

    return result.Response, nil
}

func (p *OllamaProvider) IsAvailable(ctx context.Context) bool {
    req, _ := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/api/tags", nil)
    resp, err := p.client.Do(req)
    if err != nil {
        return false
    }
    resp.Body.Close()
    return resp.StatusCode == 200
}
```

---

## 11. Error Handling

```go
// internal/errors/errors.go

package errors

import "errors"

// Sentinel errors
var (
    ErrNoRepository     = errors.New("not a git repository")
    ErrNoCommits        = errors.New("no commits found")
    ErrNoTags           = errors.New("no tags found")
    ErrInvalidConfig    = errors.New("invalid configuration")
    ErrPluginNotFound   = errors.New("plugin not found")
    ErrAIUnavailable    = errors.New("AI provider unavailable")
    ErrNotApproved      = errors.New("release not approved")
    ErrWrongBranch      = errors.New("releases only allowed from configured branches")
)

// Error wraps an error with additional context
type Error struct {
    Op      string // Operation that failed
    Kind    Kind   // Category of error
    Err     error  // Underlying error
    Details string // Additional details
}

type Kind int

const (
    KindUnknown Kind = iota
    KindConfig
    KindGit
    KindVersion
    KindAI
    KindPlugin
    KindState
    KindNetwork
    KindValidation
)

func (e *Error) Error() string {
    if e.Details != "" {
        return fmt.Sprintf("%s: %s: %v", e.Op, e.Details, e.Err)
    }
    return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *Error) Unwrap() error {
    return e.Err
}

// Recoverable returns true if the error can be recovered from
func (e *Error) Recoverable() bool {
    switch e.Kind {
    case KindAI, KindNetwork, KindPlugin:
        return true
    default:
        return false
    }
}

// E constructs an Error
func E(op string, kind Kind, err error, details ...string) *Error {
    e := &Error{Op: op, Kind: kind, Err: err}
    if len(details) > 0 {
        e.Details = details[0]
    }
    return e
}
```

---

## 12. Build & Release

### 12.1 Makefile

```makefile
# Makefile

BINARY_NAME=release-pilot
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}"

.PHONY: all build test lint clean install

all: lint test build

build:
	go build ${LDFLAGS} -o bin/${BINARY_NAME} ./cmd/release-pilot

install:
	go install ${LDFLAGS} ./cmd/release-pilot

test:
	go test -race -coverprofile=coverage.out ./...

test-integration:
	go test -race -tags=integration ./test/integration/...

lint:
	golangci-lint run

clean:
	rm -rf bin/ dist/ coverage.out

# Generate mocks
generate:
	go generate ./...

# Build for all platforms
release:
	goreleaser release --clean

release-snapshot:
	goreleaser release --snapshot --clean
```

### 12.2 GoReleaser Configuration

```yaml
# .goreleaser.yaml

version: 2

project_name: release-pilot

before:
  hooks:
    - go mod tidy
    - go generate ./...

builds:
  - id: release-pilot
    main: ./cmd/release-pilot
    binary: release-pilot
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.Date}}

archives:
  - id: default
    formats:
      - tar.gz
      - zip
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - LICENSE
      - README.md

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"

brews:
  - name: release-pilot
    repository:
      owner: releasepilot
      name: homebrew-tap
    homepage: "https://github.com/releasepilot/release-pilot"
    description: "AI-powered release management CLI"
    install: |
      bin.install "release-pilot"

nfpms:
  - id: packages
    package_name: release-pilot
    vendor: ReleasePilot
    homepage: "https://github.com/releasepilot/release-pilot"
    maintainer: "ReleasePilot Team"
    description: "AI-powered release management CLI"
    formats:
      - deb
      - rpm

sboms:
  - artifacts: archive

signs:
  - artifacts: checksum
```

---

## 13. Testing Strategy

### 13.1 Unit Test Example

```go
// internal/service/git/conventional_test.go

package git_test

import (
    "testing"

    "github.com/releasepilot/release-pilot/internal/service/git"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestParseConventionalCommit(t *testing.T) {
    tests := []struct {
        name     string
        message  string
        expected *git.ConventionalCommit
        wantErr  bool
    }{
        {
            name:    "simple feat",
            message: "feat: add new feature",
            expected: &git.ConventionalCommit{
                Type:    git.CommitTypeFeat,
                Subject: "add new feature",
            },
        },
        {
            name:    "feat with scope",
            message: "feat(api): add new endpoint",
            expected: &git.ConventionalCommit{
                Type:    git.CommitTypeFeat,
                Scope:   "api",
                Subject: "add new endpoint",
            },
        },
        {
            name:    "breaking change with !",
            message: "feat!: breaking change",
            expected: &git.ConventionalCommit{
                Type:     git.CommitTypeFeat,
                Subject:  "breaking change",
                Breaking: true,
            },
        },
        {
            name:    "non-conventional commit",
            message: "Update readme",
            wantErr: true,
        },
    }

    svc := git.NewService()
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := svc.ParseConventionalCommit(tt.message)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.expected.Type, result.Type)
            assert.Equal(t, tt.expected.Scope, result.Scope)
            assert.Equal(t, tt.expected.Subject, result.Subject)
            assert.Equal(t, tt.expected.Breaking, result.Breaking)
        })
    }
}

func TestDetectReleaseType(t *testing.T) {
    tests := []struct {
        name     string
        commits  []git.ConventionalCommit
        expected git.ReleaseType
    }{
        {
            name: "breaking change = major",
            commits: []git.ConventionalCommit{
                {Type: git.CommitTypeFeat, Breaking: true},
            },
            expected: git.ReleaseTypeMajor,
        },
        {
            name: "feature = minor",
            commits: []git.ConventionalCommit{
                {Type: git.CommitTypeFeat},
            },
            expected: git.ReleaseTypeMinor,
        },
        {
            name: "fix only = patch",
            commits: []git.ConventionalCommit{
                {Type: git.CommitTypeFix},
            },
            expected: git.ReleaseTypePatch,
        },
    }

    svc := git.NewService()
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := svc.DetectReleaseType(tt.commits)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### 13.2 Integration Test Example

```go
// test/integration/plan_test.go

//go:build integration

package integration

import (
    "os"
    "os/exec"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/require"
)

func TestPlanCommand(t *testing.T) {
    // Create temp git repo
    dir := t.TempDir()

    // Initialize git repo
    runGit(t, dir, "init")
    runGit(t, dir, "config", "user.email", "test@test.com")
    runGit(t, dir, "config", "user.name", "Test")

    // Create initial commit and tag
    writeFile(t, dir, "README.md", "# Test")
    runGit(t, dir, "add", ".")
    runGit(t, dir, "commit", "-m", "chore: initial commit")
    runGit(t, dir, "tag", "v1.0.0")

    // Add feature commit
    writeFile(t, dir, "feature.go", "package main")
    runGit(t, dir, "add", ".")
    runGit(t, dir, "commit", "-m", "feat: add new feature")

    // Run plan command
    cmd := exec.Command("release-pilot", "plan", "--json")
    cmd.Dir = dir
    output, err := cmd.Output()
    require.NoError(t, err)

    // Verify output
    require.Contains(t, string(output), `"release_type":"minor"`)
}

func runGit(t *testing.T, dir string, args ...string) {
    cmd := exec.Command("git", args...)
    cmd.Dir = dir
    require.NoError(t, cmd.Run())
}

func writeFile(t *testing.T, dir, name, content string) {
    path := filepath.Join(dir, name)
    require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}
```

---

## 14. Security Considerations

### 14.1 Credential Management

| Credential | Storage Method |
|------------|----------------|
| AI API Keys | Environment variables only (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`) |
| GitHub Token | `GITHUB_TOKEN` env var or `gh` CLI authentication |
| Plugin Credentials | Plugin-specific env vars, `${VAR}` expansion in config |

### 14.2 Security Practices

- **No credentials in config files** - Only `${ENV_VAR}` references allowed
- **Secure plugin communication** - gRPC with TLS for external plugins
- **Input sanitization** - Commit messages sanitized before AI prompts
- **Minimal permissions** - Plugins run with least privilege
- **Audit logging** - All release actions logged with timestamps
- **Binary verification** - Release binaries signed and checksummed

---

## 15. Performance Targets

| Operation | Target |
|-----------|--------|
| `release-pilot plan` | < 1s for repos with < 1000 commits |
| `release-pilot notes` (no AI) | < 500ms |
| `release-pilot notes` (with AI) | < 10s |
| Plugin loading | < 200ms total |
| Binary size | < 20MB |
| Memory usage | < 50MB typical |

---

## 16. Glossary

| Term | Definition |
|------|------------|
| **Conventional Commit** | Commit message following the Conventional Commits spec |
| **Lifecycle Hook** | Extension point where plugins execute |
| **Release State** | Persisted data about an in-progress release |
| **Semver** | Semantic Versioning (MAJOR.MINOR.PATCH) |
| **go-plugin** | HashiCorp's plugin system using gRPC |
