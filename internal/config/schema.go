// Package config provides configuration management for Relicta.
package config

import (
	"time"
)

// Config is the root configuration for Relicta.
type Config struct {
	// Versioning configures version management.
	Versioning VersioningConfig `mapstructure:"versioning" json:"versioning"`
	// Git configures git operations and authentication.
	Git GitConfig `mapstructure:"git" json:"git"`
	// Changelog configures changelog generation.
	Changelog ChangelogConfig `mapstructure:"changelog" json:"changelog"`
	// AI configures AI integration.
	AI AIConfig `mapstructure:"ai" json:"ai"`
	// Plugins configures plugin loading and execution.
	Plugins []PluginConfig `mapstructure:"plugins" json:"plugins"`
	// Workflow configures the release workflow.
	Workflow WorkflowConfig `mapstructure:"workflow" json:"workflow"`
	// Output configures output settings.
	Output OutputConfig `mapstructure:"output" json:"output"`
	// Telemetry configures observability and tracing.
	Telemetry TelemetryConfig `mapstructure:"telemetry" json:"telemetry"`
	// Governance configures Change Governance Protocol (CGP) settings.
	Governance GovernanceConfig `mapstructure:"governance" json:"governance"`
	// Webhooks configures webhook notifications for release events.
	Webhooks []WebhookConfig `mapstructure:"webhooks" json:"webhooks,omitempty"`
	// Monorepo configures multi-package/monorepo versioning support.
	Monorepo MonorepoConfig `mapstructure:"monorepo" json:"monorepo,omitempty"`
}

// VersioningConfig configures version management.
type VersioningConfig struct {
	// Strategy is the versioning strategy (conventional, manual).
	Strategy string `mapstructure:"strategy" json:"strategy"`
	// TagPrefix is the prefix for version tags (default: "v").
	TagPrefix string `mapstructure:"tag_prefix" json:"tag_prefix"`
	// GitTag indicates whether to create a git tag.
	GitTag bool `mapstructure:"git_tag" json:"git_tag"`
	// GitPush indicates whether to push the tag to remote.
	GitPush bool `mapstructure:"git_push" json:"git_push"`
	// GitSign indicates whether to sign the tag with GPG.
	GitSign bool `mapstructure:"git_sign" json:"git_sign"`
	// PrereleaseSuffix is the suffix for prerelease versions (e.g., "alpha", "beta", "rc").
	PrereleaseSuffix string `mapstructure:"prerelease_suffix" json:"prerelease_suffix,omitempty"`
	// BuildMetadata is optional build metadata to append to the version.
	BuildMetadata string `mapstructure:"build_metadata" json:"build_metadata,omitempty"`
	// BumpFrom specifies where to read the current version from (tag, file, package.json).
	BumpFrom string `mapstructure:"bump_from" json:"bump_from"`
	// VersionFile is the file to update with the new version (if BumpFrom is "file").
	VersionFile string `mapstructure:"version_file" json:"version_file,omitempty"`
}

// GitConfig configures git operations and authentication.
type GitConfig struct {
	// DefaultRemote is the default remote name (default: "origin").
	DefaultRemote string `mapstructure:"default_remote" json:"default_remote,omitempty"`
	// UseCLIFallback enables falling back to git CLI when go-git fails.
	// This is useful for authentication with credential helpers (default: true).
	UseCLIFallback *bool `mapstructure:"use_cli_fallback" json:"use_cli_fallback,omitempty"`
	// Auth configures git authentication.
	Auth GitAuthConfig `mapstructure:"auth" json:"auth,omitempty"`
}

// GitAuthConfig configures git authentication.
type GitAuthConfig struct {
	// Type is the authentication type: "auto" (default), "token", "ssh", "basic".
	// "auto" uses system credential helpers via git CLI fallback.
	// "token" uses a personal access token for HTTPS authentication.
	// "ssh" uses SSH key authentication.
	// "basic" uses username/password authentication.
	Type string `mapstructure:"type" json:"type,omitempty"`
	// Token is the personal access token for HTTPS auth (can use env var expansion).
	// Used when Type is "token" or for GitHub/GitLab APIs.
	Token string `mapstructure:"token" json:"token,omitempty"`
	// Username is the username for basic auth.
	Username string `mapstructure:"username" json:"username,omitempty"`
	// Password is the password for basic auth (can use env var expansion).
	Password string `mapstructure:"password" json:"password,omitempty"`
	// SSHKeyPath is the path to the SSH private key file.
	SSHKeyPath string `mapstructure:"ssh_key_path" json:"ssh_key_path,omitempty"`
	// SSHKeyPassword is the password for the SSH key (can use env var expansion).
	SSHKeyPassword string `mapstructure:"ssh_key_password" json:"ssh_key_password,omitempty"`
}

// UseCLI returns whether to use CLI fallback (defaults to true).
func (g *GitConfig) UseCLI() bool {
	if g.UseCLIFallback == nil {
		return true
	}
	return *g.UseCLIFallback
}

// ChangelogConfig configures changelog generation.
type ChangelogConfig struct {
	// File is the changelog file path.
	File string `mapstructure:"file" json:"file"`
	// Format is the changelog format (keep-a-changelog, conventional, custom).
	Format string `mapstructure:"format" json:"format"`
	// ProductName is the product name for display in changelogs.
	ProductName string `mapstructure:"product_name" json:"product_name,omitempty"`
	// GroupBy specifies how to group changes (type, scope, none).
	GroupBy string `mapstructure:"group_by" json:"group_by"`
	// Template is a custom template file path.
	Template string `mapstructure:"template" json:"template,omitempty"`
	// IncludeCommitHash includes commit hashes in the changelog.
	IncludeCommitHash bool `mapstructure:"include_commit_hash" json:"include_commit_hash"`
	// IncludeAuthor includes author information in the changelog.
	IncludeAuthor bool `mapstructure:"include_author" json:"include_author"`
	// IncludeDate includes dates in the changelog.
	IncludeDate bool `mapstructure:"include_date" json:"include_date"`
	// LinkCommits links commit hashes to the repository.
	LinkCommits bool `mapstructure:"link_commits" json:"link_commits"`
	// LinkIssues links issue references to the issue tracker.
	LinkIssues bool `mapstructure:"link_issues" json:"link_issues"`
	// RepositoryURL is the repository URL for linking.
	RepositoryURL string `mapstructure:"repository_url" json:"repository_url,omitempty"`
	// IssueURL is the issue tracker URL pattern.
	IssueURL string `mapstructure:"issue_url" json:"issue_url,omitempty"`
	// Exclude lists commit types to exclude from the changelog.
	Exclude []string `mapstructure:"exclude" json:"exclude,omitempty"`
	// Categories customizes category labels for commit types.
	Categories map[string]string `mapstructure:"categories" json:"categories,omitempty"`
}

// AIConfig configures AI integration.
type AIConfig struct {
	// Enabled indicates whether AI features are enabled.
	Enabled bool `mapstructure:"enabled" json:"enabled"`
	// Provider is the AI provider (openai, ollama, anthropic, gemini, azure-openai).
	// Use "ollama" for local/offline LLM support.
	// Use "anthropic" or "claude" for Anthropic Claude API.
	// Use "gemini" for Google Gemini API.
	// Use "azure-openai" for Azure OpenAI Service.
	Provider string `mapstructure:"provider" json:"provider"`
	// Model is the model to use.
	// For OpenAI: "gpt-4", "gpt-3.5-turbo", "gpt-4o", etc.
	// For Ollama: "llama3.2", "mistral", "codellama", etc.
	// For Anthropic: "claude-sonnet-4-20250514", "claude-3-opus-20240229", etc.
	// For Gemini: "gemini-2.0-flash-exp", "gemini-1.5-pro", "gemini-1.5-flash", etc.
	// For Azure OpenAI: Use your deployment name.
	Model string `mapstructure:"model" json:"model"`
	// APIKey is the API key (can use environment variable expansion).
	APIKey string `mapstructure:"api_key" json:"api_key,omitempty"`
	// BaseURL is the API base URL (for custom endpoints).
	// For Ollama, defaults to "http://localhost:11434/v1".
	// For Azure OpenAI, use https://YOUR_RESOURCE.openai.azure.com/openai/deployments/YOUR_DEPLOYMENT.
	BaseURL string `mapstructure:"base_url" json:"base_url,omitempty"`
	// APIVersion is the API version (required for Azure OpenAI, e.g., "2024-02-15-preview").
	APIVersion string `mapstructure:"api_version" json:"api_version,omitempty"`
	// Tone is the tone for generated content (technical, friendly, professional, excited).
	Tone string `mapstructure:"tone" json:"tone"`
	// Audience is the target audience (developers, users, public, marketing).
	Audience string `mapstructure:"audience" json:"audience"`
	// IncludeEmoji includes emojis in AI-generated content.
	IncludeEmoji bool `mapstructure:"include_emoji" json:"include_emoji"`
	// MaxTokens is the maximum tokens for AI responses.
	MaxTokens int `mapstructure:"max_tokens" json:"max_tokens"`
	// Temperature controls randomness (0.0-2.0).
	Temperature float64 `mapstructure:"temperature" json:"temperature"`
	// Timeout is the API request timeout.
	Timeout time.Duration `mapstructure:"timeout" json:"timeout"`
	// RetryAttempts is the number of retry attempts for failed requests.
	RetryAttempts int `mapstructure:"retry_attempts" json:"retry_attempts"`
	// CustomPrompts allows custom prompt templates.
	CustomPrompts CustomPrompts `mapstructure:"custom_prompts" json:"custom_prompts,omitempty"`
}

// CustomPrompts allows customization of AI prompts.
type CustomPrompts struct {
	// ChangelogSystem is the system prompt for changelog generation.
	ChangelogSystem string `mapstructure:"changelog_system" json:"changelog_system,omitempty"`
	// ChangelogUser is the user prompt template for changelog generation.
	ChangelogUser string `mapstructure:"changelog_user" json:"changelog_user,omitempty"`
	// ReleaseNotesSystem is the system prompt for release notes generation.
	ReleaseNotesSystem string `mapstructure:"release_notes_system" json:"release_notes_system,omitempty"`
	// ReleaseNotesUser is the user prompt template for release notes generation.
	ReleaseNotesUser string `mapstructure:"release_notes_user" json:"release_notes_user,omitempty"`
	// MarketingSystem is the system prompt for marketing blurb generation.
	MarketingSystem string `mapstructure:"marketing_system" json:"marketing_system,omitempty"`
	// MarketingUser is the user prompt template for marketing blurb generation.
	MarketingUser string `mapstructure:"marketing_user" json:"marketing_user,omitempty"`
}

// PluginCapabilities defines security restrictions for plugin execution.
type PluginCapabilities struct {
	// AllowNetwork enables network access for the plugin.
	AllowNetwork bool `mapstructure:"allow_network" json:"allow_network"`
	// AllowFilesystem enables filesystem access for the plugin.
	AllowFilesystem bool `mapstructure:"allow_filesystem" json:"allow_filesystem"`
	// AllowedPaths restricts filesystem access to specific paths.
	AllowedPaths []string `mapstructure:"allowed_paths" json:"allowed_paths,omitempty"`
	// AllowEnvRead enables reading environment variables.
	AllowEnvRead bool `mapstructure:"allow_env_read" json:"allow_env_read"`
	// AllowedEnvVars restricts env access to specific variables.
	AllowedEnvVars []string `mapstructure:"allowed_env_vars" json:"allowed_env_vars,omitempty"`
	// MaxMemoryMB is the maximum memory the plugin can use (0 = unlimited).
	MaxMemoryMB int64 `mapstructure:"max_memory_mb" json:"max_memory_mb"`
	// MaxCPUPercent is the maximum CPU percentage (0 = unlimited).
	MaxCPUPercent int `mapstructure:"max_cpu_percent" json:"max_cpu_percent"`
	// MaxFileDescriptors is the maximum number of open files (0 = unlimited).
	// On Linux, this is enforced via RLIMIT_NOFILE.
	MaxFileDescriptors int `mapstructure:"max_file_descriptors" json:"max_file_descriptors"`
	// MaxCPUSeconds is the maximum CPU time in seconds (0 = unlimited).
	// On Linux, this is enforced via RLIMIT_CPU. Note this is CPU time, not wall-clock time.
	MaxCPUSeconds int `mapstructure:"max_cpu_seconds" json:"max_cpu_seconds"`
}

// PluginConfig configures a single plugin.
type PluginConfig struct {
	// Name is the plugin name.
	Name string `mapstructure:"name" json:"name"`
	// Enabled indicates whether the plugin is enabled (default: true).
	Enabled *bool `mapstructure:"enabled" json:"enabled,omitempty"`
	// Path is the path to the plugin binary (if not in PATH).
	Path string `mapstructure:"path" json:"path,omitempty"`
	// Config contains plugin-specific configuration.
	Config map[string]any `mapstructure:"config" json:"config,omitempty"`
	// Hooks specifies which hooks this plugin should run on.
	Hooks []string `mapstructure:"hooks" json:"hooks,omitempty"`
	// Timeout is the plugin execution timeout.
	Timeout time.Duration `mapstructure:"timeout" json:"timeout,omitempty"`
	// ContinueOnError indicates whether to continue if the plugin fails.
	ContinueOnError bool `mapstructure:"continue_on_error" json:"continue_on_error"`
	// Capabilities defines security restrictions for the plugin.
	Capabilities *PluginCapabilities `mapstructure:"capabilities" json:"capabilities,omitempty"`
}

// IsEnabled returns whether the plugin is enabled.
func (p *PluginConfig) IsEnabled() bool {
	if p.Enabled == nil {
		return true
	}
	return *p.Enabled
}

// WorkflowConfig configures the release workflow.
type WorkflowConfig struct {
	// RequireApproval requires manual approval before publishing.
	RequireApproval bool `mapstructure:"require_approval" json:"require_approval"`
	// AllowedBranches restricts releases to specific branches.
	AllowedBranches []string `mapstructure:"allowed_branches" json:"allowed_branches,omitempty"`
	// RequireCleanWorkingTree requires no uncommitted changes.
	RequireCleanWorkingTree bool `mapstructure:"require_clean_working_tree" json:"require_clean_working_tree"`
	// RequireUpToDate requires the branch to be up-to-date with remote.
	RequireUpToDate bool `mapstructure:"require_up_to_date" json:"require_up_to_date"`
	// DryRunByDefault runs in dry-run mode by default.
	DryRunByDefault bool `mapstructure:"dry_run_by_default" json:"dry_run_by_default"`
	// AutoCommitChangelog automatically commits changelog changes.
	AutoCommitChangelog bool `mapstructure:"auto_commit_changelog" json:"auto_commit_changelog"`
	// ChangelogCommitMessage is the commit message for changelog updates.
	ChangelogCommitMessage string `mapstructure:"changelog_commit_message" json:"changelog_commit_message,omitempty"`
	// PreReleaseHook is a command to run before the release.
	PreReleaseHook string `mapstructure:"pre_release_hook" json:"pre_release_hook,omitempty"`
	// PostReleaseHook is a command to run after the release.
	PostReleaseHook string `mapstructure:"post_release_hook" json:"post_release_hook,omitempty"`
}

// OutputConfig configures output settings.
type OutputConfig struct {
	// Format is the output format (text, json, yaml).
	Format string `mapstructure:"format" json:"format"`
	// Color enables colored output.
	Color bool `mapstructure:"color" json:"color"`
	// Verbose enables verbose output.
	Verbose bool `mapstructure:"verbose" json:"verbose"`
	// Quiet suppresses non-essential output.
	Quiet bool `mapstructure:"quiet" json:"quiet"`
	// LogFile is the path to a log file.
	LogFile string `mapstructure:"log_file" json:"log_file,omitempty"`
	// LogLevel is the log level (debug, info, warn, error).
	LogLevel string `mapstructure:"log_level" json:"log_level"`
	// PluginAuditLog is the path to the plugin audit log file.
	PluginAuditLog string `mapstructure:"plugin_audit_log" json:"plugin_audit_log,omitempty"`
}

// TelemetryConfig configures observability and tracing.
type TelemetryConfig struct {
	// Tracing configures distributed tracing.
	Tracing TracingConfig `mapstructure:"tracing" json:"tracing"`
	// Metrics configures metrics collection.
	Metrics MetricsConfig `mapstructure:"metrics" json:"metrics"`
}

// TracingConfig configures distributed tracing.
type TracingConfig struct {
	// Enabled indicates whether tracing is enabled.
	Enabled bool `mapstructure:"enabled" json:"enabled"`
	// Endpoint is the OTLP endpoint URL (e.g., "localhost:4317").
	Endpoint string `mapstructure:"endpoint" json:"endpoint,omitempty"`
	// Insecure disables TLS for the OTLP connection.
	Insecure bool `mapstructure:"insecure" json:"insecure"`
	// SampleRate is the sampling rate (0.0 to 1.0, default 1.0 = sample all).
	SampleRate float64 `mapstructure:"sample_rate" json:"sample_rate"`
	// Headers are additional headers to send with OTLP requests.
	Headers map[string]string `mapstructure:"headers" json:"headers,omitempty"`
}

// MetricsConfig configures metrics collection.
type MetricsConfig struct {
	// Enabled indicates whether metrics are enabled.
	Enabled bool `mapstructure:"enabled" json:"enabled"`
	// Endpoint is the metrics endpoint (for Prometheus scraping).
	Endpoint string `mapstructure:"endpoint" json:"endpoint,omitempty"`
	// Port is the port for the metrics HTTP server.
	Port int `mapstructure:"port" json:"port"`
}

// GovernanceConfig configures Change Governance Protocol (CGP) settings.
// CGP enables policy-based governance for release decisions, especially
// in agentic systems where AI agents propose changes.
type GovernanceConfig struct {
	// Enabled indicates whether CGP governance is enabled.
	Enabled bool `mapstructure:"enabled" json:"enabled"`
	// StrictMode enforces governance decisions (blocks releases that don't pass).
	// When false, governance operates in advisory mode (logs warnings but doesn't block).
	StrictMode bool `mapstructure:"strict_mode" json:"strict_mode"`
	// AutoApproveThreshold is the risk score (0.0-1.0) below which changes
	// may be auto-approved without human review.
	AutoApproveThreshold float64 `mapstructure:"auto_approve_threshold" json:"auto_approve_threshold"`
	// MaxAutoApproveRisk is the maximum risk score (0.0-1.0) for auto-approval.
	// Changes above this threshold always require human review.
	MaxAutoApproveRisk float64 `mapstructure:"max_auto_approve_risk" json:"max_auto_approve_risk"`
	// RequireHumanForBreaking forces human review for breaking changes (major bumps).
	RequireHumanForBreaking bool `mapstructure:"require_human_for_breaking" json:"require_human_for_breaking"`
	// RequireHumanForSecurity forces human review for security-related changes.
	RequireHumanForSecurity bool `mapstructure:"require_human_for_security" json:"require_human_for_security"`
	// TrustedActors is a list of actor IDs that are trusted for auto-approval.
	// These actors can auto-approve low-risk changes without additional review.
	TrustedActors []string `mapstructure:"trusted_actors" json:"trusted_actors,omitempty"`
	// MemoryEnabled indicates whether Release Memory (historical tracking) is enabled.
	MemoryEnabled bool `mapstructure:"memory_enabled" json:"memory_enabled"`
	// MemoryPath is the path to the Release Memory storage file.
	// Defaults to ".relicta/governance/memory.json" in the repository root.
	MemoryPath string `mapstructure:"memory_path" json:"memory_path,omitempty"`
	// PolicyDir is the directory containing policy DSL files (.policy).
	// Defaults to ".relicta/policies" in the repository root.
	PolicyDir string `mapstructure:"policy_dir" json:"policy_dir,omitempty"`
	// Policies is a list of custom policy rules defined inline in YAML.
	Policies []GovernancePolicyConfig `mapstructure:"policies" json:"policies,omitempty"`
}

// GovernancePolicyConfig configures a custom governance policy rule.
type GovernancePolicyConfig struct {
	// Name is the unique policy name.
	Name string `mapstructure:"name" json:"name"`
	// Description describes what this policy enforces.
	Description string `mapstructure:"description" json:"description,omitempty"`
	// Enabled indicates whether this policy is active.
	Enabled *bool `mapstructure:"enabled" json:"enabled,omitempty"`
	// Priority determines evaluation order (higher = evaluated first).
	Priority int `mapstructure:"priority" json:"priority"`
	// Conditions are the conditions that trigger this policy.
	Conditions []PolicyConditionConfig `mapstructure:"conditions" json:"conditions,omitempty"`
	// Action is the action to take when conditions are met.
	Action string `mapstructure:"action" json:"action"` // approve, deny, require_review
	// Message is an optional message to include with the decision.
	Message string `mapstructure:"message" json:"message,omitempty"`
}

// PolicyConditionConfig configures a single policy condition.
type PolicyConditionConfig struct {
	// Field is the field to evaluate (e.g., "risk_score", "bump_type", "actor_kind").
	Field string `mapstructure:"field" json:"field"`
	// Operator is the comparison operator (eq, ne, gt, gte, lt, lte, in, contains).
	Operator string `mapstructure:"operator" json:"operator"`
	// Value is the value to compare against.
	Value any `mapstructure:"value" json:"value"`
}

// IsPolicyEnabled returns whether the policy is enabled.
func (p *GovernancePolicyConfig) IsPolicyEnabled() bool {
	if p.Enabled == nil {
		return true
	}
	return *p.Enabled
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	enabled := true
	useCLIFallback := true
	return &Config{
		Versioning: VersioningConfig{
			Strategy:  "conventional",
			TagPrefix: "v",
			GitTag:    true,
			GitPush:   true,
			GitSign:   false,
			BumpFrom:  "tag",
		},
		Git: GitConfig{
			DefaultRemote:  "origin",
			UseCLIFallback: &useCLIFallback,
			Auth: GitAuthConfig{
				Type: "auto", // Use system credential helpers via git CLI
			},
		},
		Changelog: ChangelogConfig{
			File:              "CHANGELOG.md",
			Format:            "keep-a-changelog",
			GroupBy:           "type",
			IncludeCommitHash: true,
			IncludeAuthor:     false,
			IncludeDate:       true,
			LinkCommits:       false, // Auto-enabled if repository_url is detected from git
			LinkIssues:        false, // Must be explicitly enabled with issue_url
			Exclude:           []string{"chore", "ci", "docs", "style", "test"},
			Categories: map[string]string{
				"feat":     "Features",
				"fix":      "Bug Fixes",
				"perf":     "Performance Improvements",
				"refactor": "Code Refactoring",
				"revert":   "Reverts",
				"build":    "Build System",
			},
		},
		AI: AIConfig{
			Enabled:       false,
			Provider:      "openai",
			Model:         "gpt-4",
			Tone:          "professional",
			Audience:      "developers",
			MaxTokens:     2048,
			Temperature:   0.7,
			Timeout:       30 * time.Second,
			RetryAttempts: 3,
		},
		Plugins: []PluginConfig{
			{
				Name:    "github",
				Enabled: &enabled,
				Config: map[string]any{
					"draft": false,
				},
			},
		},
		Workflow: WorkflowConfig{
			RequireApproval:         true,
			AllowedBranches:         []string{"main", "master"},
			RequireCleanWorkingTree: true,
			RequireUpToDate:         false,
			DryRunByDefault:         false,
			AutoCommitChangelog:     true,
			ChangelogCommitMessage:  "chore(release): update changelog for ${version}",
		},
		Output: OutputConfig{
			Format:   "text",
			Color:    true,
			Verbose:  false,
			Quiet:    false,
			LogLevel: "info",
		},
		Telemetry: TelemetryConfig{
			Tracing: TracingConfig{
				Enabled:    false,
				Endpoint:   "localhost:4317",
				Insecure:   true,
				SampleRate: 1.0,
			},
			Metrics: MetricsConfig{
				Enabled: false,
				Port:    9090,
			},
		},
		Governance: GovernanceConfig{
			Enabled:                 false, // Disabled by default, opt-in for CGP
			StrictMode:              false, // Advisory mode by default
			AutoApproveThreshold:    0.3,   // Auto-approve changes with risk < 30%
			MaxAutoApproveRisk:      0.5,   // Never auto-approve risk > 50%
			RequireHumanForBreaking: true,  // Always require human for major bumps
			RequireHumanForSecurity: true,  // Always require human for security changes
			TrustedActors:           []string{},
			MemoryEnabled:           true,                              // Track historical data when governance is enabled
			MemoryPath:              ".relicta/governance/memory.json", // Default storage path
			Policies:                []GovernancePolicyConfig{},
		},
		Monorepo: MonorepoConfig{
			Enabled:                false, // Disabled by default, opt-in for monorepo mode
			Strategy:               MonorepoStrategyIndependent,
			PackagePaths:           []string{},
			ExcludePaths:           []string{},
			PackageOverrides:       map[string]PackageOverrideConfig{},
			ReleaseGroups:          []ReleaseGroupConfig{},
			RootPackage:            false,
			DependencyCoordination: true, // Coordinate internal dependency updates by default
			VersionFiles:           defaultVersionFiles(),
			Changelog: MonorepoChangelogConfig{
				PerPackage:          true, // Generate per-package changelogs
				RootChangelog:       true, // Also generate root changelog
				Format:              "conventional",
				IncludePackageLinks: true,
			},
		},
	}
}

// defaultVersionFiles returns the default version file configurations
// for common package types.
func defaultVersionFiles() map[string]VersionFileConfig {
	return map[string]VersionFileConfig{
		"npm": {
			File:   "package.json",
			Field:  "version",
			Update: true,
		},
		"cargo": {
			File:   "Cargo.toml",
			Field:  "version",
			Update: true,
		},
		"python": {
			Files:        []string{"pyproject.toml", "setup.py", "__version__.py"},
			Field:        "version",
			Pattern:      `__version__\s*=\s*["']([^"']+)["']`,
			Update:       true,
			UpdateFormat: `__version__ = "{{.Version}}"`,
		},
		"go_module": {
			File:   "go.mod",
			Update: false, // Go modules use git tags, not file versions
		},
		"maven": {
			File:   "pom.xml",
			Field:  "version",
			Update: true,
		},
		"gradle": {
			Files:        []string{"build.gradle", "build.gradle.kts"},
			Pattern:      `version\s*=\s*["']([^"']+)["']`,
			Update:       true,
			UpdateFormat: `version = "{{.Version}}"`,
		},
		"composer": {
			File:   "composer.json",
			Field:  "version",
			Update: true,
		},
		"gem": {
			Files:        []string{"*.gemspec", "lib/*/version.rb"},
			Pattern:      `VERSION\s*=\s*["']([^"']+)["']`,
			Update:       true,
			UpdateFormat: `VERSION = "{{.Version}}"`,
		},
		"nuget": {
			Files:  []string{"*.csproj", "*.fsproj"},
			Field:  "Version",
			Update: true,
		},
	}
}

// GitHubPluginConfig is the configuration for the GitHub plugin.
type GitHubPluginConfig struct {
	// Owner is the repository owner.
	Owner string `mapstructure:"owner" json:"owner,omitempty"`
	// Repo is the repository name.
	Repo string `mapstructure:"repo" json:"repo,omitempty"`
	// Token is the GitHub token (can use environment variable expansion).
	Token string `mapstructure:"token" json:"token,omitempty"`
	// Draft creates the release as a draft.
	Draft bool `mapstructure:"draft" json:"draft"`
	// Prerelease marks the release as a prerelease.
	Prerelease bool `mapstructure:"prerelease" json:"prerelease"`
	// GenerateReleaseNotes uses GitHub's auto-generated release notes.
	GenerateReleaseNotes bool `mapstructure:"generate_release_notes" json:"generate_release_notes"`
	// Assets is a list of files to upload as release assets.
	Assets []string `mapstructure:"assets" json:"assets,omitempty"`
	// DiscussionCategory creates a discussion for the release.
	DiscussionCategory string `mapstructure:"discussion_category" json:"discussion_category,omitempty"`
}

// NPMPluginConfig is the configuration for the npm plugin.
type NPMPluginConfig struct {
	// Registry is the npm registry URL.
	Registry string `mapstructure:"registry" json:"registry,omitempty"`
	// Tag is the npm dist-tag to use.
	Tag string `mapstructure:"tag" json:"tag,omitempty"`
	// Access is the package access level (public, restricted).
	Access string `mapstructure:"access" json:"access,omitempty"`
	// OTP is the one-time password for 2FA.
	OTP string `mapstructure:"otp" json:"otp,omitempty"`
	// DryRun performs a dry-run publish.
	DryRun bool `mapstructure:"dry_run" json:"dry_run"`
	// PackageDir is the directory containing package.json.
	PackageDir string `mapstructure:"package_dir" json:"package_dir,omitempty"`
}

// SlackPluginConfig is the configuration for the Slack plugin.
type SlackPluginConfig struct {
	// WebhookURL is the Slack webhook URL.
	WebhookURL string `mapstructure:"webhook" json:"webhook,omitempty"`
	// Channel is the channel to post to (overrides webhook default).
	Channel string `mapstructure:"channel" json:"channel,omitempty"`
	// Username is the bot username.
	Username string `mapstructure:"username" json:"username,omitempty"`
	// IconEmoji is the bot icon emoji.
	IconEmoji string `mapstructure:"icon_emoji" json:"icon_emoji,omitempty"`
	// IconURL is the bot icon URL.
	IconURL string `mapstructure:"icon_url" json:"icon_url,omitempty"`
	// NotifyOnSuccess sends notification on successful release.
	NotifyOnSuccess bool `mapstructure:"notify_on_success" json:"notify_on_success"`
	// NotifyOnError sends notification on failed release.
	NotifyOnError bool `mapstructure:"notify_on_error" json:"notify_on_error"`
	// IncludeChangelog includes changelog in the notification.
	IncludeChangelog bool `mapstructure:"include_changelog" json:"include_changelog"`
	// Mentions is a list of users/groups to mention.
	Mentions []string `mapstructure:"mentions" json:"mentions,omitempty"`
}

// DiscordPluginConfig is the configuration for the Discord plugin.
type DiscordPluginConfig struct {
	// WebhookURL is the Discord webhook URL (https://discord.com/api/webhooks/...).
	WebhookURL string `mapstructure:"webhook" json:"webhook,omitempty"`
	// Username is the bot username (overrides webhook default).
	Username string `mapstructure:"username" json:"username,omitempty"`
	// AvatarURL is the bot avatar URL (overrides webhook default).
	AvatarURL string `mapstructure:"avatar_url" json:"avatar_url,omitempty"`
	// NotifyOnSuccess sends notification on successful release.
	NotifyOnSuccess bool `mapstructure:"notify_on_success" json:"notify_on_success"`
	// NotifyOnError sends notification on failed release.
	NotifyOnError bool `mapstructure:"notify_on_error" json:"notify_on_error"`
	// IncludeChangelog includes changelog in the notification.
	IncludeChangelog bool `mapstructure:"include_changelog" json:"include_changelog"`
	// Mentions is a list of users/roles to mention (format: <@user_id> or <@&role_id>).
	Mentions []string `mapstructure:"mentions" json:"mentions,omitempty"`
	// ThreadID posts to a specific thread within the channel (optional).
	ThreadID string `mapstructure:"thread_id" json:"thread_id,omitempty"`
	// Color is the embed color in decimal (default varies by status).
	Color int `mapstructure:"color" json:"color,omitempty"`
}

// GitLabPluginConfig is the configuration for the GitLab plugin.
type GitLabPluginConfig struct {
	// BaseURL is the GitLab instance URL (default: https://gitlab.com).
	BaseURL string `mapstructure:"base_url" json:"base_url,omitempty"`
	// ProjectID is the GitLab project ID or path (e.g., "group/project").
	ProjectID string `mapstructure:"project_id" json:"project_id,omitempty"`
	// Token is the GitLab personal access token (can use environment variable expansion).
	Token string `mapstructure:"token" json:"token,omitempty"`
	// Name is the release name (default: "Release {version}").
	Name string `mapstructure:"name" json:"name,omitempty"`
	// Description is the release description (uses release notes if empty).
	Description string `mapstructure:"description" json:"description,omitempty"`
	// Ref is the tag ref for the release.
	Ref string `mapstructure:"ref" json:"ref,omitempty"`
	// ReleasedAt is the release date (ISO 8601 format).
	ReleasedAt string `mapstructure:"released_at" json:"released_at,omitempty"`
	// Milestones is a list of milestones to associate with the release.
	Milestones []string `mapstructure:"milestones" json:"milestones,omitempty"`
	// Assets is a list of files to upload as release assets.
	Assets []string `mapstructure:"assets" json:"assets,omitempty"`
	// AssetLinks is a list of external asset links.
	AssetLinks []GitLabAssetLink `mapstructure:"asset_links" json:"asset_links,omitempty"`
}

// GitLabAssetLink represents an external asset link for a GitLab release.
type GitLabAssetLink struct {
	// Name is the display name for the link.
	Name string `mapstructure:"name" json:"name"`
	// URL is the URL of the asset.
	URL string `mapstructure:"url" json:"url"`
	// FilePath is the direct asset path within the release.
	FilePath string `mapstructure:"filepath" json:"filepath,omitempty"`
	// LinkType is the type of link (other, runbook, image, package).
	LinkType string `mapstructure:"link_type" json:"link_type,omitempty"`
}

// BlastRadiusConfig is the configuration for blast radius analysis in monorepos.
type BlastRadiusConfig struct {
	// Enabled indicates whether blast radius analysis is enabled.
	Enabled bool `mapstructure:"enabled" json:"enabled"`
	// PackagePaths is a list of glob patterns for package locations.
	PackagePaths []string `mapstructure:"package_paths" json:"package_paths,omitempty"`
	// ExcludePaths is a list of paths to exclude from analysis.
	ExcludePaths []string `mapstructure:"exclude_paths" json:"exclude_paths,omitempty"`
	// SharedDirs lists directories containing shared code.
	SharedDirs []string `mapstructure:"shared_dirs" json:"shared_dirs,omitempty"`
	// RootPackage indicates if the root directory is also a package.
	RootPackage bool `mapstructure:"root_package" json:"root_package"`
	// IncludeTransitive includes transitive dependency impacts.
	IncludeTransitive bool `mapstructure:"include_transitive" json:"include_transitive"`
	// CalculateRisk calculates risk scores for impacts.
	CalculateRisk bool `mapstructure:"calculate_risk" json:"calculate_risk"`
	// MaxTransitiveDepth limits transitive dependency depth (0 = unlimited).
	MaxTransitiveDepth int `mapstructure:"max_transitive_depth" json:"max_transitive_depth"`
	// IgnoreDevDependencies excludes dev dependencies from analysis.
	IgnoreDevDependencies bool `mapstructure:"ignore_dev_dependencies" json:"ignore_dev_dependencies"`
}

// MonorepoStrategy defines the versioning strategy for monorepos.
type MonorepoStrategy string

const (
	// MonorepoStrategyIndependent allows each package to have its own version.
	MonorepoStrategyIndependent MonorepoStrategy = "independent"
	// MonorepoStrategyLockstep keeps all packages at the same version.
	MonorepoStrategyLockstep MonorepoStrategy = "lockstep"
	// MonorepoStrategyHybrid allows groups of packages with different strategies.
	MonorepoStrategyHybrid MonorepoStrategy = "hybrid"
)

// MonorepoConfig configures multi-package/monorepo versioning support.
// This enables independent versioning for packages within a monorepo,
// coordinated multi-package releases, and automatic version file updates.
type MonorepoConfig struct {
	// Enabled indicates whether monorepo mode is enabled.
	Enabled bool `mapstructure:"enabled" json:"enabled"`
	// Strategy is the versioning strategy (independent, lockstep, hybrid).
	// - independent: Each package has its own version.
	// - lockstep: All packages share the same version.
	// - hybrid: Groups of packages can have different strategies.
	Strategy MonorepoStrategy `mapstructure:"strategy" json:"strategy"`
	// PackagePaths is a list of glob patterns for package locations.
	// These extend the blast_radius package paths for versioning purposes.
	// Example: ["packages/*", "plugins/*"]
	PackagePaths []string `mapstructure:"package_paths" json:"package_paths,omitempty"`
	// ExcludePaths is a list of paths to exclude from package discovery.
	// Example: ["packages/internal-*"]
	ExcludePaths []string `mapstructure:"exclude_paths" json:"exclude_paths,omitempty"`
	// PackageOverrides provides per-package configuration overrides.
	// Keys are package paths relative to repository root.
	PackageOverrides map[string]PackageOverrideConfig `mapstructure:"package_overrides" json:"package_overrides,omitempty"`
	// VersionFiles configures how to detect and update version files.
	VersionFiles map[string]VersionFileConfig `mapstructure:"version_files" json:"version_files,omitempty"`
	// ReleaseGroups configures coordinated release groups.
	// Packages in the same group can be released together with a shared strategy.
	ReleaseGroups []ReleaseGroupConfig `mapstructure:"release_groups" json:"release_groups,omitempty"`
	// Changelog configures per-package and root changelog generation.
	Changelog MonorepoChangelogConfig `mapstructure:"changelog" json:"changelog,omitempty"`
	// RootPackage indicates if the root directory is also a package.
	RootPackage bool `mapstructure:"root_package" json:"root_package"`
	// DependencyCoordination enables automatic dependency version updates
	// when releasing packages with internal dependencies.
	DependencyCoordination bool `mapstructure:"dependency_coordination" json:"dependency_coordination"`
}

// PackageOverrideConfig provides per-package configuration overrides.
type PackageOverrideConfig struct {
	// TagPrefix overrides the tag prefix for this package.
	// Example: "core-v" produces tags like "core-v1.2.3"
	TagPrefix string `mapstructure:"tag_prefix" json:"tag_prefix,omitempty"`
	// VersionFile specifies the version file to update for this package.
	// Example: "package.json" or "Cargo.toml"
	VersionFile string `mapstructure:"version_file" json:"version_file,omitempty"`
	// VersionField specifies the field in the version file to update.
	// Example: "version" for package.json
	VersionField string `mapstructure:"version_field" json:"version_field,omitempty"`
	// ChangelogFile specifies a custom changelog file for this package.
	// Defaults to "CHANGELOG.md" within the package directory.
	ChangelogFile string `mapstructure:"changelog_file" json:"changelog_file,omitempty"`
	// Private indicates this package should not be published.
	Private bool `mapstructure:"private" json:"private"`
	// PrereleaseSuffix overrides the prerelease suffix for this package.
	PrereleaseSuffix string `mapstructure:"prerelease_suffix" json:"prerelease_suffix,omitempty"`
	// SkipVersioning disables versioning for this package.
	SkipVersioning bool `mapstructure:"skip_versioning" json:"skip_versioning"`
}

// VersionFileConfig configures how to detect and update version files.
type VersionFileConfig struct {
	// File is the version file name (e.g., "package.json", "Cargo.toml").
	File string `mapstructure:"file" json:"file"`
	// Files is a list of version file names to check (for types with multiple options).
	// Example for Python: ["setup.py", "pyproject.toml", "__version__.py"]
	Files []string `mapstructure:"files" json:"files,omitempty"`
	// Field is the field name containing the version (e.g., "version").
	Field string `mapstructure:"field" json:"field,omitempty"`
	// Pattern is a regex pattern to match and extract version.
	// Used for files without structured format (e.g., __version__.py).
	Pattern string `mapstructure:"pattern" json:"pattern,omitempty"`
	// Update indicates whether to update the version in this file.
	// Set to false for formats that use git tags only (like go.mod).
	Update bool `mapstructure:"update" json:"update"`
	// UpdateFormat is a template for how to write the version.
	// Example: "__version__ = '{{.Version}}'"
	UpdateFormat string `mapstructure:"update_format" json:"update_format,omitempty"`
}

// ReleaseGroupConfig configures a coordinated release group.
// Packages in a group can be released together with a shared strategy.
type ReleaseGroupConfig struct {
	// Name is the unique name for this release group.
	Name string `mapstructure:"name" json:"name"`
	// Packages is a list of package paths or glob patterns in this group.
	// Example: ["packages/core", "packages/shared"] or ["plugins/*"]
	Packages []string `mapstructure:"packages" json:"packages"`
	// Strategy is the versioning strategy for this group.
	// - lockstep: All packages in group share the same version.
	// - independent: Packages in group have independent versions.
	Strategy MonorepoStrategy `mapstructure:"strategy" json:"strategy"`
	// TagPrefix is the tag prefix for lockstep releases.
	// Example: "core-v" produces tags like "core-v1.2.3"
	TagPrefix string `mapstructure:"tag_prefix" json:"tag_prefix,omitempty"`
	// SharedChangelog indicates whether packages share a single changelog.
	SharedChangelog bool `mapstructure:"shared_changelog" json:"shared_changelog"`
}

// MonorepoChangelogConfig configures changelog generation for monorepos.
type MonorepoChangelogConfig struct {
	// PerPackage generates individual changelogs for each package.
	PerPackage bool `mapstructure:"per_package" json:"per_package"`
	// RootChangelog generates a root-level CHANGELOG.md aggregating all changes.
	RootChangelog bool `mapstructure:"root_changelog" json:"root_changelog"`
	// Format is the changelog format (conventional, keep-a-changelog, custom).
	Format string `mapstructure:"format" json:"format,omitempty"`
	// IncludePackageLinks includes links between package changelogs.
	IncludePackageLinks bool `mapstructure:"include_package_links" json:"include_package_links"`
}

// JiraPluginConfig is the configuration for the Jira plugin.
type JiraPluginConfig struct {
	// BaseURL is the Jira instance URL (e.g., "https://your-domain.atlassian.net").
	BaseURL string `mapstructure:"base_url" json:"base_url,omitempty"`
	// Username is the Jira username (email for Jira Cloud).
	Username string `mapstructure:"username" json:"username,omitempty"`
	// Token is the Jira API token (can use environment variable expansion).
	Token string `mapstructure:"token" json:"token,omitempty"`
	// ProjectKey is the Jira project key (e.g., "PROJ").
	ProjectKey string `mapstructure:"project_key" json:"project_key,omitempty"`
	// IssuePattern is a regex pattern to extract issue keys from commits (default: `[A-Z][A-Z0-9]*-\d+`).
	IssuePattern string `mapstructure:"issue_pattern" json:"issue_pattern,omitempty"`
	// CreateVersion creates a version in Jira for the release.
	CreateVersion bool `mapstructure:"create_version" json:"create_version"`
	// ReleaseVersion marks the version as released.
	ReleaseVersion bool `mapstructure:"release_version" json:"release_version"`
	// UpdateFixVersion adds the version to fix version of linked issues.
	UpdateFixVersion bool `mapstructure:"update_fix_version" json:"update_fix_version"`
	// TransitionIssues transitions issues to a specified status.
	TransitionIssues bool `mapstructure:"transition_issues" json:"transition_issues"`
	// TransitionName is the name of the transition to apply (e.g., "Done", "Released").
	TransitionName string `mapstructure:"transition_name" json:"transition_name,omitempty"`
	// AddComment adds a comment to linked issues.
	AddComment bool `mapstructure:"add_comment" json:"add_comment"`
	// CommentTemplate is the comment template (supports {{.Version}}, {{.Repository}}, {{.ReleaseURL}}).
	CommentTemplate string `mapstructure:"comment_template" json:"comment_template,omitempty"`
	// VersionPrefix is a prefix for the Jira version name (e.g., "v").
	VersionPrefix string `mapstructure:"version_prefix" json:"version_prefix,omitempty"`
	// VersionDescription is a description template for the Jira version.
	VersionDescription string `mapstructure:"version_description" json:"version_description,omitempty"`
}

// WebhookConfig configures a webhook endpoint for release event notifications.
type WebhookConfig struct {
	// Name is a friendly name for this webhook.
	Name string `mapstructure:"name" json:"name"`
	// URL is the webhook endpoint URL.
	URL string `mapstructure:"url" json:"url"`
	// Secret is an optional HMAC secret for signing payloads.
	// When set, payloads are signed with X-Relicta-Signature header.
	Secret string `mapstructure:"secret" json:"secret,omitempty"`
	// Events is a list of event names to send (empty = all events).
	// Event names: release.initialized, release.planned, release.versioned,
	// release.notes_generated, release.approved, release.published,
	// release.failed, release.canceled, release.plugin_executed
	// Supports wildcards like "release.*" for all release events.
	Events []string `mapstructure:"events" json:"events,omitempty"`
	// Headers are custom headers to include in the request.
	Headers map[string]string `mapstructure:"headers" json:"headers,omitempty"`
	// Timeout is the HTTP request timeout (default: 10s).
	Timeout time.Duration `mapstructure:"timeout" json:"timeout,omitempty"`
	// RetryCount is the number of retries for failed requests (default: 3).
	RetryCount int `mapstructure:"retry_count" json:"retry_count,omitempty"`
	// RetryDelay is the delay between retries (default: 1s).
	RetryDelay time.Duration `mapstructure:"retry_delay" json:"retry_delay,omitempty"`
	// Enabled indicates whether this webhook is active (default: true).
	Enabled *bool `mapstructure:"enabled" json:"enabled,omitempty"`
}

// IsWebhookEnabled returns whether the webhook is enabled.
func (w *WebhookConfig) IsWebhookEnabled() bool {
	if w.Enabled == nil {
		return true
	}
	return *w.Enabled
}

// ConfigFileNames to search for.
// Only .relicta.{yaml,yml,json,toml} is supported for consistency
// with Go ecosystem conventions (.goreleaser.yaml, .golangci.yml, etc.).
var ConfigFileNames = []string{
	".relicta",
}

// ConfigFileExtensions supported by Viper.
var ConfigFileExtensions = []string{
	"yaml",
	"yml",
	"json",
	"toml",
}
