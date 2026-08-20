// Package cli provides the command-line interface for Relicta.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/relicta-tech/relicta/v4/internal/application/governance"
	appmonorepo "github.com/relicta-tech/relicta/v4/internal/application/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/container"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/communication"
	"github.com/relicta-tech/relicta/v4/internal/domain/release"
	releaseapp "github.com/relicta-tech/relicta/v4/internal/domain/release/app"
	releasedomain "github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/v4/internal/domain/release/ports"
	gitservice "github.com/relicta-tech/relicta/v4/internal/infrastructure/git"
)

var (
	publishSkipApproval bool
	publishSkipTag      bool
	publishSkipPush     bool
	publishSkipPlugins  bool
)

func init() {
	publishCmd.Flags().BoolVarP(&publishSkipApproval, "skip-approval", "A", false, "skip approval check")
	publishCmd.Flags().BoolVarP(&publishSkipTag, "skip-tag", "T", false, "skip git tag creation")
	publishCmd.Flags().BoolVarP(&publishSkipPush, "skip-push", "P", false, "skip pushing to remote")
	publishCmd.Flags().BoolVarP(&publishSkipPlugins, "skip-plugins", "G", false, "skip running plugins")
}

// shouldCreateTag returns whether a tag should be created.
func shouldCreateTag() bool {
	return !publishSkipTag && cfg.Versioning.GitTag
}

// shouldPushTag returns whether the tag should be pushed.
func shouldPushTag() bool {
	return !publishSkipPush && cfg.Versioning.GitPush
}

// shouldRunPlugins returns whether plugins should be executed.
func shouldRunPlugins() bool {
	return !publishSkipPlugins && len(cfg.Plugins) > 0
}

// displayPublishActions displays what actions will be performed.
func displayPublishActions(nextVersion string) {
	fmt.Println()
	printTitle("Release Actions")
	fmt.Println()
	fmt.Printf("  Version:    %s%s\n", cfg.Versioning.TagPrefix, nextVersion)
	fmt.Printf("  Create tag: %v\n", shouldCreateTag())
	fmt.Printf("  Push:       %v\n", shouldPushTag())
	fmt.Printf("  Plugins:    %v\n", shouldRunPlugins())
	// Listed because --dry-run does not run them, and a preview that omits the two commands
	// the real publish would execute is not previewing the release the operator configured.
	if cfg.Workflow.PreReleaseHook != "" {
		fmt.Printf("  Pre hook:   %s\n", cfg.Workflow.PreReleaseHook)
	}
	if cfg.Workflow.PostReleaseHook != "" {
		fmt.Printf("  Post hook:  %s\n", cfg.Workflow.PostReleaseHook)
	}
	fmt.Println()
}

// outputStepResults outputs the results of step executions.
func outputStepResults(results []releaseapp.StepResult) {
	if len(results) == 0 {
		return
	}

	fmt.Println()
	printTitle("Step Results")
	fmt.Println()
	for _, result := range results {
		if result.Skipped {
			printInfo(fmt.Sprintf("  %s: skipped", result.StepName))
		} else if result.Success {
			printSuccess(fmt.Sprintf("  %s: %s", result.StepName, result.Output))
		} else {
			printErrorResult(fmt.Sprintf("  %s: %s", result.StepName, result.Error))
		}
	}
}

// handleChangelogUpdate updates the changelog file if configured.
//
// Skips a version already present, which is what makes a retried publish safe: the changelog
// is now written before the tag, so a publish that fails afterwards leaves an entry behind,
// and the second attempt must not insert it twice.
func handleChangelogUpdate(rel *release.ReleaseRun) {
	if cfg.Changelog.File == "" || rel.Notes() == nil || rel.Notes().Text == "" {
		return
	}

	entry := changelogEntryFor(rel)
	if changelogAlreadyContains(cfg.Changelog.File, entry) {
		printInfo(fmt.Sprintf("%s already describes this release", cfg.Changelog.File))
		return
	}

	printInfo(fmt.Sprintf("Updating %s...", cfg.Changelog.File))
	if err := updateChangelogFile(cfg.Changelog.File, entry); err != nil {
		printWarning(fmt.Sprintf("Failed to update changelog: %v", err))
	} else {
		printSuccess(fmt.Sprintf("Updated %s", cfg.Changelog.File))
	}
}

// changelogEntryFor renders the entry to insert into the changelog: the release notes under a
// version heading.
//
// The heading is added here rather than baked into the notes because the notes are shown
// wherever a release is announced, while the heading is what makes a changelog *file* a
// sequence of releases. Without it every release ran into the previous one — the file was a
// flat list of bullets with nothing marking where 0.2.0 ended and 0.1.0 began — and
// findVersionEntryPoint, which inserts a new release above the last one, had no "## [" to find
// and appended to the bottom instead.
//
// Notes that already open with their own version heading — an AI provider asked to write in
// Keep a Changelog style will — are left as they are rather than given a second one.
func changelogEntryFor(rel *release.ReleaseRun) string {
	notes := strings.TrimSpace(stripChangelogHeader(rel.Notes().Text))

	heading := communication.RenderVersionHeading(communication.ChangelogEntry{
		Version: rel.VersionNext(),
		Date:    time.Now(),
	})

	if strings.HasPrefix(notes, "## ") {
		return notes
	}
	return heading + "\n\n" + notes
}

// changelogAlreadyContains reports whether the changelog already carries this release's notes.
func changelogAlreadyContains(filename, notes string) bool {
	data, err := os.ReadFile(filename) // #nosec G304 -- user-specified changelog path
	if err != nil {
		return false
	}
	entry := strings.TrimSpace(stripChangelogHeader(notes))
	return entry != "" && strings.Contains(string(data), entry)
}

// releaseCommitPaths lists the files relicta itself writes as part of a release: the changelog
// it renders, and every version-bearing manifest `relicta bump` updated.
//
// These are the paths the release commit covers, and — because they are relicta's own edits
// rather than the operator's uncommitted work — the paths the clean-tree gate ignores when
// that commit is going to happen.
func releaseCommitPaths(ctx context.Context) []string {
	if cfg == nil {
		return nil
	}

	paths := make([]string, 0, 4)
	if cfg.Changelog.File != "" {
		paths = append(paths, cfg.Changelog.File)
	}
	for _, target := range cfg.Versioning.ResolvedVersionFiles() {
		if target.Path != "" {
			paths = append(paths, target.Path)
		}
	}
	return append(paths, monorepoManifestPaths(ctx)...)
}

// monorepoManifestPaths is the per-package half of the same list.
//
// In a monorepo the version-bearing manifests are the packages' own, and they are found by
// walking monorepo.package_paths rather than read from configuration. Without them the release
// commit left every package.json `relicta bump` had just written uncommitted, and the clean-tree
// gate then refused the publish — reproduced against the shipped binary:
//
//	⚠ Uncommitted changes to tracked files:
//	    packages/api/package.json
//	    packages/web/package.json
//
// Best-effort: a discovery failure yields no paths rather than an error, because the caller is
// building a list of files to commit and the repository-wide ones are still worth committing.
func monorepoManifestPaths(ctx context.Context) []string {
	if cfg == nil || !cfg.Monorepo.Enabled {
		return nil
	}

	root, err := os.Getwd()
	if err != nil {
		return nil
	}

	paths, err := container.NewMonorepoBumper(nil, nil).ManifestPaths(ctx,
		appmonorepo.PlanInput{
			RepoRoot:     root,
			PackagePaths: cfg.Monorepo.PackagePaths,
			ExcludePaths: cfg.Monorepo.ExcludePaths,
		})
	if err != nil {
		return nil
	}
	return paths
}

// commitReleaseArtifacts writes the changelog and commits it with the version files, so the
// tag that follows points at a commit containing both.
//
// workflow.auto_commit_changelog defaults to true and was read by nothing, which left the
// release incoherent in two ways at once. The changelog was written after the tag was created,
// so the tag never contained the release notes describing it; and `relicta bump` writes the
// configured version manifests without committing them, so the tagged package.json still
// carried the previous version. Verified against the shipped binary: after a full release the
// tree held an uncommitted CHANGELOG.md and a modified package.json, and `git show v0.1.0:
// package.json` reported 0.0.0 for a release tagged 0.1.0.
//
// Once require_clean_working_tree was enforced, the same gap stopped the release outright —
// bump dirtied package.json and publish then refused it. A release tool has to commit what it
// writes, or it cannot honestly ask for a clean tree.
func commitReleaseArtifacts(ctx context.Context, rel *release.ReleaseRun, ver string) error {
	handleChangelogUpdate(rel)

	if cfg == nil || !cfg.Workflow.AutoCommitChangelog {
		return nil
	}

	paths := releaseCommitPaths(ctx)
	if len(paths) == 0 {
		return nil
	}

	svc, err := gitservice.NewService()
	if err != nil {
		return fmt.Errorf("failed to open repository for the release commit: %w", err)
	}

	message := strings.ReplaceAll(cfg.Workflow.ChangelogCommitMessage, "${version}", ver)
	if message == "" {
		message = "chore(release): " + ver
	}

	hash, err := svc.CommitPaths(ctx, paths, message)
	if err != nil {
		// Refuse rather than tag: the alternative is a tag on a commit that lacks the
		// changelog and version files, which is the state this exists to prevent.
		return fmt.Errorf("failed to commit the release files: %w", err)
	}
	if hash == "" {
		// Nothing differed — a re-run after the commit already landed, or a project that
		// keeps no changelog and no version files.
		return nil
	}

	printSuccess(fmt.Sprintf("Committed release files (%s)", hash[:min(len(hash), 7)]))
	return nil
}

// printPublishSummary prints the final release summary.
func printPublishSummary(nextVersion, tagName string, remoteURL string) {
	fmt.Println()
	printTitle("Release Summary")
	fmt.Println()
	fmt.Printf("  Version:    %s%s\n", cfg.Versioning.TagPrefix, nextVersion)
	if cfg.Versioning.GitTag {
		fmt.Printf("  Tag:        %s\n", tagName)
	} else {
		fmt.Printf("  Tag:        %s (not created — tagging is disabled)\n", tagName)
	}
	fmt.Printf("  Status:     published\n")
	fmt.Printf("  Published:  %s\n", time.Now().Format(time.RFC3339))

	printSuccess("Release completed successfully!")

	// Show helpful hints for creating platform releases
	if !hasPlugin(cfg, "github") && isGitHubRemote(remoteURL) {
		fmt.Println()
		printInfo("To create a GitHub Release, either:")
		printSubtle("  • Run: relicta plugin install github")
		printSubtle("  • Or manually: gh release create " + tagName + " --generate-notes")
	}
	if !hasPlugin(cfg, "gitlab") && isGitLabRemote(remoteURL) {
		fmt.Println()
		printInfo("To create a GitLab Release, run: relicta plugin install gitlab")
	}

	fmt.Println()
	printInfo("Run 'relicta plan' to start a new release.")
	fmt.Println()
}

// runPublish implements the publish command.
func runPublish(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	printTitle("Release Publish")
	fmt.Println()

	warnRepositoryWideInAMonorepo("publish")

	if dryRun {
		printDryRunBanner()
	}

	// Fold --skip-push into the config before the container reads it, so the
	// flag and the setting cannot disagree about whether to push. The publisher
	// is configured from cfg.Versioning.GitPush at construction; without this the
	// flag would only affect what gets printed, which is how it behaved before:
	// "Push: false" on screen, tag pushed regardless.
	if publishSkipPush {
		cfg.Versioning.GitPush = false
	}

	// And --skip-tag into GitTag, for the same reason and after the same defect: the flag
	// reached the summary and the JSON only, so `publish --skip-tag` printed
	// "Create tag: false" and tagged anyway.
	if publishSkipTag {
		cfg.Versioning.GitTag = false
	}

	// Initialize container
	app, err := newContainerApp(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer closeApp(app)

	// Get repository info for domain services
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
	if services == nil || services.PublishRelease == nil {
		return fmt.Errorf("PublishRelease use case not available")
	}

	return runPublishWithServices(ctx, app, repoInfo.Path, repoInfo.RemoteURL)
}

// runPublishWithServices publishes using the PublishReleaseUseCase.
func runPublishWithServices(ctx context.Context, app cliApp, repoPath, remoteURL string) error {
	services := app.ReleaseServices()

	// Load release from repository to get version
	run, err := services.Repository.LoadLatest(ctx, repoPath)
	if err != nil {
		return fmt.Errorf("failed to load release: %w", err)
	}

	// The gates every tagging path applies, in the one place both of them read from — the
	// autonomy budget, then require_clean_working_tree, then require_up_to_date. See
	// publish_gates.go for why they run in that order and what happened while `relicta
	// release` had its own copy of none of them.
	if err := enforcePrePublishGates(ctx, run.RiskScore()); err != nil {
		return err
	}

	nextVersion := run.VersionNext().String()

	// Output JSON if requested
	if outputJSON {
		return outputPublishJSONFromServices(run)
	}

	// Get governance evaluation for outcome tracking (if enabled)
	var govResult *governance.EvaluateReleaseOutput
	if app.HasGovernance() {
		// Load legacy release for governance (it reads from same path)
		if rel, err := getLatestRelease(ctx, app); err == nil {
			govResult, _ = evaluateGovernanceForPublish(ctx, app, rel)
			captureGovernanceAnalytics(ctx, app, string(run.ID()), govResult)
			if govResult != nil && cfg.Governance.StrictMode && govResult.Decision == cgp.DecisionRejected {
				printError("Release blocked by governance policy")
				return fmt.Errorf("release denied by governance")
			}
		}
	}

	// Display planned actions
	displayPublishActions(nextVersion)

	// Dry run - skip actual changes
	if dryRun {
		return nil
	}

	tagName := cfg.Versioning.TagPrefix + nextVersion

	// The pre-release hook and then the release commit, below the dry-run return for the
	// obvious reason: --dry-run promises to change nothing, and a hook is somebody else's
	// code. Shared with `relicta release` — see publish_gates.go.
	if err := prepareReleaseForPublish(ctx, app, nextVersion, tagName); err != nil {
		return err
	}

	// Track publish start time for duration recording
	publishStart := time.Now()

	// Execute publish use case with spinner
	spinner := NewSpinner("Publishing release...")
	spinner.Start()

	input := releaseapp.PublishReleaseInput{
		RepoRoot: repoPath,
		RunID:    run.ID(),
		Actor: ports.ActorInfo{
			Type: "user",
			ID:   "cli",
		},
		Force:  true, // Force since we already validated
		DryRun: false,
	}

	output, err := services.PublishRelease.Execute(ctx, input)

	spinner.Stop()

	if err != nil {
		printError(fmt.Sprintf("Failed to publish release: %v", err))
		// Record failure outcome to Release Memory
		if govResult != nil {
			if rel, relErr := getLatestRelease(ctx, app); relErr == nil {
				recordPublishOutcome(ctx, app, rel, govResult, false, time.Since(publishStart))
			}
		}
		captureReleaseDuration(ctx, app, string(run.ID()), nextVersion, time.Since(publishStart).Milliseconds(), false)
		return fmt.Errorf("failed to publish release: %w", err)
	}

	// Capture release duration + success for DORA-style analytics.
	captureReleaseDuration(ctx, app, string(run.ID()), nextVersion, time.Since(publishStart).Milliseconds(), true)

	// Record success outcome to Release Memory
	if govResult != nil {
		if rel, relErr := getLatestRelease(ctx, app); relErr == nil {
			recordPublishOutcome(ctx, app, rel, govResult, true, time.Since(publishStart))
		}
	}

	// Output step results
	outputStepResults(output.StepResults)

	// The post-release hook runs here: the tag exists and every step has reported, so the
	// release the hook is announcing has actually happened. Before the summary rather than
	// after, because the summary is this command's closing statement — it should stay last,
	// and it stays true either way, since a failing hook does not un-ship a release.
	runPostReleaseHook(ctx, nextVersion, tagName)

	printPublishSummary(nextVersion, tagName, remoteURL)

	return nil
}

// evaluateGovernanceForPublish evaluates the release for governance tracking.
func evaluateGovernanceForPublish(ctx context.Context, app cliApp, rel *release.ReleaseRun) (*governance.EvaluateReleaseOutput, error) {
	govService := app.GovernanceService()
	if govService == nil {
		return nil, fmt.Errorf("governance service not available")
	}

	gitAdapter := app.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	actor := createCGPActor()

	input := governance.EvaluateReleaseInput{
		Release: rel,
		Actor:   actor,
		// This one reads history as well as evaluating, so keying it by the checkout
		// path meant it asked for the history of a directory rather than of the
		// repository — and found none, whatever had been recorded.
		Repository:     repoInfo.GovernanceID(),
		IncludeHistory: cfg.Governance.MemoryEnabled,
	}

	return govService.EvaluateRelease(ctx, input)
}

// recordPublishOutcome records the actual publish outcome to Release Memory.
func recordPublishOutcome(ctx context.Context, app cliApp, rel *release.ReleaseRun, govResult *governance.EvaluateReleaseOutput, success bool, duration time.Duration) {
	govService := app.GovernanceService()
	if govService == nil {
		return
	}

	gitAdapter := app.GitAdapter()
	repoInfo, err := gitAdapter.GetInfo(ctx)
	if err != nil {
		return
	}

	// Determine outcome
	outcome := governance.OutcomeSuccess
	if !success {
		outcome = governance.OutcomeFailure
	}

	actor := createCGPActor()

	// Get risk info from governance result or use defaults
	var riskScore float64
	var decision cgp.DecisionType
	var breakingChanges, securityChanges, filesChanged int

	if govResult != nil {
		riskScore = govResult.RiskScore
		decision = govResult.Decision
	}

	// Extract change metrics from release plan
	var firstCommitAt time.Time
	if plan := release.GetPlan(rel); plan != nil && plan.HasChangeSet() {
		cats := plan.GetChangeSet().Categories()
		breakingChanges = len(cats.Breaking)
		filesChanged = plan.GetChangeSet().Summary().TotalCommits
		// Where DORA lead time starts: the oldest change in this release, not the
		// newest. Lead time asks how long a change waited to reach users, so the
		// release's slowest change is the one that answers it.
		firstCommitAt = earliestCommitDate(plan.GetChangeSet().Commits())
	}

	input := governance.RecordOutcomeInput{
		ReleaseID: rel.ID(),
		// The canonical governance identity, not the checkout path. Recording under
		// the path meant the outcome was invisible to every reader that asked by
		// name or remote — `relicta history` was empty in every repository, and
		// earned trust never found the history it escalates on.
		Repository:      repoInfo.GovernanceID(),
		Version:         rel.Summary().VersionNext,
		Actor:           actor,
		RiskScore:       riskScore,
		Decision:        decision,
		BreakingChanges: breakingChanges,
		SecurityChanges: securityChanges,
		FilesChanged:    filesChanged,
		Outcome:         outcome,
		Duration:        duration,
		FirstCommitAt:   firstCommitAt,
	}

	if err := govService.RecordReleaseOutcome(ctx, input); err != nil {
		printWarning(fmt.Sprintf("Failed to record publish outcome: %v", err))
	}
}

// updateChangelogFile updates the changelog file with new content.
func updateChangelogFile(filename, newContent string) error {
	// Strip any "# Changelog" header from the new content if present
	// This handles cases where the content was generated with a header
	newContent = stripChangelogHeader(newContent)

	// Read existing content
	existingContent := ""
	if data, err := os.ReadFile(filename); err == nil { // #nosec G304 -- user-specified changelog path
		existingContent = string(data)
	}

	var finalContent string

	if existingContent == "" {
		// New file - create with standard header
		finalContent = "# Changelog\n\nAll notable changes to this project will be documented in this file.\n\n" + newContent + "\n"
	} else {
		// Find the first version entry (## [x.y.z] or ## [Unreleased])
		// Insert new content before it
		insertPoint := findVersionEntryPoint(existingContent)

		if insertPoint > 0 {
			finalContent = existingContent[:insertPoint] + newContent + "\n\n" + existingContent[insertPoint:]
		} else {
			// No existing version entries found, append after header
			finalContent = existingContent + "\n\n" + newContent + "\n"
		}
	}

	return os.WriteFile(filename, []byte(finalContent), filePermReadable)
}

// stripChangelogHeader removes any "# Changelog" header from the content.
func stripChangelogHeader(content string) string {
	lines := strings.Split(content, "\n")
	startIdx := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip "# Changelog" or similar headers
		if strings.HasPrefix(trimmed, "# ") && strings.Contains(strings.ToLower(trimmed), "changelog") {
			startIdx = i + 1
			continue
		}
		// Skip empty lines after the header
		if startIdx > 0 && trimmed == "" && i == startIdx {
			startIdx = i + 1
			continue
		}
		// Found actual content
		if trimmed != "" {
			break
		}
	}

	if startIdx > 0 && startIdx < len(lines) {
		return strings.Join(lines[startIdx:], "\n")
	}
	return content
}

// findVersionEntryPoint finds the byte position of the first version entry in the changelog.
func findVersionEntryPoint(content string) int {
	lines := strings.Split(content, "\n")
	pos := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Look for version entries: "## [x.y.z]" or "## [Unreleased]"
		if strings.HasPrefix(trimmed, "## [") {
			return pos
		}
		pos += len(line) + 1 // +1 for newline
	}

	return 0
}

// outputPublishJSONFromServices outputs publish information as JSON from domain services.
func outputPublishJSONFromServices(run *releasedomain.ReleaseRun) error {
	output := map[string]any{
		"release_id":   string(run.ID()),
		"version":      run.VersionNext().String(),
		"tag_name":     cfg.Versioning.TagPrefix + run.VersionNext().String(),
		"state":        string(run.State()),
		"dry_run":      dryRun,
		"ci_mode":      ciMode,
		"skip_tag":     publishSkipTag,
		"skip_push":    publishSkipPush,
		"skip_plugins": publishSkipPlugins,
		"actions": map[string]bool{
			"create_tag":  !publishSkipTag && cfg.Versioning.GitTag,
			"push_tag":    !publishSkipPush && cfg.Versioning.GitPush,
			"run_plugins": !publishSkipPlugins,
		},
	}

	if run.Notes() != nil && run.Notes().Text != "" {
		output["release_notes"] = run.Notes().Text
	}

	if len(cfg.Plugins) > 0 {
		var plugins []string
		for _, p := range cfg.Plugins {
			if p.IsEnabled() {
				plugins = append(plugins, p.Name)
			}
		}
		output["plugins"] = plugins
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// earliestCommitDate returns the oldest commit date in a release, or zero when none
// is known.
//
// The oldest rather than the newest, because DORA lead time for changes measures how
// long a change waited to reach users: a release containing a three-week-old commit
// and one from this morning has a three-week lead time, and reporting the morning's
// would describe the fastest change in the batch instead of the change the metric
// asks about.
//
// Zero when nothing carries a date, which the report reads as unknown rather than as
// the epoch — a substituted zero would turn every such release into a 56-year lead
// time and dominate the average.
func earliestCommitDate(commits []*changes.ConventionalCommit) time.Time {
	var earliest time.Time
	for _, c := range commits {
		if c == nil {
			continue
		}
		date := c.Date()
		if date.IsZero() {
			continue
		}
		if earliest.IsZero() || date.Before(earliest) {
			earliest = date
		}
	}
	return earliest
}

// enforceCleanWorkingTree refuses to publish with uncommitted changes when
// workflow.require_clean_working_tree asks for a clean tree.
//
// The setting defaults to true and was read by no code at all, so every user had this gate
// switched on and relicta published from dirty trees regardless — verified against the
// shipped binary, which tagged v0.1.0 with a modified tracked file present and reported
// "Release completed successfully!".
//
// Why it matters more than tidiness: the tag points at a commit, so uncommitted work is
// precisely the code that is NOT in the release. Publishing from a dirty tree ships a version
// whose author has changes they believe are in it.
//
// Untracked files do not count — see ModifiedTrackedFiles. relicta's own .relicta/ directory
// is untracked in any repository that has not ignored it, and a gate that refused every
// release would be switched off rather than obeyed.
func enforceCleanWorkingTree(ctx context.Context) error {
	if cfg == nil || !cfg.Workflow.RequireCleanWorkingTree {
		return nil
	}

	svc, err := gitservice.NewService()
	if err != nil {
		return fmt.Errorf("could not read the working tree, and "+
			"workflow.require_clean_working_tree requires it: %w", err)
	}

	modified, err := svc.ModifiedTrackedFiles(ctx)
	if err != nil {
		// Unreadable status is not permission to publish: this gate exists to stop a
		// release the operator did not intend, and "I could not tell" is not "it is fine".
		return fmt.Errorf("could not determine whether the working tree is clean, and "+
			"workflow.require_clean_working_tree requires it: %w", err)
	}

	modified = withoutReleaseCommitPaths(ctx, modified)
	if len(modified) == 0 {
		return nil
	}

	shown := modified
	if len(shown) > 10 {
		shown = shown[:10]
	}
	printWarning("Uncommitted changes to tracked files:")
	for _, path := range shown {
		fmt.Fprintf(os.Stderr, "    %s\n", path)
	}
	if len(modified) > len(shown) {
		fmt.Fprintf(os.Stderr, "    ... and %d more\n", len(modified)-len(shown))
	}

	return fmt.Errorf("%d uncommitted change(s) to tracked files, and "+
		"workflow.require_clean_working_tree is set: the tag would point at a commit that "+
		"does not contain them. Commit or stash them, or set "+
		"workflow.require_clean_working_tree: false", len(modified))
}

// enforceUpToDate refuses to publish from a branch that trails its remote when
// workflow.require_up_to_date asks for a fresh branch.
//
// The setting was read by nothing. Verified against the shipped binary: with
// require_up_to_date: true and a colleague's "fix: urgent fix from someone else" sitting on
// origin/main unmerged, publish tagged v0.1.0 and reported "Release completed successfully!".
// The tag named the tip of main and did not contain the fix, and the notes did not mention it.
//
// It fetches — see BehindRemote for why a comparison against whatever the last `git fetch`
// left behind is worse than no check at all. The fetch is announced, because a release command
// that silently opens a network connection is a surprise the operator should not have to read
// the source to discover.
//
// The three answers this gate can get, and why they end differently:
//
// Behind: refuse. The tag would name a commit that is missing work already on the remote,
// which is the whole condition the operator switched this on to prevent.
//
// Cannot tell — the fetch failed, the credentials expired, HEAD is detached and belongs to no
// branch: refuse. #302 settled this: for a gate the operator deliberately switched on, "I
// could not tell" is not "it is fine". The gate defaults to false, so nobody arrives here by
// accident; whoever did asked relicta to prove the branch is current, and an unreachable
// remote is the absence of that proof, not its substitute.
//
// Nothing to trail — no remote is configured, or the branch has never been pushed: pass, with
// a note saying the check was vacuous. This is a determinate answer, not an unknown one: a
// repository with no remote cannot fall behind one, and no amount of operator action would
// ever satisfy a gate that refused it. A gate that can never be satisfied gets switched off
// rather than obeyed — the same reasoning that keeps untracked files out of the clean-tree
// gate — and the setting would become a permanent publish ban instead of a check.
func enforceUpToDate(ctx context.Context) error {
	if cfg == nil || !cfg.Workflow.RequireUpToDate {
		return nil
	}

	svc, err := gitservice.NewService()
	if err != nil {
		return fmt.Errorf("could not open the repository, and "+
			"workflow.require_up_to_date requires reading it: %w", err)
	}

	announce := func(remote string) {
		printInfo(fmt.Sprintf("Fetching %s to check the branch is up to date "+
			"(workflow.require_up_to_date)...", remote))
	}

	fresh, err := svc.BehindRemote(ctx, announce)
	if err != nil {
		return fmt.Errorf("could not determine whether the branch is up to date, and "+
			"workflow.require_up_to_date requires knowing: %w. Fix the remote access, or "+
			"set workflow.require_up_to_date: false", err)
	}

	switch {
	case !fresh.HasRemote():
		printInfo("No remote is configured, so there is nothing for this branch to trail.")
		return nil
	case !fresh.IsPublished():
		printInfo(fmt.Sprintf("Branch %s is not on %s yet, so there is nothing for it to trail.",
			fresh.Branch, fresh.Remote))
		return nil
	case fresh.Behind > 0:
		printWarning(fmt.Sprintf("%s is %d commit(s) ahead of this branch.",
			fresh.RemoteRef, fresh.Behind))
		return fmt.Errorf("branch %s is %d commit(s) behind %s, and "+
			"workflow.require_up_to_date is set: the tag would name a commit that does not "+
			"contain work already on the remote. Pull or rebase first, or set "+
			"workflow.require_up_to_date: false",
			fresh.Branch, fresh.Behind, fresh.RemoteRef)
	}

	printSuccess(fmt.Sprintf("Branch %s is up to date with %s", fresh.Branch, fresh.RemoteRef))
	return nil
}

// withoutReleaseCommitPaths drops the files this release is about to commit itself.
//
// `relicta bump` writes the configured version manifests, so by publish time package.json is
// modified — by relicta, one step earlier in the same release. Counting that as the operator's
// uncommitted work made the two settings contradict each other: with version_files configured
// and require_clean_working_tree at its default, every publish refused a dirty tree that
// relicta had dirtied.
//
// Only excluded when auto_commit_changelog is on, because only then does relicta commit them.
// With it off nothing here is committed by relicta, the operator is managing these files by
// hand, and their being uncommitted is exactly what the gate should report.
func withoutReleaseCommitPaths(ctx context.Context, modified []string) []string {
	managed := make(map[string]struct{}, 4)
	if cfg != nil && cfg.Workflow.AutoCommitChangelog {
		for _, path := range releaseCommitPaths(ctx) {
			managed[filepath.ToSlash(filepath.Clean(path))] = struct{}{}
		}
	}

	kept := make([]string, 0, len(modified))
	for _, path := range modified {
		clean := filepath.ToSlash(filepath.Clean(path))
		if _, isManaged := managed[clean]; isManaged {
			continue
		}
		if isRelictaStorePath(clean) {
			continue
		}
		kept = append(kept, path)
	}
	return kept
}

// isRelictaStorePath reports whether a path belongs to relicta's own release store.
//
// .relicta/ holds the run state relicta writes during every release. Projects that commit it —
// the natural result of `git add -A` once, and the way a team keeps its governance history in
// the repository — otherwise found the second release blocked by the first release's own
// bookkeeping. That is relicta's state, not the operator's uncommitted work, and the gate is
// only asking about the latter.
func isRelictaStorePath(cleanPath string) bool {
	return cleanPath == ".relicta" || strings.HasPrefix(cleanPath, ".relicta/")
}
