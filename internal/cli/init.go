// Package cli provides the command-line interface for ReleasePilot.
package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/felixgeelhaar/release-pilot/internal/config"
	"github.com/felixgeelhaar/release-pilot/internal/service/git"
)

var (
	initForce       bool
	initInteractive bool
	initFormat      string
)

func init() {
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "overwrite existing config file")
	initCmd.Flags().BoolVarP(&initInteractive, "interactive", "i", true, "run interactive setup")
	initCmd.Flags().StringVar(&initFormat, "format", "yaml", "config file format (yaml, json)")
}

// runInit implements the init command.
func runInit(cmd *cobra.Command, args []string) error {
	printTitle("ReleasePilot Setup")
	fmt.Println()

	// Check for existing config
	existingConfig, _ := config.FindConfigFile(".")
	if existingConfig != "" && !initForce {
		printWarning(fmt.Sprintf("Config file already exists: %s", existingConfig))
		printInfo("Use --force to overwrite")
		return nil
	}

	// Determine config file name
	configFile := "release.config.yaml"
	if initFormat == "json" {
		configFile = "release.config.json"
	}

	// Start with defaults
	cfg := config.DefaultConfig()

	// Try to detect repository settings
	if err := detectRepoSettings(cfg); err != nil {
		if verbose {
			printWarning(fmt.Sprintf("Could not detect repository settings: %v", err))
		}
	}

	// Interactive setup
	if initInteractive {
		if err := runInteractiveSetup(cfg); err != nil {
			return err
		}
	}

	// Write config file
	if err := config.WriteConfig(cfg, configFile); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	printSuccess(fmt.Sprintf("Created %s", configFile))
	fmt.Println()

	// Print next steps
	printTitle("Next Steps")
	fmt.Println()
	fmt.Println("  1. Review and customize your config file")
	fmt.Println("  2. Set up required environment variables:")
	fmt.Println()
	if cfg.AI.Enabled {
		printSubtle("     export OPENAI_API_KEY=your-api-key")
	}
	if hasPlugin(cfg, "github") {
		printSubtle("     export GITHUB_TOKEN=your-token")
	}
	if hasPlugin(cfg, "slack") {
		printSubtle("     export SLACK_WEBHOOK_URL=your-webhook-url")
	}
	fmt.Println()
	fmt.Println("  3. Run 'release-pilot plan' to analyze your commits")
	fmt.Println()

	return nil
}

// detectRepoSettings detects repository settings from the environment.
func detectRepoSettings(cfg *config.Config) error {
	// Try to open git repository
	gitSvc, err := git.NewService()
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Get repository info
	info, err := gitSvc.GetRepositoryInfo(ctx)
	if err != nil {
		return err
	}

	// Try to detect remote URL and extract owner/repo
	remoteURL, err := gitSvc.GetRemoteURL(ctx, "origin")
	if err == nil {
		cfg.Changelog.RepositoryURL = parseGitHubURL(remoteURL)
	}

	// Set default branch
	if info.DefaultBranch != "" {
		cfg.Workflow.AllowedBranches = []string{info.DefaultBranch}
	}

	return nil
}

// parseGitHubURL extracts GitHub repository URL from a git remote URL.
func parseGitHubURL(remoteURL string) string {
	// Handle SSH format: git@github.com:owner/repo.git
	if strings.HasPrefix(remoteURL, "git@github.com:") {
		path := strings.TrimPrefix(remoteURL, "git@github.com:")
		path = strings.TrimSuffix(path, ".git")
		return "https://github.com/" + path
	}

	// Handle HTTPS format: https://github.com/owner/repo.git
	if strings.Contains(remoteURL, "github.com") {
		url := strings.TrimSuffix(remoteURL, ".git")
		if !strings.HasPrefix(url, "https://") {
			url = "https://" + strings.TrimPrefix(url, "http://")
		}
		return url
	}

	return ""
}

// promptYesNo prompts the user for a yes/no response with a default value.
// defaultYes indicates whether the default is yes (true) or no (false).
func promptYesNo(reader *bufio.Reader, prompt string, defaultYes bool) bool {
	fmt.Print(prompt)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if defaultYes {
		// Default is yes, only return false for explicit "n" or "no"
		return response != "n" && response != "no"
	}
	// Default is no, only return true for explicit "y" or "yes"
	return response == "y" || response == "yes"
}

// promptString prompts the user for a string value with an optional default.
func promptString(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	response, _ := reader.ReadString('\n')
	return strings.TrimSpace(response)
}

// promptVersioningStrategy prompts for versioning strategy configuration.
func promptVersioningStrategy(reader *bufio.Reader, cfg *config.Config) {
	if !promptYesNo(reader, "Use conventional commits for versioning? [Y/n]: ", true) {
		cfg.Versioning.Strategy = "manual"
	}

	if response := promptString(reader, "Tag prefix (default: v): "); response != "" {
		cfg.Versioning.TagPrefix = response
	}
}

// promptAIConfiguration prompts for AI configuration.
func promptAIConfiguration(reader *bufio.Reader, cfg *config.Config) {
	cfg.AI.Enabled = promptYesNo(reader, "Enable AI-powered release notes? [y/N]: ", false)

	if !cfg.AI.Enabled {
		return
	}

	if response := promptString(reader, "OpenAI model (default: gpt-4): "); response != "" {
		cfg.AI.Model = response
	}

	response := promptString(reader, "AI tone [technical/friendly/professional]: ")
	response = strings.ToLower(response)
	if response == "technical" || response == "friendly" || response == "professional" {
		cfg.AI.Tone = response
	}
}

// promptPluginConfiguration prompts for plugin configuration.
func promptPluginConfiguration(reader *bufio.Reader, cfg *config.Config) {
	fmt.Println()
	fmt.Println("Enable plugins:")

	// GitHub
	if promptYesNo(reader, "  GitHub releases? [Y/n]: ", true) {
		ensurePlugin(cfg, "github")
	} else {
		removePlugin(cfg, "github")
	}

	// npm
	if promptYesNo(reader, "  npm publish? [y/N]: ", false) {
		ensurePlugin(cfg, "npm")
	}

	// Slack
	if promptYesNo(reader, "  Slack notifications? [y/N]: ", false) {
		ensurePlugin(cfg, "slack")
	}
}

// promptWorkflowConfiguration prompts for workflow configuration.
func promptWorkflowConfiguration(reader *bufio.Reader, cfg *config.Config) {
	fmt.Println()
	cfg.Workflow.RequireApproval = promptYesNo(reader, "Require approval before publishing? [Y/n]: ", true)
	fmt.Println()
}

// runInteractiveSetup runs the interactive setup wizard.
func runInteractiveSetup(cfg *config.Config) error {
	reader := bufio.NewReader(os.Stdin)

	printInfo("Answer the following questions to configure ReleasePilot")
	fmt.Println()

	promptVersioningStrategy(reader, cfg)
	promptAIConfiguration(reader, cfg)
	promptPluginConfiguration(reader, cfg)
	promptWorkflowConfiguration(reader, cfg)

	return nil
}

// hasPlugin checks if a plugin is configured.
func hasPlugin(cfg *config.Config, name string) bool {
	for _, p := range cfg.Plugins {
		if p.Name == name {
			return p.IsEnabled()
		}
	}
	return false
}

// ensurePlugin ensures a plugin is in the config.
func ensurePlugin(cfg *config.Config, name string) {
	for i, p := range cfg.Plugins {
		if p.Name == name {
			enabled := true
			cfg.Plugins[i].Enabled = &enabled
			return
		}
	}

	// Add new plugin
	enabled := true
	plugin := config.PluginConfig{
		Name:    name,
		Enabled: &enabled,
	}

	// Set default config for known plugins
	switch name {
	case "github":
		plugin.Config = map[string]any{
			"draft": false,
		}
	case "npm":
		plugin.Config = map[string]any{
			"access": "public",
		}
	case "slack":
		plugin.Config = map[string]any{
			"webhook":           "${SLACK_WEBHOOK_URL}",
			"notify_on_success": true,
			"notify_on_error":   true,
		}
	}

	cfg.Plugins = append(cfg.Plugins, plugin)
}

// removePlugin removes a plugin from the config.
func removePlugin(cfg *config.Config, name string) {
	var filtered []config.PluginConfig
	for _, p := range cfg.Plugins {
		if p.Name != name {
			filtered = append(filtered, p)
		}
	}
	cfg.Plugins = filtered
}
