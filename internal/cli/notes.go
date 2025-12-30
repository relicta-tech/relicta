// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	releaseapp "github.com/relicta-tech/relicta/internal/domain/release/app"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
)

var (
	notesOutput       string
	notesTone         string
	notesAudience     string
	notesIncludeEmoji bool
	notesLanguage     string
	notesUseAI        bool
)

func init() {
	notesCmd.Flags().StringVarP(&notesOutput, "output", "o", "", "output file (default: stdout)")
	notesCmd.Flags().StringVar(&notesTone, "tone", "", "AI tone (technical, friendly, professional, marketing)")
	notesCmd.Flags().StringVar(&notesAudience, "audience", "", "target audience (developers, users, public, stakeholders)")
	notesCmd.Flags().BoolVar(&notesIncludeEmoji, "emoji", false, "include emojis in output")
	notesCmd.Flags().StringVar(&notesLanguage, "language", "English", "output language")
	notesCmd.Flags().BoolVar(&notesUseAI, "ai", false, "use AI to generate notes (requires OPENAI_API_KEY)")
}

// buildNotesInputForServices creates the input for the GenerateNotes use case.
func buildNotesInputForServices(repoRoot string, hasAI bool) releaseapp.GenerateNotesInput {
	return releaseapp.GenerateNotesInput{
		RepoRoot: repoRoot,
		Options: ports.NotesOptions{
			AudiencePreset: notesAudience,
			TonePreset:     notesTone,
			UseAI:          notesUseAI && hasAI,
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

	// Output results
	if outputJSON {
		return outputNotesJSONFromServices(ctx, output, repoPath, app)
	}

	// Output notes
	fmt.Println()
	printTitle("Release Notes")
	fmt.Println()
	if output.Notes != nil {
		fmt.Println(output.Notes.Text)
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

// outputNotesJSONFromServices outputs notes as JSON from domain services.
func outputNotesJSONFromServices(ctx context.Context, output *releaseapp.GenerateNotesOutput, repoPath string, app cliApp) error {
	result := map[string]any{
		"release_id":   string(output.RunID),
		"inputs_hash":  output.InputsHash,
		"ai_generated": output.Notes != nil && output.Notes.Provider != "" && output.Notes.Provider != "basic",
	}

	if output.Notes != nil {
		result["release_notes"] = output.Notes.Text
		result["tone_preset"] = output.Notes.TonePreset
		result["audience_preset"] = output.Notes.AudiencePreset
		result["provider"] = output.Notes.Provider
		result["model"] = output.Notes.Model
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
