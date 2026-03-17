package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/application/governance"
	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/domain/release"
)

var evaluateCmd = &cobra.Command{
	Use:   "evaluate",
	Short: "Evaluate release risk and governance decision",
	Long: `Evaluate the current release against CGP governance rules.

This command runs policy evaluation and risk assessment for the active
release, then shows the decision and required actions.

Examples:
  # Evaluate current release
  relicta evaluate

  # Machine-readable output
  relicta evaluate --json`,
	RunE: runEvaluate,
}

func init() {
	rootCmd.AddCommand(evaluateCmd)
}

type evaluateOutput struct {
	ReleaseID       string               `json:"release_id"`
	Decision        cgp.DecisionType     `json:"decision"`
	RiskScore       float64              `json:"risk_score"`
	Severity        cgp.Severity         `json:"severity"`
	CanAutoApprove  bool                 `json:"can_auto_approve"`
	RequiredActions []cgp.RequiredAction `json:"required_actions,omitempty"`
	RiskFactors     []cgp.RiskFactor     `json:"risk_factors,omitempty"`
	Rationale       []string             `json:"rationale,omitempty"`
}

func runEvaluate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	app, err := newContainerApp(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer closeApp(app)

	if !app.HasGovernance() || app.GovernanceService() == nil {
		return fmt.Errorf("governance service not available - enable governance in .relicta.yaml")
	}

	rel, err := getLatestReleaseForEvaluate(ctx, app)
	if err != nil {
		return err
	}

	result, err := evaluateGovernance(ctx, app, rel)
	if err != nil {
		return fmt.Errorf("governance evaluation failed: %w", err)
	}

	if outputJSON {
		return outputEvaluateJSON(string(rel.ID()), result)
	}

	printTitle("Governance Evaluation")
	fmt.Println()
	displayGovernanceResult(result)
	printEvaluateNextStep(result)
	return nil
}

func outputEvaluateJSON(releaseID string, result *governance.EvaluateReleaseOutput) error {
	out := evaluateOutput{
		ReleaseID:       releaseID,
		Decision:        result.Decision,
		RiskScore:       result.RiskScore,
		Severity:        result.Severity,
		CanAutoApprove:  result.CanAutoApprove,
		RequiredActions: result.RequiredActions,
		RiskFactors:     result.RiskFactors,
		Rationale:       result.Rationale,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func printEvaluateNextStep(result *governance.EvaluateReleaseOutput) {
	fmt.Println()
	fmt.Println("Next step:")

	switch result.Decision {
	case cgp.DecisionApproved:
		fmt.Println("  $ relicta approve")
	case cgp.DecisionApprovalRequired, cgp.DecisionDeferred:
		fmt.Println("  $ relicta approve")
	case cgp.DecisionRejected:
		fmt.Println("  Review blocking policies, then run: $ relicta plan")
	default:
		fmt.Println("  $ relicta status")
	}
}

// getLatestReleaseForEvaluate loads the latest release without user-facing banner output.
func getLatestReleaseForEvaluate(ctx context.Context, app cliApp) (*release.ReleaseRun, error) {
	gitAdapter := app.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	releaseRepo := app.ReleaseRepository()
	rel, err := releaseRepo.FindLatest(ctx, repoInfo.Path)
	if err != nil {
		return nil, fmt.Errorf("no release state found: %w", err)
	}
	return rel, nil
}
