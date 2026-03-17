// Package cli provides the command-line interface for Relicta.
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/application/versioning"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

var (
	promoteFrom    string
	promoteTo      string
	promoteVersion string
)

var promoteCmd = &cobra.Command{
	Use:   "promote",
	Short: "Promote a release from one channel to another",
	Long: `Promote a release from one channel to another.

Promotion creates a new tag on the same commit with the target channel's
prerelease identifier. For example:
  v1.2.0-canary.1 -> v1.2.0-alpha.1 -> v1.2.0-beta.1 -> v1.2.0-rc.1 -> v1.2.0

Available channels (ordered by stability):
  canary  - Bleeding-edge releases
  alpha   - Early development releases
  beta    - Feature-complete but potentially unstable
  next    - Release candidates (maps to -rc.N suffix)
  stable  - Production releases (no prerelease suffix)

Examples:
  # Promote latest canary to alpha
  relicta promote --from canary --to alpha

  # Promote a specific version to stable
  relicta promote --from beta --to stable --version v1.2.0-beta.3

  # Promote from next (rc) to stable
  relicta promote --from next --to stable

  # Dry run to preview the promotion
  relicta promote --from alpha --to beta --dry-run`,
	RunE: runPromote,
}

func init() {
	promoteCmd.Flags().StringVar(&promoteFrom, "from", "", "source channel (required)")
	promoteCmd.Flags().StringVar(&promoteTo, "to", "", "target channel (required)")
	promoteCmd.Flags().StringVar(&promoteVersion, "version", "", "specific version to promote (default: latest on source channel)")

	_ = promoteCmd.MarkFlagRequired("from")
	_ = promoteCmd.MarkFlagRequired("to")
}

func runPromote(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	printTitle("Release Promotion")
	fmt.Println()

	if dryRun {
		printDryRunBanner()
	}

	// Initialize container
	app, err := newContainerApp(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer closeApp(app)

	// Build channel registry from config
	registry := version.NewChannelRegistry()
	if cfg.Channels.Enabled {
		for _, def := range cfg.Channels.Definitions {
			ch := version.NewChannel(
				def.Name,
				version.StabilityLevel(def.Stability),
				def.TagPattern,
				def.PromotesTo,
				version.Prerelease(def.Prerelease),
			)
			registry.Register(ch)
		}
		if cfg.Channels.Default != "" {
			if err := registry.SetDefault(cfg.Channels.Default); err != nil {
				printWarning(fmt.Sprintf("Invalid default channel: %v", err))
			}
		}
	}

	// Create promotion use case
	promoteUC := versioning.NewPromoteReleaseUseCase(app.GitAdapter(), registry)

	input := versioning.PromoteReleaseInput{
		TagPrefix:   cfg.Versioning.TagPrefix,
		FromChannel: promoteFrom,
		ToChannel:   promoteTo,
		Version:     promoteVersion,
		DryRun:      dryRun,
	}

	output, err := promoteUC.Execute(ctx, input)
	if err != nil {
		return fmt.Errorf("promotion failed: %w", err)
	}

	if outputJSON {
		result := map[string]any{
			"source_version": output.SourceVersion.String(),
			"target_version": output.TargetVersion.String(),
			"from_channel":   output.FromChannel,
			"to_channel":     output.ToChannel,
			"promotion_path": output.PromotionPath,
			"promoted_at":    output.PromotedAt.Format("2006-01-02T15:04:05Z07:00"),
			"dry_run":        dryRun,
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}

	printInfo(fmt.Sprintf("Source:  %s (channel: %s)", output.SourceVersion.String(), output.FromChannel))
	printInfo(fmt.Sprintf("Target:  %s (channel: %s)", output.TargetVersion.String(), output.ToChannel))
	fmt.Println()

	if dryRun {
		printInfo("Dry run - no changes made")
	} else {
		tagName := cfg.Versioning.TagPrefix + output.TargetVersion.String()
		printSuccess(fmt.Sprintf("Promoted to %s", tagName))
		fmt.Println()
		printTitle("Next Steps")
		fmt.Println()
		fmt.Println("  1. Push the tag: git push origin " + tagName)
		fmt.Println("  2. Run 'relicta publish' to complete the release")
		fmt.Println()
	}

	return nil
}
