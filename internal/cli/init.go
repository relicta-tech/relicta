// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/infrastructure/git"
	"github.com/relicta-tech/relicta/internal/ui/wizard"
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
	// Check for existing config
	existingConfig, _ := config.FindConfigFile(".")
	if existingConfig != "" && !initForce {
		printWarning(fmt.Sprintf("Config file already exists: %s", existingConfig))
		printInfo("Use --force to overwrite")
		return nil
	}

	// Interactive wizard mode
	if initInteractive {
		result, err := wizard.RunWizard(".")
		if err != nil {
			return fmt.Errorf("wizard failed: %w", err)
		}

		// Handle wizard result
		switch result.State {
		case wizard.StateSuccess:
			// Wizard completed successfully, config already saved
			return nil

		case wizard.StateQuit:
			// User quit the wizard
			printInfo("Setup canceled")
			return nil

		case wizard.StateError:
			// Wizard encountered an error
			return fmt.Errorf("wizard error: %w", result.Error)

		default:
			return fmt.Errorf("unexpected wizard state: %v", result.State)
		}
	}

	// Non-interactive mode: fall back to old behavior
	printTitle("Relicta Setup")
	fmt.Println()

	// Determine config file name
	configFile := ".relicta.yaml"
	if initFormat == "json" {
		configFile = ".relicta.json"
	}

	// Start with defaults
	cfg := config.DefaultConfig()

	// Try to detect repository settings
	if err := detectRepoSettings(cfg); err != nil {
		if verbose {
			printWarning(fmt.Sprintf("Could not detect repository settings: %v", err))
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
	fmt.Println("  3. Run 'relicta plan' to analyze your commits")
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
		githubURL := parseGitHubURL(remoteURL)
		cfg.Changelog.RepositoryURL = githubURL

		// Auto-enable GitHub plugin for GitHub repositories
		if githubURL != "" {
			ensurePlugin(cfg, "github")
		}
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

// isGitHubRemote checks if the remote URL points to GitHub.
func isGitHubRemote(remoteURL string) bool {
	return strings.Contains(remoteURL, "github.com")
}

// isGitLabRemote checks if the remote URL points to GitLab.
func isGitLabRemote(remoteURL string) bool {
	return strings.Contains(remoteURL, "gitlab.com") || strings.Contains(remoteURL, "gitlab.")
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
