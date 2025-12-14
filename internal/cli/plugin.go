package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/relicta-tech/relicta/internal/plugin/manager"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage Relicta plugins",
	Long: `Manage plugins for Relicta.

Plugins extend Relicta's functionality for version control systems,
package managers, notification services, and more.

Examples:
  # List available plugins
  relicta plugin list --available

  # Install a plugin
  relicta plugin install github

  # Configure a plugin interactively
  relicta plugin configure github

  # Update a plugin
  relicta plugin update github

  # Get plugin information
  relicta plugin info github`,
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List plugins",
	Long: `List installed plugins or available plugins from the registry.

By default, shows installed plugins. Use --available to show all plugins
from the registry.`,
	RunE: runPluginList,
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install <name>",
	Short: "Install a plugin",
	Long: `Install a plugin from the registry.

Downloads the plugin binary for your platform and makes it available
for use. Plugins must be enabled after installation with 'plugin enable'.`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginInstall,
}

var pluginUninstallCmd = &cobra.Command{
	Use:     "uninstall <name>",
	Aliases: []string{"remove"},
	Short:   "Uninstall a plugin",
	Long:    `Remove an installed plugin and its associated files.`,
	Args:    cobra.ExactArgs(1),
	RunE:    runPluginUninstall,
}

var pluginEnableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: "Enable a plugin",
	Long:  `Enable an installed plugin to use it in releases.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runPluginEnable,
}

var pluginDisableCmd = &cobra.Command{
	Use:   "disable <name>",
	Short: "Disable a plugin",
	Long:  `Disable a plugin without uninstalling it.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runPluginDisable,
}

var pluginInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show detailed information about a plugin",
	Long: `Display detailed information about a plugin including:
- Description and metadata
- Version information
- Installation status
- Configuration schema
- Required hooks`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginInfo,
}

var pluginUpdateCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "Update a plugin to the latest version",
	Long:  `Update an installed plugin to the latest version from the registry.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runPluginUpdate,
}

var pluginConfigureCmd = &cobra.Command{
	Use:   "configure <name>",
	Short: "Interactively configure a plugin",
	Long: `Interactively configure a plugin by prompting for required and optional settings.

This command helps you set up plugin configuration by:
- Showing current configuration values
- Prompting for required fields
- Suggesting defaults for optional fields
- Validating inputs based on field types
- Updating release.config.yaml with new settings`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginConfigure,
}

var (
	pluginListAvailable bool
	pluginListRefresh   bool
)

func init() {
	// Add plugin command to root
	rootCmd.AddCommand(pluginCmd)

	// Add subcommands to plugin
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginInstallCmd)
	pluginCmd.AddCommand(pluginUninstallCmd)
	pluginCmd.AddCommand(pluginEnableCmd)
	pluginCmd.AddCommand(pluginDisableCmd)
	pluginCmd.AddCommand(pluginInfoCmd)
	pluginCmd.AddCommand(pluginUpdateCmd)
	pluginCmd.AddCommand(pluginConfigureCmd)

	// Flags for plugin list
	pluginListCmd.Flags().BoolVarP(&pluginListAvailable, "available", "a", false, "Show all available plugins from registry")
	pluginListCmd.Flags().BoolVarP(&pluginListRefresh, "refresh", "r", false, "Force refresh registry cache")
}

func runPluginList(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	mgr, err := manager.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create plugin manager: %w", err)
	}

	var entries []manager.PluginListEntry
	if pluginListAvailable {
		entries, err = mgr.ListAvailable(ctx, pluginListRefresh)
		if err != nil {
			return fmt.Errorf("failed to list available plugins: %w", err)
		}
	} else {
		entries, err = mgr.ListInstalled(ctx)
		if err != nil {
			return fmt.Errorf("failed to list installed plugins: %w", err)
		}
	}

	// Display plugins
	if len(entries) == 0 {
		if pluginListAvailable {
			fmt.Println("No plugins available in registry.")
		} else {
			fmt.Println("No plugins installed.")
			fmt.Println()
			fmt.Println("Use 'relicta plugin list --available' to see available plugins.")
			fmt.Println("Use 'relicta plugin install <name>' to install a plugin.")
		}
		return nil
	}

	if pluginListAvailable {
		displayAvailablePlugins(entries)
	} else {
		displayInstalledPlugins(entries)
	}

	return nil
}

func displayInstalledPlugins(entries []manager.PluginListEntry) {
	fmt.Println("Installed Plugins:")
	fmt.Println()

	for _, entry := range entries {
		// Status icon
		var statusIcon, statusText string
		switch entry.Status {
		case manager.StatusEnabled:
			statusIcon = "✓"
			statusText = "enabled"
		case manager.StatusInstalled:
			statusIcon = "✗"
			statusText = "disabled"
		case manager.StatusUpdateAvailable:
			statusIcon = "⚠"
			statusText = "update available"
		default:
			statusIcon = " "
			statusText = "unknown"
		}

		version := entry.Info.Version
		if entry.Installed != nil {
			version = entry.Installed.Version
		}

		fmt.Printf("  %s %-15s (%-8s)  %s  %s\n",
			statusIcon,
			entry.Info.Name,
			version,
			formatStatus(statusText),
			entry.Info.Description,
		)
	}

	fmt.Println()
	fmt.Println("Use 'relicta plugin info <name>' for more details.")
}

func displayAvailablePlugins(entries []manager.PluginListEntry) {
	fmt.Println("Available Plugins:")
	fmt.Println()

	// Group by category
	categories := make(map[string][]manager.PluginListEntry)
	for _, entry := range entries {
		category := entry.Info.Category
		if category == "" {
			category = "other"
		}
		categories[category] = append(categories[category], entry)
	}

	// Display by category
	categoryNames := []string{"vcs", "notification", "package_manager", "project_management", "container", "other"}
	categoryTitles := map[string]string{
		"vcs":                "Version Control:",
		"notification":       "Notifications:",
		"package_manager":    "Package Managers:",
		"project_management": "Project Management:",
		"container":          "Containers:",
		"other":              "Other:",
	}

	for _, category := range categoryNames {
		plugins, ok := categories[category]
		if !ok || len(plugins) == 0 {
			continue
		}

		fmt.Println(categoryTitles[category])
		for _, entry := range plugins {
			// Status indicator
			var status string
			switch entry.Status {
			case manager.StatusEnabled:
				status = "✓ installed"
			case manager.StatusInstalled:
				status = "✓ installed"
			case manager.StatusUpdateAvailable:
				status = "⚠ update"
			default:
				status = ""
			}

			fmt.Printf("  %-12s %-8s  %-12s %s\n",
				entry.Info.Name,
				entry.Info.Version,
				status,
				entry.Info.Description,
			)
		}
		fmt.Println()
	}

	fmt.Println("Use 'relicta plugin install <name>' to install a plugin.")
	fmt.Println("Use 'relicta plugin info <name>' for more details.")
}

func formatStatus(status string) string {
	statusColors := map[string]string{
		"enabled":          "enabled",
		"disabled":         "disabled",
		"update available": "update",
	}

	if colored, ok := statusColors[status]; ok {
		return colored
	}
	return status
}

func getCategoryTitle(category string) string {
	switch category {
	case "vcs":
		return "Version Control"
	case "notification":
		return "Notifications"
	case "package_manager":
		return "Package Managers"
	case "project_management":
		return "Project Management"
	case "container":
		return "Containers"
	default:
		return cases.Title(language.English).String(category)
	}
}

func runPluginInstall(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	pluginName := args[0]

	mgr, err := manager.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create plugin manager: %w", err)
	}

	fmt.Printf("Installing plugin %q...\n", pluginName)

	if err := mgr.Install(ctx, pluginName); err != nil {
		return fmt.Errorf("failed to install plugin: %w", err)
	}

	fmt.Println()
	printSuccess(fmt.Sprintf("Plugin %q installed successfully", pluginName))
	fmt.Println()
	fmt.Println("To use this plugin:")
	fmt.Printf("  1. Enable it: relicta plugin enable %s\n", pluginName)
	fmt.Printf("  2. Configure it in release.config.yaml\n")

	return nil
}

func runPluginUninstall(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	pluginName := args[0]

	mgr, err := manager.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create plugin manager: %w", err)
	}

	fmt.Printf("Uninstalling plugin %q...\n", pluginName)

	if err := mgr.Uninstall(ctx, pluginName); err != nil {
		return fmt.Errorf("failed to uninstall plugin: %w", err)
	}

	printSuccess(fmt.Sprintf("Plugin %q uninstalled successfully", pluginName))

	return nil
}

func runPluginEnable(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	pluginName := args[0]

	mgr, err := manager.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create plugin manager: %w", err)
	}

	if err := mgr.Enable(ctx, pluginName); err != nil {
		return fmt.Errorf("failed to enable plugin: %w", err)
	}

	printSuccess(fmt.Sprintf("Plugin %q enabled", pluginName))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Configure the plugin in release.config.yaml")
	fmt.Println("  2. Run relicta commands to use the plugin")

	return nil
}

func runPluginDisable(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	pluginName := args[0]

	mgr, err := manager.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create plugin manager: %w", err)
	}

	if err := mgr.Disable(ctx, pluginName); err != nil {
		return fmt.Errorf("failed to disable plugin: %w", err)
	}

	printSuccess(fmt.Sprintf("Plugin %q disabled", pluginName))

	return nil
}

func runPluginInfo(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	pluginName := args[0]

	mgr, err := manager.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create plugin manager: %w", err)
	}

	entry, err := mgr.GetPluginInfo(ctx, pluginName)
	if err != nil {
		return fmt.Errorf("failed to get plugin info: %w", err)
	}

	// Display plugin information
	fmt.Printf("Plugin: %s\n\n", entry.Info.Name)

	// Basic information
	fmt.Println("Information:")
	fmt.Printf("  Description:  %s\n", entry.Info.Description)
	fmt.Printf("  Category:     %s\n", getCategoryTitle(entry.Info.Category))
	fmt.Printf("  Version:      %s\n", entry.Info.Version)
	fmt.Printf("  Author:       %s\n", entry.Info.Author)
	fmt.Printf("  License:      %s\n", entry.Info.License)
	fmt.Printf("  Homepage:     %s\n", entry.Info.Homepage)
	fmt.Println()

	// Installation status
	if entry.Installed != nil {
		fmt.Println("Installation:")
		statusText := "disabled"
		if entry.Installed.Enabled {
			statusText = "enabled"
		}
		fmt.Printf("  Status:       %s\n", statusText)
		fmt.Printf("  Version:      %s\n", entry.Installed.Version)
		fmt.Printf("  Installed:    %s\n", entry.Installed.InstalledAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Binary:       %s\n", entry.Installed.BinaryPath)
		if entry.Status == manager.StatusUpdateAvailable {
			fmt.Printf("  ⚠ Update available: %s → %s\n", entry.Installed.Version, entry.Info.Version)
		}
		fmt.Println()
	} else {
		fmt.Println("Installation:")
		fmt.Println("  Status:       Not installed")
		fmt.Println()
	}

	// Hooks
	if len(entry.Info.Hooks) > 0 {
		fmt.Println("Hooks:")
		for _, hook := range entry.Info.Hooks {
			fmt.Printf("  - %s\n", hook)
		}
		fmt.Println()
	}

	// Configuration schema
	if len(entry.Info.ConfigSchema) > 0 {
		fmt.Println("Configuration:")
		for key, schema := range entry.Info.ConfigSchema {
			required := ""
			if schema.Required {
				required = " (required)"
			}
			fmt.Printf("  %s%s:\n", key, required)
			fmt.Printf("    Type:        %s\n", schema.Type)
			fmt.Printf("    Description: %s\n", schema.Description)
			if schema.Default != nil {
				fmt.Printf("    Default:     %v\n", schema.Default)
			}
			if schema.Env != "" {
				fmt.Printf("    Environment: %s\n", schema.Env)
			}
			if len(schema.Options) > 0 {
				fmt.Printf("    Options:     %v\n", schema.Options)
			}
			fmt.Println()
		}
	}

	// Next steps
	if entry.Installed == nil {
		fmt.Println("Next steps:")
		fmt.Printf("  relicta plugin install %s\n", pluginName)
	} else if !entry.Installed.Enabled {
		fmt.Println("Next steps:")
		fmt.Printf("  relicta plugin enable %s\n", pluginName)
	} else if entry.Status == manager.StatusUpdateAvailable {
		fmt.Println("Next steps:")
		fmt.Printf("  relicta plugin update %s\n", pluginName)
	}

	return nil
}

func runPluginUpdate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	pluginName := args[0]

	mgr, err := manager.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create plugin manager: %w", err)
	}

	// Check if plugin is installed
	entry, err := mgr.GetPluginInfo(ctx, pluginName)
	if err != nil {
		return fmt.Errorf("failed to get plugin info: %w", err)
	}

	if entry.Installed == nil {
		return fmt.Errorf("plugin %q is not installed", pluginName)
	}

	// Check if update is available
	if entry.Status != manager.StatusUpdateAvailable {
		fmt.Printf("Plugin %q is already up to date (version %s)\n", pluginName, entry.Installed.Version)
		return nil
	}

	fmt.Printf("Updating plugin %q from %s to %s...\n", pluginName, entry.Installed.Version, entry.Info.Version)

	// Uninstall old version
	if err := mgr.Uninstall(ctx, pluginName); err != nil {
		return fmt.Errorf("failed to uninstall old version: %w", err)
	}

	// Install new version
	if err := mgr.Install(ctx, pluginName); err != nil {
		return fmt.Errorf("failed to install new version: %w", err)
	}

	// Re-enable if it was enabled before
	if entry.Installed.Enabled {
		if err := mgr.Enable(ctx, pluginName); err != nil {
			return fmt.Errorf("failed to re-enable plugin: %w", err)
		}
	}

	printSuccess(fmt.Sprintf("Plugin %q updated to version %s", pluginName, entry.Info.Version))

	return nil
}

func runPluginConfigure(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	pluginName := args[0]

	mgr, err := manager.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create plugin manager: %w", err)
	}

	// Get plugin info
	entry, err := mgr.GetPluginInfo(ctx, pluginName)
	if err != nil {
		return fmt.Errorf("failed to get plugin info: %w", err)
	}

	// Check if plugin is installed
	if entry.Installed == nil {
		fmt.Printf("Plugin %q is not installed.\n", pluginName)
		fmt.Printf("Install it first with: relicta plugin install %s\n", pluginName)
		return nil
	}

	// Check if plugin is enabled
	if !entry.Installed.Enabled {
		fmt.Printf("Plugin %q is not enabled.\n", pluginName)
		fmt.Printf("Enable it first with: relicta plugin enable %s\n", pluginName)
		return nil
	}

	// Display configuration guide
	fmt.Printf("Configuration Guide for %s\n", entry.Info.Name)
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()

	if len(entry.Info.ConfigSchema) == 0 {
		fmt.Println("This plugin does not require any configuration.")
		return nil
	}

	fmt.Println("Add the following configuration to your release.config.yaml file:")
	fmt.Println()
	fmt.Println("plugins:")
	fmt.Printf("  %s:\n", pluginName)

	// Show required fields first
	var requiredFields, optionalFields []string
	for key, schema := range entry.Info.ConfigSchema {
		if schema.Required {
			requiredFields = append(requiredFields, key)
		} else {
			optionalFields = append(optionalFields, key)
		}
	}

	// Display required fields
	if len(requiredFields) > 0 {
		fmt.Println("    # Required fields:")
		for _, key := range requiredFields {
			schema := entry.Info.ConfigSchema[key]
			fmt.Printf("    %s: ", key)

			if schema.Env != "" {
				fmt.Printf("${%s}  # or set value directly\n", schema.Env)
			} else {
				fmt.Printf("<value>  # %s\n", schema.Description)
			}
		}
		fmt.Println()
	}

	// Display optional fields
	if len(optionalFields) > 0 {
		fmt.Println("    # Optional fields:")
		for _, key := range optionalFields {
			schema := entry.Info.ConfigSchema[key]
			fmt.Printf("    # %s: ", key)

			if schema.Default != nil {
				fmt.Printf("%v  # default: %v\n", schema.Default, schema.Description)
			} else if schema.Env != "" {
				fmt.Printf("${%s}  # %s\n", schema.Env, schema.Description)
			} else {
				fmt.Printf("<value>  # %s\n", schema.Description)
			}
		}
		fmt.Println()
	}

	// Show environment variable alternatives
	var envVars []string
	for key, schema := range entry.Info.ConfigSchema {
		if schema.Env != "" {
			envVars = append(envVars, fmt.Sprintf("  %s - for %s", schema.Env, key))
		}
	}

	if len(envVars) > 0 {
		fmt.Println("Alternatively, you can use environment variables:")
		for _, envVar := range envVars {
			fmt.Println(envVar)
		}
		fmt.Println()
	}

	// Show additional information
	if len(entry.Info.Hooks) > 0 {
		fmt.Println("This plugin will be triggered during these hooks:")
		for _, hook := range entry.Info.Hooks {
			fmt.Printf("  - %s\n", hook)
		}
		fmt.Println()
	}

	fmt.Println("For more detailed information, run:")
	fmt.Printf("  relicta plugin info %s\n", pluginName)

	return nil
}
