// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	apprelease "github.com/relicta-tech/relicta/internal/application/release"
	"github.com/relicta-tech/relicta/internal/domain/communication"
	"github.com/relicta-tech/relicta/internal/domain/release"
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

// parseNoteTone parses the tone flag and returns the corresponding NoteTone.
func parseNoteTone(tone string) communication.NoteTone {
	switch tone {
	case "technical":
		return communication.ToneTechnical
	case "friendly":
		return communication.ToneFriendly
	case "professional":
		return communication.ToneProfessional
	case "marketing":
		return communication.ToneMarketing
	default:
		return communication.ToneProfessional
	}
}

// parseNoteAudience parses the audience flag and returns the corresponding NoteAudience.
func parseNoteAudience(audience string) communication.NoteAudience {
	switch audience {
	case "developers":
		return communication.AudienceDevelopers
	case "users":
		return communication.AudienceUsers
	case "public":
		return communication.AudiencePublic
	case "stakeholders":
		return communication.AudienceStakeholders
	default:
		return communication.AudienceDevelopers
	}
}

// buildGenerateNotesInput creates the input for the GenerateNotes use case.
func buildGenerateNotesInput(rel *release.ReleaseRun, hasAI bool) apprelease.GenerateNotesInput {
	return apprelease.GenerateNotesInput{
		ReleaseID:        rel.ID(),
		UseAI:            notesUseAI && hasAI,
		Tone:             parseNoteTone(notesTone),
		Audience:         parseNoteAudience(notesAudience),
		IncludeChangelog: true,
		RepositoryURL:    cfg.Changelog.RepositoryURL,
	}
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

// writeNotesToFile writes the release notes to a file.
func writeNotesToFile(output *apprelease.GenerateNotesOutput, filename string) error {
	content := output.ReleaseNotes.Render()
	if err := os.WriteFile(filename, []byte(content), filePermReadable); err != nil {
		return fmt.Errorf("failed to write notes to file: %w", err)
	}
	printSuccess(fmt.Sprintf("Release notes written to %s", filename))
	return nil
}

// outputNotesToStdout outputs the release notes to stdout.
func outputNotesToStdout(output *apprelease.GenerateNotesOutput) {
	fmt.Println()
	if output.Changelog != nil {
		printTitle("Changelog")
		fmt.Println()
		fmt.Println(output.Changelog.Render())
		fmt.Println()
	}
	printTitle("Release Notes")
	fmt.Println()
	fmt.Println(output.ReleaseNotes.Render())
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

	// Try domain services first
	if err := app.InitReleaseServices(ctx, repoInfo.Path); err == nil && app.HasReleaseServices() {
		services := app.ReleaseServices()
		if services != nil && services.GenerateNotes != nil {
			return runNotesWithServices(ctx, app, repoInfo.Path)
		}
	}

	// Fall back to legacy
	return runNotesLegacy(ctx, app, repoInfo.Path)
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

// runNotesLegacy generates notes using the legacy use case.
func runNotesLegacy(ctx context.Context, app cliApp, repoPath string) error {
	// Find the latest release
	releaseRepo := app.ReleaseRepository()
	rel, err := releaseRepo.FindLatest(ctx, repoPath)
	if err != nil {
		printError("No release in progress")
		printInfo("Run 'relicta plan' to start a new release")
		return fmt.Errorf("no release state found")
	}

	// Build input and execute use case
	input := buildGenerateNotesInput(rel, app.HasAI())

	// Show spinner (unless JSON output)
	var spinner *Spinner
	if !outputJSON {
		spinnerMsg := "Generating release notes..."
		if input.UseAI {
			spinnerMsg = "Generating release notes with AI..."
		}
		spinner = NewSpinner(spinnerMsg)
		spinner.Start()
	}

	output, err := app.GenerateNotes().Execute(ctx, input)

	if spinner != nil {
		spinner.Stop()
	}

	if err != nil {
		return fmt.Errorf("failed to generate notes: %w", err)
	}

	// Output results
	if outputJSON {
		return outputNotesJSON(output, rel)
	}

	// Write to file or stdout
	if notesOutput != "" {
		if err := writeNotesToFile(output, notesOutput); err != nil {
			return err
		}
	} else {
		outputNotesToStdout(output)
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

// outputNotesJSON outputs the notes as JSON.
func outputNotesJSON(output *apprelease.GenerateNotesOutput, rel *release.ReleaseRun) error {
	result := map[string]any{
		"release_id": string(rel.ID()),
		"state":      string(rel.State()),
	}

	if plan := release.GetPlan(rel); plan != nil {
		result["version"] = plan.NextVersion.String()
	}

	if output.Changelog != nil {
		result["changelog"] = output.Changelog.Render()
	}

	if output.ReleaseNotes != nil {
		result["release_notes"] = output.ReleaseNotes.Render()
		result["summary"] = output.ReleaseNotes.Summary()
		result["ai_generated"] = output.ReleaseNotes.IsAIGenerated()
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
