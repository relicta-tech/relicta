// Package monorepo provides domain model for multi-package/monorepo versioning.
package monorepo

import (
	"fmt"
	"sort"
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// TagPattern defines how release tags are formatted for a package.
type TagPattern string

const (
	// TagPatternSlash uses "pkg-name/v1.2.3" format (Go module convention).
	TagPatternSlash TagPattern = "slash"
	// TagPatternAt uses "@scope/pkg@1.2.3" format (npm convention).
	TagPatternAt TagPattern = "at"
	// TagPatternPrefix uses "pkg-name-v1.2.3" format (generic convention).
	TagPatternPrefix TagPattern = "prefix"
)

// PackageVersionInfo holds version information for a single package
// as tracked across releases.
type PackageVersionInfo struct {
	// PackagePath is the path relative to the workspace root.
	PackagePath string
	// PackageName is the display name of the package.
	PackageName string
	// CurrentVersion is the current released version.
	CurrentVersion version.SemanticVersion
	// LastReleaseTag is the git tag of the last release for this package.
	LastReleaseTag string
	// TagPattern determines the tag naming convention.
	TagPattern TagPattern
}

// FormatTag generates the release tag for a given version.
func (info *PackageVersionInfo) FormatTag(ver version.SemanticVersion) string {
	vStr := "v" + ver.String()

	switch info.TagPattern {
	case TagPatternSlash:
		return info.PackagePath + "/" + vStr
	case TagPatternAt:
		return info.PackageName + "@" + ver.String()
	case TagPatternPrefix:
		// Use the last path segment as prefix
		prefix := info.PackagePath
		if idx := strings.LastIndex(prefix, "/"); idx >= 0 {
			prefix = prefix[idx+1:]
		}
		return prefix + "-" + vStr
	default:
		return info.PackagePath + "/" + vStr
	}
}

// ParseTagVersion extracts the version from a tag based on the pattern.
func ParseTagVersion(tag string, pattern TagPattern) (version.SemanticVersion, error) {
	var verStr string

	switch pattern {
	case TagPatternSlash:
		// "pkg-name/v1.2.3" -> "1.2.3"
		parts := strings.Split(tag, "/")
		if len(parts) < 2 {
			return version.Zero, fmt.Errorf("invalid slash tag format: %s", tag)
		}
		verStr = parts[len(parts)-1]
	case TagPatternAt:
		// "@scope/pkg@1.2.3" -> "1.2.3"
		idx := strings.LastIndex(tag, "@")
		if idx < 0 || idx == 0 {
			return version.Zero, fmt.Errorf("invalid at tag format: %s", tag)
		}
		verStr = tag[idx+1:]
	case TagPatternPrefix:
		// "pkg-name-v1.2.3" -> "v1.2.3"
		idx := strings.LastIndex(tag, "-v")
		if idx < 0 {
			return version.Zero, fmt.Errorf("invalid prefix tag format: %s", tag)
		}
		verStr = tag[idx+1:]
	default:
		// Try slash format
		parts := strings.Split(tag, "/")
		verStr = parts[len(parts)-1]
	}

	return version.Parse(verStr)
}

// VersionPlan contains calculated version bumps for all packages in a release.
type VersionPlan struct {
	// Entries maps package paths to their version plan entries.
	Entries map[string]*VersionPlanEntry
	// Strategy is the versioning strategy used.
	Strategy MonorepoStrategy
	// ReleaseGroups maps group names to the packages in each group.
	ReleaseGroups map[string][]string
}

// NewVersionPlan creates a new empty version plan.
func NewVersionPlan(strategy MonorepoStrategy) *VersionPlan {
	return &VersionPlan{
		Entries:       make(map[string]*VersionPlanEntry),
		Strategy:      strategy,
		ReleaseGroups: make(map[string][]string),
	}
}

// AddEntry adds a version plan entry for a package.
func (vp *VersionPlan) AddEntry(entry *VersionPlanEntry) {
	vp.Entries[entry.PackagePath] = entry
}

// GetEntry returns the entry for a package path.
func (vp *VersionPlan) GetEntry(pkgPath string) *VersionPlanEntry {
	return vp.Entries[pkgPath]
}

// AffectedPackages returns paths of packages that have version bumps.
func (vp *VersionPlan) AffectedPackages() []string {
	var affected []string
	for path, entry := range vp.Entries {
		if entry.BumpType != BumpTypeNone {
			affected = append(affected, path)
		}
	}
	sort.Strings(affected)
	return affected
}

// VersionPlanEntry is the version plan for a single package.
type VersionPlanEntry struct {
	// PackagePath is the path to the package.
	PackagePath string
	// PackageName is the display name.
	PackageName string
	// CurrentVersion is the current version.
	CurrentVersion version.SemanticVersion
	// NextVersion is the planned next version.
	NextVersion version.SemanticVersion
	// BumpType is the version bump type.
	BumpType BumpType
	// BumpReason explains why this bump is required.
	BumpReason string
	// TagName is the release tag that will be created.
	TagName string
	// IsDirectChange indicates the package has direct file changes.
	IsDirectChange bool
	// IsDependencyChange indicates the package is bumped due to dependency changes.
	IsDependencyChange bool
	// AffectedDependencies lists the dependencies that caused this bump.
	AffectedDependencies []string
	// ReleaseGroup is the group this package belongs to, if any.
	ReleaseGroup string
}

// VersionCalculator calculates per-package version bumps for monorepo releases.
type VersionCalculator struct {
	// tagPattern determines tag naming convention.
	tagPattern TagPattern
}

// NewVersionCalculator creates a new version calculator with the given tag pattern.
func NewVersionCalculator(pattern TagPattern) *VersionCalculator {
	return &VersionCalculator{
		tagPattern: pattern,
	}
}

// CalculateVersionPlan calculates the version plan for affected packages.
// It handles direct changes, dependency propagation, and release group coordination.
func (vc *VersionCalculator) CalculateVersionPlan(
	packages []*PackageRelease,
	strategy MonorepoStrategy,
	releaseGroups map[string][]string,
) *VersionPlan {
	plan := NewVersionPlan(strategy)
	plan.ReleaseGroups = releaseGroups

	// Step 1: Calculate direct bumps from changes
	for _, pkg := range packages {
		entry := &VersionPlanEntry{
			PackagePath:    pkg.PackagePath,
			PackageName:    pkg.PackageName,
			CurrentVersion: pkg.CurrentVersion,
			BumpType:       pkg.BumpType,
			IsDirectChange: pkg.HasChanges(),
		}

		if pkg.BumpType != BumpTypeNone {
			entry.NextVersion = CalculateNextVersion(pkg.CurrentVersion, pkg.BumpType)
			entry.BumpReason = fmt.Sprintf("direct changes (%d files, %d commits)", len(pkg.ChangedFiles), pkg.CommitCount)
		}

		info := &PackageVersionInfo{
			PackagePath: pkg.PackagePath,
			PackageName: pkg.PackageName,
			TagPattern:  vc.tagPattern,
		}
		if !entry.NextVersion.IsZero() {
			entry.TagName = info.FormatTag(entry.NextVersion)
		}

		plan.AddEntry(entry)
	}

	// Step 2: Propagate dependency bumps
	vc.propagateDependencyBumps(plan, packages)

	// Step 3: Apply strategy-specific coordination
	switch strategy {
	case StrategyLockstep:
		vc.applyLockstepStrategy(plan)
	case StrategyHybrid:
		vc.applyHybridStrategy(plan)
	}

	return plan
}

// propagateDependencyBumps propagates version bumps to dependent packages.
// If package A depends on package B and B has changes, A gets at least a patch bump.
func (vc *VersionCalculator) propagateDependencyBumps(plan *VersionPlan, packages []*PackageRelease) {
	// Build dependency graph: dependents[B] = [A, C, ...] (packages that depend on B)
	dependents := make(map[string][]string)
	for _, pkg := range packages {
		for _, dep := range pkg.Dependencies {
			dependents[dep] = append(dependents[dep], pkg.PackagePath)
		}
	}

	// Iteratively propagate bumps until stable
	changed := true
	for changed {
		changed = false
		for _, pkg := range packages {
			entry := plan.GetEntry(pkg.PackagePath)
			if entry == nil || entry.BumpType == BumpTypeNone {
				continue
			}

			// This package has a bump, propagate to dependents
			for _, depPath := range dependents[pkg.PackagePath] {
				depEntry := plan.GetEntry(depPath)
				if depEntry == nil {
					continue
				}

				// Only propagate if the dependent does not already have an equal or greater bump
				if compareBumpTypes(depEntry.BumpType, BumpTypePatch) < 0 {
					depEntry.BumpType = BumpTypePatch
					depEntry.NextVersion = CalculateNextVersion(depEntry.CurrentVersion, BumpTypePatch)
					depEntry.IsDependencyChange = true
					depEntry.AffectedDependencies = append(depEntry.AffectedDependencies, pkg.PackagePath)
					depEntry.BumpReason = fmt.Sprintf("dependency %s changed", pkg.PackagePath)

					info := &PackageVersionInfo{
						PackagePath: depEntry.PackagePath,
						PackageName: depEntry.PackageName,
						TagPattern:  vc.tagPattern,
					}
					depEntry.TagName = info.FormatTag(depEntry.NextVersion)

					changed = true
				}
			}
		}
	}
}

// applyLockstepStrategy ensures all packages share the same version.
func (vc *VersionCalculator) applyLockstepStrategy(plan *VersionPlan) {
	// Find highest bump and highest current version
	highestBump := BumpTypeNone
	var highestVersion version.SemanticVersion

	for _, entry := range plan.Entries {
		if compareBumpTypes(entry.BumpType, highestBump) > 0 {
			highestBump = entry.BumpType
		}
		if compareVersionsInternal(entry.CurrentVersion, highestVersion) > 0 {
			highestVersion = entry.CurrentVersion
		}
	}

	if highestBump == BumpTypeNone {
		return
	}

	// Apply same version to all packages
	nextVersion := CalculateNextVersion(highestVersion, highestBump)
	for _, entry := range plan.Entries {
		entry.BumpType = highestBump
		entry.NextVersion = nextVersion
		entry.BumpReason = "lockstep release"

		info := &PackageVersionInfo{
			PackagePath: entry.PackagePath,
			PackageName: entry.PackageName,
			TagPattern:  vc.tagPattern,
		}
		entry.TagName = info.FormatTag(nextVersion)
	}
}

// applyHybridStrategy applies per-group lockstep within release groups.
func (vc *VersionCalculator) applyHybridStrategy(plan *VersionPlan) {
	for groupName, groupPackages := range plan.ReleaseGroups {
		// Find highest bump within the group
		highestBump := BumpTypeNone
		var highestVersion version.SemanticVersion

		for _, pkgPath := range groupPackages {
			entry := plan.GetEntry(pkgPath)
			if entry == nil {
				continue
			}
			if compareBumpTypes(entry.BumpType, highestBump) > 0 {
				highestBump = entry.BumpType
			}
			if compareVersionsInternal(entry.CurrentVersion, highestVersion) > 0 {
				highestVersion = entry.CurrentVersion
			}
		}

		if highestBump == BumpTypeNone {
			continue
		}

		// Apply same version to all packages in the group
		nextVersion := CalculateNextVersion(highestVersion, highestBump)
		for _, pkgPath := range groupPackages {
			entry := plan.GetEntry(pkgPath)
			if entry == nil {
				continue
			}
			entry.BumpType = highestBump
			entry.NextVersion = nextVersion
			entry.ReleaseGroup = groupName
			entry.BumpReason = fmt.Sprintf("group %q lockstep release", groupName)

			info := &PackageVersionInfo{
				PackagePath: entry.PackagePath,
				PackageName: entry.PackageName,
				TagPattern:  vc.tagPattern,
			}
			entry.TagName = info.FormatTag(nextVersion)
		}
	}
}

// compareBumpTypes compares two bump types by severity.
func compareBumpTypes(a, b BumpType) int {
	priority := map[BumpType]int{
		BumpTypeNone:  0,
		BumpTypePatch: 1,
		BumpTypeMinor: 2,
		BumpTypeMajor: 3,
	}
	pa, pb := priority[a], priority[b]
	if pa < pb {
		return -1
	}
	if pa > pb {
		return 1
	}
	return 0
}

// compareVersionsInternal compares two semantic versions.
func compareVersionsInternal(a, b version.SemanticVersion) int {
	if a.Major() != b.Major() {
		if a.Major() < b.Major() {
			return -1
		}
		return 1
	}
	if a.Minor() != b.Minor() {
		if a.Minor() < b.Minor() {
			return -1
		}
		return 1
	}
	if a.Patch() != b.Patch() {
		if a.Patch() < b.Patch() {
			return -1
		}
		return 1
	}
	return 0
}

// ReleaseOrder computes the order in which packages should be released,
// respecting dependency relationships. Packages with no dependencies are released first.
func ReleaseOrder(packages []*PackageRelease) []string {
	// Build adjacency list
	deps := make(map[string][]string)
	allPkgs := make(map[string]bool)

	for _, pkg := range packages {
		allPkgs[pkg.PackagePath] = true
		deps[pkg.PackagePath] = append(deps[pkg.PackagePath], pkg.Dependencies...)
	}

	// Topological sort (Kahn's algorithm)
	inDegree := make(map[string]int)
	for pkg := range allPkgs {
		inDegree[pkg] = 0
	}
	for pkg, pkgDeps := range deps {
		if !allPkgs[pkg] {
			continue
		}
		for _, dep := range pkgDeps {
			if allPkgs[dep] {
				inDegree[pkg]++
			}
		}
	}

	// Start with packages that have no internal dependencies
	var queue []string
	for pkg, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, pkg)
		}
	}
	sort.Strings(queue) // Deterministic ordering

	var order []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		// Find packages that depend on current
		for pkg, pkgDeps := range deps {
			for _, dep := range pkgDeps {
				if dep == current {
					inDegree[pkg]--
					if inDegree[pkg] == 0 {
						queue = append(queue, pkg)
						sort.Strings(queue)
					}
				}
			}
		}
	}

	// If there's a cycle, add remaining packages
	if len(order) < len(allPkgs) {
		for pkg := range allPkgs {
			found := false
			for _, o := range order {
				if o == pkg {
					found = true
					break
				}
			}
			if !found {
				order = append(order, pkg)
			}
		}
	}

	return order
}
