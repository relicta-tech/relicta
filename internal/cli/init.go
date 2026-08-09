// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/config"
	"github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
	"github.com/relicta-tech/relicta/v4/internal/ui/wizard"
)

// githubWorkflowsDir is where GitHub Actions workflows live, relative to the
// repository root.
const githubWorkflowsDir = ".github/workflows"

var (
	initForce       bool
	initInteractive bool
	initFormat      string
	initQuick       bool
	initGuided      bool
)

func init() {
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "overwrite existing config file")
	initCmd.Flags().BoolVarP(&initInteractive, "interactive", "i", false, "run the interactive wizard (deprecated alias for --guided)")
	initCmd.Flags().StringVar(&initFormat, "format", "yaml", "config file format (yaml, json)")
	initCmd.Flags().BoolVar(&initQuick, "quick", false, "explicit quick mode (now the default): detect from git + manifests, write defaults, no prompts")
	initCmd.Flags().BoolVar(&initGuided, "guided", false, "opt in to the 8-step interactive setup wizard")
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

	// The interactive wizard is opt-in only (--guided, or the deprecated
	// --interactive alias). Everything else — including a bare `relicta init`
	// — runs zero-config quick mode: detect from the environment, write
	// sensible defaults, no prompts. Goal-Gradient + Doherty Threshold: get to
	// a first governed release fast, with the wizard one flag away for those
	// who want explanatory prompts.
	if initGuided || initInteractive {
		return runInitGuided()
	}

	return runInitQuick()
}

// runInitGuided runs the 8-step interactive setup wizard.
func runInitGuided() error {
	result, err := wizard.RunWizard(".")
	if err != nil {
		return fmt.Errorf("wizard failed: %w", err)
	}

	switch result.State {
	case wizard.StateSuccess:
		// Wizard completed successfully, config already saved.
		return nil
	case wizard.StateQuit:
		printInfo("Setup canceled")
		return nil
	case wizard.StateError:
		return fmt.Errorf("wizard error: %w", result.Error)
	default:
		return fmt.Errorf("unexpected wizard state: %v", result.State)
	}
}

// runInitQuick performs zero-prompt project initialization.
//
// Strategy: detect git remote + repo info, write a sensible default config,
// print the resolved file path. Total interaction: zero prompts.
//
// Designed for power users and CI bootstrap. The 8-step `--guided` wizard
// remains for first-time users who want explanatory prompts.
func runInitQuick() error {
	configFile := ".relicta.yaml"
	if initFormat == "json" {
		configFile = ".relicta.json"
	}

	cfg := config.DefaultConfig()

	// New projects are governed (ADR-011).
	//
	// The schema default stays false, so upgrading an existing project changes
	// nothing: a pipeline that runs `relicta approve --ci` today keeps working. A
	// project created from here on gets risk scoring and the breaking-change gate
	// without having to discover a setting — which is the difference between "the
	// governance layer for software change" and a versioning tool with an optional
	// extra.
	//
	// Written explicitly into the file rather than left to a default, so reading
	// the config still tells the truth about what this project does. That is what
	// keeps the split between generated and default values honest.
	cfg.Governance.Enabled = true

	if err := detectRepoSettings(cfg); err != nil && verbose {
		printWarning(fmt.Sprintf("Could not detect repository settings: %v", err))
	}

	if err := config.WriteConfig(cfg, configFile); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	printSuccess(fmt.Sprintf("Created %s", configFile))

	// The github plugin is auto-enabled for GitHub remotes, and it publishes a
	// release itself. If CI already publishes one on tag push, enabling both
	// double-publishes — so say so at the moment the config is written rather
	// than leaving it to be discovered at publish time (issue #194).
	if hasPlugin(cfg, "github") {
		if wf, found := detectTagTriggeredWorkflow(); found {
			printWarning(fmt.Sprintf("%s publishes a release on tag push, and the github plugin publishes one too", wf))
			printSubtle("  Enabling both will create two releases for the same tag.")
			// plugins is a list of entries, not a map keyed by name, so
			// "plugins.github.enabled" is a path that does not exist in the file
			// this command just wrote.
			printSubtle("  Disable one: set enabled: false on the github entry under plugins,")
			printSubtle("  or drop the tag trigger in the workflow.")
		}
	}

	// gitpush is off by default; be explicit that the tag stops locally, so
	// nobody concludes publish is broken when no release appears.
	if cfg.Versioning.GitTag && !cfg.Versioning.GitPush {
		printSubtle("  versioning.git_push is false: 'relicta publish' tags locally and does not push.")
	}

	// Surface any tokens the detected config will need, so the user isn't
	// surprised at publish time. Kept terse — quick mode is zero-prompt.
	var envHints []string
	if cfg.AI.Enabled {
		envHints = append(envHints, "export OPENAI_API_KEY=<your-api-key>")
	}
	if hasPlugin(cfg, "github") {
		envHints = append(envHints, "export GITHUB_TOKEN=<your-token>")
	}
	if hasPlugin(cfg, "slack") {
		envHints = append(envHints, "export SLACK_WEBHOOK_URL=<your-webhook-url>")
	}
	for _, h := range envHints {
		printSubtle("  " + h)
	}

	printInfo("Run 'relicta plan' to start your first release, or 'relicta init --guided' for the full setup wizard.")
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

// detectTagTriggeredWorkflow reports whether any GitHub Actions workflow triggers
// on tag push, which is what makes an unattended `relicta publish` able to start
// a real release. It returns the workflow's path for use in the warning.
//
// It also checks that the workflow looks like it publishes a release, rather than
// merely running on a tag. Without that, the scan returned whichever
// tag-triggered workflow sorted first and asserted it "publishes a release" —
// on this repository that named docker.yaml, which only pushes a container
// image, while release.yaml, the workflow that actually runs GoReleaser, was
// never mentioned. A warning that names the wrong file sends the reader to edit
// something harmless and leaves the real conflict in place.
//
// Deliberately a cheap textual check rather than a YAML parse: it only drives a
// warning, and the shapes in the wild vary ("tags:", "tags-ignore:", branch and
// tag filters combined). A false negative costs a missing warning; parsing every
// workflow to be exact would cost more than the warning is worth.
func detectTagTriggeredWorkflow() (string, bool) {
	entries, err := os.ReadDir(githubWorkflowsDir)
	if err != nil {
		return "", false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}

		path := filepath.Join(githubWorkflowsDir, name)
		data, err := os.ReadFile(path) //nolint:gosec // path is confined to .github/workflows
		if err != nil {
			continue
		}

		if hasTagPushTrigger(string(data)) && publishesRelease(string(data)) {
			return path, true
		}
	}

	return "", false
}

// releasePublishers are the ways a workflow creates a GitHub release. Pushing a
// container image or uploading build artifacts is not one of them, which is the
// distinction that matters here: only a workflow that creates a release can
// collide with the github plugin.
var releasePublishers = []string{
	"goreleaser", // goreleaser/goreleaser-action, or a direct invocation
	"softprops/action-gh-release",
	"gh release create",
	"actions/create-release",
	"ncipollo/release-action",
}

func publishesRelease(content string) bool {
	lower := strings.ToLower(content)
	for _, marker := range releasePublishers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// hasTagPushTrigger looks for a `tags:` key inside a `push:` block, ignoring
// `tags-ignore:`.
func hasTagPushTrigger(content string) bool {
	inPush := false
	pushIndent := 0

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		if trimmed == "push:" {
			inPush = true
			pushIndent = indent
			continue
		}

		// A key at or above push's indentation ends the push block.
		if inPush && indent <= pushIndent {
			inPush = false
		}

		if inPush && (trimmed == "tags:" || strings.HasPrefix(trimmed, "tags: ")) {
			return true
		}
	}

	return false
}
