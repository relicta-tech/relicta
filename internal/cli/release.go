// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	apprelease "github.com/relicta-tech/relicta/internal/application/release"
	"github.com/relicta-tech/relicta/internal/application/versioning"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/version"
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

	// Detect release mode (normal vs tag-push)
	mode, existingVersion, err := detectReleaseMode(ctx, app, cfg.Versioning.TagPrefix)
	if err != nil {
		return fmt.Errorf("failed to detect release mode: %w", err)
	}

	// Handle tag-push mode (HEAD is already tagged)
	if mode == releaseModeTagPush && existingVersion != nil {
		return runReleaseTagPush(ctx, app, *existingVersion)
	}

	// Normal release flow
	return runReleaseNew(ctx, app)
}

// runReleaseNew runs the normal release workflow for new commits.
func runReleaseNew(ctx context.Context, app cliApp) error {
	// Step 1: Plan
	printStep(1, 5, "Planning release")
	planOutput, err := runReleasePlan(ctx, app)
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

	// Show plan summary
	fmt.Printf("  Version: %s → %s (%s)\n",
		planOutput.CurrentVersion.String(),
		planOutput.NextVersion.String(),
		planOutput.ReleaseType)
	fmt.Printf("  Commits: %d\n", commitCount)
	fmt.Println()

	// Step 2: Bump version
	printStep(2, 5, "Bumping version")
	bumpOutput, err := runReleaseBump(ctx, app, planOutput)
	if err != nil {
		return fmt.Errorf("bump failed: %w", err)
	}
	fmt.Printf("  Created tag: %s\n", bumpOutput.TagName)
	fmt.Println()

	// Step 3: Generate notes
	printStep(3, 5, "Generating release notes")
	notesOutput, err := runReleaseNotes(ctx, app, planOutput)
	if err != nil {
		return fmt.Errorf("notes failed: %w", err)
	}
	if notesOutput.ReleaseNotes != nil {
		fmt.Printf("  Generated release notes: %s\n", notesOutput.ReleaseNotes.Title())
	}
	fmt.Println()

	// Step 4: Approve
	printStep(4, 5, "Reviewing release")
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
	printStep(5, 5, "Publishing release")
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

// runReleaseTagPush handles the tag-push scenario where HEAD is already tagged.
// This is common in CI/CD when the workflow is triggered by a tag push.
func runReleaseTagPush(ctx context.Context, app cliApp, ver version.SemanticVersion) error {
	tagName := cfg.Versioning.TagPrefix + ver.String()

	printInfo(fmt.Sprintf("Detected existing tag: %s", tagName))
	printInfo("Running in tag-push mode - skipping plan and bump")
	fmt.Println()

	// Step 1: Get commits for the existing tag to generate notes
	printStep(1, 3, "Analyzing changes for existing tag")
	gitAdapter := app.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	// Get previous version tag to find commits between
	tags, err := gitAdapter.GetTags(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tags: %w", err)
	}

	// Find the previous version tag (the highest version that is less than current)
	var prevTagName string
	var prevVersion *version.SemanticVersion
	versionTags := tags.FilterByPrefix(cfg.Versioning.TagPrefix).VersionTags()
	for _, t := range versionTags {
		tagVer := t.Version()
		if tagVer != nil && tagVer.LessThan(ver) {
			if prevVersion == nil || tagVer.GreaterThan(*prevVersion) {
				prevTagName = t.Name()
				prevVersion = tagVer
			}
		}
	}

	// Get commits between previous tag and current
	var commitCount int
	if prevTagName != "" {
		commits, err := gitAdapter.GetCommitsBetween(ctx, prevTagName, tagName)
		if err != nil {
			printWarning(fmt.Sprintf("Could not get commits: %v", err))
		} else {
			commitCount = len(commits)
		}
	}

	fmt.Printf("  Found %d commits since %s\n", commitCount, prevTagName)
	fmt.Println()

	// Create a plan for the existing tag
	planInput := apprelease.PlanReleaseInput{
		RepositoryPath: repoInfo.Path,
		Branch:         repoInfo.CurrentBranch,
		FromRef:        prevTagName,
		ToRef:          tagName,
		DryRun:         dryRun,
		TagPrefix:      cfg.Versioning.TagPrefix,
	}

	planOutput, err := app.PlanRelease().Execute(ctx, planInput)
	if err != nil {
		// If plan fails (e.g., no commits), create a minimal plan
		printWarning(fmt.Sprintf("Plan failed: %v - using minimal plan", err))
		planOutput = &apprelease.PlanReleaseOutput{
			ReleaseID:      release.ReleaseID(fmt.Sprintf("rel-%d", time.Now().UnixNano())),
			CurrentVersion: ver, // Use same as next for existing tag
			NextVersion:    ver,
			ReleaseType:    "patch",
		}
	}

	// Override with the actual version from the tag
	planOutput.NextVersion = ver

	// Step 2: Generate notes
	printStep(2, 3, "Generating release notes")
	notesOutput, err := runReleaseNotes(ctx, app, planOutput)
	if err != nil {
		printWarning(fmt.Sprintf("Notes generation failed: %v", err))
		// Continue anyway - we can publish without notes
	} else if notesOutput.ReleaseNotes != nil {
		fmt.Printf("  Generated release notes: %s\n", notesOutput.ReleaseNotes.Title())
	}
	fmt.Println()

	// Auto-approve in tag-push mode (the tag exists, user already approved by pushing)
	if !releaseAutoApprove {
		printInfo("Auto-approving (tag already exists)")
	}
	approveInput := apprelease.ApproveReleaseInput{
		ReleaseID:  planOutput.ReleaseID,
		ApprovedBy: "tag-push-auto",
	}
	_, _ = app.ApproveRelease().Execute(ctx, approveInput)

	// Step 3: Publish
	printStep(3, 3, "Publishing release")
	if dryRun {
		printInfo("Dry run - skipping actual publish")
		printSuccess(fmt.Sprintf("Release workflow completed for %s (dry run)", tagName))
		return nil
	}

	// Publish - tag already exists, just run plugins
	publishInput := apprelease.PublishReleaseInput{
		ReleaseID: planOutput.ReleaseID,
		DryRun:    dryRun,
		CreateTag: false, // Tag already exists
		PushTag:   false, // Tag already pushed (triggered by tag push)
		TagPrefix: cfg.Versioning.TagPrefix,
		Remote:    "origin",
	}

	publishOutput, err := app.PublishRelease().Execute(ctx, publishInput)
	if err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}

	fmt.Println()
	printSuccess(fmt.Sprintf("Released %s successfully!", ver.String()))

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

	return nil
}

// runReleasePlan executes the plan step.
func runReleasePlan(ctx context.Context, c cliApp) (*apprelease.PlanReleaseOutput, error) {
	gitAdapter := c.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	input := apprelease.PlanReleaseInput{
		RepositoryPath: repoInfo.Path,
		Branch:         repoInfo.CurrentBranch,
		FromRef:        "", // Use latest tag
		ToRef:          "HEAD",
		DryRun:         dryRun,
		TagPrefix:      cfg.Versioning.TagPrefix,
	}

	return c.PlanRelease().Execute(ctx, input)
}

// runReleaseBump executes the bump step.
func runReleaseBump(ctx context.Context, c cliApp, plan *apprelease.PlanReleaseOutput) (*versioning.SetVersionOutput, error) {
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

	input := versioning.SetVersionInput{
		Version:    ver,
		TagPrefix:  cfg.Versioning.TagPrefix,
		CreateTag:  cfg.Versioning.GitTag,
		PushTag:    cfg.Versioning.GitPush && !releaseSkipPush,
		Remote:     "origin",
		TagMessage: fmt.Sprintf("Release %s", ver.String()),
		DryRun:     dryRun,
	}

	output, err := c.SetVersion().Execute(ctx, input)
	if err != nil {
		return nil, err
	}

	// Update release state (same as bump command - non-fatal if fails)
	_ = updateReleaseVersion(ctx, c, output.Version)

	return output, nil
}

// runReleaseNotes executes the notes generation step.
func runReleaseNotes(ctx context.Context, c cliApp, plan *apprelease.PlanReleaseOutput) (*apprelease.GenerateNotesOutput, error) {
	input := apprelease.GenerateNotesInput{
		ReleaseID:        plan.ReleaseID,
		UseAI:            cfg.AI.Enabled,
		Tone:             parseNoteTone(cfg.AI.Tone),
		Audience:         parseNoteAudience(cfg.AI.Audience),
		IncludeChangelog: true,
		RepositoryURL:    cfg.Changelog.RepositoryURL,
	}

	return c.GenerateNotes().Execute(ctx, input)
}

// runReleaseApprove handles the approval step.
func runReleaseApprove(ctx context.Context, c cliApp, plan *apprelease.PlanReleaseOutput, notes *apprelease.GenerateNotesOutput, autoApprove bool) (bool, error) {
	if autoApprove {
		printInfo("Auto-approving release")
		// Actually approve the release in the domain model
		input := apprelease.ApproveReleaseInput{
			ReleaseID:  plan.ReleaseID,
			ApprovedBy: "auto-approve",
		}
		_, err := c.ApproveRelease().Execute(ctx, input)
		if err != nil {
			return false, fmt.Errorf("auto-approve failed: %w", err)
		}
		return true, nil
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

	if notes.ReleaseNotes != nil {
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
		// Actually approve the release in the domain model
		input := apprelease.ApproveReleaseInput{
			ReleaseID:  plan.ReleaseID,
			ApprovedBy: "user",
		}
		_, err := c.ApproveRelease().Execute(ctx, input)
		if err != nil {
			return false, fmt.Errorf("approval failed: %w", err)
		}
	}
	return approved, nil
}

// runReleasePublish executes the publish step.
func runReleasePublish(ctx context.Context, c cliApp, plan *apprelease.PlanReleaseOutput) (*apprelease.PublishReleaseOutput, error) {
	input := apprelease.PublishReleaseInput{
		ReleaseID: plan.ReleaseID,
		DryRun:    dryRun,
		CreateTag: cfg.Versioning.GitTag && !releaseSkipPush,
		PushTag:   cfg.Versioning.GitPush && !releaseSkipPush,
		TagPrefix: cfg.Versioning.TagPrefix,
		Remote:    "origin",
	}

	return c.PublishRelease().Execute(ctx, input)
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
