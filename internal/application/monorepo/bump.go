package monorepo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/relicta-tech/relicta/v4/internal/domain/monorepo"
	"github.com/relicta-tech/relicta/v4/internal/domain/sourcecontrol"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
	"github.com/relicta-tech/relicta/v4/internal/domain/workspace"
)

// PackageBump is one package's outcome: what it is at, what it moves to, and which files
// carry the number.
type PackageBump struct {
	// Name is the package's own name, from its manifest.
	Name string
	// Path is the absolute path to the package directory.
	Path string
	// Type is the manifest kind the version is read from and written to.
	Type monorepo.PackageType
	// Current is the version in the package's manifest today.
	Current version.SemanticVersion
	// Next is the version its own commits earn it.
	Next version.SemanticVersion
	// Bump is the increment between them.
	Bump monorepo.BumpType
	// Commits is how many commits in the range touched this package.
	Commits int
	// Files are the manifests that will be rewritten.
	Files []string
}

// BumpPlan is the per-package result of one analysis.
type BumpPlan struct {
	// Packages holds every package with a version to move, in discovery order. Packages no
	// commit touched are absent: in an independent monorepo, an untouched package keeps its
	// version, and listing it as a no-op invites the reader to think it was considered and
	// declined.
	Packages []PackageBump
	// Discovered is how many packages the globs matched, whether or not they changed. Reported
	// so that "nothing to release" and "nothing found" stay distinguishable.
	Discovered int
	// FromRef and ToRef are the range analyzed.
	FromRef string
	ToRef   string
}

// PlanInput describes one repository to plan.
type PlanInput struct {
	// RepoRoot is the repository root the globs are relative to.
	RepoRoot string
	// PackagePaths are the glob patterns from monorepo.package_paths.
	PackagePaths []string
	// ExcludePaths are the patterns from monorepo.exclude_paths.
	ExcludePaths []string
	// FromRef is the base of the commit range; empty means all history.
	FromRef string
	// ToRef is its head; empty means HEAD.
	ToRef string
}

// BumpService computes and applies per-package versions for an independent monorepo.
//
// Independent only, deliberately. lockstep and hybrid are refused at config load rather than
// half-served here: the analyzer can synchronize lockstep versions, but nothing tags or
// releases them, and a strategy that computes the right number and then does nothing with it
// is the defect this subsystem was already an instance of.
type BumpService struct {
	detector workspace.Detector
	analyzer *MonorepoAnalyzer
	gitRepo  sourcecontrol.GitRepository
	writer   *CompositeVersionWriter
}

// NewBumpService wires the discovery, analysis and writing halves together.
func NewBumpService(detector workspace.Detector, analyzer *MonorepoAnalyzer, gitRepo sourcecontrol.GitRepository) *BumpService {
	return &BumpService{
		detector: detector,
		analyzer: analyzer,
		gitRepo:  gitRepo,
		writer:   NewCompositeVersionWriter(),
	}
}

// Plan reports what each package's own commits earn it, without writing anything.
func (s *BumpService) Plan(ctx context.Context, input PlanInput) (*BumpPlan, error) {
	if input.RepoRoot == "" {
		return nil, fmt.Errorf("repository root is required")
	}
	if len(input.PackagePaths) == 0 {
		return nil, fmt.Errorf("monorepo.package_paths is empty, so there is nothing to version " +
			"per package; add the globs that match your packages, for example [\"packages/*\"]")
	}

	ws := &workspace.Workspace{
		RootPath:     input.RepoRoot,
		PackagePaths: input.PackagePaths,
		ExcludePaths: input.ExcludePaths,
		Strategy:     workspace.StrategyIndependent,
	}

	packages, err := s.detector.DiscoverPackages(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("failed to discover packages: %w", err)
	}
	if len(packages) == 0 {
		return nil, fmt.Errorf("no packages matched monorepo.package_paths %v under %s",
			input.PackagePaths, input.RepoRoot)
	}
	// Discovery reports repository-relative paths; everything downstream opens files. The
	// commit-to-package mapping trims the root back off, so absolute paths are what both
	// halves can work from.
	for _, pkg := range packages {
		if !filepath.IsAbs(pkg.Path) {
			pkg.Path = filepath.Join(input.RepoRoot, pkg.Path)
		}
	}
	ws.Packages = packages

	toRef := input.ToRef
	if toRef == "" {
		toRef = "HEAD"
	}

	out, err := s.analyzer.Analyze(ctx, MonorepoAnalyzeInput{
		RepositoryPath: input.RepoRoot,
		FromRef:        input.FromRef,
		ToRef:          toRef,
		Workspace:      ws,
		Strategy:       monorepo.StrategyIndependent,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to analyze packages: %w", err)
	}

	plan := &BumpPlan{
		Discovered: len(packages),
		FromRef:    input.FromRef,
		ToRef:      toRef,
	}
	for _, result := range out.Packages {
		if result.BumpType == monorepo.BumpTypeNone {
			continue
		}

		// Measure from the last release, not from the working tree. The analyzer reads the
		// manifest on disk, which is the file the previous bump wrote, so bumping twice
		// without releasing took 2.1.3 to 3.0.0 and then to 4.0.0 off the same commit.
		// Reading the manifest as it stood at the base ref makes a second run report the
		// same answer as the first, which is what the repository-wide path does by taking
		// its current version from the tag.
		current, next := result.CurrentVersion, result.NextVersion
		if base, ok := s.versionAtRef(ctx, input.FromRef, input.RepoRoot, result.PackagePath, result.PackageType); ok {
			current = base
			next = monorepo.CalculateNextVersion(base, result.BumpType)
		}

		plan.Packages = append(plan.Packages, PackageBump{
			Name:    result.PackageName,
			Path:    result.PackagePath,
			Type:    result.PackageType,
			Current: current,
			Next:    next,
			Bump:    result.BumpType,
			Commits: len(result.Commits),
			Files:   s.writer.Files(result.PackagePath, result.PackageType),
		})
	}
	return plan, nil
}

// versionAtRef reads a package's version from its manifest as that manifest stood at ref.
//
// The bytes are staged into a temporary directory and handed to the same writer that reads the
// working tree, rather than parsed a second way here: two parsers for one manifest format drift,
// and the one that only runs for a base version would drift unnoticed.
//
// Reports false whenever the answer is not knowable — no base ref, no such file at it (a package
// added since the last release), or a manifest that does not parse — and the caller then keeps
// what the working tree says.
func (s *BumpService) versionAtRef(
	ctx context.Context,
	ref, repoRoot, pkgPath string,
	pkgType monorepo.PackageType,
) (version.SemanticVersion, bool) {
	if ref == "" || s.gitRepo == nil {
		return version.Zero, false
	}

	staged, err := os.MkdirTemp("", "relicta-base-manifest")
	if err != nil {
		return version.Zero, false
	}
	defer func() { _ = os.RemoveAll(staged) }()

	var found bool
	for _, file := range s.writer.Files(pkgPath, pkgType) {
		rel, relErr := filepath.Rel(repoRoot, file)
		if relErr != nil {
			continue
		}
		content, readErr := s.gitRepo.GetFileAtRef(ctx, ref, rel)
		if readErr != nil || len(content) == 0 {
			continue
		}
		if writeErr := os.WriteFile(filepath.Join(staged, filepath.Base(file)), content, 0o600); writeErr != nil {
			continue
		}
		found = true
	}
	if !found {
		return version.Zero, false
	}

	ver, err := s.writer.ReadVersion(ctx, staged, pkgType)
	if err != nil {
		return version.Zero, false
	}
	return ver, true
}

// Apply writes each package's next version into its own manifest and returns the files it
// changed.
//
// A failure stops at the package that failed: the manifests written before it keep their new
// versions rather than being rolled back. Reverting would mean writing files again to undo
// writes that a broken tree already interrupted, and the operator can see exactly how far it
// got from the returned list plus the error.
func (s *BumpService) Apply(ctx context.Context, plan *BumpPlan) ([]string, error) {
	if plan == nil {
		return nil, fmt.Errorf("no plan to apply")
	}

	written := make([]string, 0, len(plan.Packages))
	for _, pkg := range plan.Packages {
		if err := s.writer.WriteVersion(ctx, pkg.Path, pkg.Type, pkg.Next); err != nil {
			return written, fmt.Errorf("failed to write version for %s: %w", pkg.Name, err)
		}
		written = append(written, pkg.Files...)
	}
	return written, nil
}
