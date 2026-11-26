// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	releaseapp "github.com/relicta-tech/relicta/v5/internal/domain/release/app"
	"github.com/relicta-tech/relicta/v5/internal/domain/release/ports"
)

var (
	notesOutput       string
	notesDraftFile    string
	notesTone         string
	notesAudience     string
	notesIncludeEmoji bool
	notesLanguage     string
)

func init() {
	notesCmd.Flags().StringVarP(&notesOutput, "output", "o", "", "output file (default: stdout)")
	notesCmd.Flags().StringVar(&notesDraftFile, "draft-file", "", "read a customer-facing draft from a file (relative paths use the repository root)")
	notesCmd.Flags().StringVarP(&notesTone, "tone", "t", "", "AI tone (technical, friendly, professional, marketing)")
	notesCmd.Flags().StringVarP(&notesAudience, "audience", "a", "", "target audience (developers, users, public, stakeholders)")
	notesCmd.Flags().BoolVar(&notesIncludeEmoji, "emoji", false, "include emojis in output")
	notesCmd.Flags().StringVarP(&notesLanguage, "language", "l", "English", "output language")
}

// buildNotesInputForServices creates the input for the GenerateNotes use case.
func buildNotesInputForServices(repoRoot string, hasAI bool) releaseapp.GenerateNotesInput {
	return releaseapp.GenerateNotesInput{
		RepoRoot: repoRoot,
		Options: ports.NotesOptions{
			AudiencePreset: notesAudience,
			TonePreset:     notesTone,
			IncludeEmoji:   notesIncludeEmoji,
			UseAI:          cfg.AI.Enabled && hasAI,
			Language:       notesLanguage,
			RepositoryURL:  cfg.Changelog.RepositoryURL,
		},
		Actor: ports.ActorInfo{
			Type: "user",
			ID:   "cli",
		},
		Force: false,
	}
}

// printNotesNextSteps prints the next steps after generating notes.
func printNotesNextSteps() {
	fmt.Println()
	printTitle("Next Steps")
	fmt.Println()
	fmt.Println("  1. Review the generated notes above")
	fmt.Println("  2. Run 'relicta approve' to review and approve")
	fmt.Println("  3. Run 'relicta publish' to execute the release")
	fmt.Println()
}

// runNotes implements the notes command.
func runNotes(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// ai.include_emoji becomes the default for --emoji, so the setting and the flag are one
	// answer rather than two. Folded here, as --skip-push is folded into versioning.git_push,
	// and only when the flag was not given: an explicit `--emoji=false` has to beat a config
	// that asks for them.
	if !cmd.Flags().Changed("emoji") && cfg != nil {
		notesIncludeEmoji = cfg.AI.IncludeEmoji
	}

	printTitle("Release Notes Generation")
	fmt.Println()

	// Initialize container
	app, err := newContainerApp(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer closeApp(app)

	// Get latest release from repository
	gitAdapter := app.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	// Initialize domain services
	if err := app.InitReleaseServices(ctx, repoInfo.Path); err != nil {
		return fmt.Errorf("failed to initialize release services: %w", err)
	}

	if !app.HasReleaseServices() {
		return fmt.Errorf("release services not available")
	}
	if cfg != nil && cfg.AI.Enabled && !app.HasAI() {
		printWarning("AI notes were requested but no configured provider is available; generating a deterministic changelog instead.")
	}

	services := app.ReleaseServices()
	if services == nil || services.GenerateNotes == nil {
		return fmt.Errorf("GenerateNotes use case not available")
	}

	return runNotesWithServices(ctx, app, repoInfo.Path)
}

// runNotesWithServices generates notes using the GenerateNotesUseCase.
func runNotesWithServices(ctx context.Context, app cliApp, repoPath string) error {
	services := app.ReleaseServices()

	// Build input
	input := buildNotesInputForServices(repoPath, app.HasAI())
	if notesDraftFile != "" {
		draftPath := notesDraftFile
		if !filepath.IsAbs(draftPath) {
			draftPath = filepath.Join(repoPath, draftPath)
		}
		draft, err := os.ReadFile(draftPath)
		if err != nil {
			return fmt.Errorf("failed to read release-notes draft %q: %w", notesDraftFile, err)
		}
		if strings.TrimSpace(string(draft)) == "" {
			return fmt.Errorf("release-notes draft %q is empty", notesDraftFile)
		}
		input.DraftText = string(draft)
	}
	if !input.Options.UseAI && (notesAudience != "" || notesTone != "" || notesLanguage != "English") {
		printWarning("--audience, --tone, and --language only affect AI-generated notes. Enable AI in .relicta.yaml.")
	}

	// Tell the user which auto-detected provider will serve this request —
	// only now that AI is actually being used (issue #127: this notice used
	// to print eagerly on every command).
	if input.Options.UseAI && cfg != nil && cfg.AI.AutoSelected != "" && len(cfg.AI.DetectedProviders) > 1 {
		printWarning(fmt.Sprintf("Multiple AI provider API keys detected: %s", strings.Join(cfg.AI.DetectedProviders, ", ")))
		printSubtle(fmt.Sprintf("   Auto-selected '%s' based on priority order. Configure ai.provider to override.", cfg.AI.AutoSelected))
	}

	// Show spinner (unless JSON output)
	var spinner *Spinner
	if !outputJSON {
		spinnerMsg := "Generating release notes..."
		if input.Options.UseAI {
			spinnerMsg = "Generating release notes with AI..."
		}
		spinner = NewSpinner(spinnerMsg)
		spinner.Start()
	}

	output, err := services.GenerateNotes.Execute(ctx, input)

	if spinner != nil {
		spinner.Stop()
	}

	if err != nil {
		return fmt.Errorf("failed to generate notes: %w", err)
	}

	// Each package's run gets its own notes, from its own changeset. Approval reads them, so
	// without this a package could be planned and versioned and then never approved.
	generatePackageNotes(ctx, app, repoPath)

	// Output results
	if outputJSON {
		return outputNotesJSONFromServices(ctx, output, repoPath, app)
	}

	// Output notes
	fmt.Println()
	printTitle("Release Notes")
	if output.Notes != nil {
		fmt.Printf("Source: %s\n", notesSourceLabel(output.Notes.Provider))
		if output.Notes.Provider == "changelog-fallback" {
			printWarning("AI release-note prose could not be generated; the saved text contains only the changelog. Rewrite it for customers before approval.")
		} else if input.Options.UseAI && output.Notes.Provider == "basic" {
			printWarning("AI-generated release-note prose was not produced; deterministic changelog text was saved. Rewrite it for customers before approval.")
		}
		fmt.Println()
		fmt.Println(output.Notes.Text)
	} else {
		fmt.Println()
	}

	// Write to file if specified
	if notesOutput != "" {
		if output.Notes != nil {
			if err := os.WriteFile(notesOutput, []byte(output.Notes.Text), filePermReadable); err != nil {
				return fmt.Errorf("failed to write notes to file: %w", err)
			}
			printSuccess(fmt.Sprintf("Release notes written to %s", notesOutput))
		}
	}

	printNotesNextSteps()
	return nil
}

// notesSourceLabel describes how the persisted release notes were authored.
func notesSourceLabel(provider string) string {
	switch provider {
	case "", "basic":
		return "deterministic template"
	case "agent":
		return "calling agent draft"
	case "changelog-fallback":
		return "AI changelog only; release-note prose unavailable"
	default:
		return "AI-generated (" + provider + ")"
	}
}

// outputNotesJSONFromServices outputs notes as JSON from domain services.
func outputNotesJSONFromServices(ctx context.Context, output *releaseapp.GenerateNotesOutput, repoPath string, app cliApp) error {
	result := map[string]any{
		"release_id":   string(output.RunID),
		"inputs_hash":  output.InputsHash,
		"ai_generated": output.Notes != nil && output.Notes.Provider != "" && output.Notes.Provider != "basic" && output.Notes.Provider != "agent",
	}

	if output.Notes != nil {
		result["release_notes"] = output.Notes.Text
		result["tone_preset"] = output.Notes.TonePreset
		result["audience_preset"] = output.Notes.AudiencePreset
		result["provider"] = output.Notes.Provider
		result["model"] = output.Notes.Model
		if output.Notes.Provider == "changelog-fallback" {
			result["warning"] = "AI release-note prose could not be generated; the saved text contains only the changelog. Rewrite it for customers before approval."
		} else if cfg != nil && cfg.AI.Enabled && output.Notes.Provider == "basic" {
			result["warning"] = "AI-generated release-note prose was not produced; deterministic changelog text was saved. Rewrite it for customers before approval."
		}
	}

	// Try to get version from the release
	if app.HasReleaseServices() {
		services := app.ReleaseServices()
		if services != nil && services.Repository != nil {
			if run, err := services.Repository.LoadLatest(ctx, repoPath); err == nil {
				result["version"] = run.VersionNext().String()
				result["state"] = string(run.State())
			}
		}
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
