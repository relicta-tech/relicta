// Package cli provides the command-line interface for Relicta.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/application/governance"
	apprelease "github.com/relicta-tech/relicta/internal/application/release"
	"github.com/relicta-tech/relicta/internal/cgp"
	"github.com/relicta-tech/relicta/internal/container"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/ui"
)

var (
	approveYes         bool
	approveEdit        bool
	approveEditor      string
	approveInteractive bool
)

func init() {
	approveCmd.Flags().BoolVarP(&approveYes, "yes", "y", false, "automatically approve without prompting")
	approveCmd.Flags().BoolVarP(&approveEdit, "edit", "e", false, "edit release notes before approving")
	approveCmd.Flags().StringVar(&approveEditor, "editor", "", "editor to use (default: $EDITOR or vim)")
	approveCmd.Flags().BoolVarP(&approveInteractive, "interactive", "i", false, "use interactive TUI for approval")
}

// getLatestRelease retrieves the latest release from the repository.
func getLatestRelease(ctx context.Context, dddContainer *container.DDDContainer) (*release.Release, error) {
	gitAdapter := dddContainer.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	releaseRepo := dddContainer.ReleaseRepository()
	rel, err := releaseRepo.FindLatest(ctx, repoInfo.Path)
	if err != nil {
		printError("No release in progress")
		printInfo("Run 'relicta plan' to start a new release")
		return nil, fmt.Errorf("no release state found")
	}

	return rel, nil
}

// isReleaseAlreadyApproved checks if the release is already approved and prints info.
func isReleaseAlreadyApproved(rel *release.Release) bool {
	if rel.State() == release.StateApproved || rel.IsApproved() {
		printInfo("Release already approved")
		printInfo("Run 'relicta publish' to execute the release")
		return true
	}
	return false
}

// shouldUseInteractiveApproval returns true if interactive TUI should be used.
func shouldUseInteractiveApproval() bool {
	return approveInteractive && !ciMode && !approveYes
}

// handleNotesEditing handles editing of release notes if requested.
func handleNotesEditing(rel *release.Release) (*string, error) {
	if !approveEdit || rel.Notes() == nil {
		return nil, nil
	}

	notes, err := editReleaseNotes(rel.Notes().Changelog)
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

// executeApproval executes the approval use case.
func executeApproval(ctx context.Context, dddContainer *container.DDDContainer, rel *release.Release, editedNotes *string) error {
	input := apprelease.ApproveReleaseInput{
		ReleaseID:   rel.ID(),
		ApprovedBy:  getApproverName(),
		AutoApprove: approveYes,
		EditedNotes: editedNotes,
	}

	_, err := dddContainer.ApproveRelease().Execute(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to approve release: %w", err)
	}
	return nil
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

	printTitle("Release Approval")
	fmt.Println()

	// Initialize container
	dddContainer, err := container.NewInitializedDDDContainer(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer dddContainer.Close()

	// Get latest release
	rel, err := getLatestRelease(ctx, dddContainer)
	if err != nil {
		return err
	}

	// Check if already approved
	if isReleaseAlreadyApproved(rel) {
		return nil
	}

	// Output JSON if requested
	if outputJSON {
		return outputApproveJSON(rel)
	}

	// Use interactive TUI if requested
	if shouldUseInteractiveApproval() {
		return runInteractiveApproval(ctx, dddContainer, rel)
	}

	// Display release summary
	displayReleaseSummary(rel)

	// CGP Governance evaluation (if enabled)
	var govResult *governance.EvaluateReleaseOutput
	if dddContainer.HasGovernance() {
		govResult, err = evaluateGovernance(ctx, dddContainer, rel)
		if err != nil {
			// In advisory mode, log warning but continue
			if !cfg.Governance.StrictMode {
				printWarning(fmt.Sprintf("Governance evaluation failed: %v", err))
			} else {
				return fmt.Errorf("governance evaluation failed: %w", err)
			}
		} else {
			// Display governance results
			displayGovernanceResult(govResult)

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
				printSuccess("Auto-approved by governance (low risk)")
				if !dryRun {
					if err := executeApproval(ctx, dddContainer, rel, nil); err != nil {
						return err
					}
					// Record successful outcome
					recordReleaseOutcome(ctx, dddContainer, rel, govResult, true)
				} else {
					printWarning("Dry run - approval not saved")
				}
				printApproveNextSteps()
				return nil
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
		printWarning("Dry run - approval not saved")
		return nil
	}

	// Execute approval
	if err := executeApproval(ctx, dddContainer, rel, editedNotes); err != nil {
		return err
	}

	// Record successful outcome to Release Memory
	if govResult != nil {
		recordReleaseOutcome(ctx, dddContainer, rel, govResult, true)
	}

	printApproveNextSteps()
	return nil
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

	// Determine trust level
	trustLevel := cgp.TrustLevelLimited
	if kind == cgp.ActorKindHuman {
		trustLevel = cgp.TrustLevelTrusted
	}

	// Check if actor is in trusted list
	if governance.IsActorTrusted(&cfg.Governance, cgp.Actor{ID: identity}) {
		trustLevel = cgp.TrustLevelFull
	}

	return cgp.Actor{
		ID:         fmt.Sprintf("%s:%s", kind.String(), identity),
		Kind:       kind,
		Name:       identity,
		TrustLevel: trustLevel,
	}
}

// evaluateGovernance evaluates the release through CGP governance.
func evaluateGovernance(ctx context.Context, dddContainer *container.DDDContainer, rel *release.Release) (*governance.EvaluateReleaseOutput, error) {
	govService := dddContainer.GovernanceService()
	if govService == nil {
		return nil, fmt.Errorf("governance service not available")
	}

	// Get repository info
	gitAdapter := dddContainer.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	actor := createCGPActor()

	input := governance.EvaluateReleaseInput{
		Release:        rel,
		Actor:          actor,
		Repository:     repoInfo.Path,
		IncludeHistory: cfg.Governance.MemoryEnabled,
	}

	return govService.EvaluateRelease(ctx, input)
}

// displayGovernanceResult displays the governance evaluation result.
func displayGovernanceResult(result *governance.EvaluateReleaseOutput) {
	fmt.Println()
	printTitle("Governance Evaluation")
	fmt.Println()

	// Risk score with color indicator
	riskPct := result.RiskScore * 100
	riskLabel := "LOW"
	if result.RiskScore >= 0.7 {
		riskLabel = "HIGH"
	} else if result.RiskScore >= 0.4 {
		riskLabel = "MEDIUM"
	}

	fmt.Printf("  Risk Score:     %.1f%% (%s)\n", riskPct, riskLabel)
	fmt.Printf("  Severity:       %s\n", result.Severity)
	fmt.Printf("  Decision:       %s\n", result.Decision)
	fmt.Printf("  Auto-Approve:   %v\n", result.CanAutoApprove)

	// Display risk factors if any
	if len(result.RiskFactors) > 0 {
		fmt.Println()
		fmt.Println("  Risk Factors:")
		for _, factor := range result.RiskFactors {
			fmt.Printf("    - [%s] %s (%.1f%%)\n", factor.Category, factor.Description, factor.Score*100)
		}
	}

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

// recordReleaseOutcome records the release outcome to Release Memory.
func recordReleaseOutcome(ctx context.Context, dddContainer *container.DDDContainer, rel *release.Release, govResult *governance.EvaluateReleaseOutput, success bool) {
	govService := dddContainer.GovernanceService()
	if govService == nil || govResult == nil {
		return
	}

	// Get repository info
	gitAdapter := dddContainer.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return
	}

	outcome := governance.OutcomeSuccess
	if !success {
		outcome = governance.OutcomeFailure
	}

	actor := createCGPActor()

	input := governance.RecordOutcomeInput{
		ReleaseID:  rel.ID(),
		Repository: repoInfo.Path,
		Version:    rel.Summary().NextVersion,
		Actor:      actor,
		RiskScore:  govResult.RiskScore,
		Decision:   govResult.Decision,
		Outcome:    outcome,
	}

	if err := govService.RecordReleaseOutcome(ctx, input); err != nil {
		// Non-fatal - just log the warning
		printWarning(fmt.Sprintf("Failed to record release outcome: %v", err))
	}
}

// displayReleaseSummary displays the release summary for review.
func displayReleaseSummary(rel *release.Release) {
	fmt.Println()
	printTitle("Release Summary")
	fmt.Println()

	summary := rel.Summary()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  Release ID:\t%s\n", summary.ID)
	fmt.Fprintf(w, "  Current version:\t%s\n", summary.CurrentVersion)
	fmt.Fprintf(w, "  Next version:\t%s\n", summary.NextVersion)
	fmt.Fprintf(w, "  Release type:\t%s\n", summary.ReleaseType)
	fmt.Fprintf(w, "  Total commits:\t%d\n", summary.CommitCount)
	fmt.Fprintf(w, "  Branch:\t%s\n", summary.Branch)
	fmt.Fprintf(w, "  State:\t%s\n", summary.State.String())
	_ = w.Flush() // Ignore flush error for stdout display

	// Show changes overview
	if rel.Plan() != nil && rel.Plan().HasChangeSet() {
		fmt.Println()
		printTitle("Changes Overview")
		fmt.Println()

		changeSet := rel.Plan().GetChangeSet()
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
	if rel.Notes() != nil && rel.Notes().Changelog != "" {
		fmt.Println()
		printTitle("Release Notes Preview")
		fmt.Println()

		// Show first 20 lines
		lines := strings.Split(rel.Notes().Changelog, "\n")
		maxLines := 20
		if len(lines) < maxLines {
			maxLines = len(lines)
		}
		for i := 0; i < maxLines; i++ {
			fmt.Printf("  %s\n", lines[i])
		}
		if len(lines) > maxLines {
			fmt.Printf("  ... (%d more lines)\n", len(lines)-maxLines)
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
func validateEditor(editor string) (string, error) {
	// Extract just the binary name (handle paths like /usr/bin/vim)
	baseName := filepath.Base(editor)

	// Check against whitelist
	if !allowedEditors[baseName] {
		return "", fmt.Errorf("editor %q is not in the allowed list. Allowed editors: vim, nvim, nano, emacs, vi, code, subl, gedit, kate, micro, helix, pico", baseName)
	}

	// Use LookPath to safely resolve the editor binary
	resolvedPath, err := exec.LookPath(baseName)
	if err != nil {
		return "", fmt.Errorf("editor %q not found in PATH: %w", baseName, err)
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

	// Create temp file with restrictive permissions (0600 = owner read/write only)
	tmpfile, err := os.CreateTemp("", "release-notes-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpfile.Name()
	defer os.Remove(tmpPath)

	// Set restrictive permissions explicitly
	if err := os.Chmod(tmpPath, 0600); err != nil {
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
	cmd := exec.Command(resolvedEditor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor failed: %w", err)
	}

	// Read edited content
	content, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read edited file: %w", err)
	}

	return string(content), nil
}

// outputApproveJSON outputs the approval information as JSON.
func outputApproveJSON(rel *release.Release) error {
	summary := rel.Summary()

	output := map[string]any{
		"release_id":      string(summary.ID),
		"current_version": summary.CurrentVersion,
		"next_version":    summary.NextVersion,
		"release_type":    summary.ReleaseType,
		"commit_count":    summary.CommitCount,
		"branch":          summary.Branch,
		"approved":        summary.IsApproved,
		"state":           summary.State.String(),
		"ci_mode":         ciMode,
	}

	if summary.NextVersion != "" {
		output["tag_name"] = cfg.Versioning.TagPrefix + summary.NextVersion
	}

	// Add changes summary if available
	if rel.Plan() != nil && rel.Plan().HasChangeSet() {
		changeSet := rel.Plan().GetChangeSet()
		cats := changeSet.Categories()
		output["changes_summary"] = map[string]int{
			"breaking":    len(cats.Breaking),
			"features":    len(cats.Features),
			"fixes":       len(cats.Fixes),
			"performance": len(cats.Perf),
			"other":       len(cats.Other),
		}
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// buildTUISummary builds the TUI summary from a release.
func buildTUISummary(rel *release.Release) ui.ReleaseSummary {
	summary := rel.Summary()

	tuiSummary := ui.ReleaseSummary{
		ReleaseID:      string(summary.ID),
		CurrentVersion: summary.CurrentVersion,
		NextVersion:    summary.NextVersion,
		ReleaseType:    summary.ReleaseType,
		CommitCount:    summary.CommitCount,
		Branch:         summary.Branch,
	}

	// Add changes info
	if rel.Plan() != nil && rel.Plan().HasChangeSet() {
		changeSet := rel.Plan().GetChangeSet()
		cats := changeSet.Categories()
		tuiSummary.BreakingCount = len(cats.Breaking)
		tuiSummary.FeatureCount = len(cats.Features)
		tuiSummary.FixCount = len(cats.Fixes)
		tuiSummary.PerfCount = len(cats.Perf)
		tuiSummary.OtherCount = len(cats.Other) + len(cats.Docs) + len(cats.Refactors) + len(cats.Tests) + len(cats.Chores) + len(cats.Build) + len(cats.CI)
	}

	// Add release notes
	if rel.Notes() != nil {
		tuiSummary.ReleaseNotes = rel.Notes().Changelog
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
func buildGovernanceSummaryForTUI(ctx context.Context, dddContainer *container.DDDContainer, rel *release.Release) *ui.GovernanceSummary {
	govService := dddContainer.GovernanceService()
	if govService == nil {
		return nil
	}

	// Get repo info
	repoURL := ""
	if gitAdapter := dddContainer.GitAdapter(); gitAdapter != nil {
		if info, err := gitAdapter.GetInfo(ctx); err == nil {
			repoURL = info.RemoteURL
		}
	}

	// Create actor
	actor := createCGPActor()

	// Evaluate
	input := governance.EvaluateReleaseInput{
		Release:    rel,
		Actor:      actor,
		Repository: repoURL,
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
func handleEditApprovalResult(rel *release.Release) (*string, bool, error) {
	if rel.Notes() == nil {
		printWarning("No release notes to edit")
		return nil, false, nil
	}

	notes, err := editReleaseNotes(rel.Notes().Changelog)
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
func processTUIApprovalResult(result ui.ApprovalResult, rel *release.Release) (*string, bool, error) {
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
func runInteractiveApproval(ctx context.Context, dddContainer *container.DDDContainer, rel *release.Release) error {
	// Build TUI summary
	tuiSummary := buildTUISummary(rel)

	// Add governance info if enabled
	if dddContainer.HasGovernance() {
		govSummary := buildGovernanceSummaryForTUI(ctx, dddContainer, rel)
		if govSummary != nil {
			tuiSummary.Governance = govSummary
		}
	}

	// Run TUI
	result, err := ui.RunApprovalTUI(tuiSummary)
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
	if err := executeApproval(ctx, dddContainer, rel, editedNotes); err != nil {
		return err
	}

	printApproveNextSteps()
	return nil
}
