package communication

import (
	"sort"
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
)

// BundleType categorizes a bundle of related changes.
type BundleType string

const (
	BundleTypeBreaking    BundleType = "breaking"
	BundleTypeFeature     BundleType = "feature"
	BundleTypeBugfix      BundleType = "bugfix"
	BundleTypePerformance BundleType = "performance"
	BundleTypeSecurity    BundleType = "security"
	BundleTypeDocs        BundleType = "docs"
	BundleTypeChore       BundleType = "chore"
)

// BundledChange represents a single change within a bundle.
type BundledChange struct {
	// Hash is the commit hash.
	Hash string
	// Description is the human-readable change description.
	Description string
	// Scope is the commit scope (e.g., "auth", "api").
	Scope string
	// Breaking indicates whether this is a breaking change.
	Breaking bool
	// BreakingMessage contains the breaking change description, if applicable.
	BreakingMessage string
	// Author is the commit author name.
	Author string
	// Component is the inferred component for monorepo grouping.
	Component string
}

// Bundle groups related changes into a coherent theme.
type Bundle struct {
	// Type categorizes the bundle.
	Type BundleType
	// Theme is a short label describing the bundle (e.g., "Authentication improvements").
	Theme string
	// Summary is a one-sentence summary of the bundle.
	Summary string
	// Changes are the individual changes in this bundle.
	Changes []BundledChange
	// Scopes are the unique scopes found in this bundle.
	Scopes []string
}

// Bundler groups changes from a ChangeSet into thematic bundles.
type Bundler struct{}

// NewBundler creates a new change bundler.
func NewBundler() *Bundler {
	return &Bundler{}
}

// BundleChanges groups a changeset into thematic bundles.
// Changes are grouped first by type (breaking, feature, fix, etc.),
// then by scope to form coherent themes.
func (b *Bundler) BundleChanges(cs *changes.ChangeSet) []Bundle {
	if cs == nil || cs.IsEmpty() {
		return nil
	}

	cats := cs.Categories()
	var bundles []Bundle

	// Breaking changes always get their own top-level bundle.
	if len(cats.Breaking) > 0 {
		bundles = append(bundles, b.bundleByScope(BundleTypeBreaking, "Breaking Changes", cats.Breaking)...)
	}

	// Features grouped by scope.
	nonBreakingFeatures := filterNonBreaking(cats.Features)
	if len(nonBreakingFeatures) > 0 {
		bundles = append(bundles, b.bundleByScope(BundleTypeFeature, "Features", nonBreakingFeatures)...)
	}

	// Bug fixes grouped by scope.
	if len(cats.Fixes) > 0 {
		bundles = append(bundles, b.bundleByScope(BundleTypeBugfix, "Bug Fixes", cats.Fixes)...)
	}

	// Performance improvements.
	if len(cats.Perf) > 0 {
		bundles = append(bundles, b.bundleByScope(BundleTypePerformance, "Performance", cats.Perf)...)
	}

	// Documentation.
	if len(cats.Docs) > 0 {
		bundles = append(bundles, b.bundleByScope(BundleTypeDocs, "Documentation", cats.Docs)...)
	}

	// Security-related changes are extracted from fixes and chores by keyword.
	securityChanges := extractSecurityChanges(cs.Commits())
	if len(securityChanges) > 0 {
		bundles = append(bundles, Bundle{
			Type:    BundleTypeSecurity,
			Theme:   "Security",
			Changes: securityChanges,
			Scopes:  uniqueScopes(securityChanges),
		})
	}

	// Remaining chores, build, CI.
	remaining := mergeCommits(cats.Chores, cats.Build, cats.CI, cats.Refactors)
	if len(remaining) > 0 {
		bundles = append(bundles, b.bundleByScope(BundleTypeChore, "Maintenance", remaining)...)
	}

	return bundles
}

// bundleByScope groups commits of the same type by scope.
// If there are multiple scopes, each gets its own sub-bundle.
// Commits without a scope go into a general bundle.
func (b *Bundler) bundleByScope(bundleType BundleType, baseTheme string, commits []*changes.ConventionalCommit) []Bundle {
	scopeMap := make(map[string][]*changes.ConventionalCommit)
	for _, c := range commits {
		scope := c.Scope()
		if scope == "" {
			scope = "_general"
		}
		scopeMap[scope] = append(scopeMap[scope], c)
	}

	// If all commits share a single scope, produce one bundle.
	if len(scopeMap) == 1 {
		bc := toBundledChanges(commits)
		return []Bundle{{
			Type:    bundleType,
			Theme:   buildTheme(baseTheme, commits),
			Summary: buildBundleSummary(bundleType, bc),
			Changes: bc,
			Scopes:  uniqueScopes(bc),
		}}
	}

	// Multiple scopes: create one bundle per scope.
	var bundles []Bundle
	sortedScopes := sortedKeys(scopeMap)
	for _, scope := range sortedScopes {
		scopeCommits := scopeMap[scope]
		bc := toBundledChanges(scopeCommits)
		theme := baseTheme
		if scope != "_general" {
			theme = baseTheme + " (" + scope + ")"
		}
		bundles = append(bundles, Bundle{
			Type:    bundleType,
			Theme:   theme,
			Summary: buildBundleSummary(bundleType, bc),
			Changes: bc,
			Scopes:  uniqueScopes(bc),
		})
	}

	return bundles
}

// toBundledChanges converts conventional commits to bundled changes.
func toBundledChanges(commits []*changes.ConventionalCommit) []BundledChange {
	result := make([]BundledChange, 0, len(commits))
	for _, c := range commits {
		result = append(result, BundledChange{
			Hash:            c.Hash(),
			Description:     c.Subject(),
			Scope:           c.Scope(),
			Breaking:        c.IsBreaking(),
			BreakingMessage: c.BreakingMessage(),
			Author:          c.Author(),
		})
	}
	return result
}

// buildTheme creates a descriptive theme for a bundle.
func buildTheme(baseTheme string, commits []*changes.ConventionalCommit) string {
	if len(commits) == 0 {
		return baseTheme
	}

	// If all commits share a scope, include it.
	scope := commits[0].Scope()
	allSameScope := scope != ""
	for _, c := range commits[1:] {
		if c.Scope() != scope {
			allSameScope = false
			break
		}
	}

	if allSameScope && scope != "" {
		return baseTheme + " (" + scope + ")"
	}
	return baseTheme
}

// buildBundleSummary generates a one-line summary for a bundle.
func buildBundleSummary(bundleType BundleType, items []BundledChange) string {
	if len(items) == 0 {
		return ""
	}

	if len(items) == 1 {
		return items[0].Description
	}

	switch bundleType {
	case BundleTypeBreaking:
		return strings.Join([]string{
			string(rune('0' + len(items))),
			" breaking change(s) requiring attention",
		}, "")
	case BundleTypeFeature:
		return items[0].Description + " and more"
	default:
		return ""
	}
}

// filterNonBreaking returns commits that are not breaking changes.
func filterNonBreaking(commits []*changes.ConventionalCommit) []*changes.ConventionalCommit {
	result := make([]*changes.ConventionalCommit, 0, len(commits))
	for _, c := range commits {
		if !c.IsBreaking() {
			result = append(result, c)
		}
	}
	return result
}

// extractSecurityChanges finds commits that are security-related based on keywords.
func extractSecurityChanges(commits []*changes.ConventionalCommit) []BundledChange {
	securityKeywords := []string{"security", "cve", "vulnerability", "auth", "xss", "csrf", "injection", "sanitize"}

	var result []BundledChange
	for _, c := range commits {
		subject := strings.ToLower(c.Subject())
		scope := strings.ToLower(c.Scope())
		for _, kw := range securityKeywords {
			if strings.Contains(subject, kw) || strings.Contains(scope, kw) {
				result = append(result, BundledChange{
					Hash:        c.Hash(),
					Description: c.Subject(),
					Scope:       c.Scope(),
					Breaking:    c.IsBreaking(),
					Author:      c.Author(),
				})
				break
			}
		}
	}
	return result
}

// mergeCommits merges multiple commit slices into one.
func mergeCommits(slices ...[]*changes.ConventionalCommit) []*changes.ConventionalCommit {
	total := 0
	for _, s := range slices {
		total += len(s)
	}
	result := make([]*changes.ConventionalCommit, 0, total)
	for _, s := range slices {
		result = append(result, s...)
	}
	return result
}

// uniqueScopes extracts unique scopes from bundled changes.
func uniqueScopes(items []BundledChange) []string {
	seen := make(map[string]struct{})
	var scopes []string
	for _, item := range items {
		if item.Scope != "" {
			if _, ok := seen[item.Scope]; !ok {
				seen[item.Scope] = struct{}{}
				scopes = append(scopes, item.Scope)
			}
		}
	}
	sort.Strings(scopes)
	return scopes
}

// sortedKeys returns the sorted keys of a map.
func sortedKeys(m map[string][]*changes.ConventionalCommit) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// BundleChangesForMonorepo groups changes by component (scope prefix) for monorepo scenarios.
// It re-bundles the output of BundleChanges, grouping bundles by their first scope segment.
func (b *Bundler) BundleChangesForMonorepo(cs *changes.ChangeSet) map[string][]Bundle {
	bundles := b.BundleChanges(cs)
	if len(bundles) == 0 {
		return nil
	}

	result := make(map[string][]Bundle)
	for _, bundle := range bundles {
		component := inferComponent(bundle)
		result[component] = append(result[component], bundle)
	}

	return result
}

// inferComponent determines which component a bundle belongs to.
// Uses the first scope segment (before '/') as the component name.
func inferComponent(bundle Bundle) string {
	if len(bundle.Scopes) == 0 {
		return "core"
	}

	// Use the first scope's prefix as the component.
	scope := bundle.Scopes[0]
	if idx := strings.IndexByte(scope, '/'); idx > 0 {
		return scope[:idx]
	}
	return scope
}
