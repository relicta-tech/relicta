// Package config provides configuration management for ReleasePilot.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/viper"

	rperrors "github.com/felixgeelhaar/release-pilot/internal/errors"
)

// Pre-compiled regex patterns for environment variable expansion.
// These are compiled once at package initialization to avoid repeated compilation.
var (
	// envVarPattern matches ${VAR} or ${VAR:-default} syntax
	envVarPattern = regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)
	// simpleEnvVarPattern matches $VAR syntax
	simpleEnvVarPattern = regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)
)

// Loader handles configuration loading and merging.
type Loader struct {
	v           *viper.Viper
	configPath  string
	searchPaths []string
}

// NewLoader creates a new configuration loader.
func NewLoader() *Loader {
	v := viper.New()
	v.SetEnvPrefix("RELEASE_PILOT")
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

	return false
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

	// Warn if multiple AI providers detected.
	// Note: Using stderr directly instead of structured logging is intentional here.
	// These are user-facing CLI warnings that must be visible regardless of log level.
	if len(detectedProviders) > 1 && selectedProvider != "" {
		fmt.Fprintf(os.Stderr, `⚠️  Multiple AI provider API keys detected: %s
   Auto-selected '%s' based on priority order.
   To use a different provider, configure ai.provider in your config file.
`, strings.Join(detectedProviders, ", "), selectedProvider)
	}
}

// loadConfigFile loads the configuration file.
func (l *Loader) loadConfigFile() error {
	// If explicit path provided, use it
	if l.configPath != "" {
		l.v.SetConfigFile(l.configPath)
		if err := l.v.ReadInConfig(); err != nil {
			return fmt.Errorf("reading config file %s: %w", l.configPath, err)
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

	// No config file found - this is OK, we use defaults
	return nil
}

// expandEnvVars expands environment variables in sensitive configuration fields.
func (l *Loader) expandEnvVars(cfg *Config) {
	// Expand AI API key
	cfg.AI.APIKey = expandEnvVar(cfg.AI.APIKey)
	cfg.AI.BaseURL = expandEnvVar(cfg.AI.BaseURL)

	// Expand plugin configurations
	for i := range cfg.Plugins {
		expandPluginConfig(cfg.Plugins[i].Config)
	}

	// Expand workflow hooks
	cfg.Workflow.PreReleaseHook = expandEnvVar(cfg.Workflow.PreReleaseHook)
	cfg.Workflow.PostReleaseHook = expandEnvVar(cfg.Workflow.PostReleaseHook)

	// Expand changelog URLs
	cfg.Changelog.RepositoryURL = expandEnvVar(cfg.Changelog.RepositoryURL)
	cfg.Changelog.IssueURL = expandEnvVar(cfg.Changelog.IssueURL)

	// Expand output log file
	cfg.Output.LogFile = expandEnvVar(cfg.Output.LogFile)
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

	// Set all values from config
	v.Set("versioning", cfg.Versioning)
	v.Set("changelog", cfg.Changelog)
	v.Set("ai", cfg.AI)
	v.Set("plugins", cfg.Plugins)
	v.Set("workflow", cfg.Workflow)
	v.Set("output", cfg.Output)

	// Write to file
	if err := v.WriteConfigAs(path); err != nil {
		return rperrors.ConfigWrap(err, op, "failed to write config file")
	}

	return nil
}

// WriteDefaultConfig writes the default configuration to a file.
func WriteDefaultConfig(path string) error {
	return WriteConfig(DefaultConfig(), path)
}

// LoadFromFile loads configuration from a specific file.
func LoadFromFile(path string) (*Config, error) {
	return NewLoader().WithConfigPath(path).Load()
}

// LoadFromDirectory loads configuration from a directory.
func LoadFromDirectory(dir string) (*Config, error) {
	return NewLoader().WithSearchPaths(dir).Load()
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
		searchPaths = []string{"."}
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

// ConfigExists returns true if a config file exists in the given directory.
func ConfigExists(dir string) bool {
	_, err := FindConfigFile(dir)
	return err == nil
}
