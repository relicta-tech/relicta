// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/internal/application/versioning"
	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/communication"
	releaseapp "github.com/relicta-tech/relicta/internal/domain/release/app"
	releasedomain "github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/domain/release/ports"
	"github.com/relicta-tech/relicta/internal/domain/version"
	servicerelease "github.com/relicta-tech/relicta/internal/service/release"
)

var (
	releaseAutoApprove bool
	releaseSkipPush    bool
	releaseForce       string
)

// releaseMode represents the detected release mode.
type releaseMode int

const (
	// releaseModeNew is a normal release with new commits.
	releaseModeNew releaseMode = iota
	// releaseModeTagPush is triggered when HEAD is already tagged (e.g., tag push in CI).
	releaseModeTagPush
)

// releaseWorkflowSteps is the total number of steps in the release workflow.
const releaseWorkflowSteps = 5

// releaseWorkflowContext holds shared state for the release workflow.
// This ensures mode detection happens once and state is shared between steps.
type releaseWorkflowContext struct {
	mode            releaseMode
	existingVersion *version.SemanticVersion
	prevTagName     string // Previous version tag for commit range
}

var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Run the complete release workflow",
	Long: `Run the complete release workflow in one command.

This is equivalent to running:
  relicta plan → bump → notes → approve → publish

By default, the command runs interactively and prompts for approval.
Use --yes to auto-approve for CI/CD pipelines.

Examples:
  # Interactive release (prompts for approval)
  relicta release

  # Auto-approve for CI/CD
  relicta release --yes

  # Dry run to preview changes
  relicta release --dry-run

  # Force a specific version
  relicta release --force v2.0.0`,
	RunE: runRelease,
}

func init() {
	releaseCmd.Flags().BoolVarP(&releaseAutoApprove, "yes", "y", false, "auto-approve the release without prompting")
	releaseCmd.Flags().BoolVar(&releaseSkipPush, "skip-push", false, "skip pushing to remote")
	releaseCmd.Flags().StringVar(&releaseForce, "force", "", "force a specific version (e.g., v2.0.0)")
}

// runRelease implements the release command - full workflow in one step.
// The helper functions (runReleasePlan, runReleaseBump) detect and handle
// tag-push mode internally, so no explicit mode detection is needed here.
func runRelease(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	printTitle("Relicta Release")
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

	// Run the release workflow - helpers detect tag-push mode internally
	return runReleaseWorkflow(ctx, app)
}

// runReleaseWorkflow executes the 5-step release workflow using helper functions.
// Mode detection happens once at the start and is shared via releaseWorkflowContext.
func runReleaseWorkflow(ctx context.Context, app cliApp) error {
	// Detect release mode once at the start
	wfCtx, err := detectWorkflowContext(ctx, app)
	if err != nil {
		return fmt.Errorf("failed to detect release mode: %w", err)
	}

	// Step 1: Plan
	printStep(1, releaseWorkflowSteps, "Planning release")
	planOutput, err := runReleasePlan(ctx, app, wfCtx)
	if err != nil {
		return fmt.Errorf("plan failed: %w", err)
	}

	// Check if there are any changes
	commitCount := 0
	if planOutput.ChangeSet != nil {
		commitCount = planOutput.ChangeSet.CommitCount()
	}
	if commitCount == 0 {
		printInfo("No changes since last release")
		return nil
	}

	// Show plan summary - use effective bump type for display when ReleaseType is "none"
	bumpTypeDisplay := planOutput.ReleaseType.String()
	if bumpTypeDisplay == "none" {
		bumpTypeDisplay = effectiveBumpType(planOutput.CurrentVersion, planOutput.NextVersion)
	}
	fmt.Printf("  Version: %s → %s (%s)\n",
		planOutput.CurrentVersion.String(),
		planOutput.NextVersion.String(),
		bumpTypeDisplay)
	fmt.Printf("  Commits: %d\n", commitCount)
	fmt.Println()

	// Step 2: Bump version
	printStep(2, releaseWorkflowSteps, "Bumping version")
	bumpOutput, err := runReleaseBump(ctx, app, wfCtx, planOutput)
	if err != nil {
		return fmt.Errorf("bump failed: %w", err)
	}
	if bumpOutput.TagCreated {
		fmt.Printf("  Created tag: %s\n", bumpOutput.TagName)
	} else {
		fmt.Printf("  Using tag: %s\n", bumpOutput.TagName)
	}
	fmt.Println()

	// Step 3: Generate notes
	printStep(3, releaseWorkflowSteps, "Generating release notes")
	notesOutput, err := runReleaseNotes(ctx, app, planOutput)
	if err != nil {
		return fmt.Errorf("notes failed: %w", err)
	}
	if notesOutput.ReleaseNotes != nil {
		fmt.Printf("  Generated release notes: %s\n", notesOutput.ReleaseNotes.Title())
	}
	fmt.Println()

	// Step 4: Approve
	printStep(4, releaseWorkflowSteps, "Reviewing release")
	approved, err := runReleaseApprove(ctx, app, planOutput, notesOutput, releaseAutoApprove)
	if err != nil {
		return fmt.Errorf("approval failed: %w", err)
	}
	if !approved {
		printWarning("Release canceled by user")
		return nil
	}
	fmt.Println()

	// Step 5: Publish
	printStep(5, releaseWorkflowSteps, "Publishing release")
	if dryRun {
		printInfo("Dry run - skipping actual publish")
		printSuccess("Release workflow completed (dry run)")
		return nil
	}

	publishOutput, err := runReleasePublish(ctx, app, planOutput)
	if err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}

	fmt.Println()

	// Show appropriate success message based on skip-push
	if releaseSkipPush {
		printSuccess(fmt.Sprintf("Created %s locally (push skipped)", bumpOutput.Version.String()))
		fmt.Printf("  ✓ Tag created locally: %s\n", bumpOutput.TagName)
		printInfo("Run 'git push origin --tags' to publish when ready")
	} else {
		printSuccess(fmt.Sprintf("Released %s successfully!", bumpOutput.Version.String()))

		// Show publish summary
		if publishOutput.TagName != "" {
			fmt.Printf("  ✓ Tag: %s\n", publishOutput.TagName)
		}
		if publishOutput.ReleaseURL != "" {
			fmt.Printf("  ✓ URL: %s\n", publishOutput.ReleaseURL)
		}
		for _, pr := range publishOutput.PluginResults {
			if pr.Success {
				fmt.Printf("  ✓ Plugin %s: %s\n", pr.PluginName, pr.Message)
			}
		}
	}

	return nil
}

// runReleasePlan executes the plan step using the shared workflow context.
// In tag-push mode, it plans for the existing tag instead of HEAD.
func runReleasePlan(ctx context.Context, c cliApp, wfCtx *releaseWorkflowContext) (*servicerelease.AnalyzeOutput, error) {
	gitAdapter := c.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	// Use pre-detected mode from workflow context
	var fromRef, toRef string
	if wfCtx.mode == releaseModeTagPush && wfCtx.existingVersion != nil {
		// In tag-push mode, plan for the existing tag
		tagName := cfg.Versioning.TagPrefix + wfCtx.existingVersion.String()
		toRef = tagName
		fromRef = wfCtx.prevTagName // Use pre-computed previous tag
	} else {
		fromRef = "" // Use latest tag
		toRef = "HEAD"
	}

	// Use release analyzer for analysis
	analyzer := c.ReleaseAnalyzer()
	if analyzer == nil {
		return nil, fmt.Errorf("release analyzer not available")
	}

	input := servicerelease.AnalyzeInput{
		RepositoryPath: repoInfo.Path,
		Branch:         repoInfo.CurrentBranch,
		FromRef:        fromRef,
		ToRef:          toRef,
		TagPrefix:      cfg.Versioning.TagPrefix,
	}

	output, err := analyzer.Analyze(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze commits: %w", err)
	}

	// In tag-push mode, ensure next version matches the existing tag
	if wfCtx.mode == releaseModeTagPush && wfCtx.existingVersion != nil {
		output.NextVersion = *wfCtx.existingVersion
	}

	// Persist the release run using DDD services
	if !dryRun {
		if err := persistReleasePlan(ctx, c, output, repoInfo.Path, repoInfo.Name); err != nil {
			return nil, fmt.Errorf("failed to persist release plan: %w", err)
		}
	}

	return output, nil
}

// persistReleasePlan persists the release plan using DDD services.
func persistReleasePlan(ctx context.Context, c cliApp, output *servicerelease.AnalyzeOutput, repoPath, repoID string) error {
	if err := c.InitReleaseServices(ctx, repoPath); err != nil {
		return fmt.Errorf("failed to initialize release services: %w", err)
	}
	if !c.HasReleaseServices() {
		return fmt.Errorf("release services not available")
	}
	services := c.ReleaseServices()
	if services == nil || services.PlanRelease == nil {
		return fmt.Errorf("PlanRelease use case not available")
	}

	// Convert domain types for DDD input
	bumpKind := releaseTypeToBumpKind(output.ReleaseType)

	planInput := releaseapp.PlanReleaseInput{
		RepoRoot:       repoPath,
		RepoID:         repoID,
		ChangeSet:      output.ChangeSet,
		CurrentVersion: &output.CurrentVersion,
		NextVersion:    &output.NextVersion,
		BumpKind:       &bumpKind,
		Actor: ports.ActorInfo{
			Type: "user",
			ID:   "cli",
		},
		Force: false,
	}

	_, err := services.PlanRelease.Execute(ctx, planInput)
	return err
}

// releaseTypeToBumpKind converts changes.ReleaseType to domain.BumpKind.
func releaseTypeToBumpKind(rt changes.ReleaseType) releasedomain.BumpKind {
	switch rt {
	case changes.ReleaseTypeMajor:
		return releasedomain.BumpMajor
	case changes.ReleaseTypeMinor:
		return releasedomain.BumpMinor
	case changes.ReleaseTypePatch:
		return releasedomain.BumpPatch
	default:
		return releasedomain.BumpPatch
	}
}

// runReleaseBump executes the bump step using the shared workflow context.
// In tag-push mode, it skips tag creation since the tag already exists.
// Tag creation is handled here; publish step does not create tags.
func runReleaseBump(ctx context.Context, c cliApp, wfCtx *releaseWorkflowContext, plan *servicerelease.AnalyzeOutput) (*versioning.SetVersionOutput, error) {
	var ver version.SemanticVersion
	if releaseForce != "" {
		parsed, err := version.Parse(releaseForce)
		if err != nil {
			return nil, fmt.Errorf("invalid version %q: %w", releaseForce, err)
		}
		ver = parsed
	} else {
		ver = plan.NextVersion
	}

	// Use pre-detected mode: skip tag creation if tag already exists
	createTag := cfg.Versioning.GitTag
	if wfCtx.mode == releaseModeTagPush && wfCtx.existingVersion != nil && wfCtx.existingVersion.Compare(ver) == 0 {
		createTag = false // Tag already exists
	}

	input := versioning.SetVersionInput{
		Version:    ver,
		TagPrefix:  cfg.Versioning.TagPrefix,
		CreateTag:  createTag,
		PushTag:    cfg.Versioning.GitPush && !releaseSkipPush,
		Remote:     "origin",
		TagMessage: fmt.Sprintf("Release %s", ver.String()),
		DryRun:     dryRun,
	}

	output, err := c.SetVersion().Execute(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to set version: %w", err)
	}

	// Update release state - MUST succeed for workflow to continue
	if err := updateReleaseVersion(ctx, c, output.Version); err != nil {
		return nil, fmt.Errorf("failed to update release state: %w", err)
	}

	return output, nil
}

// releaseNotesResult holds the result of notes generation for the workflow.
type releaseNotesResult struct {
	ReleaseNotes *communication.ReleaseNotes
}

// releasePublishResult holds the result of publishing for the workflow.
type releasePublishResult struct {
	TagName       string
	ReleaseURL    string
	PluginResults []releasePluginResult
}

// releasePluginResult holds the result of a plugin execution.
type releasePluginResult struct {
	PluginName string
	Success    bool
	Message    string
}

// runReleaseNotes executes the notes generation step using DDD services.
func runReleaseNotes(ctx context.Context, c cliApp, plan *servicerelease.AnalyzeOutput) (*releaseNotesResult, error) {
	gitAdapter := c.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	if err := c.InitReleaseServices(ctx, repoInfo.Path); err != nil {
		return nil, fmt.Errorf("failed to initialize release services: %w", err)
	}
	if !c.HasReleaseServices() {
		return nil, fmt.Errorf("release services not available")
	}
	services := c.ReleaseServices()
	if services == nil || services.GenerateNotes == nil {
		return nil, fmt.Errorf("GenerateNotes use case not available")
	}

	input := releaseapp.GenerateNotesInput{
		RepoRoot: repoInfo.Path,
		Options: ports.NotesOptions{
			AudiencePreset: cfg.AI.Audience,
			TonePreset:     cfg.AI.Tone,
			UseAI:          cfg.AI.Enabled,
			RepositoryURL:  cfg.Changelog.RepositoryURL,
		},
		Actor: ports.ActorInfo{
			Type: "user",
			ID:   "cli",
		},
		Force: false,
	}

	output, err := services.GenerateNotes.Execute(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to generate notes: %w", err)
	}

	// Build a ReleaseNotes object from output
	result := &releaseNotesResult{}
	if output.Notes != nil {
		notes := communication.NewReleaseNotesBuilder(plan.NextVersion).
			WithTitle(fmt.Sprintf("Release %s", plan.NextVersion.String())).
			WithSummary(output.Notes.Text).
			Build()
		result.ReleaseNotes = notes
	}
	return result, nil
}

// runReleaseApprove handles the approval step.
func runReleaseApprove(ctx context.Context, c cliApp, plan *servicerelease.AnalyzeOutput, notes *releaseNotesResult, autoApprove bool) (bool, error) {
	if autoApprove {
		printInfo("Auto-approving release")
		return runReleaseApproveExecute(ctx, c, "auto-approve")
	}

	// Interactive approval
	if !isTerminal() {
		return false, fmt.Errorf("interactive approval required but not running in terminal (use --yes for non-interactive)")
	}

	// Show release preview
	fmt.Println()
	printSubtitle("Release Preview")
	fmt.Printf("  Version: %s\n", plan.NextVersion.String())
	fmt.Printf("  Type: %s\n", plan.ReleaseType)
	if plan.ChangeSet != nil {
		fmt.Printf("  Changes: %d commits\n", plan.ChangeSet.CommitCount())
		if plan.ChangeSet.HasBreakingChanges() {
			printWarning("  ⚠ Contains breaking changes")
		}
	}
	fmt.Println()

	if notes != nil && notes.ReleaseNotes != nil {
		fmt.Println("Release Notes Preview:")
		preview := notes.ReleaseNotes.Render()
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		fmt.Println(preview)
		fmt.Println()
	}

	// Prompt for approval
	fmt.Print("Approve this release? [y/N]: ")
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		return false, nil // Treat as "no"
	}

	approved := response == "y" || response == "Y" || response == "yes" || response == "Yes"
	if approved {
		return runReleaseApproveExecute(ctx, c, "user")
	}
	return approved, nil
}

// runReleaseApproveExecute executes the approval using DDD services.
func runReleaseApproveExecute(ctx context.Context, c cliApp, approvedBy string) (bool, error) {
	gitAdapter := c.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get repository info: %w", err)
	}

	if err := c.InitReleaseServices(ctx, repoInfo.Path); err != nil {
		return false, fmt.Errorf("failed to initialize release services: %w", err)
	}
	if !c.HasReleaseServices() {
		return false, fmt.Errorf("release services not available")
	}
	services := c.ReleaseServices()
	if services == nil || services.ApproveRelease == nil {
		return false, fmt.Errorf("ApproveRelease use case not available")
	}

	input := releaseapp.ApproveReleaseInput{
		RepoRoot: repoInfo.Path,
		Actor: ports.ActorInfo{
			Type: "user",
			ID:   approvedBy,
		},
		AutoApprove: true,
		Force:       true,
	}
	_, err = services.ApproveRelease.Execute(ctx, input)
	if err != nil {
		return false, fmt.Errorf("approval failed: %w", err)
	}
	return true, nil
}

// runReleasePublish executes the publish step using DDD services.
// Tag creation is handled by runReleaseBump; this step only publishes.
func runReleasePublish(ctx context.Context, c cliApp, plan *servicerelease.AnalyzeOutput) (*releasePublishResult, error) {
	gitAdapter := c.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	if err := c.InitReleaseServices(ctx, repoInfo.Path); err != nil {
		return nil, fmt.Errorf("failed to initialize release services: %w", err)
	}
	if !c.HasReleaseServices() {
		return nil, fmt.Errorf("release services not available")
	}
	services := c.ReleaseServices()
	if services == nil || services.PublishRelease == nil {
		return nil, fmt.Errorf("PublishRelease use case not available")
	}

	input := releaseapp.PublishReleaseInput{
		RepoRoot: repoInfo.Path,
		Actor: ports.ActorInfo{
			Type: "user",
			ID:   "cli",
		},
		Force:  true,
		DryRun: dryRun,
	}

	output, err := services.PublishRelease.Execute(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to publish release: %w", err)
	}

	// Build result
	result := &releasePublishResult{
		TagName: cfg.Versioning.TagPrefix + plan.NextVersion.String(),
	}

	// Convert step results to plugin results format
	for _, step := range output.StepResults {
		if step.StepName != "" {
			result.PluginResults = append(result.PluginResults, releasePluginResult{
				PluginName: step.StepName,
				Success:    step.Success,
				Message:    step.Output,
			})
		}
	}

	return result, nil
}

// Helper functions

func printStep(current, total int, message string) {
	fmt.Printf("[%d/%d] %s\n", current, total, message)
}

func printSubtitle(s string) {
	fmt.Println(styles.Bold.Render(s))
}

func isTerminal() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// detectWorkflowContext creates a releaseWorkflowContext by detecting the mode once.
// This includes finding the existing version if HEAD is tagged, and the previous tag for commit range.
func detectWorkflowContext(ctx context.Context, c cliApp) (*releaseWorkflowContext, error) {
	mode, existingVer, err := detectReleaseMode(ctx, c, cfg.Versioning.TagPrefix)
	if err != nil {
		return nil, err
	}

	wfCtx := &releaseWorkflowContext{
		mode:            mode,
		existingVersion: existingVer,
	}

	// In tag-push mode, find the previous tag for commit range
	if mode == releaseModeTagPush && existingVer != nil {
		prevTag, err := findPreviousVersionTag(ctx, c, existingVer)
		if err == nil && prevTag != "" {
			wfCtx.prevTagName = prevTag
		}
	}

	return wfCtx, nil
}

// findPreviousVersionTag finds the highest version tag that is less than the given version.
func findPreviousVersionTag(ctx context.Context, c cliApp, currentVer *version.SemanticVersion) (string, error) {
	tags, err := c.GitAdapter().GetTags(ctx)
	if err != nil {
		return "", err
	}

	var prevTagName string
	var prevVer version.SemanticVersion

	for _, t := range tags.FilterByPrefix(cfg.Versioning.TagPrefix).VersionTags() {
		tagVer := t.Version()
		if tagVer == nil || !tagVer.LessThan(*currentVer) {
			continue
		}
		// Find the highest version less than current
		if prevTagName == "" || tagVer.GreaterThan(prevVer) {
			prevTagName = t.Name()
			prevVer = *tagVer
		}
	}

	return prevTagName, nil
}

// detectReleaseMode determines if this is a tag-push scenario.
// Returns the mode and the version tag if HEAD is tagged.
func detectReleaseMode(ctx context.Context, c cliApp, tagPrefix string) (releaseMode, *version.SemanticVersion, error) {
	gitAdapter := c.GitAdapter()

	// Get HEAD commit hash
	headCommit, err := gitAdapter.GetLatestCommit(ctx, "HEAD")
	if err != nil {
		return releaseModeNew, nil, nil // Can't detect, assume new release
	}

	// Get all version tags
	tags, err := gitAdapter.GetTags(ctx)
	if err != nil {
		return releaseModeNew, nil, nil // Can't detect, assume new release
	}

	// Check if any version tag points to HEAD
	for _, tag := range tags.FilterByPrefix(tagPrefix).VersionTags() {
		if tag.Hash() == headCommit.Hash() {
			ver := tag.Version()
			if ver != nil {
				return releaseModeTagPush, ver, nil
			}
		}
	}

	return releaseModeNew, nil, nil
}

// effectiveBumpType derives the actual bump type from comparing two versions.
// This is used for display when ReleaseType is "none" but a version bump still occurs.
func effectiveBumpType(current, next version.SemanticVersion) string {
	if next.Major() > current.Major() {
		return "major"
	}
	if next.Minor() > current.Minor() {
		return "minor"
	}
	if next.Patch() > current.Patch() {
		return "patch"
	}
	return "none"
}
