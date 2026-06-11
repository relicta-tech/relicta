// Package monorepo provides application services for multi-package/monorepo versioning.
package monorepo

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/relicta-tech/relicta/v4/internal/domain/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
	"github.com/relicta-tech/relicta/v4/internal/domain/workspace"
)

// OrchestratorInput contains input parameters for orchestrated multi-package releases.
type OrchestratorInput struct {
	// RepositoryPath is the absolute path to the repository root.
	RepositoryPath string
	// FromRef is the base git reference (e.g., previous release tag).
	FromRef string
	// ToRef is the target git reference (e.g., HEAD).
	ToRef string
	// Workspace is the detected workspace configuration.
	Workspace *workspace.Workspace
	// Strategy is the versioning strategy to use.
	Strategy monorepo.MonorepoStrategy
	// TagPattern is the tag pattern for versioning.
	TagPattern monorepo.TagPattern
	// TargetPackages restricts orchestration to specific packages (nil = all).
	TargetPackages []string
	// ReleaseGroups maps group names to package paths.
	ReleaseGroups map[string][]string
	// DryRun prevents actual release operations.
	DryRun bool
}

// OrchestratorOutput contains the results of orchestrated release.
type OrchestratorOutput struct {
	// Release is the monorepo release aggregate.
	Release *monorepo.MonorepoRelease
	// VersionPlan is the calculated version plan.
	VersionPlan *monorepo.VersionPlan
	// ReleaseOrder is the order in which packages should be released.
	ReleaseOrder []string
	// PackageResults maps package paths to their individual results.
	PackageResults map[string]*PackageReleaseResult
}

// PackageReleaseResult contains the result of releasing a single package.
type PackageReleaseResult struct {
	// PackagePath is the path to the package.
	PackagePath string
	// PackageName is the display name.
	PackageName string
	// CurrentVersion is the version before release.
	CurrentVersion version.SemanticVersion
	// NextVersion is the new version after release.
	NextVersion version.SemanticVersion
	// TagName is the git tag created.
	TagName string
	// Notes are the generated release notes.
	Notes string
	// Approved indicates whether the package was approved.
	Approved bool
	// Published indicates whether the package was published.
	Published bool
	// Error contains any error that occurred during release.
	Error error
}

// ReleaseStepFunc is a function executed during a release step.
// It receives the package path and returns an error if the step fails.
type ReleaseStepFunc func(ctx context.Context, pkgPath string, release *monorepo.MonorepoRelease) error

// Orchestrator coordinates multi-package releases in a monorepo.
// It manages the lifecycle: plan -> bump -> notes -> approve -> publish
// for each package, respecting dependency ordering and release groups.
type Orchestrator struct {
	analyzer    *MonorepoAnalyzer
	versionCalc *monorepo.VersionCalculator
	logger      *slog.Logger

	// Step hooks allow plugging in custom behavior.
	onNotes   ReleaseStepFunc
	onApprove ReleaseStepFunc
	onPublish ReleaseStepFunc
}

// NewOrchestrator creates a new release orchestrator.
func NewOrchestrator(
	analyzer *MonorepoAnalyzer,
	tagPattern monorepo.TagPattern,
) *Orchestrator {
	return &Orchestrator{
		analyzer:    analyzer,
		versionCalc: monorepo.NewVersionCalculator(tagPattern),
		logger:      slog.Default().With("service", "monorepo_orchestrator"),
	}
}

// WithNotesStep sets the release notes generation step.
func (o *Orchestrator) WithNotesStep(fn ReleaseStepFunc) *Orchestrator {
	o.onNotes = fn
	return o
}

// WithApproveStep sets the approval step.
func (o *Orchestrator) WithApproveStep(fn ReleaseStepFunc) *Orchestrator {
	o.onApprove = fn
	return o
}

// WithPublishStep sets the publish step.
func (o *Orchestrator) WithPublishStep(fn ReleaseStepFunc) *Orchestrator {
	o.onPublish = fn
	return o
}

// Plan performs workspace analysis and creates a version plan without executing releases.
func (o *Orchestrator) Plan(ctx context.Context, input OrchestratorInput) (*OrchestratorOutput, error) {
	if input.Workspace == nil {
		return nil, fmt.Errorf("workspace is required")
	}

	o.logger.Info("planning monorepo release",
		"repository", input.RepositoryPath,
		"strategy", input.Strategy,
		"packages", len(input.Workspace.Packages),
	)

	// Step 1: Analyze changes per package
	analyzeInput := MonorepoAnalyzeInput{
		RepositoryPath: input.RepositoryPath,
		FromRef:        input.FromRef,
		ToRef:          input.ToRef,
		Workspace:      input.Workspace,
		Strategy:       input.Strategy,
	}

	analysisOutput, err := o.analyzer.Analyze(ctx, analyzeInput)
	if err != nil {
		return nil, fmt.Errorf("analysis failed: %w", err)
	}

	// Step 2: Create release aggregate from analysis
	release, err := o.analyzer.CreateMonorepoRelease(ctx, analyzeInput, analysisOutput)
	if err != nil {
		return nil, fmt.Errorf("creating release: %w", err)
	}

	// Step 3: Filter to target packages if specified
	if len(input.TargetPackages) > 0 {
		o.filterToTargetPackages(release, input.TargetPackages)
	}

	// Step 4: Calculate version plan with dependency propagation
	versionPlan := o.versionCalc.CalculateVersionPlan(
		release.Packages,
		input.Strategy,
		input.ReleaseGroups,
	)

	// Step 5: Apply version plan to release packages
	for _, pkg := range release.Packages {
		entry := versionPlan.GetEntry(pkg.PackagePath)
		if entry == nil {
			continue
		}
		if entry.BumpType != BumpTypeNone && pkg.BumpType == monorepo.BumpTypeNone {
			// Package was bumped due to dependency propagation
			if err := pkg.SetVersion(entry.NextVersion, entry.BumpType); err != nil {
				o.logger.Warn("failed to set dependency version",
					"package", pkg.PackagePath,
					"error", err,
				)
			}
		}
		pkg.SetTagName(entry.TagName)
	}

	// Step 6: Compute release order
	releaseOrder := monorepo.ReleaseOrder(release.GetIncludedPackages())

	output := &OrchestratorOutput{
		Release:        release,
		VersionPlan:    versionPlan,
		ReleaseOrder:   releaseOrder,
		PackageResults: make(map[string]*PackageReleaseResult),
	}

	// Populate initial package results
	for _, pkg := range release.GetIncludedPackages() {
		output.PackageResults[pkg.PackagePath] = &PackageReleaseResult{
			PackagePath:    pkg.PackagePath,
			PackageName:    pkg.PackageName,
			CurrentVersion: pkg.CurrentVersion,
			NextVersion:    pkg.NextVersion,
			TagName:        pkg.TagName,
		}
	}

	return output, nil
}

// Execute performs the full release orchestration: plan -> bump -> notes -> approve -> publish.
func (o *Orchestrator) Execute(ctx context.Context, input OrchestratorInput) (*OrchestratorOutput, error) {
	// First, plan
	output, err := o.Plan(ctx, input)
	if err != nil {
		return nil, err
	}

	if input.DryRun {
		o.logger.Info("dry run - skipping release execution")
		return output, nil
	}

	release := output.Release

	// Set versions on the release
	if err := release.SetVersions(); err != nil {
		return nil, fmt.Errorf("setting versions: %w", err)
	}

	// Execute release steps for each package in dependency order
	for _, pkgPath := range output.ReleaseOrder {
		pkg := release.GetPackageByPath(pkgPath)
		if pkg == nil || !pkg.IsIncluded() {
			continue
		}

		result := output.PackageResults[pkgPath]

		// Notes step
		if o.onNotes != nil {
			if err := o.onNotes(ctx, pkgPath, release); err != nil {
				result.Error = fmt.Errorf("notes generation failed: %w", err)
				o.logger.Warn("notes step failed", "package", pkgPath, "error", err)
				continue
			}
			result.Notes = pkg.Notes
		}

		// Approve step
		if o.onApprove != nil {
			if err := o.onApprove(ctx, pkgPath, release); err != nil {
				result.Error = fmt.Errorf("approval failed: %w", err)
				o.logger.Warn("approve step failed", "package", pkgPath, "error", err)
				continue
			}
			result.Approved = true
		} else {
			result.Approved = true // Auto-approve if no hook
		}

		// Publish step
		if o.onPublish != nil {
			if err := o.onPublish(ctx, pkgPath, release); err != nil {
				result.Error = fmt.Errorf("publish failed: %w", err)
				o.logger.Warn("publish step failed", "package", pkgPath, "error", err)
				continue
			}
			result.Published = true
		}
	}

	// Mark release notes as ready
	if err := release.GenerateNotes(); err != nil {
		o.logger.Warn("failed to transition to notes_ready", "error", err)
	}

	return output, nil
}

// filterToTargetPackages marks non-target packages as excluded.
func (o *Orchestrator) filterToTargetPackages(release *monorepo.MonorepoRelease, targets []string) {
	targetSet := make(map[string]bool)
	for _, t := range targets {
		targetSet[t] = true
	}

	for _, pkg := range release.Packages {
		if !targetSet[pkg.PackagePath] && !targetSet[pkg.PackageName] {
			_ = pkg.Exclude() // Best-effort exclusion
		}
	}
}

// BumpTypeNone is re-exported for convenience.
const BumpTypeNone = monorepo.BumpTypeNone
