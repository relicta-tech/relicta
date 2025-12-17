package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/plugin/manager"
)

var pluginSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for plugins",
	Long: `Search for plugins in the registry by name, description, or category.

Examples:
  # Search for plugins
  relicta plugin search github
  relicta plugin search notification
  relicta plugin search slack`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginSearch,
}

func init() {
	pluginCmd.AddCommand(pluginSearchCmd)
}

func runPluginSearch(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	query := args[0]

	mgr, err := manager.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create plugin manager: %w", err)
	}

	matches, err := mgr.Search(ctx, query)
	if err != nil {
		return err
	}

	if len(matches) == 0 {
		fmt.Printf("No plugins found matching %q\n", query)
		return nil
	}

	fmt.Printf("Found %d plugin(s) matching %q:\n\n", len(matches), query)

	for _, plugin := range matches {
		fmt.Printf("  %s (%s)\n", plugin.Name, plugin.Version)
		fmt.Printf("    %s\n", plugin.Description)
		fmt.Printf("    Category: %s\n", plugin.Category)
		if plugin.Author != "" {
			fmt.Printf("    Author: %s\n", plugin.Author)
		}
		fmt.Println()
	}

	fmt.Println("Install with: relicta plugin install <name>")

	return nil
}
