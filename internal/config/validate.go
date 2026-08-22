// Package config provides configuration management for Relicta.
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	rperrors "github.com/relicta-tech/relicta/v4/internal/errors"
)

// openAIKeyLength is the standard length of OpenAI API keys (e.g., "sk-..." format).
const openAIKeyLength = 51

// isOpenAIKeyFormat checks if a string appears to be an OpenAI API key format.
// OpenAI keys are 51 characters long and start with "sk-".
// Returns false for environment variable references (${...}).
func isOpenAIKeyFormat(key string) bool {
	return key != "" &&
		!strings.HasPrefix(key, "${") &&
		len(key) == openAIKeyLength &&
		strings.HasPrefix(key, "sk-")
}

// ValidationError contains all validation errors and warnings.
type ValidationError struct {
	Errors   []string
	Warnings []string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	var parts []string

	if len(e.Errors) > 0 {
		parts = append(parts, fmt.Sprintf("Errors:\n  - %s", strings.Join(e.Errors, "\n  - ")))
	}

	if len(e.Warnings) > 0 {
		parts = append(parts, fmt.Sprintf("Warnings:\n  - %s", strings.Join(e.Warnings, "\n  - ")))
	}

	return fmt.Sprintf("configuration validation failed:\n%s", strings.Join(parts, "\n"))
}

// HasErrors returns true if there are validation errors.
func (e *ValidationError) HasErrors() bool {
	return len(e.Errors) > 0
}

// HasWarnings returns true if there are validation warnings.
func (e *ValidationError) HasWarnings() bool {
	return len(e.Warnings) > 0
}

// Addf adds a formatted error to the validation error.
func (e *ValidationError) Addf(format string, args ...any) {
	e.Errors = append(e.Errors, fmt.Sprintf(format, args...))
}

// Warnf adds a formatted warning to the validation error.
func (e *ValidationError) Warnf(format string, args ...any) {
	e.Warnings = append(e.Warnings, fmt.Sprintf(format, args...))
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
	v.validatePersistence(cfg.Persistence)
	v.validateMonorepo(cfg.Monorepo)
	v.validateTelemetry(cfg.Telemetry)
	v.validateAttestation(cfg.Attestation)
	v.validatePluginSecurity(cfg.PluginSecurity)
	v.validateChannels(cfg.Channels)

	// Print warnings to stderr even if there are no errors
	if v.errors.HasWarnings() {
		fmt.Fprintf(os.Stderr, "\n⚠️  Configuration Warnings:\n")
		for _, warning := range v.errors.Warnings {
			fmt.Fprintf(os.Stderr, "  - %s\n", warning)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

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

	// If bump_from is file, a version file must be specified — either the
	// deprecated single version_file or the version_files list.
	if cfg.BumpFrom == "file" && len(cfg.ResolvedVersionFiles()) == 0 {
		v.errors.Addf("versioning.version_files: required when bump_from is 'file'")
	}

	v.validateVersionFiles(cfg.VersionFiles)

	// prerelease_suffix is read by nothing, and says so rather than sitting there looking
	// like it works.
	//
	// A warning rather than an error, because a project that has it set is not misconfigured
	// — it is doing something that has no effect, and failing their release over that would
	// be a worse trade than telling them. The precedent is the deprecated ai.provider value
	// a few functions down.
	//
	// Not simply wired up, which was tried and rejected twice. Read literally — every bump
	// becomes a prerelease — a project could never cut a stable release through `bump` again:
	// measured, 1.3.0-beta.1 bumps to 1.3.0-beta.2 and never to 1.3.0. Wired as the default
	// for a bare `--prerelease`, it required pflag's NoOptDefVal, which silently stops
	// `--prerelease beta` and `-p beta` from binding their value: an unread setting would
	// have become one that overrides an explicit flag. The capability already exists and is
	// reachable, so the honest fix is to point at it.
	if cfg.PrereleaseSuffix != "" {
		v.errors.Warnf("versioning.prerelease_suffix: has no effect — nothing reads it. " +
			"Use --prerelease (e.g. 'relicta bump --prerelease rc') or --channel for a " +
			"single release, and 'relicta promote' to graduate one to stable")
	}

	// Note: Empty tag_prefix is valid (some repos use tags without prefix)
}

// validateMonorepo refuses the parts of the monorepo block that nothing implements.
//
// The section used to be read by nothing at all: a repository with `enabled: true` and two
// packages at different versions was given a single repository-wide 0.0.0 -> 0.1.0, and neither
// package.json was touched. `strategy: independent` now versions each package from its own
// commits and writes its own manifest.
//
// lockstep, hybrid and release_groups are not wired. They are errors rather than warnings, and
// that asymmetry with the old warning is deliberate: a warning was right when the whole section
// did nothing, because the release still behaved as the user's repository-wide config said it
// would. Now the same file gets per-package versioning for one strategy and repository-wide
// versioning for the others, and silently giving somebody who asked for lockstep the opposite of
// lockstep is worse than refusing to start.
func (v *Validator) validateMonorepo(cfg MonorepoConfig) {
	if !cfg.Enabled {
		return
	}

	switch cfg.Strategy {
	case MonorepoStrategyIndependent:
		if len(cfg.PackagePaths) == 0 {
			v.errors.Addf("monorepo.package_paths: required when monorepo.enabled is true — " +
				"without globs there is nothing to version per package, for example [\"packages/*\"]")
		}
	case MonorepoStrategyLockstep, MonorepoStrategyHybrid:
		v.errors.Addf("monorepo.strategy: %q is not implemented yet; only %q is. Packages would "+
			"otherwise be versioned independently, which is the opposite of what you asked for",
			cfg.Strategy, MonorepoStrategyIndependent)
	default:
		v.errors.Addf("monorepo.strategy: must be %q, got %q", MonorepoStrategyIndependent, cfg.Strategy)
	}

	if len(cfg.ReleaseGroups) > 0 {
		v.errors.Addf("monorepo.release_groups: not implemented yet — every package is released " +
			"on its own commits, so a group would be silently ignored")
	}

	// version_files ships with a default map naming the manifests each package type carries,
	// and that default agrees with what the writers detect — so it is only wrong when somebody
	// changes it, expecting the change to take effect.
	// An absent map is not a customized one: nothing reads this, so config loading leaves it
	// empty unless the file says otherwise.
	if len(cfg.VersionFiles) > 0 && !reflect.DeepEqual(cfg.VersionFiles, defaultVersionFiles()) {
		v.errors.Addf("monorepo.version_files: customizing this is not implemented yet — a " +
			"package's version is read from and written to the manifest in its own directory " +
			"(package.json, Cargo.toml, pyproject.toml, go.mod and the rest), which is detected " +
			"rather than configured")
	}
	if cfg.DependencyCoordination {
		v.errors.Addf("monorepo.dependency_coordination: not implemented yet — releasing a " +
			"package does not update the versions its dependants pin, so leaving this on would " +
			"promise a coordination that does not happen")
	}
	if cfg.Changelog.IncludePackageLinks {
		v.errors.Addf("monorepo.changelog.include_package_links: not implemented yet — a " +
			"package's changelog entry carries its commits, not links between packages")
	}

	for path, override := range cfg.PackageOverrides {
		if override.VersionFile != "" || override.VersionField != "" {
			v.errors.Addf("monorepo.package_overrides.%s: version_file and version_field are not "+
				"implemented yet — the manifest in the package's own directory is what carries "+
				"its version. tag_prefix, changelog_file and skip_versioning are honored", path)
		}
	}
}

// validateChannels reports the per-channel approval settings that nothing performs.
//
// `relicta promote` builds its channel registry from name, stability, tag_pattern, promotes_to
// and prerelease — and not from require_approval or auto_approve, which nothing reads.
// ChannelDefinitionConfig.NeedsApproval() exists to answer the question and has no caller.
//
// Warned rather than refused: promote has no approval step at all to attach them to, so this is
// a feature that has not been built rather than a setting wired to the wrong place. Refusing
// would stop a promotion that works today over a gate that never existed.
func (v *Validator) validateChannels(cfg ChannelsConfig) {
	if !cfg.Enabled {
		return
	}

	for _, def := range cfg.Definitions {
		if def.RequireApproval != nil || (def.AutoApprove != nil && *def.AutoApprove) {
			v.errors.Warnf("channels.definitions[%s]: require_approval and auto_approve are "+
				"not read — `relicta promote` has no approval step, so a promotion to this "+
				"channel is neither gated nor auto-approved by these settings", def.Name)
			return
		}
	}
}

// validatePluginSecurity reports the plugin-security settings that nothing performs.
//
// plugin_security.auto_install promises to install missing required plugins before a release runs. Nothing
// reads it, and installing a plugin means fetching and executing code — so this is refused
// rather than quietly implemented: a release tool that starts downloading executables because
// a config key looked plausible is not a decision to make on somebody's behalf.
func (v *Validator) validatePluginSecurity(cfg PluginSecurityConfig) {
	// A *bool: unset means the documented default (on when Required is non-empty), and only
	// an explicit true is somebody asking for it.
	if cfg.AutoInstall != nil && *cfg.AutoInstall {
		v.errors.Warnf("plugin_security.auto_install: not implemented — a missing plugin is " +
			"reported rather than fetched. Installing one means running code that is not in " +
			"the repository, which relicta does not do without being asked each time")
	}
}

// validateAttestation checks the signing configuration before a release starts.
//
// Every problem here used to surface during publish, from inside the attestation step: keyless
// mode fails with "keyless signing requires sigstore-go", and local mode with no key fails with
// "key_path is required". By then the tag exists and the release is half done — and unless
// attestation.required is set the failure is a warning, so the operator gets a release with no
// attestation and a line of log saying why.
//
// Startup is where a configuration problem belongs. `attestation.enabled` gates all of it: a
// repository that has not asked for attestations is not told about signing modes.
func (v *Validator) validateAttestation(cfg AttestationConfig) {
	if !cfg.Enabled {
		return
	}

	switch cfg.SigningMode {
	case "", "none":
		// Unsigned attestations: a provenance record without a signature is still a record,
		// and this is the default.

	case "local":
		if cfg.KeyPath == "" {
			v.errors.Addf("attestation.key_path: required when signing_mode is \"local\" — " +
				"without it signing fails partway through publish, after the tag exists")
		}

	case "keyless":
		v.errors.Addf("attestation.signing_mode: \"keyless\" is not implemented — it needs " +
			"sigstore-go, and signing fails partway through publish today. Use \"local\" with " +
			"a key_path, or \"none\" for an unsigned attestation")

	default:
		v.errors.Addf("attestation.signing_mode: must be \"keyless\", \"local\" or \"none\", got %q",
			cfg.SigningMode)
	}

	// The Sigstore endpoints only mean anything to the mode that is not implemented.
	if cfg.RekorURL != "" || cfg.FulcioURL != "" {
		v.errors.Warnf("attestation.rekor_url and fulcio_url configure keyless signing, which " +
			"is not implemented; they are read by nothing")
	}
}

// validateTelemetry reports the telemetry settings that describe an export nothing performs.
//
// The whole block was read by nothing. Metrics are now served — `relicta metrics` takes its
// port and path from here, and the dashboard exposes them when enabled — but tracing has no
// OTLP exporter behind it: InitTracer builds a logging tracer and says so in a comment, and
// the endpoint, headers, TLS setting and sample rate describe a connection no code can make.
//
// Warnings rather than errors, on the same reasoning as the original monorepo warning: a
// project with these set is not misconfigured, it is expecting something that does not happen
// yet, and refusing to start would take away the tracing it does get.
func (v *Validator) validateTelemetry(cfg TelemetryConfig) {
	if !cfg.Tracing.Enabled {
		return
	}

	if cfg.Tracing.Endpoint != "" {
		v.errors.Warnf("telemetry.tracing.endpoint: no OTLP exporter is implemented, so spans "+
			"are written to the log rather than sent to %s. tracing.enabled does install the "+
			"tracer; the endpoint, headers, insecure and sample_rate settings are not read",
			cfg.Tracing.Endpoint)
		return
	}

	if len(cfg.Tracing.Headers) > 0 || cfg.Tracing.Insecure || cfg.Tracing.SampleRate > 0 {
		v.errors.Warnf("telemetry.tracing: headers, insecure and sample_rate configure an OTLP " +
			"export that is not implemented; spans are written to the log")
	}
}

// validateVersionFiles checks each version target. Catching these at load time
// matters more than usual: the alternative is discovering a bad format or a
// missing key partway through a release.
func (v *Validator) validateVersionFiles(targets []VersionTarget) {
	validFormats := []string{
		string(VersionFormatSemver),
		string(VersionFormatSemverBuild),
		string(VersionFormatInteger),
		string(VersionFormatTemplate),
	}
	validStrategies := []string{string(StrategyReplace), string(StrategyIncrement)}

	seen := make(map[string]int, len(targets))

	for i, t := range targets {
		field := fmt.Sprintf("versioning.version_files[%d]", i)

		if t.Path == "" {
			v.errors.Addf("%s.path: required", field)
		} else if prev, dup := seen[t.Path+"\x00"+t.Key]; dup {
			// The same path+key twice means the second write silently wins.
			v.errors.Addf("%s: duplicates entry %d (same path and key)", field, prev)
		} else {
			seen[t.Path+"\x00"+t.Key] = i
		}

		if t.Format != "" && !slices.Contains(validFormats, string(t.Format)) {
			v.errors.Addf("%s.format: must be one of %v, got %q", field, validFormats, t.Format)
		}
		if t.Strategy != "" && !slices.Contains(validStrategies, string(t.Strategy)) {
			v.errors.Addf("%s.strategy: must be one of %v, got %q", field, validStrategies, t.Strategy)
		}

		if t.Format == VersionFormatTemplate && t.Template == "" {
			v.errors.Addf("%s.template: required when format is 'template'", field)
		}
		if t.Template != "" && t.Format != VersionFormatTemplate {
			v.errors.Addf("%s.template: only applies when format is 'template'", field)
		}
		if t.Strategy == StrategyIncrement && t.Format != VersionFormatInteger {
			v.errors.Addf("%s.strategy: 'increment' requires format 'integer'", field)
		}
	}
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
	// Note: link_commits defaults to false and is auto-enabled when repository_url
	// is detected from git remote, so we treat missing URL as an error if explicitly enabled.
	if cfg.LinkCommits && cfg.RepositoryURL == "" {
		v.errors.Addf("changelog.link_commits: enabled but repository_url is not set (auto-detection may have failed)")
	}
	if cfg.RepositoryURL != "" {
		if _, err := url.Parse(cfg.RepositoryURL); err != nil {
			v.errors.Addf("changelog.repository_url: invalid URL: %s", cfg.RepositoryURL)
		}
	}

	// link_issues must be explicitly configured with issue_url
	if cfg.LinkIssues && cfg.IssueURL == "" {
		v.errors.Addf("changelog.link_issues: enabled but issue_url is not set")
	}
	if cfg.IssueURL != "" {
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
	validProviders := []string{"openai", "anthropic", "claude", "ollama", "gemini", "azure-openai"}
	if !slices.Contains(validProviders, cfg.Provider) {
		v.errors.Addf("ai.provider: must be one of %v, got %q", validProviders, cfg.Provider)
	}

	// Warn about deprecated "claude" provider
	if cfg.Provider == "claude" {
		v.errors.Warnf("ai.provider: 'claude' is deprecated, use 'anthropic' instead")
	}

	// Azure OpenAI specific validation
	if cfg.Provider == "azure-openai" {
		if cfg.BaseURL == "" && os.Getenv("AZURE_OPENAI_ENDPOINT") == "" {
			v.errors.Addf("ai.base_url: required for Azure OpenAI (set via config or AZURE_OPENAI_ENDPOINT env var)")
		}
		// Warn if using generic OpenAI key format with Azure
		if isOpenAIKeyFormat(cfg.APIKey) {
			v.errors.Warnf("ai.api_key: appears to be an OpenAI key but provider is 'azure-openai' (Azure keys are 32 hex characters)")
		}
	}

	// Validate model
	if cfg.Model == "" {
		v.errors.Addf("ai.model: required when AI is enabled")
	}

	// Validate API key is provided (after env expansion)
	if cfg.APIKey == "" {
		// Check if it's provided via environment variable (provider-specific or generic)
		providerEnvVars := map[string]string{
			"openai":       "OPENAI_API_KEY",
			"anthropic":    "ANTHROPIC_API_KEY",
			"claude":       "ANTHROPIC_API_KEY",
			"gemini":       "GEMINI_API_KEY",
			"azure-openai": "AZURE_OPENAI_KEY",
			"ollama":       "", // Ollama doesn't require an API key
		}

		envVar := providerEnvVars[cfg.Provider]
		genericEnvVar := "RELICTA_AI_API_KEY"

		// Ollama doesn't require an API key
		if cfg.Provider == "ollama" {
			return
		}

		if os.Getenv(envVar) == "" && os.Getenv(genericEnvVar) == "" {
			v.errors.Addf("ai.api_key: required when AI is enabled (set via config or %s env var)", envVar)
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
	// Warn about high temperature values
	if cfg.Temperature > 1.0 {
		v.errors.Warnf("ai.temperature: value %.1f is unusually high (typical range is 0.0-1.0)", cfg.Temperature)
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

// validatePersistence refuses a backend the build cannot honor, at load.
//
// ADR-013's first consequence is that `persistence.backend` stops lying, and half of that
// is refusing a value nothing can serve instead of ignoring it. PersistenceConfig.Validate
// held the rule already and no load path called it, so a typo — `backend: postgress` — read
// as "not postgres", and relicta wrote the team's audit trail to local files while they
// believed it was in their database.
//
// Delegating rather than restating the rule keeps one definition of a valid persistence
// section; the resolver that opens the store checks the same one, because a config that
// loads and then cannot be served is the same lie one step later.
func (v *Validator) validatePersistence(cfg PersistenceConfig) {
	// An unset backend is not a choice the operator made. The loader defaults it to
	// "file", so an empty value here means a Config assembled in code, and telling such a
	// caller that "" is unsupported would report a problem they do not have.
	if cfg.Backend == "" {
		return
	}
	if err := cfg.Validate(); err != nil {
		v.errors.Addf("persistence: %v", err)
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
