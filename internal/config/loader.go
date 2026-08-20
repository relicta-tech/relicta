// Package config provides configuration management for Relicta.
package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"

	rperrors "github.com/relicta-tech/relicta/v4/internal/errors"
	"github.com/relicta-tech/relicta/v4/internal/security"
)

// Pre-compiled regex patterns for environment variable expansion.
// These are compiled once at package initialization to avoid repeated compilation.
var (
	// envVarPattern matches ${VAR} or ${VAR:-default} syntax
	envVarPattern = regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)
	// simpleEnvVarPattern matches $VAR syntax
	simpleEnvVarPattern = regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)
	// gitSSHURLPattern matches git@host:owner/repo.git format
	gitSSHURLPattern = regexp.MustCompile(`^git@([^:]+):(.+?)(?:\.git)?$`)
	// gitHTTPURLPattern matches https://host/owner/repo.git format
	gitHTTPURLPattern = regexp.MustCompile(`^https?://([^/]+)/(.+?)(?:\.git)?$`)
)

var gitRemoteURLFetcher = fetchGitRemoteURL

// Loader handles configuration loading and merging.
type Loader struct {
	v           *viper.Viper
	configPath  string
	searchPaths []string

	// onlySearchPaths stops the ancestor walk below from running.
	//
	// The walk exists so that relicta run from a subdirectory still finds the repository's
	// own config, and it searches upward from the *process* working directory. That is
	// right for the invoking repository and wrong for any other: a caller asking about a
	// group member's checkout wants that member's config or the defaults, never the
	// ancestors of wherever the command happened to be run.
	onlySearchPaths bool

	// detectedProviders / autoSelectedProvider record the outcome of
	// zero-config AI provider auto-detection. They are surfaced on the
	// loaded Config so callers can tell the user which provider was
	// picked at the moment AI is actually used — not eagerly at startup
	// (issue #127).
	detectedProviders    []string
	autoSelectedProvider string
}

// NewLoader creates a new configuration loader.
func NewLoader() *Loader {
	v := viper.New()
	v.SetEnvPrefix("RELICTA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	return &Loader{
		v:           v,
		searchPaths: []string{"."},
	}
}

// WithConfigPath sets an explicit config file path.
func (l *Loader) WithConfigPath(path string) *Loader {
	l.configPath = path
	return l
}

// WithSearchPaths adds directories to search for config files.
func (l *Loader) WithSearchPaths(paths ...string) *Loader {
	l.searchPaths = append(l.searchPaths, paths...)
	return l
}

// Load loads the configuration.
func (l *Loader) Load() (*Config, error) {
	const op = "config.Load"

	// Set defaults
	l.setDefaults()

	// Auto-detect AI provider from environment if no config file exists
	configFileFound := l.configFileExists()
	if !configFileFound {
		l.autoDetectAI()
	}

	// Load config file
	if err := l.loadConfigFile(); err != nil {
		return nil, rperrors.ConfigWrap(err, op, "failed to load config file")
	}

	// Unmarshal into Config struct
	cfg := &Config{}
	if err := l.v.Unmarshal(cfg); err != nil {
		return nil, rperrors.ConfigWrap(err, op, "failed to unmarshal config")
	}

	// Expand environment variables in sensitive fields
	l.expandEnvVars(cfg)

	// Auto-detect repository URL from git remote if not configured
	l.autoDetectRepositoryURL(cfg)

	// Surface AI auto-detection outcome for callers that use AI.
	cfg.AI.AutoSelected = l.autoSelectedProvider
	cfg.AI.DetectedProviders = l.detectedProviders

	return cfg, nil
}

// setDefaults sets default values using Viper.
func (l *Loader) setDefaults() {
	defaults := DefaultConfig()

	// Versioning defaults
	l.v.SetDefault("versioning.strategy", defaults.Versioning.Strategy)
	l.v.SetDefault("versioning.tag_prefix", defaults.Versioning.TagPrefix)
	l.v.SetDefault("versioning.git_tag", defaults.Versioning.GitTag)
	l.v.SetDefault("versioning.git_push", defaults.Versioning.GitPush)
	l.v.SetDefault("versioning.git_sign", defaults.Versioning.GitSign)
	l.v.SetDefault("versioning.bump_from", defaults.Versioning.BumpFrom)

	// Changelog defaults
	l.v.SetDefault("changelog.file", defaults.Changelog.File)
	l.v.SetDefault("changelog.format", defaults.Changelog.Format)
	l.v.SetDefault("changelog.group_by", defaults.Changelog.GroupBy)
	l.v.SetDefault("changelog.include_commit_hash", defaults.Changelog.IncludeCommitHash)
	l.v.SetDefault("changelog.include_author", defaults.Changelog.IncludeAuthor)
	l.v.SetDefault("changelog.include_date", defaults.Changelog.IncludeDate)
	l.v.SetDefault("changelog.link_commits", defaults.Changelog.LinkCommits)
	l.v.SetDefault("changelog.link_issues", defaults.Changelog.LinkIssues)
	l.v.SetDefault("changelog.exclude", defaults.Changelog.Exclude)
	l.v.SetDefault("changelog.categories", defaults.Changelog.Categories)

	// AI defaults
	l.v.SetDefault("ai.enabled", defaults.AI.Enabled)
	l.v.SetDefault("ai.provider", defaults.AI.Provider)
	l.v.SetDefault("ai.model", defaults.AI.Model)
	l.v.SetDefault("ai.tone", defaults.AI.Tone)
	l.v.SetDefault("ai.audience", defaults.AI.Audience)
	l.v.SetDefault("ai.max_tokens", defaults.AI.MaxTokens)
	l.v.SetDefault("ai.temperature", defaults.AI.Temperature)
	l.v.SetDefault("ai.timeout", defaults.AI.Timeout)
	l.v.SetDefault("ai.retry_attempts", defaults.AI.RetryAttempts)

	// Workflow defaults
	l.v.SetDefault("workflow.require_approval", defaults.Workflow.RequireApproval)
	l.v.SetDefault("workflow.allowed_branches", defaults.Workflow.AllowedBranches)
	l.v.SetDefault("workflow.require_clean_working_tree", defaults.Workflow.RequireCleanWorkingTree)
	l.v.SetDefault("workflow.require_up_to_date", defaults.Workflow.RequireUpToDate)
	l.v.SetDefault("workflow.dry_run_by_default", defaults.Workflow.DryRunByDefault)
	l.v.SetDefault("workflow.auto_commit_changelog", defaults.Workflow.AutoCommitChangelog)
	l.v.SetDefault("workflow.changelog_commit_message", defaults.Workflow.ChangelogCommitMessage)

	// Output defaults
	l.v.SetDefault("output.format", defaults.Output.Format)
	l.v.SetDefault("output.color", defaults.Output.Color)
	l.v.SetDefault("output.verbose", defaults.Output.Verbose)
	l.v.SetDefault("output.quiet", defaults.Output.Quiet)
	l.v.SetDefault("output.log_level", defaults.Output.LogLevel)

	// Set per key rather than per section: a config naming only backend and
	// connection_string must still inherit pool_size and migration_mode, and viper
	// merges defaults key by key.
	l.v.SetDefault("persistence.backend", defaults.Persistence.Backend)
	l.v.SetDefault("persistence.pool_size", defaults.Persistence.PoolSize)
	l.v.SetDefault("persistence.migration_mode", defaults.Persistence.MigrationMode)
	l.v.SetDefault("persistence.file_path", defaults.Persistence.FilePath)

	// Monorepo defaults, per key for the same reason persistence's are: a config naming only
	// enabled and package_paths must still inherit the strategy and the changelog settings.
	// Without these, `monorepo: {enabled: true, package_paths: [...]}` loaded with an empty
	// strategy and was refused by validation — a reasonable configuration rejected for
	// leaving out a key that has a default.
	l.v.SetDefault("monorepo.enabled", defaults.Monorepo.Enabled)
	l.v.SetDefault("monorepo.strategy", string(defaults.Monorepo.Strategy))
	l.v.SetDefault("monorepo.root_package", defaults.Monorepo.RootPackage)
	l.v.SetDefault("monorepo.dependency_coordination", defaults.Monorepo.DependencyCoordination)
	l.v.SetDefault("monorepo.changelog.per_package", defaults.Monorepo.Changelog.PerPackage)
	l.v.SetDefault("monorepo.changelog.root_changelog", defaults.Monorepo.Changelog.RootChangelog)
	l.v.SetDefault("monorepo.changelog.format", defaults.Monorepo.Changelog.Format)
	l.v.SetDefault("monorepo.changelog.include_package_links", defaults.Monorepo.Changelog.IncludePackageLinks)

	// Governance defaults (CGP)
	l.v.SetDefault("governance.enabled", defaults.Governance.Enabled)
	l.v.SetDefault("governance.strict_mode", defaults.Governance.StrictMode)
	l.v.SetDefault("governance.auto_approve_threshold", defaults.Governance.AutoApproveThreshold)
	l.v.SetDefault("governance.max_auto_approve_risk", defaults.Governance.MaxAutoApproveRisk)
	l.v.SetDefault("governance.require_human_for_breaking", defaults.Governance.RequireHumanForBreaking)
	l.v.SetDefault("governance.require_human_for_security", defaults.Governance.RequireHumanForSecurity)
	l.v.SetDefault("governance.memory_enabled", defaults.Governance.MemoryEnabled)
	l.v.SetDefault("governance.memory_path", defaults.Governance.MemoryPath)

	// Dashboard defaults
	l.v.SetDefault("dashboard.enabled", defaults.Dashboard.Enabled)
	l.v.SetDefault("dashboard.address", defaults.Dashboard.Address)
	l.v.SetDefault("dashboard.auth.mode", string(defaults.Dashboard.Auth.Mode))
	l.v.SetDefault("dashboard.auth.session_max_age", defaults.Dashboard.Auth.SessionMaxAge)
	l.v.SetDefault("dashboard.read_timeout", defaults.Dashboard.ReadTimeout)
	l.v.SetDefault("dashboard.write_timeout", defaults.Dashboard.WriteTimeout)
	l.v.SetDefault("dashboard.idle_timeout", defaults.Dashboard.IdleTimeout)
}

// configFileExists checks if a config file exists in search paths.
func (l *Loader) configFileExists() bool {
	// Check explicit path first
	if l.configPath != "" {
		_, err := os.Stat(l.configPath)
		return err == nil
	}

	// Search for config file in paths
	for _, searchPath := range l.searchPaths {
		for _, name := range ConfigFileNames {
			for _, ext := range ConfigFileExtensions {
				configFile := filepath.Join(searchPath, name+"."+ext)
				if _, err := os.Stat(configFile); err == nil {
					return true
				}
			}
		}
	}

	// Must agree with loadConfigFile, which also searches ancestors up to the
	// repository root. Otherwise a subdirectory run would report "no config"
	// and auto-detect an AI provider while still loading the file it found.
	return findConfigInAncestors() != ""
}

// autoDetectAI detects AI provider from environment variables and sets sensible defaults.
// This enables zero-config AI usage when users have API keys in their environment.
func (l *Loader) autoDetectAI() {
	// Detect all available AI providers
	detectedProviders := []string{}

	if os.Getenv("OPENAI_API_KEY") != "" {
		detectedProviders = append(detectedProviders, "openai (OPENAI_API_KEY)")
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		detectedProviders = append(detectedProviders, "anthropic (ANTHROPIC_API_KEY)")
	}
	if os.Getenv("GEMINI_API_KEY") != "" {
		detectedProviders = append(detectedProviders, "gemini (GEMINI_API_KEY)")
	}
	if os.Getenv("AZURE_OPENAI_KEY") != "" && os.Getenv("AZURE_OPENAI_ENDPOINT") != "" {
		detectedProviders = append(detectedProviders, "azure-openai (AZURE_OPENAI_KEY + AZURE_OPENAI_ENDPOINT)")
	}
	if os.Getenv("OLLAMA_HOST") != "" {
		detectedProviders = append(detectedProviders, "ollama (OLLAMA_HOST)")
	}

	// No providers detected
	if len(detectedProviders) == 0 {
		return
	}

	// Check for AI provider API keys in order of preference
	// If an API key is found, auto-enable AI with sensible defaults
	selectedProvider := ""

	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		l.v.SetDefault("ai.enabled", true)
		l.v.SetDefault("ai.provider", "openai")
		l.v.SetDefault("ai.api_key", "${OPENAI_API_KEY}")
		// Use fast model by default for quick responses
		if l.v.GetString("ai.model") == "" {
			l.v.SetDefault("ai.model", "gpt-4o-mini")
		}
		selectedProvider = "openai"
	} else if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		l.v.SetDefault("ai.enabled", true)
		l.v.SetDefault("ai.provider", "anthropic")
		l.v.SetDefault("ai.api_key", "${ANTHROPIC_API_KEY}")
		if l.v.GetString("ai.model") == "" {
			l.v.SetDefault("ai.model", "claude-sonnet-4")
		}
		selectedProvider = "anthropic"
	} else if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		l.v.SetDefault("ai.enabled", true)
		l.v.SetDefault("ai.provider", "gemini")
		l.v.SetDefault("ai.api_key", "${GEMINI_API_KEY}")
		if l.v.GetString("ai.model") == "" {
			l.v.SetDefault("ai.model", "gemini-2.0-flash-exp")
		}
		selectedProvider = "gemini"
	} else if apiKey := os.Getenv("AZURE_OPENAI_KEY"); apiKey != "" {
		baseURL := os.Getenv("AZURE_OPENAI_ENDPOINT")
		if baseURL != "" {
			l.v.SetDefault("ai.enabled", true)
			l.v.SetDefault("ai.provider", "azure-openai")
			l.v.SetDefault("ai.api_key", "${AZURE_OPENAI_KEY}")
			l.v.SetDefault("ai.base_url", "${AZURE_OPENAI_ENDPOINT}")
			if l.v.GetString("ai.model") == "" {
				// User needs to specify deployment name
				l.v.SetDefault("ai.model", "gpt-4")
			}
			selectedProvider = "azure-openai"
		}
	} else if os.Getenv("OLLAMA_HOST") != "" {
		l.v.SetDefault("ai.enabled", true)
		l.v.SetDefault("ai.provider", "ollama")
		l.v.SetDefault("ai.base_url", "${OLLAMA_HOST}")
		if l.v.GetString("ai.model") == "" {
			l.v.SetDefault("ai.model", "llama3.2")
		}
		selectedProvider = "ollama"
	}

	// Record the outcome instead of warning eagerly: provider detection
	// runs for every command, including ones that never touch AI. The
	// notice is printed by callers at the moment AI is actually requested
	// (see Config.AI.AutoSelected / DetectedProviders).
	l.detectedProviders = detectedProviders
	l.autoSelectedProvider = selectedProvider
}

// autoDetectRepositoryURL detects the repository URL from git remote.
// If repository_url is not configured and we can detect it from git,
// we set it and enable link_commits automatically.
func (l *Loader) autoDetectRepositoryURL(cfg *Config) {
	// Skip if repository_url is already configured
	if cfg.Changelog.RepositoryURL != "" {
		return
	}

	// Try to detect from git remote
	repoURL := gitRemoteURLFetcher()
	if repoURL == "" {
		return
	}

	// Set the detected repository URL
	cfg.Changelog.RepositoryURL = repoURL

	// Auto-enable link_commits since we have a valid repository URL
	// Only if it wasn't explicitly set to false in the config file
	if !l.v.IsSet("changelog.link_commits") {
		cfg.Changelog.LinkCommits = true
	}
}

var gitCommand = exec.Command

// fetchGitRemoteURL attempts to get the repository URL from git remote.
// Returns an empty string if not in a git repository or no remote is configured.
func fetchGitRemoteURL() string {
	// Run git remote get-url origin
	cmd := gitCommand("git", "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	remoteURL := strings.TrimSpace(string(output))
	if remoteURL == "" {
		return ""
	}

	// Convert to HTTPS URL for linking
	return convertToHTTPSURL(remoteURL)
}

// convertToHTTPSURL converts a git remote URL to an HTTPS URL suitable for linking.
// Supports both SSH (git@host:owner/repo.git) and HTTPS formats.
func convertToHTTPSURL(remoteURL string) string {
	// Check if it's an SSH URL (git@host:owner/repo.git)
	if matches := gitSSHURLPattern.FindStringSubmatch(remoteURL); len(matches) == 3 {
		host := matches[1]
		path := matches[2]
		return fmt.Sprintf("https://%s/%s", host, path)
	}

	// Check if it's already an HTTPS URL
	if matches := gitHTTPURLPattern.FindStringSubmatch(remoteURL); len(matches) == 3 {
		host := matches[1]
		path := matches[2]
		return fmt.Sprintf("https://%s/%s", host, path)
	}

	// Return as-is if we can't parse it
	return remoteURL
}

// loadConfigFile loads the configuration file.
func (l *Loader) loadConfigFile() error {
	// If explicit path provided, validate and use it
	if l.configPath != "" {
		// Validate path to prevent directory traversal attacks
		validPath, err := security.ValidateConfigPath(l.configPath)
		if err != nil {
			return fmt.Errorf("invalid config path: %w", err)
		}
		l.v.SetConfigFile(validPath)
		if err := l.v.ReadInConfig(); err != nil {
			return fmt.Errorf("reading config file %s: %w", validPath, err)
		}
		return nil
	}

	// Search for config file in paths
	for _, searchPath := range l.searchPaths {
		for _, name := range ConfigFileNames {
			for _, ext := range ConfigFileExtensions {
				configFile := filepath.Join(searchPath, name+"."+ext)
				if _, err := os.Stat(configFile); err == nil {
					l.v.SetConfigFile(configFile)
					if err := l.v.ReadInConfig(); err != nil {
						return fmt.Errorf("reading config file %s: %w", configFile, err)
					}
					return nil
				}
			}
		}
	}

	// Nothing in the explicit search paths: walk up towards the repository root.
	//
	// Without this, running relicta from a subdirectory silently falls back to
	// built-in defaults instead of the repository's own .relicta.yaml. Those
	// defaults include versioning.git_tag and versioning.git_push = true, so a
	// project that had deliberately disabled pushing would get a tag pushed
	// anyway purely because of the directory the command ran in.
	if l.onlySearchPaths {
		// The caller named a directory explicitly. Its config or the defaults; the
		// working directory's ancestors are somebody else's answer.
		return nil
	}

	if configFile := findConfigInAncestors(); configFile != "" {
		l.v.SetConfigFile(configFile)
		if err := l.v.ReadInConfig(); err != nil {
			return fmt.Errorf("reading config file %s: %w", configFile, err)
		}
		return nil
	}

	// No config file found - this is OK, we use defaults
	return nil
}

// findConfigInAncestors looks for a config file in each parent of the working
// directory, stopping at the repository root (the directory holding .git). It
// returns "" when no config file is found, or when the working directory is not
// inside a repository, so behavior outside a checkout is unchanged.
func findConfigInAncestors() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	for dir := cwd; ; {
		// Only search at or below the repository root.
		for _, name := range ConfigFileNames {
			for _, ext := range ConfigFileExtensions {
				candidate := filepath.Join(dir, name+"."+ext)
				if _, err := os.Stat(candidate); err == nil {
					return candidate
				}
			}
		}

		// Stop once this directory is the repository root.
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return ""
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the filesystem root without finding a repository.
			return ""
		}
		dir = parent
	}
}

// expandEnvVars expands environment variables in sensitive configuration fields.
//
// Security: Environment variable expansion is limited to a whitelist of fields:
//   - AI credentials (api_key, base_url)
//   - Plugin configurations (for tokens/credentials)
//   - Changelog URLs (repository_url, issue_url)
//   - Output log file path
//
// Expanded values are used for:
//   - HTTP API calls (safe - no shell interpretation)
//   - File paths (validated separately)
//   - Display purposes
//
// The workflow hooks (PreReleaseHook, PostReleaseHook) are deliberately NOT on
// that list, though they used to be. They are now executed, by `sh -c` in
// internal/cli/release_hooks.go, and expanding a variable into a command string
// before the shell parses it is the textbook injection: a variable holding
// `; rm -rf /` stops being a value and becomes a second command. The shell does
// the expansion itself, after parsing, where `$VAR` can only ever be an argument.
// Hooks keep working exactly as written; they simply expand at the moment they
// run, which is also the moment whose environment they meant.
func (l *Loader) expandEnvVars(cfg *Config) {
	// Expand AI API key
	cfg.AI.APIKey = expandEnvVar(cfg.AI.APIKey)
	cfg.AI.BaseURL = expandEnvVar(cfg.AI.BaseURL)

	// Expand plugin configurations
	for i := range cfg.Plugins {
		expandPluginConfig(cfg.Plugins[i].Config)
	}

	// Workflow hooks are not expanded here on purpose — see above. `sh -c` expands
	// them when it runs them.

	// Expand changelog URLs
	cfg.Changelog.RepositoryURL = expandEnvVar(cfg.Changelog.RepositoryURL)
	cfg.Changelog.IssueURL = expandEnvVar(cfg.Changelog.IssueURL)

	// Expand output log file
	cfg.Output.LogFile = expandEnvVar(cfg.Output.LogFile)
}

// sensitiveEnvVars are system variables that should not be expanded
// in config values to prevent accidental information disclosure.
var sensitiveEnvVars = map[string]bool{
	"PATH": true, "HOME": true, "USER": true, "SHELL": true,
	"PWD": true, "HOSTNAME": true, "LOGNAME": true,
	"SSH_AUTH_SOCK": true, "SSH_AGENT_PID": true,
	"AWS_SECRET_ACCESS_KEY": true, "AWS_SESSION_TOKEN": true,
}

// expandEnvVar expands environment variables in a string.
// Supports both ${VAR} and $VAR syntax.
func expandEnvVar(s string) string {
	if s == "" {
		return s
	}

	// Use pre-compiled pattern for ${VAR} or ${VAR:-default}
	result := envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		submatch := envVarPattern.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}

		varName := submatch[1]
		defaultValue := ""
		if len(submatch) > 2 {
			defaultValue = submatch[2]
		}

		if sensitiveEnvVars[varName] {
			return match // leave unexpanded
		}

		if value := os.Getenv(varName); value != "" {
			return value
		}
		return defaultValue
	})

	// Also expand simple $VAR syntax (but not $$) using pre-compiled pattern
	result = simpleEnvVarPattern.ReplaceAllStringFunc(result, func(match string) string {
		varName := match[1:] // Remove leading $
		if value := os.Getenv(varName); value != "" {
			return value
		}
		return match
	})

	return result
}

// expandPluginConfig expands environment variables in plugin configuration.
func expandPluginConfig(config map[string]any) {
	if config == nil {
		return
	}

	for key, value := range config {
		switch v := value.(type) {
		case string:
			config[key] = expandEnvVar(v)
		case map[string]any:
			expandPluginConfig(v)
		}
	}
}

// GetConfigPath returns the path to the loaded config file, if any.
func (l *Loader) GetConfigPath() string {
	return l.v.ConfigFileUsed()
}

// MergeConfig merges additional configuration values.
func (l *Loader) MergeConfig(values map[string]any) error {
	for key, value := range values {
		l.v.Set(key, value)
	}
	return nil
}

// WriteConfig writes the current configuration to a file.
func WriteConfig(cfg *Config, path string) error {
	const op = "config.WriteConfig"

	v := viper.New()

	// Convert each section to a map keyed by its mapstructure tags before
	// handing it to viper.
	//
	// Passing the structs directly wrote Go field names lowercased — `tagprefix`,
	// `apikey`, `includecommithash` — because viper's writer never consults
	// mapstructure tags, while the loader reads nothing else. 51 of the 74 keys
	// `relicta init` produced did not match the schema, so almost everything a
	// user configured was silently ignored: `tag_prefix: "rel-"` was honored and
	// `tagprefix: "rel-"`, the form init itself wrote, was not.
	//
	// mapstructure.Decode reads the same tags the loader does, which keeps one
	// source of truth rather than adding a parallel set of yaml tags to 354
	// fields that could drift.
	sections := []struct {
		key   string
		value any
	}{
		{"versioning", cfg.Versioning},
		{"changelog", cfg.Changelog},
		{"ai", cfg.AI},
		{"plugins", cfg.Plugins},
		{"workflow", cfg.Workflow},
		{"output", cfg.Output},

		// Governance is written even though it is off by default, because it is the
		// capability the product exists for and it was undiscoverable: `relicta
		// evaluate` said "enable governance in .relicta.yaml" about a section the
		// generated file did not contain, leaving the reader to guess both the key
		// and its nesting.
		{"governance", cfg.Governance},
	}
	for _, section := range sections {
		encoded, err := encodeSection(section.value)
		if err != nil {
			return rperrors.ConfigWrap(err, op, "failed to encode "+section.key+" section")
		}
		v.Set(section.key, encoded)
	}

	// Write to file
	if err := v.WriteConfigAs(path); err != nil {
		return rperrors.ConfigWrap(err, op, "failed to write config file")
	}

	// Viper emits alphabetically-sorted keys with no comments, so the settings
	// that can fire a public release read exactly like the one that sets log
	// level. Annotate those afterwards rather than hand-maintaining a template
	// that would drift from the schema (issue #194).
	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		if err := annotateConfigFile(path); err != nil {
			return rperrors.ConfigWrap(err, op, "failed to annotate config file")
		}
	}

	return nil
}

// encodeSection converts a config section into a value keyed by its mapstructure
// tags, so the written file uses the same names the loader reads.
//
// Sections are not uniformly structs — cfg.Plugins is a []PluginConfig — so this
// dispatches on kind rather than assuming a struct and failing on the one section
// that is a slice.
//
// Durations are rendered as strings ("30s") rather than the integer nanoseconds a
// plain encode produces: the file is meant to be edited by hand, and
// 30000000000 is not something anyone should have to recognize. The loader
// already accepts both, via viper's StringToTimeDurationHookFunc.
func encodeSection(section any) (any, error) {
	rv := reflect.ValueOf(section)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		out := make([]any, 0, rv.Len())
		for i := range rv.Len() {
			encoded, err := encodeSection(rv.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			out = append(out, encoded)
		}
		return out, nil

	case reflect.Struct:
		if d, ok := rv.Interface().(time.Duration); ok {
			return d.String(), nil
		}
		var out map[string]any
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{Result: &out})
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(rv.Interface()); err != nil {
			return nil, err
		}
		return stringifyDurations(out), nil

	default:
		if d, ok := rv.Interface().(time.Duration); ok {
			return d.String(), nil
		}
		return rv.Interface(), nil
	}
}

// stringifyDurations rewrites time.Duration values to their String() form,
// recursively through nested maps and slices.
func stringifyDurations(value any) map[string]any {
	m, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	for k, val := range m {
		switch typed := val.(type) {
		case time.Duration:
			m[k] = typed.String()
		case map[string]any:
			stringifyDurations(typed)
		case []any:
			for i, item := range typed {
				if nested, isMap := item.(map[string]any); isMap {
					stringifyDurations(nested)
				} else if d, isDur := item.(time.Duration); isDur {
					typed[i] = d.String()
				}
			}
		}
	}
	return m
}

// configAnnotations maps a config key to the comment explaining what it does at
// publish time. Keys are matched on their own line regardless of nesting, and a
// key that is absent is simply skipped, so this cannot break when the schema
// changes — it only ever adds comment lines.
var configAnnotations = []struct {
	key   string
	lines []string
}{
	// Keys are the mapstructure names, which are what the writer now emits. They
	// used to be "gittag"/"gitpush", matching the lowercased Go field names the
	// writer produced before it was taught to use mapstructure tags — and when
	// that changed these silently stopped matching, dropping the irreversibility
	// warning from every generated config. TestGeneratedConfigCarriesSafetyNotes
	// exists so a future rename cannot do that again.
	{"git_tag", []string{
		"Create a git tag for the release. Local only; see git_push below.",
	}},
	{"git_push", []string{
		"Push the tag to the remote. THIS IS IRREVERSIBLE and, in any",
		"repository whose release workflow triggers on tag push, it starts a",
		"real public release. Leave false to tag locally and push yourself.",
	}},
	{"governance.enabled", []string{
		"Turn on the Change Governance Protocol: risk scoring, policy",
		"evaluation, and approval gates. With this off, relicta versions and",
		"publishes but does not govern — 'relicta evaluate' and",
		"'relicta analytics' are unavailable and 'relicta approve' asks nothing.",
		"With it on, a breaking or security-related change requires human",
		"approval before publish; see require_human_for_breaking below.",
	}},
	{"plugins", []string{
		"Plugins run at publish time. The github plugin creates a GitHub",
		"release; if your CI already publishes one on tag push, enabling both",
		"will publish twice.",
	}},
}

// annotateConfigFile inserts explanatory comments above the settings whose
// effects are hardest to guess from their names.
func annotateConfigFile(path string) error {
	data, err := os.ReadFile(path) //nolint:gosec // path is the file just written by WriteConfigAs
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines)+16)

	out = append(out,
		"# Relicta configuration. Generated by 'relicta init'.",
		"#",
		"# Commented settings below are the ones that act on the outside world at",
		"# publish time. Everything else is safe to leave alone.",
		"",
	)

	// section is the top-level key currently being emitted, so an annotation can
	// be scoped to one. Without this, a key like "enabled" matched in every
	// section that has one: the governance explanation was inserted under ai: and
	// plugins: as well, three copies of the same paragraph in one file.
	section := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// A top-level key is unindented and ends the previous section.
		if line != "" && line[0] != ' ' && line[0] != '\t' && line[0] != '#' &&
			strings.HasSuffix(trimmed, ":") {
			section = strings.TrimSuffix(trimmed, ":")
		}

		for _, ann := range configAnnotations {
			key := ann.key
			// "governance.enabled" applies only inside governance:. An unqualified
			// key still matches wherever it appears.
			if dot := strings.IndexByte(key, '.'); dot >= 0 {
				if section != key[:dot] {
					continue
				}
				key = key[dot+1:]
			}
			if trimmed == key+":" || strings.HasPrefix(trimmed, key+": ") {
				indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				for _, c := range ann.lines {
					out = append(out, indent+"# "+c)
				}
				break
			}
		}
		out = append(out, line)
	}

	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o600)
}

// WriteDefaultConfig writes the default configuration to a file.
func WriteDefaultConfig(path string) error {
	return WriteConfig(DefaultConfig(), path)
}

// LoadFromFile loads configuration from a specific file.
func LoadFromFile(path string) (*Config, error) {
	return NewLoader().WithConfigPath(path).Load()
}

// LoadFromDirectory loads configuration from a directory, and only from that directory.
//
// It used to be NewLoader().WithSearchPaths(dir), which does not mean this: NewLoader seeds the
// search path with "." and WithSearchPaths appends, so the *process working directory* was
// searched first and won. Every caller names a directory precisely because it is not the working
// directory — a group member's checkout, or a repository the dashboard server was asked about —
// so each of them silently got the invoking repository's configuration instead.
//
// That was not cosmetic once persistence.backend began selecting a store (ADR-013). Verified
// against the shipped binary: a group member whose approved run was in SQLite while the calling
// repository used files was reported as "no release has been planned", and the group executor
// would have published it through the caller's store rather than its own.
//
// A directory with no config file is not an error. The loader returns defaults, which is what a
// repository that has never been configured has always had.
func LoadFromDirectory(dir string) (*Config, error) {
	loader := NewLoader()
	loader.searchPaths = []string{dir}
	loader.onlySearchPaths = true
	return loader.Load()
}

// MustLoad loads configuration and panics on error.
func MustLoad() *Config {
	cfg, err := NewLoader().Load()
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}
	return cfg
}

// FindConfigFile searches for a config file and returns its path.
func FindConfigFile(searchPaths ...string) (string, error) {
	if len(searchPaths) == 0 {
		// No explicit paths: resolve exactly as loading does, which includes
		// walking up to the repository root. Without this, callers checking
		// "is there a config?" disagreed with the loader that then found one.
		return ResolveConfigFile()
	}

	for _, searchPath := range searchPaths {
		for _, name := range ConfigFileNames {
			for _, ext := range ConfigFileExtensions {
				configFile := filepath.Join(searchPath, name+"."+ext)
				if _, err := os.Stat(configFile); err == nil {
					return configFile, nil
				}
			}
		}
	}

	return "", rperrors.NotFound("config.FindConfigFile", "no config file found")
}

// ResolveConfigFile returns the config file that Load would use, or an error if
// there is none. This is the single answer to "which config applies here?" —
// callers that ask the question a different way end up disagreeing with the
// loader, which is what made `relicta health` report "no configuration file
// found" from a subdirectory while every command loaded one (issue #199).
func ResolveConfigFile() (string, error) {
	for _, name := range ConfigFileNames {
		for _, ext := range ConfigFileExtensions {
			candidate := name + "." + ext
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}

	if found := findConfigInAncestors(); found != "" {
		return found, nil
	}

	return "", rperrors.NotFound("config.ResolveConfigFile", "no config file found")
}

// ConfigExists returns true if a config file exists in the given directory.
func ConfigExists(dir string) bool {
	_, err := FindConfigFile(dir)
	return err == nil
}
