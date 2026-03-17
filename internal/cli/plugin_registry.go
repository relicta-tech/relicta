package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/plugin/manager"
)

var pluginRegistryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Manage plugin registries",
	Long: `Manage plugin registries for discovering and installing plugins.

Relicta supports multiple plugin registries. The official registry is always
enabled and takes precedence. Community registries can be added to discover
third-party plugins.

Examples:
  # List all configured registries
  relicta plugin registry list

  # Add a community registry
  relicta plugin registry add awesome-plugins https://example.com/registry.yaml

  # Remove a registry
  relicta plugin registry remove awesome-plugins

  # Disable a registry temporarily
  relicta plugin registry disable awesome-plugins`,
}

var pluginRegistryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured registries",
	Long:  `List all configured plugin registries with their status and priority.`,
	RunE:  runPluginRegistryList,
}

var pluginRegistryAddCmd = &cobra.Command{
	Use:   "add <name> <url> [priority]",
	Short: "Add a new registry",
	Long: `Add a new plugin registry.

The registry should be a YAML file with the same format as the official registry.
Priority determines the order (higher = checked first). Default priority is 100.

Example:
  relicta plugin registry add community https://example.com/plugins/registry.yaml
  relicta plugin registry add company https://internal.example.com/registry.yaml 500`,
	Args: cobra.RangeArgs(2, 3),
	RunE: runPluginRegistryAdd,
}

var pluginRegistryRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove a registry",
	Long:    `Remove a plugin registry. The official registry cannot be removed.`,
	Args:    cobra.ExactArgs(1),
	RunE:    runPluginRegistryRemove,
}

var pluginRegistryEnableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: "Enable a disabled registry",
	Long:  `Enable a previously disabled plugin registry.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runPluginRegistryEnable,
}

var pluginRegistryDisableCmd = &cobra.Command{
	Use:   "disable <name>",
	Short: "Disable a registry",
	Long:  `Disable a plugin registry without removing it. The official registry cannot be disabled.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runPluginRegistryDisable,
}

func init() {
	pluginCmd.AddCommand(pluginRegistryCmd)

	pluginRegistryCmd.AddCommand(pluginRegistryListCmd)
	pluginRegistryCmd.AddCommand(pluginRegistryAddCmd)
	pluginRegistryCmd.AddCommand(pluginRegistryRemoveCmd)
	pluginRegistryCmd.AddCommand(pluginRegistryEnableCmd)
	pluginRegistryCmd.AddCommand(pluginRegistryDisableCmd)
}

func runPluginRegistryList(cmd *cobra.Command, args []string) error {
	mgr, err := newPluginManager()
	if err != nil {
		return fmt.Errorf("failed to create plugin manager: %w", err)
	}

	registries := mgr.ListRegistries()

	if len(registries) == 0 {
		fmt.Println("No registries configured.")
		return nil
	}

	fmt.Println("Plugin Registries:")
	fmt.Println()

	for _, reg := range registries {
		statusIcon := "✓"
		statusText := "enabled"
		if !reg.Enabled {
			statusIcon = "✗"
			statusText = "disabled"
		}

		// Mark official registry
		name := reg.Name
		if reg.Name == manager.OfficialRegistryName {
			name = fmt.Sprintf("%s (official)", reg.Name)
		}

		fmt.Printf("  %s %-20s  priority: %-4d  %s\n", statusIcon, name, reg.Priority, statusText)
		fmt.Printf("    %s\n", reg.URL)
		fmt.Println()
	}

	return nil
}

func runPluginRegistryAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	url := args[1]
	priority := 100 // Default priority

	if len(args) > 2 {
		p, err := strconv.Atoi(args[2])
		if err != nil {
			return fmt.Errorf("invalid priority %q: must be a number", args[2])
		}
		priority = p
	}

	mgr, err := newPluginManager()
	if err != nil {
		return fmt.Errorf("failed to create plugin manager: %w", err)
	}

	if err := mgr.AddRegistry(name, url, priority); err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Registry %q added successfully", name))
	fmt.Println()
	fmt.Println("Run 'relicta plugin list --available --refresh' to see plugins from the new registry.")

	return nil
}

func runPluginRegistryRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	mgr, err := newPluginManager()
	if err != nil {
		return fmt.Errorf("failed to create plugin manager: %w", err)
	}

	if err := mgr.RemoveRegistry(name); err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Registry %q removed", name))

	return nil
}

func runPluginRegistryEnable(cmd *cobra.Command, args []string) error {
	name := args[0]

	mgr, err := newPluginManager()
	if err != nil {
		return fmt.Errorf("failed to create plugin manager: %w", err)
	}

	if err := mgr.EnableRegistry(name, true); err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Registry %q enabled", name))

	return nil
}

func runPluginRegistryDisable(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Prevent disabling official registry
	if name == manager.OfficialRegistryName {
		return fmt.Errorf("cannot disable the official registry")
	}

	mgr, err := newPluginManager()
	if err != nil {
		return fmt.Errorf("failed to create plugin manager: %w", err)
	}

	if err := mgr.EnableRegistry(name, false); err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Registry %q disabled", name))

	return nil
}
