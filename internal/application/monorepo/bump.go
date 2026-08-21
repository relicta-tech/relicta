package monorepo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
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
	// Tag is the tag this package's release will carry.
	Tag string
	// BaseRef is the tag the commits were counted from — this package's own last release
	// where it has one, and the repository-wide fallback where it does not.
	BaseRef string
	// Changes are this package's own commits, categorized. Carried so a caller can render
	// the package's changelog from the same analysis that decided its version, rather than
	// walking the log a second time and risking a different answer.
	Changes *changes.ChangeSet
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
	// TagPrefixes are the per-package tag prefixes from
	// monorepo.package_overrides.<path>.tag_prefix, keyed by the path as configured.
	TagPrefixes map[string]string
	// Skip lists packages that monorepo.package_overrides.<path>.skip_versioning excludes,
	// keyed the same way.
	//
	// Excluded from everything, not just the version: a package relicta does not version has
	// no version to tag and no release to describe, so tagging or writing a changelog for it
	// would claim a release nobody asked for.
	Skip map[string]bool
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
	packages = withoutSkipped(packages, input)
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

	// Each package is measured from its own last release. Packages released at different
	// times have different bases, so they are analyzed in groups — one commit walk per
	// distinct base rather than one per package, since in practice most share one.
	//
	// The repository-wide ref is the fallback for a package that has never been released
	// under its own tag, which is every package before the first per-package publish.
	groups := s.groupByBaseRef(ctx, ws.Packages, input)

	plan := &BumpPlan{
		Discovered: len(packages),
		FromRef:    input.FromRef,
		ToRef:      toRef,
	}

	var results []*PackageAnalysisResult
	baseOf := make(map[string]string, len(ws.Packages))
	for baseRef, group := range groups {
		scoped := *ws
		scoped.Packages = group

		out, analyzeErr := s.analyzer.Analyze(ctx, MonorepoAnalyzeInput{
			RepositoryPath: input.RepoRoot,
			FromRef:        baseRef,
			ToRef:          toRef,
			Workspace:      &scoped,
			Strategy:       monorepo.StrategyIndependent,
		})
		if analyzeErr != nil {
			return nil, fmt.Errorf("failed to analyze packages: %w", analyzeErr)
		}
		for _, result := range out.Packages {
			baseOf[result.PackagePath] = baseRef
			results = append(results, result)
		}
	}

	// Discovery order, not map order: two runs of the same command must print the same
	// table in the same order.
	sort.Slice(results, func(i, j int) bool { return results[i].PackagePath < results[j].PackagePath })

	for _, result := range results {
		if result.BumpType == monorepo.BumpTypeNone {
			continue
		}

		// Measure from the last release, not from the working tree. The analyzer reads the
		// manifest on disk, which is the file the previous bump wrote, so bumping twice
		// without releasing took 2.1.3 to 3.0.0 and then to 4.0.0 off the same commit.
		// Reading the manifest as it stood at the base ref makes a second run report the
		// same answer as the first, which is what the repository-wide path does by taking
		// its current version from the tag.
		baseRef := baseOf[result.PackagePath]
		current, next := result.CurrentVersion, result.NextVersion
		if base, ok := s.versionAtRef(ctx, baseRef, input.RepoRoot, result.PackagePath, result.PackageType); ok {
			current = base
			next = monorepo.CalculateNextVersion(base, result.BumpType)
		}

		rel := relativeTo(input.RepoRoot, result.PackagePath)
		plan.Packages = append(plan.Packages, PackageBump{
			Name:    result.PackageName,
			Path:    result.PackagePath,
			Type:    result.PackageType,
			Current: current,
			Next:    next,
			Bump:    result.BumpType,
			Commits: len(result.Commits),
			Files:   s.writer.Files(result.PackagePath, result.PackageType),
			Tag:     TagNameFor(rel, input.TagPrefixes, next),
			BaseRef: baseRef,
			Changes: result.ChangeSet,
		})
	}
	return plan, nil
}

// withoutSkipped drops the packages skip_versioning excludes.
//
// Applied at discovery so that every later step — analysis, writing, tagging, the changelog —
// sees one list. A filter applied per step is a filter one step will eventually forget.
func withoutSkipped(packages []*workspace.Package, input PlanInput) []*workspace.Package {
	if len(input.Skip) == 0 {
		return packages
	}

	kept := make([]*workspace.Package, 0, len(packages))
	for _, pkg := range packages {
		abs := pkg.Path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(input.RepoRoot, abs)
		}
		if input.Skip[relativeTo(input.RepoRoot, abs)] {
			continue
		}
		kept = append(kept, pkg)
	}
	return kept
}

// groupByBaseRef pairs each package with the ref its commits are counted from: the highest
// version tag carrying its own prefix, or the repository-wide fallback when it has none.
func (s *BumpService) groupByBaseRef(
	ctx context.Context,
	packages []*workspace.Package,
	input PlanInput,
) map[string][]*workspace.Package {
	var tags sourcecontrol.TagList
	if s.gitRepo != nil {
		if all, err := s.gitRepo.GetTags(ctx); err == nil {
			tags = all
		}
	}

	groups := make(map[string][]*workspace.Package)
	for _, pkg := range packages {
		prefix := TagPrefixFor(relativeTo(input.RepoRoot, pkg.Path), input.TagPrefixes)
		base := input.FromRef
		if tag, ok := latestTagWithPrefix(tags, prefix); ok {
			base = tag
		}
		groups[base] = append(groups[base], pkg)
	}
	return groups
}

// latestTagWithPrefix returns the highest version tag carrying prefix.
//
// Highest by version rather than most recent by date: a patch on an older line is tagged after
// a newer minor, and taking the newest tag would measure the next release from the wrong place.
func latestTagWithPrefix(tags sourcecontrol.TagList, prefix string) (string, bool) {
	var bestName string
	var best version.SemanticVersion
	found := false

	for _, tag := range tags {
		ver, ok := VersionFromTag(tag.Name(), prefix)
		if !ok {
			continue
		}
		if !found || ver.Compare(best) > 0 {
			best, bestName, found = ver, tag.Name(), true
		}
	}
	return bestName, found
}

// relativeTo is the package path as the configuration writes it.
func relativeTo(root, pkgPath string) string {
	if rel, err := filepath.Rel(root, pkgPath); err == nil {
		return rel
	}
	return pkgPath
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

// PackageTag is one package's release marker.
type PackageTag struct {
	// Name is the package's own name.
	Name string
	// RelPath is the package directory, relative to the repository root.
	RelPath string
	// Tag is the tag to create.
	Tag string
	// Version is the version the package's manifest currently claims.
	Version version.SemanticVersion
}

// ReleaseTags lists the tag each package's current manifest version calls for.
//
// Read from the working tree rather than from a plan, because publish runs after bump has
// already written the manifests, and possibly after somebody edited one by hand. The manifest
// is what the package will ship as, so it is what the tag must name. A package whose tag
// already exists is not filtered here — the caller creates tags idempotently, and deciding it
// twice would mean two answers to one question.
func (s *BumpService) ReleaseTags(ctx context.Context, input PlanInput) ([]PackageTag, error) {
	if input.RepoRoot == "" || len(input.PackagePaths) == 0 {
		return nil, nil
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
	packages = withoutSkipped(packages, input)

	tags := make([]PackageTag, 0, len(packages))
	for _, pkg := range packages {
		abs := pkg.Path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(input.RepoRoot, abs)
		}
		rel := relativeTo(input.RepoRoot, abs)

		pkgType := packageTypeFor(abs, ws)
		ver, readErr := s.writer.ReadVersion(ctx, abs, pkgType)
		if readErr != nil {
			// A package with no readable version has nothing to tag. Skipping it rather than
			// failing keeps one unversioned directory from blocking a release of the rest.
			continue
		}

		tags = append(tags, PackageTag{
			Name:    pkg.Name,
			RelPath: rel,
			Tag:     TagNameFor(rel, input.TagPrefixes, ver),
			Version: ver,
		})
	}

	sort.Slice(tags, func(i, j int) bool { return tags[i].RelPath < tags[j].RelPath })
	return tags, nil
}

// ManifestPaths lists every discovered package's version files, relative to the repository
// root.
//
// These are the files `relicta bump` writes in a monorepo, and the release commit has to cover
// them for the same reason it covers the repository's own manifests: the tag must point at a
// commit that contains the versions it claims, and the clean-tree gate must not count relicta's
// own edits as the operator's uncommitted work.
func (s *BumpService) ManifestPaths(ctx context.Context, input PlanInput) ([]string, error) {
	tags, err := s.ReleaseTags(ctx, input)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(tags))
	for _, pkg := range tags {
		abs := filepath.Join(input.RepoRoot, pkg.RelPath)
		for _, file := range s.writer.Files(abs, packageTypeFor(abs, nil)) {
			paths = append(paths, relativeTo(input.RepoRoot, file))
		}
	}
	return paths, nil
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
