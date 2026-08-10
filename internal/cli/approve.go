// Package cli provides the command-line interface for Relicta.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/application/governance"
	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/domain/release"
	releaseapp "github.com/relicta-tech/relicta/v4/internal/domain/release/app"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/v4/internal/ui"
	pkgcgp "github.com/relicta-tech/relicta/v4/pkg/cgp"
)

// emitApprovalCardJSON writes the canonical ApprovalCard JSON to stdout.
// Used by `relicta approve --json` and any CI script that parses approval
// results without re-implementing the schema.
func emitApprovalCardJSON(card pkgcgp.ApprovalCard) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(card); err != nil {
		return fmt.Errorf("emit approval card: %w", err)
	}
	return nil
}

// approvalCardVersion safely extracts the proposed version string from a
// release run, returning empty if the next version is the zero value.
func approvalCardVersion(rel *release.ReleaseRun) string {
	if rel == nil {
		return ""
	}
	v := rel.VersionNext()
	if v.IsZero() {
		return ""
	}
	return v.String()
}

const (
	// maxNotesPreviewLines is the maximum number of lines to show in release notes preview.
	maxNotesPreviewLines = 20

	// filePermReadable is file permission for user-readable files.
	filePermReadable = 0o644

	// filePermPrivate is restrictive file permission (owner read/write only).
	filePermPrivate = 0o600
)

var (
	approveYes            bool
	approveOverride       bool
	approveOverrideReason string
	approveEdit           bool
	approveEditor         string
	approveInteractive    bool
)

var runApprovalTUI = ui.RunApprovalTUI

func init() {
	approveCmd.Flags().BoolVarP(&approveYes, "yes", "y", false, "automatically approve without prompting")
	approveCmd.Flags().BoolVar(&approveOverride, "override-governance", false,
		"approve despite governance requiring human review; requires --reason and is recorded as an override")
	approveCmd.Flags().StringVar(&approveOverrideReason, "reason", "",
		"why governance is being overridden (required with --override-governance)")
	approveCmd.Flags().BoolVarP(&approveEdit, "edit", "e", false, "edit release notes before approving")
	approveCmd.Flags().StringVarP(&approveEditor, "editor", "E", "", "editor to use (default: $EDITOR or vim)")
	approveCmd.Flags().BoolVarP(&approveInteractive, "interactive", "i", false, "use interactive TUI for approval")
}

// getLatestRelease retrieves the latest release from the repository.
func getLatestRelease(ctx context.Context, app cliApp) (*release.ReleaseRun, error) {
	gitAdapter := app.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	// Load through the release services' repository, the same one plan wrote with.
	//
	// app.ReleaseRepository() is a second implementation of the same aggregate
	// that reconstructs runs without commits, HEAD or a changeset, so governance
	// could not evaluate them: `relicta approve --ci` loaded a run reporting
	// commit_count 0 and current_version 0.0.0, governance failed with "either
	// commitRange or commits is required", and — because that failure is only a
	// warning outside strict mode — the release was approved with no governance
	// applied at all. Consolidating the two is tracked in roady; reading from the
	// one that has the data is what makes the gate able to run.
	if err := app.InitReleaseServices(ctx, repoInfo.Path); err != nil {
		return nil, fmt.Errorf("failed to initialize release services: %w", err)
	}
	rel, err := loadLatestReleaseRun(ctx, app, repoInfo.Path)
	if err != nil {
		printError("No release in progress")
		printInfo("Run 'relicta plan' to start a new release")
		return nil, fmt.Errorf("no release state found: %w", err)
	}

	// Show which release run is being used for transparency
	displayLoadedRelease(rel)

	return rel, nil
}

// displayLoadedRelease shows information about the loaded release run.
// This helps users understand which run is being used, especially when
// there are multiple runs in the .relicta/releases directory.
func displayLoadedRelease(rel *release.ReleaseRun) {
	summary := rel.Summary()
	version := summary.VersionNext
	if version == "" {
		version = "(pending)"
	}

	// Use short ID for cleaner display
	id := string(rel.ID())
	if len(id) > 12 {
		id = id[:12]
	}

	printInfo(fmt.Sprintf("Using release %s (v%s, %s)", id, version, summary.State))
}

// isReleaseAlreadyApproved checks if the release is already approved and prints info.
func isReleaseAlreadyApproved(rel *release.ReleaseRun) bool {
	if rel.State() == release.StateApproved || rel.IsApproved() {
		if !outputJSON {
			printInfo("Release already approved")
			printInfo("Run 'relicta publish' to execute the release")
		}
		return true
	}
	return false
}

// validateReleaseStateForApproval checks if the release is in a valid state for approval.
// Returns an error with guidance if the state is invalid.
func validateReleaseStateForApproval(rel *release.ReleaseRun) error {
	state := rel.State()

	switch state {
	case release.StateNotesReady:
		// Ideal state - ready for approval
		return nil
	case release.StatePlanned, release.StateVersioned:
		// Allow but warn about missing notes
		printWarning("Release notes have not been generated")
		printInfo("Consider running 'relicta notes' first for better release documentation")
		return nil
	case release.StateDraft:
		printError("Release has not been planned yet")
		printInfo("Run 'relicta plan' to analyze commits and prepare the release")
		return fmt.Errorf("release in state '%s' cannot be approved - run 'relicta plan' first", state)
	case release.StatePublishing:
		printError("Release is currently being published")
		printInfo("Wait for the publish operation to complete")
		return fmt.Errorf("release in state '%s' cannot be approved - publication in progress", state)
	case release.StatePublished:
		printInfo("Release has already been published")
		printInfo("Nothing to approve - this release is complete")
		return fmt.Errorf("release in state '%s' is already complete", state)
	case release.StateFailed:
		printError("Previous release attempt failed")
		printInfo("Run 'relicta plan' to start a fresh release")
		return fmt.Errorf("release in state '%s' cannot be approved - start fresh with 'relicta plan'", state)
	default:
		return fmt.Errorf("release in unexpected state '%s'", state)
	}
}

// shouldUseInteractiveApproval returns true if interactive TUI should be used.
func shouldUseInteractiveApproval() bool {
	return approveInteractive && !ciMode && !approveYes
}

// handleNotesEditing handles editing of release notes if requested.
func handleNotesEditing(rel *release.ReleaseRun) (*string, error) {
	if !approveEdit || rel.Notes() == nil {
		return nil, nil
	}

	notes, err := editReleaseNotes(rel.Notes().Text)
	if err != nil {
		return nil, fmt.Errorf("failed to edit release notes: %w", err)
	}
	fmt.Println()
	printInfo("Notes edited - changes will be applied during approval")
	return &notes, nil
}

// promptForApproval prompts the user for approval confirmation.
// Returns true if approved, false otherwise.
func promptForApproval() (bool, error) {
	if approveYes || ciMode || !cfg.Workflow.RequireApproval {
		return true, nil
	}

	fmt.Println()
	fmt.Print("Do you approve this release? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read input: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

// getApproverName returns the name of the approver from environment.
func getApproverName() string {
	approvedBy := os.Getenv("USER")
	if approvedBy == "" {
		return "unknown"
	}
	return approvedBy
}

// executeApproval executes the approval use case using domain services and
// returns the authoritative approval output.
func executeApproval(ctx context.Context, app cliApp, rel *release.ReleaseRun, editedNotes *string) (*releaseapp.ApproveReleaseOutput, error) {
	// Get repository info for domain services
	gitAdapter := app.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	// Initialize domain services
	if err := app.InitReleaseServices(ctx, repoInfo.Path); err != nil {
		return nil, fmt.Errorf("failed to initialize release services: %w", err)
	}
	if !app.HasReleaseServices() {
		return nil, fmt.Errorf("release services not available")
	}
	services := app.ReleaseServices()
	if services == nil || services.ApproveRelease == nil {
		return nil, fmt.Errorf("ApproveRelease use case not available")
	}
	return executeApprovalWithServices(ctx, app, repoInfo.Path, rel, editedNotes)
}

// executeApprovalWithServices executes approval using the ApproveReleaseUseCase.
func executeApprovalWithServices(ctx context.Context, app cliApp, repoPath string, rel *release.ReleaseRun, editedNotes *string) (*releaseapp.ApproveReleaseOutput, error) {
	services := app.ReleaseServices()

	// Enforce the per-actor autonomy budget before recording approval — the
	// CLI previously skipped this gate that the MCP surface applies.
	if err := enforceActorBudget("approve", rel.RiskScore()); err != nil {
		return nil, err
	}

	// Handle edited notes separately - update the release before approval
	if editedNotes != nil {
		// Update notes on the release and save
		releaseRepo := app.ReleaseRepository()
		if err := rel.UpdateNotesText(*editedNotes); err != nil {
			return nil, fmt.Errorf("failed to update notes: %w", err)
		}
		if err := releaseRepo.Save(ctx, rel); err != nil {
			return nil, fmt.Errorf("failed to save release with updated notes: %w", err)
		}
	}

	input := releaseapp.ApproveReleaseInput{
		RepoRoot: repoPath,
		RunID:    rel.ID(),
		Actor: ports.ActorInfo{
			Type: approverActorType(),
			ID:   getApproverName(),
		},
		AutoApprove: approveYes,
		Force:       true, // Force since we've already validated state

		// An override's reason is the audit trail's only record of why a
		// governance gate was bypassed, so it is marked as such. Without this it
		// was printed to the terminal and persisted nowhere — the approval looked
		// like any other.
		Justification: approvalJustification(),
	}

	out, err := services.ApproveRelease.Execute(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to approve release: %w", err)
	}
	return out, nil
}

// printApproveNextSteps prints the next steps after approval.
func printApproveNextSteps() {
	printSuccess("Release approved")
	fmt.Println()

	printTitle("Next Steps")
	fmt.Println()
	fmt.Println("  Run 'relicta publish' to execute the release")
	fmt.Println()
}

// runApprove implements the approve command.
func runApprove(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// An override with no reason is worthless in an audit trail: six months later
	// the record would say a governance gate was bypassed and not why. Refuse it
	// up front rather than accept an empty justification.
	if approveOverride && strings.TrimSpace(approveOverrideReason) == "" {
		return errors.New("--override-governance requires --reason explaining why " +
			"the governance gate is being bypassed; it is recorded in the audit trail")
	}
	if !approveOverride && strings.TrimSpace(approveOverrideReason) != "" {
		return errors.New("--reason only applies with --override-governance")
	}

	if !outputJSON {
		printTitle("Release Approval")
		fmt.Println()
	}

	// Initialize container
	app, err := newContainerApp(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer closeApp(app)

	// Get latest release
	rel, err := getLatestRelease(ctx, app)
	if err != nil {
		return err
	}

	// Check if already approved (idempotent success)
	if isReleaseAlreadyApproved(rel) {
		if outputJSON {
			return outputApproveJSON(rel)
		}
		return nil
	}

	// Validate release is in a valid state for approval
	if err := validateReleaseStateForApproval(rel); err != nil {
		return err
	}

	// JSON output is non-interactive — refuse to prompt mid-stream rather
	// than silently skipping the approval. The previous code returned a
	// status dump here WITHOUT approving, so `approve --ci` exited 0 as a
	// no-op and the release never advanced (issue #136). JSON mode now
	// runs the same approval flow (including governance evaluation) and
	// emits the result at the end.
	if outputJSON && !approveYes && !ciMode && cfg.Workflow.RequireApproval {
		return fmt.Errorf("approve --json is non-interactive: pass --ci or --yes, or set workflow.require_approval=false")
	}

	// Use interactive TUI if requested
	if shouldUseInteractiveApproval() {
		return runInteractiveApproval(ctx, app, rel)
	}

	// Display release summary
	if !outputJSON {
		displayReleaseSummary(rel)
	}

	// CGP Governance evaluation (if enabled)
	var govResult *governance.EvaluateReleaseOutput
	if app.HasGovernance() {
		spinner := NewSpinner("Evaluating governance policies...")
		spinner.Start()
		govResult, err = evaluateGovernance(ctx, app, rel)
		spinner.Stop()
		if err != nil {
			// In advisory mode, log warning but continue
			if !cfg.Governance.StrictMode {
				printWarning(fmt.Sprintf("Governance evaluation failed: %v", err))
			} else {
				return fmt.Errorf("governance evaluation failed: %w", err)
			}
		} else {
			// Capture risk + policy-decision analytics from the evaluation.
			captureGovernanceAnalytics(ctx, app, string(rel.ID()), govResult)

			// JSON mode: emit the canonical ApprovalCard so downstream
			// consumers (CI scripts, web dashboard, MCP relay) all see the
			// same wire shape regardless of which Relicta surface produced it.
			if outputJSON {
				card := governance.BuildApprovalCard(governance.ApprovalCardInput{
					Result:    govResult,
					ReleaseID: string(rel.ID()),
					Version:   approvalCardVersion(rel),
					Actor:     pkgcgp.Actor{Kind: string(rel.ActorType()), ID: rel.ActorID()},
				})
				if err := emitApprovalCardJSON(card); err != nil {
					return err
				}
			} else {
				displayGovernanceResult(govResult)
			}

			// Check if release is blocked in strict mode
			if cfg.Governance.StrictMode && govResult.Decision == cgp.DecisionRejected {
				printError("Release blocked by governance policy")
				for _, rationale := range govResult.Rationale {
					fmt.Printf("  - %s\n", rationale)
				}
				return fmt.Errorf("release denied by governance: %s", strings.Join(govResult.Rationale, "; "))
			}

			// Auto-approve if allowed and conditions met
			if govResult.CanAutoApprove && approveYes {
				if !outputJSON {
					printSuccess("Auto-approved by governance (low risk)")
				}
				var approveOut *releaseapp.ApproveReleaseOutput
				if !dryRun {
					out, err := executeApproval(ctx, app, rel, nil)
					if err != nil {
						return err
					}
					approveOut = out
				} else if !outputJSON {
					printWarning("Dry run - approval not saved")
				}
				if outputJSON {
					return outputApproveResultJSON(rel, approveOut)
				}
				printApproveNextSteps()
				return nil
			}

			// Governance said a human must look at this. Refuse to self-approve.
			//
			// Before this, displayAutoApprovalBlocked printed "Auto-approval not
			// available" and execution fell through to promptForApproval, which
			// returns true for --ci and --yes. So a release governance had marked
			// approval_required with can_auto_approve=false was approved anyway, by
			// automation, with no human involved — and the audit trail recorded it
			// as a normal approval attributed to whoever's git identity was
			// configured. The gate reported itself and did not gate, which is worse
			// than having no gate at all: a pipeline believes it is governed.
			//
			// Interactive runs still prompt, because a human being present is
			// exactly the condition governance is asking for. Only non-interactive
			// self-approval is refused.
			if !govResult.CanAutoApprove && (ciMode || approveYes) {
				if !approveOverride {
					if !outputJSON {
						displayAutoApprovalBlocked(govResult)
					}
					return errApprovalRequiresHuman(govResult)
				}

				// An override is a governance event in its own right, so it is
				// recorded with its reason rather than passed off as an approval.
				printWarning(fmt.Sprintf("Governance override: %s", approveOverrideReason))
				if !outputJSON {
					printSubtle("  Recorded in the audit trail as an override, not an approval.")
				}
			}
		}
	}

	// Edit release notes if requested
	editedNotes, err := handleNotesEditing(rel)
	if err != nil {
		return err
	}

	// Prompt for approval
	approved, err := promptForApproval()
	if err != nil {
		return err
	}
	if !approved {
		printWarning("Release not approved")
		return nil
	}

	// Dry run check
	if dryRun {
		if outputJSON {
			return outputApproveJSON(rel)
		}
		printWarning("Dry run - approval not saved")
		return nil
	}

	// Execute approval
	approveOut, err := executeApproval(ctx, app, rel, editedNotes)
	if err != nil {
		return err
	}

	// Capture the approval outcome for actor/bottleneck analytics.
	captureApprovalOutcome(ctx, app, string(rel.ID()), "approved", 0)

	if outputJSON {
		return outputApproveResultJSON(rel, approveOut)
	}
	printApproveNextSteps()
	return nil
}

// outputApproveResultJSON emits the post-approval JSON view. Fields the
// legacy file repository does not round-trip (version proposal, approval)
// are taken from the authoritative use-case output — re-reading the run
// through the legacy repo is what produced the reported "tag_name":
// "v0.0.0" (issue #136).
func outputApproveResultJSON(rel *release.ReleaseRun, out *releaseapp.ApproveReleaseOutput) error {
	payload := approveJSONPayload(rel)
	if out != nil {
		payload["approved"] = out.Approved
		payload["state"] = release.StateApproved.String()
		payload["approved_by"] = out.ApprovedBy
		if out.VersionNext != "" && out.VersionNext != "0.0.0" {
			payload["next_version"] = out.VersionNext
			payload["tag_name"] = cfg.Versioning.TagPrefix + out.VersionNext
		}
	}
	return encodeApproveJSON(payload)
}

// createCGPActor creates a CGP actor from the current environment.
func createCGPActor() cgp.Actor {
	// Determine actor kind based on environment
	kind := cgp.ActorKindHuman

	// Check for CI environment indicators
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" ||
		os.Getenv("GITLAB_CI") == "true" || os.Getenv("JENKINS_URL") != "" {
		kind = cgp.ActorKindCI
	}

	// Get actor identity
	identity := os.Getenv("USER")
	if identity == "" {
		identity = os.Getenv("USERNAME") // Windows
	}
	if identity == "" {
		identity = "unknown"
	}

	// Check for GitHub Actions specific identity
	if actor := os.Getenv("GITHUB_ACTOR"); actor != "" {
		identity = actor
	}

	id := fmt.Sprintf("%s:%s", kind.String(), identity)

	// Trust level. Trust is NEVER inferred from actor kind or the absence of CI
	// markers — both are environment signals an attacker can spoof to escalate.
	// (Previously a "human" actor — i.e. any invocation with no CI markers set —
	// was granted TrustLevelTrusted, so unsetting CI=true was enough to unlock
	// auto-approval.) Every actor now starts Limited (may propose, may NOT
	// auto-approve); elevation to Full requires explicit membership in the
	// operator-authored governance.trusted_actors allowlist.
	trustLevel := cgp.TrustLevelLimited
	if governance.IsActorTrusted(&cfg.Governance, cgp.Actor{ID: id, Name: identity}) {
		trustLevel = cgp.TrustLevelFull
	}

	return cgp.Actor{
		ID:         id,
		Kind:       kind,
		Name:       identity,
		TrustLevel: trustLevel,
	}
}

// evaluateGovernance evaluates the release through CGP governance.
func evaluateGovernance(ctx context.Context, app cliApp, rel *release.ReleaseRun) (*governance.EvaluateReleaseOutput, error) {
	govService := app.GovernanceService()
	if govService == nil {
		return nil, fmt.Errorf("governance service not available")
	}

	// Get repository info
	gitAdapter := app.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	actor := createCGPActor()

	input := governance.EvaluateReleaseInput{
		Release:        rel,
		Actor:          actor,
		Repository:     repoInfo.GovernanceID(),
		IncludeHistory: cfg.Governance.MemoryEnabled,
	}

	return govService.EvaluateRelease(ctx, input)
}

// displayGovernanceResult displays the governance evaluation result.
func displayGovernanceResult(result *governance.EvaluateReleaseOutput) {
	fmt.Println()
	printTitle("Governance Evaluation")
	fmt.Println()

	// Render the risk score using the shared lipgloss component so it has
	// proportional visual weight with the decision it drives (Von Restorff).
	// Falls back to plain text with severity glyphs when NO_COLOR or non-TTY.
	factors := make([]ui.RiskMeterFactor, 0, len(result.RiskFactors))
	for _, f := range result.RiskFactors {
		factors = append(factors, ui.RiskMeterFactor{
			Category:    f.Category,
			Description: f.Description,
			Score:       f.Score,
		})
	}
	fmt.Print(ui.RenderRisk(result.RiskScore, factors))

	fmt.Printf("  Severity:       %s\n", result.Severity)

	// Name the strongest factor when it is materially worse than the aggregate.
	//
	// The overall score is a weighted blend, so a breaking authentication change
	// showed "Risk 18.0% — LOW" while carrying a security_impact factor at 50%.
	// The arithmetic is fine and the reading is wrong: a person scanning that line
	// concludes the change is low risk. The aggregate drives the decision and stays
	// where it is; this says which factor is doing the work.
	if top, ok := dominantRiskFactor(result.RiskFactors, result.RiskScore); ok {
		fmt.Printf("  Driven by:      %s at %.0f%% (%s)\n",
			top.Category, top.Score*100, top.Description)
	}
	fmt.Printf("  Decision:       %s\n", result.Decision)
	fmt.Printf("  Auto-Approve:   %v\n", result.CanAutoApprove)

	// Display required actions if any
	if len(result.RequiredActions) > 0 {
		fmt.Println()
		fmt.Println("  Required Actions:")
		for _, action := range result.RequiredActions {
			fmt.Printf("    - [%s] %s\n", action.Type, action.Description)
		}
	}

	// Display rationale if decision is not approved
	if result.Decision != cgp.DecisionApproved && len(result.Rationale) > 0 {
		fmt.Println()
		fmt.Println("  Rationale:")
		for _, r := range result.Rationale {
			fmt.Printf("    - %s\n", r)
		}
	}

	// Display historical context if available
	if result.HistoricalContext != nil && result.HistoricalContext.RecentReleases > 0 {
		fmt.Println()
		fmt.Println("  Historical Context:")
		fmt.Printf("    Recent Releases: %d\n", result.HistoricalContext.RecentReleases)
		fmt.Printf("    Success Rate:    %.1f%%\n", result.HistoricalContext.SuccessRate*100)
		if result.HistoricalContext.RollbackRate > 0 {
			fmt.Printf("    Rollback Rate:   %.1f%%\n", result.HistoricalContext.RollbackRate*100)
		}
	}
}

// displayAutoApprovalBlocked explains why auto-approval isn't available.
// This helps users understand what conditions prevent auto-approval.
func displayAutoApprovalBlocked(result *governance.EvaluateReleaseOutput) {
	fmt.Println()
	printInfo("Auto-approval not available:")
	fmt.Println()

	reasons := []string{}

	// Check risk score against thresholds
	if result.RiskScore > 0.3 {
		reasons = append(reasons, fmt.Sprintf("Risk score %.0f%% exceeds auto-approve threshold (30%%)", result.RiskScore*100))
	}

	// Check decision
	if result.Decision == cgp.DecisionApprovalRequired {
		reasons = append(reasons, "Governance requires manual review")
	}

	// Check for blocking risk factors
	for _, factor := range result.RiskFactors {
		if factor.Category == "security" {
			reasons = append(reasons, "Security-related changes detected")
			break
		}
	}
	for _, factor := range result.RiskFactors {
		if factor.Category == "breaking" {
			reasons = append(reasons, "Breaking changes detected")
			break
		}
	}

	// Check required actions
	for _, action := range result.RequiredActions {
		reasons = append(reasons, fmt.Sprintf("Required: %s", action.Description))
	}

	// If we couldn't determine specific reasons, give a generic message
	if len(reasons) == 0 {
		reasons = append(reasons, "Governance policy requires manual approval for this release")
	}

	// Display the reasons
	for _, reason := range reasons {
		fmt.Printf("  • %s\n", reason)
	}

	fmt.Println()
	printInfo("Manual approval required. Review the changes above and confirm.")
}

// recordReleaseOutcome records the release outcome to Release Memory.
// Approval deliberately records no release outcome.
//
// It used to record one, marked successful, at the moment of approval — and
// `relicta publish` records another when the release actually happens. With the
// repository identity made canonical both landed under the same key, and one
// release appeared in `relicta history` twice: "Summary: 2 releases, 100% success
// rate" for a single publish.
//
// The duplicate is the visible half. The important half is that an approval is not
// a release: a change approved and then never published, or one whose publish
// failed, was recorded as a successful release. Calibration compares predicted risk
// against actual outcomes and reputation is built from them, so both were being fed
// an event that had not happened yet, twice.
//
// The approval itself is not lost. It is recorded on the run — approver, actor type
// and justification (#240) — and captured for analytics by captureApprovalOutcome,
// which is where approval-level events belong.

// displayReleaseSummary displays the release summary for review.
func displayReleaseSummary(rel *release.ReleaseRun) {
	fmt.Println()
	printTitle("Release Summary")
	fmt.Println()

	summary := rel.Summary()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  Release ID:\t%s\n", summary.ID)
	fmt.Fprintf(w, "  Current version:\t%s\n", summary.VersionCurrent)
	fmt.Fprintf(w, "  Next version:\t%s\n", summary.VersionNext)
	fmt.Fprintf(w, "  Bump type:\t%s\n", summary.BumpKind)
	fmt.Fprintf(w, "  Total commits:\t%d\n", summary.CommitCount)
	fmt.Fprintf(w, "  Branch:\t%s\n", rel.Branch())
	fmt.Fprintf(w, "  State:\t%s\n", summary.State.String())
	_ = w.Flush() // Ignore flush error for stdout display

	// Show changes overview
	if plan := release.GetPlan(rel); plan != nil && plan.HasChangeSet() {
		fmt.Println()
		printTitle("Changes Overview")
		fmt.Println()

		changeSet := plan.GetChangeSet()
		cats := changeSet.Categories()

		if len(cats.Breaking) > 0 {
			fmt.Printf("  Breaking changes: %d\n", len(cats.Breaking))
		}
		if len(cats.Features) > 0 {
			fmt.Printf("  Features:         %d\n", len(cats.Features))
		}
		if len(cats.Fixes) > 0 {
			fmt.Printf("  Bug fixes:        %d\n", len(cats.Fixes))
		}
		if len(cats.Perf) > 0 {
			fmt.Printf("  Performance:      %d\n", len(cats.Perf))
		}
		if len(cats.Other) > 0 {
			fmt.Printf("  Other:            %d\n", len(cats.Other))
		}
	}

	// Show release notes preview
	if rel.Notes() != nil && rel.Notes().Text != "" {
		fmt.Println()
		printTitle("Release Notes Preview")
		fmt.Println()

		lines := strings.Split(rel.Notes().Text, "\n")
		previewLines := maxNotesPreviewLines
		if len(lines) < previewLines {
			previewLines = len(lines)
		}
		for i := 0; i < previewLines; i++ {
			fmt.Printf("  %s\n", lines[i])
		}
		if len(lines) > previewLines {
			fmt.Printf("  ... (%d more lines)\n", len(lines)-previewLines)
		}
	}

	// Show configured plugins
	if len(cfg.Plugins) > 0 {
		fmt.Println()
		printTitle("Plugins to Execute")
		fmt.Println()
		for _, p := range cfg.Plugins {
			if p.IsEnabled() {
				fmt.Printf("  - %s\n", p.Name)
			}
		}
	}
}

// allowedEditors is a whitelist of safe editors to prevent command injection.
var allowedEditors = map[string]bool{
	"vim":    true,
	"nvim":   true,
	"nano":   true,
	"emacs":  true,
	"vi":     true,
	"code":   true,
	"subl":   true,
	"gedit":  true,
	"kate":   true,
	"micro":  true,
	"helix":  true,
	"hx":     true,
	"pico":   true,
	"joe":    true,
	"ne":     true,
	"mcedit": true,
}

// validateEditor checks if the editor is in the allowed list and resolves its path safely.
// Security: Validates both the requested name AND the resolved binary name to prevent
// symlink attacks where ~/bin/vim could point to a malicious binary.
func validateEditor(editor string) (string, error) {
	// Extract just the binary name (handle paths like /usr/bin/vim)
	baseName := filepath.Base(editor)

	// Check against whitelist
	if !allowedEditors[baseName] {
		return "", fmt.Errorf("editor %q is not in the allowed list; allowed editors: vim, nvim, nano, emacs, vi, code, subl, gedit, kate, micro, helix, pico", baseName)
	}

	// Use LookPath to safely resolve the editor binary
	resolvedPath, err := exec.LookPath(baseName)
	if err != nil {
		return "", fmt.Errorf("editor %q not found in PATH: %w", baseName, err)
	}

	// Security: Verify the resolved binary name also matches an allowed editor
	// This prevents symlink attacks where ~/bin/vim -> /path/to/malicious
	resolvedBase := filepath.Base(resolvedPath)
	if !allowedEditors[resolvedBase] {
		return "", fmt.Errorf("editor resolved to unexpected binary %q (expected %q)", resolvedPath, baseName)
	}

	return resolvedPath, nil
}

// editReleaseNotes opens an editor for editing release notes.
func editReleaseNotes(notes string) (string, error) {
	// Determine editor
	editor := approveEditor
	if editor == "" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim"
		}
	}

	// Validate and resolve editor path securely
	resolvedEditor, err := validateEditor(editor)
	if err != nil {
		return "", fmt.Errorf("invalid editor: %w", err)
	}

	// Create temp file with restrictive permissions
	tmpfile, err := os.CreateTemp("", "release-notes-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpfile.Name()
	defer os.Remove(tmpPath)

	// Set restrictive permissions explicitly
	if err := os.Chmod(tmpPath, filePermPrivate); err != nil {
		_ = tmpfile.Close()
		return "", fmt.Errorf("failed to set temp file permissions: %w", err)
	}

	// Write current notes to temp file
	if _, err := tmpfile.WriteString(notes); err != nil {
		_ = tmpfile.Close()
		return "", fmt.Errorf("failed to write to temp file: %w", err)
	}
	if err := tmpfile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	// Open editor with resolved safe path
	cmd := exec.Command(resolvedEditor, tmpPath) // #nosec G204 -- editor path validated above
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor failed: %w", err)
	}

	// Read edited content
	content, err := os.ReadFile(tmpPath) // #nosec G304 -- temp file we just created
	if err != nil {
		return "", fmt.Errorf("failed to read edited file: %w", err)
	}

	return string(content), nil
}

// outputApproveJSON outputs the approval information as JSON.
func outputApproveJSON(rel *release.ReleaseRun) error {
	return encodeApproveJSON(approveJSONPayload(rel))
}

// approveJSONPayload builds the machine-readable approve view of a release.
func approveJSONPayload(rel *release.ReleaseRun) map[string]any {
	summary := rel.Summary()

	output := map[string]any{
		"release_id":      string(summary.ID),
		"current_version": summary.VersionCurrent,
		"next_version":    summary.VersionNext,
		"bump_kind":       string(summary.BumpKind),
		"commit_count":    summary.CommitCount,
		"branch":          rel.Branch(),
		"approved":        rel.Approval() != nil,
		"state":           summary.State.String(),
		"ci_mode":         ciMode,
	}

	if summary.VersionNext != "" {
		output["tag_name"] = cfg.Versioning.TagPrefix + summary.VersionNext
	}

	// Add changes summary if available
	if plan := release.GetPlan(rel); plan != nil && plan.HasChangeSet() {
		changeSet := plan.GetChangeSet()
		cats := changeSet.Categories()
		output["changes_summary"] = map[string]int{
			"breaking":    len(cats.Breaking),
			"features":    len(cats.Features),
			"fixes":       len(cats.Fixes),
			"performance": len(cats.Perf),
			"other":       len(cats.Other),
		}
	}

	return output
}

// encodeApproveJSON writes the approve JSON document to stdout.
func encodeApproveJSON(output map[string]any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// buildTUISummary builds the TUI summary from a release.
func buildTUISummary(rel *release.ReleaseRun) ui.ReleaseSummary {
	summary := rel.Summary()

	tuiSummary := ui.ReleaseSummary{
		ReleaseID:      string(summary.ID),
		CurrentVersion: summary.VersionCurrent,
		NextVersion:    summary.VersionNext,
		ReleaseType:    string(summary.BumpKind),
		CommitCount:    summary.CommitCount,
		Branch:         rel.Branch(),
	}

	// Add changes info
	if plan := release.GetPlan(rel); plan != nil && plan.HasChangeSet() {
		changeSet := plan.GetChangeSet()
		cats := changeSet.Categories()
		tuiSummary.BreakingCount = len(cats.Breaking)
		tuiSummary.FeatureCount = len(cats.Features)
		tuiSummary.FixCount = len(cats.Fixes)
		tuiSummary.PerfCount = len(cats.Perf)
		tuiSummary.OtherCount = len(cats.Other) + len(cats.Docs) + len(cats.Refactors) + len(cats.Tests) + len(cats.Chores) + len(cats.Build) + len(cats.CI)
	}

	// Add release notes
	if rel.Notes() != nil {
		tuiSummary.ReleaseNotes = rel.Notes().Text
	}

	// Add plugins
	for _, p := range cfg.Plugins {
		if p.IsEnabled() {
			tuiSummary.Plugins = append(tuiSummary.Plugins, p.Name)
		}
	}

	return tuiSummary
}

// buildGovernanceSummaryForTUI builds governance summary for TUI display.
func buildGovernanceSummaryForTUI(ctx context.Context, app cliApp, rel *release.ReleaseRun) *ui.GovernanceSummary {
	govService := app.GovernanceService()
	if govService == nil {
		return nil
	}

	// The same canonical identity the rest of governance uses. This read the raw
	// remote URL, so the TUI summary evaluated against a different repository key
	// than `approve` recorded under — one of three identities in use for the same
	// repository.
	repoID := ""
	if gitAdapter := app.GitAdapter(); gitAdapter != nil {
		if info, err := gitAdapter.GetInfo(ctx); err == nil {
			repoID = info.GovernanceID()
		}
	}

	// Create actor
	actor := createCGPActor()

	// Evaluate
	input := governance.EvaluateReleaseInput{
		Release:    rel,
		Actor:      actor,
		Repository: repoID,
	}

	result, err := govService.EvaluateRelease(ctx, input)
	if err != nil {
		return nil
	}

	// Build TUI summary
	govSummary := &ui.GovernanceSummary{
		RiskScore:      result.RiskScore,
		Severity:       string(result.Severity),
		Decision:       string(result.Decision),
		CanAutoApprove: result.CanAutoApprove,
	}

	// Add risk factors
	for _, factor := range result.RiskFactors {
		govSummary.RiskFactors = append(govSummary.RiskFactors, ui.RiskFactor{
			Category:    factor.Category,
			Description: factor.Description,
			Score:       factor.Score,
		})
	}

	// Add required actions
	for _, action := range result.RequiredActions {
		govSummary.RequiredActions = append(govSummary.RequiredActions, action.Description)
	}

	return govSummary
}

// handleEditApprovalResult handles the edit result from TUI approval.
// Returns the edited notes and whether approval should proceed.
func handleEditApprovalResult(rel *release.ReleaseRun) (*string, bool, error) {
	if rel.Notes() == nil {
		printWarning("No release notes to edit")
		return nil, false, nil
	}

	notes, err := editReleaseNotes(rel.Notes().Text)
	if err != nil {
		return nil, false, fmt.Errorf("failed to edit release notes: %w", err)
	}
	printInfo("Notes edited - changes will be applied during approval")

	// After editing, prompt again
	fmt.Println()
	fmt.Print("Do you approve this release after editing? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return nil, false, fmt.Errorf("failed to read input: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		return nil, false, nil
	}

	return &notes, true, nil
}

// processTUIApprovalResult processes the TUI approval result and returns edited notes and whether to proceed.
func processTUIApprovalResult(result ui.ApprovalResult, rel *release.ReleaseRun) (*string, bool, error) {
	switch result {
	case ui.ApprovalAccepted:
		return nil, true, nil
	case ui.ApprovalRejected:
		printWarning("Release not approved")
		return nil, false, nil
	case ui.ApprovalEdit:
		return handleEditApprovalResult(rel)
	default:
		printWarning("Release not approved")
		return nil, false, nil
	}
}

// runInteractiveApproval runs the interactive TUI for approval.
func runInteractiveApproval(ctx context.Context, app cliApp, rel *release.ReleaseRun) error {
	// Build TUI summary
	tuiSummary := buildTUISummary(rel)

	// Add governance info if enabled
	if app.HasGovernance() {
		govSummary := buildGovernanceSummaryForTUI(ctx, app, rel)
		if govSummary != nil {
			tuiSummary.Governance = govSummary
		}
	}

	// Run TUI
	result, err := runApprovalTUI(tuiSummary)
	if err != nil {
		return fmt.Errorf("interactive approval failed: %w", err)
	}

	// Process TUI result
	editedNotes, proceed, err := processTUIApprovalResult(result, rel)
	if err != nil {
		return err
	}
	if !proceed {
		return nil
	}

	// Dry run check
	if dryRun {
		printWarning("Dry run - approval not saved")
		return nil
	}

	// Execute approval (reuse the common helper)
	if _, err := executeApproval(ctx, app, rel, editedNotes); err != nil {
		return err
	}

	printApproveNextSteps()
	return nil
}

// errApprovalRequiresHuman explains that automation may not approve this release
// and what to do about it.
//
// The message lists governance's own reasons rather than a generic refusal,
// because the operator's next step depends on which of them applies: a risk score
// over the threshold is a different conversation from a breaking change needing
// review.
func errApprovalRequiresHuman(result *governance.EvaluateReleaseOutput) error {
	var b strings.Builder
	b.WriteString("governance requires human approval for this release\n")

	if len(result.RequiredActions) > 0 {
		b.WriteString("\nRequired before release:\n")
		for _, action := range result.RequiredActions {
			fmt.Fprintf(&b, "  - %s\n", action.Description)
		}
	}

	b.WriteString("\nEither approve it interactively:\n")
	b.WriteString("  relicta approve\n")
	b.WriteString("\nor override deliberately, which is recorded in the audit trail:\n")
	b.WriteString("  relicta approve --override-governance --reason \"...\"\n")

	return errors.New(b.String())
}

// approvalJustification returns the text recorded with the approval.
//
// Only an override produces one today. It is prefixed so a reader of the audit
// trail can tell a bypass from a normal note, without having to know which flag
// was passed.
func approvalJustification() string {
	if approveOverride && strings.TrimSpace(approveOverrideReason) != "" {
		return "governance override: " + strings.TrimSpace(approveOverrideReason)
	}
	return ""
}

// approverActorType reports whether a human or automation is approving.
//
// It was hardcoded to "user", so an approval made by a pipeline was recorded as a
// human decision attributed to whatever $USER the runner happened to use. For a
// tool whose product is the audit trail that is a falsehood in the record: asked
// later who approved a release, it would name a person who was not involved.
func approverActorType() domain.ActorType {
	if ciMode {
		return domain.ActorCI
	}
	return domain.ActorHuman
}

// dominantRiskFactorThreshold is how far above the aggregate a single factor has
// to sit before it is called out. Set so a factor merely in line with the overall
// score stays quiet, and one that the blend has flattened does not.
const dominantRiskFactorThreshold = 0.2

// dominantRiskFactor returns the highest-scoring factor when it exceeds the
// aggregate by enough to be worth naming, and reports whether there is one.
func dominantRiskFactor(factors []cgp.RiskFactor, aggregate float64) (cgp.RiskFactor, bool) {
	var top cgp.RiskFactor
	for _, f := range factors {
		if f.Score > top.Score {
			top = f
		}
	}
	if top.Score-aggregate < dominantRiskFactorThreshold {
		return cgp.RiskFactor{}, false
	}
	return top, true
}
