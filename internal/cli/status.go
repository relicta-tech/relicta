// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current release status",
	Long: `Display information about the current release state.

This command shows:
  - Active release run (if any)
  - Current state in the release workflow
  - Version being released
  - Risk assessment (if available)

Examples:
  # Check current release status
  relicta status

  # Output as JSON
  relicta status --json`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

// StatusOutput represents the status command output.
type StatusOutput struct {
	HasActiveRelease bool       `json:"has_active_release"`
	ReleaseID        string     `json:"release_id,omitempty"`
	State            string     `json:"state,omitempty"`
	CurrentVersion   string     `json:"current_version,omitempty"`
	NextVersion      string     `json:"next_version,omitempty"`
	BumpKind         string     `json:"bump_kind,omitempty"`
	RiskScore        float64    `json:"risk_score,omitempty"`
	CreatedAt        *time.Time `json:"created_at,omitempty"`
	UpdatedAt        *time.Time `json:"updated_at,omitempty"`
	CommitCount      int        `json:"commit_count,omitempty"`
	Message          string     `json:"message,omitempty"`
	NextSteps        []string   `json:"next_steps,omitempty"`
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

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

	output := &StatusOutput{}

	// Try to load the latest release run
	run, err := loadLatestReleaseRun(ctx, app, repoInfo.Path)
	if err != nil {
		// No active release
		output.HasActiveRelease = false
		output.Message = "No active release found. Run 'relicta plan' to start a new release."
		output.NextSteps = []string{"relicta plan"}
	} else {
		output.HasActiveRelease = true
		output.ReleaseID = string(run.ID())
		output.State = string(run.State())

		vCurrent := run.VersionCurrent()
		vNext := run.VersionNext()
		if !vCurrent.IsZero() {
			output.CurrentVersion = vCurrent.String()
		}
		if !vNext.IsZero() {
			output.NextVersion = vNext.String()
		}
		output.BumpKind = string(run.BumpKind())
		output.RiskScore = run.RiskScore()
		output.CommitCount = len(run.Commits())

		createdAt := run.CreatedAt()
		updatedAt := run.UpdatedAt()
		output.CreatedAt = &createdAt
		output.UpdatedAt = &updatedAt

		output.NextSteps = getNextSteps(run.State())
		output.Message = getStateMessage(run.State())
	}

	if outputJSON {
		return outputStatusJSON(output)
	}
	return outputStatusText(output)
}

func loadLatestReleaseRun(ctx context.Context, app cliApp, repoRoot string) (*domain.ReleaseRun, error) {
	if !app.HasReleaseServices() {
		return nil, fmt.Errorf("release services not available")
	}
	services := app.ReleaseServices()
	if services == nil || services.Repository == nil {
		return nil, fmt.Errorf("repository not available")
	}

	return services.Repository.LoadLatest(ctx, repoRoot)
}

func getNextSteps(state domain.RunState) []string {
	switch state {
	case domain.StateDraft:
		return []string{"relicta plan"}
	case domain.StatePlanned:
		return []string{"relicta bump"}
	case domain.StateVersioned:
		return []string{"relicta notes"}
	case domain.StateNotesReady:
		return []string{"relicta approve"}
	case domain.StateApproved:
		return []string{"relicta publish"}
	case domain.StatePublishing:
		return []string{"Wait for publish to complete or run 'relicta publish' to retry"}
	case domain.StatePublished:
		return []string{"Release complete! Run 'relicta plan' for next release"}
	case domain.StateFailed:
		return []string{"relicta reset", "Then run 'relicta plan' to start over"}
	case domain.StateCanceled:
		return []string{"relicta reset", "Then run 'relicta plan' to start over"}
	default:
		return []string{"relicta status"}
	}
}

func getStateMessage(state domain.RunState) string {
	switch state {
	case domain.StateDraft:
		return "Release is in draft state"
	case domain.StatePlanned:
		return "Release is planned, ready for version bump"
	case domain.StateVersioned:
		return "Version bumped, ready for release notes"
	case domain.StateNotesReady:
		return "Release notes generated, ready for approval"
	case domain.StateApproved:
		return "Release approved, ready to publish"
	case domain.StatePublishing:
		return "Release is being published..."
	case domain.StatePublished:
		return "Release published successfully"
	case domain.StateFailed:
		return "Release failed. Use 'relicta reset' to clear state"
	case domain.StateCanceled:
		return "Release was canceled. Use 'relicta reset' to clear state"
	default:
		return fmt.Sprintf("Release is in '%s' state", state)
	}
}

func outputStatusJSON(output *StatusOutput) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputStatusText(output *StatusOutput) error {
	printTitle("Release Status")
	fmt.Println()

	if !output.HasActiveRelease {
		printInfo(output.Message)
		fmt.Println()
		fmt.Println("Next step:")
		for _, step := range output.NextSteps {
			fmt.Printf("  $ %s\n", step)
		}
		return nil
	}

	// Show active release info
	fmt.Printf("  Release ID: %s\n", output.ReleaseID)
	fmt.Printf("  State:      %s\n", formatState(output.State))
	fmt.Println()

	if output.CurrentVersion != "" || output.NextVersion != "" {
		fmt.Println("Version:")
		if output.CurrentVersion != "" {
			fmt.Printf("  Current: %s\n", output.CurrentVersion)
		}
		if output.NextVersion != "" {
			fmt.Printf("  Next:    %s (%s)\n", output.NextVersion, output.BumpKind)
		}
		fmt.Println()
	}

	if output.CommitCount > 0 {
		fmt.Printf("Changes: %d commit(s)\n", output.CommitCount)
		fmt.Println()
	}

	if output.RiskScore > 0 {
		fmt.Printf("Risk Score: %.2f\n", output.RiskScore)
		fmt.Println()
	}

	if output.CreatedAt != nil {
		fmt.Printf("Created: %s\n", output.CreatedAt.Format(time.RFC3339))
	}
	if output.UpdatedAt != nil {
		fmt.Printf("Updated: %s\n", output.UpdatedAt.Format(time.RFC3339))
	}
	fmt.Println()

	// Show next steps
	fmt.Println("Next step:")
	for _, step := range output.NextSteps {
		fmt.Printf("  $ %s\n", step)
	}

	return nil
}

func formatState(state string) string {
	switch state {
	case "planned":
		return styles.Info.Render("planned")
	case "versioned":
		return styles.Info.Render("versioned")
	case "notes_ready":
		return styles.Info.Render("notes ready")
	case "approved":
		return styles.Success.Render("approved")
	case "publishing":
		return styles.Warning.Render("publishing")
	case "published":
		return styles.Success.Render("published")
	case "failed":
		return styles.Error.Render("failed")
	case "canceled":
		return styles.Warning.Render("canceled")
	default:
		return state
	}
}
