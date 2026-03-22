package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/domain/communication"
)

var (
	commAudiences    string
	commOutputFormat string
	commOutputDir    string
	commProductName  string
)

func init() {
	communicateCmd.Flags().StringVar(&commAudiences, "audiences", "all", "comma-separated audiences or 'all' (engineering,product,executive,external)")
	communicateCmd.Flags().StringVar(&commOutputFormat, "format", "markdown", "output format (markdown, plaintext, html)")
	communicateCmd.Flags().StringVarP(&commOutputDir, "output-dir", "o", "", "directory to write audience-specific files (default: stdout)")
	communicateCmd.Flags().StringVar(&commProductName, "product", "", "product name for branding")

	rootCmd.AddCommand(communicateCmd)
}

var communicateCmd = &cobra.Command{
	Use:   "communicate",
	Short: "Generate audience-specific release narratives",
	Long: `Generate release communication tailored to different audiences.

Produces audience-specific narratives from the current release changes.
Supports engineering, product, executive, and external audiences.

Examples:
  relicta communicate --audiences all
  relicta communicate --audiences engineering,product --format html
  relicta communicate --audiences executive --output-dir ./release-comms`,
	RunE: runCommunicate,
}

func runCommunicate(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	printTitle("Audience-Aware Release Communication")
	fmt.Println()

	// Initialize container
	app, err := newContainerApp(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer closeApp(app)

	// Get repository info
	gitAdapter := app.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	// Initialize release services
	if err := app.InitReleaseServices(ctx, repoInfo.Path); err != nil {
		return fmt.Errorf("failed to initialize release services: %w", err)
	}

	if !app.HasReleaseServices() {
		return fmt.Errorf("release services not available; run 'relicta plan' first")
	}

	services := app.ReleaseServices()
	if services == nil || services.Repository == nil {
		return fmt.Errorf("release repository not available")
	}

	// Load latest run
	run, err := services.Repository.LoadLatest(ctx, repoInfo.Path)
	if err != nil {
		return fmt.Errorf("failed to load release run: %w", err)
	}

	if !run.HasChangeSet() {
		return fmt.Errorf("no changeset found in release run; run 'relicta plan' first")
	}

	// Resolve audiences
	audiences, err := resolveAudiences(commAudiences)
	if err != nil {
		return err
	}

	// Validate output format
	format := communication.OutputFormat(commOutputFormat)
	if !communication.IsValidOutputFormat(commOutputFormat) {
		return fmt.Errorf("invalid output format %q: use markdown, plaintext, or html", commOutputFormat)
	}

	// Bundle changes
	bundler := communication.NewBundler()
	bundles := bundler.BundleChanges(run.ChangeSet())

	// Build narrative input
	input := communication.NarrativeInput{
		Version:     run.VersionNext().String(),
		ProductName: commProductName,
		Bundles:     bundles,
		ChangeSet:   run.ChangeSet(),
		Format:      format,
	}
	currentVer := run.VersionCurrent()
	if currentVer.String() != "" && currentVer.String() != "0.0.0" {
		input.PreviousVersion = currentVer.String()
	}

	// Create narrative generator
	var aiCompleter communication.AICompleter
	if app.HasAI() {
		aiCompleter = app.AI()
	}
	generator := communication.NewNarrativeGenerator(aiCompleter)

	return generateAndOutputNarratives(ctx, generator, input, audiences)
}

func generateAndOutputNarratives(ctx context.Context, gen *communication.NarrativeGenerator, input communication.NarrativeInput, audiences []communication.Audience) error {
	// Generate spinner (only in interactive mode)
	var spinner *Spinner
	if !outputJSON {
		spinner = NewSpinner("Generating audience-specific narratives...")
		spinner.Start()
	}

	narratives, err := gen.GenerateAll(ctx, input, audiences)

	if spinner != nil {
		spinner.Stop()
	}

	if err != nil {
		return fmt.Errorf("failed to generate narratives: %w", err)
	}

	// Output
	if outputJSON {
		return outputNarrativesJSON(narratives)
	}

	for i, n := range narratives {
		if i > 0 {
			fmt.Println()
			fmt.Println(strings.Repeat("=", 60))
			fmt.Println()
		}

		printTitle(fmt.Sprintf("Audience: %s", n.Audience))
		fmt.Println()
		fmt.Println(n.Body)

		// Write to file if output dir specified
		if commOutputDir != "" {
			filename := fmt.Sprintf("%s/%s.%s", commOutputDir, n.Audience, fileExtension(n.Format))
			if err := os.MkdirAll(commOutputDir, 0o755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}
			if err := os.WriteFile(filename, []byte(n.Body), filePermReadable); err != nil {
				return fmt.Errorf("failed to write %s: %w", filename, err)
			}
			printSuccess(fmt.Sprintf("Written to %s", filename))
		}
	}

	return nil
}

func outputNarrativesJSON(narratives []*communication.Narrative) error {
	results := make([]map[string]any, len(narratives))
	for i, n := range narratives {
		results[i] = map[string]any{
			"audience":     string(n.Audience),
			"title":        n.Title,
			"body":         n.Body,
			"format":       string(n.Format),
			"provider":     n.Provider,
			"generated_at": n.GeneratedAt,
		}
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
}

func resolveAudiences(audienceStr string) ([]communication.Audience, error) {
	commConfig := communication.DefaultCommunicationConfig()
	if cfg != nil && cfg.Communication.DefaultAudience != "" {
		commConfig.DefaultAudience = communication.AudienceType(cfg.Communication.DefaultAudience)
		if cfg.Communication.Audiences != nil {
			commConfig.Audiences = make(map[communication.AudienceType]communication.AudienceConfig)
			for k, v := range cfg.Communication.Audiences {
				commConfig.Audiences[communication.AudienceType(k)] = communication.AudienceConfig{
					Name:         v.Name,
					Tone:         v.Tone,
					DetailLevel:  v.DetailLevel,
					Sections:     v.Sections,
					CustomPrompt: v.CustomPrompt,
				}
			}
		}
	}

	if audienceStr == "all" {
		return commConfig.ResolveAllAudiences()
	}

	parts := strings.Split(audienceStr, ",")
	var audiences []communication.Audience
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if !communication.IsValidAudienceType(p) {
			return nil, fmt.Errorf("invalid audience %q: valid types are engineering, product, executive, external", p)
		}
		aud, err := commConfig.ResolveAudience(communication.AudienceType(p))
		if err != nil {
			return nil, err
		}
		audiences = append(audiences, aud)
	}

	return audiences, nil
}

func fileExtension(format communication.OutputFormat) string {
	switch format {
	case communication.OutputHTML:
		return "html"
	case communication.OutputPlainText:
		return "txt"
	default:
		return "md"
	}
}
