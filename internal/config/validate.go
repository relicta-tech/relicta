// Package config provides configuration management for ReleasePilot.
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	rperrors "github.com/felixgeelhaar/release-pilot/internal/errors"
)

// ValidationError contains all validation errors.
type ValidationError struct {
	Errors []string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("configuration validation failed:\n  - %s", strings.Join(e.Errors, "\n  - "))
}

// HasErrors returns true if there are validation errors.
func (e *ValidationError) HasErrors() bool {
	return len(e.Errors) > 0
}

// Addf adds a formatted error to the validation error.
func (e *ValidationError) Addf(format string, args ...any) {
	e.Errors = append(e.Errors, fmt.Sprintf(format, args...))
}

// Validator validates configuration.
type Validator struct {
	errors *ValidationError
}

// NewValidator creates a new configuration validator.
func NewValidator() *Validator {
	return &Validator{
		errors: &ValidationError{},
	}
}

// Validate validates the configuration.
func (v *Validator) Validate(cfg *Config) error {
	v.validateVersioning(cfg.Versioning)
	v.validateChangelog(cfg.Changelog)
	v.validateAI(cfg.AI)
	v.validatePlugins(cfg.Plugins)
	v.validateWorkflow(cfg.Workflow)
	v.validateOutput(cfg.Output)

	if v.errors.HasErrors() {
		return rperrors.Validation("config.Validate", v.errors.Error())
	}

	return nil
}

// validateVersioning validates versioning configuration.
func (v *Validator) validateVersioning(cfg VersioningConfig) {
	// Validate strategy
	validStrategies := []string{"conventional", "manual"}
	if !slices.Contains(validStrategies, cfg.Strategy) {
		v.errors.Addf("versioning.strategy: must be one of %v, got %q", validStrategies, cfg.Strategy)
	}

	// Validate bump_from
	validBumpFrom := []string{"tag", "file", "package.json"}
	if !slices.Contains(validBumpFrom, cfg.BumpFrom) {
		v.errors.Addf("versioning.bump_from: must be one of %v, got %q", validBumpFrom, cfg.BumpFrom)
	}

	// If bump_from is file, version_file must be specified
	if cfg.BumpFrom == "file" && cfg.VersionFile == "" {
		v.errors.Addf("versioning.version_file: required when bump_from is 'file'")
	}

	// Note: Empty tag_prefix is valid (some repos use tags without prefix)
}

// validateChangelog validates changelog configuration.
func (v *Validator) validateChangelog(cfg ChangelogConfig) {
	// Validate format
	validFormats := []string{"keep-a-changelog", "conventional", "custom"}
	if !slices.Contains(validFormats, cfg.Format) {
		v.errors.Addf("changelog.format: must be one of %v, got %q", validFormats, cfg.Format)
	}

	// Validate group_by
	validGroupBy := []string{"type", "scope", "none"}
	if !slices.Contains(validGroupBy, cfg.GroupBy) {
		v.errors.Addf("changelog.group_by: must be one of %v, got %q", validGroupBy, cfg.GroupBy)
	}

	// If format is custom, template must be specified
	if cfg.Format == "custom" && cfg.Template == "" {
		v.errors.Addf("changelog.template: required when format is 'custom'")
	}

	// Validate template file exists if specified
	if cfg.Template != "" {
		if _, err := os.Stat(cfg.Template); os.IsNotExist(err) {
			v.errors.Addf("changelog.template: file does not exist: %s", cfg.Template)
		}
	}

	// Validate URLs if link options are enabled
	if cfg.LinkCommits && cfg.RepositoryURL != "" {
		if _, err := url.Parse(cfg.RepositoryURL); err != nil {
			v.errors.Addf("changelog.repository_url: invalid URL: %s", cfg.RepositoryURL)
		}
	}

	if cfg.LinkIssues && cfg.IssueURL != "" {
		if _, err := url.Parse(cfg.IssueURL); err != nil {
			v.errors.Addf("changelog.issue_url: invalid URL: %s", cfg.IssueURL)
		}
	}

	// Validate changelog file path
	// Note: If changelog directory doesn't exist, it will be created when needed
}

// validateAI validates AI configuration.
func (v *Validator) validateAI(cfg AIConfig) {
	if !cfg.Enabled {
		return // Skip validation if AI is disabled
	}

	// Validate provider
	validProviders := []string{"openai"}
	if !slices.Contains(validProviders, cfg.Provider) {
		v.errors.Addf("ai.provider: must be one of %v, got %q", validProviders, cfg.Provider)
	}

	// Validate model
	if cfg.Model == "" {
		v.errors.Addf("ai.model: required when AI is enabled")
	}

	// Validate API key is provided (after env expansion)
	if cfg.APIKey == "" {
		// Check if it's provided via environment variable
		if os.Getenv("OPENAI_API_KEY") == "" && os.Getenv("RELEASE_PILOT_AI_API_KEY") == "" {
			v.errors.Addf("ai.api_key: required when AI is enabled (set via config or OPENAI_API_KEY env var)")
		}
	}

	// Validate tone
	validTones := []string{"technical", "friendly", "professional", "excited"}
	if !slices.Contains(validTones, cfg.Tone) {
		v.errors.Addf("ai.tone: must be one of %v, got %q", validTones, cfg.Tone)
	}

	// Validate audience
	validAudiences := []string{"developers", "users", "public", "marketing"}
	if !slices.Contains(validAudiences, cfg.Audience) {
		v.errors.Addf("ai.audience: must be one of %v, got %q", validAudiences, cfg.Audience)
	}

	// Validate temperature
	if cfg.Temperature < 0 || cfg.Temperature > 2 {
		v.errors.Addf("ai.temperature: must be between 0 and 2, got %f", cfg.Temperature)
	}

	// Validate max_tokens
	if cfg.MaxTokens < 1 || cfg.MaxTokens > 128000 {
		v.errors.Addf("ai.max_tokens: must be between 1 and 128000, got %d", cfg.MaxTokens)
	}

	// Validate timeout
	if cfg.Timeout <= 0 {
		v.errors.Addf("ai.timeout: must be positive")
	}

	// Validate retry_attempts
	if cfg.RetryAttempts < 0 {
		v.errors.Addf("ai.retry_attempts: must be non-negative, got %d", cfg.RetryAttempts)
	}

	// Validate base_url if provided
	if cfg.BaseURL != "" {
		if _, err := url.Parse(cfg.BaseURL); err != nil {
			v.errors.Addf("ai.base_url: invalid URL: %s", cfg.BaseURL)
		}
	}
}

// validatePlugins validates plugin configurations.
func (v *Validator) validatePlugins(plugins []PluginConfig) {
	seenNames := make(map[string]bool)

	for i, plugin := range plugins {
		// Validate name
		if plugin.Name == "" {
			v.errors.Addf("plugins[%d].name: required", i)
			continue
		}

		// Check for duplicates
		if seenNames[plugin.Name] {
			v.errors.Addf("plugins[%d].name: duplicate plugin name %q", i, plugin.Name)
		}
		seenNames[plugin.Name] = true

		// Validate path if specified
		if plugin.Path != "" {
			if _, err := os.Stat(plugin.Path); os.IsNotExist(err) {
				v.errors.Addf("plugins[%d].path: file does not exist: %s", i, plugin.Path)
			}
		}

		// Validate timeout
		if plugin.Timeout < 0 {
			v.errors.Addf("plugins[%d].timeout: must be non-negative", i)
		}

		// Validate hooks if specified
		validHooks := []string{
			"pre_init", "post_init",
			"pre_plan", "post_plan",
			"pre_version", "post_version",
			"pre_notes", "post_notes",
			"pre_approve", "post_approve",
			"pre_publish", "post_publish",
			"on_success", "on_error",
		}
		for _, hook := range plugin.Hooks {
			if !slices.Contains(validHooks, hook) {
				v.errors.Addf("plugins[%d].hooks: invalid hook %q, must be one of %v", i, hook, validHooks)
			}
		}

		// Plugin-specific validation
		v.validatePluginConfig(i, plugin)
	}
}

// validatePluginConfig validates plugin-specific configuration.
func (v *Validator) validatePluginConfig(index int, plugin PluginConfig) {
	switch plugin.Name {
	case "github":
		v.validateGitHubPlugin(index, plugin.Config)
	case "npm":
		v.validateNPMPlugin(index, plugin.Config)
	case "slack":
		v.validateSlackPlugin(index, plugin.Config)
	}
}

// validateGitHubPlugin validates GitHub plugin configuration.
func (v *Validator) validateGitHubPlugin(index int, config map[string]any) {
	if config == nil {
		return
	}

	// Validate owner and repo if provided
	if owner, ok := config["owner"].(string); ok && owner == "" {
		v.errors.Addf("plugins[%d].config.owner: cannot be empty", index)
	}
	if repo, ok := config["repo"].(string); ok && repo == "" {
		v.errors.Addf("plugins[%d].config.repo: cannot be empty", index)
	}

	// Token validation is optional - user might set it via GITHUB_TOKEN environment variable
}

// validateNPMPlugin validates npm plugin configuration.
func (v *Validator) validateNPMPlugin(index int, config map[string]any) {
	if config == nil {
		return
	}

	// Validate access
	if access, ok := config["access"].(string); ok {
		validAccess := []string{"public", "restricted", ""}
		if !slices.Contains(validAccess, access) {
			v.errors.Addf("plugins[%d].config.access: must be 'public' or 'restricted', got %q", index, access)
		}
	}

	// Validate registry URL
	if registry, ok := config["registry"].(string); ok && registry != "" {
		if _, err := url.Parse(registry); err != nil {
			v.errors.Addf("plugins[%d].config.registry: invalid URL: %s", index, registry)
		}
	}
}

// validateSlackPlugin validates Slack plugin configuration.
func (v *Validator) validateSlackPlugin(index int, config map[string]any) {
	if config == nil {
		return
	}

	// Webhook URL is required
	webhook, _ := config["webhook"].(string)
	if webhook == "" && os.Getenv("SLACK_WEBHOOK_URL") == "" {
		v.errors.Addf("plugins[%d].config.webhook: required for Slack plugin", index)
	}

	// Validate webhook URL format
	if webhook != "" {
		if _, err := url.Parse(webhook); err != nil {
			v.errors.Addf("plugins[%d].config.webhook: invalid URL: %s", index, webhook)
		}
	}
}

// validateWorkflow validates workflow configuration.
func (v *Validator) validateWorkflow(cfg WorkflowConfig) {
	// Validate allowed_branches
	// Note: Having no branch restrictions with approval required is valid
	// Note: Hook commands are validated at runtime

	// Validate changelog_commit_message
	if cfg.AutoCommitChangelog && cfg.ChangelogCommitMessage == "" {
		v.errors.Addf("workflow.changelog_commit_message: required when auto_commit_changelog is enabled")
	}
}

// validateOutput validates output configuration.
func (v *Validator) validateOutput(cfg OutputConfig) {
	// Validate format
	validFormats := []string{"text", "json", "yaml"}
	if !slices.Contains(validFormats, cfg.Format) {
		v.errors.Addf("output.format: must be one of %v, got %q", validFormats, cfg.Format)
	}

	// Validate log_level
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !slices.Contains(validLogLevels, cfg.LogLevel) {
		v.errors.Addf("output.log_level: must be one of %v, got %q", validLogLevels, cfg.LogLevel)
	}

	// Quiet and verbose are mutually exclusive
	if cfg.Quiet && cfg.Verbose {
		v.errors.Addf("output: quiet and verbose cannot both be enabled")
	}

	// Validate log_file directory exists
	if cfg.LogFile != "" {
		dir := filepath.Dir(cfg.LogFile)
		if dir != "." && dir != "" {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				v.errors.Addf("output.log_file: directory does not exist: %s", dir)
			}
		}
	}
}

// Validate is a convenience function to validate configuration.
func Validate(cfg *Config) error {
	return NewValidator().Validate(cfg)
}

// ValidateAndLoad loads and validates configuration.
func ValidateAndLoad() (*Config, error) {
	cfg, err := NewLoader().Load()
	if err != nil {
		return nil, err
	}

	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
